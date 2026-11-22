// Design: docs/architecture/mrt.md -- RIB snapshot bridge for MRT TABLE_DUMP_V2

package rib

import (
	"context"
	"encoding/binary"
	"log/slog"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/bgp/attrpool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
)

var activeManager atomic.Pointer[RIBManager]

// RIBDumpBridge implements registry.RIBDumpCallback by forwarding to the
// active RIBManager. Registered as coordinator extra "rib.dumpCallback"
// so the MRT component can trigger periodic TABLE_DUMP_V2 snapshots.
var RIBDumpBridge ribDumpBridge

type ribDumpBridge struct{}

func (ribDumpBridge) DumpRIB(visitor registry.RIBDumpVisitor) {
	mgr := activeManager.Load()
	if mgr == nil {
		return
	}
	mgr.dumpRIBForMRT(visitor)
}

func (r *RIBManager) dumpRIBForMRT(visitor registry.RIBDumpVisitor) {
	type peerSnapshot struct {
		addr  string // canonical label for visitor.OnPeer (string boundary)
		asn   uint32
		bgpID [4]byte
		ipv6  bool
		rib   *storage.PeerRIB
	}

	r.peerMu.Lock()
	snaps := make([]peerSnapshot, 0, len(r.bgpPeers))
	for owner, peerRIB := range r.bgpPeers {
		addr, known := r.peerAddressLocked(owner)
		if !known {
			continue
		}
		// PeerRIB caches the canonical address string for the visitor label;
		// the typed key answers the IPv6 question without re-parsing.
		s := peerSnapshot{addr: peerRIB.PeerAddr(), rib: peerRIB}
		if meta := r.peerMeta[owner]; meta != nil {
			s.asn = meta.PeerASN
			// RFC 6396 Section 4.3.1: a PEER_INDEX_TABLE entry carries the
			// PEER's BGP Identifier. RouterID is this speaker's, so every entry
			// of every dump used to name the collector instead of the peer.
			binary.BigEndian.PutUint32(s.bgpID[:], meta.RemoteRouterID)
		}
		s.ipv6 = !addr.Unmap().Is4()
		snaps = append(snaps, s)
	}
	for _, peerRIB := range r.ribInPool[bgpProtocolID] {
		snaps = append(snaps, peerSnapshot{addr: peerRIB.PeerAddr(), rib: peerRIB, ipv6: peerRIB.PeerAddress().Is6()})
	}
	r.peerMu.Unlock()

	// Per-snapshot index, no peer-keyed map needed: snaps and indices align.
	peerIndices := make([]uint16, len(snaps))
	for i := range snaps {
		s := &snaps[i]
		peerIndices[i] = visitor.OnPeer(s.addr, s.asn, s.bgpID, s.ipv6)
	}

	type routeSnap struct {
		nlri  []byte
		entry storage.RouteEntry
	}

	attrBuf := make([]byte, 0, 4096)
	// Runs after the walk, with no peer or family lock held.
	var skipped mrtSkippedRoutes
	defer skipped.report()
	for i := range snaps {
		s := &snaps[i]
		peerIdx := peerIndices[i]
		if s.rib == nil {
			continue
		}

		for _, fam := range s.rib.Families() {
			// Read before IterateFamily: PeerRIB.IsAddPath takes the lock the walk holds.
			// It is the ADD-PATH state negotiated for this peer's family, the one
			// fact that says how every stored key of the family is framed, and the
			// visitor callback chosen below hands it to the writer.
			addPath := s.rib.IsAddPath(fam)
			onAddPath := addPath && visitor.OnRouteAddPath != nil
			cidr := mrtPrefixFamily(fam)
			// Snapshot route entries under short per-family lock, then process outside.
			// The walk is the wire one: RFC 6396 Section 4.3.3 says of RIB_GENERIC
			// "It consists of an AFI, Subsequent AFI (SAFI), and a single NLRI
			// entry.  The NLRI information is specific to the AFI and SAFI values."
			// A parser therefore reads the field as the NLRI of that AFI and SAFI,
			// so a labeled unicast route is written with its label stack (RFC 8277
			// Section 2.3), which the stored key leaves out, and never as a bare
			// prefix whose Length a labeled parser would read as label bits.
			var routes []routeSnap
			s.rib.IterateFamilyWire(fam, func(nlriBytes []byte, entry storage.RouteEntry) bool {
				cp := make([]byte, len(nlriBytes))
				copy(cp, nlriBytes)
				routes = append(routes, routeSnap{nlri: cp, entry: entry})
				return true
			})

			afi := uint16(fam.AFI)
			safi := uint16(fam.SAFI)
			for _, rt := range routes {
				attrBuf = reconstructWireAttrs(rt.entry, attrBuf[:0])
				if len(attrBuf) == 0 {
					continue
				}
				// What the wire walk yields, every family:
				//
				//   ADD-PATH:  [Path Identifier (4, big-endian)][NLRI ...]
				//   otherwise: [NLRI ...]
				//
				// where NLRI is the one an UPDATE carries for the route. For an
				// IPv4/IPv6 unicast or multicast family it is
				// [prefix length (1, bits)][prefix (ceil(bits/8))]; for labeled
				// unicast it is [Length (1, bits)][labels (3 each)][prefix]. The Path
				// Identifier is peeled for every ADD-PATH family, so the writer places
				// it where the subtype puts it: in the RIB Entry (subtypes 8 to 11) or
				// back in front of the NLRI (subtype 12).
				// RFC 8050 Section 2: "In order to support Path Identifier
				// information as defined in [RFC7911], new subtypes need to be added."
				nlri := rt.nlri
				var pathID uint32
				if addPath {
					if len(nlri) < 4 {
						// Storage frames every ADD-PATH NLRI with its Path Identifier
						// (FamilyRIB.buildNLRIBytes, appendFramedKey, labeledWireNLRI),
						// so this one is a storage defect. Writing it would record bytes its subtype
						// contradicts; it is counted and reported once per dump.
						skipped.noPathID++
						continue
					}
					pathID = binary.BigEndian.Uint32(nlri[:4])
					nlri = nlri[4:]
				}
				var prefixLen uint8
				prefix := nlri
				if cidr {
					if len(nlri) == 0 {
						skipped.noPrefixLength++
						continue
					}
					prefixLen = nlri[0]
					prefix = nlri[1:]
				}
				// RFC 8050 Section 4.1
				if onAddPath {
					visitor.OnRouteAddPath(peerIdx, afi, safi, pathID, prefixLen, prefix, attrBuf)
					continue
				}
				visitor.OnRoute(peerIdx, afi, safi, prefixLen, prefix, attrBuf)
			}
		}
	}
}

