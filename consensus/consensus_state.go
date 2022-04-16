package consensus

import (
	cfg "github.com/232425wxy/BFT/config"
	"github.com/232425wxy/BFT/crypto"
	"github.com/232425wxy/BFT/gossip"
	srevents "github.com/232425wxy/BFT/libs/events"
	srjson "github.com/232425wxy/BFT/libs/json"
	"github.com/232425wxy/BFT/libs/log"
	srmath "github.com/232425wxy/BFT/libs/math"
	sros "github.com/232425wxy/BFT/libs/os"
	srrand "github.com/232425wxy/BFT/libs/rand"
	"github.com/232425wxy/BFT/libs/service"
	srtime "github.com/232425wxy/BFT/libs/time"
	types2 "github.com/232425wxy/BFT/proto/types"
	sm "github.com/232425wxy/BFT/state"
	"github.com/232425wxy/BFT/types"
	"bytes"
	"errors"
	"fmt"
	"github.com/gogo/protobuf/proto"
	"io/ioutil"
	"os"
	"runtime/debug"
	"sync"
	"time"
)

// 共识过程中可能出现的错误
var (
	ErrInvalidProposalSignature   = errors.New("error invalid proposal signature")
	ErrInvalidProposalPOLRound    = errors.New("error invalid proposal POL round")
	ErrAddingVote                 = errors.New("error adding vote")

	errPubKeyIsNotSet = errors.New("pubkey is not set. Look for \"Can't get private validator pubkey\" errors")
)

var msgQueueSize = 10000

// msgs from the reactor which may update the state
type msgInfo struct {
	// Message 是一个只含有 ValidateBasic() error 方法的接口，
	// 实现该接口的实体有：Block、BlockID、Header、Reply、Proof、Part 等等
	Msg    Message   `json:"msg"`
	PeerID gossip.ID `json:"peer_key"`
}

// internally generated messages which may update the state
type timeoutInfo struct {
	Duration time.Duration                `json:"duration"`
	Height   int64                        `json:"height"`
	Round int32         `json:"round"`
	Step  RoundStepType `json:"step"`
}

func (ti *timeoutInfo) String() string {
	return fmt.Sprintf("timeoutInfo(%v ; %d/%d %v)", ti.Duration, ti.Height, ti.Round, ti.Step)
}

// txNotifier 定义的 TxsAvailable() 方法被 CListMempool 实现了
type txNotifier interface {
	TxsAvailable() <-chan struct{}
}

// State 负责处理共识算法的执行部分，它处理 vote 和 proposal，并在达成一致共识后，将新区块
// 追加到区块链上，并在应用程序上执行交易。
// 内部状态机接收来自对等点、内部 validator 和计时器的输入
type State struct {
	service.BaseService

	// 共识算法的配置参数
	config *cfg.ConsensusConfig

	// 用来对 vote 进行签名的 validator
	privValidator types.PrivValidator

	// 用来存储区块和 commits 的
	blockStore sm.BlockStore

	// 创建和执行 blocks
	blockExec *sm.BlockExecutor

	// 用来提醒我们有可用的合法交易
	txNotifier txNotifier

	mtx sync.RWMutex

	// 内部状态
	RoundState
	state sm.State // State until height-1.

	// privValidator 的公钥, 在一个块的时间内记住，以避免对HSM的额外请求
	privValidatorPubKey crypto.PubKey

	// state 的状态可能会因为新消息的到来而被改变，这些消息来自于对等节点 peers、自己或者因为超时产生的消息
	peerMsgQueue     chan msgInfo
	internalMsgQueue chan msgInfo
	timeoutTicker    *TimeoutTicker

	// 待添加的 vote 和 区块 Part 可被推送到该通道里，可以被 reactor 计算
	statsMsgQueue chan msgInfo

	// 我们使用 eventBus 在 reactor 中触发消息广播，并通知外部订阅者，例如。通过websocket
	eventBus *types.EventBus

	// 预写日志可以帮助我们从故障中恢复到正常状态，并且可以避免给冲突的 vote 签名
	wal          WAL
	replayMode   bool // so we don't log signing errors during replay
	doWALCatchup bool // 决定了我们是否能迎头赶上

	// closed when we finish shutting down
	done chan struct{}

	// 在共识状态和反应器之间同步 pubsub，state 只会触发 EventNewRoundStep 和 EventVote
	evsw srevents.EventSwitch

	t *time.Timer

	waitNextEvaluation int
}

// StateOption sets an optional parameter on the State.
type StateOption func(*State)

// NewState 返回一个新的共识状态机实例 State.
func NewState(
	config *cfg.ConsensusConfig,
	state sm.State,
	blockExec *sm.BlockExecutor,
	blockStore sm.BlockStore,
	txNotifier txNotifier,
	options ...StateOption,
) *State {
	// cs : consensus state
	cs := &State{
		config:           config,
		blockExec:        blockExec,
		blockStore:       blockStore,
		txNotifier:       txNotifier,
		peerMsgQueue:     make(chan msgInfo, msgQueueSize), // peerMsgQueue 一次最多可以存储 1000 条 msgInfo
		internalMsgQueue: make(chan msgInfo, msgQueueSize), // internalMsgQueue 一次最多可以存储 1000 条 msgInfo
		timeoutTicker:    NewTimeoutTicker(),
		statsMsgQueue:    make(chan msgInfo, msgQueueSize), // statsMsgQueue 一次最多可以存储 1000 条 msgInfo
		done:             make(chan struct{}),
		doWALCatchup:     true,     // 能迎头赶上
		wal:              nilWAL{}, // 一个空的什么也实现不了的 wal
		evsw:             srevents.NewEventSwitch(),
		t:                time.NewTimer(100 * time.Millisecond),
		waitNextEvaluation: 10,
	}

	// We have no votes, so reconstruct LastCommits from SeenCommit.
	if state.LastBlockHeight > 0 {
		cs.reconstructLastCommit(state)
	}

	cs.updateToState(state)

	// NOTE: we do not call scheduleRound0 yet, we do that upon Start()

	cs.BaseService = *service.NewBaseService(log.NewCRLogger("info").With("module", "consensus-state"), "State", cs)
	for _, option := range options {
		option(cs)
	}

	return cs
}

// SetEventBus sets event bus.
func (cs *State) SetEventBus(b *types.EventBus) {
	cs.eventBus = b
	cs.blockExec.SetEventBus(b)
}

// String returns a string.
func (cs *State) String() string {
	// better not to access shared variables
	return "ConsensusState"
}

// GetRoundStateJSON returns a json of RoundState.
func (cs *State) GetRoundStateJSON() ([]byte, error) {
	cs.mtx.RLock()
	defer cs.mtx.RUnlock()
	return srjson.Marshal(cs.RoundState)
}

// GetRoundStateSimpleJSON returns a json of RoundStateSimple
func (cs *State) GetRoundStateSimpleJSON() ([]byte, error) {
	cs.mtx.RLock()
	defer cs.mtx.RUnlock()
	return srjson.Marshal(cs.RoundState.RoundStateSimple(cs.state.LastBlockID, cs.Round))
}

// SetPrivValidator sets the private validator account for signing votes. It
// immediately requests pubkey and caches it.
func (cs *State) SetPrivValidator(priv types.PrivValidator) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	cs.privValidator = priv

	if err := cs.updatePrivValidatorPubKey(); err != nil {
		cs.Logger.Errorw("Failed to get private validator pubkey", "err", err)
	}
}

// LoadCommit 从数据库里加载指定高度区块的commit
func (cs *State) LoadCommit(height int64) *types.Reply {
	cs.mtx.RLock()
	defer cs.mtx.RUnlock()

	if height == cs.blockStore.Height() {
		return cs.blockStore.LoadSeenReply(height)
	}

	return cs.blockStore.LoadBlockReply(height)
}

