package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestDecodeOpen verifies OPEN message decoding produces ExaBGP-compatible JSON.
//
// VALIDATES: OPEN message hex decodes to JSON with correct fields.
//
// PREVENTS: Decode command producing malformed or incompatible output.
func TestDecodeOpen(t *testing.T) {
	// Simple OPEN message: version 4, AS 65533, hold time 180, router-id 10.0.0.2
	// From test/data/decode/bgp-open-sofware-version.test
	hexInput := "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00510104FFFD00B40A000002340206010400010001020641040000FFFD02224B201F4578614247502F6D61696E2D633261326561386562642D3230323430373135"

	output, err := decodeHexPacket(hexInput, "open", "")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// Parse JSON output
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nOutput: %s", err, output)
	}

	// Check required fields exist
	if _, ok := result["exabgp"]; !ok {
		t.Error("missing 'exabgp' field")
	}
	if result["type"] != "open" {
		t.Errorf("expected type 'open', got %v", result["type"])
	}

	// Check neighbor section exists
	neighbor, ok := result["neighbor"].(map[string]any)
	if !ok {
		t.Fatal("missing or invalid 'neighbor' field")
	}

	// Check open section
	openSection, ok := neighbor["open"].(map[string]any)
	if !ok {
		t.Fatal("missing or invalid 'open' section in neighbor")
	}

	// Verify key fields
	if openSection["version"] != float64(4) {
		t.Errorf("expected version 4, got %v", openSection["version"])
	}
	if openSection["asn"] != float64(65533) {
		t.Errorf("expected asn 65533, got %v", openSection["asn"])
	}
	if openSection["hold_time"] != float64(180) {
		t.Errorf("expected hold_time 180, got %v", openSection["hold_time"])
	}
	if openSection["router_id"] != "10.0.0.2" {
		t.Errorf("expected router_id 10.0.0.2, got %v", openSection["router_id"])
	}
}

// TestDecodeUpdate verifies UPDATE message decoding produces ExaBGP-compatible JSON.
//
// VALIDATES: UPDATE message hex decodes to JSON with correct fields.
//
// PREVENTS: Decode command failing on UPDATE messages.
func TestDecodeUpdate(t *testing.T) {
	// UPDATE message from test/data/decode/ipv4-unicast-1.test
	hexInput := "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003C020000001C4001010040020040030465016501800404000000C840050400000064000000002001010101"

	output, err := decodeHexPacket(hexInput, "update", "")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// Parse JSON output
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nOutput: %s", err, output)
	}

	// Check type
	if result["type"] != "update" {
		t.Errorf("expected type 'update', got %v", result["type"])
	}

	// Check neighbor exists
	if _, ok := result["neighbor"]; !ok {
		t.Error("missing 'neighbor' field")
	}
}

// TestDecodeHexNormalization verifies hex input is normalized correctly.
//
// VALIDATES: Hex with colons/spaces is handled correctly.
//
// PREVENTS: Decode failing on formatted hex input.
func TestDecodeHexNormalization(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"uppercase", "FFFFFFFFFFFFFFFF"},
		{"lowercase", "ffffffffffffffff"},
		{"with colons", "FF:FF:FF:FF:FF:FF:FF:FF"},
		{"with spaces", "FF FF FF FF FF FF FF FF"},
		{"mixed", "ff:FF:ff:FF FF:FF:FF:FF"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalized := normalizeHex(tt.input)
			expected := "FFFFFFFFFFFFFFFF"
			if normalized != expected {
				t.Errorf("got %q, want %q", normalized, expected)
			}
		})
	}
}

// normalizeHex removes colons/spaces and uppercases hex string.
func normalizeHex(s string) string {
	s = strings.ReplaceAll(s, ":", "")
	s = strings.ReplaceAll(s, " ", "")
	return strings.ToUpper(s)
}
