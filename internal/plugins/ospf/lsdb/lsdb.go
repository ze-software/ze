// Design: docs/architecture/ospf/ospf-7-lsdb-flooding.md -- per-area OSPFv2 LSDB store.
// RFC 2328 Section 12: LSA identity and area scoping.

package lsdb

import (
	"sort"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// MaxLSAsPerArea bounds one area's LSDB against a hostile flood of distinct keys.
const MaxLSAsPerArea = 16384

// MaxASExternalLSAs bounds the AS-wide Type 5 store. It is a var (not a const) so the
// capacity-rejection path -- which only fires at 16384 entries, far too many to fill via
// the public API in a test -- can be exercised by lowering it; production never mutates it.
var MaxASExternalLSAs = 16384

// MaxOpaqueASLSAs bounds the AS-wide Type 11 opaque store against a hostile flood of
// distinct opaque keys (RFC 5250 §8 DoS bound). A var for the same test reason as
// MaxASExternalLSAs; production never mutates it.
var MaxOpaqueASLSAs = 16384

type areaDB struct {
	entries map[types.LSAKey]*Entry
	sorted  []types.LSAKey
}

func newAreaDB() *areaDB { return &areaDB{entries: make(map[types.LSAKey]*Entry)} }

func (d *areaDB) rebuildSortedLocked() {
	keys := make([]types.LSAKey, 0, len(d.entries))
	for k := range d.entries {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Less(keys[j]) })
	d.sorted = keys
}

// LSDB stores OSPF LSAs. Area-scoped LSAs live in their area DB, Type 5
// AS-External LSAs live once in the AS-wide DB, and OSPFv3 Link-LSAs live in a
// per-interface link-scope DB keyed by the local interface name.
type LSDB struct {
	mu sync.RWMutex

	areas      map[types.AreaID]*areaDB
	asExternal *areaDB
	// asOpaque is the AS-wide (Type 11) opaque-LSA store, parallel to asExternal.
	// RFC 5250 §3.1: Type-11 opaque LSAs flood AS-wide like Type-5 AS-External-LSAs, so
	// they live once in a single AS-wide store, not per-area.
	asOpaque  *areaDB
	links     map[string]*areaDB
	linkAreas map[string]types.AreaID
	areaTypes map[types.AreaID]string
	// nssaTranslator[area] is true when this router advertises the RFC 3101 Nt-bit for
	// that NSSA (it is an attached NSSA whose translate role is not `never`); combined
	// with ABR status at origination time to set the Router-LSA Nt-bit.
	nssaTranslator map[types.AreaID]bool
	now            func() time.Time

	selfRouter types.RouterID
	timers     TimerConfig
	topology   TopologyFunc
	tx         TxFunc
	encoder    PacketEncoder
	onChange   func(types.AreaID)
	// selfFlushSuppress (OSPF Graceful Restart, RFC 3623 sec 2) suppresses the fight-back
	// flush of received self-LSAs while the restarting router is in-restart, so its
	// pre-restart LSAs are kept. contentChangeObserver notifies OSPF GR of each content-
	// changing install for the helper's sec 3.2 strict-LSA-checking exit. Both nil until the
	// engine wires them; the store/flood path never depends on them.
	selfFlushSuppress     func() bool
	contentChangeObserver func(types.AreaID, types.LSType)
	// opaqueDelivery is invoked (outside the lock) on every newer opaque-LSA install so
	// the engine can deliver it to the registered consumer (RFC 5250 §3). nil until the
	// engine wires it; the store/flood path never depends on it.
	opaqueDelivery func(OpaqueDelivery)

	retransmit  map[NeighborKey]map[types.LSAKey]*retransmitEntry
	delayedAck  map[string]map[types.LSAKey]packet.LSAHeader
	arrival     map[types.AreaID]map[types.LSAKey]time.Time
	linkArrival map[string]map[types.LSAKey]time.Time
	own         map[types.AreaID]map[types.LSAKey]ownRecord
	linkOwn     map[string]map[types.LSAKey]ownRecord

	mLSAs            metrics.GaugeVec
	mOriginations    metrics.CounterVec
	mRefreshes       metrics.CounterVec
	mPurges          metrics.CounterVec
	mUpdatesSent     metrics.CounterVec
	mUpdatesReceived metrics.CounterVec
	mAcksSent        metrics.CounterVec
	mRetransmissions metrics.CounterVec
}

