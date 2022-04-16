package consensus

import (
	"errors"
	"fmt"
	"github.com/232425wxy/BFT/gossip"
	"github.com/232425wxy/BFT/libs/bits"
	srevents "github.com/232425wxy/BFT/libs/events"
	srjson "github.com/232425wxy/BFT/libs/json"
	srlog "github.com/232425wxy/BFT/libs/log"
	srtime "github.com/232425wxy/BFT/libs/time"
	protoconsensus "github.com/232425wxy/BFT/proto/consensus"
	prototypes "github.com/232425wxy/BFT/proto/types"
	sm "github.com/232425wxy/BFT/state"
	"github.com/232425wxy/BFT/types"
	"github.com/gogo/protobuf/proto"
	"reflect"
	"sync"
	"time"
)

const (
	StateChannel       = byte(0x20)
	DataChannel        = byte(0x21)
	VoteChannel        = byte(0x22)
	VoteSetBitsChannel = byte(0x23)

	maxMsgSize = 1048576

	blocksToContributeToBecomeGoodPeer = 10000
	votesToContributeToBecomeGoodPeer  = 10000
)

//-----------------------------------------------------------------------------

// Reactor 定义了共识服务的 reactor
type Reactor struct {
	// service.BaseService + gossip.Switch
	gossip.BaseReactor

	// 共识状态机
	conS *State

	mtx      sync.RWMutex
	waitSync bool
	eventBus *types.EventBus
}

// ReactorOption 共识 Reactor 的配置选项
type ReactorOption func(*Reactor)

// NewReactor 根据给定的共识状态机实例化一个 Reactor
func NewReactor(consensusState *State, waitSync bool, options ...ReactorOption) *Reactor {
	conR := &Reactor{
		conS:     consensusState,
		waitSync: waitSync,
	}

	conR.BaseReactor = *gossip.NewBaseReactor("Consensus", conR)

	for _, option := range options {
		option(conR)
	}

	return conR
}

// OnStart implements BaseService by subscribing to events, which later will be
// broadcasted to other peers and starting state if we're not in fast sync.
func (conR *Reactor) OnStart() error {
	conR.Logger.Infow("Reactor ", "waitSync", conR.WaitSync())

	// 从共识状态机的 statsMsgQueue 消息通道里获取消息，持续评估 peer 的质量
	go conR.peerStatsRoutine()

	conR.subscribeToBroadcastEvents()

	if !conR.WaitSync() {
		// 如果 Reactor 的 waitSync 等于 false，则启动 Reactor 的共识状态机
		err := conR.conS.Start()
		if err != nil {
			return err
		}
	}

	return nil
}

// OnStop implements BaseService by unsubscribing from events and stopping
// state.
func (conR *Reactor) OnStop() {
	conR.unsubscribeFromBroadcastEvents()
	if err := conR.conS.Stop(); err != nil {
		conR.Logger.Errorw("Error stopping consensus state", "err", err)
	}
	if !conR.WaitSync() {
		conR.conS.Wait()
	}
}

// SwitchToConsensus 仅由 node 的 startStateSync 和 blockchain 的 poolRoutine 两个方法调用
func (conR *Reactor) SwitchToConsensus(state sm.State, skipWAL bool) {
	conR.Logger.Infow("SwitchToConsensus")

	// We have no votes, so reconstruct LastCommits from SeenCommit.
	if state.LastBlockHeight > 0 {
		conR.conS.reconstructLastCommit(state)
	}

	conR.conS.updateToState(state)

	conR.mtx.Lock()
	conR.waitSync = false
	conR.mtx.Unlock()

	if skipWAL {
		conR.conS.doWALCatchup = false
	}
	err := conR.conS.Start()
	if err != nil {
		panic(fmt.Sprintf(`Failed to start consensus state: %v`, err))
	}
}

// GetChannels implements Reactor
func (conR *Reactor) GetChannels() []*gossip.ChannelDescriptor {
	return []*gossip.ChannelDescriptor{
		{
			ID:                  StateChannel,
			Priority:            6,
			SendQueueCapacity:   100,
			RecvMessageCapacity: maxMsgSize,
		},
		{
			ID: DataChannel,
			Priority:            10,
			SendQueueCapacity:   100,
			RecvBufferCapacity:  50 * 4096,
			RecvMessageCapacity: maxMsgSize,
		},
		{
			ID:                  VoteChannel,
			Priority:            7,
			SendQueueCapacity:   100,
			RecvBufferCapacity:  100 * 100,
			RecvMessageCapacity: maxMsgSize,
		},
		{
			ID:                  VoteSetBitsChannel,
			Priority:            1,
			SendQueueCapacity:   2,
			RecvBufferCapacity:  1024,
			RecvMessageCapacity: maxMsgSize,
		},
	}
}

// InitPeer 被switch的addPeer方法调用，InitPeer 必须在 AddPeer 之前调用
func (conR *Reactor) InitPeer(peer *gossip.Peer) *gossip.Peer {
	peerState := NewPeerState(peer).SetLogger(conR.Logger)
	peer.Set(types.PeerStateKey, peerState)
	return peer
}

// AddPeer 被switch的addPeer方法调用，AddPeer 必须在 InitPeer 之后调用
func (conR *Reactor) AddPeer(peer *gossip.Peer) {
	if !conR.IsRunning() {
		return
	}

	peerState, ok := peer.Get(types.PeerStateKey).(*PeerState)
	if !ok {
		panic(fmt.Sprintf("peer %v has no state", peer))
	}
	// Begin routines for this peer.
	go conR.gossipDataRoutine(peer, peerState)
	go conR.gossipVotesRoutine(peer, peerState)
	go conR.queryMaj23Routine(peer, peerState)

	// Send our state to peer.
	// If we're fast_syncing, broadcast a RoundStepMessage later upon SwitchToConsensus().
	if !conR.WaitSync() {
		conR.sendNewRoundStepMessage(peer)
	}
}

// RemovePeer is a noop.
func (conR *Reactor) RemovePeer(peer *gossip.Peer, reason interface{}) {
	if !conR.IsRunning() {
		return
	}
}

