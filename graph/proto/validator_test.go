package proto

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gburgyan/aat/graph"
	"github.com/gburgyan/aat/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const descriptorRef = "shop.protoset"

// loadedValidator returns a validator holding the shop descriptors under
// descriptorRef, as CollectSpecPaths and LoadSpec would leave it.
func loadedValidator(t *testing.T) *Validator {
	t.Helper()
	path := testutil.WriteDescriptorSet(t, "shop.protoset", testutil.ShopFile())
	v := NewValidator()
	require.NoError(t, v.LoadSpec(descriptorRef, path))
	return v
}

// graphWith returns a one-node graph carrying the given proto ref.
func graphWith(node *graph.Node) *graph.Graph {
	return &graph.Graph{Proto: descriptorRef, Nodes: map[string]*graph.Node{"createCart": node}}
}

func messages(t *testing.T, result *graph.SpecValidationResult) string {
	t.Helper()
	var b strings.Builder
	for _, issue := range result.Issues {
		b.WriteString(string(issue.Severity) + ": " + issue.Message + "\n")
	}
	return b.String()
}

func TestValidate_ValidNode(t *testing.T) {
	result := loadedValidator(t).Validate(graphWith(&graph.Node{
		Adapter: "createCart",
		Proto:   &graph.ProtoRef{Service: "shop.v1.Carts", Method: "CreateCart"},
		Inputs:  []graph.Input{{Name: "customer_id", Type: "string"}, {Name: "currency", Type: "string"}},
		Outputs: []graph.Output{{Name: "cartId", Type: "string"}, {Name: "subtotal", Type: "integer"}},
	}))
	assert.False(t, result.HasIssues(), messages(t, result))
}

func TestValidate_InputsAcceptEitherSpelling(t *testing.T) {
	// A template may be written with proto names or JSON names, and
	// protojson.Unmarshal accepts both, so the validator must too.
	result := loadedValidator(t).Validate(graphWith(&graph.Node{
		Proto:  &graph.ProtoRef{Service: "shop.v1.Carts", Method: "CreateCart"},
		Inputs: []graph.Input{{Name: "customerId", Type: "string"}},
	}))
	assert.False(t, result.HasIssues(), messages(t, result))
}

func TestValidate_UnknownServiceOrMethod(t *testing.T) {
	t.Run("unknown service", func(t *testing.T) {
		result := loadedValidator(t).Validate(graphWith(&graph.Node{
			Proto: &graph.ProtoRef{Service: "shop.v1.Orders", Method: "Create"},
		}))
		require.True(t, result.HasErrors())
		assert.Contains(t, messages(t, result), `service "shop.v1.Orders" not found`)
	})

	t.Run("misspelled method suggests the fix", func(t *testing.T) {
		result := loadedValidator(t).Validate(graphWith(&graph.Node{
			Proto: &graph.ProtoRef{Service: "shop.v1.Carts", Method: "createCart"},
		}))
		require.True(t, result.HasErrors())
		assert.Contains(t, messages(t, result), `did you mean "CreateCart"?`)
	})
}

func TestValidate_StreamingMethodsAreRejected(t *testing.T) {
	// Unary only: a step is one request and one response.
	result := loadedValidator(t).Validate(graphWith(&graph.Node{
		Proto: &graph.ProtoRef{Service: "shop.v1.Carts", Method: "WatchCart"},
	}))
	require.True(t, result.HasErrors())
	assert.Contains(t, messages(t, result), "a server-streaming method")
	assert.Contains(t, messages(t, result), "aat runs unary methods")
}

func TestValidate_UnknownOutput(t *testing.T) {
	result := loadedValidator(t).Validate(graphWith(&graph.Node{
		Proto:   &graph.ProtoRef{Service: "shop.v1.Carts", Method: "CreateCart"},
		Outputs: []graph.Output{{Name: "orderId", Type: "string"}},
	}))
	require.True(t, result.HasErrors())
	assert.Contains(t, messages(t, result), `output "orderId" reads "orderId": shop.v1.Cart does not declare "orderId"`)
}

func TestValidate_OutputPathWrittenWithTheProtoName(t *testing.T) {
	// The papercut this check exists for: both spellings resolve against the
	// descriptor, but the response is encoded under the JSON name, so
	// "cart_id" would read nothing at runtime.
	v := loadedValidator(t).WithOutputPaths(OutputPaths{
		"createCart": {"cartId": "cart_id"},
	})
	result := v.Validate(graphWith(&graph.Node{
		Proto:   &graph.ProtoRef{Service: "shop.v1.Carts", Method: "CreateCart"},
		Outputs: []graph.Output{{Name: "cartId", Type: "string"}},
	}))
	require.True(t, result.HasErrors())
	assert.Contains(t, messages(t, result), `but the response encodes that field as "cartId"`)
}

