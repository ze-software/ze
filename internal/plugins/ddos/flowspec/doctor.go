// Design: docs/guide/ddos-mitigation.md -- a FlowSpec rule no responder owns
// Related: register.go -- announcementKey, the two check registrations
// Related: responder.go -- encodeAnnouncementRecord, decodeAnnouncementRecord, classifyRecord
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
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin/process"
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
	switch reading := readRecordedAnnouncement(d).(type) {
	case recordAbsent:
		// No record is the healthy answer: every rule this plugin announced was
		// withdrawn.
		return nil, nil
	case recordUnreadable:
		return nil, fmt.Errorf("ddos-flowspec could not read its announcement record, so it cannot say whether a FlowSpec rule is left at the BGP peers: %w", reading.err)
	case recordCorrupt:
		return []rpc.DoctorCheckDiagnostic{corruptRecordDiagnostic(reading.err)}, nil
	case recordedRule:
		return ruleFindings(reading.rule, live, isLive, unownedOrphanDiagnostic), nil
	}
	panic("BUG: ddos-flowspec: answerOrphanRuntimeCheck met a record reading of no known kind")
}

// orphanReport builds the report of a recorded rule nothing withdraws. Each
// check passes its own, because what clears the finding differs: a running
// plugin withdraws the rule when it restarts, while with the section removed no
// plugin runs to withdraw it.
type orphanReport func(m flowspecMatch) rpc.DoctorCheckDiagnostic

// ruleFindings judges the recorded rule m against the live announcement. A
// record the live responder holds is the normal state of a running
// mitigation. Any other recorded rule may still be at the peers, and report
// reports it.
func ruleFindings(m, live flowspecMatch, isLive bool, report orphanReport) []rpc.DoctorCheckDiagnostic {
	if isLive {
		if m == live {
			return nil
		}
	}
	return []rpc.DoctorCheckDiagnostic{report(m)}
}

// recordRemovalToday says what removes the record while no plugin start does.
// The store refuses a writer beside the daemon that owns it (storage open.go
// lockStoreFile answers ErrBusy), so `ze data rm` removes the record only while
// ze is stopped.
func recordRemovalToday(msg *textbuf.Buffer) {
	msg.Str("ze data rm ").Str(announcementKey.Pattern).
		Str(" removes the record only while ze is stopped, because the store refuses a second writer while the daemon owns it")
}

// unownedOrphanDiagnostic is the runtime report: the plugin runs and its
// responder does not hold the recorded rule as live, so the record outlived the
// last withdrawal of that rule. Three causes leave it: the engine did not take
// the withdrawal (the engine was gone, the connection closed, the budget ran
// out); the engine took it and refused to send it (the send permission, when
// no peer attaches the plugin); or the engine sent it and the record removal
// after it failed (responder.go withdrawThrough and withdrawRecordedRule log
// that, and end the announcement). Only the first two leave the rule at the
// peers, and the report cannot tell the three apart, so it names all three and
// says the peers "may" still hold the rule. Its fix is the plugin restart: the restarted plugin's start
// (withdrawRecordedAnnouncement, from OnStarted) withdraws the recorded rule and
// removes the record in one step, which a withdrawal typed at the CLI does not.
func unownedOrphanDiagnostic(m flowspecMatch) rpc.DoctorCheckDiagnostic {
	victim := m.DstPrefix.String()
	var msg textbuf.Buffer
	msg.Str("ddos-flowspec holds the record of a FlowSpec rule it announced for ").Str(victim).
		Str(", and no running responder holds that rule as its live announcement. Either the last withdrawal of the rule did not reach the BGP peers, because the BGP engine did not take it or refused to send it, or the BGP engine sent the withdrawal and the plugin could not remove the record after it. The plugin logged why under the code ").
		Str(diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement).
		Str(". Every BGP peer that took the rule may still discard or rate-limit traffic to ").Str(victim).
		Str(". A restart of the plugin withdraws the recorded rule and removes the record; a second withdrawal of a rule the peers no longer hold changes nothing. If the cause of the failed withdrawal or removal remains, the restart fails the same way, the plugin logs the cause again, and this report stays")
	return rpc.DoctorCheckDiagnostic{
		Code:     diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement,
		Severity: string(diagnostic.SeverityWarning),
		Message:  msg.String(),
		Fix:      process.RestartCommand(Name),
	}
}

