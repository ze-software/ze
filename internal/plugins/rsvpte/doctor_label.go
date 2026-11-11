// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RSVP-TE label entries a replay did not restore
// Related: fib.go -- busFIB records the entries a writer's replay did not install
// Related: register.go -- the rsvp-te-label-entries runtime check and its OnDoctorCheck answer
// Related: ../../core/mplsfib/refusal.go -- Refusals, the record every label source keeps
//
// An entry a writer refuses on first apply refuses its LSP, which the
// engine already reports and retries. An entry a writer asked for again
// (replay, after VPP connected late or restarted) and did not acknowledge as
// installed leaves an LSP that RSVP-TE holds up with no forwarding under it.
// busFIB keeps each such entry, with the cause, until it is applied again or
// a later replay succeeds. The rsvp-te-label-entries check runs in the
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
// message, and the next replay the writer asks for applies the entry again.
const labelEntriesFix = "fix the writer's cause named in the message; RSVP-TE applies its entries again when the data plane's writer next opens"

// labelEntriesCheckDecl declares the runtime check in RSVP-TE's Stage 1
// registration, so `show doctor` asks RSVP-TE's process for it.
var labelEntriesCheckDecl = rpc.DoctorCheckDecl{
	Name:  labelEntriesCheckName,
	Phase: rpc.DoctorPhaseRuntime,
	Codes: []string{mplsfibevents.CodeDoctorNoWriter},
}

// labelDoctorFindings answers the rsvp-te-label-entries check with every
// entry a replay did not restore. Safe for concurrent use.
func (b *busFIB) labelDoctorFindings() []rpc.DoctorCheckDiagnostic {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.refusals.Findings("rsvp-te", labelEntriesFix)
}
