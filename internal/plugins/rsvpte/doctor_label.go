// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RSVP-TE label entries a writer refused or a replay did not restore
// Related: fib.go -- busFIB records entries a writer refused on apply or replay
// Related: register.go -- the rsvp-te-label-entries runtime check and its OnDoctorCheck answer
// Related: ../../core/mplsfib/refusal.go -- Refusals, the record every label source keeps
//
// busFIB records every label entry a writer refused on live apply as well as
// every installed entry a writer's replay did not restore, with the writer's
// cause, until a later apply of the same slot succeeds (or a later replay
// restores that exact entry). The rsvp-te-label-entries check runs in the
// runtime phase, through RSVP-TE's own doctor callback, because the record
// lives in RSVP-TE's process, which may be a forked one.
package rsvpte

import (
	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// labelEntriesCheckName is the runtime doctor check RSVP-TE answers itself.
const labelEntriesCheckName = "rsvp-te-label-entries"

// labelEntriesFix is what clears a finding: the writer's cause is in the
// message; RSVP-TE applies a refused add again when its reservation is next
// refreshed, and an installed entry when the data plane's writer next opens.
const labelEntriesFix = "fix the writer's cause named in the message; RSVP-TE applies a refused add again when its reservation is next refreshed, and an installed entry when the data plane's writer next opens"

// labelEntriesCheckDecl declares the runtime check in RSVP-TE's Stage 1
// registration, so `show doctor` asks RSVP-TE's process for it.
var labelEntriesCheckDecl = rpc.DoctorCheckDecl{
	Name:  labelEntriesCheckName,
	Phase: rpc.DoctorPhaseRuntime,
	Codes: []string{mplsfibevents.CodeDoctorNoWriter},
}

// labelDoctorFindings answers the rsvp-te-label-entries check with every
// entry a writer refused on apply or a replay did not restore. Safe for
// concurrent use.
func (b *busFIB) labelDoctorFindings() []rpc.DoctorCheckDiagnostic {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.refusals.Findings("rsvp-te", labelEntriesFix)
}
