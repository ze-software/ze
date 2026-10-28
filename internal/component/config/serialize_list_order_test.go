// Design: docs/architecture/config/syntax.md -- the printer's list entry order
// Related: serialize.go -- ListEntryKeys, the one key order every printer asks

package config

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// listOrderSchema holds one list of every kind a printer handles differently:
// a plain list printed as one block per entry (peer), a positional list
// printed as one multi-entry block (server), a Flex list (vpls), an
// InlineList (route) and an `ordered-by user` list (term).
//
// Every key carries a one-letter prefix naming its list, so the position of a
// key in the printed text belongs to exactly one list.
func listOrderSchema() *Schema {
	schema := NewSchema()
	schema.Define("peer", List(TypeString,
		Field("description", Leaf(TypeString)),
	))
	server := List(TypeString, Field("port", Leaf(TypeUint16)))
	server.KeyName = "address"
	schema.Define("server", server)
	schema.Define("vpls", Flex(Field("rd", Leaf(TypeString))))
	schema.Define("route", InlineList(TypeString, Field("next-hop", Leaf(TypeString))))
	term := List(TypeString, Field("action", Leaf(TypeString)))
	term.UserOrdered = true
	schema.Define("term", term)
	return schema
}

// listOrderLists names each list of listOrderSchema with its keys in the order
// the inputs below write them: zulu, alpha, mike, which is not key order.
var listOrderLists = []struct {
	name string
	keys []string
}{
	{"peer", []string{"pzulu", "palpha", "pmike"}},
	{"server", []string{"szulu", "salpha", "smike"}},
	{"vpls", []string{"vzulu", "valpha", "vmike"}},
	{"route", []string{"rzulu", "ralpha", "rmike"}},
	{"term", []string{"tzulu", "talpha", "tmike"}},
}

const listOrderHierarchical = `peer pzulu {
	description z
}
peer palpha {
	description a
}
peer pmike {
	description m
}
server {
	szulu 1
	salpha 2
	smike 3
}
vpls vzulu {
	rd z
}
vpls valpha {
	rd a
}
vpls vmike {
	rd m
}
route rzulu next-hop z
route ralpha next-hop a
route rmike next-hop m
term tzulu {
	action z
}
term talpha {
	action a
}
term tmike {
	action m
}
`

const listOrderSet = `set peer pzulu description z
set peer palpha description a
set peer pmike description m
set server szulu port 1
set server salpha port 2
set server smike port 3
set vpls vzulu rd z
set vpls valpha rd a
set vpls vmike rd m
set route rzulu next-hop z
set route ralpha next-hop a
set route rmike next-hop m
set term tzulu action z
set term talpha action a
set term tmike action m
`

// listOrderSetLines returns the lines of listOrderSet that write the lists
// named, each line prefixed with prefix.
func listOrderSetLines(lists []string, prefix string) string {
	var b strings.Builder
	for line := range strings.SplitSeq(strings.TrimSuffix(listOrderSet, "\n"), "\n") {
		for _, list := range lists {
			if strings.HasPrefix(line, "set "+list+" ") {
				b.WriteString(prefix)
				b.WriteString(line)
				b.WriteString("\n")
			}
		}
	}
	return b.String()
}

// listOrderTree is one way of building the tree the printers read, and the
// lists of listOrderLists it builds.
type listOrderTree struct {
	name  string
	lists []string
	build func(t *testing.T, schema *Schema, lists []string) (*Tree, *MetaTree)
}

