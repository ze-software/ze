// Design: plan/learned/669-bng-5-pppoe.md -- kernel integration (AF_PPPOX, AF_PACKET)

//go:build linux

package pppoe

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
	"unsafe"

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

// resolveInterface looks up an interface by name and returns its
// index, hardware address, and MTU.
func resolveInterface(name string) (ifindex int, hwaddr [EthALen]byte, mtu int, err error) {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		return 0, hwaddr, 0, fmt.Errorf("pppoe: socket for ioctl(%s): %w", name, err)
	}
	defer unix.Close(fd) //nolint:errcheck // helper socket

	ifr, err := unix.NewIfreq(name)
	if err != nil {
		return 0, hwaddr, 0, fmt.Errorf("pppoe: ifreq(%s): %w", name, err)
	}

	if err := unix.IoctlIfreq(fd, unix.SIOCGIFINDEX, ifr); err != nil {
		return 0, hwaddr, 0, fmt.Errorf("pppoe: SIOCGIFINDEX(%s): %w", name, err)
	}
	ifindex = int(ifr.Uint32())

	if err := unix.IoctlIfreq(fd, unix.SIOCGIFHWADDR, ifr); err != nil {
		return 0, hwaddr, 0, fmt.Errorf("pppoe: SIOCGIFHWADDR(%s): %w", name, err)
	}
	// SIOCGIFHWADDR returns a sockaddr{sa_family(2), sa_data(14)}.
	// Ifreq has no hwaddr accessor (upstream TODO). Read the raw union:
	// skip IFNAMSIZ (16) name to reach ifru, then 2 for sa_family.
	ifrUnion := (*[24]byte)(unsafe.Add(unsafe.Pointer(ifr), unix.IFNAMSIZ)) //nolint:gosec // ioctl data layout
	copy(hwaddr[:], ifrUnion[2:2+EthALen])

	if err := unix.IoctlIfreq(fd, unix.SIOCGIFMTU, ifr); err != nil {
		return 0, hwaddr, 0, fmt.Errorf("pppoe: SIOCGIFMTU(%s): %w", name, err)
	}
	mtu = int(ifr.Uint32())

	return ifindex, hwaddr, mtu, nil
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

// probeKernelPPPoE checks that the kernel PPPoE module is loaded.
func probeKernelPPPoE() error {
	fd, err := unix.Socket(afPPPOX, unix.SOCK_STREAM, pxProtoOE)
	if err == nil {
		unix.Close(fd) //nolint:errcheck // probe
		return nil
	}
	if moduleBuiltIn("pppoe") {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, "modprobe", "pppoe").Run(); err != nil { //nolint:gosec // constant arg
		return fmt.Errorf("pppoe: failed to load kernel module: %w", err)
	}
	return nil
}

func moduleBuiltIn(name string) bool {
	_, err := os.Stat("/sys/module/" + name)
	return err == nil
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

// ResolveInterface looks up an interface by name.
func ResolveInterface(name string) (int, [EthALen]byte, int, error) { return resolveInterface(name) }

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
