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
