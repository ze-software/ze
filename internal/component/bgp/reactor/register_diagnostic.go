// Design: docs/architecture/doctor-and-health-checks.md -- event codes an owner raises on the report bus
// Related: session_write_budget.go -- reportWriteBudgetExpiry, which raises the code registered here

package reactor

import "github.com/ze-software/ze/internal/core/diagnostic"

// The reactor raises a write budget expiry, and a close behind a held write
// path, on the report bus, so it declares both codes to the recent-events
// doctor check, which then reports each event under its code in `show doctor`.
func init() {
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: diagnostic.CodeDoctorBGPPeerNotReading}); err != nil {
		panic("BUG: bgp reactor: write budget expiry event code registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: diagnostic.CodeDoctorBGPWritePathHeld}); err != nil {
		panic("BUG: bgp reactor: held write path event code registration failed: " + err.Error())
	}
}
