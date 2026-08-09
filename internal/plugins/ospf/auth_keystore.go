// Design: docs/architecture/ospf/ospf-12-auth.md -- OSPFv2 authentication key store.
// Related: internal/plugins/ospf/packet -- the Sign/Verify crypto backend.
// RFC: rfc/short/rfc2328.md (App D), rfc/short/rfc5709.md, rfc/short/rfc7474.md

package ospf

import (
	"crypto/sha1" //nolint:gosec // G505: not used as a security primitive; only to diffuse a high-resolution clock into a 32-bit boot-count seed (RFC 7474 high word) when ZeFS persistence is unavailable.
	"encoding/binary"
	"io/fs"
	"path/filepath"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/config/secret"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/paths"
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
// key, accepts any chain key on receive (hitless rotation), and enforces the RFC 2328
// App D / RFC 7474 non-decreasing cryptographic sequence number per neighbor.
type authStore struct {
	mu         sync.Mutex
	chains     map[string][]resolvedKey // interface -> keys (sign with [0], accept any)
	srcByIface map[string][4]byte       // interface -> IPv4 source address (RFC 7474 Apad bind)
	sendSeq    map[string]uint32        // interface -> per-packet send counter (low-order word)
	recvSeq    map[replayKey]uint64     // last accepted sequence
	// bootCount is the RFC 7474 high-order boot word. It is the authoritative
	// monotonic source so the aggregate 64-bit cryptographic sequence strictly
	// increases for the router's lifetime, including across a cold restart (a peer
	// enforcing a strictly-increasing sequence keeps the adjacency). The engine seeds
	// it from the ZeFS-persisted, incremented boot count (loadOSPFBootCount) when
	// persistence is available; otherwise newAuthStore seeds it from a hashed
	// high-resolution clock (bootCountFromClock) which advances on every restart.
	bootCount uint32
	// now is the wall clock used for send-key lifetime selection. It defaults to
	// time.Now and is overridden in tests for deterministic lifetime windows.
	now func() time.Time
}

func newAuthStore() *authStore {
	return &authStore{
		chains:     map[string][]resolvedKey{},
		srcByIface: map[string][4]byte{},
		sendSeq:    map[string]uint32{},
		recvSeq:    map[replayKey]uint64{},
		bootCount:  bootCountFromClock(),
		now:        time.Now,
	}
}

// setBootCount overrides the RFC 7474 high-order boot word with the authoritative
// (ZeFS-persisted, incremented) value the engine resolves at startup. Call once,
// before the store signs any packet.
func (s *authStore) setBootCount(bc uint32) {
	s.mu.Lock()
	s.bootCount = bc
	s.mu.Unlock()
}

// bootCountFromClock derives a boot-count seed by hashing a high-resolution
// timestamp and truncating the digest to 32 bits. RFC 7474 requires the aggregate
// sequence to strictly increase across restarts; when ZeFS persistence is
// unavailable a plain seconds-granularity wall clock can collide on a fast
// restart, so the nanosecond clock is diffused through SHA-1 to spread successive
// boots across the 32-bit space.
func bootCountFromClock() uint32 {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(time.Now().UnixNano()))
	sum := sha1.Sum(buf[:]) //nolint:gosec // G401: diffusion of a timestamp, not a security digest.
	return binary.BigEndian.Uint32(sum[:4])
}

// bootCountStore is the minimal ZeFS surface loadOSPFBootCount needs. Satisfied by
// *zefs.BlobStore.
type bootCountStore interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm fs.FileMode) error
}

// loadOSPFBootCount reads the persisted OSPF auth boot count from store, increments
// it once (this boot), writes it back, and returns the incremented value. RFC 7474
// §3: the aggregate 64-bit sequence (boot count high word | per-packet low word) must
// strictly increase across a cold restart; persisting and incrementing the boot count
// is the authoritative mechanism for that. When store is nil or any read/write fails,
// it falls back to the hashed high-resolution clock seed (which still advances per
// restart, just without durable monotonicity). This is the single per-boot write; the
// per-packet counter never touches ZeFS.
func loadOSPFBootCount(store bootCountStore) uint32 {
	if store == nil {
		return bootCountFromClock()
	}
	key := zefs.KeyOSPFAuthBootCount.Key()
	var prev uint32
	if data, err := store.ReadFile(key); err == nil && len(data) == 4 {
		prev = binary.BigEndian.Uint32(data)
	}
	next := prev + 1
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], next)
	if err := store.WriteFile(key, buf[:], 0); err != nil {
		// The increment could not be made durable; fall back so the high word still
		// advances this boot rather than reusing prev (which a peer would reject).
		return bootCountFromClock()
	}
	return next
}

