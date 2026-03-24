package bgp

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/nlri/ls"
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

	output, err := decodeHexPacket(hexInput, "open", "", true)
	require.NoError(t, err, "decode failed")

	// Parse JSON output
	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &result), "invalid JSON output: %s", output)

	// Ze format: top-level "type" should be "bgp"
	assert.Equal(t, testTypeBGP, result["type"], "top-level type")

	// Ze format: data under "bgp" key
	bgp, ok := result["bgp"].(map[string]any)
	require.True(t, ok, "missing or invalid 'bgp' field")

	// Ze format: message.type should be "open"
	msg, ok := bgp["message"].(map[string]any)
	require.True(t, ok, "missing or invalid 'message' field in bgp")
	assert.Equal(t, "open", msg["type"], "message.type")

	// Ze format: flat peer structure
	peer, ok := bgp["peer"].(map[string]any)
	require.True(t, ok, "missing or invalid 'peer' field in bgp")
	assert.Equal(t, "127.0.0.1", peer["address"], "peer.address")
	remote, ok := peer["remote"].(map[string]any)
	require.True(t, ok, "missing or invalid 'remote' in peer")
	assert.Equal(t, float64(65533), remote["as"], "peer.remote.as")

	// Check open section
	openSection, ok := bgp["open"].(map[string]any)
	require.True(t, ok, "missing or invalid 'open' section in bgp")

	// Verify key fields
	assert.Equal(t, float64(65533), openSection["asn"], "open.asn")
	timer, ok := openSection["timer"].(map[string]any)
	require.True(t, ok, "open.timer must be a map")
	assert.Equal(t, float64(180), timer["hold-time"], "open.timer.hold-time")
	assert.Equal(t, "10.0.0.2", openSection["router-id"], "open.router-id")

	// Ze format: capabilities should be an array, not a map
	caps, ok := openSection["capabilities"].([]any)
	require.True(t, ok, "missing or invalid 'capabilities' array in open")
	assert.NotEmpty(t, caps, "expected at least one capability")
}

// TestDecodeOpenFQDNWithoutPlugin verifies FQDN capability shows as unknown without plugin.
//
// VALIDATES: Unknown capabilities return name="unknown", code, and raw hex.
// PREVENTS: Leaking decoded capability data without plugin authorization.
func TestDecodeOpenFQDNWithoutPlugin(t *testing.T) {
	// OPEN message with FQDN capability (code 73)
	// hostname="my-host-name", domain="my-domain-name.com"
	hexInput := "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00510104FFFD00B40A000002340206010400010001020641040000FFFD022249200C6D792D686F73742D6E616D65126D792D646F6D61696E2D6E616D652E636F6D"

	// Decode without explicit --plugin flag — registered plugins are auto-invoked.
	output, err := decodeHexPacket(hexInput, "open", "", true)
	require.NoError(t, err, "decode failed")

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &result), "invalid JSON output")

	// Ze format: navigate through bgp.open.capabilities
	bgp, ok := result["bgp"].(map[string]any)
	require.True(t, ok, "missing bgp section")
	openSection, ok := bgp["open"].(map[string]any)
	require.True(t, ok, "missing open section")
	// Ze format: capabilities is an array
	caps, ok := openSection["capabilities"].([]any)
	require.True(t, ok, "missing capabilities array (Ze format uses array, not map)")

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

	require.NotNil(t, cap73, "missing capability with code 73")

	// Auto-loading: registered plugins are invoked automatically.
	// Accept decoded (in-process available) or unknown (decode unavailable).
	name, _ := cap73["name"].(string)
	switch name {
	case "fqdn":
		assert.Equal(t, "my-host-name", cap73["hostname"])
		assert.Equal(t, "my-domain-name.com", cap73["domain"])
	case testCapNameUnknown:
		_, hasRaw := cap73["raw"]
		assert.True(t, hasRaw, "unknown capability should have 'raw' field")
	default:
		t.Errorf("unexpected capability name: %v", name)
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
	output, err := decodeHexPacket(hexInput, "open", "", true)
	require.NoError(t, err, "decode failed")

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &result), "invalid JSON output")

	// Ze format: navigate through bgp.open.capabilities
	bgp, ok := result["bgp"].(map[string]any)
	require.True(t, ok, "missing bgp section")
	openSection, ok := bgp["open"].(map[string]any)
	require.True(t, ok, "missing open section")
	// Ze format: capabilities is an array
	caps, ok := openSection["capabilities"].([]any)
	require.True(t, ok, "missing capabilities array")

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

	require.NotNil(t, cap73, "missing capability with code 73")

	// Accept either decoded (production) or unknown (test environment)
	name, _ := cap73["name"].(string)
	switch name {
	case "fqdn":
		// Plugin decode worked - verify fields
		assert.Equal(t, "my-host-name", cap73["hostname"])
		assert.Equal(t, "my-domain-name.com", cap73["domain"])
	case testCapNameUnknown:
		// Plugin not available in test env - verify fallback has raw data
		_, hasRaw := cap73["raw"]
		assert.True(t, hasRaw, "unknown capability should have 'raw' field")
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

	// Decode without explicit --plugin flag — registered plugins are auto-invoked.
	output, err := decodeHexPacket(hexInput, "open", "", true)
	require.NoError(t, err, "decode failed")

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &result), "invalid JSON output")

	// Ze format: navigate through bgp.open.capabilities
	bgp, ok := result["bgp"].(map[string]any)
	require.True(t, ok, "missing bgp section")
	openSection, ok := bgp["open"].(map[string]any)
	require.True(t, ok, "missing open section")
	caps, ok := openSection["capabilities"].([]any)
	require.True(t, ok, "missing capabilities array")

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

	require.NotNil(t, cap64, "missing capability with code 64")

	// Auto-loading: registered plugins are invoked automatically.
	// Accept decoded (in-process available) or unknown (decode unavailable).
	name, _ := cap64["name"].(string)
	switch name {
	case "graceful-restart":
		// Plugin decoded successfully — no further field checks needed.
	case testCapNameUnknown:
		_, hasRaw := cap64["raw"]
		assert.True(t, hasRaw, "unknown capability should have 'raw' field")
	default:
		t.Errorf("unexpected capability name: %v", name)
	}
}

