package types

import (
	srbytes "github.com/232425wxy/BFT/libs/bytes"
	"github.com/232425wxy/BFT/libs/protoio"
	srtime "github.com/232425wxy/BFT/libs/time"
	prototypes "github.com/232425wxy/BFT/proto/types"
	"errors"
	"fmt"
	"time"
)

// PrePrepare 定义了协商一致的区块提案
type PrePrepare struct {
	Type   prototypes.SignedMsgType
	Height int64     `json:"height"`
	Round     int32     `json:"round"`
	POLRound  int32     `json:"pol_round"`
	BlockID   BlockID   `json:"block_id"`
	Timestamp time.Time `json:"timestamp"`
	Signature []byte    `json:"signature"`
}

// NewPrePrepare 实例化一个 PrePrepare
func NewPrePrepare(height int64, round int32, polRound int32, blockID BlockID) *PrePrepare {
	return &PrePrepare{
		Type:      prototypes.PrePrepareType,
		Height:    height,
		Round:     round,
		BlockID:   blockID,
		POLRound:  polRound,
		Timestamp: srtime.Now(),
	}
}

// ValidateBasic 对 PrePrepare 的字段做一些基本检查
func (p *PrePrepare) ValidateBasic() error {
	if p.Type != prototypes.PrePrepareType {
		return errors.New("invalid Type")
	}
	if p.Height < 0 {
		return errors.New("negative Height")
	}
	if p.Round < 0 {
		return errors.New("negative Round")
	}
	if p.POLRound < -1 {
		return errors.New("negative POLRound (exception: -1)")
	}
	if err := p.BlockID.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong BlockID: %v", err)
	}
	// ValidateBasic above would pass even if the BlockID was empty:
	if !p.BlockID.IsComplete() {
		return fmt.Errorf("expected a complete, non-empty BlockID, got: %v", p.BlockID)
	}

	// NOTE: Timestamp validation is subtle and handled elsewhere.

	if len(p.Signature) == 0 {
		return errors.New("signature is missing")
	}

	if len(p.Signature) > 64 {
		return fmt.Errorf("signature is too big (max: %d)", 64)
	}
	return nil
}

func (p *PrePrepare) String() string {
	return fmt.Sprintf("PrePrepare{[Height:%v, Round:%v, BlockID:%v, POLRound:%v, Signature:%X] @ %s}",
		p.Height,
		p.Round,
		p.BlockID,
		p.POLRound,
		srbytes.Fingerprint(p.Signature),
		// CanonicalTime可用于以规范的方式将时间字符串化
		CanonicalTime(p.Timestamp))
}

func PrePrepareSignBytes(chainID string, p *prototypes.PrePrepare) []byte {
	pb := CanonicalizePrePrepare(chainID, p)
	bz, err := protoio.MarshalDelimited(&pb)
	if err != nil {
		panic(err)
	}

	return bz
}

func (p *PrePrepare) ToProto() *prototypes.PrePrepare {
	if p == nil {
		return &prototypes.PrePrepare{}
	}
	pb := new(prototypes.PrePrepare)

	pb.BlockID = p.BlockID.ToProto()
	pb.Type = p.Type
	pb.Height = p.Height
	pb.Round = p.Round
	pb.PolRound = p.POLRound
	pb.Timestamp = p.Timestamp
	pb.Signature = p.Signature

	return pb
}

func PrePrepareFromProto(pp *prototypes.PrePrepare) (*PrePrepare, error) {
	if pp == nil {
		return nil, errors.New("nil PrePrepare")
	}

	p := new(PrePrepare)

	blockID, err := BlockIDFromProto(&pp.BlockID)
	if err != nil {
		return nil, err
	}

	p.BlockID = *blockID
	p.Type = pp.Type
	p.Height = pp.Height
	p.Round = pp.Round
	p.POLRound = pp.PolRound
	p.Timestamp = pp.Timestamp
	p.Signature = pp.Signature

	return p, p.ValidateBasic()
}
