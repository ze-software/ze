// RFC: rfc/short/rfc7606.md — revised UPDATE error handling
// Overview: session_validation.go — enforceRFC7606, the receive-path walk
//
// The eager attribute span index (plan/spec-wire-edit-1-base-index.md) is published as part
// of the receive path. It must change no RFC 7606 verdict and no emitted byte, so the table
// below characterizes every verdict class the walk produces and is asserted after.

package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// spanTestAttrs is a well-formed well-known-mandatory attribute set.
func spanTestAttrs() []byte {
	return []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP = 192.0.2.1
	}
}

// spanTestPrefixSIDAttrs appends a well-formed Prefix-SID (Label-Index TLV) to them.
func spanTestPrefixSIDAttrs() []byte {
	return append(spanTestAttrs(), 0xC0, 40, 10, 1, 0, 7, 0, 0, 0, 0x00, 0x00, 0x03, 0x09)
}

// spanTestSession builds an EBGP session (local 65001, peer 65002) with the default
// Prefix-SID policy (discard), which is what makes the RFC 8669 rows below fire.
func spanTestSession() *Session {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.ReceiveHoldTime = 90 * time.Second
	return NewSession(settings)
}

// TestRFC7606VerdictsUnchangedByEagerIndex pins the action, the error-ness and the exact
// emitted payload of every receive-path verdict class.
//
// VALIDATES: AC-2, AC-3 — "Every RFC 7606 action enforceRFC7606 returns today, for every
// input, byte for byte and verdict for verdict."
// PREVENTS: an index published inside the receive walk silently turning an accept into a
// reset, or shifting the bytes the strip and discard branches emit.
func TestRFC7606VerdictsUnchangedByEagerIndex(t *testing.T) {
	dupOrigin := append(spanTestAttrs(), 0x40, 0x01, 0x01, 0x02)
	truncated := append(spanTestAttrs(), 0x40, 0x05, 0x04, 0x00) // LOCAL_PREF claims 4, holds 1

	tests := []struct {
		name       string
		body       []byte
		wantAction message.RFC7606Action
		wantErr    bool
		wantHex    string // hex of the returned payload; "" means "byte-identical to input"
	}{
		{
			name:       "well-formed",
			body:       makeUpdateBody(nil, spanTestAttrs(), []byte{24, 10, 0, 0}),
			wantAction: message.RFC7606ActionNone,
		},
		{
			name:       "body too short for section headers",
			body:       []byte{0x00, 0x00},
			wantAction: message.RFC7606ActionSessionReset,
			wantErr:    true,
		},
		{
			name:       "withdrawn length exceeds body",
			body:       []byte{0xff, 0x00, 0x00, 0x00},
			wantAction: message.RFC7606ActionSessionReset,
			wantErr:    true,
		},
		{
			name:       "attribute length exceeds body",
			body:       []byte{0x00, 0x00, 0xff, 0x00},
			wantAction: message.RFC7606ActionSessionReset,
			wantErr:    true,
		},
		{
			name:       "nlri prefix length above family maximum",
			body:       makeUpdateBody(nil, spanTestAttrs(), []byte{33, 10, 0, 0, 0}),
			wantAction: message.RFC7606ActionSessionReset,
			wantErr:    true,
		},
		{
			name:       "duplicate non-mp attribute",
			body:       makeUpdateBody(nil, dupOrigin, []byte{24, 10, 0, 0}),
			wantAction: message.RFC7606ActionNone,
			wantHex:    textbuf.StringHex(makeUpdateBody(nil, spanTestAttrs(), []byte{24, 10, 0, 0})),
		},
		{
			// RFC 7606 Section 7.4: a LOCAL_PREF whose length is not 4 is treat-as-withdraw,
			// not a reset, and the body is returned untouched for the caller to re-synthesize.
			name:       "attribute length error",
			body:       makeUpdateBody(nil, truncated, []byte{24, 10, 0, 0}),
			wantAction: message.RFC7606ActionTreatAsWithdraw,
		},
		{
			// draft-mangin-idr-attr-tombstone-00 Section 4.2: new_flags = 0x80 | (orig & 0x50),
			// so a transitive Prefix-SID (0xC0) yields a transitive marker. The length field is
			// unchanged (10) and the value carries (code 40, reason 1) then zeroes.
			name:       "prefix-sid from ebgp",
			body:       makeUpdateBody(nil, spanTestPrefixSIDAttrs(), []byte{24, 10, 0, 0}),
			wantAction: message.RFC7606ActionAttributeDiscard,
			wantHex: textbuf.StringHex(makeUpdateBody(nil,
				append(spanTestAttrs(), 0xC0, 252, 10, 40, 1, 0, 0, 0, 0, 0, 0, 0, 0),
				[]byte{24, 10, 0, 0})),
		},
		{
			name:       "empty update (ipv4 end-of-rib)",
			body:       makeUpdateBody(nil, nil, nil),
			wantAction: message.RFC7606ActionNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := spanTestSession()
			wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(append([]byte(nil), tt.body...), 0))
			assert.Equal(t, tt.wantAction, action, "action")
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			want := tt.wantHex
			if want == "" {
				want = textbuf.StringHex(tt.body)
			}
			assert.Equal(t, want, textbuf.StringHex(wu.Payload()), "emitted payload")
		})
	}
}

