// Design: plan/learned/706-cpe-2-dhcp-server.md -- DHCP packet handling (RFC 2131/2132)

package dhcpserver

import (
	"encoding/binary"
	"net"
	"net/netip"

	"github.com/ze-software/ze/internal/core/clock"
)

// RFC 2131 Section 2: DHCP message op codes.
const (
	opRequest = 1
	opReply   = 2
)

// RFC 2131 Section 2: hardware type/length for Ethernet.
const (
	htypeEthernet = 1
	hlenEthernet  = 6
)

// RFC 2131 Section 2: magic cookie at start of options field.
const magicCookie = 0x63825363

// RFC 2132 Section 9.6: DHCP message type values.
const (
	msgDiscover = 1
	msgOffer    = 2
	msgRequest  = 3
	msgDecline  = 4
	msgAck      = 5
	msgNak      = 6
	msgRelease  = 7
	msgInform   = 8
)

// RFC 2132: option codes used by this server.
const (
	optPad            = 0
	optSubnetMask     = 1
	optRouter         = 3
	optDNS            = 6
	optDomainName     = 15
	optVendorSpecific = 43
	optRequestedIP    = 50
	optLeaseTime      = 51
	optMessageType    = 53
	optServerID       = 54
	optParamReqList   = 55
	optT1             = 58
	optT2             = 59
	optVendorClassID  = 60
	optTFTPServerName = 66
	optBootfileName   = 67
	optUserClass      = 77
	optClientArch     = 93
	optEnd            = 255
)

// RFC 4578 Section 2.1: client system architecture types.
const pxeArchUEFIx64 = 7

// RFC 2131 Section 2: fixed header size before options.
const dhcpHeaderLen = 236

// Minimum valid DHCP packet: header + magic cookie + message type option + end.
const minPacketLen = dhcpHeaderLen + 4 + 3 + 1

type dhcpHandler struct {
	subnet      subnetConfig
	serverIP    netip.Addr
	pxe         pxeConfig
	pool        *pool
	leases      *leaseTable
	staticByMAC map[string]netip.Addr
}

func newDHCPHandler(sub subnetConfig, serverIP netip.Addr, pxe pxeConfig) *dhcpHandler {
	p := newPool(sub.Ranges, sub.StaticMappings)
	lt := newLeaseTable(p, clock.RealClock{})

	staticByMAC := make(map[string]netip.Addr, len(sub.StaticMappings))
	for _, sm := range sub.StaticMappings {
		staticByMAC[string(sm.MAC)] = sm.IP
	}

	return &dhcpHandler{
		subnet:      sub,
		serverIP:    serverIP,
		pxe:         pxe,
		pool:        p,
		leases:      lt,
		staticByMAC: staticByMAC,
	}
}

func (h *dhcpHandler) handle(pkt []byte) []byte {
	if len(pkt) < minPacketLen {
		return nil
	}
	if pkt[0] != opRequest {
		return nil
	}
	if binary.BigEndian.Uint32(pkt[236:240]) != magicCookie {
		return nil
	}
	if pkt[2] == 0 {
		return nil
	}

	if !h.matchesSubnet(pkt) {
		return nil
	}

	msgType := parseMsgType(pkt)
	mac := extractMAC(pkt)

	switch msgType {
	case msgDiscover:
		return h.handleDiscover(pkt, mac)
	case msgRequest:
		return h.handleRequest(pkt, mac)
	case msgRelease:
		h.handleRelease(mac)
		return nil
	case msgDecline:
		h.handleDecline(pkt, mac)
		return nil
	default:
		return nil
	}
}

func (h *dhcpHandler) matchesSubnet(pkt []byte) bool {
	giaddr := netip.AddrFrom4([4]byte(pkt[24:28]))
	if giaddr.IsValid() && !giaddr.IsUnspecified() {
		return h.subnet.Prefix.Contains(giaddr)
	}
	ciaddr := netip.AddrFrom4([4]byte(pkt[12:16]))
	if ciaddr.IsValid() && !ciaddr.IsUnspecified() {
		return h.subnet.Prefix.Contains(ciaddr)
	}
	requestedIP := parseOptionAddr(pkt, optRequestedIP)
	if requestedIP.IsValid() {
		return h.subnet.Prefix.Contains(requestedIP)
	}
	return true
}

