package gossip

import (
	srlog "BFT/libs/log"
	protogossip "BFT/proto/gossip"
	"context"
	"fmt"
	"net"
	"time"

	"golang.org/x/net/netutil"

	"BFT/crypto"
	"BFT/libs/protoio"
)

const (
	defaultDialTimeout      = time.Second     // 拨号超时时间
	defaultFilterTimeout    = 5 * time.Second // 过滤超时时间
	defaultHandshakeTimeout = 3 * time.Second // 握手超时时间
)

// IPResolver is a behaviour subset of net.Resolver.
type IPResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

// accept 是容器，用于将升级后的连接和 NodeInfo 从异步运行的 routine 传递到 Accept 方法
type accept struct {
	netAddr  *NetAddress
	conn     net.Conn
	nodeInfo *NodeInfo
	err      error
}

// peerConfig 用于捆绑数据，我们需要用 MultiConn 完全设置一个 Peer，
// 由 Accept 和 Dial 的调用者(目前是 Switch)提供，这是一个临时措施，
// 直到 Reactor 设置不那么动态，我们引入 PeerBehaviour 的概念来交流重要的 Peer 生命周期事件。
type peerConfig struct {
	chDescs     []*ChannelDescriptor
	onPeerError func(*Peer, interface{})
	outbound    bool
	// isPersistent 允许您设置一个功能，给定套接字地址 NetAddress (对于出站对等节点)
	// 或自报告地址(对于入站对等节点)，告诉该对等节点是否是持久的。
	isPersistent func(*NetAddress) bool
	reactorsByCh map[byte]Reactor
}

// transportLifecycle 为调用者绑定了控制启动和停止行为的方法
type transportLifecycle interface {
	Close() error
	Listen(NetAddress) error
}

// ConnFilterFunc 在建立新连接后由过滤器钩子实现。现有连接集与新连接的所有解析ip一起传递。
// net.Conn 是新连接，[]net.IP 是新连接对等节点的 IP 地址集
type ConnFilterFunc func(*ConnSet, net.Conn, []net.IP) error

// ConnDuplicateIPFilter 解析并保留所有进入连接的ip，并拒绝来自已知ip的新ip。
func ConnDuplicateIPFilter() ConnFilterFunc {
	return func(cs *ConnSet, c net.Conn, ips []net.IP) error {
		for _, ip := range ips {
			if cs.HasIP(ip) {
				return ErrRejected{
					conn:        c,
					err:         fmt.Errorf("ip<%v> already connected", ip),
					isDuplicate: true,
				}
			}
		}

		return nil
	}
}

// TransportOption 设置 Transport 的可选参数
type TransportOption func(*Transport)

// TransportConnFilters 设置拒绝特定新连接的过滤器
func TransportConnFilters(filters ...ConnFilterFunc) TransportOption {
	return func(mt *Transport) { mt.connFilters = filters }
}

// TransportMaxIncomingConnections 设置同时连接(传入)的最大数量。默认值:0(无限)
func TransportMaxIncomingConnections(n int) TransportOption {
	return func(mt *Transport) { mt.maxIncomingConnections = n }
}

// Transport 接受和拨号 tcp 连接，并将它们升级到多路对等节点
type Transport struct {
	netAddr                NetAddress
	listener               net.Listener
	maxIncomingConnections int

	acceptc chan accept
	closec  chan struct{}

	// 用于查找重复 ip 和 id 检查
	conns       *ConnSet
	connFilters []ConnFilterFunc

	dialTimeout      time.Duration
	filterTimeout    time.Duration
	handshakeTimeout time.Duration
	nodeInfo         NodeInfo
	nodeKey          NodeKey
	resolver         IPResolver

	// 当我们当前参数化peerConn和peer时，仍然需要这个配置。所有相关配置都应该重构为具有合理默认值的选项
	mConfig MultiConnConfig
	srlog.CRLogger
}

// 测试 multiplexTransport 是否实现了 Transport 接口和 transportLifecycle 接口
var _ transportLifecycle = (*Transport)(nil)

// NewTransport 返回一个tcp连接的多路对等体
func NewTransport(
	nodeInfo NodeInfo,
	nodeKey NodeKey,
	mConfig MultiConnConfig,
) *Transport {
	return &Transport{
		acceptc:          make(chan accept),
		closec:           make(chan struct{}),
		dialTimeout:      defaultDialTimeout,
		filterTimeout:    defaultFilterTimeout,
		handshakeTimeout: defaultHandshakeTimeout,
		mConfig:          mConfig,
		nodeInfo:         nodeInfo,
		nodeKey:          nodeKey,
		conns:            NewConnSet(),
		resolver:         net.DefaultResolver,
		CRLogger:         srlog.NewCRLogger("info"),
	}
}

