package node

import (
	"BFT/blockchain"
	"BFT/gossip"
	srtime "BFT/libs/time"
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	_ "net/http/pprof" // nolint: gosec // securely exposed on separate, optional port
	"path/filepath"
	"sort"
	"strings"
	"time"

	dbm "github.com/tendermint/tm-db"

	cfg "BFT/config"
	cs "BFT/consensus"
	"BFT/crypto"
	tmjson "BFT/libs/json"
	"BFT/libs/log"
	"BFT/libs/pubsub"
	"BFT/libs/service"
	mempl "BFT/mempool"
	"BFT/proxy"
	rpccore "BFT/rpc/core"
	rpcserver "BFT/rpc/jsonrpc/server"
	sm "BFT/state"
	"BFT/state/indexer"
	blockidxkv "BFT/state/indexer/block/kv"
	"BFT/state/txindex"
	"BFT/state/txindex/kv"
	"BFT/store"
	"BFT/types"
)

//------------------------------------------------------------------------------

// DBContext 指定用于加载新 DB 的配置信息
type DBContext struct {
	ID     string
	Config *cfg.Config
}

// DBProvider 接受一个 DBContext 并返回一个实例化的 DB
type DBProvider func(*DBContext) (dbm.DB, error)

// DefaultDBProvider 使用 DBContext.Config 中指定的 DBBackend 和 DBDir 返回一个数据库
func DefaultDBProvider(ctx *DBContext) (dbm.DB, error) {
	dbType := dbm.BackendType(ctx.Config.DBBackend)
	return dbm.NewDB(ctx.ID, dbType, ctx.Config.DBDir())
}

// GenesisDocProvider 返回一个 GenesisDoc，它允许 GenesisDoc 从文件系统
// 以外的源文件中提取，例如从分布式键值存储集群中提取
type GenesisDocProvider func() (*types.GenesisDoc, error)

// DefaultGenesisDocProviderFunc 返回一个 GenesisDocProvider，它从文件
// 系统的 config.GenesisFile() 中加载 GenesisDoc
func DefaultGenesisDocProviderFunc(config *cfg.Config) GenesisDocProvider {
	return func() (*types.GenesisDoc, error) {
		return types.GenesisDocFromFile(config.GenesisFile())
	}
}

// Provider 接受一个配置和一个日志记录器，并返回一个准备运行的节点
type Provider func(*cfg.Config, log.CRLogger) (*Node, error)

// DefaultNewNode 返回一个带有默认设置 PrivValidator、ClientCreator、GenesisDoc 和
// DBProvider 的 SRBFT 节点，。它实现了NodeProvider。
func DefaultNewNode(config *cfg.Config, logger log.CRLogger) (*Node, error) {
	// 加载节点的私钥
	nodeKey, err := gossip.LoadOrGenNodeKey(config.NodeKeyFile())
	if err != nil {
		return nil, fmt.Errorf("failed to load or gen node key %s: %w", config.NodeKeyFile(), err)
	}

	return NewNode(config,
		// 根据配置文件加载生成 PrivValidator
		types.LoadOrGenFilePV(config.PrivValidatorKeyFile(), config.PrivValidatorStateFile()),
		nodeKey, // 节点私钥
		proxy.DefaultClientCreator(),
		DefaultGenesisDocProviderFunc(config),
		DefaultDBProvider,
		logger,
	)
}

// Option 为 Node 配置选项
type Option func(*Node)

//------------------------------------------------------------------------------

