// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- priority-decrement tracking
// RFC: rfc/short/rfc9568.md (VRRPv3) -- Section 5.2.4 priority, Section 8.3.2 priority spacing
// Related: routing_fixture_linux.go -- registers both fixtures below, beside the other VRRP ones
//
// The fixtures behind test/vrrp/vrrp-track.ci. They prove against a real kernel
// what the unit tests prove against the model: an operator who configures
// `track interface <name> priority-decrement <n>` sees the advertised priority
// fall by n while that interface is down, and rise again when it returns.
//
// The observation is the PRIORITY BYTE of the advertisement, captured off the
// parent's veth peer with an AF_PACKET socket. That is what a neighboring
// router reads and elects on, so it is the only proof that the decrement
// reached the wire rather than only the config. Reading it back from `show
// vrrp` would prove what Ze believes about itself.

//go:build linux

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// vrrpTrackParent is the throwaway veth this group advertises from, so the
	// capture sees this group's frames and nothing else.
	vrrpTrackParent     = "zeptrk0"
	vrrpTrackParentPeer = "zeptrk0p"
	vrrpTrackRealCIDR   = "203.0.113.251/24"
	// vrrpTracked is the interface the group tracks: a second veth with no
	// address and no relationship to the group's family, because Ze reads its
	// OPERATIONAL STATE and nothing else.
	vrrpTracked     = "zetrk1"
	vrrpTrackedPeer = "zetrk1p"

	// vrrpTrackPriority, vrrpTrackDecrement and vrrpTrackLowered are the
	// arithmetic under test: 200 while the tracked interface is up, 50 while it
	// is down. The gap is wide on purpose, so a peer at any priority between
	// them would take over (RFC 9568 Section 8.3.2).
	vrrpTrackPriority  = 200
	vrrpTrackDecrement = 150
	vrrpTrackLowered   = vrrpTrackPriority - vrrpTrackDecrement

	// vrrpTrackCaptureWait bounds the wait for an advertisement at a given
	// priority. Several advertisement intervals, so a lost frame costs a retry
	// rather than the test.
	vrrpTrackCaptureWait = 20 * time.Second

	// The wire offsets the capture reads. An Ethernet header, then an IPv4
	// header, then the VRRP header. RFC 9568 Section 5.2 lays that header out
	// as "|Version| Type  | Virtual Rtr ID|   Priority    |IPvX Addr Count|",
	// so Priority is the THIRD octet and its offset is 2. Offset 1 is the
	// Virtual Router ID, and reading it here returned this group's vrid of 12
	// for every advertisement, whatever the priority on the wire was.
	vrrpTrackEthHeaderLen = 14
	vrrpTrackEthTypeIPv4  = 0x0800
	vrrpTrackProtoVRRP    = 112
	vrrpTrackPriorityByte = 2
)

// vrrpTrackSetup builds the parent veth the group advertises from and the veth
// it tracks. Both are up, so the group starts undecremented.
func vrrpTrackSetup(context.Context, []string) error {
	if err := vrrpTrackVeth(vrrpTrackParent, vrrpTrackParentPeer, vrrpTrackRealCIDR); err != nil {
		return err
	}
	return vrrpTrackVeth(vrrpTracked, vrrpTrackedPeer, "")
}

// vrrpTrackVeth creates one veth pair, brings both ends up, and gives the near
// end an address when one is asked for.
func vrrpTrackVeth(name, peer, cidr string) error {
	if err := addRoutingLink(&netlink.Veth{Name: name, PeerName: peer}); err != nil {
		return fmt.Errorf("add %s veth: %w", name, err)
	}
	near, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("find %s: %w", name, err)
	}
	if cidr != "" {
		if err := addRoutingAddress(near, cidr); err != nil {
			return fmt.Errorf("address %s: %w", name, err)
		}
	}
	if err := netlink.LinkSetUp(near); err != nil {
		return fmt.Errorf("bring %s up: %w", name, err)
	}
	far, err := netlink.LinkByName(peer)
	if err != nil {
		return fmt.Errorf("find %s: %w", peer, err)
	}
	if err := netlink.LinkSetUp(far); err != nil {
		return fmt.Errorf("bring %s up: %w", peer, err)
	}
	return nil
}

// vrrpTrackSetLink brings the tracked interface up or down, which is the event
// the whole feature reacts to.
func vrrpTrackSetLink(up bool) error {
	link, err := netlink.LinkByName(vrrpTracked)
	if err != nil {
		return fmt.Errorf("find %s: %w", vrrpTracked, err)
	}
	if up {
		if err := netlink.LinkSetUp(link); err != nil {
			return fmt.Errorf("bring %s up: %w", vrrpTracked, err)
		}
		return nil
	}
	if err := netlink.LinkSetDown(link); err != nil {
		return fmt.Errorf("take %s down: %w", vrrpTracked, err)
	}
	return nil
}