// NetAddress 实现 Transport 接口，返回自身的 NetAddress
func (mt *Transport) NetAddress() NetAddress {
	return mt.netAddr
}

// Accept 实现 Transport 接口，从 Transport.acceptc 中获取新的底层连接 net.Conn
// 以该连接对等端的 nodeInfo 和 netAddr，然后将该新的连接包装成 Peer 并返回
// Accept 会在 Switch 的 acceptRoutine 里被持续循环调用
func (mt *Transport) Accept(cfg peerConfig) (*Peer, error) {
	select {
	// 这种情况下不应该有任何阻塞操作，以确保有质量的对等连接可以被使用，
	// 感觉主要原因还在于 acceptc 是无缓冲的
	case a := <-mt.acceptc:
		if a.err != nil {
			return nil, a.err
		}

		cfg.outbound = false

		return mt.wrapPeer(a.conn, a.nodeInfo, cfg, a.netAddr), nil
	case <-mt.closec:
		return nil, ErrTransportClosed{}
	}
}

// Dial 实现 Transport 接口，通过给给定节点的地址拨号，与其建立新的连接，并将该连接升级为加密连接，然后包装成 Peer 并返回
func (mt *Transport) Dial(addr NetAddress, cfg peerConfig) (*Peer, error) {
	c, err := addr.DialTimeout(mt.dialTimeout)
	if err != nil {
		return nil, err
	}

	if err := mt.filterConn(c); err != nil {
		return nil, err
	}

	secretConn, nodeInfo, err := mt.upgrade(c, &addr)
	if err != nil {
		return nil, err
	}

	cfg.outbound = true

	p := mt.wrapPeer(secretConn, nodeInfo, cfg, &addr)

	return p, nil
}

// Close 实现 transportLifecycle 接口，如果正在监听某个地址，那么就停止监听
func (mt *Transport) Close() error {
	close(mt.closec)

	if mt.listener != nil {
		return mt.listener.Close()
	}

	return nil
}

// Listen 实现 transportLifecycle 接口，开始监听指定地址，如果 .maxIncomingConnections > 0
// 则得到的侦听器最多接受来自提供的侦听器的 .maxIncomingConnections 个同时连接
func (mt *Transport) Listen(addr NetAddress) error {
	ln, err := net.Listen("tcp", addr.DialString())

	if err != nil {
		return err
	}

	mt.Infow(fmt.Sprintf("p2p starting listen on %s", addr.DialString()))

	if mt.maxIncomingConnections > 0 {
		ln = netutil.LimitListener(ln, mt.maxIncomingConnections)
	}

	mt.netAddr = addr
	mt.listener = ln

	go mt.acceptPeers()

	return nil
}

// AddChannel 向nodeInfo注册一个通道。
// 注意:NodeInfo 必须是 NodeInfo 类型，否则 Transport 的 Channels 将不会被更新
func (mt *Transport) AddChannel(chID byte) {
	if !mt.nodeInfo.HasChannel(chID) {
		mt.nodeInfo.Channels = append(mt.nodeInfo.Channels, chID)
	}
}

// acceptPeers 被作为一个 routine 持续运行
func (mt *Transport) acceptPeers() {
	for {
		// 通过网络侦听器获取一个新的连接
		c, err := mt.listener.Accept()
		if err != nil {
			// 在有错误发生的情况下，如果 Transport 被关闭了，就退出 acceptPeers routine
			select {
			case _, ok := <-mt.closec:
				if !ok {
					return
				}
			default:

			}
			// 在有错误发生且 Transport 没有被关闭的情况下，将该错误包装到 accept 容器里，然后被 .Accept 方法接收处理
			mt.acceptc <- accept{err: err}
			return
		}

		// 连接升级和过滤应该是异步的，以避免阻塞
		go func(c net.Conn) {
			defer func() {
				if r := recover(); r != nil {
					err := ErrRejected{
						conn:          c,
						err:           fmt.Errorf("recovered from panic: %v", r),
						isAuthFailure: true,
					}
					select {
					case mt.acceptc <- accept{err: err}:
					case <-mt.closec:
						// 在 panic 的情况下，如果 Transport 已经被关闭了，就放弃新连接
						_ = c.Close()
						return
					}
				}
			}()

			var (
				nodeInfo   *NodeInfo
				secretConn *SecretConn
				netAddr    *NetAddress
			)

			err := mt.filterConn(c) // 对新连接进行过滤
			if err == nil {
				// 将新连接升级为加密连接
				secretConn, nodeInfo, err = mt.upgrade(c, nil)
				if err == nil {
					// 在有了加密连接后，就可以构建该加密连接的 NetAddress 了，注意 NetAddress 是对等端的
					addr := c.RemoteAddr()
					id := PubKeyToID(secretConn.RemotePubKey())
					netAddr = NewNetAddress(id, addr)
				}
			}

			select {
			case mt.acceptc <- accept{netAddr, secretConn, nodeInfo, err}:
				// 是升级后的连接可用
			case <-mt.closec:
				// 如果 Transport 已经被关闭了，就放弃新连接
				_ = c.Close()
				return
			}
		}(c)
	}
}

