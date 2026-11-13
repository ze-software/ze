// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- exact directional scope.
// Related: xfrm_state_scope_linux.go -- scope projection and capability refusal.
// Related: xfrm_forward_default_linux.go -- the GETDEFAULT answer that carries the receipt.

//go:build linux

package dataplane

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

// Ze's private XFRM netlink attributes. Ze's runtime kernel declares them in
// include/uapi/linux/xfrm.h (gokrazy/kernel/patches/0003-xfrm-state-scope.patch),
// after Linux 7.3's last upstream attribute, XFRMA_IPTFS_PKT_SIZE (41). The
// vendored netlink library knows neither: a state request carries the scope in
// XfrmState.ExtraAttrs, and a state dump returns it there.
const (
	// xfrmaStateScope carries struct xfrm_user_state_scope on a state request
	// and on each scoped state of a dump.
	xfrmaStateScope = 42
	// xfrmaZECaps carries the read-only struct xfrm_user_ze_caps on a
	// GETDEFAULT answer.
	xfrmaZECaps = 43
)

// The vendored library returns a state attribute in ExtraAttrs only when its
// type is above nl.XFRMA_MAX, so xfrmaStateScope must stay above it or a dump
// would drop the scope with no error. This array length goes negative, and the
// build fails, when a re-vendored library raises XFRMA_MAX to 42 or more. Then
// the kernel patch and Ze must move the private numbers above it, or the
// library must return the attribute another way. xfrmaZECaps needs no such
// check: parseXFRMZECapabilities walks the GETDEFAULT answer itself.
var _ [xfrmaStateScope - nl.XFRMA_MAX - 1]struct{}

// struct xfrm_user_state_scope, the value of xfrmaStateScope, in native byte
// order. Each record is the kernel's struct xfrm_selector (nl.SizeofXfrmSelector,
// 56 bytes); the records form an ordered OR-set, never a covering selector.
//
//	0         4         8                                  8+56*count
//	+---------+---------+---------------------------------+
//	|  flags  |  count  |  struct xfrm_selector[count]    |
//	+---------+---------+---------------------------------+
const (
	xfrmStateScopeHeaderLen = 8
	// xfrmStateScopeNonInitial is flag bit 0: the negotiated permission for
	// noninitial fragments. Every other flag bit is refused.
	xfrmStateScopeNonInitial uint32 = 1
)

// xfrmStateScopeAttr returns the xfrmaStateScope attribute that carries s on a
// state request. The payload belongs to this one control-plane request, so it is
// allocated here rather than taken from a pool; the library sends it unchanged.
// RFC 4301 Section 5.2: "Then match the packet against the inbound selectors identified by the SAD entry to verify that the received packet is appropriate for the SA via which it was received."
// The kernel performs that match against the records written here.
func (s *SAStateScope) xfrmStateScopeAttr() (netlink.XfrmAttr, error) {
	selectors := make([]nl.XfrmSelector, len(s.selectors))
	for i := range s.selectors {
		selectors[i] = s.selectors[i].xfrmSelector()
	}
	var flags uint32
	if s.noninitial {
		flags = xfrmStateScopeNonInitial
	}
	payload := make([]byte, xfrmStateScopeHeaderLen+len(selectors)*nl.SizeofXfrmSelector)
	if _, err := writeXFRMStateScope(payload, 0, flags, selectors); err != nil {
		return netlink.XfrmAttr{}, err
	}
	return netlink.XfrmAttr{Type: xfrmaStateScope, Value: payload}, nil
}

// writeXFRMStateScope writes struct xfrm_user_state_scope into buf at off and
// returns the byte count. It checks the flags, the record count, every record
// and the buffer before it writes, so a refusal leaves buf unchanged. The record
// check is the one parseXFRMStateScope applies, so both boundaries agree.
func writeXFRMStateScope(buf []byte, off int, flags uint32, selectors []nl.XfrmSelector) (int, error) {
	if flags & ^xfrmStateScopeNonInitial != 0 {
		return 0, errors.New("XFRM state scope contains unknown flags")
	}
	if err := saStateScopeCount(len(selectors)); err != nil {
		return 0, err
	}
	for i := range selectors {
		if _, err := saScopeSelectorFromKernel(&selectors[i]); err != nil {
			return 0, fmt.Errorf("XFRM state scope selector %d: %w", i, err)
		}
	}
	if off < 0 {
		return 0, errors.New("XFRM state scope output offset is negative")
	}
	if off > len(buf) {
		return 0, errors.New("XFRM state scope output offset exceeds the buffer")
	}
	size := xfrmStateScopeHeaderLen + len(selectors)*nl.SizeofXfrmSelector
	if len(buf)-off < size {
		return 0, errors.New("XFRM state scope output buffer is too short")
	}
	binary.NativeEndian.PutUint32(buf[off:], flags)
	binary.NativeEndian.PutUint32(buf[off+4:], uint32(len(selectors)))
	for i := range selectors {
		copy(buf[off+xfrmStateScopeHeaderLen+i*nl.SizeofXfrmSelector:], selectors[i].Serialize())
	}
	return size, nil
}

