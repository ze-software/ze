// Design: docs/architecture/exabgp-bridge.md -- standalone runner mirrors its script's exit status without writing withdrawals (E-6, S42, AC-45).
// Related: bridge.go -- Bridge.Run and Bridge.runIO.

package bridge

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestStandaloneExitsWithItsScript verifies that the standalone Bridge runner
// exits promptly with its script's exact exit code (preserving *exec.ExitError)
// even when Ze stdin remains open, and writes no withdrawal commands to stdout
// after the script exits.
//
// VALIDATES: AC-45 / S42 / E-6 — OwnerLedger is deleted, so standalone mode
// mirrors the script's exit code directly without emitting withdrawals or
// blocking on stdin EOF.
// PREVENTS: standalone mode swallowing a non-zero exit code, emitting stale
// OwnerLedger withdrawals on exit, or hanging forever waiting for os.Stdin EOF.
func TestStandaloneExitsWithItsScript(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	// Pipe simulating Ze's stdin/stdout toward the standalone bridge.
	// Deliberately keep zeInW open until after runIO returns to prove runIO
	// does not block waiting for stdin EOF once the script exits.
	zeInR, zeInW := io.Pipe()
	defer func() { _ = zeInW.Close() }()
	zeOutR, zeOutW := io.Pipe()
	defer func() { _ = zeOutR.Close() }()

	// Script announces one route, waits for its ack, then exits with code 3.
	b := NewBridge([]string{
		"sh", "-c",
		`printf 'announce route 10.9.0.0/24 next-hop 192.0.2.1\n'; IFS= read -r _; exit 3`,
	})

	// Capture every line written to zeOut and answer startup + MuxConn dispatch/flush.
	var captured bytes.Buffer
	responderDone := make(chan struct{})
	go func() {
		defer close(responderDone)
		scanner := bufio.NewScanner(zeOutR)
		for scanner.Scan() {
			line := scanner.Text()
			captured.WriteString(line)
			captured.WriteByte('\n')

			switch {
			case line == "declare done":
				_, _ = io.WriteString(zeInW, "config done\n")
			case line == "capability done":
				_, _ = io.WriteString(zeInW, "registry done\n")
			case strings.HasPrefix(line, "#"):
				id, verb, _, err := parseMuxLine(line)
				if err != nil {
					continue
				}
				switch verb {
				case "ze-plugin-engine:dispatch-command":
					term := strings.TrimRight(string(rpc.AppendAnswerTerminator(nil, id, 0, 0, "")), "\n")
					_, _ = io.WriteString(zeInW, term+"\n")
				case "ze-bgp:peer-flush":
					_, _ = io.WriteString(zeInW, formatMuxOK(id)+"\n")
				}
			}
		}
	}()

	var stderrBuf bytes.Buffer
	runErrCh := make(chan error, 1)
	go func() {
		runErrCh <- b.runIO(ctx, zeInR, zeOutW, &stderrBuf)
	}()

	var runErr error
	select {
	case runErr = <-runErrCh:
	case <-time.After(5 * time.Second):
		t.Fatal("Bridge.runIO blocked after script exited (likely waiting on open stdin)")
	}

	// Close pipes so responder finishes capturing everything written before runIO returned.
	_ = zeOutW.Close()
	_ = zeInW.Close()
	<-responderDone

	require.Error(t, runErr, "expected script exit error")
	exitErr, ok := errors.AsType[*exec.ExitError](runErr)
	require.True(t, ok, "error must wrap *exec.ExitError, got %T: %v", runErr, runErr)
	assert.Equal(t, 3, exitErr.ExitCode(), "standalone bridge must mirror script exit code 3")

	out := captured.String()
	assert.Contains(t, out, "nlri add 10.9.0.0/24", "the script's own announce must have been dispatched")
	assert.NotContains(t, out, "nlri del", "standalone exit must write no withdrawals to stdout")
	assert.NotContains(t, out, "restore-configured", "standalone exit must write no restoring withdrawals")
}
