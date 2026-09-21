// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- config reconciliation
// RFC: rfc/short/rfc7296.md -- COOKIE threshold (Section 2.6), NAT traversal (Section 2.23)
// RFC: rfc/short/rfc3948.md -- UDP encapsulation of ESP on port 4500 (Section 2.1)
// Related: register.go -- runEngine, which owns the one ikeEngineState and feeds it both deliveries
// Related: reconcile.go -- reconcilePeers and stopPeerSession, the peer half of an apply
package engine

import (
	"log/slog"
	"maps"
	"reflect"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/core/eap"
)

// ikeListeners is the pair of UDP sockets the engine receives IKE on, with the host
// they are bound to.
//
// The host is recorded because a bound socket cannot be re-addressed in place. An
// operator who edits `vpn ipsec interface`, or the local-address of the peer the bind
// followed, is asking for a different socket, and the only way to give them one is to
// close this pair and open another. Comparing the host is how an apply tells that edit
// from every other one.
//
// A nil member beside a non-empty host is a bind that FAILED and was reported. The host
// still records what the pair was asked for, so a later apply at the same address
// retries the failed half instead of reading the working half as the whole answer.
type ikeListeners struct {
	host string
	ike  *transport.UDPTransport
	natt *transport.UDPTransport
}

// boundElsewhere reports whether a socket is open at an address the configuration no
// longer asks for. Nothing open means nothing to rebind: open below then binds at the
// new host, which is what the daemon's first apply does.
func (l *ikeListeners) boundElsewhere(host string) bool {
	if l.ike == nil && l.natt == nil {
		return false
	}
	return l.host != host
}

// open binds whichever of the two sockets is absent at host and starts its receive
// loops. It is the create half alone: a socket that is already open is serving peers,
// so close is a separate decision with an order of its own.
//
// A failed bind is reported and leaves that member nil. The engine keeps running,
// because the other socket can still carry a tunnel.
func (l *ikeListeners) open(host string, table *SATable, log *slog.Logger) {
	l.host = host

	if l.ike == nil {
		tr, err := transport.NewUDPTransport(ikeAddr(host), log)
		if err != nil {
			log.Warn("ike: failed to start UDP transport", "error", err)
		} else {
			l.ike = tr
			go tr.Run()
			go dispatchInbound(tr, table, log)
		}
	}

	if l.natt != nil {
		return
	}

	// RFC 3948 Section 2.1: the NAT-T listener on port 4500 carries UDP-encapsulated
	// IKE and ESP.
	natt, err := transport.NewNATTTransport(nattAddr(host), log)
	if err != nil {
		// Recorded, not only logged. Without the socket ze receives no
		// UDP-encapsulated ESP at all, which is a stronger failure than the UDP_ENCAP
		// one below, and the doctor check read an unset state as "no NAT-T listener
		// was ever asked for" and said nothing. A guard that goes quiet on the worse
		// failure fails open (ai/rules/evidence.md).
		setUDPEncapFailure(err)
		countUDPEncapFailure()
		log.Warn("ike: failed to start NAT-T transport", "error", err)
		return
	}
	l.natt = natt

	// RFC 7296 Section 2.23 MUST: "all devices MUST be able to receive and process
	// both UDP-encapsulated ESP and non-UDP-encapsulated ESP packets at any time."
	//
	// Ze holds port 4500. The kernel decapsulates ESP that arrives there only while
	// this option is set. Without it, every encapsulated ESP datagram reaches
	// dispatchNATTInbound, which reads an ESP SPI in place of the non-ESP marker and
	// drops it. The installed XFRM state then matches nothing.
	//
	// It runs BEFORE natt.Run, so no datagram is read on an unprepared socket.
	//
	// The failure is reported rather than swallowed. It separates a working tunnel
	// from one that carries no traffic. Doctor check ipsec-udp-encap reads the same
	// state, so an operator sees it first (ai/rules/repo-maintenance.md).
	if err := transport.EnableESPInUDP(natt.Conn()); err != nil {
		setUDPEncapFailure(err)
		log.Warn("ike: udp encapsulation not enabled on port 4500, encapsulated ESP will be dropped",
			"port", transport.NATTPort, "syscall", "setsockopt UDP_ENCAP", "error", err)
		countUDPEncapFailure()
	} else {
		setUDPEncapFailure(nil)
	}
	go natt.Run()
	go dispatchNATTInbound(natt, table, log)
}

