// Design: docs/architecture/bgp/filter-irr.md -- IRR prefix-list filter plugin entry point

package filter_irr

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
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

	// firstResolveWait bounds how long handleFilterUpdate blocks an IRR-filtered
	// UPDATE that arrives before the ASN's first background resolution has
	// finished. It closes the startup-resolution race (the UPDATE waits for the
	// list instead of getting a spurious "no-prefix-list" reject) without ever
	// gating ze startup: configure returns immediately and only this one filtered
	// UPDATE waits. It is comfortably above a typical IRR round-trip yet short
	// enough that a genuinely-unreachable IRR server fails the UPDATE closed
	// promptly rather than hanging the route's processing. It sits one second
	// below rpc.FilterUpdateTimeout, the reactor's deadline for this call: the
	// verdict given when the wait expires, the fail-closed reject, only counts if it reaches the reactor before that deadline rejects
	// the UPDATE on its own.
	firstResolveWait = rpc.FilterUpdateTimeout - time.Second
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
	asn       uint32
	asSet     string
	list      *irrPrefixList
	lastOK    time.Time
	lastErr   string
	v4Count   int
	v6Count   int
	peerAddrs []string

	// firstDone is closed exactly once, by signalFirstResolved, when this ASN's
	// FIRST resolution attempt completes (whether it succeeded, failed, or found
	// nothing). A filter UPDATE that arrives while list is still nil waits on
	// this channel (bounded) before deciding -- so the spurious "no-prefix-list"
	// reject of the startup-resolution race becomes a short wait, while a
	// genuinely-empty list after resolution still fails closed. closeOnce guards
	// the close so the periodic and manual refreshes that re-resolve the same ASN
	// never close it twice.
	firstDone chan struct{}
	closeOnce sync.Once
}

// newASNState builds an enrolled ASN's state with its first-resolution signal
// armed (open). Every ASN the configure path enrolls goes through here so
// handleFilterUpdate always has a non-nil channel to wait on.
func newASNState(asn uint32, asSet string) *asnState {
	return &asnState{asn: asn, asSet: asSet, firstDone: make(chan struct{})}
}

// signalFirstResolved closes firstDone the first time the ASN's resolution
// completes; later refreshes are no-ops. Safe when firstDone is nil (test
// states constructed as literals): the close is simply skipped.
func (st *asnState) signalFirstResolved() {
	if st == nil || st.firstDone == nil {
		return
	}
	st.closeOnce.Do(func() { close(st.firstDone) })
}

// isClosed reports whether ch has been closed, without blocking. Used to skip
// the first-resolution wait in the steady state (the common case once the first
// resolution finished long ago).
func isClosed(ch chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
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

	mu          sync.RWMutex
	byASN       map[uint32]*asnState
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

	p.OnFilterUpdate(func(in *sdk.FilterUpdateInput) (*sdk.FilterUpdateOutput, error) {
		return plug.handleFilterUpdate(in), nil
	})

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

	if err := p.Run(ctx, &sdk.Registration{
		Commands:    commandDecls(),
		WantsConfig: []string{configRootBGP},
	}); err != nil {
		logger().Error("filter-irr plugin failed", "error", err)
		return 1
	}
	return 0
}

// bgpSubtreeFromSections returns the bgp subtree a configure or verify
// delivery carries, nil when it carries none, and an error when the subtree is
// not valid JSON.
func bgpSubtreeFromSections(sections []sdk.ConfigSection) (map[string]any, error) {
	var bgpCfg map[string]any
	for _, section := range sections {
		if section.Root != configRootBGP {
			continue
		}
		parsed, ok := configjson.ParseBGPSubtree(section.Data)
		if !ok {
			return nil, errFilterIrrInvalidBgpConfigJson
		}
		bgpCfg = parsed
	}
	return bgpCfg, nil
}

