// Design: docs/architecture/bgp/egress-attribute-rules.md -- restricted marked-route policy rewrites.
// Related: filter_atomic_aggregate.go -- native route-key admission.
package reactor

import (
	"bytes"
	"encoding/binary"
	"sync"
	"testing"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// TestAtomicAggregatePolicyFamilies checks useful subset and reordering results,
// then inserts a new identity for every linked family. Native framing, rather
// than an assumed CIDR length, must decide all of these cases.
func TestAtomicAggregatePolicyFamilies(t *testing.T) {
	family.RegisterTestFamilies()
	first := atomicPolicyNativeFamilies(1)
	second := atomicPolicyNativeFamilies(2)
	inserted := atomicPolicyNativeFamilies(3)
	for _, name := range family.RegisteredFamilyNames() {
		fam, ok := family.LookupFamily(name)
		if !ok {
			t.Fatalf("registered family %q cannot be looked up", name)
		}
		a, ok := first[fam]
		if !ok {
			t.Fatalf("registered family %s needs a native admission fixture", name)
		}
		b, c := second[fam], inserted[fam]
		t.Run(name, func(t *testing.T) {
			for _, addPath := range []bool{false, true} {
				ctx := atomicPolicyContext(t, fam, addPath)
				one, two, extra := a, b, c
				if addPath {
					one = atomicPolicyPath(0, a)
					two = atomicPolicyPath(7, b)
					extra = atomicPolicyPath(9, c)
				}
				input := atomicPolicyMP(fam, bytes.Join([][]byte{one, two}, nil), true)
				atomicPolicyWant(t, input, atomicPolicyMP(fam, two, true), ctx, true)
				atomicPolicyWant(t, input, atomicPolicyMP(fam, bytes.Join([][]byte{two, one}, nil), true), ctx, true)
				atomicPolicyWant(t, input, atomicPolicyMP(fam, extra, true), ctx, false)
			}
		})
	}
}

// TestAtomicAggregatePolicyBoundary separates the received marker condition
// from the effective output and preserves useful withdrawals and attribute edits.
func TestAtomicAggregatePolicyBoundary(t *testing.T) {
	fam := family.IPv4Unicast
	ctx := atomicPolicyContext(t, fam, false)
	original := []byte{24, 10, 42, 0}
	input := atomicPolicyLegacy(original, true)
	for _, tc := range []struct {
		name   string
		input  []byte
		output []byte
		allow  bool
	}{
		{"equal", input, atomicPolicyLegacy(original, true), true},
		{"attribute-only", input, makeUpdateBody(nil, []byte{0x40, 6, 0, 0x40, 1, 1, 2}, original), true},
		{"more-specific", input, atomicPolicyLegacy([]byte{25, 10, 42, 0, 0}, true), false},
		{"less-specific-disabled", input, atomicPolicyLegacy([]byte{16, 10, 42}, true), false},
		{"marker-removed", input, atomicPolicyLegacy(original, false), false},
		{"marker-malformed", input, makeUpdateBody(nil, []byte{0x40, 6, 1, 0}, original), false},
		{"marker-optional", input, makeUpdateBody(nil, []byte{0xc0, 6, 0}, original), false},
		{"marker-duplicate", input, makeUpdateBody(nil, []byte{0x40, 6, 0, 0x40, 6, 0}, original), false},
		{"withdrawal", input, makeUpdateBody(original, nil, nil), true},
		{"empty", input, makeUpdateBody(nil, nil, nil), true},
		{"unmarked-control", atomicPolicyLegacy(original, false), atomicPolicyLegacy([]byte{25, 10, 42, 0, 0}, false), true},
		{"unmarked-adds-marker", atomicPolicyLegacy(original, false), atomicPolicyLegacy([]byte{25, 10, 42, 0, 0}, true), true},
		{"unmarked-mp-adds-marker", atomicPolicyMP(family.IPv6Unicast, []byte{32, 0x20, 1, 0xdb, 8}, false),
			atomicPolicyMP(family.IPv6Unicast, []byte{33, 0x20, 1, 0xdb, 8, 0}, true), true},
		{"body-to-mp", input, atomicPolicyMP(fam, original, true), true},
		{"family-changed", input, atomicPolicyMP(family.IPv6Unicast, original, true), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			allowed, err := atomicAggregatePolicyAllowed(tc.input, tc.output, ctx)
			if tc.allow && err != nil {
				t.Fatal(err)
			}
			if allowed != tc.allow {
				t.Fatalf("allowed = %v, want %v; error = %v", allowed, tc.allow, err)
			}
		})
	}
}