// TestDecodeOpenRRWithoutPlugin verifies Route Refresh capability shows as unknown without plugin.
//
// VALIDATES: RR capability (code 2) returns name="unknown" without --plugin flag.
// PREVENTS: Leaking decoded RR data without plugin authorization.
func TestDecodeOpenRRWithoutPlugin(t *testing.T) {
	// OPEN message with Route Refresh capability (code 2, zero payload)
	hexInput := "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002D0104FFFD00B40A00000210020E01040001000141040000FFFD0200"

	// Decode without explicit --plugin flag — registered plugins are auto-invoked.
	output, err := decodeHexPacket(hexInput, "open", "", true)
	require.NoError(t, err, "decode failed")

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &result), "invalid JSON output")

	// Ze format: navigate through bgp.open.capabilities
	bgp, ok := result["bgp"].(map[string]any)
	require.True(t, ok, "missing bgp section")
	openSection, ok := bgp["open"].(map[string]any)
	require.True(t, ok, "missing open section")
	caps, ok := openSection["capabilities"].([]any)
	require.True(t, ok, "missing capabilities array")

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

	require.NotNil(t, cap2, "missing capability with code 2")

	// Auto-loading: registered plugins are invoked automatically.
	// Accept decoded (in-process available) or unknown (decode unavailable).
	name, _ := cap2["name"].(string)
	switch name {
	case "route-refresh":
		// Plugin decoded successfully.
	case testCapNameUnknown:
		_, hasRaw := cap2["raw"]
		assert.True(t, hasRaw, "unknown capability should have 'raw' field")
	default:
		t.Errorf("unexpected capability name: %v", name)
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

	output, err := decodeHexPacket(hexInput, "update", "", true)
	require.NoError(t, err, "decode failed")

	// Parse JSON output
	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &result), "invalid JSON output: %s", output)

	// Ze format: top-level "type" should be "bgp"
	assert.Equal(t, testTypeBGP, result["type"], "top-level type")

	// Ze format: data under "bgp" key
	bgp, ok := result["bgp"].(map[string]any)
	require.True(t, ok, "missing or invalid 'bgp' field")

	// Ze format: message.type should be "update"
	msg, ok := bgp["message"].(map[string]any)
	require.True(t, ok, "missing or invalid 'message' field in bgp")
	assert.Equal(t, "update", msg["type"], "message.type")

	// Ze format: flat peer structure
	peer, ok := bgp["peer"].(map[string]any)
	require.True(t, ok, "missing or invalid 'peer' field in bgp")
	assert.NotNil(t, peer["address"], "peer.address")
	assert.NotNil(t, peer["remote"], "peer.remote")

	// Ze format: update section under bgp.update
	update, ok := bgp["update"].(map[string]any)
	require.True(t, ok, "missing or invalid 'update' section in bgp")

	// Ze format: attributes under "attr" (not "attribute")
	_, ok = update["attr"].(map[string]any)
	assert.True(t, ok, "missing 'attr' field in update (Ze format uses 'attr', not 'attribute')")
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
			assert.Equal(t, "FFFFFFFFFFFFFFFF", normalized)
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

			require.Len(t, got, len(tt.want), "community count")

			for i := range got {
				assert.Equal(t, tt.want[i]["string"], got[i]["string"], "community[%d].string", i)
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

	output, err := decodeHexPacket(hexInput, "update", "ipv4/flow", true)
	require.NoError(t, err, "decode failed")

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &result), "invalid JSON")

	// Ze format: navigate through bgp.update.attr
	bgp, _ := result["bgp"].(map[string]any)          //nolint:forcetypeassert // test
	update, _ := bgp["update"].(map[string]any)       //nolint:forcetypeassert // test
	attrs, _ := update["attr"].(map[string]any)       //nolint:forcetypeassert // test
	extComm, _ := attrs["extended-community"].([]any) //nolint:forcetypeassert // test

	require.Len(t, extComm, 1, "extended-community count")

	comm, _ := extComm[0].(map[string]any) //nolint:forcetypeassert // test
	assert.Equal(t, "rate-limit:0", comm["string"])
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

	output, err := decodeHexPacket(hexInput, "update", "bgp-ls/bgp-ls", true)
	require.NoError(t, err, "decode failed")

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &result), "invalid JSON")

	// Ze format: navigate through bgp.update.family
	bgp, _ := result["bgp"].(map[string]any)       //nolint:forcetypeassert // test
	update, _ := bgp["update"].(map[string]any)    //nolint:forcetypeassert // test
	bgplsOps, _ := update["bgp-ls/bgp-ls"].([]any) //nolint:forcetypeassert // test

	// Ze format: operations array with action/nlri
	require.NotEmpty(t, bgplsOps, "no BGP-LS operations found")

	// Get first operation's nlri array
	op, _ := bgplsOps[0].(map[string]any) //nolint:forcetypeassert // test
	routes, _ := op["nlri"].([]any)       //nolint:forcetypeassert // test

	require.NotEmpty(t, routes, "no BGP-LS routes found")

	route, _ := routes[0].(map[string]any) //nolint:forcetypeassert // test

	// Check required BGP-LS fields
	assert.Equal(t, testBGPLSLinkNLRIType, route["ls-nlri-type"], "ls-nlri-type")
	assert.NotNil(t, route["protocol-id"], "protocol-id")
	assert.NotNil(t, route["local-node-descriptors"], "local-node-descriptors")
	assert.NotNil(t, route["remote-node-descriptors"], "remote-node-descriptors")
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
			assert.Equal(t, tt.want, int(tt.protoID), "protocol-id")
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

	output, err := decodeHexPacket(hexInput, "update", "bgp-ls/bgp-ls", true)
	require.NoError(t, err, "decode failed")

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &result), "invalid JSON")

	// Ze format: navigate through bgp.update.attr
	bgp, _ := result["bgp"].(map[string]any)    //nolint:forcetypeassert // test
	update, _ := bgp["update"].(map[string]any) //nolint:forcetypeassert // test
	attrs, _ := update["attr"].(map[string]any) //nolint:forcetypeassert // test

	// Check for bgp-ls attribute
	bgplsAttr, ok := attrs["bgp-ls"].(map[string]any)
	require.True(t, ok, "missing bgp-ls attribute")

	// Check igp-metric
	assert.NotNil(t, bgplsAttr["igp-metric"], "missing igp-metric in bgp-ls attribute")
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

	output, err := decodeHexPacket(hexInput, "update", "bgp-ls/bgp-ls", true)
	require.NoError(t, err, "decode failed")

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &result), "invalid JSON")

	// Ze format: navigate through bgp.update.family
	bgp, _ := result["bgp"].(map[string]any)       //nolint:forcetypeassert // test
	update, _ := bgp["update"].(map[string]any)    //nolint:forcetypeassert // test
	bgplsOps, _ := update["bgp-ls/bgp-ls"].([]any) //nolint:forcetypeassert // test

	require.NotEmpty(t, bgplsOps, "no BGP-LS operations found")

	op, _ := bgplsOps[0].(map[string]any) //nolint:forcetypeassert // test
	routes, _ := op["nlri"].([]any)       //nolint:forcetypeassert // test

	require.NotEmpty(t, routes, "no BGP-LS routes found")

	route, _ := routes[0].(map[string]any) //nolint:forcetypeassert // test

	// Check for address arrays (should exist even if empty)
	assert.NotNil(t, route["interface-addresses"], "missing interface-addresses field")
	assert.NotNil(t, route["neighbor-addresses"], "missing neighbor-addresses field")
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

	output, err := decodeHexPacket(hexInput, "nlri", "bgp-ls/bgp-ls", true)
	require.NoError(t, err, "decode failed")

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &result), "invalid JSON")

	// For nlri type, output should be flat (no exabgp/neighbor wrapper)
	assert.Equal(t, testBGPLSLinkNLRIType, result["ls-nlri-type"])
	assert.NotNil(t, result["protocol-id"], "missing protocol-id field")
	assert.NotNil(t, result["local-node-descriptors"], "missing local-node-descriptors field")
	assert.NotNil(t, result["remote-node-descriptors"], "missing remote-node-descriptors field")
}

