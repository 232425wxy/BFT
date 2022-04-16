package types

import (
	"github.com/232425wxy/BFT/crypto"
	"github.com/232425wxy/BFT/crypto/merkle"
	"github.com/232425wxy/BFT/crypto/srhash"
	srbytes "github.com/232425wxy/BFT/libs/bytes"
	"github.com/232425wxy/BFT/proto/types"
	"errors"
	"fmt"
	gogotypes "github.com/gogo/protobuf/types"
	"time"
)

// Header defines the structure of a Tendermint block header.
// NOTE: changes to the Header should be duplicated in:
// - header.Hash()
// - abci.Header
type Header struct {
	// basic block info
	// ChainID 区块链的 ID，随便起一个，例如 “test”
	ChainID string    `json:"chain_id"`
	Height  int64     `json:"height"`
	Time    time.Time `json:"time"`

	// 上一个区块的信息
	LastBlockID BlockID `json:"last_block_id"`

	// commit 上一个区块时，将 Reply.Signatures 里的每个 ReplySig 转换为 protobuf Marshal
	// 编码的字节数组，然后将它们作为 merkle 的叶子节点，计算 merkle 的根哈希值，将其作为 Reply
	// 的哈希值，然后此处 LastCommitHash 就是 commit 上一个区块时，产生的 Reply 的哈希值
	LastCommitHash srbytes.HexBytes `json:"last_commit_hash"`
	// 将每个 tx 作为 merkle 的叶子节点，然后计算 merkle 的根哈希，作为 DataHash
	DataHash srbytes.HexBytes `json:"data_hash"` // transactions

	// 当前区块共识参数的哈希值，只计算了共识参数里的一个字段的哈希值：
	//	- params.Block.MaxBytes
	ConsensusHash srbytes.HexBytes `json:"consensus_hash"`
	// 前一个区块 commit 后的最新 state
	AppHash srbytes.HexBytes `json:"app_hash"`

	ProposerAddress Address          `json:"proposer_address"` // original proposer of the block
}

// Populate 用 state 里的数据填充 Header。在 MakeBlock 之后调用它来使 Header 完整
func (h *Header) Populate(
	chainID string,
	timestamp time.Time, lastBlockID BlockID,
	valHash, nextValHash []byte,
	consensusHash, appHash, lastResultsHash []byte,
	proposerAddress Address,
) {
	h.ChainID = chainID
	h.Time = timestamp
	h.LastBlockID = lastBlockID
	h.ConsensusHash = consensusHash
	h.AppHash = appHash
	h.ProposerAddress = proposerAddress
}

// ValidateBasic performs stateless validation on a Header returning an error
// if any validation fails.
//
// NOTE: Timestamp validation is subtle and handled elsewhere.
func (h Header) ValidateBasic() error {
	if len(h.ChainID) > MaxChainIDLen {
		return fmt.Errorf("chainID is too long; got: %d, max: %d", len(h.ChainID), MaxChainIDLen)
	}

	if h.Height < 0 {
		return errors.New("negative Height")
	} else if h.Height == 0 {
		return errors.New("zero Height")
	}

	if err := h.LastBlockID.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong LastBlockID: %w", err)
	}

	if err := srhash.ValidateHash(h.LastCommitHash); err != nil {
		return fmt.Errorf("wrong LastCommitHash: %v", err)
	}

	if err := srhash.ValidateHash(h.DataHash); err != nil {
		return fmt.Errorf("wrong DataHash: %v", err)
	}

	if len(h.ProposerAddress) != crypto.AddressSize {
		return fmt.Errorf(
			"invalid ProposerAddress length; got: %d, expected: %d",
			len(h.ProposerAddress), crypto.AddressSize,
		)
	}

	// Basic validation of hashes related to application data.
	// Will validate fully against state in state#ValidateBlock.
	if err := srhash.ValidateHash(h.ConsensusHash); err != nil {
		return fmt.Errorf("wrong ConsensusHash: %v", err)
	}

	return nil
}

// Hash 返回 Header 的哈希值。
// 根据报头字段的顺序，计算出 Merkle 的根哈希。
// 如果 ValidatorHash 缺失则返回 nil，因为 Header 是无效的。
func (h *Header) Hash() srbytes.HexBytes {
	pbt, err := gogotypes.StdTimeMarshal(h.Time)
	if err != nil {
		return nil
	}

	pbbi := h.LastBlockID.ToProto()
	bzbi, err := pbbi.Marshal()
	if err != nil {
		return nil
	}
	return merkle.HashFromByteSlices([][]byte{
		cdcEncode(h.ChainID),
		cdcEncode(h.Height),
		pbt,
		bzbi,
		cdcEncode(h.LastCommitHash),
		cdcEncode(h.DataHash),
		cdcEncode(h.ConsensusHash),
		cdcEncode(h.AppHash),
		cdcEncode(h.ProposerAddress),
	})
}

// StringIndented returns an indented string representation of the header.
func (h *Header) StringIndented(indent string) string {
	if h == nil {
		return "nil-Header"
	}
	return fmt.Sprintf(`Header{
%s  ChainID:        %v
%s  Height:         %v
%s  Time:           %v
%s  LastBlockID:    %v
%s  LastCommits:     %v
%s  Data:           %v
%s  App:            %v
%s  Consensus:      %v
%s  Primary:       %v
%s}#%v`,
		indent, h.ChainID,
		indent, h.Height,
		indent, h.Time,
		indent, h.LastBlockID,
		indent, h.LastCommitHash,
		indent, h.DataHash,
		indent, h.AppHash,
		indent, h.ConsensusHash,
		indent, h.ProposerAddress,
		indent, h.Hash())
}

// ToProto converts Header to protobuf
func (h *Header) ToProto() *types.Header {
	if h == nil {
		return nil
	}

	return &types.Header{
		ChainID:            h.ChainID,
		Height:             h.Height,
		Time:               h.Time,
		LastBlockId:        h.LastBlockID.ToProto(),
		ConsensusHash:      h.ConsensusHash,
		AppHash:            h.AppHash,
		DataHash:           h.DataHash,
		LastCommitHash:     h.LastCommitHash,
		ProposerAddress:    h.ProposerAddress,
	}
}

// HeaderFromProto sets a protobuf Header to the given pointer.
// It returns an error if the header is invalid.
func HeaderFromProto(ph *types.Header) (Header, error) {
	if ph == nil {
		return Header{}, errors.New("nil Header")
	}

	h := new(Header)

	bi, err := BlockIDFromProto(&ph.LastBlockId)
	if err != nil {
		return Header{}, err
	}

	h.ChainID = ph.ChainID
	h.Height = ph.Height
	h.Time = ph.Time
	h.LastBlockID = *bi
	h.ConsensusHash = ph.ConsensusHash
	h.AppHash = ph.AppHash
	h.DataHash = ph.DataHash
	h.LastCommitHash = ph.LastCommitHash
	h.ProposerAddress = ph.ProposerAddress

	return *h, h.ValidateBasic()
}
