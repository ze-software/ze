package process

import (
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/report"
)

// raisedError answers the newest error event the bus holds for (code,
// subject), false when there is none.
func raisedError(code, subject string) (report.Issue, bool) {
	errs := report.Errors(0)
	for i := len(errs) - 1; i >= 0; i-- {
		if errs[i].Code == code && errs[i].Subject == subject {
			return errs[i], true
		}
	}
	return report.Issue{}, false
}

// TestStopWithLongDetailStillRaises proves a stop whose detail is longer than
// the bus accepts in a message is still raised, with the message fitted, so
// the plugins health row reads down. Method: record a handshake stop whose
// detail is about 4 KiB of error text, then read the bus warning and the row.
//
// PREVENTS: a plugin down with a long handshake or restart error showing in
// `ze doctor` while `show warnings` lists nothing and the plugins row reads
// healthy, because the bus refused the unfitted message.
func TestStopWithLongDetailStillRaises(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	detail := strings.Repeat("handshake refused é ", 200)
	pm := NewProcessManager([]plugin.PluginConfig{{Name: "long", Internal: true}})
	pm.RecordStopped("long", PluginStop{Cause: StopHandshakeFailed, Detail: detail})

	stop, down := pm.Stopped("long")
	if !down {
		t.Fatal("Stopped(long) = not down after RecordStopped")
	}
	whole := StoppedMessage("long", &stop)
	want := report.FitMessage(whole)
	if len(want) >= len(whole) {
		t.Fatalf("the stop message (%d octets) fits the bus whole, so this test proves nothing", len(whole))
	}
	found := false
	for _, w := range report.Warnings() {
		if w.Code != diagnostic.CodeDoctorPluginStopped || w.Subject != "long" {
			continue
		}
		found = true
		if w.Message != want {
			t.Errorf("warning message is not the fitted stop message:\n got %q\nwant %q", w.Message, want)
		}
		if !utf8.ValidString(w.Message) {
			t.Error("the fitted warning message is not valid UTF-8")
		}
	}
	if !found {
		t.Fatal("no doctor-plugin-stopped warning for a stop whose detail exceeds the bus message bound")
	}
	if row := pluginsRow(t); row.Status != health.StatusDown {
		t.Fatalf("plugins row = %s (%q), want down", row.Status, row.Reason)
	}
}

// TestCrashAndRelayEventsWithLongTextStillRaise proves the two other raises
// that carry an error's text, the crash event and the stopped stderr relay,
// still reach the bus when that text is longer than the message bound.
//
// PREVENTS: a crash or a relay stop vanishing from `ze doctor`'s recent
// events because its exit status or cause was long.
func TestCrashAndRelayEventsWithLongTextStillRaise(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	long := strings.Repeat("wait: no child processes é ", 100)
	pm := NewProcessManager([]plugin.PluginConfig{{Name: "p1", Internal: true}})
	pm.ReportCrash("p1", ExitedWith("an unreaped exit: "+long), "ignore")
	raiseRelayStopped("p1", errors.New(long))

	for _, code := range []string{diagnostic.CodeDoctorPluginCrashed, diagnostic.CodeDoctorPluginStderrRelayStopped} {
		event, ok := raisedError(code, "p1")
		if !ok {
			t.Errorf("no %s event for a cause longer than the bus message bound", code)
			continue
		}
		if !utf8.ValidString(event.Message) {
			t.Errorf("%s message is not valid UTF-8", code)
		}
	}
}

// TestExitNotRecordedIsNotAnExit proves a process with no exit recorded
// within the wait is never said to have ended: ExitStatus answers a value of
// its own, the stop message and the crash event each say the connection
// closed and the process had not ended, naming the wait bound, and the crash
// event carries no exit-status detail.
//
// PREVENTS: "its process ended with ... the process had not ended", the
// self-contradiction of a fallback status read as an exit, and a second
// copy of the wait bound in the text.
func TestExitNotRecordedIsNotAnExit(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	start := time.Now()
	exit := NewProcess(plugin.PluginConfig{Name: "p1"}).ExitStatus()
	if exit.kind != exitUnrecorded {
		t.Fatalf("ExitStatus with nothing recorded = %+v, want no exit recorded", exit)
	}
	if waited := time.Since(start); waited < exitStatusWaitMax {
		t.Errorf("ExitStatus gave up after %s, before its bound %s", waited, exitStatusWaitMax)
	}
	later := "had not ended " + exitStatusWaitMax.String() + " later"

	message := StoppedMessage("p1", &PluginStop{Cause: StopPolicyExit, Policy: "ignore", ExitStatus: exit})
	if strings.Contains(message, "ended with") {
		t.Errorf("stop message %q says the process ended", message)
	}
	if !strings.Contains(message, "its connection closed, and its process "+later) {
		t.Errorf("stop message %q does not say the process had not ended within the bound", message)
	}

	pm := NewProcessManager([]plugin.PluginConfig{{Name: "p1", Internal: true}})
	pm.ReportCrash("p1", exit, "ignore")
	crash, ok := raisedError(diagnostic.CodeDoctorPluginCrashed, "p1")
	if !ok {
		t.Fatal("no doctor-plugin-crashed event for a process with no recorded exit")
	}
	for _, banned := range []string{"ended with", "exited"} {
		if strings.Contains(crash.Message, banned) {
			t.Errorf("crash event %q says %q of a process that had not ended", crash.Message, banned)
		}
	}
	for _, want := range []string{"closed its connection", later, "ignore"} {
		if !strings.Contains(crash.Message, want) {
			t.Errorf("crash event %q does not name %q", crash.Message, want)
		}
	}
	if status, has := crash.Detail["exit-status"]; has {
		t.Errorf("crash event carries exit-status %v for a process with no recorded exit", status)
	}
}
