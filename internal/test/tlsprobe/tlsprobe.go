// Design: docs/architecture/dns/secure-transports.md -- BCP 195 cipher-suite probe
// Related: bcp195.go -- the RFC 9325 section 4.1 suite tables and the assertion

// Package tlsprobe offers a hand-built TLS 1.2 ClientHello to a listener and
// reports how the server answered. Go's crypto/tls client cannot offer a suite
// it does not implement (NULL, export, single DES, static ECDH), so a test that
// must prove a server refuses one of those writes the ClientHello itself.
//
// The probe speaks only the first flight: it sends one ClientHello record and
// reads one record back. It never completes a handshake.
package tlsprobe

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

// Outcome names what the server sent in reply to the ClientHello.
type Outcome uint8

// The outcomes of one probe. Zero is Unspecified, so an Answer nobody filled
// is never read as a refusal.
const (
	OutcomeUnspecified Outcome = iota
	// OutcomeServerHello: the server selected a suite and answered ServerHello.
	OutcomeServerHello
	// OutcomeAlert: the server answered with a TLS alert record.
	OutcomeAlert
	// OutcomeClosed: the server closed the connection without a record.
	OutcomeClosed
)

// Answer is the server's reply to one probe.
type Answer struct {
	Outcome Outcome
	// Suite and Compression are the server's selections, set for OutcomeServerHello.
	Suite       uint16
	Compression uint8
	// SecureRenegotiation reports a renegotiation_info extension in the
	// ServerHello (RFC 5746), set for OutcomeServerHello.
	SecureRenegotiation bool
	// Alert is the alert description, set for OutcomeAlert.
	Alert uint8
}

// TLS record and handshake constants from RFC 5246 sections 6.2.1, 7.2 and 7.4.
const (
	recordHandshake      = 22
	recordAlert          = 21
	handshakeClientHello = 1
	handshakeServerHello = 2
	recordHeaderOctets   = 5
	recordBodyOctetsMax  = 1 << 14
	probeTimeout         = 5 * time.Second
)

// Extension type codes carried by the probe's ClientHello.
const (
	extSupportedGroups      = 0x000a
	extECPointFormats       = 0x000b
	extSignatureAlgorithms  = 0x000d
	extExtendedMasterSecret = 0x0017
	extRenegotiationInfo    = 0xff01
)

// Offer dials address over TCP, sends a TLS 1.2 ClientHello offering exactly
// suites and compression methods, and returns the server's first reply. The
// ClientHello carries the extensions a Go server needs to pick an ECDHE suite
// for an ECDSA or an RSA certificate (x25519 and P-256, uncompressed points,
// ECDSA-P256, RSA-PSS and RSA PKCS#1 v1.5 signatures), plus renegotiation_info.
// An error means the probe itself failed, never that the server refused.
func Offer(ctx context.Context, address string, suites []uint16, compression []uint8) (Answer, error) {
	hello, err := clientHello(suites, compression)
	if err != nil {
		return Answer{}, err
	}
	d := net.Dialer{Timeout: probeTimeout}
	conn, err := d.DialContext(ctx, "tcp", address)
	if err != nil {
		return Answer{}, fmt.Errorf("tlsprobe: dial %s: %w", address, err)
	}
	defer func() { _ = conn.Close() }()

	if err := conn.SetDeadline(time.Now().Add(probeTimeout)); err != nil {
		return Answer{}, fmt.Errorf("tlsprobe: deadline: %w", err)
	}
	if _, err := conn.Write(hello); err != nil {
		return Answer{}, fmt.Errorf("tlsprobe: write ClientHello: %w", err)
	}
	return readReply(conn)
}

