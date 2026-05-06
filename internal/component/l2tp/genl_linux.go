// Design: docs/research/l2tpv2-implementation-guide.md -- S21 Linux kernel L2TP subsystem
// Related: kernel_event.go -- event types consumed by the worker

//go:build linux

package l2tp

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

// L2TP Generic Netlink family name and version.
// RFC 2661 Section 21: the kernel L2TP module registers family "l2tp".
const (
	genlL2TPName    = "l2tp"
	genlL2TPVersion = 1
)

// L2TP Generic Netlink commands.
// These match the kernel's l2tp_genl_ops table in net/l2tp/l2tp_netlink.c.
const (
	l2tpCmdTunnelCreate  = 1
	l2tpCmdTunnelDelete  = 2
	l2tpCmdSessionCreate = 5
	l2tpCmdSessionDelete = 6
)

// L2TP Generic Netlink attributes used by Phase 5.
// Full table in net/l2tp/l2tp_netlink.c; only attributes referenced
// by tunnel/session create/delete are defined here.
const (
	l2tpAttrPwType        = 1  // NLA_U16: pseudowire type (7 = PPP)
	l2tpAttrEncapType     = 2  // NLA_U16: encapsulation (0 = UDP)
	l2tpAttrProtoVersion  = 7  // NLA_U8:  protocol version
	l2tpAttrConnID        = 9  // NLA_U32: local tunnel ID
	l2tpAttrPeerConnID    = 10 // NLA_U32: peer tunnel ID
	l2tpAttrSessionID     = 11 // NLA_U32: local session ID
	l2tpAttrPeerSessionID = 12 // NLA_U32: peer session ID
	l2tpAttrRecvSeq       = 18 // NLA_U8:  require sequence numbers on recv
	l2tpAttrSendSeq       = 19 // NLA_U8:  include sequence numbers on send
	l2tpAttrLNSMode       = 20 // NLA_U8:  LNS mode (auto-enables seq)
	l2tpAttrFD            = 23 // NLA_U32: file descriptor of UDP socket
	l2tpAttrIPSAddr       = 24 // NLA_U32: source IPv4 address
	l2tpAttrIPDAddr       = 25 // NLA_U32: destination IPv4 address
	l2tpAttrUDPSPort      = 26 // NLA_U16: source UDP port
	l2tpAttrUDPDPort      = 27 // NLA_U16: destination UDP port
)

// L2TPv2 pseudowire type for PPP sessions.
const l2tpPWTypePPP uint16 = 7

// l2tpEncapUDP is the encapsulation type for UDP.
const l2tpEncapUDP uint16 = 0

// genlHeader is a 4-byte Generic Netlink message header with an
// explicitly zeroed reserved field. The vishvananda nl.Genlmsg struct
// is only 2 bytes (cmd + version) but serializes 4 bytes via unsafe
// pointer, leaking stack garbage into the reserved field. The kernel
// L2TP module rejects non-zero reserved bytes with ERANGE.
type genlHeader [4]byte

func newGenlHeader(cmd, version uint8) *genlHeader {
	var h genlHeader
	h[0] = cmd
	h[1] = version
	return &h
}

func (h *genlHeader) Len() int          { return 4 }
func (h *genlHeader) Serialize() []byte { return h[:] }

// genlConn holds a resolved Generic Netlink family for the L2TP module.
type genlConn struct {
	familyID uint16
}

// resolveGenlFamily resolves the L2TP Generic Netlink family. This must
// be called after the kernel module is loaded (probeKernelModules).
// RFC 2661 Section 21: family "l2tp", version 1.
func resolveGenlFamily() (*genlConn, error) {
	family, err := netlink.GenlFamilyGet(genlL2TPName)
	if err != nil {
		return nil, fmt.Errorf("l2tp: resolve genl family %q: %w", genlL2TPName, err)
	}
	return &genlConn{familyID: family.ID}, nil
}