// xfrmStateScopeFromAttrs returns the scope a dumped state carries. A state with
// no xfrmaStateScope is a legacy state and answers nil; the caller records that
// it read the state (SAInfo.StateScopeRead). A second scope attribute, or one
// that does not parse, fails the whole dump rather than becoming nil.
func xfrmStateScopeFromAttrs(attrs []netlink.XfrmAttr) (*SAStateScope, error) {
	var scope *SAStateScope
	for i := range attrs {
		if attrs[i].Type != xfrmaStateScope {
			continue
		}
		if scope != nil {
			return nil, errors.New("kernel state contains duplicate scope attributes")
		}
		parsed, err := parseXFRMStateScope(attrs[i].Value)
		if err != nil {
			return nil, err
		}
		scope = parsed
	}
	return scope, nil
}

// parseXFRMStateScope reads struct xfrm_user_state_scope from a state dump. It
// bounds the count before it computes a length, requires the exact length,
// validates every record and returns a scope that owns its records.
func parseXFRMStateScope(payload []byte) (*SAStateScope, error) {
	if len(payload) < xfrmStateScopeHeaderLen {
		return nil, errors.New("kernel state scope header is truncated")
	}
	flags := binary.NativeEndian.Uint32(payload)
	if flags & ^xfrmStateScopeNonInitial != 0 {
		return nil, errors.New("kernel state scope contains unknown flags")
	}
	count := binary.NativeEndian.Uint32(payload[4:])
	if count > saStateScopeMaxSelectors {
		return nil, fmt.Errorf("kernel state scope has %d selectors, exceeding the limit of %d", count, saStateScopeMaxSelectors)
	}
	if err := saStateScopeCount(int(count)); err != nil {
		return nil, err
	}
	if len(payload) != xfrmStateScopeHeaderLen+int(count)*nl.SizeofXfrmSelector {
		return nil, errors.New("kernel state scope length does not match its selector count")
	}
	owned := make([]saScopeSelector, count)
	for i := range owned {
		var raw nl.XfrmSelector
		start := xfrmStateScopeHeaderLen + i*nl.SizeofXfrmSelector
		copy(raw.Serialize(), payload[start:start+nl.SizeofXfrmSelector])
		selector, err := saScopeSelectorFromKernel(&raw)
		if err != nil {
			return nil, fmt.Errorf("kernel state scope selector %d: %w", i, err)
		}
		owned[i] = selector
	}
	return &SAStateScope{selectors: owned, noninitial: flags == xfrmStateScopeNonInitial}, nil
}

// xfrmSelector projects one canonical record onto the kernel's struct xfrm_selector.
func (s saScopeSelector) xfrmSelector() nl.XfrmSelector {
	selector := nl.XfrmSelector{Proto: s.proto, PrefixlenS: uint8(s.src.Bits()),
		PrefixlenD: uint8(s.dst.Bits()), Sport: nl.Swap16(s.srcPort.Port),
		SportMask: nl.Swap16(s.srcPort.Mask), Dport: nl.Swap16(s.dstPort.Port),
		DportMask: nl.Swap16(s.dstPort.Mask), Ifindex: s.ifindex}
	if s.src.Addr().Is4() {
		selector.Family = unix.AF_INET
		src, dst := s.src.Addr().As4(), s.dst.Addr().As4()
		copy(selector.Saddr[:], src[:])
		copy(selector.Daddr[:], dst[:])
		return selector
	}
	selector.Family = unix.AF_INET6
	selector.Saddr = nl.XfrmAddress(s.src.Addr().As16())
	selector.Daddr = nl.XfrmAddress(s.dst.Addr().As16())
	return selector
}

// saScopeSelectorFromKernel validates one raw struct xfrm_selector with the
// canonical record rules. The writer and the parser both call it.
func saScopeSelectorFromKernel(raw *nl.XfrmSelector) (saScopeSelector, error) {
	if raw.User != 0 {
		return saScopeSelector{}, errors.New("user selector is unsupported")
	}
	if raw.Pad != [3]byte{} {
		return saScopeSelector{}, errors.New("selector padding is nonzero")
	}
	src, err := saScopeAddressFromKernel(raw.Saddr, raw.Family)
	if err != nil {
		return saScopeSelector{}, fmt.Errorf("source address: %w", err)
	}
	dst, err := saScopeAddressFromKernel(raw.Daddr, raw.Family)
	if err != nil {
		return saScopeSelector{}, fmt.Errorf("destination address: %w", err)
	}
	selector := saScopeSelector{
		src: netip.PrefixFrom(src, int(raw.PrefixlenS)), dst: netip.PrefixFrom(dst, int(raw.PrefixlenD)),
		proto: raw.Proto, ifindex: raw.Ifindex,
		srcPort: PortMatch{Port: nl.Swap16(raw.Sport), Mask: nl.Swap16(raw.SportMask)},
		dstPort: PortMatch{Port: nl.Swap16(raw.Dport), Mask: nl.Swap16(raw.DportMask)},
	}
	if err := selector.validate(); err != nil {
		return saScopeSelector{}, err
	}
	return selector, nil
}

