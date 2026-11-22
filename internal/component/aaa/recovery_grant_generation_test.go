package aaa

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recoveryGrant is the break-glass grant seen through both authorization
// methods, which must answer alike.
type recoveryGrant interface {
	Authorizer
	CommandArgsAuthorizer
}

// recoveryGrantForTest returns the break-glass grant a local login of a user
// stamped at generation holds, typed so both authorization methods are checked.
func recoveryGrantForTest(t *testing.T, generation uint64) recoveryGrant {
	t.Helper()
	grant := AuthorizerForResult(nil, AuthResult{
		Authenticated:  true,
		Source:         SourceLocal,
		Profiles:       []string{ReservedRecoveryProfile},
		UserGeneration: generation,
	})
	typed, ok := grant.(recoveryGrant)
	require.True(t, ok, "the recovery grant must answer typed commands too")
	return typed
}

// assertGrantHolds checks both authorization methods, because the two must
// answer alike for one grant.
func assertGrantHolds(t *testing.T, grant recoveryGrant, username string, want bool, why string) {
	t.Helper()
	assert.Equal(t, want, grant.Authorize(username, "", "config commit", false), why)
	assert.Equal(t, want, grant.AuthorizeCommandArgs(username, "", "config", []string{"commit"}, "", false), why)
}

// VALIDATES: a break-glass recovery grant stays valid while ANOTHER local user
// is changed, added or removed, because it is checked against its own user's
// generation only.
// PREVENTS: the grant being revoked by the whole-set counter it used to read,
// which logged the recovery admin out whenever any other user's password,
// profiles or keys changed, or a user was added or removed.
func TestRecoveryGrantSurvivesAnotherUsersChange(t *testing.T) {
	t.Cleanup(func() { SetAcceptedLocalUsers(nil) })
	SetAcceptedLocalUsers([]UserCredential{
		{Name: "admin", UserGeneration: 3},
		{Name: "alice", UserGeneration: 4},
	})
	grant := recoveryGrantForTest(t, 3)
	assertGrantHolds(t, grant, "admin", true, "the grant is valid at its own generation")

	SetAcceptedLocalUsers([]UserCredential{
		{Name: "admin", UserGeneration: 3},
		{Name: "alice", UserGeneration: 9},
	})
	assertGrantHolds(t, grant, "admin", true, "another user's changed credentials must not revoke the grant")

	SetAcceptedLocalUsers([]UserCredential{
		{Name: "admin", UserGeneration: 3},
		{Name: "alice", UserGeneration: 9},
		{Name: "bob", UserGeneration: 10},
	})
	assertGrantHolds(t, grant, "admin", true, "adding a user must not revoke the grant")

	SetAcceptedLocalUsers([]UserCredential{{Name: "admin", UserGeneration: 3}})
	assertGrantHolds(t, grant, "admin", true, "removing another user must not revoke the grant")
}

// VALIDATES: a change to the grant user's own record, its removal, or the
// retirement of the accepted identity revokes the grant.
// PREVENTS: a per-user check that never revokes, which would make the grant
// permanent.
func TestRecoveryGrantRevokedByItsOwnUsersChange(t *testing.T) {
	t.Cleanup(func() { SetAcceptedLocalUsers(nil) })
	accepted := []UserCredential{
		{Name: "admin", UserGeneration: 3},
		{Name: "alice", UserGeneration: 4},
	}

	SetAcceptedLocalUsers(accepted)
	grant := recoveryGrantForTest(t, 3)
	assertGrantHolds(t, grant, "admin", true, "the grant is valid at its own generation")
	SetAcceptedLocalUsers([]UserCredential{
		{Name: "admin", UserGeneration: 11},
		{Name: "alice", UserGeneration: 4},
	})
	assertGrantHolds(t, grant, "admin", false, "a new generation for the grant's own user must revoke it")

	SetAcceptedLocalUsers(accepted)
	assertGrantHolds(t, grant, "admin", true, "the grant is valid at its own generation")
	SetAcceptedLocalUsers([]UserCredential{{Name: "alice", UserGeneration: 4}})
	assertGrantHolds(t, grant, "admin", false, "removing the grant's own user must revoke it")

	SetAcceptedLocalUsers(accepted)
	assertGrantHolds(t, grant, "admin", true, "the grant is valid at its own generation")
	SetAcceptedLocalUsers(nil)
	assertGrantHolds(t, grant, "admin", false, "a retired identity must revoke every grant")
}

// VALIDATES: the grant is checked against the user who presents it, and a
// generation no local record stamped grants nothing.
// PREVENTS: a grant presented under another username matching that user's
// entry, and an unstamped zero generation matching an unstamped record.
func TestRecoveryGrantRefusesAnotherUserAndZeroGeneration(t *testing.T) {
	t.Cleanup(func() { SetAcceptedLocalUsers(nil) })
	SetAcceptedLocalUsers([]UserCredential{
		{Name: "admin", UserGeneration: 3},
		{Name: "alice", UserGeneration: 4},
		{Name: "unstamped"},
	})

	grant := recoveryGrantForTest(t, 3)
	assertGrantHolds(t, grant, "admin", true, "the grant is valid for its own user")
	assertGrantHolds(t, grant, "alice", false, "the grant must not hold for another user")

	unstamped := recoveryGrantForTest(t, 0)
	assertGrantHolds(t, unstamped, "unstamped", false, "generation 0 is no grant, even against an unstamped record")
}
