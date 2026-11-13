// Design: docs/guide/ddos-mitigation.md -- a FlowSpec rule no responder owns
// Related: register.go -- announcementKey, the two check registrations
// Related: responder.go -- encodeAnnouncementRecord, decodeAnnouncementRecord, classifyRecord
//
// The record of the live announcement is judged by two checks, because no one
// process can see every case. While the plugin runs it holds its live
// responder, so it answers a runtime check itself, internal and forked alike.
// Once the plugin is not running (its section removed, the plugin stopped while
// its section is still configured, the daemon stopped) the daemon no longer
// asks it anything: collectDoctorChecksFrom
// (internal/component/doctor/cmd/show.go) asks only the processes the process
// manager still holds. A registered post-config check then reads the record
// from the store doctor opened. With the section configured it first asks the
// daemon's plugin snapshot (plugin/server PluginDoctorSnapshot) whether the
// plugin runs: configured is not running, because this plugin declares no
// failure policy, so after it ends on its own the daemon records it stopped and
// does not start it again (server/failure_policy.go applyFailurePolicy). The
// snapshot answers for both placements, because the daemon's process manager
// holds an in-process and a forked plugin alike; the package's own
// activeResponder would see only an in-process one.
package flowspec

import (
	"errors"
	"fmt"
	"io/fs"
	"strconv"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
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

// orphanRecordCheck is the registered check for the record while no plugin
// runs to answer orphanRuntimeCheck: ddos flowspec is not configured, or it is
// configured and the daemon's plugin snapshot says the plugin is not running.
// It reads config, the store and that snapshot, so it takes a config phase and
// runs in whichever process answers doctor; outside the daemon the snapshot is
// not published, and the check says what it cannot tell.
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
// responder does not hold the recorded rule as live. The report names two
// alternatives, because it cannot tell them apart. The first is that no
// withdrawal of the rule is known to have reached the BGP peers: none was sent
// (the start read the record as unreadable, and a later read decodes it), the
// engine did not take it (the engine was gone, the connection closed, the
// budget ran out), the engine refused to send it (the send permission, when no
// peer attaches the plugin), or the dispatch answered an error after the engine
// had sent it (its answer did not come back). The second is that the engine sent it and the record removal
// after it failed (responder.go withdrawThrough and withdrawRecordedRule log
// that, and end the announcement). The rule may still be at the peers in the
// first alternative only, so the report says the peers "may" still hold it.
// Its fix is the plugin restart: the restarted plugin's start
// (withdrawRecordedAnnouncement, from OnStarted) withdraws the recorded rule and
// removes the record in one step, which a withdrawal typed at the CLI does not.
func unownedOrphanDiagnostic(m flowspecMatch) rpc.DoctorCheckDiagnostic {
	victim := m.DstPrefix.String()
	var msg textbuf.Buffer
	msg.Str("ddos-flowspec holds the record of a FlowSpec rule it announced for ").Str(victim).
		Str(", and no running responder holds that rule as its live announcement. Either no withdrawal of the rule is known to have reached the BGP peers (none was sent, the BGP engine did not take it or refused to send it, or its answer did not come back), or the BGP engine sent the withdrawal and the plugin could not remove the record after it. The plugin logged why under the code ").
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
	return rpc.DoctorCheckDiagnostic{
		Code:     diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement,
		Severity: string(diagnostic.SeverityWarning),
		Message:  msg.String(),
		Fix:      withdrawalCommand(m),
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

// checkOrphanRecord reports a recorded rule no running plugin holds, reading
// the record from the store doctor opened and the plugin's run from the
// daemon's plugin snapshot.
func checkOrphanRecord(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	return judgeOrphanRecord(ctx, readPluginRun)
}

// judgeOrphanRecord is checkOrphanRecord with the question "does the plugin
// run" passed in, so a test can answer it for each case the daemon can be in.
// readRun is asked only while the section is configured.
func judgeOrphanRecord(ctx diagnostic.DoctorCheckContext, readRun func() pluginRun) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok {
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement,
			Severity: diagnostic.SeverityWarning,
			Message:  "ddos-flowspec: this doctor run holds no parsed configuration, so it cannot say whether a recorded FlowSpec rule has a running plugin to withdraw it; the command below prints the record, if one is stored",
			Fix:      recordReadCommand(),
		}}
	}
	report := orphanReport(unconfiguredOrphanDiagnostic)
	if tree.GetContainerPath(configRoot) != nil {
		var running bool
		report, running = configuredOrphanReport(readRun())
		if running {
			// refuse-by-default: guards orphanRuntimeCheckDecl -- the running plugin's runtime check judges the record against its live responder, so a second report here would repeat it or flag a healthy mitigation
			return nil
		}
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
		return doctorDiagnostics(ruleFindings(reading.rule, flowspecMatch{}, false, report))
	}
	panic("BUG: ddos-flowspec: judgeOrphanRecord met a record reading of no known kind")
}

