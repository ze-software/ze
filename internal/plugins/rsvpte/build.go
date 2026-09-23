// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RSVP-TE full-message encoders
// RFC: rfc/short/rfc2205.md
// RFC: rfc/short/rfc3209.md
// RFC: rfc/short/rfc4090.md
// Related: wire.go -- per-object Encode*/Decode* primitives this composes
// Related: frr.go -- FAST_REROUTE/SESSION_ATTRIBUTE added to PATH (RFC 4090)
// Related: fsm.go -- PathStateBlock / ResvStateBlock provide the object values
//
// wire.go provides per-object encoders and a whole-message DECODER, but no
// whole-message ENCODER. build.go composes the object encoders in the object
// order RFC 2205/3209 prescribe, patches the common-header Length, and fills
// the RFC 2205 Section 3.1 one's-complement checksum so peers (e.g. FRR) accept
// the message.
package rsvpte

import (
	"encoding/binary"
	"net/netip"
	"time"
)

// maxRSVPMessage holds all supported objects at their decode limits: the
// fixed PATH objects (100), ERO header and 64 IPv6 hops (1284), SESSION_ATTRIBUTE
// (276), FAST_REROUTE (24), and RRO header and 32 IPv6 entries (644).
// This is a codec bound; transport errors, including path-MTU errors, propagate.
const maxRSVPMessage = 100 + objHdrLen + maxExplicitRouteHops*20 +
	maxSessionAttrLen + objHdrLen + frrBodyLen + objHdrLen + maxRecordRouteHops*20

// maxRSVPPacket leaves room for the IPv4 header and Router Alert option and
// rounds down to an RSVP object boundary.
const maxRSVPPacket = 65508

// objEncoder writes one object at buf[0:] and returns the byte count, or -1
// when its caller-owned buffer cannot hold the complete object.
type objEncoder func(buf []byte) int

// encodeMessage writes the common header followed by each object in order,
// patches the header Length, and fills the checksum. RFC 2205 Section 3.1: the
// checksum is the one's-complement sum over the whole message with the checksum
// field taken as zero.
// Extra sizes reserve space for retained objects. Callers MUST reject a nil
// result, which means the complete message could not fit the native carrier.
func encodeMessage(msgType, ttl uint8, encoders []objEncoder, extra ...int) []byte {
	size := maxRSVPMessage
	for _, n := range extra {
		if n < 0 || n > maxRSVPPacket {
			return nil
		}
		size = min(size+n, maxRSVPPacket)
	}
	buf := make([]byte, size)
	off := rsvpHdrLen
	for _, enc := range encoders {
		if enc == nil {
			continue
		}
		n := enc(buf[off:])
		if n < 0 || n > len(buf)-off {
			return nil
		}
		off += n
	}
	encodeHeader(buf, Header{Version: rsvpVersion, MsgType: msgType, TTL: ttl, Length: uint16(off)})
	out := buf[:off]
	// Checksum is computed with the checksum field (bytes 2:4) zeroed; EncodeHeader
	// already wrote 0 there.
	setMessageChecksum(out)
	return out
}

func setMessageChecksum(raw []byte) {
	raw[2], raw[3] = 0, 0
	binary.BigEndian.PutUint16(raw[2:4], internetChecksum(raw))
}

// internetChecksum computes the standard 16-bit one's-complement checksum
// (RFC 1071) used by RSVP (RFC 2205 Section 3.1).
func internetChecksum(b []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(b); i += 2 {
		sum += uint32(b[i])<<8 | uint32(b[i+1])
	}
	if len(b)%2 == 1 {
		sum += uint32(b[len(b)-1]) << 8
	}
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}

// refreshMillis converts a refresh period to the milliseconds TIME_VALUES carries
// (RFC 2205 Section A.4), clamped to the default when unset.
func refreshMillis(d time.Duration) uint32 {
	if d <= 0 {
		d = DefaultRefreshPeriod
	}
	return uint32(d / time.Millisecond)
}