// Node 是一个完整的 SRBFT 节点的最高级别接口，它包括所有配置信息和正在运行的服务
type Node struct {
	service.BaseService

	// config
	config        *cfg.Config
	genesisDoc    *types.GenesisDoc   // initial validator set
	privValidator types.PrivValidator // local node's validator key

	// network
	transport   *gossip.Transport
	sw          *gossip.Switch   // p2p connections
	addrBook    *gossip.AddrBook // known peers
	nodeInfo    *gossip.NodeInfo
	nodeKey     *gossip.NodeKey // 节点私钥，用于身份验证
	isListening bool

	// services
	eventBus         *types.EventBus // pub/sub for services
	stateStore       sm.Store
	blockStore       *store.BlockStore // 将区块链存储到磁盘
	bcReactor        gossip.Reactor    // for fast-syncing
	mempoolReactor   *mempl.Reactor    // 用来广播 transactions
	mempool          mempl.Mempool
	consensusState   *cs.State               // 最新的共识状态
	consensusReactor *cs.Reactor             // 参与共识
	proxyApp         *proxy.AppConns          // 连接到应用程序
	rpcListeners     []net.Listener          // rpc servers
	txIndexer        txindex.TxIndexer
	blockIndexer     indexer.BlockIndexer
	indexerService   *txindex.IndexerService
	prometheusSrv    *http.Server
}

// 初始化存储区块链和状态的数据库
func initDBs(config *cfg.Config, dbProvider DBProvider) (blockStore *store.BlockStore, stateDB dbm.DB, err error) {
	var blockStoreDB dbm.DB
	blockStoreDB, err = dbProvider(&DBContext{"blockstore", config})
	if err != nil {
		return
	}
	blockStore = store.NewBlockStore(blockStoreDB)

	stateDB, err = dbProvider(&DBContext{"state", config})
	if err != nil {
		return
	}

	return
}

// 创建并启动代理应用连接
func createAndStartProxyAppConns(clientCreator *proxy.ClientCreator, logger log.CRLogger) (*proxy.AppConns, error) {
	proxyApp := proxy.NewAppConns(clientCreator)
	proxyApp.SetLogger(logger.With("module", "proxy"))
	if err := proxyApp.Start(); err != nil {
		return nil, fmt.Errorf("error starting proxy app connections: %v", err)
	}
	return proxyApp, nil
}

// 创建并启动事件总线
func createAndStartEventBus(logger log.CRLogger) (*types.EventBus, error) {
	eventBus := types.NewEventBus()
	eventBus.SetLogger(logger.With("module", "events"))
	if err := eventBus.Start(); err != nil {
		return nil, err
	}
	return eventBus, nil
}

// 创建并启动索引服务
func createAndStartIndexerService(
	config *cfg.Config,
	dbProvider DBProvider,
	eventBus *types.EventBus,
	logger log.CRLogger,
) (*txindex.IndexerService, txindex.TxIndexer, indexer.BlockIndexer, error) {

	var (
		txIndexer    txindex.TxIndexer
		blockIndexer indexer.BlockIndexer
	)

	switch config.TxIndex.Indexer {
	case "kv":
		store, err := dbProvider(&DBContext{"tx_index_kv", config})
		if err != nil {
			return nil, nil, nil, err
		}

		txIndexer = kv.NewTxIndex(store)
		blockIndexer = blockidxkv.New(dbm.NewPrefixDB(store, []byte("block_events")))
	}

	indexerService := txindex.NewIndexerService(txIndexer, blockIndexer, eventBus)
	indexerService.SetLogger(logger.With("module", "txindex"))

	if err := indexerService.Start(); err != nil {
		return nil, nil, nil, err
	}

	return indexerService, txIndexer, blockIndexer, nil
}

func doHandshake(
	stateStore sm.Store,
	state sm.State,
	blockStore sm.BlockStore,
	genDoc *types.GenesisDoc,
	eventBus types.BlockEventPublisher,
	proxyApp *proxy.AppConns,
	consensusLogger log.CRLogger) error {

	handshaker := cs.NewHandshaker(stateStore, state, blockStore, genDoc)
	handshaker.SetLogger(consensusLogger)
	handshaker.SetEventBus(eventBus)
	if err := handshaker.Handshake(proxyApp); err != nil {
		return fmt.Errorf("error during handshake: %v", err)
	}
	return nil
}

