//go:build linux

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
//   CGO_ENABLED=0 go run -buildvcs=false scripts/evidence/l2tp-pppox-diag/main.go

package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
	"unsafe" //nolint:gosec // needed for SYS_CONNECT pointer arg

	"github.com/vishvananda/netlink"
	nl "github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	genlL2TPVersion = 1
	// genlHeaderSize is sizeof(struct genlmsghdr) (linux/genetlink.h): the
	// command, the version, and two reserved bytes ahead of the attributes.
	genlHeaderSize = 4

	// L2TP generic netlink commands (linux/l2tp.h). The GET pair is what this
	// diagnostic dumps with NLM_F_DUMP to read back what the CREATE pair made.
	l2tpCmdTunnelCreate  = 1
	l2tpCmdTunnelGet     = 4
	l2tpCmdSessionCreate = 5
	l2tpCmdSessionGet    = 8

	// L2TP generic netlink attributes (linux/l2tp.h).
	l2tpAttrPwType        = 1
	l2tpAttrEncapType     = 2
	l2tpAttrProtoVersion  = 7
	l2tpAttrIfName        = 8
	l2tpAttrConnID        = 9
	l2tpAttrPeerConnID    = 10
	l2tpAttrSessionID     = 11
	l2tpAttrPeerSessionID = 12
	l2tpAttrFD            = 23
	l2tpAttrIPSAddr       = 24
	l2tpAttrIPDAddr       = 25
	l2tpAttrUDPSPort      = 26
	l2tpAttrUDPDPort      = 27
	l2tpAttrMTU           = 28
	l2tpAttrStats         = 30

	afPPPOX      = 24
	pxProtoOL2TP = 1

	sockaddrPPPoL2TPSize = 38
)

// l2tpAttrScalar describes one L2TP attribute whose value is an unsigned
// integer: the name to print it under, and the width the kernel sends it at.
type l2tpAttrScalar struct {
	name  string
	width int
}

// l2tpAttrScalars names the scalar L2TP attributes this diagnostic can meet in
// a tunnel or a session dump (linux/l2tp.h). An attribute absent from this
// table prints its type and its bytes rather than a guessed name, because a
// value shown under the wrong label is worse than one shown as hex.
var l2tpAttrScalars = map[uint16]l2tpAttrScalar{
	l2tpAttrPwType:        {name: "pw_type", width: 2},
	l2tpAttrEncapType:     {name: "encap_type", width: 2},
	l2tpAttrProtoVersion:  {name: "proto_version", width: 1},
	l2tpAttrConnID:        {name: "conn_id", width: 4},
	l2tpAttrPeerConnID:    {name: "peer_conn_id", width: 4},
	l2tpAttrSessionID:     {name: "session_id", width: 4},
	l2tpAttrPeerSessionID: {name: "peer_session_id", width: 4},
	l2tpAttrFD:            {name: "fd", width: 4},
	l2tpAttrUDPSPort:      {name: "udp_sport", width: 2},
	l2tpAttrUDPDPort:      {name: "udp_dport", width: 2},
	l2tpAttrMTU:           {name: "mtu", width: 2},
}

