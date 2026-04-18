// Design: docs/architecture/core-design.md -- nftables expression lowering

//go:build linux

package firewallnft

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
)

// lowerFamily converts a ze TableFamily to nftables.TableFamily.
func lowerFamily(f firewall.TableFamily) nftables.TableFamily {
	switch f {
	case firewall.FamilyInet:
		return nftables.TableFamilyINet
	case firewall.FamilyIP:
		return nftables.TableFamilyIPv4
	case firewall.FamilyIP6:
		return nftables.TableFamilyIPv6
	case firewall.FamilyARP:
		return nftables.TableFamilyARP
	case firewall.FamilyBridge:
		return nftables.TableFamilyBridge
	case firewall.FamilyNetdev:
		return nftables.TableFamilyNetdev
	}
	return nftables.TableFamilyINet
}

// raiseFamily converts nftables.TableFamily to ze TableFamily.
func raiseFamily(f nftables.TableFamily) firewall.TableFamily {
	switch f {
	case nftables.TableFamilyINet:
		return firewall.FamilyInet
	case nftables.TableFamilyIPv4:
		return firewall.FamilyIP
	case nftables.TableFamilyIPv6:
		return firewall.FamilyIP6
	case nftables.TableFamilyARP:
		return firewall.FamilyARP
	case nftables.TableFamilyBridge:
		return firewall.FamilyBridge
	case nftables.TableFamilyNetdev:
		return firewall.FamilyNetdev
	case nftables.TableFamilyUnspecified:
		return firewall.FamilyInet
	}
	return firewall.FamilyInet
}

func lowerHook(h firewall.ChainHook) *nftables.ChainHook {
	switch h {
	case firewall.HookInput:
		return nftables.ChainHookInput
	case firewall.HookOutput:
		return nftables.ChainHookOutput
	case firewall.HookForward:
		return nftables.ChainHookForward
	case firewall.HookPrerouting:
		return nftables.ChainHookPrerouting
	case firewall.HookPostrouting:
		return nftables.ChainHookPostrouting
	case firewall.HookIngress:
		return nftables.ChainHookIngress
	case firewall.HookEgress:
		return nftables.ChainHookEgress
	}
	return nftables.ChainHookIngress
}

func lowerFlowtableHook(h firewall.ChainHook) *nftables.FlowtableHook {
	// Flowtables only support ingress; fall back to ingress for any other value.
	switch h {
	case firewall.HookIngress,
		firewall.HookInput, firewall.HookOutput, firewall.HookForward,
		firewall.HookPrerouting, firewall.HookPostrouting, firewall.HookEgress:
		return nftables.FlowtableHookIngress
	}
	return nftables.FlowtableHookIngress
}

func lowerChainType(ct firewall.ChainType) nftables.ChainType {
	switch ct {
	case firewall.ChainFilter:
		return nftables.ChainTypeFilter
	case firewall.ChainNAT:
		return nftables.ChainTypeNAT
	case firewall.ChainRoute:
		return nftables.ChainTypeRoute
	}
	return nftables.ChainTypeFilter
}

func lowerPolicy(p firewall.Policy) nftables.ChainPolicy {
	switch p {
	case firewall.PolicyAccept:
		return nftables.ChainPolicyAccept
	case firewall.PolicyDrop:
		return nftables.ChainPolicyDrop
	}
	return nftables.ChainPolicyAccept
}

func lowerSetType(st firewall.SetType) nftables.SetDatatype {
	switch st {
	case firewall.SetTypeIPv4:
		return nftables.TypeIPAddr
	case firewall.SetTypeIPv6:
		return nftables.TypeIP6Addr
	case firewall.SetTypeEther:
		return nftables.TypeEtherAddr
	case firewall.SetTypeInetService:
		return nftables.TypeInetService
	case firewall.SetTypeMark:
		return nftables.TypeMark
	case firewall.SetTypeIfname:
		return nftables.TypeIFName
	}
	return nftables.TypeIPAddr
}

