// Design: plan/learned/726-diag-crash-capture.md -- offline crash file CLI

// codegen:skip -- offline fallback wired via the command registry, not a runtime plugin.

package crashes

import (
	"os"

	"github.com/ze-software/ze/internal/component/command/registry"
)

func init() {
	// `show crashes [latest | name <file>]` is a daemon command (crashes-cmd).
	// When no daemon is reachable -- which is exactly when you inspect a crash,
	// since the daemon has died -- serve the same crash files in-process.
	// Registered as an offline fallback (never a plain local) so it does not
	// shadow the daemon command while the daemon is up.
	registry.MustRegisterOfflineFallback("show crashes", offlineShowCrashes)
}

// offlineShowCrashes adapts the daemon grammar (`show crashes [latest | name
// <file>]`) to RunShow, which takes the bare selector: `name <file>` selects one
// report, `latest` the newest, and no argument lists all.
func offlineShowCrashes(args []string) int {
	if len(args) > 0 && args[0] == "name" {
		if len(args) < 2 {
			os.Stderr.WriteString("error: 'show crashes name' requires a filename\n") //nolint:errcheck // CLI error
			return 1
		}
		return RunShow([]string{args[1]})
	}
	return RunShow(args)
}
