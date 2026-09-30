// Design: docs/architecture/bfd.md -- session registry and authentication
// Related: auth_join.go -- sharedAuthLocked, replaceAuth
//
// VALIDATES: a request that revives a released session through the shared
// join (sharedEntryLocked) gives that session its own authentication, as the
// exact-key revive (reviveReleasedLocked) does. A released session has no
// client left relying on the authentication it was built with.
// PREVENTS: a client that asks for Keyed SHA1 reviving a released
// unauthenticated session and running on it unauthenticated.
package engine

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/packet"
)

// TestSharedReviveTakesTheRequestAuth releases an unauthenticated session
// after a packet arrived, so the engine keeps it for its Detection Time, then
// asks for the same peer with no local address and a Keyed SHA1 auth block.
// The request revives the released session, and the session now carries the
// request's authentication.
func TestSharedReviveTakesTheRequestAuth(t *testing.T) {
	clk := &steppedClock{now: time.Unix(1_000_000, 0)}
	l := NewLoop(&captureTransport{}, clk)
	keptReq := reqFor(addrB, addrA)
	keptHandle, err := l.EnsureSession(keptReq)
	if err != nil {
		t.Fatalf("EnsureSession kept: %v", err)
	}
	kept := keptReq.Key()
	l.handleInbound(inboundControlState(kept.Peer, kept.Local, kept.Interface, 0, packet.StateDown))
	if err := l.ReleaseSession(keptHandle); err != nil {
		t.Fatalf("ReleaseSession: %v", err)
	}

	authReq := reqFor(addrB, addrA)
	authReq.Local = netip.Addr{}
	authReq.Auth = &api.AuthSettings{Type: packet.AuthTypeKeyedSHA1, KeyID: 3, Secret: []byte("REVIVE-SECRET")}
	h, err := l.EnsureSession(authReq)
	if err != nil {
		t.Fatalf("EnsureSession with auth: %v", err)
	}
	if h.Key() != kept {
		t.Fatalf("the request got session %+v, want the released session %+v", h.Key(), kept)
	}
	l.mu.Lock()
	entry := l.sessions[kept]
	released := entry.released
	hasAuth := entry.machine.HasAuth()
	same := sameAuth(entry.auth, authReq.Auth)
	l.mu.Unlock()
	if released {
		t.Fatal("the session is still released after the request revived it")
	}
	if !hasAuth {
		t.Fatal("the revived session has no authentication pair, want the request's Keyed SHA1")
	}
	if !same {
		t.Fatal("the revived session records an authentication other than the request's")
	}
}
