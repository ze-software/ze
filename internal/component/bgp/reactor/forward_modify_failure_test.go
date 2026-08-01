package reactor

import (
	"encoding/binary"
	"encoding/hex"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
)

// VALIDATES: AC-2 — the reason label set of ze_bgp_update_modify_failed_total is
// closed and stable, so a dashboard or alert keyed on a label keeps working and
// a peer cannot drive label cardinality.
// PREVENTS: A renamed label silently breaking every alert built on it, and an
// unbounded label set reachable from peer-controlled input.
func TestModifyFailureLabelsAreClosedAndStable(t *testing.T) {
	cases := []struct {
		failure modifyFailure
		label   string
		failed  bool
	}{
		{modifyFailureNone, "no-failure", false},
		{modifyFailureMalformed, "malformed", true},
		{modifyFailureOverflow, "overflow", true},
		{modifyFailureAttrLenRange, "attr-length-range", true},
		{modifyFailureWithdrawnSize, "withdrawn-size", true},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			assert.Equal(t, tc.label, tc.failure.String(), "label must not drift")
			assert.Equal(t, tc.failed, tc.failure.failed(), "failed() must agree with the label")
		})
	}

	// The set is closed: a value no constant above produced still maps into it
	// rather than escaping as a fresh label.
	assert.Equal(t, "unclassified", modifyFailure(200).String(),
		"an out-of-range value must fold into the closed set")

	// Every label is distinct, or two failure kinds would be indistinguishable
	// on a dashboard.
	seen := make(map[string]bool, len(cases))
	for _, tc := range cases {
		require.False(t, seen[tc.label], "duplicate label %q", tc.label)
		seen[tc.label] = true
	}
}

// VALIDATES: AC-3 — "nothing to modify" is NOT a failure. The route is forwarded
// as-is and no counter increments.
// PREVENTS: The legitimate empty case being counted as a modify failure, which
// would make the counter fire on every unmodified route and hide the real ones.
func TestBuildModifiedPayloadNoModsIsNotFailure(t *testing.T) {
	attrs := makeAttr(0x40, 1, []byte{0x00}) // ORIGIN=IGP
	payload := buildModTestPayload(attrs, nil)

	var mods filterapi.ModAccumulator
	result, _, fail := buildModifiedPayload(payload, &mods, nil, nil, nil)

	assert.Nil(t, result, "no mods returns no payload")
	assert.Equal(t, modifyFailureNone, fail, "no mods is not a failure")
	assert.False(t, fail.failed(), "caller must forward, not suppress")
}

// VALIDATES: AC-1 — an overflow during the attribute walk is reported as a
// FAILURE, distinguishable from "nothing to modify".
// PREVENTS: The fail-open this spec exists to close. Before this, an oversize
// modification returned the same nil as the empty case and every caller
// forwarded the route UNMODIFIED, leaking whatever the policy was stripping.
func TestBuildModifiedPayloadOverflowReportsFailure(t *testing.T) {
	origin := makeAttr(0x40, 1, []byte{0x00})
	localPref := makeAttr(0x40, 5, []byte{0, 0, 0, 100})
	attrs := slices.Concat(origin, localPref)
	payload := buildModTestPayload(attrs, []byte{24, 10, 0, 0})

	// Fill the buffer so the verbatim LOCAL_PREF copy cannot fit.
	bigHandler := filterapi.AttrModHandler(func(_ []byte, _ []filterapi.AttrOp, buf []byte, off int) int {
		n := len(buf) - off - 6 // LOCAL_PREF needs 7
		if n <= 0 || off+n > len(buf) {
			return off
		}
		for i := range n {
			buf[off+i] = 0xAA
		}
		return off + n
	})

	var mods filterapi.ModAccumulator
	mods.Op(1, filterapi.AttrModSet, []byte{0x00})

	result, _, fail := buildModifiedPayload(payload, &mods,
		map[uint8]filterapi.AttrModHandler{1: bigHandler}, nil, nil)

	require.Nil(t, result, "overflow produces no payload")
	assert.Equal(t, modifyFailureOverflow, fail, "overflow must name itself")
	assert.True(t, fail.failed(), "caller must suppress, never forward unmodified")
}

