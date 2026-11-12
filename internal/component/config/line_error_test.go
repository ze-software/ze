package config

import (
	"strings"
	"testing"
)

// TestSplitLineNamesTheLineOnce proves a set-format parse refusal reaches
// `ze config validate` with "parse config" once, from the loader, and the
// line apart from the message, so the printer's "line N: " is the only one.
// Method: an unknown keyword on line 1 of a set-format file, through
// ParseTreeForValidation, the parse runValidation and LoadConfig share; the
// rendered error and SplitLine's answer are both compared in full.
func TestSplitLineNamesTheLineOnce(t *testing.T) {
	_, err := ParseTreeForValidation("set zzqqxx 1\n")
	if err == nil {
		t.Fatal("an unknown top-level keyword parsed")
	}
	full := err.Error()
	if !strings.HasPrefix(full, "parse config: config contains fields unknown to this build\n") {
		t.Fatalf("refusal = %q, want it to open with parse config: config contains fields unknown to this build", full)
	}
	if n := strings.Count(full, "parse config"); n != 1 {
		t.Errorf("refusal carries parse config %d times: %q", n, full)
	}
	if n := strings.Count(full, "line 1: "); n != 1 {
		t.Errorf("refusal names line 1 %d times: %q", n, full)
	}
	if n := strings.Count(full, "unknown field: zzqqxx"); n != 1 {
		t.Errorf("refusal names the field %d times: %q", n, full)
	}
	if !strings.Contains(full, "\nfix: ") {
		t.Errorf("refusal carries no fix line: %q", full)
	}
	line, message := SplitLine(err)
	if line != 1 {
		t.Errorf("SplitLine line = %d, want 1", line)
	}
	if want := strings.Replace(full, "line 1: ", "", 1); message != want {
		t.Errorf("SplitLine message = %q, want %q", message, want)
	}
	if strings.Contains(message, "line ") {
		t.Errorf("message still names a line: %q", message)
	}
}

// TestSplitLineNamesEachUnknownFieldOnce proves a set-format config with an
// unknown field on two lines names each field once, with its own line: the
// strict refusal's line apart (SplitLine), the other field's line in the
// message.
// Method: unknown keywords on lines 1 and 2, through ParseTreeForValidation;
// the rendered error and SplitLine's answer are counted per line and field.
func TestSplitLineNamesEachUnknownFieldOnce(t *testing.T) {
	_, err := ParseTreeForValidation("set zzqqxx 1\nset yyqqxx 2\n")
	if err == nil {
		t.Fatal("two unknown top-level keywords parsed")
	}
	full := err.Error()
	for _, want := range []string{"line 1: ", "line 2: ", "unknown field: zzqqxx", "unknown field: yyqqxx"} {
		if n := strings.Count(full, want); n != 1 {
			t.Errorf("refusal carries %q %d times: %q", want, n, full)
		}
	}
	line, message := SplitLine(err)
	if line != 1 {
		t.Errorf("SplitLine line = %d, want 1", line)
	}
	if strings.Contains(message, "line 1: ") {
		t.Errorf("message names the printed line again: %q", message)
	}
	if n := strings.Count(message, "line 2: unknown field: yyqqxx"); n != 1 {
		t.Errorf("message names line 2's field %d times: %q", n, message)
	}
}

// TestSplitLineKeepsAnUnlinedError proves the loader's wrap keeps its segment
// and hint once around a line error, the hint before the refusal so a
// refusal's fix: line stays the last line (AC-10), and an error no parser
// layer built is handed back whole with line 0, so nothing is cut from it.
func TestSplitLineKeepsAnUnlinedError(t *testing.T) {
	err := lineErrorf(4, "x")
	wrapped := &parseConfigError{cause: err, hint: "hint"}
	if line, message := SplitLine(wrapped); line != 4 || message != "parse config: hint\n\nx" {
		t.Errorf("SplitLine(parse config wrap) = %d, %q", line, message)
	}
	if got := wrapped.Error(); got != "parse config: hint\n\nline 4: x" {
		t.Errorf("Error() = %q", got)
	}
	if line, message := SplitLine(errPluginAutoNotYetImplemented); line != 0 || message != errPluginAutoNotYetImplemented.Error() {
		t.Errorf("SplitLine(plain) = %d, %q", line, message)
	}
}
