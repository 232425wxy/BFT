package consensus

import (
	"BFT/gossip"
	"BFT/libs/bits"
	srmath "BFT/libs/math"
	protoconsensus "BFT/proto/consensus"
	prototypes "BFT/proto/types"
	"BFT/types"
	"errors"
	"fmt"
	"github.com/gogo/protobuf/proto"
)

func MsgToProto(msg Message) (*protoconsensus.Message, error) {
	if msg == nil {
		return nil, errors.New("consensus: message is nil")
	}
	var pb protoconsensus.Message

	switch msg := msg.(type) {
	case *NewRoundStepMessage:
		pb = protoconsensus.Message{
			Sum: &protoconsensus.Message_NewRoundStep{
				NewRoundStep: &protoconsensus.NewRoundStep{
					Height:                msg.Height,
					Round:                 msg.Round,
					Step:                  uint32(msg.Step),
					SecondsSinceStartTime: msg.SecondsSinceStartTime,
					LastReplyRound:       msg.LastCommitRound,
				},
			},
		}
	case *NewValidBlockMessage:
		pbPartSetHeader := msg.BlockPartSetHeader.ToProto()
		pbBits := msg.BlockParts.ToProto()
		pb = protoconsensus.Message{
			Sum: &protoconsensus.Message_NewValidBlock{
				NewValidBlock: &protoconsensus.NewValidBlock{
					Height:             msg.Height,
					Round:              msg.Round,
					BlockPartSetHeader: pbPartSetHeader,
					BlockParts:         pbBits,
					IsReply:           msg.IsCommit,
				},
			},
		}
	case *PrePrepareMessage:
		pbP := msg.PrePrepare.ToProto()
		pb = protoconsensus.Message{
			Sum: &protoconsensus.Message_PrePrepare{
				PrePrepare: &protoconsensus.PrePrepare{
					PrePrepare: *pbP,
				},
			},
		}
	case *PrePreparePOLMessage:
		pbBits := msg.PrePreparePOL.ToProto()
		pb = protoconsensus.Message{
			Sum: &protoconsensus.Message_PrePreparePol{
				PrePreparePol: &protoconsensus.PrePreparePOL{
					Height:           msg.Height,
					PrePreparePolRound: msg.PrePreparePOLRound,
					PrePreparePol:      *pbBits,
				},
			},
		}
	case *BlockPartMessage:
		parts, err := msg.Part.ToProto()
		if err != nil {
			return nil, fmt.Errorf("msg to proto error: %w", err)
		}
		pb = protoconsensus.Message{
			Sum: &protoconsensus.Message_BlockPart{
				BlockPart: &protoconsensus.BlockPart{
					Height: msg.Height,
					Round:  msg.Round,
					Part:   *parts,
				},
			},
		}
	case *VoteMessage:
		vote := msg.Vote.ToProto()
		pb = protoconsensus.Message{
			Sum: &protoconsensus.Message_Vote{
				Vote: &protoconsensus.Vote{
					Vote: vote,
				},
			},
		}
	case *HasVoteMessage:
		pb = protoconsensus.Message{
			Sum: &protoconsensus.Message_HasVote{
				HasVote: &protoconsensus.HasVote{
					Height: msg.Height,
					Round:  msg.Round,
					Type:   msg.Type,
					Index:  msg.Index,
				},
			},
		}
	case *VoteSetMaj23Message:
		bi := msg.BlockID.ToProto()
		pb = protoconsensus.Message{
			Sum: &protoconsensus.Message_VoteSetMaj23{
				VoteSetMaj23: &protoconsensus.VoteSetMaj23{
					Height:  msg.Height,
					Round:   msg.Round,
					Type:    msg.Type,
					BlockID: bi,
				},
			},
		}
	case *VoteSetBitsMessage:
		bi := msg.BlockID.ToProto()
		bits := msg.Votes.ToProto()

		vsb := &protoconsensus.Message_VoteSetBits{
			VoteSetBits: &protoconsensus.VoteSetBits{
				Height:  msg.Height,
				Round:   msg.Round,
				Type:    msg.Type,
				BlockID: bi,
			},
		}

		if bits != nil {
			vsb.VoteSetBits.Votes = *bits
		}

		pb = protoconsensus.Message{
			Sum: vsb,
		}

	default:
		return nil, fmt.Errorf("consensus: message not recognized: %T", msg)
	}

	return &pb, nil
}

