// Design: docs/architecture/core-design.md -- nftables expression lowering
// Related: readback_linux.go -- inverse direction (kernel -> ze model)
//
// Register allocation. nftables packet evaluation holds a 16-register
// file that each expression writes to then reads from, in sequence,
// within one rule. The helpers below use registers as follows:
//
//   r1: scratch -- every match (payload/meta + cmp) writes and reads r1
//       in the same expression block; the value does not outlive the
//       block. Also used by set-mark/NAT action blocks to carry the
//       address or mark value to the Meta / NAT / Masq expression that
//       follows.
//   r2: NAT port scratch -- written by an Immediate immediately before
//       the consuming NAT expression, so it never outlives the action
//       block. Set-mark / set-conn-mark masked paths fold the value
//       into the single Bitwise step and do not need r2.
//   r3: NAT port-range upper bound -- written by an Immediate in the
//       same block as the NAT that consumes it.
//
// The sequential "write-then-read" discipline means two action blocks
// can reuse r2 / r3 without clobbering each other, because every block
// writes the register it needs just before reading. Adding a new helper
// that reads a register it did not itself write is the only way to
// introduce a real conflict; that pattern is forbidden.

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
// Unknown values reject rather than coerce to a default: silently
// programming a different family than the operator asked for would
// violate rules/exact-or-reject.md.
func lowerFamily(f firewall.TableFamily) (nftables.TableFamily, error) {
	switch f {
	case firewall.FamilyInet:
		return nftables.TableFamilyINet, nil
	case firewall.FamilyIP:
		return nftables.TableFamilyIPv4, nil
	case firewall.FamilyIP6:
		return nftables.TableFamilyIPv6, nil
	case firewall.FamilyARP:
		return nftables.TableFamilyARP, nil
	case firewall.FamilyBridge:
		return nftables.TableFamilyBridge, nil
	case firewall.FamilyNetdev:
		return nftables.TableFamilyNetdev, nil
	}
	return 0, fmt.Errorf("unknown table family %q", f)
}

// raiseFamily converts nftables.TableFamily to ze TableFamily. Unknown values
// return an error so readback surfaces kernel state it cannot represent instead
// of misattributing the table family.
func raiseFamily(f nftables.TableFamily) (firewall.TableFamily, error) {
	switch f {
	case nftables.TableFamilyINet:
		return firewall.FamilyInet, nil
	case nftables.TableFamilyIPv4:
		return firewall.FamilyIP, nil
	case nftables.TableFamilyIPv6:
		return firewall.FamilyIP6, nil
	case nftables.TableFamilyARP:
		return firewall.FamilyARP, nil
	case nftables.TableFamilyBridge:
		return firewall.FamilyBridge, nil
	case nftables.TableFamilyNetdev:
		return firewall.FamilyNetdev, nil
	}
	return 0, fmt.Errorf("unknown kernel table family %d", f)
}

func lowerHook(h firewall.ChainHook) (*nftables.ChainHook, error) {
	switch h {
	case firewall.HookInput:
		return nftables.ChainHookInput, nil
	case firewall.HookOutput:
		return nftables.ChainHookOutput, nil
	case firewall.HookForward:
		return nftables.ChainHookForward, nil
	case firewall.HookPrerouting:
		return nftables.ChainHookPrerouting, nil
	case firewall.HookPostrouting:
		return nftables.ChainHookPostrouting, nil
	case firewall.HookIngress:
		return nftables.ChainHookIngress, nil
	case firewall.HookEgress:
		return nftables.ChainHookEgress, nil
	}
	return nil, fmt.Errorf("unknown chain hook %q", h)
}

// lowerFlowtableHook rejects any hook other than ingress: flowtables are
// defined only at ingress, and silently remapping input/output/etc. to
// ingress would drop the operator's intent.
func lowerFlowtableHook(h firewall.ChainHook) (*nftables.FlowtableHook, error) {
	if h == firewall.HookIngress {
		return nftables.FlowtableHookIngress, nil
	}
	return nil, fmt.Errorf("flowtable hook %q unsupported; only \"ingress\" is valid", h)
}

