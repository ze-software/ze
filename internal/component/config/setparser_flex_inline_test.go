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

// TestSetFormRoundTripsFlexValues proves both persisted forms retain every Flex
// value, including repeated and multi-word values, in their written order.
//
// PREVENTS: loading each serialized set line as a scalar replacement, keeping
// only the last value instead of the sequence the hierarchical parser read.
func TestSetFormRoundTripsFlexValues(t *testing.T) {
	schema := NewSchema()
	schema.Define("action", Flex())
	schema.Define("scope", Container(Field("vpls", Flex(Field("rd", Leaf(TypeString))))))
	input := "action zulu;\naction alpha;\naction zulu;\n" +
		"scope { vpls zulu one; vpls alpha two; vpls zulu one; }\n"
	original, err := NewParser(schema).Parse(input)
	if err != nil {
		t.Fatalf("parse hierarchical input: %v", err)
	}
	wantSet := "set action zulu\nset action alpha\nset action zulu\n" +
		"set scope vpls \"zulu one\"\nset scope vpls \"alpha two\"\nset scope vpls \"zulu one\"\n"
	if got := SerializeSet(original, schema); got != wantSet {
		t.Fatalf("original set text:\n%s\nwant:\n%s", got, wantSet)
	}
	for _, parser := range setFormParsers {
		t.Run(parser.name, func(t *testing.T) {
			reread := parser.parse(t, schema, wantSet)
			if got := reread.GetMultiValues("action"); !slices.Equal(got, []string{"zulu", "alpha", "zulu"}) {
				t.Errorf("action values = %q, want [zulu alpha zulu]", got)
			}
			scope := reread.GetContainer("scope")
			if scope == nil {
				t.Fatal("scope container missing")
			}
			if got := scope.GetMultiValues("vpls"); !slices.Equal(got, []string{"zulu one", "alpha two", "zulu one"}) {
				t.Errorf("scope vpls values = %q, want [zulu one alpha two zulu one]", got)
			}
			if got := SerializeSet(reread, schema); got != wantSet {
				t.Errorf("set round trip:\n%s\nwant:\n%s", got, wantSet)
			}
			if got, want := Serialize(reread, schema), Serialize(original, schema); got != want {
				t.Errorf("hierarchical round trip:\n%s\nwant:\n%s", got, want)
			}
		})
	}
	t.Run("persisted metadata", func(t *testing.T) {
		meta := NewMetaTree()
		meta.SetEntry("action", MetaEntry{User: "thomas"})
		meta.GetOrCreateContainer("scope").SetEntry("vpls", MetaEntry{User: "alice"})
		persisted := SerializeSetWithMeta(original, meta, schema)
		reread, rereadMeta, err := NewSetParser(schema).ParseWithMeta(persisted)
		if err != nil {
			t.Fatalf("parse persisted metadata: %v", err)
		}
		if got := SerializeSet(reread, schema); got != wantSet {
			t.Errorf("persisted metadata values:\n%s\nwant:\n%s", got, wantSet)
		}
		if got := SerializeSetWithMeta(reread, rereadMeta, schema); got != persisted {
			t.Errorf("persisted metadata round trip:\n%s\nwant:\n%s", got, persisted)
		}
	})
}

// TestSetParserSingleFlexValueStaysReplaceable proves reading a single Flex
// value leaves the editor's scalar Tree.Set replacement semantics intact.
//
// PREVENTS: a parsed scalar becoming a stale multi-value alongside the editor's
// replacement, so a subsequent save writes both the old and the new value.
func TestSetParserSingleFlexValueStaysReplaceable(t *testing.T) {
	schema := listOrderSchema()
	for _, parser := range setFormParsers {
		t.Run(parser.name, func(t *testing.T) {
			tree := parser.parse(t, schema, "set vpls before\n")
			tree.Set("vpls", "after")
			if got := SerializeSet(tree, schema); got != "set vpls after\n" {
				t.Errorf("set after replacement = %q, want %q", got, "set vpls after\n")
			}
		})
	}
}

// TestFlexValuesSurviveHierarchicalPrint proves a container holding repeated
// Flex values cannot be inlined into a form that drops its only child.
//
// PREVENTS: a successful set-file replay followed by hierarchical printing
// silently discarding the Flex values when choosing a one-line container form.
func TestFlexValuesSurviveHierarchicalPrint(t *testing.T) {
	schema := NewSchema()
	schema.Define("scope", Container(Field("action", Flex())))
	original, err := NewParser(schema).Parse("scope { action zulu; action alpha; action zulu; }")
	if err != nil {
		t.Fatalf("parse hierarchical input: %v", err)
	}
	printed := Serialize(original, schema)
	reread, err := NewParser(schema).Parse(printed)
	if err != nil {
		t.Fatalf("parse hierarchical print %q: %v", printed, err)
	}
	scope := reread.GetContainer("scope")
	if scope == nil {
		t.Fatalf("scope missing after hierarchical print %q", printed)
	}
	if got := scope.GetMultiValues("action"); !slices.Equal(got, []string{"zulu", "alpha", "zulu"}) {
		t.Errorf("action values after hierarchical print %q = %q, want [zulu alpha zulu]", printed, got)
	}
}
