package gossip

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/232425wxy/BFT/config"
	"github.com/232425wxy/BFT/libs/cmap"
	"github.com/232425wxy/BFT/libs/rand"
	"github.com/232425wxy/BFT/libs/service"
)

const (
	// dialRandomizerIntervalMilliseconds 给对等端拨号的时候，如果一次拨号不成功，
	// 就会继续尝试拨号，那么随机等待的时间就是 dialRandomizerIntervalMilliseconds
	// 默认是在 3000ms 以内。
	// 设置这个时间是为了防止 DoS 攻击，与 reconnectInterval 搭配使用。
	dialRandomizerIntervalMilliseconds = 3000

	// reconnectInterval 是每次尝试重新建立连接失败后，尝试下一次连接前需等待的时间间隔，默认是 5s
	// 与 dialRandomizerIntervalMilliseconds 搭配使用
	reconnectInterval = 5 * time.Second

	// 与对等端建立连接可能会失败，那么在失败的情况下，会尝试重新与对等端建立连接
	// reconnectAttempts 是尝试重新建立连接的次数，默认是 20 次
	reconnectAttempts = 3

	// 指数回退，3^10 s = 16.4025 hours
	reconnectBackOffAttempts    = 3
	reconnectBackOffBaseSeconds = 3
)

// ConstructMultiConnConfig 将 P2PConfig 转换为 conn.ConstructMultiConnConfig
func ConstructMultiConnConfig(cfg *config.P2PConfig) MultiConnConfig {
	mConfig := DefaultMultiConnConfig()
	mConfig.FlushThrottle = cfg.FlushThrottleTimeout
	mConfig.MaxPacketMsgPayloadSize = cfg.MaxPacketMsgPayloadSize
	return mConfig
}


// PeerFilterFunc 将在一个新的对等体完全建立后由过滤器钩子实现。
type PeerFilterFunc func(*PeerSet, *Peer) error

//-----------------------------------------------------------------------------

// Switch 处理对等连接并公开一个API来接收 “Reactor” 上的传入消息，
// 每个 “Reactor” 负责处理一个或多个 “Channel” 的传入消息。因此，
// 当需要发送消息出去的时候，通常由 Peer 执行发送操作，而传入消息是在 Reactor 上接收的。
type Switch struct {
	service.BaseService

	config       *config.P2PConfig
	reactors     map[string]Reactor
	chDescs      []*ChannelDescriptor
	reactorsByCh map[byte]Reactor
	peers        *PeerSet
	dialing      *cmap.CMap
	reconnecting *cmap.CMap
	nodeInfo     *NodeInfo // 自身信息
	nodeKey      *NodeKey // 掌管着自身秘钥
	addrBook     *AddrBook
	// 我们将与之保持持续联系的对等地址
	persistentPeersAddrs []*NetAddress

	transport *Transport

	filterTimeout time.Duration
	peerFilters   []PeerFilterFunc

	rng *rand.Rand // 随机种子，确定拨号时需等待的随机时间以及给哪些地址进行拨号的顺序
}

// NetAddress 返回 Switch 正在监听的地址，返回的对象是 *NetAddress
func (sw *Switch) NetAddress() *NetAddress {
	addr := sw.transport.NetAddress()
	return &addr
}

// NewSwitch creates a new Switch with the given config.
func NewSwitch(cfg *config.P2PConfig, transport *Transport) *Switch {
	sw := &Switch{
		config:               cfg,
		reactors:             make(map[string]Reactor),
		chDescs:              make([]*ChannelDescriptor, 0),
		reactorsByCh:         make(map[byte]Reactor),
		peers:                NewPeerSet(),
		dialing:              cmap.NewCMap(),
		reconnecting:         cmap.NewCMap(),
		transport:            transport,
		filterTimeout:        defaultFilterTimeout,
		persistentPeersAddrs: make([]*NetAddress, 0),
	}

	// Ensure we have a completely undeterministic PRNG.
	sw.rng = rand.NewRand()

	sw.BaseService = *service.NewBaseService(nil, "P2P Switch", sw)

	return sw
}

