// Design: docs/architecture/appliance/on-device-installer.md -- the TFTP client's check of the server's OACK
// Related: tftp.go -- opcodes, option names and ranges
// Related: request.go -- the Request this check judges the answer against

package tftp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strconv"
	"strings"
)

// Negotiated is what a valid OACK grants. An option the OACK does not carry
// keeps its RFC 1350 value: DefaultBlockSize octets, a window of one block,
// and no transfer size.
type Negotiated struct {
	Blksize    int   // octets per DATA payload
	Windowsize int   // blocks the server sends before it waits for an ACK
	Tsize      int64 // the file size the server announced, valid when HasTsize
	HasTsize   bool
}

// Unnegotiated is the transfer an RRQ starts when the server answers with
// DATA rather than an OACK, so that no option was acknowledged.
//
// RFC 2347 Negotiation Protocol: "An option not acknowledged by the server
// must be ignored by the client and server as if it were never requested."
func Unnegotiated() Negotiated {
	return Negotiated{Blksize: DefaultBlockSize, Windowsize: WindowsizeMin}
}

// OptionError is an OACK the client refuses. The client answers it with an
// ERROR carrying CodeOptionNegotiation and the error's text, then ends the
// transfer.
type OptionError struct {
	Reason string
}

func (e *OptionError) Error() string { return "invalid option acknowledgement: " + e.Reason }

// ParseOACK checks the OACK pkt (opcode included) against the options req
// asked for and returns what it grants. It refuses an option the client did
// not request, an option named twice, a field that is not NUL-terminated, a
// value that is not a decimal number, a blksize outside 8..req.Blksize and a
// windowsize outside 1..req.Windowsize, each as an *OptionError.
//
// RFC 2347 Negotiation Protocol: "If the client rejects the OACK, then it
// sends an ERROR packet, with error code 8, to the server and the transfer is
// terminated."
//
//	 2 bytes   string  1 byte  string  1 byte
//	| opc=6 |  opt1  |   0  | value1 |   0  | ...
func ParseOACK(pkt []byte, req Request) (Negotiated, error) {
	granted := Unnegotiated()
	if len(pkt) < 2 {
		return granted, &OptionError{Reason: "the packet is shorter than its opcode"}
	}
	if binary.BigEndian.Uint16(pkt[:2]) != OpOACK {
		return granted, errors.New("BUG: ParseOACK called on a packet that is not an OACK")
	}

	var seen seenOptions
	rest := pkt[2:]
	for len(rest) > 0 {
		name, value, remaining, err := nextPair(rest)
		if err != nil {
			return granted, err
		}
		rest = remaining
		if err := seen.mark(name); err != nil {
			return granted, err
		}
		if err := grant(&granted, name, value, req); err != nil {
			return granted, err
		}
	}
	return granted, nil
}

// grant applies one acknowledged option to granted.
func grant(granted *Negotiated, name, value string, req Request) error {
	switch {
	case strings.EqualFold(name, OptionBlksize):
		return grantBlksize(granted, value, req)
	case strings.EqualFold(name, OptionTsize):
		return grantTsize(granted, value, req)
	case strings.EqualFold(name, OptionWindowsize):
		return grantWindowsize(granted, value, req)
	}
	// RFC 2347 Negotiation Protocol: "If the client receives an OACK
	// containing an unrequested option, it should respond with an ERROR
	// packet, with error code 8, and terminate the transfer."
	return &OptionError{Reason: "option " + strconv.Quote(name) + " was not requested"}
}

// grantBlksize takes an acknowledged blksize.
//
// RFC 2348 Blocksize Option Specification: "The client must then either use
// the size specified in the OACK, or send an ERROR packet, with error code 8,
// to terminate the transfer."
func grantBlksize(granted *Negotiated, value string, req Request) error {
	// RFC 2347 Negotiation Protocol: "The server must not include in the
	// OACK any option which had not been specifically requested by the
	// client; that is, only the client may initiate option negotiation."
	if req.Blksize == 0 {
		return &OptionError{Reason: "option blksize was not requested"}
	}
	size, err := ParseDecimal(OptionBlksize, value)
	if err != nil {
		return err
	}
	// RFC 2348 Blocksize Option Specification: "The specified value must be
	// less than or equal to the value specified by the client."
	if size > int64(req.Blksize) {
		return &OptionError{Reason: "blksize " + value + " is above the " + strconv.Itoa(req.Blksize) + " requested"}
	}
	if size < BlksizeMin {
		return &OptionError{Reason: "blksize " + value + " is below the minimum of 8"}
	}
	granted.Blksize = int(size)
	return nil
}

