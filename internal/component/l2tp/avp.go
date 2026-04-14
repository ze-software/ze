// Design: docs/architecture/wire/l2tp.md — L2TP AVP parse and encode
// RFC: rfc/short/rfc2661.md — RFC 2661 Section 4.1 (AVP format)
// Related: header.go — AVPs follow the header in a control-message payload
// Related: avp_compound.go — compound-value AVPs (Result Code, Q.931, Call Errors, ACCM)
// Related: hidden.go — AVPs with the H bit use hidden-value encryption
// Related: errors.go — ErrInvalidAVPLen

package l2tp

import "encoding/binary"

// AVPType is the 16-bit Attribute Type field of an AVP. For Vendor ID = 0
// (IETF standard space), the values follow the RFC 2661 catalog.
type AVPType uint16

// RFC 2661 AVP catalog (Vendor ID = 0).
const (
	AVPMessageType               AVPType = 0
	AVPResultCode                AVPType = 1
	AVPProtocolVersion           AVPType = 2
	AVPFramingCapabilities       AVPType = 3
	AVPBearerCapabilities        AVPType = 4
	AVPTieBreaker                AVPType = 5
	AVPFirmwareRevision          AVPType = 6
	AVPHostName                  AVPType = 7
	AVPVendorName                AVPType = 8
	AVPAssignedTunnelID          AVPType = 9
	AVPReceiveWindowSize         AVPType = 10
	AVPChallenge                 AVPType = 11
	AVPQ931CauseCode             AVPType = 12
	AVPChallengeResponse         AVPType = 13
	AVPAssignedSessionID         AVPType = 14
	AVPCallSerialNumber          AVPType = 15
	AVPMinimumBPS                AVPType = 16
	AVPMaximumBPS                AVPType = 17
	AVPBearerType                AVPType = 18
	AVPFramingType               AVPType = 19
	AVPCalledNumber              AVPType = 21
	AVPCallingNumber             AVPType = 22
	AVPSubAddress                AVPType = 23
	AVPTxConnectSpeed            AVPType = 24
	AVPPhysicalChannelID         AVPType = 25
	AVPInitialReceivedLCPConfReq AVPType = 26
	AVPLastSentLCPConfReq        AVPType = 27
	AVPLastReceivedLCPConfReq    AVPType = 28
	AVPProxyAuthenType           AVPType = 29
	AVPProxyAuthenName           AVPType = 30
	AVPProxyAuthenChallenge      AVPType = 31
	AVPProxyAuthenID             AVPType = 32
	AVPProxyAuthenResponse       AVPType = 33
	AVPCallErrors                AVPType = 34
	AVPACCM                      AVPType = 35
	AVPRandomVector              AVPType = 36
	AVPPrivateGroupID            AVPType = 37
	AVPRxConnectSpeed            AVPType = 38
	AVPSequencingRequired        AVPType = 39
)

// MessageType is the value of the Message Type AVP (AVPMessageType).
type MessageType uint16

// RFC 2661 Section 3.2 control message types.
const (
	MsgSCCRQ   MessageType = 1
	MsgSCCRP   MessageType = 2
	MsgSCCCN   MessageType = 3
	MsgStopCCN MessageType = 4
	MsgHello   MessageType = 6
	MsgOCRQ    MessageType = 7
	MsgOCRP    MessageType = 8
	MsgOCCN    MessageType = 9
	MsgICRQ    MessageType = 10
	MsgICRP    MessageType = 11
	MsgICCN    MessageType = 12
	MsgCDN     MessageType = 14
	MsgWEN     MessageType = 15
	MsgSLI     MessageType = 16
)

// AVPFlags is the decoded M/H/reserved flag state of an AVP.
// RFC 2661 Section 4.1: M in bit 0, H in bit 1, reserved in bits 2-5.
type AVPFlags uint8

const (
	// FlagMandatory reflects the M bit (unrecognized mandatory AVP => tear down).
	FlagMandatory AVPFlags = 1 << 0
	// FlagHidden reflects the H bit (value is encrypted, Section 4.3).
	FlagHidden AVPFlags = 1 << 1
	// FlagReserved indicates that one or more reserved bits were non-zero on
	// the wire. Per RFC 2661 Section 4.1, the AVP must then be treated as
	// unrecognized; upstream applies the M-bit handling rule.
	FlagReserved AVPFlags = 1 << 2
)

// AVPHeaderLen is the fixed 6-byte AVP header length.
const AVPHeaderLen = 6

