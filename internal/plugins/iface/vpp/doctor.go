// Design: ai/rules/repo-maintenance.md -- self-contained doctor checks owned by
// the plugin that owns the runtime dependency.
// Overview: register.go -- init() registers these checks.
//
// The vpp interface backend adds runtime dependencies that ze doctor surfaces,
// so a misconfiguration is caught before apply rather than as a raw VPP API
// error:
//
//   - wireguard: a wireguard interface under backend vpp needs the VPP
//     wireguard plugin. The plugin loads only when vpp.plugins.wireguard is
//     enabled (plugin default { disable }); doctor warns when the interface is
//     configured but the toggle is off.
//   - lcp netns: LCP TAPs land in vpp.lcp.netns, and VPP resolves that leaf as a
//     namespace NAME under /var/run/netns/. doctor warns when the leaf names a
//     namespace ze does not run in (BGP cannot bind there, see A-4) and when that
//     namespace is absent from this host (LCP pair creation then fails at apply).
//     One of ze's own host markers is a single warning instead: it is wrong for
//     one reason, the remedy is the empty leaf, and the host probe is skipped
//     because the namespace being there would not make the markers work.
//   - lcp plugin: enabling vpp.lcp makes startup.conf load linux_cp_plugin.so
//     (component/vpp/startupconf.go). A VPP built without it accepts the config
//     and then fails the whole apply at the binapi layer with a raw VPP error,
//     so doctor probes the RUNNING VPP and says so first.

package ifacevpp

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// registerDoctorChecks installs the vpp iface backend's doctor checks. Called
// from init() in register.go so the checks travel with the plugin.
func registerDoctorChecks() error {
	checks := []diagnostic.DoctorCheck{
		{
			Name:         "vpp-wireguard-plugin",
			Phase:        diagnostic.DoctorPhasePostConfig,
			Order:        740,
			Component:    "vpp",
			Dependencies: []string{"vpp-wireguard-plugin"},
			Platforms:    []string{diagnostic.DoctorPlatformAny},
			Codes:        []string{"doctor-vpp-wireguard"},
			Check:        checkVPPWireguardPlugin,
		},
		{
			Name:         "vpp-lcp-plugin",
			Phase:        diagnostic.DoctorPhasePostConfig,
			Order:        742,
			Component:    "vpp",
			Dependencies: []string{"vpp-lcp"},
			Platforms:    []string{diagnostic.DoctorPlatformAny},
			Codes:        []string{"doctor-vpp-lcp-plugin"},
			Check:        checkVPPLCPPlugin,
		},
		{
			Name:         "vpp-lcp-netns",
			Phase:        diagnostic.DoctorPhasePostConfig,
			Order:        741,
			Component:    "vpp",
			Dependencies: []string{"vpp-lcp"},
			Platforms:    []string{diagnostic.DoctorPlatformAny},
			Codes:        []string{"doctor-vpp-lcp-netns"},
			Check:        checkVPPLCPNetns,
		},
	}
	for i := range checks {
		if err := diagnostic.RegisterDoctorCheck(checks[i]); err != nil {
			return err
		}
	}
	return nil
}

// checkVPPWireguardPlugin warns when a wireguard interface is configured under
// the vpp backend but vpp.plugins.wireguard is not enabled: without the toggle
// startup.conf leaves wireguard_plugin.so disabled and the interface fails to
// program at apply.
func checkVPPWireguardPlugin(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	ifaceTree := tree.GetContainer("interface")
	if ifaceTree == nil {
		return nil
	}
	if backend, _ := ifaceTree.Get("backend"); backend != "vpp" {
		return nil
	}
	if len(ifaceTree.GetList("wireguard")) == 0 {
		return nil
	}
	if wireguardPluginEnabled(tree) {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     "doctor-vpp-wireguard",
		Severity: diagnostic.SeverityError,
		Message:  "wireguard interface configured under backend vpp but vpp.plugins.wireguard is not enabled; wireguard_plugin.so will not load and the interface will fail at apply",
	}}
}

// wireguardPluginEnabled reports whether vpp.plugins.wireguard is set true.
func wireguardPluginEnabled(tree *config.Tree) bool {
	plugins := tree.GetContainerPath("vpp/plugins")
	if plugins == nil {
		return false
	}
	v, _ := plugins.Get("wireguard")
	return v == "true"
}

