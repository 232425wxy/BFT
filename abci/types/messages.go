package types

import (
	protoabci "BFT/proto/abci"
	"bufio"
	"encoding/binary"
	"github.com/gogo/protobuf/proto"
	"io"
)

const (
	maxMsgSize = 104857600 // 100MB
)

// WriteMessage 将 proto.Message 写入到 io.Writer 中
// 总共有以下几个步骤：
//	1. 将 protobuf 类型消息编码成字节切片
//	2. 计算第一步中获得的字节切片的长度，将其写入到 io.Writer 中
//	3. 将字节切片的内容写入到 io.Writer 中
func WriteMessage(msg proto.Message, w io.Writer) error {
	bz, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	var buf [10]byte
	n := binary.PutVarint(buf[:], int64(len(bz)))
	_, err = w.Write(buf[:n])
	if err != nil {
		return err
	}
	_, err = w.Write(bz)
	return err
}

// ReadMessage 从 io.Reader 中读取数据到 proto.Message 中
//	1. 先从 io.Reader 中读取 proto.Message 消息的长度
//	2. 根据第一步读取出的消息长度，创建一个字节切片，将 io.Reader 里剩余的消息读入到创建的字节切片里
func ReadMessage(r io.Reader, msg proto.Message) error {
	reader, ok := r.(*bufio.Reader)
	if !ok {
		reader = bufio.NewReader(r)
	}
	length64, err := binary.ReadVarint(reader)
	if err != nil {
		return err
	}
	length := int(length64)
	if length < 0 || length > maxMsgSize {
		return io.ErrShortBuffer
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return err
	}
	return proto.Unmarshal(buf, msg)
}

// ToRequestEcho 将 string 类型的 message 消息，包装成：
// &protoabci.Request{Value:&protoabci.Request_Echo{Echo:&protoabci.RequestEcho{Message:message}}}
func ToRequestEcho(message string) *protoabci.Request {
	return &protoabci.Request{
		Value: &protoabci.Request_Echo{&protoabci.RequestEcho{Message: message}},
	}
}

// ToRequestFlush 将一个空的 protoabci.RequestFlush{}，包装成：
// &protoabci.Request{Value: &protoabci.Request_Flush{Flush:&protoabci.RequestFlush{}}}
func ToRequestFlush() *protoabci.Request {
	return &protoabci.Request{
		Value: &protoabci.Request_Flush{&protoabci.RequestFlush{}},
	}
}

// ToRequestInfo 将 protoabci.RequestInfo 类型的 req 消息，包装成：
// &protoabci.Request{Value: &protoabci.Request_Info{Info:&req}}
func ToRequestInfo(req protoabci.RequestInfo) *protoabci.Request {
	return &protoabci.Request{
		Value: &protoabci.Request_Info{&req},
	}
}

// ToRequestSetOption 将 protoabci.RequestSetOption 类型的 req 消息，包装成：
// &protoabci.Request{Value: &protoabci.Request_SetOption{SetOption:&req}}
func ToRequestSetOption(req protoabci.RequestSetOption) *protoabci.Request {
	return &protoabci.Request{
		Value: &protoabci.Request_SetOption{&req},
	}
}

// ToRequestDeliverTx 将 protoabci.RequestDeliverTx 类型的 req 消息，包装成：
// &protoabci.Request{Value: &protoabci.Request_DeliverTx{DeliverTx:&req}}
func ToRequestDeliverTx(req protoabci.RequestDeliverTx) *protoabci.Request {
	return &protoabci.Request{
		Value: &protoabci.Request_DeliverTx{&req},
	}
}

// ToRequestCheckTx 将 protoabci.RequestCheckTx 类型的 req 消息，包装成：
// &protoabci.Request{Value: &protoabci.Request_CheckTx{CheckTx:&req}}
func ToRequestCheckTx(req protoabci.RequestCheckTx) *protoabci.Request {
	return &protoabci.Request{
		Value: &protoabci.Request_CheckTx{&req},
	}
}

// ToRequestCommit 将一个空的 &protoabci.RequestCommit{}，包装成：
// &protoabci.Request{Value: &protoabci.Request_Commit{Reply:&protoabci.RequestCommit{}}}
func ToRequestCommit() *protoabci.Request {
	return &protoabci.Request{
		Value: &protoabci.Request_Commit{&protoabci.RequestCommit{}},
	}
}

// ToRequestQuery 将 protoabci.RequestQuery 类型的 req 消息，包装成：
// &protoabci.Request{Value: &protoabci.Request_Query{Query:&req}}
func ToRequestQuery(req protoabci.RequestQuery) *protoabci.Request {
	return &protoabci.Request{
		Value: &protoabci.Request_Query{&req},
	}
}

// ToRequestInitChain 将 protoabci.RequestInitChain 类型的 req 消息，包装成：
// &protoabci.Request{Value: &protoabci.Request_InitChain{InitChain:&req}}
func ToRequestInitChain(req protoabci.RequestInitChain) *protoabci.Request {
	return &protoabci.Request{
		Value: &protoabci.Request_InitChain{&req},
	}
}