// TestReceivePathPublishesSpanIndex proves the base carries its attribute index once
// enforceRFC7606 returns, with no consumer accessor called first.
//
// VALIDATES: AC-1 — the index exists on the base before publication.
// PREVENTS: a return to lazy construction, where the first forward-path reader pays the
// walk and, before this spec, a write lock as well.
func TestReceivePathPublishesSpanIndex(t *testing.T) {
	s := spanTestSession()
	body := makeUpdateBody(nil, spanTestAttrs(), []byte{24, 10, 0, 0})

	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err)
	require.Equal(t, message.RFC7606ActionNone, action)

	// A first Attrs() call that had to construct the AttributesWire would allocate it.
	// Zero allocations is the observable difference between eager and lazy.
	allocs := testing.AllocsPerRun(50, func() {
		if _, err := wu.Attrs(); err != nil {
			t.Fatal(err)
		}
	})
	assert.Zero(t, allocs, "enforceRFC7606 must have published the base already")

	attrs, err := wu.Attrs()
	require.NoError(t, err)
	require.NotNil(t, attrs)
	assert.Equal(t, 3, attrs.Count(), "one span per attribute, in wire order")
}

// TestInPlaceDiscardPrecedesIndexBuild is the A-6 guard for the in-place branch.
//
// ApplyAttrDiscard's in-place branch overwrites the type-code byte with ATTR_TOMBSTONE and
// builds no new WireUpdate, so an index frozen before that write would report the original
// code as present after it has been tombstoned.
//
// VALIDATES: A-6 — the bytes the walk indexes are the bytes that get published.
// PREVENTS: an Attrs() call added earlier in enforceRFC7606 freezing a pre-tombstone index,
// which would leave every attribute-based policy acting on a discarded attribute.
func TestInPlaceDiscardPrecedesIndexBuild(t *testing.T) {
	s := spanTestSession()
	body := makeUpdateBody(nil, spanTestPrefixSIDAttrs(), []byte{24, 10, 0, 0})

	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err)
	require.Equal(t, message.RFC7606ActionAttributeDiscard, action)

	attrs, err := wu.Attrs()
	require.NoError(t, err)
	require.NotNil(t, attrs)

	hasSID, err := attrs.Has(attribute.AttrPrefixSID)
	require.NoError(t, err)
	assert.False(t, hasSID, "the tombstoned Prefix-SID must not be present in the published index")

	hasTombstone, err := attrs.Has(attribute.AttrTombstone)
	require.NoError(t, err)
	assert.True(t, hasTombstone, "the index must see the ATTR_TOMBSTONE the in-place branch wrote")
}

// TestStripRebuildIndexMatchesPublished is the A-6 guard for the other branch.
//
// The RFC 7606 Section 3.g keep-first strip rebuilds the body and wraps it in a NEW
// WireUpdate, so every span offset after the first stripped range shifts.
//
// VALIDATES: A-6 — the index describes the bytes actually published, not the bytes walked.
// PREVENTS: spans surviving a rebuild that moved the attributes they point at, and an index
// built over the pre-strip bytes failing outright on the duplicate the strip removed.
func TestStripRebuildIndexMatchesPublished(t *testing.T) {
	s := spanTestSession()
	// ORIGIN, AS_PATH, NEXT_HOP, a duplicate ORIGIN the strip removes, then MED.
	attrs := append(spanTestAttrs(), 0x40, 0x01, 0x01, 0x02)
	attrs = append(attrs, 0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x2a)
	body := makeUpdateBody(nil, attrs, []byte{24, 10, 0, 0})

	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err)
	require.Equal(t, message.RFC7606ActionNone, action)

	aw, err := wu.Attrs()
	require.NoError(t, err, "an index over the pre-strip bytes would fail on the duplicate")
	require.NotNil(t, aw)

	med, err := aw.GetRaw(attribute.AttrMED)
	require.NoError(t, err)
	assert.Equal(t, []byte{0x00, 0x00, 0x00, 0x2a}, med,
		"MED must be read from its post-strip offset, not its pre-strip one")
	assert.Equal(t, 4, aw.Count(), "the duplicate ORIGIN must be gone from the index")
}
