// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: store_types.proto

package store

import (
	fmt "fmt"
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

type BlockStoreState struct {
	Base                 int64    `protobuf:"varint,1,opt,name=base,proto3" json:"base,omitempty"`
	Height               int64    `protobuf:"varint,2,opt,name=height,proto3" json:"height,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *BlockStoreState) Reset()         { *m = BlockStoreState{} }
func (m *BlockStoreState) String() string { return proto.CompactTextString(m) }
func (*BlockStoreState) ProtoMessage()    {}
func (*BlockStoreState) Descriptor() ([]byte, []int) {
	return fileDescriptor_eef78dd516380b41, []int{0}
}
func (m *BlockStoreState) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *BlockStoreState) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_BlockStoreState.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *BlockStoreState) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BlockStoreState.Merge(m, src)
}
func (m *BlockStoreState) XXX_Size() int {
	return m.Size()
}
func (m *BlockStoreState) XXX_DiscardUnknown() {
	xxx_messageInfo_BlockStoreState.DiscardUnknown(m)
}

var xxx_messageInfo_BlockStoreState proto.InternalMessageInfo

func (m *BlockStoreState) GetBase() int64 {
	if m != nil {
		return m.Base
	}
	return 0
}

func (m *BlockStoreState) GetHeight() int64 {
	if m != nil {
		return m.Height
	}
	return 0
}

func init() {
	proto.RegisterType((*BlockStoreState)(nil), "store.BlockStoreState")
}

func init() { proto.RegisterFile("store_types.proto", fileDescriptor_eef78dd516380b41) }

var fileDescriptor_eef78dd516380b41 = []byte{
	// 120 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x12, 0x2c, 0x2e, 0xc9, 0x2f,
	0x4a, 0x8d, 0x2f, 0xa9, 0x2c, 0x48, 0x2d, 0xd6, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x62, 0x05,
	0x0b, 0x29, 0xd9, 0x72, 0xf1, 0x3b, 0xe5, 0xe4, 0x27, 0x67, 0x07, 0x83, 0x78, 0xc1, 0x25, 0x89,
	0x25, 0xa9, 0x42, 0x42, 0x5c, 0x2c, 0x49, 0x89, 0xc5, 0xa9, 0x12, 0x8c, 0x0a, 0x8c, 0x1a, 0xcc,
	0x41, 0x60, 0xb6, 0x90, 0x18, 0x17, 0x5b, 0x46, 0x6a, 0x66, 0x7a, 0x46, 0x89, 0x04, 0x13, 0x58,
	0x14, 0xca, 0x73, 0x12, 0x38, 0xf1, 0x48, 0x8e, 0xf1, 0xc2, 0x23, 0x39, 0xc6, 0x07, 0x8f, 0xe4,
	0x18, 0x67, 0x3c, 0x96, 0x63, 0x48, 0x62, 0x03, 0x1b, 0x6f, 0x0c, 0x08, 0x00, 0x00, 0xff, 0xff,
	0x38, 0x13, 0xf4, 0xe9, 0x73, 0x00, 0x00, 0x00,
}

func (m *BlockStoreState) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *BlockStoreState) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *BlockStoreState) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if m.Height != 0 {
		i = encodeVarintStoreTypes(dAtA, i, uint64(m.Height))
		i--
		dAtA[i] = 0x10
	}
	if m.Base != 0 {
		i = encodeVarintStoreTypes(dAtA, i, uint64(m.Base))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func encodeVarintStoreTypes(dAtA []byte, offset int, v uint64) int {
	offset -= sovStoreTypes(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *BlockStoreState) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Base != 0 {
		n += 1 + sovStoreTypes(uint64(m.Base))
	}
	if m.Height != 0 {
		n += 1 + sovStoreTypes(uint64(m.Height))
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func sovStoreTypes(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozStoreTypes(x uint64) (n int) {
	return sovStoreTypes(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *BlockStoreState) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowStoreTypes
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
			return fmt.Errorf("proto: BlockStoreState: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: BlockStoreState: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Base", wireType)
			}
			m.Base = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowStoreTypes
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Base |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Height", wireType)
			}
			m.Height = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowStoreTypes
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Height |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipStoreTypes(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthStoreTypes
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthStoreTypes
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
func skipStoreTypes(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowStoreTypes
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
					return 0, ErrIntOverflowStoreTypes
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
					return 0, ErrIntOverflowStoreTypes
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
				return 0, ErrInvalidLengthStoreTypes
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupStoreTypes
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthStoreTypes
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthStoreTypes        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowStoreTypes          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupStoreTypes = fmt.Errorf("proto: unexpected end of group")
)
