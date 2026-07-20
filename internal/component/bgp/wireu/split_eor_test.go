package wireu

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RFC 4724 Section 2: a multiprotocol End-of-RIB marker is an UPDATE carrying only an
// MP_UNREACH_NLRI attribute whose value is AFI/SAFI with no withdrawn NLRI. Splitting a
// mixed UPDATE for RFC 7606 Section 5.1 compliance must not MANUFACTURE such a marker: an
// empty MP_UNREACH withdraws nothing, so emitting it alone would forge an EoR that ends a
// peer's graceful-restart deferral early (RFC 4724 Section 4.1).

// eorMarker is the 23-byte wire EoR for IPv4 unicast is the empty UPDATE; for another AF it
// is an MP_UNREACH with AFI/SAFI only. This helper reports whether a split-out body is an
// MP-EoR: no withdrawn routes, no NLRI, and its only attribute is an MP_UNREACH with a
// 3-byte value.
func isMPEoRBody(t *testing.T, body []byte) bool {
	t.Helper()
	wu := NewWireUpdate(body, 0)
	_, isEOR := wu.IsEOR()
	return isEOR
}

// VALIDATES: an UPDATE mixing IPv4 withdrawn routes with an EMPTY MP_UNREACH (AFI/SAFI only)
// is split without producing an End-of-RIB-shaped message.
// PREVENTS: the wire splitter manufacturing a spurious multiprotocol EoR, which a peer in
// graceful restart reads as "table transfer complete" and flushes stale routes on.
//
// RFC requirement: RFC7606-5.1-2 positive -- honoring the split must not forge an RFC 4724
// End-of-RIB marker out of a degenerate empty MP_UNREACH.
func TestSplitWireUpdateDoesNotManufactureEoR(t *testing.T) {
	// IPv4 withdrawn: 20 x /24, enough that with the empty MP_UNREACH the shape is mixed.
	var withdrawn []byte
	for i := range 20 {
		withdrawn = append(withdrawn, 0x18, 0x0a, byte(i), 0x00)
	}
	// MP_UNREACH_NLRI, AFI 2 / SAFI 1, NO withdrawn prefixes -- an EoR marker on its own.
	mpUnreach := []byte{0x80, 0x0f, 0x03, 0x00, 0x02, 0x01}

	attrs := append([]byte{0x40, 0x01, 0x01, 0x00}, mpUnreach...) // ORIGIN + empty MP_UNREACH
	body := []byte{byte(len(withdrawn) >> 8), byte(len(withdrawn))}
	body = append(body, withdrawn...)
	body = append(body, byte(len(attrs)>>8), byte(len(attrs)))
	body = append(body, attrs...)

	wu := NewWireUpdate(body, 0)
	require.True(t, wu.MixesNLRIFields(), "guard: withdrawn + MP_UNREACH is a mixed shape")

	chunks, err := SplitWireUpdate(wu, 120, nil)
	require.NoError(t, err)
	require.NotEmpty(t, chunks)

	for i, c := range chunks {
		assert.Falsef(t, isMPEoRBody(t, c.Payload()),
			"chunk %d is an End-of-RIB marker; splitting must not synthesize one from an "+
				"empty MP_UNREACH", i)
	}

	// The real content -- the IPv4 withdrawals -- must survive.
	var gotWithdrawn int
	for _, c := range chunks {
		wd, err := c.Withdrawn()
		require.NoError(t, err)
		gotWithdrawn += len(wd)
	}
	assert.Equal(t, len(withdrawn), gotWithdrawn, "IPv4 withdrawals must survive the split")
}

// VALIDATES: a genuine standalone multiprotocol EoR is never routed into the splitter and
// is returned unchanged.
// PREVENTS: a real EoR being altered -- it is not mixed, so the fast path must return it as-is.
func TestSplitWireUpdateGenuineEoRUntouched(t *testing.T) {
	body := []byte{0x00, 0x00, 0x00, 0x06, 0x80, 0x0f, 0x03, 0x00, 0x02, 0x01}
	wu := NewWireUpdate(body, 0)
	_, isEOR := wu.IsEOR()
	require.True(t, isEOR, "guard: fixture is a multiprotocol EoR")
	require.False(t, wu.MixesNLRIFields(), "a standalone EoR is not mixed")

	chunks, err := SplitWireUpdate(wu, 120, nil)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Same(t, wu, chunks[0], "a genuine EoR must pass through untouched")
}
