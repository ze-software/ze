package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const plugin14PollDelay = 200 * time.Millisecond

type plugin14Event map[string]any

type plugin14EventScenario func(context.Context, *sdk.Plugin, <-chan plugin14Event) error

func init() {
	Register("plugin/rib-reconnect", func(ctx context.Context, _ []string) error {
		return plugin14RunUntilShutdown(ctx, "announce-routes", []plugin14Update{{"*", "update text nhop 10.0.0.1 nlri ipv4/unicast add 192.168.1.0/24"}})
	})
	Register("plugin/rib-withdrawal", func(ctx context.Context, _ []string) error {
		return plugin14RunUntilShutdown(ctx, "announce-routes", []plugin14Update{
			{"*", "update text nhop 10.0.0.1 nlri ipv4/unicast add 192.168.1.0/24"},
			{"*", "update text nlri ipv4/unicast del 192.168.1.0/24"},
		})
	})
	Register("plugin/role-otc-egress-filter", plugin14Observer("test-egress", plugin14OTCEgressFilter))
	Register("plugin/role-otc-egress-stamp", plugin14Observer("test-stamp", plugin14OTCEgressStamp))
	Register("plugin/role-otc-export-unknown", plugin14Observer("test-unknown", plugin14OTCExportUnknown))
	Register("plugin/role-otc-fwd-withdraw", plugin14Observer("test-withdraw", plugin14OTCForwardWithdrawal))
	Register("plugin/role-otc-ingress-reject", plugin14Observer("test-otc", plugin14OTCIngressReject))
	Register("plugin/role-otc-rs-client-dest-stamp", plugin14Observer("test-rs-client-stamp", plugin14OTCRSClientStamp))
	Register("plugin/role-otc-rs-withdraw-eor", plugin14Observer("test-rs-eor", plugin14OTCRSWithdrawEOR))
	Register("plugin/role-otc-unicast-scope", plugin14Observer("test-scope", plugin14OTCUnicastScope))
	Register("plugin/route-modify-localpref", plugin14Observer("test-modify-lp", plugin14ModifyLocalpref))
	Register("plugin/rpf-multicast", plugin14Observer("rpf-multicast-test", plugin14RPFMulticast))
	Register("plugin/rpki-as-set", plugin14Observer("rpki-asset-test", plugin14RPKIAsSet))
	Register("plugin/rpki-aspa-disabled", plugin14EventObserver("rpki-aspa-disabled-test", []string{textRPKIDirectionReceived}, plugin14ASPADisabled))
	Register("plugin/rpki-aspa-invalid", plugin14ASPAStateObserver("rpki-aspa-invalid-test", "invalid"))
	Register("plugin/rpki-aspa-policy-logonly", plugin14EventObserver("rpki-aspa-policy-logonly-test", []string{textRPKIDirectionReceived}, plugin14ASPAPolicyLogOnly))
	Register("plugin/rpki-aspa-policy-reject", plugin14EventObserver("rpki-aspa-policy-reject-test", []string{textRPKIDirectionReceived}, func(ctx context.Context, p *sdk.Plugin, events <-chan plugin14Event) error {
		return plugin14ASPAPolicyReject(ctx, p, events, "invalid", "ASPA policy", " policy")
	}))
	Register("plugin/rpki-aspa-policy-unknown-reject", plugin14EventObserver("rpki-aspa-policy-unknown-reject-test", []string{textRPKIDirectionReceived}, func(ctx context.Context, p *sdk.Plugin, events <-chan plugin14Event) error {
		return plugin14ASPAPolicyReject(ctx, p, events, "unknown", "ASPA unknown policy", " unknown-action=reject")
	}))
	Register("plugin/rpki-aspa-unknown", plugin14ASPAStateObserver("rpki-aspa-unknown-test", "unknown"))
	Register("plugin/rpki-aspa-valid", plugin14ASPAStateObserver("rpki-aspa-valid-test", "valid"))
	Register("plugin/rpki-cache-connect", plugin14Observer("rpki-cache-test", plugin14RPKICacheConnect))
	Register("plugin/rpki-cache-update", plugin14Observer("rpki-update-test", plugin14RPKICacheUpdate))
	Register("plugin/rpki-decorator-autoload", plugin14EventObserver("rpki-auto-test", []string{textUpdateRPKIDirectionReceived}, plugin14DecoratorAutoload))
	Register("plugin/rpki-decorator-merge", plugin14EventObserver("rpki-dec-test", []string{textUpdateRPKIDirectionReceived}, plugin14DecoratorMerge))
	Register("plugin/rpki-decorator-register", plugin14EventObserver("rpki-reg-test", []string{textUpdateRPKIDirectionReceived}, plugin14DecoratorRegister))
	Register("plugin/rpki-decorator-timeout", plugin14EventObserver("rpki-to-test", []string{textUpdateRPKIDirectionReceived}, plugin14DecoratorTimeout))
	Register("plugin/rpki-event-multi", plugin14EventObserver("rpki-event-multi-test", []string{textRPKIDirectionReceived}, plugin14RPKIEventMulti))
	Register("plugin/rpki-event-unavailable", plugin14EventObserver("rpki-unavail-test", []string{textRPKIDirectionReceived}, plugin14RPKIEventUnavailable))
	Register("plugin/rpki-event-valid", plugin14EventObserver("rpki-event-test", []string{textRPKIDirectionReceived}, plugin14RPKIEventValid))
	Register("plugin/rpki-group-action", plugin14Observer("rpki-group-test", plugin14RPKIGroupAction))
	Register("plugin/rpki-maxlength", plugin14Observer("rpki-maxlen-test", plugin14RPKIMaxlength))
	Register("plugin/rpki-multi-prefix", plugin14Observer("rpki-multi-test", plugin14RPKIMultiPrefix))
}

