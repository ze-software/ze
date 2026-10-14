// VALIDATES: SR-Policy NLRI splitter correctly splits concatenated NLRIs by length-bit prefix.
// PREVENTS: Wrong NLRI boundaries, silent truncation, unregistered splitter.

package srpolicy

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisyntax"
	"github.com/ze-software/ze/internal/core/family"
)

// srPolicyV4 and srPolicyV6 are the two families SplitSRPolicy is registered
// for. The tests below drive it through nlrisplit.Split, the entry point every
// caller uses, which materializes the NLRIs the walk visits.
var (
	srPolicyV4 = family.Family{AFI: family.AFIIPv4, SAFI: family.SAFISRPolicy}
	srPolicyV6 = family.Family{AFI: family.AFIIPv6, SAFI: family.SAFISRPolicy}
)

func TestSRPolicySplitIPv4Single(t *testing.T) {
	// 1-byte length (96 bits = 0x60) + 12-byte body
	data := make([]byte, 13)
	data[0] = 96 // 12*8 bits
	result, err := nlrisplit.Split(srPolicyV4, data, false)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Len(t, result[0], 13)
}

func TestSRPolicySplitIPv4Multiple(t *testing.T) {
	// Two IPv4 SR-Policy NLRIs concatenated
	data := make([]byte, 26) // 13 + 13
	data[0] = 96
	data[13] = 96
	result, err := nlrisplit.Split(srPolicyV4, data, false)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Len(t, result[0], 13)
	assert.Len(t, result[1], 13)
}

func TestSRPolicySplitIPv6Single(t *testing.T) {
	// 1-byte length (192 bits = 0xC0) + 24-byte body
	data := make([]byte, 25)
	data[0] = 192 // 24*8 bits
	result, err := nlrisplit.Split(srPolicyV6, data, false)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Len(t, result[0], 25)
}

