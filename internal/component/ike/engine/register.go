// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- IKE engine component registration
// Related: cookie.go -- the COOKIE challenge that gates the half-open slot in tryResponderSAInit
// Related: notify_error.go -- the unprotected sender answering a datagram that matched no SA
package engine

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/ikeprobe"
	"github.com/ze-software/ze/internal/core/ipsecinventory"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

// peersWithoutLocalAddress counts the peers that named no local-address of their own, so
// the engine has to take one from the configured `interface`. It bounds nothing: the
// population is the peer map, which the config parser already built.
func peersWithoutLocalAddress(cfg *ipsec.IPsecConfig) int {
	count := 0
	for name := range cfg.Peers {
		if cfg.Peers[name].LocalAddress == "" {
			count++
		}
	}
	return count
}

// peersNeedInterfaceAddress reports whether ANY peer depends on the configured interface
// for its local address. It is the condition that turns a failed interface read from a
// warning into a refusal: a configuration whose every peer carries its own local-address
// is unaffected by the interface, so refusing it would be refusing a working config.
func peersNeedInterfaceAddress(cfg *ipsec.IPsecConfig) bool {
	return peersWithoutLocalAddress(cfg) > 0
}

// applyPhase names which delivery is applying a configuration. The two differ in ONE
// decision and in nothing else: what a peer set the configured interface cannot bind
// does. A reload REFUSES it, so the transaction rolls back and the running tunnels are
// untouched. Startup WARNS and applies the configuration anyway, because there is no
// previous configuration to fall back to and no running tunnel to protect: refusing there
// would leave the box with no IPsec at all, and no transport, over an interface an
// operator can bring up after the daemon starts.
type applyPhase int

const (
	applyStartup applyPhase = iota
	applyReload
)

// unbindablePeers reports the configuration a reload MUST refuse: the interface some
// peers take their local address from supplied none, so those peers are unbindable. A nil
// return means the configuration can be applied.
//
// It is given the interface lookup's RESULT and does not perform its own. A second lookup
// can answer differently from the caller's, and the peers would then be applied with the
// empty LocalAddress this refusal exists to catch.
//
// A configuration whose every peer carries its own local-address is unaffected by the
// interface, so it draws nil: refusing it would refuse a config that works.
//
// ifErr separates the two failures. A lookup that never ran is reported as itself,
// because naming "no IPv4 address" there sends the operator to the interface
// configuration for a fault that is not in it.
func unbindablePeers(cfg *ipsec.IPsecConfig, ifErr error) error {
	if !peersNeedInterfaceAddress(cfg) {
		return nil
	}
	if ifErr != nil {
		return fmt.Errorf("ike config: cannot read addresses of interface %q (%w), and %d peer(s) have no local-address of their own",
			cfg.Interface, ifErr, peersWithoutLocalAddress(cfg))
	}
	return fmt.Errorf("ike config: interface %q has no IPv4 address, and %d peer(s) have no local-address of their own",
		cfg.Interface, peersWithoutLocalAddress(cfg))
}

// ikeConfigStaging carries a reload's configuration from the VERIFY phase that parsed it
// to the APPLY phase that commits it, and refuses an apply that has nothing staged.
//
// The plugin protocol splits a reload in two, and the apply request carries diff sections
// rather than the configuration, so an engine that does not stash what verify parsed has
// nothing to apply. `stage` and `commit` both run on the SDK's dispatch goroutine, which
// serves one callback at a time, so the field needs no lock.
//
// commit CONSUMES the staged value, which is what makes a second apply refuse rather than
// re-apply a configuration the coordinator already committed. A verify whose transaction
// is then abandoned leaves a value here, and the next verify overwrites it: verify always
// precedes the apply of its own transaction.
//
// apply MUST be non-nil. It is the engine's whole apply path, and a zero-valued staging
// would answer every reload with a nil-pointer panic rather than a refusal.
type ikeConfigStaging struct {
	pending *ipsec.IPsecConfig
	apply   func(*ipsec.IPsecConfig) error
}

// stage records the configuration the verify phase accepted. The caller MUST have
// validated it first: stage performs no checks of its own.
func (s *ikeConfigStaging) stage(cfg *ipsec.IPsecConfig) { s.pending = cfg }

// commit applies the staged configuration and clears it. It MUST be called after stage,
// within the same transaction, and it returns errIKEApplyWithoutVerify when it is not.
func (s *ikeConfigStaging) commit() error {
	cfg := s.pending
	s.pending = nil
	if cfg == nil {
		// Fail closed. runVerify and runApply select their participants with the SAME
		// predicate (filterDiffs, config/transaction/orchestrator.go), so an apply that
		// reaches this engine without a verify having staged its config is a protocol
		// violation rather than a normal state. Reporting success here would tell the
		// operator the commit landed while the engine kept the previous configuration,
		// which is this spec's own defect in miniature: a successful commit, a
		// `sighup reload complete`, and no change on the wire. The ddos-local plugin
		// refuses the same case for the same reason (errApplyWithoutVerify,
		// internal/plugins/ddos/local/register.go).
		return errIKEApplyWithoutVerify
	}
	return s.apply(cfg)
}

