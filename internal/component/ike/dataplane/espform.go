// Design: plan/release/validation/spec-ipsec-esp-dual-form-receive.md -- one Child SA receives both ESP forms
// RFC: rfc/short/rfc7296.md -- receive both ESP forms at any time (Section 2.23)
// RFC: rfc/short/rfc3948.md -- UDP encapsulation of ESP, header and port rules

package dataplane

import (
	"cmp"
	"encoding/binary"
	"net/netip"
	"slices"
	"sync"
	"time"
)

// espFormRate and espFormBurst bound how many refused datagrams are re-presented per
// second. Re-presentation is 1:1 and never amplifies, and XFRM still rejects a forged
// payload at the crypto check. The re-presentation runs in userspace, one read from the
// raw ESP socket and one write into ze-espform per datagram, and that cost is what the
// bound protects: an off-path flood aimed at a watched SPI cannot spend an unbounded
// share of the daemon's CPU reaching the crypto check. Only this userspace path is
// bound; the kernel's own receive of the encapsulated form is not.
const (
	espFormRate  = 5000
	espFormBurst = 10000
)

// espFormTarget is the address pair one watched inbound SA was installed with.
type espFormTarget struct {
	peer                netip.Addr
	local               netip.Addr
	peerPort, localPort uint16
}

// espFormRegistry holds the SPIs whose inbound XFRM state carries an encapsulation
// template, and is therefore the set whose BARE datagrams the kernel refuses.
//
// Only those SPIs are re-presented by the complete path, and only those are written into
// the fast path's map (espFormReceiver.watchFastPath). A template-free SA's bare traffic
// is ACCEPTED by XFRM, so re-presenting or rewriting it would hand XFRM a duplicate of, or
// the wrong form for, every packet the kernel already accepts. Membership is the guard
// that keeps that traffic off both paths.
//
// One SPI can be carried by two watched SAs. XFRM keys an inbound state on its
// destination, SPI and protocol (__xfrm_state_lookup), and Ze draws each inbound SPI at
// random with no check against the SPIs already in use (generateESPSPI,
// engine/child.go), so two Child SAs on two local addresses can carry one SPI, and the
// kernel installs both. A registry keyed on the SPI alone would let the second Watch
// replace the first SA's target, so the complete path would serve the second SA alone
// and drop the first SA's bare ESP. So an SA is the pair of its SPI and its local
// address, and the complete path serves each, told apart by the destination address of
// the datagram. One SA per SPI is the rule and two the rare collision, so each SPI holds
// a short list, in the order its SAs were watched.
//
// The zero value is ready for use. Safe for concurrent use. The receiver changes it only
// while it holds its own lock, so a change and the start or stop it implies are one step.
type espFormRegistry struct {
	mu      sync.Mutex
	watched map[uint32][]espFormTarget
}

// watch adds the SA that carries spi to target.local, with target's peer and ports, and
// reports whether it is the FIRST SA watched. An SA already watched on that SPI and local
// address is replaced, ports included. The receiver decides its own start from whether
// its paths are open, read under its lock in the same step as this call, which under
// that lock is the same answer.
func (r *espFormRegistry) watch(spi uint32, target espFormTarget) (first bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.watched == nil {
		r.watched = make(map[uint32][]espFormTarget)
	}
	first = len(r.watched) == 0
	sas := r.watched[spi]
	for i := range sas {
		if sas[i].local == target.local {
			sas[i] = target
			return first
		}
	}
	r.watched[spi] = append(sas, target)
	return first
}

// forget removes the SA that carries spi to local, and reports whether the set is now
// EMPTY, so the caller knows to release its reader socket and its device. Another SA on
// the same SPI and a different local address stays watched. It reports false for an SA
// that was not watched, so a repeated removal never asks the caller to release them
// twice.
func (r *espFormRegistry) forget(spi uint32, local netip.Addr) (last bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	sas := r.watched[spi]
	for i := range sas {
		if sas[i].local != local {
			continue
		}
		sas = slices.Delete(sas, i, i+1)
		if len(sas) == 0 {
			delete(r.watched, spi)
		} else {
			r.watched[spi] = sas
		}
		return len(r.watched) == 0
	}
	return false
}

// forgetAll drops every SPI.
func (r *espFormRegistry) forgetAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.watched = nil
}

// target reports the address pair of the SA that carries spi to local, and whether it
// is watched at all.
func (r *espFormRegistry) target(spi uint32, local netip.Addr) (espFormTarget, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range r.watched[spi] {
		if t.local == local {
			return t, true
		}
	}
	return espFormTarget{}, false
}

