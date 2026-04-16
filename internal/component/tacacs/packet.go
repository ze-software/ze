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

	// PacketHeader flags -- both exported so test mocks and external
	// tooling that emit or parse TACACS+ wire bytes can reference them.

	// FlagUnencrypted (TAC_PLUS_UNENCRYPTED_FLAG, RFC 8907 §4.5): when set,
	// the body is NOT obfuscated with the MD5 pseudo-pad. Ze never emits
	// this flag. On receive, a set flag disables the XOR step.
	FlagUnencrypted = 0x01
	// FlagSingleConnect (TAC_PLUS_SINGLE_CONNECT_FLAG, RFC 8907 §4.5): the
	// client sets this on the first packet of a TCP connection to signal it
	// can reuse the TCP across sessions; the server echoes it on its reply
	// if it supports single-connect. If set in both directions, subsequent
	// sessions share the TCP connection.
	FlagSingleConnect = 0x04

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
// Retained for tests; production paths use MarshalInto to write directly
// into a pooled wire buffer.
func (h *PacketHeader) MarshalBinary() []byte {
	buf := make([]byte, hdrLen)
	h.MarshalInto(buf)
	return buf
}

// MarshalInto writes the 12-byte header into dst in network byte order.
// dst MUST have at least hdrLen bytes available; the caller normally
// passes `buf[:hdrLen]` where `buf` is a pool buffer whose body slot
// `buf[hdrLen:]` has already been filled by a request type's
// MarshalBinaryInto. Returns hdrLen for symmetry with body writers.
func (h *PacketHeader) MarshalInto(dst []byte) int {
	_ = dst[hdrLen-1] // bounds check hint for the compiler
	dst[0] = h.Version
	dst[1] = h.Type
	dst[2] = h.SeqNo
	dst[3] = h.Flags
	binary.BigEndian.PutUint32(dst[4:8], h.SessionID)
	binary.BigEndian.PutUint32(dst[8:12], h.Length)
	return hdrLen
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

	// Build the initial MD5 input: session_id(4) + key(N) + version(1) + seq_no(1).
	// TACACS+ shared secrets are typically 16-64 bytes, so the 256-byte stack
	// scratch covers every realistic key. Larger keys fall back to the heap.
	inputLen := 4 + len(key) + 2
	var stack [256]byte
	var input []byte
	if inputLen <= len(stack) {
		input = stack[:inputLen]
	} else {
		input = make([]byte, inputLen)
	}
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

// MarshalInto writes the packet's wire bytes into the provided buffer.
// Returns the number of bytes written. The caller MUST provide a buffer
// at least `hdrLen + len(p.Body)` long; a pooled buffer of poolBufSize
// (65547) is always large enough because maxBodyLen bounds p.Body.
//
// This replaces the old `Marshal() []byte`: callers that used to let the
// implementation allocate now supply a pool buffer, eliminating the
// runtime `make([]byte, N)` on every send path.
func (p *Packet) MarshalInto(buf, key []byte) (int, error) {
	if len(p.Body) > maxBodyLen {
		return 0, ErrBodyTooBig
	}
	need := hdrLen + len(p.Body)
	if len(buf) < need {
		return 0, fmt.Errorf("marshal buffer too small: need %d, have %d", need, len(buf))
	}
	p.Header.Length = uint32(len(p.Body))

	// Inline the header write so no temporary slice is allocated.
	buf[0] = p.Header.Version
	buf[1] = p.Header.Type
	buf[2] = p.Header.SeqNo
	buf[3] = p.Header.Flags
	binary.BigEndian.PutUint32(buf[4:8], p.Header.SessionID)
	binary.BigEndian.PutUint32(buf[8:12], p.Header.Length)

	copy(buf[hdrLen:need], p.Body)
	if len(key) > 0 {
		Encrypt(buf[hdrLen:need], p.Header.SessionID, key, p.Header.Version, p.Header.SeqNo)
	}
	return need, nil
}

// Marshal encodes a packet to wire format with optional encryption and
// returns a freshly-allocated slice. Retained only for round-trip unit
// tests; production code paths use MarshalInto with a pooled buffer.
func (p *Packet) Marshal(key []byte) ([]byte, error) {
	if len(p.Body) > maxBodyLen {
		return nil, ErrBodyTooBig
	}
	wire := make([]byte, hdrLen+len(p.Body))
	n, err := p.MarshalInto(wire, key)
	if err != nil {
		return nil, err
	}
	return wire[:n], nil
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

	// UnmarshalPacket is only used by round-trip and fuzz tests. The
	// production receive path in client.go/trySend reads into the pooled
	// buffer directly and calls UnmarshalPacketHeader on header bytes.
	// Copying here keeps the decrypt-in-place step from mutating the
	// caller's input slice, which would be a surprising side-effect for
	// a function named Unmarshal. Since this is never on the hot path,
	// the bounded allocation is acceptable.
	body := make([]byte, bodyLen)
	copy(body, data[hdrLen:hdrLen+bodyLen])

	if len(key) > 0 && hdr.Flags&FlagUnencrypted == 0 {
		Encrypt(body, hdr.SessionID, key, hdr.Version, hdr.SeqNo)
	}

	return &Packet{Header: hdr, Body: body}, nil
}
