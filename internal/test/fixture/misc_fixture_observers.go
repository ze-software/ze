package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("encode/watchdog-encode", observeDriver("service-watchdog", watchdogScenario))
	Register("managed/config-push-transactional-observer", managedRejectObserver)
	Register("parse/healthcheck-parse", idleDriver)
	Register("reload/mgmt-guard-reload-auth-rebuild", reloadRESTObserver("mgmt-auth-rebuild-test", reloadRESTAuthRebuild))
	Register("reload/mgmt-guard-reload-refuses-unauth", reloadRESTObserver("mgmt-refuses-unauth-test", reloadRESTRefusesUnauth))
	Register("reload/mgmt-guard-reload-unbuilt-transport", reloadRESTObserver("mgmt-unbuilt-transport-test", reloadRESTUnbuiltTransport))
	Register("reload/reload-import-policy-applies", observeDriver("reload-policy-applies", reloadPolicyScenario))
	Register("reload/reload-prefix-updated-clears-stale", observeDriver("prefix-stale-reload", reloadPrefixScenario))
	Register("reload/signal-quit", observeDriver("quit-test", signalQuitScenario))
	Register("runner/json-assertion-survives-sink-peer", observeDriver("sink-json", sinkJSONScenario))
	Register("vpp/vpp-fib-route-lookup-observer", observeDriver("lookup-test", vppLookupScenario))
}

func observeDriver(pluginName string, scenario ObserverScenario) Driver {
	return func(ctx context.Context, args []string) error {
		if len(args) != 0 {
			return fmt.Errorf("%s takes no arguments", pluginName)
		}
		return Observe(ctx, pluginName, sdk.Registration{}, scenario)
	}
}

func idleDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("healthcheck fixture takes no arguments")
	}
	<-ctx.Done()
	return nil
}

func dispatchMap(ctx context.Context, plugin *sdk.Plugin, command string) (string, map[string]any, error) {
	status, raw, err := plugin.DispatchCommand(ctx, command)
	if err != nil {
		return status, nil, err
	}
	var value any
	if len(raw) != 0 {
		if err := json.Unmarshal(raw, &value); err != nil {
			return status, nil, fmt.Errorf("decode %q: %w", command, err)
		}
		if encoded, ok := value.(string); ok && encoded != "" {
			if err := json.Unmarshal([]byte(encoded), &value); err != nil {
				return status, nil, fmt.Errorf("decode structured answer for %q: %w", command, err)
			}
		}
	}
	object, ok := value.(map[string]any)
	if !ok {
		return status, nil, fmt.Errorf("decode %q: answer is %T, want object", command, value)
	}
	return status, object, nil
}

func peerDetail(ctx context.Context, plugin *sdk.Plugin, selector string) (map[string]any, error) {
	status, value, err := dispatchMap(ctx, plugin, "show bgp peer "+selector+" detail")
	if err != nil {
		return nil, err
	}
	if status != "done" {
		return nil, fmt.Errorf("show bgp peer %s detail status=%s", selector, status)
	}
	peers, ok := value["peers"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("show bgp peer %s detail has no peers map", selector)
	}
	if exact, ok := peers[selector].(map[string]any); ok {
		return exact, nil
	}
	for _, raw := range peers {
		if peer, ok := raw.(map[string]any); ok {
			return peer, nil
		}
	}
	return nil, fmt.Errorf("show bgp peer %s detail returned no peer", selector)
}

func peerCounter(ctx context.Context, plugin *sdk.Plugin, selector, counter string) int64 {
	peer, err := peerDetail(ctx, plugin, selector)
	if err != nil {
		return -1
	}
	value, ok := peer[counter].(float64)
	if !ok {
		return -1
	}
	return int64(value)
}

func waitPeerCounter(ctx context.Context, plugin *sdk.Plugin, selector, counter string, want int64, attempts int) bool {
	return Poll(ctx, attempts, 100*time.Millisecond, func() bool {
		return peerCounter(ctx, plugin, selector, counter) >= want
	})
}

func quiesce(ctx context.Context, plugin *sdk.Plugin) error {
	status, err := Dispatch(ctx, plugin, "request quiesce", nil)
	if err != nil {
		return err
	}
	if status != "done" {
		return fmt.Errorf("request quiesce status=%s", status)
	}
	return nil
}

func watchdogScenario(ctx context.Context, plugin *sdk.Plugin) error {
	if !waitPeerCounter(ctx, plugin, "*", "eor-sent", 1, 100) {
		return errors.New("initial-sync EOR never reached the wire")
	}
	for range 3 {
		for _, command := range []string{"request bgp watchdog announce dnsr", "request bgp watchdog withdraw dnsr"} {
			status, err := Dispatch(ctx, plugin, command, nil)
			if err != nil || status != "done" {
				return fmt.Errorf("%s status=%s: %w", command, status, err)
			}
			if err := quiesce(ctx, plugin); err != nil {
				return err
			}
		}
	}
	return nil
}

