package capability

import "fmt"

// RFC 9234 Section 4.1, Table 1: BGP Role values.
const (
	RoleProvider  uint8 = 0
	RoleRS        uint8 = 1
	RoleRSClient  uint8 = 2
	RoleCustomer  uint8 = 3
	RolePeer      uint8 = 4
	RoleMaxValid  uint8 = 4
	RoleUndefined uint8 = 255 // Sentinel: no Role capability present
)

// roleNames maps wire values to human-readable names.
var roleNames = [5]string{"provider", "rs", "rs-client", "customer", "peer"}

// RoleName returns the human-readable name for a role value.
// Returns "" for unknown values.
func RoleName(value uint8) string {
	if value > RoleMaxValid {
		return ""
	}
	return roleNames[value]
}

// Role represents the BGP Role capability (RFC 9234, code 9).
// RFC 9234 Section 4.1: "The length of the value field of the BGP Role
// Capability is 1.".
type Role struct {
	role uint8 // Wire value: 0=Provider, 1=RS, 2=RS-Client, 3=Customer, 4=Peer
}

// parseRole parses a Role capability from wire bytes.
// RFC 9234 Section 4.1: capability length MUST be 1.
func parseRole(data []byte) (Capability, error) {
	if len(data) != 1 {
		return nil, fmt.Errorf("RFC 9234: Role capability length must be 1, got %d", len(data))
	}
	return &Role{role: data[0]}, nil
}

// Code returns the capability type code (9).
func (r *Role) Code() Code { return CodeRole }

// Len returns the TLV-encoded length (code + length + 1 byte value = 3).
func (r *Role) Len() int { return 3 }

// WriteTo writes the Role capability TLV into buf at offset.
func (r *Role) WriteTo(buf []byte, off int) int {
	writeCapabilityTo(buf, off, CodeRole, 1)
	buf[off+2] = r.role
	return r.Len()
}

// Value returns the raw role wire value.
func (r *Role) Value() uint8 { return r.role }

// RFC 9234 Section 4.2, Table 2: Valid local→peer role pairs.
// Indexed as validRolePairs[local][peer].
var validRolePairs [5][5]bool

func init() {
	validRolePairs[RoleProvider][RoleCustomer] = true
	validRolePairs[RoleCustomer][RoleProvider] = true
	validRolePairs[RoleRS][RoleRSClient] = true
	validRolePairs[RoleRSClient][RoleRS] = true
	validRolePairs[RolePeer][RolePeer] = true
}

// ValidateRolePair checks if a local/peer role pair is valid per RFC 9234 Table 2.
// Returns true if the pair is valid.
func ValidateRolePair(local, peer uint8) bool {
	if local > RoleMaxValid || peer > RoleMaxValid {
		return false
	}
	return validRolePairs[local][peer]
}

// ErrRoleMismatch is returned when RFC 9234 role validation fails.
var ErrRoleMismatch = fmt.Errorf("RFC 9234: role mismatch")

// ValidateRole performs RFC 9234 Section 4.2 role validation.
//
// It extracts role capabilities from local and peer capability lists,
// validates the pair per Table 2, and enforces strict mode.
//
// Returns nil if validation passes, or an error describing the failure.
// The caller should send NOTIFICATION 2/11 (Role Mismatch) on error.
func ValidateRole(local, peer []Capability, strict bool) error {
	localRole := RoleUndefined
	var peerRoles []uint8

	// Extract local Role capability (what we sent).
	for _, c := range local {
		if r, ok := c.(*Role); ok {
			localRole = r.role
			break
		}
	}

	// If we didn't send Role, nothing to validate.
	// RFC 9234 Section 4.2: Validation only applies when Role is advertised.
	if localRole == RoleUndefined {
		return nil
	}

	// Extract peer Role capabilities.
	for _, c := range peer {
		if r, ok := c.(*Role); ok {
			peerRoles = append(peerRoles, r.role)
		}
	}

	// RFC 9234 Section 4.2: "If multiple BGP Role Capabilities are received
	// and not all of them have the same value, then the BGP speaker MUST reject
	// the connection using the Role Mismatch Notification."
	if len(peerRoles) > 1 {
		first := peerRoles[0]
		for _, v := range peerRoles[1:] {
			if v != first {
				return fmt.Errorf("%w: peer sent multiple different Role values", ErrRoleMismatch)
			}
		}
	}

	// RFC 9234 Section 4.2: "If the BGP Role Capability is sent but one is not
	// received from the peer, the BGP Speaker SHOULD ignore the absence."
	// But: strict mode MAY reject.
	if len(peerRoles) == 0 {
		if strict {
			return fmt.Errorf("%w: peer did not send Role capability (strict mode)", ErrRoleMismatch)
		}
		return nil
	}

	peerRole := peerRoles[0]

	// RFC 9234 Section 4.2: "If the Roles do not correspond, the BGP speaker
	// MUST reject the connection using the Role Mismatch Notification (code 2, subcode 11)."
	if !ValidateRolePair(localRole, peerRole) {
		return fmt.Errorf("%w: local=%s peer=%s",
			ErrRoleMismatch, RoleName(localRole), RoleName(peerRole))
	}

	return nil
}
