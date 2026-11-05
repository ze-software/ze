// Design: docs/architecture/testing/runner-architecture.md -- peer readiness and child ownership.

package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// peerStartupRecord installs external stand-ins for the peer and its consumer.
// These test capture and startup ordering; the real peer owns the BGP exchange.
func peerStartupRecord(t *testing.T, orchestrated bool, timeout, body string) (*Runner, *Record, string) {
	t.Helper()
	r, rec := lastHelperRecord(t, "peer-startup", timeout, &RunCommand{})
	dir := t.TempDir()
	consumer := filepath.Join(dir, "consumer-ran")
	release := filepath.Join(dir, "consumer-release")
	if err := syscall.Mkfifo(release, 0o600); err != nil {
		t.Fatal(err)
	}
	r.lePath = filepath.Join(dir, "peer")
	if err := os.WriteFile(r.lePath, []byte("#!/bin/sh\nrelease='"+release+"'\n"+body), 0o700); err != nil {
		t.Fatal(err)
	}
	r.zePath = filepath.Join(dir, "consumer")
	if err := os.WriteFile(r.zePath, []byte("#!/bin/sh\nprintf consumer-ran > '"+consumer+
		"'\necho consumer-ran\necho release > '"+release+"'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	rec.RunCommands = nil
	if orchestrated {
		rec.RunCommands = []RunCommand{
			{Mode: modeBackground, Seq: 1, Exec: "le test peer"},
			{Mode: modeForeground, Seq: 2, Exec: r.zePath, ExpectStdout: []string{"consumer-ran"}},
		}
	}
	return r, rec, consumer
}

// TestPeerStartupUsesCaseBudget drives both production startup paths past the
// old five-second cutoff. Six seconds is an intentional delayed-ready input,
// not an upper wall-clock assertion: success requires the following consumer
// and the peer's positive completion, with the case deadline still enforced.
func TestPeerStartupUsesCaseBudget(t *testing.T) {
	for _, orchestrated := range []bool{false, true} {
		name := "legacy"
		if orchestrated {
			name = "orchestrated"
		}
		t.Run(name, func(t *testing.T) {
			// Like the real peer, completion requires the consumer's exchange.
			// Otherwise legacy teardown may signal a consumer that never ran.
			r, rec, consumer := peerStartupRecord(t, orchestrated, "30s",
				"sleep 6\necho 'listening on fixture'\nread reply < \"$release\"\necho successful\n")
			if !r.runTest(t.Context(), rec, &RunOptions{}) {
				t.Fatalf("peer within case budget refused: %v", rec.Error)
			}
			got, err := os.ReadFile(consumer)
			if err != nil {
				t.Fatalf("consumer never ran after peer readiness: %v", err)
			}
			if string(got) != "consumer-ran" {
				t.Fatalf("consumer result=%q", got)
			}
			if !strings.Contains(rec.PeerOutput, "successful") {
				t.Fatalf("peer never completed: %q", rec.PeerOutput)
			}
		})
	}
}

// TestPeerStartupReportsExit preserves the child's exit status and both final
// streams instead of consuming the remaining case budget waiting for readiness.
func TestPeerStartupReportsExit(t *testing.T) {
	for _, orchestrated := range []bool{false, true} {
		name := "legacy"
		if orchestrated {
			name = "orchestrated"
		}
		t.Run(name, func(t *testing.T) {
			r, rec, consumer := peerStartupRecord(t, orchestrated, "30s",
				"echo startup-out\necho startup-err >&2\nexit 23\n")
			if r.runTest(t.Context(), rec, &RunOptions{}) {
				t.Fatal("peer exit before listening passed")
			}
			for _, want := range []string{"exited before listening", "exit status 23", "startup-out", "startup-err"} {
				if rec.Error == nil || !strings.Contains(rec.Error.Error(), want) {
					t.Errorf("startup error=%v, want %q", rec.Error, want)
				}
			}
			if rec.FailureType != FailTypePeerNeverBound {
				t.Errorf("failure=%q, want %q", rec.FailureType, FailTypePeerNeverBound)
			}
			if _, err := os.Stat(consumer); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("consumer ran without readiness: %v", err)
			}
		})
	}
}

// TestPeerStartupHonorsDeadline leaves the peer unready and requires the actual
// case deadline cause; a separate fixed startup timer cannot supply it.
func TestPeerStartupHonorsDeadline(t *testing.T) {
	for _, orchestrated := range []bool{false, true} {
		name := "legacy"
		if orchestrated {
			name = "orchestrated"
		}
		t.Run(name, func(t *testing.T) {
			r, rec, consumer := peerStartupRecord(t, orchestrated, "100ms", "exec sleep 60\n")
			if r.runTest(t.Context(), rec, &RunOptions{}) {
				t.Fatal("unready peer passed at case deadline")
			}
			if !errors.Is(rec.Error, context.DeadlineExceeded) {
				t.Errorf("startup error=%v, want case deadline cause", rec.Error)
			}
			if _, err := os.Stat(consumer); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("consumer ran without readiness: %v", err)
			}
		})
	}
}

// TestPeerStartupHonorsCancellation cancels only after the actual peer owner
// starts. The run must join that child and retain cancellation as its cause.
func TestPeerStartupHonorsCancellation(t *testing.T) {
	for _, orchestrated := range []bool{false, true} {
		name := "legacy"
		if orchestrated {
			name = "orchestrated"
		}
		t.Run(name, func(t *testing.T) {
			r, rec, consumer := peerStartupRecord(t, orchestrated, "30s", "exec sleep 60\n")
			started := make(chan struct{})
			r.observeProcessExit = func(p *ownedProcess) error {
				close(started)
				return p.beforeReap()
			}
			ctx, cancel := context.WithCancel(t.Context())
			result := make(chan bool, 1)
			finished := make(chan struct{})
			defer func() {
				cancel()
				<-finished
			}()
			go func() {
				result <- r.runTest(ctx, rec, &RunOptions{})
				close(finished)
			}()
			select {
			case <-started:
			case passed := <-result:
				t.Fatalf("run ended before peer started: passed=%v, error=%v", passed, rec.Error)
			case <-t.Context().Done():
				t.Fatal("peer owner did not start")
			}
			cancel()
			if <-result {
				t.Fatal("unready peer passed after cancellation")
			}
			if !errors.Is(rec.Error, context.Canceled) {
				t.Errorf("startup error=%v, want cancellation cause", rec.Error)
			}
			if _, err := os.Stat(consumer); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("consumer ran without readiness: %v", err)
			}
		})
	}
}
