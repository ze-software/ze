package rpc

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/taskguard"
)

// readerGuardEvidence is what the real reader guard has produced so far: its
// count for one task and the guard's ERROR lines in the log ring. A test takes
// it before the panic and compares after, so other tests' panics do not count.
type readerGuardEvidence struct {
	panics uint64
	lines  int
}

func snapshotReaderGuard(t *testing.T, task string) readerGuardEvidence {
	t.Helper()
	count, declared := readerPanics().guard.Panics(task)
	if !declared {
		t.Fatalf("task %q is not declared on the plugin rpc guard", task)
	}
	entries, err := slogutil.GlobalLogRing().Snapshot(0, "ERROR", "plugin.rpc")
	if err != nil {
		t.Fatalf("log ring snapshot: %v", err)
	}
	lines := 0
	for _, e := range entries {
		if strings.Contains(e.Message, "an internal Ze defect ended this unit") {
			lines++
		}
	}
	return readerGuardEvidence{panics: count, lines: lines}
}

// requireReaderContained asserts one more panic was counted, logged and
// crash-recorded since before.
func requireReaderContained(t *testing.T, task string, before readerGuardEvidence) {
	t.Helper()
	after := snapshotReaderGuard(t, task)
	if after.panics != before.panics+1 {
		t.Errorf("plugin %s panics = %d, want %d: the panic was not counted", task, after.panics, before.panics+1)
	}
	if after.lines != before.lines+1 {
		t.Errorf("guard ERROR lines = %d, want %d: the panic was not logged", after.lines, before.lines+1)
	}
	crashes := crashlog.ListCrashes()
	if len(crashes) != 1 || crashes[0].Kind != crashlog.KindRecovered {
		t.Errorf("crash records = %+v, want one %q record", crashes, crashlog.KindRecovered)
	}
}

// panickingReader is a connection whose Read panics: the state a defect in
// the transport under a Conn would leave. Close records that it ran.
type panickingReader struct{ closed chan struct{} }

func (r *panickingReader) Read([]byte) (int, error) { panic("boom: transport defect") }

func (r *panickingReader) Close() error {
	close(r.closed)
	return nil
}

type discardWriteCloser struct{}

func (discardWriteCloser) Write(b []byte) (int, error) { return len(b), nil }
func (discardWriteCloser) Close() error                { return nil }

// TestRPCReadLoopPanicContained: a panic in one connection's frame reader
// (Conn.readLoop, through a transport whose Read panics) ends that connection
// only. Its reader returns the contained panic as its error and the transport
// is closed, while a second connection keeps reading requests. The panic is
// counted on the real guard, logged and crash-recorded. Without the boundary
// the panic ends the test binary.
func TestRPCReadLoopPanicContained(t *testing.T) {
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))
	before := snapshotReaderGuard(t, taskRPCRead)

	broken := &panickingReader{closed: make(chan struct{})}
	brokenConn := NewConn(broken, discardWriteCloser{})
	brokenConn.SetLabel("plugin-a")

	engineSide, pluginSide := net.Pipe()
	t.Cleanup(func() { _ = pluginSide.Close() })
	healthy := NewConn(engineSide, engineSide)
	t.Cleanup(func() { _ = healthy.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := brokenConn.ReadRequest(ctx)
	var contained *taskguard.Panic
	if !errors.As(err, &contained) {
		t.Fatalf("ReadRequest on the panicking connection = %v, want the contained *taskguard.Panic", err)
	}
	if contained.Unit != "plugin connection plugin-a" {
		t.Errorf("contained unit = %q, want the connection's label", contained.Unit)
	}
	select {
	case <-broken.closed:
	case <-ctx.Done():
		t.Fatal("the panicking connection's transport was not closed")
	}
	if !errors.As(brokenConn.Err(), &contained) {
		t.Errorf("Err() = %v, want the contained panic", brokenConn.Err())
	}

	go func() { _, _ = pluginSide.Write([]byte("#7 ze-plugin-engine:ping {}\n")) }()
	req, err := healthy.ReadRequest(ctx)
	if err != nil {
		t.Fatalf("the healthy connection stopped reading after the other one panicked: %v", err)
	}
	if req.ID != 7 {
		t.Errorf("healthy request id = %d, want 7", req.ID)
	}

	requireReaderContained(t, taskRPCRead, before)
}

// TestMuxReadLoopPanicContained: a panic in one MuxConn's line router
// (MuxConn.readLoop, through a pending call whose channel a defect closed, so
// routing its response sends on a closed channel) ends that connection only:
// Done and Requests close, a later call fails with the contained panic, and
// the remote side reads EOF. A second MuxConn keeps routing inbound requests.
// Without the boundary the panic ends the test binary.
func TestMuxReadLoopPanicContained(t *testing.T) {
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))
	before := snapshotReaderGuard(t, taskRPCRoute)

	brokenLocal, brokenRemote := net.Pipe()
	t.Cleanup(func() { _ = brokenRemote.Close() })
	broken := NewMuxConn(NewConn(brokenLocal, brokenLocal))
	t.Cleanup(func() { _ = broken.Close() })

	healthyLocal, healthyRemote := net.Pipe()
	t.Cleanup(func() { _ = healthyRemote.Close() })
	healthy := NewMuxConn(NewConn(healthyLocal, healthyLocal))
	t.Cleanup(func() { _ = healthy.Close() })

	closedCall := make(chan []byte, 1)
	close(closedCall)
	broken.pending.Store("1", closedCall)
	if _, err := brokenRemote.Write([]byte("#1 ok\n")); err != nil {
		t.Fatalf("write the response: %v", err)
	}

	select {
	case <-broken.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the panicking MuxConn's reader did not end")
	}
	if _, open := <-broken.Requests(); open {
		t.Error("Requests is still open after the contained panic")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var contained *taskguard.Panic
	if _, err := broken.CallRPC(ctx, "ze-plugin-engine:ping", nil); !errors.As(err, &contained) {
		t.Errorf("CallRPC after the panic = %v, want the contained *taskguard.Panic", err)
	}
	// A net.Pipe read returns EOF at once when the other end is closed, and
	// blocks while it is open, so a closed local end is what lets this return.
	if _, err := bufio.NewReader(brokenRemote).ReadByte(); !errors.Is(err, io.EOF) {
		t.Errorf("remote read after the panic = %v, want EOF: the connection was not closed", err)
	}

	go func() { _, _ = healthyRemote.Write([]byte("#9 ze-plugin-callback:ping {}\n")) }()
	select {
	case req := <-healthy.Requests():
		if req == nil || req.ID != 9 {
			t.Errorf("healthy request = %+v, want id 9", req)
		}
	case <-ctx.Done():
		t.Fatal("the healthy MuxConn stopped routing after the other one panicked")
	}

	requireReaderContained(t, taskRPCRoute, before)
}
