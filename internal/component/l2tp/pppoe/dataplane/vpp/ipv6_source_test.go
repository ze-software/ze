// Design: docs/architecture/l2tp/bng-5-pppoe.md -- source identity on a bound PPP link
// VALIDATES: RFC-valid RS and DHCPv6 sources differ from IPv6CP's interface identifier.
package vpp

import (
	"bytes"
	"context"
	"encoding/hex"
	"net/netip"
	"testing"
	"time"
)

// RFC 4861 Section 6.1.1 admits a solicitation through its envelope and body
// checks; it does not require the sender to retain its IPv6CP identifier.
// Literal packets carry independently calculated pseudoheader checksums.
func TestVPPIPv6CarrierAdmitsRouterSolicitationSources(t *testing.T) {
	tests := []struct{ name, packet string }{
		{"unspecified", "6000000000083aff00000000000000000000000000000000ff02000000000000000000000000000285007bb800000000"},
		{"another-link-local", "6000000000083afffe800000000000000000000000000003ff02000000000000000000000000000285007d3400000000"},
		{"global-unicast", "6000000000083aff20010db8000000000000000000000003ff02000000000000000000000000000285004dfc00000000"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			socket, carrier, _ := ipv6Fixture(t)
			socket.dispatchIPv6(ipv6Vector(t, test.packet))
			if len(carrier.rs) != 1 {
				t.Fatal("valid solicitation source was not queued")
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := carrier.ReadRS(ctx); err != nil {
				t.Fatal(err)
			}
			if socket.ipv6DiscardCount() != 0 {
				t.Fatal("valid source discarded")
			}
		})
	}
}

// RFC 9915 Sections 5 and 18.3.10 permit other client source addresses and
// require replying to the source actually received, through the same link.
// Both directions are literal vectors, independent of the production codec.
func TestVPPIPv6CarrierRepliesToActualDHCPv6Source(t *testing.T) {
	tests := []struct{ name, packet, source, response string }{
		{"another-link-local",
			"60000000000c1101fe800000000000000000000000000003ff02000000000000000000000001000202220223000cfd0601000001",
			"[fe80::3]:546",
			"005760000000000d1101fe800000000000000000000000000001fe80000000000000000000000000000302230222000df88707000001ff"},
		{"global-unicast",
			"60000000000c110120010db8000000000000000000000003ff02000000000000000000000001000202220223000ccdce01000001",
			"[2001:db8::3]:546",
			"005760000000000d1101fe80000000000000000000000000000120010db800000000000000000000000302230222000dc94f07000001ff"},
		{"ephemeral-source-port",
			"60000000000c1101fe800000000000000000000000000003ff020000000000000000000000010002dc700223000c22b801000001",
			"[fe80::3]:546",
			"005760000000000d1101fe800000000000000000000000000001fe80000000000000000000000000000302230222000df88707000001ff"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			socket, carrier, endpoint := ipv6Fixture(t)
			socket.dispatchIPv6(ipv6Vector(t, test.packet))
			if len(carrier.dhcp) != 1 {
				t.Fatal("valid DHCPv6 source was not queued")
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			var payload [16]byte
			n, peer, err := carrier.ReadDHCPv6(ctx, payload[:])
			if err != nil {
				t.Fatal(err)
			}
			if peer != netip.MustParseAddrPort(test.source) {
				t.Fatalf("actual source %v", peer)
			}
			if !bytes.Equal(payload[:n], []byte{1, 0, 0, 1}) {
				t.Fatalf("request %x", payload[:n])
			}
			if _, err := carrier.WriteDHCPv6([]byte{7, 0, 0, 1, 255}, peer); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(endpoint.writes[0], ipv6Vector(t, test.response)) {
				t.Fatalf("response %x", endpoint.writes[0])
			}
			if socket.ipv6DiscardCount() != 0 {
				t.Fatal("valid source discarded")
			}
		})
	}
}

// Widening admission to valid unicast sources must retain the negative
// boundary. These invalid sources have correct transport checksums.
func TestVPPIPv6CarrierRejectsNonUnicastControlSources(t *testing.T) {
	for _, packet := range []string{
		"6000000000083affff020000000000000000000000000003ff02000000000000000000000000000285007cb200000000",
		"60000000000c110100000000000000000000000000000000ff02000000000000000000000001000202220223000cfb8a01000001",
		"60000000000c1101ff020000000000000000000000000003ff02000000000000000000000001000202220223000cfc8401000001",
	} {
		socket, carrier, _ := ipv6Fixture(t)
		socket.dispatchIPv6(ipv6Vector(t, packet))
		if len(carrier.rs) != 0 {
			t.Fatal("multicast RS source admitted")
		}
		if len(carrier.dhcp) != 0 {
			t.Fatal("non-unicast DHCPv6 source admitted")
		}
		if socket.ipv6DiscardCount() != 1 {
			t.Fatal("missing source refusal")
		}
	}
}

// An outbound DHCPv6 target is an IPv6 unicast reply address, not an IPv6CP
// identifier. Non-network, multicast and mapped addresses remain refused.
func TestVPPIPv6CarrierRejectsInvalidDHCPv6ReplyTargets(t *testing.T) {
	_, carrier, _ := ipv6Fixture(t)
	for _, target := range []string{"[::]:546", "[ff02::1]:546", "[::1]:546", "[::ffff:192.0.2.1]:546", "192.0.2.1:546", "[fe80::3]:547"} {
		if _, err := carrier.WriteDHCPv6([]byte{7, 0, 0, 1}, netip.MustParseAddrPort(target)); err == nil {
			t.Fatalf("invalid target %s accepted", target)
		}
	}
}

// RFC 4861 and RFC 9915 sources remain negative-space fuzz seeds alongside
// the original negotiated-source packet seeds.
func FuzzVPPIPv6SourceAdmission(f *testing.F) {
	for _, literal := range []string{
		"6000000000083aff20010db8000000000000000000000003ff02000000000000000000000000000285004dfc00000000",
		"60000000000c110120010db8000000000000000000000003ff02000000000000000000000001000202220223000ccdce01000001",
	} {
		packet, err := hex.DecodeString(literal)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(packet)
	}
	f.Fuzz(func(t *testing.T, packet []byte) { socket, _, _ := ipv6Fixture(t); socket.dispatchIPv6(packet) })
}

// RFC 9915 Section 16 removes server-unicast request delivery. This valid
// checksum input must be discarded because its Layer 3 target is unicast.
func TestVPPIPv6CarrierRejectsUnicastDHCPv6Requests(t *testing.T) {
	socket, carrier, _ := ipv6Fixture(t)
	socket.dispatchIPv6(ipv6Vector(t, "60000000000c110120010db8000000000000000000000003fe80000000000000000000000000000102220223000cce5201000001"))
	if len(carrier.dhcp) != 0 {
		t.Fatal("obsolete server-unicast delivery accepted")
	}
	if socket.ipv6DiscardCount() != 1 {
		t.Fatal("unicast delivery refusal not counted")
	}
}