// RFC 2131 Section 4.3.1: if no address available, remain silent.
func (h *dhcpHandler) handleDiscover(pkt []byte, mac net.HardwareAddr) []byte {
	addr, ok := h.allocateForClient(mac)
	if !ok {
		return nil
	}
	return h.buildReply(pkt, msgOffer, addr)
}

func (h *dhcpHandler) handleRequest(pkt []byte, mac net.HardwareAddr) []byte {
	requestedIP := parseOptionAddr(pkt, optRequestedIP)
	serverID := parseOptionAddr(pkt, optServerID)

	// RFC 2131 Section 4.3.2: SELECTING state (server identifier present).
	if serverID.IsValid() {
		if serverID != h.serverIP {
			return nil
		}
		if !requestedIP.IsValid() {
			return h.buildNak(pkt)
		}
		return h.commitBinding(pkt, mac, requestedIP)
	}

	// RFC 2131 Section 4.3.2: INIT-REBOOT (requested IP present, no server ID).
	if requestedIP.IsValid() {
		if !h.subnet.Prefix.Contains(requestedIP) {
			return h.buildNak(pkt)
		}
		return h.commitBinding(pkt, mac, requestedIP)
	}

	// RFC 2131 Section 4.3.2: RENEWING/REBINDING (ciaddr filled in).
	ciaddr := netip.AddrFrom4([4]byte(pkt[12:16]))
	if ciaddr.IsValid() && !ciaddr.IsUnspecified() {
		return h.commitBinding(pkt, mac, ciaddr)
	}

	return h.buildNak(pkt)
}

func (h *dhcpHandler) commitBinding(pkt []byte, mac net.HardwareAddr, addr netip.Addr) []byte {
	h.pool.reserve(addr, mac)
	h.leases.add(mac, addr, h.subnet.LeaseTimeSec)
	return h.buildReply(pkt, msgAck, addr)
}

func (h *dhcpHandler) handleRelease(mac net.HardwareAddr) {
	h.leases.release(mac)
}

// RFC 2131 Section 4.3.3: mark the declined address as not available.
func (h *dhcpHandler) handleDecline(pkt []byte, mac net.HardwareAddr) {
	declined := parseOptionAddr(pkt, optRequestedIP)
	if declined.IsValid() {
		h.pool.markUnavailable(declined)
	}
	h.leases.release(mac)
}

func (h *dhcpHandler) allocateForClient(mac net.HardwareAddr) (netip.Addr, bool) {
	if ip, ok := h.staticByMAC[string(mac)]; ok {
		return ip, true
	}
	return h.pool.allocate(mac)
}

