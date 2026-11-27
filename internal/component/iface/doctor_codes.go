// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/iface.

package iface

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorIfaceSelectorUnmatched = "doctor-iface-selector-unmatched"
	codeDoctorIfaceSelectorAmbiguous = "doctor-iface-selector-ambiguous"
	// CodeDoctorIfaceBackendClose is the doctor diagnostic code "doctor-iface-backend-close".
	CodeDoctorIfaceBackendClose = "doctor-iface-backend-close"
	// CodeDoctorIfaceOwnedDeviceCleanup is the doctor diagnostic code "doctor-iface-owned-device-cleanup".
	CodeDoctorIfaceOwnedDeviceCleanup = "doctor-iface-owned-device-cleanup"
	// CodeDoctorIfaceOwnedDeviceReconcile is the doctor diagnostic code "doctor-iface-owned-device-reconcile".
	CodeDoctorIfaceOwnedDeviceReconcile = "doctor-iface-owned-device-reconcile"
	// CodeDoctorIfaceStateUnread is the doctor diagnostic code "doctor-iface-state-unread".
	CodeDoctorIfaceStateUnread       = "doctor-iface-state-unread"
	codeDoctorIfaceStateUnread       = CodeDoctorIfaceStateUnread
	codeDoctorIfaceAddressAdd        = "doctor-iface-address-add"
	codeDoctorIfaceSysctlEmit        = "doctor-iface-sysctl-emit"
	codeDoctorIfaceRollbackEmit      = "doctor-iface-rollback-emit"
	codeDoctorIfaceStaleRouteCleanup = "doctor-iface-stale-route-cleanup"
	exampleDoctorJSON                = "ze doctor --json"
	exampleShowDoctor                = "ze show doctor"
	exampleShowWarnings              = "ze show warnings"
	exampleShowErrors                = "ze show errors"
	exampleShowInterface             = "ze show interface"
	reportKeyError                   = "error"
)

// The codes the check emits. internal/core/diagnostic/codes.go declares each
// one, so `ze show diagnostic <code>` answers. Each is its own declaration: one code
// is one fact, and a block of them reads as a second list of the code
// registry (./le arch enumeration check).

// codeIfaceMissing names a configured ethernet device sysfs does not hold.
const codeIfaceMissing = "doctor-iface-missing"

// codeIfaceDown names a configured ethernet device whose operstate is down.
const codeIfaceDown = "doctor-iface-down"

