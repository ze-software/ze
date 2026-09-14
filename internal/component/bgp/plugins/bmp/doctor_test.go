// Design: docs/features/ai-first.md -- the readiness check this plugin owns
// Detail: doctor.go -- checkBMPCollectors and its registration

package bmp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// withCollectorProbe stands in the TCP probe for one test and records every
// address it was asked about.
func withCollectorProbe(t *testing.T, reachable bool) *[]string {
	t.Helper()
	previous := bmpTCPReachable
	var asked []string
	bmpTCPReachable = func(addr string, _ time.Duration) bool {
		asked = append(asked, addr)
		return reachable
	}
	t.Cleanup(func() { bmpTCPReachable = previous })
	return &asked
}

// collectorTree builds a config with one BMP sender collector at the given
// address, with a port leaf when port is not empty.
func collectorTree(addr, port string) *config.Tree {
	tree := config.NewTree()
	sender := tree.GetOrCreateContainer("bgp").GetOrCreateContainer("bmp").GetOrCreateContainer("sender")
	collector := config.NewTree()
	collector.Set("address", addr)
	if port != "" {
		collector.Set("port", port)
	}
	sender.AddListEntry("collector", "c1", collector)
	return tree
}

// TestCheckBMPCollectorsReportsUnreachableCollectors drives the check over a
// collector that refuses the connection.
//
// VALIDATES: AC-10 external service reachability (BMP). The diagnostic is a
// warning under doctor-bmp-unreachable, and the probe was given the address
// and the port the config names.
// PREVENTS: a sender that never delivers a message with nothing in the report
// to say why.
func TestCheckBMPCollectorsReportsUnreachableCollectors(t *testing.T) {
	asked := withCollectorProbe(t, false)

	diags := checkBMPCollectors(diagnostic.DoctorCheckContext{Tree: collectorTree("192.0.2.10", "11019")})

	require.Len(t, diags, 1)
	assert.Equal(t, codeBMPUnreachable, diags[0].Code)
	assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
	assert.Equal(t, []string{"192.0.2.10:11019"}, *asked)
}

// TestCheckBMPCollectorsProbesTheSchemaDefaultPort drives a collector entry
// that names no port.
//
// VALIDATES: the probe goes to the port ze-bmp-conf.yang defaults the leaf to.
// PREVENTS: the check probing a port the sender never connects to, so its
// verdict says nothing about the session the daemon will open.
func TestCheckBMPCollectorsProbesTheSchemaDefaultPort(t *testing.T) {
	asked := withCollectorProbe(t, true)

	assert.Empty(t, checkBMPCollectors(diagnostic.DoctorCheckContext{Tree: collectorTree("192.0.2.10", "")}))
	assert.Equal(t, []string{"192.0.2.10:" + collectorDefaultPort}, *asked)
}

// TestCheckBMPCollectorsIsSilentWithoutCollectors is the negative half.
//
// VALIDATES: no diagnostic for a config without a sender block, without a bgp
// block, without a tree, for a collector that names no address, and for one
// that answers.
// PREVENTS: a warning on every box that runs no BMP sender.
func TestCheckBMPCollectorsIsSilentWithoutCollectors(t *testing.T) {
	asked := withCollectorProbe(t, false)

	assert.Empty(t, checkBMPCollectors(diagnostic.DoctorCheckContext{}))
	assert.Empty(t, checkBMPCollectors(diagnostic.DoctorCheckContext{Tree: config.NewTree()}))
	noSender := config.NewTree()
	noSender.GetOrCreateContainer("bgp").GetOrCreateContainer("bmp")
	assert.Empty(t, checkBMPCollectors(diagnostic.DoctorCheckContext{Tree: noSender}))
	assert.Empty(t, checkBMPCollectors(diagnostic.DoctorCheckContext{Tree: collectorTree("", "11019")}))
	assert.Empty(t, *asked, "a config that names no collector address probes nothing")

	bmpTCPReachable = func(string, time.Duration) bool { return true }
	assert.Empty(t, checkBMPCollectors(diagnostic.DoctorCheckContext{Tree: collectorTree("192.0.2.10", "11019")}))
}

// TestBMPDoctorCheckRegistered asks the registry the doctor runner reads
// whether it holds this plugin's check, at the phase and order declared, and
// whether the code it emits resolves for `ze explain`.
//
// VALIDATES: the init() in register.go installed the check and the registry
// accepted it.
// PREVENTS: a check that `ze doctor` stopped running while its unit tests
// stayed green.
func TestBMPDoctorCheckRegistered(t *testing.T) {
	var found *diagnostic.DoctorCheck
	checks := diagnostic.DoctorChecksForPhase(bmpDoctorCheck.Phase)
	for i := range checks {
		if checks[i].Name == bmpDoctorCheck.Name {
			found = &checks[i]
			break
		}
	}
	require.NotNilf(t, found, "doctor check %q is not registered for phase %q", bmpDoctorCheck.Name, bmpDoctorCheck.Phase)
	assert.Equal(t, bmpDoctorCheck.Order, found.Order)
	assert.Equal(t, bmpDoctorCheck.Component, found.Component)

	diagnostic.RegisterBuiltinCodes()
	assert.NotNil(t, diagnostic.Lookup(codeBMPUnreachable), "diagnostic code is not registered")
}
