// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- exact directional scope.
// Related: state_scope.go -- immutable canonical records; xfrm_linux.go -- boundaries.
// Related: xfrm_state_scope_abi_linux.go -- the private kernel ABI: wire form and receipt.

//go:build linux

package dataplane

import (
	"errors"
	"fmt"
	"sync"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/core/opererr"
)

// SAStateScopeSupported declares XFRM's complete directional representation.
// InstallSA still requires a fresh full capability receipt for every scoped state.
func (*xfrmBackend) SAStateScopeSupported() bool { return true }

// CheckSAStateScope reads the same fresh receipt that InstallSA requires, so
// doctor can expose a missing dependency before the first Child SA attempt.
// Safe for concurrent use; the check owns its read and changes no host state.
func (*xfrmBackend) CheckSAStateScope() error {
	return checkXFRMStateScopeCapabilities()
}

// checkXFRMStateScopeCapabilities reads the live namespace before InstallSA can
// publish any state. Every explicit scope needs all seven features, independently
// of whether this particular SA permits noninitial fragments or carries the ICMP
// error check: the commit no longer refuses an inbound discard entry beside a peer
// whose check is on (engine/icmp_overlap.go), because the reverse check bit 5 names
// honors that entry, so a kernel without bit 5 installs no Child SA at all. Bit 6
// is the only route by which a forwarded ICMP error that an out DISCARD covers, the
// catch-all's included, reaches its return SA (RFC 4301 Section 6.2), so a kernel
// without it installs no Child SA either. Bit 7 is what lets a forwarded ICMP error
// that arrived on the SA past an operator bypass's FWD transit gate be judged by its
// payload (RFC 4301 Section 5.2, step 4), so a kernel without it installs none either.
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
		xfrmZECapFragmentContext | xfrmZECapFragmentGuard | xfrmZECapICMPReverse |
		xfrmZECapICMPForwardMap | xfrmZECapICMPForwardGate
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
	return opererr.Wrap(errors.Join(ErrSAOptionRefused, cause),
		"Ze cannot verify kernel support for the IPsec SA's complete traffic selector scope",
		"the SA needs verified state, ICMP quote, ICMP reverse and fragment checks, the forwarded ICMP error mapping and the forwarded ICMP error past a transit gate, before any state can be installed; rebuild and boot Ze's native runtime kernel",
		RuntimeKernelBuildFix())
}

// xfrmForwardGather keeps the first answer GathersForwardFragments read from the live
// kernel. The zero value has read nothing. Safe for concurrent use: mu guards both
// fields.
type xfrmForwardGather struct {
	mu      sync.Mutex
	known   bool
	gathers bool
}

// GathersForwardFragments reports whether the live kernel advertises K3's fragment
// guard, capability bit 3 (xfrmZECapFragmentGuard). While any policy whose selector
// holds a protocol or a port owns a guard reference, which every port-scoped BYPASS
// does, a template-free one in FWD included (xfrm_policy_insert takes it before
// publication, xfrm_policy_hold_fragment_guard), the PRE_ROUTING hooks
// xfrm4_fragment_gather and xfrm6_fragment_gather reassemble every fragment ahead of
// ip_forward's XFRM_POLICY_FWD check, so that check reads the ports of the whole
// datagram. A fragment that never completes stays in the queue until it expires and is
// never forwarded.
//
// RFC 4301 Section 7.4: "All implementations MUST support stateful fragment checking to
// accommodate BYPASS traffic for which a non-trivial port range is specified."
//
// A GETDEFAULT answer without Ze's receipt proves no guard, which is the answer false.
// A read that fails, or a receipt that is not Ze's ABI, is no answer, and its error is
// returned with false.
//
// The first answer is kept, so every later call repeats it. The bit cannot change while
// the kernel runs (xfrm_fragment_ready is __ro_after_init, set once by
// xfrm_fragment_init), and the engine derives one set of policies for an entry's
// install, replacement and removal from this answer (engine/spd_policy.go,
// spdEntryPolicies): a later read that failed would otherwise split them. A failure
// keeps nothing, so the next call reads again.
func (b *xfrmBackend) GathersForwardFragments() (bool, error) {
	b.forwardGather.mu.Lock()
	defer b.forwardGather.mu.Unlock()
	if b.forwardGather.known {
		return b.forwardGather.gathers, nil
	}
	caps, err := xfrmScopeCapsRead()
	if errors.Is(err, errXFRMZECapabilitiesAbsent) {
		b.forwardGather.known = true
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read the kernel's XFRM capability receipt: %w", err)
	}
	if caps == nil {
		return false, errors.New("the kernel returned no capability receipt")
	}
	b.forwardGather.known = true
	b.forwardGather.gathers = caps.Features.Has(xfrmZECapFragmentGuard)
	return b.forwardGather.gathers, nil
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
