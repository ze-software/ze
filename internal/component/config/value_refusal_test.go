// Design: docs/architecture/config/yang-config-design.md -- Value refusals
// Related: leaf_value.go -- the refusal each restriction builds
// Related: parser_diagnostics.go -- valueRefusal, namedRefusal: the path named at the parse root
// Related: setparser_refusal.go -- the set line's frame
// Related: list_key.go -- EnsureListEntry judges a new key

package config

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/suggest"
)

// capabilityHelp is the ze:help of each route-refresh value in the test schema.
var capabilityHelp = map[string]string{
	"enable":  "Advertise the capability",
	"disable": "Do not advertise it",
	"require": "Refuse a peer that does not advertise it",
	"refuse":  "Refuse a peer that advertises it",
}

// refusalSchema is a small schema shaped like the real one: bgp peer <name>
// session capability route-refresh (an enum leaf with help), an enum-keyed
// list with a child (ospf redistribute <source> metric), and a container of
// every leaf-list kind with an enum type, under a keyed list.
func refusalSchema() *Schema {
	enums := []string{"enable", "disable", "require", "refuse"}
	routeRefresh := &LeafNode{Type: TypeString, Enums: enums, EnumHelp: capabilityHelp}
	peer := List(TypeString, Field("session", Container(Field("capability", Container(Field("route-refresh", routeRefresh))))))
	peer.KeyName = "name"
	peer.KeyLeaf = &LeafNode{Type: TypeString}

	source := &LeafNode{Type: TypeString, Enums: []string{"connected", "static"}}
	redistribute := List(TypeString, Field("metric", Leaf(TypeUint32)))
	redistribute.KeyName = "source"
	redistribute.KeyLeaf = source

	colors := []string{"red", "blue"}
	shade := List(TypeString, Field("label", Leaf(TypeString)))
	shade.KeyName = "id"
	shade.KeyLeaf = &LeafNode{Type: TypeString, Enums: colors}
	pair := List(TypeString, Field("color", &LeafNode{Type: TypeString, Enums: colors}))
	pair.KeyName = "id"
	pair.KeyLeaf = &LeafNode{Type: TypeString}
	item := List(TypeString,
		Field("color", &LeafNode{Type: TypeString, Enums: colors}),
		Field("words", &MultiLeafNode{Type: TypeString, Enums: colors}),
		Field("tags", &BracketLeafListNode{Type: TypeString, Enums: colors}),
		Field("modes", &ValueOrArrayNode{Type: TypeString, ValidValues: colors}),
	)
	item.KeyName = "id"
	item.KeyLeaf = &LeafNode{Type: TypeString}

	schema := NewSchema()
	schema.Define("bgp", Container(Field("peer", peer)))
	schema.Define("ospf", Container(Field("redistribute", redistribute)))
	schema.Define("top", Container(Field("item", item), Field("shade", shade), Field("pair", pair)))
	return schema
}

// routeRefreshFile is a file whose fifth line gives route-refresh value.
func routeRefreshFile(value string) string {
	return "bgp {\n peer peer1 {\n  session {\n   capability {\n    route-refresh " + value + ";\n   }\n  }\n }\n}\n"
}

// TestParserEnumRefusalNamesFullPath parses a file holding a misspelled enum
// value and asserts the whole refusal by equality: the line, the path once
// with spaces and the key included, the suggestion, every value with its
// help in declared order, and the file-form fix last. The path is written at
// the parse root, so dropping WithPath leaves "route-refresh:" alone (AC-1,
// AC-2).
func TestParserEnumRefusalNamesFullPath(t *testing.T) {
	_, err := NewParser(refusalSchema()).Parse(routeRefreshFile("requre"))
	require.Error(t, err)
	want := "line 5: bgp peer peer1 session capability route-refresh: \"requre\" is not an accepted value (did you mean require?)\n" +
		"route-refresh accepts only these values:\n" +
		"  enable   Advertise the capability\n" +
		"  disable  Do not advertise it\n" +
		"  require  Refuse a peer that does not advertise it\n" +
		"  refuse   Refuse a peer that advertises it\n" +
		"fix: route-refresh require;"
	assert.Equal(t, want, err.Error())

	_, err = NewParser(refusalSchema()).Parse(routeRefreshFile("banana"))
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "did you mean")
	assert.True(t, strings.HasSuffix(err.Error(), "\nfix: route-refresh <enable|disable|require|refuse>;"), err.Error())
	for _, old := range []string{"write:", "invalid enum", "bgp/peer"} {
		assert.NotContains(t, err.Error(), old)
	}
}

