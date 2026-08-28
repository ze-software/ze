package fixture

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

type p12Scenario = ObserverScenario

type p12Peer struct {
	name string
	addr string
}

func init() {
	Register("plugin/prefix-filter-chain-order", p12ObserveDriver("test-chain", sdk.Registration{}, p12PrefixChainOrder))
	Register("plugin/prefix-filter-entry-order", p12ObserveDriver("test-prefix-order", sdk.Registration{}, p12PrefixEntryOrder))
	Register("plugin/prefix-filter-modify-partial", p12ObserveDriver("test-modify-partial", sdk.Registration{}, p12WaitRoutes(2)))
	Register("plugin/prefix-filter-plain", p12ObserveDriver("test-plain", sdk.Registration{}, p12WaitRoutes(1)))
	Register("plugin/prefix-filter-reject", p12ObserveDriver("test-prefix-reject", sdk.Registration{}, p12PrefixReject))
	Register("plugin/prefix-filter-shortform", p12ObserveDriver("test-shortform", sdk.Registration{}, p12WaitRoutes(1)))
	Register("plugin/prefix-warn-only-drops-nlri", p12ObserveDriver("prefix-drop-test", sdk.Registration{}, p12WarnOnlyDropsNLRI))
	Register("plugin/prefixsid-ebgp-discard-single-walk", p12SubscribedDriver("shutdown-after-up", []string{"update"}, p12RouteServerReplay))
	Register("plugin/quiesce-barrier", p12ObserveDriver("quiesce-barrier", sdk.Registration{}, p12QuiesceBarrier))
	Register("plugin/rbac-web-config-deny", p12WebRBAC)
	Register("plugin/reactor-bus-subscribe", p12SubscribedDriver("bus-test", []string{"state"}, p12BusSubscribe))

	Register("plugin/redistribute-as112-announce", p12ObserveDriver("as112-announce-test", sdk.Registration{}, p12AS112Announce))
	Register("plugin/redistribute-as112-community", p12ObserveDriver("as112-community-test", sdk.Registration{}, p12AS112Single("request fakeas112 emit add asn 112 community nopeer")))
	Register("plugin/redistribute-as112-not-imported", p12ObserveDriver("as112-nocfg-test", sdk.Registration{}, p12RedistributeNotConfigured("fakeas112")))
	Register("plugin/redistribute-as112-origin-ebgp", p12ObserveDriver("as112-ebgp-test", sdk.Registration{}, p12AS112Single("request fakeas112 emit add asn 112")))
	Register("plugin/redistribute-as112-withdraw", p12ObserveDriver("as112-withdraw-test", sdk.Registration{}, p12AS112Withdraw))

	p12RegisterFilterFixtures()

	Register("plugin/redistribute-l2tp-announce", p12ObserveDriver("l2tp-announce-test", sdk.Registration{}, p12L2TPAnnounce))
	Register("plugin/redistribute-l2tp-multi-peer-nexthop", p12ObserveDriver("l2tp-nexthop-test", sdk.Registration{}, p12L2TPMultiPeer))
	Register("plugin/redistribute-l2tp-not-configured", p12ObserveDriver("l2tp-nocfg-test", sdk.Registration{}, p12RedistributeNotConfigured("fakel2tp")))
	Register("plugin/redistribute-l2tp-withdraw", p12ObserveDriver("l2tp-withdraw-test", sdk.Registration{}, p12L2TPWithdraw))
	Register("plugin/redistribute-late-join-configadd", p12WaitForDaemonDriver("late-join-configadd-test", p12LateJoinConfigAdd))
	Register("plugin/redistribute-late-join-configadd-trigger", p12LateJoinTrigger)
	Register("plugin/relay-withdraw-nexthop-self", p12ObserveDriver("relay-nhself", sdk.Registration{}, p12RelayWithdraw(false)))
	Register("plugin/relay-withdraw-reflector", p12ObserveDriver("relay-rr", sdk.Registration{}, p12RelayWithdraw(true)))
	Register("plugin/reload-hello-interval", p12ObserveDriver("reload-hello-interval-test", sdk.Registration{}, p12ReloadL2TP("hello-interval", 45, 90)))
	Register("plugin/reload-hello-retries", p12ObserveDriver("reload-hello-retries-test", sdk.Registration{}, p12ReloadL2TP("hello-retries", 1, 4)))
	Register("plugin/reload-hello-interval-trigger", p12ReloadTrigger)
	Register("plugin/reload-hello-retries-trigger", p12ReloadTrigger)
}

