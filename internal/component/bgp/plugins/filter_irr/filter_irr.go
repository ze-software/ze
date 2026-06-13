// Design: plan/spec-filter-irr.md -- IRR prefix-list filter plugin entry point

package filter_irr

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/configjson"
	"codeberg.org/thomas-mangin/ze/internal/component/resolve/irr"
	"codeberg.org/thomas-mangin/ze/internal/component/resolve/peeringdb"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

var errFilterIrrInvalidBgpConfigJson = errors.New("filter-irr: invalid bgp config JSON")

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
}

type irrPlugin struct {
	plugin    *sdk.Plugin
	irrClient *irr.IRR
	pdbClient *peeringdb.PeeringDB

	mu          sync.RWMutex
	byASN       map[uint32]*asnState
	config      *irrConfig
	nextRefresh time.Time

	stopCh      chan struct{}
	refreshStop chan struct{}
	refreshing  atomic.Bool
}

func RunFilterIRR(conn net.Conn) int {
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
	plug.config = cfg

	plug.irrClient = irr.NewIRR(cfg.Server)
	plug.pdbClient = peeringdb.NewPeeringDB(cfg.PeeringDBURL)

	newByASN := make(map[uint32]*asnState)
	for _, peer := range cfg.Peers {
		if peer.Disabled {
			continue
		}
		st, exists := newByASN[peer.RemoteASN]
		if !exists {
			st = &asnState{asn: peer.RemoteASN, asSet: peer.ASSet}
			newByASN[peer.RemoteASN] = st
		}
		st.peerAddrs = append(st.peerAddrs, peer.PeerAddr)
		if peer.ASSet != "" && st.asSet == "" {
			st.asSet = peer.ASSet
		}
	}

	if plug.refreshStop != nil {
		close(plug.refreshStop)
	}
	plug.refreshStop = make(chan struct{})

	plug.mu.Lock()
	plug.byASN = newByASN
	plug.mu.Unlock()

	plug.loadCache()

	go plug.refreshAll()
	go plug.refreshLoop(cfg.RefreshInterval, plug.refreshStop)

	logger().Debug("configured", "peers", len(cfg.Peers), "asns", len(newByASN), "server", cfg.Server)
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
	if st != nil {
		list = st.list
	}
	plug.mu.RUnlock()

	if list == nil || len(list.entries) == 0 {
		logger().Info("irr filter reject", "filter", in.Filter, "peer", in.Peer, "reason", "no-prefix-list")
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}

	nlriField := extractNLRIField(in.Update)
	if list.evaluateUpdate(nlriField) {
		logger().Info("irr filter accept", "filter", in.Filter, "peer", in.Peer, "nlri", nlriField)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}
	}
	logger().Info("irr filter reject", "filter", in.Filter, "peer", in.Peer, "nlri", nlriField)
	return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
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

