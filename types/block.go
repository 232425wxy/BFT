package types

import (
	srbytes "github.com/232425wxy/BFT/libs/bytes"
	"github.com/232425wxy/BFT/proto/types"
	"bytes"
	"errors"
	"fmt"
	"github.com/gogo/protobuf/proto"
	"sync"
)

const (
	// MaxHeaderBytes 是最大 header 大小。
	// 注意:因为应用程序哈希可以是任意大小，因此 header 的大小没有上限，因此这个数字应该被视为软限制
	MaxHeaderBytes int64 = 626

	// MaxOverheadForBlock 编码一个区块的最大开销
	//
	// Uvarint length of MaxBlockSizeBytes: 4 bytes
	// 2 fields (2 embedded):               2 bytes
	// Uvarint length of Data.Txs:          4 bytes
	// Data.Txs field:                      1 byte
	MaxOverheadForBlock int64 = 11
)

// Block 定义了区块链的单元——区块
type Block struct {
	mtx sync.Mutex
	Header     `json:"header"`
	Data      `json:"data"`
	LastReply *Reply `json:"last_commit"`
}

// MakeBlock returns a new block with an empty header, except what can be
// computed from itself.
// It populates the same set of fields validated by ValidateBasic.
func MakeBlock(height int64, txs []Tx, lastCommit *Reply) *Block {
	block := &Block{
		Header: Header{
			Height: height,
		},
		Data: Data{
			Txs: txs,
		},
		LastReply: lastCommit,
	}
	block.fillHeader()
	return block
}

// ValidateBasic 执行不涉及状态数据的基本验证。
// 它检查块的内部一致性。
// 进一步的验证使用state#ValidateBlock。
func (b *Block) ValidateBasic() error {
	if b == nil {
		return errors.New("nil block")
	}

	b.mtx.Lock()
	defer b.mtx.Unlock()

	if err := b.Header.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid header: %w", err)
	}

	// 验证上一次 commit 和它的哈希值
	if b.LastReply == nil {
		return errors.New("nil LastCommits")
	}
	if err := b.LastReply.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong LastCommits: %v", err)
	}

	if !bytes.Equal(b.LastCommitHash, b.LastReply.Hash()) {
		return fmt.Errorf("wrong Header.LastCommitHash. Expected %v, got %v",
			b.LastReply.Hash(),
			b.LastCommitHash,
		)
	}

	// 注意:b.Data.Txs 可以为 nil，但是 b.Data.Hash() 仍然可以正常工作。
	if !bytes.Equal(b.DataHash, b.Data.Hash()) {
		return fmt.Errorf(
			"wrong Header.DataHash. Expected %v, got %v",
			b.Data.Hash(),
			b.DataHash,
		)
	}

	return nil
}

// fillHeader 填充 Header.LastCommitHash / Header.DataHash
//	主要就是计算 Header 中相关字段的根哈希，感觉只要某个字段是切片形式的，都会用一个根哈希
//	来标识它们
func (b *Block) fillHeader() {
	// b.LastCommitHash 是 Header 里的字段
	if b.LastCommitHash == nil {
		b.LastCommitHash = b.LastReply.Hash()
	}
	// b.DataHash 是 Header 里的字段
	if b.DataHash == nil {
		b.DataHash = b.Data.Hash()
	}
}

// Hash 计算区块哈希值，如果 Block=nil，或者 Block.LastReply=nil，
// 则为了安全，会返回 nil，否则返回 Header.Hash()
func (b *Block) Hash() srbytes.HexBytes {
	if b == nil {
		return nil
	}
	b.mtx.Lock()
	defer b.mtx.Unlock()

	if b.LastReply == nil {
		return nil
	}
	b.fillHeader()
	return b.Header.Hash()
}

// MakePartSet 将一个区块拆分成若干个 Part，组成一个 PartSet，
// 这是一种将区块传递给其他节点的方式。
// 注意：partSize 必须大于 0
func (b *Block) MakePartSet(partSize uint32) *PartSet {
	if b == nil {
		return nil
	}
	b.mtx.Lock()
	defer b.mtx.Unlock()

	pbb, err := b.ToProto()
	if err != nil {
		panic(err)
	}
	bz, err := proto.Marshal(pbb)
	if err != nil {
		panic(err)
	}
	return NewPartSetFromData(bz, partSize)
}

// EqualsTo 判断区块的哈希值是否等于给定的哈希值
func (b *Block) EqualsTo(hash []byte) bool {
	if len(hash) == 0 {
		return false
	}
	if b == nil {
		return false
	}
	return bytes.Equal(b.Hash(), hash)
}

