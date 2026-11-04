// Design: docs/architecture/bgp/filter-irr.md -- IRR prefix-list filter plugin entry point

package filter_irr

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/component/resolve/irr"
	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/internal/component/resolve/peeringdb"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

var errFilterIrrInvalidBgpConfigJson = errors.New("filter-irr: invalid bgp config JSON")

const (
	// perASNRefreshTimeout bounds a single ASN's IRR resolution on the periodic
	// and manual ("update bgp irr ...") refresh paths, where no startup barrier
	// is waiting.
	perASNRefreshTimeout = 30 * time.Second

	configRootBGP = "bgp"
)

var logger = slogutil.LazyLogger("bgp.filter.irr")

type irrMetrics struct {
	prefixesCached  metrics.Gauge
	refreshOutcomes metrics.CounterVec
	lastRefresh     metrics.Gauge
}

var irrMetricsPtr atomic.Pointer[irrMetrics]

func SetMetricsRegistry(reg metrics.Registry) {
	m := &irrMetrics{
		prefixesCached:  reg.Gauge("ze_irr_prefixes_cached", "Total IRR-resolved prefixes cached across all ASNs."),
		refreshOutcomes: reg.CounterVec("ze_irr_refresh_outcomes_total", "IRR refresh outcomes.", []string{"result"}),
		lastRefresh:     reg.Gauge("ze_irr_last_refresh_timestamp", "Unix timestamp of last successful IRR refresh."),
	}
	irrMetricsPtr.Store(m)
}

type asnState struct {
	asn uint32
	// configured is the as-set the operator wrote for this ASN, empty when the
	// config names none and PeeringDB chooses it. It changes only with config,
	// and it is the only query hint: an empty one makes every lookup ask
	// PeeringDB, so an upstream AS-SET change is followed on the next refresh.
	configured string
	// asSet is the as-set this ASN last resolved under: configured when the
	// config names one, otherwise the one the last lookup used, empty before
	// the first lookup. It is what `show bgp irr` reports, never the hint.
	asSet string
	// lookups is canceled when a reload replaces this state with one that
	// does not carry it over (the ASN dropped, its configured set changed or
	// deleted), so a lookup still in flight for it commits nothing to the
	// store (store.commit). A state carried over shares its predecessor's.
	lookups       context.Context
	cancelLookups context.CancelFunc
	list          *irrPrefixList
	lastOK        time.Time
	lastErr       string
	v4Count       int
	v6Count       int
	peerAddrs     []string
}

// newASNState builds an enrolled ASN's state, unresolved. Every ASN the
// configure path enrolls goes through here, so each holds a lookups context a
// reload can cancel.
func newASNState(asn uint32, configured string) *asnState {
	lookups, cancel := context.WithCancel(context.Background())
	return &asnState{
		asn: asn, configured: configured, asSet: configured,
		lookups: lookups, cancelLookups: cancel,
	}
}

// carriedOverBy reports whether next, the state a reload built for this ASN,
// keeps this state's resolution: it does only while the configured as-set is
// the same, including both being empty. A changed or deleted set answers a
// question the operator no longer asks.
func (st *asnState) carriedOverBy(next *asnState) bool {
	if next == nil {
		return false
	}
	return next.configured == st.configured
}

// supersede cancels this state's in-flight lookups. Safe on a test literal
// that carries no lookups context.
func (st *asnState) supersede() {
	if st.cancelLookups != nil {
		st.cancelLookups()
	}
}

// irrSnapshot is what the filter evaluator reads: one immutable value per
// publish, swapped whole, so a route is judged wholly by one config and one
// set of lists (AC-34). It holds every enrolled ASN, mapped to its list, nil
// while unresolved. The goroutines that write byASN under mu publish it
// (publishLocked); the evaluator takes no lock and never waits.
type irrSnapshot struct {
	asns map[uint32]irrSnapshotEntry
}

// irrSnapshotEntry is one enrolled ASN in a snapshot.
type irrSnapshotEntry struct {
	list   *irrPrefixList // nil while the ASN has no list: unresolved
	failed bool           // the last lookup failed (the list, if any, is the last good one)
}

// lookup answers asn's entry and whether this config enrolls it. Safe on a
// nil snapshot, which enrolls nothing: the plugin before its first configure.
func (snap *irrSnapshot) lookup(asn uint32) (irrSnapshotEntry, bool) {
	if snap == nil {
		return irrSnapshotEntry{}, false
	}
	entry, ok := snap.asns[asn]
	return entry, ok
}

