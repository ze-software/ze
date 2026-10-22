// Design: docs/architecture/api/json-format.md -- normalized OPEN capability values.

package format

import (
	"encoding/hex"
	"testing"
)

// TestDecodeOpenOpaqueCapabilityValues checks that the normalized value excludes
// the capability code and length, for GR, LLGR, an unknown code and an empty
// capability. Their distinct payload shapes catch both missing and extra framing.
func TestDecodeOpenOpaqueCapabilityValues(t *testing.T) {
	for _, tc := range []struct {
		name  string
		code  byte
		value string
	}{
		{"gr-header-only", 64, "0003"},
		{"gr-ipv4", 64, "000300010180"},
		{"gr-dual-family", 64, "00030001018000020180"},
		{"llgr-ipv4", 71, "0001018000003c"},
		{"llgr-dual-family", 71, "0001018000003c0002018000003c"},
		{"unknown", 200, "deadbeef"},
		{"empty-route-refresh", 2, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value, err := hex.DecodeString(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			body := []byte{4, 0, 1, 0, 90, 192, 0, 2, 1,
				byte(4 + len(value)), 2, byte(2 + len(value)), tc.code, byte(len(value))}
			body = append(body, value...)
			decoded := DecodeOpen(body)
			if len(decoded.Capabilities) != 1 {
				t.Fatalf("decoded capabilities = %v, want one", decoded.Capabilities)
			}
			got := decoded.Capabilities[0]
			if got.Code != tc.code {
				t.Errorf("capability code = %d, want %d", got.Code, tc.code)
			}
			if got.Value != tc.value {
				t.Errorf("capability %d value = %q, want payload-only %q", tc.code, got.Value, tc.value)
			}
		})
	}
}

// TestDecodeOpenExtendedParams checks that an OPEN in the RFC 9072 extended
// envelope lists its capabilities. Each parameter there has a two-octet length,
// so a reader of one-octet lengths finds none of them: the OPEN ExaBGP 5.0
// sends with twenty families came out with an empty capability list. The two
// capabilities sit in two parameters, so a reader that stops after the first
// is caught too.
//
// RFC 9072 Section 2: "the Parameter Length field is extended to be a
// two-octet unsigned integer."
func TestDecodeOpenExtendedParams(t *testing.T) {
	body, err := hex.DecodeString("04" + "0001" + "005a" + "c0000201" + "ff" + "ff" + "0012" +
		"020006" + "010400010001" + // Multiprotocol ipv4/unicast
		"020006" + "41040000fde9") // Four-octet AS 65001
	if err != nil {
		t.Fatal(err)
	}
	decoded := DecodeOpen(body)
	if len(decoded.Capabilities) != 2 {
		t.Fatalf("decoded capabilities = %v, want multiprotocol and asn4", decoded.Capabilities)
	}
	if got := decoded.Capabilities[0]; got.Name != "multiprotocol" || got.Value != "ipv4/unicast" {
		t.Errorf("first capability = %+v, want multiprotocol ipv4/unicast", got)
	}
	if decoded.ASN != 65001 {
		t.Errorf("ASN = %d, want 65001 from the Four-octet AS capability", decoded.ASN)
	}
}

// TestDecodeOpenEmptyExtendedNextHop checks that an Extended Next Hop
// capability listing no family is still listed, with no value, as an empty
// Paths-Limit is. ExaBGP 4.2 and 5.0 send one, and a script reading the OPEN
// sees `nexthop()` there; dropping it hid a capability the peer sent.
func TestDecodeOpenEmptyExtendedNextHop(t *testing.T) {
	body := []byte{4, 0, 1, 0, 90, 192, 0, 2, 1, 4, 2, 2, 5, 0}
	decoded := DecodeOpen(body)
	if len(decoded.Capabilities) != 1 {
		t.Fatalf("decoded capabilities = %v, want the empty extended next hop", decoded.Capabilities)
	}
	if got := decoded.Capabilities[0]; got.Code != 5 || got.Value != "" {
		t.Errorf("capability = %+v, want code 5 with no value", got)
	}
}