func (cs *State) OnStart() error {
	// 从硬盘里面加载 WAL 数据
	if _, ok := cs.wal.(nilWAL); ok {
		if err := cs.loadWalFile(); err != nil {
			return err
		}
	}

	if cs.doWALCatchup {
		repairAttempted := false

	LOOP:
		for {
			err := cs.catchupReplay(cs.Height)
			switch {
			case err == nil:
				break LOOP

			case !IsDataCorruptionError(err):
				cs.Logger.Errorw("Error on catchup replay; proceeding to start state anyway", "err", err)
				break LOOP

			case repairAttempted:
				return err
			}

			cs.Logger.Errorw("The WAL file is corrupted; attempting repair", "err", err)

			if err := cs.wal.Stop(); err != nil {
				return err
			}

			repairAttempted = true

			corruptedFile := fmt.Sprintf("%s.CORRUPTED", cs.config.WalFile())
			if err := sros.CopyFile(cs.config.WalFile(), corruptedFile); err != nil {
				return err
			}

			cs.Logger.Debugw("Backed up WAL file", "src", cs.config.WalFile(), "dst", corruptedFile)

			// 3) try to repair (WAL file will be overwritten!)
			if err := repairWalFile(corruptedFile, cs.config.WalFile()); err != nil {
				cs.Logger.Errorw("The WAL repair failed", "err", err)
				return err
			}

			cs.Logger.Infow("Repair WAL successfully")

			// reload WAL file
			if err := cs.loadWalFile(); err != nil {
				return err
			}
		}
	}

	if err := cs.evsw.Start(); err != nil {
		return err
	}

	// 开启超时计时器装置
	if err := cs.timeoutTicker.Start(); err != nil {
		return err
	}

	// 开启接收消息的routine
	go cs.receiveRoutine()

	// 因为自己刚刚启动，或者说刚刚加入区块链系统，所以我们得给自己发送一个开始对下一个区块达成共识的信号，
	// GetRoundState 是线程安全的，使用它来获取共识轮次的状态，不会与 receiveRoutine 之间发生竞争访问
	cs.scheduleRound0()

	return nil
}

// loadWalFile 从硬盘里加载 WAL 数据，覆盖 State.wal，此方法在 OnStart 中被调用
func (cs *State) loadWalFile() error {
	wal, err := cs.OpenWAL(cs.config.WalFile())
	if err != nil {
		cs.Logger.Errorw("Failed to load state WAL", "err", err)
		return err
	}

	cs.wal = wal
	return nil
}

// OnStop implements service.Service.
func (cs *State) OnStop() {
	if err := cs.evsw.Stop(); err != nil {
		cs.Logger.Errorw("Failed trying to stop eventSwitch", "error", err)
	}

	if err := cs.timeoutTicker.Stop(); err != nil {
		cs.Logger.Errorw("Failed trying to stop timeoutTicket", "error", err)
	}
}

// 等待程序退出
func (cs *State) Wait() {
	<-cs.done
}

// OpenWAL 打开一个预写日志用来记录所有共识消息
func (cs *State) OpenWAL(walFile string) (WAL, error) {
	wal, err := NewWAL(walFile)
	if err != nil {
		cs.Logger.Errorw("Failed to open WAL", "file", walFile, "err", err)
		return nil, err
	}

	wal.SetLogger(cs.Logger.With("WAL", walFile))

	if err := wal.Start(); err != nil {
		cs.Logger.Errorw("Failed to start WAL", "err", err)
		return nil, err
	}

	return wal, nil
}

//------------------------------------------------------------
// 用来管理共识状态的内部方法

var lazy = 1000

// updateHeight 仅在 updateToState 方法中被调用
func (cs *State) updateHeight(height int64) {
	cs.Height = height
}

// 更新共识状态的 round 和 step：
//	State.Round = round
//	State.Step = step
// 仅在 updateToState、enterNewRound、enterPrePrepare、enterPrepare、enterPrepareWait、enterPrecommit、enterReply 7个方法中被调用
func (cs *State) updateRoundStep(round int32, step RoundStepType) {
	cs.Round = round
	cs.Step = step
}

// scheduleRound0 的目的是过个大概1s以后，就进入下一个区块高度，
// 该方法仅在 OnStart 和 finalizeAppend 两个方法中被调用
func (cs *State) scheduleRound0() {
	// 一般来说，不管系统是刚启动调用 scheduleRound0 还是commit一个新区块后调用scheduleRound0，
	// cs.StartTime.Sub(srtime.Now()) 差不多都等于1s，原因请看 updateToState 里对 cs.StartTime
	// 的赋值操作
	sleepDuration := cs.StartTime.Sub(srtime.Now())
	cs.scheduleTimeout(sleepDuration, cs.Height, 0, RoundStepNewHeight)
}

// scheduleTimeout 被以下6个方法调用：
//	scheduleRound0、handleTxsAvailable、enterNewRound、enterPrePrepare、enterPrepareWait、enterPrecommitWait
func (cs *State) scheduleTimeout(duration time.Duration, height int64, round int32, step RoundStepType) {
	cs.timeoutTicker.ScheduleTimeout(timeoutInfo{duration, height, round, step})
}

// sendInternalMessage 被以下两个方法调用：
//	1. decidePrePrepare：当提出一个新proposal以后，将proposal送到内部消息通道中
//	2. signAddVote：对某个投票消息进行签名后，将签过名的投票推送到内部消息通道中
func (cs *State) sendInternalMessage(mi msgInfo) {
	select {
	case cs.internalMsgQueue <- mi:
	default:
		go func() { cs.internalMsgQueue <- mi }()
	}
}

// reconstructLastCommit 从状态数据库中加载上次确认区块时产生的precommits，并将其赋值给 State.LastCommits
// reconstructLastCommit 仅被以下两个函数调用：
//	1. NewState  2. SwitchToConsensus
func (cs *State) reconstructLastCommit(state sm.State) {
	seenCommit := cs.blockStore.LoadSeenReply(state.LastBlockHeight)
	if seenCommit == nil {
		panic(fmt.Sprintf("failed to reconstruct last commit; seen commit for height %v not found", state.LastBlockHeight))
	}

	lastPrecommits := types.ReplyToVoteSet(state.ChainID, seenCommit, state.LastValidators)
	if !lastPrecommits.HasTwoThirdsMajority() {
		panic("failed to reconstruct last commit; does not have +2/3 maj")
	}
	cs.LastCommits = lastPrecommits
}

// updateToState 在 SwitchToConsensus、NewState 两个初始情况下被调用，除此以外，updateToState 还会被
// finalizeAppend 方法调用，用来在确认一个区块后更新共识状态
func (cs *State) updateToState(state sm.State) {
	// 1. 如果 updateToState 是在 finalizeAppend 方法中被调用，则此时 cs.ReplyRound > -1，state.LastBlockHeight 等于当前
	//    被 commit 的区块高度，cs.Height 等于当前工作的区块高度，当前工作的区块高度肯定等于当前被 commit 的区块高度，因此
	//    cs.Height 必须等于 state.LastBlockHeight
	// 2. 如果 updateToState 是在 NewState 方法中被调用，则此时 cs.Height = 0，cs.ReplyRound = 0, state.LastBlockHeight >= 0
	// 3. 如果 updateToState 是在 SwitchToConsensus 方法中被调用，则此时 cs.ReplyRound = -1，cs.Height > 0，state.LastBlockHeight >= 0
	if cs.ReplyRound > -1 && 0 < cs.Height && cs.Height != state.LastBlockHeight {
		panic(fmt.Sprintf("updateToState() expected state height of %v but found %v", cs.Height, state.LastBlockHeight))
	}

	// 如果是在 NewState 方法中调用 updateToState 的，则此处 cs.state.IsEmpty() = true
	if !cs.state.IsEmpty() {
		// cs.state 表示的是旧状态，旧状态的 LastBlockHeight 还等于上一个被 commit 的区块高度，因此 cs.state.LastBlockHeight+1
		// 表示的是当前工作的高度，因此必须等于 cs.Height
		if cs.state.LastBlockHeight > 0 && cs.state.LastBlockHeight+1 != cs.Height {
			panic(fmt.Sprintf("inconsistent cs.state.LastBlockHeight+1 %v vs cs.Height %v", cs.state.LastBlockHeight+1, cs.Height))
		}
		if cs.state.LastBlockHeight > 0 && cs.Height == cs.state.InitialHeight {
			panic(fmt.Sprintf("inconsistent cs.state.LastBlockHeight %v, expected 0 for initial height %v", cs.state.LastBlockHeight, cs.state.InitialHeight))
		}

		// 当通过调用 SwitchToConsensus 来执行 updateToState 的时候，此情况会发生
		if state.LastBlockHeight <= cs.state.LastBlockHeight {
			cs.newStep()
			return
		}
	}

	validators := state.Validators

	switch {
	case state.LastBlockHeight == 0:  // 此种情况仅在通过调用 NewState 方法来执行 updateToState 的时候发生
		cs.LastCommits = (*types.VoteSet)(nil)
	case cs.ReplyRound > -1 && cs.Votes != nil:
		if !cs.Votes.Commits(cs.ReplyRound).HasTwoThirdsMajority() {
			panic(fmt.Sprintf("wanted to form a commit, but precommits (H/R: %d/%d) didn't have 2/3+: %v", state.LastBlockHeight, cs.ReplyRound, cs.Votes.Commits(cs.ReplyRound)))
		}

		cs.LastCommits = cs.Votes.Commits(cs.ReplyRound)

	case cs.LastCommits == nil:
		panic(fmt.Sprintf("last commit cannot be empty after initial block (H:%d)", state.LastBlockHeight+1))
	}

	height := state.LastBlockHeight + 1
	if height == 1 {
		height = state.InitialHeight
	}

	// 更新自己所在的工作高度
	cs.updateHeight(height)

	// 成功确认一个区块后，或者初始化后进入一个新的区块高度，从外循环开始（想象博客里的那张图）
	cs.updateRoundStep(0, RoundStepNewHeight)

	// 在系统初始化的时候，CommitTime 是零值，只有在commit一个新的区块时，CommitTime等于commit区块时的时间
	if cs.CommitTime.IsZero() {
		// 之前还没有commit过一个区块
		// 此处等于 1s + now
		cs.StartTime = cs.config.Commit(srtime.Now())
	} else {
		// 上一次commit区块的时间加上1s
		cs.StartTime = cs.config.Commit(cs.CommitTime)
	}

	cs.Validators = validators
	cs.PrePrepare = nil
	cs.PrePrepareBlock = nil
	cs.PrePrepareBlockParts = nil
	cs.LockedRound = -1
	cs.LockedBlock = nil
	cs.LockedBlockParts = nil
	cs.ValidRound = -1
	cs.ValidBlock = nil
	cs.ValidBlockParts = nil
	cs.Votes = NewHeightVoteSet(state.ChainID, height, validators)
	cs.ReplyRound = -1
	cs.LastValidators = state.LastValidators
	cs.TriggeredTimeoutCommit = false
	cs.state = state

	cs.newStep()
}

