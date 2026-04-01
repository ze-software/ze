// Design: docs/architecture/testing/ci-format.md — test runner framework
//
// Package functional provides a functional test runner with AI-friendly diagnostics.
package runner

import (
	"os"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// ANSI color codes.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[91m"
	colorGreen  = "\033[92m"
	colorYellow = "\033[93m"
	colorCyan   = "\033[96m"
	colorGray   = "\033[90m"
)

// Colors provides TTY-aware color formatting.
type Colors struct {
	enabled bool
}

// NewColors creates a Colors instance using the shared color detection.
// Respects NO_COLOR, TERM=dumb, and ze.log.color env vars.
func NewColors() *Colors {
	return &Colors{
		enabled: slogutil.UseColor(os.Stdout),
	}
}

// NewColorsWithOverride creates Colors with explicit enable/disable.
func NewColorsWithOverride(enabled bool) *Colors {
	return &Colors{enabled: enabled}
}

// Enabled returns true if colors are enabled.
func (c *Colors) Enabled() bool {
	return c.enabled
}

// Red formats text in red (errors, failures).
func (c *Colors) Red(s string) string {
	if !c.enabled {
		return s
	}
	return colorRed + s + colorReset
}

// Green formats text in green (success, expected values).
func (c *Colors) Green(s string) string {
	if !c.enabled {
		return s
	}
	return colorGreen + s + colorReset
}

// Yellow formats text in yellow (warnings, field names).
func (c *Colors) Yellow(s string) string {
	if !c.enabled {
		return s
	}
	return colorYellow + s + colorReset
}

// Cyan formats text in cyan (headers, labels).
func (c *Colors) Cyan(s string) string {
	if !c.enabled {
		return s
	}
	return colorCyan + s + colorReset
}

// Gray formats text in gray (de-emphasized).
func (c *Colors) Gray(s string) string {
	if !c.enabled {
		return s
	}
	return colorGray + s + colorReset
}

// Reset returns the reset sequence if colors are enabled.
func (c *Colors) Reset() string {
	if !c.enabled {
		return ""
	}
	return colorReset
}

// LineSeparator returns a colored line separator.
func (c *Colors) LineSeparator() string {
	return c.Cyan("───────────────────────────────────────────────────────────────────────────────")
}

// DoubleSeparator returns a double line separator.
func (c *Colors) DoubleSeparator() string {
	return c.Cyan("═══════════════════════════════════════════════════════════════════════════════")
}
