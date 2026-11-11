// Design: docs/architecture/cli/plugin-modes.md -- the registered CLI reaches the plugin runner.
// Related: register.go -- NTP's registered CLIHandler.

package ntp

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestNTPCLIEntryServesMetadata proves the registered CLI callback serves the
// shared plugin modes instead of terminating before the engine entry point.
func TestNTPCLIEntryServesMetadata(t *testing.T) {
	registration := registry.Lookup("ntp")
	if registration == nil {
		t.Fatal("NTP is not registered")
	}
	if registration.CLIHandler == nil {
		t.Fatal("NTP has no registered CLI handler")
	}
	for _, flag := range []string{"--features", "--yang"} {
		t.Run(flag, func(t *testing.T) {
			if code := registration.CLIHandler([]string{flag}); code != 0 {
				t.Fatalf("ze plugin ntp %s exit=%d, want metadata success", flag, code)
			}
		})
	}
}
