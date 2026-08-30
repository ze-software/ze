package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/reload-listener-rejected", reloadListenerRejected13)
	Register("plugin/reload-shared-secret", reloadSharedSecret13)
	Register("plugin/reload-listener-rejected-trigger", reloadTrigger13)
	Register("plugin/reload-shared-secret-trigger", reloadTrigger13)
	Register("plugin/remove-private-as-export-originated", originatedPrivateAS13("remove-private-as-export-originated"))
	Register("plugin/remove-private-as-export", forwardedPrivateAS13("remove-private-as-export", "receiver-peer", "stripped"))
	Register("plugin/remove-private-as-import", removePrivateASImport13)
	Register("plugin/remove-private-as-replace-originated", originatedPrivateAS13("remove-private-as-replace-originated"))
	Register("plugin/remove-private-as-replace-peer", forwardedPrivateAS13("remove-private-as-replace-peer", "receiver-peer", "rewritten"))
	Register("plugin/resolve-ping", resolvePing13)
	Register("plugin/rfc4271-partial-unknown-transitive", routeServerReplay13("shutdown-after-up", "10.0.0.0/24"))
	Register("plugin/rfc7606-54-bgpls-override-propagates", routeServerReplay13("shutdown-after-up", "10.0.0.0/24"))
	Register("plugin/rfc7606-54-discard-unrecognized-mup-nlri", routeServerReplay13("shutdown-after-up", "10.0.0.0/24"))
	Register("plugin/rfc7606-54-discard-unrecognized-nlri", routeServerReplay13("shutdown-after-up", "10.0.0.0/24"))
	Register("plugin/rfc7606-receive-combinations", routeServerReplay13("shutdown-after-forward", "10.40.0.0/24"))
	Register("plugin/rfc7606-relay-one-field", routeServerReplay13("shutdown-after-up", "10.0.0.0/24"))
	Register("plugin/rfc7606-reset", passivePlugin13("rfc7606-test"))
	Register("plugin/rfc7606-withdraw", rfc7606Withdraw13)
	Register("plugin/rfc9552-52-rs-opaque-withdraw-peer-down", opaqueWithdraw13)
}

type commandResult13 struct {
	status string
	raw    json.RawMessage
	err    error
}

func command13(ctx context.Context, plugin *sdk.Plugin, text string) commandResult13 {
	var raw json.RawMessage
	status, err := Dispatch(ctx, plugin, text, &raw)
	return commandResult13{status: status, raw: raw, err: err}
}

func decodeJSON13(raw json.RawMessage, value any) error {
	if err := json.Unmarshal(raw, value); err == nil {
		return nil
	}
	var encoded string
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return err
	}
	return json.Unmarshal([]byte(encoded), value)
}

func (r commandResult13) object() map[string]any {
	if r.err != nil || len(r.raw) == 0 {
		return nil
	}
	var out map[string]any
	if decodeJSON13(r.raw, &out) != nil {
		return nil
	}
	return out
}

func (r commandResult13) text() string {
	var decoded string
	if json.Unmarshal(r.raw, &decoded) == nil {
		return decoded
	}
	return string(r.raw)
}

func requireStatus13(label string, result commandResult13, want string) error {
	if result.status != want {
		return fmt.Errorf("%s: status=%s want %s: %.200s: %w", label, result.status, want, result.raw, result.err)
	}
	if result.err != nil && want != statusError {
		return fmt.Errorf("%s: %w", label, result.err)
	}
	return nil
}

func pollCommand13(ctx context.Context, plugin *sdk.Plugin, command string, attempts int, delay time.Duration, accept func(commandResult13) bool) commandResult13 {
	var result commandResult13
	Poll(ctx, attempts, delay, func() bool {
		result = command13(ctx, plugin, command)
		return accept(result)
	})
	return result
}

func done13(result commandResult13) bool { return result.err == nil && result.status == statusDone }

func number13(value any) int {
	switch n := value.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		value, _ := n.Int64()
		return int(value)
	default:
		return 0
	}
}

func peerRows13(result commandResult13) map[string]any {
	if !done13(result) {
		return nil
	}
	rows, _ := result.object()["peers"].(map[string]any)
	return rows
}

