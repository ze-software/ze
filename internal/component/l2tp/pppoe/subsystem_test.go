// Design: docs/architecture/l2tp/subscriber-session-model.md -- PPPoE lifecycle publication
//
// Goal: prove that a PPPoE teardown publishes the identifier pair the address
// pool allocated under, because that is what makes the release reachable.
// Method: drive Subsystem.handlePPPEvent, the entry point the driver's event
// consumer calls, and read the payload off a recording bus.
//
// The consumer half is proven in internal/component/l2tp/plugins/pool:
// TestPoolReleasesPPPoEAddressOnSessionDown releases on exactly the pair
// asserted here.

package pppoe

import (
	"log/slog"
	"net"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
	"github.com/ze-software/ze/pkg/ze"
)

// recordBus is an EventBus that dispatches to in-process subscribers only,
// which is what the engine bus does for the pool plugin.
type recordBus struct {
	mu       sync.Mutex
	handlers map[string][]func(any)
}

var _ ze.EventBus = (*recordBus)(nil)

func newRecordBus() *recordBus {
	return &recordBus{handlers: make(map[string][]func(any))}
}

func (b *recordBus) Emit(namespace, eventType string, payload any) (int, error) {
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

func (b *recordBus) Subscribe(namespace, eventType string, handler func(any)) func() {
	key := namespace + "/" + eventType
	b.mu.Lock()
	b.handlers[key] = append(b.handlers[key], handler)
	b.mu.Unlock()
	return func() {}
}

// newTestSubsystem builds a subsystem holding one interface server, with the
// discovery socket closed (-1) so any PADT it sends fails harmlessly.
func newTestSubsystem(t *testing.T, bus ze.EventBus, ifIndex int) *Subsystem {
	t.Helper()
	logger := slog.Default()
	srv := &InterfaceServer{
		ifName:   "eth0",
		ifIndex:  ifIndex,
		sessions: newSessionTable("eth0", 0),
		acName:   "ze-test",
		discFD:   -1,
		logger:   logger,
	}
	return &Subsystem{
		logger:  logger,
		discFD:  -1,
		servers: map[int]*InterfaceServer{ifIndex: srv},
		bus:     bus,
	}
}

// VALIDATES: AC-1 -- a PPPoE session that comes up and goes down publishes
// subevents.SessionDown carrying the pool's allocation key.
// PREVENTS: regression to the state where PPPoE emitted a session-down whose
// Session left AccessIfIndex unset, so every consumer keyed on the PPP
// driver's request pair read (0, sid) and released nothing.
func TestPPPoESessionDownPublishesPoolKey(t *testing.T) {
	const ifIndex = 7
	const sid = uint16(42)

	bus := newRecordBus()
	sub := newTestSubsystem(t, bus, ifIndex)

	mac := net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0x01}
	srv := sub.servers[ifIndex]
	if err := srv.sessions.Add(&Session{SID: sid, MAC: mac, IfName: "eth0", UnitNum: 3, PppoxFD: -1}); err != nil {
		t.Fatalf("seed session table: %v", err)
	}
	t.Cleanup(func() { subscriber.DefaultRegistry.Remove(pppoeSessionID(ifIndex, sid)) })

	var down *subevents.SessionDownPayload
	subevents.SessionDown.Subscribe(bus, func(p *subevents.SessionDownPayload) { down = p })

	sub.handlePPPEvent(ppp.EventSessionUp{TunnelID: uint16(ifIndex), SessionID: sid})
	if _, ok := subscriber.DefaultRegistry.Get(pppoeSessionID(ifIndex, sid)); !ok {
		t.Fatal("session-up did not register the session")
	}

	sub.handlePPPEvent(ppp.EventSessionDown{TunnelID: uint16(ifIndex), SessionID: sid, Reason: "peer hangup"})

	if down == nil {
		t.Fatal("session-down published no subscriber event")
	}
	// The pool allocated under the pair ppp.EventIPRequest carried, which
	// startPPPoEPoolDrain forwards straight from the driver: (ifindex, sid).
	tunnelID, sessionID := down.Session.PPPKey()
	if tunnelID != uint16(ifIndex) || sessionID != sid {
		t.Fatalf("PPPKey = (%d, %d), want (%d, %d)", tunnelID, sessionID, ifIndex, sid)
	}
	if down.Session.AccessType != subscriber.AccessPPPoE {
		t.Fatalf("AccessType = %q, want %q", down.Session.AccessType, subscriber.AccessPPPoE)
	}
	if _, ok := subscriber.DefaultRegistry.Get(pppoeSessionID(ifIndex, sid)); ok {
		t.Fatal("session-down left the session in the registry")
	}
}

// VALIDATES: AC-1 -- a session torn down before it reached session-up still
// publishes, because IPCP has already taken an address for it.
// PREVENTS: a leak on the NCP-failure path, where the registry holds no entry
// and a registry-gated publication would emit nothing at all.
func TestPPPoESessionDownPublishesWithoutSessionUp(t *testing.T) {
	const ifIndex = 9
	const sid = uint16(11)

	bus := newRecordBus()
	sub := newTestSubsystem(t, bus, ifIndex)

	var down *subevents.SessionDownPayload
	subevents.SessionDown.Subscribe(bus, func(p *subevents.SessionDownPayload) { down = p })

	sub.handlePPPEvent(ppp.EventSessionDown{TunnelID: uint16(ifIndex), SessionID: sid, Reason: "ncp failed"})

	if down == nil {
		t.Fatal("session-down with no registry entry published no subscriber event")
	}
	tunnelID, sessionID := down.Session.PPPKey()
	if tunnelID != uint16(ifIndex) || sessionID != sid {
		t.Fatalf("PPPKey = (%d, %d), want (%d, %d)", tunnelID, sessionID, ifIndex, sid)
	}
}
