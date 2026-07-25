// Design: docs/architecture/mrt.md — MRT record building from BGP events

package mrt

import (
	"net"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	mrtfmt "github.com/ze-software/ze/internal/mrt"
)

type poolBuf struct {
	b []byte
}

// maxRecordLen bounds the largest MRT record OnBGPMessage can build: the MRT
// (extended) common header + the BGP4MP common header wrapping a maximum-size
// BGP message. A BGP message length is a 16-bit field, so at most 65535 bytes
// (RFC 8654 extended messages); the BGP4MP common header is at most 44 bytes
// (PeerAS4(4)+LocalAS4(4)+IfIndex(2)+AFI(2)+two 16-byte IPv6 addresses); the MRT
// header at most CommonHeaderLen+ExtTimestampLen. Sizing the pooled record
// buffer to this maximum keeps a peer's max-size extended UPDATE from
// overflowing it in OnBGPMessage (`record := pb.b[:off+msgLen]`) and panicking
// the session goroutine -- a remotely-triggerable crash.
const (
	maxBGPMessageLen   = 65535 // 16-bit BGP Length field (RFC 8654 extended messages)
	maxBGP4MPCommonLen = 44    // AS4(8)+IfIndex(2)+AFI(2)+IPv6 peer(16)+local(16)
	maxRecordLen       = mrtfmt.CommonHeaderLen + mrtfmt.ExtTimestampLen + maxBGP4MPCommonLen + maxBGPMessageLen
)

var bufPool = sync.Pool{
	New: func() any { return &poolBuf{b: make([]byte, maxRecordLen)} },
}

func getBuf() *poolBuf {
	pb, ok := bufPool.Get().(*poolBuf)
	if !ok {
		return &poolBuf{b: make([]byte, maxRecordLen)}
	}
	return pb
}

func writeHeader(buf []byte, extTimestamp bool, now time.Time, typ, subtype uint16, msgLen int) {
	ts := uint32(now.Unix())
	if extTimestamp {
		usec := uint32(now.Nanosecond() / 1000)
		mrtfmt.WriteExtendedHeader(buf, 0, ts, usec, typ, subtype, uint32(msgLen+mrtfmt.ExtTimestampLen))
	} else {
		mrtfmt.WriteCommonHeader(buf, 0, ts, typ, subtype, uint32(msgLen))
	}
}

// peerInfoToHeader writes peer/local IP into ipBuf (caller-owned, avoids heap escape)
// and fills hdr whose IP slices point into ipBuf.
// ipBuf must be at least 32 bytes (2 x 16 for IPv6).
func peerInfoToHeader(peer *plugin.PeerInfo, ipBuf []byte, hdr *mrtfmt.BGP4MPHeader) {
	hdr.PeerAS = peer.PeerAS
	hdr.LocalAS = peer.LocalAS
	hdr.IfIndex = 0
	addr := peer.Address
	localAddr := peer.LocalAddress
	if addr.Is6() {
		hdr.AFI = mrtfmt.AFIIPv6
		p := addr.As16()
		copy(ipBuf[0:16], p[:])
		l := localAddr.As16()
		copy(ipBuf[16:32], l[:])
		hdr.PeerIP = ipBuf[0:16]
		hdr.LocalIP = ipBuf[16:32]
	} else {
		hdr.AFI = mrtfmt.AFIIPv4
		p := addr.As4()
		copy(ipBuf[0:4], p[:])
		l := localAddr.As4()
		copy(ipBuf[4:8], l[:])
		hdr.PeerIP = ipBuf[0:4]
		hdr.LocalIP = ipBuf[4:8]
	}
}

func localSubtype(subtype uint16) uint16 {
	switch subtype {
	case mrtfmt.BGP4MPMessageAS4:
		return mrtfmt.BGP4MPMessageAS4Local
	case mrtfmt.BGP4MPMessageAS4AP:
		return mrtfmt.BGP4MPMessageAS4LocalAP
	case mrtfmt.BGP4MPMessage:
		return mrtfmt.BGP4MPMessageLocal
	case mrtfmt.BGP4MPMessageAP:
		return mrtfmt.BGP4MPMessageLocalAP
	}
	return subtype
}

func (c *Component) dumpRIB() {
	if c.routes == nil {
		return
	}
	if c.ribDumper == nil {
		c.logger.Debug("mrt: RIB dump skipped, no RIB callback")
		return
	}
	if err := c.routes.Rotate(); err != nil {
		c.logger.Warn("mrt: rotate rib file", "error", err)
		return
	}
	c.writeTableDumpV2()
}

