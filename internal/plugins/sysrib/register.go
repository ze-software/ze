package sysrib

import (
	"context"
	"fmt"
	"net"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/rib/locrib"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sysribevents "codeberg.org/thomas-mangin/ze/internal/plugins/sysrib/events"
	sysribschema "codeberg.org/thomas-mangin/ze/internal/plugins/sysrib/schema"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

func init() {
	_ = events.RegisterNamespace(sysribevents.Namespace,
		sysribevents.EventBestChange, sysribevents.EventReplayRequest,
	)

	reg := registry.Registration{
		Name:        "rib",
		Description: "System RIB: selects best route across protocols by admin distance",
		Features:    "yang",
		YANG:        sysribschema.ZeRibConfYANG,
		ConfigRoots: []string{"rib"},
		RunEngine:   runSysRIBPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg any) {
			if r, ok := reg.(metrics.Registry); ok {
				SetMetricsRegistry(r)
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
		fmt.Fprintf(os.Stderr, "sysrib: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func runSysRIBPlugin(conn net.Conn) int {
	logger().Debug("sysrib plugin starting (RPC)")

	p := sdk.NewWithConn("rib", conn)
	defer func() { _ = p.Close() }()

	// Wire the process-wide Loc-RIB so sysrib's run() picks the
	// cross-protocol OnChange source when in-process plugins share a
	// singleton; returns nil in forked subprocesses, leaving the
	// EventBus fallback path active.
	SetLocRIB(locrib.Default())

	s := newSysRIB()

	// pendingDist holds the validated admin-distance map between verify and apply.
	var pendingDist map[string]int

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != "rib" {
				continue
			}
			dist, err := parseAdminDistanceConfig(section.Data)
			if err != nil {
				return err
			}
			pendingDist = dist
		}
		return nil
	})

	// previousDist tracks the last applied admin distances for rollback.
	// Initialized from OnConfigure so the first reload rollback restores startup state.
	var previousDist map[string]int
	var activeJournal *sdk.Journal

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != "rib" {
				continue
			}
			dist, err := parseAdminDistanceConfig(section.Data)
			if err != nil {
				logger().Error("admin-distance config parse failed", "error", err)
				return err
			}
			s.mu.Lock()
			s.adminDist = dist
			s.mu.Unlock()
			previousDist = dist
			logger().Info("admin-distance config loaded", "distances", dist)
		}
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		dist := pendingDist
		pendingDist = nil
		if dist == nil {
			return nil
		}

		oldDist := previousDist
		j := sdk.NewJournal()
		err := j.Record(
			func() error {
				s.mu.Lock()
				s.adminDist = dist
				s.mu.Unlock()

				changes := s.reapplyAdminDistances()
				for famName, ch := range changes {
					if len(ch) > 0 {
						publishChanges(ch, famName)
					}
				}
				return nil
			},
			func() error {
				// Rollback: restore previous admin distances.
				rollbackDist := oldDist
				if rollbackDist == nil {
					rollbackDist = make(map[string]int)
				}
				s.mu.Lock()
				s.adminDist = rollbackDist
				s.mu.Unlock()

				changes := s.reapplyAdminDistances()
				for famName, ch := range changes {
					if len(ch) > 0 {
						publishChanges(ch, famName)
					}
				}
				return nil
			},
		)
		if err != nil {
			j.Rollback()
			return err
		}

		previousDist = dist
		activeJournal = j
		logger().Info("admin-distance config reloaded via transaction", "distances", dist)
		return nil
	})

	p.OnConfigRollback(func(_ string) error {
		j := activeJournal
		activeJournal = nil
		if j == nil {
			return nil
		}
		if errs := j.Rollback(); len(errs) > 0 {
			return fmt.Errorf("sysrib rollback: %d errors", len(errs))
		}
		logger().Info("sysrib config rolled back")
		return nil
	})

	p.OnStarted(func(ctx context.Context) error {
		go s.run(ctx)
		return nil
	})

	p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, string, error) {
		if command == "rib show" {
			data, err := s.showRIB()
			if err != nil {
				return "error", "", err
			}
			return "done", data, nil
		}
		return "error", "", fmt.Errorf("unknown command: %s", command)
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{"rib"},
		VerifyBudget: 1,
		ApplyBudget:  2,
		Commands: []sdk.CommandDecl{
			{Name: "rib show"},
		},
	})
	if err != nil {
		logger().Error("sysrib plugin failed", "error", err)
		return 1
	}

	return 0
}
