// Design: docs/architecture/bgp/egress-attribute-rules.md -- source retirement after marked subsets.
// Related: filter_atomic_subset.go -- native identity difference and owned withdrawals.
package reactor

import (
	"bytes"
	"testing"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
)

// TestAtomicAggregateSubsetFamilies removes one native route while retaining
// its sibling for every registered family, with and without ADD-PATH zero.
func TestAtomicAggregateSubsetFamilies(t *testing.T) {
	family.RegisterTestFamilies()
	first, second := atomicPolicyNativeFamilies(1), atomicPolicyNativeFamilies(2)
	for _, name := range family.RegisteredFamilyNames() {
		fam, ok := family.LookupFamily(name)
		if !ok {
			t.Fatalf("registered family %q cannot be looked up", name)
		}
		a, ok := first[fam]
		if !ok {
			t.Fatalf("registered family %s needs a source retirement fixture", name)
		}
		b := second[fam]
		t.Run(name, func(t *testing.T) {
			for _, addPath := range []bool{false, true} {
				ctx := atomicPolicyContext(t, fam, addPath)
				one, two := a, b
				if addPath {
					one, two = atomicPolicyPath(0, a), atomicPolicyPath(7, b)
				}
				input := atomicPolicyMP(fam, bytes.Join([][]byte{one, two}, nil), true)
				output := atomicPolicyMP(fam, two, true)
				bodies, err := atomicAggregatePolicyRemovals(input, output, ctx)
				if err != nil {
					t.Fatal(err)
				}
				if len(bodies) != 1 {
					t.Fatalf("retirement bodies=%d, want one", len(bodies))
				}
				want := atomicSubsetKeys(t, [][]byte{atomicPolicyMP(fam, one, true)}, ctx, false)
				atomicSubsetWantKeys(t, bodies, ctx, want)
			}
		})
	}
}

