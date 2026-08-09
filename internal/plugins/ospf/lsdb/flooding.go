// Design: docs/architecture/ospf/ospf-7-lsdb-flooding.md -- RFC 2328 Section 13 flooding.
// RFC: rfc/short/rfc2328.md -- Section 13 flooding; rfc/short/rfc5250.md -- opaque scope.
// RFC 2328 Section 13.3: flood out eligible interfaces and retransmit until acked.

package lsdb

import (
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

const (
	AreaTypeNormal = "normal"
	AreaTypeStub   = "stub"
	AreaTypeNSSA   = "nssa"

	NetworkBroadcast         = "broadcast"
	NetworkPointToPoint      = "point-to-point"
	NetworkNBMA              = "nbma"
	NetworkPointToMultipoint = "point-to-multipoint"
	// NetworkVirtual marks a synthetic virtual-link interface (RFC 2328 section 15 / RFC 5340
	// section 4.2). It is a backbone point-to-point interface whose Router-LSA link is a
	// Type-4 virtual record (IPv4) / RouterLinkTypeVirtual record (IPv6); it carries no
	// Network-LSA/stub link and its packets are routed, not link-local.
	NetworkVirtual = "virtual"

	InterfaceStateDown   = "down"
	InterfaceStateDR     = "dr"
	InterfaceStateBackup = "backup"

	NeighborStateExchange = "exchange"
	NeighborStateLoading  = "loading"
	NeighborStateFull     = "full"

	MaxRetransmitPerNeighbor = 16384
	MaxDelayedAcksPerIface   = 4096
)

// TxFunc sends a fully encoded OSPF packet payload on an interface.
type TxFunc func(interfaceName string, dst netip.Addr, payload []byte) error

// TopologyFunc returns a point-in-time value snapshot of OSPF interfaces and
// neighbors. LSDB does not retain pointers into interface or neighbor state.
type TopologyFunc func() []InterfaceInfo

// InterfaceInfo is the LSDB/flooding view of an OSPF interface.
type InterfaceInfo struct {
	Name        string
	AreaID      types.AreaID
	AreaType    string
	NetworkType string
	State       string
	Priority    uint8
	Passive     bool
	Address     [4]byte
	NetworkMask [4]byte
	// InterfaceID is the OSPFv3 Interface ID (the OS ifindex) used for this interface's
	// links in the address-free Router-LSA (RFC 5340 sec 3.4.3); OSPFv2 origination
	// ignores it (OSPFv2 Router-LSA links carry IP addresses, not Interface IDs).
	InterfaceID        uint32
	Cost               uint16
	RouterID           types.RouterID
	Options            types.Options
	DR                 types.RouterID
	BDR                types.RouterID
	RetransmitInterval uint16
	TransmitDelay      uint16
	Neighbors          []NeighborInfo
	// IsV6 selects the OSPFv3 link-local multicast groups (ff02::5/ff02::6) for flooding
	// instead of the OSPFv2 groups (224.0.0.5/224.0.0.6); a raw IPv6 socket rejects an
	// IPv4 destination.
	IsV6          bool
	IPv6LinkLocal netip.Addr
	IPv6Prefixes  []netip.Prefix
	// IPv6Addresses are the interface's global unmasked IPv6 addresses. A
	// point-to-multipoint interface advertises each as a /128 LA-bit host route in its
	// Intra-Area-Prefix-LSA (RFC 5340 App A.4.10); other network types use the masked
	// IPv6Prefixes subnet.
	IPv6Addresses []netip.Addr
	// LDPSyncWithholdTransit, when set, suppresses this interface's transit (Link
	// Type 2) link in the Router-LSA: RFC 6138 §4 withholds the link-to-pseudonode
	// advertisement for a broadcast segment until LDP is synchronized with all peers,
	// unless the interface is a cut-edge. The stub link for the subnet is unaffected.
	// The engine sets this only for a non-cut-edge broadcast interface whose LDP-sync
	// state is not yet synchronized; P2P interfaces instead set LDPSyncMaxMetric.
	LDPSyncWithholdTransit bool
	// LDPSyncMaxMetric, when set, advertises this interface's point-to-point / transit
	// link at LSInfinity (RFC 5443 §2 cost-out while LDP is not yet synchronized),
	// mirroring the router-wide RFC 6987 max-metric path. Only the p2p/transit link is
	// raised; the connected-subnet stub link keeps the configured cost (unlike a blanket
	// InterfaceInfo.Cost override, which would also cost out the stub). The engine sets
	// this only for a P2P interface whose LDP-sync state is not yet synchronized.
	LDPSyncMaxMetric bool
	// VirtualTransitArea is the transit area a NetworkVirtual interface runs through (RFC
	// 2328 section 15). The virtual link's Type-4 record is emitted into the backbone
	// Router-LSA, but the Router-LSA V-bit is set in the TRANSIT area's Router-LSA (RFC
	// 2328 App A.4.2 / section 16.3 TransitCapability). Zero for non-virtual interfaces.
	VirtualTransitArea types.AreaID
}

// NeighborInfo is the flooding view of one neighbor.
type NeighborInfo struct {
	RouterID types.RouterID
	// Address is the neighbor's reachable source (IPv4 for OSPFv2, IPv6 link-local
	// for OSPFv3): the unicast retransmit destination and the SPF next-hop.
	Address netip.Addr
	State   string
	// InterfaceID is the neighbor's advertised OSPFv3 Interface ID (zero for OSPFv2),
	// used as the Neighbor Interface ID in the address-free Router-LSA p2p link.
	InterfaceID uint32
	// OpaqueCapable is true when the neighbor set the O-bit in its Database Description
	// packets (RFC 5250 §3.1). Opaque LSAs (types 9/10/11) are flooded ONLY to
	// opaque-capable neighbors; a non-opaque neighbor is never queued for one.
	OpaqueCapable bool
}

// NeighborKey identifies one retransmit list.
type NeighborKey struct {
	Interface string
	RouterID  types.RouterID
}

type retransmitEntry struct {
	area types.AreaID
	key  types.LSAKey
	lsa  packet.LSAHeader
	raw  []byte
	sent bool
	last time.Time
}

// ReceiveInput is one decoded LS Update from the packet dispatcher.
type ReceiveInput struct {
	Interface string
	AreaID    types.AreaID
	RouterID  types.RouterID
	Src       netip.Addr
	Update    packet.LSUpdate
}

// AckInput is one decoded LS Ack from the packet dispatcher.
type AckInput struct {
	Interface string
	AreaID    types.AreaID
	RouterID  types.RouterID
	Ack       packet.LSAck
}

// ReceiveUpdate runs the RFC 2328 Section 13 receive procedure for every LSA in
// an LS Update. Packet header authentication and area validation are upstream.
func (d *LSDB) ReceiveUpdate(in ReceiveInput) string {
	d.mUpdatesReceived.With(in.Interface).Inc()
	iface, ok := d.interfaceByName(in.Interface)
	if !ok {
		return "interface"
	}
	for _, lsa := range in.Update.LSAs {
		if !lsa.VerifyChecksum() {
			return "bad-lsa-checksum"
		}
		if shouldDropByArea(iface.AreaType, lsa.Header.Type) {
			continue
		}
		if isLinkLSAType(lsa.Header.Type) {
			if d.handleSelfLinkReceived(in.Interface, lsa) {
				continue
			}
			res, ok := d.installLink(in.Interface, in.AreaID, lsa, false, true)
			if !ok {
				return "lsdb-reject"
			}
			switch res.Freshness {
			case Newer:
				d.removeFromAllRetransmit(in.AreaID, lsa.Header.Key())
				d.ackForReceive(in, lsa.Header, false, false, false)
				d.notifyChange(in.AreaID)
				// OSPF Graceful Restart (RFC 3623 sec 3.2): surface the content change to the
				// helper's strict-LSA-checking exit.
				d.notifyContentChange(in.AreaID, lsa.Header.Type)
				// RFC 5250 Section 3: deliver a Type-9 opaque LSA to its consumer on a
				// newer install only; the store + flood above already ran for every LSA.
				d.deliverOpaqueOnNewer(in, lsa)
			case Equal:
				implied := d.clearRetransmit(NeighborKey{Interface: in.Interface, RouterID: in.RouterID}, in.AreaID, lsa.Header)
				d.ackForReceive(in, lsa.Header, true, implied, false)
			case Older:
				if res.Entry != nil {
					dbh := res.Entry.Header(d.now())
					if dbh.Age.IsMaxAge() && dbh.Sequence.IsMax() {
						continue
					}
					d.sendDirectLinkLSUpdate(in.Interface, in.Src, in.AreaID, res.Entry, iface.TransmitDelay)
				}
			}
			continue
		}
		if d.handleSelfReceived(in.AreaID, lsa) {
			continue
		}
		if lsa.Header.Age.IsMaxAge() {
			if _, exists := d.Lookup(in.AreaID, lsa.Header.Key()); !exists && !d.hasExchangeOrLoadingForKey(in.AreaID, lsa.Header.Key()) {
				d.sendAck(in.Interface, in.Src, in.AreaID, []packet.LSAHeader{lsa.Header})
				continue
			}
		}
		res, ok := d.install(in.AreaID, lsa, false, true)
		if !ok {
			return "lsdb-reject"
		}
		switch res.Freshness {
		case Newer:
			d.removeFromAllRetransmit(in.AreaID, lsa.Header.Key())
			floodedBack := d.floodExcept(in.Interface, in.RouterID, in.AreaID, lsa.Header.Key())
			d.ackForReceive(in, lsa.Header, false, false, floodedBack)
			d.notifyChange(in.AreaID)
			// OSPF Graceful Restart (RFC 3623 sec 3.2): surface the content change to the
			// helper's strict-LSA-checking exit.
			d.notifyContentChange(in.AreaID, lsa.Header.Type)
			// RFC 5250 Section 3: deliver a Type-10/11 opaque LSA to its consumer on a
			// newer install only; the store + flood above already ran for every LSA.
			d.deliverOpaqueOnNewer(in, lsa)
		case Equal:
			implied := d.clearRetransmit(NeighborKey{Interface: in.Interface, RouterID: in.RouterID}, in.AreaID, lsa.Header)
			d.ackForReceive(in, lsa.Header, true, implied, false)
		case Older:
			if res.Entry != nil {
				// RFC 2328 §13 (step 8): if the database copy has LS age MaxAge and LS sequence
				// number MaxSequenceNumber, the sequence is wrapping and the old instance must be
				// completely flushed before any new instance; discard the older received LSA
				// silently -- do not send the database copy back and do not acknowledge it.
				dbh := res.Entry.Header(d.now())
				if dbh.Age.IsMaxAge() && dbh.Sequence.IsMax() {
					continue
				}
				d.sendDirectLSUpdate(in.Interface, in.Src, in.AreaID, res.Entry, iface.TransmitDelay)
			}
		}
	}
	return ""
}

// ReceiveAck removes acknowledged LSAs from a neighbor's retransmit list. A
// MaxAge purge is deleted only after every retransmit list has acknowledged it.
func (d *LSDB) ReceiveAck(in AckInput) string {
	for _, h := range in.Ack.Headers {
		d.clearRetransmit(NeighborKey{Interface: in.Interface, RouterID: in.RouterID}, in.AreaID, h)
		d.deletePurgedIfAcked(in.AreaID, h.Key())
	}
	return ""
}

// isASWideType reports whether an LSA type floods AS-wide (no area match): a Type-5
// AS-External-LSA or a Type-11 (AS-scope) opaque LSA. RFC 5250 Section 3.1: Type-11's
// flooding scope equals Type-5, but Type-11 carries no scope bits, so ASExternal() is false
// for it and it must be named explicitly. Retransmit bookkeeping keyed by area treats these
// as "matches any area", since one AS-wide instance is flooded into every area at once.
func isASWideType(t types.LSType) bool {
	return t.ASWide() || t == types.LSTypeOpaqueAS
}

// shouldDropByArea is the RFC 2328 sec 3.6 / RFC 3101 receive-side area filter: a stub or
// NSSA area accepts neither AS-External nor ASBR-Summary / Inter-Area-Router LSAs, and an
// NSSA-LSA is valid ONLY inside an NSSA. The type classification is address-family-neutral
// (OSPFv2 Type 5/4/7 or the OSPFv3 scope-typed 0x4005/0x2004/0x2007), so the same filter
// applies to both families.
func shouldDropByArea(areaType string, typ types.LSType) bool {
	stubLike := areaType == AreaTypeStub || areaType == AreaTypeNSSA
	switch {
	// RFC 5250 Section 3.1: a Type-11 (AS-scope) opaque LSA MUST NOT be flooded into a
	// stub or NSSA area, and one received on such an interface MUST be discarded --
	// the same rule as a Type-5 AS-External LSA (isASWideType covers both).
	case isASWideType(typ) || typ.InterAreaRouter():
		return stubLike
	case typ.NSSA():
		return areaType != AreaTypeNSSA
	default:
		return false
	}
}

func (d *LSDB) interfaceByName(name string) (InterfaceInfo, bool) {
	d.mu.RLock()
	topology := d.topology
	d.mu.RUnlock()
	if topology == nil {
		return InterfaceInfo{}, false
	}
	ifaces := topology()
	for idx := range ifaces {
		if ifaces[idx].Name == name {
			return ifaces[idx], true
		}
	}
	return InterfaceInfo{}, false
}

func (d *LSDB) topologySnapshot() []InterfaceInfo {
	d.mu.RLock()
	topology := d.topology
	d.mu.RUnlock()
	if topology == nil {
		return nil
	}
	return topology()
}

// floodExcept floods the stored LSA out every eligible interface. incoming is the interface
// the LSA was received on (empty for self-originated floods) and sender is the neighbor it
// was received from. RFC 2328 §13.3 step 1(b,c): the LSA is normally NOT re-flooded out the
// receiving interface, EXCEPT when this router is the Designated Router on it and the LSA was
// received from a DROther (not the DR/BDR) -- the DR must re-flood to AllSPFRouters so the
// other DROthers on the segment receive it. The return value reports whether the LSA was
// flooded back out the receiving interface; RFC 2328 Table 19 uses that to suppress the ack.
func (d *LSDB) floodExcept(incoming string, sender types.RouterID, area types.AreaID, key types.LSAKey) bool {
	ifs := d.topologySnapshot()
	if len(ifs) == 0 {
		return false
	}
	var entry *Entry
	d.mu.RLock()
	store := d.dbForReadLocked(area, key)
	if store != nil {
		entry = store.entries[key]
	}
	d.mu.RUnlock()
	if entry == nil {
		return false
	}
	floodedBack := false
	for idx := range ifs {
		iface := &ifs[idx]
		onReceiving := incoming != "" && iface.Name == incoming
		nonBroadcast := isNonBroadcastNetwork(iface.NetworkType)
		if onReceiving && !nonBroadcast {
			// Re-flood out the receiving interface only as the DR receiving from a DROther
			// (not from the BDR); the BDR and DROthers never re-flood back (RFC 2328 §13.3).
			// NBMA and point-to-multipoint have no multicast DR relay, so the per-neighbor
			// unicast fan-out below reaches every OTHER adjacent neighbor (RFC 2328 §13.3
			// Table 19); there is no DR-relay suppression to apply.
			if iface.State != InterfaceStateDR || sender == iface.BDR {
				continue
			}
		}
		if !eligibleInterface(*iface, area, key.Type) {
			continue
		}
		lsa, ok := entry.LSA(d.now())
		if !ok {
			continue
		}
		raw := lsa.RawBytes
		queued := false
		var unicast []netip.Addr
		for _, nbr := range iface.Neighbors {
			if !isFloodEligibleNeighborState(nbr.State) || nbr.RouterID == (types.RouterID{}) {
				continue
			}
			// Never retransmit back to the neighbor the LSA was received from.
			if onReceiving && nbr.RouterID == sender {
				continue
			}
			// RFC 5250 Section 3.1: opaque LSAs are flooded ONLY to opaque-capable
			// neighbors (those that set the O-bit in their DD packets). Skip a
			// non-opaque neighbor for any opaque LSA (types 9/10/11).
			if key.Type.IsOpaque() && !nbr.OpaqueCapable {
				continue
			}
			if d.queueRetransmit(iface.AreaID, NeighborKey{Interface: iface.Name, RouterID: nbr.RouterID}, lsa.Header, raw) {
				queued = true
			}
			if nonBroadcast && nbr.Address.IsValid() {
				unicast = append(unicast, nbr.Address)
			}
		}
		if queued {
			if nonBroadcast {
				// Fan out one unicast copy per Flood-eligible neighbor (RFC 2328 §13.3);
				// the built LSA buffer is reused for each send.
				for _, dst := range unicast {
					d.sendLSUpdate(iface.Name, dst, iface.AreaID, []packet.LSA{lsa})
				}
			} else {
				d.sendLSUpdate(iface.Name, floodDestination(*iface), iface.AreaID, []packet.LSA{lsa})
			}
			if onReceiving {
				floodedBack = true
			}
		}
	}
	return floodedBack
}

// eligibleInterface is the send-side counterpart of shouldDropByArea: an ABR must not flood
// AS-External / ASBR-Summary into a stub or NSSA area, and an NSSA-LSA is flooded only within
// its own NSSA. AS-External is AS-wide (no area match); all others are area-scoped. The type
// classification is address-family-neutral (OSPFv2 Type 5/4/7 or OSPFv3 0x4005/0x2004/0x2007).
func eligibleInterface(iface InterfaceInfo, area types.AreaID, typ types.LSType) bool {
	stubLike := iface.AreaType == AreaTypeStub || iface.AreaType == AreaTypeNSSA
	switch {
	// RFC 5250 Section 3.1: Type-11 opaque flooding scope equals Type-5 AS-External --
	// AS-wide (no area match) but never out a stub/NSSA interface (isASWideType covers both).
	// Type-10 opaque is area-scoped and Type-9 opaque is link-scoped (floodLink), so only the
	// AS-wide types take this branch; Type-10 falls to the area-scoped default below.
	case isASWideType(typ):
		return !stubLike
	case typ.InterAreaRouter():
		return iface.AreaID == area && !stubLike
	case typ.NSSA():
		return iface.AreaID == area && iface.AreaType == AreaTypeNSSA
	default:
		return iface.AreaID == area
	}
}

func firstNonZero(a, b uint16) uint16 {
	if a != 0 {
		return a
	}
	return b
}

// allSPFRoutersV6 / allDRoutersV6 are the OSPFv3 link-local multicast groups (RFC 5340
// sec 2.9), the IPv6 equivalents of OSPFv2's 224.0.0.5 / 224.0.0.6.
var (
	allSPFRoutersV6 = netip.MustParseAddr("ff02::5")
	allDRoutersV6   = netip.MustParseAddr("ff02::6")
)

func floodDestination(iface InterfaceInfo) netip.Addr {
	toDRouters := iface.NetworkType == NetworkBroadcast &&
		iface.State != InterfaceStateDR && iface.State != InterfaceStateBackup
	if iface.IsV6 {
		if toDRouters {
			return allDRoutersV6
		}
		return allSPFRoutersV6
	}
	if toDRouters {
		return transport.AllDRouters
	}
	return transport.AllSPFRouters
}

func (d *LSDB) queueRetransmit(area types.AreaID, nbr NeighborKey, h packet.LSAHeader, raw []byte) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	lst := d.retransmit[nbr]
	if lst == nil {
		lst = make(map[types.LSAKey]*retransmitEntry)
		d.retransmit[nbr] = lst
	}
	key := h.Key()
	if lst[key] == nil && len(lst) >= MaxRetransmitPerNeighbor {
		return false
	}
	owned := make([]byte, len(raw))
	copy(owned, raw)
	// RFC 2328 §13.5: a flooded LSA is retransmitted every RxmtInterval until acknowledged;
	// the FIRST retransmission waits a full interval after the initial flood. Stamp the queue
	// time so RetransmitTick does not resend on its very next tick (last left zero would).
	lst[key] = &retransmitEntry{area: area, key: key, lsa: h, raw: owned, last: d.now()}
	return true
}