type plugin14Update struct {
	peer    string
	command string
}

func plugin14RunUntilShutdown(ctx context.Context, name string, updates []plugin14Update) error {
	plugin, err := newObserver(name)
	if err != nil {
		return fmt.Errorf("connect observer %s: %w", name, err)
	}
	defer plugin.Close() //nolint:errcheck // fixture teardown, so a close failure changes no assertion

	result := make(chan error, 1)
	plugin.OnAllPluginsReady(func() error {
		for _, update := range updates {
			if _, _, err := plugin.UpdateRoute(ctx, update.peer, update.command); err != nil {
				result <- fmt.Errorf("send %q: %w", update.command, err)
				return err
			}
			status, _, err := plugin.DispatchCommand(ctx, "request quiesce")
			if err != nil || status != rpc.StatusDone {
				barrierErr := fmt.Errorf("wait for update acknowledgement: status=%q: %w", status, err)
				result <- barrierErr
				return barrierErr
			}
		}
		result <- nil
		return nil
	})

	runErr := plugin.Run(ctx, sdk.Registration{})
	select {
	case scenarioErr := <-result:
		return errors.Join(scenarioErr, runErr)
	default:
		return runErr
	}
}

func plugin14Observer(name string, scenario ObserverScenario) Driver {
	return func(ctx context.Context, _ []string) error {
		return Observe(ctx, name, sdk.Registration{}, scenario)
	}
}

func plugin14EventObserver(name string, subscriptions []string, scenario plugin14EventScenario) Driver {
	return func(ctx context.Context, _ []string) error {
		events := make(chan plugin14Event, 256)
		setup := func(plugin *sdk.Plugin) error {
			if len(subscriptions) != 0 {
				plugin.SetStartupSubscriptions(subscriptions, nil, "")
			}
			plugin.OnEvent(func(event string) error {
				var decoded plugin14Event
				if err := json.Unmarshal([]byte(event), &decoded); err != nil {
					return nil //nolint:nilerr // a malformed event is skipped, and failing the handler would end the session
				}
				select {
				case events <- decoded:
				default:
					return errors.New("observer event buffer exhausted")
				}
				return nil
			})
			return nil
		}
		return observeConfigured(ctx, name, sdk.Registration{}, setup, func(ctx context.Context, plugin *sdk.Plugin) error {
			return scenario(ctx, plugin, events)
		})
	}
}

func plugin14ASPAStateObserver(name, want string) Driver {
	return plugin14EventObserver(name, []string{textRPKIDirectionReceived}, func(ctx context.Context, _ *sdk.Plugin, events <-chan plugin14Event) error {
		if ok := plugin14WaitEvent(ctx, events, 15*time.Second, func(event plugin14Event) bool {
			return plugin14RPKISection(event, false)["aspa-state"] == want
		}); !ok {
			return fmt.Errorf("no rpki event with aspa-state=%s", want)
		}
		fmt.Fprintf(os.Stderr, "OK: rpki event has aspa-state=%s\n", want)
		return nil
	})
}

func plugin14Dispatch(ctx context.Context, plugin *sdk.Plugin, command string) (string, any, error) {
	var value any
	status, err := Dispatch(ctx, plugin, command, &value)
	return status, value, err
}

func plugin14DispatchMap(ctx context.Context, plugin *sdk.Plugin, command string) (string, map[string]any, error) {
	status, value, err := plugin14Dispatch(ctx, plugin, command)
	if err != nil {
		return status, nil, err
	}
	row, _ := value.(map[string]any)
	return status, row, nil
}

func plugin14Text(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func plugin14Map(value any) map[string]any {
	row, _ := value.(map[string]any)
	return row
}

func plugin14Maps(value any) []any {
	rows, _ := value.([]any)
	return rows
}

func plugin14Number(value any) int {
	switch number := value.(type) {
	case float64:
		return int(number)
	case int:
		return number
	case json.Number:
		parsed, _ := number.Int64()
		return int(parsed)
	default:
		return 0
	}
}

func plugin14PeerRows(data map[string]any) map[string]any {
	return plugin14Map(data["peers"])
}

func plugin14PeerCounter(ctx context.Context, plugin *sdk.Plugin, peer, counter string) int {
	_, data, err := plugin14DispatchMap(ctx, plugin, "show bgp peer * detail")
	if err != nil {
		return 0
	}
	return plugin14Number(plugin14Map(plugin14PeerRows(data)[peer])[counter])
}

func plugin14Quiesce(ctx context.Context, plugin *sdk.Plugin) error {
	status, _, err := plugin14Dispatch(ctx, plugin, "request quiesce")
	if err != nil || status != rpc.StatusDone {
		return fmt.Errorf("quiesce barrier did not settle: status=%q: %w", status, err)
	}
	return nil
}

func plugin14PollCommand(ctx context.Context, plugin *sdk.Plugin, attempts int, command string, predicate func(string, any) bool) (string, any, error) {
	var status string
	var value any
	var lastErr error
	matched := Poll(ctx, attempts, plugin14PollDelay, func() bool {
		status, value, lastErr = plugin14Dispatch(ctx, plugin, command)
		return lastErr == nil && predicate(status, value)
	})
	if !matched {
		if lastErr != nil {
			return status, value, lastErr
		}
		return status, value, fmt.Errorf("condition not reached after %d attempts", attempts)
	}
	return status, value, nil
}

func plugin14WaitEvent(ctx context.Context, events <-chan plugin14Event, timeout time.Duration, predicate func(plugin14Event) bool) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-timer.C:
			return false
		case event := <-events:
			if predicate(event) {
				return true
			}
		}
	}
}

