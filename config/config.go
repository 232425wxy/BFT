package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"
)

const (
	// LogFormatPlain 是一种彩色文本格式
	LogFormatPlain = "plain"
	// LogFormatJSON 是一种 json 文本格式
	LogFormatJSON = "json"

	// DefaultLogLevel 默认的日志记录等级为 info，因此默认情况下，不会记录 debug 日志
	DefaultLogLevel = "info"
)

var (
	DefaultHomeDir   = "CRBFT"
	defaultConfigDir = "config"
	defaultDataDir   = "data"

	defaultConfigFileName  = "config.toml"
	defaultGenesisJSONName = "genesis.json"

	defaultPrivValKeyName   = "priv_validator_key.json"
	defaultPrivValStateName = "priv_validator_state.json"

	defaultNodeKeyName  = "node_key.json"
	defaultAddrBookName = "addrbook.json"

	defaultConfigFilePath   = filepath.Join(defaultConfigDir, defaultConfigFileName)  // config/config.toml
	defaultGenesisJSONPath  = filepath.Join(defaultConfigDir, defaultGenesisJSONName) // config/genesis.json
	defaultPrivValKeyPath   = filepath.Join(defaultConfigDir, defaultPrivValKeyName)  // config/priv_validator_key.json
	defaultPrivValStatePath = filepath.Join(defaultDataDir, defaultPrivValStateName)  // data/priv_validator_state.json

	defaultNodeKeyPath  = filepath.Join(defaultConfigDir, defaultNodeKeyName)  // config/node_key.json
	defaultAddrBookPath = filepath.Join(defaultConfigDir, defaultAddrBookName) // config/addrbook.json
)

// Config 定义了节点的顶级配置
type Config struct {
	// 顶级选项使用匿名结构 squash 标志可将 BaseConfig 里的字段提到 Config 中
	BaseConfig `mapstructure:",squash"`

	// 服务选项
	RPC             *RPCConfig             `mapstructure:"rpc"`
	P2P             *P2PConfig             `mapstructure:"p2p"`
	Mempool         *MempoolConfig         `mapstructure:"mempool"`
	Consensus       *ConsensusConfig       `mapstructure:"consensus"`
	TxIndex         *TxIndexConfig         `mapstructure:"tx_index"`
}

// DefaultConfig 为节点生成一个默认配置
func DefaultConfig() *Config {
	return &Config{
		BaseConfig:      DefaultBaseConfig(),
		RPC:             DefaultRPCConfig(),
		P2P:             DefaultP2PConfig(),
		Mempool:         DefaultMempoolConfig(),
		Consensus:       DefaultConsensusConfig(),
		TxIndex:         DefaultTxIndexConfig(),
	}
}

// SetRoot 为所有种类的配置文件存放地址设置根目录
func (cfg *Config) SetRoot(root string) *Config {
	cfg.BaseConfig.RootDir = root
	cfg.RPC.RootDir = root
	cfg.P2P.RootDir = root
	cfg.Mempool.RootDir = root
	cfg.Consensus.RootDir = root
	return cfg
}

// ValidateBasic 测试所有种类的配置是否正确
func (cfg *Config) ValidateBasic() error {
	if err := cfg.BaseConfig.ValidateBasic(); err != nil {
		return err
	}
	if err := cfg.RPC.ValidateBasic(); err != nil {
		return fmt.Errorf("error in [rpc] section: %w", err)
	}
	if err := cfg.P2P.ValidateBasic(); err != nil {
		return fmt.Errorf("error in [p2p] section: %w", err)
	}
	if err := cfg.Mempool.ValidateBasic(); err != nil {
		return fmt.Errorf("error in [mempool] section: %w", err)
	}
	if err := cfg.Consensus.ValidateBasic(); err != nil {
		return fmt.Errorf("error in [consensus] section: %w", err)
	}
	return nil
}


//-----------------------------------------------------------------------------
// BaseConfig

