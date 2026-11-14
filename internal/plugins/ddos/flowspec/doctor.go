// Design: docs/guide/ddos-mitigation.md -- a FlowSpec rule no responder owns
// Related: register.go -- announcementKey, the two check registrations
// Related: responder.go -- encodeAnnouncementRecord, decodeAnnouncementRecord, classifyRecord
//
// The record of the live announcement is judged by three checks, because no
// one process can see every case. While the plugin runs it holds its live
// responder, so it answers a runtime check itself (orphanRuntimeCheck). Once
// the plugin is not running the daemon no longer asks it anything:
// collectDoctorChecksFrom (internal/component/doctor/cmd/show.go) asks only
// the processes the process manager still holds. Two registered checks then
// read the record:
//
//   - orphanRecordCheck, at the post-config phase, while the configuration
//     doctor read does not name ddos flowspec. It reads config and the store
//     only, so it runs in whichever process answers doctor.
//   - stoppedRecordCheck, at the runtime phase, while the daemon holds no
//     running ddos-flowspec process: the plugin configured and not running, or
//     no process of that name while the saved configuration doctor read names
//     ddos flowspec (or doctor read none). It reads the daemon's plugin
//     snapshot (plugin/server PluginDoctorSnapshot) and the daemon's store
//     (statestore), live state only the daemon holds, so only the daemon route
//     runs it; the judged tree tells it only which records orphanRecordCheck
//     already reported. Configured is not running: a stopped startup
//     handshake, or a run that ended under no failure policy, leaves the
//     plugin recorded stopped and not started again (server/startup.go,
//     server/failure_policy.go applyFailurePolicy).
//
// Without a daemon the local route runs no runtime check, and its
// doctor-runtime-not-checked row says so (internal/component/doctor
// local_route.go runtimeNotChecked).
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
	"github.com/ze-software/ze/internal/core/statestore"
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
		// refuse-by-default: guards stoppedRecordCheck -- with the section configured, the running plugin's runtime check (orphanRuntimeCheckDecl) judges the record against its live responder, and stoppedRecordCheck judges it in the daemon while no running ddos-flowspec process holds it, reading this same tree to know the section is configured; this phase also runs with no daemon, where nothing can tell a running mitigation from a stopped plugin
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
	return storedRecordFindings(storedRecordReading(ctx.Store), unconfiguredOrphanDiagnostic)
}

