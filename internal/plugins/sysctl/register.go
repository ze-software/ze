package sysctl

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net"
	"os"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sysctlschema "codeberg.org/thomas-mangin/ze/internal/plugins/sysctl/schema"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// eventBusMu guards eventBusRef. An interface cannot be stored in
// atomic.Pointer directly, so a mutex is used (same pattern as iface).
var (
	eventBusMu  sync.Mutex
	eventBusRef ze.EventBus
)

func setEventBusRef(eb ze.EventBus) {
	eventBusMu.Lock()
	defer eventBusMu.Unlock()
	eventBusRef = eb
}

func getEventBusRef() ze.EventBus {
	eventBusMu.Lock()
	defer eventBusMu.Unlock()
	return eventBusRef
}

func init() {
	reg := registry.Registration{
		Name:        "sysctl",
		Description: "Kernel tunable management: three-layer precedence, restore on stop",
		Features:    "yang",
		YANG:        sysctlschema.ZeSysctlConfYANG,
		ConfigRoots: []string{"sysctl"},
		RunEngine:   runSysctlPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureEventBus: func(eb any) {
			if e, ok := eb.(ze.EventBus); ok {
				setEventBusRef(e)
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
		fmt.Fprintf(os.Stderr, "sysctl: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func runSysctlPlugin(conn net.Conn) int {
	log := logger()
	log.Debug("sysctl plugin starting")

	p := sdk.NewWithConn("sysctl", conn)
	defer func() { _ = p.Close() }()

	be := newBackend()
	s := newStore(be, log)

	eb := getEventBusRef()

	// Subscribe to EventBus events.
	var unsubscribers []func()
	if eb != nil {
		unsubscribers = append(unsubscribers,
			// Default events from other plugins.
			eb.Subscribe(plugin.NamespaceSysctl, plugin.EventSysctlDefault, func(payload string) {
				var ev struct {
					Key    string `json:"key"`
					Value  string `json:"value"`
					Source string `json:"source"`
				}
				if err := json.Unmarshal([]byte(payload), &ev); err != nil {
					log.Warn("sysctl: bad default event", "err", err)
					return
				}
				applied, err := s.setDefault(ev.Key, ev.Value, ev.Source)
				if err != nil {
					log.Warn("sysctl: default write failed", "key", ev.Key, "err", err)
					return
				}
				if applied != "" {
					if _, emitErr := eb.Emit(plugin.NamespaceSysctl, plugin.EventSysctlApplied, applied); emitErr != nil {
						log.Debug("sysctl: applied emit failed", "err", emitErr)
					}
				}
			}),
			// Transient set events (from CLI).
			eb.Subscribe(plugin.NamespaceSysctl, plugin.EventSysctlSet, func(payload string) {
				var ev struct {
					Key   string `json:"key"`
					Value string `json:"value"`
				}
				if err := json.Unmarshal([]byte(payload), &ev); err != nil {
					log.Warn("sysctl: bad set event", "err", err)
					return
				}
				applied, err := s.setTransient(ev.Key, ev.Value)
				if err != nil {
					log.Warn("sysctl: set failed", "key", ev.Key, "err", err)
					return
				}
				if applied != "" {
					if _, emitErr := eb.Emit(plugin.NamespaceSysctl, plugin.EventSysctlApplied, applied); emitErr != nil {
						log.Debug("sysctl: applied emit failed", "err", emitErr)
					}
				}
			}),
			// Query: show active keys.
			eb.Subscribe(plugin.NamespaceSysctl, plugin.EventSysctlShowRequest, func(payload string) {
				var req struct {
					RequestID string `json:"request-id"`
				}
				_ = json.Unmarshal([]byte(payload), &req)
				result := s.showEntries()
				resp, _ := json.Marshal(struct {
					RequestID string `json:"request-id"`
					Entries   string `json:"entries"`
				}{RequestID: req.RequestID, Entries: result})
				if _, err := eb.Emit(plugin.NamespaceSysctl, plugin.EventSysctlShowResult, string(resp)); err != nil {
					log.Debug("sysctl: show-result emit failed", "err", err)
				}
			}),
			// Query: list known keys.
			eb.Subscribe(plugin.NamespaceSysctl, plugin.EventSysctlListRequest, func(payload string) {
				var req struct {
					RequestID string `json:"request-id"`
				}
				_ = json.Unmarshal([]byte(payload), &req)
				result := listKnownKeys()
				resp, _ := json.Marshal(struct {
					RequestID string `json:"request-id"`
					Entries   string `json:"entries"`
				}{RequestID: req.RequestID, Entries: result})
				if _, err := eb.Emit(plugin.NamespaceSysctl, plugin.EventSysctlListResult, string(resp)); err != nil {
					log.Debug("sysctl: list-result emit failed", "err", err)
				}
			}),
			// Query: describe one key.
			eb.Subscribe(plugin.NamespaceSysctl, plugin.EventSysctlDescribeRequest, func(payload string) {
				var req struct {
					RequestID string `json:"request-id"`
					Key       string `json:"key"`
				}
				_ = json.Unmarshal([]byte(payload), &req)
				result := s.describeKey(req.Key)
				resp, _ := json.Marshal(struct {
					RequestID string `json:"request-id"`
					Detail    string `json:"detail"`
				}{RequestID: req.RequestID, Detail: result})
				if _, err := eb.Emit(plugin.NamespaceSysctl, plugin.EventSysctlDescribeResult, string(resp)); err != nil {
					log.Debug("sysctl: describe-result emit failed", "err", err)
				}
			}),
		)
	}

	// applyFromJSON parses sysctl config JSON and applies settings.
	applyFromJSON := func(data string) error {
		settings := parseSysctlConfig(data)
		if len(settings) == 0 {
			return nil
		}
		applied, errs := s.applyConfig(settings)
		if len(errs) > 0 {
			return fmt.Errorf("sysctl config: %w", errs[0])
		}
		if eb != nil {
			for _, payload := range applied {
				if _, emitErr := eb.Emit(plugin.NamespaceSysctl, plugin.EventSysctlApplied, payload); emitErr != nil {
					log.Debug("sysctl: applied emit failed", "err", emitErr)
				}
			}
		}
		log.Info("sysctl config applied", "keys", len(settings))
		return nil
	}

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, sec := range sections {
			if sec.Root != "sysctl" {
				continue
			}
			return applyFromJSON(sec.Data)
		}
		return nil
	})

	p.OnConfigVerify(func(_ []sdk.ConfigSection) error {
		return nil
	})

	p.OnConfigApply(func(sections []sdk.ConfigDiffSection) error {
		for _, sec := range sections {
			if sec.Root != "sysctl" {
				continue
			}
			// Merge Added and Changed into one settings map. Keys absent
			// from the result cause applyConfig to clear the config layer
			// (handling Removed implicitly).
			settings := make(map[string]string)
			for _, data := range []string{sec.Added, sec.Changed} {
				maps.Copy(settings, parseSysctlConfig(data))
			}
			applied, errs := s.applyConfig(settings)
			if len(errs) > 0 {
				return fmt.Errorf("sysctl config: %w", errs[0])
			}
			if eb != nil {
				for _, payload := range applied {
					if _, emitErr := eb.Emit(plugin.NamespaceSysctl, plugin.EventSysctlApplied, payload); emitErr != nil {
						log.Debug("sysctl: applied emit failed", "err", emitErr)
					}
				}
			}
			log.Info("sysctl config reloaded", "keys", len(settings))
			return nil
		}
		return nil
	})

	p.OnConfigRollback(func(_ string) error {
		log.Info("sysctl config rolled back")
		return nil
	})

	p.OnStarted(func(_ context.Context) error {
		log.Info("sysctl plugin started")
		return nil
	})

	const (
		statusDone  = "done"
		statusError = "error"
	)

	p.OnExecuteCommand(func(_, command string, args []string, _ string) (string, string, error) {
		switch command {
		case "sysctl show":
			return statusDone, s.showEntries(), nil
		case "sysctl list":
			return statusDone, listKnownKeys(), nil
		case "sysctl describe":
			if len(args) < 1 {
				return statusError, "", fmt.Errorf("sysctl describe: requires key argument")
			}
			return statusDone, s.describeKey(args[0]), nil
		case "sysctl set":
			if len(args) < 2 {
				return statusError, "", fmt.Errorf("sysctl set: requires key and value arguments")
			}
			applied, err := s.setTransient(args[0], args[1])
			if err != nil {
				return statusError, "", fmt.Errorf("sysctl set %s: %w", args[0], err)
			}
			if applied != "" && eb != nil {
				if _, emitErr := eb.Emit(plugin.NamespaceSysctl, plugin.EventSysctlApplied, applied); emitErr != nil {
					log.Debug("sysctl: applied emit failed", "err", emitErr)
				}
			}
			return statusDone, applied, nil
		}
		return statusError, "", fmt.Errorf("unknown command: %s", command)
	})

	ctx := context.Background()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{"sysctl"},
		VerifyBudget: 1,
		ApplyBudget:  1,
		Commands: []sdk.CommandDecl{
			{Name: "sysctl show", Description: "Show all active sysctl keys with source and persistence"},
			{Name: "sysctl list", Description: "List all known sysctl keys with descriptions"},
			{Name: "sysctl describe", Description: "Show detail for one sysctl key", Args: []string{"key"}},
			{Name: "sysctl set", Description: "Set a transient sysctl value", Args: []string{"key", "value"}},
		},
	})
	if err != nil {
		log.Error("sysctl plugin failed", "error", err)
		return 1
	}

	// Unsubscribe event handlers.
	for _, unsub := range unsubscribers {
		unsub()
	}

	// Restore original values on clean stop.
	s.restoreAll()

	return 0
}
