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
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/core/diagnostic"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
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
	var tb textbuf.Buffer
	msg := tb.Str("bgp is enabled and vpp.lcp.netns=").Quoted(netns).
		Str(" is not root-reachable; BGP cannot bind on an LCP-shadowed interface in a separate namespace. Set vpp.lcp.netns to host or root, or run BGP in that namespace.").String()
	return []diagnostic.Diagnostic{{
		Code:     "doctor-vpp-lcp-netns",
		Severity: diagnostic.SeverityWarning,
		Message:  msg,
	}}
}

// lcpNetnsIsRootReachable reports whether an LCP netns name denotes the host
// (root) network namespace, where ze's BGP listener runs by default. VPP's
// per-pair netns override maps these to the host netns.
func lcpNetnsIsRootReachable(netns string) bool {
	switch netns {
	case "", "host", "root":
		return true
	default:
		return false
	}
}
