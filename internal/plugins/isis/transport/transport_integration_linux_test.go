//go:build integration && linux

// Design: plan/spec-isis-3-l2-transport.md -- raw L2 QEMU integration tests
//
// These exercise the real AF_PACKET/SOCK_RAW backend on a veth pair inside a
// dedicated network namespace. They require CAP_NET_ADMIN (to create the netns
// and veth) and CAP_NET_RAW (to open the raw socket); when those are missing the
// tests t.Skip rather than fail (e.g. on a developer laptop). They run under the
// `ze-qemu-integration-test` target, which derives its package set from the
// `integration && linux` build tag (mk/test-integration.mk), and are also listed
// in scripts/evidence/qemu-all-tests.sh.
//
// They validate spec assumptions A-1 (PPPoE AF_PACKET pattern generalises to
// 802.3+LLC), A-2 (raw multicast receive of ISO MACs via PACKET_ADD_MEMBERSHIP,
// no promiscuous mode), A-4 (ioctl MTU exposed; smaller peer frame infers a
// neighbor MTU), and A-5 (raw-socket open under CAP_NET_RAW).

package transport

import (
	"bytes"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/ze-software/ze/internal/component/iface"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink" // register the netlink iface backend so iface.Resolve works
)

const (
	vethA = "zeisis0"
	vethB = "zeisis1"
)

func nsName(testName string) string {
	name := strings.NewReplacer("/", "_", " ", "_", "(", "", ")", "").Replace(testName)
	if len(name) > 8 {
		name = name[len(name)-8:]
	}
	return "zisis_" + name
}

// withVethPair creates a veth pair in a fresh network namespace and brings both
// ends up, running fn inside that namespace. It skips when the required
// capabilities are absent.
func withVethPair(t *testing.T, mtuA, mtuB int, fn func()) {
	t.Helper()

	runtime.LockOSThread()
	unlocked := false
	unlock := func() {
		if !unlocked {
			runtime.UnlockOSThread()
			unlocked = true
		}
	}

	origNS, err := netns.Get()
	if err != nil {
		unlock()
		t.Skipf("requires CAP_NET_ADMIN: %v", err)
	}

	name := nsName(t.Name())
	newNS, err := netns.NewNamed(name)
	if err != nil {
		origNS.Close()
		unlock()
		t.Skipf("requires CAP_NET_ADMIN: %v", err)
	}

	t.Cleanup(func() {
		if rerr := netns.Set(origNS); rerr != nil {
			t.Errorf("restore namespace: %v", rerr)
		}
		origNS.Close()
		newNS.Close()
		netns.DeleteNamed(name) //nolint:errcheck // best-effort cleanup
		unlock()
	})

	if err := netlink.LinkAdd(&netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: vethA, MTU: mtuA},
		PeerName:  vethB,
	}); err != nil {
		t.Skipf("add veth (needs CAP_NET_ADMIN): %v", err)
	}
	for name, mtu := range map[string]int{vethA: mtuA, vethB: mtuB} {
		link, lerr := netlink.LinkByName(name)
		if lerr != nil {
			t.Fatalf("link %q: %v", name, lerr)
		}
		if mtu > 0 {
			if serr := netlink.LinkSetMTU(link, mtu); serr != nil {
				t.Fatalf("set mtu %q: %v", name, serr)
			}
		}
		if uerr := netlink.LinkSetUp(link); uerr != nil {
			t.Fatalf("up %q: %v", name, uerr)
		}
	}

	// OpenCircuit now resolves the interface through the iface resolver, which
	// needs the netlink backend loaded (as it always is in production). Load it
	// inside the netns so iface.Resolve finds the veths created above.
	if err := iface.LoadBackend("netlink"); err != nil {
		t.Fatalf("load iface backend: %v", err)
	}

	fn()
}

