// Design: docs/architecture/config/syntax.md -- empty list entries are configuration.
// Related: serialize_set.go -- plain and metadata-bearing set writers.

package config

import "testing"

// TestSetFormEmptySingleKeyEntryRoundTrip preserves the showcase's empty dummy
// declaration through all three writers and both parsers. Nonempty and inactive
// controls reject redundant address lines and accidental activation.
func TestSetFormEmptySingleKeyEntryRoundTrip(t *testing.T) {
	schema := NewSchema()
	schema.Define("interface", Container(
		Field("dummy", List(TypeString,
			Field("description", Leaf(TypeString)),
			Field("unit", List(TypeString,
				Field("description", Leaf(TypeString)),
			)),
		)),
	))
	for _, tc := range []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "showcase dummy",
			input: "interface { dummy zetrk1 {} }",
			want:  "set interface dummy zetrk1\n",
		},
		{
			name:  "nonempty dummy",
			input: "interface { dummy zetrk1 { description uplink; } }",
			want:  "set interface dummy zetrk1 description uplink\n",
		},
		{
			name:  "nested empty entry",
			input: "interface { dummy zetrk1 { unit 0 {} } }",
			want:  "set interface dummy zetrk1 unit 0\n",
		},
		{
			name:  "inactive empty dummy",
			input: "interface { inactive: dummy zetrk1 {} }",
			want:  "nop interface dummy zetrk1\n",
		},
		{
			name:  "inactive nonempty dummy",
			input: "interface { inactive: dummy zetrk1 { description uplink; } }",
			want:  "nop interface dummy zetrk1\nset interface dummy zetrk1 description uplink\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree, err := NewParser(schema).Parse(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			for _, writer := range []struct {
				name string
				out  string
			}{
				{"plain", SerializeSet(tree, schema)},
				{"metadata", SerializeSetWithMeta(tree, NewMetaTree(), schema)},
				{"annotated", SerializeAnnotatedSubtreeSet(tree, NewMetaTree(), schema.root, ShowColumns{})},
			} {
				t.Run(writer.name, func(t *testing.T) {
					if writer.out != tc.want {
						t.Errorf("serialized %q, want %q", writer.out, tc.want)
					}
					plain, err := NewSetParser(schema).Parse(writer.out)
					if err != nil {
						t.Fatal(err)
					}
					withMeta, meta, err := NewSetParser(schema).ParseWithMeta(writer.out)
					if err != nil {
						t.Fatal(err)
					}
					for _, replay := range []*Tree{plain, withMeta} {
						if got := Serialize(replay, schema); got != Serialize(tree, schema) {
							t.Errorf("replayed configuration %q, want %q", got, Serialize(tree, schema))
						}
					}
					if got := SerializeSetWithMeta(withMeta, meta, schema); got != tc.want {
						t.Errorf("metadata replay serialized %q, want %q", got, tc.want)
					}
				})
			}
		})
	}
}
