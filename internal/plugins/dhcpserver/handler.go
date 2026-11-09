// Design: docs/architecture/provisioning/dhcp-server.md -- DHCP packet handling (RFC 2131/2132)

package dhcpserver

import (
	"encoding/binary"
	"net"
	"net/netip"
	"slices"
	"sync"

	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/dnssource"
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

// dhcpHandler is one configured subnet: its pool, its lease table and the
// replies it builds. A segment (segment.go) chooses which handler serves a
// request; a handler never decides that itself.
type dhcpHandler struct {
	// network names the shared network the subnet belongs to. The subnets of
	// one shared network share one segment (resolveSegment).
	network     string
	subnet      subnetConfig
	pxe         pxeConfig
	pool        *pool
	leases      *leaseTable
	staticByMAC map[string]netip.Addr
	// learned holds the system-dns records the subnet's received class reads.
	// A handler built alone holds its own empty Follower, the state before any
	// lease; dhcpServer.install points every handler at the plugin's one.
	learned *dnssource.Follower
	// dropped is the last set of host-only addresses the option 6 selection
	// refused, logged when it changes rather than on every reply.
	dropped dnsDropLog
}

// dnsDropLog remembers the host-only addresses a subnet's selection last
// dropped, so each change is logged once (D-16). Safe for concurrent use.
type dnsDropLog struct {
	mu   sync.Mutex
	last []netip.Addr
}

// changed records dropped and reports whether it differs from the last set.
func (d *dnsDropLog) changed(dropped []netip.Addr) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if slices.Equal(d.last, dropped) {
		return false
	}
	d.last = slices.Clone(dropped)
	return true
}

// dnsOffer is what a subnet hands one client for name resolution: the option
// 6 servers and the lease length the reply carries (D-9).
type dnsOffer struct {
	servers  []netip.Addr
	leaseSec uint32
}

// shortLeaseSec is the lease a subnet hands out while its option 6 list waits
// for a received entry (D-9).
//
// RFC 5625 Section 5.3: "Alternatively, the DHCP server might only issue very
// short (i.e., 60 second) leases while the WAN link is down, only reverting to
// more typical lease lengths once the WAN link is up and the upstream DNS
// servers are known."
const shortLeaseSec = 60

// option6ServersMax is the most addresses one option 6 carries: its length
// octet holds 255, and each address is four octets.
const option6ServersMax = 63

func newDHCPHandler(sub subnetConfig, pxe pxeConfig) *dhcpHandler {
	p := newPool(sub.Ranges, sub.StaticMappings)
	return newDHCPHandlerWithTables(sub, pxe, p, newLeaseTable(p, clock.RealClock{}))
}

// newDHCPHandlerWithTables builds a handler for sub over an existing pool and
// lease table. A reload passes the tables of the previous handler of the same
// subnet when its address plan is unchanged, so the leases survive while the
// options and PXE settings are new. lt MUST be the lease table of p.
func newDHCPHandlerWithTables(sub subnetConfig, pxe pxeConfig, p *pool, lt *leaseTable) *dhcpHandler {
	// A subnet built with no list holds no written dns-source container,
	// which is the state D-5 answers with the built-in default; parseSubnet
	// already sets it, so this names the same default for a subnet built in
	// code. The handler never selects from a zero List (List.IsZero).
	if sub.Sources.IsZero() {
		sub.Sources = subnetDNSDefault
	}
	staticByMAC := make(map[string]netip.Addr, len(sub.StaticMappings))
	for _, sm := range sub.StaticMappings {
		staticByMAC[string(sm.MAC)] = sm.IP
	}

	return &dhcpHandler{
		subnet:      sub,
		pxe:         pxe,
		pool:        p,
		leases:      lt,
		staticByMAC: staticByMAC,
		learned:     &dnssource.Follower{},
	}
}