// Size 返回区块数据的大小，单位是字节。
// 先将 Block 转换为 protobuf 形式，然后调用 protobuf 的 Size() 方法返回大小
func (b *Block) Size() int {
	pbb, err := b.ToProto()
	if err != nil {
		return 0
	}

	return pbb.Size()
}

// String returns a string representation of the block
//
// See StringIndented.
func (b *Block) String() string {
	return b.StringIndented("")
}

// StringIndented returns an indented String.
//
// Header
// Data
// LastReply
// Hash
func (b *Block) StringIndented(indent string) string {
	if b == nil {
		return "nil-Block"
	}
	return fmt.Sprintf(`Block{
%s  %v
%s  %v
%s  %v
%s}#%v`,
		indent, b.Header.StringIndented(indent+"  "),
		indent, b.Data.StringIndented(indent+"  "),
		indent, b.LastReply.StringIndented(indent+"  "),
		indent, b.Hash())
}

// StringShort returns a shortened string representation of the block.
func (b *Block) StringShort() string {
	if b == nil {
		return "nil-Block"
	}
	return fmt.Sprintf("Block#%X", b.Hash())
}

// ToProto converts Block to protobuf
func (b *Block) ToProto() (*types.Block, error) {
	if b == nil {
		return nil, errors.New("nil Block")
	}

	pb := new(types.Block)

	pb.Header = *b.Header.ToProto()
	pb.LastReply = b.LastReply.ToProto()
	pb.Data = b.Data.ToProto()

	return pb, nil
}

// BlockFromProto sets a protobuf Block to the given pointer.
// It returns an error if the block is invalid.
func BlockFromProto(bp *types.Block) (*Block, error) {
	if bp == nil {
		return nil, errors.New("nil block")
	}

	b := new(Block)
	h, err := HeaderFromProto(&bp.Header)
	if err != nil {
		return nil, err
	}
	b.Header = h
	data, err := DataFromProto(&bp.Data)
	if err != nil {
		return nil, err
	}
	b.Data = data

	if bp.LastReply != nil {
		lc, err := ReplyFromProto(bp.LastReply)
		if err != nil {
			return nil, err
		}
		b.LastReply = lc
	}

	return b, b.ValidateBasic()
}

//-----------------------------------------------------------------------------

// MaxDataBytes 返回一个区块数据部分的最大大小。
//
// XXX: Panics on negative result.
func MaxDataBytes(maxBytes int64, valsCount int) int64 {
	maxDataBytes := maxBytes -
		MaxOverheadForBlock -
		MaxHeaderBytes -
		MaxReplyBytes(valsCount)

	if maxDataBytes < 0 {
		panic(fmt.Sprintf(
			"Negative MaxDataBytes. Block.MaxBytes=%d is too small to accommodate header&lastCommit=%d", maxBytes, -(maxDataBytes - maxBytes)))
	}

	return maxDataBytes
}

// MaxBlockDataBytes 返回块一个区块数据部分的最大大小。
func MaxBlockDataBytes(maxBytes int64, valsCount int) int64 {
	maxDataBytes := maxBytes -
		MaxOverheadForBlock -
		MaxHeaderBytes -
		MaxReplyBytes(valsCount)

	if maxDataBytes < 0 {
		panic(fmt.Sprintf("Block.MaxBytes=%d is too small to accommodate header&lastCommit&=%d", maxBytes, -(maxDataBytes - maxBytes)))
	}

	return maxDataBytes
}

//-------------------------------------

// BlockIDFlag 表示签名是针对哪个 BlockID。
type BlockIDFlag byte

const (
	// BlockIDFlagAbsent - 没有从验证器收到投票。
	BlockIDFlagAbsent BlockIDFlag = iota + 1
	// BlockIDFlagCommit - 为 Reply.BlockID 投票。
	BlockIDFlagCommit
	// BlockIDFlagNil - voted for nil.
	BlockIDFlagNil
)

const (
	// MaxReplyOverheadBytes
	// Max size of commit without any commitSigs -> 82 for BlockID, 8 for Height, 4 for Round.
	MaxReplyOverheadBytes int64 = 94
	// MaxReplySigBytes
	// Reply sig size is made up of 64 bytes for the signature, 20 bytes for the address,
	// 1 byte for the flag and 14 bytes for the timestamp
	MaxReplySigBytes int64 = 99 // 为什么会多出 10 字节
)
