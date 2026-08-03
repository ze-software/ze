package reactor

import (
	"encoding/binary"
	"encoding/hex"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/test/sim"
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
		// These three arise on paths that keep building rather than returning.
		// They suppress like the rest: a route missing a modification the
		// policy required must not go out. See modifyFailure.failed.
		{modifyFailureNoHandler, "no-handler", true},
		{modifyFailureHandlerFault, "handler-fault", true},
		{modifyFailureTruncated, "truncated", true},
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

// VALIDATES: AC-15 — the modify-failure warning is bounded to one line per
// reason per interval, and the next emitted line reports how many were swallowed
// so the rate is still visible.
// PREVENTS: the log amplification an independent review found after T1-1 shipped.
// The failure fires once per DESTINATION at the peer's send rate, so a fan-out of
// N turned one bad UPDATE into N warnings. That is a logging denial of service
// against the operator, on a path a peer influences.
//
// Time is injected rather than slept on: a test that waits out a real second
// asserts on elapsed time, which is the load-sensitive shape ai/rules/completion.md
// bans.
func TestModifyFailureLogRateLimits(t *testing.T) {
	var l modifyFailureLog
	const t0 = int64(1_000_000_000)

	emit, suppressed := l.allow(modifyFailureOverflow, t0)
	require.True(t, emit, "the first failure of a reason must always be logged")
	assert.Zero(t, suppressed, "nothing was swallowed before the first line")

	for i := range 500 {
		emit, _ := l.allow(modifyFailureOverflow, t0+int64(i))
		require.False(t, emit, "a burst inside the window must be swallowed, not logged")
	}

	// A different reason has its own window, so one noisy reason cannot mute another.
	emit, _ = l.allow(modifyFailureNoHandler, t0+1)
	assert.True(t, emit, "each reason carries its own bound")

	// Past the window, the next line reports the burst it swallowed.
	emit, suppressed = l.allow(modifyFailureOverflow, t0+int64(modifyFailureLogInterval)+1)
	require.True(t, emit, "the window must reopen")
	assert.Equal(t, uint64(500), suppressed,
		"the emitted line must carry the count it replaced, or the rate is invisible")

	// The count resets, so the next line does not re-report the same burst.
	for range 3 {
		l.allow(modifyFailureOverflow, t0+int64(modifyFailureLogInterval)+2)
	}
	_, suppressed = l.allow(modifyFailureOverflow, t0+2*int64(modifyFailureLogInterval)+3)
	assert.Equal(t, uint64(3), suppressed, "each line reports only its own window")
}

// VALIDATES: AC-15 — the limiter's per-reason arrays cover every declared reason,
// and a value outside the set is folded in rather than dropped or panicking.
// PREVENTS: a reason added to the iota block below modifyFailureCount, which
// would size the arrays without being reachable through them and index out of
// range on the first failure of that kind.
func TestModifyFailureLogCoversEveryReason(t *testing.T) {
	var l modifyFailureLog
	require.Equal(t, int(modifyFailureCount), len(l.nextAllowed),
		"the limiter must have a slot for every declared reason")

	for i := range int(modifyFailureCount) {
		f := modifyFailure(i)
		emit, _ := l.allow(f, 1)
		assert.True(t, emit, "%s must get its own first line", f.String())
	}

	// A value no constant produced: folded in, never a panic and never silence.
	assert.NotPanics(t, func() { l.allow(modifyFailure(200), 1) },
		"an out-of-range reason must fold into the closed set, as String() already does")
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

// VALIDATES: AC-1 — a modification whose exact size exceeds the RFC 8654 body
// ceiling is reported as a FAILURE, distinguishable from "nothing to modify".
// PREVENTS: The fail-open this spec exists to close. Before this, an oversize
// modification returned the same nil as the empty case and every caller
// forwarded the route UNMODIFIED, leaking whatever the policy was stripping.
//
// The mechanism changed with the exactly-sized rebuild and the assertion did
// not. Overflow used to mean "the handler wrote past the slack buffer", which
// cannot happen once the buffer is sized from the plan. It now means the only
// oversize left: an edit whose exact body would exceed the 65516-octet ceiling
// RFC 8654 sets, and which therefore fits no peer under any negotiated size.
func TestBuildModifiedPayloadOverflowReportsFailure(t *testing.T) {
	// A source attribute section just under the 2-octet Total Path Attribute
	// Length ceiling, plus an NLRI tail, so the BODY crosses 65516 while the
	// attribute section stays inside 65535 and reports this failure rather than
	// the attr-length one.
	bigValue := make([]byte, 65000)
	bigAttr := make([]byte, 4+len(bigValue))
	bigAttr[0] = 0xD0 // optional, transitive, extended length
	bigAttr[1] = 99
	binary.BigEndian.PutUint16(bigAttr[2:4], uint16(len(bigValue)))
	copy(bigAttr[4:], bigValue)
	payload := buildModTestPayload(bigAttr, make([]byte, 2000))

	// A new attribute of 4 + 500 bytes: the section reaches 65508, still inside
	// the 2-octet field, while the body reaches 67512.
	addHandler := filterapi.AttrModHandler(func(p *filterapi.AttrPlan) {
		p.Op(0)
		p.Emit(0xC0, p.Code())
	})

	var mods filterapi.ModAccumulator
	mods.Op(200, filterapi.AttrModSet, make([]byte, 500))

	result, _, fail := buildModifiedPayload(payload, &mods,
		map[uint8]filterapi.AttrModHandler{200: addHandler}, nil, nil)

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

	// Source attrs occupy 65004. A 4 + 600 byte attribute crosses the 2-octet
	// Total Path Attribute Length field at 65535.
	bigHandler := filterapi.AttrModHandler(func(p *filterapi.AttrPlan) {
		p.Op(0)
		p.Emit(0xC0, p.Code())
	})

	var mods filterapi.ModAccumulator
	mods.Op(200, filterapi.AttrModSet, make([]byte, 600))

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

// VALIDATES: AC-1 — a modification that does not land because the build kept
// going, rather than because it returned early, is STILL reported as a failure.
// PREVENTS: the fail-open an independent review found after T1-1 shipped. T1-1
// closed only the paths that RETURN nil. Four paths warn and carry on — a
// missing handler, a faulting handler, and the same two for a newly added
// attribute — and each produced a well-formed payload plus modifyFailureNone,
// so every caller forwarded a route the policy had not changed. For a community
// strip or a private-ASN removal that is the exact leak the policy exists to
// prevent.
func TestBuildModifiedPayloadUnappliedModificationIsFailure(t *testing.T) {
	origin := makeAttr(0x40, 1, []byte{0x00})
	community := makeAttr(0xC0, 8, []byte{0x00, 0x64, 0x00, 0x01})
	nlri := []byte{24, 10, 0, 0}

	panicking := filterapi.AttrModHandler(func(_ *filterapi.AttrPlan) {
		panic("handler fault under test")
	})
	// A handler naming bytes that are not in the source value. The offset-range
	// check this replaces guarded the same class of fault at write time; the
	// plan refuses it at construction, before a buffer exists.
	badFragment := filterapi.AttrModHandler(func(p *filterapi.AttrPlan) {
		p.Keep(0, 1<<20) // far outside the source value
		p.Emit(0xC0, p.Code())
	})

	cases := []struct {
		name     string
		payload  []byte
		handlers map[uint8]filterapi.AttrModHandler
		mods     func(*filterapi.ModAccumulator)
		want     modifyFailure
	}{
		{
			// The policy asks to strip a community and no handler is
			// registered. The route must NOT go out carrying it.
			name:     "no handler for an existing attribute",
			payload:  buildModTestPayload(slices.Concat(origin, community), nlri),
			handlers: map[uint8]filterapi.AttrModHandler{},
			mods: func(m *filterapi.ModAccumulator) {
				m.Op(8, filterapi.AttrModRemove, []byte{0x00, 0x64, 0x00, 0x01})
			},
			want: modifyFailureNoHandler,
		},
		{
			// The policy asks to ADD an attribute and no handler exists, so it
			// is never written. Silently skipping would drop, for example, the
			// RFC 9234 OTC marker.
			name:     "no handler for a new attribute",
			payload:  buildModTestPayload(origin, nlri),
			handlers: map[uint8]filterapi.AttrModHandler{},
			mods: func(m *filterapi.ModAccumulator) {
				m.Op(35, filterapi.AttrModSet, []byte{0, 0, 0xFD, 0xE8})
			},
			want: modifyFailureNoHandler,
		},
		{
			name:     "handler panics on an existing attribute",
			payload:  buildModTestPayload(slices.Concat(origin, community), nlri),
			handlers: map[uint8]filterapi.AttrModHandler{8: panicking},
			mods: func(m *filterapi.ModAccumulator) {
				m.Op(8, filterapi.AttrModRemove, []byte{0x00, 0x64, 0x00, 0x01})
			},
			want: modifyFailureHandlerFault,
		},
		{
			name:     "handler names bytes outside the source value",
			payload:  buildModTestPayload(slices.Concat(origin, community), nlri),
			handlers: map[uint8]filterapi.AttrModHandler{8: badFragment},
			mods: func(m *filterapi.ModAccumulator) {
				m.Op(8, filterapi.AttrModRemove, []byte{0x00, 0x64, 0x00, 0x01})
			},
			want: modifyFailureHandlerFault,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var mods filterapi.ModAccumulator
			tc.mods(&mods)

			result, _, fail := buildModifiedPayload(tc.payload, &mods, tc.handlers, nil, nil)

			assert.Equal(t, tc.want, fail, "an unapplied modification must name itself")
			assert.True(t, fail.failed(), "the caller must suppress, never forward a route missing the change")
			assert.Nil(t, result, "a failed build hands out no payload")
		})
	}
}

// VALIDATES: AC-1 — a truncated attribute section is a failure, not an early
// exit. The walk stops, so the remaining attributes are never copied.
// PREVENTS: emitting a silently short attribute section that still parses.
func TestBuildModifiedPayloadTruncatedSectionIsFailure(t *testing.T) {
	origin := makeAttr(0x40, 1, []byte{0x00})
	// Declare an attribute length the section cannot satisfy: a 3-byte header
	// claiming 200 value bytes, with none present.
	truncated := []byte{0x40, 5, 200}
	payload := buildModTestPayload(slices.Concat(origin, truncated), nil)

	var mods filterapi.ModAccumulator
	mods.Op(1, filterapi.AttrModSet, []byte{0x00})

	result, _, fail := buildModifiedPayload(payload, &mods,
		attrModHandlersWithDefaults(), nil, nil)

	assert.Equal(t, modifyFailureTruncated, fail, "truncation must name itself")
	assert.True(t, fail.failed(), "an incomplete rebuild must not go out")
	assert.Nil(t, result, "a truncated rebuild hands out no payload")
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
			// CORRECTED BY CHILD 2, and this is the ONE golden that moves.
			//
			// MED (code 4) used to be emitted AFTER LOCAL_PREF (code 5), because
			// the forward-modify path appended new attributes after every source
			// attribute while both announce rails emitted ascending type-code
			// order. One route therefore reached the wire in two byte orders
			// depending on which path built it. RFC 4271 Section 5 describes
			// ascending order, so the merge-insert rebuild emits 1, 4, 5 and the
			// two paths now agree.
			//
			// Every OTHER row in this corpus must stay byte-identical: they either
			// add no attribute, or add one that already sorted last.
			name:    "set-local-pref-and-add-med",
			payload: buildModTestPayload(slices.Concat(origin, localPref), nlri),
			mods: func(m *filterapi.ModAccumulator) {
				m.Op(5, filterapi.AttrModSet, []byte{0, 0, 0, 200})
				m.Op(4, filterapi.AttrModSet, []byte{0, 0, 0, 50})
			},
			// ORIGIN, MED=50, LOCAL_PREF=200, 10.0.0.0/24
			wantHex: "000000124001010080040400000032400504000000c8180a0000",
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

// VALIDATES: AC-10, AC-13 -- the corpus emits the same bytes when the output
// buffer comes from the PER-PEER pool as when it comes from the sync.Pool
// fallback.
// PREVENTS: the gap AC-10 claimed to cover and did not. TestGoldenBytesUnchangedTier1
// passes pp == nil for every case, so it only ever exercised the sync.Pool
// fallback in acquireModBuf. The per-peer branch (idx > 0) is the one the
// forward rails actually take, and a pooled buffer handed back short or still
// holding the previous route's bytes would not have shown up anywhere.
//
// The pool backing is poisoned before each case, so a result slice that reaches
// past what the build wrote carries 0xEE and fails the hex compare rather than
// looking plausible.
func TestGoldenBytesUnchangedTier1PooledBuffer(t *testing.T) {
	for _, tc := range goldenModifyCorpus() {
		t.Run(tc.name, func(t *testing.T) {
			pp := newPeerPool(message.MaxMsgLen)
			// Stale bytes from a previous route, in every buffer of the pool.
			for i := range pp.backing {
				pp.backing[i] = 0xEE
			}

			var mods filterapi.ModAccumulator
			tc.mods(&mods)

			result, bufIdx, fail := buildModifiedPayload(tc.payload, &mods,
				attrModHandlersWithDefaults(), pp, nil)

			require.Equal(t, modifyFailureNone, fail, "corpus case must not fail on the pooled path")
			require.NotNil(t, result, "corpus case must produce a payload")
			require.Positive(t, bufIdx,
				"guard: this test is about the PER-PEER pool; bufIdx 0 means it silently took the fallback and proved nothing")

			got := hex.EncodeToString(result)
			assert.Equal(t, tc.wantHex, got,
				"a pooled output buffer must emit the same bytes as the fallback, with no stale 0xEE")

			pp.Return(bufIdx)
		})
	}
}

// VALIDATES: AC-10, AC-13 -- a payload larger than one already built from the
// same pooled buffer does not inherit the shorter one's tail.
// PREVENTS: the specific dirty-buffer shape the poison above cannot reach.
// Poisoning proves the build does not read bytes it never wrote; this proves it
// does not read bytes IT wrote on a previous route, which is the realistic
// failure because a recycled buffer holds a well-formed UPDATE rather than 0xEE.
func TestPooledBufferIsNotReusedDirty(t *testing.T) {
	corpus := goldenModifyCorpus()
	require.GreaterOrEqual(t, len(corpus), 2, "guard: this test needs two differently sized cases")

	pp := newPeerPool(message.MaxMsgLen)

	// Longest case first, so the buffer is left holding its bytes.
	var longest goldenModifyCase
	for _, tc := range corpus {
		if len(tc.wantHex) > len(longest.wantHex) {
			longest = tc
		}
	}
	var shortest goldenModifyCase
	shortest.wantHex = longest.wantHex
	for _, tc := range corpus {
		if len(tc.wantHex) < len(shortest.wantHex) {
			shortest = tc
		}
	}
	require.NotEqual(t, longest.name, shortest.name, "guard: two distinct sizes are needed")

	for _, tc := range []goldenModifyCase{longest, shortest, longest} {
		var mods filterapi.ModAccumulator
		tc.mods(&mods)

		result, bufIdx, fail := buildModifiedPayload(tc.payload, &mods,
			attrModHandlersWithDefaults(), pp, nil)

		require.Equal(t, modifyFailureNone, fail, "%s must not fail", tc.name)
		require.Positive(t, bufIdx, "%s must take the per-peer pool", tc.name)
		assert.Equal(t, tc.wantHex, hex.EncodeToString(result),
			"%s must emit its own bytes, never a tail left by the previous route", tc.name)

		pp.Return(bufIdx)
	}
}

// VALIDATES: countModifyFailure reads the reactor's INJECTED clock, so the
// one-line-per-second suppression window advances with simulated time in a
// chaos or simulation run rather than with wall time.
// PREVENTS: the window silently reverting to time.Now(). TestNoDirectTimeCalls
// (internal/core/clock) is a textual grep, so it would keep passing if the call
// moved behind a helper that still read wall time; this asserts the behavior.
func TestCountModifyFailureUsesInjectedClock(t *testing.T) {
	fc := sim.NewFakeClock(time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC))
	r := &Reactor{clock: fc}

	emit, suppressed := r.countModifyFailure(modifyFailureOverflow)
	require.True(t, emit, "the first failure of a reason must be logged")
	require.Zero(t, suppressed, "nothing was suppressed before the first emission")

	emit, _ = r.countModifyFailure(modifyFailureOverflow)
	require.False(t, emit, "a repeat inside the window must be suppressed")

	// Wall time would still be inside the window here, so this is the assertion
	// that separates an injected clock from time.Now().
	fc.Add(modifyFailureLogInterval + time.Nanosecond)
	emit, suppressed = r.countModifyFailure(modifyFailureOverflow)
	require.True(t, emit, "the window must reopen once the INJECTED clock passes it")
	assert.Equal(t, uint64(1), suppressed, "the suppressed repeat must be reported by the next emission")
}
