// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: crtrust.proto

package crtrust

import (
	encoding_binary "encoding/binary"
	fmt "fmt"
	proto "github.com/gogo/protobuf/proto"
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
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

type Notice struct {
	Src    string `protobuf:"bytes,2,opt,name=Src,proto3" json:"Src,omitempty"`
	Height int64  `protobuf:"varint,3,opt,name=Height,proto3" json:"Height,omitempty"`
}

func (m *Notice) Reset()         { *m = Notice{} }
func (m *Notice) String() string { return proto.CompactTextString(m) }
func (*Notice) ProtoMessage()    {}
func (*Notice) Descriptor() ([]byte, []int) {
	return fileDescriptor_41dff6735b53e2c7, []int{0}
}
func (m *Notice) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Notice) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Notice.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Notice) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Notice.Merge(m, src)
}
func (m *Notice) XXX_Size() int {
	return m.Size()
}
func (m *Notice) XXX_DiscardUnknown() {
	xxx_messageInfo_Notice.DiscardUnknown(m)
}

var xxx_messageInfo_Notice proto.InternalMessageInfo

func (m *Notice) GetSrc() string {
	if m != nil {
		return m.Src
	}
	return ""
}

func (m *Notice) GetHeight() int64 {
	if m != nil {
		return m.Height
	}
	return 0
}

type LocalEvaluation struct {
	PeerEval map[string]float64 `protobuf:"bytes,1,rep,name=PeerEval,proto3" json:"PeerEval,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"fixed64,2,opt,name=value,proto3"`
	Src      string             `protobuf:"bytes,2,opt,name=Src,proto3" json:"Src,omitempty"`
	Height   int64              `protobuf:"varint,3,opt,name=Height,proto3" json:"Height,omitempty"`
}

func (m *LocalEvaluation) Reset()         { *m = LocalEvaluation{} }
func (m *LocalEvaluation) String() string { return proto.CompactTextString(m) }
func (*LocalEvaluation) ProtoMessage()    {}
func (*LocalEvaluation) Descriptor() ([]byte, []int) {
	return fileDescriptor_41dff6735b53e2c7, []int{1}
}
func (m *LocalEvaluation) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *LocalEvaluation) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_LocalEvaluation.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *LocalEvaluation) XXX_Merge(src proto.Message) {
	xxx_messageInfo_LocalEvaluation.Merge(m, src)
}
func (m *LocalEvaluation) XXX_Size() int {
	return m.Size()
}
func (m *LocalEvaluation) XXX_DiscardUnknown() {
	xxx_messageInfo_LocalEvaluation.DiscardUnknown(m)
}

var xxx_messageInfo_LocalEvaluation proto.InternalMessageInfo

func (m *LocalEvaluation) GetPeerEval() map[string]float64 {
	if m != nil {
		return m.PeerEval
	}
	return nil
}

func (m *LocalEvaluation) GetSrc() string {
	if m != nil {
		return m.Src
	}
	return ""
}

func (m *LocalEvaluation) GetHeight() int64 {
	if m != nil {
		return m.Height
	}
	return 0
}

func init() {
	proto.RegisterType((*Notice)(nil), "crtrust.Notice")
	proto.RegisterType((*LocalEvaluation)(nil), "crtrust.LocalEvaluation")
	proto.RegisterMapType((map[string]float64)(nil), "crtrust.LocalEvaluation.PeerEvalEntry")
}

func init() { proto.RegisterFile("crtrust.proto", fileDescriptor_41dff6735b53e2c7) }

