// Design: docs/architecture/ldp/mpls-ldp.md -- LDP label entries the data plane does not hold
// Related: fib.go -- ldpFIB records each entry no writer carried out
// Related: register.go -- the ldp-label-entries runtime check and its OnDoctorCheck answer
// Related: ../../core/mplsfib/refusal.go -- Refusals, the record every label source keeps
//
// LDP hands every label entry to the writer of the selected data plane and
// learns from the acknowledgement whether it was installed. An entry no
// writer carried out (no writer running, or the writer refused it) is kept,
// with the cause, until a later apply of the same FEC or in-label succeeds.
// The ldp-label-entries check runs in the runtime phase, through LDP's own
// doctor callback, because the record lives in LDP's process, which may be a
// forked one.
package ldp

import (
	"errors"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// labelEntriesCheckName is the runtime doctor check LDP answers itself.
const labelEntriesCheckName = "ldp-label-entries"

// labelEntriesFix is what clears a finding: the writer's cause is in the
// message, and LDP applies every entry it wants again when a writer opens.
const labelEntriesFix = "fix the writer's cause named in the message; LDP applies the entry again when the data plane's writer opens"

// errNoEventBus is the cause recorded when LDP has no event bus to reach a
// writer through: a wiring defect, since the engine always configures one.
var errNoEventBus = errors.New("ldp has no event bus, so no forwarding writer can be reached")

// labelEntriesCheckDecl declares the runtime check in LDP's Stage 1
// registration, so `show doctor` asks LDP's process for it.
var labelEntriesCheckDecl = rpc.DoctorCheckDecl{
	Name:  labelEntriesCheckName,
	Phase: rpc.DoctorPhaseRuntime,
	Codes: []string{mplsfibevents.CodeDoctorNoWriter},
}

// labelDoctorFindings answers the ldp-label-entries check with every entry
// no writer carried out. Safe for concurrent use.
func (f *ldpFIB) labelDoctorFindings() []rpc.DoctorCheckDiagnostic {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.refusals.Findings("ldp", labelEntriesFix)
}