// handleConfigure makes bgpCfg the filter's config, at boot and on every
// applied or rolled-back reload. The new per-ASN state is published under
// plug.mu in one swap, so a concurrent filter UPDATE reads the old state or
// the new one, never a mix.
func (plug *irrPlugin) handleConfigure(bgpCfg map[string]any) {
	cfg := parseIRRConfig(bgpCfg)

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
		// "import [ bgp-filter-irr:<asn> ]" reference has no `show bgp irr`
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
		if peer.ASSet != "" && st.asSet == "" {
			st.asSet = peer.ASSet
		}
	}

	// All mutable plugin fields are published under plug.mu so a concurrent
	// `show bgp irr` / refresh never reads a half-updated configure.
	plug.mu.Lock()
	for asn, newSt := range newByASN {
		oldSt, ok := plug.byASN[asn]
		if !ok {
			continue
		}
		// A list resolved for another AS-SET answers a question the operator
		// no longer asks. Carrying it over would filter against the old set,
		// with firstDone already signaled, until the next refresh: the edit
		// would be accepted and not enforced.
		if newSt.asSet != "" && newSt.asSet != oldSt.asSet {
			continue
		}
		newSt.list = oldSt.list
		newSt.lastOK = oldSt.lastOK
		newSt.lastErr = oldSt.lastErr
		newSt.v4Count = oldSt.v4Count
		newSt.v6Count = oldSt.v6Count
		if newSt.asSet == "" && oldSt.asSet != "" {
			newSt.asSet = oldSt.asSet
		}
		// A reconfigure that carries over a resolved list has already cleared the
		// first-resolution window for this ASN: a filter UPDATE must not wait for
		// the upcoming re-resolution. Arm the new state as already-resolved.
		if newSt.list != nil {
			newSt.signalFirstResolved()
		}
	}
	oldByASN := plug.byASN
	plug.byASN = newByASN
	plug.prefixStore = ps
	plug.config = cfg
	if plug.refreshStop != nil {
		close(plug.refreshStop)
	}
	plug.refreshStop = make(chan struct{})
	refreshStop := plug.refreshStop
	plug.mu.Unlock()

	forgetUnconfigured(ps, oldByASN, newByASN)

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
			"action", "add `capability { route-refresh enable }` to this peer, or to its group, and enable Route Refresh on the peer's side too; the capability is negotiated when the session next comes up")
	}

	// Resolve every enrolled ASN ONCE in the background, NOT inline. configure
	// runs inside the plugin startup handshake, gated by the engine's stage
	// barrier. A synchronous first resolution here would make ze's startup
	// depend on reaching the IRR server (default RADB), so an unreachable or slow
	// server would fail the configure stage and bring the whole BGP plugin set
	// down -- even for a config that has BGP peers but no IRR import filter.
	// Startup must never depend on external IRR reachability, so the first
	// resolution is detached. The startup-resolution race (an UPDATE arriving for
	// an IRR-filtered peer before this finishes) is closed at the filter layer:
	// handleFilterUpdate waits (bounded) on each ASN's firstDone signal, which
	// refreshAll closes per ASN below. The periodic refreshLoop keeps the list
	// fresh thereafter.
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
// asks for: an ASN no longer enrolled, and an ASN whose configured as-set is not
// the one its entry was resolved for. store.commit keeps a cached family only
// for the same AS-SET, so an old-set entry cannot hand its prefixes to the new
// set; purging it keeps the store to the sets the config names.
// This is the store's half of "a changed as-set starts from no list";
// handleConfigure's carry-over rule is the plugin's half.
func forgetUnconfigured(ps *store.PrefixStore, oldByASN, newByASN map[uint32]*asnState) {
	for asn := range oldByASN {
		if _, kept := newByASN[asn]; !kept {
			ps.Purge(asnName(asn))
		}
	}
	for asn, st := range newByASN {
		if st.asSet == "" {
			continue // the as-set is discovered, so no configured one to differ from
		}
		entry := ps.Get(asnName(asn))
		if entry == nil {
			continue
		}
		if entry.ASSet != st.asSet {
			ps.Purge(asnName(asn))
		}
	}
}

// initialResolve performs the first background resolution of every enrolled
// ASN. It runs detached from the configure path (so startup never blocks on IRR
// network I/O) and, unlike refreshAll, does NOT honor the refreshing CAS guard:
// it must always do the work, because each ASN's firstDone signal (which
// handleFilterUpdate waits on) is only closed by a completed resolution attempt.
// signalFirstResolved is also called as a backstop for every enrolled ASN, so an
// ASN whose live state vanished mid-refresh (e.g. a concurrent reconfigure) can
// never leave a filter UPDATE blocked for the full wait timeout.
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

	// Backstop: refreshASN signals firstDone on its own completion paths, but a
	// reconfigure between the snapshot above and now may have replaced the state;
	// signal whatever is live so no ASN is left with an open firstDone.
	plug.mu.RLock()
	for _, st := range plug.byASN {
		st.signalFirstResolved()
	}
	plug.mu.RUnlock()
}

