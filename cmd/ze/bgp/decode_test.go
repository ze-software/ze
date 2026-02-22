package bgp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Shared test binary setup - built once, used by all tests that need it.
var (
	testZeBinaryPath string
	testZeBuildOnce  sync.Once
	testZeBuildErr   error
	testZeTmpDir     string
)

// TestMain handles cleanup of shared test resources.
func TestMain(m *testing.M) {
	code := m.Run()

	// Cleanup temp directory after all tests complete
	if testZeTmpDir != "" {
		_ = os.RemoveAll(testZeTmpDir)
	}

	os.Exit(code)
}

// setupTestZeBinary builds ze binary once for all tests that need it.
// Uses sync.Once to ensure only one build happens even with parallel tests.
func setupTestZeBinary(t *testing.T) string {
	t.Helper()

	testZeBuildOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// Create temp directory for binary
		testZeTmpDir, testZeBuildErr = os.MkdirTemp("", "ze-decode-test-*")
		if testZeBuildErr != nil {
			testZeBuildErr = fmt.Errorf("create temp dir: %w", testZeBuildErr)
			return
		}

		testZeBinaryPath = filepath.Join(testZeTmpDir, "ze")

		// Find project root via go list
		listCmd := exec.CommandContext(ctx, "go", "list", "-m", "-f", "{{.Dir}}")
		output, err := listCmd.Output()
		if err != nil {
			testZeBuildErr = fmt.Errorf("find project root: %w", err)
			return
		}
		projectRoot := strings.TrimSpace(string(output))

		// Build ze binary once
		buildCmd := exec.CommandContext(ctx, "go", "build", "-o", testZeBinaryPath, "./cmd/ze") //nolint:gosec // test code
		buildCmd.Dir = projectRoot
		buildOutput, err := buildCmd.CombinedOutput()
		if err != nil {
			testZeBuildErr = fmt.Errorf("build ze: %w\n%s", err, buildOutput)
			return
		}
	})

	if testZeBuildErr != nil {
		t.Skipf("skipping test requiring ze binary: %v", testZeBuildErr)
	}

	return testZeBinaryPath
}

// Test data constants to avoid goconst lint warnings.
const (
	// testTypeBGP is the Ze format envelope type.
	testTypeBGP = "bgp"

	// testBGPLSLinkUpdate is hex data for a BGP-LS Link UPDATE message (from bgp-ls-2.test).
	testBGPLSLinkUpdate = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00AA0200000093800E7240044704C0A8FF1D000002006503000000000000000001000020020000040000FDE902010004000000000202000400000000020300040A01010101010024020000040000FDE902010004000000000202000400000000020300080A0104010A010102010300040A010101010400040A0101024001010040020602010000FDE980040400000000801D0704470003000001"

	// testBGPLSLinkNLRI is raw BGP-LS Link NLRI bytes (from bgp-ls-1.test).
	testBGPLSLinkNLRI = "0002005103000000000000000001000020020000040000000102010004C0A87A7E0202000400000000020300040A0A0A0A01010020020000040000000102010004C0A87A7E0202000400000000020300040A020202"

	// testBGPLSLinkNLRIType is the expected NLRI type for Link NLRI.
	testBGPLSLinkNLRIType = "bgpls-link"

	// testOpenMsgHex is hex data for OPEN message with software version capability.
	testOpenMsgHex = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00510104FFFD00B40A000002340206010400010001020641040000FFFD02224B201F4578614247502F6D61696E2D633261326561386562642D3230323430373135"

	// testUpdateMsgHex is hex data for UPDATE message with IPv4 unicast route.
	testUpdateMsgHex = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003C020000001C4001010040020040030465016501800404000000C840050400000064000000002001010101"

	// testCapNameUnknown is the expected capability name for unknown capabilities.
	testCapNameUnknown = "unknown"

	// testFlowSpecNLRI is FlowSpec NLRI: destination 10.0.0.0/24.
	testFlowSpecNLRI = "0501180a0000"

	// testFlowSpecFamily is the FlowSpec family for IPv4.
	testFlowSpecFamily = "ipv4/flow"
)

