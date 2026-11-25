// Design: docs/architecture/api/architecture.md -- reusable external plugin fixtures for worked bus scenarios S1-S44.
// Overview: register_bus_scenario_fixtures.go -- driver registrations for worked bus scenario fixtures.
// Related: bus_send_fixtures.go -- busRunPlugin, busPollFile, busWriteAtomicFile, busAssertUndeclaredRefusal, busAssertAllowListRefusal.
// Related: bus_implied_fixtures.go -- busAssertImpliedDriver for | implied structural assertions.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	busMsgBGPRefresh     = "bgp:refresh"
	busMsgBGPRouteChange = "bgp:route-change"

	busTagDirection    = "direction"
	busTagDirReceived  = "received"
	busTagSource       = "source"
	busTagSourceBGP    = "bgp"
	busTagSourceStatic = "static"

	busSenderModeUpdate           = "update"
	busSenderModeUpdateAndRefresh = "update-and-refresh"

	busFlagUndeclaredUpdate = "--undeclared-update"

	busStateNoGrantMarker = "holds no grant to write state key"
)

type busSendExpect uint8

const (
	busSendExpectUnspecified busSendExpect = iota
	busSendExpectOK
	busSendExpectRefused
	busSendExpectAllowRefused
)

type busEventLogRecorder struct {
	mu       sync.Mutex
	outFile  string
	recorded []string
}

func (r *busEventLogRecorder) appendEvent(event string) error {
	r.mu.Lock()
	r.recorded = append(r.recorded, strings.TrimSpace(event))
	var body textbuf.Buffer
	for _, line := range r.recorded {
		body.Str(line).Byte('\n')
	}
	snapshot := body.String()
	r.mu.Unlock()
	return busWriteAtomicFile(r.outFile, snapshot)
}

func busBGPReceivedUpdateDecl() sdk.MessageReceiveDecl {
	return sdk.MessageReceiveDecl{
		Message: busMsgBGPUpdate,
		Tags: []rpc.MessageTagDecl{{
			Name:   busTagDirection,
			Values: []string{busTagDirReceived},
		}},
	}
}

// busRunScenarioPlugin answers Stage 1 declaration query mode through
// sdk.RunOrDeclare before opening any connection to the daemon, and otherwise
// runs the live plugin session through busRunPlugin.
func busRunScenarioPlugin(
	ctx context.Context,
	fallbackName string,
	reg *sdk.Registration,
	setup func(*sdk.Plugin) error,
	scenario ObserverScenario,
) error {
	var runErr error
	code := sdk.RunOrDeclare(reg, func() int {
		runErr = busRunPlugin(ctx, fallbackName, reg, setup, scenario)
		if runErr != nil {
			return 1
		}
		return 0
	})
	if runErr != nil {
		return runErr
	}
	if code != 0 {
		return fmt.Errorf("plugin %s exited %d", fallbackName, code)
	}
	return nil
}

func busUpdateRouteCommand(prefix string) string {
	var tb textbuf.Buffer
	return tb.Str("update text origin igp nhop 192.0.2.1 nlri add ").Str(prefix).String()
}

func parseBusSendExpect(raw string) (busSendExpect, error) {
	switch raw {
	case "ok", "expect-ok":
		return busSendExpectOK, nil
	case "expect-refused", "refused":
		return busSendExpectRefused, nil
	case "allow-refused", "expect-allow-refused":
		return busSendExpectAllowRefused, nil
	default:
		return busSendExpectUnspecified, fmt.Errorf("unknown send expectation %q", raw)
	}
}

func busAssertSendRefused(operation string, err error) error {
	if err == nil {
		return fmt.Errorf("%s succeeded, want send refusal", operation)
	}
	refusal := err.Error()
	if baseErr := busCheckRefusalBase(operation, refusal); baseErr != nil {
		return baseErr
	}
	if strings.Contains(refusal, "may not send") {
		return nil
	}
	if strings.Contains(refusal, "refused") {
		return nil
	}
	if strings.Contains(refusal, "cannot send") {
		return nil
	}
	return fmt.Errorf("%s failed with non-bus error: %s", operation, refusal)
}