func plugin14NextEvent(ctx context.Context, events <-chan plugin14Event, timeout time.Duration) (plugin14Event, bool) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, false
	case <-timer.C:
		return nil, false
	case event := <-events:
		return event, true
	}
}

// plugin14EventTimeout bounds one wait for the next event.
const plugin14EventTimeout = 500 * time.Millisecond

func plugin14WaitEventAttempts(ctx context.Context, events <-chan plugin14Event, attempts int, predicate func(plugin14Event) bool) (plugin14Event, bool) {
	for range attempts {
		event, ok := plugin14NextEvent(ctx, events, plugin14EventTimeout)
		if ok && predicate(event) {
			return event, true
		}
		if ctx.Err() != nil {
			break
		}
	}
	return nil, false
}

func plugin14MessageType(event plugin14Event) string {
	bgp := plugin14Map(event["bgp"])
	message := plugin14Map(bgp["message"])
	typeName, _ := message["type"].(string)
	return typeName
}

func plugin14RPKISection(event plugin14Event, allowUnavailable bool) map[string]any {
	if plugin14MessageType(event) != "rpki" {
		return nil
	}
	rpki := plugin14Map(plugin14Map(event["bgp"])["rpki"])
	if rpki == nil || (!allowUnavailable && rpki["status"] == "unavailable") {
		return nil
	}
	return rpki
}

func plugin14RPKIEventHasPrefix(event plugin14Event, prefix, state string) bool {
	rpki := plugin14RPKISection(event, false)
	family := plugin14Map(rpki["ipv4/unicast"])
	return family[prefix] == state
}

func plugin14EORSent(status string, value any) bool {
	if status != rpc.StatusDone {
		return false
	}
	for _, row := range plugin14PeerRows(plugin14Map(value)) {
		if plugin14Number(plugin14Map(row)["eor-sent"]) >= 1 {
			return true
		}
	}
	return false
}

func plugin14DestGotRoute(status string, value any) bool {
	if status != rpc.StatusDone {
		return false
	}
	for _, row := range plugin14PeerRows(plugin14Map(value)) {
		peer := plugin14Map(row)
		if plugin14Number(peer["updates-sent"])-plugin14Number(peer["eor-sent"]) >= 1 {
			return true
		}
	}
	return false
}

func plugin14OTCEgressFilter(ctx context.Context, plugin *sdk.Plugin) error {
	_, _, err := plugin14PollCommand(ctx, plugin, 100, "show bgp peer * detail", func(status string, value any) bool {
		if status != rpc.StatusDone {
			return false
		}
		count := 0
		for _, row := range plugin14PeerRows(plugin14Map(value)) {
			if plugin14Number(plugin14Map(row)["eor-sent"]) >= 1 {
				count++
			}
		}
		return count >= 2
	})
	if err != nil {
		return fmt.Errorf("ze never sent an initial-sync EOR to both peers: %w", err)
	}
	if err := plugin14Quiesce(ctx, plugin); err != nil {
		return fmt.Errorf("forwarding decision not yet made: %w", err)
	}
	_, detail, err := plugin14DispatchMap(ctx, plugin, "show bgp peer * detail")
	if err != nil {
		return err
	}
	rows := plugin14PeerRows(detail)
	source := plugin14Map(rows["127.0.0.1"])
	dest := plugin14Map(rows["127.0.0.2"])
	if source == nil || dest == nil {
		return fmt.Errorf("peer detail missing a peer row")
	}
	if plugin14Number(source["updates-received"]) < 1 {
		return fmt.Errorf("source peer UPDATE never reached ze: %v", source)
	}
	leaked := plugin14Number(dest["updates-sent"]) - plugin14Number(dest["eor-sent"])
	if leaked != 0 {
		return fmt.Errorf("provider route leaked to the provider dest peer: %v", dest)
	}
	_, adj, _ := plugin14DispatchMap(ctx, plugin, "show bgp adj-rib-in status")
	total := -1
	if value, exists := adj["total-routes"]; exists {
		total = plugin14Number(value)
	}
	fmt.Fprintf(os.Stderr, "OK: route accepted from the provider source (%d in adj-rib-in), %d routes beyond EOR sent to the provider dest\n", total, leaked)
	return nil
}

