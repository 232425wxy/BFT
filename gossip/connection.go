package gossip

import (
	protogossip "github.com/232425wxy/BFT/proto/gossip"
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"reflect"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gogo/protobuf/proto"

	"github.com/232425wxy/BFT/libs/log"
	srmath "github.com/232425wxy/BFT/libs/math"
	"github.com/232425wxy/BFT/libs/protoio"
	"github.com/232425wxy/BFT/libs/service"
	"github.com/232425wxy/BFT/libs/timer"
)

const (
	defaultMaxPacketMsgPayloadSize = 1024 // 默认的数据包的最大有效载荷：1024字节

	numBatchPacketMsgs = 10              // 默认一次最多发送给 peer 的数据包个数：10个包
	minReadBufferSize  = 102400            // 读缓冲区的大小
	minWriteBufferSize = 655360           // 写缓冲区的大小
	updateStats        = 2 * time.Second // 更新一个 channel 的 recentlySent 的时间间隔

	// 其中一些默认值写入用户配置flushThrottle, sendRate, recvRate
	defaultFlushThrottle = 100 * time.Millisecond

	defaultSendQueueCapacity   = 1             // 默认发送队列的容量
	defaultRecvBufferCapacity  = 4096000          // 默认接收缓冲区的大小：4KB
	defaultRecvMessageCapacity = 22020096      // 默认一次接收数据的最大大小：21MB
	// 默认发送数据的超时时间，当有数据需要发送出去的时候，会先将数据推入到
	// channel 的发送队列里，但是如果 channel 的发送队列满了，会等待一段
	// 时间，但是如果在超时时间内还没有将数据推入到 channel 的发送队列中，就会返回 false，表示发送失败
	defaultSendTimeout  = 10 * time.Second
	defaultPingInterval = 60 * time.Second // 默认给 peer 发送 Ping 的时间间隔
	defaultPongTimeout  = 45 * time.Second // 默认从 peer 处接收 Pong 的超时时间
)

type receiveCbFunc func(chID byte, msgBytes []byte)
type errorCbFunc func(interface{})

// MultiConn 多路连接
// 每个 peer 都有一个 “MultiConn” (多路连接)实例。
// 每个 “MultiConn” 处理多个抽象通信 “Channel” 上的消息传输。每个通道都有一个全局唯一的 id。
// 每个 “Channel” 的 id 和相对优先级在连接初始化时配置。
// 有两种发送消息的方法:
// 		func (m MultiConn) Send(chID byte, msgBytes []byte) bool {}
//		func (m MultiConn) TrySend(chID byte, msgBytes []byte}) bool {}
// “Send(chID, msgBytes)“ 是一个阻塞调用，它会等待直到 “msg” 成功排队到 “chID” 的
// channel 里，或者直到请求超时，消息 “msg” 是使用 Protobuf 序列化的
// “TrySend(chID, msgBytes)“ 是一个非阻塞调用，如果 id 为 chID 的 channel 的发送队列已满则返回 false。
// 入站消息由 onReceive 回调函数处理
type MultiConn struct {
	service.BaseService

	conn          net.Conn
	bufConnReader *bufio.Reader
	bufConnWriter *bufio.Writer
	// 当 sendSignal 收到一个空结构体，就会激活sendRoutine里的发送PacketMsg的功能
	sendSignal chan struct{}
	pong       chan struct{}
	// 每个 MultiConn 都与若干个 Channel 相关联，这些 Channel 是 MultiConn 与consensus、
	// blockchain、mempool 之间进行消息传递的桥梁
	channels []*Channel
	// 一个以 chID 为键 *Channel 为值的 map
	channelsIdx map[byte]*Channel
	// 对于收到的消息，我们想要对这个消息作更进一步的处理，可以调用 onRecvCbFunc，
	// 比如 chID 对应 consensus 模块，则可将该消息交给 consensus 模块去处理
	onReceive receiveCbFunc
	onError   errorCbFunc
	errored   uint32
	config    MultiConnConfig

	// Close quitSendRoutine 会导致 sendRoutine 协程退出.
	// 当 sendRoutine 退出时，doneSendRoutine 会 close
	quitSendRoutine chan struct{}
	doneSendRoutine chan struct{}

	// close quitRecvRoutine 将导致 recvRoutine 退出
	quitRecvRoutine chan struct{}

	stopMtx sync.Mutex

	flushTimer *timer.ThrottleTimer // 定期地将 bufConnWriter 缓冲区里的数据刷新到 conn 中
	pingTicker *time.Ticker         // 定时向 peer 发送 Ping 消息

	// 发送出 Ping 消息后，就会开启 pongTimer 计时，期望能在计时结束前收到 Pong 消息
	pongTimer *time.Timer
	// 接收到 Pong 消息就往 pongTimeoutCh 里推送 false，否则在 pongTimeout 超时以后，就往里面推送 true
	pongTimeoutCh chan bool

	// updateStatsTicker 用来定时更新每个 Channel 里的 recentlySent 值
	updateStatsTicker *time.Ticker // update channel stats periodically

	created time.Time // 创建时间

	// _maxPacketMsgSize 数据包的最大大小
	_maxPacketMsgSize int
}

