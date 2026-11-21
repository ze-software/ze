// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions
// Related: dataplane.go -- SPParams and PolicyInfo, the entries a role names
// RFC: rfc/short/rfc4301.md -- Section 4.4.1, the SPD selection function of a multi-SPD implementation

package dataplane

import (
	"strconv"

	"github.com/ze-software/ze/internal/core/textbuf"
)

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
	// PolicyRoleIKEBypass is the IKE control-plane bypass: the per-socket policy each
	// of Ze's IKE sockets carries (BypassIKESocket, ike_socket_bypass_linux.go), told
	// apart by PriorityIKESocket. On VPP, which reads no socket policy, the
	// bypass is instead SPD entries the backend installs itself (ensureIKEBypass).
	PolicyRoleIKEBypass
	// PolicyRoleChildSA is a negotiated Child SA's policy.
	PolicyRoleChildSA
	// PolicyRoleOperator is an operator `vpn ipsec policy` entry.
	PolicyRoleOperator
	// PolicyRoleOSPF is an OSPFv3 RFC 4552 manual-SA policy, which the OSPF plugin
	// installs through the same dataplane (plugins/ospf/ipsec_install.go). It is Ze's
	// own entry, so it is never left to read as "unknown".
	PolicyRoleOSPF
	// PolicyRoleESPFormSocket is the per-socket policy of Ze's ESP form reader
	// (espFormBypassInboundPolicy), told apart by PriorityESPFormSocket.
	PolicyRoleESPFormSocket
	// PolicyRoleSocketOverride is any other per-socket policy: an application's
	// override of the SPD (RFC 4301 Section 4.4.1).
	PolicyRoleSocketOverride
)

// unknownName is the word a renderer shows for a value Ze holds no name for: a
// role nobody recorded, a guard status nobody set, a treatment outside the set.
// Declared once so every table in the package spells it the same way.
const unknownName = "unknown"

// policyRoleNames is the one declaration of each role's rendered name, indexed by
// the role. The names are kebab-case because they reach JSON.
var policyRoleNames = [...]string{
	PolicyRoleUnspecified:        unknownName,
	PolicyRoleCatchAll:           "catch-all",
	PolicyRoleInterfaceCatchAll:  "interface-catch-all",
	PolicyRoleInterfaceExemption: "interface-exemption",
	PolicyRoleIKEBypass:          "ike-bypass",
	PolicyRoleChildSA:            "child-sa",
	PolicyRoleOperator:           "operator",
	PolicyRoleOSPF:               "ospf",
	PolicyRoleESPFormSocket:      "esp-form-socket",
	PolicyRoleSocketOverride:     "socket-override",
}

// socketPolicyRole names a per-socket policy from its priority, the mark Ze writes
// into each of its own: PriorityESPFormSocket on the ESP form reader, and
// PriorityIKESocket on every IKE socket. A process that copies a mark
// is listed as Ze's; doctor still counts every socket policy, so the copy is not
// hidden there.
func socketPolicyRole(priority int) PolicyRole { //nolint:unused // called only by the Linux backend (xfrm_linux.go policyInfoFromKernel); a non-Linux lint pass sees no caller
	switch priority {
	case PriorityESPFormSocket:
		return PolicyRoleESPFormSocket
	case PriorityIKESocket:
		return PolicyRoleIKEBypass
	}
	return PolicyRoleSocketOverride
}