type irrPlugin struct {
	plugin *sdk.Plugin
	// prefixStore holds the resolved lists in memory only, and lives as long as
	// the plugin: the first configure creates it and every later one points it
	// at the configured servers (UseClients). It is never written to disk, so a
	// restart starts with no list and resolves every enrolled ASN again.
	prefixStore *store.PrefixStore
	// started is set by OnStarted, after the startup handshake. Configure runs
	// inside that handshake, so the first configure leaves resolution to
	// OnStarted, and only a reload configure, which arrives after it, starts
	// resolution itself. Guarded by mu.
	started bool

	mu    sync.RWMutex
	byASN map[uint32]*asnState
	// snapshot is byASN as the evaluator reads it, republished by every
	// writer of byASN or of a state's list (publishLocked).
	snapshot    atomic.Pointer[irrSnapshot]
	config      *irrConfig
	nextRefresh time.Time
	lastRefresh time.Time

	stopCh      chan struct{}
	refreshStop chan struct{}
	refreshing  atomic.Bool

	// readmit holds the prefixes the filter rejected because the list did not
	// authorize them yet (readmit.go). Nil in tests that build the plugin as a
	// literal, where nothing is held.
	readmit *readmitter
}

func runFilterIRR(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-filter-irr", conn)
	defer func() { _ = p.Close() }()

	plug := &irrPlugin{
		plugin: p,
		byASN:  make(map[uint32]*asnState),
		stopCh: make(chan struct{}),
	}
	plug.readmit = newReadmitter(dispatchEngine{plugin: p}, plug, readmitRetryInterval)

	// A plugin holding OnConfigure alone answers every config-apply OK and
	// changes nothing, so a live IRR edit was accepted and ignored. The reload
	// pair is what makes a commit reach the filter (config_tx.go).
	newIRRConfigTx(plug).Register(p)

	// The one evaluator registration (FR-8): the typed bridge slot when the
	// plugin runs internal, the filter-update batch when it runs forked.
	p.OnFilter(plug.evaluate)

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
		return plug.handleCommand(command, args)
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	defer close(plug.stopCh)
	p.OnStarted(func(_ context.Context) error {
		plug.mu.Lock()
		plug.started = true
		ps := plug.prefixStore
		cfg := plug.config
		refreshStop := plug.refreshStop
		plug.mu.Unlock()
		// The re-admission worker lives as long as the plugin, across every
		// configure cycle, and stops with stopCh.
		go plug.readmit.run(plug.stopCh)
		if ps == nil {
			return nil
		}
		go plug.initialResolve()
		go plug.refreshLoop(cfg.RefreshInterval, refreshStop)
		return nil
	})

	// The IRR and PeeringDB clients this plugin uses live in its prefix
	// store, so the store answers clear irr cache and clear peeringdb cache.
	p.OnResolverCacheClear(store.ResolverCaches(), plug.clearResolverCache)

	if err := p.Run(ctx, &sdk.Registration{
		Commands: commandDecls(),
		// A forked program has no registry row to supply ConfigReads, so its
		// Stage 1 declaration must request the policy settings as well as peers.
		WantsConfig: []string{configRootBGP, policy.RootName},
		Filters:     filterDecls(),
	}); err != nil {
		logger().Error("filter-irr plugin failed", "error", err)
		return 1
	}
	return 0
}

// clearResolverCache answers clear irr cache and clear peeringdb cache for the
// clients the prefix store uses now. Before the first configure the plugin
// holds no client, so it holds no entry and zero is the true count.
func (plug *irrPlugin) clearResolverCache(input rpc.ResolverCacheClearInput) (int, error) {
	plug.mu.RLock()
	ps := plug.prefixStore
	plug.mu.RUnlock()
	if ps == nil {
		return 0, nil
	}
	return ps.ClearResolverCache(input)
}

