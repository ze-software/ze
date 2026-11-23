// VALIDATES: the relocated-container and retired-leaf registries: a
// registration makes the load parsers refuse the old shape with its own hint
// naming `ze config migrate`, MigrationSchema accepts the old shape, and
// ApplyRelocations moves a block to its top-level root. Ze writes the
// conversion of a format change only once it has a stable release, so each
// test registers a test-only relocation or retired leaf and restores the
// registry when it ends.
// PREVENTS: the registries and their hints rotting while no production
// package registers into them.
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/config/redistribute/yang" // the redistribute root the tests relocate
	_ "github.com/ze-software/ze/internal/component/hub/yang"                 // the environment container the tests use as old parent
)

const (
	testRetiredLeaf = "retired-test-leaf"
	testReplacement = "new-test-leaf N"
)

// assertMigrateHint checks that err is an unknown-field refusal carrying the
// registered hint (want) and the migrate command, in place of a closest-key
// guess.
func assertMigrateHint(t *testing.T, err error, want string) {
	t.Helper()
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, want)
	assert.Contains(t, msg, "fix: ze config migrate -o <new-file> <file>")
	assert.NotContains(t, msg, "did you mean", "the registered hint replaces the closest-key guess")
}

// TestRetiredLeafRefusedWithItsHint: a leaf registered as retired, in a
// container and in a list entry, is refused by both load parsers with the
// replacement and the migrate command, and MigrationSchema parses it.
func TestRetiredLeafRefusedWithItsHint(t *testing.T) {
	t.Cleanup(RegisterRetiredLeafForTest([]string{"environment"}, testRetiredLeaf, Leaf(TypeUint32), testReplacement))
	t.Cleanup(RegisterRetiredLeafForTest([]string{"redistribute", "destination"}, testRetiredLeaf, Leaf(TypeUint32), testReplacement))

	schema, err := YANGSchema()
	require.NoError(t, err)

	const inContainer = "environment {\n\t" + testRetiredLeaf + " 5\n}\n"
	containerHint := testRetiredLeaf + " is no longer a leaf of environment: write " + testReplacement + " instead"
	_, err = NewParser(schema).Parse(inContainer)
	assertMigrateHint(t, err, containerHint)
	_, err = NewSetParser(schema).Parse("set environment " + testRetiredLeaf + " 5\n")
	assertMigrateHint(t, err, containerHint)

	const inList = "redistribute {\n\tdestination ospf {\n\t\t" + testRetiredLeaf + " 5\n\t}\n}\n"
	listHint := testRetiredLeaf + " is no longer a leaf of redistribute destination: write " + testReplacement + " instead"
	_, err = NewParser(schema).Parse(inList)
	assertMigrateHint(t, err, listHint)
	_, err = NewSetParser(schema).Parse("set redistribute destination ospf " + testRetiredLeaf + " 5\n")
	assertMigrateHint(t, err, listHint)

	migrationSchema, err := MigrationSchema()
	require.NoError(t, err)
	tree, err := NewParser(migrationSchema).Parse(inContainer)
	require.NoError(t, err, "the migration schema accepts the retired leaf")
	value, ok := tree.GetContainer("environment").Get(testRetiredLeaf)
	require.True(t, ok)
	assert.Equal(t, "5", value)

	_, err = NewParser(schema).Parse(inContainer)
	assertMigrateHint(t, err, containerHint)
}

// TestRetiredLeafForTestRestores: after the restore function runs, a fresh
// schema no longer carries the retired-leaf hint.
func TestRetiredLeafForTestRestores(t *testing.T) {
	restore := RegisterRetiredLeafForTest([]string{"environment"}, testRetiredLeaf, Leaf(TypeUint32), testReplacement)
	restore()

	schema, err := YANGSchema()
	require.NoError(t, err)
	_, err = NewParser(schema).Parse("environment {\n\t" + testRetiredLeaf + " 5\n}\n")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "is no longer a leaf of")
}

