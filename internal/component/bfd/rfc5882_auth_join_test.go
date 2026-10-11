// Design: docs/guide/bfd.md -- Authentication, one session per remote system
// Related: rfc5882_bgp_auth_test.go -- the harness these tests follow
// Related: engine/engine.go -- EnsureSession refuses a join whose authentication differs
//
// Two clients that name the same remote system share one BFD session (RFC 5882
// Section 4.4), and one session carries one authentication configuration. A
// client that names an authenticated profile and lands on an unauthenticated
// session would run unauthenticated with no word to the operator, and the
// reverse join would silently put a client that asked for no authentication
// onto a session its neighbor may not authenticate. These tests apply a bfd
// section through the plugin's config path, open a session through the real
// pluginService.EnsureSession (which resolves the profile) and a running engine
// loop, then ask for the same remote system again with a second profile. The
// far end of an in-memory transport pair reads what the first session puts on
// the wire after the second request.
//
// VALIDATES: RFC 5882 Section 10.2 -- a client that asks for BFD
// authentication never runs on an unauthenticated session, and a session's
// authentication is never changed by a second client's join.
// PREVENTS: a join that compares only the key and lands a client on a session
// whose authentication differs from what its profile configures.
package bfd

import (
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/engine"
	"github.com/ze-software/ze/internal/component/bfd/packet"
	"github.com/ze-software/ze/internal/component/bfd/transport"
	"github.com/ze-software/ze/internal/core/clock"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// authJoinSection defines seven profiles. "secure" and "secure-twin" carry the
// same Keyed SHA1 auth block under two names; "rekeyed" differs from "secure"
// only in its key ID, "resecret" only in its secret, "md5" only in its Auth
// Type (Keyed MD5, neither meticulous); "meticulous" names Meticulous Keyed
// SHA1, which differs from "secure" in both its Auth Type and its Meticulous
// flag; "open" carries none.
const authJoinSection = `{"bfd":{"profile":{` +
	`"secure":{"auth":{"type":"keyed-sha1","key-id":"7","secret":"JOIN-SECRET"}},` +
	`"secure-twin":{"auth":{"type":"keyed-sha1","key-id":"7","secret":"JOIN-SECRET"}},` +
	`"rekeyed":{"auth":{"type":"keyed-sha1","key-id":"8","secret":"JOIN-SECRET"}},` +
	`"resecret":{"auth":{"type":"keyed-sha1","key-id":"7","secret":"OTHER-SECRET"}},` +
	`"md5":{"auth":{"type":"keyed-md5","key-id":"7","secret":"JOIN-SECRET"}},` +
	`"meticulous":{"auth":{"type":"meticulous-keyed-sha1","key-id":"7","secret":"JOIN-SECRET"}},` +
	`"open":{}}}}`

// authJoinFixture applies authJoinSection, runs the single-hop loop over one
// end of an in-memory pair, and returns the Service, the loop and the far end.
func authJoinFixture(t *testing.T) (*pluginService, *engine.Loop, *transport.Loopback) {
	t.Helper()

	cfg, err := parseSections([]sdk.ConfigSection{{Root: configRoot, Data: authJoinSection}})
	if err != nil {
		t.Fatalf("parseSections: %v", err)
	}
	state := newRuntimeState()
	runtimeStateGuard.Lock()
	err = state.applyConfig(cfg)
	runtimeStateGuard.Unlock()
	if err != nil {
		t.Fatalf("applyConfig: %v", err)
	}

	near, far := transport.Pair(api.SingleHop, bgpLocalAddr, bgpPeerAddr)
	loop := engine.NewLoop(near, clock.RealClock{})
	if err := loop.Start(); err != nil {
		t.Fatalf("loop.Start: %v", err)
	}
	t.Cleanup(func() {
		if err := loop.Stop(); err != nil {
			t.Errorf("loop.Stop: %v", err)
		}
	})
	state.loops[loopKey{vrf: api.DefaultVRF, mode: api.SingleHop}] = loop
	return &pluginService{state: state}, loop, far
}

// authJoinNextControl discards what the far end has already received and
// returns the raw bytes of the next Control packet Ze sends, so the packet
// read was built after the call that preceded this one.
func authJoinNextControl(t *testing.T, far *transport.Loopback) []byte {
	t.Helper()
	for drained := false; !drained; {
		select {
		case in := <-far.RX():
			in.Release()
		default:
			drained = true
		}
	}
	select {
	case in := <-far.RX():
		raw := append([]byte(nil), in.Bytes...)
		in.Release()
		return raw
	case <-time.After(bgpUpWait):
		t.Fatalf("no Control packet reached the peer in %s", bgpUpWait)
	}
	return nil
}

// assertWireAuth checks that raw carries the A bit and a Keyed SHA1 section
// with key ID 7 when authenticated is true, and neither when it is false.
func assertWireAuth(t *testing.T, raw []byte, authenticated bool) {
	t.Helper()
	c, _, err := packet.ParseControl(raw)
	if err != nil {
		t.Fatalf("ParseControl: %v", err)
	}
	if c.Auth != authenticated {
		t.Fatalf("first session's Control packet A bit = %v, want %v", c.Auth, authenticated)
	}
	if !authenticated {
		return
	}
	if raw[packet.MandatoryLen] != packet.AuthTypeKeyedSHA1 {
		t.Errorf("Auth Type = %d, want %d (Keyed SHA1)", raw[packet.MandatoryLen], packet.AuthTypeKeyedSHA1)
	}
	if raw[packet.MandatoryLen+2] != 7 {
		t.Errorf("Key ID = %d, want 7", raw[packet.MandatoryLen+2])
	}
}

// authJoinCase is one ordered pair of profiles naming the same remote system.
type authJoinCase struct {
	name, first, second string
	// firstAuth says whether the first profile carries an auth block.
	firstAuth bool
	// flipMeticulous makes the second request carry the authentication
	// "second" resolves to with only its Meticulous flag inverted, under no
	// profile (authJoinSecondRequest).
	flipMeticulous bool
}

// authJoinRefusedCases are the pairs whose effective authentication differs.
var authJoinRefusedCases = []authJoinCase{
	{name: "auth then no auth", first: "secure", second: "open", firstAuth: true},
	{name: "no auth then auth", first: "open", second: "secure", firstAuth: false},
	{name: "different key id", first: "secure", second: "rekeyed", firstAuth: true},
	{name: "different secret", first: "secure", second: "resecret", firstAuth: true},
	{name: "different auth type", first: "secure", second: "md5", firstAuth: true},
	{name: "keyed then meticulous", first: "secure", second: "meticulous", firstAuth: true},
	{name: "different meticulous flag", first: "secure", second: "secure", firstAuth: true, flipMeticulous: true},
}

// authJoinSecondRequest builds the second client's request for tc. A case
// that names a profile asks for it, as a protocol client does. A
// flipMeticulous case resolves tc.second, inverts only the Meticulous flag of
// its authentication, and sends that request under no profile, which
// pluginService.EnsureSession passes through unresolved. No profile can
// express that difference: authTypeFromEnum gives each meticulous enum its own
// wire Auth Type, so a profile differing in Meticulous differs in Type too, and
// the Type comparison alone would refuse it.
func authJoinSecondRequest(t *testing.T, svc *pluginService, tc authJoinCase) api.SessionRequest {
	t.Helper()
	if !tc.flipMeticulous {
		return bgpPeerSessionRequest(tc.second)
	}
	runtimeStateGuard.Lock()
	resolved, err := resolveProfile(bgpPeerSessionRequest(tc.second), svc.state.cfg.profiles)
	runtimeStateGuard.Unlock()
	if err != nil {
		t.Fatalf("resolveProfile(%q): %v", tc.second, err)
	}
	if resolved.Auth == nil {
		t.Fatalf("profile %q resolves to no authentication, want one to flip", tc.second)
	}
	flipped := *resolved.Auth
	flipped.Meticulous = !flipped.Meticulous
	resolved.Auth = &flipped
	resolved.Profile = ""
	return resolved
}

// RFC requirement: RFC5882-10.2-1 negative -- "BFD authentication SHOULD be
// used and is strongly encouraged.", and a second client asks for the remote
// system 192.0.2.1 that a first client's session already runs to, under a
// profile whose authentication differs from the session's: authenticated then
// unauthenticated, unauthenticated then authenticated, a different key ID, a
// different secret, a different Auth Type alone (Keyed MD5 against Keyed SHA1),
// Keyed SHA1 then Meticulous Keyed SHA1, and the Meticulous flag alone (the
// first profile's authentication with only that flag inverted, carried under
// no profile because no profile can express it). Each of the four fields
// sameAuth compares is the only difference in at least one case. The request is refused with engine.ErrAuthMismatch and an
// error naming the peer, whether it names the session's local address (the
// exact key) or leaves it unset (the shared join), the loop still holds one
// session, and the first session's next Control packet carries the
// authentication of the first profile (A bit, Keyed SHA1, key ID 7) or none.
func TestRFC5882AuthJoinDifferentAuthRefused(t *testing.T) {
	arms := []struct {
		name  string
		local bool
	}{
		{name: "exact key", local: true},
		{name: "shared join", local: false},
	}
	for _, tc := range authJoinRefusedCases {
		for _, arm := range arms {
			t.Run(tc.name+"/"+arm.name, func(t *testing.T) {
				svc, loop, far := authJoinFixture(t)
				if _, err := svc.EnsureSession(bgpPeerSessionRequest(tc.first)); err != nil {
					t.Fatalf("EnsureSession(%q): %v", tc.first, err)
				}

				req := authJoinSecondRequest(t, svc, tc)
				if !arm.local {
					req.Local = netip.Addr{}
				}
				h, err := svc.EnsureSession(req)
				if err == nil {
					t.Fatalf("EnsureSession(second, %s) joined session %v, want refusal", tc.name, h.Key())
				}
				if !errors.Is(err, engine.ErrAuthMismatch) {
					t.Fatalf("EnsureSession(second, %s) error = %v, want engine.ErrAuthMismatch", tc.name, err)
				}
				if !strings.Contains(err.Error(), bgpPeerAddr.String()) {
					t.Errorf("error %q does not name the peer %s", err, bgpPeerAddr)
				}
				if n := len(loop.Snapshot()); n != 1 {
					t.Fatalf("loop holds %d sessions, want 1", n)
				}
				assertWireAuth(t, authJoinNextControl(t, far), tc.firstAuth)
			})
		}
	}
}

// RFC requirement: RFC5882-10.2-1 positive -- a second client that names the
// same remote system 192.0.2.1 under a profile whose authentication is the
// first profile's shares the first session: "secure" then "secure-twin" (the
// same Keyed SHA1 key 7 and secret under another name), and "open" then
// "open". EnsureSession answers the first session's key, the loop holds one
// session, and that session's next Control packet still carries the first
// profile's authentication or none.
func TestRFC5882AuthJoinIdenticalAuthShares(t *testing.T) {
	cases := []authJoinCase{
		{name: "same auth under two names", first: "secure", second: "secure-twin", firstAuth: true},
		{name: "no auth twice", first: "open", second: "open", firstAuth: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, loop, far := authJoinFixture(t)
			first, err := svc.EnsureSession(bgpPeerSessionRequest(tc.first))
			if err != nil {
				t.Fatalf("EnsureSession(%q): %v", tc.first, err)
			}
			second, err := svc.EnsureSession(bgpPeerSessionRequest(tc.second))
			if err != nil {
				t.Fatalf("EnsureSession(%q): %v, want the shared session", tc.second, err)
			}
			if second.Key() != first.Key() {
				t.Errorf("second handle key %v, want the first session's %v", second.Key(), first.Key())
			}
			if n := len(loop.Snapshot()); n != 1 {
				t.Fatalf("loop holds %d sessions, want 1", n)
			}
			assertWireAuth(t, authJoinNextControl(t, far), tc.firstAuth)
		})
	}
}