func (d *LSDB) clearRetransmit(nbr NeighborKey, area types.AreaID, h packet.LSAHeader) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	lst := d.retransmit[nbr]
	if lst == nil {
		return false
	}
	entry := lst[h.Key()]
	if entry == nil || entry.area != area || !sameInstance(h, entry.lsa) {
		return false
	}
	delete(lst, h.Key())
	if len(lst) == 0 {
		delete(d.retransmit, nbr)
	}
	return true
}

func sameInstance(a, b packet.LSAHeader) bool {
	return a.Key() == b.Key() && a.Sequence == b.Sequence && a.Checksum == b.Checksum && a.Length == b.Length
}

func (d *LSDB) removeFromAllRetransmit(area types.AreaID, key types.LSAKey) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for nbr, lst := range d.retransmit {
		if entry := lst[key]; entry != nil && (isASWideType(key.Type) || entry.area == area) {
			delete(lst, key)
		}
		if len(lst) == 0 {
			delete(d.retransmit, nbr)
		}
	}
}

// RetransmitTick resends every LSA still on retransmit lists whose interval is
// due. Tests call this directly; the engine runs it from its timer loop.
func (d *LSDB) RetransmitTick(now time.Time) int {
	ifs := d.topologySnapshot()
	ifaceByName := make(map[string]*InterfaceInfo, len(ifs))
	for idx := range ifs {
		ifaceByName[ifs[idx].Name] = &ifs[idx]
	}
	type send struct {
		iface string
		dst   netip.Addr
		area  types.AreaID
		lsa   packet.LSA
	}
	var sends []send
	d.mu.Lock()
	for nbr, lst := range d.retransmit {
		iface, ok := ifaceByName[nbr.Interface]
		if !ok || !hasFloodEligibleNeighbor(*iface, nbr.RouterID) {
			delete(d.retransmit, nbr)
			continue
		}
		interval := time.Duration(firstNonZero(iface.RetransmitInterval, 5)) * time.Second
		for key, entry := range lst {
			if !entry.last.IsZero() && now.Sub(entry.last) < interval {
				continue
			}
			raw := make([]byte, len(entry.raw))
			copy(raw, entry.raw)
			if len(raw) < types.LSAHeaderLen {
				delete(lst, key)
				continue
			}
			// Re-stamp the LS age and resend the stored raw verbatim. Do NOT decode the
			// body: the retransmit list spans every LSA family including OSPFv3 Link-LSAs
			// (type 0x0008), and routing raw through the OSPFv2 codec (packet.DecodeLSA)
			// fails on any v6 LSA and would silently drop it from the list, breaking RFC
			// 2328 sec 13.5 reliable flooding. The neutral header was captured at queue time
			// (entry.lsa); the encoders re-emit RawBytes verbatim, so no body decode is
			// needed (mirrors Entry.LSA and the flood-send path).
			age, _ := types.LSAgeFromBytes(raw[:2])
			age = age.Add(firstNonZero(iface.TransmitDelay, 1))
			age.WriteTo(raw, 0)
			hdr := entry.lsa
			hdr.Age = age
			lsa := packet.LSA{Header: hdr, Body: raw[types.LSAHeaderLen:], RawBytes: raw}
			entry.sent = true
			entry.last = now
			sends = append(sends, send{iface: nbr.Interface, dst: neighborAddr(*iface, nbr.RouterID), area: entry.area, lsa: lsa})
			d.mRetransmissions.With(areaLabel(entry.area)).Inc()
		}
	}
	d.mu.Unlock()
	for i := range sends {
		s := &sends[i]
		d.sendLSUpdate(s.iface, s.dst, s.area, []packet.LSA{s.lsa})
	}
	return len(sends)
}

