package role

import (
	"testing"

	"github.com/stretchr/testify/assert"

	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// roleCap creates a ValidateOpenCapability for Role (code 9) with given hex value.
func roleCap(hex string) sdk.ValidateOpenCapability {
	return sdk.ValidateOpenCapability{Code: roleCapCode, Hex: hex}
}

// TestValidateOpenRolePair_ValidPairs verifies RFC 9234 Table 2 valid role pairs.
//
// VALIDATES: Customer↔Provider, RS↔RS-Client, Peer↔Peer are accepted.
// PREVENTS: Valid role pairs being rejected, breaking legitimate sessions.
func TestValidateOpenRolePair_ValidPairs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		localRole string
		localHex  string
		remoteHex string
	}{
		{"customer_provider", "customer", "03", "00"},
		{"provider_customer", "provider", "00", "03"},
		{"rs_rsclient", "rs", "01", "02"},
		{"rsclient_rs", "rs-client", "02", "01"},
		{"peer_peer", "peer", "04", "04"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &peerRoleConfig{role: tt.localRole}
			input := &sdk.ValidateOpenInput{
				Peer: "10.0.0.1",
				Local: sdk.ValidateOpenMessage{
					Capabilities: []sdk.ValidateOpenCapability{roleCap(tt.localHex)},
				},
				Remote: sdk.ValidateOpenMessage{
					Capabilities: []sdk.ValidateOpenCapability{roleCap(tt.remoteHex)},
				},
			}

			output := validateOpenRolePair(cfg, input)
			assert.True(t, output.Accept, "valid pair %s should be accepted", tt.name)
		})
	}
}

// TestValidateOpenRolePair_InvalidPairs verifies invalid pairs are rejected with NOTIFICATION 2/11.
//
// VALIDATES: Customer↔Customer, Provider↔Provider rejected with correct codes.
// PREVENTS: Invalid pairs being accepted, allowing route leaks per RFC 9234.
func TestValidateOpenRolePair_InvalidPairs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		localRole string
		localHex  string
		remoteHex string
	}{
		{"customer_customer", "customer", "03", "03"},
		{"provider_provider", "provider", "00", "00"},
		{"rs_rs", "rs", "01", "01"},
		{"rsclient_rsclient", "rs-client", "02", "02"},
		{"provider_rs", "provider", "00", "01"},
		{"customer_peer", "customer", "03", "04"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &peerRoleConfig{role: tt.localRole}
			input := &sdk.ValidateOpenInput{
				Peer: "10.0.0.1",
				Local: sdk.ValidateOpenMessage{
					Capabilities: []sdk.ValidateOpenCapability{roleCap(tt.localHex)},
				},
				Remote: sdk.ValidateOpenMessage{
					Capabilities: []sdk.ValidateOpenCapability{roleCap(tt.remoteHex)},
				},
			}

			output := validateOpenRolePair(cfg, input)
			assert.False(t, output.Accept, "invalid pair %s should be rejected", tt.name)
			assert.Equal(t, uint8(2), output.NotifyCode, "NOTIFICATION code must be 2 (OPEN)")
			assert.Equal(t, uint8(11), output.NotifySubcode, "NOTIFICATION subcode must be 11 (Role Mismatch)")
			assert.NotEmpty(t, output.Reason, "reason should describe the mismatch")
		})
	}
}

// TestValidateOpenRolePair_NoPeerRole_NoStrict verifies accept when peer has no Role cap (non-strict).
//
// VALIDATES: Without strict mode, missing peer Role is accepted per RFC 9234 Section 4.2.
// PREVENTS: Sessions being rejected when strict mode is not configured.
func TestValidateOpenRolePair_NoPeerRole_NoStrict(t *testing.T) {
	t.Parallel()

	cfg := &peerRoleConfig{role: "customer", strict: false}
	input := &sdk.ValidateOpenInput{
		Peer: "10.0.0.1",
		Local: sdk.ValidateOpenMessage{
			Capabilities: []sdk.ValidateOpenCapability{roleCap("03")},
		},
		Remote: sdk.ValidateOpenMessage{
			// No Role capability — only a Multiprotocol one
			Capabilities: []sdk.ValidateOpenCapability{
				{Code: 1, Hex: "00010001"},
			},
		},
	}

	output := validateOpenRolePair(cfg, input)
	assert.True(t, output.Accept, "should accept when peer has no Role and strict is false")
}

