// Code generated by protoc-gen-go. DO NOT EDIT.
// source: protocol.proto

package internal

import (
	fmt "fmt"
	math "math"

	proto "github.com/golang/protobuf/proto"
	v11 "go.temporal.io/api/common/v1"
	v1 "go.temporal.io/api/failure/v1"
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

type Frame struct {
	Messages             []*Message `protobuf:"bytes,1,rep,name=messages,proto3" json:"messages,omitempty"`
	XXX_NoUnkeyedLiteral struct{}   `json:"-"`
	XXX_unrecognized     []byte     `json:"-"`
	XXX_sizecache        int32      `json:"-"`
}

func (m *Frame) Reset()         { *m = Frame{} }
func (m *Frame) String() string { return proto.CompactTextString(m) }
func (*Frame) ProtoMessage()    {}
func (*Frame) Descriptor() ([]byte, []int) {
	return fileDescriptor_2bc2336598a3f7e0, []int{0}
}

func (m *Frame) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Frame.Unmarshal(m, b)
}
func (m *Frame) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Frame.Marshal(b, m, deterministic)
}
func (m *Frame) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Frame.Merge(m, src)
}
func (m *Frame) XXX_Size() int {
	return xxx_messageInfo_Frame.Size(m)
}
func (m *Frame) XXX_DiscardUnknown() {
	xxx_messageInfo_Frame.DiscardUnknown(m)
}

var xxx_messageInfo_Frame proto.InternalMessageInfo

func (m *Frame) GetMessages() []*Message {
	if m != nil {
		return m.Messages
	}
	return nil
}

// Single communication message.
type Message struct {
	Id uint64 `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
	// command name (if any)
	Command string `protobuf:"bytes,2,opt,name=command,proto3" json:"command,omitempty"`
	// command options in json format.
	Options []byte `protobuf:"bytes,3,opt,name=options,proto3" json:"options,omitempty"`
	// error response.
	Failure *v1.Failure `protobuf:"bytes,4,opt,name=failure,proto3" json:"failure,omitempty"`
	// invocation or result payloads.
	Payloads             *v11.Payloads `protobuf:"bytes,5,opt,name=payloads,proto3" json:"payloads,omitempty"`
	XXX_NoUnkeyedLiteral struct{}      `json:"-"`
	XXX_unrecognized     []byte        `json:"-"`
	XXX_sizecache        int32         `json:"-"`
}

func (m *Message) Reset()         { *m = Message{} }
func (m *Message) String() string { return proto.CompactTextString(m) }
func (*Message) ProtoMessage()    {}
func (*Message) Descriptor() ([]byte, []int) {
	return fileDescriptor_2bc2336598a3f7e0, []int{1}
}

func (m *Message) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Message.Unmarshal(m, b)
}
func (m *Message) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Message.Marshal(b, m, deterministic)
}
func (m *Message) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Message.Merge(m, src)
}
func (m *Message) XXX_Size() int {
	return xxx_messageInfo_Message.Size(m)
}
func (m *Message) XXX_DiscardUnknown() {
	xxx_messageInfo_Message.DiscardUnknown(m)
}

var xxx_messageInfo_Message proto.InternalMessageInfo

func (m *Message) GetId() uint64 {
	if m != nil {
		return m.Id
	}
	return 0
}

func (m *Message) GetCommand() string {
	if m != nil {
		return m.Command
	}
	return ""
}

func (m *Message) GetOptions() []byte {
	if m != nil {
		return m.Options
	}
	return nil
}

func (m *Message) GetFailure() *v1.Failure {
	if m != nil {
		return m.Failure
	}
	return nil
}

func (m *Message) GetPayloads() *v11.Payloads {
	if m != nil {
		return m.Payloads
	}
	return nil
}

func init() {
	proto.RegisterType((*Frame)(nil), "temporal.roadrunner.internal.Frame")
	proto.RegisterType((*Message)(nil), "temporal.roadrunner.internal.Message")
}

func init() { proto.RegisterFile("protocol.proto", fileDescriptor_2bc2336598a3f7e0) }

var fileDescriptor_2bc2336598a3f7e0 = []byte{
	// 257 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x7c, 0x90, 0x41, 0x4b, 0x33, 0x31,
	0x10, 0x86, 0xc9, 0xb6, 0xfd, 0xb6, 0x5f, 0x2a, 0x3d, 0xe4, 0x14, 0x8a, 0x87, 0x50, 0x2c, 0xec,
	0x29, 0x4b, 0xeb, 0x4d, 0xbc, 0x78, 0xe9, 0x41, 0x10, 0x24, 0xff, 0x60, 0x6c, 0xa2, 0x04, 0x92,
	0x4c, 0x48, 0xb6, 0x05, 0x7f, 0xa1, 0x7f, 0x4b, 0xb6, 0x9b, 0xad, 0x88, 0xe2, 0x6d, 0x26, 0xef,
	0xfb, 0x84, 0x87, 0xa1, 0xcb, 0x98, 0xb0, 0xc3, 0x03, 0x3a, 0x79, 0x1e, 0xd8, 0x75, 0x67, 0x7c,
	0xc4, 0x04, 0x4e, 0x26, 0x04, 0x9d, 0x8e, 0x21, 0x98, 0x24, 0x6d, 0xe8, 0x4c, 0x0a, 0xe0, 0x56,
	0x37, 0x63, 0xda, 0x42, 0xb4, 0xed, 0x01, 0xbd, 0xc7, 0xd0, 0x9e, 0xb6, 0xad, 0x37, 0x39, 0xc3,
	0x9b, 0x19, 0xfe, 0x58, 0x6d, 0xbe, 0xb5, 0x5e, 0xc1, 0xba, 0x63, 0x32, 0x3f, 0x6a, 0xeb, 0x47,
	0x3a, 0xdb, 0x27, 0xf0, 0x86, 0x3d, 0xd0, 0x79, 0x49, 0x32, 0x27, 0x62, 0xd2, 0x2c, 0x76, 0x1b,
	0xf9, 0x97, 0x86, 0x7c, 0x1a, 0xda, 0xea, 0x82, 0xad, 0x3f, 0x08, 0xad, 0xcb, 0x2b, 0x5b, 0xd2,
	0xca, 0x6a, 0x4e, 0x04, 0x69, 0xa6, 0xaa, 0xb2, 0x9a, 0x71, 0x5a, 0xf7, 0xa6, 0x10, 0x34, 0xaf,
	0x04, 0x69, 0xfe, 0xab, 0x71, 0xed, 0x13, 0x8c, 0x9d, 0xc5, 0x90, 0xf9, 0x44, 0x90, 0xe6, 0x4a,
	0x8d, 0x2b, 0xbb, 0xa3, 0x75, 0xf1, 0xe6, 0x53, 0x41, 0x9a, 0xc5, 0x4e, 0x7c, 0x19, 0x41, 0xb4,
	0xb2, 0x84, 0xf2, 0xb4, 0x95, 0xfb, 0x61, 0x54, 0x23, 0xc0, 0xee, 0xe9, 0x3c, 0xc2, 0xbb, 0x43,
	0xd0, 0x99, 0xcf, 0x7e, 0x83, 0x87, 0xbb, 0xf5, 0xec, 0x73, 0xe9, 0xa9, 0x0b, 0xf1, 0xf2, 0xef,
	0x7c, 0x9c, 0xdb, 0xcf, 0x00, 0x00, 0x00, 0xff, 0xff, 0x59, 0xb1, 0x79, 0xd4, 0x99, 0x01, 0x00,
	0x00,
}