func TestSRPolicySplitEmpty(t *testing.T) {
	result, err := nlrisplit.Split(srPolicyV4, nil, false)
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestSRPolicySplitTruncated(t *testing.T) {
	// Length says 96 bits (12 bytes) but only 5 bytes of body
	data := make([]byte, 6)
	data[0] = 96
	_, err := nlrisplit.Split(srPolicyV4, data, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "extends past data")
}

func TestSRPolicySplitZeroLength(t *testing.T) {
	data := []byte{0}
	_, err := nlrisplit.Split(srPolicyV4, data, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "zero-length")
}

func TestSRPolicySplitNotByteAligned(t *testing.T) {
	data := []byte{97} // 97 bits, not byte-aligned
	_, err := nlrisplit.Split(srPolicyV4, data, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not byte-aligned")
}

func TestSRPolicySplitRegistered(t *testing.T) {
	assert.True(t, nlrisplit.Supported(srPolicyV4), "ipv4/sr-policy splitter must be registered")
	assert.True(t, nlrisplit.Supported(srPolicyV6), "ipv6/sr-policy splitter must be registered")
}

// TestSRPolicyWalkAllocatesNothing pins the reason SplitSRPolicy is a walk: a
// count pass with no visitor builds nothing, so a caller that wants only the
// number of NLRIs pays no allocation for it.
func TestSRPolicyWalkAllocatesNothing(t *testing.T) {
	data := make([]byte, 26)
	data[0] = 96
	data[13] = 96

	var walkErr error
	count := 0
	allocs := testing.AllocsPerRun(100, func() {
		count, walkErr = SplitSRPolicy(data, false, nil)
	})
	require.NoError(t, walkErr)
	assert.Equal(t, 2, count)
	assert.Zero(t, allocs, "the count pass must allocate nothing")
}

// TestSRPolicyWalkStopsBeforeMalformedTail checks both native endpoint widths, with and
// without ADD-PATH. Under ADD-PATH every entry carries its 4-octet path identifier, and the
// visited entry starts with it.
func TestSRPolicyWalkStopsBeforeMalformedTail(t *testing.T) {
	for _, bodyBytes := range []int{12, 24} {
		for _, addPath := range []bool{false, true} {
			first := srPolicyEntry(bodyBytes*8, bodyBytes, addPath)
			data := append(append([]byte(nil), first...), first[:len(first)-1]...)
			for _, keepWalking := range []bool{false, true} {
				visited := 0
				count, err := SplitSRPolicy(data, addPath, func(entry []byte) bool {
					visited++
					if !bytes.Equal(entry, first) {
						t.Errorf("body=%d ADD-PATH=%v: entry = %x, want %x", bodyBytes, addPath, entry, first)
					}
					if &entry[0] != &data[0] {
						t.Error("visitor entry does not alias the input")
					}
					return keepWalking
				})
				if count != 1 {
					t.Errorf("body=%d ADD-PATH=%v continue=%v: count = %d, want 1", bodyBytes, addPath, keepWalking, count)
				}
				if visited != 1 {
					t.Errorf("body=%d ADD-PATH=%v continue=%v: visits = %d, want 1", bodyBytes, addPath, keepWalking, visited)
				}
				if (err != nil) != keepWalking {
					t.Errorf("body=%d ADD-PATH=%v continue=%v: error = %v", bodyBytes, addPath, keepWalking, err)
				}
			}
			count, err := SplitSRPolicy(data, addPath, nil)
			if count != 1 {
				t.Errorf("body=%d ADD-PATH=%v nil visitor: count = %d, want 1", bodyBytes, addPath, count)
			}
			if err == nil {
				t.Errorf("body=%d ADD-PATH=%v nil visitor accepted malformed tail", bodyBytes, addPath)
			}
		}
	}
}

// srPolicyEntry builds one NLRI section entry: an optional path identifier, a length octet
// of lenBits, and bodyBytes octets of body.
func srPolicyEntry(lenBits, bodyBytes int, addPath bool) []byte {
	var out []byte
	if addPath {
		out = append(out, 0x00, 0x00, 0x00, 0x07)
	}
	out = append(out, byte(lenBits))
	for i := range bodyBytes {
		out = append(out, byte(0xA0+i))
	}
	return out
}

// VALIDATES: the registered RFC 9830 Section 5 rule per AFI. A walkable section whose NLRIs
// all carry the AFI's one length is well-formed; one NLRI of another length makes it
// treat-as-withdraw; a section that cannot be walked is session reset. ADD-PATH gives the
// same verdicts behind a path identifier, and a cut-short path identifier is unwalkable. The
// retain answer keeps exactly the well-formed NLRIs, and SplitSRPolicy frames one entry per
// path identifier plus NLRI.
// PREVENTS: a 13-octet NLRI entering the RIB or a relay, a 24-octet NLRI accepted under AFI
// 1, and every NLRI from an ADD-PATH peer misjudged because the path identifier is read as
// the length octet (spec AC-3, AC-6, AC-8).
func TestSRPolicySyntaxRuleVerdicts(t *testing.T) {
	type verdictCase struct {
		name     string
		nlriBits int
		section  func(addPath bool) []byte
		want     nlrisyntax.Action
		kept     int // NLRIs the retain answer keeps, when walkable
	}
	cat := func(parts ...[]byte) []byte { return bytes.Join(parts, nil) }
	e := srPolicyEntry
	cases := []verdictCase{
		{"AFI 1 12 octets", nlriBitsIPv4, func(ap bool) []byte { return e(96, 12, ap) }, nlrisyntax.ActionWellFormed, 1},
		{"AFI 2 24 octets", nlriBitsIPv6, func(ap bool) []byte { return e(192, 24, ap) }, nlrisyntax.ActionWellFormed, 1},
		{"AFI 1 two back to back", nlriBitsIPv4, func(ap bool) []byte { return cat(e(96, 12, ap), e(96, 12, ap)) }, nlrisyntax.ActionWellFormed, 2},
		{"AFI 1 13 octets", nlriBitsIPv4, func(ap bool) []byte { return e(104, 13, ap) }, nlrisyntax.ActionTreatAsWithdraw, 0},
		{"AFI 1 24 octets", nlriBitsIPv4, func(ap bool) []byte { return e(192, 24, ap) }, nlrisyntax.ActionTreatAsWithdraw, 0},
		{"AFI 2 12 octets", nlriBitsIPv6, func(ap bool) []byte { return e(96, 12, ap) }, nlrisyntax.ActionTreatAsWithdraw, 0},
		{"AFI 2 25 octets", nlriBitsIPv6, func(ap bool) []byte { return e(200, 25, ap) }, nlrisyntax.ActionTreatAsWithdraw, 0},
		{"AFI 1 12 beside 13", nlriBitsIPv4, func(ap bool) []byte { return cat(e(96, 12, ap), e(104, 13, ap)) }, nlrisyntax.ActionTreatAsWithdraw, 1},
		{"zero length", nlriBitsIPv4, func(ap bool) []byte { return e(0, 0, ap) }, nlrisyntax.ActionSessionReset, 0},
		{"100 bits", nlriBitsIPv4, func(ap bool) []byte { return e(100, 12, ap) }, nlrisyntax.ActionSessionReset, 0},
		{"overrun", nlriBitsIPv4, func(ap bool) []byte { return e(96, 11, ap) }, nlrisyntax.ActionSessionReset, 0},
		{"13 then unwalkable tail", nlriBitsIPv4, func(ap bool) []byte { return cat(e(104, 13, ap), e(96, 3, ap)) }, nlrisyntax.ActionSessionReset, 0},
	}
	for _, addPath := range []bool{false, true} {
		for _, c := range cases {
			section := c.section(addPath)
			rule := srPolicySyntaxRule(c.nlriBits)
			judged := rule.Judge(section, addPath)
			if judged.Action != c.want {
				t.Errorf("%s ADD-PATH=%v: Judge = %v (%q), want %v", c.name, addPath, judged.Action, judged.Reason, c.want)
			}
			if c.want != nlrisyntax.ActionWellFormed && judged.Reason == "" {
				t.Errorf("%s ADD-PATH=%v: verdict carries no reason", c.name, addPath)
			}
			kept, dropped, framed := rule.Retain(section, addPath)
			if framed != (c.want != nlrisyntax.ActionSessionReset) {
				t.Errorf("%s ADD-PATH=%v: Retain framed = %v", c.name, addPath, framed)
				continue
			}
			if !framed {
				continue
			}
			got, err := SplitSRPolicy(kept, addPath, nil)
			if err != nil || got != c.kept {
				t.Errorf("%s ADD-PATH=%v: Retain kept %d NLRIs (%v), want %d", c.name, addPath, got, err, c.kept)
			}
			total, _ := SplitSRPolicy(section, addPath, nil)
			if dropped != total-c.kept {
				t.Errorf("%s ADD-PATH=%v: Retain dropped %d, want %d", c.name, addPath, dropped, total-c.kept)
			}
			if dropped == 0 && len(section) > 0 && &kept[0] != &section[0] {
				t.Errorf("%s ADD-PATH=%v: a section with nothing removed was copied", c.name, addPath)
			}
		}
	}

	// A path identifier cut short is unwalkable under ADD-PATH; the same three octets
	// without ADD-PATH are a framed length octet the walk then judges.
	short := []byte{0x00, 0x00, 0x00}
	if got := srPolicySyntaxRule(nlriBitsIPv4).Judge(short, true).Action; got != nlrisyntax.ActionSessionReset {
		t.Errorf("cut-short path identifier: Judge = %v, want session reset", got)
	}
	if _, err := SplitSRPolicy(short, true, nil); err == nil {
		t.Error("cut-short path identifier: SplitSRPolicy accepted it")
	}

	// SplitSRPolicy frames one entry per path identifier plus NLRI under ADD-PATH.
	two := cat(e(96, 12, true), e(96, 12, true))
	var entries [][]byte
	count, err := SplitSRPolicy(two, true, func(entry []byte) bool {
		entries = append(entries, entry)
		return true
	})
	require.NoError(t, err)
	require.Equal(t, 2, count)
	for _, entry := range entries {
		assert.Equal(t, e(96, 12, true), entry)
	}

	// Both families are registered with their AFI's length.
	for fam, bits := range map[family.Family]int{srPolicyV4: nlriBitsIPv4, srPolicyV6: nlriBitsIPv6} {
		rule, ok := nlrisyntax.Get(fam)
		require.True(t, ok, "%s: no syntax rule registered", fam)
		assert.Equal(t, nlrisyntax.ActionWellFormed, rule.Judge(e(bits, bits/8, false), false).Action, "%s", fam)
	}
}
