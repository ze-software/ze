// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- protected-host forwarding.
// Related: rfc4301_sad_selector_linux_test.go -- isolated namespace and authenticated ESP sender.
//
//go:build linux

package engine

import (
	"encoding/binary"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/slogutil"
)

const (
	forwardKernelLink = "ze-fwd-host"
	forwardKernelHost = "10.2.0.2"
	forwardKernelIfID = 42
)

// TestChildTunnelForwardsProtectedHostTraffic sends real authenticated ESP to a
// nonlocal protected destination. The dummy egress's packet counter measures
// forwarding, not decryption or installation. Removing only FWD proves the drop
// detector, and restoring it must restore forwarding without replacing the SA.
// A refused rekey must preserve that same protected-host path on the old SA.
func TestChildTunnelForwardsProtectedHostTraffic(t *testing.T) {
	if !sadSelOwnNamespace(t) {
		return
	}
	sadSelNetns(t)
	// Set forwarding before per-interface defaults: Linux changes router defaults
	// when ip_forward changes. Disable IPv6 before link creation so the egress
	// counter cannot include address-configuration traffic.
	for _, setting := range []struct{ path, value string }{
		{"/proc/sys/net/ipv4/ip_forward", "1"},
		{"/proc/sys/net/ipv4/conf/all/rp_filter", "0"},
		{"/proc/sys/net/ipv4/conf/default/rp_filter", "0"},
		{"/proc/sys/net/ipv4/conf/lo/rp_filter", "0"},
		{"/proc/sys/net/ipv6/conf/default/disable_ipv6", "1"},
	} {
		if err := os.WriteFile(setting.path, []byte(setting.value), 0o600); err != nil {
			t.Fatalf("set isolated router %s: %v", setting.path, err)
		}
	}
	for _, link := range []netlink.Link{
		&netlink.Dummy{Name: forwardKernelLink},
		&netlink.Xfrmi{Name: "ze-fwd-xfrm", Ifid: forwardKernelIfID},
	} {
		if err := netlink.LinkAdd(link); err != nil {
			t.Fatal(err)
		}
		if err := netlink.LinkSetUp(link); err != nil {
			t.Fatal(err)
		}
	}
	egress, err := netlink.LinkByName(forwardKernelLink)
	if err != nil {
		t.Fatal(err)
	}
	if err := netlink.RouteAdd(&netlink.Route{LinkIndex: egress.Attrs().Index,
		Dst: mustCIDRNet(t, forwardKernelHost+"/32"), Scope: netlink.SCOPE_LINK}); err != nil {
		t.Fatal(err)
	}
	child := forwardKernelChild(t)
	sender := newSadSelSender(t, child)
	packet := sadSelIPv4(sadSelInnerRemote, sadSelProtoUDP, sadSelUDP(sadSelPort))
	copy(packet[16:20], net.ParseIP(forwardKernelHost).To4())
	binary.BigEndian.PutUint16(packet[10:12], 0)
	binary.BigEndian.PutUint16(packet[10:12], sadSelChecksum(packet[:20]))

	before := forwardKernelPackets(t)
	noPolicy := sadSelStat(t, sadSelStatNoPols)
	sender.send(t, sadSelProtoIPv4, packet)
	awaitForwardKernelCounter(t, "protected packet forwarding", before+1, forwardKernelPackets)
	if got := sadSelStat(t, sadSelStatNoPols); got != noPolicy {
		t.Fatalf("valid protected traffic changed XfrmInNoPols: %d -> %d", noPolicy, got)
	}

	replacement := newRekeyedChild(child, 0x71950001, 0x71950002, child.Keys, true, nil, child.Mode)
	refusing := &forwardKernelRefusalDP{Dataplane: dataplane.Get()}
	err = installChildSA(replacement, ipsec.ESPProposal{Number: 1, Encryption: ipsec.EncryptionAES256GCM},
		refusing, slogutil.DiscardLogger())
	if !errors.Is(err, errForwardPolicyInstall) {
		t.Fatalf("replacement error=%v, want outbound-policy refusal", err)
	}
	sender.send(t, sadSelProtoIPv4, packet)
	awaitForwardKernelCounter(t, "protected forwarding after refused rekey", before+2, forwardKernelPackets)
	if got := sadSelStat(t, sadSelStatNoPols); got != noPolicy {
		t.Fatalf("refused rekey broke the old receive policy: XfrmInNoPols=%d", got)
	}

	forward := childPolicies(child, dataplane.SADirFwd)[0]
	if err := dataplane.Get().RemovePolicyParams(forward); err != nil {
		t.Fatal(err)
	}
	sender.send(t, sadSelProtoIPv4, packet)
	awaitForwardKernelCounter(t, "missing forward-policy rejection", uint64(noPolicy+1),
		func(t *testing.T) uint64 { return uint64(sadSelStat(t, sadSelStatNoPols)) })
	if got := forwardKernelPackets(t); got != before+2 {
		t.Fatalf("packet without a forward PROTECT policy reached egress: %d -> %d", before+2, got)
	}
	if err := dataplane.Get().InstallPolicy(forward); err != nil {
		t.Fatal(err)
	}
	sender.send(t, sadSelProtoIPv4, packet)
	awaitForwardKernelCounter(t, "restored protected packet forwarding", before+3, forwardKernelPackets)
	if got := sadSelStat(t, sadSelStatNoPols); got != noPolicy+1 {
		t.Fatalf("restoring the policy did not stop rejection: XfrmInNoPols=%d", got)
	}
	removeChildSA(child, dataplane.Get(), slogutil.DiscardLogger())
	policies, err := dataplane.Get().ListPolicies()
	if err != nil {
		t.Fatal(err)
	}
	for _, policy := range policies {
		if policy.IfID == forwardKernelIfID {
			t.Fatalf("Child teardown left policy direction %v installed", policy.Dir)
		}
	}
	t.Logf("protected egress packets: %d -> %d; missing-policy drops: %d -> %d",
		before, forwardKernelPackets(t), noPolicy, sadSelStat(t, sadSelStatNoPols))
}

