package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// eth8Unit is the unit 0 of the entry the round-6 tests rename eth9 to.
var eth8Unit = []string{"interface", "ethernet", "eth8", "unit", "0"}

// eth8AddressPath is the bracket address leaf-list under eth8Unit, as text.
const eth8AddressPath = "interface ethernet eth8 unit 0 ipv4 address"

// commitRound6 commits the session on the direct path or the candidate path
// and answers the result with the content the commit wrote.
func commitRound6(t *testing.T, ed *Editor, configPath string, candidate bool) (*CommitResult, string) {
	t.Helper()
	if candidate {
		res, content, err := ed.CommitSessionCandidate(time.Now())
		require.NoError(t, err)
		return res, content
	}
	res, err := ed.CommitSession()
	require.NoError(t, err)
	data, err := ed.store.ReadFile(configPath)
	require.NoError(t, err)
	return res, string(data)
}

// followFix runs the discard command a conflict's fix line names.
func followFix(t *testing.T, ed *Editor, fix string) {
	t.Helper()
	cmd, _, found := strings.Cut(fix, " drops")
	require.True(t, found, "fix line %q names no discard", fix)
	path, ok := strings.CutPrefix(cmd, "discard ")
	require.True(t, ok, "fix line %q is not a discard", fix)
	require.NoError(t, ed.DiscardSessionPath(strings.Fields(path)))
}

// TestRenameThenEditCommitsClean proves that a leaf set or deleted below an
// entry the session renamed commits with no conflict when nobody else
// committed, on both commit paths.
// VALIDATES: round-6 BLOCKER 1; the entry's Previous is read at the path the
// renames started from, where the commit check reads the committed value.
// PREVENTS: "STALE ... eth8 unit 0 description: you=moved, committed=lab"
// with nobody else committing.
func TestRenameThenEditCommitsClean(t *testing.T) {
	cases := []struct {
		name    string
		edit    func(ed *Editor) error
		holds   string
		without string
	}{
		{"set description", func(ed *Editor) error { return ed.SetValue(eth8Unit, "description", "moved") }, "moved", "description lab"},
		{"set address list", func(ed *Editor) error {
			return ed.SetValue(append(append([]string{}, eth8Unit...), "ipv4"), "address", "192.0.2.3/30")
		}, "192.0.2.3/30", "192.0.2.1/24"},
		{"delete description", func(ed *Editor) error { return ed.DeleteValue(eth8Unit, "description") }, "eth8", "description lab"},
	}
	for _, tc := range cases {
		for _, candidate := range []bool{false, true} {
			ed, _, configPath := newLoadSessionEditor(t, validBGPConfig+bracketIfaceWithInactive("192.0.2.1/24", ""))
			require.NoError(t, ed.RenameListEntry([]string{"interface"}, "ethernet", "eth9", "eth8"))
			require.NoError(t, tc.edit(ed))
			res, content := commitRound6(t, ed, configPath, candidate)
			require.Empty(t, res.Conflicts, "%s candidate=%v: %s", tc.name, candidate, commitConflictsText(res.Conflicts))
			assert.Contains(t, content, tc.holds, "%s candidate=%v", tc.name, candidate)
			assert.NotContains(t, content, tc.without, "%s candidate=%v", tc.name, candidate)
			assert.NotContains(t, content, "eth9", "%s candidate=%v", tc.name, candidate)
		}
	}
}

// TestRenameThenMemberConflictFixKeepsRename proves that a member conflict
// after a rename names the renamed path, and that following its fix line
// drops the one member change and keeps the rename and the session's other
// edits, on both commit paths.
// VALIDATES: round-6 BLOCKER 2 and ISSUE 4; a discard drops an op only when
// the op's own path lies at or under the discarded path.
// PREVENTS: the fix dropping the rename, leaving eth9 committed and a false
// STALE on eth8's description.
func TestRenameThenMemberConflictFixKeepsRename(t *testing.T) {
	cases := []struct {
		name  string
		edit  func(ed *Editor) error
		other string
	}{
		{"deactivate", func(ed *Editor) error {
			_, err := ed.ApplyActivation(addressMember("192.0.2.2/30"), false)
			return err
		}, "member removed"},
		{"insert", func(ed *Editor) error {
			return ed.InsertLeafListValue(ifaceIPv4Path, "address", "192.0.2.4/30", "before", "192.0.2.2/30")
		}, "insert reference removed"},
	}
	for _, tc := range cases {
		for _, candidate := range []bool{false, true} {
			ed, _, configPath := newLoadSessionEditor(t, validBGPConfig+bracketIfaceWithInactive("192.0.2.1/24 192.0.2.2/30", ""))
			require.NoError(t, tc.edit(ed))
			require.NoError(t, ed.SetValue([]string{"interface", "ethernet", "eth9", "unit", "0"}, "description", "renamed"))
			require.NoError(t, ed.RenameListEntry([]string{"interface"}, "ethernet", "eth9", "eth8"))
			require.NoError(t, ed.store.WriteFile(configPath, []byte(validBGPConfig+bracketIfaceWithInactive("192.0.2.1/24", "")), 0o600))

			res, _ := commitRound6(t, ed, configPath, candidate)
			require.Len(t, res.Conflicts, 1, "%s candidate=%v: %s", tc.name, candidate, commitConflictsText(res.Conflicts))
			c := res.Conflicts[0]
			assert.Equal(t, eth8AddressPath, c.Path, "%s candidate=%v", tc.name, candidate)
			assert.Equal(t, tc.other, c.OtherValue, "%s candidate=%v", tc.name, candidate)
			assert.True(t, strings.HasPrefix(c.Fix, "discard "+eth8AddressPath+" "), "%s candidate=%v fix=%q", tc.name, candidate, c.Fix)

			followFix(t, ed, c.Fix)
			res, content := commitRound6(t, ed, configPath, candidate)
			require.Empty(t, res.Conflicts, "%s candidate=%v: %s", tc.name, candidate, commitConflictsText(res.Conflicts))
			assert.Contains(t, content, "eth8", "%s candidate=%v: the rename must survive the fix", tc.name, candidate)
			assert.NotContains(t, content, "eth9", "%s candidate=%v", tc.name, candidate)
			assert.Contains(t, content, "renamed", "%s candidate=%v: the description must survive the fix", tc.name, candidate)
		}
	}
}