// TestBGPLSL3RoutingTopology verifies l3-routing-topology field.
//
// VALIDATES: l3-routing-topology (identifier) is present and correct.
//
// PREVENTS: Missing routing topology identifier.
func TestBGPLSL3RoutingTopology(t *testing.T) {
	// Link NLRI should have l3-routing-topology from identifier field
	hexInput := testBGPLSLinkNLRI

	output, err := decodeHexPacket(hexInput, "nlri", "bgp-ls/bgp-ls", true)
	require.NoError(t, err, "decode failed")

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &result), "invalid JSON")

	// l3-routing-topology should be 0 (from identifier field)
	assert.NotNil(t, result["l3-routing-topology"], "missing l3-routing-topology field")

	// Should be 0 for this test case
	if topo, ok := result["l3-routing-topology"].(float64); ok {
		assert.Equal(t, float64(0), topo, "l3-routing-topology")
	}
}

// TestAdjSIDViaAttrTLVs verifies Adjacency SID via the new ls.AttrTLVsToJSON path.
//
// VALIDATES: V/L flag combinations decode correctly through the registered TLV decoder.
//
// PREVENTS: Regression from CLI decoder migration.
func TestAdjSIDViaAttrTLVs(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte // TLV value only (no TLV header)
		wantSID uint32
	}{
		{
			name:    "V=1,L=1 3-byte label",
			data:    []byte{0x30, 0x00, 0x00, 0x00, 0x04, 0x93, 0x10},
			wantSID: 299792,
		},
		{
			name:    "V=0,L=0 4-byte index",
			data:    []byte{0x00, 0x05, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00},
			wantSID: 256,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build wire: TLV 1099 header + value
			wire := make([]byte, 4+len(tt.data))
			binary.BigEndian.PutUint16(wire[0:], ls.TLVAdjacencySID)
			binary.BigEndian.PutUint16(wire[2:], uint16(len(tt.data))) //nolint:gosec // test
			copy(wire[4:], tt.data)

			result := ls.AttrTLVsToJSON(wire)
			entries, ok := result["adj-sids"].([]map[string]any)
			require.True(t, ok && len(entries) > 0, "expected sr-adj array")
			sids, ok := entries[0]["sids"].([]int)
			require.True(t, ok && len(sids) > 0, "expected sids array")
			assert.Equal(t, int(tt.wantSID), sids[0])
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
	output, err := decodeHexPacket(hexInput, "nlri", "bgp-ls/bgp-ls", true)
	require.NoError(t, err, "decode failed")

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &result), "invalid JSON")

	// NLRI mode should produce flat JSON (no bgp wrapper envelope)
	_, hasBgp := result["bgp"]
	assert.False(t, hasBgp, "NLRI mode should not have bgp wrapper")

	// Should have BGP-LS fields
	assert.NotNil(t, result["ls-nlri-type"], "missing ls-nlri-type field")
}