// ToRequestBeginBlock 将 protoabci.RequestBeginBlock 类型的 req 消息，包装成：
// &protoabci.Request{Value: &protoabci.Request_BeginBlock{BeginBlock:&req}}
func ToRequestBeginBlock(req protoabci.RequestBeginBlock) *protoabci.Request {
	return &protoabci.Request{
		Value: &protoabci.Request_BeginBlock{&req},
	}
}

// ToRequestEndBlock 将 protoabci.RequestEndBlock 类型的 req 消息，包装成：
// &protoabci.Request{Value: &protoabci.Request_EndBlock{EndBlock:&req}}
func ToRequestEndBlock(req protoabci.RequestEndBlock) *protoabci.Request {
	return &protoabci.Request{
		Value: &protoabci.Request_EndBlock{&req},
	}
}

// ToResponseException 将 string 类型的 errStr 消息，包装成：
// &protoabci.Response{Value: &protoabci.Response_Exception{Exception:&protoabci.ResponseException{Error: errStr}}}
func ToResponseException(errStr string) *protoabci.Response {
	return &protoabci.Response{
		Value: &protoabci.Response_Exception{&protoabci.ResponseException{Error: errStr}},
	}
}

// ToResponseEcho 将 string 类型的 message 消息，包装成：
// &protoabci.Response{Value: &protoabci.Response_Echo{Echo:&protoabci.ResponseEcho{Message: message}}}
func ToResponseEcho(message string) *protoabci.Response {
	return &protoabci.Response{
		Value: &protoabci.Response_Echo{&protoabci.ResponseEcho{Message: message}},
	}
}

// ToResponseFlush 将一个空的 &protoabci.ResponseFlush{} 消息，包装成：
// &protoabci.Response{Value: &protoabci.Response_Flush{Flush:&protoabci.ResponseFlush{}}}
func ToResponseFlush() *protoabci.Response {
	return &protoabci.Response{
		Value: &protoabci.Response_Flush{&protoabci.ResponseFlush{}},
	}
}

// ToResponseInfo 将 protoabci.ResponseInfo 类型的 res 消息，包装成：
// &protoabci.Response{Value: &protoabci.Response_Info{Info:&res}}
func ToResponseInfo(res protoabci.ResponseInfo) *protoabci.Response {
	return &protoabci.Response{
		Value: &protoabci.Response_Info{&res},
	}
}

// ToResponseSetOption 将 protoabci.ResponseSetOption 类型的 res 消息，包装成：
// &protoabci.Response{Value: &protoabci.Response_SetOption{SetOption:&res}}
func ToResponseSetOption(res protoabci.ResponseSetOption) *protoabci.Response {
	return &protoabci.Response{
		Value: &protoabci.Response_SetOption{&res},
	}
}

// ToResponseDeliverTx 将 protoabci.ResponseDeliverTx 类型的 res 消息，包装成：
// &protoabci.Response{Value: &protoabci.Response_DeliverTx{DeliverTx:&res}}
func ToResponseDeliverTx(res protoabci.ResponseDeliverTx) *protoabci.Response {
	return &protoabci.Response{
		Value: &protoabci.Response_DeliverTx{&res},
	}
}

// ToResponseCheckTx 将 protoabci.ResponseCheckTx 类型的 res 消息，包装成：
// &protoabci.Response{Value: &protoabci.Response_CheckTx{CheckTx:&res}}
func ToResponseCheckTx(res protoabci.ResponseCheckTx) *protoabci.Response {
	return &protoabci.Response{
		Value: &protoabci.Response_CheckTx{&res},
	}
}

// ToResponseCommit 将 protoabci.ResponseCommit 类型的 res 消息，包装成：
// &protoabci.Response{Value: &protoabci.Response_Commit{Reply:&res}}
func ToResponseCommit(res protoabci.ResponseCommit) *protoabci.Response {
	return &protoabci.Response{
		Value: &protoabci.Response_Commit{&res},
	}
}

// ToResponseQuery 将 protoabci.ResponseQuery 类型的 res 消息，包装成：
// &protoabci.Response{Value: &protoabci.Response_Query{Query:&res}}
func ToResponseQuery(res protoabci.ResponseQuery) *protoabci.Response {
	return &protoabci.Response{
		Value: &protoabci.Response_Query{&res},
	}
}

// ToResponseInitChain 将 protoabci.ResponseInitChain 类型的 res 消息，包装成：
// &protoabci.Response{Value: &protoabci.Response_InitChain{InitChain:&res}}
func ToResponseInitChain(res protoabci.ResponseInitChain) *protoabci.Response {
	return &protoabci.Response{
		Value: &protoabci.Response_InitChain{&res},
	}
}

// ToResponseBeginBlock 将 protoabci.ResponseBeginBlock 类型的 res 消息，包装成：
// &protoabci.Response{Value: &protoabci.Response_BeginBlock{BeginBlock:&res}}
func ToResponseBeginBlock(res protoabci.ResponseBeginBlock) *protoabci.Response {
	return &protoabci.Response{
		Value: &protoabci.Response_BeginBlock{&res},
	}
}

// ToResponseEndBlock 将 protoabci.ResponseEndBlock 类型的 res 消息，包装成：
// &protoabci.Response{Value: &protoabci.Response_EndBlock{EndBlock:&res}}
func ToResponseEndBlock(res protoabci.ResponseEndBlock) *protoabci.Response {
	return &protoabci.Response{
		Value: &protoabci.Response_EndBlock{&res},
	}
}
