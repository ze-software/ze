package fixture

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func plugin15RPKIPassthrough(ctx context.Context, p *sdk.Plugin) error {
	if err := plugin15RPKIReady(ctx, p, false); err != nil {
		return err
	}
	r := plugin15PollCommand(ctx, p, "show bgp adj-rib-in", 40, 250*time.Millisecond, func(r plugin15Result) bool {
		return plugin15Done(r) && strings.Contains(r.text(), "0.0.0.0")
	})
	if !plugin15Done(r) {
		return fmt.Errorf("rib routes received status=%s: %s", r.status, r.text())
	}
	text := r.text()
	if !strings.Contains(text, "0.0.0.0") {
		return fmt.Errorf("default route not in adj-rib-in: %s", text)
	}
	if strings.Contains(text, `"validation-state":4`) || strings.Contains(text, `"validation-state": 4`) {
		return fmt.Errorf("route stuck in pending state without RPKI plugin: %s", text)
	}
	fmt.Fprintln(os.Stderr, "OK: route stored immediately without RPKI (passthrough)")
	return nil
}

func plugin15RPKICount(ctx context.Context, p *sdk.Plugin) float64 {
	r := plugin15Dispatch(ctx, p, "show bgp rpki status")
	m, _ := plugin15Map(r)
	return plugin15Number(m["vrp-count-ipv4"])
}

func plugin15UpdatesReceived(ctx context.Context, p *sdk.Plugin) float64 {
	r := plugin15Dispatch(ctx, p, "show bgp peer peer1 detail")
	m, _ := plugin15Map(r)
	peer := plugin15NestedMap(m, "peers", "127.0.0.1")
	return plugin15Number(peer["updates-received"])
}

func plugin15RPKIReady(ctx context.Context, p *sdk.Plugin, requireVRP bool) error {
	if !Poll(ctx, 40, 250*time.Millisecond, func() bool {
		if plugin15UpdatesReceived(ctx, p) < 1 {
			return false
		}
		return !requireVRP || plugin15RPKICount(ctx, p) >= 1
	}) {
		if requireVRP {
			return fmt.Errorf("the peer UPDATE and RTR cache never both became ready")
		}
		return fmt.Errorf("ze never received the peer UPDATE")
	}
	if r := plugin15Dispatch(ctx, p, "request quiesce"); !plugin15Done(r) {
		return fmt.Errorf("quiesce failed: status=%s data=%s", r.status, r.text())
	}
	return nil
}

func plugin15RPKIPerPeer(ctx context.Context, p *sdk.Plugin) error {
	if !Poll(ctx, 150, 200*time.Millisecond, func() bool { return plugin15RPKICount(ctx, p) >= 1 }) {
		return fmt.Errorf("RTR never synced a VRP")
	}
	if !Poll(ctx, 150, 200*time.Millisecond, func() bool { return plugin15UpdatesReceived(ctx, p) >= 1 }) {
		return fmt.Errorf("ze never received the UPDATE")
	}
	if r := plugin15Dispatch(ctx, p, "request quiesce"); !plugin15Done(r) {
		return fmt.Errorf("quiesce failed: %s", r.text())
	}
	rib := plugin15Dispatch(ctx, p, "show bgp adj-rib-in")
	if !plugin15Done(rib) || !strings.Contains(rib.text(), "10.0.1.0/24") {
		return fmt.Errorf("Invalid route 10.0.1.0/24 should be ACCEPTED via per-peer override: status=%s data=%s", rib.status, rib.text())
	}
	status, err := plugin15Map(plugin15Dispatch(ctx, p, "show bgp rpki status"))
	if err != nil {
		return err
	}
	rows, _ := status["peer-actions"].([]any)
	var entry map[string]any
	for _, row := range rows {
		m, _ := row.(map[string]any)
		if m["peer"] == "127.0.0.1" {
			entry = m
			break
		}
	}
	if entry == nil {
		return fmt.Errorf("peer-actions missing 127.0.0.1: %v", rows)
	}
	invalid, _ := entry["invalid"].(map[string]any)
	if invalid["action"] != "accept" || invalid["source"] != "peer" {
		return fmt.Errorf("peer 127.0.0.1 invalid action/source wrong: %v", invalid)
	}
	notFound, _ := entry["not-found"].(map[string]any)
	if notFound["source"] != "global" {
		return fmt.Errorf("peer 127.0.0.1 not-found should inherit global: %v", notFound)
	}
	fmt.Fprintln(os.Stderr, "OK: Invalid route accepted via per-peer override; status reports peer-actions")
	return nil
}

