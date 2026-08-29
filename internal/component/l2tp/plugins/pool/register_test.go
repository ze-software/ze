// Design: docs/architecture/l2tp/subscriber-session-model.md -- pool release on session-down
//
// Goal: prove an address taken from the pool comes back when the session that
// took it goes down, on both access types. Method: allocate through the
// registered pool handler, which is the call startPPPoEPoolDrain and the L2TP
// pool drain both make, then publish the teardown on the bus the plugin
// subscribed to in setEventBus. Nothing here calls ipv4Pool.release.
//
// The pool holds ONE address, so a leak is unambiguous: the second allocation
// is refused rather than returning a different address.
//
// The PPPoE payload's shape is not invented here. pppoe.Subsystem.onSessionDown
// is what fills AccessType, PPPoESID and AccessIfIndex, and
// TestPPPoESessionDownPublishesPoolKey (internal/component/l2tp/pppoe) asserts
// it sets exactly those. The L2TP payload comes from
// subscriberBridge.onSessionDown, asserted by TestBridgeSessionDownCarriesPoolKey
// (internal/component/l2tp).

package l2tppool

import (
	"net/netip"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
	"github.com/ze-software/ze/pkg/ze"
)

// releaseBus dispatches to in-process subscribers, which is what the engine
// bus does for a plugin running in process.
type releaseBus struct {
	mu       sync.Mutex
	handlers map[string][]func(any)
}

var _ ze.EventBus = (*releaseBus)(nil)

func newReleaseBus() *releaseBus {
	return &releaseBus{handlers: make(map[string][]func(any))}
}

func (b *releaseBus) Emit(namespace, eventType string, payload any) (int, error) {
	key := namespace + "/" + eventType
	b.mu.Lock()
	src := b.handlers[key]
	hs := make([]func(any), len(src))
	copy(hs, src)
	b.mu.Unlock()
	for _, h := range hs {
		h(payload)
	}
	return 0, nil
}

func (b *releaseBus) Subscribe(namespace, eventType string, handler func(any)) func() {
	key := namespace + "/" + eventType
	b.mu.Lock()
	b.handlers[key] = append(b.handlers[key], handler)
	b.mu.Unlock()
	return func() {}
}

// newSingleAddressPlugin builds a plugin whose default pool holds exactly one
// address, subscribed to the bus the way ConfigureEventBus subscribes it.
func newSingleAddressPlugin(t *testing.T, bus ze.EventBus) *poolPlugin {
	t.Helper()
	p := &poolPlugin{
		pool: newIPv4Pool(
			netip.MustParseAddr("10.0.0.1"),
			netip.MustParseAddr("10.0.0.10"),
			netip.MustParseAddr("10.0.0.10"),
			netip.Addr{}, netip.Addr{},
		),
	}
	p.setEventBus(bus)
	t.Cleanup(func() {
		p.busMu.Lock()
		if p.unsub != nil {
			p.unsub()
		}
		p.busMu.Unlock()
	})
	return p
}

// allocate asks the plugin for an address exactly as the transport pool drains
// do: one ppp.EventIPRequest carrying the PPP driver's session pair.
func allocate(t *testing.T, p *poolPlugin, tunnelID, sessionID uint16) netip.Addr {
	t.Helper()
	resp := p.handle(ppp.EventIPRequest{
		TunnelID:  tunnelID,
		SessionID: sessionID,
		Family:    ppp.AddressFamilyIPv4,
	})
	if !resp.Accept {
		t.Fatalf("pool refused (%d, %d): %s", tunnelID, sessionID, resp.Reason)
	}
	return resp.Peer
}

