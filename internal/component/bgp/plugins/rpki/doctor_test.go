// Design: docs/features/ai-first.md -- the readiness check this plugin owns
// Detail: doctor.go -- checkRPKICacheServers and its registration

package rpki

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// withCacheProbe stands in the TCP probe for one test and records every
// address it was asked about.
func withCacheProbe(t *testing.T, reachable bool) *[]string {
	t.Helper()
	previous := rpkiTCPReachable
	var asked []string
	rpkiTCPReachable = func(addr string, _ time.Duration) bool {
		asked = append(asked, addr)
		return reachable
	}
	t.Cleanup(func() { rpkiTCPReachable = previous })
	return &asked
}

// cacheServerTree builds a config with one RPKI cache at the given address,
// with a port leaf when port is not empty.
func cacheServerTree(addr, port string) *config.Tree {
	tree := config.NewTree()
	rpki := tree.GetOrCreateContainer("bgp").GetOrCreateContainer("rpki")
	server := config.NewTree()
	server.Set("trusted-network", "true")
	if port != "" {
		server.Set("port", port)
	}
	rpki.AddListEntry("cache-server", addr, server)
	return tree
}

// TestCheckRPKICacheServersReportsUnreachableCaches drives the check over a
// cache that refuses the connection.
//
// VALIDATES: AC-10 external service reachability (RPKI). The diagnostic is a
// warning under doctor-rpki-unreachable, and the probe was given the address
// and the port the config names.
// PREVENTS: an RTR client that never synchronizes with nothing in the report
// to say why.
func TestCheckRPKICacheServersReportsUnreachableCaches(t *testing.T) {
	asked := withCacheProbe(t, false)

	diags := checkRPKICacheServers(diagnostic.DoctorCheckContext{Tree: cacheServerTree("192.0.2.1", "8282")})

	require.Len(t, diags, 1)
	assert.Equal(t, codeRPKIUnreachable, diags[0].Code)
	assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
	assert.Equal(t, []string{"192.0.2.1:8282"}, *asked)
}

// TestCheckRPKICacheServersProbesTheRTRDefaultPort drives a cache entry that
// names no port.
//
// VALIDATES: the probe goes to port 323, the port the plugin itself connects
// to for such an entry (rpki_config.go).
// PREVENTS: the check probing a port the plugin never uses, so its verdict
// says nothing about the session the daemon will open.
func TestCheckRPKICacheServersProbesTheRTRDefaultPort(t *testing.T) {
	asked := withCacheProbe(t, true)

	assert.Empty(t, checkRPKICacheServers(diagnostic.DoctorCheckContext{Tree: cacheServerTree("192.0.2.1", "")}))
	assert.Equal(t, []string{"192.0.2.1:323"}, *asked)
}

// TestCheckRPKICacheServersIsSilentWithoutCaches is the negative half.
//
// VALIDATES: no diagnostic for a config without an rpki block, without a bgp
// block, without a tree, and for one whose cache answers.
// PREVENTS: a warning on every box that runs no RTR client.
func TestCheckRPKICacheServersIsSilentWithoutCaches(t *testing.T) {
	asked := withCacheProbe(t, false)

	assert.Empty(t, checkRPKICacheServers(diagnostic.DoctorCheckContext{}))
	assert.Empty(t, checkRPKICacheServers(diagnostic.DoctorCheckContext{Tree: config.NewTree()}))
	noRPKI := config.NewTree()
	noRPKI.GetOrCreateContainer("bgp")
	assert.Empty(t, checkRPKICacheServers(diagnostic.DoctorCheckContext{Tree: noRPKI}))
	emptyRPKI := config.NewTree()
	emptyRPKI.GetOrCreateContainer("bgp").GetOrCreateContainer("rpki")
	assert.Empty(t, checkRPKICacheServers(diagnostic.DoctorCheckContext{Tree: emptyRPKI}))
	assert.Empty(t, *asked, "a config that names no cache probes nothing")

	rpkiTCPReachable = func(string, time.Duration) bool { return true }
	assert.Empty(t, checkRPKICacheServers(diagnostic.DoctorCheckContext{Tree: cacheServerTree("192.0.2.1", "323")}))
}

// TestRPKIDoctorCheckRegistered asks the registry the doctor runner reads
// whether it holds this plugin's check, at the phase and order declared, and
// whether the code it emits resolves for `ze explain`.
//
// VALIDATES: the init() in register.go installed the check and the registry
// accepted it.
// PREVENTS: a check that `ze doctor` stopped running while its unit tests
// stayed green.
func TestRPKIDoctorCheckRegistered(t *testing.T) {
	var found *diagnostic.DoctorCheck
	checks := diagnostic.DoctorChecksForPhase(rpkiDoctorCheck.Phase)
	for i := range checks {
		if checks[i].Name == rpkiDoctorCheck.Name {
			found = &checks[i]
			break
		}
	}
	require.NotNilf(t, found, "doctor check %q is not registered for phase %q", rpkiDoctorCheck.Name, rpkiDoctorCheck.Phase)
	assert.Equal(t, rpkiDoctorCheck.Order, found.Order)
	assert.Equal(t, rpkiDoctorCheck.Component, found.Component)

	diagnostic.RegisterBuiltinCodes()
	assert.NotNil(t, diagnostic.Lookup(codeRPKIUnreachable), "diagnostic code is not registered")
}
