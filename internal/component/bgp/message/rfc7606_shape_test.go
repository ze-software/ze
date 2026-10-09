package message

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// RFC 7606 Section 5.1, second bullet: "An UPDATE message MUST NOT contain more than one of
// the following: non-empty Withdrawn Routes field, non-empty Network Layer Reachability
// Information field, MP_REACH_NLRI attribute, and MP_UNREACH_NLRI attribute."
//
// NLRIBearingFieldCount is the single producer of "how many does this UPDATE carry", used by
// both splitters. If it undercounts, a mixed UPDATE is relayed unchanged and the MUST is
// violated; if it overcounts, a compliant UPDATE is split for nothing and the zero-copy
// forward path is lost.

const (
	testOrigin   = 0x40 // flags
	testAttrLen1 = 0x01
)

// shapeAttrs builds a path-attribute block: always ORIGIN, plus the requested MP attributes.
func shapeAttrs(mpReach, mpUnreach bool) []byte {
	attrs := []byte{testOrigin, 0x01, testAttrLen1, 0x00} // ORIGIN = IGP
	if mpUnreach {
		// MP_UNREACH_NLRI (15): AFI 2 / SAFI 1 + one /64.
		value := []byte{0x00, 0x02, 0x01, 0x40, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00}
		attrs = append(attrs, 0x80, 0x0f, byte(len(value)))
		attrs = append(attrs, value...)
	}
	if mpReach {
		// MP_REACH_NLRI (14): AFI 2 / SAFI 1, 16-byte next hop, reserved, one /64.
		value := []byte{0x00, 0x02, 0x01, 0x10}
		value = append(value,
			0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x00, 0x01, // next hop
			0x00,                                                 // reserved
			0x40, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01, 0x00, 0x00) // 2001:db8:0:1::/64
		attrs = append(attrs, 0x80, 0x0e, byte(len(value)))
		attrs = append(attrs, value...)
	}
	return attrs
}

var (
	shapeWithdrawn = []byte{0x18, 0x0a, 0x00, 0x00} // 10.0.0.0/24
	shapeNLRI      = []byte{0x18, 0xc0, 0x00, 0x02} // 192.0.2.0/24
)

// VALIDATES: every combination of the four NLRI-bearing fields is counted exactly.
// PREVENTS: an undercount relaying a mixed UPDATE unchanged, and an overcount splitting a
// compliant one (which would cost the zero-copy forward path for nothing).
//
// RFC requirement: RFC7606-5.1-2 positive -- every combination of the four NLRI-bearing
// fields is counted, which is what tells a sender it must split before transmitting.
func TestNLRIBearingFieldCountEveryCombination(t *testing.T) {
	for _, tc := range []struct {
		name               string
		withdrawn, nlri    bool
		mpReach, mpUnreach bool
		want               int
	}{
		{name: "none", want: 0},
		{name: "withdrawn only", withdrawn: true, want: 1},
		{name: "nlri only", nlri: true, want: 1},
		{name: "mp-reach only", mpReach: true, want: 1},
		{name: "mp-unreach only", mpUnreach: true, want: 1},
		{name: "withdrawn+nlri", withdrawn: true, nlri: true, want: 2},
		{name: "withdrawn+mp-reach", withdrawn: true, mpReach: true, want: 2},
		{name: "withdrawn+mp-unreach", withdrawn: true, mpUnreach: true, want: 2},
		{name: "nlri+mp-reach", nlri: true, mpReach: true, want: 2},
		{name: "nlri+mp-unreach", nlri: true, mpUnreach: true, want: 2},
		{name: "both mp", mpReach: true, mpUnreach: true, want: 2},
		{name: "withdrawn+nlri+mp-reach", withdrawn: true, nlri: true, mpReach: true, want: 3},
		{name: "withdrawn+nlri+mp-unreach", withdrawn: true, nlri: true, mpUnreach: true, want: 3},
		{name: "withdrawn+both mp", withdrawn: true, mpReach: true, mpUnreach: true, want: 3},
		{name: "nlri+both mp", nlri: true, mpReach: true, mpUnreach: true, want: 3},
		{name: "all four", withdrawn: true, nlri: true, mpReach: true, mpUnreach: true, want: 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var w, n []byte
			if tc.withdrawn {
				w = shapeWithdrawn
			}
			if tc.nlri {
				n = shapeNLRI
			}
			attrs := shapeAttrs(tc.mpReach, tc.mpUnreach)

			assert.Equal(t, tc.want, NLRIBearingFieldCount(w, attrs, n))

			u := &Update{WithdrawnRoutes: w, PathAttributes: attrs, NLRI: n}
			assert.Equal(t, tc.want > 1, u.MixesNLRIFields())
		})
	}
}

