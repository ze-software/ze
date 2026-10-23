// Design: docs/architecture/wire/attributes.md -- FlowSpec traffic-action extended communities
// Related: flowspec_encode.go -- FlowSpecRedirectIPv6 and ParseFlowSpecRedirectIPv6, under test

package attribute

import (
	"encoding/hex"
	"net/netip"
	"strings"
	"testing"
)

// TestFlowSpecRedirectIPv6Octets checks the rt-redirect-ipv6 community against
// octets built by hand from the layouts of RFC 8956 Section 6.1 and RFC 5701
// Section 2: type 0x00, sub-type 0x0d, the 16-octet address, then the 2-octet
// local administrator. The expected hex is written out rather than computed, so
// a wrong type, a wrong offset or a byte-order slip in the encoder shows here.
func TestFlowSpecRedirectIPv6Octets(t *testing.T) {
	tests := []struct {
		name  string
		addr  string
		value string
		want  string
	}{
		{"documentation prefix, value 100", "2001:db8::1", "100", "000d" + "20010db8000000000000000000000001" + "0064"},
		{"both local octets used", "2a02:b80:0:1::1", "65535", "000d" + "2a020b80000000010000000000000001" + "ffff"},
		{"value zero is a value", "2001:db8::", "0", "000d" + "20010db8000000000000000000000000" + "0000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ec, err := FlowSpecRedirectIPv6(netip.MustParseAddr(tt.addr), tt.value)
			if err != nil {
				t.Fatalf("FlowSpecRedirectIPv6(%s, %s): %v", tt.addr, tt.value, err)
			}
			if got := hex.EncodeToString(ec[:]); got != tt.want {
				t.Fatalf("octets = %s, want %s", got, tt.want)
			}
		})
	}
}

// TestFlowSpecRedirectIPv6RoundTrip decodes a community to the text ze prints
// for it, parses that text back, and requires the same octets: the spelling an
// operator reads in `show` output is one they can send.
func TestFlowSpecRedirectIPv6RoundTrip(t *testing.T) {
	for _, wire := range []string{
		"000d" + "20010db8000000000000000000000001" + "0064",
		"000d" + "2a020b80000000010000000000000001" + "ffff",
		"000d" + "00000000000000000000ffff01020304" + "0001", // an IPv4-mapped IPv6 address
	} {
		raw, err := hex.DecodeString(wire)
		if err != nil {
			t.Fatal(err)
		}
		var received IPv6ExtendedCommunity
		copy(received[:], raw)

		text := received.String()
		if !strings.HasPrefix(text, "redirect:[") {
			t.Fatalf("%s decoded to %q, want the redirect:[...] form", wire, text)
		}
		sent, matched, err := ParseFlowSpecRedirectIPv6(text)
		if err != nil {
			t.Fatalf("ParseFlowSpecRedirectIPv6(%q): %v", text, err)
		}
		if !matched {
			t.Fatalf("ParseFlowSpecRedirectIPv6(%q) did not match the form the decoder writes", text)
		}
		if sent != received {
			t.Fatalf("%q encoded to %x, want %x", text, sent[:], received[:])
		}
	}
}

// TestParseFlowSpecRedirectIPv6Refusals checks that a token which opens as the
// IPv6 redirect and then goes wrong is matched and refused with the reason,
// never handed to the 8-octet parser, and that other tokens are left alone.
func TestParseFlowSpecRedirectIPv6Refusals(t *testing.T) {
	refused := []struct {
		token string
		names string
	}{
		{"redirect:[2001:db8::zz]:100", "not an IPv6 address"},
		{"redirect:[1.2.3.4]:100", "without brackets"},
		{"redirect:[2001:db8::1]:65536", "0-65535"},
		{"redirect:[2001:db8::1]:-1", "0-65535"},
		{"redirect:[2001:db8::1]:", "0-65535"},
		{"redirect:[2001:db8::1]", "redirect:[<IPv6 address>]:<0-65535>"},
		{"redirect:[fe80::1%eth0]:100", "zone"},
	}
	for _, tt := range refused {
		_, matched, err := ParseFlowSpecRedirectIPv6(tt.token)
		if !matched {
			t.Errorf("%q: not matched, want matched and refused", tt.token)
			continue
		}
		if err == nil {
			t.Errorf("%q: accepted, want a refusal naming %q", tt.token, tt.names)
			continue
		}
		if !strings.Contains(err.Error(), tt.names) {
			t.Errorf("%q: error %q does not name %q", tt.token, err, tt.names)
		}
	}

	for _, token := range []string{"redirect:65000:100", "redirect:192.0.2.1:100", "target:65000:1", "redirect-to-nexthop"} {
		if _, matched, err := ParseFlowSpecRedirectIPv6(token); matched || err != nil {
			t.Errorf("%q: matched=%v err=%v, want it left to the other parsers", token, matched, err)
		}
	}
}

// TestFlowSpecRedirectNamesTheIPv6Spelling checks that the 8-octet redirect,
// given an IPv6 administrator, refuses it by naming the bracketed form that
// sends it, so an operator who wrote `redirect 2001:db8::1 100` learns the fix.
func TestFlowSpecRedirectNamesTheIPv6Spelling(t *testing.T) {
	_, err := FlowSpecRedirect("2001:db8::1", "100")
	if err == nil {
		t.Fatal("an IPv6 administrator was accepted by the 8-octet redirect")
	}
	if !strings.Contains(err.Error(), "redirect:[2001:db8::1]:<0-65535>") {
		t.Fatalf("error %q does not name the bracketed spelling", err)
	}
}