// newStep 将 RoundState 代表的 height、round 和 step 写入到预写日志里，
// 如果 State.eventBus 不为 nil，则发布 EventNewRoundStep 事件
// newStep 可被以下7个函数调用：
//	updateToState、enterPrePrepare、enterPrepare、enterPrepareWait、enterPrecommit、enterPrecommitWait、enterReply
func (cs *State) newStep() {
	// 将 types.EventDataRoundState 写入到预写日志里
	rs := cs.RoundStateEvent()
	if err := cs.wal.Write(rs); err != nil {
		cs.Logger.Errorw("Failed writing to WAL", "err", err)
	}

	if cs.eventBus != nil {
		if err := cs.eventBus.PublishEventNewRoundStep(rs); err != nil {
			cs.Logger.Errorw("Failed publishing new round step", "err", err)
		}

		cs.evsw.FireEvent(types.EventNewRoundStep, &cs.RoundState)
	}
}

//-----------------------------------------
// the commands go routines

// receiveRoutine 时刻准备接收以下4种消息：
//	1. 内存池中有可用交易数据的提示消息
//	2. 来自于其他节点发送过来的共识消息
//	3. 来自本地内部的消息
//	4. 超时消息
func (cs *State) receiveRoutine() {
	// onExit 就是退出共识状态机的意思
	onExit := func(cs *State) {
		if err := cs.wal.Stop(); err != nil {
			cs.Logger.Errorw("Failed trying to stop WAL", "error", err)
		}

		cs.wal.Wait()
		close(cs.done)
	}

	defer func() {
		if r := recover(); r != nil {
			cs.Logger.Errorw("CONSENSUS FAILURE!!!!!!!", "err", r, "stack", string(debug.Stack()))
			panic(string(debug.Stack()))
		}
	}()

	for {
		var mi msgInfo

		select {
		case <-cs.txNotifier.TxsAvailable():
			cs.handleTxsAvailable()

		case mi = <-cs.peerMsgQueue:
			if err := cs.wal.Write(mi); err != nil {
				cs.Logger.Errorw("Failed writing to WAL", "err", err)
			}
			cs.handleMsg(mi)

		case mi = <-cs.internalMsgQueue:
			err := cs.wal.WriteSync(mi) // NOTE: fsync
			if err != nil {
				panic(fmt.Sprintf("failed to write %v msg to consensus WAL due to %v; check your file system and restart the node", mi, err))
			}

			if _, ok := mi.Msg.(*VoteMessage); ok {
			}
			cs.handleMsg(mi)

		case ti := <-cs.timeoutTicker.Chan():
			if err := cs.wal.Write(ti); err != nil {
				cs.Logger.Errorw("Failed writing to WAL", "err", err)
			}
			cs.handleTimeout(ti, cs.RoundState)

		case <-cs.Quit():
			onExit(cs)
			return
		}
	}
}

// handleMsg 被 receiveRoutine 和 readReplayMessage 两个函数调用
// handleMsg 主要处理以下3种消息：
//	1. proposal消息   2. BlockPart消息   3. vote消息
func (cs *State) handleMsg(mi msgInfo) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	var (
		added bool
		err   error
	)

	msg, peerID := mi.Msg, mi.PeerID

	switch msg := msg.(type) {
	case *PrePrepareMessage:
		err = cs.setProposal(msg.PrePrepare)

	case *BlockPartMessage:
		added, err = cs.addProposalBlockPart(msg, peerID)
		if added {
			cs.statsMsgQueue <- mi
		}

		if err != nil && msg.Round != cs.Round {
			cs.Logger.Errorw("Received block part from wrong round", "height", cs.Height, "cs_round", cs.Round, "block_round", msg.Round)
			err = nil
		}

	case *VoteMessage:
		// 尝试添加投票，因为该投票可能是一个错误的投票，例如重复投票、矛盾投票等
		added, err = cs.tryAddVote(msg.Vote, peerID)
		if added {
			cs.statsMsgQueue <- mi
		}

	default:
		cs.Logger.Errorw("Unknown msg type", "type", fmt.Sprintf("%T", msg))
		return
	}
}

var start = srtime.Now()

// handleTimeout 被 receiveRoutine 和 readReplayMessage 两个函数调用
func (cs *State) handleTimeout(ti timeoutInfo, rs RoundState) {
	if ti.Height != rs.Height || ti.Round < rs.Round || (ti.Round == rs.Round && ti.Step < rs.Step) {
		return
	}

	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	switch ti.Step {
	case RoundStepNewHeight:
		start = srtime.Now()
		cs.enterNewRound(ti.Height, 0)  // 进入下一个区块高度

	case RoundStepNewRound:
		cs.enterPrePrepare(ti.Height, 0) // 进入propose阶段

	case RoundStepPrePrepare:
		if err := cs.eventBus.PublishEventTimeoutPropose(cs.RoundStateEvent()); err != nil {
			cs.Logger.Errorw("Failed publishing timeout pre_prepare", "err", err)
		}
		cs.enterPrepare(ti.Height, ti.Round)

	case RoundStepPrepareWait:
		if err := cs.eventBus.PublishEventTimeoutWait(cs.RoundStateEvent()); err != nil {
			cs.Logger.Errorw("Failed publishing prepare wait", "err", err)
		}

		cs.enterPrecommit(ti.Height, ti.Round)

	case RoundStepCommitWait:
		if err := cs.eventBus.PublishEventTimeoutWait(cs.RoundStateEvent()); err != nil {
			cs.Logger.Errorw("Failed publishing commit wait", "err", err)
		}

		cs.enterPrecommit(ti.Height, ti.Round)
		cs.enterNewRound(ti.Height, ti.Round+1)

	default:
		panic(fmt.Sprintf("invalid timeout step: %v", ti.Step))
	}

}

// handleTxsAvailable 在有合法的交易数据时，会检查共识过程所处的阶段：
//	1. 如果处于 RoundStepNewHeight 阶段，会设置一个 timeoutCommit 超时时间计划，一旦超时，
//	   则会立马调用 enterPrePrepare() 方法提出新的区块
//	2. 如果处于 RoundStepNewRound 阶段，则会直接调用 enterPrePrepare() 方法，提出新的区块
func (cs *State) handleTxsAvailable() {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	// 我们仅在 Round 等于 0 的时候去处理合法的交易信息
	if cs.Round != 0 {
		return
	}

	switch cs.Step {
	case RoundStepNewHeight: // timeoutCommit phase
		if cs.needProofBlock(cs.Height) {
			// enterPrePrepare will be called by enterNewRound
			return
		}
		timeoutCommit := cs.StartTime.Sub(srtime.Now()) + 1*time.Millisecond
		cs.scheduleTimeout(timeoutCommit, cs.Height, 0, RoundStepNewRound)
	case RoundStepNewRound: // after timeoutCommit
		cs.enterPrePrepare(cs.Height, 0)
	}
}


