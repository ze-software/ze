// Design: docs/architecture/mrt.md — wire format encoding

package mrt

import "encoding/binary"

var be = binary.BigEndian

func WriteCommonHeader(buf []byte, off int, timestamp uint32, typ, subtype uint16, msgLen uint32) int {
	be.PutUint32(buf[off:], timestamp)
	be.PutUint16(buf[off+4:], typ)
	be.PutUint16(buf[off+6:], subtype)
	be.PutUint32(buf[off+8:], msgLen)
	return CommonHeaderLen
}

func WriteExtendedHeader(buf []byte, off int, timestamp, microsecond uint32, typ, subtype uint16, msgLen uint32) int {
	n := WriteCommonHeader(buf, off, timestamp, typ, subtype, msgLen)
	be.PutUint32(buf[off+n:], microsecond)
	return n + ExtTimestampLen
}

func WritePeerEntry(buf []byte, off int, p *PeerEntry) int {
	start := off
	buf[off] = p.Type
	off++
	copy(buf[off:], p.BGPID[:])
	off += 4
	if p.IsIPv6() {
		off += copyIPField(buf[off:], p.IP, 16)
	} else {
		off += copyIPField(buf[off:], p.IP, 4)
	}
	if p.IsAS4() {
		be.PutUint32(buf[off:], p.ASN)
		off += 4
	} else {
		be.PutUint16(buf[off:], uint16(p.ASN))
		off += 2
	}
	return off - start
}

func WritePeerIndexTable(buf []byte, off int, collectorBGPID [4]byte, viewName string, peers []PeerEntry) int {
	start := off
	copy(buf[off:], collectorBGPID[:])
	off += 4
	be.PutUint16(buf[off:], uint16(len(viewName)))
	off += 2
	off += copy(buf[off:], viewName)
	be.PutUint16(buf[off:], uint16(len(peers)))
	off += 2
	for i := range peers {
		off += WritePeerEntry(buf, off, &peers[i])
	}
	return off - start
}

func prefixBytes(prefixLen uint8) int {
	return (int(prefixLen) + 7) / 8
}

func WriteRIBHeader(buf []byte, off int, seq uint32, prefixLen uint8, prefix []byte) int {
	start := off
	be.PutUint32(buf[off:], seq)
	off += 4
	buf[off] = prefixLen
	off++
	n := prefixBytes(prefixLen)
	copy(buf[off:], prefix[:n])
	off += n
	return off - start
}

func WriteRIBGenericHeader(buf []byte, off int, seq uint32, afi uint16, safi uint8, nlri []byte) int {
	start := off
	be.PutUint32(buf[off:], seq)
	off += 4
	be.PutUint16(buf[off:], afi)
	off += 2
	buf[off] = safi
	off++
	copy(buf[off:], nlri)
	off += len(nlri)
	return off - start
}

func WriteRIBEntry(buf []byte, off int, e *RIBEntry) int {
	start := off
	be.PutUint16(buf[off:], e.PeerIndex)
	off += 2
	be.PutUint32(buf[off:], e.OrigTime)
	off += 4
	be.PutUint16(buf[off:], uint16(len(e.Attributes)))
	off += 2
	copy(buf[off:], e.Attributes)
	off += len(e.Attributes)
	return off - start
}

func WriteRIBEntryAddPath(buf []byte, off int, e *RIBEntry) int {
	start := off
	be.PutUint16(buf[off:], e.PeerIndex)
	off += 2
	be.PutUint32(buf[off:], e.OrigTime)
	off += 4
	be.PutUint32(buf[off:], e.PathID)
	off += 4
	be.PutUint16(buf[off:], uint16(len(e.Attributes)))
	off += 2
	copy(buf[off:], e.Attributes)
	off += len(e.Attributes)
	return off - start
}

func WriteRIBEntries(buf []byte, off int, entries []RIBEntry, addPath bool) int {
	start := off
	be.PutUint16(buf[off:], uint16(len(entries)))
	off += 2
	if addPath {
		for i := range entries {
			off += WriteRIBEntryAddPath(buf, off, &entries[i])
		}
	} else {
		for i := range entries {
			off += WriteRIBEntry(buf, off, &entries[i])
		}
	}
	return off - start
}

func writeBGP4MPCommon(buf []byte, off int, h *BGP4MPHeader, as4 bool) int {
	start := off
	if as4 {
		be.PutUint32(buf[off:], h.PeerAS)
		off += 4
		be.PutUint32(buf[off:], h.LocalAS)
		off += 4
	} else {
		be.PutUint16(buf[off:], uint16(h.PeerAS))
		off += 2
		be.PutUint16(buf[off:], uint16(h.LocalAS))
		off += 2
	}
	be.PutUint16(buf[off:], h.IfIndex)
	off += 2
	be.PutUint16(buf[off:], h.AFI)
	off += 2
	ipLen := 4
	if h.AFI == AFIIPv6 {
		ipLen = 16
	}
	off += copyIPField(buf[off:], h.PeerIP, ipLen)
	off += copyIPField(buf[off:], h.LocalIP, ipLen)
	return off - start
}

// copyIPField copies exactly n bytes from ip into dst, zero-padding if ip is short.
func copyIPField(dst, ip []byte, n int) int {
	copied := copy(dst[:n], ip)
	for i := copied; i < n; i++ {
		dst[i] = 0
	}
	return n
}

func WriteBGP4MPMessage(buf []byte, off int, h *BGP4MPHeader, as4 bool, bgpMsg []byte) int {
	start := off
	off += writeBGP4MPCommon(buf, off, h, as4)
	copy(buf[off:], bgpMsg)
	off += len(bgpMsg)
	return off - start
}

func WriteBGP4MPStateChange(buf []byte, off int, h *BGP4MPHeader, as4 bool, oldState, newState uint16) int {
	start := off
	off += writeBGP4MPCommon(buf, off, h, as4)
	be.PutUint16(buf[off:], oldState)
	off += 2
	be.PutUint16(buf[off:], newState)
	off += 2
	return off - start
}

func WriteTableDump(buf []byte, off int, r *TableDumpRecord) int {
	start := off
	be.PutUint16(buf[off:], r.ViewNumber)
	off += 2
	be.PutUint16(buf[off:], r.SeqNumber)
	off += 2
	copy(buf[off:], r.Prefix)
	off += len(r.Prefix)
	buf[off] = r.PrefixLen
	off++
	buf[off] = r.Status
	off++
	be.PutUint32(buf[off:], r.OrigTime)
	off += 4
	copy(buf[off:], r.PeerIP)
	off += len(r.PeerIP)
	be.PutUint16(buf[off:], r.PeerAS)
	off += 2
	be.PutUint16(buf[off:], uint16(len(r.Attributes)))
	off += 2
	copy(buf[off:], r.Attributes)
	off += len(r.Attributes)
	return off - start
}
