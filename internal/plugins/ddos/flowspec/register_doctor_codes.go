// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/ddos/flowspec.

package flowspec

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorDDoSFlowspecOrphanAnnouncement = "doctor-ddos-flowspec-orphan-announcement"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorDDoSFlowspecOrphanAnnouncement,
		Title:       "A DDoS FlowSpec rule may still be at the BGP peers with no responder to withdraw it",
		Description: "ddos-flowspec records each FlowSpec rule it announces in its runtime state key meta/ddos-flowspec/announcement and removes the record when the rule is withdrawn. A record that no running responder holds as its live announcement means one of three things. No withdrawal of the rule was sent at all: the plugin or the daemon stopped, or was killed, with the rule announced, or the ddos flowspec section was removed while the plugin was not running. A withdrawal was sent and is not known to have reached the BGP peers, because the BGP engine did not take it, refused to send it, or its answer did not come back, on the way out, at start, or when the section was removed. Or the BGP engine sent the withdrawal and the plugin could not remove the record after it. Until the rule is withdrawn, every BGP peer that took it may still discard or rate-limit traffic to the victim prefix. The plugin withdraws a recorded rule and removes its record each time it starts; a second withdrawal of a rule the peers no longer hold changes nothing. To withdraw it now, send the withdrawal the report prints, then remove the record with ze data rm meta/ddos-flowspec/announcement, which works while ze runs. Keep that order: a record removed before its rule is withdrawn leaves the rule at the peers with no trace in the store and no report to find it. ze refuses the removal while a ddos-flowspec plugin runs, because that plugin manages the record: then run ze request plugin restart ddos-flowspec, whose start withdraws the recorded rule and removes the record. A record that cannot be decoded is reported as corrupt and never sent: check the peers for a FlowSpec rule ze announced, withdraw it, then remove the record; while a ddos-flowspec plugin runs, its next announcement replaces the record.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-ddos-flowspec-orphan-announcement"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: flowspec: doctor code registration refused: " + err.Error())
		}
	}
}