func peerField13(result commandResult13, field string) (any, bool) {
	for _, value := range peerRows13(result) {
		row, ok := value.(map[string]any)
		if !ok {
			continue
		}
		value, ok := row[field]
		return value, ok
	}
	return nil, false
}

func peerCounter13(result commandResult13, field string) int {
	total := 0
	for _, value := range peerRows13(result) {
		if row, ok := value.(map[string]any); ok {
			total += number13(row[field])
		}
	}
	return total
}

// eorSentExpected13 is the number of peers these fixtures wait to see send EOR.
const eorSentExpected13 = 1

func waitEORSent13(ctx context.Context, plugin *sdk.Plugin, selector string) error {
	result := pollCommand13(ctx, plugin, "show bgp peer "+selector+" detail", 40, 250*time.Millisecond, func(result commandResult13) bool {
		count := 0
		for _, value := range peerRows13(result) {
			if row, ok := value.(map[string]any); ok && number13(row["eor-sent"]) >= 1 {
				count++
			}
		}
		return count >= eorSentExpected13
	})
	count := 0
	for _, value := range peerRows13(result) {
		if row, ok := value.(map[string]any); ok && number13(row["eor-sent"]) >= 1 {
			count++
		}
	}
	if count < eorSentExpected13 {
		return fmt.Errorf("ze never sent its End-of-RIB to %s", selector)
	}
	return nil
}

func observe13(ctx context.Context, name string, scenario ObserverScenario) error {
	return Observe(ctx, name, sdk.Registration{}, scenario)
}

func runPlugin13(ctx context.Context, name string, setup func(*sdk.Plugin), scenario func(context.Context, *sdk.Plugin) error, shutdown bool) error {
	plugin, err := newObserver(name)
	if err != nil {
		return err
	}
	defer plugin.Close() //nolint:errcheck // fixture teardown, so a close failure changes no assertion
	if setup != nil {
		setup(plugin)
	}
	result := make(chan error, 1)
	plugin.OnAllPluginsReady(func() error {
		go func() {
			err := scenario(ctx, plugin)
			result <- err
			if shutdown || err != nil {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_, _, _ = plugin.DispatchCommand(shutdownCtx, "request shutdown")
			}
		}()
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

func passivePlugin13(name string) Driver {
	return func(ctx context.Context, args []string) error {
		if len(args) != 0 {
			return fmt.Errorf("%s takes no arguments", name)
		}
		plugin, err := newObserver(name)
		if err != nil {
			return err
		}
		defer plugin.Close() //nolint:errcheck // fixture teardown, so a close failure changes no assertion
		return plugin.Run(ctx, sdk.Registration{})
	}
}

func reloadListenerRejected13(ctx context.Context, args []string) error {
	return observe13(ctx, "reload-listener-rejected-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		readPort := func() (int, error) {
			result := pollCommand13(ctx, plugin, "show l2tp listeners", 40, 100*time.Millisecond, done13)
			if err := requireStatus13("show l2tp listeners", result, "done"); err != nil {
				return 0, err
			}
			var listeners []map[string]any
			if err := decodeJSON13(result.raw, &listeners); err != nil {
				return 0, err
			}
			if len(listeners) != 1 {
				return 0, fmt.Errorf("listener count=%d want 1", len(listeners))
			}
			return number13(listeners[0]["port"]), nil
		}
		readGeneration := func() int {
			result := command13(ctx, plugin, "show reload-status")
			if !done13(result) {
				return -1
			}
			generation, ok := result.object()["generation"]
			if !ok {
				return -1
			}
			return number13(generation)
		}
		initial, err := readPort()
		if err != nil || initial == 0 {
			return fmt.Errorf("before SIGHUP: bad initial port=%d: %w", initial, err)
		}
		baseline := -1
		Poll(ctx, 40, 100*time.Millisecond, func() bool { baseline = readGeneration(); return baseline >= 0 })
		if baseline < 0 {
			return errors.New("before SIGHUP: reload generation unavailable")
		}
		if err := os.WriteFile("observer.initial-ok", []byte("ok"), 0o600); err != nil {
			return err
		}
		if !Poll(ctx, 100, 100*time.Millisecond, func() bool { _, err := os.Stat("reload.done"); return err == nil }) {
			return errors.New("trigger did not create reload.done")
		}
		generation := baseline
		Poll(ctx, 100, 100*time.Millisecond, func() bool { generation = readGeneration(); return generation > baseline })
		if generation <= baseline {
			return fmt.Errorf("reload generation never advanced past %d", baseline)
		}
		after, err := readPort()
		if err != nil {
			return err
		}
		if after != initial {
			return fmt.Errorf("listener port changed %d->%d; expected no change", initial, after)
		}
		return nil
	})
}

func reloadSharedSecret13(ctx context.Context, args []string) error {
	return observe13(ctx, "reload-shared-secret-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		readSecret := func() string {
			result := command13(ctx, plugin, "show l2tp config")
			if !done13(result) {
				return ""
			}
			secret, _ := result.object()["shared-secret"].(string)
			return secret
		}
		before := ""
		Poll(ctx, 40, 100*time.Millisecond, func() bool { before = readSecret(); return before != "" })
		if before != "<unset>" {
			return fmt.Errorf("before SIGHUP: shared-secret=%q want <unset>", before)
		}
		if err := os.WriteFile("observer.initial-ok", []byte("ok"), 0o600); err != nil {
			return err
		}
		if !Poll(ctx, 100, 100*time.Millisecond, func() bool { _, err := os.Stat("reload.done"); return err == nil }) {
			return errors.New("trigger did not create reload.done")
		}
		after := ""
		Poll(ctx, 50, 100*time.Millisecond, func() bool { after = readSecret(); return after == valueRedacted })
		if after != valueRedacted {
			return fmt.Errorf("after SIGHUP: shared-secret=%q want <set>", after)
		}
		return nil
	})
}

