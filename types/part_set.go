package types

import (
	"github.com/232425wxy/BFT/crypto/merkle"
	"github.com/232425wxy/BFT/crypto/srhash"
	"github.com/232425wxy/BFT/libs/bits"
	srbytes "github.com/232425wxy/BFT/libs/bytes"
	srjson "github.com/232425wxy/BFT/libs/json"
	srmath "github.com/232425wxy/BFT/libs/math"
	prototypes "github.com/232425wxy/BFT/proto/types"
	"bytes"
	"errors"
	"fmt"
	"io"
	"sync"
)

var (
	ErrPartSetUnexpectedIndex = errors.New("error part set unexpected index")
	ErrPartSetInvalidProof    = errors.New("error part set invalid proof")
)

type Part struct {
	Index uint32           `json:"index"`
	Bytes srbytes.HexBytes `json:"bytes"`
	Proof merkle.Proof     `json:"proof"`
}

// ValidateBasic 验证 Part 内部字段的一致性
func (part *Part) ValidateBasic() error {
	if len(part.Bytes) > int(BlockPartSizeBytes) {
		return fmt.Errorf("too big: %d bytes, max: %d", len(part.Bytes), BlockPartSizeBytes)
	}
	if err := part.Proof.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong Proof: %w", err)
	}
	return nil
}

// String 返回 Part 的字符串打印形式
func (part *Part) String() string {
	return part.StringIndented("")
}

// StringIndented returns an indented Part.
//
// See merkle.Proof#StringIndented
func (part *Part) StringIndented(indent string) string {
	return fmt.Sprintf(`Part{#%v
%s  Bytes: %X...
%s  Proof: %v
%s}`,
		part.Index,
		indent, srbytes.Fingerprint(part.Bytes),
		indent, part.Proof.StringIndented(indent+"  "),
		indent)
}

func (part *Part) ToProto() (*prototypes.Part, error) {
	if part == nil {
		return nil, errors.New("nil part")
	}
	pb := new(prototypes.Part)
	proof := part.Proof.ToProto()

	pb.Index = part.Index
	pb.Bytes = part.Bytes
	pb.Proof = *proof

	return pb, nil
}

func PartFromProto(pb *prototypes.Part) (*Part, error) {
	if pb == nil {
		return nil, errors.New("nil part")
	}

	part := new(Part)
	proof, err := merkle.ProofFromProto(&pb.Proof)
	if err != nil {
		return nil, err
	}
	part.Index = pb.Index
	part.Bytes = pb.Bytes
	part.Proof = *proof

	return part, part.ValidateBasic()
}

//-------------------------------------

type PartSetHeader struct {
	Total uint32           `json:"total"` // Part 的总数
	Hash  srbytes.HexBytes `json:"hash"`  // 将 Part 作为 merkle 的叶子节点，构成的 merkle 树的根哈希值
}

// String returns a string representation of PartSetHeader.
//
// 1. total number of parts
// 2. first 6 bytes of the hash
func (psh PartSetHeader) String() string {
	return fmt.Sprintf("%v:%X", psh.Total, srbytes.Fingerprint(psh.Hash))
}

// IsZero 判断 PartSetHeader.Total=0 和 PartSetHeader.Hash=0
func (psh PartSetHeader) IsZero() bool {
	return psh.Total == 0 && len(psh.Hash) == 0
}

// Equals 判断两个 PartSetHeader 是否相等
func (psh PartSetHeader) Equals(other PartSetHeader) bool {
	return psh.Total == other.Total && bytes.Equal(psh.Hash, other.Hash)
}

// ValidateBasic performs basic validation.
func (psh PartSetHeader) ValidateBasic() error {
	// Hash can be empty in case of POLBlockID.PartSetHeader in PrePrepare.
	if err := srhash.ValidateHash(psh.Hash); err != nil {
		return fmt.Errorf("wrong Hash: %w", err)
	}
	return nil
}

// ToProto converts PartSetHeader to protobuf
func (psh *PartSetHeader) ToProto() prototypes.PartSetHeader {
	if psh == nil {
		return prototypes.PartSetHeader{}
	}

	return prototypes.PartSetHeader{
		Total: psh.Total,
		Hash:  psh.Hash,
	}
}

// PartSetHeaderFromProto sets a protobuf PartSetHeader to the given pointer
func PartSetHeaderFromProto(ppsh *prototypes.PartSetHeader) (*PartSetHeader, error) {
	if ppsh == nil {
		return nil, errors.New("nil PartSetHeader")
	}
	psh := new(PartSetHeader)
	psh.Total = ppsh.Total
	psh.Hash = ppsh.Hash

	return psh, psh.ValidateBasic()
}

//-------------------------------------

type PartSet struct {
	total uint32
	hash  []byte // 以 parts 为 merkle 的叶子，构成的 merkle 树的根哈希值

	mtx           sync.Mutex
	parts         []*Part
	partsBitArray *bits.BitArray
	count         uint32 // 每添加一个Part，count 自增 1，当添加完所有 Part 后，count 等于 total
	// 总大小的计数(以字节为单位)。用于确保部件集不超过最大块字节
	byteSize int64
}