var fileDescriptor_41dff6735b53e2c7 = []byte{
	// 212 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0x4d, 0x2e, 0x2a, 0x29,
	0x2a, 0x2d, 0x2e, 0xd1, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x62, 0x87, 0x72, 0x95, 0x8c, 0xb8,
	0xd8, 0xfc, 0xf2, 0x4b, 0x32, 0x93, 0x53, 0x85, 0x04, 0xb8, 0x98, 0x83, 0x8b, 0x92, 0x25, 0x98,
	0x14, 0x18, 0x35, 0x38, 0x83, 0x40, 0x4c, 0x21, 0x31, 0x2e, 0x36, 0x8f, 0xd4, 0xcc, 0xf4, 0x8c,
	0x12, 0x09, 0x66, 0x05, 0x46, 0x0d, 0xe6, 0x20, 0x28, 0x4f, 0x69, 0x0f, 0x23, 0x17, 0xbf, 0x4f,
	0x7e, 0x72, 0x62, 0x8e, 0x6b, 0x59, 0x62, 0x4e, 0x69, 0x62, 0x49, 0x66, 0x7e, 0x9e, 0x90, 0x13,
	0x17, 0x47, 0x40, 0x6a, 0x6a, 0x11, 0x48, 0x44, 0x82, 0x51, 0x81, 0x59, 0x83, 0xdb, 0x48, 0x4d,
	0x0f, 0x66, 0x25, 0x9a, 0x5a, 0x3d, 0x98, 0x42, 0xd7, 0xbc, 0x92, 0xa2, 0xca, 0x20, 0xb8, 0x3e,
	0xe2, 0x5d, 0x20, 0x65, 0xcd, 0xc5, 0x8b, 0x62, 0x08, 0x48, 0x6b, 0x76, 0x6a, 0xa5, 0x04, 0x23,
	0x44, 0x6b, 0x76, 0x6a, 0xa5, 0x90, 0x08, 0x17, 0x2b, 0xc8, 0xc6, 0x54, 0xb0, 0x71, 0x8c, 0x41,
	0x10, 0x8e, 0x15, 0x93, 0x05, 0xa3, 0x93, 0xc4, 0x89, 0x47, 0x72, 0x8c, 0x17, 0x1e, 0xc9, 0x31,
	0x3e, 0x78, 0x24, 0xc7, 0x38, 0xe1, 0xb1, 0x1c, 0xc3, 0x85, 0xc7, 0x72, 0x0c, 0x37, 0x1e, 0xcb,
	0x31, 0x24, 0xb1, 0x81, 0x03, 0xc7, 0x18, 0x10, 0x00, 0x00, 0xff, 0xff, 0x0d, 0xd5, 0xfe, 0x95,
	0x2d, 0x01, 0x00, 0x00,
}

func (m *Notice) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Notice) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Notice) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.Height != 0 {
		i = encodeVarintCrtrust(dAtA, i, uint64(m.Height))
		i--
		dAtA[i] = 0x18
	}
	if len(m.Src) > 0 {
		i -= len(m.Src)
		copy(dAtA[i:], m.Src)
		i = encodeVarintCrtrust(dAtA, i, uint64(len(m.Src)))
		i--
		dAtA[i] = 0x12
	}
	return len(dAtA) - i, nil
}

func (m *LocalEvaluation) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *LocalEvaluation) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *LocalEvaluation) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.Height != 0 {
		i = encodeVarintCrtrust(dAtA, i, uint64(m.Height))
		i--
		dAtA[i] = 0x18
	}
	if len(m.Src) > 0 {
		i -= len(m.Src)
		copy(dAtA[i:], m.Src)
		i = encodeVarintCrtrust(dAtA, i, uint64(len(m.Src)))
		i--
		dAtA[i] = 0x12
	}
	if len(m.PeerEval) > 0 {
		for k := range m.PeerEval {
			v := m.PeerEval[k]
			baseI := i
			i -= 8
			encoding_binary.LittleEndian.PutUint64(dAtA[i:], uint64(math.Float64bits(float64(v))))
			i--
			dAtA[i] = 0x11
			i -= len(k)
			copy(dAtA[i:], k)
			i = encodeVarintCrtrust(dAtA, i, uint64(len(k)))
			i--
			dAtA[i] = 0xa
			i = encodeVarintCrtrust(dAtA, i, uint64(baseI-i))
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func encodeVarintCrtrust(dAtA []byte, offset int, v uint64) int {
	offset -= sovCrtrust(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *Notice) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.Src)
	if l > 0 {
		n += 1 + l + sovCrtrust(uint64(l))
	}
	if m.Height != 0 {
		n += 1 + sovCrtrust(uint64(m.Height))
	}
	return n
}

func (m *LocalEvaluation) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.PeerEval) > 0 {
		for k, v := range m.PeerEval {
			_ = k
			_ = v
			mapEntrySize := 1 + len(k) + sovCrtrust(uint64(len(k))) + 1 + 8
			n += mapEntrySize + 1 + sovCrtrust(uint64(mapEntrySize))
		}
	}
	l = len(m.Src)
	if l > 0 {
		n += 1 + l + sovCrtrust(uint64(l))
	}
	if m.Height != 0 {
		n += 1 + sovCrtrust(uint64(m.Height))
	}
	return n
}

