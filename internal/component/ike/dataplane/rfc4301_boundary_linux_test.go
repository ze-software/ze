// VALIDATES: the state Ze installs into Linux XFRM at its own boundary, for the RFC
// 4301 obligations the kernel performs on that state (owner ruling 2026-08-31: a
// requirement met through a lower layer still carries a test asserting what Ze
// installs). The SAD entry carries the (SPI, destination, protocol) lookup triple, the
// address pair, the mode and the replay window; the SPD entry carries the selector,
// the direction, the order and the transform template, or no template for a bypass
// or a discard.
// PREVENTS: an SA or a policy the kernel cannot evaluate as Ze meant it being
// installed widened or silently wrong, which is the fail-open shape of every
// per-packet obligation below.

//go:build linux

package dataplane

import (
	"net"
	"testing"

	"github.com/vishvananda/netlink"
)

func boundarySA(spi uint32) SAParams {
	return SAParams{
		SPI:       spi,
		Src:       net.ParseIP("192.0.2.1"),
		Dst:       net.ParseIP("198.51.100.1"),
		Proto:     ProtoESP,
		Mode:      ModeTunnel,
		Dir:       SADirOut,
		ReplayWin: 64,
		EncAlgo:   "aes256",
		AuthAlgo:  "sha256",
		EncKey:    make([]byte, 32),
		AuthKey:   make([]byte, 32),
	}
}

func boundaryPolicy(t *testing.T, mode uint8, dir SADir, port PortMatch) SPParams {
	t.Helper()
	_, src, err := net.ParseCIDR("10.0.0.0/24")
	if err != nil {
		t.Fatalf("parse local prefix: %v", err)
	}
	_, dst, err := net.ParseCIDR("10.1.0.0/24")
	if err != nil {
		t.Fatalf("parse remote prefix: %v", err)
	}
	p := SPParams{
		Src:        src,
		Dst:        dst,
		Dir:        dir,
		Action:     SPActionProtect,
		Mode:       mode,
		Proto:      ProtoESP,
		ReqID:      7,
		Priority:   PriorityChildSA,
		UpperProto: 6,
		SrcPort:    port,
		DstPort:    AnyPortMatch(),
	}
	if mode == ModeTunnel {
		p.TunnelSrc = net.ParseIP("192.0.2.1")
		p.TunnelDst = net.ParseIP("198.51.100.1")
	}
	return p
}

// RFC requirement: RFC4301-4-1 positive -- the SAD entry Ze installs carries the SPI, the destination address and the security protocol the kernel looks an inbound packet up by.
// RFC requirement: RFC4301-4.1-8 positive -- the SAD entry carries both the source and the destination address, which is the address matching Ze negotiates for a unicast SA.
// RFC requirement: RFC4301-4.1-9 positive -- two SAs with the same selector and different SPIs build two distinct SAD entries, each carrying its own SPI.
// RFC requirement: RFC4301-4.4.2.1-1 positive -- the SAD entry carries the SPI, the mode, the protocol, the encryption and integrity transforms with their keys, and the anti-replay window.
// RFC requirement: RFC4301-4.4.2.1-3 positive -- the SAD entry carries the anti-replay window Ze negotiated, which is what lets the kernel accept a sequence number ahead of or behind its own counter.
func TestRFC4301BoundarySADEntryCarriesWhatTheKernelLooksUp(t *testing.T) {
	first, err := xfrmStateFromParams(boundarySA(0x1000))
	if err != nil {
		t.Fatalf("xfrmStateFromParams: %v", err)
	}
	if first.Spi != 0x1000 {
		t.Errorf("spi %#x, want 0x1000", first.Spi)
	}
	if !first.Dst.Equal(net.ParseIP("198.51.100.1")) || !first.Src.Equal(net.ParseIP("192.0.2.1")) {
		t.Errorf("address pair %s -> %s, want 192.0.2.1 -> 198.51.100.1", first.Src, first.Dst)
	}
	if first.Proto != netlink.XFRM_PROTO_ESP {
		t.Errorf("protocol %d, want ESP", first.Proto)
	}
	if first.Mode != netlink.XFRM_MODE_TUNNEL {
		t.Errorf("mode %d, want tunnel", first.Mode)
	}
	if first.ReplayWindow != 64 {
		t.Errorf("replay window %d, want 64", first.ReplayWindow)
	}
	if first.Crypt == nil || first.Auth == nil {
		t.Fatalf("transforms crypt=%v auth=%v, want both", first.Crypt, first.Auth)
	}
	if len(first.Crypt.Key) != 32 || len(first.Auth.Key) != 32 {
		t.Errorf("key lengths crypt=%d auth=%d, want 32 and 32", len(first.Crypt.Key), len(first.Auth.Key))
	}

	second, err := xfrmStateFromParams(boundarySA(0x2000))
	if err != nil {
		t.Fatalf("xfrmStateFromParams second: %v", err)
	}
	if second.Spi == first.Spi {
		t.Errorf("two SAs share spi %#x", second.Spi)
	}
	if second.Spi != 0x2000 {
		t.Errorf("second spi %#x, want 0x2000", second.Spi)
	}
}

