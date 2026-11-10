package interoplab

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The record test lab: a catalog registered under recordLabSuite whose
// scenario root, joined to recordLabCheckout, holds each plan's directory, so
// Suite.catalog names the lab and ReplayScenario finds the same checkers.
const (
	recordLabCheckout = "/record-checkout"
	recordLabRoot     = "test/record-lab"
	recordLabSuite    = "record-test-lab"
	// recordLabSecret is an environment value a checker passes. It MUST NOT
	// reach a recording.
	recordLabSecret = "s3cret-psk-value"
)

// recordLabCheckers are the record test lab's scenarios. records-calls makes
// one call of each kind the fake Docker runner answers, and judges the exec
// and the consumed image it read; passes and
// asserts-red make one exec and judge it.
var recordLabCheckers = map[string]Checker{
	"records-calls": func(ctx context.Context, check *CheckContext) error {
		result, err := check.Lab.Exec(ctx, "ze", []string{"cat", "/ready"},
			[]EnvironmentVariable{{Name: "ZE_TEST_PSK", Value: recordLabSecret}})
		if err != nil {
			return err
		}
		if !strings.Contains(result.Stdout, "ready") {
			return errors.New("scenario records-calls assertion 1: no ready")
		}
		//nolint:errcheck // the recording, not the answer, is under test
		check.Lab.Query(ctx, "ze", []string{"show", "bgp"}, nil)
		//nolint:errcheck // the recording, not the answer, is under test
		check.Lab.Signal(ctx, "ze", "HUP")
		//nolint:errcheck // the recording, not the answer, is under test
		check.Lab.Logs(ctx, "ze", 5)
		identity, err := check.Lab.PeerImage(ctx, "ze")
		if err != nil {
			return err
		}
		if identity != phaseLabImage {
			return errors.New("scenario records-calls assertion 2: consumed image " + identity + ", want " + phaseLabImage)
		}
		return nil
	},
	"passes": func(ctx context.Context, check *CheckContext) error {
		answer, err := check.Lab.Query(ctx, "ze", []string{"cat", "/ready"}, nil)
		if err != nil {
			return err
		}
		if !strings.Contains(answer, "ready") {
			return errors.New("scenario passes assertion 1: no ready")
		}
		return nil
	},
	"asserts-red": func(ctx context.Context, check *CheckContext) error {
		result, err := check.Lab.Exec(ctx, "ze", []string{"cat", "/ready"}, nil)
		if err != nil {
			return err
		}
		return errors.New("scenario asserts-red assertion 2: wanted a route, read " + strings.TrimSpace(result.Stdout))
	},
}

// registerRecordLab registers the record test lab once per test binary.
func registerRecordLab() {
	if _, held := CatalogNamed(recordLabSuite); held {
		return
	}
	RegisterCatalog(Catalog{
		Suite: recordLabSuite,
		Scenarios: func(string) ([]ScenarioSource, error) {
			sources := make([]ScenarioSource, 0, len(recordLabCheckers))
			for _, name := range slices.Sorted(maps.Keys(recordLabCheckers)) {
				sources = append(sources, recordLabPlan(name).Source)
			}
			return sources, nil
		},
		RunScenario:      func(context.Context, string, string) SuiteReport { return SuiteReport{} },
		Package:          "internal/le/interoplab/recordtest/",
		ScenarioRoot:     recordLabRoot,
		SelectorVariable: "ZE_RECORD_TEST_SCENARIO",
		Action:           "test integration/interop-record-test",
	})
}

// recordLabPlan is the record test lab's scenario name, one peer, no probe.
func recordLabPlan(name string) ScenarioPlan {
	plan := phasePlan(name, recordLabCheckers[name])
	plan.Source.Directory = recordLabCheckout + "/" + recordLabRoot + "/" + name
	return plan
}

// recordSuite runs the named scenarios of the record test lab, recording
// under recordings.
func recordSuite(progress *bytes.Buffer, recordings string, names ...string) Suite {
	registerRecordLab()
	plans := make([]ScenarioPlan, 0, len(names))
	for _, name := range names {
		plans = append(plans, recordLabPlan(name))
	}
	suite := phaseSuite(progress, plans...)
	suite.Checkout = recordLabCheckout
	suite.Recordings = recordings
	return suite
}

