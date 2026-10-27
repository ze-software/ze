// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- panic containment in the receive and advertisement tasks
// Related: backend_linux.go -- linuxInstance.receive, the boundary under test

//go:build linux

package transport

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/taskguard"
	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
)

// receiveRig is one virtual router's socket reader over an AF_UNIX datagram
// pair: the reader owns rx, the test writes into peer.
type receiveRig struct {
	li      *linuxInstance
	peer    int
	items   chan RxItem
	metrics *atomic.Pointer[transportMetrics]
}

// newReceiveRig starts the receive task of one linuxInstance, built the way
// OpenInstance builds it but over a socketpair, with the transport's metrics
// pointer handed in by the caller.
func newReceiveRig(t *testing.T, vrid uint8, mp *atomic.Pointer[transportMetrics]) *receiveRig {
	t.Helper()
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_DGRAM, 0)
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	// The reader wakes on this timeout to see Close, as the real socket does.
	tv := unix.Timeval{Usec: 50_000}
	if err := unix.SetsockoptTimeval(fds[0], unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv); err != nil {
		t.Fatalf("SO_RCVTIMEO: %v", err)
	}
	spec := InstanceSpec{Family: packet.V4, VRID: vrid, Parent: "eth0"}
	items := make(chan RxItem, 4)
	li := &linuxInstance{
		spec:   spec,
		family: packet.V4,
		rxFD:   fds[0],
		txFD:   -1,
		annFD:  -1,
		sink: rxSink{
			key:      spec.key(),
			ch:       items,
			counters: newInstanceCounters(mp, spec.Parent, spec.VRID, spec.Family),
		},
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	go li.readLoop()
	t.Cleanup(func() {
		if err := li.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
		if err := unix.Close(fds[1]); err != nil {
			t.Errorf("close peer: %v", err)
		}
	})
	return &receiveRig{li: li, peer: fds[1], items: items, metrics: mp}
}

func (r *receiveRig) send(t *testing.T, datagram []byte) {
	t.Helper()
	if err := unix.Send(r.peer, datagram, 0); err != nil {
		t.Fatalf("send: %v", err)
	}
}

// expect waits for the reader to deliver the advertisement that follows a
// 20-byte IPv4 header.
func (r *receiveRig) expect(t *testing.T, what string, payload []byte) {
	t.Helper()
	select {
	case item := <-r.items:
		if !bytes.Equal(item.Payload, payload) {
			t.Fatalf("%s: delivered payload %x, want %x", what, item.Payload, payload)
		}
		if item.Key != r.li.spec.key() {
			t.Fatalf("%s: delivered key %v, want %v", what, item.Key, r.li.spec.key())
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("%s: the socket reader delivered nothing", what)
	}
}

// ipv4Datagram is a raw proto-112 datagram as the IPv4 socket returns it: a
// 20-byte header (TTL 255, 192.0.2.9 to 224.0.0.18) and then payload.
func ipv4Datagram(payload []byte) []byte {
	header := []byte{
		0x45, 0, 0, 0, 0, 0, 0, 0, 255, 112, 0, 0,
		192, 0, 2, 9,
		224, 0, 0, 18,
	}
	return append(header, payload...)
}

// TestVRRPReceivePanicContained proves the socket reader contains a Ze defect
// met while it reads one datagram: the process stays up, the same reader
// delivers the next advertisement, the other virtual router's reader never
// stops, and the panic is logged, counted and crash-recorded. The receive task
// changes no virtual router state, so its restart is the reader resuming: the
// FSM is never involved and stays where it was.
// METHOD: two readers over socketpairs, VRID 7 and VRID 8. VRID 7's counters
// read an empty metrics pointer, a state only a Ze defect produces (Transport
// seeds it with the no-op metrics before any instance opens), so the first
// datagram it must count as malformed dereferences nil inside the reader. The
// test then seeds the pointer and sends a well-formed datagram, which must
// arrive; VRID 8 must deliver before and after.
// PREVENTS: a panic in the receive path killing the daemon, or ending the
// reader and leaving the virtual router deaf to its peers.
func TestVRRPReceivePanicContained(t *testing.T) {
	restoreCrashDir := crashlog.SetCrashDirForTest(t.TempDir())
	prior := receiveGuard
	var logs bytes.Buffer
	receiveGuard = taskguard.New("vrrp", slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() {
		receiveGuard = prior
		restoreCrashDir()
	})
	guard := receiveGuard

	seeded := &atomic.Pointer[transportMetrics]{}
	seeded.Store(nopTransportMetrics())
	healthy := newReceiveRig(t, 8, seeded)
	defect := newReceiveRig(t, 7, &atomic.Pointer[transportMetrics]{})

	healthy.send(t, ipv4Datagram([]byte{0x31, 8, 1}))
	healthy.expect(t, "vrid 8 before the panic", []byte{0x31, 8, 1})

	// Three bytes are short of an IPv4 header: the reader counts it malformed.
	defect.send(t, []byte{0x45, 0, 0})
	deadline := time.Now().Add(5 * time.Second)
	for {
		count, _ := guard.Panics(taskReceive)
		if count == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the malformed datagram never reached the guard")
		}
		time.Sleep(time.Millisecond)
	}

	defect.metrics.Store(nopTransportMetrics())
	defect.send(t, ipv4Datagram([]byte{0x31, 7, 2}))
	defect.expect(t, "vrid 7 after the panic", []byte{0x31, 7, 2})
	healthy.send(t, ipv4Datagram([]byte{0x31, 8, 3}))
	healthy.expect(t, "vrid 8 after the panic", []byte{0x31, 8, 3})

	if count, declared := guard.Panics(taskReceive); count != 1 || !declared {
		t.Errorf("Panics(receive) = %d, %v, want 1, true", count, declared)
	}
	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("got %d log lines, want 1: %q", len(lines), logs.String())
	}
	var line map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &line); err != nil {
		t.Fatalf("log line is not JSON: %v: %q", err, lines[0])
	}
	want := map[string]string{
		"level": "ERROR", "component": "vrrp", "task": taskReceive, "unit": "vrid 7 ipv4 on eth0",
	}
	for key, value := range want {
		if line[key] != value {
			t.Errorf("log field %s = %v, want %q", key, line[key], value)
		}
	}
	if panicText, _ := line["panic"].(string); !strings.Contains(panicText, "nil pointer dereference") {
		t.Errorf("log field panic = %q, want the nil dereference", panicText)
	}
	crashes := crashlog.ListCrashes()
	if len(crashes) != 1 {
		t.Fatalf("ListCrashes returned %d records, want 1", len(crashes))
	}
	if crashes[0].Kind != crashlog.KindRecovered {
		t.Errorf("crash record kind %q, want %q", crashes[0].Kind, crashlog.KindRecovered)
	}
}
