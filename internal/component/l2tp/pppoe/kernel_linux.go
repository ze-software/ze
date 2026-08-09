// Design: docs/architecture/l2tp/bng-5-pppoe.md -- kernel integration (AF_PPPOX, AF_PACKET)

//go:build linux

package pppoe

import (
	"errors"
	"fmt"
	"time"

	"golang.org/x/sys/unix"
)

var errSocketClosed = errors.New("pppoe: socket closed")

const (
	afPPPOX   = 24
	pxProtoOE = 0
)

// openDiscoverySocket creates a shared AF_PACKET/SOCK_RAW socket for
// PPPoE discovery frames (ethertype 0x8863). One socket handles all
// access interfaces; dispatch is by ifindex from recvfrom.
func openDiscoverySocket() (int, error) {
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(htons(unix.ETH_P_PPP_DISC)))
	if err != nil {
		return -1, fmt.Errorf("pppoe: socket(AF_PACKET): %w", err)
	}

	if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_BROADCAST, 1); err != nil {
		unix.Close(fd) //nolint:errcheck // rollback
		return -1, fmt.Errorf("pppoe: setsockopt SO_BROADCAST: %w", err)
	}

	sa := &unix.SockaddrLinklayer{
		Protocol: htons(unix.ETH_P_PPP_DISC),
	}
	if err := unix.Bind(fd, sa); err != nil {
		unix.Close(fd) //nolint:errcheck // rollback
		return -1, fmt.Errorf("pppoe: bind(AF_PACKET): %w", err)
	}

	return fd, nil
}

func closeDiscoverySocket(fd int) {
	if fd >= 0 {
		unix.Close(fd) //nolint:errcheck // shutdown
	}
}

// readDiscoveryFrame reads a single discovery frame from the socket.
// Returns the number of bytes read and the interface index.
func readDiscoveryFrame(fd int, buf []byte) (int, int, error) {
	n, from, err := unix.Recvfrom(fd, buf, 0)
	if err != nil {
		if errors.Is(err, unix.EBADF) || errors.Is(err, unix.EINVAL) {
			return 0, 0, errSocketClosed
		}
		return 0, 0, err
	}
	sll, ok := from.(*unix.SockaddrLinklayer)
	if !ok {
		return n, 0, nil
	}
	return n, sll.Ifindex, nil
}

// sendDiscoveryFrame sends a raw discovery frame on the specified interface.
func sendDiscoveryFrame(fd, ifindex int, frame []byte) error {
	if len(frame) < EthHdrLen {
		return fmt.Errorf("pppoe: frame too short to send (%d bytes)", len(frame))
	}

	var dstMAC [8]byte
	copy(dstMAC[:], frame[0:EthALen])

	sa := &unix.SockaddrLinklayer{
		Protocol: htons(unix.ETH_P_PPP_DISC),
		Ifindex:  ifindex,
		Halen:    EthALen,
		Addr:     dstMAC,
	}
	return unix.Sendto(fd, frame, 0, sa)
}

// pppoeCreate creates a PPPoE session socket (AF_PPPOX + PX_PROTO_OE)
// and connects it to the subscriber.
// RFC 2516 Section 4: after PADS, the kernel handles session framing.
func pppoeCreate(ifname string, sid uint16, remoteMAC [EthALen]byte) (int, error) {
	fd, err := unix.Socket(afPPPOX, unix.SOCK_DGRAM, pxProtoOE)
	if err != nil {
		return -1, fmt.Errorf("pppoe: socket(AF_PPPOX): %w", err)
	}

	sa := &unix.SockaddrPPPoE{
		SID:    sid,
		Remote: remoteMAC[:],
		Dev:    ifname,
	}
	if err := unix.Connect(fd, sa); err != nil {
		unix.Close(fd) //nolint:errcheck // rollback
		return -1, fmt.Errorf("pppoe: connect(PPPoE sid=%d dev=%s): %w", sid, ifname, err)
	}
	return fd, nil
}

func closePPPoxFD(fd int) {
	if fd >= 0 {
		unix.Close(fd) //nolint:errcheck // shutdown
	}
}

func htons(v uint16) uint16 {
	return (v<<8)&0xff00 | (v>>8)&0x00ff
}

// Exported wrappers for cross-package use (PPPoE client in iface).

// OpenDiscoverySocket creates a shared AF_PACKET socket for PPPoE discovery.
func OpenDiscoverySocket() (int, error) { return openDiscoverySocket() }

// CloseDiscoveryFD closes a discovery socket file descriptor.
func CloseDiscoveryFD(fd int) { closeDiscoverySocket(fd) }

// ReadDiscoveryFrame reads a single discovery frame from the socket.
func ReadDiscoveryFrame(fd int, buf []byte) (int, int, error) { return readDiscoveryFrame(fd, buf) }

// SendDiscoveryFrame sends a raw discovery frame on the specified interface.
func SendDiscoveryFrame(fd, ifindex int, frame []byte) error {
	return sendDiscoveryFrame(fd, ifindex, frame)
}

// PPPoECreate creates a PPPoE session socket.
func PPPoECreate(ifname string, sid uint16, remoteMAC [EthALen]byte) (int, error) {
	return pppoeCreate(ifname, sid, remoteMAC)
}

// ClosePPPoxFD closes a PPPoX socket file descriptor.
func ClosePPPoxFD(fd int) { closePPPoxFD(fd) }

// SetRecvTimeout sets SO_RCVTIMEO on a socket so blocking reads return
// periodically, allowing the caller to check stop signals.
func SetRecvTimeout(fd int, d time.Duration) error {
	sec := int64(d / time.Second)
	usec := int64((d % time.Second) / time.Microsecond)
	tv := unix.Timeval{Sec: sec, Usec: usec}
	return unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv)
}
