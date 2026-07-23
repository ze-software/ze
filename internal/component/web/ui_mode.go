// Design: docs/architecture/web-interface.md -- UI mode selection
// Related: handler.go -- URL routing
// Related: render.go -- Template rendering

package web

import (
	"net/http"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

// UIMode selects which web UI the hub serves.
type UIMode int

const (
	// UIModeFinder serves the established Finder columns UI.
	UIModeFinder UIMode = iota
	// UIModeWorkbench serves the experimental RouterOS-style operator workbench.
	UIModeWorkbench
)

const (
	// uiModeCookie is the legacy cookie name. It is ignored during the
	// Workbench cutover so stale Finder cookies cannot select the old shell.
	uiModeCookie = "ze-ui"
	// uiModeSwitchCookie is written only by the current UI switch controls.
	uiModeSwitchCookie = "ze-ui-mode"

	uiModeTokenFinder    = "finder"
	uiModeTokenWorkbench = "workbench"
)

// String returns the canonical token for the mode.
func (m UIMode) String() string {
	switch m {
	case UIModeFinder:
		return uiModeTokenFinder
	default:
		return uiModeTokenWorkbench
	}
}

// ParseUIMode converts a token to a UIMode. Unknown or empty values
// fall back to Workbench. Finder remains available as an explicit rollback.
func ParseUIMode(s string) UIMode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case uiModeTokenFinder:
		return UIModeFinder
	default:
		return UIModeWorkbench
	}
}

// GetUIMode reads ze.web.ui-mode from the env registry and returns the startup
// mode. Workbench is the normal UI; Finder is only an explicit rollback.
func GetUIMode() UIMode {
	return ParseUIMode(env.Get("ze.web.ui-mode"))
}

// ReadUIModeFromRequest checks the current UI switch cookie before falling back
// to the server-selected startup mode. The legacy ze-ui cookie is deliberately
// ignored so old browser state cannot strand operators on Finder.
func ReadUIModeFromRequest(r *http.Request, fallback UIMode) UIMode {
	c, err := r.Cookie(uiModeSwitchCookie)
	if err != nil || c.Value == "" {
		return fallback
	}
	return ParseUIMode(c.Value)
}