// Cleanup 从连接集中根据给定的 Peer 对应的地址删除 connSetItem 并关闭连接
func (mt *Transport) Cleanup(p *Peer) {
	mt.conns.RemoveAddr(p.RemoteAddr())
	_ = p.CloseConn()
}

// cleanup 从连接集中根据给定的连接对象，删除对应的 connSetItem
func (mt *Transport) cleanup(c net.Conn) error {
	mt.conns.RemoveConn(c)

	return c.Close()
}

// filterConn 筛选连接，需要进行的步骤如下：
// 	1. 判断连接集中是否已存在该连接，判断依据是连接的远程地址
// 	2. 根据 Transport 的 IP 地址解析器和连接自身，解析出它的 IPs
// 	3. 利用 Transport.connFilters 过滤器，根据已有连接集、
//	   新连接自身以及新连接对应的 IPs，对新连接进行过滤
// 	4. 一旦有过滤器在过滤的过程中发生了错误，就会停止其他过滤器的过滤操作，
// 	   然后直接返回错误，当过滤器接受了新连接，就将新连接和它对应的 IP 地址放入连接集中
// 	注意：过滤操作有超时时间限制，在超时时间内没有完成过滤操作的话，会返回过滤超时的错误
func (mt *Transport) filterConn(c net.Conn) (err error) {
	defer func() {
		if err != nil {
			_ = c.Close()
		}
	}()

	// 如果新连接已经存在于连接集中，就会拒绝该新连接
	if mt.conns.HasConn(c) {
		return ErrRejected{conn: c, isDuplicate: true}
	}

	// 去 DNS 中，为到来的新连接解析 IP 地址
	ips, err := resolveIPs(mt.resolver, c)
	if err != nil {
		return err
	}

	errc := make(chan error, len(mt.connFilters))

	for _, f := range mt.connFilters {
		go func(f ConnFilterFunc, c net.Conn, ips []net.IP, errc chan<- error) {
			errc <- f(mt.conns, c, ips)
		}(f, c, ips, errc)
	}

	for i := 0; i < cap(errc); i++ {
		select {
		case err := <-errc:
			if err != nil {
				return ErrRejected{conn: c, err: err, isFiltered: true}
			}
		case <-time.After(mt.filterTimeout):
			return ErrFilterTimeout{}
		}

	}

	mt.conns.Set(c, ips)

	return nil
}

// upgrade 将普通连接升级为加密连接，需要经过的步骤如下：
//	1. 调用 upgradeSecretConn 方法将普通连接升级为加密连接，如果出错，就返回错误
//	2. 通过加密连接，获取对等端的 ID，并且如果对等端的地址不为空的话，会比较地址里的 ID 与刚刚获取到的 ID 是否相等，不相等的话就返回错误
//	3. 通过 handshake 方法与对等端交换 nodeInfo，如果发生错误，则返回错误
//	4. 验证对等端的 nodeInfo 的正确性，如果验证不通过，则返回错误
//	5. 验证通过加密连接获得的对等端的 ID 与对等端 nodeInfo 中自报告的 ID 是否相等，如果不相等则返回错误
//	6. 判断自己的 ID 与对等端 nodeInfo 中的 ID 是否相等，相等的话表示自己和自己建立连接，那么就没必要了，所以也返回一个错误
//	7. 判断自己的 nodeInfo 是否与对等端的 nodeInfo 是否兼容
// 以上 7 步都通过后，则返回加密连接和对等端的 nodeInfo
func (mt *Transport) upgrade(c net.Conn, dialedAddr *NetAddress) (secretConn *SecretConn, nodeInfo *NodeInfo, err error) {
	defer func() {
		if err != nil {
			_ = mt.cleanup(c)
		}
	}()

	secretConn, err = upgradeSecretConn(c, mt.handshakeTimeout, mt.nodeKey.PrivKey)
	if err != nil {
		return nil, nil, ErrRejected{
			conn:          c,
			err:           fmt.Errorf("secret conn failed: %v", err),
			isAuthFailure: true,
		}
	}

	// For outgoing conns, ensure connection key matches dialed key.
	connID := PubKeyToID(secretConn.RemotePubKey())
	if dialedAddr != nil {
		if dialedID := dialedAddr.ID; connID != dialedID {
			return nil, nil, ErrRejected{
				conn: c,
				id:   connID,
				err: fmt.Errorf(
					"conn.ID (%v) dialed ID (%v) mismatch",
					connID,
					dialedID,
				),
				isAuthFailure: true,
			}
		}
	}

	nodeInfo, err = handshake(secretConn, mt.handshakeTimeout, mt.nodeInfo)
	if err != nil {
		return nil, nil, ErrRejected{
			conn:          c,
			err:           fmt.Errorf("handshake failed: %v", err),
			isAuthFailure: true,
		}
	}

	if err := nodeInfo.Validate(); err != nil {
		return nil, nil, ErrRejected{
			conn:              c,
			err:               err,
			isNodeInfoInvalid: true,
		}
	}

	// 确保连接的节点 ID 与节点自身报告的 ID 相等
	if connID != nodeInfo.ID() {
		return nil, nil, ErrRejected{
			conn: c,
			id:   connID,
			err: fmt.Errorf(
				"conn.ID (%v) NodeInfo.ID (%v) mismatch",
				connID,
				nodeInfo.ID(),
			),
			isAuthFailure: true,
		}
	}

	// Reject self.
	if mt.nodeInfo.ID() == nodeInfo.ID() {
		return nil, nil, ErrRejected{
			addr:   *NewNetAddress(nodeInfo.ID(), c.RemoteAddr()),
			conn:   c,
			id:     nodeInfo.ID(),
			isSelf: true,
		}
	}

	if err := mt.nodeInfo.CompatibleWith(nodeInfo); err != nil {
		return nil, nil, ErrRejected{
			conn:           c,
			err:            err,
			id:             nodeInfo.ID(),
			isIncompatible: true,
		}
	}

	return secretConn, nodeInfo, nil
}

