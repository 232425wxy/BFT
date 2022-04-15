package mempool

import (
	"BFT/config"
	"BFT/gossip"
	"BFT/libs/clist"
	srlog "BFT/libs/log"
	protomem "BFT/proto/mempool"
	"BFT/types"
	"errors"
	"fmt"
	"time"
)

const (
	MempoolChannel = byte(0x30)

	peerCatchupSleepIntervalMS = 100 // If peer is behind, sleep this amount
)

// Reactor 处理 mempool tx在对等体之间的广播。
type Reactor struct {
	gossip.BaseReactor
	config  *config.MempoolConfig
	mempool *CListMempool
}

// NewReactor returns a new Reactor with the given config and mempool.
func NewReactor(config *config.MempoolConfig, mempool *CListMempool) *Reactor {
	memR := &Reactor{
		config:  config,
		mempool: mempool,
	}
	memR.BaseReactor = *gossip.NewBaseReactor("Mempool", memR)
	return memR
}

// InitPeer implements Reactor by creating a state for the peer.
//	为 peer 分配一个内部的 id(uint16)
func (memR *Reactor) InitPeer(peer *gossip.Peer) *gossip.Peer {
	return peer
}

// SetLogger sets the Logger on the reactor and the underlying mempool.
func (memR *Reactor) SetLogger(l srlog.CRLogger) {
	memR.Logger = l
	memR.mempool.SetLogger(l)
}

// OnStart implements gossip.BaseReactor.
func (memR *Reactor) OnStart() error {
	if !memR.config.Broadcast {
		memR.Logger.Infow("Tx broadcasting is disabled")
	}
	return nil
}

// GetChannels implements Reactor by returning the list of channels for this
// reactor.
// 	mempool 的 reactor 管理着 MempoolChannel 这唯一一个 Channel
func (memR *Reactor) GetChannels() []*gossip.ChannelDescriptor {
	// largestTx 最大的 tx
	largestTx := make([]byte, memR.config.MaxTxBytes)
	batchMsg := protomem.Message{
		Sum: &protomem.Message_Txs{
			Txs: &protomem.Txs{Txs: [][]byte{largestTx}},
		},
	}

	return []*gossip.ChannelDescriptor{
		{
			ID:                  MempoolChannel,
			Priority:            5,
			RecvMessageCapacity: batchMsg.Size(), // 所能收到的数据大小，这里表示收到的一个 tx 的最大大小
		},
	}
}

// AddPeer implements Reactor.
// 	它启动一个广播 routine，确保以后所有的 txs 被转发到这个新加入的 peer
func (memR *Reactor) AddPeer(peer *gossip.Peer) {
	if memR.config.Broadcast {
		go memR.broadcastTxRoutine(peer)
	}
}

// RemovePeer implements Reactor.
//	回收该 peer 之前占用的内部 id(uint16)
func (memR *Reactor) RemovePeer(peer *gossip.Peer, reason interface{}) {
	// broadcast routine checks if peer is gone and returns
}

// Receive implements Reactor.
// It adds any received transactions to the mempool.
func (memR *Reactor) Receive(chID byte, src *gossip.Peer, msgBytes []byte) {
	msg, err := memR.decodeMsg(msgBytes)
	if err != nil {
		memR.Logger.Errorw("Error decoding message", "src", src, "chId", chID, "err", err)
		// 解码错误，就把发送这个消息的 peer 给关了
		memR.Switch.StopPeerForError(src, err)
		return
	}
	memR.Logger.Debugw("Receive", "src", src, "chId", chID, "msg", msg)

	txInfo := TxInfo{SenderP2PID: src.ID()} // 构造 TxInfo，存储着谁发送该 tx 的节点信息
	for _, tx := range msg.Txs {
		err = memR.mempool.CheckTx(tx, nil, txInfo) // 检查 peer 发送过来的每个 tx
		if err == ErrTxInCache {
			memR.Logger.Debugw("Tx already exists in cache", "tx", txID(tx))
		} else if err != nil {
			memR.Logger.Infow("Could not check tx", "tx", txID(tx), "err", err)
		}
	}
	// broadcasting happens from go routines per peer
}

