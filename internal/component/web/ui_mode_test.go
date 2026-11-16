package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	// Side-effect import: registers ze.web.ui-mode.
	_ "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/env"
)

// TestReadUIModeFromRequestUsesYANGEditorByDefault follows the startup selector
// and request selector without an operator override.
// VALIDATES: AC-31 of spec-web-cli-config-one-off-defects: a fresh browser opens
// the graphical YANG editor.
// PREVENTS: a parser fallback selecting the unfinished Workbench by default.
func TestReadUIModeFromRequestUsesYANGEditorByDefault(t *testing.T) {
	t.Setenv("ze.web.ui-mode", "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/show/", http.NoBody)
	assert.Equal(t, UIModeFinder, ReadUIModeFromRequest(req, GetUIMode()))
}

// TestReadUIModeFromRequestKeepsWorkbenchSelection checks an explicit browser
// choice against the normal YANG editor startup mode.
// VALIDATES: AC-31 preserves the current switch cookie for Workbench.
// PREVENTS: changing the default from disabling the experimental alternative.
func TestReadUIModeFromRequestKeepsWorkbenchSelection(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/show/", http.NoBody)
	req.AddCookie(&http.Cookie{Name: uiModeSwitchCookie, Value: uiModeTokenWorkbench})

	assert.Equal(t, UIModeWorkbench, ReadUIModeFromRequest(req, UIModeFinder))
}

// TestUIMode_DefaultsToFinder verifies that with no ze.web.ui-mode env var set,
// the hub defaults to the graphical YANG editor.
//
// VALIDATES: AC-31: the graphical YANG editor is the default UI.
// PREVENTS: Selecting the unfinished Workbench on the main page by default.
func TestUIMode_DefaultsToFinder(t *testing.T) {
	t.Setenv("ze.web.ui-mode", "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	assert.Equal(t, UIModeFinder, GetUIMode())
}

// TestUIMode_OptInWorkbench verifies that ze.web.ui-mode=workbench selects the workbench.
//
// VALIDATES: AC-1 (workbench opt-in renders the workbench shell).
// PREVENTS: The opt-in switch silently being ignored.
func TestUIMode_OptInWorkbench(t *testing.T) {
	t.Setenv("ze.web.ui-mode", "workbench")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	assert.Equal(t, UIModeWorkbench, GetUIMode())
}

// TestUIMode_RollbackFinder verifies that ze.web.ui-mode=finder selects the
// graphical YANG editor explicitly.
//
// VALIDATES: Explicit selection of the graphical YANG editor works.
// PREVENTS: Ignoring a configured Finder selection.
func TestUIMode_RollbackFinder(t *testing.T) {
	t.Setenv("ze.web.ui-mode", "finder")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	assert.Equal(t, UIModeFinder, GetUIMode())
}

// TestReadUIModeFromRequestIgnoresStaleFinderCookie verifies that an old
// browser cookie cannot override an explicit Workbench startup selection.
//
// VALIDATES: Only the current switch cookie can override the startup mode.
// PREVENTS: Retired browser state overriding the selected interface.
func TestReadUIModeFromRequestIgnoresStaleFinderCookie(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/show/", http.NoBody)
	req.AddCookie(&http.Cookie{Name: uiModeCookie, Value: uiModeTokenFinder})

	assert.Equal(t, UIModeWorkbench, ReadUIModeFromRequest(req, UIModeWorkbench))
}

// TestReadUIModeFromRequestAcceptsCurrentSwitchCookie verifies that explicit
// UI switch controls can still select Finder or Workbench.
//
// VALIDATES: Operators can switch between Finder and Workbench intentionally.
// PREVENTS: CLI return navigation becoming one-way.
func TestReadUIModeFromRequestAcceptsCurrentSwitchCookie(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/show/", http.NoBody)
	req.AddCookie(&http.Cookie{Name: uiModeSwitchCookie, Value: uiModeTokenFinder})

	assert.Equal(t, UIModeFinder, ReadUIModeFromRequest(req, UIModeWorkbench))
}

// TestParseUIMode_KnownTokens verifies the parser recognizes both labels in
// any case and selects the graphical YANG editor for unknown values.
//
// VALIDATES: Robustness against operator typos and case variation.
// PREVENTS: An operator typo silently switching to the wrong UI.
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
			assert.Equal(t, tc.want, parseUIMode(tc.input))
		})
	}
}
