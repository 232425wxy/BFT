// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: conn.proto

package gossip

import (
	crypto "github.com/232425wxy/BFT/proto/crypto"
	fmt "fmt"
	_ "github.com/gogo/protobuf/gogoproto"
	proto "github.com/golang/protobuf/proto"
	io "io"
	math "math"
	math_bits "math/bits"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type PacketPing struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *PacketPing) Reset()         { *m = PacketPing{} }
func (m *PacketPing) String() string { return proto.CompactTextString(m) }
func (*PacketPing) ProtoMessage()    {}
func (*PacketPing) Descriptor() ([]byte, []int) {
	return fileDescriptor_f401a58c1fc7ceef, []int{0}
}
func (m *PacketPing) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *PacketPing) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_PacketPing.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *PacketPing) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PacketPing.Merge(m, src)
}
func (m *PacketPing) XXX_Size() int {
	return m.Size()
}
func (m *PacketPing) XXX_DiscardUnknown() {
	xxx_messageInfo_PacketPing.DiscardUnknown(m)
}

var xxx_messageInfo_PacketPing proto.InternalMessageInfo

type PacketPong struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *PacketPong) Reset()         { *m = PacketPong{} }
func (m *PacketPong) String() string { return proto.CompactTextString(m) }
func (*PacketPong) ProtoMessage()    {}
func (*PacketPong) Descriptor() ([]byte, []int) {
	return fileDescriptor_f401a58c1fc7ceef, []int{1}
}
func (m *PacketPong) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *PacketPong) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_PacketPong.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *PacketPong) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PacketPong.Merge(m, src)
}
func (m *PacketPong) XXX_Size() int {
	return m.Size()
}
func (m *PacketPong) XXX_DiscardUnknown() {
	xxx_messageInfo_PacketPong.DiscardUnknown(m)
}

var xxx_messageInfo_PacketPong proto.InternalMessageInfo

