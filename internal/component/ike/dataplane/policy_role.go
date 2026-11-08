// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions
// Related: dataplane.go -- SPParams and PolicyInfo, the entries a role names
// RFC: rfc/short/rfc4301.md -- Section 4.4.1, the SPD selection function of a multi-SPD implementation

package dataplane

import "strconv"

// PolicyRole names why Ze installed one SPD entry. Ze keeps more than one SPD (the
// node-wide one at if_id 0, and one per XFRM interface at its if_id), and an entry's
// selector alone does not say which job it does: an operator `order` can take any
// priority, so the role is recorded where the entry is built, never inferred from it.
//
// The zero value is PolicyRoleUnspecified, which a renderer shows as "unknown": an
// entry nobody recorded a role for, or one Ze did not install at all.
type PolicyRole uint8

const (
	// PolicyRoleUnspecified is the zero value: no role was recorded.
	PolicyRoleUnspecified PolicyRole = iota
	// PolicyRoleCatchAll is the final entry of the node-wide SPD at if_id 0
	// (engine/unmatched.go), whose disposition the `unmatched` leaf decides.
	PolicyRoleCatchAll
	// PolicyRoleInterfaceCatchAll is the final DISCARD entry of one XFRM interface's
	// SPD, at that interface's if_id (engine/unmatched_xfrm_interface.go).
	PolicyRoleInterfaceCatchAll
	// PolicyRoleInterfaceExemption is an if_id 0 BYPASS entry that hands traffic
	// routed into a bound XFRM interface on to that interface's SPD: the SPD
	// selection function (engine/unmatched_xfrm_interface.go).
	PolicyRoleInterfaceExemption
	// PolicyRoleIKEBypass is the IKE control-plane bypass (engine/bypass.go).
	PolicyRoleIKEBypass
	// PolicyRoleChildSA is a negotiated Child SA's policy.
	PolicyRoleChildSA
	// PolicyRoleOperator is an operator `vpn ipsec policy` entry.
	PolicyRoleOperator
)

// policyRoleNames is the one declaration of each role's rendered name, indexed by
// the role. The names are kebab-case because they reach JSON.
var policyRoleNames = [...]string{
	PolicyRoleUnspecified:        "unknown",
	PolicyRoleCatchAll:           "catch-all",
	PolicyRoleInterfaceCatchAll:  "interface-catch-all",
	PolicyRoleInterfaceExemption: "interface-exemption",
	PolicyRoleIKEBypass:          "ike-bypass",
	PolicyRoleChildSA:            "child-sa",
	PolicyRoleOperator:           "operator",
}

// String answers the role's rendered name. A value outside the declared set is a Ze
// defect, and is named with its number rather than passed off as a declared role.
func (r PolicyRole) String() string {
	if int(r) < len(policyRoleNames) {
		return policyRoleNames[r]
	}
	return "invalid-role-" + strconv.Itoa(int(r))
}
