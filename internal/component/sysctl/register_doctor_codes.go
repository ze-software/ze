// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/sysctl.

package sysctl

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorSysctlEventEmit      = "doctor-sysctl-event-emit"
	codeDoctorSysctlOriginalUnread = "doctor-sysctl-original-unread"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        doctorSysctlProcfsCode,
		Title:       "Sysctl key not writable or undetermined",
		Description: "Sysctl settings are configured, and this process cannot write one or more configured /proc/sys key files (net.* keys need CAP_NET_ADMIN or root; other keys need root; /proc/sys may be read-only) or cannot determine whether they are writable.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-sysctl-procfs"},
	},
	{
		Code:        codeDoctorSysctlEventEmit,
		Title:       "Sysctl EventBus notification could not be emitted",
		Description: "The sysctl plugin applied a kernel tunable or answered a query, then failed to emit the result event on the EventBus. Subscribing plugins or query callers did not receive the notification even though the kernel write completed. Verify active tunables with `ze show sysctl` and inspect daemon health with `ze show doctor`.",
		Examples:    []string{"ze show doctor", "ze show errors", "ze show sysctl", "ze show diagnostic " + codeDoctorSysctlEventEmit},
	},
	{
		Code:        codeDoctorSysctlOriginalUnread,
		Title:       "Original kernel sysctl value could not be read before writing",
		Description: "Before modifying a kernel tunable for the first time, the sysctl plugin could not read the pre-existing value from the OS. The new value is still written, but Ze has no saved original value to restore when this tunable is cleared or when Ze stops cleanly. Inspect active tunables with `ze show sysctl` and check `/proc/sys` permissions with `ze show doctor`.",
		Examples:    []string{"ze show doctor", "ze show errors", "ze show sysctl", "ze show diagnostic " + codeDoctorSysctlOriginalUnread},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: sysctl: doctor code registration refused: " + err.Error())
		}
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorSysctlEventEmit}); err != nil {
		panic("BUG: sysctl: event-emit event code registration refused: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorSysctlOriginalUnread}); err != nil {
		panic("BUG: sysctl: original-unread event code registration refused: " + err.Error())
	}
}
