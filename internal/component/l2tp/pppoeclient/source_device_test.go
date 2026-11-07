// Design: docs/architecture/iface/logical-name-resolution.md -- consumers resolve through iface
// Related: dialer.go -- Dial, the one resolution every kernel call takes its NIC from
//
// Goal: prove Dial connects the PPPoE session socket to the kernel device its
// source-interface resolved to, the same device discovery ran on, and never
// to the configured (logical) name.
// Method: replace the dialer's kernel seams. The resolver maps logical "wan"
// to kernel device "eth7" at ifindex 9 (what `ethernet wan { os-name eth7 }`
// or a mac match selector gives). A scripted AC answers PADI with PADO and
// PADR with PADS on ifindex 9. The session socket seam records the binding it
// receives and refuses, so Dial stops before /dev/ppp.

package pppoeclient

import (
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
)

var errInjectedSessionSocket = errors.New("pppoeclient test: session socket refused")

// TestDialSessionSocketUsesResolvedDevice fails if the configured name rather
// than the resolved device reaches PPPoECreate.
func TestDialSessionSocketUsesResolvedDevice(t *testing.T) {
	const deviceIfindex = 9
	binding := pppoe.InterfaceBinding{Device: "eth7", Ifindex: deviceIfindex, HWAddr: testHostMAC, MTU: 1500}

	restoreResolve, restoreOpen, restoreTimeout := resolveSourceInterface, openDiscoverySocket, setDiscoveryTimeout
	restoreCreate, restoreSend, restoreRead := createSessionSocket, sendDiscoveryFrame, readDiscoveryFrame
	t.Cleanup(func() {
		resolveSourceInterface, openDiscoverySocket, setDiscoveryTimeout = restoreResolve, restoreOpen, restoreTimeout
		createSessionSocket, sendDiscoveryFrame, readDiscoveryFrame = restoreCreate, restoreSend, restoreRead
	})

	resolveSourceInterface = func(name string) (pppoe.InterfaceBinding, error) {
		if name != "wan" {
			return pppoe.InterfaceBinding{}, errors.New("no such interface")
		}
		return binding, nil
	}
	// A negative descriptor: CloseDiscoveryFD ignores it, so no real fd is closed.
	openDiscoverySocket = func() (int, error) { return -1, nil }
	setDiscoveryTimeout = func(int, time.Duration) error { return nil }

	// The scripted AC: Dial runs on one goroutine, so the queue needs no lock.
	var queue [][]byte
	var sentIfindexes []int
	sendDiscoveryFrame = func(_ int, ifindex int, frame []byte) error {
		sentIfindexes = append(sentIfindexes, ifindex)
		pkt, err := pppoe.ParseDiscovery(frame)
		if err != nil {
			return err
		}
		var buf [pppoe.EthMaxLen]byte
		var reply []byte
		switch pkt.Code {
		case pppoe.CodePADI:
			reply = pppoe.BuildPADO(buf[:], testACMAC, &pkt, "ze", nil, []byte("cookie"))
		case pppoe.CodePADR:
			reply = pppoe.BuildPADS(buf[:], testACMAC, &pkt, "ze", 0x002a)
		}
		if reply != nil {
			queue = append(queue, append([]byte(nil), reply...))
		}
		return nil
	}
	readDiscoveryFrame = func(_ int, buf []byte) (int, int, error) {
		if len(queue) == 0 {
			return 0, 0, errFakeNoDiscoveryFrame
		}
		frame := queue[0]
		queue = queue[1:]
		return copy(buf, frame), deviceIfindex, nil
	}

	var created []pppoe.InterfaceBinding
	createSessionSocket = func(source pppoe.InterfaceBinding, sid uint16, _ [pppoe.EthALen]byte) (int, error) {
		created = append(created, source)
		if sid != 0x002a {
			t.Errorf("session socket SESSION_ID = 0x%04x, want 0x002a", sid)
		}
		return -1, errInjectedSessionSocket
	}

	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	_, err := (&Dialer{}).Dial(iface.PPPoEClientConfig{SourceInterface: "wan", Timing: testDialTiming}, stop, slog.Default())
	if !errors.Is(err, errInjectedSessionSocket) {
		t.Fatalf("Dial error = %v, want the injected session socket refusal", err)
	}
	if len(created) != 1 {
		t.Fatalf("session socket created %d times, want 1", len(created))
	}
	if created[0].Device != "eth7" {
		t.Fatalf("session socket device = %q, want the resolved kernel device %q (configured name %q)", created[0].Device, "eth7", "wan")
	}
	if created[0].Ifindex != deviceIfindex {
		t.Fatalf("session socket binding ifindex = %d, want %d: discovery and session must share one resolution", created[0].Ifindex, deviceIfindex)
	}
	// PADI, PADR, and the PADT that answers the refused session socket.
	if len(sentIfindexes) != 3 {
		t.Fatalf("discovery sent %d frames, want PADI, PADR and PADT", len(sentIfindexes))
	}
	for i, ifindex := range sentIfindexes {
		if ifindex != deviceIfindex {
			t.Fatalf("discovery frame %d left on ifindex %d, want %d", i, ifindex, deviceIfindex)
		}
	}
}
