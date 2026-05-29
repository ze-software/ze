package show

import (
	"strings"
	"testing"
)

// VALIDATES: parsePolicyTestArgs extracts direction, filter, hex, and asn4.
// PREVENTS: Argument parsing regressions for show policy test.
func TestParsePolicyTestArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantDir  string
		wantFilt string
		wantHex  string
		wantASN4 bool
		wantErr  bool
	}{
		{
			name:     "export_with_hex",
			args:     []string{"export", "update", "hex", "DEADBEEF"},
			wantDir:  "export",
			wantHex:  "DEADBEEF",
			wantASN4: true,
		},
		{
			name:     "import_with_filter",
			args:     []string{"import", "filter", "DROP_PRIVATE_AS", "update", "hex", "CAFEBABE"},
			wantDir:  "import",
			wantFilt: "DROP_PRIVATE_AS",
			wantHex:  "CAFEBABE",
			wantASN4: true,
		},
		{
			name:     "export_asn4_false",
			args:     []string{"export", "update", "hex", "AABB", "source-asn4", "false"},
			wantDir:  "export",
			wantHex:  "AABB",
			wantASN4: false,
		},
		{
			name:    "missing_direction",
			args:    []string{"update", "hex", "AABB"},
			wantErr: true,
		},
		{
			name:    "missing_hex",
			args:    []string{"export"},
			wantErr: true,
		},
		{
			name:    "missing_filter_name",
			args:    []string{"export", "filter"},
			wantErr: true,
		},
		{
			name:    "missing_asn4_value",
			args:    []string{"export", "update", "hex", "AA", "source-asn4"},
			wantErr: true,
		},
		{
			name:    "invalid_asn4_value",
			args:    []string{"export", "update", "hex", "AA", "source-asn4", "maybe"},
			wantErr: true,
		},
		{
			name:    "empty_args",
			args:    nil,
			wantErr: true,
		},
		{
			// A typo'd keyword must error, not silently leave asn4 at its default.
			name:    "unknown_token_typo",
			args:    []string{"export", "update", "hex", "AABB", "source-asn", "false"},
			wantErr: true,
		},
		{
			name:    "unknown_token_garbage",
			args:    []string{"export", "garbage", "update", "hex", "AABB"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, filt, hexStr, asn4, err := parsePolicyTestArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if dir != tt.wantDir {
				t.Errorf("direction = %q, want %q", dir, tt.wantDir)
			}
			if filt != tt.wantFilt {
				t.Errorf("filter = %q, want %q", filt, tt.wantFilt)
			}
			if hexStr != tt.wantHex {
				t.Errorf("hex = %q, want %q", hexStr, tt.wantHex)
			}
			if asn4 != tt.wantASN4 {
				t.Errorf("asn4 = %v, want %v", asn4, tt.wantASN4)
			}
		})
	}
}

// VALIDATES: handleShowPolicyTest rejects bad hex before calling reactor.
// PREVENTS: Invalid input reaching plugin IPC.
func TestHandleShowPolicyTestRejectsBadHex(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "invalid_hex_chars",
			args:    []string{"export", "update", "hex", "ZZZZ"},
			wantErr: "invalid hex",
		},
		{
			name:    "too_short",
			args:    []string{"export", "update", "hex", "FFFF"},
			wantErr: "too short",
		},
		{
			name: "not_update_type",
			// 19 bytes: 16-byte marker + 2-byte length (0x0013=19) + type=1 (OPEN, not UPDATE)
			args:    []string{"export", "update", "hex", "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001301"},
			wantErr: "not a BGP UPDATE",
		},
		{
			name:    "too_long",
			args:    []string{"export", "update", "hex", strings.Repeat("FF", 65536)},
			wantErr: "too long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ctx with nil reactor to test early rejection before reactor call
			resp, err := handleShowPolicyTest(nil, tt.args)
			if err != nil {
				t.Fatalf("unexpected Go error: %v", err)
			}
			if resp.Status != "error" {
				t.Fatalf("status = %q, want error", resp.Status)
			}
			if resp.Error == "" {
				t.Fatal("expected error message")
			}
			if !strings.Contains(resp.Error, tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", resp.Error, tt.wantErr)
			}
		})
	}
}
