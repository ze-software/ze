// Design: ai/rules/plugins.md -- spec-feature-gate-12 Group A present validation
//
//go:build ze_flowexport && ze_ddos && ze_anomaly && ze_as112 && ze_geodns && ze_dhcpserver && ze_pxe && ze_trafficusage && ze_policyroute && ze_cos && ze_copp && ze_mpls

package hub

// VALIDATES: with the spec-feature-gate-12 Group A build tags (all default-on in
// ZE_FEATURES) every gated plugin is registered in the plugin registry, and the
// MPLS show-forwarding RPC is registered -- i.e. the generated all_ze_<x>.go
// group files reach the composition root. Table-driven over one compound
// constraint because the two exercised lanes are all-on (default build) and
// all-off (bare ze_core); per-tag granularity is kept where it discriminates
// (the per-subtree nm needles in build_tag_gate12_absent_test.go).
// PREVENTS: a regression where a Group A tag is set but its plugin is not wired
// -- a generated group file dropped, or the manifest line not reaching the
// generator.

import (
	"testing"

	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// gate12GroupAPlugins maps each Group A build tag to the plugin-registry names
// its gated packages register.
var gate12GroupAPlugins = map[string][]string{
	"ze_flowexport":   {"flow-export"},
	"ze_ddos":         {"ddos-detect", "ddos-flowspec", "ddos-flowtriq", "ddos-local", "ddos-observe"},
	"ze_anomaly":      {"anomaly-detect", "anomaly-observe", "anomaly-shape"},
	"ze_as112":        {"as112"},
	"ze_geodns":       {"geodns"},
	"ze_dhcpserver":   {"dhcpserver"},
	"ze_pxe":          {"tftpserver", "imageserver"},
	"ze_trafficusage": {"traffic-usage"},
	"ze_policyroute":  {"policy-routes"},
	"ze_cos":          {"cos"},
	"ze_copp":         {"copp"},
}

func TestBuildTag_Gate12GroupA_Present(t *testing.T) {
	for tag, names := range gate12GroupAPlugins {
		for _, name := range names {
			if !pluginreg.Has(name) {
				t.Errorf("%s build: plugin %q not registered", tag, name)
			}
		}
	}
}

// TestBuildTag_Gate12GroupA_MPLSPresent covers ze_mpls, which registers an RPC
// (not a plugin-registry entry): the show-forwarding wire method must be in the
// built-in RPC surface.
func TestBuildTag_Gate12GroupA_MPLSPresent(t *testing.T) {
	for _, reg := range pluginserver.AllBuiltinRPCs() {
		if reg.WireMethod == "ze-show:mpls-forwarding" {
			return
		}
	}
	t.Fatal("ze_mpls build: ze-show:mpls-forwarding RPC not registered")
}
