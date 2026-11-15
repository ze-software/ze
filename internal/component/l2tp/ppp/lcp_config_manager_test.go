// Design: docs/research/l2tpv2-ze-integration.md -- configuration admission to the driver

package ppp

import (
	"testing"
	"time"
)

// TestPPPManagerAppliesLCPRestartConfig observes the admitted live session after
// the proxy-up event, so a missing copy in spawnSession cannot pass unnoticed.
func TestPPPManagerAppliesLCPRestartConfig(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 19201)
	t.Cleanup(func() { closeConn(pair.peerEnd) })
	ops, _, _ := newFakeOps()
	d := makeTestDriver(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Stop)
	c, err := NewLCPConfig(500*time.Millisecond, 3, 4, 6)
	if err != nil {
		t.Fatal(err)
	}
	stream := buildOptionStream([]LCPOption{mruOpt(1500), magicOpt(0xcafebabe)})
	d.SessionsIn() <- StartSession{LCP: c, TunnelID: 19, SessionID: 2, ChanFD: 19201, UnitFD: 19202, UnitNum: 7, LNSMode: true, MaxMRU: 1500, DisableIPCP: true, DisableIPv6CP: true, ProxyLCPInitialRecv: stream, ProxyLCPLastSent: stream, ProxyLCPLastRecv: stream}
	drainTwoEvents(t, d.EventsOut(), time.Second)
	d.mu.Lock()
	s := d.sessions[sessionKey{19, 2}]
	if s == nil {
		d.mu.Unlock()
		t.Fatal("admitted session missing")
	}
	got := s.lcpConfig
	d.mu.Unlock()
	if got != c {
		t.Fatalf("session LCP = %+v, want %+v", got, c)
	}
}

// TestPPPManagerRefusesMissingLCPConfig proves a direct caller cannot omit the
// validated snapshot and receive a session with zero timers or retry limits.
func TestPPPManagerRefusesMissingLCPConfig(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 19301)
	t.Cleanup(func() { closeConn(pair.peerEnd) })
	ops, _, _ := newFakeOps()
	d := makeTestDriver(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Stop)
	d.SessionsIn() <- StartSession{TunnelID: 19, SessionID: 3, ChanFD: 19301, UnitFD: 19302}
	select {
	case event := <-d.EventsOut():
		if _, ok := event.(EventSessionRejected); !ok {
			t.Fatalf("event = %T, want rejection", event)
		}
	case <-time.After(time.Second):
		t.Fatal("invalid configuration was not rejected")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.sessions) != 0 {
		t.Fatalf("invalid session admitted: %d", len(d.sessions))
	}
}
