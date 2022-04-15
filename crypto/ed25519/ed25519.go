package ed25519

import (
	"BFT/crypto"
	"BFT/crypto/srhash"
	srjson "BFT/libs/json"
	"bytes"
	"crypto/ed25519"
	"fmt"
	"io"
)

//-------------------------------------

var _ crypto.PrivKey = PrivKey{}

const (
	PrivKeyName = "BFT/PrivKeyEd25519" // 私钥的名字
	PubKeyName  = "BFT/PubKeyEd25519"  // 公钥的名字
	// PubKeySize 公钥的长度，32字节
	PubKeySize = 32
	// SignatureSize 签名的长度，64字节
	SignatureSize = 64
	// SeedSize 私钥种子的长度，32字节
	SeedSize = 32
	// KeyType 秘钥类型
	KeyType = "ed25519"
)

func init() {
	srjson.RegisterType(PubKey{}, PubKeyName)
	srjson.RegisterType(PrivKey{}, PrivKeyName)
}

// PrivKey implements crypto.PrivKey.
type PrivKey []byte

// Bytes 返回私钥的字节形式
func (privKey PrivKey) Bytes() []byte {
	return privKey
}

// Sign 在提供的消息上生成一个签名。
// 这假设 privkey 是正确的 golang 格式。
// 前 32 个字节应该是随机的，对应于普通的ed25519私钥。
// 后 32 个字节应该是压缩的公钥。
// 如果这些条件不满足，Sign 将恐慌或产生错误的签名。
func (privKey PrivKey) Sign(msg []byte) ([]byte, error) {
	signatureBytes := ed25519.Sign(ed25519.PrivateKey(privKey), msg)
	return signatureBytes, nil
}

// PubKey 从私钥中获取相应的公钥。
// 如果私钥没有初始化，则会出现恐慌。
func (privKey PrivKey) PubKey() crypto.PubKey {
	// 如果私钥的后 32 位都是 0，则表示私钥还没有被初始化
	initialized := false
	for _, v := range privKey[32:] {
		if v != 0 {
			initialized = true
			break
		}
	}

	if !initialized {
		panic("The PrivateKey hasn't been initialized!")
	}

	pubkeyBytes := make([]byte, PubKeySize)
	copy(pubkeyBytes, privKey[32:])
	return PubKey(pubkeyBytes)
}

func (privKey PrivKey) Type() string {
	return KeyType
}

// GenPrivKey 生成新的ed25519私钥。
// 它将 OS 随机性与当前全局随机种子结合使用
func GenPrivKey() PrivKey {
	return genPrivKey(crypto.CReader())
}

// genPrivKey 使用提供的读取器生成一个新的ed25519私钥。
func genPrivKey(rand io.Reader) PrivKey {
	seed := make([]byte, SeedSize)

	_, err := io.ReadFull(rand, seed) // 从 Reader 里读取内容到 buf 里
	if err != nil {
		panic(err)
	}

	return PrivKey(ed25519.NewKeyFromSeed(seed))
}

// GenPrivKeyFromSecret 使用 SHA256 散列该 secret，并使用该32字节输出来创建私钥。
func GenPrivKeyFromSecret(secret []byte) PrivKey {
	seed := srhash.Sha256(secret) // 计算秘密值的哈希值，其实目的就是为了获得长度为 32 的字节切片

	return PrivKey(ed25519.NewKeyFromSeed(seed))
}

//-------------------------------------

var _ crypto.PubKey = PubKey{}

// PubKeyEd25519 implements crypto.PubKey for the Ed25519 signature scheme.
type PubKey []byte

// Address 公钥 SHA256 散列值的前 20 字节，
// 然后返回的是 40 个 16 进制数字组成的切片
func (pubKey PubKey) Address() crypto.Address {
	if len(pubKey) != PubKeySize {
		panic("pubkey is incorrect size")
	}
	return crypto.Address(srhash.SumTruncated(pubKey))
}

// Bytes 返回公钥的字节切片形式
func (pubKey PubKey) Bytes() []byte {
	return []byte(pubKey)
}

// VerifySignature 验证签名的正确性
func (pubKey PubKey) VerifySignature(msg []byte, sig []byte) bool {
	// make sure we use the same algorithm to sign
	if len(sig) != SignatureSize {
		return false
	}

	return ed25519.Verify(ed25519.PublicKey(pubKey), msg, sig)
}

// 返回公钥的字符串形式
func (pubKey PubKey) String() string {
	return fmt.Sprintf("PubKeyEd25519{%X}", []byte(pubKey))
}

func (pubKey PubKey) Type() string {
	return KeyType
}

func (pubKey PubKey) Equals(other crypto.PubKey) bool {
	if otherEd, ok := other.(PubKey); ok {
		return bytes.Equal(pubKey[:], otherEd[:])
	}

	return false
}
