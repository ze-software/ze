// Design: docs/architecture/config/syntax.md -- complete set-format line parsing.

package config

import (
	"strings"
	"testing"
)

// TestSetParsersAcceptLongLines checks the canonical parsers, independently of
// upload framing. Comments and unconstrained string values above 64 KiB must
// not truncate the input or hide a later command or its metadata.
func TestSetParsersAcceptLongLines(t *testing.T) {
	long := strings.Repeat("x", 70*1024)
	for _, ending := range []string{"\n", "\r\n"} {
		for _, annotated := range []bool{false, true} {
			name := "plain"
			command := "set label " + long
			if annotated {
				name = "metadata"
				command = "#alice @ssh set label " + long
			}
			if ending == "\r\n" {
				name += "-crlf"
			}
			t.Run(name, func(t *testing.T) {
				schema := NewSchema()
				schema.Define("label", &LeafNode{Type: TypeString})
				// The last line is an inert user-only metadata prefix in the
				// annotated form and a comment in the plain form.
				input := "# " + long + ending + command + ending + "#" + long
				parser := NewSetParser(schema)
				var tree *Tree
				var err error
				if annotated {
					var meta *MetaTree
					tree, meta, err = parser.ParseWithMeta(input)
					if err == nil {
						entry, ok := meta.GetEntry("label")
						if !ok {
							t.Fatal("long lines hid the command metadata")
						}
						if entry.User != "alice" {
							t.Fatalf("metadata user = %q, want alice", entry.User)
						}
						if entry.Source != "ssh" {
							t.Fatalf("metadata source = %q, want ssh", entry.Source)
						}
					}
				} else {
					tree, err = parser.Parse(input)
				}
				if err != nil {
					t.Fatalf("valid long input refused: %v", err)
				}
				value, present := tree.Get("label")
				if !present {
					t.Fatal("long comment hid the following configuration")
				}
				if value != long {
					t.Fatal("long string value changed or was truncated")
				}
			})
		}
	}
}

// TestSetParsersLongLineKeepsRefusalLine proves a long ignored line cannot
// replace a later schema refusal with a reader error or shift its line number.
func TestSetParsersLongLineKeepsRefusalLine(t *testing.T) {
	input := "\r\n# " + strings.Repeat("x", 70*1024) + "\r\nset label retained\r\nset absent value\r\n"
	for _, format := range []ConfigFormat{FormatSet, FormatSetMeta} {
		name := "plain"
		if format == FormatSetMeta {
			name = "metadata"
		}
		t.Run(name, func(t *testing.T) {
			schema := NewSchema()
			schema.Define("label", &LeafNode{Type: TypeString})
			tree, refusals := ParseSetDiagnostics(schema, input, format)
			if tree != nil {
				t.Fatal("invalid input exposed a partial configuration")
			}
			if len(refusals) != 1 {
				t.Fatalf("got %d refusals, want the one absent-field refusal", len(refusals))
			}
			line, message := SplitLine(refusals[0])
			if line != 4 {
				t.Fatalf("refusal line = %d, want 4: %s", line, message)
			}
			if !strings.Contains(message, "absent") {
				t.Fatalf("refusal lost the invalid field: %s", message)
			}
		})
	}
}
