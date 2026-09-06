// Design: docs/architecture/ike/ipsec-3-data-model.md -- the typed model between YANG and the engine
// Related: traffic_selector.go -- PortSelector, the port form both surfaces share
// Related: validate.go -- ValidatePolicyOrder, the same reserved band this file honors
// RFC: rfc/short/rfc4301.md -- SPD dispositions and ordering (Sections 4.4.1, 4.4.1.1, 7.4)

package ipsec

import (
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

// ErrSPDPolicy is the sentinel every refusal in this file wraps, so a caller can tell
// an operator-authored SPD entry apart from a peer or a group failure.
var ErrSPDPolicy = errors.New("ipsec spd policy")

// SPDDirection names which half of the Security Policy Database an entry belongs to.
//
// RFC 4301 Section 4.4.1 splits the database: "SPD-O: ... entries that define the
// policy for outbound traffic" and "SPD-I: ... entries that define the policy for
// inbound traffic". A policy is one-way, so an entry names one of them, or asks for
// the mirrored pair.
//
// The zero value names NEITHER, following the 1-based convention Mode and SADir use
// in the dataplane package. An entry that reached the engine with an unset direction
// skipped the parser, and installing it into a direction chosen here would put the
// operator's rule on a side they did not name.
type SPDDirection uint8

const (
	// SPDDirOut is RFC 4301's SPD-O: traffic leaving for the unprotected side.
	SPDDirOut SPDDirection = 1
	// SPDDirIn is RFC 4301's SPD-I: traffic arriving from the unprotected side.
	SPDDirIn SPDDirection = 2
	// SPDDirBoth installs the mirrored pair, one entry in each database.
	SPDDirBoth SPDDirection = 3
)

// String renders the operator spelling of this direction.
func (d SPDDirection) String() string {
	switch d {
	case SPDDirOut:
		return "out"
	case SPDDirIn:
		return "in"
	case SPDDirBoth:
		return "both"
	}
	return "unspecified"
}

// SPDPolicy is one operator-authored entry of the Security Policy Database.
//
// It carries the two dispositions of RFC 4301 Section 4.4.1 that no peer can produce.
// A PROTECT entry names an ESP transform and a peer to negotiate keys with, so it is
// written under site-to-site peer and reaches the dataplane through childPolicyParams.
// A BYPASS or a DISCARD entry names neither, so nothing negotiates it and nothing
// would install it without this type.
//
// Section 7.4 is why the DISCARD half exists: "All implementations MUST support
// DISCARDing of fragments using the normal SPD packet classification mechanisms."
// The disposition alone is not the obligation; an administrator has to be able to
// write one, which is what this type and its YANG list are.
type SPDPolicy struct {
	// Name identifies the entry in the configuration and in show output. It never
	// reaches a dataplane, which identifies a policy by its selector alone.
	Name string

	// Action is the disposition. It is SPActionBypass or SPActionDiscard, never
	// SPActionProtect: parseSPDPolicy refuses an absent or a protect value rather
	// than letting the SPAction zero value read as a disposition the operator did
	// not write (ai/rules/principles.md).
	Action dataplane.SPAction

	// Order is where this entry sits in the operator's total ordering of the
	// database. LOWER VALUE MEANS HIGHER PRECEDENCE, matching SPParams.Priority and
	// SiteToSitePeer.PolicyPriority, so one number means one thing across the
	// surface.
	Order uint32

	// Direction is which half of the database the entry belongs to.
	Direction SPDDirection

	// Protocol is the next-layer protocol selector of Section 4.4.1.1. Zero is that
	// section's ANY.
	Protocol uint8

	// LocalPrefix and RemotePrefix are the address selectors, always as written by
	// the operator: local is Ze's own side. The inbound entry of a both-direction
	// pair swaps them, because the local side of a flow is the DESTINATION of an
	// inbound packet, and that swap happens where the entry becomes SPParams rather
	// than here.
	LocalPrefix  *net.IPNet
	LocalPort    PortSelector
	RemotePrefix *net.IPNet
	RemotePort   PortSelector
}

// parseSPDPolicies reads the operator's policy list from the ipsec container.
// An absent list yields no entries, which is what every configuration written
// before the list existed carries.
func parseSPDPolicies(t *config.Tree) (map[string]SPDPolicy, error) {
	entries := t.GetListOrdered("policy")
	out := make(map[string]SPDPolicy, len(entries))
	for _, entry := range entries {
		p, err := parseSPDPolicy(entry.Key, entry.Value)
		if err != nil {
			return nil, err
		}
		out[entry.Key] = p
	}
	return out, nil
}

// parseSPDPolicy reads one entry. Every default it resolves is written here rather
// than left to a Go zero value, because two of the fields have a zero that reads as a
// legitimate value: SPAction's zero is PROTECT and SPDDirection's zero is no
// direction at all.
func parseSPDPolicy(name string, t *config.Tree) (SPDPolicy, error) {
	if err := validateName(name); err != nil {
		return SPDPolicy{}, fmt.Errorf("%w %q: %w", ErrSPDPolicy, name, err)
	}

	p := SPDPolicy{
		Name: name,
		// The YANG default. Resolved here so a policy that reaches the engine
		// carries a rank the kernel can order, never a zero that would outrank the
		// IKE control-plane bypass (dataplane.PriorityIKEBypass).
		Order:      1000,
		Direction:  SPDDirBoth,
		LocalPort:  AnyPort(),
		RemotePort: AnyPort(),
	}

	action, ok := t.Get("action")
	if !ok {
		return p, fmt.Errorf(
			"%w %q: action is required; accepted values are bypass and discard",
			ErrSPDPolicy, name)
	}
	switch action {
	case "bypass":
		p.Action = dataplane.SPActionBypass
	case "discard":
		p.Action = dataplane.SPActionDiscard
	default:
		return p, fmt.Errorf(
			"%w %q: action %q is not an SPD disposition this list carries; accepted values are bypass and discard. "+
				"A protect entry names a transform and a peer, so it is written under site-to-site peer",
			ErrSPDPolicy, name, action)
	}

	if v, ok := t.Get("order"); ok {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return p, fmt.Errorf("%w %q: order %q is not a number: %w", ErrSPDPolicy, name, v, err)
		}
		p.Order = uint32(n)
	}

	if v, ok := t.Get("direction"); ok {
		switch v {
		case "out":
			p.Direction = SPDDirOut
		case "in":
			p.Direction = SPDDirIn
		case "both":
			p.Direction = SPDDirBoth
		default:
			return p, fmt.Errorf(
				"%w %q: direction %q is not a side of the IPsec boundary; accepted values are out, in and both",
				ErrSPDPolicy, name, v)
		}
	}

	if v, ok := t.Get("protocol"); ok {
		proto, err := strconv.ParseUint(v, 10, 8)
		if err != nil {
			return p, fmt.Errorf("%w %q: protocol %q is not a number in 0..255: %w", ErrSPDPolicy, name, v, err)
		}
		p.Protocol = uint8(proto)
	}

	for _, side := range []struct {
		name   string
		prefix **net.IPNet
		port   *PortSelector
	}{
		{"local", &p.LocalPrefix, &p.LocalPort},
		{"remote", &p.RemotePrefix, &p.RemotePort},
	} {
		sub := t.GetContainer(side.name)
		if sub == nil {
			continue
		}
		if v, ok := sub.Get("prefix"); ok {
			_, n, err := net.ParseCIDR(v)
			if err != nil {
				return p, fmt.Errorf("%w %q: %s prefix %q: %w", ErrSPDPolicy, name, side.name, v, err)
			}
			*side.prefix = n
		}
		if v, ok := sub.Get("port"); ok {
			sel, err := parseSPDPort(name, side.name, v)
			if err != nil {
				return p, err
			}
			*side.port = sel
		}
	}

	return p, nil
}

