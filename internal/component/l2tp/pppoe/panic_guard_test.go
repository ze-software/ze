//go:build linux

package pppoe

import (
	"bytes"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/crashlog"
)

// guardLogBuffer is the guard's log sink: the discovery reader writes it while
// the test reads it, so every access holds the lock.
type guardLogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *guardLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *guardLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// swapPPPoEPanics replaces the PPPoE panic boundary with one that logs to the
// returned buffer, and points the crash records at a temporary directory, until
// the test ends.
func swapPPPoEPanics(t *testing.T) (*pppoePanicTasks, *guardLogBuffer) {
	t.Helper()
	guardLog := &guardLogBuffer{}
	tasks := newPPPoEPanicTasks(slog.New(slog.NewTextHandler(guardLog, nil)))
	saved := pppoePanics
	pppoePanics = func() *pppoePanicTasks { return tasks }
	t.Cleanup(func() { pppoePanics = saved })
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))
	return tasks, guardLog
}

// panicHost is the PADR source whose admission the injected panic breaks.
var panicHost = [EthALen]byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x0b}

// startPanicReader runs the real discoveryReader over one end of an AF_UNIX
// datagram socketpair, serving srv as ifindex 0 (an AF_UNIX source address is
// not link-layer, so readDiscoveryFrame answers ifindex 0). It answers the
// other end, which the test writes discovery frames into.
func startPanicReader(t *testing.T, srv *InterfaceServer) int {
	t.Helper()
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_DGRAM, 0)
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	// The receive timeout lets the reader see the stop channel, as the
	// production discovery socket's 100ms timeout does.
	timeout := unix.NsecToTimeval((50 * time.Millisecond).Nanoseconds())
	if err := unix.SetsockoptTimeval(fds[0], unix.SOL_SOCKET, unix.SO_RCVTIMEO, &timeout); err != nil {
		t.Fatalf("SO_RCVTIMEO: %v", err)
	}
	sub := newDiscoveryReaderTestSubsystem(fds[0])
	sub.servers = map[int]*InterfaceServer{0: srv}
	go sub.discoveryReader()
	t.Cleanup(func() {
		close(sub.stop)
		<-sub.readDone
		unix.Close(fds[0]) //nolint:errcheck // test teardown
		unix.Close(fds[1]) //nolint:errcheck // test teardown
	})
	return fds[1]
}

// sendPADR writes the PADR host sends after a PADO from srv into the reader.
func sendPADR(t *testing.T, fd int, srv *InterfaceServer, host [EthALen]byte) {
	t.Helper()
	pkt := padrFrom(srv, host, "", nil, nil)
	var buf [EthMaxLen]byte
	b := NewBuilder(buf[:], host, srv.hwAddr, CodePADR, 0)
	for i := range pkt.Tags {
		b.AddTagCopy(&pkt.Tags[i])
	}
	if _, err := unix.Write(fd, b.Finish()); err != nil {
		t.Fatalf("write PADR: %v", err)
	}
}

// awaitFrame reads frames the server sent until one with code reaches host,
// and answers every frame read on the way, that one included.
func awaitFrame(t *testing.T, sent <-chan *Packet, code byte, host [EthALen]byte) []*Packet {
	t.Helper()
	var seen []*Packet
	deadline := time.After(5 * time.Second)
	for {
		select {
		case pkt := <-sent:
			seen = append(seen, pkt)
			if pkt.Code == code && pkt.DstMAC == host {
				return seen
			}
		case <-deadline:
			t.Fatalf("no frame with code 0x%02x reached %s; frames sent: %d", code, net.HardwareAddr(host[:]), len(seen))
			return nil
		}
	}
}