// buildPath encodes a PATH message from path state. RFC 3209 Section 2: object
// order is SESSION, RSVP_HOP, TIME_VALUES, [ERO], LABEL_REQUEST, SENDER_TEMPLATE,
// SENDER_TSPEC, [ADSPEC], [RRO]. hop is this node's address (the downstream
// neighbor's PHOP) and ttl is the IP TTL echoed in the common header.
func buildPath(psb *pathStateBlock, hop netip.Addr, ttl uint8) []byte {
	encoders := []objEncoder{
		func(b []byte) int { return encodeSessionIPv4(b, psb.Session) },
		func(b []byte) int { return encodeRSVPHop(b, rsvpHop{NextHop: hop}) },
		func(b []byte) int {
			return encodeTimeValues(b, timeValues{RefreshPeriod: refreshMillis(psb.RefreshPeriod)})
		},
	}
	if len(psb.ERO) > 0 {
		encoders = append(encoders, func(b []byte) int { return encodeERO(b, psb.ERO) })
	}
	encoders = append(encoders, func(b []byte) int { return encodeLabelRequest(b, psb.LabelRequest) })
	// RFC 4090: when local protection is requested, the head-end adds a
	// SESSION_ATTRIBUTE (protection-desired flags, RFC 4090 Section 4.3) and a
	// FAST_REROUTE object (Section 4.1) so transit PLRs arm a backup. RFC 3209
	// Section 4.7 places SESSION_ATTRIBUTE after LABEL_REQUEST; FAST_REROUTE
	// follows it. A transit relays the SESSION_ATTRIBUTE bytes it received
	// (RFC 3209 Section 4.7.4: "SHALL forward the object unmodified"), whether
	// or not they request protection, and the FAST_REROUTE it received,
	// unchanged, and inserts none of either when the head-end sent none (RFC
	// 4090 Section 4.1).
	if len(psb.SessionAttr) > 0 {
		encoders = append(encoders, func(b []byte) int { return encodeOpaqueObject(b, psb.SessionAttr) })
	}
	if psb.Protection != nil {
		pr := psb.Protection
		if len(psb.SessionAttr) == 0 && !pr.Transit {
			encoders = append(encoders, func(b []byte) int { return encodeSessionAttr(b, pr.sessionAttr()) })
		}
		if fr, ok := pr.fastRerouteObject(); ok {
			encoders = append(encoders, func(b []byte) int { return encodeFastReroute(b, fr) })
		}
	}
	encoders = append(encoders,
		func(b []byte) int { return encodeSenderTemplate(b, psb.SenderTemplate) },
		func(b []byte) int { return encodeFlowSpecWithRaw(b, ClassSenderTSpec, psb.SenderTSpec, psb.SenderTSpecRaw) },
	)
	if len(psb.Adspec) > 0 {
		encoders = append(encoders, func(b []byte) int { return encodeOpaqueObject(b, psb.Adspec) })
	}
	for _, raw := range psb.ForwardObjects {
		encoders = append(encoders, func(b []byte) int { return encodeOpaqueObject(b, raw) })
	}
	if psb.RecordRoute {
		encoders = append(encoders, func(b []byte) int { return encodeRRO(b, psb.RRO) })
	}
	return encodeMessage(MsgTypePath, ttl, encoders,
		len(psb.Adspec)+len(psb.SenderTSpecRaw)+opaqueObjectsSize(psb.ForwardObjects))
}