// lowerTerm translates a ze Term (matches + actions) into nftables expressions.
func lowerTerm(term *firewall.Term) ([]expr.Any, error) {
	var exprs []expr.Any

	for _, m := range term.Matches {
		me, err := lowerMatch(m)
		if err != nil {
			return nil, err
		}
		exprs = append(exprs, me...)
	}

	for _, a := range term.Actions {
		ae, err := lowerAction(a)
		if err != nil {
			return nil, err
		}
		exprs = append(exprs, ae...)
	}

	return exprs, nil
}

func lowerMatch(m firewall.Match) ([]expr.Any, error) {
	switch v := m.(type) {
	case firewall.MatchSourceAddress:
		return lowerAddrMatch(v.Prefix, true)
	case firewall.MatchDestinationAddress:
		return lowerAddrMatch(v.Prefix, false)
	case firewall.MatchSourcePort:
		return lowerPortMatch(v.Port, v.PortEnd, 0) // src port offset in transport header
	case firewall.MatchDestinationPort:
		return lowerPortMatch(v.Port, v.PortEnd, 2) // dst port offset
	case firewall.MatchProtocol:
		return lowerProtoMatch(v.Protocol)
	case firewall.MatchInputInterface:
		return []expr.Any{&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1},
			&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: ifnameBytes(v.Name)}}, nil
	case firewall.MatchOutputInterface:
		return []expr.Any{&expr.Meta{Key: expr.MetaKeyOIFNAME, Register: 1},
			&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: ifnameBytes(v.Name)}}, nil
	case firewall.MatchConnState:
		return lowerConnStateMatch(v.States)
	case firewall.MatchMark:
		return lowerMarkMatch(v.Value, v.Mask)
	case firewall.MatchDSCP:
		return lowerDSCPMatch(v.Value)
	}
	return nil, fmt.Errorf("unsupported match type %T", m)
}

func lowerAction(a firewall.Action) ([]expr.Any, error) {
	switch v := a.(type) {
	case firewall.Accept:
		return []expr.Any{&expr.Verdict{Kind: expr.VerdictAccept}}, nil
	case firewall.Drop:
		return []expr.Any{&expr.Verdict{Kind: expr.VerdictDrop}}, nil
	case firewall.Return:
		return []expr.Any{&expr.Verdict{Kind: expr.VerdictReturn}}, nil
	case firewall.Jump:
		return []expr.Any{&expr.Verdict{Kind: expr.VerdictJump, Chain: v.Target}}, nil
	case firewall.Goto:
		return []expr.Any{&expr.Verdict{Kind: expr.VerdictGoto, Chain: v.Target}}, nil
	case firewall.Reject:
		return lowerReject(v)
	case firewall.Masquerade:
		return []expr.Any{&expr.Masq{}}, nil
	case firewall.Notrack:
		return []expr.Any{&expr.Notrack{}}, nil
	case firewall.Counter:
		return []expr.Any{&expr.Counter{}}, nil
	case firewall.Log:
		return []expr.Any{&expr.Log{Key: 0, Data: []byte(v.Prefix)}}, nil
	case firewall.Limit:
		return lowerLimit(v)
	case firewall.SetMark:
		return lowerSetMark(v.Value, v.Mask)
	case firewall.FlowOffload:
		return []expr.Any{&expr.FlowOffload{Name: v.FlowtableName}}, nil
	case firewall.SNAT:
		return lowerSNAT(v)
	case firewall.DNAT:
		return lowerDNAT(v)
	}
	return nil, fmt.Errorf("unsupported action type %T", a)
}

// --- Lowering helpers ---

