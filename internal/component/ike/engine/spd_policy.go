// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SA/SP installation
// Related: bypass.go -- the IKE control-plane bypass these entries must never outrank
// Related: child.go -- childPolicies, the producer of every NEGOTIATED SPD entry
// Related: spd_order.go -- verifySPDOrder, the commit refusal of an order a tie decides
// Related: doctor_spd_install.go -- the ipsec-spd-install check of an entry the start could not install
// RFC: rfc/short/rfc4301.md -- SPD dispositions and ordering (Sections 4.4.1, 7.4)

package engine

import (
	"cmp"
	"context"
	"log/slog"
	"maps"
	"net"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
)

// spdPolicyParams turns one operator-authored SPD entry into the policies it installs.
//
// A both-direction entry becomes a MIRRORED PAIR rather than one policy, because a
// Security Policy is one-way (RFC 4301 Section 4.4.1 splits the database into SPD-O
// and SPD-I). The inbound half swaps the two sides: the local prefix is the SOURCE of
// an outbound packet and the DESTINATION of an inbound one, and the ports swap with
// them. childPolicies performs the same swap for a negotiated entry, so the two
// producers agree on what "local" means.
//
// The result is empty for an entry with no direction. That is not a default this
// function picks: SPDDirection's zero names neither side, so an entry carrying it
// skipped the parser, and choosing a side here would install the operator's rule on a
// half of the database they never named (ai/rules/principles.md).
//
// An entry with an ICMP code range installs one policy per mask-aligned code block in
// each direction (spdICMPMatches), all derived here, so install, removal and shutdown
// always act on the same set.
func spdPolicyParams(p ipsec.SPDPolicy) []dataplane.SPParams {
	icmp := spdICMPMatches(p)
	out := make([]dataplane.SPParams, 0, 2*max(1, len(icmp)))
	for _, dir := range []dataplane.SADir{dataplane.SADirOut, dataplane.SADirIn} {
		if !spdPolicyHasDirection(p.Direction, dir) {
			continue
		}
		if icmp == nil {
			out = append(out, spdPolicyDirection(p, dir))
			continue
		}
		for _, m := range icmp {
			sp := spdPolicyDirection(p, dir)
			// RFC 4301 Section 4.4.1.1: the type and code are one selector for the
			// message, not a source and a destination field, so they do not swap
			// with the direction as ports do.
			sp.SrcPort, sp.DstPort = m.typ, m.code
			out = append(out, sp)
		}
	}
	return out
}

// spdPolicyHasDirection reports whether an operator direction includes one policy
// direction.
func spdPolicyHasDirection(d ipsec.SPDDirection, dir dataplane.SADir) bool {
	if dir == dataplane.SADirOut {
		return d == ipsec.SPDDirOut || d == ipsec.SPDDirBoth
	}
	return d == ipsec.SPDDirIn || d == ipsec.SPDDirBoth
}

// spdICMPMatch is one ICMP selector as the kernel holds it: the message type in the
// source port of the selector and the code in its destination port
// (dataplane.IsICMPProto).
type spdICMPMatch struct {
	typ, code dataplane.PortMatch
}