// TestRelocatedContainerRefusedWithItsHint: a block registered as relocated
// is refused at its old position by both load parsers, naming the new
// top-level root and the migrate command, and MigrationSchema parses it there.
func TestRelocatedContainerRefusedWithItsHint(t *testing.T) {
	t.Cleanup(RegisterRelocatedContainerForTest("environment", "redistribute"))

	schema, err := YANGSchema()
	require.NoError(t, err)

	const oldPosition = "environment {\n\tredistribute {\n\t\tdestination ospf {\n\t\t}\n\t}\n}\n"
	const hint = "redistribute is no longer inside environment: it is now the top-level redistribute { ... } block"
	_, err = NewParser(schema).Parse(oldPosition)
	assertMigrateHint(t, err, hint)
	_, err = NewSetParser(schema).Parse("set environment redistribute destination ospf\n")
	assertMigrateHint(t, err, hint)

	migrationSchema, err := MigrationSchema()
	require.NoError(t, err)
	tree, err := NewParser(migrationSchema).Parse(oldPosition)
	require.NoError(t, err, "the migration schema accepts the block at its old position")
	assert.True(t, HasRelocatedContainers(tree))
}

// relocationTree answers a tree holding a destination entry under
// environment redistribute (the old position) and, when current is not "",
// one under the top-level redistribute root.
func relocationTree(old, current string) *Tree {
	tree := NewTree()
	moved := NewTree()
	moved.AddListEntry("destination", old, NewTree())
	environment := NewTree()
	environment.SetContainer("redistribute", moved)
	tree.SetContainer("environment", environment)
	if current != "" {
		root := NewTree()
		root.AddListEntry("destination", current, NewTree())
		tree.SetContainer("redistribute", root)
	}
	return tree
}

// TestApplyRelocationsMovesBlockToRoot: ApplyRelocations moves the block from
// its old position into the top-level root, merging it with the entries the
// root already holds.
func TestApplyRelocationsMovesBlockToRoot(t *testing.T) {
	t.Cleanup(RegisterRelocatedContainerForTest("environment", "redistribute"))

	tree := relocationTree("ospf", "bgp")
	require.True(t, HasRelocatedContainers(tree))

	moved, err := ApplyRelocations(tree)
	require.NoError(t, err)
	assert.False(t, HasRelocatedContainers(moved))
	assert.Nil(t, moved.GetContainer("environment").GetContainer("redistribute"), "the old position is empty")
	destinations := moved.GetContainer("redistribute").GetList("destination")
	assert.Contains(t, destinations, "ospf", "the moved entry reached the root")
	assert.Contains(t, destinations, "bgp", "the root's own entry stays")
}

// TestApplyRelocationsRefusesEntryAtBothPositions: an entry configured at the
// old and the new position is refused rather than one overwriting the other.
func TestApplyRelocationsRefusesEntryAtBothPositions(t *testing.T) {
	t.Cleanup(RegisterRelocatedContainerForTest("environment", "redistribute"))

	_, err := ApplyRelocations(relocationTree("ospf", "ospf"))
	require.ErrorIs(t, err, errRelocationConflict)
	assert.Contains(t, err.Error(), "environment redistribute destination ospf and redistribute destination ospf")
}

// TestRelocatedContainerForTestRestores: after the restore function runs, no
// relocation is detected and a fresh schema refuses the old position without
// the relocation hint.
func TestRelocatedContainerForTestRestores(t *testing.T) {
	restore := RegisterRelocatedContainerForTest("environment", "redistribute")
	restore()

	assert.False(t, HasRelocatedContainers(relocationTree("ospf", "")))
	schema, err := YANGSchema()
	require.NoError(t, err)
	_, err = NewParser(schema).Parse("environment {\n\tredistribute {\n\t}\n}\n")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "is no longer inside")
}