func MsgFromProto(msg *protoconsensus.Message) (Message, error) {
	if msg == nil {
		return nil, errors.New("consensus: nil message")
	}
	var pb Message

	switch msg := msg.Sum.(type) {
	case *protoconsensus.Message_NewRoundStep:
		rs, err := srmath.SafeConvertUint8(int64(msg.NewRoundStep.Step))
		// deny message based on possible overflow
		if err != nil {
			return nil, fmt.Errorf("denying message due to possible overflow: %w", err)
		}
		pb = &NewRoundStepMessage{
			Height:                msg.NewRoundStep.Height,
			Round:                 msg.NewRoundStep.Round,
			Step:                  RoundStepType(rs),
			SecondsSinceStartTime: msg.NewRoundStep.SecondsSinceStartTime,
			LastCommitRound:       msg.NewRoundStep.LastReplyRound,
		}
	case *protoconsensus.Message_NewValidBlock:
		pbPartSetHeader, err := types.PartSetHeaderFromProto(&msg.NewValidBlock.BlockPartSetHeader)
		if err != nil {
			return nil, fmt.Errorf("parts to proto error: %w", err)
		}

		pbBits := new(bits.BitArray)
		pbBits.FromProto(msg.NewValidBlock.BlockParts)

		pb = &NewValidBlockMessage{
			Height:             msg.NewValidBlock.Height,
			Round:              msg.NewValidBlock.Round,
			BlockPartSetHeader: *pbPartSetHeader,
			BlockParts:         pbBits,
			IsCommit:           msg.NewValidBlock.IsReply,
		}
	case *protoconsensus.Message_PrePrepare:
		pbP, err := types.PrePrepareFromProto(&msg.PrePrepare.PrePrepare)
		if err != nil {
			return nil, fmt.Errorf("proposal msg to proto error: %w", err)
		}

		pb = &PrePrepareMessage{
			PrePrepare: pbP,
		}
	case *protoconsensus.Message_PrePreparePol:
		pbBits := new(bits.BitArray)
		pbBits.FromProto(&msg.PrePreparePol.PrePreparePol)
		pb = &PrePreparePOLMessage{
			Height:             msg.PrePreparePol.Height,
			PrePreparePOLRound: msg.PrePreparePol.PrePreparePolRound,
			PrePreparePOL:      pbBits,
		}
	case *protoconsensus.Message_BlockPart:
		parts, err := types.PartFromProto(&msg.BlockPart.Part)
		if err != nil {
			return nil, fmt.Errorf("blockpart msg to proto error: %w", err)
		}
		pb = &BlockPartMessage{
			Height: msg.BlockPart.Height,
			Round:  msg.BlockPart.Round,
			Part:   parts,
		}
	case *protoconsensus.Message_Vote:
		vote, err := types.VoteFromProto(msg.Vote.Vote)
		if err != nil {
			return nil, fmt.Errorf("vote msg to proto error: %w", err)
		}

		pb = &VoteMessage{
			Vote: vote,
		}
	case *protoconsensus.Message_HasVote:
		pb = &HasVoteMessage{
			Height: msg.HasVote.Height,
			Round:  msg.HasVote.Round,
			Type:   msg.HasVote.Type,
			Index:  msg.HasVote.Index,
		}
	case *protoconsensus.Message_VoteSetMaj23:
		bi, err := types.BlockIDFromProto(&msg.VoteSetMaj23.BlockID)
		if err != nil {
			return nil, fmt.Errorf("voteSetMaj23 msg to proto error: %w", err)
		}
		pb = &VoteSetMaj23Message{
			Height:  msg.VoteSetMaj23.Height,
			Round:   msg.VoteSetMaj23.Round,
			Type:    msg.VoteSetMaj23.Type,
			BlockID: *bi,
		}
	case *protoconsensus.Message_VoteSetBits:
		bi, err := types.BlockIDFromProto(&msg.VoteSetBits.BlockID)
		if err != nil {
			return nil, fmt.Errorf("voteSetBits msg to proto error: %w", err)
		}
		bits := new(bits.BitArray)
		bits.FromProto(&msg.VoteSetBits.Votes)

		pb = &VoteSetBitsMessage{
			Height:  msg.VoteSetBits.Height,
			Round:   msg.VoteSetBits.Round,
			Type:    msg.VoteSetBits.Type,
			BlockID: *bi,
			Votes:   bits,
		}
	default:
		return nil, fmt.Errorf("consensus: message not recognized: %T", msg)
	}

	if err := pb.ValidateBasic(); err != nil {
		return nil, err
	}

	return pb, nil
}

