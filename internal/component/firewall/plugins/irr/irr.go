// Design: plan/learned/913-firewall-irr.md -- firewall IRR plugin entry point
//
// Package irr implements IRR-based prefix-list filtering for firewall rules.
// It resolves ASN and AS-SET references to prefix lists via the shared
// PrefixStore, populates nftables interval sets, and registers them through
// the firewall table registry for merged apply.
package irr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
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
}

var irrMetricsPtr atomic.Pointer[irrMetrics]

func setMetricsRegistry(reg metrics.Registry) {
	m := &irrMetrics{
		prefixesCached:  reg.Gauge("ze_firewall_irr_prefixes_cached", "Total IRR-resolved prefixes cached for firewall filtering."),
		refreshOutcomes: reg.CounterVec("ze_firewall_irr_refresh_outcomes_total", "Firewall IRR refresh outcomes.", []string{"result"}),
		lastRefresh:     reg.Gauge("ze_firewall_irr_last_refresh_timestamp", "Unix timestamp of last successful firewall IRR refresh."),
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

	var pendingRefs []irrRef

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg := parseIRRConfig(sections)
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

		if err := plug.applyTables(); err != nil {
			logger().Warn("firewall-irr: initial apply failed", "error", err)
		}

		if cfg.RefreshInterval > 0 {
			go plug.refreshLoop(cfg.RefreshInterval, refreshStop)
		}

		logger().Debug("configured", "server", cfg.Server, "refresh-interval", cfg.RefreshInterval)
		return nil
	})

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		refs := extractIRRRefs(sections)
		if len(refs) == 0 {
			pendingRefs = nil
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
		for _, ref := range refs {
			entry := ps.Get(ref.Name)
			if entry == nil {
				var tb textbuf.Buffer
				if ref.IsASSet {
					tb.Str("firewall irr: no cached prefix data for as-set ").Str(ref.Name)
					tb.Str("; run 'update firewall irr as-set ").Str(ref.Name).Str("' first")
				} else {
					num := ref.Name
					if len(num) > 2 && (num[0] == 'A' || num[0] == 'a') && (num[1] == 'S' || num[1] == 's') {
						num = num[2:]
					}
					tb.Str("firewall irr: no cached prefix data for ").Str(ref.Name)
					tb.Str("; run 'update firewall irr asn ").Str(num).Str("' first")
				}
				return errors.New(tb.String())
			}
		}
		pendingRefs = refs
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		refs := pendingRefs
		pendingRefs = nil
		_ = refs
		return plug.applyTables()
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
		},
		WantsConfig: []string{"firewall"},
	}); err != nil {
		logger().Error("firewall-irr plugin failed", "error", err)
		return 1
	}
	return 0
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
	for _, ref := range cfg.allRefs() {
		_, err := ps.Refresh(ctx, ref.Name, "")
		if err != nil {
			logger().Warn("firewall-irr: refresh failed", "name", ref.Name, "error", err)
			incRefreshOutcome("error")
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		incRefreshOutcome("success")
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
	_, err := ps.Refresh(ctx, name, "")
	if err != nil {
		incRefreshOutcome("error")
		return err
	}
	incRefreshOutcome("success")

	plug.mu.Lock()
	plug.lastRefresh = time.Now()
	plug.mu.Unlock()

	updateMetricsGauges(ps, plug.configRefs())
	return nil
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

func updateMetricsGauges(ps *store.PrefixStore, refs []irrRef) {
	m := irrMetricsPtr.Load()
	if m == nil {
		return
	}
	total := 0
	for _, ref := range refs {
		entry := ps.Get(ref.Name)
		if entry == nil {
			continue
		}
		total += len(entry.IPv4) + len(entry.IPv6)
	}
	m.prefixesCached.Set(float64(total))
	m.lastRefresh.Set(float64(time.Now().Unix()))
}

func extractIRRRefs(sections []sdk.ConfigSection) []irrRef {
	for _, s := range sections {
		if s.Root != "firewall" {
			continue
		}
		var root map[string]any
		if json.Unmarshal([]byte(s.Data), &root) != nil {
			continue
		}
		refs := extractRefsFromConfig(root)
		refs = append(refs, extractIfaceRefs(root)...)
		return refs
	}
	return nil
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