func (cs *State) enterNewRound(height int64, round int32) {
	logger := cs.Logger.With("height", height, "round", round)

	if cs.Height != height || round < cs.Round || (cs.Round == round && cs.Step != RoundStepNewHeight) {
		return
	}

	logger.Debugw("Enter new round")

	// increment validators if necessary
	validators := cs.Validators
	if cs.Round < round {  // 在 cs.Round 轮次里共识失败了
		validators = validators.Copy()
		validators.CopyIteratePrimary(cs.state.LastBlockID, round) // 更新validator的优先级，重新选出proposer
	}

	cs.updateRoundStep(round, RoundStepNewRound)
	cs.Validators = validators
	if round == 0 {

	} else {
		// 第一轮共识失败了，重置proposal
		cs.PrePrepare = nil
		cs.PrePrepareBlock = nil
		cs.PrePrepareBlockParts = nil
	}

	cs.Votes.SetRound(srmath.SafeAddInt32(round, 1))
	cs.TriggeredTimeoutCommit = false

	if err := cs.eventBus.PublishEventNewRound(cs.NewRoundEvent(cs.state.LastBlockID, cs.Round)); err != nil {
		cs.Logger.Errorw("Failed publishing new round", "err", err)
	}

	waitForTxs := cs.config.WaitForTxs() && round == 0 && !cs.needProofBlock(height)
	if waitForTxs {
		if cs.config.CreateEmptyBlocksInterval > 0 {
			// 如果 waitForTxs 等于 true，则会等待cs.config.CreateEmptyBlocksInterval这么长时间才进入propose阶段，
			// 但是默认情况下，cs.config.CreateEmptyBlocksInterval等于0
			cs.scheduleTimeout(cs.config.CreateEmptyBlocksInterval, height, round, RoundStepNewRound)
		}
	} else {
		cs.enterPrePrepare(height, round)
	}
}

// needProofBlock 如果给定的区块高度height等于系统初始高度，则 needProofBlock 返回 true，
// 如果共识状态的AppHash不等于height-1时的AppHash，则 needProofBlock 返回true
func (cs *State) needProofBlock(height int64) bool {
	if height == cs.state.InitialHeight {
		return true
	}

	lastBlockMeta := cs.blockStore.LoadBlockMeta(height - 1)
	if lastBlockMeta == nil {
		panic(fmt.Sprintf("needProofBlock: last block meta for height %d not found", height-1))
	}

	return !bytes.Equal(cs.state.AppHash, lastBlockMeta.Header.AppHash)  // 一般情况下，这里都是返回false
}

// enterPrePrepare 确认自己是不是 proposer，是的话则负责提出新的 proposal，proposal 的来源有两种：
//	1. POL 锁定的区块
//	2. 从内存池里获取新的合法交易数据，组成一个新区块
func (cs *State) enterPrePrepare(height int64, round int32) {
	logger := cs.Logger.With("height", height, "round", round)

	if cs.Height != height || round < cs.Round || (cs.Round == round && RoundStepPrePrepare <= cs.Step) {
		return
	}

	logger.Infow("Enter pre_prepare step")

	defer func() {
		cs.updateRoundStep(round, RoundStepPrePrepare)
		cs.newStep()
		if cs.isPrePrepareComplete() {
			// 如果我们已经收到了完整的proposal，则进入prevote阶段
			cs.enterPrepare(height, cs.Round)
		}
	}()

	// 如果round等于0，默认最多等待3秒中进入prevote阶段，如果round等于1，默认最多等待3.5秒进入prevote阶段
	cs.scheduleTimeout(cs.config.Propose(round), height, round, RoundStepPrePrepare)

	if cs.privValidator == nil {
		return
	}


	if cs.privValidatorPubKey == nil {
		return
	}

	address := cs.privValidatorPubKey.Address()

	// 在 validator 集合中查找是否存在自己的地址，如果不存在，则说明自己不是 validator
	if !cs.Validators.HasAddress(address) {
		logger.Infow("PREPREPARE STEP; NOT VALIDATOR", "addr", address)
		return
	}

	// 如果自己是 proposer，那么就要负责发起新的 proposal 了
	if cs.isPrimary(address) {
		_, val := cs.Validators.GetByAddress(address.Bytes())
		logger.Infow("PREPREPARE STEP; PRIMARY IS ME", "PRIMARY", address, "VotingPower", val.VotingPower)
		cs.decidePrePrepare(height, round)
	} else {
		logger.Infow("PREPREPARE STEP; PRIMARY IS NOT ME", "PRIMARY", cs.Validators.GetPrimary(cs.state.LastBlockID, cs.Round).Address)
	}
}

func (cs *State) isPrimary(address []byte) bool {
	return bytes.Equal(cs.Validators.GetPrimary(cs.state.LastBlockID, cs.Round).Address, address)
}

// decidePrePrepare 如果之前 round 中有 POL，那么就以 POL 为 proposal，否则就创建新的 proposal
func (cs *State) decidePrePrepare(height int64, round int32) {
	var block *types.Block
	var blockParts *types.PartSet

	if cs.ValidBlock != nil {
		block, blockParts = cs.ValidBlock, cs.ValidBlockParts
	} else {
		block, blockParts = cs.createPrePrepareBlock() // 否则从内存池里拿到合法交易数据创建新的 block
		if block == nil {
			return
		}
	}

	if err := cs.wal.FlushAndSync(); err != nil {
		cs.Logger.Errorw("Failed flushing WAL to disk")
	}

	// 创建 prePrepare，prePrepare 是由 block 构成的
	propBlockID := types.BlockID{Hash: block.Hash(), PartSetHeader: blockParts.Header()}
	prePrepare := types.NewPrePrepare(height, round, cs.ValidRound, propBlockID)

	// 对proposal进行签名
	p := prePrepare.ToProto()
	if err := cs.privValidator.SignProposal(cs.state.ChainID, p); err == nil {
		prePrepare.Signature = p.Signature
		cs.sendInternalMessage(msgInfo{&PrePrepareMessage{prePrepare}, ""})

		for i := 0; i < int(blockParts.Total()); i++ {
			part := blockParts.GetPart(i)
			cs.sendInternalMessage(msgInfo{&BlockPartMessage{cs.Height, cs.Round, part}, ""})
		}

		cs.Logger.Infow("Signed pre_prepare", "height", height, "round", round, "pre_prepare", prePrepare)
	} else if !cs.replayMode {
		cs.Logger.Errorw("Failed signing pre_prepare", "height", height, "round", round, "err", err)
	}
}

// 如果我们收到了完整的提案则返回true，或者提案中显示POLRound大于0，则表示该提案是过去发起过的，
// 并且已经获得了大多数prevote的支持，则我们返回true
func (cs *State) isPrePrepareComplete() bool {
	if cs.PrePrepare == nil || cs.PrePrepareBlock == nil {
		return false
	}
	if cs.PrePrepare.POLRound < 0 {
		return true
	}
	// 如果cs.PrePrepare.POLRound大于0，表示该proposal是之前cs.PrePrepare.POLRound轮次中被提出过的，
	// 并且已经获得了大多数prevote的支持，那么这些都是proposer告诉我们的，为了验证proposer说法是否正
	// 确，我们还得验证在cs.PrePrepare.POLRound轮次中我们是否收到了足够多的支持proposal的prevote
	return cs.Votes.Prepares(cs.PrePrepare.POLRound).HasTwoThirdsMajority()
}

// createPrePrepareBlock 创建新区块：
//	从内存池里获取合法交易数据，构造区块
func (cs *State) createPrePrepareBlock() (block *types.Block, blockParts *types.PartSet) {
	// 获取上一个区块的commit信息，以此作为构建下一个区块的基础
	var commit *types.Reply
	switch {
	case cs.Height == cs.state.InitialHeight:
		commit = types.NewReply(0, 0, types.BlockID{}, nil)

	case cs.LastCommits.HasTwoThirdsMajority():
		commit = cs.LastCommits.MakeReply()

	default:
		cs.Logger.Errorw("Cannot construct pre_prepare without the previous block")
		return
	}

	proposerAddr := cs.privValidatorPubKey.Address()

	return cs.blockExec.CreateProposalBlock(cs.Height, cs.state, commit, proposerAddr)
}


