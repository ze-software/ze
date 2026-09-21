// Design: docs/architecture/bfd.md -- RFC 5883 conformance coverage
//
// Proves RFC 5883 Section 4.1: two multihop sessions between the same pair of
// systems exist only when at least one endpoint address differs. EnsureSession
// keys a session on (peer, local, VRF, interface, mode), so a second request for
// the same address pair joins the existing session instead of creating a second
// one that the address pair alone could not demultiplex.

package engine

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/core/clock"
)

func multihopReq(peer, local string) api.SessionRequest {
	return api.SessionRequest{
		Peer:                  netip.MustParseAddr(peer),
		Local:                 netip.MustParseAddr(local),
		Mode:                  api.MultiHop,
		DesiredMinTxInterval:  10_000,
		RequiredMinRxInterval: 10_000,
		DetectMult:            3,
	}
}

// TestRFC5883MultihopSessionsNeedDistinctEndpoint asks one engine for two
// multihop sessions toward the same peer system from two local loopbacks, then
// for a second session over an address pair it already holds, and counts the
// sessions and discriminators each request produced.
//
// RFC requirement: RFC5883-4.1-1 positive — two multihop sessions to the same peer address whose local addresses differ are two sessions with two distinct local discriminators.
// RFC requirement: RFC5883-4.1-1 negative — a second multihop request over an address pair the engine already holds does not create a second session: the session count stays at one and the existing session is shared.
func TestRFC5883MultihopSessionsNeedDistinctEndpoint(t *testing.T) {
	l := NewLoop(&captureTransport{}, clock.RealClock{})

	fromLoopback1 := multihopReq("192.0.2.1", "198.51.100.1")
	fromLoopback2 := multihopReq("192.0.2.1", "198.51.100.2")
	if _, err := l.EnsureSession(fromLoopback1); err != nil {
		t.Fatalf("EnsureSession from loopback 1: %v", err)
	}
	if _, err := l.EnsureSession(fromLoopback2); err != nil {
		t.Fatalf("EnsureSession from loopback 2: %v", err)
	}

	l.mu.Lock()
	n := len(l.sessions)
	d1 := l.sessions[fromLoopback1.Key()].machine.LocalDiscriminator()
	d2 := l.sessions[fromLoopback2.Key()].machine.LocalDiscriminator()
	l.mu.Unlock()
	if n != 2 {
		t.Fatalf("two distinct local addresses produced %d sessions, want 2", n)
	}
	if d1 == d2 {
		t.Fatalf("the two sessions share local discriminator %d; want distinct", d1)
	}

	if _, err := l.EnsureSession(fromLoopback1); err != nil {
		t.Fatalf("second EnsureSession over the held address pair: %v", err)
	}
	l.mu.Lock()
	n = len(l.sessions)
	rc := l.sessions[fromLoopback1.Key()].machine.Refcount()
	l.mu.Unlock()
	if n != 2 {
		t.Fatalf("a repeated address pair produced a third session (%d total); want the pair to share one", n)
	}
	if rc != 2 {
		t.Fatalf("refcount of the shared session = %d, want 2", rc)
	}
}
