// VALIDATES: the session editor driver's `kill` step ends a client whose
// terminal output nobody has read (spec-session-editor-file-mode-parity).
// PREVENTS: a .ci hanging until its timeout because the killed client cannot
// exit while its unread output fills the PTY.

package fixture

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
)

// TestSessionEditorKillDrainsTerminal kills a client that has filled its PTY
// with output the driver never read, as a session editor does when a script
// sends commands and then only polls the daemon. The method is a child that
// writes without end; the kill must return well inside the PTY read deadline.
func TestSessionEditorKillDrainsTerminal(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), "sh", "-c", "yes 'a status line redrawn every poll'")
	terminal, err := pty.Start(cmd)
	if err != nil {
		t.Fatalf("start under a PTY: %v", err)
	}
	defer terminal.Close() //nolint:errcheck // test teardown

	// Let the child fill the PTY buffer and block on its next write.
	time.Sleep(300 * time.Millisecond)

	done := make(chan error, 1)
	go sessionEditorKillWorker(cmd, terminal, done)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("kill: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("kill did not return: the killed client is waiting for its output to be read")
	}
	if cmd.ProcessState == nil {
		t.Fatal("kill returned before the client was reaped")
	}
}

// sessionEditorKillWorker runs the kill and hands its answer to done, so the
// test can bound a kill that never returns.
func sessionEditorKillWorker(cmd *exec.Cmd, terminal *os.File, done chan<- error) {
	_, err := killSessionEditor(cmd, terminal)
	done <- err
}

// TestSessionEditorQuitReportsLiveClient runs the script driver against a real
// PTY client that asks for confirmation instead of exiting after quit. The
// driver's own PTY timeout must report that prompt while the outer context is
// still live, and its deferred cleanup must terminate and reap the client.
func TestSessionEditorQuitReportsLiveClient(t *testing.T) {
	dir := t.TempDir()
	fakeZe := filepath.Join(dir, "ze")
	pidPath := filepath.Join(dir, "client.pid")
	script := `#!/bin/sh
printf '%s\n' "$$" > "$3"
printf '\033[?1049hready\n'
IFS= read -r command
if [ "$command" != quit ]; then
    exit 2
fi
printf 'Pending changes. Use commit now or discard all.\n'
while IFS= read -r ignored; do :; done
`
	if err := os.WriteFile(fakeZe, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	ctx, cancel := context.WithTimeout(t.Context(), ptyReadTimeout04+5*time.Second)
	defer cancel()
	err := sessionEditorRun(ctx, os.Environ(), pidPath, []sessionEditorStep{
		{verb: sessionEditorWait, text: "ready"},
	})
	if err == nil {
		t.Fatal("a client awaiting quit confirmation must fail")
	}
	if ctx.Err() != nil {
		t.Fatalf("the outer deadline killed the client before the fixture reported its own timeout: %v", err)
	}
	if !strings.Contains(err.Error(), "config editor did not exit after quit: output deadline expired") {
		t.Fatalf("missing the driver's bounded quit refusal: %v", err)
	}
	if !strings.Contains(err.Error(), "Pending changes. Use commit now or discard all.") {
		t.Fatalf("quit refusal lost the terminal prompt: %v", err)
	}
	pidText, readErr := os.ReadFile(pidPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	pid, parseErr := strconv.Atoi(strings.TrimSpace(string(pidText)))
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	if _, waitErr := syscall.Wait4(pid, nil, syscall.WNOHANG, nil); !errors.Is(waitErr, syscall.ECHILD) {
		t.Fatalf("the fixture returned before reaping client %d: %v", pid, waitErr)
	}
}
