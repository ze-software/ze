// Design: docs/research/l2tpv2-implementation-guide.md -- LCP options + negotiation
// Related: lcp.go -- Configure-Request/Ack/Nak/Reject packets that carry options
// RFC: rfc/short/rfc1661.md

package ppp

import (
	"encoding/binary"
	"errors"
)

// LCP option type values from RFC 1661 Section 6 and RFC 1570.
// Types 4 (Quality-Protocol), 6 (reserved), 9-13 (FCS, Self-Describing-
// Padding, Numbered-Mode, etc.) are not implemented in 6a.
const (
	LCPOptMRU       uint8 = 1 // RFC 1661 §6.1
	LCPOptACCM      uint8 = 2 // RFC 1662 §7.1
	LCPOptAuthProto uint8 = 3 // RFC 1661 §6.2
	LCPOptMagic     uint8 = 5 // RFC 1661 §6.4
	LCPOptPFC       uint8 = 7 // RFC 1661 §6.5
	LCPOptACFC      uint8 = 8 // RFC 1661 §6.6
)

// LCP option header: Type + Length. Length is the TOTAL option length
// including the two header bytes. RFC 1661 Section 6.
const lcpOptHeaderLen = 2

// errOptionTooShort means a buffer cannot fit even the option header, so
// the option's own Length octet already lies past the end of the buffer.
var errOptionTooShort = errors.New("ppp: LCP option shorter than 2-byte header")

// errOptionBadLength means the Length field counts fewer octets than the
// Type and Length fields it must itself account for. The option is inside
// the buffer. Only its own Length is impossible.
var errOptionBadLength = errors.New("ppp: LCP option Length field below the 2-octet header")

// errOptionLengthMismatch means the option's Data is indicated by its
// Length to extend past the end of the buffer.
//
// ipcp.go and ipv6cp.go return this sentinel for a Length less than the
// header minimum as well. Neither codec separates the two faults, because
// on the Configure-Request path scanNCPOptions (ncp.go) has already
// separated them before either codec runs.
var errOptionLengthMismatch = errors.New("ppp: LCP option Length field does not fit buffer")

// LCPOption is a parsed LCP option. Data is a sub-slice of the input
// buffer (no copy).
//
// Raw holds the option's own octets, its Type and Length header included,
// exactly as they arrived. It is set only for an option a reply MUST carry
// unchanged, and WriteLCPOptions then emits those octets instead of
// encoding Type and Data.
//
// RFC 1661 Section 5.4: "The Options field is filled with only the
// unacceptable Configuration Options from the Configure-Request. All
// recognizable and negotiable Configuration Options are filtered out of the
// Configure-Reject, but otherwise the Configuration Options MUST NOT be
// reordered or modified in any way." An option whose Length octet counts
// fewer than the two its own header occupies cannot be rebuilt from Type
// and Data, because the encoder writes the Length it computes and that
// Length is never below 2. The octets themselves are the only form of the
// option that Section 5.4 leaves ze free to send.
type LCPOption struct {
	Type uint8
	Data []byte
	Raw  []byte
}

// LCPOptionFault is what one walk over an option list found.
//
// RFC 1661 Section 6 answers the malformed shapes two different ways -- a
// Configure-Nak for an option whose own Length is invalid, no reply at all
// for an option that is not contained in the packet -- so one error value
// cannot carry the answer. A single "the options are bad" flag also has to
// spend its one bad value on "I cannot read this", which every caller
// then reads as "I read it and disliked the values"
// (ai/rules/principles.md: a value that is silently wrong must not be
// reachable).
type LCPOptionFault uint8

const (
	// LCPOptionsOK: the list walks cleanly to the end of the buffer.
	LCPOptionsOK LCPOptionFault = iota
	// LCPOptionsHeaderShort: the octets left over cannot hold a Type and a
	// Length, so the last option's Length field is past the end.
	LCPOptionsHeaderShort
	// LCPOptionsBadLength: an option declares a Length less than the two
	// octets its own Type and Length fields occupy.
	LCPOptionsBadLength
	// LCPOptionsPastEnd: an option's Data is indicated by its Length to run
	// past the end of the buffer.
	LCPOptionsPastEnd
)

