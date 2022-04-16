package gossip

import (
	"github.com/232425wxy/BFT/libs/service"
)

// Reactor 负责处理一个或多个 Channel 上的传入消息。当 Reactor 被添加到 Switch 时
// 调用 GetChannels；当一个新的 Peer 加入我们的节点时，将调用 InitPeer 和 AddPeer；
// 当 Peer 停止时调用 RemovePeer；当在与此 Reactor 关联的 Channel 上接收到消息时，调用
// Receive 方法。当需要给 Peer 发送消息时，应该调用 Peer 的 Send 或者 TrySend 方法。
type Reactor interface {
	service.Service

	// SetSwitch 设置一个 Switch
	SetSwitch(*Switch)

	// GetChannels 返回 MultiConn.ChannelDescriptor 列表。确保所有加到 Switch 上的 Reactor 的每个 ID 都是唯一的。
	GetChannels() []*ChannelDescriptor

	// InitPeer 在对等节点启动之前，由 Switch 调用 InitPeer。使用它来初始化对等节点的数据(例如对等节点状态)。
	// 注意:如果 Switch 无法启动对等节点，则不会调用 AddPeer 或 RemovePeer；不要在 Reactor 本身中存储任何
	// 与对等节点相关联的数据，除非你不想有一个永远不会被清理的状态。
	InitPeer(peer *Peer) *Peer

	// AddPeer 在添加对等节点并成功启动后由 Switch 调用，使用它来开始与对等节点的通信。
	AddPeer(peer *Peer)

	// RemovePeer 当对等节点停止时(由于错误或其他原因)，由 Switch 调用 RemovePeer。
	RemovePeer(peer *Peer, reason interface{})

	// Receive 当从对等端接收到 msgBytes 时，Switch 调用 Receive；
	// 注意:如果没有复制，Reactor 不能在 Receive 完成后保留 msgBytes；
	// CONTRACT: msgBytes 不能是 nil。
	Receive(chID byte, peer *Peer, msgBytes []byte)
}

//--------------------------------------

type BaseReactor struct {
	service.BaseService
	Switch *Switch
}

func NewBaseReactor(name string, impl Reactor) *BaseReactor {
	return &BaseReactor{
		BaseService: *service.NewBaseService(nil, name, impl),
		Switch:      nil,
	}
}

func (br *BaseReactor) SetSwitch(sw *Switch) {
	br.Switch = sw
}
func (*BaseReactor) GetChannels() []*ChannelDescriptor { return nil }
func (*BaseReactor) AddPeer(peer *Peer)                {}
func (*BaseReactor) RemovePeer(peer *Peer, reason interface{})      {}
func (*BaseReactor) Receive(chID byte, peer *Peer, msgBytes []byte) {}
func (*BaseReactor) InitPeer(peer *Peer) *Peer                       { return peer }
