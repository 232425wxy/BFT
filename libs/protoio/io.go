package protoio

import (
	"github.com/gogo/protobuf/proto"
	"io"
)

type WriteCloser interface {
	WriteMsg(proto.Message) (int, error)
	io.Closer
}

type ReadCloser interface {
	ReadMsg(msg proto.Message) (int, error)
	io.Closer
}

type marshaler interface {
	MarshalTo(data []byte) (n int, err error)
}

func getSize(v interface{}) (int, bool) {
	if sz, ok := v.(interface {
		Size() (n int)
	}); ok {
		return sz.Size(), true
	} else if sz, ok := v.(interface {
		ProtoSize() (n int)
	}); ok {
		return sz.ProtoSize(), true
	} else {
		return 0, false
	}
}

// byteReader wraps an io.Reader and implements io.ByteReader, required by
// binary.ReadUvarint(). Reading one byte at a time is extremely slow, but this
// is what Amino did previously anyway, and the caller can wrap the underlying
// reader in a bufio.Reader if appropriate.
type byteReader struct {
	reader    io.Reader
	buf       []byte
	bytesRead int // keeps track of bytes read via ReadByte()
}

func newByteReader(r io.Reader) *byteReader {
	return &byteReader{
		reader: r,
		buf:    make([]byte, 1),
	}
}

func (r *byteReader) ReadByte() (byte, error) {
	n, err := r.reader.Read(r.buf)
	r.bytesRead += n
	if err != nil {
		return 0x00, err
	}
	return r.buf[0], nil
}
