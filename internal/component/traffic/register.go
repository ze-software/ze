// Design: docs/architecture/core-design.md -- Traffic component reactor
// Related: backend.go -- Backend interface consumed by OnConfigure/OnConfigApply
// Related: config.go -- ParseTrafficConfig called from parseTrafficSections

package traffic

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	trafficschema "codeberg.org/thomas-mangin/ze/internal/component/traffic/schema"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// configRootTraffic is the top-level YANG config root that the traffic plugin
// owns. The YANG container and this string MUST match.
const configRootTraffic = "traffic-control"

// backendLeafPath is the YANG path surfaced in backend-gate error text so
// operators know where to change the backend leaf.
const backendLeafPath = "/traffic-control/backend"

// backendGateSchema caches the config schema used by validateBackendGate.
// Built lazily on first commit/verify to avoid paying YANG load cost at
// daemon startup. Schema is immutable after build -- safe for concurrent
// reads from any goroutine.
var (
	backendGateSchemaOnce sync.Once
	backendGateSchema     *config.Schema
	backendGateSchemaErr  error
)

// validateBackendGate runs the ze:backend commit-time feature check for the
// traffic-control section. Mirrors the iface plugin's gate so daemon commit
// and offline `ze config validate` emit the same diagnostics.
func validateBackendGate(sections []sdk.ConfigSection, activeBackend string) error {
	backendGateSchemaOnce.Do(func() {
		backendGateSchema, backendGateSchemaErr = config.YANGSchema()
	})
	if backendGateSchemaErr != nil {
		return fmt.Errorf("traffic backend gate: schema load: %w", backendGateSchemaErr)
	}
	for _, s := range sections {
		if s.Root != configRootTraffic {
			continue
		}
		errs := config.ValidateBackendFeaturesJSON(
			s.Data, backendGateSchema,
			configRootTraffic, activeBackend, backendLeafPath,
		)
		if len(errs) == 0 {
			return nil
		}
		msgs := make([]string, 0, len(errs))
		for _, e := range errs {
			msgs = append(msgs, e.Error())
		}
		return fmt.Errorf("traffic-control commit rejected:\n  %s", strings.Join(msgs, "\n  "))
	}
	return nil
}

// trafficConfig is the parsed, in-memory view of the traffic-control section.
// Held between OnConfigVerify and OnConfigApply.
type trafficConfig struct {
	Backend    string
	Interfaces map[string]InterfaceQoS
}

// parseTrafficSections extracts the traffic-control section from the SDK
// payload and returns a trafficConfig. Absent sections yield the default
// backend and an empty interface map so callers need not special-case the
// "no traffic-control configured" path.
func parseTrafficSections(sections []sdk.ConfigSection) (*trafficConfig, error) {
	for _, s := range sections {
		if s.Root != configRootTraffic {
			continue
		}
		return parseTrafficSectionData(s.Data)
	}
	return &trafficConfig{Backend: defaultBackendName, Interfaces: map[string]InterfaceQoS{}}, nil
}

// parseTrafficSectionData parses the raw JSON string delivered to the plugin.
// ParseTrafficConfig ignores the `backend` leaf, so we extract it directly
// here from the same JSON; the existing parser produces the per-interface QoS
// map. Single unmarshal, two outputs.
func parseTrafficSectionData(data string) (*trafficConfig, error) {
	cfg := &trafficConfig{Backend: defaultBackendName, Interfaces: map[string]InterfaceQoS{}}

	if strings.TrimSpace(data) == "" {
		return cfg, nil
	}

	var root map[string]any
	if err := json.Unmarshal([]byte(data), &root); err != nil {
		return nil, fmt.Errorf("traffic-control config: unmarshal: %w", err)
	}
	if tcMap, ok := root[configRootTraffic].(map[string]any); ok {
		if b, ok := tcMap["backend"].(string); ok && b != "" {
			cfg.Backend = b
		}
	}

	ifaces, err := ParseTrafficConfig(data)
	if err != nil {
		return nil, err
	}
	cfg.Interfaces = ifaces
	return cfg, nil
}

// setLogger stores the engine-provided logger for package-level use.
// It writes to the shared loggerPtr declared in backend.go.
func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

func init() {
	reg := registry.Registration{
		Name:        "traffic",
		Description: "Traffic control (tc) qdisc, class, and filter management",
		Features:    "yang",
		YANG:        trafficschema.ZeTrafficControlConfYANG,
		ConfigRoots: []string{configRootTraffic},
		RunEngine:   runEngine,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(_ []string) int {
		return 1
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "traffic: registration failed: %v\n", err)
		os.Exit(1)
	}
}