func plugin14OTCEgressStamp(ctx context.Context, plugin *sdk.Plugin) error {
	if err := plugin14Quiesce(ctx, plugin); err != nil {
		return err
	}
	status, value, err := plugin14PollCommand(ctx, plugin, 24, "show bgp peer dest-peer detail", plugin14DestGotRoute)
	if err != nil || !plugin14DestGotRoute(status, value) {
		return fmt.Errorf("dest peer was never sent the OTC-stamped route: %s", plugin14Text(value))
	}
	_, adj, err := plugin14DispatchMap(ctx, plugin, "show bgp adj-rib-in status")
	if err != nil {
		return err
	}
	total := plugin14Number(adj["total-routes"])
	if _, exists := adj["total-routes"]; !exists {
		return errors.New("adj-rib-in query failed")
	}
	fmt.Fprintf(os.Stderr, "INFO: %d route(s) in adj-rib-in from source Customer peer\n", total)
	fmt.Fprintf(os.Stderr, "INFO: dest-peer detail after the re-advertisement: %s\n", plugin14Text(value))
	fmt.Fprintln(os.Stderr, "OK: dest peer was sent the OTC-stamped route")
	return nil
}

func plugin14OTCExportUnknown(ctx context.Context, plugin *sdk.Plugin) error {
	_, _, err := plugin14PollCommand(ctx, plugin, 100, "show bgp adj-rib-in status", func(_ string, value any) bool {
		return plugin14Number(plugin14Map(value)["total-routes"]) >= 1
	})
	if err != nil {
		return err
	}
	if err := plugin14Quiesce(ctx, plugin); err != nil {
		return err
	}
	_, data, err := plugin14DispatchMap(ctx, plugin, "show bgp adj-rib-in status")
	if err != nil {
		return err
	}
	total := plugin14Number(data["total-routes"])
	if total < 1 {
		return fmt.Errorf("adj-rib-in never stored the route, got total=%d", total)
	}
	fmt.Fprintln(os.Stderr, "OK: route from untagged peer accepted while dest wire assertion verified forwarding")
	return nil
}

func plugin14WaitDestUpdates(ctx context.Context, plugin *sdk.Plugin, count int, what string) error {
	if !Poll(ctx, 40, plugin14PollDelay, func() bool {
		_, data, err := plugin14DispatchMap(ctx, plugin, "show bgp peer * detail")
		if err != nil {
			return false
		}
		dest := plugin14Map(plugin14PeerRows(data)["127.0.0.2"])
		return plugin14Number(dest["updates-sent"])-plugin14Number(dest["eor-sent"]) >= count
	}) {
		return fmt.Errorf("dest peer was never sent %s", what)
	}
	fmt.Fprintf(os.Stderr, "OK: dest was sent %s\n", what)
	return nil
}

func plugin14OTCForwardWithdrawal(ctx context.Context, plugin *sdk.Plugin) error {
	if err := plugin14Quiesce(ctx, plugin); err != nil {
		return err
	}
	if err := plugin14WaitDestUpdates(ctx, plugin, 1, "the stamped advertisement"); err != nil {
		return err
	}
	if _, _, err := plugin.UpdateRoute(ctx, "127.0.0.1", "update text origin igp nhop 1.1.1.1 nlri ipv4/unicast add 192.0.2.0/24"); err != nil {
		return err
	}
	return plugin14WaitDestUpdates(ctx, plugin, 2, "the forwarded withdrawal")
}

func plugin14OTCIngressReject(ctx context.Context, plugin *sdk.Plugin) error {
	Poll(ctx, 100, plugin14PollDelay, func() bool {
		return plugin14PeerCounter(ctx, plugin, "127.0.0.1", "updates-received") >= 1
	})
	if err := plugin14Quiesce(ctx, plugin); err != nil {
		return err
	}
	_, data, err := plugin14DispatchMap(ctx, plugin, "show bgp adj-rib-in status")
	if err != nil {
		return err
	}
	total, exists := data["total-routes"]
	if !exists {
		return errors.New("adj-rib-in query failed: total-routes missing")
	}
	if plugin14Number(total) != 0 {
		return fmt.Errorf("OTC ingress filter did NOT reject the customer route leak: adj-rib-in total-routes=%d, want 0", plugin14Number(total))
	}
	fmt.Fprintln(os.Stderr, "OK: OTC ingress filter rejected route leak from customer (adj-rib-in empty)")
	return nil
}

func plugin14OTCRSClientStamp(ctx context.Context, plugin *sdk.Plugin) error {
	if err := plugin14Quiesce(ctx, plugin); err != nil {
		return err
	}
	return plugin14WaitDestUpdates(ctx, plugin, 1, "the OTC-stamped route")
}

func plugin14OTCRSWithdrawEOR(ctx context.Context, plugin *sdk.Plugin) error {
	if err := plugin14Quiesce(ctx, plugin); err != nil {
		return err
	}
	if err := plugin14WaitDestUpdates(ctx, plugin, 1, "the OTC-stamped advertisement"); err != nil {
		return err
	}
	if _, _, err := plugin.UpdateRoute(ctx, "127.0.0.1", "update text origin igp nhop 1.1.1.1 nlri ipv4/unicast add 192.0.2.0/24"); err != nil {
		return err
	}
	if err := plugin14WaitDestUpdates(ctx, plugin, 2, "the forwarded withdrawal"); err != nil {
		return err
	}
	if _, _, err := plugin.UpdateRoute(ctx, "127.0.0.1", "update text origin igp nhop 1.1.1.1 nlri ipv4/unicast add 198.51.100.0/24"); err != nil {
		return err
	}
	if err := plugin14WaitDestUpdates(ctx, plugin, 3, "the relayed End-of-RIB"); err != nil {
		return err
	}
	status, value, err := plugin14PollCommand(ctx, plugin, 100, "show bgp peer * detail", plugin14EORSent)
	if err != nil || !plugin14EORSent(status, value) {
		return errors.New("ze did not send the End-of-RIB to the peer before shutdown")
	}
	return nil
}

