// Design: plan/learned/958-ospf-4-component-config.md -- OSPFv2 engine skeleton + dispatcher
// Related: transport/transport.go -- raw IPv4 transport consumed here
// RFC: rfc/short/rfc2328.md -- OSPFv2; rfc/short/rfc3101.md -- NSSA translator stability
package ospf

import (
	"context"
	"log/slog"
	"maps"
	"net/netip"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	ospfiface "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/iface"
	ospflsdb "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/lsdb"
	ospfneighbor "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/neighbor"
	ospfspf "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/spf"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/transport"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

type engine struct {
	transport Transport
	dispatch  *dispatcher
	log       *slog.Logger

	mu                sync.Mutex
	cfg               ospfConfig
	areas             map[types.AreaID]*area
	running           map[string]interfaceConfig
	interfaces        map[string]*ospfiface.Interface
	neighbors         *ospfneighbor.Table
	lsdb              *ospflsdb.LSDB
	spf               *ospfspf.Computer
	ifaceMetric       ospfiface.Metrics
	neighborMetric    ospfneighbor.Metrics
	mASBR             metrics.Gauge
	mExternalLSAs     metrics.Gauge
	mNSSATranslations metrics.CounterVec
	mAuthFailures     metrics.CounterVec
	auth              *authStore
	// translations records the network -> source-NSSA of each Type 7 this router has
	// translated to a Type 5 (RFC 3101 §3.6), so a translation can be withdrawn when its
	// source Type 7 disappears or this router loses the translator role. Guarded by mu.
	translations map[[4]byte]types.AreaID
	// redistExternals records the networks this router originates as a Type 5 by its OWN
	// redistribution. RFC 3101 §3.6: the translator must NOT translate (nor purge) a
	// network it already advertises as a redistributed Type 5 -- both paths share the one
	// (Type5, network, self) LSA key. Guarded by mu.
	redistExternals map[[4]byte]bool
	// redistV6 maps each IPv6 prefix this router redistributes to the OSPFv3 AS-External-LSA
	// (0x4005) Link State ID assigned for it -- the OSPFv3 LSID is an arbitrary index (the
	// 128-bit prefix does not fit), so it is tracked here for re-origination and withdrawal.
	// redistV6Next is the monotonic LSID allocator. Guarded by mu (v6 engine only).
	redistV6     map[netip.Prefix]types.LinkStateID
	redistV6Next uint32
	// translatorState carries the per-NSSA translator-stability grace (RFC 3101 §3.5): a
	// router that loses the election keeps translating until the stability interval
	// elapses, so a transient flap of the elected translator does not open a Type 5 gap.
	// Guarded by mu.
	translatorState map[types.AreaID]translatorGrace
	// nssaMu serializes the NSSA reconciliation (applyNSSADefaults + translateNSSA),
	// which runs from both the reconcile (config-apply) goroutine and the retransmit
	// tick: without it two passes could interleave their read-compute-write of the
	// translations / translator-grace maps and double-originate or lose a withdraw.
	nssaMu         sync.Mutex
	sink           *eventSink
	receiveOnce    sync.Once
	retransmitOnce sync.Once
	// defaultInfoOriginated records whether this engine currently originates the
	// Type 5 default via `default-information originate`. redistDefaultInjected records
	// whether a `redistribute` rule currently injects 0.0.0.0/0. Both intents share the
	// one Type 5 default LSA key, so each path purges it only when the OTHER intent is
	// also gone (a withdraw never drops a default the other still wants). Guarded by mu.
	defaultInfoOriginated bool
	redistDefaultInjected bool
	// defaultInfoMu serializes applyDefaultInformation so the reconcile (config-apply)
	// caller and the Loc-RIB watcher worker cannot interleave: without it a stale worker
	// run could re-originate a default that a concurrent config-disable just withdrew.
	defaultInfoMu    sync.Mutex
	defaultWatchOnce sync.Once
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
}

func newEngine(t Transport) *engine { return newEngineWithCodec(t, v4Codec{}) }

// newEngineWithCodec builds an engine driven by a specific wire codec. newEngine is the
// OSPFv2 convenience wrapper (v4Codec); the IPv6 family supplies v6Codec together with an
// ospfv3 transport, so one engine drives either address family.
func newEngineWithCodec(t Transport, codec Codec) *engine {
	ctx, cancel := context.WithCancel(context.Background())
	neighborMetric := ospfneighbor.NopMetrics()
	db := ospflsdb.New(time.Now)
	e := &engine{
		transport:         t,
		dispatch:          newDispatcher(codec),
		log:               logger(),
		areas:             make(map[types.AreaID]*area),
		running:           make(map[string]interfaceConfig),
		interfaces:        make(map[string]*ospfiface.Interface),
		neighbors:         ospfneighbor.NewTable(neighborMetric),
		lsdb:              db,
		ifaceMetric:       ospfiface.NopMetrics(),
		neighborMetric:    neighborMetric,
		mASBR:             metrics.NopRegistry{}.Gauge("", ""),
		mExternalLSAs:     metrics.NopRegistry{}.Gauge("", ""),
		mNSSATranslations: metrics.NopRegistry{}.CounterVec("", "", nil),
		mAuthFailures:     metrics.NopRegistry{}.CounterVec("", "", nil),
		auth:              newAuthStore(),
		translatorState:   make(map[types.AreaID]translatorGrace),
		translations:      make(map[[4]byte]types.AreaID),
		redistExternals:   make(map[[4]byte]bool),
		redistV6:          make(map[netip.Prefix]types.LinkStateID),
		ctx:               ctx,
		cancel:            cancel,
	}
	db.SetTopology(e.lsdbTopology)
	// RFC 7474 §3: seed the authoritative high-order boot word from the ZeFS-persisted,
	// incremented boot count so the aggregate cryptographic sequence strictly increases
	// across a cold restart. loadOSPFBootCount tolerates an absent store and falls back
	// to a hashed high-resolution clock seed. Done once per engine, never per packet.
	e.auth.setBootCount(loadOSPFBootCount(openBootCountStore()))
	e.initSPF()
	e.dispatch.areaOK = e.acceptsAreaOnInterface
	e.installStubHandlers()
	e.installAuthHooks()
	e.neighbors.SetLSDB(db)
	e.neighbors.SetEventSink(neighborEventSink{onChange: e.originateSelfLSAs})
	if t != nil {
		db.SetTx(t.SendPacket)
		e.neighbors.SetSender(t)
		t.OnInterfaceDown(e.onInterfaceDown)
		t.OnInterfaceUp(e.onInterfaceUp)
	}
	return e
}

func (e *engine) installStubHandlers() {
	e.dispatch.register(PacketTypeHello, e.handleHello)
	e.dispatch.register(PacketTypeDBDesc, e.handleDBDesc)
	e.dispatch.register(PacketTypeLSReq, e.handleLSReq)
	e.dispatch.register(PacketTypeLSUpdate, e.handleLSUpdate)
	e.dispatch.register(PacketTypeLSAck, e.handleLSAck)
}

func (e *engine) handleHello(rp transport.RawPacket, h Header) {
	e.mu.Lock()
	ifc, name, ok := e.interfaceByIfIndexLocked(rp.IfIndex)
	e.mu.Unlock()
	if !ok {
		return
	}
	// The Hello body is decoded through the codec (version-specific) so a v6 instance decodes
	// via ospfv3/packet; the interface FSM consumes the AF-neutral types.Hello.
	hello, err := e.dispatch.codec.DecodeHello(rp.Payload)
	if err != nil {
		if e.transport != nil {
			e.transport.RecordDrop(name, dropReasonDecode)
		}
		return
	}
	if reason := ifc.ReceiveDecodedHello(h.RouterID, rp.Src, hello, time.Now()); reason != "" && e.transport != nil {
		e.transport.RecordDrop(name, reason)
	}
}

func (e *engine) handleDBDesc(rp transport.RawPacket, h Header) {
	e.handleNeighborPacket(rp, h, PacketTypeDBDesc)
}

func (e *engine) handleLSReq(rp transport.RawPacket, h Header) {
	e.handleNeighborPacket(rp, h, PacketTypeLSReq)
}

func (e *engine) handleLSUpdate(rp transport.RawPacket, h Header) {
	e.mu.Lock()
	ic, ok := e.runningByIfIndexLocked(rp.IfIndex)
	e.mu.Unlock()
	if !ok {
		return
	}
	up, err := e.dispatch.codec.DecodeLSUpdate(rp.Payload)
	if err != nil {
		if e.transport != nil {
			e.transport.RecordDrop(ic.Name, dropReasonDecode)
		}
		return
	}
	if reason := e.neighbors.AcceptsFlooding(ic.Name, h.RouterID); reason != "" {
		if e.transport != nil {
			e.transport.RecordDrop(ic.Name, reason)
		}
		return
	}
	if e.lsdb != nil {
		if reason := e.lsdb.ReceiveUpdate(ospflsdb.ReceiveInput{Interface: ic.Name, AreaID: h.AreaID, RouterID: h.RouterID, Src: rp.Src, Update: up}); reason != "" {
			if e.transport != nil {
				e.transport.RecordDrop(ic.Name, reason)
			}
			return
		}
	}
	if reason := e.neighbors.HandleLSUpdate(ic.Name, h.RouterID, up); reason != "" && e.transport != nil {
		e.transport.RecordDrop(ic.Name, reason)
	}
}

func (e *engine) handleLSAck(rp transport.RawPacket, h Header) {
	e.mu.Lock()
	ic, ok := e.runningByIfIndexLocked(rp.IfIndex)
	e.mu.Unlock()
	if !ok {
		return
	}
	ack, err := e.dispatch.codec.DecodeLSAck(rp.Payload)
	if err != nil {
		if e.transport != nil {
			e.transport.RecordDrop(ic.Name, dropReasonDecode)
		}
		return
	}
	if reason := e.neighbors.AcceptsFlooding(ic.Name, h.RouterID); reason != "" {
		if e.transport != nil {
			e.transport.RecordDrop(ic.Name, reason)
		}
		return
	}
	if e.lsdb != nil {
		if reason := e.lsdb.ReceiveAck(ospflsdb.AckInput{Interface: ic.Name, AreaID: h.AreaID, RouterID: h.RouterID, Ack: ack}); reason != "" && e.transport != nil {
			e.transport.RecordDrop(ic.Name, reason)
		}
	}
}

func (e *engine) handleNeighborPacket(rp transport.RawPacket, h Header, typ PacketType) {
	e.mu.Lock()
	ic, ok := e.runningByIfIndexLocked(rp.IfIndex)
	e.mu.Unlock()
	if !ok {
		return
	}
	// Decode the framing body through the codec (version-specific) so the v6 instance decodes
	// via ospfv3/packet. Only DBDesc/LSReq reach here (handleDBDesc/handleLSReq); LSUpdate has
	// its own handler (its typed LSA bodies stay version-specific until the AFPrefixStrategy).
	var reason string
	switch typ {
	case PacketTypeDBDesc:
		dd, err := e.dispatch.codec.DecodeDBDesc(rp.Payload)
		if err != nil {
			reason = dropReasonDecode
		} else {
			reason = e.neighbors.HandleDBDesc(ic.Name, h.RouterID, dd)
		}
	case PacketTypeLSReq:
		lsr, err := e.dispatch.codec.DecodeLSReq(rp.Payload)
		if err != nil {
			reason = dropReasonDecode
		} else {
			reason = e.neighbors.HandleLSReq(ic.Name, h.RouterID, lsr)
		}
	default:
		return
	}
	if reason != "" && e.transport != nil {
		e.transport.RecordDrop(ic.Name, reason)
	}
}

func (e *engine) interfaceByIfIndexLocked(ifindex int) (*ospfiface.Interface, string, bool) {
	ic, ok := e.runningByIfIndexLocked(ifindex)
	if !ok {
		return nil, "", false
	}
	ifc, ok := e.interfaces[ic.Name]
	return ifc, ic.Name, ok
}

func (e *engine) runningByIfIndexLocked(ifindex int) (interfaceConfig, bool) {
	if e.transport == nil {
		return interfaceConfig{}, false
	}
	name, ok := e.transport.InterfaceNameByIfIndex(ifindex)
	if !ok {
		return interfaceConfig{}, false
	}
	ic, ok := e.running[name]
	return ic, ok
}

func (e *engine) setConfig(cfg ospfConfig) {
	e.mu.Lock()
	e.cfg = cfg
	e.areas = newAreas(cfg)
	if e.lsdb != nil {
		e.lsdb.SetSelfRouterID(cfg.RouterID)
		e.lsdb.SetTimers(ospflsdb.TimerConfig{
			MinLSArrival:  time.Duration(cfg.Timers.MinLSArrivalMS) * time.Millisecond,
			MinLSInterval: time.Duration(cfg.Timers.MinLSIntervalMS) * time.Millisecond,
		})
		areaTypes := make(map[types.AreaID]string, len(cfg.Areas))
		ntAreas := make(map[types.AreaID]bool, len(cfg.Areas))
		for _, area := range cfg.Areas {
			areaTypes[area.AreaID] = string(area.AreaType)
			// RFC 3101 §3.5: advertise the Nt-bit for NSSAs whose translate role is not
			// `never`, so the highest-Router-ID candidate (not a higher-RID `never` ABR)
			// is elected the Type 7 -> Type 5 translator.
			if area.AreaType == areaTypeNSSA && area.NSSATranslateRole != translateRoleNever {
				ntAreas[area.AreaID] = true
			}
		}
		e.lsdb.SetAreaTypes(areaTypes)
		e.lsdb.SetNSSATranslatorAreas(ntAreas)
	}
	e.mu.Unlock()
	// OSPFv3 family: the neighbor table (DD/LSReq/LSUpdate) and the LSDB (flooded
	// LSUpdate/LSAck) must encode via the v6 encoder (ospfv3/packet). setMetrics also sets
	// the neighbor encoder, but it is not called for the v6 engine, so wire both here where
	// the config (and Instance ID) is known. OSPFv2 keeps the default encoders.
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		// RFC 5340 sec 4.2.2: drop received OSPFv3 packets whose Instance ID does not match
		// the configured one (the per-instance demux). OSPFv2 keeps the 0 default.
		e.dispatch.instanceID = cfg.InstanceID
		if e.neighbors != nil {
			e.neighbors.SetEncoder(v6Encoder{instanceID: cfg.InstanceID})
		}
		if e.lsdb != nil {
			e.lsdb.SetPacketEncoder(v6Encoder{instanceID: cfg.InstanceID})
		}
	}
	e.auth.configure(cfg)
	e.configureSPF(cfg)
}