// pluginRun is what the daemon's plugin snapshot says of ddos-flowspec. It is
// exactly one of four outcomes, each holding only its own payload:
// pluginRunning, pluginNotRunning, pluginNotRun and pluginRunUnknown. A failure
// to ask is its own outcome, never "not running": outside the daemon nothing
// can say whether a daemon holds the rule live.
type pluginRun interface{ pluginRun() }

// pluginRunning: the daemon runs the plugin, internal or forked, so its runtime
// check judges the record.
type pluginRunning struct{}

// pluginNotRunning: the daemon holds the plugin configured and it is not
// running. why is the sentence that says so, with the stop's cause when the
// daemon recorded one.
type pluginNotRunning struct{ why string }

// pluginNotRun: the daemon's plugin set does not name ddos-flowspec, so the
// daemon runs no plugin for the section this doctor run read.
type pluginNotRun struct{}

// pluginRunUnknown: this process has no plugin snapshot to ask (the local
// doctor process, or a daemon whose plugin startup has not begun).
type pluginRunUnknown struct{ err error }

func (pluginRunning) pluginRun()    {}
func (pluginNotRunning) pluginRun() {}
func (pluginNotRun) pluginRun()     {}
func (pluginRunUnknown) pluginRun() {}

// notStartedWhy is the reason of a configured plugin the daemon holds that has
// not reached running and that no stop recorded: it is starting, or between a
// stop and its record.
const notStartedWhy = "configured plugin " + Name + " has not reached the running stage"

// readPluginRun asks the daemon's plugin snapshot whether ddos-flowspec runs.
func readPluginRun() pluginRun {
	snapshot, err := pluginserver.PluginDoctorSnapshot()
	return pluginRunFromSnapshot(&snapshot, err)
}

// pluginRunFromSnapshot reads ddos-flowspec's run from one snapshot answer.
func pluginRunFromSnapshot(snapshot *pluginserver.DoctorSnapshot, err error) pluginRun {
	if err != nil {
		return pluginRunUnknown{err: err}
	}
	for i := range snapshot.Plugins {
		state := &snapshot.Plugins[i]
		if state.Name != Name {
			continue
		}
		if state.Running {
			return pluginRunning{}
		}
		if !state.Configured {
			return pluginNotRun{}
		}
		if state.Down {
			return pluginNotRunning{why: process.StoppedMessage(Name, &state.Stop)}
		}
		return pluginNotRunning{why: notStartedWhy}
	}
	return pluginNotRun{}
}

// configuredOrphanReport answers, for a configured section, the report a
// recorded rule gets, or running true when the plugin runs and its runtime
// check owns the judgement.
func configuredOrphanReport(run pluginRun) (report orphanReport, running bool) {
	switch r := run.(type) {
	case pluginRunning:
		return nil, true
	case pluginNotRunning:
		return stoppedOrphanReport(r.why), false
	case pluginNotRun:
		return unrunOrphanDiagnostic, false
	case pluginRunUnknown:
		return unknownRunOrphanReport(r.err), false
	}
	panic("BUG: ddos-flowspec: configuredOrphanReport met a plugin run of no known kind")
}

