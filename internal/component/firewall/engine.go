// Design: docs/architecture/core-design.md -- Firewall plugin engine (SDK 5-stage)
// Related: register.go -- plugin registration that invokes runEngine
// Related: backend.go -- Backend interface + Load/Get/Close
// Related: accessor.go -- LastApplied/ActiveBackendName consumed by show handlers
// Related: config.go -- ParseFirewallConfig produces []Table from JSON

package firewall

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	sysctlevents "codeberg.org/thomas-mangin/ze/internal/plugins/sysctl/events"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

var (
	errFirewallNoBackendConfiguredAndNo      = errors.New("firewall: no backend configured and no OS default available")
	errFirewallConfigApplyNoBackendAvailable = errors.New("firewall config apply: no backend available")
)

// configRootFirewall is the YANG config root the firewall plugin owns.
// The YANG container name and this constant MUST match.
const configRootFirewall = "firewall"

// backendLeafPath is surfaced in backend-gate error text so operators
// know where to change the backend leaf.
const backendLeafPath = "/firewall/backend"

// backendGateSchema caches the config schema used by validateBackendGate.
// Built lazily on first commit/verify -- YANG load is paid only when a
// gate-enforcing RPC fires.
var (
	backendGateSchemaOnce sync.Once
	backendGateSchema     *config.Schema
	backendGateSchemaErr  error
)

// validateBackendGate runs the ze:backend commit-time feature check.
// Mirrors iface and traffic so the daemon commit path and offline
// `ze config validate` emit the same diagnostics. The firewall YANG
// currently carries seven `ze:backend "nft"` annotations (connection-
// state / connection-mark matches and five action/modifier fields), so
// the gate actively rejects non-nft backends that attempt to configure
// those features -- it is not a placeholder.
func validateBackendGate(sections []sdk.ConfigSection, activeBackend string) error {
	backendGateSchemaOnce.Do(func() {
		backendGateSchema, backendGateSchemaErr = config.YANGSchema()
	})
	if backendGateSchemaErr != nil {
		return fmt.Errorf("firewall backend gate: schema load: %w", backendGateSchemaErr)
	}
	for _, s := range sections {
		if s.Root != configRootFirewall {
			continue
		}
		errs := config.ValidateBackendFeaturesJSON(
			s.Data, backendGateSchema,
			configRootFirewall, activeBackend, backendLeafPath,
		)
		if len(errs) == 0 {
			return nil
		}
		msgs := make([]string, 0, len(errs))
		for _, e := range errs {
			msgs = append(msgs, e.Error())
		}
		return fmt.Errorf("firewall commit rejected:\n  %s", textbuf.Join(msgs, "\n  "))
	}
	return nil
}