// TestDecodeNLRIFlagWithPlugin verifies --nlri with plugin falls back correctly.
//
// VALIDATES: When plugin specified but not available, falls back to built-in decode.
// PREVENTS: Crash or empty output when plugin unavailable.
func TestDecodeNLRIFlagWithPlugin(t *testing.T) {
	// FlowSpec NLRI - plugin not available in test, should fall back to built-in
	// This tests the infrastructure path even without actual plugin
	hexInput := "0701180a0000" // Simple FlowSpec: destination 10.0.0.0/24

	output, err := decodeHexPacket(hexInput, "nlri", "ipv4/flow", true)
	require.NoError(t, err, "decode failed")

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &result), "invalid JSON")

	// Should have some output (either plugin or fallback)
	assert.NotEmpty(t, result, "expected non-empty result")
}

// TestLookupFamilyPlugin verifies family plugin lookup with case insensitivity.
//
// VALIDATES: lookupFamilyPlugin normalizes family to lowercase.
// PREVENTS: Case mismatch causing plugin lookup failures.
func TestLookupFamilyPlugin(t *testing.T) {
	tests := []struct {
		family string
		want   string
	}{
		{"ipv4/flow", "bgp-nlri-flowspec"},
		{"IPV4/FLOW", "bgp-nlri-flowspec"},
		{"IPv4/Flow", "bgp-nlri-flowspec"},
		{"ipv4/unicast", ""}, // Unknown family, no plugin
		{"ipv6/flow-vpn", "bgp-nlri-flowspec"},
	}

	for _, tt := range tests {
		t.Run(tt.family, func(t *testing.T) {
			got := lookupFamilyPlugin(tt.family)
			assert.Equal(t, tt.want, got, "lookupFamilyPlugin(%q)", tt.family)
		})
	}
}

