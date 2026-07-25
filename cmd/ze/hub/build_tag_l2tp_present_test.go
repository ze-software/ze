// Design: ai/rules/feature-gate-registration.md -- ze_l2tp/ze_radius present (compile-out) validation
//
//go:build ze_l2tp && ze_radius

package hub

// VALIDATES: with the ze_l2tp and ze_radius build tags (default-on in
// ZE_FEATURES) the BNG is wired: the bngRegister seam is filled by
// register_l2tp.go, the BNG plugins (local auth, pool, shaper) and the
// dependent authradius plugin (ze_l2tp && ze_radius group file) are
// registered, and the l2tp/pppoe config schemas parse.
// PREVENTS: a regression where the tags are set but a composition root's
// gated file dropped (register_l2tp.go, the generated group files, or the
// aaa/all radius sibling).

import (
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestBuildTag_L2TP_Present(t *testing.T) {
	if bngRegister == nil {
		t.Error("ze_l2tp build: bngRegister seam not filled (register_l2tp.go init missing)")
	}
	for _, name := range []string{"l2tp-auth-local", "l2tp-pool", "l2tp-shaper", "l2tp-auth-radius"} {
		if !pluginreg.Has(name) {
			t.Errorf("ze_l2tp/ze_radius build: plugin %q not registered", name)
		}
	}
	cases := map[string]string{
		"l2tp":  "l2tp {\n\tenabled true;\n}\n",
		"pppoe": "pppoe {\n\tenabled true;\n}\n",
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := zeconfig.ParseTreeWithYANG(cfg, nil); err != nil {
				t.Fatalf("full-feature build rejected %s config: %v", name, err)
			}
		})
	}
}
