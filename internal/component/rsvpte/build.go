// Design: plan/spec-mpls-3-rsvp-te.md -- RSVP-TE full-message encoders
// Related: wire.go -- per-object Encode*/Decode* primitives this composes
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
	EncodeHeader(buf, Header{Version: rsvpVersion, MsgType: msgType, TTL: ttl, Length: uint16(off)})
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

// BuildPath encodes a PATH message from path state. RFC 3209 Section 2: object
// order is SESSION, RSVP_HOP, TIME_VALUES, [ERO], LABEL_REQUEST, SENDER_TEMPLATE,
// SENDER_TSPEC, [RRO]. hop is this node's address (the downstream neighbour's
// PHOP) and ttl is the IP TTL echoed in the common header.
func BuildPath(psb *PathStateBlock, hop netip.Addr, ttl uint8) []byte {
	encoders := []objEncoder{
		func(b []byte) int { return EncodeSessionIPv4(b, psb.Session) },
		func(b []byte) int { return EncodeRSVPHop(b, RSVPHop{NextHop: hop}) },
		func(b []byte) int {
			return EncodeTimeValues(b, TimeValues{RefreshPeriod: refreshMillis(psb.RefreshPeriod)})
		},
	}
	if len(psb.ERO) > 0 {
		encoders = append(encoders, func(b []byte) int { return EncodeERO(b, psb.ERO) })
	}
	encoders = append(encoders,
		func(b []byte) int { return EncodeLabelRequest(b, psb.LabelRequest) },
		func(b []byte) int { return EncodeSenderTemplate(b, psb.SenderTemplate) },
		func(b []byte) int { return EncodeFlowSpec(b, ClassSenderTSpec, psb.SenderTSpec) },
	)
	return encodeMessage(MsgTypePath, ttl, encoders)
}

// BuildResv encodes a RESV message from reservation state. RFC 3209 Section 2:
// object order is SESSION, RSVP_HOP, TIME_VALUES, STYLE, FLOWSPEC, FILTER_SPEC,
// LABEL, [RRO]. The filter spec identifies the sender being reserved for.
func BuildResv(rsb *ResvStateBlock, filter SenderTemplateIPv4, refresh time.Duration, hop netip.Addr, ttl uint8) []byte {
	style := rsb.Style
	if style == 0 {
		style = StyleFixedFilter
	}
	encoders := []objEncoder{
		func(b []byte) int { return EncodeSessionIPv4(b, rsb.Session) },
		func(b []byte) int { return EncodeRSVPHop(b, RSVPHop{NextHop: hop}) },
		func(b []byte) int { return EncodeTimeValues(b, TimeValues{RefreshPeriod: refreshMillis(refresh)}) },
		func(b []byte) int { return EncodeStyle(b, style) },
		func(b []byte) int { return EncodeFlowSpec(b, ClassFlowSpec, rsb.FlowSpec) },
		func(b []byte) int { return EncodeSenderTemplate(b, filter) },
		func(b []byte) int { return EncodeLabelObject(b, rsb.Label) },
	}
	return encodeMessage(MsgTypeResv, ttl, encoders)
}

// BuildPathTear encodes a PathTear message. RFC 2205 Section 3.1.5: it carries
// SESSION, RSVP_HOP and the sender descriptor so the path state is removed
// hop-by-hop downstream.
func BuildPathTear(psb *PathStateBlock, hop netip.Addr, ttl uint8) []byte {
	encoders := []objEncoder{
		func(b []byte) int { return EncodeSessionIPv4(b, psb.Session) },
		func(b []byte) int { return EncodeRSVPHop(b, RSVPHop{NextHop: hop}) },
		func(b []byte) int { return EncodeSenderTemplate(b, psb.SenderTemplate) },
		func(b []byte) int { return EncodeFlowSpec(b, ClassSenderTSpec, psb.SenderTSpec) },
	}
	return encodeMessage(MsgTypePathTear, ttl, encoders)
}

// BuildResvTear encodes a ResvTear message removing reservation state upstream.
func BuildResvTear(rsb *ResvStateBlock, filter SenderTemplateIPv4, hop netip.Addr, ttl uint8) []byte {
	style := rsb.Style
	if style == 0 {
		style = StyleFixedFilter
	}
	encoders := []objEncoder{
		func(b []byte) int { return EncodeSessionIPv4(b, rsb.Session) },
		func(b []byte) int { return EncodeRSVPHop(b, RSVPHop{NextHop: hop}) },
		func(b []byte) int { return EncodeStyle(b, style) },
		func(b []byte) int { return EncodeSenderTemplate(b, filter) },
	}
	return encodeMessage(MsgTypeResvTear, ttl, encoders)
}

// BuildPathErr encodes a PathErr message reporting an error toward the head-end.
// RFC 2205 Section 3.1.3: SESSION, ERROR_SPEC, then the sender descriptor.
func BuildPathErr(session SessionIPv4, sender SenderTemplateIPv4, tspec FlowSpec, es ErrorSpec, hop netip.Addr, ttl uint8) []byte {
	encoders := []objEncoder{
		func(b []byte) int { return EncodeSessionIPv4(b, session) },
		func(b []byte) int { return EncodeRSVPHop(b, RSVPHop{NextHop: hop}) },
		func(b []byte) int { return EncodeErrorSpec(b, es) },
		func(b []byte) int { return EncodeSenderTemplate(b, sender) },
		func(b []byte) int { return EncodeFlowSpec(b, ClassSenderTSpec, tspec) },
	}
	return encodeMessage(MsgTypePathErr, ttl, encoders)
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
)
