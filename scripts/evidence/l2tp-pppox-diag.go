// Design: docs/research/l2tpv2-ze-integration.md -- PPPoL2TP socket diagnostic
//
// Diagnostic: exercises the full Ze L2TP PPPoL2TP code path.
//
// 1. Creates a UDP listener socket (same as Ze's listener)
// 2. Creates tunnel via Generic Netlink (fd-based, proto v2)
// 3. Creates session via Generic Netlink
// 4. Dumps sockaddr_pppol2tp bytes
// 5. Attempts PPPoL2TP socket connect
//
// Usage (inside QEMU VM):
//   go run -buildvcs=false scripts/evidence/l2tp-pppox-diag.go

package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"unsafe" //nolint:gosec // needed for SYS_CONNECT pointer arg

	"github.com/vishvananda/netlink"
	nl "github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

const (
	genlL2TPVersion = 1

	l2tpCmdTunnelCreate  = 1
	l2tpCmdSessionCreate = 5

	l2tpAttrPwType        = 1
	l2tpAttrEncapType     = 2
	l2tpAttrProtoVersion  = 7
	l2tpAttrConnID        = 9
	l2tpAttrPeerConnID    = 10
	l2tpAttrSessionID     = 11
	l2tpAttrPeerSessionID = 12
	l2tpAttrFD            = 23

	afPPPOX      = 24
	pxProtoOL2TP = 1

	sockaddrPPPoL2TPSize = 38
)

type genlHeader [4]byte

func (h *genlHeader) Len() int          { return 4 }
func (h *genlHeader) Serialize() []byte { return h[:] }

func newGenlHeader(cmd, version uint8) *genlHeader {
	var h genlHeader
	h[0] = cmd
	h[1] = version
	return &h
}

