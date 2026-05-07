// Design: plan/spec-bng-5-pppoe.md -- kernel integration (AF_PPPOX, AF_PACKET)

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

const (
	pppiocGChan   = 0x80047437
	pppiocAttChan = 0x40047438
	pppiocNewUnit = 0xc004743e
	pppiocConnect = 0x4004743a
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

// devPPPSetup opens /dev/ppp, attaches the PPP channel from the PPPoX
// socket, allocates a PPP unit, and creates the pppN interface.
func devPPPSetup(pppoxFD int) (chanFD, unitFD, unitNum int, err error) {
	chanIdx, err := ioctlGetInt(pppoxFD, pppiocGChan)
	if err != nil {
		return -1, -1, -1, fmt.Errorf("pppoe: PPPIOCGCHAN: %w", err)
	}

	chanFD, err = openDevPPP()
	if err != nil {
		return -1, -1, -1, err
	}
	if err := ioctlSetInt(chanFD, pppiocAttChan, chanIdx); err != nil {
		unix.Close(chanFD) //nolint:errcheck // rollback
		return -1, -1, -1, fmt.Errorf("pppoe: PPPIOCATTCHAN: %w", err)
	}

	unitFD, err = openDevPPP()
	if err != nil {
		unix.Close(chanFD) //nolint:errcheck // rollback
		return -1, -1, -1, err
	}
	unitNum = -1
	unitNum, err = ioctlGetSetInt(unitFD, pppiocNewUnit, unitNum)
	if err != nil {
		unix.Close(unitFD) //nolint:errcheck // rollback
		unix.Close(chanFD) //nolint:errcheck // rollback
		return -1, -1, -1, fmt.Errorf("pppoe: PPPIOCNEWUNIT: %w", err)
	}

	return chanFD, unitFD, unitNum, nil
}

func pppConnect(chanFD, unitNum int) error {
	if err := ioctlSetInt(chanFD, pppiocConnect, unitNum); err != nil {
		return fmt.Errorf("pppoe: PPPIOCCONNECT: %w", err)
	}
	return nil
}

func openDevPPP() (int, error) {
	f, err := os.OpenFile("/dev/ppp", os.O_RDWR, 0)
	if err != nil {
		return -1, fmt.Errorf("pppoe: open /dev/ppp: %w", err)
	}
	rawFD, err := dupFD(f)
	f.Close() //nolint:errcheck // source replaced by rawFD
	if err != nil {
		return -1, fmt.Errorf("pppoe: dup /dev/ppp fd: %w", err)
	}
	return rawFD, nil
}

func dupFD(f *os.File) (int, error) {
	raw, err := f.SyscallConn()
	if err != nil {
		return -1, err
	}
	var fd int
	var opErr error
	if err := raw.Control(func(fdp uintptr) {
		fd, opErr = unix.Dup(int(fdp))
	}); err != nil {
		return -1, err
	}
	return fd, opErr
}

func ioctlGetInt(fd int, req uint) (int, error) {
	var val int32
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&val))) //nolint:gosec // ioctl pointer arg
	if errno != 0 {
		return 0, errno
	}
	return int(val), nil
}

func ioctlSetInt(fd int, req uint, val int) error {
	v := int32(val)
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&v))) //nolint:gosec // ioctl pointer arg
	if errno != 0 {
		return errno
	}
	return nil
}

func ioctlGetSetInt(fd int, req uint, val int) (int, error) {
	v := int32(val)
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&v))) //nolint:gosec // ioctl pointer arg
	if errno != 0 {
		return 0, errno
	}
	return int(v), nil
}

func htons(v uint16) uint16 {
	return (v<<8)&0xff00 | (v>>8)&0x00ff
}