// TestValidateOpenRolePair_NoPeerRole_Strict verifies reject when peer has no Role cap (strict).
//
// VALIDATES: With strict mode, missing peer Role is rejected with NOTIFICATION 2/11.
// PREVENTS: Sessions proceeding without Role when strict mode requires it.
func TestValidateOpenRolePair_NoPeerRole_Strict(t *testing.T) {
	t.Parallel()

	cfg := &peerRoleConfig{role: "customer", strict: true}
	input := &sdk.ValidateOpenInput{
		Peer: "10.0.0.1",
		Local: sdk.ValidateOpenMessage{
			Capabilities: []sdk.ValidateOpenCapability{roleCap("03")},
		},
		Remote: sdk.ValidateOpenMessage{
			Capabilities: []sdk.ValidateOpenCapability{},
		},
	}

	output := validateOpenRolePair(cfg, input)
	assert.False(t, output.Accept, "should reject when peer has no Role and strict is true")
	assert.Equal(t, uint8(2), output.NotifyCode)
	assert.Equal(t, uint8(11), output.NotifySubcode)
	assert.Contains(t, output.Reason, "strict")
}

// TestValidateOpenRolePair_MultipleDifferentRoles verifies reject for conflicting Role caps.
//
// VALIDATES: RFC 9234 Section 4.2 requires rejection when multiple different Roles are received.
// PREVENTS: Ambiguous role assignment when peer sends conflicting capabilities.
func TestValidateOpenRolePair_MultipleDifferentRoles(t *testing.T) {
	t.Parallel()

	cfg := &peerRoleConfig{role: "customer"}
	input := &sdk.ValidateOpenInput{
		Peer: "10.0.0.1",
		Local: sdk.ValidateOpenMessage{
			Capabilities: []sdk.ValidateOpenCapability{roleCap("03")},
		},
		Remote: sdk.ValidateOpenMessage{
			Capabilities: []sdk.ValidateOpenCapability{
				roleCap("00"), // Provider
				roleCap("03"), // Customer — different!
			},
		},
	}

	output := validateOpenRolePair(cfg, input)
	assert.False(t, output.Accept, "should reject when peer sends multiple different Roles")
	assert.Equal(t, uint8(2), output.NotifyCode)
	assert.Equal(t, uint8(11), output.NotifySubcode)
	assert.Contains(t, output.Reason, "multiple")
}

// TestValidateOpenRolePair_MultipleSameRoles verifies accept for duplicate same-value Role caps.
//
// VALIDATES: RFC 9234 Section 4.2 only rejects when values differ, not when duplicated.
// PREVENTS: False rejection when peer happens to send the same Role twice.
func TestValidateOpenRolePair_MultipleSameRoles(t *testing.T) {
	t.Parallel()

	cfg := &peerRoleConfig{role: "customer"}
	input := &sdk.ValidateOpenInput{
		Peer: "10.0.0.1",
		Local: sdk.ValidateOpenMessage{
			Capabilities: []sdk.ValidateOpenCapability{roleCap("03")},
		},
		Remote: sdk.ValidateOpenMessage{
			Capabilities: []sdk.ValidateOpenCapability{
				roleCap("00"), // Provider
				roleCap("00"), // Provider again — same value
			},
		},
	}

	output := validateOpenRolePair(cfg, input)
	assert.True(t, output.Accept, "should accept when peer sends same Role multiple times")
}

// TestValidateOpenRolePair_NoLocalConfig verifies accept when no config for this peer.
//
// VALIDATES: When no Role config exists for a peer, validation accepts (no opinion).
// PREVENTS: Peers being rejected when no role policy was configured.
func TestValidateOpenRolePair_NoLocalConfig(t *testing.T) {
	t.Parallel()

	input := &sdk.ValidateOpenInput{
		Peer:  "10.0.0.1",
		Local: sdk.ValidateOpenMessage{},
		Remote: sdk.ValidateOpenMessage{
			Capabilities: []sdk.ValidateOpenCapability{roleCap("00")},
		},
	}

	// nil config = no role configured for this peer
	output := validateOpenRolePair(nil, input)
	assert.True(t, output.Accept, "should accept when no Role config for peer")
}

// --- Loose mode (non-strict) comprehensive tests ---