// VALIDATES: AC-1 -- a PPPoE session's address returns to the pool on teardown
// and the next session gets it.
// PREVENTS: regression to the state where poolPlugin subscribed to
// l2tpevents.SessionDown alone, which PPPoE never emits, so every PPPoE session
// leaked one address until the pool was exhausted and only a restart recovered.
// Delete the Subscribe call in setEventBus and this test reports the address
// never came back.
func TestPoolReleasesPPPoEAddressOnSessionDown(t *testing.T) {
	const ifIndex = 7
	const sid = uint16(42)

	bus := newReleaseBus()
	p := newSingleAddressPlugin(t, bus)

	first := allocate(t, p, uint16(ifIndex), sid)
	if _, allocated, available := p.pool.stats(); allocated != 1 || available != 0 {
		t.Fatalf("after allocate: allocated=%d available=%d, want 1 and 0", allocated, available)
	}

	if _, err := subevents.SessionDown.Emit(bus, &subevents.SessionDownPayload{
		Session: subscriber.Session{
			ID:            "pppoe-7-42",
			AccessType:    subscriber.AccessPPPoE,
			PPPoESID:      sid,
			AccessIfIndex: ifIndex,
		},
		Reason: "peer hangup",
	}); err != nil {
		t.Fatalf("emit session-down: %v", err)
	}

	_, allocated, available := p.pool.stats()
	if allocated != 0 || available != 1 {
		t.Fatalf("PPPoE address never came back: allocated=%d available=%d, want 0 and 1", allocated, available)
	}

	second := allocate(t, p, uint16(ifIndex), sid+1)
	if second != first {
		t.Fatalf("next session got %s, want the released %s", second, first)
	}
}

// VALIDATES: AC-1 -- the L2TP release still works after the migration, so the
// fix adds a transport rather than swapping one for the other.
// PREVENTS: a change that moves the pool onto the subscriber namespace and
// silently drops L2TP, which the bridge re-emits onto that same topic.
func TestPoolReleasesL2TPAddressOnSessionDown(t *testing.T) {
	const tunnelID = uint16(3)
	const sessionID = uint16(8)

	bus := newReleaseBus()
	p := newSingleAddressPlugin(t, bus)

	first := allocate(t, p, tunnelID, sessionID)

	if _, err := subevents.SessionDown.Emit(bus, &subevents.SessionDownPayload{
		Session: subscriber.Session{
			ID:         "l2tp-3-8",
			AccessType: subscriber.AccessL2TP,
			TunnelID:   tunnelID,
			SessionID:  sessionID,
		},
		Reason: "session-down",
	}); err != nil {
		t.Fatalf("emit session-down: %v", err)
	}

	_, allocated, available := p.pool.stats()
	if allocated != 0 || available != 1 {
		t.Fatalf("L2TP address never came back: allocated=%d available=%d, want 0 and 1", allocated, available)
	}

	second := allocate(t, p, tunnelID, sessionID)
	if second != first {
		t.Fatalf("next session got %s, want the released %s", second, first)
	}
}

// VALIDATES: a second teardown for one session releases nothing further, so a
// duplicate delivery cannot free an address the next session already holds.
// PREVENTS: the corruption a double release would cause if sessionAddrs did not
// hand its entry to exactly one caller: ipv4Pool.release clears the bitmap bit
// by address, so a second call after reassignment would free a live address.
func TestPoolSessionDownReleasesOnce(t *testing.T) {
	const tunnelID = uint16(3)
	const sessionID = uint16(8)

	bus := newReleaseBus()
	p := newSingleAddressPlugin(t, bus)

	first := allocate(t, p, tunnelID, sessionID)

	down := &subevents.SessionDownPayload{
		Session: subscriber.Session{
			ID:         "l2tp-3-8",
			AccessType: subscriber.AccessL2TP,
			TunnelID:   tunnelID,
			SessionID:  sessionID,
		},
		Reason: "session-down",
	}
	if _, err := subevents.SessionDown.Emit(bus, down); err != nil {
		t.Fatalf("emit first session-down: %v", err)
	}

	// The next subscriber takes the address the first one returned.
	second := allocate(t, p, tunnelID+1, sessionID)
	if second != first {
		t.Fatalf("next session got %s, want the released %s", second, first)
	}

	// A repeat of the first session's teardown must not free it again.
	if _, err := subevents.SessionDown.Emit(bus, down); err != nil {
		t.Fatalf("emit repeated session-down: %v", err)
	}

	_, allocated, available := p.pool.stats()
	if allocated != 1 || available != 0 {
		t.Fatalf("repeated session-down freed a live address: allocated=%d available=%d, want 1 and 0",
			allocated, available)
	}
}
