package flowexport

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/ddosevent"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/plugins/flowexport/enrich"
	flowexportyang "github.com/ze-software/ze/internal/plugins/flowexport/yang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

const configRootFlowExport = "flow-export"

var loggerPtr atomic.Pointer[slog.Logger]

var activeExporter atomic.Pointer[exporter]

var eventBusPtr atomic.Pointer[ze.EventBus]

func setEventBus(eb ze.EventBus) {
	if eb != nil {
		eventBusPtr.Store(&eb)
	}
}

func getEventBus() ze.EventBus {
	if p := eventBusPtr.Load(); p != nil {
		return *p
	}
	return nil
}

// getExporter returns the active exporter, or nil if not configured.
func getExporter() *exporter {
	return activeExporter.Load()
}

func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

func init() {
	loggerPtr.Store(slogutil.DiscardLogger())

	reg := registry.Registration{
		Name:        "flow-export",
		Description: "sFlow, NetFlow v9, and IPFIX counter export",
		Features:    "yang",
		YANG:        flowexportyang.ZeFlowExportConfYANG,
		ConfigRoots: []string{configRootFlowExport},
		// Counter export is driven by the interface rate tracker's per-second
		// snapshot callback (RegisterCollectNotify -> notifyFromRateTracker).
		// That tracker only runs inside the interface plugin's engine, which
		// is otherwise auto-loaded only when an `interface {}` section is
		// present. Declaring the dependency makes configuring flow-export
		// alone enough to start the tracker, so counter datagrams flow without
		// the operator having to also add an interface section.
		Dependencies: []string{"interface"},
		RunEngine:    runEngine,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg metrics.Registry) {
			BindMetrics(reg)
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			setEventBus(eb)
		},
		DoctorChecks: []registry.DoctorCheckDef{{
			Name:         "flow-export-conntrack-tracking",
			Phase:        rpc.DoctorPhasePostConfig,
			Order:        761,
			Dependencies: []string{"config-loaded"},
			Platforms:    []string{"any"},
			Codes:        []string{"doctor-flowexport-conntrack-unavailable"},
			Check:        checkConntrackTracking,
		}},
	}
	reg.CLIHandler = func(_ []string) int {
		return 1
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "flow-export: registration failed: %v\n", err)
		os.Exit(1)
	}

	RegisterHealthCheck()
}

func runEngine(conn net.Conn) int {
	log := loggerPtr.Load()
	log.Debug("flow-export plugin starting")

	p := sdk.NewWithConn("flow-export", conn)
	defer func() { _ = p.Close() }()

	// iface.SubscribeCollectNotify below registers a callback into iface's
	// package-level subscriber list as a plain Go function call, not through
	// DirectBridge/DispatchCommand -- that only reaches the engine's real
	// rate tracker (internal/component/iface's own background collect loop)
	// when this plugin shares process memory with it. It is flow-export's
	// only counter data source, unconditional, no fallback, so an external
	// flow-export would silently never export a single datagram, with no
	// error anywhere. Refuse to start rather than degrade silently.
	if !p.IsInternal() {
		log.Error("flow-export: refusing to start as an external plugin process -- the interface rate-tracker subscription (iface.SubscribeCollectNotify) is a same-process call and would silently no-op across a process boundary; configure flow-export to run internal")
		return 1
	}

	collectSubID := iface.SubscribeCollectNotify(notifyFromRateTracker)

	// configure builds (or tears down) the exporter from a parsed config.
	// Shared by OnConfigure (boot) and OnConfigApply (reload): a reload that
	// adds or changes the flow-export section is driven through verify+apply,
	// not OnConfigure, so the exporter must be (re)built here too. A nil or
	// empty cfg tears the exporter down (a reload that removed the section).
	configure := func(cfg *Config) error {
		if cfg == nil || cfg.IsEmpty() {
			if prev := activeExporter.Swap(nil); prev != nil {
				prev.stop()
				log.Info("flow-export stopped (configuration removed)")
			} else {
				log.Debug("flow-export: no configuration, plugin idle")
			}
			return nil
		}

		exp, err := newExporter(cfg)
		if err != nil {
			return fmt.Errorf("flow-export exporter: %w", err)
		}

		wireEncoders(exp, cfg)
		startFlowSubsystems(exp, cfg)

		prev := activeExporter.Swap(exp)
		if prev != nil {
			prev.stop()
		}

		log.Info("flow-export configured",
			"collectors", len(cfg.Collectors),
			"sampling", len(cfg.Sampling),
			"conntrack", cfg.Conntrack.Enabled,
			"bgp-enrichment", cfg.Enrichment.BGP)
		return nil
	}

	// parseSections extracts and validates the flow-export config from the
	// delivered sections. Returns a nil cfg when no flow-export section is
	// present (boot without the section); an empty Config when the section is
	// present but empty (a reload that removed it, delivered as "{}").
	parseSections := func(sections []sdk.ConfigSection) (*Config, error) {
		for _, s := range sections {
			if s.Root != configRootFlowExport {
				continue
			}
			cfg, err := ParseConfig(s.Data)
			if err != nil {
				return nil, fmt.Errorf("flow-export config: %w", err)
			}
			if err := cfg.Validate(); err != nil {
				return nil, fmt.Errorf("flow-export config: %w", err)
			}
			return cfg, nil
		}
		// No flow-export section present: an empty Config drives configure()
		// down the idle/teardown path, same as a nil would.
		return &Config{}, nil
	}

	// pendingCfg carries the verified reload config from OnConfigVerify into
	// OnConfigApply: the reload transaction delivers only a diff to apply, so
	// the full verified config is stashed here. Config transactions are
	// serialized by the engine, so a plain captured variable is safe.
	var pendingCfg *Config

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, err := parseSections(sections)
		if err != nil {
			return err
		}
		return configure(cfg)
	})

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		cfg, err := parseSections(sections)
		if err != nil {
			return fmt.Errorf("flow-export config verify: %w", err)
		}
		pendingCfg = cfg
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		cfg := pendingCfg
		pendingCfg = nil
		return configure(cfg)
	})

	p.OnConfigRollback(func(_ string) error {
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{configRootFlowExport},
		VerifyBudget: 2,
		ApplyBudget:  10,
	}); err != nil {
		log.Error("flow-export plugin failed", "error", err)
		iface.UnsubscribeCollectNotify(collectSubID)
		return 1
	}

	iface.UnsubscribeCollectNotify(collectSubID)

	if exp := activeExporter.Swap(nil); exp != nil {
		exp.stop()
	}
	log.Info("flow-export plugin stopped")
	return 0
}

