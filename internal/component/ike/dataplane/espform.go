// Design: plan/spec-ipsec-esp-dual-form-receive.md -- one Child SA receives both ESP forms
// RFC: rfc/short/rfc7296.md -- receive both ESP forms at any time (Section 2.23)
// RFC: rfc/short/rfc3948.md -- UDP encapsulation of ESP, header and port rules

package dataplane

import (
	"encoding/binary"
	"net/netip"
	"sync"
	"time"
)

// espFormRate and espFormBurst bound how many refused datagrams are re-presented per
// second. Re-presentation is 1:1 and never amplifies, and XFRM still rejects a forged
// payload at the crypto check. The bound exists so an off-path flood aimed at a watched
// SPI cannot spend an unbounded share of the daemon's CPU reaching that check.
const (
	espFormRate  = 5000
	espFormBurst = 10000
)

// espFormTarget is the address pair one watched inbound SA was installed with.
type espFormTarget struct {
	peer  netip.Addr
	local netip.Addr
}

// espFormRegistry holds the SPIs whose inbound XFRM state carries an encapsulation
// template, and is therefore the set whose BARE datagrams the kernel refuses.
//
// Only those SPIs are re-presented. A template-free SA's bare traffic is ACCEPTED by
// XFRM, so re-presenting it would inject a duplicate of every packet the kernel already
// decrypted. Membership is the guard that keeps this path off the fast path.
//
// The zero value is ready for use. Safe for concurrent use.
type espFormRegistry struct {
	mu      sync.Mutex
	watched map[uint32]espFormTarget
}

// watch adds one SPI and reports whether it is the FIRST, so the caller knows to open its
// sockets.
func (r *espFormRegistry) watch(spi uint32, peer, local netip.Addr) (first bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.watched == nil {
		r.watched = make(map[uint32]espFormTarget)
	}
	first = len(r.watched) == 0
	r.watched[spi] = espFormTarget{peer: peer, local: local}
	return first
}

// forget removes one SPI and reports whether the set is now EMPTY, so the caller knows to
// release its sockets. It reports false for an SPI that was not watched, so a repeated
// removal never asks the caller to release sockets twice.
func (r *espFormRegistry) forget(spi uint32) (last bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.watched[spi]; !ok {
		return false
	}
	delete(r.watched, spi)
	return len(r.watched) == 0
}

// forgetAll drops every SPI.
func (r *espFormRegistry) forgetAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.watched = nil
}

// target reports the address pair for a watched SPI, and whether it is watched at all.
func (r *espFormRegistry) target(spi uint32) (espFormTarget, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.watched[spi]
	return t, ok
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
// (rfc/full/rfc7296.txt:3543). A peer that encapsulates ESP therefore runs its IKE on
// 4500, which is the only port pair this re-presentation can legally use.
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

// espFormPacketLen is the byte count writeESPForm needs for the payload given.
func espFormPacketLen(esp []byte) int {
	return espFormHeaderLen + len(esp)
}

// writeESPForm writes one IPv4 datagram carrying esp inside a UDP header into buf, and
// returns the number of bytes written. It reports 0 when the payload is unusable or buf
// is too small, so a caller that ignores the length cannot transmit a partial header.
//
// The datagram re-presents a BARE ESP packet in the UDP-encapsulated form. Linux XFRM
// binds one inbound state to one wire form: net/xfrm/xfrm_input.c compares
// `(x->encap ? x->encap->encap_type : 0) != encap_type` and raises XfrmInStateMismatch
// when they disagree. A state carrying an ESP-in-UDP template therefore refuses bare ESP
// outright, and RFC 7296 Section 2.23 still requires that form to be received. Sending
// the same payload back through the port-4500 socket, which carries UDP_ENCAP_ESPINUDP,
// makes the kernel strip the header again and hand XFRM the encapsulation type its
// template demands.
//
// src is the peer that sent the datagram and dst is the local address it was sent to.
//
// dst IS load-bearing and that is measured: __xfrm_state_lookup keys on destination, SPI,
// protocol and family (net/xfrm/xfrm_state.c), so a wrong destination reaches no state.
//
// src is NOT part of that lookup key, and TestEncapOneStateAcceptsBothForms does not
// discriminate on it: substituting the local address for the peer's still reaches the
// crypto check, measured by mutating this line. The peer's address is preserved anyway
// because it is what the SA was installed with, and because the probes here stop at the
// crypto check with unusable keys and therefore cannot observe any ingress check that
// runs after a successful decryption. That an ingress check exists is NOT claimed here
// (ai/rules/evidence.md).
//
// Both checksum fields stay zero. The IPv4 header checksum is filled by the kernel for
// an IP_HDRINCL socket, and a zero UDP checksum means "not computed" for IPv4
// (RFC 768), which the receiver accepts.
func writeESPForm(buf []byte, src, dst netip.Addr, esp []byte) int {
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
	buf[1] = 0
	binary.BigEndian.PutUint16(buf[2:4], uint16(total))
	binary.BigEndian.PutUint16(buf[4:6], 0) // identification, the kernel supplies one
	binary.BigEndian.PutUint16(buf[6:8], 0) // no fragmentation flags, no offset
	buf[8] = 64                             // TTL: this datagram is delivered locally
	buf[9] = 17                             // UDP
	binary.BigEndian.PutUint16(buf[10:12], 0)
	copy(buf[12:16], srcBytes[:])
	copy(buf[16:20], dstBytes[:])

	// UDP header, both ports 4500 per RFC 3948 Section 2.1.
	binary.BigEndian.PutUint16(buf[20:22], espFormUDPPort)
	binary.BigEndian.PutUint16(buf[22:24], espFormUDPPort)
	binary.BigEndian.PutUint16(buf[24:26], uint16(espFormUDPHeaderLen+len(esp)))
	binary.BigEndian.PutUint16(buf[26:28], 0)

	copy(buf[espFormHeaderLen:], esp)
	return total
}
