// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- config reconciliation
// RFC: rfc/short/rfc7296.md -- COOKIE threshold (Section 2.6), NAT traversal (Section 2.23)
// RFC: rfc/short/rfc3948.md -- UDP encapsulation of ESP on port 4500 (Section 2.1)
// Related: register.go -- runEngine, which owns the one ikeEngineState and feeds it both deliveries
// Related: reconcile.go -- reconcilePeers and stopPeerSession, the peer half of an apply
package engine

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
)

// ikeListeners is one pair of UDP sockets the engine receives IKE on, with the host it
// was bound for. The engine holds one pair per local origin (peerPlan, dynamic_wan.go):
// the primary pair, and under `interface` one pair per distinct literal local-address.
//
// The host is recorded because a bound socket cannot be re-addressed in place. An
// operator who edits `vpn ipsec interface`, or an interface address that moves, asks for
// a different socket, and the only way to give one is to close this pair and open
// another. Comparing the host is how a rebind tells that move from every other change.
//
// A nil member beside a non-empty host is a bind that FAILED and was reported. The host
// still records what the pair was asked for, so a later rebind at the same address
// retries the failed half instead of reading the working half as the whole answer.
//
// nattShared marks a NAT-T member that is the engine's one wildcard port-4500 socket
// (ikeEngineState.sharedNATT), which every pair shares when live XFRM migration is
// available. closeSockets leaves that socket open: no interface event moves it.
type ikeListeners struct {
	host       string
	ike        *transport.UDPTransport
	natt       *transport.UDPTransport
	nattShared bool
}

// boundElsewhere reports whether a socket is open at an address the configuration no
// longer asks for. Nothing open means nothing to rebind: openPair then binds at the
// new host, which is what the daemon's first apply does.
func (l *ikeListeners) boundElsewhere(host string) bool {
	if l.ike == nil && l.natt == nil {
		return false
	}
	return l.host != host
}

// natTraversalMobile reports whether the NAT-T socket listens on the wildcard: MOBIKE
// changes the destination of an established IKE SA, packet-info records the concrete
// destination and SendFrom preserves the chosen reply source. Under the test port every
// socket follows its host, so two local daemons can share one port knob.
func natTraversalMobile() bool {
	_, mobile := dataplane.Get().(dataplane.TunnelMigrator)
	return mobile && ikeTestPortFn() == ""
}

