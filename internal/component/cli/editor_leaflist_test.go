package cli

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// leafListSeedConfig seeds a valid config with an existing leaf-list so
// delete/idempotency tests start from committed members.
const leafListSeedConfig = validBGPConfig + `
system {
	name-server [ 9.9.9.9 ]
}
`

// committedNameServers parses the committed config file and returns the
// name-server members from the multi-value store (the store every
// serializer reads). Reading through a fresh parse proves the change
// survives a restart.
func committedNameServers(t *testing.T, configPath string) []string {
	t.Helper()
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree, _, err := config.NewSetParser(schema).ParseWithMeta(stripSchemaStamp(string(data)))
	require.NoError(t, err)
	system := tree.GetContainer("system")
	require.NotNil(t, system, "committed config should contain the system container")
	return system.GetSlice("name-server")
}

// committedNameServerState re-reads the committed config and returns the full
// ordered member view (including deactivated members, tagged), for tests that
// must observe per-member deactivation surviving commit.
func committedNameServerState(t *testing.T, configPath string) []config.MemberState {
	t.Helper()
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree, _, err := config.NewSetParser(schema).ParseWithMeta(stripSchemaStamp(string(data)))
	require.NoError(t, err)
	system := tree.GetContainer("system")
	require.NotNil(t, system, "committed config should contain the system container")
	return system.GetMultiValuesState("name-server")
}

// stripSchemaStamp drops the leading "# ze-schema:" comment line so the
// set parser sees only set/delete lines.
func stripSchemaStamp(content string) string {
	if strings.HasPrefix(content, "# ze-schema:") {
		if _, rest, found := strings.Cut(content, "\n"); found {
			return rest
		}
	}
	return content
}

// newLeafListSessionEditor creates a filesystem-backed editor with an active session.
func newLeafListSessionEditor(t *testing.T, seed string) (*Editor, string) {
	t.Helper()
	configPath := writeTestConfig(t, seed)
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ed.Close() })
	ed.SetSession(NewEditSession("thomas", "local"))
	return ed, configPath
}

// TestSessionLeafListSetCommits is the AC-1 regression for Bug B: a session
// `set system name-server 8.8.8.8` used to report "0 change(s) applied"
// because writeThroughSet stored the value in the scalar map while every
// leaf-list serializer reads the multi-value map.
//
// VALIDATES: AC-1 "Session committed: 1 change(s) applied; change file
// non-empty; committed config contains name-server 8.8.8.8 in the
// multi-value store; survives restart".
// PREVENTS: leaf-list session edits silently dropped at serialize/commit.
func TestSessionLeafListSetCommits(t *testing.T) {
	ed, configPath := newLeafListSessionEditor(t, validBGPConfig)

	require.NoError(t, ed.SetValue([]string{"system"}, "name-server", "8.8.8.8"))

	changeData, err := os.ReadFile(ChangePath(configPath, "thomas"))
	require.NoError(t, err, "change file should exist after write-through")
	assert.Contains(t, string(changeData), "8.8.8.8",
		"change file must contain the leaf-list member (was empty before the fix)")

	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)
	assert.Equal(t, 1, result.Applied, "leaf-list set must count as an applied change")

	members := committedNameServers(t, configPath)
	assert.Equal(t, []string{"8.8.8.8"}, members,
		"committed config must hold the member in the multi-value store")
}

// TestSessionLeafListAddMember verifies JunOS-style add-member semantics.
//
// VALIDATES: AC-2 "BOTH members present after commit (add-member, not replace)".
// PREVENTS: second `set` replacing the whole leaf-list.
func TestSessionLeafListAddMember(t *testing.T) {
	ed, configPath := newLeafListSessionEditor(t, validBGPConfig)

	require.NoError(t, ed.SetValue([]string{"system"}, "name-server", "8.8.8.8"))
	require.NoError(t, ed.SetValue([]string{"system"}, "name-server", "1.1.1.1"))

	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)
	assert.Equal(t, 2, result.Applied, "each added member counts as one change")

	members := committedNameServers(t, configPath)
	assert.ElementsMatch(t, []string{"8.8.8.8", "1.1.1.1"}, members,
		"both members must survive commit (add-member, not replace)")
}

// TestSessionLeafListAddMemberPreservesExisting: adding a member must not
// drop members already committed before the session started.
//
// VALIDATES: AC-2 add-member against a non-empty committed leaf-list.
// PREVENTS: commit replacing the committed list with only the session's members.
func TestSessionLeafListAddMemberPreservesExisting(t *testing.T) {
	ed, configPath := newLeafListSessionEditor(t, leafListSeedConfig)

	require.NoError(t, ed.SetValue([]string{"system"}, "name-server", "8.8.8.8"))

	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)

	members := committedNameServers(t, configPath)
	assert.ElementsMatch(t, []string{"9.9.9.9", "8.8.8.8"}, members,
		"committed member must be preserved alongside the new one")
}

