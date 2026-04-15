// Design: docs/research/l2tpv2-implementation-guide.md -- S21 PPPoL2TP socket API, /dev/ppp
// Related: kernel_event.go -- event types that trigger pppox operations

//go:build linux

package l2tp

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// PPPoL2TP socket constants.
// RFC 2661 Section 21: AF_PPPOX socket with PX_PROTO_OL2TP protocol.
const (
	afPPPOX      = 24 // AF_PPPOX address family
	pxProtoOL2TP = 1  // PX_PROTO_OL2TP protocol number
)

// SOL_PPPOL2TP is the socket option level for PPPoL2TP sockets.
// Section 24.24: this value is architecture-dependent. 273 is correct
// for x86_64 and arm64 on Linux 5.x+.
const solPPPOL2TP = 273

// PPPoL2TP socket options (set via setsockopt with solPPPOL2TP).
// Only options used by Phase 5 are defined.
const (
	pppol2tpSORecvSeq = 2 // Require sequence numbers on received data
	pppol2tpSOSendSeq = 3 // Include sequence numbers on sent data
	pppol2tpSOLNSMode = 4 // LNS mode (auto-enables send+recv seq)
)

// /dev/ppp ioctl numbers.
// These are architecture-dependent but stable on x86_64 and arm64.
const (
	pppiocGChan   = 0x800437b4 // PPPIOCGCHAN: get PPP channel index
	pppiocAttChan = 0x400437b8 // PPPIOCATTCHAN: attach channel to /dev/ppp fd
	pppiocNewUnit = 0xc004743e // PPPIOCNEWUNIT: allocate PPP unit (creates pppN)
	pppiocConnect = 0x4004743a // PPPIOCCONNECT: connect channel to unit
)

// sockaddrPPPoL2TP is the binary layout of struct sockaddr_pppol2tp
// for IPv4 L2TPv2 sessions. Used with connect() on AF_PPPOX sockets.
//
// The struct layout matches the kernel's sockaddr_pppol2tp:
//
//	sa_family  (2 bytes): AF_PPPOX
//	sa_protocol(4 bytes): PX_PROTO_OL2TP
//	pid        (4 bytes): 0 (current process)
//	fd         (4 bytes): tunnel UDP socket fd
//	addr       (16 bytes): sockaddr_in (peer address)
//	s_tunnel   (2 bytes): local tunnel ID
//	s_session  (2 bytes): local session ID
//	d_tunnel   (2 bytes): remote tunnel ID
//	d_session  (2 bytes): remote session ID
type sockaddrPPPoL2TP struct {
	Family   uint16
	Protocol uint32
	PID      int32
	FD       int32
	Addr     rawSockaddrInet4
	STunnel  uint16
	SSession uint16
	DTunnel  uint16
	DSession uint16
}

// rawSockaddrInet4 is the binary layout of struct sockaddr_in.
type rawSockaddrInet4 struct {
	Family uint16
	Port   uint16 // network byte order
	Addr   [4]byte
	Zero   [8]byte
}

// buildSockaddrPPPoL2TP constructs the binary representation of
// sockaddr_pppol2tp for connect(). The peer address must be IPv4.
func buildSockaddrPPPoL2TP(
	socketFD int,
	peerAddr netip.AddrPort,
	localTID, localSID, remoteTID, remoteSID uint16,
) ([]byte, error) {
	if !peerAddr.Addr().Is4() {
		return nil, fmt.Errorf("l2tp: pppol2tp sockaddr requires IPv4, got %s", peerAddr.Addr())
	}

	sa := sockaddrPPPoL2TP{
		Family:   afPPPOX,
		Protocol: pxProtoOL2TP,
		PID:      0,
		FD:       int32(socketFD),
		Addr: rawSockaddrInet4{
			Family: unix.AF_INET,
			Port:   htons(peerAddr.Port()),
			Addr:   peerAddr.Addr().As4(),
		},
		STunnel:  localTID,
		SSession: localSID,
		DTunnel:  remoteTID,
		DSession: remoteSID,
	}

	size := unsafe.Sizeof(sa)
	buf := make([]byte, size)
	// sockaddr_pppol2tp has no Go-friendly accessor; the kernel reads
	// the raw byte layout. #nosec G103 -- required for binary struct copy.
	copy(buf, (*[128]byte)(unsafe.Pointer(&sa))[:size]) //nolint:gosec // sockaddr binary layout
	return buf, nil
}

