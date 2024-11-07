// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        (unknown)
// source: openmatch/frontend.proto

package openmatch

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type CreateTicketRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Ticket *Ticket `protobuf:"bytes,1,opt,name=ticket,proto3" json:"ticket,omitempty"`
}

func (x *CreateTicketRequest) Reset() {
	*x = CreateTicketRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_openmatch_frontend_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CreateTicketRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateTicketRequest) ProtoMessage() {}

func (x *CreateTicketRequest) ProtoReflect() protoreflect.Message {
	mi := &file_openmatch_frontend_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateTicketRequest.ProtoReflect.Descriptor instead.
func (*CreateTicketRequest) Descriptor() ([]byte, []int) {
	return file_openmatch_frontend_proto_rawDescGZIP(), []int{0}
}

func (x *CreateTicketRequest) GetTicket() *Ticket {
	if x != nil {
		return x.Ticket
	}
	return nil
}

type DeleteTicketRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	TicketId string `protobuf:"bytes,1,opt,name=ticket_id,json=ticketId,proto3" json:"ticket_id,omitempty"`
}

func (x *DeleteTicketRequest) Reset() {
	*x = DeleteTicketRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_openmatch_frontend_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DeleteTicketRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DeleteTicketRequest) ProtoMessage() {}

func (x *DeleteTicketRequest) ProtoReflect() protoreflect.Message {
	mi := &file_openmatch_frontend_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DeleteTicketRequest.ProtoReflect.Descriptor instead.
func (*DeleteTicketRequest) Descriptor() ([]byte, []int) {
	return file_openmatch_frontend_proto_rawDescGZIP(), []int{1}
}

func (x *DeleteTicketRequest) GetTicketId() string {
	if x != nil {
		return x.TicketId
	}
	return ""
}

type GetTicketRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	TicketId string `protobuf:"bytes,1,opt,name=ticket_id,json=ticketId,proto3" json:"ticket_id,omitempty"`
}

func (x *GetTicketRequest) Reset() {
	*x = GetTicketRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_openmatch_frontend_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetTicketRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetTicketRequest) ProtoMessage() {}

func (x *GetTicketRequest) ProtoReflect() protoreflect.Message {
	mi := &file_openmatch_frontend_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetTicketRequest.ProtoReflect.Descriptor instead.
func (*GetTicketRequest) Descriptor() ([]byte, []int) {
	return file_openmatch_frontend_proto_rawDescGZIP(), []int{2}
}

func (x *GetTicketRequest) GetTicketId() string {
	if x != nil {
		return x.TicketId
	}
	return ""
}

type WatchAssignmentsRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	TicketId string `protobuf:"bytes,1,opt,name=ticket_id,json=ticketId,proto3" json:"ticket_id,omitempty"`
}

func (x *WatchAssignmentsRequest) Reset() {
	*x = WatchAssignmentsRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_openmatch_frontend_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *WatchAssignmentsRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*WatchAssignmentsRequest) ProtoMessage() {}

func (x *WatchAssignmentsRequest) ProtoReflect() protoreflect.Message {
	mi := &file_openmatch_frontend_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use WatchAssignmentsRequest.ProtoReflect.Descriptor instead.
func (*WatchAssignmentsRequest) Descriptor() ([]byte, []int) {
	return file_openmatch_frontend_proto_rawDescGZIP(), []int{3}
}

func (x *WatchAssignmentsRequest) GetTicketId() string {
	if x != nil {
		return x.TicketId
	}
	return ""
}

type WatchAssignmentsResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Assignment *Assignment `protobuf:"bytes,1,opt,name=assignment,proto3" json:"assignment,omitempty"`
}

func (x *WatchAssignmentsResponse) Reset() {
	*x = WatchAssignmentsResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_openmatch_frontend_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *WatchAssignmentsResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*WatchAssignmentsResponse) ProtoMessage() {}

func (x *WatchAssignmentsResponse) ProtoReflect() protoreflect.Message {
	mi := &file_openmatch_frontend_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use WatchAssignmentsResponse.ProtoReflect.Descriptor instead.
func (*WatchAssignmentsResponse) Descriptor() ([]byte, []int) {
	return file_openmatch_frontend_proto_rawDescGZIP(), []int{4}
}

func (x *WatchAssignmentsResponse) GetAssignment() *Assignment {
	if x != nil {
		return x.Assignment
	}
	return nil
}

type AcknowledgeBackfillRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	BackfillId string      `protobuf:"bytes,1,opt,name=backfill_id,json=backfillId,proto3" json:"backfill_id,omitempty"`
	Assignment *Assignment `protobuf:"bytes,2,opt,name=assignment,proto3" json:"assignment,omitempty"`
}