// wireEncoders creates and assigns protocol encoders to each collector
// using the registered encoder factories. Each collector gets a counter
// encoder and, where the protocol supports it, a flow-sample (sFlow) or
// flow-record (NetFlow v9, IPFIX) encoder for spec-2 per-flow export.
func wireEncoders(exp *exporter, cfg *Config) {
	log := loggerPtr.Load()
	for _, cc := range cfg.Collectors {
		factory := lookupEncoderFactory(cc.Protocol)
		if factory == nil {
			log.Warn("flow-export: no encoder for protocol", "protocol", cc.Protocol)
			continue
		}
		exp.setEncoder(cc.Name, factory(cc, exp.startTime))

		if fsf := lookupFlowSampleFactory(cc.Protocol); fsf != nil {
			exp.setFlowSampleEncoder(cc.Name, fsf(cc, exp.startTime))
		}
		if frf := lookupFlowRecordFactory(cc.Protocol); frf != nil {
			exp.setFlowRecordEncoder(cc.Name, frf(cc, exp.startTime))
		}
	}
}

// startFlowSubsystems wires the spec-2 lifecycle: BGP RIB enrichment, packet
// sampling, and conntrack flow records. Each is optional and gated on config;
// all teardown is registered as an exporter stopper so a config reload (which
// stops the previous exporter) releases them cleanly.
func startFlowSubsystems(exp *exporter, cfg *Config) {
	if cfg.Enrichment.BGP {
		enricher := enrich.NewEnricher()
		exp.setEnricher(enricher)
		builder := newBGPEnrichBuilder(enricher)
		builder.Start(getEventBus())
		exp.addStopper(builder.Stop)
	}

	if len(cfg.Sampling) > 0 {
		sw := newSamplingWorker(exp, cfg.Sampling)
		sw.Start()
		exp.addStopper(sw.Stop)
	}

	if cfg.Conntrack.Enabled {
		cw := newConntrackWorker(exp, cfg.Conntrack)
		cw.Start()
		exp.conntrack = cw
		exp.addStopper(cw.Stop)

		// Force a fresh conntrack dump the moment an attack is detected, so the
		// recent-flow ring reflects the in-progress attack when the DDoS
		// characterizer reads it. The periodic dump cadence is the operator's
		// active-timeout (default 60s, up to an hour), far too coarse to catch an
		// attack that confirms within seconds -- without this the characterizer
		// reads a pre-attack ring and always falls back to generic-flood. flow-export
		// already consumes the shared event bus (BGP RIB enrichment); ddosevent is a
		// core event type, so this stays a plugin->core dependency.
		if eb := getEventBus(); eb != nil {
			unsub := ddosevent.Detected.Subscribe(eb, func(_ *ddosevent.AttackDetected) {
				exp.refreshConntrack()
			})
			exp.addStopper(unsub)
		}
	}
}

