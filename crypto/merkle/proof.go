package merkle

import (
	"BFT/crypto/srhash"
	pbcrypto "BFT/proto/crypto"
	"bytes"
	"errors"
	"fmt"
)

const (
	// MaxAunts is the maximum number of aunts that can be included in a Proof.
	// This corresponds to a tree of size 2^100, which should be sufficient for all conceivable purposes.
	// This maximum helps prevent Denial-of-Service attacks by limitting the size of the proofs.
	MaxAunts = 100
)

// Proof 代表 merkle 的 proof，只包含叶子哈希，不包含根哈希
type Proof struct {
	Total    int64    `json:"total"`     // Total number of items.
	Index    int64    `json:"index"`     // Index item 的索引值，从 0 开始计数
	LeafHash []byte   `json:"leaf_hash"` // Hash item 的哈希值，即叶子哈希
	Aunts    [][]byte `json:"aunts"`     // Hashes 叶子节点在 merkle 中 兄弟和 aunt 的集合
}

// ProofsFromByteSlices 根据给定的 items，生成一系列的 Proof 和 根哈希
// proofs[0] is the proof for items[0].
func ProofsFromByteSlices(items [][]byte) (rootHash []byte, proofs []*Proof) {
	trails, rootSPN := trailsFromByteSlices(items)
	rootHash = rootSPN.Hash
	proofs = make([]*Proof, len(items))
	for i, trail := range trails {
		proofs[i] = &Proof{
			Total:    int64(len(items)),
			Index:    int64(i),
			LeafHash: trail.Hash,
			Aunts:    trail.FlattenAunts(),
		}
	}
	return
}

// Verify that the Proof proves the root hash.
// Check sp.Index/sp.Total manually if needed
func (sp *Proof) Verify(rootHash []byte, leaf []byte) error {
	leafHash := leafHash(leaf)
	if sp.Total < 0 {
		return errors.New("proof total must be positive")
	}
	if sp.Index < 0 {
		return errors.New("proof index cannot be negative")
	}
	if !bytes.Equal(sp.LeafHash, leafHash) {
		return fmt.Errorf("invalid leaf hash: wanted %X got %X", leafHash, sp.LeafHash)
	}
	computedHash := sp.ComputeRootHash()
	if !bytes.Equal(computedHash, rootHash) {
		return fmt.Errorf("invalid root hash: wanted %X got %X", rootHash, computedHash)
	}
	return nil
}

// ComputeRootHash 根据一个 Proof 计算根哈希值
func (sp *Proof) ComputeRootHash() []byte {
	return computeHashFromAunts(
		sp.Index,
		sp.Total,
		sp.LeafHash,
		sp.Aunts,
	)
}

// String implements the stringer interface for Proof.
// It is a wrapper around StringIndented.
func (sp *Proof) String() string {
	return sp.StringIndented("")
}

// StringIndented generates a canonical string representation of a Proof.
func (sp *Proof) StringIndented(indent string) string {
	return fmt.Sprintf(`Proof{
%s  Aunts: %X
%s}`,
		indent, sp.Aunts,
		indent)
}

// ValidateBasic performs basic validation.
// NOTE: it expects the LeafHash and the elements of Aunts to be of size srhash.Size,
// and it expects at most MaxAunts elements in Aunts.
func (sp *Proof) ValidateBasic() error {
	if sp.Total < 0 {
		return errors.New("negative Total")
	}
	if sp.Index < 0 {
		return errors.New("negative Index")
	}
	if len(sp.LeafHash) != srhash.Size {
		return fmt.Errorf("expected LeafHash size to be %d, got %d", srhash.Size, len(sp.LeafHash))
	}
	if len(sp.Aunts) > MaxAunts {
		return fmt.Errorf("expected no more than %d aunts, got %d", MaxAunts, len(sp.Aunts))
	}
	for i, auntHash := range sp.Aunts {
		if len(auntHash) != srhash.Size {
			return fmt.Errorf("expected Aunts#%d size to be %d, got %d", i, srhash.Size, len(auntHash))
		}
	}
	return nil
}

func (sp *Proof) ToProto() *pbcrypto.Proof {
	if sp == nil {
		return nil
	}
	pb := new(pbcrypto.Proof)

	pb.Total = sp.Total
	pb.Index = sp.Index
	pb.LeafHash = sp.LeafHash
	pb.Aunts = sp.Aunts

	return pb
}

