package trace

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestPrintTraceHumanPass(t *testing.T) {
	var buf bytes.Buffer
	steps := []StepResult{
		{Step: 1, Line: 5, Kind: "action", Assert: "open", Passed: true},
		{Step: 2, Line: 8, Kind: "expect", Assert: "element", Passed: true},
	}
	PrintTrace(&buf, "test.wb", steps, false)
	out := buf.String()

	if !strings.Contains(out, "✓") {
		t.Error("expected pass glyph in output")
	}
	if strings.Contains(out, "✗") {
		t.Error("unexpected fail glyph in output")
	}
	if !strings.Contains(out, "action open") {
		t.Error("expected 'action open' in output")
	}
	if !strings.Contains(out, "expect element") {
		t.Error("expected 'expect element' in output")
	}
}

func TestPrintTraceHumanFail(t *testing.T) {
	var buf bytes.Buffer
	steps := []StepResult{
		{Step: 1, Line: 3, Kind: "action", Assert: "click", Passed: true},
		{Step: 2, Line: 7, Kind: "expect", Assert: "url", Passed: false, Detail: "not found"},
	}
	PrintTrace(&buf, "test.wb", steps, false)
	out := buf.String()

	if !strings.Contains(out, "✗") {
		t.Error("expected fail glyph in output")
	}
	if !strings.Contains(out, "-> not found") {
		t.Error("expected detail in output")
	}
}

func TestPrintTraceMachineFormat(t *testing.T) {
	var buf bytes.Buffer
	steps := []StepResult{
		{Step: 1, Line: 5, Kind: "expect", Assert: "element", Passed: false, Detail: "missing"},
	}
	PrintTrace(&buf, "scenario.wb", steps, false)
	out := buf.String()

	// Find the VERIFY STEP line
	for line := range strings.SplitSeq(out, "\n") {
		if !strings.HasPrefix(line, "VERIFY STEP: ") {
			continue
		}
		jsonStr := strings.TrimPrefix(line, "VERIFY STEP: ")
		var m map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
			t.Fatalf("invalid JSON in VERIFY STEP: %v", err)
		}
		if m["file"] != "scenario.wb" {
			t.Errorf("file = %v, want scenario.wb", m["file"])
		}
		if m["status"] != "fail" {
			t.Errorf("status = %v, want fail", m["status"])
		}
		if m["detail"] != "missing" {
			t.Errorf("detail = %v, want missing", m["detail"])
		}
		return
	}
	t.Error("no VERIFY STEP line found")
}

func TestPrintTraceLineNumber(t *testing.T) {
	var buf bytes.Buffer
	steps := []StepResult{
		{Step: 1, Line: 42, Kind: "action", Assert: "open", Passed: true},
	}
	PrintTrace(&buf, "test.wb", steps, false)
	out := buf.String()

	if !strings.Contains(out, " 42 ") {
		t.Errorf("expected line number 42 in output, got: %s", out)
	}
}

func TestPrintTraceStepFallback(t *testing.T) {
	var buf bytes.Buffer
	steps := []StepResult{
		{Step: 3, Kind: "expect", Assert: "status", Passed: true},
	}
	PrintTrace(&buf, "test.et", steps, false)
	out := buf.String()

	if !strings.Contains(out, "  3 ") {
		t.Errorf("expected step number 3 in output, got: %s", out)
	}
}