// VALIDATES: AC-1 — a rebuilt attribute section that does not fit the 2-octet
// Total Path Attribute Length field (RFC 4271 Section 4.3) reports a failure.
// PREVENTS: An attr_len wrap silently forwarding the unmodified route.
func TestBuildModifiedPayloadAttrLenRangeReportsFailure(t *testing.T) {
	// The output buffer is sized len(payload)+256, so a large NLRI tail buys
	// the headroom needed to WRITE past 65535 in the attribute section. Without
	// that tail the buffer runs out first and the failure reported is overflow,
	// which is a different path with a different label.
	bigValue := make([]byte, 65000)
	bigAttr := make([]byte, 4+len(bigValue))
	bigAttr[0] = 0xD0 // optional, transitive, extended length
	bigAttr[1] = 99
	binary.BigEndian.PutUint16(bigAttr[2:4], uint16(len(bigValue)))
	copy(bigAttr[4:], bigValue)
	payload := buildModTestPayload(bigAttr, make([]byte, 2000))

	// Source attrs occupy 65004. 600 more crosses the 2-octet field at 65535
	// while staying inside the buffer.
	bigHandler := filterapi.AttrModHandler(func(_ []byte, _ []filterapi.AttrOp, buf []byte, off int) int {
		n := 600
		if off+n > len(buf) {
			return off
		}
		for i := range n {
			buf[off+i] = 0xFF
		}
		return off + n
	})

	var mods filterapi.ModAccumulator
	mods.Op(200, filterapi.AttrModSet, []byte{0x01})

	result, _, fail := buildModifiedPayload(payload, &mods,
		map[uint8]filterapi.AttrModHandler{200: bigHandler}, nil, nil)

	require.Nil(t, result, "attr_len overflow produces no payload")
	assert.Equal(t, modifyFailureAttrLenRange, fail, "attr_len overflow must name itself")
	assert.True(t, fail.failed())
}

// VALIDATES: AC-1 — a withdrawn rewrite beyond the 2-octet Withdrawn Routes
// Length field reports a failure.
// PREVENTS: An oversize withdrawal rewrite forwarding the original withdrawal,
// which desyncs the peer's adj-rib-out from ours.
func TestBuildModifiedPayloadWithdrawnSizeReportsFailure(t *testing.T) {
	withdrawn := []byte{24, 10, 0, 0}
	payload := make([]byte, 2+len(withdrawn)+2)
	binary.BigEndian.PutUint16(payload[0:2], uint16(len(withdrawn)))
	copy(payload[2:], withdrawn)

	var mods filterapi.ModAccumulator
	mods.SetWithdrawnRewrite(make([]byte, 65536)) // one past the field

	result, _, fail := buildModifiedPayload(payload, &mods, nil, nil, nil)

	require.Nil(t, result, "oversize withdrawn rewrite produces no payload")
	assert.Equal(t, modifyFailureWithdrawnSize, fail, "withdrawn size must name itself")
	assert.True(t, fail.failed())
}

// VALIDATES: AC-1 — a payload that does not parse as an UPDATE body reports a
// failure rather than the "nothing to modify" nil.
// PREVENTS: A malformed body silently forwarding unmodified. Reaching this means
// bytes that passed RFC 7606 validation do not re-parse here, which is a bug we
// must see rather than absorb.
func TestBuildModifiedPayloadMalformedReportsFailure(t *testing.T) {
	cases := map[string][]byte{
		"shorter_than_header":  {0x00},
		"withdrawn_len_beyond": {0x00, 0xFF, 0x00, 0x00},
		"attr_len_beyond":      {0x00, 0x00, 0xFF, 0x00},
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			var mods filterapi.ModAccumulator
			mods.Op(1, filterapi.AttrModSet, []byte{0x00})

			result, _, fail := buildModifiedPayload(payload, &mods, nil, nil, nil)

			require.Nil(t, result)
			assert.Equal(t, modifyFailureMalformed, fail, "malformed must name itself")
			assert.True(t, fail.failed(), "caller must suppress")
		})
	}
}

// goldenModifyCase is one row of the Tier 1 byte-identity corpus.
type goldenModifyCase struct {
	name    string
	payload []byte
	mods    func(*filterapi.ModAccumulator)
	// wantHex is the exact output of buildModifiedPayload BEFORE any Tier 1
	// item landed. Every Tier 1 item must leave it untouched.
	wantHex string
}

