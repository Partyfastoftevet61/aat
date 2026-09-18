package protoreg

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// Registry holds the descriptors loaded from one or more descriptor sets.
//
// Files is never protoregistry.GlobalFiles: registering a path twice panics,
// and the protobuf runtime already holds the well-known types there. Every
// load builds its own.
type Registry struct {
	// Files indexes every descriptor loaded, by full name and by path.
	Files *protoregistry.Files
	// Types resolves a message by name. protojson needs it to encode a
	// google.protobuf.Any, whose payload names its type at runtime; a
	// protoregistry.Files is not itself a resolver.
	Types *dynamicpb.Types
}

// LoadDescriptorSets reads FileDescriptorSet files and indexes what they
// declare. Sets may depend on each other's files, so they are merged before
// resolution and a file declared twice is kept once.
func LoadDescriptorSets(paths ...string) (*Registry, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("no descriptor set given")
	}

	merged := &descriptorpb.FileDescriptorSet{}
	seen := make(map[string]string) // proto path -> the descriptor set that declared it
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading descriptor set %s: %w", path, err)
		}
		var fds descriptorpb.FileDescriptorSet
		if err := proto.Unmarshal(data, &fds); err != nil {
			return nil, fmt.Errorf("parsing descriptor set %s: %w: a descriptor set is the binary output of `protoc --descriptor_set_out` or `buf build -o`, not .proto source", path, err)
		}
		if len(fds.File) == 0 {
			return nil, fmt.Errorf("descriptor set %s declares no files", path)
		}
		for _, file := range fds.File {
			name := file.GetName()
			if from, dup := seen[name]; dup {
				// Sets built from overlapping imports repeat their
				// dependencies; the descriptors are identical, so keep one.
				if from != path {
					continue
				}
				return nil, fmt.Errorf("descriptor set %s declares %s twice", path, name)
			}
			seen[name] = path
			merged.File = append(merged.File, file)
		}
	}

	files, err := protodesc.NewFiles(merged)
	if err != nil {
		return nil, fmt.Errorf("resolving descriptors from %s: %w", strings.Join(paths, ", "), err)
	}
	return &Registry{Files: files, Types: dynamicpb.NewTypes(files)}, nil
}

// Method returns the descriptor for a fully qualified method, written either as
// "pkg.Service/Method" or "pkg.Service.Method".
func (r *Registry) Method(service, method string) (protoreflect.MethodDescriptor, error) {
	desc, err := r.Files.FindDescriptorByName(protoreflect.FullName(service))
	if err != nil {
		return nil, fmt.Errorf("service %q not found: %w%s", service, err, r.didYouMeanService(service))
	}
	sd, ok := desc.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil, fmt.Errorf("%q is a %s, not a service", service, descriptorKind(desc))
	}
	md := sd.Methods().ByName(protoreflect.Name(method))
	if md == nil {
		return nil, fmt.Errorf("service %q has no method %q%s", service, method, didYouMean(method, methodNames(sd)))
	}
	return md, nil
}

// Services returns every service's full name, in sorted order.
func (r *Registry) Services() []string {
	var names []string
	r.Files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		services := fd.Services()
		for i := 0; i < services.Len(); i++ {
			names = append(names, string(services.Get(i).FullName()))
		}
		return true
	})
	sort.Strings(names)
	return names
}

func (r *Registry) didYouMeanService(service string) string {
	return didYouMean(service, r.Services())
}

func methodNames(sd protoreflect.ServiceDescriptor) []string {
	methods := sd.Methods()
	names := make([]string, 0, methods.Len())
	for i := 0; i < methods.Len(); i++ {
		names = append(names, string(methods.Get(i).Name()))
	}
	return names
}

// didYouMean suggests the candidate that differs from name only in case, or
// that contains it, so a near miss names its fix instead of listing everything.
func didYouMean(name string, candidates []string) string {
	for _, c := range candidates {
		if c != name && strings.EqualFold(c, name) {
			return fmt.Sprintf(" (did you mean %q?)", c)
		}
	}
	for _, c := range candidates {
		if short := c[strings.LastIndex(c, ".")+1:]; strings.EqualFold(short, name) {
			return fmt.Sprintf(" (did you mean %q?)", c)
		}
	}
	return ""
}

func descriptorKind(desc protoreflect.Descriptor) string {
	switch desc.(type) {
	case protoreflect.MessageDescriptor:
		return "message"
	case protoreflect.EnumDescriptor:
		return "enum"
	case protoreflect.FieldDescriptor:
		return "field"
	default:
		return "descriptor"
	}
}
