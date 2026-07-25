// Design: plan/learned/1124-vrrp-first-hop-redundancy.md -- announcer burst semantics tests

package transport

import (
	"bytes"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
)

// recordingSender captures announcement frames and requested sleep durations so
// the burst count and spacing can be asserted without real time.
type recordingSender struct {
	mu     sync.Mutex
	frames [][]byte
	sleeps []time.Duration
}

func (r *recordingSender) send(frame []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frames = append(r.frames, append([]byte(nil), frame...))
	return nil
}

func (r *recordingSender) sleep(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sleeps = append(r.sleeps, d)
}

func (r *recordingSender) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.frames)
}

func v4Builder(vip netip.Addr, buf []byte) (int, bool) {
	if !vip.Is4() {
		return 0, false
	}
	return BuildGARP(buf, [6]byte{0x00, 0x00, 0x5e, 0x00, 0x01, 0x0a}, vip.As4()), true
}

func TestAnnounceBurstRepeatsAndSpacing(t *testing.T) {
	// VALIDATES: AC-8 -- exactly announceRepeatCount (3) frames per VIP with
	// announceRepeatInterval (100ms) spacing; a VIP of the wrong family builds no
	// frame; the worker drains queued bursts and stop terminates it.
	rs := &recordingSender{}
	var sent int
	a := newAnnouncer(v4Builder, rs.send, func(err error) {
		if err == nil {
			sent++
		}
	})
	a.sleep = rs.sleep

	// runBurst is synchronous: assert exact repeat count and spacing directly.
	a.runBurst([]netip.Addr{netip.MustParseAddr("192.0.2.1")})
	if rs.count() != announceRepeatCount {
		t.Fatalf("frames per VIP = %d, want %d", rs.count(), announceRepeatCount)
	}
	if sent != announceRepeatCount {
		t.Fatalf("reported sends = %d, want %d", sent, announceRepeatCount)
	}
	// announceRepeatCount frames means announceRepeatCount-1 inter-frame sleeps.
	if len(rs.sleeps) != announceRepeatCount-1 {
		t.Fatalf("sleeps = %d, want %d", len(rs.sleeps), announceRepeatCount-1)
	}
	for _, d := range rs.sleeps {
		if d != announceRepeatInterval {
			t.Fatalf("spacing = %v, want %v", d, announceRepeatInterval)
		}
	}

	// Two VIPs -> 2*announceRepeatCount frames.
	rs2 := &recordingSender{}
	a2 := newAnnouncer(v4Builder, rs2.send, func(error) {})
	a2.sleep = rs2.sleep
	a2.runBurst([]netip.Addr{netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("192.0.2.2")})
	if rs2.count() != 2*announceRepeatCount {
		t.Fatalf("2-VIP frames = %d, want %d", rs2.count(), 2*announceRepeatCount)
	}

	// Zero VIPs -> zero frames, no error.
	rs3 := &recordingSender{}
	a3 := newAnnouncer(v4Builder, rs3.send, func(error) {})
	a3.sleep = rs3.sleep
	a3.runBurst(nil)
	if rs3.count() != 0 {
		t.Fatalf("zero-VIP frames = %d, want 0", rs3.count())
	}

	// Wrong-family VIP builds no frame.
	rs4 := &recordingSender{}
	a4 := newAnnouncer(v4Builder, rs4.send, func(error) {})
	a4.sleep = rs4.sleep
	a4.runBurst([]netip.Addr{netip.MustParseAddr("2001:db8::1")})
	if rs4.count() != 0 {
		t.Fatalf("wrong-family frames = %d, want 0", rs4.count())
	}
}

func TestAnnounceWorkerDrainsAndStops(t *testing.T) {
	// VALIDATES: the worker goroutine drains an enqueued burst and close()
	// terminates it (goroutine-lifecycle: one long-lived announcer per instance).
	rs := &recordingSender{}
	a := newAnnouncer(v4Builder, rs.send, func(error) {})
	a.sleep = func(time.Duration) {} // no real spacing in the worker test
	a.start()
	a.enqueue([]netip.Addr{netip.MustParseAddr("192.0.2.1")})

	deadline := time.After(2 * time.Second)
	for rs.count() < announceRepeatCount {
		select {
		case <-deadline:
			t.Fatalf("worker did not drain burst: %d frames", rs.count())
		default:
			time.Sleep(time.Millisecond)
		}
	}
	a.close() // must return (worker terminated)
}

func TestAnnounceMasterBurstWiring(t *testing.T) {
	// VALIDATES: Wiring / AC-8 -- AnnounceMaster on an open instance reaches the
	// announce sender (fake handle records frames); each frame is a GARP.
	withParentAddrs(t, []iface.AddrInfo{{Address: "192.0.2.251", Family: "ipv4"}})
	fb := &fakeBackend{}
	tr := New(fb)
	key, err := tr.OpenInstance(v4Spec())
	if err != nil {
		t.Fatalf("OpenInstance: %v", err)
	}
	h := fb.last()

	tr.AnnounceMaster(key, []netip.Addr{netip.MustParseAddr("192.0.2.1")})

	deadline := time.After(2 * time.Second)
	for h.announceCount() < announceRepeatCount {
		select {
		case <-deadline:
			t.Fatalf("announce not delivered: %d frames", h.announceCount())
		default:
			time.Sleep(time.Millisecond)
		}
	}
	h.mu.Lock()
	frame := append([]byte(nil), h.announces[0]...)
	h.mu.Unlock()
	if len(frame) != GARPFrameLen {
		t.Fatalf("announce frame len = %d, want %d", len(frame), GARPFrameLen)
	}
	if !bytes.Equal(frame[0:6], []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}) {
		t.Fatalf("announce frame dst not broadcast: % x", frame[0:6])
	}
	tr.Close()
}
