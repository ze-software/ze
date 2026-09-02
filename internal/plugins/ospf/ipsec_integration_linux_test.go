//go:build integration && linux

// VALIDATES: spec-ospf-ext-16 AC-2/AC-7/AC-10/AC-13 + A-1/A-6/R-2b -- the RFC 4552
// installer and the extended XFRM dataplane program real kernel state: an AH transport SA
// with integrity only (no encryption), a proto-89 transport-mode policy, and the full
// install/remove lifecycle visible in `ip xfrm`. Requires CAP_NET_ADMIN (QEMU/root).
// PREVENTS: a malformed AH state, a policy that matches all traffic instead of proto 89,
// and orphaned XFRM state after interface down.
package ospf

import (
	"net"
	"net/netip"
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

// requireXFRM skips unless the kernel XFRM stack is usable (CAP_NET_ADMIN).
func requireXFRM(t *testing.T) dataplane.Dataplane {
	t.Helper()
	if _, err := netlink.XfrmPolicyList(netlink.FAMILY_ALL); err != nil {
		t.Skipf("requires CAP_NET_ADMIN / kernel XFRM: %v", err)
	}
	if err := dataplane.Load("xfrm"); err != nil {
		t.Skipf("xfrm backend load: %v", err)
	}
	dp := dataplane.Get()
	if dp == nil {
		t.Skip("no active dataplane after load")
	}
	return dp
}

func TestXFRMInstallAHSA(t *testing.T) {
	dp := requireXFRM(t)
	sa := dataplane.SAParams{
		SPI:      0x4552a1,
		Src:      net.ParseIP("fe80::1"),
		Dst:      net.ParseIP("ff02::5"),
		Proto:    dataplane.ProtoAH,
		Mode:     dataplane.ModeTransport,
		AuthAlgo: "sha256",
		AuthKey:  make([]byte, 32),
	}
	if err := dp.InstallSA(sa); err != nil {
		t.Fatalf("InstallSA(AH): %v", err)
	}
	defer func() { _ = dp.RemoveSA(sa.SPI, sa.Dst, sa.Proto) }()

	states, err := netlink.XfrmStateList(netlink.FAMILY_ALL)
	if err != nil {
		t.Fatalf("XfrmStateList: %v", err)
	}
	var found *netlink.XfrmState
	for i := range states {
		if states[i].Spi == int(sa.SPI) {
			found = &states[i]
		}
	}
	if found == nil {
		t.Fatal("installed AH SA not found in kernel")
	}
	if found.Auth == nil {
		t.Error("AH SA has no integrity transform")
	}
	if found.Crypt != nil {
		t.Error("AH SA must not carry an encryption transform (RFC 4302)")
	}
	// The kernel reports XFRM_MODE_TRANSPORT as 0. dataplane.ModeTransport is 1, because
	// Ze's constants are 1-based so an unset Mode field is never a valid mode
	// (dataplane.go). Comparing the two directly asserted 0 == 1 and could never pass for
	// a correctly installed transport SA.
	//
	// This is the two-vocabulary hazard kernelXFRMMode's own comment records, reproduced
	// in a test: the conversion exists precisely because the numbering differs, so the
	// read-back must be compared against the KERNEL number.
	const kernelTransportMode = 0 // XFRM_MODE_TRANSPORT, uapi/linux/xfrm.h
	if int(found.Mode) != kernelTransportMode {
		t.Errorf("AH SA reached the kernel as mode %d, want XFRM_MODE_TRANSPORT (%d)",
			found.Mode, kernelTransportMode)
	}
}

func TestXFRMTransportPolicyUpperProto(t *testing.T) {
	dp := requireXFRM(t)
	anyNet := &net.IPNet{IP: net.IPv6zero, Mask: net.CIDRMask(0, 128)}
	llNet := &net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(128, 128)}
	p := dataplane.SPParams{
		Src:        llNet,
		Dst:        anyNet,
		Dir:        dataplane.SADirOut,
		Proto:      dataplane.ProtoESP,
		Mode:       dataplane.ModeTransport,
		ReqID:      0x4552,
		UpperProto: 89,
	}
	if err := dp.InstallPolicy(p); err != nil {
		t.Fatalf("InstallPolicy(proto 89): %v", err)
	}
	defer func() { _ = dp.RemovePolicyParams(p) }()

	pols, err := netlink.XfrmPolicyList(netlink.FAMILY_ALL)
	if err != nil {
		t.Fatalf("XfrmPolicyList: %v", err)
	}
	var found bool
	for i := range pols {
		if int(pols[i].Proto) == 89 && pols[i].Dir == netlink.Dir(dataplane.SADirOut-1) {
			found = true
		}
	}
	if !found {
		t.Fatal("proto-89 transport-mode policy not found in kernel")
	}
}