func sovCrtrust(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozCrtrust(x uint64) (n int) {
	return sovCrtrust(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *Notice) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowCrtrust
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
			return fmt.Errorf("proto: Notice: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Notice: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Src", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowCrtrust
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
				return ErrInvalidLengthCrtrust
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthCrtrust
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Src = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Height", wireType)
			}
			m.Height = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowCrtrust
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
			skippy, err := skipCrtrust(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthCrtrust
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *LocalEvaluation) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowCrtrust
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
			return fmt.Errorf("proto: LocalEvaluation: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: LocalEvaluation: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field PeerEval", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowCrtrust
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
				return ErrInvalidLengthCrtrust
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthCrtrust
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.PeerEval == nil {
				m.PeerEval = make(map[string]float64)
			}
			var mapkey string
			var mapvalue float64
			for iNdEx < postIndex {
				entryPreIndex := iNdEx
				var wire uint64
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return ErrIntOverflowCrtrust
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
				if fieldNum == 1 {
					var stringLenmapkey uint64
					for shift := uint(0); ; shift += 7 {
						if shift >= 64 {
							return ErrIntOverflowCrtrust
						}
						if iNdEx >= l {
							return io.ErrUnexpectedEOF
						}
						b := dAtA[iNdEx]
						iNdEx++
						stringLenmapkey |= uint64(b&0x7F) << shift
						if b < 0x80 {
							break
						}
					}
					intStringLenmapkey := int(stringLenmapkey)
					if intStringLenmapkey < 0 {
						return ErrInvalidLengthCrtrust
					}
					postStringIndexmapkey := iNdEx + intStringLenmapkey
					if postStringIndexmapkey < 0 {
						return ErrInvalidLengthCrtrust
					}
					if postStringIndexmapkey > l {
						return io.ErrUnexpectedEOF
					}
					mapkey = string(dAtA[iNdEx:postStringIndexmapkey])
					iNdEx = postStringIndexmapkey
				} else if fieldNum == 2 {
					var mapvaluetemp uint64
					if (iNdEx + 8) > l {
						return io.ErrUnexpectedEOF
					}
					mapvaluetemp = uint64(encoding_binary.LittleEndian.Uint64(dAtA[iNdEx:]))
					iNdEx += 8
					mapvalue = math.Float64frombits(mapvaluetemp)
				} else {
					iNdEx = entryPreIndex
					skippy, err := skipCrtrust(dAtA[iNdEx:])
					if err != nil {
						return err
					}
					if (skippy < 0) || (iNdEx+skippy) < 0 {
						return ErrInvalidLengthCrtrust
					}
					if (iNdEx + skippy) > postIndex {
						return io.ErrUnexpectedEOF
					}
					iNdEx += skippy
				}
			}
			m.PeerEval[mapkey] = mapvalue
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Src", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowCrtrust
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
				return ErrInvalidLengthCrtrust
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthCrtrust
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Src = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Height", wireType)
			}
			m.Height = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowCrtrust
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
			skippy, err := skipCrtrust(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthCrtrust
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipCrtrust(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowCrtrust
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
					return 0, ErrIntOverflowCrtrust
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
					return 0, ErrIntOverflowCrtrust
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
				return 0, ErrInvalidLengthCrtrust
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupCrtrust
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthCrtrust
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthCrtrust        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowCrtrust          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupCrtrust = fmt.Errorf("proto: unexpected end of group")
)
