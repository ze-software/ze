// Design: docs/architecture/doctor-and-health-checks.md -- plugin lifecycle checks
// Related: last_stop.go -- RecordStopCleanupLate, the record these write
// Related: reload.go -- stopTransaction, the config transaction writer
// Related: server.go -- cleanup, the plugin engine writer
// Related: ../process/manager.go -- ProcessManager.CleanupLate, the engines it saw late

package server

import (
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// stopOwnerConfigTransaction is the owner name the record carries for the
// config transaction, which is no plugin.
const stopOwnerConfigTransaction = "config transaction"

// stopTransactionLate logs and records a config transaction that did not
// unwind within grace, so the next local `ze doctor` reports it
// (doctor-plugin-stop-cleanup-late) after ze is down.
func stopTransactionLate(grace time.Duration) {
	logger().Warn("shutdown: config transaction did not unwind in time, closing plugin connections anyway",
		"code", diagnostic.CodeDoctorPluginStopCleanupLate, "grace", grace)
	var detail textbuf.Buffer
	detail.Str("the in-flight config transaction did not unwind within ").Str(grace.String()).
		Str(", so plugin connections were closed under it and a plugin may hold a half-applied change")
	RecordStopCleanupLate(stopOwnerConfigTransaction, detail.String())
}

// recordLateEngines records each plugin engine that did not finish its
// shutdown cleanup within the manager's grace. The manager logs the line
// with the code; this writes what the next run reports.
func recordLateEngines(plugins []string) {
	for _, name := range plugins {
		RecordStopCleanupLate(name, "the plugin engine did not finish its shutdown cleanup in time, "+
			"so resources it installed (routes, kernel policies, sockets) may be left behind")
	}
}
