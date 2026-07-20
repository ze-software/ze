// VALIDATES: editor commands with "-" — set/deactivate emit the modified config to
// stdout (pipeline stage, AC-10); edit/rollback/history reject "-" with a clear,
// stdin-SPECIFIC error (user decision 2026-07-17, AC-11 deviation, AC-12); a real
// path is unaffected (AC-13).
// PREVENTS: a stdin-sourced edit silently writing a file; a pipeline stage
// producing no output; and (critically) a reject test that would pass on ANY
// non-zero exit — the rejection message is asserted so deleting the guard fails
// the test rather than falling through to an unrelated error.
package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/core/cliio"
)

// validPipelineConfig is a fully valid config (peers carry a local ip) so the
// output of a `set -` stage passes `validate -`.
const validPipelineConfig = `bgp {
	router-id 1.2.3.4;
	session {
		asn {
			local 65000;
		}
	}
	peer edge1 {
		connection {
			local {
				ip 2.2.2.2;
			}
			remote {
				ip 1.1.1.1;
			}
		}
		session {
			asn {
				remote 65001;
			}
		}
	}
}
`

// captureStderr redirects os.Stderr for the duration of fn and returns fn's exit
// code plus everything written to stderr. Used to assert WHY a command rejected
// "-", not merely that it exited non-zero. NOTE: fn's stderr is fully buffered in
// the pipe before ReadAll, so fn must write less than the OS pipe buffer (~64 KB);
// the reject messages here are ~100 bytes, well within that. Only use this for
// commands with small, bounded stderr.
func captureStderr(t *testing.T, fn func() int) (int, string) {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	rc := fn()
	if closeErr := w.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	os.Stderr = old
	data, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if closeErr := r.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	return rc, string(data)
}

// TestSetStdoutSink: `ze config set - <path> <val>` reads stdin, applies the set,
// and emits the modified config to stdout with no file written (AC-10).
func TestSetStdoutSink(t *testing.T) {
	var out bytes.Buffer
	restore := cliio.SwapStreams(strings.NewReader(showTestConfig), &out)
	defer restore()

	rc := cmdSetImpl(storage.NewFilesystem(), []string{"-", "bgp", "session", "asn", "local", "65123"})
	if rc != exitOK {
		t.Fatalf("set - exit = %d, want %d", rc, exitOK)
	}
	if !strings.Contains(out.String(), "65123") {
		t.Fatalf("stdout missing the set value 65123:\n%s", out.String())
	}
}

// TestSetReloadFlagAcceptedStdin verifies `--reload` is a recognized flag and,
// on a stdin ("-") config, still emits the modified config without contacting a
// daemon (the stdin path has no on-disk file to reload, so the notify gate is
// skipped regardless of --reload). This exercises the new flag surface without
// any daemon side effect.
func TestSetReloadFlagAcceptedStdin(t *testing.T) {
	var out bytes.Buffer
	restore := cliio.SwapStreams(strings.NewReader(showTestConfig), &out)
	defer restore()

	rc := cmdSetImpl(storage.NewFilesystem(), []string{"--reload", "-", "bgp", "session", "asn", "local", "65123"})
	if rc != exitOK {
		t.Fatalf("set --reload - exit = %d, want %d", rc, exitOK)
	}
	if !strings.Contains(out.String(), "65123") {
		t.Fatalf("stdout missing the set value 65123:\n%s", out.String())
	}
}

// TestConfigPipelineStdin proves the pipeline story: `set -` emits a config that
// is itself a valid pipeline stage, so feeding its stdout into `validate -`
// succeeds (AC-10, End-to-End Story 3).
func TestConfigPipelineStdin(t *testing.T) {
	var setOut bytes.Buffer
	restore := cliio.SwapStreams(strings.NewReader(validPipelineConfig), &setOut)
	rc := cmdSetImpl(storage.NewFilesystem(), []string{"-", "bgp", "session", "asn", "local", "65123"})
	restore()
	if rc != exitOK {
		t.Fatalf("set - exit = %d, want %d", rc, exitOK)
	}

	var valOut bytes.Buffer
	restore2 := cliio.SwapStreams(strings.NewReader(setOut.String()), &valOut)
	defer restore2()
	if rc := cmdValidate([]string{"-"}); rc != exitOK {
		t.Fatalf("validate of set output exit = %d, want %d", rc, exitOK)
	}
}

