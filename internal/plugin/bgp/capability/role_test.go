package capability

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateRolePair_ValidPairs verifies all valid pairs per RFC 9234 Table 2.
//
// VALIDATES: All 5 valid local↔peer role pairs are accepted.
// PREVENTS: False rejection of valid BGP sessions.
func TestValidateRolePair_ValidPairs(t *testing.T) {
	pairs := [][2]uint8{
		{RoleProvider, RoleCustomer},
		{RoleCustomer, RoleProvider},
		{RoleRS, RoleRSClient},
		{RoleRSClient, RoleRS},
		{RolePeer, RolePeer},
	}
	for _, p := range pairs {
		assert.True(t, ValidateRolePair(p[0], p[1]),
			"pair (%s, %s) should be valid", RoleName(p[0]), RoleName(p[1]))
	}
}

// TestValidateRolePair_InvalidPairs verifies all 20 invalid pairs rejected.
//
// VALIDATES: Invalid role combinations are rejected per RFC 9234 Table 2.
// PREVENTS: Accepting sessions with mismatched roles (route leak risk).
func TestValidateRolePair_InvalidPairs(t *testing.T) {
	valid := map[[2]uint8]bool{
		{RoleProvider, RoleCustomer}: true,
		{RoleCustomer, RoleProvider}: true,
		{RoleRS, RoleRSClient}:       true,
		{RoleRSClient, RoleRS}:       true,
		{RolePeer, RolePeer}:         true,
	}
	for local := uint8(0); local <= RoleMaxValid; local++ {
		for peer := uint8(0); peer <= RoleMaxValid; peer++ {
			if valid[[2]uint8{local, peer}] {
				continue
			}
			assert.False(t, ValidateRolePair(local, peer),
				"pair (%s, %s) should be invalid", RoleName(local), RoleName(peer))
		}
	}
}

// TestValidateRolePair_Boundary verifies boundary values.
//
// BOUNDARY: 4 (last valid), 5 (first invalid above).
func TestValidateRolePair_Boundary(t *testing.T) {
	// 4 is max valid value
	assert.True(t, ValidateRolePair(RolePeer, RolePeer))
	// 5 is first invalid
	assert.False(t, ValidateRolePair(5, 0))
	assert.False(t, ValidateRolePair(0, 5))
	assert.False(t, ValidateRolePair(255, 255))
}

// TestRoleName verifies role value to name mapping.
func TestRoleName(t *testing.T) {
	assert.Equal(t, "provider", RoleName(0))
	assert.Equal(t, "rs", RoleName(1))
	assert.Equal(t, "rs-client", RoleName(2))
	assert.Equal(t, "customer", RoleName(3))
	assert.Equal(t, "peer", RoleName(4))
	assert.Equal(t, "", RoleName(5))
	assert.Equal(t, "", RoleName(255))
}

// makeRoleCap creates a Role capability with the given role value.
func makeRoleCap(value uint8) *Role {
	return &Role{role: value}
}

// TestValidateRole_ValidPair verifies full validation accepts valid pairs.
//
// VALIDATES: ValidateRole returns nil for valid local/peer role combinations.
// PREVENTS: False rejection during OPEN processing.
func TestValidateRole_ValidPair(t *testing.T) {
	local := []Capability{makeRoleCap(RoleCustomer)}
	peer := []Capability{makeRoleCap(RoleProvider)}

	err := ValidateRole(local, peer, false)
	require.NoError(t, err)
}

// TestValidateRole_InvalidPair verifies validation rejects mismatched pairs.
//
// VALIDATES: ValidateRole returns ErrRoleMismatch for invalid combinations.
// PREVENTS: Accepting sessions that would enable route leaks.
func TestValidateRole_InvalidPair(t *testing.T) {
	local := []Capability{makeRoleCap(RoleCustomer)}
	peer := []Capability{makeRoleCap(RoleCustomer)} // Customer↔Customer = invalid

	err := ValidateRole(local, peer, false)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRoleMismatch))
	assert.Contains(t, err.Error(), "customer")
}

// TestValidateRole_NoLocalRole verifies no validation when we didn't send Role.
//
// VALIDATES: ValidateRole skips validation when local didn't advertise Role.
// PREVENTS: Rejecting peers when we don't participate in Role negotiation.
func TestValidateRole_NoLocalRole(t *testing.T) {
	local := []Capability{} // No Role capability
	peer := []Capability{makeRoleCap(RoleProvider)}

	err := ValidateRole(local, peer, false)
	require.NoError(t, err)
}

// TestValidateRole_NoPeerRole_NoStrict verifies default behavior when peer has no Role.
//
// RFC 9234 Section 4.2: SHOULD ignore absence of Role from peer.
func TestValidateRole_NoPeerRole_NoStrict(t *testing.T) {
	local := []Capability{makeRoleCap(RoleCustomer)}
	peer := []Capability{} // No Role capability

	err := ValidateRole(local, peer, false)
	require.NoError(t, err)
}

// TestValidateRole_NoPeerRole_Strict verifies strict mode rejects missing Role.
//
// RFC 9234 Section 4.2: Operator MAY apply strict mode.
func TestValidateRole_NoPeerRole_Strict(t *testing.T) {
	local := []Capability{makeRoleCap(RoleCustomer)}
	peer := []Capability{} // No Role capability

	err := ValidateRole(local, peer, true)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRoleMismatch))
	assert.Contains(t, err.Error(), "strict")
}

