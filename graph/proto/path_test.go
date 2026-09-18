package proto

import (
	"testing"

	"github.com/gburgyan/aat/graph"
	"github.com/gburgyan/aat/internal/protoreg"
	"github.com/gburgyan/aat/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const pointsRef = "points.protoset"

// pointsGraph is a graph whose one node calls method on the vectors.v1
// fixture, with an output for each path.
func pointsGraph(method string, paths map[string]string) (*graph.Graph, OutputPaths) {
	node := &graph.Node{Proto: &graph.ProtoRef{Service: "vectors.v1." + serviceOf(method), Method: method}}
	for name := range paths {
		node.Outputs = append(node.Outputs, graph.Output{Name: name, Type: "string"})
	}
	return &graph.Graph{Proto: pointsRef, Nodes: map[string]*graph.Node{"n": node}}, OutputPaths{"n": paths}
}

func serviceOf(method string) string {
	if method == "Create" || method == "Get" {
		return "Collections"
	}
	return "Points"
}

func pointsValidator(t *testing.T) *Validator {
	t.Helper()
	v := NewValidator()
	require.NoError(t, v.LoadSpec(pointsRef, testutil.WriteDescriptorSet(t, pointsRef, testutil.PointsFile())))
	return v
}

// The response paths a Qdrant-shaped API's extract rules take: through map
// fields keyed by anything, into a forked Value's oneof members, and through
// oneofs, which encode as their member's name.
var pointsPathCases = []struct {
	method  string
	path    string
	problem string // "" when the path resolves
}{
	{method: "Get", path: "result.config.metadata.project.stringValue"},
	{method: "Get", path: "result.config.metadata.key.stringValue"}, // "key" is just a key here
	{method: "Get", path: `result.config.metadata.my\.dotted.integerValue`},
	{method: "Get", path: "result.config.metadata.nested.structValue.fields.inner.listValue.values.0.integerValue"},
	{method: "Get", path: "result.config.vectorsConfig.params.size"},
	{method: "Get", path: "result.config.vectorsConfig.paramsMap.map.dense.distance"},
	{method: "Get", path: "result.pointsCount"},
	{method: "Search", path: "result.0.payload.city.stringValue"},
	{method: "Search", path: "result.#.payload.city.stringValue"},
	{method: "Search", path: "result.0.id.num"},
	{method: "Search", path: "result.0.tagsByRank.7"},
	{method: "Search", path: "result.0.payload"},
	{method: "Search", path: "result.0.payload.city"},
	{method: "Get", path: "result.config.metadata.project.nope", problem: `vectors.v1.Value does not declare "nope"`},
	{method: "Get", path: "result.config.metadata.project.stringValue.x", problem: `"stringValue" is a string, which has nothing below it`},
	{method: "Search", path: "result.0.payload.city.key", problem: `vectors.v1.Value does not declare "key"`},
	{method: "Search", path: "result.0.tagsByRank.7.x", problem: `"7" is a string, which has nothing below it`},
	{method: "Search", path: "result.0.id.nums", problem: `vectors.v1.PointId does not declare "nums"`},
	{method: "Search", path: "result.0.id.NUM", problem: `vectors.v1.PointId does not declare "NUM" (did you mean "num"?)`},
}

func TestValidate_OutputPathsThroughMapsAndOneofs(t *testing.T) {
	for _, tt := range pointsPathCases {
		t.Run(tt.path, func(t *testing.T) {
			g, paths := pointsGraph(tt.method, map[string]string{"out": tt.path})
			result := pointsValidator(t).WithOutputPaths(paths).Validate(g)
			if tt.problem == "" {
				assert.False(t, result.HasIssues(), messages(t, result))
				return
			}
			require.Len(t, result.Issues, 1)
			assert.Equal(t, `output "out" reads "`+tt.path+`": `+tt.problem, result.Issues[0].Message)
		})
	}
}

// What the codec writes is what the validator reasons about: every path the
// validator accepts reads something from a response encoded by protoreg, and
// every path it rejects reads nothing.
func TestPaths_AgreeWithTheCodec(t *testing.T) {
	reg, err := protoreg.LoadDescriptorSets(testutil.WriteDescriptorSet(t, pointsRef, testutil.PointsFile()))
	require.NoError(t, err)

	responses := map[string]string{
		"Get": `{"result": {"config": {
			"vectorsConfig": {"paramsMap": {"map": {"dense": {"size": "4", "distance": "Cosine"}}}},
			"metadata": {
				"project": {"stringValue": "aat-qdrant"},
				"key": {"stringValue": "a key named key"},
				"my.dotted": {"integerValue": "7"},
				"nested": {"structValue": {"fields": {"inner": {"listValue": {"values": [{"integerValue": "1"}]}}}}}
			}}, "pointsCount": "0"}}`,
		"Search": `{"result": [{"id": {"num": "0"}, "score": 0.5,
			"payload": {"city": {"stringValue": "Berlin"}},
			"tagsByRank": {"7": "seventh"}}]}`,
	}
	encoded := make(map[string]string, len(responses))
	for method, doc := range responses {
		md, err := reg.Method("vectors.v1."+serviceOf(method), method)
		require.NoError(t, err)
		msg, err := reg.JSONToMessage(md.Output(), []byte(doc))
		require.NoError(t, err, method)
		out, err := reg.MessageToJSON(msg)
		require.NoError(t, err, method)
		encoded[method] = string(out)
	}

	for _, tt := range pointsPathCases {
		// paramsMap is set in the sample, so params is absent by design, and
		// "result.config.vectorsConfig.params.size" is valid but unset.
		if tt.path == "result.config.vectorsConfig.params.size" {
			continue
		}
		t.Run(tt.path, func(t *testing.T) {
			found := gjson.Get(encoded[tt.method], tt.path).Exists()
			assert.Equal(t, tt.problem == "", found, "validator verdict %q, codec output %s", tt.problem, encoded[tt.method])
		})
	}
}

func TestValidate_InputsAreCheckedWhereTheTemplatePlacesThem(t *testing.T) {
	create := func(inputs ...string) *graph.Graph {
		node := &graph.Node{Proto: &graph.ProtoRef{Service: "vectors.v1.Collections", Method: "Create"}}
		for _, in := range inputs {
			node.Inputs = append(node.Inputs, graph.Input{Name: in, Type: "string"})
		}
		return &graph.Graph{Proto: pointsRef, Nodes: map[string]*graph.Node{"n": node}}
	}

	tests := []struct {
		name       string
		placements map[string][]string
		inputs     []string
		want       string // the issues, one per line
	}{
		{
			name: "nested fields, oneof members, and map values resolve",
			placements: map[string][]string{
				"collectionName": {"collectionName"},
				"vectorSize":     {"vectorsConfig.params.size"},
				"denseSize":      {"vectorsConfig.paramsMap.map.dense.size"},
				"project":        {"metadata.project.stringValue"},
				"region":         {`labels.my\.region`},
				"anyKey":         {"labels.*"},
			},
			inputs: []string{"collectionName", "vectorSize", "denseSize", "project", "region", "anyKey"},
		},
		{
			name:       "the proto name spelling is accepted in a request",
			placements: map[string][]string{"vectorSize": {"vectors_config.params.size"}},
			inputs:     []string{"vectorSize"},
		},
		{
			name:       "an input sent only as metadata has nothing to check",
			placements: map[string][]string{"tenant": {}},
			inputs:     []string{"tenant"},
		},
		{
			name:       "a misspelled nested key is an error",
			placements: map[string][]string{"vectorSize": {"vectorsConfig.Params.size"}},
			inputs:     []string{"vectorSize"},
			want:       `error: input "vectorSize" is sent at "vectorsConfig.Params.size": vectors.v1.VectorsConfig does not declare "Params" (did you mean "params"?)` + "\n",
		},
		{
			name:       "a field below a scalar is an error",
			placements: map[string][]string{"collectionName": {"collectionName.x"}},
			inputs:     []string{"collectionName"},
			want:       `error: input "collectionName" is sent at "collectionName.x": "collectionName" is a string, which has nothing below it` + "\n",
		},
		{
			name:       "an input the template doesn't place is still checked by name",
			placements: map[string][]string{},
			inputs:     []string{"distance"},
			want:       `warning: input "distance" is not a field of vectors.v1.CreateCollection; it must reach the request another way, such as metadata` + "\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := pointsValidator(t).WithInputPaths(InputPaths{"n": tt.placements})
			assert.Equal(t, tt.want, messages(t, v.Validate(create(tt.inputs...))))
		})
	}
}