// closeSockets shuts both sockets and empties the pair, which ends the two receive
// loops open started.
//
// The caller MUST have stopped every peer session that holds them first. PeerSession.ike
// and PeerSession.natt are immutable after startPeerSession, so a session that outlives
// this call keeps a closed file descriptor and every message it sends is refused.
func (l *ikeListeners) closeSockets(log *slog.Logger) {
	if l.ike != nil {
		if err := l.ike.Close(); err != nil {
			log.Warn("ike: transport close error", "error", err)
		}
		l.ike = nil
	}
	if l.natt != nil {
		if err := l.natt.Close(); err != nil {
			log.Warn("ike: NAT-T transport close error", "error", err)
		}
		l.natt = nil
	}
}

// reEstablishCtx is what operator `clear` needs to start the peers again: the
// configuration the engine is running and the two sockets it is running them on. It is
// republished by every apply, so a `clear` after a reload re-initiates against the
// sockets that reload left bound rather than the ones the daemon started with.
type reEstablishCtx struct {
	cfg  *ipsec.IPsecConfig
	tr   *transport.UDPTransport
	natt *transport.UDPTransport
}

// ikeEngineState is the running state one IKE engine owns across the configurations it
// is given: the two sockets, the peer sessions, the SA table, the operator's SPD
// entries and the remote-access address pool.
//
// applyConfig is the only writer, and both deliveries reach it. OnConfigure carries the
// configuration the daemon starts with and OnConfigApply carries every reload after
// that. The SDK dispatch goroutine serves one callback at a time, so no member needs a
// lock. The peers map is the exception, and it is guarded by peersMu because the shared
// dispatch goroutine reads it too.
//
// peers is the SAME map object as activePeersMap, which operator `clear` relies on:
// TerminateAllSAs deletes from that map and calls reEstablishFn, and the reconcile then
// sees the peer absent and starts a fresh session (TestTerminateAllSAsReinitiates).
type ikeEngineState struct {
	table   *SATable
	peers   map[string]*PeerSession
	metrics *IPsecMetrics
	log     *slog.Logger

	listeners ikeListeners

	// pool is the remote-access address pool, and poolCfg is the configuration it was
	// built from. The pair is what lets applyConfig tell an edited pool from an
	// unedited one, for the reason ikeListeners records its host.
	pool    *eap.Pool
	poolCfg ipsec.VirtualIPPool

	// installedSPD is the operator's SPD entries as currently installed. It is tracked
	// rather than re-derived from the live configuration because the removal half needs
	// the PREVIOUS selectors: the kernel identifies a policy by its selector alone, so
	// an entry whose prefix was edited can only be removed under the prefix it was
	// installed with (installSPDPolicies, spd_policy.go).
	installedSPD map[string]ipsec.SPDPolicy

	// startupCfg carries the daemon's first configuration across to OnAllPluginsReady,
	// which is where its peers start. It is nil once those peers run, and it stays nil
	// for every reload.
	//
	// There is no member holding the configuration the running peers were started from.
	// The sessions ARE that configuration (reconcilePeers), so a copy beside them could
	// only disagree with them.
	startupCfg *ipsec.IPsecConfig

	// reCtx is read by the operator `clear` path on another goroutine, so it is atomic
	// where the members above are not.
	reCtx atomic.Pointer[reEstablishCtx]
}