// ikeEngineState is the running state one IKE engine owns across the configurations it
// is given: the two sockets, the peer sessions, the SA table, the operator's SPD
// entries and the remote-access address pool.
//
// Two goroutines write it: the SDK dispatch goroutine (applyConfig, from OnConfigure and
// OnConfigApply, and OnAllPluginsReady) and the address worker (followAddress,
// dynamic_wan.go). Each holds applyMu for its whole step, so the members need no lock of
// their own. The peers map is the exception, and it is guarded by peersMu because the
// shared dispatch goroutine reads it too.
//
// peers is the SAME map object as activePeersMap, which operator `clear` relies on:
// TerminateAllSAs deletes from that map and calls reEstablishFn, and the reconcile then
// sees the peer absent and starts a fresh session (TestTerminateAllSAsReinitiates).
type ikeEngineState struct {
	// issueRaiser reaches the daemon report bus in either plugin process mode.
	issueRaiser espFormIssueRaiser
	table       *SATable
	peers       map[string]*PeerSession
	metrics     *IPsecMetrics
	log         *slog.Logger

	// applyMu serializes every writer of this state: an apply, the deferred startup
	// reconcile, and the address worker's re-plan.
	applyMu sync.Mutex

	// listeners is the primary pair: the interface's address under `interface`, else
	// the one pair every peer holds (peerPlan).
	listeners ikeListeners

	// literalPairs are the pairs of peers whose own local-address differs from the
	// interface's current address, keyed by that address. An interface address event
	// never touches them (AC-5).
	literalPairs map[string]*ikeListeners

	// sharedNATT is the one wildcard port-4500 socket every pair holds when live XFRM
	// migration is available (openPair). Nil otherwise, and before the first bind.
	sharedNATT *transport.UDPTransport

	// applied is the configuration as the operator wrote it, before the interface
	// address was filled in. The address worker re-plans from it (planPeers).
	applied *ipsec.IPsecConfig

	// ifAddr is the interface address the current plan was derived for, empty when the
	// interface has none or does not exist.
	ifAddr string

	// ifMissing records that the last read found the configured interface absent
	// (iface.ErrInterfaceNotFound), so readInterfaceAddr logs an absence once, when it
	// begins. Written under applyMu.
	ifMissing bool

	// plan is the current plan, the one the running sessions follow.
	plan *peerPlan

	// worker follows interface addresses and refreshes `respond` names. Nil in a unit
	// test that starts none; kickAddressWorker then does nothing.
	worker *addressWorker

	// installedSPD is the operator's SPD entries as currently installed. It is tracked
	// rather than re-derived from the live configuration because the removal half needs
	// the PREVIOUS selectors: the kernel identifies a policy by its selector alone, so
	// an entry whose prefix was edited can only be removed under the prefix it was
	// installed with (installSPDPolicies, spd_policy.go).
	installedSPD map[string]ipsec.SPDPolicy

	// xfrmEntries is the SPD entries of the bound XFRM interfaces as currently
	// installed, keyed by selector (unmatched_xfrm_interface.go): each interface SPD's
	// own catch-all and the if_id 0 exemptions above the node catch-all. Nil until the
	// first install fills it. Tracked for the reason installedSPD is: an address the
	// interface lost is removed under the selector it was installed with.
	xfrmEntries map[spdSelector]*xfrmInterfaceEntry

	// spdApplied records whether this engine reached the SPD writers of its first
	// apply: the IKE bypass and the catch-all (applyConfig). Ze's SPD appears there and
	// is released as one unit on exit (releaseSPD), so an engine that never applied a
	// configuration holds no SPD entry, and an early SDK startup failure removes no
	// policy it never managed.
	spdApplied bool

	// ikeBypassInstalled records that an apply installed the IKE bypass, once, after
	// the catch-all it follows.
	ikeBypassInstalled bool

	// discardInForce is true once an apply installed a DISCARD catch-all, and false
	// after one installed a BYPASS. The commit check reads it on the SDK's verify path
	// (refuseImplicitDiscard, unmatched.go), so it is atomic.
	discardInForce atomic.Bool

	// startupCfg carries the daemon's first plan across to OnAllPluginsReady, which is
	// where its peers start. It is nil once those peers run, and it stays nil for every
	// reload. An address that moves before then re-plans it in place (adoptPlan).
	//
	// The sessions are what the running peers were started from (reconcilePeers); plan
	// above is the derivation they were started by, which the next re-plan replaces.
	startupCfg *peerPlan

	// reCtx is read by the operator `clear` path on another goroutine, so it is atomic
	// where the members above are not.
	reCtx atomic.Pointer[reEstablishCtx]

	// espFormTUN is the outcome of the last apply's attempt to open the TUN clone device
	// the dual-form receiver needs, nil when that apply made none (recordESPFormTUN). The
	// ipsec-esp-form-fast-path doctor check reads it from the doctor callback's goroutine,
	// so it is atomic.
	espFormTUN atomic.Pointer[espFormTUNRecord]
}

// openPair binds whichever socket of l is absent and starts its receive loop. The IKE
// listener follows host. The NAT-T listener follows host too, unless it is mobile, when
// the pair takes the engine's one wildcard socket. Closing existing listeners is a
// separate ordered decision (rebindListeners).
//
// A failed bind is reported and leaves that member nil. The engine keeps running,
// because the other socket can still carry a tunnel.
func (s *ikeEngineState) openPair(l *ikeListeners, host string) {
	l.host = host

	if l.ike == nil {
		tr, err := transport.NewUDPTransport(ikeAddr(host), s.log)
		if err != nil {
			s.log.Warn("ike: failed to start UDP transport", "address", host, "error", err)
		} else {
			l.ike = tr
			go tr.Run()
			go dispatchInbound(tr, s.table, s.log)
		}
	}

	if l.natt != nil {
		return
	}
	if natTraversalMobile() {
		if s.sharedNATT == nil {
			s.sharedNATT = openNATT("0.0.0.0", s.table, s.log)
		}
		l.natt = s.sharedNATT
		l.nattShared = l.natt != nil
		return
	}
	l.natt = openNATT(host, s.table, s.log)
	l.nattShared = false
}

