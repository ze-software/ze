// Design: internal/component/bgp/reactor/peer_bfd.go -- the EXEMPLAR BFD client this mirrors.
// Design: plan/learned/560-bfd-3-bgp-client.md -- nil-safe Service lookup, per-session
// subscriber worker, stop+done handshake.
// RFC: rfc/short/rfc5881.md (single-hop UDP 3784, both ends Active, TTL/Hop-Limit 255 GTSM);
// rfc/short/rfc5880.md (Down/AdminDown detection); rfc/short/rfc5882.md (client model:
// BFD is a failure detector, not a session driver).
//
// BFD-for-OSPF glue on the unified v2/v3 engine. When an OSPF adjacency reaches Full on a
// BFD-enabled interface and the in-process BFD engine is available, the engine opens a
// single-hop BFD session for the neighbor. A BFD Down/AdminDown drives the AF-neutral NSM
// down seam (neighbor.Table.NeighborDown), declaring the neighbor dead far faster than the
// RouterDeadInterval timer. BFD is strictly additive: with no BFD plugin, or BFD not enabled
// on the interface, OSPF runs exactly as before on the Hello/Dead timers, on both families.
//
// The lifecycle, client map, subscriber discipline, and metrics are AF-neutral and shared;
// only the request builder forks by codec.IsV6() (bfdRequestForNeighbor vs
// bfdRequestForNeighborV6). Each engine instance (IPv4 and IPv6) owns its own client map, so
// a v6 down never touches a v4 neighbor (R-4).
package ospf

import (
	"net/netip"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/core/metrics"
	ospfneighbor "github.com/ze-software/ze/internal/plugins/ospf/neighbor"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

const (
	// bfdRegisterReasonPluginAbsent labels a register failure caused by the BFD plugin not
	// being loaded in this process (api.GetService returned nil).
	bfdRegisterReasonPluginAbsent = "plugin-absent"
	// bfdRegisterReasonEnsureError labels a register failure from EnsureSession itself.
	bfdRegisterReasonEnsureError = "ensure-error"
	// neighborStateFull is the neighbor.Snapshot / FloodNeighbor State string for a Full
	// adjacency (the only state that gets a BFD session).
	neighborStateFull = "full"
)

// ospfBFDMetrics is the shared ze_ospf_bfd_* series. The registry is get-or-create by name,
// so the IPv4 and IPv6 engine instances produce ONE series each and the interface label
// distinguishes the families (learned 970: unified ze_ospf_* namespace, never ze_ospfv3_*).
type ospfBFDMetrics struct {
	Sessions         metrics.GaugeVec   // labels: interface, area
	SessionDownTotal metrics.CounterVec // labels: interface
	RegisterFailures metrics.CounterVec // labels: interface, reason
}

func nopBFDMetrics() ospfBFDMetrics {
	nop := metrics.NopRegistry{}
	return ospfBFDMetrics{
		Sessions:         nop.GaugeVec("", "", nil),
		SessionDownTotal: nop.CounterVec("", "", nil),
		RegisterFailures: nop.CounterVec("", "", nil),
	}
}

// bfdClientKey identifies a per-neighbor BFD client within one engine instance.
type bfdClientKey struct {
	iface  string
	router types.RouterID
}

// bfdClient holds the per-neighbor BFD session state. All fields are guarded by engine.bfdMu
// except the subscriber's read of stop/sub/done/handle (immutable after creation). Mirrors
// reactor.bfdClient: a per-session subscriber worker with a stop+done handshake.
type bfdClient struct {
	key    bfdClientKey
	area   string
	handle api.SessionHandle
	sub    <-chan api.StateChange
	stop   chan struct{}
	done   chan struct{}
	// state is the last BFD state observed by the subscriber, surfaced by `show ospf
	// neighbor`. Guarded by engine.bfdMu.
	state string
}

// setBFDMetrics installs the shared ze_ospf_bfd_* series. Called once per engine instance
// (both IPv4 and IPv6) with the same registry, so the get-or-create registry returns one
// shared series per name.
func (e *engine) setBFDMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	e.mu.Lock()
	e.bfdMetrics = ospfBFDMetrics{
		Sessions: reg.GaugeVec(
			"ze_ospf_bfd_sessions",
			"Current OSPF BFD sessions by interface and area.",
			[]string{"interface", "area"},
		),
		SessionDownTotal: reg.CounterVec(
			"ze_ospf_bfd_session_down_total",
			"Total OSPF adjacencies declared down by a BFD session failure, by interface.",
			[]string{"interface"},
		),
		RegisterFailures: reg.CounterVec(
			"ze_ospf_bfd_register_failures_total",
			"Total OSPF BFD session registration failures, by interface and reason.",
			[]string{"interface", "reason"},
		),
	}
	e.mu.Unlock()
}

