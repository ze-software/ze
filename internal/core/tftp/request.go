// Design: docs/architecture/appliance/on-device-installer.md -- the TFTP client's request and acknowledgement
// Related: tftp.go -- opcodes, option names and ranges
// Related: oack.go -- the check of the server's answer to this request

package tftp

import (
	"encoding/binary"
	"errors"
	"strconv"
	"strings"
)

// Request is the set of options a client asks for in its RRQ. A zero Blksize
// or Windowsize means the option is not requested, and false Tsize the same:
// ParseOACK reads that zero as "not requested" and refuses the option in the
// server's answer, which is the guard RFC 2347 asks for.
type Request struct {
	Blksize    int  // octets asked for, BlksizeMin..BlksizeMax, or 0
	Tsize      bool // ask for the transfer size with a value of "0"
	Windowsize int  // blocks asked for, WindowsizeMin..WindowsizeMax, or 0
}

// BuildRRQ returns the read request for filename in octet mode carrying the
// options of req.
//
// RFC 2347 Packet Formats: "The options and values are all NULL-terminated,
// in keeping with the original request format."
//
//	 2 bytes    string   1 byte  string  1 byte  string  1 byte  string  1 byte
//	| opc=1 | filename |   0  | octet |   0  |  opt1  |   0  | value1 |   0  | ...
func BuildRRQ(filename string, req Request) ([]byte, error) {
	if filename == "" {
		return nil, errors.New("the file name is empty")
	}
	// RFC 7440 Section 3: "Note that all fields except "opc" MUST be ASCII
	// strings followed by a single-byte NULL character."
	if strings.IndexByte(filename, 0) >= 0 {
		return nil, errors.New("the file name holds a NUL byte, which ends a TFTP field")
	}
	if !isASCII(filename) {
		return nil, errors.New("the file name holds a byte outside ASCII: a TFTP field is an ASCII string")
	}
	if err := req.valid(); err != nil {
		return nil, err
	}

	pkt := make([]byte, 2, RequestOctetsMax)
	binary.BigEndian.PutUint16(pkt, OpRRQ)
	pkt = appendField(pkt, filename)
	pkt = appendField(pkt, ModeOctet)
	if req.Blksize > 0 {
		pkt = appendField(pkt, OptionBlksize)
		pkt = strconv.AppendInt(pkt, int64(req.Blksize), 10)
		pkt = append(pkt, 0)
	}
	// RFC 2349 Transfer Size Option Specification: "In Read Request
	// packets, a size of "0" is specified in the request and the size of the
	// file, in octets, is returned in the OACK."
	if req.Tsize {
		pkt = appendField(pkt, OptionTsize)
		pkt = appendField(pkt, "0")
	}
	if req.Windowsize > 0 {
		pkt = appendField(pkt, OptionWindowsize)
		pkt = strconv.AppendInt(pkt, int64(req.Windowsize), 10)
		pkt = append(pkt, 0)
	}
	// RFC 2347 Packet Formats: "The maximum size of a request packet is 512
	// octets."
	if len(pkt) > RequestOctetsMax {
		return nil, errors.New("the file name is too long: the read request would exceed 512 octets")
	}
	return pkt, nil
}

// valid refuses a request whose option values are outside their RFC ranges,
// so the client never asks for what no server may grant.
func (req Request) valid() error {
	// RFC 2348 Blocksize Option Specification: "Valid values range between
	// "8" and "65464" octets, inclusive."
	if req.Blksize != 0 {
		if !inRange(req.Blksize, BlksizeMin, BlksizeMax) {
			return errors.New("blksize " + strconv.Itoa(req.Blksize) + " is outside 8..65464")
		}
	}
	// RFC 7440 Section 3: "The valid values range MUST be between 1 and 65535
	// blocks, inclusive."
	if req.Windowsize != 0 {
		if !inRange(req.Windowsize, WindowsizeMin, WindowsizeMax) {
			return errors.New("windowsize " + strconv.Itoa(req.Windowsize) + " is outside 1..65535")
		}
	}
	return nil
}

// inRange reports whether low <= value <= high.
func inRange(value, low, high int) bool {
	if value < low {
		return false
	}
	return value <= high
}

// isASCII reports whether every byte of s is below 128.
func isASCII(s string) bool {
	for i := range len(s) {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func appendField(pkt []byte, field string) []byte {
	pkt = append(pkt, field...)
	return append(pkt, 0)
}

// WriteACK writes the acknowledgement of block into buf, which holds at least
// HeaderOctets, and returns the octets written.
//
// RFC 1350 Section 5: "The block number in an ACK echoes the block number of
// the DATA packet being acknowledged."
//
//	 2 bytes     2 bytes
//	 ---------------------
//	| Opcode |   Block #  |
//	 ---------------------
func WriteACK(buf []byte, block uint16) int {
	binary.BigEndian.PutUint16(buf[0:2], OpACK)
	binary.BigEndian.PutUint16(buf[2:4], block)
	return HeaderOctets
}
