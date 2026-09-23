// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- what a transit relays: the RRO it
// drops when it no longer fits (RFC 3209 Section 4.4.3), the SESSION_ATTRIBUTE it forwards
// unmodified (Section 4.7.4), and the LABEL it never emits without a LABEL_REQUEST
// (Section 4.2.4).
package rsvpte

import (
	"bytes"
	"net/netip"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// transitPath signals the protected-transit PSB through a plrEngine transit
// (router 10.0.0.2, NHOP 10.0.0.3) with no protection and returns the engine
// and its transport.
func transitPath(t *testing.T) (*engine, *fakeTransport) {
	t.Helper()
	e, ft, _ := plrEngine(t)
	ingress := netip.MustParseAddr("10.0.0.1")
	psb := protectedTransitPSB(nil)
	e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})
	_, _, ok := ft.lastByType(MsgTypePath)
	require.True(t, ok, "transit relays the PATH")
	return e, ft
}

// resvWithRRO returns a RESV from the merge point 10.0.0.3 whose RRO holds
// hops entries.
func resvWithRRO(hops int) (netip.Addr, []byte) {
	mp := netip.MustParseAddr("10.0.0.3")
	psb := protectedTransitPSB(nil)
	rro := make([]rroEntry, hops)
	for i := range rro {
		rro[i] = rroEntry{Type: RROSubIPv4, Address: mp}
	}
	rsb := &resvStateBlock{Session: psb.Session, Label: labelObject{Label: 18000}, Style: StyleSharedExplicit, RRO: rro}
	return mp, buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, mp)
}

// RFC requirement: RFC3209-4.4.3-3 positive — a transit whose own subobject would grow the RRO past what one message holds relays the RESV with no RRO at all and still brings the LSP up.
func TestRFC3209RRODroppedWhenTooBig(t *testing.T) {
	e, ft := transitPath(t)
	mp, resv := resvWithRRO(maxRecordRouteHops)
	e.handlePacket(Packet{Src: mp, Payload: resv})

	relayed, _, ok := ft.lastByType(MsgTypeResv)
	require.True(t, ok, "transit relays the RESV upstream")
	require.Len(t, relayed.FlowDescriptors, 1)
	require.Len(t, relayed.FlowDescriptors[0].Filters, 1)
	assert.False(t, relayed.FlowDescriptors[0].Filters[0].HasRRO, "the RRO is dropped from the message")
	lsp, ok := e.table.Get(protectedKey())
	require.True(t, ok)
	lsp.mu.Lock()
	defer lsp.mu.Unlock()
	assert.Equal(t, LSPStateUp, lsp.State, "message processing continues as normal")
	assert.True(t, lsp.RSB.RRODropped, "the drop is kept on the reservation state")
}

// RFC requirement: RFC3209-4.4.3-3 negative — a transit whose own subobject still fits the RRO does not drop it: the relayed RESV carries the route with this node first.
func TestRFC3209RROKeptWhenItFits(t *testing.T) {
	e, ft := transitPath(t)
	mp, resv := resvWithRRO(maxRecordRouteHops - 1)
	e.handlePacket(Packet{Src: mp, Payload: resv})

	relayed, _, ok := ft.lastByType(MsgTypeResv)
	require.True(t, ok)
	require.Len(t, relayed.FlowDescriptors, 1)
	require.Len(t, relayed.FlowDescriptors[0].Filters, 1)
	filter := relayed.FlowDescriptors[0].Filters[0]
	require.True(t, filter.HasRRO, "a route that fits is relayed")
	require.Len(t, filter.RRO, maxRecordRouteHops)
	assert.Equal(t, netip.MustParseAddr("10.0.0.2"), filter.RRO[0].Address)
}