func (e *engine) acceptsAreaOnInterface(ifindex int, id types.AreaID) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	ic, ok := e.runningByIfIndexLocked(ifindex)
	return ok && ic.AreaID == id
}

func (e *engine) setMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	e.mu.Lock()
	e.ifaceMetric = ospfiface.Metrics{
		InterfaceUp: reg.GaugeVec(
			"ze_ospf_interface_up",
			"Current OSPF interface operational state by area and interface.",
			[]string{"area", "interface"},
		),
		DRElections: reg.CounterVec(
			"ze_ospf_dr_elections_total",
			"Total OSPF DR/BDR elections that changed the elected role, by interface.",
			[]string{"interface"},
		),
	}
	e.neighborMetric = ospfneighbor.Metrics{
		Neighbors: reg.GaugeVec(
			"ze_ospf_neighbors",
			"Current OSPF neighbor count by area, interface, and state.",
			[]string{"area", "interface", "state"},
		),
		AdjacenciesFull: reg.GaugeVec(
			"ze_ospf_adjacencies_full",
			"Current number of full OSPF adjacencies by area.",
			[]string{"area"},
		),
		NSMEvents: reg.CounterVec(
			"ze_ospf_nsm_events_total",
			"Total OSPF neighbor state machine events by event.",
			[]string{"event"},
		),
	}
	e.mASBR = reg.Gauge(
		"ze_ospf_asbr",
		"Whether this OSPF router is currently an AS Boundary Router.",
	)
	e.mExternalLSAs = reg.Gauge(
		"ze_ospf_external_lsas",
		"Current number of self-originated OSPF Type 5 AS-External-LSAs.",
	)
	e.mNSSATranslations = reg.CounterVec(
		"ze_ospf_nssa_translations_total",
		"Total NSSA Type 7 to Type 5 LSA translations performed by the elected translator, by area.",
		[]string{"area"},
	)
	e.mAuthFailures = reg.CounterVec(
		"ze_ospf_auth_failures_total",
		"Total OSPF packets dropped for failing authentication, by interface and reason.",
		[]string{"interface", "reason"},
	)
	e.neighbors = ospfneighbor.NewTable(e.neighborMetric)
	if e.lsdb != nil {
		e.lsdb.SetMetrics(reg)
		e.neighbors.SetLSDB(e.lsdb)
	} else {
		e.neighbors.SetLSDB(nil)
	}
	if e.transport != nil {
		e.neighbors.SetSender(e.transport)
	}
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		// OSPFv3 family: encode DD/LSReq/LSUpdate via the v6 encoder (ospfv3/packet).
		e.neighbors.SetEncoder(v6Encoder{instanceID: e.cfg.InstanceID})
	}
	e.neighbors.SetEventSink(neighborEventSink{sink: e.sink, onChange: e.originateSelfLSAs})
	e.mu.Unlock()
	if e.spf != nil {
		e.spf.SetMetrics(reg)
	}
}