func lowerChainType(ct firewall.ChainType) (nftables.ChainType, error) {
	switch ct {
	case firewall.ChainFilter:
		return nftables.ChainTypeFilter, nil
	case firewall.ChainNAT:
		return nftables.ChainTypeNAT, nil
	case firewall.ChainRoute:
		return nftables.ChainTypeRoute, nil
	}
	return "", fmt.Errorf("unknown chain type %q", ct)
}

func lowerPolicy(p firewall.Policy) (nftables.ChainPolicy, error) {
	switch p {
	case firewall.PolicyAccept:
		return nftables.ChainPolicyAccept, nil
	case firewall.PolicyDrop:
		return nftables.ChainPolicyDrop, nil
	}
	return 0, fmt.Errorf("unknown chain policy %q", p)
}

func lowerSetType(st firewall.SetType) (nftables.SetDatatype, error) {
	switch st {
	case firewall.SetTypeIPv4:
		return nftables.TypeIPAddr, nil
	case firewall.SetTypeIPv6:
		return nftables.TypeIP6Addr, nil
	case firewall.SetTypeEther:
		return nftables.TypeEtherAddr, nil
	case firewall.SetTypeInetService:
		return nftables.TypeInetService, nil
	case firewall.SetTypeMark:
		return nftables.TypeMark, nil
	case firewall.SetTypeIfname:
		return nftables.TypeIFName, nil
	}
	return nftables.SetDatatype{}, fmt.Errorf("unknown set type %d", st)
}

// lowerCtx carries the nftables Conn + parent Table to helpers that need to
// register anonymous sets before emitting a Lookup expression. A nil ctx
// blocks lowerings that require a set (multi-range port match); pure-
// expression lowerings ignore it.
//
// sets is the table's named-set registry, keyed by ze set name. Named-set
// matches (MatchInSet) look up the *nftables.Set here to recover its ID +
// Name for the Lookup expression. Sets must be added to the connection
// (conn.AddSet) before the chain that references them; applyTable applies
// sets before chains to preserve that order.
type lowerCtx struct {
	conn  *nftables.Conn
	table *nftables.Table
	sets  map[string]*nftables.Set
}

