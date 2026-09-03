// MEASURES: which of two XFRM states sharing ONE request id the kernel gives an OUTBOUND
// packet, whether that choice follows the SPI or the install order, and what it takes to
// steer it. The inbound half is measured too: which state a received SPI reaches while
// its sibling sends.
//
// xfrm_reqid_resolution_integration_linux_test.go measured the SUCCESSION -- one policy
// resolving to a state and then to its replacement. It never measured the OVERLAP. It
// installs the replacement and removes the predecessor with no packet offered between
// the two calls, so the window a make-before-break rekey opens, both states installed
// and one packet to place, was never probed.
//
// PREVENTS: two designs resting on a reading of that window rather than a measurement.
// installChildTolerant (engine/child.go) installs the successor beside the predecessor
// and assumes traffic moves to the successor; if the kernel kept the predecessor, Ze
// would protect traffic with a key the peer is entitled to have dropped. RFC 4552
// Section 10.1 asks for the same window on purpose for a manual key rollover: step 2 is
// to "replace the original outbound SA with one using the new SPI and key values" while
// both receiving SAs stay installed.
//
// THE INSTRUMENT IS THE PER-STATE PACKET COUNTER, never the installed state: a state
// that exists is not a state that was chosen. XfrmOutNoStates brackets every reading, as
// in the sibling file, so a namespace whose datagrams never reach the outbound XFRM hook
// cannot pass as an answer.

//go:build integration && linux

package dataplane

import (
	"net"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

const (
	// A third SPI, for the one state in this file that carries no mark. It is the
	// control of the mark probe: without it, "the marked states were skipped" and "the
	// namespace stopped resolving anything" are the same reading.
	overlapSPIC = 0x0e310003

	// Two marks, one for each state of the pair. The kernel matches a state's mark
	// against the mark the POLICY carries, so these are what a rekey would have to put
	// on its states to steer the choice with, and the policy would have to carry one too.
	overlapMarkOld  = 0x0e310010
	overlapMarkNew  = 0x0e310020
	overlapMarkMask = 0xffffffff
)

// overlapStats returns the kernel's own accounting for one state.
//
// reqidStatePackets answers the packet count alone. Every reading here also needs
// AddTime and UseTime, because they are the two fields that could order two candidate
// states at one request id, and mark, because a state that carries one is not found
// without it.
func overlapStats(t *testing.T, spi uint32, mark *netlink.XfrmMark) netlink.XfrmStateStats {
	t.Helper()
	got, err := netlink.XfrmStateGet(&netlink.XfrmState{
		Src:   net.ParseIP(reqidTunnelSrc),
		Dst:   net.ParseIP(reqidTunnelDst),
		Proto: netlink.XFRM_PROTO_ESP,
		Spi:   int(spi),
		Mark:  mark,
	})
	if err != nil {
		t.Fatalf("xfrm state get spi=%#x mark=%v: %v", spi, mark, err)
	}
	return got.Statistics
}

// overlapWhichCarried offers ONE packet to the installed policy and answers which of the
// candidate states protected it.
//
// It installs nothing and removes nothing, so a caller can take several readings against
// one population of states while it changes the POLICY between them.
func overlapWhichCarried(t *testing.T, candidates []uint32, mark *netlink.XfrmMark, socketMark uint32) uint32 {
	t.Helper()
	before := make([]uint64, len(candidates))
	for i, spi := range candidates {
		before[i] = overlapStats(t, spi, mark).Packets
	}

	delta, writeErr := overlapProbe(t, socketMark)
	if delta != 0 {
		t.Fatalf("with %d states installed at the policy's reqid, %s moved by %d (write=%v); "+
			"the policy claimed the datagram and resolved to no state at all",
			len(candidates), ownerStatOutNoStates, delta, writeErr)
	}

	var carried []uint32
	for i, spi := range candidates {
		after := overlapStats(t, spi, mark)
		t.Logf("state %#x: packets %d -> %d, add_time=%d use_time=%d",
			spi, before[i], after.Packets, after.AddTime, after.UseTime)
		if after.Packets > before[i] {
			carried = append(carried, spi)
		}
	}
	if len(carried) != 1 {
		t.Fatalf("expected exactly ONE state of %#x to carry the packet, %d did (%#x); "+
			"the reading below would name a state the kernel did not single out",
			candidates, len(carried), carried)
	}
	return carried[0]
}

// overlapCarrierFor installs the states in the ORDER given, takes one reading, and
// removes them again.
//
// The order is the parameter because it is one of the two candidate rules. The kernel
// could order two states at one request id by the SPI it was handed or by when the state
// arrived, and only running both orders separates them.
func overlapCarrierFor(t *testing.T, b *xfrmBackend, order []uint32) uint32 {
	t.Helper()
	for _, spi := range order {
		if err := b.InstallSA(reqidSAParams(spi)); err != nil {
			t.Fatalf("installing state %#x at reqid %#x: %v", spi, reqidValue, err)
		}
	}
	defer func() {
		for _, spi := range order {
			if err := b.RemoveSA(spi, net.ParseIP(reqidTunnelDst), ProtoESP); err != nil {
				t.Errorf("removing state %#x: %v", spi, err)
			}
		}
	}()
	return overlapWhichCarried(t, order, nil, 0)
}

// overlapProbe sends ONE datagram down the inner flow with the socket mark given, and
// reports how far XfrmOutNoStates moved.
//
// It is reqidOutDelta with a socket mark. The two are kept apart rather than merged:
// reqidOutDelta is the instrument of the succession measurement beside this file, every
// reading it has taken was taken on an unmarked socket, and a mark parameter there would
// put this file's concern into all of them.
func overlapProbe(t *testing.T, mark uint32) (int, error) {
	t.Helper()
	before := encapStat(t, ownerStatOutNoStates)

	// A fresh unconnected socket per probe: a connected one caches its bundle and the
	// second send would never consult the policy again.
	var lc net.ListenConfig
	pc, err := lc.ListenPacket(t.Context(), "udp", net.JoinHostPort(reqidLocalAddr, "0"))
	if err != nil {
		t.Fatalf("bind udp on %s: %v", reqidLocalAddr, err)
	}
	if mark != 0 {
		overlapMarkSocket(t, pc, mark)
	}
	_, writeErr := pc.WriteTo([]byte("ze-overlap-probe"),
		&net.UDPAddr{IP: net.ParseIP(reqidRemoteAddr), Port: reqidProbePort})
	if cerr := pc.Close(); cerr != nil {
		t.Errorf("close probe socket: %v", cerr)
	}
	return encapStat(t, ownerStatOutNoStates) - before, writeErr
}

// overlapMarkSocket puts an SO_MARK on the probe socket, which is what makes the flow
// carry the mark a marked policy matches on.
func overlapMarkSocket(t *testing.T, pc net.PacketConn, mark uint32) {
	t.Helper()
	uc, ok := pc.(*net.UDPConn)
	if !ok {
		t.Fatalf("the probe socket is %T, and only a *net.UDPConn can be marked", pc)
	}
	rc, err := uc.SyscallConn()
	if err != nil {
		t.Fatalf("raw conn for the probe socket: %v", err)
	}
	var setErr error
	if ctlErr := rc.Control(func(fd uintptr) {
		setErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_MARK, int(mark))
	}); ctlErr != nil {
		t.Fatalf("control the probe socket: %v", ctlErr)
	}
	if setErr != nil {
		t.Fatalf("SO_MARK %#x on the probe socket: %v", mark, setErr)
	}
}

