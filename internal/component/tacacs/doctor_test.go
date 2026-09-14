// Design: ai/rules/repo-maintenance.md -- TACACS+ server reachability readiness check tests
// Detail: doctor.go -- checkTACACSServers and its registration

package tacacs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// withServerProbe stands in the TCP probe for one test and records every
// address it was asked about.
func withServerProbe(t *testing.T, reachable bool) *[]string {
	t.Helper()
	previous := tacacsTCPReachable
	var asked []string
	tacacsTCPReachable = func(addr string, _ time.Duration) bool {
		asked = append(asked, addr)
		return reachable
	}
	t.Cleanup(func() { tacacsTCPReachable = previous })
	return &asked
}

// tacacsServerTree builds a config with one TACACS+ server at the given
// address, with a port leaf when port is not empty.
func tacacsServerTree(addr, port string) *config.Tree {
	tree := config.NewTree()
	tacacs := tree.GetOrCreateContainer("system").GetOrCreateContainer("authentication").GetOrCreateContainer("tacacs")
	server := config.NewTree()
	server.Set("address", addr)
	server.Set("key", "secret")
	if port != "" {
		server.Set("port", port)
	}
	tacacs.AddListEntry("server", addr, server)
	return tree
}

// TestCheckTACACSServersReportsUnreachableServers drives the check over a
// server that refuses the connection.
//
// VALIDATES: AC-6 unreachable TACACS+ servers return doctor-tacacs-unreachable
// as a warning, and the probe was given the address and the port the config
// names.
// PREVENTS: AAA outages being discovered only after login attempts fail.
func TestCheckTACACSServersReportsUnreachableServers(t *testing.T) {
	asked := withServerProbe(t, false)

	diags := checkTACACSServers(diagnostic.DoctorCheckContext{Tree: tacacsServerTree("192.0.2.1", "4949")})

	require.Len(t, diags, 1)
	assert.Equal(t, codeTACACSUnreachable, diags[0].Code)
	assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
	assert.Equal(t, []string{"192.0.2.1:4949"}, *asked)
}

// TestCheckTACACSServersProbesTheDefaultPort drives a server entry that names
// no port.
//
// VALIDATES: the probe goes to port 49, the port the backend itself connects
// to for such an entry (config.go, ExtractConfig).
// PREVENTS: the check probing a port the backend never uses, so its verdict
// says nothing about the session the daemon will open.
func TestCheckTACACSServersProbesTheDefaultPort(t *testing.T) {
	asked := withServerProbe(t, true)

	assert.Empty(t, checkTACACSServers(diagnostic.DoctorCheckContext{Tree: tacacsServerTree("192.0.2.1", "")}))
	assert.Equal(t, []string{"192.0.2.1:49"}, *asked)
}

// TestCheckTACACSServersIsSilentWithoutServers is the negative half.
//
// VALIDATES: no diagnostic for a config without a tacacs block and for a nil
// tree, and nothing is probed.
// PREVENTS: a warning on every box that authenticates locally.
func TestCheckTACACSServersIsSilentWithoutServers(t *testing.T) {
	asked := withServerProbe(t, false)

	assert.Empty(t, checkTACACSServers(diagnostic.DoctorCheckContext{Tree: config.NewTree()}), "no block")
	assert.Empty(t, checkTACACSServers(diagnostic.DoctorCheckContext{}), "nil tree")
	assert.Empty(t, *asked, "nothing to probe")
}

// TestTACACSDoctorCheckRegistered asks the registry the doctor runner reads
// whether it holds this backend's check, at the phase and order the
// declaration states, and whether the code it emits resolves for `ze explain`.
//
// VALIDATES: the init() in register.go installed the check and the registry
// accepted it.
// PREVENTS: a check that `ze doctor` stopped running while its unit tests
// stayed green.
func TestTACACSDoctorCheckRegistered(t *testing.T) {
	want := tacacsDoctorCheck
	var found *diagnostic.DoctorCheck
	checks := diagnostic.DoctorChecksForPhase(want.Phase)
	for i := range checks {
		if checks[i].Name == want.Name {
			found = &checks[i]
			break
		}
	}
	require.NotNil(t, found, "doctor check %q is not registered for phase %q", want.Name, want.Phase)
	assert.Equal(t, want.Order, found.Order)
	assert.Equal(t, want.Component, found.Component)

	diagnostic.RegisterBuiltinCodes()
	assert.NotNil(t, diagnostic.Lookup(codeTACACSUnreachable), "diagnostic code %q is not registered", codeTACACSUnreachable)
}
