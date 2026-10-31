// Design: docs/guide/bgp-policy.md -- Policy model, the RFC 8212 default for an absent direction
// Related: peer_run.go -- the Established transition that calls warnRFC8212AbsentPolicy
// Related: ../filterapi/policy.go -- Policy.AcceptsAtEnd, the verdict this warning explains

package reactor

import (
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// rfc8212Direction is one policy direction of a peer, with the words the
// establishment warning uses for it.
type rfc8212Direction struct {
	name    string // the config container under filter: import or export
	blocked string // the warning's message: what RFC 8212 stops on this session
}

var (
	rfc8212Import = rfc8212Direction{
		name:    "import",
		blocked: "RFC 8212: this EBGP peer has no import policy, so Ze uses none of the routes it receives from it",
	}
	rfc8212Export = rfc8212Direction{
		name:    "export",
		blocked: "RFC 8212: this EBGP peer has no export policy, so Ze sends it no routes",
	}
)

// warnRFC8212AbsentPolicy logs one warning for each direction that RFC 8212
// closes on the session that just reached Established. It MUST run once per
// establishment, after resolveDynamicPeerSettings, so a dynamic peer is judged
// on the AS its OPEN carried. An IBGP session and a configured direction log
// nothing. Every input comes from one settings snapshot, so the session type
// and the two policies cannot come from different reloads.
//
// The warning keeps no state: a new session warns again, because the operator
// who missed the first line meets the same silence on the next session.
func (p *Peer) warnRFC8212AbsentPolicy() {
	settings := p.settingsSnapshot()
	if settings.IsIBGP() {
		return
	}
	// RFC 8212 Section 3: "Routes contained in an Adj-RIB-In associated with an EBGP peer SHALL NOT be considered eligible in the Decision Process if no explicit Import Policy has been applied."
	warnRFC8212Direction(settings, settings.ImportPolicy, rfc8212Import)
	// RFC 8212 Section 3: "Routes SHALL NOT be added to an Adj-RIB-Out associated with an EBGP peer if no explicit Export Policy has been applied."
	warnRFC8212Direction(settings, settings.ExportPolicy, rfc8212Export)
}

// warnRFC8212Direction logs the warning for one direction when no layer
// configured it. Automatic and inactive references are not a configured
// policy (Policy.Configured), the same test Policy.AcceptsAtEnd applies.
func warnRFC8212Direction(settings *PeerSettings, policy filterapi.Policy, direction rfc8212Direction) {
	if policy.Configured() {
		return
	}
	peerLogger().Warn(direction.blocked,
		"peer", settings.Name,
		"address", settings.Address,
		"direction", direction.name,
		"fix", rfc8212Fix(settings, direction.name),
		"then", "commit now")
}

// rfc8212Fix returns the configuration editor command that gives the absent
// direction an explicit policy at the level the peer is configured at. A
// dynamic peer is configured only through its group, so the command names the
// group and never the generated dyn- name. `final accept` is an example of an
// explicit policy, and accepting every route is the operator's decision.
func rfc8212Fix(settings *PeerSettings, direction string) string {
	var tb textbuf.Buffer
	tb.Str("set bgp ")
	switch {
	case settings.IsDynamic:
		tb.Str("group ").Str(settings.GroupName)
	case settings.GroupName != "":
		tb.Str("group ").Str(settings.GroupName).Str(" peer ").Str(settings.Name)
	default:
		tb.Str("peer ").Str(settings.Name)
	}
	tb.Str(" filter ").Str(direction).Str(" final accept")
	return tb.String()
}
