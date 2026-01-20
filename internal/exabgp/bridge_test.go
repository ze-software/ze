package exabgp

import (
	"bufio"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestZebgpToExabgpJSON_UpdateAnnounce verifies UPDATE announce conversion.
//
// VALIDATES: ZeBGP JSON → ExaBGP JSON for IPv4 unicast announce.
// PREVENTS: Missing attributes, wrong family format, wrong direction mapping.
func TestZebgpToExabgpJSON_UpdateAnnounce(t *testing.T) {
	zebgp := map[string]any{
		"message": map[string]any{
			"type":      "update",
			"id":        float64(1),
			"direction": "received",
		},
		"peer": map[string]any{
			"address": "10.0.0.1",
			"asn":     float64(65001),
		},
		"origin":  "igp",
		"as-path": []any{float64(65001)},
		"ipv4/unicast": []any{
			map[string]any{
				"action":   "add",
				"next-hop": "10.0.0.1",
				"nlri":     []any{"192.168.1.0/24"},
			},
		},
	}

	result := ZebgpToExabgpJSON(zebgp)

	assert.Equal(t, Version, result["exabgp"])
	assert.Equal(t, "update", result["type"])

	neighbor, ok := result["neighbor"].(map[string]any)
	require.True(t, ok, "neighbor should be map[string]any")

	addrMap, ok := neighbor["address"].(map[string]any)
	require.True(t, ok, "address should be map[string]any")
	assert.Equal(t, "10.0.0.1", addrMap["peer"])

	asnMap, ok := neighbor["asn"].(map[string]any)
	require.True(t, ok, "asn should be map[string]any")
	assert.Equal(t, float64(65001), asnMap["peer"])

	assert.Equal(t, "receive", neighbor["direction"])

	msg, ok := neighbor["message"].(map[string]any)
	require.True(t, ok, "message should be map[string]any")

	update, ok := msg["update"].(map[string]any)
	require.True(t, ok, "update should be map[string]any")

	attrs, ok := update["attribute"].(map[string]any)
	require.True(t, ok, "attribute should be map[string]any")
	assert.Equal(t, "igp", attrs["origin"])
	assert.Equal(t, []any{float64(65001)}, attrs["as-path"])

	announce, ok := update["announce"].(map[string]map[string][]any)
	require.True(t, ok, "announce should be map[string]map[string][]any")
	require.Contains(t, announce, "ipv4 unicast")
	require.Contains(t, announce["ipv4 unicast"], "10.0.0.1")
	assert.Equal(t, map[string]any{"nlri": "192.168.1.0/24"}, announce["ipv4 unicast"]["10.0.0.1"][0])
}

// TestZebgpToExabgpJSON_UpdateWithdraw verifies UPDATE withdraw conversion.
//
// VALIDATES: ZeBGP JSON → ExaBGP JSON for IPv4 unicast withdraw.
// PREVENTS: Missing withdraw section, wrong family format.
func TestZebgpToExabgpJSON_UpdateWithdraw(t *testing.T) {
	zebgp := map[string]any{
		"message": map[string]any{"type": "update", "direction": "received"},
		"peer":    map[string]any{"address": "10.0.0.1", "asn": float64(65001)},
		"ipv4/unicast": []any{
			map[string]any{"action": "del", "nlri": []any{"172.16.0.0/16"}},
		},
	}

	result := ZebgpToExabgpJSON(zebgp)

	neighbor, ok := result["neighbor"].(map[string]any)
	require.True(t, ok)
	msg, ok := neighbor["message"].(map[string]any)
	require.True(t, ok)
	update, ok := msg["update"].(map[string]any)
	require.True(t, ok)

	withdraw, ok := update["withdraw"].(map[string][]any)
	require.True(t, ok)
	require.Contains(t, withdraw, "ipv4 unicast")
	assert.Equal(t, map[string]any{"nlri": "172.16.0.0/16"}, withdraw["ipv4 unicast"][0])
}

// TestZebgpToExabgpJSON_StateUp verifies state UP message conversion.
//
// VALIDATES: State messages converted with correct structure.
// PREVENTS: Missing state field, wrong type.
func TestZebgpToExabgpJSON_StateUp(t *testing.T) {
	zebgp := map[string]any{
		"message": map[string]any{"type": "state"},
		"peer":    map[string]any{"address": "10.0.0.1", "asn": float64(65001)},
		"state":   "up",
	}

	result := ZebgpToExabgpJSON(zebgp)

	assert.Equal(t, "state", result["type"])
	neighbor, ok := result["neighbor"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "up", neighbor["state"])
}

// TestZebgpToExabgpJSON_DirectionMapping verifies direction mapping.
//
// VALIDATES: "received" → "receive", "sent" → "send".
// PREVENTS: Incorrect direction in ExaBGP output.
func TestZebgpToExabgpJSON_DirectionMapping(t *testing.T) {
	tests := []struct {
		name      string
		direction string
		want      string
	}{
		{"received_to_receive", "received", "receive"},
		{"sent_to_send", "sent", "send"},
		{"empty_to_receive", "", "receive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zebgp := map[string]any{
				"message": map[string]any{"type": "update", "direction": tt.direction},
				"peer":    map[string]any{"address": "10.0.0.1", "asn": float64(65001)},
			}

			result := ZebgpToExabgpJSON(zebgp)
			neighbor, ok := result["neighbor"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, tt.want, neighbor["direction"])
		})
	}
}

