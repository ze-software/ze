// Design: plan/learned/740-ipsec-7-ikev2-engine.md -- responder COOKIE challenge
// RFC: rfc/short/rfc7296.md -- COOKIE (Sections 2.6, 2.6.1)
// Related: register.go -- tryResponderSAInit, where the challenge gates the half-open slot
// Related: sa_init_retry.go -- the initiator half that echoes a received cookie
//
// RFC 7296 Section 2.6 states the attack this file answers.
//
// "Two expected attacks against IKE are state and CPU exhaustion, where the target is
// flooded with session initiation requests from forged IP addresses. These attacks can
// be made less effective if a responder uses minimal CPU and commits no state to an SA
// until it knows the initiator can receive packets at the address from which it claims
// to be sending them."
//
// Everything here therefore runs BEFORE any state is committed, allocates nothing per
// payload, and remembers nothing about the sender. The cookie is recomputed on receipt
// rather than stored, so a flood costs this node one HMAC and no memory.
package engine

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/ike/wire"
)

// RFC 7296 Section 2.6 MUST: "The data associated with this notification MUST be
// between 1 and 64 octets in length (inclusive)".
//
// The bound governs two paths, not one. It bounds what this node MINTS, and it bounds
// what this node ECHOES after a peer sends one (sa_init_retry.go). A peer that sends a
// 600-octet COOKIE must not have it reflected.
const (
	minCookieLen = 1
	maxCookieLen = 64
)

// cookieDataLen is the length of a cookie this node mints: one version octet plus a
// SHA-256 digest. It sits inside the 1..64 bound above, and sendCookieChallenge
// re-checks that bound rather than trusting this constant.
const cookieDataLen = 1 + sha256.Size

// cookieRotateInterval bounds the life of one cookie secret.
//
// RFC 7296 Section 2.6: "The responder should change the value of <secret> frequently,
// especially if under attack." A cookie minted under the previous secret stays valid
// for one further interval, which the same section permits: the responder "MAY keep the
// old value of <secret> around for a short time and accept cookies computed from either
// one". A conforming initiator answers a challenge within one round trip, so one
// interval is generous.
const cookieRotateInterval = 60 * time.Second

// maxPreStatePayloads bounds the payload chain scanSAInitPreState walks.
//
// The chain arrives unauthenticated and before any state exists, so the walk is itself
// a CPU-exhaustion surface. RFC 7296 puts SA, KE, Nonce and a handful of notifies in an
// IKE_SA_INIT; 32 is far above any conforming message and far below a chain built to
// spin this loop.
const maxPreStatePayloads = 32

// cookieThreshold is the number of half-open IKE SAs this responder tolerates before it
// challenges an inbound initiation.
//
// It is monotone and carries no magic value. Zero, the default, means every inbound
// initiation is challenged: at zero half-open SAs the count already meets the
// threshold. A larger value tolerates that many half-open handshakes first.
//
// Zero is the default because the defect the challenge closes is reachable at zero
// half-open SAs. One spoofed datagram bearing a configured peer's source address takes
// that peer's only half-open slot (responderBusy, reconcile.go) for
// responderHandshakeTimeout, and a datagram every 30 seconds denies that peer
// indefinitely. A threshold of one would let that first datagram through unchallenged
// and leave the defect open, so the cost -- one extra round trip on every inbound
// handshake -- is paid on the first initiation.
//
// Written by the ike plugin's OnConfigure callback, read on the dispatch goroutine.
var cookieThreshold atomic.Uint32

// SetCookieThreshold publishes the configured half-open tolerance.
func SetCookieThreshold(n uint32) { cookieThreshold.Store(n) }

// CookieThreshold reports the half-open tolerance currently in force.
func CookieThreshold() uint32 { return cookieThreshold.Load() }

// cookieSecret is the rotating key a cookie is computed under.
//
// RFC 7296 Section 2.6 leaves the construction free: "The exact algorithms and syntax
// used to generate cookies do not affect interoperability and hence are not specified
// here." The secret is generated here and never derives from anything a peer supplies,
// so no peer can predict or steer a cookie it has not been given.
type cookieSecret struct {
	mu            sync.Mutex
	current       [sha256.Size]byte
	previous      [sha256.Size]byte
	version       uint8
	previousValid bool
	rotatedAt     time.Time
}

// responderCookies holds the process-wide cookie secret. One responder, one secret: the
// cookie proves the sender receives at its claimed address, which is a property of this
// node rather than of a peer.
var responderCookies cookieSecret

