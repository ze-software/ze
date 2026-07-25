// Design: plan/learned/935-isis-10-auth.md -- IS-IS authentication key store.
//
// RFC: rfc/short/rfc5304.md -- area/domain/link-level authentication strings (sec 2)
// RFC: rfc/short/rfc5310.md -- Security Associations keyed by Key ID, algorithm agility (sec 2)
//
// The key store turns the resolved YANG key chains (config.go KeyChainConfig,
// owned by isis-4) into the runtime keys the packet auth backend signs and
// verifies with. It decodes the $9$-encoded secrets to derive the HMAC keys held
// ONLY in memory (internal/component/config/secret), selects a single ACTIVE
// signing key per chain (the active key plus all currently-valid keys accepted
// on receive give hitless rotation, spec AC-4, A-5), and resolves which chain
// applies per PDU class and level:
//
//   - IIH (per-interface): the circuit's level-1/level-2 auth-key-chain (the
//     Link Level Authentication string, RFC 5304 sec 2);
//   - LSP / CSNP / PSNP (per-level): the area key for L1, the domain key for L2
//     (the per-level auth-key-chain, RFC 5304 sec 2).
//
// The store is built once per config apply and read on the hot path (sign on TX,
// verify on RX), so it is immutable after build and safe for concurrent reads.

package isis

import (
	"time"

	"github.com/ze-software/ze/internal/component/config/secret"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
)

// maxKeysPerChain bounds the number of keys the verify path tries per PDU, so a
// configuration (or a forged-PDU flood against a huge chain) cannot amplify CPU
// cost without bound (spec Security Review: resource use). Realistic chains hold
// a handful of keys (an active key plus one or two rotation standbys).
const maxKeysPerChain = 16

// authKey is one resolved, ready-to-use key: the packet-layer key (algorithm +
// decoded secret + key-id) plus the send/accept validity windows for hitless
// rotation. A zero send/accept window means "always valid" (no lifetime set).
type authKey struct {
	key    packet.Key
	send   lifetime
	accept lifetime
}

// lifetime is a [start, end] validity window. A zero value (both unset) means
// "always valid". start unset means "valid from the beginning"; end unset means
// "valid forever".
type lifetime struct {
	start    time.Time
	end      time.Time
	hasStart bool
	hasEnd   bool
}

// contains reports whether t falls within the window (inclusive). An all-unset
// window always contains t.
func (l lifetime) contains(t time.Time) bool {
	if l.hasStart && t.Before(l.start) {
		return false
	}
	if l.hasEnd && t.After(l.end) {
		return false
	}
	return true
}

// keyChain is a resolved named chain of keys, ordered by key-id (config order).
type keyChain struct {
	name string
	keys []authKey
}

// activeKey returns the key to SIGN with at time now: the first key whose SEND
// lifetime contains now (config order, lowest key-id first). Returns ok=false
// when no key is currently valid for signing (RFC 5310 sec 3.4: if no active key
// is found the PDU is discarded -- the caller then sends unsigned, i.e. leaves
// the PDU unchanged, which a peer under auth will reject). A cleartext or
// HMAC-MD5 chain with no lifetime always has an active key.
func (c *keyChain) activeKey(now time.Time) (packet.Key, bool) {
	for _, k := range c.keys {
		if k.send.contains(now) {
			return k.key, true
		}
	}
	return packet.Key{}, false
}

// acceptKeys returns every key whose ACCEPT lifetime contains now, for verify
// (RFC 5304 sec 2: an implementation MAY check a set of passwords; spec AC-4
// hitless rotation accepts the old and new key during the overlap window). The
// slice is capped at maxKeysPerChain.
func (c *keyChain) acceptKeys(now time.Time) []packet.Key {
	out := make([]packet.Key, 0, len(c.keys))
	for _, k := range c.keys {
		if !k.accept.contains(now) {
			continue
		}
		out = append(out, k.key)
		if len(out) >= maxKeysPerChain {
			break
		}
	}
	return out
}

// keyStore holds the resolved chains by name plus the per-level (area/domain) and
// per-interface (circuit) chain references. It is immutable after build.
type keyStore struct {
	chains map[string]*keyChain

	// Per-level (LSP/CSNP/PSNP) chains: area key for L1, domain key for L2.
	level1 *keyChain
	level2 *keyChain

	// Per-interface (IIH) chains, keyed by interface name then level. A circuit
	// may set a different chain per level; an unset level inherits no chain.
	ifaceL1 map[string]*keyChain
	ifaceL2 map[string]*keyChain
}

// newKeyStore resolves cfg into a keyStore. It decodes the $9$ secrets to derive
// the HMAC keys (held only in memory), maps the algorithm enum to the packet
// algorithm, and wires the per-level and per-interface chain references from the
// config's auth-key-chain leaves. A config with no key chains yields an empty
// store (authentication disabled, the default).
func newKeyStore(cfg Config) *keyStore {
	ks := &keyStore{
		chains:  make(map[string]*keyChain, len(cfg.KeyChains)),
		ifaceL1: make(map[string]*keyChain),
		ifaceL2: make(map[string]*keyChain),
	}
	for _, kc := range cfg.KeyChains {
		ks.chains[kc.Name] = resolveChain(kc)
	}
	ks.level1 = ks.chains[cfg.Level1AuthKeyChain]
	ks.level2 = ks.chains[cfg.Level2AuthKeyChain]
	for _, ic := range cfg.Interfaces {
		if name := ic.Level1.AuthKeyChain; name != "" {
			if c := ks.chains[name]; c != nil {
				ks.ifaceL1[ic.Name] = c
			}
		}
		if name := ic.Level2.AuthKeyChain; name != "" {
			if c := ks.chains[name]; c != nil {
				ks.ifaceL2[ic.Name] = c
			}
		}
	}
	return ks
}

