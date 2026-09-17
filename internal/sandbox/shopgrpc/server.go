package shopgrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/gburgyan/aat/internal/grpcstatus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// serviceName is the service the descriptor set declares.
const serviceName = "shop.v1.Payments"

// apiKeyHeader is the credential the payments API takes. Over gRPC it arrives
// as metadata, which is what a header is on this side of the wire.
const apiKeyHeader = "x-api-key"

// regionHeader selects the sandbox region. The HTTP API puts the region in the
// path; a gRPC method has no path, so it travels as metadata instead, and
// defaults to the region the server was built with.
const regionHeader = "x-region"

// Server serves shop.v1.Payments over gRPC by forwarding to the payments HTTP
// handler.
type Server struct {
	handler http.Handler
	files   *protoregistry.Files
	types   *dynamicpb.Types
	region  string
}

// New builds a server from a FileDescriptorSet — the artifact
// `protoc --descriptor_set_out` writes — and the payments HTTP handler it
// fronts. region is the sandbox region its requests are routed to.
func New(descriptorSet []byte, payments http.Handler, region string) (*Server, error) {
	var fds descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(descriptorSet, &fds); err != nil {
		return nil, fmt.Errorf("parsing the payments descriptor set: %w", err)
	}
	files, err := protodesc.NewFiles(&fds)
	if err != nil {
		return nil, fmt.Errorf("resolving the payments descriptors: %w", err)
	}
	if _, err := files.FindDescriptorByName(serviceName); err != nil {
		return nil, fmt.Errorf("the descriptor set declares no %s: %w", serviceName, err)
	}
	return &Server{handler: payments, files: files, types: dynamicpb.NewTypes(files), region: region}, nil
}

// Register adds the service to a gRPC server. Methods are taken from the
// descriptors rather than from generated code, so adding an RPC to the .proto
// and regenerating is enough for it to be served.
func (s *Server) Register(srv *grpc.Server) error {
	desc, err := s.files.FindDescriptorByName(serviceName)
	if err != nil {
		return err
	}
	sd, ok := desc.(protoreflect.ServiceDescriptor)
	if !ok {
		return fmt.Errorf("%s is not a service", serviceName)
	}

	methods := sd.Methods()
	grpcDesc := &grpc.ServiceDesc{
		ServiceName: serviceName,
		HandlerType: (*any)(nil),
		Metadata:    sd.ParentFile().Path(),
	}
	for i := 0; i < methods.Len(); i++ {
		md := methods.Get(i)
		if md.IsStreamingClient() || md.IsStreamingServer() {
			return fmt.Errorf("%s.%s streams; this sandbox serves unary methods", serviceName, md.Name())
		}
		grpcDesc.Methods = append(grpcDesc.Methods, grpc.MethodDesc{
			MethodName: string(md.Name()),
			Handler: func(_ any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
				in := dynamicpb.NewMessage(md.Input())
				if err := dec(in); err != nil {
					return nil, status.Error(codes.InvalidArgument, err.Error())
				}
				return s.call(ctx, md, in)
			},
		})
	}
	srv.RegisterService(grpcDesc, struct{}{})
	return nil
}

// call forwards one RPC to the payments handler and turns its reply back into
// the method's response message.
func (s *Server) call(ctx context.Context, md protoreflect.MethodDescriptor, in *dynamicpb.Message) (any, error) {
	body, err := s.requestBody(in)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	incoming, _ := metadata.FromIncomingContext(ctx)

	path, err := s.httpPath(md, s.regionFor(incoming))
	if err != nil {
		return nil, err
	}

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	if keys := incoming.Get(apiKeyHeader); len(keys) > 0 {
		req.Header.Set("X-API-Key", keys[0])
	}
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req.WithContext(ctx))

	if rec.Code >= 400 {
		return nil, s.statusFromError(rec.Code, rec.Body.Bytes())
	}
	return s.responseMessage(md, rec.Body.Bytes())
}

// regionFor returns the region a call is for: the one its metadata names, or
// the server's default.
func (s *Server) regionFor(md metadata.MD) string {
	if values := md.Get(regionHeader); len(values) > 0 && values[0] != "" {
		return values[0]
	}
	return s.region
}

// httpPath is the payments route a method forwards to.
func (s *Server) httpPath(md protoreflect.MethodDescriptor, region string) (string, error) {
	switch md.Name() {
	case "Charge":
		return "/" + region + "/v1/payments/charges", nil
	case "Refund":
		return "/" + region + "/v1/payments/refunds", nil
	default:
		return "", status.Errorf(codes.Unimplemented, "%s.%s has no HTTP route in this sandbox", serviceName, md.Name())
	}
}

// requestBody renders a request message as the JSON the payments API takes.
//
// It is not protojson's output: a 64-bit integer is a string in proto3 JSON
// and a number in the shop's API, and an unset field must stay out of the body
// so the handler's own "required" errors still fire.
func (s *Server) requestBody(in *dynamicpb.Message) ([]byte, error) {
	out := map[string]any{}
	fields := in.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if !in.Has(fd) {
			continue
		}
		switch fd.Kind() {
		case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
			out[fd.JSONName()] = in.Get(fd).Int()
		case protoreflect.StringKind:
			out[fd.JSONName()] = in.Get(fd).String()
		default:
			out[fd.JSONName()] = in.Get(fd).Interface()
		}
	}
	return json.Marshal(out)
}

// responseMessage builds a method's response from the payments API's JSON.
// Unknown fields are discarded rather than refused: the HTTP API may carry
// more than the proto declares, and that is the sandbox's problem to tolerate,
// not the caller's.
func (s *Server) responseMessage(md protoreflect.MethodDescriptor, body []byte) (any, error) {
	msg := dynamicpb.NewMessage(md.Output())
	opts := protojson.UnmarshalOptions{DiscardUnknown: true, Resolver: s.types}
	if err := opts.Unmarshal(body, msg); err != nil {
		return nil, status.Errorf(codes.Internal, "encoding the %s reply: %v", md.Name(), err)
	}
	return msg, nil
}

// statusFromError turns the shop's error envelope into a gRPC status. The
// shop answers {"error":{"code","message"}}; the code becomes the status
// message's prefix so a caller keeps the stable code the API documents.
func (s *Server) statusFromError(httpStatus int, body []byte) error {
	code := codes.Code(grpcstatus.FromHTTPStatus(httpStatus))

	var envelope struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || envelope.Error.Message == "" {
		return status.Error(code, strings.TrimSpace(string(body)))
	}
	if envelope.Error.Code == "" {
		return status.Error(code, envelope.Error.Message)
	}
	return status.Errorf(code, "%s: %s", envelope.Error.Code, envelope.Error.Message)
}

// BannerLine is the line the sandbox prints for its gRPC listener, alongside
// the HTTP ones. The caller supplies the demo key: shop and shopgrpc are both
// leaf packages, so neither imports the other.
func BannerLine(addr, apiKey string) string {
	return fmt.Sprintf("aat-sandbox: payments gRPC  grpc://%s   (service %s, metadata %s: %s)",
		addr, serviceName, apiKeyHeader, apiKey)
}
