// Design: yang_schema.go -- yangToLeaf lifts union member types; schema.go -- ValidateLeafValue
//
// VALIDATES: a union-typed list key, and a union leaf, are judged against each
// member type, so a value no member accepts is refused at every parser path.
// PREVENTS: `bgp policy prefix-list <n> entry <key>` accepting any string as a prefix.
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// unionKeyModule mirrors the prefix-list entry list: a key that is a union of
// two pattern-restricted string types, plus a ranged integer member so the test
// also shows a member's range is applied.
const unionKeyModule = `module m { namespace "urn:m"; prefix m;
	typedef v4 { type string { pattern '[0-9.]+/[0-9]+'; } }
	typedef v6 { type string { pattern '[0-9a-f:]+/[0-9]+'; } }
	list entry {
		key "prefix";
		leaf prefix { type union { type v4; type v6; type uint8 { range "1..10"; } } }
	}
}`

// unionKeyList loads unionKeyModule and converts the list the way the schema
// build does on load.
func unionKeyList(t *testing.T) *ListNode {
	t.Helper()
	loader := yang.NewLoader()
	require.NoError(t, loader.AddModuleFromText("m.yang", unionKeyModule))
	schema, err := loader.Resolve()
	require.NoError(t, err)
	entry := schema.GetEntry("m")
	require.NotNil(t, entry)
	listEntry := entry.Dir["entry"]
	require.NotNil(t, listEntry, "module must define list entry")
	resetSchemaBuildErrors()
	node := yangToNode(listEntry, "m/entry")
	require.NoError(t, flushSchemaBuildErrors())
	list, ok := node.(*ListNode)
	require.True(t, ok, "list entry converted to %T, want *ListNode", node)
	return list
}

// TestUnionListKeyJudgedAgainstEachMemberType converts the list and asks the
// key validator every parser calls about a value each member accepts and about
// values no member accepts. A value that matches neither pattern, and an
// integer outside the member's range, are refused; the refusal names the union.
func TestUnionListKeyJudgedAgainstEachMemberType(t *testing.T) {
	list := unionKeyList(t)

	for _, ok := range []string{"10.0.0.0/8", "2001:db8::/32", "5"} {
		assert.NoError(t, ValidateListKey(list, ok), "key %q matches a member type", ok)
	}
	for _, bad := range []string{"not-a-prefix", "11", "0"} {
		err := ValidateListKey(list, bad)
		if assert.Error(t, err, "key %q matches no member type and must be refused", bad) {
			assert.Contains(t, err.Error(), "union")
		}
	}
}
