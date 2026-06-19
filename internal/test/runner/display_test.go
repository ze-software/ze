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
	if !strings.Contains(out, "ze-test ui "+rec.Nick) {
		t.Fatalf("missing top-level rerun command:\n%s", out)
	}
	if strings.Contains(out, "ze-test bgp ui") {
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
	if !strings.Contains(out, "ze-test editor "+rec.Nick) {
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