func checkBusUpdateExpect(
	expect busSendExpect,
	announced uint32,
	err error,
	epPath, epName string,
) error {
	switch expect {
	case busSendExpectOK:
		if err != nil {
			return fmt.Errorf("update-route bgp:update failed: %w", err)
		}
		if announced < 1 {
			return fmt.Errorf("update-route bgp:update announced=%d, want >= 1", announced)
		}
		return nil
	case busSendExpectRefused:
		return busAssertSendRefused("update-route bgp:update", err)
	case busSendExpectAllowRefused:
		return busAssertAllowListRefusal("update-route bgp:update", err, epPath, epName, busMsgBGPUpdate)
	case busSendExpectUnspecified:
		panic("BUG: fixture: unspecified busSendExpect for update")
	default:
		panic("BUG: fixture: unknown busSendExpect for update")
	}
}

func checkBusRefreshExpect(
	expect busSendExpect,
	err error,
	epPath, epName string,
) error {
	switch expect {
	case busSendExpectOK:
		if err != nil {
			return fmt.Errorf("emit-event bgp:refresh failed: %w", err)
		}
		return nil
	case busSendExpectRefused:
		return busAssertSendRefused("emit-event bgp:refresh", err)
	case busSendExpectAllowRefused:
		return busAssertAllowListRefusal("emit-event bgp:refresh", err, epPath, epName, busMsgBGPRefresh)
	case busSendExpectUnspecified:
		panic("BUG: fixture: unspecified busSendExpect for refresh")
	default:
		panic("BUG: fixture: unknown busSendExpect for refresh")
	}
}

// busBGPCollectorDriver implements an external collector endpoint declaring
// reception of bgp:state and bgp:update (direction=received) and no sends. It
// records every delivered JSON event line to outFile, and optionally waits for
// a go marker to attempt an undeclared UpdateRoute and assert Reason 1 refusal.
// Usage: plugin/bus-bgp-collector <out-file> [--undeclared-update <go-file> <peer-selector> <done-file>].
func busBGPCollectorDriver(ctx context.Context, args []string) error {
	var outFile, goFile, peerSel, doneFile string
	switch len(args) {
	case 1:
		outFile = args[0]
	case 5:
		if args[1] != busFlagUndeclaredUpdate {
			return fmt.Errorf("bus-bgp-collector: expected %s flag, got %q", busFlagUndeclaredUpdate, args[1])
		}
		outFile = args[0]
		goFile = args[2]
		peerSel = args[3]
		doneFile = args[4]
	default:
		return errors.New("usage: plugin/bus-bgp-collector <out-file> [--undeclared-update <go-file> <peer-selector> <done-file>]")
	}

	reg := &sdk.Registration{
		Receives: []sdk.MessageReceiveDecl{
			{Message: busMsgBGPState},
			busBGPReceivedUpdateDecl(),
		},
	}

	recorder := &busEventLogRecorder{outFile: outFile}
	var scenario ObserverScenario
	if goFile != "" {
		scenario = func(ctx context.Context, p *sdk.Plugin) error {
			epPath := busEndpointPath(busCurrentEndpointName("collector"))
			if err := busPollFile(ctx, goFile); err != nil {
				return fmt.Errorf("wait for %s: %w", goFile, err)
			}
			_, _, err := p.UpdateRoute(ctx, peerSel, busUpdateRouteCommand("10.0.1.0/24"))
			if err := busAssertUndeclaredRefusal("update-route bgp:update", err, epPath, busMsgBGPUpdate); err != nil {
				return err
			}
			if err := busWriteAtomicFile(doneFile, "refused\n"); err != nil {
				return err
			}
			var tb textbuf.Buffer
			return tb.Str("OK: undeclared bgp:update send from ").Str(epPath).Str(" was refused\n").StdErr()
		}
	}

	return busRunScenarioPlugin(ctx, "collector", reg, func(p *sdk.Plugin) error {
		p.OnEvent(recorder.appendEvent)
		return nil
	}, scenario)
}