// spdICMPMatches projects an entry's ICMP type and code selector onto kernel selectors.
// It answers nil for an entry with no ICMP selector, whose ports apply unchanged.
//
// RFC 4301 Section 4.4.1.1: "Given a policy entry with a range of Types (T-start to
// T-end) and a range of Codes (C-start to C-end), and an ICMP packet with Type t and
// Code c, an implementation MUST test for a match using
//
//	(T-start*256) + C-start <= (t*256) + c <= (T-end*256) + C-end"
//
// The entry carries one type (T-start equal to T-end, ValidateSPDPolicies refuses a
// code under ANY type), so the test reduces to t equal to T and C-start <= c <= C-end.
// The type is one exact match. The code range becomes the mask-aligned blocks of
// dataplane.MaskedBlocks, whose union is exactly C-start..C-end, so the policies
// together match a message exactly when the RFC's test does. ANY code is one any-code
// match, and ANY type and ANY code is one match with neither constrained.
func spdICMPMatches(p ipsec.SPDPolicy) []spdICMPMatch {
	if p.ICMP.IsAny() {
		return nil
	}
	if !dataplane.IsICMPProto(p.Protocol) {
		return nil
	}
	typ := dataplane.AnyPortMatch()
	if p.ICMP.HasType {
		typ = dataplane.ExactPortMatch(uint16(p.ICMP.Type))
	}
	if !p.ICMP.HasCode {
		return []spdICMPMatch{{typ: typ, code: dataplane.AnyPortMatch()}}
	}
	blocks := dataplane.MaskedBlocks(uint16(p.ICMP.CodeStart), uint16(p.ICMP.CodeEnd))
	out := make([]spdICMPMatch, 0, len(blocks))
	for _, code := range blocks {
		out = append(out, spdICMPMatch{typ: typ, code: code})
	}
	return out
}

// spdPolicyDirection builds the policy for one direction.
//
// It sets NO template field. An operator entry is BYPASS or DISCARD, and neither
// hands traffic to a transform, so Mode, ReqID, TunnelSrc, TunnelDst and SAID all stay
// at their zero values and the backend's template-free path builds the policy
// (xfrmPolicyFromParams, xfrm_linux.go).
//
// Owner stays empty for the same reason. policyOwners exempts every template-free
// policy (policy_owner.go, isTemplateFree), because ownership exists to stop one
// PEER's selector taking another peer's live tunnel over, and an operator entry
// belongs to the node rather than to a peer.
func spdPolicyDirection(p ipsec.SPDPolicy, dir dataplane.SADir) dataplane.SPParams {
	src, dst := p.LocalPrefix, p.RemotePrefix
	srcPort, dstPort := spdPortMatch(p.LocalPort), spdPortMatch(p.RemotePort)
	if dir == dataplane.SADirIn {
		src, dst = p.RemotePrefix, p.LocalPrefix
		srcPort, dstPort = spdPortMatch(p.RemotePort), spdPortMatch(p.LocalPort)
	}
	return dataplane.SPParams{
		Src:    src,
		Dst:    dst,
		Dir:    dir,
		Action: p.Action,
		Role:   dataplane.PolicyRoleOperator,
		// RFC 4301 Section 4.4.1: the management interface "MUST support (total)
		// ordering of these entries, as seen via this interface". The operator's
		// order IS the kernel priority; ValidateSPDPolicies has already refused a
		// rank that would outrank the IKE control-plane bypass.
		Priority:   int(p.Order),
		UpperProto: p.Protocol,
		SrcPort:    srcPort,
		DstPort:    dstPort,
	}
}

// spdPortMatch converts the configured port form to the selector the backend takes.
// Only ANY and one exact port reach here: ValidateSPDPolicies refuses every other
// form at commit, so this never has to widen one.
func spdPortMatch(p ipsec.PortSelector) dataplane.PortMatch {
	if p.IsAny() {
		return dataplane.AnyPortMatch()
	}
	return dataplane.ExactPortMatch(p.Port)
}

// Fix lines of an operator entry the dataplane refused (spdInstallRefusal).
const (
	// spdInstallFixCommit retries a refused reload: the running entries were kept, so
	// committing again after the cause is gone installs the edit.
	spdInstallFixCommit = "commit"
	// spdInstallFixRestart retries an entry the start could not install. A reload
	// reaches the engine only when `vpn` changed, so an untouched configuration is
	// installed again by a restart, or by any commit that changes `vpn ipsec`. It is
	// also the fix of a divergence (spdDivergence): only a restart rebuilds a selector
	// no later commit rewrites.
	spdInstallFixRestart = "ze signal restart"
)

// spdDivergenceKind names how a reconcile left one selector of the dataplane holding a
// form the running configuration does not mean. Zero names no way, so a record that
// carries it was never filled in.
type spdDivergenceKind uint8

