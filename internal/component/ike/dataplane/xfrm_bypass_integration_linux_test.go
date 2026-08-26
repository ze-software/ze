// VALIDATES: that a real Linux XFRM stack accepts the IKE bypass beside a Child SA
// policy whose selector covers the same flow, and RESOLVES the contest in the
// bypass's favor. The unit tests assert the two priority NUMBERS; only the kernel
// can say which policy it actually applies, and the ordering rule (lowest number
// wins, ties broken by insertion order) is kernel behavior that no amount of
// reading proves.
// PREVENTS: shipping the priority pair on reasoning alone. If the kernel ranked the
// other way, or ignored priority for a template-free policy, every one of the unit
// tests would still be green and every tunnel with a wide selector would still be
// broken.

//go:build integration && linux

package dataplane

import (
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// The test range. RFC 5737 TEST-NET-2, carried on a dummy link this test creates and
// deletes, so the ESP policy below can never capture the host's real traffic.
//
// The ESP policy is deliberately scoped to this /24 rather than to 0.0.0.0/0. The
// production defect involves a wildcard, but the MECHANISM under test is a selector
// that covers the IKE endpoints, and a genuine 0.0.0.0/0 policy demanding a state
// that does not exist would black-hole the whole VM, including the 9p mount this
// test's source is read through. The BYPASS is left exactly as production builds it,
// wildcard and all, because that is the half whose shape is being proven.
const (
	bypassTestLink = "zebyp0"
	bypassTestCIDR = "198.51.100.2/24"
	bypassTestSelf = "198.51.100.2"
	bypassTestPeer = "198.51.100.1"
)

// espPolicyCapturingTestRange is the Child SA policy shape installChildSA emits,
// narrowed to the test range and pointed at tunnel endpoints for which no state is
// ever installed. A flow it captures therefore CANNOT be sent: the kernel finds the
// policy, fails to resolve a state, and refuses the write. That refusal is the
// signal this test reads.
func espPolicyCapturingTestRange() SPParams {
	_, testRange, _ := net.ParseCIDR("198.51.100.0/24")
	_, anyV4, _ := net.ParseCIDR("0.0.0.0/0")
	return SPParams{
		Src:       anyV4,
		Dst:       testRange,
		Dir:       SADirOut,
		Proto:     ProtoESP,
		Mode:      ModeTunnel,
		ReqID:     0x7e52,
		Priority:  PriorityChildSA,
		TunnelSrc: net.ParseIP("192.0.2.10"),
		TunnelDst: net.ParseIP("192.0.2.20"),
	}
}

// ikeBypassOut is the outbound half of the production bypass for UDP 500. It is
// written out here rather than imported, because internal/component/ike/engine
// (which builds it) imports this package and the dependency cannot run backwards.
// TestIKEBypassPoliciesSelectorAndPriority pins the engine-side shape; this pins what
// the kernel does with it.
func ikeBypassOut(port uint16) SPParams {
	_, anyV4, _ := net.ParseCIDR("0.0.0.0/0")
	return SPParams{
		Src:        anyV4,
		Dst:        anyV4,
		Dir:        SADirOut,
		Action:     SPActionBypass,
		Priority:   PriorityIKEBypass,
		UpperProto: unix.IPPROTO_UDP,
		SrcPort:    ExactPortMatch(port),
		DstPort:    AnyPortMatch(),
	}
}

// xfrmOutNoStates reads the kernel's XfrmOutNoStates counter.
//
// THIS, not the return value of the write, is the instrument. MEASURED in the QEMU
// VM: with net.core.xfrm_larval_drop = 1 (the default there) an outbound policy that
// resolves to no state DROPS the packet and still lets sendto() report success, so a
// successful write says nothing at all about whether the datagram left the host. The
// counter is what moves. It is also the same family of counter the original field
// defect was measured with (XfrmInTmplMismatch on the receiving side).
func xfrmOutNoStates(t *testing.T) int {
	t.Helper()
	raw, err := os.ReadFile("/proc/net/xfrm_stat")
	if err != nil {
		t.Fatalf("reading /proc/net/xfrm_stat: %v; it is the instrument this test measures with", err)
	}
	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "XfrmOutNoStates" {
			n, convErr := strconv.Atoi(fields[1])
			if convErr != nil {
				t.Fatalf("parsing XfrmOutNoStates %q: %v", fields[1], convErr)
			}
			return n
		}
	}
	t.Fatal("XfrmOutNoStates is absent from /proc/net/xfrm_stat")
	return 0
}

