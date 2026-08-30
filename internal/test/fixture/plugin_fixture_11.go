package fixture

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/command-partial-fault", commandPartialFaultDriver)
	Register("plugin/eof-no-spin", eofNoSpin)
	Register("plugin/fast", fastRoutes)
	Register("plugin/metrics-owned", metricsOwned)
	Register("plugin/metrics-registered", metricsRegistered)
	Register("plugin/nexthop", nexthopRoutes)
	Register("plugin/notification", notificationRoute)
	Register("plugin/owned-command-streams", ownedCommandStreamsDriver)
	Register("plugin/reads-engine-answer", readsEngineAnswerDriver)
	Register("plugin/reconnect", reconnectRoute)
	Register("plugin/refresh", refreshRoute)
	Register("plugin/registration", registrationObserver)
	Register("plugin/watchdog", watchdogCommands)
	Register("plugin/policy-chain-plain-names", policyChainNames)
	Register("plugin/policy-list-show", routeCountAtLeastOne("test-prefix-accept", "OK: daemon running with policy YANG, %d route(s) accepted"))
	Register("plugin/policy-routes-show", policyRoutesShow)
	Register("plugin/policy-test-as4path-suppress", policyAS4Suppress)
	Register("plugin/policy-test-configured-export", policyConfiguredExport)
	Register("plugin/policy-test-configured-import", policyConfiguredImport)
	Register("plugin/policy-test-errors", policyErrors)
	Register("plugin/policy-test-reject-bad-hex", policyBadHex)
	Register("plugin/policy-test-remove-private-as", policyRemovePrivateAS)
	Register("plugin/prefix-count-installed-cross-family", prefixCountExact(2, false, "cross-family"))
	Register("plugin/prefix-count-installed-reannounce", prefixCountExact(2, true, "reannounce"))
	Register("plugin/prefix-count-installed", prefixCountBounded(2, "installed"))
	Register("plugin/prefix-count-offered", prefixCountBounded(2, "offered"))
	Register("plugin/prefix-filter-accept", routeCountAtLeastOne("test-prefix-accept", "OK: %d matching route(s) accepted by prefix-list"))
}

// runPlugin11 runs a scenario after startup and then remains connected until
// the daemon sends its shutdown notification. Route-producing fixtures use
// this path because their peer, rather than the plugin, decides when the test
// has observed every expected wire message.
func runPlugin11(ctx context.Context, name string, registration sdk.Registration, setup func(*sdk.Plugin), scenario ObserverScenario) error {
	plugin, err := newObserver(name)
	if err != nil {
		return fmt.Errorf("connect plugin %s: %w", name, err)
	}
	defer plugin.Close() //nolint:errcheck // fixture teardown, so a close failure changes no assertion
	if setup != nil {
		setup(plugin)
	}
	result := make(chan error, 1)
	plugin.OnAllPluginsReady(func() error {
		go func() {
			scenarioErr := invokeScenario(ctx, plugin, scenario)
			result <- scenarioErr
			if scenarioErr != nil {
				_ = plugin.Close()
			}
		}()
		return nil
	})
	runErr := plugin.Run(ctx, registration)
	select {
	case scenarioErr := <-result:
		return errors.Join(scenarioErr, runErr)
	default:
		return runErr
	}
}

func dispatchValue(ctx context.Context, plugin *sdk.Plugin, command string) (string, any, error) {
	status, raw, err := plugin.DispatchCommand(ctx, command)
	if err != nil {
		return status, nil, err
	}
	if len(raw) == 0 {
		return status, nil, nil
	}
	for range 4 {
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			return status, nil, fmt.Errorf("decode %q: %w", command, err)
		}
		text, encoded := value.(string)
		if !encoded || !json.Valid([]byte(text)) {
			return status, value, nil
		}
		raw = json.RawMessage(text)
	}
	return status, nil, fmt.Errorf("decode %q: result remained JSON text after 4 layers", command)
}

func dispatchMap11(ctx context.Context, plugin *sdk.Plugin, command string) (string, map[string]any, error) {
	status, value, err := dispatchValue(ctx, plugin, command)
	if err != nil {
		return status, nil, err
	}
	m, ok := value.(map[string]any)
	if !ok {
		return status, nil, fmt.Errorf("%q returned %T, want object", command, value)
	}
	return status, m, nil
}

func quiesce11(ctx context.Context, plugin *sdk.Plugin) error {
	status, _, err := plugin.DispatchCommand(ctx, "request quiesce")
	if err != nil {
		return err
	}
	if status != statusDone {
		return fmt.Errorf("request quiesce status=%s", status)
	}
	return nil
}