// runEngine is the engine-mode entry point for the traffic plugin. It uses
// the SDK 5-stage protocol to receive configuration and drives the active
// backend's Apply call on startup and each reload.
func runEngine(conn net.Conn) int {
	log := loggerPtr.Load()
	log.Debug("traffic plugin starting")

	p := sdk.NewWithConn("traffic", conn)
	defer func() { _ = p.Close() }()

	// pendingCfg carries the verified reload config from OnConfigVerify into
	// OnConfigApply. Cleared when OnConfigApply consumes it.
	var pendingCfg *trafficConfig

	// activeCfg tracks the last successfully applied config so OnConfigApply
	// can diff against it and OnConfigRollback can restore it. Initialized
	// from OnConfigure so the first reload rollback restores startup state.
	var activeCfg atomic.Pointer[trafficConfig]
	var activeJournal *sdk.Journal

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, err := parseTrafficSections(sections)
		if err != nil {
			return fmt.Errorf("traffic-control config: %w", err)
		}

		// No traffic-control section in the payload: operator has no QoS
		// configured. Remain idle -- no backend load, no Apply call.
		if !hasTrafficSection(sections) {
			log.Debug("traffic-control: no configuration, plugin idle")
			activeCfg.Store(cfg)
			return nil
		}

		if cfg.Backend == "" {
			return fmt.Errorf("traffic-control: no backend configured and no OS default available")
		}

		if err := validateBackendGate(sections, cfg.Backend); err != nil {
			return err
		}

		if err := LoadBackend(cfg.Backend); err != nil {
			return fmt.Errorf("traffic-control backend %q: %w", cfg.Backend, err)
		}
		log.Info("traffic-control backend loaded", "backend", cfg.Backend)

		b := GetBackend()
		if err := b.Apply(cfg.Interfaces); err != nil {
			return fmt.Errorf("traffic-control config apply: %w", err)
		}
		activeCfg.Store(cfg)
		log.Info("traffic-control config applied", "interfaces", len(cfg.Interfaces))
		return nil
	})

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		cfg, err := parseTrafficSections(sections)
		if err != nil {
			return fmt.Errorf("traffic-control config: %w", err)
		}
		// A reload that drops the traffic-control section is a valid state
		// (operator removed QoS). Accept it without a backend-gate call;
		// OnConfigApply will apply the empty desired state.
		if !hasTrafficSection(sections) {
			pendingCfg = cfg
			log.Debug("traffic-control config verified: no traffic-control section")
			return nil
		}
		if cfg.Backend == "" {
			return fmt.Errorf("traffic-control: no backend configured and no OS default available")
		}
		if err := validateBackendGate(sections, cfg.Backend); err != nil {
			return err
		}
		if err := RunVerifier(cfg.Backend, cfg.Interfaces); err != nil {
			return fmt.Errorf("traffic-control backend %q: %w", cfg.Backend, err)
		}
		pendingCfg = cfg
		log.Debug("traffic-control config verified", "backend", cfg.Backend)
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		cfg := pendingCfg
		pendingCfg = nil
		if cfg == nil {
			log.Warn("traffic-control config apply: no pending config (verify not called?)")
			return nil
		}

		previousCfg := activeCfg.Load()

		// Pick the backend to program: explicit config wins, then the
		// previously-applied backend, finally the OS default.
		desiredBackend := cfg.Backend
		if desiredBackend == "" && previousCfg != nil {
			desiredBackend = previousCfg.Backend
		}
		if desiredBackend == "" {
			desiredBackend = defaultBackendName
		}
		if desiredBackend == "" {
			return fmt.Errorf("traffic-control config apply: no backend available")
		}

		if GetBackend() == nil || (previousCfg != nil && previousCfg.Backend != desiredBackend) {
			if err := LoadBackend(desiredBackend); err != nil {
				return fmt.Errorf("traffic-control backend %q: %w", desiredBackend, err)
			}
			log.Info("traffic-control backend loaded", "backend", desiredBackend)
		}

		b := GetBackend()
		j := sdk.NewJournal()
		err := j.Record(
			func() error {
				if applyErr := b.Apply(cfg.Interfaces); applyErr != nil {
					return fmt.Errorf("traffic-control reload: %w", applyErr)
				}
				return nil
			},
			func() error {
				// Rollback: re-apply previous config. When no previous
				// config exists (first reload after boot with no initial
				// section), an empty desired state unprograms any qdiscs
				// that the failed reload installed.
				desired := map[string]InterfaceQoS{}
				if previousCfg != nil {
					desired = previousCfg.Interfaces
				}
				if rollbackErr := b.Apply(desired); rollbackErr != nil {
					return fmt.Errorf("traffic-control rollback: %w", rollbackErr)
				}
				return nil
			},
		)
		if err != nil {
			j.Rollback()
			return err
		}
		activeCfg.Store(cfg)
		activeJournal = j
		log.Info("traffic-control config reloaded", "interfaces", len(cfg.Interfaces))
		return nil
	})

	p.OnConfigRollback(func(_ string) error {
		j := activeJournal
		activeJournal = nil
		if j == nil {
			return nil
		}
		if errs := j.Rollback(); len(errs) > 0 {
			return fmt.Errorf("traffic-control rollback: %d errors", len(errs))
		}
		log.Info("traffic-control config rolled back")
		return nil
	})

	ctx := context.Background()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{configRootTraffic},
		VerifyBudget: 2,
		ApplyBudget:  10,
	}); err != nil {
		log.Error("traffic plugin failed", "error", err)
		return 1
	}

	if err := CloseBackend(); err != nil {
		log.Warn("traffic backend close failed", "error", err)
	}
	log.Info("traffic backend closed")

	return 0
}

// hasTrafficSection reports whether the SDK delivered a traffic-control
// section in this payload. Used so the reactor can distinguish "config
// genuinely carries no traffic-control block" from "config has the block
// but no interfaces underneath" -- the latter still demands a gate call.
func hasTrafficSection(sections []sdk.ConfigSection) bool {
	for _, s := range sections {
		if s.Root == configRootTraffic {
			return true
		}
	}
	return false
}
