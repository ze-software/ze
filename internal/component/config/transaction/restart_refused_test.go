package transaction

import (
	"fmt"
	"strings"
	"testing"
)

// TestRefusedRestartDoesNotClaimPluginLiveness keeps a refusal separate from
// the state of a plugin that may have exited before the restart was attempted.
func TestRefusedRestartDoesNotClaimPluginLiveness(t *testing.T) {
	err := fmt.Errorf("daemon stopping: %w", ErrRestartRefused)
	got := brokenRollbackLine("stopped-plugin", "listener lost", map[string]error{
		"stopped-plugin": err,
	})
	for _, want := range []string{
		"stopped-plugin could not put its running config back (listener lost)",
		"ze refused to restart it (daemon stopping: ze refused to restart the plugin)",
		"check its status and log lines before retrying the reload: ze show system subsystem list",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rollback outcome %q does not contain %q", got, want)
		}
	}
	for _, unsupported := range []string{"still runs", "is not running"} {
		if strings.Contains(got, unsupported) {
			t.Errorf("rollback outcome %q claims liveness from a refusal", got)
		}
	}
}
