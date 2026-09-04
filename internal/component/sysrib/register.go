package sysrib

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	sysribyang "github.com/ze-software/ze/internal/component/sysrib/yang"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/rib/distance"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/slogutil"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

const configRootRIB = "rib"

func init() {
	_ = events.RegisterNamespace(sysribevents.Namespace,
		sysribevents.EventBestChange, sysribevents.EventReplayRequest,
	)

	reg := registry.Registration{
		Name:                    configRootRIB,
		Description:             "System RIB: selects best route across protocols by admin distance",
		Features:                "yang",
		YANG:                    sysribyang.ZeRibConfYANG,
		ConfigRoots:             []string{configRootRIB},
		InProcessConfigVerifier: verifySysRIBConfig,
		RunEngine:               runSysRIBPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg metrics.Registry) {
			SetMetricsRegistry(reg)
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			setEventBus(eb)
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

// publishDistances installs the resolved table on the shared seam
// (internal/core/rib/distance) so the PRODUCERS stamp the operator's value.
//
// This is not a convenience. locrib.selectBest ranks paths on what the producer
// stamped and runs BEFORE sysrib sees the route: sysrib consumes one
// already-arbitrated best per prefix. A distance that reaches sysrib alone
// therefore cannot change cross-protocol selection, however carefully it was
// resolved. The seam is how the one declaration reaches the only layer that can
// act on it.
//
// Called from every site that assigns s.adminDist, the rollback included, so
// the seam and the map cannot disagree.
func publishDistances(dist map[string]int) {
	distance.Set(func(protocol string) (uint8, bool) {
		d, ok := dist[protocol]
		if !ok || d < 0 || d > 255 {
			return 0, false
		}
		return uint8(d), true //nolint:gosec // bounded immediately above
	})
}

func verifySysRIBConfig(sections []sdk.ConfigSection) error {
	for _, section := range sections {
		if section.Root != configRootRIB {
			continue
		}
		if _, err := parseAdminDistanceConfig(section.Data); err != nil {
			return err
		}
	}
	return nil
}

func runSysRIBPlugin(conn net.Conn) int {
	logger().Debug("sysrib plugin starting (RPC)")

	p := sdk.NewWithConn(configRootRIB, conn)
	defer func() { _ = p.Close() }()

	// Wire the process-wide Loc-RIB so sysrib's run() picks the
	// cross-protocol OnChange source when in-process plugins share a
	// singleton; returns nil in forked subprocesses, leaving the
	// EventBus fallback path active.
	SetLocRIB(locrib.Default())

	s := newSysRIB()

	// pendingDist holds the validated distance map between verify and apply.
	var pendingDist map[string]int

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != configRootRIB {
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
			if section.Root != configRootRIB {
				continue
			}
			dist, err := parseAdminDistanceConfig(section.Data)
			if err != nil {
				logger().Error("distance config parse failed", "error", err)
				return err
			}
			s.mu.Lock()
			s.adminDist = dist
			s.mu.Unlock()
			publishDistances(dist)
			previousDist = dist
			logger().Info("distance config loaded", "distances", dist)
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
				publishDistances(dist)
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
				publishDistances(rollbackDist)
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
		logger().Info("distance config reloaded via transaction", "distances", dist)
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

	const cmdDone = "done"
	const cmdError = "error"

	p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, any, error) {
		switch command {
		case "show rib":
			data, err := s.showRIB()
			if err != nil {
				return cmdError, "", err
			}
			return cmdDone, data, nil
		case "show nexthop-table":
			data, err := s.showNHTable()
			if err != nil {
				return cmdError, "", err
			}
			return cmdDone, data, nil
		case "show ecmp-groups":
			data, err := s.showECMPGroups()
			if err != nil {
				return cmdError, "", err
			}
			return cmdDone, data, nil
		default:
			return cmdError, "", fmt.Errorf("unknown command: %s", command)
		}
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{configRootRIB},
		VerifyBudget: 1,
		ApplyBudget:  2,
		Commands: []sdk.CommandDecl{
			{Name: "show rib"},
			{Name: "show nexthop-table"},
			{Name: "show ecmp-groups"},
		},
	})
	if err != nil {
		logger().Error("sysrib plugin failed", "error", err)
		return 1
	}

	return 0
}