// applyConfig is the ONE place a parsed configuration becomes running state, and both
// delivery paths reach it: OnConfigure carries the configuration the daemon starts with,
// and OnConfigApply carries every reload after that.
//
// It returns an error for ONE condition, and only on the reload phase: a configuration
// whose peers depend on `interface` for their local address, when that interface cannot
// supply one. See applyPhase and unbindablePeers for why the two deliveries answer that
// condition differently.
//
// The order of the steps below is load-bearing, and each one states why it sits where it
// does.
func (s *ikeEngineState) applyConfig(cfg *ipsec.IPsecConfig, phase applyPhase) error {
	// The interface lookup and the refusal it can raise run FIRST, before anything
	// about the running engine changes. A refused reload rolls the transaction back,
	// and a mutation made ahead of the refusal survives that rollback: the operator is
	// told the commit failed and the box keeps a cookie threshold and a set of SPD
	// entries from the configuration that was rejected. Both used to be applied above
	// this check.
	ifIP := ""
	var ifErr error
	if cfg.Interface != "" {
		ifIP, ifErr = resolveInterfaceAddrFn(cfg.Interface)
	}

	switch {
	case cfg.Interface == "":
		// No interface is configured, so no peer takes its local address from one.
	case ifErr != nil, ifIP == "":
		// The lookup failed, or the interface has no IPv4 address. Every peer that
		// named no local-address of its own is now unbindable.
		//
		// A RELOAD refuses that, because of what it would otherwise do. The peers keep
		// the empty LocalAddress the parser gave them, peerConfigChanged compares that
		// against the address the running sessions resolved successfully at startup,
		// and every one of them is stopped and restarted into a state that cannot
		// bind. A transient interface read would take down every working tunnel.
		//
		// STARTUP applies the configuration anyway and says so. There is no previous
		// configuration to fall back to and no running tunnel to protect, so a refusal
		// would start no peer, no IKE socket and no NAT-T socket at all, including for
		// the peers that carry their own local-address and are unaffected. An interface
		// that comes up after the daemon does is ordinary at boot.
		switch unbindable := unbindablePeers(cfg, ifErr); {
		case unbindable != nil && phase == applyReload:
			return unbindable
		case unbindable != nil:
			s.log.Warn("ike: peers without local-address will fail", "error", unbindable)
		case ifErr != nil:
			s.log.Warn("ike: cannot read interface addresses", "interface", cfg.Interface, "error", ifErr)
		default:
			s.log.Warn("ike: no IPv4 address on interface", "interface", cfg.Interface)
		}
	default:
		for name := range cfg.Peers {
			peer := cfg.Peers[name]
			if peer.LocalAddress == "" {
				peer.LocalAddress = ifIP
				cfg.Peers[name] = peer
				s.log.Debug("ike: resolved local-address from interface",
					"peer", name, "interface", cfg.Interface, "address", ifIP)
			}
		}
	}

	// RFC 7296 Section 2.6. Published before any peer is reconciled, so an initiation
	// that arrives during the reconcile is judged against the configuration being
	// applied rather than the one being replaced.
	setCookieThreshold(cfg.CookieThreshold)

	// RFC 4301 Section 4.4.1 gives the SPD three dispositions, and the operator writes
	// the two that no negotiation produces. They are reconciled BEFORE the peers for
	// the reason the cookie threshold is published first: an entry that discards
	// traffic must be in force before the tunnels that traffic could otherwise take are
	// built. They also outrank a peer's entries by default, so installing them second
	// would leave a window in which the lower-ranked entry is the only match
	// (installSPDPolicies, spd_policy.go).
	installSPDPolicies(dataplane.Get(), s.installedSPD, cfg.Policies, s.log)
	s.installedSPD = cfg.Policies
	// The catch-all is re-asserted on every apply, because the backend upserts a
	// template-free policy, so a changed disposition replaces the entry in place
	// (installUnmatched, unmatched.go).
	installUnmatched(dataplane.Get(), cfg.Unmatched, s.log)

	// The listen host of BOTH sockets. It is computed once, and from the ONE interface
	// lookup above, because the engine listens at one address and its two sockets must
	// agree on which. They did not: the NAT-T socket took the wildcard whenever no
	// interface was configured, so it claimed port 4500 for the whole host while the
	// IKE socket was bound to one address.
	peerLocal := ""
	for name := range cfg.Peers {
		if la := cfg.Peers[name].LocalAddress; la != "" {
			peerLocal = la
			break
		}
	}
	s.rebindListeners(ikeListenHost(ifIP, peerLocal), len(cfg.Peers) > 0)

	s.reloadPool(cfg.RemoteAccess)

	// Peer reconciliation is the only part of an apply that puts packets on the wire,
	// and at STARTUP it is deferred to OnAllPluginsReady. Phase 1 loads a config-path
	// plugin such as this one before Phase 2 spawns an `external` plugin, so initiating
	// here means the first sa-up is emitted while no external process has subscribed
	// yet: the event is routed correctly and delivered to nobody
	// (plugin/server/subscribe.go getMatching returns an empty set). BGP already
	// answers this by starting its reactor at configure and its peers from
	// coord.OnPostStartup (bgp/plugin/register.go).
	//
	// A reload has no such window, so it reconciles here as it always did.
	s.reCtx.Store(&reEstablishCtx{cfg: cfg, tr: s.listeners.ike, natt: s.listeners.natt})
	if phase == applyStartup {
		s.startupCfg = cfg
	} else {
		s.reconcile(cfg)
	}

	if s.metrics != nil {
		s.metrics.Update()
	}

	// RFC 7296 Section 2.16 discourages an EAP method that establishes no shared key,
	// and eap-md5 is one. The line goes out HERE, once for each configuration this
	// daemon adopts, rather than once for each handshake: it is a fact about what the
	// operator wrote, so it is worth as many lines as there are configurations and no
	// more.
	warnKeylessEAPModes(cfg, s.log)

	s.log.Info("ike engine configured", "peers", len(cfg.Peers))
	return nil
}