// neighborEventSinkValue builds the AF-neutral neighbor lifecycle sink: the report-bus sink,
// the BFD open/release hooks, and the self-LSA re-origination onChange. Shared by every site
// that installs the sink so the three stay in lockstep.
func (e *engine) neighborEventSinkValue() neighborEventSink {
	return neighborEventSink{
		sink:     e.sink,
		onFull:   e.onNeighborFull,
		onLost:   e.bfdNeighborLost,
		onChange: e.originateSelfLSAs,
	}
}

// onNeighborFull is the AF-neutral onFull hook: it bridges a neighbor reaching Full to every
// subsystem that reacts to that transition. Today that is the BFD session open (RFC 5880/5881)
// and the RFC 3623 sec 2.2 trigger-1 Graceful Restart early exit (all pre-restart adjacencies
// re-Full). BFD-for-OSPF and GR both layer ON TOP of the existing sink; neither replaces it.
func (e *engine) onNeighborFull(snap ospfneighbor.Snapshot) {
	e.bfdNeighborFull(snap)
	e.grNeighborFull(snap)
	// Segment Routing (spec-ospf-ext-5 AC-12): allocate an Adj-SID from the SRLB and
	// install its pop/forward entry when a neighbor reaches Full (>= 2-Way).
	e.srAdjNeighborFull(snap)
}

// bfdNeighborFull opens a single-hop BFD session for a neighbor that has reached Full, when
// BFD is enabled on its interface and the BFD plugin is loaded. Called from
// neighborEventSink.NeighborUp (AF-neutral). No-op when BFD is not enabled or the neighbor's
// address is not yet known -- OSPF then runs on the Hello/Dead timers alone.
func (e *engine) bfdNeighborFull(snap ospfneighbor.Snapshot) {
	id, err := types.ParseRouterID(snap.RouterID)
	if err != nil {
		return
	}
	cfg, ok := e.interfaceBFDConfig(snap.Interface)
	if !ok || !cfg.Enabled {
		return
	}
	// R-2: fetch the raw netip.Addr (with any IPv6 zone), not the Snapshot.Address string.
	peer, ok := e.neighbors.NeighborAddress(snap.Interface, id)
	if !ok {
		return
	}
	e.startBFDSession(snap.Interface, id, snap.Area, peer, cfg)
}

// bfdNeighborLost releases the BFD session for a neighbor that has left Full for ANY reason
// (inactivity timer, interface down, clear, or a BFD-driven down). Called from
// neighborEventSink.NeighborDown. Idempotent: a no-op when no session is open (e.g. the
// subscriber already tore itself down on a BFD Down, avoiding a self-join deadlock).
func (e *engine) bfdNeighborLost(snap ospfneighbor.Snapshot) {
	// Segment Routing (spec-ospf-ext-5 AC-13): withdraw the Adj-SID + free its SRLB
	// label when a neighbor leaves Full (RFC 8665 §7.4.1 / RFC 8666 §8.4.1).
	e.srAdjNeighborLost(snap)
	id, err := types.ParseRouterID(snap.RouterID)
	if err != nil {
		return
	}
	e.stopBFDSession(bfdClientKey{iface: snap.Interface, router: id})
}

