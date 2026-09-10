package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeLayeredProject writes a one-node project whose recipe applies a layer
// and returns the manifest path. files replaces base files by relative path.
func writeLayeredProject(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	project := map[string]string{
		"aat-project.yaml": "name: layered\ngraph: graph.yaml\ntemplates: templates/\nworkflows: workflows/\nlayers: layers/\nplans: plans/\n",
		"graph.yaml": `version: "1.0.0"
workflows:
  - name: search
    description: Search flights
    template: workflows/search.yaml
nodes:
  SearchAir:
    description: search
    adapter: search.adapter
    inputs:
      - name: origin
        type: string
        default: JFK
    outputs:
      - name: offerId
        type: string
`,
		"templates/search.adapter.yaml": `adapter: search.adapter
protocol: http
request:
  method: GET
  path: /search/{{origin}}
response:
  extract:
    offerId: "$.offerId"
`,
		"workflows/search.yaml": "execution:\n  steps:\n    - node: SearchAir\n",
		"layers/west.yaml":      "name: west\ninputs:\n  origin: LAX\n",
		"plans/west.yaml":       "kind: recipe\nselection:\n  workflow: search\n  layers: [west]\n",
	}
	for rel, content := range files {
		project[rel] = content
	}
	for rel, content := range project {
		path := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "layers"), 0o755))
	return filepath.Join(dir, "aat-project.yaml")
}

func TestValidate_RecipeLayersResolved(t *testing.T) {
	manifest := writeLayeredProject(t, nil)

	var buf bytes.Buffer
	code := validateCommand(&validateArgs{ManifestPath: manifest}, &buf)

	assert.Equal(t, 0, code, buf.String())
	assert.Regexp(t, `Layers:\s+OK \(1 layers\)`, buf.String())
}

func TestValidate_RecipeUnknownLayerFails(t *testing.T) {
	manifest := writeLayeredProject(t, map[string]string{
		"plans/west.yaml": "kind: recipe\nselection:\n  workflow: search\n  layers: [east]\n",
	})

	var buf bytes.Buffer
	code := validateCommand(&validateArgs{ManifestPath: manifest}, &buf)

	assert.Equal(t, 1, code, buf.String())
	assert.Contains(t, buf.String(), `layer "east" not found`)
}

func TestValidate_RecipeLayersWithoutLayersDirFails(t *testing.T) {
	manifest := writeLayeredProject(t, map[string]string{
		"aat-project.yaml": "name: layered\ngraph: graph.yaml\ntemplates: templates/\nworkflows: workflows/\nplans: plans/\n",
	})

	var buf bytes.Buffer
	code := validateCommand(&validateArgs{ManifestPath: manifest}, &buf)

	assert.Equal(t, 1, code, buf.String())
	assert.Contains(t, buf.String(), "no layers directory is configured")
}

func TestValidate_LayerKeyTypoFails(t *testing.T) {
	manifest := writeLayeredProject(t, map[string]string{
		"layers/west.yaml": "name: west\ninputs:\n  SearchAir.orign: LAX\n",
	})

	var buf bytes.Buffer
	code := validateCommand(&validateArgs{ManifestPath: manifest}, &buf)

	assert.Equal(t, 1, code, buf.String())
	assert.Contains(t, buf.String(), `layer "west": input "SearchAir.orign" matches no node input`)
}

func TestPlanValidate_RecipeLayersNeedLayersDir(t *testing.T) {
	dir := filepath.Dir(writeLayeredProject(t, nil))
	args := &planValidateArgs{
		GraphPath: filepath.Join(dir, "graph.yaml"),
		PlanPath:  filepath.Join(dir, "plans", "west.yaml"),
	}
	assert.Equal(t, 1, planValidateCommand(args), "a recipe with layers needs a layers directory")

	args.LayersDir = filepath.Join(dir, "layers")
	assert.Equal(t, 0, planValidateCommand(args))
}
