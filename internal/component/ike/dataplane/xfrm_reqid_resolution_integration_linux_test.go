// VALIDATES: the assumption every make-before-break Child SA rekey rests on -- that ONE
// installed XFRM policy resolves to SUCCESSIVE states through the request id, so a rekey
// that installs new states and never touches the policy still carries traffic.
// RFC 4301 Section 4.4.1.2 states it: an SPD entry's template names the request id, and
// the SAD entry it resolves to is looked up by that id rather than by any SPI the entry
// was created beside.
//
// PREVENTS: the whole rekey design being taken on a reading of that section. Ze installs
// the replacement pair's states while the retired pair is still installed
// (installChildTolerant, engine/child.go), and it re-installs an IDENTICAL policy which
// the backend upserts (xfrmBackend.InstallPolicy). If the kernel bound a policy to the
// state that existed when the policy was written, the replacement would carry nothing
// and every tunnel would go dark at its first Child SA rekey. That was recorded as
// assumption A-1 of spec-fixit-child-sa-rekey-policy, load-bearing and never
// measured: no test in this package installed an XFRM state against an installed policy
// at all.
//
// THE INSTRUMENT IS XfrmOutNoStates, and the direction of its silence is the reading.
// A PROTECT policy that matches a datagram and resolves to NO state raises it; one that
// resolves raises nothing. So "the counter did not move" is the positive result here,
// which is exactly the shape that passes vacuously when a fixture is broken. Every
// reading below is therefore bracketed by a control that must move it: the same probe
// with no state installed, taken before and after.

//go:build integration && linux

package dataplane

import (
	"net"
	"testing"

	"github.com/vishvananda/netlink"
)

const (
	// The inner flow. It is a different pair from ownerLocalCIDR/ownerRemoteCIDR so a
	// stray policy from the sibling probe can never answer for one installed here.
	reqidLocalCIDR  = "10.220.0.0/24"
	reqidRemoteCIDR = "10.221.0.0/24"
	reqidLocalAddr  = "10.220.0.1"
	reqidRemoteAddr = "10.221.0.1"

	// The tunnel endpoints. reqidTunnelSrc is given to the egress device, so the
	// encapsulated packet has somewhere to go: a state whose outer datagram cannot be
	// routed is never used, and the counter would then read as "unresolved".
	reqidTunnelSrc = "192.0.2.10"
	reqidTunnelDst = "192.0.2.20"

	reqidEgressDev = "ze-rq0"
	reqidProbePort = 9

	// One request id, two SPIs. This IS the rekey: the replacement inherits ReqID from
	// the retired pair (newRekeyedChild, engine/rekey.go) and generates a fresh SPI.
	reqidValue = 0x0e31
	reqidSPIA  = 0x0e310001
	reqidSPIB  = 0x0e310002
)

// reqidSPParams is the outbound Child SA policy, installed ONCE and never touched again.
//
// IfID is 0 for the reason ownerSPParams records: __xfrm_policy_match requires the
// policy's if_id to equal the flow's, and ordinary traffic carries 0.
func reqidSPParams() SPParams {
	_, local, _ := net.ParseCIDR(reqidLocalCIDR)
	_, remote, _ := net.ParseCIDR(reqidRemoteCIDR)
	return SPParams{
		Src:       local,
		Dst:       remote,
		Dir:       SADirOut,
		Proto:     ProtoESP,
		Mode:      ModeTunnel,
		ReqID:     reqidValue,
		Priority:  PriorityChildSA,
		Owner:     "reqid-probe",
		TunnelSrc: net.ParseIP(reqidTunnelSrc),
		TunnelDst: net.ParseIP(reqidTunnelDst),
	}
}

// reqidSAParams is one outbound ESP state at the policy's request id. Only the SPI
// varies between the retired pair's state and the replacement's.
func reqidSAParams(spi uint32) SAParams {
	return SAParams{
		SPI:      spi,
		Src:      net.ParseIP(reqidTunnelSrc),
		Dst:      net.ParseIP(reqidTunnelDst),
		Proto:    ProtoESP,
		Mode:     ModeTunnel,
		ReqID:    reqidValue,
		Dir:      SADirOut,
		EncAlgo:  "aes256",
		EncKey:   make([]byte, 32),
		AuthAlgo: "sha256",
		AuthKey:  make([]byte, 32),
	}
}