// AVPMaxLen is the maximum AVP Length the 10-bit field can express.
const AVPMaxLen = 1023

// AVPIterator walks the AVP stream in an L2TP control-message payload.
//
// Zero-copy: the value returned by Next is a slice of the buffer passed to
// NewAVPIterator. The caller MUST NOT modify the buffer while iterating.
type AVPIterator struct {
	data   []byte
	offset int
	err    error
}

// NewAVPIterator returns an iterator over the AVP stream in payload.
func NewAVPIterator(payload []byte) AVPIterator {
	return AVPIterator{data: payload}
}

// Next returns the next AVP. On exhaustion or on the first malformed AVP,
// it returns ok=false and Err reports the cause.
func (it *AVPIterator) Next() (vendorID uint16, attrType AVPType, flags AVPFlags, value []byte, ok bool) {
	if it.err != nil || it.offset >= len(it.data) {
		return 0, 0, 0, nil, false
	}
	if it.offset+AVPHeaderLen > len(it.data) {
		it.err = ErrInvalidAVPLen
		return 0, 0, 0, nil, false
	}
	word := binary.BigEndian.Uint16(it.data[it.offset:])
	length := int(word & 0x03FF)
	// RFC 2661 Section 4.1: Length < 6 is malformed; so is Length extending past payload.
	if length < AVPHeaderLen || it.offset+length > len(it.data) {
		it.err = ErrInvalidAVPLen
		return 0, 0, 0, nil, false
	}
	if word&0x8000 != 0 {
		flags |= FlagMandatory
	}
	if word&0x4000 != 0 {
		flags |= FlagHidden
	}
	// Reserved bits (2-5) of the first byte => bits 11-8 of `word`.
	if word&0x3C00 != 0 {
		flags |= FlagReserved
	}
	vendorID = binary.BigEndian.Uint16(it.data[it.offset+2:])
	attrType = AVPType(binary.BigEndian.Uint16(it.data[it.offset+4:]))
	value = it.data[it.offset+AVPHeaderLen : it.offset+length]
	it.offset += length
	return vendorID, attrType, flags, value, true
}

// Err returns the iteration error, if any. Nil means clean exhaustion.
func (it *AVPIterator) Err() error { return it.err }

// Remaining returns the number of bytes not yet consumed.
func (it *AVPIterator) Remaining() int { return len(it.data) - it.offset }

// WriteAVPHeader writes the 6-byte AVP header into buf at off, with the
// given flags, vendorID, attrType, and total length (valueLen + AVPHeaderLen).
// Returns AVPHeaderLen.
//
// Precondition: AVPHeaderLen <= totalLen <= AVPMaxLen. The Length field is
// 10 bits; violating the upper bound would silently truncate and produce
// malformed wire bytes, so we panic instead (programmer error, not a
// runtime condition).
func WriteAVPHeader(buf []byte, off int, flags AVPFlags, vendorID uint16, attrType AVPType, totalLen int) int {
	if totalLen < AVPHeaderLen || totalLen > AVPMaxLen {
		panic("BUG: l2tp WriteAVPHeader totalLen out of range [6,1023]")
	}
	var word uint16
	if flags&FlagMandatory != 0 {
		word |= 0x8000
	}
	if flags&FlagHidden != 0 {
		word |= 0x4000
	}
	word |= uint16(totalLen) & 0x03FF
	binary.BigEndian.PutUint16(buf[off:], word)
	binary.BigEndian.PutUint16(buf[off+2:], vendorID)
	binary.BigEndian.PutUint16(buf[off+4:], uint16(attrType))
	return AVPHeaderLen
}

// WriteAVPBytes writes an AVP whose value is the raw bytes in value.
// Returns AVPHeaderLen + len(value).
func WriteAVPBytes(buf []byte, off int, mandatory bool, vendorID uint16, attrType AVPType, value []byte) int {
	total := AVPHeaderLen + len(value)
	flags := AVPFlags(0)
	if mandatory {
		flags |= FlagMandatory
	}
	WriteAVPHeader(buf, off, flags, vendorID, attrType, total)
	copy(buf[off+AVPHeaderLen:], value)
	return total
}

// WriteAVPEmpty writes a header-only AVP (no value), used by
// AVPSequencingRequired and similar. Returns AVPHeaderLen.
func WriteAVPEmpty(buf []byte, off int, mandatory bool, vendorID uint16, attrType AVPType) int {
	flags := AVPFlags(0)
	if mandatory {
		flags |= FlagMandatory
	}
	return WriteAVPHeader(buf, off, flags, vendorID, attrType, AVPHeaderLen)
}