// storedRecordFindings judges one reading of the stored record for the two
// registered checks: report builds the report of a recorded rule, because what
// clears it differs between them.
func storedRecordFindings(reading recordReading, report orphanReport) []diagnostic.Diagnostic {
	switch reading := reading.(type) {
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
	panic("BUG: ddos-flowspec: storedRecordFindings met a record reading of no known kind")
}

// stoppedRecordCheck is the registered check for the record while the daemon
// holds no running ddos-flowspec plugin process, so no running plugin answers
// orphanRuntimeCheck, and the saved configuration doctor read leaves the record
// to it. It reads the daemon's plugin snapshot and the daemon's store, state
// only the daemon holds, so it takes the runtime phase: only the daemon route
// runs it (doctor.RunRuntimeChecks). A runtime check is handed no store, so it
// reads the store statestore publishes, the one the plugin's state calls
// write. It is handed the tree the post-config phase judged, and reads it only
// to agree with orphanRecordCheck, which judged the same saved file: never as
// what the daemon runs.
var stoppedRecordCheck = diagnostic.DoctorCheck{
	Name:         "ddos-flowspec-stopped-record",
	Phase:        diagnostic.DoctorPhaseRuntime,
	Order:        762,
	Component:    Name,
	Dependencies: []string{"plugin-state"},
	Platforms:    []string{"any"},
	Codes: []string{
		diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement,
		diagnostic.CodeDoctorStorageUnavailable,
	},
	Check: checkStoppedRecord,
}

// checkStoppedRecord reports a recorded rule while the daemon holds no running
// ddos-flowspec plugin process and no other check reports it.
func checkStoppedRecord(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	return judgeStoppedRecord(judgedTree(ctx), readPluginRun(), daemonRecordReading)
}

// judgedTree answers the tree the post-config phase judged, as the runtime
// phase hands it over (doctor RunRuntimeChecks). The runner hands a nil tree
// as either a nil interface or a nil *config.Tree; both mean that phase did not
// run. Any other value only a runner defect produces.
func judgedTree(ctx diagnostic.DoctorCheckContext) *config.Tree {
	if ctx.Tree == nil {
		// refuse-by-default: guards unrunReport -- a nil tree is the answer "the post-config phase did not run", which unrunReport reports as what doctor cannot tell
		return nil
	}
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok {
		panic("BUG: ddos-flowspec: the runtime phase handed a tree that is not *config.Tree")
	}
	return tree
}

// judgeStoppedRecord is checkStoppedRecord with the judged tree, the plugin's
// run and the record's reading passed in, so a test can answer each for every
// case the daemon can be in. readRecord is asked only when the run leaves the
// record to this check.
func judgeStoppedRecord(judged *config.Tree, run pluginRun, readRecord func() recordReading) []diagnostic.Diagnostic {
	report, owned := stoppedRunReport(run, judged)
	if !owned {
		// refuse-by-default: guards orphanRecordCheck -- another check owns the record: a running plugin's own runtime check (orphanRuntimeCheckDecl) judges it, or, while the saved configuration doctor read does not name ddos flowspec, the post-config orphanRecordCheck reported it from that same tree; a report here would list it twice
		return nil
	}
	reading := readRecord()
	if unreadable, ok := reading.(recordUnreadable); ok {
		if errors.Is(unreadable.err, statestore.ErrUnavailable) {
			return []diagnostic.Diagnostic{{
				Code:     diagnostic.CodeDoctorStorageUnavailable,
				Severity: diagnostic.SeverityWarning,
				Message:  "ddos-flowspec FlowSpec record: refused, this daemon publishes no store, so it cannot say whether a FlowSpec rule the plugin announced is still at the BGP peers; the command below reads the record from the store, or names why that store cannot be opened",
				Fix:      recordReadCommand(),
			}}
		}
	}
	return storedRecordFindings(reading, report)
}

// daemonRecordReading reads the record from the store the daemon publishes,
// the one the plugin's state calls read and write (plugin/server
// dispatch_state.go).
func daemonRecordReading() recordReading {
	return classifyRecord(statestore.Read(announcementKey.Pattern))
}

// pluginRun is what the daemon's plugin snapshot says of ddos-flowspec. It is
// exactly one of four outcomes, each holding only its own payload:
// pluginRunning, pluginNotRunning, pluginNotRun and pluginRunUnknown. A failure
// to ask is its own outcome, never "not running".
type pluginRun interface{ pluginRun() }

// pluginRunning: the daemon runs the plugin, so its runtime check judges the
// record.
type pluginRunning struct{}

// pluginNotRunning: the daemon holds the plugin configured and it is not
// running. why is the sentence that says so, with the stop's cause when the
// daemon recorded one. cleanupOwed is the snapshot's CleanupOwed: a stopped
// process of the plugin has not run its cleanup, and ze refuses the restart
// until it has.
type pluginNotRunning struct {
	why         string
	cleanupOwed bool
}

// pluginNotRun: the snapshot holds no configured plugin named ddos-flowspec.
// The process name is what the snapshot keys on (PluginDoctorState.Name), and
// it carries no registry name, so a block that runs this implementation under
// another name (`plugin { internal <other> { use ddos-flowspec } }`) is
// pluginNotRun too.
type pluginNotRun struct{}

// pluginRunUnknown: the daemon has no plugin snapshot to ask yet (its plugin
// startup has not begun, or its plugin server is not published).
type pluginRunUnknown struct{ err error }

func (pluginRunning) pluginRun()    {}
func (pluginNotRunning) pluginRun() {}
func (pluginNotRun) pluginRun()     {}
func (pluginRunUnknown) pluginRun() {}

// notStartedWhy is the reason of a configured plugin that is not running and
// that no stop recorded: it is starting, or it ended and its stop is not
// recorded yet.
const notStartedWhy = "configured plugin " + Name + " is not running, and the daemon records no stop of it: it is starting, or it ended and its stop is not recorded yet"

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
			return pluginNotRunning{why: process.StoppedMessage(Name, &state.Stop), cleanupOwed: state.CleanupOwed}
		}
		return pluginNotRunning{why: notStartedWhy, cleanupOwed: state.CleanupOwed}
	}
	return pluginNotRun{}
}

