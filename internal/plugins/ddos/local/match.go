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
func buildDropTerm(name string, v ddosevent.VectorTuple) firewall.Term {
	var matches []firewall.Match
	if v.DstPrefix.IsValid() {
		matches = append(matches, firewall.MatchDestinationAddress{Prefix: v.DstPrefix})
	}
	// The name comes from the one canonical table every firewall backend
	// resolves against; a private copy here narrowed mitigations for six of the
	// ten protocols by silently dropping their protocol condition. A number
	// with no canonical name still contributes no match: MatchProtocol carries
	// a name, and digits would fail inside Backend.Apply, which returns before
	// its single Flush and would leave every other owner's ruleset unapplied.
	if protocol, ok := firewall.ProtocolName(v.Proto); ok {
		matches = append(matches, firewall.MatchProtocol{Protocol: protocol})
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