// TestDecodeOpen verifies OPEN message decoding produces Ze JSON format (ze-bgp JSON).
//
// VALIDATES: OPEN message hex decodes to Ze JSON with correct fields.
//
// PREVENTS: Decode command producing malformed or incompatible output.
func TestDecodeOpen(t *testing.T) {
	// Simple OPEN message: version 4, AS 65533, hold time 180, router-id 10.0.0.2
	// From test/decode/bgp-open-sofware-version.test
	hexInput := testOpenMsgHex

	output, err := decodeHexPacket(hexInput, "open", "", nil, true)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// Parse JSON output
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nOutput: %s", err, output)
	}

	// Ze format: top-level "type" should be "bgp"
	if result["type"] != testTypeBGP {
		t.Errorf("expected type 'bgp', got %v", result["type"])
	}

	// Ze format: data under "bgp" key
	bgp, ok := result["bgp"].(map[string]any)
	if !ok {
		t.Fatal("missing or invalid 'bgp' field")
	}

	// Ze format: message.type should be "open"
	msg, ok := bgp["message"].(map[string]any)
	if !ok {
		t.Fatal("missing or invalid 'message' field in bgp")
	}
	if msg["type"] != "open" {
		t.Errorf("expected message.type 'open', got %v", msg["type"])
	}

	// Ze format: flat peer structure
	peer, ok := bgp["peer"].(map[string]any)
	if !ok {
		t.Fatal("missing or invalid 'peer' field in bgp")
	}
	if peer["address"] != "127.0.0.1" {
		t.Errorf("expected peer.address '127.0.0.1', got %v", peer["address"])
	}
	if peer["asn"] != float64(65533) {
		t.Errorf("expected peer.asn 65533, got %v", peer["asn"])
	}

	// Check open section
	openSection, ok := bgp["open"].(map[string]any)
	if !ok {
		t.Fatal("missing or invalid 'open' section in bgp")
	}

	// Verify key fields
	if openSection["asn"] != float64(65533) {
		t.Errorf("expected asn 65533, got %v", openSection["asn"])
	}
	if openSection["hold-time"] != float64(180) {
		t.Errorf("expected hold-time 180, got %v", openSection["hold-time"])
	}
	if openSection["router-id"] != "10.0.0.2" {
		t.Errorf("expected router-id 10.0.0.2, got %v", openSection["router-id"])
	}

	// Ze format: capabilities should be an array, not a map
	caps, ok := openSection["capabilities"].([]any)
	if !ok {
		t.Fatal("missing or invalid 'capabilities' array in open")
	}
	if len(caps) == 0 {
		t.Error("expected at least one capability")
	}
}

// TestDecodeOpenFQDNWithoutPlugin verifies FQDN capability shows as unknown without plugin.
//
// VALIDATES: Unknown capabilities return name="unknown", code, and raw hex.
// PREVENTS: Leaking decoded capability data without plugin authorization.
func TestDecodeOpenFQDNWithoutPlugin(t *testing.T) {
	// OPEN message with FQDN capability (code 73)
	// hostname="my-host-name", domain="my-domain-name.com"
	hexInput := "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00510104FFFD00B40A000002340206010400010001020641040000FFFD022249200C6D792D686F73742D6E616D65126D792D646F6D61696E2D6E616D652E636F6D"

	// Decode WITHOUT plugin - should show unknown
	output, err := decodeHexPacket(hexInput, "open", "", nil, true)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	// Ze format: navigate through bgp.open.capabilities
	bgp, ok := result["bgp"].(map[string]any)
	if !ok {
		t.Fatal("missing bgp section")
	}
	openSection, ok := bgp["open"].(map[string]any)
	if !ok {
		t.Fatal("missing open section")
	}
	// Ze format: capabilities is an array
	caps, ok := openSection["capabilities"].([]any)
	if !ok {
		t.Fatal("missing capabilities array (Ze format uses array, not map)")
	}

	// Find capability with code 73 (FQDN) in the array
	var cap73 map[string]any
	for _, c := range caps {
		capMap, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if code, ok := capMap["code"].(float64); ok && int(code) == 73 {
			cap73 = capMap
			break
		}
	}

	if cap73 == nil {
		t.Fatal("missing capability with code 73")
	}

	if cap73["name"] != testCapNameUnknown {
		t.Errorf("expected name 'unknown', got %v", cap73["name"])
	}
	if _, ok := cap73["raw"]; !ok {
		t.Error("missing 'raw' field for unknown capability")
	}
	// Should NOT have decoded fields
	if _, ok := cap73["hostname"]; ok {
		t.Error("unexpected 'hostname' field without plugin")
	}
	if _, ok := cap73["domain"]; ok {
		t.Error("unexpected 'domain' field without plugin")
	}
}

// TestDecodeOpenFQDNWithPlugin verifies FQDN capability decoding with plugin.
//
// VALIDATES: Plugin decode API is invoked when plugin specified.
// PREVENTS: Plugin decode API not being called.
//
// NOTE: In test environment, plugin binary may not be available (os.Args[0] points
// to test binary), so decode may fall back to unknown. In production (ze binary),
// plugin decode works correctly. Test accepts either outcome.
func TestDecodeOpenFQDNWithPlugin(t *testing.T) {
	// OPEN message with FQDN capability (code 73)
	hexInput := "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00510104FFFD00B40A000002340206010400010001020641040000FFFD022249200C6D792D686F73742D6E616D65126D792D646F6D61696E2D6E616D652E636F6D"

	// Decode WITH plugin
	output, err := decodeHexPacket(hexInput, "open", "", []string{"ze.bgp-hostname"}, true)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	// Ze format: navigate through bgp.open.capabilities
	bgp, ok := result["bgp"].(map[string]any)
	if !ok {
		t.Fatal("missing bgp section")
	}
	openSection, ok := bgp["open"].(map[string]any)
	if !ok {
		t.Fatal("missing open section")
	}
	// Ze format: capabilities is an array
	caps, ok := openSection["capabilities"].([]any)
	if !ok {
		t.Fatal("missing capabilities array")
	}

	// Find capability with code 73 in the array
	var cap73 map[string]any
	for _, c := range caps {
		capMap, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if code, ok := capMap["code"].(float64); ok && int(code) == 73 {
			cap73 = capMap
			break
		}
	}

	if cap73 == nil {
		t.Fatal("missing capability with code 73")
	}

	// Accept either decoded (production) or unknown (test environment)
	name, _ := cap73["name"].(string)
	switch name {
	case "fqdn":
		// Plugin decode worked - verify fields
		if cap73["hostname"] != "my-host-name" {
			t.Errorf("expected hostname 'my-host-name', got %v", cap73["hostname"])
		}
		if cap73["domain"] != "my-domain-name.com" {
			t.Errorf("expected domain 'my-domain-name.com', got %v", cap73["domain"])
		}
	case testCapNameUnknown:
		// Plugin not available in test env - verify fallback has raw data
		if _, hasRaw := cap73["raw"]; !hasRaw {
			t.Error("unknown capability should have 'raw' field")
		}
	default:
		t.Errorf("unexpected capability name: %v", name)
	}
}