func originatedPrivateAS13(name string) Driver {
	return func(ctx context.Context, args []string) error {
		return runPlugin13(ctx, name, nil, func(ctx context.Context, plugin *sdk.Plugin) error {
			if _, _, err := plugin.UpdateRoute(ctx, "*", "update text origin igp as-path [64496 64512 64497] nhop 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24"); err != nil {
				return err
			}
			if err := requireStatus13("wait for update acknowledgement", command13(ctx, plugin, "request quiesce"), "done"); err != nil {
				return err
			}
			if strings.Contains(name, "replace") {
				fmt.Fprintln(os.Stderr, "INFO: originated route carrying private ASN 64512")
			}
			return nil
		}, false)
	}
}

func forwardedPrivateAS13(name, peer, adjective string) Driver {
	return func(ctx context.Context, args []string) error {
		return observe13(ctx, name, func(ctx context.Context, plugin *sdk.Plugin) error {
			result := pollCommand13(ctx, plugin, "show bgp peer "+peer+" detail", 150, 100*time.Millisecond, func(result commandResult13) bool {
				return peerCounter13(result, "updates-sent")-peerCounter13(result, "eor-sent") >= 1
			})
			if peerCounter13(result, "updates-sent")-peerCounter13(result, "eor-sent") < 1 {
				return fmt.Errorf("ze never forwarded the %s route to %s", adjective, peer)
			}
			return nil
		})
	}
}

func removePrivateASImport13(ctx context.Context, args []string) error {
	return observe13(ctx, "test-remove-private-as", func(ctx context.Context, plugin *sdk.Plugin) error {
		result := pollCommand13(ctx, plugin, "show bgp adj-rib-in status", 20, 250*time.Millisecond, func(result commandResult13) bool {
			return done13(result) && number13(result.object()["total-routes"]) >= 1
		})
		if number13(result.object()["total-routes"]) < 1 {
			return errors.New("adj-rib-in never received a route")
		}
		return nil
	})
}

