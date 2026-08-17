// Design: docs/architecture/bgp/filter-irr.md -- IRR prefix-list filter plugin entry point

package filter_irr

import (
	"context"
	"errors"
	"net"
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
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

var errFilterIrrInvalidBgpConfigJson = errors.New("filter-irr: invalid bgp config JSON")

const (
	// perASNRefreshTimeout bounds a single ASN's IRR resolution on the periodic
	// and manual ("update bgp irr ...") refresh paths, where no startup barrier
	// is waiting.
	perASNRefreshTimeout = 30 * time.Second

	// firstResolveWait bounds how long handleFilterUpdate blocks an IRR-filtered
	// UPDATE that arrives before the ASN's first background resolution has
	// finished. It closes the startup-resolution race (the UPDATE waits for the
	// list instead of getting a spurious "no-prefix-list" reject) without ever
	// gating ze startup: configure returns immediately and only this one filtered
	// UPDATE waits. It is comfortably above a typical IRR round-trip yet short
	// enough that a genuinely-unreachable IRR server fails the UPDATE closed
	// promptly rather than hanging the route's processing.
	firstResolveWait = 5 * time.Second
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
	plugin      *sdk.Plugin
	prefixStore *store.PrefixStore

	mu          sync.RWMutex
	byASN       map[uint32]*asnState
	config      *irrConfig
	nextRefresh time.Time
	lastRefresh time.Time

	stopCh      chan struct{}
	refreshStop chan struct{}
	refreshing  atomic.Bool
}

func runFilterIRR(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-filter-irr", conn)
	defer func() { _ = p.Close() }()

	plug := &irrPlugin{
		plugin: p,
		byASN:  make(map[uint32]*asnState),
		stopCh: make(chan struct{}),
	}

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != "bgp" {
				continue
			}
			bgpCfg, ok := configjson.ParseBGPSubtree(section.Data)
			if !ok {
				return errFilterIrrInvalidBgpConfigJson
			}
			plug.handleConfigure(bgpCfg)
		}
		return nil
	})

	p.OnFilterUpdate(func(in *sdk.FilterUpdateInput) (*sdk.FilterUpdateOutput, error) {
		return plug.handleFilterUpdate(in), nil
	})

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
		return plug.handleCommand(command, args)
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	defer close(plug.stopCh)

	if err := p.Run(ctx, sdk.Registration{
		Commands: []sdk.CommandDecl{
			{Name: "show bgp irr", Description: "Show IRR filter status per ASN"},
			{Name: "show bgp irr prefix", Description: "Show IRR-resolved prefixes for a peer", Args: []string{"<peer>"}},
			{Name: "show bgp irr check", Description: "Check if a prefix is accepted by IRR filter", Args: []string{"<peer>", "<prefix>"}},
			{Name: "update bgp irr all", Description: "Refresh all IRR prefix-lists immediately"},
			{Name: "update bgp irr asn", Description: "Refresh IRR prefix-list for a specific ASN", Args: []string{"<asn>"}},
			{Name: "update bgp irr as-set", Description: "Refresh IRR prefix-list for a specific AS-SET", Args: []string{"<as-set>"}},
		},
		WantsConfig: []string{"bgp"},
	}); err != nil {
		logger().Error("filter-irr plugin failed", "error", err)
		return 1
	}
	return 0
}

func (plug *irrPlugin) handleConfigure(bgpCfg map[string]any) {
	cfg := parseIRRConfig(bgpCfg)

	irrClient := irr.NewIRR(cfg.Server)
	if cfg.SourceAddress != "" {
		irrClient.SetSourceAddress(cfg.SourceAddress)
	}
	ps := store.New(irrClient, peeringdb.NewPeeringDB(cfg.PeeringDBURL), cacheStorePath())
	if err := ps.Open(); err != nil {
		logger().Warn("irr: prefix store open failed", "error", err)
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
	plug.byASN = newByASN
	plug.prefixStore = ps
	plug.config = cfg
	if plug.refreshStop != nil {
		close(plug.refreshStop)
	}
	plug.refreshStop = make(chan struct{})
	refreshStop := plug.refreshStop
	plug.mu.Unlock()

	plug.loadFromStore()

	// Resolve every enrolled ASN ONCE in the background, NOT inline. configure
	// runs inside the plugin startup handshake, gated by the engine's stage
	// barrier; a synchronous first resolution here would make ze-build startup depend
	// on reaching the IRR server (default RADB), so an unreachable or slow IRR
	// server would fail the configure stage and bring the whole BGP plugin set
	// down -- even for a config that has BGP peers but no IRR import filter.
	// Startup must never depend on external IRR reachability, so the first
	// resolution is detached. The startup-resolution race (an UPDATE arriving for
	// an IRR-filtered peer before this finishes) is closed at the filter layer:
	// handleFilterUpdate waits (bounded) on each ASN's firstDone signal, which
	// refreshAll closes per ASN below. The periodic refreshLoop keeps the list
	// fresh thereafter.
	go plug.initialResolve()

	go plug.refreshLoop(cfg.RefreshInterval, refreshStop)

	// "peers" counts the peers ENROLLED for IRR resolution, not every BGP peer:
	// an operator reading this line is asking how much IRR work was set up, and
	// the config's total peer count would answer a different question.
	enrolledPeers := 0
	for _, st := range newByASN {
		enrolledPeers += len(st.peerAddrs)
	}
	logger().Debug("configured", "peers", enrolledPeers, "asns", len(newByASN), "server", cfg.Server)
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
	// prefix-list -- or while only a stale cached list (loaded from the store) is
	// present. Until the first NETWORK resolution completes (firstDone closed),
	// wait (bounded) for it instead of evaluating against nothing or against stale
	// data; the background refresh closes firstDone on completion and re-reads the
	// now-fresh list below. On timeout (IRR slow/unreachable) we fall through with
	// whatever list we have -- the cached fallback if any, else the unchanged
	// fail-closed reject. This blocks only this one IRR-filtered UPDATE; peers
	// without an IRR import never reach here, and startup is never gated.
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

	if list == nil || len(list.entries) == 0 {
		logger().Info("irr filter reject", "filter", in.Filter, "peer", in.Peer, "reason", "no-prefix-list")
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}

	nlriField := extractNLRIField(in.Update)
	partition := list.partitionUpdate(nlriField)

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

	// Every prefix out of list: reject the whole update.
	if len(partition.accepted) == 0 {
		logger().Info("irr filter reject", "filter", in.Filter, "peer", in.Peer, "nlri", nlriField)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}

	// Every prefix in list: accept unmodified.
	if len(partition.rejected) == 0 {
		logger().Info("irr filter accept", "filter", in.Filter, "peer", in.Peer, "nlri", nlriField)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}
	}

	// Mixed: keep the in-list prefixes, drop the unauthorized ones via a modify
	// delta carrying only the accepted subset.
	delta := buildModifyDelta(partition)
	logger().Info("irr filter modify", "filter", in.Filter, "peer", in.Peer,
		"accepted", len(partition.accepted), "rejected", len(partition.rejected), "nlri", nlriField)
	return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Update: delta}
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
// refreshes use the per-ASN timeout via refreshASN.
func (plug *irrPlugin) refreshASNCtx(ctx context.Context, asn uint32) {
	plug.mu.RLock()
	st := plug.byASN[asn]
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

	// The shared store owns AS-SET discovery (PeeringDB), the IRR query, and
	// zefs persistence. The ASN is the stable identity/key; the configured (or
	// previously-discovered) AS-SET is passed as a hint.
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
