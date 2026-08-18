// Design: docs/architecture/firewall/firewall-irr.md -- firewall IRR plugin entry point
// Related: doctor.go -- the ze doctor check over the same references verifyRefs guards
//
// Package irr implements IRR-based prefix-list filtering for firewall rules.
// It resolves ASN and AS-SET references to prefix lists via the shared
// PrefixStore, populates nftables interval sets, and registers them through
// the firewall table registry for merged apply.
//
// A refresh that learns nothing never replaces cached prefixes: the guard is
// PrefixStore.Refresh in internal/component/resolve/irr/store, which returns
// store.ErrNoPrefixes and keeps the last known good data. This package counts
// that outcome apart from a success, logs it, and reports it as a stale entry.
package irr

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/resolve/irr"
	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/internal/component/resolve/peeringdb"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

var logger = slogutil.LazyLogger("firewall.irr")

type irrMetrics struct {
	prefixesCached  metrics.Gauge
	refreshOutcomes metrics.CounterVec
	lastRefresh     metrics.Gauge
	dataAge         metrics.Gauge
}

var irrMetricsPtr atomic.Pointer[irrMetrics]

func setMetricsRegistry(reg metrics.Registry) {
	m := &irrMetrics{
		prefixesCached:  reg.Gauge("ze_firewall_irr_prefixes_cached", "Total IRR-resolved prefixes cached for firewall filtering."),
		refreshOutcomes: reg.CounterVec("ze_firewall_irr_refresh_outcomes_total", "Firewall IRR refresh outcomes.", []string{"result"}),
		lastRefresh:     reg.Gauge("ze_firewall_irr_last_refresh_timestamp", "Unix timestamp of last firewall IRR refresh that learned prefixes."),
		dataAge:         reg.Gauge("ze_firewall_irr_data_age_seconds", "Age in seconds of the oldest IRR prefix data the firewall is enforcing."),
	}
	irrMetricsPtr.Store(m)
}

type irrPlugin struct {
	plugin      *sdk.Plugin
	prefixStore *store.PrefixStore

	mu          sync.RWMutex
	config      *irrConfig
	lastRefresh time.Time

	stopCh      chan struct{}
	refreshStop chan struct{}
	refreshing  atomic.Bool
}

func runFirewallIRR(conn net.Conn) int {
	p := sdk.NewWithConn("firewall-irr", conn)
	defer func() { _ = p.Close() }()

	plug := &irrPlugin{
		plugin: p,
		stopCh: make(chan struct{}),
	}

	// pendingCfg carries the config verified by OnConfigVerify into
	// OnConfigApply, which receives only a diff. Without it a reload would
	// apply the config the daemon started with: the refs, the interface
	// bindings, the server and the refresh interval all live here.
	var pendingCfg *irrConfig

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		return plug.configure(parseIRRConfig(sections))
	})

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		cfg := parseIRRConfig(sections)
		refs := cfg.allRefs()
		if len(refs) == 0 {
			pendingCfg = cfg
			return nil
		}
		ps := plug.getPrefixStore()
		if ps == nil {
			ps = store.New(
				irr.NewIRR(defaultServer),
				nil,
				cacheStorePath(),
			)
			if err := ps.Open(); err != nil {
				return fmt.Errorf("firewall-irr: cache not available: %w", err)
			}
		}
		if err := verifyRefs(ps, refs); err != nil {
			return err
		}
		pendingCfg = cfg
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		cfg := pendingCfg
		pendingCfg = nil
		if cfg == nil {
			// No verify ran for this transaction, so nothing new was
			// approved: reconcile what is already configured.
			return plug.applyTables()
		}
		return plug.configure(cfg)
	})

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
		return plug.handleCommand(command, args)
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	defer close(plug.stopCh)

	if err := p.Run(ctx, sdk.Registration{
		Commands: []sdk.CommandDecl{
			{Name: "show firewall irr", Description: "Show IRR filter status for all cached entries"},
			{Name: "show firewall irr prefix", Description: "Show IRR-resolved prefixes for a cached entry", Args: []string{"<asn-or-as-set>"}},
			{Name: "update firewall irr all", Description: "Refresh all cached IRR prefix-lists"},
			{Name: "update firewall irr asn", Description: "Fetch/refresh IRR prefix-list for an ASN", Args: []string{"<asn>"}},
			{Name: "update firewall irr as-set", Description: "Fetch/refresh IRR prefix-list for an AS-SET", Args: []string{"<as-set>"}},
			{Name: "clear firewall irr asn", Description: "Remove the cached IRR prefix-list for an ASN", Args: []string{"<asn>"}},
			{Name: "clear firewall irr as-set", Description: "Remove the cached IRR prefix-list for an AS-SET", Args: []string{"<as-set>"}},
		},
		WantsConfig: []string{"firewall"},
	}); err != nil {
		logger().Error("firewall-irr plugin failed", "error", err)
		return 1
	}
	return 0
}

