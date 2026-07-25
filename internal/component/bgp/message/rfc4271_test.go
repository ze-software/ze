package message

import (
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rfc-test-change-approved: 2026-07-22 Thomas approved the msgtype/routeaction
// package rename (spec-feature-gate-10-bgp). MessageType/Type* moved to
// internal/core/bgp/msgtype and the route-action enum to
// internal/core/bgp/routeaction so MRT, sysrib and the FIB backends keep
// compiling when the BGP engine is compiled out (//go:build ze_bgp). Every hunk
// in this file is a package-qualifier requalification: no assertion was added,
// removed, reworded, weakened or re-tagged, verified by normalising the diff
// under the renaming and confirming the add/delete multisets cancel.

// TestRFC4271MarkerAllOnesOnSend verifies every message ze encodes carries the
// all-ones marker written by writeHeader (message.go:31-42).
//
// VALIDATES: The 16-octet marker of an encoded KEEPALIVE/NOTIFICATION is all 0xFF.
//
// PREVENTS: A peer resynchronising the stream because ze emitted a non-ones marker.
//
// RFC requirement: RFC4271-4.1-1 positive -- writeHeader fills the first 16 octets of
// every encoded message with 0xFF (internal/component/bgp/message/message.go:33-35), so
// a packed KEEPALIVE and a packed NOTIFICATION both carry the all-ones marker.
func TestRFC4271MarkerAllOnesOnSend(t *testing.T) {
	for _, msg := range []Message{
		NewKeepalive(),
		&Notification{ErrorCode: NotifyCease, ErrorSubcode: NotifyCeaseAdminShutdown},
	} {
		data := PackTo(msg, nil)
		require.GreaterOrEqual(t, len(data), HeaderLen)
		for i := range MarkerLen {
			assert.Equal(t, byte(0xFF), data[i], "%s marker byte %d", msg.Type(), i)
		}
	}
}

// TestRFC4271MarkerNotAllOnesRejected verifies a received marker that is not all ones is
// refused by ParseHeader (header.go:96-100).
//
// VALIDATES: A single non-0xFF marker octet, at any position, fails the header parse.
//
// PREVENTS: Treating unsynchronised bytes as a framed BGP message.
//
// RFC requirement: RFC4271-4.1-1 negative -- ParseHeader returns ErrInvalidMarker for a
// header whose marker is not 16 octets of 0xFF, at the first, a middle and the last
// marker position (internal/component/bgp/message/header.go:96-100).
func TestRFC4271MarkerNotAllOnesRejected(t *testing.T) {
	for _, pos := range []int{0, 7, MarkerLen - 1} {
		data := makeHeader(HeaderLen, byte(msgtype.TypeKEEPALIVE))
		data[pos] = 0xFE
		_, err := ParseHeader(data)
		assert.ErrorIs(t, err, ErrInvalidMarker, "marker byte %d corrupted", pos)
	}
}

// TestRFC4271SmallestLengthOnSend verifies the Length field ze writes is the smallest
// value the message content requires.
//
// VALIDATES: KEEPALIVE declares exactly 19; an UPDATE declares exactly
// 23 + withdrawn + attrs + NLRI, matching the bytes actually written.
//
// PREVENTS: Padding a message so a peer reads past its real content.
//
// RFC requirement: RFC4271-4.1-2 positive -- Keepalive.Len returns HeaderLen and
// Update.Len returns 23 + withdrawn + attrs + NLRI, and writeHeader stamps that exact
// value into the Length field (internal/component/bgp/message/keepalive.go:41-43,
// internal/component/bgp/message/update.go:120-121, message.go:37-39).
func TestRFC4271SmallestLengthOnSend(t *testing.T) {
	ka := PackTo(NewKeepalive(), nil)
	require.Len(t, ka, HeaderLen)
	hdr, err := ParseHeader(ka)
	require.NoError(t, err)
	assert.Equal(t, uint16(HeaderLen), hdr.Length, "KEEPALIVE declares the 19-octet minimum")

	u := &Update{
		WithdrawnRoutes: []byte{0x18, 0x0a, 0x00, 0x01},
		PathAttributes:  []byte{0x40, 0x01, 0x01, 0x00},
		NLRI:            []byte{0x18, 0x0a, 0x00, 0x02},
	}
	want := HeaderLen + 2 + 4 + 2 + 4 + 4
	buf := make([]byte, 128)
	n := u.WriteTo(buf, 0, nil)
	require.Equal(t, want, n, "bytes written equal the computed smallest length")
	uhdr, err := ParseHeader(buf)
	require.NoError(t, err)
	assert.Equal(t, uint16(want), uhdr.Length) //nolint:gosec // want is a small constant
}

// TestRFC4271NonSmallestLengthRejected verifies a declared Length larger than the message
// content requires is rejected for the one type whose size is fixed.
//
// VALIDATES: A KEEPALIVE declaring 20 or 100 octets fails ValidateLength; a body-carrying
// KEEPALIVE is refused by UnpackKeepalive with the erroneous length in the Data field.
//
// PREVENTS: Accepting a padded KEEPALIVE and resynchronising on the padding.
//
// RFC requirement: RFC4271-4.1-2 negative -- the smallest value required for a KEEPALIVE
// is exactly 19, and ValidateLength rejects any larger declared length
// (internal/component/bgp/message/header.go:140-142,155-162); UnpackKeepalive rejects a
// body with Message Header Error / Bad Message Length carrying the erroneous length
// (internal/component/bgp/message/keepalive.go:59-70).
func TestRFC4271NonSmallestLengthRejected(t *testing.T) {
	for _, length := range []uint16{20, 100} {
		h := Header{Length: length, Type: msgtype.TypeKEEPALIVE}
		err := h.ValidateLength()
		require.Error(t, err, "KEEPALIVE length %d", length)
		var notif *Notification
		require.ErrorAs(t, err, &notif)
		assert.Equal(t, NotifyMessageHeader, notif.ErrorCode)
		assert.Equal(t, NotifyHeaderBadLength, notif.ErrorSubcode)
	}

	_, err := UnpackKeepalive([]byte{0x00})
	require.Error(t, err)
	var notif *Notification
	require.ErrorAs(t, err, &notif)
	assert.Equal(t, []byte{0x00, 0x14}, notif.Data, "Data carries the erroneous length 20")
}

// TestRFC4271MessageLengthWithinBounds verifies lengths inside the 19..4096 window parse.
//
// VALIDATES: 19, a mid-range value and 4096 are all accepted.
//
// PREVENTS: Rejecting legal message sizes.
//
// RFC requirement: RFC4271-4.1-3 positive -- ParseHeader accepts a Length of 19 and
// ValidateLengthWithMax accepts up to 4096 without the Extended Message capability
// (internal/component/bgp/message/header.go:106-108,195-213).
func TestRFC4271MessageLengthWithinBounds(t *testing.T) {
	for _, tc := range []struct {
		typ    msgtype.MessageType
		length uint16
	}{
		{msgtype.TypeKEEPALIVE, HeaderLen},
		{msgtype.TypeUPDATE, 1000},
		{msgtype.TypeUPDATE, MaxMsgLen},
	} {
		data := makeHeader(tc.length, byte(tc.typ))
		h, err := ParseHeader(data)
		require.NoError(t, err, "length %d", tc.length)
		require.Equal(t, tc.length, h.Length)
		assert.NoError(t, h.ValidateLengthWithMax(false), "length %d", tc.length)
	}
}

// TestRFC4271MessageLengthOutOfBounds verifies lengths outside 19..4096 are refused.
//
// VALIDATES: 18 fails the parse; 4097 and 65535 fail the bound check.
//
// PREVENTS: Buffer sizing from an out-of-range declared length.
//
// RFC requirement: RFC4271-4.1-3 negative -- a Length below 19 yields ErrInvalidLength
// (internal/component/bgp/message/header.go:106-108) and a Length above 4096 without the
// Extended Message capability yields Message Header Error / Bad Message Length
// (internal/component/bgp/message/header.go:207-213).
func TestRFC4271MessageLengthOutOfBounds(t *testing.T) {
	_, err := ParseHeader(makeHeader(18, byte(msgtype.TypeUPDATE)))
	assert.ErrorIs(t, err, ErrInvalidLength)

	for _, length := range []uint16{MaxMsgLen + 1, 65535} {
		h := Header{Length: length, Type: msgtype.TypeUPDATE}
		verr := h.ValidateLengthWithMax(false)
		require.Error(t, verr, "length %d", length)
		var notif *Notification
		require.ErrorAs(t, verr, &notif)
		assert.Equal(t, NotifyMessageHeader, notif.ErrorCode)
		assert.Equal(t, NotifyHeaderBadLength, notif.ErrorSubcode)
	}
}

// TestRFC4271BadLengthNotificationCarriesLength verifies the Data field of the header
// error carries the offending Length field.
//
// VALIDATES: Data is the two octets of the erroneous Length, big-endian.
//
// PREVENTS: A peer receiving a diagnostic it cannot correlate with the bad message.
//
// Untagged: RFC4271-6.1-3 is recorded {gap} in rfc/short/rfc4271.md because ParseHeader
// reports a sub-19 Length with a bare sentinel and no NOTIFICATION
// (internal/component/bgp/message/header.go:106-108). This test covers only the four
// length conditions that DO produce a conformant Notification
// (internal/component/bgp/message/header.go:155-171,207-213), so it cannot stand as
// coverage of the whole obligation.
func TestRFC4271BadLengthNotificationCarriesLength(t *testing.T) {
	h := Header{Length: 4097, Type: msgtype.TypeUPDATE}
	err := h.ValidateLengthWithMax(false)
	require.Error(t, err)
	var notif *Notification
	require.ErrorAs(t, err, &notif)
	assert.Equal(t, NotifyMessageHeader, notif.ErrorCode)
	assert.Equal(t, NotifyHeaderBadLength, notif.ErrorSubcode)
	assert.Equal(t, []byte{0x10, 0x01}, notif.Data, "4097 big-endian")

	short := Header{Length: 22, Type: msgtype.TypeUPDATE}
	err = short.ValidateLength()
	require.Error(t, err)
	require.ErrorAs(t, err, &notif)
	assert.Equal(t, []byte{0x00, 0x16}, notif.Data, "22 big-endian")
}

// TestRFC4271ValidLengthProducesNoNotification verifies a well-formed length is not
// reported as a header error.
//
// VALIDATES: The per-type minimum and the 4096 ceiling both validate clean.
//
// PREVENTS: Spurious Bad Message Length NOTIFICATIONs tearing down healthy sessions.
//
// Untagged for the same reason as the test above: a length that satisfies the per-type
// minimum and the ceiling returns nil, so no Message Header Error is raised
// (internal/component/bgp/message/header.go:172-173,215).
func TestRFC4271ValidLengthProducesNoNotification(t *testing.T) {
	for _, tc := range []struct {
		typ msgtype.MessageType
		len uint16
	}{
		{msgtype.TypeOPEN, MinOpenLen},
		{msgtype.TypeUPDATE, MinUpdateLen},
		{msgtype.TypeNOTIFICATION, MinNotificationLen},
		{msgtype.TypeKEEPALIVE, KeepaliveLen},
		{msgtype.TypeUPDATE, MaxMsgLen},
	} {
		h := Header{Length: tc.len, Type: tc.typ}
		assert.NoError(t, h.ValidateLengthWithMax(false), "%s length %d", tc.typ, tc.len)
	}
}

// TestRFC4271AttrFlagsLowNibbleIgnoredOnReceive verifies the unused low-order four bits
// of the attribute flags octet do not change how an attribute is handled.
//
// VALIDATES: ORIGIN/AS_PATH/NEXT_HOP with flags 0x4F validate exactly as with 0x40.
//
// PREVENTS: Rejecting an interoperable UPDATE because a peer left reserved bits set.
//
// RFC requirement: RFC4271-4.3-4 positive -- validateAttributeFlags inspects only the
// Optional (0x80) and Transitive (0x40) bits, so the reserved low-order four bits are
// ignored and an UPDATE carrying them validates clean
// (internal/component/bgp/message/rfc7606.go:99-147).
func TestRFC4271AttrFlagsLowNibbleIgnoredOnReceive(t *testing.T) {
	pathAttrs := []byte{
		0x4F, 0x01, 0x01, 0x00, // ORIGIN = IGP, low nibble all set
		0x4F, 0x02, 0x00, // AS_PATH (empty), low nibble all set
		0x4F, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP, low nibble all set
	}
	// 0x4F has the Extended Length bit (0x10) set, which is a real encoding
	// choice; strip it here so the remaining reserved bits are what is exercised.
	pathAttrs[0] = 0x4F &^ 0x10
	pathAttrs[4] = 0x4F &^ 0x10
	pathAttrs[7] = 0x4F &^ 0x10

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action, result.Description)
}

