// VPP interface register: init() side-effects and backend conformance -- the
// "vpp" backend registered into iface's registry, the show-vpp wire methods
// registered with the plugin server, the registered doctor checks, the health
// check, and the compile-time iface.Backend interface satisfaction.
package ifacevpp

import (
	"errors"
	"os"
	"path/filepath"
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

// vppLCPTreeNetnsLeaf builds a tree whose vpp/lcp/netns leaf is SET, including
// to the empty string. vppLCPTree cannot express that: there "" means the leaf
// is omitted, so the YANG default "dataplane" applies instead.
func vppLCPTreeNetnsLeaf(bgp bool, netns string) *config.Tree {
	tree := vppLCPTree(bgp, "", true)
	tree.GetContainerPath("vpp/lcp").Set("netns", netns)
	return tree
}

// setLCPNetnsDir points the named-namespace probe at dir for one test, and
// restores it afterwards.
//
// Every checkVPPLCPNetns test calls this. Left alone, the probe reads the real
// /var/run/netns on a Linux host, so whether a test saw a diagnostic would
// depend on which namespaces that host happens to have. Passing "" is the
// non-Linux production value and disables the host leg, which is how a test
// isolates the config leg.
func setLCPNetnsDir(t *testing.T, dir string) {
	t.Helper()
	prev := lcpNetnsDir
	lcpNetnsDir = dir
	t.Cleanup(func() { lcpNetnsDir = prev })
}

// lcpNetnsDiagWith returns the one diagnostic whose message contains want. The
// check emits a config-leg and a host-leg diagnostic independently, so a test
// asserting on one of them must name which.
func lcpNetnsDiagWith(t *testing.T, diags []diagnostic.Diagnostic, want string) diagnostic.Diagnostic {
	t.Helper()
	var found []diagnostic.Diagnostic
	for i := range diags {
		if strings.Contains(diags[i].Message, want) {
			found = append(found, diags[i])
		}
	}
	if len(found) != 1 {
		t.Fatalf("expected exactly 1 diagnostic containing %q, got %d of %d: %+v", want, len(found), len(diags), diags)
	}
	if found[0].Code != "doctor-vpp-lcp-netns" {
		t.Errorf("code = %q, want doctor-vpp-lcp-netns", found[0].Code)
	}
	if found[0].Severity != diagnostic.SeverityWarning {
		t.Errorf("severity = %v, want warning", found[0].Severity)
	}
	return found[0]
}

// TestDoctorLCPNetnsIsolated verifies A-4/AC-8: BGP enabled + LCP in an isolated
// netns (the default "dataplane") emits doctor-vpp-lcp-netns.
func TestDoctorLCPNetnsIsolated(t *testing.T) {
	setLCPNetnsDir(t, "")
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
	setLCPNetnsDir(t, "")
	diags := checkVPPLCPNetns(diagnostic.DoctorCheckContext{Tree: vppLCPTree(true, "", true)})
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic for default netns, got %d", len(diags))
	}
}

// TestDoctorLCPNetnsRootMarkerWarns verifies the check speaks for vpp.lcp.netns
// host and root, which it passed in SILENCE until 2026-08-07.
//
// This test replaces TestDoctorLCPNetnsRootReachable, which asserted 0
// diagnostics for those two values. That assertion pinned the defect: ze's
// marker set is ze's alone, and lcp_set_default_ns
// (third_party/vpp-linux-cp/src/lcp.c) formats ANY non-empty leaf into
// /var/run/netns/<name> and opens it. ze writes the leaf as VPP's global default
// (internal/component/vpp/startupconf.go) and lcpPairNetns maps a marker to an
// empty per-pair field, which lcp_itf_pair_create resolves back to that same
// global default (third_party/vpp-linux-cp/src/lcp_interface.c). So netns=host
// asks VPP for a namespace literally called host, tap_create_if fails when it is
// absent, and the whole config apply fails at the binapi layer.
// VALIDATES: plan/deferrals/fixit-vpp-lcp-netns-remediation.md R-5.
// PREVENTS: an operator setting host or root, believing it means the namespace
// ze runs in, and getting a broken dataplane with no diagnostic.
func TestDoctorLCPNetnsRootMarkerWarns(t *testing.T) {
	setLCPNetnsDir(t, "")
	for _, ns := range []string{"host", "root"} {
		// No bgp stanza: the marker is wrong even where nothing binds, because
		// LCP pair creation is what fails.
		diags := checkVPPLCPNetns(diagnostic.DoctorCheckContext{Tree: vppLCPTree(false, ns, true)})
		d := lcpNetnsDiagWith(t, diags, "is not the host network namespace")
		if !strings.Contains(d.Message, `"`+ns+`"`) {
			t.Errorf("netns %q: message does not quote the offending value\ngot: %q", ns, d.Message)
		}
		if !strings.Contains(d.Message, "/var/run/netns/"+ns) {
			t.Errorf("netns %q: message does not name the path VPP opens\ngot: %q", ns, d.Message)
		}
		assertLCPNetnsAdvice(t, "doctor message for netns "+ns, d.Message)
	}
}

