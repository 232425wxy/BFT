package gossip

import (
	"BFT/libs/cmap"
	"BFT/libs/log"
	"BFT/libs/service"
	"fmt"
	"net"
	"sync"
)

// peerConn 包含底层连接 net.Conn,以及一些参数：
//		outbound：表明该 Peer 是否是出站节点
//		persistent： 表明该 Peer 是否是 persistent Peer
//		socketAddr：*NetAddress，如果该 Peer 是出站的，则 socketAddr 是我们用来给该 Peer 拨号的地址
//		ip：Peer 的 IP 地址
type peerConn struct {
	outbound   bool
	persistent bool
	conn       net.Conn // source connection

	socketAddr *NetAddress

	// cached RemoteIP()
	ip net.IP
}

func newPeerConn(outbound, persistent bool, conn net.Conn, socketAddr *NetAddress) peerConn {

	return peerConn{
		outbound:   outbound,
		persistent: persistent,
		conn:       conn,
		socketAddr: socketAddr,
	}
}

// ID 返回 Peer 的 ID
// 注意：只有当 Peer 的底层连接是 SecretConn 时才有 ID，不然调用该方法会 panic
func (pc peerConn) ID() ID {
	return PubKeyToID(pc.conn.(*SecretConn).RemotePubKey())
}

// RemoteIP 返回 Peer 的 IP 地址，只返回一个
func (pc peerConn) RemoteIP() net.IP {
	if pc.ip != nil {
		return pc.ip
	}

	host, _, err := net.SplitHostPort(pc.conn.RemoteAddr().String())
	if err != nil {
		panic(err)
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		panic(err)
	}

	pc.ip = ips[0]

	return pc.ip
}

// Peer 实现 Peer 接口，
// 在使用 Peer 之前，需要先建立连接。
type Peer struct {
	service.BaseService

	// 底层连接
	peerConn
	mconn *MultiConn

	// Peer 知道的节点信息和 Channel，这里的 channels 与 NodeInfo.Channels 相等
	nodeInfo *NodeInfo
	channels []byte

	// 用户数据
	Data *cmap.CMap
}

type PeerOption func(*Peer)

func newPeer(
	pc peerConn,
	mConfig MultiConnConfig,
	nodeInfo *NodeInfo,
	reactorsByCh map[byte]Reactor,
	chDescs []*ChannelDescriptor,
	onPeerError func(*Peer, interface{}),
	options ...PeerOption,
) *Peer {
	p := &Peer{
		peerConn:      pc,
		nodeInfo:      nodeInfo,
		channels:      nodeInfo.Channels,
		Data:          cmap.NewCMap(),
	}

	// 将 peerConn 里的 conn 包装成 MultiConn
	p.mconn = createMultiConn(
		pc.conn,
		p,
		reactorsByCh,
		chDescs,
		onPeerError,
		mConfig,
	)
	p.BaseService = *service.NewBaseService(nil, "Peer", p)
	for _, option := range options {
		option(p)
	}

	return p
}

// String 入站 Peer 与 出站 Peer 的返回值不一样
func (p *Peer) String() string {
	if p.outbound {
		return fmt.Sprintf("Peer{%v %v out}", p.mconn, p.ID())
	}

	return fmt.Sprintf("Peer{%v %v in}", p.mconn, p.ID())
}

//---------------------------------------------------
// Implements service.Service

// SetLogger implements BaseService.
func (p *Peer) SetLogger(l log.CRLogger) {
	p.Logger = l
	p.mconn.SetLogger(l)
}

// OnStart implements BaseService.
func (p *Peer) OnStart() error {
	if err := p.BaseService.OnStart(); err != nil {
		return err
	}

	if err := p.mconn.Start(); err != nil {
		return err
	}

	return nil
}

// FlushStop 将 Peer 的底层连接 MultiConn 中还未发送出去的数据发送出去，然后在关闭 Peer
func (p *Peer) FlushStop() {
	p.BaseService.OnStop()
	p.mconn.FlushStop() // stop everything and close the conn
}

