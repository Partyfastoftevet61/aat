package aat

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestShopExampleFS_MatchesDisk guards the explicit //go:embed patterns in
// embed.go: every file under examples/shop must be embedded, so that
// `aat-sandbox init` extracts the complete project. Local artifacts that the
// example's own .gitignore excludes are skipped.
func TestShopExampleFS_MatchesDisk(t *testing.T) {
	root := filepath.Join("examples", "shop")
	ignored := exampleIgnoreRules(t, filepath.Join(root, ".gitignore"))

	var onDisk []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ignored(d.Name(), d.IsDir()) {
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

// exampleIgnoreRules reads a .gitignore made of simple name patterns (a
// trailing "/" matches directories only) and returns a matcher. Patterns this
// test cannot interpret, such as negations or paths with a "/" inside, fail the
// test so the example's ignore file stays simple enough to mirror here.
func exampleIgnoreRules(t *testing.T, path string) func(name string, isDir bool) bool {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	type rule struct {
		pattern string
		dirOnly bool
	}
	var rules []rule
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		dirOnly := strings.HasSuffix(line, "/")
		pattern := strings.TrimSuffix(line, "/")
		require.False(t, strings.HasPrefix(pattern, "!") || strings.Contains(pattern, "/"),
			"unsupported .gitignore pattern %q in %s", line, path)
		rules = append(rules, rule{pattern: pattern, dirOnly: dirOnly})
	}

	return func(name string, isDir bool) bool {
		for _, r := range rules {
			if r.dirOnly && !isDir {
				continue
			}
			if ok, _ := filepath.Match(r.pattern, name); ok {
				return true
			}
		}
		return false
	}
}