func p12ObserveDriver(name string, registration sdk.Registration, scenario p12Scenario) Driver {
	return func(ctx context.Context, _ []string) error {
		return Observe(ctx, name, registration, scenario)
	}
}

// p12SubscribedDriver is the subscription-capable counterpart to Observe. The
// SDK must install the event handler and carry subscriptions in the ready RPC
// before its event loop starts.
func p12SubscribedDriver(name string, events []string, setup func(*sdk.Plugin) p12Scenario) Driver {
	return func(ctx context.Context, _ []string) error {
		plugin, err := newObserver(name)
		if err != nil {
			return fmt.Errorf("connect observer %s: %w", name, err)
		}
		defer plugin.Close() //nolint:errcheck // the run result carries transport failures

		plugin.SetStartupSubscriptions(events, nil, "")
		scenario := setup(plugin)
		result := make(chan error, 1)
		plugin.OnAllPluginsReady(func() error {
			go func() {
				scenarioErr := invokeScenario(ctx, plugin, scenario)
				result <- scenarioErr
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_, _, _ = plugin.DispatchCommand(shutdownCtx, "request shutdown")
			}()
			return nil
		})

		runErr := plugin.Run(ctx, sdk.Registration{})
		select {
		case scenarioErr := <-result:
			if scenarioErr != nil {
				return errorsJoinP12(scenarioErr, runErr)
			}
		default:
		}
		return runErr
	}
}

// p12WaitForDaemonDriver preserves observers that deliberately remain alive
// until the daemon reload/test harness shuts them down.
func p12WaitForDaemonDriver(name string, scenario p12Scenario) Driver {
	return func(ctx context.Context, _ []string) error {
		plugin, err := newObserver(name)
		if err != nil {
			return fmt.Errorf("connect observer %s: %w", name, err)
		}
		defer plugin.Close() //nolint:errcheck

		bye := make(chan struct{})
		var once sync.Once
		plugin.OnBye(func(string) { once.Do(func() { close(bye) }) })
		result := make(chan error, 1)
		plugin.OnAllPluginsReady(func() error {
			go func() {
				if err := invokeScenario(ctx, plugin, scenario); err != nil {
					result <- err
					shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					_, _, _ = plugin.DispatchCommand(shutdownCtx, "request shutdown")
					return
				}
				select {
				case <-ctx.Done():
					result <- nil
				case <-bye:
					result <- nil
				}
			}()
			return nil
		})
		runErr := plugin.Run(ctx, sdk.Registration{})
		select {
		case scenarioErr := <-result:
			return errorsJoinP12(scenarioErr, runErr)
		default:
			return runErr
		}
	}
}

func errorsJoinP12(first, second error) error {
	if first == nil {
		return second
	}
	if second == nil {
		return first
	}
	return fmt.Errorf("%v; %w", first, second)
}

func p12DispatchObject(ctx context.Context, plugin *sdk.Plugin, command string) (string, map[string]any, error) {
	status, raw, err := plugin.DispatchCommand(ctx, command)
	if err != nil || len(raw) == 0 {
		return status, nil, err
	}
	for range 4 {
		var decoded any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return status, nil, fmt.Errorf("decode %q result: %w", command, err)
		}
		switch data := decoded.(type) {
		case map[string]any:
			return status, data, nil
		case []any:
			if len(data) == 1 {
				if object, ok := data[0].(map[string]any); ok {
					return status, object, nil
				}
			}
			return status, nil, fmt.Errorf("decode %q result: got %d rows, want one object", command, len(data))
		case string:
			if !json.Valid([]byte(data)) {
				return status, nil, fmt.Errorf("decode %q result: got text, want object", command)
			}
			raw = json.RawMessage(data)
		default:
			return status, nil, fmt.Errorf("decode %q result: got %T, want object", command, decoded)
		}
	}
	return status, nil, fmt.Errorf("decode %q result: remained JSON text after 4 layers", command)
}

func p12RequireDone(ctx context.Context, plugin *sdk.Plugin, command string) (map[string]any, error) {
	status, err := Dispatch(ctx, plugin, command, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", command, err)
	}
	if status != "done" {
		return nil, fmt.Errorf("%s: status=%s", command, status)
	}
	return nil, nil
}

func p12Number(value any) (int, bool) {
	switch number := value.(type) {
	case float64:
		integer := int(number)
		return integer, float64(integer) == number
	case json.Number:
		integer, err := number.Int64()
		if err != nil || int64(int(integer)) != integer {
			return 0, false
		}
		return int(integer), true
	case int:
		return number, true
	case int64:
		if int64(int(number)) != number {
			return 0, false
		}
		return int(number), true
	default:
		return 0, false
	}
}
func p12FindField(value any, key string) (any, bool) {
	switch data := value.(type) {
	case map[string]any:
		if field, found := data[key]; found {
			return field, true
		}
		for _, child := range data {
			if field, found := p12FindField(child, key); found {
				return field, true
			}
		}
	case []any:
		for _, child := range data {
			if field, found := p12FindField(child, key); found {
				return field, true
			}
		}
	}
	return nil, false
}