// TestLooseMode_AllRolesAcceptMissingPeerRole verifies loose mode for all 5 local roles.
//
// VALIDATES: Every local role accepts sessions when peer has no Role capability (loose mode).
// PREVENTS: Loose mode silently failing for specific role values.
func TestLooseMode_AllRolesAcceptMissingPeerRole(t *testing.T) {
	t.Parallel()

	for _, localRole := range []string{roleProvider, roleCustomer, rolePeer, roleRS, roleRSClient} {
		t.Run(localRole, func(t *testing.T) {
			t.Parallel()
			cfg := &peerRoleConfig{role: localRole, strict: false}
			input := &sdk.ValidateOpenInput{
				Peer:   "10.0.0.1",
				Remote: sdk.ValidateOpenMessage{Capabilities: []sdk.ValidateOpenCapability{}},
			}
			output := validateOpenRolePair(cfg, input)
			assert.True(t, output.Accept, "loose mode: %s should accept missing peer role", localRole)
		})
	}
}

// TestLooseMode_PeerSendsUnknownRoleValue verifies loose mode accepts unknown role values.
//
// VALIDATES: Unknown role value (e.g., 5, 255) is treated as invalid pair but not as crash.
// PREVENTS: Panic or undefined behavior on unknown role values from peers.
func TestLooseMode_PeerSendsUnknownRoleValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		remoteHex string
	}{
		{"value_5", "05"},
		{"value_127", "7F"},
		{"value_255", "FF"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &peerRoleConfig{role: roleProvider, strict: false}
			input := &sdk.ValidateOpenInput{
				Peer: "10.0.0.1",
				Remote: sdk.ValidateOpenMessage{
					Capabilities: []sdk.ValidateOpenCapability{roleCap(tt.remoteHex)},
				},
			}
			output := validateOpenRolePair(cfg, input)
			// Unknown values are not in validRolePairs, so pair is invalid -> reject.
			assert.False(t, output.Accept, "unknown role value should be rejected as invalid pair")
			assert.Equal(t, uint8(2), output.NotifyCode)
			assert.Equal(t, uint8(11), output.NotifySubcode)
		})
	}
}

// TestLooseMode_PeerSendsMalformedRoleCap verifies malformed role capability is handled.
//
// VALIDATES: Malformed capability (wrong length, bad hex) is skipped gracefully.
// PREVENTS: Crash on malformed capability data from peers.
func TestLooseMode_PeerSendsMalformedRoleCap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		caps []sdk.ValidateOpenCapability
	}{
		// Empty hex -> len(data)==0 -> skipped by extractRolesFromCaps
		{"empty_hex", []sdk.ValidateOpenCapability{{Code: roleCapCode, Hex: ""}}},
		// Two bytes -> len(data)!=1 -> skipped
		{"two_bytes", []sdk.ValidateOpenCapability{{Code: roleCapCode, Hex: "0003"}}},
		// Invalid hex string -> DecodeString fails -> skipped
		{"invalid_hex", []sdk.ValidateOpenCapability{{Code: roleCapCode, Hex: "ZZ"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &peerRoleConfig{role: roleProvider, strict: false}
			input := &sdk.ValidateOpenInput{
				Peer:   "10.0.0.1",
				Remote: sdk.ValidateOpenMessage{Capabilities: tt.caps},
			}
			output := validateOpenRolePair(cfg, input)
			// Malformed caps are skipped -> treated as no role -> loose mode accepts.
			assert.True(t, output.Accept, "malformed cap in loose mode should accept (treated as no role)")
		})
	}
}

// --- Strict mode comprehensive tests ---

// TestStrictMode_AllRolesRejectMissingPeerRole verifies strict mode for all 5 local roles.
//
// VALIDATES: Every local role rejects sessions when peer has no Role capability (strict mode).
// PREVENTS: Strict mode silently passing for specific role values.
func TestStrictMode_AllRolesRejectMissingPeerRole(t *testing.T) {
	t.Parallel()

	for _, localRole := range []string{roleProvider, roleCustomer, rolePeer, roleRS, roleRSClient} {
		t.Run(localRole, func(t *testing.T) {
			t.Parallel()
			cfg := &peerRoleConfig{role: localRole, strict: true}
			input := &sdk.ValidateOpenInput{
				Peer:   "10.0.0.1",
				Remote: sdk.ValidateOpenMessage{Capabilities: []sdk.ValidateOpenCapability{}},
			}
			output := validateOpenRolePair(cfg, input)
			assert.False(t, output.Accept, "strict mode: %s should reject missing peer role", localRole)
			assert.Equal(t, uint8(2), output.NotifyCode)
			assert.Equal(t, uint8(11), output.NotifySubcode)
			assert.Contains(t, output.Reason, "strict")
		})
	}
}

