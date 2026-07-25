// Register the offline fallback for `show host` with the command registry.
// `show host [section]` is a daemon command (host-cmd plugin); this fallback
// serves the same hardware inventory in-process (host.DetectSection) when no
// daemon is reachable, so an operator can read it before the daemon is up.
// Imported by cmd/ze for its side effects.

// codegen:skip -- offline fallback wired via the command registry, not a runtime plugin.

package host

import (
	"github.com/ze-software/ze/internal/component/command/registry"
)

func init() {
	// Offline fallback only: consulted solely when the daemon is unreachable,
	// so it never shadows the daemon `show host` command. RunShow takes the
	// section as its first arg, matching the tokens after `show host`.
	registry.MustRegisterOfflineFallback("show host", RunShow)
}
