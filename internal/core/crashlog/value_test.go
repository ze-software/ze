// Design: docs/architecture/diagnostics/crash-capture.md -- the panic value in every report
// Related: crashlog.go -- appendValue under test

package crashlog

import (
	"errors"
	"strings"
	"testing"
)

// panickyStringer is a panic value whose String method itself panics, the way a
// value with a nil field dereferenced in String does.
type panickyStringer struct{}

func (panickyStringer) String() string { panic("String exploded") }

// panickyError is an error whose Error method itself panics.
type panickyError struct{}

func (panickyError) Error() string { panic("Error exploded") }

// TestBuildCrashReportPrintsEveryValue proves a fatal report carries the panic
// value whatever its type, and that a value whose String or Error method panics
// is reported rather than panicking the crash path.
// METHOD: build a report for each value and look for its printed form after
// the panic heading.
// PREVENTS: a panic with an integer, a struct or a runtime error reaching the
// operator as "<unknown panic value>", and a defective Stringer turning a crash
// report into a second panic.
func TestBuildCrashReportPrintsEveryValue(t *testing.T) {
	type detail struct {
		Peer string
		Code int
	}
	cases := []struct {
		name  string
		value any
		want  string
	}{
		{"string", "boom", "boom"},
		{"error", errors.New("index out of range"), "index out of range"},
		{"integer", 42, "42"},
		{"struct", detail{Peer: "192.0.2.1", Code: 7}, "{192.0.2.1 7}"},
		{"panicking String", panickyStringer{}, "%!v(PANIC=String method: String exploded)"},
		{"panicking Error", panickyError{}, "%!v(PANIC=Error method: Error exploded)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := buildCrashReport(tc.value, nil)
			_, panicSection, found := strings.Cut(report, "=== Panic ===\n")
			if !found {
				t.Fatalf("report has no panic section:\n%s", report)
			}
			if !strings.HasPrefix(panicSection, tc.want) {
				t.Errorf("panic section starts %q, want %q", firstLine(panicSection), tc.want)
			}
			if strings.Contains(report, "<unknown panic value>") {
				t.Errorf("report still carries the placeholder:\n%s", report)
			}
		})
	}
}

// firstLine returns s up to its first newline.
func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
