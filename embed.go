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
// because directory patterns skip them.
//
//go:embed examples/shop/openapi.yaml examples/shop/README.md
var shopExample embed.FS

// ShopExampleFS returns the embedded examples/shop project rooted at the
// project directory (openapi.yaml, and once the project lands, aat-project.yaml
// at the top level).
func ShopExampleFS() (fs.FS, error) {
	return fs.Sub(shopExample, "examples/shop")
}
