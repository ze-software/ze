// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- following a dynamic WAN address
// Related: apply.go -- applyConfig and rebindListeners, which this worker re-runs on an address event
// Related: reconcile.go -- reconcilePeersWith, which starts each peer on the socket pair of its origin
// Related: respond_names.go -- the DNS responder cache this worker refreshes
// Related: doctor_interface_address.go -- the doctor view of the peers that wait
// RFC: rfc/short/rfc4555.md -- address change triggers (Section 3.5)

package engine

import (
	"context"
	"errors"
	"maps"
	"net"
	"slices"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	"github.com/ze-software/ze/internal/core/network"
	"github.com/ze-software/ze/pkg/ze"
)

// addressQueueDepth bounds the address events waiting for the worker. An event is a
// trigger and the interface resolver is the truth, so one pending trigger carries any
// number of events: a send that finds the slot full drops the event, and the re-read the
// pending trigger causes already sees what it announced.
const addressQueueDepth = 1

// watchAddressTableFn starts the watch of the kernel address table the worker keeps open.
// It is network.WatchAddresses; a test replaces it and restores it in t.Cleanup, because
// the watcher registry is a one-shot slot the netlink backend's init() fills.
var watchAddressTableFn = network.WatchAddresses

// addressWatchRetry is the wait before the worker starts the address table watch again
// after one failed to start or ended on its own.
const addressWatchRetry = 5 * time.Second

// peerPlan is what one configuration means for the address the engine has now: which
// peers run, each with the local address it sends from, which socket pair each peer
// holds, and which peers wait for an address.
//
// The listener set it describes (spec home-cpe-4, "Listener set"):
//   - no `interface`: every peer holds the primary pair, bound as ikeListenHost says;
//   - `interface` set, the peer names no local-address, or the interface's current
//     address: the primary pair, bound to that address and absent while there is none;
//   - `interface` set, the peer names another literal: a pair bound to that literal, so
//     an interface address event never touches its socket or its session.
type peerPlan struct {
	// cfg is the configuration the sessions run: the operator's, with the interface
	// address filled into every peer that names no local-address, and the peers that
	// wait for one left out. It is a copy; the operator's configuration is not changed.
	cfg *ipsec.IPsecConfig
	// hostOf is the bind host of the pair each running peer holds.
	hostOf map[string]string
	// primaryHost is the host of the primary pair, empty when the interface has no
	// address and the pair is closed.
	primaryHost string
	// literalHosts are the hosts of the pairs bound to a peer's own literal, `interface`
	// mode only.
	literalHosts map[string]bool
	// waiting are the peers held back because the interface has no address, sorted.
	waiting []string
}

// planPeers derives the plan of cfg for the interface address ifIP (empty: none, or the
// lookup failed). It is a pure function of its inputs, so the apply path and the address
// worker reach the same answer for the same address.
func planPeers(cfg *ipsec.IPsecConfig, ifIP string) *peerPlan {
	running := *cfg
	running.Peers = make(map[string]ipsec.SiteToSitePeer, len(cfg.Peers))
	plan := &peerPlan{cfg: &running, hostOf: make(map[string]string, len(cfg.Peers)), literalHosts: map[string]bool{}}

	if cfg.Interface == "" {
		// One pair for every peer, as before the listener set: the wildcard in
		// production, and under the test port the first local-address found.
		peerLocal := ""
		for name := range cfg.Peers {
			if la := cfg.Peers[name].LocalAddress; la != "" {
				peerLocal = la
				break
			}
		}
		plan.primaryHost = ikeListenHost("", peerLocal)
		for name := range cfg.Peers {
			running.Peers[name] = cfg.Peers[name]
			plan.hostOf[name] = plan.primaryHost
		}
		return plan
	}

	plan.primaryHost = ifIP
	for name := range cfg.Peers {
		peer := cfg.Peers[name]
		host := ifIP
		switch {
		case peer.LocalAddress == "" && ifIP == "":
			plan.waiting = append(plan.waiting, name)
			continue
		case peer.LocalAddress == "":
			peer.LocalAddress = ifIP
		case peer.LocalAddress == ifIP:
			// One address and port cannot be bound twice, so a literal equal to the
			// interface address shares its pair, and follows it when it leaves.
		default:
			host = peer.LocalAddress
			plan.literalHosts[host] = true
		}
		running.Peers[name] = peer
		plan.hostOf[name] = host
	}
	slices.Sort(plan.waiting)
	return plan
}

// primaryWanted reports whether a running peer holds the primary pair.
func (p *peerPlan) primaryWanted() bool {
	if p.primaryHost == "" {
		return false
	}
	for _, host := range p.hostOf {
		if host == p.primaryHost {
			return true
		}
	}
	return false
}

