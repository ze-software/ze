package fixture

import (
	"log/slog"
	"testing"
)

// TestNothing logs a warning with no doctor code; a test file is no site.
func TestNothing(t *testing.T) {
	slog.Warn("fixture test: not a site")
}