const (
	spdDivergenceUnspecified spdDivergenceKind = iota
	// spdDivergenceNotRestored: the undo of a refused install could not put back the
	// running form of an entry the install had replaced in place, so the selector holds
	// the refused form, or nothing.
	spdDivergenceNotRestored
	// spdDivergenceNotTakenBack: the undo of a refused install could not remove a policy
	// the install wrote at a selector the running configuration does not hold.
	spdDivergenceNotTakenBack
	// spdDivergencePreviousKept: an accepted reload could not remove the previous form of
	// an entry it changed or removed, so that form still acts on its traffic.
	spdDivergencePreviousKept
)

// spdDivergence is one selector at which a reconcile left the dataplane disagreeing with
// the running configuration, as a value the ipsec-spd-install doctor check keeps: how it
// diverged, the selector (its direction included), the priority of the policy the step
// was writing or removing, and the dataplane's answer.
type spdDivergence struct {
	kind     spdDivergenceKind
	selector spdSelector
	priority int
	cause    string
}

// newSPDDivergence records the failed step on sp.
func newSPDDivergence(kind spdDivergenceKind, sp *dataplane.SPParams, err error) spdDivergence {
	return spdDivergence{kind: kind, selector: selectorOf(sp), priority: sp.Priority, cause: err.Error()}
}

// message says what the dataplane holds at the selector, and why that is wrong.
func (d spdDivergence) message() string {
	policy := "the operator SPD policy " + d.selector.source + " -> " + d.selector.target + " (" +
		dirLabel(d.selector.dir) + ", priority " + strconv.Itoa(d.priority) + ")"
	var what string
	switch d.kind {
	case spdDivergenceNotRestored:
		what = "The dataplane lacks " + policy + ", which the running configuration holds: a commit " +
			"replaced it, the dataplane refused that commit, and putting it back failed"
	case spdDivergenceNotTakenBack:
		what = "The dataplane holds " + policy + ", which the running configuration does not: an " +
			"install the dataplane then refused wrote it, and taking it back failed"
	case spdDivergencePreviousKept:
		what = "The dataplane still holds " + policy + ", the previous form of an entry the running " +
			"configuration changed or removed, and it still acts on its traffic: removing it failed"
	case spdDivergenceUnspecified:
		panic("BUG: an SPD divergence with no kind")
	default:
		panic("BUG: unknown SPD divergence kind")
	}
	return what + ". The dataplane answered: " + d.cause + "."
}

// spdReconcile is what one reconcile of the operator's entries left in the dataplane,
// selector by selector, as values: each selector it left holding what the running
// configuration means (settled), and each it left holding something else (diverged).
// A selector can appear in both, settled by a write and diverged by the undo of that
// write after it, so a reader applies settled first (recordSPDReconcile).
type spdReconcile struct {
	settled  []spdSelector
	diverged []spdDivergence
}

// add appends what another step of the same reconcile left, after what r holds.
func (r *spdReconcile) add(step spdReconcile) {
	r.settled = append(r.settled, step.settled...)
	r.diverged = append(r.diverged, step.diverged...)
}