//---------------------------------------------------------------------
// Switch setup

// AddReactor 给 Switch 添加一个新的 Reactor，Switch、Reactor 和 Channel 的关系如下：
// 							             Switch
//							                |
//				----------------------------|----------------------------
//				|		                    |			                |
//			 Reactor_1     			     Reactor_2       ...  	     Reactor_n
//              |                           |                           |
//      --------|--------           --------|--------           --------|--------
//      |               |           |               |           |               |
//  Channel_1 ... Channel_t    Channel_t+1 ... Channel_l ... Channel_m+1 ... Channel_n
func (sw *Switch) AddReactor(name string, reactor Reactor) Reactor {
	for _, chDesc := range reactor.GetChannels() {
		chID := chDesc.ID
		// 由于每个 Channel 上的消息只能被唯一一个 Reactor 处理，因此给 Switch 添加
		// 新的 Reactor 的时候，新 Reactor 掌管的 Channel 绝对不能和 Switch 已有的
		// Reactor 掌管的 Channel 重复！
		if sw.reactorsByCh[chID] != nil {
			panic(fmt.Sprintf("Channel %X has multiple reactors %v & %v", chID, sw.reactorsByCh[chID], reactor))
		}
		sw.chDescs = append(sw.chDescs, chDesc)
		sw.reactorsByCh[chID] = reactor
	}
	sw.reactors[name] = reactor
	reactor.SetSwitch(sw)
	return reactor
}

// RemoveReactor 从 Switch 中删除指定的 Reactor，同时删除 Reactor 连带的 Channels
func (sw *Switch) RemoveReactor(name string, reactor Reactor) {
	for _, chDesc := range reactor.GetChannels() {
		// 从 Switch 中删除 Reactor 掌管的 Channel
		for i := 0; i < len(sw.chDescs); i++ {
			if chDesc.ID == sw.chDescs[i].ID {
				sw.chDescs = append(sw.chDescs[:i], sw.chDescs[i+1:]...)
				break
			}
		}
		delete(sw.reactorsByCh, chDesc.ID)
	}
	delete(sw.reactors, name)
	reactor.SetSwitch(nil)
}

// Reactors 返回在 Switch 处注册过的 Reactor
func (sw *Switch) Reactors() map[string]Reactor {
	return sw.reactors
}

// Reactor 根据给定的 Reactor 的名字，返回特定的 Reactor
func (sw *Switch) Reactor(name string) Reactor {
	return sw.reactors[name]
}

// SetNodeInfo 该方法通常在创建 Switch 时调用，设置自身的 nodeInfo
func (sw *Switch) SetNodeInfo(nodeInfo *NodeInfo) {
	sw.nodeInfo = nodeInfo
}

// NodeInfo 返回自身的 nodeInfo
func (sw *Switch) NodeInfo() *NodeInfo {
	return sw.nodeInfo
}

// SetNodeKey 通常是在创建 Switch 时调用，设置自身的私钥，即 nodeKey
func (sw *Switch) SetNodeKey(nodeKey *NodeKey) {
	sw.nodeKey = nodeKey
}

//---------------------------------------------------------------------
// Service start/stop

// OnStart 实现 service.Service 接口，然后启动所有 Reactor
func (sw *Switch) OnStart() error {
	// Start reactors
	for _, reactor := range sw.reactors {
		err := reactor.Start()
		if err != nil {
			return fmt.Errorf("failed to start %v: %w", reactor, err)
		}
	}

	// Start accepting Peers.
	go sw.acceptRoutine()

	return nil
}

// OnStop 实现 service.Service 接口，停止所有 Reactor 和 Peer
func (sw *Switch) OnStop() {
	// Stop peers
	for _, p := range sw.peers.List() {
		sw.stopAndRemovePeer(p, nil)
	}

	// Stop reactors
	sw.Logger.Debugw("Switch: Stopping reactors")
	for _, reactor := range sw.reactors {
		if err := reactor.Stop(); err != nil {
			sw.Logger.Warnw("error while stopped reactor", "reactor", reactor, "error", err)
		}
	}
}

//---------------------------------------------------------------------
// Peers