// recordingPath is where the fixture Docker client's run records scenario.
func recordingPath(recordings, scenario string) string {
	return filepath.Join(recordings, recordLabSuite, scenario+"-"+fixtureOwner().run+".jsonl")
}

// readRecordingLines answers every line of the recording at path, decoded
// into a generic map.
func readRecordingLines(t *testing.T, path string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // a recording the test's own suite wrote under t.TempDir
	if err != nil {
		t.Fatalf("read recording: %v", err)
	}
	var lines []map[string]any
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(nil, 8<<20)
	for scanner.Scan() {
		var line map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			t.Fatalf("recording line %q: %v", scanner.Text(), err)
		}
		lines = append(lines, line)
	}
	return lines
}

// TestRunScenarioRecordsCheckerCalls runs the record test lab through
// Suite.Run, the entry point every interop lab reaches, with a scripted
// Docker runner, and reads the recordings it leaves.
// VALIDATES: AC-5. Every CheckerLab call the checker makes is written in call
// order, after a version header and before the lab's verdict, the real Lab's
// PeerImage included, with the image ID it observed as its answer; an environment
// variable's name is written and its value is not; a selector run keeps a
// passing scenario's recording and names it on the progress stream; a full
// run removes the passing recordings, keeps the failed one, and names it in
// the failed scenario's diagnostics and in the report text.
// PREVENTS: a checker that can only be re-judged by a lab run, and a secret a
// checker passed landing in a recording copied into testdata/.
func TestRunScenarioRecordsCheckerCalls(t *testing.T) {
	t.Run("selector run keeps a pass, in call order, names only", func(t *testing.T) {
		recordings := t.TempDir()
		var progress bytes.Buffer
		report := recordSuite(&progress, recordings, "records-calls").Run(t.Context())
		if report.Passed != 1 {
			t.Fatalf("report %#v, want one pass", report)
		}
		path := recordingPath(recordings, "records-calls")
		data, err := os.ReadFile(path) //nolint:gosec // the test's own recording
		if err != nil {
			t.Fatalf("a selector run kept no recording of its passing scenario: %v\nprogress:\n%s", err, progress.String())
		}
		if bytes.Contains(data, []byte(recordLabSecret)) {
			t.Errorf("the recording carries the environment value %q:\n%s", recordLabSecret, data)
		}
		if !bytes.Contains(data, []byte(`"ZE_TEST_PSK"`)) {
			t.Errorf("the recording lacks the environment name ZE_TEST_PSK:\n%s", data)
		}
		lines := readRecordingLines(t, path)
		if len(lines) == 0 {
			t.Fatal("empty recording")
		}
		header := lines[0]
		if header["version"] != float64(recordingVersion) || header["lab"] != recordLabSuite || header["scenario"] != "records-calls" {
			t.Errorf("header %v, want version %d, lab %s, scenario records-calls", header, recordingVersion, recordLabSuite)
		}
		var methods []string
		for _, line := range lines[1 : len(lines)-1] {
			method, _ := line["method"].(string)
			methods = append(methods, method)
		}
		want := []string{"exec", "query", "signal", "logs", "peer-image"}
		if !slices.Equal(methods, want) {
			t.Errorf("recorded methods %v, want %v in call order", methods, want)
		}
		if image := lines[len(lines)-2]; image["peer"] != "ze" || image["stdout"] != phaseLabImage {
			t.Errorf("peer-image line %v, want peer ze and the consumed image %s as its answer", image, phaseLabImage)
		}
		verdict, _ := lines[len(lines)-1]["verdict"].(map[string]any)
		if verdict["passed"] != true {
			t.Errorf("last line %v, want the lab's passing verdict", lines[len(lines)-1])
		}
		if !strings.Contains(progress.String(), "interop: records-calls: checker recording: "+path+"\n") {
			t.Errorf("progress does not name the kept recording %s:\n%s", path, progress.String())
		}
	})
	t.Run("full run keeps only the failure and names it", func(t *testing.T) {
		recordings := t.TempDir()
		var progress bytes.Buffer
		report := recordSuite(&progress, recordings, "asserts-red", "passes", "records-calls").Run(t.Context())
		if report.Passed != 2 || report.Failed != 1 {
			t.Fatalf("report passed %d failed %d, want 2 and 1: %#v", report.Passed, report.Failed, report)
		}
		for _, name := range []string{"passes", "records-calls"} {
			if _, err := os.Stat(recordingPath(recordings, name)); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("a full run kept the passing recording of %s (stat error %v)", name, err)
			}
		}
		failed := recordingPath(recordings, "asserts-red")
		if _, err := os.Stat(failed); err != nil {
			t.Fatalf("a full run kept no recording of its failed scenario: %v", err)
		}
		result := report.Scenarios[0]
		if result.Name != "asserts-red" || !slices.Contains(result.Diagnostics, "checker recording: "+failed+"\n") {
			t.Errorf("failed result %#v does not name its recording %s in its diagnostics", result, failed)
		}
		if !strings.Contains(report.Text(), "interop: asserts-red: checker recording: "+failed+"\n") {
			t.Errorf("report text does not name the recording:\n%s", report.Text())
		}
	})
	t.Run("unlistable catalog keeps every recording and says why", func(t *testing.T) {
		const (
			suite = "record-test-lab-unlistable"
			root  = "test/record-lab-unlistable"
		)
		if _, held := CatalogNamed(suite); !held {
			RegisterCatalog(Catalog{
				Suite: suite,
				Scenarios: func(string) ([]ScenarioSource, error) {
					return nil, errors.New("scenario directory unreadable")
				},
				RunScenario:      func(context.Context, string, string) SuiteReport { return SuiteReport{} },
				Package:          "internal/le/interoplab/recordtestunlistable/",
				ScenarioRoot:     root,
				SelectorVariable: "ZE_RECORD_UNLISTABLE_SCENARIO",
				Action:           "test integration/interop-record-unlistable",
			})
		}
		recordings := t.TempDir()
		var progress bytes.Buffer
		plan := recordLabPlan("passes")
		plan.Source.Directory = recordLabCheckout + "/" + root + "/passes"
		run := phaseSuite(&progress, plan)
		run.Checkout = recordLabCheckout
		run.Recordings = recordings
		report := run.Run(t.Context())
		if report.Passed != 1 {
			t.Fatalf("report %#v, want one pass", report)
		}
		want := "interop: checker recordings: keeping every recording because the lab could not list its scenarios: scenario directory unreadable\n"
		if !strings.Contains(progress.String(), want) {
			t.Errorf("progress lacks %q:\n%s", want, progress.String())
		}
		kept := filepath.Join(recordings, suite, "passes-"+fixtureOwner().run+".jsonl")
		if _, err := os.Stat(kept); err != nil {
			t.Errorf("an unlistable catalog's passing recording was not kept: %v", err)
		}
	})
}