// errIKEApplyWithoutVerify rejects a config-apply that arrives with no configuration
// staged by config-verify. The reload transaction drives both phases over one participant
// set, so this state cannot be reached by a conforming coordinator, and reporting success
// for it would leave the engine running the previous configuration while the operator is
// told the commit landed.
var errIKEApplyWithoutVerify = errors.New("ike config apply: no verified config staged (config-apply arrived without config-verify); refusing to report success over the previous config")

var (
	loggerPtr      atomic.Pointer[slog.Logger]
	eventBusPtr    atomic.Pointer[ze.EventBus]
	activeTablePtr atomic.Pointer[SATable]

	peersMu        sync.RWMutex
	activePeersMap map[string]*PeerSession

	reEstablishFn atomic.Pointer[func()]
)

func ActiveTable() *SATable                           { return activeTablePtr.Load() }
func SetActiveTableForTest(t *SATable)                { activeTablePtr.Store(t) }
func SetActivePeersForTest(m map[string]*PeerSession) { setActivePeers(m) }

func ActivePeers() map[string]*PeerSession {
	peersMu.RLock()
	defer peersMu.RUnlock()
	if activePeersMap == nil {
		return nil
	}
	out := make(map[string]*PeerSession, len(activePeersMap))
	maps.Copy(out, activePeersMap)
	return out
}

// PeerInfoMap returns a snapshot of peer info for all active sessions.
func PeerInfoMap() map[string]PeerInfo {
	peersMu.RLock()
	defer peersMu.RUnlock()
	if activePeersMap == nil {
		return nil
	}
	out := make(map[string]PeerInfo, len(activePeersMap))
	for name, ps := range activePeersMap {
		out[name] = ps.Info()
	}
	return out
}

func setActivePeers(m map[string]*PeerSession) {
	peersMu.Lock()
	activePeersMap = m
	dataplaneChanged()
	peersMu.Unlock()
}

func TerminateAllSAs() int {
	peersMu.Lock()
	if activePeersMap == nil {
		peersMu.Unlock()
		return 0
	}
	snapshot := make(map[string]*PeerSession, len(activePeersMap))
	maps.Copy(snapshot, activePeersMap)
	peersMu.Unlock()

	table := ActiveTable()
	bus := getEventBus()
	log := getLogger()
	count := 0
	for name, ps := range snapshot {
		// Delete from the active map BEFORE stopping (like TerminatePeerSA), so the
		// shared dispatch goroutine cannot accept a fresh IKE_SA_INIT for this peer that
		// would escape the cleanup below and leak.
		peersMu.Lock()
		delete(activePeersMap, name)
		dataplaneChanged()
		peersMu.Unlock()
		// StopGraceful: the owner loop sends an authenticated INFORMATIONAL Delete on
		// its way out (RFC 7296 Section 1.4) so the peer tears down at once instead of
		// waiting for the DPD timeout -- the operator-visible half of the fix.
		ps.StopGraceful()
		// getSA (mutex-guarded): a responder's ps.sa is written by the dispatch
		// goroutine, not joined by Stop() (Finding 3).
		if sa := ps.getSA(); sa != nil && table != nil {
			table.Remove(sa.InitiatorSPI, sa.ResponderSPI)
			emitSADown(bus, sa, log)
		}
		ps.cleanupPendingSA(table, dataplane.Get(), bus, log)
		count++
	}

	if fn := reEstablishFn.Load(); fn != nil {
		(*fn)()
	}
	return count
}

func TerminatePeerSA(name string) bool {
	peersMu.Lock()
	if activePeersMap == nil {
		peersMu.Unlock()
		return false
	}
	ps, ok := activePeersMap[name]
	if !ok {
		peersMu.Unlock()
		return false
	}
	delete(activePeersMap, name)
	dataplaneChanged()
	peersMu.Unlock()

	ps.StopGraceful()
	table := ActiveTable()
	bus := getEventBus()
	log := getLogger()
	// getSA (mutex-guarded): a responder's ps.sa is written by the dispatch goroutine,
	// not joined by Stop() (Finding 3).
	if sa := ps.getSA(); sa != nil && table != nil {
		table.Remove(sa.InitiatorSPI, sa.ResponderSPI)
		emitSADown(bus, sa, log)
	}
	ps.cleanupPendingSA(table, dataplane.Get(), bus, log)

	if fn := reEstablishFn.Load(); fn != nil {
		(*fn)()
	}
	return true
}