// configure installs cfg as the plugin's running configuration: it opens a
// prefix store against the configured IRR server, replaces the refresh loop,
// and reconciles the tables the config asks for. Both OnConfigure and
// OnConfigApply call it, so a reload changes what a restart would have changed.
// An apply that skipped it left the plugin serving the config the daemon
// started with, and a term added by a commit never reached the registry.
func (plug *irrPlugin) configure(cfg *irrConfig) error {
	ps := store.New(
		irr.NewIRR(cfg.Server),
		peeringdb.NewPeeringDB(cfg.PeeringDBURL),
		cacheStorePath(),
	)
	if err := ps.Open(); err != nil {
		logger().Warn("firewall-irr: prefix store open failed", "error", err)
	}

	plug.mu.Lock()
	plug.prefixStore = ps
	plug.config = cfg
	if plug.refreshStop != nil {
		close(plug.refreshStop)
	}
	plug.refreshStop = make(chan struct{})
	refreshStop := plug.refreshStop
	plug.mu.Unlock()

	// A reference with no prefixes is refused at verify, and a commit is the
	// only path that runs verify: a daemon start reads its config through
	// OnConfigure alone. Report each one here so the operator sees the same
	// entry name and the same fetch command at startup that a commit would
	// have refused with, rather than a firewall that quietly enforces nothing.
	warnUncachedRefs(ps, cfg.allRefs())

	if err := plug.applyTables(); err != nil {
		logger().Warn("firewall-irr: apply failed", "error", err)
	}

	if cfg.RefreshInterval > 0 {
		go plug.refreshLoop(cfg.RefreshInterval, refreshStop)
	}

	logger().Debug("configured", "server", cfg.Server, "refresh-interval", cfg.RefreshInterval)
	return nil
}

// warnUncachedRefs logs the refusal message verifyRefs would have produced for
// every reference holding no prefixes. It reports all of them, where verifyRefs
// stops at the first: a commit needs one reason to refuse, and an operator
// reading a startup log needs the whole list.
func warnUncachedRefs(ps *store.PrefixStore, refs []irrRef) {
	for _, ref := range refs {
		entry := ps.Get(ref.Name)
		if entry != nil && !entry.PrefixList().Empty() {
			continue
		}
		logger().Warn(uncachedRefMessage(ref, entry == nil),
			"effect", "the rules naming it filter nothing until its prefixes are fetched")
	}
}

// verifyRefs refuses a config whose IRR references have no prefixes to enforce.
// An absent entry and an entry holding zero prefixes are both refused: neither
// can filter anything, and committing either leaves the operator believing a
// filter is in place. The message names the command that fetches the data.
func verifyRefs(ps *store.PrefixStore, refs []irrRef) error {
	for _, ref := range refs {
		entry := ps.Get(ref.Name)
		if entry != nil && !entry.PrefixList().Empty() {
			continue
		}
		return errors.New(uncachedRefMessage(ref, entry == nil))
	}
	return nil
}

// uncachedRefMessage names the entry that has no prefixes and the command that
// fetches it. absent separates "nothing is cached under this name" from "the
// cached entry is empty", because the two send an operator to different places.
func uncachedRefMessage(ref irrRef, absent bool) string {
	var tb textbuf.Buffer
	if absent {
		tb.Str("firewall irr: no cached prefix data for ")
	} else {
		tb.Str("firewall irr: cached entry holds no prefixes for ")
	}
	if ref.IsASSet {
		tb.Str("as-set ").Str(ref.Name)
		tb.Str("; run 'update firewall irr as-set ").Str(ref.Name).Str("' first")
	} else {
		tb.Str(ref.Name)
		tb.Str("; run 'update firewall irr asn ").Str(bareASN(ref.Name)).Str("' first")
	}
	return tb.String()
}

func (plug *irrPlugin) getPrefixStore() *store.PrefixStore {
	plug.mu.RLock()
	defer plug.mu.RUnlock()
	return plug.prefixStore
}

