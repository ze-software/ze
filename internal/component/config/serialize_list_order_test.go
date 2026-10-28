// Design: docs/architecture/config/syntax.md -- the printer's list entry order
// Related: serialize.go -- listBlockKeys, the one key order every printer asks

package config

import (
	"slices"
	"strings"
	"testing"
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