// TestRFC4271AttrFlagsHighBitsNotIgnored verifies the flag check is not vacuous: the
// defined high-order bits are still enforced.
//
// VALIDATES: ORIGIN marked Optional, and ORIGIN marked non-Transitive, are both rejected.
//
// PREVENTS: A blanket "ignore all flags" reading of the low-nibble rule.
//
// RFC requirement: RFC4271-4.3-4 negative -- ignoring the reserved low-order bits does
// not extend to the defined bits: a well-known attribute with the Optional bit set, or
// with the Transitive bit clear, is still refused
// (internal/component/bgp/message/rfc7606.go:126-144).
func TestRFC4271AttrFlagsHighBitsNotIgnored(t *testing.T) {
	optional := []byte{
		0xCF, 0x01, 0x01, 0x00, // ORIGIN marked optional, low nibble set
		0x40, 0x02, 0x00,
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01,
	}
	optional[0] = 0xCF &^ 0x10
	res := ValidateUpdateRFC7606(optional, true, false, false)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, res.Action)
	assert.Equal(t, uint8(1), res.AttrCode)

	nonTransitive := []byte{
		0x0F, 0x01, 0x01, 0x00, // ORIGIN with Transitive clear, low nibble set
		0x40, 0x02, 0x00,
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01,
	}
	nonTransitive[0] = 0x0F &^ 0x10
	res = ValidateUpdateRFC7606(nonTransitive, true, false, false)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, res.Action)
	assert.Equal(t, uint8(1), res.AttrCode)
}