// handleConfigure makes cfg the filter's config, at boot and on every
// applied or rolled-back reload. The new per-ASN state is published under
// plug.mu in one swap, so a concurrent filter UPDATE reads the old state or
// the new one, never a mix.
func (plug *irrPlugin) handleConfigure(delivery irrDelivery) {
	cfg := parseIRRConfig(delivery.policy, delivery.bgp)

	irrClient := irr.NewIRR(cfg.Server)
	if cfg.SourceAddress != "" {
		irrClient.SetSourceAddress(cfg.SourceAddress)
	}
	pdb := peeringdb.NewPeeringDB(cfg.PeeringDBURL)
	plug.mu.RLock()
	ps := plug.prefixStore
	started := plug.started
	plug.mu.RUnlock()
	// One store for the plugin's life, never one per configure: the store's
	// per-family last-known-good (store.commit) merges a refresh into the entry
	// it already holds, and a fresh store would hold none after every reload.
	if ps == nil {
		ps = store.New(irrClient, pdb, nil)
	} else {
		ps.UseClients(irrClient, pdb)
	}

	newByASN := make(map[uint32]*asnState)
	for _, peer := range cfg.Peers {
		if peer.Disabled {
			continue
		}
		// Enroll ONLY peers that asked for IRR filtering. Enrolling on the mere
		// presence of a remote ASN made every BGP config -- including one with
		// no IRR filter anywhere -- issue a PeeringDB HTTPS lookup and a whois
		// query to the IRR server (default RADB) per peer at every startup, on
		// the operator's behalf and without being asked. That also contradicted
		// the documented behavior (docs/guide/irr-filtering.md: a peer with no
		// "import { filters [ bgp-filter-irr:<asn> ]; final accept; }" reference has no `show bgp irr`
		// entry) and this plugin's own vocabulary ("no IRR-filtered peer with
		// ASN %d", command.go). An unenrolled peer's filter chain never reaches
		// handleFilterUpdate, so nothing else observes the difference.
		if !peer.UsesIRR {
			continue
		}
		st, exists := newByASN[peer.RemoteASN]
		if !exists {
			st = newASNState(peer.RemoteASN, peer.ASSet)
			newByASN[peer.RemoteASN] = st
		}
		st.peerAddrs = append(st.peerAddrs, peer.PeerAddr)
		if peer.ASSet != "" && st.configured == "" {
			st.configured = peer.ASSet
			st.asSet = peer.ASSet
		}
	}

	// All mutable plugin fields are published under plug.mu so a concurrent
	// `show bgp irr` / refresh never reads a half-updated configure.
	plug.mu.Lock()
	oldByASN := plug.byASN
	for asn, oldSt := range oldByASN {
		newSt := newByASN[asn]
		// A list resolved for another AS-SET answers a question the operator
		// no longer asks. Carrying it over would filter against the old set,
		// until the next refresh: the edit
		// would be accepted and not enforced. Deleting the configured set is
		// a change too, because PeeringDB chooses the set again. The old
		// state's lookups are canceled, so one in flight commits nothing
		// that store.discoverASSet could later hand back as its fallback.
		if !oldSt.carriedOverBy(newSt) {
			oldSt.supersede()
			continue
		}
		newSt.supersede() // its own context is replaced by the one it inherits
		newSt.lookups, newSt.cancelLookups = oldSt.lookups, oldSt.cancelLookups
		newSt.asSet = oldSt.asSet
		newSt.list = oldSt.list
		newSt.lastOK = oldSt.lastOK
		newSt.lastErr = oldSt.lastErr
		newSt.v4Count = oldSt.v4Count
		newSt.v6Count = oldSt.v6Count
	}
	// The purge runs before the new states are published: a refresh that reads
	// a new state must find the store already without the entries it drops, or
	// with PeeringDB down discoverASSet hands a deleted set back. The store
	// takes only its own lock and never calls back, so holding plug.mu is safe.
	forgetUnconfigured(ps, oldByASN, newByASN)
	plug.byASN = newByASN
	plug.publishLocked() // a changed as-set publishes its ASN unresolved
	plug.prefixStore = ps
	plug.config = cfg
	if plug.refreshStop != nil {
		close(plug.refreshStop)
	}
	plug.refreshStop = make(chan struct{})
	refreshStop := plug.refreshStop
	plug.mu.Unlock()

	// A prefix held for a peer this config no longer filters under that ASN
	// has no list to be re-evaluated against, and no peer to ask.
	if plug.readmit != nil {
		owners := make(map[heldOwner]struct{}, len(cfg.Peers))
		for i := range cfg.Peers {
			peer := &cfg.Peers[i]
			if peer.enrolled() && peer.RemoteIP.IsValid() {
				owners[heldOwner{peer: peer.RemoteIP, asn: peer.RemoteASN}] = struct{}{}
			}
		}
		plug.readmit.retain(owners)
	}
	for _, name := range peersMissingRefreshGrant(cfg) {
		logger().Warn("irr: a route this peer sends before its IRR list authorizes it stays rejected until the peer sends it again: the peer does not let bgp-filter-irr ask it for its routes",
			"peer", name,
			"action", "add `attach process bgp-filter-irr { send [ refresh ] }` to this peer, or to its group")
	}
	for _, name := range peersMissingRouteRefresh(cfg) {
		logger().Warn("irr: a route this peer sends before its IRR list authorizes it stays rejected until the peer sends it again: ze does not advertise Route Refresh to this peer, so the capability is never negotiated and bgp-filter-irr cannot ask the peer for its routes",
			"peer", name,
			"action", "add `session { capability { route-refresh enable } }` to this peer, or to its group, and enable Route Refresh on the peer's side too; the capability is negotiated when the session next comes up")
	}

	// Resolve every enrolled ASN ONCE in the background, NOT inline. configure
	// runs inside the plugin startup handshake, gated by the engine's stage
	// barrier. A synchronous first resolution here would make ze's startup
	// depend on reaching the IRR server (default RADB), so an unreachable or slow
	// server would fail the configure stage and bring the whole BGP plugin set
	// down -- even for a config that has BGP peers but no IRR import filter.
	// Startup must never depend on external IRR reachability, so the first
	// resolution is detached. An UPDATE arriving for an IRR-filtered peer before
	// it finishes is judged at once against the unresolved list: its announced
	// prefixes take the filter's on-error reject, are held, and the readmitter
	// asks the peer for them again once the list is published (AC-32). The
	// periodic refreshLoop keeps the list fresh thereafter.
	if started {
		go plug.initialResolve()
		go plug.refreshLoop(cfg.RefreshInterval, refreshStop)
	}

	// "peers" counts the peers ENROLLED for IRR resolution, not every BGP peer:
	// an operator reading this line is asking how much IRR work was set up, and
	// the config's total peer count would answer a different question.
	enrolledPeers := 0
	for _, st := range newByASN {
		enrolledPeers += len(st.peerAddrs)
	}
	logger().Debug("configured", "peers", enrolledPeers, "asns", len(newByASN), "server", cfg.Server)
}

