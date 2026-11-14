// Design: docs/architecture/doctor-and-health-checks.md -- event codes an owner raises on the report bus
// Related: session_write_budget.go -- reportWriteBudgetExpiry, which raises the code registered here

package reactor

import "github.com/ze-software/ze/internal/core/diagnostic"

// The reactor raises a write budget expiry on the report bus, so it declares
// the code to the recent-events doctor check, which then reports the event
// under that code in `show doctor`.
func init() {
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: diagnostic.CodeDoctorBGPPeerNotReading}); err != nil {
		panic("BUG: bgp reactor: write budget expiry event code registration failed: " + err.Error())
	}
}