// BaseConfig 为节点定义了基础配置信息
type BaseConfig struct { //nolint: maligned
	// ChainID 区块链ID
	ChainID string `mapstructure:"chain_id"`

	// RootDir 是所有数据的根目录。这应该在viper中设置，这样它就可以 unmarshal 到这个结构中
	RootDir string `mapstructure:"home"`

	// ProxyApp ABCI 应用程序的 TCP 或 UNIX 套接字地址，或使用 SRBFT 二进制文件编译的 ABCI 应用程序的名称
	ProxyApp string `mapstructure:"proxy_app"`

	// 如果该节点已经落后于区块链很多区块了，FastSyncMode 允许该节点通过并行下载落后的区块并通过快速验证它们的提交来追赶上区块链的进度
	FastSyncMode bool `mapstructure:"fast_sync"`

	// 数据库后端: goleveldb | cleveldb | boltdb | rocksdb | badgerdb
	// 默认情况下，此处选择 “goleveldb”，该数据库由纯 go 语言实现，稳定，最出名的实现方案可在此
	// 处查看：https://github.com/syndtr/goleveldb
	DBBackend string `mapstructure:"db_backend"`

	// 存放数据库的相对目录
	DBPath string `mapstructure:"db_dir"`

	// 日志输出等级
	LogLevel string `mapstructure:"log_level"`

	// 包含初始 Validator集 和其他元数据的 JSON 文件的路径
	Genesis string `mapstructure:"genesis_file"`

	// JSON 文件的路径，该文件包含要用作共识协议中的 Validator 的私钥
	PrivValidatorKey string `mapstructure:"priv_validator_key_file"`

	// 包含 Validator 最新状态的 JSON 文件路径
	PrivValidatorState string `mapstructure:"priv_validator_state_file"`

	// 节点监听来自外部 PrivValidator 进程的连接的 TCP 或 UNIX 套接字地址
	PrivValidatorListenAddr string `mapstructure:"priv_validator_laddr"`

	// 包含用于 p2p 身份验证加密的私钥的 JSON 文件
	NodeKey string `mapstructure:"node_key_file"`
}

// DefaultBaseConfig 为节点返回一个默认的基础配置信息
func DefaultBaseConfig() BaseConfig {
	return BaseConfig{
		Genesis:            defaultGenesisJSONPath,  // config/genesis.json
		PrivValidatorKey:   defaultPrivValKeyPath,   // config/priv_validator_key.json
		PrivValidatorState: defaultPrivValStatePath, // config/priv_validator_state.json
		NodeKey:            defaultNodeKeyPath,
		ProxyApp:           "tcp://127.0.0.1:26658",
		LogLevel:           DefaultLogLevel,
		FastSyncMode:       true,
		DBBackend:          "goleveldb",
		DBPath:             "data",
	}
}

// GenesisFile 返回 genesis.json 文件的绝对路径
func (cfg BaseConfig) GenesisFile() string {
	return rootify(cfg.Genesis, cfg.RootDir)
}

// PrivValidatorKeyFile 返回 priv_validator_key.json 文件的绝对路径
func (cfg BaseConfig) PrivValidatorKeyFile() string {
	return rootify(cfg.PrivValidatorKey, cfg.RootDir)
}

// PrivValidatorStateFile 返回 priv_validator_state.json 文件的绝对路径
func (cfg BaseConfig) PrivValidatorStateFile() string {
	return rootify(cfg.PrivValidatorState, cfg.RootDir)
}

// NodeKeyFile 返回 node_key.json 文件的绝对路径
func (cfg BaseConfig) NodeKeyFile() string {
	return rootify(cfg.NodeKey, cfg.RootDir)
}

// DBDir 返回数据库目录存放位置的绝对路径
func (cfg BaseConfig) DBDir() string {
	return rootify(cfg.DBPath, cfg.RootDir)
}

// ValidateBasic 检查日志记录形式是否是 LogFormatPlain | LogFormatJSON 中的一个，不是的话就会返回错误
func (cfg BaseConfig) ValidateBasic() error {
	return nil
}

//-----------------------------------------------------------------------------
// RPCConfig

