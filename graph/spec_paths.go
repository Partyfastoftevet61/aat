package graph

import "path/filepath"

// SpecPath is one spec file a project reads: Ref is the name it is written
// under — a graph's default, a node's override, or a project-level entry — and
// Path is where that name resolved to on disk.
type SpecPath struct {
	Ref  string
	Path string
}

// ResolveSpecPaths merges the spec files a graph names with the ones a project
// names.
//
// graphPaths are refs written in the graph, resolved against graphDir.
// projectPaths arrive already resolved against the manifest's directory, which
// config.LoadManifest does, and are used exactly as given. The distinction is
// carried by which argument a path arrives in and is never inferred from
// whether it looks absolute: a manifest loaded through a relative path yields
// relative-but-already-resolved entries, and joining those onto graphDir
// doubles the prefix (examples/shop/examples/shop/openapi.yaml).
//
// Graph refs come first, so a file named in both places keeps the ref a node
// looks it up by. The result holds one entry per distinct Ref, because every
// ref a node may name has to be loadable; SpecPathList collapses it further for
// consumers that merge everything into a single registry.
func ResolveSpecPaths(graphPaths []string, graphDir string, projectPaths []string) []SpecPath {
	var out []SpecPath
	seen := make(map[string]bool)

	add := func(ref, path string) {
		if ref == "" || seen[ref] {
			return
		}
		seen[ref] = true
		out = append(out, SpecPath{Ref: ref, Path: path})
	}

	for _, ref := range graphPaths {
		resolved := ref
		if !filepath.IsAbs(ref) {
			resolved = filepath.Join(graphDir, ref)
		}
		add(ref, resolved)
	}
	for _, ref := range projectPaths {
		add(ref, ref)
	}
	return out
}

// SpecPathList returns the distinct file paths of a merged set, in order, for a
// consumer that loads them all into one registry. Two refs can resolve to the
// same file — a project and a graph naming it both — and a loader is entitled
// to reject the same path twice.
func SpecPathList(paths []SpecPath) []string {
	var out []string
	seen := make(map[string]bool)
	for _, sp := range paths {
		if sp.Path == "" || seen[sp.Path] {
			continue
		}
		seen[sp.Path] = true
		out = append(out, sp.Path)
	}
	return out
}