// firewallConfig carries parsed firewall state from OnConfigVerify into
// OnConfigApply. Backend is the selected backend name; Tables is the
// desired kernel state. RawData carries the JSON for global-options extraction.
type firewallConfig struct {
	Backend string
	Tables  []Table
	RawData string
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
		cfg.RawData = s.Data
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

// VerifyConfig validates firewall config without loading or applying a backend.
func VerifyConfig(sections []sdk.ConfigSection) error {
	_, err := parseAndVerifyFirewallSections(sections)
	return err
}

func parseAndVerifyFirewallSections(sections []sdk.ConfigSection) (*firewallConfig, error) {
	cfg, err := parseFirewallSections(sections)
	if err != nil {
		return nil, err
	}
	if !hasFirewallSection(sections) {
		return cfg, nil
	}
	if cfg.Backend == "" {
		return nil, errFirewallNoBackendConfiguredAndNo
	}
	if _, err := ExtractGlobalOptions(cfg.RawData); err != nil {
		return nil, err
	}
	if err := validateBackendGate(sections, cfg.Backend); err != nil {
		return nil, err
	}
	if err := ValidateTables(cfg.Tables); err != nil {
		return nil, err
	}
	if err := RunVerifier(cfg.Backend, cfg.Tables); err != nil {
		return nil, err
	}
	return cfg, nil
}

// setLogger stores the engine-provided logger for package-level use.
func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

const globalOptionsSource = "firewall:global-options"

// emitGlobalOptionsSysctlDefaults extracts global-options from the config
// section and emits each as a sysctl default via EventBus. The sysctl
// plugin receives these at the default layer, so explicit sysctl config
// settings always win. On reload, clears previous defaults first so
// removed global-options revert to the original OS value.
func emitGlobalOptionsSysctlDefaults(eb ze.EventBus, data string, reload bool, log *slog.Logger) {
	if eb == nil {
		return
	}
	opts, err := ExtractGlobalOptions(data)
	if err != nil {
		log.Warn("firewall: global-options extraction failed", "err", err)
		return
	}
	if reload {
		clearPayload, _ := json.Marshal(struct {
			Source string `json:"source"`
		}{Source: globalOptionsSource})
		if _, emitErr := eb.Emit(sysctlevents.Namespace, sysctlevents.EventClearSourceDefaults, string(clearPayload)); emitErr != nil {
			log.Warn("firewall: clear global-options defaults failed", "err", emitErr)
		}
	}
	type sysctlDefault struct {
		Key    string `json:"key"`
		Value  string `json:"value"`
		Source string `json:"source"`
	}
	for key, value := range opts {
		payload, _ := json.Marshal(sysctlDefault{Key: key, Value: value, Source: globalOptionsSource})
		if _, emitErr := eb.Emit(sysctlevents.Namespace, sysctlevents.EventDefault, string(payload)); emitErr != nil {
			log.Warn("firewall: emit sysctl default failed", "key", key, "err", emitErr)
		}
	}
	if len(opts) > 0 {
		log.Info("firewall: emitted global-options sysctl defaults", "keys", len(opts))
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

	eb := getEventBusRef()

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
			return errFirewallNoBackendConfiguredAndNo
		}

		if err := validateBackendGate(sections, cfg.Backend); err != nil {
			return err
		}
		if err := ValidateTables(cfg.Tables); err != nil {
			return err
		}

		if err := LoadBackend(cfg.Backend); err != nil {
			return fmt.Errorf("firewall backend %q: %w", cfg.Backend, err)
		}
		log.Info("firewall backend loaded", "backend", cfg.Backend)

		RegisterTables("firewall", cfg.Tables)
		if err := ApplyAll(); err != nil {
			return fmt.Errorf("firewall config apply: %w", err)
		}
		StoreLastApplied(cfg.Tables)
		activeCfg.Store(cfg)
		log.Info("firewall config applied", "tables", len(cfg.Tables))

		emitGlobalOptionsSysctlDefaults(eb, cfg.RawData, false, log)
		return nil
	})

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		cfg, err := parseAndVerifyFirewallSections(sections)
		if err != nil {
			return err
		}
		if !hasFirewallSection(sections) {
			pendingCfg = cfg
			log.Debug("firewall config verified: no firewall section")
			return nil
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
			return errFirewallConfigApplyNoBackendAvailable
		}

		if GetBackend() == nil || (previousCfg != nil && previousCfg.Backend != desiredBackend) {
			if err := LoadBackend(desiredBackend); err != nil {
				return fmt.Errorf("firewall backend %q: %w", desiredBackend, err)
			}
			log.Info("firewall backend loaded", "backend", desiredBackend)
		}

		j := sdk.NewJournal()
		err := j.Record(
			func() error {
				RegisterTables("firewall", cfg.Tables)
				if applyErr := ApplyAll(); applyErr != nil {
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
				RegisterTables("firewall", desired)
				if rollbackErr := ApplyAll(); rollbackErr != nil {
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

		emitGlobalOptionsSysctlDefaults(eb, cfg.RawData, true, log)
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
