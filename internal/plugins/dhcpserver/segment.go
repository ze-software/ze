// Design: docs/architecture/provisioning/dhcp-server.md -- segment resolution and subnet selection
// Related: register.go -- serveRequests, the request task that resolves a segment for each packet
// Related: handler.go -- dhcpHandler, the pool, lease table and replies of one subnet
// Related: neighbor_linux.go -- readDeviceAddrs, the receiving device's addresses

package dhcpserver

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"slices"
)

// segment is the set of subnets one request may be served from: every subnet
// of every shared network that holds the relay's address (giaddr non-zero) or
// an address of the receiving device (giaddr zero), which is the meaning ISC
// dhcpd and Kea give a shared network. subnets[0] is the home subnet, the one
// a new address is allocated from first: the subnet holding giaddr, else the
// subnet holding the device's earliest address. The others follow in
// configuration order. A segment is built for one request and not kept.
//
// Static mappings are read across the whole segment, pools one subnet at a
// time, so a client reserved in any subnet of its segment gets its
// reservation, whichever subnet the receiving interface is addressed in.
//
// device and relayed decide the server identifier every reply carries
// (serverID), and which identifiers a SELECTING request may name
// (serverIDOwner).
type segment struct {
	subnets []*dhcpHandler
	// device lists the receiving device's IPv4 addresses, primary first.
	device []netip.Addr
	// relayed is true when a relay forwarded the request (giaddr non-zero).
	relayed bool
	// warn records why a reply was withheld; requestServer rate-limits it.
	// A segment a test builds without one writes nothing.
	warn func(msg string, attrs ...any)
}

// errNoServerAddress and errServerIdentifierNotHeld are the two reasons
// serverID cannot answer.
var (
	errNoServerAddress         = errors.New("the listen interface holds no address the client can reach the server at")
	errServerIdentifierNotHeld = errors.New("the listen interface does not hold the subnet's server-identifier")
)

// resolveSegment answers the segment a request belongs to, and false when no
// configured subnet holds giaddr (relayed) or any of device (giaddr zero).
// device lists the receiving device's IPv4 addresses, primary first; for a
// relayed request it chooses only the server identifier (serverID).
func resolveSegment(handlers []*dhcpHandler, giaddr netip.Addr, device []netip.Addr) (segment, bool) {
	held := subnetsHolding(handlers, device)
	if !giaddr.IsUnspecified() {
		held = subnetsHolding(handlers, []netip.Addr{giaddr})
	}
	if len(held) == 0 {
		return segment{}, false
	}
	home := held[0]
	seg := segment{subnets: []*dhcpHandler{home}, device: device, relayed: !giaddr.IsUnspecified()}
	for _, h := range handlers {
		if h == home {
			continue
		}
		if !slices.ContainsFunc(held, func(o *dhcpHandler) bool { return o.network == h.network }) {
			continue
		}
		seg.subnets = append(seg.subnets, h)
	}
	return seg, true
}

// subnetsHolding answers the handlers whose subnet contains one of addrs,
// ordered by the address that matched them first, so the first handler holds
// the earliest address that any subnet holds.
func subnetsHolding(handlers []*dhcpHandler, addrs []netip.Addr) []*dhcpHandler {
	var held []*dhcpHandler
	for _, addr := range addrs {
		for _, h := range handlers {
			if !h.subnet.Prefix.Contains(addr) {
				continue
			}
			if slices.Contains(held, h) {
				continue
			}
			held = append(held, h)
		}
	}
	return held
}

// validRequest reports whether pkt is a BOOTREQUEST the server reads: long
// enough for the header, the magic cookie and a message type, with a non-zero
// hardware address length. Anything else is dropped unanswered.
func validRequest(pkt []byte) bool {
	if len(pkt) < minPacketLen {
		return false
	}
	if pkt[0] != opRequest {
		return false
	}
	if binary.BigEndian.Uint32(pkt[236:240]) != magicCookie {
		return false
	}
	return pkt[2] != 0
}