// OnStop 实现 service.Service 接口，
// 会调用 Peer 的底层连接 MultiConn 的 OnStop 方法。
func (p *Peer) OnStop() {
	p.BaseService.OnStop()
	if err := p.mconn.Stop(); err != nil { // stop everything and close the conn
		p.Logger.Debugw("Error while stopping Peer", "err", err)
	}
}

//---------------------------------------------------
// Implements Peer

// ID returns the Peer's ID - the hex encoded hash of its pubkey.
func (p *Peer) ID() ID {
	return p.nodeInfo.ID()
}

// IsOutbound 为 true 的话，表示该 Peer 是我们主动拨号要求建立连接的，即该 Peer 是出站节点
func (p *Peer) IsOutbound() bool {
	return p.peerConn.outbound
}

// IsPersistent returns true if the Peer is persitent, false otherwise.
func (p *Peer) IsPersistent() bool {
	return p.peerConn.persistent
}

// NodeInfo returns a copy of the Peer's NodeInfo.
func (p *Peer) NodeInfo() *NodeInfo {
	return p.nodeInfo
}

// SocketAddr 返回套接字的地址。
// 对于出站对等点，它是所拨打的地址(在 DNS 解析后)；
// 对于入站对等点，它是由底层连接返回的地址；
// (不是在 Peer 的 NodeInfo 中报告的内容)。
func (p *Peer) SocketAddr() *NetAddress {
	return p.peerConn.socketAddr
}

// Status returns the Peer's ConnectionStatus.
func (p *Peer) Status() ConnectionStatus {
	return p.mconn.Status()
}

// Send 发送 msg 到由 chID 标识的 Channel 中。如果在 MultiConn 指定的
// 超时时间内没有将 msg 推送给 Channel 的发送队列，则返回false。
func (p *Peer) Send(chID byte, msgBytes []byte) bool {
	if !p.IsRunning() {
		return false
	} else if !p.hasChannel(chID) {
		return false
	}
	res := p.mconn.Send(chID, msgBytes)
	return res
}

// TrySend 发送 msg 到由 chID 标识的 Channel 的发送队列中，如果发送队列已满，立即返回false
func (p *Peer) TrySend(chID byte, msgBytes []byte) bool {
	if !p.IsRunning() {
		return false
	} else if !p.hasChannel(chID) {
		return false
	}
	res := p.mconn.TrySend(chID, msgBytes)
	return res
}

// Get 根据给定的 key，从 .Data 中查找对应的值并返回
func (p *Peer) Get(key string) interface{} {
	return p.Data.Get(key)
}

// Set 往 .Data 中添加新的键值对
func (p *Peer) Set(key string, data interface{}) {
	p.Data.Set(key, data)
}

// hasChannel 判断 Peer 是否知道给定的 Channel ID
func (p *Peer) hasChannel(chID byte) bool {
	for _, ch := range p.channels {
		if ch == chID {
			return true
		}
	}
	// NOTE: probably will want to remove this
	// but could be helpful while the feature is new
	p.Logger.Debugw(
		"Unknown channel for Peer",
		"channel",
		chID,
		"channels",
		p.channels,
	)
	return false
}

// CloseConn 关闭底层的 net.Conn 连接，用于在对等端根本没有启动的情况下进行清理
func (p *Peer) CloseConn() error {
	return p.peerConn.conn.Close()
}


// RemoteAddr returns Peer's remote network address.
func (p *Peer) RemoteAddr() net.Addr {
	return p.peerConn.conn.RemoteAddr()
}

// CanSend Peer 通过其掌握的 MultiConn 判断指定 ID 的
// Channel 的发送队列是否满了，满了则返回 false，反之返回 true
func (p *Peer) CanSend(chID byte) bool {
	if !p.IsRunning() {
		return false
	}
	return p.mconn.CanSend(chID)
}

//------------------------------------------------------------------
// helper funcs