// 获取节点的地址，判断自己的地址是否在 validator 集合中，是的话，则表明自己是一个 validator
func logNodeStartupInfo(state sm.State, pubKey crypto.PubKey, consensusLogger log.CRLogger) {
	addr := pubKey.Address()
	// Log whether this node is a validator or an observer
	if state.Validators.HasAddress(addr) {
		consensusLogger.Infow("This node is a validator", "addr", addr, "pubKey", pubKey)
	} else {
		consensusLogger.Infow("This node is not a validator", "addr", addr, "pubKey", pubKey)
	}
}

// 判断自己是否是系统中唯一的 validator
func onlyValidatorIsUs(state sm.State, pubKey crypto.PubKey) bool {
	if state.Validators.Size() > 1 {
		return false
	}
	addr, _ := state.Validators.GetByIndex(0)
	return bytes.Equal(pubKey.Address(), addr)
}

// 创建内存池和内存池reactor
func createMempoolAndMempoolReactor(config *cfg.Config, proxyApp *proxy.AppConns,
	state sm.State, logger log.CRLogger) (*mempl.Reactor, *mempl.CListMempool) {

	mempool := mempl.NewCListMempool(
		config.Mempool,
		proxyApp.Mempool(),
		state.LastBlockHeight,
		mempl.WithPreCheck(sm.TxPreCheck(state)),
	)
	mempoolLogger := logger.With("module", "mempool")
	mempoolReactor := mempl.NewReactor(config.Mempool, mempool)
	mempoolReactor.SetLogger(mempoolLogger)

	if config.Consensus.WaitForTxs() {
		mempool.EnableTxsAvailable()
	}
	return mempoolReactor, mempool
}

// 创建区块链 reactor
func createBlockchainReactor(state sm.State,
	blockExec *sm.BlockExecutor,
	blockStore *store.BlockStore,
	fastSync bool,
	logger log.CRLogger) (bcReactor gossip.Reactor, err error) {

	bcReactor = blockchain.NewBlockchainReactor(state.Copy(), blockExec, blockStore, fastSync)

	bcReactor.SetLogger(logger.With("module", "blockchain"))
	return bcReactor, nil
}

// 创建 consensus reactor
func createConsensusReactor(config *cfg.Config,
	state sm.State,
	blockExec *sm.BlockExecutor,
	blockStore sm.BlockStore,
	mempool *mempl.CListMempool,
	privValidator types.PrivValidator,
	waitSync bool,
	eventBus *types.EventBus,
	consensusLogger log.CRLogger) (*cs.Reactor, *cs.State) {
	consensusState := cs.NewState(
		config.Consensus,
		state.Copy(),
		blockExec,
		blockStore,
		mempool,
	)
	consensusState.SetLogger(consensusLogger)
	if privValidator != nil {
		consensusState.SetPrivValidator(privValidator)
	}
	consensusReactor := cs.NewReactor(consensusState, waitSync)
	consensusReactor.SetLogger(consensusLogger)
	// services which will be publishing and/or subscribing for messages (events)
	// consensusReactor will set it on consensusState and blockExecutor
	consensusReactor.SetEventBus(eventBus)
	return consensusReactor, consensusState
}

// 创建 transport
func createTransport(
	config *cfg.Config,
	nodeInfo *gossip.NodeInfo,
	nodeKey *gossip.NodeKey,
) (*gossip.Transport) {
	var (
		mConnConfig = gossip.ConstructMultiConnConfig(config.P2P)
		transport   = gossip.NewTransport(*nodeInfo, *nodeKey, mConnConfig)
		connFilters = []gossip.ConnFilterFunc{}
	)

	connFilters = append(connFilters, gossip.ConnDuplicateIPFilter())

	gossip.TransportConnFilters(connFilters...)(transport)

	// 限制最大入站连接数
	gossip.TransportMaxIncomingConnections(config.P2P.MaxNumInboundPeers)(transport)

	return transport
}