func (e *engine) setEventSink(s *eventSink) {
	e.mu.Lock()
	e.sink = s
	if e.neighbors != nil {
		e.neighbors.SetEventSink(neighborEventSink{sink: e.sink, onChange: e.originateSelfLSAs})
	}
	e.mu.Unlock()
}

func (e *engine) openInterfaces() error {
	e.mu.Lock()
	enrolled := e.cfg.enrolledInterfaces()
	activeCount := len(e.cfg.activeInterfaces())
	e.mu.Unlock()
	if activeCount > 0 && e.transport != nil {
		e.startReceiveLoop()
	}
	if activeCount > 0 {
		e.startNeighborRetransmitLoop()
	}
	for _, ic := range enrolled {
		if err := e.openConfiguredInterface(ic); err != nil {
			return err
		}
	}
	return nil
}

func (e *engine) openConfiguredInterface(ic interfaceConfig) error {
	if ic.Passive || ic.NetworkType == networkLoopback {
		e.mu.Lock()
		e.running[ic.Name] = ic
		e.startInterfaceLocked(ic)
		e.mu.Unlock()
		return nil
	}
	return e.openInterface(ic)
}

func (e *engine) openInterface(ic interfaceConfig) error {
	e.startNeighborRetransmitLoop()
	if e.transport == nil {
		e.mu.Lock()
		e.running[ic.Name] = ic
		e.startInterfaceLocked(ic)
		e.mu.Unlock()
		return nil
	}
	e.startReceiveLoop()
	e.transport.EnableInterface(ic.Name)
	return e.transport.HandleLinkUp(ic.Name)
}

