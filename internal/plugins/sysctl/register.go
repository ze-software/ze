package sysctl

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net"
	"os"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sysctlreg "codeberg.org/thomas-mangin/ze/internal/core/sysctl"
	sysctlevents "codeberg.org/thomas-mangin/ze/internal/plugins/sysctl/events"
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
	_ = events.RegisterNamespace(sysctlevents.Namespace,
		sysctlevents.EventDefault, sysctlevents.EventSet, sysctlevents.EventApplied,
		sysctlevents.EventShowRequest, sysctlevents.EventShowResult,
		sysctlevents.EventListRequest, sysctlevents.EventListResult,
		sysctlevents.EventDescribeRequest, sysctlevents.EventDescribeResult,
		sysctlevents.EventClearProfileDefaults,
	)

	reg := registry.Registration{
		Name:        "sysctl",
		Description: "Kernel tunable management: three-layer precedence, restore on stop",
		Features:    "yang",
		YANG:        sysctlschema.ZeSysctlConfYANG,
		ConfigRoots: []string{configRoot},
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

const configRoot = "sysctl"

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
			eb.Subscribe(sysctlevents.Namespace, sysctlevents.EventDefault, func(payload string) {
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
					if _, emitErr := eb.Emit(sysctlevents.Namespace, sysctlevents.EventApplied, applied); emitErr != nil {
						log.Debug("sysctl: applied emit failed", "err", emitErr)
					}
				}
			}),
			// Transient set events (from CLI).
			eb.Subscribe(sysctlevents.Namespace, sysctlevents.EventSet, func(payload string) {
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
					if _, emitErr := eb.Emit(sysctlevents.Namespace, sysctlevents.EventApplied, applied); emitErr != nil {
						log.Debug("sysctl: applied emit failed", "err", emitErr)
					}
				}
			}),
			// Query: show active keys.
			eb.Subscribe(sysctlevents.Namespace, sysctlevents.EventShowRequest, func(payload string) {
				var req struct {
					RequestID string `json:"request-id"`
				}
				_ = json.Unmarshal([]byte(payload), &req)
				result := s.showEntries()
				resp, _ := json.Marshal(struct {
					RequestID string `json:"request-id"`
					Entries   string `json:"entries"`
				}{RequestID: req.RequestID, Entries: result})
				if _, err := eb.Emit(sysctlevents.Namespace, sysctlevents.EventShowResult, string(resp)); err != nil {
					log.Debug("sysctl: show-result emit failed", "err", err)
				}
			}),
			// Query: list known keys.
			eb.Subscribe(sysctlevents.Namespace, sysctlevents.EventListRequest, func(payload string) {
				var req struct {
					RequestID string `json:"request-id"`
				}
				_ = json.Unmarshal([]byte(payload), &req)
				result := listKnownKeys()
				resp, _ := json.Marshal(struct {
					RequestID string `json:"request-id"`
					Entries   string `json:"entries"`
				}{RequestID: req.RequestID, Entries: result})
				if _, err := eb.Emit(sysctlevents.Namespace, sysctlevents.EventListResult, string(resp)); err != nil {
					log.Debug("sysctl: list-result emit failed", "err", err)
				}
			}),
			// Clear profile defaults for an interface (before re-emission on reload).
			eb.Subscribe(sysctlevents.Namespace, sysctlevents.EventClearProfileDefaults, func(payload string) {
				var ev struct {
					Interface string `json:"interface"`
				}
				if err := json.Unmarshal([]byte(payload), &ev); err != nil {
					log.Warn("sysctl: bad clear-profile-defaults event", "err", err)
					return
				}
				s.clearProfileDefaults(ev.Interface)
			}),
			// Query: describe one key.
			eb.Subscribe(sysctlevents.Namespace, sysctlevents.EventDescribeRequest, func(payload string) {
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
				if _, err := eb.Emit(sysctlevents.Namespace, sysctlevents.EventDescribeResult, string(resp)); err != nil {
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
				if _, emitErr := eb.Emit(sysctlevents.Namespace, sysctlevents.EventApplied, payload); emitErr != nil {
					log.Debug("sysctl: applied emit failed", "err", emitErr)
				}
			}
		}
		log.Info("sysctl config applied", "keys", len(settings))
		return nil
	}

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, sec := range sections {
			if sec.Root != configRoot {
				continue
			}
			// Register user-defined profiles before applying settings.
			for _, prof := range parseSysctlProfileConfig(sec.Data) {
				sysctlreg.RegisterProfile(prof)
				log.Info("sysctl: registered user profile", "name", prof.Name, "keys", len(prof.Settings))
			}
			return applyFromJSON(sec.Data)
		}
		return nil
	})

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		for _, sec := range sections {
			if sec.Root != configRoot {
				continue
			}
			// Validate per-key settings.
			settings := parseSysctlConfig(sec.Data)
			for key, value := range settings {
				if err := validateKey(key); err != nil {
					return err
				}
				if err := sysctlreg.Validate(key, value); err != nil {
					return err
				}
			}
			// Validate keys inside user-defined profiles.
			for _, prof := range parseSysctlProfileConfig(sec.Data) {
				for _, setting := range prof.Settings {
					if err := validateKey(setting.Key); err != nil {
						return fmt.Errorf("profile %s: %w", prof.Name, err)
					}
					if err := sysctlreg.Validate(setting.Key, setting.Value); err != nil {
						return fmt.Errorf("profile %s: %w", prof.Name, err)
					}
				}
			}
		}
		return nil
	})

	var activeJournal *sdk.Journal

	p.OnConfigApply(func(sections []sdk.ConfigDiffSection) error {
		for _, sec := range sections {
			if sec.Root != configRoot {
				continue
			}
			// Deregister profiles that were removed from config.
			for _, prof := range parseSysctlProfileConfig(sec.Removed) {
				sysctlreg.DeregisterProfile(prof.Name)
				log.Info("sysctl: deregistered user profile", "name", prof.Name)
			}
			// Re-register user-defined profiles from added/changed config.
			for _, data := range []string{sec.Added, sec.Changed} {
				for _, prof := range parseSysctlProfileConfig(data) {
					sysctlreg.RegisterProfile(prof)
					log.Info("sysctl: registered user profile", "name", prof.Name, "keys", len(prof.Settings))
				}
			}

			// Merge Added and Changed into one settings map. Keys absent
			// from the result cause applyConfig to clear the config layer
			// (handling Removed implicitly).
			settings := make(map[string]string)
			for _, data := range []string{sec.Added, sec.Changed} {
				maps.Copy(settings, parseSysctlConfig(data))
			}

			// Snapshot config state before apply for rollback.
			snap := s.snapshotConfig()
			j := sdk.NewJournal()
			err := j.Record(
				func() error {
					applied, errs := s.applyConfig(settings)
					if len(errs) > 0 {
						return fmt.Errorf("sysctl config: %w", errs[0])
					}
					if eb != nil {
						for _, payload := range applied {
							if _, emitErr := eb.Emit(sysctlevents.Namespace, sysctlevents.EventApplied, payload); emitErr != nil {
								log.Debug("sysctl: applied emit failed", "err", emitErr)
							}
						}
					}
					return nil
				},
				func() error {
					s.rollbackConfig(snap)
					return nil
				},
			)
			if err != nil {
				j.Rollback()
				return err
			}
			activeJournal = j
			log.Info("sysctl config reloaded", "keys", len(settings))
			return nil
		}
		return nil
	})

	p.OnConfigRollback(func(_ string) error {
		j := activeJournal
		activeJournal = nil
		if j == nil {
			return nil
		}
		if errs := j.Rollback(); len(errs) > 0 {
			return fmt.Errorf("sysctl rollback: %d errors", len(errs))
		}
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
		case "sysctl list-profiles":
			return statusDone, listProfiles(), nil
		case "sysctl describe-profile":
			if len(args) < 1 {
				return statusError, "", fmt.Errorf("sysctl describe-profile: requires profile name argument")
			}
			return statusDone, describeProfile(args[0]), nil
		case "sysctl set":
			if len(args) < 2 {
				return statusError, "", fmt.Errorf("sysctl set: requires key and value arguments")
			}
			applied, err := s.setTransient(args[0], args[1])
			if err != nil {
				return statusError, "", fmt.Errorf("sysctl set %s: %w", args[0], err)
			}
			if applied != "" && eb != nil {
				if _, emitErr := eb.Emit(sysctlevents.Namespace, sysctlevents.EventApplied, applied); emitErr != nil {
					log.Debug("sysctl: applied emit failed", "err", emitErr)
				}
			}
			return statusDone, applied, nil
		}
		return statusError, "", fmt.Errorf("unknown command: %s", command)
	})

	ctx := context.Background()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{configRoot},
		VerifyBudget: 1,
		ApplyBudget:  1,
		Commands: []sdk.CommandDecl{
			{Name: "sysctl show", Description: "Show all active sysctl keys with source and persistence"},
			{Name: "sysctl list", Description: "List all known sysctl keys with descriptions"},
			{Name: "sysctl describe", Description: "Show detail for one sysctl key", Args: []string{"key"}},
			{Name: "sysctl set", Description: "Set a transient sysctl value", Args: []string{"key", "value"}},
			{Name: "sysctl list-profiles", Description: "List all registered sysctl profiles"},
			{Name: "sysctl describe-profile", Description: "Show detail for one sysctl profile", Args: []string{"name"}},
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
