// Design: docs/architecture/provisioning/dhcp-server.md -- the DHCP server the probe exchanges with
// Related: probe_linux.go -- the veth pair and the AF_PACKET sockets that carry these frames
// Related: step.go -- the exchange a .ci names, which becomes one of these requests

// Package dhcpprobe is a DHCPv4 client for functional tests. It writes one
// BOOTREQUEST with exactly the fields a test names (ciaddr, giaddr, Requested
// IP Address, Server Identifier, the BROADCAST bit) and reads the server's
// BOOTREPLY off the wire, so a .ci can drive every RFC 2131 Section 4.3.2
// client state against a running daemon. A real client cannot do that: it
// chooses its own state and never sends a request the test asks for.
package dhcpprobe

import (
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
)

// MsgType is the DHCP Message Type (RFC 2132 Section 9.6).
type MsgType uint8

// MsgType values. Zero is not a DHCP message type, so a forgotten type is
// refused rather than sent.
const (
	MsgUnspecified MsgType = 0
	MsgDiscover    MsgType = 1
	MsgOffer       MsgType = 2
	MsgRequest     MsgType = 3
	MsgDecline     MsgType = 4
	MsgAck         MsgType = 5
	MsgNak         MsgType = 6
	MsgRelease     MsgType = 7
)

// String names the type the way the probe prints it and a .ci matches it.
func (t MsgType) String() string {
	switch t {
	case MsgDiscover:
		return "discover"
	case MsgOffer:
		return "offer"
	case MsgRequest:
		return "request"
	case MsgDecline:
		return "decline"
	case MsgAck:
		return "ack"
	case MsgNak:
		return "nak"
	case MsgRelease:
		return "release"
	case MsgUnspecified:
		return "unspecified"
	}
	return "unknown"
}

// parseMsgType reads a type name a .ci wrote.
func parseMsgType(name string) (MsgType, error) {
	for t := MsgDiscover; t <= MsgRelease; t++ {
		if t.String() == name {
			return t, nil
		}
	}
	return MsgUnspecified, errors.New("dhcpprobe: unknown message type " + name +
		" (want discover, offer, request, decline, ack, nak or release)")
}

// Request is one BOOTREQUEST. A zero address is a field the client leaves
// zero (ciaddr, giaddr) or an option it does not send (Requested IP Address,
// Server Identifier).
type Request struct {
	Type      MsgType
	XID       uint32
	MAC       net.HardwareAddr
	Broadcast bool
	CIAddr    netip.Addr
	GIAddr    netip.Addr
	Requested netip.Addr
	ServerID  netip.Addr
}

// Reply is what the probe read from one BOOTREPLY. An address that is not
// valid is an option the reply did not carry.
type Reply struct {
	Type     MsgType
	XID      uint32
	YIAddr   netip.Addr
	ServerID netip.Addr
	Router   netip.Addr
	Mask     netip.Addr
	DNS      netip.Addr
	// Dest is the IP destination and UDP port the server sent the reply to.
	Dest netip.AddrPort
}

// BOOTP and DHCP framing (RFC 2131 Section 2, RFC 2132 Section 3).
const (
	ethHeaderLen  = 14
	ipv4HeaderLen = 20
	udpHeaderLen  = 8
	bootpLen      = 236
	cookieLen     = 4
	// bootpMinLen is the minimum BOOTP message RFC 1542 Section 2.1 has a
	// relay accept, so the request is padded to it.
	bootpMinLen = 300

	etherTypeIPv4 = 0x0800
	protoUDP      = 17
	portServer    = 67
	portClient    = 68

	opRequest = 1
	opReply   = 2

	optPad       = 0
	optMask      = 1
	optRouter    = 3
	optDNS       = 6
	optRequested = 50
	optMsgType   = 53
	optServerID  = 54
	optEnd       = 255

	flagBroadcast = 0x8000
)

var magicCookie = [cookieLen]byte{99, 130, 83, 99}