func setLogger(l *slog.Logger)   { loggerPtr.Store(l) }
func getLogger() *slog.Logger    { return loggerPtr.Load() }
func setEventBus(eb ze.EventBus) { eventBusPtr.Store(&eb) }

func getEventBus() ze.EventBus {
	p := eventBusPtr.Load()
	if p == nil {
		return nil
	}
	return *p
}

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
	RegisterHealthCheck()
	registerIPsecRedistSources()
	ipsecinventory.Register(inventorySnapshot)
	ikeprobe.Register(probePeer)

	reg := registry.Registration{
		Name:        "ike",
		Description: "IKEv2 engine for native IPsec VPN",
		ConfigRoots: []string{configRootVPN, configRootPKI},
		RunEngine:   runEngine,
		// The SAME chain the runtime OnConfigVerify below runs, reached OFFLINE by
		// ze config validate and by the web and hub commit paths
		// (config.VerifyPluginConfig, internal/component/config/plugin_verify.go).
		//
		// Without this field the registry skips the plugin outright, so every IPsec
		// semantic refusal -- an undefined ike-group, an unresolvable certificate, an
		// EAP-TLS peer with no trust anchor, a traffic selector no backend can program --
		// was invisible to ze config validate and only surfaced at commit. An operator's
		// pre-commit check passed a config the daemon then refused.
		//
		// validateIPsecSections satisfies the InProcessConfigVerifier contract: it is
		// side-effect free, and it resolves certificate names against the CANDIDATE pki
		// section rather than the live store (config.go).
		InProcessConfigVerifier: validateIPsecSections,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			setEventBus(eb)
		},
		DoctorChecks: []registry.DoctorCheckDef{{
			Name:         "ipsec-interface",
			Phase:        rpc.DoctorPhasePostConfig,
			Order:        730,
			Dependencies: []string{doctorDepConfigLoaded, "interface"},
			Platforms:    []string{doctorPlatformAny},
			Codes:        []string{diagnostic.CodeDoctorIPsecIface},
			Check:        checkIPsecInterface,
		}, {
			// RFC 7296 Section 2.23 makes receiving UDP-encapsulated ESP a MUST. The
			// kernel does it only while the NAT-T socket carries UDP_ENCAP, and a
			// failure there is invisible: the tunnel establishes and carries nothing.
			Name:         "ipsec-udp-encap",
			Phase:        rpc.DoctorPhasePostConfig,
			Order:        731,
			Dependencies: []string{doctorDepConfigLoaded},
			Platforms:    []string{doctorPlatformAny},
			Codes:        []string{diagnostic.CodeDoctorIPsecUDPEncap},
			Check:        checkIPsecUDPEncap,
		}, {
			// RFC 7296 Section 3.6's hash-and-url is an outbound network dependency:
			// ze publishes its certificate at an operator-named http URL and the peer
			// fetches it. A URL the peer cannot reach fails on the PEER's side, so
			// nothing in ze's own logs explains it (ai/rules/repo-maintenance.md).
			Name:         "ipsec-cert-url",
			Phase:        rpc.DoctorPhasePostConfig,
			Order:        732,
			Dependencies: []string{doctorDepConfigLoaded},
			Platforms:    []string{doctorPlatformAny},
			Codes:        []string{diagnostic.CodeDoctorIPsecCertURL, diagnostic.CodeDoctorIPsecCertURLDenied},
			Check:        checkIPsecCertURL,
		}, {
			// RFC 7296 Section 2.6's COOKIE challenge is gated on a count whose
			// ceiling is the number of responding peers. A threshold above that
			// ceiling is never met, so the defense is off and nothing says so.
			Name:         "ipsec-cookie-threshold",
			Phase:        rpc.DoctorPhasePostConfig,
			Order:        733,
			Dependencies: []string{doctorDepConfigLoaded},
			Platforms:    []string{doctorPlatformAny},
			Codes:        []string{diagnostic.CodeDoctorIPsecCookieThreshold},
			Check:        checkIPsecCookieThreshold,
		}},
	}
	reg.CLIHandler = func(_ []string) int { return 1 }
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "ike: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func runEngine(conn net.Conn) int {
	log := getLogger()
	log.Debug("ike engine starting")

	if err := dataplane.Load(ikeDataplaneName()); err != nil {
		log.Warn("ike: dataplane load failed, SA installation disabled", "error", err)
	}
	// Exempt ze's own IKE traffic from IPsec BEFORE any peer can negotiate a Child
	// SA. A negotiated selector that covers the peer's IKE address would otherwise
	// capture the exchange that maintains the very tunnel it belongs to
	// (installIKEBypass, bypass.go).
	installIKEBypass(dataplane.Get(), log)
	// Released on EVERY exit of runEngine, which is why this is a defer. The four
	// policies are node-wide rather than per-peer, so they outlive the process that
	// installed them: an error return that skipped the removal left ze's IKE traffic
	// exempt from IPsec for a daemon that is no longer running, and nothing on the box
	// ever cleaned it up. The engine owns what it installed on every way out, and
	// removeIKEBypass is idempotent, so a removal that finds nothing is expected.
	//
	// The backend closes here too, and AFTER the removal: CloseBackend clears the
	// active backend, so a removal ordered after it would have no dataplane to talk to
	// and would silently release nothing.
	// The running state this engine owns across every configuration it is given
	// (ikeEngineState, apply.go). It is built before the deferred release below,
	// because that release reads the SPD entries the apply path installed.
	table := NewSATable()
	activeTablePtr.Store(table)
	state := &ikeEngineState{
		table:        table,
		peers:        make(map[string]*PeerSession),
		log:          log,
		installedSPD: map[string]ipsec.SPDPolicy{},
	}
	setActivePeers(state.peers)

	defer func() {
		removeIKEBypass(dataplane.Get(), log)
		// The operator's entries are node-wide in exactly the way the bypass is, so
		// they are released on the same every-exit path and for the same reason: a
		// DISCARD that outlives the process keeps dropping traffic for a daemon that
		// is no longer running (removeSPDPolicies, spd_policy.go).
		removeSPDPolicies(dataplane.Get(), state.installedSPD, log)
		if state.unmatchedApplied {
			removeUnmatched(dataplane.Get(), log)
		}
		if err := dataplane.CloseBackend(); err != nil {
			log.Warn("ike: dataplane close error", "error", err)
		}
	}()

	p := sdk.NewWithConn("ike", conn)
	defer closeSDK(p)

	if reg := registry.GetMetricsRegistry(); reg != nil {
		state.metrics = RegisterMetrics(reg)
	}

	// Operator `clear` re-initiates against the configuration and the sockets the LAST
	// apply published, which is why it reads reCtx rather than closing over a socket
	// variable. It passes no previous configuration: every session it bounced is
	// already out of the peers map, so the reconcile sees each name as absent and
	// starts it fresh (TestTerminateAllSAsReinitiates).
	reEstablish := func() {
		rc := state.reCtx.Load()
		if rc == nil || rc.cfg == nil {
			return
		}
		reconcilePeers(rc.cfg, state.peers, state.table, rc.tr, rc.natt, getEventBus(), log)
	}
	reEstablishFn.Store(&reEstablish)

	metricsStop := make(chan struct{})
	if state.metrics != nil {
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-metricsStop:
					return
				case <-ticker.C:
					state.metrics.Update()
				}
			}
		}()
	}

	staging := &ikeConfigStaging{apply: func(cfg *ipsec.IPsecConfig) error {
		return state.applyConfig(cfg, applyReload)
	}}

	// Reject a structurally valid but self-inconsistent config before it is
	// applied: a peer naming an undefined ike-group or esp-group, a certificate
	// reference the PKI store cannot resolve, an EAP-TLS peer with no trust
	// anchor, or a malformed remote-access pool.
	//
	// The staging parse is parseVPNSections and MUST stay so. parseIPsecSections,
	// which OnConfigure below uses, calls pki.Load on any `pki` section it is handed,
	// and that swaps the PROCESS-WIDE store: a verify that adopted a candidate's
	// certificates would leave a REJECTED config's PKI installed in a running daemon.
	// pki.ParseJSON's own comment states that contract, and validateIPsecSections
	// already honors it by resolving names against a throwaway candidate set.
	//
	// One precondition is NOT enforced in code and belongs to whoever changes the
	// registration below. Both parsers answer a delivery carrying NO `vpn` section
	// with an EMPTY config and a nil error, which staged and committed would stop
	// every peer on the box. That cannot happen while the runtime root set is
	// WantsConfig ["vpn"] alone, because the plugin is then a participant only when
	// `vpn` has diffs, and a removed root still arrives as `vpn` carrying "{}".
	// ConfigRoots ["vpn","pki"] in the registry above is WIDER on purpose: it feeds
	// the offline verifier, which never stages anything. Making the two agree would
	// arm both hazards at once.
	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		if err := validateIPsecSections(sections); err != nil {
			return fmt.Errorf("ike config: %w", err)
		}
		cfg, err := parseVPNSections(sections)
		if err != nil {
			return fmt.Errorf("ike config: %w", err)
		}
		staging.stage(cfg)
		return nil
	})

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, err := parseIPsecSections(sections)
		if err != nil {
			return fmt.Errorf("ike config: %w", err)
		}
		// applyStartup: the interface branch inside applyConfig (apply.go) states why the
		// two deliveries answer an unbindable peer set differently. It logs that
		// condition and applies the configuration, so this phase returns an error only
		// for a failure a running daemon could not have.
		return state.applyConfig(cfg, applyStartup)
	})

	// The reload half. Without this handler the SDK answers config-apply OK and calls
	// nothing (sdk_callbacks.go, OnConfigApply), so every reload verified the operator's
	// edit, reported success, and left the engine running the configuration it started
	// with. reconcilePeers was reachable from startup and from operator `clear` alone.
	//
	// The diff sections are not read. The engine reconciles the whole peer set against
	// what is running (reconcilePeers), so the WHOLE configuration is what it needs, and
	// a diff would be a second description of a change the reconciler already computes.
	//
	// No ApplyBudget is declared beside WantsConfig below, so the orchestrator's
	// 30-second default bounds this (computeTieredDeadline, config/transaction). The
	// work is one Stop plus one start for each peer whose configuration changed, and a
	// Stop returns as soon as the session goroutine reaches its stopCh.
	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		return staging.commit()
	})

	// The startup half of the reconcile, held back until every plugin in every
	// phase has completed its handshake and both registries are frozen. That is
	// the earliest moment an external plugin can hold a subscription, so it is
	// the earliest moment the first sa-up has a reader.
	//
	// It runs on the same event loop as every other callback, so the ikeEngineState
	// members it reads need no lock, for the reason applyConfig (apply.go)
	// states. Nothing here waits on peer activity, which is the one thing
	// sendPostStartupToAll cannot be asked to do (plugin/server/poststartup.go).
	p.OnAllPluginsReady(func() error {
		if state.startupCfg == nil {
			return nil
		}
		cfg := state.startupCfg
		state.startupCfg = nil
		state.reconcile(cfg)
		log.Info("ike peers started", "peers", len(cfg.Peers))
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()

	if err := p.Run(ctx, sdk.Registration{
		WantsConfig: []string{configRootVPN},
	}); err != nil {
		log.Error("ike engine failed", "error", err)
		return 1
	}

	// Cleanup after Run returns (shutdown).
	reEstablishFn.Store(nil)
	close(metricsStop)
	peersMu.Lock()
	shutdownPeers := make(map[string]*PeerSession, len(state.peers))
	maps.Copy(shutdownPeers, state.peers)
	peersMu.Unlock()
	shutdownBus := getEventBus()
	for name, ps := range shutdownPeers {
		ps.Stop()
		ps.cleanupPendingSA(state.table, dataplane.Get(), shutdownBus, log)
		peersMu.Lock()
		delete(state.peers, name)
		dataplaneChanged()
		peersMu.Unlock()
	}
	state.listeners.closeSockets(log)
	// The IKE bypass and the backend are released by the deferred cleanup registered
	// beside installIKEBypass, so this clean shutdown and every error exit release the
	// same set. It runs after this line, which is after every peer has stopped, so
	// nothing is left that still needs the exemption (installIKEBypass, bypass.go,
	// for why the bypass outlives every peer rather than being removed per Child SA).

	return 0
}

func closeSDK(p *sdk.Plugin) {
	if err := p.Close(); err != nil {
		getLogger().Debug("ike: sdk close", "error", err)
	}
}

// inboundRateLimiter is a simple token-bucket rate limiter for inbound IKE packets.
type inboundRateLimiter struct {
	mu       sync.Mutex
	tokens   int
	max      int
	lastFill time.Time
	rate     int // tokens per second
}

func newInboundRateLimiter(perSecond, burst int) *inboundRateLimiter {
	return &inboundRateLimiter{
		tokens:   burst,
		max:      burst,
		rate:     perSecond,
		lastFill: time.Now(),
	}
}

func (l *inboundRateLimiter) allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(l.lastFill)
	if elapsed >= time.Second {
		l.tokens = l.max
		l.lastFill = now
	} else {
		fill := int(elapsed.Seconds() * float64(l.rate))
		l.tokens = min(l.tokens+fill, l.max)
		if fill > 0 {
			l.lastFill = now
		}
	}
	if l.tokens <= 0 {
		return false
	}
	l.tokens--
	return true
}

