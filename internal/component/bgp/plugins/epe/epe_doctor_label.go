// Design: docs/architecture/wire/nlri-bgpls.md -- PeerNode SID labels the data plane does not hold
// Related: epe_source.go -- Source records each label no writer carried out
// Related: epe_plugin.go -- the bgp-epe-label-entries runtime check and its OnDoctorCheck answer
// Related: ../../../../core/mplsfib/refusal.go -- Refusals, the record every label source keeps
//
// EPE installs one MPLS pop per established, configured peer through the
// writer of the selected data plane. A label no writer installed, a removal
// no writer carried out, or a label a writer's replay did not restore is
// kept, with the writer's cause, until the data plane holds what EPE wants.
// The bgp-epe-label-entries check runs in the runtime phase, through EPE's
// own doctor callback, because the record lives in EPE's process.

package epe

import (
	"net/netip"

	"github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// labelEntriesCheckName is the runtime doctor check EPE answers itself.
const labelEntriesCheckName = "bgp-epe-label-entries"

// labelEntriesFix is what clears a finding: the writer's cause is in the
// message, and EPE applies its labels again on the next peer or config change,
// or when the data plane's writer opens.
const labelEntriesFix = "fix the writer's cause named in the message; BGP EPE applies its PeerNode SID labels again on the next peer or config change, and when the data plane's writer opens"

// labelEntriesCheckDecl declares the runtime check in EPE's Stage 1
// registration, so `show doctor` asks EPE's process for it.
var labelEntriesCheckDecl = rpc.DoctorCheckDecl{
	Name:  labelEntriesCheckName,
	Phase: rpc.DoctorPhaseRuntime,
	Codes: []string{mplsfib.CodeDoctorNoWriter},
}

// labelDoctorFindings answers the bgp-epe-label-entries check with every
// label the data plane disagrees with EPE about. Safe for concurrent use.
func (s *Source) labelDoctorFindings() []rpc.DoctorCheckDiagnostic {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.refusals.Findings(Name, labelEntriesFix)
}

// recordUnacquiredLocked records every label EPE wants as not installed, for
// cause: the writer did not hand back the labels a previous EPE left, so EPE
// installs none until it does. Caller holds s.mu.
func (s *Source) recordUnacquiredLocked(desired map[netip.Addr]uint32, cause error) {
	for peer, label := range desired {
		s.refusals.Record(&mplsfib.Entry{Action: mplsfib.ActionAdd, Op: mplsfib.OpPop,
			InLabel: label, NextHop: peer, Source: epeMPLSSource}, cause)
	}
}

// forgetSettledLocked drops each refusal the data plane no longer disagrees
// with EPE about: an add whose peer no longer wants that label, or now holds
// it, and a removal whose label is no longer installed for its peer, or that
// the peer wants again. Caller holds s.mu.
func (s *Source) forgetSettledLocked(desired map[netip.Addr]uint32) {
	s.refusals.ClearIf(func(e *mplsfib.Entry) bool {
		installed, held := s.installed[e.NextHop]
		holds := held && installed == e.InLabel
		wanted, wants := desired[e.NextHop]
		wantsIt := wants && wanted == e.InLabel
		if e.Action == mplsfib.ActionRemove {
			// Still disputed only while the label is held and not wanted.
			if !holds {
				return true
			}
			return wantsIt
		}
		if !wantsIt {
			return true
		}
		return holds
	})
}
