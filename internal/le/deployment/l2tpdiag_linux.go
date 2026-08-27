//go:build linux

// Design: scripts/evidence/l2tp-pppox-diag/main.go -- native PPPoL2TP operation order
// Related: scripts/evidence/l2tp-tunnel-diag/main.go -- native tunnel operation order
// Detail: l2tpdiagreport.go -- the shared report

package deployment

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net"
	"strings"
	"syscall"

	nl "github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	genlL2TPVersion      = 1
	genlHeaderSize       = 4
	l2tpCmdTunnelCreate  = 1
	l2tpCmdTunnelGet     = 4
	l2tpCmdSessionCreate = 5
	l2tpCmdSessionGet    = 8

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

	pppiocGChan   = 0x80047437
	pppiocAttChan = 0x40047438
	pppiocNewUnit = 0xc004743e
	pppiocConnect = 0x4004743a
)

type l2tpAttrScalar struct {
	name  string
	width int
}

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

func (h *genlHeader) Len() int          { return len(h) }
func (h *genlHeader) Serialize() []byte { return h[:] }

func newGenlHeader(command uint8) *genlHeader {
	var header genlHeader
	header[0] = command
	header[1] = genlL2TPVersion
	return &header
}

type diagnosticLink struct {
	Name      string
	Index     int
	Type      string
	MTU       int
	Flags     net.Flags
	OperState string
}

type l2tpDiagnosticLinuxOps interface {
	Socket(domain, kind, protocol int) (int, error)
	SetReusePort(fd int) error
	BindIPv4(fd int, address [4]byte, port uint16) error
	Family(name string) (uint16, error)
	Execute(operation string, request *nl.NetlinkRequest) ([][]byte, error)
	Connect(fd int, address []byte) error
	IoctlGetInt(fd int, request uint) (int, error)
	IoctlSetInt(fd int, request uint, value int) error
	IoctlGetSetInt(fd int, request uint, value int) (int, error)
	OpenPPP() (int, error)
	ProcPPPoL2TP() string
	DevPPP() string
	Link(name string) (diagnosticLink, error)
	Operations() []string
}

var newL2TPDiagnosticLinuxOps = configuredL2TPDiagnosticLinuxOps
var checkL2TPDiagnosticPrerequisites = l2tpDiagnosticPrerequisites