// checkVPPLCPNetns warns when the LCP TAPs will not land where ze can use them.
// A non-empty vpp.lcp.netns takes one of two routes:
//
//   - a MARKER ("host", "root") gets ONE diagnostic. The operator wrote a name
//     believing it meant the host namespace; VPP has no such name, and the one
//     remedy is the empty leaf. Whether /var/run/netns/<marker> happens to exist
//     changes nothing about that answer, so the host probe is not consulted:
//     present, the TAPs land there and ze still cannot bind; absent, LCP pair
//     creation fails. A second diagnostic telling the operator to CREATE the
//     namespace would entrench the belief the first one corrects.
//   - any other NAME gets up to two, because they are separate facts with
//     separate consequences: what the CONFIG proves on every platform (the leaf
//     names a namespace ze does not run in, so BGP cannot bind on an
//     LCP-shadowed interface, A-4), and what the HOST proves where named
//     namespaces exist (the namespace VPP will open is absent, so LCP pair
//     creation fails at apply). A host that has the namespace still has the
//     BGP-bind problem, which is why the second does not replace the first.
func checkVPPLCPNetns(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	lcp := tree.GetContainerPath("vpp/lcp")
	if lcp == nil || !lcpEnabled(tree) {
		return nil
	}
	// Default netns is "dataplane" when the leaf is omitted (YANG default).
	netns := "dataplane"
	if v, ok := lcp.Get("netns"); ok {
		netns = v
	}
	// An EMPTY leaf is the one value that leaves the TAPs where ze runs, and it
	// is the only value this check passes in silence. lcp_set_default_ns clears
	// the global default for it (third_party/vpp-linux-cp/src/lcp.c), so
	// lcp_itf_pair_create falls back to no namespace at all and creates the TAP
	// in VPP's own namespace (third_party/vpp-linux-cp/src/lcp_interface.c).
	// Every other value, ze's own markers included, is a namespace NAME.
	if netns == "" {
		return nil
	}
	if lcpNetnsIsRootMarker(netns) {
		return []diagnostic.Diagnostic{lcpNetnsMarkerDiagnostic(netns)}
	}
	var diags []diagnostic.Diagnostic
	if d, ok := lcpNetnsConfigDiagnostic(tree, netns); ok {
		diags = append(diags, d)
	}
	if d, ok := lcpNetnsHostDiagnostic(netns); ok {
		diags = append(diags, d)
	}
	return diags
}

// lcpNetnsMarkerDiagnostic reports a vpp.lcp.netns that carries one of ze's own
// host markers. It is emitted whether or not BGP is configured: the value is
// wrong even where nothing binds, because LCP pair creation is what fails.
//
// The remediation names the EMPTY leaf first. It is the only value that puts the
// TAPs in the namespace ze runs in, and no marker does that.
func lcpNetnsMarkerDiagnostic(netns string) diagnostic.Diagnostic {
	var tb textbuf.Buffer
	msg := tb.Str("vpp.lcp.netns=").Quoted(netns).
		Str(" is not the host network namespace. VPP resolves the leaf as a namespace name under ").Str(lcpNetnsPathPrefix).
		Str(", so the LCP TAPs land in ").Str(lcpNetnsPathPrefix).Str(netns).
		Str(" and ze cannot bind on them from its own namespace. Leave vpp.lcp.netns empty to keep the TAPs in VPP's own network namespace, where ze runs; the other remedy is to run ze in the ").Quoted(netns).
		Str(" namespace so BGP binds where the TAPs are, or see ze explain doctor-vpp-lcp-netns").String()
	return diagnostic.Diagnostic{
		Code:     "doctor-vpp-lcp-netns",
		Severity: diagnostic.SeverityWarning,
		Message:  msg,
	}
}