// TestPPPoEReadPanicContained
// VALIDATES: a panic while the discovery reader admits one host's PADR is
// contained at dispatchDiscovery: the reader keeps serving, the session that
// PADR was admitting ends (no PADT before the PADS, since the host holds no
// session; a PADT after it, RFC 2516 Section 5.5), a session admitted earlier
// is untouched, and the panic is logged, counted and crash-recorded.
// METHOD: the real discoveryReader over a socketpair; the injected panic fires
// in the server's own seams, pppoeCreateFn (before the PADS) or sendFrameFn on
// the PADS itself (after the PADS is marked sent).
// PREVENTS: one malformed admission ending discovery for every access
// interface, and a host left believing in a session Ze abandoned.
func TestPPPoEReadPanicContained(t *testing.T) {
	cases := []struct {
		name string
		// panicCreate breaks the victim's admission in pppoeCreateFn, before
		// the PADS; otherwise the panic fires as the victim's PADS is sent.
		panicCreate bool
		wantPADT    bool
	}{
		{name: "before PADS", panicCreate: true, wantPADT: false},
		{name: "after PADS", panicCreate: false, wantPADT: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tasks, guardLog := swapPPPoEPanics(t)

			// Only the reader goroutine sends, and the channel hands each
			// parsed frame to the test.
			sent := make(chan *Packet, 64)
			srv := newRecordingServer(new([][]byte), nil, 8)
			srv.acName = "ze-ac"
			srv.pppDriver = ppp.NewProductionDriver(slog.Default(), admissionBackend{})
			srv.devPPPSetupFn = func(int) (int, int, int, error) { return -1, -1, 7, nil }
			srv.pppoeCreateFn = func(_ string, _ uint16, remoteMAC [EthALen]byte) (int, error) {
				if tc.panicCreate && remoteMAC == panicHost {
					panic("injected test panic in pppoeCreateFn")
				}
				return -1, nil
			}
			srv.sendFrameFn = func(frame []byte) {
				// The server reuses its frame buffer, so the test keeps a copy.
				pkt, err := ParseDiscovery(append([]byte(nil), frame...))
				if err != nil {
					t.Errorf("the server sent an unparsable frame: %v", err)
					return
				}
				if !tc.panicCreate && pkt.Code == CodePADS && pkt.DstMAC == panicHost {
					panic("injected test panic sending the PADS")
				}
				sent <- &pkt
			}
			fd := startPanicReader(t, srv)

			earlier := [EthALen]byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x0a}
			later := [EthALen]byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x0c}

			sendPADR(t, fd, srv, earlier)
			earlierPADS := awaitFrame(t, sent, CodePADS, earlier)
			earlierSID := earlierPADS[len(earlierPADS)-1].SID

			sendPADR(t, fd, srv, panicHost)
			sendPADR(t, fd, srv, later)
			// The reader handles frames in order, so the later host's PADS
			// proves the reader survived the victim's frame, and every frame
			// the victim drew was sent before it.
			frames := awaitFrame(t, sent, CodePADS, later)

			var toVictim []*Packet
			for _, pkt := range frames {
				if pkt.DstMAC == earlier {
					t.Errorf("the earlier host was sent code 0x%02x after the panic", pkt.Code)
				}
				if pkt.DstMAC == panicHost {
					toVictim = append(toVictim, pkt)
				}
			}
			if tc.wantPADT {
				if len(toVictim) != 1 || toVictim[0].Code != CodePADT || toVictim[0].SID == 0 {
					t.Fatalf("after the PADS the victim was sent %d frame(s), want one PADT with its SESSION_ID", len(toVictim))
				}
			} else if len(toVictim) != 0 {
				t.Fatalf("before the PADS the victim was sent %d frame(s), want none", len(toVictim))
			}

			if left := srv.sessions.sessionsByMAC(net.HardwareAddr(panicHost[:])); len(left) != 0 {
				t.Fatalf("the victim's half-admitted session is still in the table (SID 0x%04x)", left[0].SID)
			}
			if srv.sessions.Lookup(earlierSID) == nil {
				t.Fatalf("the earlier host's session 0x%04x left the table", earlierSID)
			}
			if got := srv.sessions.Count(); got != 2 {
				t.Fatalf("session table holds %d session(s), want the earlier and later hosts' 2", got)
			}

			count, ok := tasks.guard.Panics("discovery")
			if !ok || count != 1 {
				t.Fatalf("guard count for pppoe/discovery = %d (declared %v), want 1", count, ok)
			}
			line := guardLog.String()
			for _, want := range []string{"component=pppoe", "task=discovery", net.HardwareAddr(panicHost[:]).String(), `message-type="discovery code 25"`} {
				if !strings.Contains(line, want) {
					t.Fatalf("guard log line lacks %q:\n%s", want, line)
				}
			}
			crashes := crashlog.ListCrashes()
			if len(crashes) != 1 || crashes[0].Kind != crashlog.KindRecovered {
				t.Fatalf("crash records = %+v, want one of kind %q", crashes, crashlog.KindRecovered)
			}
		})
	}
}