// TestRFC3209RRODropSticksAcrossRefresh preserves Ze's local overflow policy.
func TestRFC3209RRODropSticksAcrossRefresh(t *testing.T) {
	e, ft := transitPath(t)
	mp, big := resvWithRRO(maxRecordRouteHops)
	e.handlePacket(Packet{Src: mp, Payload: big})
	_, short := resvWithRRO(1)
	e.handlePacket(Packet{Src: mp, Payload: short})

	relayed, _, ok := ft.lastByType(MsgTypeResv)
	require.True(t, ok)
	assert.Equal(t, 2, ft.countByType(MsgTypeResv), "both RESVs were relayed")
	require.Len(t, relayed.FlowDescriptors, 1)
	require.Len(t, relayed.FlowDescriptors[0].Filters, 1)
	assert.False(t, relayed.FlowDescriptors[0].Filters[0].HasRRO, "subsequent Resv messages carry no RRO")
}

// TestRFC3209RRORefreshRecordsWhenNeverDropped keeps recording below the bound.
func TestRFC3209RRORefreshRecordsWhenNeverDropped(t *testing.T) {
	e, ft := transitPath(t)
	mp, first := resvWithRRO(1)
	e.handlePacket(Packet{Src: mp, Payload: first})
	_, second := resvWithRRO(2)
	e.handlePacket(Packet{Src: mp, Payload: second})

	relayed, _, ok := ft.lastByType(MsgTypeResv)
	require.True(t, ok)
	assert.Equal(t, 2, ft.countByType(MsgTypeResv))
	require.Len(t, relayed.FlowDescriptors, 1)
	require.Len(t, relayed.FlowDescriptors[0].Filters, 1)
	filter := relayed.FlowDescriptors[0].Filters[0]
	require.True(t, filter.HasRRO)
	assert.Len(t, filter.RRO, 3, "this node ahead of the two downstream hops")
}

// objectBytes returns the first object of classNum in an encoded message, header
// included, and false when the message carries none.
func objectBytes(raw []byte, classNum uint8) ([]byte, bool) {
	off := rsvpHdrLen
	for off+objHdrLen <= len(raw) {
		hdr, err := decodeObjectHeader(raw[off:])
		if err != nil || hdr.Length < objHdrLen || off+int(hdr.Length) > len(raw) {
			return nil, false
		}
		if hdr.ClassNum == classNum {
			return raw[off : off+int(hdr.Length)], true
		}
		off += int(hdr.Length)
	}
	return nil, false
}

// lastSentPayload returns the raw bytes of the last message of msgType the
// transport sent.
func (f *fakeTransport) lastSentPayload(msgType uint8) ([]byte, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, sent := range slices.Backward(f.sent) {
		msg, err := DecodeMessage(sent.payload)
		if err == nil && msg.Header.MsgType == msgType {
			return sent.payload, true
		}
	}
	return nil, false
}

// sessionAttrRA encodes a C-Type 1 (LSP_TUNNEL_RA) SESSION_ATTRIBUTE with three
// resource-affinity masks and a flags byte requesting no protection, the shape
// a peer Ze does not fully decode would send.
func sessionAttrRA(buf []byte) int {
	name := "affine"
	body := 12 + 4 + 8
	encodeObjectHeader(buf, objectHeader{Length: uint16(objHdrLen + body), ClassNum: ClassSessionAttr, CType: CTypeSessionAttrRA})
	masks := []byte{0, 0, 0, 1, 0, 0, 0, 2, 0, 0, 0, 4}
	copy(buf[4:16], masks)
	buf[16] = 3 // setup priority
	buf[17] = 5 // hold priority
	buf[18] = 0 // no protection desired
	buf[19] = uint8(len(name))
	for i := 20; i < 4+body; i++ {
		buf[i] = 0
	}
	copy(buf[20:], name)
	return objHdrLen + body
}

