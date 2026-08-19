// VALIDATES: spec-fixit-ike-resource-lifetime-leaks AC-5 on a real kernel -- after a
// tunnel's state and policy are torn down twice, the XFRM state table holds no state of
// this tunnel, the XFRM policy table holds no policy of it, and the ESP form receiver
// holds neither the SPI nor its raw sockets. It also settles A-2: a stranded espForms
// entry keeps a REAL host-wide raw ESP socket open, which only a kernel can show.
// PREVENTS: the second teardown, and every failed first one, leaving the raw ESP sockets
// open for a tunnel that is gone. The Forget sat below the delete's error return, so an
// SA whose state the kernel had already dropped -- a hard lifetime expiry, an operator
// flush, a peer Delete that removeChildSA answered twice -- stayed watched for the life
// of the process. The receiver then makes the kernel clone every bare ESP packet on the
// box, which taxes the fast path this design exists to preserve.
//
// Auto-enrolled in the QEMU integration run through the derived `integration && linux`
// package list (mk/test-integration.mk, ZE_QEMU_INTEGRATION_PKGS). It needs CAP_NET_ADMIN
// for the state and policy, and CAP_NET_RAW for the ESP form sockets, so it skips rather
// than fails where it has neither.

//go:build integration && linux

package dataplane

import (
	"errors"
	"net"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/slogutil"
)

const (
	teardownSPI   uint32 = 0x7e0d0001
	teardownReqID uint32 = 0x7e0d
)

// teardownPolicy is the Child SA policy shape installChildSA emits, narrowed to
// RFC 5737 TEST-NET-3 so it can never capture traffic this VM depends on.
func teardownPolicy(t *testing.T) SPParams {
	t.Helper()
	_, local, err := net.ParseCIDR("203.0.113.0/24")
	if err != nil {
		t.Fatalf("parse the local prefix: %v", err)
	}
	_, remote, err := net.ParseCIDR("198.51.100.0/24")
	if err != nil {
		t.Fatalf("parse the remote prefix: %v", err)
	}
	return SPParams{
		Src:       local,
		Dst:       remote,
		Dir:       SADirOut,
		Action:    SPActionProtect,
		Owner:     "ac5-teardown",
		Proto:     ProtoESP,
		Mode:      ModeTunnel,
		ReqID:     teardownReqID,
		Priority:  PriorityChildSA,
		TunnelSrc: net.ParseIP("203.0.113.10"),
		TunnelDst: net.ParseIP("198.51.100.10"),
	}
}

func teardownPolicyInstalled(t *testing.T, b *xfrmBackend, want SPParams) bool {
	t.Helper()
	policies, err := b.ListPolicies()
	if err != nil {
		skipUnprivileged(t, err)
		t.Fatalf("ListPolicies: %v", err)
	}
	for _, p := range policies {
		if p.ReqID == want.ReqID && p.Dir == want.Dir {
			return true
		}
	}
	return false
}

func TestXFRMDoubleTeardownLeavesNothing(t *testing.T) {
	local := net.ParseIP("203.0.113.10")
	remote := net.ParseIP("198.51.100.10")

	b := &xfrmBackend{espForms: newESPFormReceiver(slogutil.DiscardLogger())}

	// The inbound state of a NAT-traversing tunnel: it asks for both ESP wire forms,
	// so the backend watches its SPI beside the kernel and opens the raw sockets.
	// RFC 7296 Section 2.23.
	err := b.InstallSA(SAParams{
		SPI: teardownSPI, Src: remote, Dst: local, Proto: ProtoESP, Mode: ModeTunnel,
		ReqID: teardownReqID, ReplayWin: 64,
		EncAlgo: "aes256", EncKey: make([]byte, 32),
		AuthAlgo: "sha256", AuthKey: make([]byte, 32),
		UDPEncap: true, UDPEncapSPort: 4500, UDPEncapDPort: 4500,
		AcceptBothESPForms: true,
	})
	if err != nil {
		skipUnprivileged(t, err)
		t.Fatalf("InstallSA: %v", err)
	}
	t.Cleanup(func() { _ = b.RemoveSA(teardownSPI, local, ProtoESP) })

	if !b.espForms.running() {
		t.Fatal("the installed SA asked for both ESP forms and the receiver holds no socket, so the release asserted below proves nothing")
	}

	policy := teardownPolicy(t)
	if err := b.InstallPolicy(policy); err != nil {
		skipUnprivileged(t, err)
		t.Fatalf("InstallPolicy: %v", err)
	}
	t.Cleanup(func() { _ = b.RemovePolicyParams(policy) })
	if !teardownPolicyInstalled(t, b, policy) {
		t.Fatal("the Child SA policy is absent from the kernel right after it was installed, so its removal proves nothing")
	}

	// The kernel drops the state on its own: a hard lifetime expiry, an operator
	// flush, or the peer's Delete answered by a removeChildSA that already ran. This
	// is what makes the FIRST teardown below fail, which is the ordinary case that
	// stranded the ESP form.
	state := &netlink.XfrmState{Dst: local, Proto: netlink.Proto(ProtoESP), Spi: int(teardownSPI)}
	if err := netlink.XfrmStateDel(state); err != nil {
		t.Fatalf("deleting the state behind the backend's back: %v", err)
	}

	// Teardown, twice. Both are expected to report the missing state, and neither may
	// leave anything of this tunnel behind.
	for i := 1; i <= 2; i++ {
		if err := b.RemoveSA(teardownSPI, local, ProtoESP); err == nil {
			t.Errorf("teardown %d: the kernel accepted a delete for a state that is gone", i)
		} else if !errors.Is(err, unix.ESRCH) {
			t.Logf("teardown %d reported %v", i, err)
		}
		if err := b.RemovePolicyParams(policy); err != nil && i == 1 {
			t.Errorf("teardown %d: RemovePolicyParams: %v", i, err)
		}
	}

	if findSA(t, b, teardownSPI) != nil {
		t.Errorf("the XFRM state table still holds spi %#x after two teardowns", teardownSPI)
	}
	if teardownPolicyInstalled(t, b, policy) {
		t.Errorf("the XFRM policy table still holds the Child SA policy (reqid %#x) after two teardowns", policy.ReqID)
	}
	if _, watched := b.espForms.reg.target(teardownSPI); watched {
		t.Errorf("the ESP form receiver still watches spi %#x after two teardowns", teardownSPI)
	}
	if b.espForms.running() {
		t.Error("the ESP form receiver still holds its raw ESP sockets after the last watched SA was torn down: " +
			"the kernel clones every bare ESP packet on this host for a tunnel that no longer exists")
	}
}