// publishWaiting hands the doctor the peers this plan holds back. The caller holds
// applyMu.
func publishWaiting(cfg *ipsec.IPsecConfig, plan *peerPlan) {
	interfaceWaitPtr.Store(&interfaceWait{iface: cfg.Interface, peers: slices.Clone(plan.waiting)})
}

// addressWorker follows interface addresses for one engine: it re-reads the configured
// interface and the bound XFRM interfaces on every change of the kernel address table,
// on every `interface/addr-added`, `interface/addr-removed` and `interface/created`
// event, and refreshes the `respond` peers' names every respondNameRefresh.
//
// The kernel address table watch is the source that always runs. The bus events come
// from the iface component's monitor, which starts only when the configuration has an
// `interface` block: `vpn ipsec interface ppp0` with no such block (the WAN is named only
// here) has no monitor, so with the bus alone a WAN that came up after start was never
// followed. The bus subscriptions stay because they carry what the kernel table does
// not: an interface created with no address, and the addresses a VPP backend reports.
//
// Lifecycle: startAddressWorker creates the trigger channel, subscribes, and starts the
// one goroutine, which opens the address table watch and keeps it open; kick enqueues
// without blocking; Stop unsubscribes, cancels the watch, closes stop and waits for the
// goroutine. The goroutine reaches engine state only through
// ikeEngineState.followAddress, which takes applyMu, so it never runs beside an apply.
type addressWorker struct {
	trigger chan struct{}
	stop    chan struct{}
	done    chan struct{}
	unsub   []func()
	// cancel ends the address table watch; the watcher then closes its channel.
	cancel context.CancelFunc
}

// startAddressWorker subscribes to the interface address events on bus and starts the
// worker, which watches the kernel address table whether or not bus is set. A nil bus (a
// unit test that wires none) starts the worker with no subscription.
func startAddressWorker(s *ikeEngineState, bus ze.EventBus) *addressWorker {
	ctx, cancel := context.WithCancel(context.Background())
	w := &addressWorker{
		trigger: make(chan struct{}, addressQueueDepth),
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
		cancel:  cancel,
	}
	if bus != nil {
		// The payload is not read: it names a kernel device, while the configured
		// interface is a logical name the resolver maps (resolveInterfaceAddr). Any
		// address event is a reason to ask the resolver again.
		trigger := func(any) { w.kick() }
		w.unsub = append(w.unsub,
			bus.Subscribe(ifaceevents.Namespace, ifaceevents.EventAddrAdded, trigger),
			bus.Subscribe(ifaceevents.Namespace, ifaceevents.EventAddrRemoved, trigger),
			// A bound XFRM interface created again comes back under a new index, and
			// one with no address raises no address event (planXFRMInterfaceEntries).
			bus.Subscribe(ifaceevents.Namespace, ifaceevents.EventCreated, trigger),
		)
	}
	go w.run(ctx, s)
	return w
}

// kick asks the worker to re-read the interface and re-resolve the `respond` names. It
// never blocks: a trigger already pending covers this one.
func (w *addressWorker) kick() {
	select {
	case w.trigger <- struct{}{}:
	default:
	}
}

// run is the worker loop. It ends only when stop closes. A nil changes channel is a watch
// that is not running: the select never picks it, and the retry timer starts it again.
func (w *addressWorker) run(ctx context.Context, s *ikeEngineState) {
	defer close(w.done)
	ticker := time.NewTicker(respondNameRefresh)
	defer ticker.Stop()

	retry := time.NewTimer(addressWatchRetry)
	retry.Stop()
	defer retry.Stop()

	changes := w.watchAddresses(ctx, s, retry)
	for {
		select {
		case <-w.stop:
			return
		case _, open := <-changes:
			if open {
				w.kick()
				continue
			}
			// The watcher closed the channel while the engine runs: its kernel
			// subscription failed, so the table is no longer watched.
			s.log.Warn("ike: the kernel interface address table watch ended, ze starts it again",
				"interface", interfaceName(s), "retry", addressWatchRetry,
				"fix", "none if the next line reports the watch running; if this repeats, look for the netlink error ze logged before it, and for kernel messages with: dmesg | grep -i netlink")
			changes = nil
			retry.Reset(addressWatchRetry)
		case <-retry.C:
			changes = w.watchAddresses(ctx, s, retry)
		case <-w.trigger:
			s.followAddress()
			resolveRespondNames(respondNames.queries(), w.stop, s.log)
		case <-ticker.C:
			resolveRespondNames(respondNames.queries(), w.stop, s.log)
		}
	}
}