// listOrderBuilders are the parsers that build a tree from operator text.
var listOrderBuilders = []listOrderTree{
	{"parser", []string{"peer", "server", "vpls", "route", "term"},
		func(t *testing.T, schema *Schema, _ []string) (*Tree, *MetaTree) {
			t.Helper()
			tree, err := NewParser(schema).Parse(listOrderHierarchical)
			if err != nil {
				t.Fatalf("parse hierarchical input: %v", err)
			}
			return tree, NewMetaTree()
		}},
	{"set parser", []string{"peer", "server", "vpls", "route", "term"},
		func(t *testing.T, schema *Schema, lists []string) (*Tree, *MetaTree) {
			t.Helper()
			tree, err := NewSetParser(schema).Parse(listOrderSetLines(lists, ""))
			if err != nil {
				t.Fatalf("parse set input: %v", err)
			}
			return tree, NewMetaTree()
		}},
	{"set-meta parser", []string{"peer", "server", "vpls", "route", "term"},
		func(t *testing.T, schema *Schema, lists []string) (*Tree, *MetaTree) {
			t.Helper()
			// The committed file's form: each line carries the author, the
			// origin and the time of the change.
			input := listOrderSetLines(lists, "#thomas @ssh %2026-10-28T12:00:00Z ")
			tree, meta, err := NewSetParser(schema).ParseWithMeta(input)
			if err != nil {
				t.Fatalf("parse set-meta input: %v", err)
			}
			return tree, meta
		}},
}

// listOrderPrinter is one printer of a whole tree.
type listOrderPrinter struct {
	name  string
	print func(tree *Tree, meta *MetaTree, schema *Schema) string
}

var listOrderPrinters = []listOrderPrinter{
	{"Serialize", func(tree *Tree, _ *MetaTree, schema *Schema) string {
		return Serialize(tree, schema)
	}},
	{"Serialize of MaskSecrets", func(tree *Tree, _ *MetaTree, schema *Schema) string {
		return Serialize(MaskSecrets(tree, schema), schema)
	}},
	{"SerializeSet", func(tree *Tree, _ *MetaTree, schema *Schema) string {
		return SerializeSet(tree, schema)
	}},
	{"SerializeSetWithMeta", SerializeSetWithMeta},
	{"SerializeAnnotatedTree", func(tree *Tree, meta *MetaTree, schema *Schema) string {
		return SerializeAnnotatedTree(tree, meta, schema, ShowColumns{Author: true, Changes: true})
	}},
	{"SerializeAnnotatedSet", func(tree *Tree, meta *MetaTree, schema *Schema) string {
		return SerializeAnnotatedSet(tree, meta, schema, ShowColumns{Author: true, Changes: true})
	}},
	{"SerializeBlame", SerializeBlame},
}

// printedKeyOrder returns where each key first appears in output, refusing a
// key the output does not hold.
func printedKeyOrder(t *testing.T, output string, keys []string) []int {
	t.Helper()
	positions := make([]int, len(keys))
	for i, key := range keys {
		positions[i] = strings.Index(output, key)
		if positions[i] < 0 {
			t.Errorf("output drops entry %q:\n%s", key, output)
		}
	}
	return positions
}

// assertKeysInOrder fails unless output prints keys in the order given.
func assertKeysInOrder(t *testing.T, output, list string, keys []string) {
	t.Helper()
	positions := printedKeyOrder(t, output, keys)
	for i := 1; i < len(keys); i++ {
		if positions[i-1] < 0 {
			continue
		}
		if positions[i] < 0 {
			continue
		}
		if positions[i-1] > positions[i] {
			t.Errorf("list %s prints %q before %q, want the recorded order %v:\n%s",
				list, keys[i], keys[i-1], keys, output)
		}
	}
}

// TestEveryListPrintsRecordedOrder proves AC-2 and AC-3: every printer writes
// the entries of every list kind in the order the tree recorded them, never
// sorted by key. The method is one input per builder (the hierarchical parser,
// the set parser, the set-meta parser) writing each list zulu, alpha, mike,
// printed by every whole-tree printer, one case through MaskSecrets.
func TestEveryListPrintsRecordedOrder(t *testing.T) {
	schema := listOrderSchema()
	for _, builder := range listOrderBuilders {
		for _, printer := range listOrderPrinters {
			t.Run(builder.name+"/"+printer.name, func(t *testing.T) {
				tree, meta := builder.build(t, schema, builder.lists)
				output := printer.print(tree, meta, schema)
				for _, list := range listOrderLists {
					if !slices.Contains(builder.lists, list.name) {
						continue // This builder does not read the list back (listOrderBuilders).
					}
					assertKeysInOrder(t, output, list.name, list.keys)
				}
			})
		}
	}
}

