//go:build integration && linux

// Design: docs/contributing/patching-the-standard-library.md -- complete statistics descriptor custody.
// Related: govpp_stats_identity_linux_test.go -- genuine Unixpacket segment transfers.
// API: go.fd.io/govpp v0.13.0 adapter/statsclient; github.com/ftrvxmtrx/fd Get transfers every returned file.
package vpp

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"
	"testing"
	"time"

	"go.fd.io/govpp/adapter"
	"go.fd.io/govpp/adapter/statsclient"
)

// A usable first segment and an extra native descriptor must both be closed
// synchronously. The mapped first segment must still return its actual scalar.
// MUTATION: retaining first-file-only cleanup leaks the separately identified extra descriptor.
func TestGoVPPStatsClosesEveryReceivedDescriptor(t *testing.T) {
	for _, pinned := range []bool{false, true} {
		t.Run(fmt.Sprintf("pinned=%t", pinned), func(t *testing.T) {
			first := statsIdentitySegment(t)
			extra := newStatsCustodyFile(t)
			client, done := connectStatsCustodyPair(t, first, extra, pinned)
			if err := client.Connect(); err != nil {
				t.Fatal("original valid segment was refused:", err)
			}
			requireStatsIdentityPeer(t, done)
			requireStatsCustodyFiles(t, first, extra)
			rows, err := client.DumpStats("^/fixture/scalar$")
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 1 {
				t.Fatalf("original scalar rows=%d, want1", len(rows))
			}
			value, ok := rows[0].Data.(adapter.ScalarStat)
			if !ok {
				t.Fatalf("original scalar type=%T", rows[0].Data)
			}
			if value != 4242 {
				t.Fatalf("original scalar=%v, want4242", value)
			}
		})
	}
}

// A zero-length first file fails its actual mmap; that original failure must
// close the entire transferred pair without publishing a statistics mapping.
// MUTATION: first-file-only cleanup leaks the extra descriptor on this error path too.
func TestGoVPPStatsMappingFailureClosesEveryReceivedDescriptor(t *testing.T) {
	for _, pinned := range []bool{false, true} {
		t.Run(fmt.Sprintf("pinned=%t", pinned), func(t *testing.T) {
			first := newStatsCustodyFile(t)
			extra := newStatsCustodyFile(t)
			client, done := connectStatsCustodyPair(t, first, extra, pinned)
			err := client.Connect()
			if err == nil {
				t.Fatal("empty original mapping was admitted")
			}
			if !strings.HasPrefix(err.Error(), "mapping shared memory failed:") {
				t.Fatalf("original mmap cause was replaced: %v", err)
			}
			requireStatsIdentityPeer(t, done)
			requireStatsCustodyFiles(t, first, extra)
			if _, err := client.DumpStats(); !errors.Is(err, adapter.ErrStatsDisconnected) {
				t.Fatalf("failed original mapping published statistics: %v", err)
			}
		})
	}
}

// The existing fixture owns and joins its one peer. Both files remain owned by
// this question; SCM_RIGHTS creates distinct receiver descriptors for them.
// No parallel question may run here: the short GC setting prevents an unrelated
// finalizer from disguising a missing synchronous Close and is restored on exit.
func connectStatsCustodyPair(t *testing.T, first, extra *os.File, pinned bool) (
	*statsclient.StatsClient, <-chan error) {
	t.Helper()
	previousGC := debug.SetGCPercent(-1)
	t.Cleanup(func() { debug.SetGCPercent(previousGC) })
	requireStatsCustodyFiles(t, first, extra)
	const probe = "original-stats-custody"
	path, done, _ := startStatsIdentityPeer(t, func(conn *net.UnixConn) error {
		if pinned {
			if err := readStatsIdentityProbe(conn, probe); err != nil {
				return err
			}
		}
		rights := syscall.UnixRights(int(first.Fd()), int(extra.Fd()))
		n, ancillary, err := conn.WriteMsgUnix([]byte{1}, rights, nil)
		if err != nil {
			return err
		}
		if n != 1 {
			return fmt.Errorf("descriptor pair payload=%d, want1", n)
		}
		if ancillary != len(rights) {
			return fmt.Errorf("descriptor pair ancillary=%d, want%d", ancillary, len(rights))
		}
		return statsIdentityEOF(conn)
	})
	options := []statsclient.Option{statsclient.SetSocketRetryTimeout(time.Second)}
	if pinned {
		options = append(options, statsclient.WithPinnedConnection(func(conn *net.UnixConn) error {
			return writeStatsIdentityProbe(conn, probe)
		}))
	}
	client := statsclient.NewStatsClient(path, options...)
	t.Cleanup(func() {
		if err := client.Disconnect(); err != nil {
			t.Error(err)
		}
	})
	return client, done
}

func newStatsCustodyFile(t *testing.T) *os.File {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "custody-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	})
	return file
}

// Count only descriptors for these two retained device/inode identities.
// Unrelated descriptor churn is permitted; no pathname grants close authority.
func requireStatsCustodyFiles(t *testing.T, files ...*os.File) {
	t.Helper()
	const fdDir = "/proc/self/fd"
	entries, err := os.ReadDir(fdDir)
	if err != nil {
		t.Fatal("cannot observe actual descriptor custody:", err)
	}
	for _, file := range files {
		original, err := file.Stat()
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, entry := range entries {
			actual, err := os.Stat(filepath.Join(fdDir, entry.Name()))
			if errors.Is(err, os.ErrNotExist) {
				continue // an unrelated descriptor closed after the directory snapshot
			}
			if err != nil {
				t.Fatal("descriptor custody observation failed:", err)
			}
			if os.SameFile(original, actual) {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("original file %q has%d live descriptors, want only its one retained sender", file.Name(), count)
		}
	}
}