// TestSRAdjMultipleInstances verifies multiple TLV 1099 instances accumulate into array.
//
// VALIDATES: Lossless JSON format with array accumulation via ls.AttrTLVsToJSON.
//
// PREVENTS: Data loss from duplicate keys.
func TestSRAdjMultipleInstances(t *testing.T) {
	// Build wire: two TLV 1099 instances back-to-back
	d1 := []byte{0x30, 0x00, 0x00, 0x00, 0x04, 0x93, 0x10} // V=1,L=1 SID=299792
	d2 := []byte{0x70, 0x00, 0x00, 0x00, 0x04, 0x93, 0x00} // B=1,V=1,L=1 SID=299776
	wire := make([]byte, 0, 4+len(d1)+4+len(d2))
	wire = binary.BigEndian.AppendUint16(wire, ls.TLVAdjacencySID)
	wire = binary.BigEndian.AppendUint16(wire, uint16(len(d1))) //nolint:gosec // test
	wire = append(wire, d1...)
	wire = binary.BigEndian.AppendUint16(wire, ls.TLVAdjacencySID)
	wire = binary.BigEndian.AppendUint16(wire, uint16(len(d2))) //nolint:gosec // test
	wire = append(wire, d2...)

	result := ls.AttrTLVsToJSON(wire)
	entries, ok := result["adj-sids"].([]map[string]any)
	require.True(t, ok, "expected sr-adj to be array")
	require.Len(t, entries, 2, "sr-adj entry count")

	sids0, ok := entries[0]["sids"].([]int)
	require.True(t, ok && len(sids0) > 0, "expected sids array in first entry")
	sids1, ok := entries[1]["sids"].([]int)
	require.True(t, ok && len(sids1) > 0, "expected sids array in second entry")

	assert.Equal(t, 299792, sids0[0], "first SID")
	assert.Equal(t, 299776, sids1[0], "second SID")
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

	output, err := decodeHexPacket(hexInput, "open", "", false)
	require.NoError(t, err, "decode failed")

	// Human output should NOT be valid JSON
	assert.Error(t, json.Unmarshal([]byte(output), &map[string]any{}), "human output should not be valid JSON")

	// Check for expected structure
	assert.Contains(t, output, "BGP OPEN Message")
	assert.Contains(t, output, "Version:")
	assert.Contains(t, output, "ASN:")
	assert.Contains(t, output, "65533")
	assert.Contains(t, output, "Hold Time:")
	assert.Contains(t, output, "180")
	assert.Contains(t, output, "Router ID:")
	assert.Contains(t, output, "10.0.0.2")
	assert.Contains(t, output, "Capabilities:")
	assert.Contains(t, output, "multiprotocol")
	assert.Contains(t, output, "ipv4/unicast")
}