func executeL2TPPPoXDiagnostic(options l2tpDiagnosticOptions) (L2TPDiagnosticReport, error) {
	report := L2TPDiagnosticReport{Diagnostic: l2tpPPPoXDiagnosticName, Dumps: []L2TPDiagnosticDump{}, Retained: []L2TPDiagnosticObject{}}
	var page textbuf.Buffer
	page.Str("=== L2TP PPPoL2TP Full-Path Diagnostic ===\n").
		Str("Using packed sockaddr size: ").Int(sockaddrPPPoL2TPSize).Str(" bytes\n\n")

	if err := checkL2TPDiagnosticPrerequisites("the L2TP PPPoL2TP diagnostic"); err != nil {
		report.Output = page.String()
		return report, fatalDiagnosticError("FATAL: ", err)
	}
	operations := newL2TPDiagnosticLinuxOps()
	finish := func(err error) (L2TPDiagnosticReport, error) {
		report.Output = page.String()
		report.Operations = operations.Operations()
		return report, err
	}

	page.Str("--- Step 1: Create UDP listener socket ---\n")
	udpFD, err := operations.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		return finish(fatalDiagnosticError("FATAL: ", l2tpDiagnosticError("socket: ", err)))
	}
	if err := operations.SetReusePort(udpFD); err != nil {
		page.Str("note: SO_REUSEPORT refused on the UDP socket: ").Err(err).Byte('\n')
	}
	if err := operations.BindIPv4(udpFD, options.Local, options.SourcePort); err != nil {
		return finish(fatalDiagnosticError("FATAL: ", l2tpDiagnosticError("bind: ", err)))
	}
	page.Str("  UDP socket fd=").Int(int64(udpFD)).Str(" bound to ").Str(ipv4Text(options.Local)).
		Byte(':').Uint16(options.SourcePort).Str("\n\n")

	page.Str("--- Step 2: Resolve L2TP genl family ---\n")
	familyID, err := operations.Family("l2tp")
	if err != nil {
		return finish(fatalDiagnosticError("FATAL: ", l2tpDiagnosticError("resolve l2tp genl family: ", err)))
	}
	page.Str("  family ID: ").Uint16(familyID).Str("\n\n")

	page.Str("--- Step 3: Create tunnel (fd-based, proto v2) ---\n").
		Str("  localTID=").Uint32(options.TunnelID).Str(" remoteTID=").Uint32(options.PeerTunnelID).
		Str(" fd=").Int(int64(udpFD)).Byte('\n')
	tunnel := newPPPoXTunnelRequest(familyID, udpFD, options)
	appendNetlinkMessage(&page, tunnel.Serialize(), "  ")
	if _, err := operations.Execute("tunnel-create", tunnel); err != nil {
		return finish(fatalDiagnosticError("FATAL: ", l2tpDiagnosticError("tunnel create: ", err)))
	}
	report.Retained = append(report.Retained, L2TPDiagnosticObject{Kind: "tunnel", ID: options.TunnelID, PeerID: options.PeerTunnelID, Retained: true})
	page.Str("  TUNNEL CREATE: SUCCESS\n\n")

	page.Str("--- Step 3b: Verify tunnel ---\n")
	showL2TPDiagnosticDump(&page, &report, operations, familyID, l2tpCmdTunnelGet, "tunnel")
	page.Byte('\n')

	page.Str("--- Step 4: Create session ---\n").
		Str("  tunnelID=").Uint32(options.TunnelID).Str(" localSID=").Uint16(options.SessionID).
		Str(" remoteSID=").Uint16(options.PeerSessionID).Str(" pwType=PPP(7)\n")
	session := newPPPoXSessionRequest(familyID, options)
	appendNetlinkMessage(&page, session.Serialize(), "  ")
	if _, err := operations.Execute("session-create", session); err != nil {
		return finish(fatalDiagnosticError("FATAL: ", l2tpDiagnosticError("session create: ", err)))
	}
	report.Retained = append(report.Retained, L2TPDiagnosticObject{Kind: "session", ID: uint32(options.SessionID), PeerID: uint32(options.PeerSessionID), Retained: true})
	page.Str("  SESSION CREATE: SUCCESS\n\n")

	page.Str("--- Step 4b: Verify session ---\n")
	showL2TPDiagnosticDump(&page, &report, operations, familyID, l2tpCmdSessionGet, "session")
	page.Byte('\n')

	page.Str("--- Step 5: PPPoL2TP connect ---\n")
	sockaddr := packPPPoXSockaddr(udpFD, options)
	page.Str("  sockaddr (").Int(int64(len(sockaddr))).Str(" bytes):\n").Str(hex.Dump(sockaddr))
	appendPPPoXSockaddrBreakdown(&page, sockaddr)

	pppoxFD, err := operations.Socket(afPPPOX, unix.SOCK_DGRAM, pxProtoOL2TP)
	if err != nil {
		return finish(fatalDiagnosticError("FATAL: ", l2tpDiagnosticError("pppox socket: ", err)))
	}
	page.Str("  PPPoL2TP socket fd=").Int(int64(pppoxFD)).Byte('\n')
	if err := operations.Connect(pppoxFD, sockaddr); err != nil {
		page.Str("  PPPOX CONNECT: FAILED: ").Err(err).Str(" (errno=").Int(errnoNumber(err)).Str(")\n\n").
			Str("--- /proc/net/pppol2tp ---\n").Str(operations.ProcPPPoL2TP()).Byte('\n').
			Str("--- Check /dev/ppp ---\n").Str(operations.DevPPP())
		report.Verdict = L2TPDiagnosticFailed
		return finish(nil)
	}
	page.Str("  PPPOX CONNECT: SUCCESS\n\n--- Step 6: /dev/ppp setup ---\n")

	channel, err := operations.IoctlGetInt(pppoxFD, pppiocGChan)
	if err != nil {
		page.Str("  PPPIOCGCHAN: FAILED: ").Err(err).Byte('\n')
		report.Verdict = L2TPDiagnosticFailed
		return finish(nil)
	}
	page.Str("  PPPIOCGCHAN: channel index = ").Int(int64(channel)).Byte('\n')

	channelFD, err := operations.OpenPPP()
	if err != nil {
		return finish(fatalDiagnosticError("FATAL: ", l2tpDiagnosticError("open /dev/ppp: ", err)))
	}
	if err := operations.IoctlSetInt(channelFD, pppiocAttChan, channel); err != nil {
		page.Str("  PPPIOCATTCHAN: FAILED: ").Err(err).Byte('\n')
		report.Verdict = L2TPDiagnosticFailed
		return finish(nil)
	}
	page.Str("  PPPIOCATTCHAN: attached channel ").Int(int64(channel)).Str(" to fd ").Int(int64(channelFD)).Byte('\n')

	unitFD, err := operations.OpenPPP()
	if err != nil {
		return finish(fatalDiagnosticError("FATAL: ", l2tpDiagnosticError("open /dev/ppp (unit): ", err)))
	}
	unit, err := operations.IoctlGetSetInt(unitFD, pppiocNewUnit, -1)
	if err != nil {
		page.Str("  PPPIOCNEWUNIT: FAILED: ").Err(err).Byte('\n')
		report.Verdict = L2TPDiagnosticFailed
		return finish(nil)
	}
	page.Str("  PPPIOCNEWUNIT: allocated ppp").Int(int64(unit)).Str(" (fd ").Int(int64(unitFD)).Str(")\n")
	if err := operations.IoctlSetInt(channelFD, pppiocConnect, unit); err != nil {
		page.Str("  PPPIOCCONNECT: FAILED: ").Err(err).Byte('\n')
		report.Verdict = L2TPDiagnosticFailed
		return finish(nil)
	}
	page.Str("  PPPIOCCONNECT: channel ").Int(int64(channel)).Str(" -> ppp").Int(int64(unit)).Str("\n\n")

	name := textbuf.StrInt("ppp", int64(unit))
	page.Str("--- Step 7: Verify ppp interface ---\n")
	link, err := operations.Link(name)
	if err != nil {
		page.Str("  (link ").Str(name).Str(": ").Err(err).Str(")\n\n")
	} else {
		page.Str("  ").Str(link.Name).Str(": index=").Int(int64(link.Index)).Str(" type=").Str(link.Type).
			Str(" mtu=").Int(int64(link.MTU)).Str(" flags=<").Str(link.Flags.String()).Str("> state=").Str(link.OperState).Str("\n\n")
	}
	page.Str("=== DIAGNOSTIC COMPLETE: FULL PPPoL2TP STACK WORKING ===\n")
	report.Verdict = L2TPDiagnosticWorking
	return finish(nil)
}

