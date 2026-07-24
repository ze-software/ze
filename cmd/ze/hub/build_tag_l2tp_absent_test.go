// Design: ai/rules/feature-gate-registration.md -- ze_l2tp/ze_radius absent (compile-out) validation
//
//go:build !ze_l2tp && !ze_radius

package hub

// VALIDATES: without the ze_l2tp / ze_radius build tags (the bare ze_core
// lane) the BNG is gone: the bngRegister seam stays nil, none of the BNG or
// radius plugins are registered, and l2tp / pppoe / radius-auth config blocks
// are rejected as unknown -- while the rest of the plugin registry is still
// populated. The binary symbol-drop proof (the whole l2tp subtree including
// events, the radius client, and both dependent directions) is in
// build_tag_gate12_absent_test.go.
// PREVENTS: a regression where the BNG leaks into a hardened build through
// one of its many roots (hub seam, dispatch CLI, web pages, diag captures,
// the cos dynamic handler, or the aaa/all radius sibling).

import (
	"strings"
	"testing"

	zeconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
	pluginreg "codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

func TestBuildTag_L2TP_Absent(t *testing.T) {
	if len(pluginreg.Names()) == 0 {
		t.Fatal("plugin registry empty; cannot validate l2tp absence (all.go not linked)")
	}
	if bngRegister != nil {
		t.Error("bare build: bngRegister seam unexpectedly filled (not compiled out)")
	}
	for _, name := range []string{"l2tp-auth-local", "l2tp-pool", "l2tp-shaper", "l2tp-auth-radius"} {
		if pluginreg.Has(name) {
			t.Errorf("bare build: plugin %q unexpectedly registered (not compiled out)", name)
		}
	}
}

// TestBuildTag_L2TP_AbsentRejectsConfig proves the BNG and RADIUS config
// schemas are gone too, not just the engines: l2tp, pppoe, and the RADIUS
// admin-auth block (gated under plain ze_radius, which is also off in this
// !ze_l2tp && !ze_radius build) must all be rejected as unknown. The radius
// case is the fail-closed proof that AAA cannot silently drop a
// configured-but-compiled-out method (the analogue of the tacacs case in
// build_tag_gate12_group_b_absent_test.go).
func TestBuildTag_L2TP_AbsentRejectsConfig(t *testing.T) {
	cases := map[string]string{
		"l2tp":         "l2tp {\n\tenabled true;\n}\n",
		"pppoe":        "pppoe {\n\tenabled true;\n}\n",
		"radius-admin": "system {\n\tauthentication {\n\t\tradius {\n\t\t\tserver 192.0.2.1 {\n\t\t\t\tport 1812;\n\t\t\t}\n\t\t}\n\t}\n}\n",
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
