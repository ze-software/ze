package bridge

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestZebgpToExabgpJSON_UpdateAnnounce verifies UPDATE announce conversion.
//
// VALIDATES: ZeBGP ze-bgp JSON JSON → ExaBGP JSON for IPv4 unicast announce.
// PREVENTS: Missing attributes, wrong family format, wrong direction mapping.
func TestZebgpToExabgpJSON_UpdateAnnounce(t *testing.T) {
	// ze-bgp JSON format: peer at bgp level, update data in bgp.update
	zebgp := map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"type": "update",
			"peer": map[string]any{
				"address": "10.0.0.1",
				"remote":  map[string]any{"address": "10.0.0.1", "as": float64(65001)},
			},
			"update": map[string]any{
				"message": map[string]any{
					"id":        float64(1),
					"direction": "received",
				},
				"attr": map[string]any{
					"origin":  "igp",
					"as-path": []any{float64(65001)},
				},
				"nlri": map[string]any{
					"ipv4/unicast": []any{
						map[string]any{
							"action":   "add",
							"next-hop": "10.0.0.1",
							"nlri":     []any{"192.168.1.0/24"},
						},
					},
				},
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

func TestZebgpToExabgpJSON_RouterID(t *testing.T) {
	zebgp := map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"type": "update",
			"peer": map[string]any{
				"remote":    map[string]any{"address": "10.0.0.1", "as": float64(65001)},
				"router-id": "1.2.3.4",
			},
			"update": map[string]any{
				"message": map[string]any{"direction": "received"},
				"attr":    map[string]any{"origin": "igp"},
				"nlri":    map[string]any{},
			},
		},
	}

	result := ZebgpToExabgpJSON(zebgp)
	neighbor, ok := result["neighbor"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "1.2.3.4", neighbor["router-id"])

	// Without router-id, field should be absent
	peerMap, ok := zebgp["bgp"].(map[string]any)["peer"].(map[string]any)
	require.True(t, ok)
	delete(peerMap, "router-id")
	result2 := ZebgpToExabgpJSON(zebgp)
	neighbor2, ok := result2["neighbor"].(map[string]any)
	require.True(t, ok)
	_, hasRouterID := neighbor2["router-id"]
	assert.False(t, hasRouterID)
}

// TestZebgpToExabgpJSON_FlowSpecExtendedCommunityNormalize verifies outgoing bridge normalization.
//
// VALIDATES: Redundant :bytes suffix is stripped; :packets passes through (matches ExaBGP main).
// PREVENTS: Outgoing JSON containing non-canonical :bytes suffix.
func TestZebgpToExabgpJSON_FlowSpecExtendedCommunityNormalize(t *testing.T) {
	zebgp := map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"type": "update",
			"peer": map[string]any{"remote": map[string]any{"address": "10.0.0.1", "as": float64(65001)}},
			"update": map[string]any{
				"message": map[string]any{"direction": "received"},
				"attr": map[string]any{
					"extended-community": []any{
						map[string]any{"string": "rate-limit:1000:packets", "value": float64(9226749737724149760)},
						map[string]any{"string": "rate-limit:9600:bytes", "value": float64(9225060887890886656)},
						map[string]any{"string": "redirect:65001:119", "value": float64(9225903013837668471)},
					},
				},
				"nlri": map[string]any{
					"ipv4/flow": []any{
						map[string]any{"action": "add", "nlri": []any{"source-ipv4 10.0.1.0/24"}},
					},
				},
			},
		},
	}

	result := ZebgpToExabgpJSON(zebgp)

	neighbor, ok := result["neighbor"].(map[string]any)
	require.True(t, ok)
	msg, ok := neighbor["message"].(map[string]any)
	require.True(t, ok)
	update, ok := msg["update"].(map[string]any)
	require.True(t, ok)
	attrs, ok := update["attribute"].(map[string]any)
	require.True(t, ok)
	extComms, ok := attrs["extended-community"].([]any)
	require.True(t, ok)
	require.Len(t, extComms, 3)

	first, ok := extComms[0].(map[string]any)
	require.True(t, ok)
	second, ok := extComms[1].(map[string]any)
	require.True(t, ok)
	third, ok := extComms[2].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "rate-limit:1000:packets", first["string"])
	assert.Equal(t, "rate-limit:9600", second["string"])
	assert.Equal(t, "redirect:65001:119", third["string"])

	// Input map stays untouched. Bridge must not mutate the caller's event payload.
	originalBGP, ok := zebgp["bgp"].(map[string]any)
	require.True(t, ok)
	originalUpdate, ok := originalBGP["update"].(map[string]any)
	require.True(t, ok)
	originalAttrs, ok := originalUpdate["attr"].(map[string]any)
	require.True(t, ok)
	originalExtComms, ok := originalAttrs["extended-community"].([]any)
	require.True(t, ok)
	originalFirst, ok := originalExtComms[0].(map[string]any)
	require.True(t, ok)
	originalSecond, ok := originalExtComms[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "rate-limit:1000:packets", originalFirst["string"])
	assert.Equal(t, "rate-limit:9600:bytes", originalSecond["string"])
}