// NewPartSetFromData 从数据字节中返回一个不可变的完整PartSet。
// 数据字节被分割成若干个大小为 partSize 大小的 Part，并计算默克尔树。
// CONTRACT: partSize 大于零。
// 这里的 data 是区块的内容：先将 Block 转换成 protobuf 形式，然后调用
// protobuf 的 Marshal 编码方式，将其转换成字节数组，就是这里的 data
func NewPartSetFromData(data []byte, partSize uint32) *PartSet {
	// 计算需要多少个 Part
	total := (uint32(len(data)) + partSize - 1) / partSize
	// 初始化 parts
	parts := make([]*Part, total)
	partsBytes := make([][]byte, total)
	partsBitArray := bits.NewBitArray(int(total))
	for i := uint32(0); i < total; i++ {
		part := &Part{
			Index: i,
			Bytes: data[i*partSize : srmath.MinInt(len(data), int((i+1)*partSize))],
		}
		parts[i] = part
		partsBytes[i] = part.Bytes
		partsBitArray.SetIndex(int(i), true)
	}
	// Compute merkle proofs
	root, proofs := merkle.ProofsFromByteSlices(partsBytes)
	for i := uint32(0); i < total; i++ {
		parts[i].Proof = *proofs[i]
	}
	return &PartSet{
		total:         total,
		hash:          root,
		parts:         parts,
		partsBitArray: partsBitArray,
		count:         total,
		byteSize:      int64(len(data)),
	}
}

// Returns an empty PartSet ready to be populated.
func NewPartSetFromHeader(header PartSetHeader) *PartSet {
	return &PartSet{
		total:         header.Total,
		hash:          header.Hash,
		parts:         make([]*Part, header.Total),
		partsBitArray: bits.NewBitArray(int(header.Total)),
		count:         0,
		byteSize:      0,
	}
}

func (ps *PartSet) Header() PartSetHeader {
	if ps == nil {
		return PartSetHeader{}
	}
	return PartSetHeader{
		Total: ps.total,
		Hash:  ps.hash,
	}
}

// 判断自己与给定的 PartSetHeader 是否一样
func (ps *PartSet) HeaderEqualsTo(header PartSetHeader) bool {
	if ps == nil {
		return false
	}
	return ps.Header().Equals(header)
}

func (ps *PartSet) BitArray() *bits.BitArray {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return ps.partsBitArray.Copy()
}

func (ps *PartSet) Hash() []byte {
	if ps == nil {
		return merkle.HashFromByteSlices(nil)
	}
	return ps.hash
}

func (ps *PartSet) HashesTo(hash []byte) bool {
	if ps == nil {
		return false
	}
	return bytes.Equal(ps.hash, hash)
}

func (ps *PartSet) Count() uint32 {
	if ps == nil {
		return 0
	}
	return ps.count
}

func (ps *PartSet) ByteSize() int64 {
	if ps == nil {
		return 0
	}
	return ps.byteSize
}

func (ps *PartSet) Total() uint32 {
	if ps == nil {
		return 0
	}
	return ps.total
}

func (ps *PartSet) AddPart(part *Part) (bool, error) {
	if ps == nil {
		return false, nil
	}
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	// 待添加的 Part 的索引值不应该大于或等于 Part 的总个数
	if part.Index >= ps.total {
		return false, ErrPartSetUnexpectedIndex
	}

	// 如果该待添加的 Part 已经存在于 PartSet里了，就返回 false
	if ps.parts[part.Index] != nil {
		return false, nil
	}

	// 验证 merkle
	if part.Proof.Verify(ps.Hash(), part.Bytes) != nil {
		return false, ErrPartSetInvalidProof
	}

	// 以上验证通过后，则添加该 Part
	ps.parts[part.Index] = part
	ps.partsBitArray.SetIndex(int(part.Index), true)
	ps.count++
	ps.byteSize += int64(len(part.Bytes))
	return true, nil
}

func (ps *PartSet) GetPart(index int) *Part {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return ps.parts[index]
}

// IsComplete 判断该 PartSet 是否收集齐所有 Part
func (ps *PartSet) IsComplete() bool {
	return ps.count == ps.total
}

func (ps *PartSet) GetReader() io.Reader {
	if !ps.IsComplete() {
		panic("Cannot GetReader() on incomplete PartSet")
	}
	return NewPartSetReader(ps.parts)
}

type PartSetReader struct {
	i      int
	parts  []*Part
	reader *bytes.Reader
}

func NewPartSetReader(parts []*Part) *PartSetReader {
	return &PartSetReader{
		i:      0,
		parts:  parts,
		reader: bytes.NewReader(parts[0].Bytes),
	}
}

// Read 把 PartSet 中的 parts 全部读取到 p 里面
func (psr *PartSetReader) Read(p []byte) (n int, err error) {
	// Len返回切片中未读部分的字节数。
	readerLen := psr.reader.Len()
	if readerLen >= len(p) {
		return psr.reader.Read(p)
	} else if readerLen > 0 { // 如果 readerLen 小于 p 的长度
		n1, err := psr.Read(p[:readerLen])
		if err != nil {
			return n1, err
		}
		n2, err := psr.Read(p[readerLen:])
		return n1 + n2, err
	}

	psr.i++
	if psr.i >= len(psr.parts) {
		return 0, io.EOF
	}
	psr.reader = bytes.NewReader(psr.parts[psr.i].Bytes)
	return psr.Read(p)
}

// StringShort returns a short version of String.
//
// (Count of Total)
func (ps *PartSet) StringShort() string {
	if ps == nil {
		return "nil-PartSet"
	}
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return fmt.Sprintf("(%v of %v)", ps.Count(), ps.Total())
}

func (ps *PartSet) MarshalJSON() ([]byte, error) {
	if ps == nil {
		return []byte("{}"), nil
	}

	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	return srjson.Marshal(struct {
		CountTotal    string         `json:"count/total"`
		PartsBitArray *bits.BitArray `json:"parts_bit_array"`
	}{
		fmt.Sprintf("%d/%d", ps.Count(), ps.Total()),
		ps.partsBitArray,
	})
}