// TestParserRefusalIsTyped proves the parser's refusal reaches a caller typed:
// errors.As finds the LineError, the ValidationError and the operator error,
// and the ValidationError keeps the facts the JSON diagnostic reads (AC-21).
func TestParserRefusalIsTyped(t *testing.T) {
	_, err := NewParser(refusalSchema()).Parse(routeRefreshFile("requre"))
	var line *LineError
	require.True(t, errors.As(err, &line))
	assert.Equal(t, 5, line.Line())
	var refusal *configyang.ValidationError
	require.True(t, errors.As(err, &refusal))
	assert.Equal(t, configyang.ErrTypeEnum, refusal.Type)
	assert.Equal(t, "requre", refusal.Got)
	assert.Equal(t, "enable, disable, require, refuse", refusal.Expected)
	assert.Equal(t, []string{"bgp", "peer", "peer1", "session", "capability", "route-refresh"}, refusal.Segments())
	var operator *opererr.Error
	require.True(t, errors.As(err, &operator))
	assert.Equal(t, "route-refresh require;", operator.Fix())
}

// TestParseAtEnumRefusalNamesContextPath loads a snippet at an editor
// context and asserts the refusal names the context, its key included, before
// the leaf, with the leaf line as the fix (AC-19).
func TestParseAtEnumRefusalNamesContextPath(t *testing.T) {
	_, err := NewParser(refusalSchema()).ParseAt("route-refresh requre;\n", []string{"bgp", "peer", "peer1", "session", "capability"})
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "line 1: bgp peer peer1 session capability route-refresh: \"requre\" is not an accepted value"), err.Error())
	assert.True(t, strings.HasSuffix(err.Error(), "\nfix: route-refresh require;"), err.Error())
}

// TestParserMissingEnumValueRefusal parses an enum leaf with no value and
// asserts the missing-value problem, the value list and the alternation fix
// (AC-18).
func TestParserMissingEnumValueRefusal(t *testing.T) {
	_, err := NewParser(refusalSchema()).Parse("bgp { peer peer1 { session { capability { route-refresh; } } } }")
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, "bgp peer peer1 session capability route-refresh: needs a value\nroute-refresh accepts only these values:\n")
	assert.True(t, strings.HasSuffix(msg, "\nfix: route-refresh <enable|disable|require|refuse>;"), msg)
	assert.NotContains(t, msg, "needs a value, one of")
}

// TestEveryValueArmNamesTheFullPath walks each value and key arm of the
// hierarchical parser with a value outside its enumeration and asserts the
// refusal names the full path once, with spaces, keys included, and no slash
// path. An arm that returns its refusal outside syntaxRefusal names only its
// own leaf, so its row goes red (AC-5, AC-25, AC-39).
func TestEveryValueArmNamesTheFullPath(t *testing.T) {
	tests := []struct {
		arm, input, problem string
	}{
		{"leaf", "top { item a { color pink; } }", `top item a color: "pink"`},
		{"multi-leaf", "top { item a { words pink; } }", `top item a words: "pink"`},
		{"bracket leaf-list", "top { item a { tags [ red pink ]; } }", `top item a tags: "pink"`},
		{"value-or-array", "top { item a { modes [ red pink ]; } }", `top item a modes: "pink"`},
		{"inactive bracket member", "top { item a { inactive: tags pink; } }", `top item a tags: "pink"`},
		{"inactive value-or-array member", "top { item a { inactive: modes pink; } }", `top item a modes: "pink"`},
		{"list key", "top { shade pink { label x; } }", `top shade pink: "pink"`},
		{"list key in an inline block", "top { shade { pink; } }", `top shade pink: "pink"`},
		{"inline entry child", "top { pair a pink; }", `top pair a color: "pink"`},
		{"list key, enum key list", "ospf { redistribute bogus { metric 30; } }", `ospf redistribute bogus: "bogus"`},
	}
	for _, tt := range tests {
		t.Run(tt.arm, func(t *testing.T) {
			_, err := NewParser(refusalSchema()).Parse(tt.input)
			require.Error(t, err)
			msg := err.Error()
			assert.Contains(t, msg, tt.problem+" is not an accepted value")
			assert.NotContains(t, msg, "top/")
			assert.NotContains(t, msg, "ospf/")
			assert.NotContains(t, msg, "(valid: ")
			assert.Contains(t, msg, "\nfix: ")
		})
	}
}