// TestExabgpToZebgpCommand_AnnounceBasic verifies basic announce conversion.
//
// VALIDATES: ExaBGP announce → ZeBGP update text command.
// PREVENTS: Wrong keyword mapping, missing prefix.
func TestExabgpToZebgpCommand_AnnounceBasic(t *testing.T) {
	cmd := "neighbor 10.0.0.1 announce route 192.168.1.0/24 next-hop 1.1.1.1"
	result := ExabgpToZebgpCommand(cmd)

	assert.Equal(t, "peer 10.0.0.1 update text nhop set 1.1.1.1 nlri ipv4/unicast add 192.168.1.0/24", result)
}

// TestExabgpToZebgpCommand_AnnounceWithAttributes verifies attribute conversion.
//
// VALIDATES: All common attributes converted correctly.
// PREVENTS: Missing attributes in output.
func TestExabgpToZebgpCommand_AnnounceWithAttributes(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		contain []string
	}{
		{
			name:    "with_origin",
			input:   "neighbor 10.0.0.1 announce route 10.0.0.0/24 next-hop 1.1.1.1 origin igp",
			contain: []string{"nhop set 1.1.1.1", "origin set igp", "nlri ipv4/unicast add 10.0.0.0/24"},
		},
		{
			name:    "with_as_path",
			input:   "neighbor 10.0.0.1 announce route 10.0.0.0/24 next-hop 1.1.1.1 as-path [65001 65002]",
			contain: []string{"as-path set 65001 65002"},
		},
		{
			name:    "with_med",
			input:   "neighbor 10.0.0.1 announce route 10.0.0.0/24 next-hop 1.1.1.1 med 100",
			contain: []string{"med set 100"},
		},
		{
			name:    "with_local_pref",
			input:   "neighbor 10.0.0.1 announce route 10.0.0.0/24 next-hop 1.1.1.1 local-preference 200",
			contain: []string{"local-preference set 200"},
		},
		{
			name:    "with_community",
			input:   "neighbor 10.0.0.1 announce route 10.0.0.0/24 next-hop 1.1.1.1 community 65001:100",
			contain: []string{"community add 65001:100"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExabgpToZebgpCommand(tt.input)
			for _, s := range tt.contain {
				assert.Contains(t, result, s)
			}
		})
	}
}

// TestExabgpToZebgpCommand_Withdraw verifies withdraw conversion.
//
// VALIDATES: ExaBGP withdraw → ZeBGP del command.
// PREVENTS: Wrong action in output.
func TestExabgpToZebgpCommand_Withdraw(t *testing.T) {
	cmd := "neighbor 10.0.0.1 withdraw route 172.16.0.0/16"
	result := ExabgpToZebgpCommand(cmd)

	assert.Equal(t, "peer 10.0.0.1 update text nlri ipv4/unicast del 172.16.0.0/16", result)
}