// openNATT binds the port-4500 socket at host, enables UDP encapsulation on it and starts
// its receive loop, or reports the failure and answers nil.
func openNATT(host string, table *SATable, log *slog.Logger) *transport.UDPTransport {
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
		log.Warn("ike: failed to start NAT-T transport", "address", host, "error", err)
		return nil
	}

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
	return natt
}

// closeSockets shuts both sockets and empties the pair, which ends the two receive
// loops openPair started. A shared NAT-T member is dropped from the pair and left open:
// ikeEngineState.closeAllListeners closes it at exit.
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
	if l.natt != nil && !l.nattShared {
		if err := l.natt.Close(); err != nil {
			log.Warn("ike: NAT-T transport close error", "error", err)
		}
	}
	l.natt = nil
	l.nattShared = false
}

// reEstablishCtx is what operator `clear` needs to start the peers again: the
// configuration the engine is running and the socket pair each peer runs on. It is
// republished by every apply and every address move, so a `clear` after either
// re-initiates against the sockets now bound rather than the ones the daemon started
// with. sockets is a frozen snapshot (socketSnapshot), because `clear` runs on another
// goroutine than the one that moves the pairs.
type reEstablishCtx struct {
	cfg     *ipsec.IPsecConfig
	sockets peerSockets
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
	s.applyMu.Lock()
	defer s.applyMu.Unlock()

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
		// The lookup failed (the interface is absent, or the read did not answer), or
		// the interface has no IPv4 address. Every peer that named no local-address of
		// its own is now unbindable.
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
		// that comes up after the daemon does is ordinary at boot: the peers that need
		// its address wait (planPeers) and the address worker starts them when it
		// appears (followAddress, dynamic_wan.go).
		switch unbindable := unbindablePeers(cfg, ifErr); {
		case unbindable != nil && phase == applyReload:
			return unbindable
		case unbindable != nil:
			s.log.Warn("ike: peers without local-address wait for the interface address", "error", unbindable)
		case ifErr != nil:
			s.log.Warn("ike: cannot read interface addresses", "interface", cfg.Interface, "error", ifErr)
		default:
			s.log.Warn("ike: no IPv4 address on interface", "interface", cfg.Interface)
		}
	default:
		// The interface has an address. planPeers below fills it into every peer that
		// names no local-address of its own.
	}

	// RFC 4301 Section 4.4.1 gives the SPD three dispositions, and the operator writes
	// the two that no negotiation produces. They are reconciled BEFORE the peers: an
	// entry that discards traffic must be in force before the tunnels that traffic
	// could otherwise take are built. They also outrank a peer's entries by default, so
	// installing them second would leave a window in which the lower-ranked entry is
	// the only match (installSPDPolicies, spd_policy.go).
	//
	// The catch-all goes in first, ahead of the operator's entries and the cookie
	// threshold, and it is the one step of an apply that
	// can refuse the configuration after the interface check above. A catch-all that
	// could not be installed, DISCARD or BYPASS, leaves an inbound packet meeting no
	// SPD-I entry at all, so the apply fails and the configuration is not reported
	// applied (installUnmatched, unmatched.go). Running it before the operator's entries
	// and the cookie threshold means a refusal leaves those unchanged. The entries a
	// partial install did place carry the configured disposition: under DISCARD they
	// drop what they match, under BYPASS they pass it in the clear, which is what the
	// packet would meet with no entry at all, so neither opens anything the refused apply
	// did not already leave open. spdApplied is set first, so releaseSPD removes them on
	// exit. The catch-all is
	// re-asserted on every apply, because the backend upserts a template-free policy, so
	// a changed disposition replaces the entry in place.
	//
	// Ze's SPD comes into being here, on the first apply, and is released as one unit
	// on exit (releaseSPD). The catch-all is the FIRST entry in and the LAST out, so at
	// no instant does Ze hold an SPD entry without the final entry beneath it, and
	// outside [first apply, exit] Ze holds no SPD entry and no SA (unless a removal at
	// exit failed, when releaseSPD keeps the catch-all beneath what is left and says so):
	// it is not acting as
	// an IPsec implementation then, and no packet is looked up in an SPD-I of Ze's
	// (owner decision 2026-10-20, Q6 B; docs/guide/ipsec.md, "Traffic no entry
	// matches"). The IKE bypass follows it on the first apply. No IKE socket is open
	// before this apply (listeners.open runs below), so a DISCARD catch-all installed
	// ahead of the bypass drops no IKE exchange. The bypass carries no configuration, so
	// it is installed once (installIKEBypass, bypass.go).
	//
	// Each XFRM interface a peer binds is an SPD of its own, at its if_id, and gets its
	// own final entry (RFC 4301 Section 4.4.1, Section 5.2). A route-based peer's traffic
	// is looked up at if_id 0 before it reaches its XFRM interface, so under a DISCARD
	// catch-all each bound interface also needs the exemption entries above it: the SPD
	// selection function (RFC 4301 Section 4.4.1, unmatched_xfrm_interface.go). On a
	// reload they go in BEFORE the catch-all is re-asserted, the interface catch-all
	// ahead of the exemptions, so a catch-all turning to DISCARD never drops a bound
	// tunnel's traffic, and the stale ones come out AFTER it, so a catch-all turning to
	// BYPASS never does either. On the first apply the catch-all still goes in first.
	//
	// An interface catch-all the backend refuses fails the apply, as the node-wide one
	// does: the interface's SPD would have no final entry (installXFRMInterfaceEntries).
	// On a reload the refusal comes before the node catch-all is re-asserted, so the
	// running disposition stays. On the first apply the node catch-all, the IKE bypass
	// and the prune have run, so the SPD state matches discardInForce, and releaseSPD
	// removes them on exit.
	//
	// The application override guard (RFC 4301 Section 4.4.1) follows the catch-all on
	// the first apply, before the IKE bypass and before any IKE socket opens, so no
	// application can exempt a socket from an SPD Ze has started to enforce. A guard
	// that cannot attach under deny fails the apply and installs nothing further. On a
	// reload it moves FIRST, ahead of every SPD change, so a refused guard leaves the
	// running SPD and its disposition as they were.
	interfaceEntries := s.planXFRMInterfaceEntries(cfg)
	if s.spdApplied {
		if err := applyApplicationOverride(dataplane.Get(), cfg.ApplicationOverride); err != nil {
			return err
		}
		if err := s.installXFRMInterfaceEntries(dataplane.Get(), interfaceEntries); err != nil {
			return err
		}
	}
	firstApply := !s.spdApplied
	s.spdApplied = true
	if err := installUnmatched(dataplane.Get(), cfg.Unmatched, s.log); err != nil {
		return err
	}
	if firstApply {
		if err := applyApplicationOverride(dataplane.Get(), cfg.ApplicationOverride); err != nil {
			// The catch-all is in force, so the refusal of an implicit discard on the
			// next commit judges the SPD as it is.
			s.discardInForce.Store(cfg.Unmatched == dataplane.SPActionDiscard)
			return err
		}
	}
	if !s.ikeBypassInstalled {
		installIKEBypass(dataplane.Get(), s.log)
		s.ikeBypassInstalled = true
	}
	interfaceErr := s.installXFRMInterfaceEntries(dataplane.Get(), interfaceEntries)
	s.pruneXFRMInterfaceEntries(dataplane.Get(), interfaceEntries)
	s.discardInForce.Store(cfg.Unmatched == dataplane.SPActionDiscard)
	if interfaceErr != nil {
		return interfaceErr
	}
	if phase == applyStartup {
		// The start is never verified, so refuseImplicitDiscard and verifyICMPOverlap
		// cannot run here.
		warnImplicitDiscardAtStart(cfg, s.log)
		warnICMPOverlapAtStart(cfg, s.log)
	}
	installSPDPolicies(dataplane.Get(), s.installedSPD, cfg.Policies, s.log)
	s.installedSPD = cfg.Policies

	// RFC 7296 Section 2.6. Published before any peer is reconciled, so an initiation
	// that arrives during the reconcile is judged against the configuration being
	// applied rather than the one being replaced. It follows the catch-all so that a
	// refused catch-all leaves the running threshold unchanged.
	setCookieThreshold(cfg.CookieThreshold)

	// The plan for the address the interface has now: which peers run, the local
	// address each sends from, the socket pair each holds, and which wait. The
	// operator's configuration is kept as written, so the address worker can re-plan it
	// when the address moves (followAddress, dynamic_wan.go).
	plan := planPeers(cfg, ifIP)
	s.applied = cfg
	s.ifAddr = ifIP
	// The interface check above already logged an absent interface.
	s.ifMissing = interfaceAbsent(ifErr)
	s.rebindListeners(plan)
	s.plan = plan
	publishWaiting(cfg, plan)

	s.table.virtualIP.reload(cfg.RemoteAccess, s.log)

	// Before the peers start, so the device's refusal is logged ahead of the first Child
	// SA it refuses. It refuses nothing itself: the refusal is per SA, at install.
	s.recordESPFormTUN(cfg)

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
	s.reCtx.Store(&reEstablishCtx{cfg: plan.cfg, sockets: s.socketSnapshot(plan)})
	if phase == applyStartup {
		s.startupCfg = plan
	} else {
		s.reconcile(plan)
	}

	// The `respond` peers named by DNS: a new or renamed one matches nothing until the
	// worker answers, and the worker re-reads the interface too, which covers an
	// address event that arrived while this apply ran (R-2).
	respondNames.sync(cfg)
	s.kickAddressWorker()

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

