// Design: docs/architecture/l2tp/subscriber-session-model.md -- transport-scoped subscriber allocation ownership

package l2tppool

import (
	"io"
	"log/slog"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
	"github.com/ze-software/ze/internal/core/report"
)

func identityRequest(access subscriber.AccessType) ppp.EventIPRequest {
	return ppp.EventIPRequest{
		SubscriberID: subscriber.SessionID(access, 7, 42),
		TunnelID:     7, SessionID: 42, Family: ppp.AddressFamilyIPv4,
	}
}

func identitySession(access subscriber.AccessType) subscriber.Session {
	session := subscriber.Session{ID: subscriber.SessionID(access, 7, 42), AccessType: access}
	if access == subscriber.AccessL2TP {
		session.TunnelID, session.SessionID = 7, 42
	} else {
		session.AccessIfIndex, session.PPPoESID = 7, 42
	}
	return session
}

// TestPoolTransportIdentitySurvivesReloadAndRelease assigns distinct addresses
// to equal numeric pairs in different transports, carries them through apply
// and rollback, then releases either answer without changing the other owner.
func TestPoolTransportIdentitySurvivesReloadAndRelease(t *testing.T) {
	for _, access := range []subscriber.AccessType{subscriber.AccessL2TP, subscriber.AccessPPPoE} {
		for _, method := range []string{"teardown", "failed-answer"} {
			t.Run(string(access)+"/"+method, func(t *testing.T) {
				p := &poolPlugin{}
				bus := newReleaseBus()
				p.setEventBus(bus)
				t.Cleanup(func() {
					p.busMu.Lock()
					defer p.busMu.Unlock()
					for _, unsubscribe := range p.unsub {
						unsubscribe()
					}
				})
				tx := p.configTransaction()
				if err := tx.Configure(reloadPoolSection(t, map[string]any{
					"ipv4": reloadIPv4("10.0.0.10", "10.0.0.11"),
				})); err != nil {
					t.Fatal(err)
				}
				firstRequest := identityRequest(subscriber.AccessL2TP)
				secondRequest := identityRequest(subscriber.AccessPPPoE)
				first, second := p.handle(firstRequest), p.handle(secondRequest)
				if !first.Accept {
					t.Fatalf("L2TP refused: %s", first.Reason)
				}
				if !second.Accept {
					t.Fatalf("PPPoE refused: %s", second.Reason)
				}
				if first.Peer == second.Peer {
					t.Fatalf("different subscriber IDs with numeric pair 7/42 share address %s", first.Peer)
				}
				if first.AllocationID == 0 {
					t.Fatal("first answer has no allocation token")
				}
				if second.AllocationID == 0 {
					t.Fatal("second answer has no allocation token")
				}
				if first.AllocationID == second.AllocationID {
					t.Fatal("different subscribers share an allocation token")
				}
				if err := tx.Verify(reloadPoolSection(t, map[string]any{
					"ipv4": reloadIPv4("10.0.0.10", "10.0.0.12"),
				})); err != nil {
					t.Fatal(err)
				}
				if err := tx.Apply(); err != nil {
					t.Fatal(err)
				}
				if err := tx.Rollback(); err != nil {
					t.Fatal(err)
				}
				released, preserved := first, second
				request, survivor := firstRequest, secondRequest
				if access == subscriber.AccessPPPoE {
					released, preserved = second, first
					request, survivor = secondRequest, firstRequest
				}
				if method == "teardown" {
					p.onSessionDown(&subevents.SessionDownPayload{Session: identitySession(access)})
				} else {
					subevents.ReleaseUndeliveredPoolAnswer(slog.New(slog.NewTextHandler(io.Discard, nil)), bus, request, released)
				}
				_, used, _ := p.pool.stats()
				if used != 1 {
					t.Fatalf("after one owner ends, allocated=%d, want 1", used)
				}
				if answer := p.handle(survivor); answer.Peer != preserved.Peer {
					t.Fatalf("surviving subscriber changed address to %s, want %s", answer.Peer, preserved.Peer)
				}
				third := p.handle(ppp.EventIPRequest{
					SubscriberID: subscriber.SessionID(subscriber.AccessL2TP, 7, 43),
					TunnelID:     7, SessionID: 43, Family: ppp.AddressFamilyIPv4,
				})
				if !third.Accept {
					t.Fatalf("released address could not be allocated: %s", third.Reason)
				}
				if third.Peer != released.Peer {
					t.Fatalf("next address=%s, want released %s", third.Peer, released.Peer)
				}
				p.onSessionDown(&subevents.SessionDownPayload{Session: identitySession(subscriber.AccessL2TP)})
				p.onSessionDown(&subevents.SessionDownPayload{Session: identitySession(subscriber.AccessPPPoE)})
				p.onSessionDown(&subevents.SessionDownPayload{Session: subscriber.Session{
					ID:         subscriber.SessionID(subscriber.AccessL2TP, 7, 43),
					AccessType: subscriber.AccessL2TP, TunnelID: 7, SessionID: 43,
				}})
				if _, used, _ := p.pool.stats(); used != 0 {
					t.Fatalf("all subscribers ended but %d allocations remain", used)
				}
			})
		}
	}
}

