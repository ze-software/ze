// Design: docs/architecture/config/change-file-structural-ops.md -- restore unvalidated candidates.

package config

import (
	"slices"
	"testing"
)

func candidateChangeSchema() *Schema {
	schema := NewSchema()
	schema.Define("color", &LeafNode{Type: TypeString, Enums: []string{"red", "blue"}})
	schema.Define("words", &MultiLeafNode{Type: TypeString, Enums: []string{"red", "blue"}})
	schema.Define("bracket", &BracketLeafListNode{Type: TypeString, Enums: []string{"red", "blue"}})
	schema.Define("plain", &ValueOrArrayNode{Type: TypeString, ValidValues: []string{"red", "blue"}})
	schema.Define("count", &LeafNode{Type: TypeUint16, Ranges: []NumericRange{{Min: "1", Max: "10"}}})
	schema.Define("label", &LeafNode{Type: TypeString, Lengths: []NumericRange{{Min: "1", Max: "3"}}})
	schema.Define("code", &LeafNode{Type: TypeString, Patterns: []string{"[a-z]+"}})
	schema.Define("secret", &LeafNode{Type: TypeString, Sensitive: true})
	entry := List(TypeString, Field("color", &LeafNode{Type: TypeString, Enums: []string{"red", "blue"}}))
	entry.KeyName = "name"
	entry.KeyLeaf = &LeafNode{Type: TypeString, Enums: []string{"red", "blue"}}
	schema.Define("entry", entry)
	return schema
}

// TestCandidateChangeFilePreservesInvalidValues checks the restored payload
// and ownership through another serialization, while every ordinary parser
// retains its refusal and accepts a valid control.
func TestCandidateChangeFilePreservesInvalidValues(t *testing.T) {
	schema := candidateChangeSchema()
	for _, test := range []struct {
		name  string
		value string
		list  bool
	}{
		{"color", "bogus", false},
		{"words", "bogus", false},
		{"bracket", "red bogus", true},
		{"plain", "red bogus", true},
		{"count", "0", false},
		{"count", "not-a-number", false},
		{"label", "toolong", false},
		{"code", "123", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := test.value
			if test.list {
				value = "[ " + value + " ]"
			}
			line := "set " + test.name + " " + value
			content := "#alice @api %2026-11-12T20:00:00Z " + line + "\n"
			if _, err := NewSetParser(schema).Parse(line); err == nil {
				t.Fatal("ordinary set parser accepted the invalid candidate")
			}
			if _, _, err := NewSetParser(schema).ParseWithMeta(content); err == nil {
				t.Fatal("ordinary metadata parser accepted the invalid candidate")
			}
			if _, _, _, err := ParseChangeFile(content, NewSetParser(schema)); err == nil {
				t.Fatal("ordinary change-file parser accepted the invalid candidate")
			}
			for cycle := range 2 {
				tree, meta, ops, err := ParseCandidateChangeFile(content, schema)
				if err != nil {
					t.Fatalf("cycle %d: %v", cycle, err)
				}
				if len(ops) != 0 {
					t.Fatalf("leaf value became structural operations: %+v", ops)
				}
				if test.list {
					if members := tree.GetMultiValues(test.name); !slices.Equal(members, []string{"red", "bogus"}) {
						t.Fatalf("cycle %d lost candidate members: %q", cycle, members)
					}
				} else {
					value, ok := tree.Get(test.name)
					if !ok {
						t.Fatalf("cycle %d lost the candidate scalar", cycle)
					}
					if value != test.value {
						t.Fatalf("cycle %d changed candidate scalar: %q", cycle, value)
					}
				}
				entries := meta.SessionEntries(schema, "alice@api%2026-11-12T20:00:00Z")
				wantEntries := 1
				if test.name == "plain" {
					wantEntries = 2
				}
				if len(entries) != wantEntries {
					t.Fatalf("cycle %d lost candidate ownership: %+v", cycle, entries)
				}
				content = SerializeChangeFile(tree, meta, ops, schema)
			}
		})
	}
	if _, _, _, err := ParseChangeFile("#alice @api set color blue\n", NewSetParser(schema)); err != nil {
		t.Fatalf("strict parser refused a valid control: %v", err)
	}
}