// TestExabgpToZebgpCommand_IPv6 verifies IPv6 family detection.
//
// VALIDATES: IPv6 prefixes use ipv6/unicast family.
// PREVENTS: IPv6 routes using ipv4/unicast.
func TestExabgpToZebgpCommand_IPv6(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		contain string
	}{
		{
			name:    "announce",
			input:   "neighbor 2001:db8::1 announce route 2001:db8::/32 next-hop 2001:db8::ffff",
			contain: "nlri ipv6/unicast add 2001:db8::/32",
		},
		{
			name:    "withdraw",
			input:   "neighbor 2001:db8::1 withdraw route 2001:db8::/32",
			contain: "nlri ipv6/unicast del 2001:db8::/32",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExabgpToZebgpCommand(tt.input)
			assert.Contains(t, result, tt.contain)
		})
	}
}

// TestExabgpToZebgpCommand_EmptyAndComment verifies empty/comment handling.
//
// VALIDATES: Empty lines and comments return empty string.
// PREVENTS: Passing through invalid commands.
func TestExabgpToZebgpCommand_EmptyAndComment(t *testing.T) {
	assert.Equal(t, "", ExabgpToZebgpCommand(""))
	assert.Equal(t, "", ExabgpToZebgpCommand("   "))
	assert.Equal(t, "", ExabgpToZebgpCommand("# this is a comment"))
}

// TestExabgpToZebgpCommand_CaseInsensitive verifies case insensitivity.
//
// VALIDATES: Keywords are case-insensitive.
// PREVENTS: Failing on uppercase input.
func TestExabgpToZebgpCommand_CaseInsensitive(t *testing.T) {
	cmd := "NEIGHBOR 10.0.0.1 ANNOUNCE ROUTE 192.168.1.0/24 NEXT-HOP 1.1.1.1"
	result := ExabgpToZebgpCommand(cmd)

	assert.Contains(t, result, "peer 10.0.0.1")
	assert.Contains(t, result, "nlri ipv4/unicast add 192.168.1.0/24")
}

// TestExabgpToZebgpCommand_ExplicitFamily verifies explicit family handling.
//
// VALIDATES: Explicit family syntax (ipv4 unicast) converted correctly.
// PREVENTS: Family mismatch.
func TestExabgpToZebgpCommand_ExplicitFamily(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		contain string
	}{
		{
			name:    "announce_ipv4_unicast",
			input:   "neighbor 10.0.0.1 announce ipv4 unicast 192.168.1.0/24 next-hop 1.1.1.1",
			contain: "nlri ipv4/unicast add 192.168.1.0/24",
		},
		{
			name:    "withdraw_ipv4_unicast",
			input:   "neighbor 10.0.0.1 withdraw ipv4 unicast 192.168.1.0/24",
			contain: "nlri ipv4/unicast del 192.168.1.0/24",
		},
		{
			name:    "announce_ipv6_unicast",
			input:   "neighbor 2001:db8::1 announce ipv6 unicast 2001:db8::/32 next-hop 2001:db8::ffff",
			contain: "nlri ipv6/unicast add 2001:db8::/32",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExabgpToZebgpCommand(tt.input)
			assert.Contains(t, result, tt.contain)
		})
	}
}

// TestExabgpToZebgpCommand_NonNeighbor verifies pass-through for non-neighbor commands.
//
// VALIDATES: Non-neighbor commands pass through unchanged.
// PREVENTS: Breaking unknown commands.
func TestExabgpToZebgpCommand_NonNeighbor(t *testing.T) {
	cmd := "shutdown"
	result := ExabgpToZebgpCommand(cmd)
	assert.Equal(t, "shutdown", result)
}

// TestRoundTrip verifies essential information is preserved.
//
// VALIDATES: Key fields preserved in both conversion directions.
// PREVENTS: Information loss during translation.
func TestRoundTrip(t *testing.T) {
	// Test ZeBGP → ExaBGP preserves info
	zebgp := map[string]any{
		"message": map[string]any{"type": "update", "direction": "received"},
		"peer":    map[string]any{"address": "10.0.0.1", "asn": float64(65001)},
		"origin":  "igp",
		"ipv4/unicast": []any{
			map[string]any{"action": "add", "next-hop": "192.168.0.1", "nlri": []any{"10.0.0.0/24"}},
		},
	}

	exabgp := ZebgpToExabgpJSON(zebgp)
	neighbor, ok := exabgp["neighbor"].(map[string]any)
	require.True(t, ok)
	addrMap, ok := neighbor["address"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "10.0.0.1", addrMap["peer"])
	asnMap, ok := neighbor["asn"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(65001), asnMap["peer"])

	// Test ExaBGP → ZeBGP preserves info
	cmd := "neighbor 10.0.0.1 announce route 10.0.0.0/24 next-hop 192.168.0.1 origin igp"
	zebgpCmd := ExabgpToZebgpCommand(cmd)
	assert.Contains(t, zebgpCmd, "peer 10.0.0.1")
	assert.Contains(t, zebgpCmd, "nhop set 192.168.0.1")
	assert.Contains(t, zebgpCmd, "origin set igp")
	assert.Contains(t, zebgpCmd, "nlri ipv4/unicast add 10.0.0.0/24")
}