// codeIfaceMACOverrideByName names an entry that overrides a MAC on a device it
// reaches by name.
const codeIfaceMACOverrideByName = "doctor-iface-mac-override-by-name"

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeIfaceMissing,
		Title:       "Configured interface not found",
		Description: "An ethernet interface named in the config does not exist on the system.",
		Examples:    []string{exampleDoctorJSON},
	},
	{
		Code:        codeDoctorIfaceSelectorUnmatched,
		Title:       "Interface hardware selector matches no device",
		Description: "An ethernet interface binds to hardware by mac/match, and no device on this system carries that address. The binding stays deferred until one appears, so the interface is left unconfigured.",
		Examples:    []string{exampleDoctorJSON},
	},
	{
		Code:        codeDoctorIfaceSelectorAmbiguous,
		Title:       "Interface hardware selector matches several devices",
		Description: "An ethernet interface binds to hardware by mac/match, and more than one device carries that address. Nothing distinguishes them, so the config apply refuses rather than guessing which port the interface's addresses reach.",
		Examples:    []string{exampleDoctorJSON},
	},
	{
		Code:        codeIfaceMACOverrideByName,
		Title:       "Interface MAC override is bound by name",
		Description: "An ethernet interface overrides its hardware address with mac/address but reaches its device by name. The address is written to whichever device holds that name, so a kernel rename hands one NIC's address to another. Bind the entry with mac/match against the NIC's permanent address instead.",
		Examples:    []string{exampleDoctorJSON},
	},
	{
		Code:        codeIfaceDown,
		Title:       "Configured interface is down",
		Description: "An ethernet interface named in the config exists but its link is not up.",
		Examples:    []string{exampleDoctorJSON},
	},
	{
		Code:        CodeDoctorIfaceBackendClose,
		Title:       "Interface backend did not close cleanly",
		Description: "When the interface component stopped, closing its backend (the netlink sockets, or the connection to VPP) failed. The kernel or VPP keeps what was programmed; only the backend's own handles may not have been released. The next start opens a fresh backend. The event names the backend's answer; a repeat on every stop points at the backend itself, such as a VPP that no longer answers.",
		Examples:    []string{exampleShowDoctor, "ze show errors", "ze show diagnostic doctor-iface-backend-close"},
	},
	{
		Code:        CodeDoctorIfaceOwnedDeviceCleanup,
		Title:       "Owned interface devices of an ended plugin were not all deleted",
		Description: "A plugin that asked the interface component for owned bridges and links ended, or the interface component stopped, and the deletion of those devices failed for at least one of them. The devices carry the link alias ze:owned:<plugin>, so `ip -d link show` lists what remains. The next interface reconcile (any interface commit, or the plugin declaring its set again) deletes a marked device no registration names, and so does the next ze start; the finding clears when such a pass succeeds. The finding and the log line name the kernel's answer.",
		Examples:    []string{exampleShowDoctor, "ip -d link show", "ze show diagnostic doctor-iface-owned-device-cleanup"},
	},
	{
		Code:        CodeDoctorIfaceOwnedDeviceReconcile,
		Title:       "Owned interface devices do not match what their plugins asked for",
		Description: "A kernel step for a plugin-owned bridge or link failed and could not be undone. Either a bridge set a plugin declared was refused and the previous set could not be made again, so a bridge the plugin still counts on is missing or wrong, or a device created half-way could not be deleted, so it holds a name the plugin asked for. The finding names the plugin or the device and the kernel's answer. Restarting the plugin declares its set again, which reconciles it; a half-made device is deleted by name. The finding clears when an owned pass succeeds and the device is gone.",
		Examples:    []string{exampleShowDoctor, "ip -d link show", "ze show diagnostic doctor-iface-owned-device-reconcile"},
	},
	{
		Code:        codeDoctorIfaceStateUnread,
		Title:       "Interface state not read before its configuration was applied",
		Description: "Before it writes a link setting (MTU, admin state, bridge STP, bridge membership, VLAN QoS maps) or the keys and peers of a WireGuard device it did not just create, ze reads the device's current state and writes only what differs. For this interface the read failed, so ze wrote the configured values without knowing whether they were already in place. The warning names the interface (`<interface> wireguard` for a WireGuard read) and the backend's answer, which is the cause; on Linux a WireGuard read without CAP_NET_ADMIN is refused, as the write is. The apply itself reports any write that fails, so a configuration that applied is in place. Run `ze show interface name <interface> detail` to read the device's state again; the warning clears when a later apply reads that interface's state, and at the next ze start. A backend that is not ready yet, or a device the read does not find by its name, raises no warning.",
		Examples:    []string{exampleShowDoctor, exampleShowWarnings, "ze show interface name eth0 detail", "ze show diagnostic doctor-iface-state-unread"},
	},
	{
		Code:        codeDoctorIfaceAddressAdd,
		Title:       "Configured interface address could not be added during deferred reconcile",
		Description: "While full interface reconciliation was deferred because the backend was still connecting, the additive fallback attempted to install a configured IP address and the backend refused it for a reason other than not being ready. The configured address is absent from the device until a later apply or reconciliation succeeds. Inspect the interface with `ze show interface name <interface> detail` and reapply the configuration with `ze request reload`.",
		Examples:    []string{exampleShowDoctor, exampleShowErrors, "ze show interface name eth0 detail", "ze show diagnostic " + codeDoctorIfaceAddressAdd},
	},
	{
		Code:        codeDoctorIfaceSysctlEmit,
		Title:       "Interface sysctl default could not be published to the sysctl plugin",
		Description: "The interface component failed to publish a per-interface sysctl setting or profile change on the EventBus. The sysctl plugin did not receive the tunable update, so the kernel setting on the named interface may remain at its previous value. Inspect active kernel tunables with `ze show sysctl` and reapply with `ze request reload`.",
		Examples:    []string{exampleShowDoctor, exampleShowErrors, "ze show sysctl", "ze show diagnostic " + codeDoctorIfaceSysctlEmit},
	},
	{
		Code:        codeDoctorIfaceRollbackEmit,
		Title:       "Interface rollback notification could not be published",
		Description: "After rolling back a failed interface configuration transaction, the interface component failed to emit the `interface:rollback` event on the EventBus. Downstream plugins that react to interface rollbacks did not receive the notification and may hold stale interface state. Inspect interface state with `ze show interface` and run `ze request reload` to resynchronize consumers.",
		Examples:    []string{exampleShowDoctor, exampleShowErrors, exampleShowInterface, "ze show diagnostic " + codeDoctorIfaceRollbackEmit},
	},
	{
		Code:        codeDoctorIfaceStaleRouteCleanup,
		Title:       "Stale kernel IPv6 default routes could not be listed for cleanup",
		Description: "Before suppressing kernel Router Advertisement default routes on an interface, ze attempted to list pre-existing `::/0` routes so they could be removed, and the backend lookup failed. Pre-existing kernel default routes may remain beside Ze-managed routes with different metrics. Inspect routes with `ip -6 route show default dev <interface>` and remove any stale kernel default route.",
		Examples:    []string{exampleShowDoctor, exampleShowErrors, exampleShowInterface, "ze show diagnostic " + codeDoctorIfaceStaleRouteCleanup},
	},
}

// registerDoctorCodes registers this package's doctor diagnostic codes.
func registerDoctorCodes() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: iface: doctor code registration refused: " + err.Error())
		}
	}
}
