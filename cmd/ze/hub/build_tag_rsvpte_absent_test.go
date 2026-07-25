// Design: ai/rules/feature-gate-registration.md -- ze_rsvpte absent (compile-out) validation
//
//go:build !ze_rsvpte

package hub

// VALIDATES: without the ze_rsvpte build tag (e.g. ze-stripped or a bare ze_core
// build), the RSVP-TE plugin is NOT registered and its config schema is absent,
// while the rest of the plugin registry is still populated. The binary
// symbol-drop proof is in build_tag_protocols_absent_test.go.
// PREVENTS: a regression where rsvp-te leaks into a hardened build via an
// always-on import or an ungated registration/schema import.

import (
	"strings"
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestBuildTag_RSVPTE_Absent(t *testing.T) {
	if len(pluginreg.Names()) == 0 {
		t.Fatal("plugin registry empty; cannot validate rsvp-te absence (all.go not linked)")
	}
	if pluginreg.Has("rsvp-te") {
		t.Fatal("non-ze_rsvpte build: rsvp-te plugin unexpectedly registered (not compiled out)")
	}
}

func TestBuildTag_RSVPTE_AbsentRejectsRSVPTEConfig(t *testing.T) {
	_, err := zeconfig.ParseTreeWithYANG("rsvp-te {\n\tenabled true;\n}\n", nil)
	if err == nil {
		t.Fatal("non-ze_rsvpte build unexpectedly accepted rsvp-te config")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("rsvp-te config rejection = %v, want clean unknown-field rejection", err)
	}
}