// TimerConfig carries the RFC fixed timers plus per-interface defaults. Tests can
// shorten these without sleeping.
type TimerConfig struct {
	MinLSArrival  time.Duration
	MinLSInterval time.Duration
}

// defaultTimers returns RFC 2328 default timer values for LSDB operations.
func defaultTimers() TimerConfig {
	return TimerConfig{
		MinLSArrival:  time.Second,
		MinLSInterval: 5 * time.Second,
	}
}

// New constructs an empty OSPFv2 LSDB. now may be nil, in which case time.Now is
// used. Metrics are no-ops until SetMetrics is called.
func New(now func() time.Time) *LSDB {
	if now == nil {
		now = time.Now
	}
	nop := metrics.NopRegistry{}
	return &LSDB{
		areas:            make(map[types.AreaID]*areaDB),
		asExternal:       newAreaDB(),
		asOpaque:         newAreaDB(),
		links:            make(map[string]*areaDB),
		linkAreas:        make(map[string]types.AreaID),
		areaTypes:        make(map[types.AreaID]string),
		nssaTranslator:   make(map[types.AreaID]bool),
		now:              now,
		timers:           defaultTimers(),
		retransmit:       make(map[NeighborKey]map[types.LSAKey]*retransmitEntry),
		delayedAck:       make(map[string]map[types.LSAKey]packet.LSAHeader),
		arrival:          make(map[types.AreaID]map[types.LSAKey]time.Time),
		linkArrival:      make(map[string]map[types.LSAKey]time.Time),
		own:              make(map[types.AreaID]map[types.LSAKey]ownRecord),
		linkOwn:          make(map[string]map[types.LSAKey]ownRecord),
		mLSAs:            nop.GaugeVec("", "", nil),
		mOriginations:    nop.CounterVec("", "", nil),
		mRefreshes:       nop.CounterVec("", "", nil),
		mPurges:          nop.CounterVec("", "", nil),
		mUpdatesSent:     nop.CounterVec("", "", nil),
		mUpdatesReceived: nop.CounterVec("", "", nil),
		mAcksSent:        nop.CounterVec("", "", nil),
		mRetransmissions: nop.CounterVec("", "", nil),
	}
}

// SetSelfRouterID records the local Router ID for self-originated LSA handling.
func (d *LSDB) SetSelfRouterID(id types.RouterID) {
	d.mu.Lock()
	d.selfRouter = id
	d.mu.Unlock()
}

// SetTimers replaces LSDB timers. Zero fields keep the current/default value.
func (d *LSDB) SetTimers(t TimerConfig) {
	d.mu.Lock()
	if t.MinLSArrival > 0 {
		d.timers.MinLSArrival = t.MinLSArrival
	}
	if t.MinLSInterval > 0 {
		d.timers.MinLSInterval = t.MinLSInterval
	}
	d.mu.Unlock()
}

// SetAreaTypes records config area kinds so AS-external LSAs stay hidden from
// stub and NSSA areas even when Type 5 LSAs exist in the AS-wide store.
func (d *LSDB) SetAreaTypes(areaTypes map[types.AreaID]string) {
	d.mu.Lock()
	d.areaTypes = make(map[types.AreaID]string, len(areaTypes))
	for area, kind := range areaTypes {
		if kind == "" {
			kind = AreaTypeNormal
		}
		d.areaTypes[area] = kind
	}
	d.mu.Unlock()
}

// SetNSSATranslatorAreas records the NSSA areas for which this router advertises the
// RFC 3101 Nt-bit (attached NSSAs whose translate role is not `never`). At Router-LSA
// origination the bit is set only when this router is also an ABR for the area.
func (d *LSDB) SetNSSATranslatorAreas(areas map[types.AreaID]bool) {
	d.mu.Lock()
	d.nssaTranslator = make(map[types.AreaID]bool, len(areas))
	for area, on := range areas {
		if on {
			d.nssaTranslator[area] = true
		}
	}
	d.mu.Unlock()
}