// watching reports whether any SA carries spi, on any local address.
func (r *espFormRegistry) watching(spi uint32) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.watched[spi]) > 0
}

// fastTarget reports the SA the fast path serves for spi, the first one watched of those
// that carry it, and whether any does. The fast path's map holds one target per SPI, and
// the program rewrites only a datagram from that target's peer to its local address, so
// the bare ESP of a second SA on the same SPI reaches the complete path, which serves it.
func (r *espFormRegistry) fastTarget(spi uint32) (espFormTarget, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	sas := r.watched[spi]
	if len(sas) == 0 {
		return espFormTarget{}, false
	}
	return sas[0], true
}

// shared lists each watched SA the fast path does not serve because an SA watched before
// it carries the same SPI to another local address (fastTarget keeps the first), sorted
// by SPI and then by that SA's local address. A forget of either SA takes its entry out,
// because forget removes the SA from the list this reads. Nil when no SPI is carried by
// two SAs. The doctor check reads it through ESPFormFastPathState.SharedSPIs.
func (r *espFormRegistry) shared() []ESPFormSharedSPI {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []ESPFormSharedSPI
	for spi, sas := range r.watched {
		for i := 1; i < len(sas); i++ {
			out = append(out, ESPFormSharedSPI{SPI: spi, Served: sas[0].local, Unserved: sas[i].local})
		}
	}
	slices.SortFunc(out, func(a, b ESPFormSharedSPI) int {
		if bySPI := cmp.Compare(a.SPI, b.SPI); bySPI != 0 {
			return bySPI
		}
		return a.Unserved.Compare(b.Unserved)
	})
	return out
}

// espFormLimiter is the token bucket behind espFormRate. The clock is a parameter so the
// bound is testable without sleeping (ai/rules/completion.md).
//
// NOT safe for concurrent use; the caller serializes it.
type espFormLimiter struct {
	tokens   int
	lastFill time.Time
}

func newESPFormLimiter(now time.Time) espFormLimiter {
	return espFormLimiter{tokens: espFormBurst, lastFill: now}
}

// allow reports whether one more datagram may be re-presented.
func (l *espFormLimiter) allow(now time.Time) bool {
	if elapsed := now.Sub(l.lastFill); elapsed >= time.Second {
		l.tokens = espFormBurst
		l.lastFill = now
	} else if fill := int(elapsed.Seconds() * float64(espFormRate)); fill > 0 {
		l.tokens = min(l.tokens+fill, espFormBurst)
		l.lastFill = now
	}
	if l.tokens <= 0 {
		return false
	}
	l.tokens--
	return true
}

// espFormUDPPort is the port RFC 3948 reserves for UDP-encapsulated ESP.
//
// RFC 3948 Section 2.1: "the Source Port and Destination Port MUST be the same as that
// used by IKE traffic", and RFC 7296 Section 2.23 forbids UDP encapsulation on port 500
// (rfc/full/rfc7296.txt:3543). The local listener uses 4500; MOBIKE retargeting also
// carries the translated peer port observed on its authenticated IKE exchange.
const espFormUDPPort = 4500

const (
	espFormIPv4HeaderLen = 20
	espFormUDPHeaderLen  = 8
	espFormHeaderLen     = espFormIPv4HeaderLen + espFormUDPHeaderLen

	// espFormMinESPLen is the shortest ESP payload worth re-presenting: four octets of
	// SPI and four of sequence number (RFC 4303 Section 2). Anything shorter cannot
	// carry an SPI, so it can never match a watched SA.
	espFormMinESPLen = 8

	// espFormMaxESPLen bounds one re-presented payload so the total datagram stays
	// inside the IPv4 total-length field.
	espFormMaxESPLen = 0xFFFF - espFormHeaderLen
)

// espFormSPI reads the Security Parameters Index out of an ESP payload.
//
// RFC 4303 Section 2.1 puts the SPI in the first four octets. It reports ok false for a
// payload too short to carry one, so a truncated datagram can never be read as SPI zero
// and matched against a watched SA (ai/rules/evidence.md).
func espFormSPI(esp []byte) (uint32, bool) {
	if len(esp) < espFormMinESPLen {
		return 0, false
	}
	return binary.BigEndian.Uint32(esp[:4]), true
}