// tunnelCreate sends L2TP_CMD_TUNNEL_CREATE to the kernel.
// RFC 2661 Section 21: creates the kernel-side tunnel context that
// intercepts L2TP data messages (T=0) on the UDP socket.
//
// Passes the listener's unconnected UDP socket fd. The kernel associates
// the existing socket with the L2TP tunnel for data-plane interception.
func (g *genlConn) tunnelCreate(localTID, remoteTID uint16, socketFD int, peerAddr netip.AddrPort) (connFD int, err error) {
	req := nl.NewNetlinkRequest(int(g.familyID), unix.NLM_F_ACK)
	req.AddData(newGenlHeader(l2tpCmdTunnelCreate, genlL2TPVersion))
	req.AddData(nl.NewRtAttr(l2tpAttrConnID, nl.Uint32Attr(uint32(localTID))))
	req.AddData(nl.NewRtAttr(l2tpAttrPeerConnID, nl.Uint32Attr(uint32(remoteTID))))
	req.AddData(nl.NewRtAttr(l2tpAttrProtoVersion, nl.Uint8Attr(2)))
	req.AddData(nl.NewRtAttr(l2tpAttrEncapType, nl.Uint16Attr(l2tpEncapUDP)))
	req.AddData(nl.NewRtAttr(l2tpAttrFD, nl.Uint32Attr(uint32(socketFD))))

	_, execErr := req.Execute(unix.NETLINK_GENERIC, 0)
	if execErr != nil {
		return -1, fmt.Errorf("l2tp: genl tunnel create (local=%d peer=%d fd=%d): %w",
			localTID, remoteTID, socketFD, execErr)
	}
	return -1, nil
}

// tunnelDelete sends L2TP_CMD_TUNNEL_DELETE to the kernel.
// RFC 2661 Section 24.25: tunnel deletion after all sessions removed.
func (g *genlConn) tunnelDelete(localTID uint16) error {
	req := nl.NewNetlinkRequest(int(g.familyID), unix.NLM_F_ACK)
	req.AddData(newGenlHeader(l2tpCmdTunnelDelete, genlL2TPVersion))
	req.AddData(nl.NewRtAttr(l2tpAttrConnID, nl.Uint32Attr(uint32(localTID))))

	_, err := req.Execute(unix.NETLINK_GENERIC, 0)
	if err != nil {
		return fmt.Errorf("l2tp: genl tunnel delete (local=%d): %w", localTID, err)
	}
	return nil
}

// sessionCreateParams holds the parameters for L2TP_CMD_SESSION_CREATE.
type sessionCreateParams struct {
	tunnelID  uint16
	localSID  uint16
	remoteSID uint16
	lnsMode   bool
	sendSeq   bool
	recvSeq   bool
}

// sessionCreate sends L2TP_CMD_SESSION_CREATE to the kernel.
// RFC 2661 Section 21: creates the kernel-side session within an
// existing kernel tunnel. The kernel session must exist before the
// PPPoL2TP socket can connect (Section 24.21).
func (g *genlConn) sessionCreate(p sessionCreateParams) error {
	req := nl.NewNetlinkRequest(int(g.familyID), unix.NLM_F_ACK)
	req.AddData(newGenlHeader(l2tpCmdSessionCreate, genlL2TPVersion))
	req.AddData(nl.NewRtAttr(l2tpAttrConnID, nl.Uint32Attr(uint32(p.tunnelID))))
	req.AddData(nl.NewRtAttr(l2tpAttrSessionID, nl.Uint32Attr(uint32(p.localSID))))
	req.AddData(nl.NewRtAttr(l2tpAttrPeerSessionID, nl.Uint32Attr(uint32(p.remoteSID))))
	req.AddData(nl.NewRtAttr(l2tpAttrPwType, nl.Uint16Attr(l2tpPWTypePPP)))

	if p.lnsMode {
		req.AddData(nl.NewRtAttr(l2tpAttrLNSMode, nl.Uint8Attr(1)))
	}
	if p.sendSeq {
		req.AddData(nl.NewRtAttr(l2tpAttrSendSeq, nl.Uint8Attr(1)))
	}
	if p.recvSeq {
		req.AddData(nl.NewRtAttr(l2tpAttrRecvSeq, nl.Uint8Attr(1)))
	}

	_, err := req.Execute(unix.NETLINK_GENERIC, 0)
	if err != nil {
		return fmt.Errorf("l2tp: genl session create (tunnel=%d session=%d): %w",
			p.tunnelID, p.localSID, err)
	}
	return nil
}

