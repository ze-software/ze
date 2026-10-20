// Design: ai/rules/plugins.md -- ze_l2tp/ze_bgp absent (compile-out) validation
// Related: ../../../internal/component/plugin/all/health_rpcs_owner_test.go -- the same rpcs with both owners
//
//go:build !ze_l2tp && !ze_bgp

package hub

import (
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestHealthRPCsAbsentWithoutOwner proves a build with neither L2TP nor BGP
// publishes no health rpc of either: the rpcs live in the owners' API
// modules, which compile out with them.
//
// VALIDATES: without ze_l2tp and ze_bgp, no rpc in PublishedRPCs.Commands,
// Protocol or Unnamed is l2tp-health or bgp-health, and ze-cli-show-api
// declares neither.
// PREVENTS: an always-linked module declaring an rpc that nothing in the
// binary answers, published under Unnamed with no method to send.
func TestHealthRPCsAbsentWithoutOwner(t *testing.T) {
	if len(pluginreg.Names()) == 0 {
		t.Fatal("plugin registry empty; cannot validate health rpc absence (all.go not linked)")
	}
	loader := configyang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}
	if err := loader.LoadRegistered(); err != nil {
		t.Fatalf("LoadRegistered: %v", err)
	}
	schema, err := loader.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	pub, err := configyang.PublishedRPCs(schema)
	if err != nil {
		t.Fatalf("PublishedRPCs: %v", err)
	}

	owned := map[string]bool{"l2tp-health": true, "bgp-health": true}
	for set, metas := range map[string][]configyang.RPCMeta{
		"Commands": pub.Commands,
		"Protocol": pub.Protocol,
		"Unnamed":  pub.Unnamed,
	} {
		for _, meta := range metas {
			if owned[meta.Name] {
				t.Errorf("bare build publishes %s:%s under %s; its owner is compiled out", meta.Module, meta.Name, set)
			}
		}
	}
	for _, meta := range configyang.ExtractRPCs(schema, "ze-cli-show-api") {
		if owned[meta.Name] {
			t.Errorf("ze-cli-show-api declares %s, which its owner's API module carries", meta.Name)
		}
	}
}
