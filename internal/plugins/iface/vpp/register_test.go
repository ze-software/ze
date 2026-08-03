// VPP interface register: init() side-effects and backend conformance -- the
// "vpp" backend registered into iface's registry, the show-vpp wire methods
// registered with the plugin server, the registered doctor checks, the health
// check, and the compile-time iface.Backend interface satisfaction.
package ifacevpp

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/iface"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/health"
)

func TestShowVPP_RegisteredWireMethods(t *testing.T) {
	wanted := map[string]bool{
		"ze-show:vpp-trace-start": false,
		"ze-show:vpp-trace-show":  false,
		"ze-show:vpp-trace-clear": false,
		"ze-show:vpp-runtime":     false,
	}
	for _, r := range pluginserver.AllBuiltinRPCs() {
		if _, ok := wanted[r.WireMethod]; ok {
			require.NotNil(t, r.Handler, "%s handler must not be nil", r.WireMethod)
			wanted[r.WireMethod] = true
		}
	}
	for wm, seen := range wanted {
		require.True(t, seen, "%s not registered via pluginserver.RegisterRPCs", wm)
	}
}

func TestVPPHealthCheckSocketMissing(t *testing.T) {
	status, _ := checkVPPHealth()
	assert.Equal(t, health.StatusHealthy, status, "healthy when VPP socket does not exist")
}

func vppWireguardTree(backend string, pluginEnabled bool) *config.Tree {
	tree := config.NewTree()
	ifaceTree := tree.GetOrCreateContainer("interface")
	ifaceTree.Set("backend", backend)
	wg := config.NewTree()
	wg.Set("name", "wg0")
	ifaceTree.AddListEntry("wireguard", "wg0", wg)
	if pluginEnabled {
		plugins := tree.GetOrCreateContainer("vpp").GetOrCreateContainer("plugins")
		plugins.Set("wireguard", "true")
	}
	return tree
}

// TestDoctorWireguardPluginMissing verifies AC-8: a wireguard interface under
// backend vpp with the plugin toggle off emits doctor-vpp-wireguard.
func TestDoctorWireguardPluginMissing(t *testing.T) {
	diags := checkVPPWireguardPlugin(diagnostic.DoctorCheckContext{Tree: vppWireguardTree("vpp", false)})
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if diags[0].Code != "doctor-vpp-wireguard" {
		t.Errorf("code = %q, want doctor-vpp-wireguard", diags[0].Code)
	}
}

// TestDoctorWireguardPluginEnabled verifies no warning when the toggle is on.
func TestDoctorWireguardPluginEnabled(t *testing.T) {
	diags := checkVPPWireguardPlugin(diagnostic.DoctorCheckContext{Tree: vppWireguardTree("vpp", true)})
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics with plugin enabled, got %d", len(diags))
	}
}

// TestDoctorWireguardNetlinkBackend verifies the check is scoped to the vpp
// backend: a wireguard interface under netlink does not warn.
func TestDoctorWireguardNetlinkBackend(t *testing.T) {
	diags := checkVPPWireguardPlugin(diagnostic.DoctorCheckContext{Tree: vppWireguardTree("netlink", false)})
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics under netlink backend, got %d", len(diags))
	}
}

// TestDoctorWireguardNoInterface verifies no warning when no wireguard interface
// is configured, and a nil tree is tolerated.
func TestDoctorWireguardNoInterface(t *testing.T) {
	tree := config.NewTree()
	tree.GetOrCreateContainer("interface").Set("backend", "vpp")
	if diags := checkVPPWireguardPlugin(diagnostic.DoctorCheckContext{Tree: tree}); len(diags) != 0 {
		t.Errorf("expected 0 diagnostics without a wireguard interface, got %d", len(diags))
	}
	if diags := checkVPPWireguardPlugin(diagnostic.DoctorCheckContext{Tree: nil}); len(diags) != 0 {
		t.Errorf("expected 0 diagnostics for nil tree, got %d", len(diags))
	}
}

func vppLCPTree(bgp bool, netns string, lcpEnabled bool) *config.Tree {
	tree := config.NewTree()
	if bgp {
		tree.GetOrCreateContainer("bgp").Set("router-id", "10.0.0.1")
	}
	lcp := tree.GetOrCreateContainer("vpp").GetOrCreateContainer("lcp")
	if !lcpEnabled {
		lcp.Set("enabled", "false")
	}
	if netns != "" {
		lcp.Set("netns", netns)
	}
	return tree
}