// isNSSATranslatorArea reports whether this router advertises the Nt-bit for area.
func (d *LSDB) isNSSATranslatorArea(area types.AreaID) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.nssaTranslator[area]
}

// SetTopology wires a live topology snapshot provider used by origination and
// flooding. The LSDB stores only values returned by the function.
func (d *LSDB) SetTopology(fn TopologyFunc) {
	d.mu.Lock()
	d.topology = fn
	d.mu.Unlock()
}

// SetTx wires OSPF packet transmission. The transport owns raw sockets; LSDB only
// builds packet bytes and asks for a send.
func (d *LSDB) SetTx(tx TxFunc) {
	d.mu.Lock()
	d.tx = tx
	d.mu.Unlock()
}

// SetPacketEncoder installs the address-family encoder for flooded LSUpdate / LSAck packets.
// The engine calls this for the OSPFv3 family; OSPFv2 uses the default (nil -> v4PacketEncoder).
func (d *LSDB) SetPacketEncoder(enc PacketEncoder) {
	d.mu.Lock()
	d.encoder = enc
	d.mu.Unlock()
}

// SetOnChange wires the SPF trigger. The callback runs after the LSDB lock is
// released and may re-enter the LSDB through the SPF Source interface.
func (d *LSDB) SetOnChange(fn func(types.AreaID)) {
	d.mu.Lock()
	d.onChange = fn
	d.mu.Unlock()
}

func (d *LSDB) notifyChange(area types.AreaID) {
	d.mu.RLock()
	fn := d.onChange
	d.mu.RUnlock()
	if fn != nil {
		fn(area)
	}
}

// SetSelfFlushSuppress wires the OSPF Graceful Restart self-flush suppression predicate
// (RFC 3623 sec 2): while it returns true, a received self-originated LSA the restarting
// router holds no local record of is NOT fight-back-flushed; it is accepted as valid so the
// router keeps its pre-restart LSAs across the restart.
func (d *LSDB) SetSelfFlushSuppress(fn func() bool) {
	d.mu.Lock()
	d.selfFlushSuppress = fn
	d.mu.Unlock()
}

// selfFlushSuppressed reports whether the GR self-flush suppression is active.
func (d *LSDB) selfFlushSuppressed() bool {
	d.mu.RLock()
	fn := d.selfFlushSuppress
	d.mu.RUnlock()
	return fn != nil && fn()
}

// SetContentChangeObserver wires an observer invoked (outside the lock) on every content-
// changing (Newer) LSA install with the changed LSA's area and type. OSPF Graceful Restart
// uses it to drive the helper's RFC 3623 sec 3.2 strict-LSA-checking exit.
func (d *LSDB) SetContentChangeObserver(fn func(types.AreaID, types.LSType)) {
	d.mu.Lock()
	d.contentChangeObserver = fn
	d.mu.Unlock()
}

func (d *LSDB) notifyContentChange(area types.AreaID, lsType types.LSType) {
	d.mu.RLock()
	fn := d.contentChangeObserver
	d.mu.RUnlock()
	if fn != nil {
		fn(area, lsType)
	}
}

// Metric label names for the OSPF LSDB and flooding series.
const (
	labelArea      = "area"
	labelInterface = "interface"
	labelLSAType   = "type"
)

// SetMetrics registers the OSPF LSDB/flooding series owned by spec ospf-7.
func (d *LSDB) SetMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	d.mu.Lock()
	d.mLSAs = reg.GaugeVec("ze_ospf_lsdb_lsas", "Current OSPF LSAs in the link-state database, by area and LSA type.", []string{labelArea, labelLSAType})
	d.mOriginations = reg.CounterVec("ze_ospf_lsa_originations_total", "Total OSPF self-originated LSAs, by LSA type.", []string{labelLSAType})
	d.mRefreshes = reg.CounterVec("ze_ospf_lsa_refreshes_total", "Total OSPF self-originated LSA refreshes, by LSA type.", []string{labelLSAType})
	d.mPurges = reg.CounterVec("ze_ospf_lsa_purges_total", "Total OSPF LSAs purged at MaxAge, by LSA type.", []string{labelLSAType})
	d.mUpdatesSent = reg.CounterVec("ze_ospf_lsupdates_sent_total", "Total OSPF Link State Update packets sent by flooding, by interface.", []string{labelInterface})
	d.mUpdatesReceived = reg.CounterVec("ze_ospf_lsupdates_received_total", "Total OSPF Link State Update packets received by flooding, by interface.", []string{labelInterface})
	d.mAcksSent = reg.CounterVec("ze_ospf_lsacks_sent_total", "Total OSPF Link State Acknowledgment packets sent by flooding, by interface.", []string{labelInterface})
	d.mRetransmissions = reg.CounterVec("ze_ospf_retransmissions_total", "Total OSPF LSA retransmissions by area.", []string{labelArea})
	d.mu.Unlock()
	d.publishAllSizeMetrics()
}

