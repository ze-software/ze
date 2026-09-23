// Design: docs/architecture/rsvpte/mpls-rsvp-te-fast-reroute.md -- protected confirmation carriage.
// RFC: rfc/short/rfc2205.md, rfc/short/rfc4090.md
package rsvpte

import "net/netip"

func encodeResvConfirm(buf []byte, receiver netip.Addr) int {
	encodeObjectHeader(buf, objectHeader{Length: 8, ClassNum: ClassResvConfirm, CType: CTypeIPv4})
	address := receiver.As4()
	copy(buf[4:8], address[:])
	return 8
}

func buildResvConf(msg *ParsedMessage, descriptor *flowDescriptor, origin netip.Addr) []byte {
	encoders := []objEncoder{
		func(b []byte) int { return encodeSessionIPv4(b, msg.Session) },
		func(b []byte) int { return encodeErrorSpec(b, errorSpec{ErrorNode: origin}) },
		func(b []byte) int { return encodeResvConfirm(b, msg.ResvConfirm) },
	}
	for _, raw := range msg.ForwardObjects {
		if raw[2]&0xc0 == 0xc0 {
			encoders = append(encoders, func(b []byte) int { return encodeOpaqueObject(b, raw) })
		}
	}
	encoders = append(encoders,
		func(b []byte) int { return encodeStyle(b, msg.Style) },
		func(b []byte) int { return encodeFlowSpecWithRaw(b, ClassFlowSpec, descriptor.FlowSpec, descriptor.FlowSpecRaw) },
	)
	for _, filter := range descriptor.Filters {
		encoders = append(encoders, func(b []byte) int { return encodeFilterSpec(b, filter.Filter) })
	}
	return encodeMessage(MsgTypeResvConf, defaultIPTTL, encoders,
		len(descriptor.FlowSpecRaw)+12*len(descriptor.Filters)+opaqueObjectsSize(msg.ForwardObjects))
}

// reservationCovers compares token-bucket demand. Service mismatch cannot
// confirm a request; a smaller minimum policed unit is more demanding.
func reservationCovers(have, want FlowSpec) bool {
	return have.Service == want.Service && have.TokenRate >= want.TokenRate &&
		have.TokenBucket >= want.TokenBucket && have.PeakRate >= want.PeakRate &&
		have.MaxPacketSize >= want.MaxPacketSize && have.MinPolicedUnit <= want.MinPolicedUnit
}

// confirmationRoute uses the protected LSP's selected bypass, not whichever
// route happens to reach its merge point. Callers hold no LSP lock.
func (e *engine) confirmationRoute(lsp *LSP, receiver netip.Addr, filter senderTemplateIPv4) (PathRoute, senderTemplateIPv4) {
	route := PathRoute{Destination: receiver, Source: e.cfg().RouterID, NextHop: receiver}
	if lsp == nil {
		return route, filter
	}
	lsp.mu.Lock()
	defer lsp.mu.Unlock()
	if lsp.NextHop.IsValid() {
		route.NextHop = lsp.NextHop
	}
	if lsp.PSB != nil {
		filter = lsp.PSB.SenderTemplate
	}
	if lsp.ProtectionInUse && lsp.Bypass != nil {
		route.NextHop, route.Lookup = lsp.Bypass.TunnelEndpoint, lsp.Bypass.TunnelEndpoint
		route.IfIndex = 0
		route.TableID = bypassTableID(*lsp.Bypass)
		filter.SenderAddr = lsp.RepairSender
	}
	return route, filter
}

func (e *engine) sendResvConf(lsp *LSP, msg *ParsedMessage, descriptor *flowDescriptor) error {
	if !msg.HasResvConfirm || !validUnicast(msg.ResvConfirm) || len(descriptor.Filters) == 0 {
		return nil
	}
	route, filter := e.confirmationRoute(lsp, msg.ResvConfirm, descriptor.Filters[0].Filter)
	outgoing := *descriptor
	outgoing.Filters = []reservationFilter{{Filter: filter}}
	copyMessage := *msg
	copyMessage.ForwardObjects = appendObjectUnion(e.reservationObjects(lsp), msg.ForwardObjects)
	raw := buildResvConf(&copyMessage, &outgoing, e.cfg().RouterID)
	if len(raw) == 0 {
		return errReservationMessage
	}
	return e.transport.SendPath(route, raw)
}

func validUnicast(address netip.Addr) bool {
	return address.Is4() && !address.IsUnspecified() && !address.IsMulticast()
}

func (e *engine) handleResvConf(pkt Packet, msg *ParsedMessage) {
	if !msg.HasResvConfirm || !validUnicast(msg.ResvConfirm) || msg.Header.TTL <= 1 {
		return
	}
	if pkt.Dst.IsValid() && pkt.Dst != msg.ResvConfirm {
		return
	}
	if e.isLocalAddress(msg.ResvConfirm) {
		return
	}
	for _, descriptor := range msg.FlowDescriptors {
		for _, received := range descriptor.Filters {
			key := keyFromFilter(msg.Session, received.Filter)
			lsp, found := e.table.Get(key)
			if !found {
				lsp, _ = e.mergedLSP(key)
			}
			route, filter := e.confirmationRoute(lsp, msg.ResvConfirm, received.Filter)
			outgoing := descriptor
			outgoing.Filters = []reservationFilter{{Filter: filter}}
			raw := buildResvConf(msg, &outgoing, msg.ErrorSpec.ErrorNode)
			if len(raw) == 0 {
				return
			}
			// Rebuild into owned storage; a transport receive buffer is borrowed.
			raw[4] = msg.Header.TTL - 1
			setMessageChecksum(raw)
			if err := e.transport.SendPath(route, raw); err != nil {
				e.log.Warn("rsvp-te: confirmation relay failed", "error", err)
			}
		}
	}
}