type reloadRESTMode int

const (
	reloadRESTAuthRebuild reloadRESTMode = iota
	reloadRESTRefusesUnauth
	reloadRESTUnbuiltTransport
)

func reloadRESTObserver(pluginName string, mode reloadRESTMode) Driver {
	return func(ctx context.Context, args []string) error {
		if len(args) != 1 {
			return errors.New("reload REST observer requires the REST port")
		}
		port, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("REST port: %w", err)
		}
		return Observe(ctx, pluginName, sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
			return runReloadRESTScenario(ctx, plugin, mode, port)
		})
	}
}

func restStatus(ctx context.Context, port int, token string) int {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/api/v1/commands", port), nil)
	if err != nil {
		return 0
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		return 0
	}
	defer response.Body.Close() //nolint:errcheck // status is the assertion
	return response.StatusCode
}

func reloadGeneration(ctx context.Context, plugin *sdk.Plugin) int64 {
	status, value, err := dispatchMap(ctx, plugin, "show reload-status")
	if err != nil || status != "done" {
		return -1
	}
	generation, ok := value["generation"].(float64)
	if !ok {
		return -1
	}
	return int64(generation)
}

func runReloadRESTScenario(ctx context.Context, plugin *sdk.Plugin, mode reloadRESTMode, port int) error {
	bootToken := ""
	initialStatus := http.StatusOK
	if mode != reloadRESTAuthRebuild {
		bootToken = "boot-token"
		initialStatus = http.StatusOK
	}
	if !Poll(ctx, 60, 250*time.Millisecond, func() bool { return restStatus(ctx, port, bootToken) == initialStatus }) {
		return fmt.Errorf("before SIGHUP: REST never answered as expected: status=%d", restStatus(ctx, port, bootToken))
	}
	if mode != reloadRESTAuthRebuild && restStatus(ctx, port, "") != http.StatusUnauthorized {
		return fmt.Errorf("before SIGHUP: unauthenticated read status=%d, want 401", restStatus(ctx, port, ""))
	}
	baseline := int64(-1)
	if !Poll(ctx, 40, 100*time.Millisecond, func() bool {
		baseline = reloadGeneration(ctx, plugin)
		return baseline >= 0
	}) {
		return errors.New("before SIGHUP: show reload-status returned no generation")
	}
	if err := os.WriteFile("observer.initial-ok", []byte("ok"), 0o600); err != nil {
		return err
	}
	if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
		_, err := os.Stat("reload.done")
		return err == nil
	}) {
		return errors.New("reload trigger never created reload.done")
	}
	generation := int64(-1)
	if !Poll(ctx, 150, 100*time.Millisecond, func() bool {
		generation = reloadGeneration(ctx, plugin)
		return generation > baseline
	}) {
		return fmt.Errorf("reload generation never advanced past %d: got %d", baseline, generation)
	}
	switch mode {
	case reloadRESTAuthRebuild:
		if status := restStatus(ctx, port, ""); status != http.StatusUnauthorized {
			return fmt.Errorf("after SIGHUP: unauthenticated read status=%d, want 401", status)
		}
		if status := restStatus(ctx, port, "reload-secret"); status != http.StatusOK {
			return fmt.Errorf("after SIGHUP: reloaded token status=%d, want 200", status)
		}
	case reloadRESTRefusesUnauth:
		if status := restStatus(ctx, port, ""); status != http.StatusUnauthorized {
			return fmt.Errorf("after refused SIGHUP: unauthenticated read status=%d, want 401", status)
		}
		if status := restStatus(ctx, port, "boot-token"); status != http.StatusOK {
			return fmt.Errorf("after refused SIGHUP: boot token status=%d, want 200", status)
		}
	case reloadRESTUnbuiltTransport:
		if status := restStatus(ctx, port, ""); status != http.StatusOK {
			return fmt.Errorf("after SIGHUP: unauthenticated read status=%d, want 200", status)
		}
	}
	return nil
}

