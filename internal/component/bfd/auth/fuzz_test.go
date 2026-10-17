package auth

import (
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/packet"
)

// FuzzAuthDigest feeds random bytes into Verifier.Verify across all four
// keyed digest variants. The digest verifier must never panic on
// adversarial input, must never report nil error on truncated or
// length-forged auth bodies, and must never move an established replay
// floor on a rejected packet.
//
// A first packet is the one exception, by owner decision D-14
// (2026-09-27): RFC 5880 Sections 6.7.3 and 6.7.4 list "bfd.RcvAuthSeq
// MUST be set to the value of the received Sequence Number field" before
// the digest comparison, so a first packet that passes the sequence step
// and then fails the digest has still seeded the floor, to its own
// sequence number and to nothing else. Each input therefore runs twice:
// against a fresh SeqState, where a rejection may only seed the floor to
// the packet's sequence, and against a SeqState already initialized,
// where a rejection must leave the floor exactly where it was.
//
// The seed corpus is synthesized from in-test round-trip Sign/Verify
// pairs for each auth type so the fuzzer starts from known-good bytes
// and mutates outward. A second set of hand-forged cases drives the
// bounds-check paths (length overflow, zero length, header-only
// fragments).
//
// VALIDATES: spec-bfd-5b auth-body fuzz coverage. The existing
// packet/fuzz_test.go exercises ParseControl/ParseAuth; this target
// adds coverage for the digest verification step inside
// internal/component/bfd/auth that the packet parser does not reach.
// PREVENTS: regression where a forged Auth Len or a short digest
// body panics the verifier, or where a crafted replay sequence is
// accepted without the SeqState guard firing.
func FuzzAuthDigest(f *testing.F) {
	for _, seed := range authSeedCorpus(f) {
		f.Add(seed.data, seed.authType, seed.keyID, seed.secret)
	}
	f.Fuzz(func(t *testing.T, data []byte, authType, keyID uint8, secret []byte) {
		cfg := Settings{
			Type:   authType,
			KeyID:  keyID,
			Secret: secret,
		}
		v, err := NewVerifier(cfg)
		if err != nil {
			// Unsupported type / invalid key length is a legitimate
			// rejection path, not a fuzz finding.
			return
		}
		c := packet.Control{Auth: true, Length: byte(minInt(len(data), 255))}
		// The sequence window is 3*DetectMult wide, so a zero DetectMult
		// would stop every packet at the sequence step on initialized state.
		// Take it from the packet's own header byte, as the receive path does.
		if len(data) > 2 {
			c.DetectMult = data[2]
		}
		// The sequence number the packet carries, where it is long enough to
		// carry one: D-14 allows a rejected first packet to seed exactly this.
		seq, hasSeq := uint32(0), len(data) >= packet.MandatoryLen+8
		if hasSeq {
			seq = binary.BigEndian.Uint32(data[packet.MandatoryLen+4:])
		}

		// Fresh state. Must not panic regardless of the Control.Length value
		// the fuzzer picked. A return error (mismatch, short body, sequence
		// regress) is the expected outcome on almost every random draw.
		var fresh SeqState
		if err := v.Verify(data, c, &fresh); err != nil && fresh.Initialized() {
			if !hasSeq || fresh.Last() != seq {
				t.Fatalf("rejected first packet set the floor to %d, not its own sequence %d (has sequence %v): err=%v",
					fresh.Last(), seq, hasSeq, err)
			}
		}

		// Initialized state, with the floor just behind the packet's sequence
		// so the sequence step admits it and the digest step decides.
		var known SeqState
		known.Advance(seq - 1)
		before := known.Last()
		if err := v.Verify(data, c, &known); err != nil && known.Last() != before {
			t.Fatalf("SeqState advanced on rejected packet: before=%d after=%d err=%v",
				before, known.Last(), err)
		}
	})
}

// authFuzzSeed is one row in the seed corpus.
type authFuzzSeed struct {
	data     []byte
	authType uint8
	keyID    uint8
	secret   []byte
}

// authSeedCorpus returns a representative set of inputs for FuzzAuthDigest.
// Round-trips generate a valid body for every supported auth type; the
// hand-crafted rows poke the length-bounds paths.
func authSeedCorpus(f *testing.F) []authFuzzSeed {
	var seeds []authFuzzSeed
	for _, t := range []uint8{
		packet.AuthTypeKeyedMD5,
		packet.AuthTypeMeticulousKeyedMD5,
		packet.AuthTypeKeyedSHA1,
		packet.AuthTypeMeticulousKeyedSHA1,
	} {
		cfg := Settings{
			Type:  t,
			KeyID: 7,
			// 16 bytes: the Keyed MD5 maximum, so every digest type accepts it.
			Secret: []byte("fuzz-seed-secret"),
		}
		signer, err := NewSigner(cfg)
		if err != nil {
			// A seed that cannot be built leaves the fuzzer without its
			// known-good starting point, so it is a test defect, not a skip.
			f.Fatalf("seed signer for auth type %d: %v", t, err)
		}
		buf := make([]byte, packet.MandatoryLen+signer.BodyLen())
		// Lay down a minimal Control section so the digest covers
		// well-formed mandatory bytes.
		c := packet.Control{
			Version:               packet.Version,
			State:                 packet.StateDown,
			Auth:                  true,
			DetectMult:            3,
			Length:                byte(packet.MandatoryLen + signer.BodyLen()),
			MyDiscriminator:       0x01020304,
			DesiredMinTxInterval:  1_000_000,
			RequiredMinRxInterval: 1_000_000,
		}
		c.WriteTo(buf, 0)
		signer.Sign(buf, packet.MandatoryLen, 42)
		// The second seed is the same packet checked under another key: the
		// sequence step admits it and the digest step rejects it, the D-14
		// first-packet path.
		seeds = append(seeds, authFuzzSeed{
			data:     append([]byte(nil), buf...),
			authType: t,
			keyID:    7,
			secret:   append([]byte(nil), cfg.Secret...),
		}, authFuzzSeed{
			data:     append([]byte(nil), buf...),
			authType: t,
			keyID:    7,
			secret:   []byte("fuzz-other-key!!"),
		})
	}

	// Boundary / malformed rows. The fuzzer blends these with the
	// round-trip seeds, driving coverage into the length-check
	// and short-body branches that the happy path cannot reach.
	seeds = append(seeds,
		authFuzzSeed{data: []byte{}, authType: packet.AuthTypeKeyedMD5, keyID: 1, secret: []byte("k")},
		authFuzzSeed{data: []byte{0xFF}, authType: packet.AuthTypeKeyedSHA1, keyID: 1, secret: []byte("k")},
		authFuzzSeed{
			data:     make([]byte, packet.MandatoryLen),
			authType: packet.AuthTypeMeticulousKeyedSHA1,
			keyID:    255,
			secret:   []byte("short"),
		},
	)
	return seeds
}

// minInt returns the smaller of two ints (avoid clash with generic
// max in the session package for this minimal helper).
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