// TestCandidateChangeFileRefusesCorruption pins the distinction between an
// invalid leaf value and syntax or addressing that cannot describe an edit.
func TestCandidateChangeFileRefusesCorruption(t *testing.T) {
	for _, input := range []string{
		"set unknown blue",
		"set entry red unknown blue",
		"set entry bogus color blue",
		"set color",
		"set color red blue",
		"set color \"unterminated",
		"set secret \"$9$!!!!!!\"",
		"invent color red",
		"rename entry red blue",
		"insert-member plain red before",
	} {
		t.Run(input, func(t *testing.T) {
			tree, meta, ops, err := ParseCandidateChangeFile(input+"\n", candidateChangeSchema())
			if err == nil {
				t.Fatalf("corrupt candidate was accepted: tree=%v meta=%v ops=%v", tree, meta, ops)
			}
			if tree != nil {
				t.Fatal("a corrupt candidate returned a partial tree")
			}
			if meta != nil {
				t.Fatal("a corrupt candidate returned partial metadata")
			}
			if ops != nil {
				t.Fatal("a corrupt candidate returned partial operations")
			}
		})
	}
}

// TestCandidateChangeFileRejectsBrokenBrackets preserves delimiter refusals
// when a candidate reader defers the members' leaf constraints.
func TestCandidateChangeFileRejectsBrokenBrackets(t *testing.T) {
	for _, leaf := range []string{"bracket", "plain"} {
		for _, value := range []string{"[ red bogus", "red bogus ]", "[ red ] bogus", "[ red [ bogus ] ]"} {
			t.Run(leaf+" "+value, func(t *testing.T) {
				content := "#alice @api set " + leaf + " " + value + "\n"
				schema := candidateChangeSchema()
				if _, _, _, err := ParseChangeFile(content, NewSetParser(schema)); err == nil {
					t.Fatal("strict control accepted broken delimiters")
				}
				if _, _, _, err := ParseCandidateChangeFile(content, schema); err == nil {
					t.Fatal("candidate decoder accepted broken delimiters")
				}
			})
		}
	}
}

// TestCandidateChangeFileKeepsQuotedBracketMembers keeps brackets and spaces
// that belong to invalid member data through serialization and restoration.
func TestCandidateChangeFileKeepsQuotedBracketMembers(t *testing.T) {
	for _, leaf := range []string{"bracket", "plain"} {
		t.Run(leaf, func(t *testing.T) {
			for _, value := range []string{"[ \"[\" \"red ] blue\" \"]\" ]", "\"[\" \"red ] blue\" \"]\""} {
				content := "#alice @api set " + leaf + " " + value + "\n"
				schema := candidateChangeSchema()
				for cycle := range 2 {
					tree, meta, ops, err := ParseCandidateChangeFile(content, schema)
					if err != nil {
						t.Fatalf("cycle %d: %v", cycle, err)
					}
					if members := tree.GetMultiValues(leaf); !slices.Equal(members, []string{"[", "red ] blue", "]"}) {
						t.Fatalf("cycle %d changed quoted member boundaries: %q", cycle, members)
					}
					content = SerializeChangeFile(tree, meta, ops, schema)
				}
			}
		})
	}
}

// TestCandidateChangeFileKeepsLegacyBracketMembers preserves the accepted
// joined spelling while candidate restoration defers invalid member values.
func TestCandidateChangeFileKeepsLegacyBracketMembers(t *testing.T) {
	schema := candidateChangeSchema()
	content := "#alice @api set bracket \"red blue\"\n"
	for cycle := range 2 {
		tree, meta, ops, err := ParseCandidateChangeFile(content, schema)
		if err != nil {
			t.Fatal(err)
		}
		if members := tree.GetMultiValues("bracket"); !slices.Equal(members, []string{"red", "blue"}) {
			t.Fatalf("cycle %d lost accepted legacy members: %q", cycle, members)
		}
		content = SerializeChangeFile(tree, meta, ops, schema)
	}
}