// buildFrame answers the Ethernet frame that carries req from mac, broadcast
// at the link and IP layers. The IP source is 0.0.0.0 whatever the request
// says: the server reads ciaddr and giaddr from the message, never from the
// IP header, and a limited broadcast from 0.0.0.0 is the one IP source the
// receiving kernel accepts whatever its routes and rp_filter say.
func buildFrame(req Request) ([]byte, error) {
	if req.Type == MsgUnspecified {
		return nil, errors.New("dhcpprobe: the request has no message type")
	}
	if len(req.MAC) != 6 {
		return nil, errors.New("dhcpprobe: the request needs a 6-octet Ethernet hardware address, got " + req.MAC.String())
	}
	msg := make([]byte, bootpLen, bootpMinLen)
	msg[0] = opRequest
	msg[1] = 1 // htype Ethernet
	msg[2] = 6 // hlen
	binary.BigEndian.PutUint32(msg[4:8], req.XID)
	if req.Broadcast {
		binary.BigEndian.PutUint16(msg[10:12], flagBroadcast)
	}
	sourcePort := uint16(portClient)
	if req.GIAddr.IsValid() {
		msg[3] = 1 // hops: one relay forwarded it
		sourcePort = portServer
	}
	putAddr(msg[12:16], req.CIAddr)
	putAddr(msg[24:28], req.GIAddr)
	copy(msg[28:34], req.MAC)
	msg = append(msg, magicCookie[:]...)
	msg = append(msg, optMsgType, 1, byte(req.Type))
	if req.Requested.IsValid() {
		msg = appendAddrOption(msg, optRequested, req.Requested)
	}
	if req.ServerID.IsValid() {
		msg = appendAddrOption(msg, optServerID, req.ServerID)
	}
	msg = append(msg, optEnd)
	for len(msg) < bootpMinLen {
		msg = append(msg, optPad)
	}

	frame := make([]byte, ethHeaderLen+ipv4HeaderLen+udpHeaderLen+len(msg))
	copy(frame[0:6], []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
	copy(frame[6:12], req.MAC)
	binary.BigEndian.PutUint16(frame[12:14], etherTypeIPv4)
	ip := frame[ethHeaderLen : ethHeaderLen+ipv4HeaderLen]
	ip[0] = 0x45
	binary.BigEndian.PutUint16(ip[2:4], uint16(ipv4HeaderLen+udpHeaderLen+len(msg))) //nolint:gosec // bounded: a BOOTREQUEST of a few hundred octets
	ip[8] = 64
	ip[9] = protoUDP
	copy(ip[16:20], []byte{255, 255, 255, 255})
	binary.BigEndian.PutUint16(ip[10:12], ^checksum(0, ip))
	udp := frame[ethHeaderLen+ipv4HeaderLen:]
	binary.BigEndian.PutUint16(udp[0:2], sourcePort)
	binary.BigEndian.PutUint16(udp[2:4], portServer)
	binary.BigEndian.PutUint16(udp[4:6], uint16(udpHeaderLen+len(msg))) //nolint:gosec // bounded as above
	// The UDP checksum stays zero, which RFC 768 lets an IPv4 sender use for
	// "no checksum".
	copy(udp[udpHeaderLen:], msg)
	return frame, nil
}

// parseReply reads a BOOTREPLY out of an Ethernet frame. ok is false for any
// frame that is not an IPv4 UDP datagram to port 67 or 68 carrying a
// BOOTREPLY with the DHCP magic cookie: the capture sees every frame on the
// link, ARP included, and only a reply answers.
func parseReply(frame []byte) (reply Reply, ok bool) {
	if len(frame) < ethHeaderLen+ipv4HeaderLen+udpHeaderLen {
		return Reply{}, false
	}
	if binary.BigEndian.Uint16(frame[12:14]) != etherTypeIPv4 {
		return Reply{}, false
	}
	ip := frame[ethHeaderLen:]
	headerLen := int(ip[0]&0x0f) * 4
	if ip[0]>>4 != 4 {
		return Reply{}, false
	}
	if headerLen < ipv4HeaderLen {
		return Reply{}, false
	}
	if ip[9] != protoUDP {
		return Reply{}, false
	}
	if len(ip) < headerLen+udpHeaderLen {
		return Reply{}, false
	}
	udp := ip[headerLen:]
	port := binary.BigEndian.Uint16(udp[2:4])
	if port != portClient && port != portServer {
		return Reply{}, false
	}
	msg := udp[udpHeaderLen:]
	if len(msg) < bootpLen+cookieLen {
		return Reply{}, false
	}
	if msg[0] != opReply {
		return Reply{}, false
	}
	if [cookieLen]byte(msg[bootpLen:bootpLen+cookieLen]) != magicCookie {
		return Reply{}, false
	}
	reply.XID = binary.BigEndian.Uint32(msg[4:8])
	reply.YIAddr = netip.AddrFrom4([4]byte(msg[16:20]))
	reply.Dest = netip.AddrPortFrom(netip.AddrFrom4([4]byte(ip[16:20])), port)
	readOptions(msg[bootpLen+cookieLen:], &reply)
	return reply, reply.Type != MsgUnspecified
}

// readOptions fills reply from the options field. The walk stops at End, at
// the end of the field, or at an option whose length runs past it. The
// server never overloads file or sname (dhcp-server.md, "Traps"), so the
// options field is the only one read.
func readOptions(options []byte, reply *Reply) {
	for i := 0; i < len(options); {
		code := options[i]
		if code == optEnd {
			return
		}
		if code == optPad {
			i++
			continue
		}
		if i+1 >= len(options) {
			return
		}
		size := int(options[i+1])
		value := options[i+2:]
		if size > len(value) {
			return
		}
		value = value[:size]
		i += 2 + size
		switch code {
		case optMsgType:
			if size == 1 {
				reply.Type = MsgType(value[0])
			}
		case optServerID:
			reply.ServerID = firstAddr(value)
		case optRouter:
			reply.Router = firstAddr(value)
		case optMask:
			reply.Mask = firstAddr(value)
		case optDNS:
			reply.DNS = firstAddr(value)
		}
	}
}

// firstAddr answers the first IPv4 address of an address option, or the zero
// Addr (an option the reply did not carry in a usable form) when it is
// shorter than one address.
func firstAddr(value []byte) netip.Addr {
	if len(value) < 4 {
		return netip.Addr{}
	}
	return netip.AddrFrom4([4]byte(value[:4]))
}

func putAddr(field []byte, addr netip.Addr) {
	if !addr.IsValid() {
		return
	}
	four := addr.As4()
	copy(field, four[:])
}

func appendAddrOption(msg []byte, code byte, addr netip.Addr) []byte {
	four := addr.As4()
	return append(append(msg, code, 4), four[:]...)
}

// checksum folds the one's-complement sum of data into sum (RFC 1071).
func checksum(sum uint32, data []byte) uint16 {
	for i := 0; i+1 < len(data); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(data[i : i+2]))
	}
	if len(data)%2 == 1 {
		sum += uint32(data[len(data)-1]) << 8
	}
	for sum > 0xffff {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return uint16(sum) //nolint:gosec // folded to 16 bits above
}