// dispatchNATTInbound reads packets from the NAT-T transport (port 4500) and
// dispatches IKE packets after stripping the non-ESP marker.
// RFC 3948 Section 2.2: 4 zero bytes prefix distinguishes IKE from ESP.
func dispatchNATTInbound(tr *transport.UDPTransport, table *SATable, log *slog.Logger) {
	limiter := newInboundRateLimiter(100, 200)
	// RFC 7296 Section 2.21.4 rate-limits SENDING in answer to unprotected messages.
	// It is a separate budget from the inbound processing limiter above.
	// A flood of forged out-of-SA datagrams therefore cannot spend the allowance that
	// keeps real sessions alive (notify_error.go).
	notifyLimiter := newOutboundNotifyLimiter(unprotectedNotifyRate, unprotectedNotifyBurst)

	for pkt := range tr.Recv() {
		if transport.IsNATKeepalive(pkt.Data) {
			continue
		}

		ikeData, isIKE := transport.StripNonESPMarker(pkt.Data)
		if !isIKE {
			continue
		}

		if len(ikeData) < 28 {
			continue
		}
		if ikeData[17]>>4 != 2 {
			continue
		}
		if !limiter.allow() {
			continue
		}

		var iSPI, rSPI [8]byte
		copy(iSPI[:], ikeData[0:8])
		copy(rSPI[:], ikeData[8:16])

		if iSPI == [8]byte{} {
			continue
		}

		nattPkt := transport.Packet{
			Data:       ikeData,
			RemoteAddr: pkt.RemoteAddr,
			LocalAddr:  pkt.LocalAddr,
			// The marker is stripped, the arrival socket is not. RFC 7296
			// Section 2.11 answers on the socket the request reached.
			NATT: pkt.NATT,
		}

		sa := table.Lookup(iSPI, rSPI)
		if sa == nil {
			sa = table.lookupByInitiatorSPI(iSPI)
		}
		if sa == nil {
			if tryResponderSAInit(nattPkt, iSPI, rSPI, table, tr, log) {
				continue
			}
			log.Debug("ike: no SA for NAT-T packet", "ispi", SPIHex(iSPI), "rspi", SPIHex(rSPI))
			// RFC 7296 Section 2.21.4 gives a MAY here.
			// A message received outside the context of a known IKE SA, and that is
			// not a request to start one, can draw an unprotected INVALID_IKE_SPI.
			// answerOutOfSA holds each guard.
			answerOutOfSA(tr, nattPkt, true, notifyLimiter, log)
			continue
		}

		routeInbound(sa, nattPkt, table, tr, log)
	}
}