func (conR *Reactor) Receive(chID byte, src *gossip.Peer, msgBytes []byte) {
	if !conR.IsRunning() {
		return
	}

	msg, err := decodeMsg(msgBytes)
	if err != nil {
		conR.Logger.Errorw("Error decoding message", "src", src, "chId", chID, "err", err)
		conR.Switch.StopPeerForError(src, err)
		return
	}

	if err = msg.ValidateBasic(); err != nil {
		conR.Logger.Errorw("Peer sent us invalid msg", "peer", src, "msg", msg, "err", err)
		conR.Switch.StopPeerForError(src, err)
		return
	}

	conR.Logger.Debugw("Receive", "src", src, "chId", chID, "msg", msg)

	// Get peer states
	ps, ok := src.Get(types.PeerStateKey).(*PeerState)
	if !ok {
		panic(fmt.Sprintf("Peer %v has no state", src))
	}

	switch chID {
	case StateChannel:
		switch msg := msg.(type) {
		case *NewRoundStepMessage:
			conR.conS.mtx.Lock()
			initialHeight := conR.conS.state.InitialHeight
			conR.conS.mtx.Unlock()
			if err = msg.ValidateHeight(initialHeight); err != nil {
				conR.Logger.Errorw("Peer sent us invalid msg", "peer", src, "msg", msg, "err", err)
				conR.Switch.StopPeerForError(src, err)
				return
			}
			ps.ApplyNewRoundStepMessage(msg)
		case *NewValidBlockMessage:
			ps.ApplyNewValidBlockMessage(msg)
		case *HasVoteMessage:
			ps.ApplyHasVoteMessage(msg)
		case *VoteSetMaj23Message:  // peer 声称自己收集到了+2/3个投票
			cs := conR.conS
			cs.mtx.Lock()
			height, votes := cs.Height, cs.Votes
			cs.mtx.Unlock()
			if height != msg.Height {
				return
			}
			// Peer claims to have a maj23 for some BlockID at H,R,S,
			err := votes.SetPeerMaj23(msg.Round, msg.Type, ps.peer.ID(), msg.BlockID)
			if err != nil {
				conR.Switch.StopPeerForError(src, err)
				return
			}
			var ourVotes *bits.BitArray
			switch msg.Type {
			case prototypes.PrepareType:
				ourVotes = votes.Prepares(msg.Round).BitArrayByBlockID(msg.BlockID)
			case prototypes.CommitType:
				ourVotes = votes.Commits(msg.Round).BitArrayByBlockID(msg.BlockID)
			default:
				panic("Bad VoteSetBitsMessage field Type. Forgot to add a check in ValidateBasic?")
			}
			src.TrySend(VoteSetBitsChannel, MustEncode(&VoteSetBitsMessage{Height: msg.Height, Round: msg.Round, Type: msg.Type, BlockID: msg.BlockID, Votes: ourVotes}))
		default:
			conR.Logger.Errorw(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
		}

	case DataChannel:
		if conR.WaitSync() {
			conR.Logger.Infow("Ignoring message received during sync", "msg", msg)
			return
		}
		switch msg := msg.(type) {
		case *PrePrepareMessage:
			ps.SetHasProposal(msg.PrePrepare) // 标记一下该peer给我们发送给proposal
			conR.conS.peerMsgQueue <- msgInfo{msg, src.ID()}
		case *PrePreparePOLMessage:
			ps.ApplyProposalPOLMessage(msg)
		case *BlockPartMessage:
			ps.SetHasProposalBlockPart(msg.Height, msg.Round, int(msg.Part.Index))
			conR.conS.peerMsgQueue <- msgInfo{msg, src.ID()}
		default:
			conR.Logger.Errorw(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
		}

	case VoteChannel:
		if conR.WaitSync() {
			conR.Logger.Infow("Ignoring message received during sync", "msg", msg)
			return
		}
		switch msg := msg.(type) {
		case *VoteMessage:
			cs := conR.conS
			cs.mtx.RLock()
			height, valSize, lastCommitSize := cs.Height, cs.Validators.Size(), cs.LastCommits.Size()
			cs.mtx.RUnlock()
			ps.EnsureVoteBitArrays(height, valSize)
			ps.EnsureVoteBitArrays(height-1, lastCommitSize)
			ps.SetHasVote(msg.Vote)
			cs.peerMsgQueue <- msgInfo{msg, src.ID()}

		default:
			conR.Logger.Errorw(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
		}

	case VoteSetBitsChannel:
		if conR.WaitSync() {
			conR.Logger.Infow("Ignoring message received during sync", "msg", msg)
			return
		}
		switch msg := msg.(type) {
		case *VoteSetBitsMessage:
			cs := conR.conS
			cs.mtx.Lock()
			height, votes := cs.Height, cs.Votes
			cs.mtx.Unlock()

			if height == msg.Height {
				var ourVotes *bits.BitArray
				switch msg.Type {
				case prototypes.PrepareType:
					ourVotes = votes.Prepares(msg.Round).BitArrayByBlockID(msg.BlockID)
				case prototypes.CommitType:
					ourVotes = votes.Commits(msg.Round).BitArrayByBlockID(msg.BlockID)
				default:
					panic("Bad VoteSetBitsMessage field Type. Forgot to add a check in ValidateBasic?")
				}
				ps.ApplyVoteSetBitsMessage(msg, ourVotes)
			} else {
				ps.ApplyVoteSetBitsMessage(msg, nil)
			}
		default:
			// don't punish (leave room for soft upgrades)
			conR.Logger.Errorw(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
		}

	default:
		conR.Logger.Errorw(fmt.Sprintf("Unknown chId %X", chID))
	}
}

// SetEventBus sets event bus.
func (conR *Reactor) SetEventBus(b *types.EventBus) {
	conR.eventBus = b
	conR.conS.SetEventBus(b)
}

// WaitSync 返回 Reactor 的 waitSync
func (conR *Reactor) WaitSync() bool {
	conR.mtx.RLock()
	defer conR.mtx.RUnlock()
	return conR.waitSync
}

//--------------------------------------

// subscribeToBroadcastEvents 为 reactor 订阅相关事件：
//	NewRoundStep、ValidBlock、Vote，listenerID 是 consensus-reactor
func (conR *Reactor) subscribeToBroadcastEvents() {
	const subscriber = "consensus-reactor"
	if err := conR.conS.evsw.AddListenerForEvent(subscriber, types.EventNewRoundStep,
		func(data srevents.EventData) {
			conR.broadcastNewRoundStepMessage(data.(*RoundState))
		}); err != nil {
		conR.Logger.Errorw("Error adding listener for events", "err", err)
	}

	if err := conR.conS.evsw.AddListenerForEvent(subscriber, types.EventValidBlock,
		func(data srevents.EventData) {
			conR.broadcastNewValidBlockMessage(data.(*RoundState))
		}); err != nil {
		conR.Logger.Errorw("Error adding listener for events", "err", err)
	}

	if err := conR.conS.evsw.AddListenerForEvent(subscriber, types.EventVote,
		func(data srevents.EventData) {
			conR.broadcastHasVoteMessage(data.(*types.Vote))
		}); err != nil {
		conR.Logger.Errorw("Error adding listener for events", "err", err)
	}
}

func (conR *Reactor) unsubscribeFromBroadcastEvents() {
	const subscriber = "consensus-reactor"
	conR.conS.evsw.RemoveListener(subscriber)
}

// 告诉其他节点自己进入了新的step，仅会在调用newStep函数的时候才会广播
func (conR *Reactor) broadcastNewRoundStepMessage(rs *RoundState) {
	nrsMsg := makeRoundStepMessage(rs)
	conR.Switch.Broadcast(StateChannel, MustEncode(nrsMsg))
}

// 告诉其他节点自己获得了一个valid block
func (conR *Reactor) broadcastNewValidBlockMessage(rs *RoundState) {
	csMsg := &NewValidBlockMessage{
		Height:             rs.Height,
		Round:              rs.Round,
		BlockPartSetHeader: rs.PrePrepareBlockParts.Header(),
		BlockParts:         rs.PrePrepareBlockParts.BitArray(),
		IsCommit:           rs.Step == RoundStepReply,
	}
	conR.Switch.Broadcast(StateChannel, MustEncode(csMsg))
}

// Broadcasts 告诉其他节点自己添加了一个vote
func (conR *Reactor) broadcastHasVoteMessage(vote *types.Vote) {
	msg := &HasVoteMessage{
		Height: vote.Height,
		Round:  vote.Round,
		Type:   vote.Type,
		Index:  vote.ValidatorIndex,
	}
	conR.Switch.Broadcast(StateChannel, MustEncode(msg))
}

// makeRoundStepMessage 仅被 broadcastNewRoundStepMessage 和 sendNewRoundStepMessage 两个函数调用
func makeRoundStepMessage(rs *RoundState) (nrsMsg *NewRoundStepMessage) {
	nrsMsg = &NewRoundStepMessage{
		Height:                rs.Height,
		Round:                 rs.Round,
		Step:                  rs.Step,
		SecondsSinceStartTime: int64(time.Since(rs.StartTime).Seconds()),
		LastCommitRound:       rs.LastCommits.GetRound(),
	}
	return
}

func (conR *Reactor) sendNewRoundStepMessage(peer *gossip.Peer) {
	rs := conR.conS.RoundState
	nrsMsg := makeRoundStepMessage(&rs)
	peer.Send(StateChannel, MustEncode(nrsMsg))
}

func (conR *Reactor) gossipDataRoutine(peer *gossip.Peer, ps *PeerState) {
	logger := conR.Logger.With("peer", peer)

OUTER_LOOP:
	for {
		if !peer.IsRunning() || !conR.IsRunning() {
			logger.Errorw("Stopping gossipDataRoutine for peer", "peer", peer.ID(), "ip", peer.RemoteIP())
			return
		}
		rs := conR.conS.RoundState
		prs := ps.GetRoundState()

		// Send proposal Block parts?
		if rs.PrePrepareBlockParts.HeaderEqualsTo(prs.ProposalBlockPartSetHeader) {
			if index, ok := rs.PrePrepareBlockParts.BitArray().Sub(prs.ProposalBlockParts.Copy()).PickRandom(); ok {
				// 这里的PickRandom是合理的，因为对于发送过的part，我们会用SetHasProposalBlockPart进行标记
				part := rs.PrePrepareBlockParts.GetPart(index)
				msg := &BlockPartMessage{
					Height: rs.Height, // This tells peer that this part applies to us.
					Round:  rs.Round,  // This tells peer that this part applies to us.
					Part:   part,
				}
				logger.Debugw("Sending block part", "height", prs.Height, "round", prs.Round)
				if peer.Send(DataChannel, MustEncode(msg)) {
					ps.SetHasProposalBlockPart(prs.Height, prs.Round, index)
				}
				continue OUTER_LOOP
			}
		}

		// If the peer is on a previous height that we have, help catch up.
		blockStoreBase := conR.conS.blockStore.Base()
		if blockStoreBase > 0 && 0 < prs.Height && prs.Height < rs.Height && prs.Height >= blockStoreBase {
			heightLogger := logger.With("height", prs.Height)

			// if we never received the commit message from the peer, the block parts wont be initialized
			if prs.ProposalBlockParts == nil {
				blockMeta := conR.conS.blockStore.LoadBlockMeta(prs.Height)
				if blockMeta == nil {
					heightLogger.Errorw("Failed to load block meta", "blockstoreBase", blockStoreBase, "blockstoreHeight", conR.conS.blockStore.Height())
					time.Sleep(conR.conS.config.PeerGossipSleepDuration)  // 默认睡眠100ms
				} else {
					ps.InitProposalBlockParts(blockMeta.BlockID.PartSetHeader)
				}
				// continue the loop since prs is a copy and not effected by this initialization
				continue OUTER_LOOP
			}
			conR.gossipDataForCatchup(heightLogger, &rs, prs, ps, peer)
			continue OUTER_LOOP
		}

		if (rs.Height != prs.Height) || (rs.Round != prs.Round) {
			time.Sleep(conR.conS.config.PeerGossipSleepDuration)
			continue OUTER_LOOP
		}

		if rs.PrePrepare != nil && !prs.Proposal {
			{
				msg := &PrePrepareMessage{PrePrepare: rs.PrePrepare}
				logger.Debugw("Sending proposal", "height", prs.Height, "round", prs.Round)
				if peer.Send(DataChannel, MustEncode(msg)) {
					ps.SetHasProposal(rs.PrePrepare)
				}
			}

			if 0 <= rs.PrePrepare.POLRound {
				msg := &PrePreparePOLMessage{
					Height:             rs.Height,
					PrePreparePOLRound: rs.PrePrepare.POLRound,
					PrePreparePOL:      rs.Votes.Prepares(rs.PrePrepare.POLRound).BitArray(),
				}
				logger.Debugw("Sending POL", "height", prs.Height, "round", prs.Round)
				peer.Send(DataChannel, MustEncode(msg))
			}
			continue OUTER_LOOP
		}

		// Nothing to do. Sleep.
		time.Sleep(conR.conS.config.PeerGossipSleepDuration)
		continue OUTER_LOOP
	}
}

func (conR *Reactor) gossipDataForCatchup(logger srlog.CRLogger, rs *RoundState, prs *PeerRoundState, ps *PeerState, peer *gossip.Peer) {
	if index, ok := prs.ProposalBlockParts.Not().PickRandom(); ok {
		// Ensure that the peer's PartSetHeader is correct
		blockMeta := conR.conS.blockStore.LoadBlockMeta(prs.Height)
		if blockMeta == nil {
			logger.Errorw("Failed to load block meta", "ourHeight", rs.Height, "blockstoreBase", conR.conS.blockStore.Base(), "blockstoreHeight", conR.conS.blockStore.Height())
			time.Sleep(conR.conS.config.PeerGossipSleepDuration)
			return
		} else if !blockMeta.BlockID.PartSetHeader.Equals(prs.ProposalBlockPartSetHeader) {
			logger.Infow("Peer ProposalBlockPartSetHeader mismatch, sleeping", "blockPartSetHeader", blockMeta.BlockID.PartSetHeader, "peerBlockPartSetHeader", prs.ProposalBlockPartSetHeader)
			time.Sleep(conR.conS.config.PeerGossipSleepDuration)
			return
		}
		// Load the part
		part := conR.conS.blockStore.LoadBlockPart(prs.Height, index)
		if part == nil {
			logger.Errorw("Could not load part", "index", index, "blockPartSetHeader", blockMeta.BlockID.PartSetHeader, "peerBlockPartSetHeader", prs.ProposalBlockPartSetHeader)
			time.Sleep(conR.conS.config.PeerGossipSleepDuration)
			return
		}
		// Send the part
		msg := &BlockPartMessage{
			Height: prs.Height, // Not our height, so it doesn't matter.
			Round:  prs.Round,  // Not our height, so it doesn't matter.
			Part:   part,
		}
		logger.Debugw("Sending block part for catchup", "round", prs.Round, "index", index)
		if peer.Send(DataChannel, MustEncode(msg)) {
			ps.SetHasProposalBlockPart(prs.Height, prs.Round, index)
		} else {
			logger.Debugw("Sending block part for catchup failed")
		}
		return
	}
	//  Logger.Infow("No parts to send in catch-up, sleeping")
	time.Sleep(conR.conS.config.PeerGossipSleepDuration)
}

func (conR *Reactor) gossipVotesRoutine(peer *gossip.Peer, ps *PeerState) {
	logger := conR.Logger.With("peer", peer)

OUTER_LOOP:
	for {
		// Manage disconnects from self or peer.
		if !peer.IsRunning() || !conR.IsRunning() {
			logger.Errorw("Stopping gossipVotesRoutine for peer", "peer", peer.ID(), "ip", peer.RemoteIP())
			return
		}
		rs := conR.conS.RoundState
		prs := ps.GetRoundState()

		// If height matches, then send LastCommits, Prepares, Commits.
		if rs.Height == prs.Height {
			heightLogger := logger.With("height", prs.Height)
			if conR.gossipVotesForHeight(heightLogger, &rs, prs, ps) {
				continue OUTER_LOOP
			}
		}

		// Special catchup logic.
		// If peer is lagging by height 1, send LastCommits.
		if prs.Height != 0 && rs.Height == prs.Height+1 {
			if ps.PickSendVote(rs.LastCommits) {
				logger.Debugw("Picked rs.LastCommits to send", "height", prs.Height)
				continue OUTER_LOOP
			}
		}

		// Catchup logic
		// If peer is lagging by more than 1, send Reply.
		blockStoreBase := conR.conS.blockStore.Base()
		if blockStoreBase > 0 && prs.Height != 0 && rs.Height >= prs.Height+2 && prs.Height >= blockStoreBase {
			// Load the block commit for prs.Height,
			// which contains precommit signatures for prs.Height.
			if commit := conR.conS.blockStore.LoadBlockReply(prs.Height); commit != nil {
				if ps.PickSendVote(commit) {
					logger.Debugw("Picked Catchup commit to send", "height", prs.Height)
					continue OUTER_LOOP
				}
			}
		}

		time.Sleep(conR.conS.config.PeerGossipSleepDuration)
		continue OUTER_LOOP
	}
}

func (conR *Reactor) gossipVotesForHeight(logger srlog.CRLogger, rs *RoundState, prs *PeerRoundState, ps *PeerState) bool {

	// If there are lastCommits to send...
	if prs.Step == RoundStepNewHeight {
		if ps.PickSendVote(rs.LastCommits) {
			logger.Debugw("Picked rs.LastCommits to send")
			return true
		}
	}
	// If there are POL prevotes to send...
	if prs.Step <= RoundStepPrePrepare && prs.Round != -1 && prs.Round <= rs.Round && prs.ProposalPOLRound != -1 {
		if polPrevotes := rs.Votes.Prepares(prs.ProposalPOLRound); polPrevotes != nil {
			if ps.PickSendVote(polPrevotes) {
				logger.Debugw("Picked rs.Prepares(prs.PrePreparePOLRound) to send",
					"round", prs.ProposalPOLRound)
				return true
			}
		}
	}
	// If there are prevotes to send...
	if prs.Step <= RoundStepPrepareWait && prs.Round != -1 && prs.Round <= rs.Round {
		if ps.PickSendVote(rs.Votes.Prepares(prs.Round)) {
			logger.Debugw("Picked rs.Prepares(prs.Round) to send", "round", prs.Round)
			return true
		}
	}
	// If there are precommits to send...
	if prs.Step <= RoundStepCommitWait && prs.Round != -1 && prs.Round <= rs.Round {
		if ps.PickSendVote(rs.Votes.Commits(prs.Round)) {
			logger.Debugw("Picked rs.Commits(prs.Round) to send", "round", prs.Round)
			return true
		}
	}
	// If there are prevotes to send...Needed because of validBlock mechanism
	if prs.Round != -1 && prs.Round <= rs.Round {
		if ps.PickSendVote(rs.Votes.Prepares(prs.Round)) {
			logger.Debugw("Picked rs.Prepares(prs.Round) to send", "round", prs.Round)
			return true
		}
	}
	// If there are POLPrevotes to send...
	if prs.ProposalPOLRound != -1 {
		if polPrevotes := rs.Votes.Prepares(prs.ProposalPOLRound); polPrevotes != nil {
			if ps.PickSendVote(polPrevotes) {
				logger.Debugw("Picked rs.Prepares(prs.PrePreparePOLRound) to send",
					"round", prs.ProposalPOLRound)
				return true
			}
		}
	}

	return false
}

// NOTE: `queryMaj23Routine` has a simple crude design since it only comes
// into play for liveness when there's a signature DDoS attack happening.
func (conR *Reactor) queryMaj23Routine(peer *gossip.Peer, ps *PeerState) {
	logger := conR.Logger.With("peer", peer)

OUTER_LOOP:
	for {
		// Manage disconnects from self or peer.
		if !peer.IsRunning() || !conR.IsRunning() {
			logger.Infow("Stopping queryMaj23Routine for peer", "peer", peer.ID(), "ip", peer.RemoteIP())
			return
		}

		// Maybe send Height/Round/Prepares
		{
			rs := conR.conS.RoundState
			prs := ps.GetRoundState()
			if rs.Height == prs.Height {
				if maj23, ok := rs.Votes.Prepares(prs.Round).TwoThirdsMajority(); ok {
					peer.TrySend(StateChannel, MustEncode(&VoteSetMaj23Message{
						Height:  prs.Height,
						Round:   prs.Round,
						Type:    prototypes.PrepareType,
						BlockID: maj23,
					}))
					time.Sleep(conR.conS.config.PeerQueryMaj23SleepDuration)
				}
			}
		}

		// Maybe send Height/Round/Commits
		{
			rs := conR.conS.RoundState
			prs := ps.GetRoundState()
			if rs.Height == prs.Height {
				if maj23, ok := rs.Votes.Commits(prs.Round).TwoThirdsMajority(); ok {
					peer.TrySend(StateChannel, MustEncode(&VoteSetMaj23Message{
						Height:  prs.Height,
						Round:   prs.Round,
						Type:    prototypes.CommitType,
						BlockID: maj23,
					}))
					time.Sleep(conR.conS.config.PeerQueryMaj23SleepDuration)
				}
			}
		}

		// Maybe send Height/Round/PrePreparePOL
		{
			rs := conR.conS.RoundState
			prs := ps.GetRoundState()
			if rs.Height == prs.Height && prs.ProposalPOLRound >= 0 {
				if maj23, ok := rs.Votes.Prepares(prs.ProposalPOLRound).TwoThirdsMajority(); ok {
					peer.TrySend(StateChannel, MustEncode(&VoteSetMaj23Message{
						Height:  prs.Height,
						Round:   prs.ProposalPOLRound,
						Type:    prototypes.PrepareType,
						BlockID: maj23,
					}))
					time.Sleep(conR.conS.config.PeerQueryMaj23SleepDuration)
				}
			}
		}

		// Little point sending LastCommitRound/LastCommits,
		// These are fleeting and non-blocking.

		// Maybe send Height/CatchupCommitRound/CatchupCommit.
		{
			prs := ps.GetRoundState()
			if prs.CatchupCommitRound != -1 && prs.Height > 0 && prs.Height <= conR.conS.blockStore.Height() &&
				prs.Height >= conR.conS.blockStore.Base() {
				if commit := conR.conS.LoadCommit(prs.Height); commit != nil {
					peer.TrySend(StateChannel, MustEncode(&VoteSetMaj23Message{
						Height:  prs.Height,
						Round:   commit.Round,
						Type:    prototypes.CommitType,
						BlockID: commit.BlockID,
					}))
					time.Sleep(conR.conS.config.PeerQueryMaj23SleepDuration)
				}
			}
		}

		time.Sleep(conR.conS.config.PeerQueryMaj23SleepDuration)

		continue OUTER_LOOP
	}
}

// peerStatsRoutine 用来时刻更新 peer 的状态
func (conR *Reactor) peerStatsRoutine() {
	// 不停地循环，作为一个 goroutine 运行
	for {
		if !conR.IsRunning() {
			// 如果 Reactor 不在运行，则返回 goroutine
			conR.Logger.Infow("Stopping peerStatsRoutine")
			return
		}

		select {
		case msg := <-conR.conS.statsMsgQueue:
			// 从共识状态机的 statsMsgQueue 中获取新消息，消息中包含 peer 的 id，
			// 根据 peer 的 id 从 p2p 的 Switch 中获取 peer
			peer := conR.Switch.Peers().Get(msg.PeerID)
			if peer == nil {
				conR.Logger.Debugw("Attempt to update stats for non-existent peer", "peer", msg.PeerID)
				continue
			}
		case <-conR.conS.Quit():
			// 如果共识状态机退出了，则退出该 goroutine
			return

		case <-conR.Quit():
			return
		}
	}
}

func (conR *Reactor) String() string {
	// better not to access shared variables
	return "ConsensusReactor" // conR.StringIndented("")
}

// StringIndented returns an indented string representation of the Reactor
func (conR *Reactor) StringIndented(indent string) string {
	s := "ConsensusReactor{\n"
	s += indent + "  " + conR.conS.StringIndented(indent+"  ") + "\n"
	for _, peer := range conR.Switch.Peers().List() {
		ps, ok := peer.Get(types.PeerStateKey).(*PeerState)
		if !ok {
			panic(fmt.Sprintf("Peer %v has no state", peer))
		}
		s += indent + "  " + ps.StringIndented(indent+"  ") + "\n"
	}
	s += indent + "}"
	return s
}

//-----------------------------------------------------------------------------


// PeerState contains the known state of a peer, including its connection and
// threadsafe access to its PeerRoundState.
// NOTE: THIS GETS DUMPED WITH rpc/core/consensus.go.
// Be mindful of what you Expose.
type PeerState struct {
	peer   *gossip.Peer
	logger srlog.CRLogger

	mtx   sync.Mutex      // NOTE: Modify below using setters, never directly.
	PRS   PeerRoundState  `json:"round_state"` // Exposed.
	Stats *peerStateStats `json:"stats"`       // Exposed.
}

// peerStateStats holds internal statistics for a peer.
type peerStateStats struct {
	Votes      int `json:"votes"`
	BlockParts int `json:"block_parts"`
}

func (pss peerStateStats) String() string {
	return fmt.Sprintf("peerStateStats{votes: %d, blockParts: %d}",
		pss.Votes, pss.BlockParts)
}

// NewPeerState returns a new PeerState for the given Peer
func NewPeerState(peer *gossip.Peer) *PeerState {
	return &PeerState{
		peer:   peer,
		logger: srlog.NewCRLogger("info"),
		PRS: PeerRoundState{
			Round:              -1,
			ProposalPOLRound:   -1,
			LastCommitRound:    -1,
			CatchupCommitRound: -1,
		},
		Stats: &peerStateStats{},
	}
}

// SetLogger allows to set a logger on the peer state. Returns the peer state
// itself.
func (ps *PeerState) SetLogger(logger srlog.CRLogger) *PeerState {
	ps.logger = logger
	return ps
}

// GetRoundState returns an shallow copy of the PeerRoundState.
// There's no point in mutating it since it won't change PeerState.
func (ps *PeerState) GetRoundState() *PeerRoundState {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	prs := ps.PRS // copy
	return &prs
}

// ToJSON returns a json of PeerState.
func (ps *PeerState) ToJSON() ([]byte, error) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	return srjson.Marshal(ps)
}

func (ps *PeerState) GetHeight() int64 {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return ps.PRS.Height
}

// SetHasProposal sets the given proposal as known for the peer.
func (ps *PeerState) SetHasProposal(proposal *types.PrePrepare) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.PRS.Height != proposal.Height || ps.PRS.Round != proposal.Round {
		return
	}

	if ps.PRS.Proposal {
		return
	}

	ps.PRS.Proposal = true  // 标记一下该peer已经给我们发送给proposal过着我们已经给该peer发送过proposal

	// ps.PRS.PrePrepareBlockParts is set due to NewValidBlockMessage
	if ps.PRS.ProposalBlockParts != nil {
		return
	}

	ps.PRS.ProposalBlockPartSetHeader = proposal.BlockID.PartSetHeader
	ps.PRS.ProposalBlockParts = bits.NewBitArray(int(proposal.BlockID.PartSetHeader.Total))
	ps.PRS.ProposalPOLRound = proposal.POLRound
	ps.PRS.ProposalPOL = nil // Nil until PrePreparePOLMessage received.
}

// InitProposalBlockParts initializes the peer's proposal block parts header and bit array.
func (ps *PeerState) InitProposalBlockParts(partSetHeader types.PartSetHeader) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.PRS.ProposalBlockParts != nil {
		return
	}

	ps.PRS.ProposalBlockPartSetHeader = partSetHeader
	ps.PRS.ProposalBlockParts = bits.NewBitArray(int(partSetHeader.Total))
}

// SetHasProposalBlockPart sets the given block part index as known for the peer.
// SetHasProposalBlockPart 被 gossipDataForCatchup、gossipDataRoutine、Receive 3 个函数调用
func (ps *PeerState) SetHasProposalBlockPart(height int64, round int32, index int) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.PRS.Height != height || ps.PRS.Round != round {
		return
	}

	ps.PRS.ProposalBlockParts.SetIndex(index, true)
}