// VALIDATES: an UPDATE with no path attributes at all is still counted from its two legacy
// sections.
// PREVENTS: a nil-attrs early return skipping the withdrawn/NLRI check.
func TestNLRIBearingFieldCountNoAttributes(t *testing.T) {
	assert.Equal(t, 2, NLRIBearingFieldCount(shapeWithdrawn, nil, shapeNLRI))
	assert.Equal(t, 0, NLRIBearingFieldCount(nil, nil, nil))
}

// VALIDATES: attribute bytes that stop parsing mid-way do not invent a violation.
// PREVENTS: a truncated attribute block turning a relayable UPDATE into a split (or, worse,
// an error) on a path whose input was already validated by enforceRFC7606.
func TestNLRIBearingFieldCountTruncatedAttributes(t *testing.T) {
	full := shapeAttrs(true, true)
	// Cut inside the MP_REACH value: the iterator stops, MP_UNREACH before it still counts.
	truncated := full[:len(full)-4]
	assert.Equal(t, 1, NLRIBearingFieldCount(nil, truncated, nil),
		"only the attributes that parsed may be counted")

	// A header that cannot even be read yields nothing rather than a panic.
	assert.Equal(t, 0, NLRIBearingFieldCount(nil, []byte{0x80}, nil))
}

// VALIDATES: SplitCompliant splits a mixed UPDATE that already fits.
// PREVENTS: the fits fast path relaying a mixed shape unchanged, which is the whole gap this
// spec closes.
//
// RFC requirement: RFC7606-5.1-2 positive -- an UPDATE carrying more than one NLRI-bearing
// field is split into one field per message even when it already fits.
func TestSplitCompliantSplitsMixedUpdateThatFits(t *testing.T) {
	u := &Update{
		WithdrawnRoutes: shapeWithdrawn,
		PathAttributes:  shapeAttrs(true, true),
		NLRI:            shapeNLRI,
	}
	require.True(t, u.MixesNLRIFields(), "guard: the fixture must start non-compliant")
	require.LessOrEqual(t,
		HeaderLen+4+len(u.WithdrawnRoutes)+len(u.PathAttributes)+len(u.NLRI), 4096,
		"guard: the fixture must FIT, so only the shape can force the split")

	var got []int
	s := NewSplitter()
	require.NoError(t, s.SplitCompliant(u, 4096, func(family.Family) bool { return false }, func(c *Update) error {
		got = append(got, NLRIBearingFieldCount(c.WithdrawnRoutes, c.PathAttributes, c.NLRI))
		return nil
	}))

	require.Len(t, got, 4, "each of the four fields needs its own message")
	for i, n := range got {
		assert.LessOrEqualf(t, n, 1, "emitted UPDATE %d carries %d NLRI-bearing fields", i, n)
	}
}

// VALIDATES: a compliant UPDATE that fits is emitted verbatim, as one message, with its
// PathAttributes slice untouched.
// PREVENTS: SplitCompliant charging the re-encode cost on the common single-field case. The
// identity assertion is the one that matters: an equal-but-copied slice would still pass a
// value comparison while having lost the zero-copy property.
//
// RFC requirement: RFC7606-5.1-2 negative -- an UPDATE with at most one NLRI-bearing field
// is already compliant and is emitted unchanged, not split.
func TestSplitCompliantPassesThroughCompliantUpdate(t *testing.T) {
	attrs := shapeAttrs(false, false)
	u := &Update{PathAttributes: attrs, NLRI: shapeNLRI}
	require.False(t, u.MixesNLRIFields())

	var emitted []*Update
	s := NewSplitter()
	require.NoError(t, s.SplitCompliant(u, 4096, func(family.Family) bool { return false }, func(c *Update) error {
		emitted = append(emitted, c)
		return nil
	}))

	require.Len(t, emitted, 1)
	assert.Same(t, u, emitted[0], "a compliant UPDATE that fits must be passed through as-is")
}