// ConstructMultiConnConfig is a MultiConn configuration.
type MultiConnConfig struct {
	// 数据包的最大有效载荷
	MaxPacketMsgPayloadSize int `mapstructure:"max_packet_msg_payload_size"`

	// 将数据刷新到底层连接 conn 的缓冲池里时间间隔
	FlushThrottle time.Duration `mapstructure:"flush_throttle"`

	// 给 peer 发送 Ping 的时间间隔
	PingInterval time.Duration `mapstructure:"ping_interval"`

	// 从 peer 处等待接收 Pong 的超时时间
	PongTimeout time.Duration `mapstructure:"pong_timeout"`
}

// DefaultMultiConnConfig 返回默认配置选项
// 		SendRate:                500KB/s
//		RecvRate:                500KB/s
//		MaxPacketMsgPayloadSize: 1KB
//		FlushThrottle:           100ms
//		PingInterval:            60s
//		PongTimeout:             45s
func DefaultMultiConnConfig() MultiConnConfig {
	return MultiConnConfig{
		MaxPacketMsgPayloadSize: defaultMaxPacketMsgPayloadSize,
		FlushThrottle:           defaultFlushThrottle,
		PingInterval:            defaultPingInterval,
		PongTimeout:             defaultPongTimeout,
	}
}

// NewMultiConnWithConfig wraps net.Conn and creates multiplex connection with a config
func NewMultiConnWithConfig(
	conn net.Conn,
	chDescs []*ChannelDescriptor,
	onReceive receiveCbFunc,
	onError errorCbFunc,
	config MultiConnConfig,
) *MultiConn {
	if config.PongTimeout >= config.PingInterval {
		panic("pongTimeout must be less than pingInterval (otherwise, next ping will reset pong timer)")
	}

	mconn := &MultiConn{
		conn:          conn,
		bufConnReader: bufio.NewReaderSize(conn, minReadBufferSize),
		bufConnWriter: bufio.NewWriterSize(conn, minWriteBufferSize),
		sendSignal:    make(chan struct{}, 1),
		pong:          make(chan struct{}, 1),
		onReceive:     onReceive,
		onError:       onError,
		config:        config,
		created:       time.Now(),
	}

	// Create channels
	var channelsIdx = map[byte]*Channel{}
	var channels = []*Channel{}

	for _, desc := range chDescs {
		channel := newChannel(mconn, *desc)
		channelsIdx[channel.desc.ID] = channel
		channels = append(channels, channel)
	}
	mconn.channels = channels
	mconn.channelsIdx = channelsIdx

	mconn.BaseService = *service.NewBaseService(nil, "MultiConn", mconn)

	mconn._maxPacketMsgSize = mconn.maxPacketMsgSize()

	return mconn
}