type forwardKernelRefusalDP struct{ dataplane.Dataplane }

func (d *forwardKernelRefusalDP) InstallPolicy(policy dataplane.SPParams) error {
	if policy.Dir == dataplane.SADirOut {
		return errForwardPolicyInstall
	}
	return d.Dataplane.InstallPolicy(policy)
}

func forwardKernelChild(t *testing.T) *ChildSA {
	t.Helper()
	if err := dataplane.Load("xfrm"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := dataplane.CloseBackend(); err != nil {
			t.Error(err)
		}
	})
	sa := testSA()
	sa.IsInitiator = true
	sa.NegotiatedTSi = mustCIDRNet(t, "10.2.0.0/24")
	sa.NegotiatedTSr = mustCIDRNet(t, "10.1.0.0/24")
	sa.NegotiatedPairs = []tsPair{{
		I: tsSelector{Net: sa.NegotiatedTSi, Port: ipsec.PortSelector{Form: ipsec.PortSingle, Port: sadSelPort}, Proto: sadSelProtoUDP},
		R: tsSelector{Net: sa.NegotiatedTSr, Proto: sadSelProtoUDP},
	}}
	group := ipsec.ESPGroup{Name: "forward-gcm", Lifetime: 3600, PFS: ipsec.PFSDisable,
		Proposals: []ipsec.ESPProposal{{Number: 1, Encryption: ipsec.EncryptionAES256GCM}}}
	child, err := createFirstChildSA(sa, group, sadSelLocalOuter, sadSelRemoteOuter,
		forwardKernelIfID, dataplane.Get(), slogutil.DiscardLogger())
	if err != nil {
		t.Fatal(err)
	}
	if !child.ESPInstalled {
		t.Fatal("Child SA was not installed into the real kernel")
	}
	t.Cleanup(child.Clear)
	return child
}

func forwardKernelPackets(t *testing.T) uint64 {
	t.Helper()
	link, err := netlink.LinkByName(forwardKernelLink)
	if err != nil {
		t.Fatal(err)
	}
	if link.Attrs().Statistics == nil {
		t.Fatal("kernel returned no egress counters")
	}
	return link.Attrs().Statistics.TxPackets
}

func awaitForwardKernelCounter(t *testing.T, name string, want uint64, read func(*testing.T) uint64) {
	t.Helper()
	var got uint64
	for range 100 {
		got = read(t)
		if got == want {
			return
		}
		if got > want {
			t.Fatalf("%s counter=%d, want %d", name, got, want)
		}
		// sleep(poll-interval): wait for the kernel's packet-processing counter.
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s counter=%d, want %d (XfrmInNoPols=%d)", name, got, want,
		sadSelStat(t, sadSelStatNoPols))
}