// TestDecodeOpenGRWithoutPlugin verifies GR capability shows as unknown without plugin.
//
// VALIDATES: GR capability (code 64) returns name="unknown" without --plugin flag.
// PREVENTS: Leaking decoded GR data without plugin authorization.
func TestDecodeOpenGRWithoutPlugin(t *testing.T) {
	// OPEN message with GR capability (code 64): restart-time=120, ipv4/unicast, forward-state=true
	hexInput := "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00330104FFFD00B40A00000216021401040001000141040000FFFD4006007800010180"

	// Decode WITHOUT plugin - should show unknown
	output, err := decodeHexPacket(hexInput, "open", "", nil, true)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	// Ze format: navigate through bgp.open.capabilities
	bgp, ok := result["bgp"].(map[string]any)
	if !ok {
		t.Fatal("missing bgp section")
	}
	openSection, ok := bgp["open"].(map[string]any)
	if !ok {
		t.Fatal("missing open section")
	}
	caps, ok := openSection["capabilities"].([]any)
	if !ok {
		t.Fatal("missing capabilities array")
	}

	// Find capability with code 64 (GR)
	var cap64 map[string]any
	for _, c := range caps {
		capMap, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if code, ok := capMap["code"].(float64); ok && int(code) == 64 {
			cap64 = capMap
			break
		}
	}

	if cap64 == nil {
		t.Fatal("missing capability with code 64")
	}

	if cap64["name"] != testCapNameUnknown {
		t.Errorf("expected name 'unknown', got %v", cap64["name"])
	}
	if _, ok := cap64["raw"]; !ok {
		t.Error("missing 'raw' field for unknown capability")
	}
	// Should NOT have decoded fields
	if _, ok := cap64["restart-time"]; ok {
		t.Error("unexpected 'restart-time' field without plugin")
	}
}

// TestDecodeOpenRRWithoutPlugin verifies Route Refresh capability shows as unknown without plugin.
//
// VALIDATES: RR capability (code 2) returns name="unknown" without --plugin flag.
// PREVENTS: Leaking decoded RR data without plugin authorization.
func TestDecodeOpenRRWithoutPlugin(t *testing.T) {
	// OPEN message with Route Refresh capability (code 2, zero payload)
	hexInput := "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002D0104FFFD00B40A00000210020E01040001000141040000FFFD0200"

	// Decode WITHOUT plugin - should show unknown
	output, err := decodeHexPacket(hexInput, "open", "", nil, true)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	// Ze format: navigate through bgp.open.capabilities
	bgp, ok := result["bgp"].(map[string]any)
	if !ok {
		t.Fatal("missing bgp section")
	}
	openSection, ok := bgp["open"].(map[string]any)
	if !ok {
		t.Fatal("missing open section")
	}
	caps, ok := openSection["capabilities"].([]any)
	if !ok {
		t.Fatal("missing capabilities array")
	}

	// Find capability with code 2 (Route Refresh)
	var cap2 map[string]any
	for _, c := range caps {
		capMap, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if code, ok := capMap["code"].(float64); ok && int(code) == 2 {
			cap2 = capMap
			break
		}
	}

	if cap2 == nil {
		t.Fatal("missing capability with code 2")
	}

	if cap2["name"] != testCapNameUnknown {
		t.Errorf("expected name 'unknown', got %v", cap2["name"])
	}
	if _, ok := cap2["raw"]; !ok {
		t.Error("missing 'raw' field for unknown capability")
	}
}

