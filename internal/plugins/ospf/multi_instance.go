// Design: plan/learned/1036-ospf-ext-12-multi-instance.md -- OSPFv2 Multi-Instance (RFC 6549):
// one full OSPFv2 engine per configured Instance ID, demuxed by the shared dispatcher rule.
// Related: instance.go -- the per-engine skeleton this file drives one-per-instance.
// Related: config.go -- ospfConfig.instanceIDSet / forInstance derive the per-instance set.
// RFC: rfc/short/rfc6549.md
package ospf

import (
	"fmt"
	"maps"
	"slices"
	"sync"

	"github.com/ze-software/ze/internal/core/metrics"
	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	ospfneighbor "github.com/ze-software/ze/internal/plugins/ospf/neighbor"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
)

// dropReasonInstanceMismatch labels a transport drop for a packet whose Instance ID did
// not match the receiving engine's configured Instance ID (RFC 6549 sec 2 / 3.1).
const dropReasonInstanceMismatch = "instance-mismatch"

// unknownInterface is the metric label used when a dropped packet's ifindex cannot be
// resolved to an interface name (no drop is recorded against the transport in that case).
const unknownInterface = "unknown"

// installInstanceEncoders stamps this engine's Instance ID into its transmit encoders so
// every outgoing DD/LSReq/LSUpdate/LSAck carries it (RFC 6549 sec 3 on transmit). The
// Hello encoder is set from iface.Config.InstanceID in startInterfaceLocked. The OSPFv3
// family always swaps in the v6 encoder; the OSPFv2 base instance (0) keeps the default
// encoders (byte-for-byte identical output), so single-instance OSPFv2 is unchanged.
func (e *engine) installInstanceEncoders(instanceID uint8) {
	if e.dispatch == nil {
		return
	}
	switch {
	case e.dispatch.codec.IsV6():
		if e.neighbors != nil {
			e.neighbors.SetEncoder(v6Encoder{instanceID: instanceID, emitAF: e.emitAFBit()})
		}
		if e.lsdb != nil {
			e.lsdb.SetPacketEncoder(v6Encoder{instanceID: instanceID, emitAF: e.emitAFBit()})
		}
	case instanceID != 0:
		if e.neighbors != nil {
			e.neighbors.SetEncoder(ospfneighbor.NewV4Encoder(instanceID))
		}
		if e.lsdb != nil {
			e.lsdb.SetPacketEncoder(ospflsdb.NewV4PacketEncoder(instanceID))
		}
	}
}

// instanceIDForV4 returns the OSPFv2 Interface Instance ID to stamp into Hellos: the
// engine's Instance ID for the IPv4 family, 0 for the IPv6 family (whose Instance ID is
// threaded through the v6 encoder, not the OSPFv2 header).
func instanceIDForV4(d *dispatcher, instanceID uint8) uint8 {
	if d != nil && d.codec.IsV6() {
		return 0
	}
	return instanceID
}

// recordInstanceMismatch increments ze_ospf_instance_mismatch_drops_total{interface} and
// records the transport drop when the dispatcher discards a packet for an Instance ID that
// does not match this engine (RFC 6549 sec 2 / 3.1 demux). It runs before any handler.
func (e *engine) recordInstanceMismatch(rp transport.RawPacket) {
	name := unknownInterface
	if e.transport != nil {
		if n, ok := e.transport.InterfaceNameByIfIndex(rp.IfIndex); ok {
			name = n
		}
	}
	e.mInstanceMismatch.With(name).Inc()
	if e.transport != nil && name != unknownInterface {
		e.transport.RecordDrop(name, dropReasonInstanceMismatch)
	}
}

// instanceSummaryView is one row of `show ospf instance`: the per-instance engine's
// Instance ID and the size of its (isolated) area/interface/neighbor/LSDB state.
type instanceSummaryView struct {
	InstanceID     uint8  `json:"instance-id"`
	RouterID       string `json:"router-id"`
	AreaCount      int    `json:"area-count"`
	InterfaceCount int    `json:"interface-count"`
	NeighborCount  int    `json:"neighbor-count"`
	LSACount       int    `json:"lsa-count"`
}

// instanceSummary renders this engine as an `show ospf instance` row for Instance ID id.
func (e *engine) instanceSummary(id uint8) instanceSummaryView {
	e.mu.Lock()
	cfg := e.cfg
	interfaceCount := len(e.cfg.Interfaces)
	e.mu.Unlock()
	neighborCount := 0
	if e.neighbors != nil {
		neighborCount = len(e.neighbors.Snapshot())
	}
	return instanceSummaryView{
		InstanceID:     id,
		RouterID:       cfg.RouterID.String(),
		AreaCount:      len(cfg.Areas),
		InterfaceCount: interfaceCount,
		NeighborCount:  neighborCount,
		LSACount:       e.lsdbLSACount(),
	}
}

// lsdbLSACount totals the LSAs across this engine's isolated database (area-scoped,
// AS-external, and OSPFv3 link-scoped), 0 when the engine has no LSDB.
func (e *engine) lsdbLSACount() int {
	if e.lsdb == nil {
		return 0
	}
	snap := e.lsdb.Snapshot()
	count := len(snap.ASExternal)
	for _, a := range snap.Areas {
		count += len(a.LSAs)
	}
	for _, l := range snap.Links {
		count += len(l.LSAs)
	}
	return count
}

// instanceManager owns the set of OSPFv2 engines, one per configured Instance ID (RFC
// 6549). The base instance 0 engine is always present and additionally owns redistribution
// and default-origination on the base IPv4 unicast table; non-zero instances run the core
// link-state protocol demuxed by their Instance ID on their own transport. It reconciles
// the set on every config change (add new instances, tear down removed ones).
type instanceManager struct {
	build      func(id uint8) *engine
	mInstances metrics.Gauge

	mu      sync.Mutex
	engines map[uint8]*engine
	started bool
}

