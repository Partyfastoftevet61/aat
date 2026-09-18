package testutil

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// PointsFile describes a vector-database proto shaped like Qdrant's, for the
// shapes ShopFile lacks: map fields, oneofs, and a fork of google.protobuf.Value
// that the JSON codec treats as an ordinary message.
//
//	syntax = "proto3";
//	package vectors.v1;
//
//	enum NullValue { NULL_VALUE = 0; }
//	enum Distance { UnknownDistance = 0; Cosine = 1; Dot = 2; }
//	message Struct { map<string, Value> fields = 1; }
//	message Value {
//	  oneof kind {
//	    NullValue null_value = 1; double double_value = 2; int64 integer_value = 3;
//	    string string_value = 4; bool bool_value = 5;
//	    Struct struct_value = 6; ListValue list_value = 7;
//	  }
//	}
//	message ListValue { repeated Value values = 1; }
//	message PointId { oneof point_id_options { uint64 num = 1; string uuid = 2; } }
//	message VectorParams { uint64 size = 1; Distance distance = 2; }
//	message VectorParamsMap { map<string, VectorParams> map = 1; }
//	message VectorsConfig { oneof config { VectorParams params = 1; VectorParamsMap params_map = 2; } }
//	message CreateCollection {
//	  string collection_name = 1; optional VectorsConfig vectors_config = 2;
//	  map<string, Value> metadata = 3; map<string, string> labels = 4;
//	}
//	message GetCollection { string collection_name = 1; }
//	message CollectionConfig { VectorsConfig vectors_config = 1; map<string, Value> metadata = 2; }
//	message CollectionInfo { CollectionConfig config = 1; optional uint64 points_count = 2; }
//	message GetCollectionResponse { CollectionInfo result = 1; double time = 2; }
//	message PointStruct { PointId id = 1; map<string, Value> payload = 2; repeated float vector = 3; }
//	message UpsertPoints { string collection_name = 1; optional bool wait = 2; repeated PointStruct points = 3; }
//	message SearchPoints { string collection_name = 1; repeated float vector = 2; uint64 limit = 3; }
//	message ScoredPoint {
//	  PointId id = 1; map<string, Value> payload = 2; float score = 3;
//	  map<int64, string> tags_by_rank = 4; string shard_key = 5;
//	}
//	message SearchResponse { repeated ScoredPoint result = 1; }
//	message OperationResponse { bool result = 1; }
//	service Collections {
//	  rpc Create(CreateCollection) returns (OperationResponse);
//	  rpc Get(GetCollection) returns (GetCollectionResponse);
//	}
//	service Points {
//	  rpc Upsert(UpsertPoints) returns (OperationResponse);
//	  rpc Search(SearchPoints) returns (SearchResponse);
//	}
func PointsFile() *descriptorpb.FileDescriptorProto {
	const pkg = ".vectors.v1."
	value := pkg + "Value"

	return &descriptorpb.FileDescriptorProto{
		Name:    proto.String("vectors/v1/points.proto"),
		Package: proto.String("vectors.v1"),
		Syntax:  proto.String("proto3"),
		EnumType: []*descriptorpb.EnumDescriptorProto{
			enumProto("NullValue", "NULL_VALUE"),
			enumProto("Distance", "UnknownDistance", "Cosine", "Dot"),
		},
		MessageType: []*descriptorpb.DescriptorProto{
			withMap(&descriptorpb.DescriptorProto{Name: proto.String("Struct")}, "fields", 1, tString, tMessage, value),
			{
				Name: proto.String("Value"),
				Field: []*descriptorpb.FieldDescriptorProto{
					inOneof(fieldProto("null_value", 1, tEnum, one, pkg+"NullValue"), 0),
					inOneof(fieldProto("double_value", 2, tDouble, one, ""), 0),
					inOneof(fieldProto("integer_value", 3, tInt64, one, ""), 0),
					inOneof(fieldProto("string_value", 4, tString, one, ""), 0),
					inOneof(fieldProto("bool_value", 5, tBool, one, ""), 0),
					inOneof(fieldProto("struct_value", 6, tMessage, one, pkg+"Struct"), 0),
					inOneof(fieldProto("list_value", 7, tMessage, one, pkg+"ListValue"), 0),
				},
				OneofDecl: oneofs("kind"),
			},
			{
				Name:  proto.String("ListValue"),
				Field: []*descriptorpb.FieldDescriptorProto{fieldProto("values", 1, tMessage, many, value)},
			},
			{
				Name: proto.String("PointId"),
				Field: []*descriptorpb.FieldDescriptorProto{
					inOneof(fieldProto("num", 1, tUint64, one, ""), 0),
					inOneof(fieldProto("uuid", 2, tString, one, ""), 0),
				},
				OneofDecl: oneofs("point_id_options"),
			},
			{
				Name: proto.String("VectorParams"),
				Field: []*descriptorpb.FieldDescriptorProto{
					fieldProto("size", 1, tUint64, one, ""),
					fieldProto("distance", 2, tEnum, one, pkg+"Distance"),
				},
			},
			withMap(&descriptorpb.DescriptorProto{Name: proto.String("VectorParamsMap")}, "map", 1, tString, tMessage, pkg+"VectorParams"),
			{
				Name: proto.String("VectorsConfig"),
				Field: []*descriptorpb.FieldDescriptorProto{
					inOneof(fieldProto("params", 1, tMessage, one, pkg+"VectorParams"), 0),
					inOneof(fieldProto("params_map", 2, tMessage, one, pkg+"VectorParamsMap"), 0),
				},
				OneofDecl: oneofs("config"),
			},
			withMap(withMap(&descriptorpb.DescriptorProto{
				Name: proto.String("CreateCollection"),
				Field: []*descriptorpb.FieldDescriptorProto{
					fieldProto("collection_name", 1, tString, one, ""),
					optional(fieldProto("vectors_config", 2, tMessage, one, pkg+"VectorsConfig"), 0),
				},
				OneofDecl: oneofs("_vectors_config"),
			}, "metadata", 3, tString, tMessage, value), "labels", 4, tString, tString, ""),
			{
				Name:  proto.String("GetCollection"),
				Field: []*descriptorpb.FieldDescriptorProto{fieldProto("collection_name", 1, tString, one, "")},
			},
			withMap(&descriptorpb.DescriptorProto{
				Name:  proto.String("CollectionConfig"),
				Field: []*descriptorpb.FieldDescriptorProto{fieldProto("vectors_config", 1, tMessage, one, pkg+"VectorsConfig")},
			}, "metadata", 2, tString, tMessage, value),
			{
				Name: proto.String("CollectionInfo"),
				Field: []*descriptorpb.FieldDescriptorProto{
					fieldProto("config", 1, tMessage, one, pkg+"CollectionConfig"),
					optional(fieldProto("points_count", 2, tUint64, one, ""), 0),
				},
				OneofDecl: oneofs("_points_count"),
			},
			{
				Name: proto.String("GetCollectionResponse"),
				Field: []*descriptorpb.FieldDescriptorProto{
					fieldProto("result", 1, tMessage, one, pkg+"CollectionInfo"),
					fieldProto("time", 2, tDouble, one, ""),
				},
			},
			withMap(&descriptorpb.DescriptorProto{
				Name: proto.String("PointStruct"),
				Field: []*descriptorpb.FieldDescriptorProto{
					fieldProto("id", 1, tMessage, one, pkg+"PointId"),
					fieldProto("vector", 3, tFloat, many, ""),
				},
			}, "payload", 2, tString, tMessage, value),
			{
				Name: proto.String("UpsertPoints"),
				Field: []*descriptorpb.FieldDescriptorProto{
					fieldProto("collection_name", 1, tString, one, ""),
					optional(fieldProto("wait", 2, tBool, one, ""), 0),
					fieldProto("points", 3, tMessage, many, pkg+"PointStruct"),
				},
				OneofDecl: oneofs("_wait"),
			},
			{
				Name: proto.String("SearchPoints"),
				Field: []*descriptorpb.FieldDescriptorProto{
					fieldProto("collection_name", 1, tString, one, ""),
					fieldProto("vector", 2, tFloat, many, ""),
					fieldProto("limit", 3, tUint64, one, ""),
				},
			},
			withMap(withMap(&descriptorpb.DescriptorProto{
				Name: proto.String("ScoredPoint"),
				Field: []*descriptorpb.FieldDescriptorProto{
					fieldProto("id", 1, tMessage, one, pkg+"PointId"),
					fieldProto("score", 3, tFloat, one, ""),
					fieldProto("shard_key", 5, tString, one, ""),
				},
			}, "payload", 2, tString, tMessage, value), "tags_by_rank", 4, tInt64, tString, ""),
			{
				Name:  proto.String("SearchResponse"),
				Field: []*descriptorpb.FieldDescriptorProto{fieldProto("result", 1, tMessage, many, pkg+"ScoredPoint")},
			},
			{
				Name:  proto.String("OperationResponse"),
				Field: []*descriptorpb.FieldDescriptorProto{fieldProto("result", 1, tBool, one, "")},
			},
		},
		Service: []*descriptorpb.ServiceDescriptorProto{
			serviceProto("Collections", pkg, [3]string{"Create", "CreateCollection", "OperationResponse"}, [3]string{"Get", "GetCollection", "GetCollectionResponse"}),
			serviceProto("Points", pkg, [3]string{"Upsert", "UpsertPoints", "OperationResponse"}, [3]string{"Search", "SearchPoints", "SearchResponse"}),
		},
	}
}