func (h *dhcpHandler) buildReply(req []byte, msgType byte, yiaddr netip.Addr) []byte {
	resp := make([]byte, 1500)
	resp[0] = opReply
	resp[1] = req[1]
	resp[2] = req[2]
	copy(resp[4:8], req[4:8])     // xid
	copy(resp[10:12], req[10:12]) // flags
	copy(resp[24:28], req[24:28]) // giaddr
	copy(resp[28:44], req[28:44]) // chaddr

	// RFC 2131 Table 3: ciaddr = "from DHCPREQUEST or 0" for ACK.
	if msgType == msgAck {
		copy(resp[12:16], req[12:16])
	}

	ip4 := yiaddr.As4()
	copy(resp[16:20], ip4[:])

	sip := h.serverIP.As4()
	copy(resp[20:24], sip[:])

	binary.BigEndian.PutUint32(resp[236:240], magicCookie)
	off := 240
	limit := len(resp) - 1

	off = safeAppendOption(resp, off, limit, optMessageType, []byte{msgType})
	off = safeAppendOption(resp, off, limit, optServerID, sip[:])

	if msgType == msgOffer || msgType == msgAck {
		// RFC 2132 Section 3.3: subnet mask MUST appear before router.
		mask := prefixToMask(h.subnet.Prefix)
		off = safeAppendOption(resp, off, limit, optSubnetMask, mask[:])

		if h.subnet.DefaultRouter.IsValid() {
			rip := h.subnet.DefaultRouter.As4()
			off = safeAppendOption(resp, off, limit, optRouter, rip[:])
		}

		if len(h.subnet.DNSServers) > 0 {
			dns := make([]byte, 0, 4*len(h.subnet.DNSServers))
			for _, d := range h.subnet.DNSServers {
				dip := d.As4()
				dns = append(dns, dip[:]...)
			}
			off = safeAppendOption(resp, off, limit, optDNS, dns)
		}

		if h.subnet.DomainName != "" {
			off = safeAppendOption(resp, off, limit, optDomainName, []byte(h.subnet.DomainName))
		}

		var lt [4]byte
		binary.BigEndian.PutUint32(lt[:], h.subnet.LeaseTimeSec)
		off = safeAppendOption(resp, off, limit, optLeaseTime, lt[:])

		// RFC 2131 Section 4.4.5: T1 = 0.5 * lease, T2 = 0.875 * lease.
		t1 := h.subnet.LeaseTimeSec / 2
		t2 := h.subnet.LeaseTimeSec * 7 / 8
		var t1b, t2b [4]byte
		binary.BigEndian.PutUint32(t1b[:], t1)
		binary.BigEndian.PutUint32(t2b[:], t2)
		off = safeAppendOption(resp, off, limit, optT1, t1b[:])
		off = safeAppendOption(resp, off, limit, optT2, t2b[:])

		off = h.appendPXEOptions(req, resp, off, limit)
	}

	resp[off] = optEnd

	return resp[:off+1]
}

func (h *dhcpHandler) appendPXEOptions(req, resp []byte, off, limit int) int {
	if !h.pxe.Enabled || !isPXEClient(req) {
		return off
	}

	if h.pxe.BootScriptURL != "" && isIPXE(req) {
		off = safeAppendOption(resp, off, limit, optVendorClassID, []byte("PXEClient"))
		off = safeAppendOption(resp, off, limit, optBootfileName, []byte(h.pxe.BootScriptURL))
		return off
	}

	arch := parsePXEArch(req)
	bootfile := h.pxe.BootfileBIOS
	if arch == pxeArchUEFIx64 {
		bootfile = h.pxe.BootfileUEFI
	}

	// RFC 2131 Section 4.3.1: siaddr identifies the TFTP server for next bootstrap step.
	tftpIP := h.pxe.TFTPServer.As4()
	copy(resp[20:24], tftpIP[:])

	// RFC 2132 Section 9.6: option 60 vendor class identifier.
	off = safeAppendOption(resp, off, limit, optVendorClassID, []byte("PXEClient"))

	// RFC 2132 Section 9.9: option 66 TFTP server name.
	off = safeAppendOption(resp, off, limit, optTFTPServerName, []byte(h.pxe.TFTPServer.String()))

	// RFC 2132 Section 9.10: option 67 bootfile name.
	off = safeAppendOption(resp, off, limit, optBootfileName, []byte(bootfile))

	// PXE Specification 2.1 §3.3: option 43 vendor-specific.
	// Sub-option 6 (PXE_DISCOVERY_CONTROL) bit 3: download the bootfile from
	// DHCP option 67 directly, skip Boot Server Discovery entirely.
	off = safeAppendOption(resp, off, limit, optVendorSpecific, []byte{
		6, 1, 0x08,
		255,
	})

	return off
}

func (h *dhcpHandler) buildNak(req []byte) []byte {
	resp := make([]byte, 300)
	resp[0] = opReply
	resp[1] = req[1]
	resp[2] = req[2]
	copy(resp[4:8], req[4:8])     // xid
	copy(resp[10:12], req[10:12]) // flags
	copy(resp[24:28], req[24:28]) // giaddr
	copy(resp[28:44], req[28:44]) // chaddr

	binary.BigEndian.PutUint32(resp[236:240], magicCookie)
	off := 240
	limit := len(resp) - 1

	off = safeAppendOption(resp, off, limit, optMessageType, []byte{msgNak})
	sip := h.serverIP.As4()
	off = safeAppendOption(resp, off, limit, optServerID, sip[:])

	resp[off] = optEnd

	return resp
}

