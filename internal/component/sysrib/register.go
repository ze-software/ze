package sysrib

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"
	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	sysribyang "github.com/ze-software/ze/internal/component/sysrib/yang"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/rib/distance"
	"github.com/ze-software/ze/internal/core/slogutil"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const configRootRIB = "rib"

func init() {
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code:        routeViewDoctorCode,
		Title:       "The system routing view is unavailable",
		Description: "The system RIB is initializing, reconstructing, stopped, or failed to synchronize its authoritative routing view and forwarding output. The runtime doctor finding reports the actual consumer state and the configured instance's restart command in either placement.",
		Examples:    []string{"ze show doctor", "ze show diagnostic " + routeViewDoctorCode},
	}); err != nil {
		panic("BUG: sysrib routing-view diagnostic registration refused: " + err.Error())
	}
	// The filter attachment points this plugin's config holds, with the route
	// fields their routes carry and a filter there may set (the route field
	// vocabulary, internal/component/policy/fields.go).
	policy.MustRegisterRouteAttachment(configRootRIB, "rib import",
		[]policy.Field{policy.FieldPrefix, policy.FieldFamily, policy.FieldProtocol, policy.FieldNextHop, policy.FieldInterface, policy.FieldMetric, policy.FieldTag, policy.FieldLevel, policy.FieldRouteType},
		[]policy.Field{policy.FieldDistance, policy.FieldTag})
	// The import chain is compiled into the Loc-RIB gate by the policy plugin.
	policy.MustRegisterRIBImport(policy.RIBImport{Attachment: "rib import", Root: configRootRIB, Scope: policy.RIBScopeCentral})

	_ = events.RegisterNamespace(sysribevents.Namespace,
		sysribevents.EventBestChange, sysribevents.EventReplayRequest,
	)

	reg := registry.Registration{
		Name:        configRootRIB,
		Description: "System RIB: resolves authoritative routes and publishes forwarding changes",
		Features:    "yang",
		YANG:        sysribyang.ZeRibConfYANG,
		ConfigRoots: []string{configRootRIB},
		// StartAfter, never a load: BGP can populate the shared authority first,
		// but the system RIB also consumes every other registered route source
		// without a BGP process (./le plugin coupling check).
		StartAfter: []string{"bgp-rib"},
		// policy is a hard dependency of every protocol with a filter attachment:
		// the filter instances its chains name live under the policy root.
		Dependencies:            []string{policy.PluginName},
		InProcessConfigVerifier: verifySysRIBConfig,
		RunEngine:               runSysRIBPlugin,
		Commands:                commandDecls(),
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg metrics.Registry) {
			SetMetricsRegistry(reg)
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
// (internal/core/rib/distance), where the Loc-RIB reads it each time it ranks.
//
// This is not a convenience. locrib.selectBest ranks across protocols BEFORE
// sysrib sees the route: sysrib consumes one already-arbitrated best per
// prefix. A distance that reaches sysrib alone therefore cannot change
// cross-protocol selection, however carefully it was resolved. The seam is how
// the one declaration reaches the only layer that can act on it.
//
// Callers MUST retain an immutable table and reselect the authoritative Loc-RIB
// after publication. The authenticated routing-view distance handler performs
// that operation and acknowledges its completed authority barrier.
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
	_, _, err := parseRouteViewConfiguration(sections)
	return err
}

func runSysRIBPlugin(conn net.Conn) int {
	logger().Debug("sysrib plugin starting (RPC)")
	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := runRouteViewPlugin(ctx, sdk.NewWithConn(configRootRIB, conn)); err != nil {
		logger().Error("sysrib plugin failed", "error", err, "code", diagnostic.CodeDoctorPluginStopped)
		return 1
	}
	return 0
}

