// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: params.proto

package types

import (
	bytes "bytes"
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

// ConsensusParams contains consensus critical parameters that determine the
// validity of blocks.
type ConsensusParams struct {
	Block                BlockParams     `protobuf:"bytes,1,opt,name=block,proto3" json:"block"`
	Validator            ValidatorParams `protobuf:"bytes,2,opt,name=validator,proto3" json:"validator"`
	XXX_NoUnkeyedLiteral struct{}        `json:"-"`
	XXX_unrecognized     []byte          `json:"-"`
	XXX_sizecache        int32           `json:"-"`
}

func (m *ConsensusParams) Reset()         { *m = ConsensusParams{} }
func (m *ConsensusParams) String() string { return proto.CompactTextString(m) }
func (*ConsensusParams) ProtoMessage()    {}
func (*ConsensusParams) Descriptor() ([]byte, []int) {
	return fileDescriptor_8679b07c520418a1, []int{0}
}
func (m *ConsensusParams) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *ConsensusParams) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_ConsensusParams.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *ConsensusParams) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ConsensusParams.Merge(m, src)
}
func (m *ConsensusParams) XXX_Size() int {
	return m.Size()
}
func (m *ConsensusParams) XXX_DiscardUnknown() {
	xxx_messageInfo_ConsensusParams.DiscardUnknown(m)
}

var xxx_messageInfo_ConsensusParams proto.InternalMessageInfo

func (m *ConsensusParams) GetBlock() BlockParams {
	if m != nil {
		return m.Block
	}
	return BlockParams{}
}

func (m *ConsensusParams) GetValidator() ValidatorParams {
	if m != nil {
		return m.Validator
	}
	return ValidatorParams{}
}

// BlockParams contains limits on the block size.
type BlockParams struct {
	// Max block size, in bytes.
	// Note: must be greater than 0
	MaxBytes int64 `protobuf:"varint,1,opt,name=max_bytes,json=maxBytes,proto3" json:"max_bytes,omitempty"`
	// Minimum time increment between consecutive blocks (in milliseconds) If the
	// block header timestamp is ahead of the system clock, decrease this value.
	//
	// Not exposed to the application.
	TimeIotaMs           int64    `protobuf:"varint,2,opt,name=time_iota_ms,json=timeIotaMs,proto3" json:"time_iota_ms,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *BlockParams) Reset()         { *m = BlockParams{} }
func (m *BlockParams) String() string { return proto.CompactTextString(m) }
func (*BlockParams) ProtoMessage()    {}
func (*BlockParams) Descriptor() ([]byte, []int) {
	return fileDescriptor_8679b07c520418a1, []int{1}
}
func (m *BlockParams) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *BlockParams) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_BlockParams.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *BlockParams) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BlockParams.Merge(m, src)
}
func (m *BlockParams) XXX_Size() int {
	return m.Size()
}
func (m *BlockParams) XXX_DiscardUnknown() {
	xxx_messageInfo_BlockParams.DiscardUnknown(m)
}

var xxx_messageInfo_BlockParams proto.InternalMessageInfo

func (m *BlockParams) GetMaxBytes() int64 {
	if m != nil {
		return m.MaxBytes
	}
	return 0
}

func (m *BlockParams) GetTimeIotaMs() int64 {
	if m != nil {
		return m.TimeIotaMs
	}
	return 0
}

// ValidatorParams restrict the public key types validators can use.
// NOTE: uses ABCI pubkey naming, not Amino names.
type ValidatorParams struct {
	PubKeyTypes          []string `protobuf:"bytes,1,rep,name=pub_key_types,json=pubKeyTypes,proto3" json:"pub_key_types,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ValidatorParams) Reset()         { *m = ValidatorParams{} }
func (m *ValidatorParams) String() string { return proto.CompactTextString(m) }
func (*ValidatorParams) ProtoMessage()    {}
func (*ValidatorParams) Descriptor() ([]byte, []int) {
	return fileDescriptor_8679b07c520418a1, []int{2}
}
func (m *ValidatorParams) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *ValidatorParams) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_ValidatorParams.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *ValidatorParams) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ValidatorParams.Merge(m, src)
}
func (m *ValidatorParams) XXX_Size() int {
	return m.Size()
}
func (m *ValidatorParams) XXX_DiscardUnknown() {
	xxx_messageInfo_ValidatorParams.DiscardUnknown(m)
}

var xxx_messageInfo_ValidatorParams proto.InternalMessageInfo

func (m *ValidatorParams) GetPubKeyTypes() []string {
	if m != nil {
		return m.PubKeyTypes
	}
	return nil
}

