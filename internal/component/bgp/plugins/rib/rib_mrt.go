// Design: docs/architecture/mrt.md -- RIB snapshot bridge for MRT TABLE_DUMP_V2

package rib

import (
	"encoding/binary"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/bgp/attrpool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/component/plugin/registry"
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
	for addr, peerRIB := range r.bgpPeers {
		// PeerRIB caches the canonical address string for the visitor label;
		// the typed key answers the IPv6 question without re-parsing.
		s := peerSnapshot{addr: peerRIB.PeerAddr(), rib: peerRIB}
		if meta := r.peerMeta[addr]; meta != nil {
			s.asn = meta.PeerASN
			binary.BigEndian.PutUint32(s.bgpID[:], meta.RouterID)
		}
		s.ipv6 = !addr.Unmap().Is4()
		snaps = append(snaps, s)
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
	for i := range snaps {
		s := &snaps[i]
		peerIdx := peerIndices[i]
		if s.rib == nil {
			continue
		}

		for _, fam := range s.rib.Families() {
			// Snapshot route entries under short per-family lock, then process outside.
			var routes []routeSnap
			s.rib.IterateFamily(fam, func(nlriBytes []byte, entry storage.RouteEntry) bool {
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
				switch {
				case (fam.AFI == 1 || fam.AFI == 2) && fam.SAFI == 1 && len(rt.nlri) > 0:
					visitor.OnRoute(peerIdx, afi, safi, rt.nlri[0], rt.nlri[1:], attrBuf)
				default:
					visitor.OnRoute(peerIdx, afi, safi, 0, rt.nlri, attrBuf)
				}
			}
		}
	}
}

// reconstructWireAttrs rebuilds full BGP path attribute bytes from pooled handles.
// buf is a reusable buffer (caller passes buf[:0] to reuse capacity).
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
	appendIfValid(7, 0xC0, pool.Aggregator, b.Aggregator)
	appendIfValid(8, 0xC0, pool.Communities, b.Communities)
	appendIfValid(9, 0x80, pool.OriginatorID, b.OriginatorID)
	appendIfValid(10, 0x80, pool.ClusterList, b.ClusterList)
	appendIfValid(16, 0xC0, pool.ExtCommunities, b.ExtCommunities)
	appendIfValid(32, 0xC0, pool.LargeCommunities, b.LargeCommunities)

	for _, a := range attrs {
		buf = appendWireAttr(buf, a.code, a.flags, a.data)
	}

	if b.HasOtherAttrs() {
		data, err := pool.OtherAttrs.Get(b.OtherAttrs)
		if err == nil {
			buf = appendOtherAttrsWire(buf, data)
		}
	}

	return buf
}

func appendWireAttr(buf []byte, code, flags uint8, value []byte) []byte {
	if len(value) > 255 {
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