func (c *Component) writeTableDumpV2() {
	now := time.Now()
	ts := uint32(now.Unix())
	addPath := c.config.AddPath
	hdrSize := c.headerSize()

	// Single DumpRIB call: OnPeer collects peers, the PEER_INDEX_TABLE is
	// written on the first OnRoute call (all peers have been enumerated by
	// then), and subsequent OnRoute calls write RIB entries.
	// RFC 6396 Section 4.3.1: PEER_INDEX_TABLE MUST precede RIB entries.
	pb := getBuf()
	defer bufPool.Put(pb)

	var peers []mrtfmt.PeerEntry
	var seq uint32
	pitWritten := false

	visitor := registry.RIBDumpVisitor{
		OnPeer: func(peerAddr string, peerASN uint32, bgpID [4]byte, isIPv6 bool) uint16 {
			idx := uint16(len(peers))
			pe := mrtfmt.PeerEntry{
				Type:  mrtfmt.PeerAS4,
				BGPID: bgpID,
				ASN:   peerASN,
			}
			if isIPv6 {
				pe.Type |= mrtfmt.PeerIPv6
				pe.IP = make([]byte, 16)
			} else {
				pe.IP = make([]byte, 4)
			}
			parseIPIntoPeer(peerAddr, &pe)
			peers = append(peers, pe)
			return idx
		},
		OnRoute: func(peerIndex, afi, _ uint16, prefixLen uint8, prefix, attrs []byte) {
			if !pitWritten {
				pitWritten = true
				c.writePeerIndexTable(hdrSize, now, peers)
			}

			seq++
			entry := mrtfmt.RIBEntry{
				PeerIndex:  peerIndex,
				OrigTime:   ts,
				Attributes: attrs,
			}

			subtype := ribSubtype(afi, addPath)

			needed := hdrSize + 4 + 1 + len(prefix) + 2 + 8 + 2 + len(attrs) + 4
			buf := pb.b
			if needed > len(buf) {
				buf = make([]byte, needed)
			}

			msgLen := mrtfmt.WriteRIBHeader(buf, hdrSize, seq, prefixLen, prefix)
			msgLen += mrtfmt.WriteRIBEntries(buf, hdrSize+msgLen, []mrtfmt.RIBEntry{entry}, addPath)
			writeHeader(buf, c.config.ExtendedTimestamp, now, mrtfmt.TypeTableDumpV2, subtype, msgLen)

			if err := c.routes.Write(buf[:hdrSize+msgLen]); err != nil {
				c.logger.Warn("mrt: write rib entry", "error", err)
			}
		},
	}
	c.ribDumper.DumpRIB(visitor)

	if seq == 0 && len(peers) > 0 {
		c.writePeerIndexTable(hdrSize, now, peers)
	}

	c.logger.Info("mrt: RIB dump complete", "peers", len(peers), "sequences", seq)
}

func (c *Component) writePeerIndexTable(hdrSize int, now time.Time, peers []mrtfmt.PeerEntry) {
	pitSize := 4 + 2 + 2 // collectorBGPID + viewNameLen + peerCount
	for i := range peers {
		pitSize += 5 // type(1) + bgpid(4)
		pitSize += len(peers[i].IP)
		if peers[i].IsAS4() {
			pitSize += 4
		} else {
			pitSize += 2
		}
	}
	pitBuf := make([]byte, hdrSize+pitSize)
	pitLen := mrtfmt.WritePeerIndexTable(pitBuf, hdrSize, [4]byte{}, "", peers)
	writeHeader(pitBuf, c.config.ExtendedTimestamp, now, mrtfmt.TypeTableDumpV2, mrtfmt.TDV2PeerIndexTable, pitLen)
	if err := c.routes.Write(pitBuf[:hdrSize+pitLen]); err != nil {
		c.logger.Warn("mrt: write peer index table", "error", err)
	}
}

func ribSubtype(afi uint16, addPath bool) uint16 {
	switch {
	case afi == mrtfmt.AFIIPv4 && !addPath:
		return mrtfmt.TDV2RIBIPv4Unicast
	case afi == mrtfmt.AFIIPv4 && addPath:
		return mrtfmt.TDV2RIBIPv4UnicastAP
	case afi == mrtfmt.AFIIPv6 && !addPath:
		return mrtfmt.TDV2RIBIPv6Unicast
	case afi == mrtfmt.AFIIPv6 && addPath:
		return mrtfmt.TDV2RIBIPv6UnicastAP
	default:
		return mrtfmt.TDV2RIBGeneric
	}
}

func parseIPIntoPeer(addr string, pe *mrtfmt.PeerEntry) {
	ip := net.ParseIP(addr)
	if ip == nil {
		return
	}
	if v4 := ip.To4(); v4 != nil {
		copy(pe.IP, v4)
	} else {
		copy(pe.IP, ip.To16())
	}
}

func (c *Component) headerSize() int {
	if c.config.ExtendedTimestamp {
		return mrtfmt.CommonHeaderLen + mrtfmt.ExtTimestampLen
	}
	return mrtfmt.CommonHeaderLen
}

func (c *Component) bgp4mpTypeSubtype() (uint16, uint16) {
	typ := mrtfmt.TypeBGP4MP
	if c.config.ExtendedTimestamp {
		typ = mrtfmt.TypeBGP4MPET
	}
	subtype := mrtfmt.BGP4MPMessageAS4
	if c.config.AddPath {
		subtype = mrtfmt.BGP4MPMessageAS4AP
	}
	return typ, subtype
}
