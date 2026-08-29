// Design: docs/architecture/ospf/ospf-2-wire.md -- shared test fixtures

package packet

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func mustRouterID(t *testing.T, s string) types.RouterID {
	t.Helper()
	id, err := types.ParseRouterID(s)
	if err != nil {
		t.Fatalf("ParseRouterID(%q): %v", s, err)
	}
	return id
}

func mustAreaID(t *testing.T, s string) types.AreaID {
	t.Helper()
	id, err := types.ParseAreaID(s)
	if err != nil {
		t.Fatalf("ParseAreaID(%q): %v", s, err)
	}
	return id
}

func mustLSID(t *testing.T, s string) types.LinkStateID {
	t.Helper()
	id, err := types.ParseLinkStateID(s)
	if err != nil {
		t.Fatalf("ParseLinkStateID(%q): %v", s, err)
	}
	return id
}

func mustAge(t *testing.T, seconds uint16) types.LSAge {
	t.Helper()
	return types.LSAge(seconds)
}

func mustMetric(t *testing.T, cost uint32) types.Metric {
	t.Helper()
	metric, err := types.NewMetric(cost)
	if err != nil {
		t.Fatalf("NewMetric(%d): %v", cost, err)
	}
	return metric
}

func sampleHeader(t *testing.T, pt PacketType) Header {
	t.Helper()
	return Header{
		Type:     pt,
		RouterID: mustRouterID(t, "10.0.0.1"),
		AreaID:   mustAreaID(t, "0"),
		AuType:   AuTypeNull,
	}
}

func sampleLSAHeader(t *testing.T, typ types.LSType, lsid string) LSAHeader {
	t.Helper()
	return LSAHeader{
		Age:               mustAge(t, 10),
		Options:           types.OptionE | types.OptionO,
		Type:              typ,
		LinkStateID:       mustLSID(t, lsid),
		AdvertisingRouter: mustRouterID(t, "10.0.0.1"),
		Sequence:          types.InitialSequenceNumber,
	}
}

func encodePacket(t *testing.T, p Packet) []byte {
	t.Helper()
	buf := make([]byte, p.EncodedLen())
	n := (&p).WriteTo(buf, 0)
	if n != len(buf) {
		t.Fatalf("Packet.WriteTo wrote %d, want %d", n, len(buf))
	}
	return buf
}

func encodeLSA(t *testing.T, lsa LSA) []byte {
	t.Helper()
	buf := make([]byte, lsa.EncodedLen())
	n := (&lsa).WriteTo(buf, 0)
	if n != len(buf) {
		t.Fatalf("LSA.WriteTo wrote %d, want %d", n, len(buf))
	}
	return buf
}

func sampleHello(t *testing.T) Hello {
	t.Helper()
	return Hello{
		NetworkMask:   [4]byte{255, 255, 255, 0},
		HelloInterval: 10,
		Options:       types.OptionE | types.OptionO,
		Priority:      1,
		DeadInterval:  40,
		DR:            [4]byte{10, 0, 0, 1},
		BDR:           [4]byte{10, 0, 0, 2},
		Neighbors:     []types.RouterID{mustRouterID(t, "10.0.0.2"), mustRouterID(t, "10.0.0.3")},
	}
}

func sampleRouterLSA(t *testing.T) LSA {
	t.Helper()
	return LSA{
		Header: sampleLSAHeader(t, types.LSTypeRouter, "10.0.0.1"),
		Router: &RouterLSA{
			Flags: RouterFlagB | RouterFlagE,
			Links: []RouterLink{
				{LinkID: mustLSID(t, "10.0.0.2"), LinkData: [4]byte{10, 0, 0, 1}, Type: RouterLinkTypeP2P, Metric: mustMetric(t, 10)},
				{LinkID: mustLSID(t, "10.0.0.254"), LinkData: [4]byte{10, 0, 0, 1}, Type: RouterLinkTypeTransit, Metric: mustMetric(t, 65535)},
				{LinkID: mustLSID(t, "192.0.2.0"), LinkData: [4]byte{255, 255, 255, 0}, Type: RouterLinkTypeStub, Metric: mustMetric(t, 20)},
				{LinkID: mustLSID(t, "10.0.0.4"), LinkData: [4]byte{10, 0, 0, 1}, Type: RouterLinkTypeVirtual, Metric: mustMetric(t, 30)},
			},
		},
	}
}

func sampleNetworkLSA(t *testing.T) LSA {
	t.Helper()
	return LSA{
		Header: sampleLSAHeader(t, types.LSTypeNetwork, "10.0.0.254"),
		Network: &NetworkLSA{
			NetworkMask:     [4]byte{255, 255, 255, 0},
			AttachedRouters: []types.RouterID{mustRouterID(t, "10.0.0.1"), mustRouterID(t, "10.0.0.2")},
		},
	}
}

func sampleSummaryLSA(t *testing.T, typ types.LSType) LSA {
	t.Helper()
	return LSA{
		Header:  sampleLSAHeader(t, typ, "192.0.2.0"),
		Summary: &SummaryLSA{NetworkMask: [4]byte{255, 255, 255, 0}, Metric: SummaryMetricMax},
	}
}

func sampleExternalLSA(t *testing.T, typ types.LSType) LSA {
	t.Helper()
	h := sampleLSAHeader(t, typ, "203.0.113.0")
	if typ == types.LSTypeNSSA {
		h.Options = h.Options.Set(types.OptionNP)
	}
	return LSA{
		Header: h,
		External: &ExternalLSA{
			NetworkMask:      [4]byte{255, 255, 255, 0},
			ExternalType2:    true,
			Metric:           ExternalMetricMax,
			ForwardingAddr:   [4]byte{0, 0, 0, 0},
			ExternalRouteTag: 0xfeedcafe,
		},
	}
}

// fletcherSums runs the ISO Fletcher accumulation of RFC 905 Annex B over the covered
// region of an LSA, which is the LSA from the Options field on (the two LS Age octets
// excluded). RFC 2328 Section 12.1.7 chooses the two LS Checksum octets so that both
// sums are zero over that region, so this is the property a correct LS checksum has.
//
// It exists so a test can judge an LS checksum without calling the code that produced
// it. FletcherChecksum and FletcherVerify share fletcherModulus, so a modulus wrong in
// both agrees with itself while the wire bytes disagree with every other OSPF speaker.
// The 255 below is written out here for that reason.
func fletcherSums(covered []byte) (int, int) {
	c0, c1 := 0, 0
	for _, b := range covered {
		c0 = (c0 + int(b)) % 255
		c1 = (c1 + c0) % 255
	}
	return c0, c1
}