func executeL2TPTunnelDiagnostic(options l2tpDiagnosticOptions) (L2TPDiagnosticReport, error) {
	report := L2TPDiagnosticReport{Diagnostic: l2tpTunnelDiagnosticName, Dumps: []L2TPDiagnosticDump{}, Retained: []L2TPDiagnosticObject{}}
	var page textbuf.Buffer
	if err := checkL2TPDiagnosticPrerequisites("the L2TP tunnel diagnostic"); err != nil {
		return report, fatalDiagnosticError("", err)
	}
	operations := newL2TPDiagnosticLinuxOps()
	finish := func(err error) (L2TPDiagnosticReport, error) {
		report.Output = page.String()
		report.Operations = operations.Operations()
		return report, err
	}

	familyID, err := operations.Family("l2tp")
	if err != nil {
		return finish(fatalDiagnosticError("", l2tpDiagnosticError("resolve l2tp genl family: ", err)))
	}
	page.Str("l2tp genl family ID: ").Uint16(familyID).Byte('\n')
	request := newTunnelV3Request(familyID, options)
	page.Str("netlink message (").Int(int64(len(request.Serialize()))).Str(" bytes):\n").
		Str(hex.Dump(request.Serialize())).Byte('\n')
	if _, err := operations.Execute("tunnel-create", request); err != nil {
		page.Str("FAILED: ").Err(err).Byte('\n').
			Str("errno interpretation: ERANGE=").Int(int64(unix.ERANGE)).Str(" EINVAL=").Int(int64(unix.EINVAL)).
			Str(" EEXIST=").Int(int64(unix.EEXIST)).Str(" ENOENT=").Int(int64(unix.ENOENT)).
			Str(" EADDRNOTAVAIL=").Int(int64(unix.EADDRNOTAVAIL)).Byte('\n')
		return finish(reportedDiagnosticError(l2tpDiagnosticError("tunnel create: ", err)))
	}
	report.Retained = append(report.Retained, L2TPDiagnosticObject{Kind: "tunnel", ID: options.TunnelID, PeerID: options.PeerTunnelID, Retained: true})
	page.Str("SUCCESS: tunnel created\n")
	showTunnelDiagnosticDump(&page, &report, operations, familyID)
	report.Verdict = L2TPDiagnosticWorking
	return finish(nil)
}