// reconcileSPDPolicies reconciles the operator's SPD entries against what is installed,
// for a configuration that can still be refused: a reload, or a first apply that
// arrived as one. It is all or nothing, and it answers, beside the refusal, what it left
// in the dataplane selector by selector (spdReconcile), for the ipsec-spd-install
// doctor check.
//
// RFC 4301 Section 4.4.1: "The management interface for the SPD MUST allow creation of
// entries consistent with the selectors defined in Section 4.4.1.1, and MUST support
// (total) ordering of these entries, as seen via this interface." An entry the
// configuration holds and the dataplane does not was never created, so an install the
// dataplane refuses fails the apply with an operator error naming the entry, the cause
// and the retry, after this call has put back every entry it changed: the running
// configuration stays in force, entry for entry, and the commit is refused.
//
// The new form of an edited entry is installed BEFORE its old form is removed, so the
// traffic it names never meets the next entry down while the edit lands; for a DISCARD
// entry that window would let the traffic cross the boundary. Where the two forms
// share a selector the install replaces the old form in place, because the database
// holds one entry per selector (spdSelector, spd_left.go; the XFRM backend upserts,
// xfrmBackend.InstallPolicy), and the old form is then not removed at all: a removal
// by that selector would delete the form just installed.
//
// The entries reach the dataplane in their order, lowest first, and in name order
// within one order (spdInstallOrder), never in map order.
//
// A platform with no XFRM at all is no refusal: the control plane must run where no
// dataplane exists to hold any entry, which is how installIKEBypass tolerates the same
// case. A removal that finds nothing is silent.
func reconcileSPDPolicies(dp dataplane.Dataplane, previous, next map[string]ipsec.SPDPolicy, log *slog.Logger) (spdReconcile, error) {
	var outcome spdReconcile
	if dp == nil {
		return outcome, nil
	}

	var written []dataplane.SPParams
	for _, name := range spdInstallOrder(next) {
		if held, ok := previous[name]; ok && spdPolicySame(held, next[name]) {
			continue
		}
		entries := spdPolicyParams(next[name])
		for i := range entries {
			err := dp.InstallPolicy(entries[i])
			if err == nil {
				written = append(written, entries[i])
				continue
			}
			if isXFRMUnsupported(err) {
				log.Debug("ike: spd policies unavailable on this platform", "error", err)
				return spdReconcile{}, nil
			}
			// The undo decides every written selector, so the writes settle nothing.
			return undoSPDInstall(dp, previous, written, log), spdInstallRefusal(name, next[name], &entries[i], err, spdInstallFixCommit)
		}
	}

	for i := range written {
		outcome.settled = append(outcome.settled, selectorOf(&written[i]))
	}
	outcome.add(removeReplacedSPDPolicies(dp, previous, next, log))
	return outcome, nil
}

// installSPDPoliciesAtStart installs the operator's SPD entries the daemon starts with,
// which nothing can refuse: a start must never refuse the IPsec block (owner decision
// 2026-11-19, "Install, log, doctor"). Every entry the dataplane holds is installed;
// one it refuses is logged at Error with its fix, left out of the installed set, and
// answered in refused, which the ipsec-spd-install doctor check reports.
//
// Each entry goes in whole or not at all: the halves of a refused entry that did go in
// are removed again, so installed is exactly what the dataplane holds, the next reload
// installs the refused entry afresh (it is absent from the previous set), and the exit
// removes no half it does not know of. A half whose removal fails is answered in
// outcome as diverged, beside the selectors the start settled.
func installSPDPoliciesAtStart(dp dataplane.Dataplane, next map[string]ipsec.SPDPolicy, log *slog.Logger) (installed map[string]ipsec.SPDPolicy, refused map[string]*opererr.Error, outcome spdReconcile) {
	if dp == nil {
		return next, nil, outcome
	}
	installed = make(map[string]ipsec.SPDPolicy, len(next))
	for _, name := range spdInstallOrder(next) {
		entries := spdPolicyParams(next[name])
		var written []dataplane.SPParams
		var refusal *opererr.Error
		for i := range entries {
			err := dp.InstallPolicy(entries[i])
			if err == nil {
				written = append(written, entries[i])
				continue
			}
			if isXFRMUnsupported(err) {
				log.Debug("ike: spd policies unavailable on this platform", "error", err)
				return next, nil, spdReconcile{}
			}
			refusal = spdInstallRefusal(name, next[name], &entries[i], err, spdInstallFixRestart)
			break
		}
		if refusal == nil {
			installed[name] = next[name]
			for i := range written {
				outcome.settled = append(outcome.settled, selectorOf(&written[i]))
			}
			continue
		}
		outcome.add(undoSPDInstall(dp, nil, written, log))
		log.LogAttrs(context.Background(), slog.LevelError, "ike: "+refusal.Problem(),
			refusal.LogAttrs(slog.String("code", diagnostic.CodeDoctorIPsecSPDInstallRefused))...)
		if refused == nil {
			refused = make(map[string]*opererr.Error)
		}
		refused[name] = refusal
	}
	return installed, refused, outcome
}

