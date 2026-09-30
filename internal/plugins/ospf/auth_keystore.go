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
	// nextBootCount advances the durable high word on low-word wrap. Runtime
	// initialization MUST install it together with bootCount.
	nextBootCount func() (uint32, error)
	// now is the wall clock used for send-key selection and for the receive-side
	// accept-lifetime gate. It defaults to time.Now and is overridden in tests for
	// deterministic lifetime windows.
	now func() time.Time
	// lastKeyNoticed records the interfaces already notified that their last key expired
	// (RFC 5709 Section 3.2), so the notification is sent once per chain and not once per
	// packet. configure clears it with the chain it describes.
	lastKeyNoticed map[string]bool
	// sink is the operator event bus the notification goes to; nil when the engine runs
	// without one, in which case the warning log alone carries it.
	sink *eventSink
}

func newAuthStore() *authStore {
	return &authStore{
		chains:         map[string][]resolvedKey{},
		srcByIface:     map[string][4]byte{},
		sendSeq:        map[string]uint32{},
		recvSeq:        map[replayKey]uint64{},
		bootCount:      0,
		now:            time.Now,
		lastKeyNoticed: map[string]bool{},
	}
}

// setSink installs the operator event bus RFC 5709's last-key notification is sent on.
// Safe for concurrent use.
func (s *authStore) setSink(sink *eventSink) {
	s.mu.Lock()
	s.sink = sink
	s.mu.Unlock()
}