// inboundQueueDepth bounds the per-session owner-loop inbound queue. Control-plane
// exchanges are one-at-a-time per SA, so a small buffer absorbs the establish
// hand-off window (SA marked established before maintainSA starts consuming)
// without letting a stalled owner back up the shared dispatch goroutine.
const inboundQueueDepth = 16

// lookupPeerSession returns the running session for a peer name, or nil.
func lookupPeerSession(name string) *PeerSession {
	peersMu.RLock()
	defer peersMu.RUnlock()
	if activePeersMap == nil {
		return nil
	}
	return activePeersMap[name]
}

// routeInbound delivers a received packet to the correct handler. For the SA that
// maintainSA currently owns it hands the packet to that owner loop (single-owner
// model, spec-ipsec-13) via a non-blocking send; every other SA -- an initial or a
// PARALLEL half-open handshake (spec-fixit-ipsec-clear-reestablish) -- is handled
// inline on the dispatch goroutine. If the owner queue is full the packet is dropped
// and the peer will retransmit.
func routeInbound(sa *SA, pkt transport.Packet, table *SATable, tr *transport.UDPTransport, log *slog.Logger) {
	// Key the owner-loop hand-off on SA identity, not the peer name: `ps.ownedSA` is
	// an atomic.Pointer the session goroutine keeps pointed at the exact SA maintainSA
	// owns (updated on an IKE-SA rekey swap too), so reading it here on the shared
	// dispatch goroutine does not race owner-side sa.State writes, and a parallel
	// half-open SA of the same peer is NOT misdelivered to the established SA's owner
	// loop (which would decrypt it under the wrong keys). RFC 7296 Section 2.4.
	if ps := lookupPeerSession(sa.PeerName); ps != nil && ps.inbound != nil && ps.ownedSA.Load() == sa {
		select {
		case ps.inbound <- pkt:
		default:
			// A blocking send would stall every peer on this goroutine, so a full
			// queue drops the packet. RFC 7296 Section 2.1 makes that survivable in
			// both directions, but only because both are retransmitted.
			//
			// A dropped REQUEST costs latency, because the peer repeats it. A
			// dropped RESPONSE costs latency only where this side repeats its own
			// request. serviceRekeyRetransmit does that for a rekey, and
			// retransmitDPD does it for a liveness probe. A self-initiated request
			// that nothing repeats puts its answer at risk here.
			log.Warn("ike: owner inbound queue full, dropping packet", "peer", sa.PeerName)
		}
		return
	}
	handleInbound(sa, pkt, table, tr, log)
}

