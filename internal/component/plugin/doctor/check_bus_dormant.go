// Design: docs/architecture/doctor-and-health-checks.md -- dormant bus reception check
// Related: register_delivery.go -- registers this check at the runtime phase
// Related: checks_delivery.go -- snapshotCheck adapter and sibling delivery row builders
// Related: ../server/doctor_snapshot.go -- DoctorSnapshot.DormantReceptions, the state it reads

package doctor

import (
	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// dormantReceptionRows answers one informational diagnostic per dormant bus
// reception in snapshot: a declared reception that currently produces no active
// delivery edge (for example because no sender of the message is configured).
func dormantReceptionRows(snapshot *server.DoctorSnapshot) []diagnostic.Diagnostic {
	rows := make([]diagnostic.Diagnostic, 0, len(snapshot.DormantReceptions))
	for i := range snapshot.DormantReceptions {
		rows = append(rows, dormantReceptionRow(&snapshot.DormantReceptions[i]))
	}
	return rows
}

// dormantReceptionRow formats one dormant reception finding with the receiving
// participant path, the message name, the missing-sender or filter reason, and
// the operator action to activate or inspect it.
func dormantReceptionRow(dr *server.BusDormantReception) diagnostic.Diagnostic {
	var message textbuf.Buffer
	message.Str("bus reception on ").Str(dr.ParticipantPath).Str(" for message ").Str(dr.Message).
		Str(" is dormant")
	if dr.Reason != "" {
		message.Str(": ").Str(dr.Reason)
	} else {
		message.Str(": no matching sender is currently active")
	}
	var fix textbuf.Buffer
	fix.Str("configure a sender that declares ").Str(dr.Message).
		Str(" (and restart ze if adding an external endpoint), or inspect active edges with `ze show event delivery`")
	return diagnostic.Diagnostic{
		Code:     codeDoctorBusDormantReception,
		Severity: diagnostic.SeverityInfo,
		Message:  message.String(),
		Fix:      fix.String(),
	}
}