// grantTsize takes an acknowledged transfer size.
//
// RFC 2349 Transfer Size Option Specification: "In Read Request packets, a
// size of "0" is specified in the request and the size of the file, in
// octets, is returned in the OACK."
func grantTsize(granted *Negotiated, value string, req Request) error {
	if !req.Tsize {
		return &OptionError{Reason: "option tsize was not requested"}
	}
	size, err := ParseDecimal(OptionTsize, value)
	if err != nil {
		return err
	}
	granted.Tsize = size
	granted.HasTsize = true
	return nil
}

// grantWindowsize takes an acknowledged windowsize.
//
// RFC 7440 Section 3: "The client MUST then either use the size specified in
// the OACK or send an ERROR packet, with error code 8, to terminate the
// transfer."
func grantWindowsize(granted *Negotiated, value string, req Request) error {
	if req.Windowsize == 0 {
		return &OptionError{Reason: "option windowsize was not requested"}
	}
	blocks, err := ParseDecimal(OptionWindowsize, value)
	if err != nil {
		return err
	}
	// RFC 7440 Section 3: "The valid values range MUST be between 1 and
	// 65535 blocks, inclusive."
	if blocks < WindowsizeMin {
		return &OptionError{Reason: "windowsize " + value + " is below the minimum of 1"}
	}
	// RFC 7440 Section 3: "The specified value MUST be less than or equal to
	// the value specified by the client."
	if blocks > int64(req.Windowsize) {
		return &OptionError{Reason: "windowsize " + value + " is above the " + strconv.Itoa(req.Windowsize) + " requested"}
	}
	granted.Windowsize = int(blocks)
	return nil
}

// ParseDecimal reads the value of option name written as a plain decimal
// number: one or more ASCII digits, no sign, no space. It refuses anything
// else, and a number too large for int64, as an *OptionError. The client
// reads an OACK value with it and the server an RRQ value, so the two sides
// read numbers one way.
//
// RFC 2348 Blocksize Option Specification and RFC 2349 Timeout Interval
// Option Specification give each value as a number "specified in ASCII";
// RFC 7440 Section 3: "The base-10 ASCII string representation of the number
// of blocks in a window."
func ParseDecimal(name, value string) (int64, error) {
	if value == "" {
		return 0, &OptionError{Reason: "option " + name + " has an empty value"}
	}
	for i := range len(value) {
		if !isDigit(value[i]) {
			return 0, &OptionError{Reason: "option " + name + " value " + strconv.Quote(value) + " is not a decimal number"}
		}
	}
	number, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, &OptionError{Reason: "option " + name + " value " + value + " is out of range"}
	}
	return number, nil
}

// isDigit reports whether c is an ASCII decimal digit.
func isDigit(c byte) bool {
	if c < '0' {
		return false
	}
	return c <= '9'
}

// nextPair cuts one NUL-terminated name and value off the front of rest.
func nextPair(rest []byte) (name, value string, remaining []byte, err error) {
	nameEnd := bytes.IndexByte(rest, 0)
	if nameEnd < 0 {
		return "", "", nil, &OptionError{Reason: "an option name is not NUL-terminated"}
	}
	if nameEnd == 0 {
		return "", "", nil, &OptionError{Reason: "an option name is empty"}
	}
	name = string(rest[:nameEnd])
	rest = rest[nameEnd+1:]
	valueEnd := bytes.IndexByte(rest, 0)
	if valueEnd < 0 {
		return "", "", nil, &OptionError{Reason: "the value of option " + strconv.Quote(name) + " is not NUL-terminated"}
	}
	return name, string(rest[:valueEnd]), rest[valueEnd+1:], nil
}

// seenOptions refuses an option the OACK names twice.
type seenOptions struct {
	names []string
}

func (s *seenOptions) mark(name string) error {
	for _, earlier := range s.names {
		if strings.EqualFold(earlier, name) {
			return &OptionError{Reason: "option " + strconv.Quote(name) + " appears twice"}
		}
	}
	s.names = append(s.names, name)
	return nil
}
