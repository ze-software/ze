// Design: docs/features/ai-first.md — system readiness checks for agent tooling
// Related: doctor.go — readiness check runner and output contract
// Related: checks_helpers.go — shared config-tree navigation helpers

// Config coherence checks: semantic validation bridge, BGP filter reference
// resolution, TCP MD5 platform support for configured passwords, and the
// interface backend's runtime dependency (VPP API socket).

package doctor

import (
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/network"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// diagnosticBGPMD5 names a TCP-MD5 configuration fault an operator sees in
// `ze doctor` output.
const diagnosticBGPMD5 = "doctor-bgp-md5"

func checkIfaceBackend(tree *config.Tree) []diagnostic.Diagnostic {
	ifaceBlock := tree.GetContainer("interface")
	if ifaceBlock == nil {
		return nil
	}
	backend, ok := ifaceBlock.Get("backend")
	if !ok || backend == "" {
		return nil
	}

	if backend == "vpp" {
		sockPath := ""
		if vpp := tree.GetContainer("vpp"); vpp != nil {
			sockPath, _ = vpp.Get("api-socket")
		}
		return checkVPPSocket(sockPath)
	}

	return nil
}

func checkConfigReferences(tree *config.Tree) []diagnostic.Diagnostic {
	bgpBlock := tree.GetContainer("bgp")
	if bgpBlock == nil {
		return nil
	}

	// Collect defined filter instance names from bgp/policy.
	// Policy lists (prefix-list, as-path, etc.) are added by plugins via YANG
	// augment. Each list's keys are filter instance names.
	defined := make(map[string]bool)
	if policy := bgpBlock.GetContainer("policy"); policy != nil {
		policyMap := policy.ToMap()
		collectFilterNamesFromMap(policyMap, defined)
	}

	var diags []diagnostic.Diagnostic

	// Collect all filter references from global, group, and peer levels.
	if filter := bgpBlock.GetContainer("filter"); filter != nil {
		diags = append(diags, checkFilterRefs(filter, defined, "bgp/filter")...)
	}

	var tb textbuf.Buffer
	groups := bgpBlock.GetListOrdered("group")
	for _, g := range groups {
		groupPath := tb.Reset().Str("bgp/group/").Str(g.Key).Str("/filter").String()
		if filter := g.Value.GetContainer("filter"); filter != nil {
			diags = append(diags, checkFilterRefs(filter, defined, groupPath)...)
		}
		peers := g.Value.GetListOrdered("peer")
		for _, p := range peers {
			peerPath := tb.Reset().Str("bgp/group/").Str(g.Key).Str("/peer/").Str(p.Key).Str("/filter").String()
			if filter := p.Value.GetContainer("filter"); filter != nil {
				diags = append(diags, checkFilterRefs(filter, defined, peerPath)...)
			}
		}
	}

	peers := bgpBlock.GetListOrdered("peer")
	for _, p := range peers {
		peerPath := tb.Reset().Str("bgp/peer/").Str(p.Key).Str("/filter").String()
		if filter := p.Value.GetContainer("filter"); filter != nil {
			diags = append(diags, checkFilterRefs(filter, defined, peerPath)...)
		}
	}

	return diags
}

// collectFilterNamesFromMap walks the policy map (from ToMap()) and collects
// all second-level keys as filter instance names. The map structure is:
//
//	{"prefix-list": {"customers": {...}}, "as-path": {"as1234": {...}}}
func collectFilterNamesFromMap(m map[string]any, defined map[string]bool) {
	for _, v := range m {
		sub, ok := v.(map[string]any)
		if !ok {
			continue
		}
		for name := range sub {
			defined[name] = true
		}
	}
}

// filterInstanceName extracts the filter instance name from a reference.
// Filter references can use three forms:
//   - "bgp-filter-prefix:customers"  (plugin-process:name)
//   - "prefix-list:customers"        (filter-type:name)
//   - "customers"                    (plain name)
//
// All resolve to the same instance name after stripping the prefix.
func filterInstanceName(ref string) string {
	if i := strings.LastIndex(ref, ":"); i >= 0 {
		return ref[i+1:]
	}
	return ref
}

func checkFilterRefs(filter *config.Tree, defined map[string]bool, path string) []diagnostic.Diagnostic {
	var diags []diagnostic.Diagnostic
	var tb textbuf.Buffer
	for _, dir := range []string{"import", "export"} {
		// Use the structural member view so deactivated refs are validated too:
		// a deactivated ref to an undefined filter is still a latent config
		// error worth flagging. The member value is clean (no inactive: prefix).
		for _, m := range filter.GetMultiValuesState(dir) {
			name := filterInstanceName(m.Value)
			if len(defined) == 0 || !defined[name] {
				diags = append(diags, diagnostic.Diagnostic{
					Code:     "doctor-config-reference",
					Severity: diagnostic.SeverityError,
					Message:  tb.Reset().Str(path).Byte('/').Str(dir).Str(": references undefined filter '").Str(m.Value).Byte('\'').String(),
				})
			}
		}
	}
	return diags
}

func checkSemanticValidation(tree *config.Tree) []diagnostic.Diagnostic {
	return config.ValidateSemantics(tree)
}

// checkBGPPeerConfig runs the engine's own peer resolution over the tree, the
// same gate `ze config validate` applies (cmd_validate.go runValidation), and
// reports a failure as an error diagnostic.
//
// Without it `ze doctor` answered "ready": true for a config the daemon refuses
// to start on. test/plugin/mpls-doctor.ci declared a peer family
// `ipv4/mpls-unicast`, which does not exist -- the registered names are
// `ipv4/mpls-label` and `ipv4/mpls-vpn` -- and `ze config validate` rejects it
// with "peer peer1: unknown address family". Doctor loaded the same file, found
// no labeled family (so its MPLS-module warning correctly stayed silent),
// reported ready, and exited 0. The test then failed on the missing warning
// while the ACTUAL defect -- a config error doctor cannot see -- went unreported
// on the operator-facing path too.
//
// config.ValidateSemantics cannot host this: the peer validator lives behind the
// infra seam (infra.SetBGPPeerValidator, filled by internal/component/bgp/config
// so ze_bgp can compile out), and infra imports config, so config importing
// infra would be a cycle. The seam is nil with the BGP engine compiled out, and
// ValidateBGPPeers then returns nil -- no BGP, no peers to reject.
// The tree is CLONED before it is handed over, and that is load-bearing rather
// than defensive. ValidateBGPPeers reaches PeersFromConfigTree, which calls
// config.PruneInactive -- documented at internal/component/config/prune.go:168
// as "modified in place. Call on a clone if the original must be preserved."
// Doctor's tree is shared by ~30 later checks in runChecks, so validating in
// place silently deleted every `inactive:` peer from underneath them: a config
// whose inactive peer names an unbindable local ip stopped producing
// doctor-bgp-listen for it, but ONLY when a bgp{} block was present (this
// function returns early otherwise), making the whole report order- and
// content-dependent. Whether doctor should skip inactive nodes is a real
// question; it must not be answered as a side effect of a function named
// "validate".
func checkBGPPeerConfig(tree *config.Tree) []diagnostic.Diagnostic {
	if tree == nil || tree.GetContainer("bgp") == nil {
		return nil
	}
	err := infra.ValidateBGPPeers(tree.Clone())
	if err == nil {
		return nil
	}
	// ERROR, so the report is not ready and `ze doctor` exits 1 (owner decision,
	// 2026-07-27). The daemon will not start on this config, and a readiness
	// check that answers "ready" for a config the engine refuses is the operator
	// trap this check exists to close. Two test/ui fixtures asserted exit 0 while
	// carrying engine-invalid configs (doctor-bgp-listen omitted `prefix
	// maximum`, doctor-bgp-md5 omitted `connection local ip`); both were the
	// defect, and both are fixed rather than accommodated.
	var tb textbuf.Buffer
	return []diagnostic.Diagnostic{{
		Code:     "doctor-config-bgp-peer",
		Severity: diagnostic.SeverityError,
		Message:  tb.Str("bgp peer configuration rejected (the daemon will not start on it): ").Err(err).String(),
	}}
}

func checkBGPMD5(tree *config.Tree) []diagnostic.Diagnostic {
	if network.TCPMD5Supported() {
		return nil
	}
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return nil
	}

	hasMD5 := func(parent, node *config.Tree) bool {
		if pw, ok := inheritedValue(parent, node, "connection", "md5", "password"); ok && pw != "" {
			return true
		}
		return false
	}

	var tb textbuf.Buffer
	for _, p := range bgp.GetListOrdered("peer") {
		if hasMD5(nil, p.Value) {
			return []diagnostic.Diagnostic{{
				Code:     diagnosticBGPMD5,
				Severity: diagnostic.SeverityWarning,
				Message:  tb.Reset().Str("BGP peer ").Str(p.Key).Str(" requires TCP MD5 but platform does not support it").String(),
			}}
		}
	}
	for _, g := range bgp.GetListOrdered("group") {
		if hasMD5(nil, g.Value) {
			return []diagnostic.Diagnostic{{
				Code:     diagnosticBGPMD5,
				Severity: diagnostic.SeverityWarning,
				Message:  tb.Reset().Str("BGP group ").Str(g.Key).Str(" requires TCP MD5 but platform does not support it").String(),
			}}
		}
		for _, p := range g.Value.GetListOrdered("peer") {
			if hasMD5(g.Value, p.Value) {
				return []diagnostic.Diagnostic{{
					Code:     diagnosticBGPMD5,
					Severity: diagnostic.SeverityWarning,
					Message:  tb.Reset().Str("BGP peer ").Str(g.Key).Byte('/').Str(p.Key).Str(" requires TCP MD5 but platform does not support it").String(),
				}}
			}
		}
	}
	return nil
}
