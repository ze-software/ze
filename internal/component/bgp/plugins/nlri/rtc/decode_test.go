package rtc

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
	input := "decode nlri ipv4/rtc 600000fde900020000fde90064\n"
	var output bytes.Buffer
	RunDecode(strings.NewReader(input), &output)

	response := output.String()
	if !strings.Contains(response, "decoded json") {
		t.Fatalf("expected 'decoded json' prefix, got: %s", response)
	}
	if !strings.Contains(response, `"origin-as"`) {
		t.Errorf("missing origin-as in response: %s", response)
	}
	if !strings.Contains(response, `"is-default"`) {
		t.Errorf("missing is-default in response: %s", response)
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

// TestDecodeNLRIHex tests RTC NLRI hex decoding.
//
// VALIDATES: DecodeNLRIHex produces correct JSON for valid RTC NLRI.
// PREVENTS: Regression in RTC decode pipeline (hex→parse→JSON).
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
