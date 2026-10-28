// Design: docs/architecture/config/syntax.md — list entries on one line.

package config

import (
	"reflect"
	"testing"
)

// TestFormatOneLineListEntries checks that a list entry written on one line
// survives `ze config fmt`, for the two shapes FuzzFormatRoundTrip found:
//   - a key that is one of the list's child names: the positional block form
//     `internal { use 000 }` reads back as an anonymous entry setting the
//     child `use`, so the printer writes such a list one block per entry;
//   - a leaf-list child before the last (`pppoe { interface 0 0 }` gives `0`
//     to service-name): the parser stored it with a plain Set, where no
//     printer reads a leaf-list, so it now goes through the member store.
//
// Method: parse, Serialize, parse again, compare ToMap and the second print.
func TestFormatOneLineListEntries(t *testing.T) {
	schema, err := YANGSchema()
	if err != nil {
		t.Fatalf("YANG schema: %v", err)
	}
	for _, input := range []string{
		"plugin internal  use 000",
		"plugin { internal use 000; internal other 001; }",
		"pppoe{interface 0 0}",
		"pppoe { interface eth0 svc 10 }",
	} {
		source, err := NewParser(schema).Parse(input)
		if err != nil {
			t.Fatalf("%q refused: %v", input, err)
		}
		formatted := Serialize(source, schema)
		reparsed, err := NewParser(schema).Parse(formatted)
		if err != nil {
			t.Fatalf("formatted text of %q does not parse: %v\n%s", input, err, formatted)
		}
		if !reflect.DeepEqual(source.ToMap(), reparsed.ToMap()) {
			t.Errorf("formatted text of %q lowers to another map\nsource    %v\nformatted %v\n%s",
				input, source.ToMap(), reparsed.ToMap(), formatted)
		}
		if again := Serialize(reparsed, schema); again != formatted {
			t.Errorf("second format of %q changes the output:\n%s\nvs\n%s", input, formatted, again)
		}
	}
}
