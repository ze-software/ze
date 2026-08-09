// Design: docs/architecture/ospf/ospf-4-component-config.md -- OSPFv2 engine skeleton + dispatcher
// Related: transport/transport.go -- raw IPv4 transport consumed here
// RFC: rfc/short/rfc2328.md -- OSPFv2; rfc/short/rfc3101.md -- NSSA translator stability
package ospf

import (
	"context"
	"log/slog"
	"maps"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
	ospfiface "github.com/ze-software/ze/internal/plugins/ospf/iface"
	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	ospfneighbor "github.com/ze-software/ze/internal/plugins/ospf/neighbor"
	ospfspf "github.com/ze-software/ze/internal/plugins/ospf/spf"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	"github.com/ze-software/ze/pkg/ze"
)

type engine struct {
	transport Transport
	dispatch  *dispatcher
	log       *slog.Logger
	// ipsec installs RFC 4552 kernel IPsec for the IPv6 (OSPFv3) family; nil for the
	// IPv4 family and when no interface configures IPsec. Set via installIPsecHooks.
	ipsec *ipsecInstaller

	// af is this engine's OSPFv3 address family (RFC 5838), derived once from its
	// Instance ID at spawn. It selects the Loc-RIB install family, the prefix address
	// width, and the AF-bit rule. For the OSPFv2 (v4Codec) engine the field is unused;
	// installFamily returns family.IPv4Unicast unconditionally there.
	af addressFamily
	// multiAF is true when the router runs more than one OSPFv3 address family, so the
	// default IPv6-unicast instance emits the AF-bit too (RFC 5838 §2.5); a lone
	// IPv6-unicast instance keeps the IPv6-base wire bytes (no AF-bit). Atomic because the
	// config-apply path writes it while the interface send goroutine reads it.
	multiAF atomic.Bool
	// mAFBitMismatch counts neighbors not brought to Full for a missing AF-bit on a
	// non-default AF (RFC 5838 §2.5), by AF. No-op until setMetrics wires a registry.
	mAFBitMismatch metrics.CounterVec

	mu         sync.Mutex
	cfg        ospfConfig
	areas      map[types.AreaID]*area
	running    map[string]interfaceConfig
	interfaces map[string]*ospfiface.Interface
	neighbors  *ospfneighbor.Table
	lsdb       *ospflsdb.LSDB
	spf        *ospfspf.Computer
	// srInstaller programs Segment Routing MPLS forwarding (spec-ospf-ext-5) from the
	// post-SPF hook; nil until initSPF wires it. It reads remote SR state from the LSDB
	// and emits push/swap/pop entries on the shared mpls-fib bus.
	srInstaller *srInstaller
	// srAdj drives the Adj-SID lifecycle off the neighbor Full<->non-Full transition
	// (SRLB allocation + pop/forward install + withdraw); nil until initSPF wires it.
	srAdj *srAdjManager
	// virtualLinks is the engine-side runtime of each configured OSPF virtual link
	// (spec-ospf-ext-7), keyed by (transit area, neighbor). configureVirtualLinks builds it
	// from config; onVirtualLinksResolved (the SPF callback) drives each link up/down from
	// its transit area's SPF result; virtualLinkTopology surfaces a reachable link as a
	// synthetic backbone interface for origination. Guarded by mu.
	virtualLinks      map[virtualLinkKey]*virtualLinkRuntime
	mVirtualLinks     metrics.GaugeVec
	mVirtualCost      metrics.GaugeVec
	mVirtualAdjChgs   metrics.CounterVec
	ifaceMetric       ospfiface.Metrics
	neighborMetric    ospfneighbor.Metrics
	mASBR             metrics.Gauge
	mExternalLSAs     metrics.Gauge
	mNSSATranslations metrics.CounterVec
	mAuthFailures     metrics.CounterVec
	mInstanceMismatch metrics.CounterVec
	// opaque holds the RFC 5250 opaque-carrier metric series (spec-ospf-ext-1).
	opaque opaqueMetrics
	// ted is the RFC 3630 / RFC 5392 Traffic Engineering Database (spec-ospf-ext-2): a
	// passive, link-keyed store fed by the TE opaque consumer's OnReceive. te holds its
	// metric series. Both are owned by the TE consumer (te.go); nil-safe when TE is inert.
	ted    *ted
	te     teMetrics
	teOrig *teOriginator
	// ri is the RFC 7770 Router Information LSA metric series and riOrig the OSPFv2 opaque
	// origination state (spec-ospf-ext-3); both are AF-neutral and owned by the RI consumer
	// (ri.go). riGRState is the graceful-restart capability seam (ext-9 sets it; nil = not
	// GR-capable) feeding the RFC 7770 sec 2.5 informational bits 0/1. riSeen tracks OSPFv3
	// peer RI LSA identities already counted into ze_ospf_ri_received_total (guarded by riMu).
	ri             riMetrics
	riOrig         *riOriginator
	riGRState      func() (capable, helper bool)
	riLSAsGauge    *gaugeVecTracker
	riCapBitsGauge *gaugeVecTracker
	riMu           sync.Mutex
	riSeen         map[riOrigKey]struct{}
	// ext is the RFC 7684 Extended Prefix/Link metric series (spec-ospf-ext-4); extOrig the
	// Opaque Type 7/8 origination state (stable Opaque IDs + withdraw diffing); extRecv the
	// received-attribute resolver (lowest-Opaque-ID dedup + Type-11 reachability). All are
	// owned by the Extended Prefix/Link consumers (ext_prefix.go / ext_link.go).
	ext                extMetrics
	extOrig            *extOriginator
	extRecv            *extReceiver
	extPrefixLSAsGauge *gaugeVecTracker
	// gauge trackers zero a labeled gauge series whose label set drains between refreshes
	// (metrics.GaugeVec has no Reset): the opaque population + opaque-capable-neighbor gauges
	// (opaque.go) and the TE LSA + TE database-link gauges (te.go).
	opaqueLSAsGauge  *gaugeVecTracker
	capableNbrsGauge *gaugeVecTracker
	teLSAsGauge      *gaugeVecTracker
	teDBLinksGauge   *gaugeVecTracker
	// opaqueReachableFn is the RFC 5250 §5 originator-reachability seam for Type-11
	// opaque LSAs; set to spfRouterReachable in newEngine, overridable in tests.
	opaqueReachableFn func(types.RouterID) bool
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
	nssaMu sync.Mutex
	// nssaDefaultAreas tracks the areas where applyNSSADefaults owns a
	// self-originated default. It lets reconciliation withdraw defaults from
	// areas removed from config. Guarded by nssaMu.
	nssaDefaultAreas map[types.AreaID]struct{}
	// ipv4Address is the live forwarding-address lookup. Tests replace it with
	// a deterministic lookup. A nil value uses interfaceIPv4Address.
	ipv4Address    func(string) [4]byte
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
	// ldpSync holds the per-interface RFC 5443 / RFC 6138 LDP-IGP synchronization
	// machines; nil-safe methods make it inert when no interface enables ldp-sync.
	// ldpSyncUnsub removes the LDP event subscription on shutdown (R-7).
	ldpSync      *ldpSyncManager
	ldpSyncUnsub func()
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	// BFD (RFC 5880 / RFC 5881) client state. bfdClients maps a Full neighbor to its
	// single-hop BFD session; guarded by bfdMu (a dedicated lock, never nested inside mu, so
	// the subscriber can drive NeighborDown without lock-order inversion). bfdMetrics is the
	// shared ze_ospf_bfd_* series; bfdWarnOnce fires the plugin-absent warning once. Each
	// engine instance owns its own map, so a v6 down never touches a v4 neighbor (R-4).
	bfdMu       sync.Mutex
	bfdClients  map[bfdClientKey]*bfdClient
	bfdMetrics  ospfBFDMetrics
	bfdWarnOnce sync.Once
	// gr is the RFC 3623 / RFC 5187 Graceful Restart control plane (spec-ospf-ext-9): the
	// shared restarter + helper state machines. Nil-safe/inert until the operator enables
	// graceful-restart; both address families drive the same manager.
	gr *grManager
}

