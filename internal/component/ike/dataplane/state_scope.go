// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- immutable state scope.
// Related: dataplane.go -- SAParams.Scope and SAInfo.Scope.

package dataplane

import (
	"errors"
	"fmt"
	"math"
	"net"
	"net/netip"
)

const saStateScopeMaxSelectors = 64

// SAStateScope owns a complete directional selector set. Its zero value and nil
// are uninitialized. Installers MUST refuse an explicit uninitialized scope.
// Construction owns the records, and accessors never expose their storage.
// A successfully constructed scope is immutable and safe for concurrent use.
type SAStateScope struct {
	selectors  []saScopeSelector
	noninitial bool
}

type saScopeSelector struct {
	src, dst         netip.Prefix
	proto            uint8
	srcPort, dstPort PortMatch
	ifindex          int32
}

// NewSAStateScope validates and owns canonical records after policy expansion.
// Callers MUST pass every expanded directional record before installing any SA.
// RFC 4301 Section 4.4.2: "For each of the selectors defined in Section 4.4.1.1,
// the entry for an inbound SA in the SAD MUST be initially populated with the value
// or values negotiated at the time the SA was created."
func NewSAStateScope(selectors []SASelector, noninitial bool) (*SAStateScope, error) {
	if err := saStateScopeCount(len(selectors)); err != nil {
		return nil, err
	}
	owned := make([]saScopeSelector, len(selectors))
	for i := range selectors {
		selector, err := saScopeSelectorFromParams(&selectors[i])
		if err != nil {
			return nil, fmt.Errorf("%w: state scope selector %d: %w", ErrSAOptionRefused, i, err)
		}
		owned[i] = selector
	}
	return &SAStateScope{selectors: owned, noninitial: noninitial}, nil
}

// Selectors returns an independent copy of the complete ordered selector set.
// Nil and uninitialized scopes return no records, which an installer MUST refuse.
func (s *SAStateScope) Selectors() []SASelector {
	if s == nil {
		// refuse-by-default: guards SAParams -- nil is the absent optional scope.
		return nil
	}
	selectors := make([]SASelector, len(s.selectors))
	for i := range s.selectors {
		selectors[i] = s.selectors[i].params()
	}
	return selectors
}

// NonInitialFragments reports the negotiated noninitial-fragment permission.
// Nil and uninitialized scopes have no negotiated permission.
func (s *SAStateScope) NonInitialFragments() bool {
	if s == nil {
		return false
	}
	return s.noninitial
}

func saStateScopeCount(count int) error {
	if count == 0 {
		return fmt.Errorf("%w: state scope contains no selectors", ErrSAOptionRefused)
	}
	if count > saStateScopeMaxSelectors {
		return fmt.Errorf("%w: state scope has %d expanded selectors, exceeding the limit of %d", ErrSAOptionRefused, count, saStateScopeMaxSelectors)
	}
	return nil
}

func saScopeSelectorFromParams(s *SASelector) (saScopeSelector, error) {
	src, err := saScopePrefix(s.Src)
	if err != nil {
		return saScopeSelector{}, fmt.Errorf("source prefix: %w", err)
	}
	dst, err := saScopePrefix(s.Dst)
	if err != nil {
		return saScopeSelector{}, fmt.Errorf("destination prefix: %w", err)
	}
	if s.IfIndex < 0 {
		return saScopeSelector{}, errors.New("interface index is negative")
	}
	if s.IfIndex > math.MaxInt32 {
		return saScopeSelector{}, errors.New("interface index exceeds the kernel's signed 32-bit field")
	}
	selector := saScopeSelector{src: src, dst: dst, proto: s.UpperProto,
		srcPort: s.SrcPort, dstPort: s.DstPort, ifindex: int32(s.IfIndex)}
	if err := selector.validate(); err != nil {
		return saScopeSelector{}, err
	}
	return selector, nil
}

func saScopePrefix(prefix *net.IPNet) (netip.Prefix, error) {
	if prefix == nil {
		return netip.Prefix{}, errors.New("prefix is absent")
	}
	bits, width := prefix.Mask.Size()
	if width == 0 {
		return netip.Prefix{}, errors.New("prefix mask is not contiguous")
	}
	addr, ok := netip.AddrFromSlice(prefix.IP)
	if !ok {
		return netip.Prefix{}, errors.New("prefix address is invalid")
	}
	if width == 32 {
		addr = addr.Unmap()
	}
	if addr.BitLen() != width {
		return netip.Prefix{}, errors.New("prefix mask and address families differ")
	}
	canonical := netip.PrefixFrom(addr, bits)
	if canonical != canonical.Masked() {
		return netip.Prefix{}, errors.New("prefix contains host bits")
	}
	return canonical, nil
}

func (s saScopeSelector) validate() error {
	if !s.src.IsValid() {
		return errors.New("source prefix is invalid")
	}
	if !s.dst.IsValid() {
		return errors.New("destination prefix is invalid")
	}
	if s.src != s.src.Masked() {
		return errors.New("source prefix contains host bits")
	}
	if s.dst != s.dst.Masked() {
		return errors.New("destination prefix contains host bits")
	}
	if s.src.Addr().BitLen() != s.dst.Addr().BitLen() {
		return errors.New("selector mixes address families")
	}
	if err := saScopePort(s.srcPort); err != nil {
		return fmt.Errorf("source port: %w", err)
	}
	if err := saScopePort(s.dstPort); err != nil {
		return fmt.Errorf("destination port: %w", err)
	}
	constrained := s.srcPort.Mask != 0 || s.dstPort.Mask != 0
	if constrained {
		// IANA IP protocol numbers. The scope ABI supports these port/type fields.
		switch s.proto {
		case 6, 17, 132, 136, UpperProtoICMP, UpperProtoICMPv6:
		default:
			return fmt.Errorf("protocol %d has no supported constrained port fields", s.proto)
		}
	}
	if s.proto == UpperProtoICMP {
		if !s.src.Addr().Is4() {
			return errors.New("ICMP selector is not IPv4")
		}
	}
	if s.proto == UpperProtoICMPv6 {
		if !s.src.Addr().Is6() {
			return errors.New("ICMPv6 selector is not IPv6")
		}
	}
	if s.ifindex < 0 {
		return errors.New("interface index is negative")
	}
	return nil
}

func saScopePort(port PortMatch) error {
	if port.Port & ^port.Mask != 0 {
		return errors.New("port contains bits outside its mask")
	}
	inverse := ^port.Mask
	if inverse&(inverse+1) != 0 {
		return errors.New("port mask is not contiguous")
	}
	return nil
}

func (s saScopeSelector) params() SASelector {
	return SASelector{
		Src:        &net.IPNet{IP: s.src.Addr().AsSlice(), Mask: net.CIDRMask(s.src.Bits(), s.src.Addr().BitLen())},
		Dst:        &net.IPNet{IP: s.dst.Addr().AsSlice(), Mask: net.CIDRMask(s.dst.Bits(), s.dst.Addr().BitLen())},
		UpperProto: s.proto, SrcPort: s.srcPort, DstPort: s.dstPort, IfIndex: int(s.ifindex),
	}
}