// lcpNetnsConfigDiagnostic reports what the config alone proves about a
// vpp.lcp.netns that names an ordinary namespace. It reads no host state, so it
// speaks on every platform, and it is gated on BGP because there the only harm
// is the bind (A-4). Markers never reach it (lcpNetnsMarkerDiagnostic).
func lcpNetnsConfigDiagnostic(tree *config.Tree, netns string) (diagnostic.Diagnostic, bool) {
	if tree.GetContainer("bgp") == nil {
		return diagnostic.Diagnostic{}, false
	}
	// Remediation: no NAME puts the TAPs where a root-netns ze can bind, because
	// VPP resolves the leaf as a namespace name under /var/run/netns/ (linux-cp
	// lcp_set_default_ns, third_party/vpp-linux-cp/src/lcp.c) and ze has no
	// netns-aware listener (RealListenerFactory.Listen,
	// internal/core/network/network.go). The empty leaf is not a name, and it is
	// the one value that does: lcp_itf_pair_create then creates the TAP in VPP's
	// own namespace. Naming another value here is what the previous message did,
	// and following it fails LCP pair creation. The long form lives in the
	// registered code's description (internal/core/diagnostic/codes.go), which ze
	// explain prints.
	var tb textbuf.Buffer
	msg := tb.Str("bgp is enabled and vpp.lcp.netns=").Quoted(netns).
		Str(" is not root-reachable; BGP cannot bind on an LCP-shadowed interface in a separate namespace. VPP resolves the leaf as a namespace name under /var/run/netns/, so no name puts the TAPs where a root-netns ze binds. Leave vpp.lcp.netns empty to keep the TAPs in VPP's own network namespace, or run ze in the ").Quoted(netns).
		Str(" namespace so BGP binds where the TAPs are; see ze explain doctor-vpp-lcp-netns").String()
	return diagnostic.Diagnostic{
		Code:     "doctor-vpp-lcp-netns",
		Severity: diagnostic.SeverityWarning,
		Message:  msg,
	}, true
}

// lcpNetnsHostDiagnostic reports what THIS host proves about a non-empty
// vpp.lcp.netns: whether the namespace VPP will open is there. It says nothing
// where the host keeps no named namespaces (lcpNetnsDir empty), because absence
// of the directory is not evidence about the target machine.
//
// A probe that cannot answer gets its own diagnostic rather than being folded
// into "the namespace is absent": a stat error is not evidence of absence, and
// the two ask different things of the operator.
func lcpNetnsHostDiagnostic(netns string) (diagnostic.Diagnostic, bool) {
	if lcpNetnsDir == "" {
		return diagnostic.Diagnostic{}, false
	}
	resolves, err := lcpNetnsResolves(netns)
	var tb textbuf.Buffer
	switch {
	case err != nil:
		msg := tb.Str("vpp.lcp.netns=").Quoted(netns).Str(" could not be checked against ").Str(lcpNetnsPathPrefix).Str(": ").Err(err).
			Str(". VPP opens that path for the LCP TAPs, so a missing namespace fails LCP pair creation at apply. Leave vpp.lcp.netns empty to keep the TAPs in VPP's own network namespace, or run ze in the ").Quoted(netns).
			Str(" namespace so BGP binds where the TAPs are; see ze explain doctor-vpp-lcp-netns").String()
		return diagnostic.Diagnostic{
			Code:     "doctor-vpp-lcp-netns",
			Severity: diagnostic.SeverityWarning,
			Message:  msg,
		}, true
	case !resolves:
		msg := tb.Str("vpp.lcp.netns=").Quoted(netns).Str(" names a network namespace that is absent from ").Str(lcpNetnsPathPrefix).
			Str(" on this host. VPP opens that path for the LCP TAPs, so LCP pair creation fails at apply with a raw VPP error. Leave vpp.lcp.netns empty to keep the TAPs in VPP's own network namespace, or create the namespace with ip netns add ").Str(netns).
			Str(" and run ze in the ").Quoted(netns).
			Str(" namespace so BGP binds where the TAPs are").String()
		return diagnostic.Diagnostic{
			Code:     "doctor-vpp-lcp-netns",
			Severity: diagnostic.SeverityWarning,
			Message:  msg,
		}, true
	}
	return diagnostic.Diagnostic{}, false
}

// lcpNetnsPathPrefix is where VPP's linux-cp resolves vpp.lcp.netns:
// lcp_set_default_ns formats the leaf into /var/run/netns/<name> and opens it
// (third_party/vpp-linux-cp/src/lcp.c), and lcp_itf_pair_create hands a per-pair
// name to the same convention through tap_create_if
// (third_party/vpp-linux-cp/src/lcp_interface.c). VPP hardcodes the path, so ze
// states it as a fact about VPP rather than as a location ze chooses.
const lcpNetnsPathPrefix = "/var/run/netns/"