// RFC requirement: RFC3209-4.7.4-3 positive — a transit relays a received C-Type 1 SESSION_ATTRIBUTE that requests no protection byte for byte, affinity masks and all.
func TestRFC3209SessionAttributeRelayedUnmodified(t *testing.T) {
	e, ft, _ := plrEngine(t)
	ingress := netip.MustParseAddr("10.0.0.1")
	psb := protectedTransitPSB(nil)
	encoders := []objEncoder{
		func(b []byte) int { return encodeSessionIPv4(b, psb.Session) },
		func(b []byte) int { return encodeRSVPHop(b, rsvpHop{NextHop: ingress}) },
		func(b []byte) int {
			return encodeTimeValues(b, timeValues{RefreshPeriod: refreshMillis(DefaultRefreshPeriod)})
		},
		func(b []byte) int { return encodeERO(b, psb.ERO) },
		func(b []byte) int { return encodeLabelRequest(b, psb.LabelRequest) },
		sessionAttrRA,
		func(b []byte) int { return encodeSenderTemplate(b, psb.SenderTemplate) },
		func(b []byte) int { return encodeFlowSpec(b, ClassSenderTSpec, psb.SenderTSpec) },
	}
	received := encodeMessage(MsgTypePath, 64, encoders)
	want, ok := objectBytes(received, ClassSessionAttr)
	require.True(t, ok)

	e.handlePacket(Packet{Src: ingress, Payload: received})

	relayed, ok := ft.lastSentPayload(MsgTypePath)
	require.True(t, ok, "transit relays the PATH")
	got, ok := objectBytes(relayed, ClassSessionAttr)
	require.True(t, ok, "the relayed PATH carries the SESSION_ATTRIBUTE")
	assert.True(t, bytes.Equal(want, got), "relayed object differs: want %x got %x", want, got)
}

// RFC requirement: RFC3209-4.7.4-3 negative — a transit never inserts a SESSION_ATTRIBUTE the head-end did not send: a PATH without one is relayed without one.
func TestRFC3209SessionAttributeNotInserted(t *testing.T) {
	_, ft := transitPath(t)
	require.Equal(t, 1, ft.countByType(MsgTypePath), "exactly one PATH is relayed")
	relayed, ok := ft.lastSentPayload(MsgTypePath)
	require.True(t, ok)
	_, present := objectBytes(relayed, ClassSessionAttr)
	assert.False(t, present, "no SESSION_ATTRIBUTE appears in the relayed PATH")
}

// RFC requirement: RFC3209-4.2.4-2 positive — the egress answers a PATH that carries a LABEL_REQUEST with a RESV that carries a LABEL.
func TestRFC3209LabelFollowsLabelRequest(t *testing.T) {
	e, ft, _ := testEngine(t, "10.0.0.9", nil)
	ingress := netip.MustParseAddr("10.0.0.1")
	psb := protectedTransitPSB(nil)
	psb.ERO = psb.ERO[len(psb.ERO)-1:]
	e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})

	resv, _, ok := ft.lastByType(MsgTypeResv)
	require.True(t, ok, "egress answers a RESV")
	require.Len(t, resv.FlowDescriptors, 1)
	require.Len(t, resv.FlowDescriptors[0].Filters, 1)
	assert.True(t, resv.FlowDescriptors[0].Filters[0].HasLabel, "the RESV carries a LABEL")
	assert.NotZero(t, resv.FlowDescriptors[0].Filters[0].Label.Label)
}

