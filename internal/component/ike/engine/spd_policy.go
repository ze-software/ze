// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SA/SP installation
// Related: bypass.go -- the IKE control-plane bypass these entries must never outrank
// Related: child.go -- childPolicyParams, the producer of every NEGOTIATED SPD entry
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
// them. childPolicyParams performs the same swap for a negotiated entry, so the two
// producers agree on what "local" means.
//
// The result is empty for an entry with no direction. That is not a default this
// function picks: SPDDirection's zero names neither side, so an entry carrying it
// skipped the parser, and choosing a side here would install the operator's rule on a
// half of the database they never named (ai/rules/principles.md).
func spdPolicyParams(p ipsec.SPDPolicy) []dataplane.SPParams {
	out := make([]dataplane.SPParams, 0, 2)
	if p.Direction == ipsec.SPDDirOut || p.Direction == ipsec.SPDDirBoth {
		out = append(out, spdPolicyDirection(p, dataplane.SADirOut))
	}
	if p.Direction == ipsec.SPDDirIn || p.Direction == ipsec.SPDDirBoth {
		out = append(out, spdPolicyDirection(p, dataplane.SADirIn))
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
		for _, sp := range spdPolicyParams(previous[name]) {
			if err := dp.RemovePolicyParams(sp); err != nil {
				log.Debug("ike: remove spd policy", "policy", name, "dir", sp.Dir, "error", err)
			}
		}
	}

	for name := range next {
		if _, held := previous[name]; held && spdPolicySame(previous[name], next[name]) {
			continue
		}
		for _, sp := range spdPolicyParams(next[name]) {
			if err := dp.InstallPolicy(sp); err != nil {
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
// the box then explains why.
func removeSPDPolicies(dp dataplane.Dataplane, policies map[string]ipsec.SPDPolicy, log *slog.Logger) {
	if dp == nil {
		return
	}
	for name := range policies {
		for _, sp := range spdPolicyParams(policies[name]) {
			if err := dp.RemovePolicyParams(sp); err != nil {
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
	}
	return "unknown"
}
