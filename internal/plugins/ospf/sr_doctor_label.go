// Design: docs/architecture/mpls/mpls-kernel.md -- Segment Routing label entries the data plane does not hold
// Related: sr_fib.go -- srFIB records each entry no writer carried out
// Related: register.go -- the ospf-sr-label-entries runtime check and its OnDoctorCheck answer
// Related: ../../core/mplsfib/refusal.go -- Refusals, the record every label source keeps
//
// OSPF Segment Routing hands every label entry to the writer of the selected
// data plane and learns from the acknowledgement whether it was installed.
// An entry no writer carried out (no writer running, the writer refused it,
// a replay the writer asked for that it did not install, or no event bus at
// all) is kept, with the cause, until a later apply of the same slot
// succeeds. Each OSPF engine (every OSPFv2 instance, every OSPFv3 address
// family) owns one srFIB, so the check answers from every srFIB the process
// holds. It runs in the runtime phase, through OSPF's own doctor callback,
// because the record lives in OSPF's process, which may be a forked one.

package ospf

import (
	"errors"
	"sync"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// srLabelEntriesCheckName is the runtime doctor check OSPF answers for its
// Segment Routing labels.
const srLabelEntriesCheckName = "ospf-sr-label-entries"

// srLabelEntriesFix is what clears a finding: the cause is in the message,
// and SR applies every entry it wants again when a writer opens.
const srLabelEntriesFix = "fix the cause named in the message; OSPF Segment Routing applies the entry again when the data plane's writer opens"

// errSRNoEventBus is the cause recorded when an SR producer has no event bus
// to reach a writer through. The engine hands its bus only to a plugin it runs
// in its own process, so an OSPF the operator runs as a forked process
// (`plugin { external ... }`) meets this for every SR label it wants.
var errSRNoEventBus = errors.New("OSPF has no event bus, so no forwarding writer can be reached: Segment Routing labels are programmed only when OSPF runs inside the daemon, not as an external plugin")

// srLabelEntriesCheckDecl declares the runtime check in OSPF's Stage 1
// registration, so `show doctor` asks OSPF's process for it.
var srLabelEntriesCheckDecl = rpc.DoctorCheckDecl{
	Name:  srLabelEntriesCheckName,
	Phase: rpc.DoctorPhaseRuntime,
	Codes: []string{mplsfibevents.CodeDoctorNoWriter},
}

// srFIBSet is every srFIB the OSPF engines of this process hold, so one
// doctor answer covers them all. Safe for concurrent use.
type srFIBSet struct {
	mu   sync.Mutex
	fibs map[*srFIB]struct{}
}

// srLabelSources holds the srFIB of each running OSPF engine.
var srLabelSources srFIBSet

// add holds f until the returned function forgets it, which the engine MUST
// call on shutdown.
func (s *srFIBSet) add(f *srFIB) func() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fibs == nil {
		s.fibs = make(map[*srFIB]struct{})
	}
	s.fibs[f] = struct{}{}
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.fibs, f)
	}
}

// srLabelDoctorFindings answers the ospf-sr-label-entries check from every
// srFIB the process holds. Safe for concurrent use.
func srLabelDoctorFindings() []rpc.DoctorCheckDiagnostic {
	srLabelSources.mu.Lock()
	fibs := make([]*srFIB, 0, len(srLabelSources.fibs))
	for f := range srLabelSources.fibs {
		fibs = append(fibs, f)
	}
	srLabelSources.mu.Unlock()
	var findings []rpc.DoctorCheckDiagnostic
	for _, f := range fibs {
		findings = append(findings, f.labelDoctorFindings()...)
	}
	return findings
}

// labelDoctorFindings answers for the entries this srFIB wants that no writer
// carried out. Safe for concurrent use.
func (f *srFIB) labelDoctorFindings() []rpc.DoctorCheckDiagnostic {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.refusals.Findings(f.sourceName(), srLabelEntriesFix)
}

// sourceName names the address family's SR producer in a finding.
func (f *srFIB) sourceName() string {
	if f.source == mplsSourceOSPFv3SR {
		return "ospfv3-sr"
	}
	return "ospf-sr"
}