// TestZebgpToExabgpJSON_UpdateWithdraw verifies UPDATE withdraw conversion.
//
// VALIDATES: ZeBGP ze-bgp JSON JSON → ExaBGP JSON for IPv4 unicast withdraw.
// PREVENTS: Missing withdraw section, wrong family format.
func TestZebgpToExabgpJSON_UpdateWithdraw(t *testing.T) {
	// ze-bgp JSON format
	zebgp := map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"type": "update",
			"peer": map[string]any{"remote": map[string]any{"address": "10.0.0.1", "as": float64(65001)}},
			"update": map[string]any{
				"message": map[string]any{"direction": "received"},
				"nlri": map[string]any{
					"ipv4/unicast": []any{
						map[string]any{"action": "del", "nlri": []any{"172.16.0.0/16"}},
					},
				},
			},
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
	// ze-bgp JSON format: state is simple string at bgp level
	zebgp := map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"type":  "state",
			"peer":  map[string]any{"address": "10.0.0.1", "remote": map[string]any{"as": float64(65001)}},
			"state": "up",
		},
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
			// ze-bgp JSON format: message.type and message.direction at bgp level
			zebgp := map[string]any{
				"type": "bgp",
				"bgp": map[string]any{
					"peer":    map[string]any{"address": "10.0.0.1", "remote": map[string]any{"as": float64(65001)}},
					"message": map[string]any{"type": "update", "direction": tt.direction},
					"update":  map[string]any{},
				},
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

	assert.Equal(t, "peer 10.0.0.1 update text nhop 1.1.1.1 nlri ipv4/unicast add 192.168.1.0/24", result)
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
			contain: []string{"nhop 1.1.1.1", "origin igp", "nlri ipv4/unicast add 10.0.0.0/24"},
		},
		{
			name:    "with_as_path",
			input:   "neighbor 10.0.0.1 announce route 10.0.0.0/24 next-hop 1.1.1.1 as-path [65001 65002]",
			contain: []string{"as-path 65001 65002"},
		},
		{
			name:    "with_med",
			input:   "neighbor 10.0.0.1 announce route 10.0.0.0/24 next-hop 1.1.1.1 med 100",
			contain: []string{"med 100"},
		},
		{
			name:    "with_local_pref",
			input:   "neighbor 10.0.0.1 announce route 10.0.0.0/24 next-hop 1.1.1.1 local-preference 200",
			contain: []string{"local-preference 200"},
		},
		{
			name:    "with_community",
			input:   "neighbor 10.0.0.1 announce route 10.0.0.0/24 next-hop 1.1.1.1 community 65001:100",
			contain: []string{"community 65001:100"},
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

// TestExabgpToZebgpCommand_FlowSpecRateLimitPackets verifies ExaBGP FlowSpec command bridging.
//
// VALIDATES: ExaBGP current unit syntax and 5.0 alias become Ze update text.
// PREVENTS: RFC 8955 packet-rate actions being dropped by the ExaBGP bridge.
func TestExabgpToZebgpCommand_FlowSpecRateLimitPackets(t *testing.T) {
	cmd := "neighbor 10.0.0.1 announce ipv4 flow source-ipv4 10.0.1.0/24 protocol =tcp destination-port =3128 extended-community [rate-limit:1000:packets]"
	result := ExabgpToZebgpCommand(cmd)

	assert.Equal(t, "peer 10.0.0.1 update text extended-community [rate-limit:1000:packets] nlri ipv4/flow add source-ipv4 10.0.1.0/24 protocol tcp destination-port =3128", result)
}

func TestExabgpToZebgpCommand_FlowSpecRateLimitPacketsLegacyAlias(t *testing.T) {
	cmd := "neighbor 10.0.0.1 announce ipv4 flow source-ipv4 10.0.1.0/24 protocol =tcp destination-port =3128 extended-community [rate-limit-packets:1000]"
	result := ExabgpToZebgpCommand(cmd)

	assert.Equal(t, "peer 10.0.0.1 update text extended-community [rate-limit:1000:packets] nlri ipv4/flow add source-ipv4 10.0.1.0/24 protocol tcp destination-port =3128", result)
}

func TestExabgpToZebgpCommand_FlowSpecRateLimitBytesExplicit(t *testing.T) {
	cmd := "neighbor 10.0.0.1 announce ipv4 flow source-ipv4 10.0.1.0/24 protocol =tcp extended-community [rate-limit:9600:bytes]"
	result := ExabgpToZebgpCommand(cmd)

	assert.Equal(t, "peer 10.0.0.1 update text extended-community [rate-limit:9600] nlri ipv4/flow add source-ipv4 10.0.1.0/24 protocol tcp", result)
}

func TestExabgpToZebgpCommand_FlowSpecVPNFamily(t *testing.T) {
	cmd := "neighbor 10.0.0.1 announce ipv4 flow source-ipv4 10.0.0.1/32 rd 65535:65536 extended-community [rate-limit:0]"
	result := ExabgpToZebgpCommand(cmd)

	assert.Equal(t, "peer 10.0.0.1 update text extended-community [rate-limit:0] nlri ipv4/flow-vpn add rd 65535:65536 source-ipv4 10.0.0.1/32", result)
}

// TestExabgpToZebgpCommand_FlowSpecBarePrefixKeyword drives the two bare prefix
// keywords ExaBGP accepts on input through the bridge.
//
// VALIDATES: normalizeFlowSpecComponentToken rewrites `source` and `destination`
// into the family-qualified keyword ze speaks, and the route's family decides
// which of the two spellings it means.
// PREVENTS: a hand-written ExaBGP config using the alias reaching ze as a
// keyword ze no longer has. isFlowSpecComponentKeyword refuses the bare word.
// The token is then read as an argument of the component before it, and the
// prefix leaves the announcement with no diagnostic.
func TestExabgpToZebgpCommand_FlowSpecBarePrefixKeyword(t *testing.T) {
	v4 := ExabgpToZebgpCommand("neighbor 10.0.0.1 announce ipv4 flow source 10.0.1.0/24 protocol =tcp extended-community [rate-limit:9600]")
	assert.Equal(t, "peer 10.0.0.1 update text extended-community [rate-limit:9600] nlri ipv4/flow add source-ipv4 10.0.1.0/24 protocol tcp", v4)

	v6 := ExabgpToZebgpCommand("neighbor 10.0.0.1 announce ipv6 flow destination 2001:db8::/32 next-header =tcp extended-community [rate-limit:9600]")
	assert.Equal(t, "peer 10.0.0.1 update text extended-community [rate-limit:9600] nlri ipv6/flow add destination-ipv6 2001:db8::/32 next-header tcp", v6)
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
	// Test ZeBGP ze-bgp JSON → ExaBGP preserves info
	zebgp := map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"type": "update",
			"peer": map[string]any{"remote": map[string]any{"address": "10.0.0.1", "as": float64(65001)}},
			"update": map[string]any{
				"message": map[string]any{"direction": "received"},
				"attr":    map[string]any{"origin": "igp"},
				"nlri": map[string]any{
					"ipv4/unicast": []any{
						map[string]any{"action": "add", "next-hop": "192.168.0.1", "nlri": []any{"10.0.0.0/24"}},
					},
				},
			},
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
	assert.Contains(t, zebgpCmd, "nhop 192.168.0.1")
	assert.Contains(t, zebgpCmd, "origin igp")
	assert.Contains(t, zebgpCmd, "nlri ipv4/unicast add 10.0.0.0/24")
}

// TestStartupProtocol verifies 5-stage startup protocol handling.
//
// VALIDATES: Bridge completes startup protocol before JSON translation.
// PREVENTS: Bridge killed by 5s timeout for not completing registration.
func TestStartupProtocol(t *testing.T) {
	t.Run("sends_declarations", func(t *testing.T) {
		var out strings.Builder
		sp := newStartupProtocol(nil, &out)

		sp.sendDeclarations()

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
		sp := newStartupProtocol(nil, &out)

		sp.sendCapabilityDone()

		assert.Equal(t, "capability done\n", out.String())
	})

	t.Run("sends_ready", func(t *testing.T) {
		var out strings.Builder
		sp := newStartupProtocol(nil, &out)

		sp.SendReady()

		assert.Equal(t, "ready\n", out.String())
	})

	t.Run("waits_for_config_done", func(t *testing.T) {
		input := strings.NewReader("config peer 10.0.0.1 local-as 65001\nconfig done\n")
		scanner := bufio.NewScanner(input)
		sp := newStartupProtocol(scanner, nil)

		err := sp.waitForConfigDone()

		assert.NoError(t, err)
	})

	t.Run("waits_for_registry_done", func(t *testing.T) {
		input := strings.NewReader("registry name exabgp-compat\nregistry done\n")
		scanner := bufio.NewScanner(input)
		sp := newStartupProtocol(scanner, nil)

		err := sp.waitForRegistryDone()

		assert.NoError(t, err)
	})

	t.Run("full_protocol_sequence", func(t *testing.T) {
		// Simulate ZeBGP input: config lines then registry lines
		input := strings.NewReader("config peer 10.0.0.1\nconfig done\nregistry name test\nregistry done\n")
		scanner := bufio.NewScanner(input)
		var out strings.Builder
		sp := newStartupProtocol(scanner, &out)

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
		sp := newStartupProtocol(nil, &out)
		sp.Families = []string{"ipv4/unicast", "ipv6/unicast"}

		sp.sendDeclarations()

		output := out.String()
		assert.Contains(t, output, "declare family ipv4 unicast\n")
		assert.Contains(t, output, "declare family ipv6 unicast\n")
	})

	t.Run("scanner_reuse_preserves_data", func(t *testing.T) {
		// VALIDATES: Scanner can be reused after startup without data loss.
		// PREVENTS: Buffered data lost when creating new scanner.
		input := strings.NewReader("config done\nregistry done\n{\"json\":\"event\"}\n")
		scanner := bufio.NewScanner(input)
		sp := newStartupProtocol(scanner, io.Discard)

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
		sp := newStartupProtocol(nil, &out)
		sp.Families = []string{} // Empty!

		sp.sendDeclarations()

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
		sp := newStartupProtocol(nil, &out)
		sp.RouteRefresh = true

		sp.sendCapabilityDone()

		output := out.String()
		// Route-refresh is code 2, no payload (RFC 2918: 0-length value).
		assert.Contains(t, output, "capability hex 2\n")
		assert.Contains(t, output, "capability done\n")
	})

	t.Run("add_path_receive", func(t *testing.T) {
		var out strings.Builder
		sp := newStartupProtocol(nil, &out)
		sp.Families = []string{"ipv4/unicast"}
		sp.AddPathMode = "receive" //nolint:goconst // CLI test value.

		sp.sendCapabilityDone()

		output := out.String()
		// ADD-PATH is code 69, payload is AFI(2) + SAFI(1) + Mode(1)
		// ipv4/unicast = AFI 1, SAFI 1, receive = 1
		// Encoded: 00 01 01 01 = "00010101"
		assert.Contains(t, output, "capability hex 69 00010101\n")
		assert.Contains(t, output, "capability done\n")
	})

	t.Run("add_path_send", func(t *testing.T) {
		var out strings.Builder
		sp := newStartupProtocol(nil, &out)
		sp.Families = []string{"ipv4/unicast"}
		sp.AddPathMode = "send"

		sp.sendCapabilityDone()

		output := out.String()
		// Send mode = 2, so payload ends with 02
		assert.Contains(t, output, "capability hex 69 00010102\n")
	})

	t.Run("add_path_both", func(t *testing.T) {
		var out strings.Builder
		sp := newStartupProtocol(nil, &out)
		sp.Families = []string{"ipv4/unicast"}
		sp.AddPathMode = "both" //nolint:goconst // CLI test value.

		sp.sendCapabilityDone()

		output := out.String()
		// Both mode = 3, so payload ends with 03
		assert.Contains(t, output, "capability hex 69 00010103\n")
	})

	t.Run("add_path_multiple_families", func(t *testing.T) {
		var out strings.Builder
		sp := newStartupProtocol(nil, &out)
		sp.Families = []string{"ipv4/unicast", "ipv6/unicast"}
		sp.AddPathMode = "receive" //nolint:goconst // CLI test value.

		sp.sendCapabilityDone()

		output := out.String()
		// ipv4/unicast: 00 01 01 01
		// ipv6/unicast: 00 02 01 01 (AFI 2)
		// Combined: "0001010100020101"
		assert.Contains(t, output, "capability hex 69 0001010100020101\n")
	})

	t.Run("both_capabilities", func(t *testing.T) {
		var out strings.Builder
		sp := newStartupProtocol(nil, &out)
		sp.RouteRefresh = true
		sp.Families = []string{"ipv4/unicast"}
		sp.AddPathMode = "receive" //nolint:goconst // CLI test value.

		sp.sendCapabilityDone()

		output := out.String()
		// Both capabilities should be present
		assert.Contains(t, output, "capability hex 2\n")
		assert.Contains(t, output, "capability hex 69 00010101\n")
		assert.Contains(t, output, "capability done\n")
	})

	t.Run("no_capabilities_default", func(t *testing.T) {
		var out strings.Builder
		sp := newStartupProtocol(nil, &out)
		// No RouteRefresh, no AddPathMode

		sp.sendCapabilityDone()

		output := out.String()
		// Should only have capability done, no capability lines
		assert.Equal(t, "capability done\n", output)
	})

	t.Run("add_path_ignored_without_mode", func(t *testing.T) {
		var out strings.Builder
		sp := newStartupProtocol(nil, &out)
		sp.Families = []string{"ipv4/unicast", "ipv6/unicast"}
		// AddPathMode not set

		sp.sendCapabilityDone()

		output := out.String()
		// No ADD-PATH capability without mode
		assert.NotContains(t, output, "capability hex 69")
		assert.Equal(t, "capability done\n", output)
	})

	t.Run("add_path_evpn", func(t *testing.T) {
		var out strings.Builder
		sp := newStartupProtocol(nil, &out)
		sp.Families = []string{"l2vpn/evpn"}
		sp.AddPathMode = "both" //nolint:goconst // CLI test value.

		sp.sendCapabilityDone()

		output := out.String()
		// l2vpn/evpn: AFI 25 (0x0019), SAFI 70 (0x46), mode 3
		// Encoded: 00 19 46 03 = "00194603"
		assert.Contains(t, output, "capability hex 69 00194603\n")
	})

	t.Run("add_path_vpn", func(t *testing.T) {
		var out strings.Builder
		sp := newStartupProtocol(nil, &out)
		sp.Families = []string{"ipv4/mpls-vpn"}
		sp.AddPathMode = "send" //nolint:goconst // CLI test value.

		sp.sendCapabilityDone()

		output := out.String()
		// ipv4/mpls-vpn: AFI 1, SAFI 128 (0x80), mode 2
		// Encoded: 00 01 80 02 = "00018002"
		assert.Contains(t, output, "capability hex 69 00018002\n")
	})

	t.Run("add_path_flow_vpn", func(t *testing.T) {
		var out strings.Builder
		sp := newStartupProtocol(nil, &out)
		sp.Families = []string{"ipv4/flow-vpn"}
		sp.AddPathMode = "receive" //nolint:goconst // CLI test value.

		sp.sendCapabilityDone()

		output := out.String()
		// ipv4/flow-vpn: AFI 1, SAFI 134 (0x86), mode 1
		// Encoded: 00 01 86 01 = "00018601"
		assert.Contains(t, output, "capability hex 69 00018601\n")
	})

	t.Run("add_path_unknown_family_skipped", func(t *testing.T) {
		// VALIDATES: Unknown families are skipped, valid ones still encoded.
		// PREVENTS: Single bad family breaking entire ADD-PATH capability.
		// Note: slog.Warn is called for unknown family (defense-in-depth logging).
		var out strings.Builder
		sp := newStartupProtocol(nil, &out)
		sp.Families = []string{"ipv4/unicast", "invalid/family", "ipv6/unicast"}
		sp.AddPathMode = "receive" //nolint:goconst // CLI test value.

		sp.sendCapabilityDone()

		output := out.String()
		// Only valid families encoded: ipv4/unicast + ipv6/unicast
		// Invalid family silently skipped (with warning log)
		assert.Contains(t, output, "capability hex 69 0001010100020101\n")
	})

	t.Run("add_path_all_invalid_families", func(t *testing.T) {
		// VALIDATES: All invalid families results in no ADD-PATH capability.
		// PREVENTS: Empty capability payload being sent.
		var out strings.Builder
		sp := newStartupProtocol(nil, &out)
		sp.Families = []string{"invalid/family", "bad/safi"}
		sp.AddPathMode = "receive" //nolint:goconst // CLI test value.

		sp.sendCapabilityDone()

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
		"ipv4/mpls-vpn",
		"ipv4/mpls-vpn",
		"ipv4/flow",
		"ipv4/flow-vpn",
		"ipv6/mpls-vpn",
		"l2vpn/evpn",
		"ipv4/sr-policy",
		"ipv6/sr-policy",
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

// TestEncodeAddPathHex verifies standalone ADD-PATH capability encoding.
//
// VALIDATES: EncodeAddPathHex produces correct RFC 7911 hex payload.
// PREVENTS: SDK mode sending malformed ADD-PATH capability to engine.
func TestEncodeAddPathHex(t *testing.T) {
	tests := []struct {
		name     string
		families []string
		mode     string
		want     string
	}{
		{"receive_ipv4", []string{"ipv4/unicast"}, "receive", "00010101"},
		{"send_ipv4", []string{"ipv4/unicast"}, "send", "00010102"},
		{"both_ipv4", []string{"ipv4/unicast"}, "both", "00010103"},
		{"both_ipv6", []string{"ipv6/unicast"}, "both", "00020103"},
		{"multi_family", []string{"ipv4/unicast", "ipv6/unicast"}, "both", "0001010300020103"},
		{"invalid_mode", []string{"ipv4/unicast"}, "invalid", ""},
		{"empty_mode", []string{"ipv4/unicast"}, "", ""},
		{"empty_families", nil, "both", "00010103"}, // defaults to ipv4/unicast
		{"unknown_family", []string{"unknown/family"}, "both", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EncodeAddPathHex(tt.families, tt.mode)
			assert.Equal(t, tt.want, got)
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
		sp := newStartupProtocol(nil, &out)
		sp.Families = bridge.Families
		sp.RouteRefresh = bridge.RouteRefresh
		sp.AddPathMode = bridge.AddPathMode

		sp.sendCapabilityDone()

		output := out.String()
		assert.Contains(t, output, "capability hex 2\n", "route-refresh capability should be output")
	})

	t.Run("bridge_wires_add_path_to_protocol", func(t *testing.T) {
		// VALIDATES: Bridge.AddPathMode actually produces protocol output.
		// PREVENTS: Field exists but isn't wired to StartupProtocol.
		bridge := NewBridge([]string{"echo"})
		bridge.AddPathMode = "receive" //nolint:goconst // CLI test value.

		var out strings.Builder
		sp := newStartupProtocol(nil, &out)
		sp.Families = bridge.Families
		sp.RouteRefresh = bridge.RouteRefresh
		sp.AddPathMode = bridge.AddPathMode

		sp.sendCapabilityDone()

		output := out.String()
		assert.Contains(t, output, "capability hex 69", "ADD-PATH capability should be output")
	})

	t.Run("bridge_wires_families_to_protocol", func(t *testing.T) {
		// VALIDATES: Bridge.Families actually produces protocol output.
		bridge := NewBridge([]string{"echo"})
		bridge.Families = []string{"ipv4/unicast", "ipv6/unicast"}

		var out strings.Builder
		sp := newStartupProtocol(nil, &out)
		sp.Families = bridge.Families

		sp.sendDeclarations()

		output := out.String()
		assert.Contains(t, output, "declare family ipv4 unicast\n")
		assert.Contains(t, output, "declare family ipv6 unicast\n")
	})
}

// TestZebgpToExabgpJSON_Negotiated verifies negotiated capabilities conversion.
//
// VALIDATES: ZeBGP ze-bgp JSON negotiated message → ExaBGP negotiated JSON format.
// PREVENTS: ExaBGP plugins not receiving capability info after OPEN exchange.
func TestZebgpToExabgpJSON_Negotiated(t *testing.T) {
	// ze-bgp JSON format: message.type at bgp level, negotiated data in bgp.negotiated
	zebgp := map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"peer": map[string]any{
				"address": "10.0.0.1",
				"remote":  map[string]any{"address": "10.0.0.1", "as": float64(65001)},
			},
			"message": map[string]any{
				"type": "negotiated",
			},
			"negotiated": map[string]any{
				"timer":    map[string]any{"hold-time": float64(90)},
				"asn4":     true,
				"families": []any{"ipv4/unicast", "ipv6/unicast"},
				"add-path": map[string]any{
					"send":    []any{"ipv4/unicast"},
					"receive": []any{"ipv4/unicast"},
				},
			},
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
// VALIDATES: Handles negotiated message with only required fields.
// PREVENTS: Nil pointer panics when optional fields missing.
func TestZebgpToExabgpJSON_NegotiatedMinimal(t *testing.T) {
	// Ze format with minimal fields (type in message).
	zebgp := map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"peer": map[string]any{
				"address": "10.0.0.1",
				"remote":  map[string]any{"address": "10.0.0.1", "as": float64(65001)},
			},
			"message": map[string]any{
				"type": "negotiated",
			},
			"negotiated": map[string]any{
				"timer": map[string]any{"hold-time": float64(180)},
				"asn4":  false,
			},
		},
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
	// ze-bgp JSON format with no negotiated object
	zebgp := map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"peer": map[string]any{
				"address": "10.0.0.1",
				"remote":  map[string]any{"address": "10.0.0.1", "as": float64(65001)},
			},
			"message": map[string]any{
				"type": "negotiated",
			},
			// No "negotiated" field
		},
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

// TestParseMuxLine verifies MuxConn line parsing.
//
// VALIDATES: MuxConn lines are correctly parsed into id, verb, payload.
// PREVENTS: Events dropped because bridge cannot parse MuxConn wire format.
func TestParseMuxLine(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantID  uint64
		wantV   string
		wantP   string
		wantErr bool
	}{
		{
			name:   "deliver_batch",
			line:   `#42 ze-plugin-callback:deliver-batch {"events":["ev1"]}`,
			wantID: 42,
			wantV:  "ze-plugin-callback:deliver-batch",
			wantP:  `{"events":["ev1"]}`,
		},
		{
			name:   "ok_response",
			line:   `#7 ok {"status":"done"}`,
			wantID: 7,
			wantV:  "ok",
			wantP:  `{"status":"done"}`,
		},
		{
			name:   "ok_empty",
			line:   "#99 ok",
			wantID: 99,
			wantV:  "ok",
			wantP:  "",
		},
		{
			name:   "error_response",
			line:   `#5 error {"message":"fail"}`,
			wantID: 5,
			wantV:  "error",
			wantP:  `{"message":"fail"}`,
		},
		{
			name:    "missing_hash",
			line:    "42 ok",
			wantErr: true,
		},
		{
			name:    "empty_line",
			line:    "",
			wantErr: true,
		},
		{
			name:    "hash_only",
			line:    "#42",
			wantErr: true,
		},
		{
			name:   "large_id",
			line:   "#18446744073709551615 ok",
			wantID: 18446744073709551615,
			wantV:  "ok",
			wantP:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, verb, payload, err := parseMuxLine(tt.line)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, id)
			assert.Equal(t, tt.wantV, verb)
			assert.Equal(t, tt.wantP, payload)
		})
	}
}

// TestFormatMuxResponse verifies MuxConn response formatting.
//
// VALIDATES: Responses formatted as #<id> ok\n for acknowledgements.
// PREVENTS: Ze dropping bridge responses due to malformed framing.
func TestFormatMuxResponse(t *testing.T) {
	tests := []struct {
		name string
		id   uint64
		want string
	}{
		{"zero_id", 0, "#0 ok"},
		{"normal_id", 42, "#42 ok"},
		{"large_id", 999999, "#999999 ok"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatMuxOK(tt.id)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestFormatMuxRequest verifies MuxConn request formatting for dispatch.
//
// VALIDATES: Commands wrapped as #<id> ze-plugin-engine:dispatch-command {"command":"..."}\n.
// PREVENTS: Ze dropping bridge commands due to missing MuxConn framing.
func TestFormatMuxRequest(t *testing.T) {
	tests := []struct {
		name    string
		id      uint64
		command string
		want    string
	}{
		{
			name:    "simple_command",
			id:      1,
			command: "peer 10.0.0.1 update text nhop 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24",
			want:    `#1 ze-plugin-engine:dispatch-command {"command":"peer 10.0.0.1 update text nhop 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24"}`,
		},
		{
			name:    "command_with_special_chars",
			id:      99,
			command: `peer 2001:db8::1 update text nhop 2001:db8::2 nlri ipv6/unicast add 2001:db8::/32`,
			want:    `#99 ze-plugin-engine:dispatch-command {"command":"peer 2001:db8::1 update text nhop 2001:db8::2 nlri ipv6/unicast add 2001:db8::/32"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDispatchRequest(tt.id, tt.command)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestExtractBatchEvents verifies deliver-batch event extraction.
//
// VALIDATES: Events extracted from deliver-batch JSON payload.
// PREVENTS: Events lost because bridge cannot extract them from MuxConn batch payload.
func TestExtractBatchEvents(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    []string
		wantErr bool
	}{
		{
			name:    "single_event",
			payload: `{"events":["{\"type\":\"bgp\"}"]}`,
			want:    []string{`{"type":"bgp"}`},
		},
		{
			name:    "multiple_events",
			payload: `{"events":["{\"type\":\"bgp\",\"bgp\":{}}","{\"type\":\"bgp\",\"bgp\":{}}"]}`,
			want:    []string{`{"type":"bgp","bgp":{}}`, `{"type":"bgp","bgp":{}}`},
		},
		{
			name:    "empty_events",
			payload: `{"events":[]}`,
			want:    []string{},
		},
		{
			name:    "invalid_json",
			payload: `not json`,
			wantErr: true,
		},
		{
			name:    "missing_events_key",
			payload: `{"data":"foo"}`,
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, err := extractBatchEvents(tt.payload)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, events)
		})
	}
}

// TestMuxConnEventTranslation verifies end-to-end MuxConn event reception and translation.
//
// VALIDATES: Bridge receives MuxConn deliver-batch, translates events, forwards to plugin, sends ok.
// PREVENTS: Events silently dropped after 5-stage startup.
func TestMuxConnEventTranslation(t *testing.T) {
	// Build a MuxConn deliver-batch line containing a ze-bgp UPDATE event.
	zeEvent := `{"type":"bgp","bgp":{"peer":{"remote":{"address":"10.0.0.1","as":65001}},"message":{"type":"update","direction":"received"},"update":{"attr":{"origin":"igp"},"nlri":{"ipv4/unicast":[{"action":"add","next-hop":"10.0.0.1","nlri":["192.168.1.0/24"]}]}}}}`
	// The event is JSON-encoded as a string inside the events array.
	eventsJSON, err := json.Marshal([]string{zeEvent})
	require.NoError(t, err)
	batchLine := fmt.Sprintf(`#1 ze-plugin-callback:deliver-batch {"events":%s}`, string(eventsJSON))

	// Set up a scanner reading the batch line.
	scanner := bufio.NewScanner(strings.NewReader(batchLine + "\n"))

	// Capture plugin output and ze response output.
	var pluginBuf strings.Builder
	var zeBuf strings.Builder
	zeOut := &syncWriter{w: &zeBuf}

	b := NewBridge([]string{"echo"})
	b.running = true

	ctx := context.Background()
	b.zebgpToPluginWithScanner(ctx, scanner, &pluginBuf, zeOut, newPendingResponses())

	// The bridge should have written ExaBGP JSON to the plugin.
	pluginOutput := pluginBuf.String()
	assert.Contains(t, pluginOutput, "exabgp")
	assert.Contains(t, pluginOutput, "10.0.0.1")

	// The bridge should have sent #1 ok back to ze.
	zeOutput := zeBuf.String()
	assert.Contains(t, zeOutput, "#1 ok")
}

// TestMuxConnCommandFormatting verifies bridge wraps commands in MuxConn dispatch format
// and injects a flush after route commands.
//
// VALIDATES: ExaBGP commands translated, wrapped in MuxConn dispatch-command RPC,
// followed by a ze-bgp:peer-flush RPC that blocks until response.
// PREVENTS: Commands dropped by ze because bridge writes raw text instead of MuxConn frames.
func TestMuxConnCommandFormatting(t *testing.T) {
	pluginOutput := "neighbor 10.0.0.1 announce route 192.168.1.0/24 next-hop 1.1.1.1\n"

	pluginReader := strings.NewReader(pluginOutput)
	var zeBuf strings.Builder
	var pluginAckBuf strings.Builder
	zeOut := &syncWriter{w: &zeBuf}
	pending := newPendingResponses()

	b := NewBridge([]string{"echo"})
	b.running = true

	// pluginToZebgp blocks on dispatch ack, then the flush response.
	// Signal each only once pluginToZebgp has registered the corresponding
	// waiter, so the signal lands on a real waiter instead of racing a sleep.
	go func() {
		require.Eventually(t, func() bool { return pending.isWaiting(1) },
			2*time.Second, time.Millisecond, "dispatch waiter (id 1) should register")
		pending.signal(1, pendingResult{ok: true}) // dispatch ack
		require.Eventually(t, func() bool { return pending.isWaiting(2) },
			2*time.Second, time.Millisecond, "flush waiter (id 2) should register")
		pending.signal(2, pendingResult{ok: true}) // flush
	}()

	ctx := context.Background()
	b.pluginToZebgp(ctx, pluginReader, &pluginAckBuf, zeOut, pending)

	// The output to ze should have both dispatch-command and peer-flush.
	zeOutput := zeBuf.String()
	assert.Contains(t, zeOutput, "ze-plugin-engine:dispatch-command")
	assert.Contains(t, zeOutput, "peer 10.0.0.1 update text")
	assert.Contains(t, zeOutput, "nlri ipv4/unicast add 192.168.1.0/24")
	assert.Contains(t, zeOutput, "ze-bgp:peer-flush")
	assert.Contains(t, zeOutput, `"selector":"10.0.0.1"`)
}

// TestMuxConnDeliverEvent verifies single event delivery via deliver-event RPC.
//
// VALIDATES: Bridge parses deliver-event MuxConn request, translates, forwards to plugin.
// PREVENTS: Single-event delivery broken while batch works.
func TestMuxConnDeliverEvent(t *testing.T) {
	zeEvent := `{"type":"bgp","bgp":{"peer":{"remote":{"address":"10.0.0.2","as":65002}},"message":{"type":"state","direction":"received"},"state":"up"}}`
	eventPayload, err := json.Marshal(map[string]string{"event": zeEvent})
	require.NoError(t, err)
	line := fmt.Sprintf("#5 ze-plugin-callback:deliver-event %s", string(eventPayload))

	scanner := bufio.NewScanner(strings.NewReader(line + "\n"))

	var pluginBuf strings.Builder
	var zeBuf strings.Builder
	zeOut := &syncWriter{w: &zeBuf}

	b := NewBridge([]string{"echo"})
	b.running = true

	ctx := context.Background()
	b.zebgpToPluginWithScanner(ctx, scanner, &pluginBuf, zeOut, newPendingResponses())

	// Plugin should receive ExaBGP formatted state event.
	pluginOutput := pluginBuf.String()
	assert.Contains(t, pluginOutput, "exabgp")
	assert.Contains(t, pluginOutput, "10.0.0.2")
	assert.Contains(t, pluginOutput, "state")

	// Bridge should send ok response back to ze.
	assert.Contains(t, zeBuf.String(), "#5 ok")
}

// TestMuxConnUnknownVerbRespondsOK verifies unknown verbs get an ok response.
//
// VALIDATES: Bridge does not stall ze on unknown MuxConn verbs.
// PREVENTS: Ze hanging because bridge never responds to an unknown request.
func TestMuxConnUnknownVerbRespondsOK(t *testing.T) {
	line := `#99 ze-plugin-callback:some-future-rpc {"data":"test"}`
	scanner := bufio.NewScanner(strings.NewReader(line + "\n"))

	var pluginBuf strings.Builder
	var zeBuf strings.Builder
	zeOut := &syncWriter{w: &zeBuf}

	b := NewBridge([]string{"echo"})
	b.running = true

	ctx := context.Background()
	b.zebgpToPluginWithScanner(ctx, scanner, &pluginBuf, zeOut, newPendingResponses())

	// Bridge should respond ok even for unknown verbs.
	assert.Contains(t, zeBuf.String(), "#99 ok")

	// Plugin should receive nothing (unknown verb not forwarded).
	assert.Empty(t, pluginBuf.String())
}

// TestMuxConnDispatchResponseIgnored verifies ok/error responses are silently handled.
//
// VALIDATES: Dispatch responses (from commands sent by pluginToZebgp) do not cause errors.
// PREVENTS: Bridge treating dispatch responses as unknown requests.
func TestMuxConnDispatchResponseIgnored(t *testing.T) {
	// Simulate a dispatch response arriving on the event scanner.
	line := `#1 ok {"status":"done"}`
	scanner := bufio.NewScanner(strings.NewReader(line + "\n"))

	var pluginBuf strings.Builder
	var zeBuf strings.Builder
	zeOut := &syncWriter{w: &zeBuf}

	b := NewBridge([]string{"echo"})
	b.running = true

	ctx := context.Background()
	b.zebgpToPluginWithScanner(ctx, scanner, &pluginBuf, zeOut, newPendingResponses())

	// No output to plugin (response, not an event).
	assert.Empty(t, pluginBuf.String())

	// No output to ze (response doesn't need a response).
	assert.Empty(t, zeBuf.String())
}

// TestMuxConnAnswerLinesAreNotAcknowledged checks that the lines of a record
// answer reach the waiter that asked for it and are never acknowledged back to
// ze. The method: a head, one record and a terminator arrive under the id of a
// registered waiter, and the test reads what the bridge wrote to ze and to the
// plugin.
//
// A dispatch the bridge sends is answered by that sequence, so every kind has
// to be named on the response arm. On the unknown-verb arm each line would earn
// an `ok` line ze never asked for, and the dispatch would wait for a result
// that already went past.
//
// VALIDATES: the bridge speaks the answer grammar rather than a second copy of
// it, and a kind token is a response and not an unknown verb.
// PREVENTS: the bridge acknowledging the daemon's own answer lines, which puts
// unrequested responses on the wire and stalls every dispatch it sends.
func TestMuxConnAnswerLinesAreNotAcknowledged(t *testing.T) {
	answer := strings.Join([]string{
		`#6 top map 5:peers 0:`,
		`#6 row 19:{"peer":"10.0.0.1"}`,
		`#6 end 1 0 0:`,
	}, "\n")
	scanner := bufio.NewScanner(strings.NewReader(answer + "\n"))

	var pluginBuf strings.Builder
	var zeBuf strings.Builder
	zeOut := &syncWriter{w: &zeBuf}

	b := NewBridge([]string{"echo"})
	b.running = true

	pending := newPendingResponses()
	waiter := pending.register(6)

	ctx := context.Background()
	b.zebgpToPluginWithScanner(ctx, scanner, &pluginBuf, zeOut, pending)

	assert.Empty(t, zeBuf.String(), "an answer line needs no acknowledgement")
	assert.Empty(t, pluginBuf.String(), "an answer line is not an event to translate")

	select {
	case result := <-waiter:
		assert.True(t, result.ok, "the answer released the dispatch that asked for it")
	default:
		t.Fatal("the answer never released the waiter, so a dispatch would block until its context expired")
	}
}

// TestMuxConnNotUnderstoodAnswerFailsTheWaiter checks that a command text the
// daemon did not understand fails the dispatch that sent it. The method: one
// nay line arrives under the id of a registered waiter, and the test reads what
// the waiter received.
//
// VALIDATES: the nay kind carries a failure, so a bridge dispatch of a command
// that does not exist is reported rather than reported as success, and the
// reason reaches the plugin decoded rather than as the positional tail it
// traveled in.
// PREVENTS: a typo reaching ExaBGP as a command that ran, and an operator
// reading the counted lengths in front of the message as part of it.
func TestMuxConnNotUnderstoodAnswerFailsTheWaiter(t *testing.T) {
	line := "#6 " + string(rpc.AppendAnswerNotUnderstood(nil, rpc.AnswerNoID, "", "unknown command: shwo bgp peers"))
	scanner := bufio.NewScanner(strings.NewReader(line + "\n"))

	var pluginBuf strings.Builder
	var zeBuf strings.Builder
	zeOut := &syncWriter{w: &zeBuf}

	b := NewBridge([]string{"echo"})
	b.running = true

	pending := newPendingResponses()
	waiter := pending.register(6)

	ctx := context.Background()
	b.zebgpToPluginWithScanner(ctx, scanner, &pluginBuf, zeOut, pending)

	assert.Empty(t, zeBuf.String(), "a nay line needs no acknowledgement")

	select {
	case result := <-waiter:
		assert.False(t, result.ok, "a command the daemon did not understand is a failure")
		assert.Equal(t, "unknown command: shwo bgp peers", result.errText,
			"the reason reached the plugin with its counted lengths still in front of it")
	default:
		t.Fatal("the nay line never released the waiter")
	}
}

// TestMuxConnMultipleBatchEvents verifies multiple events in a single batch.
//
// VALIDATES: All events in a deliver-batch are translated and forwarded.
// PREVENTS: Only first event processed, rest dropped.
func TestMuxConnMultipleBatchEvents(t *testing.T) {
	event1 := `{"type":"bgp","bgp":{"peer":{"remote":{"address":"10.0.0.1","as":65001}},"message":{"type":"update","direction":"received"},"update":{"nlri":{"ipv4/unicast":[{"action":"add","next-hop":"10.0.0.1","nlri":["192.168.1.0/24"]}]}}}}`
	event2 := `{"type":"bgp","bgp":{"peer":{"remote":{"address":"10.0.0.2","as":65002}},"message":{"type":"update","direction":"received"},"update":{"nlri":{"ipv4/unicast":[{"action":"add","next-hop":"10.0.0.2","nlri":["172.16.0.0/16"]}]}}}}`
	eventsJSON, err := json.Marshal([]string{event1, event2})
	require.NoError(t, err)
	line := fmt.Sprintf(`#10 ze-plugin-callback:deliver-batch {"events":%s}`, string(eventsJSON))

	scanner := bufio.NewScanner(strings.NewReader(line + "\n"))

	var pluginBuf strings.Builder
	var zeBuf strings.Builder
	zeOut := &syncWriter{w: &zeBuf}

	b := NewBridge([]string{"echo"})
	b.running = true

	ctx := context.Background()
	b.zebgpToPluginWithScanner(ctx, scanner, &pluginBuf, zeOut, newPendingResponses())

	// Both events should be translated and forwarded.
	pluginOutput := pluginBuf.String()
	assert.Contains(t, pluginOutput, "10.0.0.1")
	assert.Contains(t, pluginOutput, "192.168.1.0/24")
	assert.Contains(t, pluginOutput, "10.0.0.2")
	assert.Contains(t, pluginOutput, "172.16.0.0/16")

	// Single ok response for the batch.
	assert.Contains(t, zeBuf.String(), "#10 ok")
}

// TestExtractPeerAddress verifies peer address extraction from translated commands.
//
// VALIDATES: Peer address extracted correctly for flush injection.
// PREVENTS: Flush sent with wrong selector or empty selector.
func TestExtractPeerAddress(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string
	}{
		{"ipv4", "peer 10.0.0.1 update text nhop 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24", "10.0.0.1"},
		{"ipv6", "peer 2001:db8::1 update text nhop 2001:db8::2 nlri ipv6/unicast add 2001:db8::/32", "2001:db8::1"},
		{"not_peer", "show bgp", ""},
		{"peer_only", "peer ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ExtractPeerAddress(tt.command))
		})
	}
}

// TestIsRouteCommand verifies route command detection.
//
// VALIDATES: Only route update commands trigger flush injection.
// PREVENTS: Flush injected for non-route commands.
func TestIsRouteCommand(t *testing.T) {
	assert.True(t, IsRouteCommand("peer 10.0.0.1 update text nhop 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24"))
	assert.False(t, IsRouteCommand("peer 10.0.0.1 show bgp"))
	assert.False(t, IsRouteCommand(""))
}

// TestFlushBlocksUntilResponse verifies that pluginToZebgp blocks on flush until
// the event goroutine signals the response.
//
// VALIDATES: AC-3 -- flush injection blocks until response arrives.
// PREVENTS: Bridge continuing before engine finishes processing routes.
func TestFlushBlocksUntilResponse(t *testing.T) {
	pluginOutput := "neighbor 10.0.0.1 announce route 10.0.0.0/24 next-hop 1.1.1.1\n"

	pluginReader := strings.NewReader(pluginOutput)
	var zeBuf strings.Builder
	var pluginAckBuf strings.Builder
	zeOut := &syncWriter{w: &zeBuf}
	pending := newPendingResponses()

	b := NewBridge([]string{"echo"})
	b.running = true

	done := make(chan struct{})
	go func() {
		defer close(done)
		ctx := context.Background()
		b.pluginToZebgp(ctx, pluginReader, &pluginAckBuf, zeOut, pending)
	}()

	// Ack the dispatch first so pluginToZebgp reaches the flush wait.
	// Wait until the dispatch waiter is registered so the signal cannot race
	// ahead of pluginToZebgp's register/wait.
	require.Eventually(t, func() bool { return pending.isWaiting(1) },
		2*time.Second, time.Millisecond, "dispatch waiter (id 1) should register")
	pending.signal(1, pendingResult{ok: true})

	// pluginToZebgp should NOT finish yet -- it's blocked on flush.
	select {
	case <-done:
		t.Fatal("pluginToZebgp should block until flush response")
	case <-time.After(50 * time.Millisecond):
		// Expected: still blocked.
	}

	// Signal the flush response. Confirm the flush waiter is registered first
	// so the signal lands on a real waiter rather than racing registration.
	require.Eventually(t, func() bool { return pending.isWaiting(2) },
		2*time.Second, time.Millisecond, "flush waiter (id 2) should register")
	pending.signal(2, pendingResult{ok: true}) // flush ID

	// Now it should finish.
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("pluginToZebgp did not complete after flush signal")
	}
}

// TestPendingResponseSignalAndWait verifies the pending response mechanism.
//
// VALIDATES: register/signal/wait cycle works correctly.
// PREVENTS: Flush hanging forever or signaling wrong waiter.
func TestPendingResponseSignalAndWait(t *testing.T) {
	p := newPendingResponses()

	// Register and signal immediately.
	ch := p.register(42)
	assert.True(t, p.signal(42, pendingResult{ok: true}))
	res, err := p.wait(context.Background(), 42, ch)
	assert.NoError(t, err)
	assert.True(t, res.ok)

	// Signal without registration returns false.
	assert.False(t, p.signal(99, pendingResult{ok: true}))

	// Wait with canceled context returns error and removes the waiter.
	ch2 := p.register(100)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = p.wait(ctx, 100, ch2)
	assert.Error(t, err)
	// Leak check: a late signal on id=100 must find no waiter now.
	assert.False(t, p.signal(100, pendingResult{ok: true}), "timed-out waiter should have been removed from the map")
}

// TestBridgeSRPolicyFamilyParse verifies sr-policy family parsing in parseFamilyToAFISAFI.
//
// VALIDATES: AC-8, AC-9 -- parseFamilyToAFISAFI returns correct AFI/SAFI for sr-policy.
// PREVENTS: sr-policy family rejected by bridge.
func TestBridgeSRPolicyFamilyParse(t *testing.T) {
	afi, safi := parseFamilyToAFISAFI("ipv4/sr-policy")
	assert.Equal(t, uint16(1), afi)
	assert.Equal(t, uint16(73), safi)

	afi, safi = parseFamilyToAFISAFI("ipv6/sr-policy")
	assert.Equal(t, uint16(2), afi)
	assert.Equal(t, uint16(73), safi)

	// L2VPN should not support sr-policy
	afi, safi = parseFamilyToAFISAFI("l2vpn/sr-policy")
	assert.Equal(t, uint16(0), afi)
	assert.Equal(t, uint16(0), safi)
}

// TestBridgeSRPolicyCommand verifies ExaBGP SR-Policy command translation.
//
// VALIDATES: AC-11 -- ExaBGP sr-policy announce translated to Ze format.
// PREVENTS: SR-Policy commands silently dropped or mistranslated.
func TestBridgeSRPolicyCommand(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"announce_ipv4",
			"neighbor 10.0.0.1 announce ipv4 sr-policy distinguisher 0 color 100 endpoint 10.0.0.1 next-hop 1.2.3.4",
			"peer 10.0.0.1 update text nhop 1.2.3.4 nlri ipv4/sr-policy add distinguisher 0 color 100 endpoint 10.0.0.1",
		},
		{
			"announce_ipv4_with_preference",
			"neighbor 10.0.0.1 announce ipv4 sr-policy distinguisher 0 color 100 endpoint 10.0.0.1 next-hop 1.2.3.4 preference 200",
			"peer 10.0.0.1 update text nhop 1.2.3.4 nlri ipv4/sr-policy add distinguisher 0 color 100 endpoint 10.0.0.1 preference 200",
		},
		{
			"announce_ipv4_full_tunnel_encap",
			"neighbor 10.0.0.1 announce ipv4 sr-policy distinguisher 0 color 100 endpoint 10.0.0.1 next-hop 1.2.3.4 preference 100 priority 10 binding-sid mpls 24000 segment-list weight 1 segment type-a mpls 16001",
			"peer 10.0.0.1 update text nhop 1.2.3.4 nlri ipv4/sr-policy add distinguisher 0 color 100 endpoint 10.0.0.1 preference 100 priority 10 binding-sid mpls 24000 segment-list weight 1 segment type-a mpls 16001",
		},
		{
			"announce_ipv6",
			"neighbor 10.0.0.1 announce ipv6 sr-policy distinguisher 1 color 200 endpoint 2001:db8::1 next-hop 2001:db8::2",
			"peer 10.0.0.1 update text nhop 2001:db8::2 nlri ipv6/sr-policy add distinguisher 1 color 200 endpoint 2001:db8::1",
		},
		{
			"withdraw_ipv4",
			"neighbor 10.0.0.1 withdraw ipv4 sr-policy distinguisher 0 color 100 endpoint 10.0.0.1",
			"peer 10.0.0.1 update text nlri ipv4/sr-policy del distinguisher 0 color 100 endpoint 10.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExabgpToZebgpCommand(tt.input)
			assert.Equal(t, tt.want, result)
		})
	}
}