// TestRFC4271AttributesOutOfOrderAccepted verifies attribute order is not load-bearing on
// receive.
//
// VALIDATES: NEXT_HOP, AS_PATH, ORIGIN (descending type code) validates exactly as the
// ascending order does.
//
// PREVENTS: Dropping a legal UPDATE from an implementation that does not sort attributes.
//
// RFC requirement: RFC4271-5-6 positive -- ValidateUpdateRFC7606 walks the attribute
// section sequentially with no ordering constraint, so a descending-type-code attribute
// list is accepted (internal/component/bgp/message/rfc7606.go:213-326).
func TestRFC4271AttributesOutOfOrderAccepted(t *testing.T) {
	descending := []byte{
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP (3)
		0x40, 0x02, 0x00, // AS_PATH (2)
		0x40, 0x01, 0x01, 0x00, // ORIGIN (1)
	}
	res := ValidateUpdateRFC7606(descending, true, false, false)
	require.Equal(t, RFC7606ActionNone, res.Action, res.Description)

	interleaved := []byte{
		0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x0a, // MED (4)
		0x40, 0x01, 0x01, 0x00, // ORIGIN (1)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP (3)
		0x40, 0x02, 0x00, // AS_PATH (2)
	}
	res = ValidateUpdateRFC7606(interleaved, true, false, false)
	require.Equal(t, RFC7606ActionNone, res.Action, res.Description)
}