// watchAddresses opens the kernel address table watch and answers its channel, then kicks
// the worker, because the table may have moved while no watch ran. When the watch cannot
// open it answers nil: it arms retry for a failure that can pass, and leaves it unarmed in
// a build with no watcher, which stays so for the life of the process. Called only from
// run.
func (w *addressWorker) watchAddresses(ctx context.Context, s *ikeEngineState, retry *time.Timer) <-chan struct{} {
	changes, err := watchAddressTableFn(ctx)
	if errors.Is(err, network.ErrAddressWatchNotRegistered) {
		s.log.Warn("ike: the kernel interface address table is not watched, so a `vpn ipsec interface` address that appears or changes is followed only through the interface monitor",
			"interface", interfaceName(s), "error", err,
			"fix", "run ze on Linux, where the netlink interface backend watches the table; elsewhere configure the WAN under `interface` so its monitor reports the address")
		return nil
	}
	// refuse-by-default: guards run -- a nil channel is the watch that is not running, which run's select never picks until the retry armed here starts it again
	if err != nil {
		s.log.Warn("ike: cannot watch the kernel interface address table, ze tries again",
			"interface", interfaceName(s), "retry", addressWatchRetry, "error", err,
			"fix", "none if the watch starts on the next try; if this repeats, look for kernel messages with: dmesg | grep -i netlink")
		retry.Reset(addressWatchRetry)
		return nil
	}
	w.kick()
	return changes
}

// interfaceName is the `vpn ipsec interface` the engine last applied, for a log line, or
// empty before the first apply and when none is configured.
func interfaceName(s *ikeEngineState) string {
	s.applyMu.Lock()
	defer s.applyMu.Unlock()
	if s.applied == nil {
		return ""
	}
	return s.applied.Interface
}

// Stop unsubscribes, ends the address table watch and waits for the worker to return.
// MUST be called before the engine stops its peers (ikeEngineState.shutdown), so no event
// restarts one behind the stop.
func (w *addressWorker) Stop() {
	for _, unsub := range w.unsub {
		unsub()
	}
	w.cancel()
	close(w.stop)
	<-w.done
}

// followAddress re-reads the configured interface and, when the address moved or a
// literal pair is still missing its socket, re-plans the running configuration onto it:
// the primary pair moves, only the sessions that held it stop, and the reconcile starts
// the peers the new address serves. It then restarts any route-chosen session whose
// source left the host, and rebuilds the SPD entries of the bound XFRM interfaces (each
// interface SPD's catch-all and the if_id 0 exemptions) from what they hold now. It does nothing before the first apply.
//
// The event is a trigger and the resolver is the truth, so a burst of events, or an event
// for another interface, costs one read and changes nothing (R-1: act only on a change).
// An interface that no longer exists holds no address (readInterfaceAddr), so its peers
// stop and wait as they do when the address leaves.
func (s *ikeEngineState) followAddress() {
	s.applyMu.Lock()
	defer s.applyMu.Unlock()
	if s.applied == nil || s.plan == nil {
		return
	}
	if s.applied.Interface != "" {
		ifIP, err := s.readInterfaceAddr()
		switch {
		case err != nil:
			// A read that did not answer is not "no address": keep what runs. Taking
			// every tunnel down for a transient lookup fault is what the reload refusal
			// prevents. A device the backend reports absent is not this case: it
			// arrives here as the empty address (readInterfaceAddr).
			s.log.Warn("ike: cannot read interface addresses, keeping the current address",
				"interface", s.applied.Interface, "address", s.ifAddr, "error", err)
		case ifIP != s.ifAddr:
			s.log.Info("ike: interface address changed",
				"interface", s.applied.Interface, "from", s.ifAddr, "to", ifIP)
			s.ifAddr = ifIP
			s.adoptPlan(planPeers(s.applied, ifIP))
		case s.literalPairIncomplete():
			s.adoptPlan(planPeers(s.applied, ifIP))
		}
	}
	// A bound XFRM interface that gained or lost an address, or came back under a new
	// index or if_id, needs its SPD entries and its tunnel-source guard rebuilt
	// (unmatched_xfrm_interface.go, source_guard.go).
	entries := s.planXFRMInterfaceEntries(s.applied)
	s.installXFRMInterfaceEntries(dataplane.Get(), entries)
	s.pruneXFRMInterfaceEntries(dataplane.Get(), entries)
	s.restartVanishedRouteSources()
}

