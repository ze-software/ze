package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests of this file pin what the definition order does to a saved file
// and what it leaves alone (docs/architecture/config/syntax.md, "Child
// order"): a committed set-meta file reorders once and then holds still, and
// the values no schema node names keep their sorted print.

// TestSetMetaResaveStable: a set-meta file saved in the old name order loads,
// saves its lines in definition order, loads again to the same tree, and a
// second save changes no byte. Method: the fixture container direct declares
// remote, local, timers; the stored file holds local, remote, timers, the
// order the converter gave before.
//
// VALIDATES: AC-7, R-1, R-9.
// PREVENTS: a committed file that keeps moving on every save, which fires the
// archive and history digests on every commit, or a reorder that changes what
// the file parses to.
func TestSetMetaResaveStable(t *testing.T) {
	schema := orderFixtureSchema(t, "order-groupings.yang", "order-blocks-conf.yang")
	stored := "#alice set direct local l\n" +
		"#alice set direct remote r\n" +
		"#alice set direct timers hold 3\n"

	tree, meta, err := NewSetParser(schema).ParseWithMeta(stored)
	require.NoError(t, err)
	first := SerializeSetWithMeta(tree, meta, schema)
	remote := strings.Index(first, "set direct remote r")
	local := strings.Index(first, "set direct local l")
	timers := strings.Index(first, "set direct timers hold 3")
	require.NotEqual(t, -1, remote, first)
	require.NotEqual(t, -1, local, first)
	require.NotEqual(t, -1, timers, first)
	assert.Less(t, remote, local, "remote is declared before local:\n%s", first)
	assert.Less(t, local, timers, "local is declared before timers:\n%s", first)

	reloaded, reloadedMeta, err := NewSetParser(schema).ParseWithMeta(first)
	require.NoError(t, err, first)
	assert.Equal(t, tree.ToMap(), reloaded.ToMap(), "the reordered file parses to the stored tree")
	assert.Equal(t, first, SerializeSetWithMeta(reloaded, reloadedMeta, schema),
		"a second save changes no byte")
}

// TestFreeformStaysSorted: the values no schema node orders print sorted,
// beside schema children that print in definition order. Method: a container
// declaring second, then first, then a freeform block, whose words and whose
// two unknown values are written against name order.
//
// VALIDATES: AC-8, D-3.
// PREVENTS: the definition order reaching values that have no definition, so
// that their print follows insertion or map order.
func TestFreeformStaysSorted(t *testing.T) {
	schema := NewSchema()
	schema.Define("top", Container(
		Field("second", Leaf(TypeString)),
		Field("first", Leaf(TypeString)),
		Field("free", Freeform()),
	))
	tree, err := NewParser(schema).Parse("top { first f; second s; free { zulu; alpha; mike; } }")
	require.NoError(t, err)
	top := tree.GetOrCreateContainer("top")
	top.Set("zz-unknown", "z")
	top.Set("aa-unknown", "a")

	printed := Serialize(tree, schema)
	previous := -1
	for _, line := range []string{"second s", "first f", "alpha", "mike", "zulu", "aa-unknown a", "zz-unknown z"} {
		at := strings.Index(printed, line)
		require.NotEqual(t, -1, at, "output lacks %q:\n%s", line, printed)
		assert.Less(t, previous, at, "%q prints out of order:\n%s", line, printed)
		previous = at
	}
}
