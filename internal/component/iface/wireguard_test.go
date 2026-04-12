package iface

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// Fixed test keys. Using hardcoded base64 keeps tests deterministic and
// readable. These are not real secrets -- do NOT use in production config.
const (
	// 32 bytes of 0x01, base64-encoded.
	testPrivKeyB64 = "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE="
	// 32 bytes of 0x02.
	testPubKey1B64 = "AgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgI="
	// 32 bytes of 0x03.
	testPubKey2B64 = "AwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwM="
	// 32 bytes of 0x04.
	testPSKB64 = "BAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQ="
)

// TestParseWireguardMinimal verifies the happy path for a wireguard entry
// with one peer, matching AC-1 expectations.
//
// VALIDATES: AC-1 — config with wireguard wg0 + private-key + one peer
// with public-key, endpoint, allowed-ips parses into a valid wireguardEntry.
// PREVENTS: Silent field drop when parser adds new leaves.
func TestParseWireguardMinimal(t *testing.T) {
	m := map[string]any{
		"private-key": testPrivKeyB64,
		"listen-port": "51820",
		"fwmark":      "0",
		"peer": map[string]any{
			"site2": map[string]any{
				"public-key": testPubKey1B64,
				"endpoint": map[string]any{
					"ip":   "198.51.100.2",
					"port": "51820",
				},
				"allowed-ips": []any{"10.0.0.2/32", "192.168.10.0/24"},
			},
		},
	}

	entry, err := parseWireguardEntry("wg0", m)
	require.NoError(t, err)

	assert.Equal(t, "wg0", entry.Spec.Name)
	assert.True(t, entry.Spec.ListenPortSet)
	assert.Equal(t, uint16(51820), entry.Spec.ListenPort)
	assert.Equal(t, uint32(0), entry.Spec.FirewallMark)

	// Private key round-trip: base64 in → same base64 out via String()
	assert.Equal(t, testPrivKeyB64, entry.Spec.PrivateKey.String())

	require.Len(t, entry.Spec.Peers, 1)
	peer := entry.Spec.Peers[0]
	assert.Equal(t, "site2", peer.Name)
	assert.Equal(t, testPubKey1B64, peer.PublicKey.String())
	assert.Equal(t, "198.51.100.2", peer.EndpointIP)
	assert.Equal(t, uint16(51820), peer.EndpointPort)
	assert.ElementsMatch(t, []string{"10.0.0.2/32", "192.168.10.0/24"}, peer.AllowedIPs)
	assert.False(t, peer.HasPresharedKey)
	assert.False(t, peer.Disable)
}

// TestParseWireguardTwoPeers verifies that a wireguard block with two peers
// parses into two WireguardPeerSpec entries.
//
// VALIDATES: Peer list cardinality and per-peer field isolation.
// PREVENTS: Peer fields bleeding between list entries.
func TestParseWireguardTwoPeers(t *testing.T) {
	m := map[string]any{
		"private-key": testPrivKeyB64,
		"peer": map[string]any{
			"a": map[string]any{
				"public-key":  testPubKey1B64,
				"allowed-ips": []any{"10.0.0.1/32"},
			},
			"b": map[string]any{
				"public-key":  testPubKey2B64,
				"allowed-ips": []any{"10.0.0.2/32"},
			},
		},
	}

	entry, err := parseWireguardEntry("wg0", m)
	require.NoError(t, err)
	require.Len(t, entry.Spec.Peers, 2)

	// Peers come out in map iteration order (non-deterministic) -- verify by name.
	byName := map[string]WireguardPeerSpec{}
	for _, p := range entry.Spec.Peers {
		byName[p.Name] = p
	}
	require.Contains(t, byName, "a")
	require.Contains(t, byName, "b")
	assert.Equal(t, testPubKey1B64, byName["a"].PublicKey.String())
	assert.Equal(t, testPubKey2B64, byName["b"].PublicKey.String())
	assert.Equal(t, []string{"10.0.0.1/32"}, byName["a"].AllowedIPs)
	assert.Equal(t, []string{"10.0.0.2/32"}, byName["b"].AllowedIPs)
}