// PickSendVote picks a vote and sends it to the peer.
// Returns true if vote was sent.
func (ps *PeerState) PickSendVote(votes types.VoteSetReader) bool {
	if vote, ok := ps.PickVoteToSend(votes); ok {
		msg := &VoteMessage{vote}
		ps.logger.Debugw("Sending vote message", "ps", ps, "vote", vote)
		if ps.peer.Send(VoteChannel, MustEncode(msg)) {
			ps.SetHasVote(vote)
			return true
		}
		return false
	}
	return false
}

// PickVoteToSend picks a vote to send to the peer.
// Returns true if a vote was picked.
// NOTE: `votes` must be the correct Size() for the Height().
func (ps *PeerState) PickVoteToSend(votes types.VoteSetReader) (vote *types.Vote, ok bool) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if votes.Size() == 0 {
		return nil, false
	}

	height, round, votesType, size := votes.GetHeight(), votes.GetRound(), prototypes.SignedMsgType(votes.Type()), votes.Size()

	// Lazily set data using 'votes'.
	if votes.IsReply() {
		ps.ensureCatchupCommitRound(height, round, size)
	}
	ps.ensureVoteBitArrays(height, size)

	psVotes := ps.getVoteBitArray(height, round, votesType)
	if psVotes == nil {
		return nil, false // Not something worth sending
	}
	if index, ok := votes.BitArray().Sub(psVotes).PickRandom(); ok {
		return votes.GetByIndex(int32(index)), true
	}
	return nil, false
}