// forgetUnconfigured drops from the store every entry the new config no longer
// asks for: an ASN no longer enrolled, an ASN whose configured as-set is not
// the one its entry was resolved for, and an ASN whose configured as-set the
// new config deleted. store.commit keeps a cached family only for the same
// AS-SET, so an old-set entry cannot hand its prefixes to the new set; purging
// it keeps the store to the sets the config names. The deleted set's entry
// must go too, because store.discoverASSet keeps a cached entry's AS-SET when
// PeeringDB fails, and it would hand the deleted set back.
// This is the store's half of "a changed as-set starts from no list";
// handleConfigure's carry-over rule is the plugin's half. The caller MUST hold
// plug.mu, so no refresh reads a new state before the purge.
func forgetUnconfigured(ps *store.PrefixStore, oldByASN, newByASN map[uint32]*asnState) {
	for asn := range oldByASN {
		if _, kept := newByASN[asn]; !kept {
			ps.Purge(asnName(asn))
		}
	}
	for asn, st := range newByASN {
		if st.configured == "" {
			if old := oldByASN[asn]; old != nil && old.configured != "" {
				ps.Purge(asnName(asn))
			}
			continue // the as-set is discovered, so no configured one to differ from
		}
		entry := ps.Get(asnName(asn))
		if entry == nil {
			continue
		}
		if entry.ASSet != st.configured {
			ps.Purge(asnName(asn))
		}
	}
}

// initialResolve performs the first background resolution of every enrolled
// ASN. It runs detached from the configure path (so startup never blocks on IRR
// network I/O) and, unlike refreshAll, does NOT honor the refreshing CAS guard:
// it must always do the work, because until it does every enrolled ASN is
// unresolved and its announced prefixes are rejected and held.
func (plug *irrPlugin) initialResolve() {
	plug.mu.RLock()
	asns := make([]uint32, 0, len(plug.byASN))
	for asn := range plug.byASN {
		asns = append(asns, asn)
	}
	plug.mu.RUnlock()

	for _, asn := range asns {
		plug.refreshASN(asn)
	}
}

// evaluate is the plugin's filter evaluator, on both transports (OnFilter).
// It honors the fast-path contract: it reads only the published snapshot,
// never waits, and keeps nothing of route past its return.
func (plug *irrPlugin) evaluate(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
	in, err := route.Input()
	if err != nil {
		return nil, err
	}
	return plug.handleFilterUpdate(&in), nil
}

