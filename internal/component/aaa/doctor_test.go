// Detail: doctor.go -- checkAAALocalFallback

package aaa

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// authTree builds the system/authentication subtree a config declares. Each
// remote backend named gets one server entry, and each user named gets one
// entry, which is all this check reads.
func authTree(t *testing.T, remote, users []string) *config.Tree {
	t.Helper()
	tree := config.NewTree()
	auth := tree.GetOrCreateContainer("system").GetOrCreateContainer("authentication")
	for _, backend := range remote {
		server := config.NewTree()
		server.Set("port", "49")
		auth.GetOrCreateContainer(backend).AddListEntry("server", "192.0.2.1", server)
	}
	for _, name := range users {
		entry := config.NewTree()
		entry.Set("password", "$2a$04$notarealhash")
		auth.AddListEntry("user", name, entry)
	}
	return tree
}

func fallbackDiagnostics(t *testing.T, tree *config.Tree) []diagnostic.Diagnostic {
	t.Helper()
	return checkAAALocalFallback(diagnostic.DoctorCheckContext{Tree: tree})
}

// TestDoctorWarnsWhenARemoteBackendIsTheOnlyWayIn is the case the check exists
// for. The chain reaches the local backend only when the remote one fails to
// answer, and that fallback is an ACCOUNT: with none declared, an unreachable
// server or a backend whose config will not build leaves no login at all.
//
// VALIDATES: `ze doctor` names the risk from the config, before the daemon
// starts and before an operator discovers it during an outage.
// PREVENTS: the lockout this session's work made survivable but not visible.
func TestDoctorWarnsWhenARemoteBackendIsTheOnlyWayIn(t *testing.T) {
	for _, tc := range []struct {
		name    string
		remote  []string
		expects string
	}{
		{name: "tacacs alone", remote: []string{"tacacs"}, expects: "TACACS+"},
		{name: "radius alone", remote: []string{"radius"}, expects: "RADIUS"},
		{name: "both", remote: []string{"tacacs", "radius"}, expects: "RADIUS and TACACS+"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			found := fallbackDiagnostics(t, authTree(t, tc.remote, nil))

			require.Len(t, found, 1, "a remote backend with no local user must be reported")
			assert.Equal(t, "doctor-aaa-no-local-fallback", found[0].Code)
			assert.Equal(t, diagnostic.SeverityWarning, found[0].Severity)
			assert.Contains(t, found[0].Message, tc.expects, "the message names the backends the config declares")
			assert.Contains(t, found[0].Message, "system.authentication.user", "the message names the repair")
		})
	}
}

// TestDoctorIsSilentWhenALocalAccountExists is what keeps the check from
// warning about every box that uses central auth correctly. One local user is
// the whole requirement: the chain has something to fall back to.
func TestDoctorIsSilentWhenALocalAccountExists(t *testing.T) {
	assert.Empty(t, fallbackDiagnostics(t, authTree(t, []string{"tacacs"}, []string{"opsadmin"})),
		"a declared local user is exactly what the chain falls back to")
}

// TestDoctorIsSilentWithNoRemoteBackend covers the other half of the guard. A
// box with local accounts alone has nothing to fall back FROM, so there is no
// risk to name and a warning would be noise on most configs.
func TestDoctorIsSilentWithNoRemoteBackend(t *testing.T) {
	assert.Empty(t, fallbackDiagnostics(t, authTree(t, nil, []string{"opsadmin"})))
	assert.Empty(t, fallbackDiagnostics(t, authTree(t, nil, nil)))
}

// TestDoctorIsSilentWhenABackendDeclaresNoServer checks the shape an operator
// leaves behind while editing: an empty `tacacs {}` container configures no
// server, so no backend decides logins and there is nothing to warn about.
func TestDoctorIsSilentWhenABackendDeclaresNoServer(t *testing.T) {
	tree := config.NewTree()
	auth := tree.GetOrCreateContainer("system").GetOrCreateContainer("authentication")
	auth.GetOrCreateContainer("tacacs").Set("timeout", "5")

	assert.Empty(t, fallbackDiagnostics(t, tree), "a backend with no server authenticates nobody")
}