func (ps *PeerState) getVoteBitArray(height int64, round int32, votesType prototypes.SignedMsgType) *bits.BitArray {
	if !types.IsVoteTypeValid(votesType) {
		return nil
	}

	if ps.PRS.Height == height {
		if ps.PRS.Round == round {
			switch votesType {
			case prototypes.PrepareType:
				return ps.PRS.Prevotes
			case prototypes.CommitType:
				return ps.PRS.Precommits
			}
		}
		if ps.PRS.CatchupCommitRound == round {
			switch votesType {
			case prototypes.PrepareType:
				return nil
			case prototypes.CommitType:
				return ps.PRS.CatchupCommit
			}
		}
		if ps.PRS.ProposalPOLRound == round {
			switch votesType {
			case prototypes.PrepareType:
				return ps.PRS.ProposalPOL
			case prototypes.CommitType:
				return nil
			}
		}
		return nil
	}
	if ps.PRS.Height == height+1 {
		if ps.PRS.LastCommitRound == round {
			switch votesType {
			case prototypes.PrepareType:
				return nil
			case prototypes.CommitType:
				return ps.PRS.LastCommit
			}
		}
		return nil
	}
	return nil
}

// 'round': A round for which we have a +2/3 commit.
func (ps *PeerState) ensureCatchupCommitRound(height int64, round int32, numValidators int) {
	if ps.PRS.Height != height {
		return
	}
	/*
		NOTE: This is wrong, 'round' could change.
		e.g. if orig round is not the same as block LastCommits round.
		if ps.CatchupCommitRound != -1 && ps.CatchupCommitRound != round {
			panic(fmt.Sprintf(
				"Conflicting CatchupCommitRound. Height: %v,
				Orig: %v,
				New: %v",
				height,
				ps.CatchupCommitRound,
				round))
		}
	*/
	if ps.PRS.CatchupCommitRound == round {
		return // Nothing to do!
	}
	ps.PRS.CatchupCommitRound = round
	if round == ps.PRS.Round {
		ps.PRS.CatchupCommit = ps.PRS.Precommits
	} else {
		ps.PRS.CatchupCommit = bits.NewBitArray(numValidators)
	}
}

