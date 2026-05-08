// Design: docs/research/l2tpv2-implementation-guide.md -- S21 PPPoL2TP socket API, /dev/ppp
// Related: kernel_event.go -- event types that trigger pppox operations

//go:build linux

package l2tp

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"unsafe"

	"golang.org/x/sys/unix"
)

// sockaddrPPPoL2TPSize is the packed size of struct sockaddr_pppol2tp.
// The kernel header uses __packed, so there is NO alignment padding
// between sa_family (uint16) and sa_protocol (uint32). Go's struct
// alignment would insert 2 bytes of padding, producing a 40-byte
// struct instead of the kernel's 38 bytes.
const sockaddrPPPoL2TPSize = 38

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

// buildSockaddrPPPoL2TP constructs the packed binary representation of
// struct sockaddr_pppol2tp for connect(). The peer address must be IPv4.
//
// The kernel struct is __packed (38 bytes). Go's natural alignment would
// insert 2 bytes padding between sa_family (uint16) and sa_protocol
// (uint32), producing 40 bytes. We serialize manually at packed offsets.
//
// Packed layout:
//
//	[0:2]   sa_family   (uint16 LE): AF_PPPOX
//	[2:6]   sa_protocol (uint32 LE): PX_PROTO_OL2TP
//	[6:10]  pid         (int32  LE): 0
//	[10:14] fd          (int32  LE): tunnel UDP socket fd
//	[14:30] sockaddr_in:
//	  [14:16] sin_family (uint16 LE): AF_INET
//	  [16:18] sin_port   (uint16 BE): peer port
//	  [18:22] sin_addr   (4 bytes):   peer IPv4
//	  [22:30] sin_zero   (8 bytes):   zero
//	[30:32] s_tunnel    (uint16 LE): local tunnel ID
//	[32:34] s_session   (uint16 LE): local session ID
//	[34:36] d_tunnel    (uint16 LE): remote tunnel ID
//	[36:38] d_session   (uint16 LE): remote session ID
func buildSockaddrPPPoL2TP(
	socketFD int,
	peerAddr netip.AddrPort,
	localTID, localSID, remoteTID, remoteSID uint16,
) ([]byte, error) {
	if !peerAddr.Addr().Is4() {
		return nil, fmt.Errorf("l2tp: pppol2tp sockaddr requires IPv4, got %s", peerAddr.Addr())
	}

	buf := make([]byte, sockaddrPPPoL2TPSize)
	binary.LittleEndian.PutUint16(buf[0:2], afPPPOX)
	binary.LittleEndian.PutUint32(buf[2:6], pxProtoOL2TP)
	binary.LittleEndian.PutUint32(buf[6:10], 0) // pid
	binary.LittleEndian.PutUint32(buf[10:14], uint32(socketFD))
	binary.LittleEndian.PutUint16(buf[14:16], unix.AF_INET)
	binary.BigEndian.PutUint16(buf[16:18], peerAddr.Port())
	ip4 := peerAddr.Addr().As4()
	copy(buf[18:22], ip4[:])
	// [22:30] sin_zero already zeroed
	binary.LittleEndian.PutUint16(buf[30:32], localTID)
	binary.LittleEndian.PutUint16(buf[32:34], localSID)
	binary.LittleEndian.PutUint16(buf[34:36], remoteTID)
	binary.LittleEndian.PutUint16(buf[36:38], remoteSID)
	return buf, nil
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