func hasFloodEligibleNeighbor(iface InterfaceInfo, router types.RouterID) bool {
	for _, n := range iface.Neighbors {
		if n.RouterID == router && isFloodEligibleNeighborState(n.State) {
			return true
		}
	}
	return false
}

func isFloodEligibleNeighborState(state string) bool {
	switch state {
	case NeighborStateExchange, NeighborStateLoading, NeighborStateFull:
		return true
	default:
		return false
	}
}

func (d *LSDB) hasExchangeOrLoadingForKey(area types.AreaID, key types.LSAKey) bool {
	topology := d.topologySnapshot()
	for idx := range topology {
		iface := &topology[idx]
		if isASWideType(key.Type) {
			if !eligibleInterface(*iface, area, key.Type) {
				continue
			}
		} else if iface.AreaID != area {
			continue
		}
		for _, n := range iface.Neighbors {
			if n.State == NeighborStateExchange || n.State == NeighborStateLoading {
				return true
			}
		}
	}
	return false
}

func neighborAddr(iface InterfaceInfo, router types.RouterID) netip.Addr {
	for _, n := range iface.Neighbors {
		if n.RouterID == router && n.Address.IsValid() {
			return n.Address
		}
	}
	return floodDestination(iface)
}

// ackForReceive applies the RFC 2328 Table 19 acknowledgment decision. floodedBack is true
// when the LSA was flooded back out the receiving interface (§13.3) -- then no ack is sent.
// duplicate is true for a same-instance (Equal) LSA; impliedAck is true when that duplicate
// was on the receiving adjacency's retransmit list (step 7a, already cleared by the caller).
func (d *LSDB) ackForReceive(in ReceiveInput, h packet.LSAHeader, duplicate, impliedAck, floodedBack bool) {
	if floodedBack {
		return // Table 19: flooded back out the receiving interface -> no acknowledgment
	}
	iface, _ := d.interfaceByName(in.Interface)
	isBackup := iface.State == InterfaceStateBackup
	fromDR := in.RouterID == iface.DR
	switch {
	case duplicate && !impliedAck:
		// Duplicate not treated as an implied ack -> direct ack in every interface state.
		d.sendAck(in.Interface, in.Src, in.AreaID, []packet.LSAHeader{h})
	case duplicate:
		// Duplicate treated as an implied ack -> the Backup acks (delayed) only when the LSA
		// came from the DR; every other state sends no acknowledgment.
		if isBackup && fromDR {
			d.queueDelayedAck(in.Interface, h)
		}
	default:
		// More recent, not flooded back -> the Backup acks (delayed) only when from the DR;
		// every other state sends a delayed acknowledgment.
		if !isBackup || fromDR {
			d.queueDelayedAck(in.Interface, h)
		}
	}
}

