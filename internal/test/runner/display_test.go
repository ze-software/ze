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
	display.DebugHints()

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
	display.DebugHints()

	out := buf.String()
	if !strings.Contains(out, "ze-test editor -p test/editor/commands/show-full.et") {
		t.Fatalf("missing editor rerun command:\n%s", out)
	}
}
