// Design: ai/rules/plugins.md -- ze_rsvpte present build validation
//
//go:build ze_rsvpte

package hub

// VALIDATES: with the ze_rsvpte build tag (a default ze / ze-appliance feature),
// the RSVP-TE plugin is registered in the plugin registry (registered name
// "rsvp-te").
// PREVENTS: a regression where ze_rsvpte is set but rsvp-te is not wired (the
// generated all_ze_rsvpte.go blank import is dropped, or the tag stops reaching
// the generator).

import (
	"testing"

	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestBuildTag_RSVPTE_Present(t *testing.T) {
	if !pluginreg.Has("rsvp-te") {
		t.Fatal("ze_rsvpte build: rsvp-te plugin not registered")
	}
}
