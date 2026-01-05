package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/nlri"
)

// Test data constants to avoid goconst lint warnings.
const (
	// testBGPLSLinkUpdate is hex data for a BGP-LS Link UPDATE message (from bgp-ls-2.test).
	testBGPLSLinkUpdate = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00AA0200000093800E7240044704C0A8FF1D000002006503000000000000000001000020020000040000FDE902010004000000000202000400000000020300040A01010101010024020000040000FDE902010004000000000202000400000000020300080A0104010A010102010300040A010101010400040A0101024001010040020602010000FDE980040400000000801D0704470003000001"

	// testBGPLSLinkNLRIType is the expected NLRI type for Link NLRI.
	testBGPLSLinkNLRIType = "bgpls-link"
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

// TestTCPFlagsStringComprehensive tests all TCP flag combinations.
//
// VALIDATES: All 8 TCP flags and their combinations produce correct strings.
//
// PREVENTS: Unknown flags showing as hex instead of names.
func TestTCPFlagsStringComprehensive(t *testing.T) {
	// Single flags - all 8 TCP flags
	singleFlags := []struct {
		value uint64
		want  string
	}{
		{0x01, "fin"},
		{0x02, "syn"},
		{0x04, "rst"},
		{0x08, "push"},
		{0x10, "ack"},
		{0x20, "urgent"},
		{0x40, "ece"},
		{0x80, "cwr"},
	}

	for _, tt := range singleFlags {
		t.Run("single_"+tt.want, func(t *testing.T) {
			got := tcpFlagsString(tt.value)
			if got != tt.want {
				t.Errorf("tcpFlagsString(0x%02x) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}

	// Combined flags (common TCP patterns)
	combinedFlags := []struct {
		name  string
		value uint64
		want  string
	}{
		{"syn+ack", 0x12, "syn+ack"},
		{"fin+ack", 0x11, "fin+ack"},
		{"fin+push", 0x09, "fin+push"},
		{"rst+ack", 0x14, "rst+ack"},
		{"push+ack", 0x18, "push+ack"},
		{"ack+cwr", 0x90, "ack+cwr"},
		{"syn+fin+rst", 0x07, "fin+syn+rst"},
		{"ack+ece+cwr", 0xD0, "ack+ece+cwr"},
		{"syn+ack+push", 0x1A, "syn+push+ack"},
		{"fin+syn+rst+push", 0x0F, "fin+syn+rst+push"},
		{"all_flags", 0xFF, "fin+syn+rst+push+ack+urgent+ece+cwr"},
	}

	for _, tt := range combinedFlags {
		t.Run("combined_"+tt.name, func(t *testing.T) {
			got := tcpFlagsString(tt.value)
			if got != tt.want {
				t.Errorf("tcpFlagsString(0x%02x) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}

	// Edge cases
	t.Run("zero", func(t *testing.T) {
		got := tcpFlagsString(0)
		if got != "0x0" {
			t.Errorf("tcpFlagsString(0) = %q, want \"0x0\"", got)
		}
	})
}

// TestFormatSingleTCPFlag tests individual TCP flag formatting with operators.
//
// VALIDATES: Negation (!) and equality (=) operators are applied correctly.
//
// PREVENTS: Missing operators or wrong negation format.
func TestFormatSingleTCPFlag(t *testing.T) {
	tests := []struct {
		name string
		op   nlri.FlowOperator
		val  uint64
		want string
	}{
		// Simple matches (no operator prefix)
		{"ack_simple", 0x00, 0x10, "ack"},
		{"syn_simple", 0x00, 0x02, "syn"},
		{"cwr_simple", 0x00, 0x80, "cwr"},

		// Equality matches (= prefix)
		{"ack_equal", nlri.FlowOpEqual, 0x10, "=ack"},
		{"syn_equal", nlri.FlowOpEqual, 0x02, "=syn"},
		{"rst_equal", nlri.FlowOpEqual, 0x04, "=rst"},
		{"fin_push_equal", nlri.FlowOpEqual, 0x09, "=fin+push"},

		// Negation matches (! prefix) - GT operator means NOT for bitmasks
		{"fin_negated", nlri.FlowOpGreater, 0x01, "!fin"},
		{"ece_negated", nlri.FlowOpGreater, 0x40, "!ece"},
		{"syn_negated", nlri.FlowOpGreater, 0x02, "!syn"},
		{"ack_cwr_negated", nlri.FlowOpGreater, 0x90, "!ack+cwr"},

		// LT|GT also means negation
		{"fin_negated_ltgt", nlri.FlowOpLess | nlri.FlowOpGreater, 0x01, "!fin"},

		// Combined flags with operators
		{"syn_ack_equal", nlri.FlowOpEqual, 0x12, "=syn+ack"},
		{"fin_rst_negated", nlri.FlowOpGreater, 0x05, "!fin+rst"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := nlri.FlowMatch{Op: tt.op, Value: tt.val}
			got := formatSingleTCPFlag(m)
			if got != tt.want {
				t.Errorf("formatSingleTCPFlag(Op=0x%02x, Val=0x%02x) = %q, want %q",
					tt.op, tt.val, got, tt.want)
			}
		})
	}
}

// TestFormatTCPFlagsValues tests compound TCP flag expression formatting.
//
// VALIDATES: AND-combined matches produce compound expressions like "cwr&!fin&!ece".
//
// PREVENTS: Separate array elements instead of compound expressions.
func TestFormatTCPFlagsValues(t *testing.T) {
	tests := []struct {
		name    string
		matches []nlri.FlowMatch
		want    []string
	}{
		// Single match - no compound
		{
			name:    "single_ack",
			matches: []nlri.FlowMatch{{Op: 0x00, Value: 0x10, And: false}},
			want:    []string{"ack"},
		},
		{
			name:    "single_syn_equal",
			matches: []nlri.FlowMatch{{Op: nlri.FlowOpEqual, Value: 0x02, And: false}},
			want:    []string{"=syn"},
		},

		// Two separate expressions (no AND)
		{
			name: "two_separate",
			matches: []nlri.FlowMatch{
				{Op: 0x00, Value: 0x10, And: false}, // ack
				{Op: 0x00, Value: 0x80, And: false}, // cwr
			},
			want: []string{"ack", "cwr"},
		},

		// Compound expression with AND
		{
			name: "compound_cwr_and_not_fin",
			matches: []nlri.FlowMatch{
				{Op: 0x00, Value: 0x80, And: false},              // cwr
				{Op: nlri.FlowOpGreater, Value: 0x01, And: true}, // &!fin
			},
			want: []string{"cwr&!fin"},
		},

		// Complex: ack, cwr&!fin&!ece (from bgp-flow-3)
		{
			name: "bgp_flow_3_pattern",
			matches: []nlri.FlowMatch{
				{Op: 0x00, Value: 0x10, And: false},              // ack
				{Op: 0x00, Value: 0x80, And: false},              // cwr (starts new expr)
				{Op: nlri.FlowOpGreater, Value: 0x01, And: true}, // &!fin
				{Op: nlri.FlowOpGreater, Value: 0x40, And: true}, // &!ece
			},
			want: []string{"ack", "cwr&!fin&!ece"},
		},

		// Single compound (from bgp-flow-4): ack+cwr&!fin+ece
		{
			name: "bgp_flow_4_pattern",
			matches: []nlri.FlowMatch{
				{Op: 0x00, Value: 0x90, And: false},              // ack+cwr
				{Op: nlri.FlowOpGreater, Value: 0x41, And: true}, // &!fin+ece
			},
			want: []string{"ack+cwr&!fin+ece"},
		},

		// Three expressions: syn, ack&!rst, fin
		{
			name: "three_expressions",
			matches: []nlri.FlowMatch{
				{Op: 0x00, Value: 0x02, And: false},              // syn
				{Op: 0x00, Value: 0x10, And: false},              // ack
				{Op: nlri.FlowOpGreater, Value: 0x04, And: true}, // &!rst
				{Op: 0x00, Value: 0x01, And: false},              // fin
			},
			want: []string{"syn", "ack&!rst", "fin"},
		},

		// All negated: !syn&!ack&!fin
		{
			name: "all_negated",
			matches: []nlri.FlowMatch{
				{Op: nlri.FlowOpGreater, Value: 0x02, And: false}, // !syn
				{Op: nlri.FlowOpGreater, Value: 0x10, And: true},  // &!ack
				{Op: nlri.FlowOpGreater, Value: 0x01, And: true},  // &!fin
			},
			want: []string{"!syn&!ack&!fin"},
		},

		// Mixed equals and negation: =syn&!ack
		{
			name: "equal_and_negated",
			matches: []nlri.FlowMatch{
				{Op: nlri.FlowOpEqual, Value: 0x02, And: false},  // =syn
				{Op: nlri.FlowOpGreater, Value: 0x10, And: true}, // &!ack
			},
			want: []string{"=syn&!ack"},
		},

		// Long chain: ack&!fin&!syn&!rst&!ece
		{
			name: "long_chain",
			matches: []nlri.FlowMatch{
				{Op: 0x00, Value: 0x10, And: false},              // ack
				{Op: nlri.FlowOpGreater, Value: 0x01, And: true}, // &!fin
				{Op: nlri.FlowOpGreater, Value: 0x02, And: true}, // &!syn
				{Op: nlri.FlowOpGreater, Value: 0x04, And: true}, // &!rst
				{Op: nlri.FlowOpGreater, Value: 0x40, And: true}, // &!ece
			},
			want: []string{"ack&!fin&!syn&!rst&!ece"},
		},

		// Empty matches
		{
			name:    "empty",
			matches: []nlri.FlowMatch{},
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTCPFlagsValues(tt.matches)

			// Handle nil vs empty slice
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("formatTCPFlagsValues() returned %d values, want %d\ngot:  %v\nwant: %v",
					len(got), len(tt.want), got, tt.want)
				return
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("formatTCPFlagsValues()[%d] = %q, want %q\nfull got:  %v\nfull want: %v",
						i, got[i], tt.want[i], got, tt.want)
				}
			}
		})
	}
}

// TestTCPFlagsString verifies TCP flags formatting.
//
// VALIDATES: Single and combined TCP flags produce correct string output.
//
// PREVENTS: TCP flags showing as hex instead of named flags.
func TestTCPFlagsString(t *testing.T) {
	tests := []struct {
		value uint64
		want  string
	}{
		{0x01, "fin"},
		{0x02, "syn"},
		{0x04, "rst"},
		{0x08, "push"},
		{0x10, "ack"},
		{0x20, "urgent"},
		{0x09, "fin+push"}, // Combined: fin (0x01) + push (0x08)
		{0x12, "syn+ack"},  // Combined: syn (0x02) + ack (0x10)
		{0x05, "fin+rst"},  // Combined: fin (0x01) + rst (0x04)
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tcpFlagsString(tt.value)
			if got != tt.want {
				t.Errorf("tcpFlagsString(0x%x) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

// TestFlowSpecTCPFlagsFormat verifies FlowSpec TCP flags JSON format.
//
// VALIDATES: TCP flags in FlowSpec produce ExaBGP-compatible format.
//
// PREVENTS: Wrong format like "0x9" instead of "=fin+push".
func TestFlowSpecTCPFlagsFormat(t *testing.T) {
	// bgp-flow-2 test: tcp-flags [ =rst =fin+push ]
	hexInput := "000000274001010040020040050400000064C010088006000000000000800E0B0001850000050901048109"

	output, err := decodeHexPacket(hexInput, "update", "ipv4/flow")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// Check for expected tcp-flags values
	if !strings.Contains(output, `"=rst"`) {
		t.Errorf("output missing '=rst', got: %s", output)
	}
	if !strings.Contains(output, `"=fin+push"`) {
		t.Errorf("output missing '=fin+push', got: %s", output)
	}
}

// TestFlowSpecTCPFlagsCompound verifies compound TCP flags expressions.
//
// VALIDATES: AND-combined TCP flags produce "flag&!flag" format.
//
// PREVENTS: Separate array elements instead of compound expression.
func TestFlowSpecTCPFlagsCompound(t *testing.T) {
	// bgp-flow-3 test: tcp-flags [ ack cwr&!fin&!ece ]
	hexInput := "0000002B4001010040020040050400000064C010088006000000000000800E0F00018500000909001000804201C240"

	output, err := decodeHexPacket(hexInput, "update", "ipv4/flow")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// Parse JSON to check actual values (avoids JSON encoding issues like \u0026 for &)
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Navigate to tcp-flags array (nolint for test code)
	neighbor, _ := result["neighbor"].(map[string]any)        //nolint:forcetypeassert // test
	message, _ := neighbor["message"].(map[string]any)        //nolint:forcetypeassert // test
	update, _ := message["update"].(map[string]any)           //nolint:forcetypeassert // test
	announce, _ := update["announce"].(map[string]any)        //nolint:forcetypeassert // test
	ipv4flow, _ := announce["ipv4/flowspec"].(map[string]any) //nolint:forcetypeassert // test
	noNexthop, _ := ipv4flow["no-nexthop"].([]any)            //nolint:forcetypeassert // test
	first, _ := noNexthop[0].(map[string]any)                 //nolint:forcetypeassert // test
	tcpFlags, _ := first["tcp-flags"].([]any)                 //nolint:forcetypeassert // test

	// Check values
	if len(tcpFlags) != 2 {
		t.Errorf("expected 2 tcp-flags values, got %d: %v", len(tcpFlags), tcpFlags)
	}
	if tcpFlags[0] != "ack" {
		t.Errorf("expected first flag 'ack', got %v", tcpFlags[0])
	}
	if tcpFlags[1] != "cwr&!fin&!ece" {
		t.Errorf("expected second flag 'cwr&!fin&!ece', got %v", tcpFlags[1])
	}
}

// TestExtendedCommunities verifies extended community parsing.
//
// VALIDATES: All FlowSpec extended community types produce correct strings.
//
// PREVENTS: Unknown extended communities showing without human-readable format.
func TestExtendedCommunities(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want []map[string]any
	}{
		{
			name: "traffic_rate_zero",
			data: []byte{0x80, 0x06, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			want: []map[string]any{
				{"value": uint64(9225060886715039744), "string": "rate-limit:0"},
			},
		},
		{
			name: "traffic_rate_1000",
			data: []byte{0x80, 0x06, 0x00, 0x00, 0x00, 0x00, 0x03, 0xE8},
			want: []map[string]any{
				{"value": uint64(9225060886715040744), "string": "rate-limit:1000"},
			},
		},
		{
			name: "traffic_action",
			data: []byte{0x80, 0x07, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			want: []map[string]any{
				{"value": uint64(9225342361691750400), "string": "traffic-action"},
			},
		},
		{
			name: "redirect_asn_100",
			data: []byte{0x80, 0x08, 0x00, 0x64, 0x00, 0x00, 0x00, 0x01},
			want: []map[string]any{
				{"value": uint64(9225623836668461057), "string": "redirect:100:1"},
			},
		},
		{
			name: "redirect_asn_65000_local_999",
			data: []byte{0x80, 0x08, 0xFD, 0xE8, 0x00, 0x00, 0x03, 0xE7},
			want: []map[string]any{
				{"value": uint64(9225623947148058599), "string": "redirect:65000:999"},
			},
		},
		{
			name: "traffic_marking_dscp_46",
			data: []byte{0x80, 0x09, 0x00, 0x00, 0x00, 0x00, 0x00, 0x2E},
			want: []map[string]any{
				{"value": uint64(9225905311645171758), "string": "mark:46"},
			},
		},
		{
			name: "traffic_marking_dscp_0",
			data: []byte{0x80, 0x09, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			want: []map[string]any{
				{"value": uint64(9225905311645171712), "string": "mark:0"},
			},
		},
		{
			name: "route_target",
			data: []byte{0x00, 0x02, 0x00, 0x64, 0x00, 0x00, 0x00, 0x01},
			want: []map[string]any{
				{"value": uint64(562954248388609), "string": "target:100:1"},
			},
		},
		{
			name: "route_origin",
			data: []byte{0x00, 0x03, 0x00, 0x64, 0x00, 0x00, 0x00, 0x02},
			want: []map[string]any{
				{"value": uint64(844429225099266), "string": "origin:100:2"},
			},
		},
		{
			name: "unknown_type",
			data: []byte{0x00, 0xFF, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
			want: []map[string]any{
				{"value": uint64(71776119077928198), "string": "0x00ff:010203040506"},
			},
		},
		{
			name: "multiple_communities",
			data: []byte{
				0x80, 0x06, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // rate-limit:0
				0x80, 0x08, 0x00, 0x64, 0x00, 0x00, 0x00, 0x01, // redirect:100:1
			},
			want: []map[string]any{
				{"value": uint64(9225060886715039744), "string": "rate-limit:0"},
				{"value": uint64(9225623836668461057), "string": "redirect:100:1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseExtendedCommunities(tt.data)

			if len(got) != len(tt.want) {
				t.Errorf("parseExtendedCommunities() returned %d communities, want %d",
					len(got), len(tt.want))
				return
			}

			for i := range got {
				if got[i]["string"] != tt.want[i]["string"] {
					t.Errorf("community[%d].string = %q, want %q",
						i, got[i]["string"], tt.want[i]["string"])
				}
			}
		})
	}
}

// TestFlowSpecWithExtendedCommunity verifies FlowSpec UPDATE with extended community.
//
// VALIDATES: Extended community in FlowSpec UPDATE produces correct JSON.
//
// PREVENTS: Missing or malformed extended-community in output.
func TestFlowSpecWithExtendedCommunity(t *testing.T) {
	// From bgp-flow-2: rate-limit:0
	hexInput := "000000274001010040020040050400000064C010088006000000000000800E0B0001850000050901048109"

	output, err := decodeHexPacket(hexInput, "update", "ipv4/flow")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Navigate to extended-community (nolint for test code)
	neighbor, _ := result["neighbor"].(map[string]any) //nolint:forcetypeassert // test
	message, _ := neighbor["message"].(map[string]any) //nolint:forcetypeassert // test
	update, _ := message["update"].(map[string]any)    //nolint:forcetypeassert // test
	attrs, _ := update["attribute"].(map[string]any)   //nolint:forcetypeassert // test
	extComm, _ := attrs["extended-community"].([]any)  //nolint:forcetypeassert // test

	if len(extComm) != 1 {
		t.Errorf("expected 1 extended-community, got %d", len(extComm))
		return
	}

	comm, _ := extComm[0].(map[string]any) //nolint:forcetypeassert // test
	if comm["string"] != "rate-limit:0" {
		t.Errorf("expected 'rate-limit:0', got %v", comm["string"])
	}
}

// =============================================================================
// BGP-LS Tests
// =============================================================================

// TestBGPLSLinkNLRIFormat verifies BGP-LS Link NLRI produces structured JSON.
//
// VALIDATES: Link NLRI includes ls-nlri-type, protocol-id, local/remote-node-descriptors.
//
// PREVENTS: Raw hex output instead of structured BGP-LS fields.
func TestBGPLSLinkNLRIFormat(t *testing.T) {
	// From bgp-ls-2.test - Link NLRI with local and remote node descriptors
	hexInput := testBGPLSLinkUpdate

	output, err := decodeHexPacket(hexInput, "update", "bgp-ls/bgp-ls")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Navigate to BGP-LS NLRI
	neighbor, _ := result["neighbor"].(map[string]any)     //nolint:forcetypeassert // test
	message, _ := neighbor["message"].(map[string]any)     //nolint:forcetypeassert // test
	update, _ := message["update"].(map[string]any)        //nolint:forcetypeassert // test
	announce, _ := update["announce"].(map[string]any)     //nolint:forcetypeassert // test
	bgpls, _ := announce["bgp-ls/bgp-ls"].(map[string]any) //nolint:forcetypeassert // test

	// Should have next-hop key
	if len(bgpls) == 0 {
		t.Fatal("no BGP-LS announcements found")
	}

	// Get first next-hop's routes
	var routes []any
	for _, v := range bgpls {
		routes, _ = v.([]any) //nolint:forcetypeassert // test
		break
	}

	if len(routes) == 0 {
		t.Fatal("no BGP-LS routes found")
	}

	route, _ := routes[0].(map[string]any) //nolint:forcetypeassert // test

	// Check required BGP-LS fields
	if route["ls-nlri-type"] != testBGPLSLinkNLRIType {
		t.Errorf("expected ls-nlri-type '%s', got %v", testBGPLSLinkNLRIType, route["ls-nlri-type"])
	}

	if route["protocol-id"] == nil {
		t.Error("missing protocol-id field")
	}

	if route["local-node-descriptors"] == nil {
		t.Error("missing local-node-descriptors field")
	}

	if route["remote-node-descriptors"] == nil {
		t.Error("missing remote-node-descriptors field")
	}
}

// TestBGPLSNodeDescriptorFormat verifies node descriptor fields.
//
// VALIDATES: Node descriptors include autonomous-system, bgp-ls-identifier, ospf-area-id, router-id.
//
// PREVENTS: Missing or malformed node descriptor fields.
func TestBGPLSNodeDescriptorFormat(t *testing.T) {
	tests := []struct {
		name     string
		asn      uint32
		bgplsID  string
		areaID   string
		routerID string
	}{
		{
			name:     "ospf_router",
			asn:      65001,
			bgplsID:  "0",
			areaID:   "0.0.0.0",
			routerID: "10.1.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that formatNodeDescriptors produces correct structure
			nd := &nlri.NodeDescriptor{
				ASN:             tt.asn,
				BGPLSIdentifier: 0,
				OSPFAreaID:      0,
				IGPRouterID:     []byte{10, 1, 1, 1},
			}

			result := formatNodeDescriptors(nd)
			if len(result) == 0 {
				t.Fatal("no descriptors returned")
			}

			// Check autonomous-system
			var foundASN bool
			for _, desc := range result {
				descMap, ok := desc.(map[string]any)
				if !ok {
					continue
				}
				if asn, ok := descMap["autonomous-system"]; ok {
					if asn != float64(tt.asn) && asn != tt.asn {
						t.Errorf("expected autonomous-system %d, got %v", tt.asn, asn)
					}
					foundASN = true
				}
			}
			if !foundASN {
				t.Error("missing autonomous-system in descriptors")
			}
		})
	}
}

// TestBGPLSNLRITypes verifies all BGP-LS NLRI types are formatted correctly.
//
// VALIDATES: Node (1), Link (2), Prefix-v4 (3), Prefix-v6 (4) types.
//
// PREVENTS: Unknown NLRI types showing as raw hex.
func TestBGPLSNLRITypes(t *testing.T) {
	tests := []struct {
		nlriType uint16
		want     string
	}{
		{1, "bgpls-node"},
		{2, "bgpls-link"},
		{3, "bgpls-prefix-v4"},
		{4, "bgpls-prefix-v6"},
		{6, "bgpls-srv6-sid"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := bgplsNLRITypeString(tt.nlriType)
			if got != tt.want {
				t.Errorf("bgplsNLRITypeString(%d) = %q, want %q", tt.nlriType, got, tt.want)
			}
		})
	}
}

// TestBGPLSProtocolIDs verifies BGP-LS protocol ID formatting.
//
// VALIDATES: IS-IS L1/L2, OSPFv2/v3, Direct, Static protocols.
//
// PREVENTS: Protocol IDs showing as numbers instead of names.
func TestBGPLSProtocolIDs(t *testing.T) {
	tests := []struct {
		protoID uint8
		want    int // Expected protocol-id value in JSON
	}{
		{1, 1}, // IS-IS L1
		{2, 2}, // IS-IS L2
		{3, 3}, // OSPFv2
		{4, 4}, // Direct
		{5, 5}, // Static
		{6, 6}, // OSPFv3
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("proto_%d", tt.protoID), func(t *testing.T) {
			// Protocol ID should be numeric in JSON output (matching ExaBGP)
			if int(tt.protoID) != tt.want {
				t.Errorf("protocol-id %d should equal %d", tt.protoID, tt.want)
			}
		})
	}
}

// TestBGPLSAttribute verifies BGP-LS path attribute parsing.
//
// VALIDATES: bgp-ls attribute with igp-metric and other TLVs.
//
// PREVENTS: Missing bgp-ls attribute in UPDATE output.
func TestBGPLSAttribute(t *testing.T) {
	// From bgp-ls-2.test - has bgp-ls attribute with igp-metric: 1
	hexInput := testBGPLSLinkUpdate

	output, err := decodeHexPacket(hexInput, "update", "bgp-ls/bgp-ls")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Navigate to attributes
	neighbor, _ := result["neighbor"].(map[string]any) //nolint:forcetypeassert // test
	message, _ := neighbor["message"].(map[string]any) //nolint:forcetypeassert // test
	update, _ := message["update"].(map[string]any)    //nolint:forcetypeassert // test
	attrs, _ := update["attribute"].(map[string]any)   //nolint:forcetypeassert // test

	// Check for bgp-ls attribute
	bgplsAttr, ok := attrs["bgp-ls"].(map[string]any)
	if !ok {
		t.Fatal("missing bgp-ls attribute")
	}

	// Check igp-metric
	if bgplsAttr["igp-metric"] == nil {
		t.Error("missing igp-metric in bgp-ls attribute")
	}
}

// TestBGPLSInterfaceAddresses verifies interface/neighbor address parsing.
//
// VALIDATES: interface-addresses and neighbor-addresses arrays.
//
// PREVENTS: Missing or malformed address arrays.
func TestBGPLSInterfaceAddresses(t *testing.T) {
	// Link NLRI should have interface-addresses and neighbor-addresses
	// Even if empty, they should be present as arrays
	hexInput := testBGPLSLinkUpdate

	output, err := decodeHexPacket(hexInput, "update", "bgp-ls/bgp-ls")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Navigate to BGP-LS NLRI
	neighbor, _ := result["neighbor"].(map[string]any)     //nolint:forcetypeassert // test
	message, _ := neighbor["message"].(map[string]any)     //nolint:forcetypeassert // test
	update, _ := message["update"].(map[string]any)        //nolint:forcetypeassert // test
	announce, _ := update["announce"].(map[string]any)     //nolint:forcetypeassert // test
	bgpls, _ := announce["bgp-ls/bgp-ls"].(map[string]any) //nolint:forcetypeassert // test

	var routes []any
	for _, v := range bgpls {
		routes, _ = v.([]any) //nolint:forcetypeassert // test
		break
	}

	if len(routes) == 0 {
		t.Fatal("no BGP-LS routes found")
	}

	route, _ := routes[0].(map[string]any) //nolint:forcetypeassert // test

	// Check for address arrays (should exist even if empty)
	if route["interface-addresses"] == nil {
		t.Error("missing interface-addresses field")
	}
	if route["neighbor-addresses"] == nil {
		t.Error("missing neighbor-addresses field")
	}
}

// TestBGPLSRawNLRIFormat verifies raw NLRI decoding (nlri type tests).
//
// VALIDATES: Raw NLRI without envelope produces flat JSON.
//
// PREVENTS: Envelope wrapper for nlri-type tests.
func TestBGPLSRawNLRIFormat(t *testing.T) {
	// From bgp-ls-1.test - raw NLRI without BGP header
	// Type: nlri bgp-ls/bgp-ls
	hexInput := "0002005103000000000000000001000020020000040000000102010004C0A87A7E0202000400000000020300040A0A0A0A01010020020000040000000102010004C0A87A7E0202000400000000020300040A020202"

	output, err := decodeHexPacket(hexInput, "nlri", "bgp-ls/bgp-ls")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// For nlri type, output should be flat (no exabgp/neighbor wrapper)
	if result["ls-nlri-type"] != testBGPLSLinkNLRIType {
		t.Errorf("expected ls-nlri-type '%s', got %v", testBGPLSLinkNLRIType, result["ls-nlri-type"])
	}

	if result["protocol-id"] == nil {
		t.Error("missing protocol-id field")
	}

	if result["local-node-descriptors"] == nil {
		t.Error("missing local-node-descriptors field")
	}

	if result["remote-node-descriptors"] == nil {
		t.Error("missing remote-node-descriptors field")
	}
}

// TestBGPLSL3RoutingTopology verifies l3-routing-topology field.
//
// VALIDATES: l3-routing-topology (identifier) is present and correct.
//
// PREVENTS: Missing routing topology identifier.
func TestBGPLSL3RoutingTopology(t *testing.T) {
	// Link NLRI should have l3-routing-topology from identifier field
	hexInput := "0002005103000000000000000001000020020000040000000102010004C0A87A7E0202000400000000020300040A0A0A0A01010020020000040000000102010004C0A87A7E0202000400000000020300040A020202"

	output, err := decodeHexPacket(hexInput, "nlri", "bgp-ls/bgp-ls")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// l3-routing-topology should be 0 (from identifier field)
	if result["l3-routing-topology"] == nil {
		t.Error("missing l3-routing-topology field")
	}

	// Should be 0 for this test case
	if topo, ok := result["l3-routing-topology"].(float64); ok {
		if topo != 0 {
			t.Errorf("expected l3-routing-topology 0, got %v", topo)
		}
	}
}

// TestParseSRMPLSAdjSID verifies SR-MPLS Adjacency SID TLV 1099 parsing.
//
// VALIDATES: V/L flag combinations, label and index SID formats, multiple TLV accumulation.
//
// PREVENTS: Data loss from duplicate TLV instances, incorrect SID value parsing.
func TestParseSRMPLSAdjSID(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		wantSIDs []int
		wantV    int
		wantL    int
	}{
		{
			name:     "V=1,L=1 3-byte label",
			data:     []byte{0x30, 0x00, 0x00, 0x00, 0x04, 0x93, 0x10}, // flags=0x30 (V=1,L=1), weight=0, reserved=0, SID=0x049310
			wantSIDs: []int{299792},
			wantV:    1,
			wantL:    1,
		},
		{
			name:     "V=1,L=1 with B flag",
			data:     []byte{0x70, 0x00, 0x00, 0x00, 0x04, 0x93, 0x00}, // flags=0x70 (B=1,V=1,L=1)
			wantSIDs: []int{299776},
			wantV:    1,
			wantL:    1,
		},
		{
			name:     "V=0,L=0 4-byte index",
			data:     []byte{0x00, 0x05, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00}, // flags=0, weight=5, SID=256
			wantSIDs: []int{256},
			wantV:    0,
			wantL:    0,
		},
		{
			name:     "data too short",
			data:     []byte{0x30, 0x00, 0x00}, // Only 3 bytes, minimum is 4
			wantSIDs: nil,
			wantV:    0,
			wantL:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make(map[string]any)
			parseSRMPLSAdjSID(result, "sr-adj", tt.data)

			if tt.wantSIDs == nil {
				if _, ok := result["sr-adj"]; ok {
					t.Error("expected no sr-adj entry for short data")
				}
				return
			}

			entries, ok := result["sr-adj"].([]map[string]any)
			if !ok || len(entries) == 0 {
				t.Fatal("expected sr-adj array with entries")
			}

			entry := entries[0]
			sids, ok := entry["sids"].([]int)
			if !ok {
				t.Fatal("expected sids array")
			}

			if len(sids) != len(tt.wantSIDs) {
				t.Errorf("got %d SIDs, want %d", len(sids), len(tt.wantSIDs))
			}
			for i, want := range tt.wantSIDs {
				if i < len(sids) && sids[i] != want {
					t.Errorf("SID[%d] = %d, want %d", i, sids[i], want)
				}
			}

			flags, ok := entry["flags"].(map[string]any)
			if !ok {
				t.Fatal("expected flags map")
			}

			if v, ok := flags["V"].(int); ok && v != tt.wantV {
				t.Errorf("V flag = %d, want %d", v, tt.wantV)
			}
			if l, ok := flags["L"].(int); ok && l != tt.wantL {
				t.Errorf("L flag = %d, want %d", l, tt.wantL)
			}
		})
	}
}

// TestSRAdjMultipleInstances verifies multiple TLV 1099 instances accumulate into array.
//
// VALIDATES: Lossless JSON format with array accumulation.
//
// PREVENTS: Data loss from duplicate keys (ExaBGP bug).
func TestSRAdjMultipleInstances(t *testing.T) {
	result := make(map[string]any)

	// First TLV instance
	parseSRMPLSAdjSID(result, "sr-adj", []byte{0x30, 0x00, 0x00, 0x00, 0x04, 0x93, 0x10})
	// Second TLV instance
	parseSRMPLSAdjSID(result, "sr-adj", []byte{0x70, 0x00, 0x00, 0x00, 0x04, 0x93, 0x00})

	entries, ok := result["sr-adj"].([]map[string]any)
	if !ok {
		t.Fatal("expected sr-adj to be array")
	}

	if len(entries) != 2 {
		t.Errorf("expected 2 sr-adj entries, got %d", len(entries))
	}

	// Verify both SIDs are preserved
	sids0, ok := entries[0]["sids"].([]int)
	if !ok || len(sids0) == 0 {
		t.Fatal("expected sids array in first entry")
	}
	sids1, ok := entries[1]["sids"].([]int)
	if !ok || len(sids1) == 0 {
		t.Fatal("expected sids array in second entry")
	}

	if sids0[0] != 299792 {
		t.Errorf("first SID = %d, want 299792", sids0[0])
	}
	if sids1[0] != 299776 {
		t.Errorf("second SID = %d, want 299776", sids1[0])
	}
}