// TestInsertConflictNamesInsertOnce proves that an insert refused for its
// reference reports the insert as typed, and that a toggle of the inserted
// member reports no second conflict, since the insert's fix drops both.
// VALIDATES: round-6 ISSUE 1 and NOTE 2.
// PREVENTS: "you=set <path> <member>" on a bracket leaf, which reads as
// replacing the list, and a "member removed" conflict whose fix says to add
// back a member the commit never held.
func TestInsertConflictNamesInsertOnce(t *testing.T) {
	path := strings.Join(bracketAddressPath, " ")
	for _, candidate := range []bool{false, true} {
		ed, _, configPath := newLoadSessionEditor(t, validBGPConfig+bracketIfaceWithInactive("192.0.2.1/24 192.0.2.2/30", ""))
		require.NoError(t, ed.InsertLeafListValue(ifaceIPv4Path, "address", "192.0.2.4/30", "before", "192.0.2.2/30"))
		_, err := ed.ApplyActivation(addressMember("192.0.2.4/30"), false)
		require.NoError(t, err)
		require.NoError(t, ed.store.WriteFile(configPath, []byte(validBGPConfig+bracketIfaceWithInactive("192.0.2.1/24", "")), 0o600))

		res, _ := commitRound6(t, ed, configPath, candidate)
		require.Len(t, res.Conflicts, 1, "candidate=%v: %s", candidate, commitConflictsText(res.Conflicts))
		c := res.Conflicts[0]
		assert.Equal(t, "insert "+path+" 192.0.2.4/30 before 192.0.2.2/30", c.MyValue, "candidate=%v", candidate)
		text := commitConflictsText(res.Conflicts)
		assert.Contains(t, text, "you=insert "+path+" 192.0.2.4/30 before 192.0.2.2/30, committed=insert reference removed (was 192.0.2.2/30)\n")
		assert.Contains(t, text, "    fix: discard "+path+" 192.0.2.4/30 drops the insert and your other changes to that member")

		followFix(t, ed, c.Fix)
		res, content := commitRound6(t, ed, configPath, candidate)
		require.Empty(t, res.Conflicts, "candidate=%v: %s", candidate, commitConflictsText(res.Conflicts))
		assert.NotContains(t, content, "192.0.2.4/30", "candidate=%v", candidate)
	}
}

// TestPathRemovedConflictTextHasNoEmptyWas proves that a conflict with no
// previous value prints no "(was )".
// VALIDATES: round-6 NOTE 1.
// PREVENTS: "committed=path removed (was )".
func TestPathRemovedConflictTextHasNoEmptyWas(t *testing.T) {
	ed, _, configPath := newLoadSessionEditor(t, validBGPConfig+bracketIfaceWithInactive("192.0.2.1/24 192.0.2.2/30", ""))
	_, err := ed.ApplyActivation(addressMember("192.0.2.2/30"), false)
	require.NoError(t, err)
	require.NoError(t, ed.store.WriteFile(configPath, []byte(validBGPConfig), 0o600))
	res, err := ed.CommitSession()
	require.NoError(t, err)
	require.Len(t, res.Conflicts, 1)
	text := commitConflictsText(res.Conflicts)
	assert.Contains(t, text, "committed=path removed\n")
	assert.NotContains(t, text, "(was )")
}

// TestSSHCommitSurfacesPrintFixLine proves that the SSH `commit` and
// `commit confirmed` print a member conflict's fix line, driven from the
// model's command handlers rather than the text helper.
// VALIDATES: round-6 ISSUE 2, the SSH surfaces.
// PREVENTS: a surface rendering conflicts without the command that resolves
// them.
func TestSSHCommitSurfacesPrintFixLine(t *testing.T) {
	path := strings.Join(bracketAddressPath, " ")
	// The model validates the whole config before it commits, and an
	// interface section needs its backend for that.
	withBackend := func(addrs string) string {
		return validBGPConfig + strings.Replace(bracketIfaceWithInactive(addrs, ""), "interface {\n", "interface {\n\tbackend netlink;\n", 1)
	}
	for _, confirmed := range []bool{false, true} {
		ed, _, configPath := newLoadSessionEditor(t, withBackend("192.0.2.1/24 192.0.2.2/30"))
		_, err := ed.ApplyActivation(addressMember("192.0.2.2/30"), false)
		require.NoError(t, err)
		require.NoError(t, ed.store.WriteFile(configPath, []byte(withBackend("192.0.2.1/24")), 0o600))
		model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
		require.NoError(t, err)
		var result commandResult
		if confirmed {
			result, err = model.cmdCommitConfirmed(60, false)
		} else {
			result, err = model.cmdCommitSession(false)
		}
		require.NoError(t, err, "confirmed=%v", confirmed)
		assert.Contains(t, result.output, "    fix: discard "+path+" 192.0.2.2/30 drops the toggle", "confirmed=%v", confirmed)
	}
}