func plugin15AdjRIBTotal(ctx context.Context, p *sdk.Plugin) float64 {
	r := plugin15Dispatch(ctx, p, "show bgp adj-rib-in status")
	if !plugin15Done(r) {
		return -1
	}
	m, _ := plugin15Map(r)
	return plugin15Number(m["total-routes"])
}

func plugin15RPKILateSync(ctx context.Context, p *sdk.Plugin) error {
	if !Poll(ctx, 24, 500*time.Millisecond, func() bool { return plugin15UpdatesReceived(ctx, p) >= 1 }) {
		return fmt.Errorf("ze never received the UPDATE")
	}
	if !Poll(ctx, 24, 500*time.Millisecond, func() bool { return plugin15AdjRIBTotal(ctx, p) >= 1 }) {
		return fmt.Errorf("route 10.0.1.0/24 was never installed as NotFound")
	}
	if plugin15RPKICount(ctx, p) != 0 {
		return fmt.Errorf("the cache synced before the UPDATE arrived, so this run does not exercise re-validation")
	}
	if !Poll(ctx, 24, 500*time.Millisecond, func() bool { return plugin15RPKICount(ctx, p) >= 1 }) {
		return fmt.Errorf("the RTR cache never synced a VRP")
	}
	if !Poll(ctx, 24, 500*time.Millisecond, func() bool { return plugin15AdjRIBTotal(ctx, p) == 0 }) {
		return fmt.Errorf("route 10.0.1.0/24 stayed in the Adj-RIB-In after the VRP made it Invalid")
	}
	fmt.Fprintln(os.Stderr, "OK re-validation removed 10.0.1.0/24 once the cache synced")
	return nil
}

func plugin15RPKITimeout(ctx context.Context, p *sdk.Plugin) error {
	if err := plugin15RPKIReady(ctx, p, false); err != nil {
		return err
	}
	r := plugin15PollCommand(ctx, p, "show bgp adj-rib-in", 60, 500*time.Millisecond, func(r plugin15Result) bool {
		text := r.text()
		return plugin15Done(r) && strings.Contains(text, "10.0.1.0/24") && !strings.Contains(text, `"validation-state":4`) && !strings.Contains(text, `"validation-state": 4`)
	})
	if !plugin15Done(r) {
		return fmt.Errorf("rib routes received status=%s: %s", r.status, r.text())
	}
	text := r.text()
	if !strings.Contains(text, "10.0.1.0/24") {
		return fmt.Errorf("route 10.0.1.0/24 not promoted after timeout: %s", text)
	}
	if strings.Contains(text, `"validation-state":4`) || strings.Contains(text, `"validation-state": 4`) {
		return fmt.Errorf("route 10.0.1.0/24 still pending (state 4) after timeout: %s", text)
	}
	fmt.Fprintln(os.Stderr, "OK: route 10.0.1.0/24 promoted after timeout (fail-open)")
	return nil
}

