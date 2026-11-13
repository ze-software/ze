// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- exact directional scope.
// Related: state_scope.go -- immutable canonical records; xfrm_linux.go -- boundaries.
// Related: xfrm_state_scope_abi_linux.go -- the private kernel ABI: wire form and receipt.

//go:build linux

package dataplane

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/vishvananda/netlink"

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
	if caps.ABI != xfrmZEABI {
		return xfrmStateScopeRefusal(fmt.Errorf("kernel scope ABI %#x differs from %#x", caps.ABI, xfrmZEABI))
	}
	if caps.MaxSelectors != saStateScopeMaxSelectors {
		return xfrmStateScopeRefusal(fmt.Errorf("kernel scope selector limit %d differs from %d", caps.MaxSelectors, saStateScopeMaxSelectors))
	}
	const required = xfrmZECapStateScope | xfrmZECapICMPQuote |
		xfrmZECapFragmentContext | xfrmZECapFragmentGuard
	const known = required | xfrmZECapICMPFinal
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
	attr, err := p.Scope.xfrmStateScopeAttr()
	if err != nil {
		return err
	}
	state.ExtraAttrs = append(state.ExtraAttrs, attr)
	if p.Mode == ModeTransport {
		selector := p.Scope.selectors[0].params()
		state.Selector = &netlink.XfrmPolicy{Src: selector.Src, Dst: selector.Dst,
			Proto: netlink.Proto(selector.UpperProto), Ifindex: selector.IfIndex,
			SrcPort: int(selector.SrcPort.Port), SrcPortMask: selector.SrcPort.Mask,
			DstPort: int(selector.DstPort.Port), DstPortMask: selector.DstPort.Mask}
	}
	return nil
}
