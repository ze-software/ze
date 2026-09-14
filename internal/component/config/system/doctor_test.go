// Design: docs/features/ai-first.md -- the readiness checks this component owns
// Detail: doctor.go -- the four checks and their registration
//
// These cases arrived from internal/component/doctor with the checks they
// drive, case for case.

package system

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func testPlatform(platformType host.PlatformType) *host.PlatformInfo {
	return &host.PlatformInfo{Type: platformType}
}

// withDNSProbe stands in the DNS probe for one test and records every server
// it was asked about.
func withDNSProbe(t *testing.T, responds bool) *[]string {
	t.Helper()
	previous := dnsResolverResponds
	var asked []string
	dnsResolverResponds = func(addr string) bool {
		asked = append(asked, addr)
		return responds
	}
	t.Cleanup(func() { dnsResolverResponds = previous })
	return &asked
}

// withUpdateCheckProbe stands in the HTTP HEAD probe for one test.
func withUpdateCheckProbe(t *testing.T, err error) {
	t.Helper()
	previous := updateCheckHTTPHead
	updateCheckHTTPHead = func(string, time.Duration) error { return err }
	t.Cleanup(func() { updateCheckHTTPHead = previous })
}

func nameServerTree(servers ...string) *config.Tree {
	tree := config.NewTree()
	tree.GetOrCreateContainer("system").SetSlice("name-server", servers)
	return tree
}

func updateCheckTree(url string) *config.Tree {
	tree := config.NewTree()
	tree.GetOrCreateContainer("system").GetOrCreateContainer("update-check").Set("url", url)
	return tree
}

func resolvConfTree(path string) *config.Tree {
	tree := config.NewTree()
	tree.GetOrCreateContainer("system").GetOrCreateContainer("dns").Set("resolv-conf-path", path)
	return tree
}

func requireDiag(t *testing.T, diags []diagnostic.Diagnostic, code string, severity diagnostic.Severity) {
	t.Helper()
	for i := range diags {
		if diags[i].Code == code {
			assert.Equal(t, severity, diags[i].Severity, "severity for %s", code)
			return
		}
	}
	require.Failf(t, "missing diagnostic", "expected %s in %+v", code, diags)
}

func assertNoDiagCode(t *testing.T, diags []diagnostic.Diagnostic, code string) {
	t.Helper()
	for i := range diags {
		assert.NotEqual(t, code, diags[i].Code, "unexpected diagnostic: %+v", diags[i])
	}
}

// --- DNS resolver tests ---

func TestCheckDNSResolvers_NoSystemBlock(t *testing.T) {
	asked := withDNSProbe(t, false)
	assert.Empty(t, checkDNSResolvers(diagnostic.DoctorCheckContext{Tree: config.NewTree()}))
	assert.Empty(t, checkDNSResolvers(diagnostic.DoctorCheckContext{}), "nil tree")
	assert.Empty(t, *asked)
}

func TestCheckDNSResolvers_NoNameServers(t *testing.T) {
	asked := withDNSProbe(t, false)
	tree := config.NewTree()
	tree.GetOrCreateContainer("system")
	assert.Empty(t, checkDNSResolvers(diagnostic.DoctorCheckContext{Tree: tree}))
	assert.Empty(t, *asked)
}

func TestCheckDNSResolvers_UnreachableServer(t *testing.T) {
	asked := withDNSProbe(t, false)
	diags := checkDNSResolvers(diagnostic.DoctorCheckContext{Tree: nameServerTree("192.0.2.254")})
	require.Len(t, diags, 1)
	assert.Equal(t, codeDNSResolver, diags[0].Code)
	assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
	assert.Equal(t, []string{"192.0.2.254"}, *asked)
}

// TestCheckDNSResolvers_OneAnswerIsEnough pins the failover shape: the first
// server that answers ends the probe.
func TestCheckDNSResolvers_OneAnswerIsEnough(t *testing.T) {
	asked := withDNSProbe(t, true)
	assert.Empty(t, checkDNSResolvers(diagnostic.DoctorCheckContext{Tree: nameServerTree("192.0.2.1", "192.0.2.2")}))
	assert.Equal(t, []string{"192.0.2.1"}, *asked)
}

func TestDNSServerResponds_Unreachable(t *testing.T) {
	if testing.Short() {
		t.Skip("requires network timeout")
	}
	assert.False(t, dnsServerResponds("192.0.2.254"))
}

// --- update-check tests ---

func TestDoctorUpdateCheckURL_Unreachable(t *testing.T) {
	withUpdateCheckProbe(t, errors.New("connection refused"))

	diags := checkUpdateCheckURL(diagnostic.DoctorCheckContext{
		Tree:     updateCheckTree("https://update.example.invalid/version.json"),
		Platform: testPlatform(host.PlatformPlainLinux),
	})
	require.Len(t, diags, 1)
	assert.Equal(t, codeUpdateCheckUnreachable, diags[0].Code)
	assert.Equal(t, "https://update.example.invalid/version.json", diags[0].Path)
}

func TestDoctorUpdateCheckURL_NoConfig(t *testing.T) {
	withUpdateCheckProbe(t, errors.New("connection refused"))
	assert.Empty(t, checkUpdateCheckURL(diagnostic.DoctorCheckContext{Tree: config.NewTree()}))
	assert.Empty(t, checkUpdateCheckURL(diagnostic.DoctorCheckContext{}), "nil tree")
}