func (x *AcknowledgeBackfillRequest) Reset() {
	*x = AcknowledgeBackfillRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_openmatch_frontend_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AcknowledgeBackfillRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AcknowledgeBackfillRequest) ProtoMessage() {}

func (x *AcknowledgeBackfillRequest) ProtoReflect() protoreflect.Message {
	mi := &file_openmatch_frontend_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AcknowledgeBackfillRequest.ProtoReflect.Descriptor instead.
func (*AcknowledgeBackfillRequest) Descriptor() ([]byte, []int) {
	return file_openmatch_frontend_proto_rawDescGZIP(), []int{5}
}

func (x *AcknowledgeBackfillRequest) GetBackfillId() string {
	if x != nil {
		return x.BackfillId
	}
	return ""
}

func (x *AcknowledgeBackfillRequest) GetAssignment() *Assignment {
	if x != nil {
		return x.Assignment
	}
	return nil
}

type AcknowledgeBackfillResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Backfill *Backfill `protobuf:"bytes,1,opt,name=backfill,proto3" json:"backfill,omitempty"`
	Tickets  []*Ticket `protobuf:"bytes,2,rep,name=tickets,proto3" json:"tickets,omitempty"`
}

func (x *AcknowledgeBackfillResponse) Reset() {
	*x = AcknowledgeBackfillResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_openmatch_frontend_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AcknowledgeBackfillResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AcknowledgeBackfillResponse) ProtoMessage() {}

func (x *AcknowledgeBackfillResponse) ProtoReflect() protoreflect.Message {
	mi := &file_openmatch_frontend_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AcknowledgeBackfillResponse.ProtoReflect.Descriptor instead.
func (*AcknowledgeBackfillResponse) Descriptor() ([]byte, []int) {
	return file_openmatch_frontend_proto_rawDescGZIP(), []int{6}
}

func (x *AcknowledgeBackfillResponse) GetBackfill() *Backfill {
	if x != nil {
		return x.Backfill
	}
	return nil
}

func (x *AcknowledgeBackfillResponse) GetTickets() []*Ticket {
	if x != nil {
		return x.Tickets
	}
	return nil
}

type CreateBackfillRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Backfill *Backfill `protobuf:"bytes,1,opt,name=backfill,proto3" json:"backfill,omitempty"`
}

func (x *CreateBackfillRequest) Reset() {
	*x = CreateBackfillRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_openmatch_frontend_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CreateBackfillRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateBackfillRequest) ProtoMessage() {}

func (x *CreateBackfillRequest) ProtoReflect() protoreflect.Message {
	mi := &file_openmatch_frontend_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateBackfillRequest.ProtoReflect.Descriptor instead.
func (*CreateBackfillRequest) Descriptor() ([]byte, []int) {
	return file_openmatch_frontend_proto_rawDescGZIP(), []int{7}
}

func (x *CreateBackfillRequest) GetBackfill() *Backfill {
	if x != nil {
		return x.Backfill
	}
	return nil
}

type DeleteBackfillRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	BackfillId string `protobuf:"bytes,1,opt,name=backfill_id,json=backfillId,proto3" json:"backfill_id,omitempty"`
}

func (x *DeleteBackfillRequest) Reset() {
	*x = DeleteBackfillRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_openmatch_frontend_proto_msgTypes[8]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DeleteBackfillRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DeleteBackfillRequest) ProtoMessage() {}

func (x *DeleteBackfillRequest) ProtoReflect() protoreflect.Message {
	mi := &file_openmatch_frontend_proto_msgTypes[8]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DeleteBackfillRequest.ProtoReflect.Descriptor instead.
func (*DeleteBackfillRequest) Descriptor() ([]byte, []int) {
	return file_openmatch_frontend_proto_rawDescGZIP(), []int{8}
}

func (x *DeleteBackfillRequest) GetBackfillId() string {
	if x != nil {
		return x.BackfillId
	}
	return ""
}

type GetBackfillRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	BackfillId string `protobuf:"bytes,1,opt,name=backfill_id,json=backfillId,proto3" json:"backfill_id,omitempty"`
}