// busBGPSenderDriver implements an external BGP sender endpoint for S6, S7,
// and S9. Mode "update" declares bgp:update and sends one UpdateRoute after
// waiting for goFile; mode "update-and-refresh" declares bgp:update and
// bgp:refresh and sends both after waiting for goFile, asserting each outcome.
// Usage:
//
//	plugin/bus-bgp-sender update <go-file> <peer-selector> <prefix> <expect> <done-file>
//	plugin/bus-bgp-sender update-and-refresh <go-file> <peer-selector> <prefix> <update-expect> <refresh-expect> <done-file>.
func busBGPSenderDriver(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return errors.New("usage: plugin/bus-bgp-sender <update|update-and-refresh> ...")
	}
	mode := args[0]
	switch mode {
	case busSenderModeUpdate:
		return runBusBGPSenderUpdate(ctx, args[1:])
	case busSenderModeUpdateAndRefresh:
		return runBusBGPSenderUpdateAndRefresh(ctx, args[1:])
	default:
		return fmt.Errorf("unknown bus-bgp-sender mode %q", mode)
	}
}

func runBusBGPSenderUpdate(ctx context.Context, args []string) error {
	if len(args) != 5 {
		return errors.New("usage: plugin/bus-bgp-sender update <go-file> <peer-selector> <prefix> <expect> <done-file>")
	}
	goFile, peerSel, prefix, expectRaw, doneFile := args[0], args[1], args[2], args[3], args[4]
	wantUpdate, err := parseBusSendExpect(expectRaw)
	if err != nil {
		return fmt.Errorf("bus-bgp-sender update expectation: %w", err)
	}

	reg := &sdk.Registration{
		Sends: []sdk.MessageSendDecl{
			{Message: busMsgBGPUpdate},
		},
	}

	return busRunScenarioPlugin(ctx, "sender", reg, nil, func(ctx context.Context, p *sdk.Plugin) error {
		epName := busCurrentEndpointName("sender")
		epPath := busEndpointPath(epName)
		if err := busPollFile(ctx, goFile); err != nil {
			return fmt.Errorf("wait for %s: %w", goFile, err)
		}
		announced, _, sendErr := p.UpdateRoute(ctx, peerSel, busUpdateRouteCommand(prefix))
		if err := checkBusUpdateExpect(wantUpdate, announced, sendErr, epPath, epName); err != nil {
			return err
		}
		if err := busWriteAtomicFile(doneFile, "done\n"); err != nil {
			return err
		}
		var tb textbuf.Buffer
		return tb.Str("OK: bus-bgp-sender update ").Str(prefix).Str(" -> ").Str(peerSel).Str(" (").Str(expectRaw).Str(")\n").StdErr()
	})
}

func runBusBGPSenderUpdateAndRefresh(ctx context.Context, args []string) error {
	if len(args) != 6 {
		return errors.New("usage: plugin/bus-bgp-sender update-and-refresh <go-file> <peer-selector> <prefix> <update-expect> <refresh-expect> <done-file>")
	}
	goFile, peerSel, prefix := args[0], args[1], args[2]
	updateRaw, refreshRaw, doneFile := args[3], args[4], args[5]

	wantUpdate, err := parseBusSendExpect(updateRaw)
	if err != nil {
		return fmt.Errorf("bus-bgp-sender update expectation: %w", err)
	}
	wantRefresh, err := parseBusSendExpect(refreshRaw)
	if err != nil {
		return fmt.Errorf("bus-bgp-sender refresh expectation: %w", err)
	}

	reg := &sdk.Registration{
		Sends: []sdk.MessageSendDecl{
			{Message: busMsgBGPUpdate},
			{Message: busMsgBGPRefresh},
		},
	}

	return busRunScenarioPlugin(ctx, "sender", reg, nil, func(ctx context.Context, p *sdk.Plugin) error {
		epName := busCurrentEndpointName("sender")
		epPath := busEndpointPath(epName)
		if err := busPollFile(ctx, goFile); err != nil {
			return fmt.Errorf("wait for %s: %w", goFile, err)
		}

		announced, _, updateErr := p.UpdateRoute(ctx, peerSel, busUpdateRouteCommand(prefix))
		if err := checkBusUpdateExpect(wantUpdate, announced, updateErr, epPath, epName); err != nil {
			return err
		}

		_, refreshErr := p.EmitEvent(ctx, "bgp", "refresh", "sent", "", `{"family":"ipv4/unicast"}`)
		if err := checkBusRefreshExpect(wantRefresh, refreshErr, epPath, epName); err != nil {
			return err
		}

		if err := busWriteAtomicFile(doneFile, "done\n"); err != nil {
			return err
		}
		var tb textbuf.Buffer
		return tb.Str("OK: bus-bgp-sender update(").Str(updateRaw).Str(") refresh(").Str(refreshRaw).Str(") verified\n").StdErr()
	})
}