// buildResv encodes a RESV message from reservation state. RFC 3209 Section
// 3.2: object order is SESSION, RSVP_HOP, TIME_VALUES, STYLE, FLOWSPEC,
// FILTER_SPEC, LABEL, [RRO]. The FILTER_SPEC identifies the sender being
// reserved for, and RFC 3209 Section 3 binds the two objects after it to it:
// "The LABEL and RECORD_ROUTE objects, are sender specific. In Resv messages
// they MUST appear after the associated FILTER_SPEC and prior to any subsequent
// FILTER_SPEC."
// A RESV travels one hop upstream toward the PHOP and is not per-hop TTL-stepped,
// so it always uses defaultIPTTL (unlike buildPath, which decrements at transit).
func buildResv(rsb *resvStateBlock, filter senderTemplateIPv4, refresh time.Duration, hop netip.Addr) []byte {
	style := rsb.Style
	if style == 0 {
		style = StyleFixedFilter
	}
	encoders := []objEncoder{
		func(b []byte) int { return encodeSessionIPv4(b, rsb.Session) },
		func(b []byte) int { return encodeRSVPHop(b, rsvpHop{NextHop: hop, LIH: rsb.Hop.LIH}) },
		func(b []byte) int { return encodeTimeValues(b, timeValues{RefreshPeriod: refreshMillis(refresh)}) },
		func(b []byte) int {
			if !rsb.ResvConfirm.IsValid() {
				return 0
			}
			return encodeResvConfirm(b, rsb.ResvConfirm)
		},
	}
	for _, raw := range rsb.ForwardObjects {
		encoders = append(encoders, func(b []byte) int { return encodeOpaqueObject(b, raw) })
	}
	encoders = append(encoders,
		func(b []byte) int { return encodeStyle(b, style) },
		func(b []byte) int { return encodeFlowSpecWithRaw(b, ClassFlowSpec, rsb.FlowSpec, rsb.FlowSpecRaw) },
		func(b []byte) int { return encodeFilterSpec(b, filter) },
		func(b []byte) int { return encodeLabelObject(b, rsb.Label) },
	)
	if len(rsb.RRO) > 0 {
		encoders = append(encoders, func(b []byte) int { return encodeRRO(b, rsb.RRO) })
	}
	return encodeMessage(MsgTypeResv, defaultIPTTL, encoders,
		len(rsb.FlowSpecRaw)+opaqueObjectsSize(rsb.ForwardObjects))
}

// buildPathTear encodes a PathTear message. RFC 2205 Section 3.1.5: it carries
// SESSION, RSVP_HOP and the sender descriptor so the path state is removed
// hop-by-hop downstream.
func buildPathTear(psb *pathStateBlock, hop netip.Addr) []byte {
	encoders := []objEncoder{
		func(b []byte) int { return encodeSessionIPv4(b, psb.Session) },
		func(b []byte) int { return encodeRSVPHop(b, rsvpHop{NextHop: hop}) },
		func(b []byte) int { return encodeSenderTemplate(b, psb.SenderTemplate) },
		func(b []byte) int { return encodeFlowSpecWithRaw(b, ClassSenderTSpec, psb.SenderTSpec, psb.SenderTSpecRaw) },
	}
	for _, raw := range psb.ForwardObjects {
		encoders = append(encoders, func(b []byte) int { return encodeOpaqueObject(b, raw) })
	}
	return encodeMessage(MsgTypePathTear, defaultIPTTL, encoders,
		len(psb.SenderTSpecRaw)+opaqueObjectsSize(psb.ForwardObjects))
}

// buildPathErr encodes a PathErr message reporting an error toward the head-end.
// RFC 2205 Section 3.1.3: SESSION, ERROR_SPEC, then the sender descriptor. Like
// buildResv/buildPathTear it uses defaultIPTTL: a PathErr is addressed to the
// previous hop, not per-hop TTL-stepped.
func buildPathErr(session sessionIPv4, sender senderTemplateIPv4, tspec FlowSpec, es errorSpec, hop netip.Addr, forward ...[]byte) []byte {
	encoders := []objEncoder{
		func(b []byte) int { return encodeSessionIPv4(b, session) },
		func(b []byte) int { return encodeErrorSpec(b, es) },
		func(b []byte) int { return encodeSenderTemplate(b, sender) },
		func(b []byte) int { return encodeFlowSpec(b, ClassSenderTSpec, tspec) },
	}
	for _, raw := range forward {
		encoders = append(encoders, func(b []byte) int { return encodeOpaqueObject(b, raw) })
	}
	return encodeMessage(MsgTypePathErr, defaultIPTTL, encoders, opaqueObjectsSize(forward))
}