// RPCConfig 为节点定义 RPC 服务的配置信息
type RPCConfig struct {
	RootDir string `mapstructure:"home"`

	// RPC 服务器要监听的 TCP 或 UNIX 套接字地址
	ListenAddress string `mapstructure:"listen_addr"`

	// 最大同时连接数(包括WebSocket)，如果您希望接受比默认值更大的数值，请确保您增加了操作系统的限制.
	// Should be < {ulimit -Sn} - {MaxNumInboundPeers} - {MaxNumOutboundPeers} - {N of wal, db and other open files}
	// 1024 - 40 - 10 - 50 = 924 = ~900
	MaxOpenConnections int `mapstructure:"max_open_connections"`

	// 最大可订阅的唯一客户端个数
	// 如果使用 /broadcast_tx_commit，则设置为每个块的 broadcast_tx_commit 调用的估计最大数量。
	MaxSubscriptionClients int `mapstructure:"max_subscription_clients"`

	// 一个客户端可以/订阅的最大唯一查询数
	MaxSubscriptionsPerClient int `mapstructure:"max_subscriptions_per_client"`

	// 在 /broadcast_tx_commit 期间等待 tx 提交需要多长时间
	// 警告:使用大于 10s 的值将导致增加全局 HTTP 写超时，这将适用于所有连接和端点。
	TimeoutBroadcastTxCommit time.Duration `mapstructure:"timeout_broadcast_tx_commit"`

	// 请求体的最大大小，以字节为单位
	MaxBodyBytes int64 `mapstructure:"max_body_bytes"`

	// 请求头的最大大小，以字节为单位
	MaxHeaderBytes int `mapstructure:"max_header_bytes"`
}

// DefaultRPCConfig 返回 RPC 服务器的默认配置
func DefaultRPCConfig() *RPCConfig {
	return &RPCConfig{
		ListenAddress:          "tcp://0.0.0.0:36657",

		MaxOpenConnections: 900,

		MaxSubscriptionClients:    100,
		MaxSubscriptionsPerClient: 50,
		TimeoutBroadcastTxCommit:  30 * time.Second,

		MaxBodyBytes:   int64(1048576), // 1MB
		MaxHeaderBytes: 1 << 20,        // same as the net/http default
	}
}

// ValidateBasic 执行基本验证(检查参数边界等)，如果任何检查失败，返回一个错误。
func (cfg *RPCConfig) ValidateBasic() error {
	if cfg.MaxOpenConnections < 0 {
		return errors.New("max_open_connections can't be negative")
	}
	if cfg.MaxSubscriptionClients < 0 {
		return errors.New("max_subscription_clients can't be negative")
	}
	if cfg.MaxSubscriptionsPerClient < 0 {
		return errors.New("max_subscriptions_per_client can't be negative")
	}
	if cfg.TimeoutBroadcastTxCommit < 0 {
		return errors.New("timeout_broadcast_tx_commit can't be negative")
	}
	if cfg.MaxBodyBytes < 0 {
		return errors.New("max_body_bytes can't be negative")
	}
	if cfg.MaxHeaderBytes < 0 {
		return errors.New("max_header_bytes can't be negative")
	}
	return nil
}


//-----------------------------------------------------------------------------
// P2PConfig

// P2PConfig 为 p2p 定义配置选项
type P2PConfig struct { //nolint: maligned
	RootDir string `mapstructure:"home"`

	// 监听入站连接的地址
	ListenAddress string `mapstructure:"listen_addr"`

	// 要保持持久连接的节点列表
	PersistentPeers string `mapstructure:"persistent_peers"`


	// 存储地址簿的路径
	AddrBook string `mapstructure:"addr_book_file"`

	// 对于需要检查地址是否可路由，需要设置为true
	// 对于私有或本地网络设置为 false
	AddrBookStrict bool `mapstructure:"addr_book_strict"`

	// 最大入站节点数
	MaxNumInboundPeers int `mapstructure:"max_num_inbound_peers"`

	// 要连接的最大出站对等体数目，不包括持久对等体
	MaxNumOutboundPeers int `mapstructure:"max_num_outbound_peers"`

	// 在刷新连接上的消息之前等待的时间
	FlushThrottleTimeout time.Duration `mapstructure:"flush_throttle_timeout"`

	// 消息包有效负载的最大大小，以字节为单位
	MaxPacketMsgPayloadSize int `mapstructure:"max_packet_msg_payload_size"`

	// 握手超时时间
	HandshakeTimeout time.Duration `mapstructure:"handshake_timeout"`
	// 拨号超时时间
	DialTimeout time.Duration `mapstructure:"dial_timeout"`
}