// TestAtomicAggregateSubsetAlreadyCovered protects output announcements and
// explicit withdrawals alike, including a source withdrawal+announcement pair.
func TestAtomicAggregateSubsetAlreadyCovered(t *testing.T) {
	fam := family.IPv4Unicast
	ctx := atomicPolicyContext(t, fam, false)
	a, b := []byte{24, 10, 42, 0}, []byte{24, 10, 43, 0}
	input := atomicPolicyLegacy(bytes.Join([][]byte{a, b}, nil), true)
	for _, tc := range []struct {
		name   string
		input  []byte
		output []byte
	}{
		{"unchanged", input, input},
		{"unmarked", atomicPolicyLegacy(bytes.Join([][]byte{a, b}, nil), false), atomicPolicyLegacy(b, false)},
		{"explicit-output-withdrawal", input, makeUpdateBody(a, []byte{0x40, 6, 0}, b)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bodies, err := atomicAggregatePolicyRemovals(tc.input, tc.output, ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(bodies) != 0 {
				t.Fatalf("covered routes produced %d redundant retirement bodies", len(bodies))
			}
		})
	}
	input = makeUpdateBody(a, []byte{0x40, 6, 0}, bytes.Join([][]byte{a, b}, nil))
	bodies, err := atomicAggregatePolicyRemovals(input, atomicPolicyLegacy(a, true), ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := atomicSubsetKeys(t, [][]byte{atomicPolicyLegacy(b, true)}, ctx, false)
	atomicSubsetWantKeys(t, bodies, ctx, want)
}

// TestAtomicAggregateSubsetMixedWithdrawals preserves an omitted original
// withdrawal beside removed legacy and MP routes, using two distinct MP bodies.
func TestAtomicAggregateSubsetMixedWithdrawals(t *testing.T) {
	v6 := family.IPv6Unicast
	vpn := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}
	ctx, err := bgpctx.Registry.Register(bgpctx.EncodingContextWithAddPath(true,
		map[family.Family]bool{v6: true, vpn: true, family.IPv4Unicast: true}))
	if err != nil {
		t.Fatal(err)
	}
	a := atomicPolicyPath(0, atomicPolicyNativeFamilies(1)[v6])
	b := atomicPolicyPath(7, atomicPolicyNativeFamilies(2)[v6])
	c := atomicPolicyPath(0, atomicPolicyNativeFamilies(3)[vpn])
	cWithdraw := atomicSubsetWithdrawn(t, vpn, c, true)
	legacy := atomicPolicyPath(0, []byte{24, 10, 44, 0})
	originalWithdrawal := atomicPolicyPath(9, []byte{24, 10, 45, 0})
	mp := atomicPolicyMP(v6, bytes.Join([][]byte{a, b}, nil), true)
	attrs := append(bytes.Clone(mp[4:]), mpUnreachAttrs(vpn, cWithdraw)...)
	input := makeUpdateBody(originalWithdrawal, attrs, legacy)
	bodies, err := atomicAggregatePolicyRemovals(input, atomicPolicyMP(v6, b, true), ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 2 {
		t.Fatalf("retirement bodies=%d, want two MP families", len(bodies))
	}
	want := atomicSubsetKeys(t, [][]byte{atomicPolicyMP(v6, a, true), atomicPolicyMP(vpn, c, true),
		atomicPolicyLegacy(bytes.Join([][]byte{legacy, originalWithdrawal}, nil), true)}, ctx, false)
	atomicSubsetWantKeys(t, bodies, ctx, want)
}

// TestAtomicAggregateSubsetNativeWithdrawal protects a path already withdrawn
// in the effective output, whose label Compatibility bytes are not a label stack.
func TestAtomicAggregateSubsetNativeWithdrawal(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}
	ctx := atomicPolicyContext(t, fam, true)
	a := atomicPolicyPath(0, atomicPolicyNativeFamilies(1)[fam])
	b := atomicPolicyPath(7, atomicPolicyNativeFamilies(2)[fam])
	withdrawn := atomicSubsetWithdrawn(t, fam, a, true)
	withdrawn[5], withdrawn[6], withdrawn[7] = 0x01, 0x02, 0x03
	keep := atomicPolicyMP(fam, b, true)
	output := makeUpdateBody(nil, append(bytes.Clone(keep[4:]), mpUnreachAttrs(fam, withdrawn)...), nil)
	input := atomicPolicyMP(fam, bytes.Join([][]byte{a, b}, nil), true)
	bodies, err := atomicAggregatePolicyRemovals(input, output, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 0 {
		t.Fatalf("already withdrawn labeled identity produced %d redundant bodies", len(bodies))
	}
}

// TestAtomicAggregateSubsetOwnedAndDeduplicated checks source aliases and
// subsequent pooled calls cannot change a retained retirement body.
func TestAtomicAggregateSubsetOwnedAndDeduplicated(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}
	ctx := atomicPolicyContext(t, fam, false)
	a := atomicPolicyNativeFamilies(1)[fam]
	labelOnly := bytes.Clone(a)
	labelOnly[2] ^= 0x10
	input := atomicPolicyMP(fam, bytes.Join([][]byte{a, labelOnly}, nil), true)
	bodies, err := atomicAggregatePolicyRemovals(input, makeUpdateBody(nil, nil, nil), ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := atomicSubsetKeys(t, [][]byte{atomicPolicyMP(fam, a, true)}, ctx, false)
	atomicSubsetWantKeys(t, bodies, ctx, want)
	retained := bytes.Clone(bodies[0])
	clear(input)
	other := atomicPolicyMP(fam, atomicPolicyNativeFamilies(2)[fam], true)
	if _, err := atomicAggregatePolicyRemovals(other, makeUpdateBody(nil, nil, nil), ctx); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(retained, bodies[0]) {
		t.Fatal("retained retirement body aliases source or reused pooled storage")
	}
}

func atomicSubsetWithdrawn(t *testing.T, fam family.Family, raw []byte, addPath bool) []byte {
	t.Helper()
	writer := nlrisplit.GetWithdrawalWriter(fam)
	if writer == nil {
		return bytes.Clone(raw)
	}
	out := make([]byte, len(raw))
	n, err := writer(out, raw, addPath, false)
	if err != nil {
		t.Fatal(err)
	}
	return out[:n]
}

func atomicSubsetKeys(t *testing.T, bodies [][]byte, ctxID bgpctx.ContextID, onlyWithdraw bool) map[fwdPathKey]int {
	t.Helper()
	keys := make(map[fwdPathKey]int)
	ctx := bgpctx.Registry.Get(ctxID)
	for _, body := range bodies {
		err := fwdBodySections(body, func(raw []byte, fam family.Family, withdraw, _ bool) error {
			if onlyWithdraw {
				if !withdraw {
					t.Fatal("retirement body contains an announcement")
				}
			}
			return fwdProvenancePaths(raw, fam, ctx, withdraw, func(native []byte, id uint32, _ bool) error {
				var key fwdPathKey
				var scratch [nlrisplit.PrefixKeyScratchSize]byte
				if err := fwdPathKeyFor(&key, fam, id, native, withdraw, scratch[:]); err != nil {
					return err
				}
				keys[key]++
				return nil
			})
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return keys
}

func atomicSubsetWantKeys(t *testing.T, bodies [][]byte, ctx bgpctx.ContextID, want map[fwdPathKey]int) {
	t.Helper()
	got := atomicSubsetKeys(t, bodies, ctx, true)
	if len(got) != len(want) {
		t.Fatalf("retired identities=%d, want %d", len(got), len(want))
	}
	for key, count := range want {
		if got[key] != count {
			t.Fatalf("retired identity family=%v path=%d count=%d, want %d", key.family, key.received, got[key], count)
		}
	}
}