// TestDoctorReadsNothingItWasNotGiven keeps the check from answering about a
// tree it never saw. `ze doctor` runs phases over configs that may be absent or
// of another type, and a check that invented a verdict there would report a
// risk the operator cannot act on.
func TestDoctorReadsNothingItWasNotGiven(t *testing.T) {
	assert.Empty(t, checkAAALocalFallback(diagnostic.DoctorCheckContext{}))
	assert.Empty(t, checkAAALocalFallback(diagnostic.DoctorCheckContext{Tree: "not a tree"}))
	assert.Empty(t, fallbackDiagnostics(t, config.NewTree()), "a tree with no system container")
}

// TestDoctorCheckIsRegistered proves the check reaches `ze doctor` at all. A
// check nothing registers is dead code, and the registration lives in a
// separate file from the check it names.
func TestDoctorCheckIsRegistered(t *testing.T) {
	var found *diagnostic.DoctorCheck
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhasePostConfig) {
		if check.Name == "aaa-no-local-fallback" {
			found = &check
			break
		}
	}
	require.NotNil(t, found, "the check must be registered from register.go")
	assert.Contains(t, found.Codes, doctorCodeNoLocalFallback)

	// The builtin table is registered by the binary entry point rather than by
	// an init, so a unit test registers it to ask what `ze show diagnostic` would print.
	diagnostic.RegisterBuiltinCodes()
	meta := diagnostic.Lookup(doctorCodeNoLocalFallback)
	require.NotNil(t, meta, "every code a check emits owes an entry ze show diagnostic can print")
	assert.NotEmpty(t, meta.Description, "ze show diagnostic must have something to print")
	assert.True(t, strings.Contains(meta.Description, "system/authentication/user"),
		"the description must name the config that repairs it")
	assert.True(t, strings.Contains(meta.Description, "no login"),
		"the description must say what the operator loses")
}

