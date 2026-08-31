package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/api-rib-inject", observer02("rib-inject-test", apiRIBInject02))
	Register("plugin/api-rib-out-clear", observer02("rib-out-clear-test", apiRIBOutClear02))
	Register("plugin/api-rib-out-show", observer02("rib-out-show-test", apiRIBOutShow02))
	Register("plugin/api-rib-withdraw", observer02("rib-withdraw-test", apiRIBWithdraw02))
	Register("plugin/api-route-refresh", observer02("route-refresh-test", apiRouteRefresh02))
	Register("plugin/api-subscribe", observer02("subscribe-test", apiSubscribe02))
	Register("plugin/api-unsubscribe", observer02("unsubscribe-test", apiUnsubscribe02))
	Register("plugin/as112-disable", observer02("as112-disable-test", as112Disable02))
	Register("plugin/as112-doh", observer02("as112-doh-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		return as112Enabled02(ctx, plugin, "127.0.0.1:18443", 20, "doh-bound")
	}))
	Register("plugin/as112-dot-pki", observer02("as112-dot-pki-test", as112DotPKI02))
	Register("plugin/as112-dot", observer02("as112-dot-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		return as112Enabled02(ctx, plugin, "127.0.0.1:18853", 20, "dot-bound")
	}))
	Register("plugin/as112-enable", observer02("as112-enable-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		return as112Enabled02(ctx, plugin, "", 0, "")
	}))
	Register("plugin/as112-health", observer02("as112-health-test", as112Health02))
	Register("plugin/asn4-transcode-pooled-buffer", routeServerObserver02("shutdown-after-up", 2, "10.0.0.0/24"))
	Register("plugin/aspath-filter-accept", observer02("test-aspath-accept", adjRIBRouteObserver02))
	Register("plugin/aspath-filter-chain", observer02("test-aspath-chain", adjRIBRouteObserver02))
	Register("plugin/aspath-filter-reject", observer02("test-aspath-reject", updateReceivedObserver02))
	Register("plugin/aspath-filter-shortform", observer02("test-aspath-short", adjRIBRouteObserver02))
	Register("plugin/aspath-length-reject", observer02("test-aspath-len", updateReceivedObserver02))
	Register("plugin/attach-process-delivery-graph", observer02("delivery-graph-test", deliveryGraph02))
	Register("plugin/authz-rpc-identity", observer02("rpc-dispatcher", authzRPCIdentity02))
	Register("plugin/bestpath-reason", observer02("reason-test", bestpathReason02))
	Register("plugin/bfd-auth-sha1", observer02("bfd-auth-sha1-test", bfdAuthSHA102))
	Register("plugin/bfd-auth-simple-password", observer02("bfd-auth-simple-password-test", bfdAuthSimplePassword02))
}

func observer02(name string, scenario ObserverScenario) Driver {
	return func(ctx context.Context, args []string) error {
		if len(args) != 0 {
			return fmt.Errorf("%s takes no arguments", name)
		}
		return Observe(ctx, name, sdk.Registration{}, scenario)
	}
}

func command02(ctx context.Context, plugin *sdk.Plugin, command string) (string, json.RawMessage, error) {
	status, data, err := plugin.DispatchCommand(ctx, command)
	if err != nil {
		if status == statusError || strings.HasPrefix(err.Error(), "rpc error:") {
			encoded, marshalErr := json.Marshal(err.Error())
			return statusError, encoded, marshalErr
		}
		return status, nil, err
	}
	return status, data, nil
}

func requireDone02(ctx context.Context, plugin *sdk.Plugin, command string) (json.RawMessage, error) {
	status, data, err := command02(ctx, plugin, command)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", command, err)
	}
	if status != statusDone {
		return nil, fmt.Errorf("%s: status=%s data=%s", command, status, data)
	}
	return data, nil
}

func decode02(raw json.RawMessage, out any) error {
	if len(raw) == 0 {
		return nil
	}
	if raw[0] == '"' {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return err
		}
		raw = json.RawMessage(text)
	}
	return json.Unmarshal(raw, out)
}

func text02(raw json.RawMessage) string {
	if len(raw) != 0 && raw[0] == '"' {
		var text string
		if json.Unmarshal(raw, &text) == nil {
			return text
		}
	}
	return string(raw)
}