// interfaceBFDConfig returns the per-interface BFD config from the running config.
func (e *engine) interfaceBFDConfig(iface string) (bfdInterfaceConfig, bool) {
	e.mu.Lock()
	ic, ok := e.running[iface]
	e.mu.Unlock()
	if !ok {
		return bfdInterfaceConfig{}, false
	}
	return ic.BFD, true
}

// startBFDSession opens a single-hop BFD session for one Full neighbor and starts its
// subscriber. Idempotent: a no-op when a session already exists for the neighbor (a reload
// enable racing an up event). Nil-safe: with no BFD plugin it logs once, counts the failure,
// and returns -- the adjacency is NOT blocked (additive contract, learned 560).
func (e *engine) startBFDSession(iface string, id types.RouterID, area string, peer netip.Addr, cfg bfdInterfaceConfig) {
	key := bfdClientKey{iface: iface, router: id}

	e.bfdMu.Lock()
	_, exists := e.bfdClients[key]
	e.bfdMu.Unlock()
	if exists {
		return
	}

	svc := api.GetService()
	if svc == nil {
		e.bfdWarnOnce.Do(func() {
			e.log.Warn("bfd enabled on ospf interface but BFD plugin not loaded; running on hello/dead timers",
				"interface", iface)
		})
		e.bfdMetrics.RegisterFailures.With(iface, bfdRegisterReasonPluginAbsent).Inc()
		return
	}
	req := e.bfdRequestForFamily(iface, peer, cfg)
	handle, err := svc.EnsureSession(req)
	if err != nil {
		e.log.Warn("bfd EnsureSession failed for ospf neighbor; running on hello/dead timers",
			"interface", iface, "neighbor", id.String(), "err", err)
		e.bfdMetrics.RegisterFailures.With(iface, bfdRegisterReasonEnsureError).Inc()
		return
	}
	sub := handle.Subscribe()
	c := &bfdClient{
		key:    key,
		area:   area,
		handle: handle,
		sub:    sub,
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
		state:  api.StateDown.String(),
	}

	e.bfdMu.Lock()
	if _, dup := e.bfdClients[key]; dup {
		// Lost a race to another opener; roll back this handle.
		e.bfdMu.Unlock()
		handle.Unsubscribe(sub)
		_ = svc.ReleaseSession(handle)
		return
	}
	e.bfdClients[key] = c
	e.bfdMu.Unlock()

	e.bfdMetrics.Sessions.With(iface, area).Inc()
	e.log.Info("bfd session opened for ospf neighbor",
		"interface", iface, "neighbor", id.String(), "peer", peer.String(),
		"min-tx-us", req.DesiredMinTxInterval, "min-rx-us", req.RequiredMinRxInterval, "mult", req.DetectMult)
	go e.runBFDSubscriber(c, svc)
}

// runBFDSubscriber is the per-session subscriber worker (one per BFD-protected neighbor). It
// drains the subscription channel until stop is signaled or the channel closes.
//
// On Down/AdminDown it declares the OSPF neighbor down through the existing NSM seam. It
// detaches ITSELF from the client map first (without joining, since it IS the subscriber),
// then calls NeighborDown -- so the resulting NeighborLost release finds nothing and never
// deadlocks waiting on this goroutine, then returns (closing done). Up/Init are logged at
// debug and do NOT change OSPF state (RFC 5882: failure detector, not a session driver).
func (e *engine) runBFDSubscriber(c *bfdClient, svc api.Service) {
	defer close(c.done)
	for {
		select {
		case <-c.stop:
			return
		case change, ok := <-c.sub:
			if !ok {
				return
			}
			e.bfdMu.Lock()
			c.state = change.State.String()
			e.bfdMu.Unlock()
			// RFC 5880 sec 6.8.1: Down carries Diag 1 (Control Detection Time Expired) on a
			// timer miss and Diag 3 (Neighbor Signaled Session Down) when the peer reports Down.
			// OSPF treats BOTH StateDown and StateAdminDown as "neighbor down" regardless of Diag.
			if change.State == api.StateDown || change.State == api.StateAdminDown {
				e.log.Warn("bfd reported ospf neighbor down; declaring adjacency down",
					"interface", c.key.iface, "neighbor", c.key.router.String(),
					"bfd-state", change.State.String(), "bfd-diag", change.Diag.String())
				e.bfdMetrics.SessionDownTotal.With(c.key.iface).Inc()
				e.detachBFDClient(c, svc)
				e.neighbors.NeighborDown(c.key.iface, c.key.router)
				return
			}
			// RFC 5882: a BFD Up/Init transition does not itself bring the OSPF adjacency up.
			e.log.Debug("bfd state change for ospf neighbor",
				"interface", c.key.iface, "neighbor", c.key.router.String(), "bfd-state", change.State.String())
		}
	}
}

