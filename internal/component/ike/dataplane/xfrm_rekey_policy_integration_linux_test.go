// VALIDATES: what a real Linux XFRM stack does when the SAME policy selector is
// installed twice. A Child SA rekey does exactly that: newRekeyedChild
// (engine/rekey.go) inherits TSLocal, TSRemote, IfID, ReqID and Mode from the
// retired pair, and installChildSA (engine/child.go) calls InstallPolicy
// unconditionally on every install, so the replacement's two policies are
// identical in every field xfrmPolicyFromParams sets.
//
// Two facts, one file. XFRM_MSG_NEWPOLICY is exclusive and refuses the repeat, and
// XFRM_MSG_UPDPOLICY upserts and accepts it. ze sent the first and every
// make-before-break rekey failed; it now sends the second.
//
// PREVENTS: reasoning about the kernel's exclusivity rule instead of measuring it,
// in EITHER direction. The exclusivity test would go quiet if it were routed through
// the backend that stopped using NEWPOLICY, so it calls netlink directly; the
// idempotence test goes red the moment the backend reverts to NEWPOLICY.

//go:build integration && linux

package dataplane

import (
	"errors"
	"net"
	"testing"

	"github.com/vishvananda/netlink"

	"golang.org/x/sys/unix"
)

// rekeySPParams builds the outbound policy a Child SA installs. The two calls a
// rekey makes differ in NOTHING this function takes, which is the point.
func rekeySPParams() SPParams {
	_, local, _ := net.ParseCIDR("10.200.0.0/24")
	_, remote, _ := net.ParseCIDR("10.201.0.0/24")
	return SPParams{
		Src:       local,
		Dst:       remote,
		Dir:       SADirOut,
		Proto:     50, // ESP
		Mode:      ModeTunnel,
		IfID:      0x7e51,
		ReqID:     0x7e51,
		TunnelSrc: net.ParseIP("192.0.2.10"),
		TunnelDst: net.ParseIP("192.0.2.20"),
	}
}

// skipWithoutPolicyPermission is the file's SINGLE skip site. Both tests below need
// CAP_NET_ADMIN to install anything, and neither can assert a thing without it.
func skipWithoutPolicyPermission(t *testing.T, err error) {
	t.Helper()
	if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
		t.Skipf("XFRM policy install needs CAP_NET_ADMIN: %v", err)
	}
}

// TestXFRMNewPolicyIsExclusive keeps measuring the KERNEL fact, unchanged: an
// XFRM_MSG_NEWPOLICY install of a selector that already exists is refused with
// EEXIST. Nothing about the kernel changed; what changed is that ze no longer sends
// that message, for the reason the next test records.
//
// It calls netlink directly rather than through the backend, because the backend is
// precisely what stopped using NEWPOLICY. Measuring a kernel rule through a caller
// that no longer exercises it would measure nothing.
func TestXFRMNewPolicyIsExclusive(t *testing.T) {
	pol, err := xfrmPolicyFromParams(rekeySPParams())
	if err != nil {
		t.Fatalf("building the policy: %v", err)
	}
	if err := netlink.XfrmPolicyAdd(pol); err != nil {
		skipWithoutPolicyPermission(t, err)
		t.Fatalf("first XFRM_MSG_NEWPOLICY install: %v", err)
	}
	t.Cleanup(func() { _ = netlink.XfrmPolicyDel(pol) })

	second := netlink.XfrmPolicyAdd(pol)
	if second == nil {
		t.Fatal("the kernel ACCEPTED a second XFRM_MSG_NEWPOLICY for one selector; " +
			"the exclusivity that forced ze onto XFRM_MSG_UPDPOLICY does not occur")
	}
	if !errors.Is(second, unix.EEXIST) {
		t.Errorf("expected EEXIST from XFRM_MSG_NEWPOLICY, got %v", second)
	}
	t.Logf("kernel refuses a repeat NEWPOLICY, as expected: %v", second)
}

// TestXFRMPolicyInstallIsIdempotent is the make-before-break rekey collision,
// measured on the FIXED path.
//
// A Child SA rekey re-installs an identical selector: newRekeyedChild
// (engine/rekey.go) inherits TSLocal, TSRemote, IfID, ReqID and Mode from the
// retired pair, and installChildSA calls InstallPolicy unconditionally on every
// install. Under XFRM_MSG_NEWPOLICY that second install was refused, the rekey
// response was abandoned, and the tunnel died at the Child SA's hard lifetime.
// MEASURED against strongSwan before the fix: "child-sa: install inbound policy:
// xfrm: policy add: file exists", once per second until "child-sa: hard lifetime
// expired".
//
// The backend now sends XFRM_MSG_UPDPOLICY, which upserts, so this asserts the
// repeat SUCCEEDS. Reverting InstallPolicy to XfrmPolicyAdd turns it red.
func TestXFRMPolicyInstallIsIdempotent(t *testing.T) {
	b := &xfrmBackend{}
	p := rekeySPParams()

	if err := b.InstallPolicy(p); err != nil {
		skipWithoutPolicyPermission(t, err)
		t.Fatalf("first install of the retired pair's policy: %v", err)
	}
	t.Cleanup(func() {
		_ = b.RemovePolicyParams(p)
	})

	// The replacement pair. Identical selector, identical everything.
	if err := b.InstallPolicy(p); err != nil {
		t.Fatalf("the replacement pair's policy install was refused (%v); every "+
			"make-before-break rekey fails here and the tunnel dies at the Child SA's "+
			"hard lifetime", err)
	}

	// STILL TRUE, and still a hazard worth pinning: ONE delete removes the selector,
	// because the kernel identifies a policy by its selector and the upsert leaves
	// exactly one. A caller retiring the superseded half of a make-before-break pair
	// must not remove the policy while the replacement is still live.
	if err := b.RemovePolicyParams(p); err != nil {
		t.Fatalf("removing the one policy: %v", err)
	}
	if err := b.RemovePolicyParams(p); err == nil {
		t.Error("a second delete succeeded, so more than one policy existed")
	}
}