// 创建节点交换机
func createSwitch(config *cfg.Config,
	transport *gossip.Transport,
	mempoolReactor *mempl.Reactor,
	bcReactor gossip.Reactor,
	consensusReactor *cs.Reactor,
	nodeInfo *gossip.NodeInfo,
	nodeKey *gossip.NodeKey,
	p2pLogger log.CRLogger) *gossip.Switch {

	sw := gossip.NewSwitch(config.P2P, transport)
	sw.SetLogger(p2pLogger)
	sw.AddReactor("MEMPOOL", mempoolReactor)
	sw.AddReactor("BLOCKCHAIN", bcReactor)
	sw.AddReactor("CONSENSUS", consensusReactor)

	sw.SetNodeInfo(nodeInfo)
	sw.SetNodeKey(nodeKey)

	p2pLogger.Infow("P2P Node ID", "ID", nodeKey.ID(), "file", config.NodeKeyFile())
	return sw
}

// 创建地址簿
func createAddrBookAndSetOnSwitch(config *cfg.Config, sw *gossip.Switch,
	p2pLogger log.CRLogger, nodeKey *gossip.NodeKey) (*gossip.AddrBook, error) {

	addrBook := gossip.NewAddrBook(config.P2P.AddrBookFile(), config.P2P.AddrBookStrict)
	addrBook.SetLogger(p2pLogger.With("book", config.P2P.AddrBookFile()))

	if config.P2P.ListenAddress != "" {
		addr, err := gossip.NewNetAddressString(gossip.IDAddressString(nodeKey.ID(), config.P2P.ListenAddress))
		if err != nil {
			return nil, fmt.Errorf("p2p.laddr is incorrect: %w", err)
		}
		addrBook.AddOurAddress(addr)
	}

	sw.SetAddrBook(addrBook)

	return addrBook, nil
}


