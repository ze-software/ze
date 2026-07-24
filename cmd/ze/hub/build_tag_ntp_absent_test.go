// Design: ai/rules/feature-gate-registration.md -- ze_ntp absent (compile-out) validation
//
//go:build !ze_ntp

package hub

// VALIDATES: without the ze_ntp build tag (e.g. a bare ze_core build), the NTP
// plugin is NOT registered and its config schema (`environment { ntp {} }`) is
// absent, while the rest of the plugin registry is still populated. `show
// system` degrades cleanly because its only coupling is the nil-safe
// registry.GetNTPSyncInfo seam. The binary symbol-drop proof is in
// build_tag_gate12_absent_test.go.
// PREVENTS: a regression where ntp leaks into a hardened build via an always-on
// import or an ungated registration/schema import.

import (
	"strings"
	"testing"

	zeconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
	pluginreg "codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

func TestBuildTag_NTP_Absent(t *testing.T) {
	if len(pluginreg.Names()) == 0 {
		t.Fatal("plugin registry empty; cannot validate ntp absence (all.go not linked)")
	}
	if pluginreg.Has("ntp") {
		t.Fatal("non-ze_ntp build: ntp plugin unexpectedly registered (not compiled out)")
	}
}

// TestBuildTag_NTP_AbsentRejectsNTPConfig proves the ntp config schema is gone
// too, not just the engine. NTP config lives under the shared `environment`
// container (ze-ntp-conf.yang), so the snippet uses a minimal environment tree
// whose only schema-gated token is `ntp`. A bare build must reject it as an
// unknown field rather than silently accept or crash.
func TestBuildTag_NTP_AbsentRejectsNTPConfig(t *testing.T) {
	const cfg = `environment {
	ntp {
		enabled true;
		interval 3600;
	}
}
`
	_, err := zeconfig.ParseTreeWithYANG(cfg, nil)
	if err == nil {
		t.Fatal("non-ze_ntp build unexpectedly accepted ntp config")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("ntp config rejection = %v, want clean unknown-field rejection", err)
	}
}

// TestBuildTag_NTP_AbsentSyncInfoNil proves the show-system seam degrades: with
// no NTP plugin registered, GetNTPSyncInfo returns nil (the provider was never
// set), which is exactly what cmd/show/system.go nil-checks before rendering
// the NTP block.
func TestBuildTag_NTP_AbsentSyncInfoNil(t *testing.T) {
	if info := pluginreg.GetNTPSyncInfo(); info != nil {
		t.Fatalf("non-ze_ntp build: GetNTPSyncInfo = %v, want nil (no provider registered)", info)
	}
}