func (plug *irrPlugin) applyTables() error {
	ps := plug.getPrefixStore()
	if ps == nil {
		firewall.RegisterTables("firewall-irr", nil)
		return nil
	}

	plug.mu.RLock()
	cfg := plug.config
	plug.mu.RUnlock()

	termRefs := cfg.termRefs()
	ifaceBindings := cfg.ifaceBindings

	if len(termRefs) == 0 && len(ifaceBindings) == 0 {
		firewall.RegisterTables("firewall-irr", nil)
		return nil
	}

	tables := buildIRRTables(ps, termRefs)
	tables = append(tables, buildIfaceTables(ps, ifaceBindings)...)
	firewall.RegisterTables("firewall-irr", tables)
	if err := firewall.ApplyAll(); err != nil {
		return fmt.Errorf("firewall-irr: apply: %w", err)
	}
	updateMetricsGauges(ps, cfg.allRefs())
	return nil
}

func (plug *irrPlugin) refreshLoop(intervalSec uint32, stop chan struct{}) {
	interval := time.Duration(intervalSec) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			plug.refreshAll()
		case <-stop:
			return
		case <-plug.stopCh:
			return
		}
	}
}

func (plug *irrPlugin) refreshAll() {
	if !plug.refreshing.CompareAndSwap(false, true) {
		return
	}
	defer plug.refreshing.Store(false)
	_ = plug.refreshAllNow()
}

func (plug *irrPlugin) refreshAllNow() error {
	ps := plug.getPrefixStore()
	if ps == nil {
		return nil
	}

	plug.mu.RLock()
	cfg := plug.config
	plug.mu.RUnlock()
	if cfg == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var firstErr error
	var learned bool
	for _, ref := range cfg.allRefs() {
		entry, err := ps.Refresh(ctx, ref.Name, "")
		switch {
		case errors.Is(err, store.ErrNoPrefixes):
			logEmptyRefresh(ref.Name, entry)
			incRefreshOutcome("empty")
		case err != nil:
			logger().Warn("firewall-irr: refresh failed, keeping the cached prefixes", "name", ref.Name, "error", err)
			incRefreshOutcome("error")
		default:
			incRefreshOutcome("success")
			logRefreshLearned(ref.Name, entry)
			learned = true
			continue
		}
		if firstErr == nil {
			firstErr = err
		}
	}

	if learned {
		markRefreshLearned()
	}
	if firstErr == nil {
		plug.mu.Lock()
		plug.lastRefresh = time.Now()
		plug.mu.Unlock()
	}

	if err := plug.applyTables(); err != nil {
		logger().Warn("firewall-irr: apply after refresh failed", "error", err)
	}
	return firstErr
}