// TestSessionLeafListSetIdempotent: setting the same member twice must not
// duplicate it (matches non-session insert semantics, which reject dups).
//
// VALIDATES: AC-3 "Idempotent: member not duplicated".
// PREVENTS: unbounded duplicate growth from repeated set commands.
func TestSessionLeafListSetIdempotent(t *testing.T) {
	ed, configPath := newLeafListSessionEditor(t, validBGPConfig)

	require.NoError(t, ed.SetValue([]string{"system"}, "name-server", "8.8.8.8"))
	require.NoError(t, ed.SetValue([]string{"system"}, "name-server", "8.8.8.8"))

	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)

	members := committedNameServers(t, configPath)
	assert.Equal(t, []string{"8.8.8.8"}, members, "duplicate set must not duplicate the member")
}

// TestSessionLeafListDeleteMember: deleting one member removes only that
// member.
//
// VALIDATES: AC-4 "Only that member removed; the other remains".
// PREVENTS: member delete wiping the whole leaf-list.
func TestSessionLeafListDeleteMember(t *testing.T) {
	seed := validBGPConfig + `
system {
	name-server [ 9.9.9.9 8.8.8.8 ]
}
`
	ed, configPath := newLeafListSessionEditor(t, seed)

	require.NoError(t, ed.DeleteLeafListValue([]string{"system"}, "name-server", "8.8.8.8"))

	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)

	members := committedNameServers(t, configPath)
	assert.Equal(t, []string{"9.9.9.9"}, members,
		"only the deleted member is removed; the other remains")
}

// TestScalarLeafStillCommits is the AC-6 control case: scalar leaves keep
// working through the same session commit path.
//
// VALIDATES: AC-6 "set bgp router-id 9.9.9.9, commit still Applied=1".
// PREVENTS: leaf-list support regressing scalar write-through/commit.
func TestScalarLeafStillCommits(t *testing.T) {
	ed, configPath := newLeafListSessionEditor(t, validBGPConfig)

	require.NoError(t, ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9"))

	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)
	assert.Equal(t, 1, result.Applied)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "router-id 9.9.9.9")
}

// TestLeafListConflictDetection: two sessions adding DIFFERENT members to
// the same leaf-list is not a conflict (members are independent); one
// session deleting the member another session adds IS a live conflict.
//
// VALIDATES: AC-7 "no false stale conflict for non-overlapping members;
// real conflict surfaced where it exists".
// PREVENTS: add-member sessions blocking each other spuriously.
func TestLeafListConflictDetection(t *testing.T) {
	configPath := writeTestConfig(t, leafListSeedConfig)

	edA, err := NewEditor(configPath)
	require.NoError(t, err)
	defer edA.Close() //nolint:errcheck // test cleanup
	edA.SetSession(NewEditSession("alice", "local"))

	edB, err := NewEditor(configPath)
	require.NoError(t, err)
	defer edB.Close() //nolint:errcheck // test cleanup
	edB.SetSession(NewEditSession("bob", "local"))

	// Non-overlapping members: no conflict in either direction.
	require.NoError(t, edA.SetValue([]string{"system"}, "name-server", "8.8.8.8"))
	require.NoError(t, edB.SetValue([]string{"system"}, "name-server", "1.1.1.1"))
	assert.Empty(t, edA.DetectConflicts(), "different members must not conflict")
	assert.Empty(t, edB.DetectConflicts(), "different members must not conflict")

	resultA, err := edA.CommitSession()
	require.NoError(t, err)
	assert.Empty(t, resultA.Conflicts, "no stale conflict for non-overlapping members")

	resultB, err := edB.CommitSession()
	require.NoError(t, err)
	assert.Empty(t, resultB.Conflicts, "no stale conflict for non-overlapping members")

	members := committedNameServers(t, configPath)
	assert.ElementsMatch(t, []string{"9.9.9.9", "8.8.8.8", "1.1.1.1"}, members,
		"both sessions' members and the seed member must all be committed")

	// Same member, opposing intents on a committed member: live conflict.
	// Bob re-asserts the seed member while Alice deletes it.
	require.NoError(t, edB.SetValue([]string{"system"}, "name-server", "9.9.9.9"))
	require.NoError(t, edA.DeleteLeafListValue([]string{"system"}, "name-server", "9.9.9.9"))
	assert.NotEmpty(t, edA.DetectConflicts(), "set vs delete of the same member must conflict")
}