// spdInstallRefusal is the operator error for one entry the dataplane refused: the
// entry, the half of it that failed, the dataplane's own answer as the cause, and the
// command that retries.
func spdInstallRefusal(name string, p ipsec.SPDPolicy, sp *dataplane.SPParams, err error, fix string) *opererr.Error {
	action := spdActionName(p.Action)
	return opererr.Wrap(err,
		"Ze could not install operator SPD policy "+name+" ("+action+", direction "+dirLabel(sp.Dir)+")",
		"The dataplane refused the entry, so the traffic `vpn ipsec policy "+name+"` names would meet the next "+
			"entry of the SPD instead of this "+action+" one; the entries in force before stay as they were. "+
			"Remove the cause the dataplane names, then retry",
		fix)
}

// undoSPDInstall takes back what one refused install call wrote, newest first. A
// written entry that replaced a previous form in place (the same selector) gets that
// form back; one that took a selector no previous entry held is removed. A step of the
// undo that fails is logged at Error and answered as diverged, because the dataplane
// then disagrees with the running configuration at that selector; a removal that failed
// is also kept in the SPD ledger (removeZePolicy), so the exit keeps the catch-all
// beneath it. Every step that succeeds answers its selector as settled.
func undoSPDInstall(dp dataplane.Dataplane, previous map[string]ipsec.SPDPolicy, written []dataplane.SPParams, log *slog.Logger) spdReconcile {
	var outcome spdReconcile
	held := make(map[spdSelector]dataplane.SPParams)
	for name := range previous {
		policies := spdPolicyParams(previous[name])
		for i := range policies {
			held[selectorOf(&policies[i])] = policies[i]
		}
	}
	for i := range slices.Backward(written) {
		sp := &written[i]
		if old, ok := held[selectorOf(sp)]; ok {
			if err := dp.InstallPolicy(old); err != nil {
				log.Error("ike: could not restore an operator SPD entry after a refused install",
					"code", diagnostic.CodeDoctorIPsecSPDUndoIncomplete,
					"dir", dirLabel(old.Dir), "src", old.Src, "dst", old.Dst, "priority", old.Priority, "error", err)
				outcome.diverged = append(outcome.diverged, newSPDDivergence(spdDivergenceNotRestored, &old, err))
				continue
			}
			outcome.settled = append(outcome.settled, selectorOf(sp))
			continue
		}
		if err := removeZePolicy(dp, *sp); !policyGone(err) {
			log.Error("ike: could not remove an operator SPD entry after a refused install",
				"code", diagnostic.CodeDoctorIPsecSPDUndoIncomplete,
				"dir", dirLabel(sp.Dir), "src", sp.Src, "dst", sp.Dst, "priority", sp.Priority, "error", err)
			outcome.diverged = append(outcome.diverged, newSPDDivergence(spdDivergenceNotTakenBack, sp, err))
			continue
		}
		outcome.settled = append(outcome.settled, selectorOf(sp))
	}
	return outcome
}

