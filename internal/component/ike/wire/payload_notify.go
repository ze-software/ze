// Design: docs/architecture/wire/buffer-writer.md — buffer-first encoding
// RFC: rfc/short/rfc7296.md — Notify payload (Section 3.10)
package wire

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// ErrSetWindowSizeLength reports SET_WINDOW_SIZE notification data whose length is not
// exactly 4 octets. RFC 7296 Section 3.10.1 gives the notification a fixed body, so a
// body of another length is refused rather than truncated or zero-extended.
var ErrSetWindowSizeLength = errors.New("set-window-size notification data length")

// ParseSetWindowSize reads the window a peer promises to keep from the Notification
// Data of a SET_WINDOW_SIZE notify.
//
// RFC 7296 Section 2.3 MUST: "The data associated with a SET_WINDOW_SIZE notification
// MUST be 4 octets long and contain the big endian representation of the number of
// messages the sender promises to keep". The length is exact, so 3 octets and 5 octets
// are both refused, and the four octets are read big-endian.
func ParseSetWindowSize(data []byte) (uint32, error) {
	if len(data) != 4 {
		return 0, fmt.Errorf("%w: %d octets, want 4", ErrSetWindowSizeLength, len(data))
	}
	return binary.BigEndian.Uint32(data), nil
}

// Notify message types (RFC 7296 Section 3.10.1).
const (
	NotifyUnsupportedCriticalPayload uint16 = 1
	NotifyInvalidIKESPI              uint16 = 4
	NotifyInvalidMajorVersion        uint16 = 5
	NotifyInvalidSyntax              uint16 = 7
	NotifyInvalidMessageID           uint16 = 9
	NotifyInvalidSPI                 uint16 = 11
	NotifyNoProposalChosen           uint16 = 14
	NotifyInvalidKEPayload           uint16 = 17
	NotifyAuthenticationFailed       uint16 = 24
	NotifySinglePairRequired         uint16 = 34
	NotifyNoAdditionalSAs            uint16 = 35
	NotifyInternalAddressFailure     uint16 = 36
	NotifyFailedCPRequired           uint16 = 37
	NotifyTSUnacceptable             uint16 = 38
	NotifyInvalidSelectors           uint16 = 39
	NotifyTemporaryFailure           uint16 = 43
	NotifyChildSANotFound            uint16 = 44

	NotifyInitialContact            uint16 = 16384
	NotifySetWindowSize             uint16 = 16385
	NotifyAdditionalTSPossible      uint16 = 16386
	NotifyIPCompSupported           uint16 = 16387
	NotifyNATDetectionSourceIP      uint16 = 16388
	NotifyNATDetectionDestIP        uint16 = 16389
	NotifyCookie                    uint16 = 16390
	NotifyUseTransportMode          uint16 = 16391
	NotifyRekeySA                   uint16 = 16393
	NotifyESPTFCPaddingNotSupported uint16 = 16394
	NotifyNonFirstFragmentsAlso     uint16 = 16395
	NotifyFragmentationSupported    uint16 = 16430
	NotifySignatureHashAlgorithms   uint16 = 16431
)

// NotifyStatusFloor is the first notify message type that reports status.
//
// RFC 7296 Section 3.10.1: "Types in the range 0 - 16383 are intended for reporting
// errors." Every type at or above this floor is a status type.
const NotifyStatusFloor uint16 = 16384

// notifyTypeNames is the registry of every notify message type this implementation
// understands. It is the single source for recognition and for log text, so a new
// constant is registered here beside its declaration and nowhere else
// (ai/rules/derive-not-hardcode.md).
var notifyTypeNames = map[uint16]string{
	NotifyUnsupportedCriticalPayload: "UNSUPPORTED_CRITICAL_PAYLOAD",
	NotifyInvalidIKESPI:              "INVALID_IKE_SPI",
	NotifyInvalidMajorVersion:        "INVALID_MAJOR_VERSION",
	NotifyInvalidSyntax:              "INVALID_SYNTAX",
	NotifyInvalidMessageID:           "INVALID_MESSAGE_ID",
	NotifyInvalidSPI:                 "INVALID_SPI",
	NotifyNoProposalChosen:           "NO_PROPOSAL_CHOSEN",
	NotifyInvalidKEPayload:           "INVALID_KE_PAYLOAD",
	NotifyAuthenticationFailed:       "AUTHENTICATION_FAILED",
	NotifySinglePairRequired:         "SINGLE_PAIR_REQUIRED",
	NotifyNoAdditionalSAs:            "NO_ADDITIONAL_SAS",
	NotifyInternalAddressFailure:     "INTERNAL_ADDRESS_FAILURE",
	NotifyFailedCPRequired:           "FAILED_CP_REQUIRED",
	NotifyTSUnacceptable:             "TS_UNACCEPTABLE",
	NotifyInvalidSelectors:           "INVALID_SELECTORS",
	NotifyTemporaryFailure:           "TEMPORARY_FAILURE",
	NotifyChildSANotFound:            "CHILD_SA_NOT_FOUND",

	NotifyInitialContact:            "INITIAL_CONTACT",
	NotifySetWindowSize:             "SET_WINDOW_SIZE",
	NotifyAdditionalTSPossible:      "ADDITIONAL_TS_POSSIBLE",
	NotifyIPCompSupported:           "IPCOMP_SUPPORTED",
	NotifyNATDetectionSourceIP:      "NAT_DETECTION_SOURCE_IP",
	NotifyNATDetectionDestIP:        "NAT_DETECTION_DESTINATION_IP",
	NotifyCookie:                    "COOKIE",
	NotifyUseTransportMode:          "USE_TRANSPORT_MODE",
	NotifyRekeySA:                   "REKEY_SA",
	NotifyESPTFCPaddingNotSupported: "ESP_TFC_PADDING_NOT_SUPPORTED",
	NotifyNonFirstFragmentsAlso:     "NON_FIRST_FRAGMENTS_ALSO",
	NotifyFragmentationSupported:    "IKEV2_FRAGMENTATION_SUPPORTED",
	NotifySignatureHashAlgorithms:   "SIGNATURE_HASH_ALGORITHMS",
}

