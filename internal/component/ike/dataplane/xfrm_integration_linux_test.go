// VALIDATES: the XFRM backend's read path against the real kernel — ListSAs
// issues XfrmStateList and returns well-typed SAInfo entries. Also validates the
// install path's state flags: a tunnel-mode state installed with no selector is
// stored with XFRM_STATE_AF_UNSPEC and selector family 0, and a state that hands a
// selector keeps that selector's family and carries no flag.
// Auto-enrolled in the QEMU integration run via the derived `integration && linux`
// package list.
// PREVENTS: a regression in the netlink XFRM state enumeration surfacing only on
// a live appliance running IKE, and a tunnel-mode Child SA the kernel narrows to
// its outer family, which carries no inner packet of the other family.
// METHOD: userns.Enter runs it as root over a user and network namespace of its
// own, so it needs no host privilege and a refused list is a failure.

//go:build linux

package dataplane

import (
	"net"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

func TestXFRMListSAs(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	b := &xfrmBackend{}
	sas, err := b.ListSAs(0)
	if err != nil {
		t.Fatalf("ListSAs: %v", err)
	}
	// The list may legitimately be empty; assert only that entries are well formed.
	for i, sa := range sas {
		if sa.Src == nil || sa.Dst == nil {
			t.Errorf("SA %d has a nil endpoint: %+v", i, sa)
		}
	}
}

// TestXFRMTunnelStateIsInstalledAFUnspec installs states through InstallSA and
// reads each back from the kernel as the raw xfrm_usersa_info, so the flags and
// the selector family are the kernel's own bytes, not a decoder's view of them.
//
// Without the flag, copy_from_user_state (net/xfrm/xfrm_user.c) gives a state
// with no selector the OUTER family, and the readback shows AF_INET or AF_INET6
// in place of 0. That is the red this test shows before the builder sets the flag.
func TestXFRMTunnelStateIsInstalledAFUnspec(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	b := &xfrmBackend{}
	v4Sel := &SASelector{Src: mustCIDR(t, "10.78.0.1/32"), Dst: mustCIDR(t, "10.78.0.2/32")}
	v6Sel := &SASelector{Src: mustCIDR(t, "2001:db8:78::1/128"), Dst: mustCIDR(t, "2001:db8:78::2/128")}
	cases := []struct {
		what      string
		spi       uint32
		src, dst  string
		mode      uint8
		sel       *SASelector
		wantFlag  bool
		wantSelAF uint16
	}{
		{"tunnel over IPv4 with no selector", 0x7e5a0001, "10.77.0.1", "10.77.0.2", ModeTunnel, nil, true, unix.AF_UNSPEC},
		{"tunnel over IPv6 with no selector", 0x7e5a0002, "2001:db8:77::1", "2001:db8:77::2", ModeTunnel, nil, true, unix.AF_UNSPEC},
		{"tunnel over IPv4 with an IPv6 selector", 0x7e5a0003, "10.77.0.1", "10.77.0.2", ModeTunnel, v6Sel, false, unix.AF_INET6},
		{"transport over IPv4 with its selector", 0x7e5a0004, "10.78.0.1", "10.78.0.2", ModeTransport, v4Sel, false, unix.AF_INET},
	}
	for _, tc := range cases {
		dst := net.ParseIP(tc.dst)
		err := b.InstallSA(SAParams{
			SPI: tc.spi, Src: net.ParseIP(tc.src), Dst: dst, Proto: ProtoESP, Mode: tc.mode,
			ReqID: 0x7e5a, ReplayWin: 32, Sel: tc.sel,
			EncAlgo: "aes256", EncKey: make([]byte, 32),
			AuthAlgo: "sha256", AuthKey: make([]byte, 32),
		})
		if err != nil {
			t.Fatalf("%s: InstallSA: %v", tc.what, err)
		}
		t.Cleanup(func() { _ = b.RemoveSA(tc.spi, dst, ProtoESP) })

		got, err := readXFRMMigrationState(&netlink.XfrmState{Spi: int(tc.spi), Dst: dst, Proto: netlink.XFRM_PROTO_ESP})
		if err != nil {
			t.Fatalf("%s: read the installed state back: %v", tc.what, err)
		}
		if flagged := got.info.Flags&nl.XFRM_STATE_AF_UNSPEC != 0; flagged != tc.wantFlag {
			t.Errorf("%s: kernel state flags = %#x, XFRM_STATE_AF_UNSPEC set = %t, want %t",
				tc.what, got.info.Flags, flagged, tc.wantFlag)
		}
		if got.info.Sel.Family != tc.wantSelAF {
			t.Errorf("%s: kernel selector family = %d, want %d; a tunnel state narrowed to its outer family drops every inner packet of the other family (xfrm_state_look_at)",
				tc.what, got.info.Sel.Family, tc.wantSelAF)
		}
		if want := uint16(nl.GetIPFamily(dst)); got.info.Family != want {
			t.Errorf("%s: kernel outer family = %d, want %d", tc.what, got.info.Family, want)
		}
	}
}
