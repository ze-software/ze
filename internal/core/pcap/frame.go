// Design: docs/architecture/diagnostics/packet-capture.md -- synthetic IP and TCP framing
// Overview: pcap.go -- the file format these frames are written into

package pcap

import (
	"encoding/binary"
	"fmt"
	"io"
	"net/netip"
	"time"
)

// The header sizes this file writes. No option is ever emitted, so each one is
// the minimum its RFC defines.
const (
	// IPv4HeaderLen is the 20-octet IPv4 header of RFC 791 with no options.
	IPv4HeaderLen = 20
	// IPv6HeaderLen is the 40-octet IPv6 header of RFC 8200 with no extension
	// headers.
	IPv6HeaderLen = 40
	// TCPHeaderLen is the 20-octet TCP header of RFC 9293 with no options.
	TCPHeaderLen = 20
)

// PayloadMax bounds one framed payload. The largest thing Ze frames is one BGP
// message, and RFC 8654 caps that at 65535 octets.
const PayloadMax = 65535

// frameMax is the largest frame WriteMessage can produce: the wider of the two
// IP headers, a TCP header, and a full payload.
const frameMax = IPv6HeaderLen + TCPHeaderLen + PayloadMax

// protocolTCP is the IP protocol number of TCP (RFC 9293 Section 3.1).
const protocolTCP = 6

// sequenceInitial is the sequence number each direction of a synthesized flow
// starts at. A real TCP initial sequence number is random; a fixed one makes a
// generated capture reproducible byte for byte, which the tests depend on, and
// a reader only ever compares sequence numbers with each other.
const sequenceInitial uint32 = 1

// Flow is one direction of a TCP conversation: who sent, who received, and on
// which ports. The reverse direction is a second Flow, and Reverse builds it.
type Flow struct {
	SourceAddr netip.Addr
	TargetAddr netip.Addr
	SourcePort uint16
	TargetPort uint16
}

// Reverse returns the Flow carrying the opposite direction of the same
// conversation.
func (f Flow) Reverse() Flow {
	return Flow{
		SourceAddr: f.TargetAddr,
		TargetAddr: f.SourceAddr,
		SourcePort: f.TargetPort,
		TargetPort: f.SourcePort,
	}
}

// String renders the flow as "source:port -> target:port", for a human reading
// a report. Never compare two flows with it; compare the struct.
func (f Flow) String() string {
	return netip.AddrPortFrom(f.SourceAddr, f.SourcePort).String() +
		" -> " + netip.AddrPortFrom(f.TargetAddr, f.TargetPort).String()
}

// Framer writes payloads as pcap records wrapped in a synthetic IP and TCP
// header, under LinkTypeRaw. It holds one sequence number for each direction it
// has seen, so consecutive records in one direction advance by the preceding
// payload's length and a reader treats none of them as a retransmission.
//
// The ports and addresses come from the caller. Ze fabricates the ports on the
// capture path, because reading the real ones costs two mutexes on the session
// read path; a generated capture is a readable reconstruction, not a
// packet-level record of what crossed the wire.
//
// Not safe for concurrent use: one export owns one Framer.
type Framer struct {
	sequences map[Flow]uint32
	buf       []byte
}

// NewFramer returns a Framer with its scratch buffer already allocated, so
// writing a record allocates nothing.
func NewFramer() *Framer {
	return &Framer{
		sequences: make(map[Flow]uint32),
		buf:       make([]byte, frameMax),
	}
}

// sequence returns the next sequence number for flow, starting the direction at
// sequenceInitial the first time it is seen.
func (f *Framer) sequence(flow Flow) uint32 {
	seq, seen := f.sequences[flow]
	if !seen {
		seq = sequenceInitial
		f.sequences[flow] = seq
	}
	return seq
}

// WriteMessage writes one payload as a pcap record holding an IP header, a TCP
// header and the payload.
//
// originalLen is the payload's length on the network. It exceeds len(payload)
// when the capture ring truncated the message: the IP length field and the
// record's original length then both declare the true size, so a reader shows
// truncation rather than a malformed packet, and the sequence number of the
// next record in the same direction advances by the true size.
//
// The IP header checksum always covers the bytes written. The TCP checksum
// covers the pseudo-header, the TCP header and the payload bytes present; for a
// truncated record the missing tail cannot be checksummed and no reader
// verifies a truncated segment.
func (f *Framer) WriteMessage(w io.Writer, ts time.Time, flow Flow, payload []byte, originalLen int) error {
	if originalLen < len(payload) {
		return fmt.Errorf("pcap: original length %d is shorter than the %d payload bytes", originalLen, len(payload))
	}
	if originalLen > PayloadMax {
		return fmt.Errorf("pcap: payload of %d bytes exceeds the %d-byte maximum", originalLen, PayloadMax)
	}
	if !flow.SourceAddr.IsValid() || !flow.TargetAddr.IsValid() {
		return fmt.Errorf("pcap: flow %s carries an invalid address", flow)
	}
	if flow.SourceAddr.Is4() != flow.TargetAddr.Is4() {
		return fmt.Errorf("pcap: flow %s mixes an IPv4 and an IPv6 address", flow)
	}

	sequence := f.sequence(flow)
	acknowledgement := f.sequence(flow.Reverse())

	ipLen := IPv6HeaderLen
	if flow.SourceAddr.Is4() {
		ipLen = IPv4HeaderLen
	}
	captured := ipLen + TCPHeaderLen + len(payload)
	original := ipLen + TCPHeaderLen + originalLen

	buf := f.buf[:captured]
	clear(buf)
	if flow.SourceAddr.Is4() {
		writeIPv4Header(buf, flow, original)
	} else {
		writeIPv6Header(buf, flow, TCPHeaderLen+originalLen)
	}
	writeTCPHeader(buf[ipLen:], flow, sequence, acknowledgement)
	copy(buf[ipLen+TCPHeaderLen:], payload)
	writeTCPChecksum(buf, flow, ipLen, TCPHeaderLen+originalLen)

	// The next record in this direction starts after every byte this one
	// carried on the wire, truncated tail included. Advancing by the captured
	// length instead would make the following record look like a retransmission.
	f.sequences[flow] = sequence + uint32(originalLen) //nolint:gosec // bounded by PayloadMax above

	return WriteRecord(w, ts, buf, original)
}