func (c *MultiConn) SetLogger(l log.CRLogger) {
	c.BaseService.SetLogger(l)
	for _, ch := range c.channels {
		ch.SetLogger(l)
	}
}

// OnStart implements BaseService
func (c *MultiConn) OnStart() error {
	if err := c.BaseService.OnStart(); err != nil {
		return err
	}
	c.flushTimer = timer.NewThrottleTimer("flush", c.config.FlushThrottle)
	c.pingTicker = time.NewTicker(c.config.PingInterval)
	c.pongTimeoutCh = make(chan bool, 1)
	c.updateStatsTicker = time.NewTicker(updateStats)
	c.quitSendRoutine = make(chan struct{})
	c.doneSendRoutine = make(chan struct{})
	c.quitRecvRoutine = make(chan struct{})
	go c.sendRoutine()
	go c.recvRoutine()
	return nil
}

// stopService 停止服务，包括：
// 		service.BaseService
// 		.flushTimer.Stop()
//		.pingTicker.Stop()
// 		.updateStatsTicker.Stop()
// 		close(.quitRecvRoutine)
// 		close(.quitSendRoutine)
func (c *MultiConn) stopServices() (alreadyStopped bool) {
	c.stopMtx.Lock()
	defer c.stopMtx.Unlock()

	select {
	case <-c.quitSendRoutine:
		// already quit
		return true
	default:
	}

	select {
	case <-c.quitRecvRoutine:
		// already quit
		return true
	default:
	}

	c.BaseService.OnStop()
	c.flushTimer.Stop()
	c.pingTicker.Stop()
	c.updateStatsTicker.Stop()

	// inform the recvRouting that we are shutting down
	close(c.quitRecvRoutine)
	close(c.quitSendRoutine)
	return false
}

// FlushStop 将剩余的等待被发送的数据发送出去后再关闭连接
func (c *MultiConn) FlushStop() {
	if c.stopServices() {
		return
	}

	// this block is unique to FlushStop
	{
		// 在这里会等待发送消息的 routine 退出
		<-c.doneSendRoutine

		// 将剩余的等待被发送的数据发送出去
		// 由于发送消息的 routine 已经退出了，所以这里可以直接调用 .sendSomePacketMsgs() 方法
		eof := c.sendSomePacketMsgs()
		for !eof {
			eof = c.sendSomePacketMsgs()
		}
		c.flush()

		// 所有消息发送完以后就可以关闭连接了
	}

	c.conn.Close()

	// 在这里还无法安全地关闭 pongSignal 通道，因为接收消息的 routine 可能接下来还会往里面写入信号
	// 我们只会在接收消息的 routine 里关闭 pongSignal 通道
}

// OnStop implements BaseService
func (c *MultiConn) OnStop() {
	if c.stopServices() {
		// 如果服务已经被关闭了，就直接返回
		return
	}

	c.conn.Close()

	// 在这里还无法安全地关闭 pongSignal 通道，因为接收消息的 routine 可能接下来还会往里面写入信号
	// 我们只会在接收消息的 routine 里关闭 pongSignal 通道
}

func (c *MultiConn) String() string {
	return fmt.Sprintf("MultiConn{%v}", c.conn.RemoteAddr())
}

// flush 将 MultiConn 中剩余的还未发送出去的数据发送出去
func (c *MultiConn) flush() {
	c.Logger.Debugw("Flush", "conn", c)
	// 将 bufConnWriter 缓冲区里的数据写入到底层的 conn 中，然后调用 net.Conn.Write() 通过网络发送出去
	err := c.bufConnWriter.Flush()
	if err != nil {
		c.Logger.Debugw("MultiConn flush failed", "err", err)
	}
}

// _recover 捕获 panic，panic 通常由于与 peer 断开连接造成的
func (c *MultiConn) _recover() {
	if r := recover(); r != nil {
		c.Logger.Errorw("MultiConn panicked", "err", r, "stack", string(debug.Stack()))
		c.stopForError(fmt.Errorf("recovered from panic: %v", r))
	}
}

