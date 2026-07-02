// Design: docs/architecture/wire/ospf.md -- OSPF Segment Routing shared control
// plane. The NP/E/M forwarding truth table that decides push / swap / PHP /
// Explicit-NULL for a Prefix-SID, parameterised by the address family's Explicit
// NULL label. Shared by both address families (IPv4 NULL 0, IPv6 NULL 2).
// RFC: rfc/short/rfc8665.md (§5 outgoing-label E/NP/M rules); rfc/short/rfc8666.md (§6)

package sr

// Explicit NULL MPLS labels (RFC 3032 / RFC 4182). Used when the Prefix-SID
// E-Flag is set: the upstream neighbor replaces the label with the family's
// Explicit NULL before forwarding (RFC 8665 §5 IPv4 label 0; RFC 8666 §6 IPv6
// label 2).
const (
	ExplicitNullV4 uint32 = 0
	ExplicitNullV6 uint32 = 2
)

// SIDFlags carries the NP/M/E/V/L flags advertised with a Prefix-SID (RFC 8665
// §5 / RFC 8666 §6). Only NP, M and E drive the outgoing-label decision; V and L
// govern the SID/Index/Label wire width and are validated by the codec.
type SIDFlags struct {
	NP bool // No-PHP: if set the penultimate hop keeps the label
	M  bool // Mapping-Server: if set NP and E are ignored on reception
	E  bool // Explicit-NULL: if set the upstream neighbor uses the Explicit NULL label
	V  bool // Value/Index: set = absolute value, clear = index
	L  bool // Local/Global: set = local significance, clear = global
}

// OutgoingAction is what an upstream neighbor of the Prefix-SID originator does
// with the label when forwarding to a directly attached SR egress.
type OutgoingAction uint8

const (
	// ActionKeep keeps the Prefix-SID label on the stack (push at ingress, swap
	// at transit). NP=1/E=0, or M set.
	ActionKeep OutgoingAction = iota
	// ActionPHP is penultimate-hop popping: forward as plain IP / pop. NP=0.
	ActionPHP
	// ActionExplicitNull replaces the label with the family Explicit NULL. NP=1/E=1.
	ActionExplicitNull
)

// OutgoingActionFor applies the RFC 8665 §5 / RFC 8666 §6 truth table to the
// next-hop router's advertised flags:
//
//	M=1            -> ignore NP and E; keep the label (ActionKeep)
//	NP=0           -> penultimate hop pops (ActionPHP); received E ignored
//	NP=1, E=0      -> keep the label on top of the stack (ActionKeep)
//	NP=1, E=1      -> replace the label with Explicit NULL (ActionExplicitNull)
func OutgoingActionFor(f SIDFlags) OutgoingAction {
	if f.M {
		return ActionKeep
	}
	if !f.NP {
		return ActionPHP
	}
	if f.E {
		return ActionExplicitNull
	}
	return ActionKeep
}

// OutgoingLabel resolves an OutgoingAction to a concrete MPLS label. label is the
// Prefix-SID label computed from the SRGB; explicitNull is the family Explicit
// NULL label (ExplicitNullV4 or ExplicitNullV6). The bool reports whether a label
// is imposed at all: false means PHP (forward as plain IP / pop, no label).
func OutgoingLabel(label uint32, action OutgoingAction, explicitNull uint32) (uint32, bool) {
	switch action {
	case ActionExplicitNull:
		return explicitNull, true
	case ActionPHP:
		return 0, false
	default: // ActionKeep
		return label, true
	}
}