// matchResponderPeer finds a running `respond` peer whose configured remote
// address equals the packet source, or nil. Used to accept an unsolicited
// IKE_SA_INIT. Called on the dispatch goroutine; reads immutable session config
// under the peers lock.
func matchResponderPeer(remoteAddr *net.UDPAddr) *PeerSession {
	if remoteAddr == nil {
		return nil
	}
	src := remoteAddr.IP.String()
	peersMu.RLock()
	defer peersMu.RUnlock()
	if activePeersMap == nil {
		return nil
	}
	for _, ps := range activePeersMap {
		if ps.peerCfg.ConnectionType != ipsec.ConnectionRespond {
			continue
		}
		if ps.peerCfg.RemoteAddress != "" && ps.peerCfg.RemoteAddress == src {
			return ps
		}
	}
	return nil
}

// tryResponderSAInit accepts an unsolicited IKE_SA_INIT request (no SATable entry)
// from a configured `respond` peer: it creates the responder SA, inserts it, and
// hands the packet to the handshake handler. Returns true when the packet was
// consumed (accepted or deliberately dropped as an unconfigured/duplicate attempt).
// RFC 7296 Section 1.2, Section 2.6.
func tryResponderSAInit(pkt transport.Packet, iSPI, rSPI [8]byte, table *SATable, tr *transport.UDPTransport, log *slog.Logger) bool {
	// Header: [18]=exchange type, [19]=flags. Must be an IKE_SA_INIT request with a
	// zero responder SPI (a fresh initiation, not a retransmit of a known SA).
	if len(pkt.Data) < 20 {
		return false
	}
	if pkt.Data[18] != wire.ExchangeIKESAInit || pkt.Data[19]&wire.FlagResponse != 0 {
		return false
	}
	if rSPI != ([8]byte{}) {
		return false
	}
	ps := matchResponderPeer(pkt.RemoteAddr)
	if ps == nil {
		log.Debug("ike: unsolicited IKE_SA_INIT from unconfigured source", "src", pkt.RemoteAddr)
		return false
	}
	// RFC 7296 Section 2.6, and the whole reason this block sits HERE rather than after
	// the CAS below: a responder "commits no state to an SA until it knows the initiator
	// can receive packets at the address from which it claims to be sending them".
	//
	// The CAS below takes this peer's ONLY half-open slot, and reapStaleHandshake frees
	// it only after responderHandshakeTimeout. Before this gate existed, one spoofed
	// datagram bearing a configured peer's source address denied that peer's IKE for 30
	// seconds, and a datagram every 30 seconds denied it indefinitely. The inbound token
	// bucket does not help: it is global and generous, and the attack needs one packet
	// per 30 seconds. Moving this block after the CAS reopens exactly that defect.
	if cookieRequired(ps) {
		cookie, nonce, scanned := scanSAInitPreState(pkt.Data)
		if !scanned || len(nonce) == 0 || !verifyCookie(cookie, iSPI, nonce, cookieRemoteIP(pkt.RemoteAddr), time.Now()) {
			if len(cookie) > 0 {
				countCookieVerifyFailure(ps.peerName)
			}
			// RFC 7296 Section 2.6 MUST: a cookie that does not match is IGNORED, not
			// rejected -- "process the message as if no cookie had been included;
			// usually this means sending a response containing a new cookie". A
			// pressured responder processes a cookie-less initiation by challenging
			// it, so that is what happens here. No SA, no CAS, no table entry.
			//
			// Do NOT "harden" this into a rejection. It would break conformance with
			// the paragraph above, and TestCkeMismatchedCookieIsIgnoredNotRejected
			// exists to catch it.
			fresh := mintCookie(iSPI, nonce, cookieRemoteIP(pkt.RemoteAddr), time.Now())
			if fresh == nil {
				log.Warn("ike: cannot mint a COOKIE, dropping the initiation", "peer", ps.peerName, "src", pkt.RemoteAddr)
				return true
			}
			log.Debug("ike: challenging inbound IKE_SA_INIT with a COOKIE", "peer", ps.peerName, "src", pkt.RemoteAddr)
			sendCookieChallenge(tr, pkt.LocalAddr, pkt.RemoteAddr, iSPI, fresh, ps.peerName, log)
			return true
		}
	}
	// One in-flight HALF-OPEN handshake per responder peer (AC-6). A genuine
	// retransmit finds the SA already in the SATable and never reaches this path.
	// RFC 7296 Section 2.4: the busy gate is NOT held across an established SA's
	// lifetime, so a fresh IKE_SA_INIT that arrives while a tunnel is up passes the
	// CAS and is accepted in PARALLEL; the established SA is never touched by this
	// unauthenticated message and is superseded only once the new SA authenticates
	// (finishResponderEstablish). This is the AC-3 / AC-7 accept-in-parallel path.
	if !ps.responderBusy.CompareAndSwap(false, true) {
		log.Debug("ike: responder busy, dropping concurrent IKE_SA_INIT", "peer", ps.peerName)
		return true
	}
	sa, err := newResponderSA(ps.peerName, ps.peerCfg, ps.ikeGroup, ps.espGroup, iSPI)
	if err == nil {
		// The peering's EAP-TLS resumption state, so the ticket keys this
		// authenticator issues under outlive the SA (eapTLSServerConfig).
		sa.Resumption = ps.resumption
		// Both sockets, before the IKE_SA_INIT is processed. detectResponderNAT runs
		// inside that processing and can float the SA at once (RFC 7296 Section 2.23).
		sa.bindSockets(ps.ike, ps.natt)
	}
	if err != nil {
		log.Warn("ike: create responder SA failed", "peer", ps.peerName, "error", err)
		ps.responderBusy.Store(false)
		return true
	}
	if !table.Insert(sa) {
		log.Debug("ike: responder SA insert conflict", "peer", ps.peerName)
		ps.responderBusy.Store(false)
		return true
	}
	if ps.ownedSA.Load() != nil {
		// An established SA already owns the loop: the new handshake coexists in the
		// second slot and drives inline on the dispatch goroutine (routeInbound keys
		// on SA identity, so it is not delivered to the old SA's owner loop).
		ps.setPendingSA(sa)
		log.Info("ike: accepting parallel inbound IKE_SA_INIT alongside established SA", "peer", ps.peerName, "src", pkt.RemoteAddr)
	} else {
		ps.setSA(sa)
		log.Info("ike: accepting inbound IKE_SA_INIT", "peer", ps.peerName, "src", pkt.RemoteAddr)
	}
	routeInbound(sa, pkt, table, tr, log)
	return true
}