// lowerTerm translates a ze Term (matches + actions) into nftables expressions.
// The context allows helpers that need to register anonymous sets (e.g. a
// multi-range port match lowers to a Lookup against an anonymous interval set).
// Pass a nil ctx in contexts that cannot materialise sets; helpers that need
// one will reject with a clear error.
func lowerTerm(ctx *lowerCtx, term *firewall.Term) ([]expr.Any, error) {
	var exprs []expr.Any

	for _, m := range term.Matches {
		me, err := lowerMatch(ctx, m)
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

func lowerMatch(ctx *lowerCtx, m firewall.Match) ([]expr.Any, error) {
	switch v := m.(type) {
	case firewall.MatchSourceAddress:
		return lowerAddrMatch(v.Prefix, true)
	case firewall.MatchDestinationAddress:
		return lowerAddrMatch(v.Prefix, false)
	case firewall.MatchSourcePort:
		return lowerPortMatch(ctx, v.Ranges, 0) // src port offset in transport header
	case firewall.MatchDestinationPort:
		return lowerPortMatch(ctx, v.Ranges, 2) // dst port offset
	case firewall.MatchProtocol:
		return lowerProtoMatch(v.Protocol)
	case firewall.MatchInputInterface:
		return lowerIfaceMatch(expr.MetaKeyIIFNAME, v.Name, v.Wildcard)
	case firewall.MatchOutputInterface:
		return lowerIfaceMatch(expr.MetaKeyOIFNAME, v.Name, v.Wildcard)
	case firewall.MatchConnState:
		return lowerConnStateMatch(v.States)
	case firewall.MatchMark:
		return lowerMarkMatch(v.Value, v.Mask)
	case firewall.MatchConnMark:
		return lowerConnMarkMatch(v.Value, v.Mask)
	case firewall.MatchDSCP:
		return lowerDSCPMatch(v.Value)
	case firewall.MatchICMPType:
		return lowerICMPTypeMatch(v.Type)
	case firewall.MatchICMPv6Type:
		return lowerICMPTypeMatch(v.Type)
	case firewall.MatchInSet:
		return lowerMatchInSet(ctx, v)
	}
	return nil, fmt.Errorf("unsupported match type %T", m)
}

// lowerMatchInSet emits Payload + Lookup for a named-set match. The
// source/destination field identifies which header byte-range to load
// (IPv4 address: 12/16, IPv6: 8/24, TCP/UDP port: 0/2 in transport
// header) and the set's KeyType determines the length. Mismatches
// between the field and the set type (e.g. source-address against an
// inet_service set) reject here rather than silently programming a
// rule that would never match.
func lowerMatchInSet(ctx *lowerCtx, m firewall.MatchInSet) ([]expr.Any, error) {
	if ctx == nil || ctx.sets == nil {
		return nil, fmt.Errorf("match-in-set %q: no set registry in lowering context", m.SetName)
	}
	set, ok := ctx.sets[m.SetName]
	if !ok {
		return nil, fmt.Errorf("match-in-set: unknown set %q (not registered on table)", m.SetName)
	}
	base, offset, length, err := matchInSetPayloadLayout(m, set)
	if err != nil {
		return nil, err
	}
	return []expr.Any{
		&expr.Payload{DestRegister: 1, Base: base, Offset: offset, Len: length},
		&expr.Lookup{SourceRegister: 1, SetID: set.ID, SetName: set.Name},
	}, nil
}

// matchInSetPayloadLayout picks the header base, byte offset, and byte
// length for a MatchInSet according to the field and the set's data
// type. Returns an error if the field and the set type disagree (e.g.
// source-address against an inet_service set).
func matchInSetPayloadLayout(m firewall.MatchInSet, set *nftables.Set) (expr.PayloadBase, uint32, uint32, error) {
	isAddrField := m.MatchField == firewall.SetFieldSourceAddr || m.MatchField == firewall.SetFieldDestAddr
	isPortField := m.MatchField == firewall.SetFieldSourcePort || m.MatchField == firewall.SetFieldDestPort
	if !isAddrField && !isPortField {
		return 0, 0, 0, fmt.Errorf("match-in-set %q: unknown field %d", m.SetName, m.MatchField)
	}
	if isAddrField {
		isSource := m.MatchField == firewall.SetFieldSourceAddr
		// Compare via SetDatatype.Name (public field) rather than
		// struct equality. SetDatatype carries an unexported
		// `nftMagic` field; direct `==` works today because both
		// sides come from the google/nftables package vars, but a
		// future vendor update adding a mutable field would break
		// the comparison silently.
		switch set.KeyType.Name {
		case nftables.TypeIPAddr.Name:
			if isSource {
				return expr.PayloadBaseNetworkHeader, 12, 4, nil
			}
			return expr.PayloadBaseNetworkHeader, 16, 4, nil
		case nftables.TypeIP6Addr.Name:
			if isSource {
				return expr.PayloadBaseNetworkHeader, 8, 16, nil
			}
			return expr.PayloadBaseNetworkHeader, 24, 16, nil
		}
		return 0, 0, 0, fmt.Errorf("match-in-set %q: address field requires ipv4_addr or ipv6_addr set type, got %q",
			m.SetName, set.KeyType.Name)
	}
	// isPortField
	if set.KeyType.Name != nftables.TypeInetService.Name {
		return 0, 0, 0, fmt.Errorf("match-in-set %q: port field requires inet_service set type, got %q",
			m.SetName, set.KeyType.Name)
	}
	if m.MatchField == firewall.SetFieldSourcePort {
		return expr.PayloadBaseTransportHeader, 0, 2, nil
	}
	return expr.PayloadBaseTransportHeader, 2, 2, nil
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
		return lowerMasquerade(v)
	case firewall.Notrack:
		return []expr.Any{&expr.Notrack{}}, nil
	case firewall.Counter:
		return lowerCounter(v)
	case firewall.Log:
		return lowerLog(v)
	case firewall.Limit:
		return lowerLimit(v)
	case firewall.SetMark:
		return lowerSetMark(v.Value, v.Mask)
	case firewall.SetConnMark:
		return lowerSetConnMark(v.Value, v.Mask)
	case firewall.SetDSCP:
		return lowerSetDSCP(v.Value)
	case firewall.FlowOffload:
		return []expr.Any{&expr.FlowOffload{Name: v.FlowtableName}}, nil
	case firewall.SNAT:
		return lowerSNAT(v)
	case firewall.DNAT:
		return lowerDNAT(v)
	case firewall.Redirect:
		return lowerRedirect(v)
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

// lowerPortMatch translates a []PortRange into nftables expressions:
//   - single-port range (Lo==Hi): Payload + Cmp(Eq)
//   - single contiguous range (Lo<Hi): Payload + Cmp(Gte) + Cmp(Lte)
//   - multiple entries: anonymous interval set registered on ctx.conn, matched
//     via Lookup against the loaded transport-header port value
//
// A ctx is required only for the multi-entry case; the single-entry path does
// not touch ctx. Callers that cannot supply a ctx receive a clear rejection
// from the multi-entry path rather than silently truncating to the first entry.
func lowerPortMatch(ctx *lowerCtx, ranges []firewall.PortRange, offset uint32) ([]expr.Any, error) {
	if len(ranges) == 0 {
		return nil, fmt.Errorf("empty port ranges")
	}
	if len(ranges) == 1 {
		r := ranges[0]
		portBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(portBytes, r.Lo)
		if r.Hi == r.Lo {
			return []expr.Any{
				&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseTransportHeader, Offset: offset, Len: 2},
				&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: portBytes},
			}, nil
		}
		portEndBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(portEndBytes, r.Hi)
		return []expr.Any{
			&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseTransportHeader, Offset: offset, Len: 2},
			&expr.Cmp{Op: expr.CmpOpGte, Register: 1, Data: portBytes},
			&expr.Cmp{Op: expr.CmpOpLte, Register: 1, Data: portEndBytes},
		}, nil
	}

	if ctx == nil || ctx.conn == nil || ctx.table == nil {
		return nil, fmt.Errorf("multi-range port match requires a table context")
	}
	set := &nftables.Set{
		Table:     ctx.table,
		Anonymous: true,
		Constant:  true,
		Interval:  true,
		KeyType:   nftables.TypeInetService,
	}
	elements := make([]nftables.SetElement, 0, len(ranges)*2)
	for _, r := range ranges {
		loBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(loBytes, r.Lo)
		// Interval closing element is Hi+1 with IntervalEnd=true so the kernel
		// stores the half-open range [Lo, Hi+1). Using Hi+1 avoids the off-by-
		// one that plain Hi produces with Interval sets.
		endBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(endBytes, r.Hi+1)
		elements = append(elements,
			nftables.SetElement{Key: loBytes},
			nftables.SetElement{Key: endBytes, IntervalEnd: true},
		)
	}
	if err := ctx.conn.AddSet(set, elements); err != nil {
		return nil, fmt.Errorf("add anonymous port set: %w", err)
	}
	return []expr.Any{
		&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseTransportHeader, Offset: offset, Len: 2},
		&expr.Lookup{SourceRegister: 1, SetID: set.ID, SetName: set.Name},
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

// lowerConnStateMatch builds a ct state bitmask from the abstract ConnState
// bitset. Kernel bits (include/uapi/linux/netfilter/nf_tables.h, mirrored in
// vendor/github.com/google/nftables/expr/ct.go) are INVALID=1, ESTABLISHED=2,
// RELATED=4, NEW=8, UNTRACKED=64 -- NOT the sequential iota order the abstract
// enum uses. Map each flag explicitly so NEW/INVALID don't swap places.
func lowerConnStateMatch(states firewall.ConnState) ([]expr.Any, error) {
	var mask uint32
	if states&firewall.ConnStateInvalid != 0 {
		mask |= expr.CtStateBitINVALID
	}
	if states&firewall.ConnStateEstablished != 0 {
		mask |= expr.CtStateBitESTABLISHED
	}
	if states&firewall.ConnStateRelated != 0 {
		mask |= expr.CtStateBitRELATED
	}
	if states&firewall.ConnStateNew != 0 {
		mask |= expr.CtStateBitNEW
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

// lowerConnMarkMatch mirrors lowerMarkMatch but reads from the
// conntrack mark rather than the packet mark. Masked compare works
// the same way: load, AND, compare.
func lowerConnMarkMatch(value, mask uint32) ([]expr.Any, error) {
	valBytes := make([]byte, 4)
	binary.NativeEndian.PutUint32(valBytes, value)
	maskBytes := make([]byte, 4)
	binary.NativeEndian.PutUint32(maskBytes, mask)
	return []expr.Any{
		&expr.Ct{Key: expr.CtKeyMARK, Register: 1},
		&expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: 4, Mask: maskBytes, Xor: make([]byte, 4)},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: valBytes},
	}, nil
}

// lowerIfaceMatch emits Meta(IIFNAME|OIFNAME) + Cmp. Exact matches
// compare the full 16-byte IFNAMSIZ-padded name; wildcards compare
// only the prefix bytes so `l2tp*` matches any interface whose name
// starts with `l2tp`. The kernel stores the interface name as a
// NUL-padded C string in the register; a shorter Cmp data slice
// targets only the first len(data) bytes.
func lowerIfaceMatch(key expr.MetaKey, name string, wildcard bool) ([]expr.Any, error) {
	if name == "" {
		return nil, fmt.Errorf("interface name must not be empty")
	}
	var data []byte
	if wildcard {
		data = []byte(name)
	} else {
		data = ifnameBytes(name)
	}
	return []expr.Any{
		&expr.Meta{Key: key, Register: 1},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: data},
	}, nil
}