// detachBFDClient removes c from the client map, unsubscribes, and releases the session
// WITHOUT joining the subscriber goroutine (the caller IS that goroutine). Idempotent: a
// no-op if c was already removed (an external stop won the race). Decrements the gauge on
// the single successful removal.
func (e *engine) detachBFDClient(c *bfdClient, svc api.Service) {
	e.bfdMu.Lock()
	existing, ok := e.bfdClients[c.key]
	if !ok || existing != c {
		e.bfdMu.Unlock()
		return
	}
	delete(e.bfdClients, c.key)
	e.bfdMu.Unlock()

	c.handle.Unsubscribe(c.sub)
	if svc != nil {
		if err := svc.ReleaseSession(c.handle); err != nil {
			e.log.Debug("bfd ReleaseSession failed", "interface", c.key.iface, "err", err)
		}
	}
	e.bfdMetrics.Sessions.With(c.key.iface, c.area).Dec()
}

// stopBFDSession releases the BFD session for a neighbor and waits for its subscriber to
// exit (the stop+done handshake). Called when the neighbor leaves Full, the interface is
// removed, BFD is disabled by reload, or the engine shuts down. Idempotent: a no-op when no
// session is open for the key. MUST NOT be called from the subscriber goroutine (it would
// deadlock on <-done); the subscriber uses detachBFDClient instead.
func (e *engine) stopBFDSession(key bfdClientKey) {
	e.bfdMu.Lock()
	c, ok := e.bfdClients[key]
	if !ok {
		e.bfdMu.Unlock()
		return
	}
	delete(e.bfdClients, key)
	e.bfdMu.Unlock()

	close(c.stop)
	c.handle.Unsubscribe(c.sub)
	// R-6: the BFD plugin may have shut down (SetService(nil)); ReleaseSession on a torn-down
	// loop is a documented no-op. The subscriber still exits on the closed stop channel.
	if svc := api.GetService(); svc != nil {
		if err := svc.ReleaseSession(c.handle); err != nil {
			e.log.Debug("bfd ReleaseSession failed", "interface", key.iface, "err", err)
		}
	}
	<-c.done
	e.bfdMetrics.Sessions.With(key.iface, c.area).Dec()
	e.log.Info("bfd session closed for ospf neighbor", "interface", key.iface, "neighbor", key.router.String())
}

// bfdReleaseInterface releases every BFD session on an interface. Used when an interface is
// removed from config (its neighbors are deleted without an NSM emit).
func (e *engine) bfdReleaseInterface(iface string) {
	e.bfdMu.Lock()
	keys := make([]bfdClientKey, 0)
	for key := range e.bfdClients {
		if key.iface == iface {
			keys = append(keys, key)
		}
	}
	e.bfdMu.Unlock()
	for _, key := range keys {
		e.stopBFDSession(key)
	}
}

// bfdStopAll releases every BFD session. Called on engine shutdown.
func (e *engine) bfdStopAll() {
	e.bfdMu.Lock()
	keys := make([]bfdClientKey, 0, len(e.bfdClients))
	for key := range e.bfdClients {
		keys = append(keys, key)
	}
	e.bfdMu.Unlock()
	for _, key := range keys {
		e.stopBFDSession(key)
	}
}

