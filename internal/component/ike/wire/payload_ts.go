// Design: docs/architecture/wire/buffer-writer.md — buffer-first encoding
// RFC: rfc/short/rfc7296.md — Traffic Selector payloads (Section 3.13)
package wire

import "encoding/binary"

// Traffic selector types (RFC 7296 Section 3.13.1).
const (
	TSTypeIPv4AddrRange uint8 = 7
	TSTypeIPv6AddrRange uint8 = 8
)

// TrafficSelector is a single traffic selector sub-structure.
type TrafficSelector struct {
	TSType       uint8
	IPProtocol   uint8
	StartPort    uint16
	EndPort      uint16
	StartAddress []byte
	EndAddress   []byte
}

const tsSubHeaderLen = 4

func (ts *TrafficSelector) length() int {
	return tsSubHeaderLen + 4 + len(ts.StartAddress) + len(ts.EndAddress)
}

func (ts *TrafficSelector) WriteTo(buf []byte, off int) int {
	buf[off] = ts.TSType
	buf[off+1] = ts.IPProtocol
	slen := uint16(ts.length())
	binary.BigEndian.PutUint16(buf[off+2:], slen)
	binary.BigEndian.PutUint16(buf[off+4:], ts.StartPort)
	binary.BigEndian.PutUint16(buf[off+6:], ts.EndPort)
	n := 8
	copy(buf[off+n:], ts.StartAddress)
	n += len(ts.StartAddress)
	copy(buf[off+n:], ts.EndAddress)
	n += len(ts.EndAddress)
	return n
}

func (ts *TrafficSelector) ReadFrom(data []byte) error {
	if len(data) < 8 {
		return ErrTruncated
	}
	ts.TSType = data[0]
	ts.IPProtocol = data[1]
	slen := int(binary.BigEndian.Uint16(data[2:4]))
	if slen > len(data) || slen < 8 {
		return ErrTruncated
	}
	ts.StartPort = binary.BigEndian.Uint16(data[4:6])
	ts.EndPort = binary.BigEndian.Uint16(data[6:8])
	addrData := data[8:slen]
	if len(addrData) == 0 || len(addrData)%2 != 0 {
		return ErrTruncated
	}
	addrLen := len(addrData) / 2
	// RFC 7296 Section 3.13.1: IPv4 addresses are 4 bytes, IPv6 are 16 bytes.
	switch ts.TSType {
	case TSTypeIPv4AddrRange:
		if addrLen != 4 {
			return ErrTruncated
		}
	case TSTypeIPv6AddrRange:
		if addrLen != 16 {
			return ErrTruncated
		}
	}
	ts.StartAddress = make([]byte, addrLen)
	ts.EndAddress = make([]byte, addrLen)
	copy(ts.StartAddress, addrData[:addrLen])
	copy(ts.EndAddress, addrData[addrLen:])
	return nil
}

// PayloadTS is the Traffic Selector payload (types 44 TSi, 45 TSr).
type PayloadTS struct {
	TSPayloadType    uint8
	TrafficSelectors []TrafficSelector
}

func (p *PayloadTS) Type() uint8 { return p.TSPayloadType }

func (p *PayloadTS) WriteTo(buf []byte, off int) int {
	buf[off] = byte(len(p.TrafficSelectors))
	buf[off+1] = 0 // reserved
	buf[off+2] = 0
	buf[off+3] = 0
	n := 4
	for i := range p.TrafficSelectors {
		n += p.TrafficSelectors[i].WriteTo(buf, off+n)
	}
	return n
}

func (p *PayloadTS) Len() int {
	n := 4
	for i := range p.TrafficSelectors {
		n += p.TrafficSelectors[i].length()
	}
	return n
}

func (p *PayloadTS) ReadFrom(data []byte) error {
	if len(data) < 4 {
		return ErrTruncated
	}
	numTS := int(data[0])
	if numTS == 0 {
		return ErrNoTrafficSelector
	}
	p.TrafficSelectors = make([]TrafficSelector, 0, numTS)
	off := 4
	for range numTS {
		if off+8 > len(data) {
			return ErrTruncated
		}
		var ts TrafficSelector
		if err := ts.ReadFrom(data[off:]); err != nil {
			return err
		}
		slen := int(binary.BigEndian.Uint16(data[off+2 : off+4]))
		p.TrafficSelectors = append(p.TrafficSelectors, ts)
		off += slen
	}
	return nil
}