// TestParseWireguardMissingPrivateKey verifies AC-7: a wireguard entry
// without a private-key leaf is rejected at parse time with a clear error.
//
// VALIDATES: AC-7 — reload rejected when private-key leaf is absent.
// PREVENTS: silently creating a wireguard interface with no private key
// (wgctrl would reject at ConfigureDevice time, but the earlier error is
// friendlier and keeps the parser layer authoritative).
func TestParseWireguardMissingPrivateKey(t *testing.T) {
	m := map[string]any{
		"listen-port": "51820",
		"peer": map[string]any{
			"site2": map[string]any{"public-key": testPubKey1B64},
		},
	}

	_, err := parseWireguardEntry("wg0", m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private-key is required")
}

// TestParseWireguardInvalidKeyLength verifies AC-6: a wireguard entry with
// a private-key whose decoded length is not 32 bytes is rejected.
//
// VALIDATES: AC-6 — decoded key must be exactly 32 bytes.
// PREVENTS: wgctrl panics or silent truncation if a short key reaches it.
func TestParseWireguardInvalidKeyLength(t *testing.T) {
	// Base64 of 16 bytes of 0x01 — decodes to 16 bytes, not 32.
	shortKey := "AQEBAQEBAQEBAQEBAQEBAQ=="

	m := map[string]any{
		"private-key": shortKey,
		"peer": map[string]any{
			"site2": map[string]any{"public-key": testPubKey1B64},
		},
	}

	_, err := parseWireguardEntry("wg0", m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private-key:")
	// wgtypes error says "incorrect key size"
	assert.Contains(t, err.Error(), "key size")
}

// TestParseWireguardBadPublicKey verifies AC-8: a peer public-key that is
// not valid base64 (or decodes to wrong length) is rejected.
//
// VALIDATES: AC-8 — peer public-key must be a valid 44-char base64 string
// decoding to 32 bytes.
// PREVENTS: peer entries entering the reconcile loop with a malformed key.
func TestParseWireguardBadPublicKey(t *testing.T) {
	m := map[string]any{
		"private-key": testPrivKeyB64,
		"peer": map[string]any{
			"site2": map[string]any{"public-key": "not-base-64-at-all!!!"},
		},
	}

	_, err := parseWireguardEntry("wg0", m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "site2")
	assert.Contains(t, err.Error(), "public-key")
}

// TestParseWireguardPersistentKeepalive verifies AC-13: the
// persistent-keepalive numeric leaf round-trips through the parser.
//
// VALIDATES: AC-13 — persistent-keepalive parses as uint16 seconds.
// PREVENTS: silently dropping the keepalive value on parse.
func TestParseWireguardPersistentKeepalive(t *testing.T) {
	m := map[string]any{
		"private-key": testPrivKeyB64,
		"peer": map[string]any{
			"site2": map[string]any{
				"public-key":           testPubKey1B64,
				"persistent-keepalive": "25",
			},
		},
	}

	entry, err := parseWireguardEntry("wg0", m)
	require.NoError(t, err)
	require.Len(t, entry.Spec.Peers, 1)
	assert.Equal(t, uint16(25), entry.Spec.Peers[0].PersistentKeepalive)
}

// TestParseWireguardPresharedKey verifies that an optional preshared-key is
// parsed into WireguardPeerSpec.PresharedKey and HasPresharedKey is set.
//
// VALIDATES: optional preshared key leaf is honored.
// PREVENTS: silently dropping PSK which would leave the peer without the
// extra symmetric handshake layer the operator configured.
func TestParseWireguardPresharedKey(t *testing.T) {
	m := map[string]any{
		"private-key": testPrivKeyB64,
		"peer": map[string]any{
			"site2": map[string]any{
				"public-key":    testPubKey1B64,
				"preshared-key": testPSKB64,
			},
		},
	}

	entry, err := parseWireguardEntry("wg0", m)
	require.NoError(t, err)
	require.Len(t, entry.Spec.Peers, 1)
	peer := entry.Spec.Peers[0]
	assert.True(t, peer.HasPresharedKey)
	assert.Equal(t, testPSKB64, peer.PresharedKey.String())
}

// TestParseWireguardDisableLeaf verifies the disable leaf on both the
// wireguard list entry and the nested peer list.
//
// VALIDATES: AC-16 and AC-17 — disable at either level round-trips.
// PREVENTS: disabled peers silently reaching the reconcile loop.
func TestParseWireguardDisableLeaf(t *testing.T) {
	m := map[string]any{
		"private-key": testPrivKeyB64,
		"disable":     "true",
		"peer": map[string]any{
			"site2": map[string]any{
				"public-key": testPubKey1B64,
				"disable":    "true",
			},
		},
	}

	entry, err := parseWireguardEntry("wg0", m)
	require.NoError(t, err)
	assert.True(t, entry.Disable)
	require.Len(t, entry.Spec.Peers, 1)
	assert.True(t, entry.Spec.Peers[0].Disable)
}

// TestParseWireguardKeyRoundTripThroughParseKey verifies that a key string
// produced by wgtypes.GenerateKey().String() round-trips through the parser
// without mutation. This is a belt-and-suspenders check that the parser
// does not re-encode or normalize key bytes.
//
// VALIDATES: parser is a pure passthrough for key material.
// PREVENTS: subtle base64 normalization bugs.
func TestParseWireguardKeyRoundTripThroughParseKey(t *testing.T) {
	// Generate a fresh key pair to avoid any coincidence with the
	// hardcoded test constants above.
	k, err := wgtypes.GenerateKey()
	require.NoError(t, err)
	encoded := k.String()
	require.True(t, strings.HasSuffix(encoded, "="), "base64 wg key should end with =")

	m := map[string]any{
		"private-key": encoded,
		"peer": map[string]any{
			"site2": map[string]any{"public-key": testPubKey1B64},
		},
	}

	entry, err := parseWireguardEntry("wg0", m)
	require.NoError(t, err)
	assert.Equal(t, encoded, entry.Spec.PrivateKey.String())
	assert.Equal(t, k, entry.Spec.PrivateKey)
}

// TestParseWireguardEndpointIPWithoutPort verifies that an endpoint with
// ip but no port is rejected.
//
// VALIDATES: endpoint requires both ip and port together.
// PREVENTS: creating a peer with UDP port 0.
func TestParseWireguardEndpointIPWithoutPort(t *testing.T) {
	m := map[string]any{
		"private-key": testPrivKeyB64,
		"peer": map[string]any{
			"site2": map[string]any{
				"public-key": testPubKey1B64,
				"endpoint": map[string]any{
					"ip": "203.0.113.1",
				},
			},
		},
	}
	_, err := parseWireguardEntry("wg0", m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint has ip but no port")
}

// TestParseWireguardEndpointPortWithoutIP verifies that an endpoint with
// port but no ip is rejected.
//
// VALIDATES: endpoint requires both ip and port together.
// PREVENTS: silently discarding a port-only endpoint.
func TestParseWireguardEndpointPortWithoutIP(t *testing.T) {
	m := map[string]any{
		"private-key": testPrivKeyB64,
		"peer": map[string]any{
			"site2": map[string]any{
				"public-key": testPubKey1B64,
				"endpoint": map[string]any{
					"port": "51820",
				},
			},
		},
	}
	_, err := parseWireguardEntry("wg0", m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint has port but no ip")
}

// TestParseWireguardDuplicatePublicKey verifies that two peers with the
// same public key are rejected.
//
// VALIDATES: duplicate public-key detection.
// PREVENTS: undefined kernel behavior with duplicate peer keys.
func TestParseWireguardDuplicatePublicKey(t *testing.T) {
	m := map[string]any{
		"private-key": testPrivKeyB64,
		"peer": map[string]any{
			"site1": map[string]any{"public-key": testPubKey1B64},
			"site2": map[string]any{"public-key": testPubKey1B64},
		},
	}
	_, err := parseWireguardEntry("wg0", m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate public-key")
}

// TestParseWireguardMACIgnored verifies that a hand-injected mac-address
// at the wireguard list level is cleared by the parser (defense-in-depth).
//
// VALIDATES: wireguard uses interface-common, not interface-l2.
// PREVENTS: SetMACAddress on a wireguard device.
func TestParseWireguardMACIgnored(t *testing.T) {
	m := map[string]any{
		"private-key": testPrivKeyB64,
		"mac-address": "aa:bb:cc:dd:ee:ff",
		"peer": map[string]any{
			"site1": map[string]any{"public-key": testPubKey1B64},
		},
	}
	entry, err := parseWireguardEntry("wg0", m)
	require.NoError(t, err)
	assert.Empty(t, entry.MACAddress,
		"wireguard must not carry mac-address (cleared by parser)")
}
