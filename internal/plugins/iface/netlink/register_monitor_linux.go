// Design: docs/features/interfaces.md -- Netlink interface monitor
// Related: monitor_report_linux.go -- the code this file registers.

//go:build linux

package ifacenetlink

import "github.com/ze-software/ze/internal/core/diagnostic"

// The unattributed-address error (monitor_report_linux.go) travels with the
// netlink backend, so removing this package removes the code. It stays out of
// the iface health row: it is raised once per lost address and nothing clears
// it, so a health verdict built on it would stay down for the daemon's life.
func init() {
	if err := diagnostic.Register(ifaceAddressUnattributedDiagnosticCode); err != nil {
		panic("BUG: register iface-netlink unattributed-address diagnostic code: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: reportCodeIfaceAddressUnattributed}); err != nil {
		panic("BUG: register iface-netlink unattributed-address event code: " + err.Error())
	}
}