func l2tpDiagnosticPrerequisites(name string) error {
	if l2tpDiagnosticRecordingEnabled() {
		return nil
	}
	if err := requireLinux(name); err != nil {
		return err
	}
	if !hasNetAdmin() {
		var tb textbuf.Buffer
		return errors.New(tb.Str(name).Str(" requires root or CAP_NET_ADMIN").String())
	}
	loadL2TPModules()
	return nil
}

func newPPPoXTunnelRequest(familyID uint16, udpFD int, options l2tpDiagnosticOptions) *nl.NetlinkRequest {
	request := nl.NewNetlinkRequest(int(familyID), unix.NLM_F_ACK)
	request.AddData(newGenlHeader(l2tpCmdTunnelCreate))
	request.AddData(nl.NewRtAttr(l2tpAttrConnID, nl.Uint32Attr(options.TunnelID)))
	request.AddData(nl.NewRtAttr(l2tpAttrPeerConnID, nl.Uint32Attr(options.PeerTunnelID)))
	request.AddData(nl.NewRtAttr(l2tpAttrProtoVersion, nl.Uint8Attr(2)))
	request.AddData(nl.NewRtAttr(l2tpAttrEncapType, nl.Uint16Attr(0)))
	request.AddData(nl.NewRtAttr(l2tpAttrFD, nl.Uint32Attr(uint32(udpFD))))
	return request
}

func newPPPoXSessionRequest(familyID uint16, options l2tpDiagnosticOptions) *nl.NetlinkRequest {
	request := nl.NewNetlinkRequest(int(familyID), unix.NLM_F_ACK)
	request.AddData(newGenlHeader(l2tpCmdSessionCreate))
	request.AddData(nl.NewRtAttr(l2tpAttrConnID, nl.Uint32Attr(options.TunnelID)))
	request.AddData(nl.NewRtAttr(l2tpAttrSessionID, nl.Uint32Attr(uint32(options.SessionID))))
	request.AddData(nl.NewRtAttr(l2tpAttrPeerSessionID, nl.Uint32Attr(uint32(options.PeerSessionID))))
	request.AddData(nl.NewRtAttr(l2tpAttrPwType, nl.Uint16Attr(7)))
	return request
}

func newTunnelV3Request(familyID uint16, options l2tpDiagnosticOptions) *nl.NetlinkRequest {
	request := nl.NewNetlinkRequest(int(familyID), unix.NLM_F_ACK)
	request.AddData(newGenlHeader(l2tpCmdTunnelCreate))
	request.AddData(nl.NewRtAttr(l2tpAttrConnID, nl.Uint32Attr(options.TunnelID)))
	request.AddData(nl.NewRtAttr(l2tpAttrPeerConnID, nl.Uint32Attr(options.PeerTunnelID)))
	request.AddData(nl.NewRtAttr(l2tpAttrProtoVersion, nl.Uint8Attr(3)))
	request.AddData(nl.NewRtAttr(l2tpAttrEncapType, nl.Uint16Attr(0)))
	request.AddData(nl.NewRtAttr(l2tpAttrUDPSPort, nl.Uint16Attr(options.SourcePort)))
	request.AddData(nl.NewRtAttr(l2tpAttrUDPDPort, nl.Uint16Attr(options.DestinationPort)))
	request.AddData(nl.NewRtAttr(l2tpAttrIPSAddr, options.Local[:]))
	request.AddData(nl.NewRtAttr(l2tpAttrIPDAddr, options.Remote[:]))
	return request
}

func newDumpRequest(familyID uint16, command uint8) *nl.NetlinkRequest {
	request := nl.NewNetlinkRequest(int(familyID), unix.NLM_F_DUMP)
	request.AddData(newGenlHeader(command))
	return request
}

