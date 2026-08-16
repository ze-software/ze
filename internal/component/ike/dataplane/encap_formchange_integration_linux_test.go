//go:build integration && linux

package dataplane

import (
	"net"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// Phase 5 evidence for plan/spec-ipsec-esp-dual-form-receive.md, acceptance criterion AC-4.
//
// AC-4 reads: "An established Child SA, and the peer changes ESP form -> traffic keeps
// flowing in both directions. The SA is not rekeyed and not deleted."
//
// The sibling TestEncapOneStateAcceptsBothForms measures that ONE state accepts both
// forms. It is necessary and not sufficient for AC-4, and it differs from this probe in
// two ways that matter:
//
//	1. It installs its state with a netlink literal and re-presents the refused datagram
//	   BY HAND. So it measures the mechanism and never the shipped code that drives it:
//	   xfrmBackend.InstallSA, espFormReceiver.Watch and the espFormReceiver.run loop are
//	   all absent from that path. Nothing in this package runs that loop against a real
//	   kernel; espform_run_linux_test.go drives it through a recorder.
//	2. It sends each form once, to a state that never carried traffic. AC-4 is about a
//	   LIVE SA whose peer switches form part way through, which is a sequence rather than
//	   a pair of independent readings.
//
// This probe closes both. It installs the SA through the production backend, sends the
// form the kernel serves, then switches to the other form mid-flight, then switches back,
// and finally reads the kernel's own state table to prove the SA behind those three
// exchanges was one SA throughout.
//
// It carries no RFC requirement tag. test/ipsec-interop is TIER_UNRUN and this package is
// integration tier, so the compliance evidence for RFC 7296 Section 2.23 stays with the
// unit-tier tagged pairs in engine/rfc7296_natt_bothforms_test.go. This probe records
// that the shipped receive path works on a live SA.

const (
	encapSPIFormChange = 0x00ABCDF0

	// encapFormChangeWait bounds the wait for a re-presented datagram. Re-presentation
	// is asynchronous: espFormReceiver.run reads the refused datagram on its own
	// goroutine and injects it, so the counter moves after the send returns rather than
	// during it.
	encapFormChangeWait = 3 * time.Second

	// encapFormChangePoll is how often the counter is re-read while waiting.
	encapFormChangePoll = 20 * time.Millisecond
)

// encapWaitForStat waits until one named xfrm counter rises above want, and reports the
// value it reached. It reports false on timeout, and the CALLER decides whether that is a
// failure, so a probe measuring an absence can use the same helper as one measuring an
// arrival.
//
// A boolean return rather than a t.Fatal: the two uses below need opposite verdicts from
// the same wait.
func encapWaitForStat(t *testing.T, name string, above int) (int, bool) {
	t.Helper()
	deadline := time.Now().Add(encapFormChangeWait)
	for {
		if got := encapStat(t, name); got > above {
			return got, true
		}
		if time.Now().After(deadline) {
			return encapStat(t, name), false
		}
		time.Sleep(encapFormChangePoll)
	}
}

// encapFormChangeSA describes the inbound half of a Child SA that floated to port 4500,
// which is the SA shape installChildSA produces for a peer whose IKE runs there
// (engine/child.go: child.UDPEncap gives the template, and inbound.AcceptBothESPForms is
// set unconditionally on the inbound state).
func encapFormChangeSA() SAParams {
	return SAParams{
		SPI:                encapSPIFormChange,
		Src:                net.ParseIP(encapLoopbackPeerAddr),
		Dst:                net.ParseIP(encapLoopbackAddr),
		Proto:              ProtoESP,
		Mode:               ModeTunnel,
		ReqID:              1,
		EncAlgo:            "aes256gcm",
		EncKey:             append([]byte(nil), encapTestAEADKey...),
		IsAEAD:             true,
		UDPEncap:           true,
		UDPEncapSPort:      encapNATTPortForTests,
		UDPEncapDPort:      encapNATTPortForTests,
		AcceptBothESPForms: true,
	}
}

// encapFormChangeNATTSocket binds the port-4500 socket in the state production runs it:
// UDP_ENCAP_ESPINUDP set, so the kernel strips the header of both the peer's own
// encapsulated datagrams and the ones espFormReceiver re-presents
// (engine/register.go, transport.EnableESPInUDP).
func encapFormChangeNATTSocket(t *testing.T) *net.UDPConn {
	t.Helper()
	uc, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP(encapLoopbackAddr), Port: encapNATTPortForTests})
	if err != nil {
		t.Fatalf("bind udp %d: %v", encapNATTPortForTests, err)
	}
	rc, err := uc.SyscallConn()
	if err != nil {
		t.Fatalf("syscall conn: %v", err)
	}
	var setErr error
	if ctlErr := rc.Control(func(fd uintptr) {
		setErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_UDP, unix.UDP_ENCAP, unix.UDP_ENCAP_ESPINUDP)
	}); ctlErr != nil {
		t.Fatalf("control: %v", ctlErr)
	}
	if setErr != nil {
		t.Fatalf("UDP_ENCAP: %v", setErr)
	}
	return uc
}

