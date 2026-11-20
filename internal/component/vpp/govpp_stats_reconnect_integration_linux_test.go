//go:build integration && linux

// Design: docs/contributing/patching-the-standard-library.md -- immutable pinned statistics mapping.
// Related: govpp_stats_identity_linux_test.go -- real V2 segment and native Unixpacket helpers.
// API: go.fd.io/govpp v0.13.0 adapter/statsclient.WithPinnedConnection suppresses hidden reconnect.
package vpp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"go.fd.io/govpp/adapter"
	"go.fd.io/govpp/adapter/statsclient"
)

// Replace the actual socket after publishing a readable original segment.
// The unpinned control must map7777 from that replacement, proving the native
// trigger works. Pinned mode must retain4242 without accepting another peer.
// MUTATION: starting monitorSocket unconditionally admits the replacement in pinned mode.
func TestGoVPPPinnedStatsDoesNotReconnectToReplacement(t *testing.T) {
	for _, pinned := range []bool{false, true} {
		t.Run(fmt.Sprintf("pinned=%t", pinned), func(t *testing.T) {
			original := statsIdentitySegment(t)
			replacement := statsIdentitySegment(t)
			var scalar [8]byte
			binary.NativeEndian.PutUint64(scalar[:], 7777)
			written, err := replacement.WriteAt(scalar[:], 72)
			if err != nil {
				t.Fatal(err)
			}
			if written != len(scalar) {
				t.Fatalf("replacement scalar write=%d, want%d", written, len(scalar))
			}
			path, originalDone, _ := startStatsIdentityPeer(t, func(conn *net.UnixConn) error {
				return sendStatsReconnectSegment(conn, original)
			})
			var validations atomic.Int32
			options := []statsclient.Option{statsclient.SetSocketRetryTimeout(time.Second)}
			if pinned {
				options = append(options, statsclient.WithPinnedConnection(func(*net.UnixConn) error {
					validations.Add(1)
					return nil
				}))
			}
			client := statsclient.NewStatsClient(path, options...)
			t.Cleanup(func() {
				if err := client.Disconnect(); err != nil {
					t.Error(err)
				}
			})
			if err := client.Connect(); err != nil {
				t.Fatal(err)
			}
			requireStatsIdentityPeer(t, originalDone)
			requireStatsReconnectScalar(t, client, 4242)
			replacementPath, accepted, replacementDone, stop := startStatsReconnectReplacement(t, replacement)
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(replacementPath, path); err != nil {
				t.Fatal(err)
			}
			if pinned {
				select {
				case <-accepted:
					t.Fatal("pinned statistics dialed the replacement socket")
				case <-time.After(time.Second):
					// A bounded absence observation; the unpinned sibling proves this trigger.
				}
				requireStatsReconnectScalar(t, client, 4242)
				if validations.Load() != 1 {
					t.Fatalf("pinned original validator calls=%d, want1", validations.Load())
				}
				stop()
				return
			}
			select {
			case err := <-replacementDone:
				if err != nil {
					t.Fatal("unpinned replacement exchange failed:", err)
				}
			case <-time.After(statsIdentityBound):
				t.Fatal("unpinned control did not complete the actual replacement transfer")
			}
			requireStatsReconnectScalar(t, client, 7777)
			stop()
		})
	}
}

func requireStatsReconnectScalar(t *testing.T, client *statsclient.StatsClient, expected adapter.ScalarStat) {
	t.Helper()
	rows, err := client.DumpStats("^/fixture/scalar$")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("native mapped scalar rows=%d, want1", len(rows))
	}
	actual, ok := rows[0].Data.(adapter.ScalarStat)
	if !ok {
		t.Fatalf("native mapped scalar type=%T", rows[0].Data)
	}
	if actual != expected {
		t.Fatalf("native mapped scalar=%v, want%v", actual, expected)
	}
}

func sendStatsReconnectSegment(conn *net.UnixConn, file *os.File) error {
	rights := syscall.UnixRights(int(file.Fd()))
	n, ancillary, err := conn.WriteMsgUnix([]byte{1}, rights, nil)
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("replacement payload=%d, want1", n)
	}
	if ancillary != len(rights) {
		return fmt.Errorf("replacement ancillary=%d, want%d", ancillary, len(rights))
	}
	return statsIdentityEOF(conn)
}

// Own one replacement listener and one bounded accept/exchange worker. stop
// closes listener/accepted socket and joins this worker, even when pinned mode
// correctly makes no connection. Channels have one sender and a bounded slot.
func startStatsReconnectReplacement(t *testing.T, file *os.File) (
	string, <-chan struct{}, <-chan error, func()) {
	t.Helper()
	// A short name: under a long TMPDIR this subtest's directory leaves no
	// room for "replacement.sock" in the 107 bytes sun_path holds, and the
	// socket must share a filesystem with the original for the rename.
	path := filepath.Join(t.TempDir(), "r.sock")
	listener, err := net.ListenUnix("unixpacket", &net.UnixAddr{Name: path, Net: "unixpacket"})
	if err != nil {
		t.Fatal(err)
	}
	if err := listener.SetDeadline(time.Now().Add(statsIdentityBound)); err != nil {
		if closeErr := listener.Close(); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal(err)
	}
	accepted := make(chan struct{})
	done := make(chan error, 1)
	joined := make(chan struct{})
	var mu sync.Mutex
	var peer *net.UnixConn
	stopped := false
	go func() {
		defer close(joined)
		conn, err := listener.AcceptUnix()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				err = nil // stop owns a deliberately unused listener in the pinned case
			}
			done <- err
			return
		}
		mu.Lock()
		if stopped {
			mu.Unlock()
			done <- conn.Close()
			return
		}
		peer = conn
		mu.Unlock()
		close(accepted)
		if err := conn.SetDeadline(time.Now().Add(statsIdentityBound)); err != nil {
			done <- errors.Join(err, conn.Close())
			return
		}
		err = sendStatsReconnectSegment(conn, file)
		closeErr := conn.Close()
		if errors.Is(closeErr, net.ErrClosed) {
			closeErr = nil
		}
		done <- errors.Join(err, closeErr)
	}()
	var stopOnce sync.Once
	stop := func() {
		stopOnce.Do(func() {
			mu.Lock()
			stopped = true
			if peer != nil {
				if err := peer.Close(); err != nil {
					if !errors.Is(err, net.ErrClosed) {
						t.Error(err)
					}
				}
			}
			mu.Unlock()
			if err := listener.Close(); err != nil {
				if !errors.Is(err, net.ErrClosed) {
					t.Error(err)
				}
			}
			select {
			case <-joined:
			case <-time.After(statsIdentityBound):
				t.Error("replacement statistics peer worker did not join")
			}
		})
	}
	t.Cleanup(stop)
	return path, accepted, done, stop
}
