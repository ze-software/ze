// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/iface/dhcp.

package ifacedhcp

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorIfaceDHCPEventEmit     = "doctor-iface-dhcp-event-emit"
	codeDoctorIfaceDHCPRemoveFailed  = "doctor-iface-dhcp-remove-failed"
	codeDoctorIfaceDHCPLeaseRegister = "doctor-iface-dhcp-lease-register"
	codeDoctorIfaceDHCPReleaseFailed = "doctor-iface-dhcp-release-failed"
	exampleShowDoctor                = "ze show doctor"
	exampleShowErrors                = "ze show errors"
	exampleShowInterface             = "ze show interface"
	reportKeyError                   = "error"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorIfaceDHCPEventEmit,
		Title:       "DHCP lease event could not be published on the EventBus",
		Description: "The interface DHCP client could not marshal or emit a lease acquisition, renewal, or expiry event on the EventBus. Consumers of DHCP lease events (such as delegated-prefix numbering or route updates) did not hear the lease change. Inspect interface leases with `ze show interface` and run `ze show doctor`.",
		Examples:    []string{exampleShowDoctor, exampleShowErrors, exampleShowInterface, "ze show diagnostic " + codeDoctorIfaceDHCPEventEmit},
	},
	{
		Code:        codeDoctorIfaceDHCPRemoveFailed,
		Title:       "DHCP-owned kernel address or route could not be removed",
		Description: "When a DHCPv4 or DHCPv6 lease expired, was declined, or stopped at shutdown, removing a Ze-owned leased address or default route from the kernel failed for a reason other than the object already being absent. The stale address or route may still remain in the kernel. Inspect the interface with `ip addr show dev <interface>` or `ip route show dev <interface>` and delete the stale entry if it is still present.",
		Examples:    []string{exampleShowDoctor, exampleShowErrors, exampleShowInterface, "ze show diagnostic " + codeDoctorIfaceDHCPRemoveFailed},
	},
	{
		Code:        codeDoctorIfaceDHCPLeaseRegister,
		Title:       "DHCPv6 leased-address ownership registration failed",
		Description: "After updating or dropping DHCPv6 bindings, updating the interface component's leased-address ownership registry failed. The interface address reconciler may hold a stale view of which addresses belong to DHCPv6 on this device. Inspect the interface with `ze show interface` and reapply with `ze request reload`.",
		Examples:    []string{exampleShowDoctor, exampleShowErrors, exampleShowInterface, "ze show diagnostic " + codeDoctorIfaceDHCPLeaseRegister},
	},
	{
		Code:        codeDoctorIfaceDHCPReleaseFailed,
		Title:       "DHCPv6 Release message could not be sent at shutdown",
		Description: "While stopping a DHCPv6 client that held an active lease, Ze could not construct the Release message or open the link socket to send it. Local addresses were removed, but the DHCPv6 server was not notified and will hold the lease until its valid lifetime expires. Inspect interface state with `ze show interface` and check link readiness with `ze show doctor`.",
		Examples:    []string{exampleShowDoctor, exampleShowErrors, exampleShowInterface, "ze show diagnostic " + codeDoctorIfaceDHCPReleaseFailed},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: ifacedhcp: doctor code registration refused: " + err.Error())
		}
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorIfaceDHCPEventEmit}); err != nil {
		panic("BUG: ifacedhcp: event-emit event code registration refused: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorIfaceDHCPRemoveFailed}); err != nil {
		panic("BUG: ifacedhcp: remove-failed event code registration refused: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorIfaceDHCPLeaseRegister}); err != nil {
		panic("BUG: ifacedhcp: lease-register event code registration refused: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorIfaceDHCPReleaseFailed}); err != nil {
		panic("BUG: ifacedhcp: release-failed event code registration refused: " + err.Error())
	}
}
