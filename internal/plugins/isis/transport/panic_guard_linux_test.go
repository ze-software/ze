// Design: docs/architecture/isis/isis-3-l2-transport.md -- panic containment in the receive tasks
//
// VALIDATES: a panic in one IS-IS circuit's socket reader or fan-in is contained by
// the transport's guard (logged, counted in ze_task_panics_total{component="isis"},
// crash-recorded), the circuit is taken down through its link-down path, and every
// other circuit keeps delivering.
package transport

import (
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/crashlog"
)

// panicBaseline reads task's count on the REAL guard and points crash records at a
// fresh directory, so requireContained can assert the delta.
func panicBaseline(t *testing.T, task string) uint64 {
	t.Helper()
	before, declared := receivePanics().guard.Panics(task)
	if !declared {
		t.Fatalf("task %q is not declared on the isis transport boundary", task)
	}
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))
	return before
}

// requireContained asserts one contained panic of task: counted once, and one recovered
// crash record naming the component, the task and the circuit.
func requireContained(t *testing.T, before uint64, task, unit string) {
	t.Helper()
	if n, _ := receivePanics().guard.Panics(task); n != before+1 {
		t.Fatalf("Panics(%q) = %d, want %d: the panic was not counted", task, n, before+1)
	}
	crashes := crashlog.ListCrashes()
	if len(crashes) != 1 || crashes[0].Kind != crashlog.KindRecovered {
		t.Fatalf("crash records = %+v, want one %q record", crashes, crashlog.KindRecovered)
	}
	want := "isis " + task + ", unit interface " + unit
	if record := crashlog.ReadCrash(crashes[0].Name); !strings.Contains(record, want) {
		t.Fatalf("crash record lacks %q:\n%s", want, record)
	}
}

// TestISISReadPanicContained runs the real readLoop of one circuit over an AF_UNIX
// datagram socketpair holding one valid IS-IS frame. Its receive channel is closed
// before the reader starts, so queueing the frame panics (send on a closed channel)
// and so does the reader's own close on exit: the guard, deferred outermost, contains
// both, the reader returns, and the crash record names the circuit. The pre-closed
// channel is the test's only seam; production code carries no hook.
func TestISISReadPanicContained(t *testing.T) {
	before := panicBaseline(t, "interface-read")
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_DGRAM, 0)
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	t.Cleanup(func() { _ = unix.Close(fds[0]); _ = unix.Close(fds[1]) })
	// A timeout keeps a reader that never sees the frame from blocking forever.
	tv := unix.NsecToTimeval((200 * time.Millisecond).Nanoseconds())
	if err := unix.SetsockoptTimeval(fds[0], unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv); err != nil {
		t.Fatalf("SO_RCVTIMEO: %v", err)
	}
	var frame [1514]byte
	n, err := BuildFrame(frame[:], AllL2ISs, [MACLen]byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x09}, samplePDU())
	if err != nil {
		t.Fatalf("BuildFrame: %v", err)
	}
	if _, err := unix.Write(fds[1], frame[:n]); err != nil {
		t.Fatalf("write frame: %v", err)
	}

	recvCh := make(chan RawFrame, 1)
	close(recvCh)
	c := &linuxCircuit{name: "eth0", fd: fds[0], ifindex: 7, mtu: 1500, recvCh: recvCh, stop: make(chan struct{})}

	var reader sync.WaitGroup
	reader.Go(c.readLoop)
	ended := make(chan struct{})
	go func() { reader.Wait(); close(ended) }()
	select {
	case <-ended:
	case <-time.After(5 * time.Second):
		close(c.stop)
		t.Fatal("readLoop never reached the frame: the reader did not panic as the test intended")
	}
	requireContained(t, before, "interface-read", "eth0")
}

// panicRecvCircuit is a CircuitHandle whose Recv panics, so rxLoop's own boundary is
// what contains it.
type panicRecvCircuit struct {
	mu     sync.Mutex
	closed bool
}

func (*panicRecvCircuit) IfIndex() int                           { return 9 }
func (*panicRecvCircuit) HWAddr() [MACLen]byte                   { return [MACLen]byte{0x02, 0, 0, 0, 0, 9} }
func (*panicRecvCircuit) MTU() int                               { return 1500 }
func (*panicRecvCircuit) Recv() <-chan RawFrame                  { panic("test: Recv seam panics") }
func (*panicRecvCircuit) Send(_, _ [MACLen]byte, _ []byte) error { return nil }

func (c *panicRecvCircuit) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *panicRecvCircuit) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

// panicRecvBackend opens the panicking circuit for eth0 and a healthy fake for any
// other name.
type panicRecvBackend struct {
	bad  *panicRecvCircuit
	good *fakeCircuit
}

func (b *panicRecvBackend) OpenCircuit(name string) (CircuitHandle, error) {
	if name == "eth0" {
		return b.bad, nil
	}
	return b.good, nil
}

// TestISISFanInPanicContained proves rxLoop's boundary: eth0's panic is contained and
// eth0 is taken down rather than left open with no fan-in, while eth1 keeps delivering.
func TestISISFanInPanicContained(t *testing.T) {
	before := panicBaseline(t, "interface-fan-in")
	backend := &panicRecvBackend{bad: &panicRecvCircuit{}, good: newFakeCircuit("eth1", 10, 1500)}
	tr := New(backend)
	downs := make(chan string, 2)
	tr.OnCircuitDown(func(_ int, name string) { downs <- name })
	tr.EnableInterface("eth1", Level2)
	if err := tr.HandleLinkUp("eth1"); err != nil {
		t.Fatalf("HandleLinkUp(eth1): %v", err)
	}
	tr.EnableInterface("eth0", Level2)
	if err := tr.HandleLinkUp("eth0"); err != nil {
		t.Fatalf("HandleLinkUp(eth0): %v", err)
	}
	t.Cleanup(tr.Close)

	select {
	case got := <-downs:
		if got != "eth0" {
			t.Fatalf("circuit down = %q, want eth0", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("eth0 never went down after its fan-in panicked")
	}
	requireContained(t, before, "interface-fan-in", "eth0")
	if !backend.bad.isClosed() {
		t.Fatal("the link-down path did not close eth0's handle")
	}

	backend.good.recvCh <- RawFrame{IfIndex: 10, DstMAC: AllL2ISs, PDU: samplePDU()}
	select {
	case rx := <-tr.Receive():
		if rx.IfIndex != 10 {
			t.Fatalf("delivered ifindex = %d, want 10 (eth1)", rx.IfIndex)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("eth1 stopped delivering after eth0's fan-in panic")
	}
}