// Broadcast 调用每个 Peer 的 Send 方法给所有对等端发送消息，且给每个 Peer 发送消息的操作
// 分别放在一个 routine 里，所以发送的顺序是不可确定的
func (sw *Switch) Broadcast(chID byte, msgBytes []byte) chan bool {
	sw.Logger.Debugw("Broadcast", "channel", chID, "msgBytes", fmt.Sprintf("%X", msgBytes))
	peers := sw.peers.List()

	var wg sync.WaitGroup
	wg.Add(len(peers))
	successChan := make(chan bool, len(peers))

	for _, peer := range peers {
		go func(p *Peer) {
			defer wg.Done()
			success := p.Send(chID, msgBytes)
			successChan <- success
		}(peer)
	}

	go func() {
		wg.Wait()
		close(successChan)
	}()

	return successChan
}

// NumPeers 返回出站/入站和正在出站拨号对等点的数量。这里不计算无条件对等点。
func (sw *Switch) NumPeers() (outbound, inbound, dialing int) {
	peers := sw.peers.List()
	for _, peer := range peers {
		if peer.IsOutbound() {
			outbound++
		} else {
			inbound++
		}
	}
	dialing = sw.dialing.Size()
	return
}

// Peers 返回与 Switch 建立连接的 Peer 集合
func (sw *Switch) Peers() *PeerSet {
	return sw.peers
}

// StopPeerForError 由于外部错误导致与对等节点断开连接；
// 如果对等端是持久的，它将尝试重新与其建立连接。
func (sw *Switch) StopPeerForError(peer *Peer, reason interface{}) {
	if !peer.IsRunning() {
		return
	}

	sw.Logger.Warnw("Stopping Peer for error", "Peer", peer, "err", reason)
	sw.stopAndRemovePeer(peer, reason)

	if peer.IsPersistent() {
		var addr *NetAddress
		if peer.IsOutbound() { // 如果该 Peer 是出站节点，就给它的 socketAddr 拨号
			addr = peer.SocketAddr()
		} else { // 如果该节点是入站节点，就给其监听地址拨号
			var err error
			addr, err = peer.NodeInfo().NetAddress()
			if err != nil {
				sw.Logger.Warnw("Wanted to reconnect to inbound Peer, but self-reported address is wrong",
					"Peer", peer, "err", err)
				return
			}
		}
		go sw.reconnectToPeer(addr)
	}
}

func (sw *Switch) stopAndRemovePeer(peer *Peer, reason interface{}) {
	sw.transport.Cleanup(peer)
	if err := peer.Stop(); err != nil {
		sw.Logger.Warnw("error while stopping Peer", "error", err) // TODO: should return error to be handled accordingly
	}

	for _, reactor := range sw.reactors {
		reactor.RemovePeer(peer, reason)
	}

	// 删除对等点应该最后进行，以避免对等点重新连接到我们的节点，Switch 在 RemovePeer 完成之前调用 InitPeer 的情况。
	sw.peers.Remove(peer)
}

