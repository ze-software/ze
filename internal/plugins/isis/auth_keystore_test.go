// Design: plan/spec-isis-10-auth.md -- IS-IS authentication key store tests.
//
// These tests exercise chain resolution (per-level area/domain, per-interface
// circuit), active-key selection, hitless rotation via send/accept lifetimes,
// and the $9$ secret decode that keeps key material out of plaintext config.

package isis

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/secret"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
)

// VALIDATES: TestISISAuthKeyStore (TDD plan) -- per-level (area/domain) and
// per-interface (circuit) chains resolve, and an active signing key is selected.
func TestISISAuthKeyStore(t *testing.T) {
	cfg := Config{
		Level1AuthKeyChain: "area-key",
		Level2AuthKeyChain: "domain-key",
		KeyChains: []KeyChainConfig{
			{Name: "area-key", Keys: []KeyConfig{{KeyID: 1, Algorithm: "hmac-md5", Secret: "areasecret"}}},
			{Name: "domain-key", Keys: []KeyConfig{{KeyID: 2, Algorithm: "hmac-sha-256", Secret: "domainsecret"}}},
			{Name: "iih-key", Keys: []KeyConfig{{KeyID: 3, Algorithm: "hmac-sha-256", Secret: "iihsecret"}}},
		},
		Interfaces: []InterfaceConfig{
			{
				Name:   "eth0",
				Level1: LevelInterfaceConfig{AuthKeyChain: "iih-key"},
				Level2: LevelInterfaceConfig{AuthKeyChain: "iih-key"},
			},
		},
	}
	ks := newKeyStore(cfg)
	if !ks.configured() {
		t.Fatal("keystore should report configured with 3 chains")
	}
	now := time.Now()

	// Per-level chains: area key for L1, domain key for L2.
	l1 := ks.levelChain(levelOne)
	if l1 == nil {
		t.Fatal("no L1 (area) chain resolved")
	}
	k1, ok := ks.signKey(l1, now)
	if !ok || k1.Algorithm != packet.AuthAlgoHMACMD5 || string(k1.Secret) != "areasecret" {
		t.Fatalf("L1 active key wrong: %+v ok=%v", k1, ok)
	}
	l2 := ks.levelChain(levelTwo)
	k2, ok := ks.signKey(l2, now)
	if !ok || k2.Algorithm != packet.AuthAlgoHMACSHA256 || k2.KeyID != 2 {
		t.Fatalf("L2 active key wrong: %+v ok=%v", k2, ok)
	}

	// Per-interface (IIH) chains.
	ih := ks.helloChain("eth0", levelOne)
	ki, ok := ks.signKey(ih, now)
	if !ok || string(ki.Secret) != "iihsecret" {
		t.Fatalf("IIH active key wrong: %+v ok=%v", ki, ok)
	}

	// A circuit/level with no chain returns nil (unauthenticated): no sign key,
	// no verify keys.
	if c := ks.helloChain("eth1", levelOne); c != nil {
		t.Fatal("unexpected chain for unconfigured interface")
	}
	if keys := ks.verifyKeys(ks.helloChain("eth1", levelOne), now); keys != nil {
		t.Fatal("unconfigured interface should yield no verify keys")
	}
}

// VALIDATES: TestISISAuthRotation (TDD plan, AC-4) -- during an overlap window
// both the old and the new key are accepted on receive, and the active signing
// key follows the send lifetime, so a key change does not drop adjacencies.
//
// RFC requirement: RFC5310-4-2 positive -- during the overlap window the store keeps
// and uses more than one key at once: verifyKeys returns BOTH the old and the new key
// (len == 2) before and after the rollover, so a PDU signed by either currently valid
// key is accepted; implementations must store and use more than one key at the same
// time (RFC 5310 sec 4).
func TestISISAuthRotation(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	old := base.Add(-time.Hour).Format(time.RFC3339)
	mid := base.Add(time.Hour).Format(time.RFC3339)
	end := base.Add(2 * time.Hour).Format(time.RFC3339)

	cfg := Config{
		Level1AuthKeyChain: "rot",
		KeyChains: []KeyChainConfig{{
			Name: "rot",
			Keys: []KeyConfig{
				// Old key: sends until mid; accepted until end (overlap window).
				{KeyID: 1, Algorithm: "hmac-sha-256", Secret: "oldkey",
					SendStart: old, SendEnd: mid, AcceptStart: old, AcceptEnd: end},
				// New key: sends from mid; accepted from old (so it overlaps).
				{KeyID: 2, Algorithm: "hmac-sha-256", Secret: "newkey",
					SendStart: mid, SendEnd: end, AcceptStart: old, AcceptEnd: end},
			},
		}},
	}
	ks := newKeyStore(cfg)
	chain := ks.levelChain(levelOne)

	// Before the rollover (now < mid): the OLD key signs; both are accepted.
	before := base
	sk, ok := ks.signKey(chain, before)
	if !ok || sk.KeyID != 1 {
		t.Fatalf("before rollover active key = %+v ok=%v, want key 1", sk, ok)
	}
	if keys := ks.verifyKeys(chain, before); len(keys) != 2 {
		t.Fatalf("before rollover accepts %d keys, want 2 (overlap)", len(keys))
	}

	// After the rollover (now > mid): the NEW key signs; both still accepted
	// until end (no flap during the overlap).
	after := base.Add(90 * time.Minute)
	sk, ok = ks.signKey(chain, after)
	if !ok || sk.KeyID != 2 {
		t.Fatalf("after rollover active key = %+v ok=%v, want key 2", sk, ok)
	}
	if keys := ks.verifyKeys(chain, after); len(keys) != 2 {
		t.Fatalf("after rollover accepts %d keys, want 2 (still overlapping)", len(keys))
	}

	// Past the end of both windows: no signing key, no accept keys.
	expired := base.Add(3 * time.Hour)
	if _, ok := ks.signKey(chain, expired); ok {
		t.Fatal("expected no active signing key past all lifetimes")
	}
	if keys := ks.verifyKeys(chain, expired); len(keys) != 0 {
		t.Fatalf("expected no accept keys past all lifetimes, got %d", len(keys))
	}
}