// TestRFC4271OutOfOrderDoesNotMaskMalformation verifies out-of-order tolerance is not a
// blanket "accept anything".
//
// VALIDATES: A descending-order list whose ORIGIN is malformed is still caught, and a
// truncated attribute section is still refused.
//
// PREVENTS: Reading "handle any order" as "skip validation".
//
// RFC requirement: RFC4271-5-6 negative -- order tolerance does not suppress the
// per-attribute checks: an out-of-order list with a bad ORIGIN length is treat-as-withdraw
// and a truncated attribute header is refused
// (internal/component/bgp/message/rfc7606.go:255-264,441-458).
func TestRFC4271OutOfOrderDoesNotMaskMalformation(t *testing.T) {
	badOrigin := []byte{
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP first
		0x40, 0x02, 0x00, // AS_PATH
		0x40, 0x01, 0x02, 0x00, 0x00, // ORIGIN length 2 -- malformed
	}
	res := ValidateUpdateRFC7606(badOrigin, true, false, false)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, res.Action)
	assert.Equal(t, uint8(1), res.AttrCode)

	truncated := []byte{
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01,
		0x40, 0x02, 0x08, 0x02, // AS_PATH claims 8 octets, 1 present
	}
	res = ValidateUpdateRFC7606(truncated, true, false, false)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, res.Action)
}