func TestISISTransportRawSocketCap(t *testing.T) {
	// VALIDATES: A-5 -- raw-socket open succeeds under CAP_NET_RAW on a real veth.
	withVethPair(t, 1500, 1500, func() {
		be := NewBackend()
		h, err := be.OpenCircuit(vethA)
		if err != nil {
			if strings.Contains(err.Error(), "CAP_NET_RAW") {
				t.Skipf("requires CAP_NET_RAW: %v", err)
			}
			t.Fatalf("OpenCircuit(%s): %v", vethA, err)
		}
		if cerr := h.Close(); cerr != nil {
			t.Errorf("Close: %v", cerr)
		}
	})
}

func TestISISTransportVethRoundTrip(t *testing.T) {
	// VALIDATES: AC-1 / A-1 / A-2 -- a frame sent to the level multicast MAC on
	// one veth end is received on the peer with the correct source ifindex, and
	// the PDU arrives byte-for-byte (no padding/alteration by the transport).
	withVethPair(t, 1500, 1500, func() {
		be := NewBackend()

		sender, err := be.OpenCircuit(vethA)
		if err != nil {
			if strings.Contains(err.Error(), "CAP_NET_RAW") {
				t.Skipf("requires CAP_NET_RAW: %v", err)
			}
			t.Fatalf("open sender: %v", err)
		}
		t.Cleanup(func() { _ = sender.Close() })

		receiver, err := be.OpenCircuit(vethB)
		if err != nil {
			t.Fatalf("open receiver: %v", err)
		}
		t.Cleanup(func() { _ = receiver.Close() })

		pdu := []byte{0x83, 0x1b, 0x01, 0x00, 0x11, 0x01, 0x00, 0x00, 0x05, 0xd9}
		if serr := sender.Send(AllL2ISs, sender.HWAddr(), pdu); serr != nil {
			t.Fatalf("send: %v", serr)
		}

		select {
		case rf := <-receiver.Recv():
			if rf.IfIndex != receiver.IfIndex() {
				t.Errorf("ifindex = %d, want %d", rf.IfIndex, receiver.IfIndex())
			}
			if rf.DstMAC != AllL2ISs {
				t.Errorf("dst = %x, want AllL2ISs", rf.DstMAC)
			}
			if !bytes.Equal(rf.PDU, pdu) {
				t.Errorf("PDU = %x, want %x (transport must not pad/alter)", rf.PDU, pdu)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("no frame received on the peer (A-2 multicast membership?)")
		}
	})
}

func TestISISTransportConcurrentSendNoTear(t *testing.T) {
	// VALIDATES: B3 transport race -- the engine fans Hello, flood, and DIS/SNP
	// sends concurrently onto the SAME circuit (transport.SendPDU releases its
	// orchestrator lock before CircuitHandle.Send). linuxCircuit.Send frames into
	// a shared sendBuf then Sendto's it; without serialization two goroutines
	// interleave BuildFrame+Sendto and transmit a torn frame. This test fires many
	// concurrent sends of two DISTINCT PDUs on one circuit and asserts that every
	// frame received on the peer is one of those PDUs byte-for-byte (a torn frame
	// would be a splice of both, or fail ParseFrame). Run under -race
	// (make ze-integration-test) the unsynchronized sendBuf access is also flagged.
	withVethPair(t, 1500, 1500, func() {
		be := NewBackend()

		sender, err := be.OpenCircuit(vethA)
		if err != nil {
			// test-relax: missing CAP_NET_RAW means no raw socket on this host;
			// skip mirrors the sibling veth tests (dev laptops lack the cap, the
			// real race coverage comes from make ze-integration-test on a host
			// with CAP_NET_ADMIN + CAP_NET_RAW where -race is enabled).
			if strings.Contains(err.Error(), "CAP_NET_RAW") {
				t.Skipf("requires CAP_NET_RAW: %v", err)
			}
			t.Fatalf("open sender: %v", err)
		}
		t.Cleanup(func() { _ = sender.Close() })

		receiver, err := be.OpenCircuit(vethB)
		if err != nil {
			t.Fatalf("open receiver: %v", err)
		}
		t.Cleanup(func() { _ = receiver.Close() })

		// Two distinct, distinguishable PDUs of different lengths: a short
		// "Hello-style" PDU and a longer "flood-style" PDU. A torn frame would mix
		// their bytes, so an exact match against one or the other proves no tear.
		helloPDU := bytes.Repeat([]byte{0xA1}, 40)
		floodPDU := bytes.Repeat([]byte{0xB2}, 600)
		helloPDU[0], floodPDU[0] = 0x83, 0x83 // plausible IS-IS leading octet

		const iterations = 200
		var wg sync.WaitGroup
		wg.Add(2)
		// Hello-style sender (to AllL2ISs).
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				if serr := sender.Send(AllL2ISs, sender.HWAddr(), helloPDU); serr != nil {
					t.Errorf("hello send: %v", serr)
					return
				}
			}
		}()
		// Flood-style sender (also to AllL2ISs, same circuit, concurrent).
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				if serr := sender.Send(AllL2ISs, sender.HWAddr(), floodPDU); serr != nil {
					t.Errorf("flood send: %v", serr)
					return
				}
			}
		}()

		// Drain received frames; each must be exactly one of the two PDUs. Raw
		// sockets may drop under load, so we do not require all 2*iterations, only
		// that what does arrive is never torn. Stop once the senders finish and the
		// receive channel goes quiet.
		done := make(chan struct{})
		go func() { wg.Wait(); close(done) }()

		recv := receiver.Recv()
		idle := time.NewTimer(2 * time.Second)
		defer idle.Stop()
		var received int
		sendersDone := false
	loop:
		for {
			select {
			case rf := <-recv:
				if !bytes.Equal(rf.PDU, helloPDU) && !bytes.Equal(rf.PDU, floodPDU) {
					t.Fatalf("torn frame: received PDU (len %d) matches neither helloPDU nor floodPDU: %x", len(rf.PDU), rf.PDU)
				}
				received++
				if !idle.Stop() {
					<-idle.C
				}
				idle.Reset(500 * time.Millisecond)
			case <-done:
				sendersDone = true
				done = nil // disable this case; keep draining until idle
			case <-idle.C:
				if sendersDone {
					break loop
				}
				// Senders still running but no frames for a while; keep waiting up
				// to the overall idle window.
				idle.Reset(500 * time.Millisecond)
			}
		}
		if received == 0 {
			t.Fatal("no frames received on the peer (A-2 multicast membership?)")
		}
		t.Logf("received %d untorn frames from %d concurrent sends", received, 2*iterations)
	})
}