// encapFormChangeState reads back the one inbound state for the probe's SPI. It fails when
// the count is not exactly one, because both other counts are the failures AC-4 forbids:
// zero means the SA was deleted, and two means it was replaced without the old one going.
func encapFormChangeState(t *testing.T, when string) netlink.XfrmState {
	t.Helper()
	states, err := netlink.XfrmStateList(netlink.FAMILY_V4)
	if err != nil {
		t.Fatalf("list xfrm states %s: %v", when, err)
	}
	var found []netlink.XfrmState
	for i := range states {
		if states[i].Spi == encapSPIFormChange {
			found = append(found, states[i])
		}
	}
	if len(found) != 1 {
		t.Fatalf("AC-4: %s the kernel holds %d states for spi %#x, want exactly 1; 0 means the SA was deleted and 2 means it was replaced",
			when, len(found), uint32(encapSPIFormChange))
	}
	return found[0]
}

// VALIDATES: AC-4. A Child SA that is already carrying traffic keeps carrying it when the
// peer changes ESP wire form, and the SA behind it is neither rekeyed nor deleted.
// PREVENTS: shipping the dual-form receive path on the strength of a hand-driven
// measurement. TestEncapOneStateAcceptsBothForms re-presents the refused datagram itself,
// so it stays green even if xfrmBackend.InstallSA never calls Watch and the
// espFormReceiver.run loop never reads a byte. This probe installs through the production
// backend and sends nothing back by hand, so the shipped loop is the only thing that can
// make row 2 pass.
//
// The sequence is the point. Row 1 and row 3 are the SAME form, on either side of the
// change, so a receive path that recovered the second form by breaking the first would
// fail row 3. Row 4 reads the kernel's state table to prove one SA served all three.
//
// One goroutine, no subtests. runtime.LockOSThread binds the NAMESPACE to this goroutine's
// thread, and a t.Run subtest would read the HOST namespace instead.
func TestEncapEstablishedSAServesAPeerFormChange(t *testing.T) {
	if !encapOwnProcess(t) {
		return
	}
	encapNetns(t)
	encapNetnsUsable(t)

	uc := encapFormChangeNATTSocket(t)
	defer func() {
		if cerr := uc.Close(); cerr != nil {
			t.Errorf("close udp: %v", cerr)
		}
	}()

	// The PRODUCTION backend. newXFRMBackend builds the espFormReceiver, and InstallSA is
	// what starts it (xfrm_linux.go: the AcceptBothESPForms && UDPEncap branch calls
	// Watch). Nothing below re-presents a datagram by hand.
	dp, err := newXFRMBackend()
	if err != nil {
		t.Fatalf("build the production xfrm backend: %v", err)
	}
	defer func() {
		if cerr := dp.Close(); cerr != nil {
			t.Errorf("close backend: %v", cerr)
		}
	}()

	// The INBOUND policy, because installChildSA installs one beside every Child SA
	// (engine/child.go) and a probe without it is further from production than it looks.
	//
	// It does NOT make this probe reproduce every production failure, and saying so is
	// the point. net/ipv4/raw.c raw_rcv runs xfrm4_policy_check before it queues a
	// packet to a raw socket, and a policy demanding ESP rejects the very datagram the
	// receiver was opened to recover. That defect was live and this probe could not see
	// it: __xfrm_policy_check2 short-circuits on DST_NOPOLICY, which every loopback dst
	// entry carries, so the check the daemon trips is skipped here. MEASURED on
	// 2026-08-03, by removing the per-socket bypass from espFormReceiver.startLocked and
	// re-running: this probe stayed green while the strongSwan lab lost 100% of its
	// traffic.
	//
	// So the proof for that half lives where a peer sends over a real interface, in
	// test/ipsec-interop/scenarios/23-esp-form-change. This probe owns the kernel
	// mechanism and the SA's identity across the change (ai/rules/evidence.md).
	anyV4 := &net.IPNet{IP: net.IPv4zero.To4(), Mask: net.CIDRMask(0, 32)}
	if err := dp.InstallPolicy(SPParams{
		Src:       anyV4,
		Dst:       anyV4,
		Dir:       SADirIn,
		Proto:     ProtoESP,
		Mode:      ModeTunnel,
		ReqID:     1,
		TunnelSrc: net.ParseIP(encapLoopbackPeerAddr),
		TunnelDst: net.ParseIP(encapLoopbackAddr),
	}); err != nil {
		t.Fatalf("install the inbound Child SA policy: %v", err)
	}

	if err := dp.InstallSA(encapFormChangeSA()); err != nil {
		t.Fatalf("install the inbound Child SA through the production backend: %v", err)
	}

	installed := encapFormChangeState(t, "after install")
	if installed.Encap == nil {
		t.Fatalf("the installed state carries no encapsulation template, so this probe measures the template-free case and not the one AC-4 is about")
	}

	// The peer sends toward the local endpoint from its own address, exactly as the SA
	// was installed. A datagram from anywhere else would be a different measurement.
	sender, err := net.DialUDP("udp4",
		&net.UDPAddr{IP: net.ParseIP(encapLoopbackPeerAddr), Port: encapNATTPortForTests},
		&net.UDPAddr{IP: net.ParseIP(encapLoopbackAddr), Port: encapNATTPortForTests})
	if err != nil {
		t.Fatalf("dial udp from the peer address: %v", err)
	}
	defer func() {
		if cerr := sender.Close(); cerr != nil {
			t.Errorf("close sender: %v", cerr)
		}
	}()

	sendEncapsulated := func() {
		if _, werr := sender.Write(encapESPBytes(encapSPIFormChange)); werr != nil {
			t.Fatalf("send encapsulated ESP: %v", werr)
		}
	}

	// Row 1: the SA is live and carrying the form its template selects.
	if got := encapVerdictOf(t, sendEncapsulated); got != encapStatProtoError {
		t.Fatalf("the established SA raised %s for the form its template selects, want %s; the SA is not carrying traffic and the rows below would measure nothing",
			got, encapStatProtoError)
	}

	// Row 2, the form change. The peer switches to bare ESP on the SAME live SA. XFRM
	// refuses it (XfrmInStateMismatch), and the shipped receiver must recover it, so the
	// datagram reaches the crypto check anyway.
	//
	// Two counters move here, so encapVerdictOf cannot be used: the kernel's own refusal
	// is expected and is precisely what the receiver exists to recover from. The
	// assertion is that ProtoError rises as well, and it is read after a bounded wait
	// because the recovery runs on the receiver's goroutine.
	mismatchBefore := encapStat(t, encapStatMismatch)
	protoBefore := encapStat(t, encapStatProtoError)
	encapInjectBare(t, encapLoopbackPeerAddr, encapESPBytes(encapSPIFormChange))

	if _, ok := encapWaitForStat(t, encapStatProtoError, protoBefore); !ok {
		t.Fatalf("AC-4: after the peer changed to bare ESP on the live SA, %s never rose within %v. The datagram never reached the crypto check, so the tunnel stops carrying traffic when the peer changes form",
			encapStatProtoError, encapFormChangeWait)
	}
	if encapStat(t, encapStatMismatch) <= mismatchBefore {
		t.Errorf("%s did not rise for the bare datagram. XFRM was expected to refuse it first, so this probe is not exercising the recovery path it claims to",
			encapStatMismatch)
	}

	// Row 3: the peer changes back. Same form as row 1, on the far side of the change.
	// A receive path that served the second form by disturbing the first fails here.
	if got := encapVerdictOf(t, sendEncapsulated); got != encapStatProtoError {
		t.Errorf("AC-4: after the form change the SA raised %s for the ORIGINAL form, want %s; traffic no longer flows in the direction that worked before",
			got, encapStatProtoError)
	}

	// Row 4: the SA is not rekeyed and not deleted. The kernel's own state table is the
	// witness. A rekey installs a new state and removes the old one, so the SPI would be
	// gone or its add time would have moved.
	after := encapFormChangeState(t, "after the form change")
	if after.Statistics.AddTime != installed.Statistics.AddTime {
		t.Errorf("AC-4: the state's add time moved from %d to %d across the form change, so the SA was replaced rather than kept",
			installed.Statistics.AddTime, after.Statistics.AddTime)
	}
	if after.Encap == nil {
		t.Errorf("AC-4: the state lost its encapsulation template across the form change; the template is what the kernel serves and nothing may rewrite it on a live SA")
	}

	// The discrimination control, on the same goroutine and namespace. An SPI with no
	// state raises a THIRD counter, so the rows above are real readings and not a counter
	// that moves for everything.
	if got := encapVerdictOf(t, func() {
		encapInjectBare(t, encapLoopbackPeerAddr, encapESPBytes(encapSPIUnknown))
	}); got != encapStatNoStates {
		t.Errorf("an SPI with no state raised %s, want %s; the rows above prove nothing",
			got, encapStatNoStates)
	}
}
