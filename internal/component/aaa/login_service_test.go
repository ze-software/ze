package aaa

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/loginservice"
)

func registerTestLoginServices(t *testing.T, names ...string) {
	t.Helper()
	snap := loginservice.Snapshot()
	t.Cleanup(func() { loginservice.Restore(snap) })
	loginservice.Restore(nil)

	for _, name := range names {
		require.NoError(t, loginservice.Register(loginservice.Entry{
			Name:        name,
			Owner:       "test/" + name,
			Description: "test service " + name,
		}))
	}
}

// TestLoginDecisionAbsentSetAdmits verifies that an account with no
// login-service set configured (LoginSet == "") is admitted on every service,
// including when no login-service sets generation has been published.
//
// VALIDATES: AC-7 (OD-3 option a) -- empty LoginSet reference admits every service.
// PREVENTS: Untouched configs failing after upgrade when login-service is omitted.
func TestLoginDecisionAbsentSetAdmits(t *testing.T) {
	registerTestLoginServices(t, "ssh", "web")
	t.Cleanup(func() { SetAcceptedLoginSets(nil) })

	SetAcceptedLoginSets(map[string][]string{
		"ssh-only": {"ssh"},
	})
	res := &AuthResult{Authenticated: true, Source: SourceLocal, LoginSet: ""}
	assert.NoError(t, CheckLoginService("alice", res, "ssh"))
	assert.NoError(t, CheckLoginService("alice", res, "web"))
	assert.NoError(t, CheckLoginSet("alice", "", "ssh"))
	assert.NoError(t, CheckLoginSet("alice", "", "web"))

	// Even when no login-sets generation is published, absent LoginSet admits.
	SetAcceptedLoginSets(nil)
	assert.NoError(t, CheckLoginService("alice", res, "ssh"))
	assert.NoError(t, CheckLoginSet("alice", "", "web"))
}

// TestLoginDecisionEmptySetRefuses verifies that referencing a defined
// login-service set with zero services refuses every gated door, staying
// distinct from an absent login-service reference.
//
// VALIDATES: AC-11 -- an empty set ([]string{}) refuses every door while an
// absent reference ("") admits every door.
// PREVENTS: Empty login-service sets collapsing into unrestricted access.
func TestLoginDecisionEmptySetRefuses(t *testing.T) {
	registerTestLoginServices(t, "ssh", "web")
	t.Cleanup(func() { SetAcceptedLoginSets(nil) })

	SetAcceptedLoginSets(map[string][]string{
		"locked": {},
	})

	restricted := &AuthResult{Authenticated: true, Source: SourceLocal, LoginSet: "locked"}
	err := CheckLoginService("alice", restricted, "ssh")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAuthRejected)

	err = CheckLoginSet("alice", "locked", "web")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAuthRejected)

	// Contrast with absent reference: admitted.
	unrestricted := &AuthResult{Authenticated: true, Source: SourceLocal, LoginSet: ""}
	assert.NoError(t, CheckLoginService("bob", unrestricted, "ssh"))
}

// TestLoginDecisionUnreadableRefusesAsRejection verifies that an uninitialized
// (nil) accepted login-sets generation or a missing set name fails closed with
// ErrAuthRejected.
//
// VALIDATES: AC-6 -- nil generation or missing set name returns ErrAuthRejected.
// PREVENTS: Read failures or missing sets degrading to allow-all or falling
// through the AAA chain as a non-terminal error.
func TestLoginDecisionUnreadableRefusesAsRejection(t *testing.T) {
	registerTestLoginServices(t, "ssh")
	t.Cleanup(func() { SetAcceptedLoginSets(nil) })

	res := &AuthResult{Authenticated: true, Source: SourceLocal, LoginSet: "ops"}

	t.Run("nil generation refuses with ErrAuthRejected", func(t *testing.T) {
		SetAcceptedLoginSets(nil)
		err := CheckLoginService("alice", res, "ssh")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrAuthRejected)

		err = CheckLoginSet("alice", "ops", "ssh")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrAuthRejected)
	})

	t.Run("missing set name refuses with ErrAuthRejected", func(t *testing.T) {
		SetAcceptedLoginSets(map[string][]string{
			"other": {"ssh"},
		})
		err := CheckLoginService("alice", res, "ssh")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrAuthRejected)

		err = CheckLoginSet("alice", "ops", "ssh")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrAuthRejected)
	})
}