func safeAppendOption(buf []byte, off, limit int, code byte, data []byte) int {
	need := 2 + len(data)
	if off+need > limit {
		return off
	}
	buf[off] = code
	buf[off+1] = byte(len(data))
	copy(buf[off+2:], data)
	return off + need
}

func parseMsgType(pkt []byte) byte {
	opts := pkt[240:]
	for i := 0; i < len(opts)-2; {
		if opts[i] == optEnd {
			break
		}
		if opts[i] == optPad {
			i++
			continue
		}
		l := int(opts[i+1])
		if opts[i] == optMessageType && l == 1 {
			return opts[i+2]
		}
		i += 2 + l
	}
	return 0
}

func parseOptionAddr(pkt []byte, code byte) netip.Addr {
	opts := pkt[240:]
	for i := 0; i < len(opts)-1; {
		if opts[i] == optEnd {
			break
		}
		if opts[i] == optPad {
			i++
			continue
		}
		l := int(opts[i+1])
		if opts[i] == code && l == 4 && i+6 <= len(opts) {
			return netip.AddrFrom4([4]byte(opts[i+2 : i+6]))
		}
		i += 2 + l
	}
	return netip.Addr{}
}

// parseOptionString returns a string-valued option (for example option 67
// bootfile name) or "" when absent. Used for request/reply logging.
func parseOptionString(pkt []byte, code byte) string {
	if len(pkt) < 240 {
		return ""
	}
	opts := pkt[240:]
	for i := 0; i+1 < len(opts); {
		if opts[i] == optEnd {
			break
		}
		if opts[i] == optPad {
			i++
			continue
		}
		l := int(opts[i+1])
		if i+2+l > len(opts) {
			break
		}
		if opts[i] == code {
			return string(opts[i+2 : i+2+l])
		}
		i += 2 + l
	}
	return ""
}

// msgTypeName maps a DHCP message-type value to its RFC 2132 name for logs.
func msgTypeName(t byte) string {
	switch t {
	case msgDiscover:
		return "DISCOVER"
	case msgOffer:
		return "OFFER"
	case msgRequest:
		return "REQUEST"
	case msgDecline:
		return "DECLINE"
	case msgAck:
		return "ACK"
	case msgNak:
		return "NAK"
	case msgRelease:
		return "RELEASE"
	case msgInform:
		return "INFORM"
	default:
		return "UNKNOWN"
	}
}

func extractMAC(pkt []byte) net.HardwareAddr {
	hlen := min(int(pkt[2]), 16)
	mac := make(net.HardwareAddr, hlen)
	copy(mac, pkt[28:28+hlen])
	return mac
}

func prefixToMask(p netip.Prefix) [4]byte {
	bits := p.Bits()
	mask := ^uint32(0) << (32 - bits)
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], mask)
	return b
}

func parseOptionBytes(pkt []byte, code byte) []byte {
	opts := pkt[240:]
	for i := 0; i < len(opts)-1; {
		if opts[i] == optEnd {
			break
		}
		if opts[i] == optPad {
			i++
			continue
		}
		l := int(opts[i+1])
		if i+2+l > len(opts) {
			break
		}
		if opts[i] == code {
			return opts[i+2 : i+2+l]
		}
		i += 2 + l
	}
	return nil
}

func isPXEClient(pkt []byte) bool {
	opt60 := parseOptionBytes(pkt, optVendorClassID)
	return len(opt60) >= 10 && string(opt60[:10]) == "PXEClient:"
}

func isIPXE(pkt []byte) bool {
	opt77 := parseOptionBytes(pkt, optUserClass)
	return len(opt77) >= 4 && string(opt77[:4]) == "iPXE"
}

// RFC 4578 Section 2.1: option 93 contains one or more 2-byte architecture types.
// Returns the first type, or 0 (BIOS) if absent or malformed.
func parsePXEArch(pkt []byte) uint16 {
	opt93 := parseOptionBytes(pkt, optClientArch)
	if len(opt93) < 2 || len(opt93)%2 != 0 {
		return 0
	}
	return binary.BigEndian.Uint16(opt93[:2])
}
