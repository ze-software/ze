package exabgp

import (
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