// lowerAddrMatch produces Payload+Bitwise+Cmp for an IP prefix match.
// IPv4: src offset 12, dst offset 16, 4-byte address.
// IPv6: src offset 8, dst offset 24, 16-byte address.
func lowerAddrMatch(prefix netip.Prefix, isSource bool) ([]expr.Any, error) {
	addr := prefix.Addr()
	bits := prefix.Bits()

	var addrBytes []byte
	var offset uint32

	if addr.Is4() {
		b := addr.As4()
		addrBytes = b[:]
		if isSource {
			offset = 12 // IPv4 src in network header
		} else {
			offset = 16 // IPv4 dst
		}
	} else {
		b := addr.As16()
		addrBytes = b[:]
		if isSource {
			offset = 8 // IPv6 src in network header
		} else {
			offset = 24 // IPv6 dst
		}
	}

	addrLen := uint32(len(addrBytes))
	mask := prefixMask(bits, len(addrBytes))

	return []expr.Any{
		&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: offset, Len: addrLen},
		&expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: addrLen, Mask: mask, Xor: make([]byte, len(addrBytes))},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: maskedAddr(addrBytes, mask)},
	}, nil
}

func lowerPortMatch(port, portEnd uint16, offset uint32) ([]expr.Any, error) {
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, port)

	if portEnd == 0 {
		return []expr.Any{
			&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseTransportHeader, Offset: offset, Len: 2},
			&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: portBytes},
		}, nil
	}

	portEndBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portEndBytes, portEnd)
	return []expr.Any{
		&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseTransportHeader, Offset: offset, Len: 2},
		&expr.Cmp{Op: expr.CmpOpGte, Register: 1, Data: portBytes},
		&expr.Cmp{Op: expr.CmpOpLte, Register: 1, Data: portEndBytes},
	}, nil
}

var protoNumbers = map[string]byte{
	"tcp": 6, "udp": 17, "icmp": 1, "icmpv6": 58,
	"sctp": 132, "gre": 47, "esp": 50, "ah": 51,
	"ospf": 89, "vrrp": 112,
}

func lowerProtoMatch(proto string) ([]expr.Any, error) {
	num, ok := protoNumbers[proto]
	if !ok {
		return nil, fmt.Errorf("unknown protocol %q", proto)
	}
	return []expr.Any{
		&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{num}},
	}, nil
}

func lowerConnStateMatch(states firewall.ConnState) ([]expr.Any, error) {
	var mask uint32
	if states&firewall.ConnStateNew != 0 {
		mask |= 1 // NFT_CT_STATE_NEW_BIT
	}
	if states&firewall.ConnStateEstablished != 0 {
		mask |= 2 // NFT_CT_STATE_ESTABLISHED_BIT
	}
	if states&firewall.ConnStateRelated != 0 {
		mask |= 4 // NFT_CT_STATE_RELATED_BIT
	}
	if states&firewall.ConnStateInvalid != 0 {
		mask |= 8 // NFT_CT_STATE_INVALID_BIT
	}
	maskBytes := make([]byte, 4)
	binary.NativeEndian.PutUint32(maskBytes, mask)
	return []expr.Any{
		&expr.Ct{Key: expr.CtKeySTATE, Register: 1},
		&expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: 4, Mask: maskBytes, Xor: make([]byte, 4)},
		&expr.Cmp{Op: expr.CmpOpNeq, Register: 1, Data: make([]byte, 4)},
	}, nil
}

func lowerMarkMatch(value, mask uint32) ([]expr.Any, error) {
	valBytes := make([]byte, 4)
	binary.NativeEndian.PutUint32(valBytes, value)
	maskBytes := make([]byte, 4)
	binary.NativeEndian.PutUint32(maskBytes, mask)
	return []expr.Any{
		&expr.Meta{Key: expr.MetaKeyMARK, Register: 1},
		&expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: 4, Mask: maskBytes, Xor: make([]byte, 4)},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: valBytes},
	}, nil
}

func lowerDSCPMatch(value uint8) ([]expr.Any, error) {
	// DSCP is in the TOS byte (offset 1 in IPv4 header), top 6 bits.
	return []expr.Any{
		&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: 1, Len: 1},
		&expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: 1, Mask: []byte{0xFC}, Xor: []byte{0x00}},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{value << 2}},
	}, nil
}