// EnsureVoteBitArrays 确保bit数组已经被分配，用于跟踪该对等体收到的投票
// EnsureVoteBitArrays 仅被 Receive 函数调用
func (ps *PeerState) EnsureVoteBitArrays(height int64, numValidators int) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	ps.ensureVoteBitArrays(height, numValidators)
}

func (ps *PeerState) ensureVoteBitArrays(height int64, numValidators int) {
	if ps.PRS.Height == height {
		if ps.PRS.Prevotes == nil {
			ps.PRS.Prevotes = bits.NewBitArray(numValidators)
		}
		if ps.PRS.Precommits == nil {
			ps.PRS.Precommits = bits.NewBitArray(numValidators)
		}
		if ps.PRS.CatchupCommit == nil {
			ps.PRS.CatchupCommit = bits.NewBitArray(numValidators)
		}
		if ps.PRS.ProposalPOL == nil {
			ps.PRS.ProposalPOL = bits.NewBitArray(numValidators)
		}
	} else if ps.PRS.Height == height+1 {
		if ps.PRS.LastCommit == nil {
			ps.PRS.LastCommit = bits.NewBitArray(numValidators)
		}
	}
}

// RecordVote PeerState.peerStateStats.Votes 自增一，然后返回 PeerState.peerStateStats.Votes，
// PeerState.peerStateStats.Votes 达到了一定数量，表明该 peer 投出了相当数量的 vote，是一个值得信任的
// peer
func (ps *PeerState) RecordVote() int {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	ps.Stats.Votes++

	return ps.Stats.Votes
}

