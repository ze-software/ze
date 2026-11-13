//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- ICMP error messages carried by a Child SA.
// Related: unmatched.go -- the actual node-wide final-disposition installer and release.
// RFC naming: untagged -- namespace behavior must be observed before claiming conformance.

package engine

import (
	"testing"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// Linux include/uapi/linux/xfrm.h: SETDEFAULT follows MAPPING (0x26), GETDEFAULT
// follows SETDEFAULT, and xfrm_userpolicy_default is three bytes in IN/FWD/OUT order.
// These are test observers/setup only; the operation being judged is installUnmatched.
const (
	forwardDefaultSet    = 0x27
	forwardDefaultGet    = 0x28
	forwardDefaultBlock  = 1
	forwardDefaultAccept = 2
)

func forwardDefaultRead(t *testing.T) [3]byte {
	t.Helper()
	req := nl.NewNetlinkRequest(forwardDefaultGet, unix.NLM_F_REQUEST)
	req.AddRawData([]byte{0, 0, 0})
	messages, err := req.Execute(unix.NETLINK_XFRM, forwardDefaultGet)
	if err != nil {
		t.Fatalf("read the private namespace's XFRM defaults: %v", err)
	}
	if len(messages) != 1 || len(messages[0]) < 3 {
		t.Fatalf("GETDEFAULT returned %d messages, want one three-byte default-policy payload: %v", len(messages), messages)
	}
	got := [3]byte{messages[0][0], messages[0][1], messages[0][2]}
	for direction, action := range got {
		if action != forwardDefaultBlock && action != forwardDefaultAccept {
			t.Fatalf("GETDEFAULT direction %d returned unknown action %d", direction, action)
		}
	}
	return got
}

func forwardDefaultSeed(t *testing.T, values [3]byte) {
	t.Helper()
	req := nl.NewNetlinkRequest(forwardDefaultSet, unix.NLM_F_ACK)
	req.AddRawData(values[:])
	if _, err := req.Execute(unix.NETLINK_XFRM, 0); err != nil {
		t.Fatalf("seed the private namespace's XFRM defaults: %v", err)
	}
	if got := forwardDefaultRead(t); got != values {
		t.Fatalf("seed XFRM defaults: got %v, want %v", got, values)
	}
}

// TestRFC4301ForwardDefaultFollowsAndReleasesUnmatched tests the real install,
// reload and release sequence. IN/OUT defaults belong to the pre-existing namespace
// and MUST survive. A second apply MUST NOT replace the original FWD snapshot with
// Ze's running value. No packet assertion is inferred from this netlink readback;
// rfc4301_forward_icmp_integration_linux_test.go supplies actual forwarding proof.
//
//ze:guest-root
func TestRFC4301ForwardDefaultFollowsAndReleasesUnmatched(t *testing.T) {
	_ = newEspPolLab(t)
	if err := dataplane.Load("xfrm"); err != nil {
		t.Fatalf("load XFRM for the private namespace: %v", err)
	}
	t.Cleanup(func() {
		if err := dataplane.CloseBackend(); err != nil {
			t.Errorf("close the private XFRM backend: %v", err)
		}
	})
	original := forwardDefaultRead(t)
	seed := [3]byte{forwardDefaultBlock, forwardDefaultAccept, forwardDefaultAccept}
	forwardDefaultSeed(t, seed)
	log := slogutil.DiscardLogger()
	t.Cleanup(func() {
		removeUnmatched(dataplane.Get(), log)
		forwardDefaultSeed(t, original)
	})
	for _, action := range []dataplane.SPAction{dataplane.SPActionDiscard, dataplane.SPActionBypass} {
		if err := installUnmatched(dataplane.Get(), action, log); err != nil {
			t.Fatalf("install unmatched %v: %v", action, err)
		}
		want := seed
		if action == dataplane.SPActionDiscard {
			want[1] = forwardDefaultBlock
		}
		if got := forwardDefaultRead(t); got != want {
			t.Errorf("unmatched %v: namespace defaults=%v, want %v; only FWD may change", action, got, want)
		}
		policies, err := dataplane.Get().ListPolicies()
		if err != nil {
			t.Fatalf("read the installed final dispositions: %v", err)
		}
		for _, policy := range policies {
			if policy.Dir == dataplane.SADirFwd && policy.Role == dataplane.PolicyRoleCatchAll {
				t.Errorf("unmatched %v installed an explicit FWD catch-all; it suppresses xfrm_in_fwd_icmp", action)
			}
		}
	}
	removeUnmatched(dataplane.Get(), log)
	if got := forwardDefaultRead(t); got != seed {
		t.Errorf("release restored namespace defaults=%v, want the original snapshot %v", got, seed)
	}
}