// opaqueObjectsSize bounds capacity for the retained object sequence.
func opaqueObjectsSize(objects [][]byte) int {
	size := 0
	for _, raw := range objects {
		if len(raw) > maxRSVPPacket-size {
			return maxRSVPPacket + 1
		}
		size += len(raw)
	}
	return size
}

// encodeOpaqueObject writes one complete retained object or fails before copying.
func encodeOpaqueObject(buf, raw []byte) int {
	header, err := decodeObjectHeader(raw)
	if err != nil || int(header.Length) != len(raw) || len(raw) > len(buf) {
		return -1
	}
	return copy(buf, raw)
}

// encodeFlowSpecWithRaw preserves an incoming TSpec's optional fragments. A
// returned FLOWSPEC retains its encoding and receives the negotiated M value.
func encodeFlowSpecWithRaw(buf []byte, class uint8, fs FlowSpec, raw []byte) int {
	if len(raw) == 0 {
		return encodeFlowSpec(buf, class, fs)
	}
	if len(raw) < objHdrLen || raw[2] != class || raw[3] != 2 {
		return -1
	}
	decoded, err := decodeFlowSpec(raw[objHdrLen:])
	if err != nil {
		return -1
	}
	n := encodeOpaqueObject(buf, raw)
	if n < 0 || class == ClassSenderTSpec {
		return n
	}
	switch decoded.Service {
	case serviceControlledLoad:
		binary.BigEndian.PutUint32(buf[32:36], fs.MaxPacketSize)
	case serviceNull:
		binary.BigEndian.PutUint32(buf[16:20], fs.MaxPacketSize)
	default:
		return -1
	}
	return n
}

// RFC 2205 Section A.5 / RFC 3209 Section 4.3.5: error codes/values used in
// ERROR_SPEC. AdmissionControlFailure rejects a reservation for insufficient
// bandwidth; RoutingProblem (with BadEROObject) reports an ERO that cannot be
// satisfied at a transit node.
const (
	ErrCodeAdmissionControlFailure uint8  = 1
	// RFC 2205 Appendix B: "Error Code = 02: Policy Control failure".
	ErrCodePolicyControlFailure    uint8  = 2
	ErrValueRequestedBandwidth     uint16 = 2
	ErrCodeRoutingProblem          uint8  = 24
	// RFC 3209 Section 4.6: Routing Problem error values 1 through 4.
	ErrValueBadEROObject           uint16 = 1
	ErrValueBadStrictNode          uint16 = 2
	ErrValueBadLooseNode           uint16 = 3
	ErrValueBadInitialSubobject    uint16 = 4
	// ErrValueNoRouteAvailable reports that the path toward the destination is
	// gone (e.g. a link on the LSP failed) -- RFC 3209 Section 4.3.5 value 5.
	ErrValueNoRouteAvailable uint16 = 5
	// RFC 4090 Section 6.5: on local repair the PLR notifies the head-end with a
	// PathErr carrying Error Code 25 ("Notify") and Error Value sub-code 3
	// ("Tunnel locally repaired"). The protected LSP is NOT torn down.
	ErrCodeNotify                 uint8  = 25
	ErrValueTunnelLocallyRepaired uint16 = 3
	// RFC 2205 Appendix B, Error Code 13: "Unknown object class". Its Error Value
	// is the 16-bit (Class-Num, C-Type) of the object that was not understood, and
	// RFC 2205 sends it only when the message is rejected, as the high-order bits
	// of the Class-Num decide. RFC 4090 Section 4.2 requires this PathErr from an
	// LSR that does not support the DETOUR object.
	ErrCodeUnknownObjectClass uint8 = 13
	ErrCodeUnknownCType uint8 = 14
	ErrCodeNoPath uint8 = 3
	ErrCodeNoSender uint8 = 4
	ErrCodeConflictingStyle uint8 = 5
	ErrCodeUnknownStyle uint8 = 6
	ErrCodeTrafficControlSystem uint8 = 22
	ErrFlagInPlace uint8 = 1
)