func plugin15RPKIRouteState(ctx context.Context, p *sdk.Plugin, prefix string, state int, success string) error {
	if err := plugin15RPKIReady(ctx, p, true); err != nil {
		return err
	}
	needleA := fmt.Sprintf(`"validation-state":%d`, state)
	needleB := fmt.Sprintf(`"validation-state": %d`, state)
	r := plugin15PollCommand(ctx, p, "show bgp adj-rib-in", 40, 250*time.Millisecond, func(r plugin15Result) bool {
		text := r.text()
		return plugin15Done(r) && strings.Contains(text, prefix) && (strings.Contains(text, needleA) || strings.Contains(text, needleB))
	})
	if !plugin15Done(r) {
		return fmt.Errorf("rib routes received status=%s: %s", r.status, r.text())
	}
	text := r.text()
	if !strings.Contains(text, prefix) {
		return fmt.Errorf("route %s not in adj-rib-in: %s", prefix, text)
	}
	if !strings.Contains(text, needleA) && !strings.Contains(text, needleB) {
		return fmt.Errorf("validation-state not %d: %s", state, text)
	}
	fmt.Fprintln(os.Stderr, success)
	return nil
}

func plugin15RPKIAccept(ctx context.Context, p *sdk.Plugin) error {
	return plugin15RPKIRouteState(ctx, p, "10.0.1.0/24", 1, "OK: route 10.0.1.0/24 accepted with validation-state=Valid")
}

func plugin15RPKINotFound(ctx context.Context, p *sdk.Plugin) error {
	return plugin15RPKIRouteState(ctx, p, "192.168.0.0/24", 2, "OK: route 192.168.0.0/24 accepted with validation-state=NotFound")
}

func plugin15RPKIBatch(ctx context.Context, p *sdk.Plugin) error {
	if err := plugin15RPKIReady(ctx, p, true); err != nil {
		return err
	}
	r := plugin15PollCommand(ctx, p, "show bgp adj-rib-in", 40, 250*time.Millisecond, func(r plugin15Result) bool {
		text := r.text()
		return plugin15Done(r) && strings.Contains(text, "10.0.1.0/24") && strings.Contains(text, "10.0.2.0/24") && strings.Contains(text, "192.168.1.0/24")
	})
	if !plugin15Done(r) {
		return fmt.Errorf("rib query failed: status=%s data=%s", r.status, r.text())
	}
	text := r.text()
	for _, prefix := range []string{"10.0.1.0/24", "10.0.2.0/24", "192.168.1.0/24"} {
		if !strings.Contains(text, prefix) {
			return fmt.Errorf("accepted route %s not in adj-rib-in: %s", prefix, text)
		}
	}
	if strings.Contains(text, "10.0.3.0/24") {
		return fmt.Errorf("10.0.3.0/24 (Invalid) should have been rejected: %s", text)
	}
	fmt.Fprintln(os.Stderr, "OK: batch validation correct: 2 Valid accepted, 1 Invalid rejected, 1 NotFound accepted")
	return nil
}

func plugin15RPKIReject(ctx context.Context, p *sdk.Plugin) error {
	if !Poll(ctx, 150, 200*time.Millisecond, func() bool { return plugin15RPKICount(ctx, p) >= 1 }) {
		return fmt.Errorf("RTR never synced a VRP")
	}
	if !Poll(ctx, 150, 200*time.Millisecond, func() bool { return plugin15UpdatesReceived(ctx, p) >= 1 }) {
		return fmt.Errorf("ze never received the UPDATE")
	}
	if r := plugin15Dispatch(ctx, p, "request quiesce"); !plugin15Done(r) {
		return fmt.Errorf("quiesce failed: %s", r.text())
	}
	r := plugin15Dispatch(ctx, p, "show bgp adj-rib-in")
	if !plugin15Done(r) {
		return fmt.Errorf("rib routes received status=%s: %s", r.status, r.text())
	}
	if strings.Contains(r.text(), "10.0.1.0/24") {
		return fmt.Errorf("route 10.0.1.0/24 should have been rejected: %s", r.text())
	}
	fmt.Fprintln(os.Stderr, "OK: route 10.0.1.0/24 correctly rejected (Invalid)")
	return nil
}