// l2tpStatsNames names the u64 counters nested inside L2TP_ATTR_STATS, indexed
// by attribute type (linux/l2tp.h). The table stops at RX_ERRORS, the last
// member this file names with certainty; a later member falls through to the
// same hex line as any other unnamed attribute.
var l2tpStatsNames = [...]string{
	1: "tx_packets",
	2: "tx_bytes",
	3: "tx_errors",
	4: "rx_packets",
	5: "rx_bytes",
	6: "rx_seq_discards",
	7: "rx_oos_packets",
	8: "rx_errors",
}

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
	say("Using packed sockaddr size: %d bytes\n", sockaddrPPPoL2TPSize)
	fmt.Println()

	// Step 1: Create UDP listener socket.
	fmt.Println("--- Step 1: Create UDP listener socket ---")
	udpFD, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		fatal("socket: %v", err)
	}
	// SO_REUSEPORT is advisory here: the bind below is what must succeed, and it
	// reports its own failure. Saying so beats discarding it, because a refused
	// sockopt is the first thing to suspect when the bind then fails on a port
	// another process holds.
	if err := unix.SetsockoptInt(udpFD, unix.SOL_SOCKET, unix.SO_REUSEPORT, 1); err != nil {
		say("note: SO_REUSEPORT refused on the UDP socket: %v\n", err)
	}
	bindAddr := unix.SockaddrInet4{Port: 1701}
	copy(bindAddr.Addr[:], net.ParseIP("0.0.0.0").To4())
	if err := unix.Bind(udpFD, &bindAddr); err != nil {
		fatal("bind: %v", err)
	}
	say("  UDP socket fd=%d bound to 0.0.0.0:1701\n", udpFD)
	fmt.Println()

	// Step 2: Resolve L2TP genl family.
	fmt.Println("--- Step 2: Resolve L2TP genl family ---")
	family, err := netlink.GenlFamilyGet("l2tp")
	if err != nil {
		fatal("resolve l2tp genl family: %v", err)
	}
	say("  family ID: %d\n", family.ID)
	fmt.Println()

	// Step 3: Create tunnel (fd-based, proto v2).
	localTID := uint16(1)
	remoteTID := uint16(100)
	fmt.Println("--- Step 3: Create tunnel (fd-based, proto v2) ---")
	say("  localTID=%d remoteTID=%d fd=%d\n", localTID, remoteTID, udpFD)
	{
		req := nl.NewNetlinkRequest(int(family.ID), unix.NLM_F_ACK)
		req.AddData(newGenlHeader(l2tpCmdTunnelCreate, genlL2TPVersion))
		req.AddData(nl.NewRtAttr(l2tpAttrConnID, nl.Uint32Attr(uint32(localTID))))
		req.AddData(nl.NewRtAttr(l2tpAttrPeerConnID, nl.Uint32Attr(uint32(remoteTID))))
		req.AddData(nl.NewRtAttr(l2tpAttrProtoVersion, nl.Uint8Attr(2)))
		req.AddData(nl.NewRtAttr(l2tpAttrEncapType, nl.Uint16Attr(0)))
		req.AddData(nl.NewRtAttr(l2tpAttrFD, nl.Uint32Attr(uint32(udpFD))))

		raw := req.Serialize()
		say("  netlink message (%d bytes):\n%s", len(raw), hex.Dump(raw))

		_, err := req.Execute(unix.NETLINK_GENERIC, 0)
		if err != nil {
			fatal("tunnel create: %v", err)
		}
		fmt.Println("  TUNNEL CREATE: SUCCESS")
	}
	fmt.Println()

	// Step 3b: Verify tunnel exists.
	fmt.Println("--- Step 3b: Verify tunnel ---")
	showL2TPDump(family.ID, l2tpCmdTunnelGet, "tunnel")
	fmt.Println()

	// Step 4: Create session.
	localSID := uint16(1)
	remoteSID := uint16(100)
	fmt.Println("--- Step 4: Create session ---")
	say("  tunnelID=%d localSID=%d remoteSID=%d pwType=PPP(7)\n", localTID, localSID, remoteSID)
	{
		req := nl.NewNetlinkRequest(int(family.ID), unix.NLM_F_ACK)
		req.AddData(newGenlHeader(l2tpCmdSessionCreate, genlL2TPVersion))
		req.AddData(nl.NewRtAttr(l2tpAttrConnID, nl.Uint32Attr(uint32(localTID))))
		req.AddData(nl.NewRtAttr(l2tpAttrSessionID, nl.Uint32Attr(uint32(localSID))))
		req.AddData(nl.NewRtAttr(l2tpAttrPeerSessionID, nl.Uint32Attr(uint32(remoteSID))))
		req.AddData(nl.NewRtAttr(l2tpAttrPwType, nl.Uint16Attr(7)))

		raw := req.Serialize()
		say("  netlink message (%d bytes):\n%s", len(raw), hex.Dump(raw))

		_, err := req.Execute(unix.NETLINK_GENERIC, 0)
		if err != nil {
			fatal("session create: %v", err)
		}
		fmt.Println("  SESSION CREATE: SUCCESS")
	}
	fmt.Println()

	// Step 4b: Verify session exists.
	fmt.Println("--- Step 4b: Verify session ---")
	showL2TPDump(family.ID, l2tpCmdSessionGet, "session")
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

	say("  sockaddr (%d bytes):\n%s", len(buf), hex.Dump(buf))
	say("  Bytes breakdown (packed offsets):\n")
	say("    [0:2]   sa_family   = %d\n", binary.LittleEndian.Uint16(buf[0:2]))
	say("    [2:6]   sa_protocol = %d\n", binary.LittleEndian.Uint32(buf[2:6]))
	say("    [6:10]  pid         = %d\n", int32(binary.LittleEndian.Uint32(buf[6:10])))
	say("    [10:14] fd          = %d\n", int32(binary.LittleEndian.Uint32(buf[10:14])))
	say("    [14:16] sin_family  = %d\n", binary.LittleEndian.Uint16(buf[14:16]))
	say("    [16:18] sin_port    = %d\n", binary.BigEndian.Uint16(buf[16:18]))
	say("    [18:22] sin_addr    = %d.%d.%d.%d\n", buf[18], buf[19], buf[20], buf[21])
	say("    [30:32] s_tunnel    = %d\n", binary.LittleEndian.Uint16(buf[30:32]))
	say("    [32:34] s_session   = %d\n", binary.LittleEndian.Uint16(buf[32:34]))
	say("    [34:36] d_tunnel    = %d\n", binary.LittleEndian.Uint16(buf[34:36]))
	say("    [36:38] d_session   = %d\n", binary.LittleEndian.Uint16(buf[36:38]))
	fmt.Println()

	// Create PPPoL2TP socket.
	pppoxFD, err := unix.Socket(afPPPOX, unix.SOCK_DGRAM, pxProtoOL2TP)
	if err != nil {
		fatal("pppox socket: %v", err)
	}
	say("  PPPoL2TP socket fd=%d\n", pppoxFD)

	// Connect. The kernel takes a sockaddr_pppol2tp by pointer and there is no
	// typed wrapper for it in x/sys/unix, so the address of the buffer this
	// function built is the only way to pass it. buf outlives the call.
	//nolint:gosec // G103: the pointer is to a local buffer this function owns, passed to the connect(2) this diagnostic exists to make.
	_, _, errno := unix.RawSyscall(
		unix.SYS_CONNECT,
		uintptr(pppoxFD),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	if errno != 0 {
		say("  PPPOX CONNECT: FAILED: %v (errno=%d)\n", errno, errno)
		fmt.Println()

		fmt.Println("--- /proc/net/pppol2tp ---")
		showProcPPPoL2TP()
		fmt.Println()

		fmt.Println("--- Check /dev/ppp ---")
		showDevPPP()

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
		say("  PPPIOCGCHAN: FAILED: %v\n", err)
		os.Exit(1)
	}
	say("  PPPIOCGCHAN: channel index = %d\n", chanIdx)

	// Open /dev/ppp and attach channel.
	devPPP, err := os.OpenFile("/dev/ppp", os.O_RDWR, 0)
	if err != nil {
		fatal("open /dev/ppp: %v", err)
	}
	chanFD := int(devPPP.Fd())

	if err := ioctlSetInt(chanFD, pppiocAttChan, chanIdx); err != nil {
		say("  PPPIOCATTCHAN: FAILED: %v\n", err)
		os.Exit(1)
	}
	say("  PPPIOCATTCHAN: attached channel %d to fd %d\n", chanIdx, chanFD)

	// Open /dev/ppp again for unit allocation.
	devPPP2, err := os.OpenFile("/dev/ppp", os.O_RDWR, 0)
	if err != nil {
		fatal("open /dev/ppp (unit): %v", err)
	}
	unitFD := int(devPPP2.Fd())

	unitNum, err := ioctlGetSetInt(unitFD, pppiocNewUnit, -1)
	if err != nil {
		say("  PPPIOCNEWUNIT: FAILED: %v\n", err)
		os.Exit(1)
	}
	say("  PPPIOCNEWUNIT: allocated ppp%d (fd %d)\n", unitNum, unitFD)

	// Connect channel to unit.
	if err := ioctlSetInt(chanFD, pppiocConnect, unitNum); err != nil {
		say("  PPPIOCCONNECT: FAILED: %v\n", err)
		os.Exit(1)
	}
	say("  PPPIOCCONNECT: channel %d -> ppp%d\n", chanIdx, unitNum)
	fmt.Println()

	// Verify ppp interface exists.
	fmt.Println("--- Step 7: Verify ppp interface ---")
	showLink(textbuf.StrInt("ppp", int64(unitNum)))
	fmt.Println()

	fmt.Println("=== DIAGNOSTIC COMPLETE: FULL PPPoL2TP STACK WORKING ===")
}

