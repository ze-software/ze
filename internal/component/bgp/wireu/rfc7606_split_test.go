package wireu

// The tests below prove that the RE-CHUNK path emits one NLRI-bearing field per
// message. They do NOT prove the full RFC 7606 Section 5.1 second-bullet MUST,
// because two relay paths still reproduce a received mixed shape (forward_body.go
// verbatim forward, and its whole emit of a re-encoded destUpdate that fits).
// Tagging them as proof of RFC7606-5.1-2 overclaims, and `./le rfc check`
// reports the contradiction against the requirement's surviving {gap} annotation.
// They stay as regression protection for the narrowed behavior that annotation
// describes.

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RFC 7606 Section 5.1, second bullet: "An UPDATE message MUST NOT contain more than one of
// the following: non-empty Withdrawn Routes field, non-empty Network Layer Reachability
// Information field, MP_REACH_NLRI attribute, and MP_UNREACH_NLRI attribute."
//
// buildCombinedUpdates used to fill all four into one message per iteration. It now drains
// each component into its own message. Nothing pinned that before this file, so a refactor
// could silently restore the violation while docs/features/rfc-status.md and
// rfc/short/rfc7606.md continued to describe the narrowed behavior.

// nlriBearingFields counts how many of the four NLRI-bearing fields an UPDATE body carries.
// Returns the count plus which ones, for a readable failure.
func nlriBearingFields(t *testing.T, body []byte) (int, []string) {
	t.Helper()
	require.GreaterOrEqual(t, len(body), 4, "UPDATE body must hold both length fields")

	var present []string
	withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
	if withdrawnLen > 0 {
		present = append(present, "withdrawn-routes")
	}
	off := 2 + withdrawnLen
	require.LessOrEqual(t, off+2, len(body))
	attrLen := int(binary.BigEndian.Uint16(body[off : off+2]))
	off += 2
	require.LessOrEqual(t, off+attrLen, len(body))
	attrs := body[off : off+attrLen]
	if len(body)-(off+attrLen) > 0 {
		present = append(present, "nlri")
	}

	for pos := 0; pos < len(attrs); {
		require.LessOrEqual(t, pos+2, len(attrs))
		flags, code := attrs[pos], attrs[pos+1]
		pos += 2
		var l int
		if flags&0x10 != 0 {
			l = int(binary.BigEndian.Uint16(attrs[pos : pos+2]))
			pos += 2
		} else {
			l = int(attrs[pos])
			pos++
		}
		switch code {
		case 14:
			present = append(present, "mp-reach")
		case 15:
			present = append(present, "mp-unreach")
		}
		pos += l
	}
	return len(present), present
}

// mixedUpdateBody builds an UPDATE carrying all four NLRI-bearing fields at once: IPv4
// withdrawn routes, an MP_UNREACH, ORIGIN plus an MP_REACH, and IPv4 NLRI.
func mixedUpdateBody() []byte {
	var withdrawn []byte
	for i := range 20 {
		withdrawn = append(withdrawn, 0x18, 0x0a, byte(i), 0x00) // 10.i.0.0/24
	}

	mpUnreachValue := []byte{0x00, 0x02, 0x01} // AFI IPv6, SAFI unicast
	for i := range 5 {
		mpUnreachValue = append(mpUnreachValue,
			0x40, 0x20, 0x01, 0x0d, 0xb8, 0x00, byte(i), 0x00, 0x00) // 2001:db8:0:i::/64
	}
	mpUnreach := append([]byte{0x90, 0x0f,
		byte(len(mpUnreachValue) >> 8), byte(len(mpUnreachValue))}, mpUnreachValue...)

	mpReachValue := []byte{
		0x00, 0x02, 0x01, 0x10,
		0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x00, 0x01,
		0x00,
	}
	for i := range 5 {
		mpReachValue = append(mpReachValue,
			0x40, 0x20, 0x01, 0x0d, 0xb8, 0x01, byte(i), 0x00, 0x00) // 2001:db8:1:i::/64
	}
	mpReach := append([]byte{0x90, 0x0e,
		byte(len(mpReachValue) >> 8), byte(len(mpReachValue))}, mpReachValue...)

	attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN = IGP
	attrs = append(attrs, mpUnreach...)
	attrs = append(attrs, mpReach...)

	var nlri []byte
	for i := range 20 {
		nlri = append(nlri, 0x18, 0xc0, 0x00, byte(i)) // 192.0.i.0/24
	}

	body := []byte{byte(len(withdrawn) >> 8), byte(len(withdrawn))}
	body = append(body, withdrawn...)
	body = append(body, byte(len(attrs)>>8), byte(len(attrs)))
	body = append(body, attrs...)
	body = append(body, nlri...)
	return body
}

