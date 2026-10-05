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