// TestDoctorLCPNetnsIsolated verifies A-4/AC-8: BGP enabled + LCP in an isolated
// netns (the default "dataplane") emits doctor-vpp-lcp-netns.
func TestDoctorLCPNetnsIsolated(t *testing.T) {
	diags := checkVPPLCPNetns(diagnostic.DoctorCheckContext{Tree: vppLCPTree(true, "dataplane", true)})
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if diags[0].Code != "doctor-vpp-lcp-netns" {
		t.Errorf("code = %q, want doctor-vpp-lcp-netns", diags[0].Code)
	}
}

// TestDoctorLCPNetnsDefaultIsolated verifies the omitted-netns case uses the
// YANG default "dataplane" and still warns.
func TestDoctorLCPNetnsDefaultIsolated(t *testing.T) {
	diags := checkVPPLCPNetns(diagnostic.DoctorCheckContext{Tree: vppLCPTree(true, "", true)})
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic for default netns, got %d", len(diags))
	}
}

// TestDoctorLCPNetnsRootReachable verifies no warning when netns is host/root.
func TestDoctorLCPNetnsRootReachable(t *testing.T) {
	for _, ns := range []string{"host", "root"} {
		if diags := checkVPPLCPNetns(diagnostic.DoctorCheckContext{Tree: vppLCPTree(true, ns, true)}); len(diags) != 0 {
			t.Errorf("netns %q: expected 0 diagnostics, got %d", ns, len(diags))
		}
	}
}

// TestDoctorLCPNetnsNoBGP verifies no warning when BGP is not configured (the
// constraint only matters for the BGP-bind goal).
func TestDoctorLCPNetnsNoBGP(t *testing.T) {
	if diags := checkVPPLCPNetns(diagnostic.DoctorCheckContext{Tree: vppLCPTree(false, "dataplane", true)}); len(diags) != 0 {
		t.Errorf("expected 0 diagnostics without bgp, got %d", len(diags))
	}
}

// TestDoctorLCPNetnsDisabled verifies no warning when LCP is disabled.
func TestDoctorLCPNetnsDisabled(t *testing.T) {
	if diags := checkVPPLCPNetns(diagnostic.DoctorCheckContext{Tree: vppLCPTree(true, "dataplane", false)}); len(diags) != 0 {
		t.Errorf("expected 0 diagnostics with lcp disabled, got %d", len(diags))
	}
}

// lcpNetnsBannedAdvice are remediations the doctor-vpp-lcp-netns surfaces must
// never carry. They are patterns, not exact strings, so a reworded regression is
// caught too and so a later rewrite of the message is not blocked for cosmetic
// reasons.
//
// Why this advice is banned: to VPP, vpp.lcp.netns is a namespace NAME resolved
// to /var/run/netns/<name> (linux-cp lcp_set_default_ns formats the path and opens
// it; third_party/vpp-linux-cp/src/lcp.c:73-74), and an empty per-pair netns falls
// back to the global default (linux-cp lcp_itf_pair_create;
// third_party/vpp-linux-cp/src/lcp_interface.c:850-855)
// which ze itself sets from the same leaf (internal/component/vpp/startupconf.go:106).
// So "host" and "root" are not the host namespace: they ask VPP for namespaces of
// those literal names, which normally do not exist, and LCP pair creation fails.
// Telling an operator to set them breaks the dataplane the check is protecting.
var lcpNetnsBannedAdvice = []struct {
	name string
	re   *regexp.Regexp
}{
	// The directive shape, value-agnostic: no vpp.lcp.netns value is a fix while
	// BGP has no netns awareness (internal/core/network/network.go:167).
	{"directs the operator to set vpp.lcp.netns to a value", regexp.MustCompile(`(?i)\b(set|use|change|switch)\b[^.;]{0,40}vpp\.lcp\.netns\b[^.;]{0,24}\b(to|=)\b`)},
	// The offered pair. Mentioning host/root to say they do NOT work is fine, so
	// this targets the phrasing that offers them as alternatives.
	{"offers host/root as the fix", regexp.MustCompile(`(?i)\b(host\s+or\s+root|root\s+or\s+host)\b`)},
	{"describes host/root as a root-reachable namespace to pick", regexp.MustCompile(`(?i)root-reachable\s+namespace\s*\(`)},
}