// offerDNS evaluates the subnet's option 6 list now. self is Ze's address
// inside the subnet on the receiving interface, invalid for a relayed request
// (D-6). The lease is shortened to shortLeaseSec while the list reaches the
// received class and finds no usable entry there (D-9); a configured lease
// shorter than that is kept.
func (h *dhcpHandler) offerDNS(self netip.Addr) dnsOffer {
	list := h.subnet.Sources
	candidates := dnssource.Candidates{
		dnssource.ClassConfigured: h.subnet.DNSServers,
		dnssource.ClassReceived:   list.Received(h.learned.Records(), h.leases.clock.Now()),
	}
	if self.IsValid() {
		candidates[dnssource.ClassSelf] = []netip.Addr{self}
	}
	selection := dnssource.Select(list, candidates)
	if h.dropped.changed(selection.Dropped) {
		for _, addr := range selection.Dropped {
			loggerPtr.Load().Warn("dhcpserver: host-only resolver not handed out in option 6",
				"path", subnetDNSSourcePath, "subnet", h.subnet.Prefix, "address", addr)
		}
	}
	offer := dnsOffer{servers: selection.Servers, leaseSec: h.subnet.LeaseTimeSec}
	if len(offer.servers) > option6ServersMax {
		loggerPtr.Load().Warn("dhcpserver: resolvers beyond what option 6 holds not handed out",
			"path", subnetDNSSourcePath, "subnet", h.subnet.Prefix, "selected", len(offer.servers), "sent", option6ServersMax)
		offer.servers = offer.servers[:option6ServersMax]
	}
	if awaitsReceived(list, selection) && offer.leaseSec > shortLeaseSec {
		offer.leaseSec = shortLeaseSec
	}
	return offer
}

// awaitsReceived reports whether list reaches its received class and found no
// usable entry there: under first-usable, no earlier class was used; under
// merge, every class is reached. A list that does not name received, or
// whose received class contributed, does not wait.
func awaitsReceived(list dnssource.List, selection dnssource.Selection) bool {
	for _, class := range list.Sources() {
		if class == dnssource.ClassReceived {
			return !slices.Contains(selection.Used, dnssource.ClassReceived)
		}
		if list.Mode() != dnssource.ModeMerge && slices.Contains(selection.Used, class) {
			return false
		}
	}
	return false
}

// commitBinding binds addr to mac in this subnet's pool and lease table and
// answers the DHCPACK, identified by serverID. The segment decides that addr
// is the client's and which identifier the reply carries (segment.commit);
// this only records it.
//
// self is Ze's address inside the subnet on the receiving interface
// (segment.selfAddress). The lease recorded is the one the ACK carries.
func (h *dhcpHandler) commitBinding(pkt []byte, mac net.HardwareAddr, addr, serverID, self netip.Addr) []byte {
	offer := h.offerDNS(self)
	h.pool.reserve(addr, mac)
	h.leases.add(mac, addr, offer.leaseSec)
	return h.buildOfferedReply(pkt, msgAck, addr, serverID, offer)
}

// buildReply answers req with an OFFER or ACK of yiaddr from this subnet. The
// segment chose serverID (segment.serverID); it fills option 54 and siaddr.
// self is Ze's address inside the subnet on the receiving interface, which
// the self class of option 6 hands out (segment.selfAddress).
func (h *dhcpHandler) buildReply(req []byte, msgType byte, yiaddr, serverID, self netip.Addr) []byte {
	return h.buildOfferedReply(req, msgType, yiaddr, serverID, h.offerDNS(self))
}