// lowerICMPTypeMatch compares the first byte of the transport header
// (ICMPv4 or ICMPv6 type) against a single value. ICMPv4 and ICMPv6
// share the same lowering: their type byte sits at transport-header
// offset 0 regardless of protocol. The `ip`/`ip6`/`inet` table family
// is what disambiguates the two at the kernel level.
func lowerICMPTypeMatch(icmpType uint8) ([]expr.Any, error) {
	return []expr.Any{
		&expr.Payload{
			OperationType: expr.PayloadLoad,
			DestRegister:  1,
			Base:          expr.PayloadBaseTransportHeader,
			Offset:        0,
			Len:           1,
		},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{icmpType}},
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

// lowerLimit emits either a packet-rate or byte-rate limiter based on
// the Dimension field populated by parseRateSpec. A zero Dimension is
// rejected: the parser always sets it, so a zero indicates a caller
// that bypassed the parser (and would silently become a packet rate
// without this guard).
func lowerLimit(l firewall.Limit) ([]expr.Any, error) {
	unit, err := lowerLimitUnit(l.Unit)
	if err != nil {
		return nil, err
	}
	var limitType expr.LimitType
	if l.Dimension == firewall.RateDimensionPackets {
		limitType = expr.LimitTypePkts
	} else if l.Dimension == firewall.RateDimensionBytes {
		limitType = expr.LimitTypePktBytes
	} else {
		return nil, fmt.Errorf("limit-rate dimension unset (parseRateSpec bypassed?)")
	}
	return []expr.Any{&expr.Limit{
		Type:  limitType,
		Rate:  l.Rate,
		Unit:  unit,
		Burst: l.Burst,
		Over:  l.Over,
	}}, nil
}

func lowerLimitUnit(unit string) (expr.LimitTime, error) {
	switch unit {
	case "second":
		return expr.LimitTimeSecond, nil
	case "minute":
		return expr.LimitTimeMinute, nil
	case "hour":
		return expr.LimitTimeHour, nil
	case "day":
		return expr.LimitTimeDay, nil
	}
	return 0, fmt.Errorf("unknown limit unit %q (want second|minute|hour|day)", unit)
}

// lowerSetConnMark writes a value into the conntrack mark. The masked
// path mirrors lowerSetMark but targets ct mark: read current, clear
// the masked bits, OR in the new value, write back.
func lowerSetConnMark(value, mask uint32) ([]expr.Any, error) {
	valBytes := make([]byte, 4)
	binary.NativeEndian.PutUint32(valBytes, value)

	if mask == 0xFFFFFFFF {
		return []expr.Any{
			&expr.Immediate{Register: 1, Data: valBytes},
			&expr.Ct{Key: expr.CtKeyMARK, SourceRegister: true, Register: 1},
		}, nil
	}

	invertedMask := make([]byte, 4)
	binary.NativeEndian.PutUint32(invertedMask, ^mask)
	maskBytes := make([]byte, 4)
	binary.NativeEndian.PutUint32(maskBytes, mask)
	return []expr.Any{
		&expr.Ct{Key: expr.CtKeyMARK, Register: 1},
		&expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: 4, Mask: invertedMask, Xor: maskedValue(value, mask)},
		&expr.Ct{Key: expr.CtKeyMARK, SourceRegister: true, Register: 1},
	}, nil
}