// serve answers a request validRequest accepted, or nil when the server stays
// silent. Safe for concurrent use: the pools and lease tables lock.
func (s segment) serve(pkt []byte) []byte {
	mac := extractMAC(pkt)
	switch parseMsgType(pkt) {
	case msgDiscover:
		return s.discover(pkt, mac)
	case msgRequest:
		return s.request(pkt, mac)
	case msgRelease:
		s.release(mac)
		return nil
	case msgDecline:
		s.decline(pkt, mac)
		return nil
	default:
		// The message type is a client's, so the set is open: an INFORM, a
		// server's own type or an unknown value draws no reply.
		return nil
	}
}

// discover answers a DHCPDISCOVER with an OFFER, or nil when no address is
// available.
//
// RFC 2131 Section 4.3.1: "If an address is available, the new address
// SHOULD be chosen as follows:"
func (s segment) discover(pkt []byte, mac net.HardwareAddr) []byte {
	// RFC 2131 Section 4.3.1: "As described in section 4.2, a server MAY, for
	// administrative reasons, assign an address other than the one requested"
	// A static mapping anywhere on the segment is that administrative choice,
	// so it is offered before any hint or pool is read.
	if h, addr, ok := s.reservation(mac); ok {
		return s.offer(h, pkt, addr)
	}
	// RFC 2131 Section 4.3.1: "The client's current address as recorded in the
	// client's current binding, ELSE"
	if h, addr, ok := s.held(mac); ok {
		return s.offer(h, pkt, addr)
	}
	// RFC 2131 Section 4.3.1: "The address requested in the 'Requested IP
	// Address' option, if that address is valid and not already allocated,
	// ELSE"
	if requested := parseOptionAddr(pkt, optRequestedIP); requested.IsValid() {
		if h := s.subnetOf(requested); h != nil && h.pool.contains(requested) {
			if reply := s.offerRequested(h, pkt, mac, requested); reply != nil {
				return reply
			}
		}
	}
	// RFC 2131 Section 4.3.1: "A new address allocated from the server's pool
	// of available addresses; the address is selected based on the subnet from
	// which the message was received (if 'giaddr' is 0) or on the address of
	// the relay agent that forwarded the message ('giaddr' when not 0)."
	// RFC 2131 Section 4.3.1: "A server is free to choose some other subnet,"
	// so when the home subnet's pool is full the segment's other pools follow.
	for _, h := range s.subnets {
		id, err := s.serverID(h)
		if err != nil {
			s.withheld(h, err)
			continue
		}
		if addr, ok := h.pool.allocate(mac); ok {
			return h.buildReply(pkt, msgOffer, addr, id, s.selfAddress(h))
		}
	}
	// RFC 2131 Section 4.3.1: if no address is available, remain silent.
	return nil
}

// request answers a DHCPREQUEST in the state RFC 2131 Section 4.3.2 reads
// from its fields: SELECTING when it names a server identifier, INIT-REBOOT
// when it names a requested address and no server identifier, RENEWING or
// REBINDING when only ciaddr is set. A request with none of the three is
// refused with a DHCPNAK.
func (s segment) request(pkt []byte, mac net.HardwareAddr) []byte {
	requested := parseOptionAddr(pkt, optRequestedIP)
	serverID := parseOptionAddr(pkt, optServerID)
	if serverID.IsValid() {
		return s.selecting(pkt, mac, serverID, requested)
	}
	if requested.IsValid() {
		return s.initReboot(pkt, mac, requested)
	}
	ciaddr := netip.AddrFrom4([4]byte(pkt[12:16]))
	if !ciaddr.IsUnspecified() {
		return s.renewing(pkt, mac, ciaddr)
	}
	return s.nak(s.subnets[0], pkt)
}

