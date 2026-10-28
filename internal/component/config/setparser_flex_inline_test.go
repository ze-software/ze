package config

import (
	"slices"
	"strings"
	"testing"
)

// setFormParsers are the two readers of the set form: the plain set parser
// (set-format files) and the set-meta parser (the committed file and the
// editor's session journal).
var setFormParsers = []struct {
	name  string
	parse func(t *testing.T, schema *Schema, input string) *Tree
}{
	{"SetParser.Parse", func(t *testing.T, schema *Schema, input string) *Tree {
		t.Helper()
		tree, err := NewSetParser(schema).Parse(input)
		if err != nil {
			t.Fatalf("parse set input: %v", err)
		}
		return tree
	}},
	{"SetParser.ParseWithMeta", func(t *testing.T, schema *Schema, input string) *Tree {
		t.Helper()
		var lines strings.Builder
		for line := range strings.SplitSeq(strings.TrimSuffix(input, "\n"), "\n") {
			lines.WriteString("#thomas @ssh %2026-10-28T12:00:00Z " + line + "\n")
		}
		tree, _, err := NewSetParser(schema).ParseWithMeta(lines.String())
		if err != nil {
			t.Fatalf("parse set-meta input: %v", err)
		}
		return tree
	}},
}

// entryKeys returns the keys of list name in recorded order.
func entryKeys(tree *Tree, name string) []string {
	var keys []string
	for _, entry := range tree.GetListOrdered(name) {
		keys = append(keys, entry.Key)
	}
	return keys
}

// TestSetParserReadsInlineListEntryField proves both set parsers store a
// field of an InlineList entry.
//
// VALIDATES: `set route <key> next-hop <value>` creates entry <key> holding
// next-hop <value>, the form serializeSetInlineList writes.
//
// PREVENTS: the set parser refusing the line ("cannot traverse
// *config.InlineListNode"), so a set-format file holding a route never loads.
func TestSetParserReadsInlineListEntryField(t *testing.T) {
	schema := listOrderSchema()
	for _, parser := range setFormParsers {
		t.Run(parser.name, func(t *testing.T) {
			tree := parser.parse(t, schema, "set route rzulu next-hop z\nset route ralpha next-hop a\n")
			if got := entryKeys(tree, "route"); !slices.Equal(got, []string{"rzulu", "ralpha"}) {
				t.Fatalf("route entries = %v, want [rzulu ralpha]", got)
			}
			entry := tree.GetList("route")["ralpha"]
			if v, _ := entry.Get("next-hop"); v != "a" {
				t.Errorf("route ralpha next-hop = %q, want %q", v, "a")
			}
		})
	}
}

// TestSetParserReadsFlexListEntries proves both set parsers read the named
// block form of a Flex node as list entries.
//
// VALIDATES: three `set vpls <key> rd <value>` lines, the form serializeSetFlex
// writes for a Flex list entry, create three entries in written order, each
// holding its rd, and no flex value.
//
// PREVENTS: each line overwriting the flex value with "<key> rd <value>", so
// that only the last entry survives a load and none is a list entry.
func TestSetParserReadsFlexListEntries(t *testing.T) {
	schema := listOrderSchema()
	input := "set vpls vzulu rd z\nset vpls valpha rd a\nset vpls vmike rd m\n"
	for _, parser := range setFormParsers {
		t.Run(parser.name, func(t *testing.T) {
			tree := parser.parse(t, schema, input)
			if got := entryKeys(tree, "vpls"); !slices.Equal(got, []string{"vzulu", "valpha", "vmike"}) {
				t.Fatalf("vpls entries = %v, want [vzulu valpha vmike]", got)
			}
			if v, ok := tree.Get("vpls"); ok {
				t.Errorf("vpls holds the value %q, want list entries only", v)
			}
			entry := tree.GetList("vpls")["valpha"]
			if v, _ := entry.Get("rd"); v != "a" {
				t.Errorf("vpls valpha rd = %q, want %q", v, "a")
			}
		})
	}
}