// htons converts a uint16 from host to network byte order.
func htons(v uint16) uint16 {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], v)
	// Reinterpret the network-order bytes as a uint16 value whose native
	// in-memory layout is the correct sockaddr_in::sin_port encoding.
	return *(*uint16)(unsafe.Pointer(&buf[0])) //nolint:gosec // sockaddr port layout
}

// pppoxCreate creates a PPPoL2TP socket and connects it to the kernel
// session identified by the tunnel and session IDs.
// RFC 2661 Section 24.21: the kernel session (L2TP_CMD_SESSION_CREATE)
// must exist before this connect() call.
func pppoxCreate(
	socketFD int,
	peerAddr netip.AddrPort,
	localTID, localSID, remoteTID, remoteSID uint16,
) (int, error) {
	fd, err := unix.Socket(afPPPOX, unix.SOCK_DGRAM, pxProtoOL2TP)
	if err != nil {
		return -1, fmt.Errorf("l2tp: pppox socket: %w", err)
	}

	sa, err := buildSockaddrPPPoL2TP(socketFD, peerAddr, localTID, localSID, remoteTID, remoteSID)
	if err != nil {
		unix.Close(fd) //nolint:errcheck // rollback path; primary error is err
		return -1, err
	}

	// connect() binds the PPPoL2TP socket to the kernel L2TP session.
	// The kernel reads &sa[0] as a sockaddr_pppol2tp; unsafe.Pointer is
	// the only way to pass an arbitrary-shape sockaddr to SYS_CONNECT.
	_, _, errno := unix.RawSyscall(
		unix.SYS_CONNECT,
		uintptr(fd),
		uintptr(unsafe.Pointer(&sa[0])), //nolint:gosec // sockaddr pointer for SYS_CONNECT
		uintptr(len(sa)),
	)
	if errno != 0 {
		unix.Close(fd) //nolint:errcheck // rollback path; primary error is errno
		return -1, fmt.Errorf("l2tp: pppox connect: %w", errno)
	}
	return fd, nil
}

// pppoxSetLNSMode sets the LNS mode socket option on a PPPoL2TP socket.
func pppoxSetLNSMode(fd int, enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	return unix.SetsockoptInt(fd, solPPPOL2TP, pppol2tpSOLNSMode, val)
}

// pppoxSetSendSeq sets the send-sequence socket option.
func pppoxSetSendSeq(fd int, enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	return unix.SetsockoptInt(fd, solPPPOL2TP, pppol2tpSOSendSeq, val)
}

// pppoxSetRecvSeq sets the recv-sequence socket option.
func pppoxSetRecvSeq(fd int, enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	return unix.SetsockoptInt(fd, solPPPOL2TP, pppol2tpSORecvSeq, val)
}

// pppSessionFDs holds the file descriptors created during PPP channel
// and unit setup via /dev/ppp. All four fds must be closed during
// teardown in reverse order (unitFD, chanFD, pppoxFD, then genl delete).
type pppSessionFDs struct {
	pppoxFD int // PPPoL2TP socket fd
	chanFD  int // /dev/ppp fd attached to channel
	unitFD  int // /dev/ppp fd with allocated PPP unit
	unitNum int // pppN unit number (the N in pppN)
}