func (d *LSDB) queueDelayedAck(iface string, h packet.LSAHeader) {
	d.mu.Lock()
	defer d.mu.Unlock()
	acks := d.delayedAck[iface]
	if acks == nil {
		acks = make(map[types.LSAKey]packet.LSAHeader)
		d.delayedAck[iface] = acks
	}
	if acks[h.Key()] == (packet.LSAHeader{}) && len(acks) >= MaxDelayedAcksPerIface {
		return
	}
	acks[h.Key()] = h
}

// FlushDelayedAcks sends coalesced delayed acknowledgements on interface.
func (d *LSDB) FlushDelayedAcks(ifaceName string) int {
	d.mu.Lock()
	acks := d.delayedAck[ifaceName]
	if len(acks) == 0 {
		d.mu.Unlock()
		return 0
	}
	headers := make([]packet.LSAHeader, 0, len(acks))
	for _, h := range acks {
		headers = append(headers, h)
	}
	delete(d.delayedAck, ifaceName)
	d.mu.Unlock()
	iface, ok := d.interfaceByName(ifaceName)
	if !ok {
		return 0
	}
	if isNonBroadcastNetwork(iface.NetworkType) {
		// NBMA / non-broadcast point-to-multipoint: acknowledge to each Flood-eligible
		// neighbor by unicast rather than to a multicast group (RFC 2328 §13.3).
		for _, nbr := range iface.Neighbors {
			if isFloodEligibleNeighborState(nbr.State) && nbr.Address.IsValid() {
				d.sendAck(ifaceName, nbr.Address, iface.AreaID, headers)
			}
		}
		return len(headers)
	}
	d.sendAck(ifaceName, floodDestination(iface), iface.AreaID, headers)
	return len(headers)
}

