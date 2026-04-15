// Design: docs/research/l2tpv2-implementation-guide.md -- S21 Linux kernel L2TP subsystem
// Related: kernel_event.go -- event types consumed by the worker

//go:build linux

package l2tp

import (
	"encoding/binary"
	"fmt"

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
	l2tpAttrProtoVersion  = 7  // NLA_U8:  protocol version (2 for L2TPv2)
	l2tpAttrConnID        = 8  // NLA_U32: local tunnel ID
	l2tpAttrPeerConnID    = 9  // NLA_U32: peer tunnel ID
	l2tpAttrSessionID     = 10 // NLA_U32: local session ID
	l2tpAttrPeerSessionID = 11 // NLA_U32: peer session ID
	l2tpAttrRecvSeq       = 17 // NLA_U8:  require sequence numbers on recv
	l2tpAttrSendSeq       = 18 // NLA_U8:  include sequence numbers on send
	l2tpAttrLNSMode       = 19 // NLA_U8:  LNS mode (auto-enables seq)
	l2tpAttrFD            = 22 // NLA_U32: file descriptor of UDP socket
)

// L2TPv2 pseudowire type for PPP sessions.
const l2tpPWTypePPP uint16 = 7

// l2tpEncapUDP is the encapsulation type for UDP.
const l2tpEncapUDP uint16 = 0

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
func (g *genlConn) tunnelCreate(localTID, remoteTID uint16, socketFD int) error {
	req := nl.NewNetlinkRequest(int(g.familyID), unix.NLM_F_ACK)
	req.AddData(&nl.Genlmsg{
		Command: l2tpCmdTunnelCreate,
		Version: genlL2TPVersion,
	})
	req.AddData(nl.NewRtAttr(l2tpAttrConnID, nl.Uint32Attr(uint32(localTID))))
	req.AddData(nl.NewRtAttr(l2tpAttrPeerConnID, nl.Uint32Attr(uint32(remoteTID))))
	req.AddData(nl.NewRtAttr(l2tpAttrProtoVersion, nl.Uint8Attr(2)))
	req.AddData(nl.NewRtAttr(l2tpAttrEncapType, nl.Uint16Attr(l2tpEncapUDP)))
	req.AddData(nl.NewRtAttr(l2tpAttrFD, nl.Uint32Attr(uint32(socketFD))))

	_, err := req.Execute(unix.NETLINK_GENERIC, 0)
	if err != nil {
		return fmt.Errorf("l2tp: genl tunnel create (local=%d peer=%d): %w", localTID, remoteTID, err)
	}
	return nil
}

// tunnelDelete sends L2TP_CMD_TUNNEL_DELETE to the kernel.
// RFC 2661 Section 24.25: tunnel deletion after all sessions removed.
func (g *genlConn) tunnelDelete(localTID uint16) error {
	req := nl.NewNetlinkRequest(int(g.familyID), unix.NLM_F_ACK)
	req.AddData(&nl.Genlmsg{
		Command: l2tpCmdTunnelDelete,
		Version: genlL2TPVersion,
	})
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
	req.AddData(&nl.Genlmsg{
		Command: l2tpCmdSessionCreate,
		Version: genlL2TPVersion,
	})
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
	req.AddData(&nl.Genlmsg{
		Command: l2tpCmdSessionDelete,
		Version: genlL2TPVersion,
	})
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
