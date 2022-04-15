package types

import (
	"BFT/crypto/srhash"
	srbytes "BFT/libs/bytes"
	"BFT/proto/types"
	"bytes"
	"errors"
	"fmt"
)

// BlockID
type BlockID struct {
	Hash          srbytes.HexBytes `json:"hash"` // 区块哈希，即 Header 的 哈希值
	PartSetHeader PartSetHeader    `json:"parts"`
}

// Equals returns true if the BlockID matches the given BlockID
func (blockID BlockID) Equals(other BlockID) bool {
	return bytes.Equal(blockID.Hash, other.Hash) &&
		blockID.PartSetHeader.Equals(other.PartSetHeader)
}

// Key 将 BlockID.PartSetHeader 转换为 protobuf 形式，然后计算 proto 编码 bz，
// 最后将 BlockID.Hash 和 bz 组合到一起并返回：
//	return fmt.Sprint(string(blockID.Hash), string(bz))
func (blockID BlockID) Key() string {
	pbph := blockID.PartSetHeader.ToProto()
	bz, err := pbph.Marshal()
	if err != nil {
		panic(err)
	}

	return fmt.Sprint(string(blockID.Hash), string(bz))
}

// ValidateBasic performs basic validation.
func (blockID BlockID) ValidateBasic() error {
	// Hash can be empty in case of POLBlockID in PrePrepare.
	if err := srhash.ValidateHash(blockID.Hash); err != nil {
		return fmt.Errorf("wrong Hash")
	}
	if err := blockID.PartSetHeader.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong PartSetHeader: %v", err)
	}
	return nil
}

// IsZero 如果这是一个空块的BlockID，则返回true。
func (blockID BlockID) IsZero() bool {
	return len(blockID.Hash) == 0 &&
		blockID.PartSetHeader.IsZero()
}

// IsComplete 如果这是非空区块的有效 BlockID，则返回true。
//	完整性检查
func (blockID BlockID) IsComplete() bool {
	return len(blockID.Hash) == srhash.Size &&
		blockID.PartSetHeader.Total > 0 &&
		len(blockID.PartSetHeader.Hash) == srhash.Size
}

// String returns a human readable string representation of the BlockID.
//
// 1. hash
// 2. part set header
//
// See PartSetHeader#String
func (blockID BlockID) String() string {
	return fmt.Sprintf(`%v:%v`, blockID.Hash, blockID.PartSetHeader)
}

// ToProto converts BlockID to protobuf
func (blockID *BlockID) ToProto() types.BlockID {
	if blockID == nil {
		return types.BlockID{}
	}

	return types.BlockID{
		Hash:          blockID.Hash,
		PartSetHeader: blockID.PartSetHeader.ToProto(),
	}
}

// FromProto sets a protobuf BlockID to the given pointer.
// It returns an error if the block id is invalid.
func BlockIDFromProto(bID *types.BlockID) (*BlockID, error) {
	if bID == nil {
		return nil, errors.New("nil BlockID")
	}

	blockID := new(BlockID)
	ph, err := PartSetHeaderFromProto(&bID.PartSetHeader)
	if err != nil {
		return nil, err
	}

	blockID.PartSetHeader = *ph
	blockID.Hash = bID.Hash

	return blockID, blockID.ValidateBasic()
}