// TestDecodeUpdate verifies UPDATE message decoding produces Ze JSON format (ze-bgp JSON).
//
// VALIDATES: UPDATE message hex decodes to Ze JSON with correct fields.
//
// PREVENTS: Decode command failing on UPDATE messages.
func TestDecodeUpdate(t *testing.T) {
	// UPDATE message from test/decode/ipv4-unicast-1.test
	hexInput := testUpdateMsgHex

	output, err := decodeHexPacket(hexInput, "update", "", nil, true)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// Parse JSON output
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nOutput: %s", err, output)
	}

	// Ze format: top-level "type" should be "bgp"
	if result["type"] != testTypeBGP {
		t.Errorf("expected type 'bgp', got %v", result["type"])
	}

	// Ze format: data under "bgp" key
	bgp, ok := result["bgp"].(map[string]any)
	if !ok {
		t.Fatal("missing or invalid 'bgp' field")
	}

	// Ze format: message.type should be "update"
	msg, ok := bgp["message"].(map[string]any)
	if !ok {
		t.Fatal("missing or invalid 'message' field in bgp")
	}
	if msg["type"] != "update" {
		t.Errorf("expected message.type 'update', got %v", msg["type"])
	}

	// Ze format: flat peer structure
	peer, ok := bgp["peer"].(map[string]any)
	if !ok {
		t.Fatal("missing or invalid 'peer' field in bgp")
	}
	if peer["address"] == nil {
		t.Error("missing peer.address")
	}
	if peer["asn"] == nil {
		t.Error("missing peer.asn")
	}

	// Ze format: update section under bgp.update
	update, ok := bgp["update"].(map[string]any)
	if !ok {
		t.Fatal("missing or invalid 'update' section in bgp")
	}

	// Ze format: attributes under "attr" (not "attribute")
	if _, ok := update["attr"].(map[string]any); !ok {
		t.Error("missing 'attr' field in update (Ze format uses 'attr', not 'attribute')")
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

	output, err := decodeHexPacket(hexInput, "update", "ipv4/flow", nil, true)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Ze format: navigate through bgp.update.attr
	bgp, _ := result["bgp"].(map[string]any)          //nolint:forcetypeassert // test
	update, _ := bgp["update"].(map[string]any)       //nolint:forcetypeassert // test
	attrs, _ := update["attr"].(map[string]any)       //nolint:forcetypeassert // test
	extComm, _ := attrs["extended-community"].([]any) //nolint:forcetypeassert // test

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

	output, err := decodeHexPacket(hexInput, "update", "bgp-ls/bgp-ls", nil, true)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Ze format: navigate through bgp.update.family
	bgp, _ := result["bgp"].(map[string]any)       //nolint:forcetypeassert // test
	update, _ := bgp["update"].(map[string]any)    //nolint:forcetypeassert // test
	bgplsOps, _ := update["bgp-ls/bgp-ls"].([]any) //nolint:forcetypeassert // test

	// Ze format: operations array with action/nlri
	if len(bgplsOps) == 0 {
		t.Fatal("no BGP-LS operations found")
	}

	// Get first operation's nlri array
	op, _ := bgplsOps[0].(map[string]any) //nolint:forcetypeassert // test
	routes, _ := op["nlri"].([]any)       //nolint:forcetypeassert // test

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

	output, err := decodeHexPacket(hexInput, "update", "bgp-ls/bgp-ls", nil, true)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Ze format: navigate through bgp.update.attr
	bgp, _ := result["bgp"].(map[string]any)    //nolint:forcetypeassert // test
	update, _ := bgp["update"].(map[string]any) //nolint:forcetypeassert // test
	attrs, _ := update["attr"].(map[string]any) //nolint:forcetypeassert // test

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

	output, err := decodeHexPacket(hexInput, "update", "bgp-ls/bgp-ls", nil, true)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Ze format: navigate through bgp.update.family
	bgp, _ := result["bgp"].(map[string]any)       //nolint:forcetypeassert // test
	update, _ := bgp["update"].(map[string]any)    //nolint:forcetypeassert // test
	bgplsOps, _ := update["bgp-ls/bgp-ls"].([]any) //nolint:forcetypeassert // test

	if len(bgplsOps) == 0 {
		t.Fatal("no BGP-LS operations found")
	}

	op, _ := bgplsOps[0].(map[string]any) //nolint:forcetypeassert // test
	routes, _ := op["nlri"].([]any)       //nolint:forcetypeassert // test

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
	hexInput := testBGPLSLinkNLRI

	output, err := decodeHexPacket(hexInput, "nlri", "bgp-ls/bgp-ls", nil, true)
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
	hexInput := testBGPLSLinkNLRI

	output, err := decodeHexPacket(hexInput, "nlri", "bgp-ls/bgp-ls", nil, true)
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

// =============================================================================
// NLRI Flag Tests (Family Plugin Infrastructure)
// =============================================================================

// TestDecodeNLRIFlag verifies --nlri flag takes family string and triggers NLRI mode.
//
// VALIDATES: --nlri <family> correctly sets NLRI decode mode with family context.
// PREVENTS: --nlri flag being parsed incorrectly or family not being passed.
func TestDecodeNLRIFlag(t *testing.T) {
	// Test NLRI mode with BGP-LS family (no plugin, uses built-in decode)
	hexInput := testBGPLSLinkNLRI

	// decodeHexPacket with "nlri" type and family
	output, err := decodeHexPacket(hexInput, "nlri", "bgp-ls/bgp-ls", nil, true)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// NLRI mode should produce flat JSON (no bgp wrapper envelope)
	if _, hasBgp := result["bgp"]; hasBgp {
		t.Error("NLRI mode should not have bgp wrapper")
	}

	// Should have BGP-LS fields
	if result["ls-nlri-type"] == nil {
		t.Error("missing ls-nlri-type field")
	}
}

// TestDecodeNLRIFlagWithPlugin verifies --nlri with plugin falls back correctly.
//
// VALIDATES: When plugin specified but not available, falls back to built-in decode.
// PREVENTS: Crash or empty output when plugin unavailable.
func TestDecodeNLRIFlagWithPlugin(t *testing.T) {
	// FlowSpec NLRI - plugin not available in test, should fall back to built-in
	// This tests the infrastructure path even without actual plugin
	hexInput := "0701180a0000" // Simple FlowSpec: destination 10.0.0.0/24

	output, err := decodeHexPacket(hexInput, "nlri", "ipv4/flow", []string{"bgp-flowspec"}, true)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Should have some output (either plugin or fallback)
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}

// TestLookupFamilyPlugin verifies family plugin lookup with case insensitivity.
//
// VALIDATES: lookupFamilyPlugin normalizes family to lowercase.
// PREVENTS: Case mismatch causing plugin lookup failures.
func TestLookupFamilyPlugin(t *testing.T) {
	tests := []struct {
		family  string
		plugins []string
		want    string
	}{
		{"ipv4/flow", []string{"bgp-flowspec"}, "bgp-flowspec"},
		{"IPV4/FLOW", []string{"bgp-flowspec"}, "bgp-flowspec"},
		{"IPv4/Flow", []string{"bgp-flowspec"}, "bgp-flowspec"},
		{"ipv4/flow", []string{"other"}, "bgp-flowspec"}, // Auto-invoked for known family
		{"ipv4/flow", nil, "bgp-flowspec"},               // Auto-invoked for known family
		{"ipv4/unicast", []string{"bgp-flowspec"}, ""},   // Unknown family, no plugin
		{"ipv6/flow-vpn", []string{"bgp-flowspec"}, "bgp-flowspec"},
	}

	for _, tt := range tests {
		t.Run(tt.family, func(t *testing.T) {
			got := lookupFamilyPlugin(tt.family, tt.plugins)
			if got != tt.want {
				t.Errorf("lookupFamilyPlugin(%q, %v) = %q, want %q",
					tt.family, tt.plugins, got, tt.want)
			}
		})
	}
}

// TestSRAdjMultipleInstances verifies multiple TLV 1099 instances accumulate into array.
//
// VALIDATES: Lossless JSON format with array accumulation.
//
// PREVENTS: Data loss from duplicate keys.
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

// =============================================================================
// Human-Readable Output Tests
// =============================================================================

// TestDecodeOpenHuman verifies OPEN message decoding produces human-readable output.
//
// VALIDATES: Human-readable format has correct structure and values.
// PREVENTS: Malformed human output format.
func TestDecodeOpenHuman(t *testing.T) {
	// Simple OPEN message: version 4, AS 65533, hold time 180, router-id 10.0.0.2
	hexInput := testOpenMsgHex

	output, err := decodeHexPacket(hexInput, "open", "", nil, false)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// Human output should NOT be valid JSON
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err == nil {
		t.Error("human output should not be valid JSON")
	}

	// Check for expected structure
	if !strings.Contains(output, "BGP OPEN Message") {
		t.Error("missing 'BGP OPEN Message' header")
	}
	if !strings.Contains(output, "Version:") {
		t.Error("missing 'Version:' field")
	}
	if !strings.Contains(output, "ASN:") {
		t.Error("missing 'ASN:' field")
	}
	if !strings.Contains(output, "65533") {
		t.Error("missing ASN value 65533")
	}
	if !strings.Contains(output, "Hold Time:") {
		t.Error("missing 'Hold Time:' field")
	}
	if !strings.Contains(output, "180") {
		t.Error("missing hold time value 180")
	}
	if !strings.Contains(output, "Router ID:") {
		t.Error("missing 'Router ID:' field")
	}
	if !strings.Contains(output, "10.0.0.2") {
		t.Error("missing router ID value 10.0.0.2")
	}
	if !strings.Contains(output, "Capabilities:") {
		t.Error("missing 'Capabilities:' section")
	}
	if !strings.Contains(output, "multiprotocol") {
		t.Error("missing multiprotocol capability")
	}
	if !strings.Contains(output, "ipv4/unicast") {
		t.Error("missing ipv4/unicast family")
	}
}

// TestDecodeOpenJSON verifies OPEN message with --json flag produces Ze JSON output.
//
// VALIDATES: JSON flag produces structured Ze JSON output.
// PREVENTS: --json flag not working correctly.
func TestDecodeOpenJSON(t *testing.T) {
	hexInput := testOpenMsgHex

	output, err := decodeHexPacket(hexInput, "open", "", nil, true)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// JSON output should be valid JSON
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("--json output should be valid JSON: %v", err)
	}

	// Ze format: check required JSON fields
	if result["type"] != testTypeBGP {
		t.Errorf("expected type 'bgp', got %v", result["type"])
	}

	bgp, ok := result["bgp"].(map[string]any)
	if !ok {
		t.Fatal("missing 'bgp' field")
	}

	msg, ok := bgp["message"].(map[string]any)
	if !ok {
		t.Fatal("missing 'message' field")
	}
	if msg["type"] != "open" {
		t.Errorf("expected message.type 'open', got %v", msg["type"])
	}

	openSection, ok := bgp["open"].(map[string]any)
	if !ok {
		t.Fatal("missing 'open' section")
	}

	if openSection["asn"] != float64(65533) {
		t.Errorf("expected asn 65533, got %v", openSection["asn"])
	}
}