func TestIPsecInstallerXFRMEndToEnd(t *testing.T) {
	requireXFRM(t)
	inst := newIPsecInstaller(nil, nil)
	inst.setTransportSource(func(string) (netip.Addr, int, bool) {
		return netip.MustParseAddr("fe80::1"), 1, true
	})
	inst.setConfig([]interfaceConfig{{
		Name:  "ipsec-itest",
		IPsec: &ipsecInterfaceConfig{SPI: 0x4552b2, Protocol: "esp", AuthAlgo: "sha256", AuthKey: hexKey(32)},
	}})

	inst.onInterfaceUp(1, "ipsec-itest")
	if _, ok := inst.status("ipsec-itest"); !ok {
		t.Fatal("installer did not record an installed SA")
	}
	states, err := netlink.XfrmStateList(netlink.FAMILY_ALL)
	if err != nil {
		t.Fatalf("XfrmStateList: %v", err)
	}
	var installed int
	for i := range states {
		if states[i].Spi == 0x4552b2 {
			installed++
		}
	}
	if installed == 0 {
		t.Error("no kernel SA after onInterfaceUp")
	}

	inst.onInterfaceDown(1, "ipsec-itest")
	states, err = netlink.XfrmStateList(netlink.FAMILY_ALL)
	if err != nil {
		t.Fatalf("XfrmStateList after down: %v", err)
	}
	for i := range states {
		if states[i].Spi == 0x4552b2 {
			t.Error("orphan kernel SA after onInterfaceDown (AC-10)")
		}
	}
}

// TestIPsecInstallerXFRMReplayWindow checks that the interface's replay-window leaf reaches
// the kernel. The method is the installer's own path -- onInterfaceUp, buildIPsecSA, the
// XFRM backend -- read back from `XfrmStateList`, so it proves the boundary Ze owns rather
// than the SAParams struct alone. RFC 4302 Section 3.4.3: "All AH implementations MUST
// support the anti-replay service, though its use may be enabled or disabled by the
// receiver on a per-SA basis." The kernel is what performs the per-packet check, so the
// window Ze installs for it is the whole of Ze's part.
func TestIPsecInstallerXFRMReplayWindow(t *testing.T) {
	requireXFRM(t)
	const spi = 0x4552b3
	inst := newIPsecInstaller(nil, nil)
	inst.setTransportSource(func(string) (netip.Addr, int, bool) {
		return netip.MustParseAddr("fe80::1"), 1, true
	})
	inst.setConfig([]interfaceConfig{{
		Name: "ipsec-replay",
		IPsec: &ipsecInterfaceConfig{
			SPI: spi, Protocol: "ah", AuthAlgo: "sha256", AuthKey: hexKey(32), ReplayWindow: 64,
		},
	}})

	inst.onInterfaceUp(1, "ipsec-replay")
	defer inst.onInterfaceDown(1, "ipsec-replay")

	states, err := netlink.XfrmStateList(netlink.FAMILY_ALL)
	if err != nil {
		t.Fatalf("XfrmStateList: %v", err)
	}
	var found *netlink.XfrmState
	for i := range states {
		if states[i].Spi == spi {
			found = &states[i]
		}
	}
	if found == nil {
		t.Fatal("installed AH SA not found in kernel")
	}
	if found.ReplayWindow != 64 {
		t.Errorf("kernel SA ReplayWindow = %d, want 64 (the configured replay-window)", found.ReplayWindow)
	}
}