// newInstanceManager seeds the manager with the pre-wired base (Instance 0) engine and a
// builder for non-zero instances. mInstances (ze_ospf_instances) may be nil in tests.
func newInstanceManager(base *engine, build func(id uint8) *engine, mInstances metrics.Gauge) *instanceManager {
	if mInstances == nil {
		mInstances = metrics.NopRegistry{}.Gauge("", "")
	}
	m := &instanceManager{
		build:      build,
		mInstances: mInstances,
		engines:    map[uint8]*engine{0: base},
	}
	m.mInstances.Set(1)
	return m
}

// engineFor returns the engine bound to Instance ID id, if the manager holds one.
func (m *instanceManager) engineFor(id uint8) (*engine, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.engines[id]
	return e, ok
}

// ensureSet reconciles engine-set membership against the wanted Instance IDs: it builds a
// fresh engine for each new ID and marks engines no longer wanted for teardown (never the
// base 0). It returns the newly created IDs (which still need setConfig/openInterfaces) and
// whether the manager has started. Removed engines are shut down after the lock is dropped.
func (m *instanceManager) ensureSet(ids []uint8) (created []uint8, started bool) {
	want := make(map[uint8]struct{}, len(ids))
	for _, id := range ids {
		want[id] = struct{}{}
	}
	m.mu.Lock()
	var removed []*engine
	for id, eng := range m.engines {
		if id == 0 {
			continue
		}
		if _, keep := want[id]; !keep {
			removed = append(removed, eng)
			delete(m.engines, id)
		}
	}
	for _, id := range ids {
		if _, ok := m.engines[id]; !ok {
			m.engines[id] = m.build(id)
			created = append(created, id)
		}
	}
	n := len(m.engines)
	started = m.started
	m.mu.Unlock()
	m.mInstances.Set(float64(n))
	// Shut down removed instances outside the lock: shutdown waits on the engine's
	// goroutines, which must not block concurrent show commands taking m.mu.
	for _, eng := range removed {
		eng.shutdown()
	}
	return created, started
}

// setConfig reconciles the engine set to cfg and pushes each engine its per-instance view.
func (m *instanceManager) setConfig(cfg ospfConfig) {
	m.ensureSet(cfg.instanceIDSet())
	for id, eng := range m.snapshot() {
		eng.setConfig(cfg.forInstance(id))
	}
}

// reconcile reconciles the engine set and applies cfg live: existing engines reconcile,
// newly created ones are configured and (if the manager has started) open their interfaces,
// and removed instances are torn down inside ensureSet.
func (m *instanceManager) reconcile(cfg ospfConfig) {
	created, started := m.ensureSet(cfg.instanceIDSet())
	createdSet := make(map[uint8]bool, len(created))
	for _, id := range created {
		createdSet[id] = true
	}
	for id, eng := range m.snapshot() {
		sub := cfg.forInstance(id)
		if !createdSet[id] {
			eng.reconcile(sub)
			continue
		}
		eng.setConfig(sub)
		if started {
			eng.subscribeIfaceEvents(getEventBus())
			// RFC 5443/6138 LDP-IGP sync: subscribe each v4 instance engine to LDP
			// SessionUp/SessionDown (unsubscribed on shutdown); no-op when LDP is absent.
			eng.subscribeLDPSyncEvents(getEventBus())
			if err := eng.openInterfaces(); err != nil {
				eng.log.Warn("ospf: instance open failed", "instance", id, "err", err)
			}
		}
	}
}

// start marks the manager started and, for a present config, configures, subscribes, and
// opens interfaces for every instance engine (the base and each non-zero instance).
func (m *instanceManager) start(cfg ospfConfig) error {
	m.mu.Lock()
	m.started = true
	m.mu.Unlock()
	m.ensureSet(cfg.instanceIDSet())
	eb := getEventBus()
	for _, id := range m.sortedIDs() {
		eng, ok := m.engineFor(id)
		if !ok {
			continue
		}
		eng.setConfig(cfg.forInstance(id))
		eng.subscribeIfaceEvents(eb)
		// RFC 5443/6138 LDP-IGP sync: subscribe each v4 instance engine to LDP
		// SessionUp/SessionDown (unsubscribed on shutdown); no-op when LDP is absent.
		eng.subscribeLDPSyncEvents(eb)
		if err := eng.openInterfaces(); err != nil {
			return fmt.Errorf("ospf: opening interfaces (instance %d): %w", id, err)
		}
	}
	return nil
}

// shutdownAll tears down every instance engine.
func (m *instanceManager) shutdownAll() {
	for _, eng := range m.snapshot() {
		eng.shutdown()
	}
}

// instanceSnapshot returns one instanceSummaryView per configured instance, sorted by
// Instance ID, for `show ospf instance`.
func (m *instanceManager) instanceSnapshot() []any {
	ids := m.sortedIDs()
	out := make([]any, 0, len(ids))
	for _, id := range ids {
		if eng, ok := m.engineFor(id); ok {
			out = append(out, eng.instanceSummary(id))
		}
	}
	return out
}

func (m *instanceManager) snapshot() map[uint8]*engine {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[uint8]*engine, len(m.engines))
	maps.Copy(out, m.engines)
	return out
}

func (m *instanceManager) sortedIDs() []uint8 {
	m.mu.Lock()
	ids := make([]uint8, 0, len(m.engines))
	for id := range m.engines {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	slices.Sort(ids)
	return ids
}
