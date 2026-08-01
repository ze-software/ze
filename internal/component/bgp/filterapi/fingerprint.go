// Design: plan/spec-wire-edit-5-fanout-dedup.md -- fingerprint the edit set, confirm by equality
// Overview: filterapi.go -- ModAccumulator, whose state this encodes
// Related: editset.go -- the rebuild plan the digest predicts the output of
package filterapi

import (
	"bytes"
	"encoding/binary"
	"slices"
)

// Deduplicating a fan-out needs to answer one question: will these two
// destinations produce the same bytes?
//
// The dangerous way to answer it is a hash. Two destinations whose edit sets
// hash alike are NOT known to be equal, and acting on a hash alone sends one
// peer another peer's UPDATE -- a wrong next-hop, a foreign cluster's
// CLUSTER_LIST, a community the policy existed to strip. The peer cannot detect
// it and neither can we.
//
// So the question is answered in two steps with different authority. A DIGEST is
// a canonical byte encoding of everything in the accumulator that can change the
// rebuild's output. A FINGERPRINT is a 64-bit hash of that digest, and it is a
// hint: it says where to look, never what to do. Sharing is authorized only by
// EditDigestEqual, a byte comparison of the two digests.
//
// One byte string feeds both roles, and that is the point. The failure this
// design exists to prevent is a field that is compared but not hashed, or hashed
// but not compared: a later edit adds something to the rebuild, forgets one of
// the two, and two destinations that need different bytes quietly share one. It
// cannot happen here, because there is no second list to forget -- the hash
// hashes the digest and the comparison compares the digest.

// EditDigestMax bounds one digest.
//
// The operation values a digest encodes are peer-influenceable: a community list
// reaches 65535 octets, and hashing that per destination would cost more than
// the rebuild it exists to skip. An edit set whose digest would exceed this
// ceiling refuses to be digested, so the destination materializes on its own.
// That is a bounded loss of an optimization, never a loss of correctness.
const EditDigestMax = 2048

// Digest value markers. They exist so an operation's value can never be confused
// with the next operation's, and so a generator-produced value is never confused
// with a pre-built one.
const (
	digestValueBuf = 'B'
	digestValueGen = 'G'
)

// AppendEditDigest appends a canonical encoding of every part of the accumulator
// that can change the bytes buildModifiedPayload produces, and reports whether a
// digest could be taken at all.
//
// It reports false, and appends nothing, when there is nothing to share (an
// accumulator with no modification has no rebuild), when a generator cannot
// answer its length, or when the encoding would exceed EditDigestMax. Every one
// of those is a refusal to dedup, which costs a rebuild and never a wrong byte
// (ai/rules/fail-closed-guards.md).
//
// The encoding is self-delimiting: every variable-length value is preceded by
// its length, so two different edit sets cannot encode to one byte string by
// moving bytes across an operation boundary.
//
// A GENERATOR's contribution is its OUTPUT, not its identity. The forward rails
// hoist one wireu.ASPathEdit above the destination loop and re-record it for
// every peer, so the generator POINTER is identical for destinations whose
// AS_PATH differs completely. Fingerprinting the pointer would equate them. This
// asks the generator to write its bytes into the digest instead, which is the
// same contract the rebuild's size query uses.
func (a *ModAccumulator) AppendEditDigest(dst []byte) ([]byte, bool) {
	if !a.HasModifications() && !a.withdraw {
		return dst, false
	}

	start := len(dst)

	// The rebuild reads GroupedOps, so the digest is taken over that same order.
	// Grouping is idempotent and in place, so this also spares the rebuild its
	// own sort on the miss path.
	ops := a.GroupedOps()

	var flags byte
	if a.withdraw {
		flags |= 1
	}
	dst = append(dst, flags)
	dst = appendDigestOptional(dst, a.nlriRewrite)
	dst = appendDigestOptional(dst, a.withdrawnRewrite)
	dst = appendDigestUint32(dst, uint32(len(ops))) //nolint:gosec // G115: bounded by the accumulator

	for i := range ops {
		op := &ops[i]
		dst = append(dst, op.Code, op.Action)
		if g := a.digestGenerator(op.GenIdx); g != nil {
			n := g.GenLen()
			if n < 0 || len(dst)-start+n > EditDigestMax {
				return dst[:start], false
			}
			dst = append(dst, digestValueGen)
			dst = appendDigestUint32(dst, uint32(n)) //nolint:gosec // G115: bounded above
			// Grow by exactly n, then let the generator fill that window. This is
			// the generator's own size-then-write contract, used here for the
			// same reason the rebuild uses it: the value exists in no buffer, so
			// staging it in a scratch slice would copy it twice
			// (ai/rules/buffer-first.md).
			dst = slices.Grow(dst, n)
			base := len(dst)
			dst = dst[:base+n]
			if g.GenWrite(dst, base) != n {
				// A generator whose write disagrees with its length is the one
				// case the rebuild also refuses. Refusing here keeps the digest
				// from describing an attribute nobody will emit.
				return dst[:start], false
			}
			continue
		}
		if len(dst)-start+len(op.Buf) > EditDigestMax {
			return dst[:start], false
		}
		dst = append(dst, digestValueBuf)
		dst = appendDigestUint32(dst, uint32(len(op.Buf))) //nolint:gosec // G115: bounded above
		dst = append(dst, op.Buf...)
	}

	if len(dst)-start > EditDigestMax {
		return dst[:start], false
	}
	return dst, true
}