// stopForError 由于某种错误而停止服务。
// 先调用 BaseService.Stop() 方法，然后调用 MultiConn.OnStop() 方法，
// 然后继续深入调用 MultiConn.stopServices() 方法。
func (c *MultiConn) stopForError(r interface{}) {
	if err := c.Stop(); err != nil {
		c.Logger.Errorw("Error stopping connection", "err", err)
	}
	if atomic.CompareAndSwapUint32(&c.errored, 0, 1) {
		if c.onError != nil {
			// 调用处理错误的回调函数处理错误
			c.onError(r)
		}
	}
}

// Send 将待发送消息推送到 ID 为 chID 的 Channel 的发送队列中，推送成功就返回 true，否则返回 false
// 如果 Channel 的发送队列满了，就会等待一段超时时间，如果在超时时间内，发送队列又不满了，就可推送成功，否
// 则超时以后还未推送成功，就返回 false
func (c *MultiConn) Send(chID byte, msgBytes []byte) bool {
	if !c.IsRunning() {
		return false
	}

	c.Logger.Debugw("Send", "channel", chID, "conn", c, "msgBytes", fmt.Sprintf("%X", msgBytes))

	// 将消息转交给 Channel
	channel, ok := c.channelsIdx[chID]
	if !ok {
		c.Logger.Errorw(fmt.Sprintf("Cannot sendSignal bytes, unknown channel %X", chID))
		return false
	}

	success := channel.sendBytes(msgBytes)
	if success {
		// 唤醒发送 routine，调用 .sendSomePacketMsgs() 方法发送数据
		select {
		case c.sendSignal <- struct{}{}:
		default:
		}
	} else {
		c.Logger.Debugw("Send failed", "channel", chID, "conn", c, "msgBytes", fmt.Sprintf("%X", msgBytes))
	}
	return success
}

// TrySend 尝试将待发送数据推送到 ID 为 chID 的 Channel 的发送队列里，如果发送队列满了，
// 则直接返回 false，表示推送失败，否则返回 true，表示推送成功。
func (c *MultiConn) TrySend(chID byte, msgBytes []byte) bool {
	if !c.IsRunning() {
		return false
	}

	c.Logger.Debugw("TrySend", "channel", chID, "conn", c, "msgBytes", fmt.Sprintf("%X", msgBytes))

	// 将消息转交给 Channel
	channel, ok := c.channelsIdx[chID]
	if !ok {
		c.Logger.Errorw(fmt.Sprintf("Cannot sendSignal bytes, unknown channel %X", chID))
		return false
	}

	ok = channel.trySendBytes(msgBytes)
	if ok {
		// 唤醒发送 routine，调用 .sendSomePacketMsgs() 方法发送数据
		select {
		case c.sendSignal <- struct{}{}:
		default:
		}
	}

	return ok
}

// CanSend 判断 ID 为 chID 的Channel 现在是否可以发送数据
func (c *MultiConn) CanSend(chID byte) bool {
	if !c.IsRunning() {
		return false
	}

	channel, ok := c.channelsIdx[chID]
	if !ok {
		c.Logger.Errorw(fmt.Sprintf("Unknown channel %X", chID))
		return false
	}
	return channel.canSend()
}

