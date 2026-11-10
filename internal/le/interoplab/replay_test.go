package interoplab

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestReplayGivesTheLabVerdict records a passing and a failing scenario
// through Suite.Run with a scripted Docker runner, then replays each
// recording through the checker the lab's catalog registers.
// VALIDATES: AC-6. A pass replays as a pass; a failure replays as a failure
// with the lab's error text, assertion number included; each replayed verdict
// equals the verdict the recording carries and the one the lab reported.
// PREVENTS: a replay that judges a checker on answers other than the ones the
// lab gave it.
func TestReplayGivesTheLabVerdict(t *testing.T) {
	recordings := t.TempDir()
	var progress bytes.Buffer
	report := recordSuite(&progress, recordings, "asserts-red", "passes").Run(t.Context())
	if report.Passed != 1 || report.Failed != 1 {
		t.Fatalf("report passed %d failed %d, want 1 and 1: %#v", report.Passed, report.Failed, report)
	}
	for _, lab := range report.Scenarios {
		t.Run(lab.Name, func(t *testing.T) {
			result, err := ReplayScenario(t.Context(), recordLabCheckout, recordingPath(recordings, lab.Name))
			if err != nil {
				t.Fatalf("replay refused: %v", err)
			}
			want := Verdict{Passed: lab.Passed, Error: lab.Error}
			if result.Lab != want {
				t.Errorf("the recording carries verdict %#v, the lab reported %#v", result.Lab, want)
			}
			if result.Replay != want {
				t.Errorf("replayed verdict %#v, the lab's %#v", result.Replay, want)
			}
		})
	}
	if !strings.Contains(report.Scenarios[0].Error, "scenario asserts-red assertion 2: ") {
		t.Errorf("the failing scenario's error %q lost its assertion number", report.Scenarios[0].Error)
	}
}