func TestISISTransportMTUExpose(t *testing.T) {
	// VALIDATES: A-4 -- the transport exposes the ioctl MTU; a smaller peer frame
	// surfaces an inferred neighbor MTU and a mismatch (ISO/IEC 10589 sec 8.2.3).
	withVethPair(t, 1500, 1500, func() {
		tr := New(NewBackend())
		tr.EnableInterface(vethA, Level2)
		if err := tr.HandleLinkUp(vethA); err != nil {
			if strings.Contains(err.Error(), "CAP_NET_RAW") {
				t.Skipf("requires CAP_NET_RAW: %v", err)
			}
			t.Fatalf("HandleLinkUp: %v", err)
		}
		t.Cleanup(tr.Close)

		mtu, ok := tr.InterfaceMTU(vethA)
		if !ok {
			t.Fatal("MTU not exposed for open circuit")
		}
		if mtu != 1500 {
			t.Errorf("exposed MTU = %d, want 1500 (ioctl value)", mtu)
		}

		var fired bool
		var gotLocal, gotNeighbor int
		tr.OnMTUMismatch(func(_ string, localMTU, neighborMTU int) {
			fired = true
			gotLocal, gotNeighbor = localMTU, neighborMTU
		})
		// Simulate a neighbor that padded its Hello to a 1492-byte link.
		tr.ObserveNeighborFrame(vethA, FrameHeaderLen+(1492-LLCHeaderLen))
		if !fired {
			t.Fatal("MTU mismatch not surfaced")
		}
		if gotLocal != 1500 || gotNeighbor != 1492 {
			t.Errorf("mismatch local=%d neighbor=%d, want 1500/1492", gotLocal, gotNeighbor)
		}
	})
}