// Discards reports whether RFC 1661 Section 6 answers this fault by
// dropping the whole packet. Section 6 says of the Data field: "When the
// Data field is indicated by the Length to extend beyond the end of the
// Information field, the entire packet is silently discarded without
// affecting the automaton." Two of the four faults are that shape.
func (f LCPOptionFault) Discards() bool {
	return f == LCPOptionsHeaderShort || f == LCPOptionsPastEnd
}

// LCPOptionWalk is one pass over the option list of a received LCP packet.
// Options holds every option read before the fault, and all of them when
// Fault is LCPOptionsOK. FaultOpt is the Type of the option that carries
// the fault. A Configure-Nak needs it to name the option it answers.
//
// FaultRaw is that option's Type and Length octets as they arrived, a
// sub-slice of the walked buffer. It is set for LCPOptionsBadLength, the
// one fault RFC 1661 Section 6 answers with a reply rather than with
// silence, so a Configure-Reject can carry the option unmodified. The two
// faults Discards reports leave it nil: those packets draw no reply at all,
// and the option that carries them is not delimited by the buffer either.
type LCPOptionWalk struct {
	Fault    LCPOptionFault
	FaultOpt uint8
	FaultRaw []byte
	Options  []LCPOption
}

// WalkLCPOptions walks an option list (the Data field of a Configure-
// Request/Ack/Nak/Reject) and classifies it. buf is the LCP Data field,
// which ParseLCPPacket has already cut to the packet's own Length field,
// so "past the end of buf" is "past the end of the Information field".
//
// The walk, its fault type and LCPNakOrReject are exported because the
// PPPoE client (internal/component/l2tp/pppoeclient) answers received LCP
// Configure-Requests too, and RFC 1661 Section 6 owes them the same three
// answers it owes the LNS. One walker keeps the two roles from drifting.
//
// RFC 1661 Section 6 specifies the Configuration Option format: Type,
// Length, Data. The Length value includes the Type and Length header
// bytes themselves.
func WalkLCPOptions(buf []byte) LCPOptionWalk {
	var out []LCPOption
	off := 0
	for off < len(buf) {
		// RFC 1661 Section 6: "When the Data field is indicated by the
		// Length to extend beyond the end of the Information field, the
		// entire packet is silently discarded without affecting the
		// automaton." One remaining octet cannot hold the Type and the
		// Length, so the option's own Length field lies past that end.
		if len(buf)-off < lcpOptHeaderLen {
			return LCPOptionWalk{Fault: LCPOptionsHeaderShort, FaultOpt: buf[off], Options: out}
		}
		optLen := int(buf[off+1])
		// RFC 1661 Section 6: "The Length field is one octet, and indicates
		// the length of this Configuration Option including the Type,
		// Length and Data fields." A Length less than 2 counts fewer octets
		// than the option's own header holds.
		if optLen < lcpOptHeaderLen {
			// The two header octets are inside the buffer, checked above, and
			// they are the whole of the option this fault can delimit: its own
			// Length counts fewer octets than they occupy, so nothing after
			// them belongs to it by any reading. RFC 1661 Section 5.4 requires
			// a Configure-Reject to carry the option unmodified, so those two
			// octets travel with the fault.
			return LCPOptionWalk{
				Fault:    LCPOptionsBadLength,
				FaultOpt: buf[off],
				FaultRaw: buf[off : off+lcpOptHeaderLen],
				Options:  out,
			}
		}
		// RFC 1661 Section 6: "When the Data field is indicated by the
		// Length to extend beyond the end of the Information field, the
		// entire packet is silently discarded without affecting the
		// automaton."
		if off+optLen > len(buf) {
			return LCPOptionWalk{Fault: LCPOptionsPastEnd, FaultOpt: buf[off], Options: out}
		}
		out = append(out, LCPOption{
			Type: buf[off],
			Data: buf[off+lcpOptHeaderLen : off+optLen],
		})
		off += optLen
	}
	return LCPOptionWalk{Fault: LCPOptionsOK, Options: out}
}

