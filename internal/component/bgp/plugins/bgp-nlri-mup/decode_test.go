package bgp_nlri_mup

import (
	"encoding/json"
	"testing"
)

// TestDecodeNLRIHex tests MUP NLRI hex decoding.
//
// VALIDATES: DecodeNLRIHex produces correct JSON for valid MUP NLRI.
// PREVENTS: Regression in MUP decode pipeline (hex→parse→JSON).
func TestDecodeNLRIHex(t *testing.T) {
	tests := []struct {
		name      string
		family    string
		hex       string
		wantKey   string
		wantValue any
		wantErr   bool
	}{
		{
			name:      "ipv4 mup type 1 ISD",
			family:    "ipv4/mup",
			hex:       "0100010c0000fde9000000640a000001",
			wantKey:   "route-type",
			wantValue: float64(1),
		},
		{
			name:      "ipv4 mup rd field",
			family:    "ipv4/mup",
			hex:       "0100010c0000fde9000000640a000001",
			wantKey:   "rd",
			wantValue: "0:65001:100",
		},
		{
			name:    "unsupported family",
			family:  "ipv4/unicast",
			hex:     "0100010c0000fde9000000640a000001",
			wantErr: true,
		},
		{
			name:    "invalid hex",
			family:  "ipv4/mup",
			hex:     "ZZZZ",
			wantErr: true,
		},
		{
			name:    "truncated data",
			family:  "ipv4/mup",
			hex:     "01",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DecodeNLRIHex(tt.family, tt.hex)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var m map[string]any
			if err := json.Unmarshal([]byte(result), &m); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			got, ok := m[tt.wantKey]
			if !ok {
				t.Fatalf("missing key %q in JSON: %s", tt.wantKey, result)
			}
			if got != tt.wantValue {
				t.Errorf("key %q = %v, want %v", tt.wantKey, got, tt.wantValue)
			}
		})
	}
}
