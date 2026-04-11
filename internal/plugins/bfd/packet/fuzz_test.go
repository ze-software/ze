package packet

import (
	"testing"
)

// FuzzParseControl feeds random bytes into ParseControl. The decoder
// must never panic on any input and must never report a nil error on a
// structurally invalid packet. Seed corpus covers every field boundary
// plus a handful of hand-crafted malformed packets.
//
// VALIDATES: codec robustness under adversarial input.
// PREVENTS: regression where a malformed packet panics or escapes the
// RFC 5880 §6.8.6 reception-check ladder.
func FuzzParseControl(f *testing.F) {
	seeds := seedCorpus()
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		c, n, err := ParseControl(data)
		if err != nil {
			return
		}
		if n != MandatoryLen {
			t.Fatalf("accepted packet with consumed=%d, want %d", n, MandatoryLen)
		}
		// RFC 5880 Section 6.8.6 invariants of an accepted packet.
		if c.Version != Version {
			t.Fatalf("accepted bad version %d", c.Version)
		}
		if c.DetectMult == 0 {
			t.Fatalf("accepted zero DetectMult")
		}
		if c.Multipoint {
			t.Fatalf("accepted M bit set")
		}
		if c.MyDiscriminator == 0 {
			t.Fatalf("accepted zero MyDiscriminator")
		}
		minLen := uint8(MandatoryLen)
		if c.Auth {
			minLen = MandatoryLen + 2
		}
		if c.Length < minLen {
			t.Fatalf("accepted length %d below minimum %d", c.Length, minLen)
		}
		if int(c.Length) > len(data) {
			t.Fatalf("accepted length %d exceeds buffer %d", c.Length, len(data))
		}
		// Round-trip only packets whose Length fits entirely in the
		// mandatory section. Auth-bearing packets carry a trailing
		// section that WriteTo does not emit, so re-encoding them
		// into a mandatory-only buffer would lose data and produce
		// an ErrLengthOverBuffer on re-parse.
		if c.Length == MandatoryLen && !c.Auth {
			pb := Acquire()
			buf := pb.Data()
			c.WriteTo(buf, 0)
			c2, _, err := ParseControl(buf[:MandatoryLen])
			Release(pb)
			if err != nil {
				t.Fatalf("round-trip ParseControl failed: %v", err)
			}
			if c2 != c {
				t.Fatalf("round-trip mismatch:\ngot:  %+v\nwant: %+v", c2, c)
			}
		}
	})
}

// FuzzParseAuth feeds random bytes into ParseAuth. The parser must never
// panic and must never return a Body slice that escapes the input bounds.
//
// VALIDATES: ParseAuth bounds on attacker-controlled Auth Len.
// PREVENTS: regression where a forged Auth Len value produces a
// slice-out-of-bounds panic or a Body that over-reads the input.
func FuzzParseAuth(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x00})
	f.Add([]byte{AuthTypeKeyedMD5, AuthLenKeyedMD5})
	f.Add([]byte{AuthTypeKeyedSHA1, AuthLenKeyedSHA1})
	f.Add([]byte{AuthTypeKeyedMD5, 0xFF, 0x00, 0x00})
	f.Fuzz(func(t *testing.T, data []byte) {
		h, err := ParseAuth(data)
		if err != nil {
			return
		}
		if int(h.Len) > len(data) {
			t.Fatalf("accepted auth len %d past data %d", h.Len, len(data))
		}
		if h.Len < AuthHeaderLen {
			t.Fatalf("accepted auth len %d below header", h.Len)
		}
		want := int(h.Len) - AuthHeaderLen
		if len(h.Body) != want {
			t.Fatalf("body length %d, want %d", len(h.Body), want)
		}
	})
}

// seedCorpus returns a representative set of valid and invalid BFD
// Control packets for the fuzzer.
func seedCorpus() [][]byte {
	good := Control{
		Version:                   1,
		State:                     StateUp,
		DetectMult:                3,
		Length:                    MandatoryLen,
		MyDiscriminator:           1,
		DesiredMinTxInterval:      1_000_000,
		RequiredMinRxInterval:     1_000_000,
		RequiredMinEchoRxInterval: 0,
	}
	b := make([]byte, MandatoryLen)
	good.WriteTo(b, 0)

	mutate := func(f func([]byte)) []byte {
		c := cloneBytes(b)
		f(c)
		return c
	}

	return [][]byte{
		cloneBytes(b),
		mutate(func(c []byte) { c[0] = 3 << 5 }),          // bad version
		mutate(func(c []byte) { c[2] = 0 }),               // zero DetectMult
		mutate(func(c []byte) { c[1] |= FlagMultipoint }), // multipoint set
		mutate(func(c []byte) { // zero MyDiscriminator
			c[4], c[5], c[6], c[7] = 0, 0, 0, 0
		}),
		mutate(func(c []byte) { c[3] = MandatoryLen + 8 }), // length exceeds data
		b[:10], // truncated
		mutate(func(c []byte) { // A=1 with length below minimum
			c[1] |= FlagAuth
			c[3] = MandatoryLen
		}),
	}
}

func cloneBytes(in []byte) []byte {
	out := make([]byte, len(in))
	copy(out, in)
	return out
}
