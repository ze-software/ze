//go:build integration && linux

// Design: docs/features/interfaces.md AC-6 -- prove the tc mirror copies
// traffic at the PACKET level, not just that the qdisc/filter installs. A frame
// injected on a veth peer ingresses the mirror source and must appear, copied,
// on the mirror destination captured via AF_PACKET. Requires CAP_NET_ADMIN
// (skipped otherwise). Run under `./le integration iface`.

package iface

import (
	"bytes"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func htonsPkt(v uint16) uint16 { return v<<8 | v>>8 } //nolint:unparam // generic htons; only ETH_P_ALL is passed today

func ifIndexFor(t *testing.T, name string) int {
	t.Helper()
	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("LinkByName %s: %v", name, err)
	}
	return link.Attrs().Index
}

func TestIntegrationMirrorPacketLevel(t *testing.T) {
	withNetNS(t, func() {
		// veth: a frame sent on msrcp ingresses msrc0; mdst0 is the mirror sink.
		createVethForTest(t, "msrc0", "msrcp")
		createDummyForTest(t, "mdst0")
		for _, n := range []string{"msrc0", "msrcp", "mdst0"} {
			if err := SetAdminUp(n); err != nil {
				t.Fatalf("SetAdminUp %s: %v", n, err)
			}
		}
		if err := SetupMirror("msrc0", "mdst0", true, false); err != nil {
			t.Fatalf("SetupMirror(ingress): %v", err)
		}
		t.Cleanup(func() { _ = RemoveMirror("msrc0") })

		dstIdx := ifIndexFor(t, "mdst0")
		srcpIdx := ifIndexFor(t, "msrcp")

		// Capture socket on the mirror destination.
		capFd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(htonsPkt(unix.ETH_P_ALL)))
		if err != nil {
			t.Fatalf("capture socket: %v", err)
		}
		defer func() { _ = unix.Close(capFd) }()
		if err := unix.Bind(capFd, &unix.SockaddrLinklayer{Protocol: htonsPkt(unix.ETH_P_ALL), Ifindex: dstIdx}); err != nil {
			t.Fatalf("bind capture on mdst0: %v", err)
		}
		tv := unix.Timeval{Sec: 1}
		if err := unix.SetsockoptTimeval(capFd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv); err != nil {
			t.Fatalf("set recv timeout: %v", err)
		}

		// A uniquely identifiable Ethernet frame (local-experimental ethertype).
		magic := []byte("ZE-MIRROR-PKT-AC6")
		frame := append([]byte{
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, // dst broadcast
			0x02, 0x00, 0x00, 0x00, 0x00, 0x01, // src
			0x88, 0xb5, // ethertype (IEEE local experimental)
		}, magic...)

		sendFd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(htonsPkt(unix.ETH_P_ALL)))
		if err != nil {
			t.Fatalf("send socket: %v", err)
		}
		defer func() { _ = unix.Close(sendFd) }()
		sendAddr := &unix.SockaddrLinklayer{
			Protocol: htonsPkt(unix.ETH_P_ALL),
			Ifindex:  srcpIdx,
			Halen:    6,
			Addr:     [8]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		}
		// Inject several copies to absorb any startup race on the tc datapath.
		for range 10 {
			if err := unix.Sendto(sendFd, frame, 0, sendAddr); err != nil {
				t.Fatalf("sendto msrcp: %v", err)
			}
		}

		buf := make([]byte, 2048)
		deadline := time.Now().Add(4 * time.Second)
		for time.Now().Before(deadline) {
			n, _, rerr := unix.Recvfrom(capFd, buf, 0)
			if rerr != nil {
				// Timeout: re-inject and keep looking until the deadline.
				_ = unix.Sendto(sendFd, frame, 0, sendAddr)
				continue
			}
			if bytes.Contains(buf[:n], magic) {
				return // mirrored frame observed on the destination
			}
		}
		t.Fatal("mirrored packet with magic payload not observed on mdst0 (ingress mirror datapath)")
	})
}