// RFC requirement: RFC3209-4.2.4-2 negative — a PATH without a LABEL_REQUEST gets no RESV from the egress, so no LABEL is ever included for that session and PHOP.
// RFC requirement: RFC3209-4.2-1 negative — a PATH without a LABEL_REQUEST is refused: no reservation state and no RESV.
func TestRFC3209NoLabelWithoutLabelRequest(t *testing.T) {
	e, ft, _ := testEngine(t, "10.0.0.9", nil)
	ingress := netip.MustParseAddr("10.0.0.1")
	psb := protectedTransitPSB(nil)
	psb.ERO = psb.ERO[len(psb.ERO)-1:]
	encoders := []objEncoder{
		func(b []byte) int { return encodeSessionIPv4(b, psb.Session) },
		func(b []byte) int { return encodeRSVPHop(b, rsvpHop{NextHop: ingress}) },
		func(b []byte) int {
			return encodeTimeValues(b, timeValues{RefreshPeriod: refreshMillis(DefaultRefreshPeriod)})
		},
		func(b []byte) int { return encodeERO(b, psb.ERO) },
		func(b []byte) int { return encodeSenderTemplate(b, psb.SenderTemplate) },
		func(b []byte) int { return encodeFlowSpec(b, ClassSenderTSpec, psb.SenderTSpec) },
	}
	e.handlePacket(Packet{Src: ingress, Payload: encodeMessage(MsgTypePath, 64, encoders)})

	_, _, sent := ft.lastByType(MsgTypeResv)
	assert.False(t, sent, "no RESV, so no LABEL, for a PATH without LABEL_REQUEST")
	_, held := e.table.Get(keyFromMessage(mustDecode(t, buildPath(psb, ingress, 64))))
	assert.False(t, held, "no reservation state is installed")
}

// mustDecode parses an encoded message or fails the test.
func mustDecode(t *testing.T, raw []byte) *ParsedMessage {
	t.Helper()
	msg, err := DecodeMessage(raw)
	require.NoError(t, err)
	return msg
}

// RFC requirement: RFC3209-3-1 positive — in a Ze RESV the LABEL object follows the FILTER_SPEC (Class-Num 10) it belongs to, with no other FILTER_SPEC between them.
func TestRFC3209LabelFollowsFilterSpec(t *testing.T) {
	psb := protectedTransitPSB(nil)
	rsb := &resvStateBlock{Session: psb.Session, Label: labelObject{Label: 18000}, Style: StyleSharedExplicit}
	raw := buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, netip.MustParseAddr("10.0.0.3"))

	var order []uint8
	off := rsvpHdrLen
	for off < len(raw) {
		hdr, err := decodeObjectHeader(raw[off:])
		require.NoError(t, err)
		order = append(order, hdr.ClassNum)
		off += int(hdr.Length)
	}
	filter, label := -1, -1
	for i, class := range order {
		switch class {
		case ClassFilterSpec:
			require.Equal(t, -1, filter, "one FILTER_SPEC per sender")
			filter = i
		case ClassLabel:
			label = i
		}
	}
	require.NotEqual(t, -1, filter, "the RESV carries a FILTER_SPEC")
	require.NotEqual(t, -1, label, "the RESV carries a LABEL")
	assert.Equal(t, filter+1, label, "LABEL immediately follows its FILTER_SPEC: %v", order)
}

// TestRFC3209PathRROWithdrawal stops route recording at both transit and egress
// after a PATH refresh withdraws the request, including cached RESV refreshes.
// RFC requirement: RFC3209-4.4.3-4 positive -- a PATH without an RRO removes the RRO from subsequent relayed and refreshed RESV messages.
func TestRFC3209PathRROWithdrawal(t *testing.T) {
	for _, router := range []string{"10.0.0.2", "10.0.0.9"} {
		t.Run(router, func(t *testing.T) {
			e, ft, _ := testEngine(t, router, nil)
			ingress := netip.MustParseAddr("10.0.0.1")
			psb := protectedTransitPSB(nil)
			if router == "10.0.0.9" {
				psb.ERO = psb.ERO[len(psb.ERO)-1:]
			}
			e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})
			mp, resv := resvWithRRO(1)
			if router == "10.0.0.2" {
				e.handlePacket(Packet{Src: mp, Payload: resv})
			}
			before, _, ok := ft.lastByType(MsgTypeResv)
			require.True(t, ok)
			require.Len(t, before.FlowDescriptors, 1)
			require.Len(t, before.FlowDescriptors[0].Filters, 1)
			require.True(t, before.FlowDescriptors[0].Filters[0].HasRRO, "route recording was requested")

			psb.RRO = nil
			psb.RecordRoute = false
			e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})
			withdrawn, _, ok := ft.lastByType(MsgTypeResv)
			require.True(t, ok)
			require.Len(t, withdrawn.FlowDescriptors, 1)
			require.Len(t, withdrawn.FlowDescriptors[0].Filters, 1)
			assert.False(t, withdrawn.FlowDescriptors[0].Filters[0].HasRRO, "the PATH change immediately withdraws the cached route")
			if router == "10.0.0.2" {
				e.handlePacket(Packet{Src: mp, Payload: resv})
			}
			lsp, ok := e.table.Get(protectedKey())
			require.True(t, ok)
			require.NoError(t, e.sendResv(lsp))
			after, _, ok := ft.lastByType(MsgTypeResv)
			require.True(t, ok)
			require.Len(t, after.FlowDescriptors, 1)
			require.Len(t, after.FlowDescriptors[0].Filters, 1)
			assert.False(t, after.FlowDescriptors[0].Filters[0].HasRRO, "withdrawn route recording stays absent on refresh")
		})
	}
}

