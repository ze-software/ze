// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/crashes.

package crashes

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorCrashCapturePstore       = "doctor-crash-capture-pstore"
	codeDoctorCrashCaptureUnarmed      = "doctor-crash-capture-unarmed"
	codeDoctorCrashDirectoryUnwritable = "doctor-crash-directory-unwritable"
	exampleDoctorJSON                  = "ze doctor --json"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorCrashCapturePstore,
		Title:       "Kernel crash record store unreadable",
		Description: "The reserved region is present on the kernel command line, but the pstore filesystem the kernel exposes the record through cannot be read. Either the running kernel was built without CONFIG_PSTORE and CONFIG_PSTORE_RAM, or mounting pstore on /sys/fs/pstore was refused. The kernel takes the reserved memory on every boot either way, so this reports RAM spent for a record nothing can read back.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-crash-capture-pstore"},
	},
	{
		Code:        codeDoctorCrashCaptureUnarmed,
		Title:       "Kernel crash capture configured but not armed",
		Description: "system crash-dump enabled is set, but the running kernel booted without the reserved memory region the capture writes into. A reservation is a kernel boot argument, so a commit records the intent and the next boot arms it: build the appliance image with image.crash-dump and reboot. Until then a kernel panic leaves no record, and the appliance has no shell to diagnose one with.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-crash-capture-unarmed"},
	},
	{
		Code:        codeDoctorCrashDirectoryUnwritable,
		Title:       "Crash directory not writable",
		Description: "No candidate crash directory could be created and written, so Ze has nowhere to store a crash report. The probe order is the ze.crash.dir override, then the config directory's crash subdirectory, then the three built-in candidates. Without one, a Go panic report is lost and a harvested kernel record is kept in the reserved region for the next boot instead of being stored.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-crash-directory-unwritable"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: crashes: doctor code registration refused: " + err.Error())
		}
	}
}
