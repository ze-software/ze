// Design: plan/spec-mpls-3-rsvp-te.md -- RSVP-TE component registration
// Related: wire.go -- RSVP message codec
// Related: fsm.go -- per-LSP state machine
// Related: admission.go -- bandwidth admission control
// Related: events.go -- event bus handles
//
// Package rsvpte implements RSVP-TE (RFC 3209) for explicitly-routed
// MPLS LSPs with bandwidth reservation. RSVP runs on raw IP (protocol 46).
package rsvpte

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	ifaceevents "codeberg.org/thomas-mangin/ze/internal/component/iface/events"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	rsvpteyang "codeberg.org/thomas-mangin/ze/internal/component/rsvpte/yang"
	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

var (
	loggerPtr   atomic.Pointer[slog.Logger]
	eventBusPtr atomic.Pointer[ze.EventBus]
)

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

func setEventBus(eb ze.EventBus) {
	if eb != nil {
		eventBusPtr.Store(&eb)
	}
}

func getEventBus() ze.EventBus {
	p := eventBusPtr.Load()
	if p == nil {
		return nil
	}
	return *p
}

type rsvpteMetrics struct {
	lspsActive      metrics.Gauge
	lspsTotal       metrics.Counter
	pathMsgSent     metrics.Counter
	pathMsgRecv     metrics.Counter
	resvMsgSent     metrics.Counter
	resvMsgRecv     metrics.Counter
	pathErrRecv     metrics.Counter
	admissionDenied metrics.Counter
}

var rsvpteMetricsPtr atomic.Pointer[rsvpteMetrics]

func setMetricsRegistry(reg metrics.Registry) {
	m := &rsvpteMetrics{
		lspsActive:      reg.Gauge("ze_rsvpte_lsps_active", "Current number of active RSVP-TE LSPs."),
		lspsTotal:       reg.Counter("ze_rsvpte_lsps_total", "Total RSVP-TE LSPs established."),
		pathMsgSent:     reg.Counter("ze_rsvpte_path_sent_total", "Total PATH messages sent."),
		pathMsgRecv:     reg.Counter("ze_rsvpte_path_recv_total", "Total PATH messages received."),
		resvMsgSent:     reg.Counter("ze_rsvpte_resv_sent_total", "Total RESV messages sent."),
		resvMsgRecv:     reg.Counter("ze_rsvpte_resv_recv_total", "Total RESV messages received."),
		pathErrRecv:     reg.Counter("ze_rsvpte_patherr_recv_total", "Total PathErr messages received."),
		admissionDenied: reg.Counter("ze_rsvpte_admission_denied_total", "Total admission control denials."),
	}
	rsvpteMetricsPtr.Store(m)
}

const cmdDone = "done"

// DefaultRefreshPeriod is the RFC 2205 default refresh interval.
const DefaultRefreshPeriod = 30 * time.Second

// DefaultRefreshMultiplier is how many missed refreshes expire state.
const DefaultRefreshMultiplier = 3

type rsvpteConfig struct {
	RouterID          netip.Addr
	RefreshPeriod     time.Duration
	RefreshMultiplier int
	Interfaces        []ifaceConfig
	Tunnels           []tunnelConfig
}

type ifaceConfig struct {
	Name            string
	MaxBW           float32
	MaxReservableBW float32
	Prefix          netip.Prefix // local link prefix; maps a neighbor to this link
}

type tunnelConfig struct {
	Name          string
	Destination   netip.Addr
	TunnelID      uint16
	Bandwidth     float32
	SetupPriority uint8
	HoldPriority  uint8
	ERO           []eroHop
}

