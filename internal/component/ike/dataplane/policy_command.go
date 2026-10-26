// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the command that removes a stranded policy
// Related: vpp_policy.go -- the VPP backend's answer to PolicyDeleteCommander

package dataplane

import (
	"net"
	"strconv"
	"strings"
)

// PolicyDeleteCommander is implemented by a backend that can name, for an operator, the
// command that removes one policy it installed. The engine puts that command in the
// warning and the refusal it writes when a policy removal failed and the policy stayed
// in the dataplane, so the operator can remove it by hand.
//
// The command is the backend's, because only the backend knows its own tool: XFRM
// answers with `ip xfrm policy delete`, VPP with `vppctl ipsec policy del`. A backend
// that does not implement the interface has named no command, and the engine says so
// rather than printing another backend's command. The noop backend does not implement
// it: its removal never fails, so it never leaves a policy to name.
type PolicyDeleteCommander interface {
	// PolicyDeleteCommand returns the command that removes the policy p names, or an
	// error saying why this backend cannot name one.
	PolicyDeleteCommand(p SPParams) (string, error)
}

// PolicyDeleteCommand implements PolicyDeleteCommander with the `ip xfrm policy delete`
// command, which needs no state of the backend: the kernel matches a removal on the
// selector alone.
func (b *xfrmBackend) PolicyDeleteCommand(p SPParams) (string, error) {
	return XFRMPolicyDeleteCommand(&p), nil
}

// XFRMPolicyDeleteCommand writes the `ip xfrm policy delete` command whose selector is
// p's.
//
// A nil prefix is written as the wildcard of the family the kernel writer gives it
// (the destination's, IPv4 when both are nil), because `ip xfrm` needs a family to
// match the entry the kernel holds. A port the selector constrains is written with
// its protocol, which `ip xfrm` requires before sport and dport. Priority is not
// written: it ranks an entry but is no part of its identity, so the kernel does not
// match a removal on it and `ip xfrm policy delete` takes no priority argument.
func XFRMPolicyDeleteCommand(p *SPParams) string {
	ipv6 := p.Dst != nil && p.Dst.IP.To4() == nil
	var b strings.Builder
	b.WriteString("ip xfrm policy delete ")
	b.WriteString(XFRMDirKeyword(p.Dir))
	b.WriteString(" src ")
	b.WriteString(xfrmPrefixLabel(p.Src, ipv6))
	b.WriteString(" dst ")
	b.WriteString(xfrmPrefixLabel(p.Dst, ipv6))
	if p.UpperProto != 0 {
		b.WriteString(" proto ")
		b.WriteString(strconv.Itoa(int(p.UpperProto)))
	}
	if !p.SrcPort.IsAny() {
		b.WriteString(" sport ")
		b.WriteString(xfrmPortLabel(p.SrcPort))
	}
	if !p.DstPort.IsAny() {
		b.WriteString(" dport ")
		b.WriteString(xfrmPortLabel(p.DstPort))
	}
	if p.IfID != 0 {
		b.WriteString(" if_id ")
		b.WriteString(string(strconv.AppendUint(nil, uint64(p.IfID), 10)))
	}
	return b.String()
}

// XFRMDirKeyword is the `ip xfrm policy` keyword for a direction, so an operator can
// paste a logged entry into the command that removes it.
func XFRMDirKeyword(dir SADir) string {
	switch dir {
	case SADirIn:
		return "dir in"
	case SADirOut:
		return "dir out"
	case SADirFwd:
		return "dir fwd"
	}
	return "dir unknown"
}

// xfrmPrefixLabel names a selector prefix, writing a nil one as its family's wildcard.
func xfrmPrefixLabel(n *net.IPNet, ipv6 bool) string {
	if n != nil {
		return n.String()
	}
	if ipv6 {
		return "::/0"
	}
	return "0.0.0.0/0"
}

// xfrmPortLabel names a port match. The XFRM backend installs only an exact port
// (xfrmSelectorPort refuses any other mask), so a partial mask can only come from
// another backend; it is written as port/mask, which `ip xfrm` refuses, rather than
// as the bare port, which would name a narrower entry than the one left behind.
func xfrmPortLabel(m PortMatch) string {
	port := strconv.Itoa(int(m.Port))
	if m.Mask == 0xffff {
		return port
	}
	return port + "/0x" + string(strconv.AppendUint(nil, uint64(m.Mask), 16))
}