// TestSetParserDeletesFlexListEntry proves the set parser's delete reaches a
// Flex named block entry, the way an editor session replays it.
//
// VALIDATES: `delete vpls valpha rd` removes that entry's rd, `delete vpls
// vmike` removes that entry, and `delete vpls` removes the remaining entries.
//
// PREVENTS: a replayed delete of a Flex entry refused as an unknown field, so
// the session never loads, or a whole-node delete leaving its entries behind.
func TestSetParserDeletesFlexListEntry(t *testing.T) {
	schema := listOrderSchema()
	input := "set vpls vzulu rd z\nset vpls valpha rd a\nset vpls vmike rd m\n" +
		"delete vpls valpha rd\ndelete vpls vmike\n"
	tree, err := NewSetParser(schema).Parse(input)
	if err != nil {
		t.Fatalf("parse set input: %v", err)
	}
	if got := entryKeys(tree, "vpls"); !slices.Equal(got, []string{"vzulu", "valpha"}) {
		t.Fatalf("vpls entries = %v, want [vzulu valpha]", got)
	}
	if v, ok := tree.GetList("vpls")["valpha"].Get("rd"); ok {
		t.Errorf("vpls valpha rd = %q after its delete", v)
	}
	tree, err = NewSetParser(schema).Parse(input + "delete vpls\n")
	if err != nil {
		t.Fatalf("parse set input with delete vpls: %v", err)
	}
	if got := entryKeys(tree, "vpls"); len(got) != 0 {
		t.Errorf("vpls entries = %v after delete vpls, want none", got)
	}
}

// TestSetParserReadsFlexContainerField proves both set parsers read the block
// form of a Flex node, `set vpls rd <value>`, as the container serializeSetFlex
// writes it from.
//
// VALIDATES: the line creates container vpls holding rd.
//
// PREVENTS: the set parser refusing the line ("cannot traverse
// *config.FlexNode").
func TestSetParserReadsFlexContainerField(t *testing.T) {
	schema := listOrderSchema()
	for _, parser := range setFormParsers {
		t.Run(parser.name, func(t *testing.T) {
			tree := parser.parse(t, schema, "set vpls rd c\n")
			child := tree.GetContainer("vpls")
			if child == nil {
				t.Fatal("vpls container missing")
			}
			if v, _ := child.Get("rd"); v != "c" {
				t.Errorf("vpls rd = %q, want %q", v, "c")
			}
		})
	}
}

// TestSetFormRoundTripsFlexAndInlineList proves the set form the set printer
// writes for Flex and InlineList entries reads back as the same tree.
//
// VALIDATES: hierarchical text, parsed, printed by SerializeSet and parsed by
// each set parser, prints the same hierarchical and set text as the original
// tree. Entry order is not judged here: the set printer's key order is what
// TestEveryListPrintsRecordedOrder judges.
//
// PREVENTS: a set-format save or an editor session silently losing or
// refusing Flex and InlineList entries.
func TestSetFormRoundTripsFlexAndInlineList(t *testing.T) {
	schema := listOrderSchema()
	original, err := NewParser(schema).Parse(listOrderHierarchical)
	if err != nil {
		t.Fatalf("parse hierarchical input: %v", err)
	}
	setText := SerializeSet(original, schema)
	for _, parser := range setFormParsers {
		t.Run(parser.name, func(t *testing.T) {
			reread := parser.parse(t, schema, setText)
			if got, want := Serialize(reread, schema), Serialize(original, schema); got != want {
				t.Errorf("hierarchical print after the set round trip:\n%s\nwant:\n%s", got, want)
			}
			if got := SerializeSet(reread, schema); got != setText {
				t.Errorf("set print after the set round trip:\n%s\nwant:\n%s", got, setText)
			}
		})
	}
}
