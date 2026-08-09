// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RSVP-TE component registration
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

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/events"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	rsvpteyang "github.com/ze-software/ze/internal/plugins/rsvpte/yang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
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
	localRepairs    metrics.Counter // RFC 4090 facility-backup local repairs performed
	protectedLSPs   metrics.Gauge   // transit LSPs with an armed bypass
	bypassLSPs      metrics.Gauge   // configured facility-backup bypass LSPs
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
		localRepairs:    reg.Counter("ze_rsvpte_local_repairs_total", "Total RFC 4090 facility-backup local repairs performed."),
		protectedLSPs:   reg.Gauge("ze_rsvpte_protected_lsps", "Current number of transit LSPs with an armed fast-reroute bypass."),
		bypassLSPs:      reg.Gauge("ze_rsvpte_bypass_lsps", "Current number of configured facility-backup bypass LSPs."),
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
	Bypasses          []bypassConfig
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
	FastReroute   *frrTunnelConfig // RFC 4090 local protection, nil when not requested
}

// frrTunnelConfig is the parsed `fast-reroute` container on a tunnel (RFC 4090).
type frrTunnelConfig struct {
	OneToOne            bool // false = facility backup (the default)
	NodeProtection      bool
	BandwidthProtection bool
	HopLimit            uint8
}

// protection builds the wire-level protectionRequest the head-end PSB carries.
func (fr *frrTunnelConfig) protection(tc tunnelConfig) *protectionRequest {
	return &protectionRequest{
		Facility:            !fr.OneToOne,
		NodeProtection:      fr.NodeProtection,
		BandwidthProtection: fr.BandwidthProtection,
		HopLimit:            fr.HopLimit,
		Bandwidth:           tc.Bandwidth,
		SetupPrio:           tc.SetupPriority,
		HoldPrio:            tc.HoldPriority,
		Name:                tc.Name,
	}
}