// devPPPSetup opens /dev/ppp, attaches the PPP channel from the PPPoL2TP
// socket, allocates a PPP unit (creating the pppN interface), and connects
// the channel to the unit.
//
// RFC 2661 Section 21: the full sequence is:
//  1. PPPIOCGCHAN on pppoxFD -> channel index
//  2. open /dev/ppp, PPPIOCATTCHAN -> channel fd
//  3. open /dev/ppp, PPPIOCNEWUNIT -> unit fd + pppN interface
//  4. PPPIOCCONNECT on channel fd -> connects channel to unit
func devPPPSetup(pppoxFD int) (chanFD, unitFD, unitNum int, err error) {
	// Step 1: get the PPP channel index from the PPPoL2TP socket.
	chanIdx, err := ioctlGetInt(pppoxFD, pppiocGChan)
	if err != nil {
		return -1, -1, -1, fmt.Errorf("l2tp: PPPIOCGCHAN: %w", err)
	}

	// Step 2: open /dev/ppp and attach to the channel.
	chanFD, err = openDevPPP()
	if err != nil {
		return -1, -1, -1, err
	}
	if err := ioctlSetInt(chanFD, pppiocAttChan, chanIdx); err != nil {
		unix.Close(chanFD) //nolint:errcheck // rollback path; primary error is err
		return -1, -1, -1, fmt.Errorf("l2tp: PPPIOCATTCHAN: %w", err)
	}

	// Step 3: open /dev/ppp and allocate a PPP unit (creates pppN).
	unitFD, err = openDevPPP()
	if err != nil {
		unix.Close(chanFD) //nolint:errcheck // rollback path; primary error is err
		return -1, -1, -1, err
	}
	unitNum = -1 // kernel assigns the unit number
	unitNum, err = ioctlGetSetInt(unitFD, pppiocNewUnit, unitNum)
	if err != nil {
		unix.Close(unitFD) //nolint:errcheck // rollback path; primary error is err
		unix.Close(chanFD) //nolint:errcheck // rollback path; primary error is err
		return -1, -1, -1, fmt.Errorf("l2tp: PPPIOCNEWUNIT: %w", err)
	}

	// Step 4: connect the channel to the unit.
	if err := ioctlSetInt(chanFD, pppiocConnect, unitNum); err != nil {
		unix.Close(unitFD) //nolint:errcheck // rollback path; primary error is err
		unix.Close(chanFD) //nolint:errcheck // rollback path; primary error is err
		return -1, -1, -1, fmt.Errorf("l2tp: PPPIOCCONNECT: %w", err)
	}

	return chanFD, unitFD, unitNum, nil
}

// openDevPPP opens /dev/ppp for read-write.
func openDevPPP() (int, error) {
	fd, err := os.OpenFile("/dev/ppp", os.O_RDWR, 0)
	if err != nil {
		return -1, fmt.Errorf("l2tp: open /dev/ppp: %w", err)
	}
	// Extract the raw fd and prevent Go from closing it when the
	// *os.File is garbage-collected. The caller manages the fd lifetime.
	rawFD, err := dupFD(fd)
	fd.Close() //nolint:errcheck // source *os.File replaced by rawFD below
	if err != nil {
		return -1, fmt.Errorf("l2tp: dup /dev/ppp fd: %w", err)
	}
	return rawFD, nil
}

// dupFD extracts the file descriptor from an *os.File. The returned fd
// is a dup of the original, so the caller owns it independently.
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

// ioctlGetInt performs an ioctl that reads an int value.
func ioctlGetInt(fd int, req uint) (int, error) {
	var val int32
	// SYS_IOCTL takes a pointer; unsafe.Pointer is the only way to pass it.
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&val))) //nolint:gosec // ioctl pointer arg
	if errno != 0 {
		return 0, errno
	}
	return int(val), nil
}

// ioctlSetInt performs an ioctl that writes an int value.
func ioctlSetInt(fd int, req uint, val int) error {
	v := int32(val)
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&v))) //nolint:gosec // ioctl pointer arg
	if errno != 0 {
		return errno
	}
	return nil
}

// ioctlGetSetInt performs an ioctl that both reads and writes an int.
// Used by PPPIOCNEWUNIT which takes a suggested unit number (-1 = auto)
// and returns the allocated number.
func ioctlGetSetInt(fd int, req uint, val int) (int, error) {
	v := int32(val)
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&v))) //nolint:gosec // ioctl pointer arg
	if errno != 0 {
		return 0, errno
	}
	return int(v), nil
}
