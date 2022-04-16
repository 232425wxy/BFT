package merkle

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

type keyEncoding int

const (
	KeyEncodingURL keyEncoding = iota // 0
	KeyEncodingHex
	KeyEncodingMax // 已知编码的数量，用于测试
)

type Key struct {
	name []byte
	enc  keyEncoding
}

type KeyPath []Key

func (pth KeyPath) AppendKey(key []byte, enc keyEncoding) KeyPath {
	return append(pth, Key{key, enc})
}

func (pth KeyPath) String() string {
	res := ""
	for _, key := range pth {
		switch key.enc {
		case KeyEncodingURL:
			// PathEscape 转义字符串，这样它可以安全地放置在URL路径段中，根据需要用%XX序列替换特殊字符(包括/)。
			res += "/" + url.PathEscape(string(key.name))
		case KeyEncodingHex:
			res += "/x:" + fmt.Sprintf("%X", key.name)
		default:
			panic("unexpected key encoding type")
		}
	}
	return res
}

// KeyPathToKeys 把类似于 “KEY1/KEY2/KEY3 转换成：{KEY1, KEY2, KEY3}这样的字节切片
func KeyPathToKeys(path string) (keys [][]byte, err error) {
	if path == "" || path[0] != '/' {
		return nil, errors.New("key path string must start with a forward slash '/'")
	}
	parts := strings.Split(path[1:], "/")
	keys = make([][]byte, len(parts))
	for i, part := range parts {
		if strings.HasPrefix(part, "x:") { // 16 进制编码
			hexPart := part[2:]
			// DecodeString返回十六进制字符串s所表示的字节切片。
			// DecodeString期望src只包含十六进制字符，并且src具有偶数长度。如果输入是畸形的，DecodeString返回错误之前解码的字节。
			key, err := hex.DecodeString(hexPart)
			if err != nil {
				return nil, fmt.Errorf("decoding hex-encoded part #%d: /%s: %w", i, part, err)
			}
			keys[i] = key
		} else {
			key, err := url.PathUnescape(part)
			if err != nil {
				return nil, fmt.Errorf("decoding url-encoded part #%d: /%s: %w", i, part, err)
			}
			keys[i] = []byte(key) // TODO Test this with random bytes, I'm not sure that it works for arbitrary bytes...
		}
	}
	return keys, nil
}
