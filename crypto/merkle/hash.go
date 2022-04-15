package merkle

import "BFT/crypto/srhash"

var (
	leafPrefix  = []byte{0}
	innerPrefix = []byte{1}
)

// returns hash(<empty>)
func emptyHash() []byte {
	return srhash.Sum([]byte{})
}

// returns hash(0x00 || leaf)
func leafHash(leaf []byte) []byte {
	return srhash.Sum(append(leafPrefix, leaf...))
}

// returns hash(0x01 || left || right)
func innerHash(left []byte, right []byte) []byte {
	return srhash.Sum(append(innerPrefix, append(left, right...)...))
}