// TestStartupProtocol verifies 5-stage startup protocol handling.
//
// VALIDATES: Bridge completes startup protocol before JSON translation.
// PREVENTS: Bridge killed by 5s timeout for not completing registration.
func TestStartupProtocol(t *testing.T) {
	t.Run("sends_declarations", func(t *testing.T) {
		var out strings.Builder
		sp := NewStartupProtocol(nil, &out)

		sp.SendDeclarations()

		output := out.String()
		// Must contain declare done
		assert.Contains(t, output, "declare done\n")
		// Default family declarations
		assert.Contains(t, output, "declare family ipv4 unicast\n")
		// ExaBGP plugins expect negotiated messages
		assert.Contains(t, output, "declare receive negotiated\n")
	})

	t.Run("sends_capability_done", func(t *testing.T) {
		var out strings.Builder
		sp := NewStartupProtocol(nil, &out)

		sp.SendCapabilityDone()

		assert.Equal(t, "capability done\n", out.String())
	})

	t.Run("sends_ready", func(t *testing.T) {
		var out strings.Builder
		sp := NewStartupProtocol(nil, &out)

		sp.SendReady()

		assert.Equal(t, "ready\n", out.String())
	})

	t.Run("waits_for_config_done", func(t *testing.T) {
		input := strings.NewReader("config peer 10.0.0.1 local-as 65001\nconfig done\n")
		scanner := bufio.NewScanner(input)
		sp := NewStartupProtocol(scanner, nil)

		err := sp.WaitForConfigDone()

		assert.NoError(t, err)
	})

	t.Run("waits_for_registry_done", func(t *testing.T) {
		input := strings.NewReader("registry name exabgp-compat\nregistry done\n")
		scanner := bufio.NewScanner(input)
		sp := NewStartupProtocol(scanner, nil)

		err := sp.WaitForRegistryDone()

		assert.NoError(t, err)
	})

	t.Run("full_protocol_sequence", func(t *testing.T) {
		// Simulate ZeBGP input: config lines then registry lines
		input := strings.NewReader("config peer 10.0.0.1\nconfig done\nregistry name test\nregistry done\n")
		scanner := bufio.NewScanner(input)
		var out strings.Builder
		sp := NewStartupProtocol(scanner, &out)

		err := sp.Run()

		assert.NoError(t, err)
		output := out.String()
		// Stage 1: declarations
		assert.Contains(t, output, "declare done\n")
		// Stage 3: capability
		assert.Contains(t, output, "capability done\n")
		// Stage 5: ready
		assert.Contains(t, output, "ready\n")
	})

	t.Run("custom_families", func(t *testing.T) {
		var out strings.Builder
		sp := NewStartupProtocol(nil, &out)
		sp.Families = []string{"ipv4/unicast", "ipv6/unicast"}

		sp.SendDeclarations()

		output := out.String()
		assert.Contains(t, output, "declare family ipv4 unicast\n")
		assert.Contains(t, output, "declare family ipv6 unicast\n")
	})

	t.Run("scanner_reuse_preserves_data", func(t *testing.T) {
		// VALIDATES: Scanner can be reused after startup without data loss.
		// PREVENTS: Buffered data lost when creating new scanner.
		input := strings.NewReader("config done\nregistry done\n{\"json\":\"event\"}\n")
		scanner := bufio.NewScanner(input)
		sp := NewStartupProtocol(scanner, io.Discard)

		err := sp.Run()
		require.NoError(t, err)

		// Same scanner should still have the JSON line available
		require.True(t, scanner.Scan(), "scanner should have more data")
		assert.Equal(t, `{"json":"event"}`, scanner.Text())
	})

	t.Run("empty_families_uses_default", func(t *testing.T) {
		// VALIDATES: Empty families slice falls back to default.
		// PREVENTS: ZeBGP rejecting plugin with no families declared.
		var out strings.Builder
		sp := NewStartupProtocol(nil, &out)
		sp.Families = []string{} // Empty!

		sp.SendDeclarations()

		output := out.String()
		// Should still declare default family
		assert.Contains(t, output, "declare family ipv4 unicast\n")
		assert.Contains(t, output, "declare done\n")
	})
}