func newEngine(t Transport) *engine { return newEngineWithCodecAF(t, v4Codec{}, afIPv4Unicast) }

// newEngineWithCodecAF builds an engine driven by a specific wire codec and OSPFv3 address
// family (RFC 5838). newEngine is the OSPFv2 convenience wrapper (v4Codec, family fixed to
// IPv4-unicast); the IPv6 family supplies v6Codec together with an ospfv3 transport and the
// AF derived from its Instance ID, so one engine implementation drives every address family.
func newEngineWithCodecAF(t Transport, codec Codec, af addressFamily) *engine {
	ctx, cancel := context.WithCancel(context.Background())
	neighborMetric := ospfneighbor.NopMetrics()
	db := ospflsdb.New(time.Now)
	e := &engine{
		transport:         t,
		dispatch:          newDispatcher(codec),
		log:               logger(),
		af:                af,
		mAFBitMismatch:    metrics.NopRegistry{}.CounterVec("", "", nil),
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
		mInstanceMismatch: metrics.NopRegistry{}.CounterVec("", "", nil),
		opaque:            nopOpaqueMetrics(),
		auth:              newAuthStore(),
		translatorState:   make(map[types.AreaID]translatorGrace),
		translations:      make(map[[4]byte]types.AreaID),
		redistExternals:   make(map[[4]byte]bool),
		redistV6:          make(map[netip.Prefix]types.LinkStateID),
		bfdClients:        make(map[bfdClientKey]*bfdClient),
		bfdMetrics:        nopBFDMetrics(),
		ctx:               ctx,
		cancel:            cancel,
	}
	// RFC 5443 / RFC 6138 LDP-IGP sync: per-interface machines that re-originate this
	// engine's self-LSAs on every sync-state change. Nil-safe, so an engine with no
	// ldp-sync interface pays nothing. Shared by the OSPFv2 and each OSPFv3 AF engine.
	e.ldpSync = newLDPSyncManager(e.originateSelfLSAs, e.log)
	// RFC 3623 / RFC 5187 Graceful Restart control plane (spec-ospf-ext-9), shared by both
	// address families. Inert until the operator enables graceful-restart.
	e.gr = newGRManager(e)
	db.SetTopology(e.lsdbTopology)
	// Graceful Restart LSDB seams (spec-ospf-ext-9): suppress the fight-back flush of the
	// restarter's own self-LSAs while in-restart (RFC 3623 sec 2), and observe content
	// changes for the helper's sec 3.2 strict-LSA-checking exit.
	db.SetSelfFlushSuppress(e.gr.suppressSelfFlush)
	db.SetContentChangeObserver(e.gr.onContentChange)
	// RFC 7474 §3: seed the authoritative high-order boot word from the ZeFS-persisted,
	// incremented boot count so the aggregate cryptographic sequence strictly increases
	// across a cold restart. loadOSPFBootCount tolerates an absent store and falls back
	// to a hashed high-resolution clock seed. Done once per engine, never per packet.
	e.auth.setBootCount(loadOSPFBootCount(openBootCountStore()))
	e.initSPF()
	e.dispatch.areaOK = e.acceptsArea
	e.dispatch.onInstanceMismatch = e.recordInstanceMismatch
	e.installStubHandlers()
	e.installAuthHooks()
	e.opaqueReachableFn = e.spfRouterReachable
	e.ted = newTED()
	// RFC 5250 sec 5: the TED consults live SPF reachability so a Type-11 inter-AS entry
	// flips usable/unusable as its originator becomes reachable or not.
	e.ted.setReachable(e.routerReachable)
	e.te = nopTEMetrics()
	// Gauge trackers (zero a drained label set; see gaugeVecTracker). Initialized here for the
	// nop-metrics path; setOpaqueMetrics/setTEMetrics rebind fresh trackers with the real gauges.
	e.opaqueLSAsGauge = newGaugeVecTracker()
	e.capableNbrsGauge = newGaugeVecTracker()
	e.teLSAsGauge = newGaugeVecTracker()
	e.teDBLinksGauge = newGaugeVecTracker()
	e.teOrig = newTEOriginator(e.lsdbTopology)
	// RFC 7770 Router Information (spec-ospf-ext-3): nop metrics + trackers until setRIMetrics
	// binds the real ze_ospf_ri_* series (called via setMetrics for both the IPv4 engine and
	// each OSPFv3 AF engine, spec-ospf-ext-15); the series are deduped by name and labeled by
	// address family. The OSPFv2 opaque origination state and the OSPFv3 received-diff set are
	// per-engine.
	e.ri = nopRIMetrics()
	e.riOrig = newRIOriginator()
	e.riSeen = make(map[riOrigKey]struct{})
	e.riLSAsGauge = newGaugeVecTracker()
	e.riCapBitsGauge = newGaugeVecTracker()
	// RFC 7684 Extended Prefix/Link (spec-ospf-ext-4): nop metrics + tracker until
	// setExtMetrics binds the real ze_ospf_ext_* series; the origination and receive state
	// is per-engine.
	e.ext = nopExtMetrics()
	e.extOrig = newExtOriginator()
	e.extRecv = newExtReceiver()
	e.extPrefixLSAsGauge = newGaugeVecTracker()
	e.wireOpaqueDelivery()
	e.neighbors.SetLSDB(db)
	e.neighbors.SetEventSink(e.neighborEventSinkValue())
	if t != nil {
		db.SetTx(t.SendPacket)
		// The neighbor table sends DD/LSReq/LSUpdate for ALL interfaces through one sender;
		// the virtual-aware sender routes virtual-link names and passes real names through.
		e.neighbors.SetSender(e.virtualSender())
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
	name, ifc, ok := e.receiveTargetLocked(rp.IfIndex, h)
	e.mu.Unlock()
	if !ok || ifc == nil {
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
	// RFC 5838 §2.5/§2.6: a non-default AF requires the AF-bit to form an adjacency; a
	// Hello without it is dropped here so the neighbor is never brought to Full. The
	// default IPv6-unicast AF accepts a missing AF-bit (§2.6).
	if !e.afHelloAccepted(hello) {
		if e.transport != nil {
			e.transport.RecordDrop(name, dropReasonAFBit)
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
	name, _, ok := e.receiveTargetLocked(rp.IfIndex, h)
	e.mu.Unlock()
	if !ok {
		return
	}
	up, err := e.dispatch.codec.DecodeLSUpdate(rp.Payload)
	if err != nil {
		if e.transport != nil {
			e.transport.RecordDrop(name, dropReasonDecode)
		}
		return
	}
	if reason := e.neighbors.AcceptsFlooding(name, h.RouterID); reason != "" {
		if e.transport != nil {
			e.transport.RecordDrop(name, reason)
		}
		return
	}
	if e.lsdb != nil {
		if reason := e.lsdb.ReceiveUpdate(ospflsdb.ReceiveInput{Interface: name, AreaID: h.AreaID, RouterID: h.RouterID, Src: rp.Src, Update: up}); reason != "" {
			if e.transport != nil {
				e.transport.RecordDrop(name, reason)
			}
			return
		}
	}
	if reason := e.neighbors.HandleLSUpdate(name, h.RouterID, up); reason != "" && e.transport != nil {
		e.transport.RecordDrop(name, reason)
	}
	// OSPFv3 Graceful Restart (RFC 5187): the native link-scope Grace-LSA (LS Type 0x000B)
	// has no opaque carrier, so the engine inspects the just-installed update for Grace-LSAs
	// and dispatches them to the shared helper. The IPv4 family instead delivers via the
	// ext-1 opaque OnReceive (graceOnReceive), so this scan is IPv6-only.
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		e.grInspectV6Update(name, up)
	}
}

func (e *engine) handleLSAck(rp transport.RawPacket, h Header) {
	e.mu.Lock()
	name, _, ok := e.receiveTargetLocked(rp.IfIndex, h)
	e.mu.Unlock()
	if !ok {
		return
	}
	ack, err := e.dispatch.codec.DecodeLSAck(rp.Payload)
	if err != nil {
		if e.transport != nil {
			e.transport.RecordDrop(name, dropReasonDecode)
		}
		return
	}
	if reason := e.neighbors.AcceptsFlooding(name, h.RouterID); reason != "" {
		if e.transport != nil {
			e.transport.RecordDrop(name, reason)
		}
		return
	}
	if e.lsdb != nil {
		if reason := e.lsdb.ReceiveAck(ospflsdb.AckInput{Interface: name, AreaID: h.AreaID, RouterID: h.RouterID, Ack: ack}); reason != "" && e.transport != nil {
			e.transport.RecordDrop(name, reason)
		}
	}
}

func (e *engine) handleNeighborPacket(rp transport.RawPacket, h Header, typ PacketType) {
	e.mu.Lock()
	name, _, ok := e.receiveTargetLocked(rp.IfIndex, h)
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
		switch {
		case err != nil:
			reason = dropReasonDecode
		case !e.afDBDescAccepted(dd):
			// RFC 5838 §2.5/§2.6: a non-default AF drops a DBDesc without the AF-bit so the
			// neighbor is never brought to Full, mirroring the Hello-path gate.
			reason = dropReasonAFBit
		default:
			reason = e.neighbors.HandleDBDesc(name, h.RouterID, dd)
		}
	case PacketTypeLSReq:
		lsr, err := e.dispatch.codec.DecodeLSReq(rp.Payload)
		if err != nil {
			reason = dropReasonDecode
		} else {
			reason = e.neighbors.HandleLSReq(name, h.RouterID, lsr)
		}
	default:
		return
	}
	if reason != "" && e.transport != nil {
		e.transport.RecordDrop(name, reason)
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
	// The engine adopts its configured Instance ID for the per-instance demux: OSPFv3 (RFC
	// 5340 sec 2.5) and OSPFv2 Multi-Instance (RFC 6549 sec 2/3.1) share the discard rule in
	// dispatcher.go. The transmit encoders must also stamp the Instance ID: setMetrics wires
	// the neighbor encoder too, but it is not called for the v6 engine, so wire both here
	// where the config (and Instance ID) is known.
	if e.dispatch != nil {
		e.dispatch.setInstanceID(cfg.InstanceID)
		e.installInstanceEncoders(cfg.InstanceID)
	}
	e.auth.configure(cfg)
	e.configureSPF(cfg)
	// RFC 3623 / RFC 5187: resolve the Graceful Restart policy onto the shared control plane
	// (restarter support/interval, helper support/strict-checking). Family-neutral.
	e.gr.configure(cfg.GracefulRestart)
	// RFC 4552 (IPv6 family only): push the desired per-interface IPsec so the
	// installer can install on interface-up and reconcile on a config change.
	if e.ipsec != nil {
		e.ipsec.setConfig(cfg.Interfaces)
	}
}

func (e *engine) acceptsArea(ifindex int, h Header) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if ic, ok := e.runningByIfIndexLocked(ifindex); ok && ic.AreaID == h.AreaID {
		return true
	}
	// RFC 2328 section 15: a routed virtual-link packet carries the backbone Area but
	// arrives on the transit interface; accept it when it matches a reachable configured
	// virtual link. Every other area mismatch remains a drop, so real-interface area
	// enforcement is unchanged.
	return e.virtualLinkTargetLocked(ifindex, h) != nil
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
		NBMANeighbors: reg.GaugeVec(
			"ze_ospf_nbma_neighbors",
			"Configured NBMA neighbor count by interface, address family, and poll state (attempt/heard).",
			[]string{"interface", "af", "state"},
		),
		NBMAPolls: reg.CounterVec(
			"ze_ospf_nbma_polls_total",
			"Total poll-rate Hellos sent to silent NBMA neighbors by interface and address family.",
			[]string{"interface", "af"},
		),
		PTMPHostRoutes: reg.GaugeVec(
			"ze_ospf_ptmp_host_routes",
			"Point-to-multipoint host routes contributed by interface and address family.",
			[]string{"interface", "af"},
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
	e.mInstanceMismatch = reg.CounterVec(
		"ze_ospf_instance_mismatch_drops_total",
		"Total OSPF packets dropped because their Instance ID did not match the receiving engine's configured Instance ID (RFC 6549), by interface.",
		[]string{"interface"},
	)
	// RFC 5838 §2.5: neighbors refused Full on a non-default AF for a missing AF-bit.
	e.mAFBitMismatch = reg.CounterVec(
		"ze_ospf_af_bit_mismatch_total",
		"Total OSPF neighbors not brought to Full for a missing RFC 5838 AF-bit on a non-default address family, by address family.",
		[]string{"af"},
	)
	e.setOpaqueMetrics(reg)
	e.setTEMetrics(reg)
	e.setRIMetrics(reg)
	e.setExtMetrics(reg)
	e.registerVirtualLinkMetrics(reg)
	e.neighbors = ospfneighbor.NewTable(e.neighborMetric)
	if e.lsdb != nil {
		e.lsdb.SetMetrics(reg)
		e.neighbors.SetLSDB(e.lsdb)
	} else {
		e.neighbors.SetLSDB(nil)
	}
	if e.transport != nil {
		e.neighbors.SetSender(e.virtualSender())
	}
	if e.dispatch != nil {
		// NewTable reset the encoder to the base v4 default; re-stamp the Instance ID so the
		// neighbor FSM (and the LSDB) keep encoding for this engine's instance (RFC 6549 /
		// RFC 5340). Idempotent for the base instance (identical output).
		e.installInstanceEncoders(e.cfg.InstanceID)
	}
	e.neighbors.SetEventSink(e.neighborEventSinkValue())
	e.mu.Unlock()
	if e.spf != nil {
		e.spf.SetMetrics(reg)
	}
	if e.ldpSync != nil {
		e.ldpSync.setMetrics(reg)
	}
	// RFC 3623 / RFC 5187 Graceful Restart series (spec-ospf-ext-9): ze_ospf_gr_* with a
	// family label, registered for both the IPv4 engine and each OSPFv3 AF engine.
	e.gr.setGRMetrics(reg)
}

func (e *engine) setEventSink(s *eventSink) {
	e.mu.Lock()
	e.sink = s
	if e.neighbors != nil {
		e.neighbors.SetEventSink(e.neighborEventSinkValue())
	}
	e.mu.Unlock()
}

func (e *engine) openInterfaces() error {
	// RFC 3623 sec 2.1 / RFC 5187: on start, if a planned restart fact is still within its
	// grace window, resume in-restart mode (suppress origination + install, retain the FIB)
	// before any self-LSA is originated. A stale/absent fact is a no-op (boots normally).
	e.gr.resumeFromNVS()
	// RFC 3623 sec 5: with unplanned-outage support enabled (opt-in) and no planned fact,
	// originate Grace-LSAs before any Hello for an unexpected restart. No-op by default.
	e.gr.maybeUnplannedRestart()
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
			// A per-interface open failure is retryable, never fatal to the engine.
			// The transport leaves the interface enabled-but-not-open and its
			// RescanInterfaces re-attempts it (v3 transport.go "HandleLinkUp" doc,
			// v2 the same), and the reconcile path in this file already logs and
			// continues on the very same call. Returning the error here made this
			// the lone escalating call site, and the escalation was total: one
			// interface that cannot open (a loopback, an IPv6-disabled link, or
			// IPv6 DAD not finished) propagated through v6EngineSet.start /
			// instanceManager.start to the plugin's post-startup callback, which
			// exits the WHOLE ospf plugin -- every instance and every address
			// family -- over one link. Log the subject and keep the others.
			e.log.Warn("ospf: interface open failed", "interface", ic.Name, "err", err)
			continue
		}
	}
	// Create the LDP-sync machines for any enabled ldp-sync interface now open, so a
	// link that comes up before LDP originates at LSInfinity (RFC 5443 §2, AC-1).
	e.updateLDPSyncMachines()
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
						if e.ldpSync != nil {
							e.ldpSync.refreshGauges(now)
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
	// Reconcile mandatory ABR and configured internal-router Type 7 defaults.
	e.applyNSSADefaults()
	// Re-run translator election + Type 7 -> Type 5 translation (role/attachment change).
	e.translateNSSA(time.Now())
	// RFC 4552 AC-11: reconcile kernel IPsec against the new config (a changed SPI/key/
	// algorithm removes the old SA/policy and installs the new; a removed block clears it).
	if e.ipsec != nil {
		e.ipsec.reconcileAll()
	}
	// Converge BFD sessions with the reloaded per-interface config: open for already-Full
	// neighbors on newly-enabled interfaces (AC-11), release on disabled/removed ones (AC-10),
	// without bouncing the adjacency.
	e.reconcileBFD(desired)
	// Reconcile the per-interface LDP-sync machines to the new config (create/remove/
	// update); re-originates if the managed set changed.
	e.updateLDPSyncMachines()
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
	// The interface's neighbors are deleted without an NSM emit, so release their BFD sessions
	// here (outside e.mu; stopBFDSession joins the subscriber, which never needs e.mu).
	e.bfdReleaseInterface(name)
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
		e.neighbors.ConfigureInterface(neighborInterfaceConfig(cfg, e.cfg.Opaque))
	}
	rt := ospfiface.New(cfg, sender, e.ifaceMetric)
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		// OSPFv3 interface: send Hellos through the v6 encoder (ospfv3/packet).
		rt.SetEncoder(v6Encoder{instanceID: e.cfg.InstanceID, emitAF: e.emitAFBit()})
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
		// RFC 6549: the OSPFv2 interface stamps this engine's Instance ID into every Hello.
		// The OSPFv3 family threads its Instance ID through the v6 encoder instead, so this
		// stays 0 for the v6 engine (its Hello encoder is swapped in startInterfaceLocked).
		InstanceID:    instanceIDForV4(e.dispatch, e.cfg.InstanceID),
		PollInterval:  ic.pollInterval(),
		NBMANeighbors: nbmaNeighbors(ic.nbmaNeighborList()),
		BFDEnabled:    ic.BFD.Enabled,
		BFDMinTxUs:    ic.BFD.MinTxUs,
		BFDMinRxUs:    ic.BFD.MinRxUs,
		BFDMultiplier: ic.BFD.Multiplier,
	}
}