func resolvePing13(ctx context.Context, args []string) error {
	return observe13(ctx, "ping-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := waitEORSent13(ctx, plugin, "peer1"); err != nil {
			return err
		}
		result := command13(ctx, plugin, "resolve ping 127.0.0.1")
		if err := requireStatus13("resolve ping", result, "done"); err != nil {
			return err
		}
		data := result.object()
		if data["destination"] != addrLoopback {
			return fmt.Errorf("expected destination 127.0.0.1, got %v", data["destination"])
		}
		if number13(data["sent"]) <= 0 || number13(data["received"]) <= 0 {
			return fmt.Errorf("ping sent or received no packets: %s", result.raw)
		}
		if loss, ok := data["loss-percent"].(float64); !ok || loss != 0 {
			return fmt.Errorf("loopback ping lost packets: %s", result.raw)
		}
		if replies, ok := data["replies"].([]any); !ok || len(replies) == 0 {
			return fmt.Errorf("ping reported no reply detail: %s", result.raw)
		}
		if _, ok := data["avg-rtt-ms"]; !ok {
			return fmt.Errorf("ping recorded replies but no RTT summary: %s", result.raw)
		}
		fmt.Fprintf(os.Stderr, "OK: resolve ping 127.0.0.1 sent=%d received=%d\n", number13(data["sent"]), number13(data["received"]))
		return nil
	})
}

func replayIdleTimeout13() time.Duration {
	for _, key := range []string{envTestBudgetDotted, envTestBudgetLower, envTestBudgetUpper} {
		if raw := os.Getenv(key); raw != "" {
			if budget, err := time.ParseDuration(raw); err == nil {
				return budget * 60 / 100
			}
		}
	}
	return 30 * time.Second
}

func emptyNLRI13(value any) bool {
	switch value := value.(type) {
	case nil:
		return true
	case []any:
		return len(value) == 0
	case map[string]any:
		return len(value) == 0
	case string:
		return value == ""
	default:
		return false
	}
}

func routeServerReplay13(name, prefix string) Driver {
	return func(ctx context.Context, args []string) error {
		events := make(chan string, 128)
		setup := func(plugin *sdk.Plugin) {
			plugin.SetStartupSubscriptions([]string{eventUpdate}, nil, "parsed")
			plugin.OnEvent(func(event string) error {
				select {
				case events <- event:
				default:
					return errors.New("route-server observer event queue full")
				}
				return nil
			})
		}
		return runPlugin13(ctx, name, setup, func(ctx context.Context, plugin *sdk.Plugin) error {
			eorPeers := make(map[string]struct{})
			forwardSeen := prefix == ""
			idleTimeout := replayIdleTimeout13()
			idle := time.NewTimer(idleTimeout)
			defer idle.Stop()
			statusTicker := time.NewTicker(250 * time.Millisecond)
			defer statusTicker.Stop()
			for len(eorPeers) < 2 || !forwardSeen {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-idle.C:
					return fmt.Errorf("route server did not replay (eor peers=%d, prefix=%q seen=%v)", len(eorPeers), prefix, forwardSeen)
				case <-statusTicker.C:
					status := command13(ctx, plugin, "show bgp peer * detail")
					for address, value := range peerRows13(status) {
						row, ok := value.(map[string]any)
						if ok && number13(row["eor-sent"]) >= 1 {
							eorPeers[address] = struct{}{}
						}
					}
				case event := <-events:
					var root map[string]any
					if json.Unmarshal([]byte(event), &root) != nil {
						continue
					}
					bgp, _ := root["bgp"].(map[string]any)
					message, _ := bgp["message"].(map[string]any)
					if message["direction"] != directionSent {
						continue
					}
					update, _ := bgp["update"].(map[string]any)
					nlri, hasNLRI := update["nlri"]
					if !hasNLRI || emptyNLRI13(nlri) {
						peer, _ := bgp["peer"].(map[string]any)
						remote, _ := peer["remote"].(map[string]any)
						if address, _ := remote["address"].(string); address != "" {
							eorPeers[address] = struct{}{}
						}
					} else if strings.Contains(event, prefix) {
						forwardSeen = true
					}
					if !idle.Stop() {
						select {
						case <-idle.C:
						default:
						}
					}
					idle.Reset(idleTimeout)
				}
			}
			return nil
		}, true)
	}
}

