package config

import (
	"reflect"
	"strings"
	"testing"
)

// TestFormatKeepsOneUndeclaredFieldInItsBlock formats a container whose only
// value is a name the schema does not declare, a subsystem level under
// `environment { log }`, and parses the result.
//
// VALIDATES: the formatted text parses to the same map, and the level stays
// inside the braces of `log`.
// PREVENTS: the inline form `log bgp.reactor.peer info`, which parseContainer
// refuses because it reads a brace-less child only when the container
// declares it. Two undeclared names never took the inline form, so only a
// config that sets one subsystem level was written back unparseable.
func TestFormatKeepsOneUndeclaredFieldInItsBlock(t *testing.T) {
	schema, err := YANGSchema()
	if err != nil {
		t.Fatalf("YANG schema: %v", err)
	}
	const input = "environment {\n\tlog {\n\t\tbgp.reactor.peer info\n\t}\n}\n"
	source, err := NewParser(schema).Parse(input)
	if err != nil {
		t.Fatalf("the input does not parse: %v", err)
	}
	formatted := Serialize(source, schema)
	reparsed, err := NewParser(schema).Parse(formatted)
	if err != nil {
		t.Fatalf("formatted text does not parse: %v\n%s", err, formatted)
	}
	if !reflect.DeepEqual(source.ToMap(), reparsed.ToMap()) {
		t.Fatalf("formatted text lowers to another map\n%s", formatted)
	}
	if !strings.Contains(formatted, "log {") {
		t.Errorf("the subsystem level left its block:\n%s", formatted)
	}

	// A declared leaf alone still takes the inline form: the refusal is for
	// the undeclared name, not for every single-value log block.
	declared, err := NewParser(schema).Parse("environment {\n\tlog {\n\t\tlevel debug\n\t}\n}\n")
	if err != nil {
		t.Fatalf("the declared input does not parse: %v", err)
	}
	if inline := Serialize(declared, schema); !strings.Contains(inline, "log level debug") {
		t.Errorf("a declared leaf lost its inline form:\n%s", inline)
	}
}