// TestStrictMode_ValidPairAccepted verifies strict mode still accepts valid pairs.
//
// VALIDATES: Strict mode only rejects missing role, not valid pairs.
// PREVENTS: Strict mode being overly restrictive and rejecting correct sessions.
func TestStrictMode_ValidPairAccepted(t *testing.T) {
	t.Parallel()

	cfg := &peerRoleConfig{role: roleProvider, strict: true}
	input := &sdk.ValidateOpenInput{
		Peer: "10.0.0.1",
		Remote: sdk.ValidateOpenMessage{
			Capabilities: []sdk.ValidateOpenCapability{roleCap("03")}, // Customer
		},
	}
	output := validateOpenRolePair(cfg, input)
	assert.True(t, output.Accept, "strict mode: valid pair should still be accepted")
}

// TestStrictMode_MalformedCapRejectsAsNoRole verifies strict rejects malformed cap.
//
// VALIDATES: Malformed capability treated as absent -> strict rejects.
// PREVENTS: Malformed capability bypassing strict enforcement.
func TestStrictMode_MalformedCapRejectsAsNoRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		caps []sdk.ValidateOpenCapability
	}{
		{"empty_hex", []sdk.ValidateOpenCapability{{Code: roleCapCode, Hex: ""}}},
		{"two_bytes", []sdk.ValidateOpenCapability{{Code: roleCapCode, Hex: "0003"}}},
		{"invalid_hex", []sdk.ValidateOpenCapability{{Code: roleCapCode, Hex: "ZZ"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &peerRoleConfig{role: roleProvider, strict: true}
			input := &sdk.ValidateOpenInput{
				Peer:   "10.0.0.1",
				Remote: sdk.ValidateOpenMessage{Capabilities: tt.caps},
			}
			output := validateOpenRolePair(cfg, input)
			assert.False(t, output.Accept, "strict: malformed cap should be treated as absent -> reject")
			assert.Contains(t, output.Reason, "strict")
		})
	}
}

// --- Complete role pair matrix ---

// TestValidateOpenRolePair_FullMatrix tests every possible local x remote combination.
//
// VALIDATES: RFC 9234 Table 2 is fully covered (5x5=25 combinations).
// PREVENTS: Any pair being misclassified as valid or invalid.
func TestValidateOpenRolePair_FullMatrix(t *testing.T) {
	t.Parallel()

	// Valid pairs from RFC 9234 Table 2.
	validPairs := map[[2]string]bool{
		{"provider", "customer"}: true,
		{"customer", "provider"}: true,
		{"rs", "rs-client"}:      true,
		{"rs-client", "rs"}:      true,
		{"peer", "peer"}:         true,
	}

	allRoles := []string{roleProvider, roleCustomer, rolePeer, roleRS, roleRSClient}

	for _, local := range allRoles {
		for _, remote := range allRoles {
			remoteVal, _ := roleNameToValue(remote)
			name := local + "_" + remote

			t.Run(name, func(t *testing.T) {
				t.Parallel()
				cfg := &peerRoleConfig{role: local}
				input := &sdk.ValidateOpenInput{
					Peer: "10.0.0.1",
					Remote: sdk.ValidateOpenMessage{
						Capabilities: []sdk.ValidateOpenCapability{
							{Code: roleCapCode, Hex: hexByte(remoteVal)},
						},
					},
				}
				output := validateOpenRolePair(cfg, input)
				shouldAccept := validPairs[[2]string{local, remote}]

				if shouldAccept {
					assert.True(t, output.Accept, "%s<->%s should be valid", local, remote)
				} else {
					assert.False(t, output.Accept, "%s<->%s should be invalid", local, remote)
					assert.Equal(t, uint8(2), output.NotifyCode)
					assert.Equal(t, uint8(11), output.NotifySubcode)
				}
			})
		}
	}
}

// hexByte converts a byte to a 2-char hex string.
func hexByte(b uint8) string {
	const hex = "0123456789abcdef"
	return string([]byte{hex[b>>4], hex[b&0xf]})
}