// reconnectToPeer 尝试重新连接到 addr，首先以固定的间隔重复连接，然后以指数回退。
// 如果在所有这些之后都没有成功，它将停止尝试，并将它留给 PEX/Addrbook 来再次找到带有 addr 的对等体
// 注意:即使握手或认证失败，它也会继续尝试。
func (sw *Switch) reconnectToPeer(addr *NetAddress) {
	if sw.reconnecting.Has(string(addr.ID)) {
		return
	}
	sw.reconnecting.Set(string(addr.ID), addr)
	defer sw.reconnecting.Delete(string(addr.ID))

	start := time.Now()
	sw.Logger.Infow("Reconnecting to Peer", "addr", addr)
	for i := 0; i < reconnectAttempts; i++ {
		if !sw.IsRunning() {
			return
		}

		err := sw.DialPeerWithAddress(addr)
		if err == nil {
			return // success
		} else if _, ok := err.(ErrCurrentlyDialingOrExistingAddress); ok {
			return
		}

		sw.Logger.Infow("Error reconnecting to Peer. Trying again", "tries", i, "err", err, "addr", addr)
		// sleep a set amount
		sw.randomSleep(reconnectInterval)
		continue
	}

	sw.Logger.Warnw("Failed to reconnect to Peer. Beginning exponential backoff", "addr", addr, "elapsed", time.Since(start))
	for i := 0; i < reconnectBackOffAttempts; i++ {
		if !sw.IsRunning() {
			return
		}

		// 最后一次会睡眠将近 16 个小时
		sleepIntervalSeconds := math.Pow(reconnectBackOffBaseSeconds, float64(i))
		sw.randomSleep(time.Duration(sleepIntervalSeconds) * time.Second)

		err := sw.DialPeerWithAddress(addr)
		if err == nil {
			return // success
		} else if _, ok := err.(ErrCurrentlyDialingOrExistingAddress); ok {
			return
		}
		sw.Logger.Infow("Error reconnecting to Peer. Trying again", "tries", i, "err", err, "addr", addr)
	}
	sw.Logger.Warnw("Failed to reconnect to Peer. Giving up", "addr", addr, "elapsed", time.Since(start))
}

// SetAddrBook 给 Switch 设置地址簿
func (sw *Switch) SetAddrBook(addrBook *AddrBook) {
	sw.addrBook = addrBook
}

//---------------------------------------------------------------------
// Dialing

// ErrAddrBookPrivate 和 ErrAddrBookPrivateSrc 两个错误实现了该接口
type privateAddr interface {
	PrivateAddr() bool
}

func isPrivateAddr(err error) bool {
	te, ok := err.(privateAddr)
	return ok && te.PrivateAddr()
}

// DialPeersAsync 以随机顺序异步地拨号一个对等体列表。
// 用于在启动时从配置或从不安全的 rpc (可信源)拨号对等点。
// 它忽略了 ErrNetAddressLookup。但是，如果有其他错误，则返回第一次遇到的错误。
func (sw *Switch) DialPeersAsync(peers []string) error {
	netAddrs, errs := NewNetAddressStrings(peers)
	// report all the errors
	for _, err := range errs {
		sw.Logger.Warnw("Error in Peer's address", "err", err)
	}
	// return first non-ErrNetAddressLookup error
	for _, err := range errs {
		if _, ok := err.(ErrNetAddressLookup); ok {
			continue
		}
		return err
	}
	sw.dialPeersAsync(netAddrs)
	return nil
}

func (sw *Switch) dialPeersAsync(netAddrs []*NetAddress) {
	ourAddr := sw.NetAddress()

	if sw.addrBook != nil {
		// add peers to `addrBook`
		for _, netAddr := range netAddrs {
			// do not add our address or ID
			if !netAddr.Same(ourAddr) {
				if err := sw.addrBook.AddAddress(netAddr); err != nil {
					if isPrivateAddr(err) {
						sw.Logger.Debugw("Won't add Peer's address to addrbook", "err", err)
					} else {
						sw.Logger.Warnw("Can't add Peer's address to addrbook", "err", err)
					}
				}
			}
		}
		// 立刻将 Switch 的地址簿写入磁盘
		sw.addrBook.Save()
	}

	// 生成一个随机序列
	perm := sw.rng.Perm(len(netAddrs))
	for i := 0; i < len(perm); i++ {
		go func(i int) {
			j := perm[i]
			addr := netAddrs[j]

			if addr.Same(ourAddr) {
				sw.Logger.Debugw("Ignore attempt to connect to ourselves", "addr", addr, "ourAddr", ourAddr)
				return
			}

			sw.randomSleep(0)

			err := sw.DialPeerWithAddress(addr)
			if err != nil {
				switch err.(type) {
				case ErrSwitchConnectToSelf, ErrSwitchDuplicatePeerID, ErrCurrentlyDialingOrExistingAddress:
					sw.Logger.Debugw("Error dialing Peer", "err", err)
				default:
					sw.Logger.Warnw("Error dialing Peer", "err", err)
				}
			}
		}(i)
	}
}

