// Design: docs/guide/operator-access-rbac.md -- the recovery account
// Related: aaa_lifecycle.go -- stampLocalIdentityGeneration, the per-user stamp the grant reads

package hub

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
)

// recoveryGrantUsers is an accepted local identity with the break-glass admin
// and one ordinary operator, each a distinct record.
func recoveryGrantUsers() []aaa.UserCredential {
	return []aaa.UserCredential{
		{
			Name:     "admin",
			Hash:     "$2a$10$admin",
			Profiles: []string{aaa.ReservedRecoveryProfile},
		},
		{
			Name:       "alice",
			Hash:       "$2a$10$alice",
			Profiles:   []string{"ops"},
			PublicKeys: []aaa.SSHPublicKey{{Name: "laptop", Type: "ssh-ed25519", Key: "AAAA"}},
		},
	}
}

// VALIDATES: a live break-glass recovery grant survives every change to
// ANOTHER local user published through the real accept path: that user's
// password, profiles or SSH keys changing, a user being added, and a user being
// removed.
// PREVENTS: the grant being checked against the whole-credential-set counter,
// which revoked the recovery admin's session whenever any other local user
// changed.
func TestRecoveryGrantSurvivesAnotherLocalUsersChange(t *testing.T) {
	resetAAABundleForTest(t)
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(recoveryGrantUsers(), nil, nil, ""))
	session := recoverySessionAuthorizer(t, "admin")
	require.True(t, session.Authorize("admin", "", "config commit", false))

	changes := []struct {
		what   string
		change func([]aaa.UserCredential) []aaa.UserCredential
	}{
		{"another user's password", func(users []aaa.UserCredential) []aaa.UserCredential {
			users[1].Hash = "$2a$10$alice-rotated"
			return users
		}},
		{"another user's profiles", func(users []aaa.UserCredential) []aaa.UserCredential {
			users[1].Profiles = []string{"read-only"}
			return users
		}},
		{"another user's SSH keys", func(users []aaa.UserCredential) []aaa.UserCredential {
			users[1].PublicKeys = nil
			return users
		}},
		{"a user added", func(users []aaa.UserCredential) []aaa.UserCredential {
			return append(users, aaa.UserCredential{Name: "bob", Hash: "$2a$10$bob"})
		}},
		{"a user removed", func(users []aaa.UserCredential) []aaa.UserCredential {
			return users[:1]
		}},
	}
	for _, tc := range changes {
		publishAcceptedLocalIdentity(newAcceptedLocalIdentity(tc.change(recoveryGrantUsers()), nil, nil, ""))
		assert.True(t, session.Authorize("admin", "", "config commit", false),
			"%s must not revoke the recovery admin's live grant", tc.what)
		typed, ok := session.(aaa.CommandArgsAuthorizer)
		require.True(t, ok)
		assert.True(t, typed.AuthorizeCommandArgs("admin", "", "config", []string{"commit"}, "", false),
			"%s must not revoke the recovery admin's live grant for a typed command", tc.what)
	}
}

// VALIDATES: a change to the grant user's OWN password, profiles or SSH keys,
// or that user's removal, published through the real accept path, revokes its
// live break-glass recovery grant.
// PREVENTS: the per-user check leaving a grant in force after the record that
// authenticated it is gone.
func TestRecoveryGrantRevokedByItsOwnUsersChange(t *testing.T) {
	changes := []struct {
		what   string
		change func([]aaa.UserCredential) []aaa.UserCredential
	}{
		{"its own password", func(users []aaa.UserCredential) []aaa.UserCredential {
			users[0].Hash = "$2a$10$admin-rotated"
			return users
		}},
		{"its own profiles", func(users []aaa.UserCredential) []aaa.UserCredential {
			users[0].Profiles = []string{aaa.ReservedRecoveryProfile, "ops"}
			return users
		}},
		{"its own SSH keys", func(users []aaa.UserCredential) []aaa.UserCredential {
			users[0].PublicKeys = []aaa.SSHPublicKey{{Name: "console", Type: "ssh-ed25519", Key: "CCCC"}}
			return users
		}},
		{"its removal", func(users []aaa.UserCredential) []aaa.UserCredential {
			return users[1:]
		}},
		{"its record repeated", func(users []aaa.UserCredential) []aaa.UserCredential {
			return append(users, users[0])
		}},
	}
	for _, tc := range changes {
		t.Run(tc.what, func(t *testing.T) {
			resetAAABundleForTest(t)
			publishAcceptedLocalIdentity(newAcceptedLocalIdentity(recoveryGrantUsers(), nil, nil, ""))
			session := recoverySessionAuthorizer(t, "admin")
			require.True(t, session.Authorize("admin", "", "config commit", false))

			publishAcceptedLocalIdentity(newAcceptedLocalIdentity(tc.change(recoveryGrantUsers()), nil, nil, ""))
			assert.False(t, session.Authorize("admin", "", "config commit", false),
				"a change to %s must revoke the recovery admin's live grant", tc.what)
			typed, ok := session.(aaa.CommandArgsAuthorizer)
			require.True(t, ok)
			assert.False(t, typed.AuthorizeCommandArgs("admin", "", "config", []string{"commit"}, "", false),
				"a change to %s must revoke the recovery admin's live grant for a typed command", tc.what)
		})
	}
}

// VALIDATES: retiring the accepted identity (daemon shutdown) revokes a live
// recovery grant.
// PREVENTS: a grant outliving the AAA state that issued it.
func TestRecoveryGrantRevokedWhenIdentityRetired(t *testing.T) {
	resetAAABundleForTest(t)
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(recoveryGrantUsers(), nil, nil, ""))
	session := recoverySessionAuthorizer(t, "admin")
	require.True(t, session.Authorize("admin", "", "config commit", false))

	closeAAABundle(nil)
	assert.False(t, session.Authorize("admin", "", "config commit", false),
		"closing the AAA state must revoke every recovery grant")
}