func update11(ctx context.Context, plugin *sdk.Plugin, command string) error {
	_, _, err := plugin.UpdateRoute(ctx, "*", command)
	return err
}

func peerDetailReady(ctx context.Context, plugin *sdk.Plugin, attempts int, field string, predicate func(any) bool) bool {
	return Poll(ctx, attempts, 250*time.Millisecond, func() bool {
		status, data, err := dispatchMap11(ctx, plugin, "show bgp peer * detail")
		if err != nil || status != statusDone || data == nil {
			return false
		}
		peers, _ := data["peers"].(map[string]any)
		for _, raw := range peers {
			row, _ := raw.(map[string]any)
			if predicate(row[field]) {
				return true
			}
		}
		return false
	})
}

func fastRoutes(ctx context.Context, _ []string) error {
	commands := []string{
		"update text nhop 101.1.101.1 nlri ipv4/unicast add 1.1.0.0/25",
		"update text nlri ipv4/unicast del 2.2.0.0/25",
		"update text nhop 101.1.101.1 nlri ipv4/unicast add 2.2.0.0/24",
		"update text nhop 1.101.1.101 nlri ipv4/unicast add 0.0.0.0/0",
	}
	return runPlugin11(ctx, "announce-routes", sdk.Registration{}, nil, func(ctx context.Context, p *sdk.Plugin) error {
		for index, command := range commands {
			if err := update11(ctx, p, command); err != nil {
				return err
			}
			if index+1 < len(commands) {
				_ = quiesce11(ctx, p)
			}
		}
		return nil
	})
}

func notificationRoute(ctx context.Context, _ []string) error {
	return runPlugin11(ctx, "announce-routes", sdk.Registration{}, nil, func(ctx context.Context, p *sdk.Plugin) error {
		return update11(ctx, p, "update text nhop 5.6.7.8 nlri ipv4/unicast add 1.2.3.4/32")
	})
}

func reconnectRoute(ctx context.Context, _ []string) error {
	return runPlugin11(ctx, "announce-routes", sdk.Registration{}, nil, func(ctx context.Context, p *sdk.Plugin) error {
		if err := update11(ctx, p, "update text nhop 1.1.1.1 nlri ipv4/unicast add 1.1.0.0/16"); err != nil {
			return err
		}
		return quiesce11(ctx, p)
	})
}

func nexthopRoutes(ctx context.Context, _ []string) error {
	commands := []string{
		"update text origin igp local-preference 500 nhop 2001::1 nlri ipv6/unicast add 2605::2/128",
		"update text origin igp local-preference 500 nhop 2001::2 nlri ipv6/unicast add 2605::2/128",
		"update text origin igp local-preference 500 nhop 2001::1 nlri ipv6/unicast add 2605::2/128",
	}
	return runPlugin11(ctx, "announce-routes", sdk.Registration{}, nil, func(ctx context.Context, p *sdk.Plugin) error {
		if err := update11(ctx, p, commands[0]); err != nil {
			return err
		}
		if !peerDetailReady(ctx, p, 24, "eor-sent", func(v any) bool { n, _ := v.(float64); return n >= 1 }) {
			return errors.New("initial-sync end-of-rib never reached the wire")
		}
		if err := quiesce11(ctx, p); err != nil {
			return err
		}
		for _, command := range commands[1:] {
			if err := update11(ctx, p, command); err != nil {
				return err
			}
			if err := quiesce11(ctx, p); err != nil {
				return err
			}
		}
		return nil
	})
}

func refreshRoute(ctx context.Context, _ []string) error {
	stateUp := make(chan struct{}, 1)
	setup := func(p *sdk.Plugin) {
		p.SetStartupSubscriptions([]string{eventState}, nil, "")
		p.OnEvent(func(event string) error {
			var doc map[string]any
			if json.Unmarshal([]byte(event), &doc) != nil {
				return nil //nolint:nilerr // a malformed event is skipped, and failing the handler would end the session
			}
			bgp, _ := doc["bgp"].(map[string]any)
			message, _ := bgp["message"].(map[string]any)
			if message["type"] == eventState && bgp["state"] == "up" {
				select {
				case stateUp <- struct{}{}:
				default:
				}
			}
			return nil
		})
	}
	return runPlugin11(ctx, "announce-routes", sdk.Registration{}, setup, func(ctx context.Context, p *sdk.Plugin) error {
		select {
		case <-stateUp:
		case <-time.After(5 * time.Second):
			return errors.New("peer state=up event not received")
		case <-ctx.Done():
			return ctx.Err()
		}
		return update11(ctx, p, "update text origin igp local-preference 100 nhop 10.0.0.1 nlri ipv4/unicast add 192.168.1.0/24")
	})
}