// removeReplacedSPDPolicies removes the previous form of every entry that left the
// configuration or changed, once the next forms are in. A form whose selector a next
// entry holds was replaced in place by that entry's install and is left alone.
// A removal that finds nothing is silent and settles its selector; one that fails
// leaves the old form acting on its traffic, so it is logged at Warn, answered as
// diverged and kept in the SPD ledger (removeZePolicy).
func removeReplacedSPDPolicies(dp dataplane.Dataplane, previous, next map[string]ipsec.SPDPolicy, log *slog.Logger) spdReconcile {
	var outcome spdReconcile
	kept := make(map[spdSelector]bool)
	for name := range next {
		policies := spdPolicyParams(next[name])
		for i := range policies {
			kept[selectorOf(&policies[i])] = true
		}
	}
	for _, name := range slices.Sorted(maps.Keys(previous)) {
		if current, ok := next[name]; ok && spdPolicySame(previous[name], current) {
			continue
		}
		entries := spdPolicyParams(previous[name])
		for i := range entries {
			sp := &entries[i]
			if kept[selectorOf(sp)] {
				continue
			}
			if err := removeZePolicy(dp, *sp); !policyGone(err) {
				log.Warn("ike: could not remove the previous form of an operator SPD entry; it still acts on its traffic",
					"code", diagnostic.CodeDoctorIPsecSPDUndoIncomplete,
					"policy", name, "dir", dirLabel(sp.Dir), "error", err)
				outcome.diverged = append(outcome.diverged, newSPDDivergence(spdDivergencePreviousKept, sp, err))
				continue
			}
			outcome.settled = append(outcome.settled, selectorOf(sp))
		}
	}
	return outcome
}

// spdInstallOrder answers the entry names in the order the dataplane receives them:
// the operator's order, lowest first, then the name. Two entries that share an order
// do not overlap (verifySPDOrder refuses the pair at commit), so the name decides
// nothing a packet can observe; it makes the sequence the same on every apply.
func spdInstallOrder(entries map[string]ipsec.SPDPolicy) []string {
	return slices.SortedFunc(maps.Keys(entries), func(a, b string) int {
		if c := cmp.Compare(entries[a].Order, entries[b].Order); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	})
}

// removeSPDPolicies releases every operator SPD entry, for engine shutdown.
//
// Ze owns what it touches. An entry that outlives the process keeps dropping or
// bypassing traffic for a daemon that is no longer running, and no configuration on
// the box then explains why. A removal that left an entry installed is recorded in the
// SPD ledger (removeZePolicy, spd_left.go), so releaseSPD keeps the catch-all beneath it.
func removeSPDPolicies(dp dataplane.Dataplane, policies map[string]ipsec.SPDPolicy, log *slog.Logger) {
	if dp == nil {
		return
	}
	for name := range policies {
		entries := spdPolicyParams(policies[name])
		for i := range entries {
			sp := &entries[i]
			if err := removeZePolicy(dp, *sp); err != nil {
				log.Debug("ike: remove spd policy", "policy", name, "dir", sp.Dir, "error", err)
			}
		}
	}
}

// spdPolicySame reports whether two versions of one named entry describe the same
// installed policies, so a reload re-installs only what actually changed.
//
// It compares every field spdPolicyParams reads and nothing else. A field this misses
// would leave the kernel holding the previous form after a commit that changed it,
// which is the quietest failure a reconciler has.
func spdPolicySame(a, b ipsec.SPDPolicy) bool {
	if a.Action != b.Action || a.Order != b.Order || a.Direction != b.Direction || a.Protocol != b.Protocol {
		return false
	}
	if a.LocalPort != b.LocalPort || a.RemotePort != b.RemotePort {
		return false
	}
	if a.ICMP != b.ICMP {
		return false
	}
	return sameIPNet(a.LocalPrefix, b.LocalPrefix) && sameIPNet(a.RemotePrefix, b.RemotePrefix)
}

// sameIPNet compares two prefixes, treating two nils as equal and one nil as unequal.
func sameIPNet(a, b *net.IPNet) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.String() == b.String()
}

// spdActionName renders a disposition for a log line. It never defaults to a
// disposition name, because a log that names the wrong one sends the reader to the
// wrong entry.
func spdActionName(a dataplane.SPAction) string {
	switch a {
	case dataplane.SPActionProtect:
		return "protect"
	case dataplane.SPActionBypass:
		return "bypass"
	case dataplane.SPActionDiscard:
		return "discard"
	default:
		panic("BUG: unknown operator SPD action")
	}
}
