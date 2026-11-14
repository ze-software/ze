// Design: docs/features/interfaces.md -- Netlink interface monitor
// Related: monitor_linux.go -- resolveAddrLink, the producer of this report.
// Related: register_monitor_linux.go -- the code's registration.

//go:build linux

package ifacenetlink

import (
	"net/netip"
	"strconv"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// reportSourceIface is the report-bus source of the interface monitor.
const reportSourceIface = "iface"

// reportCodeIfaceAddressUnattributed is the code of an address the monitor
// could not attribute to an interface, on the report bus and on its WARN line.
const reportCodeIfaceAddressUnattributed = "doctor-iface-address-unattributed"

// ifaceAddressUnattributedDiagnosticCode describes the code for `ze show
// diagnostic`. The event-code registration in register_monitor_linux.go
// requires it to be registered first.
var ifaceAddressUnattributedDiagnosticCode = diagnostic.CodeMeta{
	Code:  reportCodeIfaceAddressUnattributed,
	Title: "An interface address was not announced: the monitor could not tell which interface holds it",
	Description: "The kernel reported an address on an interface the netlink monitor had not seen yet, and " +
		"asking the kernel for that interface failed. Ze did not announce the address, so the connected " +
		"route for it is missing from the system RIB and from redistribution, and no other consumer " +
		"(OSPF, the unnumbered donor) knows it, until the address changes. The subject is the address and " +
		"the interface index; the detail carries the kernel's error. Remedy: find the device with " +
		"`ip -o addr show to <address>`, then remove and add the address again on it.",
	Examples: []string{"ze show errors", "ze show diagnostic " + reportCodeIfaceAddressUnattributed},
}

// reportAddressUnattributed records an address lost to a failed link lookup.
// It is raised once per lost address and nothing clears it, because the
// monitor never learns of the address again unless it changes, so it is an
// error event and not a warning a later state can withdraw.
func reportAddressUnattributed(index int, prefix netip.Prefix, cause error) {
	address := prefix.String()
	subject := address + " index " + strconv.Itoa(index)
	fix := "ip -o addr show to " + address + "; ip addr del " + address + " dev <device>; ip addr add " +
		address + " dev <device>"
	logger().Warn("iface monitor: an address was not announced because the monitor could not look up its interface",
		"address", address, "index", index, "error", cause,
		"effect", "its connected route and other address consumers miss it until it changes",
		"fix", fix, "code", reportCodeIfaceAddressUnattributed)
	report.RaiseErrorFix(reportSourceIface, reportCodeIfaceAddressUnattributed, subject,
		report.FitMessage("Interface address "+address+" was not announced, so its connected route is missing: "+
			"the lookup of interface index "+strconv.Itoa(index)+" failed: "+cause.Error()),
		fix, map[string]any{"address": address, "index": index, "error": cause.Error()})
}
