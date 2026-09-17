package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// ShopFile describes the proto the tests load:
//
//	syntax = "proto3";
//	package shop.v1;
//
//	enum Status { STATUS_UNSPECIFIED = 0; OPEN = 1; CHECKED_OUT = 2; }
//	message LineItem { string sku = 1; int32 quantity = 2; }
//	message CreateCartRequest { string customer_id = 1; string currency = 2; }
//	message Cart {
//	  string cart_id = 1; int64 subtotal = 2; Status status = 3;
//	  repeated LineItem items = 4; optional string coupon_code = 5;
//	}
//	service Carts {
//	  rpc CreateCart(CreateCartRequest) returns (Cart);
//	  rpc WatchCart(CreateCartRequest) returns (stream Cart);
//	}
//
// It is built here rather than checked in as a .protoset so tests need no
// protoc, and so the shapes under test are visible beside the assertions.
func ShopFile() *descriptorpb.FileDescriptorProto {
	str := descriptorpb.FieldDescriptorProto_TYPE_STRING
	i64 := descriptorpb.FieldDescriptorProto_TYPE_INT64
	i32 := descriptorpb.FieldDescriptorProto_TYPE_INT32
	enum := descriptorpb.FieldDescriptorProto_TYPE_ENUM
	msg := descriptorpb.FieldDescriptorProto_TYPE_MESSAGE
	one := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL
	many := descriptorpb.FieldDescriptorProto_LABEL_REPEATED

	field := func(name string, num int32, t descriptorpb.FieldDescriptorProto_Type, label descriptorpb.FieldDescriptorProto_Label, typeName string) *descriptorpb.FieldDescriptorProto {
		f := &descriptorpb.FieldDescriptorProto{
			Name: proto.String(name), Number: proto.Int32(num),
			Type: t.Enum(), Label: label.Enum(),
			JsonName: proto.String(jsonName(name)),
		}
		if typeName != "" {
			f.TypeName = proto.String(typeName)
		}
		return f
	}

	coupon := field("coupon_code", 5, str, one, "")
	coupon.Proto3Optional = proto.Bool(true)
	coupon.OneofIndex = proto.Int32(0)

	return &descriptorpb.FileDescriptorProto{
		Name:    proto.String("shop/v1/carts.proto"),
		Package: proto.String("shop.v1"),
		Syntax:  proto.String("proto3"),
		EnumType: []*descriptorpb.EnumDescriptorProto{{
			Name: proto.String("Status"),
			Value: []*descriptorpb.EnumValueDescriptorProto{
				{Name: proto.String("STATUS_UNSPECIFIED"), Number: proto.Int32(0)},
				{Name: proto.String("OPEN"), Number: proto.Int32(1)},
				{Name: proto.String("CHECKED_OUT"), Number: proto.Int32(2)},
			},
		}},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("LineItem"),
				Field: []*descriptorpb.FieldDescriptorProto{
					field("sku", 1, str, one, ""),
					field("quantity", 2, i32, one, ""),
				},
			},
			{
				Name: proto.String("CreateCartRequest"),
				Field: []*descriptorpb.FieldDescriptorProto{
					field("customer_id", 1, str, one, ""),
					field("currency", 2, str, one, ""),
				},
			},
			{
				Name: proto.String("Cart"),
				Field: []*descriptorpb.FieldDescriptorProto{
					field("cart_id", 1, str, one, ""),
					field("subtotal", 2, i64, one, ""),
					field("status", 3, enum, one, ".shop.v1.Status"),
					field("items", 4, msg, many, ".shop.v1.LineItem"),
					coupon,
				},
				OneofDecl: []*descriptorpb.OneofDescriptorProto{{Name: proto.String("_coupon_code")}},
			},
		},
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name: proto.String("Carts"),
			Method: []*descriptorpb.MethodDescriptorProto{
				{
					Name:       proto.String("CreateCart"),
					InputType:  proto.String(".shop.v1.CreateCartRequest"),
					OutputType: proto.String(".shop.v1.Cart"),
				},
				{
					Name:            proto.String("WatchCart"),
					InputType:       proto.String(".shop.v1.CreateCartRequest"),
					OutputType:      proto.String(".shop.v1.Cart"),
					ServerStreaming: proto.Bool(true),
				},
			},
		}},
	}
}

// jsonName is protoc's lowerCamelCase derivation for a field's JSON name.
func jsonName(s string) string {
	out := make([]byte, 0, len(s))
	up := false
	for i := 0; i < len(s); i++ {
		if s[i] == '_' {
			up = true
			continue
		}
		c := s[i]
		if up && c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		up = false
		out = append(out, c)
	}
	return string(out)
}

// WriteDescriptorSet marshals files into a descriptor set on disk and returns
// its path, as protoc --descriptor_set_out would.
func WriteDescriptorSet(t *testing.T, name string, files ...*descriptorpb.FileDescriptorProto) string {
	t.Helper()
	data, err := proto.Marshal(&descriptorpb.FileDescriptorSet{File: files})
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, data, 0o600))
	return path
}
