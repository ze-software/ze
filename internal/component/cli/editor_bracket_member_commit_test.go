package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bracketIfaceWithInactive is the ipv4 bracket leaf-list config of
// loadIfaceAddresses, with one `inactive: address <member>` statement when
// inactive is not empty.
func bracketIfaceWithInactive(addrs, inactive string) string {
	in := ""
	if inactive != "" {
		in = "\t\t\t\tinactive: address " + inactive + ";\n"
	}
	return "interface {\n\tethernet eth9 {\n\t\tunit 0 {\n\t\t\tdescription lab;\n\t\t\tipv4 {\n" +
		"\t\t\t\taddress [ " + addrs + " ];\n" + in + "\t\t\t}\n\t\t}\n\t}\n}\n"
}

// ifaceIPv4Path is the container that holds the bracket address leaf-list.
var ifaceIPv4Path = []string{"interface", "ethernet", "eth9", "unit", "0", "ipv4"}

// TestCommitLoadMergeAddsDeactivatedMember verifies that a load merge adding a
// member the input already deactivates commits it, deactivated.
//
// VALIDATES: committed [ .1 ]; load merge [ .1 .3 ] with .3 inactive;
// CommitSession: .1 active, .3 inactive in the committed file.
// PREVENTS: the member deactivation running against the committed tree before
// the entry that adds .3, which refused the commit with ".3 not found".
func TestCommitLoadMergeAddsDeactivatedMember(t *testing.T) {
	ed, _, configPath := newLoadSessionEditor(t, validBGPConfig+bracketIfaceWithInactive("192.0.2.1/24", ""))
	require.NoError(t, ed.LoadMerge(nil, parseLoadInput(t, ed, bracketIfaceWithInactive("192.0.2.1/24 192.0.2.3/30", "192.0.2.3/30"))))

	res, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, res.Conflicts)

	active, inactive := memberStates(loadIfaceIPv4(t, committedLoadTree(t, ed, configPath)))
	assert.Equal(t, []string{"192.0.2.1/24"}, active)
	assert.Equal(t, []string{"192.0.2.3/30"}, inactive)
}

// TestCommitSetThenDeactivateNewMember verifies the interactive route to the
// same shape: set the whole list with a new member, deactivate the member, then
// commit, both directly and through a saved draft.
//
// VALIDATES: set address [ .1 .3 ]; deactivate .3; SaveDraft keeps .3 inactive;
// CommitSession writes .1 active, .3 inactive.
// PREVENTS: the commit refusing the deactivation of a member only the session's
// set entry adds, and the draft replay dropping the deactivation silently.
func TestCommitSetThenDeactivateNewMember(t *testing.T) {
	ed, _, configPath := newLoadSessionEditor(t, validBGPConfig+bracketIfaceWithInactive("192.0.2.1/24", ""))
	require.NoError(t, ed.SetValue(ifaceIPv4Path, "address", "192.0.2.1/24 192.0.2.3/30"))
	_, err := ed.ApplyActivation(append(append([]string{}, bracketAddressPath...), "192.0.2.3/30"), false)
	require.NoError(t, err)

	require.NoError(t, ed.SaveDraft())
	_, inactive := memberStates(loadIfaceIPv4(t, ed.tree))
	assert.Equal(t, []string{"192.0.2.3/30"}, inactive, "the draft replay keeps the deactivation")

	res, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, res.Conflicts)

	active, inactive := memberStates(loadIfaceIPv4(t, committedLoadTree(t, ed, configPath)))
	assert.Equal(t, []string{"192.0.2.1/24"}, active)
	assert.Equal(t, []string{"192.0.2.3/30"}, inactive)
}

// TestCommitDeactivateThenDeleteMember verifies that a member the session
// deactivated and then deleted is gone after the commit.
//
// VALIDATES: committed [ .1 .2 ]; deactivate .2; delete address .2;
// CommitSession: .1 active, no .2 at all.
// PREVENTS: the deactivation being held until after the delete entry and then
// refusing the commit, or the delete leaving the member behind.
func TestCommitDeactivateThenDeleteMember(t *testing.T) {
	ed, _, configPath := newLoadSessionEditor(t, validBGPConfig+bracketIfaceWithInactive("192.0.2.1/24 192.0.2.2/30", ""))
	member := append(append([]string{}, bracketAddressPath...), "192.0.2.2/30")
	_, err := ed.ApplyActivation(member, false)
	require.NoError(t, err)
	require.NoError(t, ed.DeleteByPath(member))

	res, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, res.Conflicts)

	active, inactive := memberStates(loadIfaceIPv4(t, committedLoadTree(t, ed, configPath)))
	assert.Equal(t, []string{"192.0.2.1/24"}, active)
	assert.Empty(t, inactive)
}