// DefaultP2PConfig 返回点对点层的默认配置
func DefaultP2PConfig() *P2PConfig {
	return &P2PConfig{
		ListenAddress:                "tcp://0.0.0.0:36656",
		AddrBook:                     defaultAddrBookPath,
		AddrBookStrict:               true,
		MaxNumInboundPeers:           50,
		MaxNumOutboundPeers:          50,
		FlushThrottleTimeout:         100 * time.Millisecond,
		MaxPacketMsgPayloadSize:      1024,    // 1 kB
		HandshakeTimeout:             20 * time.Second,
		DialTimeout:                  3 * time.Second,
	}
}

// AddrBookFile 返回地址簿的绝对地址
func (cfg *P2PConfig) AddrBookFile() string {
	return rootify(cfg.AddrBook, cfg.RootDir)
}

// ValidateBasic 执行基本验证(检查参数边界等)，如果任何检查失败，返回一个错误
func (cfg *P2PConfig) ValidateBasic() error {
	if cfg.MaxNumInboundPeers < 0 {
		return errors.New("max_num_inbound_peers can't be negative")
	}
	if cfg.MaxNumOutboundPeers < 0 {
		return errors.New("max_num_outbound_peers can't be negative")
	}
	if cfg.FlushThrottleTimeout < 0 {
		return errors.New("flush_throttle_timeout can't be negative")
	}
	if cfg.MaxPacketMsgPayloadSize < 0 {
		return errors.New("max_packet_msg_payload_size can't be negative")
	}
	return nil
}

//-----------------------------------------------------------------------------
// MempoolConfig

// MempoolConfig 为内存池定义配置选项
type MempoolConfig struct {
	RootDir   string `mapstructure:"home"`
	Recheck   bool   `mapstructure:"recheck"`
	Broadcast bool   `mapstructure:"broadcast"`
	// Wal 的意思是“write-ahead log”，预写日志，WalPath 即为 预写日志的路径
	WalPath string `mapstructure:"wal_dir"`
	// 内存池中的最大事务数
	Size int `mapstructure:"size"`
	// 限制内存池中所有 txs 的总大小。
	// 这只考虑了原始事务(例如，给定1MB的事务，max_txs_bytes=5MB, mempool 将只接受 5 个事务)。
	MaxTxsBytes int64 `mapstructure:"max_txs_bytes"`
	// CacheSize 事务中的缓存大小(用于过滤我们前面看到的事务)
	CacheSize int `mapstructure:"cache_size"`
	// 单个事务的最大大小
	// 注意:在网络上传输的tx的最大大小是 {max_tx_bytes}。
	MaxTxBytes int `mapstructure:"max_tx_bytes"`
}

// DefaultMempoolConfig 为内存池返回默认配置选项
func DefaultMempoolConfig() *MempoolConfig {
	return &MempoolConfig{
		Recheck:   true,
		Broadcast: true,
		WalPath:   "",
		// 每个签名验证需要0.5 ms，直到我们实现 ABCI Recheck
		Size:        5000,
		MaxTxsBytes: 1024 * 1024 * 1024 * 10, // 1GB
		CacheSize:   100000000,
		MaxTxBytes:  1024 * 1024, // 1MB
	}
}

// WalDir 返回内存池的预写日志的绝对路径
func (cfg *MempoolConfig) WalDir() string {
	return rootify(cfg.WalPath, cfg.RootDir)
}

// WalEnabled 预写日志的路径不为空，则返回 true
func (cfg *MempoolConfig) WalEnabled() bool {
	return cfg.WalPath != ""
}

// ValidateBasic 执行基本验证(检查参数边界等)，如果任何检查失败，返回一个错误
func (cfg *MempoolConfig) ValidateBasic() error {
	if cfg.Size < 0 {
		return errors.New("size can't be negative")
	}
	if cfg.MaxTxsBytes < 0 {
		return errors.New("max_txs_bytes can't be negative")
	}
	if cfg.CacheSize < 0 {
		return errors.New("cache_size can't be negative")
	}
	if cfg.MaxTxBytes < 0 {
		return errors.New("max_tx_bytes can't be negative")
	}
	return nil
}