// overlapPinTemplateSPI rewrites the installed policy so its template names ONE SPI.
//
// It goes to netlink rather than through the backend because SPParams has no field for
// it: xfrmPolicyFromParams leaves the template's Spi at zero, which is what lets the
// kernel choose among every state at the request id. The policy is otherwise byte for
// byte the one InstallPolicy wrote, and XFRM_MSG_UPDPOLICY replaces it in place
// (xfrmBackend.InstallPolicy records why every policy is upserted).
func overlapPinTemplateSPI(t *testing.T, spi uint32) {
	t.Helper()
	pol, err := xfrmPolicyFromParams(reqidSPParams())
	if err != nil {
		t.Fatalf("building the policy: %v", err)
	}
	pol.Tmpls[0].Spi = int(spi)
	if err := netlink.XfrmPolicyUpdate(pol); err != nil {
		t.Fatalf("pinning the policy template to spi %#x: %v", spi, err)
	}
}

// overlapMarkedState builds one state of the pair with a mark on it. SAParams carries no
// mark, so this is the netlink state xfrmStateFromParams produces with the mark added.
func overlapMarkedState(t *testing.T, spi, mark uint32) *netlink.XfrmState {
	t.Helper()
	state, err := xfrmStateFromParams(reqidSAParams(spi))
	if err != nil {
		t.Fatalf("building state %#x: %v", spi, err)
	}
	state.Mark = &netlink.XfrmMark{Value: mark, Mask: overlapMarkMask}
	return state
}

