// Design: docs/architecture/bfd.md -- session registry and authentication
// Related: auth_join.go -- sharedAuthLocked, replaceAuth
//
// VALIDATES: a request that revives a released session through the shared
// join (sharedEntryLocked) gives that session its own authentication, as the
// exact-key revive (reviveReleasedLocked) does. A released session has no
// client left relying on the authentication it was built with.
// The revive also records the reviving request's profile as the one the
// session's authentication came from (replaceAuth sets authFrom), so a later
// refusal names that profile.
// PREVENTS: a client that asks for Keyed SHA1 reviving a released
// unauthenticated session and running on it unauthenticated, and a refusal
// that tells the operator a profile was edited when another profile revived
// the session.
package engine

import (
	"errors"
	"net/netip"
	"strings"
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

// TestReviveRecordsTheRevivingProfile creates an unauthenticated session from
// profile "open", releases it after a packet arrived so the engine keeps it
// for its Detection Time, and revives it with profile "rekeyed" and Keyed
// SHA1, once by the exact key (reviveReleasedLocked) and once by a shared join
// with no local address (sharedAuthLocked). Both reach replaceAuth. A request
// naming profile "secure" with other authentication is then refused, and the
// refusal must name "rekeyed" as the profile the session was built from; a
// request naming "open" again must not be told "open" was edited. If the
// revive kept "open" as the session's source, the first refusal would name
// "open" and the second would send the operator to release the session from
// every client, which helps nothing.
func TestReviveRecordsTheRevivingProfile(t *testing.T) {
	rekeyed := &api.AuthSettings{Type: packet.AuthTypeKeyedSHA1, KeyID: 9, Secret: []byte("REKEYED-SECRET")}
	secure := &api.AuthSettings{Type: packet.AuthTypeKeyedSHA1, KeyID: 3, Secret: []byte("SECURE-SECRET")}
	cases := []struct {
		name   string
		shared bool
	}{
		{name: "exact-key revive", shared: false},
		{name: "shared revive", shared: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clk := &steppedClock{now: time.Unix(1_000_000, 0)}
			l := NewLoop(&captureTransport{}, clk)
			open := reqFor(addrB, addrA)
			open.Profile = "open"
			openHandle, err := l.EnsureSession(open)
			if err != nil {
				t.Fatalf("EnsureSession open: %v", err)
			}
			kept := open.Key()
			l.handleInbound(inboundControlState(kept.Peer, kept.Local, kept.Interface, 0, packet.StateDown))
			if err := l.ReleaseSession(openHandle); err != nil {
				t.Fatalf("ReleaseSession: %v", err)
			}

			revive := reqFor(addrB, addrA)
			if tc.shared {
				revive.Local = netip.Addr{}
			}
			revive.Profile = "rekeyed"
			revive.Auth = rekeyed
			h, err := l.EnsureSession(revive)
			if err != nil {
				t.Fatalf("EnsureSession rekeyed: %v", err)
			}
			if h.Key() != kept {
				t.Fatalf("the revive got session %+v, want the released session %+v", h.Key(), kept)
			}

			other := reqFor(addrB, addrA)
			other.Profile = "secure"
			other.Auth = secure
			_, err = l.EnsureSession(other)
			if !errors.Is(err, ErrAuthMismatch) {
				t.Fatalf("EnsureSession secure after the revive: err = %v, want ErrAuthMismatch", err)
			}
			const wantBuiltFrom = `the session was built from bfd profile "rekeyed"`
			if text := err.Error(); !strings.Contains(text, wantBuiltFrom) {
				t.Errorf("refusal %q\ndoes not contain %q", text, wantBuiltFrom)
			}

			_, err = l.EnsureSession(open)
			if !errors.Is(err, ErrAuthMismatch) {
				t.Fatalf("EnsureSession open after the revive: err = %v, want ErrAuthMismatch", err)
			}
			text := err.Error()
			const wantFix = `give bfd profile "open" the auth block of bfd profile "rekeyed"`
			if !strings.Contains(text, wantFix) {
				t.Errorf("refusal %q\ndoes not contain %q", text, wantFix)
			}
			if strings.Contains(text, "changed after the session was built from it") {
				t.Errorf("refusal %q\nsays the profile was edited, but another profile revived the session", text)
			}
		})
	}
}
