package adapter

import (
	"fmt"

	"github.com/gburgyan/aat/internal/protoreg"
)

// ExecutorFactory builds the executor a target needs, choosing by scheme: a
// grpc:// or grpcs:// target gets a gRPC executor, and everything else an HTTP
// one. A run holds one factory, so every gRPC executor it builds shares the
// same connection pool and descriptors.
//
// The zero value builds HTTP executors, which is what a project with no gRPC
// nodes needs.
type ExecutorFactory struct {
	// Registry supplies the descriptors gRPC requests are built and read
	// with. A nil registry means a gRPC target is an error naming what to do
	// about it.
	Registry *protoreg.Registry
	// TLS secures grpcs:// connections. The zero value verifies against the
	// system roots.
	TLS TLSConfig

	pool *ConnPool
}

// NewExecutorFactory returns a factory that builds gRPC executors against reg.
// Pass a nil registry for a project with no gRPC nodes.
func NewExecutorFactory(reg *protoreg.Registry) *ExecutorFactory {
	return &ExecutorFactory{Registry: reg}
}

// WithTLS sets how grpcs:// connections are secured.
func (f *ExecutorFactory) WithTLS(cfg TLSConfig) *ExecutorFactory {
	f.TLS = cfg
	return f
}

// For returns an executor for the target.
func (f *ExecutorFactory) For(target string) (Executor, error) {
	if !IsGRPCTarget(target) {
		return NewHTTPExecutor(target), nil
	}
	if f.Registry == nil {
		return nil, fmt.Errorf("target %s is a gRPC service but the project loads no descriptors: name a descriptor set with proto: in aat-project.yaml or the graph", target)
	}
	if f.pool == nil {
		f.pool = NewConnPool()
	}
	return NewGRPCExecutor(target, f.pool, f.Registry, f.TLS)
}

// Close releases the connections every gRPC executor the factory built shares.
// The HTTP executors it built are closed by whoever holds them.
func (f *ExecutorFactory) Close() error {
	if f.pool == nil {
		return nil
	}
	return f.pool.Close()
}