// catchAllYieldsToICMPErrorCheck reports whether a template-free entry is the inbound
// DISCARD of an SPD catch-all, the node-wide one or an XFRM interface's, which the
// kernel's ICMP reverse check may pass. Such an entry carries XFRM_POLICY_ICMP.
//
// Ze's kernel (K1, gokrazy/kernel/patches/0003-xfrm-state-scope.patch,
// __xfrm_policy_check) admits an ICMP error that arrived on a Child SA with the
// XFRM_STATE_ICMP flag, and whose payload header reversed matches the SA, only past
// the policy the message meets as received: an inbound BLOCK policy matching the
// message's own headers drops it, unless that policy carries XFRM_POLICY_ICMP. So an
// operator's discard entry, which never carries the flag, stops the error, and the
// catch-all, which RFC 4301 Section 4.4.1 makes the entry for what is "otherwise
// unmatched", does not: an error whose payload maps to an SA is matched by that SA.
//
// RFC 4301 Section 6.2: "If the packet fails this check, the IPsec implementation
// MUST NOT forwarded the ICMP message to the destination."
// RFC 4301 Section 6.1.2: "Thus, implementers MUST provide controls to allow local
// administrators to constrain the processing of ICMP error messages received on the
// protected side of the boundary, and directed to the IPsec implementation."
//
// The role decides, never the selector or the priority: an operator may write a
// wildcard discard, and its entry is a control the reverse check must honor.
//
// It is declared here, with no platform tag, because two backends answer from it: the
// Linux backend sets the flag from it (xfrm_linux.go, xfrmPolicyFromParams), and the
// zetest recording backend reports ICMPErrorYield from it, as the kernel's readback
// does (recording_zetest.go, ListPolicies).
func catchAllYieldsToICMPErrorCheck(p SPParams) bool { //nolint:unused // called only by the Linux backend and the zetest recording backend; a non-Linux shipped lint pass sees no caller
	if p.Action != SPActionDiscard {
		return false
	}
	if p.Dir != SADirIn {
		return false
	}
	return p.Role == PolicyRoleCatchAll || p.Role == PolicyRoleInterfaceCatchAll
}

// spdNameSocket names where a per-socket policy sits: on a socket, ahead of every SPD.
const spdNameSocket = "socket"

// String answers the role's rendered name. A value outside the declared set is a Ze
// defect, and is named with its number rather than passed off as a declared role.
func (r PolicyRole) String() string {
	if int(r) < len(policyRoleNames) {
		return policyRoleNames[r]
	}
	return "invalid-role-" + strconv.Itoa(int(r))
}

// spdNameNode names the node-wide SPD, the one at if_id 0.
const spdNameNode = "node"

// SPDName answers which SPD holds the entry: "node" for if_id 0, the XFRM
// interface's name for an if_id Ze bound an interface to, and "if-id <n>" for an
// if_id no bound interface has, so a foreign SPD is still told apart from the
// node-wide one and never shown blank.
//
// RFC 4301 Section 4.4.1: "However, if an implementation supports multiple SPDs, then
// it MUST include an explicit SPD selection function that is invoked to select the
// appropriate SPD for outbound traffic processing." The kernel's if_id is the SPD-ID,
// and this is the name the operator reads it by.
func (p *PolicyInfo) SPDName() string {
	if p.PerSocket {
		return spdNameSocket
	}
	if p.IfID == 0 {
		return spdNameNode
	}
	if p.SPDInterface != "" {
		return p.SPDInterface
	}
	var b textbuf.Buffer
	return b.Str("if-id ").Uint32(p.IfID).String()
}

// nameInterfaceSPDs fills SPDInterface on every entry at an interface if_id that does
// not carry it, from another entry at the same if_id that does. A Child SA policy and
// a foreign entry share their SPD with the interface catch-all, which carries the
// name, so every entry of one SPD reads the same name. An if_id no entry names keeps
// an empty SPDInterface, which SPDName renders as "if-id <n>". Entries are edited in
// place.
func nameInterfaceSPDs(infos []PolicyInfo) {
	names := make(map[uint32]string)
	for i := range infos {
		if infos[i].IfID == 0 {
			continue
		}
		if infos[i].SPDInterface == "" {
			continue
		}
		names[infos[i].IfID] = infos[i].SPDInterface
	}
	for i := range infos {
		if infos[i].IfID == 0 {
			continue
		}
		if infos[i].SPDInterface != "" {
			continue
		}
		if name, ok := names[infos[i].IfID]; ok {
			infos[i].SPDInterface = name
		}
	}
}
