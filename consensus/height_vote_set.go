package consensus

import (
	"errors"
	"fmt"
	"github.com/232425wxy/BFT/gossip"
	srjson "github.com/232425wxy/BFT/libs/json"
	srmath "github.com/232425wxy/BFT/libs/math"
	prototypes "github.com/232425wxy/BFT/proto/types"
	"github.com/232425wxy/BFT/types"
	"strings"
	"sync"
)

// RoundVoteSet 包含两个字段：Prepares 和 Precommits
type RoundVoteSet struct {
	Prepares   *types.VoteSet
	Precommits *types.VoteSet
}

// ErrGotVoteFromUnwantedRound 当从某个 peer 处收到一个 round 不正确的 vote 时，会触发该错误
var ErrGotVoteFromUnwantedRound = errors.New("peer has sent a vote that does not match our round for more than one round", )

type HeightVoteSet struct {
	chainID string              // 区块链的ID
	height  int64               // 区块高度
	valSet  *types.ValidatorSet // validator 集合

	mtx               sync.Mutex
	round             int32                  // max tracked round
	roundVoteSets     map[int32]RoundVoteSet // roundVoteSets 的键是 round，值是在每个 round 阶段里收到的 vote，包括 prevote 和 precommit
	peerCatchupRounds map[gossip.ID][]int32  // keys: peer.ID; values: at most 2 rounds
}

func NewHeightVoteSet(chainID string, height int64, valSet *types.ValidatorSet) *HeightVoteSet {
	hvs := &HeightVoteSet{
		chainID: chainID,
	}
	hvs.Reset(height, valSet)
	return hvs
}

// Reset 重置 HeightVoteSet 的区块高度：height 和 validator 集合
func (hvs *HeightVoteSet) Reset(height int64, valSet *types.ValidatorSet) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()

	hvs.height = height
	hvs.valSet = valSet
	hvs.roundVoteSets = make(map[int32]RoundVoteSet)
	hvs.peerCatchupRounds = make(map[gossip.ID][]int32)

	hvs.addRound(0)
	hvs.round = 0
}

// Height 返回 HeightVoteSet 的区块高度
func (hvs *HeightVoteSet) Height() int64 {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	return hvs.height
}

// Round 返回 HeightVoteSet 的 round
func (hvs *HeightVoteSet) Round() int32 {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	return hvs.round
}

// SetRound 会让 HeightVoteSet 内存储的 round 数量增加：
//	从 HeightVoteSet.round-1 开始到 round，依次往 HeightVoteSet
// 	的 roundVoteSets 中添加 round 和 vote 集合对，遇到已经存在的 round
//	则直接跳
func (hvs *HeightVoteSet) SetRound(round int32) {
	hvs.mtx.Lock() // 支持多线程安全
	defer hvs.mtx.Unlock()
	newRound := srmath.SafeSubInt32(hvs.round, 1) // 新 round 等于 HeightVoteSet 的 round 减去 1
	if hvs.round != 0 && (round < newRound) {
		panic("SetRound() must increment hvs.round")
	}
	for r := newRound; r <= round; r++ {
		if _, ok := hvs.roundVoteSets[r]; ok {
			continue
		}
		hvs.addRound(r)
	}
	hvs.round = round
}

// addRound
// 	1. 如果待添加的 round 已经在 HeightVoteSet.roundVoteSets 中已经存在，则直接 panic
//	2. 创建新的 vote 集合，包括：Prepares 和 Commits 两种投票集合
//	3. 在 HeightVoteSet.roundVoteSets 中添加新的 round 对应的 投票集合
func (hvs *HeightVoteSet) addRound(round int32) {
	if _, ok := hvs.roundVoteSets[round]; ok {
		panic("addRound() for an existing round")
	}
	prevotes := types.NewVoteSet(hvs.chainID, hvs.height, round, prototypes.PrepareType, hvs.valSet)
	precommits := types.NewVoteSet(hvs.chainID, hvs.height, round, prototypes.CommitType, hvs.valSet)
	hvs.roundVoteSets[round] = RoundVoteSet{
		Prepares:   prevotes,
		Precommits: precommits,
	}
}

// Duplicate votes return added=false, err=nil.
// By convention, peerID is "" if origin is self.
func (hvs *HeightVoteSet) AddVote(vote *types.Vote, peerID gossip.ID) (added bool, err error) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	// 验证投票类型是否正确
	if !types.IsVoteTypeValid(vote.Type) {
		return
	}
	// 找到对应 round 和投票类型的投票集合
	voteSet := hvs.getVoteSet(vote.Round, vote.Type)
	if voteSet == nil { // 如果找到的投票集合是空的
		if rndz := hvs.peerCatchupRounds[peerID]; len(rndz) < 2 {
			hvs.addRound(vote.Round)
			voteSet = hvs.getVoteSet(vote.Round, vote.Type)
			hvs.peerCatchupRounds[peerID] = append(rndz, vote.Round)
		} else {
			// 发送的 vote 不正确
			err = ErrGotVoteFromUnwantedRound
			return
		}
	}
	added, err = voteSet.AddVote(vote)
	return
}

