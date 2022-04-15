package merkle

import (
	pbcrypto "BFT/proto/crypto"
	"bytes"
	"errors"
	"fmt"
)

//----------------------------------------
// ProofOp gets converted to an instance of ProofOperator:

// ProofOperator is a layer for calculating intermediate Merkle roots
// when a series of Merkle trees are chained together.
// Run() takes leaf values from a tree and returns the Merkle
// root for the corresponding tree. It takes and returns a list of bytes
// to allow multiple leaves to be part of a single proof, for instance in a range proof.
// ProofOp() encodes the ProofOperator in a generic way so it can later be
// decoded with OpDecoder.
type ProofOperator interface {
	Run([][]byte) ([][]byte, error)
	GetKey() []byte
	ProofOp() pbcrypto.ProofOp
}

//----------------------------------------
// Operations on a list of ProofOperators

// ProofOperators is a slice of ProofOperator(s).
// Each operator will be applied to the input value sequentially
// and the last Merkle root will be verified with already known data
type ProofOperators []ProofOperator

func (poz ProofOperators) VerifyValue(root []byte, keypath string, value []byte) (err error) {
	return poz.Verify(root, keypath, [][]byte{value})
}

func (poz ProofOperators) Verify(root []byte, keypath string, args [][]byte) (err error) {
	keys, err := KeyPathToKeys(keypath)
	if err != nil {
		return
	}

	for i, op := range poz {
		key := op.GetKey()
		if len(key) != 0 { // 如果 op 的 key 不为空
			if len(keys) == 0 { // 但是 keys 却是空的，那么就有问题
				return fmt.Errorf("key path has insufficient # of parts: expected no more keys but got %+v", string(key))
			}
			lastKey := keys[len(keys)-1]    // 获取 keys 的最后一个 key
			if !bytes.Equal(lastKey, key) { // 如果 keys 里最后一个 key 不等于 op 的 key，则返回错误
				return fmt.Errorf("key mismatch on operation #%d: expected %+v but got %+v", i, string(lastKey), string(key))
			}
			keys = keys[:len(keys)-1] // 将 keys 的最后一个 key 排除出去
		}
		args, err = op.Run(args) // keys 的最后一个 key 和 op 的 key 匹配成功后，op 执行 Run()，并更新 args
		if err != nil {
			return
		}
	}
	if !bytes.Equal(root, args[0]) { // 最后 args 的第一个元素在计算正常的情况下，应该是根哈希的值
		return fmt.Errorf("calculated root hash is invalid: expected %X but got %X", root, args[0])
	}
	if len(keys) != 0 { // 如果 keys 里 key 的数量和 op 的数量不相等，也会出错
		return errors.New("keypath not consumed all")
	}
	return nil
}

//----------------------------------------
// ProofRuntime - commands entrypoint

type OpDecoder func(pbcrypto.ProofOp) (ProofOperator, error)

type ProofRuntime struct {
	decoders map[string]OpDecoder
}

func NewProofRuntime() *ProofRuntime {
	return &ProofRuntime{
		decoders: make(map[string]OpDecoder),
	}
}

func (prt *ProofRuntime) RegisterOpDecoder(typ string, dec OpDecoder) {
	_, ok := prt.decoders[typ]
	if ok {
		panic("already registered for type " + typ)
	}
	prt.decoders[typ] = dec
}

func (prt *ProofRuntime) Decode(pop pbcrypto.ProofOp) (ProofOperator, error) {
	decoder := prt.decoders[pop.Type] // 每个 Type 的 op 对应一个 decoder
	if decoder == nil {
		return nil, fmt.Errorf("unrecognized proof type %v", pop.Type)
	}
	return decoder(pop)
}

func (prt *ProofRuntime) DecodeProof(proof *pbcrypto.ProofOps) (ProofOperators, error) {
	poz := make(ProofOperators, 0, len(proof.Ops))
	for _, pop := range proof.Ops {
		operator, err := prt.Decode(pop) // 把 protobuf 里的 ProofOp 解码成我们自定义的 ProofOperator
		if err != nil {
			return nil, fmt.Errorf("decoding a proof operator: %w", err)
		}
		poz = append(poz, operator)
	}
	return poz, nil
}

func (prt *ProofRuntime) VerifyValue(proof *pbcrypto.ProofOps, root []byte, keypath string, value []byte) (err error) {
	return prt.Verify(proof, root, keypath, [][]byte{value})
}

// TODO In the long run we'll need a method of classifcation of ops,
// whether existence or absence or perhaps a third?
func (prt *ProofRuntime) VerifyAbsence(proof *pbcrypto.ProofOps, root []byte, keypath string) (err error) {
	return prt.Verify(proof, root, keypath, nil)
}

func (prt *ProofRuntime) Verify(proof *pbcrypto.ProofOps, root []byte, keypath string, args [][]byte) (err error) {
	poz, err := prt.DecodeProof(proof)
	if err != nil {
		return fmt.Errorf("decoding proof: %w", err)
	}
	return poz.Verify(root, keypath, args)
}

// DefaultProofRuntime only knows about value proofs.
// To use e.g. IAVL proofs, register op-decoders as
// defined in the IAVL package.
func DefaultProofRuntime() (prt *ProofRuntime) {
	prt = NewProofRuntime()
	prt.RegisterOpDecoder(ProofOpValue, ValueOpDecoder)
	return
}