// sessionDelete sends L2TP_CMD_SESSION_DELETE to the kernel.
// RFC 2661 Section 24.25: session deletion before tunnel deletion.
func (g *genlConn) sessionDelete(tunnelID, localSID uint16) error {
	req := nl.NewNetlinkRequest(int(g.familyID), unix.NLM_F_ACK)
	req.AddData(newGenlHeader(l2tpCmdSessionDelete, genlL2TPVersion))
	req.AddData(nl.NewRtAttr(l2tpAttrConnID, nl.Uint32Attr(uint32(tunnelID))))
	req.AddData(nl.NewRtAttr(l2tpAttrSessionID, nl.Uint32Attr(uint32(localSID))))

	_, err := req.Execute(unix.NETLINK_GENERIC, 0)
	if err != nil {
		return fmt.Errorf("l2tp: genl session delete (tunnel=%d session=%d): %w",
			tunnelID, localSID, err)
	}
	return nil
}

// marshalTunnelCreateAttrs builds the NLA attribute bytes for a tunnel
// create message without sending it. Used by tests to verify attribute
// encoding independently of the netlink socket.
func marshalTunnelCreateAttrs(localTID, remoteTID uint16, socketFD int) []byte {
	var buf []byte
	buf = appendNLA(buf, l2tpAttrConnID, nl.Uint32Attr(uint32(localTID)))
	buf = appendNLA(buf, l2tpAttrPeerConnID, nl.Uint32Attr(uint32(remoteTID)))
	buf = appendNLA(buf, l2tpAttrProtoVersion, nl.Uint8Attr(2))
	buf = appendNLA(buf, l2tpAttrEncapType, nl.Uint16Attr(l2tpEncapUDP))
	buf = appendNLA(buf, l2tpAttrFD, nl.Uint32Attr(uint32(socketFD)))
	return buf
}

// marshalSessionCreateAttrs builds the NLA attribute bytes for a session
// create message. Used by tests.
func marshalSessionCreateAttrs(p sessionCreateParams) []byte {
	var buf []byte
	buf = appendNLA(buf, l2tpAttrConnID, nl.Uint32Attr(uint32(p.tunnelID)))
	buf = appendNLA(buf, l2tpAttrSessionID, nl.Uint32Attr(uint32(p.localSID)))
	buf = appendNLA(buf, l2tpAttrPeerSessionID, nl.Uint32Attr(uint32(p.remoteSID)))
	buf = appendNLA(buf, l2tpAttrPwType, nl.Uint16Attr(l2tpPWTypePPP))
	if p.lnsMode {
		buf = appendNLA(buf, l2tpAttrLNSMode, nl.Uint8Attr(1))
	}
	if p.sendSeq {
		buf = appendNLA(buf, l2tpAttrSendSeq, nl.Uint8Attr(1))
	}
	if p.recvSeq {
		buf = appendNLA(buf, l2tpAttrRecvSeq, nl.Uint8Attr(1))
	}
	return buf
}

// appendNLA appends a netlink attribute (type + length + data + padding)
// to buf. This replicates the kernel NLA encoding for test verification.
func appendNLA(buf []byte, attrType int, data []byte) []byte {
	// NLA header: 2 bytes length, 2 bytes type.
	nlaLen := 4 + len(data)
	hdr := make([]byte, 4)
	binary.LittleEndian.PutUint16(hdr[0:2], uint16(nlaLen))
	binary.LittleEndian.PutUint16(hdr[2:4], uint16(attrType))
	buf = append(buf, hdr...)
	buf = append(buf, data...)
	// NLA padding to 4-byte boundary.
	if pad := (4 - nlaLen%4) % 4; pad > 0 {
		buf = append(buf, make([]byte, pad)...)
	}
	return buf
}