// TestDoctorLCPNetnsMarkerWithLiveProbe drives the combination every other marker
// test hides: a marker AND a live named-namespace probe. Those tests all call
// setLCPNetnsDir(t, ""), which switches the host leg off, so the pairing was never
// exercised -- and on Linux it is the production one.
//
// With both legs running the check emitted TWO diagnostics that contradicted each
// other: "host is not the host network namespace" followed by "create the
// namespace with ip netns add host". The second entrenches exactly the belief the
// first corrects, and neither offered the empty leaf.
//
// Both rows matter. Absent proves the second diagnostic is gone; present proves it
// was not suppressed by the namespace happening to exist, which is the reading
// that would make the marker case look fixed for the wrong reason.
// VALIDATES: a marker yields ONE diagnostic whose remedy is the empty leaf.
// PREVENTS: doctor telling the operator to build the namespace whose name is the
// misunderstanding being reported.
func TestDoctorLCPNetnsMarkerWithLiveProbe(t *testing.T) {
	for _, tc := range []struct {
		name   string
		create bool
	}{
		{"namespace absent", false},
		{"namespace present", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.create {
				if err := os.WriteFile(filepath.Join(dir, "host"), nil, 0o600); err != nil {
					t.Fatalf("create namespace entry: %v", err)
				}
			}
			setLCPNetnsDir(t, dir)
			diags := checkVPPLCPNetns(diagnostic.DoctorCheckContext{Tree: vppLCPTree(true, "host", true)})
			if len(diags) != 1 {
				t.Fatalf("expected exactly 1 diagnostic for a marker with a live probe, got %d: %+v", len(diags), diags)
			}
			d := lcpNetnsDiagWith(t, diags, "is not the host network namespace")
			if strings.Contains(d.Message, "ip netns add") {
				t.Errorf("the marker diagnostic tells the operator to create the namespace it just called wrong\ngot: %q", d.Message)
			}
			assertLCPNetnsAdvice(t, "marker diagnostic with a live probe", d.Message)
		})
	}
}

// TestDoctorLCPNetnsEmptyLeafSilent verifies the one value that needs no
// warning. An empty leaf clears VPP's global default (lcp_set_default_ns,
// third_party/vpp-linux-cp/src/lcp.c), so lcp_itf_pair_create leaves the TAP in
// VPP's own namespace and ze can bind on it. The probe is LIVE here: an empty
// leaf must not be looked up as a namespace name either.
func TestDoctorLCPNetnsEmptyLeafSilent(t *testing.T) {
	setLCPNetnsDir(t, t.TempDir())
	if diags := checkVPPLCPNetns(diagnostic.DoctorCheckContext{Tree: vppLCPTreeNetnsLeaf(true, "")}); len(diags) != 0 {
		t.Errorf("expected 0 diagnostics for an empty netns leaf, got %d: %+v", len(diags), diags)
	}
}

// TestDoctorLCPNetnsNoBGP verifies the isolated-namespace warning stays gated on
// bgp: there the only harm is the bind. The host leg is off, so this covers the
// config leg alone.
func TestDoctorLCPNetnsNoBGP(t *testing.T) {
	setLCPNetnsDir(t, "")
	if diags := checkVPPLCPNetns(diagnostic.DoctorCheckContext{Tree: vppLCPTree(false, "dataplane", true)}); len(diags) != 0 {
		t.Errorf("expected 0 diagnostics without bgp, got %d", len(diags))
	}
}

// TestDoctorLCPNetnsDisabled verifies no warning when LCP is disabled. The probe
// is live: a disabled LCP creates no TAP, so its netns is not looked up either.
func TestDoctorLCPNetnsDisabled(t *testing.T) {
	setLCPNetnsDir(t, t.TempDir())
	if diags := checkVPPLCPNetns(diagnostic.DoctorCheckContext{Tree: vppLCPTree(true, "dataplane", false)}); len(diags) != 0 {
		t.Errorf("expected 0 diagnostics with lcp disabled, got %d", len(diags))
	}
}