func (e *engine) startReceiveLoop() {
	e.receiveOnce.Do(func() {
		recv := e.transport.Receive()
		e.wg.Go(func() {
			for {
				select {
				case <-e.ctx.Done():
					return
				case rp, ok := <-recv:
					if !ok {
						return
					}
					e.dispatch.dispatch(rp)
				}
			}
		})
	})
}

func (e *engine) startNeighborRetransmitLoop() {
	if e.neighbors == nil {
		return
	}
	e.retransmitOnce.Do(func() {
		ticker := time.NewTicker(time.Second)
		e.wg.Go(func() {
			defer ticker.Stop()
			for {
				select {
				case <-e.ctx.Done():
					return
				case now := <-ticker.C:
					e.neighbors.Retransmit(now)
					if e.lsdb != nil {
						e.lsdb.RetransmitTick(now)
						e.lsdb.Tick(now)
						e.lsdb.RefreshSelf(now)
						e.originateSelfLSAs()
						// Re-evaluate the per-NSSA Type 7 default: reconcile runs this once, but a
						// transport interface joins the NSSA asynchronously (link-up after reconcile),
						// so an ABR that became attached later would otherwise never originate the
						// default. Idempotent (OriginateNSSA short-circuits an unchanged body).
						e.applyNSSADefaults()
						e.translateNSSA(now)
						topology := e.lsdbTopology()
						for idx := range topology {
							e.lsdb.FlushDelayedAcks(topology[idx].Name)
						}
					}
				}
			}
		})
	})
}

