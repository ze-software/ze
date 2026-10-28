// Design: docs/architecture/config/syntax.md -- persisted Flex values and ownership.
// Related: setparser_meta.go, serialize_set.go -- contested Flex assignments.

package config

import (
	"slices"
	"strings"
	"testing"
)

// TestFlexContestedAssignmentsRoundTrip keeps competing whole-leaf assignments
// separate from repeated occurrences through three persisted metadata cycles.
func TestFlexContestedAssignmentsRoundTrip(t *testing.T) {
	schema := NewSchema()
	schema.Define("action", Flex())
	const input = "#alice @ssh set action first\n#bob @ssh set action second\n"
	content := input
	for cycle := range 3 {
		tree, meta, err := NewSetParser(schema).ParseWithMeta(content)
		if err != nil {
			t.Fatalf("cycle %d parse: %v", cycle, err)
		}
		if got := tree.GetMultiValues("action"); len(got) != 0 {
			t.Errorf("cycle %d competing assignments became occurrences: %q", cycle, got)
		}
		if got, ok := tree.Get("action"); !ok || got != "second" {
			t.Errorf("cycle %d effective assignment = %q, %v; want second", cycle, got, ok)
		}
		entries := meta.entries["action"]
		if len(entries) != 2 {
			t.Fatalf("cycle %d owner entries = %v, want two", cycle, entries)
		}
		if entries[0].User != "alice" || entries[0].Value != "first" || entries[1].User != "bob" || entries[1].Value != "second" {
			t.Errorf("cycle %d owner/value pairs changed: %v", cycle, entries)
		}
		content = SerializeSetWithMeta(tree, meta, schema)
		if content != input {
			t.Errorf("cycle %d persisted assignments = %q, want %q", cycle, content, input)
		}
	}
}

// TestFlexRepeatedValueBoundaries checks every printer's occurrence spelling and
// reparses all persisted forms, preserving decoded boundaries and duplicates.
func TestFlexRepeatedValueBoundaries(t *testing.T) {
	schema := NewSchema()
	schema.Define("action", Flex())
	input := "set action first\nset action \"\"\nset action \"#hash\"\n" +
		"set action \"quoted \\\"value\\\"\"\nset action \"line\\nbreak\"\n" +
		"set action \"multi word\"\nset action first\n"
	want := []string{"first", "", "#hash", "quoted \"value\"", "line\nbreak", "multi word", "first"}
	encoded := []string{"first", `""`, `"#hash"`, `"quoted \"value\""`, `"line\nbreak"`, `"multi word"`, "first"}
	for _, parser := range setFormParsers {
		t.Run(parser.name, func(t *testing.T) {
			tree := parser.parse(t, schema, input)
			if got := tree.GetMultiValues("action"); !slices.Equal(got, want) {
				t.Fatalf("loaded values = %q, want %q", got, want)
			}
			meta := NewMetaTree()
			meta.SetEntry("action", MetaEntry{User: "alice"})
			for _, printer := range listOrderPrinters {
				t.Run(printer.name, func(t *testing.T) {
					printed := printer.print(tree, meta, schema)
					var got []string
					for line := range strings.SplitSeq(printed, "\n") {
						if _, value, ok := strings.Cut(line, "action "); ok {
							got = append(got, value)
						}
					}
					if !slices.Equal(got, encoded) {
						t.Errorf("printed occurrence spellings = %q, want %q; output %q", got, encoded, printed)
					}
				})
			}
			for _, form := range []string{"hierarchical", "set", "set-meta"} {
				t.Run(form+" roundtrip", func(t *testing.T) {
					var reread *Tree
					var err error
					switch form {
					case "hierarchical":
						reread, err = NewParser(schema).Parse(Serialize(tree, schema))
					case "set":
						reread, err = NewSetParser(schema).Parse(SerializeSet(tree, schema))
					case "set-meta":
						reread, _, err = NewSetParser(schema).ParseWithMeta(SerializeSetWithMeta(tree, meta, schema))
					}
					if err != nil {
						t.Fatalf("roundtrip parse: %v", err)
					}
					if got := reread.GetMultiValues("action"); !slices.Equal(got, want) {
						t.Errorf("roundtrip values = %q, want %q", got, want)
					}
				})
			}
		})
	}
}