// selecting answers a DHCPREQUEST from a client in SELECTING state.
//
// RFC 2131 Section 4.3.2: "Client inserts the address of the selected server
// in 'server identifier', 'ciaddr' MUST be zero, 'requested IP address' MUST
// be filled in with the yiaddr value from the chosen DHCPOFFER."
func (s segment) selecting(pkt []byte, mac net.HardwareAddr, serverID, requested netip.Addr) []byte {
	// RFC 2131 Section 4.3.2: "If the DHCPREQUEST message contains a 'server
	// identifier' option, the message is in response to a DHCPOFFER message."
	// RFC 2131 Section 4.1 (serverIDOwner): any of the server's addresses on
	// the segment identifies it.
	owner := s.serverIDOwner(serverID)
	// The client selected another server's offer.
	if owner == nil {
		return nil
	}
	if !requested.IsValid() {
		return s.nak(owner, pkt)
	}
	if h, addr, ok := s.reservation(mac); ok {
		if addr != requested {
			return s.nak(owner, pkt)
		}
		return s.commit(h, pkt, mac, addr)
	}
	if h, addr, ok := s.held(mac); ok && addr == requested {
		return s.commit(h, pkt, mac, addr)
	}
	h := s.subnetOf(requested)
	if h != nil && h.pool.contains(requested) {
		// The identifier is checked before the claim, so a subnet that cannot
		// answer holds no address for the client.
		if _, err := s.serverID(h); err != nil {
			s.withheld(h, err)
			return nil
		}
		if h.pool.claim(requested, mac) {
			return s.commit(h, pkt, mac, requested)
		}
	}
	return s.nak(owner, pkt)
}

// initReboot answers a DHCPREQUEST from a client in INIT-REBOOT state, which
// asks to keep requested.
//
// RFC 2131 Section 4.3.2: "Server SHOULD send a DHCPNAK message to the client
// if the 'requested IP address' is incorrect, or is on the wrong network."
func (s segment) initReboot(pkt []byte, mac net.HardwareAddr, requested netip.Addr) []byte {
	h := s.subnetOf(requested)
	// RFC 2131 Section 4.3.2: "If the DHCP server detects that the client is on
	// the wrong net (i.e., the result of applying the local subnet mask or
	// remote subnet mask (if 'giaddr' is not zero) to 'requested IP address'
	// option value doesn't match reality), then the server SHOULD send a DHCPNAK
	// message to the client."
	if h == nil {
		return s.nak(s.subnets[0], pkt)
	}
	// RFC 2131 Section 4.3.2: "If the network is correct, then the DHCP server
	// should check if the client's notion of its IP address is correct. If not,
	// then the server SHOULD send a DHCPNAK message to the client."
	// A reservation, then a binding, is the server's record of that address.
	if owner, addr, ok := s.reservation(mac); ok {
		if addr != requested {
			return s.nak(h, pkt)
		}
		return s.commit(owner, pkt, mac, addr)
	}
	if owner, addr, ok := s.binding(mac); ok {
		if addr != requested {
			return s.nak(h, pkt)
		}
		return s.commit(owner, pkt, mac, addr)
	}
	// RFC 2131 Section 4.3.2: "If the DHCP server has no record of this client,
	// then it MUST remain silent, and MAY output a warning to the network
	// administrator."
	// The address is on the right network and the server holds neither a
	// reservation nor a binding for the client.
	return nil
}