func saScopeAddressFromKernel(address nl.XfrmAddress, family uint16) (netip.Addr, error) {
	switch family {
	case unix.AF_INET:
		if [12]byte(address[4:]) != [12]byte{} {
			return netip.Addr{}, errors.New("IPv4 address has a nonzero union tail")
		}
		return netip.AddrFrom4([4]byte(address[:4])), nil
	case unix.AF_INET6:
		return netip.AddrFrom16([16]byte(address)), nil
	default:
		return netip.Addr{}, fmt.Errorf("unknown address family %d", family)
	}
}

// struct xfrm_user_ze_caps, the value of xfrmaZECaps, in native byte order. The
// kernel appends it to the GETDEFAULT answer after struct xfrm_userpolicy_default
// (three disposition bytes) and one byte of alignment.
//
//	0            4                 8              12      16
//	+------------+-----------------+--------------+-------+
//	|  features  |  max_selectors  |  guard_refs  |  abi  |
//	+------------+-----------------+--------------+-------+
const (
	xfrmZECapsLen = 16
	// xfrmZEABI is the only abi word this dataplane speaks.
	xfrmZEABI uint32 = 0x5a450001
)

// xfrmZEFeature carries individual enforcement features reported by GETDEFAULT.
type xfrmZEFeature uint32

const (
	xfrmZECapStateScope xfrmZEFeature = 1 << iota
	xfrmZECapICMPQuote
	xfrmZECapFragmentContext
	xfrmZECapFragmentGuard
	xfrmZECapICMPFinal
)

// Has reports whether f carries every bit of required. No bits is never carried.
func (f xfrmZEFeature) Has(required xfrmZEFeature) bool {
	return required != 0 && f&required == required
}

// xfrmZECapabilities is the read-only capability receipt of Ze's kernel ABI.
type xfrmZECapabilities struct {
	Features     xfrmZEFeature
	MaxSelectors uint32
	GuardRefs    uint32
	ABI          uint32
}

// errXFRMZECapabilitiesAbsent means GETDEFAULT answered without Ze's receipt. An
// ignored attribute echoed by an old kernel does not establish enforcement.
var errXFRMZECapabilitiesAbsent = errors.New("XFRM GETDEFAULT returned no Ze capability receipt")

// readXFRMZECapabilities reads the receipt from a fresh GETDEFAULT answer of the
// live namespace. It never caches enforcement.
func readXFRMZECapabilities() (*xfrmZECapabilities, error) {
	payload, err := xfrmGetDefaultAnswer()
	if err != nil {
		return nil, err
	}
	return parseXFRMZECapabilities(payload)
}

// parseXFRMZECapabilities reads the receipt from a GETDEFAULT answer. An absent,
// duplicate, foreign or malformed receipt establishes no enforcement.
func parseXFRMZECapabilities(payload []byte) (*xfrmZECapabilities, error) {
	if len(payload) < 3 {
		return nil, errors.New("XFRM GETDEFAULT reply is truncated")
	}
	if len(payload) < 4 {
		return nil, errXFRMZECapabilitiesAbsent
	}
	attrs, err := nl.ParseRouteAttr(payload[4:])
	if err != nil {
		return nil, fmt.Errorf("XFRM GETDEFAULT attributes: %w", err)
	}
	var caps *xfrmZECapabilities
	for _, attr := range attrs {
		if attr.Attr.Type != xfrmaZECaps {
			continue
		}
		if caps != nil {
			return nil, errors.New("XFRM GETDEFAULT has duplicate capability receipts")
		}
		if len(attr.Value) != xfrmZECapsLen {
			return nil, errors.New("XFRM capability receipt has an invalid length")
		}
		caps = &xfrmZECapabilities{Features: xfrmZEFeature(binary.NativeEndian.Uint32(attr.Value)),
			MaxSelectors: binary.NativeEndian.Uint32(attr.Value[4:]),
			GuardRefs:    binary.NativeEndian.Uint32(attr.Value[8:]),
			ABI:          binary.NativeEndian.Uint32(attr.Value[12:])}
		if caps.ABI != xfrmZEABI {
			return nil, fmt.Errorf("XFRM capability ABI %#x differs from %#x", caps.ABI, xfrmZEABI)
		}
		if caps.MaxSelectors != saStateScopeMaxSelectors {
			return nil, fmt.Errorf("XFRM selector limit %d differs from %d", caps.MaxSelectors, saStateScopeMaxSelectors)
		}
		const known = xfrmZECapStateScope | xfrmZECapICMPQuote | xfrmZECapFragmentContext |
			xfrmZECapFragmentGuard | xfrmZECapICMPFinal
		if caps.Features & ^known != 0 {
			return nil, errors.New("XFRM capability receipt contains unknown feature bits")
		}
	}
	if caps == nil {
		return nil, errXFRMZECapabilitiesAbsent
	}
	return caps, nil
}
