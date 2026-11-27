// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/imageserver.
// Related: config_tx.go -- zefs temporary-directory cleanup warning sites

package imageserver

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorImageServerZefsCleanupFailed = "doctor-imageserver-zefs-cleanup-failed"

	exampleDoctorJSON     = "ze doctor --json"
	exampleShowErrors     = "ze show errors"
	exampleShowDiagPrefix = "ze show diagnostic "
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:  codeDoctorImageServerZefsCleanupFailed,
		Title: "Image server could not remove a temporary zefs directory",
		Description: "The image server failed to delete a temporary zefs staging directory after a failed build, " +
			"config reload, or shutdown. Stale staging files may remain in the system temporary directory until " +
			"removed manually. Check temporary directory permissions and free space with `ze show doctor`.",
		Examples: []string{exampleDoctorJSON, exampleShowErrors, exampleShowDiagPrefix + codeDoctorImageServerZefsCleanupFailed},
	},
}

// doctorEventCodes declares the transient report-bus event codes this package raises.
var doctorEventCodes = []diagnostic.EventCode{
	{Code: codeDoctorImageServerZefsCleanupFailed},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: imageserver: doctor code registration refused: " + err.Error())
		}
	}
	for _, ev := range doctorEventCodes {
		if err := diagnostic.RegisterEventCode(ev); err != nil {
			panic("BUG: imageserver: doctor event code registration refused: " + err.Error())
		}
	}
}
