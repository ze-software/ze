// Design: plan/spec-static-routes.md -- plugin registration and lifecycle

package static

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	bfdapi "codeberg.org/thomas-mangin/ze/internal/plugins/bfd/api"
	staticschema "codeberg.org/thomas-mangin/ze/internal/plugins/static/schema"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

var staticProtocolID redistevents.ProtocolID

func init() {
	staticProtocolID = redistevents.RegisterProtocol("static")
	redistevents.RegisterProducer(staticProtocolID)

	reg := registry.Registration{
		Name:        "static",
		Description: "Static routes: config-driven kernel/VPP route programming with ECMP",
		Features:    "yang",
		YANG:        staticschema.ZeStaticConfYANG,
		ConfigRoots: []string{"static"},
		RunEngine:   runStaticPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
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

func runStaticPlugin(conn net.Conn) int {
	logger().Debug("static plugin starting (RPC)")

	p := sdk.NewWithConn("static", conn)
	defer func() { _ = p.Close() }()

	backend := newStaticBackend()
	rm := newRouteManager(backend)

	var mu sync.Mutex
	var currentRoutes []staticRoute
	var pendingRoutes []staticRoute

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != "static" {
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
			if section.Root != "static" {
				continue
			}
			routes, err := parseStaticConfig(section.Data)
			if err != nil {
				return err
			}
			mu.Lock()
			currentRoutes = routes
			mu.Unlock()
			rm.applyRoutes(routes)
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
				rm.applyRoutes(newRoutes)
				mu.Lock()
				currentRoutes = newRoutes
				mu.Unlock()
				logger().Info("static routes reloaded")
				return nil
			},
			func() error {
				rm.applyRoutes(oldRoutes)
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

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{"static"},
		VerifyBudget: 1,
		ApplyBudget:  2,
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