// recordESPFormTUN opens and closes the TUN clone device the dual-form receiver needs,
// and predicts whether the ze-espform interface it would create keeps the inbound policy
// check and a loose reverse-path filter (dataplane.ESPFormTUNAvailable), on every apply
// that configures a peer, and records the outcome for the doctor
// (ikeEngineState.espFormTUN). Every IPsec peer can need the device: any IKE SA may float
// to UDP port 4500 (RFC 7296 Section 2.23), and a Child SA on that port is installed
// with UDP encapsulation and must receive bare ESP too (newChildSA, create_child.go),
// which the receiver re-presents through ze-espform. An apply that configures no peer,
// or whose dataplane has no dual-form receiver (VPP, a failed load), makes no attempt
// and clears the record, so the doctor reports nothing about a device nothing needs.
//
// The attempt runs here, in the engine's process, and never in the doctor check, because
// it may create the node /dev/net/tun (espFormTUNAvailable), and a doctor check only reads.
//
// A failure is logged once per apply, at Warn, with its cause and fix. It does not
// refuse the configuration: a peer that stays on UDP port 500 with no NAT still
// installs a Child SA with no UDP encapsulation, which needs no device, and the refusal
// of one that floats is made per SA, at install. The caller holds applyMu.
func (s *ikeEngineState) recordESPFormTUN(cfg *ipsec.IPsecConfig) {
	if len(cfg.Peers) == 0 {
		s.espFormTUN.Store(nil)
		return
	}
	if _, ok := dataplane.Get().(dataplane.ESPFormFastPathReporter); !ok {
		s.espFormTUN.Store(nil)
		return
	}
	record := newESPFormTUNRecord(espFormTUNAvailable())
	s.espFormTUN.Store(record)
	if record.status != espFormTUNRefused {
		return
	}
	const refusedText = "ike: Ze cannot create a usable ze-espform interface, so every Child SA on UDP port 4500 will be refused at install"
	if refusal, ok := errors.AsType[*opererr.Error](record.err); ok {
		s.log.LogAttrs(context.Background(), slog.LevelWarn, refusedText,
			refusal.LogAttrs(
				slog.String("problem", refusal.Problem()),
				slog.String("code", diagnostic.CodeDoctorIPsecESPFormTUN))...)
		return
	}
	s.log.Warn(refusedText, "error", record.err, "fix", "ze show doctor",
		"code", diagnostic.CodeDoctorIPsecESPFormTUN)
}