// WriteAVPUint8 writes an AVP with a 1-byte value. Returns AVPHeaderLen + 1.
func WriteAVPUint8(buf []byte, off int, mandatory bool, attrType AVPType, value uint8) int {
	buf[off+AVPHeaderLen] = value
	flags := AVPFlags(0)
	if mandatory {
		flags |= FlagMandatory
	}
	WriteAVPHeader(buf, off, flags, 0, attrType, AVPHeaderLen+1)
	return AVPHeaderLen + 1
}

// WriteAVPUint16 writes an AVP with a uint16 value. Returns AVPHeaderLen + 2.
func WriteAVPUint16(buf []byte, off int, mandatory bool, attrType AVPType, value uint16) int {
	binary.BigEndian.PutUint16(buf[off+AVPHeaderLen:], value)
	flags := AVPFlags(0)
	if mandatory {
		flags |= FlagMandatory
	}
	WriteAVPHeader(buf, off, flags, 0, attrType, AVPHeaderLen+2)
	return AVPHeaderLen + 2
}

// WriteAVPUint32 writes an AVP with a uint32 value. Returns AVPHeaderLen + 4.
func WriteAVPUint32(buf []byte, off int, mandatory bool, attrType AVPType, value uint32) int {
	binary.BigEndian.PutUint32(buf[off+AVPHeaderLen:], value)
	flags := AVPFlags(0)
	if mandatory {
		flags |= FlagMandatory
	}
	WriteAVPHeader(buf, off, flags, 0, attrType, AVPHeaderLen+4)
	return AVPHeaderLen + 4
}

// WriteAVPUint64 writes an AVP with a uint64 value (e.g. Tie Breaker).
// Returns AVPHeaderLen + 8.
func WriteAVPUint64(buf []byte, off int, mandatory bool, attrType AVPType, value uint64) int {
	binary.BigEndian.PutUint64(buf[off+AVPHeaderLen:], value)
	flags := AVPFlags(0)
	if mandatory {
		flags |= FlagMandatory
	}
	WriteAVPHeader(buf, off, flags, 0, attrType, AVPHeaderLen+8)
	return AVPHeaderLen + 8
}

// WriteAVPString writes an AVP whose value is a UTF-8 string without a null
// terminator. Returns AVPHeaderLen + len(s).
//
// Precondition: buf[off:] has room for AVPHeaderLen + len(s) bytes. We
// compare the copy count against len(s) and panic on truncation (rather
// than silently shorten) because a truncated string on the wire produces
// a valid-looking AVP with wrong contents.
func WriteAVPString(buf []byte, off int, mandatory bool, attrType AVPType, s string) int {
	n := copy(buf[off+AVPHeaderLen:], s)
	if n != len(s) {
		panic("BUG: l2tp WriteAVPString buf too small; caller MUST size buf to AVPHeaderLen+len(s)")
	}
	flags := AVPFlags(0)
	if mandatory {
		flags |= FlagMandatory
	}
	total := AVPHeaderLen + n
	WriteAVPHeader(buf, off, flags, 0, attrType, total)
	return total
}

// ReadAVPUint8 extracts the 1-byte value. Returns ErrInvalidAVPLen on wrong size.
func ReadAVPUint8(value []byte) (uint8, error) {
	if len(value) != 1 {
		return 0, ErrInvalidAVPLen
	}
	return value[0], nil
}

// ReadAVPUint16 extracts the uint16 value. Returns ErrInvalidAVPLen on wrong size.
func ReadAVPUint16(value []byte) (uint16, error) {
	if len(value) != 2 {
		return 0, ErrInvalidAVPLen
	}
	return binary.BigEndian.Uint16(value), nil
}

// ReadAVPUint32 extracts the uint32 value. Returns ErrInvalidAVPLen on wrong size.
func ReadAVPUint32(value []byte) (uint32, error) {
	if len(value) != 4 {
		return 0, ErrInvalidAVPLen
	}
	return binary.BigEndian.Uint32(value), nil
}

// ReadAVPUint64 extracts the uint64 value. Returns ErrInvalidAVPLen on wrong size.
func ReadAVPUint64(value []byte) (uint64, error) {
	if len(value) != 8 {
		return 0, ErrInvalidAVPLen
	}
	return binary.BigEndian.Uint64(value), nil
}