// TestDoctorUpdateCheckURL_GokrazySkipsTheProbe pins the platform split: on
// gokrazy the block is ignored, so its URL is not probed and the backend
// check reports the block instead.
func TestDoctorUpdateCheckURL_GokrazySkipsTheProbe(t *testing.T) {
	withUpdateCheckProbe(t, errors.New("connection refused"))
	assert.Empty(t, checkUpdateCheckURL(diagnostic.DoctorCheckContext{
		Tree:     updateCheckTree("https://update.example.invalid/version.json"),
		Platform: testPlatform(host.PlatformGokrazy),
	}))
}

func TestDoctorGokrazyWarnsIgnoredConfig(t *testing.T) {
	// VALIDATES: AC-10 ze doctor on gokrazy with update-check config warns that Ze self-update config is ignored.
	// PREVENTS: operators believing update-check config controls gokrazy image updates.
	diags := checkUpdateBackendConfig(diagnostic.DoctorCheckContext{
		Tree:     updateCheckTree("https://update.example.com/version.json"),
		Platform: testPlatform(host.PlatformGokrazy),
	})
	require.Len(t, diags, 1)
	assert.Equal(t, codeConfigPlatformMismatch, diags[0].Code)
	assert.Contains(t, diags[0].Message, "ignored on gokrazy")
}

func TestDoctorUpdateBackendConfig_SilentOffGokrazy(t *testing.T) {
	tree := updateCheckTree("https://update.example.com/version.json")
	assert.Empty(t, checkUpdateBackendConfig(diagnostic.DoctorCheckContext{Tree: tree, Platform: testPlatform(host.PlatformSystemd)}))
	assert.Empty(t, checkUpdateBackendConfig(diagnostic.DoctorCheckContext{Tree: tree}), "nil platform")
	assert.Empty(t, checkUpdateBackendConfig(diagnostic.DoctorCheckContext{Tree: config.NewTree(), Platform: testPlatform(host.PlatformGokrazy)}), "no block")
}

// --- resolv.conf path tests ---

func TestCheckResolvConfMismatchSystemd(t *testing.T) {
	// VALIDATES: AC-10 /tmp/resolv.conf on systemd emits doctor-config-platform-mismatch.
	// PREVENTS: gokrazy DNS defaults being silently used on standard Linux.
	diags := checkResolvConfPath(diagnostic.DoctorCheckContext{Tree: resolvConfTree("/tmp/resolv.conf"), Platform: testPlatform(host.PlatformSystemd)})

	requireDiag(t, diags, codeConfigPlatformMismatch, diagnostic.SeverityWarning)
}

func TestCheckResolvConfMismatchGokrazy(t *testing.T) {
	// VALIDATES: AC-11 /etc/resolv.conf on gokrazy emits doctor-config-platform-mismatch.
	// PREVENTS: writing DNS config into a read-only gokrazy rootfs path.
	diags := checkResolvConfPath(diagnostic.DoctorCheckContext{Tree: resolvConfTree("/etc/resolv.conf"), Platform: testPlatform(host.PlatformGokrazy)})

	requireDiag(t, diags, codeConfigPlatformMismatch, diagnostic.SeverityWarning)
}

func TestCheckResolvConfMatchGokrazy(t *testing.T) {
	// VALIDATES: AC-12 /tmp/resolv.conf on gokrazy emits no mismatch diagnostic.
	// PREVENTS: appliance DNS defaults being reported as wrong on appliances.
	diags := checkResolvConfPath(diagnostic.DoctorCheckContext{Tree: resolvConfTree("/tmp/resolv.conf"), Platform: testPlatform(host.PlatformGokrazy)})

	assertNoDiagCode(t, diags, codeConfigPlatformMismatch)
}

func TestCheckResolvConfNilPlatform(t *testing.T) {
	// VALIDATES: AC-15 nil platform preserves current behavior without new coherence diagnostics.
	// PREVENTS: platform detection failures from crashing or inventing platform-specific warnings.
	diags := checkResolvConfPath(diagnostic.DoctorCheckContext{Tree: resolvConfTree("/tmp/resolv.conf")})

	assertNoDiagCode(t, diags, codeConfigPlatformMismatch)
	assert.Empty(t, checkResolvConfPath(diagnostic.DoctorCheckContext{Platform: testPlatform(host.PlatformSystemd)}), "nil tree")
}

// TestCheckResolvConfDefaultOnSystemd pins the case an operator meets most: no
// dns block at all on a standard Linux, where the YANG default is the gokrazy
// path.
func TestCheckResolvConfDefaultOnSystemd(t *testing.T) {
	diags := checkResolvConfPath(diagnostic.DoctorCheckContext{Tree: config.NewTree(), Platform: testPlatform(host.PlatformSystemd)})
	requireDiag(t, diags, codeConfigPlatformMismatch, diagnostic.SeverityWarning)
}

// TestSystemDoctorChecksRegistered asks the registry the doctor runner reads
// whether it holds every check this component declares, at the phase and
// order each declaration states, and whether every code they emit resolves
// for `ze explain`.
//
// VALIDATES: the init() in register.go installed the checks and the registry
// accepted them.
// PREVENTS: a check that `ze doctor` stopped running while its unit tests
// stayed green.
func TestSystemDoctorChecksRegistered(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()
	for i := range systemDoctorChecks {
		want := systemDoctorChecks[i]
		var found *diagnostic.DoctorCheck
		checks := diagnostic.DoctorChecksForPhase(want.Phase)
		for j := range checks {
			if checks[j].Name == want.Name {
				found = &checks[j]
				break
			}
		}
		require.NotNil(t, found, "doctor check %q is not registered for phase %q", want.Name, want.Phase)
		assert.Equal(t, want.Order, found.Order, "order of %s", want.Name)
		assert.Equal(t, want.Component, found.Component, "component of %s", want.Name)
		for _, code := range want.Codes {
			assert.NotNil(t, diagnostic.Lookup(code), "diagnostic code %q is not registered", code)
		}
	}
}