// maskedValue returns `value & mask` as 4 little-endian bytes, suitable
// for use as the Xor operand in a bitwise-clear-and-OR step.
func maskedValue(value, mask uint32) []byte {
	out := make([]byte, 4)
	binary.NativeEndian.PutUint32(out, value&mask)
	return out
}

// lowerSetDSCP rewrites the IPv4 TOS byte's top 6 bits (DSCP) while
// preserving the bottom 2 bits (ECN). Read TOS, `(tos & 0x03) ^ (dscp << 2)`,
// write back with IPv4 header checksum recomputed.
//
// IPv6 traffic class straddles bytes 0 and 1 of the network header, so
// this lowering is IPv4-only today. A caller in a `ip6` table will land
// this expression at the same offsets and produce a wrong write; the
// verifier does not currently prevent it. When an IPv6 DSCP pattern is
// needed, add a family parameter here.
func lowerSetDSCP(dscp uint8) ([]expr.Any, error) {
	if dscp > 63 {
		return nil, fmt.Errorf("dscp %d out of range 0..63", dscp)
	}
	return []expr.Any{
		&expr.Payload{
			OperationType: expr.PayloadLoad,
			DestRegister:  1,
			Base:          expr.PayloadBaseNetworkHeader,
			Offset:        1,
			Len:           1,
		},
		&expr.Bitwise{
			SourceRegister: 1,
			DestRegister:   1,
			Len:            1,
			Mask:           []byte{0x03},
			Xor:            []byte{dscp << 2},
		},
		&expr.Payload{
			OperationType:  expr.PayloadWrite,
			SourceRegister: 1,
			Base:           expr.PayloadBaseNetworkHeader,
			Offset:         1,
			Len:            1,
			CsumType:       expr.CsumTypeInet,
			CsumOffset:     10,
		},
	}, nil
}