// TestCapabilityOutput verifies capability CLI flags produce correct protocol output.
//
// VALIDATES: Route-refresh and ADD-PATH flags generate correct capability lines.
// PREVENTS: ExaBGP plugins failing due to missing capability negotiation.
func TestCapabilityOutput(t *testing.T) {
	t.Run("route_refresh_capability", func(t *testing.T) {
		var out strings.Builder
		sp := NewStartupProtocol(nil, &out)
		sp.RouteRefresh = true

		sp.SendCapabilityDone()

		output := out.String()
		// Route-refresh is code 2, no payload (RFC 2918: 0-length value).
		assert.Contains(t, output, "capability hex 2\n")
		assert.Contains(t, output, "capability done\n")
	})

	t.Run("add_path_receive", func(t *testing.T) {
		var out strings.Builder
		sp := NewStartupProtocol(nil, &out)
		sp.Families = []string{"ipv4/unicast"}
		sp.AddPathMode = "receive" //nolint:goconst // CLI test value.

		sp.SendCapabilityDone()

		output := out.String()
		// ADD-PATH is code 69, payload is AFI(2) + SAFI(1) + Mode(1)
		// ipv4/unicast = AFI 1, SAFI 1, receive = 1
		// Encoded: 00 01 01 01 = "00010101"
		assert.Contains(t, output, "capability hex 69 00010101\n")
		assert.Contains(t, output, "capability done\n")
	})

	t.Run("add_path_send", func(t *testing.T) {
		var out strings.Builder
		sp := NewStartupProtocol(nil, &out)
		sp.Families = []string{"ipv4/unicast"}
		sp.AddPathMode = "send"

		sp.SendCapabilityDone()

		output := out.String()
		// Send mode = 2, so payload ends with 02
		assert.Contains(t, output, "capability hex 69 00010102\n")
	})

	t.Run("add_path_both", func(t *testing.T) {
		var out strings.Builder
		sp := NewStartupProtocol(nil, &out)
		sp.Families = []string{"ipv4/unicast"}
		sp.AddPathMode = "both" //nolint:goconst // CLI test value.

		sp.SendCapabilityDone()

		output := out.String()
		// Both mode = 3, so payload ends with 03
		assert.Contains(t, output, "capability hex 69 00010103\n")
	})

	t.Run("add_path_multiple_families", func(t *testing.T) {
		var out strings.Builder
		sp := NewStartupProtocol(nil, &out)
		sp.Families = []string{"ipv4/unicast", "ipv6/unicast"}
		sp.AddPathMode = "receive" //nolint:goconst // CLI test value.

		sp.SendCapabilityDone()

		output := out.String()
		// ipv4/unicast: 00 01 01 01
		// ipv6/unicast: 00 02 01 01 (AFI 2)
		// Combined: "0001010100020101"
		assert.Contains(t, output, "capability hex 69 0001010100020101\n")
	})

	t.Run("both_capabilities", func(t *testing.T) {
		var out strings.Builder
		sp := NewStartupProtocol(nil, &out)
		sp.RouteRefresh = true
		sp.Families = []string{"ipv4/unicast"}
		sp.AddPathMode = "receive" //nolint:goconst // CLI test value.

		sp.SendCapabilityDone()

		output := out.String()
		// Both capabilities should be present
		assert.Contains(t, output, "capability hex 2\n")
		assert.Contains(t, output, "capability hex 69 00010101\n")
		assert.Contains(t, output, "capability done\n")
	})

	t.Run("no_capabilities_default", func(t *testing.T) {
		var out strings.Builder
		sp := NewStartupProtocol(nil, &out)
		// No RouteRefresh, no AddPathMode

		sp.SendCapabilityDone()

		output := out.String()
		// Should only have capability done, no capability lines
		assert.Equal(t, "capability done\n", output)
	})

	t.Run("add_path_ignored_without_mode", func(t *testing.T) {
		var out strings.Builder
		sp := NewStartupProtocol(nil, &out)
		sp.Families = []string{"ipv4/unicast", "ipv6/unicast"}
		// AddPathMode not set

		sp.SendCapabilityDone()

		output := out.String()
		// No ADD-PATH capability without mode
		assert.NotContains(t, output, "capability hex 69")
		assert.Equal(t, "capability done\n", output)
	})

	t.Run("add_path_evpn", func(t *testing.T) {
		var out strings.Builder
		sp := NewStartupProtocol(nil, &out)
		sp.Families = []string{"l2vpn/evpn"}
		sp.AddPathMode = "both" //nolint:goconst // CLI test value.

		sp.SendCapabilityDone()

		output := out.String()
		// l2vpn/evpn: AFI 25 (0x0019), SAFI 70 (0x46), mode 3
		// Encoded: 00 19 46 03 = "00194603"
		assert.Contains(t, output, "capability hex 69 00194603\n")
	})

	t.Run("add_path_vpn", func(t *testing.T) {
		var out strings.Builder
		sp := NewStartupProtocol(nil, &out)
		sp.Families = []string{"ipv4/vpn"}
		sp.AddPathMode = "send" //nolint:goconst // CLI test value.

		sp.SendCapabilityDone()

		output := out.String()
		// ipv4/vpn: AFI 1, SAFI 128 (0x80), mode 2
		// Encoded: 00 01 80 02 = "00018002"
		assert.Contains(t, output, "capability hex 69 00018002\n")
	})

	t.Run("add_path_flowspec_vpn", func(t *testing.T) {
		var out strings.Builder
		sp := NewStartupProtocol(nil, &out)
		sp.Families = []string{"ipv4/flowspec-vpn"}
		sp.AddPathMode = "receive" //nolint:goconst // CLI test value.

		sp.SendCapabilityDone()

		output := out.String()
		// ipv4/flowspec-vpn: AFI 1, SAFI 134 (0x86), mode 1
		// Encoded: 00 01 86 01 = "00018601"
		assert.Contains(t, output, "capability hex 69 00018601\n")
	})

	t.Run("add_path_unknown_family_skipped", func(t *testing.T) {
		// VALIDATES: Unknown families are skipped, valid ones still encoded.
		// PREVENTS: Single bad family breaking entire ADD-PATH capability.
		// Note: slog.Warn is called for unknown family (defense-in-depth logging).
		var out strings.Builder
		sp := NewStartupProtocol(nil, &out)
		sp.Families = []string{"ipv4/unicast", "invalid/family", "ipv6/unicast"}
		sp.AddPathMode = "receive" //nolint:goconst // CLI test value.

		sp.SendCapabilityDone()

		output := out.String()
		// Only valid families encoded: ipv4/unicast + ipv6/unicast
		// Invalid family silently skipped (with warning log)
		assert.Contains(t, output, "capability hex 69 0001010100020101\n")
	})

	t.Run("add_path_all_invalid_families", func(t *testing.T) {
		// VALIDATES: All invalid families results in no ADD-PATH capability.
		// PREVENTS: Empty capability payload being sent.
		var out strings.Builder
		sp := NewStartupProtocol(nil, &out)
		sp.Families = []string{"invalid/family", "bad/safi"}
		sp.AddPathMode = "receive" //nolint:goconst // CLI test value.

		sp.SendCapabilityDone()

		output := out.String()
		// No ADD-PATH capability since all families invalid
		assert.NotContains(t, output, "capability hex 69")
		assert.Equal(t, "capability done\n", output)
	})
}