// mrtSkippedRoutes counts the routes one MRT snapshot left out because their
// stored key cannot be a valid NLRI of their family: an ADD-PATH key shorter
// than its 4-octet Path Identifier, or a unicast or multicast key with no
// prefix length. Storage never builds either, so each is a Ze defect. A dump
// can walk a whole table, so the walk counts and reports once.
type mrtSkippedRoutes struct {
	noPathID       int
	noPrefixLength int
}

// report logs one warning and raises one recent-event report for the dump,
// and does nothing when the dump skipped no route. The pointer receiver makes
// a deferred call read the counts at return, not at the defer statement.
func (s *mrtSkippedRoutes) report() {
	if s.noPathID+s.noPrefixLength == 0 {
		return
	}
	failure := opererr.New("The MRT RIB snapshot left out routes whose stored key is not a valid NLRI of their family",
		"The dump file lacks those routes: writing them would record bytes their TABLE_DUMP_V2 subtype contradicts, and the RIB never stores such a key, so this is a Ze defect",
		"Report this as a Ze defect with this log line, then request mrt dump-rib after a restart")
	logger().LogAttrs(context.Background(), slog.LevelWarn, failure.Problem(),
		failure.LogAttrs(slog.String("code", diagnostic.CodeDoctorRecentEvents),
			slog.Int("without-path-id", s.noPathID), slog.Int("without-prefix-length", s.noPrefixLength))...)
	report.RaiseError("bgp-rib", diagnostic.CodeDoctorRecentEvents, "request mrt dump-rib", failure.Error())
}

// mrtPrefixFamily reports whether fam is written under an RFC 6396 Section
// 4.3.2 AFI/SAFI-specific RIB subtype, whose header splits the NLRI into a
// prefix length and a prefix: IPv4 or IPv6, unicast or multicast. Every other
// family is written as RIB_GENERIC (Section 4.3.3) with its whole NLRI, the one
// an UPDATE carries: labeled unicast with its label stack, VPN with its labels
// and Route Distinguisher, EVPN with its labels, ESI and gateway
// (storage.FamilyRIB.IterateWire).
func mrtPrefixFamily(fam family.Family) bool {
	// RFC 6396 Section 4.3 names subtypes for unicast and multicast alone.
	//exhaustive:ignore
	switch fam.SAFI {
	case family.SAFIUnicast, family.SAFIMulticast:
	default:
		return false
	}
	return fam.AFI == family.AFIIPv4 || fam.AFI == family.AFIIPv6
}

