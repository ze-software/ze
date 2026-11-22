// Design: docs/architecture/mrt.md — MRT record building from BGP events

package mrt

import (
	"encoding/binary"
	"net"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
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
	// A dynamic peer's AS is unknown until its OPEN is negotiated, so the OPEN
	// Ze sends to it has none to record. MRT's own convention for that is 0.
	// RFC 6396 Section 4.4.1: "In some cases, the Peer AS Number may be undefined.  In such cases, the value of this field MAY be set to zero."
	hdr.PeerAS = 0
	if asn, known := peer.PeerAS.ASN(); known {
		hdr.PeerAS = asn
	}
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

// DumpRIB writes one TABLE_DUMP_V2 snapshot of the RIB to the routes stream:
// what `request mrt dump-rib` and the routes interval run. It writes nothing
// when the routes stream is not configured or no RIB provider is registered.
// Safe for concurrent use: the command and the interval loop can both call it,
// and dumpMu keeps one snapshot's records together in the file.
func (c *Component) DumpRIB() {
	c.dumpMu.Lock()
	defer c.dumpMu.Unlock()
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
	hdrSize := c.headerSize()

	// Single DumpRIB call: OnPeer collects peers, the PEER_INDEX_TABLE is
	// written on the first route callback (all peers have been enumerated by
	// then), and every route callback writes one RIB record.
	// RFC 6396 Section 4.3.1: PEER_INDEX_TABLE MUST precede RIB entries.
	pb := getBuf()
	defer bufPool.Put(pb)

	var peers []mrtfmt.PeerEntry
	var seq uint32
	pitWritten := false

	// writeRoute writes one route as one record holding one RIB entry, so the
	// subtype of every record matches the route it holds even when peers of
	// one prefix differ in ADD-PATH state. addPath is the route's own state,
	// known only from the visitor callback the RIB chose; prefix never carries
	// the Path Identifier.
	//
	// RFC 8050 Section 2: "In order to support Path Identifier information as
	// defined in [RFC7911], new subtypes need to be added."
	writeRoute := func(addPath bool, peerIndex, afi, safi uint16, pathID uint32, prefixLen uint8, prefix, attrs []byte) {
		if !pitWritten {
			pitWritten = true
			c.writePeerIndexTable(hdrSize, now, peers)
		}

		seq++
		subtype := ribSubtypeFor(afi, safi, addPath)
		generic := subtype == mrtfmt.TDV2RIBGeneric || subtype == mrtfmt.TDV2RIBGenericAP
		entry := mrtfmt.RIBEntry{
			PeerIndex:  peerIndex,
			OrigTime:   ts,
			PathID:     pathID,
			Attributes: attrs,
		}

		// header + seq(4) + AFI(2) + SAFI(1) + Path ID(4) + NLRI or prefix
		// length(1) + prefix + entry count(2) + peer index(2) + time(4) +
		// Path ID(4) + attribute length(2) + attributes
		needed := hdrSize + 4 + 3 + 4 + 1 + len(prefix) + 2 + 2 + 4 + 4 + 2 + len(attrs)
		buf := pb.b
		if needed > len(buf) {
			buf = make([]byte, needed)
		}

		var msgLen int
		switch subtype {
		case mrtfmt.TDV2RIBGenericAP:
			// RFC 8050 Section 4.2
			msgLen = mrtfmt.WriteRIBGenericHeaderAddPath(buf, hdrSize, seq, afi, uint8(safi), pathID, prefix)
		case mrtfmt.TDV2RIBGeneric:
			msgLen = mrtfmt.WriteRIBGenericHeader(buf, hdrSize, seq, afi, uint8(safi), prefix)
		default:
			msgLen = mrtfmt.WriteRIBHeader(buf, hdrSize, seq, prefixLen, prefix)
		}
		// RFC 8050 Section 4.1: "This adds a field to the RIB Entries record to
		// store the Path Identifier when used with the RIB_IPV4_UNICAST_ADDPATH,
		// RIB_IPV4_MULTICAST_ADDPATH, RIB_IPV6_UNICAST_ADDPATH, and
		// RIB_IPV6_MULTICAST_ADDPATH subtypes."
		// RFC 8050 Section 4.2: "For clarity, the RIB Entries in this subtype are
		// not redefined."
		entryAddPath := addPath && !generic
		msgLen += mrtfmt.WriteRIBEntries(buf, hdrSize+msgLen, []mrtfmt.RIBEntry{entry}, entryAddPath)
		writeHeader(buf, c.config.ExtendedTimestamp, now, mrtfmt.TypeTableDumpV2, subtype, msgLen)

		if err := c.routes.Write(buf[:hdrSize+msgLen]); err != nil {
			c.logger.Warn("mrt: write rib entry", "error", err)
		}
	}

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
		// The RIB calls OnRoute for a route of a family stored without ADD-PATH
		// and OnRouteAddPath for one stored with it (registry.RIBDumpVisitor), so
		// the subtype follows the state each route was stored under.
		OnRoute: func(peerIndex, afi, safi uint16, prefixLen uint8, prefix, attrs []byte) {
			writeRoute(false, peerIndex, afi, safi, 0, prefixLen, prefix, attrs)
		},
		OnRouteAddPath: func(peerIndex, afi, safi uint16, pathID uint32, prefixLen uint8, prefix, attrs []byte) {
			writeRoute(true, peerIndex, afi, safi, pathID, prefixLen, prefix, attrs)
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
	pitLen, err := mrtfmt.WritePeerIndexTable(pitBuf, hdrSize, [4]byte{}, "", peers)
	if err != nil {
		c.logger.Warn("mrt: encode peer index table", "error", err)
		return
	}
	writeHeader(pitBuf, c.config.ExtendedTimestamp, now, mrtfmt.TypeTableDumpV2, mrtfmt.TDV2PeerIndexTable, pitLen)
	if err := c.routes.Write(pitBuf[:hdrSize+pitLen]); err != nil {
		c.logger.Warn("mrt: write peer index table", "error", err)
		return
	}
	if c.config.Geo.Enabled {
		// RFC 6397 Section 4.1 and Section 6
		c.writeGeoPeerTable(hdrSize, now, [4]byte{}, peers)
	}
}

// writeGeoPeerTable emits a TABLE_DUMP_V2 GEO_PEER_TABLE (subtype 7) companion
// record matching the PEER_INDEX_TABLE peer order and enforcing RFC 6397
// Section 4.1 coordinate validation and Section 6 privacy filtering.
func (c *Component) writeGeoPeerTable(hdrSize int, now time.Time, collectorBGPID [4]byte, peers []mrtfmt.PeerEntry) {
	// RFC 6397 Section 4.1 and Section 6
	table, err := mrtfmt.BuildGeoPeerTableForIndex(collectorBGPID, c.config.Geo.Collector, peers, c.config.Geo.Peers)
	if err != nil {
		c.logger.Warn("mrt: build geo peer table", "code", codeMRTGeoDump, "error", err)
		return
	}
	geoBuf := make([]byte, hdrSize+mrtfmt.GeoPeerTableSize(len(table.Peers)))
	// RFC 6397 Section 4.1
	geoLen, err := mrtfmt.WriteGeoPeerTable(geoBuf, hdrSize, table)
	if err != nil {
		c.logger.Warn("mrt: encode geo peer table", "code", codeMRTGeoDump, "error", err)
		return
	}
	writeHeader(geoBuf, c.config.ExtendedTimestamp, now, mrtfmt.TypeTableDumpV2, mrtfmt.TDV2GeoPeerTable, geoLen)
	if err := c.routes.Write(geoBuf[:hdrSize+geoLen]); err != nil {
		c.logger.Warn("mrt: write geo peer table", "code", codeMRTGeoDump, "error", err)
	}
}

func ribSubtype(afi uint16, addPath bool) uint16 {
	return ribSubtypeFor(afi, 1, addPath)
}

func ribSubtypeFor(afi, safi uint16, addPath bool) uint16 {
	switch {
	case afi == mrtfmt.AFIIPv4 && safi == 1 && !addPath:
		return mrtfmt.TDV2RIBIPv4Unicast
	case afi == mrtfmt.AFIIPv4 && safi == 1 && addPath:
		return mrtfmt.TDV2RIBIPv4UnicastAP
	case afi == mrtfmt.AFIIPv4 && safi == 2 && !addPath:
		return mrtfmt.TDV2RIBIPv4Multicast
	case afi == mrtfmt.AFIIPv4 && safi == 2 && addPath:
		return mrtfmt.TDV2RIBIPv4MulticastAP
	case afi == mrtfmt.AFIIPv6 && safi == 1 && !addPath:
		return mrtfmt.TDV2RIBIPv6Unicast
	case afi == mrtfmt.AFIIPv6 && safi == 1 && addPath:
		return mrtfmt.TDV2RIBIPv6UnicastAP
	case afi == mrtfmt.AFIIPv6 && safi == 2 && !addPath:
		return mrtfmt.TDV2RIBIPv6Multicast
	case afi == mrtfmt.AFIIPv6 && safi == 2 && addPath:
		return mrtfmt.TDV2RIBIPv6MulticastAP
	case addPath:
		return mrtfmt.TDV2RIBGenericAP
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

func (c *Component) bgp4mpTypeSubtype(addPath bool) (uint16, uint16) {
	return c.bgp4mpTypeSubtypeFor(addPath, true)
}

func (c *Component) bgp4mpTypeSubtypeFor(addPath, as4 bool) (uint16, uint16) {
	typ := mrtfmt.TypeBGP4MP
	if c.config.ExtendedTimestamp {
		typ = mrtfmt.TypeBGP4MPET
	}
	var subtype uint16
	switch {
	case as4 && addPath:
		subtype = mrtfmt.BGP4MPMessageAS4AP
	case as4:
		subtype = mrtfmt.BGP4MPMessageAS4
	case addPath:
		subtype = mrtfmt.BGP4MPMessageAP
	default:
		subtype = mrtfmt.BGP4MPMessage
	}
	return typ, subtype
}

// messageAS4 reports whether the session that carried the BGP message uses
// 4-byte AS_PATH encoding (RFC 6793).
// RFC 6396 Section 4.4.2: "The BGP4MP_MESSAGE Subtype does not support 4-byte
// AS numbers.  The AS_PATH contained in these messages MUST only consist of
// 2-byte AS numbers."
// RFC 6396 Section 4.4.3: "The AS_PATH in these messages MUST only consist of
// 4-byte AS numbers."
func messageAS4(ctxID bgpctx.ContextID) bool {
	ctx := bgpctx.Registry.Get(ctxID)
	if ctx == nil {
		return true
	}
	return ctx.ASN4()
}

// updateAddPath reads only family fields, never copies or decodes the NLRI.
// RFC 8050 Section 2: "new BGP4MP/BGP4MP_ET subtypes as defined in [RFC6396]
// are required to signal to an MRT parser how to parse the NLRI.".
func updateAddPath(body []byte, ctxID bgpctx.ContextID) bool {
	ctx := bgpctx.Registry.Get(ctxID)
	if ctx == nil {
		return false
	}
	withdrawn, attrs, nlri, err := mrtfmt.UpdateSections(body)
	if err == nil {
		if len(withdrawn)+len(nlri) > 0 {
			if ctx.AddPath(family.IPv4Unicast) {
				return true
			}
		}
		iter := attribute.NewAttrIterator(attrs)
		for code, _, value, ok := iter.Next(); ok; code, _, value, ok = iter.Next() {
			// Only MP_REACH_NLRI and MP_UNREACH_NLRI carry extra family fields for ADD-PATH detection.
			//exhaustive:ignore
			switch code {
			case attribute.AttrMPReachNLRI, attribute.AttrMPUnreachNLRI:
				if len(value) < 3 {
					continue
				}
				fam := family.Family{
					AFI:  family.AFI(binary.BigEndian.Uint16(value[:2])),
					SAFI: family.SAFI(value[2]),
				}
				if ctx.AddPath(fam) {
					return true
				}
			default:
			}
		}
	}
	return false
}