// NewNode 返回一个 SRBFT 节点：
//	1. 初始化存储区块链与状态的数据库：initDBs
//	2. 创建 proxyApp 并建立到 ABCI 应用程序的连接(共识、内存池、查询)：createAndStartProxyAppConns
//	3. 创建并启动 eventBus：createAndStartEventBus
//	4. 创建并启动 indexer service：createAndStartIndexerService
//	5.
func NewNode(config *cfg.Config,
	privValidator types.PrivValidator,
	nodeKey *gossip.NodeKey,
	clientCreator *proxy.ClientCreator,
	genesisDocProvider GenesisDocProvider,
	dbProvider DBProvider,
	logger log.CRLogger,
	options ...Option) (*Node, error) {

	eventBus, err := createAndStartEventBus(logger)
	if err != nil {
		return nil, err
	}

	indexerService, txIndexer, blockIndexer, err := createAndStartIndexerService(config, dbProvider, eventBus, logger)
	if err != nil {
		return nil, err
	}

	// 初始化存储区块链与状态的数据库
	blockStore, stateDB, err := initDBs(config, dbProvider)
	if err != nil {
		return nil, err
	}

	stateStore := sm.NewStore(stateDB)

	state, genDoc, err := LoadStateFromDBOrGenesisDocProvider(stateDB, genesisDocProvider)
	if err != nil {
		return nil, err
	}

	pubKey, err := privValidator.GetPubKey()
	if err != nil {
		return nil, fmt.Errorf("can't get pubkey: %w", err)
	}

	nodeInfo, err := makeNodeInfo(config, nodeKey, genDoc, pubKey.Address())
	if err != nil {
		return nil, err
	}

	//##########################################################################################
	// 确认自己是不是拜占庭节点
	amI := ensureWhoisByzantine(splitAndTrimEmpty(config.P2P.PersistentPeers, ",", " "), config, nodeInfo.NodeID)
	if amI {
		logger = logger.With("byzantine", nodeInfo.NodeID)
	}
	//##########################################################################################

	// 创建proxyApp并建立到ABCI应用程序的连接(共识、内存池、查询)
	proxyApp, err := createAndStartProxyAppConns(clientCreator, logger)
	if err != nil {
		return nil, err
	}

	// 创建 handshaker，调用 RequestInfo，重播任何必要的块，以与应用程序同步
	consensusLogger := logger.With("module", "consensus")

	if err := doHandshake(stateStore, state, blockStore, genDoc, eventBus, proxyApp, consensusLogger); err != nil {
		return nil, err
	}

	state, err = stateStore.Load()
	if err != nil {
		return nil, fmt.Errorf("cannot load state: %w", err)
	}

	// 决定我们是否应该进行快速同步，这必须发生在握手之后，因为应用程序可能会修改验证器集，指定我们自己作为唯一的验证器。
	fastSync := config.FastSyncMode && !onlyValidatorIsUs(state, pubKey)

	logNodeStartupInfo(state, pubKey, consensusLogger)

	// Make MempoolReactor
	mempoolReactor, mempool := createMempoolAndMempoolReactor(config, proxyApp, state, logger)

	// make block executor for consensus and blockchain reactors to execute blocks
	blockExec := sm.NewBlockExecutor(stateStore, logger.With("module", "state"), proxyApp.Consensus(), mempool)

	// Make BlockchainReactor. Don't start fast sync if we're doing a state sync first.
	bcReactor, err := createBlockchainReactor(state, blockExec, blockStore, fastSync, logger)
	if err != nil {
		return nil, fmt.Errorf("could not create blockchain reactor: %w", err)
	}

	consensusReactor, consensusState := createConsensusReactor(
		config, state, blockExec, blockStore, mempool, privValidator, fastSync, eventBus, consensusLogger,
	)


	// Setup Transport.
	transport := createTransport(config, nodeInfo, nodeKey)

	// Setup Switch.
	p2pLogger := logger.With("module", "p2p")
	sw := createSwitch(config, transport, mempoolReactor, bcReactor, consensusReactor, nodeInfo, nodeKey, p2pLogger, )

	err = sw.AddPersistentPeers(splitAndTrimEmpty(config.P2P.PersistentPeers, ",", " "))
	if err != nil {
		return nil, fmt.Errorf("could not add peers from persistent_peers field: %w", err)
	}

	addrBook, err := createAddrBookAndSetOnSwitch(config, sw, p2pLogger, nodeKey)
	if err != nil {
		return nil, fmt.Errorf("could not create addrbook: %w", err)
	}

	node := &Node{
		config:        config,
		genesisDoc:    genDoc,
		privValidator: privValidator,

		transport: transport,
		sw:        sw,
		addrBook:  addrBook,
		nodeInfo:  nodeInfo,
		nodeKey:   nodeKey,

		stateStore:       stateStore,
		blockStore:       blockStore,
		bcReactor:        bcReactor,
		mempoolReactor:   mempoolReactor,
		mempool:          mempool,
		consensusState:   consensusState,
		consensusReactor: consensusReactor,
		proxyApp:         proxyApp,
		txIndexer:        txIndexer,
		indexerService:   indexerService,
		blockIndexer:     blockIndexer,
		eventBus:         eventBus,
	}
	node.BaseService = *service.NewBaseService(logger, "Node", node)

	for _, option := range options {
		option(node)
	}

	return node, nil
}

// OnStart starts the Node. It implements service.Service.
func (n *Node) OnStart() error {
	now := srtime.Now()
	genTime := n.genesisDoc.GenesisTime
	if genTime.After(now) {
		n.Logger.Infow("Genesis time is in the future. Sleeping until then...", "genTime", genTime)
		time.Sleep(genTime.Sub(now))
	}

	// Start the RPC server before the P2P server
	// so we can eg. receive txs for the first block
	if n.config.RPC.ListenAddress != "" {
		listeners, err := n.startRPC()
		if err != nil {
			return err
		}
		n.rpcListeners = listeners
	}

	// Start the transport.
	addr, err := gossip.NewNetAddressString(gossip.IDAddressString(n.nodeKey.ID(), n.config.P2P.ListenAddress))
	if err != nil {
		return err
	}
	if err := n.transport.Listen(*addr); err != nil {
		return err
	}

	n.isListening = true

	if n.config.Mempool.WalEnabled() {
		err = n.mempool.InitWAL()
		if err != nil {
			return fmt.Errorf("init mempool WAL: %w", err)
		}
	}

	// Start the switch (the P2P server).
	err = n.sw.Start()
	if err != nil {
		return err
	}

	// Always connect to persistent peers
	err = n.sw.DialPeersAsync(splitAndTrimEmpty(n.config.P2P.PersistentPeers, ",", " "))
	if err != nil {
		return fmt.Errorf("could not dial peers from persistent_peers field: %w", err)
	}

	return nil
}

