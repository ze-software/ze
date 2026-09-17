// Design: docs/architecture/ospf/ospf-12-auth.md -- OSPFv2 authentication key store.
// Related: internal/plugins/ospf/packet -- the Sign/Verify crypto backend.
// RFC: rfc/short/rfc2328.md (App D), rfc/short/rfc5709.md, rfc/short/rfc7474.md

package ospf

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/config/secret"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	"github.com/ze-software/ze/pkg/zefs"
)

// resolvedKey is one usable key with its decoded secret and the AuType its algorithm
// implies (simple -> AuType 1, md5/hmac-sha-* -> AuType 2).
type resolvedKey struct {
	keyID  uint32
	auType packet.AuType
	algo   string
	secret []byte
	// sendStart/sendStop bound the RFC 5709 / RFC 7210 send-lifetime: this key may sign
	// only while now is within [sendStart, sendStop). A zero bound is unbounded.
	sendStart time.Time
	sendStop  time.Time
	// acceptStart/acceptStop bound the RFC 7474 §4 accept-lifetime: a packet signed with
	// this key verifies only while now is within [acceptStart, acceptStop). A zero bound
	// is unbounded, so a key with no configured accept-lifetime verifies at every time.
	acceptStart time.Time
	acceptStop  time.Time
}

// acceptsAt reports whether k may verify a packet received at now.
//
// RFC 7474 Section 4: "For packet reception, the key validity interval as defined by
// AcceptLifetimeStart and AcceptLifetimeEnd must include the current time."
//
// The window is half-open [acceptStart, acceptStop), which is the interval
// lifetimeBounds documents and selectSendKey already applies to the send side.
func (k *resolvedKey) acceptsAt(now time.Time) bool {
	started := k.acceptStart.IsZero() || !now.Before(k.acceptStart)
	notStopped := k.acceptStop.IsZero() || now.Before(k.acceptStop)
	return started && notStopped
}

// replayKey identifies the anti-replay high-water-mark slot. RFC 7474 §2 requires it be
// per OSPF packet type as well as per neighbor and key-id, so legitimately reordered
// packets of different types are not dropped as false replays.
type replayKey struct {
	iface   string
	rid     types.RouterID
	keyID   uint32
	pktType packet.PacketType
}

// authStore resolves per-interface key chains (with area `inherit`), selects the signing
// key, accepts any chain key whose accept-lifetime covers the current time on receive
// (hitless rotation), and enforces the RFC 2328 App D / RFC 7474 non-decreasing
// cryptographic sequence number per neighbor.
type authStore struct {
	mu         sync.Mutex
	chains     map[string][]resolvedKey // interface -> keys (sign with the active one, accept any in its window)
	srcByIface map[string][4]byte       // interface -> IPv4 source address (RFC 7474 Apad bind)
	sendSeq    map[string]uint32        // interface -> per-packet send counter (low-order word)
	recvSeq    map[replayKey]uint64     // last accepted sequence
	// bootCount is the durable RFC 7474 high-order word. Runtime engines MUST
	// initialize it through the daemon before any packet is sent.
	bootCount uint32
	// now is the wall clock used for send-key selection and for the receive-side
	// accept-lifetime gate. It defaults to time.Now and is overridden in tests for
	// deterministic lifetime windows.
	now func() time.Time
}

func newAuthStore() *authStore {
	return &authStore{
		chains:     map[string][]resolvedKey{},
		srcByIface: map[string][4]byte{},
		sendSeq:    map[string]uint32{},
		recvSeq:    map[replayKey]uint64{},
		bootCount:  0,
		now:        time.Now,
	}
}

// setBootCount installs the durably incremented high word. Runtime callers MUST
// finish this before enabling packet processing.
func (s *authStore) setBootCount(bc uint32) {
	s.mu.Lock()
	s.bootCount = bc
	s.mu.Unlock()
}