func plugin14OTCUnicastScope(ctx context.Context, plugin *sdk.Plugin) error {
	if err := plugin14Quiesce(ctx, plugin); err != nil {
		return err
	}
	status, value, err := plugin14PollCommand(ctx, plugin, 24, "show bgp peer dest-peer detail", plugin14DestGotRoute)
	if err != nil || !plugin14DestGotRoute(status, value) {
		return fmt.Errorf("dest peer was never sent the multicast route: %s", plugin14Text(value))
	}
	_, data, err := plugin14DispatchMap(ctx, plugin, "show bgp adj-rib-in status")
	if err != nil {
		return err
	}
	if total := plugin14Number(data["total-routes"]); total < 1 {
		return fmt.Errorf("adj-rib-in never stored the multicast route, got total=%d", total)
	}
	fmt.Fprintln(os.Stderr, "OK: multicast route with OTC accepted and forwarded to the dest peer")
	return nil
}

func plugin14ModifyLocalpref(ctx context.Context, plugin *sdk.Plugin) error {
	_, _, _ = plugin14PollCommand(ctx, plugin, 100, "show bgp adj-rib-in status", func(_ string, value any) bool {
		return plugin14Number(plugin14Map(value)["total-routes"]) >= 1
	})
	return nil
}

func plugin14RPFMulticast(ctx context.Context, plugin *sdk.Plugin) error {
	status, value, err := plugin14PollCommand(ctx, plugin, 60, "show bgp peer peer1 detail", plugin14EORSent)
	if err != nil || !plugin14EORSent(status, value) {
		return errors.New("peer1 never sent its initial-sync End-of-RIB")
	}
	for _, command := range []string{
		"request bgp rib inject 10.0.0.99 ipv4/multicast 224.0.0.0/4 nhop 10.0.0.1",
		"request bgp rib inject 10.0.0.99 ipv4/multicast 224.1.0.0/16 nhop 10.0.0.2",
	} {
		status, value, err = plugin14Dispatch(ctx, plugin, command)
		if err != nil || status != rpc.StatusDone {
			return fmt.Errorf("inject status=%q data=%s: %w", status, plugin14Text(value), err)
		}
	}
	_, _, _ = plugin14PollCommand(ctx, plugin, 100, "show bgp rib rpf ipv4/multicast 224.1.2.5", func(_ string, value any) bool {
		return plugin14Map(value)["matched-prefix"] == "224.1.0.0/16"
	})
	status, data, err := plugin14DispatchMap(ctx, plugin, "show bgp rib rpf ipv4/multicast 224.1.2.5")
	if err != nil || status != rpc.StatusDone {
		return fmt.Errorf("rpf status=%q: %w", status, err)
	}
	if data["found"] != true || data["matched-prefix"] != "224.1.0.0/16" {
		return fmt.Errorf("unexpected RPF match: %v", data)
	}
	status, data, err = plugin14DispatchMap(ctx, plugin, "show bgp rib rpf ipv4/multicast 192.168.1.1")
	if err != nil || status != rpc.StatusDone || data["found"] != false {
		return fmt.Errorf("RPF no-match failed: status=%q data=%v: %w", status, data, err)
	}
	status, value, err = plugin14Dispatch(ctx, plugin, "show bgp rib rpf l2vpn/evpn 10.0.0.1")
	if status != rpc.StatusError {
		return fmt.Errorf("rpf non-CIDR should error, got status=%q data=%s: %w", status, plugin14Text(value), err)
	}
	fmt.Fprintln(os.Stderr, "OK: rpf multicast verified")
	return nil
}

func plugin14WaitRPKIReady(ctx context.Context, plugin *sdk.Plugin) error {
	if !Poll(ctx, 150, plugin14PollDelay, func() bool {
		_, data, err := plugin14DispatchMap(ctx, plugin, "show bgp rpki status")
		return err == nil && plugin14Number(data["vrp-count-ipv4"]) >= 1
	}) {
		return errors.New("RTR never synced a VRP")
	}
	if !Poll(ctx, 150, plugin14PollDelay, func() bool {
		return plugin14PeerCounter(ctx, plugin, "127.0.0.1", "updates-received") >= 1
	}) {
		return errors.New("ze never received the UPDATE")
	}
	return plugin14Quiesce(ctx, plugin)
}

// plugin14AdjRIBInCommand is the command these presence assertions read.
const plugin14AdjRIBInCommand = "show bgp adj-rib-in"

