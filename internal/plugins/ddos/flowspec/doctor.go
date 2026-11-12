// Design: docs/guide/ddos-mitigation.md -- a FlowSpec rule no responder owns
// Related: register.go -- announcementKey, the two check registrations
// Related: responder.go -- encodeAnnouncementRecord, decodeAnnouncementRecord
//
// The record of the live announcement is judged by two checks, because no one
// process can see every case. While ddos flowspec is configured the plugin runs
// and holds its live responder, so it answers a runtime check itself, internal
// and forked alike. Once the plugin is gone (its section removed, the daemon
// stopped) the daemon no longer asks it anything: collectDoctorChecksFrom
// (internal/component/doctor/cmd/show.go) asks only the processes the process
// manager still holds. A registered post-config check then reads the record
// from the store doctor opened, whenever the section is absent.
package flowspec

import (
	"errors"
	"fmt"
	"io/fs"
	"strconv"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// orphanRuntimeCheck is the plugin runtime check over the record and the live
// responder. It is declared in the SDK registration (register.go) and answered
// by the plugin's OnDoctorCheck callback, because only the running plugin holds
// the responder.
const orphanRuntimeCheck = "ddos-flowspec-orphan-announcement"

// orphanRuntimeCheckDecl is the Stage 1 declaration of orphanRuntimeCheck.
var orphanRuntimeCheckDecl = rpc.DoctorCheckDecl{
	Name:  orphanRuntimeCheck,
	Phase: rpc.DoctorPhaseRuntime,
	Codes: []string{diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement},
}

// orphanRecordCheck is the registered check for the record while ddos flowspec
// is not configured, so no plugin runs to answer orphanRuntimeCheck. It reads
// config and the store only, so it takes a config phase and runs in whichever
// process answers doctor.
var orphanRecordCheck = diagnostic.DoctorCheck{
	Name:         "ddos-flowspec-orphan-record",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        761,
	Component:    Name,
	Dependencies: []string{"config-loaded"},
	Platforms:    []string{"any"},
	Codes: []string{
		diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement,
		diagnostic.CodeDoctorStorageUnavailable,
	},
	Check: checkOrphanRecord,
}

// whyUnowned and whyUnconfigured complete the orphan report's first sentence,
// one for each check.
const (
	whyUnowned      = "no running responder holds it as its live announcement, so a withdrawal at start or on the way out did not reach the BGP engine"
	whyUnconfigured = "the ddos flowspec section is not configured, so no running plugin can withdraw it"
)

// liveMatch returns the match the live responder announces. live is false
// while no responder runs or the running one announces nothing.
func liveMatch() (m flowspecMatch, live bool) {
	r := activeResponder.Load()
	if r == nil {
		return flowspecMatch{}, false
	}
	active, target, _ := r.status()
	if !active {
		return flowspecMatch{}, false
	}
	return buildMatch(target), true
}

// answerOrphanRuntimeCheck answers the plugin doctor callback. live and isLive
// are what liveMatch returned when the request arrived.
func answerOrphanRuntimeCheck(name string, d routeDispatcher, live flowspecMatch, isLive bool) ([]rpc.DoctorCheckDiagnostic, error) {
	if name != orphanRuntimeCheck {
		return nil, errors.New("ddos-flowspec has no doctor check named " + strconv.Quote(name) +
			"; it declares only " + orphanRuntimeCheck)
	}
	data, found, err := d.Recorded()
	if err != nil {
		return nil, fmt.Errorf("ddos-flowspec could not read its announcement record, so it cannot say whether a FlowSpec rule is left at the BGP peers: %w", err)
	}
	return orphanFindings(data, found, live, isLive, whyUnowned), nil
}

// orphanFindings judges the stored record against the live announcement. A
// record the live responder holds is the normal state of a running
// mitigation. Any other record names a rule that may still be at the peers,
// and a record that does not decode is reported as corrupt and never rendered.
func orphanFindings(data []byte, found bool, live flowspecMatch, isLive bool, why string) []rpc.DoctorCheckDiagnostic {
	// refuse-by-default: guards answerOrphanRuntimeCheck -- no record is the healthy answer: every rule this plugin announced was withdrawn
	if !found {
		return nil
	}
	m, err := decodeAnnouncementRecord(data)
	if err != nil {
		return []rpc.DoctorCheckDiagnostic{corruptRecordDiagnostic(err)}
	}
	if isLive {
		if m == live {
			return nil
		}
	}
	return []rpc.DoctorCheckDiagnostic{orphanDiagnostic(m, why)}
}

// orphanDiagnostic reports a recorded rule no responder owns. Its fix is the
// exact withdrawal the plugin would send.
func orphanDiagnostic(m flowspecMatch, why string) rpc.DoctorCheckDiagnostic {
	victim := m.DstPrefix.String()
	var msg textbuf.Buffer
	msg.Str("ddos-flowspec announced a FlowSpec rule for ").Str(victim).Str(" and ").Str(why).
		Str(", so every BGP peer that took the rule may still discard or rate-limit traffic to ").Str(victim).
		Str(". The plugin withdraws a recorded rule each time it starts. To withdraw it now, run the command below, then remove the record with: ze data rm ").
		Str(announcementKey.Pattern)
	var fix textbuf.Buffer
	fix.Str(`ze cli -c "send bgp `).Str(flowspecSelector).Byte(' ').
		Str(renderFlowspecCommand(m, "", 0, "del")).Byte('"')
	return rpc.DoctorCheckDiagnostic{
		Code:     diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement,
		Severity: string(diagnostic.SeverityWarning),
		Message:  msg.String(),
		Fix:      fix.String(),
	}
}

// corruptRecordDiagnostic reports a record that does not decode. Nothing is
// rendered from it, so the operator finds the rule at the peers.
func corruptRecordDiagnostic(err error) rpc.DoctorCheckDiagnostic {
	var msg textbuf.Buffer
	msg.Str("ddos-flowspec holds a record of an announced FlowSpec rule that it cannot read (").Err(err).
		Str("), so ze cannot say which rule may still be at the BGP peers, and the plugin does not withdraw anything from it. Find the FlowSpec rule ze announced in a peer's flow routes and withdraw it, then remove the record")
	var fix textbuf.Buffer
	fix.Str("ze data rm ").Str(announcementKey.Pattern)
	return rpc.DoctorCheckDiagnostic{
		Code:     diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement,
		Severity: string(diagnostic.SeverityWarning),
		Message:  msg.String(),
		Fix:      fix.String(),
	}
}

// recordReadCommand is the command that prints the stored announcement record.
// It is the Fix of a doctor run that could not judge the record itself (no
// parsed configuration, no storage): reading the record is what answers whether
// a rule is left at the peers. `ze data cat` opens the live store on its own
// (internal/component/config/storage/cli catAnswer) and, when it cannot, its
// refusal names why.
func recordReadCommand() string {
	var tb textbuf.Buffer
	return tb.Str("ze data cat ").Str(announcementKey.Pattern).String()
}

// checkOrphanRecord reports a recorded rule while ddos flowspec is not
// configured, reading the record from the store doctor opened.
func checkOrphanRecord(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok {
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement,
			Severity: diagnostic.SeverityWarning,
			Message:  "ddos-flowspec: this doctor run holds no parsed configuration, so it cannot say whether a recorded FlowSpec rule has a running plugin to withdraw it; the command below prints the record, if one is stored",
			Fix:      recordReadCommand(),
		}}
	}
	if tree.GetContainerPath(configRoot) != nil {
		// refuse-by-default: guards orphanRuntimeCheckDecl -- a configured section runs the plugin, whose runtime check judges the record against its live responder
		return nil
	}
	if ctx.Store == nil {
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorStorageUnavailable,
			Severity: diagnostic.SeverityWarning,
			Message:  "ddos-flowspec FlowSpec record: refused, this doctor run resolved no storage, so it cannot say whether a FlowSpec rule a stopped plugin announced is still at the BGP peers; the command below reads the record from the live store, or names why that store cannot be opened",
			Fix:      recordReadCommand(),
		}}
	}
	data, err := ctx.Store.ReadKey(announcementKey.Pattern)
	if errors.Is(err, fs.ErrNotExist) {
		// refuse-by-default: guards orphanFindings -- no record is the healthy answer: every rule the plugin announced was withdrawn
		return nil
	}
	if err != nil {
		var msg textbuf.Buffer
		msg.Str("ddos-flowspec: the announcement record could not be read (").Err(err).
			Str("), so doctor cannot say whether a FlowSpec rule a stopped plugin announced is still at the BGP peers")
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement,
			Severity: diagnostic.SeverityWarning,
			Message:  msg.String(),
			Fix:      "ze data check",
		}}
	}
	findings := orphanFindings(data, true, flowspecMatch{}, false, whyUnconfigured)
	out := make([]diagnostic.Diagnostic, 0, len(findings))
	for _, f := range findings {
		out = append(out, diagnostic.Diagnostic{
			Code:     f.Code,
			Severity: diagnostic.Severity(f.Severity),
			Message:  f.Message,
			Fix:      f.Fix,
		})
	}
	return out
}