// rsvpteNumber coerces a config-tree scalar to a float64. Tree.ToMap renders
// every YANG leaf as a JSON string (e.g. "30"), so a numeric leaf arrives as a
// string here, not a JSON number. We accept both for robustness.
func rsvpteNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case string:
		f, err := strconv.ParseFloat(n, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

// rsvpteList coerces a YANG list value into its entries sorted by list key.
// Tree.ToMap renders a YANG list as a keyed map (key -> entry), so this returns
// the entries paired with their key, ordered by the key string. Pass less to
// override the default lexical key ordering (e.g. numeric for explicit-route).
func rsvpteList(v any, numericKey bool) []listEntry {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	entries := make([]listEntry, 0, len(m))
	for key, raw := range m {
		em, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		entries = append(entries, listEntry{key: key, data: em})
	}
	if numericKey {
		sort.Slice(entries, func(i, j int) bool {
			ai, _ := strconv.Atoi(entries[i].key)
			bj, _ := strconv.Atoi(entries[j].key)
			return ai < bj
		})
	} else {
		sort.Slice(entries, func(i, j int) bool { return entries[i].key < entries[j].key })
	}
	return entries
}

type listEntry struct {
	key  string
	data map[string]any
}

func parseConfig(sections []sdk.ConfigSection) (rsvpteConfig, error) {
	cfg := rsvpteConfig{
		RefreshPeriod:     DefaultRefreshPeriod,
		RefreshMultiplier: DefaultRefreshMultiplier,
	}
	for _, sec := range sections {
		if sec.Root != "rsvp-te" || sec.Data == "" {
			continue
		}
		// BuildPluginConfigSections wraps the subtree under its root key, so the
		// delivered JSON is {"rsvp-te": {...}}. Unwrap before reading leaves.
		var wrapper map[string]any
		if err := json.Unmarshal([]byte(sec.Data), &wrapper); err != nil {
			return cfg, fmt.Errorf("rsvp-te: invalid config JSON: %w", err)
		}
		tree, _ := wrapper["rsvp-te"].(map[string]any)
		if tree == nil {
			continue
		}
		if v, ok := tree["router-id"].(string); ok {
			addr, err := netip.ParseAddr(v)
			if err != nil {
				return cfg, fmt.Errorf("rsvp-te: invalid router-id: %w", err)
			}
			cfg.RouterID = addr
		}
		if v, ok := rsvpteNumber(tree["refresh-period"]); ok && v > 0 {
			cfg.RefreshPeriod = time.Duration(v) * time.Second
		}
		if v, ok := rsvpteNumber(tree["refresh-multiplier"]); ok && v > 0 {
			cfg.RefreshMultiplier = int(v)
		}
		for _, entry := range rsvpteList(tree["interface"], false) {
			m := entry.data
			ic := ifaceConfig{Name: entry.key}
			if v, ok := m["name"].(string); ok && v != "" {
				ic.Name = v
			}
			if v, ok := rsvpteNumber(m["max-bandwidth"]); ok {
				ic.MaxBW = float32(v)
			}
			if v, ok := rsvpteNumber(m["max-reservable-bandwidth"]); ok {
				ic.MaxReservableBW = float32(v)
			}
			if v, ok := m["address"].(string); ok && v != "" {
				p, err := netip.ParsePrefix(v)
				if err != nil {
					return cfg, fmt.Errorf("rsvp-te: interface %s invalid address %q: %w", ic.Name, v, err)
				}
				ic.Prefix = p
			}
			cfg.Interfaces = append(cfg.Interfaces, ic)
		}
		for _, entry := range rsvpteList(tree["tunnel"], false) {
			m := entry.data
			tc := tunnelConfig{
				Name:          entry.key,
				SetupPriority: 7,
				HoldPriority:  7,
			}
			if v, ok := m["name"].(string); ok && v != "" {
				tc.Name = v
			}
			if v, ok := m["destination"].(string); ok {
				if addr, err := netip.ParseAddr(v); err == nil {
					tc.Destination = addr
				}
			}
			if v, ok := rsvpteNumber(m["tunnel-id"]); ok {
				tc.TunnelID = uint16(v)
			}
			if v, ok := rsvpteNumber(m["bandwidth"]); ok {
				tc.Bandwidth = float32(v)
			}
			if v, ok := rsvpteNumber(m["setup-priority"]); ok {
				tc.SetupPriority = uint8(v)
			}
			if v, ok := rsvpteNumber(m["hold-priority"]); ok {
				tc.HoldPriority = uint8(v)
			}
			for _, hopEntry := range rsvpteList(m["explicit-route"], true) {
				hopMap := hopEntry.data
				hop := eroHop{}
				if v, ok := hopMap["address"].(string); ok {
					if pfx, err := netip.ParsePrefix(v); err == nil {
						hop.Address = pfx
					}
				}
				if v, ok := hopMap["type"].(string); ok && v == "loose" {
					hop.Loose = true
				}
				tc.ERO = append(tc.ERO, hop)
			}
			cfg.Tunnels = append(cfg.Tunnels, tc)
		}
	}
	return cfg, nil
}

func registerRSVPTE() {
	_ = events.RegisterNamespace(Namespace, EventLSPUp, EventLSPDown, EventPathErr)

	reg := registry.Registration{
		Name:         "rsvp-te",
		Description:  "RSVP-TE: Resource Reservation Protocol - Traffic Engineering (RFC 3209)",
		Features:     "yang",
		YANG:         rsvpteyang.ZeRSVPTEConfYANG,
		ConfigRoots:  []string{"rsvp-te"},
		Dependencies: []string{"fib-kernel", "sysctl"},
		RunEngine:    runRSVPTEEngine,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg any) {
			if r, ok := reg.(metrics.Registry); ok {
				setMetricsRegistry(r)
			}
		},
		ConfigureEventBus: func(eb any) {
			if e, ok := eb.(ze.EventBus); ok {
				setEventBus(e)
			}
		},
		DoctorChecks: []registry.DoctorCheckDef{{
			Name:         "rsvp-te-rawsock",
			Phase:        rpc.DoctorPhasePostConfig,
			Order:        721,
			Dependencies: []string{"fib-kernel"},
			Platforms:    []string{"any"},
			Codes:        []string{"doctor-rsvpte-rawsock-unavailable"},
			Check:        checkRSVPTERawSocket,
		}},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			setLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "rsvp-te: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func init() { registerRSVPTE() }

func runRSVPTEEngine(conn net.Conn) int {
	log := logger()
	log.Debug("rsvp-te engine starting")

	p := sdk.NewWithConn("rsvp-te", conn)
	defer func() { _ = p.Close() }()

	lspTable := newLSPTable()
	admission := newAdmissionController()

	var activeCfg, pendingCfg rsvpteConfig
	var havePending bool
	var eng *engine
	var configuredTunnels map[lspKey]bool
	var tunnelsMu sync.Mutex // guards activeCfg/pendingCfg/havePending, eng, configuredTunnels

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		cfg, err := parseConfig(sections)
		if err != nil {
			return err
		}
		tunnelsMu.Lock()
		pendingCfg = cfg
		havePending = true
		tunnelsMu.Unlock()
		return nil
	})

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, err := parseConfig(sections)
		if err != nil {
			return err
		}
		activeCfg = cfg
		for _, iface := range cfg.Interfaces {
			admission.setInterface(iface.Name, float64(iface.MaxBW), float64(iface.MaxReservableBW))
		}
		return nil
	})

	// OnConfigApply is the reload-pipeline commit step (OnConfigure does not fire
	// on reload). Adopt the verified pending config and reconcile the tunnel set so
	// an added tunnel signals, a changed ERO reroutes (make-before-break), and a
	// removed tunnel is torn down -- all without restarting the engine.
	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		tunnelsMu.Lock()
		defer tunnelsMu.Unlock()
		if havePending {
			activeCfg = pendingCfg
			havePending = false
		}
		cfg := activeCfg
		for _, iface := range cfg.Interfaces {
			admission.setInterface(iface.Name, float64(iface.MaxBW), float64(iface.MaxReservableBW))
		}
		configuredTunnels = reconcileTunnels(log, lspTable, cfg, eng, configuredTunnels)
		return nil
	})

	p.OnStarted(func(ctx context.Context) error {
		cfg := activeCfg
		if !cfg.RouterID.IsValid() {
			log.Warn("rsvp-te: no router-id configured, engine idle")
			return nil
		}

		// Open the raw IP transport (protocol 46). On platforms without it, or
		// without CAP_NET_RAW, the component stays up for config/show but cannot
		// signal; this is logged rather than fatal.
		t, err := newTransport(cfg.RouterID)
		if err != nil {
			log.Warn("rsvp-te: raw IP transport unavailable, signaling disabled", "error", err)
		} else {
			e := newEngine(t, lspTable, admission, newBusFIB(getEventBus(), log), cfg, log)
			tunnelsMu.Lock()
			eng = e
			tunnelsMu.Unlock()
			go func() {
				e.run(ctx)
				if cerr := t.Close(); cerr != nil {
					log.Warn("rsvp-te: transport close", "error", cerr)
				}
			}()
			go func() {
				<-ctx.Done()
				if cerr := t.Close(); cerr != nil {
					log.Warn("rsvp-te: transport close on shutdown", "error", cerr)
				}
			}()
		}

		// With several interfaces, admission maps each LSP to a link by matching
		// the neighbor against the interfaces' configured `address` prefixes. Warn
		// once at startup for any multi-interface config that omits prefixes,
		// since those interfaces cannot be resolved and their LSPs go unaccounted.
		if len(cfg.Interfaces) > 1 {
			missing := 0
			for _, ic := range cfg.Interfaces {
				if !ic.Prefix.IsValid() {
					missing++
				}
			}
			if missing > 0 {
				log.Warn("rsvp-te: multi-interface admission needs an `address` prefix per interface to map LSPs to links; interfaces without one are not admission-enforced",
					"interfaces", len(cfg.Interfaces), "without-address", missing)
			}
		}

		go runRefreshLoop(ctx, log, lspTable, cfg, eng)

		go runCleanupLoop(ctx, log, lspTable, cfg, eng)

		// React to interface-down events: an LSP whose downstream link fails is
		// torn down and a PathErr reported toward the head-end (AC-6). Only
		// meaningful when signaling is active (eng != nil). The EventBus handler
		// MUST NOT block (pkg/ze/eventbus.go), so it only enqueues the interface
		// name; a worker goroutine does the raw-socket sends and FIB mutation.
		if eng != nil {
			if eb := getEventBus(); eb != nil {
				linkDownCh := make(chan string, 16)
				go func() {
					for {
						select {
						case <-ctx.Done():
							return
						case name := <-linkDownCh:
							eng.handleLinkDown(name)
						}
					}
				}()
				unsub := eb.Subscribe(ifaceevents.Namespace, ifaceevents.EventDown, events.AsString(func(data string) {
					var ev struct {
						Name string `json:"name"`
					}
					if err := json.Unmarshal([]byte(data), &ev); err == nil && ev.Name != "" {
						select {
						case linkDownCh <- ev.Name:
						default: // worker backed up; drop (a later event re-triggers)
						}
					}
				}))
				go func() {
					<-ctx.Done()
					unsub()
				}()
			}
		}

		tunnelsMu.Lock()
		configuredTunnels = reconcileTunnels(log, lspTable, cfg, eng, configuredTunnels)
		tunnelsMu.Unlock()

		return nil
	})

	p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, any, error) {
		switch command {
		case "show rsvp-te session":
			return cmdDone, showSessions(lspTable), nil
		case "show rsvp-te interface":
			return cmdDone, showInterfaces(admission), nil
		case "show rsvp-te tunnel":
			return cmdDone, showTunnels(lspTable), nil
		default:
			return "error", "", fmt.Errorf("unknown command: %s", command)
		}
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{"rsvp-te"},
		VerifyBudget: 1,
		ApplyBudget:  1,
		Commands: []sdk.CommandDecl{
			{Name: "show rsvp-te session"},
			{Name: "show rsvp-te interface"},
			{Name: "show rsvp-te tunnel"},
		},
	})
	if err != nil {
		log.Error("rsvp-te engine failed", "error", err)
		return 1
	}

	return 0
}