// lowerRedirect emits the nftables `redir` expression. The operator's
// `redirect to <port>` loads the port into r2 and hands r2 to the
// Redir expression as RegisterProtoMin. Port 0 means redirect without
// port rewrite (rare but valid at nftables level).
func lowerRedirect(r firewall.Redirect) ([]expr.Any, error) {
	if r.Flags != 0 {
		return nil, fmt.Errorf("redirect flags not yet supported (flags=%#x)", r.Flags)
	}
	if r.Port == 0 {
		return []expr.Any{&expr.Redir{}}, nil
	}
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, r.Port)
	return []expr.Any{
		&expr.Immediate{Register: 2, Data: portBytes},
		&expr.Redir{RegisterProtoMin: 2},
	}, nil
}

// lowerSetMark applies a mark value with a mask. Full-mask (0xFFFFFFFF) is a
// direct immediate + meta write. A partial mask uses the nftables idiom
// `(mark & ~mask) ^ (value & mask)` via a single Bitwise: reading into r1
// clears the masked bits and XORs in the pre-masked value in one step, which
// works because the bits being XORed are exactly those that were just cleared.
// The previous implementation produced (value & mask) in r2 but never combined
// it with r1, so masked writes silently zeroed the masked bits.
func lowerSetMark(value, mask uint32) ([]expr.Any, error) {
	valBytes := make([]byte, 4)
	binary.NativeEndian.PutUint32(valBytes, value)

	if mask == 0xFFFFFFFF {
		return []expr.Any{
			&expr.Immediate{Register: 1, Data: valBytes},
			&expr.Meta{Key: expr.MetaKeyMARK, SourceRegister: true, Register: 1},
		}, nil
	}

	invertedMask := make([]byte, 4)
	binary.NativeEndian.PutUint32(invertedMask, ^mask)
	return []expr.Any{
		&expr.Meta{Key: expr.MetaKeyMARK, Register: 1},
		&expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: 4, Mask: invertedMask, Xor: maskedValue(value, mask)},
		&expr.Meta{Key: expr.MetaKeyMARK, SourceRegister: true, Register: 1},
	}, nil
}

