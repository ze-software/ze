// Design: docs/architecture/ospf/ospfv3-5-nssa-redist.md -- OSPFv3 NSSA Type-7 redistribution.
// Related: origination_v6_nssa.go -- forwardingAddressForAF, the producer these tests drive.
//
// VALIDATES: RFC 5838 section 2.6 on the ORIGINATED NSSA-LSA of an IPv4 address family over
// OSPFv3: the Forwarding Address encodes an IPv4 address in the leading 32 bits of the field,
// and the remaining 96 bits are zero.
// PREVENTS: an IPv4 AF instance encoding an IPv6 forwarding address, and trailing bits left
// non-zero behind the IPv4 address.
//
// Each test builds an OSPFv3 engine for an IPv4 AF, attaches it to an NSSA through eth0,
// redistributes one IPv4 prefix through InjectExternal, and decodes the NSSA-LSA the engine
// installed in its LSDB.
package ospf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
)

// rfc5838NSSAArea is the NSSA the engine's only interface, eth0, attaches to.
var rfc5838NSSAArea = types.AreaID{0, 0, 0, 9}

// rfc5838InjectNSSA builds an OSPFv3 engine for af whose eth0 forwarding address is
// ethAddr, redistributes 198.51.100.0/24, and returns the originated NSSA-LSA body.
func rfc5838InjectNSSA(t *testing.T, af addressFamily, ethAddr string) ospfv3packet.ExternalLSA {
	t.Helper()
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.9.1","areas":{"area":{"0.0.0.9":`+
		`{"area-id":"0.0.0.9","area-type":"nssa"}}},"interfaces":{"interface":{"eth0":{"area":"0.0.0.9"}}},`+
		`"redistribute":{"connected":{"source":"connected"}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	eng := newEngineWithCodecAF(transport.New(&fakeBackend{}), v6Codec{}, af)
	eng.setConfig(cfg)
	eng.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: rfc5838NSSAArea}
	addr := netip.MustParseAddr(ethAddr)
	eng.forwardingAddress = func(name string) (netip.Addr, bool) {
		if name != "eth0" {
			return netip.Addr{}, false
		}
		return addr, true
	}

	prefix := netip.MustParsePrefix("198.51.100.0/24")
	if err := eng.InjectExternal(prefix, "connected", 0); err != nil {
		t.Fatalf("InjectExternal: %v", err)
	}
	lsid, ok := eng.redistV6[prefix.Masked()]
	if !ok {
		t.Fatal("InjectExternal allocated no Link State ID for 198.51.100.0/24")
	}
	body, ok := decodeV6External(t, eng, rfc5838NSSAArea, v6NSSAKey(cfg.RouterID, lsid))
	if !ok {
		t.Fatal("no NSSA-LSA originated into the NSSA")
	}
	return body
}

// RFC requirement: RFC5838-2.6-1 positive -- for the IPv4 unicast AF and for the IPv4 multicast
// AF, the originated NSSA-LSA carries a Forwarding Address that encodes the interface's IPv4
// address 192.0.2.7 in the leading 32 bits of the field.
// RFC requirement: RFC5838-2.6-2 positive -- the originated NSSA-LSA places the IPv4 forwarding
// address in the first 32 bits of the Forwarding Address field and sets every one of the
// remaining 96 bits to zero.
func TestRFC5838IPv4AFNSSAForwardingAddressIsIPv4(t *testing.T) {
	// Goal: the originator encodes an IPv4 forwarding address for both IPv4 AFs. Method: read
	// the 16-byte field of the installed NSSA-LSA byte for byte.
	want := [16]byte{192, 0, 2, 7}
	for _, af := range []addressFamily{afIPv4Unicast, afIPv4Multicast} {
		body := rfc5838InjectNSSA(t, af, "192.0.2.7")
		if !body.HasForwardingAddr {
			t.Fatalf("%s: NSSA-LSA carries no Forwarding Address, want 192.0.2.7", af)
		}
		if body.ForwardingAddr != want {
			t.Fatalf("%s: Forwarding Address = % x, want % x (IPv4 in the first 32 bits, 96 zero bits)", af, body.ForwardingAddr, want)
		}
	}
}

// RFC requirement: RFC5838-2.6-1 negative -- an IPv4 AF instance whose interface offers only an
// IPv6 address (2001:db8:9::1) never encodes that IPv6 address as the Forwarding Address: the
// NSSA-LSA is originated with no Forwarding Address at all.
func TestRFC5838IPv4AFNSSANeverEncodesIPv6ForwardingAddress(t *testing.T) {
	// Goal: the IPv4 AF refuses a non-IPv4 forwarding address. Method: offer an IPv6 address
	// only, then decode the NSSA-LSA.
	for _, af := range []addressFamily{afIPv4Unicast, afIPv4Multicast} {
		body := rfc5838InjectNSSA(t, af, "2001:db8:9::1")
		if body.HasForwardingAddr {
			t.Fatalf("%s: Forwarding Address = % x encoded, want none: an IPv4 AF MUST encode an IPv4 address", af, body.ForwardingAddr)
		}
	}
}
