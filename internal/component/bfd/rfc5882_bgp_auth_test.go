// Design: docs/guide/bfd.md -- Authentication, and Enabling BFD on a BGP peer
// Related: enabled_test.go -- the same config apply the plugin lifecycle runs
//
// RFC 5882 Section 10.2 asks that an EBGP session advised by BFD use BFD
// authentication. Ze meets it through its own config surface: a BGP peer's
// `connection bfd { profile <name> }` names a bfd profile, and that profile's
// `auth` block configures RFC 5880 Section 6.7 authentication for the session.
// These tests start from the operator's bfd section, ask for the session with
// the request a single-hop BGP peer builds (reactor bfdRequestFor: peer, local,
// mode and the profile name, no timers and no auth of its own), and drive it
// through the real pluginService.EnsureSession and a running engine loop. The
// far end of an in-memory transport pair reads what Ze puts on the wire, or
// runs a second BFD system that answers it.
package bfd

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/engine"
	"github.com/ze-software/ze/internal/component/bfd/packet"
	"github.com/ze-software/ze/internal/component/bfd/transport"
	"github.com/ze-software/ze/internal/core/clock"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// bgpAuthSection defines two profiles a BGP peer can name: "secure" carries a
// Keyed SHA1 auth block, "open" carries none.
const bgpAuthSection = `{"bfd":{"profile":{` +
	`"secure":{"desired-min-tx-us":"100000","required-min-rx-us":"100000",` +
	`"auth":{"type":"keyed-sha1","key-id":"7","secret":"EBGP-SECRET"}},` +
	`"open":{"desired-min-tx-us":"100000","required-min-rx-us":"100000"}}}}`

var (
	bgpPeerAddr  = netip.MustParseAddr("192.0.2.1")
	bgpLocalAddr = netip.MustParseAddr("192.0.2.2")
)

// bgpUpWait bounds the handshake. While a session is not Up, RFC 5880
// Section 6.8.3 holds its transmit interval at one second or more, so the
// three-way handshake takes a few seconds on the in-memory pair.
const bgpUpWait = 6 * time.Second

// bgpDownWait is how long a refused peer is watched. It covers the same
// handshake with room to spare, so a session that would come Up does.
const bgpDownWait = 4 * time.Second

// bgpPeerSessionRequest is the request a single-hop BGP peer that names
// profile hands to the Service.
func bgpPeerSessionRequest(profile string) api.SessionRequest {
	return api.SessionRequest{Peer: bgpPeerAddr, Local: bgpLocalAddr, Mode: api.SingleHop, Profile: profile}
}

// bgpBFDSessionFor applies bgpAuthSection the way the plugin lifecycle does,
// runs the single-hop loop over one end of an in-memory pair, opens the
// session a BGP peer naming profile asks for, and returns its handle with the
// far end of the pair.
func bgpBFDSessionFor(t *testing.T, profile string) (api.SessionHandle, *transport.Loopback) {
	t.Helper()

	cfg, err := parseSections([]sdk.ConfigSection{{Root: configRoot, Data: bgpAuthSection}})
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

	handle, err := (&pluginService{state: state}).EnsureSession(bgpPeerSessionRequest(profile))
	if err != nil {
		t.Fatalf("EnsureSession(profile %q): %v", profile, err)
	}
	return handle, far
}

// firstControlFrom reads the first Control packet Ze sends to the far end and
// returns its mandatory section with the raw bytes.
func firstControlFrom(t *testing.T, far *transport.Loopback) (packet.Control, []byte) {
	t.Helper()
	select {
	case in := <-far.RX():
		raw := append([]byte(nil), in.Bytes...)
		in.Release()
		c, _, err := packet.ParseControl(raw)
		if err != nil {
			t.Fatalf("ParseControl: %v", err)
		}
		return c, raw
	case <-time.After(bgpUpWait):
		t.Fatalf("no Control packet reached the peer in %s", bgpUpWait)
	}
	return packet.Control{}, nil
}

// runBGPNeighborBFD starts a second BFD system on the far end of the pair,
// the BGP neighbor's side of the session, authenticated with auth (nil for
// none).
func runBGPNeighborBFD(t *testing.T, far *transport.Loopback, auth *api.AuthSettings) {
	t.Helper()
	loop := engine.NewLoop(far, clock.RealClock{})
	if err := loop.Start(); err != nil {
		t.Fatalf("neighbor loop.Start: %v", err)
	}
	t.Cleanup(func() {
		if err := loop.Stop(); err != nil {
			t.Errorf("neighbor loop.Stop: %v", err)
		}
	})
	req := api.SessionRequest{
		Peer:                  bgpLocalAddr,
		Local:                 bgpPeerAddr,
		VRF:                   api.DefaultVRF,
		Mode:                  api.SingleHop,
		DesiredMinTxInterval:  100000,
		RequiredMinRxInterval: 100000,
		DetectMult:            3,
		Auth:                  auth,
	}
	if _, err := loop.EnsureSession(req); err != nil {
		t.Fatalf("neighbor EnsureSession: %v", err)
	}
}

// reachesUp reports whether the session reaches Up within wait.
func reachesUp(sub <-chan api.StateChange, wait time.Duration) bool {
	deadline := time.After(wait)
	for {
		select {
		case ev, ok := <-sub:
			if !ok {
				return false
			}
			if ev.State == packet.StateUp {
				return true
			}
		case <-deadline:
			return false
		}
	}
}

// neighborSHA1 is the neighbor's Keyed SHA1 configuration with secret.
func neighborSHA1(secret string) *api.AuthSettings {
	return &api.AuthSettings{Type: packet.AuthTypeKeyedSHA1, KeyID: 7, Secret: []byte(secret)}
}