// Shorthands for building descriptors by hand.
const (
	tString  = descriptorpb.FieldDescriptorProto_TYPE_STRING
	tInt64   = descriptorpb.FieldDescriptorProto_TYPE_INT64
	tUint64  = descriptorpb.FieldDescriptorProto_TYPE_UINT64
	tDouble  = descriptorpb.FieldDescriptorProto_TYPE_DOUBLE
	tFloat   = descriptorpb.FieldDescriptorProto_TYPE_FLOAT
	tBool    = descriptorpb.FieldDescriptorProto_TYPE_BOOL
	tEnum    = descriptorpb.FieldDescriptorProto_TYPE_ENUM
	tMessage = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE
	one      = descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL
	many     = descriptorpb.FieldDescriptorProto_LABEL_REPEATED
)

// fieldProto describes a field the way protoc does, JSON name included.
func fieldProto(name string, num int32, t descriptorpb.FieldDescriptorProto_Type, label descriptorpb.FieldDescriptorProto_Label, typeName string) *descriptorpb.FieldDescriptorProto {
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

// inOneof puts a field in the message's oneof at index.
func inOneof(f *descriptorpb.FieldDescriptorProto, index int32) *descriptorpb.FieldDescriptorProto {
	f.OneofIndex = proto.Int32(index)
	return f
}

// optional makes a field proto3 optional, whose synthetic oneof is at index.
func optional(f *descriptorpb.FieldDescriptorProto, index int32) *descriptorpb.FieldDescriptorProto {
	f.Proto3Optional = proto.Bool(true)
	return inOneof(f, index)
}

func oneofs(names ...string) []*descriptorpb.OneofDescriptorProto {
	decls := make([]*descriptorpb.OneofDescriptorProto, len(names))
	for i, name := range names {
		decls[i] = &descriptorpb.OneofDescriptorProto{Name: proto.String(name)}
	}
	return decls
}

// withMap adds map<key, value> field name to msg, as protoc does: a repeated
// field of a nested, map_entry message named after the field.
func withMap(msg *descriptorpb.DescriptorProto, name string, num int32, key, value descriptorpb.FieldDescriptorProto_Type, valueType string) *descriptorpb.DescriptorProto {
	entry := mapEntryName(name)
	msg.NestedType = append(msg.NestedType, &descriptorpb.DescriptorProto{
		Name: proto.String(entry),
		Field: []*descriptorpb.FieldDescriptorProto{
			fieldProto("key", 1, key, one, ""),
			fieldProto("value", 2, value, one, valueType),
		},
		Options: &descriptorpb.MessageOptions{MapEntry: proto.Bool(true)},
	})
	msg.Field = append(msg.Field, fieldProto(name, num, tMessage, many, ".vectors.v1."+msg.GetName()+"."+entry))
	return msg
}

// mapEntryName is protoc's name for a map field's entry message: the field
// name in CamelCase, then "Entry".
func mapEntryName(field string) string {
	camel := jsonName(field)
	return string(camel[0]-'a'+'A') + camel[1:] + "Entry"
}

func enumProto(name string, values ...string) *descriptorpb.EnumDescriptorProto {
	e := &descriptorpb.EnumDescriptorProto{Name: proto.String(name)}
	for i, v := range values {
		e.Value = append(e.Value, &descriptorpb.EnumValueDescriptorProto{Name: proto.String(v), Number: proto.Int32(int32(i))})
	}
	return e
}

// serviceProto describes a service of unary methods, each given as name,
// input type, and output type within pkg.
func serviceProto(name, pkg string, methods ...[3]string) *descriptorpb.ServiceDescriptorProto {
	s := &descriptorpb.ServiceDescriptorProto{Name: proto.String(name)}
	for _, m := range methods {
		s.Method = append(s.Method, &descriptorpb.MethodDescriptorProto{
			Name: proto.String(m[0]), InputType: proto.String(pkg + m[1]), OutputType: proto.String(pkg + m[2]),
		})
	}
	return s
}

// WellKnownFiles returns the descriptors of the google.protobuf well-known
// types SnapshotsFile imports, as protoc --include_imports would add them.
func WellKnownFiles() []*descriptorpb.FileDescriptorProto {
	return []*descriptorpb.FileDescriptorProto{
		protodesc.ToFileDescriptorProto(timestamppb.File_google_protobuf_timestamp_proto),
		protodesc.ToFileDescriptorProto(durationpb.File_google_protobuf_duration_proto),
		protodesc.ToFileDescriptorProto(structpb.File_google_protobuf_struct_proto),
		protodesc.ToFileDescriptorProto(anypb.File_google_protobuf_any_proto),
		protodesc.ToFileDescriptorProto(wrapperspb.File_google_protobuf_wrappers_proto),
		protodesc.ToFileDescriptorProto(fieldmaskpb.File_google_protobuf_field_mask_proto),
	}
}

// SnapshotsFile describes a message built from the well-known types, which the
// JSON codec encodes by their own rules rather than as their fields:
//
//	syntax = "proto3";
//	package vectors.v1;
//	import "google/protobuf/{timestamp,duration,struct,any,wrappers,field_mask}.proto";
//
//	message GetSnapshot { string name = 1; }
//	message Snapshot {
//	  string name = 1; google.protobuf.Timestamp created_at = 2;
//	  google.protobuf.Duration took = 3; google.protobuf.Struct extra = 4;
//	  google.protobuf.Any detail = 5; google.protobuf.Int64Value version = 6;
//	  google.protobuf.FieldMask mask = 7; google.protobuf.Value any_value = 8;
//	}
//	service Snapshots { rpc Get(GetSnapshot) returns (Snapshot); }
//
// Load it with WellKnownFiles, which it imports.
func SnapshotsFile() *descriptorpb.FileDescriptorProto {
	const pkg = ".vectors.v1."
	return &descriptorpb.FileDescriptorProto{
		Name:    proto.String("vectors/v1/snapshots.proto"),
		Package: proto.String("vectors.v1"),
		Syntax:  proto.String("proto3"),
		Dependency: []string{
			"google/protobuf/timestamp.proto", "google/protobuf/duration.proto", "google/protobuf/struct.proto",
			"google/protobuf/any.proto", "google/protobuf/wrappers.proto", "google/protobuf/field_mask.proto",
		},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name:  proto.String("GetSnapshot"),
				Field: []*descriptorpb.FieldDescriptorProto{fieldProto("name", 1, tString, one, "")},
			},
			{
				Name: proto.String("Snapshot"),
				Field: []*descriptorpb.FieldDescriptorProto{
					fieldProto("name", 1, tString, one, ""),
					fieldProto("created_at", 2, tMessage, one, ".google.protobuf.Timestamp"),
					fieldProto("took", 3, tMessage, one, ".google.protobuf.Duration"),
					fieldProto("extra", 4, tMessage, one, ".google.protobuf.Struct"),
					fieldProto("detail", 5, tMessage, one, ".google.protobuf.Any"),
					fieldProto("version", 6, tMessage, one, ".google.protobuf.Int64Value"),
					fieldProto("mask", 7, tMessage, one, ".google.protobuf.FieldMask"),
					fieldProto("any_value", 8, tMessage, one, ".google.protobuf.Value"),
				},
			},
		},
		Service: []*descriptorpb.ServiceDescriptorProto{
			serviceProto("Snapshots", pkg, [3]string{"Get", "GetSnapshot", "Snapshot"}),
		},
	}
}