// handleFilterUpdate judges one UPDATE against the snapshot entry of the ASN
// its filter name carries, from the prefix and withdrawn fields. It never
// waits: an ASN with no list yet rejects its announced prefixes, the filter's
// on-error verdict, keeps the withdrawals, and holds the prefixes for the
// readmitter, which asks the peer for them again once refreshASNCtx publishes
// the list (FR-10, AC-32). A rejected prefix beside an admitted one, or beside
// a withdrawal, is dropped alone by a prefix remove, which the engine applies
// to the body NLRI and to MP_REACH_NLRI (spec D-5).
func (plug *irrPlugin) handleFilterUpdate(in *sdk.FilterUpdateInput) *sdk.FilterUpdateOutput {
	asn := extractASNFromFilter(in.Filter)
	if asn == 0 {
		logger().Warn("irr filter: cannot extract ASN from filter name", "filter", in.Filter, "peer", in.Peer)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}

	entry, enrolled := plug.snapshot.Load().lookup(asn)
	list := entry.list
	if list != nil && len(list.entries) == 0 {
		list = nil
	}
	judged, err := judgeNLRI(in.Fields, list)
	if err != nil {
		// Fail closed, and hold nothing: nothing in the route can be judged
		// again.
		logger().Warn("irr filter reject", "filter", in.Filter, "peer", in.Peer, "reason", "fields", "error", err)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}
	var out *sdk.FilterUpdateOutput
	if list == nil {
		out = judgeUnlisted(in, &judged, entry.failed)
	} else {
		out = judgeListed(in, &judged)
	}
	// A whole reject admits no announced prefix (judgeUnlisted admits none,
	// judgeListed rejects whole only when none is in list), so the per-prefix
	// verdicts are the whole account of what the update held.
	plug.trackVerdict(in.Peer, asn, enrolled, &judged)
	return out
}

// judgeUnlisted decides an UPDATE for an ASN that has no list yet: every
// announced prefix is rejected, and the withdrawals still go through.
func judgeUnlisted(in *sdk.FilterUpdateInput, judged *routeNLRI, lookupFailed bool) *sdk.FilterUpdateOutput {
	if out, kept := keepWithdrawals(judged); kept {
		logger().Info("irr filter keeps the withdrawals", "filter", in.Filter, "peer", in.Peer,
			"verdict", out.Action, "rejected", judged.rejected, "reason", "no-prefix-list", "lookup-failed", lookupFailed)
		return out
	}
	logger().Info("irr filter reject", "filter", in.Filter, "peer", in.Peer, "rejected", judged.rejected,
		"reason", "no-prefix-list", "lookup-failed", lookupFailed)
	return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
}

// judgeListed decides an UPDATE against the ASN's list.
func judgeListed(in *sdk.FilterUpdateInput, judged *routeNLRI) *sdk.FilterUpdateOutput {
	// Every judged prefix in list, or none to judge: accept unmodified.
	if judged.rejected == 0 {
		logger().Info("irr filter accept", "filter", in.Filter, "peer", in.Peer, "accepted", judged.accepted)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}
	}

	// Every announced prefix out of list: drop the announcements and keep the
	// withdrawals, else reject the whole update.
	if judged.accepted == 0 {
		if out, kept := keepWithdrawals(judged); kept {
			logger().Info("irr filter keeps the withdrawals", "filter", in.Filter, "peer", in.Peer,
				"verdict", out.Action, "rejected", judged.rejected)
			return out
		}
		logger().Info("irr filter reject", "filter", in.Filter, "peer", in.Peer, "rejected", judged.rejected)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}

	// Mixed: keep the in-list prefixes and drop the unauthorized ones.
	logger().Info("irr filter modify", "filter", in.Filter, "peer", in.Peer,
		"accepted", judged.accepted, "rejected", judged.rejected)
	return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Sets: judged.rejectedRemoves()}
}

// RFC 4271 Section 9: "If the UPDATE message contains a non-empty WITHDRAWN
// ROUTES field, the previously advertised routes, whose destinations
// (expressed as IP prefixes) are contained in this field, SHALL be removed
// from the Adj-RIB-In."
//
// keepWithdrawals answers an UPDATE that announces no prefix the filter
// accepts, and reports false when it carries no withdrawal to keep. The
// engine drops a rejected UPDATE whole, its withdrawals with it. A withdrawal
// alone is accepted. Beside rejected announcements it is kept by removing
// every rejected prefix: the engine drops those entries, removes an
// MP_REACH_NLRI left empty, and sends the withdrawals on (policychain
// applyPrefixIntent). Callers MUST have ruled out an accepted prefix first.
func keepWithdrawals(judged *routeNLRI) (*sdk.FilterUpdateOutput, bool) {
	if !judged.withdraws {
		return nil, false
	}
	if judged.rejected == 0 {
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, true
	}
	return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Sets: judged.rejectedRemoves()}, true
}