// TestAtomicAggregatePolicyIdentity checks padding normalization, non-key label
// changes, an opaque type change, and ADD-PATH zero as a real path identity.
func TestAtomicAggregatePolicyIdentity(t *testing.T) {
	fam := family.IPv4Unicast
	ctx := atomicPolicyContext(t, fam, false)
	atomicPolicyWant(t, atomicPolicyLegacy([]byte{17, 10, 42, 0xff}, true),
		atomicPolicyLegacy([]byte{17, 10, 42, 0x80}, true), ctx, true)
	labeled := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}
	labelInput := atomicPolicyNativeFamilies(1)[labeled]
	labelOutput := bytes.Clone(labelInput)
	labelOutput[2] ^= 0x10
	atomicPolicyWant(t, atomicPolicyMP(labeled, labelInput, true), atomicPolicyMP(labeled, labelOutput, true), ctx, true)
	pathCtx := atomicPolicyContext(t, fam, true)
	prefix := []byte{24, 10, 42, 0}
	pathInput := atomicPolicyLegacy(atomicPolicyPath(0, prefix), true)
	atomicPolicyWant(t, pathInput, atomicPolicyLegacy(atomicPolicyPath(0, prefix), true), pathCtx, true)
	atomicPolicyWant(t, pathInput, atomicPolicyLegacy(atomicPolicyPath(1, prefix), true), pathCtx, false)
	bgpls := family.Family{AFI: family.AFIBGPLS, SAFI: family.SAFIBGPLinkState}
	opaque := atomicPolicyNativeFamilies(1)[bgpls]
	changed := bytes.Clone(opaque)
	changed[1]++
	atomicPolicyWant(t, atomicPolicyMP(bgpls, opaque, true), atomicPolicyMP(bgpls, changed, true), ctx, false)
}

// TestAtomicAggregatePolicyMixed keeps legacy and MP paths independent and
// checks a newly inserted sibling cannot hide behind a retained native route.
func TestAtomicAggregatePolicyMixed(t *testing.T) {
	v6 := family.IPv6Unicast
	ctx := atomicPolicyContext(t, v6, true)
	v4 := []byte{24, 10, 42, 0}
	v6Raw := atomicPolicyPath(0, []byte{32, 0x20, 1, 0xdb, 8})
	mp := atomicPolicyMP(v6, v6Raw, true)
	input := makeUpdateBody([]byte{24, 192, 0, 2}, mp[4:], v4)
	atomicPolicyWant(t, input, makeUpdateBody(nil, mp[4:], nil), ctx, true)
	atomicPolicyWant(t, input, makeUpdateBody(nil, mp[4:], []byte{25, 10, 42, 0, 0}), ctx, false)
}

// TestAtomicAggregatePolicyMalformed refuses framing and missing context rather
// than reading a path identifier or a label octet as a prefix length.
func TestAtomicAggregatePolicyMalformed(t *testing.T) {
	fam := family.IPv4Unicast
	ctx := atomicPolicyContext(t, fam, false)
	input := atomicPolicyLegacy([]byte{24, 10, 42, 0}, true)
	for _, output := range [][]byte{{}, {0, 0, 0}, atomicPolicyLegacy([]byte{32, 10}, true)} {
		allowed, err := atomicAggregatePolicyAllowed(input, output, ctx)
		if allowed || err == nil {
			t.Fatalf("malformed output %x: allowed=%v error=%v", output, allowed, err)
		}
	}
	allowed, err := atomicAggregatePolicyAllowed(input, input, bgpctx.ContextID(0))
	if allowed || err == nil {
		t.Fatalf("unknown context: allowed=%v error=%v", allowed, err)
	}
}