// sendRoutine 通过轮询发现需要发送的数据包，并发送出去
func (c *MultiConn) sendRoutine() {
	defer c._recover()

	protoWriter := protoio.NewDelimitedWriter(c.bufConnWriter)

FOR_LOOP:
	for {
		var err error
	SELECTION:
		select {
		case <-c.flushTimer.Ch:
			// flush 可将一些数据写入到 .bufConnWriter 中，然后调用 net.Conn.Write() 将数据发送出去
			// 每调用一次 flush()，都必须调用 .flushTimer.Set()
			c.flush()
		case <-c.updateStatsTicker.C:
			for _, channel := range c.channels {
				channel.updateStats()
			}
		case <-c.pingTicker.C:
			c.Logger.Debugw("Send Ping")
			_, err = protoWriter.WriteMsg(mustWrapPacket(&protogossip.PacketPing{}))
			if err != nil {
				c.Logger.Error("Failed to sendSignal PacketPing", "err", err)
				break SELECTION
			}
			c.Logger.Debugw("Starting pong timer", "dur", c.config.PongTimeout)
			c.pongTimer = time.AfterFunc(c.config.PongTimeout, func() {
				select {
				case c.pongTimeoutCh <- true:
				default:
				}
			})
			c.flush()
		case timeout := <-c.pongTimeoutCh:
			if timeout {
				c.Logger.Debugw("Pong timeout")
				err = errors.New("pong timeout")
			} else {
				c.stopPongTimer()
			}
		case <-c.pong:
			c.Logger.Debugw("Send Pong")
			_, err = protoWriter.WriteMsg(mustWrapPacket(&protogossip.PacketPong{}))
			if err != nil {
				c.Logger.Errorw("Failed to sendSignal PacketPong", "err", err)
				break SELECTION
			}
			c.flush()
		case <-c.quitSendRoutine:
			break FOR_LOOP
		case <-c.sendSignal:
			// 批量发送一些数据包
			eof := c.sendSomePacketMsgs()
			if !eof {
				// 只要数据没发送完，就保证发送数据的 routine 持续发送数据包
				select {
				case c.sendSignal <- struct{}{}:
				default:
				}
			}
		}

		if !c.IsRunning() {
			break FOR_LOOP
		}
		if err != nil {
			c.Logger.Errorw("Connection failed @ sendRoutine", "conn", c, "err", err)
			c.stopForError(err)
			break FOR_LOOP
		}
	}

	// Cleanup
	c.stopPongTimer()
	close(c.doneSendRoutine)
}

// sendSomePacketMsgs 如果所有 Channel 里的数据都发送出去了，则返回 true
func (c *MultiConn) sendSomePacketMsgs() bool {
	// 堵塞到 .sendMonitor 说我们可以发送数据了
	//c.sendMonitor.Limit(c._maxPacketMsgSize, atomic.LoadInt64(&c.config.SendRate), true)

	// 循环发送一批数据
	for i := 0; i < numBatchPacketMsgs; i++ { // 最多循环 10 次
		if c.sendPacketMsg() {
			return true
		}
	}
	return false
}

// sendPacketMsg 仅发送一个数据包，如果所有 Channel 里的数据都被发送出去了，就返回 true
func (c *MultiConn) sendPacketMsg() bool {
	// 选择一个 Channel 将它的数据打包成PacketMsg。
	// 被选择的 Channel 的 recentlySent/priority 必须是在所有 Channel 中最小的
	var leastRatio float32 = math.MaxFloat32
	var leastChannel *Channel
	for _, channel := range c.channels {
		// 如果该 Channel 没有数据要发送，就跳过该 Channel
		if !channel.isSendPending() {
			continue
		}
		// 获取比率，并跟踪最低比率。
		ratio := float32(channel.recentlySent) / float32(channel.desc.Priority)
		if ratio < leastRatio {
			leastRatio = ratio
			leastChannel = channel
		}
	}

	// 没有需要发送消息的 Channel，表示消息都发送完了
	if leastChannel == nil {
		return true
	}

	// 让被选中的 Channel 打包数据并发送出去
	_, err := leastChannel.writePacketMsgTo(c.bufConnWriter)
	if err != nil {
		c.Logger.Errorw("Failed to write PacketMsg", "err", err)
		c.stopForError(err)
		return true
	}
	c.flushTimer.Set()
	return false
}

