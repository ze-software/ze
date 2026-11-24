// Design: ai/rules/plugins.md -- spec-feature-gate-12 Group B present validation
//
//go:build ze_tacacs && ze_exabgp

package hub

// the first version of this test asserted
// cmdreg.HasRootHandler("tacacs"/"exabgp"), which can never be true in this
// test binary: those root commands are registered by the ze binary's dispatch
// composition root (cmd/ze package main, gated dispatch_tacacs.go /
// dispatch_exabgp.go), which ./cmd/ze/hub tests never link. The assertions
// were replaced (same first run, never a passing baseline) with checks this
// binary genuinely links: tacacs config schema acceptance (exabgp no longer
// owns a config root after the exabgp-bridge plugin removal; ExaBGP scripts
// run as ordinary external bus endpoints).
// Dispatch-root absence is proven by the nm needles in
// build_tag_gate12_absent_test.go; presence by the exabgp functional suite.
//
// VALIDATES: with the ze_tacacs and ze_exabgp build tags (default-on in
// ZE_FEATURES) the tacacs config schema parses (its yang package reached the
// composition root via the generated group files).
// PREVENTS: a regression where a Group B tag is set but the generated group
// files or the aaa/all gated sibling dropped.

import (
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
)

func TestBuildTag_Gate12GroupB_Present(t *testing.T) {
	cases := map[string]string{
		"tacacs": "system {\n\tauthentication {\n\t\ttacacs {\n\t\t\tserver 192.0.2.1 {\n\t\t\t\tport 49;\n\t\t\t}\n\t\t}\n\t}\n}\n",
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := zeconfig.ParseTreeWithYANG(cfg, nil); err != nil {
				t.Fatalf("full-feature build rejected %s config: %v", name, err)
			}
		})
	}
}
