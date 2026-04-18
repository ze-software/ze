// Design: docs/architecture/core-design.md -- Firewall plugin engine (SDK 5-stage)
// Related: register.go -- plugin registration that invokes runEngine
// Related: backend.go -- Backend interface + Load/Get/Close
// Related: accessor.go -- LastApplied/ActiveBackendName consumed by show handlers
// Related: config.go -- ParseFirewallConfig produces []Table from JSON

package firewall

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// configRootFirewall is the YANG config root the firewall plugin owns.
// The YANG container name and this constant MUST match.
const configRootFirewall = "firewall"

// firewallConfig carries parsed firewall state from OnConfigVerify into
// OnConfigApply. Backend is the selected backend name; Tables is the
// desired kernel state.
type firewallConfig struct {
	Backend string
	Tables  []Table
}

// parseFirewallSections extracts the firewall section from the SDK
// payload and returns a firewallConfig. When no section is present the
// returned config has an empty Tables slice and the default backend
// name -- callers distinguish via hasFirewallSection.
func parseFirewallSections(sections []sdk.ConfigSection) (*firewallConfig, error) {
	cfg := &firewallConfig{Backend: defaultBackendName}
	for _, s := range sections {
		if s.Root != configRootFirewall {
			continue
		}
		backend, err := extractBackend(s.Data)
		if err != nil {
			return nil, err
		}
		if backend != "" {
			cfg.Backend = backend
		}
		tables, err := ParseFirewallConfig(s.Data)
		if err != nil {
			return nil, fmt.Errorf("firewall config: %w", err)
		}
		cfg.Tables = tables
		return cfg, nil
	}
	return cfg, nil
}

// extractBackend reads the `firewall/backend` leaf directly. The main
// parser (ParseFirewallConfig) ignores it because the model does not
// carry a backend field.
func extractBackend(data string) (string, error) {
	if strings.TrimSpace(data) == "" {
		return "", nil
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(data), &root); err != nil {
		return "", fmt.Errorf("firewall config: unmarshal: %w", err)
	}
	fw, ok := root[configRootFirewall].(map[string]any)
	if !ok {
		return "", nil
	}
	b, _ := fw["backend"].(string)
	return b, nil
}

// hasFirewallSection reports whether the SDK delivered a firewall
// section in this payload. Lets the engine distinguish "no firewall
// configured" (legitimately idle) from "firewall section present but
// empty" (apply empty desired state so orphan ze_* tables are removed).
func hasFirewallSection(sections []sdk.ConfigSection) bool {
	for _, s := range sections {
		if s.Root == configRootFirewall {
			return true
		}
	}
	return false
}

