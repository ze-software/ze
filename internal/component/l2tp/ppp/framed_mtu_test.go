// Design: docs/guide/l2tp.md -- the IP MTU of the subscriber's interface
// Related: framed_mtu.go -- interfaceMTU, framedMTUBelowIPv6
// Related: request.go -- StageFramedMTU
// Related: pppoe_framed_routes_test.go -- the same real-worker harness for routes

package ppp

import (
	"errors"
	"testing"
	"time"
)

// framedMTUSessionUp runs one real PPP worker through LCP (the peer omits
// its MRU, so the transmit limit is 1500), CHAP, an accepted Access-Accept
// that staged framed (zero stages nothing) and IPCP, and returns the
// session-up event with the MTU calls the interface backend received.
// The worker is PPPoE so the accept needs no L2TP accounting claim; the MTU
// selection runs in the same session code for both transports.
func framedMTUSessionUp(t *testing.T, sid, framed uint16) (EventSessionUp, []mtuCall) {
	t.Helper()
	driver := NewProductionDriver(discardLogger())
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	start, peer, owner := frameLeaseStart(t, sid)
	t.Cleanup(func() { closeConn(peer); driver.Stop() })
	backend, ok := owner.SessionHandle.(*testSessionHandle).backend.(*fakeBackend)
	if !ok {
		t.Fatal("the lease fixture no longer records interface calls on a fakeBackend")
	}
	start.PPPoE = true
	start.AuthMethod = AuthMethodCHAPMD5
	start.AuthRequired = true
	start.DisableIPCP = false
	driver.SessionsIn() <- start
	openLifecycleLCP(t, peer)
	request := readFramedRouteAuthRequest(t, driver, peer)
	if framed != 0 {
		if err := request.StageFramedMTU(framed); err != nil {
			t.Fatal(err)
		}
	}
	assignment := awaitFramedRouteAssignment(t, driver, peer, request)
	assignment.Acknowledge()
	return awaitFramedMTUSessionUp(t, driver.EventsOut()), backend.MTUCalls()
}

func awaitFramedMTUSessionUp(t *testing.T, events <-chan Event) EventSessionUp {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev := <-events:
			switch up := ev.(type) {
			case EventSessionUp:
				return up
			case EventSessionDown:
				t.Fatalf("session went down instead of up: %+v", up)
			}
		case <-deadline:
			t.Fatal("session never came up")
		}
	}
}

// TestFramedMTULowersInterfaceMTU proves the accepted Framed-MTU is the MTU
// pppN is configured with and the MTU the session-up event reports, that an
// Accept without one keeps the LCP transmit limit, and that a Framed-MTU
// above what LCP agreed never raises the interface past it. Method: three
// real workers with an LCP transmit limit of 1500, staging 1400, nothing and
// 9000, reading the backend's SetMTU calls and EventSessionUp.MTU.
func TestFramedMTULowersInterfaceMTU(t *testing.T) {
	cases := []struct {
		name   string
		sid    uint16
		framed uint16
		want   uint16
	}{
		{"Framed-MTU below the transmit limit applies", 41, 1400, 1400},
		{"no Framed-MTU keeps the transmit limit", 42, 0, 1500},
		{"Framed-MTU above the transmit limit keeps the limit", 43, 9000, 1500},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			up, calls := framedMTUSessionUp(t, tc.sid, tc.framed)
			if up.MTU != tc.want {
				t.Fatalf("EventSessionUp.MTU = %d, want %d", up.MTU, tc.want)
			}
			if len(calls) != 1 {
				t.Fatalf("interface MTU calls = %+v, want exactly one", calls)
			}
			if calls[0].mtu != int(tc.want) {
				t.Fatalf("pppN MTU set to %d, want %d", calls[0].mtu, tc.want)
			}
		})
	}
}

// TestStageFramedMTURefusesBelowIPv4 proves the session never accepts a
// staged value IPv4 cannot use: below 68 is refused before any request state
// is read, while 68 passes that check and is refused only because this zero
// request is not a live wait. Method: StageFramedMTU on a zero request.
func TestStageFramedMTURefusesBelowIPv4(t *testing.T) {
	if err := (EventAuthRequest{}).StageFramedMTU(MinIPv4MRU - 1); !errors.Is(err, ErrFramedMTUBelowIPv4) {
		t.Fatalf("staging 67 = %v, want ErrFramedMTUBelowIPv4", err)
	}
	if err := (EventAuthRequest{}).StageFramedMTU(MinIPv4MRU); !errors.Is(err, ErrRequestExpired) {
		t.Fatalf("staging 68 on a zero request = %v, want ErrRequestExpired", err)
	}
}

// TestFramedMTUBelowIPv6DisablesIPv6 proves a Framed-MTU below 1280 makes the
// session treat IPv6 as unsupported although every LCP MRU allows it, and
// that 1280 and an absent Framed-MTU do not. Method: ipv6MRUSupported on a
// session whose negotiated, local and admitted MRUs are 1500.
func TestFramedMTUBelowIPv6DisablesIPv6(t *testing.T) {
	cases := []struct {
		framed uint16
		want   bool
	}{
		{MinIPv6MRU - 1, false},
		{MinIPv6MRU, true},
		{0, true},
	}
	for _, tc := range cases {
		s := &pppSession{negotiatedMRU: MaxFrameLen, localMRU: MaxFrameLen, maxMRU: MaxFrameLen,
			authenticatedFramedMTU: tc.framed}
		if got := s.ipv6MRUSupported(); got != tc.want {
			t.Errorf("Framed-MTU %d: ipv6MRUSupported = %v, want %v", tc.framed, got, tc.want)
		}
		if got := s.framedMTUBelowIPv6(); got == tc.want {
			t.Errorf("Framed-MTU %d: framedMTUBelowIPv6 = %v, want %v", tc.framed, got, !tc.want)
		}
	}
}