// TestRFC4271NotificationUnspecifiedSubcodeIsZero verifies a NOTIFICATION raised without a
// specific subcode encodes a zero subcode octet.
//
// VALIDATES: Hold Timer Expired, which has no subcodes, encodes subcode 0 on the wire and
// decodes back to 0.
//
// PREVENTS: Emitting an undefined subcode value for an error that defines none.
//
// RFC requirement: RFC4271-6-1 positive -- the subcode octet is written unconditionally
// from Notification.ErrorSubcode, so an error raised with no subcode specified puts a zero
// on the wire (internal/component/bgp/message/notification.go:203).
func TestRFC4271NotificationUnspecifiedSubcodeIsZero(t *testing.T) {
	n := &Notification{ErrorCode: NotifyHoldTimerExpired}
	data := PackTo(n, nil)
	require.Len(t, data, HeaderLen+2)
	assert.Equal(t, byte(NotifyHoldTimerExpired), data[HeaderLen])
	assert.Equal(t, byte(0), data[HeaderLen+1], "unspecified subcode encodes as zero")

	back, err := UnpackNotification(data[HeaderLen:])
	require.NoError(t, err)
	assert.Equal(t, uint8(0), back.ErrorSubcode)
}

// TestRFC4271NotificationSpecifiedSubcodePreserved verifies the zero default does not
// overwrite a subcode that was specified.
//
// VALIDATES: Cease/Administrative Shutdown and UPDATE/Malformed Attribute List keep their
// non-zero subcodes through pack and unpack.
//
// PREVENTS: Collapsing every NOTIFICATION to subcode zero.
//
// RFC requirement: RFC4271-6-1 negative -- a specified subcode is not replaced by the
// zero default: the encoder writes the caller's value verbatim
// (internal/component/bgp/message/notification.go:203).
func TestRFC4271NotificationSpecifiedSubcodePreserved(t *testing.T) {
	for _, tc := range []struct {
		code    NotifyErrorCode
		subcode uint8
	}{
		{NotifyCease, NotifyCeaseAdminShutdown},
		{NotifyUpdateMessage, NotifyUpdateMalformedAttr},
		{NotifyOpenMessage, NotifyOpenUnacceptableHoldTime},
	} {
		data := PackTo(&Notification{ErrorCode: tc.code, ErrorSubcode: tc.subcode}, nil)
		assert.Equal(t, tc.subcode, data[HeaderLen+1], "code %d subcode", tc.code)
		assert.NotEqual(t, byte(0), data[HeaderLen+1])
	}
}

// TestRFC4271OversizeSingleRouteNotAdvertised verifies a single route that cannot fit in an
// UPDATE is not emitted.
//
// VALIDATES: An UPDATE whose path attributes alone exceed the message ceiling produces an
// error and no emitted UPDATE at all.
//
// PREVENTS: Emitting a truncated UPDATE for a route that does not fit.
//
// RFC requirement: RFC4271-9.2-10 positive -- when the attributes of a single route do not
// fit within the message size, Split returns ErrAttributesTooLarge and calls emit zero
// times, so nothing is advertised (internal/component/bgp/message/update_split.go:357-366).
func TestRFC4271OversizeSingleRouteNotAdvertised(t *testing.T) {
	s := NewSplitter()
	attrs := make([]byte, 300)
	attrs[0], attrs[1], attrs[2] = 0x40, 0x01, 0x01
	u := &Update{
		PathAttributes: attrs,
		NLRI:           []byte{24, 10, 0, 0},
	}

	emitted := 0
	err := s.Split(u, HeaderLen+4+200, false, func(*Update) error {
		emitted++
		return nil
	})
	require.Error(t, err, "an oversize single route must not be advertised")
	assert.Zero(t, emitted, "no UPDATE is emitted for a route that does not fit")
}