// RFC requirement: RFC4301-4-1 negative -- an SA whose mode the kernel cannot hold is refused and no SAD entry is built.
// RFC requirement: RFC4301-4.1-8 negative -- an SA whose mode is unknown never reaches the kernel with its address pair.
// RFC requirement: RFC4301-4.1-9 negative -- an SA the kernel cannot hold builds no entry, so no second SA is ever shadowed by a malformed one.
// RFC requirement: RFC4301-4.4.2.1-1 negative -- an SA naming an integrity transform the kernel has no name for is refused rather than installed without it.
// RFC requirement: RFC4301-4.4.2.1-3 negative -- an SA that cannot be built carries no replay window, because no SAD entry is built at all.
func TestRFC4301BoundarySADEntryIsRefusedRatherThanInstalledWrong(t *testing.T) {
	unknownMode := boundarySA(0x1000)
	unknownMode.Mode = 9
	if state, err := xfrmStateFromParams(unknownMode); err == nil || state != nil {
		t.Fatalf("unknown mode: got state %v err %v, want refusal and no state", state, err)
	}
	unknownAuth := boundarySA(0x1000)
	unknownAuth.AuthAlgo = "unknown"
	if state, err := xfrmStateFromParams(unknownAuth); err == nil || state != nil {
		t.Fatalf("unknown integrity transform: got state %v err %v, want refusal and no state", state, err)
	}
}

// RFC requirement: RFC4301-3.1-1 positive -- a tunnel mode entry (gateway) and a transport mode entry (host) each build with the mode the kernel applies.
// RFC requirement: RFC4301-4.4-1 positive -- the SPD entry Ze installs carries a selector, a disposition, a direction and an order, the externally observable fields of the Section 4.4 model.
// RFC requirement: RFC4301-4.4.1-9 positive -- one entry builds exactly one kernel policy, so there is no decorrelated group to link.
// RFC requirement: RFC4301-4.4.1-10 positive -- the entry carries the priority the kernel's ordered search reads.
// RFC requirement: RFC4301-4.4.1.1-3 positive -- an entry requiring one port carries that exact port, so a fragment without ports cannot match it.
// RFC requirement: RFC4301-5.1-1 positive -- the outbound entry carries the out direction, the selector and the ESP tunnel template the kernel's outbound steps read.
// RFC requirement: RFC4301-5.2-3 positive -- the inbound entry carries the in direction and the same selector the kernel's inbound steps check.
// RFC requirement: RFC4301-7.3-3 positive -- the outbound entry carries its exact port selector unchanged, which is what stops the kernel matching a non-initial fragment to it.
// RFC requirement: RFC4301-7.3-4 positive -- the inbound entry carries its exact port selector unchanged, which is what makes the kernel refuse a non-initial fragment on it.
// RFC requirement: RFC4301-7.3-5 positive -- the inbound entry carries the selector and the template the kernel checks every fragment of a packet against.
func TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate(t *testing.T) {
	out, err := xfrmPolicyFromParams(boundaryPolicy(t, ModeTunnel, SADirOut, ExactPortMatch(443)))
	if err != nil {
		t.Fatalf("outbound tunnel policy: %v", err)
	}
	if out.Dir != netlink.XFRM_DIR_OUT {
		t.Errorf("dir %d, want out", out.Dir)
	}
	if out.Src.String() != "10.0.0.0/24" || out.Dst.String() != "10.1.0.0/24" {
		t.Errorf("selector %s -> %s, want 10.0.0.0/24 -> 10.1.0.0/24", out.Src, out.Dst)
	}
	if out.Priority != PriorityChildSA {
		t.Errorf("priority %d, want %d", out.Priority, PriorityChildSA)
	}
	if out.SrcPort != 443 || out.DstPort != 0 {
		t.Errorf("ports %d -> %d, want exact 443 -> any", out.SrcPort, out.DstPort)
	}
	if out.Action != netlink.XFRM_POLICY_ALLOW {
		t.Errorf("action %d, want allow (protect)", out.Action)
	}
	if len(out.Tmpls) != 1 {
		t.Fatalf("templates %d, want exactly one", len(out.Tmpls))
	}
	if out.Tmpls[0].Mode != netlink.XFRM_MODE_TUNNEL || out.Tmpls[0].Proto != netlink.XFRM_PROTO_ESP {
		t.Errorf("template mode %d proto %d, want tunnel ESP", out.Tmpls[0].Mode, out.Tmpls[0].Proto)
	}
	if !out.Tmpls[0].Dst.Equal(net.ParseIP("198.51.100.1")) {
		t.Errorf("template tunnel destination %s, want 198.51.100.1", out.Tmpls[0].Dst)
	}

	in, err := xfrmPolicyFromParams(boundaryPolicy(t, ModeTunnel, SADirIn, ExactPortMatch(443)))
	if err != nil {
		t.Fatalf("inbound tunnel policy: %v", err)
	}
	if in.Dir != netlink.XFRM_DIR_IN {
		t.Errorf("dir %d, want in", in.Dir)
	}
	if in.SrcPort != 443 {
		t.Errorf("inbound source port %d, want the exact 443", in.SrcPort)
	}
	if in.Src.String() != out.Src.String() || len(in.Tmpls) != 1 {
		t.Errorf("inbound entry selector %s templates %d, want the same selector and one template", in.Src, len(in.Tmpls))
	}

	transport, err := xfrmPolicyFromParams(boundaryPolicy(t, ModeTransport, SADirOut, AnyPortMatch()))
	if err != nil {
		t.Fatalf("transport policy: %v", err)
	}
	if len(transport.Tmpls) != 1 || transport.Tmpls[0].Mode != netlink.XFRM_MODE_TRANSPORT {
		t.Fatalf("transport templates %v, want one transport-mode template", transport.Tmpls)
	}
}