func addrToUint32(addr netip.Addr) uint32 {
	b := addr.As4()
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// tunnelKey is the LSP key a configured tunnel maps to: the head-end identity
// (this router as sender, the tunnel's destination and ID). Shared by setupTunnel
// and reconcileTunnels so the configured set and the live LSPs key identically.
func tunnelKey(tc tunnelConfig, routerID netip.Addr) lspKey {
	return lspKey{
		TunnelEndpoint: tc.Destination,
		TunnelID:       tc.TunnelID,
		ExtTunnelID:    addrToUint32(routerID),
		SenderAddr:     routerID,
		LSPID:          1,
	}
}

// reconcileTunnels brings the live LSP set in line with cfg's tunnels: it sets up
// (or, for a changed ERO on an up LSP, reroutes) every configured tunnel and tears
// down the head-end LSP of any tunnel removed since prev. It returns the new
// configured-key set. eng may be nil (no transport): setup records intent only and
// teardown is skipped, since there is no signaled LSP to remove.
func reconcileTunnels(log *slog.Logger, lspTable *lspTable, cfg rsvpteConfig, eng *engine, prev map[lspKey]bool) map[lspKey]bool {
	next := make(map[lspKey]bool, len(cfg.Tunnels))
	for _, tc := range cfg.Tunnels {
		setupTunnel(log, lspTable, tc, cfg, eng)
		next[tunnelKey(tc, cfg.RouterID)] = true
	}
	if eng != nil {
		for key := range prev {
			if !next[key] {
				eng.teardownLSP(key)
				log.Info("rsvp-te: tunnel removed from config, LSP torn down",
					"dest", key.TunnelEndpoint, "tunnel-id", key.TunnelID)
			}
		}
	}
	return next
}

func setupTunnel(log *slog.Logger, lspTable *lspTable, tc tunnelConfig, cfg rsvpteConfig, eng *engine) {
	extID := addrToUint32(cfg.RouterID)
	key := tunnelKey(tc, cfg.RouterID)

	lsp, existed := lspTable.GetOrCreate(key)
	if existed && lsp.State == LSPStateUp {
		// The LSP is already up. If the configured ERO changed, trigger a
		// make-before-break reroute (RFC 3209 Section 6.1) rather than
		// disturbing the live tunnel.
		lsp.mu.Lock()
		changed := lsp.PSB != nil && !eroEqual(lsp.PSB.ERO, tc.ERO)
		lsp.mu.Unlock()
		if changed && eng != nil {
			if _, ok := eng.reroute(key, tc.ERO); ok {
				log.Info("rsvp-te: tunnel reroute (make-before-break) started", "name", tc.Name, "dest", tc.Destination)
			}
		}
		return
	}

	lsp.mu.Lock()
	lsp.Role = RoleIngress
	lsp.Bandwidth = tc.Bandwidth
	lsp.SetupPriority = tc.SetupPriority
	lsp.HoldPriority = tc.HoldPriority
	lsp.PSB = &pathStateBlock{
		Session: sessionIPv4{
			TunnelEndpoint: tc.Destination,
			TunnelID:       tc.TunnelID,
			ExtTunnelID:    extID,
		},
		SenderTemplate: senderTemplateIPv4{
			SenderAddr: cfg.RouterID,
			LSPID:      1,
		},
		ERO: tc.ERO,
		SenderTSpec: FlowSpec{
			TokenRate:      tc.Bandwidth,
			TokenBucket:    tc.Bandwidth,
			PeakRate:       tc.Bandwidth,
			MinPolicedUnit: 20,
			MaxPacketSize:  65535,
		},
		LabelRequest:  labelRequest{L3PID: 0x0800},
		RefreshPeriod: cfg.RefreshPeriod,
		LastRefresh:   time.Now(),
	}
	lsp.setState(LSPStatePathSent)
	lsp.mu.Unlock()

	// Send the PATH toward the egress. Without a transport (eng nil) the LSP
	// stays in path-sent as local intent only.
	if eng != nil {
		if err := eng.sendPath(lsp); err != nil {
			log.Warn("rsvp-te: initial PATH send failed", "name", tc.Name, "dest", tc.Destination, "error", err)
		}
	} else if m := rsvpteMetricsPtr.Load(); m != nil {
		m.pathMsgSent.Inc()
	}

	log.Info("rsvp-te: tunnel configured", "name", tc.Name, "dest", tc.Destination, "tunnel-id", tc.TunnelID)
}

// eroEqual reports whether two explicit routes are identical (same hops in the
// same order). Used to detect a configured path change that warrants a reroute.
func eroEqual(a, b []eroHop) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Loose != b[i].Loose || a[i].Address != b[i].Address {
			return false
		}
	}
	return true
}