// dbForLocked routes an LSA key to its store by flooding scope. RFC 5250 §3.1: a
// Type-11 opaque LSA is AS-wide (its own store, parallel to Type-5 AS-External); a
// Type-10 opaque LSA is area-scoped (the per-area store, the default branch); a Type-9
// opaque LSA is link-scoped and never reaches here -- it routes through installLink like
// the OSPFv3 Type-8 Link-LSA. Type-11 has no scope bits, so ASExternal() is false for it
// and the explicit opaque-AS branch must precede the default per-area routing.
func (d *LSDB) dbForLocked(area types.AreaID, key types.LSAKey) *areaDB {
	switch {
	case key.Type == types.LSTypeOpaqueAS:
		return d.asOpaque
	case key.Type.ASWide():
		return d.asExternal
	default:
		return d.areaForLocked(area)
	}
}

func (d *LSDB) dbForReadLocked(area types.AreaID, key types.LSAKey) *areaDB {
	switch {
	case key.Type == types.LSTypeOpaqueAS:
		return d.asOpaque
	case key.Type.ASWide():
		return d.asExternal
	default:
		return d.areas[area]
	}
}

func (d *LSDB) areaTypeLocked(area types.AreaID) string {
	kind := d.areaTypes[area]
	if kind == "" {
		return AreaTypeNormal
	}
	return kind
}

func (d *LSDB) areaForLocked(area types.AreaID) *areaDB {
	adb := d.areas[area]
	if adb == nil {
		adb = newAreaDB()
		d.areas[area] = adb
	}
	return adb
}

// Install validates and stores an LSA. It is idempotent for equal or older LSAs
// so the neighbor loading path can call it after the flooding path installed the
// same instance.
func (d *LSDB) Install(area types.AreaID, lsa packet.LSA) bool {
	if isLinkLSAType(lsa.Header.Type) {
		// Link-scoped LSAs (OSPFv3 Type 0x0008) live in the per-interface link store, not an
		// area DB; they must be installed via installLink. Reject here so a misrouted caller
		// cannot land a Link-LSA in dbForLocked's area store.
		return false
	}
	res, ok := d.install(area, lsa, false, false)
	if ok && res.Stored {
		d.notifyChange(area)
	}
	return ok
}

type installResult struct {
	Freshness Freshness
	Stored    bool
	Entry     *Entry
	Previous  *Entry
}