// lowerNAT emits the expressions for a source or destination NAT action.
// Register layout: r1 = address (min), r2 = port min (if any), r3 = port
// max (if the operator asked for a port range), r4 = address max (if the
// operator asked for an address range). Flags are rejected rather than
// silently dropped -- the parser does not surface NAT flags today, so a
// non-zero value implies a programmatic caller and we surface the
// omission instead of pretending.
func lowerNAT(addr, addrEnd netip.Addr, port, portEnd uint16, flags uint32, natType expr.NATType) ([]expr.Any, error) {
	if flags != 0 {
		return nil, fmt.Errorf("NAT flags not yet supported (flags=%#x)", flags)
	}
	var exprs []expr.Any
	var family uint32

	addrData, err := natAddrBytes(addr)
	if err != nil {
		return nil, err
	}
	if addr.Is4() {
		family = 2 // AF_INET
	} else {
		family = 10 // AF_INET6
	}

	exprs = append(exprs, &expr.Immediate{Register: 1, Data: addrData})
	nat := &expr.NAT{
		Type:       natType,
		Family:     family,
		RegAddrMin: 1,
	}
	if addrEnd.IsValid() {
		if addr.Is4() != addrEnd.Is4() {
			return nil, fmt.Errorf("NAT address range: mixed IPv4/IPv6 bounds")
		}
		if addrEnd.Less(addr) {
			return nil, fmt.Errorf("NAT address range %s-%s is inverted", addr, addrEnd)
		}
		endData, err := natAddrBytes(addrEnd)
		if err != nil {
			return nil, err
		}
		exprs = append(exprs, &expr.Immediate{Register: 4, Data: endData})
		nat.RegAddrMax = 4
	}
	if port != 0 {
		portBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(portBytes, port)
		exprs = append(exprs, &expr.Immediate{Register: 2, Data: portBytes})
		nat.RegProtoMin = 2
	}
	if portEnd != 0 {
		if port == 0 {
			return nil, fmt.Errorf("NAT port range requires a lower bound")
		}
		if portEnd < port {
			return nil, fmt.Errorf("NAT port range %d-%d is inverted", port, portEnd)
		}
		portEndBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(portEndBytes, portEnd)
		exprs = append(exprs, &expr.Immediate{Register: 3, Data: portEndBytes})
		nat.RegProtoMax = 3
	}
	exprs = append(exprs, nat)
	return exprs, nil
}

// natAddrBytes returns the raw 4 (IPv4) or 16 (IPv6) network-byte-order
// bytes for an address. Invalid addresses reject so a caller that
// forgot to populate Addr cannot emit a zero-valued immediate.
func natAddrBytes(a netip.Addr) ([]byte, error) {
	if !a.IsValid() {
		return nil, fmt.Errorf("invalid NAT address")
	}
	if a.Is4() {
		b := a.As4()
		return b[:], nil
	}
	b := a.As16()
	return b[:], nil
}

func lowerSNAT(s firewall.SNAT) ([]expr.Any, error) {
	return lowerNAT(s.Address, s.AddressEnd, s.Port, s.PortEnd, s.Flags, expr.NATTypeSourceNAT)
}