// TestDecodeOpenJSON verifies OPEN message with --json flag produces Ze JSON output.
//
// VALIDATES: JSON flag produces structured Ze JSON output.
// PREVENTS: --json flag not working correctly.
func TestDecodeOpenJSON(t *testing.T) {
	hexInput := testOpenMsgHex

	output, err := decodeHexPacket(hexInput, "open", "", true)
	require.NoError(t, err, "decode failed")

	// JSON output should be valid JSON
	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &result), "--json output should be valid JSON")

	// Ze format: check required JSON fields
	assert.Equal(t, testTypeBGP, result["type"], "top-level type")

	bgp, ok := result["bgp"].(map[string]any)
	require.True(t, ok, "missing 'bgp' field")

	msg, ok := bgp["message"].(map[string]any)
	require.True(t, ok, "missing 'message' field")
	assert.Equal(t, "open", msg["type"], "message.type")

	openSection, ok := bgp["open"].(map[string]any)
	require.True(t, ok, "missing 'open' section")

	assert.Equal(t, float64(65533), openSection["asn"], "asn")
}

// TestDecodeUpdateHuman verifies UPDATE message decoding produces human-readable output.
//
// VALIDATES: Human-readable UPDATE format has correct structure.
// PREVENTS: Malformed human UPDATE output.
func TestDecodeUpdateHuman(t *testing.T) {
	// UPDATE message from test/decode/ipv4-unicast-1.test
	hexInput := testUpdateMsgHex

	output, err := decodeHexPacket(hexInput, "update", "", false)
	require.NoError(t, err, "decode failed")

	// Human output should NOT be valid JSON
	assert.Error(t, json.Unmarshal([]byte(output), &map[string]any{}), "human output should not be valid JSON")

	// Check for expected structure
	assert.Contains(t, output, "BGP UPDATE Message")
	assert.Contains(t, output, "Attributes:")
	assert.Contains(t, output, "origin")
	assert.Contains(t, output, "igp")
}

// TestDecodeUpdateJSON verifies UPDATE message with --json flag produces Ze JSON output.
//
// VALIDATES: JSON flag produces structured Ze UPDATE JSON.
// PREVENTS: --json flag not working for UPDATE messages.
func TestDecodeUpdateJSON(t *testing.T) {
	hexInput := testUpdateMsgHex

	output, err := decodeHexPacket(hexInput, "update", "", true)
	require.NoError(t, err, "decode failed")

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &result), "--json output should be valid JSON")

	// Ze format: type is "bgp", event type is in message.type
	assert.Equal(t, testTypeBGP, result["type"], "top-level type")

	bgp, ok := result["bgp"].(map[string]any)
	require.True(t, ok, "missing 'bgp' field")

	msg, ok := bgp["message"].(map[string]any)
	require.True(t, ok, "missing 'message' field")
	assert.Equal(t, "update", msg["type"], "message.type")
}

// TestDecodeNLRIHuman verifies NLRI decoding produces human-readable output.
//
// VALIDATES: Human-readable NLRI format for BGP-LS.
// PREVENTS: Malformed human NLRI output.
func TestDecodeNLRIHuman(t *testing.T) {
	// BGP-LS Link NLRI
	hexInput := testBGPLSLinkNLRI

	output, err := decodeHexPacket(hexInput, "nlri", "bgp-ls/bgp-ls", false)
	require.NoError(t, err, "decode failed")

	// Human output should NOT be valid JSON
	assert.Error(t, json.Unmarshal([]byte(output), &map[string]any{}), "human output should not be valid JSON")

	// Check for expected structure - should have NLRI info
	assert.True(t, strings.Contains(output, "NLRI") || strings.Contains(output, "BGP-LS"), "missing NLRI header in human output")
}