// lcpNetnsDir is the directory the doctor check PROBES for a named network
// namespace. It is lcpNetnsPathPrefix on Linux and EMPTY elsewhere, where
// nothing keeps named namespaces and the absence of the directory says nothing
// about the machine that will run VPP.
//
// A var rather than a const so the unit tests can point the probe at a temporary
// directory and drive both outcomes on any platform. Every test that calls
// checkVPPLCPNetns must set it: left alone on Linux it reads the real
// /var/run/netns, and the result would depend on which namespaces the host
// happens to have.
var lcpNetnsDir = defaultLCPNetnsDir()

// goosLinux is the runtime.GOOS value VPP runs under. Both host-reading checks
// in this file gate on it, because neither a VPP API socket nor a named network
// namespace exists anywhere else.
const goosLinux = "linux"

// defaultLCPNetnsDir returns the namespace directory to probe on this platform.
func defaultLCPNetnsDir() string {
	if runtime.GOOS != goosLinux {
		return ""
	}
	return lcpNetnsPathPrefix
}

// lcpNetnsResolves reports whether name is a named network namespace present on
// this host. A non-nil error means the probe could not answer, which is not the
// same as absence and is never reported as absence.
func lcpNetnsResolves(name string) (bool, error) {
	if _, err := os.Stat(filepath.Join(lcpNetnsDir, name)); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// lcpNetnsIsRootMarker reports whether an LCP netns name is one ze TREATS as the
// host (root) network namespace, where its BGP listener runs by default.
//
// This is a statement about ze's own marker set ("", host, root), not about VPP,
// and the name says marker for that reason: VPP has no special namespace names.
// lcp_set_default_ns formats the leaf into the path /var/run/netns/<name> and
// opens it (third_party/vpp-linux-cp/src/lcp.c), and lcp_itf_pair_create resolves
// an EMPTY per-pair netns to that GLOBAL default
// (third_party/vpp-linux-cp/src/lcp_interface.c), which ze always writes from
// this same leaf when LCP is enabled (internal/component/vpp/startupconf.go).
// So netns=host makes VPP open /var/run/netns/host rather than stay in its own
// namespace, and these markers are not the escape hatch they look like. Verified
// in VPP's C sources, not in the generated binapi stub, which documents only that
// the field exists; recorded as A-13 in plan/spec-bgp-netns.md.
//
// Two callers, two questions. lcpPairNetns (lcp.go) asks this one, "is this one
// of ze's markers", to decide whether to send an empty per-pair netns field.
// checkVPPLCPNetns asks whether the TAPs will be usable, which a marker never
// makes true, so it warns on a marker instead of passing it.
func lcpNetnsIsRootMarker(netns string) bool {
	switch netns {
	case "", "host", "root":
		return true
	default:
		return false
	}
}

// lcpPluginSO is the VPP plugin startup.conf enables when vpp.lcp is on
// (component/vpp/startupconf.go). Matched as a substring of `vppctl show
// plugins` output rather than probed on the filesystem: what matters is whether
// the RUNNING VPP loaded it, not whether some copy exists on disk.
const lcpPluginSO = "linux_cp_plugin.so"

// vppctlPluginsHeader is the first line `vppctl show plugins` prints, ahead of
// the column titles and the numbered plugin rows. The check requires it before
// it reads an ABSENT plugin name as evidence: vppctl exits zero for output that
// is empty or truncated, and a name missing from nothing says nothing about
// which plugins the running VPP loaded.
const vppctlPluginsHeader = "Plugin path is:"

// vppProbeTimeout bounds the vppctl probe. Doctor runs before apply and must
// stay responsive on a host where VPP is wedged rather than merely absent.
const vppProbeTimeout = 3 * time.Second

// lcpPluginProbe reports which plugins the RUNNING VPP loaded. It carries the
// raw text of `vppctl show plugins`. It is NIL on every platform VPP does not
// run on. "No probe is opened there" is then a property of the value, not of a
// branch no test can reach.
//
// A var rather than a plain function, for the reason lcpNetnsDir is one: the
// unit tests drive all three answers on any platform. The three are plugin
// listed, plugin absent, and probe failed. Every test that calls
// checkVPPLCPPlugin MUST set it and MUST restore it. Left alone on Linux it
// execs the real vppctl, and the answer then depends on the host.
var lcpPluginProbe = defaultLCPPluginProbe()

// defaultLCPPluginProbe returns the plugin probe for this platform, and nil
// where VPP does not run.
func defaultLCPPluginProbe() func(context.Context) (string, error) {
	if runtime.GOOS != goosLinux {
		return nil
	}
	return vppctlShowPlugins
}

// vppctlShowPlugins asks the running VPP which plugins it loaded. It goes
// through the CLI socket vppctl dials. It reports the same error for an absent
// vppctl binary, an absent socket and a wedged VPP. A caller MUST NOT read that
// error as evidence about the plugin set.
func vppctlShowPlugins(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "vppctl", "show", "plugins").Output() //nolint:gosec // fixed command, no user input
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// lcpEnabled reports whether vpp.lcp is on. Absent container means off; a
// present container with no `enabled` leaf means on (the YANG default), which
// is why this is not a plain Get.
func lcpEnabled(tree *config.Tree) bool {
	lcp := tree.GetContainerPath("vpp/lcp")
	if lcp == nil {
		return false
	}
	enabled, ok := lcp.Get("enabled")
	return !ok || enabled != "false"
}

// checkVPPLCPPlugin reports when vpp.lcp is enabled but the running VPP does
// not load linux_cp_plugin.so.
//
// Without this, the misconfiguration surfaces only at apply time, as a raw VPP
// binapi error that fails the WHOLE config apply -- and names the failing
// message, not the missing plugin. Probing here turns that into an actionable
// pre-apply diagnostic.
//
// Severity follows what the probe PROVED, so the two answers differ.
//
// A probe that ANSWERED and did not list the plugin is an ERROR. startup.conf
// asked for linux_cp_plugin.so (component/vpp/startupconf.go) and the running
// VPP does not have it. The first lcp_itf_pair_add_del then fails the whole
// apply, so the failure is certain.
//
// A probe that gave no answer degrades to a WARNING and never claims the plugin
// is missing. `vppctl` exits non-zero for an absent binary, an absent socket and
// a wedged VPP alike. None of those is evidence about the plugin set. An error
// there would fail closed in the wrong direction: it would tell an operator to
// rebuild VPP while the real problem is that VPP is not running.
//
// Output that exits zero without vppctlPluginsHeader is no answer either. An
// empty or truncated stdout carries no plugin row at all, so linux_cp is absent
// from it whether or not VPP loaded it. That case degrades to the same WARNING,
// for the same reason: a non-answer is not evidence.
func checkVPPLCPPlugin(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	if !lcpEnabled(tree) {
		return nil
	}
	// VPP is Linux-only, so on any other host the probe is nil and nothing is
	// opened. Checked AFTER the config gate so the skip reason is "wrong
	// platform", not "config absent".
	if lcpPluginProbe == nil {
		return nil
	}

	probeCtx, cancel := context.WithTimeout(context.Background(), vppProbeTimeout)
	defer cancel()
	out, err := lcpPluginProbe(probeCtx)
	var tb textbuf.Buffer
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-vpp-lcp-plugin",
			Severity: diagnostic.SeverityWarning,
			Message: tb.Str("vpp.lcp is enabled but the running VPP could not be probed for ").
				Str(lcpPluginSO).Str(": ").Err(err).String(),
		}}
	}
	if !strings.Contains(out, vppctlPluginsHeader) {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-vpp-lcp-plugin",
			Severity: diagnostic.SeverityWarning,
			Message: tb.Str("vpp.lcp is enabled but the running VPP could not be probed for ").
				Str(lcpPluginSO).Str(": the probe exited zero without the \"").Str(vppctlPluginsHeader).
				Str("\" line, so its output is empty or truncated").String(),
		}}
	}
	if strings.Contains(out, lcpPluginSO) {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     "doctor-vpp-lcp-plugin",
		Severity: diagnostic.SeverityError,
		Message: tb.Reset().Str("vpp.lcp is enabled but the running VPP does not load ").Str(lcpPluginSO).
			Str("; the linux_cp API is unavailable and the config apply will fail at the binapi layer").String(),
	}}
}
