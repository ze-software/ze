// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- one policy per negotiated pair
// Related: ts_narrow.go -- oneTransportPair and recordInitiatorSelectors, the producers under test
// Related: rfc4301_child_multi_pair_test.go -- the transport-mode state selector these keep exact
// RFC: rfc/short/rfc7296.md -- Traffic Selector negotiation (Section 2.9)

package engine

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// transportTwoPairs builds a TSi/TSr proposal of two pairs between the transportPeer
// hosts: TCP from initiator port 179 first, UDP from initiator port 3784 second. TSi is
// the initiator's side (10.0.0.2 when this node responds, 10.0.0.1 when it initiates).
func transportTwoPairs(t *testing.T, initiator, responder string) (*wire.PayloadTS, *wire.PayloadTS) {
	t.Helper()
	tsi := &wire.PayloadTS{TSPayloadType: wire.PayloadTypeTSi, TrafficSelectors: []wire.TrafficSelector{
		wireSel(t, initiator, 179, 179, 6),
		wireSel(t, initiator, 3784, 3784, 17),
	}}
	tsr := &wire.PayloadTS{TSPayloadType: wire.PayloadTypeTSr, TrafficSelectors: []wire.TrafficSelector{
		wireSel(t, responder, 0, 65535, 6),
		wireSel(t, responder, 0, 65535, 17),
	}}
	return tsi, tsr
}

// VALIDATES: a transport-mode responder answers a two-pair proposal with exactly one
// pair, the initiator's first choice.
// PREVENTS: a transport-mode SA negotiated over several pairs, which its one inbound
// state selector can hold only as a cover wider than the pairs.
//
// Goal: RFC 7296 Section 2.9 narrowing, chosen so the SAD entry stays exact. Method: an
// unconfigured transport peer (which allows the whole proposal) proposes TCP/179 and
// UDP/3784 between the two hosts; narrowChildSelectors, the entry point every responder
// producer calls, must record the TCP pair only. A tunnel-mode control keeps both
// pairs, so the narrowing is the transport-mode decision and not a general one.
func TestRFC7296TransportResponderNarrowsToFirstPair(t *testing.T) {
	// RFC requirement: RFC7296-2.9-2 positive -- a transport-mode responder whose policy accepts a two-pair proposal narrows it to exactly one pair, and that pair is the initiator's first choice (the TCP/179 pair).
	tsi, tsr := transportTwoPairs(t, "10.0.0.2/32", "10.0.0.1/32")
	sa := &SA{PeerCfg: transportPeer(), UseTransportMode: true}
	if err := narrowChildSelectors(sa, tsi, tsr, nil); err != nil {
		t.Fatalf("narrowChildSelectors: %v", err)
	}
	if len(sa.NegotiatedPairs) != 1 {
		t.Fatalf("transport-mode answer holds %d pairs %s, want 1", len(sa.NegotiatedPairs), pairsText(sa.NegotiatedPairs))
	}
	first := sa.NegotiatedPairs[0]
	if first.I.Proto != 6 || first.I.Port.Port != 179 {
		t.Errorf("transport-mode answer kept protocol %d port %d, want the first choice TCP/179", first.I.Proto, first.I.Port.Port)
	}

	tunnel := &SA{PeerCfg: transportPeer()}
	tsi, tsr = transportTwoPairs(t, "10.0.0.2/32", "10.0.0.1/32")
	if err := narrowChildSelectors(tunnel, tsi, tsr, nil); err != nil {
		t.Fatalf("narrowChildSelectors (tunnel): %v", err)
	}
	if len(tunnel.NegotiatedPairs) != 2 {
		t.Errorf("tunnel-mode answer holds %d pairs, want both: the per-pair policies hold them exactly", len(tunnel.NegotiatedPairs))
	}
}

// VALIDATES: a transport-mode initiator refuses a responder answer of several pairs, and
// the IKE_AUTH adoption deletes the SA with TS_UNACCEPTABLE rather than install it.
// PREVENTS: installing a transport-mode state whose one selector covers more than the
// negotiated pairs, through which a decrypted packet in no pair reaches a bypass entry.
//
// Goal: the initiator side of the same exactness. Method: an initiator in transport mode
// adopts a two-pair answer through adoptAuthResponseNegotiation, the IKE_AUTH entry
// point, and through recordInitiatorSelectors, the producer every response path calls;
// both must refuse, with errTSTransportPairs and no recorded pair. A one-pair answer is
// adopted, and a tunnel-mode initiator adopts both pairs.
func TestRFC4301TransportInitiatorRefusesSeveralPairs(t *testing.T) {
	// RFC requirement: RFC4301-4.4.2-2 negative -- a transport-mode initiator answered with two selector pairs refuses the answer and records no negotiated pair, so no SAD entry wider than the negotiated values is built.
	tsi, tsr := transportTwoPairs(t, "10.0.0.1/32", "10.0.0.2/32")
	sa := &SA{PeerCfg: transportPeer(), IsInitiator: true, UseTransportMode: true}
	err := recordInitiatorSelectors(sa, tsi, tsr, nil)
	if !errors.Is(err, errTSTransportPairs) {
		t.Fatalf("two-pair transport answer: err = %v, want errTSTransportPairs", err)
	}
	if len(sa.NegotiatedPairs) != 0 {
		t.Errorf("refused answer still recorded %s", pairsText(sa.NegotiatedPairs))
	}
	if got := childResponseRefusedFix(err); got != childResponseFixTransportPairs {
		t.Errorf("the refusal names the fix %q, want the transport-pairs fix", got)
	}

	auth := &SA{PeerCfg: transportPeer(), IsInitiator: true}
	ok, notify := adoptAuthResponseNegotiation(auth, true, tsi, tsr, slogutil.DiscardLogger())
	if ok || notify != wire.NotifyTSUnacceptable {
		t.Errorf("IKE_AUTH adoption = (%v, %d), want (false, TS_UNACCEPTABLE %d)", ok, notify, wire.NotifyTSUnacceptable)
	}

	one := &SA{PeerCfg: transportPeer(), IsInitiator: true, UseTransportMode: true}
	oneI := &wire.PayloadTS{TSPayloadType: wire.PayloadTypeTSi, TrafficSelectors: tsi.TrafficSelectors[:1]}
	oneR := &wire.PayloadTS{TSPayloadType: wire.PayloadTypeTSr, TrafficSelectors: tsr.TrafficSelectors[:1]}
	if err := recordInitiatorSelectors(one, oneI, oneR, nil); err != nil {
		t.Fatalf("one-pair transport answer refused: %v", err)
	}
	if len(one.NegotiatedPairs) != 1 {
		t.Errorf("one-pair transport answer recorded %d pairs, want 1", len(one.NegotiatedPairs))
	}

	tunnel := &SA{PeerCfg: transportPeer(), IsInitiator: true}
	if err := recordInitiatorSelectors(tunnel, tsi, tsr, nil); err != nil {
		t.Fatalf("two-pair tunnel answer refused: %v", err)
	}
	if len(tunnel.NegotiatedPairs) != 2 {
		t.Errorf("two-pair tunnel answer recorded %d pairs, want 2", len(tunnel.NegotiatedPairs))
	}
}