// overlapInboundSAParams is one INBOUND state of the pair. Source and destination are
// loopback, because an inbound reading needs a datagram this namespace can deliver to
// itself, and the inbound lookup takes the destination, the SPI, the protocol and the
// family. It shares reqidValue with its sibling, which is the condition under test.
func overlapInboundSAParams(spi uint32) SAParams {
	return SAParams{
		SPI:      spi,
		Src:      net.ParseIP(encapLoopbackAddr),
		Dst:      net.ParseIP(encapLoopbackAddr),
		Proto:    ProtoESP,
		Mode:     ModeTunnel,
		ReqID:    reqidValue,
		Dir:      SADirIn,
		EncAlgo:  "aes256",
		EncKey:   make([]byte, 32),
		AuthAlgo: "sha256",
		AuthKey:  make([]byte, 32),
	}
}

// overlapInNoStates sends ONE bare ESP datagram carrying the SPI given and reports how
// far XfrmInNoStates moved.
//
// The datagram never decrypts: the reading is whether the kernel FOUND a state for that
// SPI, and it raises this counter only when it found none. A found state raises a
// different counter, and which one depends on the transform, so this probe reads the one
// counter that answers the question asked.
func overlapInNoStates(t *testing.T, spi uint32) int {
	t.Helper()
	before := encapStat(t, encapStatNoStates)

	c, err := (&net.ListenConfig{}).ListenPacket(t.Context(), "ip4:esp", encapLoopbackAddr)
	if err != nil {
		t.Skipf("no raw ESP socket (needs CAP_NET_RAW): %v", err)
	}
	_, writeErr := c.WriteTo(encapESPBytes(spi), &net.IPAddr{IP: net.ParseIP(encapLoopbackAddr)})
	if cerr := c.Close(); cerr != nil {
		t.Errorf("close raw socket: %v", cerr)
	}
	if writeErr != nil {
		t.Fatalf("send bare ESP spi=%#x: %v", spi, writeErr)
	}
	return encapStat(t, encapStatNoStates) - before
}

// TestXFRMOutboundPicksTheNewestStateAtOneReqID is the overlap window, measured.
//
// Two states sit at one request id under one policy, and one packet is offered. The
// reading names which state protected it, and the pair is then installed the other way
// round so an answer that follows the SPI and an answer that follows the install order
// disagree. That second reading is the one that separates them: the newest state now
// holds the LOWER SPI.
func TestXFRMOutboundPicksTheNewestStateAtOneReqID(t *testing.T) {
	// One namespace probe per PROCESS (encapOwnProcess). A second unshare in one binary
	// reads a namespace whose datagrams never reach XFRM, and the controls below would
	// then fail rather than mislead, which is still a wasted run.
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
	// counter MUST move. Without this reading, a probe that never reached the outbound
	// XFRM hook would report "no state carried it" for every case below.
	if delta, writeErr := overlapProbe(t, 0); delta < 1 {
		t.Fatalf("control: %s did not move with a policy and no state (delta=%d, write=%v); "+
			"the probe never reached the outbound XFRM hook and nothing below is a reading",
			ownerStatOutNoStates, delta, writeErr)
	}

	// The rekey's own order: the predecessor first, the successor second.
	if got := overlapCarrierFor(t, b, []uint32{reqidSPIA, reqidSPIB}); got != reqidSPIB {
		t.Errorf("predecessor %#x installed first, successor %#x second: the kernel protected "+
			"the packet with %#x. A make-before-break rekey that installs the successor and "+
			"waits does NOT move traffic onto it, so installChildTolerant (engine/child.go) "+
			"has to remove the predecessor or steer the choice",
			reqidSPIA, reqidSPIB, got)
	}

	// The same pair the other way round. The newest state now carries the LOWER SPI, so
	// "the kernel follows the SPI" and "the kernel follows the install order" answer
	// differently here.
	if got := overlapCarrierFor(t, b, []uint32{reqidSPIB, reqidSPIA}); got != reqidSPIA {
		t.Errorf("installed %#x first and %#x second, the kernel protected the packet with "+
			"%#x; the choice does not follow the install order, and a rekey cannot rely on "+
			"installing the successor last", reqidSPIB, reqidSPIA, got)
	}
}

