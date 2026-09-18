// Design: docs/features/interfaces.md -- the readiness checks this plugin owns
// Detail: doctor.go -- checkNTPClient and the registrations

package ntp

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// ntpTree builds the smallest ENABLED config that carries an environment/ntp
// block, with one server for each address. Every caller wants it enabled; a
// test of the disabled arm builds its own tree rather than passing a flag
// whose only value is true.
func ntpTree(servers ...string) *config.Tree {
	tree := config.NewTree()
	ntp := tree.GetOrCreateContainer(configRootEnvironment).GetOrCreateContainer("ntp")
	ntp.Set("enabled", "true")
	for _, addr := range servers {
		server := config.NewTree()
		server.Set("address", addr)
		ntp.AddListEntry("server", addr, server)
	}
	return tree
}

// withServerProbe stands in every NTP server answering, or none, for the life
// of one test.
func withServerProbe(t *testing.T, reachable bool) {
	t.Helper()
	previous := ntpServerReachable
	ntpServerReachable = func(string, time.Duration) bool { return reachable }
	t.Cleanup(func() { ntpServerReachable = previous })
}

func platform(kind host.PlatformType) *host.PlatformInfo {
	return &host.PlatformInfo{Type: kind}
}

func requireOneDiag(t *testing.T, diags []diagnostic.Diagnostic, code string, severity diagnostic.Severity) diagnostic.Diagnostic {
	t.Helper()
	if len(diags) != 1 {
		t.Fatalf("diagnostics = %d, want 1: %+v", len(diags), diags)
	}
	if diags[0].Code != code || diags[0].Severity != severity {
		t.Fatalf("diagnostic = %s/%s, want %s at %s", diags[0].Code, diags[0].Severity, code, severity)
	}
	return diags[0]
}

// TestCheckNTPClientReportsADisabledClientPerPlatform drives the disabled
// verdict over every platform: an error on gokrazy, a warning on the Linux
// kinds, and nothing on darwin, unknown, or a nil platform.
//
// VALIDATES: doctor-clock-no-sync at the severity each platform earns, naming
// the leaf to enable, and no diagnostic where ze does not manage the clock.
// PREVENTS: appliances booting with neither gokrazy NTP nor Ze NTP configured,
// and developer hosts getting appliance-specific warnings.
func TestCheckNTPClientReportsADisabledClientPerPlatform(t *testing.T) {
	cases := []struct {
		platform *host.PlatformInfo
		severity diagnostic.Severity
		reported bool
	}{
		{platform(host.PlatformGokrazy), diagnostic.SeverityError, true},
		{platform(host.PlatformSystemd), diagnostic.SeverityWarning, true},
		{platform(host.PlatformContainer), diagnostic.SeverityWarning, true},
		{platform(host.PlatformPlainLinux), diagnostic.SeverityWarning, true},
		{platform(host.PlatformDarwin), "", false},
		{platform(host.PlatformUnknown), "", false},
		{nil, "", false},
	}
	for _, tc := range cases {
		name := "nil"
		if tc.platform != nil {
			name = tc.platform.Type.String()
		}
		diags := checkNTPClient(diagnostic.DoctorCheckContext{Tree: config.NewTree(), Platform: tc.platform})
		if !tc.reported {
			if len(diags) != 0 {
				t.Fatalf("%s: diagnostics = %d, want 0: %+v", name, len(diags), diags)
			}
			continue
		}
		diag := requireOneDiag(t, diags, codeClockNoSync, tc.severity)
		if diag.Path != "environment/ntp/enabled" {
			t.Fatalf("%s: path = %q, want the enabled leaf", name, diag.Path)
		}
	}
}

// TestCheckNTPClientReportsUnreachableServers drives an enabled client whose
// only server answers nothing.
//
// VALIDATES: doctor-ntp-server-unreachable at warning severity, and no
// clock-sync finding for an enabled client.
// PREVENTS: a client that can never synchronize passing readiness.
func TestCheckNTPClientReportsUnreachableServers(t *testing.T) {
	withServerProbe(t, false)

	diags := checkNTPClient(diagnostic.DoctorCheckContext{Tree: ntpTree("pool.ntp.org"), Platform: platform(host.PlatformGokrazy)})
	requireOneDiag(t, diags, codeNTPServerUnreachable, diagnostic.SeverityWarning)
}

// TestCheckNTPClientIsSilentWhenAServerAnswers is the positive half, and the
// two cases that probe nothing: an enabled client with no server, and no
// config at all.
//
// VALIDATES: no diagnostic when a server answers, when none is configured, and
// for a nil tree.
// PREVENTS: configured Ze-owned clock sync being reported as absent.
func TestCheckNTPClientIsSilentWhenAServerAnswers(t *testing.T) {
	withServerProbe(t, true)
	if diags := checkNTPClient(diagnostic.DoctorCheckContext{Tree: ntpTree("pool.ntp.org"), Platform: platform(host.PlatformGokrazy)}); len(diags) != 0 {
		t.Fatalf("server answers: diagnostics = %d, want 0: %+v", len(diags), diags)
	}

	withServerProbe(t, false)
	if diags := checkNTPClient(diagnostic.DoctorCheckContext{Tree: ntpTree(), Platform: platform(host.PlatformGokrazy)}); len(diags) != 0 {
		t.Fatalf("no server: diagnostics = %d, want 0: %+v", len(diags), diags)
	}
	if diags := checkNTPClient(diagnostic.DoctorCheckContext{Platform: platform(host.PlatformGokrazy)}); len(diags) != 0 {
		t.Fatalf("nil tree: diagnostics = %d, want 0: %+v", len(diags), diags)
	}
}

// TestNTPDoctorChecksRegistered asks the registry the doctor runner reads
// whether it holds every check this plugin declares, at the phase and order
// each declaration states, and whether every code resolves for `ze explain`.
//
// VALIDATES: the init() in register.go installed the table and the registry
// accepted it.
// PREVENTS: a check that `ze doctor` stopped running while its unit tests
// stayed green.
func TestNTPDoctorChecksRegistered(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()
	if len(ntpDoctorChecks) == 0 {
		t.Fatal("this plugin declares no check: the loop below would pass over an empty table")
	}
	for i := range ntpDoctorChecks {
		want := ntpDoctorChecks[i]
		var found *diagnostic.DoctorCheck
		checks := diagnostic.DoctorChecksForPhase(want.Phase)
		for j := range checks {
			if checks[j].Name == want.Name {
				found = &checks[j]
				break
			}
		}
		if found == nil {
			t.Errorf("doctor check %q is not registered for phase %q", want.Name, want.Phase)
			continue
		}
		if found.Order != want.Order {
			t.Errorf("check %q: order = %d, want %d", want.Name, found.Order, want.Order)
		}
		if found.Component != want.Component {
			t.Errorf("check %q: component = %q, want %q", want.Name, found.Component, want.Component)
		}
		for _, code := range want.Codes {
			if diagnostic.Lookup(code) == nil {
				t.Errorf("check %q declares code %q, which the code registry does not hold", want.Name, code)
			}
		}
	}
}