// TestParserRefusesUnionNonMember asserts every union leaf keeps refusing a
// value no member accepts, the refusal listing the enumeration member's
// values and naming the other member types in words, and keeps accepting a
// value a member accepts (AC-13, AC-14).
func TestParserRefusesUnionNonMember(t *testing.T) {
	addr := &LeafNode{Type: TypeString, Union: []UnionMember{
		{TypeName: "enumeration", Leaf: &LeafNode{Type: TypeString, Enums: []string{"auto", "none"}, EnumHelp: map[string]string{"auto": "Pick one"}}},
		{TypeName: "ip-address", Leaf: Leaf(TypeIPv4)},
	}}
	schema := NewSchema()
	schema.Define("addr", addr)

	_, err := NewParser(schema).Parse("addr 192.0.2.1;")
	require.NoError(t, err)
	_, err = NewSetParser(schema).Parse("set addr none\n")
	require.NoError(t, err)

	_, err = NewParser(schema).Parse("addr autoo;")
	require.Error(t, err)
	want := "line 1: addr: \"autoo\" is not an accepted value (did you mean auto?)\n" +
		"addr accepts one of these values:\n" +
		"  auto  Pick one\n" +
		"  none\n" +
		"or a value of type ip-address\n" +
		"fix: addr auto;"
	assert.Equal(t, want, err.Error())

	_, err = NewSetParser(schema).Parse("set addr zzzzzz\n")
	require.Error(t, err)
	assert.True(t, strings.HasSuffix(err.Error(), "\nfix: set addr <auto|none|ip-address>"), err.Error())
	assert.NotContains(t, err.Error(), "matches no member type")
}

// TestSetParserEnumRefusalFrame asserts the set parsers' frame: the path the
// line consumed, the line's own verb in the fix (nop on a nop line), the set
// and set-meta forms alike, and a key refused where the entry would be
// created, with the rest of the line kept in the fix and nothing created
// (AC-24, AC-25 file case).
func TestSetParserEnumRefusalFrame(t *testing.T) {
	schema := refusalSchema()
	line := "set bgp peer peer1 session capability route-refresh requre\n"
	for name, parse := range map[string]func(string) error{
		"set":      func(in string) error { _, err := NewSetParser(schema).Parse(in); return err },
		"set-meta": func(in string) error { _, _, err := NewSetParser(schema).ParseWithMeta(in); return err },
	} {
		t.Run(name, func(t *testing.T) {
			err := parse(line)
			require.Error(t, err)
			msg := err.Error()
			assert.True(t, strings.HasPrefix(msg, "line 1: bgp peer peer1 session capability route-refresh: \"requre\" is not an accepted value (did you mean require?)\n"), msg)
			assert.True(t, strings.HasSuffix(msg, "\nfix: set bgp peer peer1 session capability route-refresh require"), msg)
			assert.NotContains(t, msg, "invalid value for")
			assert.NotContains(t, msg, "write:")

			err = parse("nop bgp peer peer1 session capability route-refresh requre\n")
			require.Error(t, err)
			assert.True(t, strings.HasSuffix(err.Error(), "\nfix: nop bgp peer peer1 session capability route-refresh require"), err.Error())
		})
	}

	tree, err := NewSetParser(schema).Parse("set ospf redistribute connected metric 10\n")
	require.NoError(t, err)
	assert.NotNil(t, tree.GetContainer("ospf").GetList("redistribute")["connected"], "control: the creator stores an accepted key")

	tree = NewTree()
	parser := NewSetParser(schema)
	err = parser.parseLine(tree, "set ospf redistribute bogus metric 30", 1)
	require.Error(t, err)
	msg := err.Error()
	assert.True(t, strings.HasPrefix(msg, "line 1: ospf redistribute bogus: \"bogus\" is not an accepted value\n"), msg)
	assert.True(t, strings.HasSuffix(msg, "\nfix: set ospf redistribute <connected|static> metric 30"), msg)
	assert.Nil(t, tree.GetContainer("ospf").GetList("redistribute")["bogus"], "a refused key creates no entry")

	_, err = NewSetParser(schema).Parse("set top shade pink\n")
	require.Error(t, err, "the key-only line goes through the creator")
	assert.True(t, strings.HasSuffix(err.Error(), "\nfix: set top shade <red|blue>"), err.Error())
}