func runRefreshLoop(ctx context.Context, log *slog.Logger, lspTable *lspTable, cfg rsvpteConfig, eng *engine) {
	ticker := time.NewTicker(cfg.RefreshPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refreshPaths(log, lspTable, eng)
		}
	}
}

// refreshPaths maintains RFC 2205 soft-state on every refresh tick: ingress LSPs
// re-send PATH downstream, while egress and transit LSPs re-send RESV upstream so
// the reservation does not expire if no fresh PATH/RESV arrives. When no
// transport is available (eng nil) it only stamps the PSB so local state stays
// consistent.
func refreshPaths(log *slog.Logger, lspTable *lspTable, eng *engine) {
	for _, lsp := range lspTable.All() {
		// Decide eligibility and stamp the refresh under the LSP lock, then
		// release it before sendPath/sendResv (which take the same lock).
		lsp.mu.Lock()
		if lsp.PSB == nil || lsp.State == LSPStateDown {
			lsp.mu.Unlock()
			continue
		}
		isIngress := lsp.Role == RoleIngress
		hasRSB := lsp.RSB != nil
		// Only the PATH originator (ingress) refreshes its own PSB soft-state
		// here. A transit/egress PSB is refreshed by the incoming PATH it relays
		// (RFC 2205 Section 3.4); stamping it locally would stop the cleanup loop
		// from ever expiring the LSP once the upstream stops refreshing PATH,
		// leaking the reservation and FIB state.
		if isIngress {
			lsp.PSB.LastRefresh = time.Now()
		}
		lsp.mu.Unlock()

		switch {
		case eng != nil && isIngress:
			if err := eng.sendPath(lsp); err != nil {
				log.Warn("rsvp-te: PATH refresh send failed", "lsp", lsp.Key.String(), "error", err)
			}
		case eng != nil && hasRSB:
			// Egress/transit: re-send RESV upstream (RFC 2205 Section 3.7).
			if err := eng.sendResv(lsp); err != nil {
				log.Warn("rsvp-te: RESV refresh send failed", "lsp", lsp.Key.String(), "error", err)
			}
		default:
			log.Debug("rsvp-te: refresh", "lsp", lsp.Key.String())
		}
	}
}

