// Design: docs/guide/bgp-policy.md -- distinct ordinary learned-export absence.
// Related: rfc8212_warning.go -- external RFC8212 words and editor paths stay shared.
package reactor

import (
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// PolicyAbsence describes why an unconfigured direction blocks route output.
// Code identifies the explanation; it grants no route or process authority.
type PolicyAbsence struct {
	Direction string
	Message   string
	Fix       string
	Code      string
}

// PolicyAbsences MUST receive the resolved owner for the same destination.
// A config caller supplies its validated graph result; a live caller supplies
// its captured dissemination publication. Names and client flags grant nothing.
func PolicyAbsences(settings *PeerSettings, owner pluginserver.DisseminationOwner) []PolicyAbsence {
	var absences []PolicyAbsence
	for _, absent := range RFC8212Absences(settings) {
		absences = append(absences, PolicyAbsence{Direction: absent.Direction, Message: absent.Message, Fix: absent.Fix,
			Code: diagnostic.CodeDoctorBGPRFC8212NoPolicy})
	}
	absences = append(absences, learnedExportAbsences(settings, owner)...)

	return absences
}

// learnedExportAbsences is the single condition, explanation and fix for
// ordinary internal learned export without a policy. An empty list means this
// absence does not apply; an explicit role or configured rejection stays quiet.
func learnedExportAbsences(settings *PeerSettings, owner pluginserver.DisseminationOwner) []PolicyAbsence {
	if !settings.IsIBGP() {
		return nil
	}
	if !owner.IsSet() {
		return nil
	}
	if owner.Explicit() {
		return nil
	}
	if settings.ExportPolicy.Configured() {
		return nil
	}
	return []PolicyAbsence{{Direction: "export",
		Message: "This IBGP peer has no export policy, so Ze sends it no routes learned from other peers. Local announcements remain permitted",
		Fix:     rfc8212Fix(settings, "export"), Code: diagnostic.CodeDoctorBGPLearnedExportNoPolicy}}
}

// learnedExportAbsencesForWarning takes no configuration fallback for live
// ownership: an absent or failed publication must not imply ordinary output.
func (p *Peer) learnedExportAbsencesForWarning(settings *PeerSettings) []PolicyAbsence {
	var owner pluginserver.DisseminationOwner
	snapshot := p.selectedOwnerSnapshot()
	if snapshot != nil && snapshot.err == nil && snapshot.peer == p && snapshot.session == p.currentSession() {
		owner = snapshot.owner
	}
	return learnedExportAbsences(settings, owner)
}