func (d *LSDB) install(area types.AreaID, lsa packet.LSA, self, enforceMinArrival bool) (installResult, bool) {
	raw, h, ok := normaliseLSA(lsa)
	if !ok {
		return installResult{Freshness: Older}, false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.installLocked(area, raw, h, self, enforceMinArrival)
}

// installLocked is install with d.mu already held by the caller. It exists so a
// caller that must atomically install and then mutate the resulting *Entry (e.g.
// installOriginated marking a purge) can hold the lock across both steps; otherwise
// the Entry field write races readers such as SelfExternalCount / Tick that read the
// same fields under the lock.
func (d *LSDB) installLocked(area types.AreaID, raw []byte, h packet.LSAHeader, self, enforceMinArrival bool) (installResult, bool) {
	now := d.now()
	key := h.Key()
	store := d.dbForLocked(area, key)
	existing := store.entries[key]
	if existing != nil {
		fr := CompareHeaders(h, existing.Header(now))
		if fr == Older || fr == Equal {
			return installResult{Freshness: fr, Entry: existing, Previous: existing}, true
		}
		if enforceMinArrival && d.arrivedTooSoonLocked(area, key, now) {
			return installResult{Freshness: Equal, Entry: existing, Previous: existing}, true
		}
		entry := newEntry(h, raw, now, self)
		store.entries[key] = entry
		d.noteArrivalLocked(area, key, now)
		return installResult{Freshness: Newer, Stored: true, Entry: entry, Previous: existing}, true
	}
	limit := MaxLSAsPerArea
	switch {
	case key.Type == types.LSTypeOpaqueAS:
		limit = MaxOpaqueASLSAs
	case key.Type.ASWide():
		limit = MaxASExternalLSAs
	}
	if len(store.entries) >= limit {
		return installResult{Freshness: Older}, false
	}
	if enforceMinArrival && d.arrivedTooSoonLocked(area, key, now) {
		return installResult{Freshness: Equal}, true
	}
	entry := newEntry(h, raw, now, self)
	store.entries[key] = entry
	store.rebuildSortedLocked()
	d.noteArrivalLocked(area, key, now)
	d.publishSizeMetricLocked(area, key.Type)
	return installResult{Freshness: Newer, Stored: true, Entry: entry}, true
}

func normaliseLSA(lsa packet.LSA) ([]byte, packet.LSAHeader, bool) {
	if len(lsa.RawBytes) == 0 || lsa.Router != nil || lsa.Network != nil || lsa.Summary != nil || lsa.External != nil || lsa.Opaque != nil {
		buf := make([]byte, lsa.EncodedLen())
		lsa.WriteTo(buf, 0)
		decoded, err := packet.DecodeLSA(buf)
		if err != nil || !decoded.VerifyChecksum() {
			return nil, packet.LSAHeader{}, false
		}
		return buf, decoded.Header, true
	}
	raw := make([]byte, len(lsa.RawBytes))
	copy(raw, lsa.RawBytes)
	// A received LSA (no typed body) carries the header the codec already decoded.
	// Trust it and verify the Fletcher checksum on the raw bytes directly, rather
	// than re-decoding: this path is address-family-agnostic, whereas re-decoding via
	// the OSPFv2 packet.DecodeLSA would misparse an OSPFv3 LSA (its 16-bit scope-typed
	// LS Type sits where OSPFv2 has the Options + LS Type octets). The LSA Fletcher
	// checksum is byte-identical across OSPFv2 and OSPFv3 (RFC 5340 sec A.4.2.1).
	if !packet.VerifyLSAChecksum(raw) {
		return nil, packet.LSAHeader{}, false
	}
	return raw, lsa.Header, true
}

func (d *LSDB) arrivedTooSoonLocked(area types.AreaID, key types.LSAKey, now time.Time) bool {
	arrivals := d.arrival[area]
	if arrivals == nil {
		return false
	}
	last, ok := arrivals[key]
	return ok && now.Sub(last) < d.timers.MinLSArrival
}

func (d *LSDB) noteArrivalLocked(area types.AreaID, key types.LSAKey, now time.Time) {
	arrivals := d.arrival[area]
	if arrivals == nil {
		arrivals = make(map[types.LSAKey]time.Time)
		d.arrival[area] = arrivals
	}
	arrivals[key] = now
}

// Lookup returns the current header for key.
func (d *LSDB) Lookup(area types.AreaID, key types.LSAKey) (packet.LSAHeader, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if key.Type.ASWide() && shouldDropByArea(d.areaTypeLocked(area), key.Type) {
		return packet.LSAHeader{}, false
	}
	store := d.dbForReadLocked(area, key)
	if store == nil {
		return packet.LSAHeader{}, false
	}
	entry := store.entries[key]
	if entry == nil {
		return packet.LSAHeader{}, false
	}
	return entry.Header(d.now()), true
}

// LookupLSA returns a lazy LSA view backed by an owned raw-byte copy.
func (d *LSDB) LookupLSA(area types.AreaID, key types.LSAKey) (packet.LSA, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if key.Type.ASWide() && shouldDropByArea(d.areaTypeLocked(area), key.Type) {
		return packet.LSA{}, false
	}
	store := d.dbForReadLocked(area, key)
	if store == nil {
		return packet.LSA{}, false
	}
	entry := store.entries[key]
	if entry == nil {
		return packet.LSA{}, false
	}
	return entry.LSA(d.now())
}

// Summary returns sorted LSA headers visible to the given area. Type 5 AS-wide
// LSAs, then Type 11 AS-wide opaque LSAs, are appended after area-scoped entries.
// Type 10 opaque LSAs already live in the per-area store, so they appear in the
// area-scoped section; Type 9 opaque LSAs are link-scoped and summarized per interface.
func (d *LSDB) Summary(area types.AreaID) []packet.LSAHeader {
	d.mu.RLock()
	defer d.mu.RUnlock()
	now := d.now()
	areaDB := d.areas[area]
	includeExternal := !shouldDropByArea(d.areaTypeLocked(area), types.LSTypeASExternal)
	// RFC 5250 §3.1: Type-11 opaque LSAs share Type-5's AS-wide scope, so they are
	// excluded from a stub/NSSA area's database summary exactly as Type 5 is.
	includeOpaqueAS := !shouldDropByArea(d.areaTypeLocked(area), types.LSTypeOpaqueAS)
	capacity := 0
	if includeExternal {
		capacity += len(d.asExternal.entries)
	}
	if includeOpaqueAS {
		capacity += len(d.asOpaque.entries)
	}
	if areaDB != nil {
		capacity += len(areaDB.entries)
	}
	out := make([]packet.LSAHeader, 0, capacity)
	if areaDB != nil {
		for _, key := range areaDB.sorted {
			out = append(out, areaDB.entries[key].Header(now))
		}
	}
	if includeExternal {
		for _, key := range d.asExternal.sorted {
			out = append(out, d.asExternal.entries[key].Header(now))
		}
	}
	if includeOpaqueAS {
		for _, key := range d.asOpaque.sorted {
			out = append(out, d.asOpaque.entries[key].Header(now))
		}
	}
	return out
}

// Delete removes an LSA from the relevant store.
func (d *LSDB) Delete(area types.AreaID, key types.LSAKey) bool {
	d.mu.Lock()
	store := d.dbForLocked(area, key)
	if store.entries[key] == nil {
		d.mu.Unlock()
		return false
	}
	delete(store.entries, key)
	store.rebuildSortedLocked()
	d.publishSizeMetricLocked(area, key.Type)
	d.mu.Unlock()
	d.notifyChange(area)
	return true
}

// Snapshot returns a stable CLI/API view of all LSAs.
func (d *LSDB) Snapshot() Snapshot {
	d.mu.RLock()
	defer d.mu.RUnlock()
	now := d.now()
	areas := make([]AreaSnapshot, 0, len(d.areas))
	areaIDs := make([]types.AreaID, 0, len(d.areas))
	for area := range d.areas {
		areaIDs = append(areaIDs, area)
	}
	sort.Slice(areaIDs, func(i, j int) bool { return compareAreaID(areaIDs[i], areaIDs[j]) < 0 })
	for _, area := range areaIDs {
		areas = append(areas, AreaSnapshot{Area: area, LSAs: snapshotEntries(d.areas[area], now, "", false)})
	}
	linkNames := make([]string, 0, len(d.links))
	for name := range d.links {
		linkNames = append(linkNames, name)
	}
	sort.Strings(linkNames)
	links := make([]LinkSnapshot, 0, len(linkNames))
	for _, name := range linkNames {
		links = append(links, LinkSnapshot{Interface: name, LSAs: snapshotEntries(d.links[name], now, name, true)})
	}
	return Snapshot{
		Areas:      areas,
		ASExternal: snapshotEntries(d.asExternal, now, "", false),
		ASOpaque:   snapshotEntries(d.asOpaque, now, "", false),
		Links:      links,
	}
}

func snapshotEntries(store *areaDB, now time.Time, iface string, linkScope bool) []LSASnapshot {
	out := make([]LSASnapshot, 0, len(store.entries))
	for _, key := range store.sorted {
		entry := store.entries[key]
		h := entry.Header(now)
		row := LSASnapshot{
			Type:              h.Type.String(),
			LinkStateID:       h.LinkStateID.String(),
			AdvertisingRouter: h.AdvertisingRouter.String(),
			Sequence:          h.Sequence.String(),
			Age:               h.Age.Age(),
			Checksum:          h.Checksum,
			Length:            h.Length,
		}
		if linkScope {
			row.Interface = iface
			// Only the OSPFv3 Type-8 Link-LSA carries a link-local address at this
			// offset; a Type-9 opaque LSA shares the per-interface store but its body is
			// an opaque payload, so do not misread its bytes as an address.
			if h.Type == types.LSTypeLink {
				if ll, ok := linkLocalFromRaw(entry.raw); ok {
					row.LinkLocalAddress = ll.String()
				}
			}
		}
		out = append(out, row)
	}
	return out
}

// Snapshot is the show/API view consumed by later CLI work.
type Snapshot struct {
	Areas      []AreaSnapshot `json:"areas"`
	ASExternal []LSASnapshot  `json:"as-external"`
	// ASOpaque holds the AS-wide (Type 11) opaque LSAs (RFC 5250 §3.1).
	ASOpaque []LSASnapshot  `json:"as-opaque,omitempty"`
	Links    []LinkSnapshot `json:"links,omitempty"`
}

// AreaSnapshot is one area-scoped LSDB snapshot.
type AreaSnapshot struct {
	Area types.AreaID  `json:"area"`
	LSAs []LSASnapshot `json:"lsas"`
}

// LinkSnapshot is one interface-scoped OSPFv3 Link-LSA database snapshot.
type LinkSnapshot struct {
	Interface string        `json:"interface"`
	LSAs      []LSASnapshot `json:"lsas"`
}

// LSASnapshot is the thin metadata rendered by show ospf database.
type LSASnapshot struct {
	Type              string `json:"type"`
	LinkStateID       string `json:"link-state-id"`
	AdvertisingRouter string `json:"advertising-router"`
	Sequence          string `json:"sequence"`
	Age               uint16 `json:"age"`
	Checksum          uint16 `json:"checksum"`
	Length            uint16 `json:"length"`
	Interface         string `json:"interface,omitempty"`
	LinkLocalAddress  string `json:"link-local-address,omitempty"`
}

func (d *LSDB) publishAllSizeMetrics() {
	d.mu.RLock()
	areas := make(map[types.AreaID][]types.LSType, len(d.areas))
	for area, db := range d.areas {
		seen := make(map[types.LSType]struct{})
		for key := range db.entries {
			seen[key.Type] = struct{}{}
		}
		for typ := range seen {
			areas[area] = append(areas[area], typ)
		}
	}
	// The AS-wide store holds AS-External-LSAs (function code 5) and now also any other
	// AS-scope native LSA (the RFC 7770 Router Information LSA, function code 12), so count
	// only the true externals for the as-external gauge; RI has its own ze_ospf_ri_lsas series.
	asCount := 0
	for key := range d.asExternal.entries {
		if key.Type.ASExternal() {
			asCount++
		}
	}
	opaqueASCount := len(d.asOpaque.entries)
	d.mu.RUnlock()
	for area, typesForArea := range areas {
		for _, typ := range typesForArea {
			d.publishSizeMetric(area, typ)
		}
	}
	if asCount > 0 {
		d.mLSAs.With("as", types.LSTypeASExternal.String()).Set(float64(asCount))
	}
	if opaqueASCount > 0 {
		d.mLSAs.With("as", types.LSTypeOpaqueAS.String()).Set(float64(opaqueASCount))
	}
}

func (d *LSDB) publishSizeMetricLocked(area types.AreaID, typ types.LSType) {
	areaLabel := area.String()
	count := 0
	switch typ {
	case types.LSTypeASExternal:
		areaLabel = "as"
		for key := range d.asExternal.entries {
			if key.Type == typ {
				count++
			}
		}
	case types.LSTypeOpaqueAS:
		// Type-11 opaque LSAs live in the AS-wide opaque store, counted under the "as" label.
		areaLabel = "as"
		for key := range d.asOpaque.entries {
			if key.Type == typ {
				count++
			}
		}
	default:
		for key := range d.areaForLocked(area).entries {
			if key.Type == typ {
				count++
			}
		}
	}
	d.mLSAs.With(areaLabel, typ.String()).Set(float64(count))
}

func (d *LSDB) publishSizeMetric(area types.AreaID, typ types.LSType) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	d.publishSizeMetricLocked(area, typ)
}

func areaLabel(area types.AreaID) string { return area.String() }