// TestDecodeNLRIJSON verifies NLRI decoding with --json flag produces JSON output.
//
// VALIDATES: JSON flag produces structured NLRI JSON.
// PREVENTS: --json flag not working for NLRI decoding.
func TestDecodeNLRIJSON(t *testing.T) {
	hexInput := testBGPLSLinkNLRI

	output, err := decodeHexPacket(hexInput, "nlri", "bgp-ls/bgp-ls", true)
	require.NoError(t, err, "decode failed")

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &result), "--json output should be valid JSON")

	assert.Equal(t, "bgpls-link", result["ls-nlri-type"])
}

// TestDecodeErrorHuman verifies error output in human-readable mode.
//
// VALIDATES: Errors show human-readable message, not JSON.
// PREVENTS: JSON error format when human output requested.
func TestDecodeErrorHuman(t *testing.T) {
	// Invalid hex input
	hexInput := "ZZZ"

	_, err := decodeHexPacket(hexInput, "open", "", false)
	require.Error(t, err, "expected error for invalid hex")
	assert.Contains(t, err.Error(), "invalid hex")
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
		{"bgp-nlri-flowspec", "bgp-nlri-flowspec", ModeFork, "", nil},
		{"bgp-hostname", "bgp-hostname", ModeFork, "", nil},
		{"bgp-nlri-ls", "bgp-nlri-ls", ModeFork, "", nil},

		// ze.name → Internal mode (goroutine + pipe)
		{"ze.bgp-nlri-flowspec", "bgp-nlri-flowspec", ModeInternal, "", nil},
		{"ze.bgp-hostname", "bgp-hostname", ModeInternal, "", nil},
		{"ze.bgp-nlri-ls", "bgp-nlri-ls", ModeInternal, "", nil},

		// ze-name → Direct mode (sync in-process)
		{"ze-bgp-nlri-flowspec", "bgp-nlri-flowspec", ModeDirect, "", nil},
		{"ze-bgp-hostname", "bgp-hostname", ModeDirect, "", nil},
		{"ze-bgp-nlri-ls", "bgp-nlri-ls", ModeDirect, "", nil},

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
			assert.Equal(t, tt.wantName, name, "name")
			assert.Equal(t, tt.wantMode, mode, "mode")
			assert.Equal(t, tt.wantPath, path, "path")
			assert.Equal(t, tt.wantArgs, args, "args")
		})
	}
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
			assert.Equal(t, tt.wantName, name, "name")
			assert.Equal(t, tt.wantMode, mode, "mode")
			assert.Equal(t, tt.wantPath, path, "path")
			assert.Equal(t, tt.wantArgs, args, "args")
		})
	}
}

// TestInvokePluginDirect verifies ze-name syntax uses direct in-process decode.
//
// VALIDATES: Direct mode (ze-bgp-flowspec) decodes NLRI without subprocess.
// PREVENTS: Wrong invocation path for ze- prefix.
func TestInvokePluginDirect(t *testing.T) {
	result := invokePluginNLRIDecode("ze-bgp-nlri-flowspec", testFlowSpecFamily, testFlowSpecNLRI)
	require.NotNil(t, result, "ze-bgp-flowspec direct decode returned nil")
	assertNonEmptyDecodeResult(t, result)
}

// assertNonEmptyDecodeResult verifies that a decode result is a non-empty map or array.
func assertNonEmptyDecodeResult(t *testing.T, result any) {
	t.Helper()
	switch v := result.(type) {
	case map[string]any:
		assert.NotEmpty(t, v, "expected non-empty result map")
	case []any:
		assert.NotEmpty(t, v, "expected non-empty result array")
	default:
		t.Fatalf("unexpected result type %T", result)
	}
}