// TestAtomicAggregatePolicyPoolReuse drives concurrent differently sized opaque
// keys and confirms no prior call's native membership survives pool release.
func TestAtomicAggregatePolicyPoolReuse(t *testing.T) {
	fam := family.Family{AFI: family.AFIBGPLS, SAFI: family.SAFIBGPLinkState}
	ctx := atomicPolicyContext(t, fam, false)
	var group sync.WaitGroup
	for worker := range 8 {
		group.Go(func() {
			for round := range 24 {
				a := atomicPolicyNativeFamilies(byte(worker + round + 1))[fam]
				b := bytes.Clone(a)
				b[len(b)-1] ^= 0x80
				input := atomicPolicyMP(fam, a, true)
				allowed, err := atomicAggregatePolicyAllowed(input, atomicPolicyMP(fam, b, true), ctx)
				if allowed || err != nil {
					t.Errorf("foreign pooled key: allowed=%v error=%v", allowed, err)
				}
			}
		})
	}
	group.Wait()
}

// TestAtomicAggregatePolicyHashCollision forces one local bucket to hold
// distinct native keys. Hash equality alone must never admit an inserted route.
func TestAtomicAggregatePolicyHashCollision(t *testing.T) {
	scratch := acquireAtomicPolicyScratch()
	defer scratch.release()
	scratch.count = 2
	if err := scratch.reserve(); err != nil {
		t.Fatal(err)
	}
	bucket := atomicPolicyBucket{family: family.IPv4Unicast, hash: 7}
	a := []byte{24, 10, 42, 0}
	b := []byte{24, 10, 43, 0}
	c := []byte{24, 10, 44, 0}
	scratch.insert(bucket, a)
	for _, key := range [][]byte{b, c} {
		present, err := scratch.contains(bucket, key)
		if err != nil {
			t.Fatal(err)
		}
		if present {
			t.Fatalf("hash collision admitted unrelated key %x", key)
		}
	}
	scratch.insert(bucket, b)
	for _, key := range [][]byte{a, b} {
		present, err := scratch.contains(bucket, key)
		if err != nil {
			t.Fatal(err)
		}
		if !present {
			t.Fatalf("collision chain lost source key %x", key)
		}
	}
}

// TestAtomicAggregatePolicyArenaBounds checks unproven frame counts produce an
// error before size-class indexing or storage allocation can peer-panic.
func TestAtomicAggregatePolicyArenaBounds(t *testing.T) {
	scratch := acquireAtomicPolicyScratch()
	defer scratch.release()
	for _, count := range []int{-1, maxUpdateBody + 1} {
		scratch.count = count
		if err := scratch.reserve(); err == nil {
			t.Fatalf("unproven native count %d reserved membership storage", count)
		}
	}
}

// TestAtomicAggregatePolicyLongNativeKeys exercises an IPv6 EVPN MAC/IP route
// whose normalized key is 37 octets and a larger opaque BGP-LS key. Neither key
// may be truncated at the forwarding path key's 33-octet inline boundary.
func TestAtomicAggregatePolicyLongNativeKeys(t *testing.T) {
	ctx := atomicPolicyContext(t, family.IPv4Unicast, false)
	fam := family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}
	evpn := make([]byte, 51)
	evpn[0], evpn[1], evpn[24], evpn[31], evpn[47], evpn[50] = 2, 49, 48, 128, 1, 1
	input := atomicPolicyMP(fam, evpn, true)
	attributes := bytes.Clone(evpn)
	attributes[10] = 7 // ESI is not part of MAC/IP route identity.
	atomicPolicyWant(t, input, atomicPolicyMP(fam, attributes, true), ctx, true)
	changed := bytes.Clone(evpn)
	changed[47] = 2 // Last IPv6 octet lies beyond 33 bytes of canonical key.
	atomicPolicyWant(t, input, atomicPolicyMP(fam, changed, true), ctx, false)
	bgpls := family.Family{AFI: family.AFIBGPLS, SAFI: family.SAFIBGPLinkState}
	opaque := atomicPolicyNativeFamilies(1)[bgpls]
	changed = bytes.Clone(opaque)
	changed[len(changed)-1] = 2
	atomicPolicyWant(t, atomicPolicyMP(bgpls, opaque, true), atomicPolicyMP(bgpls, changed, true), ctx, false)
}

func atomicPolicyWant(t *testing.T, input, output []byte, ctx bgpctx.ContextID, want bool) {
	t.Helper()
	allowed, err := atomicAggregatePolicyAllowed(input, output, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if allowed != want {
		t.Fatalf("allowed=%v, want %v; input=%x output=%x", allowed, want, input, output)
	}
}

func atomicPolicyContext(t *testing.T, fam family.Family, addPath bool) bgpctx.ContextID {
	t.Helper()
	ctx, err := bgpctx.Registry.Register(bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{fam: addPath}))
	if err != nil {
		t.Fatal(err)
	}
	return ctx
}