func p12NumberAtLeast(value any, minimum int) bool {
	number, ok := p12Number(value)
	return ok && number >= minimum
}

func p12PeerRows(ctx context.Context, plugin *sdk.Plugin, selector string) map[string]any {
	status, data, err := p12DispatchObject(ctx, plugin, "show bgp peer "+selector+" detail")
	if err != nil || status != "done" {
		return nil
	}
	rowsValue, _ := p12FindField(data, "peers")
	rows, _ := rowsValue.(map[string]any)
	return rows
}

func p12PeerField(ctx context.Context, plugin *sdk.Plugin, selector, address, field string) any {
	row, _ := p12PeerRows(ctx, plugin, selector)[address].(map[string]any)
	return row[field]
}

func p12PeerCounter(ctx context.Context, plugin *sdk.Plugin, selector, field string) (int, bool) {
	rows := p12PeerRows(ctx, plugin, selector)
	if len(rows) == 0 {
		return 0, false
	}
	total := 0
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			return 0, false
		}
		number, ok := p12Number(row[field])
		if !ok {
			return 0, false
		}
		total += number
	}
	return total, true
}

func p12WaitPeerCounter(ctx context.Context, plugin *sdk.Plugin, selector, field string, minimum, attempts int) bool {
	return Poll(ctx, attempts, 250*time.Millisecond, func() bool {
		total, ok := p12PeerCounter(ctx, plugin, selector, field)
		return ok && total >= minimum
	})
}

func p12WaitPeerField(ctx context.Context, plugin *sdk.Plugin, peer p12Peer, field string, predicate func(any) bool, what string) error {
	if Poll(ctx, 100, 200*time.Millisecond, func() bool {
		return predicate(p12PeerField(ctx, plugin, peer.name, peer.addr, field))
	}) {
		return nil
	}
	return fmt.Errorf("timeout waiting for %s", what)
}

func p12WaitAllPeerFields(ctx context.Context, plugin *sdk.Plugin, peers []p12Peer, field string, predicate func(p12Peer, any) bool, what string) error {
	if Poll(ctx, 100, 200*time.Millisecond, func() bool {
		for _, peer := range peers {
			if !predicate(peer, p12PeerField(ctx, plugin, peer.name, peer.addr, field)) {
				return false
			}
		}
		return true
	}) {
		return nil
	}
	return fmt.Errorf("timeout waiting for %s", what)
}

func p12RouteTotal(ctx context.Context, plugin *sdk.Plugin) (int, bool) {
	status, data, err := p12DispatchObject(ctx, plugin, "show bgp adj-rib-in status")
	if err != nil || status != "done" {
		return 0, false
	}
	value, exists := p12FindField(data, "total-routes")
	if !exists {
		return 0, false
	}
	return p12Number(value)
}

func p12Quiesce(ctx context.Context, plugin *sdk.Plugin) error {
	_, err := p12RequireDone(ctx, plugin, "request quiesce")
	return err
}

func p12Settle(ctx context.Context, cycles int, delay time.Duration) {
	Poll(ctx, cycles+1, delay, func() bool { return false })
}

func p12BusSubscribe(plugin *sdk.Plugin) p12Scenario {
	peerUp := make(chan struct{}, 1)
	plugin.OnEvent(func(event string) error {
		var decoded map[string]any
		if json.Unmarshal([]byte(event), &decoded) != nil {
			return nil
		}
		bgp, _ := decoded["bgp"].(map[string]any)
		message, _ := bgp["message"].(map[string]any)
		if message["type"] == "state" && bgp["state"] == "up" {
			select {
			case peerUp <- struct{}{}:
			default:
			}
		}
		return nil
	})

	return func(ctx context.Context, plugin *sdk.Plugin) error {
		timer := time.NewTimer(10 * time.Second)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return fmt.Errorf("no bgp state=up event delivered to the subscribed plugin")
		case <-peerUp:
			fmt.Fprintln(os.Stderr, "OK: Bus lifecycle — peer-up state event delivered")
		}
		if !p12WaitPeerCounter(ctx, plugin, "*", "eor-sent", 1, 40) {
			return fmt.Errorf("ze never put the initial-sync end-of-rib on the wire")
		}
		return nil
	}
}
