// Design: docs/architecture/testing/ci-format.md -- option=await-file
// Related: await_file.go -- awaitFile, the code under test

package peer

import (
	"bytes"
	"context"
	"errors"
	"net"
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

	if _, err := peer.awaitFile(t.Context(), quietConn(t)); err != nil {
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

	_, err = peer.awaitFile(ctx, quietConn(t))
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
	conn := quietConn(t)
	returned, err := peer.awaitFile(ctx, conn)
	if err != nil {
		t.Fatalf("awaitFile with no file declared: %v", err)
	}
	if returned != conn {
		t.Fatal("awaitFile with no file declared replaced the connection")
	}
}

// quietConn is the peer's side of a connection whose daemon sends nothing, so
// a wait on it measures the file alone.
func quietConn(t *testing.T) net.Conn {
	t.Helper()
	local, remote := net.Pipe()
	t.Cleanup(func() {
		_ = local.Close()
		_ = remote.Close()
	})
	return local
}

// TestAwaitFileTimeoutOption proves option=await-file:timeout= is parsed into
// the wait's own bound, that an absent timeout= leaves the 15 s default, and
// that a bound which is no duration or is not positive is refused rather than
// read as no wait.
func TestAwaitFileTimeoutOption(t *testing.T) {
	config := &Config{}
	if _, err := parseOptionConfig(config, "await-file", map[string]string{"path": "go", "timeout": "240s"}); err != nil {
		t.Fatalf("timeout=240s: %v", err)
	}
	if config.AwaitFileBound != 240*time.Second {
		t.Fatalf("timeout=240s stored %s", config.AwaitFileBound)
	}
	undeclared := &Config{}
	if _, err := parseOptionConfig(undeclared, "await-file", map[string]string{"path": "go"}); err != nil {
		t.Fatalf("no timeout=: %v", err)
	}
	peer, err := New(&Config{Mode: ModeCheck, AwaitFile: "go", Output: new(bytes.Buffer)})
	if err != nil {
		t.Fatalf("new peer: %v", err)
	}
	if bound := peer.awaitFileBound(); bound != 15*time.Second {
		t.Fatalf("undeclared bound is %s, want the 15s default", bound)
	}
	for _, refused := range []string{"soon", "0s", "-5s", ""} {
		if _, err := parseOptionConfig(&Config{}, "await-file", map[string]string{"path": "go", "timeout": refused}); err == nil {
			t.Fatalf("timeout=%q was accepted", refused)
		}
	}
}

// TestAwaitFileHonoursDeclaredBound proves the wait ends at the bound the
// option declared, with the bound named in the error, and not at the context
// or the 15 s default.
func TestAwaitFileHonoursDeclaredBound(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "never.up")
	peer, err := New(&Config{Mode: ModeCheck, AwaitFile: marker, AwaitFileBound: 60 * time.Millisecond, Output: new(bytes.Buffer)})
	if err != nil {
		t.Fatalf("new peer: %v", err)
	}
	started := time.Now()
	_, err = peer.awaitFile(t.Context(), quietConn(t))
	if err == nil {
		t.Fatal("awaitFile with no file and a 60ms bound returned nil")
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("the wait ended on its context, not its bound: %v", err)
	}
	if !strings.Contains(err.Error(), "60ms") || !strings.Contains(err.Error(), marker) {
		t.Fatalf("error %q names neither the 60ms bound nor the file", err)
	}
	if elapsed := time.Since(started); elapsed < 60*time.Millisecond || elapsed > 5*time.Second {
		t.Fatalf("the wait lasted %s, want the 60ms bound", elapsed)
	}
}

