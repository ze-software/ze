// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- reservation control messages.
// RFC: rfc/short/rfc2205.md
package rsvpte

import "net/netip"

// buildReservationControl writes one error descriptor or a teardown filter set.
// Byte 0: common header; then SESSION, HOP, [ERROR_SPEC], opaque objects,
// STYLE, [FLOWSPEC], FILTER_SPEC(s). Labels belong only to RESV, not its errors.
func buildReservationControl(kind uint8, session sessionIPv4, hop rsvpHop, style uint32, descriptor *flowDescriptor, es errorSpec, objects [][]byte) []byte {
	encoders := []objEncoder{
		func(b []byte) int { return encodeSessionIPv4(b, session) },
		func(b []byte) int { return encodeRSVPHop(b, hop) },
	}
	if kind == MsgTypeResvErr {
		encoders = append(encoders, func(b []byte) int { return encodeErrorSpec(b, es) })
	}
	for _, raw := range objects {
		encoders = append(encoders, func(b []byte) int { return encodeOpaqueObject(b, raw) })
	}
	encoders = append(encoders, func(b []byte) int { return encodeStyle(b, style) })
	extra := opaqueObjectsSize(objects)
	if descriptor != nil {
		if kind != MsgTypeResvTear {
			encoders = append(encoders, func(b []byte) int {
				return encodeFlowSpecWithRaw(b, ClassFlowSpec, descriptor.FlowSpec, descriptor.FlowSpecRaw)
			})
			extra += len(descriptor.FlowSpecRaw)
		}
		for _, filter := range descriptor.Filters {
			encoders = append(encoders, func(b []byte) int { return encodeFilterSpec(b, filter.Filter) })
			extra += 12
		}
	}
	return encodeMessage(kind, defaultIPTTL, encoders, extra)
}

func keyFromFilter(session sessionIPv4, filter senderTemplateIPv4) lspKey {
	return lspKey{TunnelEndpoint: session.TunnelEndpoint, TunnelID: session.TunnelID,
		ExtTunnelID: session.ExtTunnelID, SenderAddr: filter.SenderAddr, LSPID: filter.LSPID}
}

func (e *engine) sendResvError(dst netip.Addr, msg *ParsedMessage, descriptor *flowDescriptor, es errorSpec) {
	raw := buildReservationControl(MsgTypeResvErr, msg.Session,
		rsvpHop{NextHop: e.cfg().RouterID, LIH: msg.Hop.LIH}, msg.Style, descriptor, es, msg.ForwardObjects)
	if len(raw) == 0 {
		e.log.Warn("rsvp-te: reservation error exceeds carrier limit")
		return
	}
	if err := e.transport.Send(dst, raw); err != nil {
		e.log.Warn("rsvp-te: reservation error send failed", "dest", dst, "error", err)
	}
}

func (e *engine) rejectReservation(src netip.Addr, msg *ParsedMessage, descriptor *flowDescriptor, code uint8, value uint16, inPlace bool) {
	es := errorSpec{ErrorNode: e.cfg().RouterID, ErrorCode: code, ErrorValue: value}
	// RFC 2205 Section 3.1.8: "the existing reservation must be left in
	// place and the InPlace flag bit must be on in the ERROR_SPEC".
	if inPlace {
		es.Flags = ErrFlagInPlace
	}
	e.sendResvError(src, msg, descriptor, es)
}
