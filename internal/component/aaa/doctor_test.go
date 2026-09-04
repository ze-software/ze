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
func authTree(t *testing.T, remote []string, users []string) *config.Tree {
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
	// an init, so a unit test registers it to ask what `ze explain` would print.
	diagnostic.RegisterBuiltinCodes()
	meta := diagnostic.Lookup(doctorCodeNoLocalFallback)
	require.NotNil(t, meta, "every code a check emits owes an entry ze explain can print")
	assert.NotEmpty(t, meta.Description, "ze explain must have something to print")
	assert.True(t, strings.Contains(meta.Description, "system/authentication/user"),
		"the description must name the config that repairs it")
	assert.True(t, strings.Contains(meta.Description, "no login"),
		"the description must say what the operator loses")
}
