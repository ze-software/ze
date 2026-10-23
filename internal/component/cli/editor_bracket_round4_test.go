package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// addressMember answers the path of one member of the bracket address list.
func addressMember(member string) []string {
	return append(append([]string{}, bracketAddressPath...), member)
}

// TestCommitAddDeactivateDeleteNewMember proves that a member the session adds,
// deactivates and then deletes commits as never added, directly and after a
// SaveDraft.
// VALIDATES: round-4 BLOCKER 1; the member delete drops the session's member
// ops naming the member.
// PREVENTS: the held deactivate running after the delete entry and failing
// the commit with `"192.0.2.3/30" not found in address`.
func TestCommitAddDeactivateDeleteNewMember(t *testing.T) {
	for _, draft := range []bool{false, true} {
		ed, _, configPath := newLoadSessionEditor(t, validBGPConfig+bracketIfaceWithInactive("192.0.2.1/24", ""))
		require.NoError(t, ed.SetValue(ifaceIPv4Path, "address", "192.0.2.1/24 192.0.2.3/30"))
		_, err := ed.ApplyActivation(addressMember("192.0.2.3/30"), false)
		require.NoError(t, err)
		require.NoError(t, ed.DeleteByPath(addressMember("192.0.2.3/30")))
		if draft {
			require.NoError(t, ed.SaveDraft())
		}
		res, err := ed.CommitSession()
		require.NoError(t, err, "draft=%v", draft)
		require.Empty(t, res.Conflicts, "draft=%v", draft)
		active, inactive := memberStates(loadIfaceIPv4(t, committedLoadTree(t, ed, configPath)))
		assert.Equal(t, []string{"192.0.2.1/24"}, active, "draft=%v", draft)
		assert.Empty(t, inactive, "draft=%v", draft)
	}
}

// TestWholeSetAfterMemberDeleteKeepsMember proves that a whole-leaf set after a
// member delete writes the list it names, in the session view, on SaveDraft and
// on CommitSession.
// VALIDATES: round-4 BLOCKER 2; a whole-leaf set replaces the session's member
// entries for the leaf.
// PREVENTS: the delete entry replaying after the list line and dropping
// 192.0.2.2/30 from the saved draft and the commit.
func TestWholeSetAfterMemberDeleteKeepsMember(t *testing.T) {
	want := []string{"192.0.2.1/24", "192.0.2.2/30", "192.0.2.5/30"}
	for _, draft := range []bool{false, true} {
		ed, _, configPath := newLoadSessionEditor(t, validBGPConfig+bracketIfaceWithInactive("192.0.2.1/24 192.0.2.2/30", ""))
		require.NoError(t, ed.DeleteByPath(addressMember("192.0.2.2/30")))
		require.NoError(t, ed.SetValue(ifaceIPv4Path, "address", strings.Join(want, " ")))
		active, _ := memberStates(loadIfaceIPv4(t, ed.tree))
		require.Equal(t, want, active, "session view")
		if draft {
			require.NoError(t, ed.SaveDraft())
			active, _ = memberStates(loadIfaceIPv4(t, ed.tree))
			assert.Equal(t, want, active, "after SaveDraft")
		}
		res, err := ed.CommitSession()
		require.NoError(t, err, "draft=%v", draft)
		require.Empty(t, res.Conflicts, "draft=%v", draft)
		active, inactive := memberStates(loadIfaceIPv4(t, committedLoadTree(t, ed, configPath)))
		assert.Equal(t, want, active, "draft=%v", draft)
		assert.Empty(t, inactive, "draft=%v", draft)
	}
}

