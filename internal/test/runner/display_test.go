package runner

import (
	"bytes"
	"strings"
	"testing"
)

func TestDisplayDebugHintsUseSuiteSpecificCommands(t *testing.T) {
	tests := NewTests()
	rec := tests.Add("ui-failure")
	rec.Active = true
	rec.State = StateFail

	var buf bytes.Buffer
	display := NewDisplay(tests, NewColorsWithOverride(false))
	display.SetOutput(&buf)
	display.SetLabel("ui")
	display.debugHints()

	out := buf.String()
	if !strings.Contains(out, "./le test ui "+rec.Nick) {
		t.Fatalf("missing top-level rerun command:\n%s", out)
	}
	if strings.Contains(out, "./le test bgp ui") {
		t.Fatalf("debug hint used wrong BGP command:\n%s", out)
	}
}

func TestDisplayDebugHintsUseEditorPatternCommand(t *testing.T) {
	tests := NewTests()
	rec := tests.Add("test/editor/commands/show-full.et")
	rec.Active = true
	rec.State = StateFail

	var buf bytes.Buffer
	display := NewDisplay(tests, NewColorsWithOverride(false))
	display.SetOutput(&buf)
	display.SetLabel("editor")
	display.debugHints()

	out := buf.String()
	if !strings.Contains(out, "./le test editor "+rec.Nick) {
		t.Fatalf("missing editor rerun command:\n%s", out)
	}
}

func TestDisplayTestFinishedIncludesOrdinalNickAndName(t *testing.T) {
	ResetNickCounter()
	tests := NewTests()
	first := tests.Add("alpha")
	second := tests.Add("beta")
	first.Active = true
	second.Active = true

	var buf bytes.Buffer
	display := NewDisplay(tests, NewColorsWithOverride(false))
	display.SetOutput(&buf)
	display.SetParallel(1, tests.Count())
	display.Start()

	second.State = StateSuccess
	display.TestFinished(second.Nick, second.State, 0)

	out := buf.String()
	for _, want := range []string{"2/2", "PASS", second.Nick, second.Name} {
		if !strings.Contains(out, want) {
			t.Fatalf("completion line missing %q:\n%s", want, out)
		}
	}
}

// TestDisplayTestFinishedNamesSkipReason pins that a SKIP line says why.
//
// VALIDATES: a skipped test's completion line carries its SkipReason, so an
// option=needs-arch skip names the arch the test needs and the host's arch.
// PREVENTS: a skip that reads like an unexplained gap, indistinguishable from a
// test nobody can run.
func TestDisplayTestFinishedNamesSkipReason(t *testing.T) {
	ResetNickCounter()
	tests := NewTests()
	rec := tests.Add("arm64-only")
	rec.Active = true
	rec.SkipReason = "needs-arch=arm64, host GOARCH=amd64: a target arch is tested only on a CPU of that arch"

	var buf bytes.Buffer
	display := NewDisplay(tests, NewColorsWithOverride(false))
	display.SetOutput(&buf)
	display.SetParallel(1, tests.Count())
	display.Start()

	rec.State = StateSkip
	display.TestFinished(rec.Nick, rec.State, 0)

	out := buf.String()
	for _, want := range []string{"SKIP", rec.Name, "-- " + rec.SkipReason} {
		if !strings.Contains(out, want) {
			t.Fatalf("skip line missing %q:\n%s", want, out)
		}
	}
}
