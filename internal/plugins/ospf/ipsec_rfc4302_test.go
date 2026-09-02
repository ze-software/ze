// VALIDATES: RFC 4302 (IP Authentication Header) at the boundary Ze owns -- the AH
// Security Association and the AH policies the RFC 4552 installer hands the kernel
// dataplane. Linux XFRM performs every per-packet AH obligation; what Ze produces is
// the SA identifier, the address-match indication, the SPI range and the inbound
// require-policy that XFRM performs them against. Each test asserts one of those.
// PREVENTS: an AH interface installing a reserved SPI, a state selector wide enough to
// map non-OSPF traffic to the AH SA, an AH parameter settled anywhere but in the
// operator's configuration, and an interface that reads as protected with no inbound
// require-policy behind it.

package ospf

import (
	"encoding/hex"
	"errors"
	"net"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	ospfv3transport "github.com/ze-software/ze/internal/plugins/ospf/v3/transport"
)

// ahIPsec is the manually keyed AH block RFC 4552 configures on one OSPFv3 interface.
// Ze never negotiates an AH Child SA over IKEv2, so every AH SA it installs has this
// shape: a configured SPI, an HMAC-SHA integrity transform, and no encryption.
func ahIPsec(spi uint32) ipsecInterfaceConfig {
	return ipsecInterfaceConfig{SPI: spi, Protocol: "ah", AuthAlgo: "sha256", AuthKey: hexKey(32)}
}

// ahIface is the AH fixture on the fixed interface "eth1".
func ahIface(spi uint32) interfaceConfig {
	block := ahIPsec(spi)
	return interfaceConfig{Name: "eth1", IPsec: &block}
}

// RFC requirement: RFC4302-2.4-1 positive -- RFC 4302 Section 2.4: "The SPI field is
// mandatory, and this mechanism for mapping inbound traffic to unicast SAs described
// above MUST be supported by all AH implementations." buildIPsecSA gives the state the
// configured SPI, the AH protocol number, and a wildcard :: destination. That is the
// {SPI, protocol} identifier of the section's third search step: a wildcard destination
// matches neither step 1 nor step 2, so the SPI and the protocol are what resolve an
// inbound AH packet to this SA. The body asserts the SPI, the protocol number and the
// wildcard destination of the installed state.
// RFC requirement: RFC4302-2.4-1 negative -- the identifier follows the configuration
// rather than being one constant: a second interface configured with a different SPI
// builds a state carrying that SPI, so an installer writing a fixed SPI fails here.
func TestAHSAIdentifiedBySPIAndProtocol(t *testing.T) {
	sa := buildIPsecSA(testIfIndex, ahIPsec(0x1000))
	if sa.SPI != 0x1000 {
		t.Errorf("AH SA spi = %#x, want 0x1000: the configured SPI is the SA identifier", sa.SPI)
	}
	if sa.Proto != dataplane.ProtoAH {
		t.Errorf("AH SA proto = %d, want %d (AH)", sa.Proto, dataplane.ProtoAH)
	}
	if !sa.Dst.Equal(net.IPv6zero) {
		t.Errorf("AH SA dst = %v, want :: so that the SPI and the protocol resolve it", sa.Dst)
	}
	other := buildIPsecSA(testIfIndex, ahIPsec(0x2000))
	if other.SPI != 0x2000 {
		t.Errorf("second AH SA spi = %#x, want 0x2000: the SPI is configured, not fixed", other.SPI)
	}
}

