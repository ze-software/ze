// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/ssh.

package ssh

import "github.com/ze-software/ze/internal/core/diagnostic"

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeSSHHostKeyMissing,
		Title:       "SSH host key not found",
		Description: "The SSH host key file could not be found at the expected path.",
		Examples:    []string{"ze doctor --json"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: ssh: doctor code registration refused: " + err.Error())
		}
	}
}