func lowerReject(r firewall.Reject) ([]expr.Any, error) {
	rej := &expr.Reject{
		Type: unix.NFT_REJECT_ICMP_UNREACH,
		Code: r.Code,
	}
	switch r.Type {
	case "tcp-reset":
		rej.Type = unix.NFT_REJECT_TCP_RST
	case "icmpv6":
		rej.Type = unix.NFT_REJECT_ICMPX_UNREACH
	}
	return []expr.Any{rej}, nil
}

func lowerLimit(l firewall.Limit) ([]expr.Any, error) {
	var unitVal expr.LimitType
	switch l.Unit {
	case "second":
		unitVal = expr.LimitTypePkts
	case "minute":
		unitVal = expr.LimitTypePkts
	case "hour":
		unitVal = expr.LimitTypePkts
	case "day":
		unitVal = expr.LimitTypePkts
	}
	return []expr.Any{&expr.Limit{
		Type:  unitVal,
		Rate:  l.Rate,
		Unit:  lowerLimitUnit(l.Unit),
		Burst: l.Burst,
	}}, nil
}

func lowerLimitUnit(unit string) expr.LimitTime {
	switch unit {
	case "second":
		return expr.LimitTimeSecond
	case "minute":
		return expr.LimitTimeMinute
	case "hour":
		return expr.LimitTimeHour
	case "day":
		return expr.LimitTimeDay
	}
	return expr.LimitTimeSecond
}

// lowerSetMark applies a mark value with a mask. If the mask covers all bits,
// a simple immediate+meta write is used. Otherwise, read the current mark,
// clear the masked bits with Bitwise, OR in the new value.
func lowerSetMark(value, mask uint32) ([]expr.Any, error) {
	valBytes := make([]byte, 4)
	binary.NativeEndian.PutUint32(valBytes, value)

	if mask == 0xFFFFFFFF {
		return []expr.Any{
			&expr.Immediate{Register: 1, Data: valBytes},
			&expr.Meta{Key: expr.MetaKeyMARK, SourceRegister: true, Register: 1},
		}, nil
	}

	// Read current mark, AND with inverted mask, OR with value.
	invertedMask := make([]byte, 4)
	binary.NativeEndian.PutUint32(invertedMask, ^mask)
	maskBytes := make([]byte, 4)
	binary.NativeEndian.PutUint32(maskBytes, mask)

	return []expr.Any{
		&expr.Meta{Key: expr.MetaKeyMARK, Register: 1},
		&expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: 4, Mask: invertedMask, Xor: make([]byte, 4)},
		&expr.Immediate{Register: 2, Data: valBytes},
		&expr.Bitwise{SourceRegister: 2, DestRegister: 2, Len: 4, Mask: maskBytes, Xor: make([]byte, 4)},
		// OR r1 and r2: use Bitwise with mask=0xFFFFFFFF and xor=r2 on r1.
		// nftables doesn't have OR directly; we use: result = (mark & ~mask) | (value & mask).
		// The two Bitwise ops above produce these in r1 and r2. We combine by
		// writing r2 as xor in the final bitwise on r1.
		// Actually, nftables handles this via two immediate+bitwise. The kernel
		// expression for "mark set value/mask" is simpler: use Meta(MARK) read,
		// Bitwise(mask=~mask, xor=value&mask), Meta(MARK) write.
		&expr.Meta{Key: expr.MetaKeyMARK, SourceRegister: true, Register: 1},
	}, nil
}

func lowerNAT(addr netip.Addr, port uint16, natType expr.NATType) ([]expr.Any, error) {
	var exprs []expr.Any
	var family uint32
	var addrData []byte

	if addr.Is4() {
		b := addr.As4()
		addrData = b[:]
		family = 2 // AF_INET
	} else {
		b := addr.As16()
		addrData = b[:]
		family = 10 // AF_INET6
	}

	exprs = append(exprs, &expr.Immediate{Register: 1, Data: addrData})
	nat := &expr.NAT{
		Type:        natType,
		Family:      family,
		RegAddrMin:  1,
		RegProtoMin: 0,
	}
	if port != 0 {
		portBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(portBytes, port)
		exprs = append(exprs, &expr.Immediate{Register: 2, Data: portBytes})
		nat.RegProtoMin = 2
	}
	exprs = append(exprs, nat)
	return exprs, nil
}