func TestValidate_OutputPathsThroughMessagesAndLists(t *testing.T) {
	v := loadedValidator(t).WithOutputPaths(OutputPaths{
		"createCart": {
			"firstSku": "items.0.sku",
			"allSkus":  "items.#.sku",
			"computed": "", // produced by a response transform
		},
	})
	result := v.Validate(graphWith(&graph.Node{
		Proto: &graph.ProtoRef{Service: "shop.v1.Carts", Method: "CreateCart"},
		Outputs: []graph.Output{
			{Name: "firstSku", Type: "string"},
			{Name: "allSkus", Type: "string[]"},
			{Name: "computed", Type: "string"},
		},
	}))
	assert.False(t, result.HasIssues(), messages(t, result))
}

func TestValidate_OutputPathsFollowGJSON(t *testing.T) {
	// Paths are read the way gjson reads them, not split on every dot: a query
	// is one segment, and a modifier or a pipe ends what can be checked. Each
	// path that fails names where it stopped making sense.
	tests := []struct {
		path    string
		problem string // "" when the path resolves
	}{
		{path: "items.0.sku"},
		{path: "items.#.sku"},
		{path: "items.#"},
		{path: `items.#(sku=="a.b").quantity`},
		{path: `items.#(quantity>1)#.sku`},
		{path: "cartId|@tostr"},
		{path: "items.@reverse"},
		{path: "items.#.sku|@join"},
		{path: "items.sku", problem: `"items" is a list, so an index, # or a query comes next, not "sku"`},
		{path: `items.#(sku=="a").nope`, problem: `shop.v1.LineItem does not declare "nope"`},
		{path: "subtotal.x", problem: `"subtotal" is an int64, which has nothing below it`},
		{path: "cartId.#", problem: `"cartId" is a string, which has nothing below it`},
		{path: "status.0", problem: `"status" is an enum, which has nothing below it`},
		{path: "items.0.skew", problem: `shop.v1.LineItem does not declare "skew"`},
		{path: "cartIdd|@tostr", problem: `shop.v1.Cart does not declare "cartIdd"`},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			v := loadedValidator(t).WithOutputPaths(OutputPaths{"createCart": {"out": tt.path}})
			result := v.Validate(graphWith(&graph.Node{
				Proto:   &graph.ProtoRef{Service: "shop.v1.Carts", Method: "CreateCart"},
				Outputs: []graph.Output{{Name: "out", Type: "string"}},
			}))
			if tt.problem == "" {
				assert.False(t, result.HasIssues(), messages(t, result))
				return
			}
			assert.Equal(t, fmt.Sprintf("error: output \"out\" reads %q: %s\n", tt.path, tt.problem), messages(t, result))
		})
	}
}

func TestValidate_OutputPathIntoAnUnknownField(t *testing.T) {
	v := loadedValidator(t).WithOutputPaths(OutputPaths{
		"createCart": {"sku": "items.0.skew"},
	})
	result := v.Validate(graphWith(&graph.Node{
		Proto:   &graph.ProtoRef{Service: "shop.v1.Carts", Method: "CreateCart"},
		Outputs: []graph.Output{{Name: "sku", Type: "string"}},
	}))
	require.True(t, result.HasErrors())
	assert.Contains(t, messages(t, result), `does not declare`)
}

func TestValidate_UnknownInputIsAWarningUnlessItIsAMisspelling(t *testing.T) {
	t.Run("misspelling is an error with a suggestion", func(t *testing.T) {
		result := loadedValidator(t).Validate(graphWith(&graph.Node{
			Proto:  &graph.ProtoRef{Service: "shop.v1.Carts", Method: "CreateCart"},
			Inputs: []graph.Input{{Name: "customerID", Type: "string"}},
		}))
		require.True(t, result.HasErrors())
		assert.Contains(t, messages(t, result), `did you mean "customerId"?`)
	})

	t.Run("an input the message does not declare only warns", func(t *testing.T) {
		// It may legitimately travel as metadata, or build another field.
		result := loadedValidator(t).Validate(graphWith(&graph.Node{
			Proto:  &graph.ProtoRef{Service: "shop.v1.Carts", Method: "CreateCart"},
			Inputs: []graph.Input{{Name: "tenant", Type: "string"}},
		}))
		assert.False(t, result.HasErrors(), messages(t, result))
		assert.True(t, result.HasIssues())
		assert.Contains(t, messages(t, result), "such as metadata")
	})
}

func TestValidate_NodeWithBothContracts(t *testing.T) {
	result := loadedValidator(t).Validate(graphWith(&graph.Node{
		OAS:   &graph.OASRef{OperationID: "createCart"},
		Proto: &graph.ProtoRef{Service: "shop.v1.Carts", Method: "CreateCart"},
	}))
	require.True(t, result.HasErrors())
	assert.Contains(t, messages(t, result), "has both oas and proto")
}