// OnStop stops the Node. It implements service.Service.
func (n *Node) OnStop() {
	n.BaseService.OnStop()

	n.Logger.Infow("Stopping Node")

	// first stop the non-reactor services
	if err := n.eventBus.Stop(); err != nil {
		n.Logger.Errorw("Error closing eventBus", "err", err)
	}
	if err := n.indexerService.Stop(); err != nil {
		n.Logger.Errorw("Error closing indexerService", "err", err)
	}

	// now stop the reactors
	if err := n.sw.Stop(); err != nil {
		n.Logger.Errorw("Error closing switch", "err", err)
	}

	// stop mempool WAL
	if n.config.Mempool.WalEnabled() {
		n.mempool.CloseWAL()
	}

	if err := n.transport.Close(); err != nil {
		n.Logger.Errorw("Error closing transport", "err", err)
	}

	n.isListening = false

	// finally stop the listeners / external services
	for _, l := range n.rpcListeners {
		n.Logger.Infow("Closing rpc listener", "listener", l)
		if err := l.Close(); err != nil {
			n.Logger.Errorw("Error closing listener", "listener", l, "err", err)
		}
	}

	if pvsc, ok := n.privValidator.(service.Service); ok {
		if err := pvsc.Stop(); err != nil {
			n.Logger.Errorw("Error closing private validator", "err", err)
		}
	}

	if n.prometheusSrv != nil {
		if err := n.prometheusSrv.Shutdown(context.Background()); err != nil {
			// Error from closing listeners, or context timeout:
			n.Logger.Errorw("Prometheus HTTP server Shutdown", "err", err)
		}
	}
}

// ConfigureRPC makes sure RPC has all the objects it needs to operate.
func (n *Node) ConfigureRPC() error {
	pubKey, err := n.privValidator.GetPubKey()
	if err != nil {
		return fmt.Errorf("can't get pubkey: %w", err)
	}
	rpccore.SetEnvironment(&rpccore.Environment{
		ProxyAppQuery:   n.proxyApp.Query(),
		ProxyAppMempool: n.proxyApp.Mempool(),

		StateStore:     n.stateStore,
		BlockStore:     n.blockStore,
		ConsensusState: n.consensusState,
		P2PPeers:       n.sw,
		P2PTransport:   n,

		PubKey:           pubKey,
		GenDoc:           n.genesisDoc,
		TxIndexer:        n.txIndexer,
		BlockIndexer:     n.blockIndexer,
		ConsensusReactor: n.consensusReactor,
		EventBus:         n.eventBus,
		Mempool:          n.mempool,

		Logger: n.Logger.With("module", "rpc"),

		Config: *n.config.RPC,
	})
	return nil
}