// reconcile starts, stops and restarts peer sessions so the running set matches plan,
// each new session on the socket pair of its origin.
func (s *ikeEngineState) reconcile(plan *peerPlan) {
	reconcilePeersWith(plan.cfg, s.peers, s.table, s.socketsFor(plan), getEventBus(), s.log)
}

// kickAddressWorker asks the worker, when one runs, to re-read the interface and the
// `respond` names.
func (s *ikeEngineState) kickAddressWorker() {
	if s.worker != nil {
		s.worker.kick()
	}
}

// pairOf answers the pair host is bound in, or nil.
func (s *ikeEngineState) pairOf(host string) *ikeListeners {
	if host == s.listeners.host {
		return &s.listeners
	}
	return s.literalPairs[host]
}

// socketsFor answers each peer's sockets from the pairs as they are bound now. The
// caller holds applyMu for as long as it uses the answer.
func (s *ikeEngineState) socketsFor(plan *peerPlan) peerSockets {
	return func(name string) (*transport.UDPTransport, *transport.UDPTransport, string) {
		host := plan.hostOf[name]
		if pair := s.pairOf(host); pair != nil {
			return pair.ike, pair.natt, host
		}
		return nil, nil, host
	}
}

// socketSnapshot is socketsFor frozen now, for a reader on another goroutine (operator
// `clear`), which must not read the pairs while the address worker moves them.
func (s *ikeEngineState) socketSnapshot(plan *peerPlan) peerSockets {
	type socketPair struct{ ike, natt *transport.UDPTransport }
	frozen := make(map[string]socketPair, len(plan.hostOf))
	for _, host := range plan.hostOf {
		if pair := s.pairOf(host); pair != nil {
			frozen[host] = socketPair{pair.ike, pair.natt}
		}
	}
	hostOf := maps.Clone(plan.hostOf)
	return func(name string) (*transport.UDPTransport, *transport.UDPTransport, string) {
		host := hostOf[name]
		pair := frozen[host]
		return pair.ike, pair.natt, host
	}
}