// VotesSent 返回 peer 已经发送给我们投票的区块数量
func (ps *PeerState) VotesSent() int {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	return ps.Stats.Votes
}

// RecordBlockPart PeerState.peerStateStats.BlockParts 先自增一，然后返回 PeerState.peerStateStats.BlockParts，
// PeerState.peerStateStats.BlockParts 达到了一定数量，表明该 peer 是值得信任的
func (ps *PeerState) RecordBlockPart() int {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	ps.Stats.BlockParts++
	return ps.Stats.BlockParts
}

// BlockPartsSent 返回 peer 发送给我们的有用 block part 的数量
func (ps *PeerState) BlockPartsSent() int {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	return ps.Stats.BlockParts
}

// SetHasVote 用来标记peer拥有该vote，SetHasVote 被 PickSendVote 和 Receive 两个函数调用
func (ps *PeerState) SetHasVote(vote *types.Vote) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	ps.setHasVote(vote.Height, vote.Round, vote.Type, vote.ValidatorIndex)
}

func (ps *PeerState) setHasVote(height int64, round int32, voteType prototypes.SignedMsgType, index int32) {
	logger := ps.logger.With("peerH/R", fmt.Sprintf("%d/%d", ps.PRS.Height, ps.PRS.Round), "H/R", fmt.Sprintf("%d/%d", height, round))
	logger.Debugw("setHasVote", "type", voteType, "index", index)

	// NOTE: some may be nil BitArrays -> no side effects.
	psVotes := ps.getVoteBitArray(height, round, voteType)
	if psVotes != nil {
		psVotes.SetIndex(int(index), true)
	}
}