func (x *GetBackfillRequest) Reset() {
	*x = GetBackfillRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_openmatch_frontend_proto_msgTypes[9]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetBackfillRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetBackfillRequest) ProtoMessage() {}

func (x *GetBackfillRequest) ProtoReflect() protoreflect.Message {
	mi := &file_openmatch_frontend_proto_msgTypes[9]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetBackfillRequest.ProtoReflect.Descriptor instead.
func (*GetBackfillRequest) Descriptor() ([]byte, []int) {
	return file_openmatch_frontend_proto_rawDescGZIP(), []int{9}
}

func (x *GetBackfillRequest) GetBackfillId() string {
	if x != nil {
		return x.BackfillId
	}
	return ""
}

type UpdateBackfillRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Backfill *Backfill `protobuf:"bytes,1,opt,name=backfill,proto3" json:"backfill,omitempty"`
}

func (x *UpdateBackfillRequest) Reset() {
	*x = UpdateBackfillRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_openmatch_frontend_proto_msgTypes[10]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UpdateBackfillRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateBackfillRequest) ProtoMessage() {}

func (x *UpdateBackfillRequest) ProtoReflect() protoreflect.Message {
	mi := &file_openmatch_frontend_proto_msgTypes[10]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdateBackfillRequest.ProtoReflect.Descriptor instead.
func (*UpdateBackfillRequest) Descriptor() ([]byte, []int) {
	return file_openmatch_frontend_proto_rawDescGZIP(), []int{10}
}

func (x *UpdateBackfillRequest) GetBackfill() *Backfill {
	if x != nil {
		return x.Backfill
	}
	return nil
}

var File_openmatch_frontend_proto protoreflect.FileDescriptor

var file_openmatch_frontend_proto_rawDesc = []byte{
	0x0a, 0x18, 0x6f, 0x70, 0x65, 0x6e, 0x6d, 0x61, 0x74, 0x63, 0x68, 0x2f, 0x66, 0x72, 0x6f, 0x6e,
	0x74, 0x65, 0x6e, 0x64, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x09, 0x6f, 0x70, 0x65, 0x6e,
	0x6d, 0x61, 0x74, 0x63, 0x68, 0x1a, 0x18, 0x6f, 0x70, 0x65, 0x6e, 0x6d, 0x61, 0x74, 0x63, 0x68,
	0x2f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a,
	0x1b, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66,
	0x2f, 0x65, 0x6d, 0x70, 0x74, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x40, 0x0a, 0x13,
	0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x54, 0x69, 0x63, 0x6b, 0x65, 0x74, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x12, 0x29, 0x0a, 0x06, 0x74, 0x69, 0x63, 0x6b, 0x65, 0x74, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x11, 0x2e, 0x6f, 0x70, 0x65, 0x6e, 0x6d, 0x61, 0x74, 0x63, 0x68, 0x2e,
	0x54, 0x69, 0x63, 0x6b, 0x65, 0x74, 0x52, 0x06, 0x74, 0x69, 0x63, 0x6b, 0x65, 0x74, 0x22, 0x32,
	0x0a, 0x13, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x54, 0x69, 0x63, 0x6b, 0x65, 0x74, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1b, 0x0a, 0x09, 0x74, 0x69, 0x63, 0x6b, 0x65, 0x74, 0x5f,
	0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x74, 0x69, 0x63, 0x6b, 0x65, 0x74,
	0x49, 0x64, 0x22, 0x2f, 0x0a, 0x10, 0x47, 0x65, 0x74, 0x54, 0x69, 0x63, 0x6b, 0x65, 0x74, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1b, 0x0a, 0x09, 0x74, 0x69, 0x63, 0x6b, 0x65, 0x74,
	0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x74, 0x69, 0x63, 0x6b, 0x65,
	0x74, 0x49, 0x64, 0x22, 0x36, 0x0a, 0x17, 0x57, 0x61, 0x74, 0x63, 0x68, 0x41, 0x73, 0x73, 0x69,
	0x67, 0x6e, 0x6d, 0x65, 0x6e, 0x74, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1b,
	0x0a, 0x09, 0x74, 0x69, 0x63, 0x6b, 0x65, 0x74, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x08, 0x74, 0x69, 0x63, 0x6b, 0x65, 0x74, 0x49, 0x64, 0x22, 0x51, 0x0a, 0x18, 0x57,
	0x61, 0x74, 0x63, 0x68, 0x41, 0x73, 0x73, 0x69, 0x67, 0x6e, 0x6d, 0x65, 0x6e, 0x74, 0x73, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x35, 0x0a, 0x0a, 0x61, 0x73, 0x73, 0x69, 0x67,
	0x6e, 0x6d, 0x65, 0x6e, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x15, 0x2e, 0x6f, 0x70,
	0x65, 0x6e, 0x6d, 0x61, 0x74, 0x63, 0x68, 0x2e, 0x41, 0x73, 0x73, 0x69, 0x67, 0x6e, 0x6d, 0x65,
	0x6e, 0x74, 0x52, 0x0a, 0x61, 0x73, 0x73, 0x69, 0x67, 0x6e, 0x6d, 0x65, 0x6e, 0x74, 0x22, 0x74,
	0x0a, 0x1a, 0x41, 0x63, 0x6b, 0x6e, 0x6f, 0x77, 0x6c, 0x65, 0x64, 0x67, 0x65, 0x42, 0x61, 0x63,
	0x6b, 0x66, 0x69, 0x6c, 0x6c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1f, 0x0a, 0x0b,
	0x62, 0x61, 0x63, 0x6b, 0x66, 0x69, 0x6c, 0x6c, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x0a, 0x62, 0x61, 0x63, 0x6b, 0x66, 0x69, 0x6c, 0x6c, 0x49, 0x64, 0x12, 0x35, 0x0a,
	0x0a, 0x61, 0x73, 0x73, 0x69, 0x67, 0x6e, 0x6d, 0x65, 0x6e, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x15, 0x2e, 0x6f, 0x70, 0x65, 0x6e, 0x6d, 0x61, 0x74, 0x63, 0x68, 0x2e, 0x41, 0x73,
	0x73, 0x69, 0x67, 0x6e, 0x6d, 0x65, 0x6e, 0x74, 0x52, 0x0a, 0x61, 0x73, 0x73, 0x69, 0x67, 0x6e,
	0x6d, 0x65, 0x6e, 0x74, 0x22, 0x7b, 0x0a, 0x1b, 0x41, 0x63, 0x6b, 0x6e, 0x6f, 0x77, 0x6c, 0x65,
	0x64, 0x67, 0x65, 0x42, 0x61, 0x63, 0x6b, 0x66, 0x69, 0x6c, 0x6c, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x12, 0x2f, 0x0a, 0x08, 0x62, 0x61, 0x63, 0x6b, 0x66, 0x69, 0x6c, 0x6c, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x13, 0x2e, 0x6f, 0x70, 0x65, 0x6e, 0x6d, 0x61, 0x74, 0x63,
	0x68, 0x2e, 0x42, 0x61, 0x63, 0x6b, 0x66, 0x69, 0x6c, 0x6c, 0x52, 0x08, 0x62, 0x61, 0x63, 0x6b,
	0x66, 0x69, 0x6c, 0x6c, 0x12, 0x2b, 0x0a, 0x07, 0x74, 0x69, 0x63, 0x6b, 0x65, 0x74, 0x73, 0x18,
	0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x11, 0x2e, 0x6f, 0x70, 0x65, 0x6e, 0x6d, 0x61, 0x74, 0x63,
	0x68, 0x2e, 0x54, 0x69, 0x63, 0x6b, 0x65, 0x74, 0x52, 0x07, 0x74, 0x69, 0x63, 0x6b, 0x65, 0x74,
	0x73, 0x22, 0x48, 0x0a, 0x15, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x42, 0x61, 0x63, 0x6b, 0x66,
	0x69, 0x6c, 0x6c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x2f, 0x0a, 0x08, 0x62, 0x61,
	0x63, 0x6b, 0x66, 0x69, 0x6c, 0x6c, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x13, 0x2e, 0x6f,
	0x70, 0x65, 0x6e, 0x6d, 0x61, 0x74, 0x63, 0x68, 0x2e, 0x42, 0x61, 0x63, 0x6b, 0x66, 0x69, 0x6c,
	0x6c, 0x52, 0x08, 0x62, 0x61, 0x63, 0x6b, 0x66, 0x69, 0x6c, 0x6c, 0x22, 0x38, 0x0a, 0x15, 0x44,
	0x65, 0x6c, 0x65, 0x74, 0x65, 0x42, 0x61, 0x63, 0x6b, 0x66, 0x69, 0x6c, 0x6c, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x12, 0x1f, 0x0a, 0x0b, 0x62, 0x61, 0x63, 0x6b, 0x66, 0x69, 0x6c, 0x6c,
	0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x62, 0x61, 0x63, 0x6b, 0x66,
	0x69, 0x6c, 0x6c, 0x49, 0x64, 0x22, 0x35, 0x0a, 0x12, 0x47, 0x65, 0x74, 0x42, 0x61, 0x63, 0x6b,
	0x66, 0x69, 0x6c, 0x6c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1f, 0x0a, 0x0b, 0x62,
	0x61, 0x63, 0x6b, 0x66, 0x69, 0x6c, 0x6c, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x0a, 0x62, 0x61, 0x63, 0x6b, 0x66, 0x69, 0x6c, 0x6c, 0x49, 0x64, 0x22, 0x48, 0x0a, 0x15,
	0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x42, 0x61, 0x63, 0x6b, 0x66, 0x69, 0x6c, 0x6c, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x2f, 0x0a, 0x08, 0x62, 0x61, 0x63, 0x6b, 0x66, 0x69, 0x6c,
	0x6c, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x13, 0x2e, 0x6f, 0x70, 0x65, 0x6e, 0x6d, 0x61,
	0x74, 0x63, 0x68, 0x2e, 0x42, 0x61, 0x63, 0x6b, 0x66, 0x69, 0x6c, 0x6c, 0x52, 0x08, 0x62, 0x61,
	0x63, 0x6b, 0x66, 0x69, 0x6c, 0x6c, 0x32, 0xbf, 0x05, 0x0a, 0x0f, 0x46, 0x72, 0x6f, 0x6e, 0x74,
	0x65, 0x6e, 0x64, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x41, 0x0a, 0x0c, 0x43, 0x72,
	0x65, 0x61, 0x74, 0x65, 0x54, 0x69, 0x63, 0x6b, 0x65, 0x74, 0x12, 0x1e, 0x2e, 0x6f, 0x70, 0x65,
	0x6e, 0x6d, 0x61, 0x74, 0x63, 0x68, 0x2e, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x54, 0x69, 0x63,
	0x6b, 0x65, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x11, 0x2e, 0x6f, 0x70, 0x65,
	0x6e, 0x6d, 0x61, 0x74, 0x63, 0x68, 0x2e, 0x54, 0x69, 0x63, 0x6b, 0x65, 0x74, 0x12, 0x46, 0x0a,
	0x0c, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x54, 0x69, 0x63, 0x6b, 0x65, 0x74, 0x12, 0x1e, 0x2e,
	0x6f, 0x70, 0x65, 0x6e, 0x6d, 0x61, 0x74, 0x63, 0x68, 0x2e, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65,
	0x54, 0x69, 0x63, 0x6b, 0x65, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x16, 0x2e,
	0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e,
	0x45, 0x6d, 0x70, 0x74, 0x79, 0x12, 0x3b, 0x0a, 0x09, 0x47, 0x65, 0x74, 0x54, 0x69, 0x63, 0x6b,
	0x65, 0x74, 0x12, 0x1b, 0x2e, 0x6f, 0x70, 0x65, 0x6e, 0x6d, 0x61, 0x74, 0x63, 0x68, 0x2e, 0x47,
	0x65, 0x74, 0x54, 0x69, 0x63, 0x6b, 0x65, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a,
	0x11, 0x2e, 0x6f, 0x70, 0x65, 0x6e, 0x6d, 0x61, 0x74, 0x63, 0x68, 0x2e, 0x54, 0x69, 0x63, 0x6b,
	0x65, 0x74, 0x12, 0x5d, 0x0a, 0x10, 0x57, 0x61, 0x74, 0x63, 0x68, 0x41, 0x73, 0x73, 0x69, 0x67,
	0x6e, 0x6d, 0x65, 0x6e, 0x74, 0x73, 0x12, 0x22, 0x2e, 0x6f, 0x70, 0x65, 0x6e, 0x6d, 0x61, 0x74,
	0x63, 0x68, 0x2e, 0x57, 0x61, 0x74, 0x63, 0x68, 0x41, 0x73, 0x73, 0x69, 0x67, 0x6e, 0x6d, 0x65,
	0x6e, 0x74, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x23, 0x2e, 0x6f, 0x70, 0x65,
	0x6e, 0x6d, 0x61, 0x74, 0x63, 0x68, 0x2e, 0x57, 0x61, 0x74, 0x63, 0x68, 0x41, 0x73, 0x73, 0x69,
	0x67, 0x6e, 0x6d, 0x65, 0x6e, 0x74, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x30,
	0x01, 0x12, 0x64, 0x0a, 0x13, 0x41, 0x63, 0x6b, 0x6e, 0x6f, 0x77, 0x6c, 0x65, 0x64, 0x67, 0x65,
	0x42, 0x61, 0x63, 0x6b, 0x66, 0x69, 0x6c, 0x6c, 0x12, 0x25, 0x2e, 0x6f, 0x70, 0x65, 0x6e, 0x6d,
	0x61, 0x74, 0x63, 0x68, 0x2e, 0x41, 0x63, 0x6b, 0x6e, 0x6f, 0x77, 0x6c, 0x65, 0x64, 0x67, 0x65,
	0x42, 0x61, 0x63, 0x6b, 0x66, 0x69, 0x6c, 0x6c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a,
	0x26, 0x2e, 0x6f, 0x70, 0x65, 0x6e, 0x6d, 0x61, 0x74, 0x63, 0x68, 0x2e, 0x41, 0x63, 0x6b, 0x6e,
	0x6f, 0x77, 0x6c, 0x65, 0x64, 0x67, 0x65, 0x42, 0x61, 0x63, 0x6b, 0x66, 0x69, 0x6c, 0x6c, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x47, 0x0a, 0x0e, 0x43, 0x72, 0x65, 0x61, 0x74,
	0x65, 0x42, 0x61, 0x63, 0x6b, 0x66, 0x69, 0x6c, 0x6c, 0x12, 0x20, 0x2e, 0x6f, 0x70, 0x65, 0x6e,
	0x6d, 0x61, 0x74, 0x63, 0x68, 0x2e, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x42, 0x61, 0x63, 0x6b,
	0x66, 0x69, 0x6c, 0x6c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x13, 0x2e, 0x6f, 0x70,
	0x65, 0x6e, 0x6d, 0x61, 0x74, 0x63, 0x68, 0x2e, 0x42, 0x61, 0x63, 0x6b, 0x66, 0x69, 0x6c, 0x6c,
	0x12, 0x4a, 0x0a, 0x0e, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x42, 0x61, 0x63, 0x6b, 0x66, 0x69,
	0x6c, 0x6c, 0x12, 0x20, 0x2e, 0x6f, 0x70, 0x65, 0x6e, 0x6d, 0x61, 0x74, 0x63, 0x68, 0x2e, 0x44,
	0x65, 0x6c, 0x65, 0x74, 0x65, 0x42, 0x61, 0x63, 0x6b, 0x66, 0x69, 0x6c, 0x6c, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x1a, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x12, 0x41, 0x0a, 0x0b,
	0x47, 0x65, 0x74, 0x42, 0x61, 0x63, 0x6b, 0x66, 0x69, 0x6c, 0x6c, 0x12, 0x1d, 0x2e, 0x6f, 0x70,
	0x65, 0x6e, 0x6d, 0x61, 0x74, 0x63, 0x68, 0x2e, 0x47, 0x65, 0x74, 0x42, 0x61, 0x63, 0x6b, 0x66,
	0x69, 0x6c, 0x6c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x13, 0x2e, 0x6f, 0x70, 0x65,
	0x6e, 0x6d, 0x61, 0x74, 0x63, 0x68, 0x2e, 0x42, 0x61, 0x63, 0x6b, 0x66, 0x69, 0x6c, 0x6c, 0x12,
	0x47, 0x0a, 0x0e, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x42, 0x61, 0x63, 0x6b, 0x66, 0x69, 0x6c,
	0x6c, 0x12, 0x20, 0x2e, 0x6f, 0x70, 0x65, 0x6e, 0x6d, 0x61, 0x74, 0x63, 0x68, 0x2e, 0x55, 0x70,
	0x64, 0x61, 0x74, 0x65, 0x42, 0x61, 0x63, 0x6b, 0x66, 0x69, 0x6c, 0x6c, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x1a, 0x13, 0x2e, 0x6f, 0x70, 0x65, 0x6e, 0x6d, 0x61, 0x74, 0x63, 0x68, 0x2e,
	0x42, 0x61, 0x63, 0x6b, 0x66, 0x69, 0x6c, 0x6c, 0x42, 0x90, 0x01, 0x0a, 0x0d, 0x63, 0x6f, 0x6d,
	0x2e, 0x6f, 0x70, 0x65, 0x6e, 0x6d, 0x61, 0x74, 0x63, 0x68, 0x42, 0x0d, 0x46, 0x72, 0x6f, 0x6e,
	0x74, 0x65, 0x6e, 0x64, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x2c, 0x67, 0x69, 0x74,
	0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x63, 0x61, 0x73, 0x74, 0x61, 0x6e, 0x65, 0x61,
	0x69, 0x2f, 0x6d, 0x69, 0x6e, 0x69, 0x6d, 0x61, 0x74, 0x63, 0x68, 0x2f, 0x67, 0x65, 0x6e, 0x2f,
	0x6f, 0x70, 0x65, 0x6e, 0x6d, 0x61, 0x74, 0x63, 0x68, 0xa2, 0x02, 0x03, 0x4f, 0x58, 0x58, 0xaa,
	0x02, 0x09, 0x4f, 0x70, 0x65, 0x6e, 0x6d, 0x61, 0x74, 0x63, 0x68, 0xca, 0x02, 0x09, 0x4f, 0x70,
	0x65, 0x6e, 0x6d, 0x61, 0x74, 0x63, 0x68, 0xe2, 0x02, 0x15, 0x4f, 0x70, 0x65, 0x6e, 0x6d, 0x61,
	0x74, 0x63, 0x68, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0xea,
	0x02, 0x09, 0x4f, 0x70, 0x65, 0x6e, 0x6d, 0x61, 0x74, 0x63, 0x68, 0x62, 0x06, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x33,
}

var (
	file_openmatch_frontend_proto_rawDescOnce sync.Once
	file_openmatch_frontend_proto_rawDescData = file_openmatch_frontend_proto_rawDesc
)

func file_openmatch_frontend_proto_rawDescGZIP() []byte {
	file_openmatch_frontend_proto_rawDescOnce.Do(func() {
		file_openmatch_frontend_proto_rawDescData = protoimpl.X.CompressGZIP(file_openmatch_frontend_proto_rawDescData)
	})
	return file_openmatch_frontend_proto_rawDescData
}

var file_openmatch_frontend_proto_msgTypes = make([]protoimpl.MessageInfo, 11)
var file_openmatch_frontend_proto_goTypes = []any{
	(*CreateTicketRequest)(nil),         // 0: openmatch.CreateTicketRequest
	(*DeleteTicketRequest)(nil),         // 1: openmatch.DeleteTicketRequest
	(*GetTicketRequest)(nil),            // 2: openmatch.GetTicketRequest
	(*WatchAssignmentsRequest)(nil),     // 3: openmatch.WatchAssignmentsRequest
	(*WatchAssignmentsResponse)(nil),    // 4: openmatch.WatchAssignmentsResponse
	(*AcknowledgeBackfillRequest)(nil),  // 5: openmatch.AcknowledgeBackfillRequest
	(*AcknowledgeBackfillResponse)(nil), // 6: openmatch.AcknowledgeBackfillResponse
	(*CreateBackfillRequest)(nil),       // 7: openmatch.CreateBackfillRequest
	(*DeleteBackfillRequest)(nil),       // 8: openmatch.DeleteBackfillRequest
	(*GetBackfillRequest)(nil),          // 9: openmatch.GetBackfillRequest
	(*UpdateBackfillRequest)(nil),       // 10: openmatch.UpdateBackfillRequest
	(*Ticket)(nil),                      // 11: openmatch.Ticket
	(*Assignment)(nil),                  // 12: openmatch.Assignment
	(*Backfill)(nil),                    // 13: openmatch.Backfill
	(*emptypb.Empty)(nil),               // 14: google.protobuf.Empty
}
var file_openmatch_frontend_proto_depIdxs = []int32{
	11, // 0: openmatch.CreateTicketRequest.ticket:type_name -> openmatch.Ticket
	12, // 1: openmatch.WatchAssignmentsResponse.assignment:type_name -> openmatch.Assignment
	12, // 2: openmatch.AcknowledgeBackfillRequest.assignment:type_name -> openmatch.Assignment
	13, // 3: openmatch.AcknowledgeBackfillResponse.backfill:type_name -> openmatch.Backfill
	11, // 4: openmatch.AcknowledgeBackfillResponse.tickets:type_name -> openmatch.Ticket
	13, // 5: openmatch.CreateBackfillRequest.backfill:type_name -> openmatch.Backfill
	13, // 6: openmatch.UpdateBackfillRequest.backfill:type_name -> openmatch.Backfill
	0,  // 7: openmatch.FrontendService.CreateTicket:input_type -> openmatch.CreateTicketRequest
	1,  // 8: openmatch.FrontendService.DeleteTicket:input_type -> openmatch.DeleteTicketRequest
	2,  // 9: openmatch.FrontendService.GetTicket:input_type -> openmatch.GetTicketRequest
	3,  // 10: openmatch.FrontendService.WatchAssignments:input_type -> openmatch.WatchAssignmentsRequest
	5,  // 11: openmatch.FrontendService.AcknowledgeBackfill:input_type -> openmatch.AcknowledgeBackfillRequest
	7,  // 12: openmatch.FrontendService.CreateBackfill:input_type -> openmatch.CreateBackfillRequest
	8,  // 13: openmatch.FrontendService.DeleteBackfill:input_type -> openmatch.DeleteBackfillRequest
	9,  // 14: openmatch.FrontendService.GetBackfill:input_type -> openmatch.GetBackfillRequest
	10, // 15: openmatch.FrontendService.UpdateBackfill:input_type -> openmatch.UpdateBackfillRequest
	11, // 16: openmatch.FrontendService.CreateTicket:output_type -> openmatch.Ticket
	14, // 17: openmatch.FrontendService.DeleteTicket:output_type -> google.protobuf.Empty
	11, // 18: openmatch.FrontendService.GetTicket:output_type -> openmatch.Ticket
	4,  // 19: openmatch.FrontendService.WatchAssignments:output_type -> openmatch.WatchAssignmentsResponse
	6,  // 20: openmatch.FrontendService.AcknowledgeBackfill:output_type -> openmatch.AcknowledgeBackfillResponse
	13, // 21: openmatch.FrontendService.CreateBackfill:output_type -> openmatch.Backfill
	14, // 22: openmatch.FrontendService.DeleteBackfill:output_type -> google.protobuf.Empty
	13, // 23: openmatch.FrontendService.GetBackfill:output_type -> openmatch.Backfill
	13, // 24: openmatch.FrontendService.UpdateBackfill:output_type -> openmatch.Backfill
	16, // [16:25] is the sub-list for method output_type
	7,  // [7:16] is the sub-list for method input_type
	7,  // [7:7] is the sub-list for extension type_name
	7,  // [7:7] is the sub-list for extension extendee
	0,  // [0:7] is the sub-list for field type_name
}

func init() { file_openmatch_frontend_proto_init() }
func file_openmatch_frontend_proto_init() {
	if File_openmatch_frontend_proto != nil {
		return
	}
	file_openmatch_messages_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_openmatch_frontend_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*CreateTicketRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_openmatch_frontend_proto_msgTypes[1].Exporter = func(v any, i int) any {
			switch v := v.(*DeleteTicketRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_openmatch_frontend_proto_msgTypes[2].Exporter = func(v any, i int) any {
			switch v := v.(*GetTicketRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_openmatch_frontend_proto_msgTypes[3].Exporter = func(v any, i int) any {
			switch v := v.(*WatchAssignmentsRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_openmatch_frontend_proto_msgTypes[4].Exporter = func(v any, i int) any {
			switch v := v.(*WatchAssignmentsResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_openmatch_frontend_proto_msgTypes[5].Exporter = func(v any, i int) any {
			switch v := v.(*AcknowledgeBackfillRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_openmatch_frontend_proto_msgTypes[6].Exporter = func(v any, i int) any {
			switch v := v.(*AcknowledgeBackfillResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_openmatch_frontend_proto_msgTypes[7].Exporter = func(v any, i int) any {
			switch v := v.(*CreateBackfillRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_openmatch_frontend_proto_msgTypes[8].Exporter = func(v any, i int) any {
			switch v := v.(*DeleteBackfillRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_openmatch_frontend_proto_msgTypes[9].Exporter = func(v any, i int) any {
			switch v := v.(*GetBackfillRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_openmatch_frontend_proto_msgTypes[10].Exporter = func(v any, i int) any {
			switch v := v.(*UpdateBackfillRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_openmatch_frontend_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   11,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_openmatch_frontend_proto_goTypes,
		DependencyIndexes: file_openmatch_frontend_proto_depIdxs,
		MessageInfos:      file_openmatch_frontend_proto_msgTypes,
	}.Build()
	File_openmatch_frontend_proto = out.File
	file_openmatch_frontend_proto_rawDesc = nil
	file_openmatch_frontend_proto_goTypes = nil
	file_openmatch_frontend_proto_depIdxs = nil
}