// sendFromPort binds a UDP socket to the given local port on the dummy link, writes
// one datagram into the test range, and reports how far the kernel's
// XfrmOutNoStates counter moved. A non-zero delta means the datagram was captured by
// a policy that resolved to no state, and was therefore dropped rather than sent.
func sendFromPort(t *testing.T, port int) int {
	t.Helper()
	before := xfrmOutNoStates(t)
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP(bypassTestSelf), Port: port})
	if err != nil {
		t.Skipf("cannot bind UDP %s:%d in this environment: %v", bypassTestSelf, port, err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.WriteToUDP([]byte("ze-bypass-probe"), &net.UDPAddr{IP: net.ParseIP(bypassTestPeer), Port: 9}); err != nil {
		// Not the signal, but worth recording if it ever happens.
		t.Logf("write from port %d returned %v", port, err)
	}
	return xfrmOutNoStates(t) - before
}

func setupBypassTestLink(t *testing.T) {
	t.Helper()
	link := &netlink.Dummy{Name: bypassTestLink}
	if err := netlink.LinkAdd(link); err != nil {
		if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
			t.Skipf("creating a dummy link needs CAP_NET_ADMIN: %v", err)
		}
		t.Fatalf("add dummy link %s: %v", bypassTestLink, err)
	}
	t.Cleanup(func() { _ = netlink.LinkDel(link) })

	addr, err := netlink.ParseAddr(bypassTestCIDR)
	if err != nil {
		t.Fatalf("parse %s: %v", bypassTestCIDR, err)
	}
	if err := netlink.AddrAdd(link, addr); err != nil {
		t.Fatalf("add %s to %s: %v", bypassTestCIDR, bypassTestLink, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("set %s up: %v", bypassTestLink, err)
	}
}

// TestXFRMIKEBypassOutranksChildSAPolicyInKernel measures the ordering contest.
func TestXFRMIKEBypassOutranksChildSAPolicyInKernel(t *testing.T) {
	setupBypassTestLink(t)
	b := &xfrmBackend{}

	esp := espPolicyCapturingTestRange()
	if err := b.InstallPolicy(esp); err != nil {
		if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
			t.Skipf("XFRM policy install needs CAP_NET_ADMIN: %v", err)
		}
		t.Fatalf("install the Child SA policy: %v", err)
	}
	t.Cleanup(func() { _ = b.RemovePolicyParams(esp) })

	// VACUITY GUARD. Everything below reads "the counter did not move" as "the
	// bypass won". That inference is worthless unless a captured flow genuinely
	// moves it first. If this send does not, the ESP policy is not capturing the
	// range and the rest of the test proves nothing, so it is a hard failure.
	if delta := sendFromPort(t, 12345); delta == 0 {
		t.Fatal("a UDP write into the test range did not move XfrmOutNoStates while a " +
			"Child SA policy with no resolvable state covered it; the policy is not " +
			"capturing the flow, so this test cannot tell a winning bypass from an absent one")
	} else {
		t.Logf("control: captured flow from port 12345 moved XfrmOutNoStates by %d, as required", delta)
	}

	// Now the exemption, in the exact shape production installs.
	bypass := ikeBypassOut(500)
	if err := b.InstallPolicy(bypass); err != nil {
		t.Fatalf("install the IKE bypass beside the Child SA policy: %v", err)
	}
	t.Cleanup(func() { _ = b.RemovePolicyParams(bypass) })

	// THE MEASUREMENT. Both policies match this flow. The bypass carries the lower
	// priority number, so the kernel must apply it and leave the datagram alone.
	if delta := sendFromPort(t, 500); delta != 0 {
		t.Fatalf("a UDP write from local port 500 moved XfrmOutNoStates by %d; the Child SA "+
			"policy outranked the IKE bypass, so ze's own IKE traffic is handed to ESP "+
			"and the tunnel can never be rekeyed or torn down", delta)
	}
	t.Log("measured: the kernel applied the bypass to the port-500 flow and did not hand it to ESP")

	// THE BYPASS DID NOT WIDEN. The captured flow on another port must still be
	// dropped, or the exemption covers more than ze's own IKE sockets.
	if delta := sendFromPort(t, 12345); delta == 0 {
		t.Error("after installing the IKE bypass, a flow from port 12345 stopped being " +
			"captured; the exemption is wider than ze's own IKE ports")
	}
}

