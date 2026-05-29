package flowexport

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/flowexport/enrich"
	flowexportschema "codeberg.org/thomas-mangin/ze/internal/component/flowexport/schema"
	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

const configRootFlowExport = "flow-export"

var loggerPtr atomic.Pointer[slog.Logger]

var activeExporter atomic.Pointer[Exporter]

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

// GetExporter returns the active exporter, or nil if not configured.
func GetExporter() *Exporter {
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
		YANG:        flowexportschema.ZeFlowExportConfYANG,
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
		ConfigureMetrics: func(reg any) {
			if r, ok := reg.(metrics.Registry); ok {
				BindMetrics(r)
			}
		},
		ConfigureEventBus: func(eb any) {
			if e, ok := eb.(ze.EventBus); ok {
				setEventBus(e)
			}
		},
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

	iface.RegisterCollectNotify(notifyFromRateTracker)

	// configure builds (or tears down) the exporter from a parsed config.
	// Shared by OnConfigure (boot) and OnConfigApply (reload): a reload that
	// adds or changes the flow-export section is driven through verify+apply,
	// not OnConfigure, so the exporter must be (re)built here too. A nil or
	// empty cfg tears the exporter down (a reload that removed the section).
	configure := func(cfg *Config) error {
		if cfg == nil || cfg.IsEmpty() {
			if prev := activeExporter.Swap(nil); prev != nil {
				prev.Stop()
				log.Info("flow-export stopped (configuration removed)")
			} else {
				log.Debug("flow-export: no configuration, plugin idle")
			}
			return nil
		}

		exp, err := NewExporter(cfg)
		if err != nil {
			return fmt.Errorf("flow-export exporter: %w", err)
		}

		wireEncoders(exp, cfg)
		startFlowSubsystems(exp, cfg)

		prev := activeExporter.Swap(exp)
		if prev != nil {
			prev.Stop()
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
		return 1
	}

	iface.RegisterCollectNotify(nil)

	if exp := activeExporter.Swap(nil); exp != nil {
		exp.Stop()
	}
	log.Info("flow-export plugin stopped")
	return 0
}

// wireEncoders creates and assigns protocol encoders to each collector
// using the registered encoder factories. Each collector gets a counter
// encoder and, where the protocol supports it, a flow-sample (sFlow) or
// flow-record (NetFlow v9, IPFIX) encoder for spec-2 per-flow export.
func wireEncoders(exp *Exporter, cfg *Config) {
	log := loggerPtr.Load()
	for _, cc := range cfg.Collectors {
		factory := lookupEncoderFactory(cc.Protocol)
		if factory == nil {
			log.Warn("flow-export: no encoder for protocol", "protocol", cc.Protocol)
			continue
		}
		exp.SetEncoder(cc.Name, factory(cc, exp.startTime))

		if fsf := lookupFlowSampleFactory(cc.Protocol); fsf != nil {
			exp.SetFlowSampleEncoder(cc.Name, fsf(cc, exp.startTime))
		}
		if frf := lookupFlowRecordFactory(cc.Protocol); frf != nil {
			exp.SetFlowRecordEncoder(cc.Name, frf(cc, exp.startTime))
		}
	}
}

// startFlowSubsystems wires the spec-2 lifecycle: BGP RIB enrichment, packet
// sampling, and conntrack flow records. Each is optional and gated on config;
// all teardown is registered as an exporter stopper so a config reload (which
// stops the previous exporter) releases them cleanly.
func startFlowSubsystems(exp *Exporter, cfg *Config) {
	if cfg.Enrichment.BGP {
		enricher := enrich.NewEnricher()
		exp.SetEnricher(enricher)
		builder := newBGPEnrichBuilder(enricher)
		builder.Start(getEventBus())
		exp.AddStopper(builder.Stop)
	}

	if len(cfg.Sampling) > 0 {
		sw := newSamplingWorker(exp, cfg.Sampling)
		sw.Start()
		exp.AddStopper(sw.Stop)
	}

	if cfg.Conntrack.Enabled {
		cw := newConntrackWorker(exp, cfg.Conntrack)
		cw.Start()
		exp.AddStopper(cw.Stop)
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

	exp.NotifySnapshot(snap)
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
