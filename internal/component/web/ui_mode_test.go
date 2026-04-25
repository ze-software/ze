package web

import (
	"testing"

	"github.com/stretchr/testify/assert"

	// Side-effect import: registers ze.web.ui.
	_ "codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

// TestUIMode_DefaultsToFinder verifies that with no ze.web.ui env var set,
// the hub serves the Finder UI. This is the Phases 1-3 default; the Phase 4
// flip will replace this with Workbench, at which point the test is updated.
//
// VALIDATES: AC-1a (no env var -> Finder during Phases 1-3).
// PREVENTS: Accidentally shipping the V2 workbench as the active UI before
// the BGP change-and-verify loop passes the Promotion Criteria.
func TestUIMode_DefaultsToFinder(t *testing.T) {
	t.Setenv("ze.web.ui", "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	assert.Equal(t, UIModeFinder, GetUIMode())
}

// TestUIMode_OptInWorkbench verifies that ze.web.ui=workbench selects V2.
//
// VALIDATES: AC-1 (workbench opt-in renders the V2 shell).
// PREVENTS: The opt-in switch silently being ignored.
func TestUIMode_OptInWorkbench(t *testing.T) {
	t.Setenv("ze.web.ui", "workbench")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	assert.Equal(t, UIModeWorkbench, GetUIMode())
}

// TestUIMode_RollbackFinder verifies that ze.web.ui=finder selects Finder
// explicitly. During Phases 1-3 this is identical to the default; after the
// Phase 4 default flip this becomes the emergency rollback path.
//
// VALIDATES: AC-1a (explicit Finder rollback works).
// PREVENTS: The rollback switch being broken when the default flips.
func TestUIMode_RollbackFinder(t *testing.T) {
	t.Setenv("ze.web.ui", "finder")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	assert.Equal(t, UIModeFinder, GetUIMode())
}

// TestParseUIMode_KnownTokens verifies the parser recognizes both labels in
// any case and falls back to Finder for unknown values.
//
// VALIDATES: Robustness against operator typos and case variation.
// PREVENTS: An operator typing "WorkBench" and silently getting Finder
// without any indication, or an unknown token landing the user in V2.
func TestParseUIMode_KnownTokens(t *testing.T) {
	tests := []struct {
		input string
		want  UIMode
	}{
		{"", UIModeFinder},
		{"finder", UIModeFinder},
		{"Finder", UIModeFinder},
		{"FINDER", UIModeFinder},
		{"workbench", UIModeWorkbench},
		{"Workbench", UIModeWorkbench},
		{"WORKBENCH", UIModeWorkbench},
		{"unknown-mode", UIModeFinder},
		{"v2", UIModeFinder},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.want, ParseUIMode(tc.input))
		})
	}
}