// notifyFromRateTracker is the callback registered with iface.RegisterCollectNotify.
// It converts raw []iface.InterfaceInfo into a CounterSnapshot and dispatches
// to the active exporter. Called from the iface rate tracker goroutine.
func notifyFromRateTracker(ifs []iface.InterfaceInfo) {
	exp := activeExporter.Load()
	if exp == nil {
		return
	}

	snap := CounterSnapshot{
		Time:       time.Now(),
		Interfaces: make([]InterfaceCounters, 0, len(ifs)),
	}
	for i := range ifs {
		info := &ifs[i]
		if info.Stats == nil || info.Index < 0 {
			continue
		}
		// Speed/duplex come from a sysfs read kept OFF the generic ListInterfaces
		// path (which the 1Hz rate tracker and every show/web/health caller hit);
		// only the flow-export snapshot needs them, so only it pays the read, and
		// only for the interfaces it actually exports.
		speedMbps, duplex := iface.LinkSpeedDuplex(info.Name)
		snap.Interfaces = append(snap.Interfaces, interfaceCountersFrom(info, speedMbps, duplex))
	}

	exp.notifySnapshot(snap)
}

// interfaceCountersFrom converts one iface.InterfaceInfo into the sFlow
// if_counters value type. Caller has already checked Stats != nil and Index >= 0
// and supplies the sysfs speed (Mbit/s) and duplex separately (they are not on
// InterfaceInfo to keep them off the generic ListInterfaces path).
// sFlow if_counters fields 7-18 are XDR unsigned int (32-bit); truncation from
// the kernel's uint64 counters is per spec. ifSpeed is the Mbit/s value scaled
// to bit/s (0 stays 0 when the kernel reports it unknown); broadcast and
// out-multicast counters are left zero because rtnl_link_stats64 does not expose
// them (see docs/guide/flow-export.md Limitations).
func interfaceCountersFrom(info *iface.InterfaceInfo, speedMbps int, duplex string) InterfaceCounters {
	ic := InterfaceCounters{
		Name:              info.Name,
		IfIndex:           uint32(info.Index),
		IfType:            ifTypeFor(info.Type),
		IfSpeed:           uint64(speedMbps) * 1_000_000, // Mbit/s -> bit/s; 0 stays 0
		IfDirection:       ifDirectionFor(duplex),
		IfInOctets:        info.Stats.RxBytes,
		IfInUcastPkts:     uint32(info.Stats.RxPackets),
		IfInMulticastPkts: uint32(info.Stats.RxMulticast),
		IfInDiscards:      uint32(info.Stats.RxDropped),
		IfInErrors:        uint32(info.Stats.RxErrors),
		IfOutOctets:       info.Stats.TxBytes,
		IfOutUcastPkts:    uint32(info.Stats.TxPackets),
		IfOutDiscards:     uint32(info.Stats.TxDropped),
		IfOutErrors:       uint32(info.Stats.TxErrors),
	}
	if info.Promisc {
		ic.IfPromiscuousMode = 1
	}
	switch info.State {
	case "up":
		ic.IfStatus = IfStatusAdminUp | IfStatusOperUp
	case "down":
		ic.IfStatus = IfStatusAdminUp
	}
	return ic
}

// ifDirectionFor maps the sysfs duplex string to the sFlow if_counters
// ifDirection field. Empty / unknown duplex (virtual or down links) maps to
// IfDirectionUnknown, leaving the field zero per the sFlow spec.
func ifDirectionFor(duplex string) uint32 {
	switch duplex {
	case "full":
		return IfDirectionFullDuplex
	case "half":
		return IfDirectionHalfDuplex
	default:
		return IfDirectionUnknown
	}
}

// ifTypeFor maps a kernel link type (link.Type()) to an IANA ifType value for
// the sFlow if_counters ifType field. Unknown kinds map to 1 (other). The
// kernel reports loopback as a plain device, so "lo" is not specially
// distinguished here; collectors key on ethernet vs tunnel, which this covers.
func ifTypeFor(linkType string) uint32 {
	switch linkType {
	case "device":
		return 6 // ethernetCsmacd
	case "bridge":
		return 209 // bridge
	case "vlan":
		return 135 // l2vlan
	case "veth", "dummy", "tuntap":
		return 53 // propVirtual
	case "gre", "gretap", "ip6gre", "ip6gretap", "ipip", "sit", "ip6tnl", "vti", "vti6", "wireguard":
		return 131 // tunnel
	default:
		return 1 // other
	}
}
