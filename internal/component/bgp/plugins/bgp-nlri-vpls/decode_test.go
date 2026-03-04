package bgp_nlri_vpls

import (
	"encoding/json"
	"testing"
)

// TestDecodeNLRIHex tests VPLS NLRI hex decoding.
//
// VALIDATES: DecodeNLRIHex produces correct JSON for valid VPLS NLRI.
// PREVENTS: Regression in VPLS decode pipeline (hex→parse→JSON).
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
			name:      "vpls rd",
			family:    "l2vpn/vpls",
			hex:       "00110000fde9000000640005006400c803e801",
			wantKey:   "rd",
			wantValue: "0:65001:100",
		},
		{
			name:      "vpls ve-id",
			family:    "l2vpn/vpls",
			hex:       "00110000fde9000000640005006400c803e801",
			wantKey:   "ve-id",
			wantValue: float64(5),
		},
		{
			name:      "vpls label-base",
			family:    "l2vpn/vpls",
			hex:       "00110000fde9000000640005006400c803e801",
			wantKey:   "label-base",
			wantValue: float64(16000),
		},
		{
			name:    "unsupported family",
			family:  "ipv4/unicast",
			hex:     "00110000fde9000000640005006400c803e801",
			wantErr: true,
		},
		{
			name:    "invalid hex",
			family:  "l2vpn/vpls",
			hex:     "ZZZZ",
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
