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
	// UIModeWorkbench serves the V2 RouterOS-style operator workbench.
	UIModeWorkbench
)

// uiModeCookie is the cookie name used to persist the user's UI preference.
const uiModeCookie = "ze-ui"

// String returns the canonical token for the mode.
func (m UIMode) String() string {
	switch m {
	case UIModeFinder:
		return "finder"
	default:
		return "workbench"
	}
}

// ParseUIMode converts a token to a UIMode. Unknown or empty values
// fall back to workbench (the default UI).
func ParseUIMode(s string) UIMode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "finder":
		return UIModeFinder
	default:
		return UIModeWorkbench
	}
}

// GetUIMode reads ze.web.ui from the env registry and returns the
// startup default. Both UIs are always available; this only controls
// which one /show/ renders when no cookie override is set.
func GetUIMode() UIMode {
	return ParseUIMode(env.Get("ze.web.ui"))
}

// ReadUIModeFromRequest checks the ze-ui cookie for a per-user override,
// falling back to the startup default.
func ReadUIModeFromRequest(r *http.Request, fallback UIMode) UIMode {
	c, err := r.Cookie(uiModeCookie)
	if err != nil || c.Value == "" {
		return fallback
	}
	return ParseUIMode(c.Value)
}