// digestGenerator resolves a one-based GenIdx exactly as EditSet.generator does
// during the rebuild: 0 and any out-of-range index read as absent, so the digest
// falls back to Buf in precisely the cases the rebuild does.
func (a *ModAccumulator) digestGenerator(idx uint8) AttrGenerator {
	if idx == 0 || int(idx) > len(a.gens) {
		return nil
	}
	return a.gens[idx-1]
}

// appendDigestOptional encodes a slice whose ABSENCE means something different
// from its emptiness.
//
// A nil NLRI rewrite keeps every prefix; a zero-length one drops every prefix
// (buildModifiedPayload). Collapsing the two would let opposite outcomes share a
// rebuild, so presence is one byte and length is four more.
func appendDigestOptional(dst, b []byte) []byte {
	if b == nil {
		return append(dst, 0)
	}
	dst = append(dst, 1)
	dst = appendDigestUint32(dst, uint32(len(b))) //nolint:gosec // G115: bounded by the 2-octet wire fields these rewrites become
	return append(dst, b...)
}

// appendDigestUint32 appends a big-endian length prefix.
func appendDigestUint32(dst []byte, v uint32) []byte {
	return append(dst, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

// EditFingerprint hashes a digest to a 64-bit hint.
//
// Eight bytes per round, not one. The obvious choice here is FNV-1a, and it was
// the first one: it is one multiply per BYTE, and each multiply depends on the
// previous, so a 48-byte digest is a 48-deep chain of 3-cycle multiplies. That
// cost ~35ns on every destination of every fan-out -- hit or miss -- which is a
// tenth of the rebuild the whole feature exists to skip. Reading eight bytes per
// round makes the same digest six multiplies instead of forty-eight.
//
// The strength of this hash is deliberately NOT load-bearing. It selects
// candidates; EditDigestEqual authorizes them. A collision costs one wasted byte
// comparison and nothing else, which is why a mixing function rather than a
// vetted hash is the right amount of machinery here.
func EditFingerprint(digest []byte) uint64 {
	const prime = 0x9E3779B185EBCA87
	h := uint64(len(digest))*prime + 1
	for len(digest) >= 8 {
		h ^= binary.LittleEndian.Uint64(digest)
		h *= prime
		h ^= h >> 29
		digest = digest[8:]
	}
	// The tail, packed into one word so a short digest still mixes every byte.
	var tail uint64
	for i, c := range digest {
		tail |= uint64(c) << (8 * i) //nolint:gosec // i < 8 by the loop above
	}
	h ^= tail
	h *= prime
	h ^= h >> 32
	return h
}

// EditDigestEqual reports whether two digests describe the same edit set.
//
// This is the authorization, and it has no fast path. A caller MUST NOT share a
// rebuild on a fingerprint match alone: the fingerprint says where to look and
// this says whether to act (ai/rules/fail-closed-guards.md).
func EditDigestEqual(a, b []byte) bool {
	return bytes.Equal(a, b)
}
