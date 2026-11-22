// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/config/archive.

package archive

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	// CodeDoctorArchiveScheduledFailed is the doctor diagnostic code "doctor-archive-scheduled-failed".
	CodeDoctorArchiveScheduledFailed = "doctor-archive-scheduled-failed"
	exampleDoctorJSON                = "ze doctor --json"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeArchiveUnreachable,
		Title:       "Archive destination unreachable",
		Description: "A configured HTTP/HTTPS config archive destination did not respond to an HTTP HEAD probe within the timeout.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-archive-unreachable"},
	},
	{
		Code:        codeArchiveRedirected,
		Title:       "Archive destination redirects",
		Description: "A configured HTTP/HTTPS config archive destination answered the HTTP HEAD probe with a redirect. Ze follows no redirect when it uploads the config, because the upload carries the config's reversible $9$ secrets, so every upload to that location fails. Set the archive location to the URL the redirect names, as the finding's fix line shows.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-archive-redirected"},
	},
	{
		Code:        codeArchiveInvalid,
		Title:       "Archive entry invalid",
		Description: "A system archive entry holds a value the uploader cannot use, such as a timeout that is not a positive Go duration, so no destination of the config was probed. Correct the entry the message names; `ze config validate` reports the same refusal.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-archive-invalid"},
	},
	{
		Code:        CodeDoctorArchiveScheduledFailed,
		Title:       "Scheduled archive failed",
		Description: "A daily or hourly config archive, or the archive written at daemon boot, did not complete: the config file could not be read or parsed, the copy could not be written to the archive location, or the older copies past commit-revisions could not be removed. The log line and the `ze show errors` entry name the archive block, the error and the command to run. The next scheduled run tries again; correct the cause the error names, then run `ze config archive <name>` to write the archive at once.",
		Examples:    []string{"ze show errors", "ze show diagnostic " + CodeDoctorArchiveScheduledFailed},
	},
}

// registerDoctorCodes registers this package's doctor diagnostic codes.
func registerDoctorCodes() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: archive: doctor code registration refused: " + err.Error())
		}
	}
}
