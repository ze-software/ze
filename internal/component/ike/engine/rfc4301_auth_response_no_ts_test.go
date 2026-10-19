// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the Child SA's selectors are the negotiated ones
// RFC: rfc/short/rfc4301.md -- RFC4301-4.4.2-2

package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestRFC4301AuthResponseWithoutTSIsRefused drives the function handleAuthResponse
// (fsm.go) calls on an authenticated IKE_AUTH response.
//
// VALIDATES: a response that carries SAr2 and omits TSi, TSr or both is refused with
// INVALID_SYNTAX and records no negotiated pair, so no Child SA is built from it.
// PREVENTS: the Child SA going up with host-to-host selectors for every protocol and
// port, wider than the pairs ze proposed, because nothing was negotiated to install.
func TestRFC4301AuthResponseWithoutTSIsRefused(t *testing.T) {
	// RFC requirement: RFC4301-4.4.2-2 negative -- an IKE_AUTH response that accepts the Child SA (SAr2) with no TSi or no TSr payload is refused with INVALID_SYNTAX and leaves no negotiated selector to populate the SAD entry with.
	log := slogutil.DiscardLogger()
	tsi := tsPayload(t, wire.PayloadTypeTSi, "10.1.1.0/24")
	tsr := tsPayload(t, wire.PayloadTypeTSr, "10.2.1.0/24")

	cases := []struct {
		name     string
		tsi, tsr *wire.PayloadTS
	}{
		{"no TSi", nil, tsr},
		{"no TSr", tsi, nil},
		{"neither", nil, nil},
	}
	for _, c := range cases {
		sa := tisInitiator(t, tisPeer(t))
		ok, notify := adoptAuthResponseNegotiation(sa, false, true, c.tsi, c.tsr, log)
		if ok {
			t.Errorf("%s: a Child SA answer with no traffic selector was adopted", c.name)
		}
		if notify != wire.NotifyInvalidSyntax {
			t.Errorf("%s: refused with notify %d, want INVALID_SYNTAX (%d)",
				c.name, notify, wire.NotifyInvalidSyntax)
		}
		if sa.NegotiatedPairs != nil {
			t.Errorf("%s: the refused answer left negotiated pairs %s", c.name, pairsText(sa.NegotiatedPairs))
		}
	}

	// A response with no SAr2 (an EAP round, a childless answer) carries no selector and is
	// not refused for lacking one.
	childless := tisInitiator(t, tisPeer(t))
	if ok, _ := adoptAuthResponseNegotiation(childless, false, false, nil, nil, log); !ok {
		t.Error("a response carrying no Child SA was refused for carrying no traffic selector")
	}

	// The discriminator: the same SAr2 answer WITH both payloads is adopted, and the
	// recorded pair is exactly the one answered.
	full := tisInitiator(t, tisPeer(t))
	if ok, notify := adoptAuthResponseNegotiation(full, false, true, tsi, tsr, log); !ok {
		t.Fatalf("a complete narrowed answer was refused with notify %d", notify)
	}
	if got := pairsText(full.NegotiatedPairs); got != "10.1.1.0/24 <-> 10.2.1.0/24" {
		t.Errorf("the adopted answer recorded %s, want exactly the answered pair", got)
	}
}