// TestXFRMOutboundTemplateSPIPinsOneStateAtOneReqID measures the first steering
// candidate: a policy template that names the SPI.
//
// Both states stay installed for every reading. Only the POLICY changes, which is
// exactly the shape RFC 4552 Section 10.1 step 2 describes for a manual rollover, and
// the reverse move back to the predecessor is measured too.
func TestXFRMOutboundTemplateSPIPinsOneStateAtOneReqID(t *testing.T) {
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

	// CONTROL, taken first, for the reason the sibling test records.
	if delta, writeErr := overlapProbe(t, 0); delta < 1 {
		t.Fatalf("control: %s did not move with a policy and no state (delta=%d, write=%v)",
			ownerStatOutNoStates, delta, writeErr)
	}

	pair := []uint32{reqidSPIA, reqidSPIB}
	for _, spi := range pair {
		if err := b.InstallSA(reqidSAParams(spi)); err != nil {
			t.Fatalf("installing state %#x at reqid %#x: %v", spi, reqidValue, err)
		}
		t.Cleanup(func() {
			if err := b.RemoveSA(spi, net.ParseIP(reqidTunnelDst), ProtoESP); err != nil {
				t.Errorf("cleanup: removing state %#x: %v", spi, err)
			}
		})
	}

	// The predecessor, named by the template while the successor stays installed.
	overlapPinTemplateSPI(t, reqidSPIA)
	if got := overlapWhichCarried(t, pair, nil, 0); got != reqidSPIA {
		t.Fatalf("the template named spi %#x and the kernel protected the packet with %#x; "+
			"a template SPI does not pin the state, so an RFC 4552 Section 10.1 rollover "+
			"cannot be built on one", reqidSPIA, got)
	}

	// The rollover itself: the template names the successor, and nothing else moves.
	overlapPinTemplateSPI(t, reqidSPIB)
	if got := overlapWhichCarried(t, pair, nil, 0); got != reqidSPIB {
		t.Fatalf("the template moved to spi %#x and the kernel protected the packet with "+
			"%#x; the pin is not revisited once it is set, and a rollover would strand "+
			"traffic on the retired key", reqidSPIB, got)
	}

	// And back. A pin that could not be released would leave the retired SPI named in a
	// policy nobody can retire.
	overlapPinTemplateSPI(t, reqidSPIA)
	if got := overlapWhichCarried(t, pair, nil, 0); got != reqidSPIA {
		t.Fatalf("the template moved back to spi %#x and the kernel protected the packet "+
			"with %#x", reqidSPIA, got)
	}
}

// TestXFRMStateMarkNeedsThePolicyToCarryItToo measures the second steering candidate: an
// XFRM mark on the states.
//
// The reading has two halves, and the second is what makes the first usable. A marked
// state is INVISIBLE to the unmarked policy that would otherwise resolve to it, so a
// rekey cannot mark its states and leave the policy alone. A policy that carries the
// mark does select the state that matches it, on traffic that carries the mark too.
func TestXFRMStateMarkNeedsThePolicyToCarryItToo(t *testing.T) {
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
	unmarkedPolicyGone := false
	t.Cleanup(func() {
		if unmarkedPolicyGone {
			return
		}
		if err := b.RemovePolicyParams(pol); err != nil {
			t.Errorf("cleanup: removing the unmarked policy: %v", err)
		}
	})

	// CONTROL, taken first.
	if delta, writeErr := overlapProbe(t, 0); delta < 1 {
		t.Fatalf("control: %s did not move with a policy and no state (delta=%d, write=%v)",
			ownerStatOutNoStates, delta, writeErr)
	}

	oldState := overlapMarkedState(t, reqidSPIA, overlapMarkOld)
	newState := overlapMarkedState(t, reqidSPIB, overlapMarkNew)
	for _, state := range []*netlink.XfrmState{oldState, newState} {
		if err := netlink.XfrmStateAdd(state); err != nil {
			t.Fatalf("installing marked state spi=%#x: %v", state.Spi, err)
		}
		t.Cleanup(func() {
			if err := netlink.XfrmStateDel(state); err != nil {
				t.Errorf("cleanup: removing marked state spi=%#x: %v", state.Spi, err)
			}
		})
	}

	// Both states sit at the policy's request id, and both carry a mark the policy does
	// not. The counter moving IS the reading here.
	delta, writeErr := overlapProbe(t, 0)
	if delta < 1 {
		t.Fatalf("the unmarked policy resolved to a MARKED state at its reqid (%s moved by "+
			"%d, write=%v); a mark on a state is not a condition of its selection, and the "+
			"reading below cannot be trusted", ownerStatOutNoStates, delta, writeErr)
	}

	// CONTROL for that silence, and the reason it is not "the namespace stopped
	// resolving": one UNMARKED state at the same request id, and the policy takes it.
	if err := b.InstallSA(reqidSAParams(overlapSPIC)); err != nil {
		t.Fatalf("installing the unmarked control state %#x: %v", overlapSPIC, err)
	}
	if got := overlapWhichCarried(t, []uint32{overlapSPIC}, nil, 0); got != overlapSPIC {
		t.Fatalf("the unmarked control state %#x did not carry the packet, %#x did", overlapSPIC, got)
	}
	if err := b.RemoveSA(overlapSPIC, net.ParseIP(reqidTunnelDst), ProtoESP); err != nil {
		t.Fatalf("removing the unmarked control state %#x: %v", overlapSPIC, err)
	}

	// The other half. The unmarked policy goes first, because a policy is keyed by its
	// mark as well as its selector, so a marked one is a SECOND policy over the same
	// flow rather than a replacement, and two of them would leave the reading to a
	// priority tie.
	if err := b.RemovePolicyParams(pol); err != nil {
		t.Fatalf("removing the unmarked policy: %v", err)
	}
	unmarkedPolicyGone = true

	marked, err := xfrmPolicyFromParams(pol)
	if err != nil {
		t.Fatalf("building the marked policy: %v", err)
	}
	marked.Mark = &netlink.XfrmMark{Value: overlapMarkOld, Mask: overlapMarkMask}
	if err := netlink.XfrmPolicyUpdate(marked); err != nil {
		t.Fatalf("installing the marked policy: %v", err)
	}
	t.Cleanup(func() {
		if err := netlink.XfrmPolicyDel(marked); err != nil {
			t.Errorf("cleanup: removing the marked policy: %v", err)
		}
	})

	oldMark := &netlink.XfrmMark{Value: overlapMarkOld, Mask: overlapMarkMask}
	newMark := &netlink.XfrmMark{Value: overlapMarkNew, Mask: overlapMarkMask}
	beforeNew := overlapStats(t, reqidSPIB, newMark).Packets
	if got := overlapWhichCarried(t, []uint32{reqidSPIA}, oldMark, overlapMarkOld); got != reqidSPIA {
		t.Fatalf("the policy carried mark %#x and the packet carried it too, and state %#x "+
			"did not protect the packet", overlapMarkOld, reqidSPIA)
	}
	if got := overlapStats(t, reqidSPIB, newMark).Packets; got != beforeNew {
		t.Errorf("the state marked %#x also carried the packet (%d -> %d); the mark does not "+
			"single one state out", overlapMarkNew, beforeNew, got)
	}
}

