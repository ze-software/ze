// Design: docs/architecture/doctor-and-health-checks.md -- plugin external blocks the daemon cannot start
// Related: ../../../component/plugin/doctor/check_external.go -- checkExternalInProcess, the check this registration drives

// Package fakeinprocess registers a test-only plugin declared in-process
// only, so test/plugin/doctor-plugin-external-unsupported.ci can configure it
// as `plugin external` and see `ze doctor` report the block. It is never
// started: the doctor check reads the registration alone.
package fakeinprocess

import (
	"net"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// Name is the registry name the functional test configures.
const Name = "fake-in-process-only"

func init() {
	reg := registry.Registration{
		Name:              Name,
		Description:       "Test-only plugin declared in-process only (doctor-plugin-external-unsupported)",
		RunEngine:         runPlugin,
		RunsInProcessOnly: true,
		// This fixture deliberately has no external execution mode.
		CLIHandler: func(_ []string) int { return 1 },
	}
	if err := registry.Register(reg); err != nil {
		panic("BUG: " + Name + " registration failed: " + err.Error())
	}
}

// runPlugin closes the connection and exits: no test starts this plugin.
func runPlugin(conn net.Conn) int {
	conn.Close() //nolint:errcheck,gosec // nothing is served on it
	return 0
}