// buildOfferedReply is buildReply with the subnet's DNS offer already
// evaluated, so an ACK carries the lease commitBinding recorded.
func (h *dhcpHandler) buildOfferedReply(req []byte, msgType byte, yiaddr, serverID netip.Addr, offer dnsOffer) []byte {
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

	sip := serverID.As4()
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

		// RFC 2132 Section 3.8: "The minimum length for this option is 4
		// octets, and the length MUST always be a multiple of 4." An empty
		// selection sends no option 6, and each server is four octets.
		if len(offer.servers) > 0 {
			dns := make([]byte, 0, 4*len(offer.servers))
			for _, d := range offer.servers {
				dip := d.As4()
				dns = append(dns, dip[:]...)
			}
			off = safeAppendOption(resp, off, limit, optDNS, dns)
		}

		// RFC 5625 Section 5.2: "it is RECOMMENDED that the default value for
		// this option SHOULD be empty, and that this option MUST NOT be sent to
		// clients when no value is configured." Option 15 is written only for a
		// subnet whose domain-name is set.
		if h.subnet.DomainName != "" {
			off = safeAppendOption(resp, off, limit, optDomainName, []byte(h.subnet.DomainName))
		}

		var lt [4]byte
		binary.BigEndian.PutUint32(lt[:], offer.leaseSec)
		off = safeAppendOption(resp, off, limit, optLeaseTime, lt[:])

		// RFC 2131 Section 4.4.5: T1 = 0.5 * lease, T2 = 0.875 * lease.
		t1 := offer.leaseSec / 2
		t2 := offer.leaseSec * 7 / 8
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

// buildNak answers req with a DHCPNAK identified by serverID, which the
// segment chose (segment.serverID).
func (h *dhcpHandler) buildNak(req []byte, serverID netip.Addr) []byte {
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
	sip := serverID.As4()
	off = safeAppendOption(resp, off, limit, optServerID, sip[:])

	resp[off] = optEnd

	return resp
}

// safeAppendOption writes one option at buf[off:] and returns the new offset.
// An option that cannot be written whole is omitted, and off is returned
// unchanged. Config validation refuses a value longer than 255 octets, so the
// first guard is the second check of that pair.
func safeAppendOption(buf []byte, off, limit int, code byte, data []byte) int {
	// RFC 2132 Section 2: "The length octet is followed by "length" octets of
	// data." One octet cannot count more than 255, so longer data would carry a
	// length octet that disagrees with it.
	// RFC 2131 Section 4.1: "Any individual option in the 'options', 'sname' and
	// 'file' fields MUST be entirely contained in that field."
	if len(data) > 255 {
		return off
	}
	need := 2 + len(data)
	if off+need > limit {
		return off
	}
	buf[off] = code
	buf[off+1] = byte(len(data))
	copy(buf[off+2:], data)
	return off + need
}

// parseMsgType returns the DHCP message type (option 53), or 0 when the
// option is absent or is not one octet long. Zero is no message type RFC 2132
// Section 9.6 defines, so handle drops the packet on it.
func parseMsgType(pkt []byte) byte {
	data, ok := findOption(pkt, optMessageType)
	if !ok {
		return 0
	}
	if len(data) != 1 {
		return 0
	}
	return data[0]
}

// parseOptionAddr returns an IPv4-valued option (50, 54, ...), or the invalid
// netip.Addr when the option is absent or is not four octets long.
func parseOptionAddr(pkt []byte, code byte) netip.Addr {
	data, ok := findOption(pkt, code)
	if !ok {
		return netip.Addr{}
	}
	if len(data) != 4 {
		return netip.Addr{}
	}
	return netip.AddrFrom4([4]byte(data))
}

// parseOptionString returns a string-valued option (for example option 67
// bootfile name) or "" when absent. Used for request/reply logging.
func parseOptionString(pkt []byte, code byte) string {
	data, ok := findOption(pkt, code)
	if !ok {
		return ""
	}
	return string(data)
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

// parseOptionBytes returns the data of an option, aliasing pkt, or nil when the
// option is absent. A present option of length zero returns an empty, non-nil
// slice; every caller checks the length it needs.
func parseOptionBytes(pkt []byte, code byte) []byte {
	data, ok := findOption(pkt, code)
	if !ok {
		return nil
	}
	return data
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