func runCleanupLoop(ctx context.Context, log *slog.Logger, lspTable *lspTable, cfg rsvpteConfig, eng *engine) {
	ticker := time.NewTicker(cfg.RefreshPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			expired := lspTable.expiredPSBs(now, cfg.RefreshMultiplier)
			for _, key := range expired {
				// Soft-state expiry must release the same state a teardown does:
				// admission bandwidth, FIB entries, label, and the lsp-down event.
				// tearLSPLocal does all of that (and the table Remove); without it
				// an expired LSP leaks its reservation and kernel MPLS entry.
				if eng != nil {
					eng.tearLSPLocal(key)
				} else if lsp := lspTable.Remove(key); lsp != nil {
					lsp.mu.Lock()
					inLabel := lsp.InLabel
					lsp.mu.Unlock()
					lspTable.releaseLabel(inLabel)
					emitLSPDown(log, lsp, lspTable.Len())
				}
				log.Info("rsvp-te: LSP expired", "lsp", key.String())
			}
		}
	}
}

// emitLSPUp publishes an lsp-up event and updates metrics. activeCount is the
// current number of LSPs in the table so the lspsActive gauge reflects the live
// count (not, as a prior bug had it, the tunnel ID).
func emitLSPUp(log *slog.Logger, lsp *LSP, activeCount int) {
	eb := getEventBus()
	if eb == nil {
		return
	}
	lsp.mu.Lock()
	bandwidth := lsp.Bandwidth
	lsp.mu.Unlock()
	evt := &LSPEvent{
		TunnelEndpoint: lsp.Key.TunnelEndpoint.String(),
		TunnelID:       lsp.Key.TunnelID,
		LSPID:          lsp.Key.LSPID,
		Bandwidth:      bandwidth,
		State:          "up",
	}
	if _, err := LSPUp.Emit(eb, evt); err != nil {
		log.Warn("rsvp-te: lsp-up emit failed", "error", err)
	}
	if m := rsvpteMetricsPtr.Load(); m != nil {
		m.lspsActive.Set(float64(activeCount))
		m.lspsTotal.Inc()
	}
}