// busAcmeAgentDriver implements the external endpoint used by S11
// (bus-central-limit.ci). It declares Sends: [bgp:update] and Receives:
// [bgp:state, bgp:update(direction=received)], records every received event to
// logFile, and after goFile appears calls UpdateRoute and verifies the
// expected allow/refusal outcome before writing doneFile.
// Usage: plugin/bus-acme-agent <log-file> <go-file> <peer-selector> <prefix> <expect> <done-file>.
func busAcmeAgentDriver(ctx context.Context, args []string) error {
	if len(args) != 6 {
		return errors.New("usage: plugin/bus-acme-agent <log-file> <go-file> <peer-selector> <prefix> <expect> <done-file>")
	}
	logFile, goFile, peerSel := args[0], args[1], args[2]
	prefix, expectRaw, doneFile := args[3], args[4], args[5]

	wantUpdate, err := parseBusSendExpect(expectRaw)
	if err != nil {
		return fmt.Errorf("bus-acme-agent expectation: %w", err)
	}

	reg := &sdk.Registration{
		Sends: []sdk.MessageSendDecl{
			{Message: busMsgBGPUpdate},
		},
		Receives: []sdk.MessageReceiveDecl{
			{Message: busMsgBGPState},
			busBGPReceivedUpdateDecl(),
		},
	}

	recorder := &busEventLogRecorder{outFile: logFile}
	return busRunScenarioPlugin(ctx, "acme", reg, func(p *sdk.Plugin) error {
		p.OnEvent(recorder.appendEvent)
		return nil
	}, func(ctx context.Context, p *sdk.Plugin) error {
		epName := busCurrentEndpointName("acme")
		epPath := busEndpointPath(epName)
		if err := busPollFile(ctx, goFile); err != nil {
			return fmt.Errorf("wait for %s: %w", goFile, err)
		}
		announced, _, sendErr := p.UpdateRoute(ctx, peerSel, busUpdateRouteCommand(prefix))
		if err := checkBusUpdateExpect(wantUpdate, announced, sendErr, epPath, epName); err != nil {
			return err
		}
		if err := busWriteAtomicFile(doneFile, "done\n"); err != nil {
			return err
		}
		var tb textbuf.Buffer
		return tb.Str("OK: bus-acme-agent ").Str(epPath).Str(" update(").Str(expectRaw).Str(") verified\n").StdErr()
	})
}

// busDetectorDriver implements the concurrent external detector endpoint used
// by S13 (bus-no-arbitration.ci). It declares Receives: [bgp:update
// (direction=received)] and Sends: [cli:show bgp peer], logs every received
// event to logFile, and once an event contains needle dispatches
// `show bgp peer list` and writes doneFile.
// Usage: plugin/bus-detector <log-file> <needle> <done-file>.
func busDetectorDriver(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return errors.New("usage: plugin/bus-detector <log-file> <needle> <done-file>")
	}
	logFile, needle, doneFile := args[0], args[1], args[2]

	reg := &sdk.Registration{
		Sends: []sdk.MessageSendDecl{
			{Message: busMsgShowBGPPeer},
		},
		Receives: []sdk.MessageReceiveDecl{
			busBGPReceivedUpdateDecl(),
		},
	}

	recorder := &busEventLogRecorder{outFile: logFile}
	matchedCh := make(chan struct{})
	var matchOnce sync.Once

	return busRunScenarioPlugin(ctx, "detector", reg, func(p *sdk.Plugin) error {
		p.OnEvent(func(event string) error {
			if err := recorder.appendEvent(event); err != nil {
				return err
			}
			if strings.Contains(event, needle) {
				matchOnce.Do(func() { close(matchedCh) })
			}
			return nil
		})
		return nil
	}, func(ctx context.Context, p *sdk.Plugin) error {
		select {
		case <-matchedCh:
		case <-ctx.Done():
			return ctx.Err()
		}
		if err := busExpectCLISucceeds(ctx, p, busCmdShowBGPPeerList); err != nil {
			return err
		}
		if err := busWriteAtomicFile(doneFile, "done\n"); err != nil {
			return err
		}
		var tb textbuf.Buffer
		return tb.Str("OK: detector recorded ").Str(needle).Str(" and dispatched show bgp peer list\n").StdErr()
	})
}