func (e *engine) subscribeIfaceEvents(eb ze.EventBus) {
	if eb == nil || e.transport == nil {
		return
	}
	_ = e.transport.SubscribeIfaceEvents(eb)
}

type reconcileResult struct {
	opened  []string
	closed  []string
	changed map[string]bool
}

func (e *engine) reconcile(newCfg ospfConfig) reconcileResult {
	res := reconcileResult{changed: make(map[string]bool)}
	e.mu.Lock()
	oldCfg := e.cfg
	e.cfg = newCfg
	e.areas = newAreas(newCfg)
	desiredList := newCfg.enrolledInterfaces()
	current := make(map[string]interfaceConfig, len(e.running))
	maps.Copy(current, e.running)
	e.mu.Unlock()
	e.setConfig(newCfg)

	desired := make(map[string]interfaceConfig, len(desiredList))
	for _, ic := range desiredList {
		desired[ic.Name] = ic
	}
	for name := range current {
		if _, keep := desired[name]; !keep {
			e.closeInterface(name)
			res.closed = append(res.closed, name)
		}
	}
	for name, want := range desired {
		have, exists := current[name]
		switch {
		case !exists:
			if err := e.openConfiguredInterface(want); err != nil {
				e.log.Warn("ospf: reconcile open failed", "interface", name, "err", err)
				continue
			}
			if !want.Passive && want.NetworkType != networkLoopback {
				res.opened = append(res.opened, name)
			}
		case !interfaceParamsEqual(have, want) || interfaceGlobalParamsChanged(oldCfg, newCfg, want.AreaID):
			if !have.Passive && have.NetworkType != networkLoopback && (want.Passive || want.NetworkType == networkLoopback) && e.transport != nil {
				e.transport.DisableInterface(name)
			}
			if (have.Passive || have.NetworkType == networkLoopback) && !want.Passive && want.NetworkType != networkLoopback {
				e.mu.Lock()
				e.stopInterfaceLocked(name)
				e.mu.Unlock()
				if err := e.openInterface(want); err != nil {
					e.log.Warn("ospf: reconcile open failed", "interface", name, "err", err)
					continue
				}
				res.opened = append(res.opened, name)
			} else {
				e.mu.Lock()
				e.running[name] = want
				e.startInterfaceLocked(want)
				e.mu.Unlock()
			}
			res.changed[name] = true
		}
	}
	// Re-evaluate `default-information originate` against the new config: `always`
	// originates immediately, the conditional form against the current Loc-RIB, and a
	// removed/disabled rule withdraws. Live RIB changes are handled by watchDefaultRoute.
	e.applyDefaultInformation()
	// Re-evaluate per-NSSA `default-originate` (Type 7 default) against the new config.
	e.applyNSSADefaults()
	// Re-run translator election + Type 7 -> Type 5 translation (role/attachment change).
	e.translateNSSA(time.Now())
	return res
}