// VALIDATES: every UPDATE ze re-chunks carries at most one NLRI-bearing field.
// PREVENTS: the pre-change buildCombinedUpdates, which packed IPv4 withdrawn, MP_UNREACH,
// MP_REACH and IPv4 NLRI into a single message per iteration -- four at once.
//
// NOT an RFC requirement tag: see the approval note at the top of this file.
func TestSplitWireUpdateOneNLRIFieldPerMessage(t *testing.T) {
	body := mixedUpdateBody()
	before, which := nlriBearingFields(t, body)
	require.Equal(t, 4, before, "guard: the fixture must start non-compliant, got %v", which)

	wu := NewWireUpdate(body, 0)
	chunks, err := SplitWireUpdate(wu, 120, nil)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "an oversized mixed UPDATE must split")

	for i, c := range chunks {
		n, present := nlriBearingFields(t, c.Payload())
		assert.LessOrEqualf(t, n, 1,
			"chunk %d carries %d NLRI-bearing fields (%v); RFC 7606 Section 5.1 allows at most one",
			i, n, present)
	}
}

// VALIDATES: withdrawals still precede announcements across the emitted messages.
// PREVENTS: a peer briefly holding a route ze meant to withdraw. Splitting into separate
// messages makes the ordering observable where it previously did not exist, so it has to
// be asserted rather than assumed.
//
// Also pins Section 5.1's FIRST bullet ordering choice, which is ze's deliberate divergence
// (docs/architecture/wire/mp-nlri-ordering.md): MP_UNREACH is emitted before MP_REACH.
//
// NOT an RFC requirement tag: see the approval note at the top of this file.
func TestSplitWireUpdateWithdrawalsPrecedeAnnouncements(t *testing.T) {
	wu := NewWireUpdate(mixedUpdateBody(), 0)
	chunks, err := SplitWireUpdate(wu, 120, nil)
	require.NoError(t, err)

	seenAnnounce := false
	var order []string
	for _, c := range chunks {
		_, present := nlriBearingFields(t, c.Payload())
		require.Len(t, present, 1)
		order = append(order, present[0])
		switch present[0] {
		case "withdrawn-routes", "mp-unreach":
			assert.Falsef(t, seenAnnounce,
				"a withdrawal (%s) was emitted after an announcement; order was %v",
				present[0], order)
		case "mp-reach", "nlri":
			seenAnnounce = true
		}
	}
	// MP_UNREACH before MP_REACH: the Section 5.1 first-bullet divergence, unchanged.
	var iUnreach, iReach = -1, -1
	for i, o := range order {
		if o == "mp-unreach" && iUnreach < 0 {
			iUnreach = i
		}
		if o == "mp-reach" && iReach < 0 {
			iReach = i
		}
	}
	require.NotEqual(t, -1, iUnreach)
	require.NotEqual(t, -1, iReach)
	assert.Less(t, iUnreach, iReach, "MP_UNREACH must precede MP_REACH; order was %v", order)
}

// VALIDATES: splitting into one field per message loses nothing.
// PREVENTS: the restructure silently dropping a component, which is exactly the defect
// that exists today in the sibling parsed splitter (message.Splitter.splitUpdateWithMP
// drops IPv4 withdrawn and NLRI when an UPDATE also carries an MP attribute).
func TestSplitWireUpdatePreservesEveryField(t *testing.T) {
	wu := NewWireUpdate(mixedUpdateBody(), 0)
	chunks, err := SplitWireUpdate(wu, 120, nil)
	require.NoError(t, err)

	counts := map[string]int{}
	for _, c := range chunks {
		_, present := nlriBearingFields(t, c.Payload())
		for _, p := range present {
			counts[p]++
		}
	}
	for _, want := range []string{"withdrawn-routes", "mp-unreach", "mp-reach", "nlri"} {
		assert.Positivef(t, counts[want], "%s vanished from the split output", want)
	}
}