// isNonBroadcastNetwork reports whether an interface floods by per-neighbor unicast
// (NBMA and point-to-multipoint) rather than to a multicast group.
func isNonBroadcastNetwork(networkType string) bool {
	return networkType == NetworkNBMA || networkType == NetworkPointToMultipoint
}

// PacketEncoder encodes the LSDB's outgoing flooded LSUpdate and LSAck packets for the
// address family. The default (v4PacketEncoder) encodes OSPFv2 (ospf/packet); the engine
// injects an OSPFv3 encoder for the IPv6 family so flooded updates and acks go out as
// OSPFv3 -- a raw IPv6 socket rejects an OSPFv2 (version 2) packet.
type PacketEncoder interface {
	EncodeLSUpdate(routerID types.RouterID, areaID types.AreaID, u packet.LSUpdate) []byte
	EncodeLSAck(routerID types.RouterID, areaID types.AreaID, a packet.LSAck) []byte
}

// v4PacketEncoder is the OSPFv2 LSDB packet encoder (the default). instanceID is the
// engine's RFC 6549 OSPFv2 Instance ID, stamped into the flooded LSUpdate/LSAck common
// header (offset 14); 0 is the base instance and its bytes are identical to base OSPFv2.
type v4PacketEncoder struct {
	instanceID uint8
}