// rebindListeners moves the socket pairs to what plan asks for: the primary pair to its
// host, one pair for each literal host, and no pair nobody holds.
//
// The ORDER is the whole of this function, and it is why the rebind is not a
// create-if-nil line. PeerSession.ike and PeerSession.natt are immutable after
// startPeerSession, so a session that survives its pair's rebind holds a closed file
// descriptor and every message it sends is refused. The sessions that hold a moving
// pair therefore stop, then its sockets close, then the new pair opens, and the
// reconcile that follows starts those peers against it.
//
// Only the sessions that held the moved pair stop (stopSessionsOn). A peer on another
// pair, a literal local-address the interface event did not move, keeps its session and
// its socket (AC-5). A session that started while its pair had no IKE socket (the bind
// failed: the address was not on the host yet) is stopped when the socket is finally
// bound, so the reconcile restarts it on a socket that can send.
//
// Every pair that leaves closes before any pair opens. One address and port is bound
// once, and an address moves between the two kinds: the interface takes an address a
// peer names as its literal local-address, and that peer then shares the primary pair
// (planPeers). Opened first, the primary pair would bind an address the departing
// literal pair still holds, fail, and leave every peer on it without a socket.
func (s *ikeEngineState) rebindListeners(plan *peerPlan) {
	if s.listeners.boundElsewhere(plan.primaryHost) {
		s.log.Info("ike: listen address changed, rebinding",
			"from", s.listeners.host, "to", plan.primaryHost, "peers", len(s.peers))
		s.stopSessionsOn(s.listeners.host)
		s.listeners.closeSockets(s.log)
	}
	for host, pair := range s.literalPairs {
		if plan.literalHosts[host] {
			continue
		}
		s.stopSessionsOn(host)
		pair.closeSockets(s.log)
		delete(s.literalPairs, host)
	}

	if plan.primaryWanted() {
		s.openPairFor(&s.listeners, plan.primaryHost)
	} else if plan.primaryHost == "" {
		s.listeners.host = ""
	}
	for host := range plan.literalHosts {
		if s.literalPairs == nil {
			s.literalPairs = map[string]*ikeListeners{}
		}
		pair, ok := s.literalPairs[host]
		if !ok {
			pair = &ikeListeners{}
			s.literalPairs[host] = pair
		}
		s.openPairFor(pair, host)
	}
}

// openPairFor opens the absent sockets of pair at host, and stops the sessions that were
// started while its IKE socket was absent, so they restart on the socket now bound.
func (s *ikeEngineState) openPairFor(pair *ikeListeners, host string) {
	hadIKE := pair.ike != nil
	s.openPair(pair, host)
	if !hadIKE && pair.ike != nil {
		s.stopSessionsOn(host)
	}
}

// stopSessionsOn stops every session that holds the pair bound for host, removing each
// Child SA before it returns (stopPeerSession).
func (s *ikeEngineState) stopSessionsOn(host string) {
	peersMu.RLock()
	stopping := make(map[string]*PeerSession, len(s.peers))
	for name, ps := range s.peers {
		if ps.bindHost == host {
			stopping[name] = ps
		}
	}
	peersMu.RUnlock()

	dp := dataplane.Get()
	bus := getEventBus()
	for name, ps := range stopping {
		stopPeerSession(name, ps, s.peers, s.table, dp, bus, s.log)
	}
}

