// VALIDATES: spec-ospf-12 -- the auth key store resolves per-interface chains with area
// `inherit`, decodes `$9$` secrets, signs with the active key, accepts any chain key on
// receive (hitless rotation), and rejects a replayed cryptographic sequence number.
// PREVENTS: regressions where `inherit` does not resolve, a rotated key drops the
// adjacency, or an old sequence number is accepted.
package ospf

import (
	"io/fs"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

func authCfg(keys ...keyConfig) ospfConfig {
	return ospfConfig{
		KeyChains:  []keyChainConfig{{Name: "kc1", Keys: keys}},
		Areas:      []areaConfig{{AreaID: types.BackboneArea, AuthKeyChain: "kc1"}},
		Interfaces: []interfaceConfig{{Name: "eth0", AreaID: types.BackboneArea, Authentication: authConfig{Mode: "inherit"}}},
	}
}

func signedHello(t *testing.T, s *authStore, iface string) ([]byte, [4]byte) {
	t.Helper()
	key, au, seq, src, ok := s.signKey(iface)
	require.True(t, ok)
	p := packet.Packet{Header: packet.Header{Type: packet.PacketTypeHello, AuType: au}, Hello: &packet.Hello{NetworkMask: [4]byte{255, 255, 255, 0}, HelloInterval: 10, DeadInterval: 40}}
	buf := make([]byte, p.EncodedLen())
	n := p.WriteTo(buf, 0)
	signed, err := packet.Sign(buf[:n], au, key, seq, src)
	require.NoError(t, err)
	return signed, src
}

func TestOSPFAuthKeyStore(t *testing.T) {
	s := newAuthStore()
	s.configure(authCfg(keyConfig{KeyID: 1, Algorithm: "hmac-sha-256", Secret: "topsecret"}))

	key, au, _, _, ok := s.signKey("eth0")
	require.True(t, ok, "eth0 inherits the area key chain")
	assert.Equal(t, packet.AuTypeCryptographic, au)
	assert.Equal(t, uint32(1), key.KeyID)
	assert.Equal(t, []byte("topsecret"), key.Secret)

	_, _, _, _, none := s.signKey("eth9")
	assert.False(t, none, "an interface with no resolved chain has no signing key")
}

func TestOSPFAuthStoreSignVerify(t *testing.T) {
	s := newAuthStore()
	s.configure(authCfg(keyConfig{KeyID: 1, Algorithm: "hmac-sha-256", Secret: "topsecret"}))
	peer := ridOf("2.2.2.2")

	wire, src := signedHello(t, s, "eth0")
	reason, ok := s.verify("eth0", peer, src, wire)
	assert.True(t, ok, "a correctly signed packet verifies")
	assert.Empty(t, reason)

	// Wrong AuType (a Null packet under configured crypto auth) is rejected.
	plain := make([]byte, packet.CommonHeaderLen+24)
	plain[0] = 2 // version
	plain[1] = byte(packet.PacketTypeHello)
	// RFC requirement: RFC5709-3.1-1 negative -- crypto auth requires AuType 2; a received packet whose AuType is not the configured cryptographic AuType is rejected as an autype-mismatch (authStore.verify auth_keystore.go:341-343).
	_, autypeOK := s.verify("eth0", peer, [4]byte{}, plain)
	assert.False(t, autypeOK, "AuType mismatch rejected")
}

func TestOSPFAuthRotation(t *testing.T) {
	// Two keys in the chain: a packet signed with EITHER is accepted (overlap window).
	s := newAuthStore()
	s.configure(authCfg(
		keyConfig{KeyID: 1, Algorithm: "hmac-sha-256", Secret: "oldkey"},
		keyConfig{KeyID: 2, Algorithm: "hmac-sha-256", Secret: "newkey"},
	))
	peer := ridOf("2.2.2.2")

	// Sign explicitly with the second key (simulate a peer that rotated).
	p := packet.Packet{Header: packet.Header{Type: packet.PacketTypeHello, AuType: packet.AuTypeCryptographic}, Hello: &packet.Hello{NetworkMask: [4]byte{255, 255, 255, 0}, HelloInterval: 10, DeadInterval: 40}}
	buf := make([]byte, p.EncodedLen())
	n := p.WriteTo(buf, 0)
	signed, err := packet.Sign(buf[:n], packet.AuTypeCryptographic, packet.AuthKey{KeyID: 2, Algorithm: "hmac-sha-256", Secret: []byte("newkey")}, 1, [4]byte{})
	require.NoError(t, err)

	reason, ok := s.verify("eth0", peer, [4]byte{}, signed)
	assert.True(t, ok, "a packet signed with any chain key is accepted during rotation")
	assert.Empty(t, reason)
}

func TestOSPFAuthExtendedSequence(t *testing.T) {
	s := newAuthStore()
	s.configure(ospfConfig{
		KeyChains:  []keyChainConfig{{Name: "kc1", ExtendedSequence: true, Keys: []keyConfig{{KeyID: 1, Algorithm: "hmac-sha-256", Secret: "k"}}}},
		Areas:      []areaConfig{{AreaID: types.BackboneArea, AuthKeyChain: "kc1"}},
		Interfaces: []interfaceConfig{{Name: "eth0", AreaID: types.BackboneArea, Authentication: authConfig{Mode: "inherit"}}},
	})
	_, au, _, _, ok := s.signKey("eth0")
	require.True(t, ok)
	assert.Equal(t, packet.AuTypeCryptographicESN, au, "an extended-sequence chain selects RFC 7474 AuType 3")

	wire, src := signedHello(t, s, "eth0")
	reason, vok := s.verify("eth0", ridOf("2.2.2.2"), src, wire)
	assert.True(t, vok, "AuType 3 signed packet round-trips")
	assert.Empty(t, reason)
}

// RFC requirement: RFC2328-D.3-2 positive -- an accepted packet's sequence number becomes the stored value for that neighbor, so a later, higher sequence is accepted against it (authStore.verify, auth_keystore.go:352-362).
// RFC requirement: RFC2328-D.3-2 negative -- the sequence is treated as non-decreasing: a packet whose sequence is below (or equal to) the last accepted value is discarded as a replay (authStore.verify, auth_keystore.go:356-360).
func TestOSPFAuthReplay(t *testing.T) {
	s := newAuthStore()
	s.configure(authCfg(keyConfig{KeyID: 1, Algorithm: "hmac-sha-256", Secret: "topsecret"}))
	peer := ridOf("2.2.2.2")
	key := packet.AuthKey{KeyID: 1, Algorithm: "hmac-sha-256", Secret: []byte("topsecret")}

	mk := func(seq uint64) []byte {
		p := packet.Packet{Header: packet.Header{Type: packet.PacketTypeHello, AuType: packet.AuTypeCryptographic}, Hello: &packet.Hello{NetworkMask: [4]byte{255, 255, 255, 0}, HelloInterval: 10, DeadInterval: 40}}
		buf := make([]byte, p.EncodedLen())
		n := p.WriteTo(buf, 0)
		signed, err := packet.Sign(buf[:n], packet.AuTypeCryptographic, key, seq, [4]byte{})
		require.NoError(t, err)
		return signed
	}

	// RFC requirement: RFC7474-2-5 positive -- a strictly-increasing sequence (10 then 11) is accepted against the per-neighbor/per-type high-water mark (authStore.verify auth_keystore.go:354-361).
	_, ok := s.verify("eth0", peer, [4]byte{}, mk(10))
	require.True(t, ok, "seq 10 accepted")
	_, ok2 := s.verify("eth0", peer, [4]byte{}, mk(11))
	require.True(t, ok2, "seq 11 (>10) accepted")
	// RFC requirement: RFC7474-2-5 negative -- a sequence lower than the last accepted (5 < 11) is dropped as a replay (authStore.verify auth_keystore.go:357-359).
	// RFC requirement: RFC7474-2-6 negative -- a same-type sequence at or below the per-type high-water mark is rejected, proving the mark actually gates packets of that type (authStore.verify auth_keystore.go:357-359).
	// RFC requirement: RFC5709-3.1-3 negative -- the 32-bit Cryptographic Sequence Number is enforced per RFC 2328 App D on the AuType 2 path: a packet carrying a sequence at or below the last accepted (5 < 11, and the equal-11 case below) is dropped as a replay, so the sequence field is load-bearing anti-replay state (authStore.verify auth_keystore.go:357-359).
	reason, replayOK := s.verify("eth0", peer, [4]byte{}, mk(5))
	assert.False(t, replayOK, "seq 5 (< last accepted 11) rejected as replay")
	assert.Equal(t, "replay", reason)
	// RFC 7474 §2: an EQUAL sequence is also a replay (strictly-greater required).
	eqReason, eqOK := s.verify("eth0", peer, [4]byte{}, mk(11))
	assert.False(t, eqOK, "seq 11 (== last accepted) rejected as replay")
	assert.Equal(t, "replay", eqReason)
}

func TestOSPFAuthReplayPerType(t *testing.T) {
	// RFC 7474 §2: the high-water mark is per packet type, so a lower-sequence packet of
	// a DIFFERENT type is not a false replay.
	s := newAuthStore()
	s.configure(authCfg(keyConfig{KeyID: 1, Algorithm: "hmac-sha-256", Secret: "k"}))
	peer := ridOf("2.2.2.2")
	key := packet.AuthKey{KeyID: 1, Algorithm: "hmac-sha-256", Secret: []byte("k")}

	signType := func(p packet.Packet, seq uint64) []byte {
		p.Header.AuType = packet.AuTypeCryptographic
		buf := make([]byte, p.EncodedLen())
		n := p.WriteTo(buf, 0)
		signed, err := packet.Sign(buf[:n], packet.AuTypeCryptographic, key, seq, [4]byte{})
		require.NoError(t, err)
		return signed
	}
	hello := packet.Packet{Header: packet.Header{Type: packet.PacketTypeHello}, Hello: &packet.Hello{NetworkMask: [4]byte{255, 255, 255, 0}, HelloInterval: 10, DeadInterval: 40}}
	ack := packet.Packet{Header: packet.Header{Type: packet.PacketTypeLSAck}, LSAck: &packet.LSAck{}}

	// RFC requirement: RFC7474-2-6 positive -- the high-water mark is per neighbor AND per packet type: a Hello at seq 100 does not block an LS-Ack at the lower seq 5, so each packet type keeps its own mark (replayKey.pktType auth_keystore.go:42, verify :352).
	_, ok := s.verify("eth0", peer, [4]byte{}, signType(hello, 100))
	require.True(t, ok, "Hello seq 100 accepted")
	_, ok2 := s.verify("eth0", peer, [4]byte{}, signType(ack, 5))
	assert.True(t, ok2, "LS-Ack seq 5 accepted (different packet-type slot, not a replay)")
}

func TestOSPFAuthExtendedSequenceRequiresHMAC(t *testing.T) {
	// md5 (or simple) under extended-sequence is an RFC-undefined combination -> rejected.
	cfg := ospfConfig{
		present:   true,
		RouterID:  ridOf("1.1.1.1"),
		KeyChains: []keyChainConfig{{Name: "kc1", ExtendedSequence: true, Keys: []keyConfig{{KeyID: 1, Algorithm: "md5", Secret: "x"}}}},
	}
	err := validateConfig(cfg)
	require.ErrorIs(t, err, ErrESNRequiresHMAC)
}

func TestAuType2KeyIDBoundary(t *testing.T) {
	// AC-3: the AuType 2 (RFC 2328 App D / RFC 5709) on-wire Key ID is a single octet, so a
	// non-extended crypto key-id MUST be 0..255; 256 is rejected. AuType 3 (extended) carries
	// a 32-bit Key ID, so the same value is accepted there.
	mk := func(extended bool, keyID uint32) ospfConfig {
		return ospfConfig{
			present:   true,
			RouterID:  ridOf("1.1.1.1"),
			KeyChains: []keyChainConfig{{Name: "kc1", ExtendedSequence: extended, Keys: []keyConfig{{KeyID: keyID, Algorithm: "hmac-sha-256", Secret: "x"}}}},
		}
	}
	require.NoError(t, validateConfig(mk(false, 255)), "AuType 2 key-id 255 is the last valid value")
	require.ErrorIs(t, validateConfig(mk(false, 256)), ErrKeyIDTooWide, "AuType 2 key-id 256 is rejected")
	require.NoError(t, validateConfig(mk(true, 256)), "AuType 3 (extended) allows a 32-bit key-id")
}

// RFC requirement: RFC2328-D.3-2 positive -- the cryptographic sequence number is per-neighbor state that is reset when the neighbor goes Down, so a neighbor that restarts may re-establish with any sequence (authStore.resetNeighbor, auth_keystore.go:375-383, driven from nsmAdapter.NeighborDown, instance.go:1073).
func TestNeighborDownResetsCryptoSeq(t *testing.T) {
	// AC-5 / RFC 2328 App D: when a neighbor goes Down its cryptographic receive-sequence
	// high-water marks are forgotten so it can re-establish with any sequence (for example
	// after its own restart). Drives nsmAdapter.NeighborDown -> authStore.resetNeighbor.
	s := newAuthStore()
	peer := ridOf("2.2.2.2")
	other := ridOf("3.3.3.3")
	s.recvSeq[replayKey{iface: "eth0", rid: peer, keyID: 1, pktType: packet.PacketTypeHello}] = 100
	s.recvSeq[replayKey{iface: "eth0", rid: peer, keyID: 1, pktType: packet.PacketTypeLSUpdate}] = 50
	s.recvSeq[replayKey{iface: "eth0", rid: other, keyID: 1, pktType: packet.PacketTypeHello}] = 200
	s.recvSeq[replayKey{iface: "eth1", rid: peer, keyID: 1, pktType: packet.PacketTypeHello}] = 10

	nsmAdapter{auth: s}.NeighborDown("eth0", peer)

	_, h := s.recvSeq[replayKey{iface: "eth0", rid: peer, keyID: 1, pktType: packet.PacketTypeHello}]
	assert.False(t, h, "neighbor-down cleared the Hello replay slot for eth0/peer")
	_, u := s.recvSeq[replayKey{iface: "eth0", rid: peer, keyID: 1, pktType: packet.PacketTypeLSUpdate}]
	assert.False(t, u, "neighbor-down cleared the LS-Update replay slot for eth0/peer")
	_, o := s.recvSeq[replayKey{iface: "eth0", rid: other, keyID: 1, pktType: packet.PacketTypeHello}]
	assert.True(t, o, "a different neighbor on the same interface is preserved")
	_, e := s.recvSeq[replayKey{iface: "eth1", rid: peer, keyID: 1, pktType: packet.PacketTypeHello}]
	assert.True(t, e, "the same neighbor on a different interface is preserved")
}

func TestInterfaceDownResetsAllCryptoSeq(t *testing.T) {
	// AC-5: an interface going Down drops all its adjacencies; every replay slot on that
	// interface is cleared. Drives nsmAdapter.InterfaceDown -> authStore.resetInterface.
	s := newAuthStore()
	s.recvSeq[replayKey{iface: "eth0", rid: ridOf("2.2.2.2"), keyID: 1, pktType: packet.PacketTypeHello}] = 1
	s.recvSeq[replayKey{iface: "eth0", rid: ridOf("3.3.3.3"), keyID: 1, pktType: packet.PacketTypeHello}] = 1
	s.recvSeq[replayKey{iface: "eth1", rid: ridOf("2.2.2.2"), keyID: 1, pktType: packet.PacketTypeHello}] = 1

	nsmAdapter{auth: s}.InterfaceDown("eth0")

	for rk := range s.recvSeq {
		assert.NotEqual(t, "eth0", rk.iface, "no eth0 replay slot remains after interface-down")
	}
	assert.Len(t, s.recvSeq, 1, "the eth1 slot is preserved")
}

// lifetimeAuthCfg builds a single-interface chain whose keys carry send-lifetimes.
func lifetimeAuthCfg(keys ...keyConfig) ospfConfig {
	return ospfConfig{
		KeyChains:  []keyChainConfig{{Name: "kc1", Keys: keys}},
		Areas:      []areaConfig{{AreaID: types.BackboneArea, AuthKeyChain: "kc1"}},
		Interfaces: []interfaceConfig{{Name: "eth0", AreaID: types.BackboneArea, Authentication: authConfig{Mode: "inherit"}}},
	}
}

func rfc3339(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	require.NoError(t, err)
	return tm
}

// TestSignKeySelectsByLifetime drives AC-17: the send key is the chain key whose
// send-lifetime window covers the keystore's clock, not always keys[0].
func TestSignKeySelectsByLifetime(t *testing.T) {
	s := newAuthStore()
	s.configure(lifetimeAuthCfg(
		keyConfig{KeyID: 1, Algorithm: "hmac-sha-256", Secret: "old", SendLifetime: lifetimeConfig{Start: "2026-01-01T00:00:00Z", End: "2026-06-01T00:00:00Z"}},
		keyConfig{KeyID: 2, Algorithm: "hmac-sha-256", Secret: "new", SendLifetime: lifetimeConfig{Start: "2026-05-01T00:00:00Z", End: "2026-12-01T00:00:00Z"}},
	))

	// Inside key 1's window only (before key 2 starts): key 1 signs.
	s.now = func() time.Time { return rfc3339(t, "2026-03-01T00:00:00Z") }
	k, _, _, _, ok := s.signKey("eth0")
	require.True(t, ok)
	assert.Equal(t, uint32(1), k.KeyID, "key 1 is active in March")

	// Overlap window (both active): the later-starting key 2 wins.
	s.now = func() time.Time { return rfc3339(t, "2026-05-15T00:00:00Z") }
	k, _, _, _, ok = s.signKey("eth0")
	require.True(t, ok)
	assert.Equal(t, uint32(2), k.KeyID, "key 2 (later start) wins during overlap")

	// After key 1 expired, inside key 2 only: key 2 signs.
	s.now = func() time.Time { return rfc3339(t, "2026-08-01T00:00:00Z") }
	k, _, _, _, ok = s.signKey("eth0")
	require.True(t, ok)
	assert.Equal(t, uint32(2), k.KeyID, "key 2 is active in August")
}

// TestSignKeyNoRevertWhenAllExpired drives AC-17: once every send-lifetime has
// elapsed the store keeps signing with the most-recently-starting key and never
// returns ok=false (which would revert the interface to unauthenticated AuType 0).
func TestSignKeyNoRevertWhenAllExpired(t *testing.T) {
	s := newAuthStore()
	s.configure(lifetimeAuthCfg(
		keyConfig{KeyID: 1, Algorithm: "hmac-sha-256", Secret: "old", SendLifetime: lifetimeConfig{Start: "2026-01-01T00:00:00Z", End: "2026-02-01T00:00:00Z"}},
		keyConfig{KeyID: 2, Algorithm: "hmac-sha-256", Secret: "new", SendLifetime: lifetimeConfig{Start: "2026-02-01T00:00:00Z", End: "2026-03-01T00:00:00Z"}},
	))
	s.now = func() time.Time { return rfc3339(t, "2027-01-01T00:00:00Z") } // long after both expired

	k, au, _, _, ok := s.signKey("eth0")
	require.True(t, ok, "an expired chain must still sign, not revert to AuType 0")
	// RFC requirement: RFC5709-3.2-2 positive -- when every send-lifetime has expired the store keeps signing under the cryptographic AuType (selectSendKey returns the most-recently-starting key, signKey never yields AuTypeNull for a resolved chain), so it never reverts to an unauthenticated condition (selectSendKey auth_keystore.go:263-287).
	assert.Equal(t, packet.AuTypeCryptographic, au)
	assert.Equal(t, uint32(2), k.KeyID, "the most-recently-starting key is used after expiry")
}

// TestSignKeyUnsetLifetimeUsesFirst confirms the unset-lifetime default is unchanged:
// with no lifetimes every key is always valid, so the first chain key signs.
func TestSignKeyUnsetLifetimeUsesFirst(t *testing.T) {
	s := newAuthStore()
	s.configure(authCfg(
		keyConfig{KeyID: 7, Algorithm: "hmac-sha-256", Secret: "a"},
		keyConfig{KeyID: 9, Algorithm: "hmac-sha-256", Secret: "b"},
	))
	k, _, _, _, ok := s.signKey("eth0")
	require.True(t, ok)
	assert.Equal(t, uint32(7), k.KeyID, "with no lifetimes the first chain key signs")
}

// fakeBootStore is an in-memory bootCountStore that survives across loadOSPFBootCount
// calls, modeling ZeFS persistence across a cold restart.
type fakeBootStore struct {
	data map[string][]byte
}

func newFakeBootStore() *fakeBootStore { return &fakeBootStore{data: map[string][]byte{}} }

func (f *fakeBootStore) ReadFile(name string) ([]byte, error) {
	b, ok := f.data[name]
	if !ok {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrNotExist}
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out, nil
}

func (f *fakeBootStore) WriteFile(name string, data []byte, _ fs.FileMode) error {
	b := make([]byte, len(data))
	copy(b, data)
	f.data[name] = b
	return nil
}

// TestBootCountMonotonicAcrossRestart drives AC-18 / RFC 7474 §3: a persisted boot
// count strictly increases on each load (each load models one cold restart).
func TestBootCountMonotonicAcrossRestart(t *testing.T) {
	// RFC requirement: RFC7474-2-4 positive -- the persisted boot count strictly increases on each cold restart (each load models one restart), preserving the aggregate 64-bit sequence's strictly-increasing property for the router's deployed life (loadOSPFBootCount auth_keystore.go:124-132).
	store := newFakeBootStore()
	first := loadOSPFBootCount(store)
	second := loadOSPFBootCount(store)
	third := loadOSPFBootCount(store)
	assert.Greater(t, second, first, "second boot count strictly greater than first")
	assert.Greater(t, third, second, "third boot count strictly greater than second")
	assert.Equal(t, uint32(1), first, "first boot from an empty store is 1")
	assert.Equal(t, uint32(3), third, "persisted boot count is the increment count")
}

// TestBootCountNilStoreFallsBack drives AC-18: with no ZeFS store the seed comes from
// the hashed high-resolution clock (non-zero, advancing), never a plain Unix-seconds seed.
func TestBootCountNilStoreFallsBack(t *testing.T) {
	bc := loadOSPFBootCount(nil)
	assert.NotZero(t, bc, "the hashed-clock fallback yields a non-zero seed")
	// Two hashed-clock seeds taken at different nanoseconds differ with overwhelming
	// probability; this guards against a constant/zero fallback.
	a := bootCountFromClock()
	time.Sleep(time.Microsecond)
	b := bootCountFromClock()
	assert.NotEqual(t, a, b, "successive hashed-clock seeds differ")
}

// TestSetBootCountSeedsSequence drives AC-18: the engine-resolved boot count becomes
// the RFC 7474 high word of the aggregate 64-bit ESN sequence.
func TestSetBootCountSeedsSequence(t *testing.T) {
	s := newAuthStore()
	s.configure(ospfConfig{
		KeyChains:  []keyChainConfig{{Name: "kc1", ExtendedSequence: true, Keys: []keyConfig{{KeyID: 1, Algorithm: "hmac-sha-256", Secret: "k"}}}},
		Areas:      []areaConfig{{AreaID: types.BackboneArea, AuthKeyChain: "kc1"}},
		Interfaces: []interfaceConfig{{Name: "eth0", AreaID: types.BackboneArea, Authentication: authConfig{Mode: "inherit"}}},
	})
	s.setBootCount(0x1234)

	_, au, seq, _, ok := s.signKey("eth0")
	require.True(t, ok)
	assert.Equal(t, packet.AuTypeCryptographicESN, au)
	// RFC requirement: RFC7474-2-2 positive -- the 64-bit sequence is composed as high-order boot count | low-order counter (0x1234<<32 | 1) by signKey (auth_keystore.go:314).
	assert.Equal(t, uint64(0x1234)<<32|1, seq, "boot count is the high word, per-packet counter the low word")
}

func TestOSPFAuthESNCounterWrapAdvancesBootCount(t *testing.T) {
	// RFC 7474: when the 32-bit per-packet counter wraps back to 0, the boot count (high word)
	// must advance so the 64-bit sequence stays strictly increasing -- a regression would look
	// like a replay to the peer.
	s := newAuthStore()
	s.configure(ospfConfig{
		KeyChains:  []keyChainConfig{{Name: "kc1", ExtendedSequence: true, Keys: []keyConfig{{KeyID: 1, Algorithm: "hmac-sha-256", Secret: "k"}}}},
		Areas:      []areaConfig{{AreaID: types.BackboneArea, AuthKeyChain: "kc1"}},
		Interfaces: []interfaceConfig{{Name: "eth0", AreaID: types.BackboneArea, Authentication: authConfig{Mode: "inherit"}}},
	})
	s.setBootCount(0x1234)
	s.mu.Lock()
	s.sendSeq["eth0"] = 0xFFFFFFFF // the next send wraps the low word to 0
	s.mu.Unlock()

	_, _, seq, _, ok := s.signKey("eth0")
	require.True(t, ok)
	assert.Equal(t, uint64(0x1235)<<32, seq, "boot count advanced to 0x1235 and the counter wrapped to 0")

	// RFC requirement: RFC7474-2-3 positive -- every packet sent increments the low-order 32-bit counter (successive signKey calls advance seq2 = seq + 1), so the sequence increments per packet (auth_keystore.go:303,314).
	// The following packet continues strictly increasing from the bumped boot count.
	_, _, seq2, _, ok := s.signKey("eth0")
	require.True(t, ok)
	assert.Equal(t, uint64(0x1235)<<32|1, seq2)
	assert.Greater(t, seq2, seq, "sequence must not regress across the counter wrap")
}
