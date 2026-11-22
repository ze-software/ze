// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions, transit traffic
// Related: spd_policy.go -- spdEntryPolicies and spdForwardGathers, which derive the forward projection from the answer
// Related: spd_forward_bypass.go -- spdForwardBypassRefusals, which the doctor check runs on the recorded answer
// Related: apply.go -- applyConfig and releaseSPD, which read, record and pin the answer
// RFC: rfc/short/rfc4301.md -- stateful fragment checking for a port-scoped BYPASS (Section 7.4)

package engine

import (
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/opererr"
)

// spdForwardAnswer is one read of the dataplane's answer to whether its forward check
// sees whole datagrams (spdForwardGathers): gathers, or the error that kept the read
// from answering, with gathers false.
//
// The operator entries the engine installed were derived from one such answer, so the
// engine records it beside them (ikeEngineState.spdForward). A backend keeps its first
// answer once it has one, and it keeps none from a read that failed
// (dataplane.ForwardFragmentGatherer), so an answer read later can differ from the
// recorded one: a start whose read failed installs no forward projection of a
// port-scoped bypass, and a later read that succeeds says one is owed. Re-reading at
// each use would split one entry's install, replacement and removal over two answers,
// and an unchanged entry is never installed again (spdPolicySame), so the projection
// would never go in while the doctor check, reading the new answer, reported nothing.
type spdForwardAnswer struct {
	gathers bool
	err     error
}

// readSPDForwardAnswer reads dp's answer once.
func readSPDForwardAnswer(dp dataplane.Dataplane) spdForwardAnswer {
	gathers, err := spdForwardGathers(dp)
	return spdForwardAnswer{gathers: gathers, err: err}
}

// pin answers dp with every fragment-gathering read answering a, so one call of the
// SPD reconciler (spdEntryPolicies, reconcileSPDPolicies, installSPDPoliciesAtStart,
// removeSPDPolicies, spdForwardBypassRefusals) derives every entry from the one answer
// the engine records. A nil dp stays nil, so the reconciler's "no dataplane" guards
// still see none.
//
// The answer it returns carries only the Dataplane methods and the gathering answer:
// it MUST be handed only to the SPD functions above, which call nothing else of dp,
// never to code that asks dp for another optional interface.
func (a spdForwardAnswer) pin(dp dataplane.Dataplane) dataplane.Dataplane {
	if dp == nil {
		return nil
	}
	return spdForwardPinned{Dataplane: dp, answer: a}
}

// spdForwardPinned is a dataplane whose fragment-gathering answer is fixed (pin).
type spdForwardPinned struct {
	dataplane.Dataplane
	answer spdForwardAnswer
}

// GathersForwardFragments answers the pinned read.
func (d spdForwardPinned) GathersForwardFragments() (bool, error) {
	return d.answer.gathers, d.answer.err
}

// moveSPDForwardAnswer brings the installed operator entries from the answer they were
// installed with (s.spdForward) to next, and records next once the dataplane holds the
// entries as next derives them. An answer that projects the same entries changes
// nothing in the dataplane and is recorded at once. The caller holds applyMu.
//
// Only a bypass with a port projects differently on the two answers
// (spdEntryHasTransit), so only such an entry is touched: each forward policy next
// derives and the recorded answer did not is installed, in the entries' order, and each
// one the recorded answer derived and next does not is removed. An install the
// dataplane refuses takes back what this call installed and answers the operator
// error, so a reload is refused with the recorded answer still matching the dataplane.
// A removal that fails leaves the policy acting on its traffic, so it is logged at Warn
// and kept in the SPD ledger (removeZePolicy), as removeReplacedSPDPolicies does.
//
// RFC 4301 Section 7.4: "All implementations MUST support stateful fragment checking to
// accommodate BYPASS traffic for which a non-trivial port range is specified."
func (s *ikeEngineState) moveSPDForwardAnswer(dp dataplane.Dataplane, next spdForwardAnswer) error {
	if dp == nil || s.spdForward.gathers == next.gathers {
		s.spdForward = next
		return nil
	}
	var written []dataplane.SPParams
	var gone []dataplane.SPParams
	for _, name := range spdInstallOrder(s.installedSPD) {
		p := s.installedSPD[name]
		if !spdBypassHasPort(p) {
			continue
		}
		before := spdEntryPolicies(s.spdForward.pin(dp), p)
		held := make(map[spdSelector]bool, len(before))
		for i := range before {
			held[selectorOf(&before[i])] = true
		}
		after := spdEntryPolicies(next.pin(dp), p)
		owed := make(map[spdSelector]bool, len(after))
		for i := range after {
			sel := selectorOf(&after[i])
			owed[sel] = true
			if held[sel] {
				continue
			}
			if err := dp.InstallPolicy(after[i]); err != nil {
				s.takeBackForwardAnswerInstalls(dp, written)
				return spdForwardAnswerRefusal(name, p, &after[i], err)
			}
			written = append(written, after[i])
		}
		for i := range before {
			if !owed[selectorOf(&before[i])] {
				gone = append(gone, before[i])
			}
		}
	}
	for i := range gone {
		sp := &gone[i]
		if err := removeZePolicy(dp, *sp); !policyGone(err) {
			s.log.Warn("ike: could not remove a forward entry of an operator bypass that this kernel's fragment answer no longer projects; it still acts on its traffic",
				"code", codeDoctorIPsecSPDUndoIncomplete,
				"dir", dirLabel(sp.Dir), "src", sp.Src, "dst", sp.Dst, "error", err)
		}
	}
	s.spdForward = next
	return nil
}

// takeBackForwardAnswerInstalls removes, newest first, the forward policies one refused
// moveSPDForwardAnswer call installed. A removal that fails is logged at Error and kept
// in the SPD ledger (removeZePolicy), because the dataplane then holds a policy the
// recorded answer does not derive.
func (s *ikeEngineState) takeBackForwardAnswerInstalls(dp dataplane.Dataplane, written []dataplane.SPParams) {
	for i := len(written) - 1; i >= 0; i-- {
		sp := &written[i]
		if err := removeZePolicy(dp, *sp); !policyGone(err) {
			s.log.Error("ike: could not remove a forward entry of an operator bypass after a refused install",
				"code", codeDoctorIPsecSPDUndoIncomplete,
				"dir", dirLabel(sp.Dir), "src", sp.Src, "dst", sp.Dst, "priority", sp.Priority, "error", err)
		}
	}
}

// spdForwardAnswerRefusal is the operator error for a forward entry of a port-scoped
// bypass that the dataplane refused when the kernel's fragment answer, unread when the
// entry was installed, became readable.
func spdForwardAnswerRefusal(name string, p ipsec.SPDPolicy, sp *dataplane.SPParams, err error) *opererr.Error {
	return opererr.Wrap(err,
		"Ze could not install the forwarding entry of vpn ipsec policy "+name+" (direction "+dirLabel(sp.Dir)+")",
		"The entry is a bypass with a port. Ze installed it while it could not read whether this kernel reassembles "+
			"forwarded fragments first, so it went in without its forwarding entries, and the kernel now answers that it does. "+
			"Until they are installed, forwarded traffic on the entry's ports meets the next entry instead of this "+
			spdActionName(p.Action)+" one. The entries in force before stay as they were. Remove the cause the dataplane names, then retry",
		spdInstallFixCommit)
}