// TestCommitReplaceDropsDeactivatedMember verifies that a load replace whose
// input no longer holds a deactivated bracket member removes that member alone,
// through a plain and a forced commit.
//
// VALIDATES: committed [ .1 .2 ] with .2 inactive; load replace [ .1 ];
// CommitSession and CommitSessionForce both write [ .1 ] with no member
// deactivated and the ipv4 container kept.
// PREVENTS: the change file's `delete ... address .2` reading back as a delete
// of the whole leaf, which raised a false stale conflict and, forced, deleted
// the list and its container.
func TestCommitReplaceDropsDeactivatedMember(t *testing.T) {
	for _, force := range []bool{false, true} {
		cfg := validBGPConfig + bracketIfaceWithInactive("192.0.2.1/24 192.0.2.2/30", "192.0.2.2/30")
		ed, _, configPath := newLoadSessionEditor(t, cfg)
		require.NoError(t, ed.LoadReplace(nil, parseLoadInput(t, ed, validBGPConfig+bracketIfaceWithInactive("192.0.2.1/24", ""))))

		commit := ed.CommitSession
		if force {
			commit = ed.CommitSessionForce
		}
		res, err := commit()
		require.NoError(t, err, "force=%v", force)
		require.Empty(t, res.Conflicts, "force=%v", force)

		active, inactive := memberStates(loadIfaceIPv4(t, committedLoadTree(t, ed, configPath)))
		assert.Equal(t, []string{"192.0.2.1/24"}, active, "force=%v", force)
		assert.Empty(t, inactive, "force=%v", force)
	}
}

// TestCommitDeleteActiveBracketMember verifies that `delete address <member>`
// removes one active member of a bracket leaf-list in a session.
//
// VALIDATES: committed [ .1 .2 ]; DeleteByPath address .2; CommitSession
// writes [ .1 ].
// PREVENTS: the typed delete refusing a bracket member, and the recorded member
// delete committing as a whole-leaf delete.
func TestCommitDeleteActiveBracketMember(t *testing.T) {
	ed, _, configPath := newLoadSessionEditor(t, validBGPConfig+bracketIfaceWithInactive("192.0.2.1/24 192.0.2.2/30", ""))
	require.NoError(t, ed.DeleteByPath(append(append([]string{}, bracketAddressPath...), "192.0.2.2/30")))

	res, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, res.Conflicts)

	active, inactive := memberStates(loadIfaceIPv4(t, committedLoadTree(t, ed, configPath)))
	assert.Equal(t, []string{"192.0.2.1/24"}, active)
	assert.Empty(t, inactive)
}

// TestDeleteDeactivatedBracketMemberFile verifies the documented way to remove
// a deactivated member in file mode: delete the member.
//
// VALIDATES: file [ .1 .2 ] with .2 inactive; DeleteByPath address .2; Save:
// [ .1 ] with nothing deactivated.
// PREVENTS: the delete answering "not found" for a bracket member, leaving the
// operator no verb that removes a deactivated member.
func TestDeleteDeactivatedBracketMemberFile(t *testing.T) {
	filePath := writeTestConfig(t, validBGPConfig+bracketIfaceWithInactive("192.0.2.1/24 192.0.2.2/30", "192.0.2.2/30"))
	ed, err := NewEditorWithStorage(newTestTreeStore(t, filePath), filePath)
	require.NoError(t, err)
	t.Cleanup(func() { ed.Close() }) //nolint:errcheck,gosec // Best effort cleanup

	require.NoError(t, ed.DeleteByPath(append(append([]string{}, bracketAddressPath...), "192.0.2.2/30")))
	_, err = ed.Save()
	require.NoError(t, err)

	active, inactive := memberStates(loadIfaceIPv4(t, committedLoadTree(t, ed, filePath)))
	assert.Equal(t, []string{"192.0.2.1/24"}, active)
	assert.Empty(t, inactive)
}

// TestLoadIdenticalBracketListRecordsNothing verifies that loading the
// configuration the session already holds, a deactivated member included,
// records no change, by merge and by replace.
//
// VALIDATES: committed [ .1 .2 ] with .2 inactive; load merge and load replace
// of the same text leave the session with no pending change.
// PREVENTS: replace comparing the loaded members, the deactivated one
// included, with the active-only mirror and recording a false `set address`.
func TestLoadIdenticalBracketListRecordsNothing(t *testing.T) {
	cfg := validBGPConfig + bracketIfaceWithInactive("192.0.2.1/24 192.0.2.2/30", "192.0.2.2/30")
	for _, replace := range []bool{false, true} {
		ed, session, _ := newLoadSessionEditor(t, cfg)
		load := ed.LoadMerge
		if replace {
			load = ed.LoadReplace
		}
		require.NoError(t, load(nil, parseLoadInput(t, ed, cfg)))
		assert.Empty(t, pendingKinds(ed, session.ID), "replace=%v", replace)
	}
}
