package consensus

import (
	"BFT/libs/bytes"
	"BFT/types"
	"encoding/json"
	"fmt"
	"time"
)

//-----------------------------------------------------------------------------
// RoundStepType enum type

// RoundStepType 定义了共识过程中的各个阶段
type RoundStepType uint8

// RoundStepType
const (
	RoundStepNewHeight   = RoundStepType(0x01)
	RoundStepNewRound    = RoundStepType(0x02)
	RoundStepPrePrepare  = RoundStepType(0x03)
	RoundStepPrepare     = RoundStepType(0x04)
	RoundStepPrepareWait = RoundStepType(0x05)
	RoundStepCommit      = RoundStepType(0x06)
	RoundStepCommitWait  = RoundStepType(0x07)
	RoundStepReply       = RoundStepType(0x08)
)

func (rs RoundStepType) IsValid() bool {
	return uint8(rs) >= 0x01 && uint8(rs) <= 0x08
}

// String 用字符串的形式表示共识过程中的各个阶段
func (rs RoundStepType) String() string {
	switch rs {
	case RoundStepNewHeight:
		return "RoundStepNewHeight"
	case RoundStepNewRound:
		return "RoundStepNewRound"
	case RoundStepPrePrepare:
		return "RoundStepPrePrepare"
	case RoundStepPrepare:
		return "RoundStepPrepare"
	case RoundStepPrepareWait:
		return "RoundStepPrepareWait"
	case RoundStepCommit:
		return "RoundStepCommit"
	case RoundStepCommitWait:
		return "RoundStepCommitWait"
	case RoundStepReply:
		return "RoundStepReply"
	default:
		return "RoundStepUnknown" // Cannot panic.
	}
}

//-----------------------------------------------------------------------------

// RoundState 定义了内部共识状态
// NOTE: Not thread safe. Should only be manipulated by functions downstream
// of the cs.receiveRoutine
type RoundState struct {
	Height    int64         `json:"height"` // Height we are working on
	Round     int32         `json:"round"`

	Step      RoundStepType `json:"step"`
	StartTime time.Time     `json:"start_time"` // 就下一个区块达成共识的开始时间

	CommitTime         time.Time           `json:"commit_time"` // 上一次 commit 一个区块的时间，在 State 的 enterReply() 方法中被赋值更新
	Validators    *types.ValidatorSet    `json:"validators"`
	PrePrepare         *types.PrePrepare `json:"pre_prepare"`
	PrePrepareBlock      *types.Block    `json:"pre_prepare_block"`
	PrePrepareBlockParts *types.PartSet `json:"pre_prepare_block_parts"`
	LockedRound          int32          `json:"locked_round"`       // 锁定的 round
	LockedBlock        *types.Block        `json:"locked_block"`       // 被锁定的 block
	LockedBlockParts   *types.PartSet      `json:"locked_block_parts"` // 被锁定的 block parts

	// 如果我们收到了 +2/3 个 PrePrepare 消息，则表明系统中大多数诚实节点都认为当前轮次提出的 proposal 是合法的，
	// 那么就将当前轮次赋值给 ValidRound
	ValidRound int32 `json:"valid_round"`
	// 如果我们收到了 +2/3 个 PrePrepare 消息，则表明系统中大多数诚实节点都认为当前轮次提出的 proposal 是合法的，
	// 那么就将在当前轮次提出的 block 赋值给 ValidBlock
	ValidBlock *types.Block `json:"valid_block"`

	// Last known block parts of POL mentioned above.
	ValidBlockParts           *types.PartSet      `json:"valid_block_parts"`
	Votes                     *HeightVoteSet      `json:"votes"`
	ReplyRound                int32               `json:"reply_round"`
	LastCommits               *types.VoteSet      `json:"last_commits"`
	LastValidators         *types.ValidatorSet `json:"last_validators"`
	TriggeredTimeoutCommit bool                `json:"triggered_timeout_commit"`
}

// RoundStateSimple 是 RoundState 的压缩版本，用于 RPC
type RoundStateSimple struct {
	HeightRoundStep   string              `json:"height/round/step"`
	StartTime           time.Time      `json:"start_time"`
	PrePrepareBlockHash bytes.HexBytes `json:"pre_prepare_block_hash"`
	LockedBlockHash     bytes.HexBytes `json:"locked_block_hash"`
	ValidBlockHash    bytes.HexBytes      `json:"valid_block_hash"`
	Votes   json.RawMessage     `json:"height_vote_set"`
	Primary types.ValidatorInfo `json:"primary"`
}

