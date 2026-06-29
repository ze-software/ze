// Design: docs/research/l2tpv2-implementation-guide.md -- LCP options + negotiation
// Related: lcp.go -- Configure-Request/Ack/Nak/Reject packets that carry options

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

// errOptionTooShort means a buffer cannot fit even the option header.
var errOptionTooShort = errors.New("ppp: LCP option shorter than 2-byte header")

// errOptionLengthMismatch means the Length field is below the header
// minimum or extends past the source buffer.
var errOptionLengthMismatch = errors.New("ppp: LCP option Length field does not fit buffer")

// LCPOption is a parsed LCP option. Data is a sub-slice of the input
// buffer (no copy).
type LCPOption struct {
	Type uint8
	Data []byte
}

// ParseLCPOptions walks an option list (the Data field of a Configure-
// Request/Ack/Nak/Reject) and returns each option in order. Stops on
// the first malformed option and returns the options parsed so far
// plus the error.
//
// RFC 1661 Section 6 specifies the Configuration Option format: Type,
// Length, Data. The Length value includes the Type and Length header
// bytes themselves.
func ParseLCPOptions(buf []byte) ([]LCPOption, error) {
	var out []LCPOption
	off := 0
	for off < len(buf) {
		if len(buf)-off < lcpOptHeaderLen {
			return out, errOptionTooShort
		}
		optLen := int(buf[off+1])
		if optLen < lcpOptHeaderLen || off+optLen > len(buf) {
			return out, errOptionLengthMismatch
		}
		out = append(out, LCPOption{
			Type: buf[off],
			Data: buf[off+lcpOptHeaderLen : off+optLen],
		})
		off += optLen
	}
	return out, nil
}

// MaxLCPOptionDataLen is the largest data payload that fits in an LCP
// option, derived from the uint8 Length field minus the 2-byte header.
const MaxLCPOptionDataLen = 255 - lcpOptHeaderLen

// WriteLCPOption encodes a single option into buf at offset off.
// Returns total bytes written (2 + len(data)). Caller MUST ensure
// buf[off:] has cap >= 2 + len(data).
//
// Panics with "BUG: ..." if len(data) > MaxLCPOptionDataLen, because
// the resulting wire packet would be malformed (silent uint8 overflow
// in the Length field). LCP option data is always small in practice
// (MRU=2, Magic=4, Auth-Proto=2-5, etc.); a caller asking for >253
// bytes is a programmer error, not a runtime condition.
func WriteLCPOption(buf []byte, off int, optType uint8, data []byte) int {
	if len(data) > MaxLCPOptionDataLen {
		panic("BUG: LCP option data exceeds 253 bytes; uint8 Length would overflow")
	}
	buf[off] = optType
	buf[off+1] = uint8(lcpOptHeaderLen + len(data))
	n := copy(buf[off+lcpOptHeaderLen:], data)
	return lcpOptHeaderLen + n
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
}

// negotiatePeerOption decides ack/nak/reject for one peer-sent option
// against the local policy. On NAK or REJECT, suggestData is the
// option's data that should be echoed back in the Nak/Reject reply.
func negotiatePeerOption(opt LCPOption, policy LCPNegPolicy) (out negOutcome, suggestData []byte) {
	switch opt.Type {
	case LCPOptMRU:
		if len(opt.Data) != 2 {
			return negReject, opt.Data
		}
		mru := binary.BigEndian.Uint16(opt.Data)
		maxMRU := policy.MaxMRU
		if maxMRU == 0 {
			maxMRU = MaxFrameLen
		}
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
		if len(opt.Data) != 4 {
			return negReject, opt.Data
		}
		// Any non-zero magic is acceptable. Zero is reserved per
		// RFC 1661 §6.4 ("an implementation that does not support
		// the option transmits the value zero").
		if binary.BigEndian.Uint32(opt.Data) == 0 {
			return negReject, opt.Data
		}
		return negAck, opt.Data

	case LCPOptAuthProto:
		if !policy.AcceptAuthProto {
			return negReject, opt.Data
		}
		if len(opt.Data) < 2 {
			return negReject, opt.Data
		}
		// 6a accepts any auth-proto value structurally (the actual
		// auth wire handling is in 6b). REJECT is the safer default
		// when AcceptAuthProto=false.
		return negAck, opt.Data

	case LCPOptACCM:
		if len(opt.Data) != 4 {
			return negReject, opt.Data
		}
		// ze does not perform HDLC framing (kernel does); any ACCM
		// value the peer wants is fine to acknowledge.
		return negAck, opt.Data

	case LCPOptPFC:
		if len(opt.Data) != 0 {
			return negReject, opt.Data
		}
		return negAck, opt.Data

	case LCPOptACFC:
		if len(opt.Data) != 0 {
			return negReject, opt.Data
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

// WriteLCPOptions serializes a list of options into buf at offset off.
// Returns total bytes written. Caller MUST ensure buf has capacity.
func WriteLCPOptions(buf []byte, off int, opts []LCPOption) int {
	written := 0
	for _, opt := range opts {
		n := WriteLCPOption(buf, off+written, opt.Type, opt.Data)
		written += n
	}
	return written
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