// TestDoctorLCPNetnsAbsentFromHost verifies the host leg: a netns name with no
// entry under the namespace directory is reported, because tap_create_if opens
// that path and fails the whole apply when it is missing
// (third_party/vpp-linux-cp/src/lcp_interface.c).
// PREVENTS: doctor passing a config whose apply cannot succeed.
func TestDoctorLCPNetnsAbsentFromHost(t *testing.T) {
	setLCPNetnsDir(t, t.TempDir())
	diags := checkVPPLCPNetns(diagnostic.DoctorCheckContext{Tree: vppLCPTree(true, "dataplane", true)})
	d := lcpNetnsDiagWith(t, diags, "absent from")
	if !strings.Contains(d.Message, `"dataplane"`) {
		t.Errorf("message does not quote the offending value\ngot: %q", d.Message)
	}
	assertLCPNetnsAdvice(t, "doctor absent-namespace message", d.Message)
	// The config leg still speaks for the same config, and says something else.
	lcpNetnsDiagWith(t, diags, "is not root-reachable")
}

// TestDoctorLCPNetnsPresentOnHost verifies the host leg stays quiet when the
// namespace is there. Without this row the absent-namespace assertion would pass
// against a check that warned unconditionally.
func TestDoctorLCPNetnsPresentOnHost(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "dataplane"), nil, 0o600); err != nil {
		t.Fatalf("create namespace entry: %v", err)
	}
	setLCPNetnsDir(t, dir)
	diags := checkVPPLCPNetns(diagnostic.DoctorCheckContext{Tree: vppLCPTree(true, "dataplane", true)})
	for _, d := range diags {
		if strings.Contains(d.Message, "absent from") {
			t.Errorf("namespace exists but the check reports it absent\ngot: %q", d.Message)
		}
	}
	// The BGP-bind warning is unaffected by the namespace being present.
	lcpNetnsDiagWith(t, diags, "is not root-reachable")
}

// TestDoctorLCPNetnsProbeError verifies a probe that cannot answer says so
// instead of reporting absence. The namespace directory is a regular file here,
// so the stat fails with ENOTDIR, which is not fs.ErrNotExist.
// PREVENTS: an unreadable /var/run/netns turning into "the namespace does not
// exist", which would send the operator to create one that is already there.
func TestDoctorLCPNetnsProbeError(t *testing.T) {
	notADir := filepath.Join(t.TempDir(), "netns-file")
	if err := os.WriteFile(notADir, nil, 0o600); err != nil {
		t.Fatalf("create file: %v", err)
	}
	setLCPNetnsDir(t, notADir)
	diags := checkVPPLCPNetns(diagnostic.DoctorCheckContext{Tree: vppLCPTree(true, "dataplane", true)})
	d := lcpNetnsDiagWith(t, diags, "could not be checked")
	if strings.Contains(d.Message, "absent from") {
		t.Errorf("a failed probe is reported as absence\ngot: %q", d.Message)
	}
	assertLCPNetnsAdvice(t, "doctor probe-failure message", d.Message)
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
// must keep: name the subject, and name the remedies that work today.
// ai/rules/cli.md makes "what to do next" mandatory on doctor output,
// so removing the false advice without replacing it is not an option either.
var lcpNetnsRequiredAdvice = []struct {
	name string
	re   *regexp.Regexp
}{
	{"names the config leaf (the subject)", regexp.MustCompile(`vpp\.lcp\.netns`)},
	{"names the working remedy: run ze/BGP in that namespace", regexp.MustCompile(`(?i)\brun\s+(ze|bgp)\b[^.;]*\bnamespace\b`)},
	// The empty leaf is the ONE value that leaves the TAPs where ze runs:
	// lcp_set_default_ns clears the global default for it and
	// lcp_itf_pair_create then opens no namespace at all
	// (third_party/vpp-linux-cp/src/lcp.c, src/lcp_interface.c). Every surface
	// that reports this code omitted it, and the marker message was the worst
	// case: it told the operator the marker was not the host namespace and then
	// offered nothing but running ze inside a namespace of that literal name.
	// The pattern demands the DIRECTIVE, not the word: "is not empty" is a
	// description and must not satisfy it.
	{"names the empty-leaf remedy", regexp.MustCompile(`(?i)\b(leave|leaving|omit|omitting)\b[^.;]{0,40}vpp\.lcp\.netns[^.;]{0,24}\bempty\b`)},
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
	setLCPNetnsDir(t, "")
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
