// Package aat is the module root. It carries no runtime code; it exists to
// embed the examples/shop project so that `aat-sandbox init` can extract a
// ready-to-run AAT project without a source checkout.
package aat

import (
	"embed"
	"io/fs"
)

// shopExample embeds the tracked files of examples/shop. Patterns are listed
// explicitly (never `all:examples/shop`) so that untracked local artifacts
// such as _output/ or state.json are never shipped; dotfiles must be named
// because directory patterns skip them. embed_test.go fails when a file under
// examples/shop is missing from this list.
//
//go:embed examples/shop/README.md examples/shop/aat-project.yaml examples/shop/graph.yaml
//go:embed examples/shop/domain.yaml examples/shop/env.yaml examples/shop/openapi.yaml
//go:embed examples/shop/templates examples/shop/workflows examples/shop/layers examples/shop/plans
//go:embed examples/shop/internal examples/shop/overlays examples/shop/visualizers
//go:embed examples/shop/aat-kit.yaml examples/shop/KIT-README.md examples/shop/package-kit.sh
//go:embed examples/shop/.gitignore examples/shop/.mcp.json examples/shop/.claude/settings.json
//go:embed examples/shop/.aat-overrides.yaml.example
var shopExample embed.FS

// grpcPaymentsExample embeds the tracked files of examples/grpc-payments, the
// project that drives the sandbox's gRPC payments service. The descriptor set
// is embedded because the sandbox serves from it too: one artifact, so the
// service and the project that calls it cannot describe different APIs.
//
//go:embed examples/grpc-payments/README.md examples/grpc-payments/aat-project.yaml
//go:embed examples/grpc-payments/graph.yaml examples/grpc-payments/env.yaml
//go:embed examples/grpc-payments/payments.proto examples/grpc-payments/payments.protoset
//go:embed examples/grpc-payments/templates examples/grpc-payments/plans
//go:embed examples/grpc-payments/.gitignore
var grpcPaymentsExample embed.FS

// PaymentsDescriptorSet returns the FileDescriptorSet the sandbox's gRPC
// payments service is served from, and that examples/grpc-payments reads.
func PaymentsDescriptorSet() ([]byte, error) {
	return grpcPaymentsExample.ReadFile("examples/grpc-payments/payments.protoset")
}

// GRPCPaymentsExampleFS returns the embedded examples/grpc-payments project
// rooted at the project directory.
func GRPCPaymentsExampleFS() (fs.FS, error) {
	return fs.Sub(grpcPaymentsExample, "examples/grpc-payments")
}

// ShopExampleFS returns the embedded examples/shop project rooted at the
// project directory (aat-project.yaml at the top level).
func ShopExampleFS() (fs.FS, error) {
	return fs.Sub(shopExample, "examples/shop")
}