// stoppedOrphanReport is the report while ddos flowspec is configured and the
// plugin is not running, so neither its runtime check nor a responder answers
// for the rule. Its fix is the plugin restart: the start
// (withdrawRecordedAnnouncement, from OnStarted) withdraws the recorded rule
// and removes the record, which a withdrawal typed at the CLI does not.
func stoppedOrphanReport(why string) orphanReport {
	return func(m flowspecMatch) rpc.DoctorCheckDiagnostic {
		victim := m.DstPrefix.String()
		var msg textbuf.Buffer
		msg.Str("ddos-flowspec holds the record of a FlowSpec rule it announced for ").Str(victim).
			Str(", and no running plugin holds that rule: ").Str(why).
			Str(". Nothing withdraws the rule until the plugin runs again. Every BGP peer that took the rule may still discard or rate-limit traffic to ").Str(victim).
			Str(". A restart of the plugin withdraws the recorded rule and removes the record; a second withdrawal of a rule the peers no longer hold changes nothing. If that withdrawal fails, the plugin logs the cause under the code ").
			Str(diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement).
			Str(", and the running plugin's own check reports the record")
		return rpc.DoctorCheckDiagnostic{
			Code:     diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement,
			Severity: string(diagnostic.SeverityWarning),
			Message:  msg.String(),
			Fix:      process.RestartCommand(Name),
		}
	}
}

// unrunOrphanDiagnostic is the report while the configuration doctor read
// names ddos flowspec and the daemon's plugin set does not name the plugin:
// the daemon loaded another configuration. No plugin runs to restart, so its
// fix is the exact withdrawal the plugin would send.
func unrunOrphanDiagnostic(m flowspecMatch) rpc.DoctorCheckDiagnostic {
	victim := m.DstPrefix.String()
	var msg textbuf.Buffer
	msg.Str("ddos-flowspec announced a FlowSpec rule for ").Str(victim).
		Str(", and the configuration names ddos flowspec, but the daemon runs no ddos-flowspec plugin, because the configuration it loaded does not name the section, so nothing withdraws the rule. Every BGP peer that took the rule may still discard or rate-limit traffic to ").Str(victim).
		Str(". The command below withdraws the rule at the peers. It does not remove the record, so this report stays: the plugin removes the record when it next starts (load the configuration that names ddos flowspec, and its start withdraws the rule and removes the record), and ")
	recordRemovalToday(&msg)
	return rpc.DoctorCheckDiagnostic{
		Code:     diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement,
		Severity: string(diagnostic.SeverityWarning),
		Message:  msg.String(),
		Fix:      withdrawalCommand(m),
	}
}

// unknownRunOrphanReport is the report of a doctor run that holds no plugin
// snapshot to ask: it found the record and cannot tell whether a running
// plugin holds the rule as its live announcement, which is the normal state of
// a running mitigation. It says both, and its fix asks the daemon, whose run of
// this check can tell.
func unknownRunOrphanReport(err error) orphanReport {
	return func(m flowspecMatch) rpc.DoctorCheckDiagnostic {
		victim := m.DstPrefix.String()
		var msg textbuf.Buffer
		msg.Str("ddos-flowspec holds the record of a FlowSpec rule it announced for ").Str(victim).
			Str(", and ddos flowspec is configured, but this doctor run cannot tell whether the plugin runs and holds the rule as its live announcement (").Err(err).
			Str("). A running mitigation keeps this record, so the record alone is not a fault. While ze runs, the command below asks the daemon, which can tell. While ze is stopped, the next start of the plugin withdraws the recorded rule and removes the record")
		return rpc.DoctorCheckDiagnostic{
			Code:     diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement,
			Severity: string(diagnostic.SeverityWarning),
			Message:  msg.String(),
			Fix:      daemonDoctorCommand,
		}
	}
}

// daemonDoctorCommand is the doctor run through the daemon, whose plugin
// snapshot says whether the plugin runs.
const daemonDoctorCommand = "ze show doctor"

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
