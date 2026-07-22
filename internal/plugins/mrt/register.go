package mrt

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	mrtyang "codeberg.org/thomas-mangin/ze/internal/plugins/mrt/yang"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

const configRoot = "mrt"

// The RIB dump provider is NOT stored here: it lives in the leaf
// registry (registry.SetRIBDumpCallback / GetRIBDumpCallback), where the BGP
// RIB plugin publishes it from its own init(). MRT reads it at component start.
// That indirection is what lets internal/component/bgp be compiled out
// (//go:build ze_bgp) without MRT or the hub naming a BGP package.
var (
	loggerPtr   atomic.Pointer[slog.Logger]
	eventBusPtr atomic.Pointer[ze.EventBus]
	activeComp  atomic.Pointer[Component]
)

// MessageBridge implements registry.MessageCallback by forwarding to the
// active MRT Component. Registered as coordinator extra "mrt.messageCallback"
// so the BGP reactor factory can wire it during createReactorFromCoordinator.
// When no MRT config is active, OnBGPMessage returns immediately (nil check).
var MessageBridge messageBridge

type messageBridge struct{}

func (messageBridge) OnBGPMessage(peer any, msgType uint8, sent bool, rawBytes []byte) {
	comp := activeComp.Load()
	if comp == nil {
		return
	}
	comp.onBGPMessageAny(peer, msgType, sent, rawBytes)
}

// PeerBridge implements registry.PeerLifecycleCallback by forwarding to the
// active MRT Component for FSM state change recording.
var PeerBridge peerBridge

type peerBridge struct{}

func (peerBridge) OnPeerEstablished(peer any) {
	comp := activeComp.Load()
	if comp == nil {
		return
	}
	comp.onPeerEstablished(peer)
}

func (peerBridge) OnPeerClosed(peer any, _ string) {
	comp := activeComp.Load()
	if comp == nil {
		return
	}
	comp.onPeerClosed(peer)
}

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
	if p := eventBusPtr.Load(); p != nil {
		return *p
	}
	return nil
}

func init() {
	loggerPtr.Store(slogutil.DiscardLogger())

	reg := registry.Registration{
		Name:        "mrt",
		Description: "MRT routing information export (RFC 6396)",
		Features:    "yang",
		YANG:        mrtyang.ZeMRTConfYANG,
		ConfigRoots: []string{configRoot},
		RunEngine:   runEngine,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			setEventBus(eb)
		},
	}
	reg.CLIHandler = func(_ []string) int { return 1 }
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "mrt: registration failed: %v\n", err)
		os.Exit(1)
	}

	// Self-register the reactor bridges into the leaf registry seam. The BGP
	// reactor factory (bgp/config) reads them via registry.GetMRTMessageCallback /
	// GetMRTPeerCallback, so cmd/ze/hub no longer imports this package directly --
	// which is what lets //go:build ze_mrt drop MRT from the binary.
	registry.SetMRTMessageCallback(MessageBridge)
	registry.SetMRTPeerCallback(PeerBridge)
}

func runEngine(conn net.Conn) int {
	log := loggerPtr.Load()
	log.Debug("mrt plugin starting")

	p := sdk.NewWithConn("mrt", conn)
	defer func() { _ = p.Close() }()

	configure := func(cfg *Config) error { //nolint:unparam // matches sdk callback signature pattern
		if prev := activeComp.Swap(nil); prev != nil {
			prev.Stop()
		}
		if cfg == nil || cfg.IsEmpty() {
			log.Debug("mrt: no configuration, plugin idle")
			return nil
		}
		comp := New(*cfg, log)
		if cb := registry.GetRIBDumpCallback(); cb != nil {
			comp.ribDumper = cb
		}
		comp.Start(getEventBus())
		activeComp.Store(comp)
		log.Info("mrt configured")
		return nil
	}

	parseSections := func(sections []sdk.ConfigSection) (*Config, error) {
		for _, s := range sections {
			if s.Root != configRoot {
				continue
			}
			cfg, err := ParseConfig(json.RawMessage(s.Data))
			if err != nil {
				return nil, fmt.Errorf("mrt config: %w", err)
			}
			return cfg, nil
		}
		return &Config{}, nil
	}

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
			return fmt.Errorf("mrt config verify: %w", err)
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

	p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, any, error) {
		switch command {
		case "request mrt dump-rib":
			comp := activeComp.Load()
			if comp == nil {
				return "mrt not configured", nil, nil
			}
			comp.dumpRIB()
			return "rib dump triggered", nil, nil
		default:
			return "", nil, nil
		}
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{configRoot},
		VerifyBudget: 2,
		ApplyBudget:  10,
		Commands:     []sdk.CommandDecl{{Name: "request mrt dump-rib"}},
	}); err != nil {
		log.Error("mrt plugin failed", "error", err)
		return 1
	}

	if comp := activeComp.Swap(nil); comp != nil {
		comp.Stop()
	}
	log.Info("mrt plugin stopped")
	return 0
}

// ParseConfig parses MRT config from JSON section data.
//
// The plugin server delivers a section wrapped in the config root, e.g.
// {"mrt":{"updates":{...}}} (BuildPluginConfigSections keys the body by root,
// as static/fib/etc. all unwrap). Unwrap the "mrt" key before decoding the
// body; a body that is already unwrapped (no "mrt" key) is decoded as-is.
func ParseConfig(data json.RawMessage) (*Config, error) {
	if wrapper := map[string]json.RawMessage{}; json.Unmarshal(data, &wrapper) == nil {
		if inner, ok := wrapper[configRoot]; ok {
			data = inner
		}
	}

	var raw struct {
		ExtendedTimestamp *bool    `json:"extended-timestamp"`
		AddPath           *bool    `json:"add-path"`
		PeerFilter        []string `json:"peer-filter"`
		Direction         string   `json:"direction"`
		Updates           *struct {
			File     string `json:"file"`
			Interval int    `json:"interval"`
		} `json:"updates"`
		All *struct {
			File     string `json:"file"`
			Interval int    `json:"interval"`
		} `json:"all"`
		Routes *struct {
			File     string `json:"file"`
			Interval int    `json:"interval"`
		} `json:"routes"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	cfg := &Config{}
	if raw.ExtendedTimestamp != nil {
		cfg.ExtendedTimestamp = *raw.ExtendedTimestamp
	}
	if raw.AddPath != nil {
		cfg.AddPath = *raw.AddPath
	}
	cfg.PeerFilter = raw.PeerFilter
	if raw.Direction != "" && raw.Direction != "both" {
		cfg.Direction = raw.Direction
	}
	if raw.Updates != nil {
		cfg.UpdatesPath = raw.Updates.File
		cfg.UpdatesInterval = time.Duration(raw.Updates.Interval) * time.Second
	}
	if raw.All != nil {
		cfg.AllPath = raw.All.File
		cfg.AllInterval = time.Duration(raw.All.Interval) * time.Second
	}
	if raw.Routes != nil {
		cfg.RoutesPath = raw.Routes.File
		cfg.RoutesInterval = time.Duration(raw.Routes.Interval) * time.Second
	}
	return cfg, nil
}