// writeIPv4Header writes the 20-octet IPv4 header of RFC 791 Section 3.1 into
// buf, including its header checksum. totalLen is the length of the whole IP
// packet as it was on the network.
func writeIPv4Header(buf []byte, flow Flow, totalLen int) {
	source := flow.SourceAddr.As4()
	target := flow.TargetAddr.As4()

	buf[0] = 0x45 // Version 4, IHL 5 words.
	binary.BigEndian.PutUint16(buf[2:4], uint16(totalLen))
	binary.BigEndian.PutUint16(buf[6:8], 0x4000) // Don't Fragment, as a BGP session sets.
	buf[8] = 64                                  // TTL.
	buf[9] = protocolTCP
	copy(buf[12:16], source[:])
	copy(buf[16:20], target[:])
	binary.BigEndian.PutUint16(buf[10:12], checksum(buf[:IPv4HeaderLen]))
}

// writeIPv6Header writes the 40-octet IPv6 header of RFC 8200 Section 3 into
// buf. payloadLen is the length after this header, which IPv6 carries instead
// of a total length, and IPv6 has no header checksum.
func writeIPv6Header(buf []byte, flow Flow, payloadLen int) {
	source := flow.SourceAddr.As16()
	target := flow.TargetAddr.As16()

	buf[0] = 0x60 // Version 6, traffic class 0, flow label 0.
	binary.BigEndian.PutUint16(buf[4:6], uint16(payloadLen))
	buf[6] = protocolTCP // Next Header.
	buf[7] = 64          // Hop Limit.
	copy(buf[8:24], source[:])
	copy(buf[24:40], target[:])
}

// writeTCPHeader writes the 20-octet TCP header of RFC 9293 Section 3.1 into
// buf, leaving the checksum zero for writeTCPChecksum to fill.
func writeTCPHeader(buf []byte, flow Flow, sequence, acknowledgement uint32) {
	binary.BigEndian.PutUint16(buf[0:2], flow.SourcePort)
	binary.BigEndian.PutUint16(buf[2:4], flow.TargetPort)
	binary.BigEndian.PutUint32(buf[4:8], sequence)
	binary.BigEndian.PutUint32(buf[8:12], acknowledgement)
	buf[12] = 0x50                                 // Data offset 5 words, no options.
	buf[13] = 0x18                                 // PSH and ACK, which a BGP message carries.
	binary.BigEndian.PutUint16(buf[14:16], 0xFFFF) // Window, the largest a header without scaling states.
}

// writeTCPChecksum computes the TCP checksum of RFC 9293 Section 3.1 over the
// pseudo-header, the TCP header and the payload present, and writes it into the
// TCP header. tcpLen is the TCP length the pseudo-header declares, which is the
// on-network length rather than the captured one.
func writeTCPChecksum(buf []byte, flow Flow, ipLen, tcpLen int) {
	var pseudo [40]byte
	var pseudoLen int
	if flow.SourceAddr.Is4() {
		source := flow.SourceAddr.As4()
		target := flow.TargetAddr.As4()
		copy(pseudo[0:4], source[:])
		copy(pseudo[4:8], target[:])
		pseudo[9] = protocolTCP
		binary.BigEndian.PutUint16(pseudo[10:12], uint16(tcpLen))
		pseudoLen = 12
	} else {
		source := flow.SourceAddr.As16()
		target := flow.TargetAddr.As16()
		copy(pseudo[0:16], source[:])
		copy(pseudo[16:32], target[:])
		binary.BigEndian.PutUint32(pseudo[32:36], uint32(tcpLen))
		pseudo[39] = protocolTCP
		pseudoLen = 40
	}

	sum := partialSum(pseudo[:pseudoLen], 0)
	sum = partialSum(buf[ipLen:], sum)
	binary.BigEndian.PutUint16(buf[ipLen+16:ipLen+18], foldChecksum(sum))
}

// checksum returns the internet checksum of RFC 1071 over data.
func checksum(data []byte) uint16 {
	return foldChecksum(partialSum(data, 0))
}

// partialSum adds data to an in-progress internet checksum. An odd final byte
// is padded with a zero on the right, as RFC 1071 Section 1 requires.
func partialSum(data []byte, sum uint32) uint32 {
	for i := 0; i+1 < len(data); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(data[i : i+2]))
	}
	if len(data)%2 == 1 {
		sum += uint32(data[len(data)-1]) << 8
	}
	return sum
}

// foldChecksum folds the carries out of a partial sum and complements it, which
// is the last step of RFC 1071 Section 1.
func foldChecksum(sum uint32) uint16 {
	for sum>>16 != 0 {
		sum = (sum & 0xFFFF) + (sum >> 16)
	}
	return ^uint16(sum) //nolint:gosec // folded to 16 bits above
}