// unconfiguredOrphanDiagnostic is the report while ddos flowspec is not
// configured: no plugin runs to restart, so its fix is the exact withdrawal the
// plugin would send. That withdrawal does not remove the record, and the
// message says what does.
func unconfiguredOrphanDiagnostic(m flowspecMatch) rpc.DoctorCheckDiagnostic {
	victim := m.DstPrefix.String()
	var msg textbuf.Buffer
	msg.Str("ddos-flowspec announced a FlowSpec rule for ").Str(victim).
		Str(", and the ddos flowspec section is not configured, so no running plugin can withdraw it. Every BGP peer that took the rule may still discard or rate-limit traffic to ").Str(victim).
		Str(". The command below withdraws the rule at the peers. It does not remove the record, so this report stays: the plugin removes the record when it next starts (configure ddos flowspec again, and its start withdraws the rule and removes the record), and ")
	recordRemovalToday(&msg)
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

// corruptRecordDiagnostic reports a record that does not decode, for both
// checks. Nothing is rendered from it, and no plugin start withdraws from it
// (readRecordedAnnouncement), so a restart does not clear it. Its fix prints the
// stored bytes, which can still show the rule's victim, so the operator can
// withdraw that rule at the peers.
func corruptRecordDiagnostic(err error) rpc.DoctorCheckDiagnostic {
	var msg textbuf.Buffer
	msg.Str("ddos-flowspec holds a record of an announced FlowSpec rule that it cannot read (").Err(err).
		Str("), so ze cannot say which rule may still be at the BGP peers, and the plugin withdraws nothing from it, at start or after a restart. The command below prints the stored record, which can still show the rule's victim prefix: withdraw that rule at the peers with ").
		Str(manualWithdrawalCommand()).
		Str(", or ipv6/flow and destination-ipv6 for an IPv6 victim. The record and this report stay until the plugin records a new announcement over it, and ")
	recordRemovalToday(&msg)
	return rpc.DoctorCheckDiagnostic{
		Code:     diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement,
		Severity: string(diagnostic.SeverityWarning),
		Message:  msg.String(),
		Fix:      recordReadCommand(),
	}
}

// manualWithdrawalCommand is the command that withdraws, by hand, the FlowSpec
// rule for an IPv4 victim the record no longer names: what an operator types
// when the record that named the rule could not be read or decoded.
func manualWithdrawalCommand() string {
	var tb textbuf.Buffer
	return tb.Str(`ze cli -c "send bgp `).Str(flowspecSelector).
		Str(` update text mp ipv4/flow del destination-ipv4 <victim>"`).String()
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
	switch reading := storedRecordReading(ctx.Store).(type) {
	case recordAbsent:
		// No record is the healthy answer: every rule the plugin announced was
		// withdrawn.
		return nil
	case recordUnreadable:
		var msg textbuf.Buffer
		msg.Str("ddos-flowspec: the announcement record could not be read (").Err(reading.err).
			Str("), so doctor cannot say whether a FlowSpec rule a stopped plugin announced is still at the BGP peers")
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement,
			Severity: diagnostic.SeverityWarning,
			Message:  msg.String(),
			Fix:      "ze data check",
		}}
	case recordCorrupt:
		return doctorDiagnostics([]rpc.DoctorCheckDiagnostic{corruptRecordDiagnostic(reading.err)})
	case recordedRule:
		return doctorDiagnostics(ruleFindings(reading.rule, flowspecMatch{}, false, unconfiguredOrphanDiagnostic))
	}
	panic("BUG: ddos-flowspec: checkOrphanRecord met a record reading of no known kind")
}

// storedRecordReading reads the record from the store doctor opened. The
// store's not-exist answer is the absence of the record, never a failure to
// read it; every other error is.
func storedRecordReading(store storage.Storage) recordReading {
	data, err := store.ReadKey(announcementKey.Pattern)
	if errors.Is(err, fs.ErrNotExist) {
		return classifyRecord(nil, false, nil)
	}
	return classifyRecord(data, true, err)
}

// doctorDiagnostics carries the findings of the shared report builders over to
// the registered check's diagnostic type.
func doctorDiagnostics(findings []rpc.DoctorCheckDiagnostic) []diagnostic.Diagnostic {
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
