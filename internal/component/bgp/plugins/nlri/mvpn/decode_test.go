package mvpn

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
	input := "decode nlri ipv4/mvpn 010c0000fde9000000640a000001\n"
	var output bytes.Buffer
	RunDecode(strings.NewReader(input), &output)

	response := output.String()
	if !strings.Contains(response, "decoded json") {
		t.Fatalf("expected 'decoded json' prefix, got: %s", response)
	}
	// An Intra-AS I-PMSI A-D route is a type ze does not parse, so the members
	// are ExaBGP's GenericMVPN ones: the code, parsed:false, and the octets.
	if !strings.Contains(response, `"code":1`) {
		t.Errorf("missing code in response: %s", response)
	}
	if !strings.Contains(response, `"parsed":false`) {
		t.Errorf("missing parsed in response: %s", response)
	}
	if !strings.Contains(response, `"raw":"010C0000FDE9000000640A000001"`) {
		t.Errorf("missing raw in response: %s", response)
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

// TestDecodeNLRIHex tests MVPN NLRI hex decoding.
//
// VALIDATES: DecodeNLRIHex produces correct JSON for valid MVPN NLRI.
// PREVENTS: Regression in MVPN decode pipeline (hex→parse→JSON).
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
			name:      "ipv4 mvpn type 1",
			family:    "ipv4/mvpn",
			hex:       "010c0000fde9000000640a000001",
			wantKey:   "code",
			wantValue: float64(1),
		},
		{
			// A shared tree join is a route type ze parses, so the RD it carries
			// is published rather than only its octets.
			name:      "ipv4 mvpn rd field",
			family:    "ipv4/mvpn",
			hex:       "06160000FDE80001869F0000FDE8200A63C70120EFFBFFE4",
			wantKey:   "rd",
			wantValue: "0:65000:99999",
		},
		{
			name:    "unsupported family",
			family:  "l2vpn/evpn",
			hex:     "010c0000fde9000000640a000001",
			wantErr: true,
		},
		{
			name:    "invalid hex",
			family:  "ipv4/mvpn",
			hex:     "ZZZZ",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := DecodeNLRIHex(tt.family, tt.hex, false)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
					return
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			raw, err := json.Marshal(result)
			if err != nil {
				t.Fatalf("marshal failed: %v", err)
			}
			var m map[string]any
			if err := json.Unmarshal(raw, &m); err != nil {
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
