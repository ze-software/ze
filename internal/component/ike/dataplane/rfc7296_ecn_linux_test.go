// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- XFRM netlink backend

//go:build linux

package dataplane

// RFC naming: untagged -- proves the netlink state xfrmStateFromParams builds is the one Ze asked for, not ECN handling; the RFC7296-2.24 tags sit on TestEcnKernelStoresTunnelSAWithoutNoECN and engine's TestRFC7296BareESPOnTemplatedSAKeepsTheOuterCE.

import (
	"net"
	"testing"
)

// ecnTunnelParams is one tunnel-mode ESP Child SA description, shaped like the ones
// installChildSA builds. Section 2.24 binds TUNNEL mode, so the fixture is tunnel mode.
func ecnTunnelParams() SAParams {
	return SAParams{
		SPI:      0xC0FFEE01,
		Src:      net.ParseIP("10.0.0.1"),
		Dst:      net.ParseIP("10.0.0.2"),
		Proto:    ProtoESP,
		Mode:     ModeTunnel,
		ReqID:    7,
		EncAlgo:  "aes256",
		EncKey:   make([]byte, 32),
		AuthAlgo: "sha256",
		AuthKey:  make([]byte, 32),
	}
}

// VALIDATES: the netlink state xfrmStateFromParams builds for the ECN fixture carries the
// SPI and endpoints Ze asked for, maps tunnel and transport to different kernel modes, and
// refuses a mode it cannot express rather than emitting a default.
//
// PREVENTS: the kernel-state ECN test reading a state the fixture did not describe. This
// carries no RFC tag: it proves the mapping, and a check that cannot fail when ECN handling
// breaks is not evidence that ECN works.
func TestEcnXfrmStateIsTheOneZeAskedFor(t *testing.T) {
	p := ecnTunnelParams()
	state, err := xfrmStateFromParams(p)
	if err != nil {
		t.Fatalf("xfrmStateFromParams: %v", err)
	}
	if state.Spi != int(p.SPI) {
		t.Errorf("the state carries SPI %#x, want %#x, so it is not the state ze asked for",
			state.Spi, p.SPI)
	}
	if !state.Src.Equal(p.Src) || !state.Dst.Equal(p.Dst) {
		t.Errorf("the state carries %v -> %v, want %v -> %v", state.Src, state.Dst, p.Src, p.Dst)
	}

	transport := p
	transport.Mode = ModeTransport
	tState, err := xfrmStateFromParams(transport)
	if err != nil {
		t.Fatalf("xfrmStateFromParams(transport): %v", err)
	}
	if tState.Mode == state.Mode {
		t.Error("tunnel and transport map to one netlink mode, so no state can be identified as tunnel mode")
	}

	bad := p
	bad.Mode = 0
	if _, err := xfrmStateFromParams(bad); err == nil {
		t.Error("an unknown mode built a state instead of being refused, so a Child SA of no known mode reaches the kernel")
	}
}
