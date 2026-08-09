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

// maxRSVPMessage bounds a single encoded message. LSP signaling messages carry
// a handful of small objects plus a bounded ERO/RRO (<= MaxLabelStack hops), so
// they sit far below a single non-fragmented IP datagram.
const maxRSVPMessage = 1500

// objEncoder writes one object at buf[0:] and returns the bytes written, exactly
// matching the wire.go Encode* contract.
type objEncoder func(buf []byte) int

// encodeMessage writes the common header followed by each object in order,
// patches the header Length, and fills the checksum. RFC 2205 Section 3.1: the
// checksum is the one's-complement sum over the whole message with the checksum
// field taken as zero.
func encodeMessage(msgType, ttl uint8, encoders []objEncoder) []byte {
	buf := make([]byte, maxRSVPMessage)
	off := rsvpHdrLen
	for _, enc := range encoders {
		if enc == nil {
			continue
		}
		off += enc(buf[off:])
	}
	encodeHeader(buf, Header{Version: rsvpVersion, MsgType: msgType, TTL: ttl, Length: uint16(off)})
	out := buf[:off]
	// Checksum is computed with the checksum field (bytes 2:4) zeroed; EncodeHeader
	// already wrote 0 there.
	binary.BigEndian.PutUint16(out[2:4], internetChecksum(out))
	return out
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
// SENDER_TSPEC, [RRO]. hop is this node's address (the downstream neighbour's
// PHOP) and ttl is the IP TTL echoed in the common header.
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
	// follows it.
	if psb.Protection != nil {
		pr := psb.Protection
		encoders = append(encoders,
			func(b []byte) int { return encodeSessionAttr(b, pr.sessionAttr()) },
			func(b []byte) int { return encodeFastReroute(b, pr.fastReroute()) },
		)
	}
	encoders = append(encoders,
		func(b []byte) int { return encodeSenderTemplate(b, psb.SenderTemplate) },
		func(b []byte) int { return encodeFlowSpec(b, ClassSenderTSpec, psb.SenderTSpec) },
	)
	return encodeMessage(MsgTypePath, ttl, encoders)
}

// buildResv encodes a RESV message from reservation state. RFC 3209 Section 2:
// object order is SESSION, RSVP_HOP, TIME_VALUES, STYLE, FLOWSPEC, FILTER_SPEC,
// LABEL, [RRO]. The filter spec identifies the sender being reserved for.
// A RESV travels one hop upstream toward the PHOP and is not per-hop TTL-stepped,
// so it always uses defaultIPTTL (unlike buildPath, which decrements at transit).
func buildResv(rsb *resvStateBlock, filter senderTemplateIPv4, refresh time.Duration, hop netip.Addr) []byte {
	style := rsb.Style
	if style == 0 {
		style = StyleFixedFilter
	}
	encoders := []objEncoder{
		func(b []byte) int { return encodeSessionIPv4(b, rsb.Session) },
		func(b []byte) int { return encodeRSVPHop(b, rsvpHop{NextHop: hop}) },
		func(b []byte) int { return encodeTimeValues(b, timeValues{RefreshPeriod: refreshMillis(refresh)}) },
		func(b []byte) int { return encodeStyle(b, style) },
		func(b []byte) int { return encodeFlowSpec(b, ClassFlowSpec, rsb.FlowSpec) },
		func(b []byte) int { return encodeSenderTemplate(b, filter) },
		func(b []byte) int { return encodeLabelObject(b, rsb.Label) },
	}
	if len(rsb.RRO) > 0 {
		encoders = append(encoders, func(b []byte) int { return encodeRRO(b, rsb.RRO) })
	}
	return encodeMessage(MsgTypeResv, defaultIPTTL, encoders)
}

// buildPathTear encodes a PathTear message. RFC 2205 Section 3.1.5: it carries
// SESSION, RSVP_HOP and the sender descriptor so the path state is removed
// hop-by-hop downstream.
func buildPathTear(psb *pathStateBlock, hop netip.Addr) []byte {
	encoders := []objEncoder{
		func(b []byte) int { return encodeSessionIPv4(b, psb.Session) },
		func(b []byte) int { return encodeRSVPHop(b, rsvpHop{NextHop: hop}) },
		func(b []byte) int { return encodeSenderTemplate(b, psb.SenderTemplate) },
		func(b []byte) int { return encodeFlowSpec(b, ClassSenderTSpec, psb.SenderTSpec) },
	}
	return encodeMessage(MsgTypePathTear, defaultIPTTL, encoders)
}

// buildPathErr encodes a PathErr message reporting an error toward the head-end.
// RFC 2205 Section 3.1.3: SESSION, ERROR_SPEC, then the sender descriptor. Like
// buildResv/buildPathTear it uses defaultIPTTL: a PathErr is addressed to the
// previous hop, not per-hop TTL-stepped.
func buildPathErr(session sessionIPv4, sender senderTemplateIPv4, tspec FlowSpec, es errorSpec, hop netip.Addr) []byte {
	encoders := []objEncoder{
		func(b []byte) int { return encodeSessionIPv4(b, session) },
		func(b []byte) int { return encodeRSVPHop(b, rsvpHop{NextHop: hop}) },
		func(b []byte) int { return encodeErrorSpec(b, es) },
		func(b []byte) int { return encodeSenderTemplate(b, sender) },
		func(b []byte) int { return encodeFlowSpec(b, ClassSenderTSpec, tspec) },
	}
	return encodeMessage(MsgTypePathErr, defaultIPTTL, encoders)
}

// RFC 2205 Section A.5 / RFC 3209 Section 4.3.5: error codes/values used in
// ERROR_SPEC. AdmissionControlFailure rejects a reservation for insufficient
// bandwidth; RoutingProblem (with BadEROObject) reports an ERO that cannot be
// satisfied at a transit node.
const (
	ErrCodeAdmissionControlFailure uint8  = 1
	ErrValueRequestedBandwidth     uint16 = 2
	ErrCodeRoutingProblem          uint8  = 24
	ErrValueBadEROObject           uint16 = 4
	// ErrValueNoRouteAvailable reports that the path toward the destination is
	// gone (e.g. a link on the LSP failed) -- RFC 3209 Section 4.3.5 value 5.
	ErrValueNoRouteAvailable uint16 = 5
	// RFC 4090 Section 6.5: on local repair the PLR notifies the head-end with a
	// PathErr carrying Error Code 25 ("Notify") and Error Value sub-code 3
	// ("Tunnel locally repaired"). The protected LSP is NOT torn down.
	ErrCodeNotify                 uint8  = 25
	ErrValueTunnelLocallyRepaired uint16 = 3
)
