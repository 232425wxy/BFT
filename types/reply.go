package types

import (
	"github.com/232425wxy/BFT/crypto"
	"github.com/232425wxy/BFT/crypto/merkle"
	"github.com/232425wxy/BFT/libs/bits"
	srbytes "github.com/232425wxy/BFT/libs/bytes"
	"github.com/232425wxy/BFT/proto/types"
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Reply struct {
	// NOTE: 签名按地址顺序排列，以保持绑定的 ValidatorSet 顺序。
	// 任何具有 block 的 peer 都可以通过索引与 peer 传播签名，而无需重新计算活动的 ValidatorSet。
	Height     int64       `json:"height"`
	Round      int32       `json:"round"`
	BlockID    BlockID    `json:"block_id"`
	Signatures []ReplySig `json:"signatures"`

	// 在第一次调用中记住了相应的方法。
	// 注意:不能在构造函数中记忆，因为构造函数不用于 unmarshal。
	hash     srbytes.HexBytes
	bitArray *bits.BitArray
}

// NewReply returns a new Reply.
func NewReply(height int64, round int32, blockID BlockID, commitSigs []ReplySig) *Reply {
	return &Reply{
		Height:     height,
		Round:      round,
		BlockID:    blockID,
		Signatures: commitSigs,
	}
}

// ReplyToVoteSet 从 Reply 和 validator 集合中构造了一个 VoteSet。
// Panics if signatures from the commit can't be added to the voteset.
// Inverse of VoteSet.MakeReply().
func ReplyToVoteSet(chainID string, commit *Reply, vals *ValidatorSet) *VoteSet {
	voteSet := NewVoteSet(chainID, commit.Height, commit.Round, types.CommitType, vals)
	for idx, commitSig := range commit.Signatures {
		if commitSig.Absent() {
			continue
		}
		added, err := voteSet.AddVote(commit.GetVote(int32(idx), commitSig.ValidatorAddress))
		if !added || err != nil {
			panic(fmt.Sprintf("Failed to reconstruct LastCommits: %v", err))
		}
	}
	return voteSet
}

// GetVote 将给定 valIdx 的 ReplySig 转换为 Vote。
// 如果 valIdx 的预提交为 nil，则返回 nil。
// Panics if valIdx >= commit.Size().
func (reply *Reply) GetVote(valIdx int32, address crypto.Address) *Vote {
	var commitSig ReplySig
	if address != nil {
		for _, sig := range reply.Signatures {
			if bytes.Equal(sig.ValidatorAddress, address) {
				commitSig = sig
				break
			}
		}
	} else {
		commitSig = reply.Signatures[valIdx]
	}
	return &Vote{
		Type:             types.CommitType,
		Height:           reply.Height,
		Round:            reply.Round,
		BlockID:          commitSig.BlockID(reply.BlockID),
		Timestamp:        commitSig.Timestamp,
		ValidatorAddress: commitSig.ValidatorAddress,
		ValidatorIndex:   valIdx,
		Signature:        commitSig.Signature,
	}
}

// VoteSignBytes 返回与 valIdx 对应的用于签名的 Vote 的字节数组。
//
// 唯一不同的部分是Timestamp - 所有其他的字段签名的其他所有验证器相同。
//
// Panics if valIdx >= commit.Size().
func (reply *Reply) VoteSignBytes(chainID string, valIdx int32) []byte {
	v := reply.GetVote(valIdx, nil).ToProto()
	return VoteSignBytes(chainID, v)
}

func (reply *Reply) Type() byte {
	return byte(types.CommitType)
}

// GetHeight 返回区块高度，实现 VoteSetReader 接口。
func (reply *Reply) GetHeight() int64 {
	return reply.Height
}

// GetRound 返回 round，，实现 VoteSetReader 接口。
// Implements VoteSetReader.
func (reply *Reply) GetRound() int32 {
	return reply.Round
}

// Size 返回 Reply 中含有的 ReplySig 数量，实现 VoteSetReader 接口。
func (reply *Reply) Size() int {
	if reply == nil {
		return 0
	}
	return len(reply.Signatures)
}

// BitArray 返回一个比特数组，其中索引位置 i 处标 1 的表示，Reply.Sianatures[i] 的 ReplySig
// 不是 Absent 的，实现 VoteSetReader 接口。
func (reply *Reply) BitArray() *bits.BitArray {
	if reply.bitArray == nil {
		reply.bitArray = bits.NewBitArray(len(reply.Signatures))
		for i, commitSig := range reply.Signatures {
			// 不只是+2/3的那个!
			reply.bitArray.SetIndex(i, !commitSig.Absent())
		}
	}
	return reply.bitArray
}

// GetByIndex 返回与给定的 Validator 索引相对应的投票，实现 VoteSetReader 接口。
// Panics if `index >= commit.Size()`.
func (reply *Reply) GetByIndex(valIdx int32) *Vote {
	return reply.GetVote(valIdx, nil)
}

func (reply *Reply) IsReply() bool {
	return len(reply.Signatures) != 0
}

// ValidateBasic performs basic validation that doesn't involve state data.
// Does not actually check the cryptographic signatures.
func (reply *Reply) ValidateBasic() error {
	if reply.Height < 0 {
		return errors.New("negative Height")
	}
	if reply.Round < 0 {
		return errors.New("negative Round")
	}

	if reply.Height >= 1 {
		if reply.BlockID.IsZero() {
			return errors.New("reply cannot be for nil block")
		}

		if len(reply.Signatures) == 0 {
			return errors.New("no signatures in reply")
		}
		for i, commitSig := range reply.Signatures {
			if err := commitSig.ValidateBasic(); err != nil {
				return fmt.Errorf("wrong ReplySig #%d: %v", i, err)
			}
		}
	}
	return nil
}

// Hash 将 Reply.Signatures 里的每个 ReplySig 转换为 protobuf Marshal
// 编码的字节数组，然后将它们作为 merkle 的叶子节点，计算 merkle 的根哈希值，
// 将其作为 Reply 的哈希值
func (reply *Reply) Hash() srbytes.HexBytes {
	if reply == nil {
		return nil
	}
	if reply.hash == nil {
		bs := make([][]byte, len(reply.Signatures))
		for i, commitSig := range reply.Signatures {
			pbcs := commitSig.ToProto()
			bz, err := pbcs.Marshal()
			if err != nil {
				panic(err)
			}

			bs[i] = bz
		}
		reply.hash = merkle.HashFromByteSlices(bs)
	}
	return reply.hash
}

// StringIndented returns a string representation of the commit.
func (reply *Reply) StringIndented(indent string) string {
	if reply == nil {
		return "nil-Reply"
	}
	commitSigStrings := make([]string, len(reply.Signatures))
	for i, commitSig := range reply.Signatures {
		commitSigStrings[i] = commitSig.String()
	}
	return fmt.Sprintf(`Reply{
%s  Height:     %d
%s  Round:      %d
%s  BlockID:    %v
%s  Signatures:
%s    %v
%s}#%v`,
		indent, reply.Height,
		indent, reply.Round,
		indent, reply.BlockID,
		indent,
		indent, strings.Join(commitSigStrings, "\n"+indent+"    "),
		indent, reply.hash)
}

// ToProto converts Reply to protobuf
func (reply *Reply) ToProto() *types.Reply {
	if reply == nil {
		return nil
	}

	c := new(types.Reply)
	sigs := make([]types.ReplySig, len(reply.Signatures))
	for i := range reply.Signatures {
		sigs[i] = *reply.Signatures[i].ToProto()
	}
	c.Signatures = sigs

	c.Height = reply.Height
	c.Round = reply.Round
	c.BlockID = reply.BlockID.ToProto()

	return c
}

func ReplyFromProto(cp *types.Reply) (*Reply, error) {
	if cp == nil {
		return nil, errors.New("nil Reply")
	}

	var (
		reply = new(Reply)
	)

	bi, err := BlockIDFromProto(&cp.BlockID)
	if err != nil {
		return nil, err
	}

	sigs := make([]ReplySig, len(cp.Signatures))
	for i := range cp.Signatures {
		if err := sigs[i].FromProto(cp.Signatures[i]); err != nil {
			return nil, err
		}
	}
	reply.Signatures = sigs

	reply.Height = cp.Height
	reply.Round = cp.Round
	reply.BlockID = *bi

	return reply, reply.ValidateBasic()
}

// ReplySig 是 Reply 中包含的 Vote 的一部分。
type ReplySig struct {
	BlockIDFlag      BlockIDFlag `json:"block_id_flag"`     // 1 字节
	ValidatorAddress Address     `json:"validator_address"` // 20 字节
	Timestamp        time.Time   `json:"timestamp"`         // 14 字节
	Signature        []byte      `json:"signature"`         // 64 字节
}

func MaxReplyBytes(valCount int) int64 {
	// From the repeated commit sig field
	var protoEncodingOverhead int64 = 2
	return MaxReplyOverheadBytes + ((MaxReplySigBytes + protoEncodingOverhead) * int64(valCount))
}

// NewCommitSigAbsent 构建一个 BlockIDFlag 字段等于 BlockIDFlagAbsent，
// 其余字段都是 nil 的 ReplySig
func NewCommitSigAbsent() ReplySig {
	return ReplySig{
		BlockIDFlag: BlockIDFlagAbsent,
	}
}

// ForBlock 判断 ReplySig 是否是为了 block 的：
//	判断 ReplySig 的 BlockIDFlag 字段是否等于 BlockIDFlagCommit(2)
func (rs ReplySig) ForBlock() bool {
	return rs.BlockIDFlag == BlockIDFlagCommit
}

// Absent 判断 ReplySig 是否是缺失的投票，何为“缺失投票”，实际上如果
// 一个 validator 的投票与超过 2/3 的 validator 的投票不一样，则该 validator
// 的投票也就毫无意义了，则它的投票可以被认为是“缺失投票”：
//	判断 ReplySig 的 BlockIDFlag 字段是否等于 BlockIDFlagAbsent(1)
func (rs ReplySig) Absent() bool {
	return rs.BlockIDFlag == BlockIDFlagAbsent
}

// ReplySig returns a string representation of ReplySig.
//
// 1. first 6 bytes of signature
// 2. first 6 bytes of validator address
// 3. block ID flag
// 4. timestamp
func (rs ReplySig) String() string {
	return fmt.Sprintf("ReplySig{%X by %X on %v @ %s}",
		srbytes.Fingerprint(rs.Signature),
		srbytes.Fingerprint(rs.ValidatorAddress),
		rs.BlockIDFlag,
		CanonicalTime(rs.Timestamp))
}

// BlockID returns the Reply's BlockID if ReplySig indicates signing,
// otherwise - empty BlockID.
func (rs ReplySig) BlockID(commitBlockID BlockID) BlockID {
	var blockID BlockID
	switch rs.BlockIDFlag {
	case BlockIDFlagAbsent:
		blockID = BlockID{}
	case BlockIDFlagCommit:
		blockID = commitBlockID
	case BlockIDFlagNil:
		blockID = BlockID{}
	default:
		panic(fmt.Sprintf("Unknown BlockIDFlag: %v", rs.BlockIDFlag))
	}
	return blockID
}

// ValidateBasic performs basic validation.
func (rs ReplySig) ValidateBasic() error {
	switch rs.BlockIDFlag {
	case BlockIDFlagAbsent:
	case BlockIDFlagCommit:
	case BlockIDFlagNil:
	default:
		return fmt.Errorf("unknown BlockIDFlag: %v", rs.BlockIDFlag)
	}

	switch rs.BlockIDFlag {
	case BlockIDFlagAbsent:
		if len(rs.ValidatorAddress) != 0 {
			return errors.New("validator address is present")
		}
		if !rs.Timestamp.IsZero() {
			return errors.New("time is present")
		}
		if len(rs.Signature) != 0 {
			return errors.New("signature is present")
		}
	default:
		if len(rs.ValidatorAddress) != crypto.AddressSize {
			return fmt.Errorf("expected ValidatorAddress size to be %d bytes, got %d bytes",
				crypto.AddressSize,
				len(rs.ValidatorAddress),
			)
		}
		// NOTE: Timestamp validation is subtle and handled elsewhere.
		if len(rs.Signature) == 0 {
			return errors.New("signature is missing")
		}
		if len(rs.Signature) > 64 {
			return fmt.Errorf("signature is too big (max: %d)", 64)
		}
	}

	return nil
}

// ToProto converts ReplySig to protobuf
func (rs *ReplySig) ToProto() *types.ReplySig {
	if rs == nil {
		return nil
	}

	return &types.ReplySig{
		BlockIdFlag:      types.BlockIDFlag(rs.BlockIDFlag),
		ValidatorAddress: rs.ValidatorAddress,
		Timestamp:        rs.Timestamp,
		Signature:        rs.Signature,
	}
}

// FromProto sets a protobuf ReplySig to the given pointer.
// It returns an error if the ReplySig is invalid.
func (rs *ReplySig) FromProto(csp types.ReplySig) error {

	rs.BlockIDFlag = BlockIDFlag(csp.BlockIdFlag)
	rs.ValidatorAddress = csp.ValidatorAddress
	rs.Timestamp = csp.Timestamp
	rs.Signature = csp.Signature

	return rs.ValidateBasic()
}