// RFC requirement: RFC4302-2.4-5 positive -- RFC 4302 Section 2.4: "The indication of
// whether source and destination address matching is required to map inbound IPsec
// traffic to SAs MUST be set either as a side effect of manual SA configuration or via
// negotiation using an SA management protocol, e.g., IKE or Group Domain of
// Interpretation (GDOI) [RFC3547]." Ze negotiates no AH SA, so the manual RFC 4552
// configuration is what sets it: buildIPsecSA gives the AH state an explicit selector,
// and the body asserts that selector exists and carries the ::/0 source and destination
// prefixes that say address matching is not required for this SA.
// RFC requirement: RFC4302-2.4-5 negative -- the indication that is set narrows rather
// than admitting everything: the same selector names upper protocol 89, so traffic that
// is not OSPF cannot map to this AH SA, and an installer that left the selector's upper
// protocol at the 0 wildcard fails here.
func TestAHSAAddressMatchIndication(t *testing.T) {
	sa := buildIPsecSA(testIfIndex, ahIPsec(256))
	if sa.Sel == nil {
		t.Fatal("manual AH SA carries no state selector; the address-match indication is unset")
	}
	assertWildcardV6(t, "AH selector src", sa.Sel.Src)
	assertWildcardV6(t, "AH selector dst", sa.Sel.Dst)
	if sa.Sel.UpperProto != ospfv3transport.Protocol {
		t.Errorf("AH selector upper protocol = %d, want %d (OSPF); the 0 wildcard would map non-OSPF traffic to this SA",
			sa.Sel.UpperProto, ospfv3transport.Protocol)
	}
}

// RFC requirement: RFC4302-2.4-6 negative -- RFC 4302 Section 2.4: "The SPI value of
// zero (0) is reserved for local, implementation-specific use and MUST NOT be sent on
// the wire." An AH interface configured with SPI 0, and one configured with SPI 255,
// are each refused by validateIPsecInterface with ErrIPsecSPIReserved, so no AH SA
// carrying a reserved SPI is installed and none reaches the wire.
// RFC requirement: RFC4302-2.4-6 positive -- the refusal is bounded rather than blanket:
// SPI 256, the first value above the IANA-reserved range, validates and is installable,
// so an implementation that refused every AH SPI fails here.
func TestAHSPIReservedRangeRefused(t *testing.T) {
	for _, c := range []struct {
		spi      int
		reserved bool
	}{{0, true}, {255, true}, {256, false}} {
		cfg, err := parseOSPFConfig(ospfSec(v6IPsecCfg(
			`"protocol":"ah","spi":`+itoa(c.spi)+`,"algorithm":"sha256","key":"`+hexKey(32)+`"`, "")), nil)
		if err != nil {
			t.Fatalf("spi %d parse: %v", c.spi, err)
		}
		err = validateConfig(cfg)
		if c.reserved && !errors.Is(err, ErrIPsecSPIReserved) {
			t.Errorf("ah spi %d: err = %v, want ErrIPsecSPIReserved", c.spi, err)
		}
		if !c.reserved && err != nil {
			t.Errorf("ah spi %d: err = %v, want the first non-reserved SPI accepted", c.spi, err)
		}
	}
}

