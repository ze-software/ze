package server

import (
	"context"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// serverEventOf answers the error event the plugin server raised under code,
// or nil when the bus holds none.
func serverEventOf(code string) *report.Issue {
	for _, issue := range report.Errors(0) {
		if issue.Source == serverEventSource && issue.Code == code {
			return &issue
		}
	}
	return nil
}

// TestNotProblemSitesRaiseEvents proves that a fault whose only record was a
// log line now also reaches the report bus under the code the line carries,
// so the recent-events doctor check reports it. Method: one representative
// site per group, driven directly, then the bus is read.
//
// VALIDATES: AC-36 (internal defect), AC-37 (request refused), AC-38 (answer
// undelivered), AC-39 (drain incomplete) of spec-doctor-coverage-12-plugin-infra.
// PREVENTS: a code on a log line that `ze doctor` never sees (R-7).
func TestNotProblemSitesRaiseEvents(t *testing.T) {
	t.Run("internal-defect", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)
		// Red: the raiseServerEvent call in registerStreaming's empty-prefix branch.
		registerStreaming(" ", OneShotSample, "", nil)
		event := serverEventOf(diagnostic.CodeDoctorInternalDefect)
		if event == nil {
			t.Fatalf("no internal-defect event on the bus: %v", report.Errors(0))
		}
		if event.Fix != fixShowErrors {
			t.Errorf("fix %q, want %q", event.Fix, fixShowErrors)
		}
	})

	t.Run("request-refused", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)
		// Red: the raiseServerEvent call after "forward-cached: dropping
		// malformed destination" in parseForwardDestinations.
		got := parseForwardDestinations("bgp-rs", []string{"10.0.0.1", "not-an-address"})
		if len(got) != 1 {
			t.Fatalf("parsed %v, want the one valid destination", got)
		}
		event := serverEventOf(diagnostic.CodeDoctorPluginRequestRefused)
		if event == nil {
			t.Fatalf("no request-refused event on the bus: %v", report.Errors(0))
		}
		if event.Subject != "bgp-rs" {
			t.Errorf("subject %q, want the plugin bgp-rs", event.Subject)
		}
	})

	t.Run("answer-undelivered", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)
		// Nobody reads the unbuffered channel, so the delivery times out.
		req := &PendingRequest{Serial: "a", Command: "show test", Timeout: time.Millisecond,
			RespChan: make(chan *plugin.Response)}
		// Red: the raiseServerEvent call in deliver's timeout branch.
		if err := deliver(req, &plugin.Response{Status: plugin.StatusDone}); err == nil {
			t.Fatal("deliver to a caller that is not reading answered no error")
		}
		event := serverEventOf(diagnostic.CodeDoctorPluginAnswerUndelivered)
		if event == nil {
			t.Fatalf("no answer-undelivered event on the bus: %v", report.Errors(0))
		}
		if event.Subject != "show test" {
			t.Errorf("subject %q, want the command", event.Subject)
		}
	})

	t.Run("drain-incomplete", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)
		gone, stopGone := startTestCaller(t, "gone", time.Second, holdCall(0))
		stopGone()
		round := &DrainRound{callers: []*drainCaller{gone}}
		deadline := time.Now().Add(2 * time.Second)
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()
		// Red: the raiseServerEvent call after the first "drain failed" line in Run.
		round.Run(ctx, stopInput(deadline))
		event := serverEventOf(diagnostic.CodeDoctorPluginDrainIncomplete)
		if event == nil {
			t.Fatalf("no drain-incomplete event on the bus: %v", report.Errors(0))
		}
		if event.Subject != "gone" {
			t.Errorf("subject %q, want the plugin gone", event.Subject)
		}
	})
}

// TestInternalDefectCodeMatchesDiagnostic proves the report bus's copy of the
// internal-defect code, which the packages diagnostic imports raise, is the
// spelling diagnostic declares. Method: compare the two constants.
//
// VALIDATES: AC-36, AC-42 (the plugin package's and the event bus's sites).
// PREVENTS: a raise under a code no check declares.
func TestInternalDefectCodeMatchesDiagnostic(t *testing.T) {
	if got, want := report.CodeInternalDefect, diagnostic.CodeDoctorInternalDefect; got != want {
		t.Fatalf("report spells %q, diagnostic declares %q", got, want)
	}
}
