package protoio

import (
	"bytes"
	"encoding/binary"
	"github.com/gogo/protobuf/proto"
	"io"
)

// NewDelimitedWriter writes a varint-delimited Protobuf message to a writer. It is
// equivalent to the gogoproto NewDelimitedWriter, except WriteMsg() also returns the
// number of bytes written, which is necessary in the p2p package.
func NewDelimitedWriter(w io.Writer) WriteCloser {
	return &varintWriter{w, make([]byte, binary.MaxVarintLen64), nil}
}

type varintWriter struct {
	w      io.Writer
	lenBuf []byte
	buffer []byte
}

// WriteMsg 将数据写入到 io.Writer 中，写入之前，会计算数据本身的大小：size
// 然后将数据大小和数据本身进行打包：<size:data>，再将其写入到 io.Writer 中
func (w *varintWriter) WriteMsg(msg proto.Message) (int, error) {
	// 保证 msg 实现了 marshaler 接口，这是实现 protobuf 编码的基础
	if m, ok := msg.(marshaler); ok {
		// n 是需要传递的数据本身的大小
		n, ok := getSize(m)
		if ok {
			// binary.MaxVarintLen64 是存储一个数字所需要的空间大小，该数字表示的是所需要传递数据的大小
			if n+binary.MaxVarintLen64 >= len(w.buffer) {
				w.buffer = make([]byte, n+binary.MaxVarintLen64)
			}
			// PutUvarint 将 uint64(n) 写入到 w.buffer 中，并返回写入的字节数，如果 w.buffer 太小会 panic
			lenOff := binary.PutUvarint(w.buffer, uint64(n))
			_, err := m.MarshalTo(w.buffer[lenOff:])
			if err != nil {
				return 0, err
			}
			_, err = w.w.Write(w.buffer[:lenOff+n])
			return lenOff + n, err
		}
	}
	// 如果不能采用 protobuf 的编码方式，就采用以下方法
	data, err := proto.Marshal(msg)
	if err != nil {
		return 0, err
	}
	length := uint64(len(data))
	n := binary.PutUvarint(w.lenBuf, length)
	// 先发送数据的长度，再发送数据本身
	_, err = w.w.Write(w.lenBuf[:n])
	if err != nil {
		return 0, err
	}
	_, err = w.w.Write(data)
	return len(data) + n, err
}

func (w *varintWriter) Close() error {
	if closer, ok := w.w.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// MarshalDelimited 调用 varintWriter.WriteMsg 将 msg 写入到 bytes.Buffer 中，
// 返回的内容格式如下：<msg长度,msg>
func MarshalDelimited(msg proto.Message) ([]byte, error) {
	var buf bytes.Buffer
	_, err := NewDelimitedWriter(&buf).WriteMsg(msg)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