// RFC requirement: RFC4302-3.4.2-1 positive -- RFC 4302 Section 3.4.2: "If no valid
// Security Association exists for this packet the receiver MUST discard the packet;
// this is an auditable event." Linux XFRM performs that discard, and the state Ze
// installs for it is the inbound require-policy: buildIPsecPolicies emits an SADirIn
// policy carrying the AH protocol, transport mode, upper protocol 89 and the interface
// ifindex, which is what makes the kernel drop an OSPF packet that arrives on this
// interface and resolves to no AH SA. The body asserts that policy's direction,
// protocol, mode, upper-protocol selector and interface scope.
// RFC requirement: RFC4302-3.4.2-1 negative -- the require is scoped rather than
// node-wide: an OSPFv3 interface configured with no ipsec block installs no policy at
// all, so its OSPF is not discarded, and an installer emitting a node-wide inbound
// require fails here.
func TestAHInboundRequirePolicyInstalled(t *testing.T) {
	inst, fake := testInstaller(t, netip.MustParseAddr("fe80::1"))
	inst.setConfig([]interfaceConfig{ahIface(256)})
	inst.onInterfaceUp(testIfIndex, "eth1")

	var inbound *dataplane.SPParams
	for index := range fake.pols {
		if fake.pols[index].Dir == dataplane.SADirIn {
			inbound = &fake.pols[index]
		}
	}
	if inbound == nil {
		t.Fatalf("no inbound policy installed; got %d policies", len(fake.pols))
	}
	if inbound.Proto != dataplane.ProtoAH {
		t.Errorf("inbound policy proto = %d, want %d (AH)", inbound.Proto, dataplane.ProtoAH)
	}
	if inbound.Mode != dataplane.ModeTransport {
		t.Errorf("inbound policy mode = %d, want transport", inbound.Mode)
	}
	if inbound.UpperProto != ospfv3transport.Protocol {
		t.Errorf("inbound policy upper protocol = %d, want %d (OSPF)",
			inbound.UpperProto, ospfv3transport.Protocol)
	}
	if inbound.IfIndex != testIfIndex {
		t.Errorf("inbound policy ifindex = %d, want %d", inbound.IfIndex, testIfIndex)
	}

	plain, plainFake := testInstaller(t, netip.MustParseAddr("fe80::1"))
	plain.setConfig([]interfaceConfig{{Name: "eth1"}})
	plain.onInterfaceUp(testIfIndex, "eth1")
	if len(plainFake.pols) != 0 {
		t.Errorf("interface with no ipsec block installed %d policies, want 0",
			len(plainFake.pols))
	}
}

// RFC requirement: RFC4302-2-2 positive -- RFC 4302 Section 2: "AH does not contain a
// version number, therefore if there are concerns about backward compatibility, they
// MUST be addressed by using a signaling mechanism between the two IPsec peers to
// ensure compatible versions of AH, e.g., IKE [IKEv2] or an out-of-band configuration
// mechanism." Ze runs no AH signaling, so the mechanism it uses is the second one the
// sentence names: the RFC 4552 per-interface ipsec block. The body parses that block
// and asserts every AH parameter -- protocol, SPI, integrity algorithm and key -- is
// read out of the operator's configuration.
// RFC requirement: RFC4302-2-2 negative -- the out-of-band mechanism carries the whole
// agreement rather than part of it: an AH block naming no integrity algorithm is
// refused with ErrIPsecAuthAlgo instead of being given a default, so no AH parameter is
// settled anywhere but in the configuration both peers hold.
func TestAHParametersComeFromConfiguration(t *testing.T) {
	key := hexKey(32) // sha256 = 32 bytes
	cfg, err := parseOSPFConfig(ospfSec(v6IPsecCfg(
		`"protocol":"ah","spi":256,"algorithm":"sha256","key":"`+key+`"`, "")), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	if cfg.V6 == nil || len(cfg.V6.Interfaces) != 1 {
		t.Fatalf("v6 interface not parsed: %+v", cfg.V6)
	}
	block := cfg.V6.Interfaces[0].IPsec
	if block == nil {
		t.Fatal("interface AH block not parsed")
	}
	if block.Protocol != "ah" || block.SPI != 256 || block.AuthAlgo != "sha256" {
		t.Errorf("parsed AH block = %+v, want protocol ah, spi 256, sha256", block)
	}
	want, err := hex.DecodeString(key)
	if err != nil {
		t.Fatalf("decode fixture key: %v", err)
	}
	if len(block.authKeyBytes()) != len(want) {
		t.Errorf("AH integrity key = %d bytes, want %d from the configured hex string",
			len(block.authKeyBytes()), len(want))
	}
	if err := validateConfig(cfg); err != nil {
		t.Fatalf("validateConfig(well-formed AH): %v", err)
	}

	noAlgo, err := parseOSPFConfig(ospfSec(v6IPsecCfg(
		`"protocol":"ah","spi":256,"key":"`+key+`"`, "")), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig(no algorithm): %v", err)
	}
	if err := validateConfig(noAlgo); !errors.Is(err, ErrIPsecAuthAlgo) {
		t.Fatalf("validateConfig(AH with no algorithm) = %v, want ErrIPsecAuthAlgo", err)
	}
}