func (n *Node) startRPC() ([]net.Listener, error) {
	err := n.ConfigureRPC()
	if err != nil {
		return nil, err
	}

	listenAddrs := splitAndTrimEmpty(n.config.RPC.ListenAddress, ",", " ")

	config := rpcserver.DefaultConfig()
	config.MaxBodyBytes = n.config.RPC.MaxBodyBytes
	config.MaxHeaderBytes = n.config.RPC.MaxHeaderBytes
	config.MaxOpenConnections = n.config.RPC.MaxOpenConnections
	if config.WriteTimeout <= n.config.RPC.TimeoutBroadcastTxCommit {
		config.WriteTimeout = n.config.RPC.TimeoutBroadcastTxCommit + 1*time.Second
	}

	// we may expose the rpc over both a unix and tcp socket
	listeners := make([]net.Listener, len(listenAddrs))
	for i, listenAddr := range listenAddrs {
		mux := http.NewServeMux()
		rpcLogger := n.Logger.With("module", "rpc-server")
		wmLogger := rpcLogger.With("protocol", "websocket")
		wm := rpcserver.NewWebsocketManager(rpccore.Routes,
			rpcserver.OnDisconnect(func(remoteAddr string) {
				err := n.eventBus.UnsubscribeAll(context.Background(), remoteAddr)
				if err != nil && err != pubsub.ErrSubscriptionNotFound {
					wmLogger.Errorw("Failed to unsubscribe addr from events", "addr", remoteAddr, "err", err)
				}
			}),
			rpcserver.ReadLimit(config.MaxBodyBytes),
		)
		wm.SetLogger(wmLogger)
		mux.HandleFunc("/websocket", wm.WebsocketHandler)
		rpcserver.RegisterRPCFuncs(mux, rpccore.Routes, rpcLogger)
		listener, err := rpcserver.Listen(listenAddr, config)
		if err != nil {
			return nil, err
		}

		var rootHandler http.Handler = mux
		go func() {
			if err := rpcserver.Serve(listener, rootHandler, rpcLogger, config); err != nil {
				n.Logger.Errorw("Error serving server", "err", err)
			}
		}()
		listeners[i] = listener
	}

	return listeners, nil

}

// Switch returns the Node's Switch.
func (n *Node) Switch() *gossip.Switch {
	return n.sw
}

// BlockStore returns the Node's BlockStore.
func (n *Node) BlockStore() *store.BlockStore {
	return n.blockStore
}

// ConsensusState returns the Node's ConsensusState.
func (n *Node) ConsensusState() *cs.State {
	return n.consensusState
}

// ConsensusReactor returns the Node's ConsensusReactor.
func (n *Node) ConsensusReactor() *cs.Reactor {
	return n.consensusReactor
}

// MempoolReactor returns the Node's mempool reactor.
func (n *Node) MempoolReactor() *mempl.Reactor {
	return n.mempoolReactor
}

// Mempool returns the Node's mempool.
func (n *Node) Mempool() mempl.Mempool {
	return n.mempool
}

// EventBus returns the Node's EventBus.
func (n *Node) EventBus() *types.EventBus {
	return n.eventBus
}

// PrivValidator returns the Node's PrivValidator.
// XXX: for convenience only!
func (n *Node) PrivValidator() types.PrivValidator {
	return n.privValidator
}

// GenesisDoc returns the Node's GenesisDoc.
func (n *Node) GenesisDoc() *types.GenesisDoc {
	return n.genesisDoc
}

// ProxyApp returns the Node's AppConns, representing its connections to the ABCI application.
func (n *Node) ProxyApp() *proxy.AppConns {
	return n.proxyApp
}

// Config returns the Node's config.
func (n *Node) Config() *cfg.Config {
	return n.config
}

//------------------------------------------------------------------------------

func (n *Node) Listeners() []string {
	return []string{"Listener(@%v)"}
}

func (n *Node) IsListening() bool {
	return n.isListening
}

// NodeInfo returns the Node's Info from the Switch.
func (n *Node) NodeInfo() *gossip.NodeInfo {
	return n.nodeInfo
}

func makeNodeInfo(
	config *cfg.Config,
	nodeKey *gossip.NodeKey,
	genDoc *types.GenesisDoc,
	address crypto.Address,
) (*gossip.NodeInfo, error) {
	var bcChannel byte
	bcChannel = blockchain.BlockchainChannel

	nodeInfo := &gossip.NodeInfo{
		NodeID:  nodeKey.ID(),
		ChainID: genDoc.ChainID,
		Channels: []byte{bcChannel, cs.StateChannel, cs.DataChannel, cs.VoteChannel, cs.VoteSetBitsChannel, mempl.MempoolChannel},
		Address: address,
	}

	nodeInfo.ListenAddr = config.P2P.ListenAddress

	err := nodeInfo.Validate()
	return nodeInfo, err
}