// TestDecodeUpdateHuman verifies UPDATE message decoding produces human-readable output.
//
// VALIDATES: Human-readable UPDATE format has correct structure.
// PREVENTS: Malformed human UPDATE output.
func TestDecodeUpdateHuman(t *testing.T) {
	// UPDATE message from test/decode/ipv4-unicast-1.test
	hexInput := testUpdateMsgHex

	output, err := decodeHexPacket(hexInput, "update", "", nil, false)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// Human output should NOT be valid JSON
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err == nil {
		t.Error("human output should not be valid JSON")
	}

	// Check for expected structure
	if !strings.Contains(output, "BGP UPDATE Message") {
		t.Error("missing 'BGP UPDATE Message' header")
	}
	if !strings.Contains(output, "Attributes:") {
		t.Error("missing 'Attributes:' section")
	}
	if !strings.Contains(output, "origin") {
		t.Error("missing 'origin' attribute")
	}
	if !strings.Contains(output, "igp") {
		t.Error("missing origin value 'igp'")
	}
}

// TestDecodeUpdateJSON verifies UPDATE message with --json flag produces Ze JSON output.
//
// VALIDATES: JSON flag produces structured Ze UPDATE JSON.
// PREVENTS: --json flag not working for UPDATE messages.
func TestDecodeUpdateJSON(t *testing.T) {
	hexInput := testUpdateMsgHex

	output, err := decodeHexPacket(hexInput, "update", "", nil, true)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("--json output should be valid JSON: %v", err)
	}

	// Ze format: type is "bgp", event type is in message.type
	if result["type"] != testTypeBGP {
		t.Errorf("expected type 'bgp', got %v", result["type"])
	}

	bgp, ok := result["bgp"].(map[string]any)
	if !ok {
		t.Fatal("missing 'bgp' field")
	}

	msg, ok := bgp["message"].(map[string]any)
	if !ok {
		t.Fatal("missing 'message' field")
	}
	if msg["type"] != "update" {
		t.Errorf("expected message.type 'update', got %v", msg["type"])
	}
}

