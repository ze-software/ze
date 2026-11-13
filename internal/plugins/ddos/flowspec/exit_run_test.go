// Design: docs/guide/ddos-mitigation.md -- what each end of the plugin does to a live FlowSpec rule
// Related: register.go (runEngine, the deferred exit, stopResponder), exit_test.go (the engine-pair harness)

package flowspec

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRunEngineFailedRunWarnsOnceWithTheRestart proves that when the plugin's
// Run fails while a rule is live and the engine can no longer be reached,
// runEngine's deferred exit logs exactly one WARN carrying the doctor code and
// the plugin restart.
//
// VALIDATES: runEngine derives the failed end from Run's own answer: the end
// stays exitFailed unless Run returned nil.
// METHOD: runEngine over the real sdk.Plugin pair, one announcement through the
// bus, then a doctor-check callback whose state read the engine stub answers
// by closing its end of the connection. The plugin's answer to the callback
// then fails to write, the SDK's event loop returns that send error, and Run
// fails (runEngine returns 1). The deferred withdrawal meets the closed
// connection. No product seam is involved: the route is the SDK's own.
// PREVENTS: a level read from a literal, or from a flag cleared before Run's
// error is checked, so a plugin that died alone logs at INFO, which Ze's
// default WARN level hides.
//
// MUTATION: with `end := exitFailed` in runEngine written `end := exitStopped`,
// the line is INFO and the WARN count is 0.
func TestRunEngineFailedRunWarnsOnceWithTheRestart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	logs := captureExitLog(t)
	e, bus := startEngineStub(t, nil)
	announceVictim(t, e, bus)
	logs.reset()

	e.hangUpOnNext(rpc.MethodStateGet)
	go func() {
		// The answer never arrives: the stub hangs up inside the callback.
		_, _ = e.mux.CallRPC(ctx, "ze-plugin-callback:doctor-check", rpc.DoctorCheckInput{Name: orphanRuntimeCheck})
	}()
	select {
	case <-e.done:
	case <-ctx.Done():
		t.Fatalf("runEngine did not return after the engine hung up inside a callback: %v", ctx.Err())
	}
	if e.exitCode != 1 {
		t.Fatalf("runEngine returned %d after its Run failed, want 1", e.exitCode)
	}

	warns := logs.atLevel(t, "WARN")
	if len(warns) != 1 {
		t.Fatalf("WARN lines after a failed run with a live rule = %d, want 1 (log: %+v)", len(warns), logs.lines(t))
	}
	line := warns[0]
	if !strings.Contains(line.Msg, closedTransportFact) {
		t.Errorf("WARN %q does not say %q", line.Msg, closedTransportFact)
	}
	if line.Code != orphanTestCode {
		t.Errorf("WARN code = %q, want %q", line.Code, orphanTestCode)
	}
	if line.Fix != restartTestFix {
		t.Errorf("WARN fix = %q, want %q", line.Fix, restartTestFix)
	}
	if got := countCommands(e.routeCommands(), "/flow del "); got != 0 {
		t.Errorf("withdrawals that reached the engine after it hung up = %d, want 0", got)
	}
	if !e.store.holdsAnnouncementRecord() {
		t.Errorf("the record %s was removed although no withdrawal reached the engine", announcementRecordKey)
	}
}