func registrationObserver(ctx context.Context, _ []string) error {
	reg := sdk.Registration{
		Families: []sdk.FamilyDecl{{Name: familyIPv4Unicast, Mode: modeBoth, AFI: 1, SAFI: 1}},
		Commands: []sdk.CommandDecl{{Name: "show test-plugin registration"}},
	}
	return runPlugin11(ctx, "test-plugin", reg, nil, func(ctx context.Context, p *sdk.Plugin) error {
		status, data, err := dispatchMap11(ctx, p, "system command list")
		if err != nil || status != statusDone {
			return fmt.Errorf("system command list status=%s: %w", status, err)
		}
		commands, _ := data["commands"].([]any)
		found := false
		for _, raw := range commands {
			row, _ := raw.(map[string]any)
			found = found || row["value"] == "show test-plugin registration"
		}
		if !found {
			return errors.New("declared command missing from the engine command list")
		}
		if !peerDetailReady(ctx, p, 60, "state", func(v any) bool { return v == stateEstablished }) {
			return errors.New("peer never reached established")
		}
		if err := update11(ctx, p, "update text nlri ipv4/unicast eor"); err != nil {
			return err
		}
		return quiesce11(ctx, p)
	})
}

func watchdogCommands(ctx context.Context, _ []string) error {
	return runPlugin11(ctx, "service-watchdog", sdk.Registration{}, nil, func(ctx context.Context, p *sdk.Plugin) error {
		if !peerDetailReady(ctx, p, 60, "eor-sent", func(value any) bool { return number09(value) >= 1 }) {
			return errors.New("watchdog: initial End-of-RIB was not sent")
		}
		if !peerDetailReady(ctx, p, 60, "updates-sent", func(value any) bool { return number09(value) >= 2 }) {
			return errors.New("watchdog: configured startup routes were not sent")
		}
		if err := quiesce11(ctx, p); err != nil {
			return err
		}
		for range 3 {
			for _, command := range []string{"request bgp watchdog announce dnsr", cmdWatchdogWithdrawDNSR} {
				status, _, err := p.DispatchCommand(ctx, command)
				if err != nil || status != statusDone {
					return fmt.Errorf("%s status=%s: %w", command, status, err)
				}
				if err := quiesce11(ctx, p); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func metricsOwned(ctx context.Context, _ []string) error {
	return Observe(ctx, "metrics-list-test", sdk.Registration{}, func(ctx context.Context, p *sdk.Plugin) error {
		status, data, err := dispatchMap11(ctx, p, "show metrics list")
		if err != nil || status != statusDone {
			return fmt.Errorf("show metrics list status=%s: %w", status, err)
		}
		names, ok := data["names"].([]any)
		if !ok {
			return fmt.Errorf("names not a list: %v", data)
		}
		count, _ := data["count"].(float64)
		if int(count) != len(names) {
			return fmt.Errorf("count mismatch: %v != %d", count, len(names))
		}
		fmt.Fprintf(os.Stderr, "OK: metrics list returned %d names\n", len(names))
		for _, prefix := range []string{"ze_rib_", "ze_gr_"} {
			found := false
			for _, name := range names {
				text, _ := name.(string)
				found = found || strings.HasPrefix(text, prefix)
			}
			if !found {
				return fmt.Errorf("no %s* metrics in list", prefix)
			}
			fmt.Fprintf(os.Stderr, "OK: %s* metrics present\n", prefix)
		}
		return nil
	})
}

func metricSeriesCount(data map[string]any) int {
	series, _ := data["series"].([]any)
	return len(series)
}

func metricsRegistered(ctx context.Context, _ []string) error {
	return Observe(ctx, "plugin-metrics", sdk.Registration{}, func(ctx context.Context, p *sdk.Plugin) error {
		for _, name := range []string{"ze_role_route_rejects_total", "ze_role_route_suppressions_total"} {
			var count int
			ok := Poll(ctx, 40, 250*time.Millisecond, func() bool {
				status, data, err := dispatchMap11(ctx, p, "show metrics name "+name)
				count = metricSeriesCount(data)
				return err == nil && status == statusDone && count > 0
			})
			if !ok {
				return fmt.Errorf("%s is absent from the metrics registry", name)
			}
			fmt.Fprintf(os.Stderr, "OK: %s has %d series\n", name, count)
		}
		status, data, err := dispatchMap11(ctx, p, "show metrics name ze_peers_configured")
		if err != nil || status != statusDone || metricSeriesCount(data) == 0 {
			return errors.New("reactor metrics missing too; telemetry is not enabled in this test")
		}
		return metricsLabelFilter11(ctx, p, "ze_peers_configured")
	})
}

// metricsLabelFilter11 checks the two-token label filter of `show metrics name`
// against a running daemon: a complete group filters, and a group that stops
// short of its value is refused.
//
// The filter names a label no series carries, so it answers zero series. A
// filter that never reached the handler answers every series of the metric,
// which is what the retired `label=value` packing did to any token it could not
// split (plan/spec-generated-command-usage.md).
func metricsLabelFilter11(ctx context.Context, p *sdk.Plugin, metric string) error {
	status, data, err := dispatchMap11(ctx, p, "show metrics name "+metric+" label nosuchlabel nosuchvalue")
	if err != nil || status != statusDone {
		return fmt.Errorf("label filter on %s: status=%s: %w", metric, status, err)
	}
	if count := metricSeriesCount(data); count != 0 {
		return fmt.Errorf("label filter on %s matched %d series, want 0", metric, count)
	}
	fmt.Fprintf(os.Stderr, "OK: a label filter that matches nothing answers no series\n")

	// A handler error arrives as a Go error beside the status, never in the
	// data (dispatchCommandResult, pkg/plugin/sdk/sdk_engine.go).
	status, _, err = p.DispatchCommand(ctx, "show metrics name "+metric+" label nosuchlabel")
	if err == nil {
		return fmt.Errorf("label with no value on %s: status=%s, want an error", metric, status)
	}
	if status != statusError {
		return fmt.Errorf("label with no value on %s: status=%s: %w", metric, status, err)
	}
	if !strings.Contains(err.Error(), "label needs a key and a value") {
		return fmt.Errorf("label with no value on %s said: %w", metric, err)
	}
	fmt.Fprintln(os.Stderr, "OK: a label with no value is refused")
	return nil
}

func routeCount(ctx context.Context, p *sdk.Plugin, command string) (int, bool) {
	status, data, err := dispatchMap11(ctx, p, command)
	if err != nil || status != statusDone || data == nil {
		return 0, false
	}
	value, ok := data["total-routes"].(float64)
	if !ok {
		value, ok = data["count"].(float64)
	}
	return int(value), ok
}
func routeCountAtLeastOne(name, output string) Driver {
	return func(ctx context.Context, _ []string) error {
		return Observe(ctx, name, sdk.Registration{}, func(ctx context.Context, p *sdk.Plugin) error {
			var count int
			if !Poll(ctx, 40, 250*time.Millisecond, func() bool {
				var ok bool
				count, ok = routeCount(ctx, p, "show bgp adj-rib-in status")
				return ok && count >= 1
			}) {
				return fmt.Errorf("expected >=1 route, got %d", count)
			}
			if !peerDetailReady(ctx, p, 40, "connections-established", func(value any) bool { return number09(value) >= 1 }) {
				return errors.New("route was accepted without a recorded established session")
			}
			fmt.Fprintf(os.Stderr, output+"\n", count)
			return nil
		})
	}
}

func prefixCountExact(expected int, control bool, label string) Driver {
	return func(ctx context.Context, _ []string) error {
		return Observe(ctx, "prefix-count-"+label, sdk.Registration{}, func(ctx context.Context, p *sdk.Plugin) error {
			if control {
				var count int
				if !Poll(ctx, 60, 250*time.Millisecond, func() bool { count, _ = routeCount(ctx, p, "show bgp rib received count"); return count >= 1 }) {
					return errors.New("no route reached the RIB at all")
				}
				fmt.Fprintf(os.Stderr, "OK control: the RIB answers, %d routes received\n", count)
			}
			var count int
			attempts := 60
			if label == "reannounce" {
				attempts = 40
			}
			Poll(ctx, attempts, 250*time.Millisecond, func() bool { count, _ = routeCount(ctx, p, "show bgp rib received count"); return count >= expected })
			if count != expected {
				return fmt.Errorf("%s: %d routes received, wanted exactly %d", label, count, expected)
			}
			if label == "cross-family" {
				fmt.Fprintf(os.Stderr, "OK cross-family: %d routes installed after the refused message\n", count)
			} else {
				fmt.Fprintf(os.Stderr, "OK reannounce: %d routes installed, the re-announced prefix took one slot\n", count)
			}
			return nil
		})
	}
}

func prefixCountBounded(maximum int, label string) Driver {
	return func(ctx context.Context, _ []string) error {
		return Observe(ctx, "prefix-count-"+label, sdk.Registration{}, func(ctx context.Context, p *sdk.Plugin) error {
			var count int
			if !Poll(ctx, 60, 250*time.Millisecond, func() bool { count, _ = routeCount(ctx, p, "show bgp rib received count"); return count >= 1 }) {
				return errors.New("the control route never reached the RIB")
			}
			fmt.Fprintf(os.Stderr, "OK control: an in-limit route is installed, %d routes received\n", count)
			var readable bool
			for range 12 {
				count, readable = routeCount(ctx, p, "show bgp rib received count")
				if count > maximum {
					return fmt.Errorf("RIB holds %d routes, maximum is %d", count, maximum)
				}
				time.Sleep(250 * time.Millisecond)
			}
			if !readable {
				return errors.New("show bgp rib received count stopped answering")
			}
			fmt.Fprintf(os.Stderr, "OK %s: %d routes installed, never past the maximum of %d\n", label, count, maximum)
			return nil
		})
	}
}

const policyUpdate = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003E02000000234001010040020E02030000FBF00000FC000000FBF14003040101010140050400000064180A0000"

func policyObserve(ctx context.Context, name string, fn func(context.Context, *sdk.Plugin) error) error {
	return Observe(ctx, name, sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		if !peerDetailReady(ctx, plugin, 40, "state", func(value any) bool { return value == stateEstablished }) {
			return errors.New("configured policy peer never reached established")
		}
		return fn(ctx, plugin)
	})
}

func policyChainNames(ctx context.Context, _ []string) error {
	return policyObserve(ctx, "policy-chain-names", func(ctx context.Context, p *sdk.Plugin) error {
		status, data, err := dispatchMap11(ctx, p, "show policy chain peer test-peer export")
		if err != nil || status != statusDone {
			return fmt.Errorf("status=%s: %w", status, err)
		}
		chains, _ := data["chains"].([]any)
		if len(chains) == 0 {
			return fmt.Errorf("no chains in output: %v", data)
		}
		chain, _ := chains[0].(map[string]any)
		exports, _ := chain["export"].([]any)
		if len(exports) == 0 {
			return fmt.Errorf("no export refs: %v", chain)
		}
		ref, _ := exports[0].(map[string]any)
		fmt.Printf("OK: export ref name=%q canonical=%q\n", ref["name"], ref["canonical"])
		if ref["name"] != "STRIP" || ref["canonical"] != "bgp-filter-remove-private-as:STRIP" {
			return fmt.Errorf("unexpected export ref: %v", ref)
		}
		fmt.Println("OK: show policy chain exposes plain name and canonical ref")
		return nil
	})
}

func policyConfiguredExport(ctx context.Context, _ []string) error {
	return policyDirection(ctx, "policy-test-export", "export")
}
func policyConfiguredImport(ctx context.Context, _ []string) error {
	return policyDirection(ctx, "policy-test-import", "import")
}
func policyDirection(ctx context.Context, name, direction string) error {
	return policyObserve(ctx, name, func(ctx context.Context, p *sdk.Plugin) error {
		status, data, err := dispatchMap11(ctx, p, "show policy test peer receiver-peer "+direction+" update "+policyUpdate)
		if err != nil || status != statusDone {
			return fmt.Errorf("dry-run status=%s: %w", status, err)
		}
		if data["direction"] != direction {
			return fmt.Errorf("direction=%v, want %s", data["direction"], direction)
		}
		action, _ := data["action"].(string)
		if action != actionAccept && action != "modify" && action != actionReject {
			return fmt.Errorf("unexpected action: %s", action)
		}
		fmt.Printf("OK: show policy test %s returned action=%s direction=%s\n", direction, action, direction)
		if direction == directionExport {
			trace, _ := data["trace"].([]any)
			fmt.Printf("OK: trace entries: %d\n", len(trace))
		}
		return nil
	})
}

// policyErrors proves the two handler-side error paths of `show policy test
// peer`, and with them that a direction token reaches the handler as a
// DIRECTION rather than as a filter name.
//
// Each case names the message it expects, because the message is what says who
// answered. Argument validation runs before the handler, so a case that
// accepted any error would pass on a refusal from the dispatcher and prove
// nothing about the handler. The second command is the form the command's own
// documentation states, and the dispatcher refuses it outright when the model
// declares no direction leaf: `export` is then offered to the `filter` leaf,
// whose own keyword has already taken a value, and the only definition left is
// the source-asn4 enum (spec-generated-command-usage).
func policyErrors(ctx context.Context, _ []string) error {
	return policyObserve(ctx, "policy-test-errors", func(ctx context.Context, p *sdk.Plugin) error {
		cases := []struct {
			command string
			label   string
			want    string
		}{
			{"show policy test peer 192.0.2.99 export update " + policyUpdate, "peer not found", "peer not found"},
			{"show policy test peer test-peer export filter NOPE update " + policyUpdate, "unknown filter", "filter not found in peer chain"},
		}
		for _, test := range cases {
			status, _, dispatchErr := dispatchValue(ctx, p, test.command)
			if dispatchErr == nil {
				return fmt.Errorf("%s was accepted, with status %s", test.label, status)
			}
			// A refusal from argument validation answers no status at all, so
			// the status is what says who refused before the message does.
			if status != statusError {
				return fmt.Errorf("%s was refused before the handler ran, with status %q: %w", test.label, status, dispatchErr)
			}
			if !strings.Contains(dispatchErr.Error(), test.want) {
				return fmt.Errorf("%s does not carry %q, so the handler did not answer: %w", test.label, test.want, dispatchErr)
			}
			fmt.Printf("OK: %s rejected by the handler: %v\n", test.label, dispatchErr)
		}

		// The model must REQUIRE the direction, because the handler does:
		// parsePolicyTestArgs answers errMissingDirection without one. Declared
		// mandatory, the leaf makes the dispatcher refuse the call before the
		// handler runs, so the message this asks for is the dispatcher's. An
		// optional leaf would leave the same command reaching the handler.
		_, _, noDirectionErr := dispatchValue(ctx, p, "show policy test peer test-peer update "+policyUpdate)
		if noDirectionErr == nil {
			return errors.New("a call with no direction was accepted")
		}
		if !strings.Contains(noDirectionErr.Error(), "required argument missing: direction") {
			return fmt.Errorf("the call with no direction was not refused for the missing direction: %w", noDirectionErr)
		}
		fmt.Printf("OK: a call with no direction is refused: %v\n", noDirectionErr)

		fmt.Println("ALL PASS: policy test error paths")
		return nil
	})
}

func policyBadHex(ctx context.Context, _ []string) error {
	return policyObserve(ctx, "policy-test-bad-hex", func(ctx context.Context, p *sdk.Plugin) error {
		cases := []struct {
			update string
			label  string
		}{
			{"ZZZZ", "bad hex"},
			{"FFFF", "short message"},
			{"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001301", "non-UPDATE"},
		}
		for _, test := range cases {
			status, _, dispatchErr := dispatchValue(ctx, p, "show policy test peer test-peer export update "+test.update)
			if status != statusError {
				return fmt.Errorf("%s was not rejected: %s", test.label, status)
			}
			fmt.Printf("OK: %s rejected: %v\n", test.label, dispatchErr)
		}
		fmt.Println("ALL PASS: bad hex rejection tests")
		return nil
	})
}

func policyRemovePrivateAS(ctx context.Context, _ []string) error {
	return policyObserve(ctx, "policy-test-rpa", func(ctx context.Context, p *sdk.Plugin) error {
		status, data, err := dispatchMap11(ctx, p, "show policy test peer test-peer export filter STRIP update "+policyUpdate)
		if err != nil || status != statusDone {
			return fmt.Errorf("dry-run status=%s: %w", status, err)
		}
		if data["action"] != "modify" {
			return fmt.Errorf("expected action=modify, got %v", data["action"])
		}
		changed, _ := data["changed-attrs"].([]any)
		found := false
		for _, attr := range changed {
			found = found || attr == fieldASPath
		}
		if !found {
			return fmt.Errorf("as-path not in changed-attrs: %v", changed)
		}
		fmt.Println("OK: remove-private-as dry-run modified as-path (AC-9)")
		trace, _ := data["trace"].([]any)
		fmt.Printf("OK: trace entries: %d\n", len(trace))
		if len(trace) != 0 {
			first, _ := trace[0].(map[string]any)
			fmt.Printf("OK: first filter: %v action=%v\n", first["filter"], first["action"])
		}
		return nil
	})
}

func policyAS4Suppress(ctx context.Context, _ []string) error {
	const update = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003F0200000024400101004002060202FBF45BA040030401010101C011060201FA56EA0040050400000064180A0000"
	return policyObserve(ctx, "policy-test-as4", func(ctx context.Context, p *sdk.Plugin) error {
		command := "show policy test peer test-peer export filter STRIP update " + update + " source-asn4 false"
		status, data, err := dispatchMap11(ctx, p, command)
		if err != nil || status != statusDone {
			return fmt.Errorf("dry-run status=%s: %w", status, err)
		}
		changes, _ := data["wire-changes"].([]any)
		fmt.Printf("OK: action=%v wire-changes=%v\n", data["action"], changes)
		for _, change := range changes {
			if change == "AS4_PATH suppressed" {
				fmt.Println("OK: AS4_PATH suppressed surfaced in dry-run wire-changes")
				return nil
			}
		}
		return fmt.Errorf("AS4_PATH suppressed not in wire-changes: %v", changes)
	})
}

func eofNoSpin(ctx context.Context, _ []string) error {
	pluginSide, engineSide := net.Pipe()
	plugin := sdk.NewWithConn("eof-no-spin", pluginSide)
	engineConn := rpc.NewConn(engineSide, engineSide)
	engine := rpc.NewMuxConn(engineConn)
	defer plugin.Close() //nolint:errcheck // fixture teardown, so a close failure changes no assertion
	defer engine.Close() //nolint:errcheck // fixture teardown, so a close failure changes no assertion

	runResult := make(chan error, 1)
	go func() { runResult <- plugin.Run(ctx, sdk.Registration{}) }()
	nextRequest := func() (*rpc.Request, error) {
		select {
		case request := <-engine.Requests():
			return request, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
			return nil, errors.New("timed out waiting for startup request")
		}
	}

	request, err := nextRequest()
	if err != nil || request.Method != "ze-plugin-engine:declare-registration" {
		return fmt.Errorf("stage 1: request=%v: %w", request, err)
	}
	if err := engine.SendOK(ctx, request.ID); err != nil {
		return err
	}
	if _, err := engine.CallRPC(ctx, "ze-plugin-callback:configure", struct {
		Sections []sdk.ConfigSection `json:"sections"`
	}{}); err != nil {
		return err
	}
	request, err = nextRequest()
	if err != nil || request.Method != "ze-plugin-engine:declare-capabilities" {
		return fmt.Errorf("stage 3: request=%v: %w", request, err)
	}
	if err := engine.SendOK(ctx, request.ID); err != nil {
		return err
	}
	if _, err := engine.CallRPC(ctx, "ze-plugin-callback:share-registry", struct {
		Commands []sdk.RegistryCommand `json:"commands"`
	}{}); err != nil {
		return err
	}
	request, err = nextRequest()
	if err != nil || request.Method != "ze-plugin-engine:ready" {
		return fmt.Errorf("stage 5: request=%v: %w", request, err)
	}
	if err := engine.SendOK(ctx, request.ID); err != nil {
		return err
	}

	// Drop the callback transport without sending bye. Run must return rather
	// than spinning in its event loop on the persistent EOF.
	if err := engine.Close(); err != nil {
		return err
	}
	select {
	case <-runResult:
		fmt.Fprintln(os.Stderr, "OK: read_line set _shutdown on EOF")
		return nil
	case <-time.After(time.Second):
		return errors.New("SDK event loop spun after connection EOF")
	}
}

func verifyCommandStream(_ context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: plugin/owned-command-streams FILE")
	}
	file, err := os.Open(args[0])
	if err != nil {
		return err
	}
	defer file.Close() //nolint:errcheck // fixture teardown
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 20*1024*1024)
	seen := 0
	for scanner.Scan() {
		var row map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			return fmt.Errorf("row %d: %w", seen, err)
		}
		_, hasFill := row["fill"]
		_, hasIndex := row["index"]
		if len(row) != 2 || !hasFill || !hasIndex {
			return fmt.Errorf("row %d carries unexpected keys", seen)
		}
		index, ok := row["index"].(float64)
		if !ok || int(index) != seen {
			return fmt.Errorf("rows out of walk order at %d: %v", seen, row["index"])
		}
		seen++
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "OK: %d rows reached the operator, one line each, in walk order\n", seen)
	return nil
}

func verifyEngineAnswer(_ context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("usage: plugin/reads-engine-answer FILE ROWS")
	}
	want, err := strconv.Atoi(args[1])
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return err
	}
	reading, ok := doc["engine-answer"].(map[string]any)
	if !ok {
		return fmt.Errorf("answer carries unexpected keys: %v", doc)
	}
	if reading["type"] != shapeMap || reading["key"] != fieldCommands || reading["verdict"] != statusDone {
		return fmt.Errorf("unexpected streamed answer: %v", reading)
	}
	rows, _ := reading["rows"].(float64)
	if int(rows) != want {
		return fmt.Errorf("plugin walked %d rows, operator reads %d", int(rows), want)
	}
	first, firstOK := reading["first"].(string)
	last, lastOK := reading["last"].(string)
	if !firstOK || !lastOK || first == "" || last == "" || first == last {
		return fmt.Errorf("bad first/last rows: %v", reading)
	}
	fmt.Fprintf(os.Stderr, "OK: the plugin walked %d streamed rows and acted on them\n", int(rows))
	return nil
}