// TestRFC4271FittingRouteIsAdvertised verifies the refusal is specific to routes that do
// not fit.
//
// VALIDATES: The same route with a message ceiling that accommodates it is emitted once,
// carrying its attributes and NLRI.
//
// PREVENTS: Reading the oversize refusal as a splitter that never emits.
//
// RFC requirement: RFC4271-9.2-10 negative -- a route that does fit is advertised
// normally, so the suppression above is driven by the size check and not by a blanket
// refusal (internal/component/bgp/message/update_split.go:357-403).
func TestRFC4271FittingRouteIsAdvertised(t *testing.T) {
	s := NewSplitter()
	attrs := make([]byte, 300)
	attrs[0], attrs[1], attrs[2] = 0x40, 0x01, 0x01
	u := &Update{
		PathAttributes: attrs,
		NLRI:           []byte{24, 10, 0, 0},
	}

	var got []*Update
	err := s.Split(u, MaxMsgLen, false, func(chunk *Update) error {
		got = append(got, chunk)
		return nil
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, attrs, got[0].PathAttributes)
	assert.Equal(t, []byte{24, 10, 0, 0}, got[0].NLRI)
}

// TestRFC4271WellKnownAttributesAreRecognized verifies the receiver knows every well-known
// attribute type rather than treating unknown codes alike.
//
// VALIDATES: An UPDATE carrying ORIGIN, AS_PATH, NEXT_HOP, MED, LOCAL_PREF, ATOMIC_AGGREGATE
// and AGGREGATOR validates clean on an internal session, and each well-known code runs its
// own validator.
//
// PREVENTS: Silently ignoring a well-known attribute the RFC requires every speaker to know.
//
// RFC requirement: RFC4271-5-1 positive -- every RFC 4271 well-known attribute has a
// registered per-code validator that runs on receipt, so the code is recognized rather than
// skipped (internal/component/bgp/message/rfc7606.go:414-438).
// RFC requirement: RFC4271-5-2 positive -- an UPDATE carrying NLRI together with all three
// well-known mandatory attributes is accepted
// (internal/component/bgp/message/rfc7606.go:341-363).
func TestRFC4271WellKnownAttributesAreRecognized(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
		0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x0a, // MED
		0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64, // LOCAL_PREF
		0x40, 0x06, 0x00, // ATOMIC_AGGREGATE
		0xc0, 0x07, 0x08, 0x00, 0x00, 0xfd, 0xe9, 0xc0, 0x00, 0x02, 0x01, // AGGREGATOR (4-octet AS)
	}
	res := ValidateUpdateRFC7606(pathAttrs, true, true, true)
	require.Equal(t, RFC7606ActionNone, res.Action, res.Description)
}

