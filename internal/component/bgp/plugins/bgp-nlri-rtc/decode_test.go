package bgp_nlri_rtc

import (
	"encoding/json"
	"testing"
)

// TestDecodeNLRIHex tests RTC NLRI hex decoding.
//
// VALIDATES: DecodeNLRIHex produces correct JSON for valid RTC NLRI.
// PREVENTS: Regression in RTC decode pipeline (hex→parse→JSON).
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
			name:      "rtc origin-as",
			family:    "ipv4/rtc",
			hex:       "600000fde900020000fde90064",
			wantKey:   "origin-as",
			wantValue: float64(65001),
		},
		{
			name:      "rtc is-default false",
			family:    "ipv4/rtc",
			hex:       "600000fde900020000fde90064",
			wantKey:   "is-default",
			wantValue: false,
		},
		{
			name:    "unsupported family",
			family:  "ipv6/rtc",
			hex:     "600000fde900020000fde90064",
			wantErr: true,
		},
		{
			name:    "invalid hex",
			family:  "ipv4/rtc",
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