// busStateSpoofProbeDriver implements the foreign external state-write probe
// used by S15 (bus-aliased-forked-state-doctor.ci). Even when configured under
// a compiled plugin's name, its StatePut attempt to stateKey MUST be refused
// because foreign external endpoints hold no compiled state grants.
// Usage: plugin/bus-state-spoof-probe <state-key> <done-file>.
func busStateSpoofProbeDriver(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("usage: plugin/bus-state-spoof-probe <state-key> <done-file>")
	}
	stateKey, doneFile := args[0], args[1]

	reg := &sdk.Registration{}
	return busRunScenarioPlugin(ctx, "spoof", reg, nil, func(ctx context.Context, p *sdk.Plugin) error {
		err := p.StatePut(ctx, stateKey, []byte(`{"spoof":true}`))
		if err == nil {
			return fmt.Errorf("foreign external StatePut(%q) unexpectedly succeeded", stateKey)
		}
		refusal := err.Error()
		if baseErr := busCheckRefusalBase("state-put "+stateKey, refusal); baseErr != nil {
			return baseErr
		}
		if !strings.Contains(refusal, busStateNoGrantMarker) {
			return fmt.Errorf("state-put refusal omitted %q: %s", busStateNoGrantMarker, refusal)
		}
		if !strings.Contains(refusal, stateKey) {
			return fmt.Errorf("state-put refusal omitted state key %q: %s", stateKey, refusal)
		}
		if err := busWriteAtomicFile(doneFile, "refused\n"); err != nil {
			return err
		}
		var tb textbuf.Buffer
		return tb.Str("OK: foreign external state write to ").Str(stateKey).Str(" was refused\n").StdErr()
	})
}

// busTagSenderDriver implements the external tagged-event sender used by S41
// (bus-tag-filter.ci). It declares Sends: [bgp:route-change] with closed tag
// source=[bgp, static], then emits a source=static event followed by a
// source=bgp event before writing doneFile.
// Usage: plugin/bus-tag-sender <done-file>.
func busTagSenderDriver(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: plugin/bus-tag-sender <done-file>")
	}
	doneFile := args[0]

	reg := &sdk.Registration{
		Sends: []sdk.MessageSendDecl{{
			Message: busMsgBGPRouteChange,
			Tags: []rpc.MessageTagDecl{{
				Name:   busTagSource,
				Values: []string{busTagSourceBGP, busTagSourceStatic},
			}},
		}},
	}

	return busRunScenarioPlugin(ctx, "rc-sender", reg, nil, func(ctx context.Context, p *sdk.Plugin) error {
		deliveredStatic, err := p.EmitEvent(ctx, "bgp", "route-change", "", "", `{"source":"static","detail":"from-static"}`)
		if err != nil {
			return fmt.Errorf("emit static route-change: %w", err)
		}
		if deliveredStatic < 1 {
			return fmt.Errorf("emit static route-change delivered=%d, want >= 1", deliveredStatic)
		}
		deliveredBGP, err := p.EmitEvent(ctx, "bgp", "route-change", "", "", `{"source":"bgp","detail":"from-bgp"}`)
		if err != nil {
			return fmt.Errorf("emit bgp route-change: %w", err)
		}
		if deliveredBGP < 1 {
			return fmt.Errorf("emit bgp route-change delivered=%d, want >= 1", deliveredBGP)
		}
		if err := busWriteAtomicFile(doneFile, "done\n"); err != nil {
			return err
		}
		var tb textbuf.Buffer
		return tb.Str("OK: emitted static and bgp route-change events\n").StdErr()
	})
}