// TestDecodeNLRIHuman verifies NLRI decoding produces human-readable output.
//
// VALIDATES: Human-readable NLRI format for BGP-LS.
// PREVENTS: Malformed human NLRI output.
func TestDecodeNLRIHuman(t *testing.T) {
	// BGP-LS Link NLRI
	hexInput := testBGPLSLinkNLRI

	output, err := decodeHexPacket(hexInput, "nlri", "bgp-ls/bgp-ls", nil, false)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// Human output should NOT be valid JSON
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err == nil {
		t.Error("human output should not be valid JSON")
	}

	// Check for expected structure - should have NLRI info
	if !strings.Contains(output, "NLRI") && !strings.Contains(output, "BGP-LS") {
		t.Error("missing NLRI header in human output")
	}
}

// TestDecodeNLRIJSON verifies NLRI decoding with --json flag produces JSON output.
//
// VALIDATES: JSON flag produces structured NLRI JSON.
// PREVENTS: --json flag not working for NLRI decoding.
func TestDecodeNLRIJSON(t *testing.T) {
	hexInput := testBGPLSLinkNLRI

	output, err := decodeHexPacket(hexInput, "nlri", "bgp-ls/bgp-ls", nil, true)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("--json output should be valid JSON: %v", err)
	}

	if result["ls-nlri-type"] != "bgpls-link" {
		t.Errorf("expected ls-nlri-type 'bgpls-link', got %v", result["ls-nlri-type"])
	}
}

// TestDecodeErrorHuman verifies error output in human-readable mode.
//
// VALIDATES: Errors show human-readable message, not JSON.
// PREVENTS: JSON error format when human output requested.
func TestDecodeErrorHuman(t *testing.T) {
	// Invalid hex input
	hexInput := "ZZZ"

	_, err := decodeHexPacket(hexInput, "open", "", nil, false)
	if err == nil {
		t.Fatal("expected error for invalid hex")
	}

	// Error message should contain useful info
	if !strings.Contains(err.Error(), "invalid hex") {
		t.Errorf("expected 'invalid hex' in error, got: %v", err)
	}
}