// showL2TPDump prints what the kernel itself holds for one class of L2TP
// object, read back over the same generic netlink socket the creates above
// wrote to. label names the class for the reader.
//
// It does NOT fork `ip l2tp show`. iproute2 answers with a second
// implementation's view of the kernel where this file holds its own, it is a
// binary neither the appliance image nor a bare network namespace need carry,
// and so the fork reported nothing exactly where the environment is most
// minimal. The dump is the create's own mechanism, one command byte apart.
//
// A failed dump prints and returns: the probes after it still have something to
// say, so this MUST NOT end the run.
func showL2TPDump(familyID uint16, cmd uint8, label string) {
	req := nl.NewNetlinkRequest(int(familyID), unix.NLM_F_DUMP)
	req.AddData(newGenlHeader(cmd, genlL2TPVersion))

	msgs, err := req.Execute(unix.NETLINK_GENERIC, 0)
	if err != nil {
		say("  (%s dump failed: %v)\n", label, err)
		return
	}
	say("  kernel reports %d %s(s)\n", len(msgs), label)
	for index, msg := range msgs {
		if len(msg) < genlHeaderSize {
			say("  [%d] %d byte reply is shorter than the generic netlink header\n", index, len(msg))
			continue
		}
		attrs, err := nl.ParseRouteAttr(msg[genlHeaderSize:])
		if err != nil {
			say("  [%d] attributes do not parse: %v\n%s", index, err, hex.Dump(msg))
			continue
		}
		say("  [%d] %s:\n", index, label)
		for _, attr := range attrs {
			say("        %s\n", formatL2TPAttr(attr))
		}
	}
}

