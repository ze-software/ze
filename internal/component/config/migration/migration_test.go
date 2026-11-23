package migration

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// VALIDATES: the migration framework (Migrate, DryRun, NeedsMigration,
// ListTransformations and the registration API) runs what a package registers,
// in order, and leaves the input tree untouched. Ze writes the conversion of a
// format change only once it has a stable release, so each test registers its
// own test-only transformation and removes it when it ends.
// PREVENTS: a framework that cannot carry the first real conversion when one
// is written.

// errTestApply is the failure the failing test transformation answers.
var errTestApply = errors.New("test transformation refused")

// renameLeaf is a test-only transformation: while the top-level leaf from is
// set and to is not, it copies from's value into to.
func renameLeaf(name, from, to string) Transformation {
	return Transformation{
		Name:        name,
		Description: "Copy " + from + " into " + to + " (test only)",
		Detect: func(tree *config.Tree) bool {
			if _, old := tree.Get(from); !old {
				return false
			}
			_, done := tree.Get(to)
			return !done
		},
		Apply: func(tree *config.Tree) (*config.Tree, error) {
			value, _ := tree.Get(from)
			tree.Set(to, value)
			return tree, nil
		},
	}
}

// failing is a test-only transformation that is always needed, writes a leaf
// into the tree it is handed, then refuses.
func failing(name string) Transformation {
	return Transformation{
		Name:        name,
		Description: "Always refuse (test only)",
		Detect:      func(*config.Tree) bool { return true },
		Apply: func(tree *config.Tree) (*config.Tree, error) {
			tree.Set("touched", "yes")
			return nil, errTestApply
		},
	}
}

// register registers each transformation for the rest of the test.
func register(t *testing.T, ts ...Transformation) {
	t.Helper()
	for _, tr := range ts {
		t.Cleanup(RegisterTransformationForTest(tr))
	}
}

// transformationNames answers the names ListTransformations lists, in order.
func transformationNames() []string {
	var names []string
	for _, tr := range ListTransformations() {
		names = append(names, tr.Name)
	}
	return names
}

// --- Migrate() ---

// TestMigrate_NilTree verifies nil tree returns ErrNilTree.
func TestMigrate_NilTree(t *testing.T) {
	_, err := Migrate(nil)
	require.ErrorIs(t, err, ErrNilTree)
}

// TestMigrate_EmptyTree verifies an empty tree passes through with every
// transformation skipped.
func TestMigrate_EmptyTree(t *testing.T) {
	register(t, renameLeaf("test-rename", "old-leaf", "new-leaf"))

	result, err := Migrate(config.NewTree())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Applied)
	assert.Equal(t, transformationNames(), result.Skipped)
}

// TestMigrateAppliesRegisteredTransformation: a registered transformation
// whose Detect answers true runs, its output is the answer, and the input tree
// is left as it was.
func TestMigrateAppliesRegisteredTransformation(t *testing.T) {
	register(t, renameLeaf("test-rename", "old-leaf", "new-leaf"))

	tree := config.NewTree()
	tree.Set("old-leaf", "7")

	result, err := Migrate(tree)
	require.NoError(t, err)
	assert.Equal(t, []string{"test-rename"}, result.Applied)
	assert.Contains(t, result.Skipped, relocatedBlocksName)

	value, ok := result.Tree.Get("new-leaf")
	require.True(t, ok, "the migrated tree carries the rewritten leaf")
	assert.Equal(t, "7", value)

	_, touched := tree.Get("new-leaf")
	assert.False(t, touched, "Migrate works on a clone: the input tree is unchanged")
}

// TestMigrateSkipsTransformationNotNeeded: a registered transformation whose
// Detect answers false is listed as skipped and changes nothing.
func TestMigrateSkipsTransformationNotNeeded(t *testing.T) {
	register(t, renameLeaf("test-rename", "old-leaf", "new-leaf"))

	tree := config.NewTree()
	tree.Set("other-leaf", "1")

	result, err := Migrate(tree)
	require.NoError(t, err)
	assert.Empty(t, result.Applied)
	assert.Contains(t, result.Skipped, "test-rename")
	_, ok := result.Tree.Get("new-leaf")
	assert.False(t, ok)
}

// TestMigrateFailureLeavesTreeUnchanged: a transformation that refuses ends
// the migration with an error naming it, and what it wrote before refusing
// never reaches the input tree.
func TestMigrateFailureLeavesTreeUnchanged(t *testing.T) {
	register(t, failing("test-failing"))

	tree := config.NewTree()
	result, err := Migrate(tree)
	require.ErrorIs(t, err, errTestApply)
	assert.Contains(t, err.Error(), "test-failing")
	assert.Nil(t, result)
	_, touched := tree.Get("touched")
	assert.False(t, touched, "the input tree is unchanged on failure")
}