func verifyPartialFault(_ context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: plugin/command-partial-fault FILE")
	}
	raw, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(raw, &document); err != nil {
		return err
	}
	if len(document) != 2 || document["rows"] == nil || document["errors"] == nil {
		return fmt.Errorf("answer carries unexpected keys: %v", document)
	}
	var rows []map[string]any
	if err := json.Unmarshal(document["rows"], &rows); err != nil {
		return err
	}
	if len(rows) != 11 {
		return fmt.Errorf("got %d applied rows, want 11", len(rows))
	}
	for position, row := range rows {
		index := position
		if position >= 6 {
			index++
		}
		got, indexOK := row["index"].(float64)
		fill, fillOK := row["fill"].(string)
		if len(row) != 2 || !indexOK || int(got) != index || !fillOK || fill == "" {
			return fmt.Errorf("bad applied row %d: %v", position, row)
		}
	}
	var faults []map[string]any
	if err := json.Unmarshal(document["errors"], &faults); err != nil {
		return err
	}
	if len(faults) != 1 {
		return fmt.Errorf("got %d rejected rows, want one", len(faults))
	}
	fault := faults[0]
	encoded, encodedOK := fault["encoded-bytes"].(float64)
	limit, limitOK := fault["limit-bytes"].(float64)
	record, recordOK := fault["record"].(float64)
	if len(fault) != 4 || !encodedOK || !limitOK || !recordOK ||
		fault["message"] != "answer record does not fit one wire message" ||
		int(record) != 7 || int(limit) != 16777216 || encoded <= limit {
		return fmt.Errorf("bad rejected row: %v", fault)
	}
	fmt.Fprintf(os.Stderr, "OK: %d rows applied and 1 rejected reached the operator together\n", len(rows))
	return nil
}