// parseSPDPort reads one port leaf.
//
// It accepts ANY and one exact port and refuses everything else, which is exactly
// what the kernel selector Ze programs can express (xfrmSelectorPort, xfrm_linux.go).
// OPAQUE is refused rather than accepted: RFC 4301 Section 4.4.1.1 defines it as "the
// selector field is not available", which describes a PACKET rather than a rule an
// operator writes, and the traffic-selector list carries it only because RFC 7296
// Section 3.13.1 puts it on the wire.
func parseSPDPort(name, side, v string) (PortSelector, error) {
	if v == "" || v == portAnyKeyword {
		return PortSelector{Form: PortAny}, nil
	}
	port, err := strconv.ParseUint(v, 10, 16)
	if err != nil || port == 0 {
		return PortSelector{}, fmt.Errorf(
			"%w %q: %s port %q is not a port; accepted values are any or a port number in 1..65535",
			ErrSPDPolicy, name, side, v)
	}
	return PortSelector{Form: PortSingle, Port: uint16(port)}, nil
}

// ValidateSPDPolicies refuses every operator SPD entry a dataplane cannot program
// exactly, and every entry whose rank would capture the IKE control plane.
//
// It runs at config verify, which is the only place the operator can still act on the
// refusal (ai/rules/protocol.md). An entry approximated at install time would bypass
// or discard traffic the operator never named, and this list is the one surface where
// a widened selector is a security failure in both directions: a widened BYPASS
// passes protected traffic in the clear, and a widened DISCARD black-holes traffic
// the operator meant to carry.
func (c *IPsecConfig) ValidateSPDPolicies() error {
	for name := range c.Policies {
		if err := c.Policies[name].validate(); err != nil {
			return err
		}
	}
	return nil
}