// trackVerdict tells the re-admission worker what one UPDATE's verdict did to
// the prefixes it holds. A withdrawal drops them, an accepted prefix is no
// longer held, and a rejected one is held until its ASN's list authorizes it.
// Only an ASN this config enrolls holds anything: no refresh of any other ASN
// ever runs, so its hold could only leak.
func (plug *irrPlugin) trackVerdict(peerAddr string, asn uint32, enrolled bool, judged *routeNLRI) {
	if plug.readmit == nil {
		return
	}
	// The path almost every UPDATE takes: nothing to hold and nothing held. The
	// count is atomic, so this path takes no lock.
	holds := enrolled && judged.rejected > 0
	if !holds && plug.readmit.heldCount() == 0 {
		return
	}
	peer, err := netip.ParseAddr(peerAddr)
	if err != nil {
		// The engine names the peer by its address; a name that is not one
		// identifies nothing the worker could ask a ROUTE-REFRESH of.
		logger().Debug("irr: filter UPDATE names no peer address, nothing held", "peer", peerAddr)
		return
	}
	for i := range judged.families {
		plug.readmit.track(peer, asn, enrolled, &judged.families[i])
	}
}

// heldList serves the re-admission worker the ASN's current list
// (readmitSource).
func (plug *irrPlugin) heldList(asn uint32) (*irrPrefixList, bool) {
	plug.mu.RLock()
	defer plug.mu.RUnlock()
	st := plug.byASN[asn]
	if st == nil {
		return nil, false
	}
	return st.list, st.list == nil || st.lastErr != ""
}

// resolveHeld resolves the ASN for the re-admission worker (readmitSource).
func (plug *irrPlugin) resolveHeld(asn uint32) {
	plug.refreshASN(asn)
}

func extractASNFromFilter(filter string) uint32 {
	for i := len(filter) - 1; i >= 0; i-- {
		if filter[i] < '0' || filter[i] > '9' {
			if i < len(filter)-1 {
				v, ok := readUint(filter[i+1:])
				if ok && v <= 0xFFFFFFFF {
					return uint32(v) //nolint:gosec // range checked
				}
			}
			return 0
		}
	}
	v, ok := readUint(filter)
	if ok && v <= 0xFFFFFFFF {
		return uint32(v) //nolint:gosec // range checked
	}
	return 0
}

func (plug *irrPlugin) refreshLoop(intervalSec uint32, stop chan struct{}) {
	interval := time.Duration(intervalSec) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	plug.mu.Lock()
	plug.nextRefresh = time.Now().Add(interval)
	plug.mu.Unlock()

	for {
		select {
		case <-ticker.C:
			plug.refreshAll()
			plug.mu.Lock()
			plug.nextRefresh = time.Now().Add(interval)
			plug.mu.Unlock()
		case <-stop:
			return
		case <-plug.stopCh:
			return
		}
	}
}

// refreshAll is the background entry point (configure + periodic timer). The
// CAS guard collapses overlapping background refreshes into one.
func (plug *irrPlugin) refreshAll() {
	if !plug.refreshing.CompareAndSwap(false, true) {
		return
	}
	defer plug.refreshing.Store(false)
	plug.refreshAllNow()
}

// refreshAllNow refreshes every enrolled ASN synchronously, without the
// concurrent-refresh guard. The manual `update bgp irr all` command uses this so
// it always does the work and reports honest success/failure, rather than
// skipping (and falsely reporting "done") when a periodic refresh is in flight.
func (plug *irrPlugin) refreshAllNow() {
	plug.mu.RLock()
	asns := make([]uint32, 0, len(plug.byASN))
	for asn := range plug.byASN {
		asns = append(asns, asn)
	}
	plug.mu.RUnlock()

	for _, asn := range asns {
		plug.refreshASN(asn)
	}
}

func (plug *irrPlugin) refreshASN(asn uint32) {
	ctx, cancel := context.WithTimeout(context.Background(), perASNRefreshTimeout)
	defer cancel()
	plug.refreshASNCtx(ctx, asn)
}

