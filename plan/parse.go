package plan

import (
	"fmt"
	"os"

	"github.com/gburgyan/aat/internal/yamlx"
	"gopkg.in/yaml.v3"
)

// Parse unmarshals YAML bytes into a Plan. Keys that no plan field accepts
// are errors.
func Parse(data []byte) (*Plan, error) {
	var p Plan
	if err := yamlx.Decode(data, &p); err != nil {
		return nil, err
	}
	if len(p.Execution.Steps) == 0 {
		return nil, fmt.Errorf("plan must have at least one execution step")
	}
	return &p, nil
}

// ParseFile reads a YAML file and parses it into a Plan. Errors name the file.
func ParseFile(path string) (*Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading plan file: %w", err)
	}
	p, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return p, nil
}

// UnmarshalYAML implements custom YAML unmarshalling for Assertions.
// Handles two formats:
//   - Mapping with a mechanical key (the documented form)
//   - Flat list of assertion objects (treated as mechanical)
//
// It uses the callback form so strict decoding reaches the assertions (see
// internal/yamlx).
func (a *Assertions) UnmarshalYAML(unmarshal func(any) error) error {
	n, err := yamlx.Node(unmarshal)
	if err != nil {
		return err
	}
	switch n.Kind {
	case yaml.SequenceNode:
		return unmarshal(&a.Mechanical)
	case yaml.MappingNode:
		type rawAssertions Assertions
		var raw rawAssertions
		if err := unmarshal(&raw); err != nil {
			return err
		}
		*a = Assertions(raw)
		return nil
	default:
		return yamlx.KindError(n, "assertions", "a mapping or a list")
	}
}

// MarshalYAML implements custom YAML marshalling for StepValue.
// When only Default is set, marshal as a bare scalar instead of a mapping.
func (sv StepValue) MarshalYAML() (interface{}, error) {
	if !sv.Locked && sv.From == "" && sv.Select == nil && sv.Constraint == "" &&
		len(sv.Pool) == 0 && sv.PoolStrategy == nil &&
		sv.FromSelection == "" && sv.FromResolved == "" && sv.FromInput == "" && sv.Default != nil {
		return sv.Default, nil
	}
	type rawStepValue StepValue
	return rawStepValue(sv), nil
}

// UnmarshalYAML implements custom YAML unmarshalling for StepValue.
// Bare scalars (e.g., origin: "DEN") set Default only.
// Mappings unmarshal into the full StepValue struct. It uses the callback form
// so strict decoding reaches the mapping (see internal/yamlx).
func (sv *StepValue) UnmarshalYAML(unmarshal func(any) error) error {
	n, err := yamlx.Node(unmarshal)
	if err != nil {
		return err
	}
	switch n.Kind {
	case yaml.ScalarNode:
		return unmarshal(&sv.Default)
	case yaml.MappingNode:
		// The raw type has no methods, which avoids infinite recursion.
		type rawStepValue StepValue
		var raw rawStepValue
		if err := unmarshal(&raw); err != nil {
			return err
		}
		*sv = StepValue(raw)
		return nil
	default:
		return yamlx.KindError(n, "a step value", "a scalar or a mapping")
	}
}