// dispatchInbound reads packets from the transport and dispatches to the
// correct SA by SPI pair.
func dispatchInbound(tr *transport.UDPTransport, table *SATable, log *slog.Logger) {
	limiter := newInboundRateLimiter(100, 200)
	// A separate SEND budget, for the reason given in dispatchNATTInbound.
	notifyLimiter := newOutboundNotifyLimiter(unprotectedNotifyRate, unprotectedNotifyBurst)

	for pkt := range tr.Recv() {
		if len(pkt.Data) < 28 {
			continue
		}
		// RFC 7296 Section 3.1: major version in upper nibble of byte 17.
		if pkt.Data[17]>>4 != 2 {
			continue
		}
		if !limiter.allow() {
			continue
		}

		var iSPI, rSPI [8]byte
		copy(iSPI[:], pkt.Data[0:8])
		copy(rSPI[:], pkt.Data[8:16])

		// RFC 7296 Section 2.6: initiator SPI MUST NOT be zero.
		if iSPI == [8]byte{} {
			continue
		}

		sa := table.Lookup(iSPI, rSPI)
		if sa == nil {
			sa = table.lookupByInitiatorSPI(iSPI)
		}
		if sa == nil {
			if tryResponderSAInit(pkt, iSPI, rSPI, table, tr, log) {
				continue
			}
			log.Debug("ike: no SA for packet", "ispi", SPIHex(iSPI), "rspi", SPIHex(rSPI))
			// RFC 7296 Section 2.21.4, as on the NAT-T path. This socket carries no
			// non-ESP marker.
			answerOutOfSA(tr, pkt, false, notifyLimiter, log)
			continue
		}

		routeInbound(sa, pkt, table, tr, log)
	}
}

