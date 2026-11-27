// VALIDATES: `le chaos fuzz <options>` reaches the swarm fuzzer's command line
// with the options unchanged and prints its own help page.
// PREVENTS: an le command that registers and answers without calling fuzz.CLIRun.

package chaosfuzz

import (
	"io"
	"os"
	"strings"
	"testing"

	// The chaos namespace's summary: Dispatch refuses a namespace without one.
	_ "github.com/ze-software/ze/internal/le/chaos"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// TestChaosFuzzReachesFuzzer drives `le chaos fuzz` through the le dispatcher
// to verify valid execution and invalid-flag refusal both reach fuzz.CLIRun.
func TestChaosFuzzReachesFuzzer(t *testing.T) {
	t.Cleanup(leroot.SetAdmitter(func(_ string, _ []string, body func() int) int { return body() }))

	if code := leroot.Dispatch("le", []string{"chaos", "fuzz", "--seeds", "0"}); code == 0 {
		t.Error("le chaos fuzz --seeds 0 exited 0; fuzzer refusal did not reach the caller")
	}

	code := leroot.Dispatch("le", []string{
		"chaos", "fuzz", "--seeds", "1", "--base-seed", "42",
		"--duration", "5s", "--max-peers", "2", "--max-routes", "4", "--quiet",
	})
	if code != 0 {
		t.Fatalf("le chaos fuzz --seeds 1 exited %d, want 0", code)
	}
}

// TestChaosFuzzHelpIsFuzzersHelp drives `le chaos fuzz --help` through the le
// dispatcher and asserts the fuzzer's own flag table is printed.
func TestChaosFuzzHelpIsFuzzersHelp(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stderr
	os.Stderr = writer
	code := leroot.Dispatch("le", []string{"chaos", "fuzz", "--help"})
	os.Stderr = previous
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	page, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("le chaos fuzz --help exited %d, want 0", code)
	}
	if !strings.Contains(string(page), "--check-determinism") {
		t.Errorf("le chaos fuzz --help printed no fuzzer flag list:\n%s", page)
	}
}
