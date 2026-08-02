// VALIDATES: RemovePolicyParams itself, not the ownership helper underneath it. A delete
// the kernel REFUSES must leave the ownership record in place, so the next foreign peer is
// still refused that selector.
// PREVENTS: the guard failing open on its own error path. release both CHECKS the owner
// and DROPS the record, and it runs BEFORE the netlink call. A non-ENOENT failure after it
// therefore left ze with no record over a policy the kernel still held, and the next
// peer's InstallPolicy claimed that selector and upserted over a live tunnel.
//
// It runs where the entry point does. policy_owner_test.go drives the helper on every OS;
// driving the helper alone would leave the wiring between the two unproven, which is the
// exact shape ai/rules/fail-closed-guards.md names.
//
// No kernel is touched: the xfrmPolicyDel seam is swapped for the duration, because no
// real kernel refuses a well-formed delete on demand for a reason other than ENOENT. That
// also means no netns and no CAP_NET_ADMIN, so this is merge-gated rather than nightly.

//go:build linux

package dataplane

import (
	"errors"
	"net"
	"syscall"
	"testing"

	"github.com/vishvananda/netlink"
)

// delFailSelector is the ordinary site-to-site policy two peers collide on.
func delFailSelector(t *testing.T, owner string) SPParams {
	t.Helper()
	_, local, err := net.ParseCIDR("10.220.0.0/24")
	if err != nil {
		t.Fatalf("parse the local prefix: %v", err)
	}
	_, remote, err := net.ParseCIDR("10.221.0.0/24")
	if err != nil {
		t.Fatalf("parse the remote prefix: %v", err)
	}
	return SPParams{
		Src:       local,
		Dst:       remote,
		Dir:       SADirOut,
		Action:    SPActionProtect,
		Owner:     owner,
		Proto:     ProtoESP,
		Mode:      ModeTunnel,
		ReqID:     0x0d21,
		Priority:  PriorityChildSA,
		TunnelSrc: net.ParseIP("192.0.2.10"),
		TunnelDst: net.ParseIP("192.0.2.20"),
	}
}

// withPolicyDelError makes the kernel call answer err for the duration of one test.
func withPolicyDelError(t *testing.T, err error) *int {
	t.Helper()
	calls := 0
	saved := xfrmPolicyDel
	xfrmPolicyDel = func(*netlink.XfrmPolicy) error {
		calls++
		return err
	}
	t.Cleanup(func() { xfrmPolicyDel = saved })
	return &calls
}

func TestRemovePolicyParamsKeepsTheRecordWhenTheKernelRefusesTheDelete(t *testing.T) {
	b := &xfrmBackend{}
	mine := delFailSelector(t, "site-a")
	if _, err := b.policies.claim(mine); err != nil {
		t.Fatalf("the owning peer could not claim its selector: %v", err)
	}

	// EPERM stands for every refusal that is NOT "the kernel has no such policy": the
	// policy is still installed after it.
	calls := withPolicyDelError(t, syscall.EPERM)

	err := b.RemovePolicyParams(mine)
	if err == nil {
		t.Fatal("RemovePolicyParams reported success over a refused kernel delete")
	}
	if *calls != 1 {
		t.Fatalf("the kernel delete ran %d times, want 1", *calls)
	}
	if !errors.Is(err, syscall.EPERM) {
		t.Errorf("the failure is %v, want the kernel's own EPERM wrapped so a caller can read it", err)
	}

	if held, ok := b.policies.ownerOf(mine); !ok || held != "site-a" {
		t.Fatalf("owner after the refused delete = %q (present=%v), want site-a: the kernel still holds that policy", held, ok)
	}

	// The consequence the record exists to produce.
	foreign := delFailSelector(t, "site-b")
	var owned *PolicyOwnedError
	if installErr := b.InstallPolicy(foreign); !errors.As(installErr, &owned) {
		t.Fatalf("a second peer's install returned %v, want *PolicyOwnedError: it would upsert over site-a's live policy", installErr)
	}
}

func TestRemovePolicyParamsDropsTheRecordWhenTheKernelHasNoSuchPolicy(t *testing.T) {
	b := &xfrmBackend{}
	mine := delFailSelector(t, "site-a")
	if _, err := b.policies.claim(mine); err != nil {
		t.Fatalf("claim: %v", err)
	}

	withPolicyDelError(t, syscall.ENOENT)

	if err := b.RemovePolicyParams(mine); err == nil {
		t.Fatal("RemovePolicyParams hid the kernel's ENOENT from its caller")
	}
	if held, ok := b.policies.ownerOf(mine); ok {
		t.Errorf("owner after an ENOENT delete = %q, want no record: the kernel holds no such policy", held)
	}

	// A record that outlived its policy would refuse this.
	foreign := delFailSelector(t, "site-b")
	if _, err := b.policies.claim(foreign); err != nil {
		t.Fatalf("a selector the kernel no longer holds could not be claimed: %v", err)
	}
}

func TestRemovePolicyParamsDropsTheRecordWhenTheKernelAccepts(t *testing.T) {
	b := &xfrmBackend{}
	mine := delFailSelector(t, "site-a")
	if _, err := b.policies.claim(mine); err != nil {
		t.Fatalf("claim: %v", err)
	}

	calls := withPolicyDelError(t, nil)

	if err := b.RemovePolicyParams(mine); err != nil {
		t.Fatalf("an accepted delete reported %v", err)
	}
	if *calls != 1 {
		t.Fatalf("the kernel delete ran %d times, want 1", *calls)
	}
	if held, ok := b.policies.ownerOf(mine); ok {
		t.Errorf("owner after a successful delete = %q, want no record", held)
	}
}