// TestValidateRole_MultipleDifferentPeerRoles verifies rejection of multiple different Role caps.
//
// RFC 9234 Section 4.2: MUST reject if multiple Role caps have different values.
func TestValidateRole_MultipleDifferentPeerRoles(t *testing.T) {
	local := []Capability{makeRoleCap(RoleCustomer)}
	peer := []Capability{
		makeRoleCap(RoleProvider),
		makeRoleCap(RoleRS), // Different value!
	}

	err := ValidateRole(local, peer, false)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRoleMismatch))
	assert.Contains(t, err.Error(), "multiple")
}

// TestValidateRole_MultipleSamePeerRoles verifies identical Role caps treated as one.
//
// RFC 9234 Section 4.2: multiple Role caps with same value treated as one.
func TestValidateRole_MultipleSamePeerRoles(t *testing.T) {
	local := []Capability{makeRoleCap(RoleCustomer)}
	peer := []Capability{
		makeRoleCap(RoleProvider),
		makeRoleCap(RoleProvider), // Same value = OK
	}

	err := ValidateRole(local, peer, false)
	require.NoError(t, err)
}

// TestValidateRole_AllValidPairs verifies all 5 valid pairs through full validation.
func TestValidateRole_AllValidPairs(t *testing.T) {
	pairs := [][2]uint8{
		{RoleProvider, RoleCustomer},
		{RoleCustomer, RoleProvider},
		{RoleRS, RoleRSClient},
		{RoleRSClient, RoleRS},
		{RolePeer, RolePeer},
	}
	for _, p := range pairs {
		t.Run(RoleName(p[0])+"_"+RoleName(p[1]), func(t *testing.T) {
			local := []Capability{makeRoleCap(p[0])}
			peer := []Capability{makeRoleCap(p[1])}
			require.NoError(t, ValidateRole(local, peer, false))
		})
	}
}

// TestValidateRole_ParsedFromWire verifies ValidateRole works with capabilities
// parsed from real OPEN optional parameters (not hand-crafted *Plugin).
//
// VALIDATES: The full integration path: wire bytes → ParseFromOptionalParams → ValidateRole.
// PREVENTS: Bug where parseCapability produces *Unknown for CodeRole, causing
// ValidateRole's *Plugin type assertion to silently fail.
func TestValidateRole_ParsedFromWire(t *testing.T) {
	// Build OPEN optional params containing Role capability (code 9, len 1, value).
	// Format: [param_type=2, param_len, cap_code=9, cap_len=1, role_value]
	makeOptParams := func(role uint8) []byte {
		return []byte{
			2,    // Optional parameter type: Capability
			3,    // Parameter length: 3 bytes (code + len + value)
			0x09, // Capability code: Role (9)
			0x01, // Capability length: 1
			role, // Role value
		}
	}

	t.Run("valid_pair", func(t *testing.T) {
		localCaps := ParseFromOptionalParams(makeOptParams(RoleCustomer))
		peerCaps := ParseFromOptionalParams(makeOptParams(RoleProvider))

		// This would fail before the CodeRole fix: Role caps parsed as *Unknown,
		// ValidateRole's *Plugin assertion would silently skip them, returning nil
		// even for invalid pairs.
		err := ValidateRole(localCaps, peerCaps, false)
		require.NoError(t, err)
	})

	t.Run("invalid_pair_detected", func(t *testing.T) {
		localCaps := ParseFromOptionalParams(makeOptParams(RoleCustomer))
		peerCaps := ParseFromOptionalParams(makeOptParams(RoleCustomer)) // Customer↔Customer = invalid

		err := ValidateRole(localCaps, peerCaps, false)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrRoleMismatch))
	})

	t.Run("strict_mode_no_peer_role", func(t *testing.T) {
		localCaps := ParseFromOptionalParams(makeOptParams(RoleCustomer))
		peerCaps := ParseFromOptionalParams([]byte{}) // No capabilities

		err := ValidateRole(localCaps, peerCaps, true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "strict")
	})
}

// TestParseRole_LengthValidation verifies RFC 9234 length enforcement.
//
// VALIDATES: parseRole rejects capabilities with length != 1.
// PREVENTS: Accepting malformed Role capabilities from peers.
// BOUNDARY: 1 (valid), 0 (invalid below), 2 (invalid above).
func TestParseRole_LengthValidation(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
		wantVal uint8
	}{
		{"valid_length_1", []byte{RoleCustomer}, false, RoleCustomer},
		{"invalid_length_0", []byte{}, true, 0},
		{"invalid_length_2", []byte{0x03, 0x00}, true, 0},
		{"invalid_length_3", []byte{0x03, 0x00, 0x01}, true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cap, err := parseRole(tt.data)
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, cap)
				return
			}
			require.NoError(t, err)
			r, ok := cap.(*Role)
			require.True(t, ok, "parsed capability should be *Role")
			assert.Equal(t, tt.wantVal, r.Value())
		})
	}
}

// TestRoleType verifies the Role type implements Capability correctly.
//
// VALIDATES: Role.Code(), Role.Len(), Role.Value(), Role.WriteTo() all work.
// PREVENTS: Broken Capability interface implementation.
func TestRoleType(t *testing.T) {
	r := &Role{role: RolePeer}

	assert.Equal(t, CodeRole, r.Code())
	assert.Equal(t, 3, r.Len()) // TLV: code(1) + length(1) + value(1)
	assert.Equal(t, RolePeer, r.Value())

	// WriteTo produces correct TLV bytes
	buf := make([]byte, 10)
	n := r.WriteTo(buf, 0)
	assert.Equal(t, 3, n)
	assert.Equal(t, byte(CodeRole), buf[0]) // Code
	assert.Equal(t, byte(1), buf[1])        // Length
	assert.Equal(t, RolePeer, buf[2])       // Value
}
