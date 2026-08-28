// VALIDATES: the XFRM backend programs transport mode and tunnel mode as DIFFERENT
// kernel objects — the state carries XFRM_MODE_TRANSPORT (0) versus XFRM_MODE_TUNNEL (1),
// and a transport-mode policy template carries no tunnel endpoints. Also validates that a
// single-port policy selector reaches the kernel with a full port mask.
// PREVENTS: the silent-wrong-mode failure kernelXFRMMode's own comment records — Ze once
// sent ModeTransport to the kernel as XFRM_MODE_TUNNEL and the kernel accepted it with no
// error, so a unit test with a fake backend cannot tell the two apart. Only a real kernel
// read-back can.
//
// Auto-enrolled in the native QEMU integration run through the derived
// `integration && linux` package list.

//go:build integration && linux

package dataplane

import (
	"errors"
	"net"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func skipUnprivileged(t *testing.T, err error) {
	t.Helper()
	if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
		t.Skipf("XFRM programming needs CAP_NET_ADMIN: %v", err)
	}
}

func mustCIDR(t *testing.T, s string) *net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return n
}

// TestXFRMTransportAndTunnelModesDiffer installs one SA in each mode and reads both back.
func TestXFRMTransportAndTunnelModesDiffer(t *testing.T) {
	b := &xfrmBackend{}
	src := net.ParseIP("10.77.0.1")
	dst := net.ParseIP("10.77.0.2")
	key := make([]byte, 20)
	authKey := make([]byte, 32)

	cases := []struct {
		name       string
		mode       uint8
		spi        uint32
		wantKernel netlink.Mode
	}{
		{"transport", ModeTransport, 0x7e570001, netlink.Mode(kernelModeTransport)},
		{"tunnel", ModeTunnel, 0x7e570002, netlink.Mode(kernelModeTunnel)},
	}

	installed := 0
	for _, tc := range cases {
		err := b.InstallSA(SAParams{
			SPI: tc.spi, Src: src, Dst: dst, Proto: ProtoESP, Mode: tc.mode,
			ReqID: 0x7e57, ReplayWin: 32,
			EncAlgo: "aes256", EncKey: authKey,
			AuthAlgo: "sha256", AuthKey: key,
		})
		if err != nil {
			skipUnprivileged(t, err)
			t.Fatalf("%s: InstallSA: %v", tc.name, err)
		}
		installed++
		t.Cleanup(func() { _ = b.RemoveSA(tc.spi, dst, ProtoESP) })
	}
	if installed != len(cases) {
		t.Fatalf("installed %d SAs, want %d; the read-back below would prove nothing", installed, len(cases))
	}

	states, err := netlink.XfrmStateList(netlink.FAMILY_ALL)
	if err != nil {
		skipUnprivileged(t, err)
		t.Fatalf("XfrmStateList: %v", err)
	}
	seen := 0
	for i := range states {
		s := &states[i]
		for _, tc := range cases {
			if uint32(s.Spi) != tc.spi {
				continue
			}
			seen++
			if s.Mode != tc.wantKernel {
				t.Errorf("%s SA reached the kernel as mode %d, want %d; a wrong mode number is accepted silently and protects the traffic in the wrong mode",
					tc.name, s.Mode, tc.wantKernel)
			}
		}
	}
	if seen != len(cases) {
		t.Fatalf("read back %d of the %d SAs just installed; the mode assertions swept over an incomplete set", seen, len(cases))
	}
}