func runCaptured(ctx context.Context, env []string, stdin string, argv ...string) (int, string, string, error) {
	command := exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // the fixture chooses the program and its arguments
	command.Env = env
	command.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	code := 0
	if err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			code = exit.ExitCode()
		} else {
			return -1, stdout.String(), stderr.String(), err
		}
	}
	return code, stdout.String(), stderr.String(), nil
}

func policyRoutesShow(ctx context.Context, _ []string) error {
	if !Poll(ctx, 200, 50*time.Millisecond, func() bool {
		_, a := os.Stat("daemon.pid")
		_, b := os.Stat("daemon.ready")
		return a == nil && b == nil
	}) {
		return errors.New("daemon readiness files missing")
	}
	pidRaw, err := os.ReadFile("daemon.pid")
	if err != nil {
		return err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidRaw)))
	if err != nil {
		return err
	}
	var out string
	var last string
	ok := Poll(ctx, 50, 100*time.Millisecond, func() bool {
		code, stdout, stderr, runErr := runCaptured(ctx, os.Environ(), "", "ze", "cli", "-c", "show policy routes")
		last = stdout + stderr
		if runErr == nil && code == 0 {
			out = stdout
			return true
		}
		return false
	})
	if !ok {
		return fmt.Errorf("ze cli never became ready: %s", last)
	}
	var entries []map[string]any
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		return fmt.Errorf("invalid JSON from show policy routes: %w; output: %s", err, out)
	}
	if len(entries) == 0 || entries[0]["name"] == nil {
		return fmt.Errorf("expected named policy entry, got: %s", out)
	}
	fmt.Print(out)
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Signal(syscall.SIGTERM)
}