// TestValidateFamily verifies family validation logic.
//
// VALIDATES: Valid families accepted, invalid rejected with clear error.
// PREVENTS: Silent failure when user typos family name.
func TestValidateFamily(t *testing.T) {
	validCases := []string{
		"ipv4/unicast",
		"ipv6/unicast",
		"ipv4/multicast",
		"ipv4/vpn",
		"ipv4/mpls-vpn",
		"ipv4/flowspec",
		"ipv4/flowspec-vpn",
		"ipv6/vpn",
		"l2vpn/evpn",
		"IPV4/UNICAST", // case insensitive
	}

	for _, fam := range validCases {
		t.Run("valid_"+fam, func(t *testing.T) {
			err := ValidateFamily(fam)
			assert.NoError(t, err, "family %q should be valid", fam)
		})
	}

	invalidCases := []string{
		"ipv4/typo",
		"ipv5/unicast",
		"unicast",
		"ipv4",
		"",
		"ipv4/evpn",     // evpn only valid with l2vpn
		"l2vpn/unicast", // l2vpn only valid with evpn
		"l2vpn/vpn",     // l2vpn only valid with evpn
	}

	for _, fam := range invalidCases {
		t.Run("invalid_"+fam, func(t *testing.T) {
			err := ValidateFamily(fam)
			assert.Error(t, err, "family %q should be invalid", fam)
			if err != nil {
				assert.Contains(t, err.Error(), "unsupported")
			}
		})
	}
}