// bfdSessionState returns the last-observed BFD state string for a neighbor, for `show ospf
// neighbor`. The second return is false when no BFD session is open for the neighbor.
func (e *engine) bfdSessionState(key bfdClientKey) (string, bool) {
	e.bfdMu.Lock()
	defer e.bfdMu.Unlock()
	c, ok := e.bfdClients[key]
	if !ok {
		return "", false
	}
	return c.state, true
}

// reconcileBFD converges the live BFD client map with the interfaces' BFD config after a
// config reload. It opens sessions for already-Full neighbors on newly-enabled interfaces
// (AC-11) and releases sessions on disabled or removed interfaces (AC-10) WITHOUT bouncing
// the adjacency. Idempotent: startBFDSession is a no-op for a neighbor that already has a
// session, so calling this on every apply is safe.
func (e *engine) reconcileBFD(desired map[string]interfaceConfig) {
	type want struct {
		enabled bool
		cfg     bfdInterfaceConfig
		area    string
	}
	e.mu.Lock()
	wants := make(map[string]want, len(e.running))
	for name, ic := range e.running {
		// A BFD-only change does not enter the interface restart branch, so update the running
		// config's BFD here from the desired config.
		if d, ok := desired[name]; ok {
			ic.BFD = d.BFD
			e.running[name] = ic
		}
		wants[name] = want{enabled: ic.BFD.Enabled, cfg: ic.BFD, area: ic.AreaID.String()}
	}
	e.mu.Unlock()

	// Release sessions on interfaces whose BFD is now disabled or that were removed.
	e.bfdMu.Lock()
	toRelease := make([]bfdClientKey, 0)
	for key := range e.bfdClients {
		if w, ok := wants[key.iface]; !ok || !w.enabled {
			toRelease = append(toRelease, key)
		}
	}
	e.bfdMu.Unlock()
	for _, key := range toRelease {
		e.stopBFDSession(key)
	}

	// Open sessions for already-Full neighbors on enabled interfaces.
	for name, w := range wants {
		if !w.enabled {
			continue
		}
		for _, n := range e.neighbors.FloodNeighbors(name) {
			if n.State != neighborStateFull {
				continue
			}
			e.startBFDSession(name, n.RouterID, w.area, n.Address, w.cfg)
		}
	}
}

// bfdRequestForFamily builds the single-hop SessionRequest for a neighbor, dispatching by
// address family (A-6). The IPv4 (OSPFv2) family uses the interface IPv4 address as Local;
// the IPv6 (OSPFv3) family uses the interface's IPv6 link-local (never the [4]byte
// InterfaceAddress). Selected by codec.IsV6().
func (e *engine) bfdRequestForFamily(iface string, peer netip.Addr, cfg bfdInterfaceConfig) api.SessionRequest {
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		return bfdRequestForNeighborV6(peer, interfaceIPv6LinkLocal(iface), iface, cfg)
	}
	var local netip.Addr
	if v4 := interfaceIPv4Address(iface); v4 != ([4]byte{}) {
		local = netip.AddrFrom4(v4)
	}
	return bfdRequestForNeighbor(peer, local, iface, cfg)
}

// bfdRequestForNeighbor builds the IPv4 (OSPFv2) single-hop request. RFC 5881 sec 6: the
// on-subnet source/destination pair -- Peer is the neighbor's IPv4 address, Local the
// interface IPv4 address. RFC 5881 sec 3: both ends Active (Passive left false, the api
// default) so the session comes up symmetrically with FRR. RFC 5881 sec 4/5: Mode SingleHop
// binds the session to one egress interface and enforces TTL 255 (GTSM) inside the engine.
func bfdRequestForNeighbor(peer, local netip.Addr, ifname string, cfg bfdInterfaceConfig) api.SessionRequest {
	return api.SessionRequest{
		Peer:                  peer,
		Local:                 local,
		Interface:             ifname,
		Mode:                  api.SingleHop,
		DesiredMinTxInterval:  cfg.MinTxUs,
		RequiredMinRxInterval: cfg.MinRxUs,
		DetectMult:            cfg.Multiplier,
	}
}