// formatL2TPAttr renders one L2TP netlink attribute for a human.
func formatL2TPAttr(attr syscall.NetlinkRouteAttr) string {
	var tb textbuf.Buffer
	tb.Reset()

	switch attr.Attr.Type {
	case l2tpAttrIfName:
		return tb.Str("ifname=").Str(strings.TrimRight(string(attr.Value), "\x00")).String()
	case l2tpAttrIPSAddr:
		return tb.Str("ip_saddr=").Str(formatL2TPAddr(attr.Value)).String()
	case l2tpAttrIPDAddr:
		return tb.Str("ip_daddr=").Str(formatL2TPAddr(attr.Value)).String()
	case l2tpAttrStats:
		return tb.Str("stats: ").Str(formatL2TPStats(attr.Value)).String()
	}

	scalar, named := l2tpAttrScalars[attr.Attr.Type]
	if !named {
		return tb.Str("attr[").Uint16(attr.Attr.Type).Str("]=").Hex(attr.Value).String()
	}
	value, ok := readNativeUint(attr.Value, scalar.width)
	if !ok {
		return tb.Str(scalar.name).Str(": expected ").Int(int64(scalar.width)).
			Str(" bytes, kernel sent ").Int(int64(len(attr.Value))).
			Str(": ").Hex(attr.Value).String()
	}
	return tb.Str(scalar.name).Byte('=').Uint(value).String()
}

// formatL2TPAddr renders L2TP_ATTR_IP_SADDR or L2TP_ATTR_IP_DADDR, which the
// kernel sends in network order rather than the host order every scalar beside
// them uses.
func formatL2TPAddr(value []byte) string {
	if len(value) != net.IPv4len {
		var tb textbuf.Buffer
		return tb.Reset().Str("(expected ").Int(net.IPv4len).
			Str(" bytes, kernel sent ").Int(int64(len(value))).
			Str(": ").Hex(value).Byte(')').String()
	}
	return net.IP(value).String()
}

// formatL2TPStats renders the counters nested in L2TP_ATTR_STATS on one line.
func formatL2TPStats(value []byte) string {
	var tb textbuf.Buffer
	tb.Reset()

	attrs, err := nl.ParseRouteAttr(value)
	if err != nil {
		return tb.Str("(does not parse: ").Err(err).Str(": ").Hex(value).Byte(')').String()
	}
	for index, attr := range attrs {
		if index > 0 {
			tb.Byte(' ')
		}
		if int(attr.Attr.Type) >= len(l2tpStatsNames) || l2tpStatsNames[attr.Attr.Type] == "" {
			tb.Str("attr[").Uint16(attr.Attr.Type).Str("]=").Hex(attr.Value)
			continue
		}
		name := l2tpStatsNames[attr.Attr.Type]
		count, ok := readNativeUint(attr.Value, 8)
		if !ok {
			tb.Str(name).Str(": expected 8 bytes, kernel sent ").
				Int(int64(len(attr.Value))).Str(": ").Hex(attr.Value)
			continue
		}
		tb.Str(name).Byte('=').Uint(count)
	}
	return tb.String()
}

// readNativeUint reads a host-order unsigned integer of width bytes from the
// front of a netlink attribute value. It reports false when the kernel sent
// fewer bytes than the width, so the caller can say so rather than print a
// number it made up.
func readNativeUint(value []byte, width int) (uint64, bool) {
	if len(value) < width {
		return 0, false
	}
	order := nl.NativeEndian()
	switch width {
	case 1:
		return uint64(value[0]), true
	case 2:
		return uint64(order.Uint16(value)), true
	case 4:
		return uint64(order.Uint32(value)), true
	case 8:
		return order.Uint64(value), true
	}
	panic("BUG: netlink attribute width must be 1, 2, 4 or 8 bytes")
}