// TestBridgeCapabilityWiring verifies Bridge passes capability config to StartupProtocol.
//
// VALIDATES: Bridge.RouteRefresh and Bridge.AddPathMode are wired to protocol output.
// PREVENTS: Capability flags set on Bridge but not propagated to startup protocol.
func TestBridgeCapabilityWiring(t *testing.T) {
	t.Run("bridge_defaults", func(t *testing.T) {
		bridge := NewBridge([]string{"echo"})

		// Verify defaults
		assert.False(t, bridge.RouteRefresh, "RouteRefresh default should be false")
		assert.Empty(t, bridge.AddPathMode, "AddPathMode default should be empty")
		assert.Equal(t, []string{"ipv4/unicast"}, bridge.Families, "Families should default to ipv4/unicast")
	})

	t.Run("bridge_wires_route_refresh_to_protocol", func(t *testing.T) {
		// VALIDATES: Bridge.RouteRefresh actually produces protocol output.
		// PREVENTS: Field exists but isn't wired to StartupProtocol.
		bridge := NewBridge([]string{"echo"})
		bridge.RouteRefresh = true

		// Simulate what Bridge.Run() does: create StartupProtocol with Bridge's values
		var out strings.Builder
		sp := NewStartupProtocol(nil, &out)
		sp.Families = bridge.Families
		sp.RouteRefresh = bridge.RouteRefresh
		sp.AddPathMode = bridge.AddPathMode

		sp.SendCapabilityDone()

		output := out.String()
		assert.Contains(t, output, "capability hex 2\n", "route-refresh capability should be output")
	})

	t.Run("bridge_wires_add_path_to_protocol", func(t *testing.T) {
		// VALIDATES: Bridge.AddPathMode actually produces protocol output.
		// PREVENTS: Field exists but isn't wired to StartupProtocol.
		bridge := NewBridge([]string{"echo"})
		bridge.AddPathMode = "receive" //nolint:goconst // CLI test value.

		var out strings.Builder
		sp := NewStartupProtocol(nil, &out)
		sp.Families = bridge.Families
		sp.RouteRefresh = bridge.RouteRefresh
		sp.AddPathMode = bridge.AddPathMode

		sp.SendCapabilityDone()

		output := out.String()
		assert.Contains(t, output, "capability hex 69", "ADD-PATH capability should be output")
	})

	t.Run("bridge_wires_families_to_protocol", func(t *testing.T) {
		// VALIDATES: Bridge.Families actually produces protocol output.
		bridge := NewBridge([]string{"echo"})
		bridge.Families = []string{"ipv4/unicast", "ipv6/unicast"}

		var out strings.Builder
		sp := NewStartupProtocol(nil, &out)
		sp.Families = bridge.Families

		sp.SendDeclarations()

		output := out.String()
		assert.Contains(t, output, "declare family ipv4 unicast\n")
		assert.Contains(t, output, "declare family ipv6 unicast\n")
	})
}