// writeRecording writes header, calls and, when verdict is not nil, the
// verdict line to a file under t.TempDir, and answers its path.
func writeRecording(t *testing.T, header recordingHeader, calls []recordingLine, verdict *Verdict) string {
	t.Helper()
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	if err := encoder.Encode(header); err != nil {
		t.Fatal(err)
	}
	for index := range calls {
		if err := encoder.Encode(&calls[index]); err != nil {
			t.Fatal(err)
		}
	}
	if verdict != nil {
		if err := encoder.Encode(recordingLine{Verdict: verdict}); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "asserts-red-run.jsonl")
	if err := os.WriteFile(path, out.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestReplayRefusesAnUnrecordedCall replays hand-written recordings of the
// record test lab's asserts-red scenario, whose checker makes one exec of
// `cat /ready` on peer ze.
// VALIDATES: AC-7. A call the recording does not hold at its position, a call
// past the recording's end, a truncated answer, a non-text answer, a
// recording of another version, a recording with no verdict, and a checker
// that stops before the recording's last call each refuse with a
// *ReplayRefusal naming the expected and the actual call and saying the
// checker must be judged by a lab run.
// PREVENTS: a replay guessing an answer, so a checker change passes on data
// the lab never produced.
func TestReplayRefusesAnUnrecordedCall(t *testing.T) {
	registerRecordLab()
	header := recordingHeader{Version: recordingVersion, Lab: recordLabSuite, Scenario: "asserts-red",
		AnswerOctetsMax: recordingAnswerOctetsMax}
	made := RecordedCall{Method: methodExec, Peer: "ze", Argv: []string{"cat", "/ready"}}
	other := RecordedCall{Method: methodExec, Peer: "ze", Argv: []string{"cat", "/other"}}
	red := &Verdict{Error: "scenario asserts-red assertion 2: wanted a route, read ready"}
	future := header
	future.Version = recordingVersion + 1
	cases := []struct {
		name    string
		path    func(t *testing.T) string
		reason  string
		mention []string
	}{
		{"unmatched call", func(t *testing.T) string {
			return writeRecording(t, header, []recordingLine{{RecordedCall: other}}, red)
		}, "does not hold at this position", []string{"expected exec ze [cat /other]", "actual exec ze [cat /ready]"}},
		{"exhausted", func(t *testing.T) string {
			return writeRecording(t, header, nil, red)
		}, "holds 0 calls", []string{"expected none", "actual exec ze [cat /ready]"}},
		{"truncated answer", func(t *testing.T) string {
			return writeRecording(t, header, []recordingLine{{RecordedCall: made, Stdout: "rea", Truncated: true}}, red)
		}, "was cut at", []string{"expected exec ze [cat /ready]", "actual exec ze [cat /ready]"}},
		{"not text", func(t *testing.T) string {
			return writeRecording(t, header, []recordingLine{{RecordedCall: made, NotText: true}}, red)
		}, "not UTF-8", []string{"expected exec ze [cat /ready]"}},
		{"another version", func(t *testing.T) string {
			return writeRecording(t, future, []recordingLine{{RecordedCall: made}}, red)
		}, "is version 2 and this replay reads version 1", nil},
		{"no verdict", func(t *testing.T) string {
			return writeRecording(t, header, []recordingLine{{RecordedCall: made}}, nil)
		}, "no verdict line", nil},
		{"calls left", func(t *testing.T) string {
			return writeRecording(t, header, []recordingLine{{RecordedCall: made, Stdout: "ready\n"}, {RecordedCall: other}}, red)
		}, "returned after 1 of 2 recorded calls", []string{"expected exec ze [cat /other]", "actual none"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ReplayScenario(t.Context(), recordLabCheckout, tc.path(t))
			if err == nil {
				t.Fatalf("replay answered %#v, want a refusal", result)
			}
			refusal, isRefusal := errors.AsType[*ReplayRefusal](err)
			if !isRefusal {
				t.Fatalf("error %T %v is not a *ReplayRefusal", err, err)
			}
			text := refusal.Error()
			for _, want := range append([]string{tc.reason, "must be judged by a lab run"}, tc.mention...) {
				if !strings.Contains(text, want) {
					t.Errorf("refusal %q lacks %q", text, want)
				}
			}
		})
	}
}

// TestReplayAnswersPeerImage records the record test lab's records-calls
// scenario through Suite.Run, whose checker reads the ze peer's consumed
// image through the recorder in front of the real Lab, then replays it, and
// replays hand-written recordings of the same checker whose peer-image line
// names another peer or carries an error.
// VALIDATES: a peer-image call is recorded with its image ID and replayed with
// that ID, so the replayed checker passes as the lab's did; a recorded
// peer-image error is handed back to the checker, whose verdict then carries
// the lab's error text; a peer-image call the recording holds for another
// peer is refused with a *ReplayRefusal naming both calls.
// PREVENTS: a lab capability a checker reaches by a type assertion on its Lab,
// which the recorder and the replay lab hide, so the checker fails at its
// first assertion in every lab run (bgp-export-reload-gobgp, 2026-11-09).
func TestReplayAnswersPeerImage(t *testing.T) {
	t.Run("recorded identity replays", func(t *testing.T) {
		recordings := t.TempDir()
		var progress bytes.Buffer
		report := recordSuite(&progress, recordings, "records-calls").Run(t.Context())
		if report.Passed != 1 {
			t.Fatalf("report %#v, want one pass\nprogress:\n%s", report, progress.String())
		}
		result, err := ReplayScenario(t.Context(), recordLabCheckout, recordingPath(recordings, "records-calls"))
		if err != nil {
			t.Fatalf("replay refused: %v", err)
		}
		if !result.Replay.Passed || result.Replay != result.Lab {
			t.Errorf("replayed verdict %#v, the lab's %#v", result.Replay, result.Lab)
		}
	})
	registerRecordLab()
	header := recordingHeader{Version: recordingVersion, Lab: recordLabSuite, Scenario: "records-calls",
		AnswerOctetsMax: recordingAnswerOctetsMax}
	before := []recordingLine{
		{Method: methodExec, Peer: "ze", Argv: []string{"cat", "/ready"}, Environment: []string{"ZE_TEST_PSK"},
			Stdout: "ready\n"},
		{Method: methodQuery, Peer: "ze", Argv: []string{"show", "bgp"}},
		{Method: methodSignal, Peer: "ze", Signal: "HUP"},
		{Method: methodLogs, Peer: "ze", Lines: 5},
	}
	t.Run("recorded error replays", func(t *testing.T) {
		const lost = "container inspect returned no sha256 image ID"
		calls := append(slices.Clone(before), recordingLine{Method: methodPeerImage, Peer: "ze",
			Error: lost, ErrorKind: errKindOther})
		path := writeRecording(t, header, calls, &Verdict{Error: lost})
		result, err := ReplayScenario(t.Context(), recordLabCheckout, path)
		if err != nil {
			t.Fatalf("replay refused: %v", err)
		}
		if result.Replay.Passed || result.Replay.Error != lost {
			t.Errorf("replayed verdict %#v, want the recorded peer-image error %q", result.Replay, lost)
		}
	})
	t.Run("another peer refuses", func(t *testing.T) {
		calls := append(slices.Clone(before), recordingLine{Method: methodPeerImage, Peer: "frr",
			Stdout: phaseLabImage})
		path := writeRecording(t, header, calls, &Verdict{Passed: true})
		result, err := ReplayScenario(t.Context(), recordLabCheckout, path)
		refusal, isRefusal := errors.AsType[*ReplayRefusal](err)
		if !isRefusal {
			t.Fatalf("replay answered %#v, error %v, want a *ReplayRefusal", result, err)
		}
		text := refusal.Error()
		for _, want := range []string{"refused call 5", "does not hold at this position",
			"expected peer-image frr []", "actual peer-image ze []", "must be judged by a lab run"} {
			if !strings.Contains(text, want) {
				t.Errorf("refusal %q lacks %q", text, want)
			}
		}
	})
}