// stateClient is the daemon RPC surface needed by OSPF. Both internal and external
// plugins use the SDK; protocol-only engines may remain detached from a daemon.
type stateClient interface {
	StateGet(context.Context, string) ([]byte, bool, error)
	StatePut(context.Context, string, []byte) error
	StateIncrement(context.Context, string) (uint32, error)
}

// loadOSPFBootCount requests one atomic durable increment. A failed request
// MUST stop runtime initialization; a clock-derived value cannot preserve order.
func loadOSPFBootCount(ctx context.Context, client stateClient) (uint32, error) {
	if client == nil {
		return 0, errors.New("ospf: persistent state client is unavailable")
	}
	return client.StateIncrement(ctx, zefs.KeyOSPFAuthBootCount.Key())
}

func authAuType(algo string, esn bool) packet.AuType {
	switch {
	case algo == packet.AuthSimple:
		return packet.AuTypeSimple
	case esn:
		return packet.AuTypeCryptographicESN
	default:
		return packet.AuTypeCryptographic
	}
}

// decodeSecret reveals a `$9$`-encoded secret, falling back to the raw value when it is
// not encoded (plaintext config before commit-time encoding).
func decodeSecret(s string) []byte {
	if plain, err := secret.Decode(s); err == nil {
		return []byte(plain)
	}
	return []byte(s)
}

// configure rebuilds the resolved chains from cfg. Per-neighbor replay state and the
// send counters are preserved across a reconfigure (a key rotation must not reset them).
func (s *authStore) configure(cfg ospfConfig) {
	byName := make(map[string]keyChainConfig, len(cfg.KeyChains))
	for _, kc := range cfg.KeyChains {
		byName[kc.Name] = kc
	}
	areaChain := make(map[types.AreaID]string, len(cfg.Areas))
	for _, a := range cfg.Areas {
		areaChain[a.AreaID] = a.AuthKeyChain
	}
	chains := make(map[string][]resolvedKey, len(cfg.Interfaces))
	srcByIface := make(map[string][4]byte, len(cfg.Interfaces))
	for _, ic := range cfg.Interfaces {
		name := ic.Authentication.KeyChain
		if ic.Authentication.Mode == authModeInherit || name == "" {
			name = areaChain[ic.AreaID]
		}
		kc, ok := byName[name]
		if !ok || name == "" {
			continue
		}
		keys := resolveChainKeys(kc)
		if len(keys) > 0 {
			chains[ic.Name] = keys
			// RFC 7474 §5: AuType 3 binds the interface's IPv4 source address into the
			// digest. Capture it here on the config-apply (cold) path so the TX signer
			// hook never makes a per-packet address lookup syscall.
			srcByIface[ic.Name] = interfaceIPv4Address(ic.Name)
		}
	}
	// spec-ospf-ext-7 AC-18: a virtual link inherits its TRANSIT area's authentication with
	// NO synthetic-interface key registration. Its routed sends go out the transit egress
	// interface (signed against that interface's chain, which is the transit area's) and its
	// receives arrive on the transit ifindex (verified against the same interface). The
	// synthetic virtual interface has no OS ifindex, so a name-keyed entry for it would be
	// dead: both signing and verification key on the real transit interface.
	s.mu.Lock()
	s.chains = chains
	s.srcByIface = srcByIface
	s.mu.Unlock()
}

// resolveChainKeys resolves a key chain's keys into the runtime form the store holds,
// carrying both directional windows: the send-lifetime that selects the signing key and
// the RFC 7474 §4 accept-lifetime that gates reception. Lifetimes are validated by
// validateConfig before configure runs, so a parse failure here is impossible; a zero
// window means "always valid" (unset).
func resolveChainKeys(kc keyChainConfig) []resolvedKey {
	keys := make([]resolvedKey, 0, len(kc.Keys))
	for _, k := range kc.Keys {
		sendStart, sendStop, _ := lifetimeBounds(k.SendLifetime)
		acceptStart, acceptStop, _ := lifetimeBounds(k.AcceptLifetime)
		keys = append(keys, resolvedKey{
			keyID:       k.KeyID,
			auType:      authAuType(k.Algorithm, kc.ExtendedSequence),
			algo:        k.Algorithm,
			secret:      decodeSecret(k.Secret),
			sendStart:   sendStart,
			sendStop:    sendStop,
			acceptStart: acceptStart,
			acceptStop:  acceptStop,
		})
	}
	return keys
}

