// Design: docs/architecture/ospf/ospfv3-3-ipv6-transport.md -- panic containment in the receive tasks
//
// VALIDATES: a panic in one OSPFv3 interface's socket reader or fan-in is contained by
// the transport's guard (logged, counted in ze_task_panics_total{component="ospfv3"},
// crash-recorded), the interface is taken down through its link-down path, and every
// other interface keeps delivering.
package transport

import (
	"net"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/ipv6"

	"github.com/ze-software/ze/internal/core/crashlog"
)

// panicBaseline reads task's count on the REAL guard and points crash records at a
// fresh directory, so requireContained can assert the delta.
func panicBaseline(t *testing.T, task string) uint64 {
	t.Helper()
	before, declared := receivePanics().guard.Panics(task)
	if !declared {
		t.Fatalf("task %q is not declared on the ospfv3 transport boundary", task)
	}
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))
	return before
}

// requireContained asserts one contained panic of task: counted once, and one recovered
// crash record naming the component, the task and the interface.
func requireContained(t *testing.T, before uint64, task, unit string) {
	t.Helper()
	if n, _ := receivePanics().guard.Panics(task); n != before+1 {
		t.Fatalf("Panics(%q) = %d, want %d: the panic was not counted", task, n, before+1)
	}
	crashes := crashlog.ListCrashes()
	if len(crashes) != 1 || crashes[0].Kind != crashlog.KindRecovered {
		t.Fatalf("crash records = %+v, want one %q record", crashes, crashlog.KindRecovered)
	}
	want := "ospfv3 " + task + ", unit interface " + unit
	if record := crashlog.ReadCrash(crashes[0].Name); !strings.Contains(record, want) {
		t.Fatalf("crash record lacks %q:\n%s", want, record)
	}
}

// pipePacketConn is a net.PacketConn that x/net/ipv6 cannot reach a socket through,
// so ipv6.PacketConn.ReadFrom answers a plain (non-net.Error) error and readLoop takes
// its recordDrop(dropRecvErr) branch: the seam that panics.
type pipePacketConn struct{ net.Conn }

func (pipePacketConn) ReadFrom([]byte) (int, net.Addr, error) { return 0, nil, net.ErrClosed }
func (pipePacketConn) WriteTo([]byte, net.Addr) (int, error)  { return 0, net.ErrClosed }
func (p pipePacketConn) SetReadDeadline(time.Time) error      { return nil }
func (p pipePacketConn) SetDeadline(time.Time) error          { return nil }
func (p pipePacketConn) SetWriteDeadline(time.Time) error     { return nil }
func (p pipePacketConn) LocalAddr() net.Addr                  { return &net.IPAddr{} }

// TestOSPFv3ReadPanicContained runs the real readLoop of one interface whose recordDrop
// seam panics on the first read error: the guard contains it, the reader closes its
// receive channel (which rxLoop turns into the link-down), and the crash record names
// the interface.
func TestOSPFv3ReadPanicContained(t *testing.T) {
	before := panicBaseline(t, "interface-read")
	local, remote := net.Pipe()
	t.Cleanup(func() { _ = local.Close(); _ = remote.Close() })
	conn := pipePacketConn{Conn: local}
	li := &linuxInterface{
		conn:       conn,
		pc:         ipv6.NewPacketConn(conn),
		ifi:        &net.Interface{Index: 7, Name: "eth0"},
		recvCh:     make(chan RawPacket, 1),
		stop:       make(chan struct{}),
		recordDrop: func(string) { panic("test: recordDrop seam panics on eth0") },
	}

	var reader sync.WaitGroup
	reader.Go(li.readLoop)
	reader.Wait()

	if _, open := <-li.recvCh; open {
		t.Fatal("readLoop returned without closing its receive channel: rxLoop would never take the interface down")
	}
	requireContained(t, before, "interface-read", "eth0")
}

// panicRecvHandle is an InterfaceHandle whose Recv panics, so rxLoop's own boundary is
// what contains it.
type panicRecvHandle struct{ closed bool }

func (*panicRecvHandle) IfIndex() int                              { return 9 }
func (*panicRecvHandle) LinkLocalSource() netip.Addr               { return netip.MustParseAddr("fe80::9") }
func (*panicRecvHandle) Send(netip.Addr, netip.Addr, []byte) error { return nil }
func (*panicRecvHandle) Recv() <-chan RawPacket                    { panic("test: Recv seam panics") }
func (*panicRecvHandle) JoinAllSPFRouters() error                  { return nil }
func (*panicRecvHandle) JoinAllDRouters() error                    { return nil }
func (*panicRecvHandle) LeaveAllDRouters() error                   { return nil }
func (h *panicRecvHandle) Close() error                            { h.closed = true; return nil }

// quietHandle is a healthy interface: its Recv delivers what the test sends.
type quietHandle struct {
	recv chan RawPacket
	once sync.Once
}

func (*quietHandle) IfIndex() int                              { return 10 }
func (*quietHandle) LinkLocalSource() netip.Addr               { return netip.MustParseAddr("fe80::a") }
func (*quietHandle) Send(netip.Addr, netip.Addr, []byte) error { return nil }
func (h *quietHandle) Recv() <-chan RawPacket                  { return h.recv }
func (*quietHandle) JoinAllSPFRouters() error                  { return nil }
func (*quietHandle) JoinAllDRouters() error                    { return nil }
func (*quietHandle) LeaveAllDRouters() error                   { return nil }
func (h *quietHandle) Close() error                            { h.once.Do(func() { close(h.recv) }); return nil }

type panicRecvBackend struct {
	bad  *panicRecvHandle
	good *quietHandle
}

func (b *panicRecvBackend) OpenInterface(name string, _ DropRecorder) (InterfaceHandle, error) {
	if name == "eth0" {
		return b.bad, nil
	}
	return b.good, nil
}

// TestOSPFv3FanInPanicContained proves rxLoop's boundary: eth0's panic is contained and
// eth0 is taken down rather than left open with no fan-in, while eth1 keeps delivering.
func TestOSPFv3FanInPanicContained(t *testing.T) {
	before := panicBaseline(t, "interface-fan-in")
	backend := &panicRecvBackend{bad: &panicRecvHandle{}, good: &quietHandle{recv: make(chan RawPacket, 1)}}
	tr := New(backend)
	downs := make(chan string, 2)
	tr.OnInterfaceDown(func(_ int, name string) { downs <- name })
	tr.EnableInterface("eth1")
	if err := tr.HandleLinkUp("eth1"); err != nil {
		t.Fatalf("HandleLinkUp(eth1): %v", err)
	}
	tr.EnableInterface("eth0")
	if err := tr.HandleLinkUp("eth0"); err != nil {
		t.Fatalf("HandleLinkUp(eth0): %v", err)
	}
	t.Cleanup(tr.Close)

	select {
	case got := <-downs:
		if got != "eth0" {
			t.Fatalf("interface down = %q, want eth0", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("eth0 never went down after its fan-in panicked")
	}
	requireContained(t, before, "interface-fan-in", "eth0")
	if !backend.bad.closed {
		t.Fatal("the link-down path did not close eth0's handle")
	}

	payload := make([]byte, 16)
	payload[0] = 3 // OSPFv3 version; Instance ID 0 matches the default.
	backend.good.recv <- RawPacket{IfIndex: 10, Payload: payload}
	select {
	case <-tr.Receive():
	case <-time.After(5 * time.Second):
		t.Fatal("eth1 stopped delivering after eth0's fan-in panic")
	}
}
