// Design: docs/architecture/diagnostics/packet-capture.md -- pcap writer (stdlib-only)

package cmd

import (
	"encoding/binary"
	"io"
	"time"
)

const (
	pcapMagic      = 0xa1b2c3d4
	pcapVersionMaj = 2
	pcapVersionMin = 4

	// LinkTypeRaw is LINKTYPE_RAW (raw IPv4/IPv6). Used for L2TP and BGP
	// captures so tools like Wireshark can display the packets.
	LinkTypeRaw uint32 = 101
)

func writePcapHeader(w io.Writer, snapLen, linkType uint32) error { //nolint:unparam // linkType varies by protocol
	var buf [24]byte
	binary.LittleEndian.PutUint32(buf[0:4], pcapMagic)
	binary.LittleEndian.PutUint16(buf[4:6], pcapVersionMaj)
	binary.LittleEndian.PutUint16(buf[6:8], pcapVersionMin)
	binary.LittleEndian.PutUint32(buf[16:20], snapLen)
	binary.LittleEndian.PutUint32(buf[20:24], linkType)
	_, err := w.Write(buf[:])
	return err
}

func writePcapPacket(w io.Writer, ts time.Time, data []byte) error {
	var hdr [16]byte
	binary.LittleEndian.PutUint32(hdr[0:4], uint32(ts.Unix()))
	binary.LittleEndian.PutUint32(hdr[4:8], uint32(ts.Nanosecond()/1000))
	binary.LittleEndian.PutUint32(hdr[8:12], uint32(len(data)))
	binary.LittleEndian.PutUint32(hdr[12:16], uint32(len(data)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}
