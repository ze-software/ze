package interoplab

import (
	"strings"
	"testing"
)

// TestCommandErrorCarriesStdoutWhenStderrIsEmpty proves a failed command whose
// tool printed its error on stdout (GoBGP's printError) names that text, and
// that stderr, when present, stays the only output the error carries.
// VALIDATES: an exit-1 probe names its cause. PREVENTS: "exit 1" with no reason.
func TestCommandErrorCarriesStdoutWhenStderrIsEmpty(t *testing.T) {
	quiet := ProcessResult{Stdout: "  rpc error: family not configured\n", ExitCode: 1}
	text := (&commandError{Operation: "docker exec gobgp", ExitCode: 1, Stdout: failureStdout(quiet)}).Error()
	if !strings.Contains(text, "stdout: rpc error: family not configured") {
		t.Fatalf("error = %q, want the stdout text", text)
	}

	loud := ProcessResult{Stdout: "partial table", Stderr: "real cause", ExitCode: 1}
	if got := failureStdout(loud); got != "" {
		t.Fatalf("failureStdout with stderr = %q, want empty", got)
	}
	if got := failureStdout(ProcessResult{Stdout: " \n", ExitCode: 1}); got != "" {
		t.Fatalf("failureStdout of blank stdout = %q, want empty", got)
	}
}
