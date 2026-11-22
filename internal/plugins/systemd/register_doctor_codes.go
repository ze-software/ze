// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/systemd.

package systemd

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorServiceExecutable = "doctor-service-executable"
	codeDoctorServiceGroup      = "doctor-service-group"
	codeDoctorServiceRestart    = "doctor-service-restart"
	codeDoctorServiceStop       = "doctor-service-stop"
	codeDoctorServiceUnit       = "doctor-service-unit"
	codeDoctorServiceUser       = "doctor-service-user"
	exampleDoctorJSON           = "ze doctor --json"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorServiceExecutable,
		Title:       "Systemd service executable invalid",
		Description: "The installed ze systemd unit ExecStart or ExecReload runs a program that is not an existing executable file.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-service-executable"},
	},
	{
		Code:        codeDoctorServiceGroup,
		Title:       "Systemd service group missing",
		Description: "The group configured in the ze systemd unit does not exist on this host.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-service-group"},
	},
	{
		Code:  codeDoctorServiceRestart,
		Title: "Systemd policy cannot preserve daemon restart",
		Description: "An intentional in-place daemon restart exits 75 after teardown. The generated unit marks that exit successful and forces restart, while clean stop exits 0 and failures exit 1. " +
			"Doctor reads the effective Restart, SuccessExitStatus, RestartForceExitStatus and RestartPreventExitStatus properties, including drop-ins, and warns when this contract is missing or cannot be read. " +
			"Run `sudo ze install systemd --force` and `sudo systemctl daemon-reload`, then remove overriding drop-ins shown by `systemctl cat ze.service`.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-service-restart"},
	},
	{
		Code:  codeDoctorServiceStop,
		Title: "Systemd stop settings cut the stop drain short",
		Description: "On a stop, ze drains its routing protocols for up to 60 s and then tears down for up to 30 s, so systemd must wait at least 90 s before its SIGKILL and must send the stop SIGTERM to ze alone. " +
			"Doctor reads the effective values with `systemctl show -p TimeoutStopUSec -p KillMode ze.service`, so a drop-in or a distro default counts. " +
			"A TimeoutStopSec below 90 s lets systemd kill ze mid-drain; a KillMode other than mixed sends SIGTERM to every external plugin before ze can drain it. " +
			"Run `sudo ze install systemd --force` and `sudo systemctl daemon-reload`, and remove any drop-in `systemctl cat ze.service` shows overriding either setting.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-service-stop"},
	},
	{
		Code:        codeDoctorServiceUnit,
		Title:       "Systemd service unit unreadable",
		Description: "The installed ze systemd unit exists but cannot be read by doctor.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-service-unit"},
	},
	{
		Code:        codeDoctorServiceUser,
		Title:       "Systemd service user missing",
		Description: "The user configured in the ze systemd unit does not exist on this host.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-service-user"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: systemd: doctor code registration refused: " + err.Error())
		}
	}
}
