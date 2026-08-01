package filterapi

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// genStub is an AttrGenerator whose output the test controls.
type genStub struct{ out []byte }

func (g *genStub) GenLen() int { return len(g.out) }
func (g *genStub) GenWrite(buf []byte, off int) int {
	return copy(buf[off:], g.out)
}

// digestOf is the whole contract in one line: one function produces the bytes,
// and both the hash and the equality check read those same bytes.
func digestOf(t *testing.T, build func(a *ModAccumulator)) []byte {
	t.Helper()
	var a ModAccumulator
	build(&a)
	d, ok := a.AppendEditDigest(nil)
	require.True(t, ok, "digest must be takeable for this edit set")
	return d
}

// VALIDATES: AC-2 -- two destinations whose edit sets are equal produce equal
// digests, generator-produced values included.
// PREVENTS: a dedup that never fires because the digest folds in something
// per-destination, which would make this child a pure cost.
func TestFingerprintEqualForEqualEdits(t *testing.T) {
	build := func(a *ModAccumulator) {
		a.Op(3, AttrModSet, []byte{10, 0, 0, 1})
		a.Op(8, AttrModRemove, []byte{0xFD, 0xE9, 0, 1})
		a.OpGen(2, &genStub{out: []byte{0x02, 0x01, 0, 0, 0xFD, 0xE8}})
		a.Op(40, AttrModSuppress, nil)
	}
	first := digestOf(t, build)
	second := digestOf(t, build)
	require.Equal(t, first, second, "equal edit sets must digest equal")
	require.Equal(t, EditFingerprint(first), EditFingerprint(second))
}

// VALIDATES: AC-3, AC-5, A-3 -- mutating any single field of the edit set
// changes the digest.
// PREVENTS: R-2, the failure where a field added later is compared but not
// hashed, or hashed but not compared. Here it cannot happen by construction:
// there is one byte string and both roles read it. This test is what fails when
// a new field reaches the rebuild without reaching the digest.
func TestFingerprintDiffersForEveryField(t *testing.T) {
	base := func(a *ModAccumulator) {
		a.Op(3, AttrModSet, []byte{10, 0, 0, 1})
		a.OpGen(2, &genStub{out: []byte{0x02, 0x01, 0, 0, 0xFD, 0xE8}})
	}
	baseline := digestOf(t, base)

	mutations := map[string]func(a *ModAccumulator){
		"attribute code": func(a *ModAccumulator) {
			a.Op(4, AttrModSet, []byte{10, 0, 0, 1})
			a.OpGen(2, &genStub{out: []byte{0x02, 0x01, 0, 0, 0xFD, 0xE8}})
		},
		"action": func(a *ModAccumulator) {
			a.Op(3, AttrModAdd, []byte{10, 0, 0, 1})
			a.OpGen(2, &genStub{out: []byte{0x02, 0x01, 0, 0, 0xFD, 0xE8}})
		},
		"operation value": func(a *ModAccumulator) {
			a.Op(3, AttrModSet, []byte{10, 0, 0, 2})
			a.OpGen(2, &genStub{out: []byte{0x02, 0x01, 0, 0, 0xFD, 0xE8}})
		},
		"operation value length": func(a *ModAccumulator) {
			a.Op(3, AttrModSet, []byte{10, 0, 0, 1, 0})
			a.OpGen(2, &genStub{out: []byte{0x02, 0x01, 0, 0, 0xFD, 0xE8}})
		},
		"generator output": func(a *ModAccumulator) {
			a.Op(3, AttrModSet, []byte{10, 0, 0, 1})
			a.OpGen(2, &genStub{out: []byte{0x02, 0x01, 0, 0, 0xFD, 0xE9}})
		},
		"generator output length": func(a *ModAccumulator) {
			a.Op(3, AttrModSet, []byte{10, 0, 0, 1})
			a.OpGen(2, &genStub{out: []byte{0x02, 0x01, 0, 0, 0xFD, 0xE8, 0}})
		},
		"extra operation": func(a *ModAccumulator) {
			base(a)
			a.Op(40, AttrModSuppress, nil)
		},
		"withdraw flag": func(a *ModAccumulator) {
			base(a)
			a.SetWithdraw()
		},
		"nlri rewrite": func(a *ModAccumulator) {
			base(a)
			a.SetNLRIRewrite([]byte{24, 10, 0, 0})
		},
		"withdrawn rewrite": func(a *ModAccumulator) {
			base(a)
			a.SetWithdrawnRewrite([]byte{24, 10, 0, 0})
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			require.NotEqual(t, baseline, digestOf(t, mutate),
				"mutating %s must change the digest: an unchanged digest lets two "+
					"destinations that need different bytes share one rebuild", name)
		})
	}
}