// TestRFC4271WellKnownAttributeErrorsAreCaught verifies recognition is real: a malformation
// in a well-known attribute is detected per type code.
//
// VALIDATES: A bad length on each of ORIGIN, NEXT_HOP, MED, LOCAL_PREF, ATOMIC_AGGREGATE and
// AGGREGATOR is reported against that attribute's own type code; a well-known attribute
// marked Optional is refused; and a missing well-known mandatory attribute is refused.
//
// PREVENTS: A validator table that accepts anything, which would make the positive vacuous.
//
// RFC requirement: RFC4271-5-1 negative -- each well-known code is dispatched to its own
// validator and a malformation is reported against that code, which is only possible if the
// code was recognized (internal/component/bgp/message/rfc7606.go:414-438,441-545); a
// well-known attribute carrying the Optional bit is refused
// (internal/component/bgp/message/rfc7606.go:121-134).
// RFC requirement: RFC4271-5-2 negative -- an UPDATE carrying NLRI but missing a well-known
// mandatory attribute is not accepted
// (internal/component/bgp/message/rfc7606.go:341-363).
// RFC requirement: RFC4271-4.3-1 negative -- a well-known attribute received with the
// Transitive bit clear is non-conformant and is refused
// (internal/component/bgp/message/rfc7606.go:136-144).
func TestRFC4271WellKnownAttributeErrorsAreCaught(t *testing.T) {
	origin := []byte{0x40, 0x01, 0x01, 0x00}
	asPath := []byte{0x40, 0x02, 0x00}
	nextHop := []byte{0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01}
	base := func(parts ...[]byte) []byte {
		var out []byte
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}

	// Per-code malformation: each is reported against its own type code. The malformed
	// attribute is the only instance of that code, so no duplicate suppression applies.
	for _, tc := range []struct {
		name  string
		attrs []byte
		code  uint8
	}{
		{"ORIGIN length", base([]byte{0x40, 0x01, 0x02, 0x00, 0x00}, asPath, nextHop), 1},
		{"NEXT_HOP length", base(origin, asPath, []byte{0x40, 0x03, 0x03, 0xc0, 0x00, 0x02}), 3},
		{"MED length", base(origin, asPath, nextHop, []byte{0x80, 0x04, 0x03, 0x00, 0x00, 0x00}), 4},
		{"LOCAL_PREF length", base(origin, asPath, nextHop, []byte{0x40, 0x05, 0x03, 0x00, 0x00, 0x00}), 5},
		{"ATOMIC_AGGREGATE length", base(origin, asPath, nextHop, []byte{0x40, 0x06, 0x01, 0x00}), 6},
		{"AGGREGATOR length", base(origin, asPath, nextHop, []byte{0xc0, 0x07, 0x05, 0x00, 0x00, 0x00, 0x00, 0x00}), 7},
	} {
		res := ValidateUpdateRFC7606(tc.attrs, true, true, true)
		require.NotEqual(t, RFC7606ActionNone, res.Action, "%s must be caught", tc.name)
		assert.Equal(t, tc.code, res.AttrCode, "%s reported against its own type code", tc.name)
	}

	// A well-known attribute marked Optional is refused.
	optional := base(origin, asPath, nextHop)
	optional[0] = 0xC0
	res := ValidateUpdateRFC7606(optional, true, true, true)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, res.Action)
	assert.Equal(t, uint8(1), res.AttrCode)

	// A well-known attribute with the Transitive bit clear is refused.
	nonTransitive := base(origin, asPath, nextHop)
	nonTransitive[0] = 0x00
	res = ValidateUpdateRFC7606(nonTransitive, true, true, true)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, res.Action)
	assert.Equal(t, uint8(1), res.AttrCode)

	// A missing well-known mandatory attribute is refused.
	missingOrigin := []byte{
		0x40, 0x02, 0x00,
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01,
	}
	res = ValidateUpdateRFC7606(missingOrigin, true, true, true)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, res.Action)
	assert.Equal(t, uint8(1), res.AttrCode, "the missing ORIGIN is named")
}

// TestRFC4271LocalPrefKeptOnInternalSession verifies LOCAL_PREF received from an internal
// peer is retained.
//
// VALIDATES: An UPDATE with LOCAL_PREF on an internal session validates clean with no
// discard.
//
// PREVENTS: Dropping the preference an internal peer legitimately signaled.
//
// RFC requirement: RFC4271-5.1.5-3 positive -- the ignore rule is scoped to external
// sessions: on an internal session LOCAL_PREF is kept
// (internal/component/bgp/message/rfc7606.go:493-510).
func TestRFC4271LocalPrefKeptOnInternalSession(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00,
		0x40, 0x02, 0x00,
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01,
		0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64,
	}
	res := ValidateUpdateRFC7606(pathAttrs, true, true, false)
	require.Equal(t, RFC7606ActionNone, res.Action, res.Description)
	assert.Empty(t, res.DiscardEntries)
}

// TestRFC4271LocalPrefIgnoredOnExternalSession verifies a LOCAL_PREF arriving over EBGP is
// discarded.
//
// VALIDATES: The same UPDATE on an external session selects attribute-discard and names
// LOCAL_PREF as the attribute to drop.
//
// PREVENTS: An external peer dictating this speaker's local preference.
//
// RFC requirement: RFC4271-5.1.5-3 negative -- a LOCAL_PREF received over an external
// session is discarded rather than used
// (internal/component/bgp/message/rfc7606.go:493-501); the session then rewrites the stored
// UPDATE so the attribute cannot be read later
// (internal/component/bgp/reactor/session_validation.go:127-158).
func TestRFC4271LocalPrefIgnoredOnExternalSession(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00,
		0x40, 0x02, 0x00,
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01,
		0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64,
	}
	res := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionAttributeDiscard, res.Action)
	require.Len(t, res.DiscardEntries, 1)
	assert.Equal(t, uint8(5), res.DiscardEntries[0].Code, "LOCAL_PREF is the discarded attribute")
}
