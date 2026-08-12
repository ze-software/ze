package wireu

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// wkPayload builds a minimal announce UPDATE body carrying ORIGIN, AS_PATH and,
// when values is non-empty, a COMMUNITIES attribute holding them. The NLRI is
// 10.0.0.0/8, whose first octet is 0x08 -- the COMMUNITIES type code -- so every
// test here also proves the scan reads the attribute SECTION and not the bytes.
func wkPayload(values ...attribute.Community) []byte {
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN igp
		0x40, 0x02, 0x00, // AS_PATH empty
	}
	if len(values) > 0 {
		attrs = append(attrs, 0xC0, 0x08, byte(4*len(values)))
		for _, v := range values {
			attrs = binary.BigEndian.AppendUint32(attrs, uint32(v))
		}
	}
	body := []byte{0x00, 0x00} // withdrawn routes length 0
	body = binary.BigEndian.AppendUint16(body, uint16(len(attrs)))
	body = append(body, attrs...)
	return append(body, 0x08, 0x0A) // NLRI 10.0.0.0/8
}

// RFC requirement: RFC1997-Well-1 positive -- "All routes received carrying a communities
// attribute containing this value [NO_EXPORT] MUST NOT be advertised outside a BGP
// confederation boundary" (RFC 1997, Well-known Communities). Ze configures no
// confederation, so every AS it runs is "a stand-alone autonomous system that is not part
// of a confederation", which that same sentence tells us to consider a confederation
// itself: the boundary is the AS boundary and an EXTERNAL peer is outside it.
func TestWellKnownNoExportRefusesExternalPeer(t *testing.T) {
	t.Parallel()
	w, _ := ScanWellKnown(wkPayload(attribute.CommunityNoExport))
	require.Equal(t, WKNoExport, w)
	assert.False(t, w.AllowsEgressTo(false), "NO_EXPORT route must not reach an external peer")
	assert.Equal(t, "no-export", w.BlockingName(false))
}

// RFC requirement: RFC1997-Well-1 negative -- the clause's condition is "outside a BGP
// confederation boundary", and an INTERNAL peer is inside it, so the prohibition does not
// fire and the same route is advertised. Pairs with the positive above: without this the
// positive would also pass for an implementation that advertises the route to nobody.
func TestWellKnownNoExportAllowsInternalPeer(t *testing.T) {
	t.Parallel()
	w, _ := ScanWellKnown(wkPayload(attribute.CommunityNoExport))
	assert.True(t, w.AllowsEgressTo(true), "NO_EXPORT route must still reach an internal peer")
	assert.Empty(t, w.BlockingName(true))
}

// RFC requirement: RFC1997-Well-2 positive -- "All routes received carrying a communities
// attribute containing this value [NO_ADVERTISE] MUST NOT be advertised to other BGP
// peers" (RFC 1997, Well-known Communities). "Other BGP peers" is unqualified, so this is
// the strictest of the three: internal peers are refused as well as external ones.
func TestWellKnownNoAdvertiseRefusesEveryPeer(t *testing.T) {
	t.Parallel()
	w, _ := ScanWellKnown(wkPayload(attribute.CommunityNoAdvertise))
	require.Equal(t, WKNoAdvertise, w)
	assert.False(t, w.AllowsEgressTo(false), "NO_ADVERTISE route must not reach an external peer")
	assert.False(t, w.AllowsEgressTo(true), "NO_ADVERTISE route must not reach an internal peer either")
	assert.Equal(t, "no-advertise", w.BlockingName(true))
}

// RFC requirement: RFC1997-Well-2 negative -- the clause's condition is a communities
// attribute CONTAINING NO_ADVERTISE. The same prefix, the same peers, one ordinary
// community instead: the prohibition does not fire and the route is advertised to both.
func TestWellKnownNoAdvertiseAbsentAdvertisesToEveryPeer(t *testing.T) {
	t.Parallel()
	w, _ := ScanWellKnown(wkPayload(attribute.Community(0xFDE90064)))
	require.Equal(t, WellKnown(0), w)
	assert.True(t, w.AllowsEgressTo(false))
	assert.True(t, w.AllowsEgressTo(true))
}

