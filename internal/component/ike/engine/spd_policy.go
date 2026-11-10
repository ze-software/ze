// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SA/SP installation
// Related: bypass.go -- the IKE control-plane bypass these entries must never outrank
// Related: child.go -- childPolicies, the producer of every NEGOTIATED SPD entry
// RFC: rfc/short/rfc4301.md -- SPD dispositions and ordering (Sections 4.4.1, 7.4)

package engine

import (
	"log/slog"
	"net"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
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

// installSPDPolicies reconciles the operator's SPD entries against what is installed.
//
// It takes the PREVIOUS set as well as the next one, and removes what left the
// configuration before installing what joined it. The order is the guard: an entry
// whose selector is edited in place describes the SAME kernel policy under both
// versions, because the kernel identifies a policy by its selector alone, so
// installing first and removing after would install the new form and then delete it.
//
// A removal that finds nothing is expected, and so is a platform with no XFRM: the
// control plane must run where no dataplane can be programmed, which is how
// installIKEBypass tolerates the same case.
//
// An install failure is logged and never fatal. It is deliberately loud for a DISCARD
// entry, because that failure fails OPEN: the traffic the operator asked to stop at
// the boundary keeps crossing it, and nothing else in the system reports that.
func installSPDPolicies(dp dataplane.Dataplane, previous, next map[string]ipsec.SPDPolicy, log *slog.Logger) {
	if dp == nil {
		return
	}

	for name := range previous {
		if _, kept := next[name]; kept && spdPolicySame(previous[name], next[name]) {
			continue
		}
		entries := spdPolicyParams(previous[name])
		for i := range entries {
			sp := &entries[i]
			if err := removeZePolicy(dp, *sp); err != nil {
				log.Debug("ike: remove spd policy", "policy", name, "dir", sp.Dir, "error", err)
			}
		}
	}

	for name := range next {
		if _, held := previous[name]; held && spdPolicySame(previous[name], next[name]) {
			continue
		}
		entries := spdPolicyParams(next[name])
		for i := range entries {
			sp := &entries[i]
			if err := dp.InstallPolicy(*sp); err != nil {
				if isXFRMUnsupported(err) {
					log.Debug("ike: spd policies unavailable on this platform", "error", err)
					return
				}
				log.Warn("ike: could not install operator SPD policy; the traffic it names crosses the IPsec boundary unchanged",
					"policy", name, "action", spdActionName(next[name].Action), "dir", sp.Dir, "error", err)
			}
		}
	}
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
