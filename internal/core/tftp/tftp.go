// Design: docs/architecture/provisioning/tftp-server.md -- the TFTP wire facts the server and the client share
// Related: oack.go -- the client's OACK check
// Related: request.go -- the client's RRQ and ACK encoders

// Package tftp holds the TFTP wire facts that Ze's server
// (internal/plugins/tftpserver) and Ze's client (the tftp scheme of
// internal/core/fetch) both use: opcodes, error codes, the option names and
// their ranges, and the encoders and parsers of the packets each side writes
// or reads. It is a core leaf so that the client, which core holds, and the
// server, a plugin, share one declaration.
package tftp

import "encoding/binary"

// RFC 1350 Section 5: "TFTP supports five types of packets".
// RFC 2347 Packet Formats: OACK is opcode 6.
const (
	OpRRQ   uint16 = 1
	OpWRQ   uint16 = 2
	OpDATA  uint16 = 3
	OpACK   uint16 = 4
	OpERROR uint16 = 5
	OpOACK  uint16 = 6
)

// RFC 1350 Appendix, Error Codes, and RFC 2347 Packet Formats, which adds
// code 8 to terminate a transfer over option negotiation.
const (
	CodeNotDefined        uint16 = 0
	CodeFileNotFound      uint16 = 1
	CodeAccessViolation   uint16 = 2
	CodeDiskFull          uint16 = 3
	CodeIllegalOperation  uint16 = 4
	CodeUnknownTID        uint16 = 5
	CodeOptionNegotiation uint16 = 8
)

// HeaderOctets is the opcode and block number in front of a DATA payload, and
// the whole of an ACK (RFC 1350 Section 5).
const HeaderOctets = 4

// DefaultBlockSize is the block size when no blksize was acknowledged.
// RFC 1350 Section 2: "the file is sent in fixed length blocks of 512 bytes".
const DefaultBlockSize = 512

// RFC 2348 Blocksize Option Specification: "Valid values range between "8"
// and "65464" octets, inclusive."
const (
	BlksizeMin = 8
	BlksizeMax = 65464
)

// BlksizeEthernet is the largest block one untagged Ethernet frame carries
// (1500 MTU - 20 IP - 8 UDP - 4 TFTP header). The server never acknowledges a
// larger one and the client never asks for a larger one.
const BlksizeEthernet = 1468

// RFC 7440 Section 3: "The valid values range MUST be between 1 and 65535
// blocks, inclusive."
const (
	WindowsizeMin = 1
	WindowsizeMax = 65535
)

// RFC 2349 Timeout Interval Option Specification: "Valid values range between
// "1" and "255" seconds, inclusive."
const (
	TimeoutMin = 1
	TimeoutMax = 255
)

// RequestOctetsMax bounds an RRQ. RFC 2347 Packet Formats: "The maximum size
// of a request packet is 512 octets."
const RequestOctetsMax = 512

// The option names. Option names are case insensitive (RFC 2347 Packet
// Formats), so a reader compares them with strings.EqualFold.
const (
	OptionBlksize    = "blksize"
	OptionTimeout    = "timeout"
	OptionTsize      = "tsize"
	OptionWindowsize = "windowsize"
)

// ModeOctet is the only transfer mode Ze speaks (RFC 1350 Section 1).
const ModeOctet = "octet"

// BuildError returns an ERROR packet carrying code and msg.
//
// RFC 1350 Section 5: "An ERROR packet (opcode 5) takes the form depicted in
// Figure 5-4."
//
//	 2 bytes     2 bytes      string    1 byte
//	-----------------------------------------
//	| Opcode |  ErrorCode |   ErrMsg   |   0  |
//	-----------------------------------------
func BuildError(code uint16, msg string) []byte {
	pkt := make([]byte, 5+len(msg))
	binary.BigEndian.PutUint16(pkt[0:2], OpERROR)
	binary.BigEndian.PutUint16(pkt[2:4], code)
	copy(pkt[4:], msg)
	pkt[4+len(msg)] = 0
	return pkt
}