// reqidNetns gives the probe a namespace with an inner source address, a route to the
// inner destination, and a route to the tunnel destination.
//
// The egress device is a DUMMY rather than loopback, for the reason ownerNetns measured:
// a loopback egress never reaches the outbound XFRM hook, and every reading here would
// then be the silence this test treats as success.
func reqidNetns(t *testing.T) {
	t.Helper()
	encapNetns(t)
	encapNetnsUsable(t)

	egress := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: reqidEgressDev}}
	if err := netlink.LinkAdd(egress); err != nil {
		t.Fatalf("add dummy %s: %v", reqidEgressDev, err)
	}
	link, err := netlink.LinkByName(reqidEgressDev)
	if err != nil {
		t.Fatalf("%s lookup: %v", reqidEgressDev, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("%s up: %v", reqidEgressDev, err)
	}
	// The inner source, and the tunnel source. The second address is what gives the
	// ENCAPSULATED datagram a route to reqidTunnelDst on the same link.
	for _, cidr := range []string{reqidLocalAddr + "/24", reqidTunnelSrc + "/24"} {
		addr, err := netlink.ParseAddr(cidr)
		if err != nil {
			t.Fatalf("parse %s: %v", cidr, err)
		}
		if err := netlink.AddrAdd(link, addr); err != nil {
			t.Fatalf("add %s to %s: %v", cidr, reqidEgressDev, err)
		}
	}
	_, remote, err := net.ParseCIDR(reqidRemoteCIDR)
	if err != nil {
		t.Fatalf("parse %s: %v", reqidRemoteCIDR, err)
	}
	if err := netlink.RouteAdd(&netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       remote,
		Scope:     netlink.SCOPE_LINK,
	}); err != nil {
		t.Fatalf("route %s dev %s: %v", reqidRemoteCIDR, reqidEgressDev, err)
	}

	if got := ownerListPolicies(t); len(got) != 0 {
		t.Fatalf("the fresh namespace already holds %d XFRM policies (%v); no reading below could be attributed", len(got), got)
	}
}

// reqidOutDelta sends ONE datagram down the inner flow and reports how far
// XfrmOutNoStates moved. The write result is reported, never asserted on: the kernel
// answers EAGAIN when a matching policy resolves to nothing, so a failed write is one of
// the expected outcomes of a match rather than the measurement.
func reqidOutDelta(t *testing.T) (int, error) {
	t.Helper()
	before := encapStat(t, ownerStatOutNoStates)

	// A fresh unconnected socket per probe: a connected one caches its bundle and the
	// second send would never consult the policy again.
	var lc net.ListenConfig
	pc, err := lc.ListenPacket(t.Context(), "udp", net.JoinHostPort(reqidLocalAddr, "0"))
	if err != nil {
		t.Fatalf("bind udp on %s: %v", reqidLocalAddr, err)
	}
	_, writeErr := pc.WriteTo([]byte("ze-reqid-probe"),
		&net.UDPAddr{IP: net.ParseIP(reqidRemoteAddr), Port: reqidProbePort})
	if cerr := pc.Close(); cerr != nil {
		t.Errorf("close probe socket: %v", cerr)
	}
	return encapStat(t, ownerStatOutNoStates) - before, writeErr
}

// reqidStatePackets returns how many packets the kernel has put through one state.
func reqidStatePackets(t *testing.T, spi uint32) uint64 {
	t.Helper()
	got, err := netlink.XfrmStateGet(&netlink.XfrmState{
		Src:   net.ParseIP(reqidTunnelSrc),
		Dst:   net.ParseIP(reqidTunnelDst),
		Proto: netlink.XFRM_PROTO_ESP,
		Spi:   int(spi),
	})
	if err != nil {
		t.Fatalf("xfrm state get spi=%#x: %v", spi, err)
	}
	return got.Statistics.Packets
}