// VALIDATES: End-of-RIB is never split by SplitCompliant.
// PREVENTS: an EoR marker being rewritten, which would break graceful restart (RFC 4724).
func TestSplitCompliantEndOfRIBUntouched(t *testing.T) {
	u := &Update{}
	require.True(t, u.IsEndOfRIB())

	var emitted []*Update
	s := NewSplitter()
	require.NoError(t, s.SplitCompliant(u, 4096, func(family.Family) bool { return false }, func(c *Update) error {
		emitted = append(emitted, c)
		return nil
	}))
	require.Len(t, emitted, 1)
	assert.Same(t, u, emitted[0])
}

// VALIDATES: SplitCompliant still splits on SIZE, exactly as Split does.
// PREVENTS: the new entry point regressing RFC 8654 size handling while fixing the shape.
func TestSplitCompliantStillSplitsOnSize(t *testing.T) {
	var nlri []byte
	for i := range 60 {
		nlri = append(nlri, 0x18, 0xc0, 0x00, byte(i))
	}
	u := &Update{PathAttributes: shapeAttrs(false, false), NLRI: nlri}
	require.False(t, u.MixesNLRIFields())

	count := 0
	s := NewSplitter()
	require.NoError(t, s.SplitCompliant(u, 100, func(family.Family) bool { return false }, func(_ *Update) error {
		count++
		return nil
	}))
	assert.Greater(t, count, 1, "an oversized UPDATE must still split")
}

// VALIDATES: withdrawals precede announcements when a mixed fitting UPDATE is split.
// PREVENTS: a peer briefly holding a route ze meant to withdraw. Splitting makes the
// ordering observable where it previously did not exist.
func TestSplitCompliantWithdrawalsPrecedeAnnouncements(t *testing.T) {
	u := &Update{
		WithdrawnRoutes: shapeWithdrawn,
		PathAttributes:  shapeAttrs(true, true),
		NLRI:            shapeNLRI,
	}

	var order []string
	s := NewSplitter()
	require.NoError(t, s.SplitCompliant(u, 4096, func(family.Family) bool { return false }, func(c *Update) error {
		switch {
		case len(c.WithdrawnRoutes) > 0:
			order = append(order, "withdrawn")
		case len(c.NLRI) > 0:
			order = append(order, "nlri")
		case findMPAttribute(c.PathAttributes, 15).found:
			order = append(order, "mp-unreach")
		case findMPAttribute(c.PathAttributes, 14).found:
			order = append(order, "mp-reach")
		}
		return nil
	}))

	assert.Equal(t, []string{"withdrawn", "mp-unreach", "mp-reach", "nlri"}, order,
		"withdrawals before announcements; MP_UNREACH before MP_REACH")
}

