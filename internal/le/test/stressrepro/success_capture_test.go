// Design: docs/functional-tests.md -- retained evidence of named stress executions.
// Related: run.go -- successful child capture must retain its verdict under warnings.

package teststressrepro

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

const successCaptureVerdict = "1.4s  1/1  PASS  1  bgp-blackhole-honor\nVERIFY STEP bgp-blackhole-honor\npass 1/1 100.0%\n"

// Real successful children write their verdict on stdout and discovery warnings
// on stderr. The process adapter concatenates those streams; a short tail can
// therefore retain only unrelated warnings and discard all execution evidence.
func TestSuccessfulCaptureRetainsNamedVerdicts(t *testing.T) {
	warnings := strings.Repeat("warning: another draft was not discovered\n", 100)
	output := successCaptureVerdict + warnings
	capture := successCaptureRun(t, output)
	if !strings.Contains(capture, successCaptureVerdict) {
		t.Fatal("successful capture discarded its named PASS and VERIFY verdict behind warning output")
	}
	if !strings.Contains(capture, warnings) {
		t.Fatal("successful capture hid the warning output")
	}
}

// Keeping useful success evidence must not turn every successful invocation
// into an unbounded transcript. The useful verdict here remains near the end
// of an oversized output and must survive while its obsolete prefix is cut.
func TestSuccessfulCaptureRemainsBounded(t *testing.T) {
	output := "oversized-success-prefix\n" + strings.Repeat("x", 96<<10) + "\n" +
		successCaptureVerdict + strings.Repeat("warning: unrelated discovery\n", 100)
	capture := successCaptureRun(t, output)
	if !strings.Contains(capture, successCaptureVerdict) {
		t.Fatal("bounded capture discarded the named verdict near its tail")
	}
	if strings.Contains(capture, "oversized-success-prefix") {
		t.Fatal("successful capture retained its unbounded prefix")
	}
	marker := "===== invocation 1 exit=0 ok =====\n"
	_, tail, found := strings.Cut(capture, marker)
	if !found {
		t.Fatal("capture omitted the actual successful invocation marker")
	}
	// The existing tail helper counts runes. Bound its payload independently
	// of the selection/header, allowing only the capture writer's final newline.
	if len([]rune(tail)) > (64<<10)+1 {
		t.Fatalf("successful capture payload has %d runes, want at most 65537", len([]rune(tail)))
	}
}

func successCaptureRun(t *testing.T, output string) string {
	t.Helper()
	root := stressTree(t)
	fake := &fakeRunner{results: []processResult{{output: output}}}
	deps, stops := testDependencies(fake)
	opts := baseOptions()
	opts.Suite = "plugin --draft"
	opts.Test = "bgp-blackhole-honor"
	opts.AnyFailure = true
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	report, code := run(ctx, root, opts, deps)
	if code != 1 {
		t.Fatalf("successful no-reproduction run returned %d: %+v", code, report)
	}
	if report.Reproduced {
		t.Fatal("successful child became a reproduction")
	}
	if report.SetupError != "" {
		t.Fatalf("successful child became a setup failure: %s", report.SetupError)
	}
	if report.Completed != 1 {
		t.Fatalf("completed %d invocations, want one", report.Completed)
	}
	if report.Failed != 0 {
		t.Fatalf("successful child recorded %d failures", report.Failed)
	}
	if fake.count() != 1 {
		t.Fatalf("fake child ran %d times, want one", fake.count())
	}
	if *stops != 1 {
		t.Fatalf("burner owner stopped %d times, want one", *stops)
	}
	capture, err := os.ReadFile(report.Log)
	if err != nil {
		t.Fatal(err)
	}
	return string(capture)
}