// recvRoutine 读取 PacketMsgs 并使用 Channel 的 “recving” 缓冲区重建消息。
// 组装完整消息后，将其推入onReceive()。
// 块的大小取决于连接的节流方式。
func (c *MultiConn) recvRoutine() {
	defer c._recover()

	protoReader := protoio.NewDelimitedReader(c.bufConnReader, c._maxPacketMsgSize)

FOR_LOOP:
	for {
		// 阻塞到 recvMonitor 说我们可以继续接收更多的数据
		//c.recvMonitor.Limit(c._maxPacketMsgSize, atomic.LoadInt64(&c.config.RecvRate), true)

		// 解析数据包的类型
		var packet protogossip.Packet

		_, err := protoReader.ReadMsg(&packet)
		if err != nil {
			// stopServices 被调用，我们正在关闭
			// 接收将会失败，因为我们将关闭连接
			select {
			case <-c.quitRecvRoutine:
				break FOR_LOOP
			default:
			}

			if c.IsRunning() {
				if err == io.EOF {
					c.Logger.Infow("Connection is closed @ recvRoutine (likely by the other side)", "conn", c)
				} else {
					c.Logger.Debugw("Connection failed @ recvRoutine (reading byte)", "conn", c, "err", err)
				}
				c.stopForError(err)
			}
			break FOR_LOOP
		}

		// 根据数据包类型读取更多信息
		switch pkt := packet.Sum.(type) {
		case *protogossip.Packet_PacketPing:
			// 收到 Ping 消息以后，开始准备发送 Pong 消息出去
			c.Logger.Debugw("Receive Ping")
			select {
			case c.pong <- struct{}{}:
			default:

			}
		case *protogossip.Packet_PacketPong:
			c.Logger.Debugw("Receive Pong")
			select {
			// 在超时时间内收到 Pong 消息，说明超时计时已经失败了，我们不会超时啦，所以推送 false 进去
			case c.pongTimeoutCh <- false:
			default:

			}
		case *protogossip.Packet_PacketMsg:
			channelID := byte(pkt.PacketMsg.ChannelID)
			channel, ok := c.channelsIdx[channelID]
			if pkt.PacketMsg.ChannelID < 0 || pkt.PacketMsg.ChannelID > math.MaxUint8 || !ok || channel == nil {
				err := fmt.Errorf("unknown channel %X", pkt.PacketMsg.ChannelID)
				c.Logger.Debugw("Connection failed @ recvRoutine", "conn", c, "err", err)
				c.stopForError(err)
				break FOR_LOOP
			}

			msgBytes, err := channel.recvPacketMsg(*pkt.PacketMsg)
			if err != nil {
				if c.IsRunning() {
					c.Logger.Debugw("Connection failed @ recvRoutine", "conn", c, "err", err)
					c.stopForError(err)
				}
				break FOR_LOOP
			}
			if msgBytes != nil {
				c.Logger.Debugw("Received bytes", "chID", channelID, "msgBytes", msgBytes)
				// NOTE: 这里意味着 reactor.Receive() 与 p2p 的 recvRoutine 在同一线程中运行
				c.onReceive(channelID, msgBytes)
			}
		default:
			err := fmt.Errorf("unknown message type %v", reflect.TypeOf(packet))
			c.Logger.Errorw("Connection failed @ recvRoutine", "conn", c, "err", err)
			c.stopForError(err)
			break FOR_LOOP
		}
	}

	// Cleanup
	close(c.pong)
	for range c.pong {
		// Drain
	}
}

// stopPongTimer 关闭接收 Pong 的超时倒计时
func (c *MultiConn) stopPongTimer() {
	if c.pongTimer != nil {
		_ = c.pongTimer.Stop()
		c.pongTimer = nil
	}
}

// maxPacketMsgSize 返回数据包的最大大小，单位是字节
func (c *MultiConn) maxPacketMsgSize() int {
	bz, err := proto.Marshal(mustWrapPacket(&protogossip.PacketMsg{
		ChannelID: 0x01,
		EOF:       true,
		Data:      make([]byte, c.config.MaxPacketMsgPayloadSize),
	}))
	if err != nil {
		panic(err)
	}
	return len(bz)
}

