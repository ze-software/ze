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

// TestUIMode_DefaultsToWorkbench verifies that with no ze.web.ui-mode env var set,
// the hub defaults to the Workbench UI. Finder remains available only as an
// explicit rollback mode.
//
// VALIDATES: Default UI is Workbench.
// PREVENTS: Falling back to the old Finder shell on the main page.
func TestUIMode_DefaultsToWorkbench(t *testing.T) {
	t.Setenv("ze.web.ui-mode", "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	assert.Equal(t, UIModeWorkbench, GetUIMode())
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

// TestUIMode_RollbackFinder verifies that ze.web.ui-mode=finder selects Finder
// explicitly. During Phases 1-3 this is identical to the default; after the
// Phase 4 default flip this becomes the emergency rollback path.
//
// VALIDATES: AC-1a (explicit Finder rollback works).
// PREVENTS: The rollback switch being broken when the default flips.
func TestUIMode_RollbackFinder(t *testing.T) {
	t.Setenv("ze.web.ui-mode", "finder")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	assert.Equal(t, UIModeFinder, GetUIMode())
}

// TestReadUIModeFromRequestIgnoresStaleFinderCookie verifies that an old
// browser cookie cannot strand operators on the deprecated Finder shell.
//
// VALIDATES: Workbench cutover is not overridden by stale ze-ui=finder cookies.
// PREVENTS: Normal pages showing the old Finder layout and bottom CLI bar.
func TestReadUIModeFromRequestIgnoresStaleFinderCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/show/", http.NoBody)
	req.AddCookie(&http.Cookie{Name: uiModeCookie, Value: uiModeTokenFinder})

	assert.Equal(t, UIModeWorkbench, ReadUIModeFromRequest(req, UIModeWorkbench))
}

// TestReadUIModeFromRequestAcceptsCurrentSwitchCookie verifies that explicit
// UI switch controls can still select Finder or Workbench.
//
// VALIDATES: Operators can switch between Finder and Workbench intentionally.
// PREVENTS: CLI return navigation becoming one-way.
func TestReadUIModeFromRequestAcceptsCurrentSwitchCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/show/", http.NoBody)
	req.AddCookie(&http.Cookie{Name: uiModeSwitchCookie, Value: uiModeTokenFinder})

	assert.Equal(t, UIModeFinder, ReadUIModeFromRequest(req, UIModeWorkbench))
}

// TestParseUIMode_KnownTokens verifies the parser recognizes both labels in
// any case and falls back to Workbench for unknown values.
//
// VALIDATES: Robustness against operator typos and case variation.
// PREVENTS: An operator typo silently switching to the wrong UI.
func TestParseUIMode_KnownTokens(t *testing.T) {
	tests := []struct {
		input string
		want  UIMode
	}{
		{"", UIModeWorkbench},
		{"finder", UIModeFinder},
		{"Finder", UIModeFinder},
		{"FINDER", UIModeFinder},
		{"workbench", UIModeWorkbench},
		{"Workbench", UIModeWorkbench},
		{"WORKBENCH", UIModeWorkbench},
		{"unknown-mode", UIModeWorkbench},
		{"v2", UIModeWorkbench},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.want, parseUIMode(tc.input))
		})
	}
}
