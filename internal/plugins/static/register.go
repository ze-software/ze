// Design: plan/learned/650-static-routes.md -- plugin registration and lifecycle

package static

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"

	bfdapi "codeberg.org/thomas-mangin/ze/internal/component/bfd/api"
	"codeberg.org/thomas-mangin/ze/internal/component/config/redistribute"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
	"codeberg.org/thomas-mangin/ze/internal/core/routingtable"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	staticyang "codeberg.org/thomas-mangin/ze/internal/plugins/static/yang"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

const pluginName = "static"

var sourcesOnce sync.Once

// registerStaticSources registers "static" as a redistribute source so
// `redistribute { destination <proto> { import static } }` resolves. Called from
// init() (not plugin run) so the source is visible to `ze config validate`, which
// imports plugins but does not start their engines. The static plugin already emits
// route-change events (inject.go emitRouteChange); this closes the config side.
func registerStaticSources() {
	sourcesOnce.Do(func() {
		_ = redistribute.RegisterSource(redistribute.RouteSource{
			Name:        pluginName,
			Protocol:    pluginName,
			Description: "static routes",
		})
	})
}

func init() {
	registerStaticSources()

	reg := registry.Registration{
		Name:         pluginName,
		Description:  "Static routes: config-driven kernel/VPP route programming with ECMP",
		Features:     "yang",
		YANG:         staticyang.ZeStaticConfYANG,
		ConfigRoots:  []string{pluginName},
		Dependencies: []string{"routing-table"},
		// OptionalDependencies orders static AFTER the iface component when an
		// `interface` stanza is present, so the iface backend is loaded before
		// static applies a route whose next-hop names an interface. Without it
		// both plugins land in the same startup tier and static's resolve races
		// LoadBackend (spec-fixit-static-interface-nexthops A-1c). Optional, so
		// a config with no `interface` stanza leaves static unconstrained.
		OptionalDependencies:    []string{"interface"},
		InProcessConfigVerifier: verifyStaticConfig,
		DoctorChecks:            staticDoctorChecks(),
		RunEngine:               runStaticPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
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
		fmt.Fprintf(os.Stderr, "static: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func verifyStaticConfig(sections []sdk.ConfigSection) error {
	for _, section := range sections {
		if section.Root != pluginName {
			continue
		}
		if _, err := parseStaticConfig(section.Data, routingtable.GetRegistry()); err != nil {
			return err
		}
	}
	return nil
}

// warnIfExternal logs a warning when static is not running in-process.
//
// resolveNexthopIndex (backend_linux.go) resolves a route's next-hop INTERFACE
// name through iface.Resolve / iface.GetBackend, and the iface component lives
// in the HOST process. An external static plugin therefore sees
// iface.GetBackend() == nil for its entire lifetime no matter how the operator
// configured `interface { backend ... }`, so every interface-next-hop route
// fails with the misleading "no interface backend loaded" message and nothing
// else explains why.
//
// Unlike as112 / trafficusage / flowexport -- which REFUSE to start when
// external, because the same-process call is their whole purpose -- static
// still provides real value external: gateway and device next-hop routes are
// installed through netlink unaffected. So this warns rather than refuses,
// matching internal/plugins/cos/register.go.
func warnIfExternal(isInternal bool) {
	if isInternal {
		return
	}
	logger().Warn("static: running as an external plugin process -- routes whose next-hop is an INTERFACE name cannot be resolved (the iface backend lives in the host process, so this process always sees none) and will fail with 'no interface backend loaded'; run static internal if you use interface next-hops. Gateway and device next-hops are unaffected.")
}

func runStaticPlugin(conn net.Conn) int {
	logger().Debug("static plugin starting (RPC)")

	p := sdk.NewWithConn(pluginName, conn)
	warnIfExternal(p.IsInternal())
	defer func() { _ = p.Close() }()

	backend := newStaticBackend()
	rm := newRouteManager(backend)

	// Redistribute late-join replay: on a ReplayRequest re-emit the current
	// static-route set tagged with the echoed ReplayID so a peer that
	// established after injection receives them (spec-redistribute-late-join-replay).
	if bus := getEventBus(); bus != nil {
		unsub := redistevents.ReplayRequestEvent.Subscribe(bus, func(r *redistevents.ReplayRequest) {
			rm.reemitAll(r.ReplayID)
		})
		defer unsub()
	}

	var mu sync.Mutex
	var currentRoutes []staticRoute
	var pendingRoutes []staticRoute

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != pluginName {
				continue
			}
			routes, err := parseStaticConfig(section.Data, routingtable.GetRegistry())
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
			routes, err := parseStaticConfig(section.Data, routingtable.GetRegistry())
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

	p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, any, error) {
		if command == "show static" {
			data := rm.showRoutes()
			return "done", data, nil
		}
		return "error", "", fmt.Errorf("unknown command: %s", command)
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		// The "interface" root is requested so config-time validation of an
		// interface-only next-hop's reference is possible (the payload is
		// otherwise blind to interface config: BuildPluginConfigSections sends
		// only declared roots). The verify/configure handlers below still skip
		// non-"static" sections, so an interface-only reload is a parse + diff
		// no-op (spec-fixit-static-interface-nexthops C-8/R-10/R-11).
		WantsConfig:  []string{pluginName, "interface"},
		VerifyBudget: 1,
		ApplyBudget:  2,
		Commands: []sdk.CommandDecl{
			{Name: "show static"},
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