func (plug *irrPlugin) handleFilterUpdate(in *sdk.FilterUpdateInput) *sdk.FilterUpdateOutput {
	asn := extractASNFromFilter(in.Filter)
	if asn == 0 {
		logger().Warn("irr filter: cannot extract ASN from filter name", "filter", in.Filter, "peer", in.Peer)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}

	plug.mu.RLock()
	st := plug.byASN[asn]
	var list *irrPrefixList
	var firstDone chan struct{}
	if st != nil {
		list = st.list
		firstDone = st.firstDone
	}
	plug.mu.RUnlock()

	// Startup-resolution race: the first IRR resolution runs in the background
	// (configure never blocks on IRR network I/O), so the first UPDATE for an
	// IRR-filtered peer can arrive before that resolution has produced a
	// prefix-list. The list is held in memory only, so after a restart every
	// enrolled ASN starts here with no list. Until the first resolution completes
	// (firstDone closed), wait (bounded) for it instead of evaluating against
	// nothing; the background refresh closes firstDone on completion and the
	// now-resolved list is re-read below. On timeout (IRR slow/unreachable) we
	// fall through with no list, and the fail-closed reject holds each announced
	// prefix for re-admission by ROUTE-REFRESH once the list arrives. This blocks
	// only this one IRR-filtered UPDATE; peers without an IRR import never reach
	// here, and startup is never gated.
	if firstDone != nil && !isClosed(firstDone) {
		select {
		case <-firstDone:
		case <-time.After(firstResolveWait):
		case <-plug.stopCh:
		}
		plug.mu.RLock()
		if st = plug.byASN[asn]; st != nil {
			list = st.list
		}
		plug.mu.RUnlock()
	}

	nlriField := extractNLRIField(in.Update)
	var partition partitionResult
	var out *sdk.FilterUpdateOutput
	if list == nil || len(list.entries) == 0 {
		// Every announced prefix is rejected: the list that may authorize it
		// is not here yet. The worker holds each one for when it arrives.
		var none irrPrefixList
		partition = none.partitionUpdate(nlriField)
		out = judgeUnlisted(in, nlriField, &partition)
	} else {
		partition = list.partitionUpdate(nlriField)
		out = judgeListed(in, nlriField, &partition)
	}
	// A whole reject drops every announced prefix, the admitted ones too, so
	// every one is held for the next pass to ask for again.
	plug.trackVerdict(in.Peer, asn, st != nil, &partition, out.Action == sdk.FilterReject)
	return out
}

// judgeUnlisted decides an UPDATE for an ASN that has no list yet: every
// announced prefix is rejected, and the withdrawals still go through.
func judgeUnlisted(in *sdk.FilterUpdateInput, nlriField string, partition *partitionResult) *sdk.FilterUpdateOutput {
	if out, kept := keepWithdrawals(partition); kept {
		logger().Info("irr filter keeps the withdrawals", "filter", in.Filter, "peer", in.Peer, "nlri", nlriField,
			"verdict", out.Action, "reason", "no-prefix-list")
		return out
	}
	logger().Info("irr filter reject", "filter", in.Filter, "peer", in.Peer, "reason", "no-prefix-list")
	return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
}