func createMultiConn(conn net.Conn, p *Peer, reactorsByCh map[byte]Reactor, chDescs []*ChannelDescriptor, onPeerError func(*Peer, interface{}), config MultiConnConfig) *MultiConn {

	onReceive := func(chID byte, msgBytes []byte) {
		reactor := reactorsByCh[chID]
		if reactor == nil {
			// 注意，这里可以 panic，因为它在 MultiConn._recover 中被捕获，它会执行 onPeerError
			panic(fmt.Sprintf("Unknown channel %X", chID))
		}
		// 解决了一个疑惑：MultiConn 收到消息后交由对应的 Reactor 处理
		reactor.Receive(chID, p, msgBytes)
	}

	onError := func(r interface{}) {
		onPeerError(p, r)
	}

	return NewMultiConnWithConfig(
		conn,
		chDescs,
		onReceive,
		onError,
		config,
	)
}


//-----------------------------------------------------------------------------

// PeerSet 是用来保存多个 Peer 的特殊结构。peers 上的迭代速度非常快，而且线程安全。
type PeerSet struct {
	mtx    sync.Mutex
	lookup map[ID]*peerSetItem
	list   []*Peer
}

type peerSetItem struct {
	peer  *Peer
	index int
}

// NewPeerSet 创建一个新的 peerSet，初始容量为 256 个条目。
func NewPeerSet() *PeerSet {
	return &PeerSet{
		lookup: make(map[ID]*peerSetItem),
		list:   make([]*Peer, 0, 256),
	}
}

// Add 将 Peer 添加到 PeerSet 中。
// 如果对等体已经存在，它将返回一个带有原因的错误。
func (ps *PeerSet) Add(peer *Peer) error {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.lookup[peer.ID()] != nil {
		return ErrSwitchDuplicatePeerID{peer.ID()}
	}

	index := len(ps.list)
	// Appending is safe even with other goroutines
	// iterating over the ps.list slice.
	ps.list = append(ps.list, peer)
	ps.lookup[peer.ID()] = &peerSetItem{peer, index}
	return nil
}

// HasID 如果 PeerSet 包含这个 id 指向的 Peer 则返回 true ，否则返回 false
func (ps *PeerSet) HasID(peerKey ID) bool {
	ps.mtx.Lock()
	_, ok := ps.lookup[peerKey]
	ps.mtx.Unlock()
	return ok
}

// HasIP 如果集合中包含该 IP 地址所指向的 Peer，HasIP 返回 true，否则返回 false
func (ps *PeerSet) HasIP(peerIP net.IP) bool {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	return ps.hasIP(peerIP)
}

// hasIP 没有获取锁，所以它可以在已经锁定的公共方法中使用。
func (ps *PeerSet) hasIP(peerIP net.IP) bool {
	for _, item := range ps.lookup {
		if item.peer.RemoteIP().Equal(peerIP) {
			return true
		}
	}

	return false
}

// Get 通过提供的 id 查找 Peer，如果没有找到 Peer 则返回 nil
func (ps *PeerSet) Get(id ID) *Peer {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	item, ok := ps.lookup[id]
	if ok {
		return item.peer
	}
	return nil
}

// Remove 通过 Peer 的 id 删除 Peer；
// 如果对等节点被删除，则返回 true，如果在集合中未找到该对等节点，则返回 false。
func (ps *PeerSet) Remove(peer *Peer) bool {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	item := ps.lookup[peer.ID()]
	if item == nil {
		return false
	}

	index := item.index
	// 创建一个列表的新副本，但要少一项。(我们必须复制，因为我们将改变列表)。
	newList := make([]*Peer, len(ps.list)-1)
	copy(newList, ps.list)
	// 如果是最后一个 Peer，这是一个简单的特例。
	if index == len(ps.list)-1 {
		ps.list = newList
		delete(ps.lookup, peer.ID())
		return true
	}

	// 将弹出的 Peer 替换为旧列表中的最后一项
	lastPeer := ps.list[len(ps.list)-1]
	lastPeerKey := lastPeer.ID()
	lastPeerItem := ps.lookup[lastPeerKey]
	newList[index] = lastPeer
	lastPeerItem.index = index
	ps.list = newList
	delete(ps.lookup, peer.ID())
	return true
}

// Size 返回 PeerSet 含有的 Peer 的数量
func (ps *PeerSet) Size() int {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return len(ps.list)
}

// List 可在线程安全的基础上返回 Peer 列表
func (ps *PeerSet) List() []*Peer {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return ps.list
}
