// Design: ai/rules/repo-maintenance.md -- doctor checks owned by the plugin that
// owns the runtime dependency
// Related: doctor.go -- checkVPPLCPPlugin and lcpEnabled under test

package ifacevpp

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// vppctlPluginsWithLCP and vppctlPluginsWithoutLCP are `vppctl show plugins`
// output, trimmed to three rows. The check reads this text, so the fixtures
// keep its shape: a header, then one numbered row per loaded plugin.
const (
	vppctlPluginsWithLCP = ` Plugin path is: /usr/lib/x86_64-linux-gnu/vpp_plugins
     Plugin                                   Version                Description
  1. dhcp_plugin.so                           25.02-release          Dynamic Host Configuration Protocol (DHCP)
  2. linux_cp_plugin.so                       25.02-release          Linux Control Plane - Interface Mirroring
  3. nat_plugin.so                            25.02-release          Network Address Translation (NAT)
`
	vppctlPluginsWithoutLCP = ` Plugin path is: /usr/lib/x86_64-linux-gnu/vpp_plugins
     Plugin                                   Version                Description
  1. dhcp_plugin.so                           25.02-release          Dynamic Host Configuration Protocol (DHCP)
  2. wireguard_plugin.so                      25.02-release          Wireguard Security Tunnel
  3. nat_plugin.so                            25.02-release          Network Address Translation (NAT)
`
)

// fakeLCPProbe answers for the running VPP and counts the calls. A test can
// then assert that a skip opened NO probe, not only that it emitted nothing.
type fakeLCPProbe struct {
	out   string
	err   error
	calls int
}

func (f *fakeLCPProbe) run(context.Context) (string, error) {
	f.calls++
	return f.out, f.err
}

// installLCPPluginProbe points checkVPPLCPPlugin at fake for one test and
// restores the real probe afterwards. A nil fake installs a nil probe, which is
// what every platform VPP does not run on carries (defaultLCPPluginProbe).
func installLCPPluginProbe(t *testing.T, fake *fakeLCPProbe) {
	t.Helper()
	saved := lcpPluginProbe
	t.Cleanup(func() { lcpPluginProbe = saved })
	if fake == nil {
		lcpPluginProbe = nil
		return
	}
	lcpPluginProbe = fake.run
}

// lcpTree builds a config tree with a vpp/lcp container, optionally setting the
// `enabled` leaf. Passing "" omits the leaf, which is the YANG-default (on) case.
func lcpTree(enabled string) *config.Tree {
	tree := config.NewTree()
	vpp := config.NewTree()
	lcp := config.NewTree()
	if enabled != "" {
		lcp.Set("enabled", enabled)
	}
	vpp.SetContainer("lcp", lcp)
	tree.SetContainer("vpp", vpp)
	return tree
}