// renewing answers a DHCPREQUEST from a client in RENEWING or REBINDING state,
// which asks to extend its binding to ciaddr. Both states are read from the
// same fields and answered alike.
//
// RFC 2131 Section 4.3.2: "The DHCP server SHOULD check 'ciaddr' for
// correctness before replying to the DHCPREQUEST."
func (s segment) renewing(pkt []byte, mac net.HardwareAddr, ciaddr netip.Addr) []byte {
	h := s.subnetOf(ciaddr)
	// An address in no subnet of the segment is on the wrong network.
	if h == nil {
		return s.nak(s.subnets[0], pkt)
	}
	if !isHostAddress(h.subnet.Prefix, ciaddr) {
		return s.nak(h, pkt)
	}
	// A reservation is the operator's statement of the client's correct
	// address, so a client renewing another address, for example a pool lease
	// granted before the reservation was configured, is refused and returns to
	// INIT, where its DHCPDISCOVER is offered the reservation.
	if owner, addr, ok := s.reservation(mac); ok {
		if addr != ciaddr {
			return s.nak(h, pkt)
		}
		return s.commit(owner, pkt, mac, addr)
	}
	if owner, addr, ok := s.binding(mac); ok {
		if addr != ciaddr {
			return s.nak(h, pkt)
		}
		return s.commit(owner, pkt, mac, addr)
	}
	if s.heldByOther(ciaddr, mac) {
		return s.nak(h, pkt)
	}
	// The server holds no record of the client, for example after a restart
	// that lost its lease table, and nobody else holds ciaddr: the binding is
	// recorded again.
	if !h.pool.claim(ciaddr, mac) {
		return s.nak(h, pkt)
	}
	return s.commit(h, pkt, mac, ciaddr)
}

// release drops the client's binding on the segment.
//
// RFC 2131 Section 4.3.4: "Upon receipt of a DHCPRELEASE message, the server
// marks the network address as not allocated.  The server SHOULD retain a
// record of the client's initialization parameters for possible reuse in
// response to subsequent requests from the client."
func (s segment) release(mac net.HardwareAddr) {
	for _, h := range s.subnets {
		h.leases.release(mac)
	}
}

// decline marks the declined address unavailable in the subnet that holds it
// and drops the client's binding on the segment.
//
// RFC 2131 Section 4.3.3: "If the server receives a DHCPDECLINE message, the
// client has discovered through some other means that the suggested network
// address is already in use.  The server MUST mark the network address as not
// available and SHOULD notify the local system administrator of a possible
// configuration problem."
func (s segment) decline(pkt []byte, mac net.HardwareAddr) {
	if declined := parseOptionAddr(pkt, optRequestedIP); declined.IsValid() {
		if h := s.subnetOf(declined); h != nil {
			h.pool.markUnavailable(declined)
		}
	}
	s.release(mac)
}

// commit binds addr to mac in h and answers the DHCPACK. A client holds one
// binding on its segment, so an address it holds in another subnet of the
// segment is released first. When h cannot name a server identifier
// (serverID) nothing is bound and the request is not answered.
func (s segment) commit(h *dhcpHandler, pkt []byte, mac net.HardwareAddr, addr netip.Addr) []byte {
	id, err := s.serverID(h)
	if err != nil {
		s.withheld(h, err)
		return nil
	}
	for _, o := range s.subnets {
		if o == h {
			continue
		}
		previous, ok := o.pool.addrFor(mac)
		if !ok {
			continue
		}
		o.leases.release(mac)
		o.pool.release(previous)
	}
	return h.commitBinding(pkt, mac, addr, id, s.selfAddress(h))
}

// offerRequested answers the DHCPOFFER of requested from h when h's pool can
// claim it for mac, or nil. The identifier is checked before the claim, so a
// subnet that cannot answer (serverID) takes no address for the client, and
// the pools are read next.
func (s segment) offerRequested(h *dhcpHandler, pkt []byte, mac net.HardwareAddr, requested netip.Addr) []byte {
	id, err := s.serverID(h)
	if err != nil {
		s.withheld(h, err)
		return nil
	}
	if !h.pool.claim(requested, mac) {
		return nil
	}
	return h.buildReply(pkt, msgOffer, requested, id, s.selfAddress(h))
}