//------------------------------------------------------------------------------

var (
	genesisDocKey = []byte("genesisDoc")
)

// LoadStateFromDBOrGenesisDocProvider attempts to load the state from the
// database, or creates one using the given genesisDocProvider. On success this also
// returns the genesis doc loaded through the given provider.
func LoadStateFromDBOrGenesisDocProvider(
	stateDB dbm.DB,
	genesisDocProvider GenesisDocProvider,
) (sm.State, *types.GenesisDoc, error) {
	// Get genesis doc
	genDoc, err := loadGenesisDoc(stateDB)
	if err != nil {
		genDoc, err = genesisDocProvider()
		if err != nil {
			return sm.State{}, nil, err
		}
		// save genesis doc to prevent a certain class of user errors (e.g. when it
		// was changed, accidentally or not). Also good for audit trail.
		if err := saveGenesisDoc(stateDB, genDoc); err != nil {
			return sm.State{}, nil, err
		}
	}
	stateStore := sm.NewStore(stateDB)
	state, err := stateStore.LoadFromDBOrGenesisDoc(genDoc)
	if err != nil {
		return sm.State{}, nil, err
	}
	return state, genDoc, nil
}

// panics if failed to unmarshal bytes
func loadGenesisDoc(db dbm.DB) (*types.GenesisDoc, error) {
	b, err := db.Get(genesisDocKey)
	if err != nil {
		panic(err)
	}
	if len(b) == 0 {
		return nil, errors.New("genesis doc not found")
	}
	var genDoc *types.GenesisDoc
	err = tmjson.Unmarshal(b, &genDoc)
	if err != nil {
		panic(fmt.Sprintf("Failed to load genesis doc due to unmarshaling error: %v (bytes: %X)", err, b))
	}
	return genDoc, nil
}

// panics if failed to marshal the given genesis document
func saveGenesisDoc(db dbm.DB, genDoc *types.GenesisDoc) error {
	b, err := tmjson.Marshal(genDoc)
	if err != nil {
		return fmt.Errorf("failed to save genesis doc due to marshaling error: %w", err)
	}
	if err := db.SetSync(genesisDocKey, b); err != nil {
		return err
	}

	return nil
}

// splitAndTrimEmpty slices s into all subslices separated by sep and returns a
// slice of the string s with all leading and trailing Unicode code points
// contained in cutset removed. If sep is empty, SplitAndTrim splits after each
// UTF-8 sequence. First part is equivalent to strings.SplitN with a count of
// -1.  also filter out empty strings, only return non-empty strings.
func splitAndTrimEmpty(s, sep, cutset string) []string {
	if s == "" {
		return []string{}
	}

	spl := strings.Split(s, sep)
	nonEmptyStrings := make([]string, 0, len(spl))
	for i := 0; i < len(spl); i++ {
		element := strings.Trim(spl[i], cutset)
		if element != "" {
			nonEmptyStrings = append(nonEmptyStrings, element)
		}
	}
	return nonEmptyStrings
}

func round(x float64) int {
	if _, frac := math.Modf(x); frac >= 0.5 {
		return int(math.Ceil(x))
	}
	return int(math.Floor(x))
}

// TODO 信誉评估
func ensureWhoisByzantine(addrs []string, config *cfg.Config, id gossip.ID) bool {
	ids := make(gossip.IDS, len(addrs))
	for i, addr := range addrs {
		id := gossip.ID(strings.Split(addr, "@")[0])
		ids[i] = id
	}
	sort.Sort(ids)
	byzantine_ratio := config.Consensus.TestByzantineRatio
	nums := round(float64(len(addrs)) * byzantine_ratio)
	for i := 0; i < nums; i++ {
		if ids[i] == id {
			config.Consensus.IsByzantine = true
			cfg.WriteConfigFile(filepath.Join(config.RootDir, "config", "config.toml"), config)
			return true
		}
	}
	return false
}