func (e *engine) closeInterface(name string) {
	if e.transport != nil {
		e.transport.DisableInterface(name)
	}
	e.mu.Lock()
	e.stopInterfaceLocked(name)
	delete(e.running, name)
	e.mu.Unlock()
}

func (e *engine) startInterfaceLocked(ic interfaceConfig) {
	if old := e.interfaces[ic.Name]; old != nil {
		old.Stop()
	}
	var sender ospfiface.Sender
	if !ic.Passive && ic.NetworkType != networkLoopback {
		sender = e.transport
	}
	cfg := e.interfaceRuntimeConfigLocked(ic)
	if e.neighbors != nil {
		e.neighbors.ConfigureInterface(neighborInterfaceConfig(cfg))
	}
	rt := ospfiface.New(cfg, sender, e.ifaceMetric)
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		// OSPFv3 interface: send Hellos through the v6 encoder (ospfv3/packet).
		rt.SetEncoder(v6Encoder{instanceID: e.cfg.InstanceID})
	}
	if e.sink != nil {
		rt.SetEventSink(e.sink)
	}
	if e.neighbors != nil {
		rt.SetNeighborSink(nsmAdapter{table: e.neighbors, onChange: e.originateSelfLSAs, auth: e.auth})
	}
	e.interfaces[ic.Name] = rt
	if ic.Passive || ic.NetworkType == networkLoopback || e.transport == nil || e.transport.InterfaceOpen(ic.Name) {
		rt.Start()
	}
}

func (e *engine) stopInterfaceLocked(name string) {
	if rt := e.interfaces[name]; rt != nil {
		rt.Stop()
		delete(e.interfaces, name)
	}
	if e.neighbors != nil {
		e.neighbors.DeleteInterface(name)
	}
	if e.lsdb != nil {
		e.lsdb.ReleaseLink(name)
	}
}

func (e *engine) interfaceRuntimeConfigLocked(ic interfaceConfig) ospfiface.Config {
	areaKind := areaTypeNormal
	for _, a := range e.cfg.Areas {
		if a.AreaID == ic.AreaID {
			areaKind = string(a.AreaType)
			break
		}
	}
	cost := ic.Cost
	if !ic.HasCost {
		cost = 1
	}
	return ospfiface.Config{
		Name:               ic.Name,
		RouterID:           e.cfg.RouterID,
		AreaID:             ic.AreaID,
		AreaType:           areaKind,
		NetworkType:        string(ic.NetworkType),
		NetworkMask:        interfaceNetworkMask(ic.Name),
		InterfaceAddress:   interfaceIPv4Address(ic.Name),
		Cost:               cost,
		HelloInterval:      ic.HelloInterval,
		DeadInterval:       ic.DeadInterval,
		Priority:           ic.Priority,
		Passive:            ic.Passive,
		InterfaceMTU:       interfaceMTU(ic.Name),
		MTUIgnore:          ic.MTUIgnore,
		RetransmitInterval: ic.RetransmitInterval,
		IsV6:               e.dispatch.codec.IsV6(),
		InterfaceID:        interfaceIndex(ic.Name),
	}
}

func neighborInterfaceConfig(cfg ospfiface.Config) ospfneighbor.InterfaceConfig {
	return ospfneighbor.InterfaceConfig{
		Name:               cfg.Name,
		AreaID:             cfg.AreaID,
		RouterID:           cfg.RouterID,
		NetworkType:        cfg.NetworkType,
		InterfaceAddress:   cfg.InterfaceAddress,
		Options:            ospfOptionsForAreaType(cfg.AreaType),
		InterfaceMTU:       cfg.InterfaceMTU,
		MTUIgnore:          cfg.MTUIgnore,
		DeadInterval:       cfg.DeadInterval,
		RetransmitInterval: cfg.RetransmitInterval,
	}
}

func ospfOptionsForAreaType(areaKind string) types.Options {
	var o types.Options
	switch areaKind {
	case ospfiface.AreaStub:
		return o.Clear(types.OptionE)
	case ospfiface.AreaNSSA:
		return o.Clear(types.OptionE).Set(types.OptionNP)
	default:
		return o.Set(types.OptionE)
	}
}