// TestRFC3209PathRROForwarded checks the PATH direction of route recording.
// RFC requirement: RFC3209-4.4.3-4 negative -- while the PATH still requests route recording, transit forwards its accumulated RRO and includes an RRO in RESV refreshes.
func TestRFC3209PathRROForwarded(t *testing.T) {
	e, ft := transitPath(t)
	path, _, ok := ft.lastByType(MsgTypePath)
	require.True(t, ok)
	require.True(t, path.HasRRO)
	require.Len(t, path.RRO, 2)
	assert.Equal(t, netip.MustParseAddr("10.0.0.2"), path.RRO[0].Address)
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), path.RRO[1].Address)
	mp, resv := resvWithRRO(1)
	e.handlePacket(Packet{Src: mp, Payload: resv})
	lsp, ok := e.table.Get(protectedKey())
	require.True(t, ok)
	require.NoError(t, e.sendResv(lsp))
	refresh, _, ok := ft.lastByType(MsgTypeResv)
	require.True(t, ok)
	require.Len(t, refresh.FlowDescriptors, 1)
	require.Len(t, refresh.FlowDescriptors[0].Filters, 1)
	require.True(t, refresh.FlowDescriptors[0].Filters[0].HasRRO)
	assert.Len(t, refresh.FlowDescriptors[0].Filters[0].RRO, 2)
}

// TestRROLabelCountsTowardLimit covers the extra subobject added by protection.
// A route that fits without a label must be dropped when the label exceeds the cap.
func TestRROLabelCountsTowardLimit(t *testing.T) {
	e, ft, _ := plrEngine(t)
	ingress := netip.MustParseAddr("10.0.0.1")
	psb := protectedTransitPSB(&protectionRequest{Facility: true})
	e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})
	mp, resv := resvWithRRO(maxRecordRouteHops - 1)
	e.handlePacket(Packet{Src: mp, Payload: resv})
	relayed, _, ok := ft.lastByType(MsgTypeResv)
	require.True(t, ok)
	require.Len(t, relayed.FlowDescriptors, 1)
	require.Len(t, relayed.FlowDescriptors[0].Filters, 1)
	assert.False(t, relayed.FlowDescriptors[0].Filters[0].HasRRO, "the address and label must fit together")
	lsp, ok := e.table.Get(protectedKey())
	require.True(t, ok)
	assert.Equal(t, LSPStateUp, lsp.State)
}

