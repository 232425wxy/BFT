package crypto

import (
	"BFT/crypto/srhash"
	"BFT/libs/bytes"
)

const (
	// AddressSize 表示公钥地址的长度，20字节
	AddressSize = srhash.TruncatedSize
)

// Address
// address 是一个字节切片，只不过是 16 进制编码的
// []byte 给我们留下了选项来改变地址内容的长度
type Address = bytes.HexBytes

type PubKey interface {
	Address() Address // 对公钥进行哈希运算，然后取哈希值的前 20 字节作为公钥的地址
	Bytes() []byte
	VerifySignature(msg []byte, sig []byte) bool
	Equals(PubKey) bool
	Type() string
}

type PrivKey interface {
	Bytes() []byte
	Sign(msg []byte) ([]byte, error)
	PubKey() PubKey
	Type() string
}