// RFC 4724 Section 2: "For any other address family, it is an UPDATE message
// that contains only the MP_UNREACH_NLRI attribute [BGP-MP] with no withdrawn
// routes for that <AFI, SAFI>."
// Separating an empty MP withdrawal from a sibling must not invent that marker.
func TestSplitCompliantEmptyMPUnreachKeepsMessageMeaning(t *testing.T) {
	base := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN
		0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xfd, 0xe9, // AS_PATH [65001]
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP 192.0.2.1
	}
	emptyMP := []byte{0x80, 0x0f, 0x03, 0x00, 0x02, 0x01}
	mpReach := shapeAttrs(true, false)[4:]
	emptyReach := append([]byte(nil), mpReach[:24]...)
	emptyReach[2] = 21 // AFI, SAFI, next-hop length, 16-byte next hop, reserved.
	for _, tc := range []struct {
		name string
		body Update
		want Update
	}{
		{
			name: "legacy withdrawal",
			body: Update{WithdrawnRoutes: shapeWithdrawn, PathAttributes: emptyMP},
			want: Update{WithdrawnRoutes: shapeWithdrawn},
		},
		{
			name: "legacy announcement",
			body: Update{PathAttributes: append(append([]byte(nil), base...), emptyMP...), NLRI: shapeNLRI},
			want: Update{PathAttributes: base, NLRI: shapeNLRI},
		},
		{
			name: "MP announcement",
			body: Update{PathAttributes: append(append(append([]byte(nil), base...), mpReach...), emptyMP...)},
			want: Update{PathAttributes: append(append([]byte(nil), base...), mpReach...)},
		},
		{
			name: "empty MP announcement field",
			body: Update{PathAttributes: append(append(append([]byte(nil), base...), emptyReach...), emptyMP...)},
			want: Update{PathAttributes: append(append([]byte(nil), base...), emptyReach...)},
		},
		{
			name: "standalone MP End-of-RIB",
			body: Update{PathAttributes: emptyMP},
			want: Update{PathAttributes: emptyMP},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			emitted := 0
			// RFC 7606 Section 5.1; RFC 4724 Section 2.
			err := NewSplitter().SplitCompliant(&tc.body, MaxMsgLen, func(family.Family) bool { return false }, func(got *Update) error {
				emitted++
				assert.Equal(t, tc.want.WithdrawnRoutes, got.WithdrawnRoutes)
				assert.Equal(t, tc.want.PathAttributes, got.PathAttributes)
				assert.Equal(t, tc.want.NLRI, got.NLRI)
				return nil
			})
			require.NoError(t, err)
			assert.Equal(t, 1, emitted, "exactly the original sibling, or the genuine standalone EOR")
		})
	}
}

func TestSplitCompliantEmptyMPUnreachStillValidates(t *testing.T) {
	for _, tc := range []struct {
		name    string
		attrs   []byte
		maxSize int
		wantErr string
	}{
		{
			name:    "malformed MP envelope",
			attrs:   []byte{0x80, 0x0f, 0x02, 0x00, 0x02},
			maxSize: MaxMsgLen,
			wantErr: "parsing MP_UNREACH_NLRI",
		},
		{
			name:    "MP attribute budget exhausted",
			attrs:   []byte{0x80, 0x0f, 0x03, 0x00, 0x02, 0x01},
			maxSize: HeaderLen + 4 + len(shapeWithdrawn),
			wantErr: ErrAttributesTooLarge.Error(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := &Update{WithdrawnRoutes: shapeWithdrawn, PathAttributes: tc.attrs}
			emitted := 0
			// RFC 7606 Section 5.1: a split must not hide a malformed MP field.
			err := NewSplitter().SplitCompliant(u, tc.maxSize, func(family.Family) bool { return false }, func(got *Update) error {
				emitted++
				assert.Equal(t, shapeWithdrawn, got.WithdrawnRoutes)
				assert.Empty(t, got.PathAttributes)
				assert.Empty(t, got.NLRI)
				return nil
			})
			require.ErrorContains(t, err, tc.wantErr)
			assert.Equal(t, 1, emitted, "the earlier complete withdrawal remains emitted before refusal")
		})
	}
}

func TestSplitCompliantNonemptyMPWithdrawalRemains(t *testing.T) {
	mp := []byte{0x80, 0x0f, 0x0c, 0x00, 0x02, 0x01, 0x40, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0}
	u := &Update{WithdrawnRoutes: shapeWithdrawn, PathAttributes: mp}
	want := []Update{
		{WithdrawnRoutes: shapeWithdrawn},
		{PathAttributes: mp},
	}
	emitted := 0
	// RFC 7606 Section 5.1: separate both real withdrawal fields without losing either.
	err := NewSplitter().SplitCompliant(u, MaxMsgLen, func(family.Family) bool { return false }, func(got *Update) error {
		require.Less(t, emitted, len(want))
		assert.Equal(t, want[emitted].WithdrawnRoutes, got.WithdrawnRoutes)
		assert.Equal(t, want[emitted].PathAttributes, got.PathAttributes)
		assert.Equal(t, want[emitted].NLRI, got.NLRI)
		emitted++
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, len(want), emitted)
}