// Prepares 获取对应 round 的 Prepares 集合
func (hvs *HeightVoteSet) Prepares(round int32) *types.VoteSet {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	return hvs.getVoteSet(round, prototypes.PrepareType)
}

// Commits 获取对应 round 的 Commits 集合
func (hvs *HeightVoteSet) Commits(round int32) *types.VoteSet {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	return hvs.getVoteSet(round, prototypes.CommitType)
}

// POLInfo 遍历自当前区块高度开始过去每个 round 的投票集合，查找是否存在获得过超过
// 2/3 投票的 BlockID，存在的话就返回对应的 round 和 BlockID
func (hvs *HeightVoteSet) POLInfo() (polRound int32, polBlockID types.BlockID) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	for r := hvs.round; r >= 0; r-- { // 依次遍历过去每一 round 中是否存在获得过超过 2/3 投票的 BlockID，
		rvs := hvs.getVoteSet(r, prototypes.PrepareType)
		polBlockID, ok := rvs.TwoThirdsMajority()
		if ok {
			return r, polBlockID
		}
	}
	return -1, types.BlockID{}
}

// getVoteSet 根据指定的 round 和投票类型（PrevoteType、PrecommitType），获取投票集合
func (hvs *HeightVoteSet) getVoteSet(round int32, voteType prototypes.SignedMsgType) *types.VoteSet {
	// 先获取对应 round 的投票集合，内含 prevotes 集合 和 precommits 集合
	rvs, ok := hvs.roundVoteSets[round]
	if !ok {
		return nil
	}
	switch voteType {
	case prototypes.PrepareType:
		return rvs.Prepares
	case prototypes.CommitType:
		return rvs.Precommits
	default:
		panic(fmt.Sprintf("Unexpected vote type %X", voteType))
	}
}

// SetPeerMaj23 当某个 peer 声称自己收到超过 2/3 投票的 BlockID 时，调用此方法
func (hvs *HeightVoteSet) SetPeerMaj23(round int32, voteType prototypes.SignedMsgType, peerID gossip.ID, blockID types.BlockID) error {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	if !types.IsVoteTypeValid(voteType) {
		return fmt.Errorf("setPeerMaj23: Invalid vote type %X", voteType)
	}
	voteSet := hvs.getVoteSet(round, voteType)
	if voteSet == nil {
		return nil // something we don't know about yet
	}
	return voteSet.SetPeerMaj23(types.P2PID(peerID), blockID)
}

//---------------------------------------------------------
// string and json

func (hvs *HeightVoteSet) String() string {
	return hvs.StringIndented("")
}

func (hvs *HeightVoteSet) StringIndented(indent string) string {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	vsStrings := make([]string, 0, (len(hvs.roundVoteSets)+1)*2)
	// rounds 0 ~ hvs.round inclusive
	for round := int32(0); round <= hvs.round; round++ {
		voteSetString := hvs.roundVoteSets[round].Prepares.StringShort()
		vsStrings = append(vsStrings, voteSetString)
		voteSetString = hvs.roundVoteSets[round].Precommits.StringShort()
		vsStrings = append(vsStrings, voteSetString)
	}
	// all other peer catchup rounds
	for round, roundVoteSet := range hvs.roundVoteSets {
		if round <= hvs.round {
			continue
		}
		voteSetString := roundVoteSet.Prepares.StringShort()
		vsStrings = append(vsStrings, voteSetString)
		voteSetString = roundVoteSet.Precommits.StringShort()
		vsStrings = append(vsStrings, voteSetString)
	}
	return fmt.Sprintf(`HeightVoteSet{H:%v R:0~%v
%s  %v
%s}`,
		hvs.height, hvs.round,
		indent, strings.Join(vsStrings, "\n"+indent+"  "),
		indent)
}

func (hvs *HeightVoteSet) MarshalJSON() ([]byte, error) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	return srjson.Marshal(hvs.toAllRoundVotes())
}

// toAllRoundVotes 将 HeightVoteSet 存储的 round 对应的 投票信息以字符串的形式记录下来，
// 然后封装成 []roundVotes
func (hvs *HeightVoteSet) toAllRoundVotes() []roundVotes {
	totalRounds := hvs.round + 1
	allVotes := make([]roundVotes, totalRounds)
	// rounds 0 ~ hvs.round inclusive
	for round := int32(0); round < totalRounds; round++ {
		allVotes[round] = roundVotes{
			Round:              round,
			Prevotes:           hvs.roundVoteSets[round].Prepares.VoteStrings(),
			PrevotesBitArray:   hvs.roundVoteSets[round].Prepares.BitArrayString(),
			Precommits:         hvs.roundVoteSets[round].Precommits.VoteStrings(),
			PrecommitsBitArray: hvs.roundVoteSets[round].Precommits.BitArrayString(),
		}
	}
	return allVotes
}

type roundVotes struct {
	Round              int32    `json:"round"`
	Prevotes           []string `json:"prevotes"`
	PrevotesBitArray   string   `json:"prevotes_bit_array"`
	Precommits         []string `json:"precommits"`
	PrecommitsBitArray string   `json:"precommits_bit_array"`
}