// NotifyIsError reports whether a notify message type reports an error.
//
// RFC 7296 Section 3.10.1 splits the number space at 16384. The two halves get
// opposite treatment on receipt, so the split is a named test rather than a literal
// repeated at each reader.
func NotifyIsError(t uint16) bool { return t < NotifyStatusFloor }

// NotifyTypeRecognized reports whether this implementation understands a notify
// message type.
//
// It fails closed. A type absent from the registry reads false, so an unknown value
// can never pass as understood (ai/rules/fail-closed-guards.md).
func NotifyTypeRecognized(t uint16) bool {
	_, ok := notifyTypeNames[t]
	return ok
}

// NotifyTypeName returns the RFC name of a notify message type for log text, or
// "UNRECOGNIZED" when the registry holds no entry.
func NotifyTypeName(t uint16) string {
	if name, ok := notifyTypeNames[t]; ok {
		return name
	}
	return "UNRECOGNIZED"
}

// PayloadNotify is the Notify payload (type 41).
type PayloadNotify struct {
	ProtocolID       uint8
	SPISize          uint8
	NotifyMsgType    uint16
	SPI              []byte
	NotificationData []byte
}

func (p *PayloadNotify) Type() uint8 { return PayloadTypeNotify }

// spiLen reports how many SPI octets WriteTo writes. The SPI field is filled only
// when SPISize is set and the SPI slice holds at least that many octets. Every other
// case leaves the field empty, which drives the Protocol ID rule in WriteTo.
func (p *PayloadNotify) spiLen() int {
	if p.SPISize > 0 && len(p.SPI) >= int(p.SPISize) {
		return int(p.SPISize)
	}
	return 0
}

func (p *PayloadNotify) WriteTo(buf []byte, off int) int {
	spiLen := p.spiLen()
	// RFC 7296 Section 3.10 MUST: "If the SPI field is empty, this field MUST be sent
	// as zero". The Protocol ID and the SPI Size octets both follow the SPI octets that
	// are written, so a stale Protocol ID never reaches a peer beside an empty SPI.
	if spiLen == 0 {
		buf[off] = 0
		buf[off+1] = 0
	} else {
		buf[off] = p.ProtocolID
		buf[off+1] = p.SPISize
	}
	binary.BigEndian.PutUint16(buf[off+2:], p.NotifyMsgType)
	n := 4
	if spiLen > 0 {
		copy(buf[off+n:], p.SPI[:spiLen])
		n += spiLen
	}
	copy(buf[off+n:], p.NotificationData)
	n += len(p.NotificationData)
	return n
}

func (p *PayloadNotify) Len() int {
	return 4 + p.spiLen() + len(p.NotificationData)
}

func (p *PayloadNotify) ReadFrom(data []byte) error {
	if len(data) < 4 {
		return ErrTruncated
	}
	p.ProtocolID = data[0]
	p.SPISize = data[1]
	p.NotifyMsgType = binary.BigEndian.Uint16(data[2:4])
	off := 4
	if int(p.SPISize) > len(data)-off {
		return ErrTruncated
	}
	if p.SPISize > 0 {
		// RFC 7296 Section 3.10 MUST: "For notifications concerning Child SAs, this
		// field MUST contain either (2) to indicate AH or (3) to indicate ESP". A
		// notification about the IKE SA carries an empty SPI field, so an SPI here
		// always names a Child SA. No other Protocol ID value is valid.
		if p.ProtocolID != ProtocolAH && p.ProtocolID != ProtocolESP {
			return ErrNotifyProtocolID
		}
		p.SPI = make([]byte, p.SPISize)
		copy(p.SPI, data[off:off+int(p.SPISize)])
		off += int(p.SPISize)
	} else {
		// RFC 7296 Section 3.10 MUST: beside an empty SPI field the Protocol ID "MUST
		// be ignored on receipt". The octet is discarded here, so no later reader can
		// act on a value that the RFC declares dead.
		p.ProtocolID = 0
		p.SPI = nil
	}
	if off < len(data) {
		p.NotificationData = make([]byte, len(data)-off)
		copy(p.NotificationData, data[off:])
	}
	return nil
}