// setLogger stores the engine-provided logger for package-level use.
func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// runEngine is the engine-mode entry point for the firewall plugin.
// Mirrors the traffic plugin's 5-stage lifecycle: the first Apply
// happens in OnConfigure; subsequent reloads go through
// OnConfigVerify + OnConfigApply with rollback support.
func runEngine(conn net.Conn) int {
	log := loggerPtr.Load()
	log.Debug("firewall plugin starting")

	p := sdk.NewWithConn("firewall", conn)
	defer func() { _ = p.Close() }()

	// pendingCfg carries the verified config from OnConfigVerify into
	// OnConfigApply. Cleared once OnConfigApply consumes it.
	var pendingCfg *firewallConfig

	// activeCfg holds the last successfully applied config so
	// OnConfigApply can diff and OnConfigRollback can restore it.
	var activeCfg atomic.Pointer[firewallConfig]
	var activeJournal *sdk.Journal

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, err := parseFirewallSections(sections)
		if err != nil {
			return err
		}
		if !hasFirewallSection(sections) {
			log.Debug("firewall: no configuration, plugin idle")
			activeCfg.Store(cfg)
			return nil
		}
		if cfg.Backend == "" {
			return fmt.Errorf("firewall: no backend configured and no OS default available")
		}

		if err := LoadBackend(cfg.Backend); err != nil {
			return fmt.Errorf("firewall backend %q: %w", cfg.Backend, err)
		}
		log.Info("firewall backend loaded", "backend", cfg.Backend)

		b := GetBackend()
		if err := b.Apply(cfg.Tables); err != nil {
			return fmt.Errorf("firewall config apply: %w", err)
		}
		StoreLastApplied(cfg.Tables)
		activeCfg.Store(cfg)
		log.Info("firewall config applied", "tables", len(cfg.Tables))
		return nil
	})

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		cfg, err := parseFirewallSections(sections)
		if err != nil {
			return err
		}
		if !hasFirewallSection(sections) {
			pendingCfg = cfg
			log.Debug("firewall config verified: no firewall section")
			return nil
		}
		if cfg.Backend == "" {
			return fmt.Errorf("firewall: no backend configured and no OS default available")
		}
		for i := range cfg.Tables {
			if err := cfg.Tables[i].Validate(); err != nil {
				return err
			}
			for j := range cfg.Tables[i].Chains {
				if err := cfg.Tables[i].Chains[j].Validate(); err != nil {
					return fmt.Errorf("table %q: %w", cfg.Tables[i].Name, err)
				}
			}
			for j := range cfg.Tables[i].Sets {
				if err := cfg.Tables[i].Sets[j].Validate(); err != nil {
					return fmt.Errorf("table %q: %w", cfg.Tables[i].Name, err)
				}
			}
			for j := range cfg.Tables[i].Flowtables {
				if err := cfg.Tables[i].Flowtables[j].Validate(); err != nil {
					return fmt.Errorf("table %q: %w", cfg.Tables[i].Name, err)
				}
			}
		}
		pendingCfg = cfg
		log.Debug("firewall config verified", "backend", cfg.Backend, "tables", len(cfg.Tables))
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		cfg := pendingCfg
		pendingCfg = nil
		if cfg == nil {
			log.Warn("firewall config apply: no pending config (verify not called?)")
			return nil
		}

		previousCfg := activeCfg.Load()

		desiredBackend := cfg.Backend
		if desiredBackend == "" && previousCfg != nil {
			desiredBackend = previousCfg.Backend
		}
		if desiredBackend == "" {
			desiredBackend = defaultBackendName
		}
		if desiredBackend == "" {
			return fmt.Errorf("firewall config apply: no backend available")
		}

		if GetBackend() == nil || (previousCfg != nil && previousCfg.Backend != desiredBackend) {
			if err := LoadBackend(desiredBackend); err != nil {
				return fmt.Errorf("firewall backend %q: %w", desiredBackend, err)
			}
			log.Info("firewall backend loaded", "backend", desiredBackend)
		}

		b := GetBackend()
		j := sdk.NewJournal()
		err := j.Record(
			func() error {
				if applyErr := b.Apply(cfg.Tables); applyErr != nil {
					return fmt.Errorf("firewall reload: %w", applyErr)
				}
				StoreLastApplied(cfg.Tables)
				return nil
			},
			func() error {
				var desired []Table
				if previousCfg != nil {
					desired = previousCfg.Tables
				}
				if rollbackErr := b.Apply(desired); rollbackErr != nil {
					return fmt.Errorf("firewall rollback: %w", rollbackErr)
				}
				StoreLastApplied(desired)
				return nil
			},
		)
		if err != nil {
			j.Rollback()
			return err
		}
		activeCfg.Store(cfg)
		activeJournal = j
		log.Info("firewall config reloaded", "tables", len(cfg.Tables))
		return nil
	})

	p.OnConfigRollback(func(_ string) error {
		j := activeJournal
		activeJournal = nil
		if j == nil {
			return nil
		}
		if errs := j.Rollback(); len(errs) > 0 {
			return fmt.Errorf("firewall rollback: %d errors", len(errs))
		}
		log.Info("firewall config rolled back")
		return nil
	})

	ctx := context.Background()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{configRootFirewall},
		VerifyBudget: 2,
		ApplyBudget:  10,
	}); err != nil {
		log.Error("firewall plugin failed", "error", err)
		return 1
	}

	if err := CloseBackend(); err != nil {
		log.Warn("firewall backend close failed", "error", err)
	}
	log.Info("firewall backend closed")

	return 0
}