type ConnectionStatus struct {
	Duration    time.Duration // 连接已经持续启动了多长时间
	Channels    []ChannelStatus
}

type ChannelStatus struct {
	ID                byte
	SendQueueCapacity int
	SendQueueSize     int
	Priority          int
	RecentlySent      int64
}

func (c *MultiConn) Status() ConnectionStatus {
	var status ConnectionStatus
	status.Duration = time.Since(c.created)
	status.Channels = make([]ChannelStatus, len(c.channels))
	for i, channel := range c.channels {
		status.Channels[i] = ChannelStatus{
			ID:                channel.desc.ID,
			SendQueueCapacity: cap(channel.sendQueue),
			SendQueueSize:     int(atomic.LoadInt32(&channel.sendQueueSize)),
			Priority:          channel.desc.Priority,
			RecentlySent:      atomic.LoadInt64(&channel.recentlySent),
		}
	}
	return status
}

//-----------------------------------------------------------------------------

type ChannelDescriptor struct {
	ID                  byte
	Priority            int
	SendQueueCapacity   int // 默认是 1
	RecvBufferCapacity  int // 默认是 4KB
	RecvMessageCapacity int // 默认是 21MB
}

func (chDesc ChannelDescriptor) FillDefaults() (filled ChannelDescriptor) {
	if chDesc.SendQueueCapacity == 0 {
		chDesc.SendQueueCapacity = defaultSendQueueCapacity
	}
	if chDesc.RecvBufferCapacity == 0 {
		chDesc.RecvBufferCapacity = defaultRecvBufferCapacity
	}
	if chDesc.RecvMessageCapacity == 0 {
		chDesc.RecvMessageCapacity = defaultRecvMessageCapacity
	}
	filled = chDesc
	return
}

// Channel 一个 Channel 只能对应一个 MultiConn，但是一个 MultiConn 能对应 多个 Channel
type Channel struct {
	conn      *MultiConn
	desc      ChannelDescriptor
	sendQueue chan []byte
	sendQueueSize int32 // 指示发送队列里有几波数据，默认情况下，可取值：1、0
	recving       []byte
	sending       []byte
	recentlySent  int64 // 每隔一定时间会乘以 0.8

	maxPacketMsgPayloadSize int

	Logger log.CRLogger
}

func newChannel(conn *MultiConn, desc ChannelDescriptor) *Channel {
	desc = desc.FillDefaults()
	if desc.Priority <= 0 {
		panic("Channel default priority must be a positive integer")
	}
	return &Channel{
		conn:                    conn,
		desc:                    desc,
		sendQueue:               make(chan []byte, desc.SendQueueCapacity),
		recving:                 make([]byte, 0, desc.RecvBufferCapacity),
		maxPacketMsgPayloadSize: conn.config.MaxPacketMsgPayloadSize,
	}
}

func (ch *Channel) SetLogger(l log.CRLogger) {
	ch.Logger = l
}

// sendBytes 尽力在超时时间内将数据推送到发送队列里，如果超时还没推送成功，就返回 false，反之返回 true
func (ch *Channel) sendBytes(bytes []byte) bool {
	select {
	case ch.sendQueue <- bytes:
		atomic.AddInt32(&ch.sendQueueSize, 1)
		return true
	case <-time.After(defaultSendTimeout):
		return false
	}
}

// trySendBytes 尝试将数据推送到发送队列里，如果队列已满，则返回 false，反之返回 true
func (ch *Channel) trySendBytes(bytes []byte) bool {
	select {
	case ch.sendQueue <- bytes:
		atomic.AddInt32(&ch.sendQueueSize, 1)
		return true
	default:
		return false
	}
}

// Goroutine-safe
func (ch *Channel) loadSendQueueSize() (size int) {
	return int(atomic.LoadInt32(&ch.sendQueueSize))
}

// Goroutine-safe
// Use only as a heuristic.
func (ch *Channel) canSend() bool {
	return ch.loadSendQueueSize() < defaultSendQueueCapacity
}

