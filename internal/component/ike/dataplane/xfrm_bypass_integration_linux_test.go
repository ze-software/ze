// VALIDATES: that a real Linux XFRM stack accepts an operator BYPASS entry beside a
// Child SA policy whose selector covers the same flow, and RESOLVES the contest by the
// operator's order: an entry ordered ahead of PriorityChildSA takes the flow. The unit
// tests assert the two priority NUMBERS; only the kernel can say which policy it
// actually applies, and the ordering rule (lowest number wins, ties broken by
// insertion order) is kernel behavior that no amount of reading proves.
// PREVENTS: shipping RFC 4301 Section 4.4.1's ordered SPD on reasoning alone. If the
// kernel ranked the other way, or ignored priority for a template-free policy, every
// unit test would still be green and an operator BYPASS ordered ahead of a wide Child
// SA selector would hand its traffic to ESP.

//go:build linux

package dataplane

import (
	"net"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

// The test range. RFC 5737 TEST-NET-2, carried on a dummy link this test creates and
// deletes, so the ESP policy below can never capture the host's real traffic.
//
// The ESP policy is deliberately scoped to this /24 rather than to 0.0.0.0/0. A
// genuine 0.0.0.0/0 policy demanding a state that does not exist would black-hole the
// whole VM, including the 9p mount this test's source is read through. The operator
// BYPASS keeps the wildcard addresses an operator writes for a port-scoped exemption,
// because its selector is the half whose shape is being proven.
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

// operatorBypassOut is the outbound half of an operator `vpn ipsec policy` BYPASS
// entry of local port port, every address and every remote port, at order
// testOperatorOrder. It is written out here rather than built by spdPolicyDirection,
// because internal/component/ike/engine (which builds it) imports this package and
// the dependency cannot run backwards. The engine's spd_policy tests pin the shape it
// builds; this pins what the kernel does with it.
func operatorBypassOut(port uint16) SPParams {
	_, anyV4, _ := net.ParseCIDR("0.0.0.0/0")
	return SPParams{
		Src:        anyV4,
		Dst:        anyV4,
		Dir:        SADirOut,
		Action:     SPActionBypass,
		Role:       PolicyRoleOperator,
		Priority:   testOperatorOrder,
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
		t.Fatalf("bind UDP %s:%d on the test's own dummy link: %v", bypassTestSelf, port, err)
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

// TestXFRMOperatorBypassOutranksChildSAPolicyInKernel measures the ordering contest
// between an operator BYPASS ordered ahead of a Child SA policy and that policy, on a
// flow both select.
func TestXFRMOperatorBypassOutranksChildSAPolicyInKernel(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	setupBypassTestLink(t)
	b := &xfrmBackend{}

	esp := espPolicyCapturingTestRange()
	if err := b.InstallPolicy(esp); err != nil {
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

	// Now the operator's exemption, of local port 500.
	bypass := operatorBypassOut(500)
	if err := b.InstallPolicy(bypass); err != nil {
		t.Fatalf("install the operator BYPASS beside the Child SA policy: %v", err)
	}
	t.Cleanup(func() { _ = b.RemovePolicyParams(bypass) })

	// THE MEASUREMENT. Both policies match this flow. The bypass carries the lower
	// priority number, so the kernel must apply it and leave the datagram alone.
	if delta := sendFromPort(t, 500); delta != 0 {
		t.Fatalf("a UDP write from local port 500 moved XfrmOutNoStates by %d; the Child SA "+
			"policy outranked the operator BYPASS ordered ahead of it, so the SPD's order "+
			"is not the kernel's", delta)
	}
	t.Log("measured: the kernel applied the operator BYPASS to the port-500 flow and did not hand it to ESP")

	// THE BYPASS DID NOT WIDEN. The captured flow on another port must still be
	// dropped, or the exemption covers more than the port the operator named.
	if delta := sendFromPort(t, 12345); delta == 0 {
		t.Error("after installing the operator BYPASS, a flow from port 12345 stopped being " +
			"captured; the exemption is wider than the port it names")
	}
}

// TestXFRMBypassIsStoredWithoutTemplate checks what the kernel actually recorded for
// an operator BYPASS: an ALLOW policy with NO template, its order surviving the round
// trip, and the remote port left free. A bypass stored WITH a template is a protect
// policy, and it would black-hole every flow it matched instead of exempting it.
func TestXFRMBypassIsStoredWithoutTemplate(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	b := &xfrmBackend{}

	bypass := operatorBypassOut(4500)
	if err := b.InstallPolicy(bypass); err != nil {
		t.Fatalf("install the operator BYPASS: %v", err)
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
		t.Fatal("the installed operator BYPASS is not in the kernel's policy list")
	}
	if found.Action != netlink.XFRM_POLICY_ALLOW {
		t.Errorf("bypass action = %s, want allow; a block policy would DROP the traffic the operator exempted", found.Action)
	}
	if len(found.Tmpls) != 0 {
		t.Errorf("bypass stored with %d template(s), want 0; a template makes it a protect policy and the kernel would demand a state for the exempted traffic", len(found.Tmpls))
	}
	if found.Priority != testOperatorOrder {
		t.Errorf("bypass priority round-tripped as %d, want the operator's order %d", found.Priority, testOperatorOrder)
	}
	if found.DstPort != 0 {
		t.Errorf("bypass pinned remote port %d; the entry names its local port alone", found.DstPort)
	}
}

// TestXFRMOperatorEntryReinstallReplacesInPlace proves that an operator entry the
// kernel already holds is replaced, not refused with EEXIST the way an exclusive add
// answers (TestPolicyOwnerRefusesASecondPeerOnOneSelector covers the refusal a
// different owner's install gets). Two installs reach one selector in production: a
// reload that edits an entry's order or action and keeps its selector installs the
// new form over the old (reconcileSPDPolicies), and the undo of a refused reload puts
// a running entry back over itself (undoSPDInstall).
//
// METHOD: install one operator BYPASS twice, then its edited form at another order,
// and read the kernel's policy list: it must hold one policy at that selector, at the
// edited order.
func TestXFRMOperatorEntryReinstallReplacesInPlace(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	b := &xfrmBackend{}
	bypass := operatorBypassOut(500)

	if err := b.InstallPolicy(bypass); err != nil {
		t.Fatalf("first install: %v", err)
	}
	t.Cleanup(func() { _ = b.RemovePolicyParams(bypass) })

	if err := b.InstallPolicy(bypass); err != nil {
		t.Fatalf("second install of the same operator entry failed (%v); the undo of a "+
			"refused reload could not put the running entry back", err)
	}

	edited := bypass
	edited.Priority = testOperatorOrder + 1
	if err := b.InstallPolicy(edited); err != nil {
		t.Fatalf("install of the edited entry over its old form failed (%v); a reload "+
			"that changes only an entry's order would be refused", err)
	}

	policies, err := netlink.XfrmPolicyList(netlink.FAMILY_ALL)
	if err != nil {
		t.Fatalf("list policies: %v", err)
	}
	var priorities []int
	for i := range policies {
		p := &policies[i]
		if p.Dir == netlink.XFRM_DIR_OUT && p.SrcPort == 500 && p.Proto == unix.IPPROTO_UDP {
			priorities = append(priorities, p.Priority)
		}
	}
	if len(priorities) != 1 || priorities[0] != edited.Priority {
		t.Fatalf("the kernel holds the entry's selector at priorities %v, want exactly [%d]: "+
			"the edit added a policy beside the old form instead of replacing it",
			priorities, edited.Priority)
	}
}
