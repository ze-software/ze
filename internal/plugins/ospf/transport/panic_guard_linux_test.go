package transport

import (
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/crashlog"
)

// socketpairBackend opens each interface as a real linuxInterface whose receive
// socket is one end of an AF_UNIX datagram socketpair, so readLoop runs unchanged
// and the test writes the "wire" into the other end. panicOn names the
// interface whose recordDrop seam panics on the first malformed datagram.
type socketpairBackend struct {
	mu      sync.Mutex
	panicOn string
	opened  map[string]int
	wire    map[string]int
	nextIdx int
	readers sync.WaitGroup
}

func (b *socketpairBackend) OpenInterface(name string, recordDrop dropRecorder) (InterfaceHandle, error) {
	pair, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_DGRAM, 0)
	if err != nil {
		return nil, err
	}
	tv := unix.NsecToTimeval(int64(50 * time.Millisecond))
	if err := unix.SetsockoptTimeval(pair[0], unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv); err != nil {
		return nil, err
	}
	txFD, err := unix.Socket(unix.AF_UNIX, unix.SOCK_DGRAM, 0)
	if err != nil {
		return nil, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextIdx++
	b.opened[name]++
	b.wire[name] = pair[1]
	if name == b.panicOn {
		recordDrop = func(string) { panic("test: recordDrop seam panics on " + name) }
	}
	li := &linuxInterface{rxFD: pair[0], txFD: txFD, osName: name, ifindex: b.nextIdx,
		local: netip.MustParseAddr("192.0.2.1").As4(), recvCh: make(chan RawPacket, 64),
		stop: make(chan struct{}), recordDrop: recordDrop}
	b.readers.Go(li.readLoop) // as linuxBackend.OpenInterface starts it, joined by the test
	return li, nil
}

func (b *socketpairBackend) send(t *testing.T, name string, datagram []byte) {
	t.Helper()
	b.mu.Lock()
	fd := b.wire[name]
	b.mu.Unlock()
	if _, err := unix.Write(fd, datagram); err != nil {
		t.Fatalf("write to %s: %v", name, err)
	}
}

// panicBaseline is the real boundary's count for task before the test, and a
// crash directory of the test's own. The boundary is not swapped: other tests
// leave transports whose loops read it, and the guard's log line is the
// taskguard package's own tested contract (TestGuardRecordsPanic).
func panicBaseline(t *testing.T, task string) uint64 {
	t.Helper()
	before, declared := receivePanics().guard.Panics(task)
	if !declared {
		t.Fatalf("task %q is not declared on the ospf transport boundary", task)
	}
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))
	return before
}

// requireContained asserts AC-2 for one contained panic of task: counted once,
// and one recovered crash record naming the component, the task and the
// interface.
func requireContained(t *testing.T, before uint64, task, unit string) {
	t.Helper()
	if n, _ := receivePanics().guard.Panics(task); n != before+1 {
		t.Fatalf("Panics(%q) = %d, want %d: the panic was not counted", task, n, before+1)
	}
	crashes := crashlog.ListCrashes()
	if len(crashes) != 1 || crashes[0].Kind != crashlog.KindRecovered {
		t.Fatalf("crash records = %+v, want one %q record", crashes, crashlog.KindRecovered)
	}
	want := "ospf " + task + ", unit interface " + unit
	if record := crashlog.ReadCrash(crashes[0].Name); !strings.Contains(record, want) {
		t.Fatalf("crash record lacks %q:\n%s", want, record)
	}
}