// TestPoolPPPoETeardownPreservesEqualL2TPPrefix proves that a PPPoE session's
// driver pair cannot release the equal pair's L2TP-only prefix delegation.
func TestPoolPPPoETeardownPreservesEqualL2TPPrefix(t *testing.T) {
	pool, err := newIPv6PrefixPool(netip.MustParsePrefix("2001:db8:100::/60"), 64)
	if err != nil {
		t.Fatal(err)
	}
	p := &poolPlugin{v6pool: pool}
	if answer := p.handlePrefix(l2tp.PrefixRequest{TunnelID: 7, SessionID: 42}); !answer.OK {
		t.Fatal(answer.Reason)
	}
	p.onSessionDown(&subevents.SessionDownPayload{Session: identitySession(subscriber.AccessPPPoE)})
	if _, used, _ := pool.stats(); used != 1 {
		t.Fatalf("PPPoE teardown released another subscriber's L2TP prefix: used=%d", used)
	}
	p.onSessionDown(&subevents.SessionDownPayload{Session: identitySession(subscriber.AccessL2TP)})
	if _, used, _ := pool.stats(); used != 0 {
		t.Fatalf("L2TP teardown did not release its own prefix: used=%d", used)
	}
}

// TestPoolMissingSubscriberIdentityIsRefused prevents an unnamespaced request
// from entering the ownership store, without discarding valid pool capacity.
func TestPoolMissingSubscriberIdentityIsRefused(t *testing.T) {
	p := radiusTestPool()
	answer := p.handle(ppp.EventIPRequest{TunnelID: 7, SessionID: 42, Family: ppp.AddressFamilyIPv4})
	if answer.Accept {
		t.Fatal("pool admitted a request with no subscriber identity")
	}
	if answer.Reason == "" {
		t.Fatal("missing identity refusal has no reason")
	}
	errors := report.Errors(1)
	if len(errors) != 1 {
		t.Fatalf("missing identity doctor errors = %d, want 1", len(errors))
	}
	if errors[0].Code != report.CodeInternalDefect {
		t.Fatalf("missing identity doctor code = %s", errors[0].Code)
	}
	if _, used, _ := p.pool.stats(); used != 0 {
		t.Fatalf("missing identity request consumed %d slots", used)
	}
}

// TestPoolTransportIdentityStatusUsernames uses the shared registry for two
// equal driver pairs and verifies each allocation reports its own subscriber.
func TestPoolTransportIdentityStatusUsernames(t *testing.T) {
	p := radiusTestPool()
	for access, username := range map[subscriber.AccessType]string{
		subscriber.AccessL2TP: "l2tp-user", subscriber.AccessPPPoE: "pppoe-user",
	} {
		session := identitySession(access)
		session.Username = username
		subscriber.DefaultRegistry.Add(&session)
		t.Cleanup(func() { subscriber.DefaultRegistry.Remove(session.ID) })
		if answer := p.handle(identityRequest(access)); !answer.Accept {
			t.Fatal(answer.Reason)
		}
	}
	result := p.showPool()
	status, ok := result.(poolShowResult)
	if !ok {
		t.Fatalf("pool status has type %T, want poolShowResult", result)
	}
	names := make(map[string]bool)
	for _, allocation := range status.Sessions {
		names[allocation.Username] = true
	}
	if !names["l2tp-user"] {
		t.Fatal("L2TP allocation lost its subscriber username")
	}
	if !names["pppoe-user"] {
		t.Fatal("PPPoE allocation lost its subscriber username")
	}
}
