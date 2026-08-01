// Tests for the RFC 8669 Section 4 EBGP boundary rule when the Prefix-SID attribute is
// REPEATED. See rfc/short/rfc8669.md.
//
// Section 4 names the ATTRIBUTE, not its first occurrence. A peer outside the SR domain
// that sends the Prefix-SID attribute twice must have both copies discarded: one survivor
// is the attribute surviving, and it carries exactly the label indices Section 4 exists to
// keep out of the local SRGB. These tests drive enforceRFC7606, the real receive path, so
// they hold whichever of the two producing functions is at fault.

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
)

// rfc8669PrefixSIDAttrsN returns the well-known mandatory attributes followed by n
// well-formed Prefix-SID attributes, each carrying a distinct Label-Index so that a
// surviving copy identifies which one it is.
func rfc8669PrefixSIDAttrsN(n int) []byte {
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP = 192.0.2.1
	}
	for i := range n {
		value := []byte{1, 0, 7, 0, 0, 0, 0x00, 0x00, 0x03, byte(0x09 + i)} // Label-Index 777+i
		attrs = append(attrs, 0xC0, uint8(attribute.AttrPrefixSID), byte(len(value)))
		attrs = append(attrs, value...)
	}
	return attrs
}

// rfc8669IBGPSession builds an IBGP session (local AS == peer AS). The Section 4 boundary
// rule is an EBGP rule and must not fire here, whatever the peer sends.
func rfc8669IBGPSession() *Session {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65001, 0x01020301)
	settings.ReceiveHoldTime = 90 * time.Second
	return NewSession(settings)
}

// TestRFC8669PrefixSIDEveryOccurrenceDiscardedFromEBGP is the end-to-end proof of the
// Section 4 MUST for a repeated attribute.
//
// Either of the two causes leaks a copy onto the wire on its own, so this test goes red
// until both are closed and stays red if either regresses. It does not attribute the
// failure; TestApplyAttrDiscardRemovesEveryOccurrence (message package) and
// TestRFC8669PrefixSIDDiscardStillStripsUnrelatedDuplicate (below) each isolate one cause.
//
// VALIDATES: RFC 8669 Section 4 — "MUST discard the attribute" when it arrives from an
// EBGP neighbor the speaker is not configured to accept it from. No occurrence survives,
// at one, two or three copies, and the discard is the attribute-discard action of Section
// 6 (the prefix is still installed, the session is not reset).
// PREVENTS: the leak this test was written to reproduce — a second copy reaching the RIB
// and the forwarding path because the discard rewrote only the occurrence AttrFind
// returned, carrying label indices from outside the SR domain that can collide with
// locally allocated ones.
//
// RFC requirement: RFC8669-4-1 negative -- a Prefix-SID attribute repeated two or more times by an EBGP peer outside the SR domain has every occurrence discarded, leaving none on the wire.
func TestRFC8669PrefixSIDEveryOccurrenceDiscardedFromEBGP(t *testing.T) {
	tests := []struct {
		name        string
		occurrences int
	}{
		{"one", 1},
		{"two", 2},
		{"three", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := rfc8669EBGPSession(false)
			require.False(t, s.settings.AcceptSRv6PrefixSID,
				"acceptance must be opt-in: the default must be to discard")

			body := makeUpdateBody(nil, rfc8669PrefixSIDAttrsN(tt.occurrences), []byte{24, 10, 0, 0})
			wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
			require.NoError(t, err, "the boundary rule is an attribute discard, never a session reset")
			assert.Equal(t, message.RFC7606ActionAttributeDiscard, action)

			attrs := rfc8669PathAttrs(t, wu.Payload())
			remaining, _ := countAttrCode(attrs, uint8(attribute.AttrPrefixSID))
			assert.Equal(t, 0, remaining,
				"RFC 8669 Section 4: no Prefix-SID occurrence may reach the wire; %d of %d survived",
				remaining, tt.occurrences)

			markers, _ := countAttrCode(attrs, uint8(attribute.AttrTombstone))
			assert.Equal(t, 1, markers,
				"exactly one ATTR_TOMBSTONE records the discard (draft-mangin-idr-attr-tombstone-00 Section 5.1)")

			// The route itself still rides: Section 6's attribute-discard keeps the UPDATE.
			origins, _ := countAttrCode(attrs, 1)
			assert.Equal(t, 1, origins, "the rest of the UPDATE must be processed normally")
		})
	}
}

