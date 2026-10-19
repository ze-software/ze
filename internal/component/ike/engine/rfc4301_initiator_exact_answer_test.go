// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the initiator installs the answer exactly
// RFC: rfc/short/rfc4301.md -- RFC4301-4.4.2-2

package engine

import (
	"errors"
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// rangeSel builds one IPv4 answered selector over an arbitrary inclusive address range.
func rangeSel(start, end string, startPort, endPort uint16, proto uint8) wire.TrafficSelector {
	return wire.TrafficSelector{
		TSType:       wire.TSTypeIPv4AddrRange,
		IPProtocol:   proto,
		StartPort:    startPort,
		EndPort:      endPort,
		StartAddress: net.ParseIP(start).To4(),
		EndAddress:   net.ParseIP(end).To4(),
	}
}

func tsOf(payloadType uint8, sels ...wire.TrafficSelector) *wire.PayloadTS {
	return &wire.PayloadTS{TSPayloadType: payloadType, TrafficSelectors: sels}
}

// TestRFC4301InitiatorInstallsAnAddressRangeExactly drives recordInitiatorSelectors, the
// producer of NegotiatedPairs on every initiator path (IKE_AUTH, creation, rekey).
//
// VALIDATES: an answered address range that is not one prefix is recorded as the exact
// set of prefixes tiling it, one pair each, so the policies cover the whole negotiated
// range and nothing outside it.
// PREVENTS: the range being cut to the largest prefix at its start (10.1.1.5-10.1.1.9
// became 10.1.1.5/32), which installed less traffic than both ends negotiated.
func TestRFC4301InitiatorInstallsAnAddressRangeExactly(t *testing.T) {
	// RFC requirement: RFC4301-4.4.2-2 positive -- an initiator answered with an address range that is not one prefix records exactly the prefixes tiling that range, one negotiated pair each with the answered TSr, so every negotiated address and no other is installed.
	sa := tisInitiator(t, tisPeer(t))
	err := recordInitiatorSelectors(sa,
		tsOf(wire.PayloadTypeTSi, rangeSel("10.1.1.5", "10.1.1.9", 0, 65535, 0)),
		tsPayload(t, wire.PayloadTypeTSr, "10.2.1.0/24"), nil)
	if err != nil {
		t.Fatalf("a narrowed answer inside the proposal was refused: %v", err)
	}
	want := "10.1.1.5/32 <-> 10.2.1.0/24, 10.1.1.6/31 <-> 10.2.1.0/24, 10.1.1.8/31 <-> 10.2.1.0/24"
	if got := pairsText(sa.NegotiatedPairs); got != want {
		t.Errorf("recorded %s, want %s", got, want)
	}

	// A range that is one prefix stays one pair.
	one := tisInitiator(t, tisPeer(t))
	if err := recordInitiatorSelectors(one,
		tsOf(wire.PayloadTypeTSi, rangeSel("10.1.1.0", "10.1.1.255", 0, 65535, 0)),
		tsPayload(t, wire.PayloadTypeTSr, "10.2.1.0/24"), nil); err != nil {
		t.Fatalf("a prefix-shaped answer was refused: %v", err)
	}
	if got := pairsText(one.NegotiatedPairs); got != "10.1.1.0/24 <-> 10.2.1.0/24" {
		t.Errorf("a prefix-shaped answer recorded %s", got)
	}
}

// TestRFC4301InitiatorRefusesAnAnswerItCannotInstallExactly drives the same producer and
// the IKE_AUTH entry point that calls it.
//
// VALIDATES: an answered port range neither dataplane can program (TCP 1024-65535, which
// strongSwan answers for a 1024-65535 policy), and an OPAQUE port pair, are REFUSED:
// TS_UNACCEPTABLE on IKE_AUTH, and no negotiated pair recorded.
// PREVENTS: installing the start port alone (TCP 1024), or dropping the pair it cannot
// program while adopting the rest, both of which install less than was negotiated.
func TestRFC4301InitiatorRefusesAnAnswerItCannotInstallExactly(t *testing.T) {
	// RFC requirement: RFC4301-4.4.2-2 negative -- an initiator answered with a port range that no dataplane can program exactly, or with a pair it cannot program, refuses the answer (TS_UNACCEPTABLE on IKE_AUTH) and records no negotiated pair, rather than installing a narrower selector than was negotiated.
	const tcp = 6
	cases := []struct {
		name     string
		tsi, tsr *wire.PayloadTS
	}{
		{"port range", tsOf(wire.PayloadTypeTSi, wireSel(t, "10.1.1.0/24", 1024, 65535, tcp)),
			tsOf(wire.PayloadTypeTSr, wireSel(t, "10.2.1.0/24", 0, 65535, tcp))},
		{"second pair opaque", tsOf(wire.PayloadTypeTSi,
			wireSel(t, "10.1.1.0/24", 0, 65535, 0), wireSel(t, "10.1.2.0/24", 65535, 0, tcp)),
			tsOf(wire.PayloadTypeTSr, wireSel(t, "10.2.1.0/24", 0, 65535, 0), wireSel(t, "10.2.2.0/24", 0, 65535, tcp))},
	}
	for _, c := range cases {
		sa := tisInitiator(t, tisPeer(t))
		err := recordInitiatorSelectors(sa, c.tsi, c.tsr, nil)
		if !errors.Is(err, errTSUnusable) {
			t.Errorf("%s: recordInitiatorSelectors returned %v, want errTSUnusable", c.name, err)
		}
		if sa.NegotiatedPairs != nil {
			t.Errorf("%s: the refused answer recorded %s", c.name, pairsText(sa.NegotiatedPairs))
		}
		auth := tisInitiator(t, tisPeer(t))
		ok, notify := adoptAuthResponseNegotiation(auth, false, true, c.tsi, c.tsr, slogutil.DiscardLogger())
		if ok || notify != wire.NotifyTSUnacceptable {
			t.Errorf("%s: IKE_AUTH adopted=%v notify=%d, want refused with TS_UNACCEPTABLE (%d)",
				c.name, ok, notify, wire.NotifyTSUnacceptable)
		}
	}

	// The discriminator: the same pair with an exact single port is adopted as answered.
	single := tisInitiator(t, tisPeer(t))
	if err := recordInitiatorSelectors(single,
		tsOf(wire.PayloadTypeTSi, wireSel(t, "10.1.1.0/24", 1024, 1024, tcp)),
		tsOf(wire.PayloadTypeTSr, wireSel(t, "10.2.1.0/24", 0, 65535, tcp)), nil); err != nil {
		t.Fatalf("a single-port answer was refused: %v", err)
	}
	if len(single.NegotiatedPairs) != 1 || single.NegotiatedPairs[0].I.Port.Port != 1024 {
		t.Errorf("a single-port answer recorded %d pairs", len(single.NegotiatedPairs))
	}
}

// TestRangePrefixesTilesExactly checks the decomposition at its boundaries.
//
// VALIDATES: the prefixes cover the range with no gap and no overhang, including the top
// of the address space, and a range past the bound is refused rather than cut.
// PREVENTS: an off-by-one that leaves the last address out or reaches one past it.
func TestRangePrefixesTilesExactly(t *testing.T) {
	ip := func(s string) []byte { return net.ParseIP(s).To4() }
	cases := []struct {
		start, end string
		want       string
	}{
		{"10.0.0.0", "10.0.0.0", "[10.0.0.0/32]"},
		{"10.0.0.1", "10.0.0.6", "[10.0.0.1/32 10.0.0.2/31 10.0.0.4/31 10.0.0.6/32]"},
		{"255.255.255.254", "255.255.255.255", "[255.255.255.254/31]"},
		{"0.0.0.0", "255.255.255.255", "[0.0.0.0/0]"},
	}
	for _, c := range cases {
		got, ok := rangePrefixes(ip(c.start), ip(c.end), answerPairsMax)
		if !ok {
			t.Errorf("%s-%s: refused", c.start, c.end)
			continue
		}
		text := "["
		for i, n := range got {
			if i > 0 {
				text += " "
			}
			text += n.String()
		}
		if text+"]" != c.want {
			t.Errorf("%s-%s: got %s], want %s", c.start, c.end, text, c.want)
		}
	}
	if _, ok := rangePrefixes(ip("10.0.0.9"), ip("10.0.0.1"), answerPairsMax); ok {
		t.Error("an empty range was accepted")
	}
	if _, ok := rangePrefixes(ip("0.0.0.1"), ip("255.255.255.254"), 8); ok {
		t.Error("a range needing 62 prefixes was accepted under a bound of 8")
	}
}
