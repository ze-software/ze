// Design: docs/architecture/ospf/ospfv3-5-nssa-redist.md -- OSPFv3 NSSA Type-7 redistribution.
// Related: nssa.go -- translateNSSAV6, the producer these tests drive.
//
// VALIDATES: RFC 5838 section 2.6 on the AS-external-LSA an NSSA translator originates for an
// IPv4 address family over OSPFv3: the translated Forwarding Address encodes an IPv4 address in
// the leading 32 bits of the field, and the remaining 96 bits are zero.
// PREVENTS: a translator on an IPv4 AF copying a received NSSA-LSA's IPv6 forwarding address,
// or an IPv4 address with non-zero trailing bits, into its own AS-external-LSA.
package ospf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// rfc5838TranslateIPv4AF runs one translation pass on an elected NSSA ABR of address family af
// over a received P=1 NSSA-LSA for 198.51.100.0/24 whose Forwarding Address field is fa. It
// returns the translated AS-external-LSA body, and false when no Type-5 was originated.
func rfc5838TranslateIPv4AF(t *testing.T, af addressFamily, fa [16]byte) (ospfv3packet.ExternalLSA, bool) {
	t.Helper()
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.9.9","areas":{"area":{`+
		`"0.0.0.0":{"area-id":"0.0.0.0","area-type":"normal"},`+
		`"0.0.0.9":{"area-id":"0.0.0.9","area-type":"nssa","nssa":{"translate-role":"always"}}}},`+
		`"interfaces":{"interface":{"eth0":{"area":"0.0.0.0"},"eth1":{"area":"0.0.0.9"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	eng := newEngineWithCodecAF(transport.New(&fakeBackend{}), v6Codec{}, af)
	eng.setConfig(cfg)
	nssa := types.AreaID{0, 0, 0, 9}
	eng.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: types.BackboneArea}
	eng.running["eth1"] = interfaceConfig{Name: "eth1", AreaID: nssa}
	lsid := v6SummaryLSID(79)
	pfx, ok := netipToV6Prefix(netip.MustParsePrefix("198.51.100.0/24"), 0)
	if !ok {
		t.Fatal("prefix conversion failed")
	}
	pfx.Options |= ospfv3types.OptPrefixP
	body := ospfv3packet.ExternalLSA{Metric: 44, Prefix: pfx, ForwardingAddr: fa, HasForwardingAddr: true}
	installV6NSSAForTest(t, eng, nssa, types.RouterID{10, 0, 9, 2}, lsid, body, 1, ospfv3types.InitialSequenceNumber)

	eng.translateNSSA(transTime)
	return decodeV6External(t, eng, types.BackboneArea, v6ExternalKey(cfg.RouterID, lsid))
}

// RFC requirement: RFC5838-2.6-1 positive -- on the IPv4 unicast AF and on the IPv4 multicast AF,
// the AS-external-LSA translated from a received NSSA-LSA whose Forwarding Address is 192.0.2.7
// carries 192.0.2.7 as an IPv4 address in the leading 32 bits of the field.
// RFC requirement: RFC5838-2.6-2 positive -- that translated AS-external-LSA places the IPv4
// forwarding address in the first 32 bits and every one of the remaining 96 bits is zero.
func TestRFC5838IPv4AFTranslatedForwardingAddressIsIPv4(t *testing.T) {
	// Goal: the translator carries a conforming IPv4 forwarding address. Method: read the
	// 16-byte field of the translated Type-5 byte for byte.
	want := [16]byte{192, 0, 2, 7}
	for _, af := range []addressFamily{afIPv4Unicast, afIPv4Multicast} {
		body, ok := rfc5838TranslateIPv4AF(t, af, want)
		if !ok {
			t.Fatalf("%s: a P=1 NSSA-LSA with IPv4 forwarding address 192.0.2.7 was not translated", af)
		}
		if !body.HasForwardingAddr || body.ForwardingAddr != want {
			t.Fatalf("%s: translated Forwarding Address = % x (present %v), want % x", af, body.ForwardingAddr, body.HasForwardingAddr, want)
		}
	}
}

// RFC requirement: RFC5838-2.6-1 negative -- on an IPv4 AF, a received NSSA-LSA whose Forwarding
// Address is the IPv6 address 2001:db8:9::2 is not translated, so no AS-external-LSA of this
// router encodes an IPv6 forwarding address.
func TestRFC5838IPv4AFTranslatorRefusesIPv6ForwardingAddress(t *testing.T) {
	// Goal: the IPv4 AF translator refuses a non-IPv4 forwarding address. Method: feed an IPv6
	// forwarding address and look for the translated Type-5.
	fa := netip.MustParseAddr("2001:db8:9::2").As16()
	for _, af := range []addressFamily{afIPv4Unicast, afIPv4Multicast} {
		if body, ok := rfc5838TranslateIPv4AF(t, af, fa); ok {
			t.Fatalf("%s: translated AS-external-LSA carries Forwarding Address % x; an IPv4 AF MUST encode an IPv4 address", af, body.ForwardingAddr)
		}
	}
}

// RFC requirement: RFC5838-2.6-2 negative -- on an IPv4 AF, a received NSSA-LSA whose Forwarding
// Address holds 192.0.2.7 in the first 32 bits and a non-zero bit in the remaining 96 is not
// translated, so no AS-external-LSA of this router carries non-zero trailing bits.
func TestRFC5838IPv4AFTranslatorRefusesNonZeroTrailingBits(t *testing.T) {
	// Goal: the IPv4 AF translator refuses non-zero trailing bits. Method: set the last bit of
	// the field behind a valid IPv4 address and look for the translated Type-5.
	fa := [16]byte{192, 0, 2, 7, 15: 1}
	for _, af := range []addressFamily{afIPv4Unicast, afIPv4Multicast} {
		if body, ok := rfc5838TranslateIPv4AF(t, af, fa); ok {
			t.Fatalf("%s: translated AS-external-LSA carries Forwarding Address % x; the bits after the IPv4 address MUST be zero", af, body.ForwardingAddr)
		}
	}
}