// espFormBareIPv4 splits one datagram the raw ESP socket delivered into the Type of
// Service octet and the destination address its IPv4 header carries and the ESP payload
// after that header. It
// reports ok false for anything that is not a whole IPv4 datagram, so a malformed read
// is never re-presented under a TOS or a payload boundary guessed for it.
//
// The raw socket hands over the IP header: raw_rcv (net/ipv4/raw.c, v7.0 line 320) runs
// `skb_push(skb, -skb_network_offset(skb));` before queueing, and Go's
// (*net.IPConn).ReadMsgIP, unlike ReadFrom, strips nothing. The TOS read here is the octet
// IP_RECVTOS would report, since ip_cmsg_recv_tos (net/ipv4/ip_sockglue.c) answers
// `put_cmsg(msg, SOL_IP, IP_TOS, 1, &ip_hdr(skb)->tos);` from the same header.
//
// For a datagram the kernel reassembled, that header is the reassembled one, whose ECN
// field merges every fragment's: ip_frag_queue ORs `ip4_frag_ecn(ip_hdr(skb)->tos)` of
// each fragment into qp->ecn, and ip_frag_reasm (net/ipv4/ip_fragment.c, v7.0) writes
// `iph->tos |= ecn;` into the head fragment's header, with
// `ecn = ip_frag_ecn_table[qp->ecn];`. The table (net/ipv4/inet_fragment.c) answers CE
// when one fragment carried CE and the others an ECN-capable codepoint (0b10 or 0b01).
// It answers 0, leaving the head's own field, for every other set that holds no
// not-ECN-capable fragment (0b00) beside an ECN-capable one: all fragments alike, CE
// included, or 0b10 beside 0b01. A not-ECN-capable fragment mixed with ECN-capable ones
// is an 0xff entry, and ip_frag_reasm then drops the whole datagram (`goto out_fail`). So a CE mark on any fragment of a delivered datagram reaches this
// reader, and through it XFRM.
func espFormBareIPv4(datagram []byte) (tos uint8, dst netip.Addr, esp []byte, ok bool) {
	if len(datagram) < espFormIPv4HeaderLen {
		return 0, netip.Addr{}, nil, false
	}
	if datagram[0]>>4 != 4 {
		return 0, netip.Addr{}, nil, false
	}
	// RFC 791 Section 3.1: "Internet Header Length is the length of the internet
	// header in 32 bit words, and thus points to the beginning of the data."
	headerLen := int(datagram[0]&0x0F) * 4
	if headerLen < espFormIPv4HeaderLen {
		return 0, netip.Addr{}, nil, false
	}
	totalLen := int(binary.BigEndian.Uint16(datagram[2:4]))
	if totalLen < headerLen {
		return 0, netip.Addr{}, nil, false
	}
	if totalLen > len(datagram) {
		return 0, netip.Addr{}, nil, false
	}
	// RFC 791 Section 3.1: the Destination Address sits at octets 16 to 19 of the
	// fixed header, which the length check above covers.
	dst = netip.AddrFrom4([4]byte(datagram[16:20]))
	return datagram[1], dst, datagram[headerLen:totalLen], true
}

// espFormPacketLen is the byte count writeESPForm needs for the payload given.
func espFormPacketLen(esp []byte) int {
	return espFormHeaderLen + len(esp)
}