// TestSetParserLeafListEnumRefusal asserts a set-format leaf-list member
// outside the enumeration is refused naming the leaf-list, with declared
// order and no "(valid: " text (AC-5).
func TestSetParserLeafListEnumRefusal(t *testing.T) {
	_, err := NewSetParser(refusalSchema()).Parse("set top item a modes [ red pink ]\n")
	require.Error(t, err)
	msg := err.Error()
	assert.True(t, strings.HasPrefix(msg, "line 1: top item a modes: \"pink\" is not an accepted value\nmodes accepts only these values:\n  red\n  blue\n"), msg)
	assert.True(t, strings.HasSuffix(msg, "\nfix: set top item a modes <red|blue>"), msg)
	assert.NotContains(t, msg, "(valid: ")
}

// TestEnsureListEntryJudgesNewKey asserts the one creator refuses a typed new
// key its key leaf refuses, creating nothing, and creates an accepted one.
// Removing the creator's check turns the first half red (AC-25, AC-32).
func TestEnsureListEntryJudgesNewKey(t *testing.T) {
	list := refusalList(t, "ospf", "redistribute")
	tree := NewTree()

	_, err := EnsureListEntry(tree, []string{"ospf", "redistribute"}, list, ListAddress{Key: "bogus", First: "bogus", Tokens: 1})
	var refusal *configyang.ValidationError
	require.True(t, errors.As(err, &refusal), "refusal %v", err)
	assert.Equal(t, []string{"ospf", "redistribute", "bogus"}, refusal.Segments())
	assert.Equal(t, "source", refusal.Leaf())
	assert.Nil(t, tree.GetList("redistribute")["bogus"])

	entry, err := EnsureListEntry(tree, []string{"ospf", "redistribute"}, list, ListAddress{Key: "connected", First: "connected", Tokens: 1})
	require.NoError(t, err)
	assert.Same(t, entry, tree.GetList("redistribute")["connected"])
}

// TestTypedDefaultKeyStaysRefused asserts a typed `default` and a typed
// `x#3` on an enum key are judged as typed, with no suffix stripped, so both
// stay refused at creation, on the creator and through a set line. Widening
// the Tokens == 0 exemption to them turns this red (AC-14).
func TestTypedDefaultKeyStaysRefused(t *testing.T) {
	shade := refusalList(t, "top", "shade")
	for _, key := range []string{KeyDefault, "red#3"} {
		_, err := EnsureListEntry(NewTree(), []string{"top", "shade"}, shade, ListAddress{Key: key, First: key, Tokens: 1})
		require.Error(t, err, "typed key %q", key)
		_, err = NewSetParser(refusalSchema()).Parse("set top shade " + key + " label x\n")
		require.Error(t, err, "set line with key %q", key)
	}
}

// refusalList answers the list the test schema defines at root's child name.
func refusalList(t *testing.T, root, name string) *ListNode {
	t.Helper()
	container, ok := refusalSchema().Get(root).(*ContainerNode)
	require.True(t, ok, "the test schema defines %s", root)
	list, ok := container.Get(name).(*ListNode)
	require.True(t, ok, "the test schema defines %s %s", root, name)
	return list
}

// storedShadeTree answers a root holding `top shade` entries under the keys
// the shade list's key leaf refuses, `default` and `red#2`, each with the
// label "kept": entries a lookup or a read-back stores, never a typed key.
func storedShadeTree() *Tree {
	shades := NewTree()
	for _, key := range []string{KeyDefault, "red#2"} {
		entry := NewTree()
		entry.Set("label", "kept")
		shades.AddListEntry("shade", key, entry)
	}
	root := NewTree()
	root.SetContainer("top", shades)
	return root
}

