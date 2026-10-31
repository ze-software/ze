// Design: docs/guide/bgp-policy.md -- Policy model, the RFC 8212 default for an absent direction
// Related: peer_run.go -- the Established transition that calls warnRFC8212AbsentPolicy
// Related: ../filterapi/policy.go -- Policy.AcceptsAtEnd, the verdict this warning explains

package reactor

import (
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

// RFC8212Absence is one policy direction RFC 8212 closes on an EBGP session
// because no layer configured it. `ze config validate` reports it before the
// daemon starts and the reactor logs it when the session is established, from
// this one value, so the two surfaces cannot word the problem differently.
type RFC8212Absence struct {
	Direction string // the config container under filter: import or export
	Message   string // what RFC 8212 stops on this session
	Fix       string // the configuration editor command that configures the direction
}

// RFC8212Absences answers the directions RFC 8212 closes on the session the
// settings describe: none for an IBGP session or a configured direction, one
// entry per unconfigured direction otherwise. Automatic and inactive
// references are not a configured policy (Policy.Configured), the same test
// Policy.AcceptsAtEnd applies. The caller MUST pass settings whose session type
// is known: a dynamic member is judged on the AS its OPEN carried.
func RFC8212Absences(settings *PeerSettings) []RFC8212Absence {
	if settings.IsIBGP() {
		return nil
	}
	var absences []RFC8212Absence
	// RFC 8212 Section 3: "Routes contained in an Adj-RIB-In associated with an EBGP peer SHALL NOT be considered eligible in the Decision Process if no explicit Import Policy has been applied."
	if !settings.ImportPolicy.Configured() {
		absences = append(absences, rfc8212AbsenceOf(settings, rfc8212Import))
	}
	// RFC 8212 Section 3: "Routes SHALL NOT be added to an Adj-RIB-Out associated with an EBGP peer if no explicit Export Policy has been applied."
	if !settings.ExportPolicy.Configured() {
		absences = append(absences, rfc8212AbsenceOf(settings, rfc8212Export))
	}
	return absences
}

// rfc8212AbsenceOf builds the absence of one direction on the session.
func rfc8212AbsenceOf(settings *PeerSettings, direction rfc8212Direction) RFC8212Absence {
	return RFC8212Absence{
		Direction: direction.name,
		Message:   direction.blocked,
		Fix:       rfc8212Fix(settings, direction.name),
	}
}

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
	for _, absence := range RFC8212Absences(settings) {
		peerLogger().Warn(absence.Message,
			"peer", settings.Name,
			"address", settings.Address,
			"direction", absence.Direction,
			"fix", absence.Fix,
			"then", "commit now")
	}
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