// runRouteViewPlugin owns the consumer and SDK lifetimes. It MUST join and
// close a successfully opened client before releasing its SDK connection.
func runRouteViewPlugin(ctx context.Context, p *sdk.Plugin) (err error) {
	health := newRouteViewHealth(p.MetricsRegistry(), p.InstanceName)
	p.OnDoctorCheck(health.findings)
	var client *routeViewClient
	defer func() {
		if client != nil {
			err = errors.Join(err, client.Close(ctx), client.Failure())
		}
		if closeErr := p.Close(); closeErr != nil {
			if !errors.Is(closeErr, net.ErrClosed) {
				err = errors.Join(err, closeErr)
			}
		}
	}()
	initial, _, err := parseRouteViewConfiguration(nil)
	if err != nil {
		return err
	}
	s := newSysRIB()
	// ConfigTx serializes its installer. The boot installer transitions only
	// after Open and the complete daemon distance declaration are acknowledged;
	// every later configure, apply and rollback uses that same valid lease.
	var install sdk.ConfigInstaller[routeViewConfiguration]
	install = func(cfg routeViewConfiguration) error {
		opened, err := openRouteViewMonitoredClient(ctx, p, s, cfg.distances, cfg.permitted, health)
		if err != nil {
			return err
		}
		client = opened
		p.OnStarted(opened.Start)
		p.OnEvent(opened.HandleEvent)
		p.OnExecuteCommand(routeViewCommandHandler(ctx, s, opened))
		install = func(cfg routeViewConfiguration) error {
			return opened.ApplyConfiguration(ctx, cfg.distances, cfg.permitted)
		}
		return nil
	}
	tx := sdk.NewConfigTx(initial, parseRouteViewConfiguration, func(cfg routeViewConfiguration) error {
		return install(cfg)
	})
	// Register MUST precede Run. The parser declares defaults even when Stage2
	// delivers no rib section, so the boot installer always reaches the daemon.
	tx.Register(p)
	p.SetStartupSubscriptionsIn(sysribevents.Namespace, []string{sysribevents.EventReplayRequest}, nil, "json")
	p.SetEnvelope(true)
	return p.Run(ctx, &sdk.Registration{
		WantsConfig:  []string{configRootRIB},
		VerifyBudget: 1,
		ApplyBudget:  int(routeViewConfigurationApplyBudget / time.Second),
		Commands:     commandDecls(),
		DoctorChecks: routeViewDoctorDeclarations(),
	})
}

func routeViewCommandHandler(ctx context.Context, s *sysRIB, client *routeViewClient) sdk.ExecuteCommandHandler {
	const cmdDone = "done"
	const cmdError = "error"
	return func(_, command string, args []string, _ string) (string, any, error) {
		switch command {
		case "show rib":
			data, err := s.showRIB()
			if err != nil {
				return cmdError, "", err
			}
			return cmdDone, data, nil
		case "show rib candidates":
			data, err := client.Candidates(ctx, args)
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
			return cmdError, "", opererr.New("Unknown system routing command: "+command,
				"The system RIB serves show rib, show rib candidates, show nexthop-table and show ecmp-groups",
				"ze show rib")
		}
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
			Name:      "show rib",
			ShortHelp: "Show each route the system RIB holds, with its family, next hop, protocol and equal-cost paths.",
		},
		{
			Name:      "show rib candidates",
			ShortHelp: "Show every path each protocol offered the Loc-RIB, hidden ones included, with the import chain that hid each.",
			Description: "Lists, per prefix, every path a protocol offered: the selected one, the ones standing by, and the ones an import chain " +
				"hid. Each path shows its source, next hop, distance and metric, its tag, IS-IS level or OSPF route type when it carries " +
				"one, and for a hidden path the chain that hid it: `source` (the protocol's own `filter import`) or `central` " +
				"(`rib filter import`). `prefix <prefix>` limits the answer to one prefix.",
			// The selector is optional: with no argument every prefix is listed.
			Args: []string{candidatesSelectorPrefix, "<prefix>"},
		},
		{
			Name:      "show nexthop-table",
			ShortHelp: "Show each next hop the resolver tracks and the direct next hop it resolves to.",
		},
		{
			Name:      "show ecmp-groups",
			ShortHelp: "Show each prefix the system RIB holds equal-cost paths for, whether or not Ze programs them.",
		},
	}
}