// PeerState describes the state of a peer.
type PeerState interface {
	GetHeight() int64
}

// Send new mempool txs to peer.
func (memR *Reactor) broadcastTxRoutine(peer *gossip.Peer) {
	var next *clist.CElement

	for {
		// In case of both next.NextWaitChan() and peer.Quit() are variable at the same time
		if !memR.IsRunning() || !peer.IsRunning() {
			return
		}
		// This happens because the CElement we were looking at got garbage
		// collected (removed). That is, .NextWait() returned nil. Go ahead and
		// start from the beginning.
		if next == nil {
			select {
			case <-memR.mempool.TxsWaitChan(): // 当内存池里有 tx 时，memR.mempool.TxsWaitChan() 返回的 channel 就会被关闭
				if next = memR.mempool.TxsFront(); next == nil {
					continue
				}
				// 到这里是拿到了内存池里的第一个 tx 了
			case <-peer.Quit():
				return
			case <-memR.Quit():
				return
			}
		}

		// Make sure the peer is up to date.
		peerState, ok := peer.Get(types.PeerStateKey).(PeerState)
		if !ok {
			// Peer还没有一个 state。我们在 consensus reactor 中设置了它，
			// 但是当我们在 Switch 中添加 peer 时，由于我们使用的是 map，所
			// 以我们每次调用 reactor#AddPeer 的顺序都是不同的。有时其他 reactor
			// 将在 consensus reactor 之前进行初始化。我们应该等几毫秒再试。
			time.Sleep(peerCatchupSleepIntervalMS * time.Millisecond)
			continue
		}

		// 允许有 1 block 的延迟。
		memTx := next.Value.(*mempoolTx)
		if peerState.GetHeight() < memTx.Height()-1 {
			time.Sleep(peerCatchupSleepIntervalMS * time.Millisecond)
			continue
		}

		if _, ok := memTx.senders.Load(peer.ID()); !ok {
			msg := protomem.Message{
				Sum: &protomem.Message_Txs{
					Txs: &protomem.Txs{Txs: [][]byte{memTx.tx}},
				},
			}
			bz, err := msg.Marshal()
			if err != nil {
				panic(err)
			}
			success := peer.Send(MempoolChannel, bz)
			if !success {
				time.Sleep(peerCatchupSleepIntervalMS * time.Millisecond)
				continue
			}
		}

		select {
		case <-next.NextWaitChan():
			// see the start of the for loop for nil check
			next = next.Next()
		case <-peer.Quit():
			return
		case <-memR.Quit():
			return
		}
	}
}

//-----------------------------------------------------------------------------
// Messages

func (memR *Reactor) decodeMsg(bz []byte) (TxsMessage, error) {
	msg := protomem.Message{}
	err := msg.Unmarshal(bz)
	if err != nil {
		return TxsMessage{}, err
	}

	var message TxsMessage

	if i, ok := msg.Sum.(*protomem.Message_Txs); ok {
		txs := i.Txs.GetTxs()

		if len(txs) == 0 {
			return message, errors.New("empty TxsMessage")
		}

		decoded := make([]types.Tx, len(txs))
		for j, tx := range txs {
			decoded[j] = types.Tx(tx)
		}

		message = TxsMessage{
			Txs: decoded,
		}
		return message, nil
	}
	return message, fmt.Errorf("msg type: %T is not supported", msg)
}

//-------------------------------------

// TxsMessage is a Message containing transactions.
type TxsMessage struct {
	Txs []types.Tx
}

// String returns a string representation of the TxsMessage.
func (m *TxsMessage) String() string {
	return fmt.Sprintf("[TxsMessage %v]", m.Txs)
}
