// Design: docs/architecture/ike/ipsec-14-responder.md -- the authenticator's ticket key
// Detail: docs/architecture/ike/ipsec-11-interop-eap.md -- the peer's ticket cache
// RFC: rfc/short/rfc9190.md -- Sections 2.1.2, 2.1.3 and 5.7

package eap

import (
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"sync"
	"time"
)

// resumptionTicketMaxAge is the ceiling RFC 9190 Section 5.7 puts on how long an
// EAP-TLS peer keeps what a resumption needs.
//
// RFC 9190 Section 5.7: "EAP-TLS peers MUST NOT store resumption PSKs or tickets
// (and associated cached data) for longer than 604800 seconds (7 days)
// regardless of the PSK or ticket lifetime."
//
// crypto/tls REFUSES an entry older than the lifetime the ticket declares
// (Conn.loadSession, crypto/tls/handshake_client.go, against SessionState.useBy)
// but it never drops one, and NewLRUClientSessionCache holds whatever it was
// given until another key evicts it. The obligation is about STORING, so it is
// this cache that has to answer it (Resumption.Put and Resumption.Get below).
const resumptionTicketMaxAge = 604800 * time.Second

// resumptionKeyRotation and resumptionKeyMaxAge reproduce the schedule Go
// applies to the ticket keys it manages itself (ticketKeyRotation and
// ticketKeyLifetime, crypto/tls/common.go).
//
// Setting an explicit key with Config.SetSessionTicketKeys TURNS OFF that
// rotation: "Calling this function will turn off automatic session ticket key
// rotation." A single key held for the life of a peer's configuration would
// therefore protect every certificate chain that peering ever cached, so this
// schedule is what keeps the stdlib property Ze displaces rather than an extra
// mechanism on top of it.
//
// The retired key is kept for decryption alone. crypto/tls encrypts under the
// FIRST key and tries all of them to decrypt (Config.SetSessionTicketKeys), and
// it refuses a ticket older than maxSessionTicketLifetime, which is the same 7
// days. So no key outlives the tickets it protects, and a ticket minted the
// instant before a rotation is still redeemable for its whole life.
const (
	resumptionKeyRotation = 24 * time.Hour
	resumptionKeyMaxAge   = 7 * 24 * time.Hour
)

// resumptionMaxCachedSessions bounds the tickets one peering holds at once.
//
// A ze EAP-TLS peer sets no ServerName, so every session it stores lands under
// one cache key (Conn.clientSessionCacheKey, crypto/tls/handshake_client.go,
// falls back to the transport's remote address, which eapAddr.String reports as
// the constant "eap"). One entry is therefore the live count, and the cap exists
// so the map cannot grow without a bound if that key ever varies.
const resumptionMaxCachedSessions = 8

// Resumption holds the TLS 1.3 session-resumption state ONE EAP-TLS peering
// keeps across every exchange with that peer. It carries both roles' halves,
// because one operator setting governs both.
//
// THE CACHE MUST BE PER PEER, AND THAT IS A SECURITY REQUIREMENT RATHER THAN A
// PREFERENCE. Conn.clientSessionCacheKey keys on ServerName and falls back to
// the transport's remote address when there is none. EAP-TLS carries no server
// hostname, so ze sets none, and eapTLSTransport.RemoteAddr answers eapAddr{},
// whose String is the constant "eap" (eap_tls.go). One process-wide cache would
// therefore file every peering's ticket under one key and offer peer A's ticket,
// with peer A's cached certificate chain, to peer B.
//
// It implements tls.ClientSessionCache for the peer role. The server role reads
// TicketKeys.
//
// Every method is safe for concurrent use: crypto/tls calls Get and Put from the
// handshake goroutine, which is not the goroutine that built the config.
type Resumption struct {
	// now reads the clock. It is a field so a test can drive the Section 5.7
	// ceiling without waiting seven days for it.
	now func() time.Time

	// enabled is the operator's session-resumption leaf. It gates OFFERING a
	// ticket as the peer and ACCEPTING one as the authenticator, which RFC 9190
	// Section 2.1.3 leaves to each side ("It is up to the EAP-TLS peer to use
	// resumption", and "the EAP-TLS server MAY choose to require a full
	// handshake"). It NEVER gates issuance: Section 2.1.2 makes that a MUST with
	// no condition on it.
	enabled bool

	mu sync.Mutex
	// keys are the session-ticket keys, newest first. rotateKeys maintains that
	// order, and crypto/tls encrypts under keys[0].
	keys []resumptionKey
	// cached holds the peer role's tickets by crypto/tls cache key.
	cached map[string]resumptionEntry
}

// resumptionKey is one session-ticket key and the moment it was minted.
type resumptionKey struct {
	key     [32]byte
	created time.Time
}

// resumptionEntry is one cached ticket and the moment this cache accepted it.
//
// stored is when ZE took the ticket, not when the authenticator minted it, which
// is the quantity RFC 9190 Section 5.7 bounds: it limits how long the peer keeps
// the material, "regardless of the PSK or ticket lifetime".
type resumptionEntry struct {
	state  *tls.ClientSessionState
	stored time.Time
}

// NewResumption builds the resumption state for one peering.
//
// now MUST NOT be nil; a caller with nothing better passes time.Now.
func NewResumption(now func() time.Time, enabled bool) *Resumption {
	return &Resumption{
		now:     now,
		enabled: enabled,
		cached:  map[string]resumptionEntry{},
	}
}

// Enabled reports whether this peering may accept a resumed session as the
// authenticator and offer a ticket as the peer.
func (r *Resumption) Enabled() bool { return r.enabled }

