// VALIDATES: against a real Linux XFRM stack, that a peer's ESP Delete for the LIVE Child
// SA leaves the shared kernel policy in place while a parallel re-initiation's pending
// Child SA still answers to it, and that the surviving policy still CLAIMS traffic.
// PREVENTS: the fake-SPD tests in child_policy_rollback_test.go agreeing with a model of
// the kernel rather than with the kernel. The defect they cover is that outbound traffic
// left the box in the clear, and only a kernel can answer whether it did.
//
// The instrument is /proc/net/xfrm_stat, never a write result. XfrmOutNoStates rises once
// per outbound datagram that MATCHED a PROTECT policy whose template resolved to no state
// (net/xfrm/xfrm_policy.c, the -EAGAIN arm of xfrm_lookup_with_ifid). Every policy below is
// installed WITHOUT a state, so that counter answers exactly one question: did a policy
// claim this packet. sendto() cannot answer it -- it reports the same success whether XFRM
// protected the datagram, held it, or never looked at it.
//
// Egress is a DUMMY device on purpose. The sibling probe in the dataplane package measured
// that a loopback egress bypasses the outbound XFRM hook entirely, so every reading here
// would be a false "no policy claimed it" (xfrm_policy_owner_integration_linux_test.go).

//go:build integration && linux

package engine

import (
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/slogutil"
)

const (
	// delStatOutNoStates is the positive detector: it moves when a policy claimed the
	// probe datagram, and stays still when the datagram left unprotected.
	delStatOutNoStates = "XfrmOutNoStates"

	delLocalCIDR  = "10.230.0.0/24"
	delRemoteCIDR = "10.231.0.0/24"
	delLocalAddr  = "10.230.0.1"
	delRemoteAddr = "10.231.0.1"
	delProbePort  = 9

	// delEgressDev must not be loopback. See the file comment.
	delEgressDev = "ze-del0"

	delOwner = "site-a"

	// delChildEnv marks the re-executed probe process.
	delChildEnv = "ZE_CHILD_DELETE_PROBE_OWN_PROCESS"
)

// delOwnProcess gives this probe a process to itself, and returns true only in the child
// where the body must run.
//
// The dataplane package measured that one test BINARY supports exactly one network
// namespace unshare: a second probe in the same process reports that no xfrm counter moved
// at all, which is this file's NEGATIVE reading, so a probe sharing a process would
// manufacture its own false pass. The engine package's binary has one probe today, and this
// keeps that from being an ordering assumption.
func delOwnProcess(t *testing.T) bool {
	t.Helper()
	if os.Getenv(delChildEnv) == "1" {
		return true
	}
	cmd := exec.Command(os.Args[0], "-test.run", "^"+t.Name()+"$", "-test.v")
	cmd.Env = append(os.Environ(), delChildEnv+"=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("probe in its own process failed: %v\n%s", err, out)
	}
	if strings.Contains(string(out), "--- SKIP") {
		// test-relax: this propagates the child's skip rather than relaxing anything.
		// The child ran every assertion; it skips only where it always did, when the
		// kernel refuses CLONE_NEWNET or CAP_NET_ADMIN. Reporting that as a parent PASS
		// would be the fail-open reading of the same result.
		t.Skipf("probe skipped in its own process:\n%s", out)
	}
	if !strings.Contains(string(out), "--- PASS") {
		t.Fatalf("probe in its own process reported neither PASS nor SKIP:\n%s", out)
	}
	// The counter deltas are the whole measurement, and they are printed in the child.
	// Surfacing them here is what makes a green run readable as evidence rather than as a
	// bare PASS.
	t.Logf("probe output from its own process:\n%s", out)
	return false
}

// delNetns puts the probe in a namespace of its own, with a real egress device carrying an
// address inside the policy's SOURCE prefix and a route to its DESTINATION prefix. XFRM's
// outbound hook sits in route resolution, so a datagram with no route never reaches it.
func delNetns(t *testing.T) {
	t.Helper()
	runtime.LockOSThread()
	if err := unix.Unshare(unix.CLONE_NEWNET); err != nil {
		t.Skipf("no CLONE_NEWNET (needs root): %v", err)
	}
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatalf("lo lookup: %v", err)
	}
	if err := netlink.LinkSetUp(lo); err != nil {
		t.Fatalf("lo up: %v", err)
	}

	egress := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: delEgressDev}}
	if err := netlink.LinkAdd(egress); err != nil {
		t.Fatalf("add dummy %s: %v; without a non-loopback egress the outbound XFRM hook is never reached and every reading below is blind", delEgressDev, err)
	}
	link, err := netlink.LinkByName(delEgressDev)
	if err != nil {
		t.Fatalf("%s lookup: %v", delEgressDev, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("%s up: %v", delEgressDev, err)
	}
	addr, err := netlink.ParseAddr(delLocalAddr + "/24")
	if err != nil {
		t.Fatalf("parse %s/24: %v", delLocalAddr, err)
	}
	if err := netlink.AddrAdd(link, addr); err != nil {
		t.Fatalf("add %s to %s: %v", addr, delEgressDev, err)
	}
	_, remote, err := net.ParseCIDR(delRemoteCIDR)
	if err != nil {
		t.Fatalf("parse %s: %v", delRemoteCIDR, err)
	}
	route := &netlink.Route{LinkIndex: link.Attrs().Index, Dst: remote, Scope: netlink.SCOPE_LINK}
	if err := netlink.RouteAdd(route); err != nil {
		t.Fatalf("route %s dev %s: %v", delRemoteCIDR, delEgressDev, err)
	}

	if got := delListPolicies(t); len(got) != 0 {
		t.Fatalf("the fresh namespace already holds %d XFRM policies (%v); no counter reading below would be attributable", len(got), got)
	}
}

