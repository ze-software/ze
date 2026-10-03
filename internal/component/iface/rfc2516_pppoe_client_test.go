// VALIDATES: RFC 2516 Section 7 on the Host side: when a PPP session ends,
// the client goes back through Discovery (a fresh Dial) for the next one,
// after the old session's Cleanup has sent its PADT.
// PREVENTS: a client that keeps or reuses a dead SESSION_ID for a second PPP
// session instead of returning to the PPPoE Discovery stage.

package iface

import (
	"log/slog"
	"net/netip"
	"sync"
	"testing"
	"time"
)

// redialRecorder is a PPPoEDialer that ends its first session on its own
// and records what the client looked like when the second Dial arrived.
type redialRecorder struct {
	mu            sync.Mutex
	calls         int
	firstDone     chan struct{}
	firstCleaned  bool
	cleanedBefore bool // first session's Cleanup ran before the second Dial
	secondDial    chan struct{}
	client        *PPPoEClient
	sidAtSecond   uint16
	stateAtSecond string
}

func (d *redialRecorder) Dial(_ PPPoEClientConfig, _ <-chan struct{}, _ *slog.Logger) (PPPoESession, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls++
	switch d.calls {
	case 1:
		return PPPoESession{
			SessionID: 42,
			NegMTU:    1492,
			LocalIP:   netip.MustParseAddr("10.0.0.42"),
			PeerIP:    netip.MustParseAddr("10.0.0.1"),
			Done:      d.firstDone,
			Cleanup: func() {
				d.mu.Lock()
				d.firstCleaned = true
				d.mu.Unlock()
			},
		}, nil
	case 2:
		d.cleanedBefore = d.firstCleaned
		st := d.client.status()
		d.sidAtSecond = st.SessionID
		d.stateAtSecond = st.State
		close(d.secondDial)
	}
	return PPPoESession{}, errStopped
}

// RFC requirement: RFC2516-7-4 positive — after the first PPP session's Done fires, the client calls Dial again, which is the PPPoE Discovery stage (PADI onwards), before any second session exists.
// RFC requirement: RFC2516-7-4 negative — the second Dial finds the first session already cleaned up (its PADT sent) and the client holding SESSION_ID 0 in state "discovery": the old SESSION_ID is never carried into the next session.
func TestRFC2516HostReturnsToDiscoveryForTheNextSession(t *testing.T) {
	d := &redialRecorder{
		firstDone:  make(chan struct{}),
		secondDial: make(chan struct{}),
	}
	client := NewPPPoEClient(PPPoEClientConfig{Name: "pppoe0", SourceInterface: "eth0"}, d, nil, slog.Default())
	d.client = client
	client.Start()
	defer client.Stop()

	// End the first PPP session: this is "LCP terminates" as the client sees it.
	close(d.firstDone)

	// The client reconnects after ReconnectDelay(1), one second.
	select {
	case <-d.secondDial:
	case <-time.After(5 * time.Second):
		t.Fatal("the client never returned to Discovery (no second Dial) after the first session ended")
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.cleanedBefore {
		t.Error("the second Dial ran before the first session's Cleanup sent its PADT")
	}
	if d.sidAtSecond != 0 {
		t.Errorf("client held SESSION_ID 0x%04x while dialing again, want 0", d.sidAtSecond)
	}
	if d.stateAtSecond != "discovery" {
		t.Errorf("client state while dialing again = %q, want \"discovery\"", d.stateAtSecond)
	}
}