// offer answers the DHCPOFFER of addr from h, or nil when h cannot name a
// server identifier (serverID).
func (s segment) offer(h *dhcpHandler, pkt []byte, addr netip.Addr) []byte {
	id, err := s.serverID(h)
	if err != nil {
		s.withheld(h, err)
		return nil
	}
	return h.buildReply(pkt, msgOffer, addr, id, s.selfAddress(h))
}

// nak answers the DHCPNAK from h, or nil when h cannot name a server
// identifier (serverID).
func (s segment) nak(h *dhcpHandler, pkt []byte) []byte {
	id, err := s.serverID(h)
	if err != nil {
		s.withheld(h, err)
		return nil
	}
	return h.buildNak(pkt, id)
}

// serverID answers the server identifier (option 54) a reply from h carries,
// one rule for every message:
//
//  1. h's server-identifier leaf, when the receiving device holds it. Set and
//     not held, the reply is withheld (errServerIdentifierNotHeld): the leaf
//     names the address the operator wants clients to reach, and no other
//     address is substituted for it in silence.
//  2. Else the receiving device's address inside h's subnet, the device's
//     earliest one when it holds several.
//  3. Else, for a relayed request, the device's primary address: the relay
//     reached the server on this device, so the relayed client reaches the
//     server through the relay at that address.
//
// A request no relay forwarded is served only from subnets the device is
// addressed in (resolveSegment), so the third case is the relay's alone, and
// a device holding no address at all answers errNoServerAddress.
//
// RFC 2131 Section 4.1: "To accommodate potentially incomplete network
// connectivity, a server MUST choose an address as a 'server identifier'
// that, to the best of the server's knowledge, is reachable from the client."
func (s segment) serverID(h *dhcpHandler) (netip.Addr, error) {
	if configured := h.subnet.ServerIdentifier; configured.IsValid() {
		if !slices.Contains(s.device, configured) {
			return netip.Addr{}, errServerIdentifierNotHeld
		}
		return configured, nil
	}
	// RFC 2131 Section 4.1: "For example, if the DHCP server and the DHCP
	// client are connected to the same subnet (i.e., the 'giaddr' field in the
	// message from the client is zero), the server SHOULD select the IP
	// address the server is using for communication on that subnet as the
	// 'server identifier'."
	if self := s.selfAddress(h); self.IsValid() {
		return self, nil
	}
	// RFC 2131 Section 4.1: "If the server has received a message through a
	// DHCP relay agent, the server SHOULD choose an address from the interface
	// on which the message was recieved as the 'server identifier' (unless the
	// server has other, better information on which to make its choice)."
	if s.relayed && len(s.device) > 0 {
		return s.device[0], nil
	}
	return netip.Addr{}, errNoServerAddress
}

// selfAddress answers Ze's first address inside h's subnet on the receiving
// device, which the self class of option 6 hands out (D-6), and the invalid
// address when the device holds none there, as for a request a relay
// forwarded from another link: the self class then offers nothing and the
// list falls through.
func (s segment) selfAddress(h *dhcpHandler) netip.Addr {
	for _, addr := range s.device {
		if h.subnet.Prefix.Contains(addr) {
			return addr
		}
	}
	return netip.Addr{}
}

// withheld records that h answers nothing for err (serverID), naming the
// subnet, the address and the fix.
func (s segment) withheld(h *dhcpHandler, err error) {
	// refuse-by-default: guards handle -- a segment the one-subnet tests build has no log to write to
	if s.warn == nil {
		return
	}
	if errors.Is(err, errServerIdentifierNotHeld) {
		s.warn("dhcpserver: no reply, "+err.Error()+". "+
			"The server answers only from an address clients can reach it at.\n"+
			"Fix: add the server-identifier address to the listen interface, "+
			"or delete the subnet's server-identifier leaf",
			"subnet", h.subnet.Prefix.String(), "server-identifier", h.subnet.ServerIdentifier.String(),
			"addresses", s.device)
		return
	}
	s.warn("dhcpserver: no reply, "+err.Error()+".\n"+
		"Fix: add an address to the listen interface",
		"subnet", h.subnet.Prefix.String(), "addresses", s.device)
}