// validate holds the per-entry rules. It is a method on the entry so the same checks
// run for a policy that arrives from anywhere, not only from the config tree.
func (p SPDPolicy) validate() error {
	if p.LocalPrefix == nil {
		return fmt.Errorf("%w %q: local prefix is required", ErrSPDPolicy, p.Name)
	}
	if p.RemotePrefix == nil {
		return fmt.Errorf("%w %q: remote prefix is required", ErrSPDPolicy, p.Name)
	}

	// A kernel selector carries ONE address family (XfrmSelector.Family, vendor
	// nl/xfrm_linux.go). A mixed pair therefore has no faithful projection, and
	// picking a family here would install a rule over addresses the operator did not
	// write.
	if (p.LocalPrefix.IP.To4() != nil) != (p.RemotePrefix.IP.To4() != nil) {
		return fmt.Errorf(
			"%w %q: local prefix %s and remote prefix %s are different address families, and one policy selector carries one family",
			ErrSPDPolicy, p.Name, p.LocalPrefix, p.RemotePrefix)
	}

	// RFC 4301 Section 4.4.1: "Thus, a user or administrator MUST be able to order
	// the entries to express a desired access control policy." The IKE control-plane
	// bypass is an entry of the same database, and an operator entry at or above it
	// matches the IKE datagrams first. A DISCARD there stops every tunnel on the node
	// from being built, rekeyed or torn down, and the kernel reports nothing: it
	// picked the entry the operator ranked highest, exactly as asked.
	if p.Order <= dataplane.PriorityIKEBypass {
		return fmt.Errorf(
			"%w %q: order %d ranks at or above the IKE control-plane bypass (%d), so this entry would "+
				"capture the IKE exchange that builds and rekeys every tunnel on this node. Set order above %d",
			ErrSPDPolicy, p.Name, p.Order, dataplane.PriorityIKEBypass, dataplane.PriorityIKEBypass)
	}

	// RFC 4301 Section 4.4.1.1 lists the port selectors under "Next Layer Protocol",
	// and the port fields exist only for a protocol that carries them. A port
	// selector under protocol 0 asks the classifier to read a field it has not been
	// told is there, and the kernel would match it against whatever the transport
	// header decode left in the flow key.
	if !p.LocalPort.IsAny() || !p.RemotePort.IsAny() {
		if !protocolCarriesPorts(p.Protocol) {
			return fmt.Errorf(
				"%w %q: a port selector needs a protocol that carries ports, and protocol is %d. "+
					"Set protocol to 6 (TCP), 17 (UDP) or 132 (SCTP), or set both ports to any",
				ErrSPDPolicy, p.Name, p.Protocol)
		}
	}

	return nil
}

// protocolCarriesPorts reports whether an IP protocol number names a transport that
// has port fields. RFC 4301 Section 4.4.1.1 names the three: "Local Port ... TCP/UDP/
// SCTP source port".
func protocolCarriesPorts(proto uint8) bool {
	switch proto {
	case protoTCP, protoUDP, protoSCTP:
		return true
	}
	return false
}

// IP protocol numbers of the three transports RFC 4301 Section 4.4.1.1 names as
// carrying ports (IANA "Protocol Numbers" registry).
const (
	protoTCP  uint8 = 6
	protoUDP  uint8 = 17
	protoSCTP uint8 = 132
)