type PacketMsg struct {
	ChannelID            int32    `protobuf:"varint,1,opt,name=channel_id,json=channelId,proto3" json:"channel_id,omitempty"`
	EOF                  bool     `protobuf:"varint,2,opt,name=eof,proto3" json:"eof,omitempty"`
	Data                 []byte   `protobuf:"bytes,3,opt,name=data,proto3" json:"data,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *PacketMsg) Reset()         { *m = PacketMsg{} }
func (m *PacketMsg) String() string { return proto.CompactTextString(m) }
func (*PacketMsg) ProtoMessage()    {}
func (*PacketMsg) Descriptor() ([]byte, []int) {
	return fileDescriptor_f401a58c1fc7ceef, []int{2}
}
func (m *PacketMsg) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *PacketMsg) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_PacketMsg.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *PacketMsg) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PacketMsg.Merge(m, src)
}
func (m *PacketMsg) XXX_Size() int {
	return m.Size()
}
func (m *PacketMsg) XXX_DiscardUnknown() {
	xxx_messageInfo_PacketMsg.DiscardUnknown(m)
}

var xxx_messageInfo_PacketMsg proto.InternalMessageInfo

func (m *PacketMsg) GetChannelID() int32 {
	if m != nil {
		return m.ChannelID
	}
	return 0
}

func (m *PacketMsg) GetEOF() bool {
	if m != nil {
		return m.EOF
	}
	return false
}

func (m *PacketMsg) GetData() []byte {
	if m != nil {
		return m.Data
	}
	return nil
}

type Packet struct {
	// Types that are valid to be assigned to Sum:
	//	*Packet_PacketPing
	//	*Packet_PacketPong
	//	*Packet_PacketMsg
	Sum                  isPacket_Sum `protobuf_oneof:"sum"`
	XXX_NoUnkeyedLiteral struct{}     `json:"-"`
	XXX_unrecognized     []byte       `json:"-"`
	XXX_sizecache        int32        `json:"-"`
}

func (m *Packet) Reset()         { *m = Packet{} }
func (m *Packet) String() string { return proto.CompactTextString(m) }
func (*Packet) ProtoMessage()    {}
func (*Packet) Descriptor() ([]byte, []int) {
	return fileDescriptor_f401a58c1fc7ceef, []int{3}
}
func (m *Packet) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Packet) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Packet.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Packet) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Packet.Merge(m, src)
}
func (m *Packet) XXX_Size() int {
	return m.Size()
}
func (m *Packet) XXX_DiscardUnknown() {
	xxx_messageInfo_Packet.DiscardUnknown(m)
}

var xxx_messageInfo_Packet proto.InternalMessageInfo

type isPacket_Sum interface {
	isPacket_Sum()
	MarshalTo([]byte) (int, error)
	Size() int
}

type Packet_PacketPing struct {
	PacketPing *PacketPing `protobuf:"bytes,1,opt,name=packet_ping,json=packetPing,proto3,oneof" json:"packet_ping,omitempty"`
}
type Packet_PacketPong struct {
	PacketPong *PacketPong `protobuf:"bytes,2,opt,name=packet_pong,json=packetPong,proto3,oneof" json:"packet_pong,omitempty"`
}
type Packet_PacketMsg struct {
	PacketMsg *PacketMsg `protobuf:"bytes,3,opt,name=packet_msg,json=packetMsg,proto3,oneof" json:"packet_msg,omitempty"`
}

func (*Packet_PacketPing) isPacket_Sum() {}
func (*Packet_PacketPong) isPacket_Sum() {}
func (*Packet_PacketMsg) isPacket_Sum()  {}

func (m *Packet) GetSum() isPacket_Sum {
	if m != nil {
		return m.Sum
	}
	return nil
}

func (m *Packet) GetPacketPing() *PacketPing {
	if x, ok := m.GetSum().(*Packet_PacketPing); ok {
		return x.PacketPing
	}
	return nil
}

func (m *Packet) GetPacketPong() *PacketPong {
	if x, ok := m.GetSum().(*Packet_PacketPong); ok {
		return x.PacketPong
	}
	return nil
}

func (m *Packet) GetPacketMsg() *PacketMsg {
	if x, ok := m.GetSum().(*Packet_PacketMsg); ok {
		return x.PacketMsg
	}
	return nil
}

// XXX_OneofWrappers is for the internal use of the proto package.
func (*Packet) XXX_OneofWrappers() []interface{} {
	return []interface{}{
		(*Packet_PacketPing)(nil),
		(*Packet_PacketPong)(nil),
		(*Packet_PacketMsg)(nil),
	}
}

type AuthSigMessage struct {
	PubKey               crypto.PublicKey `protobuf:"bytes,1,opt,name=pub_key,json=pubKey,proto3" json:"pub_key"`
	Sig                  []byte           `protobuf:"bytes,2,opt,name=sig,proto3" json:"sig,omitempty"`
	XXX_NoUnkeyedLiteral struct{}         `json:"-"`
	XXX_unrecognized     []byte           `json:"-"`
	XXX_sizecache        int32            `json:"-"`
}

func (m *AuthSigMessage) Reset()         { *m = AuthSigMessage{} }
func (m *AuthSigMessage) String() string { return proto.CompactTextString(m) }
func (*AuthSigMessage) ProtoMessage()    {}
func (*AuthSigMessage) Descriptor() ([]byte, []int) {
	return fileDescriptor_f401a58c1fc7ceef, []int{4}
}
func (m *AuthSigMessage) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *AuthSigMessage) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_AuthSigMessage.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *AuthSigMessage) XXX_Merge(src proto.Message) {
	xxx_messageInfo_AuthSigMessage.Merge(m, src)
}
func (m *AuthSigMessage) XXX_Size() int {
	return m.Size()
}
func (m *AuthSigMessage) XXX_DiscardUnknown() {
	xxx_messageInfo_AuthSigMessage.DiscardUnknown(m)
}

var xxx_messageInfo_AuthSigMessage proto.InternalMessageInfo

func (m *AuthSigMessage) GetPubKey() crypto.PublicKey {
	if m != nil {
		return m.PubKey
	}
	return crypto.PublicKey{}
}

func (m *AuthSigMessage) GetSig() []byte {
	if m != nil {
		return m.Sig
	}
	return nil
}

func init() {
	proto.RegisterType((*PacketPing)(nil), "gossip.PacketPing")
	proto.RegisterType((*PacketPong)(nil), "gossip.PacketPong")
	proto.RegisterType((*PacketMsg)(nil), "gossip.PacketMsg")
	proto.RegisterType((*Packet)(nil), "gossip.Packet")
	proto.RegisterType((*AuthSigMessage)(nil), "gossip.AuthSigMessage")
}

func init() { proto.RegisterFile("conn.proto", fileDescriptor_f401a58c1fc7ceef) }

var fileDescriptor_f401a58c1fc7ceef = []byte{
	// 350 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x64, 0x91, 0xd1, 0x4e, 0xfa, 0x30,
	0x18, 0xc5, 0xe9, 0x7f, 0x30, 0xfe, 0xfb, 0x40, 0x03, 0x8d, 0x17, 0x88, 0xc9, 0x20, 0xbb, 0xe2,
	0xc2, 0x0c, 0x83, 0xf1, 0x01, 0x9c, 0x4a, 0x24, 0x84, 0x48, 0x26, 0xf7, 0xcb, 0x36, 0x6a, 0x59,
	0x80, 0xb6, 0xa1, 0xdb, 0xc5, 0xde, 0xc4, 0xd7, 0xf0, 0x2d, 0xb8, 0xf4, 0x09, 0x88, 0x99, 0x2f,
	0x62, 0xb6, 0xa2, 0x42, 0xbc, 0x3b, 0xe7, 0x7c, 0xfd, 0xe5, 0x3b, 0x6d, 0x01, 0x42, 0xce, 0x98,
	0x2d, 0x36, 0x3c, 0xe6, 0x58, 0xa7, 0x5c, 0xca, 0x48, 0xb4, 0xcf, 0x28, 0xa7, 0xbc, 0x88, 0xfa,
	0xb9, 0x52, 0xd3, 0xf6, 0x85, 0x33, 0x9c, 0xf5, 0x55, 0x1a, 0x6e, 0x52, 0x11, 0xf3, 0xfe, 0x92,
	0xa4, 0x52, 0x0d, 0xad, 0x3a, 0xc0, 0xd4, 0x0f, 0x97, 0x24, 0x9e, 0x46, 0x8c, 0x1e, 0x38, 0xce,
	0xa8, 0xb5, 0x00, 0x43, 0xb9, 0x89, 0xa4, 0xf8, 0x12, 0x20, 0x5c, 0xf8, 0x8c, 0x91, 0x95, 0x17,
	0xcd, 0x5b, 0xa8, 0x8b, 0x7a, 0x15, 0xe7, 0x24, 0xdb, 0x75, 0x8c, 0x3b, 0x95, 0x8e, 0xee, 0x5d,
	0x63, 0x7f, 0x60, 0x34, 0xc7, 0xe7, 0xa0, 0x11, 0xfe, 0xd2, 0xfa, 0xd7, 0x45, 0xbd, 0xff, 0x4e,
	0x35, 0xdb, 0x75, 0xb4, 0x87, 0xa7, 0xa1, 0x9b, 0x67, 0x18, 0x43, 0x79, 0xee, 0xc7, 0x7e, 0x4b,
	0xeb, 0xa2, 0x5e, 0xdd, 0x2d, 0xb4, 0xf5, 0x86, 0x40, 0x57, 0xab, 0xf0, 0x0d, 0xd4, 0x44, 0xa1,
	0x3c, 0x11, 0x31, 0x5a, 0x2c, 0xaa, 0x0d, 0xb0, 0xad, 0x6e, 0x68, 0xff, 0x76, 0x7d, 0x2c, 0xb9,
	0x20, 0x7e, 0xdc, 0x21, 0xc6, 0x19, 0x2d, 0x16, 0xff, 0xc5, 0xf8, 0x11, 0xc6, 0x19, 0xc5, 0x03,
	0xd8, 0x3b, 0x6f, 0x2d, 0x69, 0x51, 0xa9, 0x36, 0x68, 0x1e, 0x53, 0x13, 0x99, 0x43, 0x86, 0xf8,
	0x36, 0x4e, 0x05, 0x34, 0x99, 0xac, 0xad, 0x19, 0x9c, 0xde, 0x26, 0xf1, 0xe2, 0x39, 0xa2, 0x13,
	0x22, 0xa5, 0x4f, 0x09, 0xbe, 0x82, 0xaa, 0x48, 0x02, 0x6f, 0x49, 0xd2, 0x7d, 0xed, 0xa6, 0xad,
	0x1e, 0xdc, 0x9e, 0x26, 0xc1, 0x2a, 0x0a, 0xc7, 0x24, 0x75, 0xca, 0xdb, 0x5d, 0xa7, 0xe4, 0xea,
	0x22, 0x09, 0xc6, 0x24, 0xc5, 0x0d, 0xd0, 0x64, 0xa4, 0xda, 0xd6, 0xdd, 0x5c, 0x3a, 0x8d, 0x6d,
	0x66, 0xa2, 0xf7, 0xcc, 0x44, 0x1f, 0x99, 0x89, 0x5e, 0x3f, 0xcd, 0x52, 0xa0, 0x17, 0x1f, 0x75,
	0xfd, 0x15, 0x00, 0x00, 0xff, 0xff, 0x1f, 0xb9, 0x95, 0x43, 0xf1, 0x01, 0x00, 0x00,
}

func (m *PacketPing) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *PacketPing) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *PacketPing) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	return len(dAtA) - i, nil
}

func (m *PacketPong) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *PacketPong) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *PacketPong) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	return len(dAtA) - i, nil
}

func (m *PacketMsg) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *PacketMsg) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *PacketMsg) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if len(m.Data) > 0 {
		i -= len(m.Data)
		copy(dAtA[i:], m.Data)
		i = encodeVarintConn(dAtA, i, uint64(len(m.Data)))
		i--
		dAtA[i] = 0x1a
	}
	if m.EOF {
		i--
		if m.EOF {
			dAtA[i] = 1
		} else {
			dAtA[i] = 0
		}
		i--
		dAtA[i] = 0x10
	}
	if m.ChannelID != 0 {
		i = encodeVarintConn(dAtA, i, uint64(m.ChannelID))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func (m *Packet) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Packet) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Packet) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if m.Sum != nil {
		{
			size := m.Sum.Size()
			i -= size
			if _, err := m.Sum.MarshalTo(dAtA[i:]); err != nil {
				return 0, err
			}
		}
	}
	return len(dAtA) - i, nil
}

func (m *Packet_PacketPing) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Packet_PacketPing) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	if m.PacketPing != nil {
		{
			size, err := m.PacketPing.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintConn(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}
func (m *Packet_PacketPong) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Packet_PacketPong) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	if m.PacketPong != nil {
		{
			size, err := m.PacketPong.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintConn(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x12
	}
	return len(dAtA) - i, nil
}
func (m *Packet_PacketMsg) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Packet_PacketMsg) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	if m.PacketMsg != nil {
		{
			size, err := m.PacketMsg.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintConn(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x1a
	}
	return len(dAtA) - i, nil
}
func (m *AuthSigMessage) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *AuthSigMessage) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *AuthSigMessage) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if len(m.Sig) > 0 {
		i -= len(m.Sig)
		copy(dAtA[i:], m.Sig)
		i = encodeVarintConn(dAtA, i, uint64(len(m.Sig)))
		i--
		dAtA[i] = 0x12
	}
	{
		size, err := m.PubKey.MarshalToSizedBuffer(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = encodeVarintConn(dAtA, i, uint64(size))
	}
	i--
	dAtA[i] = 0xa
	return len(dAtA) - i, nil
}

func encodeVarintConn(dAtA []byte, offset int, v uint64) int {
	offset -= sovConn(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *PacketPing) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func (m *PacketPong) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func (m *PacketMsg) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.ChannelID != 0 {
		n += 1 + sovConn(uint64(m.ChannelID))
	}
	if m.EOF {
		n += 2
	}
	l = len(m.Data)
	if l > 0 {
		n += 1 + l + sovConn(uint64(l))
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func (m *Packet) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Sum != nil {
		n += m.Sum.Size()
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func (m *Packet_PacketPing) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.PacketPing != nil {
		l = m.PacketPing.Size()
		n += 1 + l + sovConn(uint64(l))
	}
	return n
}
func (m *Packet_PacketPong) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.PacketPong != nil {
		l = m.PacketPong.Size()
		n += 1 + l + sovConn(uint64(l))
	}
	return n
}
func (m *Packet_PacketMsg) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.PacketMsg != nil {
		l = m.PacketMsg.Size()
		n += 1 + l + sovConn(uint64(l))
	}
	return n
}
func (m *AuthSigMessage) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = m.PubKey.Size()
	n += 1 + l + sovConn(uint64(l))
	l = len(m.Sig)
	if l > 0 {
		n += 1 + l + sovConn(uint64(l))
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func sovConn(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozConn(x uint64) (n int) {
	return sovConn(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *PacketPing) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowConn
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: PacketPing: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: PacketPing: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		default:
			iNdEx = preIndex
			skippy, err := skipConn(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthConn
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthConn
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *PacketPong) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowConn
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: PacketPong: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: PacketPong: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		default:
			iNdEx = preIndex
			skippy, err := skipConn(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthConn
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthConn
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *PacketMsg) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowConn
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: PacketMsg: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: PacketMsg: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field ChannelID", wireType)
			}
			m.ChannelID = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowConn
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.ChannelID |= int32(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field EOF", wireType)
			}
			var v int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowConn
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				v |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			m.EOF = bool(v != 0)
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Data", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowConn
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthConn
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthConn
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Data = append(m.Data[:0], dAtA[iNdEx:postIndex]...)
			if m.Data == nil {
				m.Data = []byte{}
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipConn(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthConn
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthConn
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *Packet) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowConn
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Packet: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Packet: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field PacketPing", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowConn
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthConn
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthConn
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			v := &PacketPing{}
			if err := v.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			m.Sum = &Packet_PacketPing{v}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field PacketPong", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowConn
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthConn
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthConn
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			v := &PacketPong{}
			if err := v.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			m.Sum = &Packet_PacketPong{v}
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field PacketMsg", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowConn
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthConn
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthConn
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			v := &PacketMsg{}
			if err := v.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			m.Sum = &Packet_PacketMsg{v}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipConn(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthConn
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthConn
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *AuthSigMessage) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowConn
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: AuthSigMessage: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: AuthSigMessage: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field PubKey", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowConn
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthConn
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthConn
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := m.PubKey.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Sig", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowConn
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthConn
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthConn
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Sig = append(m.Sig[:0], dAtA[iNdEx:postIndex]...)
			if m.Sig == nil {
				m.Sig = []byte{}
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipConn(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthConn
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthConn
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipConn(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowConn
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowConn
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
		case 1:
			iNdEx += 8
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowConn
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthConn
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupConn
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthConn
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthConn        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowConn          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupConn = fmt.Errorf("proto: unexpected end of group")
)