// wrapPeer 利用对等端的 NodeInfo、peerConfig、NetAddress 将 net.Conn 包装成 Peer
func (mt *Transport) wrapPeer(c net.Conn, ni *NodeInfo, cfg peerConfig, socketAddr *NetAddress) *Peer {

	persistent := false
	if cfg.isPersistent != nil {
		if cfg.outbound {
			// 如果该连接是出站连接，那么由给对等节点拨号的地址就可以知道该连接是否是 persistent
			persistent = cfg.isPersistent(socketAddr)
		} else {
			// 如果是入站节点，那么由对等节点的自报告地址可以判断该连接是否是 persistent
			selfReportedAddr, err := ni.NetAddress()
			if err == nil {
				persistent = cfg.isPersistent(selfReportedAddr)
			}
		}
	}

	peerConn := newPeerConn(
		cfg.outbound,
		persistent,
		c,
		socketAddr,
	)

	p := newPeer(
		peerConn,
		mt.mConfig,
		ni,
		cfg.reactorsByCh,
		cfg.chDescs,
		cfg.onPeerError,
	)

	return p
}

// handshake 通过与对等节点握手，获取对等端的 nodeInfo，发送自己的 nodeInfo 和接收对等端的 nodeInfo 是同时进行的
func handshake(c net.Conn, timeout time.Duration, nodeInfo NodeInfo) (*NodeInfo, error) {
	if err := c.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}

	var (
		errc = make(chan error, 2)

		pbpeerNodeInfo protogossip.NodeInfo
		peerNodeInfo   NodeInfo
		ourNodeInfo    = nodeInfo
	)

	go func(errc chan<- error, c net.Conn) {
		_, err := protoio.NewDelimitedWriter(c).WriteMsg(ourNodeInfo.ToProto())
		errc <- err
	}(errc, c)
	go func(errc chan<- error, c net.Conn) {
		protoReader := protoio.NewDelimitedReader(c, MaxNodeInfoSize())
		_, err := protoReader.ReadMsg(&pbpeerNodeInfo)
		errc <- err
	}(errc, c)

	for i := 0; i < cap(errc); i++ {
		err := <-errc
		if err != nil {
			return nil, err
		}
	}

	peerNodeInfo, err := NodeInfoFromProto(&pbpeerNodeInfo)
	if err != nil {
		return nil, err
	}

	return &peerNodeInfo, c.SetDeadline(time.Time{})
}

// upgradeSecretConn 将普通连接升级为加密连接
func upgradeSecretConn(c net.Conn, timeout time.Duration, privKey crypto.PrivKey) (*SecretConn, error) {
	if err := c.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}

	sc, err := MakeSecretConn(c, privKey)
	if err != nil {
		return nil, err
	}

	return sc, sc.SetDeadline(time.Time{})
}

// resolveIPs 根据连接的 host 从 DNS 中解析出 IP 地址
func resolveIPs(resolver IPResolver, c net.Conn) ([]net.IP, error) {
	host, _, err := net.SplitHostPort(c.RemoteAddr().String())
	if err != nil {
		return nil, err
	}

	addrs, err := resolver.LookupIPAddr(context.Background(), host)
	if err != nil {
		return nil, err
	}

	ips := []net.IP{}

	for _, addr := range addrs {
		ips = append(ips, addr.IP)
	}

	return ips, nil
}