func waitDown(t *testing.T, downs <-chan string, want string) {
	t.Helper()
	select {
	case got := <-downs:
		if got != want {
			t.Fatalf("interface down = %q, want %q", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("interface %s never went down after its receive path ended", want)
	}
}

// TestOSPFReadPanicContained proves the OSPFv2 receive boundary (AC-7, AC-2): a
// panic in eth0's socket reader, reached through a malformed datagram on the
// real readLoop and its recordDrop seam, is logged, counted and recorded; eth0
// leaves OSPF through the link-down path and the rescan reopens it; eth1 keeps
// delivering packets throughout. Method: two socketpair interfaces behind the
// real Transport, eth0's drop recorder panics.
func TestOSPFReadPanicContained(t *testing.T) {
	before := panicBaseline(t, "interface-read")
	backend := &socketpairBackend{panicOn: "eth0", opened: map[string]int{}, wire: map[string]int{}}
	tr := New(backend)
	downs := make(chan string, 4)
	tr.OnInterfaceDown(func(_ int, name string) { downs <- name })
	for _, name := range []string{"eth0", "eth1"} {
		tr.EnableInterface(name)
		if err := tr.HandleLinkUp(name); err != nil {
			t.Fatalf("HandleLinkUp(%s): %v", name, err)
		}
	}
	t.Cleanup(func() { // before the crash directory's restore: no reader outlives the test
		tr.Close()
		backend.readers.Wait()
	})

	backend.send(t, "eth0", []byte{0x45}) // malformed: recordDrop panics
	waitDown(t, downs, "eth0")
	requireContained(t, before, "interface-read", "eth0")
	if tr.InterfaceOpen("eth0") {
		t.Fatal("eth0 still open after its reader ended: nobody reads its socket")
	}

	backend.send(t, "eth1", ipv4OSPFDatagram(netip.MustParseAddr("192.0.2.1")))
	select {
	case pkt := <-tr.Receive():
		if pkt.IfIndex != 2 {
			t.Fatalf("delivered packet from ifindex %d, want eth1's 2", pkt.IfIndex)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("eth1 stopped delivering after eth0's reader panicked")
	}

	tr.RescanInterfaces()
	if !tr.InterfaceOpen("eth0") {
		t.Fatal("the rescan did not reopen eth0")
	}
	backend.mu.Lock()
	reopened := backend.opened["eth0"]
	backend.mu.Unlock()
	if reopened != 2 {
		t.Fatalf("eth0 opened %d times, want 2 (once, then the rescan)", reopened)
	}
}

// panicRecvHandle is an InterfaceHandle whose Recv panics, so rxLoop's own
// boundary is reached at its first statement.
type panicRecvHandle struct{ closed bool }

func (*panicRecvHandle) IfIndex() int                  { return 9 }
func (*panicRecvHandle) Send(netip.Addr, []byte) error { return nil }
func (*panicRecvHandle) Recv() <-chan RawPacket        { panic("test: Recv seam panics") }
func (*panicRecvHandle) JoinAllSPFRouters() error      { return nil }
func (*panicRecvHandle) JoinAllDRouters() error        { return nil }
func (*panicRecvHandle) LeaveAllDRouters() error       { return nil }
func (h *panicRecvHandle) Close() error                { h.closed = true; return nil }

type panicRecvBackend struct{ handle *panicRecvHandle }

func (b *panicRecvBackend) OpenInterface(string, dropRecorder) (InterfaceHandle, error) {
	return b.handle, nil
}

// TestOSPFFanInPanicContained proves rxLoop's boundary: its panic is contained,
// and the interface is taken down rather than left open with no fan-in.
func TestOSPFFanInPanicContained(t *testing.T) {
	before := panicBaseline(t, "interface-fan-in")
	backend := &panicRecvBackend{handle: &panicRecvHandle{}}
	tr := New(backend)
	downs := make(chan string, 1)
	tr.OnInterfaceDown(func(_ int, name string) { downs <- name })
	tr.EnableInterface("eth0")
	if err := tr.HandleLinkUp("eth0"); err != nil {
		t.Fatalf("HandleLinkUp: %v", err)
	}
	t.Cleanup(tr.Close)

	waitDown(t, downs, "eth0")
	requireContained(t, before, "interface-fan-in", "eth0")
	if !backend.handle.closed {
		t.Fatal("the link-down path did not close the handle")
	}
}