// TestAwaitFileServesTheSessionWhileWaiting proves the wait keeps the daemon's
// session alive and loses no frame the script would have read.
//
// Method: over a pipe, the daemon side sends three KEEPALIVEs and requires a
// KEEPALIVE answer to each. In the "held" case it then sends a whole UPDATE,
// which ends the KEEPALIVE service, before the file appears; in the "partial"
// case it sends the first ten octets of that UPDATE. In the held case a
// further KEEPALIVE must still be answered, so a held frame does not end the
// service. Either way the connection the wait returns must give the script
// that UPDATE whole, then a second one the daemon sends after the wait, and
// no KEEPALIVE.
//
// PREVENTS: a wait longer than the hold time expiring the daemon's hold timer
// (bgp-local-pref-strip-gobgp waits on two other daemons' sessions), and a
// frame read while waiting being dropped from the script's view.
func TestAwaitFileServesTheSessionWhileWaiting(t *testing.T) {
	first := append(append([]byte{}, Marker...), 0, 23, MsgUPDATE, 0, 0, 0, 0)
	second := append(append([]byte{}, Marker...), 0, 27, MsgUPDATE, 0, 0, 0, 4, 24, 10, 54, 1)
	for _, before := range []struct {
		name string
		sent int // octets of first the daemon writes before the file appears
	}{
		{name: "held", sent: len(first)},
		{name: "partial", sent: 10},
	} {
		t.Run(before.name, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "release")
			peer, err := New(&Config{Mode: ModeCheck, AwaitFile: marker, AwaitFileBound: 10 * time.Second, Output: new(bytes.Buffer)})
			if err != nil {
				t.Fatalf("new peer: %v", err)
			}
			local, daemon := net.Pipe()
			t.Cleanup(func() {
				_ = local.Close()
				_ = daemon.Close()
			})
			type outcome struct {
				conn net.Conn
				err  error
			}
			waited := make(chan outcome, 1)
			go func() {
				conn, err := peer.awaitFile(t.Context(), local)
				waited <- outcome{conn, err}
			}()

			_ = daemon.SetDeadline(time.Now().Add(5 * time.Second))
			answers := newMessageReader(daemon)
			for range 3 {
				if _, err := daemon.Write(KeepaliveMsg()); err != nil {
					t.Fatalf("daemon KEEPALIVE: %v", err)
				}
				header, _, err := answers.readMessage()
				if err != nil {
					t.Fatalf("no answer to a KEEPALIVE sent while waiting: %v", err)
				}
				if header[18] != MsgKEEPALIVE {
					t.Fatalf("the wait answered a KEEPALIVE with type %d", header[18])
				}
			}
			if _, err := daemon.Write(first[:before.sent]); err != nil {
				t.Fatalf("daemon UPDATE: %v", err)
			}
			if before.sent == len(first) {
				if _, err := daemon.Write(KeepaliveMsg()); err != nil {
					t.Fatalf("daemon KEEPALIVE after the held UPDATE: %v", err)
				}
				header, _, err := answers.readMessage()
				if err != nil || header[18] != MsgKEEPALIVE {
					t.Fatalf("a KEEPALIVE after a held frame went unanswered: %v", err)
				}
			}
			if err := os.WriteFile(marker, nil, 0o600); err != nil {
				t.Fatalf("write release: %v", err)
			}
			result := <-waited
			if result.err != nil {
				t.Fatalf("awaitFile: %v", result.err)
			}

			rest := make(chan error, 1)
			go func() {
				_, err := daemon.Write(append(append([]byte{}, first[before.sent:]...), second...))
				rest <- err
			}()
			_ = result.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			script := newMessageReader(result.conn)
			for index, want := range [][]byte{first, second} {
				header, body, err := script.readMessage()
				if err != nil {
					t.Fatalf("frame %d after the wait: %v", index+1, err)
				}
				if got := append(header, body...); !bytes.Equal(got, want) {
					t.Fatalf("frame %d after the wait is %X, want %X", index+1, got, want)
				}
			}
			if err := <-rest; err != nil {
				t.Fatalf("daemon writes after the wait: %v", err)
			}
		})
	}
}