type neighborEventSink struct {
	sink     ospfneighbor.EventSink
	onChange func()
}

func (s neighborEventSink) NeighborUp(snap ospfneighbor.Snapshot) {
	if s.sink != nil {
		s.sink.NeighborUp(snap)
	}
	if s.onChange != nil {
		s.onChange()
	}
}

func (s neighborEventSink) NeighborDown(snap ospfneighbor.Snapshot) {
	if s.sink != nil {
		s.sink.NeighborDown(snap)
	}
	if s.onChange != nil {
		go s.onChange()
	}
}

type nsmAdapter struct {
	table    *ospfneighbor.Table
	onChange func()
	auth     *authStore
}

var _ ospfiface.NeighborSink = nsmAdapter{}

func (a nsmAdapter) NeighborHello(ev ospfiface.NeighborEvent) {
	if a.table == nil {
		return
	}
	_ = a.table.Hello(ospfneighbor.HelloInput{
		InterfaceName: ev.InterfaceName,
		AreaID:        ev.AreaID,
		LocalRouterID: ev.LocalRouterID,
		LocalDR:       ev.LocalDR,
		LocalBDR:      ev.LocalBDR,
		NeighborID:    ev.NeighborID,
		Address:       ev.Address,
		Priority:      ev.Priority,
		TwoWay:        ev.TwoWay,
		DeclaredDR:    ev.DeclaredDR,
		DeclaredBDR:   ev.DeclaredBDR,
		NetworkType:   ev.NetworkType,
		DeadInterval:  ev.DeadInterval,
		InterfaceMTU:  ev.InterfaceMTU,
		MTUIgnore:     ev.MTUIgnore,
		InterfaceID:   ev.InterfaceID,
		Now:           time.Now(),
	})
	if a.onChange != nil {
		a.onChange()
	}
}

func (a nsmAdapter) NeighborDown(interfaceName string, id types.RouterID) {
	if a.table != nil {
		a.table.NeighborDown(interfaceName, id)
	}
	if a.auth != nil {
		// RFC 2328 App D: forget this neighbor's cryptographic receive sequence so it can
		// re-establish with any sequence after its own restart without a false replay drop.
		a.auth.resetNeighbor(interfaceName, id)
	}
	if a.onChange != nil {
		a.onChange()
	}
}

func (a nsmAdapter) AdjOK(interfaceName string, dr, bdr types.RouterID) {
	if a.table != nil {
		a.table.AdjOK(interfaceName, dr, bdr)
	}
	if a.onChange != nil {
		a.onChange()
	}
}

func (a nsmAdapter) InterfaceDown(interfaceName string) {
	if a.table != nil {
		a.table.InterfaceDown(interfaceName)
	}
	if a.auth != nil {
		a.auth.resetInterface(interfaceName)
	}
	if a.onChange != nil {
		go a.onChange()
	}
}

func (e *engine) markInterfaceDownLocked(name string) {
	if rt := e.interfaces[name]; rt != nil {
		rt.Stop()
	}
}

func (e *engine) onInterfaceDown(_ int, name string) {
	e.mu.Lock()
	e.markInterfaceDownLocked(name)
	e.mu.Unlock()
}

func (e *engine) onInterfaceUp(_ int, name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, ic := range e.cfg.Interfaces {
		if ic.Name != name || !ic.Enabled || ic.Passive || ic.NetworkType == networkLoopback {
			continue
		}
		for _, a := range e.cfg.Areas {
			if a.AreaID == ic.AreaID {
				e.running[name] = ic
				e.startInterfaceLocked(ic)
				return
			}
		}
		return
	}
}

func interfaceParamsEqual(a, b interfaceConfig) bool {
	return a.Enabled == b.Enabled &&
		a.Passive == b.Passive &&
		a.AreaID == b.AreaID &&
		a.NetworkType == b.NetworkType &&
		a.Cost == b.Cost &&
		a.HasCost == b.HasCost &&
		a.HelloInterval == b.HelloInterval &&
		a.DeadInterval == b.DeadInterval &&
		a.Priority == b.Priority &&
		a.MTUIgnore == b.MTUIgnore &&
		a.RetransmitInterval == b.RetransmitInterval &&
		a.TransmitDelay == b.TransmitDelay &&
		a.Authentication == b.Authentication
}

func interfaceGlobalParamsChanged(oldCfg, newCfg ospfConfig, areaID types.AreaID) bool {
	return oldCfg.RouterID != newCfg.RouterID || areaTypeFor(oldCfg, areaID) != areaTypeFor(newCfg, areaID)
}

func areaTypeFor(cfg ospfConfig, areaID types.AreaID) areaType {
	for _, a := range cfg.Areas {
		if a.AreaID == areaID {
			return a.AreaType
		}
	}
	return areaTypeNormal
}