// TestSetLineKeyOnStoredEntryStaysRefused asserts a set line that types
// `default` or `red#2`, keys the shade list's key leaf refuses, is refused
// even when an entry under that key is already stored, as the set parser
// refused it before the creator judged keys: on the entry line, the key-only
// line and the set-meta line. The refusal is the creator's, framed for the
// line, and the stored entry is unchanged. Dropping the set parser's own key
// check (lineKeyRefusal) turns this red, since the creator returns a stored
// entry unjudged (AC-14).
func TestSetLineKeyOnStoredEntryStaysRefused(t *testing.T) {
	for _, key := range []string{KeyDefault, "red#2"} {
		displayKey := key
		if key == "red#2" {
			displayKey = `"red#2"`
		}
		lines := map[string]struct {
			line, fix string
			parse     func(p *SetParser, root *Tree, line string) error
		}{
			"entry line": {"set top shade " + key + " label changed", "fix: set top shade <red|blue> label changed",
				func(p *SetParser, root *Tree, line string) error { return p.parseLine(root, line, 1) }},
			"key-only line": {"set top shade " + key, "fix: set top shade <red|blue>",
				func(p *SetParser, root *Tree, line string) error { return p.parseLine(root, line, 1) }},
			"set-meta line": {"set top shade " + key + " label changed", "fix: set top shade <red|blue> label changed",
				func(p *SetParser, root *Tree, line string) error {
					return p.parseLineWithMeta(root, NewMetaTree(), line, MetaEntry{User: "alice"}, 1)
				}},
		}
		for name, tt := range lines {
			t.Run(key+" "+name, func(t *testing.T) {
				root := storedShadeTree()
				err := tt.parse(NewSetParser(refusalSchema()), root, tt.line)
				require.Error(t, err)
				msg := err.Error()
				assert.True(t, strings.HasPrefix(msg, "line 1: top shade "+displayKey+": \""+key+"\" is not an accepted value\nid accepts only these values:\n  red\n  blue\n"), msg)
				assert.True(t, strings.HasSuffix(msg, "\n"+tt.fix), msg)
				label, _ := root.GetContainer("top").GetList("shade")[key].Get("label")
				assert.Equal(t, "kept", label, "the stored entry is unchanged")
			})
		}
	}
}

// TestInactiveLineReachesStoredDuplicateEntry asserts an `inactive` set line
// naming a stored `red#2` entry marks it inactive, as the set parser did
// before the creator judged keys: the inactive walk never judged a typed key,
// and the creator returns a stored entry unjudged. A new entry under a refused
// key is refused where it would be created (Q-8a), and nothing is created.
func TestInactiveLineReachesStoredDuplicateEntry(t *testing.T) {
	root := storedShadeTree()
	parser := NewSetParser(refusalSchema())
	require.NoError(t, parser.parseLine(root, "inactive top shade red#2", 1))
	assert.True(t, root.GetContainer("top").GetList("shade")["red#2"].IsInactive())

	err := parser.parseLine(root, "inactive top shade pink", 1)
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "line 1: top shade pink: \"pink\" is not an accepted value\n"), err.Error())
	assert.True(t, strings.HasSuffix(err.Error(), "\nfix: inactive top shade <red|blue>"), err.Error())
	assert.Nil(t, root.GetContainer("top").GetList("shade")["pink"], "a refused new key creates no entry")
}

// TestLookupWalkFindsEveryStoredKey stores an anonymous entry and a `#N`
// duplicate, keys no key check produced, and asserts the lookup step, the
// creator given a lookup's address, the set parser's delete and its nop
// marker all still reach them (R-15, AC-32).
func TestLookupWalkFindsEveryStoredKey(t *testing.T) {
	shade := refusalList(t, "top", "shade")

	tree := NewTree()
	anonymous := NewTree()
	tree.AddListEntry("shade", KeyDefault, anonymous)
	addr, err := ListPathStep("shade", shade, nil)
	require.NoError(t, err)
	found, err := EnsureListEntry(tree, []string{"top", "shade"}, shade, addr)
	require.NoError(t, err)
	assert.Same(t, anonymous, found, "the anonymous step reaches the stored anonymous entry")

	duplicate := NewTree()
	tree.AddListEntry("shade", "red#2", duplicate)
	addr, err = ListPathStep("shade", shade, []string{"red#2"})
	require.NoError(t, err)
	found, err = EnsureListEntry(tree, []string{"top", "shade"}, shade, addr)
	require.NoError(t, err, "a stored #N entry is returned unjudged")
	assert.Same(t, duplicate, found)

	root := NewTree()
	root.SetContainer("top", tree)
	parser := NewSetParser(refusalSchema())
	parser.markNopInactive(root, parser.schema.root, []string{"top", "shade", "red#2"})
	assert.True(t, duplicate.IsInactive(), "the nop marker reaches the #N entry")
	require.NoError(t, parser.parseLine(root, "delete top shade red#2", 1))
	assert.Nil(t, tree.GetList("shade")["red#2"], "delete reaches the #N entry")
}