// ParseLCPOptions walks an option list and returns each option in order.
// Stops on the first malformed option and returns the options parsed so
// far plus the error.
//
// Callers that must tell the three RFC 1661 Section 6 faults apart use
// WalkLCPOptions. This wrapper serves the callers that only need "did the
// list read cleanly", which is every list ze did not receive in a
// Configure-Request.
func ParseLCPOptions(buf []byte) ([]LCPOption, error) {
	w := WalkLCPOptions(buf)
	switch w.Fault {
	case LCPOptionsHeaderShort:
		return w.Options, errOptionTooShort
	case LCPOptionsBadLength:
		return w.Options, errOptionBadLength
	case LCPOptionsPastEnd:
		return w.Options, errOptionLengthMismatch
	case LCPOptionsOK:
	}
	return w.Options, nil
}

// MaxLCPOptionDataLen is the largest data payload that fits in an LCP
// option, derived from the uint8 Length field minus the 2-byte header.
const MaxLCPOptionDataLen = 255 - lcpOptHeaderLen

// writeLCPOption encodes a single option into buf at offset off. It returns
// the octets written and whether the option fit. On false it writes nothing,
// so no caller can transmit half an option.
//
// An option carrying Raw is copied verbatim. RFC 1661 Section 5.4 says of
// the Configure-Reject that "the Configuration Options MUST NOT be reordered
// or modified in any way", and Raw is what arrived. Its length is bounded
// twice: WalkLCPOptions sub-slices it from a packet ParseLCPPacket has
// already cut to MaxFrameLen, and the room left in buf is checked here.
//
// Every other option is encoded from Type and Data. Data above
// MaxLCPOptionDataLen does not fit the uint8 Length field, so it is refused
// rather than written with a Length that wrapped. That is a Ze defect and
// not a peer's doing, because a received option can hold 253 data octets at
// most, but this function serves peer-derived options too and a panic on a
// wire path is never the answer.
func writeLCPOption(buf []byte, off int, opt LCPOption) (int, bool) {
	if off < 0 || off > len(buf) {
		return 0, false
	}
	room := len(buf) - off
	if len(opt.Raw) != 0 {
		if len(opt.Raw) > room {
			return 0, false
		}
		return copy(buf[off:], opt.Raw), true
	}
	if len(opt.Data) > MaxLCPOptionDataLen {
		return 0, false
	}
	total := lcpOptHeaderLen + len(opt.Data)
	if total > room {
		return 0, false
	}
	buf[off] = opt.Type
	buf[off+1] = uint8(total)
	copy(buf[off+lcpOptHeaderLen:], opt.Data)
	return total, true
}

// LCPOptions is the set of option values ze supports negotiating in
// 6a. Zero values mean "not configured" (do not send / use defaults).
//
// Magic MUST be generated via crypto/rand and MUST be non-zero per
// RFC 1661 §6.4 (zero is reserved as the "not negotiated" sentinel).
// Phase 10's Manager generates one Magic per session at goroutine
// start and never mutates it for the session's lifetime.
type LCPOptions struct {
	MRU       uint16 // 0 = do not send
	Magic     uint32 // 0 = do not send; MUST be crypto/rand non-zero per RFC 1661 §6.4
	AuthProto uint16 // 0 = do not send (no authentication negotiated)
	AuthData  []byte // optional auth-method-specific extension (e.g. CHAP algorithm = 0x05)
	ACCM      uint32 // 0 = use default (0xFFFFFFFF per RFC 1661 §6 for "all chars escape")
	HasACCM   bool
	PFC       bool
	ACFC      bool
}

// Negotiation outcomes per option, per RFC 1661 Section 5.
type negOutcome uint8

const (
	negAck    negOutcome = iota // option understood, value acceptable
	negNak                      // option understood, value unacceptable; suggest replacement
	negReject                   // option not understood or refused entirely
)