// TestWholeSetAfterMemberOpsMatchesView proves that the commit writes what the
// session view shows when a whole-leaf set follows member ops on the leaf.
// VALIDATES: round-4 BLOCKER 2; a member op naming a member the set drops goes,
// a deactivated member the set keeps stays deactivated, and a deactivated
// member the session deleted and lists again commits active.
// PREVENTS: a held deactivate failing the commit, and the commit disagreeing
// with the view.
func TestWholeSetAfterMemberOpsMatchesView(t *testing.T) {
	cases := []struct {
		name      string
		committed string
		inactive  string
		edit      func(t *testing.T, ed *Editor)
		active    []string
		deact     []string
	}{
		{
			name:      "added, deactivated, then a set without it",
			committed: "192.0.2.1/24",
			edit: func(t *testing.T, ed *Editor) {
				require.NoError(t, ed.SetValue(ifaceIPv4Path, "address", "192.0.2.1/24 192.0.2.3/30"))
				_, err := ed.ApplyActivation(addressMember("192.0.2.3/30"), false)
				require.NoError(t, err)
				require.NoError(t, ed.SetValue(ifaceIPv4Path, "address", "192.0.2.1/24 192.0.2.4/30"))
			},
			active: []string{"192.0.2.1/24", "192.0.2.4/30"},
			deact:  []string{"192.0.2.3/30"},
		},
		{
			name:      "committed deactivated member deleted, then listed again",
			committed: "192.0.2.1/24 192.0.2.2/30",
			inactive:  "192.0.2.2/30",
			edit: func(t *testing.T, ed *Editor) {
				require.NoError(t, ed.DeleteByPath(addressMember("192.0.2.2/30")))
				require.NoError(t, ed.SetValue(ifaceIPv4Path, "address", "192.0.2.1/24 192.0.2.2/30"))
			},
			active: []string{"192.0.2.1/24", "192.0.2.2/30"},
		},
		{
			name:      "committed deactivated member deleted, then a set without it",
			committed: "192.0.2.1/24 192.0.2.2/30",
			inactive:  "192.0.2.2/30",
			edit: func(t *testing.T, ed *Editor) {
				require.NoError(t, ed.DeleteByPath(addressMember("192.0.2.2/30")))
				require.NoError(t, ed.SetValue(ifaceIPv4Path, "address", "192.0.2.1/24 192.0.2.5/30"))
			},
			active: []string{"192.0.2.1/24", "192.0.2.5/30"},
		},
		{
			name:      "activated, then a set without it",
			committed: "192.0.2.1/24 192.0.2.2/30",
			inactive:  "192.0.2.2/30",
			edit: func(t *testing.T, ed *Editor) {
				_, err := ed.ApplyActivation(addressMember("192.0.2.2/30"), true)
				require.NoError(t, err)
				require.NoError(t, ed.SetValue(ifaceIPv4Path, "address", "192.0.2.1/24"))
			},
			active: []string{"192.0.2.1/24"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ed, _, configPath := newLoadSessionEditor(t, validBGPConfig+bracketIfaceWithInactive(tc.committed, tc.inactive))
			tc.edit(t, ed)
			active, inactive := memberStates(loadIfaceIPv4(t, ed.tree))
			require.Equal(t, tc.active, active, "session view")
			require.Equal(t, tc.deact, inactive, "session view")
			res, err := ed.CommitSession()
			require.NoError(t, err)
			require.Empty(t, res.Conflicts)
			active, inactive = memberStates(loadIfaceIPv4(t, committedLoadTree(t, ed, configPath)))
			assert.Equal(t, tc.active, active, "committed")
			assert.Equal(t, tc.deact, inactive, "committed")
		})
	}
}

// newImportSessionEditor answers a session editor over insertConflictConfig,
// whose committed `bgp filter import filters` holds alpha and bravo.
func newImportSessionEditor(t *testing.T) (*Editor, string) {
	t.Helper()
	configPath := writeTestConfig(t, insertConflictConfig)
	ed, err := NewEditorWithStorage(newTestTreeStore(t, configPath), configPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ed.Close() })
	ed.SetSession(NewEditSession("thomas", "local"))
	return ed, configPath
}

