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
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	rsvpteyang "codeberg.org/thomas-mangin/ze/internal/component/rsvpte/yang"
	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
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
	ERO           []EROHop
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
		var tree map[string]any
		if err := json.Unmarshal([]byte(sec.Data), &tree); err != nil {
			return cfg, fmt.Errorf("rsvp-te: invalid config JSON: %w", err)
		}
		if v, ok := tree["router-id"].(string); ok {
			addr, err := netip.ParseAddr(v)
			if err != nil {
				return cfg, fmt.Errorf("rsvp-te: invalid router-id: %w", err)
			}
			cfg.RouterID = addr
		}
		if v, ok := tree["refresh-period"].(float64); ok && v > 0 {
			cfg.RefreshPeriod = time.Duration(v) * time.Second
		}
		if v, ok := tree["refresh-multiplier"].(float64); ok && v > 0 {
			cfg.RefreshMultiplier = int(v)
		}
		if ifaces, ok := tree["interface"].([]any); ok {
			for _, raw := range ifaces {
				m, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				ic := ifaceConfig{}
				if v, ok := m["name"].(string); ok {
					ic.Name = v
				}
				if v, ok := m["max-bandwidth"].(string); ok {
					if f, err := strconv.ParseFloat(v, 32); err == nil {
						ic.MaxBW = float32(f)
					}
				}
				if v, ok := m["max-reservable-bandwidth"].(string); ok {
					if f, err := strconv.ParseFloat(v, 32); err == nil {
						ic.MaxReservableBW = float32(f)
					}
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
		}
		if tunnels, ok := tree["tunnel"].([]any); ok {
			for _, raw := range tunnels {
				m, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				tc := tunnelConfig{
					SetupPriority: 7,
					HoldPriority:  7,
				}
				if v, ok := m["name"].(string); ok {
					tc.Name = v
				}
				if v, ok := m["destination"].(string); ok {
					if addr, err := netip.ParseAddr(v); err == nil {
						tc.Destination = addr
					}
				}
				if v, ok := m["tunnel-id"].(float64); ok {
					tc.TunnelID = uint16(v)
				}
				if v, ok := m["bandwidth"].(string); ok {
					if f, err := strconv.ParseFloat(v, 32); err == nil {
						tc.Bandwidth = float32(f)
					}
				}
				if v, ok := m["setup-priority"].(float64); ok {
					tc.SetupPriority = uint8(v)
				}
				if v, ok := m["hold-priority"].(float64); ok {
					tc.HoldPriority = uint8(v)
				}
				if ero, ok := m["explicit-route"].([]any); ok {
					for _, hopRaw := range ero {
						hopMap, ok := hopRaw.(map[string]any)
						if !ok {
							continue
						}
						hop := EROHop{}
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
				}
				cfg.Tunnels = append(cfg.Tunnels, tc)
			}
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

	lspTable := NewLSPTable()
	admission := NewAdmissionController()

	var activeCfg rsvpteConfig
	var tunnelsMu sync.Mutex

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		_, err := parseConfig(sections)
		return err
	})

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, err := parseConfig(sections)
		if err != nil {
			return err
		}
		activeCfg = cfg
		for _, iface := range cfg.Interfaces {
			admission.SetInterface(iface.Name, float64(iface.MaxBW), float64(iface.MaxReservableBW))
		}
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
		var eng *engine
		t, err := newTransport(cfg.RouterID)
		if err != nil {
			log.Warn("rsvp-te: raw IP transport unavailable, signaling disabled", "error", err)
		} else {
			eng = newEngine(t, lspTable, admission, newBusFIB(getEventBus(), log), cfg, log)
			go func() {
				eng.run(ctx)
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

		go runCleanupLoop(ctx, log, lspTable, cfg)

		tunnelsMu.Lock()
		for _, tc := range cfg.Tunnels {
			setupTunnel(log, lspTable, tc, cfg, eng)
		}
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

func setupTunnel(log *slog.Logger, lspTable *LSPTable, tc tunnelConfig, cfg rsvpteConfig, eng *engine) {
	extID := addrToUint32(cfg.RouterID)
	key := LSPKey{
		TunnelEndpoint: tc.Destination,
		TunnelID:       tc.TunnelID,
		ExtTunnelID:    extID,
		SenderAddr:     cfg.RouterID,
		LSPID:          1,
	}

	lsp, existed := lspTable.GetOrCreate(key)
	if existed && lsp.State == LSPStateUp {
		// The LSP is already up. If the configured ERO changed, trigger a
		// make-before-break reroute (RFC 3209 Section 6.1) rather than
		// disturbing the live tunnel.
		lsp.mu.Lock()
		changed := lsp.PSB != nil && !eroEqual(lsp.PSB.ERO, tc.ERO)
		lsp.mu.Unlock()
		if changed && eng != nil {
			if _, ok := eng.Reroute(key, tc.ERO); ok {
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
	lsp.PSB = &PathStateBlock{
		Session: SessionIPv4{
			TunnelEndpoint: tc.Destination,
			TunnelID:       tc.TunnelID,
			ExtTunnelID:    extID,
		},
		SenderTemplate: SenderTemplateIPv4{
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
		LabelRequest:  LabelRequest{L3PID: 0x0800},
		RefreshPeriod: cfg.RefreshPeriod,
		LastRefresh:   time.Now(),
	}
	lsp.SetState(LSPStatePathSent)
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
func eroEqual(a, b []EROHop) bool {
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

func runRefreshLoop(ctx context.Context, log *slog.Logger, lspTable *LSPTable, cfg rsvpteConfig, eng *engine) {
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

// refreshPaths re-sends PATH for every ingress LSP to maintain RFC 2205
// soft-state. When no transport is available (eng nil) it only stamps the PSB so
// local state stays consistent.
func refreshPaths(log *slog.Logger, lspTable *LSPTable, eng *engine) {
	for _, lsp := range lspTable.All() {
		// Decide eligibility and stamp the refresh under the LSP lock, then
		// release it before sendPath (which takes the same lock).
		lsp.mu.Lock()
		if lsp.PSB == nil || lsp.State == LSPStateDown {
			lsp.mu.Unlock()
			continue
		}
		lsp.PSB.LastRefresh = time.Now()
		isIngress := lsp.Role == RoleIngress
		lsp.mu.Unlock()

		if eng != nil && isIngress {
			if err := eng.sendPath(lsp); err != nil {
				log.Warn("rsvp-te: PATH refresh send failed", "lsp", lsp.Key.String(), "error", err)
			}
			continue
		}
		log.Debug("rsvp-te: PATH refresh", "lsp", lsp.Key.String())
	}
}

func runCleanupLoop(ctx context.Context, log *slog.Logger, lspTable *LSPTable, cfg rsvpteConfig) {
	ticker := time.NewTicker(cfg.RefreshPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			expired := lspTable.ExpiredPSBs(now, cfg.RefreshMultiplier)
			for _, key := range expired {
				lsp := lspTable.Remove(key)
				if lsp != nil {
					// Snapshot under the LSP lock: the engine goroutine may
					// still hold this pointer from a Get that raced the Remove.
					lsp.mu.Lock()
					inLabel := lsp.InLabel
					lsp.mu.Unlock()
					lspTable.ReleaseLabel(inLabel)
					log.Info("rsvp-te: LSP expired", "lsp", key.String())
					emitLSPDown(log, lsp, lspTable.Len())
				}
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

func showSessions(lspTable *LSPTable) any {
	type sessionInfo struct {
		TunnelEndpoint string  `json:"tunnel-endpoint"`
		TunnelID       uint16  `json:"tunnel-id"`
		LSPID          uint16  `json:"lsp-id"`
		SenderAddr     string  `json:"sender-address"`
		State          string  `json:"state"`
		Role           string  `json:"role"`
		Bandwidth      float32 `json:"bandwidth"`
		InLabel        uint32  `json:"in-label"`
		OutLabel       uint32  `json:"out-label"`
	}
	lsps := lspTable.All()
	out := make([]sessionInfo, 0, len(lsps))
	for _, lsp := range lsps {
		lsp.mu.Lock()
		out = append(out, sessionInfo{
			TunnelEndpoint: lsp.Key.TunnelEndpoint.String(),
			TunnelID:       lsp.Key.TunnelID,
			LSPID:          lsp.Key.LSPID,
			SenderAddr:     lsp.Key.SenderAddr.String(),
			State:          lsp.State.String(),
			Role:           lsp.Role.String(),
			Bandwidth:      lsp.Bandwidth,
			InLabel:        lsp.InLabel,
			OutLabel:       lsp.OutLabel,
		})
		lsp.mu.Unlock()
	}
	return out
}

func showInterfaces(admission *AdmissionController) any {
	type ifaceInfo struct {
		Name              string  `json:"name"`
		MaxBandwidth      float64 `json:"max-bandwidth"`
		MaxReservable     float64 `json:"max-reservable"`
		ReservedBandwidth float64 `json:"reserved-bandwidth"`
		Available         float64 `json:"available-bandwidth"`
	}
	ifaces := admission.AllInterfaces()
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

func showTunnels(lspTable *LSPTable) any {
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
