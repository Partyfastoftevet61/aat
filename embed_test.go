package aat

import (
	"io/fs"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestShopExampleFS_MatchesDisk guards the explicit //go:embed patterns in
// embed.go: every file under examples/shop must be embedded, so that
// `aat-sandbox init` extracts the complete project. Local artifacts that the
// example's .gitignore excludes are skipped.
func TestShopExampleFS_MatchesDisk(t *testing.T) {
	root := filepath.Join("examples", "shop")
	local := map[string]bool{"_output": true, "state.json": true, ".aat-overrides.yaml": true, ".DS_Store": true}

	var onDisk []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if local[d.Name()] {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		onDisk = append(onDisk, filepath.ToSlash(rel))
		return nil
	})
	require.NoError(t, err)

	embedded, err := ShopExampleFS()
	require.NoError(t, err)
	var inEmbed []string
	err = fs.WalkDir(embedded, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			inEmbed = append(inEmbed, path)
		}
		return nil
	})
	require.NoError(t, err)

	sort.Strings(onDisk)
	sort.Strings(inEmbed)
	assert.Equal(t, onDisk, inEmbed, "embed.go must list every file under examples/shop")
}