// selectSendKey picks the key that signs at time now. RFC 5709 §X / RFC 7210: the
// active send key is the one whose send-lifetime [sendStart, sendStop) covers now; a
// zero bound is unbounded. Among several active keys the latest-starting one wins
// (the freshest rolled-in key). If NO key is currently active -- every send-lifetime
// has already expired -- the implementation does NOT revert to unauthenticated
// (AuType 0). It keeps signing with the most-recently-starting key so the adjacency
// survives an operator who forgot to refresh the chain; an expired key is far safer
// than dropping authentication. The caller has already checked keys is non-empty.
func selectSendKey(keys []resolvedKey, now time.Time) resolvedKey {
	var (
		active     *resolvedKey
		mostRecent = &keys[0]
	)
	for i := range keys {
		k := &keys[i]
		started := k.sendStart.IsZero() || !now.Before(k.sendStart)
		notStopped := k.sendStop.IsZero() || now.Before(k.sendStop)
		if started && notStopped {
			if active == nil || k.sendStart.After(active.sendStart) {
				active = k
			}
		}
		if k.sendStart.After(mostRecent.sendStart) {
			mostRecent = k
		}
	}
	if active != nil {
		return *active
	}
	// No active key: keep using the most-recently-starting key rather than reverting
	// to no authentication.
	return *mostRecent
}

// signKey returns the active signing key, its AuType, the next cryptographic sequence
// number, and the interface's IPv4 source address (for the AuType 3 Apad bind), or
// ok=false when no chain is resolved (no auth).
func (s *authStore) signKey(iface string) (packet.AuthKey, packet.AuType, uint64, [4]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := s.chains[iface]
	if len(keys) == 0 {
		return packet.AuthKey{}, packet.AuTypeNull, 0, [4]byte{}, false
	}
	k := selectSendKey(keys, s.now())
	var seq uint64
	if k.auType != packet.AuTypeSimple {
		s.sendSeq[iface]++
		c := s.sendSeq[iface]
		if k.auType == packet.AuTypeCryptographicESN {
			// RFC 7474: the 64-bit sequence is boot count (high word) | per-packet counter.
			// When the 32-bit per-packet counter wraps back to 0, advance the boot count so the
			// 64-bit sequence stays strictly increasing across the wrap (it never regresses,
			// which would otherwise look like a replay to the peer). The boot count is re-seeded
			// from a monotonic clock on every restart, so a mid-session bump cannot collide with
			// a later boot.
			if c == 0 {
				s.bootCount++
			}
			seq = uint64(s.bootCount)<<32 | uint64(c)
		} else {
			// RFC 2328 App D: a single 32-bit non-decreasing value, seeded from the boot word
			// so it never regresses across a restart (R-8); only the low word ships. The 32-bit
			// space is inherent to App D -- a session that exhausts it must re-key (use the ESN
			// AuType, which carries the full 64-bit sequence, to avoid the wrap entirely).
			seq = uint64(s.bootCount + c)
		}
	}
	return packet.AuthKey{KeyID: k.keyID, Algorithm: k.algo, Secret: k.secret}, k.auType, seq, s.srcByIface[iface], true
}