func map02(raw json.RawMessage) (map[string]any, error) {
	out := make(map[string]any)
	if err := decode02(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func int02(value any) int {
	switch value := value.(type) {
	case float64:
		return int(value)
	case int:
		return value
	case json.Number:
		n, _ := value.Int64()
		return int(n)
	default:
		return 0
	}
}

func eorSent02(ctx context.Context, plugin *sdk.Plugin, selector string, expected, attempts int) bool {
	return Poll(ctx, attempts, 250*time.Millisecond, func() bool {
		raw, err := requireDone02(ctx, plugin, "show bgp peer "+selector+" detail")
		if err != nil {
			return false
		}
		data, err := map02(raw)
		if err != nil {
			return false
		}
		peers, _ := data["peers"].(map[string]any)
		ready := 0
		for _, value := range peers {
			row, _ := value.(map[string]any)
			if int02(row["eor-sent"]) >= 1 {
				ready++
			}
		}
		return ready >= expected
	})
}

func quiesce02(ctx context.Context, plugin *sdk.Plugin) error {
	_, err := requireDone02(ctx, plugin, "request quiesce")
	return err
}

func apiRIBReady02(ctx context.Context, plugin *sdk.Plugin) error {
	if !eorSent02(ctx, plugin, "peer1", 1, 40) {
		return fmt.Errorf("peer1 initial-sync EOR never reached the wire")
	}
	return quiesce02(ctx, plugin)
}

func apiRIBInject02(ctx context.Context, plugin *sdk.Plugin) error {
	if err := apiRIBReady02(ctx, plugin); err != nil {
		return err
	}
	data, err := requireDone02(ctx, plugin, "request bgp rib inject 10.0.0.99 ipv4/unicast 192.168.1.0/24 origin igp aspath 64500,64501")
	if err != nil {
		return err
	}
	if !strings.Contains(text02(data), "192.168.1.0/24") {
		return fmt.Errorf("inject response missing prefix: %s", text02(data))
	}
	data, err = requireDone02(ctx, plugin, "show bgp rib received")
	if err != nil {
		return err
	}
	if !strings.Contains(text02(data), "192.168.1.0/24") {
		return fmt.Errorf("injected route not in rib show: %.200s", text02(data))
	}
	fmt.Fprintln(os.Stderr, "OK: rib inject + show verified")
	return nil
}

func apiRIBOutClear02(ctx context.Context, plugin *sdk.Plugin) error {
	if err := apiRIBReady02(ctx, plugin); err != nil {
		return err
	}
	if _, err := requireDone02(ctx, plugin, "clear bgp rib out *"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: rib clear out * dispatched")
	return nil
}

func apiRIBOutShow02(ctx context.Context, plugin *sdk.Plugin) error {
	if err := apiRIBReady02(ctx, plugin); err != nil {
		return err
	}
	if _, err := requireDone02(ctx, plugin, "show bgp rib sent"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: show bgp rib sent dispatched")
	return nil
}

func apiRIBWithdraw02(ctx context.Context, plugin *sdk.Plugin) error {
	if err := apiRIBReady02(ctx, plugin); err != nil {
		return err
	}
	if _, err := requireDone02(ctx, plugin, "request bgp rib inject 10.0.0.99 ipv4/unicast 192.168.1.0/24 origin igp"); err != nil {
		return err
	}
	data, err := requireDone02(ctx, plugin, "request bgp rib withdraw 10.0.0.99 ipv4/unicast 192.168.1.0/24")
	if err != nil {
		return err
	}
	if !strings.Contains(text02(data), `"existed":true`) {
		return fmt.Errorf("withdraw should report existed=true: %s", text02(data))
	}
	data, err = requireDone02(ctx, plugin, "show bgp rib received")
	if err != nil {
		return err
	}
	if strings.Contains(text02(data), "192.168.1.0/24") {
		return fmt.Errorf("withdrawn route still in rib show: %.200s", text02(data))
	}
	fmt.Fprintln(os.Stderr, "OK: rib inject + withdraw + show verified")
	return nil
}

func apiRouteRefresh02(ctx context.Context, plugin *sdk.Plugin) error {
	if err := apiRIBReady02(ctx, plugin); err != nil {
		return err
	}
	if _, err := requireDone02(ctx, plugin, "request peer peer1 refresh ipv4/unicast"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: route-refresh sent")
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Second):
		return nil
	}
}

func apiSubscribe02(ctx context.Context, plugin *sdk.Plugin) error {
	if err := apiRIBReady02(ctx, plugin); err != nil {
		return err
	}
	raw, err := requireDone02(ctx, plugin, "request subscribe bgp event update")
	if err != nil {
		return err
	}
	data, err := map02(raw)
	if err != nil {
		return err
	}
	if data["namespace"] != namespaceBGP || data["event"] != eventUpdate {
		return fmt.Errorf("expected bgp/update, got %v/%v", data["namespace"], data["event"])
	}
	fmt.Fprintln(os.Stderr, "OK: subscribed to bgp event update")
	return nil
}

func apiUnsubscribe02(ctx context.Context, plugin *sdk.Plugin) error {
	if err := apiRIBReady02(ctx, plugin); err != nil {
		return err
	}
	if _, err := requireDone02(ctx, plugin, "request subscribe bgp event update"); err != nil {
		return err
	}
	raw, err := requireDone02(ctx, plugin, "request unsubscribe bgp event update")
	if err != nil {
		return err
	}
	data, err := map02(raw)
	if err != nil {
		return err
	}
	if data["namespace"] != namespaceBGP {
		return fmt.Errorf("expected namespace bgp, got %v", data["namespace"])
	}
	fmt.Fprintln(os.Stderr, "OK: subscribe+unsubscribe verified")
	return nil
}

func as112Show02(ctx context.Context, plugin *sdk.Plugin) (map[string]any, error) {
	raw, err := requireDone02(ctx, plugin, "show as112")
	if err != nil {
		return nil, err
	}
	return map02(raw)
}

func as112Disable02(ctx context.Context, plugin *sdk.Plugin) error {
	if err := apiRIBReady02(ctx, plugin); err != nil {
		return err
	}
	data, err := as112Show02(ctx, plugin)
	if err != nil {
		return err
	}
	if enabled, ok := data["enabled"].(bool); !ok || enabled {
		return fmt.Errorf("as112: expected enabled=false, got %v", data)
	}
	fmt.Fprintln(os.Stderr, "OK: as112 enabled=false")
	return nil
}

func listenerBound02(ctx context.Context, address string, attempts int) bool {
	return Poll(ctx, attempts, 0, func() bool {
		dialer := net.Dialer{Timeout: time.Second}
		conn, err := dialer.DialContext(ctx, "tcp", address)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	})
}

func as112Enabled02(ctx context.Context, plugin *sdk.Plugin, address string, attempts int, suffix string) error {
	if err := apiRIBReady02(ctx, plugin); err != nil {
		return err
	}
	data, err := as112Show02(ctx, plugin)
	if err != nil {
		return err
	}
	if data["enabled"] != true {
		return fmt.Errorf("as112: expected enabled=true, got %v", data["enabled"])
	}
	if int02(data["zones"]) != 22 {
		return fmt.Errorf("as112: expected zones=22, got %v", data["zones"])
	}
	if address != "" && !listenerBound02(ctx, address, attempts) {
		return fmt.Errorf("as112 listener not bound on %s", address)
	}
	line := "OK: as112 enabled=true zones=22"
	if suffix != "" {
		line += " " + suffix
	}
	fmt.Fprintln(os.Stderr, line)
	return nil
}

func as112DotPKI02(ctx context.Context, plugin *sdk.Plugin) error {
	if err := apiRIBReady02(ctx, plugin); err != nil {
		return err
	}
	if !listenerBound02(ctx, "127.0.0.1:18854", 40) {
		return fmt.Errorf("dot: listener not bound on 127.0.0.1:18854 (pki certificate reference did not resolve)")
	}
	fmt.Fprintln(os.Stderr, "OK: as112 dot bound with pki certificate")
	return nil
}

func checkAS112Health02(status string, raw json.RawMessage, dispatchErr error, command, target string) (bool, error) {
	if status == statusError {
		reason := text02(raw)
		if dispatchErr != nil {
			reason = dispatchErr.Error()
		}
		if !strings.Contains(reason, target) {
			return false, fmt.Errorf("%s: failed without naming target %q: %q", command, target, reason)
		}
		return false, nil
	}
	if dispatchErr != nil {
		return false, fmt.Errorf("%s: %w", command, dispatchErr)
	}
	if status != statusDone {
		return false, fmt.Errorf("%s: unexpected status=%s", command, status)
	}
	data, err := map02(raw)
	if err != nil {
		return false, err
	}
	if data["healthy"] != true {
		return false, fmt.Errorf("%s: a done answer must carry healthy=true, got %v", command, data)
	}
	if data["target"] != target {
		return false, fmt.Errorf("%s: expected target=%q in response, got %v", command, target, data)
	}
	return true, nil
}

func as112Health02(ctx context.Context, plugin *sdk.Plugin) error {
	const target = "127.0.0.1:53"
	for _, command := range []string{"request as112 healthcheck", "request as112 healthcheck target 127.0.0.1"} {
		status, raw, dispatchErr := command02(ctx, plugin, command)
		healthy, err := checkAS112Health02(status, raw, dispatchErr, command, target)
		if err != nil {
			return err
		}
		if command == "request as112 healthcheck" {
			fmt.Fprintf(os.Stderr, "OK: request as112 healthcheck dispatched, healthy=%t\n", healthy)
		} else {
			fmt.Fprintf(os.Stderr, "OK: request as112 healthcheck target 127.0.0.1 dispatched, target=%s\n", target)
		}
	}
	if !eorSent02(ctx, plugin, "peer1", 1, 40) {
		return fmt.Errorf("peer1 initial-sync EOR never reached the wire")
	}
	return nil
}

func adjRIBRouteObserver02(ctx context.Context, plugin *sdk.Plugin) error {
	Poll(ctx, 20, 250*time.Millisecond, func() bool {
		raw, err := requireDone02(ctx, plugin, "show bgp adj-rib-in status")
		if err != nil {
			return false
		}
		data, err := map02(raw)
		return err == nil && int02(data["total-routes"]) >= 1
	})
	return nil
}

func peerFields02(ctx context.Context, plugin *sdk.Plugin, selector string) (map[string]any, error) {
	raw, err := requireDone02(ctx, plugin, "show bgp peer "+selector+" detail")
	if err != nil {
		return nil, err
	}
	data, err := map02(raw)
	if err != nil {
		return nil, err
	}
	peers, _ := data["peers"].(map[string]any)
	return peers, nil
}

func updateReceivedObserver02(ctx context.Context, plugin *sdk.Plugin) error {
	Poll(ctx, 100, 200*time.Millisecond, func() bool {
		peers, err := peerFields02(ctx, plugin, "peer1")
		if err != nil {
			return false
		}
		row, _ := peers["127.0.0.1"].(map[string]any)
		return int02(row["updates-received"]) >= 1
	})
	return quiesce02(ctx, plugin)
}

func deliveryGraph02(ctx context.Context, plugin *sdk.Plugin) error {
	if !eorSent02(ctx, plugin, "127.0.0.1", 1, 40) {
		return fmt.Errorf("delivery: peer1 never reached its initial-sync end-of-rib")
	}
	raw, err := requireDone02(ctx, plugin, "show event delivery")
	if err != nil {
		return err
	}
	var data struct {
		Peers []struct {
			Peer      string `json:"peer"`
			Processes []struct {
				Process string   `json:"process"`
				Receive []string `json:"receive"`
				Send    []string `json:"send"`
			} `json:"processes"`
		} `json:"peers"`
	}
	if err := decode02(raw, &data); err != nil {
		return err
	}
	peers := make(map[string]struct {
		Peer      string `json:"peer"`
		Processes []struct {
			Process string   `json:"process"`
			Receive []string `json:"receive"`
			Send    []string `json:"send"`
		} `json:"processes"`
	})
	for _, peer := range data.Peers {
		peers[peer.Peer] = peer
	}
	if len(peers) != 3 {
		return fmt.Errorf("delivery: expected 3 peers, got %d", len(peers))
	}
	fed := peers["127.0.0.1"].Processes
	if len(fed) != 1 || fed[0].Process != "delivery-graph-test" || len(fed[0].Receive) != 1 || fed[0].Receive[0] != eventState || len(fed[0].Send) != 0 {
		return fmt.Errorf("delivery: peer1 edges are %v", fed)
	}
	sender := peers["192.0.2.9"].Processes
	if len(sender) != 1 || len(sender[0].Send) != 1 || sender[0].Send[0] != eventUpdate || len(sender[0].Receive) != 0 {
		return fmt.Errorf("delivery: peer2 edges are %v", sender)
	}
	if len(peers["192.0.2.10"].Processes) != 0 {
		return fmt.Errorf("delivery: peer3 attaches nothing but has edges")
	}
	fmt.Fprintln(os.Stderr, "OK: delivery graph published for 3 peers")
	return nil
}

func authzRPCIdentity02(ctx context.Context, plugin *sdk.Plugin) error {
	if !eorSent02(ctx, plugin, "*", 1, 40) {
		return errors.New("ze never sent its initial-sync End-of-Rib")
	}
	if _, err := requireDone02(ctx, plugin, "show version"); err != nil {
		return fmt.Errorf("internal RPC dispatch refused on RBAC box: %w", err)
	}
	fmt.Fprintln(os.Stderr, "OK: internal RPC dispatch authorized on RBAC box")
	return nil
}

func bestpathReason02(ctx context.Context, plugin *sdk.Plugin) error {
	if !eorSent02(ctx, plugin, "peer1", 1, 40) {
		return fmt.Errorf("ze never sent its End-of-RIB to peer1")
	}
	if _, err := requireDone02(ctx, plugin, "request bgp rib inject 10.0.0.99 ipv4/unicast 10.1.0.0/24 origin igp aspath 65002 localpref 100"); err != nil {
		return fmt.Errorf("rib inject failed: %w", err)
	}
	var chosen map[string]any
	if !Poll(ctx, 20, 250*time.Millisecond, func() bool {
		raw, err := requireDone02(ctx, plugin, "show bgp rib best reason")
		if err != nil {
			return false
		}
		data, err := map02(raw)
		if err != nil {
			return false
		}
		entries, _ := data["best-path-reason"].([]any)
		for _, value := range entries {
			entry, _ := value.(map[string]any)
			candidates, _ := entry["candidates"].([]any)
			if entry["prefix"] == prefixTenOne && len(candidates) >= 2 {
				chosen = entry
				return true
			}
		}
		return false
	}) {
		return fmt.Errorf("no reason entry with two candidates for 10.1.0.0/24")
	}
	steps, _ := chosen["steps"].([]any)
	if len(steps) == 0 {
		return fmt.Errorf("no decision steps, candidates: %v", chosen["candidates"])
	}
	step, _ := steps[0].(map[string]any)
	stepName, _ := step["step"].(string)
	reason, _ := step["reason"].(string)
	if !strings.Contains(stepName, "local-preference") {
		return fmt.Errorf("expected local-preference step, got %v", step)
	}
	if !strings.Contains(reason, "200") || !strings.Contains(reason, "100") {
		return fmt.Errorf("reason should mention LP 200 and 100, got %s", reason)
	}
	fmt.Fprintf(os.Stderr, "OK: best-path reason: %s %s\n", stepName, reason)
	if !eorSent02(ctx, plugin, "peer1", 1, 40) {
		return fmt.Errorf("ze never sent its End-of-RIB to peer1")
	}
	return nil
}

// bfdAuthSimplePassword02 checks that a profile carrying
// `auth { type simple-password }` builds a live session and signs the packets
// it sends. RFC 5880 Section 6.7.2 requires the password and Key ID in the
// Authentication Section of EACH outgoing Control packet, so a transmitting
// session is what proves the signer is installed: a password the signer
// refused would leave no session for `show bfd session` to answer with.
func bfdAuthSimplePassword02(ctx context.Context, plugin *sdk.Plugin) error {
	if !eorSent02(ctx, plugin, "*", 1, 40) {
		return errors.New("ze never sent its initial-sync End-of-Rib")
	}
	raw, err := requireDone02(ctx, plugin, "show bfd profile name clear-text")
	if err != nil {
		return err
	}
	profile, err := map02(raw)
	if err != nil {
		return fmt.Errorf("profile name: %v (decode: %w)", profile, err)
	}
	if profile["name"] != "clear-text" {
		return fmt.Errorf("profile name: %v (decode: <nil>)", profile)
	}
	raw, err = requireDone02(ctx, plugin, "show bfd session address 203.0.113.9")
	if err != nil {
		return err
	}
	session, err := map02(raw)
	if err != nil {
		return err
	}
	if session["peer"] != addrTestNet3Nine || session["profile"] != "clear-text" {
		return fmt.Errorf("unexpected session: %v", session)
	}
	if int02(session["tx-packets"]) == 0 {
		return fmt.Errorf("no simple-password signed tx packets recorded: %v", session)
	}
	return nil
}

func bfdAuthSHA102(ctx context.Context, plugin *sdk.Plugin) error {
	if !eorSent02(ctx, plugin, "*", 1, 40) {
		return errors.New("ze never sent its initial-sync End-of-Rib")
	}
	raw, err := requireDone02(ctx, plugin, "show bfd profile name fast-auth")
	if err != nil {
		return err
	}
	profile, err := map02(raw)
	if err != nil {
		return fmt.Errorf("profile name: %v (decode: %w)", profile, err)
	}
	if profile["name"] != "fast-auth" {
		return fmt.Errorf("profile name: %v (decode: <nil>)", profile)
	}
	raw, err = requireDone02(ctx, plugin, "show bfd session address 203.0.113.9")
	if err != nil {
		return err
	}
	session, err := map02(raw)
	if err != nil {
		return err
	}
	if session["peer"] != addrTestNet3Nine || session["profile"] != "fast-auth" {
		return fmt.Errorf("unexpected session: %v", session)
	}
	if int02(session["tx-packets"]) == 0 {
		return fmt.Errorf("no signed tx packets recorded: %v", session)
	}
	return nil
}