func ProofFromProto(pb *pbcrypto.Proof) (*Proof, error) {
	if pb == nil {
		return nil, errors.New("nil proof")
	}

	sp := new(Proof)

	sp.Total = pb.Total
	sp.Index = pb.Index
	sp.LeafHash = pb.LeafHash
	sp.Aunts = pb.Aunts

	return sp, sp.ValidateBasic()
}

// 用叶子节点的哈希和它的 aunt 节点的哈希值，可以计算得到根哈希值
// 如果 aunt 的数量不对，则返回 nil
// 递归实现
func computeHashFromAunts(index, total int64, leafHash []byte, innerHashes [][]byte) []byte {
	if index >= total || index < 0 || total <= 0 {
		return nil
	}
	switch total {
	case 0:
		panic("Cannot call computeHashFromAunts() with 0 total")
	case 1:
		if len(innerHashes) != 0 {
			return nil
		}
		return leafHash
	default:
		if len(innerHashes) == 0 {
			return nil
		}
		numLeft := getSplitPoint(total) // 返回小于 total 的最大的 2 的幂次方
		if index < numLeft {
			leftHash := computeHashFromAunts(index, numLeft, leafHash, innerHashes[:len(innerHashes)-1])
			if leftHash == nil {
				return nil
			}
			return innerHash(leftHash, innerHashes[len(innerHashes)-1])
		}
		rightHash := computeHashFromAunts(index-numLeft, total-numLeft, leafHash, innerHashes[:len(innerHashes)-1])
		if rightHash == nil {
			return nil
		}
		return innerHash(innerHashes[len(innerHashes)-1], rightHash)
	}
}

// ProofNode 是一个用于构造默克尔证明的助手结构。
// 只有当一个 node 是 root 时，它的左兄弟或者右兄弟都是 nil
// node.Parent.Hash = hash(node.Hash, node.Right.Hash) or hash(node.Left.Hash, node.Hash)
type ProofNode struct {
	Hash   []byte
	Parent *ProofNode
	Left   *ProofNode // Left 兄弟  (only one of Left,Right is set)
	Right  *ProofNode // Right 兄弟 (only one of Left,Right is set)
}

// FlattenAunts aunt 是什么样的节点呢，当一个节点只要知道所有 aunt
// 节点的哈希值后，就可以求出根哈希，一个节点拥有的 aunt 的数量不超过 merkle
// 的深度
func (spn *ProofNode) FlattenAunts() [][]byte {
	// Nonrecursive impl.
	innerHashes := [][]byte{}
	for spn != nil {
		switch {
		case spn.Left != nil:
			innerHashes = append(innerHashes, spn.Left.Hash)
		case spn.Right != nil:
			innerHashes = append(innerHashes, spn.Right.Hash)
		default:
			break
		}
		spn = spn.Parent
	}
	return innerHashes
}

// trails[0].Hash is the leaf hash for items[0].
// trails[i].Parent.Parent....Parent == root for all i.
func trailsFromByteSlices(items [][]byte) (trails []*ProofNode, root *ProofNode) {
	// 递归实现
	switch len(items) {
	case 0: // 如果 items 里面没有 item
		return []*ProofNode{}, &ProofNode{emptyHash(), nil, nil, nil}
	case 1: // 如果 items 里面只有一个 item，计算该 item 的叶哈希
		trail := &ProofNode{leafHash(items[0]), nil, nil, nil}
		return []*ProofNode{trail}, trail
	default: // 如果 items 里面有不止一个 item
		k := getSplitPoint(int64(len(items)))                // 返回小于 len(items) 的最大的 2 的幂次方
		lefts, leftRoot := trailsFromByteSlices(items[:k])   // 计算前 k 个 item
		rights, rightRoot := trailsFromByteSlices(items[k:]) // 计算后 len(items)-k 个 item
		rootHash := innerHash(leftRoot.Hash, rightRoot.Hash)
		root := &ProofNode{rootHash, nil, nil, nil}
		leftRoot.Parent = root
		leftRoot.Right = rightRoot
		rightRoot.Parent = root
		rightRoot.Left = leftRoot
		return append(lefts, rights...), root
	}
}
