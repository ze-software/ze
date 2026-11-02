// Design: docs/architecture/core-design.md -- request support archive command registration

package support

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	impl "github.com/ze-software/ze/internal/component/support"
)

// The command answers data, so the daemon serves the same handler under its
// YANG node for every remote surface: there the archive is kept in the
// daemon's store and read back with `show data cat <key>` (impl.Answer).
// Collecting runs programs and reads system state, so it is not read-only.
func init() {
	command.MustRegisterLocalData("request support archive", impl.Answer, registry.Meta{
		ShortHelp: "Collect logs, config, state and diagnostics into one archive.",
		Description: "Send the archive to support when you report an issue. Modules can be selected or " +
			"excluded, and a time window narrows what the archive holds.",
		Mode: "offline",
		// Locally the archive is a file under --output and the manifest names
		// its path; on the daemon it is a store key `show data cat` reads.
		// The timestamp, each module's duration and the stored size (which
		// only the daemon reports) differ on every run.
		Artifact: &registry.Artifact{
			LocalArgs:   "--json --output",
			LocalField:  "archive-path",
			DaemonField: "archive-key",
			Volatile:    []string{"timestamp", "duration-ms", "archive-octets"},
		},
	}, impl.Render)
}