// LCPNegPolicy expresses what ze accepts FROM the peer in a peer-sent
// Configure-Request. The local Configure-Request is built separately
// from this struct (see BuildLocalConfigRequest).
type LCPNegPolicy struct {
	// MaxMRU is the largest MRU ze accepts. Peer requests above this
	// are NAKd with MaxMRU as the suggested value. Zero defaults to
	// MaxFrameLen (1500).
	MaxMRU uint16

	// AcceptAuthProto controls peer-proposed auth methods. 6a accepts
	// any non-zero AuthProto by ACK (real handling is in 6b); when
	// false (the 6a default), any peer-proposed auth is REJECTed.
	AcceptAuthProto bool

	// LocalMagic is the Magic-Number ze put in its own Configure-Request.
	// It is what a Configure-Nak of the peer's Magic-Number must differ
	// from, so the value ze offers cannot manufacture the collision the
	// option exists to detect.
	LocalMagic uint32
}

// effectiveMaxMRU is the largest MRU ze accepts. RFC 1661 Section 6.1 says
// "The default value is 1500 octets", which is what an unset policy uses.
func effectiveMaxMRU(policy LCPNegPolicy) uint16 {
	if policy.MaxMRU == 0 {
		return MaxFrameLen
	}
	return policy.MaxMRU
}

// nakMagic is a Magic-Number ze can offer the peer.
//
// RFC 1661 Section 6.4: "A Magic-Number of zero is illegal and MUST always
// be Nak'd, if it is not Rejected outright." It must also differ from ze's
// own, because Section 6.4 reads two equal Magic-Numbers as a looped-back
// link. The peer does not adopt this value: Section 6.4 has it choose a
// new Magic-Number of its own on any Configure-Nak, so the offer only has
// to be legal.
func nakMagic(local uint32) uint32 {
	if v := ^local; v != 0 {
		return v
	}
	return 1
}

// desiredLCPOption is the Configuration Option ze wants in place of one it
// refuses at the Length or the value the peer sent.
//
// RFC 1661 Section 6: "If a negotiable Configuration Option is received in
// a Configure-Request, but with an invalid or unrecognized Length, a
// Configure-Nak SHOULD be transmitted which includes the desired
// Configuration Option with an appropriate Length and Data."
//
// ok is false for a Type ze negotiates no value for: Async-Control-
// Character-Map, which ze acknowledges at any value because the kernel
// does the HDLC framing, Authentication-Protocol, which ze either accepts
// whole or refuses whole, and the two boolean options. A caller that gets
// ok=false MUST Configure-Reject instead, per RFC 1661 Section 5.4: "If
// some Configuration Options received in a Configure-Request are not
// recognizable or are not acceptable for negotiation (as configured by a
// network administrator), then the implementation MUST transmit a
// Configure-Reject." Section 5.3 forbids the alternative outright, both
// for the boolean options -- "Options which have no value fields (boolean
// options) MUST use the Configure-Reject reply instead" -- and for the
// rest, whose Nak "MUST be modified to a value acceptable to the
// Configure-Nak sender", which is the value ze does not hold.
func desiredLCPOption(optType uint8, policy LCPNegPolicy) (LCPOption, bool) {
	switch optType {
	case LCPOptMRU:
		d := make([]byte, 2)
		binary.BigEndian.PutUint16(d, effectiveMaxMRU(policy))
		return LCPOption{Type: LCPOptMRU, Data: d}, true
	case LCPOptMagic:
		d := make([]byte, 4)
		binary.BigEndian.PutUint32(d, nakMagic(policy.LocalMagic))
		return LCPOption{Type: LCPOptMagic, Data: d}, true
	}
	return LCPOption{}, false
}

// refusedOptionOutcome answers one option ze will not take as it arrived,
// either because its Length is invalid for its Type or because the value it
// carries is illegal. RFC 1661 Section 6 says of the Length field: "If a
// negotiable Configuration Option is received in a Configure-Request, but
// with an invalid or unrecognized Length, a Configure-Nak SHOULD be
// transmitted which includes the desired Configuration Option with an
// appropriate Length and Data." An illegal value earns the same reply, and
// Section 6.4 requires it of a Magic-Number of zero. received is the
// option's own data, echoed back when ze holds no desired value and has to
// Configure-Reject it instead.
func refusedOptionOutcome(optType uint8, received []byte, policy LCPNegPolicy) (negOutcome, []byte) {
	want, ok := desiredLCPOption(optType, policy)
	if !ok {
		return negReject, received
	}
	return negNak, want.Data
}