func (plug *irrPlugin) refreshAll() {
	if !plug.refreshing.CompareAndSwap(false, true) {
		return
	}
	defer plug.refreshing.Store(false)

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
	plug.mu.RLock()
	st := plug.byASN[asn]
	asSet := ""
	if st != nil {
		asSet = st.asSet
	}
	plug.mu.RUnlock()
	if st == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if asSet == "" {
		discovered, err := plug.pdbClient.LookupASSet(ctx, asn)
		if err != nil || len(discovered) == 0 {
			plug.mu.Lock()
			st.lastErr = "no AS-SET found via PeeringDB"
			plug.mu.Unlock()
			logger().Warn("irr: no AS-SET for ASN", "asn", asn, "error", err)
			incRefreshOutcome("error")
			return
		}
		asSet = discovered[0]
		plug.mu.Lock()
		st.asSet = asSet
		plug.mu.Unlock()
	}

	prefixes, err := plug.irrClient.LookupPrefixes(ctx, asSet)
	if err != nil {
		plug.mu.Lock()
		st.lastErr = err.Error()
		plug.mu.Unlock()
		logger().Warn("irr: lookup failed", "asn", asn, "as-set", asSet, "error", err)
		incRefreshOutcome("error")
		return
	}

	entries := prefixListFromIRR(prefixes)
	now := time.Now()

	plug.mu.Lock()
	st.list = &irrPrefixList{entries: entries}
	st.lastOK = now
	st.lastErr = ""
	st.v4Count = len(prefixes.IPv4)
	st.v6Count = len(prefixes.IPv6)
	plug.mu.Unlock()

	logger().Info("irr: refreshed", "asn", asn, "as-set", asSet, "v4", len(prefixes.IPv4), "v6", len(prefixes.IPv6))
	incRefreshOutcome("success")
	updateMetricsGauges(plug)
	plug.saveCache()
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

const (
	statusDone  = "done"
	statusError = "error"
)

func (plug *irrPlugin) handleCommand(command string, args []string) (string, any, error) {
	switch command {
	case "show bgp irr":
		return plug.showIRR()
	case "show bgp irr prefix":
		return plug.showIRRPrefix(args)
	case "show bgp irr check":
		return plug.showIRRCheck(args)
	case "update bgp irr all":
		go plug.refreshAll()
		return statusDone, json.RawMessage(`{"refreshing":"all"}`), nil
	case "update bgp irr asn":
		return plug.updateASN(args)
	case "update bgp irr as-set":
		return plug.updateASSet(args)
	}
	return statusError, nil, errors.New("unknown command")
}

func (plug *irrPlugin) showIRR() (string, any, error) {
	plug.mu.RLock()
	defer plug.mu.RUnlock()

	b := textbuf.Get()
	defer b.Release()

	server := defaultServer
	if plug.config != nil {
		server = plug.config.Server
	}
	b.Str(`{"server":"`).Str(server).Byte('"')
	if !plug.nextRefresh.IsZero() {
		b.Str(`,"next-refresh":"`).Str(plug.nextRefresh.Format(time.RFC3339)).Byte('"')
	}
	sortedASNs := make([]uint32, 0, len(plug.byASN))
	for asn := range plug.byASN {
		sortedASNs = append(sortedASNs, asn)
	}
	slices.Sort(sortedASNs)

	b.Str(`,"entries":[`)
	first := true
	for _, asn := range sortedASNs {
		st := plug.byASN[asn]
		if !first {
			b.Byte(',')
		}
		first = false
		b.Str(`{"asn":`).Uint32(st.asn)
		b.Str(`,"as-set":`).Quoted(st.asSet)
		switch {
		case st.lastErr != "":
			b.Str(`,"status":"error"`)
			b.Str(`,"error":`).Quoted(st.lastErr)
		case st.list != nil && len(st.list.entries) > 0:
			b.Str(`,"status":"ok"`)
		default:
			b.Str(`,"status":"pending"`)
		}
		b.Str(`,"ipv4-count":`).Int(int64(st.v4Count))
		b.Str(`,"ipv6-count":`).Int(int64(st.v6Count))
		if !st.lastOK.IsZero() {
			b.Str(`,"last-refresh":"`).Str(st.lastOK.Format(time.RFC3339)).Byte('"')
		}
		b.Str(`,"peers":[`)
		for j, addr := range st.peerAddrs {
			if j > 0 {
				b.Byte(',')
			}
			b.Quoted(addr)
		}
		b.Str(`]}`)
	}
	b.Str(`]}`)
	return statusDone, json.RawMessage(b.String()), nil
}

func (plug *irrPlugin) showIRRPrefix(args []string) (string, any, error) {
	if len(args) == 0 || args[0] == "" {
		return statusError, nil, errors.New("usage: show bgp irr prefix <peer>")
	}
	peerAddr := args[0]

	plug.mu.RLock()
	defer plug.mu.RUnlock()

	for _, st := range plug.byASN {
		if slices.Contains(st.peerAddrs, peerAddr) {
			return plug.renderPrefixes(st)
		}
	}
	return statusError, nil, errors.New("peer not found in IRR state")
}

func (plug *irrPlugin) renderPrefixes(st *asnState) (string, any, error) {
	b := textbuf.Get()
	defer b.Release()
	b.Str(`{"asn":`).Uint32(st.asn)
	b.Str(`,"as-set":`).Quoted(st.asSet)
	b.Str(`,"prefixes":[`)
	if st.list != nil {
		for i, e := range st.list.entries {
			if i > 0 {
				b.Byte(',')
			}
			b.Byte('"').Str(e.prefix.String()).Byte('"')
		}
	}
	b.Str(`]}`)
	return statusDone, json.RawMessage(b.String()), nil
}

func (plug *irrPlugin) showIRRCheck(args []string) (string, any, error) {
	if len(args) < 2 {
		return statusError, nil, errors.New("usage: show bgp irr check <peer> <prefix>")
	}
	peerAddr := args[0]
	prefixStr := args[1]

	prefix, err := netip.ParsePrefix(prefixStr)
	if err != nil {
		return statusError, nil, errors.New("invalid prefix")
	}

	plug.mu.RLock()
	defer plug.mu.RUnlock()

	var matchSt *asnState
	for _, st := range plug.byASN {
		if slices.Contains(st.peerAddrs, peerAddr) {
			matchSt = st
			break
		}
	}
	if matchSt == nil {
		return statusError, nil, errors.New("peer not found in IRR state")
	}

	accepted := false
	matchedEntry := ""
	if matchSt.list != nil {
		accepted = evaluatePrefix(matchSt.list.entries, prefix)
		if accepted {
			matchedEntry = findMatchingEntry(matchSt.list.entries, prefix)
		}
	}
	b := textbuf.Get()
	defer b.Release()
	b.Str(`{"prefix":"`).Str(prefixStr).Byte('"')
	b.Str(`,"asn":`).Uint32(matchSt.asn)
	b.Str(`,"accepted":`).Bool(accepted)
	if matchedEntry != "" {
		b.Str(`,"matched-entry":"`).Str(matchedEntry).Byte('"')
	}
	b.Byte('}')
	return statusDone, json.RawMessage(b.String()), nil
}

func (plug *irrPlugin) updateASN(args []string) (string, any, error) {
	if len(args) == 0 || args[0] == "" {
		return statusError, nil, errors.New("usage: update bgp irr asn <asn>")
	}
	v, ok := readUint(args[0])
	if !ok {
		return statusError, nil, errors.New("invalid ASN")
	}
	asn := uint32(v) //nolint:gosec // user input, range acceptable
	go plug.refreshASN(asn)
	return statusDone, json.RawMessage(`{"refreshing":"asn ` + textbuf.StringUint32(asn) + `"}`), nil
}

func (plug *irrPlugin) updateASSet(args []string) (string, any, error) {
	if len(args) == 0 || args[0] == "" {
		return statusError, nil, errors.New("usage: update bgp irr as-set <as-set>")
	}
	target := args[0]
	if err := irr.ValidateASSetName(target); err != nil {
		return statusError, nil, errors.New("invalid AS-SET name")
	}
	plug.mu.RLock()
	var toRefresh []uint32
	for asn, st := range plug.byASN {
		if st.asSet == target {
			toRefresh = append(toRefresh, asn)
		}
	}
	plug.mu.RUnlock()

	go func() {
		for _, asn := range toRefresh {
			plug.refreshASN(asn)
		}
	}()
	var tb textbuf.Buffer
	tb.Str(`{"refreshing":"as-set `).Str(target).Str(`"}`)
	return statusDone, json.RawMessage(tb.String()), nil
}
