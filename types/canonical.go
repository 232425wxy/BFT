package types

import (
	srtime "BFT/libs/time"
	"BFT/proto/types"
	"time"
)

// Canonical* 将 struct 封装在类型中，用于 amino 编码，用于 SignBytes / Signable 接口。

// TimeFormat 用于生成 signature？
const TimeFormat = time.RFC3339Nano

//-----------------------------------
// Canonicalize the structs

func CanonicalizeBlockID(bid types.BlockID) *types.CanonicalBlockID {
	rbid, err := BlockIDFromProto(&bid)
	if err != nil {
		panic(err)
	}
	var cbid *types.CanonicalBlockID
	if rbid == nil || rbid.IsZero() {
		cbid = nil
	} else {
		cbid = &types.CanonicalBlockID{
			Hash:          bid.Hash,
			PartSetHeader: CanonicalizePartSetHeader(bid.PartSetHeader),
		}
	}

	return cbid
}

// CanonicalizePartSetHeader 将给定的 PartSetHeader 转换为 CanonicalPartSetHeader。
func CanonicalizePartSetHeader(psh types.PartSetHeader) types.CanonicalPartSetHeader {
	return types.CanonicalPartSetHeader(psh)
}

// CanonicalizePrePrepare 将给定的 PrePrepare 转换为 CanonicalProposal。
func CanonicalizePrePrepare(chainID string, proposal *types.PrePrepare) types.CanonicalPrePrepare {
	return types.CanonicalPrePrepare{
		Type:      types.PrePrepareType,
		Height:    proposal.Height,       // encoded as sfixed64
		Round:     int64(proposal.Round), // encoded as sfixed64
		POLRound:  int64(proposal.PolRound),
		BlockID:   CanonicalizeBlockID(proposal.BlockID),
		Timestamp: proposal.Timestamp,
		ChainID:   chainID,
	}
}

// CanonicalizeVote 将给定的 Vote 转换为 CanonicalVote，
// 后者不包含 ValidatorIndex 、 ValidatorAddress 字段和 Signature 字段。
func CanonicalizeVote(chainID string, vote *types.Vote) types.CanonicalVote {
	return types.CanonicalVote{
		Type:      vote.Type,
		Height:    vote.Height,       // encoded as sfixed64
		Round:     int64(vote.Round), // encoded as sfixed64
		BlockID:   CanonicalizeBlockID(vote.BlockID),
		Timestamp: vote.Timestamp,
		ChainID:   chainID,
	}
}

// CanonicalTime 可用于以规范的方式将时间字符串化。
func CanonicalTime(t time.Time) string {
	return srtime.Canonical(t).Format(TimeFormat) // "2006-01-02T15:04:05.999999999Z07:00"
}