// reconcile starts, stops and restarts peer sessions so the running set matches cfg.
func (s *ikeEngineState) reconcile(cfg *ipsec.IPsecConfig) {
	reconcilePeers(cfg, s.peers, s.table, s.listeners.ike, s.listeners.natt, getEventBus(), s.log)
}

// rebindListeners moves both sockets to host when the operator's edit moved the address
// the engine listens at, and opens whichever socket is absent when wanted says the
// configuration has a peer to serve.
//
// The ORDER is the whole of this function, and it is why the rebind is not a
// create-if-nil line. PeerSession.ike and PeerSession.natt are immutable after
// startPeerSession, so a session that survives a rebind holds a closed file descriptor
// and every message it sends is refused. Every session therefore stops, then the sockets
// close, then the new pair opens, and the reconcile that follows starts every peer
// against it.
//
// Stopping the peers is not a cost the rebind pays reluctantly. A moved listen address
// is a new local address to offer, and a restart is the only thing that carries a
// configuration edit to the wire (peerConfigChanged, reconcile.go). A peer whose own
// local-address changed is restarted by that guard anyway, and this is the same answer
// for the peers that merely share the socket.
func (s *ikeEngineState) rebindListeners(host string, wanted bool) {
	if s.listeners.boundElsewhere(host) {
		s.log.Info("ike: listen address changed, rebinding",
			"from", s.listeners.host, "to", host, "peers", len(s.peers))
		s.stopAllPeers()
		s.listeners.closeSockets(s.log)
	}
	if wanted {
		s.listeners.open(host, s.table, s.log)
	}
}

// stopAllPeers tears every running session down. It is the first step of a rebind: the
// sockets a session holds cannot be swapped under it, so the session cannot outlive
// them.
func (s *ikeEngineState) stopAllPeers() {
	peersMu.RLock()
	stopping := make(map[string]*PeerSession, len(s.peers))
	maps.Copy(stopping, s.peers)
	peersMu.RUnlock()

	dp := dataplane.Get()
	bus := getEventBus()
	for name, ps := range stopping {
		stopPeerSession(name, ps, s.peers, s.table, dp, bus, s.log)
	}
}

// reloadPool rebuilds the remote-access address pool when the operator edited it, and
// releases it when remote access is removed.
//
// The comparison is over the WHOLE ipsec.VirtualIPPool and not over its Range alone. The
// DNS servers and the search domain are pushed to a client as well, so a pool rebuilt
// only on a range edit would keep offering the resolver the operator replaced. It is
// written here rather than as ipsec.VirtualIPPool.Equal because this is the type's only
// comparer, so there is no second declaration to disagree with.
//
// NOTHING READS s.pool YET, and the rebuild does not change that. eap.Pool.Allocate has
// no non-test caller, so ze negotiates no INTERNAL_IP4_ADDRESS (RFC 7296 Section 2.19)
// and a remote-access client receives no address from this pool however often it is
// rebuilt. What the rebuild does change is operator-visible: an edited range is
// validated and reported on the commit that makes it, where before the daemon held the
// range it started with and said nothing until it was restarted. The missing consumer is
// recorded in plan/journal/stale-artifact-reused.md.
func (s *ikeEngineState) reloadPool(ra *ipsec.RemoteAccessConfig) {
	if ra == nil {
		s.pool = nil
		s.poolCfg = ipsec.VirtualIPPool{}
		return
	}
	if s.pool != nil && reflect.DeepEqual(s.poolCfg, ra.Pool) {
		return
	}

	pool, err := eap.NewPool(ra.Pool.Range, ra.Pool.Range6, ra.Pool.DNS, ra.Pool.Domain)
	if err != nil {
		// The pool is dropped rather than kept, so a later apply that repeats the same
		// broken range reports it again instead of going quiet over a stale pool.
		s.pool = nil
		s.poolCfg = ipsec.VirtualIPPool{}
		s.log.Warn("ike: failed to create virtual IP pool", "range", ra.Pool.Range, "error", err)
		return
	}
	s.pool = pool
	s.poolCfg = ra.Pool
	s.log.Info("ike: virtual IP pool created", "range", ra.Pool.Range)
}