//-----------------------------------------------------------------------------
// ConsensusConfig

// ConsensusConfig 定义了共识服务的配置，包括超时和关于预写日志和块结构的细节。
type ConsensusConfig struct {
	RootDir string `mapstructure:"home"`
	WalPath string `mapstructure:"wal_file"`
	walFile string // overrides WalPath if set

	// 在给 nil 预投票之前，我们等待下一个 proposal 区块的超时时间
	TimeoutPrePrepare time.Duration `mapstructure:"timeout_propose"`
	// 每轮给 timeout_proposal 增加的时间
	TimeoutPrePrepareDelta time.Duration `mapstructure:"timeout_propose_delta"`
	// 我们收到+2/3的预投票后要等多久?不是一个单一块或nil)
	TimeoutPrepare time.Duration `mapstructure:"timeout_prevote"`
	// 每轮的timeout_prevote增加多少
	TimeoutPrepareDelta time.Duration `mapstructure:"timeout_prevote_delta"`
	// 在收到+2/3的“任何东西”的预提交后，我们需要等待多长时间?不是一个单一块或nil)
	TimeoutCommit time.Duration `mapstructure:"timeout_precommit"`
	// 每轮增加多少timeout_precommit
	TimeoutCommitDelta time.Duration `mapstructure:"timeout_precommit_delta"`
	// 提交一个block后，在开始新的高度之前等待的时间(这让我们有机会接收更多的预提交，即使我们已经有+2/3)。
	// 注意:当修改时，请确保更新 time_iota_ms 生成参数
	TimeoutReply time.Duration `mapstructure:"timeout_commit"`

	// 一旦我们有了所有的预提交，就尽快取得进展(就像TimeoutCommit = 0)
	SkipTimeoutReply bool `mapstructure:"skip_timeout_commit"`

	// EmptyBlocks模式和生成两个空块之间的时间间隔
	CreateEmptyBlocks         bool          `mapstructure:"create_empty_blocks"`
	CreateEmptyBlocksInterval time.Duration `mapstructure:"create_empty_blocks_interval"`

	// 反应堆休眠时间参数
	PeerGossipSleepDuration     time.Duration `mapstructure:"peer_gossip_sleep_duration"`
	PeerQueryMaj23SleepDuration time.Duration `mapstructure:"peer_query_maj23_sleep_duration"`

	// 节点是否是拜占庭节点
	TestIsByzantine bool `mapstructure:"test_is_byzantine"`

	TestByzantineRatio float64 `mapstructure:"test_byzantine_ratio"`

	TestWhistleblower bool `mapstructure:"test_whistleblower"`
}

// DefaultConsensusConfig 返回共识服务的默认配置
func DefaultConsensusConfig() *ConsensusConfig {
	return &ConsensusConfig{
		WalPath:                     filepath.Join(defaultDataDir, "cs.wal", "wal"),
		TimeoutPrePrepare:           2500 * time.Millisecond,
		TimeoutPrePrepareDelta:      500 * time.Millisecond,
		TimeoutPrepare:              1000 * time.Millisecond,
		TimeoutPrepareDelta:         500 * time.Millisecond,
		TimeoutCommit:               1000 * time.Millisecond,
		TimeoutCommitDelta:          500 * time.Millisecond,
		TimeoutReply:                1000 * time.Millisecond,
		SkipTimeoutReply:            false,
		CreateEmptyBlocks:           false,
		CreateEmptyBlocksInterval:   0 * time.Second,
		PeerGossipSleepDuration:     100 * time.Millisecond,
		PeerQueryMaj23SleepDuration: 2000 * time.Millisecond,
		TestByzantineRatio:          0.3,
	}
}

// WaitForTxs 在进入 propose 阶段之前，如果需要等待事务，则返回 true
// 如果返回 true 的话，最直观的体现就是不会创建空块，或者创建空块的时间间隔大于 0
func (cfg *ConsensusConfig) WaitForTxs() bool {
	return !cfg.CreateEmptyBlocks || cfg.CreateEmptyBlocksInterval > 0
}