// HashedParams is a subset of ConsensusParams.
//
// It is hashed into the Header.ConsensusHash.
type HashedParams struct {
	BlockMaxBytes        int64    `protobuf:"varint,1,opt,name=block_max_bytes,json=blockMaxBytes,proto3" json:"block_max_bytes,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *HashedParams) Reset()         { *m = HashedParams{} }
func (m *HashedParams) String() string { return proto.CompactTextString(m) }
func (*HashedParams) ProtoMessage()    {}
func (*HashedParams) Descriptor() ([]byte, []int) {
	return fileDescriptor_8679b07c520418a1, []int{3}
}
func (m *HashedParams) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *HashedParams) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_HashedParams.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *HashedParams) XXX_Merge(src proto.Message) {
	xxx_messageInfo_HashedParams.Merge(m, src)
}
func (m *HashedParams) XXX_Size() int {
	return m.Size()
}
func (m *HashedParams) XXX_DiscardUnknown() {
	xxx_messageInfo_HashedParams.DiscardUnknown(m)
}

var xxx_messageInfo_HashedParams proto.InternalMessageInfo

func (m *HashedParams) GetBlockMaxBytes() int64 {
	if m != nil {
		return m.BlockMaxBytes
	}
	return 0
}

func init() {
	proto.RegisterType((*ConsensusParams)(nil), "types.ConsensusParams")
	proto.RegisterType((*BlockParams)(nil), "types.BlockParams")
	proto.RegisterType((*ValidatorParams)(nil), "types.ValidatorParams")
	proto.RegisterType((*HashedParams)(nil), "types.HashedParams")
}

func init() { proto.RegisterFile("params.proto", fileDescriptor_8679b07c520418a1) }

var fileDescriptor_8679b07c520418a1 = []byte{
	// 298 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0x29, 0x48, 0x2c, 0x4a,
	0xcc, 0x2d, 0xd6, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x62, 0x2d, 0xa9, 0x2c, 0x48, 0x2d, 0x96,
	0x12, 0x49, 0xcf, 0x4f, 0xcf, 0x07, 0x8b, 0xe8, 0x83, 0x58, 0x10, 0x49, 0xa5, 0x5a, 0x2e, 0x7e,
	0xe7, 0xfc, 0xbc, 0xe2, 0xd4, 0xbc, 0xe2, 0xd2, 0xe2, 0x00, 0xb0, 0x2e, 0x21, 0x3d, 0x2e, 0xd6,
	0xa4, 0x9c, 0xfc, 0xe4, 0x6c, 0x09, 0x46, 0x05, 0x46, 0x0d, 0x6e, 0x23, 0x21, 0x3d, 0xb0, 0x7e,
	0x3d, 0x27, 0x90, 0x18, 0x44, 0x89, 0x13, 0xcb, 0x89, 0x7b, 0xf2, 0x0c, 0x41, 0x10, 0x65, 0x42,
	0x56, 0x5c, 0x9c, 0x65, 0x89, 0x39, 0x99, 0x29, 0x89, 0x25, 0xf9, 0x45, 0x12, 0x4c, 0x60, 0x3d,
	0x62, 0x50, 0x3d, 0x61, 0x30, 0x71, 0x14, 0x7d, 0x08, 0xe5, 0x4a, 0x3e, 0x5c, 0xdc, 0x48, 0xe6,
	0x0a, 0x49, 0x73, 0x71, 0xe6, 0x26, 0x56, 0xc4, 0x27, 0x55, 0x96, 0xa4, 0x16, 0x83, 0xad, 0x67,
	0x0e, 0xe2, 0xc8, 0x4d, 0xac, 0x70, 0x02, 0xf1, 0x85, 0x14, 0xb8, 0x78, 0x4a, 0x32, 0x73, 0x53,
	0xe3, 0x33, 0xf3, 0x4b, 0x12, 0xe3, 0x73, 0x8b, 0xc1, 0x56, 0x31, 0x07, 0x71, 0x81, 0xc4, 0x3c,
	0xf3, 0x4b, 0x12, 0x7d, 0x8b, 0x95, 0xec, 0xb9, 0xf8, 0xd1, 0x6c, 0x14, 0x52, 0xe2, 0xe2, 0x2d,
	0x28, 0x4d, 0x8a, 0xcf, 0x4e, 0xad, 0x8c, 0x07, 0x3b, 0x49, 0x82, 0x51, 0x81, 0x59, 0x83, 0x33,
	0x88, 0xbb, 0xa0, 0x34, 0xc9, 0x3b, 0xb5, 0x32, 0x04, 0x24, 0x64, 0xc5, 0xb1, 0x63, 0x81, 0x3c,
	0xe3, 0x8b, 0x05, 0xf2, 0x8c, 0x4a, 0x66, 0x5c, 0x3c, 0x1e, 0x89, 0xc5, 0x19, 0xa9, 0x29, 0x50,
	0xdd, 0x6a, 0x5c, 0xfc, 0x60, 0x3f, 0xc6, 0xa3, 0xbb, 0x8a, 0x17, 0x2c, 0xec, 0x0b, 0x75, 0x9a,
	0x93, 0xc8, 0x8a, 0x47, 0x72, 0x8c, 0x27, 0x1e, 0xc9, 0x31, 0x5e, 0x78, 0x24, 0xc7, 0xf8, 0xe0,
	0x91, 0x1c, 0xe3, 0x8c, 0xc7, 0x72, 0x0c, 0x49, 0x6c, 0xe0, 0x20, 0x36, 0x06, 0x04, 0x00, 0x00,
	0xff, 0xff, 0xed, 0x8f, 0xb1, 0x34, 0x8f, 0x01, 0x00, 0x00,
}

func (this *ConsensusParams) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*ConsensusParams)
	if !ok {
		that2, ok := that.(ConsensusParams)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if !this.Block.Equal(&that1.Block) {
		return false
	}
	if !this.Validator.Equal(&that1.Validator) {
		return false
	}
	if !bytes.Equal(this.XXX_unrecognized, that1.XXX_unrecognized) {
		return false
	}
	return true
}
func (this *BlockParams) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*BlockParams)
	if !ok {
		that2, ok := that.(BlockParams)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if this.MaxBytes != that1.MaxBytes {
		return false
	}
	if this.TimeIotaMs != that1.TimeIotaMs {
		return false
	}
	if !bytes.Equal(this.XXX_unrecognized, that1.XXX_unrecognized) {
		return false
	}
	return true
}
func (this *ValidatorParams) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*ValidatorParams)
	if !ok {
		that2, ok := that.(ValidatorParams)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if len(this.PubKeyTypes) != len(that1.PubKeyTypes) {
		return false
	}
	for i := range this.PubKeyTypes {
		if this.PubKeyTypes[i] != that1.PubKeyTypes[i] {
			return false
		}
	}
	if !bytes.Equal(this.XXX_unrecognized, that1.XXX_unrecognized) {
		return false
	}
	return true
}
func (this *HashedParams) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*HashedParams)
	if !ok {
		that2, ok := that.(HashedParams)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if this.BlockMaxBytes != that1.BlockMaxBytes {
		return false
	}
	if !bytes.Equal(this.XXX_unrecognized, that1.XXX_unrecognized) {
		return false
	}
	return true
}
func (m *ConsensusParams) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *ConsensusParams) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *ConsensusParams) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	{
		size, err := m.Validator.MarshalToSizedBuffer(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = encodeVarintParams(dAtA, i, uint64(size))
	}
	i--
	dAtA[i] = 0x12
	{
		size, err := m.Block.MarshalToSizedBuffer(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = encodeVarintParams(dAtA, i, uint64(size))
	}
	i--
	dAtA[i] = 0xa
	return len(dAtA) - i, nil
}

func (m *BlockParams) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *BlockParams) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *BlockParams) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if m.TimeIotaMs != 0 {
		i = encodeVarintParams(dAtA, i, uint64(m.TimeIotaMs))
		i--
		dAtA[i] = 0x10
	}
	if m.MaxBytes != 0 {
		i = encodeVarintParams(dAtA, i, uint64(m.MaxBytes))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func (m *ValidatorParams) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *ValidatorParams) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *ValidatorParams) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if len(m.PubKeyTypes) > 0 {
		for iNdEx := len(m.PubKeyTypes) - 1; iNdEx >= 0; iNdEx-- {
			i -= len(m.PubKeyTypes[iNdEx])
			copy(dAtA[i:], m.PubKeyTypes[iNdEx])
			i = encodeVarintParams(dAtA, i, uint64(len(m.PubKeyTypes[iNdEx])))
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func (m *HashedParams) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *HashedParams) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *HashedParams) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if m.BlockMaxBytes != 0 {
		i = encodeVarintParams(dAtA, i, uint64(m.BlockMaxBytes))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func encodeVarintParams(dAtA []byte, offset int, v uint64) int {
	offset -= sovParams(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func NewPopulatedValidatorParams(r randyParams, easy bool) *ValidatorParams {
	this := &ValidatorParams{}
	v1 := r.Intn(10)
	this.PubKeyTypes = make([]string, v1)
	for i := 0; i < v1; i++ {
		this.PubKeyTypes[i] = string(randStringParams(r))
	}
	if !easy && r.Intn(10) != 0 {
		this.XXX_unrecognized = randUnrecognizedParams(r, 2)
	}
	return this
}

type randyParams interface {
	Float32() float32
	Float64() float64
	Int63() int64
	Int31() int32
	Uint32() uint32
	Intn(n int) int
}

func randUTF8RuneParams(r randyParams) rune {
	ru := r.Intn(62)
	if ru < 10 {
		return rune(ru + 48)
	} else if ru < 36 {
		return rune(ru + 55)
	}
	return rune(ru + 61)
}
func randStringParams(r randyParams) string {
	v2 := r.Intn(100)
	tmps := make([]rune, v2)
	for i := 0; i < v2; i++ {
		tmps[i] = randUTF8RuneParams(r)
	}
	return string(tmps)
}
func randUnrecognizedParams(r randyParams, maxFieldNumber int) (dAtA []byte) {
	l := r.Intn(5)
	for i := 0; i < l; i++ {
		wire := r.Intn(4)
		if wire == 3 {
			wire = 5
		}
		fieldNumber := maxFieldNumber + r.Intn(100)
		dAtA = randFieldParams(dAtA, r, fieldNumber, wire)
	}
	return dAtA
}
func randFieldParams(dAtA []byte, r randyParams, fieldNumber int, wire int) []byte {
	key := uint32(fieldNumber)<<3 | uint32(wire)
	switch wire {
	case 0:
		dAtA = encodeVarintPopulateParams(dAtA, uint64(key))
		v3 := r.Int63()
		if r.Intn(2) == 0 {
			v3 *= -1
		}
		dAtA = encodeVarintPopulateParams(dAtA, uint64(v3))
	case 1:
		dAtA = encodeVarintPopulateParams(dAtA, uint64(key))
		dAtA = append(dAtA, byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)))
	case 2:
		dAtA = encodeVarintPopulateParams(dAtA, uint64(key))
		ll := r.Intn(100)
		dAtA = encodeVarintPopulateParams(dAtA, uint64(ll))
		for j := 0; j < ll; j++ {
			dAtA = append(dAtA, byte(r.Intn(256)))
		}
	default:
		dAtA = encodeVarintPopulateParams(dAtA, uint64(key))
		dAtA = append(dAtA, byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)))
	}
	return dAtA
}
func encodeVarintPopulateParams(dAtA []byte, v uint64) []byte {
	for v >= 1<<7 {
		dAtA = append(dAtA, uint8(uint64(v)&0x7f|0x80))
		v >>= 7
	}
	dAtA = append(dAtA, uint8(v))
	return dAtA
}
func (m *ConsensusParams) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = m.Block.Size()
	n += 1 + l + sovParams(uint64(l))
	l = m.Validator.Size()
	n += 1 + l + sovParams(uint64(l))
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func (m *BlockParams) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.MaxBytes != 0 {
		n += 1 + sovParams(uint64(m.MaxBytes))
	}
	if m.TimeIotaMs != 0 {
		n += 1 + sovParams(uint64(m.TimeIotaMs))
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func (m *ValidatorParams) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.PubKeyTypes) > 0 {
		for _, s := range m.PubKeyTypes {
			l = len(s)
			n += 1 + l + sovParams(uint64(l))
		}
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func (m *HashedParams) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.BlockMaxBytes != 0 {
		n += 1 + sovParams(uint64(m.BlockMaxBytes))
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func sovParams(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozParams(x uint64) (n int) {
	return sovParams(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *ConsensusParams) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowParams
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
			return fmt.Errorf("proto: ConsensusParams: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: ConsensusParams: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Block", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowParams
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
				return ErrInvalidLengthParams
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthParams
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := m.Block.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Validator", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowParams
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
				return ErrInvalidLengthParams
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthParams
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := m.Validator.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipParams(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthParams
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthParams
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
func (m *BlockParams) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowParams
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
			return fmt.Errorf("proto: BlockParams: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: BlockParams: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field MaxBytes", wireType)
			}
			m.MaxBytes = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowParams
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.MaxBytes |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field TimeIotaMs", wireType)
			}
			m.TimeIotaMs = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowParams
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.TimeIotaMs |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipParams(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthParams
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthParams
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
func (m *ValidatorParams) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowParams
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
			return fmt.Errorf("proto: ValidatorParams: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: ValidatorParams: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field PubKeyTypes", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowParams
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthParams
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthParams
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.PubKeyTypes = append(m.PubKeyTypes, string(dAtA[iNdEx:postIndex]))
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipParams(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthParams
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthParams
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
func (m *HashedParams) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowParams
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
			return fmt.Errorf("proto: HashedParams: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: HashedParams: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field BlockMaxBytes", wireType)
			}
			m.BlockMaxBytes = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowParams
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.BlockMaxBytes |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipParams(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthParams
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthParams
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
func skipParams(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowParams
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
					return 0, ErrIntOverflowParams
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
					return 0, ErrIntOverflowParams
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
				return 0, ErrInvalidLengthParams
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupParams
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthParams
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthParams        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowParams          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupParams = fmt.Errorf("proto: unexpected end of group")
)
