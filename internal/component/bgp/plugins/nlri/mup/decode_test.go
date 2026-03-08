package mup

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestRunDecode tests the stdin/stdout decode protocol.
//
// VALIDATES: RunDecode produces "decoded json" with correct fields for valid NLRI.
// PREVENTS: Regression in in-process decode path used by CLI fallback.
func TestRunDecode(t *testing.T) {
	t.Parallel()
	input := "decode nlri ipv4/mup 0100010c0000fde9000000640a000001\n"
	var output bytes.Buffer
	RunDecode(strings.NewReader(input), &output)

	response := output.String()
	if !strings.Contains(response, "decoded json") {
		t.Fatalf("expected 'decoded json' prefix, got: %s", response)
	}
	if !strings.Contains(response, `"route-type"`) {
		t.Errorf("missing route-type in response: %s", response)
	}
	if !strings.Contains(response, `"rd"`) {
		t.Errorf("missing rd in response: %s", response)
	}
}

// TestRunDecodeUnknown tests that unrecognized commands produce "decoded unknown".
//
// VALIDATES: RunDecode handles invalid input gracefully.
// PREVENTS: Panic or hang on malformed protocol input.
func TestRunDecodeUnknown(t *testing.T) {
	t.Parallel()
	input := "invalid command\n"
	var output bytes.Buffer
	RunDecode(strings.NewReader(input), &output)

	response := output.String()
	if !strings.Contains(response, "decoded unknown") {
		t.Fatalf("expected 'decoded unknown', got: %s", response)
	}
}

// TestDecodeNLRIHex tests MUP NLRI hex decoding.
//
// VALIDATES: DecodeNLRIHex produces correct JSON for valid MUP NLRI.
// PREVENTS: Regression in MUP decode pipeline (hex→parse→JSON).
func TestDecodeNLRIHex(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
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