// PrePrepareWait 返回等待 proposal 的超时时间
func (cfg *ConsensusConfig) PrePrepareWait(round int32) time.Duration {
	return time.Duration(
		cfg.TimeoutPrePrepare.Nanoseconds()+cfg.TimeoutPrePrepareDelta.Nanoseconds()*int64(round),
	) * time.Nanosecond
}

// PrepareWait 返回在收到任何 +2/3 prevote 后等待其余节点投票的时间：(1+0.5*round)s
func (cfg *ConsensusConfig) PrepareWait(round int32) time.Duration {
	return time.Duration(
		cfg.TimeoutPrepare.Nanoseconds()+cfg.TimeoutPrepareDelta.Nanoseconds()*int64(round),
	) * time.Nanosecond
}

// CommitWait 返回在收到任何 +2/3 precommit 后等待其余节点投票的时间：(1+0.5*round)s
func (cfg *ConsensusConfig) CommitWait(round int32) time.Duration {
	return time.Duration(
		cfg.TimeoutCommit.Nanoseconds()+cfg.TimeoutCommitDelta.Nanoseconds()*int64(round),
	) * time.Nanosecond
}

// Commit 在收到+2/3的预提交后，返回等待其余节点投票的时间
func (cfg *ConsensusConfig) Commit(t time.Time) time.Time {
	return t.Add(cfg.TimeoutReply)
}

// WalFile 返回预写日志的绝对路径
func (cfg *ConsensusConfig) WalFile() string {
	if cfg.walFile != "" {
		return cfg.walFile
	}
	return rootify(cfg.WalPath, cfg.RootDir)
}

// SetWalFile 设置预写日志的路径
func (cfg *ConsensusConfig) SetWalFile(walFile string) {
	cfg.walFile = walFile
}

// ValidateBasic 执行基本验证(检查参数边界等)，如果任何检查失败，返回一个错误。
func (cfg *ConsensusConfig) ValidateBasic() error {
	if cfg.TimeoutPrePrepare < 0 {
		return errors.New("timeout_propose can't be negative")
	}
	if cfg.TimeoutPrePrepareDelta < 0 {
		return errors.New("timeout_propose_delta can't be negative")
	}
	if cfg.TimeoutPrepare < 0 {
		return errors.New("timeout_prevote can't be negative")
	}
	if cfg.TimeoutPrepareDelta < 0 {
		return errors.New("timeout_prevote_delta can't be negative")
	}
	if cfg.TimeoutCommit < 0 {
		return errors.New("timeout_precommit can't be negative")
	}
	if cfg.TimeoutCommitDelta < 0 {
		return errors.New("timeout_precommit_delta can't be negative")
	}
	if cfg.TimeoutReply < 0 {
		return errors.New("timeout_commit can't be negative")
	}
	if cfg.CreateEmptyBlocksInterval < 0 {
		return errors.New("create_empty_blocks_interval can't be negative")
	}
	if cfg.PeerGossipSleepDuration < 0 {
		return errors.New("peer_gossip_sleep_duration can't be negative")
	}
	if cfg.PeerQueryMaj23SleepDuration < 0 {
		return errors.New("peer_query_maj23_sleep_duration can't be negative")
	}
	return nil
}

//-----------------------------------------------------------------------------
// TxIndexConfig

// TxIndexConfig
// 记住Event有以下结构:
// type: [
//  key: value,
//  ...
// ]
//
// CompositeKeys 由 `type.key` 构造
// TxIndexConfig 定义事务索引器的配置，包括索引的组合键。
type TxIndexConfig struct {
	// 事务使用的索引器
	// 选项:
	//   1) "null"
	//   2) "kv"(默认)——最简单的索引器，支持键值存储(默认为 levelDB，见 DBBackend)。
	Indexer string `mapstructure:"indexer"`
}

// DefaultTxIndexConfig 返回事务索引器的默认配置。
func DefaultTxIndexConfig() *TxIndexConfig {
	return &TxIndexConfig{
		Indexer: "kv",
	}
}

//-----------------------------------------------------------------------------
// Utils

// rootify 传入根目录 root 和相对路径 path，返回 path 的绝对路径：root/path
func rootify(path, root string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}