// vrrpTrackCapture opens an AF_PACKET capture bound to the parent's veth peer,
// which is where this group's advertisements arrive. The receive timeout keeps
// the read loop bounded, so a router that stops advertising fails on the wait
// rather than blocking forever.
func vrrpTrackCapture() (int, error) {
	peer, err := netlink.LinkByName(vrrpTrackParentPeer)
	if err != nil {
		return -1, fmt.Errorf("find %s: %w", vrrpTrackParentPeer, err)
	}
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(vrrpTrackHtons(unix.ETH_P_ALL)))
	if err != nil {
		return -1, fmt.Errorf("open capture socket: %w", err)
	}
	address := &unix.SockaddrLinklayer{
		Protocol: vrrpTrackHtons(unix.ETH_P_ALL),
		Ifindex:  peer.Attrs().Index,
	}
	if err := unix.Bind(fd, address); err != nil {
		unix.Close(fd) //nolint:errcheck // the bind already failed; the close is best effort
		return -1, fmt.Errorf("bind capture to %s: %w", vrrpTrackParentPeer, err)
	}
	timeout := unix.Timeval{Sec: 0, Usec: 200000}
	if err := unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &timeout); err != nil {
		unix.Close(fd) //nolint:errcheck // the socket is being abandoned
		return -1, fmt.Errorf("set capture timeout: %w", err)
	}
	return fd, nil
}

// vrrpTrackHtons converts a port-order constant to network order, which is what
// AF_PACKET wants in both the socket protocol and the bind address.
func vrrpTrackHtons(v uint16) uint16 {
	return v<<8 | v>>8
}

// vrrpTrackAdvertPriority reads the priority byte out of one captured Ethernet
// frame, and reports whether the frame was a VRRP advertisement at all.
func vrrpTrackAdvertPriority(frame []byte) (uint8, bool) {
	if len(frame) < vrrpTrackEthHeaderLen+1 {
		return 0, false
	}
	if uint16(frame[12])<<8|uint16(frame[13]) != vrrpTrackEthTypeIPv4 {
		return 0, false
	}
	ip := frame[vrrpTrackEthHeaderLen:]
	// The low nibble of the first byte is the IPv4 header length in 32-bit
	// words, so a router with options still lands on the right payload offset.
	headerLen := int(ip[0]&0x0f) * 4
	if headerLen < 20 || len(ip) < headerLen+vrrpTrackPriorityByte+1 {
		return 0, false
	}
	if ip[9] != vrrpTrackProtoVRRP {
		return 0, false
	}
	return ip[headerLen+vrrpTrackPriorityByte], true
}

// vrrpTrackWaitPriority reads advertisements until one carries want, and
// reports the last priority it did see when the wait runs out. Reporting that
// last value is what turns "no advertisement matched" into a usable failure: a
// router advertising 200 when 50 was expected is a decrement that never
// happened, and a router advertising nothing is a router that stopped.
func vrrpTrackWaitPriority(fd int, want uint8) error {
	buffer := make([]byte, 2048)
	deadline := time.Now().Add(vrrpTrackCaptureWait)
	seen, sawAny := uint8(0), false
	for time.Now().Before(deadline) {
		n, _, err := unix.Recvfrom(fd, buffer, 0)
		if err != nil {
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EINTR) {
				continue
			}
			return fmt.Errorf("read the capture: %w", err)
		}
		priority, ok := vrrpTrackAdvertPriority(buffer[:n])
		if !ok {
			continue
		}
		if priority == want {
			return nil
		}
		seen, sawAny = priority, true
	}
	if !sawAny {
		return fmt.Errorf("no VRRP advertisement arrived on %s within %s, so priority %d was never observed",
			vrrpTrackParentPeer, vrrpTrackCaptureWait, want)
	}
	return fmt.Errorf("the advertised priority stayed at %d within %s, want %d", seen, vrrpTrackCaptureWait, want)
}

// vrrpTrackSay prints one marker line the .ci expectations match on.
func vrrpTrackSay(marker string, priority uint8, detail string) {
	var tb textbuf.Buffer
	tb.Str(marker).Byte(' ').Uint8(priority).Byte(' ').Str(detail).Byte('\n').StdOut() //nolint:errcheck // CLI output
}

// vrrpTrackDriver flaps the tracked interface and reads the advertised priority
// off the wire on each side of the change.
func vrrpTrackDriver(ctx context.Context, _ []string) error {
	pid, err := routingDaemonPID(ctx)
	if err != nil {
		return err
	}

	fd, err := vrrpTrackCapture()
	if err != nil {
		return err
	}
	defer unix.Close(fd) //nolint:errcheck // the driver owns this socket and exits straight after

	// The router is alone on the segment, so it promotes itself once its
	// master-down timer expires and advertises the configured priority.
	if err := vrrpTrackWaitPriority(fd, vrrpTrackPriority); err != nil {
		return fmt.Errorf("with %s up: %w", vrrpTracked, err)
	}
	vrrpTrackSay("ADVERT-PRIORITY", vrrpTrackPriority, "tracked-interface-up")

	if err := vrrpTrackSetLink(false); err != nil {
		return err
	}
	if err := vrrpTrackWaitPriority(fd, vrrpTrackLowered); err != nil {
		return fmt.Errorf("with %s down: %w", vrrpTracked, err)
	}
	vrrpTrackSay("ADVERT-PRIORITY", vrrpTrackLowered, "tracked-interface-down")

	if err := vrrpTrackSetLink(true); err != nil {
		return err
	}
	if err := vrrpTrackWaitPriority(fd, vrrpTrackPriority); err != nil {
		return fmt.Errorf("after %s returned: %w", vrrpTracked, err)
	}
	vrrpTrackSay("ADVERT-PRIORITY", vrrpTrackPriority, "tracked-interface-restored")

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find daemon: %w", err)
	}
	if err := process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("stop daemon: %w", err)
	}
	Poll(ctx, 100, 50*time.Millisecond, func() bool { return process.Signal(syscall.Signal(0)) != nil })
	return nil
}