// VALIDATES: TestISISAuthSecretEncoding (TDD plan, AC-11) -- a $9$-encoded leaf
// decodes to the derived key, and plaintext key material is never retained in the
// config string form (the store holds only the decoded bytes in memory).
func TestISISAuthSecretEncoding(t *testing.T) {
	plain := "supersecretkey"
	enc, err := secret.Encode(plain)
	if err != nil {
		t.Fatalf("secret.Encode: %v", err)
	}
	if !secret.IsEncoded(enc) {
		t.Fatalf("encoded value %q lacks $9$ prefix", enc)
	}
	cfg := Config{
		Level1AuthKeyChain: "kc",
		KeyChains: []KeyChainConfig{{
			Name: "kc",
			Keys: []KeyConfig{{KeyID: 1, Algorithm: "hmac-sha-256", Secret: enc}},
		}},
	}
	ks := newKeyStore(cfg)
	k, ok := ks.signKey(ks.levelChain(levelOne), time.Now())
	if !ok {
		t.Fatal("no active key from $9$-encoded secret")
	}
	if string(k.Secret) != plain {
		t.Fatalf("decoded key = %q, want %q", string(k.Secret), plain)
	}

	// A plaintext secret (pre-commit, before auto-encode) is accepted as-is.
	ks2 := newKeyStore(Config{
		Level1AuthKeyChain: "kc",
		KeyChains:          []KeyChainConfig{{Name: "kc", Keys: []KeyConfig{{KeyID: 1, Algorithm: "hmac-md5", Secret: "rawpass"}}}},
	})
	k2, _ := ks2.signKey(ks2.levelChain(levelOne), time.Now())
	if string(k2.Secret) != "rawpass" {
		t.Fatalf("plaintext secret = %q, want rawpass", string(k2.Secret))
	}
}

// VALIDATES: a key with an unknown algorithm or an undecodable secret is dropped
// (never silently weakening auth: the absent key cannot sign or verify).
func TestISISAuthKeyStoreDropsInvalid(t *testing.T) {
	cfg := Config{
		Level1AuthKeyChain: "kc",
		KeyChains: []KeyChainConfig{{
			Name: "kc",
			Keys: []KeyConfig{
				{KeyID: 1, Algorithm: "bogus-algo", Secret: "x"},   // unknown algo
				{KeyID: 2, Algorithm: "hmac-md5", Secret: ""},      // empty secret
				{KeyID: 3, Algorithm: "hmac-md5", Secret: "valid"}, // good
			},
		}},
	}
	ks := newKeyStore(cfg)
	chain := ks.levelChain(levelOne)
	keys := ks.verifyKeys(chain, time.Now())
	if len(keys) != 1 || keys[0].KeyID != 3 {
		t.Fatalf("expected only the valid key (id 3), got %+v", keys)
	}
}

// VALIDATES: an empty config yields an unconfigured store, so the engine skips
// the sign/verify hooks (unauthenticated operation is the default).
func TestISISAuthKeyStoreEmpty(t *testing.T) {
	ks := newKeyStore(Config{})
	if ks.configured() {
		t.Fatal("empty config should be unconfigured")
	}
	if ks.levelChain(levelOne) != nil || ks.helloChain("eth0", levelOne) != nil {
		t.Fatal("empty store should resolve no chains")
	}
}

// VALIDATES: the verify path caps the number of keys tried per chain so a huge
// chain cannot amplify CPU on a forged-PDU flood (spec Security Review).
func TestISISAuthKeyStoreChainCap(t *testing.T) {
	keys := make([]KeyConfig, maxKeysPerChain+5)
	for i := range keys {
		keys[i] = KeyConfig{KeyID: uint16(i), Algorithm: "hmac-md5", Secret: "s"}
	}
	ks := newKeyStore(Config{
		Level1AuthKeyChain: "big",
		KeyChains:          []KeyChainConfig{{Name: "big", Keys: keys}},
	})
	got := ks.verifyKeys(ks.levelChain(levelOne), time.Now())
	if len(got) != maxKeysPerChain {
		t.Fatalf("verify keys = %d, want cap %d", len(got), maxKeysPerChain)
	}
}