// judgeListed decides an UPDATE against the ASN's list.
func judgeListed(in *sdk.FilterUpdateInput, nlriField string, partition *partitionResult) *sdk.FilterUpdateOutput {
	// No prefixes (attrs-only update): accept -- nothing reachable to filter.
	if len(partition.accepted) == 0 && len(partition.rejected) == 0 && !partition.hadParseError {
		logger().Info("irr filter accept", "filter", in.Filter, "peer", in.Peer, "nlri", nlriField)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}
	}

	// Malformed prefix in the text protocol -> fail-closed.
	if partition.hadParseError {
		logger().Info("irr filter reject", "filter", in.Filter, "peer", in.Peer, "nlri", nlriField, "reason", "parse-error")
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}

	// Every announced prefix out of list: drop the announcements, keep the
	// withdrawals when the engine can, else reject the whole update.
	if len(partition.accepted) == 0 {
		if out, kept := keepWithdrawals(partition); kept {
			logger().Info("irr filter keeps the withdrawals", "filter", in.Filter, "peer", in.Peer, "nlri", nlriField,
				"verdict", out.Action, "rejected", len(partition.rejected))
			return out
		}
		logger().Info("irr filter reject", "filter", in.Filter, "peer", in.Peer, "nlri", nlriField)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}

	// Every prefix in list: accept unmodified.
	if len(partition.rejected) == 0 {
		logger().Info("irr filter accept", "filter", in.Filter, "peer", in.Peer, "nlri", nlriField)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}
	}

	// Mixed, with a rejected prefix the engine cannot drop: it rewrites the
	// legacy NLRI and the MP_REACH_NLRI of a CIDR family, and refuses a changed
	// block of any other family, so a rejected FlowSpec destination, say, has no
	// modify that drops it. Fail closed.
	if !partition.rewritable() {
		logger().Info("irr filter reject", "filter", in.Filter, "peer", in.Peer, "nlri", nlriField,
			"reason", "rejected-prefix-in-unrewritable-block")
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}

	// Mixed: keep the in-list prefixes, drop the unauthorized ones via a modify
	// delta carrying only the accepted subset.
	delta := buildModifyDelta(*partition)
	logger().Info("irr filter modify", "filter", in.Filter, "peer", in.Peer,
		"accepted", len(partition.accepted), "rejected", len(partition.rejected), "nlri", nlriField)
	return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Update: delta}
}

// RFC 4271 Section 9: "If the UPDATE message contains a non-empty WITHDRAWN
// ROUTES field, the previously advertised routes, whose destinations
// (expressed as IP prefixes) are contained in this field, SHALL be removed
// from the Adj-RIB-In."
//
// keepWithdrawals answers an UPDATE that announces no prefix the filter
// accepts, and reports false when it carries no withdrawal to keep or the
// engine cannot keep it. The engine drops a rejected UPDATE whole, its
// withdrawals with it. A withdrawal alone is accepted. Beside rejected
// announcements it is kept by a modify whose delta holds the withdraw blocks
// alone, which the engine reads as an empty legacy NLRI section and an
// MP_REACH_NLRI with no prefix left, which it removes (reactor
// filter_nlri_delta.go, sectionNLRIDelta). A rejected prefix in a block the
// engine cannot rewrite, an MP_REACH_NLRI of a non-CIDR family (rewritable),
// keeps the UPDATE a whole reject. Callers MUST have ruled out a parse error
// and an accepted prefix first.
func keepWithdrawals(partition *partitionResult) (*sdk.FilterUpdateOutput, bool) {
	if partition.hadParseError {
		return nil, false
	}
	if !slices.ContainsFunc(partition.blocks, func(block nlriBlock) bool { return block.withdraws() }) {
		return nil, false
	}
	if len(partition.rejected) == 0 {
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, true
	}
	if !partition.rewritable() {
		return nil, false
	}
	return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Update: buildModifyDelta(*partition)}, true
}