// showProcPPPoL2TP prints the kernel's own pppol2tp table whole, as the `cat`
// this replaced did. The path is a constant, so no operator input reaches the
// read.
func showProcPPPoL2TP() {
	const path = "/proc/net/pppol2tp"

	data, err := os.ReadFile(path)
	if err != nil {
		say("  (read %s: %v)\n", path, err)
		return
	}
	if len(data) == 0 {
		say("  (%s is empty)\n", path)
		return
	}
	say("%s", data)
}

// showDevPPP reports what /dev/ppp is, which is the question `ls -la` was asked
// here: whether the node exists at all, and whether it is the character device
// the PPP channel ioctls need rather than a regular file left in its place.
func showDevPPP() {
	const path = "/dev/ppp"

	info, err := os.Stat(path)
	if err != nil {
		say("  (stat %s: %v)\n", path, err)
		return
	}
	say("  %s mode=%s size=%d\n", path, info.Mode(), info.Size())
	if info.Mode()&os.ModeDevice == 0 {
		say("  %s is not a device node, so the PPP channel ioctls have nothing to talk to\n", path)
		return
	}

	// The major and minor live in Rdev, and os.FileInfo.Sys() hands back a
	// *syscall.Stat_t whose Rdev the unix.Major and unix.Minor helpers do not
	// take. Reading the same inode through unix.Stat is one call and no
	// conversion between two spellings of one struct.
	var stat unix.Stat_t
	if err := unix.Stat(path, &stat); err != nil {
		say("  (unix.Stat %s: %v)\n", path, err)
		return
	}
	say("  %s device %d:%d uid=%d gid=%d\n",
		path, unix.Major(stat.Rdev), unix.Minor(stat.Rdev), stat.Uid, stat.Gid)
}

// showLink reports the kernel's own view of one interface, which is what the
// `ip link show` this replaced was asked for: that the interface exists, and
// the index, MTU, flags and operational state it carries.
func showLink(name string) {
	link, err := netlink.LinkByName(name)
	if err != nil {
		say("  (link %s: %v)\n", name, err)
		return
	}
	attrs := link.Attrs()
	say("  %s: index=%d type=%s mtu=%d flags=<%s> state=%s\n",
		attrs.Name, attrs.Index, link.Type(), attrs.MTU, attrs.Flags, attrs.OperState)
}

func ioctlGetInt(fd int, req uint) (int, error) {
	var val int32
	//nolint:gosec // G103: the pointer is to a local this function owns, and the PPP ioctls take their argument by pointer with no typed wrapper in x/sys/unix.
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&val)))
	if errno != 0 {
		return 0, errno
	}
	return int(val), nil
}

func ioctlSetInt(fd int, req uint, val int) error {
	v := int32(val)
	//nolint:gosec // G103: the pointer is to a local this function owns, and the PPP ioctls take their argument by pointer with no typed wrapper in x/sys/unix.
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&v)))
	if errno != 0 {
		return errno
	}
	return nil
}

func ioctlGetSetInt(fd int, req uint, val int) (int, error) {
	v := int32(val)
	//nolint:gosec // G103: the pointer is to a local this function owns, and the PPP ioctls take their argument by pointer with no typed wrapper in x/sys/unix.
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&v)))
	if errno != 0 {
		return 0, errno
	}
	return int(v), nil
}

// say prints one diagnostic line to stdout.
//
// It exists so this file states its output intent once. fmt.Printf is refused
// on a Ze path (ai/rules/performance.md), fmt.Fprintf to os.Stdout is the
// allowed CLI-output form, and a failed write to stdout is nothing a
// diagnostic can act on: the reader has already gone. Checking it at 47 call
// sites would say that 47 times.
func say(format string, args ...any) {
	fmt.Fprintf(os.Stdout, format, args...) //nolint:errcheck // a failed stdout write is not actionable in a one-shot diagnostic
}

// fatal prints one line to stderr and ends the process.
//
// The prefix, the caller's line and the newline go out as three writes rather
// than one concatenated format string. String concatenation with + is refused
// on a Ze path (ai/rules/performance.md), and a format string built at runtime
// is what go vet reports. This diagnostic writes from one goroutine and exits,
// so nothing can arrive between the three writes.
func fatal(format string, args ...any) {
	fmt.Fprint(os.Stderr, "FATAL: ")        //nolint:errcheck // the process is exiting; a failed stderr write changes nothing
	fmt.Fprintf(os.Stderr, format, args...) //nolint:errcheck // the process is exiting; a failed stderr write changes nothing
	fmt.Fprintln(os.Stderr)                 //nolint:errcheck // the process is exiting; a failed stderr write changes nothing
	os.Exit(1)
}