// TestXFRMTransportPolicyCarriesNoTunnelEndpoints proves the policy half.
func TestXFRMTransportPolicyCarriesNoTunnelEndpoints(t *testing.T) {
	b := &xfrmBackend{}
	sel := SPParams{
		Src:   mustCIDR(t, "10.78.1.0/24"),
		Dst:   mustCIDR(t, "10.78.2.0/24"),
		Dir:   SADirOut,
		Proto: ProtoESP,
		Mode:  ModeTransport,
		ReqID: 0x7e58,
	}
	if err := b.InstallPolicy(sel); err != nil {
		skipUnprivileged(t, err)
		t.Fatalf("InstallPolicy(transport): %v", err)
	}
	t.Cleanup(func() { _ = b.RemovePolicy(sel.Src, sel.Dst, sel.Dir) })

	policies, err := netlink.XfrmPolicyList(netlink.FAMILY_ALL)
	if err != nil {
		skipUnprivileged(t, err)
		t.Fatalf("XfrmPolicyList: %v", err)
	}
	found := false
	for i := range policies {
		p := &policies[i]
		if p.Src == nil || p.Src.String() != sel.Src.String() {
			continue
		}
		found = true
		if len(p.Tmpls) != 1 {
			t.Fatalf("transport policy carried %d templates, want 1", len(p.Tmpls))
		}
		tm := p.Tmpls[0]
		if tm.Mode != netlink.Mode(kernelModeTransport) {
			t.Errorf("transport policy template mode = %d, want %d", tm.Mode, kernelModeTransport)
		}
		// RFC 4301 Section 4.4.1.2 leaves a transport-mode template's addresses unused.
		// tunnelEndpoints refuses a caller that supplies them, so anything non-zero here
		// means the guard was bypassed.
		if tm.Src != nil && !tm.Src.IsUnspecified() {
			t.Errorf("transport policy template carries tunnel src %v, want none", tm.Src)
		}
		if tm.Dst != nil && !tm.Dst.IsUnspecified() {
			t.Errorf("transport policy template carries tunnel dst %v, want none", tm.Dst)
		}
	}
	if !found {
		t.Fatal("the transport policy just installed was not found in the kernel; the assertions above swept over nothing")
	}
}

// TestXFRMSinglePortSelectorReachesTheKernel proves a port-restricted policy is programmed
// exactly, which is what lets ipsec.ValidateTrafficSelectors accept one port and refuse
// every other range (ai/rules/protocol.md).
func TestXFRMSinglePortSelectorReachesTheKernel(t *testing.T) {
	b := &xfrmBackend{}
	sel := SPParams{
		Src:        mustCIDR(t, "10.79.1.0/24"),
		Dst:        mustCIDR(t, "10.79.2.0/24"),
		Dir:        SADirOut,
		Proto:      ProtoESP,
		Mode:       ModeTunnel,
		ReqID:      0x7e59,
		UpperProto: 6,
		DstPort:    ExactPortMatch(179),
		TunnelSrc:  net.ParseIP("10.79.0.1"),
		TunnelDst:  net.ParseIP("10.79.0.2"),
	}
	if err := b.InstallPolicy(sel); err != nil {
		skipUnprivileged(t, err)
		t.Fatalf("InstallPolicy(port): %v", err)
	}
	t.Cleanup(func() { _ = b.RemovePolicy(sel.Src, sel.Dst, sel.Dir) })

	policies, err := netlink.XfrmPolicyList(netlink.FAMILY_ALL)
	if err != nil {
		skipUnprivileged(t, err)
		t.Fatalf("XfrmPolicyList: %v", err)
	}
	found := false
	for i := range policies {
		p := &policies[i]
		if p.Src == nil || p.Src.String() != sel.Src.String() {
			continue
		}
		found = true
		if p.DstPort != 179 {
			t.Errorf("policy destination port = %d, want 179; the negotiated port did not reach the kernel selector", p.DstPort)
		}
		if p.Proto != netlink.Proto(6) {
			t.Errorf("policy upper protocol = %d, want 6 (TCP)", p.Proto)
		}
	}
	if !found {
		t.Fatal("the port-restricted policy just installed was not found in the kernel")
	}
}

// TestXFRMOpaquePortIsRefused pins the exact-or-reject boundary this package relies on:
// the backend REFUSES a port match it cannot express, rather than installing an any-port
// policy that would protect more traffic than was negotiated.
func TestXFRMOpaquePortIsRefused(t *testing.T) {
	b := &xfrmBackend{}
	err := b.InstallPolicy(SPParams{
		Src:   mustCIDR(t, "10.80.1.0/24"),
		Dst:   mustCIDR(t, "10.80.2.0/24"),
		Dir:   SADirOut,
		Proto: ProtoESP,
		Mode:  ModeTunnel,
		ReqID: 0x7e5a,
		// The OPAQUE port form of RFC 7296 Section 3.13.1: exactly port 0.
		DstPort:   PortMatch{Port: 0, Mask: 0xffff},
		TunnelSrc: net.ParseIP("10.80.0.1"),
		TunnelDst: net.ParseIP("10.80.0.2"),
	})
	if err == nil {
		_ = b.RemovePolicy(mustCIDR(t, "10.80.1.0/24"), mustCIDR(t, "10.80.2.0/24"), SADirOut)
		t.Fatal("an opaque-port policy was installed; netlink derives the mask from the port, so it would match EVERY port and protect more traffic than was negotiated")
	}
}