func (cs *State) enterPrepare(height int64, round int32) {
	logger := cs.Logger.With("height", height, "round", round)

	if cs.Height != height || round < cs.Round || (cs.Round == round && RoundStepPrepare <= cs.Step) {
		return
	}

	defer func() {
		cs.updateRoundStep(round, RoundStepPrepare)
		cs.newStep()
	}()

	logger.Infow("Enter prepare")

	// Sign and broadcast vote as necessary
	cs.doPrepare(height, round)

	// Once `addVote` hits any +2/3 prevotes, we will go to PrevoteWait
	// (so we have more time to try and collect +2/3 prevotes for a single block)
}

// doPrepare 给 proposal 进行 prevote：
//	1. 如果在之前的 round 里有锁定的 proposal，则给该 proposal 投票
//	2. 如果在超时间内没有收到 proposal 或者收到了错误的 proposal，则给 nil 投票
//	3. 给在超时间内收到的合法的 proposal 投票
func (cs *State) doPrepare(height int64, round int32) {
	logger := cs.Logger.With("height", height, "round", round)

	// 如果有 block 被锁定了，就给该 block 投票
	if cs.LockedBlock != nil {
		cs.signAddVote(types2.PrepareType, cs.LockedBlock.Hash(), cs.LockedBlockParts.Header())
		return
	}

	if cs.PrePrepareBlock == nil {
		logger.Errorw("PrePrepareBlock is nil")
		cs.signAddVote(types2.PrepareType, nil, types.PartSetHeader{})
		return
	}

	// 验证收到的 block 是否合法
	err := cs.blockExec.ValidateBlock(cs.state, cs.PrePrepareBlock)
	if err != nil {
		logger.Errorw("PrePrepareBlock is invalid", "err", err)
		cs.signAddVote(types2.PrepareType, nil, types.PartSetHeader{})
		return
	}

	// 给合法的 block 进行投票
	cs.signAddVote(types2.PrepareType, cs.PrePrepareBlock.Hash(), cs.PrePrepareBlockParts.Header())
}

// enterPrepareWait 仅仅被 addVote 方法调用，如果我们收到了大多数prevote投票，则调用 enterPrepareWait
func (cs *State) enterPrepareWait(height int64, round int32) {
	logger := cs.Logger.With("height", height, "round", round)

	if cs.Height != height || round < cs.Round || (cs.Round == round && RoundStepPrepareWait <= cs.Step) {
		logger.Errorw("Enter prepare_wait step with invalid args", "current", fmt.Sprintf("height:%v / round:%v / step:%v", cs.Height, cs.Round, cs.Step))
		return
	}

	logger.Infow("Enter prepare_wait step")

	defer func() {
		cs.updateRoundStep(round, RoundStepPrepareWait)
		cs.newStep()
	}()

	// 如果round等于0，则等待1s，如果round等于1，则等待1.5s
	cs.scheduleTimeout(cs.config.Prevote(round), height, round, RoundStepPrepareWait)
}

func (cs *State) enterPrecommit(height int64, round int32) {
	logger := cs.Logger.With("height", height, "round", round)

	if cs.Height != height || round < cs.Round || (cs.Round == round && RoundStepCommit <= cs.Step) {
		return
	}

	logger.Infow("Enter commit step")

	defer func() {
		cs.updateRoundStep(round, RoundStepCommit)
		cs.newStep()
	}()

	blockID, ok := cs.Votes.Prepares(round).TwoThirdsMajority()

	if !ok {
		cs.signAddVote(types2.CommitType, nil, types.PartSetHeader{})
		return
	}

	if err := cs.eventBus.PublishEventPolka(cs.RoundStateEvent()); err != nil {
		logger.Errorw("Failed publishing polka", "err", err)
	}

	// 既然进入了precommit阶段，则说明至少收到了+2/3个prevote投票
	polRound, _ := cs.Votes.POLInfo()
	if polRound < round {
		panic(fmt.Sprintf("this POLRound should be %v but got %v", round, polRound))
	}

	if len(blockID.Hash) == 0 {  // 超过三分之二的prevote是给nil投票的
		if cs.LockedBlock == nil {
		} else {
			cs.LockedRound = -1
			cs.LockedBlock = nil
			cs.LockedBlockParts = nil

			if err := cs.eventBus.PublishEventUnlock(cs.RoundStateEvent()); err != nil {
				logger.Errorw("Failed publishing event unlock", "err", err)
			}
		}

		cs.signAddVote(types2.CommitType, nil, types.PartSetHeader{})
		return
	}

	// 给之前锁定的区块投票
	if cs.LockedBlock.EqualsTo(blockID.Hash) {
		cs.LockedRound = round

		if err := cs.eventBus.PublishEventRelock(cs.RoundStateEvent()); err != nil {
			logger.Errorw("Failed publishing event relock", "err", err)
		}

		cs.signAddVote(types2.CommitType, blockID.Hash, blockID.PartSetHeader)
		return
	}

	// 给新提出的区块投票
	if cs.PrePrepareBlock.EqualsTo(blockID.Hash) {
		if err := cs.blockExec.ValidateBlock(cs.state, cs.PrePrepareBlock); err != nil {
			panic(fmt.Sprintf("precommit step; +2/3 prevoted for an invalid block: %v", err))
			//return
		}

		cs.LockedRound = round
		cs.LockedBlock = cs.PrePrepareBlock
		cs.LockedBlockParts = cs.PrePrepareBlockParts

		if err := cs.eventBus.PublishEventLock(cs.RoundStateEvent()); err != nil {
			logger.Errorw("Failed publishing event lock", "err", err)
		}

		cs.signAddVote(types2.CommitType, blockID.Hash, blockID.PartSetHeader)
		return
	}

	logger.Errorw("We get +2/3 prepares for a block we do not have; voting nil", "blockID", blockID)
	cs.LockedRound = -1
	cs.LockedBlock = nil
	cs.LockedBlockParts = nil

	if !cs.PrePrepareBlockParts.HeaderEqualsTo(blockID.PartSetHeader) {
		cs.PrePrepareBlock = nil
		cs.PrePrepareBlockParts = types.NewPartSetFromHeader(blockID.PartSetHeader)
	}

	if err := cs.eventBus.PublishEventUnlock(cs.RoundStateEvent()); err != nil {
		logger.Errorw("Failed publishing event unlock", "err", err)
	}

	cs.signAddVote(types2.CommitType, nil, types.PartSetHeader{})
}

// Enter: any +2/3 precommits for next round.
func (cs *State) enterPrecommitWait(height int64, round int32) {
	logger := cs.Logger.With("height", height, "round", round)

	if cs.Height != height || round < cs.Round || (cs.Round == round && cs.TriggeredTimeoutCommit) {
		return
	}

	if !cs.Votes.Commits(round).HasTwoThirdsAny() {
		panic(fmt.Sprintf("Enter commit_wait step (%v/%v), but commits does not have any +2/3 votes", height, round))
	}

	logger.Infow("Enter commit_wait step")

	defer func() {
		cs.TriggeredTimeoutCommit = true
		cs.newStep()
	}()

	// 如果round等于0，则等待1s，如果round等于1，则等待1.5s
	cs.scheduleTimeout(cs.config.Precommit(round), height, round, RoundStepCommitWait)
}

// 收到 +2/3 个precommit投票消息后，会调用此方法，enterReply 仅可被 addVote 函数调用
func (cs *State) enterReply(height int64, commitRound int32) {
	logger := cs.Logger.With("height", height, "commit_round", commitRound)

	if cs.Height != height || RoundStepReply <= cs.Step {
		return
	}

	logger.Infow("Reply to client")

	defer func() {
		cs.updateRoundStep(cs.Round, RoundStepReply)
		cs.ReplyRound = commitRound
		cs.CommitTime = srtime.Now()
		cs.newStep()
		cs.tryFinalizeAppend(height)
	}()

	blockID, _ := cs.Votes.Commits(commitRound).TwoThirdsMajority()

	// 如果锁定的 LockedBlock 与 “+2/3 precommits” 对应的 block 一样，则用 LockedBlock
	// 替换 State.PrePrepareBlock，用 LockedBlockParts 替换 State.PrePrepareBlockParts，
	// 现在 commit 的区块等于之前锁定的区块
	if cs.LockedBlock.EqualsTo(blockID.Hash) {
		cs.PrePrepareBlock = cs.LockedBlock
		cs.PrePrepareBlockParts = cs.LockedBlockParts
	}

	// 如果等待 commit 的区块是我们不知道的区块，则让 State.PrePrepareBlock=nil，并让 State.PrePrepareBlockParts
	// 等于等待 commit 的区块的 PartSetHeader
	if !cs.PrePrepareBlock.EqualsTo(blockID.Hash) {
		if !cs.PrePrepareBlockParts.HeaderEqualsTo(blockID.PartSetHeader) {
			cs.PrePrepareBlock = nil
			cs.PrePrepareBlockParts = types.NewPartSetFromHeader(blockID.PartSetHeader)

			if err := cs.eventBus.PublishEventValidBlock(cs.RoundStateEvent()); err != nil {
				logger.Errorw("Failed publishing valid block", "err", err)
			}

			cs.evsw.FireEvent(types.EventValidBlock, &cs.RoundState)
		}
	}
}