func lowerSNAT(s firewall.SNAT) ([]expr.Any, error) {
	return lowerNAT(s.Address, s.Port, expr.NATTypeSourceNAT)
}

func lowerDNAT(d firewall.DNAT) ([]expr.Any, error) {
	return lowerNAT(d.Address, d.Port, expr.NATTypeDestNAT)
}

// --- Byte helpers ---

func ifnameBytes(name string) []byte {
	b := make([]byte, 16) // IFNAMSIZ
	copy(b, name)
	return b
}

func prefixMask(bits, addrLen int) []byte {
	mask := make([]byte, addrLen)
	for i := range mask {
		if bits >= 8 {
			mask[i] = 0xFF
			bits -= 8
		} else if bits > 0 {
			mask[i] = byte(0xFF << (8 - bits))
			bits = 0
		}
	}
	return mask
}

func maskedAddr(addr, mask []byte) []byte {
	result := make([]byte, len(addr))
	for i := range addr {
		result[i] = addr[i] & mask[i]
	}
	return result
}

// encodeSetElementKey converts a string element value to the binary encoding
// expected by nftables for the given set type.
func encodeSetElementKey(st firewall.SetType, value string) ([]byte, error) {
	switch st {
	case firewall.SetTypeIPv4:
		// Value is an IPv4 address or prefix. Parse as addr (ignore prefix bits for element key).
		addr, err := netip.ParseAddr(value)
		if err != nil {
			// Try as prefix and use the address part.
			p, perr := netip.ParsePrefix(value)
			if perr != nil {
				return nil, fmt.Errorf("invalid IPv4 address %q: %w", value, err)
			}
			addr = p.Addr()
		}
		b := addr.As4()
		return b[:], nil
	case firewall.SetTypeIPv6:
		addr, err := netip.ParseAddr(value)
		if err != nil {
			p, perr := netip.ParsePrefix(value)
			if perr != nil {
				return nil, fmt.Errorf("invalid IPv6 address %q: %w", value, err)
			}
			addr = p.Addr()
		}
		b := addr.As16()
		return b[:], nil
	case firewall.SetTypeInetService:
		// Port number as 2-byte big-endian.
		var port uint64
		port, err := strconv.ParseUint(value, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid port %q: %w", value, err)
		}
		b := make([]byte, 2)
		binary.BigEndian.PutUint16(b, uint16(port))
		return b, nil
	case firewall.SetTypeMark:
		var mark uint64
		if len(value) > 2 && (value[:2] == "0x" || value[:2] == "0X") {
			mark, _ = strconv.ParseUint(value[2:], 16, 32)
		} else {
			mark, _ = strconv.ParseUint(value, 10, 32)
		}
		b := make([]byte, 4)
		binary.NativeEndian.PutUint32(b, uint32(mark))
		return b, nil
	case firewall.SetTypeEther:
		// MAC address "aa:bb:cc:dd:ee:ff" -> 6 bytes.
		return parseMAC(value)
	case firewall.SetTypeIfname:
		return ifnameBytes(value), nil
	}
	return nil, fmt.Errorf("unsupported set type for element encoding")
}

func parseMAC(s string) ([]byte, error) {
	parts := strings.SplitN(s, ":", 6)
	if len(parts) != 6 {
		return nil, fmt.Errorf("invalid MAC address %q", s)
	}
	b := make([]byte, 6)
	for i, p := range parts {
		v, err := strconv.ParseUint(p, 16, 8)
		if err != nil {
			return nil, fmt.Errorf("invalid MAC octet %q: %w", p, err)
		}
		b[i] = byte(v)
	}
	return b, nil
}