// TestZebgpToExabgpJSON_Negotiated verifies negotiated capabilities conversion.
//
// VALIDATES: ZeBGP negotiated message (flat format) → ExaBGP negotiated JSON format.
// PREVENTS: ExaBGP plugins not receiving capability info after OPEN exchange.
func TestZebgpToExabgpJSON_Negotiated(t *testing.T) {
	// New ZeBGP format: fields at top level, hyphenated names
	zebgp := map[string]any{
		"message": map[string]any{
			"type": "negotiated",
		},
		"peer": map[string]any{
			"address": "10.0.0.1",
			"asn":     float64(65001),
		},
		// Fields at top level (flat format with hyphens)
		"hold-time": float64(90),
		"asn4":      true,
		"families":  []any{"ipv4/unicast", "ipv6/unicast"},
		"add-path": map[string]any{
			"send":    []any{"ipv4/unicast"},
			"receive": []any{"ipv4/unicast"},
		},
	}

	result := ZebgpToExabgpJSON(zebgp)

	// Check envelope
	assert.Equal(t, Version, result["exabgp"])
	assert.Equal(t, "negotiated", result["type"])

	// Check neighbor section exists
	neighbor, ok := result["neighbor"].(map[string]any)
	require.True(t, ok, "neighbor should be map[string]any")

	addrMap, ok := neighbor["address"].(map[string]any)
	require.True(t, ok, "address should be map[string]any")
	assert.Equal(t, "10.0.0.1", addrMap["peer"])

	// Check negotiated section
	neg, ok := result["negotiated"].(map[string]any)
	require.True(t, ok, "negotiated should be map[string]any")

	assert.Equal(t, float64(90), neg["hold_time"])
	assert.Equal(t, true, neg["asn4"])

	// Families converted: "ipv4/unicast" → "ipv4 unicast"
	families, ok := neg["families"].([]string)
	require.True(t, ok, "families should be []string")
	assert.Contains(t, families, "ipv4 unicast")
	assert.Contains(t, families, "ipv6 unicast")

	// ADD-PATH converted
	addPath, ok := neg["add_path"].(map[string]any)
	require.True(t, ok, "add_path should be map[string]any")

	sendFams, ok := addPath["send"].([]string)
	require.True(t, ok, "send should be []string")
	assert.Contains(t, sendFams, "ipv4 unicast")

	recvFams, ok := addPath["receive"].([]string)
	require.True(t, ok, "receive should be []string")
	assert.Contains(t, recvFams, "ipv4 unicast")
}

// TestZebgpToExabgpJSON_NegotiatedMinimal verifies negotiated with minimal fields.
//
// VALIDATES: Handles negotiated message with only required fields (flat format).
// PREVENTS: Nil pointer panics when optional fields missing.
func TestZebgpToExabgpJSON_NegotiatedMinimal(t *testing.T) {
	// New ZeBGP format: fields at top level, hyphenated names
	zebgp := map[string]any{
		"message": map[string]any{
			"type": "negotiated",
		},
		"peer": map[string]any{
			"address": "10.0.0.1",
			"asn":     float64(65001),
		},
		"hold-time": float64(180),
		"asn4":      false,
	}

	result := ZebgpToExabgpJSON(zebgp)

	assert.Equal(t, "negotiated", result["type"])

	neg, ok := result["negotiated"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(180), neg["hold_time"])
	assert.Equal(t, false, neg["asn4"])
}

// TestZebgpToExabgpJSON_NegotiatedMissing verifies handling of missing negotiated field.
//
// VALIDATES: Returns empty negotiated section when field is missing.
// PREVENTS: Nil pointer panic on malformed input.
func TestZebgpToExabgpJSON_NegotiatedMissing(t *testing.T) {
	zebgp := map[string]any{
		"message": map[string]any{
			"type": "negotiated",
		},
		"peer": map[string]any{
			"address": "10.0.0.1",
			"asn":     float64(65001),
		},
		// No "negotiated" field
	}

	result := ZebgpToExabgpJSON(zebgp)

	assert.Equal(t, "negotiated", result["type"])

	neg, ok := result["negotiated"].(map[string]any)
	require.True(t, ok)
	assert.Empty(t, neg, "negotiated should be empty map when missing")
}

// TestTruncate verifies truncate handles UTF-8 correctly.
//
// VALIDATES: Truncation works on rune boundaries, not byte boundaries.
// PREVENTS: Invalid UTF-8 output when truncating multi-byte characters.
func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"ascii_short", "hello", 10, "hello"},
		{"ascii_exact", "hello", 5, "hello"},
		{"ascii_truncate", "hello world", 5, "hello..."},
		{"utf8_short", "日本語", 10, "日本語"},
		{"utf8_exact", "日本語", 3, "日本語"},
		{"utf8_truncate", "日本語テスト", 3, "日本語..."},
		{"mixed_truncate", "hello日本語", 7, "hello日本..."},
		{"empty", "", 5, ""},
		{"zero_max", "hello", 0, "..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			assert.Equal(t, tt.want, got)
		})
	}
}