func (e *engine) shutdown() {
	if e.spf != nil {
		e.spf.Stop()
	}
	e.cancel()
	if e.transport != nil {
		e.transport.Close()
	}
	e.wg.Wait()
}

func (e *engine) originateSelfLSAs() {
	e.mu.Lock()
	cfg := e.cfg
	e.mu.Unlock()
	if e.lsdb == nil || cfg.RouterID == (types.RouterID{}) {
		return
	}
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		// OSPFv3 self-LSAs are address-free and split prefixes into a separate
		// Intra-Area-Prefix-LSA, so the IPv6 family originates through the OSPFv3
		// builder instead of the OSPFv2 lsdb.OriginateFromTopology path.
		e.v6OriginateSelf(cfg.RouterID, cfg.MaxMetric.RouterLSAAlways)
		return
	}
	e.lsdb.OriginateFromTopology(cfg.RouterID, cfg.MaxMetric.RouterLSAAlways)
}

func (e *engine) lsdbTopology() []ospflsdb.InterfaceInfo {
	e.mu.Lock()
	cfg := e.cfg
	running := make([]interfaceConfig, 0, len(e.running))
	for _, ic := range e.running {
		running = append(running, ic)
	}
	ifaces := make(map[string]*ospfiface.Interface, len(e.interfaces))
	maps.Copy(ifaces, e.interfaces)
	e.mu.Unlock()

	out := make([]ospflsdb.InterfaceInfo, 0, len(running))
	for _, ic := range running {
		areaKind := string(areaTypeFor(cfg, ic.AreaID))
		state := ""
		var dr, bdr types.RouterID
		if ifc := ifaces[ic.Name]; ifc != nil {
			state = ifc.State().String()
			dr = ifc.DR()
			bdr = ifc.BDR()
		}
		cost := ic.Cost
		if !ic.HasCost {
			cost = 1
		}
		floodNeighbors := e.neighbors.FloodNeighbors(ic.Name)
		neighbors := make([]ospflsdb.NeighborInfo, 0, len(floodNeighbors))
		for _, n := range floodNeighbors {
			neighbors = append(neighbors, ospflsdb.NeighborInfo{RouterID: n.RouterID, Address: n.Address, State: n.State, InterfaceID: n.InterfaceID})
		}
		out = append(out, ospflsdb.InterfaceInfo{
			Name:               ic.Name,
			AreaID:             ic.AreaID,
			AreaType:           areaKind,
			NetworkType:        string(ic.NetworkType),
			State:              state,
			Priority:           ic.Priority,
			Passive:            ic.Passive,
			Address:            interfaceIPv4Address(ic.Name),
			NetworkMask:        interfaceNetworkMask(ic.Name),
			InterfaceID:        interfaceIndex(ic.Name),
			Cost:               cost,
			RouterID:           cfg.RouterID,
			Options:            ospfOptionsForAreaType(areaKind),
			DR:                 dr,
			BDR:                bdr,
			RetransmitInterval: ic.RetransmitInterval,
			TransmitDelay:      ic.TransmitDelay,
			Neighbors:          neighbors,
			IsV6:               e.dispatch.codec.IsV6(),
			IPv6LinkLocal:      interfaceIPv6LinkLocal(ic.Name),
			IPv6Prefixes:       interfaceIPv6Prefixes(ic.Name),
		})
	}
	return out
}

func (e *engine) neighborSnapshot() []any {
	if e.neighbors == nil {
		return nil
	}
	snap := e.neighbors.Snapshot()
	out := make([]any, 0, len(snap))
	for _, row := range snap {
		out = append(out, row)
	}
	return out
}
func (e *engine) interfaceSnapshot() []any {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]any, 0, len(e.interfaces))
	for _, ifc := range e.interfaces {
		out = append(out, ifc.Snapshot())
	}
	return out
}

func (e *engine) databaseSnapshot() []any {
	if e.lsdb == nil {
		return nil
	}
	return []any{e.lsdb.Snapshot()}
}
func (e *engine) routeSnapshot() []any {
	if e.spf == nil {
		return nil
	}
	rows := e.spf.Snapshot()
	out := make([]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, row)
	}
	return out
}
func (e *engine) borderRouterSnapshot() []any {
	if e.spf == nil {
		return nil
	}
	rows := e.spf.BorderRouterSnapshot()
	out := make([]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, row)
	}
	return out
}
func (e *engine) spfSnapshot() []any {
	if e.spf == nil {
		return nil
	}
	rows := e.spf.SPFSnapshot()
	out := make([]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, row)
	}
	return out
}