// pinnedStateDir returns the config directory ONLY when the operator pinned one
// with ze.config.dir, and "" otherwise.
//
// This mirrors the gate the daemon applies to runtime-state persistence
// (cmd/ze/hub/main.go: the `else if env.Get("ze.config.dir") != ""` branch, whose
// comment spells out why). Without an explicit pin, paths.DefaultConfigDir falls
// back to the binary-relative etc/ze, which EVERY `ze` invocation on the host
// shares. The OSPF engine runs as its own process, and zefs's lock is an
// in-process sync.RWMutex (pkg/zefs/lock.go) that cannot serialize across
// processes -- so opening that shared database.zefs put 64 functional-test
// daemons on one file. That contention is what produced the SIGBUS behind
// test/ospf/ospf-ldp-sync-restore.ci.
//
// Unpinned, both callers degrade exactly as they already do when no store can be
// found: the boot count falls back to the hashed clock seed, and GR restart facts
// are not persisted across a restart.
// resolve is passed in rather than called directly so a test can observe the
// gate: under `go test` the binary lives in a build temp dir, so
// paths.DefaultConfigDir returns "" on its own and a gate that did nothing would
// look identical to one that works.
func pinnedStateDir(resolve func() string) string {
	if env.Get("ze.config.dir") == "" {
		return ""
	}
	return resolve()
}

// openBootCountStore opens the pinned ZeFS database for boot-count persistence,
// returning nil when the operator pinned no config dir (pinnedStateDir) or no
// store can be opened there (a fresh appliance before its database exists). A nil
// return makes loadOSPFBootCount fall back to the hashed clock seed. The boot
// count is a single read+write at startup, so the store is closed immediately
// rather than held for the engine's lifetime.
func openBootCountStore() bootCountStore {
	dir := pinnedStateDir(paths.DefaultConfigDir)
	if dir == "" {
		return nil
	}
	store, err := zefs.Open(filepath.Join(dir, "database.zefs"))
	if err != nil {
		return nil
	}
	// The boot count is read+incremented+written synchronously below by the caller;
	// wrap the store so it is closed once that single operation completes.
	return &closingBootCountStore{store: store}
}

// closingBootCountStore closes the underlying BlobStore after the one-shot boot-count
// write, so the OSPF engine does not keep the shared database mmap'd for its lifetime.
type closingBootCountStore struct {
	store *zefs.BlobStore
}

func (c *closingBootCountStore) ReadFile(name string) ([]byte, error) {
	return c.store.ReadFile(name)
}

func (c *closingBootCountStore) WriteFile(name string, data []byte, perm fs.FileMode) error {
	err := c.store.WriteFile(name, data, perm)
	_ = c.store.Close() // best-effort: the boot count is the only write this boot.
	return err
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

// resolveChainKeys resolves a key chain's keys into the send-key form the store holds.
// Lifetimes are validated by validateConfig before configure runs, so a parse failure here
// is impossible; a zero window means "always valid" (unset).
func resolveChainKeys(kc keyChainConfig) []resolvedKey {
	keys := make([]resolvedKey, 0, len(kc.Keys))
	for _, k := range kc.Keys {
		start, stop, _ := lifetimeBounds(k.SendLifetime)
		keys = append(keys, resolvedKey{
			keyID:     k.KeyID,
			auType:    authAuType(k.Algorithm, kc.ExtendedSequence),
			algo:      k.Algorithm,
			secret:    decodeSecret(k.Secret),
			sendStart: start,
			sendStop:  stop,
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
func (s *authStore) verify(iface string, rid types.RouterID, src [4]byte, wire []byte) (string, bool) {
	s.mu.Lock()
	keys := s.chains[iface]
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
	for _, k := range keys {
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