// trackVerdict tells the re-admission worker what one UPDATE's verdict did to
// the prefixes it holds. A withdrawal drops them, an accepted prefix is no
// longer held, and a rejected one is held until its ASN's list authorizes it.
// rejectWhole marks an UPDATE the filter rejected whole, whose admitted
// prefixes were dropped too and are held with the rest. Only an ASN this
// config enrolls holds anything: no refresh of any other ASN ever runs, so its
// hold could only leak. An UPDATE the filter could not parse holds nothing,
// because nothing in it can be evaluated again.
func (plug *irrPlugin) trackVerdict(peerAddr string, asn uint32, enrolled bool, partition *partitionResult, rejectWhole bool) {
	if plug.readmit == nil {
		return
	}
	if partition.hadParseError {
		return
	}
	// The path almost every UPDATE takes: nothing to hold and nothing held. The
	// count is atomic, so this path takes no lock.
	holds := enrolled && (len(partition.rejected) > 0 || rejectWhole && len(partition.accepted) > 0)
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
	for i := range partition.blocks {
		plug.readmit.track(peer, asn, enrolled, &partition.blocks[i], rejectWhole)
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

// refreshASNCtx is refreshASN with a caller-supplied deadline. Every completion
// path signals the live ASN state's firstDone so handleFilterUpdate's bounded
// wait is released as soon as the first resolution attempt finishes -- whether
// it succeeded, failed, or could not run (no store). periodic and manual
// refreshes use the per-ASN timeout via refreshASN. The list lives in memory
// only: an UPDATE waiting on firstDone waits on the IRR answer and nothing else.
func (plug *irrPlugin) refreshASNCtx(ctx context.Context, asn uint32) {
	plug.mu.RLock()
	st := plug.byASN[asn]
	captured := st
	ps := plug.prefixStore
	asSet := ""
	if st != nil {
		asSet = st.asSet
	}
	plug.mu.RUnlock()
	if st == nil || ps == nil {
		// No live state, or no store to resolve against: the first-resolution
		// attempt is over for this ASN (it will fail closed). Release any waiter.
		st.signalFirstResolved()
		return
	}

	// The store owns AS-SET discovery (PeeringDB), the IRR query, and the
	// in-memory last-known-good. The ASN is the stable identity/key; the
	// configured (or previously-discovered) AS-SET is passed as a hint.
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
	if st != captured && st.asSet != asSet {
		// A reload changed this ASN's as-set while the lookup ran, so the answer
		// is for a set the config no longer names. Applying it would filter
		// against the old set, so drop it. The reload's own resolution answers
		// for the new set. A reload that kept the as-set also replaces st, and
		// its answer is still the right one, so the as-set decides, not the
		// pointer alone. The entry this lookup committed stays in the store
		// until the new set's first answer replaces it: store.commit never
		// merges another AS-SET's family, so it cannot leak into that answer,
		// and purging it here could delete the new set's entry when the
		// reload's lookup committed first.
		liveASSet := st.asSet
		plug.mu.Unlock()
		logger().Debug("irr: dropped a lookup a reload superseded", "asn", asn, "as-set", asSet, "live-as-set", liveASSet)
		return
	}
	if entry != nil {
		// Record the resolved AS-SET even on a failed lookup, so `show bgp irr`
		// reflects the fallback name and later refreshes reuse it.
		st.asSet = entry.ASSet
	}
	if err != nil {
		st.lastErr = err.Error()
		st.signalFirstResolved()
		plug.mu.Unlock()
		logger().Warn("irr: lookup failed", "asn", asn, "as-set", asSet, "error", err)
		incRefreshOutcome("error")
		return
	}

	pl := entry.PrefixList()
	st.list = &irrPrefixList{entries: prefixListFromIRR(pl)}
	st.lastOK = now
	st.lastErr = ""
	st.v4Count = len(pl.IPv4)
	st.v6Count = len(pl.IPv6)
	st.signalFirstResolved()
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

// asnName renders an ASN as its canonical "AS<n>" name, the PrefixStore key the
// BGP filter uses as a stable per-ASN identity.
func asnName(asn uint32) string {
	var tb textbuf.Buffer
	return tb.Str("AS").Uint32(asn).String()
}

const maxPrefixEntries = 500_000

func prefixListFromIRR(pl irr.PrefixList) []prefixEntry {
	total := len(pl.IPv4) + len(pl.IPv6)
	if total > maxPrefixEntries {
		logger().Warn("irr: prefix list exceeds cap, truncating", "total", total, "cap", maxPrefixEntries)
		total = maxPrefixEntries
	}
	entries := make([]prefixEntry, 0, total)
	for _, p := range pl.IPv4 {
		if len(entries) >= maxPrefixEntries {
			break
		}
		entries = append(entries, prefixEntry{
			prefix: p,
			ge:     uint8(p.Bits()),
			le:     32,
		})
	}
	for _, p := range pl.IPv6 {
		if len(entries) >= maxPrefixEntries {
			break
		}
		entries = append(entries, prefixEntry{
			prefix: p,
			ge:     uint8(p.Bits()),
			le:     128,
		})
	}
	return entries
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
