package srhash

import (
	"crypto/sha256"
	"fmt"
	"hash"
)

const (
	Size = sha256.Size
)

// New 返回一个 hash.Hash.
func New() hash.Hash {
	return sha256.New()
}

// Sum 返回 bz 的哈希值
func Sum(bz []byte) []byte {
	h := sha256.Sum256(bz)
	return h[:]
}

func Sha256(bytes []byte) []byte {
	hasher := sha256.New()
	hasher.Write(bytes)
	return hasher.Sum(nil)
}

//-------------------------------------------------------------

const (
	TruncatedSize = 20
)

// SumTruncated 先计算 bz 的 SHA256 哈希值，然后取哈希值的前 20 字节长度
func SumTruncated(bz []byte) []byte {
	hash := sha256.Sum256(bz)
	return hash[:TruncatedSize]
}

// ValidateHash 如果哈希值不为空，但是哈希值的长度不等于 32，则返回一个错误
func ValidateHash(h []byte) error {
	if len(h) > 0 && len(h) != Size {
		return fmt.Errorf("expected size to be %d bytes, got %d bytes", Size, len(h))
	}
	return nil
}