// reconstructWireAttrs rebuilds full BGP path attribute bytes from pooled handles.
// buf is a reusable buffer (caller passes buf[:0] to reuse capacity).
// The shared flag accessors keep the snapshot faithful to RFC 4271 Section 5:
// "If a path with a recognized, transitive optional attribute is accepted and
// passed along to other BGP peers and the Partial bit in the Attribute Flags
// octet is set to 1 by some previous AS, it MUST NOT be set back to 0 by the
// current AS." Snapshot framing and buffer ownership remain this writer's.
func reconstructWireAttrs(entry storage.RouteEntry, buf []byte) []byte {
	b := entry.GetBundle()

	type attr struct {
		code  uint8
		flags uint8
		data  []byte
	}
	var attrs []attr

	appendIfValid := func(code uint8, flags uint8, p *attrpool.Pool, h attrpool.Handle) {
		if !h.IsValid() {
			return
		}
		data, err := p.Get(h)
		if err != nil {
			return
		}
		attrs = append(attrs, attr{code: code, flags: flags, data: data})
	}

	appendIfValid(1, 0x40, pool.Origin, b.Origin)
	if entry.HasASPath() {
		appendIfValid(2, 0x40, pool.ASPath, entry.ASPath)
	}
	appendIfValid(3, 0x40, pool.NextHop, b.NextHop)
	appendIfValid(4, 0x80, pool.MED, b.MED)
	appendIfValid(5, 0x40, pool.LocalPref, b.LocalPref)
	appendIfValid(6, 0x40, pool.AtomicAggregate, b.AtomicAggregate)
	appendIfValid(7, entry.AggregatorFlags(), pool.Aggregator, b.Aggregator)
	appendIfValid(8, entry.CommunityFlags(), pool.Communities, b.Communities)
	appendIfValid(9, 0x80, pool.OriginatorID, b.OriginatorID)
	appendIfValid(10, 0x80, pool.ClusterList, b.ClusterList)
	appendIfValid(16, entry.ExtCommunityFlags(), pool.ExtCommunities, b.ExtCommunities)
	appendIfValid(32, entry.LargeCommunityFlags(), pool.LargeCommunities, b.LargeCommunities)

	for _, a := range attrs {
		buf = appendWireAttr(buf, a.code, a.flags, a.data)
	}

	if b.HasOtherAttrs() {
		data, err := pool.OtherAttrs.Get(b.OtherAttrs)
		if err == nil {
			buf = appendOtherAttrsWireForMRT(buf, data)
		}
	}

	return buf
}

// abbreviateMPReachForMRT strips AFI, SAFI, Reserved, and NLRI from a wire
// MP_REACH_NLRI (code 14) attribute value, returning [nhLen, nextHop...].
// RFC 6396 Section 4.3.4: "only the Next Hop Address Length and Next Hop
// Address fields are included.  The Reserved field is omitted."
func abbreviateMPReachForMRT(value []byte) ([]byte, bool) {
	if len(value) < 4 {
		return nil, false
	}
	nhLen := int(value[3])
	if 4+nhLen > len(value) {
		return nil, false
	}
	return value[3 : 4+nhLen], true
}

// appendOtherAttrsWireForMRT converts the compact OtherAttrs pool format back
// to wire for MRT TABLE_DUMP_V2 RIB entries, abbreviating MP_REACH_NLRI (code 14)
// per RFC 6396 Section 4.3.4.
func appendOtherAttrsWireForMRT(buf, data []byte) []byte {
	off := 0
	for off+4 <= len(data) {
		code := data[off]
		flags := data[off+1]
		vLen := int(data[off+2])<<8 | int(data[off+3])
		off += 4
		if off+vLen > len(data) {
			break
		}
		val := data[off : off+vLen]
		if code == 14 {
			if abbrev, ok := abbreviateMPReachForMRT(val); ok {
				buf = appendWireAttr(buf, code, flags&^0x10, abbrev)
				off += vLen
				continue
			}
		}
		buf = appendWireAttr(buf, code, flags, val)
		off += vLen
	}
	return buf
}

func appendWireAttr(buf []byte, code, flags uint8, value []byte) []byte {
	// RFC 4271 Section 4.3: "If the Extended Length bit of the Attribute Flags
	// octet is set to 1, the third and fourth octets of the path attribute
	// contain the length of the attribute data in octets."
	if len(value) > 255 || flags&0x10 != 0 {
		flags |= 0x10 // Extended Length
		buf = append(buf, flags, code, byte(len(value)>>8), byte(len(value)))
	} else {
		buf = append(buf, flags, code, byte(len(value)))
	}
	return append(buf, value...)
}

// appendOtherAttrsWire converts the compact OtherAttrs pool format back to wire.
func appendOtherAttrsWire(buf, data []byte) []byte {
	off := 0
	for off+4 <= len(data) {
		code := data[off]
		flags := data[off+1]
		vLen := int(data[off+2])<<8 | int(data[off+3])
		off += 4
		if off+vLen > len(data) {
			break
		}
		buf = appendWireAttr(buf, code, flags, data[off:off+vLen])
		off += vLen
	}
	return buf
}
