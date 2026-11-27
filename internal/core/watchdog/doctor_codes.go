// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/core/watchdog
// Related: register.go -- package init registration
// Related: watchdog.go -- liveness watchdog that raises CodeDoctorDaemonStallDetected

package watchdog

import "github.com/ze-software/ze/internal/core/diagnostic"

// CodeDoctorDaemonStallDetected is the doctor diagnostic code raised when the
// daemon liveness watchdog observes scheduler wake-up drift or a stalled
// subsystem probe.
const CodeDoctorDaemonStallDetected = "doctor-daemon-stall-detected"

func registerDoctorCodes() {
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code:  CodeDoctorDaemonStallDetected,
		Title: "Daemon liveness watchdog detected a stall",
		Description: "The always-on daemon liveness watchdog detected scheduler wake-up drift exceeding its threshold or a registered subsystem probe reporting a stall (such as a long-held configuration transaction lock or active hung command handlers). " +
			"It captured a bounded goroutine stack snapshot and triggered an automatic flight-recorder trace capture under its cooldown window. " +
			"Run `ze show system watchdog` and `ze show system goroutines mode blocked` to inspect the captured stall record and blocked goroutines.",
		Examples: []string{
			"ze show doctor",
			"ze show errors",
			"ze show system watchdog",
			"ze show diagnostic doctor-daemon-stall-detected",
		},
	}); err != nil {
		panic("BUG: watchdog: doctor code registration refused: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: CodeDoctorDaemonStallDetected}); err != nil {
		panic("BUG: watchdog: event code registration refused: " + err.Error())
	}
}