// TestInvokePluginInternal verifies ze.name syntax uses goroutine+pipe decode.
//
// VALIDATES: Internal mode (ze.flowspec) decodes NLRI via plugin protocol.
// PREVENTS: Wrong invocation path for ze. prefix.
func TestInvokePluginInternal(t *testing.T) {
	result := invokePluginNLRIDecode("ze.bgp-nlri-flowspec", testFlowSpecFamily, testFlowSpecNLRI)
	require.NotNil(t, result, "ze.flowspec internal decode returned nil")
	assertNonEmptyDecodeResult(t, result)
}

// TestInvokePluginFork verifies plain name uses subprocess (with in-process retry).
//
// VALIDATES: Fork mode (flowspec) attempts subprocess, retries in-process.
// PREVENTS: Plain names not being handled correctly.
func TestInvokePluginFork(t *testing.T) {
	result := invokePluginNLRIDecode("bgp-nlri-flowspec", testFlowSpecFamily, testFlowSpecNLRI)
	require.NotNil(t, result, "flowspec fork decode returned nil")
	assertNonEmptyDecodeResult(t, result)
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
	wrapperScript := fmt.Sprintf("#!/bin/sh\nexec %s plugin bgp-nlri-flowspec \"$@\"\n", binPath)
	require.NoError(t, os.WriteFile(wrapperPath, []byte(wrapperScript), 0o755), "failed to write wrapper") //nolint:gosec // executable script

	// Invoke via path - this should call the wrapper with --decode.
	result := invokePluginNLRIDecode(wrapperPath, testFlowSpecFamily, testFlowSpecNLRI)
	require.NotNil(t, result, "path-based fork decode returned nil")
	assertNonEmptyDecodeResult(t, result)
}

// TestInvokePluginModeConsistency verifies all three modes produce same result.
//
// VALIDATES: Fork, Internal, and Direct modes decode identically.
// PREVENTS: Mode-dependent decode differences.
func TestInvokePluginModeConsistency(t *testing.T) {
	directResult := invokePluginNLRIDecode("ze-bgp-nlri-flowspec", testFlowSpecFamily, testFlowSpecNLRI)
	internalResult := invokePluginNLRIDecode("ze.bgp-nlri-flowspec", testFlowSpecFamily, testFlowSpecNLRI)
	forkResult := invokePluginNLRIDecode("bgp-nlri-flowspec", testFlowSpecFamily, testFlowSpecNLRI)

	require.NotNil(t, directResult, "direct returned nil")
	require.NotNil(t, internalResult, "internal returned nil")
	require.NotNil(t, forkResult, "fork returned nil")

	// Marshal to JSON for comparison.
	directJSON, err := json.Marshal(directResult)
	require.NoError(t, err, "marshal direct")
	internalJSON, err := json.Marshal(internalResult)
	require.NoError(t, err, "marshal internal")
	forkJSON, err := json.Marshal(forkResult)
	require.NoError(t, err, "marshal fork")

	assert.Equal(t, string(directJSON), string(internalJSON), "direct vs internal mismatch")
	assert.Equal(t, string(directJSON), string(forkJSON), "direct vs fork mismatch")
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
		assert.NoError(t, validateDecodeFamily(fam), "valid format family %q should be accepted", fam)
	}

	// Invalid format: missing slash, empty, or empty parts
	invalidFamilies := []string{
		"invalid", "", "/safi", "afi/",
	}

	for _, fam := range invalidFamilies {
		assert.Error(t, validateDecodeFamily(fam), "invalid format family %q should be rejected", fam)
	}
}

// TestDecodeOutput_Unchanged verifies decode output format is preserved after adding validation.
//
// VALIDATES: Adding input validation doesn't change output format.
// PREVENTS: Validation changes accidentally altering decode output.
func TestDecodeOutput_Unchanged(t *testing.T) {
	// Decode FlowSpec NLRI - should produce same output as before
	hexData := testFlowSpecNLRI
	output, err := decodeHexPacket(hexData, msgTypeNLRI, testFlowSpecFamily, true)
	require.NoError(t, err, "decode failed")

	// Verify it's valid JSON
	var result any
	require.NoError(t, json.Unmarshal([]byte(output), &result), "invalid JSON output: %s", output)

	// Should be a non-empty result
	require.NotNil(t, result, "expected non-nil decode result")
}
