//go:build linux

// Design: docs/functional-tests.md -- FlowSpec peer-loss assertions stay within their case.

package fixture

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/test/runner"
)

const flowSpecPeerLookupChild = "--flowspec-peer-lookup-child"

// init stops these owned stand-ins before the test binary parses its flags.
// Their kernel-visible argv is the real command shape under examination.
func init() {
	if !slices.Contains(os.Args, flowSpecPeerLookupChild) {
		return
	}
	if _, err := os.Stdout.Write([]byte{'R'}); err != nil {
		os.Exit(1)
	}
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

// TestFlowSpecPeerLookupUsesCaseProcess drives the actual /proc lookup with
// live processes, not a copy of its matching predicate. The canonical peer is
// useful output; a legacy command, another fixture, a sibling case and a
// different executable are permitted survivors that this case must not stop.
//
// MUTATION: restore argv[1] == "peer" and the real test-peer shape disappears.
// Removing either case or executable scoping also admits a live control.
func TestFlowSpecPeerLookupUsesCaseProcess(t *testing.T) {
	work := t.TempDir()
	t.Chdir(work)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	peer := startFlowSpecLookupChild(t, work, executable, "test", "peer", flowSpecPeerLookupChild)
	startFlowSpecLookupChild(t, work, executable, "peer", flowSpecPeerLookupChild)
	startFlowSpecLookupChild(t, work, executable, "test", "fixture", flowSpecPeerLookupChild)
	startFlowSpecLookupChild(t, t.TempDir(), executable, "test", "peer", flowSpecPeerLookupChild)
	// This shell has the same "test peer" argv but is not the fixture binary.
	// Its one-byte ready fence precedes a read held open by the parent.
	if err := os.WriteFile(filepath.Join(work, "test"), []byte("printf R\nread release\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	startFlowSpecLookupChild(t, work, "sh", "test", "peer")

	peers, err := peerPIDs07()
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 1 || peers[0] != peer {
		t.Fatalf("peer processes=%v, want only this case's canonical peer %d", peers, peer)
	}
}

// startFlowSpecLookupChild owns each real child until the test ends. It MUST
// register Close immediately after StartServerProcess and join it before
// releasing the child's pipes or test directory.
func startFlowSpecLookupChild(t *testing.T, directory, executable string, args ...string) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)
	input, hold, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	ready, output, err := os.Pipe()
	if err != nil {
		input.Close() //nolint:errcheck // cleanup after the second pipe failed
		hold.Close()  //nolint:errcheck // cleanup after the second pipe failed
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, pipe := range []*os.File{input, hold, ready, output} {
			pipe.Close() //nolint:errcheck // owner has joined; only descriptor cleanup remains
		}
	})
	command := exec.CommandContext(ctx, executable, args...)
	command.Dir = directory
	command.Stdin, command.Stdout, command.Stderr = input, output, output
	owner, err := runner.StartServerProcess(command)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Errorf("lookup child cleanup: %v", err)
		}
	})
	deadline, _ := ctx.Deadline() // WithTimeout above supplies the single child budget.
	if err := ready.SetReadDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	var acknowledgment [1]byte
	if _, err := io.ReadFull(ready, acknowledgment[:]); err != nil {
		t.Fatalf("lookup child never became ready: %v", err)
	}
	if acknowledgment[0] != 'R' {
		t.Fatalf("lookup child ready byte=%q", acknowledgment)
	}
	select {
	case <-owner.Done():
		t.Fatalf("lookup child exited after readiness: %v", owner.Wait())
	default:
	}
	return command.Process.Pid
}