// RFC requirement: RFC4301-3.1-1 negative -- an entry whose mode is neither tunnel nor transport is refused and no policy is built.
// RFC requirement: RFC4301-4.4-1 negative -- an entry whose selector the kernel cannot express is refused rather than installed widened.
// RFC requirement: RFC4301-4.4.1-9 negative -- a refused entry builds no policy, so nothing partial reaches the ordered database.
// RFC requirement: RFC4301-4.4.1-10 negative -- a refused entry never reaches the kernel's ordered search at any priority.
// RFC requirement: RFC4301-4.4.1.1-3 negative -- a port mask the selector cannot carry exactly is refused rather than widened to any port, so no entry that would admit a portless fragment is built.
// RFC requirement: RFC4301-5.1-1 negative -- an outbound entry with a port mask the selector cannot express is refused.
// RFC requirement: RFC4301-5.2-3 negative -- an inbound entry with a port mask the selector cannot express is refused.
// RFC requirement: RFC4301-7.3-3 negative -- a port selector is never widened on the way to the kernel: the inexpressible mask is refused.
// RFC requirement: RFC4301-7.3-4 negative -- an inbound port selector is never widened on the way to the kernel: the inexpressible mask is refused.
// RFC requirement: RFC4301-7.3-5 negative -- a bypass entry never carries a template, so no fragment can be handed to a transform the entry did not name.
func TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened(t *testing.T) {
	unknownMode := boundaryPolicy(t, ModeTunnel, SADirOut, AnyPortMatch())
	unknownMode.Mode = 9
	if pol, err := xfrmPolicyFromParams(unknownMode); err == nil || pol != nil {
		t.Fatalf("unknown mode: got policy %v err %v, want refusal", pol, err)
	}
	for _, dir := range []SADir{SADirOut, SADirIn} {
		partialMask := boundaryPolicy(t, ModeTunnel, dir, PortMatch{Port: 443, Mask: 0x00ff})
		if pol, err := xfrmPolicyFromParams(partialMask); err == nil || pol != nil {
			t.Fatalf("dir %d partial port mask: got policy %v err %v, want refusal", dir, pol, err)
		}
	}
	bypass := boundaryPolicy(t, ModeTunnel, SADirIn, AnyPortMatch())
	bypass.Action = SPActionBypass
	bypass.TunnelSrc, bypass.TunnelDst, bypass.Mode, bypass.ReqID = nil, nil, 0, 0
	pol, err := xfrmPolicyFromParams(bypass)
	if err != nil {
		t.Fatalf("bypass entry: %v", err)
	}
	if len(pol.Tmpls) != 0 {
		t.Fatalf("bypass entry carries %d templates, want none", len(pol.Tmpls))
	}
}