func packPPPoXSockaddr(udpFD int, options l2tpDiagnosticOptions) []byte {
	buffer := make([]byte, sockaddrPPPoL2TPSize)
	binary.LittleEndian.PutUint16(buffer[0:2], afPPPOX)
	binary.LittleEndian.PutUint32(buffer[2:6], pxProtoOL2TP)
	binary.LittleEndian.PutUint32(buffer[6:10], 0)
	binary.LittleEndian.PutUint32(buffer[10:14], uint32(udpFD))
	binary.LittleEndian.PutUint16(buffer[14:16], unix.AF_INET)
	binary.BigEndian.PutUint16(buffer[16:18], options.DestinationPort)
	copy(buffer[18:22], options.Remote[:])
	binary.LittleEndian.PutUint16(buffer[30:32], uint16(options.TunnelID))
	binary.LittleEndian.PutUint16(buffer[32:34], options.SessionID)
	binary.LittleEndian.PutUint16(buffer[34:36], uint16(options.PeerTunnelID))
	binary.LittleEndian.PutUint16(buffer[36:38], options.PeerSessionID)
	return buffer
}

func appendNetlinkMessage(page *textbuf.Buffer, raw []byte, indent string) {
	page.Str(indent).Str("netlink message (").Int(int64(len(raw))).Str(" bytes):\n").Str(hex.Dump(raw))
}

func appendPPPoXSockaddrBreakdown(page *textbuf.Buffer, buffer []byte) {
	page.Str("  Bytes breakdown (packed offsets):\n").
		Str("    [0:2]   sa_family   = ").Uint16(binary.LittleEndian.Uint16(buffer[0:2])).Byte('\n').
		Str("    [2:6]   sa_protocol = ").Uint32(binary.LittleEndian.Uint32(buffer[2:6])).Byte('\n').
		Str("    [6:10]  pid         = ").Int(int64(int32(binary.LittleEndian.Uint32(buffer[6:10])))).Byte('\n').
		Str("    [10:14] fd          = ").Int(int64(int32(binary.LittleEndian.Uint32(buffer[10:14])))).Byte('\n').
		Str("    [14:16] sin_family  = ").Uint16(binary.LittleEndian.Uint16(buffer[14:16])).Byte('\n').
		Str("    [16:18] sin_port    = ").Uint16(binary.BigEndian.Uint16(buffer[16:18])).Byte('\n').
		Str("    [18:22] sin_addr    = ").Str(ipv4Text([4]byte{buffer[18], buffer[19], buffer[20], buffer[21]})).Byte('\n').
		Str("    [30:32] s_tunnel    = ").Uint16(binary.LittleEndian.Uint16(buffer[30:32])).Byte('\n').
		Str("    [32:34] s_session   = ").Uint16(binary.LittleEndian.Uint16(buffer[32:34])).Byte('\n').
		Str("    [34:36] d_tunnel    = ").Uint16(binary.LittleEndian.Uint16(buffer[34:36])).Byte('\n').
		Str("    [36:38] d_session   = ").Uint16(binary.LittleEndian.Uint16(buffer[36:38])).Str("\n\n")
}

func showL2TPDiagnosticDump(page *textbuf.Buffer, report *L2TPDiagnosticReport, operations l2tpDiagnosticLinuxOps, familyID uint16, command uint8, label string) {
	var operation textbuf.Buffer
	messages, err := operations.Execute(operation.Str(label).Str("-dump").String(), newDumpRequest(familyID, command))
	entry := L2TPDiagnosticDump{Kind: label, Messages: len(messages)}
	if err != nil {
		entry.Note = err.Error()
		report.Dumps = append(report.Dumps, entry)
		page.Str("  (").Str(label).Str(" dump failed: ").Err(err).Str(")\n")
		return
	}
	report.Dumps = append(report.Dumps, entry)
	page.Str("  kernel reports ").Int(int64(len(messages))).Byte(' ').Str(label).Str("(s)\n")
	for index, message := range messages {
		if len(message) < genlHeaderSize {
			page.Str("  [").Int(int64(index)).Str("] ").Int(int64(len(message))).Str(" byte reply is shorter than the generic netlink header\n")
			continue
		}
		attributes, err := nl.ParseRouteAttr(message[genlHeaderSize:])
		if err != nil {
			page.Str("  [").Int(int64(index)).Str("] attributes do not parse: ").Err(err).Byte('\n').Str(hex.Dump(message))
			continue
		}
		page.Str("  [").Int(int64(index)).Str("] ").Str(label).Str(":\n")
		for _, attribute := range attributes {
			page.Str("        ").Str(formatL2TPDiagnosticAttr(attribute)).Byte('\n')
		}
	}
}