func main() {
	fmt.Println("=== L2TP PPPoL2TP Full-Path Diagnostic ===")
	fmt.Printf("Using packed sockaddr size: %d bytes\n", sockaddrPPPoL2TPSize)
	fmt.Println()

	// Step 1: Create UDP listener socket.
	fmt.Println("--- Step 1: Create UDP listener socket ---")
	udpFD, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		fatal("socket: %v", err)
	}
	unix.SetsockoptInt(udpFD, unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
	bindAddr := unix.SockaddrInet4{Port: 1701}
	copy(bindAddr.Addr[:], net.ParseIP("0.0.0.0").To4())
	if err := unix.Bind(udpFD, &bindAddr); err != nil {
		fatal("bind: %v", err)
	}
	fmt.Printf("  UDP socket fd=%d bound to 0.0.0.0:1701\n", udpFD)
	fmt.Println()

	// Step 2: Resolve L2TP genl family.
	fmt.Println("--- Step 2: Resolve L2TP genl family ---")
	family, err := netlink.GenlFamilyGet("l2tp")
	if err != nil {
		fatal("resolve l2tp genl family: %v", err)
	}
	fmt.Printf("  family ID: %d\n", family.ID)
	fmt.Println()

	// Step 3: Create tunnel (fd-based, proto v2).
	localTID := uint16(1)
	remoteTID := uint16(100)
	fmt.Println("--- Step 3: Create tunnel (fd-based, proto v2) ---")
	fmt.Printf("  localTID=%d remoteTID=%d fd=%d\n", localTID, remoteTID, udpFD)
	{
		req := nl.NewNetlinkRequest(int(family.ID), unix.NLM_F_ACK)
		req.AddData(newGenlHeader(l2tpCmdTunnelCreate, genlL2TPVersion))
		req.AddData(nl.NewRtAttr(l2tpAttrConnID, nl.Uint32Attr(uint32(localTID))))
		req.AddData(nl.NewRtAttr(l2tpAttrPeerConnID, nl.Uint32Attr(uint32(remoteTID))))
		req.AddData(nl.NewRtAttr(l2tpAttrProtoVersion, nl.Uint8Attr(2)))
		req.AddData(nl.NewRtAttr(l2tpAttrEncapType, nl.Uint16Attr(0)))
		req.AddData(nl.NewRtAttr(l2tpAttrFD, nl.Uint32Attr(uint32(udpFD))))

		raw := req.Serialize()
		fmt.Printf("  netlink message (%d bytes):\n%s", len(raw), hex.Dump(raw))

		_, err := req.Execute(unix.NETLINK_GENERIC, 0)
		if err != nil {
			fatal("tunnel create: %v", err)
		}
		fmt.Println("  TUNNEL CREATE: SUCCESS")
	}
	fmt.Println()

	// Step 3b: Verify tunnel exists.
	fmt.Println("--- Step 3b: Verify tunnel ---")
	showCmd("ip", "l2tp", "show", "tunnel")
	fmt.Println()

	// Step 4: Create session.
	localSID := uint16(1)
	remoteSID := uint16(100)
	fmt.Println("--- Step 4: Create session ---")
	fmt.Printf("  tunnelID=%d localSID=%d remoteSID=%d pwType=PPP(7)\n", localTID, localSID, remoteSID)
	{
		req := nl.NewNetlinkRequest(int(family.ID), unix.NLM_F_ACK)
		req.AddData(newGenlHeader(l2tpCmdSessionCreate, genlL2TPVersion))
		req.AddData(nl.NewRtAttr(l2tpAttrConnID, nl.Uint32Attr(uint32(localTID))))
		req.AddData(nl.NewRtAttr(l2tpAttrSessionID, nl.Uint32Attr(uint32(localSID))))
		req.AddData(nl.NewRtAttr(l2tpAttrPeerSessionID, nl.Uint32Attr(uint32(remoteSID))))
		req.AddData(nl.NewRtAttr(l2tpAttrPwType, nl.Uint16Attr(7)))

		raw := req.Serialize()
		fmt.Printf("  netlink message (%d bytes):\n%s", len(raw), hex.Dump(raw))

		_, err := req.Execute(unix.NETLINK_GENERIC, 0)
		if err != nil {
			fatal("session create: %v", err)
		}
		fmt.Println("  SESSION CREATE: SUCCESS")
	}
	fmt.Println()

	// Step 4b: Verify session exists.
	fmt.Println("--- Step 4b: Verify session ---")
	showCmd("ip", "l2tp", "show", "session")
	fmt.Println()

	// Step 5: Build packed sockaddr and attempt PPPoL2TP connect.
	fmt.Println("--- Step 5: PPPoL2TP connect ---")

	peerIP := [4]byte{127, 0, 0, 1}
	peerPort := uint16(1701)

	buf := make([]byte, sockaddrPPPoL2TPSize)
	binary.LittleEndian.PutUint16(buf[0:2], afPPPOX)
	binary.LittleEndian.PutUint32(buf[2:6], pxProtoOL2TP)
	binary.LittleEndian.PutUint32(buf[6:10], 0) // pid
	binary.LittleEndian.PutUint32(buf[10:14], uint32(udpFD))
	binary.LittleEndian.PutUint16(buf[14:16], unix.AF_INET)
	binary.BigEndian.PutUint16(buf[16:18], peerPort)
	copy(buf[18:22], peerIP[:])
	binary.LittleEndian.PutUint16(buf[30:32], localTID)
	binary.LittleEndian.PutUint16(buf[32:34], localSID)
	binary.LittleEndian.PutUint16(buf[34:36], remoteTID)
	binary.LittleEndian.PutUint16(buf[36:38], remoteSID)

	fmt.Printf("  sockaddr (%d bytes):\n%s", len(buf), hex.Dump(buf))
	fmt.Printf("  Bytes breakdown (packed offsets):\n")
	fmt.Printf("    [0:2]   sa_family   = %d\n", binary.LittleEndian.Uint16(buf[0:2]))
	fmt.Printf("    [2:6]   sa_protocol = %d\n", binary.LittleEndian.Uint32(buf[2:6]))
	fmt.Printf("    [6:10]  pid         = %d\n", int32(binary.LittleEndian.Uint32(buf[6:10])))
	fmt.Printf("    [10:14] fd          = %d\n", int32(binary.LittleEndian.Uint32(buf[10:14])))
	fmt.Printf("    [14:16] sin_family  = %d\n", binary.LittleEndian.Uint16(buf[14:16]))
	fmt.Printf("    [16:18] sin_port    = %d\n", binary.BigEndian.Uint16(buf[16:18]))
	fmt.Printf("    [18:22] sin_addr    = %d.%d.%d.%d\n", buf[18], buf[19], buf[20], buf[21])
	fmt.Printf("    [30:32] s_tunnel    = %d\n", binary.LittleEndian.Uint16(buf[30:32]))
	fmt.Printf("    [32:34] s_session   = %d\n", binary.LittleEndian.Uint16(buf[32:34]))
	fmt.Printf("    [34:36] d_tunnel    = %d\n", binary.LittleEndian.Uint16(buf[34:36]))
	fmt.Printf("    [36:38] d_session   = %d\n", binary.LittleEndian.Uint16(buf[36:38]))
	fmt.Println()

	// Create PPPoL2TP socket.
	pppoxFD, err := unix.Socket(afPPPOX, unix.SOCK_DGRAM, pxProtoOL2TP)
	if err != nil {
		fatal("pppox socket: %v", err)
	}
	fmt.Printf("  PPPoL2TP socket fd=%d\n", pppoxFD)

	// Connect.
	_, _, errno := unix.RawSyscall(
		unix.SYS_CONNECT,
		uintptr(pppoxFD),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	if errno != 0 {
		fmt.Printf("  PPPOX CONNECT: FAILED: %v (errno=%d)\n", errno, errno)
		fmt.Println()

		fmt.Println("--- /proc/net/pppol2tp ---")
		showCmd("cat", "/proc/net/pppol2tp")
		fmt.Println()

		fmt.Println("--- Check /dev/ppp ---")
		showCmd("ls", "-la", "/dev/ppp")

		os.Exit(1)
	}

	fmt.Println("  PPPOX CONNECT: SUCCESS")
	fmt.Println()

	// Step 6: /dev/ppp channel setup.
	fmt.Println("--- Step 6: /dev/ppp setup ---")
	const (
		pppiocGChan   = 0x80047437 // _IOR('t', 55, int)
		pppiocAttChan = 0x40047438 // _IOW('t', 56, int)
		pppiocNewUnit = 0xc004743e // _IOWR('t', 62, int)
		pppiocConnect = 0x4004743a // _IOW('t', 58, int)
	)

	chanIdx, err := ioctlGetInt(pppoxFD, pppiocGChan)
	if err != nil {
		fmt.Printf("  PPPIOCGCHAN: FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  PPPIOCGCHAN: channel index = %d\n", chanIdx)

	// Open /dev/ppp and attach channel.
	devPPP, err := os.OpenFile("/dev/ppp", os.O_RDWR, 0)
	if err != nil {
		fatal("open /dev/ppp: %v", err)
	}
	chanFD := int(devPPP.Fd())

	if err := ioctlSetInt(chanFD, pppiocAttChan, chanIdx); err != nil {
		fmt.Printf("  PPPIOCATTCHAN: FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  PPPIOCATTCHAN: attached channel %d to fd %d\n", chanIdx, chanFD)

	// Open /dev/ppp again for unit allocation.
	devPPP2, err := os.OpenFile("/dev/ppp", os.O_RDWR, 0)
	if err != nil {
		fatal("open /dev/ppp (unit): %v", err)
	}
	unitFD := int(devPPP2.Fd())

	unitNum, err := ioctlGetSetInt(unitFD, pppiocNewUnit, -1)
	if err != nil {
		fmt.Printf("  PPPIOCNEWUNIT: FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  PPPIOCNEWUNIT: allocated ppp%d (fd %d)\n", unitNum, unitFD)

	// Connect channel to unit.
	if err := ioctlSetInt(chanFD, pppiocConnect, unitNum); err != nil {
		fmt.Printf("  PPPIOCCONNECT: FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  PPPIOCCONNECT: channel %d -> ppp%d\n", chanIdx, unitNum)
	fmt.Println()

	// Verify ppp interface exists.
	fmt.Println("--- Step 7: Verify ppp interface ---")
	showCmd("ip", "link", "show", fmt.Sprintf("ppp%d", unitNum))
	fmt.Println()

	fmt.Println("=== DIAGNOSTIC COMPLETE: FULL PPPoL2TP STACK WORKING ===")
}

func showCmd(args ...string) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Printf("  (%s: %v)\n", args[0], err)
	}
}

func ioctlGetInt(fd int, req uint) (int, error) {
	var val int32
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&val)))
	if errno != 0 {
		return 0, errno
	}
	return int(val), nil
}

func ioctlSetInt(fd int, req uint, val int) error {
	v := int32(val)
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&v)))
	if errno != 0 {
		return errno
	}
	return nil
}

func ioctlGetSetInt(fd int, req uint, val int) (int, error) {
	v := int32(val)
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&v)))
	if errno != 0 {
		return 0, errno
	}
	return int(v), nil
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "FATAL: "+format+"\n", args...)
	os.Exit(1)
}
