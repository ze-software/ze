// Design: docs/guide/config-archive.md -- doctor check and scheduled-failure event code registration for the archive component
// Related: doctor.go -- checkArchiveDestinations, the check registered here
//
// The archive component is not a plugin, so it registers through the
// component path (diagnostic.RegisterDoctorCheck) rather than a plugin
// Registration's DoctorChecks field. Keeping the check and its registration
// under internal/component/config/archive means removing the component removes
// its diagnostic with it.

package archive

import (
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	registerDoctorCodes()
	// A refusal is a programmer error in the declaration beside it -- a
	// duplicate name, a phase that does not exist, a code without the doctor
	// prefix -- and none of them can be reached from a config or a peer, so it
	// stops the process rather than leaving `ze doctor` quietly short of a check.
	if err := diagnostic.RegisterDoctorCheck(archiveDoctorCheck); err != nil {
		panic("BUG: archive doctor check registration refused: " + err.Error())
	}

	// A scheduled archive that fails (scheduler.go, reportScheduled) is an
	// event with no caller to answer, so it is raised on the report bus and
	// reported by the recent-events check, which declares the code once it is
	// registered here. codes.go describes it, which the registration requires.
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: CodeDoctorArchiveScheduledFailed}); err != nil {
		panic("BUG: archive scheduled-failure event code registration refused: " + err.Error())
	}
}