// lcpNetnsRequiredAdvice are the properties every doctor-vpp-lcp-netns surface
// must keep: name the subject, and name the one remedy that works today.
// ai/rules/cli.md makes "what to do next" mandatory on doctor output,
// so removing the false advice without replacing it is not an option either.
var lcpNetnsRequiredAdvice = []struct {
	name string
	re   *regexp.Regexp
}{
	{"names the config leaf (the subject)", regexp.MustCompile(`vpp\.lcp\.netns`)},
	{"names the working remedy: run ze/BGP in that namespace", regexp.MustCompile(`(?i)\brun\s+(ze|bgp)\b[^.;]*\bnamespace\b`)},
}

func assertLCPNetnsAdvice(t *testing.T, surface, text string) {
	t.Helper()
	for _, b := range lcpNetnsBannedAdvice {
		if b.re.MatchString(text) {
			t.Errorf("%s %s\nremediation is false: host/root are namespace NAMES to VPP, so this advice breaks LCP pair creation\ngot: %q", surface, b.name, text)
		}
	}
	for _, r := range lcpNetnsRequiredAdvice {
		if !r.re.MatchString(text) {
			t.Errorf("%s does not satisfy %q (ai/rules/cli.md: what / why / next)\ngot: %q", surface, r.name, text)
		}
	}
}

// TestDoctorLCPNetnsRemediation verifies the doctor-vpp-lcp-netns message tells
// the operator something that works. It drives the real check function, so it
// asserts the text an operator actually sees from `ze doctor`.
// VALIDATES: AC-1 -- the message does not direct the operator to set
// vpp.lcp.netns to any value, and does not offer host/root as the fix.
// VALIDATES: AC-2 -- the message names the remedy that works today (run ze in
// the configured namespace) and quotes the offending value.
// PREVENTS: ze doctor recommending a configuration that breaks the VPP dataplane.
func TestDoctorLCPNetnsRemediation(t *testing.T) {
	diags := checkVPPLCPNetns(diagnostic.DoctorCheckContext{Tree: vppLCPTree(true, "dataplane", true)})
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	msg := diags[0].Message
	assertLCPNetnsAdvice(t, "doctor message", msg)
	// The evidence leg: the offending value must be visible and quoted, so an
	// empty or look-alike netns name is not mistaken for a formatting artifact.
	if !strings.Contains(msg, `"dataplane"`) {
		t.Errorf("doctor message does not quote the offending netns value\ngot: %q", msg)
	}
}

// TestDoctorLCPNetnsCodeDescription verifies the registered diagnostic code's
// description carries the same true remedy. This is what `ze explain
// doctor-vpp-lcp-netns` prints, so it is a second operator-facing surface and
// it repeated the same false advice.
// VALIDATES: AC-3 -- the registry row names no vpp.lcp.netns value as the fix
// and names the working remedy instead.
// PREVENTS: fixing the doctor message while `ze explain` keeps handing out the
// advice that breaks the dataplane.
func TestDoctorLCPNetnsCodeDescription(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()
	meta := diagnostic.Lookup("doctor-vpp-lcp-netns")
	if meta == nil {
		t.Fatal("doctor-vpp-lcp-netns is not registered in internal/core/diagnostic/codes.go")
	}
	assertLCPNetnsAdvice(t, "ze explain description", meta.Description)
}

// TestVPPBackendImplementsInterface verifies compile-time interface compliance.
// VALIDATES: AC-1 -- Backend "vpp" implements all methods
// PREVENTS: missing method causing compile error at integration time.
func TestVPPBackendImplementsInterface(t *testing.T) {
	// Compile-time check: vppBackendImpl implements iface.Backend.
	var _ iface.Backend = (*vppBackendImpl)(nil)
}

// sentinelBackend is a non-nil factory used only as the duplicate-registration
// probe in TestVPPBackendRegistered; it must never actually run.
func sentinelBackend() (iface.Backend, error) {
	return nil, errors.New("ifacevpp test sentinel: real backend was NOT registered at init")
}

// TestVPPBackendRegistered verifies init() (register.go) wired the "vpp" iface
// backend into iface's registry: RegisterBackend rejects duplicates, so a probe
// re-registration must fail. If it succeeds, the real backend was not registered.
// PREVENTS: a silently-unregistered VPP backend the composition root cannot load.
func TestVPPBackendRegistered(t *testing.T) {
	if err := iface.RegisterBackend("vpp", sentinelBackend); err == nil {
		t.Fatal("expected duplicate-registration error, got nil (backend not registered at init?)")
	}
}
