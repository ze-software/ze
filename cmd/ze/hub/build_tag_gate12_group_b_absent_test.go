// Design: ai/rules/feature-gate-registration.md -- spec-feature-gate-12 Group B absent validation
//
//go:build !ze_tacacs && !ze_exabgp

package hub

// VALIDATES: without the ze_tacacs / ze_exabgp build tags (the bare ze_core
// lane) the TACACS+ and ExaBGP root commands are not registered, the
// exabgp-bridge plugin is absent, and a tacacs / exabgp config block is
// rejected as unknown -- while the rest of the registries stay populated.
// AAA fails closed by construction: the tacacs schema is gone, so a config
// naming tacacs can never load and silently skip the missing method. The
// binary symbol-drop proof is in build_tag_gate12_absent_test.go.
// PREVENTS: a regression where tacacs or exabgp leaks into a hardened build
// through one of their extra composition roots (aaa/all, the dispatch root).

import (
	"strings"
	"testing"

	zeconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
	pluginreg "codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// test-relax: the first version also asserted !cmdreg.HasRootHandler for
// "tacacs"/"exabgp", which passes VACUOUSLY here: those root commands are only
// registered in the ze binary's dispatch composition root (cmd/ze package
// main), which this test binary never links, so the assertions could not fail
// even if gating broke. Removed (same session as authored, never a meaningful
// baseline); the real dispatch-root drop proof is the nm needle set in
// build_tag_gate12_absent_test.go.
func TestBuildTag_Gate12GroupB_Absent(t *testing.T) {
	if len(pluginreg.Names()) == 0 {
		t.Fatal("plugin registry empty; cannot validate absence (all.go not linked)")
	}
	if pluginreg.Has("exabgp-bridge") {
		t.Error("bare build: exabgp-bridge plugin unexpectedly registered (not compiled out)")
	}
}

// TestBuildTag_Gate12GroupB_AbsentRejectsConfig proves both features' config
// schemas are gone: the tacacs block under system/authentication and the
// exabgp bridge root must be rejected as unknown fields.
func TestBuildTag_Gate12GroupB_AbsentRejectsConfig(t *testing.T) {
	cases := map[string]string{
		"tacacs": "system {\n\tauthentication {\n\t\ttacacs {\n\t\t\tserver 192.0.2.1 {\n\t\t\t\tport 49;\n\t\t\t}\n\t\t}\n\t}\n}\n",
		"exabgp": "exabgp {\n\tbridge {\n\t\trun \"./plugin.py\";\n\t}\n}\n",
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := zeconfig.ParseTreeWithYANG(cfg, nil)
			if err == nil {
				t.Fatalf("bare build unexpectedly accepted %s config", name)
			}
			if !strings.Contains(err.Error(), "unknown") {
				t.Fatalf("%s config rejection = %v, want clean unknown-field rejection", name, err)
			}
		})
	}
}