// TestAAADoctorCheckAllAccountsExcluded verifies that
// doctor-aaa-all-accounts-excluded warns with an actionable fix when every
// configured local user and remote AAA mapping is excluded from every
// registered login service, and stays silent when no accounts exist or at least
// one account can reach a registered door.
//
// VALIDATES: Post-config AAA doctor check warns on total login-service lockout
// and registers both its check and its diagnostic code metadata.
// PREVENTS: Operators committing a config where every configured account is
// locked out of every surface without a doctor warning and repair command.
func TestAAADoctorCheckAllAccountsExcluded(t *testing.T) {
	registerTestLoginServices(t, "ssh", "web")

	buildTree := func(sets map[string][]string, users map[string]string) *config.Tree {
		tree := config.NewTree()
		sys := tree.GetOrCreateContainer("system")
		authz := sys.GetOrCreateContainer("authorization")
		for name, services := range sets {
			entry := config.NewTree()
			if len(services) > 0 {
				entry.SetSlice("service", services)
			}
			authz.AddListEntry("login-service", name, entry)
		}
		auth := sys.GetOrCreateContainer("authentication")
		for name, setName := range users {
			entry := config.NewTree()
			entry.Set("password", "$2a$04$notarealhash")
			if setName != "" {
				entry.Set("login-service", setName)
			}
			auth.AddListEntry("user", name, entry)
		}
		return tree
	}

	t.Run("warns when every local user references an empty login-service set", func(t *testing.T) {
		tree := buildTree(
			map[string][]string{"locked": {}},
			map[string]string{"alice": "locked"},
		)
		diags := checkAAAAllAccountsExcluded(diagnostic.DoctorCheckContext{Tree: tree})
		require.Len(t, diags, 1)
		assert.Equal(t, CodeDoctorAAAAllAccountsExcluded, diags[0].Code)
		assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
		assert.NotEmpty(t, diags[0].Fix)
		assert.Contains(t, diags[0].Fix, "set system authorization login-service locked service")
	})

	t.Run("warns when every user references a set with only unregistered services", func(t *testing.T) {
		tree := buildTree(
			map[string][]string{"ghost": {"unregistered-door"}},
			map[string]string{"alice": "ghost"},
		)
		diags := checkAAAAllAccountsExcluded(diagnostic.DoctorCheckContext{Tree: tree})
		require.Len(t, diags, 1)
		assert.Equal(t, CodeDoctorAAAAllAccountsExcluded, diags[0].Code)
		assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
	})

	t.Run("silent when at least one user has an allowed registered door", func(t *testing.T) {
		tree := buildTree(
			map[string][]string{"locked": {}, "ops": {"ssh"}},
			map[string]string{"alice": "locked", "bob": "ops"},
		)
		assert.Empty(t, checkAAAAllAccountsExcluded(diagnostic.DoctorCheckContext{Tree: tree}))
	})

	t.Run("silent when at least one user omits login-service", func(t *testing.T) {
		tree := buildTree(
			map[string][]string{"locked": {}},
			map[string]string{"alice": "locked", "bob": ""},
		)
		assert.Empty(t, checkAAAAllAccountsExcluded(diagnostic.DoctorCheckContext{Tree: tree}))
	})

	t.Run("silent when no accounts are configured", func(t *testing.T) {
		tree := buildTree(map[string][]string{"locked": {}}, nil)
		assert.Empty(t, checkAAAAllAccountsExcluded(diagnostic.DoctorCheckContext{Tree: tree}))
	})

	t.Run("silent when remote RADIUS admits a door even if local user is locked", func(t *testing.T) {
		tree := buildTree(
			map[string][]string{"locked": {}, "ops": {"web"}},
			map[string]string{"alice": "locked"},
		)
		radius := tree.GetOrCreateContainer("system").GetOrCreateContainer("authentication").GetOrCreateContainer("radius")
		srv := config.NewTree()
		srv.Set("key", "secret")
		radius.AddListEntry("server", "192.0.2.10", srv)
		radius.Set("login-service", "ops")

		assert.Empty(t, checkAAAAllAccountsExcluded(diagnostic.DoctorCheckContext{Tree: tree}))
	})

	t.Run("warns when both local user and remote RADIUS are locked", func(t *testing.T) {
		tree := buildTree(
			map[string][]string{"locked": {}},
			map[string]string{"alice": "locked"},
		)
		radius := tree.GetOrCreateContainer("system").GetOrCreateContainer("authentication").GetOrCreateContainer("radius")
		srv := config.NewTree()
		srv.Set("key", "secret")
		radius.AddListEntry("server", "192.0.2.10", srv)
		radius.Set("login-service", "locked")

		diags := checkAAAAllAccountsExcluded(diagnostic.DoctorCheckContext{Tree: tree})
		require.Len(t, diags, 1)
		assert.Equal(t, CodeDoctorAAAAllAccountsExcluded, diags[0].Code)
	})

	t.Run("check and code metadata are registered", func(t *testing.T) {
		var found *diagnostic.DoctorCheck
		for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhasePostConfig) {
			if check.Name == "aaa-all-accounts-excluded" {
				found = &check
				break
			}
		}
		require.NotNil(t, found, "aaa-all-accounts-excluded must be registered from register.go")
		assert.Contains(t, found.Codes, CodeDoctorAAAAllAccountsExcluded)

		meta := diagnostic.Lookup(CodeDoctorAAAAllAccountsExcluded)
		require.NotNil(t, meta, "doctor-aaa-all-accounts-excluded metadata must be registered")
		assert.NotEmpty(t, meta.Title)
		assert.NotEmpty(t, meta.Description)
	})
}