// delListPolicies returns every policy in this namespace, rendered.
func delListPolicies(t *testing.T) []string {
	t.Helper()
	pols, err := netlink.XfrmPolicyList(netlink.FAMILY_ALL)
	if err != nil {
		t.Fatalf("xfrm policy list: %v", err)
	}
	out := make([]string, 0, len(pols))
	for i := range pols {
		out = append(out, pols[i].String())
	}
	return out
}

// delStat reads one counter out of the namespace's own /proc/net/xfrm_stat.
func delStat(t *testing.T, name string) int {
	t.Helper()
	raw, err := os.ReadFile("/proc/net/xfrm_stat")
	if err != nil {
		t.Fatalf("read xfrm_stat: %v", err)
	}
	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == name {
			n, convErr := strconv.Atoi(fields[1])
			if convErr != nil {
				t.Fatalf("parse %s = %q: %v", name, fields[1], convErr)
			}
			return n
		}
	}
	t.Fatalf("%s absent from /proc/net/xfrm_stat", name)
	return 0
}

// delOutDelta sends ONE datagram through the selector and reports how far the counter moved.
//
// The write error is returned, never fatal. A PROTECT policy that matches and resolves to
// no state answers the sender EAGAIN, so a failed write is one of the EXPECTED outcomes of
// a match; when nothing matches the write succeeds and the datagram leaves unprotected.
// Neither is the measurement.
func delOutDelta(t *testing.T) (int, error) {
	t.Helper()
	before := delStat(t, delStatOutNoStates)

	// A fresh, unconnected socket per probe: a connected socket caches its route, and a
	// cached bundle would answer the next send without consulting the policy.
	var lc net.ListenConfig
	pc, err := lc.ListenPacket(t.Context(), "udp", net.JoinHostPort(delLocalAddr, "0"))
	if err != nil {
		t.Fatalf("bind udp on %s: %v", delLocalAddr, err)
	}
	_, writeErr := pc.WriteTo([]byte("ze-child-delete-probe"),
		&net.UDPAddr{IP: net.ParseIP(delRemoteAddr), Port: delProbePort})
	if cerr := pc.Close(); cerr != nil {
		t.Errorf("close probe socket: %v", cerr)
	}
	return delStat(t, delStatOutNoStates) - before, writeErr
}

func delAssertClaimed(t *testing.T, what string) {
	t.Helper()
	delta, writeErr := delOutDelta(t)
	if delta < 1 {
		t.Fatalf("%s: %s did not move (delta=%d, write=%v); no policy claimed the datagram, so the traffic left IN THE CLEAR",
			what, delStatOutNoStates, delta, writeErr)
	}
	t.Logf("%s: %s +%d (write=%v) -- the flow was handed to IPsec", what, delStatOutNoStates, delta, writeErr)
}

func delAssertUnclaimed(t *testing.T, what string) {
	t.Helper()
	delta, writeErr := delOutDelta(t)
	if delta != 0 {
		t.Fatalf("%s: %s moved by %d (write=%v); a policy still claimed the datagram",
			what, delStatOutNoStates, delta, writeErr)
	}
	t.Logf("%s: %s +0 (write=%v) -- the flow left unprotected", what, delStatOutNoStates, writeErr)
}