func reloadPolicyScenario(ctx context.Context, plugin *sdk.Plugin) error {
	const update = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002F02000000144001010040020602010000FDE940030401010101180A0000"
	var policy string
	if !Poll(ctx, 50, 200*time.Millisecond, func() bool {
		peer, err := peerDetail(ctx, plugin, "peer1")
		if err != nil {
			return false
		}
		raw, _ := json.Marshal(peer["import-policy"])
		policy = string(raw)
		return strings.Contains(policy, "DENY")
	}) {
		return fmt.Errorf("reloaded import chain never reached running peer: %s", policy)
	}
	peer, err := peerDetail(ctx, plugin, "peer1")
	if err != nil {
		return err
	}
	if peer["state"] != "established" {
		return fmt.Errorf("session must survive import-policy edit, state=%v", peer["state"])
	}
	evaluate := func(selector string) (string, error) {
		status, value, err := dispatchMap(ctx, plugin, "show policy test peer "+selector+" import update "+update)
		if err != nil || status != "done" {
			return "", fmt.Errorf("policy test %s status=%s: %w", selector, status, err)
		}
		action, _ := value["action"].(string)
		return action, nil
	}
	control, err := evaluate("control-peer")
	if err != nil || control != "accept" {
		return fmt.Errorf("unedited ALLOW chain must accept, got %q: %w", control, err)
	}
	after, err := evaluate("peer1")
	if err != nil || after != "reject" {
		return fmt.Errorf("DENY chain must reject after reload, got %q: %w", after, err)
	}
	fmt.Fprintln(os.Stderr, "OK: the edited chain rejects the route the unedited chain accepts")
	return nil
}

func staleWarning(ctx context.Context, plugin *sdk.Plugin) bool {
	status, value, err := dispatchMap(ctx, plugin, "show warnings")
	if err != nil || status != "done" {
		return false
	}
	warnings, _ := value["warnings"].([]any)
	for _, raw := range warnings {
		warning, _ := raw.(map[string]any)
		if warning["source"] == "bgp" && warning["code"] == "prefix-stale" {
			return true
		}
	}
	return false
}

func reloadPrefixScenario(ctx context.Context, plugin *sdk.Plugin) error {
	if !Poll(ctx, 100, 100*time.Millisecond, func() bool { return staleWarning(ctx, plugin) }) {
		return errors.New("timed out waiting for the prefix-stale warning to be raised")
	}
	if err := os.WriteFile("saw-stale", []byte("1"), 0o600); err != nil {
		return err
	}
	if !Poll(ctx, 150, 100*time.Millisecond, func() bool { return !staleWarning(ctx, plugin) }) {
		return errors.New("timed out waiting for the prefix-stale warning to clear after reload")
	}
	fmt.Fprintln(os.Stderr, "OK: refreshed prefix dates cleared the stale warning")
	return nil
}

func signalQuitScenario(ctx context.Context, plugin *sdk.Plugin) error {
	if !waitPeerCounter(ctx, plugin, "peer1", "eor-sent", 1, 40) {
		return errors.New("initial-sync EOR never reached the wire")
	}
	status, err := Dispatch(ctx, plugin, "request halt", nil)
	if err != nil {
		return err
	}
	if status != "done" {
		return fmt.Errorf("quit status=%s", status)
	}
	fmt.Fprintln(os.Stderr, "OK: daemon quit dispatched")
	return nil
}

func sinkJSONScenario(ctx context.Context, plugin *sdk.Plugin) error {
	if !waitPeerCounter(ctx, plugin, "*", "eor-sent", 1, 100) {
		return errors.New("peer never reported eor-sent")
	}
	fmt.Fprintln(os.Stderr, "OK: eor-sent observed")
	return nil
}

func vppLookupScenario(ctx context.Context, plugin *sdk.Plugin) error {
	var value map[string]any
	var status string
	var err error
	if !Poll(ctx, 40, 100*time.Millisecond, func() bool {
		status, value, err = dispatchMap(ctx, plugin, "show route lookup 10.20.0.1")
		return err == nil && status == "done"
	}) {
		return fmt.Errorf("route lookup status=%s: %w", status, err)
	}
	if value["prefix"] != "10.20.0.0/24" || value["destination"] != "10.20.0.1" {
		return fmt.Errorf("route lookup returned prefix=%v destination=%v", value["prefix"], value["destination"])
	}
	fmt.Fprintf(os.Stderr, "OK: route lookup returned prefix=%v dest=%v proto=%v\n", value["prefix"], value["destination"], value["protocol"])
	return nil
}

func managedRejectObserver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("managed reject observer takes no arguments")
	}
	plugin, err := newObserver("managed-reject-plugin")
	if err != nil {
		return err
	}
	defer plugin.Close() //nolint:errcheck // Run carries transport errors
	plugin.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root == "bgp" && strings.Contains(section.Data, "2.2.2.2") {
				fmt.Fprintln(os.Stderr, "OK: rejecting managed candidate router-id 2.2.2.2")
				return errors.New("reject router-id 2.2.2.2")
			}
		}
		return nil
	})
	return plugin.Run(ctx, sdk.Registration{WantsConfig: []string{"bgp"}})
}