// plugin14AssertRoutePresence asserts that a validated route is, or is not, in
// the Adj-RIB-In.
//
// The two directions are not symmetrical, because installation is asynchronous
// and takes one of two rails. With the adj-rib-in validation gate already on,
// an arriving route is parked in the manager's pending map and reaches ribIn
// only when the RPKI verdict comes back (adj_rib_in/rib.go,
// installStructuredNLRIs; the promotion is rib_commands.go, acceptRoutesCommand
// and batchValidateCommand). With the gate not yet on, the route is installed at
// ingest and a later cache sync re-decides it in place (applyToInstalled). Which
// rail runs depends on whether the rpki plugin's `request bgp adj-rib-in
// enable-validation` (rpki/rpki.go) landed before the UPDATE, and neither the
// peer's updates-received counter nor `request quiesce`, which drains the
// reactor's forward pool, covers the verdict round trip.
//
// So a presence assertion WAITS for the install it asserts: a single read
// answered `{"adj-rib-in":{}}` a millisecond after the last event and failed the
// case. An absence assertion cannot wait, because an empty Adj-RIB-In is what
// both a rejected route and an unfinished validation look like; it reads once,
// as it always has.
func plugin14AssertRoutePresence(ctx context.Context, plugin *sdk.Plugin, prefix string, present bool, label string) error {
	if present {
		status, value, err := plugin14PollCommand(ctx, plugin, 40, plugin14AdjRIBInCommand, func(status string, value any) bool {
			return status == rpc.StatusDone && strings.Contains(plugin14Text(value), prefix)
		})
		if err != nil {
			return fmt.Errorf("%s: route %s never reached the adj-rib-in: status=%q data=%s: %w",
				label, prefix, status, plugin14Text(value), err)
		}
		return nil
	}

	status, value, err := plugin14Dispatch(ctx, plugin, plugin14AdjRIBInCommand)
	if err != nil || status != rpc.StatusDone {
		return fmt.Errorf("%s status=%q: %w", label, status, err)
	}
	if strings.Contains(plugin14Text(value), prefix) {
		return fmt.Errorf("%s: route %s is in the adj-rib-in and must not be: %s", label, prefix, plugin14Text(value))
	}
	return nil
}

func plugin14RPKIAsSet(ctx context.Context, plugin *sdk.Plugin) error {
	if err := plugin14WaitRPKIReady(ctx, plugin); err != nil {
		return err
	}
	if err := plugin14AssertRoutePresence(ctx, plugin, "10.0.1.0/24", false, "AS_SET validation"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: route with AS_SET correctly rejected (OriginNone -> Invalid)")
	return nil
}

func plugin14ASPADisabled(ctx context.Context, _ *sdk.Plugin, events <-chan plugin14Event) error {
	event, ok := plugin14WaitEventAttempts(ctx, events, 30, func(event plugin14Event) bool {
		return plugin14RPKISection(event, false) != nil
	})
	if !ok {
		return errors.New("no rpki event received at all")
	}
	rpki := plugin14RPKISection(event, false)
	if _, exists := rpki["aspa-state"]; exists {
		return fmt.Errorf("aspa-state present when disabled: %v", rpki)
	}
	fmt.Fprintln(os.Stderr, "OK: rpki event has no aspa-state (disabled)")
	return nil
}

func plugin14ASPAPolicyLogOnly(ctx context.Context, plugin *sdk.Plugin, events <-chan plugin14Event) error {
	if _, ok := plugin14WaitEventAttempts(ctx, events, 30, func(event plugin14Event) bool {
		return plugin14RPKISection(event, false)["aspa-state"] == verdictInvalid
	}); !ok {
		return errors.New("no rpki event with aspa-state=invalid")
	}
	fmt.Fprintln(os.Stderr, "OK: rpki event has aspa-state=invalid (log-only mode)")
	status, value, err := plugin14PollCommand(ctx, plugin, 100, "show bgp adj-rib-in", func(_ string, value any) bool {
		return strings.Contains(plugin14Text(value), "10.0.1.0/24")
	})
	if err != nil || status != rpc.StatusDone || !strings.Contains(plugin14Text(value), "10.0.1.0/24") {
		return fmt.Errorf("route 10.0.1.0/24 should be accepted under log-only policy: status=%q data=%s", status, plugin14Text(value))
	}
	fmt.Fprintln(os.Stderr, "OK: route 10.0.1.0/24 correctly accepted under log-only ASPA policy")
	return nil
}

func plugin14ASPAPolicyReject(ctx context.Context, plugin *sdk.Plugin, events <-chan plugin14Event, state, label, outputSuffix string) error {
	if _, ok := plugin14WaitEventAttempts(ctx, events, 30, func(event plugin14Event) bool {
		return plugin14RPKISection(event, false)["aspa-state"] == state
	}); !ok {
		return fmt.Errorf("no rpki event with aspa-state=%s", state)
	}
	fmt.Fprintf(os.Stderr, "OK: rpki event has aspa-state=%s\n", state)
	if err := plugin14AssertRoutePresence(ctx, plugin, "10.0.1.0/24", false, label); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "OK: route 10.0.1.0/24 correctly rejected by ASPA%s\n", outputSuffix)
	return nil
}

func plugin14RPKICacheConnect(ctx context.Context, plugin *sdk.Plugin) error {
	status, value, err := plugin14PollCommand(ctx, plugin, 60, "show bgp peer peer1 detail", plugin14EORSent)
	if err != nil || !plugin14EORSent(status, value) {
		return errors.New("peer1 never sent its initial-sync End-of-RIB")
	}
	status, value, err = plugin14PollCommand(ctx, plugin, 100, "show bgp rpki status", func(_ string, value any) bool {
		data := plugin14Map(value)
		return plugin14Number(data["vrp-count-ipv4"]) >= 1 && plugin14Number(data["sessions"]) >= 1
	})
	data := plugin14Map(value)
	if err != nil || status != rpc.StatusDone || plugin14Number(data["vrp-count-ipv4"]) < 1 || plugin14Number(data["sessions"]) < 1 {
		return fmt.Errorf("RPKI cache did not synchronize: status=%q data=%v: %w", status, data, err)
	}
	fmt.Fprintf(os.Stderr, "OK: rpki status vrp-count-ipv4=%d sessions=%d\n", plugin14Number(data["vrp-count-ipv4"]), plugin14Number(data["sessions"]))
	return nil
}