// resolveChain converts one KeyChainConfig into a runtime keyChain, decoding each
// key's secret and dropping keys with an unknown algorithm or an undecodable
// secret (a malformed key never silently weakens auth: it is simply absent, so a
// PDU it would have signed/verified is rejected).
func resolveChain(kc KeyChainConfig) *keyChain {
	out := &keyChain{name: kc.Name}
	for _, k := range kc.Keys {
		algo, ok := algoFromString(k.Algorithm)
		if !ok {
			continue
		}
		plain, ok := decodeSecret(k.Secret)
		if !ok || len(plain) == 0 {
			continue
		}
		out.keys = append(out.keys, authKey{
			key: packet.Key{
				Algorithm: algo,
				Secret:    plain,
				KeyID:     k.KeyID,
			},
			send:   parseLifetime(k.SendStart, k.SendEnd),
			accept: parseLifetime(k.AcceptStart, k.AcceptEnd),
		})
	}
	return out
}

// algoFromString maps the YANG algorithm enum token (ze-isis-conf.yang
// key-chains/key/algorithm) to the packet auth algorithm. The tokens MUST match
// the schema enum (cleartext / hmac-md5 / hmac-sha-1 / hmac-sha-256 / ...).
func algoFromString(s string) (packet.AuthAlgorithm, bool) {
	switch s {
	case "cleartext":
		return packet.AuthAlgoCleartext, true
	case "hmac-md5":
		return packet.AuthAlgoHMACMD5, true
	case "hmac-sha-1":
		return packet.AuthAlgoHMACSHA1, true
	case "hmac-sha-224":
		return packet.AuthAlgoHMACSHA224, true
	case "hmac-sha-256":
		return packet.AuthAlgoHMACSHA256, true
	case "hmac-sha-384":
		return packet.AuthAlgoHMACSHA384, true
	case "hmac-sha-512":
		return packet.AuthAlgoHMACSHA512, true
	default:
		return packet.AuthAlgoNone, false
	}
}

// decodeSecret returns the plaintext key material from a config secret leaf. A
// $9$-encoded value is decoded (the same reversible encoding PPPoE/WireGuard use,
// spec A-4); a plaintext value (operator typed it raw before commit auto-encoded
// it) is returned as-is. A decode failure returns ok=false so the key is dropped.
func decodeSecret(s string) ([]byte, bool) {
	if s == "" {
		return nil, false
	}
	if secret.IsEncoded(s) {
		plain, err := secret.Decode(s)
		if err != nil {
			return nil, false
		}
		return []byte(plain), true
	}
	return []byte(s), true
}

// parseLifetime builds a validity window from RFC3339 start/end strings. An empty
// string leaves that bound unset (open). An unparseable timestamp leaves the
// bound unset (fail-open on the bound, never fail-closed silently dropping the
// key -- a malformed lifetime should not disable a configured key).
func parseLifetime(start, end string) lifetime {
	var l lifetime
	if start != "" {
		if t, err := time.Parse(time.RFC3339, start); err == nil {
			l.start = t
			l.hasStart = true
		}
	}
	if end != "" {
		if t, err := time.Parse(time.RFC3339, end); err == nil {
			l.end = t
			l.hasEnd = true
		}
	}
	return l
}

// ---- per-PDU-class chain resolution ----

// helloChain returns the per-interface (IIH) chain for a circuit at a level, or
// nil when none is configured (IIH then sends/accepts unauthenticated).
func (ks *keyStore) helloChain(iface string, level lsdbLevel) *keyChain {
	if ks == nil {
		return nil
	}
	if level == levelTwo {
		return ks.ifaceL2[iface]
	}
	return ks.ifaceL1[iface]
}

// levelChain returns the per-level (LSP/CSNP/PSNP) chain for a level: the area
// key for L1, the domain key for L2 (RFC 5304 sec 2). nil means unauthenticated.
func (ks *keyStore) levelChain(level lsdbLevel) *keyChain {
	if ks == nil {
		return nil
	}
	if level == levelTwo {
		return ks.level2
	}
	return ks.level1
}

// lsdbLevel is a local level discriminator so the key store does not import the
// lsdb or adjacency packages (it is a leaf used by the engine wiring, which
// maps lsdb.Level / adjacency.Level into it).
type lsdbLevel uint8

const (
	levelOne lsdbLevel = iota
	levelTwo
)

// signKey returns the active signing key for a chain at now, or ok=false when no
// chain is set or no key is currently valid for signing.
func (ks *keyStore) signKey(c *keyChain, now time.Time) (packet.Key, bool) {
	if c == nil {
		return packet.Key{}, false
	}
	return c.activeKey(now)
}

// verifyKeys returns the keys accepted on receive for a chain at now, or nil when
// no chain is set (verify then accepts unauthenticated, the default).
func (ks *keyStore) verifyKeys(c *keyChain, now time.Time) []packet.Key {
	if c == nil {
		return nil
	}
	return c.acceptKeys(now)
}

// configured reports whether the store holds any chain at all (so the engine can
// skip the sign/verify hooks entirely when no auth is configured).
func (ks *keyStore) configured() bool {
	return ks != nil && len(ks.chains) > 0
}