// tryFinalizeAppend 该方法首先会尝试获取 “+2/3 precommits” 对应的区块 block，如果该区块不存在，
// 则无法完成 commit 区块的过程，然后如果等待被 commit 的区块是自己不知道的区块，则无法完成 commit
// 区块的过程。以上判断都通过，则会调用 finalizeAppend 方法 commit 区块
func (cs *State) tryFinalizeAppend(height int64) {
	logger := cs.Logger.With("height", height, "round", cs.Round)

	if cs.Height != height {
		panic(fmt.Sprintf("tryFinalizeAppend() cs.Height: %v vs height: %v", cs.Height, height))
	}

	// 获取 “+2/3 precommits” 对应的区块 block，如果该区块不存在，则无法完成 commit 区块的过程
	blockID, ok := cs.Votes.Commits(cs.ReplyRound).TwoThirdsMajority()
	if !ok || len(blockID.Hash) == 0 {
		logger.Errorw("Failed attempt to append the block to the blockchain; there was no +2/3 majority or +2/3 was for nil")
		return
	}

	// 如果等待被 commit 的区块是自己不知道的区块，则无法完成 commit 区块的过程
	if !cs.PrePrepareBlock.EqualsTo(blockID.Hash) {
		logger.Debugw("Failed attempt to append the block to the blockchain; we do not have this block", "pre_prepare_block", cs.PrePrepareBlock.Hash(), "block", blockID.Hash)
		return
	}

	cs.finalizeAppend(height)
}

// finalizeAppend 完成最终的 commit 阶段，增加区块高度，然后进入到 RoundStepNewHeight 阶段
func (cs *State) finalizeAppend(height int64) {
	logger := cs.Logger.With("height", height, "round", cs.Round)

	if cs.Height != height || cs.Step != RoundStepReply {
		return
	}

	cs.waitNextEvaluation--  // 防止网络比较差的节点落后于其他节点，在刚evaluation之后，又开始evaluation

	// 判断有没有获得超过 2/3 个 precommit 投票，如果没有的话，显然是不能 commit 的
	blockID, ok := cs.Votes.Commits(cs.ReplyRound).TwoThirdsMajority()
	if !ok {
		panic("cannot finalize commit; commit does not have 2/3 majority")
	}

	block, blockParts := cs.PrePrepareBlock, cs.PrePrepareBlockParts
	if !blockParts.HeaderEqualsTo(blockID.PartSetHeader) {
		panic("PrePrepareBlockParts header doesn't equal to commit header")
	}
	// 检查提议的区块是否等于获得 +2/3 precommit 投票的区块的哈希值
	if !block.EqualsTo(blockID.Hash) {
		panic("cannot finalize commit; proposal block does not hash to commit hash")
	}

	// 验证区块的合法性
	if err := cs.blockExec.ValidateBlock(cs.state, block); err != nil {
		panic(fmt.Errorf("+2/3 committed an invalid block: %w", err))
		//return
	}

	logger.Infow("Append block successfully", "block_hash", block.Hash(), "num_txs", len(block.Txs))

	if cs.blockStore.Height() < block.Height { // 待完成提交的区块，它的高度肯定大于已经提交的区块的高度
		// 注意:seenCommit 是提交这个块的局部理由，但可能与下一个块包含的 LastCommits 不同
		precommits := cs.Votes.Commits(cs.ReplyRound)
		seenCommit := precommits.MakeReply()
		// 将 block, blockParts, 和 seenCommit 持久化到底层的数据库
		cs.blockStore.SaveBlock(block, blockParts, seenCommit)
	} else {
		// 此种情况在我们已经保存了块但没有提交时发生
		logger.Debugw("Calling finalizeAppend on already stored block", "height", block.Height)
	}

	// 为这个高度写入EndHeightMessage{}，暗示块存储已经保存了该块。如果我们在写这个EndHeightMessage{}之前崩溃了，
	// 我们将在重新启动时通过运行ApplyBlock在ABCI握手过程中恢复。如果我们在写EndHeightMessage{}之前没有将该块保存
	// 到块存储中，我们将不得不更改WAL重放—目前它抱怨重放已经存在的#ENDHEIGHT条目的高度。无论哪种方式，状态都不应该被
	// 恢复，直到我们成功调用ApplyBlock(即。稍后在这里，或在重启后握手)。
	endMsg := EndHeightMessage{height}
	if err := cs.wal.WriteSync(endMsg); err != nil { // NOTE: fsync
		panic(fmt.Sprintf("failed to write %v msg to consensus WAL due to %v; check your file system and restart the node", endMsg, err))
	}

	// Create a copy of the state for staging and an event cache for txs.
	stateCopy := cs.state.Copy()

	// Execute and commit the block, update and save the state, and update the mempool.
	// NOTE The block.AppHash wont reflect these txs until the next block.
	var (
		err          error
	)

	stateCopy, err = cs.blockExec.ApplyBlock(stateCopy, types.BlockID{Hash: block.Hash(), PartSetHeader: blockParts.Header()}, block)
	if err != nil {
		logger.Errorw("Failed to apply block", "err", err)
		return
	}

	if cs.Height == 8 && cs.config.Evaluation {
		if cs.config.Evaluation {
			cs.Logger.Infow("======================== Start The Trust Evaluation ========================", "ToleranceDelay", cs.config.ToleranceDelay)
			cs.evsw.FireEvent(types.EventReputation, height)
			cs.waitNextEvaluation = 5
			cs.t.Reset(2000 * time.Millisecond)
		}
	} else {
		cs.t.Reset(0)
	}
	<-cs.t.C

	// NewHeightStep!
	cs.updateToState(stateCopy)

	// Private validator might have changed it's key pair => refetch pubkey.
	if err := cs.updatePrivValidatorPubKey(); err != nil {
		logger.Errorw("Failed to get private validator pubkey", "err", err)
	}

	// cs.StartTime is already set.
	// Schedule Round0 to start soon.
	cs.scheduleRound0()
}

//-----------------------------------------------------------------------------

// setProposal 为内部共识状态设置新的 proposal
func (cs *State) setProposal(proposal *types.PrePrepare) error {
	// 已经有了一个 proposal，则直接返回 nil
	if cs.PrePrepare != nil {
		return nil
	}

	// 如果 proposal 的 height 和 round 都不等于内部共识状态标识的 height 和 round，则直接返回 nil
	if proposal.Height != cs.Height || proposal.Round != cs.Round {
		return nil
	}

	// 如果 proposal 的 POLRound 小于 -1，或者大于等于 0 并且大于等于 proposal 的 round，则返回错误
	// proposal 的 POLRound 不合法
	if proposal.POLRound < -1 ||
		(proposal.POLRound >= 0 && proposal.POLRound >= proposal.Round) {
		return ErrInvalidProposalPOLRound
	}

	p := proposal.ToProto()
	// 用 proposer 的 公钥验证 proposal 签名的正确性
	if !cs.Validators.GetPrimary(cs.state.LastBlockID, cs.Round).PubKey.VerifySignature(types.PrePrepareSignBytes(cs.state.ChainID, p), proposal.Signature) {
		return ErrInvalidProposalSignature
	}

	proposal.Signature = p.Signature
	cs.PrePrepare = proposal
	// 如果内部共识状态的 PrePrepareBlockParts 不为 nil，则直接跳过，否则根据 proposal 重新设置
	if cs.PrePrepareBlockParts == nil {
		cs.PrePrepareBlockParts = types.NewPartSetFromHeader(proposal.BlockID.PartSetHeader)
	}

	return nil
}

