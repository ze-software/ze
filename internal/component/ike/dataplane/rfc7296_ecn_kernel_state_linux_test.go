// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- XFRM netlink backend
// Related: rfc7296_ecn_linux_test.go -- the same tunnel-mode fixture, checked before netlink serializes it

//go:build linux

package dataplane

import (
	"errors"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

// ecnKernelSPI is high and distinctive, so a stray SA cannot be mistaken for the one
// this file installs.
const ecnKernelSPI uint32 = 0x5ead0e01

// ecnKernelStoredSA dumps the SAD over raw netlink and returns a copy of the
// xfrm_usersa_info header the kernel holds for exactly one state with this SPI and
// protocol. It fails the test when there is none, or more than one.
//
// It decodes the header itself rather than going through netlink.XfrmStateList, because
// the library's decoded XfrmState has no field for XFRM_STATE_NOECN: only the raw flags
// octet says whether the kernel's decapsulation skips the ECN step for the state
// (xfrm_input.c, ipip_ecn_decapsulate is reached only without XFRM_STATE_NOECN).
func ecnKernelStoredSA(t *testing.T, spi uint32, proto uint8) nl.XfrmUsersaInfo {
	t.Helper()
	req := nl.NewNetlinkRequest(nl.XFRM_MSG_GETSA, unix.NLM_F_DUMP)
	msgs, err := req.Execute(unix.NETLINK_XFRM, nl.XFRM_MSG_NEWSA)
	if err != nil && !errors.Is(err, nl.ErrDumpInterrupted) {
		t.Fatalf("SAD dump: %v", err)
	}
	var stored nl.XfrmUsersaInfo
	found := 0
	for _, m := range msgs {
		if len(m) < nl.SizeofXfrmUsersaInfo {
			t.Fatalf("the SAD dump carried a %d-octet message, shorter than the %d-octet xfrm_usersa_info header",
				len(m), nl.SizeofXfrmUsersaInfo)
		}
		info := nl.DeserializeXfrmUsersaInfo(m)
		if info.Id.Spi != nl.Swap32(spi) {
			continue
		}
		if info.Id.Proto != proto {
			continue
		}
		stored = *info
		found++
	}
	if found != 1 {
		t.Fatalf("the SAD holds %d states with SPI %#x and protocol %d, want exactly the one this test installed (dump error: %v)",
			found, spi, proto, err)
	}
	return stored
}

// VALIDATES: RFC 7296 Section 2.24 on the state Linux XFRM actually holds. A tunnel-mode
// ESP Child SA installed through xfrmBackend.InstallSA, the production install path, is
// stored by the kernel in tunnel mode with XFRM_STATE_NOECN clear in its flags octet, so
// the kernel's decapsulation copies a CE mark from the outer header onto the inner one
// (ipip_ecn_decapsulate in net/xfrm/xfrm_input.c), which is the ECN full-functionality
// behavior the section requires.
//
// PREVENTS: the vacuity of TestEcnInstalledStateDisablesNothing, which scans the Go value
// Ze hands the library for a field whose name mentions "ecn" or "flag". netlink.XfrmState
// has no such field, so that scan finds nothing for every state and cannot go red. The
// NOECN bit exists only in the serialized xfrm_usersa_info header, so this test reads it
// there, after the kernel stored it.
//
// Method: install the ecnTunnelParams fixture under a distinctive SPI in a private user
// and network namespace, dump the SAD over raw netlink, and decode the one header with
// that SPI. Three facts are read from it. The mode must be tunnel, because Section 2.24
// binds tunnel mode. XFRM_STATE_AF_UNSPEC must be set: xfrmStateFromParams sets AFUnspec
// on a tunnel state that has no selector, and the kernel keeps it in the same octet as
// NOECN, so seeing it proves the octet read is the one Ze's install wrote and not a
// misdecoded offset or another state. XFRM_STATE_NOECN must be clear.
//
// MUTATION: in vendor/github.com/vishvananda/netlink/xfrm_state_linux.go,
// xfrmUsersaInfoFromXfrmState, the serializer InstallSA's netlink.XfrmStateAdd call
// runs, `msg.Flags |= nl.XFRM_STATE_AF_UNSPEC` becomes `msg.Flags |=
// nl.XFRM_STATE_NOECN`: the kernel then stores a state that disables ECN decapsulation,
// and this test goes red on the NOECN assertion.
//
// RFC requirement: RFC7296-2.24-1 positive -- a tunnel-mode ESP Child SA installed through
// xfrmBackend.InstallSA is stored by Linux XFRM in tunnel mode with XFRM_STATE_NOECN clear
// in its xfrm_usersa_info flags octet, read back over a raw netlink SAD dump beside the
// XFRM_STATE_AF_UNSPEC bit Ze's mapping sets in the same octet.
func TestEcnKernelStoresTunnelSAWithoutNoECN(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	p := ecnTunnelParams()
	p.SPI = ecnKernelSPI
	b := &xfrmBackend{}
	if err := b.InstallSA(p); err != nil {
		t.Fatalf("InstallSA of the tunnel-mode fixture: %v", err)
	}
	t.Cleanup(func() {
		if err := b.RemoveSA(p.SPI, p.Dst, ProtoESP); err != nil {
			t.Errorf("remove SA %#x: %v", p.SPI, err)
		}
	})

	stored := ecnKernelStoredSA(t, p.SPI, uint8(netlink.XFRM_PROTO_ESP))
	if stored.Mode != uint8(netlink.XFRM_MODE_TUNNEL) {
		t.Fatalf("the kernel stores SA %#x in mode %d, want tunnel (%d); Section 2.24 binds tunnel mode, so a non-tunnel state proves nothing",
			p.SPI, stored.Mode, uint8(netlink.XFRM_MODE_TUNNEL))
	}
	if stored.Flags&nl.XFRM_STATE_AF_UNSPEC == 0 {
		t.Errorf("the kernel stores SA %#x with flags %#02x and no XFRM_STATE_AF_UNSPEC, which xfrmStateFromParams sets on every selector-less tunnel state; "+
			"the octet read is not the one Ze's install wrote, so its NOECN bit proves nothing", p.SPI, stored.Flags)
	}
	if stored.Flags&nl.XFRM_STATE_NOECN != 0 {
		t.Errorf("the kernel stores tunnel-mode SA %#x with flags %#02x, which carry XFRM_STATE_NOECN: "+
			"its decapsulation skips the ECN step and drops the outer CE mark, against RFC 7296 Section 2.24", p.SPI, stored.Flags)
	}
}