// TestLCPEnabledTreatsAbsentLeafAsOn pins the gate the whole check hangs on.
//
// VALIDATES: fixit-vpp-lcp-reachability AC-11 -- the check skips when LCP is
// off, and engages when it is on.
//
// PREVENTS: reading a missing `enabled` leaf as "off". The YANG default is on,
// so `vpp { lcp { } }` DOES load linux_cp_plugin.so via startup.conf
// (component/vpp/startupconf.go); treating it as off would silently skip the
// diagnostic for the very config shape most likely to be written by hand.
func TestLCPEnabledTreatsAbsentLeafAsOn(t *testing.T) {
	for _, tt := range []struct {
		name string
		tree *config.Tree
		want bool
	}{
		{"no vpp container at all", config.NewTree(), false},
		{"lcp present, enabled omitted (YANG default on)", lcpTree(""), true},
		{"lcp explicitly enabled", lcpTree("true"), true},
		{"lcp explicitly disabled", lcpTree("false"), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := lcpEnabled(tt.tree); got != tt.want {
				t.Fatalf("lcpEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCheckVPPLCPPluginSkipsWhenNotApplicable covers every no-probe path.
//
// VALIDATES: AC-11 -- with LCP disabled, or on a host VPP does not run on, the
// check returns nothing and opens NO probe.
//
// PREVENTS: an unconditional vppctl exec. Doctor runs on every `ze doctor`. An
// exec on a host that cannot run VPP warns about a dependency the operator never
// asked for. The call counter is what makes this assert the SKIP. A check that
// probed and then discarded the answer would also return nothing, and only
// fake.calls tells the two apart.
func TestCheckVPPLCPPluginSkipsWhenNotApplicable(t *testing.T) {
	t.Run("lcp disabled opens no probe", func(t *testing.T) {
		// The fake answers "plugin absent", so anything that probes reports.
		fake := &fakeLCPProbe{out: vppctlPluginsWithoutLCP}
		installLCPPluginProbe(t, fake)
		got := checkVPPLCPPlugin(diagnostic.DoctorCheckContext{Tree: lcpTree("false")})
		if len(got) != 0 {
			t.Fatalf("expected no diagnostics with lcp disabled, got %v", got)
		}
		if fake.calls != 0 {
			t.Fatalf("lcp is disabled; expected no probe, got %d calls", fake.calls)
		}
	})

	t.Run("nil tree opens no probe", func(t *testing.T) {
		fake := &fakeLCPProbe{out: vppctlPluginsWithoutLCP}
		installLCPPluginProbe(t, fake)
		if got := checkVPPLCPPlugin(diagnostic.DoctorCheckContext{}); len(got) != 0 {
			t.Fatalf("expected no diagnostics for a nil tree, got %v", got)
		}
		if fake.calls != 0 {
			t.Fatalf("there is no config to check; expected no probe, got %d calls", fake.calls)
		}
	})

	t.Run("platform without vpp yields nothing", func(t *testing.T) {
		// A nil probe is what defaultLCPPluginProbe returns off Linux. This
		// drives the non-Linux path on every platform, not only on the one the
		// test binary runs on.
		installLCPPluginProbe(t, nil)
		got := checkVPPLCPPlugin(diagnostic.DoctorCheckContext{Tree: lcpTree("true")})
		if len(got) != 0 {
			t.Fatalf("VPP is Linux-only; expected no diagnostics here, got %v", got)
		}
	})
}

// TestCheckVPPLCPPluginMissingIsAnError drives the case the check exists for.
//
// VALIDATES: AC-4 -- `lcp.enabled` is true and the running VPP does not load
// linux_cp_plugin.so, so `ze doctor` reports an actionable diagnostic BEFORE
// apply, naming the linux_cp API as unavailable on the running VPP.
//
// PREVENTS: the diagnostic going silent for a VPP that answered. The probe
// SUCCEEDED here, so the plugin set is known and the apply failure is certain.
// Severity is Error, not the Warning a failed probe earns, and `ze doctor` must
// then report the host as not ready (doctor.go, Run).
func TestCheckVPPLCPPluginMissingIsAnError(t *testing.T) {
	fake := &fakeLCPProbe{out: vppctlPluginsWithoutLCP}
	installLCPPluginProbe(t, fake)

	got := checkVPPLCPPlugin(diagnostic.DoctorCheckContext{Tree: lcpTree("true")})
	if len(got) != 1 {
		t.Fatalf("expected exactly one diagnostic for a VPP without linux_cp, got %v", got)
	}
	if got[0].Code != "doctor-vpp-lcp-plugin" {
		t.Fatalf("diagnostic code = %q, want doctor-vpp-lcp-plugin", got[0].Code)
	}
	if got[0].Severity != diagnostic.SeverityError {
		t.Fatalf("severity = %q, want %q: the probe answered, so the apply failure is certain", got[0].Severity, diagnostic.SeverityError)
	}
	if !strings.Contains(got[0].Message, lcpPluginSO) {
		t.Fatalf("message does not name %s, so it is not actionable: %q", lcpPluginSO, got[0].Message)
	}
	if !strings.Contains(got[0].Message, "linux_cp API") {
		t.Fatalf("message does not name the unavailable linux_cp API (AC-4): %q", got[0].Message)
	}
	if fake.calls != 1 {
		t.Fatalf("expected one probe of the running VPP, got %d", fake.calls)
	}
}

// TestCheckVPPLCPPluginPresentSilent pins the other half of the same answer.
//
// VALIDATES: AC-5 -- `lcp.enabled` is true and the plugin IS present, so there
// is no diagnostic.
//
// PREVENTS: a check that reports on every VPP. LCP is on by YANG default
// (ze-vpp-conf.yang), so a false positive here would fire on every correct VPP
// deployment and, at Error severity, would call a healthy host not ready.
func TestCheckVPPLCPPluginPresentSilent(t *testing.T) {
	fake := &fakeLCPProbe{out: vppctlPluginsWithLCP}
	installLCPPluginProbe(t, fake)

	if got := checkVPPLCPPlugin(diagnostic.DoctorCheckContext{Tree: lcpTree("true")}); len(got) != 0 {
		t.Fatalf("the running VPP loads %s; expected no diagnostics, got %v", lcpPluginSO, got)
	}
	if fake.calls != 1 {
		t.Fatalf("expected one probe of the running VPP, got %d", fake.calls)
	}
}

// TestCheckVPPLCPPluginProbeUnavailable covers the answer the check cannot give.
//
// VALIDATES: AC-6 -- VPP is unreachable at doctor time, so the check degrades
// to a warning ABOUT THE PROBE and never claims the plugin is missing.
//
// PREVENTS: reading a probe failure as evidence about the plugin set. vppctl
// exits non-zero for an absent binary, an absent socket and a wedged VPP alike;
// reporting an Error there would tell an operator to rebuild VPP when the real
// problem is that VPP is not running.
func TestCheckVPPLCPPluginProbeUnavailable(t *testing.T) {
	fake := &fakeLCPProbe{err: errors.New("exec: \"vppctl\": executable file not found in $PATH")}
	installLCPPluginProbe(t, fake)

	got := checkVPPLCPPlugin(diagnostic.DoctorCheckContext{Tree: lcpTree("")})
	if len(got) != 1 {
		t.Fatalf("expected exactly one diagnostic when the probe fails, got %v", got)
	}
	if got[0].Severity != diagnostic.SeverityWarning {
		t.Fatalf("severity = %q, want %q: a failed probe is not evidence about the plugin set", got[0].Severity, diagnostic.SeverityWarning)
	}
	if !strings.Contains(got[0].Message, "could not be probed") {
		t.Fatalf("message does not say the probe failed: %q", got[0].Message)
	}
	if strings.Contains(got[0].Message, "does not load") {
		t.Fatalf("message claims the plugin is missing on a failed probe: %q", got[0].Message)
	}
	if !strings.Contains(got[0].Message, "vppctl") {
		t.Fatalf("message drops the probe error, leaving the operator nothing to act on: %q", got[0].Message)
	}
}

// TestCheckVPPLCPPluginUntrustedOutputWarns covers the probe that exits zero
// and still says nothing.
//
// VALIDATES: AC-6 -- the check degrades to a warning ABOUT THE PROBE whenever
// VPP could not answer, and never claims the plugin is missing. A zero exit is
// not an answer on its own.
//
// PREVENTS: reading "linux_cp_plugin.so is not in this text" as proof the
// running VPP does not load it. vppctlShowPlugins (doctor.go) returns
// (string(out), nil) for ANY zero exit, empty and truncated stdout included, so
// without the vppctlPluginsHeader gate an operator with a working VPP gets an
// Error telling them to rebuild it.
func TestCheckVPPLCPPluginUntrustedOutputWarns(t *testing.T) {
	for _, tt := range []struct {
		name string
		out  string
	}{
		{"empty stdout", ""},
		{"whitespace only", "\n\n"},
		{"rows without the header line", "  1. dhcp_plugin.so                           25.02-release          DHCP\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeLCPProbe{out: tt.out}
			installLCPPluginProbe(t, fake)

			got := checkVPPLCPPlugin(diagnostic.DoctorCheckContext{Tree: lcpTree("true")})
			if len(got) != 1 {
				t.Fatalf("expected exactly one diagnostic for an untrustworthy probe answer, got %v", got)
			}
			if got[0].Severity != diagnostic.SeverityWarning {
				t.Fatalf("severity = %q, want %q: a zero exit with no plugin listing is not evidence about the plugin set", got[0].Severity, diagnostic.SeverityWarning)
			}
			if !strings.Contains(got[0].Message, "could not be probed") {
				t.Fatalf("message does not say the probe could not answer: %q", got[0].Message)
			}
			if strings.Contains(got[0].Message, "does not load") {
				t.Fatalf("message claims the plugin is missing on output that proves nothing: %q", got[0].Message)
			}
			if !strings.Contains(got[0].Message, vppctlPluginsHeader) {
				t.Fatalf("message does not name the missing %q line, leaving the operator nothing to act on: %q", vppctlPluginsHeader, got[0].Message)
			}
		})
	}
}

// TestVPPLCPPluginCheckIsRegistered proves the check reaches BOTH registries
// `ze doctor` reads: the DOCTOR-CHECK registry that decides whether the check
// runs at all, and the diagnostic-CODE registry `ze explain` resolves against.
//
// VALIDATES: AC-7 -- the check is registered and its diagnostic code is known.
//
// PREVENTS: the shape ai/rules/repo-maintenance.md exists to stop: a check that is
// implemented and unit-tested but never runs, because nothing registered it.
// The code half alone does NOT prevent it. codes.go registers the code
// independently of registerDoctorChecks (doctor.go), so deleting the
// vpp-lcp-plugin entry there leaves every Lookup assertion green while nothing
// ever calls the check. The registry entry is also FETCHED AND CALLED, because
// a name in the registry proves something registered, not that
// checkVPPLCPPlugin is what `ze doctor` runs.
func TestVPPLCPPluginCheckIsRegistered(t *testing.T) {
	// Built-in codes are registered by the binary entry point, not by init(),
	// so a package-level test has to ask for them.
	diagnostic.RegisterBuiltinCodes()

	meta := diagnostic.Lookup("doctor-vpp-lcp-plugin")
	if meta == nil {
		t.Fatal("diagnostic code doctor-vpp-lcp-plugin is not registered; ze explain would not resolve it")
	}
	if meta.Description == "" {
		t.Fatal("registered code has no description; ze explain would print nothing useful")
	}

	// DoctorPhasePostConfig is the phase registerDoctorChecks declares, and the
	// phase is what the runner iterates: a check registered under another one
	// never runs for a parsed config.
	var entry diagnostic.DoctorCheck
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhasePostConfig) {
		if check.Name == "vpp-lcp-plugin" {
			entry = check
			break
		}
	}
	if entry.Name == "" {
		t.Fatalf("doctor check vpp-lcp-plugin is not registered for DoctorPhasePostConfig; ze doctor would never run it. Registered names: %v", diagnostic.DoctorCheckNames())
	}
	if !slices.Contains(entry.Codes, "doctor-vpp-lcp-plugin") {
		t.Fatalf("registered check declares codes %v, so ze doctor cannot map its diagnostic back to the check", entry.Codes)
	}
	if entry.Check == nil {
		t.Fatal("registered check carries no check function; ze doctor would execute nothing")
	}

	fake := &fakeLCPProbe{out: vppctlPluginsWithoutLCP}
	installLCPPluginProbe(t, fake)
	got := entry.Check(diagnostic.DoctorCheckContext{Tree: lcpTree("true")})
	if len(got) != 1 {
		t.Fatalf("the registered entry does not dispatch to checkVPPLCPPlugin; expected one diagnostic, got %v", got)
	}
	if got[0].Code != "doctor-vpp-lcp-plugin" {
		t.Fatalf("the registered entry emitted code %q, so it is not checkVPPLCPPlugin", got[0].Code)
	}
	if got[0].Severity != diagnostic.SeverityError {
		t.Fatalf("the registered entry emitted severity %q for a VPP without linux_cp, want %q", got[0].Severity, diagnostic.SeverityError)
	}
}