// bypassConfig is a configured facility-backup bypass LSP (RFC 4090 Section 3.2):
// an ordinary RSVP-TE LSP from this PLR to MergePoint, routed (via ERO) to avoid
// the protected resource.
type bypassConfig struct {
	Name           string
	MergePoint     netip.Addr
	NodeProtection bool
	ERO            []eroHop
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

// rsvpteBool coerces a config-tree scalar to a bool. Tree.ToMap renders a YANG
// boolean leaf as the string "true"/"false", so accept both that and a native
// bool for robustness.
func rsvpteBool(v any) bool {
	switch b := v.(type) {
	case bool:
		return b
	case string:
		return b == "true"
	default:
		return false
	}
}

// parseERO reads the explicit-route list from a tunnel or bypass entry, sorted by
// numeric hop index (ordered-by user in YANG, keyed by index here).
func parseERO(m map[string]any) ([]eroHop, error) {
	entries := rsvpteList(m["explicit-route"], true)
	ero := make([]eroHop, 0, len(entries))
	for _, hopEntry := range entries {
		hopMap := hopEntry.data
		hop := eroHop{}
		if v, ok := hopMap["address"].(string); ok && v != "" {
			pfx, err := netip.ParsePrefix(v)
			if err != nil {
				return nil, fmt.Errorf("invalid explicit-route address %q: %w", v, err)
			}
			hop.Address = pfx
		}
		if v, ok := hopMap["type"].(string); ok && v == "loose" {
			hop.Loose = true
		}
		ero = append(ero, hop)
	}
	return ero, nil
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
			// RSVP-TE here is IPv4-only: the lspKey ExtTunnelID and the bypass
			// tunnel-id derive from addrToUint32(router-id), whose As4() panics on a
			// non-IPv4 address. Reject it at parse time rather than crash later.
			if !addr.Is4() {
				return cfg, fmt.Errorf("rsvp-te: router-id must be an IPv4 address, got %q", v)
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
			if v, ok := m["destination"].(string); ok && v != "" {
				addr, err := netip.ParseAddr(v)
				if err != nil {
					return cfg, fmt.Errorf("rsvp-te: tunnel %s invalid destination %q: %w", tc.Name, v, err)
				}
				tc.Destination = addr
			}
			if v, ok := rsvpteNumber(m["tunnel-id"]); ok {
				// Range-check before the uint16 cast: a value outside 0-65535 would
				// silently wrap (65536 -> 0) and could alias another tunnel-id or slip
				// past the reserved-range check below. YANG bounds it to uint16, but
				// the parser self-validates rather than rely on that.
				if v < 0 || v > 65535 {
					return cfg, fmt.Errorf("rsvp-te: tunnel %s tunnel-id %v out of range (0-65535)", tc.Name, v)
				}
				if uint16(v) >= bypassTunnelIDBase {
					return cfg, fmt.Errorf("rsvp-te: tunnel %s tunnel-id %d is in the reserved fast-reroute bypass range (>= %d)", tc.Name, uint16(v), bypassTunnelIDBase)
				}
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
			ero, err := parseERO(m)
			if err != nil {
				return cfg, fmt.Errorf("rsvp-te: tunnel %s %w", tc.Name, err)
			}
			tc.ERO = ero
			if frRaw, ok := m["fast-reroute"].(map[string]any); ok {
				fr := &frrTunnelConfig{HopLimit: 16}
				if v, ok := frRaw["backup"].(string); ok && v == "one-to-one" {
					fr.OneToOne = true
				}
				fr.NodeProtection = rsvpteBool(frRaw["node-protection"])
				fr.BandwidthProtection = rsvpteBool(frRaw["bandwidth-protection"])
				if v, ok := rsvpteNumber(frRaw["hop-limit"]); ok {
					fr.HopLimit = uint8(v)
				}
				tc.FastReroute = fr
			}
			cfg.Tunnels = append(cfg.Tunnels, tc)
		}
		for _, entry := range rsvpteList(tree["bypass"], false) {
			m := entry.data
			bc := bypassConfig{Name: entry.key}
			if v, ok := m["name"].(string); ok && v != "" {
				bc.Name = v
			}
			if v, ok := m["merge-point"].(string); ok && v != "" {
				addr, err := netip.ParseAddr(v)
				if err != nil {
					return cfg, fmt.Errorf("rsvp-te: bypass %s invalid merge-point %q: %w", bc.Name, v, err)
				}
				bc.MergePoint = addr
			}
			bc.NodeProtection = rsvpteBool(m["node-protection"])
			bypassERO, err := parseERO(m)
			if err != nil {
				return cfg, fmt.Errorf("rsvp-te: bypass %s %w", bc.Name, err)
			}
			bc.ERO = bypassERO
			cfg.Bypasses = append(cfg.Bypasses, bc)
		}
	}
	// Bound the bypass set and reject name-hash collisions so two bypasses cannot
	// signal under the same lspKey (RFC 4090 facility backup, stable keying).
	if err := validateBypasses(cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// maxBypasses bounds the number of configured facility-backup bypass LSPs. The
// name-hash key space is 4096 (12 bits over the reserved tunnel-id range), so
// well before that, collisions force a rename; this is a coarse upper guard.
const maxBypasses = 1024

// validateBypasses rejects too many bypasses and any two bypasses whose name hash
// to the same reserved lspKey (which would make them indistinguishable).
func validateBypasses(cfg rsvpteConfig) error {
	if len(cfg.Bypasses) > maxBypasses {
		return fmt.Errorf("rsvp-te: too many bypass LSPs (%d > %d)", len(cfg.Bypasses), maxBypasses)
	}
	// bypassKey derives a tunnel-id from the router-id (addrToUint32 -> As4), which
	// panics on the zero Addr. Without a router-id the engine stays idle and
	// bypasses never signal, so there is nothing to validate.
	if !cfg.RouterID.IsValid() {
		return nil
	}
	seen := make(map[lspKey]string, len(cfg.Bypasses))
	for _, bc := range cfg.Bypasses {
		if !bc.MergePoint.IsValid() {
			continue
		}
		k := bypassKey(bc, cfg.RouterID)
		if prev, dup := seen[k]; dup {
			return fmt.Errorf("rsvp-te: bypass %q and %q collide on the same key; rename one", prev, bc.Name)
		}
		seen[k] = bc.Name
	}
	return nil
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
		ConfigureMetrics: func(reg metrics.Registry) {
			setMetricsRegistry(reg)
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			setEventBus(eb)
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
		// Push the reloaded config into the running engine so its runtime reads
		// (selectBypass, admissionInterface, message builders) see the new
		// interfaces/bypasses/refresh period; otherwise FRR keeps arming against the
		// stale startup bypass set after a reload.
		if eng != nil {
			// router-id is restart-class: setConfig keeps the engine's. A configured
			// change is ignored (and logged) so we never split-brain. Reconcile against
			// the engine's EFFECTIVE config so tunnel/bypass keys match the engine's
			// runtime reads (selectBypass) rather than the new-but-ignored router-id.
			if cfg.RouterID.IsValid() && cfg.RouterID != eng.cfg().RouterID {
				log.Warn("rsvp-te: router-id change requires a restart; keeping the running router-id",
					"running", eng.cfg().RouterID, "configured", cfg.RouterID)
			}
			eng.setConfig(cfg)
			cfg = eng.cfg()
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
		case "show rsvp-te fast-reroute":
			return cmdDone, showFastReroute(lspTable), nil
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
			{Name: "show rsvp-te fast-reroute"},
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
	// Tunnel and bypass keys derive from the router-id (addrToUint32 -> As4), which
	// panics on the zero Addr. Without a router-id the engine is idle and nothing
	// can be keyed/signaled. OnStarted guards this before calling us, but
	// OnConfigApply (reload) does not, so guard here to cover every caller.
	if !cfg.RouterID.IsValid() {
		return prev
	}
	next := make(map[lspKey]bool, len(cfg.Tunnels)+len(cfg.Bypasses))
	for _, tc := range cfg.Tunnels {
		setupTunnel(log, lspTable, tc, cfg, eng)
		next[tunnelKey(tc, cfg.RouterID)] = true
	}
	// RFC 4090 Section 3.2: signal configured facility-backup bypass LSPs alongside
	// protected tunnels so a PLR has a ready bypass to redirect onto on a failure.
	for _, bc := range cfg.Bypasses {
		setupBypass(log, lspTable, bc, cfg, eng)
		next[bypassKey(bc, cfg.RouterID)] = true
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
	updateFRRGauges(lspTable)
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
	// RFC 4090: when the tunnel requests fast-reroute, the head-end PATH carries
	// SESSION_ATTRIBUTE (protection-desired flags) and FAST_REROUTE so transit
	// PLRs arm a backup.
	if tc.FastReroute != nil {
		lsp.PSB.Protection = tc.FastReroute.protection(tc)
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

// setupBypass signals a configured facility-backup bypass LSP (RFC 4090 Section
// 3.2): an ordinary ingress LSP from this PLR to the bypass merge point along the
// configured ERO. It mirrors setupTunnel but marks the LSP as a bypass so it is
// not treated as a protected tunnel, and reserves no bandwidth of its own (the
// protected LSPs already account theirs). Without a transport (eng nil) the LSP
// stays in path-sent as local intent only.
func setupBypass(log *slog.Logger, lspTable *lspTable, bc bypassConfig, cfg rsvpteConfig, eng *engine) {
	if !bc.MergePoint.IsValid() {
		return
	}
	key := bypassKey(bc, cfg.RouterID)
	lsp, existed := lspTable.GetOrCreate(key)
	if existed && lsp.State == LSPStateUp {
		return
	}

	lsp.mu.Lock()
	lsp.Role = RoleIngress
	lsp.IsBypass = true
	lsp.PSB = &pathStateBlock{
		Session: sessionIPv4{
			TunnelEndpoint: bc.MergePoint,
			TunnelID:       key.TunnelID,
			ExtTunnelID:    key.ExtTunnelID,
		},
		SenderTemplate: senderTemplateIPv4{SenderAddr: cfg.RouterID, LSPID: 1},
		ERO:            bc.ERO,
		SenderTSpec: FlowSpec{
			MinPolicedUnit: 20,
			MaxPacketSize:  65535,
		},
		LabelRequest:  labelRequest{L3PID: 0x0800},
		RefreshPeriod: cfg.RefreshPeriod,
		LastRefresh:   time.Now(),
	}
	lsp.setState(LSPStatePathSent)
	lsp.mu.Unlock()

	if eng != nil {
		if err := eng.sendPath(lsp); err != nil {
			log.Warn("rsvp-te: bypass PATH send failed", "name", bc.Name, "merge-point", bc.MergePoint, "error", err)
		}
	}
	log.Info("rsvp-te: bypass configured", "name", bc.Name, "merge-point", bc.MergePoint, "node-protection", bc.NodeProtection)
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
	updateFRRGauges(lspTable)
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

// The `show rsvp-te ...` data builders (showSessions/showInterfaces/showTunnels)
// live in show_data.go.