// MustEncode 先将 Message 转换为 protobuf 形式，然后利用 Marshal 对其进行编码
func MustEncode(msg Message) []byte {
	pb, err := MsgToProto(msg)
	if err != nil {
		panic(err)
	}
	enc, err := proto.Marshal(pb)
	if err != nil {
		panic(err)
	}
	return enc
}

// WALToProto 将 go 语言内生的 WALMessage 转换为 protobuf 形式的 protoprotoconsensus.WALMessage，
// 这其中包括以下 WALMessage：
//	1. types.EventDataRoundState: prototypes.EventDataRoundState
//	2. msgInfo: protoprotoconsensus.MsgInfo
//	3. timeoutInfo: protoprotoconsensus.TimeoutInfo
//	4. EndHeightMessage: protoprotoconsensus.EndHeight
func WALToProto(msg WALMessage) (*protoconsensus.WALMessage, error) {
	var pb protoconsensus.WALMessage

	switch msg := msg.(type) {
	case types.EventDataRoundState:
		pb = protoconsensus.WALMessage{
			Sum: &protoconsensus.WALMessage_EventDataRoundState{
				EventDataRoundState: &prototypes.EventDataRoundState{
					Height: msg.Height,
					Round:  msg.Round,
					Step:   msg.Step,
				},
			},
		}
	case msgInfo:
		consMsg, err := MsgToProto(msg.Msg)
		if err != nil {
			return nil, err
		}
		pb = protoconsensus.WALMessage{
			Sum: &protoconsensus.WALMessage_MsgInfo{
				MsgInfo: &protoconsensus.MsgInfo{
					Msg:    *consMsg,
					PeerID: string(msg.PeerID),
				},
			},
		}
	case timeoutInfo:
		pb = protoconsensus.WALMessage{
			Sum: &protoconsensus.WALMessage_TimeoutInfo{
				TimeoutInfo: &protoconsensus.TimeoutInfo{
					Duration: msg.Duration,
					Height:   msg.Height,
					Round:    msg.Round,
					Step:     uint32(msg.Step),
				},
			},
		}
	case EndHeightMessage:
		pb = protoconsensus.WALMessage{
			Sum: &protoconsensus.WALMessage_EndHeight{
				EndHeight: &protoconsensus.EndHeight{
					Height: msg.Height,
				},
			},
		}
	default:
		return nil, fmt.Errorf("to proto: wal message not recognized: %T", msg)
	}

	return &pb, nil
}

// WALFromProto 将 protobuf 形式的 WALMessage 转换为 go 语言内生的 WALMessage：
//	1. protoprotoconsensus.WALMessage_EventDataRoundState: types.EventDataRoundState
//	2. protoprotoconsensus.WALMessage_MsgInfo: msgInfo
//	3. protoprotoconsensus.WALMessage_TimeoutInfo: timeoutInfo
//	4. protoprotoconsensus.WALMessage_EndHeight: EndHeightMessage
func WALFromProto(msg *protoconsensus.WALMessage) (WALMessage, error) {
	if msg == nil {
		return nil, errors.New("nil WAL message")
	}
	var pb WALMessage

	switch msg := msg.Sum.(type) {
	case *protoconsensus.WALMessage_EventDataRoundState:
		pb = types.EventDataRoundState{
			Height: msg.EventDataRoundState.Height,
			Round:  msg.EventDataRoundState.Round,
			Step:   msg.EventDataRoundState.Step,
		}
	case *protoconsensus.WALMessage_MsgInfo:
		walMsg, err := MsgFromProto(&msg.MsgInfo.Msg)
		if err != nil {
			return nil, fmt.Errorf("msgInfo from proto error: %w", err)
		}
		pb = msgInfo{
			Msg:    walMsg,
			PeerID: gossip.ID(msg.MsgInfo.PeerID),
		}

	case *protoconsensus.WALMessage_TimeoutInfo:
		tis, err := srmath.SafeConvertUint8(int64(msg.TimeoutInfo.Step))
		// deny message based on possible overflow
		if err != nil {
			return nil, fmt.Errorf("denying message due to possible overflow: %w", err)
		}
		pb = timeoutInfo{
			Duration: msg.TimeoutInfo.Duration,
			Height:   msg.TimeoutInfo.Height,
			Round:    msg.TimeoutInfo.Round,
			Step:     RoundStepType(tis),
		}
		return pb, nil
	case *protoconsensus.WALMessage_EndHeight:
		pb := EndHeightMessage{
			Height: msg.EndHeight.Height,
		}
		return pb, nil
	default:
		return nil, fmt.Errorf("from proto: wal message not recognized: %T", msg)
	}
	return pb, nil
}