func rfc7606Withdraw13(ctx context.Context, args []string) error {
	return runPlugin13(ctx, "rfc7606-test", func(plugin *sdk.Plugin) {
		plugin.SetStartupSubscriptions([]string{eventState}, nil, "parsed")
	}, func(ctx context.Context, plugin *sdk.Plugin) error {
		pollCommand13(ctx, plugin, "show bgp peer peer1 detail", 20, 250*time.Millisecond, func(result commandResult13) bool {
			state, _ := peerField13(result, "state")
			return state == stateEstablished
		})
		if _, _, err := plugin.UpdateRoute(ctx, "*", "update text nhop 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24"); err != nil {
			return err
		}
		result := pollCommand13(ctx, plugin, "show bgp peer peer1 detail", 100, 100*time.Millisecond, func(result commandResult13) bool {
			return peerCounter13(result, "updates-received") >= 1
		})
		if peerCounter13(result, "updates-received") < 1 {
			return errors.New("ze never counted the post-malformed EOR fence")
		}
		_, _, err := plugin.UpdateRoute(ctx, "*", "update text nhop 10.0.0.1 nlri ipv4/unicast add 10.0.1.0/24")
		return err
	}, false)
}

func opaqueWithdraw13(ctx context.Context, args []string) error {
	const source, receiver = "127.0.0.1", "127.0.0.2"
	return runPlugin13(ctx, "bgpls-withdraw", nil, func(ctx context.Context, plugin *sdk.Plugin) error {
		rsPeersUp := func() map[string]bool {
			result := command13(ctx, plugin, "show bgp rs peers")
			up := make(map[string]bool)
			if !done13(result) {
				return up
			}
			rows, _ := result.object()["peers"].([]any)
			for _, value := range rows {
				row, _ := value.(map[string]any)
				address, _ := row["address"].(string)
				isUp, _ := row["up"].(bool)
				if isUp {
					up[address] = true
				}
			}
			return up
		}
		receiverUpdates := func() int {
			result := command13(ctx, plugin, "show bgp peer "+receiver+" detail")
			return peerCounter13(result, "updates-sent") - peerCounter13(result, "eor-sent")
		}
		both := false
		Poll(ctx, 100, 250*time.Millisecond, func() bool { up := rsPeersUp(); both = up[source] && up[receiver]; return both })
		if !both {
			return errors.New("route server never reported both clients up")
		}
		if _, _, err := plugin.UpdateRoute(ctx, source, "update text origin igp nhop 1.1.1.1 nlri ipv4/unicast add 192.0.2.0/24"); err != nil {
			return err
		}
		if err := requireStatus13("wait for BGP-LS announcement", command13(ctx, plugin, "request quiesce"), "done"); err != nil {
			return err
		}
		sent := false
		Poll(ctx, 60, 250*time.Millisecond, func() bool { sent = receiverUpdates() >= 1; return sent })
		if !sent {
			return errors.New("receiver was never sent the BGP-LS announcement")
		}
		before := receiverUpdates()
		if _, _, err := plugin.UpdateRoute(ctx, source, "update text origin igp nhop 1.1.1.1 nlri ipv4/unicast add 198.51.100.0/24"); err != nil {
			return err
		}
		withdrawn := false
		Poll(ctx, 60, 250*time.Millisecond, func() bool { withdrawn = receiverUpdates() > before; return withdrawn })
		if !withdrawn {
			return errors.New("receiver was never sent the BGP-LS withdrawal")
		}
		return nil
	}, true)
}

func reloadTrigger13(ctx context.Context, args []string) error {
	for _, marker := range []string{fileDaemonPID, fileDaemonReady, "observer.initial-ok"} {
		if !Poll(ctx, 1_000, 100*time.Millisecond, func() bool {
			_, err := os.Stat(marker)
			return err == nil
		}) {
			return fmt.Errorf("wait for %s: %w", marker, ctx.Err())
		}
	}
	config, err := os.ReadFile("config2.conf")
	if err != nil {
		return fmt.Errorf("read replacement config: %w", err)
	}
	if err := os.WriteFile("ze-bgp.conf", config, 0o600); err != nil {
		return fmt.Errorf("replace live config: %w", err)
	}
	rawPID, err := os.ReadFile("daemon.pid")
	if err != nil {
		return fmt.Errorf("read daemon pid: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(rawPID)))
	if err != nil {
		return fmt.Errorf("parse daemon pid: %w", err)
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find daemon process: %w", err)
	}
	if err := process.Signal(syscall.SIGHUP); err != nil {
		return fmt.Errorf("signal daemon reload: %w", err)
	}
	if err := os.WriteFile("reload.done", nil, 0o600); err != nil {
		return fmt.Errorf("write reload marker: %w", err)
	}
	return nil
}
