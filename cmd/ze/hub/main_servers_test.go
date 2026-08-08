package hub

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/ze-software/ze/internal/component/authz"
	zeconfig "github.com/ze-software/ze/internal/component/config"
)

// systemRootWith builds the `system` root exactly as the daemon holds it: the
// map form config.Tree.ToMap() produces and applyLoadedTreeToProvider writes
// into the shared ConfigProvider on every applied reload.
func systemRootWith(names ...string) map[string]any {
	users := make(map[string]any, len(names))
	for _, name := range names {
		users[name] = map[string]any{"password": "$2a$10$" + name, "profile": "admin"}
	}
	return map[string]any{"authentication": map[string]any{"user": users}}
}

func userNames(t *testing.T, cp *zeconfig.Provider) []string {
	t.Helper()
	users, err := liveConfigUsers(cp)
	require.NoError(t, err)
	names := make([]string, 0, len(users))
	for _, u := range users {
		names = append(names, u.Name)
	}
	return names
}

// VALIDATES: liveConfigUsers answers from the provider as it stands at the
// moment of the call, so a reload that removes a user removes them from the
// answer, and one that adds a user adds them.
// PREVENTS: authentication answering from the user list the daemon booted with.
func TestLiveConfigUsersFollowsTheProvider(t *testing.T) {
	cp := zeconfig.NewProvider()
	cp.SetRoot("system", systemRootWith("alice", "bob"))
	assert.Equal(t, []string{"alice", "bob"}, userNames(t, cp))

	// A reload that deletes bob.
	cp.SetRoot("system", systemRootWith("alice"))
	assert.Equal(t, []string{"alice"}, userNames(t, cp),
		"a user the reload removed must be gone from the next answer")

	// A reload that adds carol back alongside alice.
	cp.SetRoot("system", systemRootWith("alice", "carol"))
	assert.Equal(t, []string{"alice", "carol"}, userNames(t, cp))
}

// VALIDATES: an absent system root is REPORTED as such, and is distinguishable
// from a system root that declares no users. Both answer with no users; only
// one of them is a configuration somebody wrote.
// PREVENTS: the fault this reader actually has arriving silently. Provider.Get
// answers a missing root with an empty map and a nil error, so a root the
// daemon lost and a config with no users were one indistinguishable answer, and
// the guard reading users out of it could not say which it had.
func TestLiveConfigUsersReportsAnAbsentSystemRoot(t *testing.T) {
	cp := zeconfig.NewProvider()

	users, err := liveConfigUsers(cp)
	require.ErrorIs(t, err, errNoSystemConfigRoot, "no system root at all must say so")
	assert.Empty(t, users)

	// A system root that exists and declares no users is the other fact, and it
	// is not an error.
	cp.SetRoot("system", map[string]any{"authentication": map[string]any{}})
	users, err = liveConfigUsers(cp)
	require.NoError(t, err, "a system root that declares no users is a configuration, not a fault")
	assert.Empty(t, users)

	cp.SetRoot("system", systemRootWith("alice"))
	require.Len(t, userNames(t, cp), 1)

	cp.DeleteRoot("system")
	_, err = liveConfigUsers(cp)
	require.ErrorIs(t, err, errNoSystemConfigRoot,
		"a reload that drops the whole system root must report the root, not an empty list")
}

// VALIDATES: AC-7 through the sentinel. A configuration with no system block is
// legal, and the zefs power user keeps authenticating through it.
// PREVENTS: the absent-root report being treated as a read failure, which would
// deny every local login on a config that simply declares no users.
func TestLiveLocalUsersKeepsPowerUsersWithNoSystemRoot(t *testing.T) {
	cp := zeconfig.NewProvider()
	zefs := []authz.UserConfig{{Name: "zeadmin", Hash: "$2a$10$zeadmin"}}

	live := liveLocalUsers(zefs, func() ([]authz.UserConfig, error) { return liveConfigUsers(cp) }, nil)

	users, err := live()
	require.NoError(t, err, "a config with no system root must not refuse local authentication")
	require.Len(t, users, 1)
	assert.Equal(t, "zeadmin", users[0].Name)
}

// VALIDATES: no provider is an error, never an empty user list.
// PREVENTS: an unwired provider looking exactly like a configuration that
// declares nobody, which is the difference between a fault and a valid answer.
func TestLiveConfigUsersNilProviderIsAnError(t *testing.T) {
	users, err := liveConfigUsers(nil)
	require.ErrorIs(t, err, errNoLiveConfigProvider)
	assert.Nil(t, users, "a failed read returns no users to fall back on")
}

// VALIDATES: AC-13. The REST/gRPC bearer authenticator answers from the running
// configuration, so a user a reload removes loses API access with no restart.
// PREVENTS: the third credential surface staying on the boot snapshot. REST and
// gRPC accept "Bearer <user>:<pass>" and dispatch commands, so a deleted
// operator kept full API rights until the daemon was restarted.
func TestAPIUserAuthenticatorFollowsTheRunningConfig(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), bcrypt.MinCost)
	require.NoError(t, err)

	boot := []authz.UserConfig{{Name: "alice", Hash: string(hash)}}
	current := boot

	auth := buildUserAuthenticator(boot, func() ([]authz.UserConfig, error) { return current, nil })
	require.NotNil(t, auth)

	user, ok := auth("Bearer alice:testpass")
	require.True(t, ok, "alice must authenticate while the config declares her, or the refusal below proves nothing")
	assert.Equal(t, "alice", user)

	// The reload removes alice.
	current = nil

	_, ok = auth("Bearer alice:testpass")
	assert.False(t, ok, "a user the running config no longer declares must lose API access with no restart")
}

// VALIDATES: an API authenticator with no live source stays on its boot list.
// PREVENTS: the nil branch silently authenticating nobody, which would take
// every API caller down rather than leaving the caller's own list in charge.
func TestAPIUserAuthenticatorWithoutLiveSourceUsesItsList(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), bcrypt.MinCost)
	require.NoError(t, err)

	auth := buildUserAuthenticator([]authz.UserConfig{{Name: "alice", Hash: string(hash)}}, nil)
	require.NotNil(t, auth)

	user, ok := auth("Bearer alice:testpass")
	require.True(t, ok)
	assert.Equal(t, "alice", user)
}