// readInterfaceAddr reads the address of the configured interface for followAddress.
//
// An interface the backend reports absent (iface.ErrInterfaceNotFound: it read the link
// table and found no such device, as when a PPPoE session ends and `ppp0` is deleted)
// holds no address, so it answers the empty address with no error, and the peers that
// took the address stop and wait (AC-4). Every other error is a read that did not
// answer, and is returned for the caller to keep what runs.
//
// The absence is logged once when it begins, not on every address event while it lasts:
// the worker re-reads the interface after any change of the host's address table. A read
// that finds the interface again ends the absence, so the next one is logged again. The
// caller holds applyMu.
func (s *ikeEngineState) readInterfaceAddr() (string, error) {
	ifIP, err := resolveInterfaceAddrFn(s.applied.Interface)
	if err == nil {
		s.ifMissing = false
		return ifIP, nil
	}
	if !interfaceAbsent(err) {
		return "", err
	}
	if !s.ifMissing {
		s.log.Warn("ike: the interface does not exist, so the peers without local-address stop and wait for its address",
			"interface", s.applied.Interface, "address", s.ifAddr, "error", err,
			"fix", "create the interface, or bring up the WAN session (PPPoE, DHCP) that creates it; `ze doctor` lists the waiting peers")
	}
	s.ifMissing = true
	return "", nil
}

// interfaceAbsent reports whether err is the backend's answer that the device does not
// exist, as opposed to a read that did not answer (iface.ErrInterfaceNotFound).
func interfaceAbsent(err error) bool {
	return errors.Is(err, iface.ErrInterfaceNotFound)
}

// literalPairIncomplete reports whether a literal pair failed to bind its IKE socket,
// which is what a literal the host does not hold yet looks like.
func (s *ikeEngineState) literalPairIncomplete() bool {
	for _, pair := range s.literalPairs {
		if pair.ike == nil {
			return true
		}
	}
	return false
}

// adoptPlan moves the sockets to plan and starts or stops the sessions it changes. It is
// the half of applyConfig after the SPD, and the address worker's whole action. Peer
// starts stay deferred to OnAllPluginsReady while the startup configuration is pending.
// The caller holds applyMu.
func (s *ikeEngineState) adoptPlan(plan *peerPlan) {
	s.rebindListeners(plan)
	s.plan = plan
	publishWaiting(s.applied, plan)
	s.reCtx.Store(&reEstablishCtx{cfg: plan.cfg, sockets: s.socketSnapshot(plan)})
	if s.startupCfg != nil {
		s.startupCfg = plan
		return
	}
	s.reconcile(plan)
}

// hostHoldsAddrFn reports whether an interface of this host carries ip. Swapped in unit
// tests, which cannot remove an address from the host.
var hostHoldsAddrFn = func(ip net.IP) (bool, error) {
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return false, err
	}
	for _, address := range addresses {
		if network, ok := address.(*net.IPNet); ok && network.IP.Equal(ip) {
			return true, nil
		}
	}
	return false, nil
}

// restartVanishedRouteSources restarts each route-chosen session (no `interface`, no
// local-address) whose IKE SA runs from an address the host no longer holds, so the next
// cycle asks the routing table again and comes up from the new source. The caller holds
// applyMu.
//
// A session whose SA negotiated MOBIKE is left alone. RFC 4555 Section 3.5 names "changes
// in IP addresses" as a trigger for the MOBIKE path, which moves the established SA with
// UPDATE_SA_ADDRESSES (serviceMobike, mobike.go), so a restart here would only break it.
func (s *ikeEngineState) restartVanishedRouteSources() {
	if s.startupCfg != nil {
		return
	}
	peersMu.RLock()
	running := maps.Clone(s.peers)
	peersMu.RUnlock()

	dp := dataplane.Get()
	bus := getEventBus()
	stopped := false
	for name, ps := range running {
		if ps.peerCfg.LocalAddress != "" {
			continue
		}
		sa := ps.getSA()
		if sa == nil || sa.cycleLocal == nil {
			continue
		}
		// RFC 4555 Section 3.5
		if atomic.LoadInt32(&sa.mobikeUp) == 1 {
			continue
		}
		held, err := hostHoldsAddrFn(sa.cycleLocal)
		if err != nil {
			s.log.Warn("ike: cannot list host addresses, route-chosen peers are not checked", "error", err)
			return
		}
		if held {
			continue
		}
		s.log.Info("ike: route-chosen local address left the host, restarting the peer from the new source",
			"peer", name, "address", sa.cycleLocal)
		stopPeerSession(name, ps, s.peers, s.table, dp, bus, s.log)
		stopped = true
	}
	if stopped {
		s.reconcile(s.plan)
	}
}
