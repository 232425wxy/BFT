package bytes

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// HexBytes 数组里存储的是大写的16进制
type HexBytes []byte

// MarshalJSON 是 Bytes 关键
func (bz HexBytes) MarshalJSON() ([]byte, error) {
	s := strings.ToUpper(hex.EncodeToString(bz))
	jbz := make([]byte, len(s)+2)
	jbz[0] = '"'
	copy(jbz[1:], s)
	jbz[len(jbz)-1] = '"'
	return jbz, nil
}

// UnmarshalJSON 是 Bytes 的关键
func (bz *HexBytes) UnmarshalJSON(data []byte) error {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return fmt.Errorf("invalid hex string: %s", data)
	}
	bz2, err := hex.DecodeString(string(data[1 : len(data)-1]))
	if err != nil {
		return err
	}
	*bz = bz2
	return nil
}

// Bytes 满足 light-client 的各种接口，等等……
func (bz HexBytes) Bytes() []byte {
	return bz
}

func (bz HexBytes) String() string {
	return strings.ToUpper(hex.EncodeToString(bz))
}

func FromString(str string) (HexBytes, error) {
	bz, err := hex.DecodeString(str)
	if err != nil {
		return nil, err
	}
	return HexBytes(bz), nil
}

// Fingerprint 返回字节片的前6个字节。
// 如果切片小于6字节，则指纹尾部用0填充。
func Fingerprint(slice []byte) []byte {
	fingerprint := make([]byte, 6)
	copy(fingerprint, slice)
	return fingerprint
}