func atomicPolicyPath(id uint32, raw []byte) []byte {
	return append(binary.BigEndian.AppendUint32(nil, id), raw...)
}

func atomicPolicyLegacy(raw []byte, marked bool) []byte {
	var attrs []byte
	if marked {
		attrs = []byte{0x40, 6, 0}
	}
	return makeUpdateBody(nil, attrs, raw)
}

func atomicPolicyMP(fam family.Family, raw []byte, marked bool) []byte {
	value := binary.BigEndian.AppendUint16(nil, uint16(fam.AFI))
	value = append(value, byte(fam.SAFI), 0, 0)
	value = append(value, raw...)
	attrs := []byte{0x90, 14, byte(len(value) >> 8), byte(len(value))}
	attrs = append(attrs, value...)
	if marked {
		attrs = append(attrs, 0x40, 6, 0)
	}
	return makeUpdateBody(nil, attrs, nil)
}

// atomicPolicyNativeFamilies varies a destination identity inside each owner's
// framing. The registration census in TestAtomicAggregatePolicyFamilies refuses
// a new family until a valid native fixture joins this table.
func atomicPolicyNativeFamilies(seed byte) map[family.Family][]byte {
	out := make(map[family.Family][]byte)
	for _, afi := range []family.AFI{family.AFIIPv4, family.AFIIPv6} {
		for _, safi := range []family.SAFI{family.SAFIUnicast, family.SAFIMulticast} {
			out[family.Family{AFI: afi, SAFI: safi}] = []byte{24, 10, 42, seed}
		}
		out[family.Family{AFI: afi, SAFI: family.SAFIMPLSLabel}] = []byte{48, 0, 0x10, 1, 10, 42, seed}
		vpn := []byte{112, 0, 0x10, 1, 0, 0, 0xfd, 0xe8, 0, 0, 0, 1, 10, 42, seed}
		out[family.Family{AFI: afi, SAFI: family.SAFIVPN}] = vpn
		out[family.Family{AFI: afi, SAFI: family.SAFIMUP}] = []byte{1, 0, 1, 12, 0, 0, 0xfd, 0xe8, 0, 0, 0, 1, 24, 10, 42, seed}
		addressLen := 4
		if afi == family.AFIIPv6 {
			addressLen = 16
		}
		mvpn := make([]byte, 2+8+addressLen)
		mvpn[0], mvpn[1], mvpn[len(mvpn)-1] = 1, byte(len(mvpn)-2), seed
		out[family.Family{AFI: afi, SAFI: family.SAFIMVPN}] = mvpn
		sr := make([]byte, 1+8+addressLen)
		sr[0], sr[len(sr)-1] = byte((len(sr)-1)*8), seed
		out[family.Family{AFI: afi, SAFI: family.SAFISRPolicy}] = sr
		out[family.Family{AFI: afi, SAFI: family.SAFIFlowSpec}] = []byte{5, 1, 24, 10, 42, seed}
		out[family.Family{AFI: afi, SAFI: family.SAFIFlowSpecVPN}] = []byte{13, 0, 0, 0xfd, 0xe8, 0, 0, 0, 1, 1, 24, 10, 42, seed}
	}
	rtc := make([]byte, 13)
	rtc[0], rtc[len(rtc)-1] = 96, seed
	out[family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIRTC}] = rtc
	evpn := make([]byte, 60)
	evpn[0], evpn[1], evpn[24], evpn[27] = 5, 58, 24, seed
	out[family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}] = evpn
	vpls := make([]byte, 19)
	vpls[1], vpls[11], vpls[15], vpls[18] = 17, seed, 1, 1
	out[family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIVPLS}] = vpls
	for _, safi := range []family.SAFI{family.SAFIBGPLinkState, family.SAFIBGPLinkStateVPN} {
		ls := make([]byte, 84)
		ls[1], ls[3], ls[len(ls)-1] = 1, 80, seed
		out[family.Family{AFI: family.AFIBGPLS, SAFI: safi}] = ls
	}
	return out
}