// ensureLocked initializes the secret on first use and rotates it once the interval has
// elapsed. The caller holds s.mu.
//
// It reports failure rather than continuing with a zero key. A zero secret would let
// any peer that guessed it mint a valid cookie, which is the zero-value trap
// (ai/rules/fail-closed-guards.md).
func (s *cookieSecret) ensureLocked(now time.Time) bool {
	if s.rotatedAt.IsZero() {
		if _, err := rand.Read(s.current[:]); err != nil {
			return false
		}
		s.rotatedAt = now
		return true
	}
	if now.Sub(s.rotatedAt) < cookieRotateInterval {
		return true
	}
	var fresh [sha256.Size]byte
	if _, err := rand.Read(fresh[:]); err != nil {
		// The current secret is still sound, so keep serving under it. Refusing every
		// handshake because one read of the entropy source failed would turn a
		// transient fault into the outage the cookie exists to prevent.
		return true
	}
	s.previous = s.current
	s.previousValid = true
	s.current = fresh
	s.version++
	s.rotatedAt = now
	return true
}

// cookieMAC computes the body of a cookie under one secret, or reports that it cannot.
//
// RFC 7296 Section 2.6 gives the worked example
// "Cookie = <VersionIDofSecret> | Hash(Ni | IPi | SPIi | <secret>)"
// and explains each input: the nonce ties the cookie to one exchange, the address is
// what the cookie proves reachability at, and the initiator SPI ties it to one
// initiation.
//
// This node computes an HMAC rather than a hash over a concatenation that ends in the
// secret. Hash(data | secret) with a Merkle-Damgard hash is length-extension shaped,
// and the section quoted above frees the choice of algorithm.
//
// It fails closed on a nil address and an empty nonce, because a cookie computed over
// nothing would verify for every sender.
func cookieMAC(secret *[sha256.Size]byte, version uint8, spiI [8]byte, ni []byte, ip net.IP) ([]byte, bool) {
	ip16 := ip.To16()
	if ip16 == nil || len(ni) == 0 {
		return nil, false
	}
	mac := hmac.New(sha256.New, secret[:])
	mac.Write(ni)
	mac.Write(ip16)
	mac.Write(spiI[:])
	// The version octet leads, and Sum appends the digest after it, so the result is
	// exactly cookieDataLen with no second allocation.
	out := make([]byte, 1, cookieDataLen)
	out[0] = version
	return mac.Sum(out), true
}

// mintCookie builds the cookie this node challenges an initiation with, or nil when it
// cannot be computed.
//
// The result is cookieDataLen octets, inside the 1..64 bound RFC 7296 Section 2.6
// requires. Its caller re-checks that bound rather than trusting this function.
func mintCookie(spiI [8]byte, ni []byte, ip net.IP, now time.Time) []byte {
	responderCookies.mu.Lock()
	defer responderCookies.mu.Unlock()
	if !responderCookies.ensureLocked(now) {
		return nil
	}
	data, ok := cookieMAC(&responderCookies.current, responderCookies.version, spiI, ni, ip)
	if !ok {
		return nil
	}
	return data
}

// verifyCookie reports whether data is a cookie this node minted for this initiator,
// this nonce and this address.
//
// It FAILS CLOSED, and that has to be reconciled with RFC 7296 Section 2.6, which
// requires a mismatched cookie to be IGNORED rather than rejected:
//
// "When one party receives an IKE_SA_INIT request containing a cookie whose contents do
// not match the value expected, that party MUST ignore the cookie and process the
// message as if no cookie had been included; usually this means sending a response
// containing a new cookie."
//
// The two are not in tension, because there are two guards and neither one grants a
// resource on doubt. THIS function CLASSIFIES, and it denies on every miss: empty data,
// a wrong length, an unknown secret version, an expired previous secret, a nil address,
// an empty nonce. cookieRequired decides whether a cookie is demanded at all, and it
// also denies on doubt. The ACTION taken on a false verdict is the RFC's: the message is
// not rejected, it is processed as a cookie-less IKE_SA_INIT, and a pressured responder
// processes a cookie-less initiation by issuing a new cookie. No half-open slot is
// taken. The fail-open therefore lives in the CLASSIFICATION and never in the resource
// decision.
//
// A future reader will want to "harden" the caller into a rejection. That would break
// conformance with the paragraph quoted above, and
// TestCkeMismatchedCookieIsIgnoredNotRejected exists to stop it.
func verifyCookie(data []byte, spiI [8]byte, ni []byte, ip net.IP, now time.Time) bool {
	if len(data) != cookieDataLen {
		return false
	}
	responderCookies.mu.Lock()
	defer responderCookies.mu.Unlock()
	if responderCookies.rotatedAt.IsZero() {
		// Nothing has been minted yet, so no cookie can be ours.
		return false
	}
	version := data[0]
	var secret *[sha256.Size]byte
	switch {
	case version == responderCookies.version:
		secret = &responderCookies.current
	case responderCookies.previousValid && version == responderCookies.version-1 &&
		now.Sub(responderCookies.rotatedAt) < cookieRotateInterval:
		secret = &responderCookies.previous
	default:
		return false
	}
	want, ok := cookieMAC(secret, version, spiI, ni, ip)
	if !ok {
		return false
	}
	return hmac.Equal(want, data)
}

