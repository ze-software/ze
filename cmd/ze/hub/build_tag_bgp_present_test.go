// Design: ai/rules/plugins.md -- ze_bgp present build validation
//
//go:build ze_bgp

package hub

// VALIDATES: with the ze_bgp build tag (a default ze / ze-appliance feature) the
// BGP plugin is registered in the plugin registry, the bgp{} config schema
// parses, and bgp/config's init() fills every seam the always-on side depends on
// (reactor factory, config resolver, peer validator, GR-marker writer).
// PREVENTS: a regression where ze_bgp is set but bgp is not wired -- the
// generated all_ze_bgp.go blank import dropped, the tag not reaching the
// generator, or bgp/config's init() no longer registering a seam, which would
// leave `ze config validate` silently skipping BGP checks.
//
// The bgp/config import here is a blank (side-effect) import, mirroring what
// cmd/ze/dispatch_bgp.go does in the real binary: that file, not this package,
// links bgp/config into ze. A test file may name it freely -- the compile-out
// invariant is about always-on NON-test code.

import (
	"testing"

	_ "github.com/ze-software/ze/internal/component/bgp/config"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/infra"
	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestBuildTag_BGP_Present(t *testing.T) {
	if !pluginreg.Has("bgp") {
		t.Fatal("ze_bgp build: bgp plugin not registered")
	}
	if pluginreg.GetReactorFactory() == nil {
		t.Fatal("ze_bgp build: no reactor factory registered; a bgp{} config could not build a reactor")
	}
}

// TestBuildTag_BGP_PresentAcceptsBGPConfig is the positive half of the
// absent-build rejection test: the same snippet that a bare ze_core build must
// reject as an unknown field MUST parse here. Without this pairing the absent
// test could pass for the wrong reason (a syntactically invalid snippet).
func TestBuildTag_BGP_PresentAcceptsBGPConfig(t *testing.T) {
	tree, err := zeconfig.ParseTreeWithYANG(bgpAbsenceProbeConfig, nil)
	if err != nil {
		t.Fatalf("ze_bgp build rejected a valid bgp config: %v", err)
	}

	// Resolving it also proves bgp/config's init() filled the always-on infra
	// seam. Asserted through the real path rather than a boolean: with the
	// resolver absent this call FAILS CLOSED on a tree carrying bgp{}, which is
	// exactly what `ze config dump|diff|validate` would hit.
	resolved, err := infra.ResolveBGPTree(tree)
	if err != nil {
		t.Fatalf("ze_bgp build could not resolve a bgp config through the infra seam: %v", err)
	}
	if len(resolved) == 0 {
		t.Error("resolved bgp tree is empty; the seam returned its no-engine fallback")
	}
}