// TestDiscardPathPreservesOtherSessionMembers: a partial discard of one
// session's added member must remove only that member from the shared
// per-user change tree. Delete(leafName) wiped the whole leaf-list, so the
// other session's member survived only via the serializer's orphan-intent
// fallback, which emitted its set line twice (member path + writeDeleteMetaLines).
//
// VALIDATES: member-aware removal in DiscardSessionPath partial discard.
// PREVENTS: same-user sibling sessions' members degrading to duplicated
// orphan lines in the change file after a partial discard.
func TestDiscardPathPreservesOtherSessionMembers(t *testing.T) {
	configPath := writeTestConfig(t, leafListSeedConfig)

	edA, err := NewEditor(configPath)
	require.NoError(t, err)
	defer edA.Close() //nolint:errcheck // test cleanup
	edA.SetSession(NewEditSession("thomas", "local"))

	edB, err := NewEditor(configPath)
	require.NoError(t, err)
	defer edB.Close() //nolint:errcheck // test cleanup
	// Different origin: session IDs embed user@origin%start-second, so two
	// same-origin sessions created in the same second would collide.
	edB.SetSession(NewEditSession("thomas", "ssh"))

	require.NoError(t, edA.SetValue([]string{"system"}, "name-server", "8.8.8.8"))
	require.NoError(t, edB.SetValue([]string{"system"}, "name-server", "1.1.1.1"))

	require.NoError(t, edA.DiscardSessionPath([]string{"system", "name-server"}))

	changeData, err := os.ReadFile(ChangePath(configPath, "thomas"))
	require.NoError(t, err)
	content := string(changeData)
	assert.NotContains(t, content, "8.8.8.8",
		"discarded member must leave the change file")
	assert.Equal(t, 1, strings.Count(content, "set system name-server 1.1.1.1"),
		"sibling session's member must survive in the tree exactly once, not as a duplicated orphan line")

	result, err := edB.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)
	members := committedNameServers(t, configPath)
	assert.ElementsMatch(t, []string{"9.9.9.9", "1.1.1.1"}, members,
		"commit after sibling discard must apply only the surviving member")
}

// TestCommitSessionCandidateAppliesLeafList: the transactional (candidate)
// commit path applies leaf-list members too — this is the path SSH commits
// take once the reload notifier is wired.
//
// VALIDATES: leaf-list survives the CommitSessionCandidate apply loop.
// PREVENTS: fixing only the bare CommitSession path.
func TestCommitSessionCandidateAppliesLeafList(t *testing.T) {
	ed, _ := newLeafListSessionEditor(t, validBGPConfig)

	require.NoError(t, ed.SetValue([]string{"system"}, "name-server", "8.8.8.8"))

	result, content, err := ed.CommitSessionCandidate(time.Now())
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)
	assert.Equal(t, 1, result.Applied)
	assert.Contains(t, content, "name-server 8.8.8.8",
		"candidate content must contain the leaf-list member")
}

// TestSessionLeafListInsertDeactivate: insert/deactivate/activate work in
// session mode (previously refused with errInsertNotSupportedInSessionMode
// and friends) and persist through commit with exact member order.
//
// VALIDATES: AC-5 "insert/deactivate/activate on a leaf-list works in
// session mode; persists through commit".
// PREVENTS: session editors silently lacking ordered leaf-list operations.
func TestSessionLeafListInsertDeactivate(t *testing.T) {
	seed := validBGPConfig + `
system {
	name-server [ 9.9.9.9 8.8.8.8 ]
}
`
	ed, configPath := newLeafListSessionEditor(t, seed)

	require.NoError(t, ed.InsertLeafListValue([]string{"system"}, "name-server", "1.1.1.1", config.InsertFirst, ""),
		"insert must work in session mode")
	require.NoError(t, ed.DeactivateLeafListValue([]string{"system"}, "name-server", "8.8.8.8"),
		"deactivate must work in session mode")

	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)

	members := committedNameServers(t, configPath)
	assert.Equal(t, []string{"1.1.1.1", "9.9.9.9"}, members,
		"active members survive commit in order (deactivated member excluded from effective view)")
	assert.Equal(t, []config.MemberState{
		{Value: "1.1.1.1"}, {Value: "9.9.9.9"}, {Value: "8.8.8.8", Inactive: true},
	}, committedNameServerState(t, configPath),
		"insert position and per-member deactivation must survive commit exactly")

	require.NoError(t, ed.ActivateLeafListValue([]string{"system"}, "name-server", "8.8.8.8"),
		"activate must work in session mode")
	result, err = ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)

	members = committedNameServers(t, configPath)
	assert.Equal(t, []string{"1.1.1.1", "9.9.9.9", "8.8.8.8"}, members,
		"activation must restore the member in place")
}

// TestSessionLeafListPendingDiffMatchesCommit: what the pending view shows
// is exactly what commit applies — one pending change per member.
//
// VALIDATES: AC-11 "what the + diff shows is exactly what commit applies".
// PREVENTS: 'shows value but applies 0' mismatch between diff and commit.
func TestSessionLeafListPendingDiffMatchesCommit(t *testing.T) {
	ed, _ := newLeafListSessionEditor(t, validBGPConfig)

	require.NoError(t, ed.SetValue([]string{"system"}, "name-server", "8.8.8.8"))
	require.NoError(t, ed.SetValue([]string{"system"}, "name-server", "1.1.1.1"))

	pending := ed.PendingChanges(ed.session.ID)
	require.Len(t, pending, 2, "each member must appear as its own pending change")
	values := []string{pending[0].Value, pending[1].Value}
	assert.ElementsMatch(t, []string{"8.8.8.8", "1.1.1.1"}, values)

	result, err := ed.CommitSession()
	require.NoError(t, err)
	assert.Equal(t, len(pending), result.Applied,
		"commit must apply exactly the pending changes the diff showed")
}
