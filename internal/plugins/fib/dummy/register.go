// Design: docs/architecture/core-design.md -- FIB dummy plugin
// Related: fibdummy.go -- held route table, best-change handler, counters, metrics
// Related: show.go -- command answer shapes and builders
//
// Package fibdummy implements the fib-dummy plugin, which receives the routes
// the system RIB selects, holds them in memory, counts changes, and writes them
// to no forwarding table.
package fibdummy

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/configvalue"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/internal/core/slogutil"
	fibdummyyang "github.com/ze-software/ze/internal/plugins/fib/dummy/yang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

const (
	pluginName      = "fib-dummy"
	configRoot      = "fib/dummy"
	cmdShowRoutes   = "show fib dummy"
	cmdShowCounters = "show fib dummy counters"
)

// fibDummyMetrics holds the Prometheus metrics exported by fib-dummy.
type fibDummyMetrics struct {
	routesHeld          metrics.Gauge
	routeInstalls       metrics.Counter
	routeReplaces       metrics.Counter
	routeRemovals       metrics.Counter
	labelEntriesRefused metrics.Counter
}

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

// eventBusPtr stores the EventBus configured by the daemon.
var eventBusPtr atomic.Pointer[ze.EventBus]

// metricsPtr stores the Prometheus metrics registered by SetMetricsRegistry.
var metricsPtr atomic.Pointer[fibDummyMetrics]

func init() {
	loggerPtr.Store(slogutil.DiscardLogger())

	reg := registry.Registration{
		Name:         pluginName,
		Description:  "FIB dummy: holds selected system RIB routes in memory and programs no forwarding table",
		Features:     "yang",
		ConfigRoots:  []string{configRoot},
		Dependencies: []string{"rib"},
		// ProgramsFIB with no DataPlane: `fib { dummy { } }` IS the operator's
		// FIB choice, so Ze adds no data-plane writer beside it, and fib-dummy
		// is never resolved as a route producer's data-plane writer.
		ProgramsFIB:             true,
		YANG:                    fibdummyyang.ZeFibDummyConfYANG,
		InProcessConfigVerifier: verifyFIBDummyConfig,
		Sends:                   fibDummySends(),
		Receives:                fibDummyReceives(),
		RunEngine:               runFIBDummyPlugin,
		Commands:                commandDecls(),
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
		panic("BUG: fib-dummy: registration failed: " + err.Error())
	}
}

func logger() *slog.Logger { return loggerPtr.Load() }

func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

func setEventBus(eb ze.EventBus) {
	if eb != nil {
		eventBusPtr.Store(&eb)
	}
}

func loadEventBus() ze.EventBus {
	p := eventBusPtr.Load()
	if p == nil {
		return nil
	}
	return *p
}

// SetMetricsRegistry creates the fib-dummy metrics on reg.
// Called via ConfigureMetrics before RunEngine starts.
func SetMetricsRegistry(reg metrics.Registry) {
	m := &fibDummyMetrics{
		routesHeld:          reg.Gauge("ze_fibdummy_routes_held", "Current number of system RIB routes held in memory by fib-dummy and written nowhere."),
		routeInstalls:       reg.Counter("ze_fibdummy_route_installs_total", "Route install changes received by fib-dummy since startup."),
		routeReplaces:       reg.Counter("ze_fibdummy_route_replaces_total", "Route replace changes received by fib-dummy since startup."),
		routeRemovals:       reg.Counter("ze_fibdummy_route_removals_total", "Route removal changes received by fib-dummy since startup."),
		labelEntriesRefused: reg.Counter("ze_fibdummy_label_entries_refused_total", "MPLS label entries refused by fib-dummy since startup."),
	}
	metricsPtr.Store(m)
}

func fibDummySends() []rpc.MessageSendDecl {
	return []rpc.MessageSendDecl{
		{Message: sysribevents.Namespace + ":" + sysribevents.EventReplayRequest},
		{Message: mplsfib.Namespace + ":" + mplsfib.EventReplayRequest},
	}
}

func fibDummyReceives() []rpc.MessageReceiveDecl {
	return []rpc.MessageReceiveDecl{
		{Message: sysribevents.Namespace + ":" + sysribevents.EventBestChange},
	}
}

// commandDecls names the commands this plugin serves and states what each
// answer holds (pkg/plugin/rpc/types.go, CommandDecl).
//
// It has two readers and MUST stay one function. init() puts it on the
// registry.Registration, which anything linking the composition root reads
// without starting an engine, and the runner sends it in the Stage 1
// registration message, which a running daemon reads.
func commandDecls() []sdk.CommandDecl {
	return []sdk.CommandDecl{
		{
			Name:      cmdShowRoutes,
			ShortHelp: "Show the routes fib-dummy holds and programs nowhere.",
			Description: "Lists every best route the system RIB selected and handed to fib-dummy. " +
				"Each row is held in memory only and written to no forwarding table, so every row carries programmed=no. " +
				"Named-table static routes, policy routing, the IPsec tunnel-source guard, and interface addresses are outside the system RIB and untouched.",
			Shape:         rpc.AnswerTypeTable,
			Columns:       []string{"table", "prefix", "protocol", "route-type", "next-hop", "interface", "paths", "programmed"},
			AddressFields: []string{"prefix", "next-hop"},
		},
		{
			Name:      cmdShowCounters,
			ShortHelp: "Show the route and label counters fib-dummy holds and programs nowhere.",
			Description: "Reports the count of routes fib-dummy holds in memory, the install, replace, removal, invalid-change and refused MPLS label totals since startup, " +
				"and forwarding=none to confirm no route is written to any forwarding table.",
			Shape: rpc.AnswerTypeDocument,
		},
	}
}