// refreshASNCtx is refreshASN with a caller-supplied deadline. The success and
// failure arms publish the snapshot under the same lock that writes the
// state, so the evaluator sees the new list, or the failed lookup, at once.
// Periodic and manual refreshes use the per-ASN timeout via refreshASN. The
// list lives in memory only.
func (plug *irrPlugin) refreshASNCtx(ctx context.Context, asn uint32) {
	plug.mu.RLock()
	st := plug.byASN[asn]
	captured := st
	ps := plug.prefixStore
	asSet := ""
	if st != nil {
		asSet = st.configured
	}
	plug.mu.RUnlock()
	if st == nil || ps == nil {
		// No live state, or no store to resolve against: nothing to resolve,
		// and the ASN stays unresolved (it fails closed).
		return
	}

	// The store owns AS-SET discovery (PeeringDB), the IRR query, and the
	// in-memory last-known-good. The ASN is the stable identity/key; the
	// configured AS-SET is passed as a hint, and an empty one makes the store
	// ask PeeringDB on every refresh, keeping its cached set on an outage.
	// A reload that supersedes this state cancels the lookup, and the store
	// then commits nothing. A test literal carries no lookups context.
	// The lookup's context descends from the state's, so the reload's cancel
	// reaches it synchronously, before the reload purges the store; an
	// AfterFunc would cancel later, on its own goroutine, and the commit could
	// slip in between. The caller's ctx contributes its deadline only:
	// refreshASN, the one caller, derives it from Background.
	if captured.lookups != nil {
		var cancel context.CancelFunc
		if deadline, ok := ctx.Deadline(); ok {
			ctx, cancel = context.WithDeadline(captured.lookups, deadline)
		} else {
			ctx, cancel = context.WithCancel(captured.lookups)
		}
		defer cancel()
	}
	// The store was built with no persistence (handleConfigure), so Refresh
	// writes nothing durable and nothing between the IRR answer and the
	// published list waits on a disk.
	entry, err := ps.Refresh(ctx, asnName(asn), asSet)
	now := time.Now()

	plug.mu.Lock()
	// Re-fetch under the write lock: a reconfigure between the RLock above and
	// here may have replaced byASN, orphaning the captured st. Apply the result
	// to the live entry, or drop it if the ASN is no longer enrolled.
	st = plug.byASN[asn]
	if st == nil {
		plug.mu.Unlock()
		return
	}
	if st != captured && st.lookups != captured.lookups {
		// A reload changed or deleted this ASN's as-set while the lookup ran,
		// so the answer is for a set the config no longer names. Applying it
		// would filter against the old set, so drop it. The reload's own
		// resolution answers for the new config. A reload that kept the as-set
		// also replaces st, but carries the lookups context over, and its
		// answer is still the right one, so the context decides, not the
		// pointer alone. The reload canceled this lookup, so if it had not
		// committed by then it committed nothing; one that committed first
		// was purged by the reload, under the same lock order.
		liveASSet := st.asSet
		plug.mu.Unlock()
		logger().Debug("irr: dropped a lookup a reload superseded", "asn", asn, "as-set", asSet, "live-as-set", liveASSet)
		return
	}
	if entry != nil {
		// Record the resolved AS-SET even on a failed lookup, so `show bgp irr`
		// reflects the fallback name.
		st.asSet = entry.ASSet
	}
	var pl irr.PrefixList
	var entries []prefixEntry
	if err == nil {
		pl = entry.PrefixList()
		entries, err = prefixListFromIRR(pl)
	}
	if err != nil {
		st.lastErr = err.Error()
		queried := st.asSet
		plug.publishLocked()
		plug.mu.Unlock()
		logger().Warn("irr: lookup failed", "asn", asn, "as-set", queried, "error", err)
		incRefreshOutcome("error")
		return
	}

	st.list = &irrPrefixList{entries: entries}
	st.lastOK = now
	st.lastErr = ""
	st.v4Count = len(pl.IPv4)
	st.v6Count = len(pl.IPv6)
	plug.publishLocked()
	plug.lastRefresh = now
	plug.mu.Unlock()

	// A list that arrived or changed may now authorize a prefix the filter
	// rejected, so the re-admission worker checks what it holds.
	if plug.readmit != nil {
		plug.readmit.notify()
	}

	logger().Info("irr: refreshed", "asn", asn, "as-set", entry.ASSet, "v4", len(pl.IPv4), "v6", len(pl.IPv6))
	incRefreshOutcome("success")
	updateMetricsGauges(plug)
}

// publishLocked publishes byASN as the snapshot the evaluator reads. Every
// writer of byASN, or of a state's list or lastErr, calls it before releasing
// mu, so the snapshot never lags the state show bgp irr reports. A list is
// never written after it is stored in a state, so the snapshot shares it.
// The caller MUST hold plug.mu for writing.
func (plug *irrPlugin) publishLocked() {
	snap := &irrSnapshot{asns: make(map[uint32]irrSnapshotEntry, len(plug.byASN))}
	for asn, st := range plug.byASN {
		snap.asns[asn] = irrSnapshotEntry{list: st.list, failed: st.lastErr != ""}
	}
	plug.snapshot.Store(snap)
}

// asnName renders an ASN as its canonical "AS<n>" name, the PrefixStore key the
// BGP filter uses as a stable per-ASN identity.
func asnName(asn uint32) string {
	var tb textbuf.Buffer
	return tb.Str("AS").Uint32(asn).String()
}

const maxPrefixEntries = 500_000