// setBootCount installs the durably incremented high word and the callback used
// to advance it on wrap. Runtime callers MUST finish this before enabling packet
// processing, and the callback MUST return only after the new value is durable.
func (s *authStore) setBootCount(bc uint32, increment func() (uint32, error)) {
	s.mu.Lock()
	s.bootCount = bc
	s.nextBootCount = increment
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
	// A new chain is a new key configured: RFC 5709 Section 3.2's infinite lifetime ends
	// with it, and a later expiry of the new chain is notified afresh.
	s.lastKeyNoticed = map[string]bool{}
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

// lastKeyIndex returns the index of the chain's last key: the latest-starting send
// key, and on a tie the one whose accept-lifetime ends later (a zero end is unbounded,
// so it ends last). The signer and the verifier both use this one choice, so two
// routers holding the same expired chain keep signing and accepting the same key.
// The caller has already checked keys is non-empty.
func lastKeyIndex(keys []resolvedKey) int {
	last := 0
	for i := 1; i < len(keys); i++ {
		k, best := &keys[i], &keys[last]
		if k.sendStart.After(best.sendStart) {
			last = i
			continue
		}
		if k.sendStart.Equal(best.sendStart) && endsLater(k.acceptStop, best.acceptStop) {
			last = i
		}
	}
	return last
}

// endsLater reports whether the window end a is later than b, a zero end being
// unbounded.
func endsLater(a, b time.Time) bool {
	if b.IsZero() {
		return false
	}
	return a.IsZero() || a.After(b)
}

// ended reports whether a lifetime whose end is stop is over at now. A zero stop is
// an unbounded lifetime and never ends.
func ended(stop, now time.Time) bool {
	return !stop.IsZero() && !now.Before(stop)
}

// selectSendKey picks the key that signs at time now, and reports whether it signs only
// because RFC 5709 Section 3.2 extends the last key's lifetime. RFC 7210: the active send
// key is the one whose send-lifetime [sendStart, sendStop) covers now; a zero bound is
// unbounded. Among several active keys the latest-starting one wins (the freshest
// rolled-in key). With NO active key the chain's last key signs, so the implementation
// never reverts to unauthenticated (AuType 0); the result is true when that key's
// send-lifetime has ended, and false when no key has started yet. The caller has already
// checked keys is non-empty.
func selectSendKey(keys []resolvedKey, now time.Time) (resolvedKey, bool) {
	var active *resolvedKey
	for i := range keys {
		k := &keys[i]
		started := k.sendStart.IsZero() || !now.Before(k.sendStart)
		notStopped := k.sendStop.IsZero() || now.Before(k.sendStop)
		if started && notStopped {
			if active == nil || k.sendStart.After(active.sendStart) {
				active = k
			}
		}
	}
	if active != nil {
		return *active, false
	}
	last := keys[lastKeyIndex(keys)]
	// RFC 5709 Section 3.2: "In the event that the last key associated with an interface
	// expires, it is unacceptable to revert to an unauthenticated condition, and not
	// advisable to disrupt routing. Therefore, the router should send a "last
	// Authentication Key expiration" notification to the network manager and treat the key
	// as having an infinite lifetime until the lifetime is extended, the key is deleted by
	// network management, or a new key is configured."
	return last, ended(last.sendStop, now)
}

// lastKeyExtendedIndex returns the index of the chain's last key when RFC 5709 Section
// 3.2 extends its accept-lifetime at now: no key of the chain accepts at now and the last
// key's accept-lifetime has ended. It returns -1 otherwise, including when the chain's
// windows have not opened yet, which is not an expiry.
func lastKeyExtendedIndex(keys []resolvedKey, now time.Time) int {
	for i := range keys {
		if keys[i].acceptsAt(now) {
			return -1
		}
	}
	last := lastKeyIndex(keys)
	if !ended(keys[last].acceptStop, now) {
		return -1
	}
	return last
}

// noticeLocked returns RFC 5709 Section 3.2's notification for iface the first time its
// last key is found expired, and nil after that until configure installs a new chain.
// The caller MUST hold s.mu and MUST pass the result to notifyLastKeyExpiration after
// releasing it.
func (s *authStore) noticeLocked(iface string, keyID uint32, direction string) *lastKeyExpirationEvent {
	if s.lastKeyNoticed[iface] {
		return nil
	}
	s.lastKeyNoticed[iface] = true
	return &lastKeyExpirationEvent{Interface: iface, KeyID: keyID, Direction: direction}
}

// notifyLastKeyExpiration sends notice to the network manager: a warning in the OSPF log
// and a last-key-expiration event on the operator event bus. A nil notice sends nothing.
func notifyLastKeyExpiration(sink *eventSink, notice *lastKeyExpirationEvent) {
	if notice == nil {
		return
	}
	logger().Warn("ospf: last Authentication Key expiration, keeping the key as if its lifetime were infinite",
		"interface", notice.Interface, "key-id", notice.KeyID, "direction", notice.Direction)
	sink.lastKeyExpired(notice)
}

// signKey returns the active signing key, its AuType, the next cryptographic sequence
// number, and the interface's IPv4 source address (for the AuType 3 Apad bind).
// A false result with AuTypeNull means no authentication is configured. A false
// result with another AuType refuses signing; the caller MUST discard that packet.
func (s *authStore) signKey(iface string) (packet.AuthKey, packet.AuType, uint64, [4]byte, bool) {
	var (
		notice *lastKeyExpirationEvent
		sink   *eventSink
	)
	// Deferred first so it runs after the unlock below: the notification never holds s.mu.
	defer func() { notifyLastKeyExpiration(sink, notice) }()

	s.mu.Lock()
	defer s.mu.Unlock()
	keys := s.chains[iface]
	if len(keys) == 0 {
		return packet.AuthKey{}, packet.AuTypeNull, 0, [4]byte{}, false
	}
	k, lastExpired := selectSendKey(keys, s.now())
	if lastExpired {
		// RFC 5709 Section 3.2
		notice, sink = s.noticeLocked(iface, k.keyID, "send"), s.sink
	}
	var seq uint64
	if k.auType != packet.AuTypeSimple {
		c := s.sendSeq[iface] + 1
		if k.auType == packet.AuTypeCryptographicESN {
			// RFC 7474 §2: reserve the new high word durably BEFORE emitting the
			// wrapped low word. Commit neither counter after a failed reservation,
			// so a later packet cannot bypass this gate with a nonzero low word.
			if c == 0 {
				if s.nextBootCount == nil {
					return packet.AuthKey{}, k.auType, 0, [4]byte{}, false
				}
				boot, err := s.nextBootCount()
				if err != nil {
					return packet.AuthKey{}, k.auType, 0, [4]byte{}, false
				}
				if boot <= s.bootCount {
					return packet.AuthKey{}, k.auType, 0, [4]byte{}, false
				}
				s.bootCount = boot
			}
			seq = uint64(s.bootCount)<<32 | uint64(c)
		} else {
			// RFC 2328 App D: a single 32-bit non-decreasing value, seeded from the boot word
			// so it never regresses across a restart (R-8); only the low word ships. The 32-bit
			// space is inherent to App D -- a session that exhausts it must re-key (use the ESN
			// AuType, which carries the full 64-bit sequence, to avoid the wrap entirely).
			seq = uint64(s.bootCount + c)
		}
		s.sendSeq[iface] = c
	}
	return packet.AuthKey{KeyID: k.keyID, Algorithm: k.algo, Secret: k.secret}, k.auType, seq, s.srcByIface[iface], true
}

// verify authenticates wire received from neighbor rid on iface. src is the IPv4 source
// address from the packet's IP header (bound into the AuType 3 Apad per RFC 7474 §5). It
// returns ("", true) when accepted (including when no auth is configured), or a failure
// reason and false.
//
// Only a key whose accept-lifetime covers the current time is tried, so an operator
// retires a key by closing its window while a successor is live, and never has to delete
// it from the chain. When EVERY window of the chain has closed, the chain's last key
// (lastKeyIndex, the same key selectSendKey signs with) keeps verifying as if its lifetime
// were infinite, and the network manager is notified once: RFC 5709 Section 3.2 rules out
// both the unauthenticated fallback and the routing disruption a refusal would cause. A
// packet with a non-cryptographic AuType is still refused as autype-mismatch.
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
	extended := -1
	var notice *lastKeyExpirationEvent
	if len(keys) > 0 {
		extended = lastKeyExtendedIndex(keys, now)
	}
	if extended >= 0 {
		// RFC 5709 Section 3.2
		notice = s.noticeLocked(iface, keys[extended].keyID, "receive")
	}
	sink := s.sink
	s.mu.Unlock()
	notifyLastKeyExpiration(sink, notice)
	h, _, err := packet.DecodeHeader(wire)
	if err != nil {
		return "decode", false
	}
	// RFC 2328 Section 8.2: "The AuType specified in the packet must match the AuType
	// specified for the associated area." An interface with no key chain runs Null
	// authentication (AuType 0), so a packet carrying any other AuType is refused there too.
	if len(keys) == 0 {
		if h.AuType != packet.AuTypeNull {
			return "autype-mismatch", false
		}
		return "", true
	}
	if h.AuType != keys[0].auType {
		return "autype-mismatch", false
	}
	// senderKeyID is the key the packet itself names. AuType 1 names none, so a
	// simple-password chain answers for the chain as a whole and never per key.
	senderKeyID, senderNamedKey := packet.AuthKeyID(h)
	senderKeyRetired := false
	inWindow := 0
	for i, k := range keys {
		// RFC 7474 Section 4: "For packet reception, the key validity interval as
		// defined by AcceptLifetimeStart and AcceptLifetimeEnd must include the current
		// time." A key outside its window is skipped before the digest is computed, so
		// it can neither accept the packet nor record its sequence number. The one
		// exception is the chain's expired last key, whose validity interval RFC 5709
		// Section 3.2 makes infinite ("treat the key as having an infinite lifetime").
		if i != extended && !k.acceptsAt(now) {
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
		if seen && replayedSequence(h.AuType, seq, last) {
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

// replayedSequence reports whether a verified packet's cryptographic sequence number seq
// is a replay against last, the highest sequence already accepted for the same neighbor,
// key and packet type. The two cryptographic AuTypes carry different rules.
func replayedSequence(auType packet.AuType, seq, last uint64) bool {
	if auType == packet.AuTypeCryptographicESN {
		// RFC 7474 Section 2: "Upon reception, the sequence number MUST be greater than the
		// sequence number in the last OSPF packet of that type accepted from the sending OSPF
		// neighbor. Otherwise, the OSPF packet is considered a replayed packet and dropped."
		return seq <= last
	}
	// RFC 2328 Section D.4.3 (2): "If the cryptographic sequence number found in the OSPF
	// header (see Figure 18) is less than the cryptographic sequence number recorded in the
	// sending neighbor's data structure, the OSPF packet is discarded." The AuType 2 sequence
	// is "non-decreasing" (Section D.3), so an equal sequence is accepted.
	return seq < last
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