// halfOpenResponderCount reports how many responder peers hold a half-open handshake
// slot right now.
//
// It is DERIVED from responderBusy rather than kept as a second counter beside it
// (ai/rules/derive-not-hardcode.md). responderBusy is set at the CAS in
// tryResponderSAInit and cleared at six sites across three files; a parallel counter
// would have to be decremented at every one of them, and the one that was missed would
// leave this node challenging every handshake forever.
func halfOpenResponderCount() int {
	peersMu.RLock()
	defer peersMu.RUnlock()
	n := 0
	for _, ps := range activePeersMap {
		if ps != nil && ps.responderBusy.Load() {
			n++
		}
	}
	return n
}

// cookieRequired reports whether an inbound initiation must answer a cookie challenge
// before it may take a half-open slot.
//
// RFC 7296 Section 2.6: "When a responder detects a large number of half-open IKE SAs,
// it SHOULD reply to IKE_SA_INIT requests with a response containing the COOKIE
// notification." The operator sets what "large" means through cookie-threshold, whose
// default of zero challenges every initiation.
//
// It fails closed. A nil session reads true, because demanding a cookie costs a
// conforming peer one round trip and costs an attacker the whole attack.
func cookieRequired(ps *PeerSession) bool {
	if ps == nil {
		return true
	}
	count := halfOpenResponderCount()
	if count < 0 {
		return true
	}
	return uint64(count) >= uint64(cookieThreshold.Load())
}

// cookieRemoteIP reads the address a cookie is bound to out of a packet source.
//
// It fails closed. A nil source yields a nil address, and cookieMAC refuses a nil
// address, so a packet with no readable source can never mint or verify a cookie
// (ai/rules/fail-closed-guards.md).
func cookieRemoteIP(remote *net.UDPAddr) net.IP {
	if remote == nil {
		return nil
	}
	return remote.IP
}

// scanSAInitPreState reads the first COOKIE notify and the Nonce out of a raw
// IKE_SA_INIT datagram, without parsing it.
//
// wire.Message.ReadFrom is the wrong tool on this path. It allocates a payload object
// per payload and a data slice per notify, and it runs on unauthenticated input before
// any state exists -- the CPU exhaustion this whole file prevents. This walker reads the
// generic payload headers over the raw slice and returns sub-slices of it.
//
// Every bound denies rather than continues. ok reports that the chain parsed cleanly; a
// cookie outside the 1..64 octets RFC 7296 Section 2.6 allows is a malformed chain here,
// never a value to truncate (ai/rules/exact-or-reject.md).
func scanSAInitPreState(data []byte) (cookie, nonce []byte, ok bool) {
	if len(data) < wire.HeaderLen {
		return nil, nil, false
	}
	end := int(binary.BigEndian.Uint32(data[24:28]))
	if end < wire.HeaderLen || end > len(data) {
		return nil, nil, false
	}
	next := data[16]
	off := wire.HeaderLen
	for seen := 0; next != 0 && off < end; seen++ {
		if seen >= maxPreStatePayloads {
			return nil, nil, false
		}
		if off+wire.GenericHeaderLen > end {
			return nil, nil, false
		}
		length := int(binary.BigEndian.Uint16(data[off+2 : off+4]))
		if length < wire.GenericHeaderLen {
			return nil, nil, false
		}
		payloadEnd := off + length
		// The strict advance is implied by the length check above, and it is stated
		// again because this loop runs on attacker-chosen input: a chain that fails to
		// advance is an unbounded CPU loop reachable by one datagram.
		if payloadEnd > end || payloadEnd <= off {
			return nil, nil, false
		}
		body := data[off+wire.GenericHeaderLen : payloadEnd]
		switch next {
		case wire.PayloadTypeNotify:
			if c, isCookie := preStateCookie(body); isCookie {
				if len(c) < minCookieLen || len(c) > maxCookieLen {
					return nil, nil, false
				}
				if cookie == nil {
					cookie = c
				}
			}
		case wire.PayloadTypeNonce:
			if nonce == nil && len(body) > 0 {
				nonce = body
			}
		}
		next = data[off]
		off = payloadEnd
	}
	return cookie, nonce, true
}

// preStateCookie reads the Notification Data of a notify body when that body is a
// COOKIE. The body layout is RFC 7296 Section 3.10: Protocol ID, SPI Size, Notify
// Message Type, SPI, Notification Data.
func preStateCookie(body []byte) ([]byte, bool) {
	if len(body) < 4 {
		return nil, false
	}
	if binary.BigEndian.Uint16(body[2:4]) != wire.NotifyCookie {
		return nil, false
	}
	off := 4 + int(body[1])
	if off > len(body) {
		return nil, false
	}
	return body[off:], true
}