// emitLSPDown publishes an lsp-down event and sets the lspsActive gauge to the
// post-removal table count.
func emitLSPDown(log *slog.Logger, lsp *LSP, activeCount int) {
	eb := getEventBus()
	if eb == nil {
		return
	}
	lsp.mu.Lock()
	bandwidth := lsp.Bandwidth
	lsp.mu.Unlock()
	evt := &LSPEvent{
		TunnelEndpoint: lsp.Key.TunnelEndpoint.String(),
		TunnelID:       lsp.Key.TunnelID,
		LSPID:          lsp.Key.LSPID,
		Bandwidth:      bandwidth,
		State:          "down",
	}
	if _, err := LSPDown.Emit(eb, evt); err != nil {
		log.Warn("rsvp-te: lsp-down emit failed", "error", err)
	}
	if m := rsvpteMetricsPtr.Load(); m != nil {
		m.lspsActive.Set(float64(activeCount))
	}
}

func showSessions(lspTable *lspTable) any {
	type sessionInfo struct {
		TunnelEndpoint string   `json:"tunnel-endpoint"`
		TunnelID       uint16   `json:"tunnel-id"`
		LSPID          uint16   `json:"lsp-id"`
		SenderAddr     string   `json:"sender-address"`
		State          string   `json:"state"`
		Role           string   `json:"role"`
		Bandwidth      float32  `json:"bandwidth"`
		InLabel        uint32   `json:"in-label"`
		OutLabel       uint32   `json:"out-label"`
		ERO            []string `json:"ero,omitempty"`
		RRO            []string `json:"rro,omitempty"`
	}
	lsps := lspTable.All()
	out := make([]sessionInfo, 0, len(lsps))
	for _, lsp := range lsps {
		lsp.mu.Lock()
		info := sessionInfo{
			TunnelEndpoint: lsp.Key.TunnelEndpoint.String(),
			TunnelID:       lsp.Key.TunnelID,
			LSPID:          lsp.Key.LSPID,
			SenderAddr:     lsp.Key.SenderAddr.String(),
			State:          lsp.State.String(),
			Role:           lsp.Role.String(),
			Bandwidth:      lsp.Bandwidth,
			InLabel:        lsp.InLabel,
			OutLabel:       lsp.OutLabel,
		}
		if lsp.PSB != nil {
			info.ERO = formatERO(lsp.PSB.ERO)
		}
		if lsp.RSB != nil {
			info.RRO = formatRRO(lsp.RSB.RRO)
		}
		out = append(out, info)
		lsp.mu.Unlock()
	}
	return out
}