// ApplyNewRoundStepMessage 更新peer的状态，仅在 Receive 函数中被调用
func (ps *PeerState) ApplyNewRoundStepMessage(msg *NewRoundStepMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if CompareHRS(msg.Height, msg.Round, msg.Step, ps.PRS.Height, ps.PRS.Round, ps.PRS.Step) <= 0 {
		return
	}

	// Just remember these values.
	psHeight := ps.PRS.Height
	psRound := ps.PRS.Round
	psCatchupCommitRound := ps.PRS.CatchupCommitRound
	psCatchupCommit := ps.PRS.CatchupCommit

	startTime := srtime.Now().Add(-1 * time.Duration(msg.SecondsSinceStartTime) * time.Second)
	ps.PRS.Height = msg.Height
	ps.PRS.Round = msg.Round
	ps.PRS.Step = msg.Step
	ps.PRS.StartTime = startTime
	if psHeight != msg.Height || psRound != msg.Round {
		ps.PRS.Proposal = false
		ps.PRS.ProposalBlockPartSetHeader = types.PartSetHeader{}
		ps.PRS.ProposalBlockParts = nil
		ps.PRS.ProposalPOLRound = -1
		ps.PRS.ProposalPOL = nil
		// We'll update the BitArray capacity later.
		ps.PRS.Prevotes = nil
		ps.PRS.Precommits = nil
	}
	if psHeight == msg.Height && psRound != msg.Round && msg.Round == psCatchupCommitRound {
		ps.PRS.Precommits = psCatchupCommit
	}
	if psHeight != msg.Height {
		// Shift Commits to LastCommits.
		if psHeight+1 == msg.Height && psRound == msg.LastCommitRound {
			ps.PRS.LastCommitRound = msg.LastCommitRound
			ps.PRS.LastCommit = ps.PRS.Precommits
		} else {
			ps.PRS.LastCommitRound = msg.LastCommitRound
			ps.PRS.LastCommit = nil
		}
		// We'll update the BitArray capacity later.
		ps.PRS.CatchupCommitRound = -1
		ps.PRS.CatchupCommit = nil
	}
}

// ApplyNewValidBlockMessage updates the peer state for the new valid block.
// ApplyNewValidBlockMessage 仅被 Receive 函数调用
func (ps *PeerState) ApplyNewValidBlockMessage(msg *NewValidBlockMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.PRS.Height != msg.Height {
		return
	}

	if ps.PRS.Round != msg.Round && !msg.IsCommit {
		return
	}

	ps.PRS.ProposalBlockPartSetHeader = msg.BlockPartSetHeader
	ps.PRS.ProposalBlockParts = msg.BlockParts
}

// ApplyProposalPOLMessage updates the peer state for the new proposal POL.
// ApplyProposalPOLMessage 仅被 Receive 函数调用
func (ps *PeerState) ApplyProposalPOLMessage(msg *PrePreparePOLMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.PRS.Height != msg.Height {
		return
	}
	if ps.PRS.ProposalPOLRound != msg.PrePreparePOLRound {
		return
	}

	// We might have sent some prevotes in the meantime.
	ps.PRS.ProposalPOL = msg.PrePreparePOL
}

// ApplyHasVoteMessage updates the peer state for the new vote.
// ApplyHasVoteMessage 仅被 Receive 函数调用
func (ps *PeerState) ApplyHasVoteMessage(msg *HasVoteMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.PRS.Height != msg.Height {
		return
	}

	ps.setHasVote(msg.Height, msg.Round, msg.Type, msg.Index)
}

// ApplyVoteSetBitsMessage updates the peer state for the bit-array of votes
// it claims to have for the corresponding BlockID.
// `ourVotes` is a BitArray of votes we have for msg.BlockID
// NOTE: if ourVotes is nil (e.g. msg.Height < rs.Height),
// we conservatively overwrite ps's votes w/ msg.Votes.
func (ps *PeerState) ApplyVoteSetBitsMessage(msg *VoteSetBitsMessage, ourVotes *bits.BitArray) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	votes := ps.getVoteBitArray(msg.Height, msg.Round, msg.Type)
	if votes != nil {
		if ourVotes == nil {
			votes.Update(msg.Votes)
		} else {
			otherVotes := votes.Sub(ourVotes)
			hasVotes := otherVotes.Or(msg.Votes)
			votes.Update(hasVotes)
		}
	}
}

// String returns a string representation of the PeerState
func (ps *PeerState) String() string {
	return ps.StringIndented("")
}

// StringIndented returns a string representation of the PeerState
func (ps *PeerState) StringIndented(indent string) string {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return fmt.Sprintf(`PeerState{
%s  Key        %v
%s  RoundState %v
%s  Stats      %v
%s}`,
		indent, ps.peer.ID(),
		indent, ps.PRS.StringIndented(indent+"  "),
		indent, ps.Stats,
		indent)
}

//-----------------------------------------------------------------------------
// Messages

// Message is a message that can be sent and received on the Reactor
type Message interface {
	ValidateBasic() error
}

func init() {
	srjson.RegisterType(&NewRoundStepMessage{}, "github.com/232425wxy/BFT/NewRoundStepMessage")
	srjson.RegisterType(&NewValidBlockMessage{}, "github.com/232425wxy/BFT/NewValidBlockMessage")
	srjson.RegisterType(&PrePrepareMessage{}, "github.com/232425wxy/BFT/PrePrepare")
	srjson.RegisterType(&PrePreparePOLMessage{}, "github.com/232425wxy/BFT/PrePreparePOL")
	srjson.RegisterType(&BlockPartMessage{}, "github.com/232425wxy/BFT/BlockPart")
	srjson.RegisterType(&VoteMessage{}, "github.com/232425wxy/BFT/Vote")
	srjson.RegisterType(&HasVoteMessage{}, "github.com/232425wxy/BFT/HasVote")
	srjson.RegisterType(&VoteSetMaj23Message{}, "github.com/232425wxy/BFT/VoteSetMaj23")
	srjson.RegisterType(&VoteSetBitsMessage{}, "github.com/232425wxy/BFT/VoteSetBits")
}

func decodeMsg(bz []byte) (msg Message, err error) {
	pb := &protoconsensus.Message{}
	if err = proto.Unmarshal(bz, pb); err != nil {
		return msg, err
	}

	return MsgFromProto(pb)
}

//-------------------------------------

// NewRoundStepMessage is sent for every step taken in the ConsensusState.
// For every height/round/step transition
type NewRoundStepMessage struct {
	Height                int64
	Round                 int32
	Step                  RoundStepType
	SecondsSinceStartTime int64
	LastCommitRound       int32
}

// ValidateBasic performs basic validation.
func (m *NewRoundStepMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	if m.Round < 0 {
		return errors.New("negative Round")
	}
	if !m.Step.IsValid() {
		return errors.New("invalid Step")
	}

	// NOTE: SecondsSinceStartTime may be negative

	// LastCommitRound will be -1 for the initial height, but we don't know what height this is
	// since it can be specified in genesis. The reactor will have to validate this via
	// ValidateHeight().
	if m.LastCommitRound < -1 {
		return errors.New("invalid LastCommitRound (cannot be < -1)")
	}

	return nil
}

// ValidateHeight 用我们本地存储的区块初始高度与对方的参数进行比较，ValidateHeight 仅被 Receive 函数调用
func (m *NewRoundStepMessage) ValidateHeight(initialHeight int64) error {
	if m.Height < initialHeight {
		return fmt.Errorf("invalid Height %v (lower than initial height %v)",
			m.Height, initialHeight)
	}
	if m.Height == initialHeight && m.LastCommitRound != -1 {
		return fmt.Errorf("invalid LastCommitRound %v (must be -1 for initial height %v)",
			m.LastCommitRound, initialHeight)
	}
	if m.Height > initialHeight && m.LastCommitRound < 0 {
		return fmt.Errorf("LastCommitRound can only be negative for initial height %v", // nolint
			initialHeight)
	}
	return nil
}

