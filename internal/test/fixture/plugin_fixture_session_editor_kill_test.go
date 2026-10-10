// VALIDATES: the session editor driver's `kill` step ends a client whose
// terminal output nobody has read (spec-session-editor-file-mode-parity).
// PREVENTS: a .ci hanging until its timeout because the killed client cannot
// exit while its unread output fills the PTY.

package fixture

import (
	"os"
	"os/exec"
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