func plugin14RPKICacheUpdate(ctx context.Context, plugin *sdk.Plugin) error {
	status, value, err := plugin14PollCommand(ctx, plugin, 100, "show bgp rpki roa", func(_ string, value any) bool {
		return plugin14Number(plugin14Map(value)["total-vrps"]) >= 1
	})
	data := plugin14Map(value)
	if err != nil || status != rpc.StatusDone || plugin14Number(data["total-vrps"]) < 1 {
		return fmt.Errorf("rpki roa did not load a VRP: status=%q data=%v: %w", status, data, err)
	}
	total := plugin14Number(data["total-vrps"])
	status, value, err = plugin14PollCommand(ctx, plugin, 100, "show bgp adj-rib-in", func(_ string, value any) bool {
		return strings.Contains(plugin14Text(value), "10.0.1.0/24")
	})
	if err != nil || status != rpc.StatusDone || !strings.Contains(plugin14Text(value), "10.0.1.0/24") {
		return fmt.Errorf("route 10.0.1.0/24 not in adj-rib-in: status=%q data=%s", status, plugin14Text(value))
	}
	fmt.Fprintf(os.Stderr, "OK: RTR cache updated, route validated (total-vrps=%d)\n", total)
	return nil
}

func plugin14DecoratorAutoload(ctx context.Context, _ *sdk.Plugin, events <-chan plugin14Event) error {
	if _, ok := plugin14WaitEventAttempts(ctx, events, 20, func(event plugin14Event) bool {
		return plugin14MessageType(event) == messageTypeUpdateRPKI
	}); !ok {
		return errors.New("no update-rpki events -- decorator may not have been auto-loaded")
	}
	fmt.Fprintln(os.Stderr, "OK: update-rpki event received -- decorator was auto-loaded")
	return nil
}

func plugin14DecoratorRegister(ctx context.Context, _ *sdk.Plugin, events <-chan plugin14Event) error {
	if _, ok := plugin14WaitEventAttempts(ctx, events, 20, func(event plugin14Event) bool {
		return plugin14MessageType(event) == messageTypeUpdateRPKI
	}); !ok {
		return errors.New("no update-rpki events received -- registration may have failed")
	}
	fmt.Fprintln(os.Stderr, "OK: update-rpki event received -- dynamic event type registration works")
	return nil
}

func plugin14CollectEventAttempts(ctx context.Context, events <-chan plugin14Event, attempts int, timeout time.Duration, match func(plugin14Event) bool) []plugin14Event {
	collected := make([]plugin14Event, 0, 8)
	for range attempts {
		event, ok := plugin14NextEvent(ctx, events, timeout)
		if ok && match(event) {
			collected = append(collected, event)
		}
		if ctx.Err() != nil {
			break
		}
	}
	return collected
}

func plugin14DecoratorMerge(ctx context.Context, _ *sdk.Plugin, events <-chan plugin14Event) error {
	collected := plugin14CollectEventAttempts(ctx, events, 30, 500*time.Millisecond, func(event plugin14Event) bool {
		return plugin14MessageType(event) == messageTypeUpdateRPKI
	})
	if len(collected) == 0 {
		return errors.New("no update-rpki events received")
	}
	foundValid, foundInvalid, foundNotFound := false, false, false
	for _, event := range collected {
		bgp := plugin14Map(event["bgp"])
		update := plugin14Map(bgp["update"])
		if len(update) == 0 {
			return fmt.Errorf("update-rpki event missing or empty update section: %v", event)
		}
		if _, ok := update["attr"]; !ok {
			return fmt.Errorf("update section missing attr key: %v", update)
		}
		family := plugin14Map(plugin14Map(bgp["rpki"])["ipv4/unicast"])
		foundValid = foundValid || family["9.0.1.0/24"] == verdictValid
		foundInvalid = foundInvalid || family["10.0.1.0/24"] == verdictInvalid
		foundNotFound = foundNotFound || family["11.0.1.0/24"] == "not-found"
	}
	missing := make([]string, 0, 3)
	if !foundValid {
		missing = append(missing, "valid (9.0.1.0/24)")
	}
	if !foundInvalid {
		missing = append(missing, "invalid (10.0.1.0/24)")
	}
	if !foundNotFound {
		missing = append(missing, "not-found (11.0.1.0/24)")
	}
	if len(missing) != 0 {
		return fmt.Errorf("missing states: %s in %d events", strings.Join(missing, ", "), len(collected))
	}
	fmt.Fprintf(os.Stderr, "OK: all 3 validation states found in %d update-rpki events\n", len(collected))
	return nil
}