func showInterfaces(admission *admissionController) any {
	type ifaceInfo struct {
		Name              string  `json:"name"`
		MaxBandwidth      float64 `json:"max-bandwidth"`
		MaxReservable     float64 `json:"max-reservable"`
		ReservedBandwidth float64 `json:"reserved-bandwidth"`
		Available         float64 `json:"available-bandwidth"`
	}
	ifaces := admission.allInterfaces()
	out := make([]ifaceInfo, 0, len(ifaces))
	for name, ib := range ifaces {
		out = append(out, ifaceInfo{
			Name:              name,
			MaxBandwidth:      ib.MaxBandwidth,
			MaxReservable:     ib.MaxReservable,
			ReservedBandwidth: ib.ReservedBandwidth,
			Available:         ib.Available(),
		})
	}
	return out
}

func showTunnels(lspTable *lspTable) any {
	type tunnelInfo struct {
		TunnelEndpoint string  `json:"tunnel-endpoint"`
		TunnelID       uint16  `json:"tunnel-id"`
		State          string  `json:"state"`
		Bandwidth      float32 `json:"bandwidth"`
		EROHops        int     `json:"ero-hops"`
	}
	lsps := lspTable.All()
	out := make([]tunnelInfo, 0, len(lsps))
	for _, lsp := range lsps {
		lsp.mu.Lock()
		if lsp.Role != RoleIngress {
			lsp.mu.Unlock()
			continue
		}
		eroHops := 0
		if lsp.PSB != nil {
			eroHops = len(lsp.PSB.ERO)
		}
		info := tunnelInfo{
			TunnelEndpoint: lsp.Key.TunnelEndpoint.String(),
			TunnelID:       lsp.Key.TunnelID,
			State:          lsp.State.String(),
			Bandwidth:      lsp.Bandwidth,
			EROHops:        eroHops,
		}
		lsp.mu.Unlock()
		out = append(out, info)
	}
	return out
}
