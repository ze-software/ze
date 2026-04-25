// Design: docs/architecture/web-interface.md -- UI mode selection (V2 workbench experiment)
// Related: handler.go -- URL routing
// Related: render.go -- Template rendering
//
// Spec: plan/spec-web-2-operator-workbench.md (UI Mode Contract).
//
// The hub reads `ze.web.ui` once at startup. Through Phases 1-3 the default is
// `finder` and `workbench` is opt-in for development. After the Phase 4
// Promotion Criteria pass, the default flips to `workbench` and `finder`
// becomes the emergency rollback. Phase 7 removes the variable entirely.

package web

import (
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

// String returns the canonical token for the mode.
func (m UIMode) String() string {
	switch m {
	case UIModeWorkbench:
		return "workbench"
	default:
		return "finder"
	}
}

// ParseUIMode converts the ze.web.ui token to a UIMode. Unknown or empty
// values fall back to Finder; the default is encoded here, not at the call
// site, so every reader of the mode agrees.
func ParseUIMode(s string) UIMode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "workbench":
		return UIModeWorkbench
	default:
		return UIModeFinder
	}
}

// GetUIMode reads ze.web.ui from the env registry and returns the selected
// mode. Call once at hub startup; flipping the variable later requires a
// hub restart by design (single active UI per process).
func GetUIMode() UIMode {
	return ParseUIMode(env.Get("ze.web.ui"))
}
