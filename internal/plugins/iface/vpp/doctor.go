// Design: ai/rules/doctor-checks.md -- self-contained doctor checks owned by
// the plugin that owns the runtime dependency.
// Overview: register.go -- init() registers these checks.
//
// The vpp interface backend adds two runtime dependencies that ze doctor
// surfaces so a misconfiguration is caught before apply rather than as a raw
// VPP API error:
//
//   - wireguard: a wireguard interface under backend vpp needs the VPP
//     wireguard plugin. The plugin loads only when vpp.plugins.wireguard is
//     enabled (plugin default { disable }); doctor warns when the interface is
//     configured but the toggle is off.
//   - lcp netns: LCP TAPs land in vpp.lcp.netns; BGP can only bind on a shadow
//     interface when it runs in that namespace. doctor warns when BGP is
//     enabled and the netns is not a root-reachable namespace (see A-4).

package ifacevpp

import (
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

// checkVPPLCPNetns warns when BGP is enabled and LCP is configured with a netns
// that is not root-reachable, because BGP (running in ze's own netns) cannot
// bind on an LCP-shadowed interface that lives in a separate namespace (A-4).
func checkVPPLCPNetns(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	if tree.GetContainer("bgp") == nil {
		return nil
	}
	lcp := tree.GetContainerPath("vpp/lcp")
	if lcp == nil {
		return nil
	}
	if enabled, ok := lcp.Get("enabled"); ok && enabled == "false" {
		return nil
	}
	// Default netns is "dataplane" when the leaf is omitted (YANG default).
	netns := "dataplane"
	if v, ok := lcp.Get("netns"); ok {
		netns = v
	}
	if lcpNetnsIsRootReachable(netns) {
		return nil
	}
	// Remediation: there is no config-only fix. No value of vpp.lcp.netns puts
	// the TAPs where a root-netns ze can bind, because VPP resolves the leaf as
	// a namespace NAME under /var/run/netns/ (linux-cp lcp_set_default_ns,
	// third_party/vpp-linux-cp/src/lcp.c:73-74) -- host and root
	// are not the host namespace -- and ze has no netns-aware listener
	// (RealListenerFactory.Listen, internal/core/network/network.go:167). Naming
	// a value here is what the previous message did, and following it fails LCP
	// pair creation. The long form lives in the registered code's description
	// (internal/core/diagnostic/codes.go), which ze explain prints.
	var tb textbuf.Buffer
	msg := tb.Str("bgp is enabled and vpp.lcp.netns=").Quoted(netns).
		Str(" is not root-reachable; BGP cannot bind on an LCP-shadowed interface in a separate namespace. No vpp.lcp.netns value fixes this: VPP resolves the leaf as a namespace name under /var/run/netns/, so host and root are not the host namespace. Run ze in the ").Quoted(netns).
		Str(" namespace so BGP binds where the TAPs are, or see ze explain doctor-vpp-lcp-netns").String()
	return []diagnostic.Diagnostic{{
		Code:     "doctor-vpp-lcp-netns",
		Severity: diagnostic.SeverityWarning,
		Message:  msg,
	}}
}

// lcpNetnsIsRootReachable reports whether an LCP netns name is one ze TREATS as
// the host (root) network namespace, where its BGP listener runs by default.
//
// This is a statement about ze's own marker set ("", host, root), not about VPP.
// VPP has no special namespace names: lcp_set_default_ns formats the leaf into the
// path /var/run/netns/<name> and opens it
// (third_party/vpp-linux-cp/src/lcp.c:73-74), and lcp_itf_pair_create resolves an
// EMPTY per-pair netns to that GLOBAL default
// (third_party/vpp-linux-cp/src/lcp_interface.c:850-855), which ze always writes
// from this same leaf when LCP is enabled (internal/component/vpp/startupconf.go:106).
// So netns=host makes VPP open /var/run/netns/host rather than stay in its own
// namespace, and these markers are not the escape hatch they look like. Verified
// in VPP's C sources, not in the generated binapi stub, which documents only that
// the field exists; recorded as A-13 in plan/spec-bgp-netns.md.
func lcpNetnsIsRootReachable(netns string) bool {
	switch netns {
	case "", "host", "root":
		return true
	default:
		return false
	}
}
