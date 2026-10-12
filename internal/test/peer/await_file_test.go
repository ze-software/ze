// Design: docs/architecture/testing/ci-format.md -- option=await-file
// Related: await_file.go -- awaitFile, the code under test

package peer

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestAwaitFileOptionNeedsPath proves the option parser stores the file name and
// refuses a declaration with none, which would otherwise make the wait a no-op.
func TestAwaitFileOptionNeedsPath(t *testing.T) {
	config := &Config{}
	claimed, err := parseOptionConfig(config, "await-file", map[string]string{"path": "dest.up"})
	if err != nil || !claimed || config.AwaitFile != "dest.up" {
		t.Fatalf("path=dest.up: claimed=%v err=%v file=%q", claimed, err, config.AwaitFile)
	}
	if _, err := parseOptionConfig(&Config{}, "await-file", map[string]string{}); err == nil {
		t.Fatal("option=await-file with no path was accepted")
	}
}

// TestAwaitFileHoldsUntilTheFileExists proves the wait is real: awaitFile does
// not return while the file is absent, and returns nil once another writer
// creates it.
//
// The method creates the file from the test after a delay and checks that
// awaitFile returned no earlier than that write.
//
// VALIDATES: option=await-file holds a peer's first send until another peer
//
//	block's established-file exists.
//
// PREVENTS: a sender that races the receiver's session, so ze forwards the
//
//	burst to a peer it does not yet hold Established.
func TestAwaitFileHoldsUntilTheFileExists(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "dest.up")
	peer, err := New(&Config{Mode: ModeCheck, AwaitFile: marker, Output: new(bytes.Buffer)})
	if err != nil {
		t.Fatalf("new peer: %v", err)
	}

	written := make(chan time.Time, 1)
	timer := time.AfterFunc(200*time.Millisecond, func() {
		if werr := os.WriteFile(marker, nil, 0o600); werr != nil {
			t.Errorf("write marker: %v", werr)
		}
		written <- time.Now()
	})
	defer timer.Stop()

	if err := peer.awaitFile(t.Context()); err != nil {
		t.Fatalf("awaitFile: %v", err)
	}
	returned := time.Now()
	select {
	case at := <-written:
		if returned.Before(at) {
			t.Fatalf("awaitFile returned at %v, before the file was written at %v", returned, at)
		}
	default:
		t.Fatal("awaitFile returned before the file was written")
	}
}

// TestAwaitFileFailsNamingTheFile proves a wait that ends before the file exists
// is an error naming the file, never a silent go-ahead.
func TestAwaitFileFailsNamingTheFile(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "never.up")
	peer, err := New(&Config{Mode: ModeCheck, AwaitFile: marker, Output: new(bytes.Buffer)})
	if err != nil {
		t.Fatalf("new peer: %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	err = peer.awaitFile(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("awaitFile with no file: err=%v, want the context deadline", err)
	}
	if !strings.Contains(err.Error(), marker) {
		t.Fatalf("error %q does not name the awaited file", err)
	}
}

// TestAwaitFileAbsentIsNoWait proves a peer that declares no await-file sends at
// once, so the option changes nothing for the tests that do not use it.
func TestAwaitFileAbsentIsNoWait(t *testing.T) {
	peer, err := New(&Config{Mode: ModeCheck, Output: new(bytes.Buffer)})
	if err != nil {
		t.Fatalf("new peer: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := peer.awaitFile(ctx); err != nil {
		t.Fatalf("awaitFile with no file declared: %v", err)
	}
}
