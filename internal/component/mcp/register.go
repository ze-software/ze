// Design: docs/architecture/mcp/overview.md -- doctor check registration for the mcp component
// Related: doctor.go -- checkMCPTLSMaterial, the check registered here
//
// The mcp component is not a plugin, so it registers through the component
// path (diagnostic.RegisterDoctorCheck) rather than a plugin Registration's
// DoctorChecks field. Keeping the check and its registration under
// internal/component/mcp means removing the component removes its diagnostic
// with it.

package mcp

import (
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	// A refusal is a programmer error in the declaration beside it -- a
	// duplicate name, a phase that does not exist, a code without the doctor
	// prefix -- and none of them can be reached from a config or a peer, so it
	// stops the process rather than leaving `ze doctor` quietly short of a check.
	if err := diagnostic.RegisterDoctorCheck(mcpTLSDoctorCheck); err != nil {
		panic("BUG: mcp doctor check registration refused: " + err.Error())
	}
}
