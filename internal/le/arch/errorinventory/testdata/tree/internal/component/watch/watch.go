// Fixture: the log site, a warning that names no doctor code.
package watch

import "log/slog"

// Watch logs the fixture's one warning.
func Watch(log *slog.Logger) {
	log.Warn("watch: peer flapping")
}