// RFC requirement: RFC1997-Well-3 positive -- "All routes received carrying a communities
// attribute containing this value [NO_EXPORT_SUBCONFED] MUST NOT be advertised to
// external BGP peers (this includes peers in other members autonomous systems inside a
// BGP confederation)" (RFC 1997, Well-known Communities).
func TestWellKnownNoExportSubconfedRefusesExternalPeer(t *testing.T) {
	t.Parallel()
	w, _ := ScanWellKnown(wkPayload(attribute.CommunityNoExportSubconfed))
	require.Equal(t, WKNoExportSubconfed, w)
	assert.False(t, w.AllowsEgressTo(false), "NO_EXPORT_SUBCONFED route must not reach an external peer")
	assert.Equal(t, "no-export-subconfed", w.BlockingName(false))
}

// RFC requirement: RFC1997-Well-3 negative -- the clause names EXTERNAL peers only, and
// the parenthesis extends that to confederation member-AS peers, not to internal ones. An
// internal peer is therefore outside the condition and the same route is advertised to it.
func TestWellKnownNoExportSubconfedAllowsInternalPeer(t *testing.T) {
	t.Parallel()
	w, _ := ScanWellKnown(wkPayload(attribute.CommunityNoExportSubconfed))
	assert.True(t, w.AllowsEgressTo(true), "NO_EXPORT_SUBCONFED route must still reach an internal peer")
}

// RFC requirement: RFC1997-Well-4 positive -- "The following communities have global
// significance and their operations shall be implemented in any community-attribute-aware
// BGP speaker" (RFC 1997, Well-known Communities). All three operations are applied from
// the wire values alone, with no operator policy configured anywhere in this test.
func TestWellKnownAllThreeOperationsImplemented(t *testing.T) {
	t.Parallel()
	w, _ := ScanWellKnown(wkPayload(
		attribute.CommunityNoExport,
		attribute.CommunityNoAdvertise,
		attribute.CommunityNoExportSubconfed,
	))
	require.Equal(t, WKNoExport|WKNoAdvertise|WKNoExportSubconfed, w)
	// The strictest of the set decides, so a route carrying all three reaches nobody.
	assert.False(t, w.AllowsEgressTo(true))
	assert.False(t, w.AllowsEgressTo(false))
	assert.Equal(t, "no-advertise", w.BlockingName(false))
}

// RFC requirement: RFC1997-Well-4 negative -- "the following communities" is an
// enumeration of exactly three values. A community in the same reserved 0xFFFF0000 block
// that is NOT one of them carries no RFC 1997 egress operation, so the route is
// advertised: LLGR_STALE (RFC 9494), NOPEER (RFC 3765) and BLACKHOLE (RFC 7999) each have
// their own semantics and none of them is an egress prohibition this speaker applies here.
func TestWellKnownIgnoresOtherReservedCommunities(t *testing.T) {
	t.Parallel()
	w, _ := ScanWellKnown(wkPayload(
		attribute.CommunityNoPeer,
		attribute.CommunityLLGRStale,
		attribute.CommunityBlackhole,
		attribute.CommunityGracefulShutdown,
	))
	require.Equal(t, WellKnown(0), w)
	assert.True(t, w.AllowsEgressTo(false))
	assert.True(t, w.AllowsEgressTo(true))
}