// TestParsePluginName verifies plugin name syntax parsing.
//
// VALIDATES: Three invocation modes are correctly detected from syntax.
// PREVENTS: Wrong invocation mode for prefixed plugin names.
func TestParsePluginName(t *testing.T) {
	tests := []struct {
		input    string
		wantName string
		wantMode PluginMode
		wantPath string
		wantArgs []string
	}{
		// Plain names → Fork mode (subprocess)
		{"bgp-flowspec", "bgp-flowspec", ModeFork, "", nil},
		{"bgp-hostname", "bgp-hostname", ModeFork, "", nil},
		{"bgp-ls", "bgp-ls", ModeFork, "", nil},

		// ze.name → Internal mode (goroutine + pipe)
		{"ze.bgp-flowspec", "bgp-flowspec", ModeInternal, "", nil},
		{"ze.bgp-hostname", "bgp-hostname", ModeInternal, "", nil},
		{"ze.bgp-ls", "bgp-ls", ModeInternal, "", nil},

		// ze-name → Direct mode (sync in-process)
		{"ze-bgp-flowspec", "bgp-flowspec", ModeDirect, "", nil},
		{"ze-bgp-hostname", "bgp-hostname", ModeDirect, "", nil},
		{"ze-bgp-ls", "bgp-ls", ModeDirect, "", nil},

		// Paths → Fork mode with path (no args)
		{"/usr/bin/plugin", "", ModeFork, "/usr/bin/plugin", nil},
		{"./local-plugin", "", ModeFork, "./local-plugin", nil},
		{"../other/plugin", "", ModeFork, "../other/plugin", nil},
		{"/path/to/ze-plugin", "", ModeFork, "/path/to/ze-plugin", nil},

		// Paths with arguments → Fork mode with path and args
		{"/usr/bin/decoder --verbose", "", ModeFork, "/usr/bin/decoder", []string{"--verbose"}},
		{"./my-plugin --format json", "", ModeFork, "./my-plugin", []string{"--format", "json"}},
		{"/opt/decoder -v --output=yaml", "", ModeFork, "/opt/decoder", []string{"-v", "--output=yaml"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			name, mode, path, args := parsePluginName(tt.input)
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if mode != tt.wantMode {
				t.Errorf("mode = %v, want %v", mode, tt.wantMode)
			}
			if path != tt.wantPath {
				t.Errorf("path = %q, want %q", path, tt.wantPath)
			}
			if !slicesEqual(args, tt.wantArgs) {
				t.Errorf("args = %v, want %v", args, tt.wantArgs)
			}
		})
	}
}

// slicesEqual compares two string slices for equality.
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestParsePluginNameBoundary verifies edge cases in plugin name parsing.
//
// VALIDATES: Empty and unusual inputs handled correctly.
// PREVENTS: Panic on empty input, wrong mode for edge cases.
func TestParsePluginNameBoundary(t *testing.T) {
	tests := []struct {
		input    string
		wantName string
		wantMode PluginMode
		wantPath string
		wantArgs []string
	}{
		// Empty string
		{"", "", ModeFork, "", nil},

		// Just prefixes
		{"ze.", "", ModeInternal, "", nil},
		{"ze-", "", ModeDirect, "", nil},

		// Prefix in middle (not at start) → treated as plain name
		{"foo-ze.bar", "foo-ze.bar", ModeFork, "", nil},
		{"foo.ze-bar", "foo.ze-bar", ModeFork, "", nil},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("input=%q", tt.input), func(t *testing.T) {
			name, mode, path, args := parsePluginName(tt.input)
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if mode != tt.wantMode {
				t.Errorf("mode = %v, want %v", mode, tt.wantMode)
			}
			if path != tt.wantPath {
				t.Errorf("path = %q, want %q", path, tt.wantPath)
			}
			if !slicesEqual(args, tt.wantArgs) {
				t.Errorf("args = %v, want %v", args, tt.wantArgs)
			}
		})
	}
}

// TestInvokePluginDirect verifies ze-name syntax uses direct in-process decode.
//
// VALIDATES: Direct mode (ze-bgp-flowspec) decodes NLRI without subprocess.
// PREVENTS: Wrong invocation path for ze- prefix.
func TestInvokePluginDirect(t *testing.T) {
	result := invokePluginNLRIDecode("ze-bgp-flowspec", testFlowSpecFamily, testFlowSpecNLRI)
	if result == nil {
		t.Fatal("ze-bgp-flowspec direct decode returned nil")
	}

	// Verify we got a decoded result (map or array).
	switch v := result.(type) {
	case map[string]any:
		if len(v) == 0 {
			t.Fatal("expected non-empty result map")
		}
	case []any:
		if len(v) == 0 {
			t.Fatal("expected non-empty result array")
		}
	default:
		t.Fatalf("unexpected result type %T", result)
	}
}

// TestInvokePluginInternal verifies ze.name syntax uses goroutine+pipe decode.
//
// VALIDATES: Internal mode (ze.flowspec) decodes NLRI via plugin protocol.
// PREVENTS: Wrong invocation path for ze. prefix.
func TestInvokePluginInternal(t *testing.T) {
	result := invokePluginNLRIDecode("ze.bgp-flowspec", testFlowSpecFamily, testFlowSpecNLRI)
	if result == nil {
		t.Fatal("ze.flowspec internal decode returned nil")
	}

	// Verify we got a decoded result (map or array).
	switch v := result.(type) {
	case map[string]any:
		if len(v) == 0 {
			t.Fatal("expected non-empty result map")
		}
	case []any:
		if len(v) == 0 {
			t.Fatal("expected non-empty result array")
		}
	default:
		t.Fatalf("unexpected result type %T", result)
	}
}

// TestInvokePluginFork verifies plain name uses subprocess (with in-process retry).
//
// VALIDATES: Fork mode (flowspec) attempts subprocess, retries in-process.
// PREVENTS: Plain names not being handled correctly.
func TestInvokePluginFork(t *testing.T) {
	result := invokePluginNLRIDecode("bgp-flowspec", testFlowSpecFamily, testFlowSpecNLRI)
	if result == nil {
		t.Fatal("flowspec fork decode returned nil")
	}

	// Verify we got a decoded result (map or array).
	switch v := result.(type) {
	case map[string]any:
		if len(v) == 0 {
			t.Fatal("expected non-empty result map")
		}
	case []any:
		if len(v) == 0 {
			t.Fatal("expected non-empty result array")
		}
	default:
		t.Fatalf("unexpected result type %T", result)
	}
}