// TestChangeFileReadsBackUnjudged reads back change-file metadata holding a
// stored "default" key on a list whose key refuses it, the key an edit
// under an anonymous entry leaves, and asserts the entry is materialized,
// not refused. Judging Tokens == 0 addresses turns this red (R-15).
func TestChangeFileReadsBackUnjudged(t *testing.T) {
	schema := refusalSchema()
	meta := NewMetaTree()
	meta.GetOrCreateContainer("top").GetOrCreateContainer("shade").GetOrCreateListEntry(KeyDefault).SetEntry("label", MetaEntry{User: "alice", Value: "x"})
	tree := NewTree()
	require.NoError(t, materializeMetaPaths(tree, meta, schema))
	assert.NotNil(t, tree.GetContainer("top").GetList("shade")[KeyDefault], "the stored default key reads back")
}

// TestValidateLeafValueRangeLengthPatternRefusal asserts the range, length
// and pattern kinds' problem, why and fix, the declared error-message as the
// why, and the type width reported as the range (AC-35 to AC-37).
func TestValidateLeafValueRangeLengthPatternRefusal(t *testing.T) {
	cost := &LeafNode{Type: TypeUint32, Ranges: []NumericRange{{Min: "1", Max: "65535"}}}
	for _, value := range []string{"0", "70000", "99999999999"} {
		err := ValidateLeafValue(cost, []string{"ospf", "cost"}, value)
		require.Error(t, err)
		assert.Equal(t, "ospf cost: \""+value+"\" is outside the accepted range\ncost accepts a value in 1..65535\nfix: set ospf cost <1..65535>", err.Error())
	}

	run := &LeafNode{Type: TypeString, Lengths: []NumericRange{{Min: "1", Max: "4096"}}}
	err := ValidateLeafValue(run, []string{"process", "run"}, "")
	require.Error(t, err)
	assert.Equal(t, "process run: \"\" is outside the accepted length (0 characters)\nrun accepts 1..4096 characters\nfix: set process run <1..4096 characters>", err.Error())

	rate := &LeafNode{Type: TypeString, Patterns: []string{"[0-9]+(k|m|g)bit"}, Format: configyang.NewValueFormat("a traffic rate", "rate")}
	err = ValidateLeafValue(rate, []string{"shaper", "default-rate"}, "fast")
	require.Error(t, err)
	assert.Equal(t, "shaper default-rate: \"fast\" is not in the accepted format\ndefault-rate accepts a traffic rate\nfix: set shaper default-rate <rate>", err.Error())
	assert.NotContains(t, err.Error(), "[0-9]")

	address := &LeafNode{Type: TypeIPv4}
	err = ValidateLeafValue(address, []string{"router-address"}, "10.0.0.256")
	require.EqualError(t, err, `invalid IPv4: "10.0.0.256"`, "a type refusal keeps its text")
}

// TestValidateLeafValueDecimal64Range asserts a ranged decimal64 leaf takes
// an in-range value, refuses an out-of-range one with the decimal bounds, and
// keeps the two type refusals (AC-41). Before, every value was refused,
// "range validation not supported for type string".
func TestValidateLeafValueDecimal64Range(t *testing.T) {
	multiplier := &LeafNode{Type: TypeDecimal64, FractionDigits: 2, Ranges: []NumericRange{{Min: "1.00", Max: "100.00"}}}
	path := []string{"bfd", "threshold-multiplier"}
	for _, value := range []string{"5.00", "1.00", "100.00", "2.5"} {
		require.NoError(t, ValidateLeafValue(multiplier, path, value), value)
	}
	for _, value := range []string{"0.99", "100.01", "500.00"} {
		err := ValidateLeafValue(multiplier, path, value)
		require.Error(t, err, value)
		assert.Contains(t, err.Error(), "is outside the accepted range\nthreshold-multiplier accepts a value in 1.00..100.00\n")
	}
	assert.EqualError(t, ValidateLeafValue(multiplier, path, "5.001"), `expected decimal64 with at most 2 fraction digits, got "5.001"`)
	assert.EqualError(t, ValidateLeafValue(multiplier, path, "abc"), `expected decimal64, got "abc"`)
}