// TestUnrecordedEntryPrintsLast proves AC-5: an entry the tree's recorded
// order does not name (added through the live GetList map, which bypasses
// AddListEntry) is printed after the recorded entries by every printer, and
// no printer drops it. The method parses the hierarchical input, adds one
// unrecorded entry per list, and prints with every printer.
func TestUnrecordedEntryPrintsLast(t *testing.T) {
	schema := listOrderSchema()
	unrecorded := map[string]string{
		"peer":   "pbravo",
		"server": "sbravo",
		"vpls":   "vbravo",
		"route":  "rbravo",
		"term":   "tbravo",
	}
	children := map[string][2]string{
		"peer":   {"description", "b"},
		"server": {"port", "4"},
		"vpls":   {"rd", "b"},
		"route":  {"next-hop", "b"},
		"term":   {"action", "b"},
	}
	for _, printer := range listOrderPrinters {
		t.Run(printer.name, func(t *testing.T) {
			tree, err := NewParser(schema).Parse(listOrderHierarchical)
			if err != nil {
				t.Fatalf("parse hierarchical input: %v", err)
			}
			for _, list := range listOrderLists {
				entry := NewTree()
				child := children[list.name]
				entry.Set(child[0], child[1])
				tree.GetList(list.name)[unrecorded[list.name]] = entry
			}
			output := printer.print(tree, NewMetaTree(), schema)
			for _, list := range listOrderLists {
				keys := append(append([]string{}, list.keys...), unrecorded[list.name])
				assertKeysInOrder(t, output, list.name, keys)
			}
		})
	}
}

// TestSerializeIsDeterministic proves AC-11 with 100 fresh parses per builder
// and printer, so map iteration during either parsing or printing cannot hide
// behind reusing one tree.
func TestSerializeIsDeterministic(t *testing.T) {
	schema := listOrderSchema()
	for _, builder := range listOrderBuilders {
		for _, printer := range listOrderPrinters {
			t.Run(builder.name+"/"+printer.name, func(t *testing.T) {
				var first string
				for i := range 100 {
					tree, meta := builder.build(t, schema, builder.lists)
					printed := printer.print(tree, meta, schema)
					if i == 0 {
						first = printed
						continue
					}
					if printed != first {
						t.Fatalf("parse %d prints different bytes:\n%s\nwant:\n%s", i+1, printed, first)
					}
				}
			})
		}
	}
}

// TestListEntryKeysRepairsIncompleteRecord proves stale and repeated record
// keys cannot duplicate entries, and missing entries follow in sorted order.
func TestListEntryKeysRepairsIncompleteRecord(t *testing.T) {
	entries := map[string]*Tree{
		"zulu": NewTree(), "alpha": NewTree(), "mike": NewTree(), "bravo": NewTree(),
	}
	for _, tc := range []struct {
		name  string
		order []string
		want  []string
	}{
		{"complete", []string{"zulu", "alpha", "mike", "bravo"}, []string{"zulu", "alpha", "mike", "bravo"}},
		{"missing", []string{"zulu", "alpha"}, []string{"zulu", "alpha", "bravo", "mike"}},
		{"stale and duplicate", []string{"removed", "zulu", "zulu", "alpha"}, []string{"zulu", "alpha", "bravo", "mike"}},
		{"unrecorded", nil, []string{"alpha", "bravo", "mike", "zulu"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := slices.Clone(tc.order)
			got := ListEntryKeys(entries, tc.order)
			if !slices.Equal(got, tc.want) {
				t.Errorf("keys = %q, want %q", got, tc.want)
			}
			if !slices.Equal(tc.order, original) {
				t.Errorf("record changed to %q, want %q", tc.order, original)
			}
			got[0] = "caller-owned"
			if !slices.Equal(tc.order, original) {
				t.Errorf("result aliases record: %q", tc.order)
			}
		})
	}
	if got := ListEntryKeys(nil, []string{"removed"}); len(got) != 0 {
		t.Errorf("empty list prints %q", got)
	}
}