// writeESPForm writes one IPv4 datagram carrying esp inside a UDP header into buf, and
// returns the number of bytes written. It reports 0 when the payload is unusable or buf
// is too small, so a caller that ignores the length cannot transmit a partial header.
//
// The datagram re-presents a BARE ESP packet in the UDP-encapsulated form. A Linux XFRM
// inbound state receives one wire form (Linux limitation LL-XFRM-STATE-ONE-ESP-FORM,
// docs/architecture/linux-limitations.md), so a state carrying an ESP-in-UDP template
// refuses bare ESP, and RFC 7296 Section 2.23 still requires that form to be received. Writing
// the same payload into ze-espform, addressed to the port-4500 socket, which carries
// UDP_ENCAP_ESPINUDP, makes the kernel strip the header again and hand XFRM the
// encapsulation type its template demands.
//
// src is the peer that sent the datagram and dst is the local address it was sent to.
// tos is the Type of Service octet the bare datagram arrived with (espFormBareIPv4), DSCP
// and ECN field both, and the rebuilt header carries it unchanged.
//
// tos IS load-bearing. RFC 7296 Section 2.24: decapsulators "MUST implement the tunnel
// encapsulation and decapsulation processing specified in [IPSECARCH] to prevent
// discarding of ECN congestion indications." XFRM does that processing from the outer
// header it decapsulates, which for this path is the header built here:
// xfrm4_extract_header (net/xfrm/xfrm_inout.h, v7.0) records
// `XFRM_MODE_SKB_CB(skb)->tos = iph->tos;`, and ipip_ecn_decapsulate
// (net/xfrm/xfrm_input.c, v7.0 lines 229 to 235) runs
// `if (INET_ECN_is_ce(XFRM_MODE_SKB_CB(skb)->tos)) IP_ECN_set_ce(inner_iph);`. A header
// written with TOS 0 would discard every CE mark the peer's path set on the outer header.
//
// dst IS load-bearing and that is measured: __xfrm_state_lookup keys on destination, SPI,
// protocol and family (net/xfrm/xfrm_state.c), so a wrong destination reaches no state.
//
// src is NOT part of that lookup key, and TestEncapOneStateAcceptsBothForms does not
// discriminate on it: substituting the local address for the peer's still reaches the
// crypto check, measured on that loopback probe by mutating this line. The peer's address
// is carried anyway, because it is what the SA was installed with and the stack reads it
// twice on the way in: ze-espform's reverse-path filter needs a source it can route, and
// esp_input_done2 (net/ipv4/esp4.c) compares the outer source with the state's and reports
// a changed NAT mapping (km_new_mapping) when they differ.
//
// The IPv4 header checksum is computed here, because a datagram written into a TUN is
// received as it stands and ip_rcv_core drops a header whose checksum is wrong; nothing
// fills it on the way. The UDP checksum stays zero, which means "not computed" for IPv4:
// RFC 3948 Section 2.1, "the IPv4 UDP Checksum SHOULD be transmitted as a zero value".
func writeESPForm(buf []byte, src, dst netip.Addr, tos uint8, esp []byte) int {
	if len(esp) < espFormMinESPLen || len(esp) > espFormMaxESPLen {
		return 0
	}
	if !src.Is4() || !dst.Is4() {
		return 0
	}
	total := espFormPacketLen(esp)
	if len(buf) < total {
		return 0
	}

	srcBytes, dstBytes := src.As4(), dst.As4()

	// IPv4 header. Version 4, five 32-bit words of header, no options.
	buf[0] = 0x45
	// RFC 7296 Section 2.24: the received ECN field reaches XFRM's decapsulation, which
	// copies a CE mark into the inner header. The checksum below covers this octet.
	buf[1] = tos
	binary.BigEndian.PutUint16(buf[2:4], uint16(total))
	binary.BigEndian.PutUint16(buf[4:6], 0) // identification: never fragmented, never leaves the host
	binary.BigEndian.PutUint16(buf[6:8], 0) // no fragmentation flags, no offset
	buf[8] = 64                             // TTL: this datagram is delivered locally
	buf[9] = 17                             // UDP
	binary.BigEndian.PutUint16(buf[10:12], 0)
	copy(buf[12:16], srcBytes[:])
	copy(buf[16:20], dstBytes[:])
	binary.BigEndian.PutUint16(buf[10:12], espFormIPv4Checksum(buf[:espFormIPv4HeaderLen]))

	// UDP header, both ports 4500 per RFC 3948 Section 2.1.
	binary.BigEndian.PutUint16(buf[20:22], espFormUDPPort)
	binary.BigEndian.PutUint16(buf[22:24], espFormUDPPort)
	binary.BigEndian.PutUint16(buf[24:26], uint16(espFormUDPHeaderLen+len(esp)))
	binary.BigEndian.PutUint16(buf[26:28], 0)

	copy(buf[espFormHeaderLen:], esp)
	return total
}

// espFormIPv4Checksum computes the checksum of one IPv4 header whose checksum field is
// zero. The header length is even (a whole number of 32-bit words), so every octet
// belongs to a 16-bit word.
//
// RFC 791 Section 3.1: "The checksum field is the 16 bit one's complement of the one's
// complement sum of all 16 bit words in the header.  For purposes of computing the
// checksum, the value of the checksum field is zero."
func espFormIPv4Checksum(header []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(header); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(header[i : i+2]))
	}
	// Fold the carries back in until the sum fits 16 bits: the one's complement sum.
	for sum > 0xFFFF {
		sum = (sum & 0xFFFF) + (sum >> 16)
	}
	return ^uint16(sum)
}