// NOTE: block is not necessarily valid.
// Asynchronously triggers either enterPrepare (before we timeout of propose) or tryFinalizeAppend,
// once we have the full block.
func (cs *State) addProposalBlockPart(msg *BlockPartMessage, peerID gossip.ID) (added bool, err error) {
	height, round, part := msg.Height, msg.Round, msg.Part

	// Blocks might be reused, so round mismatch is OK
	if cs.Height != height {
		cs.Logger.Debugw("Received block part from wrong height", "height", height, "round", round)
		return false, nil
	}

	// We're not expecting a block part.
	if cs.PrePrepareBlockParts == nil {
		// NOTE: this can happen when we've gone to a higher round and
		// then receive parts from the previous round - not necessarily a bad peer.
		cs.Logger.Debugw("Received a block part when we are not expecting any", "height", height, "round", round, "index", part.Index, "peer", peerID)
		return false, nil
	}

	added, err = cs.PrePrepareBlockParts.AddPart(part)
	if err != nil {
		return added, err
	}
	if cs.PrePrepareBlockParts.ByteSize() > cs.state.ConsensusParams.Block.MaxBytes {
		return added, fmt.Errorf("total size of proposal block parts exceeds maximum block bytes (%d > %d)", cs.PrePrepareBlockParts.ByteSize(), cs.state.ConsensusParams.Block.MaxBytes)
	}
	if added && cs.PrePrepareBlockParts.IsComplete() {
		bz, err := ioutil.ReadAll(cs.PrePrepareBlockParts.GetReader())
		if err != nil {
			return added, err
		}

		var pbb = new(types2.Block)
		err = proto.Unmarshal(bz, pbb)
		if err != nil {
			return added, err
		}

		block, err := types.BlockFromProto(pbb)
		if err != nil {
			return added, err
		}

		cs.PrePrepareBlock = block

		// NOTE: it's possible to receive complete proposal blocks for future rounds without having the proposal
		cs.Logger.Infow("Received complete block", "height", cs.PrePrepareBlock.Height, "hash", cs.PrePrepareBlock.Hash())

		if err := cs.eventBus.PublishEventCompleteProposal(cs.CompleteProposalEvent()); err != nil {
			cs.Logger.Errorw("Failed publishing event complete proposal", "err", err)
		}

		// Update Valid* if we can.
		prevotes := cs.Votes.Prepares(cs.Round)
		blockID, hasTwoThirds := prevotes.TwoThirdsMajority()
		if hasTwoThirds && !blockID.IsZero() && (cs.ValidRound < cs.Round) {
			if cs.PrePrepareBlock.EqualsTo(blockID.Hash) {
				cs.Logger.Debugw("Updating valid block to new block", "valid_round", cs.Round, "valid_block_hash", cs.PrePrepareBlock.Hash())

				cs.ValidRound = cs.Round
				cs.ValidBlock = cs.PrePrepareBlock
				cs.ValidBlockParts = cs.PrePrepareBlockParts
			}
		}

		if cs.Step <= RoundStepPrePrepare && cs.isPrePrepareComplete() {
			// Move onto the next step
			cs.enterPrepare(height, cs.Round)
			if hasTwoThirds { // this is optimisation as this will be triggered when prevote is added
				cs.enterPrecommit(height, cs.Round)
			}
		} else if cs.Step == RoundStepReply {
			// If we're waiting on the proposal block...
			cs.tryFinalizeAppend(height)
		}

		return added, nil
	}

	return added, nil
}

// tryAddVote 调用 addVote 将投票加入到本地，addVote 会返回投票结果：isSuccess，err
//	1. 处理新投票是一个有冲突的投票
//	2. 对于其他错误类型的投票，tryAddVote 则只会在日志中记录一下，不会做其他处理
func (cs *State) tryAddVote(vote *types.Vote, peerID gossip.ID) (bool, error) {
	isSuccess, err := cs.addVote(vote, peerID)
	if err != nil {
		if _, ok := err.(*types.ErrVoteConflictingVotes); ok {
			if cs.privValidatorPubKey == nil {
				return false, errPubKeyIsNotSet
			}

			// 将出错 vote 中的 ValidatorAddress 与我们自己的 privValidatorPubkey.Address 进行比较，
			// 判断该错误 vote 是否来自于我们自己
			if bytes.Equal(vote.ValidatorAddress, cs.privValidatorPubKey.Address()) {
				cs.Logger.Errorw("Found conflicting vote from ourselves; did you unsafe_reset a validator?", "height", vote.Height, "round", vote.Round, "type", vote.Type)
				return isSuccess, err
			}

			return isSuccess, err
		} else if errors.Is(err, types.ErrVoteNonDeterministicSignature) {
			// 投票具有非确定性签名
			cs.Logger.Debugw("Vote has non-deterministic signature", "err", err)
		} else {
			return isSuccess, ErrAddingVote
		}
	}

	return isSuccess, nil
}

