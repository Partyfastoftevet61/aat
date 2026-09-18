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

// TestEmbeddedExamples_MatchDisk guards the explicit //go:embed patterns in
// embed.go: every file under an embedded example must be embedded, so that
// `aat-sandbox init` extracts a complete project and the sandbox finds the
// descriptor set it serves from. Local artifacts that an example's own
// .gitignore excludes are skipped.
func TestEmbeddedExamples_MatchDisk(t *testing.T) {
	examples := map[string]func() (fs.FS, error){
		"shop":          ShopExampleFS,
		"grpc-payments": GRPCPaymentsExampleFS,
	}
	for name, open := range examples {
		t.Run(name, func(t *testing.T) {
			assertExampleEmbedded(t, name, open)
		})
	}
}

func assertExampleEmbedded(t *testing.T, name string, open func() (fs.FS, error)) {
	t.Helper()
	root := filepath.Join("examples", name)
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

	embedded, err := open()
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
	assert.Equal(t, onDisk, inEmbed, "embed.go must list every file under examples/"+name)
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

// TestPaymentsDescriptorSet checks the sandbox can read the descriptor set it
// serves shop.v1.Payments from — the same file examples/grpc-payments names in
// its manifest, so the service and the project that calls it cannot describe
// different APIs.
func TestPaymentsDescriptorSet(t *testing.T) {
	data, err := PaymentsDescriptorSet()
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	onDisk, err := os.ReadFile(filepath.Join("examples", "grpc-payments", "payments.protoset"))
	require.NoError(t, err)
	assert.Equal(t, onDisk, data, "the embedded descriptor set is the project's own file")
}