// resolveInterfaceAddrFn is the lookup the apply path takes (applyConfig, apply.go), so a
// test can move the address ze listens at without a second interface on the box
// (apply_test.go). Production never replaces it.
var resolveInterfaceAddrFn = resolveInterfaceAddr

// resolveInterfaceAddr returns the first IPv4 address of the logical interface, resolved
// through the shared iface resolver so the IKE bind and listen address honors the os-name
// and mac-match selectors instead of assuming the name is the kernel device.
//
// The error distinguishes two failures that both yield an empty address. The
// interface lookup itself can fail, and the interface can carry no IPv4 address.
// They need different operator action, so the caller MUST report the error and
// never assume the second cause. An empty address with a nil error means the
// interface is present and has no IPv4 address.
func resolveInterfaceAddr(name string) (string, error) {
	// EnsureBackend FIRST. iface.Addresses fails with "iface: no backend loaded" unless
	// something has loaded one, and a peer configured with `interface eth0` and no
	// `interfaces` block loads none. OnConfigure only WARNs on that error, so
	// peer.LocalAddress stayed empty and createFirstChildSA (child.go) then rejected
	// net.ParseIP("") on every retry. The tunnel installed no Child SA at all, and the
	// interop lab showed strongSwan's XFRM SA present beside none of ze's, for as long
	// as an `except (AssertionError, Exception)` in the scenario reported that as a pass.
	//
	// This is the guard ospf (plugins/ospf/interface_addr.go) and static already apply
	// before their own iface.Addresses lookups. IKE was the caller without it.
	_ = iface.EnsureBackend()
	addrs, err := iface.Addresses(name)
	if err != nil {
		return "", err
	}
	for _, a := range addrs {
		if a.Family == "ipv4" {
			return a.Address, nil
		}
	}
	return "", nil
}