func showTunnelDiagnosticDump(page *textbuf.Buffer, report *L2TPDiagnosticReport, operations l2tpDiagnosticLinuxOps, familyID uint16) {
	messages, err := operations.Execute("tunnel-dump", newDumpRequest(familyID, l2tpCmdTunnelGet))
	entry := L2TPDiagnosticDump{Kind: "tunnel", Messages: len(messages)}
	if err != nil {
		entry.Note = err.Error()
		report.Dumps = append(report.Dumps, entry)
		page.Str("note: L2TP_CMD_TUNNEL_GET dump failed: ").Err(err).Byte('\n')
		return
	}
	report.Dumps = append(report.Dumps, entry)
	page.Str("kernel reports ").Int(int64(len(messages))).Str(" tunnel message(s):\n")
	for index, message := range messages {
		page.Str("  [").Int(int64(index)).Str("] ").Int(int64(len(message))).Str(" bytes:\n").Str(hex.Dump(message))
	}
}

func formatL2TPDiagnosticAttr(attribute syscall.NetlinkRouteAttr) string {
	var tb textbuf.Buffer
	switch attribute.Attr.Type {
	case l2tpAttrIfName:
		return tb.Str("ifname=").Str(strings.TrimRight(string(attribute.Value), "\x00")).String()
	case l2tpAttrIPSAddr:
		return tb.Str("ip_saddr=").Str(formatL2TPDiagnosticAddr(attribute.Value)).String()
	case l2tpAttrIPDAddr:
		return tb.Str("ip_daddr=").Str(formatL2TPDiagnosticAddr(attribute.Value)).String()
	case l2tpAttrStats:
		return tb.Str("stats: ").Str(formatL2TPDiagnosticStats(attribute.Value)).String()
	}
	scalar, known := l2tpAttrScalars[attribute.Attr.Type]
	if !known {
		return tb.Str("attr[").Uint16(attribute.Attr.Type).Str("]=").Hex(attribute.Value).String()
	}
	value, ok := readL2TPNativeUint(attribute.Value, scalar.width)
	if !ok {
		return tb.Str(scalar.name).Str(": expected ").Int(int64(scalar.width)).
			Str(" bytes, kernel sent ").Int(int64(len(attribute.Value))).Str(": ").Hex(attribute.Value).String()
	}
	return tb.Str(scalar.name).Byte('=').Uint(value).String()
}

func formatL2TPDiagnosticAddr(value []byte) string {
	if len(value) == net.IPv4len {
		return net.IP(value).String()
	}
	var tb textbuf.Buffer
	return tb.Str("(expected ").Int(net.IPv4len).Str(" bytes, kernel sent ").
		Int(int64(len(value))).Str(": ").Hex(value).Byte(')').String()
}

func formatL2TPDiagnosticStats(value []byte) string {
	var tb textbuf.Buffer
	attributes, err := nl.ParseRouteAttr(value)
	if err != nil {
		return tb.Str("(does not parse: ").Err(err).Str(": ").Hex(value).Byte(')').String()
	}
	for index, attribute := range attributes {
		if index > 0 {
			tb.Byte(' ')
		}
		if int(attribute.Attr.Type) >= len(l2tpStatsNames) || l2tpStatsNames[attribute.Attr.Type] == "" {
			tb.Str("attr[").Uint16(attribute.Attr.Type).Str("]=").Hex(attribute.Value)
			continue
		}
		name := l2tpStatsNames[attribute.Attr.Type]
		count, ok := readL2TPNativeUint(attribute.Value, 8)
		if !ok {
			tb.Str(name).Str(": expected 8 bytes, kernel sent ").Int(int64(len(attribute.Value))).Str(": ").Hex(attribute.Value)
			continue
		}
		tb.Str(name).Byte('=').Uint(count)
	}
	return tb.String()
}

type l2tpDiagnosticContextError struct {
	prefix string
	err    error
}

func (e *l2tpDiagnosticContextError) Error() string {
	var tb textbuf.Buffer
	return tb.Str(e.prefix).Err(e.err).String()
}

func (e *l2tpDiagnosticContextError) Unwrap() error { return e.err }

func l2tpDiagnosticError(prefix string, err error) error {
	return &l2tpDiagnosticContextError{prefix: prefix, err: err}
}

func readL2TPNativeUint(value []byte, width int) (uint64, bool) {
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
	default:
		return 0, false
	}
}

func ipv4Text(address [4]byte) string { return net.IP(address[:]).String() }

func errnoNumber(err error) int64 {
	if errno, ok := errors.AsType[syscall.Errno](err); ok {
		return int64(errno)
	}
	return 0
}