// closeAllListeners closes every pair and the shared NAT-T socket. The caller MUST have
// stopped every peer first (closeSockets).
func (s *ikeEngineState) closeAllListeners(log *slog.Logger) {
	s.listeners.closeSockets(log)
	for host, pair := range s.literalPairs {
		pair.closeSockets(log)
		delete(s.literalPairs, host)
	}
	if s.sharedNATT != nil {
		if err := s.sharedNATT.Close(); err != nil {
			log.Warn("ike: NAT-T transport close error", "error", err)
		}
		s.sharedNATT = nil
	}
}

// stopAllPeers tears every running session down, removing each Child SA before it
// returns (stopPeerSession). It is the first step of every exit of runEngine, before
// the sockets close and releaseSPD runs, and a second call finds no peer.
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

// shutdown ends everything the engine runs, on every exit of runEngine, in an order
// each step depends on:
//   - operator `clear` loses its way to start peers (reEstablishFn), and the address
//     worker stops, so no session can be started behind the stop below;
//   - every peer stops, so every Child SA and its policies are gone;
//   - the IKE sockets close, so nothing left can negotiate a Child SA and install a
//     policy behind the release (closeSockets requires the peers stopped first);
//   - the SPD the engine installed is released, catch-all last (releaseSPD).
//
// An error return of the SDK used to skip the middle steps, so a session started by a
// `clear` could still install a policy between releaseSPD and the backend closing.
// Every path that installs an SPD entry needs a peer session and a socket, and both
// are gone before the release starts.
//
// The caller MUST close the backend after it returns: releaseSPD needs the dataplane.
func (s *ikeEngineState) shutdown(log *slog.Logger) {
	reEstablishFn.Store(nil)
	if s.worker != nil {
		// Before the peers stop, so no address event restarts one behind the stop.
		s.worker.Stop()
		s.worker = nil
	}
	s.stopAllPeers()
	s.closeAllListeners(log)
	s.releaseSPD(dataplane.Get(), log)
}

// releaseSPD removes the SPD this engine installed, on every exit of runEngine, as
// the one unit applyConfig installed it as: the IKE bypass, then the operator's
// entries, then the bound XFRM interfaces' exemptions and each interface SPD's own
// catch-all (after its Child SA policies, which stopAllPeers removed), and the node
// catch-all LAST. Each set is node-wide and outlives the process
// that installed it, so an entry left behind keeps acting for a daemon that is no
// longer running: a DISCARD keeps dropping traffic, a bypass keeps exempting it.
//
// The catch-all goes last so that no packet meets a database holding the catch-all's
// disposition and nothing above it: while any other Ze entry is still installed, so
// is the final entry that decides what that entry does not name. Once it is gone Ze
// holds no SPD entry and no SA (stopAllPeers ran first), which is the state before
// the first apply (owner decision 2026-10-20, Q6 B).
//
// A removal that fails leaves its entry installed, and that entry would then outlive
// the catch-all. So the catch-all is removed only when the SPD ledger (spd_left.go) is
// empty: every Child SA policy, bypass, operator entry and XFRM interface entry Ze installed is gone. Otherwise
// it stays, and warnSPDLeft names the entries left and the command that removes them.
//
// An engine that never applied a configuration installed nothing, so it removes
// nothing, and a catch-all another policy manager put in place survives its failed
// start.
func (s *ikeEngineState) releaseSPD(dp dataplane.Dataplane, log *slog.Logger) {
	if !s.spdApplied {
		return
	}
	// The application override guard goes after the catch-all, so no instant holds a
	// Ze SPD entry an application could override. When the catch-all stays because an
	// entry could not be removed, the guard is detached here all the same: the engine
	// is leaving, and the link would close with the process anyway.
	defer releaseApplicationOverride(dp, log)
	removeIKEBypass(dp, log)
	removeSPDPolicies(dp, s.installedSPD, log)
	s.pruneXFRMInterfaceEntries(dp, xfrmInterfacePlan{})
	if warnSPDLeft(dp, log) {
		return
	}
	removeUnmatched(dp, log, s.issueRaiser)
}
