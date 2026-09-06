// VALIDATES: the DISCARD disposition of RFC 4301 Section 4.4.1 reaches the Linux
// kernel as XFRM_POLICY_BLOCK, carries no transform template, and is read back as a
// discard rather than as a protect entry.
// PREVENTS: a discard entry projected as XFRM_POLICY_ALLOW. The kernel would then
// pass exactly the traffic the operator wrote the entry to stop, and nothing would
// report it: the install succeeds and the policy is present in `ip xfrm policy`.

//go:build linux

package dataplane

import (
	"net"
	"testing"

	"github.com/vishvananda/netlink"
)

// discardParams is the smallest template-free policy under test: one prefix pair, one
// direction, one disposition. Nothing else is set, because a discard names no
// transform and every template field must stay at its zero value.
func discardParams(t *testing.T, action SPAction, dir SADir) SPParams {
	t.Helper()
	_, src, err := net.ParseCIDR("192.0.2.0/24")
	if err != nil {
		t.Fatalf("parse local prefix: %v", err)
	}
	_, dst, err := net.ParseCIDR("198.51.100.0/24")
	if err != nil {
		t.Fatalf("parse remote prefix: %v", err)
	}
	return SPParams{Src: src, Dst: dst, Dir: dir, Action: action, Priority: 1000}
}

// VALIDATES: RFC4301-7.4-1. A policy carrying SPActionDiscard is built for the kernel
// as XFRM_POLICY_BLOCK with an empty template list, which is the SPD classification
// mechanism the section requires the discard to run through.
// PREVENTS: the disposition being dropped between the config and the kernel. Before
// SPActionDiscard existed, the only two actions were protect and bypass, so an
// operator had no way to express a discard at all.
// RFC requirement: RFC4301-7.4-1 positive -- a discard entry reaches the kernel as a
// blocking policy.
func TestDiscardPolicyReachesTheKernelAsBlock(t *testing.T) {
	pol, err := xfrmPolicyFromParams(discardParams(t, SPActionDiscard, SADirOut))
	if err != nil {
		t.Fatalf("xfrmPolicyFromParams for a discard: %v", err)
	}
	if pol.Action != netlink.XFRM_POLICY_BLOCK {
		t.Errorf("Action = %v, want XFRM_POLICY_BLOCK: an allow action passes the traffic the entry exists to stop", pol.Action)
	}
	// RFC 4301 Section 4.4.1 gives a discard no transform. A template would name an
	// SA for traffic that never reaches one.
	if len(pol.Tmpls) != 0 {
		t.Errorf("Tmpls len = %d, want 0: a discard hands traffic to no transform", len(pol.Tmpls))
	}
}

// VALIDATES: RFC4301-7.4-1. The BYPASS disposition of the same section still reaches
// the kernel as XFRM_POLICY_ALLOW after the discard was added beside it.
// PREVENTS: the two template-free dispositions collapsing into one. Both are built by
// the same branch of xfrmPolicyFromParams, so a mapper that answered BLOCK for
// everything template-free would black-hole ze's own IKE traffic, and every tunnel on
// the node would stop rekeying.
// RFC requirement: RFC4301-7.4-1 negative -- a bypass entry is not turned into a block.
func TestBypassPolicyIsNotBlocked(t *testing.T) {
	pol, err := xfrmPolicyFromParams(discardParams(t, SPActionBypass, SADirOut))
	if err != nil {
		t.Fatalf("xfrmPolicyFromParams for a bypass: %v", err)
	}
	if pol.Action != netlink.XFRM_POLICY_ALLOW {
		t.Errorf("Action = %v, want XFRM_POLICY_ALLOW for a bypass", pol.Action)
	}
}

// VALIDATES: xfrmPolicyAction refuses an action it cannot express rather than
// choosing one. The two kernel actions are opposites, so a default is a coin toss
// between passing and dropping the operator's traffic.
// PREVENTS: a future disposition (RESOLVE, for instance) silently taking the
// behavior of whichever branch a default happened to fall into.
func TestXfrmPolicyActionRefusesAnUnknownDisposition(t *testing.T) {
	if _, err := xfrmPolicyAction(SPAction(200)); err == nil {
		t.Fatal("xfrmPolicyAction accepted an unknown disposition; it must refuse rather than pick one of two opposite kernel actions")
	}
	// PROTECT is refused here too, because it carries a template and is built on the
	// other side of the isTemplateFree branch.
	if _, err := xfrmPolicyAction(SPActionProtect); err == nil {
		t.Fatal("xfrmPolicyAction accepted PROTECT; a protect policy is not template-free")
	}
}

// VALIDATES: RFC4301-7.4-1. A blocking policy read back from the kernel reports
// SPActionDiscard, so the inspection path tells an operator what the kernel is
// actually doing with the traffic.
// PREVENTS: the readback reporting a discard as a protect entry. policyInfoFromKernel
// derived the action from an empty template list alone, and a BLOCK policy carrying a
// template (which the kernel accepts and ignores) would have read as protect: the
// operator would be told their traffic is being encrypted while it is being dropped.
// RFC requirement: RFC4301-7.4-1 positive -- an installed discard is reported as one.
func TestPolicyReadbackReportsDiscard(t *testing.T) {
	b := &xfrmBackend{}
	_, src, _ := net.ParseCIDR("192.0.2.0/24")    //nolint:errcheck // constant prefix
	_, dst, _ := net.ParseCIDR("198.51.100.0/24") //nolint:errcheck // constant prefix

	blocked := b.policyInfoFromKernel(&netlink.XfrmPolicy{
		Src:    src,
		Dst:    dst,
		Dir:    netlink.XFRM_DIR_OUT,
		Action: netlink.XFRM_POLICY_BLOCK,
	})
	if blocked.Action != SPActionDiscard {
		t.Errorf("readback Action = %d, want SPActionDiscard (%d)", blocked.Action, SPActionDiscard)
	}

	// The negative half: an allow policy with no template is still a bypass, and one
	// WITH a template is still a protect entry.
	bypassed := b.policyInfoFromKernel(&netlink.XfrmPolicy{
		Src:    src,
		Dst:    dst,
		Dir:    netlink.XFRM_DIR_OUT,
		Action: netlink.XFRM_POLICY_ALLOW,
	})
	if bypassed.Action != SPActionBypass {
		t.Errorf("readback Action = %d for a template-free allow, want SPActionBypass (%d)", bypassed.Action, SPActionBypass)
	}
	protected := b.policyInfoFromKernel(&netlink.XfrmPolicy{
		Src:    src,
		Dst:    dst,
		Dir:    netlink.XFRM_DIR_OUT,
		Action: netlink.XFRM_POLICY_ALLOW,
		Tmpls:  []netlink.XfrmPolicyTmpl{{Proto: netlink.Proto(50), Mode: netlink.XFRM_MODE_TUNNEL}},
	})
	if protected.Action != SPActionProtect {
		t.Errorf("readback Action = %d for an allow with a template, want SPActionProtect (%d)", protected.Action, SPActionProtect)
	}
}