func engineRegistration() *sdk.Registration {
	return &sdk.Registration{
		WantsConfig:  []string{configRoot},
		VerifyBudget: 1,
		ApplyBudget:  1,
		Commands:     commandDecls(),
		Sends:        fibDummySends(),
		Receives:     fibDummyReceives(),
	}
}

func parseFIBDummyConfig(sections []sdk.ConfigSection) error {
	for _, sec := range sections {
		if sec.Root != configRoot {
			continue
		}
		if sec.Data == "" {
			continue
		}
		var delivered map[string]any
		if err := json.Unmarshal([]byte(sec.Data), &delivered); err != nil {
			return fmt.Errorf("fib/dummy: invalid config JSON: %w", err)
		}
		// An empty object is the root being removed during a config reload, not
		// a malformed section. Accept it so an operator can delete `fib { dummy { } }`.
		if len(delivered) == 0 {
			continue
		}
		if configvalue.Section(configRoot, delivered) == nil {
			return fmt.Errorf("fib/dummy: config section carries no %q object", configRoot)
		}
	}
	return nil
}

func verifyFIBDummyConfig(sections []sdk.ConfigSection) error {
	return parseFIBDummyConfig(sections)
}

func runFIBDummyPlugin(conn net.Conn) int {
	logger().Debug("fib-dummy plugin starting (RPC)")

	p := sdk.NewWithConn(pluginName, conn)
	defer func() { _ = p.Close() }()

	f := newDummyFIB(time.Now().UTC())
	var activeJournal *sdk.Journal

	var unsubMu sync.Mutex
	var unsubBestChange func()
	unsubscribeAll := func() {
		unsubMu.Lock()
		fn := unsubBestChange
		unsubBestChange = nil
		unsubMu.Unlock()
		if fn != nil {
			fn()
		}
	}
	defer unsubscribeAll()

	p.OnConfigVerify(verifyFIBDummyConfig)

	p.OnConfigure(parseFIBDummyConfig)

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		activeJournal = sdk.NewJournal()
		logger().Info("fib-dummy config applied via transaction")
		return nil
	})

	p.OnConfigRollback(func(_ string) error {
		j := activeJournal
		activeJournal = nil
		if j != nil {
			if errs := j.Rollback(); len(errs) > 0 {
				return fmt.Errorf("fib-dummy rollback: %d errors", len(errs))
			}
			logger().Info("fib-dummy config rolled back")
		}
		return nil
	})

	p.OnBye(func(_ string) error {
		unsubscribeAll()
		return nil
	})

	p.SetEventFormat("json")
	p.SetEnvelope(true)
	p.OnEvent(func(event string) error {
		// In-daemon runs consume typed *BestChangeBatch directly on the
		// EventBus so deliverEvent avoids JSON marshaling on the hot path.
		if loadEventBus() != nil {
			return nil
		}
		return f.handleEvent(event)
	})

	p.OnStarted(func(startedCtx context.Context) error {
		if eb := loadEventBus(); eb != nil {
			unsub := sysribevents.BestChange.Subscribe(eb, f.applyBatch)
			unsubMu.Lock()
			unsubBestChange = unsub
			unsubMu.Unlock()
			// Drop the Stage 5 RPC subscription so deliverEvent skips JSON
			// marshaling when only in-daemon subscribers listen.
			if err := p.UnsubscribeEvents(startedCtx); err != nil {
				return fmt.Errorf("fib-dummy: clear rpc subscription: %w", err)
			}
			if _, err := sysribevents.ReplayRequest.Emit(eb, &replay.Request{ReplayID: replay.Broadcast}); err != nil {
				return fmt.Errorf("fib-dummy: emit replay request: %w", err)
			}
			return nil
		}

		reqBody, err := json.Marshal(&replay.Request{ReplayID: replay.Broadcast})
		if err != nil {
			return fmt.Errorf("fib-dummy: marshal replay request: %w", err)
		}
		if _, err := p.EmitEvent(startedCtx, sysribevents.Namespace, sysribevents.EventReplayRequest, "", "", string(reqBody)); err != nil {
			return fmt.Errorf("fib-dummy: emit replay request: %w", err)
		}
		return nil
	})

	p.OnExecuteCommand(func(_, command string, _ []string, _ string, _ []sdk.PipeOperation) (string, any, error) {
		switch command {
		case cmdShowRoutes:
			return rpc.StatusDone, f.routeRows(), nil
		case cmdShowCounters:
			return rpc.StatusDone, f.counters(), nil
		default:
			return rpc.StatusError, "", opererr.New(
				"Unknown fib-dummy command: "+command,
				"fib-dummy serves show fib dummy and show fib dummy counters",
				"ze show fib dummy",
			)
		}
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, engineRegistration()); err != nil {
		logger().Error("fib-dummy plugin failed", "error", err, "code", diagnostic.CodeDoctorPluginStopped)
		return 1
	}

	return 0
}
