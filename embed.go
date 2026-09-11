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

// ShopExampleFS returns the embedded examples/shop project rooted at the
// project directory (aat-project.yaml at the top level).
func ShopExampleFS() (fs.FS, error) {
	return fs.Sub(shopExample, "examples/shop")
}
