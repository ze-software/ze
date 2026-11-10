// Package tool is a fixture: development tooling, outside the site roots.
package tool

import "log/slog"

// Run logs a warning with no doctor code; the scan never reads it.
func Run(log *slog.Logger) {
	log.Warn("tool: not a product site")
}
