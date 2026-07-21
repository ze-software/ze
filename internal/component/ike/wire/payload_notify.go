// Design: docs/architecture/wire/buffer-writer.md — buffer-first encoding
// RFC: rfc/short/rfc7296.md — Notify payload (Section 3.10)
package wire

import "encoding/binary"

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

// PayloadNotify is the Notify payload (type 41).
type PayloadNotify struct {
	ProtocolID       uint8
	SPISize          uint8
	NotifyMsgType    uint16
	SPI              []byte
	NotificationData []byte
}

func (p *PayloadNotify) Type() uint8 { return PayloadTypeNotify }

func (p *PayloadNotify) WriteTo(buf []byte, off int) int {
	buf[off] = p.ProtocolID
	buf[off+1] = p.SPISize
	binary.BigEndian.PutUint16(buf[off+2:], p.NotifyMsgType)
	n := 4
	if p.SPISize > 0 && len(p.SPI) >= int(p.SPISize) {
		copy(buf[off+n:], p.SPI[:p.SPISize])
		n += int(p.SPISize)
	} else if p.SPISize > 0 {
		buf[off+1] = 0
	}
	copy(buf[off+n:], p.NotificationData)
	n += len(p.NotificationData)
	return n
}

func (p *PayloadNotify) Len() int {
	n := 4
	// Mirror WriteTo: SPI bytes are written only when SPISize>0 and the SPI
	// slice is long enough; otherwise no SPI bytes are written.
	if p.SPISize > 0 && len(p.SPI) >= int(p.SPISize) {
		n += int(p.SPISize)
	}
	n += len(p.NotificationData)
	return n
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
		p.SPI = make([]byte, p.SPISize)
		copy(p.SPI, data[off:off+int(p.SPISize)])
		off += int(p.SPISize)
	}
	if off < len(data) {
		p.NotificationData = make([]byte, len(data)-off)
		copy(p.NotificationData, data[off:])
	}
	return nil
}
