package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// bracketAddressPath is the ipv4 address bracket leaf-list loadIfaceAddresses
// writes, with the member to deactivate appended by each test.
var bracketAddressPath = []string{"interface", "ethernet", "eth9", "unit", "0", "ipv4", "address"}

// memberStates answers the address leaf-list's members split by deactivation,
// so a test can compare them without depending on member order.
func memberStates(tree *config.Tree) (active, inactive []string) {
	for _, state := range tree.GetMultiValuesState("address") {
		if state.Inactive {
			inactive = append(inactive, state.Value)
			continue
		}
		active = append(active, state.Value)
	}
	return active, inactive
}

// TestDeactivateBracketMemberSurvivesSave verifies that deactivating one member
// of a bracket leaf-list in file mode keeps it, deactivated, through a save
// and a fresh load of the file.
//
// VALIDATES: address [ .1/24 .2/30 ], deactivate .2/30, Save, reopen: .1/24
// active, .2/30 inactive.
// PREVENTS: the save writing the scalar mirror, which names active members
// only, so the deactivated member was deleted from the file.
func TestDeactivateBracketMemberSurvivesSave(t *testing.T) {
	filePath := writeTestConfig(t, validBGPConfig+loadIfaceAddresses("192.0.2.1/24 192.0.2.2/30"))
	store := newTestTreeStore(t, filePath)
	ed, err := NewEditorWithStorage(store, filePath)
	require.NoError(t, err)
	t.Cleanup(func() { ed.Close() }) //nolint:errcheck,gosec // Best effort cleanup

	_, err = ed.ApplyActivation(append(append([]string{}, bracketAddressPath...), "192.0.2.2/30"), false)
	require.NoError(t, err)
	_, err = ed.Save()
	require.NoError(t, err)

	active, inactive := memberStates(loadIfaceIPv4(t, committedLoadTree(t, ed, filePath)))
	assert.Equal(t, []string{"192.0.2.1/24"}, active)
	assert.Equal(t, []string{"192.0.2.2/30"}, inactive)
}

// TestDeactivateBracketMemberSurvivesCommit verifies the same in session mode:
// the commit writes the member deactivated.
//
// VALIDATES: session deactivate .2/30, CommitSession: the committed file holds
// .1/24 active and .2/30 inactive.
// PREVENTS: the committed file losing the member, as the save did.
func TestDeactivateBracketMemberSurvivesCommit(t *testing.T) {
	ed, _, configPath := newLoadSessionEditor(t, validBGPConfig+loadIfaceAddresses("192.0.2.1/24 192.0.2.2/30"))
	_, err := ed.ApplyActivation(append(append([]string{}, bracketAddressPath...), "192.0.2.2/30"), false)
	require.NoError(t, err)
	_, err = ed.CommitSession()
	require.NoError(t, err)

	active, inactive := memberStates(loadIfaceIPv4(t, committedLoadTree(t, ed, configPath)))
	assert.Equal(t, []string{"192.0.2.1/24"}, active)
	assert.Equal(t, []string{"192.0.2.2/30"}, inactive)
}

// TestLoadMergeKeepsDeactivatedBracketMember verifies that a load merge into a
// bracket leaf-list keeps a member the operator deactivated, deactivated.
//
// VALIDATES: deactivate .2/30, merge [ .3/30 ]: .1/24 and .3/30 active, .2/30
// inactive.
// PREVENTS: the merge reading the active members only and writing the list
// whole, which deleted the deactivated member.
func TestLoadMergeKeepsDeactivatedBracketMember(t *testing.T) {
	filePath := writeTestConfig(t, validBGPConfig+loadIfaceAddresses("192.0.2.1/24 192.0.2.2/30"))
	ed, err := NewEditorWithStorage(newTestTreeStore(t, filePath), filePath)
	require.NoError(t, err)
	t.Cleanup(func() { ed.Close() }) //nolint:errcheck,gosec // Best effort cleanup

	_, err = ed.ApplyActivation(append(append([]string{}, bracketAddressPath...), "192.0.2.2/30"), false)
	require.NoError(t, err)
	require.NoError(t, ed.LoadMerge(nil, parseLoadInput(t, ed, loadIfaceAddresses("192.0.2.3/30"))))

	active, inactive := memberStates(loadIfaceIPv4(t, ed.tree))
	assert.ElementsMatch(t, []string{"192.0.2.1/24", "192.0.2.3/30"}, active)
	assert.Equal(t, []string{"192.0.2.2/30"}, inactive)
}

// TestSessionFirstBracketWriteKeepsMembersAfterCommit verifies that a bracket
// leaf-list first written in a session holds members in the editor's tree
// after the commit, not one joined value.
//
// VALIDATES: set ipv6 address "2001:db8::1/64 2001:db8::2/64" where none
// existed, CommitSession: GetSlice and ToMap answer the two members.
// PREVENTS: the commit replaying the change with Tree.Set on a name the
// committed tree never held, which stored the joined text as a scalar.
func TestSessionFirstBracketWriteKeepsMembersAfterCommit(t *testing.T) {
	ed, _, _ := newLoadSessionEditor(t, validBGPConfig+loadIfaceAddresses("192.0.2.1/24"))
	path := []string{"interface", "ethernet", "eth9", "unit", "0", "ipv6"}
	require.NoError(t, ed.SetValue(path, "address", "2001:db8::1/64 2001:db8::2/64"))
	_, err := ed.CommitSession()
	require.NoError(t, err)

	want := []string{"2001:db8::1/64", "2001:db8::2/64"}
	unit := ed.tree.GetContainer("interface").GetList("ethernet")["eth9"].GetList("unit")["0"]
	v6 := unit.GetContainer("ipv6")
	require.NotNil(t, v6)
	assert.Equal(t, want, v6.GetSlice("address"))
	assert.Equal(t, want, v6.ToMap()["address"])
}