// delChild builds a Child SA on the probe's selector.
//
// IfID is 0, and that is load-bearing. __xfrm_policy_match requires pol->if_id to equal
// the flow's, and ordinary traffic carries 0, so a policy installed with a non-zero IfID
// binds to an XFRM interface and claims nothing this probe sends.
func delChild(t *testing.T, inSPI, outSPI uint32) *ChildSA {
	t.Helper()
	_, local, err := net.ParseCIDR(delLocalCIDR)
	if err != nil {
		t.Fatalf("parse %s: %v", delLocalCIDR, err)
	}
	_, remote, err := net.ParseCIDR(delRemoteCIDR)
	if err != nil {
		t.Fatalf("parse %s: %v", delRemoteCIDR, err)
	}
	return &ChildSA{
		InboundSPI:  inSPI,
		OutboundSPI: outSPI,
		LocalAddr:   net.ParseIP("192.0.2.10"),
		RemoteAddr:  net.ParseIP("192.0.2.20"),
		TSLocal:     local,
		TSRemote:    remote,
		Owner:       delOwner,
		Mode:        modeTunnel,
		ReqID:       defaultReqID,
	}
}

// delInstallPolicies installs one Child SA's two policies through the production selector
// builder, so the delete selector this test exercises is the install selector by
// construction. No STATE is installed: the counter needs a policy that resolves to none.
func delInstallPolicies(t *testing.T, dp dataplane.Dataplane, child *ChildSA) {
	t.Helper()
	for _, dir := range []dataplane.SADir{dataplane.SADirIn, dataplane.SADirOut} {
		if err := dp.InstallPolicy(childPolicyParams(child, dir)); err != nil {
			if strings.Contains(err.Error(), "operation not permitted") ||
				strings.Contains(err.Error(), "permission denied") {
				t.Skipf("XFRM policy install needs CAP_NET_ADMIN: %v", err)
			}
			t.Fatalf("install the %v policy: %v", dir, err)
		}
	}
}

// TestPeerDeleteKeepsKernelPolicyForPendingChild is the kernel measurement behind
// TestPeerDeleteOfLiveChildKeepsPolicyForPendingChild.
//
// A peer's ESP Delete naming the live Child SA arrives while a parallel re-initiation's
// Child SA is installed on the same selector, so the kernel holds ONE policy per direction
// shared by both. The Delete must leave it, and the surviving policy must still claim
// traffic. Before the fix, closeDesignatedChildSAs removed it and the promoted Child SA
// came up with states and no policy.
func TestPeerDeleteKeepsKernelPolicyForPendingChild(t *testing.T) {
	if !delOwnProcess(t) {
		return
	}
	delNetns(t)

	if err := dataplane.Load("xfrm"); err != nil {
		t.Fatalf("load the xfrm backend: %v", err)
	}
	t.Cleanup(func() {
		if err := dataplane.CloseBackend(); err != nil {
			t.Errorf("close the xfrm backend: %v", err)
		}
	})
	dp := dataplane.Get()
	if dp == nil {
		t.Fatal("the xfrm backend loaded but Get returned nil")
	}
	log := slogutil.DiscardLogger()
	ps := &PeerSession{peerName: delOwner}

	live := delChild(t, 0x1001, 0x1002)
	delInstallPolicies(t, dp, live)
	ps.setChildSA(live)

	// finishResponderEstablish installed this one before the supersede token reached the
	// owner loop. Its policies UPSERT the live pair's selector, so the kernel is left
	// holding one policy per direction for two Child SAs.
	pending := delChild(t, 0x2001, 0x2002)
	delInstallPolicies(t, dp, pending)
	ps.setPendingChild(pending)

	held := delListPolicies(t)
	if len(held) != 2 {
		t.Fatalf("after two installs of one selector the kernel holds %d policies, want 2 (one per direction): %v", len(held), held)
	}

	// The positive control. Without it a broken fixture would read as "the policy was
	// correctly absent" and every assertion below would pass vacuously.
	delAssertClaimed(t, "before the peer's Delete")

	// RFC 7296 Section 1.4.1: the peer names the SPI it expects in its own inbound
	// packets, which is this node's OUTBOUND half.
	_, down := ps.closeDesignatedChildSAs(espDeletePayload([]uint32{live.OutboundSPI}), dp, log)
	if !down {
		t.Fatal("the peer's Delete of the live Child SA did not report the session child down")
	}

	delAssertClaimed(t, "after the peer's Delete of the live Child SA")

	after := delListPolicies(t)
	if len(after) != 2 {
		t.Fatalf("the Delete left %d policies, want the 2 the pending Child SA still answers to: %v", len(after), after)
	}

	// The negative control. Removing the survivor's policies for real must SILENCE the
	// counter; if it does not, the readings above were never about these policies.
	for _, dir := range []dataplane.SADir{dataplane.SADirIn, dataplane.SADirOut} {
		if err := dp.RemovePolicyParams(childPolicyParams(pending, dir)); err != nil {
			t.Fatalf("removing the surviving %v policy: %v", dir, err)
		}
	}
	delAssertUnclaimed(t, "after the surviving policies are removed for real")

	if left := delListPolicies(t); len(left) != 0 {
		t.Errorf("the probe left %d XFRM policies behind: %v", len(left), left)
	}
}