// VALIDATES: the scan reads the COMMUNITIES attribute section, never the raw payload.
// PREVENTS: an NLRI or a next-hop octet run that happens to spell 0xFFFFFF01 being read
// as NO_EXPORT and silently withholding a route nobody tagged.
func TestScanWellKnownIgnoresNonAttributeBytes(t *testing.T) {
	t.Parallel()
	// Attributes: ORIGIN only. NLRI: 255.255.255.1/32, whose four octets ARE NO_EXPORT.
	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	body := []byte{0x00, 0x00}
	body = binary.BigEndian.AppendUint16(body, uint16(len(attrs)))
	body = append(body, attrs...)
	body = append(body, 0x20, 0xFF, 0xFF, 0xFF, 0x01)
	w, ok := ScanWellKnown(body)
	assert.True(t, ok)
	assert.Equal(t, WellKnown(0), w)
}

// VALIDATES: a readable payload with no COMMUNITIES attribute, one with an empty
// COMMUNITIES attribute, and a withdraw-only UPDATE all answer the empty set and report
// that they were read.
// PREVENTS: an absent value being read as a prohibition, which would withhold every route
// that carries no community at all.
func TestScanWellKnownEmptyAnswers(t *testing.T) {
	t.Parallel()
	read := func(payload []byte) WellKnown {
		t.Helper()
		w, ok := ScanWellKnown(payload)
		assert.True(t, ok, "the payload is well formed, so the scan must report it was read")
		return w
	}
	assert.Equal(t, WellKnown(0), read(wkPayload()))
	// Withdraw-only UPDATE: 2 octets of withdrawn routes, no attributes, no NLRI.
	assert.Equal(t, WellKnown(0), read([]byte{0x00, 0x02, 0x08, 0x0A, 0x00, 0x00}))
	// COMMUNITIES present but zero-length. RFC 7606 Section 7.8 calls that malformed and
	// validateCommunityAttr (message/rfc7606.go) treats the route as withdrawn, so no such
	// payload reaches the forward rails. What this case pins is the value loop's tolerance
	// of a section it cannot step through, not the shape being valid.
	attrs := []byte{0x40, 0x01, 0x01, 0x00, 0xC0, 0x08, 0x00}
	body := []byte{0x00, 0x00, 0x00, byte(len(attrs))}
	body = append(body, attrs...)
	assert.Equal(t, WellKnown(0), read(append(body, 0x08, 0x0A)))
}

// VALIDATES: a payload whose sections do not parse answers the empty set AND reports that
// it could not be read.
// PREVENTS: a SILENT fail-open. The empty set advertises the route to every peer, which is
// the right answer for a parse hiccup and the wrong one to reach without saying so: an
// upstream change that puts unparsed bytes on the egress path would otherwise turn this
// branch into a leak nobody can see (ai/rules/evidence.md).
func TestScanWellKnownUnreadablePayloadSaysSo(t *testing.T) {
	t.Parallel()
	for name, payload := range map[string][]byte{
		"nil":                 nil,
		"one octet":           {0x00},
		"withdrawn overruns":  {0x00, 0x08, 0x0A},
		"attribute truncated": {0x00, 0x00, 0x00, 0x10, 0x40},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			w, ok := ScanWellKnown(payload)
			assert.False(t, ok, "an unreadable payload must report that it was not read")
			assert.Equal(t, WellKnown(0), w, "the gate fails OPEN: no prohibition is invented")
			assert.True(t, w.AllowsEgressTo(false))
		})
	}
}

// VALIDATES: ScanWellKnown allocates nothing.
// PREVENTS: an allocation on the egress fan-out, where this runs once per UPDATE and the
// exactly-sized rebuild beside it exists to stay allocation-free (ai/rules/performance.md).
func TestScanWellKnownZeroAlloc(t *testing.T) {
	payload := wkPayload(attribute.Community(0xFDE90064), attribute.CommunityNoExport)
	allocs := testing.AllocsPerRun(200, func() {
		if w, _ := ScanWellKnown(payload); w == 0 {
			t.Fatal("expected NO_EXPORT to be seen")
		}
	})
	assert.Zero(t, allocs, "ScanWellKnown must not allocate")
}