// TestXFRMPolicyResolvesToAReplacedState is assumption A-1, measured.
//
// One policy is installed and is never written again. A state arrives at its request id,
// a SECOND state replaces the first at the SAME request id, and the policy must go on
// resolving. That is the entire make-before-break rekey, reduced to the kernel behavior
// it depends on.
func TestXFRMPolicyResolvesToAReplacedState(t *testing.T) {
	// One namespace probe per PROCESS: a second unshare in one binary reads a namespace
	// whose datagrams never reach XFRM, and this test's success condition IS a still
	// counter, so it would manufacture its own false pass (encapOwnProcess).
	if !encapOwnProcess(t) {
		return
	}
	reqidNetns(t)

	b := &xfrmBackend{}
	pol := reqidSPParams()

	if err := b.InstallPolicy(pol); err != nil {
		skipWithoutPolicyPermission(t, err)
		t.Fatalf("installing the Child SA policy: %v", err)
	}
	t.Cleanup(func() {
		if err := b.RemovePolicyParams(pol); err != nil {
			t.Errorf("cleanup: removing the policy: %v", err)
		}
	})

	// CONTROL, taken first. The policy claims the flow and resolves to nothing, so the
	// counter MUST move. Without this reading, every "did not move" below would also be
	// produced by a namespace whose datagrams never reach XFRM at all.
	if delta, writeErr := reqidOutDelta(t); delta < 1 {
		t.Fatalf("control: %s did not move with a policy and no state (delta=%d, write=%v); "+
			"the probe never reached the outbound XFRM hook and nothing below would be a reading",
			ownerStatOutNoStates, delta, writeErr)
	}

	// The retired pair's state.
	if err := b.InstallSA(reqidSAParams(reqidSPIA)); err != nil {
		t.Fatalf("installing state A at reqid %#x: %v", reqidValue, err)
	}
	beforeA := reqidStatePackets(t, reqidSPIA)
	delta, writeErr := reqidOutDelta(t)
	if delta != 0 {
		t.Fatalf("with state A installed at the policy's reqid, %s moved by %d (write=%v); "+
			"the policy did not resolve to a state that shares its request id, and the whole "+
			"rekey design is wrong", ownerStatOutNoStates, delta, writeErr)
	}
	if got := reqidStatePackets(t, reqidSPIA); got <= beforeA {
		t.Fatalf("state A carried no packet (%d -> %d); the silence above was not resolution", beforeA, got)
	}

	// THE REKEY. The replacement's state arrives at the SAME request id, the retired
	// one is removed, and the POLICY IS NOT TOUCHED between the two readings.
	if err := b.InstallSA(reqidSAParams(reqidSPIB)); err != nil {
		t.Fatalf("installing state B at reqid %#x beside state A: %v", reqidValue, err)
	}
	if err := b.RemoveSA(reqidSPIA, net.ParseIP(reqidTunnelDst), ProtoESP); err != nil {
		t.Fatalf("removing the retired state A: %v", err)
	}
	t.Cleanup(func() {
		if err := b.RemoveSA(reqidSPIB, net.ParseIP(reqidTunnelDst), ProtoESP); err != nil {
			t.Errorf("cleanup: removing state B: %v", err)
		}
	})

	beforeB := reqidStatePackets(t, reqidSPIB)
	delta, writeErr = reqidOutDelta(t)
	if delta != 0 {
		t.Fatalf("after the retired state was replaced at the same reqid, %s moved by %d "+
			"(write=%v); an installed policy does NOT follow its request id to a new state, so "+
			"a Child SA rekey must reinstall the policy and this tunnel went dark",
			ownerStatOutNoStates, delta, writeErr)
	}
	if got := reqidStatePackets(t, reqidSPIB); got <= beforeB {
		t.Fatalf("the replacement state carried no packet (%d -> %d); the policy resolved "+
			"somewhere else", beforeB, got)
	}
	t.Logf("one policy resolved to state %#x and then to its replacement %#x at reqid %#x",
		reqidSPIA, reqidSPIB, reqidValue)

	// CONTROL, taken last. Removing the only state must make the counter move again, so
	// the two silences above were readings and not a dead instrument.
	if err := b.RemoveSA(reqidSPIB, net.ParseIP(reqidTunnelDst), ProtoESP); err != nil {
		t.Fatalf("removing state B for the closing control: %v", err)
	}
	delta, writeErr = reqidOutDelta(t)
	// Reinstalled before the assertion, so the deferred cleanup has a state to remove
	// whichever way this reads.
	if err := b.InstallSA(reqidSAParams(reqidSPIB)); err != nil {
		t.Fatalf("reinstalling state B after the closing control: %v", err)
	}
	if delta < 1 {
		t.Errorf("closing control: %s did not move after the last state was removed "+
			"(delta=%d, write=%v); the instrument was dead and the readings above prove nothing",
			ownerStatOutNoStates, delta, writeErr)
	}
}