// TestXFRMInboundReachesTheStateTheSPINames measures the fourth question: what becomes
// of the OTHER state while its sibling sends.
//
// Two inbound states share one request id, and a datagram is offered for each SPI in
// turn. A request id that decided the inbound side too would leave one of the two
// unreachable, and a peer that is still sending under the retired key would go unheard
// through the whole rekey.
func TestXFRMInboundReachesTheStateTheSPINames(t *testing.T) {
	if !encapOwnProcess(t) {
		return
	}
	encapNetns(t)
	encapNetnsUsable(t)

	b := &xfrmBackend{}
	for _, spi := range []uint32{reqidSPIA, reqidSPIB} {
		// No skip on a permission error here. encapNetns has already entered a network
		// namespace of its own, which needs the same capability, so a refusal at this
		// point is a defect rather than an unprivileged host.
		if err := b.InstallSA(overlapInboundSAParams(spi)); err != nil {
			t.Fatalf("installing inbound state %#x at reqid %#x: %v", spi, reqidValue, err)
		}
		t.Cleanup(func() {
			if err := b.RemoveSA(spi, net.ParseIP(encapLoopbackAddr), ProtoESP); err != nil {
				t.Errorf("cleanup: removing inbound state %#x: %v", spi, err)
			}
		})
	}

	// CONTROL, taken first. An SPI neither state holds MUST raise the counter, or every
	// silence below is a datagram that died before the inbound XFRM hook.
	if delta := overlapInNoStates(t, encapSPIUnknown); delta < 1 {
		t.Fatalf("control: %s did not move for spi %#x, which no state holds (delta=%d); "+
			"the datagram never reached the inbound XFRM hook", encapStatNoStates, encapSPIUnknown, delta)
	}

	if delta := overlapInNoStates(t, reqidSPIA); delta != 0 {
		t.Errorf("the predecessor's spi %#x raised %s (delta=%d); the retired key stops "+
			"receiving the moment its successor is installed at the same reqid",
			reqidSPIA, encapStatNoStates, delta)
	}
	if delta := overlapInNoStates(t, reqidSPIB); delta != 0 {
		t.Errorf("the successor's spi %#x raised %s (delta=%d); the two states at one reqid "+
			"do not both receive", reqidSPIB, encapStatNoStates, delta)
	}
}