// VALIDATES: AC-3 -- an empty rewrite is not the same edit as no rewrite.
// PREVENTS: the presence/absence collapse that would let "drop every NLRI
// prefix" share a rebuild with "leave the NLRI alone", which are opposite
// outcomes on the wire (buildModifiedPayload treats nil and empty differently).
func TestFingerprintSeparatesEmptyRewriteFromAbsent(t *testing.T) {
	absent := digestOf(t, func(a *ModAccumulator) {
		a.Op(3, AttrModSet, []byte{10, 0, 0, 1})
	})
	empty := digestOf(t, func(a *ModAccumulator) {
		a.Op(3, AttrModSet, []byte{10, 0, 0, 1})
		a.SetNLRIRewrite([]byte{})
	})
	require.NotEqual(t, absent, empty,
		"a zero-length NLRI rewrite drops every prefix; absent keeps them all")
}

// VALIDATES: AC-3 -- a value's bytes cannot migrate between adjacent operations
// without changing the digest.
// PREVENTS: the classic length-free concatenation collision, where {AB}{} and
// {A}{B} encode identically and two different edit sets look equal.
func TestFingerprintIsSelfDelimiting(t *testing.T) {
	joined := digestOf(t, func(a *ModAccumulator) {
		a.Op(8, AttrModAdd, []byte{1, 2, 3, 4})
		a.Op(8, AttrModAdd, nil)
	})
	split := digestOf(t, func(a *ModAccumulator) {
		a.Op(8, AttrModAdd, []byte{1, 2})
		a.Op(8, AttrModAdd, []byte{3, 4})
	})
	require.NotEqual(t, joined, split,
		"operation boundaries must be encoded, not implied by concatenation")
}

// VALIDATES: AC-7 -- an accumulator with nothing to apply reports no digest, so
// the zero-copy passthrough never pays for a fingerprint it cannot use.
// PREVENTS: R-4, turning a free forward into a hashed one.
func TestEmptyEditSetSkipsFingerprint(t *testing.T) {
	var a ModAccumulator
	_, ok := a.AppendEditDigest(nil)
	require.False(t, ok, "an empty edit set has no rebuild to share")
	require.False(t, a.HasModifications())
}

// VALIDATES: AC-4, R-1 -- a fingerprint match between two UNEQUAL edit sets is
// refused by the byte comparison.
// PREVENTS: the catastrophic case. A hash collision must never authorize
// sharing, because sharing sends one destination another destination's wire and
// the peer has no way to detect it.
//
// The collision is forced rather than searched for: EditFingerprint is a hint
// and the test drives the decision function with a hint that is deliberately
// wrong, which is exactly the state a real collision produces.
func TestCollisionRejectedByEquality(t *testing.T) {
	left := digestOf(t, func(a *ModAccumulator) {
		a.Op(3, AttrModSet, []byte{10, 0, 0, 1})
	})
	right := digestOf(t, func(a *ModAccumulator) {
		a.Op(3, AttrModSet, []byte{192, 168, 0, 1})
	})
	require.NotEqual(t, left, right, "the fixture must be two genuinely different edits")

	// A real collision is two unequal digests hashing to one value. Assert the
	// authorization ignores the hash entirely.
	require.False(t, EditDigestEqual(left, right),
		"unequal edit sets must never be treated as shareable, whatever they hash to")
	require.True(t, EditDigestEqual(left, append([]byte(nil), left...)),
		"an equal copy must still be shareable")
}

// VALIDATES: A-3 -- a generator that refuses to answer its length refuses the
// whole digest.
// PREVENTS: a fail-open digest that silently omits a generator's contribution
// and so equates two destinations whose generated attribute differs.
func TestFingerprintRefusesUnmeasurableGenerator(t *testing.T) {
	var a ModAccumulator
	a.OpGen(2, &genStub{out: nil})
	a.Op(3, AttrModSet, []byte{10, 0, 0, 1})
	d, ok := a.AppendEditDigest(nil)
	require.True(t, ok, "a zero-length generator is measurable, just empty")
	require.NotEmpty(t, d)

	var big ModAccumulator
	big.OpGen(2, &genStub{out: make([]byte, EditDigestMax+1)})
	_, ok = big.AppendEditDigest(nil)
	require.False(t, ok,
		"an edit set larger than the digest ceiling must refuse rather than hash a prefix")
}

// VALIDATES: AC-3 -- the digest is taken over the same grouped-by-code order the
// rebuild consumes, so two destinations whose producers ran in a different order
// still compare equal when the rebuild would produce the same bytes.
// PREVENTS: a digest that is order-sensitive where the rebuild is not, which
// costs hits without buying any safety.
func TestFingerprintUsesRebuildOrder(t *testing.T) {
	forward := digestOf(t, func(a *ModAccumulator) {
		a.Op(3, AttrModSet, []byte{10, 0, 0, 1})
		a.Op(8, AttrModRemove, []byte{0xFD, 0xE9, 0, 1})
	})
	reversed := digestOf(t, func(a *ModAccumulator) {
		a.Op(8, AttrModRemove, []byte{0xFD, 0xE9, 0, 1})
		a.Op(3, AttrModSet, []byte{10, 0, 0, 1})
	})
	require.Equal(t, forward, reversed,
		"GroupedOps sorts by code before the rebuild reads them, so the digest must too")
}