func TestValidate_NodeWithoutAProtoRefIsSkipped(t *testing.T) {
	result := loadedValidator(t).Validate(graphWith(&graph.Node{
		OAS: &graph.OASRef{OperationID: "createCart"},
	}))
	assert.False(t, result.HasIssues(), messages(t, result))
}

func TestValidate_NoDescriptorSetNamed(t *testing.T) {
	v := NewValidator()
	result := v.Validate(&graph.Graph{Nodes: map[string]*graph.Node{
		"createCart": {Proto: &graph.ProtoRef{Service: "shop.v1.Carts", Method: "CreateCart"}},
	}})
	require.True(t, result.HasErrors())
	assert.Contains(t, messages(t, result), "needs a descriptor set")
}

func TestValidate_ProjectDescriptorFallback(t *testing.T) {
	// The graph names no descriptor set; aat-project.yaml does.
	path := testutil.WriteDescriptorSet(t, "shop.protoset", testutil.ShopFile())
	v := NewValidator().WithProjectDescriptors([]string{path})
	require.NoError(t, v.LoadSpec(path, path))

	result := v.Validate(&graph.Graph{Nodes: map[string]*graph.Node{
		"createCart": {Proto: &graph.ProtoRef{Service: "shop.v1.Carts", Method: "CreateCart"}},
	}})
	assert.False(t, result.HasIssues(), messages(t, result))
}

func TestValidate_ProjectDescriptorStillCatchesAnUnknownMethod(t *testing.T) {
	path := testutil.WriteDescriptorSet(t, "shop.protoset", testutil.ShopFile())
	v := NewValidator().WithProjectDescriptors([]string{path})
	require.NoError(t, v.LoadSpec(path, path))

	result := v.Validate(&graph.Graph{Nodes: map[string]*graph.Node{
		"createCart": {Proto: &graph.ProtoRef{Service: "shop.v1.Carts", Method: "Nope"}},
	}})
	require.True(t, result.HasErrors(), messages(t, result))
	assert.Contains(t, messages(t, result), "Nope")
}

func TestValidate_GraphDescriptorWinsOverProject(t *testing.T) {
	// The graph's ref is the one checked; a project ref that has never been
	// loaded is not consulted and so cannot mask the graph's error.
	v := loadedValidator(t).WithProjectDescriptors([]string{"unloaded.protoset"})

	result := v.Validate(graphWith(&graph.Node{
		Proto: &graph.ProtoRef{Service: "shop.v1.Carts", Method: "CreateCart"},
	}))
	assert.False(t, result.HasIssues(), messages(t, result))
}

func TestValidate_NoDescriptorAnywhereNamesAllThreePlaces(t *testing.T) {
	result := NewValidator().Validate(&graph.Graph{Nodes: map[string]*graph.Node{
		"createCart": {Proto: &graph.ProtoRef{Service: "shop.v1.Carts", Method: "CreateCart"}},
	}})
	require.True(t, result.HasErrors())
	msg := messages(t, result)
	assert.Contains(t, msg, "on the node")
	assert.Contains(t, msg, "on the graph")
	assert.Contains(t, msg, "aat-project.yaml")
}

func TestCountNodes(t *testing.T) {
	g := &graph.Graph{Nodes: map[string]*graph.Node{
		"a": {Proto: &graph.ProtoRef{Service: "shop.v1.Carts", Method: "CreateCart"}},
		"b": {OAS: &graph.OASRef{OperationID: "listProducts"}},
		"c": {},
	}}
	assert.Equal(t, 1, CountNodes(g))
	assert.Equal(t, 0, CountNodes(&graph.Graph{}))
}

func TestCollectSpecPaths(t *testing.T) {
	g := &graph.Graph{
		Proto: "shop.protoset",
		Nodes: map[string]*graph.Node{
			"a": {Proto: &graph.ProtoRef{Service: "shop.v1.Carts", Method: "CreateCart"}},
			"b": {Proto: &graph.ProtoRef{Service: "pay.v1.Payments", Method: "Charge", Descriptor: "pay.protoset"}},
			"c": {Proto: &graph.ProtoRef{Service: "shop.v1.Carts", Method: "GetCart", Descriptor: "shop.protoset"}},
			"d": {OAS: &graph.OASRef{OperationID: "listProducts"}},
		},
	}
	assert.ElementsMatch(t, []string{"shop.protoset", "pay.protoset"}, NewValidator().CollectSpecPaths(g))
}

func TestResolveNodeDescriptor(t *testing.T) {
	own := &graph.Node{Proto: &graph.ProtoRef{Descriptor: "pay.protoset"}}
	assert.Equal(t, "pay.protoset", ResolveNodeDescriptor(own, "shop.protoset"))

	inherited := &graph.Node{Proto: &graph.ProtoRef{}}
	assert.Equal(t, "shop.protoset", ResolveNodeDescriptor(inherited, "shop.protoset"))
}

func TestValidator_SatisfiesSpecValidator(t *testing.T) {
	var _ graph.SpecValidator = NewValidator()
}