// DialPeerWithAddress 根据给定的对等端的地址，给对等端拨号，如果已经正在给该地址拨号，或者这个地址
// 属于某个已知的 Peer，则返回 ErrCurrentlyDialingOrExistingAddress 错误，否则，如果拨号成功，
// 就调用 Switch 的 addPeer 方法将该 Peer 放入 Peer 集合中
func (sw *Switch) DialPeerWithAddress(addr *NetAddress) error {
	if sw.IsDialingOrExistingAddress(addr) {
		return ErrCurrentlyDialingOrExistingAddress{addr.String()}
	}

	sw.dialing.Set(string(addr.ID), addr)
	defer sw.dialing.Delete(string(addr.ID))

	return sw.addOutboundPeer(addr)
}

// randomSleep 随机睡眠一段时间，睡眠时间取值范围为： [interval, dialRandomizerIntervalMilliseconds+interval]
func (sw *Switch) randomSleep(interval time.Duration) {
	r := time.Duration(sw.rng.Int63n(dialRandomizerIntervalMilliseconds)) * time.Millisecond
	time.Sleep(r + interval)
}

// IsDialingOrExistingAddress 判断我们是否正在给指定的地址拨号，或者该地址是否已经属于某个已知 Peer
func (sw *Switch) IsDialingOrExistingAddress(addr *NetAddress) bool {
	return sw.dialing.Has(string(addr.ID)) || sw.peers.HasID(addr.ID) || sw.peers.HasIP(addr.IP)
}

// AddPersistentPeers 允许我们设置持久对等体。它忽略了 ErrNetAddressLookup 错误。
// 但是，如果有其他错误，则返回第一次遇到的错误。
func (sw *Switch) AddPersistentPeers(addrs []string) error {
	sw.Logger.Infow("Adding persistent peers", "addrs", addrs)
	netAddrs, errs := NewNetAddressStrings(addrs)

	for _, err := range errs {
		sw.Logger.Warnw("Error in Peer's address", "err", err)
	}
	// 返回第一个非 ErrNetAddressLookup 的错误
	for _, err := range errs {
		if _, ok := err.(ErrNetAddressLookup); ok {
			continue
		}
		return err
	}
	sw.persistentPeersAddrs = netAddrs

	return nil
}

// AddPrivatePeerIDs 往地址簿中添加私密节点的 ID
func (sw *Switch) AddPrivatePeerIDs(ids []string) error {
	validIDs := make([]string, 0, len(ids))
	for i, id := range ids {
		err := validateID(ID(id))
		if err != nil {
			return fmt.Errorf("wrong ID #%d: %w", i, err)
		}
		validIDs = append(validIDs, id)
	}

	return nil
}

// IsPeerPersistent 通过翻阅 Switch 的 persistentPeersAddrs，查看给定的地址是否是 persistent
func (sw *Switch) IsPeerPersistent(na *NetAddress) bool {
	for _, pa := range sw.persistentPeersAddrs {
		if pa.Equals(na) {
			return true
		}
	}
	return false
}