// TicketKeys answers the session-ticket keys the authenticator encrypts a
// NewSessionTicket under, newest first, rotating them when the newest is due.
//
// It is called on every EAP-TLS exchange, which is what drives the rotation: the
// schedule needs no timer and no goroutine of its own.
func (r *Resumption) TicketKeys() ([][32]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.rotateKeys(r.now()); err != nil {
		return nil, err
	}
	keys := make([][32]byte, 0, len(r.keys))
	for _, k := range r.keys {
		keys = append(keys, k.key)
	}
	return keys, nil
}

// rotateKeys mints a new encryption key when the newest one is due, and retires
// every key past resumptionKeyMaxAge. The caller holds r.mu.
//
// A failed mint is an ERROR rather than a reused key or an empty set: a session
// ticket encrypted under a key the operator cannot rely on is worse than no
// ticket, and an empty set would panic Config.SetSessionTicketKeys
// (ai/rules/principles.md).
func (r *Resumption) rotateKeys(now time.Time) error {
	if len(r.keys) > 0 && now.Sub(r.keys[0].created) < resumptionKeyRotation {
		return nil
	}

	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		return fmt.Errorf("eap-tls: mint a session ticket key: %w", err)
	}

	kept := make([]resumptionKey, 0, len(r.keys)+1)
	kept = append(kept, resumptionKey{key: key, created: now})
	for _, k := range r.keys {
		if now.Sub(k.created) < resumptionKeyMaxAge {
			kept = append(kept, k)
		}
	}
	r.keys = kept
	return nil
}

// ClientCache answers the tls.Config.ClientSessionCache the peer role uses, and
// nil when the operator turned resumption off for this peering.
//
// A nil cache is what stops the peer OFFERING a ticket: Conn.loadSession
// (crypto/tls/handshake_client.go) returns before it writes psk_key_exchange_modes
// when there is none, so the ClientHello carries no PSK and no resumption is
// possible. The literal nil matters, because a nil *Resumption inside a non-nil
// interface would pass crypto/tls's check and then be called.
func (r *Resumption) ClientCache() tls.ClientSessionCache {
	if !r.enabled {
		return nil
	}
	return r
}

// Put stores a ticket the authenticator issued, and drops every entry this cache
// has held for longer than RFC 9190 Section 5.7 permits.
//
// A nil state is the removal the tls.ClientSessionCache contract defines, and
// crypto/tls sends one when it finds a cached chain it can no longer verify.
func (r *Resumption) Put(key string, state *tls.ClientSessionState) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now()
	r.dropExpired(now)

	if state == nil {
		delete(r.cached, key)
		return
	}

	// Make room before the insert, so the cap is a ceiling on what is held
	// rather than on what is held after the next expiry sweep.
	if _, replacing := r.cached[key]; !replacing {
		r.dropOldestOver(resumptionMaxCachedSessions - 1)
	}
	r.cached[key] = resumptionEntry{state: state, stored: now}
}

// Get answers a stored ticket, and refuses one this cache has held for longer
// than RFC 9190 Section 5.7 permits.
//
// The expired entry is DELETED rather than merely withheld. Withholding it would
// satisfy the sentence about using a ticket and not the one that is written,
// which is about storing it.
func (r *Resumption) Get(key string) (*tls.ClientSessionState, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.dropExpired(r.now())
	entry, ok := r.cached[key]
	if !ok {
		return nil, false
	}
	return entry.state, true
}

// dropExpired deletes every entry past the Section 5.7 ceiling. The caller holds
// r.mu.
func (r *Resumption) dropExpired(now time.Time) {
	for key, entry := range r.cached {
		if now.Sub(entry.stored) >= resumptionTicketMaxAge {
			delete(r.cached, key)
		}
	}
}

// dropOldestOver deletes the oldest entries until at most limit remain. The
// caller holds r.mu.
//
// The loop is bounded by the map, and each pass removes one entry, so it runs at
// most len(r.cached) times.
func (r *Resumption) dropOldestOver(limit int) {
	for len(r.cached) > limit {
		oldestKey := ""
		var oldest time.Time
		for key, entry := range r.cached {
			if oldestKey == "" || entry.stored.Before(oldest) {
				oldestKey, oldest = key, entry.stored
			}
		}
		delete(r.cached, oldestKey)
	}
}

// peerSessionCache answers the tls.Config.ClientSessionCache for one peering,
// and nil for a peer configured with no resumption state at all.
//
// A PeerTLSConfig carries none when the caller built it before resumption
// existed, which every test harness that drives the peer directly does. Such a
// peer offers no ticket and runs a full handshake every time, which is
// conformant: RFC 9190 Section 2.1.3 leaves using resumption to the peer.
func peerSessionCache(r *Resumption) tls.ClientSessionCache {
	if r == nil {
		return nil
	}
	return r.ClientCache()
}

// refuseResumption is the tls.Config.UnwrapSession an authenticator installs
// when the operator turned resumption off for this peering.
//
// It answers (nil, nil), which crypto/tls reads as "this identity names no
// session" and turns into a full handshake (serverHandshakeStateTLS13.
// checkForResumption, crypto/tls/handshake_server_tls13.go, continues to the
// next identity and then falls through). An ERROR there would terminate the
// connection with a fatal alert instead, and RFC 9190 Section 5.7 asks for the
// full handshake: "the EAP-TLS server MAY choose to require a full handshake".
func refuseResumption(_ []byte, _ tls.ConnectionState) (*tls.SessionState, error) {
	//nolint:nilnil // crypto/tls defines (nil, nil) as "this identity names no session"
	return nil, nil
}
