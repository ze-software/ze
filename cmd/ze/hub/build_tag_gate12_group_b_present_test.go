// Design: ai/rules/feature-gate-registration.md -- spec-feature-gate-12 Group B present validation
//
//go:build ze_tacacs && ze_exabgp

package hub

// test-relax: the first version of this test asserted
// cmdreg.HasRootHandler("tacacs"/"exabgp"), which can never be true in this
// test binary: those root commands are registered by the ze binary's dispatch
// composition root (cmd/ze package main, gated dispatch_tacacs.go /
// dispatch_exabgp.go), which ./cmd/ze/hub tests never link. The assertions
// were replaced (same first run, never a passing baseline) with checks this
// binary genuinely links: schema acceptance + bridge plugin registration.
// Dispatch-root absence is proven by the nm needles in
// build_tag_gate12_absent_test.go; presence by the exabgp functional suite.
//
// VALIDATES: with the ze_tacacs and ze_exabgp build tags (default-on in
// ZE_FEATURES) the tacacs and exabgp config schemas parse (their yang packages
// reached the composition root via the generated group files) and the
// in-process ExaBGP bridge plugin is in the plugin registry.
// PREVENTS: a regression where a Group B tag is set but the generated group
// files or the aaa/all gated sibling dropped.

import (
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestBuildTag_Gate12GroupB_Present(t *testing.T) {
	if !pluginreg.Has("exabgp-bridge") {
		t.Error("ze_exabgp build: exabgp-bridge plugin not registered")
	}
	cases := map[string]string{
		"tacacs": "system {\n\tauthentication {\n\t\ttacacs {\n\t\t\tserver 192.0.2.1 {\n\t\t\t\tport 49;\n\t\t\t}\n\t\t}\n\t}\n}\n",
		"exabgp": "exabgp {\n\tbridge {\n\t\trun \"./plugin.py\";\n\t}\n}\n",
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := zeconfig.ParseTreeWithYANG(cfg, nil); err != nil {
				t.Fatalf("full-feature build rejected %s config: %v", name, err)
			}
		})
	}
}