// TestMigrateRunsRegisteredInOrderAfterBuiltins: registered transformations run
// after the built-in ones, in registration order, each on the output of the
// one before.
func TestMigrateRunsRegisteredInOrderAfterBuiltins(t *testing.T) {
	register(t,
		renameLeaf("test-first", "old-leaf", "mid-leaf"),
		renameLeaf("test-second", "mid-leaf", "new-leaf"),
	)
	assert.Equal(t, []string{relocatedBlocksName, "test-first", "test-second"}, transformationNames())

	tree := config.NewTree()
	tree.Set("old-leaf", "7")

	result, err := Migrate(tree)
	require.NoError(t, err)
	assert.Equal(t, []string{"test-first", "test-second"}, result.Applied)
	value, ok := result.Tree.Get("new-leaf")
	require.True(t, ok, "the second transformation saw the first one's output")
	assert.Equal(t, "7", value)
}

// --- DryRun ---

// TestDryRun_NilTree verifies nil returns ErrNilTree.
func TestDryRun_NilTree(t *testing.T) {
	_, err := DryRun(nil)
	require.ErrorIs(t, err, ErrNilTree)
}

// TestDryRun_NoChanges verifies all-done case.
func TestDryRun_NoChanges(t *testing.T) {
	register(t, renameLeaf("test-rename", "old-leaf", "new-leaf"))

	result, err := DryRun(config.NewTree())
	require.NoError(t, err)
	assert.True(t, result.WouldSucceed)
	assert.Empty(t, result.WouldApply)
	assert.Equal(t, transformationNames(), result.AlreadyDone)
}

// TestDryRun_WouldApply verifies detection of needed changes, without
// changing the tree.
func TestDryRun_WouldApply(t *testing.T) {
	register(t, renameLeaf("test-rename", "old-leaf", "new-leaf"))

	tree := config.NewTree()
	tree.Set("old-leaf", "7")

	result, err := DryRun(tree)
	require.NoError(t, err)
	assert.True(t, result.WouldSucceed)
	assert.Equal(t, []string{"test-rename"}, result.WouldApply)
	_, touched := tree.Get("new-leaf")
	assert.False(t, touched, "a dry run changes nothing")
}

// TestDryRun_ReportsFailure: a transformation that would refuse is named in
// the analysis with its error, and the dry run itself answers no error.
func TestDryRun_ReportsFailure(t *testing.T) {
	register(t, failing("test-failing"))

	tree := config.NewTree()
	result, err := DryRun(tree)
	require.NoError(t, err)
	assert.False(t, result.WouldSucceed)
	assert.Equal(t, "test-failing", result.FailedAt)
	require.ErrorIs(t, result.Error, errTestApply)
	_, touched := tree.Get("touched")
	assert.False(t, touched, "a dry run changes nothing")
}

// --- NeedsMigration ---

// TestNeedsMigration_True verifies detection when migration needed.
func TestNeedsMigration_True(t *testing.T) {
	register(t, renameLeaf("test-rename", "old-leaf", "new-leaf"))

	tree := config.NewTree()
	tree.Set("old-leaf", "7")
	assert.True(t, NeedsMigration(tree))
}

// TestNeedsMigration_False verifies no detection when already migrated.
func TestNeedsMigration_False(t *testing.T) {
	register(t, renameLeaf("test-rename", "old-leaf", "new-leaf"))

	tree := config.NewTree()
	tree.Set("old-leaf", "7")
	tree.Set("new-leaf", "7")
	assert.False(t, NeedsMigration(tree))
}

// TestNeedsMigration_Nil verifies nil returns false.
func TestNeedsMigration_Nil(t *testing.T) {
	assert.False(t, NeedsMigration(nil))
}

// --- ListTransformations and registration ---

// TestListTransformations verifies it returns a copy with all entries.
func TestListTransformations(t *testing.T) {
	list := ListTransformations()
	require.Len(t, list, len(transformations))

	// Verify it's a copy (modifying doesn't affect original)
	list[0].Name = "modified"
	assert.NotEqual(t, "modified", transformations[0].Name)
}

// TestMustRegisterTransformationRefusesDuplicate: a second registration under
// one name is a Ze defect and panics.
func TestMustRegisterTransformationRefusesDuplicate(t *testing.T) {
	register(t, renameLeaf("test-rename", "old-leaf", "new-leaf"))

	assert.PanicsWithValue(t, "BUG: migration: transformation test-rename registered twice", func() {
		MustRegisterTransformation(renameLeaf("test-rename", "a", "b"))
	})
}

// TestMustRegisterTransformationRefusesShadowingBuiltin: a registration under
// a built-in step's name panics.
func TestMustRegisterTransformationRefusesShadowingBuiltin(t *testing.T) {
	assert.PanicsWithValue(t, "BUG: migration: transformation "+relocatedBlocksName+" shadows a built-in one", func() {
		MustRegisterTransformation(renameLeaf(relocatedBlocksName, "a", "b"))
	})
	assert.Equal(t, []string{relocatedBlocksName}, transformationNames())
}

// TestRegisterTransformationForTestRestores: the restore function answered by
// RegisterTransformationForTest removes the registration, so no later test
// runs it.
func TestRegisterTransformationForTestRestores(t *testing.T) {
	restore := RegisterTransformationForTest(renameLeaf("test-rename", "old-leaf", "new-leaf"))
	assert.Contains(t, transformationNames(), "test-rename")
	restore()
	assert.Equal(t, []string{relocatedBlocksName}, transformationNames())
}