func lowerDNAT(d firewall.DNAT) ([]expr.Any, error) {
	return lowerNAT(d.Address, d.AddressEnd, d.Port, d.PortEnd, d.Flags, expr.NATTypeDestNAT)
}

// lowerMasquerade rejects fields the backend does not program rather
// than silently dropping them. The parser emits Masquerade{} today, so
// any non-zero field implies a programmatic caller and surfacing the
// gap is preferable to a silent approximation.
func lowerMasquerade(m firewall.Masquerade) ([]expr.Any, error) {
	if m.Port != 0 || m.PortEnd != 0 {
		return nil, fmt.Errorf("masquerade with port mapping not yet supported (port=%d-%d)", m.Port, m.PortEnd)
	}
	if m.Flags != 0 {
		return nil, fmt.Errorf("masquerade flags not yet supported (flags=%#x)", m.Flags)
	}
	return []expr.Any{&expr.Masq{}}, nil
}

// lowerCounter rejects named counters -- nftables named counters live as
// a separate table-scoped Object plus an Objref expression, not the
// anonymous Counter we emit. Implementing named counters is follow-up
// work; rejecting here stops us silently turning a named counter into
// an anonymous one.
func lowerCounter(c firewall.Counter) ([]expr.Any, error) {
	if c.Name != "" {
		return nil, fmt.Errorf("named counter %q not yet supported; omit the name for an anonymous counter", c.Name)
	}
	return []expr.Any{&expr.Counter{}}, nil
}

// lowerLog translates every log field the operator set. google/nftables
// gates each NFTA_LOG_* attribute on a Key bit; without the bit the
// attribute is never serialised, which is why the previous Key=0
// implementation silently dropped the prefix along with everything
// else. Level / Group / Snaplen are pointer-valued in the model so
// `level 0` (emerg) is distinguishable from "operator did not set
// level": nil means "kernel default", *p is the requested value.
func lowerLog(l firewall.Log) ([]expr.Any, error) {
	e := &expr.Log{}
	if l.Prefix != "" {
		e.Data = []byte(l.Prefix)
		e.Key |= 1 << unix.NFTA_LOG_PREFIX
	}
	if l.Level != nil {
		e.Level = expr.LogLevel(*l.Level)
		e.Key |= 1 << unix.NFTA_LOG_LEVEL
	}
	if l.Group != nil {
		e.Group = *l.Group
		e.Key |= 1 << unix.NFTA_LOG_GROUP
	}
	if l.Snaplen != nil {
		e.Snaplen = *l.Snaplen
		e.Key |= 1 << unix.NFTA_LOG_SNAPLEN
	}
	return []expr.Any{e}, nil
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

// decodeSetElementKey is the inverse of encodeSetElementKey. It
// converts a kernel-side key blob back to the operator's string form
// so readback can surface set members in the format they were
// originally written. Unknown or malformed keys surface as a hex
// fallback rather than an error: CLI readback must not fail the whole
// listing because one set has a surprise.
func decodeSetElementKey(st firewall.SetType, key []byte) string {
	switch st {
	case firewall.SetTypeIPv4:
		if len(key) == 4 {
			addr, _ := netip.AddrFromSlice(key)
			return addr.String()
		}
	case firewall.SetTypeIPv6:
		if len(key) == 16 {
			addr, _ := netip.AddrFromSlice(key)
			return addr.String()
		}
	case firewall.SetTypeInetService:
		if len(key) == 2 {
			port := binary.BigEndian.Uint16(key)
			return strconv.FormatUint(uint64(port), 10)
		}
	case firewall.SetTypeMark:
		if len(key) == 4 {
			mark := binary.NativeEndian.Uint32(key)
			return fmt.Sprintf("%#x", mark)
		}
	case firewall.SetTypeEther:
		if len(key) == 6 {
			return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", key[0], key[1], key[2], key[3], key[4], key[5])
		}
	case firewall.SetTypeIfname:
		// ifname is a NUL-padded C string. Trim at the first NUL so
		// the display matches the name the operator typed.
		i := 0
		for i < len(key) && key[i] != 0 {
			i++
		}
		return string(key[:i])
	}
	return fmt.Sprintf("%#x", key)
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