func (cs *State) addVote(vote *types.Vote, peerID gossip.ID) (added bool, err error) {
	// 如果我们处在 RoundStepNewHeight 阶段，然后收到来自于上一个区块高度的 precommit，那么我们会将其添加到
	// LastCommits 集合中，但如果我们不在 RoundStepNewHeight 阶段，则会直接忽略来自于上一个区块高度的 precommit
	if vote.Height+1 == cs.Height && vote.Type == types2.CommitType {
		if cs.Step != RoundStepNewHeight {
			return
		}

		added, err = cs.LastCommits.AddVote(vote)
		if !added {
			return
		}

		cs.Logger.Debugw("Added vote to last COMMIT vote set", "last_commit", cs.LastCommits.StringShort())
		if err := cs.eventBus.PublishEventVote(types.EventDataVote{Vote: vote}); err != nil {
			return added, err
		}

		cs.evsw.FireEvent(types.EventVote, vote)

		// 如果我们已经收集齐上一个区块高度中的所有 precommit 投票，并且配置允许跳过 timeoutCommit 时间，那么我们则直接跳过
		if cs.config.SkipTimeoutCommit && cs.LastCommits.HasAll() {
			cs.enterNewRound(cs.Height, 0)
		}

		return
	}

	if vote.Height != cs.Height {
		cs.Logger.Debugw("Vote ignored and not added", "vote_height", vote.Height, "cs_height", cs.Height, "peer", peerID)
		return
	}

	height := cs.Height

	// 将 vote 添加到本地投票集合中
	added, err = cs.Votes.AddVote(vote, peerID)
	if !added {
		// 如果没有添加成功，则返回
		return
	}

	if err := cs.eventBus.PublishEventVote(types.EventDataVote{Vote: vote}); err != nil {
		return added, err
	}
	cs.evsw.FireEvent(types.EventVote, vote)

	// 下面将按照 vote 的类型进行对应的处理
	switch vote.Type {
	case types2.PrepareType:
		// 获取本地投票集合中针对 vote.Round 轮次的所有 prevote 投票
		prevotes := cs.Votes.Prepares(vote.Round)
		cs.Logger.Debugw("Added vote to prevote", "vote", vote, "prevotes", prevotes.StringShort())

		// 判断在 vote.Round 轮次中是否收到 +2/3 个 prevote 投票
		if blockID, ok := prevotes.TwoThirdsMajority(); ok {
			// 如果在 vote.Round 轮次中收到 +2/3 个 prevote 投票，我们还需要进行以下判断
			if (cs.LockedBlock != nil) &&
				(cs.LockedRound < vote.Round) &&
				(vote.Round <= cs.Round) &&
				!cs.LockedBlock.EqualsTo(blockID.Hash) {

				cs.Logger.Debugw("Unlocking because of POL", "locked_round", cs.LockedRound, "pol_round", vote.Round)

				cs.LockedRound = -1
				cs.LockedBlock = nil
				cs.LockedBlockParts = nil

				if err := cs.eventBus.PublishEventUnlock(cs.RoundStateEvent()); err != nil {
					return added, err
				}
			}

			// 如果我们收到了 +2/3 个 prevote 投票，则说明本轮次提出的 proposal 是合法的，得到大多数诚实节点的认可
			if len(blockID.Hash) != 0 && (cs.ValidRound < vote.Round) && (vote.Round == cs.Round) {
				if cs.PrePrepareBlock.EqualsTo(blockID.Hash) {
					cs.Logger.Debugw("Updating valid block because of POL", "valid_round", cs.ValidRound, "pol_round", vote.Round)
					cs.ValidRound = vote.Round                   // 更新 ValidRound
					cs.ValidBlock = cs.PrePrepareBlock           // 更新 ValidBlock
					cs.ValidBlockParts = cs.PrePrepareBlockParts // 更新 ValidBlockParts
				} else {
					cs.Logger.Debugw("Valid block we do not know about; set PrePrepareBlock=nil", "PrePrepareBlock", cs.PrePrepareBlock.Hash(), "block_id", blockID.Hash)
					// 如果大多数诚实节点认可的 block 与我们自己本地的 block 不一样，那么就让我们本地的 PrePrepareBlock 等于 nil
					cs.PrePrepareBlock = nil
				}
				// 如果我们本地自己的 PrePrepareBlockParts 与大多数诚实的节点认可的 PartSetHeader 不一样，那么将我们本地
				// 存储的 PrePrepareBlockParts 改为大家认可的 PartSet
				if !cs.PrePrepareBlockParts.HeaderEqualsTo(blockID.PartSetHeader) {
					cs.PrePrepareBlockParts = types.NewPartSetFromHeader(blockID.PartSetHeader)
				}

				cs.evsw.FireEvent(types.EventValidBlock, &cs.RoundState)  // 告诉其他节点自己获得了一个valid block
				if err := cs.eventBus.PublishEventValidBlock(cs.RoundStateEvent()); err != nil {
					return added, err
				}
			}
		}

		switch {
		case cs.Round < vote.Round && prevotes.HasTwoThirdsAny():
			// 如果待加入的新 vote 是来自于未来的轮次的，并且我们已经收到了 2/3 个针对该投票的 prevote，那么我们就直接进入 vote.Round
			cs.enterNewRound(height, vote.Round)

		case cs.Round == vote.Round && RoundStepPrepare <= cs.Step:
			blockID, ok := prevotes.TwoThirdsMajority()
			if ok && (cs.isPrePrepareComplete() || len(blockID.Hash) == 0) {
				// 如果待加入的新 vote 正是来自于当前轮次的，并且已经收到了针对该 vote 超过 2/3 个 prevote，则进入 precommit 阶段
				cs.enterPrecommit(height, vote.Round)
			} else if prevotes.HasTwoThirdsAny() {
				cs.enterPrepareWait(height, vote.Round)
			}

		case cs.PrePrepare != nil && 0 <= cs.PrePrepare.POLRound && cs.PrePrepare.POLRound == vote.Round:
			if cs.isPrePrepareComplete() {
				// 如果
				cs.enterPrepare(height, cs.Round)
			}
		}

	case types2.CommitType: // 如果收到的 vote 是 precommit
		precommits := cs.Votes.Commits(vote.Round) // 获取 vote.Round 轮次收到的所有 precommit
		cs.Logger.Infow("Added vote to COMMIT vote set", "height", vote.Height, "round", vote.Round, "validator", vote.ValidatorAddress.String(), "vote_timestamp", vote.Timestamp, "data", precommits.LogString())
		// 判断在 vote.Round 轮次中收到的 precommit 是否达到 2/3
		blockID, ok := precommits.TwoThirdsMajority()
		if ok {
			// Executed as TwoThirdsMajority could be from a higher round
			cs.enterNewRound(height, vote.Round)
			cs.enterPrecommit(height, vote.Round)

			if len(blockID.Hash) != 0 { // 如果收到了超过 2/3 个 precommit，则会进入 commit 阶段
				cs.enterReply(height, vote.Round)
				if cs.config.SkipTimeoutCommit && precommits.HasAll() {
					cs.enterNewRound(cs.Height, 0)
				}
			} else {
				cs.enterPrecommitWait(height, vote.Round)
			}
		} else if cs.Round <= vote.Round && precommits.HasTwoThirdsAny() {
			cs.enterNewRound(height, vote.Round)
			cs.enterPrecommitWait(height, vote.Round)
		}

	default:
		panic(fmt.Sprintf("unexpected vote type %v", vote.Type))
	}

	return added, err
}

// CONTRACT: cs.privValidator is not nil.
func (cs *State) signVote(msgType types2.SignedMsgType, hash []byte, header types.PartSetHeader, ) (*types.Vote, error) {
	srrand.Seed(time.Now().UnixNano())
	if err := cs.wal.FlushAndSync(); err != nil {
		return nil, err
	}

	if cs.privValidatorPubKey == nil {
		return nil, errPubKeyIsNotSet
	}

	addr := cs.privValidatorPubKey.Address()
	valIdx, _ := cs.Validators.GetByAddress(addr)

	//#################################################################################################
	// 拜占庭节点所为！
	if cs.config.IsByzantine {
		if len(hash) != 0 {
			hash[srrand.Int()%len(hash)] = byte(srrand.Int()%255+1)
		}
		if srrand.Float64() > 0.5 {
			return nil, errors.New("I am byzantine node")
		}
		time.Sleep(time.Millisecond * time.Duration(lazy) + time.Duration(srrand.Intn(1000)) * time.Millisecond)
	}
	//#################################################################################################

	vote := &types.Vote{
		ValidatorAddress: addr,
		ValidatorIndex:   valIdx,
		Height:           cs.Height,
		Round:            cs.Round,
		Timestamp:        cs.voteTime(),
		Type:             msgType,
		BlockID:          types.BlockID{Hash: hash, PartSetHeader: header},
	}

	v := vote.ToProto()
	err := cs.privValidator.SignVote(cs.state.ChainID, v)
	vote.Signature = v.Signature

	return vote, err
}

func (cs *State) voteTime() time.Time {
	now := srtime.Now()
	minVoteTime := now

	timeIota := time.Duration(cs.state.ConsensusParams.Block.TimeIotaMs) * time.Millisecond
	if cs.LockedBlock != nil {
		minVoteTime = cs.LockedBlock.Time.Add(timeIota)
	} else if cs.PrePrepareBlock != nil {
		minVoteTime = cs.PrePrepareBlock.Time.Add(timeIota)
	}

	if now.After(minVoteTime) {
		return now
	}
	return minVoteTime
}

// sign the vote and publish on internalMsgQueue
func (cs *State) signAddVote(msgType types2.SignedMsgType, hash []byte, header types.PartSetHeader) *types.Vote {
	if cs.privValidator == nil { // the node does not have a key
		return nil
	}

	if cs.privValidatorPubKey == nil {
		// Vote won't be signed, but it's not critical.
		cs.Logger.Errorw(fmt.Sprintf("signAddVote: %v", errPubKeyIsNotSet))
		return nil
	}

	// If the node not in the validator set, do nothing.
	if !cs.Validators.HasAddress(cs.privValidatorPubKey.Address()) {
		return nil
	}

	vote, err := cs.signVote(msgType, hash, header)
	if err == nil {
		cs.sendInternalMessage(msgInfo{&VoteMessage{vote}, ""})
		cs.Logger.Debugw("Signed and pushed vote", "height", cs.Height, "round", cs.Round, "vote", vote)
		return vote
	}

	cs.Logger.Errorw("Failed signing vote", "height", cs.Height, "round", cs.Round, "vote", vote, "err", err)
	return nil
}

// updatePrivValidatorPubKey get's the private validator public key and
// memoizes it. This func returns an error if the private validator is not
// responding or responds with an error.
func (cs *State) updatePrivValidatorPubKey() error {
	if cs.privValidator == nil {
		return nil
	}

	pubKey, err := cs.privValidator.GetPubKey()
	if err != nil {
		return err
	}
	cs.privValidatorPubKey = pubKey
	return nil
}

//---------------------------------------------------------

func CompareHRS(h1 int64, r1 int32, s1 RoundStepType, h2 int64, r2 int32, s2 RoundStepType) int {
	if h1 < h2 {
		return -1
	} else if h1 > h2 {
		return 1
	}
	if r1 < r2 {
		return -1
	} else if r1 > r2 {
		return 1
	}
	if s1 < s2 {
		return -1
	} else if s1 > s2 {
		return 1
	}
	return 0
}

// repairWalFile decodes messages from src (until the decoder errors) and
// writes them to dst.
func repairWalFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	var (
		dec = NewWALDecoder(in)
		enc = NewWALEncoder(out)
	)

	// best-case repair (until first error is encountered)
	for {
		msg, err := dec.Decode()
		if err != nil {
			break
		}

		err = enc.Encode(msg)
		if err != nil {
			return fmt.Errorf("failed to encode msg: %w", err)
		}
	}

	return nil
}
