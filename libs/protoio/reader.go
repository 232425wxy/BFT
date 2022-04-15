package protoio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/gogo/protobuf/proto"
)

// NewDelimitedReader reads varint-delimited Protobuf messages from a reader.
// Unlike the gogoproto NewDelimitedReader, this does not buffer the reader,
// which may cause poor performance but is necessary when only reading single
// messages (e.g. in the p2p package). It also returns the number of bytes
// read, which is necessary for the p2p package.
func NewDelimitedReader(r io.Reader, maxSize int) ReadCloser {
	var closer io.Closer
	if c, ok := r.(io.Closer); ok {
		closer = c
	}
	return &varintReader{r, nil, maxSize, closer}
}

type varintReader struct {
	r       io.Reader
	buf     []byte
	maxSize int
	closer  io.Closer
}

func (r *varintReader) ReadMsg(msg proto.Message) (int, error) {
	br := newByteReader(r.r)
	// 读取 r.r 底层缓冲区第一个无符号整数，并以 uint64 类型返回
	l, err := binary.ReadUvarint(br)
	// 如果读取的第一个无符号整数是 uint32 类型的，则此处的 n 应该等于 4
	n := br.bytesRead
	if err != nil {
		return n, err
	}
	length := int(l)
	if l >= uint64(^uint(0)>>1) || length < 0 || n+length < 0 {
		return n, fmt.Errorf("invalid out of message length %v", l)
	}
	if length > r.maxSize {
		return n, fmt.Errorf("message exceeds max size (%v > %v)", length, r.maxSize)
	}
	if len(r.buf) < length {
		r.buf = make([]byte, length)
	}
	buf := r.buf[:length]
	nr, err := io.ReadFull(r.r, buf)
	n += nr
	if err != nil {
		return n, err
	}
	return n, proto.Unmarshal(buf, msg)
}

func (r *varintReader) Close() error {
	if r.closer != nil {
		return r.closer.Close()
	}
	return nil
}

// UnmarshalDelimited data 数据的格式应该是这样的：<length:content>
func UnmarshalDelimited(data []byte, msg proto.Message) error {
	_, err := NewDelimitedReader(bytes.NewReader(data), len(data)).ReadMsg(msg)
	return err
}