// isSendPending 在判断 Channel 是否有数据需要发送的同时，如果发送队列里有数据，
// 就将里面的数据传给 .sending
func (ch *Channel) isSendPending() bool {
	if len(ch.sending) == 0 {
		if len(ch.sendQueue) == 0 {
			return false
		}
		ch.sending = <-ch.sendQueue
	}
	return true
}

// nextPacketMsg 将 .sending 缓冲区里的数据打包成数据包
func (ch *Channel) nextPacketMsg() protogossip.PacketMsg {
	packet := protogossip.PacketMsg{ChannelID: int32(ch.desc.ID)}
	maxSize := ch.maxPacketMsgPayloadSize
	packet.Data = ch.sending[:srmath.MinInt(maxSize, len(ch.sending))]
	if len(ch.sending) <= maxSize {
		packet.EOF = true
		ch.sending = nil
		atomic.AddInt32(&ch.sendQueueSize, -1) // decrement sendQueueSize
	} else {
		packet.EOF = false
		ch.sending = ch.sending[srmath.MinInt(maxSize, len(ch.sending)):]
	}
	return packet
}

// writePacketMsgTo 先通过 .nextPacketMsg() 方法获取到一个数据包，然后将该数据包通过 protobuf 序列化发送出去
func (ch *Channel) writePacketMsgTo(w io.Writer) (n int, err error) {
	packet := ch.nextPacketMsg()
	n, err = protoio.NewDelimitedWriter(w).WriteMsg(mustWrapPacket(&packet))
	atomic.AddInt64(&ch.recentlySent, int64(n))
	return
}

// recvPacketMsg 解析 peer 发送过来的数据包，peer 可能发送的数据比较大，所以分成若干个包发送过来了，
// 所以每次解析数据包的时候都得判断当前数据包的 EOF 标志是否为 true，为 true 就表示该数据包是本轮发送数
// 据的最后一个包
func (ch *Channel) recvPacketMsg(packet protogossip.PacketMsg) ([]byte, error) {
	ch.Logger.Debugw("Read PacketMsg", "conn", ch.conn, "packet", packet)
	var recvCap, recvReceived = ch.desc.RecvMessageCapacity, len(ch.recving) + len(packet.Data)
	if recvCap < recvReceived {
		return nil, fmt.Errorf("received message exceeds available capacity: %v < %v", recvCap, recvReceived)
	}
	// 将数据一点点的拼在一起
	ch.recving = append(ch.recving, packet.Data...)
	if packet.EOF {
		msgBytes := ch.recving

		ch.recving = ch.recving[:0] // // 将存放接收数据的缓冲区清空
		return msgBytes, nil
	}
	return nil, nil
}

// 定期调用此函数以更新统计信息，以便进行节流。不是goroutine-safe
func (ch *Channel) updateStats() {
	// 乘法衰减，每个两秒让 recentlySent 乘以 0.8
	atomic.StoreInt64(&ch.recentlySent, int64(float64(atomic.LoadInt64(&ch.recentlySent))*0.8))
}

//----------------------------------------
// Packet

// mustWrapPacket 将 PacketPing、PacketPong、PacketMsg 打包成 Packet 发送出去
func mustWrapPacket(pb proto.Message) *protogossip.Packet {
	var msg protogossip.Packet

	switch pb := pb.(type) {
	case *protogossip.Packet: // already a packet
		msg = *pb
	case *protogossip.PacketPing:
		msg = protogossip.Packet{
			Sum: &protogossip.Packet_PacketPing{
				PacketPing: pb,
			},
		}
	case *protogossip.PacketPong:
		msg = protogossip.Packet{
			Sum: &protogossip.Packet_PacketPong{
				PacketPong: pb,
			},
		}
	case *protogossip.PacketMsg:
		msg = protogossip.Packet{
			Sum: &protogossip.Packet_PacketMsg{
				PacketMsg: pb,
			},
		}
	default:
		panic(fmt.Errorf("unknown packet type %T", pb))
	}

	return &msg
}