// stoppedRunReport answers, for the daemon's runtime check, the report a
// recorded rule gets, or owned false when another check owns the record: the
// running plugin's own runtime check, or, while the saved configuration
// doctor read does not name ddos flowspec, the post-config check. judged is
// read only when the daemon holds no ddos-flowspec process.
func stoppedRunReport(run pluginRun, judged *config.Tree) (report orphanReport, owned bool) {
	switch r := run.(type) {
	case pluginRunning:
		return nil, false
	case pluginNotRunning:
		return stoppedOrphanReport(r), true
	case pluginNotRun:
		return unrunReport(judged)
	case pluginRunUnknown:
		return unknownRunOrphanReport(r.err), true
	}
	panic("BUG: ddos-flowspec: stoppedRunReport met a plugin run of no known kind")
}

// unrunReport answers the report of a record while the daemon holds no plugin
// process named ddos-flowspec, from the tree the post-config phase judged. The
// snapshot keys on the process name only, so that state is either a daemon
// that runs a configuration without the section or a plugin block that runs
// ddos-flowspec under another name, and no report here can tell them apart.
func unrunReport(judged *config.Tree) (report orphanReport, owned bool) {
	if judged == nil {
		return noConfigurationUnrunDiagnostic, true
	}
	if judged.GetContainerPath(configRoot) == nil {
		// refuse-by-default: guards orphanRecordCheck -- the saved configuration doctor read does not name ddos flowspec, so the post-config orphanRecordCheck reported this record from that same tree; a report here would list it twice
		return nil, false
	}
	return unrunUndecidedDiagnostic, true
}

// runningConfigCommand prints the configuration the daemon runs, from memory.
// It is the fix of the two reports that cannot tell whether a plugin process
// under another name holds the rule: the running configuration says so, and it
// changes nothing.
const runningConfigCommand = "ze show config running"

// unrunUndecidedSentence is what the unrun reports say they cannot tell.
const unrunUndecidedSentence = "doctor cannot tell which, so it offers no withdrawal"

// noConfigurationSentence is what the report says when the post-config phase
// did not run.
const noConfigurationSentence = "This doctor run loaded no saved configuration"

// unrunUndecidedDiagnostic is the report while the saved configuration doctor
// read names ddos flowspec and the daemon holds no plugin process named
// ddos-flowspec.
func unrunUndecidedDiagnostic(m flowspecMatch) rpc.DoctorCheckDiagnostic {
	var msg textbuf.Buffer
	msg.Str("ddos-flowspec holds the record of a FlowSpec rule it announced for ").Str(m.DstPrefix.String()).
		Str(". The saved configuration names ddos flowspec, and the daemon holds no plugin process named ").Str(Name).
		Str(". ")
	unrunCases(&msg, m)
	return unrunDiagnostic(&msg)
}

// noConfigurationUnrunDiagnostic is the report while the daemon holds no
// plugin process named ddos-flowspec and the post-config phase did not run, so
// doctor does not know whether the saved configuration names ddos flowspec.
func noConfigurationUnrunDiagnostic(m flowspecMatch) rpc.DoctorCheckDiagnostic {
	var msg textbuf.Buffer
	msg.Str("ddos-flowspec holds the record of a FlowSpec rule it announced for ").Str(m.DstPrefix.String()).
		Str(", and the daemon holds no plugin process named ").Str(Name).
		Str(". ").Str(noConfigurationSentence).
		Str(" (a doctor-config row above says why), so it cannot say whether ddos flowspec is configured. ")
	unrunCases(&msg, m)
	return unrunDiagnostic(&msg)
}

// unrunCases writes the two cases a daemon with no ddos-flowspec process can
// be in, and what the operator looks for and runs in each. The withdrawal is
// named only for the case where no process runs the plugin: sent while a
// process under another name announces the same rule, it ends a running
// mitigation. That process cannot read the record, because the store key is
// granted to the name ddos-flowspec alone (register.go, RegisterPluginKeys),
// so its own doctor check does not judge this record.
func unrunCases(msg *textbuf.Buffer, m flowspecMatch) {
	msg.Str("Either the daemon runs a configuration without the ddos flowspec section, or a plugin block runs ").Str(Name).
		Str(" under another name; ").Str(unrunUndecidedSentence).
		Str(", because a withdrawal sent while a plugin announces the same rule ends a running mitigation. The command below prints the configuration the daemon runs. If it holds a plugin block with use ").Str(Name).
		Str(", that process runs the plugin under another name and may announce the same rule as a live mitigation. It cannot read or withdraw this record, which only the name ").Str(Name).
		Str(" may use, so do not send the withdrawal while it mitigates traffic to ").Str(m.DstPrefix.String()).
		Str(". If it holds no ddos flowspec section, no process runs the plugin, nothing withdraws the rule, and every BGP peer that took it may still discard or rate-limit traffic to ").Str(m.DstPrefix.String()).
		Str(": withdraw it at the peers with ").Str(withdrawalCommand(m)).
		Str(". That withdrawal does not remove the record: the plugin removes it when it next starts, and ")
	recordRemovalToday(msg)
}