// clientHello builds one TLS record holding the ClientHello (RFC 5246 section
// 7.4.1.2) with a random 32-octet random and an empty session id.
func clientHello(suites []uint16, compression []uint8) ([]byte, error) {
	ext := make([]byte, 0, 64)
	ext = appendExt(ext, extSupportedGroups, []byte{0x00, 0x04, 0x00, 0x1d, 0x00, 0x17})
	ext = appendExt(ext, extECPointFormats, []byte{0x01, 0x00})
	ext = appendExt(ext, extSignatureAlgorithms, []byte{0x00, 0x06, 0x04, 0x03, 0x08, 0x04, 0x04, 0x01})
	ext = appendExt(ext, extExtendedMasterSecret, nil)
	ext = appendExt(ext, extRenegotiationInfo, []byte{0x00})

	body := make([]byte, 0, 128)
	body = append(body, 0x03, 0x03)
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return nil, fmt.Errorf("tlsprobe: random: %w", err)
	}
	body = append(body, random...)
	body = append(body, 0x00)
	body = binary.BigEndian.AppendUint16(body, uint16(2*len(suites)))
	for _, suite := range suites {
		body = binary.BigEndian.AppendUint16(body, suite)
	}
	body = append(body, uint8(len(compression)))
	body = append(body, compression...)
	body = binary.BigEndian.AppendUint16(body, uint16(len(ext)))
	body = append(body, ext...)

	record := make([]byte, 0, recordHeaderOctets+4+len(body))
	record = append(record, recordHandshake, 0x03, 0x01)
	record = binary.BigEndian.AppendUint16(record, uint16(4+len(body)))
	record = append(record, handshakeClientHello, 0x00)
	record = binary.BigEndian.AppendUint16(record, uint16(len(body)))
	return append(record, body...), nil
}

func appendExt(buf []byte, kind uint16, data []byte) []byte {
	buf = binary.BigEndian.AppendUint16(buf, kind)
	buf = binary.BigEndian.AppendUint16(buf, uint16(len(data)))
	return append(buf, data...)
}

// readReply reads one record and classifies it.
func readReply(conn net.Conn) (Answer, error) {
	header := make([]byte, recordHeaderOctets)
	if _, err := io.ReadFull(conn, header); err != nil {
		if closedByPeer(err) {
			return Answer{Outcome: OutcomeClosed}, nil
		}
		return Answer{}, fmt.Errorf("tlsprobe: read record header: %w", err)
	}
	length := int(binary.BigEndian.Uint16(header[3:5]))
	if length > recordBodyOctetsMax {
		return Answer{}, fmt.Errorf("tlsprobe: record of %d octets exceeds %d", length, recordBodyOctetsMax)
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(conn, body); err != nil {
		return Answer{}, fmt.Errorf("tlsprobe: read record body: %w", err)
	}
	switch header[0] {
	case recordAlert:
		if length < 2 {
			return Answer{}, fmt.Errorf("tlsprobe: alert record of %d octets", length)
		}
		return Answer{Outcome: OutcomeAlert, Alert: body[1]}, nil
	case recordHandshake:
		return parseServerHello(body)
	}
	return Answer{}, fmt.Errorf("tlsprobe: unexpected record type %d", header[0])
}

// parseServerHello reads the selected suite, the compression method and the
// extension list of a ServerHello (RFC 5246 section 7.4.1.3).
func parseServerHello(msg []byte) (Answer, error) {
	// type(1) length(3) version(2) random(32) session_id_length(1)
	const fixedOctets = 1 + 3 + 2 + 32 + 1
	if len(msg) < fixedOctets || msg[0] != handshakeServerHello {
		return Answer{}, errors.New("tlsprobe: first handshake message is not a ServerHello")
	}
	off := fixedOctets + int(msg[fixedOctets-1])
	if len(msg) < off+3 {
		return Answer{}, errors.New("tlsprobe: ServerHello truncated before the suite")
	}
	answer := Answer{
		Outcome:     OutcomeServerHello,
		Suite:       binary.BigEndian.Uint16(msg[off : off+2]),
		Compression: msg[off+2],
	}
	off += 3
	if len(msg) < off+2 {
		return answer, nil
	}
	end := off + 2 + int(binary.BigEndian.Uint16(msg[off:off+2]))
	if end > len(msg) {
		return Answer{}, errors.New("tlsprobe: ServerHello extensions overrun the message")
	}
	for off += 2; off+4 <= end; {
		kind := binary.BigEndian.Uint16(msg[off : off+2])
		size := int(binary.BigEndian.Uint16(msg[off+2 : off+4]))
		if kind == extRenegotiationInfo {
			answer.SecureRenegotiation = true
		}
		off += 4 + size
	}
	return answer, nil
}

func closedByPeer(err error) bool {
	if errors.Is(err, io.EOF) {
		return true
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var opErr *net.OpError
	return errors.As(err, &opErr) && !opErr.Timeout()
}