// TestLoginDecisionEmptyOrUnregisteredServiceRefuses verifies that an empty or
// unregistered service name is refused for any account naming a login set,
// while a registered service in the set is admitted and one outside is refused.
//
// VALIDATES: Empty or unregistered service names fail closed with
// ErrAuthRejected even if the set textually contains the unregistered name.
// PREVENTS: Callers omitting AuthRequest.Service or naming an unregistered
// surface bypassing the login-service gate.
func TestLoginDecisionEmptyOrUnregisteredServiceRefuses(t *testing.T) {
	registerTestLoginServices(t, "ssh", "web")
	t.Cleanup(func() { SetAcceptedLoginSets(nil) })

	SetAcceptedLoginSets(map[string][]string{
		"ops": {"ssh", "ghost-door"},
	})

	res := &AuthResult{Authenticated: true, Source: SourceLocal, LoginSet: "ops"}

	// Empty service name is refused.
	err := CheckLoginService("alice", res, "")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAuthRejected)

	err = CheckLoginSet("alice", "ops", "")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAuthRejected)

	// Unregistered service name is refused even if present in the set slice.
	err = CheckLoginService("alice", res, "ghost-door")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAuthRejected)

	err = CheckLoginSet("alice", "ops", "ghost-door")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAuthRejected)

	// Registered service outside the set is refused.
	err = CheckLoginService("alice", res, "web")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAuthRejected)

	// Registered service inside the set is admitted.
	assert.NoError(t, CheckLoginService("alice", res, "ssh"))
	assert.NoError(t, CheckLoginSet("alice", "ops", "ssh"))
}

// TestLoginDecisionAdmitsRecoveryResult verifies that a local result carrying
// ReservedRecoveryProfile is admitted before reading any login set, while a
// remote result carrying that profile name is never exempt.
//
// VALIDATES: AC-8 (OD-2 option a) -- local ReservedRecoveryProfile is exempt
// even with an empty set or nil generation; remote spoofed recovery profile is
// still gated and refused.
// PREVENTS: Operators locking out the stored recovery account, or a remote
// backend bypassing login-service restrictions via ReservedRecoveryProfile.
func TestLoginDecisionAdmitsRecoveryResult(t *testing.T) {
	registerTestLoginServices(t, "ssh")
	t.Cleanup(func() { SetAcceptedLoginSets(nil) })

	localRecovery := &AuthResult{
		Authenticated: true,
		Source:        SourceLocal,
		Profiles:      []string{ReservedRecoveryProfile},
		LoginSet:      "locked",
	}

	// Admitted even with nil generation.
	SetAcceptedLoginSets(nil)
	assert.NoError(t, CheckLoginService("zeadmin", localRecovery, "ssh"))

	// Admitted even when its named set is empty.
	SetAcceptedLoginSets(map[string][]string{"locked": {}})
	assert.NoError(t, CheckLoginService("zeadmin", localRecovery, "ssh"))

	// A remote result carrying ReservedRecoveryProfile is NEVER exempt.
	remoteSpoof := &AuthResult{
		Authenticated: true,
		Source:        "tacacs",
		Profiles:      []string{ReservedRecoveryProfile},
		LoginSet:      "locked",
	}
	err := CheckLoginService("zeadmin", remoteSpoof, "ssh")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAuthRejected)

	// A nil result fails closed with ErrAuthRejected.
	err = CheckLoginService("zeadmin", nil, "ssh")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAuthRejected)
}