// RFC requirement: RFC5882-10.2-1 positive -- "BFD authentication SHOULD be
// used": the session a BGP peer opens by naming the profile "secure", whose
// auth block is Keyed SHA1 key 7, sends Control packets with the A bit set and
// an Authentication Section of Auth Type 4 (Keyed SHA1), Auth Len 28 and Key
// ID 7, and comes Up against a neighbor configured with the same key.
//
// Goal and method: the operator's bfd section is applied, the BGP peer's
// request goes through pluginService.EnsureSession, and the first packet the
// far end reads is decoded; a neighbor loop with the same Keyed SHA1 key then
// answers, and the BGP session's handle must report Up.
func TestRFC5882BGPPeerAuthProfileSignsItsSession(t *testing.T) {
	handle, far := bgpBFDSessionFor(t, "secure")
	sub := handle.Subscribe()
	defer handle.Unsubscribe(sub)

	c, raw := firstControlFrom(t, far)
	if !c.Auth {
		t.Fatalf("Control packet A bit clear, want set: the profile's auth block did not reach the BGP peer's session")
	}
	if int(c.Length) > len(raw) {
		t.Fatalf("Length %d exceeds the %d bytes read", c.Length, len(raw))
	}
	hdr, err := packet.ParseAuth(raw[packet.MandatoryLen:c.Length])
	if err != nil {
		t.Fatalf("ParseAuth: %v", err)
	}
	if hdr.Type != packet.AuthTypeKeyedSHA1 {
		t.Errorf("Auth Type = %d, want %d (Keyed SHA1, the profile's type)", hdr.Type, packet.AuthTypeKeyedSHA1)
	}
	if hdr.Len != packet.AuthLenKeyedSHA1 {
		t.Errorf("Auth Len = %d, want %d", hdr.Len, packet.AuthLenKeyedSHA1)
	}
	if len(hdr.Body) == 0 || hdr.Body[0] != 7 {
		t.Errorf("Auth Key ID = %v, want 7 (the profile's key-id)", hdr.Body)
	}

	runBGPNeighborBFD(t, far, neighborSHA1("EBGP-SECRET"))
	if !reachesUp(sub, bgpUpWait) {
		t.Fatalf("authenticated BGP peer session not Up in %s against a neighbor with the same key", bgpUpWait)
	}
}

// RFC requirement: RFC5882-10.2-1 negative -- authentication the BGP peer's
// profile configures is enforced, not advisory: a neighbor that signs Keyed
// SHA1 key 7 with a different secret never brings the session Up, because
// every packet it sends fails the digest check (RFC 5880 Section 6.7.4) and
// is discarded.
//
// Goal and method: the same session as the positive test, answered by a
// neighbor whose secret differs; the handle is watched for longer than the
// positive handshake takes and must not report Up.
func TestRFC5882BGPPeerAuthProfileRefusesMismatchedNeighbor(t *testing.T) {
	handle, far := bgpBFDSessionFor(t, "secure")
	sub := handle.Subscribe()
	defer handle.Unsubscribe(sub)

	runBGPNeighborBFD(t, far, neighborSHA1("WRONG-SECRET"))
	if reachesUp(sub, bgpDownWait) {
		t.Fatalf("BGP peer session came Up against a neighbor whose Keyed SHA1 secret differs")
	}
}

// RFC requirement: RFC5882-10.2-1 negative -- a neighbor that omits
// authentication cannot talk the BGP peer's authenticated session Up: its
// packets carry the A bit clear while the session uses authentication, and
// RFC 5880 Section 6.8.6 discards them, so Ze never falls back to running the
// session unauthenticated.
//
// Goal and method: the same session as the positive test, answered by a
// neighbor with no auth configured; the handle must not report Up.
func TestRFC5882BGPPeerAuthProfileRefusesUnauthenticatedNeighbor(t *testing.T) {
	handle, far := bgpBFDSessionFor(t, "secure")
	sub := handle.Subscribe()
	defer handle.Unsubscribe(sub)

	runBGPNeighborBFD(t, far, nil)
	if reachesUp(sub, bgpDownWait) {
		t.Fatalf("authenticated BGP peer session came Up against a neighbor sending no authentication")
	}
}

// RFC requirement: RFC5882-10.2-1 negative -- how Ze handles the absence of
// authentication on its own side: the SHOULD permits it, so a BGP peer whose
// profile "open" has no auth block runs the session unauthenticated, sending
// Control packets with the A bit clear and no Authentication Section (Length
// 24), and comes Up against a neighbor that uses none.
//
// Goal and method: the BGP peer's request names "open"; the first packet the
// far end reads is decoded, then an unauthenticated neighbor answers and the
// handle must report Up.
func TestRFC5882BGPPeerProfileWithoutAuthRunsUnauthenticated(t *testing.T) {
	handle, far := bgpBFDSessionFor(t, "open")
	sub := handle.Subscribe()
	defer handle.Unsubscribe(sub)

	c, _ := firstControlFrom(t, far)
	if c.Auth {
		t.Fatalf("Control packet A bit set for a profile with no auth block")
	}
	if c.Length != packet.MandatoryLen {
		t.Errorf("Length = %d, want %d: no Authentication Section", c.Length, packet.MandatoryLen)
	}

	runBGPNeighborBFD(t, far, nil)
	if !reachesUp(sub, bgpUpWait) {
		t.Fatalf("unauthenticated BGP peer session not Up in %s against an unauthenticated neighbor", bgpUpWait)
	}
}
