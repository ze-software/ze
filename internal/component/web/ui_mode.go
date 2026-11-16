// Design: docs/architecture/web-interface.md -- UI mode selection
// Related: handler.go -- URL routing
// Related: render.go -- Template rendering

package web

import (
	"net/http"
	"strings"

	"github.com/ze-software/ze/internal/core/env"
)

// UIMode selects which web UI the hub serves.
type UIMode int

const (
	// UIModeFinder serves the established Finder columns UI.
	UIModeFinder UIMode = iota
	// UIModeWorkbench serves the unfinished experimental operator workbench.
	UIModeWorkbench
)

const (
	// uiModeCookie is the retired cookie name. Only the current switch cookie
	// records an explicit choice, so stale browser state cannot override startup.
	uiModeCookie = "ze-ui"
	// uiModeSwitchCookie is written only by the current UI switch controls.
	uiModeSwitchCookie = "ze-ui-mode"

	uiModeTokenFinder    = "finder"
	uiModeTokenWorkbench = "workbench"
	// uiModeTokenCLI is not a selectable UI mode. It is the LayoutData.ActiveUI
	// value the /cli terminal page sets, and pageLayout reads it to leave the
	// commit bar out of that page.
	uiModeTokenCLI = "cli"
)

// String returns the canonical token for the mode.
func (m UIMode) String() string {
	switch m {
	case UIModeFinder:
		return uiModeTokenFinder
	case UIModeWorkbench:
		return uiModeTokenWorkbench
	default:
		panic("BUG: unknown normalized web UI mode")
	}
}

// parseUIMode converts a token to a UIMode. Unknown or empty values
// select the graphical YANG editor. The unfinished Workbench requires an
// explicit selection.
func parseUIMode(s string) UIMode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case uiModeTokenWorkbench:
		return UIModeWorkbench
	default:
		return UIModeFinder
	}
}

// GetUIMode reads ze.web.ui-mode from the env registry and returns the startup
// mode. Finder is the graphical YANG editor; Workbench is an explicit experiment.
func GetUIMode() UIMode {
	return parseUIMode(env.Get("ze.web.ui-mode"))
}

// ReadUIModeFromRequest checks the current UI switch cookie before falling back
// to the server-selected startup mode. The legacy ze-ui cookie is deliberately
// ignored; the current switch cookie preserves an operator's explicit choice.
func ReadUIModeFromRequest(r *http.Request, fallback UIMode) UIMode {
	c, err := r.Cookie(uiModeSwitchCookie)
	if err != nil || c.Value == "" {
		return fallback
	}
	return parseUIMode(c.Value)
}