func (plug *irrPlugin) refreshName(name string) error {
	ps := plug.getPrefixStore()
	if ps == nil {
		return fmt.Errorf("firewall-irr: no prefix store")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	entry, err := ps.Refresh(ctx, name, "")
	if errors.Is(err, store.ErrNoPrefixes) {
		logEmptyRefresh(name, entry)
		incRefreshOutcome("empty")
		updateMetricsGauges(ps, plug.configRefs())
		return keptDataError(name, entry)
	}
	if err != nil {
		incRefreshOutcome("error")
		return err
	}
	incRefreshOutcome("success")
	logRefreshLearned(name, entry)
	markRefreshLearned()

	plug.mu.Lock()
	plug.lastRefresh = time.Now()
	plug.mu.Unlock()

	updateMetricsGauges(ps, plug.configRefs())
	return nil
}

// logRefreshLearned records a refresh that learned prefixes, with the counts it
// now enforces. Its counterpart logEmptyRefresh reports the opposite outcome, so
// a log carrying one and not the other says which refreshes worked.
func logRefreshLearned(name string, entry *store.CachedEntry) {
	v4, v6 := 0, 0
	if entry != nil {
		v4, v6 = len(entry.IPv4), len(entry.IPv6)
	}
	logger().Debug("firewall-irr: refreshed", "name", name, "ipv4", v4, "ipv6", v6)
}

// logEmptyRefresh names the AS-SET whose refresh learned nothing and says what
// is still being enforced in its place.
func logEmptyRefresh(name string, entry *store.CachedEntry) {
	kept := 0
	if entry != nil {
		kept = len(entry.IPv4) + len(entry.IPv6)
	}
	logger().Warn("firewall-irr: refresh learned no prefixes, keeping the cached ones",
		"name", name, "kept-prefixes", kept)
}

// keptDataError tells the operator what the refresh did instead of what it
// failed to do: which prefixes stay enforced, how old they are, and how to
// remove them when the AS-SET is gone upstream for good.
func keptDataError(name string, entry *store.CachedEntry) error {
	var tb textbuf.Buffer
	tb.Str("firewall irr: ").Str(name).Str(" returned no prefixes; ")
	if entry == nil || entry.PrefixList().Empty() {
		tb.Str("nothing is cached for it")
		return errors.New(tb.String())
	}
	tb.Str("keeping ").Int(int64(len(entry.IPv4) + len(entry.IPv6))).Str(" cached prefixes")
	if !entry.RefreshedAt.IsZero() {
		tb.Str(" learned ").Str(entry.RefreshedAt.Format(time.RFC3339))
	}
	tb.Str("; run 'clear firewall irr ")
	if isASSetName(name) {
		tb.Str("as-set ").Str(name)
	} else {
		tb.Str("asn ").Str(bareASN(name))
	}
	tb.Str("' to remove them")
	return errors.New(tb.String())
}

// isASSetName reports whether name is an AS-SET reference rather than a bare
// "AS<n>" identity. The store keys both, and the two take different verbs.
func isASSetName(name string) bool {
	return bareASN(name) == name
}

// bareASN strips the "AS" prefix from an "AS<n>" name, returning name unchanged
// when what follows is not a number.
func bareASN(name string) string {
	if len(name) <= 2 || (name[0] != 'A' && name[0] != 'a') || (name[1] != 'S' && name[1] != 's') {
		return name
	}
	num := name[2:]
	if _, err := strconv.ParseUint(num, 10, 32); err != nil {
		return name
	}
	return num
}

func (plug *irrPlugin) configRefs() []irrRef {
	plug.mu.RLock()
	defer plug.mu.RUnlock()
	if plug.config == nil {
		return nil
	}
	return plug.config.allRefs()
}

func incRefreshOutcome(result string) {
	if m := irrMetricsPtr.Load(); m != nil {
		m.refreshOutcomes.With(result).Inc()
	}
}

// updateMetricsGauges publishes how much data is enforced and how old the
// oldest of it is. It does NOT stamp lastRefresh: that gauge answers "when did
// a refresh last learn prefixes", and it runs after refreshes that learned
// nothing as well as after ones that did.
func updateMetricsGauges(ps *store.PrefixStore, refs []irrRef) {
	m := irrMetricsPtr.Load()
	if m == nil {
		return
	}
	total := 0
	var oldest time.Duration
	now := time.Now()
	for _, ref := range refs {
		entry := ps.Get(ref.Name)
		if entry == nil {
			continue
		}
		total += len(entry.IPv4) + len(entry.IPv6)
		if age := entryAge(entry, now); age > oldest {
			oldest = age
		}
	}
	m.prefixesCached.Set(float64(total))
	m.dataAge.Set(oldest.Seconds())
}

// entryAge is how long ago the entry's prefixes were learned. An entry with no
// learn timestamp (seeded, or migrated from the legacy cache) reports zero.
func entryAge(entry *store.CachedEntry, now time.Time) time.Duration {
	if entry.RefreshedAt.IsZero() {
		return 0
	}
	return now.Sub(entry.RefreshedAt)
}

// markRefreshLearned stamps the last-refresh gauge. Only a refresh that learned
// prefixes calls it: an answer carrying nothing must not read as a refresh that
// worked.
func markRefreshLearned() {
	if m := irrMetricsPtr.Load(); m != nil {
		m.lastRefresh.Set(float64(time.Now().Unix()))
	}
}

func extractIfaceRefs(root map[string]any) []irrRef {
	fw, ok := root["firewall"].(map[string]any)
	if !ok {
		return nil
	}
	irrBlock, ok := fw["irr"].(map[string]any)
	if !ok {
		return nil
	}
	ifaceMap, ok := irrBlock["interface"].(map[string]any)
	if !ok {
		return nil
	}
	var refs []irrRef
	for _, v := range ifaceMap {
		entry, ok := v.(map[string]any)
		if !ok {
			continue
		}
		asSet, ok := entry["source-as-set"].(string)
		if !ok || asSet == "" {
			continue
		}
		refs = append(refs, irrRef{Name: asSet, IsASSet: true, IsSrc: true})
	}
	return refs
}