// TestRFC8669PrefixSIDKeptPathsKeepExactlyOneCopy proves the multi-occurrence discard does
// not over-fire on the two paths where the attribute is legitimately kept.
//
// On both paths the Section 4 branch never runs, so what governs is RFC 7606 Section 3.g:
// "discard all but the first occurrence" of a duplicated non-MP attribute. Exactly one
// copy survives however many arrived, and no ATTR_TOMBSTONE is written because nothing was
// discarded as malformed.
//
// VALIDATES: RFC 8669 Section 4 — "unless it is configured to accept the attribute from
// the EBGP neighbor" — and the IBGP case the rule never covers. RFC 7606 Section 3.g
// keep-first applies to both.
// PREVENTS: a fix to the repeated-attribute leak that strips the attribute from peers
// inside the SR domain, which would make Segment Routing unusable across those sessions.
//
// RFC requirement: RFC8669-4-1 positive -- an EBGP peer configured to be inside the SR domain, and an IBGP peer, keep one Prefix-SID attribute on the wire however many copies arrived.
func TestRFC8669PrefixSIDKeptPathsKeepExactlyOneCopy(t *testing.T) {
	tests := []struct {
		name        string
		session     func() *Session
		occurrences int
	}{
		{"ebgp-configured-to-accept-one", func() *Session { return rfc8669EBGPSession(true) }, 1},
		{"ebgp-configured-to-accept-two", func() *Session { return rfc8669EBGPSession(true) }, 2},
		{"ebgp-configured-to-accept-three", func() *Session { return rfc8669EBGPSession(true) }, 3},
		{"ibgp-one", rfc8669IBGPSession, 1},
		{"ibgp-two", rfc8669IBGPSession, 2},
		{"ibgp-three", rfc8669IBGPSession, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.session()
			body := makeUpdateBody(nil, rfc8669PrefixSIDAttrsN(tt.occurrences), []byte{24, 10, 0, 0})

			wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
			require.NoError(t, err)
			assert.Equal(t, message.RFC7606ActionNone, action,
				"a Prefix-SID the speaker may keep is not an error of any kind")

			attrs := rfc8669PathAttrs(t, wu.Payload())
			remaining, first := countAttrCode(attrs, uint8(attribute.AttrPrefixSID))
			require.Equal(t, 1, remaining,
				"RFC 7606 Section 3.g keep-first: exactly one copy survives, %d arrived", tt.occurrences)
			assert.Equal(t, byte(0x09), first[len(first)-1],
				"the FIRST occurrence is the one kept (its Label-Index is the lowest)")

			markers, _ := countAttrCode(attrs, uint8(attribute.AttrTombstone))
			assert.Equal(t, 0, markers, "nothing was discarded, so no marker may be written")
		})
	}
}

// TestRFC8669PrefixSIDDiscardStillStripsUnrelatedDuplicate is the discriminator for the
// second of the two causes: the Section 4 branch in enforceRFC7606 REPLACED the validator
// result with a freshly constructed one, which carried no DuplicateRanges, so the RFC 7606
// Section 3.g keep-first strip below it was skipped entirely.
//
// The duplicate here is an ORIGIN, an attribute the Prefix-SID discard never touches. No
// change to ApplyAttrDiscard can close this, so it fails on the second cause alone.
//
// VALIDATES: RFC 7606 Section 3.g — "discard all but the first occurrence" still runs when
// the Section 4 discard raises the action on the same UPDATE.
// PREVENTS: any duplicated attribute riding an UPDATE that also carries a Prefix-SID from
// an EBGP peer surviving keep-first, which the attribute index rejects as a hard error and
// which silently drops MP routes at the RIB.
func TestRFC8669PrefixSIDDiscardStillStripsUnrelatedDuplicate(t *testing.T) {
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP        (first: kept)
		0x40, 0x01, 0x01, 0x02, // ORIGIN = INCOMPLETE (later: stripped by Section 3.g)
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP = 192.0.2.1
	}
	value := []byte{1, 0, 7, 0, 0, 0, 0x00, 0x00, 0x03, 0x09}
	attrs = append(attrs, 0xC0, uint8(attribute.AttrPrefixSID), byte(len(value)))
	attrs = append(attrs, value...)

	s := rfc8669EBGPSession(false)
	body := makeUpdateBody(nil, attrs, []byte{24, 10, 0, 0})

	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err)
	require.Equal(t, message.RFC7606ActionAttributeDiscard, action,
		"the Prefix-SID discard is what raises the action here")

	out := rfc8669PathAttrs(t, wu.Payload())

	origins, firstOrigin := countAttrCode(out, 1)
	assert.Equal(t, 1, origins,
		"RFC 7606 Section 3.g keep-first must still strip the duplicate ORIGIN when Section 4 fires")
	require.Len(t, firstOrigin, 1)
	assert.Equal(t, byte(0x00), firstOrigin[0], "the FIRST ORIGIN (IGP) is the one kept")

	prefixSIDs, _ := countAttrCode(out, uint8(attribute.AttrPrefixSID))
	assert.Equal(t, 0, prefixSIDs, "and the Prefix-SID is still discarded")
}