// unrunDiagnostic is the finding of an unrun report, whose fix reads the
// running configuration and changes nothing.
func unrunDiagnostic(msg *textbuf.Buffer) rpc.DoctorCheckDiagnostic {
	return rpc.DoctorCheckDiagnostic{
		Code:     diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement,
		Severity: string(diagnostic.SeverityWarning),
		Message:  msg.String(),
		Fix:      runningConfigCommand,
	}
}

// restartRefusedSentence is what the stopped report says while a stopped
// process of the plugin owes its cleanup: ze refuses the restart then
// (server/reload_tx.go errReplacedCleanupLate).
const restartRefusedSentence = "ze refuses that restart until the cleanup of the plugin's stopped process has finished, because that cleanup would remove what a new process registered under the name"

// stoppedOrphanReport is the report while ddos flowspec is configured and the
// plugin is not running, so neither its runtime check nor a responder answers
// for the rule. Its fix is the plugin restart: the start
// (withdrawRecordedAnnouncement, from OnStarted) withdraws the recorded rule
// and removes the record, which a withdrawal typed at the CLI does not. While
// a stopped process still owes its cleanup the restart is refused, so the
// report says so and points at the doctor-plugin-stopped row, whose producer
// (plugin/doctor pluginStoppedRow) says what that cleanup waits for and names
// the last resort.
func stoppedOrphanReport(run pluginNotRunning) orphanReport {
	return func(m flowspecMatch) rpc.DoctorCheckDiagnostic {
		victim := m.DstPrefix.String()
		var msg textbuf.Buffer
		msg.Str("ddos-flowspec holds the record of a FlowSpec rule it announced for ").Str(victim).
			Str(", and no running plugin holds that rule: ").Str(run.why).
			Str(". Nothing withdraws the rule until the plugin runs again. Every BGP peer that took the rule may still discard or rate-limit traffic to ").Str(victim).
			Str(". A restart of the plugin withdraws the recorded rule and removes the record; a second withdrawal of a rule the peers no longer hold changes nothing. If that withdrawal fails, the plugin logs the cause under the code ").
			Str(diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement).
			Str(", and the running plugin's own check reports the record")
		if run.cleanupOwed {
			msg.Str(". ").Str(restartRefusedSentence).
				Str(". Run the restart below once that cleanup has finished: while the plugin is recorded stopped, its ").
				Str(diagnostic.CodeDoctorPluginStopped).
				Str(" row says whether the cleanup is still owed, and names the last resort if it never finishes")
		}
		return rpc.DoctorCheckDiagnostic{
			Code:     diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement,
			Severity: string(diagnostic.SeverityWarning),
			Message:  msg.String(),
			Fix:      process.RestartCommand(Name),
		}
	}
}

// unknownRunOrphanReport is the report of a daemon whose plugin snapshot
// answered an error (its plugin startup has not begun, or its plugin server is
// not published): it found the record and cannot tell yet whether a running
// plugin holds the rule as its live announcement, which is the normal state of
// a running mitigation. It says both, and its fix asks the daemon again, which
// can tell once its plugins started.
func unknownRunOrphanReport(err error) orphanReport {
	return func(m flowspecMatch) rpc.DoctorCheckDiagnostic {
		victim := m.DstPrefix.String()
		var msg textbuf.Buffer
		msg.Str("ddos-flowspec holds the record of a FlowSpec rule it announced for ").Str(victim).
			Str(", and the daemon cannot tell yet whether the plugin runs and holds the rule as its live announcement (").Err(err).
			Str("). A running mitigation keeps this record, so the record alone is not a fault, and a start of the plugin withdraws the recorded rule and removes the record. Once the daemon has started its plugins, the command below asks again and can tell")
		return rpc.DoctorCheckDiagnostic{
			Code:     diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement,
			Severity: string(diagnostic.SeverityWarning),
			Message:  msg.String(),
			Fix:      daemonDoctorCommand,
		}
	}
}

// daemonDoctorCommand is the doctor run through the daemon, whose plugin
// snapshot says whether the plugin runs once its plugins started.
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