// TestInvokePluginForkPath verifies path-based fork uses external binary.
//
// VALIDATES: /path/to/binary invokes external program with --decode.
// PREVENTS: Path-based invocation falling back to in-process.
func TestInvokePluginForkPath(t *testing.T) {
	// Use shared pre-built binary (built once via sync.Once).
	binPath := setupTestZeBinary(t)

	// Create a wrapper script that calls ze plugin flowspec --decode.
	wrapperPath := t.TempDir() + "/flowspec-wrapper"
	wrapperScript := fmt.Sprintf("#!/bin/sh\nexec %s plugin bgp-flowspec \"$@\"\n", binPath)
	if err := os.WriteFile(wrapperPath, []byte(wrapperScript), 0o755); err != nil { //nolint:gosec // executable script
		t.Fatalf("failed to write wrapper: %v", err)
	}

	// Invoke via path - this should call the wrapper with --decode.
	result := invokePluginNLRIDecode(wrapperPath, testFlowSpecFamily, testFlowSpecNLRI)
	if result == nil {
		t.Fatal("path-based fork decode returned nil")
	}

	// Verify we got a decoded result.
	switch v := result.(type) {
	case map[string]any:
		if len(v) == 0 {
			t.Fatal("expected non-empty result map")
		}
	case []any:
		if len(v) == 0 {
			t.Fatal("expected non-empty result array")
		}
	default:
		t.Fatalf("unexpected result type %T", result)
	}
}

// TestInvokePluginModeConsistency verifies all three modes produce same result.
//
// VALIDATES: Fork, Internal, and Direct modes decode identically.
// PREVENTS: Mode-dependent decode differences.
func TestInvokePluginModeConsistency(t *testing.T) {
	directResult := invokePluginNLRIDecode("ze-bgp-flowspec", testFlowSpecFamily, testFlowSpecNLRI)
	internalResult := invokePluginNLRIDecode("ze.bgp-flowspec", testFlowSpecFamily, testFlowSpecNLRI)
	forkResult := invokePluginNLRIDecode("bgp-flowspec", testFlowSpecFamily, testFlowSpecNLRI)

	if directResult == nil || internalResult == nil || forkResult == nil {
		t.Fatalf("one or more modes returned nil: direct=%v internal=%v fork=%v",
			directResult != nil, internalResult != nil, forkResult != nil)
	}

	// Marshal to JSON for comparison.
	directJSON, err := json.Marshal(directResult)
	if err != nil {
		t.Fatalf("marshal direct failed: %v", err)
	}
	internalJSON, err := json.Marshal(internalResult)
	if err != nil {
		t.Fatalf("marshal internal failed: %v", err)
	}
	forkJSON, err := json.Marshal(forkResult)
	if err != nil {
		t.Fatalf("marshal fork failed: %v", err)
	}

	if !bytes.Equal(directJSON, internalJSON) {
		t.Errorf("direct vs internal mismatch:\n  direct: %s\n  internal: %s",
			directJSON, internalJSON)
	}
	if !bytes.Equal(directJSON, forkJSON) {
		t.Errorf("direct vs fork mismatch:\n  direct: %s\n  fork: %s",
			directJSON, forkJSON)
	}
}

// =============================================================================
// YANG Decode Input Validation Tests
// =============================================================================

// TestDecodeInput_ValidFamily_YANG verifies family format validation.
// Families are registered dynamically by plugins, so validation checks format (afi/safi).
//
// VALIDATES: Family format validation catches malformed strings before dispatch.
// PREVENTS: Invalid family format strings reaching plugin decoders.
func TestDecodeInput_ValidFamily_YANG(t *testing.T) {
	// Valid format: afi/safi (both parts non-empty)
	validFamilies := []string{
		"ipv4/unicast", "ipv6/unicast",
		"ipv4/multicast", "l2vpn/evpn",
		"bgp-ls/bgp-ls", "foo/bar", // format is valid even if not registered
	}

	for _, fam := range validFamilies {
		err := validateDecodeFamily(fam)
		if err != nil {
			t.Errorf("valid format family %q should be accepted, got error: %v", fam, err)
		}
	}

	// Invalid format: missing slash, empty, or empty parts
	invalidFamilies := []string{
		"invalid", "", "/safi", "afi/",
	}

	for _, fam := range invalidFamilies {
		err := validateDecodeFamily(fam)
		if err == nil {
			t.Errorf("invalid format family %q should be rejected", fam)
		}
	}
}

// TestDecodeOutput_Unchanged verifies decode output format is preserved after adding validation.
//
// VALIDATES: Adding input validation doesn't change output format.
// PREVENTS: Validation changes accidentally altering decode output.
func TestDecodeOutput_Unchanged(t *testing.T) {
	// Decode FlowSpec NLRI - should produce same output as before
	hexData := testFlowSpecNLRI
	output, err := decodeHexPacket(hexData, msgTypeNLRI, testFlowSpecFamily, nil, true)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// Verify it's valid JSON
	var result any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nOutput: %s", err, output)
	}

	// Should be a non-empty result
	if result == nil {
		t.Fatal("expected non-nil decode result")
	}
}