// nbmaNeighbors maps the resolved config neighbor list to the iface runtime form.
func nbmaNeighbors(in []nbmaNeighborConfig) []ospfiface.NBMANeighbor {
	if len(in) == 0 {
		return nil
	}
	out := make([]ospfiface.NBMANeighbor, 0, len(in))
	for _, n := range in {
		out = append(out, ospfiface.NBMANeighbor{
			Address:   n.Address,
			RouterID:  n.RouterID,
			LinkLocal: n.LinkLocal,
			Priority:  n.Priority,
		})
	}
	return out
}

func neighborInterfaceConfig(cfg ospfiface.Config, opaque bool) ospfneighbor.InterfaceConfig {
	opts := ospfOptionsForAreaType(cfg.AreaType)
	// RFC 5250 §3.1 / App A.1: advertise opaque capability by setting the O-bit in
	// Database Description packets when opaque is enabled. The O-bit is a DD-only signal:
	// it is NOT set in Hellos (expectedOptionsLocked) nor part of the Hello E/N match, so
	// enabling it does not break adjacency with a non-opaque peer (A-6).
	if opaque {
		opts = opts.Set(types.OptionO)
	}
	return ospfneighbor.InterfaceConfig{
		Name:               cfg.Name,
		AreaID:             cfg.AreaID,
		RouterID:           cfg.RouterID,
		NetworkType:        cfg.NetworkType,
		InterfaceAddress:   cfg.InterfaceAddress,
		Options:            opts,
		InterfaceMTU:       cfg.InterfaceMTU,
		MTUIgnore:          cfg.MTUIgnore,
		DeadInterval:       cfg.DeadInterval,
		RetransmitInterval: cfg.RetransmitInterval,
		BFDEnabled:         cfg.BFDEnabled,
		BFDMinTxUs:         cfg.BFDMinTxUs,
		BFDMinRxUs:         cfg.BFDMinRxUs,
		BFDMultiplier:      cfg.BFDMultiplier,
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
	sink ospfneighbor.EventSink
	// onFull/onLost bridge the AF-neutral Full<->non-Full transition to the BFD lifecycle
	// (open a session on Full, release it on leaving Full). BFD-for-OSPF layers ON TOP of the
	// existing sink + self-LSA re-origination, never in place of them.
	onFull   func(ospfneighbor.Snapshot)
	onLost   func(ospfneighbor.Snapshot)
	onChange func()
}

func (s neighborEventSink) NeighborUp(snap ospfneighbor.Snapshot) {
	if s.sink != nil {
		s.sink.NeighborUp(snap)
	}
	if s.onFull != nil {
		s.onFull(snap)
	}
	if s.onChange != nil {
		s.onChange()
	}
}

func (s neighborEventSink) NeighborDown(snap ospfneighbor.Snapshot) {
	if s.sink != nil {
		s.sink.NeighborDown(snap)
	}
	if s.onLost != nil {
		s.onLost(snap)
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

func (e *engine) onInterfaceDown(ifindex int, name string) {
	// RFC 4552: tear down the kernel IPsec for this interface before dropping adjacencies.
	if e.ipsec != nil {
		e.ipsec.onInterfaceDown(ifindex, name)
	}
	e.mu.Lock()
	e.markInterfaceDownLocked(name)
	e.mu.Unlock()
	// RFC 5443 §2 / A-8: interface down resets the LDP-sync machine so the next
	// bring-up starts not-synchronized (costed out). Called outside e.mu because the
	// reset re-originates (re-enters e.mu).
	if e.ldpSync != nil {
		e.ldpSync.reset(name)
	}
}

func (e *engine) onInterfaceUp(ifindex int, name string) {
	// RFC 4552 R-1/AC-7: install the kernel policy+SA BEFORE starting the interface FSM
	// (below) so the first Hello is already protected. Runs outside e.mu (the installer
	// has its own lock and calls netlink).
	if e.ipsec != nil {
		e.ipsec.onInterfaceUp(ifindex, name)
	}
	if e.startInterfaceUpLocked(name) {
		// Bring the LDP-sync machine up in not-synchronized for a returning link.
		e.updateLDPSyncMachines()
	}
}

// startInterfaceUpLocked (re)starts a configured, area-bound interface after a link-up
// and reports whether it did, so the caller can reconcile the LDP-sync machines outside
// e.mu (updateLDPSyncMachines re-originates, which re-enters e.mu).
func (e *engine) startInterfaceUpLocked(name string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, ic := range e.cfg.Interfaces {
		if ic.Name != name || !ic.Enabled || ic.Passive || ic.NetworkType == networkLoopback {
			continue
		}
		bound := false
		for _, a := range e.cfg.Areas {
			if a.AreaID == ic.AreaID {
				bound = true
				break
			}
		}
		if !bound {
			return false
		}
		e.running[name] = ic
		e.startInterfaceLocked(ic)
		return true
	}
	return false
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
		a.Authentication == b.Authentication &&
		ipsecEqual(a.IPsec, b.IPsec) &&
		a.LDPSyncEnabled == b.LDPSyncEnabled &&
		a.LDPSyncHoldDown == b.LDPSyncHoldDown
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
	// RFC 5443 R-7: drop the LDP event subscription and stop every per-interface timer
	// first so no stale handler reads freed engine state during teardown.
	if e.ldpSyncUnsub != nil {
		e.ldpSyncUnsub()
		e.ldpSyncUnsub = nil
	}
	if e.ldpSync != nil {
		e.ldpSync.stop()
	}
	// spec-ospf-ext-7: stop every synthetic virtual interface (spawned goroutines) before
	// releasing the rest of the engine.
	e.stopVirtualInterfaces()
	// Stop any pending graceful-restart timers so no restarter/helper callback fires after
	// teardown (spec-ospf-ext-9). A graceful-restart stop retains the FIB (suppressInstall);
	// a normal shutdown lets the SPF Computer.Stop run RemoveAll.
	e.gr.stop()
	// Release BFD sessions (and join their subscribers) before canceling the engine context.
	e.bfdStopAll()
	if e.spf != nil {
		e.spf.Stop()
	}
	if e.ipsec != nil {
		e.ipsec.Close()
	}
	e.cancel()
	if e.transport != nil {
		e.transport.Close()
	}
	e.wg.Wait()
}

func (e *engine) originateSelfLSAs() {
	// RFC 3623 sec 2: while in graceful restart the restarting router MUST NOT originate its
	// self-LSAs; it relies on its pre-restart LSAs. This is the shared chokepoint, so gating
	// it here suppresses every self-LSA type for BOTH address families (A-7).
	if e.gr.suppressOrigination() {
		return
	}
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
	// RFC 5250 §3: when opaque capability is enabled, drive each registered consumer's
	// opaque LSA origination on the same self-LSA pass. Skipped when disabled so a router
	// without the `opaque` leaf neither advertises the O-bit nor originates opaque LSAs.
	if cfg.Opaque {
		e.originateOpaqueLSAs(cfg.RouterID)
	}
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
			neighbors = append(neighbors, ospflsdb.NeighborInfo{RouterID: n.RouterID, Address: n.Address, State: n.State, InterfaceID: n.InterfaceID, OpaqueCapable: n.OpaqueCapable})
		}
		// RFC 3623 sec 3: while helping a restarting neighbor X, keep advertising the
		// adjacency to X (as Full) regardless of the live NSM state, and keep X as DR if X
		// was DR. Merge the helper sessions into the topology view (both families).
		neighbors = e.gr.mergeHelpingNeighbors(ic.Name, neighbors)
		if x, ok := e.gr.helperDR(ic.Name); ok {
			dr = x
		}
		// RFC 3623 sec 2: while this router is itself restarting, re-assert its DR role on a
		// segment where a Waiting-state Hello still lists it as DR (it was DR before restart).
		if e.gr.shouldReElectSelfDR(state == ospfiface.StateWaiting.String(), dr == cfg.RouterID) {
			dr = cfg.RouterID
		}
		info := ospflsdb.InterfaceInfo{
			Name:               ic.Name,
			AreaID:             ic.AreaID,
			AreaType:           areaKind,
			NetworkType:        string(ic.NetworkType),
			State:              state,
			Priority:           ic.Priority,
			Passive:            ic.Passive,
			Address:            interfaceIPv4Address(ic.Name),
			NetworkMask:        interfaceNetworkMask(ic.Name),
			InterfaceID:        e.grInterfaceID(ic.Name),
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
			IPv6Addresses:      interfaceIPv6Addresses(ic.Name),
		}
		// RFC 5443 / RFC 6138 LDP-IGP sync: while an ldp-sync interface is not yet
		// synchronized, force the P2P link metric to LSInfinity, or (broadcast, non-
		// cut-edge) withhold the transit link. Computed at origination only, so the
		// configured cost survives for restoration.
		e.applyLDPSyncOverride(&info, ic)
		out = append(out, info)
	}
	// spec-ospf-ext-7: append each reachable virtual link as a synthetic backbone
	// point-to-point interface so origination emits its Type-4 / RouterLinkTypeVirtual
	// record. Backbone-only by construction (the synthetic InterfaceInfo is always Area 0).
	return append(out, e.virtualLinkTopology()...)
}

// The engine's `show ospf ...` snapshot accessors live in instance_snapshots.go.