// serverIDs lists every address serverIDOwner accepts, for the log line that
// names the identifiers a client did not select (logExchange).
func (s segment) serverIDs() []netip.Addr {
	var ids []netip.Addr
	for _, h := range s.subnets {
		if id, err := s.serverID(h); err == nil && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	for _, addr := range s.device {
		if s.subnetOf(addr) != nil && !slices.Contains(ids, addr) {
			ids = append(ids, addr)
		}
	}
	return ids
}

// reservation answers the static mapping for mac on the segment and the subnet
// that holds it, and false when the segment reserves no address for mac.
func (s segment) reservation(mac net.HardwareAddr) (*dhcpHandler, netip.Addr, bool) {
	for _, h := range s.subnets {
		if addr, ok := h.staticByMAC[string(mac)]; ok {
			return h, addr, true
		}
	}
	return nil, netip.Addr{}, false
}

// binding answers the lease mac holds on the segment and its subnet, and false
// when it holds none.
func (s segment) binding(mac net.HardwareAddr) (*dhcpHandler, netip.Addr, bool) {
	for _, h := range s.subnets {
		if l := h.leases.lookup(mac); l != nil {
			return h, l.addr, true
		}
	}
	return nil, netip.Addr{}, false
}

// held answers the address a pool of the segment has offered or bound to mac
// and its subnet, and false when none has.
func (s segment) held(mac net.HardwareAddr) (*dhcpHandler, netip.Addr, bool) {
	for _, h := range s.subnets {
		if addr, ok := h.pool.addrFor(mac); ok {
			return h, addr, true
		}
	}
	return nil, netip.Addr{}, false
}

// heldByOther reports whether a client other than mac is bound to addr, or
// reserved it, on the segment.
func (s segment) heldByOther(addr netip.Addr, mac net.HardwareAddr) bool {
	for _, h := range s.subnets {
		if l := h.leases.lookupByAddr(addr); l != nil && !bytes.Equal(l.mac, mac) {
			return true
		}
		for _, sm := range h.subnet.StaticMappings {
			if sm.IP == addr && !bytes.Equal(sm.MAC, mac) {
				return true
			}
		}
	}
	return false
}

// subnetOf answers the subnet of the segment that contains addr, or nil when
// none does.
func (s segment) subnetOf(addr netip.Addr) *dhcpHandler {
	for _, h := range s.subnets {
		if h.subnet.Prefix.Contains(addr) {
			return h
		}
	}
	return nil
}

// serverIDOwner answers the subnet of the segment that answers a SELECTING
// request naming serverID, or nil when serverID is not the server's on this
// segment. The server's addresses are every identifier serverID gives a subnet
// of the segment (a held server-identifier leaf, the device's address in the
// subnet, the device's primary address for a relayed request) and every
// device address inside a subnet of the segment.
//
// RFC 2131 Section 4.1: "A server with multiple network addresses MUST be
// prepared to to accept any of its network addresses as identifying that
// server in a DHCP message."
func (s segment) serverIDOwner(serverID netip.Addr) *dhcpHandler {
	for _, h := range s.subnets {
		if id, err := s.serverID(h); err == nil && id == serverID {
			return h
		}
	}
	if !slices.Contains(s.device, serverID) {
		return nil
	}
	return s.subnetOf(serverID)
}

// isHostAddress reports whether addr can be a host of prefix: neither its
// network address nor, for a prefix shorter than /31, its broadcast address.
func isHostAddress(prefix netip.Prefix, addr netip.Addr) bool {
	if prefix.Bits() >= 31 {
		return true
	}
	network := addrToUint32(prefix.Masked().Addr())
	broadcast := network | ^uint32(0)>>prefix.Bits()
	a := addrToUint32(addr)
	if a == network {
		return false
	}
	return a != broadcast
}
