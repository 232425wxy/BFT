package consensus

import (
	"github.com/232425wxy/BFT/libs/bits"
	"github.com/232425wxy/BFT/types"
	"fmt"
	"time"
)

//-----------------------------------------------------------------------------

// PeerRoundState 指示 peer 的状态
type PeerRoundState struct {
	Height int64         `json:"height"` // peer 所处的区块高度
	Round  int32         `json:"round"`  // peer 所处的 round，如果此处等于 -1，则表示不知道 peer 所处的 round
	Step   RoundStepType `json:"step"`   // peer 所处的可能的 8 个阶段

	// 在本区块高度时开始 round 0 的时间
	StartTime time.Time `json:"start_time"`

	// True if peer has proposal for this round
	Proposal                   bool                `json:"proposal"`
	ProposalBlockPartSetHeader types.PartSetHeader `json:"proposal_block_part_set_header"`
	ProposalBlockParts         *bits.BitArray      `json:"proposal_block_parts"`
	// POL Round，如果没有则等于 -1
	ProposalPOLRound int32 `json:"proposal_pol_round"`

	// 在收到 PrePreparePOLMessage 消息之前一直都是 nil
	ProposalPOL     *bits.BitArray `json:"proposal_pol"`
	Prevotes        *bits.BitArray `json:"prevotes"`          // All votes peer has for this round
	Precommits      *bits.BitArray `json:"precommits"`        // All precommits peer has for this round
	LastCommitRound int32          `json:"last_commit_round"` // Round of commit for last height. -1 if none.
	LastCommit      *bits.BitArray `json:"last_commit"`       // All commit precommits of commit for last height.

	// Round that we have commit for. Not necessarily unique. -1 if none.
	CatchupCommitRound int32 `json:"catchup_commit_round"`

	// All commit precommits peer has for this height & CatchupCommitRound
	CatchupCommit *bits.BitArray `json:"catchup_commit"`
}

// String returns a string representation of the PeerRoundState
func (prs PeerRoundState) String() string {
	return prs.StringIndented("")
}

// StringIndented returns a string representation of the PeerRoundState
func (prs PeerRoundState) StringIndented(indent string) string {
	return fmt.Sprintf(`PeerRoundState{
%s  %v/%v/%v @%v
%s  PrePrepare %v -> %v
%s  POL      %v (round %v)
%s  Prepares   %v
%s  Commits %v
%s  LastCommits %v (round %v)
%s  Catchup    %v (round %v)
%s}`,
		indent, prs.Height, prs.Round, prs.Step, prs.StartTime,
		indent, prs.ProposalBlockPartSetHeader, prs.ProposalBlockParts,
		indent, prs.ProposalPOL, prs.ProposalPOLRound,
		indent, prs.Prevotes,
		indent, prs.Precommits,
		indent, prs.LastCommit, prs.LastCommitRound,
		indent, prs.CatchupCommit, prs.CatchupCommitRound,
		indent)
}