// TestSessionAttributeOversizeRejected sends an object with more padding than
// its one-octet Name Length can describe, before the relay copies its raw bytes.
func TestSessionAttributeOversizeRejected(t *testing.T) {
	e, ft, _ := plrEngine(t)
	ingress := netip.MustParseAddr("10.0.0.1")
	psb := protectedTransitPSB(nil)
	received := buildPath(psb, ingress, 64)
	attr := make([]byte, maxRSVPMessage)
	encodeObjectHeader(attr, objectHeader{
		Length: uint16(len(attr)), ClassNum: ClassSessionAttr, CType: CTypeSessionAttr,
	})
	received = append(received, attr...)
	received[6] = byte(len(received) >> 8)
	received[7] = byte(len(received))
	setMessageChecksum(received)
	e.handlePacket(Packet{Src: ingress, Payload: received})
	assert.Zero(t, sentCount(ft), "an oversized raw object must not reach the relay encoder")
	assert.Zero(t, e.table.Len(), "malformed PATH installs no state")
}

// TestPathCarriesMaximumRelayObjects combines independently valid object limits.
// The raw C-Type 1 object must not consume space reserved for later PATH objects.
func TestPathCarriesMaximumRelayObjects(t *testing.T) {
	psb := protectedTransitPSB(&protectionRequest{Facility: true})
	psb.ERO = make([]eroHop, maxExplicitRouteHops)
	for i := range psb.ERO {
		psb.ERO[i] = eroHop{Address: netip.MustParsePrefix("2001:db8::1/128")}
	}
	psb.RRO = make([]rroEntry, maxRecordRouteHops)
	for i := range psb.RRO {
		psb.RRO[i] = rroEntry{Type: RROSubIPv6, Address: netip.MustParseAddr("2001:db8::2")}
	}
	psb.SessionAttr = make([]byte, maxSessionAttrLen)
	encodeObjectHeader(psb.SessionAttr, objectHeader{
		Length: maxSessionAttrLen, ClassNum: ClassSessionAttr, CType: CTypeSessionAttrRA,
	})
	psb.SessionAttr[19] = 255
	for i := 20; i < 275; i++ {
		psb.SessionAttr[i] = 'a'
	}
	raw := buildPath(psb, netip.MustParseAddr("10.0.0.1"), 64)
	msg, err := DecodeMessage(raw)
	require.NoError(t, err)
	assert.Len(t, msg.ERO, maxExplicitRouteHops)
	assert.Len(t, msg.RRO, maxRecordRouteHops)
	assert.Equal(t, psb.SenderTemplate, msg.SenderTemplate)
	wantTSpec := psb.SenderTSpec
	wantTSpec.Service = serviceGeneral
	assert.Equal(t, wantTSpec, msg.SenderTSpec)
	got, ok := objectBytes(raw, ClassSessionAttr)
	require.True(t, ok)
	assert.Equal(t, psb.SessionAttr, got)
	assert.Zero(t, internetChecksum(raw), "the complete PATH retains its checksum")
}

// TestSessionAttributeFirstObjectRelayed keeps the first SESSION_ATTRIBUTE when
// a peer sends a second one with different flags, as RFC 3209 Section 4.7.4 says.
func TestSessionAttributeFirstObjectRelayed(t *testing.T) {
	e, ft, _ := plrEngine(t)
	ingress := netip.MustParseAddr("10.0.0.1")
	psb := protectedTransitPSB(nil)
	first := make([]byte, 64)
	first = first[:sessionAttrRA(first)]
	psb.SessionAttr = first
	raw := buildPath(psb, ingress, 64)
	var second [8]byte
	encodeSessionAttr(second[:], sessionAttribute{Flags: SessAttrLocalProtection})
	raw = append(raw, second[:]...)
	raw[6] = byte(len(raw) >> 8)
	raw[7] = byte(len(raw))
	setMessageChecksum(raw)
	e.handlePacket(Packet{Src: ingress, Payload: raw})
	relayed, ok := ft.lastSentPayload(MsgTypePath)
	require.True(t, ok)
	got, ok := objectBytes(relayed, ClassSessionAttr)
	require.True(t, ok)
	assert.Equal(t, first, got, "only the first SESSION_ATTRIBUTE is meaningful")
}