// TestMigrateOutputDash: `ze config migrate -o -` writes the migrated config to
// stdout and creates no file (AC-8, write side).
func TestMigrateOutputDash(t *testing.T) {
	in := writeTestConfig(t, showTestConfig)
	var out bytes.Buffer
	restore := cliio.SwapStreams(strings.NewReader(""), &out)
	defer restore()

	// When an output path is given, the config is written there (stdout for "-")
	// and NOT also returned, so the caller does not double-print.
	if _, _, _, err := configMigrateWithWarnings(in, "-", ""); err != nil {
		t.Fatalf("configMigrateWithWarnings(-o -): %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("migrate -o - wrote nothing to stdout")
	}
	if !strings.Contains(out.String(), "bgp") {
		t.Fatalf("migrate -o - stdout missing config content:\n%s", out.String())
	}
}

// TestDeactivateStdoutSink: `ze config deactivate - <path>` emits the modified
// config to stdout (AC-10, AC-3 for deactivate).
func TestDeactivateStdoutSink(t *testing.T) {
	var out bytes.Buffer
	restore := cliio.SwapStreams(strings.NewReader(showTestConfig), &out)
	defer restore()

	rc := cmdDeactivateImpl(storage.NewFilesystem(), []string{"-", "bgp", "router-id"})
	if rc != exitOK {
		t.Fatalf("deactivate - exit = %d, want %d", rc, exitOK)
	}
	if out.Len() == 0 {
		t.Fatal("deactivate - produced no config on stdout")
	}
}

// TestRollbackRejectsStdin: `ze config rollback <N> -` exits non-zero WITH the
// stdin-specific message — a piped config has no on-disk revision history (AC-11
// deviation). Asserting the message (not just non-zero) means deleting the guard
// fails this test instead of falling through to a "no such file" error.
func TestRollbackRejectsStdin(t *testing.T) {
	restore := cliio.SwapStreams(strings.NewReader(showTestConfig), &bytes.Buffer{})
	defer restore()
	rc, stderr := captureStderr(t, func() int {
		return cmdRollbackImpl(storage.NewFilesystem(), []string{"2", "-"})
	})
	if rc == exitOK {
		t.Fatal("rollback 2 - exited 0; want a non-zero rejection")
	}
	if !strings.Contains(stderr, "revision history") {
		t.Fatalf("rollback - must reject with the stdin-specific message; got: %q", stderr)
	}
}

// TestHistoryRejectsStdin: `ze config history -` is rejected with the
// stdin-specific message (no revision history).
func TestHistoryRejectsStdin(t *testing.T) {
	restore := cliio.SwapStreams(strings.NewReader(showTestConfig), &bytes.Buffer{})
	defer restore()
	rc, stderr := captureStderr(t, func() int {
		return cmdHistoryImpl(storage.NewFilesystem(), []string{"-"})
	})
	if rc == exitOK {
		t.Fatal("history - exited 0; want a non-zero rejection")
	}
	if !strings.Contains(stderr, "revision history") {
		t.Fatalf("history - must reject with the stdin-specific message; got: %q", stderr)
	}
}

// TestEditRejectsStdin: `ze config edit -` is rejected with the stdin-specific
// message — no TTY over a consumed stdin (A-5, AC-12).
func TestEditRejectsStdin(t *testing.T) {
	restore := cliio.SwapStreams(strings.NewReader(showTestConfig), &bytes.Buffer{})
	defer restore()
	rc, stderr := captureStderr(t, func() int {
		return cmdEditWithStorage(storage.NewFilesystem(), []string{"-"})
	})
	if rc == exitOK {
		t.Fatal("edit - exited 0; want a non-zero rejection")
	}
	if !strings.Contains(stderr, "cannot read a config from stdin") {
		t.Fatalf("edit - must reject with the stdin-specific message; got: %q", stderr)
	}
}