func plugin14DecoratorTimeout(ctx context.Context, _ *sdk.Plugin, events <-chan plugin14Event) error {
	collected := plugin14CollectEventAttempts(ctx, events, 20, 500*time.Millisecond, func(event plugin14Event) bool {
		return plugin14MessageType(event) == messageTypeUpdateRPKI
	})
	if len(collected) == 0 {
		return errors.New("no update-rpki events received after timeout")
	}
	for _, event := range collected {
		bgp := plugin14Map(event["bgp"])
		if len(plugin14Map(bgp["update"])) == 0 {
			return fmt.Errorf("update-rpki event missing update section: %v", event)
		}
		if _, exists := bgp["rpki"]; exists {
			fmt.Fprintln(os.Stderr, "WARNING: rpki section present despite no RTR (may be stale)")
		}
	}
	fmt.Fprintf(os.Stderr, "OK: %d update-rpki events with UPDATE data, graceful timeout\n", len(collected))
	return nil
}

func plugin14RPKIEventMulti(ctx context.Context, _ *sdk.Plugin, events <-chan plugin14Event) error {
	if ok := plugin14WaitEvent(ctx, events, 12*time.Second, func(event plugin14Event) bool {
		return plugin14RPKIEventHasPrefix(event, "10.0.1.0/24", "valid")
	}); !ok {
		return errors.New("no rpki event with valid state for prefix")
	}
	fmt.Fprintln(os.Stderr, "OK: rpki event has per-prefix validation states")
	return nil
}

func plugin14RPKIEventUnavailable(ctx context.Context, _ *sdk.Plugin, events <-chan plugin14Event) error {
	if ok := plugin14WaitEvent(ctx, events, 15*time.Second, func(event plugin14Event) bool {
		return plugin14RPKISection(event, true)["status"] == "unavailable"
	}); !ok {
		return errors.New("expected rpki=unavailable, not found")
	}
	fmt.Fprintln(os.Stderr, "OK: rpki event shows unavailable (empty cache)")
	return nil
}

func plugin14RPKIEventValid(ctx context.Context, _ *sdk.Plugin, events <-chan plugin14Event) error {
	if ok := plugin14WaitEvent(ctx, events, 15*time.Second, func(event plugin14Event) bool {
		return plugin14RPKIEventHasPrefix(event, "10.0.1.0/24", "valid")
	}); !ok {
		return errors.New("no rpki event with valid state for 10.0.1.0/24")
	}
	fmt.Fprintln(os.Stderr, "OK: rpki event has 10.0.1.0/24=valid")
	return nil
}

func plugin14RPKIGroupAction(ctx context.Context, plugin *sdk.Plugin) error {
	if err := plugin14WaitRPKIReady(ctx, plugin); err != nil {
		return err
	}
	if err := plugin14AssertRoutePresence(ctx, plugin, "10.0.1.0/24", true, "group override"); err != nil {
		return err
	}
	_, statusData, err := plugin14DispatchMap(ctx, plugin, "show bgp rpki status")
	if err != nil {
		return err
	}
	var selected map[string]any
	for _, value := range plugin14Maps(statusData["peer-actions"]) {
		row := plugin14Map(value)
		if row["peer"] == addrLoopback {
			selected = row
			break
		}
	}
	if selected == nil {
		return fmt.Errorf("peer-actions missing 127.0.0.1: %v", statusData["peer-actions"])
	}
	invalid := plugin14Map(selected["invalid"])
	if invalid["action"] != actionAccept || invalid["source"] != sourceGroup {
		return fmt.Errorf("peer 127.0.0.1 invalid action/source wrong (want accept/group): %v", invalid)
	}
	fmt.Fprintln(os.Stderr, "OK: Invalid route accepted via group inheritance; status source=group")
	return nil
}

func plugin14RPKIMaxlength(ctx context.Context, plugin *sdk.Plugin) error {
	if err := plugin14WaitRPKIReady(ctx, plugin); err != nil {
		return err
	}
	if err := plugin14AssertRoutePresence(ctx, plugin, "10.0.1.0/25", false, "maxLength validation"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: route 10.0.1.0/25 correctly rejected (exceeds maxLength /24)")
	return nil
}

func plugin14RPKIMultiPrefix(ctx context.Context, plugin *sdk.Plugin) error {
	if err := plugin14WaitRPKIReady(ctx, plugin); err != nil {
		return err
	}
	status, value, err := plugin14PollCommand(ctx, plugin, 40, "show bgp adj-rib-in", func(_ string, value any) bool {
		text := plugin14Text(value)
		return strings.Contains(text, "10.0.1.0/24") && strings.Contains(text, "192.168.1.0/24")
	})
	if err != nil || status != rpc.StatusDone {
		return fmt.Errorf("rib routes received status=%q data=%s: %w", status, plugin14Text(value), err)
	}
	text := plugin14Text(value)
	if !strings.Contains(text, "10.0.1.0/24") {
		return fmt.Errorf("valid route 10.0.1.0/24 not in RIB: %s", text)
	}
	if strings.Contains(text, "10.0.2.0/24") {
		return fmt.Errorf("invalid route 10.0.2.0/24 should be rejected: %s", text)
	}
	if !strings.Contains(text, "192.168.1.0/24") {
		return fmt.Errorf("notfound route 192.168.1.0/24 not in RIB: %s", text)
	}
	fmt.Fprintln(os.Stderr, "OK: 2 routes accepted, 1 rejected (multi-prefix validation)")
	return nil
}