// TestRecordedOrderMetadataByteRoundTrip proves replay of the committed
// set-with-metadata form preserves bytes, including author, origin and time,
// for every list kind and for user-ordered terms.
func TestRecordedOrderMetadataByteRoundTrip(t *testing.T) {
	schema := listOrderSchema()
	lists := []string{"peer", "server", "vpls", "route", "term"}
	input := listOrderSetLines(lists, "#thomas @ssh %2026-10-28T12:00:00Z ")
	tree, meta, err := NewSetParser(schema).ParseWithMeta(input)
	if err != nil {
		t.Fatalf("parse committed file: %v", err)
	}
	if got := SerializeSetWithMeta(tree, meta, schema); got != input {
		t.Fatalf("committed bytes changed:\n%s\nwant:\n%s", got, input)
	}
}

// TestRecordedOrderMasking proves the display mask removes actual secrets
// without sorting list entries, in every printer rather than an empty mask.
func TestRecordedOrderMasking(t *testing.T) {
	schema := NewSchema()
	secret := Leaf(TypeString)
	secret.Sensitive = true
	schema.Define("peer", List(TypeString, Field("password", secret)))
	tree, err := NewParser(schema).Parse(
		"peer zulu { password swordfish; } peer alpha { password hiddenvalue; }")
	if err != nil {
		t.Fatalf("parse secret input: %v", err)
	}
	for _, printer := range listOrderPrinters {
		t.Run(printer.name, func(t *testing.T) {
			masked := MaskSecrets(tree, schema)
			output := printer.print(masked, NewMetaTree(), schema)
			assertKeysInOrder(t, output, "peer", []string{"zulu", "alpha"})
			for _, clear := range []string{"swordfish", "hiddenvalue"} {
				if strings.Contains(output, clear) {
					t.Errorf("masked print exposes %q:\n%s", clear, output)
				}
			}
			if strings.Count(output, SecretDataPlaceholder) != 2 {
				t.Errorf("masked print loses secret placeholders:\n%s", output)
			}
		})
	}
}

// TestPrintersRepairStaleListRecords proves every printer uses the shared
// producer even when a live-map mutation leaves missing, removed and repeated
// record keys together. Each live entry must appear exactly once.
func TestPrintersRepairStaleListRecords(t *testing.T) {
	schema := listOrderSchema()
	for _, printer := range listOrderPrinters {
		t.Run(printer.name, func(t *testing.T) {
			tree, err := NewParser(schema).Parse(listOrderHierarchical)
			if err != nil {
				t.Fatalf("parse input: %v", err)
			}
			for _, list := range listOrderLists {
				tree.listOrder[list.name] = []string{"removed", list.keys[0], list.keys[0]}
			}
			output := printer.print(tree, NewMetaTree(), schema)
			for _, list := range listOrderLists {
				assertKeysInOrder(t, output, list.name, list.keys)
				for _, key := range list.keys {
					if count := strings.Count(output, key); count != 1 {
						t.Errorf("entry %q appears %d times, want once:\n%s", key, count, output)
					}
				}
			}
			if strings.Contains(output, "removed") {
				t.Errorf("removed record key printed:\n%s", output)
			}
		})
	}
}