// TestInsertAfterSessionAddedReference proves that a positional insert whose
// reference member the session itself added commits with no conflict, on both
// commit paths.
// VALIDATES: round-4 ISSUE 1; the insert pre-scan reads the committed tree and
// the tree after the session's entries.
// PREVENTS: a false "insert reference removed" stale conflict.
func TestInsertAfterSessionAddedReference(t *testing.T) {
	filter := []string{"bgp", "filter", "import"}
	edit := func(t *testing.T, ed *Editor) {
		require.NoError(t, ed.SetValue(filter, "filters", "charlie"))
		require.NoError(t, ed.InsertLeafListValue(filter, "filters", "delta", config.InsertAfter, "charlie"))
	}

	t.Run("CommitSession", func(t *testing.T) {
		ed, configPath := newImportSessionEditor(t)
		edit(t, ed)
		res, err := ed.CommitSession()
		require.NoError(t, err)
		require.Empty(t, res.Conflicts)
		got := committedLoadTree(t, ed, configPath).GetContainer("bgp").GetContainer("filter").GetContainer("import").GetMultiValues("filters")
		assert.Equal(t, []string{"alpha", "bravo", "charlie", "delta"}, got)
	})
	t.Run("CommitSessionCandidate", func(t *testing.T) {
		ed, _ := newImportSessionEditor(t)
		edit(t, ed)
		res, content, err := ed.CommitSessionCandidate(time.Now())
		require.NoError(t, err)
		require.Empty(t, res.Conflicts)
		tree, _, err := config.NewSetParser(ed.schema).ParseWithMeta(content)
		require.NoError(t, err)
		got := tree.GetContainer("bgp").GetContainer("filter").GetContainer("import").GetMultiValues("filters")
		assert.Equal(t, []string{"alpha", "bravo", "charlie", "delta"}, got)
	})
}

// TestDeactivateMemberRemovedConcurrently proves that a commit whose
// deactivated member a concurrent commit removed answers a stale conflict, on
// both commit paths.
// VALIDATES: round-4 NOTE 1; member deactivate and activate ops have a
// pre-scan. Round-5 ISSUE 3: the conflict names the toggle and its fix.
// PREVENTS: the commit failing with a raw `"192.0.2.2/30" not found` error.
func TestDeactivateMemberRemovedConcurrently(t *testing.T) {
	for _, candidate := range []bool{false, true} {
		ed, _, configPath := newLoadSessionEditor(t, validBGPConfig+bracketIfaceWithInactive("192.0.2.1/24 192.0.2.2/30", ""))
		_, err := ed.ApplyActivation(addressMember("192.0.2.2/30"), false)
		require.NoError(t, err)
		require.NoError(t, ed.store.WriteFile(configPath, []byte(validBGPConfig+bracketIfaceWithInactive("192.0.2.1/24", "")), 0o600))
		var res *CommitResult
		if candidate {
			res, _, err = ed.CommitSessionCandidate(time.Now())
		} else {
			res, err = ed.CommitSession()
		}
		require.NoError(t, err, "candidate=%v", candidate)
		require.Len(t, res.Conflicts, 1, "candidate=%v", candidate)
		c := res.Conflicts[0]
		assert.Equal(t, ConflictStale, c.Type)
		assert.Equal(t, strings.Join(bracketAddressPath, " "), c.Path)
		assert.Equal(t, "member removed", c.OtherValue)
		assert.Equal(t, "192.0.2.2/30", c.PreviousValue)
		// Round-5 ISSUE 3: the report names the toggle and the command
		// that resolves it, never "set <path> " and a re-set instruction.
		path := strings.Join(bracketAddressPath, " ")
		assert.Equal(t, "deactivate "+path+" 192.0.2.2/30", c.MyValue)
		text := commitConflictsText(res.Conflicts)
		assert.Contains(t, text, "  STALE "+path+": you=deactivate "+path+" 192.0.2.2/30, committed=member removed (was 192.0.2.2/30)\n")
		assert.Contains(t, text, "    fix: discard "+path+" 192.0.2.2/30 drops the toggle")
		assert.NotContains(t, text, "Re-set conflicting values")
		// The named fix resolves it: the discard drops the toggle alone.
		require.NoError(t, ed.DiscardSessionPath(strings.Fields(path+" 192.0.2.2/30")))
		if candidate {
			res, _, err = ed.CommitSessionCandidate(time.Now())
		} else {
			res, err = ed.CommitSession()
		}
		require.NoError(t, err, "candidate=%v", candidate)
		assert.Empty(t, res.Conflicts, "candidate=%v", candidate)
	}
}
