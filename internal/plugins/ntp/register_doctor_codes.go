// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/ntp.

package ntp

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorNTPClockSyncedEmitFailed = "doctor-ntp-clock-synced-emit-failed"
	codeDoctorNTPRTCWriteFailed        = "doctor-ntp-rtc-write-failed"
	codeDoctorNTPTimePersistFailed     = "doctor-ntp-time-persist-failed"
	codeDoctorNTPDHCPEventMalformed    = "doctor-ntp-dhcp-event-malformed"

	exampleDoctorJSON     = "ze doctor --json"
	exampleShowWarnings   = "ze show warnings"
	exampleShowErrors     = "ze show errors"
	exampleShowDiagPrefix = "ze show diagnostic "
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeClockNoSync,
		Title:       "No clock synchronization configured",
		Description: "Ze NTP is disabled on a platform where clock synchronization must be provided by Ze or explicitly verified externally.",
		Examples:    []string{exampleDoctorJSON, exampleShowDiagPrefix + codeClockNoSync},
	},
	{
		Code:        codeNTPClockPrivilege,
		Title:       "NTP clock adjustment privilege missing",
		Description: "NTP is configured but the process lacks CAP_SYS_TIME. Clock adjustment will fail unless running as root or with the capability granted.",
		Examples:    []string{exampleDoctorJSON, exampleShowDiagPrefix + codeNTPClockPrivilege},
	},
	{
		Code:        codeNTPServerUnreachable,
		Title:       "NTP servers unreachable",
		Description: "None of the configured NTP servers responded to a reachability probe. Clock synchronization may fail.",
		Examples:    []string{exampleDoctorJSON, exampleShowDiagPrefix + codeNTPServerUnreachable},
	},
	{
		Code:        codeDoctorNTPClockSyncedEmitFailed,
		Title:       "NTP could not emit the clock-synced event",
		Description: "The NTP client synchronized the system clock, but publishing the ntp:clock-synced event on the event bus failed. Downstream plugins waiting for initial clock synchronization may not learn that the clock is ready. Check daemon event bus health with `ze show doctor`.",
		Examples:    []string{exampleDoctorJSON, exampleShowErrors, exampleShowDiagPrefix + codeDoctorNTPClockSyncedEmitFailed},
	},
	{
		Code:        codeDoctorNTPRTCWriteFailed,
		Title:       "NTP could not update the hardware RTC",
		Description: "The NTP client synchronized the system clock, but writing the synced time to /dev/rtc0 failed. Hardware clock drift will persist across reboots until the RTC device is writable and healthy. Check /dev/rtc0 permissions and kernel driver status with `ze show doctor`.",
		Examples:    []string{exampleDoctorJSON, exampleShowWarnings, exampleShowDiagPrefix + codeDoctorNTPRTCWriteFailed},
	},
	{
		Code:        codeDoctorNTPTimePersistFailed,
		Title:       "NTP could not persist the last-known clock time",
		Description: "The NTP client synchronized the system clock, but writing the last-known timestamp to the daemon managed state store failed. Devices without a battery-backed RTC will boot with a stale timestamp until persistence succeeds. Check store health with `ze show doctor`.",
		Examples:    []string{exampleDoctorJSON, exampleShowWarnings, exampleShowDiagPrefix + codeDoctorNTPTimePersistFailed},
	},
	{
		Code:        codeDoctorNTPDHCPEventMalformed,
		Title:       "NTP received a malformed DHCP lease event",
		Description: "The NTP client could not decode a DHCP lease event payload on the event bus, so NTP servers advertised via DHCP option 42 in that event were ignored. Check interface DHCP client logs and event bus integrity with `ze show system ntp` and `ze show doctor`.",
		Examples:    []string{exampleDoctorJSON, exampleShowErrors, exampleShowDiagPrefix + codeDoctorNTPDHCPEventMalformed},
	},
}

// doctorEventCodes declares the transient report-bus event codes this package raises.
var doctorEventCodes = []diagnostic.EventCode{
	{Code: codeDoctorNTPClockSyncedEmitFailed},
	{Code: codeDoctorNTPRTCWriteFailed},
	{Code: codeDoctorNTPTimePersistFailed},
	{Code: codeDoctorNTPDHCPEventMalformed},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: ntp: doctor code registration refused: " + err.Error())
		}
	}
	for _, ev := range doctorEventCodes {
		if err := diagnostic.RegisterEventCode(ev); err != nil {
			panic("BUG: ntp: doctor event code registration refused: " + err.Error())
		}
	}
}