// acceptRoutine 被放在 go-routine 中持续运行
// 时刻从 Transport.Accept 中获取新的 Peer
func (sw *Switch) acceptRoutine() {
	for {
		p, err := sw.transport.Accept(peerConfig{
			chDescs:      sw.chDescs,
			onPeerError:  sw.StopPeerForError,
			reactorsByCh: sw.reactorsByCh,
			isPersistent: sw.IsPeerPersistent,
		})
		if err != nil {
			switch err := err.(type) {
			case ErrRejected:
				if err.IsSelf() {
					// RemoveConn the given address from the address book and add to our addresses
					// to avoid dialing in the future.
					addr := err.Addr()
					sw.addrBook.RemoveAddress(&addr)
					sw.addrBook.AddOurAddress(&addr)
				}

				sw.Logger.Infow(
					"Inbound Peer rejected",
					"err", err,
					"numPeers", sw.peers.Size(),
				)

				continue
			case ErrFilterTimeout:
				sw.Logger.Warnw(
					"Peer filter timed out",
					"err", err,
				)

				continue
			case ErrTransportClosed:
				sw.Logger.Warnw(
					"Stopped accept routine, as transport is closed",
					"numPeers", sw.peers.Size(),
				)
			default:
				sw.Logger.Warnw(
					"Accept on transport errored",
					"err", err,
					"numPeers", sw.peers.Size(),
				)
				// 我们可以有一个重试循环周围的acceptRoutine，
				// 但这将需要停止并让节点最终关闭。因此，不妨 panic，
				// 让流程管理器重新启动节点。让节点在没有acceptRoutine
				// 的情况下运行是没有意义的，因为它将不能接受新的连接。
				panic(fmt.Errorf("accept routine exited: %v", err))
			}

			break
		}

		// 如果新 Peer 不是无条件的 Peer，那么在我们含有的 Peer 达到上限时，则忽略该新 Peer
		_, in, _ := sw.NumPeers()
		if in >= sw.config.MaxNumInboundPeers {
			sw.Logger.Infow(
				"Ignoring inbound connection: already have enough inbound peers",
				"address", p.SocketAddr(),
				"have", in,
				"max", sw.config.MaxNumInboundPeers,
			)

			sw.transport.Cleanup(p)

			continue
		}


		if err := sw.addPeer(p); err != nil {
			sw.transport.Cleanup(p)
			if p.IsRunning() {
				_ = p.Stop()
			}
			sw.Logger.Infow(
				"Ignoring inbound connection: error while adding Peer",
				"err", err,
				"id", p.ID(),
			)
		}
	}
}

// addOutboundPeer 添加出站 Peer
func (sw *Switch) addOutboundPeer(addr *NetAddress) error {
	sw.Logger.Infow("Dialing Peer", "address", addr)

	p, err := sw.transport.Dial(*addr, peerConfig{
		chDescs:      sw.chDescs,
		onPeerError:  sw.StopPeerForError,
		isPersistent: sw.IsPeerPersistent,
		reactorsByCh: sw.reactorsByCh,
	})
	if err != nil {
		if e, ok := err.(ErrRejected); ok {
			if e.IsSelf() {
				// 如果我们拨号的地址使我们自己的地址，那么就从地址簿中删除该地址，并且将该地址添加到我们自己地址的集合中
				sw.addrBook.RemoveAddress(addr)
				sw.addrBook.AddOurAddress(addr)

				return err
			}
		}

		// 如果该地址是 persistent，那么在拨号失败的情况下，应该给该地址重拨
		if sw.IsPeerPersistent(addr) {
			go sw.reconnectToPeer(addr)
		}

		return err
	}

	if err := sw.addPeer(p); err != nil {
		sw.transport.Cleanup(p)
		if p.IsRunning() {
			_ = p.Stop()
		}
		return err
	}

	return nil
}

// addPeer 启动 Peer 并将其添加到 Switch 中。如果对等端被过滤掉、启动失败或无法添加，则返回错误。
func (sw *Switch) addPeer(p *Peer) error {
	p.SetLogger(sw.Logger.With("Peer", p.SocketAddr()))

	// 处理 Switch 已停止但我们还想试图添加对等点的情况。
	if !sw.IsRunning() {
		sw.Logger.Warnw("Won't start a Peer - switch is not running", "Peer", p)
		return nil
	}

	// 初始化peer
	for _, reactor := range sw.reactors {
		p = reactor.InitPeer(p)
	}

	// 启动对等体的 send/recv routine
	// 必须在将其添加到对等集之前启动它，以防止同时调用 Start 和 Stop。
	err := p.Start()
	if err != nil {
		// Should never happen
		sw.Logger.Warnw("Error starting Peer", "err", err, "Peer", p)
		return err
	}

	// 将 Peer 添加到 peerSet 中，在启动反应堆之前这样做，以便如果产生错误，
	// 我们将找到对等体并移除它，.Add 应该不会出错，因为我们已经检查了 peers.HasConn()
	if err := sw.peers.Add(p); err != nil {
		return err
	}

	// 启动对等体的所有 Reactor
	for _, reactor := range sw.reactors {
		reactor.AddPeer(p)
	}

	sw.Logger.Infow("Added Peer", "Peer", p)

	return nil
}
