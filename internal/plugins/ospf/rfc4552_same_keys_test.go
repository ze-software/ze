// Design: docs/architecture/ospf/ospf-ext-16-ipsec-auth.md -- OSPFv3 IPsec SA installation.
// Related: ipsec_install.go -- buildIPsecInterfaceSAs, the set of SAs one interface installs.
//
// VALIDATES: RFC 4552 section 7, same SPI and same keys for inbound and outbound: every
// state of an interface's SA set (the two multicast states that serve both directions and
// the inbound unicast state) carries the configured SPI and the configured integrity and
// encryption keys, byte for byte.
// PREVENTS: an installer that keys the inbound state differently from the outbound one.
package ospf

import (
	"bytes"
	"net/netip"
	"testing"
)

// rfc4552AssertSameKeys fails unless every state of the set carries spi and the key
// material of c.
func rfc4552AssertSameKeys(t *testing.T, c ipsecInterfaceConfig, spi uint32) {
	t.Helper()
	set := buildIPsecInterfaceSAs(testIfIndex, netip.MustParseAddr("fe80::1"), c)
	if len(set) != 3 {
		t.Fatalf("SA set holds %d states, want 3", len(set))
	}
	for i := range set {
		if set[i].SPI != spi {
			t.Errorf("state to %v spi = %#x, want %#x", set[i].Dst, set[i].SPI, spi)
		}
		if !bytes.Equal(set[i].AuthKey, c.authKeyBytes()) {
			t.Errorf("state to %v integrity key %x, want the configured %x", set[i].Dst, set[i].AuthKey, c.authKeyBytes())
		}
		if !bytes.Equal(set[i].EncKey, c.encKeyBytes()) {
			t.Errorf("state to %v encryption key %x, want the configured %x", set[i].Dst, set[i].EncKey, c.encKeyBytes())
		}
	}
}

// RFC requirement: RFC4552-7-1 positive -- an ESP interface with an integrity and an
// encryption key installs every state of its SA set, inbound unicast and the two
// bidirectional multicast states, with the one configured SPI and byte-identical keys.
// RFC requirement: RFC4552-7-1 negative -- the shared key is the configured one, not a
// constant: the same interface configured with other keys and another SPI carries those on
// every state, so a state keyed from anything but the configuration fails here.
func TestRFC4552EveryStateSharesSPIAndKeys(t *testing.T) {
	// Goal: same SPI and same keys in both directions. Method: build the whole SA set for
	// two configurations and compare every state's SPI and key bytes with the configuration.
	first := ipsecInterfaceConfig{SPI: 256, Protocol: ipsecProtoESP, AuthAlgo: "sha256", AuthKey: hexKey(32), EncAlgo: "aes", EncKey: hexKey(16)}
	if len(first.authKeyBytes()) != 32 || len(first.encKeyBytes()) != 16 {
		t.Fatalf("fixture keys decode to %d and %d bytes, want 32 and 16", len(first.authKeyBytes()), len(first.encKeyBytes()))
	}
	rfc4552AssertSameKeys(t, first, 256)
	second := first
	second.SPI = 300
	second.AuthKey = hexKeyOf("cd", 32)
	second.EncKey = hexKeyOf("ef", 16)
	rfc4552AssertSameKeys(t, second, 300)
}

// hexKeyOf is nBytes of the byte pair octet, hex-encoded.
func hexKeyOf(octet string, nBytes int) string {
	var b bytes.Buffer
	for range nBytes {
		b.WriteString(octet)
	}
	return b.String()
}
