package redistribute

import (
	"context"
	"net"
	"strings"
	"sync"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/issuequeue"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

// Compile-time contract: the SDK plugin is the engine connection a consumer
// factory builds on (configredist.ConsumerFactory, buildFactoryConsumers).
var _ configredist.RouteUpdater = (*sdk.Plugin)(nil)

func init() {
	// The filter attachment points this plugin's config holds, with the route
	// fields their routes carry and a filter there may set (the route field
	// vocabulary, internal/component/policy/fields.go).
	// RFC 8893 Section 3: redistribution policy carries rpki-validation-state
	// using the same primitive as BGP ingress and egress policy.
	policy.MustRegisterRouteAttachment(Name, chainAttachment,
		[]policy.Field{policy.FieldPrefix, policy.FieldFamily, policy.FieldProtocol, policy.FieldNextHop, policy.FieldMetric, policy.FieldTag, policy.FieldRPKIValidationState},
		[]policy.Field{policy.FieldTag, policy.FieldMetric})

	// A withdrawal its consumer could not confirm (withdrawal_diagnostic.go).
	if err := diagnostic.Register(unconfirmedWithdrawalMeta); err != nil {
		panic("BUG: redistribution withdrawal diagnostic registration: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: unconfirmedWithdrawalCode}); err != nil {
		panic("BUG: redistribution withdrawal event registration: " + err.Error())
	}

	reg := registry.Registration{
		Name:        Name,
		Description: "Redistribute orchestrator: dispatches protocol route events to registered consumers",
		ConfigRoots: []string{configRoot},
		// policy: each import's chain names instances under the policy root,
		// which this plugin compiles itself (compileRuleChains).
		ConfigReads: []string{policy.RootName},
		// policy: the redistribute import chains name filters under the policy root.
		Dependencies: []string{policy.PluginName},
		// bgp is order only. A destination such as BGP whose consumer a
		// factory builds over this plugin's connection (buildFactoryConsumers)
		// needs its engine up before the consumer sends, and its peer state
		// events are what fire the peer-up replay. A config with no bgp block
		// starts no bgp: redistribution between other protocols needs none, and
		// a rule naming BGP without the block is refused before startup
		// (config.RefuseRedistributeRules).
		StartAfter: []string{"bgp"},
		RunEngine:  runPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(r metrics.Registry) {
			setMetricsRegistry(r)
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
		panic("BUG: " + Name + " registration failed: " + err.Error())
	}
}

func runPlugin(conn net.Conn) int {
	logger().Debug(Name + " plugin starting (RPC)")

	p := sdk.NewWithConn(Name, conn)
	defer func() { _ = p.Close() }()
	// The consumer episodes a previous run dispatched to ended with it.
	consumerHeld.beginRun()

	ctx, cancel := sdk.SignalContext()
	issues := issuequeue.Start(p)
	runtimeCtx := withRedistributionIssues(ctx, issues)
	var workers sync.WaitGroup
	// OnStarted runs synchronously before Run enters its event loop. Its run
	// worker MUST finish before the original queue stops and p closes. Stop
	// seals report admission and drains accepted reports. A copied event
	// callback that runs later keeps this queue and logs its late refusal.
	defer func() {
		cancel()
		workers.Wait()
		issues.Stop()
	}()

	// Peer-up trigger: fire a targeted redistribute replay to a peer that
	// establishes after an injection (spec-redistribute-late-join-replay).
	coord := newReplayCoordinator()
	setReplayCoordinator(coord)

	// The import rules are this plugin's to install: it owns the redistribute
	// root, and the evaluator it reads is the one it sets.
	rules := &ruleState{runtimeContext: runtimeCtx}
	p.OnConfigure(rules.configure)
	p.OnConfigVerify(rules.verify)
	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error { return rules.apply() })
	p.OnConfigRollback(func(_ string) error { return rules.rollback() })
	p.OnConfigCommitted(func(_ string) error {
		rules.committed()
		return nil
	})

	factoryConsumers := constructFactoryConsumers(p)
	p.OnStarted(func(ctx context.Context) error {
		publishFactoryConsumers(factoryConsumers)
		workers.Go(func() {
			run(withRedistributionIssues(ctx, issues))
		})
		return nil
	})

	// Subscribe to peer state; on a down->up edge fire a targeted replay so a
	// newly-established peer receives the current redistribute route set. Mirrors
	// the watchdog's state-subscription pattern (in-process DirectBridge via
	// OnStructuredEvent, text fallback via OnEvent).
	// The state events are BGP's. A build without BGP registers no bgp
	// namespace, so the plugin subscribes to nothing rather than ask for a
	// namespace the engine cannot resolve.
	if events.IsValidNamespace(bgpStateNamespace) {
		p.SetStartupSubscriptionsIn(bgpStateNamespace, []string{"state"}, nil, "")
	}
	p.SetEncoding("text")
	p.OnStructuredEvent(func(events []any) error {
		bus := getEventBus()
		for _, event := range events {
			se, ok := event.(*rpc.StructuredEvent)
			if !ok {
				continue
			}
			if se.PeerAddress == "" {
				continue
			}
			if se.State == rpc.SessionStateUp {
				coord.onStateUp(bus, se.PeerAddress, se.Reason)
			} else if se.State != rpc.SessionStateUnspecified {
				coord.onPeerDown(se.PeerAddress)
			}
		}
		return nil
	})
	p.OnEvent(func(eventStr string) error {
		peerAddr, state, reason := parseStateEvent(eventStr)
		if peerAddr == "" {
			return nil
		}
		bus := getEventBus()
		if state == "up" {
			coord.onStateUp(bus, peerAddr, reason)
		} else {
			coord.onPeerDown(peerAddr)
		}
		return nil
	})

	if err := p.Run(ctx, &sdk.Registration{WantsConfig: []string{configRoot, policy.RootName}}); err != nil {
		logger().Error(Name+" plugin failed", "error", err)
		return 1
	}
	return 0
}

// bgpStateNamespace is the event namespace the BGP peer state events live in.
const bgpStateNamespace = "bgp"

// Consumer constructors register optional SDK callbacks before Run. Registry
// publication still waits for Started, because it can trigger source replay.
// A destination registers its factory from init; this orchestrator imports no
// protocol and a build without the destination carries no factory.
// constructFactoryConsumers MUST precede SDK.Run: constructors register their
// optional callbacks. publishFactoryConsumers MUST follow actual SDK readiness.
func constructFactoryConsumers(updater configredist.RouteUpdater) []configredist.RedistConsumer {
	factories := configredist.ConsumerFactories()
	consumers := make([]configredist.RedistConsumer, 0, len(factories))
	for _, factory := range factories {
		consumers = append(consumers, factory.New(updater))
	}
	return consumers
}

// publishFactoryConsumers MUST follow constructFactoryConsumers and Started.
// Registry visibility may fire a producer replay and therefore requires ready I/O.
func publishFactoryConsumers(consumers []configredist.RedistConsumer) {
	for _, consumer := range consumers {
		if configredist.ReregisterConsumer(consumer) {
			logger().Debug(Name+": consumer rebuilt", "destination", consumer.Name())
		}
	}
}

// parseStateEvent extracts the peer address, state and reason from a text state event
// (the non-DirectBridge / external-plugin delivery path). Format:
// "peer 10.0.0.1 remote as 65001 state up [reason attached] initial-replay 42\n".
// It reads the value after state, not the final token; replay receipts are not
// relevant to this non-readiness consumer. Unrecognized lines return ("", "", "").
func parseStateEvent(text string) (peerAddr, state, reason string) {
	fields := strings.Fields(strings.TrimRight(text, "\n"))
	if len(fields) < 4 {
		return "", "", ""
	}
	if fields[0] != "peer" {
		return "", "", ""
	}
	for i := 2; i < len(fields)-1; i++ {
		switch fields[i] {
		case "state":
			i++
			state = fields[i]
		case "reason":
			i++
			reason = fields[i]
		}
	}
	if state == "" {
		return "", "", ""
	}
	return fields[1], state, reason
}
