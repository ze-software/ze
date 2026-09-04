// VALIDATES: spec-radius-acct-session-attributes AC-6 -- every locally
// initiated session teardown names the RFC 2866 Section 5.10 cause that is
// true of it, and the session-down event carries it out of the reactor.
// PREVENTS: a teardown that reaches no subscriber at all. Before this spec the
// timeout and administrative paths removed the session from the tunnel map
// without emitting (l2tp, session-down), so RADIUS sent no Accounting-Stop,
// the pool never released the address and the shaper never dropped its rules.

package l2tp

import (
	"log/slog"
	"net/netip"
	"sync"
	"testing"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/pkg/ze"
)

// causeBus is the smallest EventBus that records session-down payloads.
type causeBus struct {
	mu       sync.Mutex
	handlers []func(any)
	seen     []*l2tpevents.SessionDownPayload
}

var _ ze.EventBus = (*causeBus)(nil)

func newCauseBus() *causeBus { return &causeBus{} }

func (b *causeBus) Emit(_, eventType string, payload any) (int, error) {
	if eventType == l2tpevents.SessionDownEvent {
		if p, ok := payload.(*l2tpevents.SessionDownPayload); ok {
			b.mu.Lock()
			b.seen = append(b.seen, p)
			b.mu.Unlock()
		}
	}
	b.mu.Lock()
	hs := append([]func(any){}, b.handlers...)
	b.mu.Unlock()
	for _, h := range hs {
		h(payload)
	}
	return 0, nil
}

func (b *causeBus) Subscribe(_, _ string, handler func(any)) func() {
	if handler == nil {
		return func() {}
	}
	b.mu.Lock()
	b.handlers = append(b.handlers, handler)
	b.mu.Unlock()
	return func() {}
}

func (b *causeBus) downEvents() []*l2tpevents.SessionDownPayload {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]*l2tpevents.SessionDownPayload{}, b.seen...)
}

func TestTeardownSessionByIDEmitsCause(t *testing.T) {
	cases := []struct {
		name  string
		cause l2tpevents.TerminateCause
	}{
		{"session-timeout", l2tpevents.TerminateCauseSessionTimeout},
		{"idle-timeout", l2tpevents.TerminateCauseIdleTimeout},
		{"administrative", l2tpevents.TerminateCauseAdminReset},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newReactorForSnapshot(t)
			// An unstarted listener refuses the CDN with
			// errListenerNotStarted, which teardownSessionByID logs. The
			// event this test reads is emitted either way, and no UDP socket
			// is needed to prove it.
			r.listener = newUDPListener(netip.MustParseAddrPort("127.0.0.1:0"), slog.Default())
			bus := newCauseBus()
			r.eventBus = bus
			sess := &L2TPSession{localSID: 7, state: L2TPSessionEstablished, username: "grace"}
			insertEstablishedTunnel(t, r, 11, 22, netip.MustParseAddrPort("10.0.0.9:1701"), sess)

			if err := r.teardownSessionByID(7, tc.cause); err != nil {
				t.Fatalf("teardownSessionByID: %v", err)
			}

			events := bus.downEvents()
			if len(events) != 1 {
				t.Fatalf("session-down events = %d, want 1", len(events))
			}
			if events[0].Cause != tc.cause {
				t.Errorf("cause = %d, want %d", events[0].Cause, tc.cause)
			}
			if events[0].Username != "grace" {
				t.Errorf("username = %q, want %q", events[0].Username, "grace")
			}
		})
	}
}

func TestTeardownSessionOnTunnelEmitsAdminReset(t *testing.T) {
	r := newReactorForSnapshot(t)
	r.listener = newUDPListener(netip.MustParseAddrPort("127.0.0.1:0"), slog.Default())
	bus := newCauseBus()
	r.eventBus = bus
	sess := &L2TPSession{localSID: 9, state: L2TPSessionEstablished, username: "ada"}
	insertEstablishedTunnel(t, r, 11, 22, netip.MustParseAddrPort("10.0.0.9:1701"), sess)

	if n := r.TeardownAllSessions(); n != 1 {
		t.Fatalf("TeardownAllSessions = %d, want 1", n)
	}

	events := bus.downEvents()
	if len(events) != 1 {
		t.Fatalf("session-down events = %d, want 1", len(events))
	}
	if events[0].Cause != l2tpevents.TerminateCauseAdminReset {
		t.Errorf("cause = %d, want %d", events[0].Cause, l2tpevents.TerminateCauseAdminReset)
	}
}