// TestRefuseInvalidCustomSectionsOneRefusalPerLine asserts the section walk's
// refusals are joined one per line, never with "; " (AC-10).
func TestRefuseInvalidCustomSectionsOneRefusalPerLine(t *testing.T) {
	first := configyang.EnumRefusal(configyang.RefusedValue{Path: []string{"a", "b"}, Leaf: "b", Value: "x"}, nil)
	second := configyang.RangeRefusal(configyang.RefusedValue{Path: []string{"a", "c"}, Leaf: "c", Value: "0"}, "1..2", "")
	found := []SectionValidationError{{Section: "a", Err: *first}, {Section: "a", Err: *second}}
	assert.Equal(t, "a: "+first.Error(), found[0].Message())
	assert.NotContains(t, found[1].Message(), "redacted")
}

// TestSectionRefusalsJoinOnePerBlock asserts the walk's verdict over two
// value refusals by equality: the validation header, then each refusal as a
// block of its own after its section, each block ending with its own fix:
// line, the blocks parted by a newline and never by "; " (AC-10). The
// verdict stays an ErrCustomValidation.
func TestSectionRefusalsJoinOnePerBlock(t *testing.T) {
	first := configyang.EnumRefusal(configyang.RefusedValue{Path: []string{"ospf", "mode"}, Leaf: "mode", Value: "x"},
		[]suggest.Value{{Name: "fast"}, {Name: "slow"}})
	second := configyang.RangeRefusal(configyang.RefusedValue{Path: []string{"ospf", "cost"}, Leaf: "cost", Value: "0"}, "1..2", "")
	err := joinSectionRefusals([]SectionValidationError{{Section: "ospf", Err: *first}, {Section: "ospf", Err: *second}})
	require.ErrorIs(t, err, ErrCustomValidation)
	want := ErrCustomValidation.Error() + ":\n" +
		"ospf: ospf mode: \"x\" is not an accepted value\nmode accepts only these values:\n  fast\n  slow\nfix: set ospf mode <fast|slow>\n" +
		"ospf: ospf cost: \"0\" is outside the accepted range\ncost accepts a value in 1..2\nfix: set ospf cost <1..2>"
	assert.Equal(t, want, err.Error())
	assert.NotContains(t, err.Error(), "; ")
}

// TestLegacyHintPrecedesTheRefusal asserts the loader writes its ExaBGP
// hint before the refusal, so the refusal's fix stays the last line (AC-10).
func TestLegacyHintPrecedesTheRefusal(t *testing.T) {
	refusal := newLineError(3, errors.New("refused\nfix: do this"))
	err := &parseConfigError{cause: refusal, hint: "Hint: migrate"}
	assert.Equal(t, "parse config: Hint: migrate\n\nline 3: refused\nfix: do this", err.Error())
	line, message := SplitLine(err)
	assert.Equal(t, 3, line)
	assert.Equal(t, "parse config: Hint: migrate\n\nrefused\nfix: do this", message)
}

// TestInlineLeafListEnumRefusal writes a one-line list entry whose leaf-list
// child, in the bracket and the value-or-array kind, holds a member outside
// the enumeration, and asserts the member's refusal names the full path once,
// keys included, lists the accepted values in declared order, carries no
// "(valid: " text and ends with a fix: line; the same entry with accepted
// members parses (AC-5).
//
// The one-line entry is positional: its values fill the item's children in
// declared order (color, words, tags, modes), a leaf-list before the last
// child takes one member, and a bracket list fills the last child. So the
// bracket kind (tags) is reached as the third value, and the value-or-array
// kind (modes), the last child, by a bracket list.
func TestInlineLeafListEnumRefusal(t *testing.T) {
	entries := map[string]struct{ refused, accepted string }{
		"tags":  {"top { item a red red pink; }", "top { item a red red blue; }"},
		"modes": {"top { item a [ red pink ]; }", "top { item a [ red blue ]; }"},
	}
	for child, tt := range entries {
		t.Run(child, func(t *testing.T) {
			_, err := NewParser(refusalSchema()).Parse(tt.refused)
			require.Error(t, err)
			msg := err.Error()
			assert.True(t, strings.HasPrefix(msg, "line 1: top item a "+child+": \"pink\" is not an accepted value\n"+child+" accepts only these values:\n  red\n  blue\nfix: "), msg)
			assert.NotContains(t, msg, "(valid: ")
			assert.NotContains(t, msg, "top/")

			_, err = NewParser(refusalSchema()).Parse(tt.accepted)
			require.NoError(t, err, "control: accepted members parse")
		})
	}
}