// TestRecordingAnswerCap bounds one answer field at the declared cap.
// VALIDATES: spec boundary "recording answer cap": the cap itself is kept
// whole and unmarked; cap+1 keeps the cap's octets and is marked truncated;
// an answer that is not UTF-8 is marked so replay refuses it.
// PREVENTS: a relay dump filling the session scratch, and a cut answer
// replayed as if whole.
func TestRecordingAnswerCap(t *testing.T) {
	var atCap RecordedAnswer
	kept := atCap.bound(strings.Repeat("a", recordingAnswerOctetsMax))
	if len(kept) != recordingAnswerOctetsMax || atCap.Truncated {
		t.Errorf("an answer of the cap kept %d octets, truncated %v; want %d and false", len(kept), atCap.Truncated, recordingAnswerOctetsMax)
	}
	var overCap RecordedAnswer
	kept = overCap.bound(strings.Repeat("a", recordingAnswerOctetsMax+1))
	if len(kept) != recordingAnswerOctetsMax || !overCap.Truncated {
		t.Errorf("an answer of cap+1 kept %d octets, truncated %v; want %d and true", len(kept), overCap.Truncated, recordingAnswerOctetsMax)
	}
	var binary RecordedAnswer
	binary.bound("\xff\xfe")
	if !binary.NotText {
		t.Error("an answer that is not UTF-8 is not marked not-text")
	}
}
