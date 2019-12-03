// Code generated by protoc-gen-go. DO NOT EDIT.
// source: proto/message.proto

package proto

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	math "math"
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

type ConnectionRequest struct {
	// protocol version number
	Version uint32 `protobuf:"varint,1,opt,name=version,proto3" json:"version,omitempty"`
	// string form of the return address (e.g. "192.0.2.1:25", "[2001:db8::1]:80")
	From string `protobuf:"bytes,2,opt,name=from,proto3" json:"from,omitempty"`
	// string form of the recipient address
	To string `protobuf:"bytes,3,opt,name=to,proto3" json:"to,omitempty"`
	// unix time
	Timestamp            int64    `protobuf:"varint,4,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ConnectionRequest) Reset()         { *m = ConnectionRequest{} }
func (m *ConnectionRequest) String() string { return proto.CompactTextString(m) }
func (*ConnectionRequest) ProtoMessage()    {}
func (*ConnectionRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_33f3a5e1293a7bcd, []int{0}
}

func (m *ConnectionRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ConnectionRequest.Unmarshal(m, b)
}
func (m *ConnectionRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ConnectionRequest.Marshal(b, m, deterministic)
}
func (m *ConnectionRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ConnectionRequest.Merge(m, src)
}
func (m *ConnectionRequest) XXX_Size() int {
	return xxx_messageInfo_ConnectionRequest.Size(m)
}
func (m *ConnectionRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_ConnectionRequest.DiscardUnknown(m)
}

var xxx_messageInfo_ConnectionRequest proto.InternalMessageInfo

func (m *ConnectionRequest) GetVersion() uint32 {
	if m != nil {
		return m.Version
	}
	return 0
}

func (m *ConnectionRequest) GetFrom() string {
	if m != nil {
		return m.From
	}
	return ""
}

func (m *ConnectionRequest) GetTo() string {
	if m != nil {
		return m.To
	}
	return ""
}

func (m *ConnectionRequest) GetTimestamp() int64 {
	if m != nil {
		return m.Timestamp
	}
	return 0
}

type SignedConnectionRequest struct {
	// data of the request
	Request *ConnectionRequest `protobuf:"bytes,1,opt,name=request,proto3" json:"request,omitempty"`
	// signature of the request
	Signature            []byte   `protobuf:"bytes,2,opt,name=signature,proto3" json:"signature,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *SignedConnectionRequest) Reset()         { *m = SignedConnectionRequest{} }
func (m *SignedConnectionRequest) String() string { return proto.CompactTextString(m) }
func (*SignedConnectionRequest) ProtoMessage()    {}
func (*SignedConnectionRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_33f3a5e1293a7bcd, []int{1}
}

func (m *SignedConnectionRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_SignedConnectionRequest.Unmarshal(m, b)
}
func (m *SignedConnectionRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_SignedConnectionRequest.Marshal(b, m, deterministic)
}
func (m *SignedConnectionRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SignedConnectionRequest.Merge(m, src)
}
func (m *SignedConnectionRequest) XXX_Size() int {
	return xxx_messageInfo_SignedConnectionRequest.Size(m)
}
func (m *SignedConnectionRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_SignedConnectionRequest.DiscardUnknown(m)
}

var xxx_messageInfo_SignedConnectionRequest proto.InternalMessageInfo

func (m *SignedConnectionRequest) GetRequest() *ConnectionRequest {
	if m != nil {
		return m.Request
	}
	return nil
}

func (m *SignedConnectionRequest) GetSignature() []byte {
	if m != nil {
		return m.Signature
	}
	return nil
}

func init() {
	proto.RegisterType((*ConnectionRequest)(nil), "proto.ConnectionRequest")
	proto.RegisterType((*SignedConnectionRequest)(nil), "proto.SignedConnectionRequest")
}

func init() { proto.RegisterFile("proto/message.proto", fileDescriptor_33f3a5e1293a7bcd) }

var fileDescriptor_33f3a5e1293a7bcd = []byte{
	// 214 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x64, 0x8e, 0xbd, 0x4e, 0x03, 0x31,
	0x10, 0x84, 0x75, 0x97, 0x40, 0x94, 0xe5, 0x47, 0xc2, 0x14, 0xb8, 0xa0, 0x38, 0x42, 0x73, 0xd5,
	0x9d, 0x14, 0xde, 0x00, 0xde, 0xc0, 0x74, 0x74, 0xce, 0xb1, 0x18, 0x0b, 0xec, 0x35, 0xde, 0x3d,
	0x9e, 0x1f, 0xb1, 0x51, 0x94, 0x22, 0xd5, 0xce, 0x8c, 0x46, 0xfb, 0x0d, 0xdc, 0x96, 0x4a, 0x42,
	0x63, 0x42, 0x66, 0x1f, 0x70, 0x50, 0x67, 0xce, 0xf4, 0x6c, 0x08, 0x6e, 0x5e, 0x28, 0x67, 0x9c,
	0x24, 0x52, 0x76, 0xf8, 0x33, 0x23, 0x8b, 0xb1, 0xb0, 0xfa, 0xc5, 0xca, 0x91, 0xb2, 0x6d, 0xba,
	0xa6, 0xbf, 0x72, 0x07, 0x6b, 0x0c, 0x2c, 0x3f, 0x2a, 0x25, 0xdb, 0x76, 0x4d, 0xbf, 0x76, 0xaa,
	0xcd, 0x35, 0xb4, 0x42, 0x76, 0xa1, 0x49, 0x2b, 0x64, 0xee, 0x61, 0x2d, 0x31, 0x21, 0x8b, 0x4f,
	0xc5, 0x2e, 0xbb, 0xa6, 0x5f, 0xb8, 0x63, 0xb0, 0xf9, 0x82, 0xbb, 0xd7, 0x18, 0x32, 0xbe, 0x9f,
	0x62, 0xb7, 0xb0, 0xaa, 0x7b, 0xa9, 0xd8, 0x8b, 0xad, 0xdd, 0x6f, 0x1d, 0x4e, 0xaa, 0xee, 0x50,
	0xfc, 0x87, 0x71, 0x0c, 0xd9, 0xcb, 0x5c, 0x51, 0x57, 0x5d, 0xba, 0x63, 0xf0, 0xfc, 0xf8, 0xf6,
	0x10, 0xa2, 0x7c, 0xce, 0xbb, 0x61, 0xa2, 0x34, 0x4e, 0xbe, 0x10, 0x33, 0x7e, 0xe3, 0x18, 0x88,
	0x39, 0x96, 0x51, 0xbf, 0xef, 0xce, 0xf5, 0x3c, 0xfd, 0x05, 0x00, 0x00, 0xff, 0xff, 0x16, 0x07,
	0x40, 0xfc, 0x27, 0x01, 0x00, 0x00,
}