// verify authenticates wire received from neighbor rid on iface. src is the IPv4 source
// address from the packet's IP header (bound into the AuType 3 Apad per RFC 7474 §5). It
// returns ("", true) when accepted (including when no auth is configured), or a failure
// reason and false.
//
// Only a key whose accept-lifetime covers the current time is tried, so an operator
// retires a key by closing its window and never has to delete it from the chain. The
// receive side of an expired chain fails CLOSED: no key is tried, no digest is compared,
// and the packet is refused. That is the opposite of selectSendKey, which keeps signing
// with an expired key, and both choices point the same way -- signing with a stale key is
// safer than sending unauthenticated, and refusing a stale key is safer than
// authenticating a neighbor the operator believes is retired.
//
// The "accept-lifetime" reason is reported when the window is what refused the packet:
// the sender named a key this chain holds and that key is outside its window, or no key
// of the chain is inside one. A secret that is genuinely wrong still reports
// "digest-mismatch", so the counter sends the operator to the clock or to the key
// material and never to the wrong one of the two.
func (s *authStore) verify(iface string, rid types.RouterID, src [4]byte, wire []byte) (string, bool) {
	s.mu.Lock()
	keys := s.chains[iface]
	now := s.now()
	s.mu.Unlock()
	if len(keys) == 0 {
		return "", true // auth not configured on this interface
	}
	h, _, err := packet.DecodeHeader(wire)
	if err != nil {
		return "decode", false
	}
	if h.AuType != keys[0].auType {
		return "autype-mismatch", false
	}
	// senderKeyID is the key the packet itself names. AuType 1 names none, so a
	// simple-password chain answers for the chain as a whole and never per key.
	senderKeyID, senderNamedKey := packet.AuthKeyID(h)
	senderKeyRetired := false
	inWindow := 0
	for _, k := range keys {
		// RFC 7474 Section 4: "For packet reception, the key validity interval as
		// defined by AcceptLifetimeStart and AcceptLifetimeEnd must include the current
		// time." A key outside its window is skipped before the digest is computed, so
		// it can neither accept the packet nor record its sequence number.
		if !k.acceptsAt(now) {
			if senderNamedKey && k.keyID == senderKeyID {
				senderKeyRetired = true
			}
			continue
		}
		inWindow++
		seq, ok := packet.Verify(wire, h.AuType, packet.AuthKey{KeyID: k.keyID, Algorithm: k.algo, Secret: k.secret}, src)
		if !ok {
			continue
		}
		if h.AuType == packet.AuTypeSimple {
			return "", true
		}
		rk := replayKey{iface: iface, rid: rid, keyID: k.keyID, pktType: h.Type}
		s.mu.Lock()
		last, seen := s.recvSeq[rk]
		// RFC 7474 §2: the received sequence MUST be strictly greater than the last
		// accepted; an equal sequence is a replay (the send counter increments per packet).
		if seen && seq <= last {
			s.mu.Unlock()
			return "replay", false
		}
		s.recvSeq[rk] = seq
		s.mu.Unlock()
		return "", true
	}
	if senderKeyRetired || inWindow == 0 {
		return "accept-lifetime", false
	}
	if keys[0].auType == packet.AuTypeSimple {
		return "password-mismatch", false
	}
	return "digest-mismatch", false
}

// resetNeighbor clears the cryptographic receive-sequence high-water marks for neighbor
// rid on iface. RFC 2328 Appendix D / RFC 7474 §2: when a neighbor goes Down its recorded
// sequence is forgotten so it may re-establish with any sequence (for example after its
// own restart) without being rejected as a replay.
func (s *authStore) resetNeighbor(iface string, rid types.RouterID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for rk := range s.recvSeq {
		if rk.iface == iface && rk.rid == rid {
			delete(s.recvSeq, rk)
		}
	}
}

// resetInterface clears the cryptographic receive-sequence high-water marks for every
// neighbor on iface, used when the interface itself goes Down and all its adjacencies drop.
func (s *authStore) resetInterface(iface string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for rk := range s.recvSeq {
		if rk.iface == iface {
			delete(s.recvSeq, rk)
		}
	}
}