// goldenModifyCorpus is the transform matrix. It is deliberately built from the
// same helpers the rest of this package's tests use, so a fixture change shows
// up as a diff here rather than as a silent divergence.
func goldenModifyCorpus() []goldenModifyCase {
	origin := makeAttr(0x40, 1, []byte{0x00})
	localPref := makeAttr(0x40, 5, []byte{0, 0, 0, 100})
	community := makeAttr(0xC0, 8, []byte{0x00, 0x64, 0x00, 0x01})
	nlri := []byte{24, 10, 0, 0}

	return []goldenModifyCase{
		{
			name:    "set-local-pref",
			payload: buildModTestPayload(slices.Concat(origin, localPref), nlri),
			mods: func(m *filterapi.ModAccumulator) {
				m.Op(5, filterapi.AttrModSet, []byte{0, 0, 0, 200})
			},
			// ORIGIN, LOCAL_PREF=200, 10.0.0.0/24
			wantHex: "0000000b40010100400504000000c8180a0000",
		},
		{
			// A NEW attribute: exercises the unconsumed-op pass, which is a
			// different branch from replacing an attribute already present.
			name:    "add-med",
			payload: buildModTestPayload(origin, nlri),
			mods: func(m *filterapi.ModAccumulator) {
				m.Op(4, filterapi.AttrModSet, []byte{0, 0, 0, 50})
			},
			// ORIGIN, MED=50, 10.0.0.0/24
			wantHex: "0000000b4001010080040400000032180a0000",
		},
		{
			name:    "remove-community",
			payload: buildModTestPayload(slices.Concat(origin, community), nlri),
			mods: func(m *filterapi.ModAccumulator) {
				m.Op(8, filterapi.AttrModRemove, []byte{0x00, 0x64, 0x00, 0x01})
			},
			// ORIGIN only: the whole community list went away.
			wantHex: "0000000440010100180a0000",
		},
		{
			name:    "nlri-rewrite",
			payload: buildModTestPayload(origin, nlri),
			mods: func(m *filterapi.ModAccumulator) {
				m.SetNLRIRewrite([]byte{24, 172, 16, 0})
			},
			// ORIGIN, 172.16.0.0/24
			wantHex: "000000044001010018ac1000",
		},
		{
			// RFC 4456 Section 8 reflection: ORIGINATOR_ID set-if-absent plus
			// CLUSTER_LIST prepend, the pair a route reflector emits.
			name:    "rr-originator-and-cluster",
			payload: buildModTestPayload(slices.Concat(origin, localPref), nlri),
			mods: func(m *filterapi.ModAccumulator) {
				m.Op(9, filterapi.AttrModSet, []byte{10, 0, 0, 1})
				m.Op(10, filterapi.AttrModPrepend, []byte{0, 0, 0, 7})
			},
			// ORIGIN, LOCAL_PREF=100, ORIGINATOR_ID=10.0.0.1, CLUSTER_LIST=7.
			// Codes 1, 5, 9, 10: ascending here only because the added codes
			// happen to sort after the source ones.
			wantHex: "0000001940010100400504000000648009040a000001800a0400000007180a0000",
		},
		{
			// Pins T2-7 rather than endorsing it. MED (code 4) is emitted AFTER
			// LOCAL_PREF (code 5) because the forward-modify path appends new
			// attributes after every source attribute, while both announce rails
			// are pinned to ascending type-code order. One route can therefore
			// reach the wire in two byte orders depending on which path built it.
			// Tier 1 must NOT change this; umbrella child 2 merge-inserts and
			// will legitimately update this golden.
			name:    "set-local-pref-and-add-med",
			payload: buildModTestPayload(slices.Concat(origin, localPref), nlri),
			mods: func(m *filterapi.ModAccumulator) {
				m.Op(5, filterapi.AttrModSet, []byte{0, 0, 0, 200})
				m.Op(4, filterapi.AttrModSet, []byte{0, 0, 0, 50})
			},
			wantHex: "0000001240010100400504000000c880040400000032180a0000",
		},
	}
}

// VALIDATES: AC-10 — every Tier 1 item leaves emitted bytes identical, with the
// single deliberate exception of the AC-1 suppression.
// PREVENTS: A change that looks like a pure cost or reporting fix quietly moving
// a byte on the wire. A wire regression is invisible to the rest of this suite
// and only shows against a real peer.
func TestGoldenBytesUnchangedTier1(t *testing.T) {
	for _, tc := range goldenModifyCorpus() {
		t.Run(tc.name, func(t *testing.T) {
			var mods filterapi.ModAccumulator
			tc.mods(&mods)

			result, _, fail := buildModifiedPayload(tc.payload, &mods,
				attrModHandlersWithDefaults(), nil, nil)

			require.Equal(t, modifyFailureNone, fail, "corpus case must not fail")
			require.NotNil(t, result, "corpus case must produce a payload")

			got := hex.EncodeToString(result)
			if tc.wantHex == "" {
				t.Fatalf("golden not pinned for %q; observed output is:\n\t%s", tc.name, got)
			}
			assert.Equal(t, tc.wantHex, got,
				"Tier 1 must not change emitted bytes. If this is deliberate, it is NOT a Tier 1 item.")
		})
	}
}
