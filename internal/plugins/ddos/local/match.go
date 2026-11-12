// Design: docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md -- vector to firewall term

package local

import (
	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/ddosevent"
)

// buildDropTerm renders the vector as one nftables drop term. Every field is
// optional and contributes a match only when set.
//
// Caller MUST pass a vector with a valid DstPrefix. A vector whose fields are all
// zero yields a term with NO matches, which nftables renders as an unconditional
// `counter drop` on a base hook -- a blackhole, not a mitigation. The single caller
// (responder.applyMitigation) rejects an invalid DstPrefix before reaching here.
//
// The protocol is the first match, ahead of the victim address, though the
// order changes no packet the term selects. The IKE engine's ESP form fast
// path rewrites bare ESP before netfilter runs, so it stays on only while no
// input-path rule can drop ESP, and its analyzer reads a rule as unable to
// match ESP only from the rule's own first two expressions, a protocol load
// and its compare against a protocol other than ESP (espFormExcludesESP,
// internal/component/ike/dataplane/espform_firewall_linux.go). An address
// first would open the rule with the address load, so every armed mitigation
// would keep that fast path shut. A vector whose protocol has no canonical
// name opens with the address and still closes it, which is right: that drop
// can match ESP sent to the victim. The term goes into an ip or ip6 table
// (familyFromPrefix), whose family is the table's own, so the lowering owes no
// family check ahead of the address load (nfprotoGuard,
// internal/plugins/firewall/nft/lower_linux.go); in an inet table it would
// emit one directly ahead of that load, wherever the address match sits.
func buildDropTerm(name string, v ddosevent.VectorTuple) firewall.Term {
	var matches []firewall.Match
	// The name comes from the one canonical table every firewall backend
	// resolves against; a private copy here narrowed mitigations for six of the
	// ten protocols by silently dropping their protocol condition. A number
	// with no canonical name still contributes no match: MatchProtocol carries
	// a name, and digits would fail inside Backend.Apply, which returns before
	// its single Flush and would leave every other owner's ruleset unapplied.
	if protocol, ok := firewall.ProtocolName(v.Proto); ok {
		matches = append(matches, firewall.MatchProtocol{Protocol: protocol})
	}
	if v.DstPrefix.IsValid() {
		matches = append(matches, firewall.MatchDestinationAddress{Prefix: v.DstPrefix})
	}
	if v.DstPort != 0 {
		matches = append(matches, firewall.MatchDestinationPort{
			Ranges: []firewall.PortRange{{Lo: v.DstPort, Hi: v.DstPort}},
		})
	}
	if v.SrcPort != 0 {
		matches = append(matches, firewall.MatchSourcePort{
			Ranges: []firewall.PortRange{{Lo: v.SrcPort, Hi: v.SrcPort}},
		})
	}
	if v.TCPFlags != 0 {
		// Match packets whose set flags include the vector's flags (SYN for a
		// SYN flood). Mask == Flags means "examine exactly these bits, require
		// them set" -- an exact match on the discriminating flags (AC-9).
		f := firewall.TCPFlags(v.TCPFlags)
		matches = append(matches, firewall.MatchTCPFlags{Flags: f, Mask: f})
	}
	return firewall.Term{
		Name:    name,
		Matches: matches,
		Actions: []firewall.Action{firewall.Counter{}, firewall.Drop{}},
	}
}
