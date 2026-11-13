// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- exact directional scope.
// Related: state_scope.go -- immutable canonical records; xfrm_linux.go -- boundaries.
// Upstream: https://github.com/vishvananda/netlink, v1.3.1, Ze's XFRM scope extension.

//go:build linux

package dataplane

import (
	"errors"
	"fmt"
	"net/netip"
	"runtime"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/opererr"
)

// checkXFRMStateScopeCapabilities reads the live namespace before InstallSA can
// publish any state. Every explicit scope needs all four features, independently
// of whether this particular SA permits noninitial fragments.
func checkXFRMStateScopeCapabilities() error {
	caps, err := xfrmScopeCapsRead()
	if err != nil {
		return xfrmStateScopeRefusal(err)
	}
	if caps == nil {
		return xfrmStateScopeRefusal(errors.New("the kernel returned no capability receipt"))
	}
	if caps.ABI != netlink.XfrmZEABI {
		return xfrmStateScopeRefusal(fmt.Errorf("kernel scope ABI %#x differs from %#x", caps.ABI, netlink.XfrmZEABI))
	}
	if caps.MaxSelectors != saStateScopeMaxSelectors {
		return xfrmStateScopeRefusal(fmt.Errorf("kernel scope selector limit %d differs from %d", caps.MaxSelectors, saStateScopeMaxSelectors))
	}
	const required = netlink.XfrmZECapStateScope | netlink.XfrmZECapICMPQuote |
		netlink.XfrmZECapFragmentContext | netlink.XfrmZECapFragmentGuard
	const known = required | netlink.XfrmZECapICMPFinal
	if caps.Features & ^known != 0 {
		return xfrmStateScopeRefusal(fmt.Errorf("kernel scope capabilities contain unknown features %#x", caps.Features & ^known))
	}
	if !caps.Features.Has(required) {
		return xfrmStateScopeRefusal(fmt.Errorf("kernel scope features %#x do not provide all required features %#x", caps.Features, required))
	}
	return nil
}

func xfrmStateScopeRefusal(cause error) error {
	// Keep the exact native command contract of RuntimeKernelBuild in
	// internal/le/build/hostdriver/hostdriver.go. Shipped dataplane code MUST NOT
	// import le's build tooling to format the repair.
	fix := "  ./le build host-driver\n  ./ze-host appliance kernel --target runtime --arch " + runtime.GOARCH
	return opererr.Wrap(errors.Join(ErrSAOptionRefused, cause),
		"Ze cannot verify kernel support for the IPsec SA's complete traffic selector scope",
		"the SA needs verified state, ICMP quote and fragment checks before any state can be installed; rebuild and boot Ze's native runtime kernel",
		fix)
}

// xfrmStateScopeFromParams projects only owned records. For transport the kernel
// requires the legacy selector to be byte-identical to the single scope record.
// A tunnel's complete set MUST NOT be reduced to a legacy covering selector.
func xfrmStateScopeFromParams(p SAParams, state *netlink.XfrmState) error {
	if p.Scope == nil {
		// refuse-by-default: guards SAParams -- nil retains the legacy Sel contract.
		return nil
	}
	if err := saStateScopeCount(len(p.Scope.selectors)); err != nil {
		return err
	}
	if p.Mode == ModeTransport {
		if len(p.Scope.selectors) != 1 {
			return fmt.Errorf("%w: a transport SA needs exactly one state scope selector", ErrSAOptionRefused)
		}
	}
	if p.Sel != nil {
		if len(p.Scope.selectors) != 1 {
			return fmt.Errorf("%w: a legacy SA selector conflicts with a multiple-record state scope", ErrSAOptionRefused)
		}
		legacy, err := saScopeSelectorFromParams(p.Sel)
		if err != nil {
			return fmt.Errorf("%w: legacy SA selector: %w", ErrSAOptionRefused, err)
		}
		if legacy != p.Scope.selectors[0] {
			return fmt.Errorf("%w: the legacy SA selector differs from its state scope", ErrSAOptionRefused)
		}
	}
	scope := &netlink.XfrmStateScope{Selectors: make([]nl.XfrmSelector, len(p.Scope.selectors))}
	if p.Scope.noninitial {
		scope.Flags = netlink.XfrmStateScopeNonInitial
	}
	for i := range p.Scope.selectors {
		scope.Selectors[i] = p.Scope.selectors[i].xfrmSelector()
	}
	state.StateScope = scope
	if p.Mode == ModeTransport {
		selector := p.Scope.selectors[0].params()
		state.Selector = &netlink.XfrmPolicy{Src: selector.Src, Dst: selector.Dst,
			Proto: netlink.Proto(selector.UpperProto), Ifindex: selector.IfIndex,
			SrcPort: int(selector.SrcPort.Port), SrcPortMask: selector.SrcPort.Mask,
			DstPort: int(selector.DstPort.Port), DstPortMask: selector.DstPort.Mask}
	}
	return nil
}

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

// saStateScopeFromKernel validates the mutable DTO again before publishing an
// owned scope. Invalid data MUST fail the whole dump instead of becoming nil.
func saStateScopeFromKernel(scope *netlink.XfrmStateScope) (*SAStateScope, error) {
	if scope.Flags & ^netlink.XfrmStateScopeNonInitial != 0 {
		return nil, errors.New("kernel state scope contains unknown flags")
	}
	if err := saStateScopeCount(len(scope.Selectors)); err != nil {
		return nil, err
	}
	owned := make([]saScopeSelector, len(scope.Selectors))
	for i := range scope.Selectors {
		selector, err := saScopeSelectorFromKernel(&scope.Selectors[i])
		if err != nil {
			return nil, fmt.Errorf("kernel state scope selector %d: %w", i, err)
		}
		owned[i] = selector
	}
	return &SAStateScope{selectors: owned, noninitial: scope.Flags == netlink.XfrmStateScopeNonInitial}, nil
}

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