// TestXFRMBypassIsStoredWithoutTemplate checks what the kernel actually recorded:
// the bypass must be an ALLOW policy with NO template, and the two priorities must
// survive the round trip. A bypass stored WITH a template is a protect policy, and
// it would black-hole every flow it matched instead of exempting it.
func TestXFRMBypassIsStoredWithoutTemplate(t *testing.T) {
	b := &xfrmBackend{}

	bypass := ikeBypassOut(4500)
	if err := b.InstallPolicy(bypass); err != nil {
		if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
			t.Skipf("XFRM policy install needs CAP_NET_ADMIN: %v", err)
		}
		t.Fatalf("install the IKE bypass: %v", err)
	}
	t.Cleanup(func() { _ = b.RemovePolicyParams(bypass) })

	policies, err := netlink.XfrmPolicyList(netlink.FAMILY_ALL)
	if err != nil {
		t.Fatalf("list policies: %v", err)
	}
	var found *netlink.XfrmPolicy
	for i := range policies {
		p := &policies[i]
		if p.Dir == netlink.XFRM_DIR_OUT && p.SrcPort == 4500 && p.Proto == unix.IPPROTO_UDP {
			found = p
			break
		}
	}
	if found == nil {
		t.Fatal("the installed bypass is not in the kernel's policy list")
	}
	if found.Action != netlink.XFRM_POLICY_ALLOW {
		t.Errorf("bypass action = %s, want allow; a block policy would DROP ze's IKE instead of exempting it", found.Action)
	}
	if len(found.Tmpls) != 0 {
		t.Errorf("bypass stored with %d template(s), want 0; a template makes it a protect policy and the kernel would demand a state for ze's own IKE", len(found.Tmpls))
	}
	if found.Priority != PriorityIKEBypass {
		t.Errorf("bypass priority round-tripped as %d, want %d", found.Priority, PriorityIKEBypass)
	}
	if found.DstPort != 0 {
		t.Errorf("bypass pinned remote port %d; a NAT rewrites it and the exemption would stop matching", found.DstPort)
	}
}

// TestXFRMBypassInstallIsIdempotent proves the peer-independence claim in
// installIKEBypass: every peer installs the same set, so the second install must
// succeed rather than answer EEXIST the way a Child SA policy does
// (TestXFRMSecondInstallOfOneSelectorIsRefused measures that contrast).
func TestXFRMBypassInstallIsIdempotent(t *testing.T) {
	b := &xfrmBackend{}
	bypass := ikeBypassOut(500)

	if err := b.InstallPolicy(bypass); err != nil {
		if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
			t.Skipf("XFRM policy install needs CAP_NET_ADMIN: %v", err)
		}
		t.Fatalf("first install: %v", err)
	}
	t.Cleanup(func() { _ = b.RemovePolicyParams(bypass) })

	if err := b.InstallPolicy(bypass); err != nil {
		t.Fatalf("second install of the same bypass failed (%v); a second peer would "+
			"fail to start because the first already installed the shared exemption", err)
	}
}