// NewV4PacketEncoder returns the OSPFv2 LSDB packet encoder for the given Instance ID
// (RFC 6549). The engine installs it via SetPacketEncoder for a non-base instance; the
// base instance uses the zero-value default.
func NewV4PacketEncoder(instanceID uint8) PacketEncoder {
	return v4PacketEncoder{instanceID: instanceID}
}

func (e v4PacketEncoder) EncodeLSUpdate(routerID types.RouterID, areaID types.AreaID, u packet.LSUpdate) []byte {
	p := packet.Packet{Header: packet.Header{RouterID: routerID, AreaID: areaID, InstanceID: e.instanceID}, LSUpdate: &u}
	buf := make([]byte, p.EncodedLen())
	p.WriteTo(buf, 0)
	return buf
}

func (e v4PacketEncoder) EncodeLSAck(routerID types.RouterID, areaID types.AreaID, a packet.LSAck) []byte {
	p := packet.Packet{Header: packet.Header{RouterID: routerID, AreaID: areaID, InstanceID: e.instanceID}, LSAck: &a}
	buf := make([]byte, p.EncodedLen())
	p.WriteTo(buf, 0)
	return buf
}

func (d *LSDB) packetEncoder() PacketEncoder {
	if d.encoder != nil {
		return d.encoder
	}
	return v4PacketEncoder{}
}

