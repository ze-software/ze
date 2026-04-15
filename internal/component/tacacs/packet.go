// Design: (none -- new TACACS+ component)
// Detail: authen.go -- authentication START/REPLY (RFC 8907 Section 5)
// Detail: author.go -- authorization REQUEST/RESPONSE (RFC 8907 Section 6)
// Detail: acct.go -- accounting REQUEST/REPLY (RFC 8907 Section 7)
// Detail: client.go -- TCP client with server failover
// Detail: authenticator.go -- bridges client to authz.Authenticator
// Detail: authorizer.go -- bridges client to per-command authorization
// Detail: accounting.go -- bridges client to aaa.Accountant
// Detail: config.go -- config extraction from YANG tree

// Package tacacs implements the TACACS+ protocol (RFC 8907) client.
// RFC 8907 Section 4 -- packet header and body encryption.
package tacacs

import (
	"crypto/md5" //nolint:gosec // RFC 8907 Section 4.6 mandates MD5 for pseudo-pad
	"encoding/binary"
	"errors"
	"fmt"
)

// RFC 8907 Section 4.1 -- header constants.
const (
	hdrLen = 12 // fixed header size in bytes

	// PacketHeader flags.
	flagUnencrypted = 0x01 // TAC_PLUS_UNENCRYPTED_FLAG

	// Maximum body length (uint16 max, practical limit).
	maxBodyLen = 65535
)

// PacketHeader is a 12-byte TACACS+ packet header.
// RFC 8907 Section 4.1.
type PacketHeader struct {
	Version   uint8  // upper nibble = major, lower nibble = minor
	Type      uint8  // 0x01=authen, 0x02=author, 0x03=acct
	SeqNo     uint8  // starts at 1, increments per packet
	Flags     uint8  // bit flags
	SessionID uint32 // random, constant per session
	Length    uint32 // body length
}

// MarshalBinary encodes the header to 12 bytes in network byte order.
func (h *PacketHeader) MarshalBinary() []byte {
	buf := make([]byte, hdrLen)
	buf[0] = h.Version
	buf[1] = h.Type
	buf[2] = h.SeqNo
	buf[3] = h.Flags
	binary.BigEndian.PutUint32(buf[4:8], h.SessionID)
	binary.BigEndian.PutUint32(buf[8:12], h.Length)
	return buf
}

// UnmarshalPacketHeader decodes a 12-byte header from network byte order.
func UnmarshalPacketHeader(data []byte) (PacketHeader, error) {
	if len(data) < hdrLen {
		return PacketHeader{}, fmt.Errorf("header too short: %d bytes", len(data))
	}
	return PacketHeader{
		Version:   data[0],
		Type:      data[1],
		SeqNo:     data[2],
		Flags:     data[3],
		SessionID: binary.BigEndian.Uint32(data[4:8]),
		Length:    binary.BigEndian.Uint32(data[8:12]),
	}, nil
}

// Errors for packet processing.
var (
	ErrBadSecret   = errors.New("bad secret: body length mismatch after decryption")
	ErrBodyTooBig  = errors.New("body exceeds maximum length")
	ErrSeqOverflow = errors.New("sequence number overflow")
)

// Encrypt obfuscates or de-obfuscates the packet body using the MD5 pseudo-pad.
// RFC 8907 Section 4.6. XOR is self-inverse, so encrypt == decrypt.
//
// The pseudo-pad is generated as:
//
//	MD5_1 = MD5(session_id || key || version || seq_no)
//	MD5_i = MD5(session_id || key || version || seq_no || MD5_(i-1))
//
// Each block is 16 bytes. The pad is truncated to body length and XORed.
func Encrypt(body []byte, sessionID uint32, key []byte, version, seqNo uint8) {
	if len(body) == 0 || len(key) == 0 {
		return
	}

	// Build the initial MD5 input: session_id(4) + key(N) + version(1) + seq_no(1)
	inputLen := 4 + len(key) + 2
	input := make([]byte, inputLen)
	binary.BigEndian.PutUint32(input[0:4], sessionID)
	copy(input[4:4+len(key)], key)
	input[inputLen-2] = version
	input[inputLen-1] = seqNo

	h := md5.New() //nolint:gosec // RFC 8907 Section 4.6 mandates MD5
	var prevSum []byte

	for off := 0; off < len(body); off += md5.Size {
		h.Reset()
		h.Write(input)
		if len(prevSum) > 0 {
			h.Write(prevSum)
		}
		prevSum = h.Sum(nil)

		end := min(off+md5.Size, len(body))
		for i := off; i < end; i++ {
			body[i] ^= prevSum[i-off]
		}
	}
}

// Packet is a complete TACACS+ packet (header + body).
type Packet struct {
	Header PacketHeader
	Body   []byte
}

// Marshal encodes a packet to wire format with optional encryption.
// If key is non-empty, the body is encrypted in place.
func (p *Packet) Marshal(key []byte) ([]byte, error) {
	if len(p.Body) > maxBodyLen {
		return nil, ErrBodyTooBig
	}
	p.Header.Length = uint32(len(p.Body))

	hdr := p.Header.MarshalBinary()
	pkt := make([]byte, hdrLen+len(p.Body))
	copy(pkt[:hdrLen], hdr)
	copy(pkt[hdrLen:], p.Body)

	if len(key) > 0 {
		Encrypt(pkt[hdrLen:], p.Header.SessionID, key, p.Header.Version, p.Header.SeqNo)
	}
	return pkt, nil
}

// UnmarshalPacket decodes a packet from wire bytes and decrypts the body if key is non-empty.
func UnmarshalPacket(data, key []byte) (*Packet, error) {
	hdr, err := UnmarshalPacketHeader(data)
	if err != nil {
		return nil, err
	}
	bodyLen := int(hdr.Length)
	if len(data) < hdrLen+bodyLen {
		return nil, fmt.Errorf("packet truncated: need %d bytes, have %d", hdrLen+bodyLen, len(data))
	}
	if bodyLen > maxBodyLen {
		return nil, ErrBodyTooBig
	}

	body := make([]byte, bodyLen)
	copy(body, data[hdrLen:hdrLen+bodyLen])

	if len(key) > 0 && hdr.Flags&flagUnencrypted == 0 {
		Encrypt(body, hdr.SessionID, key, hdr.Version, hdr.SeqNo)
	}

	return &Packet{Header: hdr, Body: body}, nil
}