// TestContainerInlineEligibilityUsesSchema proves the inline boundary keeps
// Flex statements in their container block and retains value-array members.
// Every printer retains each occurrence; hierarchical and set file forms
// reparse to the same set output. Array layout is not part of this contract.
func TestContainerInlineEligibilityUsesSchema(t *testing.T) {
	for _, tc := range []struct {
		name      string
		child     Node
		body      string
		mustBlock bool
	}{
		{"repeated Flex", Flex(), "action zulu; action alpha; action zulu;", true},
		{"single Flex", Flex(), "action zulu;", true},
		{"array", valueOrArray(TypeString), "action [ zulu alpha ];", false},
		{"single array member", valueOrArray(TypeString), "action zulu;", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			schema := NewSchema()
			container := Container(Field("action", tc.child))
			schema.Define("scope", container)
			tree, err := NewParser(schema).Parse("scope { " + tc.body + " }")
			if err != nil {
				t.Fatalf("parse input: %v", err)
			}
			if tc.mustBlock && CanInlineContainer(tree.GetContainer("scope"), container) {
				t.Error("Flex container must not inline its value statements")
			}
			for _, printer := range listOrderPrinters {
				t.Run(printer.name, func(t *testing.T) {
					output := printer.print(tree, NewMetaTree(), schema)
					if got, want := strings.Count(output, "zulu"), strings.Count(tc.body, "zulu"); got != want {
						t.Errorf("zulu appears %d times, want %d:\n%s", got, want, output)
					}
					if got, want := strings.Count(output, "alpha"), strings.Count(tc.body, "alpha"); got != want {
						t.Errorf("alpha appears %d times, want %d:\n%s", got, want, output)
					}
					switch printer.name {
					case "Serialize", "Serialize of MaskSecrets", "SerializeAnnotatedTree":
						if tc.mustBlock && strings.Contains(output, "scope action") {
							t.Errorf("Flex container lost its block:\n%s", output)
						}
					}
					var reparsed *Tree
					var err error
					switch printer.name {
					case "Serialize", "Serialize of MaskSecrets":
						reparsed, err = NewParser(schema).Parse(output)
					case "SerializeSet":
						reparsed, err = NewSetParser(schema).Parse(output)
					case "SerializeSetWithMeta":
						reparsed, _, err = NewSetParser(schema).ParseWithMeta(output)
					default:
						return // Annotated and blame gutters are display text, not config files.
					}
					if err != nil {
						t.Fatalf("reparse printed values: %v\n%s", err, output)
					}
					if got, want := SerializeSet(reparsed, schema), SerializeSet(tree, schema); got != want {
						t.Errorf("printed values changed on replay:\n%s\nwant:\n%s", got, want)
					}
				})
			}
		})
	}
}