// negotiatePeerOption decides ack/nak/reject for one peer-sent option
// against the local policy. On NAK or REJECT, suggestData is the
// option's data that should be echoed back in the Nak/Reject reply.
func negotiatePeerOption(opt LCPOption, policy LCPNegPolicy) (out negOutcome, suggestData []byte) {
	switch opt.Type {
	case LCPOptMRU:
		// RFC 1661 Section 6.1 fixes the option at "Length: 4", so any
		// other Length is invalid for this Type.
		if len(opt.Data) != 2 {
			return refusedOptionOutcome(LCPOptMRU, opt.Data, policy)
		}
		mru := binary.BigEndian.Uint16(opt.Data)
		maxMRU := effectiveMaxMRU(policy)
		if mru > maxMRU {
			suggest := make([]byte, 2)
			binary.BigEndian.PutUint16(suggest, maxMRU)
			return negNak, suggest
		}
		// Floor per RFC 1661 §6.1 is 64 bytes; peers below that are
		// NAKd up to the floor.
		if mru < 64 {
			suggest := make([]byte, 2)
			binary.BigEndian.PutUint16(suggest, 64)
			return negNak, suggest
		}
		return negAck, opt.Data

	case LCPOptMagic:
		// RFC 1661 Section 6.4 fixes the option at "Length: 6", so any
		// other Length is invalid for this Type. RFC 1661 Section 6.4: "if
		// an implementation does transmit a Configure-Request with a
		// Magic-Number Configuration Option, then it MUST NOT respond with
		// a Configure-Reject when it receives a Configure-Request with a
		// Magic-Number Configuration Option." Ze always transmits one, so
		// this Length draws a Nak.
		if len(opt.Data) != 4 {
			return refusedOptionOutcome(LCPOptMagic, opt.Data, policy)
		}
		// RFC 1661 Section 6.4: "A Magic-Number of zero is illegal and MUST
		// always be Nak'd, if it is not Rejected outright." The MUST NOT
		// quoted above takes the Reject half of that choice away from ze,
		// which transmits a Magic-Number on every Configure-Request it
		// sends: run draws a non-zero value before LCP starts (generateMagic
		// in session_run.go never returns zero) and sendConfigureRequest
		// hands it to BuildLocalConfigRequest, which emits the option for
		// any non-zero value. So zero draws the Nak, carrying the value
		// nakMagic offers. Every other Magic-Number value is acknowledged.
		if binary.BigEndian.Uint32(opt.Data) == 0 {
			return refusedOptionOutcome(LCPOptMagic, opt.Data, policy)
		}
		return negAck, opt.Data

	case LCPOptAuthProto:
		if !policy.AcceptAuthProto {
			return negReject, opt.Data
		}
		// RFC 1661 Section 6.2 fixes the option at "Length: >= 4", so
		// fewer than two data octets is invalid for this Type.
		if len(opt.Data) < 2 {
			return refusedOptionOutcome(LCPOptAuthProto, opt.Data, policy)
		}
		// 6a accepts any auth-proto value structurally (the actual
		// auth wire handling is in 6b). REJECT is the safer default
		// when AcceptAuthProto=false.
		return negAck, opt.Data

	case LCPOptACCM:
		// The Async-Control-Character-Map is a four-octet bit map (see the
		// LCPOptACCM declaration above), so any other Length is invalid
		// for this Type.
		if len(opt.Data) != 4 {
			return refusedOptionOutcome(LCPOptACCM, opt.Data, policy)
		}
		// ze does not perform HDLC framing (kernel does); any ACCM
		// value the peer wants is fine to acknowledge.
		return negAck, opt.Data

	case LCPOptPFC:
		// RFC 1661 Section 6.5 fixes the option at "Length: 2", so any
		// data octet at all is invalid for this Type. RFC 1661 Section
		// 5.3: "Options which have no value fields (boolean options) MUST
		// use the Configure-Reject reply instead", which is what
		// refusedOptionOutcome returns for a Type carrying no desired
		// value.
		if len(opt.Data) != 0 {
			return refusedOptionOutcome(LCPOptPFC, opt.Data, policy)
		}
		return negAck, opt.Data

	case LCPOptACFC:
		// RFC 1661 Section 6.6 fixes the option at "Length: 2"; the
		// boolean-option rule quoted above applies here too.
		if len(opt.Data) != 0 {
			return refusedOptionOutcome(LCPOptACFC, opt.Data, policy)
		}
		return negAck, opt.Data
	}
	// Unknown option: REJECT per RFC 1661 §5 ("Configure-Reject is
	// used... when some Configuration Options received in a
	// Configure-Request are not recognizable").
	return negReject, opt.Data
}