// TestRunEngineStoppedByTheEngineLogsInfo proves that when the engine closes
// the plugin's connection, which is what a daemon stop looks like to a forked
// plugin, runEngine's deferred exit logs the closed-transport line at INFO and
// no WARN.
//
// VALIDATES: a Run that the engine ended returns nil (the SDK reads a closed
// connection as a clean shutdown), runEngine then records exitStopped, and the
// withdrawal's error is read as the closed transport it is: a forked plugin's
// call after the engine closed its end answers the mux reader's terminal
// error, "mux conn read error: EOF".
// METHOD: runEngine over the real sdk.Plugin pair, one announcement through the
// bus, then the stub's stop, which closes the engine's end without a `bye`.
// PREVENTS: a WARN at every daemon stop with a live mitigation, which says the
// rule "could not be withdrawn" and sends the operator to ze doctor for a rule
// the next start withdraws from its record.
//
// MUTATION: with `end = exitStopped` after Run in runEngine removed, the line is
// the WARN closed-transport line and the INFO count is 0.
func TestRunEngineStoppedByTheEngineLogsInfo(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	logs := captureExitLog(t)
	e, bus := startEngineStub(t, nil)
	announceVictim(t, e, bus)
	logs.reset()

	e.stop(ctx)
	if e.exitCode != 0 {
		t.Fatalf("runEngine returned %d after the engine closed its connection, want 0", e.exitCode)
	}

	if warns := logs.atLevel(t, "WARN"); len(warns) != 0 {
		t.Errorf("WARN lines after the engine stopped the plugin = %+v, want none", warns)
	}
	var closed int
	for _, info := range logs.atLevel(t, "INFO") {
		if !strings.Contains(info.Msg, closedTransportFact) {
			continue
		}
		closed++
		if info.Fix != restartTestFix {
			t.Errorf("INFO fix = %q, want %q", info.Fix, restartTestFix)
		}
	}
	if closed != 1 {
		t.Errorf("INFO lines saying %q = %d, want 1 (log: %+v)", closedTransportFact, closed, logs.lines(t))
	}
	if !e.store.holdsAnnouncementRecord() {
		t.Errorf("the record %s was removed although no withdrawal reached the engine", announcementRecordKey)
	}
}

// TestByeWithARefusedWithdrawalWarnsWithTheManualWithdrawal proves that a
// `bye` whose withdrawal does not reach the peers logs one WARN that carries
// the withdrawal by hand and promises no start of the plugin.
//
// VALIDATES: the engine sends `bye` only when it removes the plugin
// (notifyPluginRemoval, reason "removed"), so no start follows: with the
// section gone, the post-config check (orphanRecordCheck) reports the record,
// and the fix the operator needs is the withdrawal itself.
// METHOD: runEngine over the real sdk.Plugin pair, one announcement through the
// bus, then every update-route refused and the engine's `bye`.
// PREVENTS: a line that tells the operator the next start withdraws the rule
// and that the plugin restart under fix starts it, for a plugin that is no
// longer configured.
//
// MUTATION: with the OnBye handler calling stopResponder(resp, exitStopped),
// the WARN carries the plugin restart under fix.
func TestByeWithARefusedWithdrawalWarnsWithTheManualWithdrawal(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	logs := captureExitLog(t)
	e, bus := startEngineStub(t, nil)
	announceVictim(t, e, bus)
	logs.reset()

	e.refuseRoutes("send refused: no peer this selector names attaches this process with the required send permission")
	e.bye(ctx)
	e.waitExit(ctx)

	warns := logs.atLevel(t, "WARN")
	if len(warns) != 1 {
		t.Fatalf("WARN lines after a bye whose withdrawal was refused = %d, want 1 (log: %+v)", len(warns), logs.lines(t))
	}
	line := warns[0]
	if line.Fix != orphanTestFix {
		t.Errorf("WARN fix = %q, want the withdrawal by hand %q", line.Fix, orphanTestFix)
	}
	if line.Code != orphanTestCode {
		t.Errorf("WARN code = %q, want %q", line.Code, orphanTestCode)
	}
	if strings.Contains(line.Msg, "next start") {
		t.Errorf("WARN %q promises a start of a plugin the engine removed", line.Msg)
	}
	if !strings.HasPrefix(line.Msg, "ddos-flowspec: the FlowSpec rule could not be withdrawn on the way out") {
		t.Errorf("WARN %q does not open with the phrase test/reload/ddos-flowspec-parent-removed-withdraws.ci rejects", line.Msg)
	}
	if !e.store.holdsAnnouncementRecord() {
		t.Errorf("the record %s was removed although the withdrawal was refused", announcementRecordKey)
	}
}
