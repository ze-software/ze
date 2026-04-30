// Design: plan/spec-static-routes.md -- plugin registration and lifecycle

package static

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	bfdapi "codeberg.org/thomas-mangin/ze/internal/plugins/bfd/api"
	staticschema "codeberg.org/thomas-mangin/ze/internal/plugins/static/schema"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

const pluginName = "static"

func init() {
	reg := registry.Registration{
		Name:                    pluginName,
		Description:             "Static routes: config-driven kernel/VPP route programming with ECMP",
		Features:                "yang",
		YANG:                    staticschema.ZeStaticConfYANG,
		ConfigRoots:             []string{pluginName},
		InProcessConfigVerifier: verifyStaticConfig,
		RunEngine:               runStaticPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
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
		fmt.Fprintf(os.Stderr, "static: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func verifyStaticConfig(sections []sdk.ConfigSection) error {
	for _, section := range sections {
		if section.Root != pluginName {
			continue
		}
		if _, err := parseStaticConfig(section.Data); err != nil {
			return err
		}
	}
	return nil
}

func runStaticPlugin(conn net.Conn) int {
	logger().Debug("static plugin starting (RPC)")

	p := sdk.NewWithConn(pluginName, conn)
	defer func() { _ = p.Close() }()

	backend := newStaticBackend()
	rm := newRouteManager(backend)

	var mu sync.Mutex
	var currentRoutes []staticRoute
	var pendingRoutes []staticRoute

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != pluginName {
				continue
			}
			routes, err := parseStaticConfig(section.Data)
			if err != nil {
				return err
			}
			mu.Lock()
			pendingRoutes = routes
			mu.Unlock()
		}
		return nil
	})

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != pluginName {
				continue
			}
			routes, err := parseStaticConfig(section.Data)
			if err != nil {
				return err
			}
			mu.Lock()
			currentRoutes = routes
			mu.Unlock()
			if applyErr := rm.applyRoutes(routes); applyErr != nil {
				return fmt.Errorf("static routes: %w", applyErr)
			}
			logger().Info("static routes loaded", "count", len(routes))
		}
		return nil
	})

	var activeJournal *sdk.Journal

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		mu.Lock()
		newRoutes := pendingRoutes
		oldRoutes := currentRoutes
		pendingRoutes = nil
		mu.Unlock()

		if newRoutes == nil {
			return nil
		}

		j := sdk.NewJournal()
		err := j.Record(
			func() error {
				if applyErr := rm.applyRoutes(newRoutes); applyErr != nil {
					return fmt.Errorf("static routes apply: %w", applyErr)
				}
				mu.Lock()
				currentRoutes = newRoutes
				mu.Unlock()
				logger().Info("static routes reloaded")
				return nil
			},
			func() error {
				if applyErr := rm.applyRoutes(oldRoutes); applyErr != nil {
					return fmt.Errorf("static routes rollback: %w", applyErr)
				}
				mu.Lock()
				currentRoutes = oldRoutes
				mu.Unlock()
				logger().Info("static routes rolled back")
				return nil
			},
		)
		if err != nil {
			j.Rollback()
			return err
		}

		activeJournal = j
		return nil
	})

	p.OnConfigRollback(func(_ string) error {
		j := activeJournal
		activeJournal = nil
		if j == nil {
			return nil
		}
		if errs := j.Rollback(); len(errs) > 0 {
			return fmt.Errorf("static rollback: %d errors", len(errs))
		}
		return nil
	})

	p.OnStarted(func(_ context.Context) error {
		if svc := bfdapi.GetService(); svc != nil {
			rm.setBFD(svc)
			logger().Info("static: BFD service available")
		} else {
			logger().Info("static: BFD service not available, running without BFD")
		}
		return nil
	})

	p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, string, error) {
		if command == "static show" {
			data := rm.showRoutes()
			out, err := json.Marshal(data)
			if err != nil {
				return "error", "", err
			}
			return "done", string(out), nil
		}
		return "error", "", fmt.Errorf("unknown command: %s", command)
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{pluginName},
		VerifyBudget: 1,
		ApplyBudget:  2,
		Commands: []sdk.CommandDecl{
			{Name: "static show"},
		},
	})
	if err != nil {
		logger().Error("static plugin failed", "error", err)
		return 1
	}

	rm.shutdown()

	if err := backend.close(); err != nil {
		logger().Warn("static: backend close failed", "error", err)
	}

	return 0
}
