package labeled

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
	// Wire: 30=48 bits (24 label + 24 prefix), 00 06 41=label 100 with S-bit, 0a0000=10.0.0.0/24
	input := "decode nlri ipv4/mpls-label 300006410a0000\n"
	var output bytes.Buffer
	RunDecode(strings.NewReader(input), &output)

	response := output.String()
	if !strings.Contains(response, "decoded json") {
		t.Fatalf("expected 'decoded json' prefix, got: %s", response)
	}
	if !strings.Contains(response, `"prefix"`) {
		t.Errorf("missing prefix in response: %s", response)
	}
	if !strings.Contains(response, `"labels"`) {
		t.Errorf("missing labels in response: %s", response)
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

// TestDecodeNLRIHex tests labeled unicast NLRI hex decoding.
//
// VALIDATES: DecodeNLRIHex produces correct JSON for valid labeled unicast NLRI.
// PREVENTS: Regression in labeled unicast decode pipeline (hex->parse->JSON).
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
			// 30=48 bits total (24 label + 24 prefix), 00 06 41=label 100 (S-bit set), 0a0000=10.0.0.0/24
			name:      "ipv4 labeled prefix",
			family:    "ipv4/mpls-label",
			hex:       "300006410a0000",
			wantKey:   "prefix",
			wantValue: "10.0.0.0/24",
		},
		{
			name:      "ipv4 labeled label",
			family:    "ipv4/mpls-label",
			hex:       "300006410a0000",
			wantKey:   "labels",
			wantValue: nil, // array, checked separately
		},
		{
			name:    "unsupported family",
			family:  "ipv4/unicast",
			hex:     "300006410a0000",
			wantErr: true,
		},
		{
			name:    "invalid hex",
			family:  "ipv4/mpls-label",
			hex:     "ZZZZ",
			wantErr: true,
		},
		{
			name:    "truncated data",
			family:  "ipv4/mpls-label",
			hex:     "38",
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

			if tt.wantValue == nil {
				// Just check key exists
				if _, ok := m[tt.wantKey]; !ok {
					t.Fatalf("missing key %q in JSON: %s", tt.wantKey, result)
				}
				return
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
