package types

import (
	"github.com/232425wxy/BFT/crypto/merkle"
	"github.com/232425wxy/BFT/crypto/srhash"
	srbytes "github.com/232425wxy/BFT/libs/bytes"
	prototypes "github.com/232425wxy/BFT/proto/types"
	"bytes"
	"errors"
	"fmt"
)

// Tx 是一个任意字节数组
// NOTE: Tx has no types at this level, so when wire encoded it's just length-prefixed.
type Tx []byte

// Hash 计算 transaction 的哈希值
func (tx Tx) Hash() []byte {
	return srhash.Sum(tx)
}

// String 返回 transaction 16 进制编码的字符串
func (tx Tx) String() string {
	return fmt.Sprintf("Tx{%X}", []byte(tx))
}

// Txs 是一个 transaction 列表
type Txs []Tx

// Hash 将一组 transaction 的哈希值作为 merkle 的叶子，然后计算根哈希值，
// 作为这一组 transaction 的哈希值
func (txs Txs) Hash() []byte {
	txBzs := make([][]byte, len(txs))
	for i := 0; i < len(txs); i++ {
		txBzs[i] = txs[i].Hash()
	}
	return merkle.HashFromByteSlices(txBzs)
}

// Index 给定一个 transaction，返回该 transaction 在 交易列表里的索引值
func (txs Txs) Index(tx Tx) int {
	for i := range txs {
		if bytes.Equal(txs[i], tx) {
			return i
		}
	}
	return -1
}

// IndexByHash 给定一个 transaction 哈希值，通过该哈希值，
// 与这组交易的每个 transaction 的哈希值进行比较，然后返回
// 该 transaction 在 交易列表里的索引值
func (txs Txs) IndexByHash(hash []byte) int {
	for i := range txs {
		if bytes.Equal(txs[i].Hash(), hash) {
			return i
		}
	}
	return -1
}

// Proof 根据给定的一组 transaction 和这组 transaction 中某个 transaction' 的索引值 i，
// 以这组 transaction 中每个 transaction 的哈希值为 merkle 的叶子节点，然后计算 merkle
// 的根哈希值和所有叶子节点的 proofs，然后获取 transaction' 对应位置的 proof = proofs[i]，
// 最后将 merkle 的根哈希值、transaction' 和 proofs[i] 组装成 TxProof，并将其返回
func (txs Txs) Proof(i int) TxProof {
	l := len(txs)
	if i < 0 || i >= l {
		panic("Index out of Txs")
	}
	bzs := make([][]byte, l)
	for i := 0; i < l; i++ {
		bzs[i] = txs[i].Hash()
	}
	root, proofs := merkle.ProofsFromByteSlices(bzs)

	return TxProof{
		RootHash: root,
		Data:     txs[i],
		Proof:    *proofs[i],
	}
}

// TxProof 表示 merkle 中存在 transaction 的 merkle proof。
type TxProof struct {
	RootHash srbytes.HexBytes `json:"root_hash"`
	Data     Tx               `json:"data"`
	Proof    merkle.Proof     `json:"proof"`
}

// Leaf 返回 transaction 在 merkle 树中作为叶子节点的值，其实就是该 transaction 的哈希值
func (tp TxProof) Leaf() []byte {
	return tp.Data.Hash()
}

// Validate 验证 TxProof 的正确性，入参是 merkle 的根哈希
func (tp TxProof) Validate(dataHash []byte) error {
	if !bytes.Equal(dataHash, tp.RootHash) {
		return errors.New("proof matches different data hash")
	}
	if tp.Proof.Index < 0 {
		return errors.New("proof index cannot be negative")
	}
	if tp.Proof.Total <= 0 {
		return errors.New("proof total must be positive")
	}
	valid := tp.Proof.Verify(tp.RootHash, tp.Leaf())
	if valid != nil {
		return errors.New("proof is not internally consistent")
	}
	return nil
}

func (tp TxProof) ToProto() prototypes.TxProof {

	pbProof := tp.Proof.ToProto()

	pbtp := prototypes.TxProof{
		RootHash: tp.RootHash,
		Data:     tp.Data,
		Proof:    pbProof,
	}

	return pbtp
}

// ComputeProtoSizeForTxs 将多个 tx 包装成 prototypes.Data{} 数据结构，
// 然后利用 protobuf 的 Size() 方法计算其大小
func ComputeProtoSizeForTxs(txs []Tx) int64 {
	data := Data{Txs: txs}
	pdData := data.ToProto()
	return int64(pdData.Size())
}