func (d *LSDB) sendDirectLSUpdate(iface string, dst netip.Addr, area types.AreaID, entry *Entry, transmitDelay uint16) {
	if !dst.IsValid() {
		return
	}
	lsa, ok := entry.LSA(d.now())
	if !ok {
		return
	}
	lsa.Header.Age = lsa.Header.Age.Add(transmitDelay)
	lsa.Header.Age.WriteTo(lsa.RawBytes, 0)
	d.sendLSUpdate(iface, dst, area, []packet.LSA{lsa})
}

func (d *LSDB) sendLSUpdate(iface string, dst netip.Addr, area types.AreaID, lsas []packet.LSA) {
	if len(lsas) == 0 || !dst.IsValid() {
		return
	}
	d.mu.RLock()
	tx := d.tx
	routerID := d.selfRouter
	enc := d.packetEncoder()
	d.mu.RUnlock()
	if tx == nil {
		return
	}
	buf := enc.EncodeLSUpdate(routerID, area, packet.LSUpdate{LSAs: lsas})
	_ = tx(iface, dst, buf)
	d.mUpdatesSent.With(iface).Inc()
}

func (d *LSDB) sendAck(iface string, dst netip.Addr, area types.AreaID, headers []packet.LSAHeader) {
	if len(headers) == 0 || !dst.IsValid() {
		return
	}
	d.mu.RLock()
	tx := d.tx
	routerID := d.selfRouter
	enc := d.packetEncoder()
	d.mu.RUnlock()
	if tx == nil {
		return
	}
	buf := enc.EncodeLSAck(routerID, area, packet.LSAck{Headers: headers})
	_ = tx(iface, dst, buf)
	d.mAcksSent.With(iface).Inc()
}

func (d *LSDB) deletePurgedIfAcked(area types.AreaID, key types.LSAKey) {
	retainForLoading := d.hasExchangeOrLoadingForKey(area, key)
	d.mu.Lock()
	store := d.dbForLocked(area, key)
	entry := store.entries[key]
	if entry == nil || !entry.purged {
		d.mu.Unlock()
		return
	}
	for _, lst := range d.retransmit {
		if entry := lst[key]; entry != nil && (isASWideType(key.Type) || entry.area == area) {
			d.mu.Unlock()
			return
		}
	}
	if retainForLoading {
		d.mu.Unlock()
		return
	}
	if entry.self {
		// Drop the own-sequence record once a self-LSA is purged AND removed (fully acked): the
		// instance is gone from the domain, so a later re-origination correctly restarts from
		// InitialSequenceNumber. This deletes the key rather than leaving a stale/zeroed entry,
		// matching the link-scope path's cleanup.
		delete(d.own[area], key)
	}
	delete(store.entries, key)
	store.rebuildSortedLocked()
	d.publishSizeMetricLocked(area, key.Type)
	d.mu.Unlock()
	d.notifyChange(area)
}