// keyPairSchema is a list keyed by two leaves, `peer vrf`, the BFD
// single-hop-session shape: the first key value is typed bare, a later key
// leaf as a `vrf <value>` pair, and vrf is an enumeration.
func keyPairSchema() *Schema {
	session := List(TypeString,
		Field("peer", Leaf(TypeString)),
		Field("vrf", &LeafNode{Type: TypeString, Enums: []string{"red", "blue"}}),
		Field("label", Leaf(TypeString)),
	)
	session.KeyName = "peer vrf"
	session.KeyLeaf = &LeafNode{Type: TypeString}
	schema := NewSchema()
	schema.Define("top", Container(Field("session", session)))
	return schema
}

// TestParseListAddressPairRefusal asserts a non-first key pair whose value
// its leaf refuses is answered as that leaf's refusal, bare: the path names
// the list, the first key value and the key leaf, with no "invalid <leaf> for
// <list>:" prefix; through a set line the path starts at the root and the fix
// is the line with the pair's value replaced and the rest kept. An accepted
// pair is read into the address (AC-42).
func TestParseListAddressPairRefusal(t *testing.T) {
	top, ok := keyPairSchema().Get("top").(*ContainerNode)
	require.True(t, ok)
	session, ok := top.Get("session").(*ListNode)
	require.True(t, ok)

	_, err := ParseListAddress("session", session, []string{"192.0.2.1", "vrf", "pink", "label", "x"})
	var refusal *configyang.ValidationError
	require.True(t, errors.As(err, &refusal), "refusal %v", err)
	assert.Equal(t, []string{"session", "192.0.2.1", "vrf"}, refusal.Segments())
	assert.Equal(t, "vrf", refusal.Leaf())
	assert.True(t, strings.HasPrefix(err.Error(), "session 192.0.2.1 vrf: \"pink\" is not an accepted value\nvrf accepts only these values:\n  red\n  blue\n"), err.Error())
	assert.NotContains(t, err.Error(), "invalid vrf for session")

	addr, err := ParseListAddress("session", session, []string{"192.0.2.1", "vrf", "blue", "label", "x"})
	require.NoError(t, err)
	assert.Equal(t, []ListKeyPair{{Leaf: "vrf", Value: "blue"}}, addr.Pairs)
	assert.Equal(t, 3, addr.Tokens)

	_, err = NewSetParser(keyPairSchema()).Parse("set top session 192.0.2.1 vrf pink label x\n")
	require.Error(t, err)
	msg := err.Error()
	assert.True(t, strings.HasPrefix(msg, "line 1: top session 192.0.2.1 vrf: \"pink\" is not an accepted value\n"), msg)
	assert.True(t, strings.HasSuffix(msg, "\nfix: set top session 192.0.2.1 vrf <red|blue> label x"), msg)
}

// TestValidateNodeValueLeavesNonLeafNodesToTheEditor pins that the shared
// set-time value check judges only what carries a value restriction.
//
// Goal: `set firewall ... then accept true` sets a presence container, and a
// value check that refused every non-leaf node refused it with "accept holds
// no value" on the SSH editor and the web.
// Prevents: the value check turning away a setting it has no restriction for.
func TestValidateNodeValueLeavesNonLeafNodesToTheEditor(t *testing.T) {
	path := []string{"firewall", "table", "filter", "chain", "input", "term", "t", "then", "accept"}
	for name, node := range map[string]Node{
		"presence container": &ContainerNode{},
		"flex":               &FlexNode{},
		"freeform":           &FreeformNode{},
		"inline list":        &InlineListNode{},
	} {
		require.NoError(t, ValidateNodeValue(node, path, "true"), name)
	}
	// A path the schema does not resolve still has no answer.
	require.Error(t, ValidateNodeValue(nil, path, "true"))
}