// 将 RoundState 压缩为 RoundStateSimple
func (rs *RoundState) RoundStateSimple(b types.BlockID, times int32) RoundStateSimple {
	votesJSON, err := rs.Votes.MarshalJSON()
	if err != nil {
		panic(err)
	}

	// 获取 proposer 的地址和索引
	addr := rs.Validators.GetPrimary(b, times).Address
	idx, _ := rs.Validators.GetByAddress(addr)

	return RoundStateSimple{
		HeightRoundStep:     fmt.Sprintf("%d/%d/%d", rs.Height, rs.Round, rs.Step),
		StartTime:           rs.StartTime,
		PrePrepareBlockHash: rs.PrePrepareBlock.Hash(),
		LockedBlockHash:     rs.LockedBlock.Hash(),
		ValidBlockHash:      rs.ValidBlock.Hash(),
		Votes:               votesJSON,
		Primary: types.ValidatorInfo{
			Address: addr,
			Index:   idx,
		},
	}
}

// NewRoundEvent 将 RoundState 和 proposer 信息作为一个事件返回
func (rs *RoundState) NewRoundEvent(b types.BlockID, times int32) types.EventDataNewRound {
	// 获取 proposer 的地址和索引
	addr := rs.Validators.GetPrimary(b, times).Address
	idx, _ := rs.Validators.GetByAddress(addr)

	return types.EventDataNewRound{
		Height: rs.Height,
		Round:  rs.Round,
		Step:   rs.Step.String(),
		Proposer: types.ValidatorInfo{
			Address: addr,
			Index:   idx,
		},
	}
}

// CompleteProposalEvent 将提议的 proposal 信息作为一个事件返回
func (rs *RoundState) CompleteProposalEvent() types.EventDataCompleteProposal {
	// 利用 RoundState 的 PrePrepareBlock 和 PrePrepareBlockParts 来构造 BlockID
	blockID := types.BlockID{
		Hash:          rs.PrePrepareBlock.Hash(),
		PartSetHeader: rs.PrePrepareBlockParts.Header(),
	}

	return types.EventDataCompleteProposal{
		Height:  rs.Height,
		Round:   rs.Round,
		Step:    rs.Step.String(), // 用字符串表示共识的各个阶段
		BlockID: blockID,
	}
}

// RoundStateEvent 提取 RoundState 的 Height Round 和 Step 作为一个事件返回
func (rs *RoundState) RoundStateEvent() types.EventDataRoundState {
	return types.EventDataRoundState{
		Height: rs.Height,
		Round:  rs.Round,
		Step:   rs.Step.String(),
	}
}

// String returns a string
func (rs *RoundState) String() string {
	return rs.StringIndented("")
}

// StringIndented returns a string
func (rs *RoundState) StringIndented(indent string) string {
	return fmt.Sprintf(`RoundState{
%s  H:%v R:%v S:%v
%s  StartTime:     %v
%s  CommitTime:    %v
%s  Validators:    %v
%s  PrePrepare:      %v
%s  PrePrepareBlock: %v %v
%s  LockedRound:   %v
%s  LockedBlock:   %v %v
%s  ValidRound:   %v
%s  ValidBlock:   %v %v
%s  Votes:         %v
%s  LastCommits:    %v
%s  LastValidators:%v
%s}`,
		indent, rs.Height, rs.Round, rs.Step,
		indent, rs.StartTime,
		indent, rs.CommitTime,
		indent, rs.Validators.StringIndented(indent+"  "),
		indent, rs.PrePrepare,
		indent, rs.PrePrepareBlockParts.StringShort(), rs.PrePrepareBlock.StringShort(),
		indent, rs.LockedRound,
		indent, rs.LockedBlockParts.StringShort(), rs.LockedBlock.StringShort(),
		indent, rs.ValidRound,
		indent, rs.ValidBlockParts.StringShort(), rs.ValidBlock.StringShort(),
		indent, rs.Votes.StringIndented(indent+"  "),
		indent, rs.LastCommits.StringShort(),
		indent, rs.LastValidators.StringIndented(indent+"  "),
		indent)
}

// StringShort 返回 RoundState 的字符串概要信息：
//	RoundState{H:height R:round S:step ST:startTime}
func (rs *RoundState) StringShort() string {
	return fmt.Sprintf(`RoundState{H:%v R:%v S:%v ST:%v}`,
		rs.Height, rs.Round, rs.Step, rs.StartTime)
}
