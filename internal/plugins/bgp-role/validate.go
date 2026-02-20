// Design: docs/architecture/core-design.md — BGP role plugin
// Design: rfc/short/rfc9234.md

package bgp_role

import (
	"encoding/hex"
	"fmt"

	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// RFC 9234 Section 4.2, Table 2: Valid local→remote role pairs.
// Values: Provider=0, RS=1, RS-Client=2, Customer=3, Peer=4.
var validRolePairs = map[[2]uint8]bool{
	{0, 3}: true, // Provider ↔ Customer
	{3, 0}: true, // Customer ↔ Provider
	{1, 2}: true, // RS ↔ RS-Client
	{2, 1}: true, // RS-Client ↔ RS
	{4, 4}: true, // Peer ↔ Peer
}

// isValidRolePair checks if a local/remote role pair is valid per RFC 9234 Table 2.
func isValidRolePair(local, remote uint8) bool {
	return validRolePairs[[2]uint8{local, remote}]
}

// extractRolesFromCaps extracts Role capability values (code 9) from a capability list.
// Skips malformed entries (wrong length or invalid hex).
func extractRolesFromCaps(caps []sdk.ValidateOpenCapability) []uint8 {
	var roles []uint8
	for _, c := range caps {
		if c.Code != roleCapCode {
			continue
		}
		data, err := hex.DecodeString(c.Hex)
		if err != nil || len(data) != 1 {
			continue
		}
		roles = append(roles, data[0])
	}
	return roles
}

// validateOpenRolePair validates an OPEN pair for Role compatibility per RFC 9234.
// Returns ValidateOpenOutput with Accept=true if the pair is valid,
// or Accept=false with NOTIFICATION codes 2/11 (Role Mismatch) if invalid.
//
// cfg may be nil if no Role config exists for this peer (always accepts).
func validateOpenRolePair(cfg *peerRoleConfig, input *sdk.ValidateOpenInput) *sdk.ValidateOpenOutput {
	if cfg == nil {
		return &sdk.ValidateOpenOutput{Accept: true}
	}

	// Get our local role value from config.
	localRole, ok := roleNameToValue(cfg.role)
	if !ok {
		// Invalid config role name — should not happen (validated at config parse).
		return &sdk.ValidateOpenOutput{Accept: true}
	}

	// Extract peer role(s) from remote capabilities (code 9).
	peerRoles := extractRolesFromCaps(input.Remote.Capabilities)

	// RFC 9234 Section 4.2: "If multiple BGP Role Capabilities are received
	// and not all of them have the same value, then the BGP speaker MUST reject
	// the connection using the Role Mismatch Notification."
	if len(peerRoles) > 1 {
		first := peerRoles[0]
		for _, v := range peerRoles[1:] {
			if v != first {
				return &sdk.ValidateOpenOutput{
					Accept:        false,
					NotifyCode:    2,
					NotifySubcode: 11,
					Reason:        "peer sent multiple different Role capabilities",
				}
			}
		}
	}

	// RFC 9234 Section 4.2: "If the BGP Role Capability is sent but one is not
	// received from the peer, the BGP Speaker SHOULD ignore the absence."
	// But: strict mode MAY reject.
	if len(peerRoles) == 0 {
		if cfg.strict {
			return &sdk.ValidateOpenOutput{
				Accept:        false,
				NotifyCode:    2,
				NotifySubcode: 11,
				Reason:        "peer did not send Role capability (strict mode)",
			}
		}
		return &sdk.ValidateOpenOutput{Accept: true}
	}

	peerRole := peerRoles[0]

	// RFC 9234 Section 4.2: "If the Roles do not correspond, the BGP speaker
	// MUST reject the connection using the Role Mismatch Notification
	// (code 2, subcode 11)."
	if !isValidRolePair(localRole, peerRole) {
		localName, _ := roleValueToName(localRole)
		peerName, nameOK := roleValueToName(peerRole)
		if !nameOK {
			peerName = fmt.Sprintf("unknown(%d)", peerRole)
		}
		return &sdk.ValidateOpenOutput{
			Accept:        false,
			NotifyCode:    2,
			NotifySubcode: 11,
			Reason:        fmt.Sprintf("role mismatch: local=%s peer=%s", localName, peerName),
		}
	}

	return &sdk.ValidateOpenOutput{Accept: true}
}