func prefixListFromIRR(pl irr.PrefixList) ([]prefixEntry, error) {
	// A list over the cap is refused whole, never cut. A cut fills the budget
	// with IPv4 first, so the IPv6 half is lost and every IPv6 route rejected
	// while the refresh reports success.
	total := len(pl.IPv4) + len(pl.IPv6)
	if total > maxPrefixEntries {
		return nil, fmt.Errorf("irr: prefix list of %d entries (%d IPv4, %d IPv6) exceeds the %d-entry cap, "+
			"so it is refused and the previous list stays in force: narrow the AS-SET",
			total, len(pl.IPv4), len(pl.IPv6), maxPrefixEntries)
	}
	entries := make([]prefixEntry, 0, total)
	for _, p := range pl.IPv4 {
		entries = append(entries, prefixEntry{
			prefix: p,
			ge:     uint8(p.Bits()),
			le:     32,
		})
	}
	for _, p := range pl.IPv6 {
		entries = append(entries, prefixEntry{
			prefix: p,
			ge:     uint8(p.Bits()),
			le:     128,
		})
	}
	return entries, nil
}

func incRefreshOutcome(result string) {
	if m := irrMetricsPtr.Load(); m != nil {
		m.refreshOutcomes.With(result).Inc()
	}
}

func updateMetricsGauges(plug *irrPlugin) {
	m := irrMetricsPtr.Load()
	if m == nil {
		return
	}
	plug.mu.RLock()
	total := 0
	for _, st := range plug.byASN {
		total += st.v4Count + st.v6Count
	}
	plug.mu.RUnlock()
	m.prefixesCached.Set(float64(total))
	m.lastRefresh.Set(float64(time.Now().Unix()))
}

// commandDecls names the commands this plugin serves and states what each
// answer holds (pkg/plugin/rpc/types.go, CommandDecl).
//
// It has two readers and MUST stay one function. init() puts it on the
// registry.Registration, which anything linking the composition root reads
// without starting an engine, and the runner sends it in the Stage 1
// registration message, which a running daemon reads.
func commandDecls() []sdk.CommandDecl {
	return []sdk.CommandDecl{
		{Name: "show bgp irr", ShortHelp: "Show IRR filter status per ASN"},
		{Name: "show bgp irr prefix", ShortHelp: "Show IRR-resolved prefixes for a peer", Args: []string{"<peer>"}},
		{Name: "show bgp irr check", ShortHelp: "Check if a prefix is accepted by IRR filter", Args: []string{"<peer>", "<prefix>"}},
		{Name: "update bgp irr all", ShortHelp: "Refresh all IRR prefix-lists immediately"},
		{Name: "update bgp irr asn", ShortHelp: "Refresh IRR prefix-list for a specific ASN", Args: []string{"<asn>"}},
		{Name: "update bgp irr as-set", ShortHelp: "Refresh IRR prefix-list for a specific AS-SET", Args: []string{"<as-set>"}},
	}
}

// asnFilterNamePattern covers the filter names this plugin answers, one per
// AS number: a decimal ASN from 1 to 4294967295 with no leading zero, the
// form extractASNFromFilter reads. AS 0 is refused: it is reserved (RFC 7607)
// and no peer's remote AS, and extractASNFromFilter answers 0 for a name it
// cannot read. The ten-digit alternatives bound the value to 32 bits,
// which a digit count alone cannot, so a reference such as
// bgp-filter-irr:4294967296 fails config load instead of rejecting every
// route at run time (D-10).
const asnFilterNamePattern = `[1-9][0-9]{0,8}` +
	`|[1-3][0-9]{9}` +
	`|4[01][0-9]{8}` +
	`|42[0-8][0-9]{7}` +
	`|429[0-3][0-9]{6}` +
	`|4294[0-8][0-9]{5}` +
	`|42949[0-5][0-9]{4}` +
	`|429496[0-6][0-9]{3}` +
	`|4294967[01][0-9]{2}` +
	`|42949672[0-8][0-9]` +
	`|429496729[0-5]`

// filterDecls is the plugin's Stage 1 filter declaration: one filter per AS
// number, on import, failing closed. It reads the announced and the withdrawn
// NLRI, and answers a rejected prefix as a prefix remove, which needs no set
// declared (docs/plugin-development/protocol.md, "The Sets of a filter-update
// answer").
func filterDecls() []sdk.FilterDecl {
	return []sdk.FilterDecl{{
		NamePattern: asnFilterNamePattern,
		Direction:   sdk.FilterImport,
		Reads:       []string{fieldPrefix, fieldWithdrawn},
		OnError:     sdk.OnErrorReject,
	}}
}