// TestListEntryWritersMatchCanonicalForms proves each entry form preserves
// independently specified fields, member order, inactive state and comments
// when its emitted text is parsed, including a nested InlineList entry.
func TestListEntryWritersMatchCanonicalForms(t *testing.T) {
	positional := List(TypeString, Field("port", Leaf(TypeUint16)))
	positional.KeyName = "address"
	for _, tc := range []struct {
		name    string
		node    Node
		input   string
		block   bool
		grouped bool
		want    map[string]any
	}{
		{"plain", List(TypeString, Field("description", Leaf(TypeString))),
			"# above\nentry zulu { description value; } # after\n", true, false,
			map[string]any{"description": "value"}},
		{"inactive", List(TypeString, Field("description", Leaf(TypeString))),
			"inactive: entry zulu { description value; }", true, false,
			map[string]any{"description": "value"}},
		{"positional", positional, "entry { zulu 123; }", false, true,
			map[string]any{"port": "123"}},
		{"leaf-list", List(TypeString, Field("members", valueOrArray(TypeString))),
			"entry zulu { members [ b a ]; }", false, false,
			map[string]any{"members": []string{"b", "a"}}},
		{"Flex", Flex(Field("rd", Leaf(TypeString))),
			"entry zulu { rd value; }", true, false,
			map[string]any{"rd": "value"}},
		{"InlineList inline", InlineList(TypeString, Field("next-hop", Leaf(TypeString))),
			"entry zulu next-hop value;", false, false,
			map[string]any{"next-hop": "value"}},
		{"InlineList block", InlineList(TypeString, Field("details", Container(Field("description", Leaf(TypeString))))),
			"entry zulu { details { description value; } }", true, false,
			map[string]any{"details": map[string]any{"description": "value"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			schema := NewSchema()
			schema.Define("entry", tc.node)
			tree, err := NewParser(schema).Parse(tc.input)
			if err != nil {
				t.Fatalf("parse input: %v", err)
			}
			entries := tree.GetList("entry")
			entry := entries["zulu"]
			if entry == nil {
				t.Fatal("parsed entry is missing")
			}
			var printed textbuf.Buffer
			var block bool
			switch node := tc.node.(type) {
			case *ListNode:
				grouped := ListUsesMultiBlock(entries, node)
				if grouped != tc.grouped {
					t.Fatalf("grouped form = %v, want %v", grouped, tc.grouped)
				}
				if grouped {
					printed.Str("entry {\n")
					WritePositionalListEntry(&printed, entry, "zulu", node, "\t")
					printed.Str("}\n")
				} else {
					block = WriteListEntry(&printed, entry, "entry", "zulu", node, "", "")
				}
			case *FlexNode:
				WriteFlexListEntry(&printed, entry, "entry", "zulu", node, "")
				block = true
			case *InlineListNode:
				block = WriteInlineListEntry(&printed, entry, "entry", "zulu", node, "")
			default:
				t.Fatalf("unsupported test node %T", tc.node)
			}
			if block != tc.block {
				t.Errorf("block form = %v, want %v", block, tc.block)
			}
			reparsed, err := NewParser(schema).Parse(printed.String())
			if err != nil {
				t.Fatalf("parse emitted entry: %v\n%s", err, printed.String())
			}
			replayedEntries := reparsed.GetListOrdered("entry")
			if len(replayedEntries) != 1 {
				t.Fatalf("replayed %d entries, want one", len(replayedEntries))
			}
			if replayedEntries[0].Key != "zulu" {
				t.Errorf("replayed key = %q, want zulu", replayedEntries[0].Key)
			}
			replayed := replayedEntries[0].Value
			if got := replayed.ToMap(); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("replayed fields = %#v, want %#v", got, tc.want)
			}
			if got, want := replayed.IsInactive(), tc.name == "inactive"; got != want {
				t.Errorf("replayed inactive = %v, want %v", got, want)
			}
			if tc.name == "plain" {
				comments := replayed.commentsOfBlock().Statement
				if !slices.Equal(comments.Leading, []string{" above"}) {
					t.Errorf("leading comment = %q, want [ above]", comments.Leading)
				}
				if !slices.Equal(comments.Trailing, []string{" after"}) {
					t.Errorf("trailing comment = %q, want [ after]", comments.Trailing)
				}
			}
		})
	}
}

// TestListUsesMultiBlockChecksEveryEntry proves a positional sibling cannot
// make the diff group a list whose other entry omits an earlier field.
func TestListUsesMultiBlockChecksEveryEntry(t *testing.T) {
	node := List(TypeString, Field("priority", Leaf(TypeUint16)), Field("port", Leaf(TypeUint16)))
	node.KeyName = "address"
	first := NewTree()
	first.Set("priority", "1")
	first.Set("port", "123")
	entries := map[string]*Tree{"zulu": first}
	if !ListUsesMultiBlock(entries, node) {
		t.Fatal("complete positional entry must use grouped form")
	}
	later := NewTree()
	later.Set("port", "456")
	entries["alpha"] = later
	if ListUsesMultiBlock(entries, node) {
		t.Fatal("list with an omitted earlier field must use named entry blocks")
	}
}
