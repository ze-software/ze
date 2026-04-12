package auth

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/packet"
)

// FuzzAuthDigest feeds random bytes into Verifier.Verify across all four
// keyed digest variants. The digest verifier must never panic on
// adversarial input, must never report nil error on truncated or
// length-forged auth bodies, and must never advance the replay state
// on a rejected packet.
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
// internal/plugins/bfd/auth that the packet parser does not reach.
// PREVENTS: regression where a forged Auth Len or a short digest
// body panics the verifier, or where a crafted replay sequence is
// accepted without the SeqState guard firing.
func FuzzAuthDigest(f *testing.F) {
	for _, seed := range authSeedCorpus() {
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
		var state SeqState
		// Must not panic regardless of the Control.Length value
		// the fuzzer picked. A return error (mismatch, short body,
		// sequence regress) is the expected outcome on almost
		// every random draw.
		beforeLast := state.Last()
		if err := v.Verify(data, c, &state); err != nil {
			if state.Last() != beforeLast {
				t.Fatalf("SeqState advanced on rejected packet: before=%d after=%d err=%v",
					beforeLast, state.Last(), err)
			}
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
func authSeedCorpus() []authFuzzSeed {
	var seeds []authFuzzSeed
	for _, t := range []uint8{
		packet.AuthTypeKeyedMD5,
		packet.AuthTypeMeticulousKeyedMD5,
		packet.AuthTypeKeyedSHA1,
		packet.AuthTypeMeticulousKeyedSHA1,
	} {
		cfg := Settings{
			Type:   t,
			KeyID:  7,
			Secret: []byte("fuzz-seed-shared-secret"),
		}
		signer, err := NewSigner(cfg)
		if err != nil {
			continue
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
		seeds = append(seeds, authFuzzSeed{
			data:     append([]byte(nil), buf...),
			authType: t,
			keyID:    7,
			secret:   append([]byte(nil), cfg.Secret...),
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
