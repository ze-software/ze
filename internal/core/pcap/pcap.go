// Design: docs/architecture/diagnostics/packet-capture.md -- the one owner of the pcap file format
// Detail: frame.go -- synthetic IP and TCP headers around a payload
// Detail: reader.go -- reading a pcap file back, either endianness
// Detail: reassemble.go -- TCP stream reassembly across records
//
// Package pcap writes and reads the classic libpcap file format with the
// standard library alone, so an appliance needs no tcpdump and no libpcap.
// It knows pcap, IP and TCP. It knows no application protocol: a caller that
// frames BGP messages does that framing itself, over the byte streams
// Reassembler returns.
package pcap

import (
	"encoding/binary"
	"fmt"
	"io"
	"time"
)

// The link types this package writes or reads. A pcap file declares one of
// these in its file header, and it decides how the first byte of every record
// is read.
const (
	// LinkTypeEthernet is DLT_EN10MB: each record starts with a 14-byte
	// Ethernet header. AF_PACKET delivers frames in this shape.
	LinkTypeEthernet uint32 = 1
	// LinkTypeLinuxSLL is DLT_LINUX_SLL, the 16-byte cooked header tcpdump
	// writes when it captures on the "any" pseudo-interface.
	LinkTypeLinuxSLL uint32 = 113
	// LinkTypeRaw is DLT_RAW: each record starts with an IP header, and the
	// version nibble of its first byte says whether that header is IPv4 or
	// IPv6. One link type therefore carries both families.
	LinkTypeRaw uint32 = 101
	// LinkTypeIPv4 is DLT_IPV4: raw IPv4 only. Ze reads it and never writes
	// it, because it cannot carry an IPv6 peer.
	LinkTypeIPv4 uint32 = 228
	// LinkTypeIPv6 is DLT_IPV6: raw IPv6 only.
	LinkTypeIPv6 uint32 = 229
	// LinkTypeLinuxSLL2 is DLT_LINUX_SLL2, the 20-byte second-generation
	// cooked header of libpcap 1.11 and later.
	LinkTypeLinuxSLL2 uint32 = 276
)

// The fixed sizes of the two pcap headers, and the bound on one record.
const (
	// FileHeaderLen is the 24-byte header at the start of every pcap file.
	FileHeaderLen = 24
	// RecordHeaderLen is the 16-byte header before every record's bytes.
	RecordHeaderLen = 16
	// RecordBytesMax bounds one record read from an untrusted file. A record
	// declaring more than this is refused rather than allocated: the length
	// field is attacker-controlled, and the largest thing Ze ever writes is
	// one 65535-byte BGP message under a 40-byte IPv6 and 20-byte TCP header.
	RecordBytesMax = 1 << 18
)

// The four file magics. The first two carry microsecond timestamps and the
// last two nanosecond timestamps; each pair spells the same number in the two
// byte orders, which is how a reader learns the file's endianness.
const (
	magicMicro = 0xa1b2c3d4
	magicNano  = 0xa1b23c4d
)

// The pcap file format version this package writes. It has not changed since
// libpcap 0.4 and every reader expects it.
const (
	versionMajor = 2
	versionMinor = 4
)

// WriteFileHeader writes the 24-byte pcap file header: magic, version 2.4, a
// zero timezone and sigfigs, the snapshot length, and the link type. Timestamps
// in the records that follow are microseconds.
func WriteFileHeader(w io.Writer, snapLen, linkType uint32) error {
	var buf [FileHeaderLen]byte
	binary.LittleEndian.PutUint32(buf[0:4], magicMicro)
	binary.LittleEndian.PutUint16(buf[4:6], versionMajor)
	binary.LittleEndian.PutUint16(buf[6:8], versionMinor)
	binary.LittleEndian.PutUint32(buf[16:20], snapLen)
	binary.LittleEndian.PutUint32(buf[20:24], linkType)
	_, err := w.Write(buf[:])
	return err
}

// WriteRecord writes one record: the 16-byte record header, then data.
//
// originalLen is the length the packet had on the network, which is longer than
// data whenever the capture truncated it. A reader shows the difference as
// truncation instead of reporting a malformed packet. Pass len(data) when
// nothing was truncated.
func WriteRecord(w io.Writer, ts time.Time, data []byte, originalLen int) error {
	if originalLen < len(data) {
		return fmt.Errorf("pcap: original length %d is shorter than the %d captured bytes", originalLen, len(data))
	}

	var hdr [RecordHeaderLen]byte
	binary.LittleEndian.PutUint32(hdr[0:4], uint32(ts.Unix()))            //nolint:gosec // seconds since the epoch, the field pcap defines as 32 bits
	binary.LittleEndian.PutUint32(hdr[4:8], uint32(ts.Nanosecond()/1000)) //nolint:gosec // below one million by construction
	binary.LittleEndian.PutUint32(hdr[8:12], uint32(len(data)))           //nolint:gosec // a slice length, never negative
	binary.LittleEndian.PutUint32(hdr[12:16], uint32(originalLen))        //nolint:gosec // checked above to be at least len(data)
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}
