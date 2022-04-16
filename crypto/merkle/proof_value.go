package merkle

import (
	"github.com/232425wxy/BFT/crypto/srhash"
	pbcrypto "github.com/232425wxy/BFT/proto/crypto"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

const ProofOpValue = "simple:v"

// ValueOp 接受一个键和一个值作为参数，并产生根哈希。
// 相应的树结构是SimpleMap树。SimpleMap采用hash，
// 而目前 SRBFT 使用 srhash。SimpleValueOp应该支持在trhash中使用的哈希函数
//
// 如果生成的根哈希与预期的哈希匹配，则证明是正确的。
type ValueOp struct {
	// Encoded in ProofOp.Key.
	key []byte

	// To encode in ProofOp.Data
	Proof *Proof `json:"proof"`
}

var _ ProofOperator = ValueOp{}

func NewValueOp(key []byte, proof *Proof) ValueOp {
	return ValueOp{
		key:   key,
		Proof: proof,
	}
}

func ValueOpDecoder(pop pbcrypto.ProofOp) (ProofOperator, error) {
	if pop.Type != ProofOpValue { // 判断 Type 是否等于 “simple:v”
		return nil, fmt.Errorf("unexpected ProofOp.Type; got %v, want %v", pop.Type, ProofOpValue)
	}
	var pbop pbcrypto.ValueOp // 作者说这里有点奇怪
	err := pbop.Unmarshal(pop.Data)
	if err != nil {
		return nil, fmt.Errorf("decoding ProofOp.Data into ValueOp: %w", err)
	}

	sp, err := ProofFromProto(pbop.Proof) // 将 protobuf 的 ProofOp.Data 转换成 protobuf 的 ValueOp，然后将 protobuf 的 ValueOp.Proof 转换成自定义的 Proof
	if err != nil {
		return nil, err
	}
	return NewValueOp(pop.Key, sp), nil
}

func (op ValueOp) ProofOp() pbcrypto.ProofOp {
	pbval := pbcrypto.ValueOp{
		Key:   op.key,
		Proof: op.Proof.ToProto(),
	}
	bz, err := pbval.Marshal()
	if err != nil {
		panic(err)
	}
	return pbcrypto.ProofOp{
		Type: ProofOpValue,
		Key:  op.key,
		Data: bz, // 由 protobuf 的 ValueOp Marshal 过来的
	}
}

func (op ValueOp) String() string {
	return fmt.Sprintf("ValueOp{%v}", op.GetKey())
}

// Run 就是计算 Args 第一个参数的哈希值，然后和 op.Proof.LeafHash 比较一不一样
func (op ValueOp) Run(args [][]byte) ([][]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("expected 1 arg, got %v", len(args))
	}
	value := args[0]
	hasher := srhash.New()
	hasher.Write(value)
	vhash := hasher.Sum(nil) // 直接计算 Args 第一个元素的哈希值

	bz := new(bytes.Buffer)
	// Wrap <op.Key, vhash> to hash the KVPair.
	_ = encodeByteSlice(bz, op.key) // nolint: errcheck // does not error
	_ = encodeByteSlice(bz, vhash)  // nolint: errcheck // does not error
	kvhash := leafHash(bz.Bytes())  // bz: [len(op.key)|op.key|len(vhash)|vhash]

	if !bytes.Equal(kvhash, op.Proof.LeafHash) {
		return nil, fmt.Errorf("leaf hash mismatch: want %X got %X", op.Proof.LeafHash, kvhash)
	}

	return [][]byte{
		op.Proof.ComputeRootHash(),
	}, nil
}

func (op ValueOp) GetKey() []byte {
	return op.key
}

// Uvarint length prefixed byteslice
func encodeByteSlice(w io.Writer, bz []byte) (err error) {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], uint64(len(bz))) // 先把 bz 的长度写到 buf 里
	_, err = w.Write(buf[0:n])
	if err != nil {
		return
	}
	_, err = w.Write(bz) // 再把 bz 写进去
	return
}
