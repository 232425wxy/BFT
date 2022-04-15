package crypto

import (
	crand "crypto/rand"
	"encoding/hex"
	"io"
)

// CRandBytes 返回长度为 numBytes 的随机字节切片
func CRandBytes(numBytes int) []byte {
	b := make([]byte, numBytes)
	_, err := crand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

// CRandHex 随机返回 numDigits 个 16 进制数表示的字符串
// numDigits 个 16 进制数，只要 numDigits/2 个字节表示就可
func CRandHex(numDigits int) string {
	return hex.EncodeToString(CRandBytes(numDigits / 2))
}

// Returns a crand.Reader.
func CReader() io.Reader {
	return crand.Reader
}