// NegotiatePeerOptions runs every option in opts through
// negotiatePeerOption and returns three slices: the ack list, the nak
// list (with adjusted values), and the reject list.
//
// The reply rule per RFC 1661 §5 is: REJECT takes precedence over NAK
// over ACK -- if any option must be rejected, ze sends Configure-
// Reject containing only the rejected options. If none rejected but
// some need NAK, ze sends Configure-Nak with the NAKd options. Only
// when all are ACKable does ze send Configure-Ack.
//
// The caller decides which reply to actually emit; this function does
// not. Each returned LCPOption's Data points into freshly-allocated
// memory (NAK suggestions) or into the input slice (ACK echoes).
func NegotiatePeerOptions(opts []LCPOption, policy LCPNegPolicy) (acks, naks, rejects []LCPOption) {
	for _, opt := range opts {
		out, data := negotiatePeerOption(opt, policy)
		entry := LCPOption{Type: opt.Type, Data: data}
		switch out {
		case negAck:
			acks = append(acks, entry)
		case negNak:
			naks = append(naks, entry)
		case negReject:
			rejects = append(rejects, entry)
		}
	}
	return
}

// WriteLCPOptions serializes a list of options into buf at offset off. It
// returns the octets written and whether every option fit.
//
// On false the caller MUST NOT transmit what was written. A reply carrying
// some of the options it owes is a different packet from the one the RFC
// asks for, and RFC 1661 Section 5.4 fills the Options field with "only the
// unacceptable Configuration Options from the Configure-Request", not with
// the prefix of them that happened to fit.
//
// The list can be longer than the request that produced it: a Magic-Number
// received at Length 2 is answered by six octets, so a peer that fills a
// frame with them asks for a reply three times the size of its own packet.
// The room left in buf is therefore checked for each option instead of
// assumed from the caller.
func WriteLCPOptions(buf []byte, off int, opts []LCPOption) (int, bool) {
	written := 0
	for _, opt := range opts {
		n, ok := writeLCPOption(buf, off+written, opt)
		if !ok {
			return written, false
		}
		written += n
	}
	return written, true
}

// BuildLocalConfigRequest constructs ze's Configure-Request option
// list from an LCPOptions struct. Zero-valued fields are omitted. The
// returned options' Data fields are owned by the caller (allocated
// here).
func BuildLocalConfigRequest(o LCPOptions) []LCPOption {
	var opts []LCPOption
	if o.MRU != 0 {
		d := make([]byte, 2)
		binary.BigEndian.PutUint16(d, o.MRU)
		opts = append(opts, LCPOption{Type: LCPOptMRU, Data: d})
	}
	if o.AuthProto != 0 {
		d := make([]byte, 2+len(o.AuthData))
		binary.BigEndian.PutUint16(d[:2], o.AuthProto)
		copy(d[2:], o.AuthData)
		opts = append(opts, LCPOption{Type: LCPOptAuthProto, Data: d})
	}
	if o.Magic != 0 {
		d := make([]byte, 4)
		binary.BigEndian.PutUint32(d, o.Magic)
		opts = append(opts, LCPOption{Type: LCPOptMagic, Data: d})
	}
	if o.HasACCM {
		d := make([]byte, 4)
		binary.BigEndian.PutUint32(d, o.ACCM)
		opts = append(opts, LCPOption{Type: LCPOptACCM, Data: d})
	}
	if o.PFC {
		opts = append(opts, LCPOption{Type: LCPOptPFC, Data: nil})
	}
	if o.ACFC {
		opts = append(opts, LCPOption{Type: LCPOptACFC, Data: nil})
	}
	return opts
}