// String returns a string representation.
func (m *NewRoundStepMessage) String() string {
	return fmt.Sprintf("[NewRoundStep H:%v R:%v S:%v LCR:%v]",
		m.Height, m.Round, m.Step, m.LastCommitRound)
}

//-------------------------------------

// NewValidBlockMessage is sent when a validator observes a valid block B in some round r,
// i.e., there is a PrePrepare for block B and 2/3+ prevotes for the block B in the round r.
// In case the block is also committed, then IsCommit flag is set to true.
type NewValidBlockMessage struct {
	Height             int64
	Round              int32
	BlockPartSetHeader types.PartSetHeader
	BlockParts         *bits.BitArray
	IsCommit           bool
}

// ValidateBasic performs basic validation.
func (m *NewValidBlockMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	if m.Round < 0 {
		return errors.New("negative Round")
	}
	if err := m.BlockPartSetHeader.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong BlockPartSetHeader: %v", err)
	}
	if m.BlockParts.Size() == 0 {
		return errors.New("empty blockParts")
	}
	if m.BlockParts.Size() != int(m.BlockPartSetHeader.Total) {
		return fmt.Errorf("blockParts bit array size %d not equal to BlockPartSetHeader.Total %d",
			m.BlockParts.Size(),
			m.BlockPartSetHeader.Total)
	}
	if m.BlockParts.Size() > int(types.MaxBlockPartsCount) {
		return fmt.Errorf("blockParts bit array is too big: %d, max: %d", m.BlockParts.Size(), types.MaxBlockPartsCount)
	}
	return nil
}

// String returns a string representation.
func (m *NewValidBlockMessage) String() string {
	return fmt.Sprintf("[ValidBlockMessage H:%v R:%v BP:%v BA:%v IsReply:%v]",
		m.Height, m.Round, m.BlockPartSetHeader, m.BlockParts, m.IsCommit)
}

//-------------------------------------

// PrePrepareMessage 用来包装 proposal，当新的区块被提出来的时候会构建此消息
type PrePrepareMessage struct {
	PrePrepare *types.PrePrepare
}

// ValidateBasic performs basic validation.
func (m *PrePrepareMessage) ValidateBasic() error {
	return m.PrePrepare.ValidateBasic()
}

// String returns a string representation.
func (m *PrePrepareMessage) String() string {
	return fmt.Sprintf("[PrePrepare %v]", m.PrePrepare)
}

//-------------------------------------

// PrePreparePOLMessage is sent when a previous proposal is re-proposed.
type PrePreparePOLMessage struct {
	Height             int64
	PrePreparePOLRound int32
	PrePreparePOL      *bits.BitArray
}

// ValidateBasic performs basic validation.
func (m *PrePreparePOLMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	if m.PrePreparePOLRound < 0 {
		return errors.New("negative PrePreparePOLRound")
	}
	if m.PrePreparePOL.Size() == 0 {
		return errors.New("empty PrePreparePOL bit array")
	}
	if m.PrePreparePOL.Size() > types.MaxVotesCount {
		return fmt.Errorf("proposalPOL bit array is too big: %d, max: %d", m.PrePreparePOL.Size(), types.MaxVotesCount)
	}
	return nil
}

// String returns a string representation.
func (m *PrePreparePOLMessage) String() string {
	return fmt.Sprintf("[PrePreparePOL H:%v POLR:%v POL:%v]", m.Height, m.PrePreparePOLRound, m.PrePreparePOL)
}

//-------------------------------------

// BlockPartMessage is sent when gossipping a piece of the proposed block.
type BlockPartMessage struct {
	Height int64
	Round  int32
	Part   *types.Part
}

// ValidateBasic performs basic validation.
func (m *BlockPartMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	if m.Round < 0 {
		return errors.New("negative Round")
	}
	if err := m.Part.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong Part: %v", err)
	}
	return nil
}

// String returns a string representation.
func (m *BlockPartMessage) String() string {
	return fmt.Sprintf("[BlockPart H:%v R:%v P:%v]", m.Height, m.Round, m.Part)
}

//-------------------------------------

// VoteMessage is sent when voting for a proposal (or lack thereof).
type VoteMessage struct {
	Vote *types.Vote
}

// ValidateBasic performs basic validation.
func (m *VoteMessage) ValidateBasic() error {
	return m.Vote.ValidateBasic()
}

// String returns a string representation.
func (m *VoteMessage) String() string {
	return fmt.Sprintf("[Vote %v]", m.Vote)
}

//-------------------------------------

// HasVoteMessage is sent to indicate that a particular vote has been received.
type HasVoteMessage struct {
	Height int64
	Round int32
	Type  prototypes.SignedMsgType
	Index int32
}

// ValidateBasic performs basic validation.
func (m *HasVoteMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	if m.Round < 0 {
		return errors.New("negative Round")
	}
	if !types.IsVoteTypeValid(m.Type) {
		return errors.New("invalid Type")
	}
	if m.Index < 0 {
		return errors.New("negative Index")
	}
	return nil
}

// String returns a string representation.
func (m *HasVoteMessage) String() string {
	return fmt.Sprintf("[HasVote VI:%v V:{%v/%02d/%v}]", m.Index, m.Height, m.Round, m.Type)
}

//-------------------------------------

// VoteSetMaj23Message is sent to indicate that a given BlockID has seen +2/3 votes.
type VoteSetMaj23Message struct {
	Height  int64
	Round   int32
	Type    prototypes.SignedMsgType
	BlockID types.BlockID
}

// ValidateBasic performs basic validation.
func (m *VoteSetMaj23Message) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	if m.Round < 0 {
		return errors.New("negative Round")
	}
	if !types.IsVoteTypeValid(m.Type) {
		return errors.New("invalid Type")
	}
	if err := m.BlockID.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong BlockID: %v", err)
	}
	return nil
}

// String returns a string representation.
func (m *VoteSetMaj23Message) String() string {
	return fmt.Sprintf("[VSM23 %v/%02d/%v %v]", m.Height, m.Round, m.Type, m.BlockID)
}

//-------------------------------------

// VoteSetBitsMessage is sent to communicate the bit-array of votes seen for the BlockID.
type VoteSetBitsMessage struct {
	Height  int64
	Round   int32
	Type    prototypes.SignedMsgType
	BlockID types.BlockID
	Votes   *bits.BitArray
}

// ValidateBasic performs basic validation.
func (m *VoteSetBitsMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	if !types.IsVoteTypeValid(m.Type) {
		return errors.New("invalid Type")
	}
	if err := m.BlockID.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong BlockID: %v", err)
	}
	// NOTE: Votes.Size() can be zero if the node does not have any
	if m.Votes.Size() > types.MaxVotesCount {
		return fmt.Errorf("votes bit array is too big: %d, max: %d", m.Votes.Size(), types.MaxVotesCount)
	}
	return nil
}

// String returns a string representation.
func (m *VoteSetBitsMessage) String() string {
	return fmt.Sprintf("[VSB %v/%02d/%v %v %v]", m.Height, m.Round, m.Type, m.BlockID, m.Votes)
}

//-------------------------------------
