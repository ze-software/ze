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
	m, err := plugin15Map(r)
	if err != nil {
		return -1
	}
	count, ok := m["vrp-count-ipv4"].(float64)
	if !ok {
		return -1
	}
	return count
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
	if err := plugin14AssertRouteValidation(ctx, p, "10.0.1.0/24", 3, false, "per-peer accept override"); err != nil {
		return err
	}
	status, err := plugin15Map(plugin15Dispatch(ctx, p, "show bgp rpki status"))
	if err != nil {
		return err
	}
	rows, _ := status["peer-actions"].([]any)
	var entry map[string]any
	for _, row := range rows {
		m, _ := row.(map[string]any)
		if m["peer"] == addrLoopback {
			entry = m
			break
		}
	}
	if entry == nil {
		return fmt.Errorf("peer-actions missing 127.0.0.1: %v", rows)
	}
	invalid, _ := entry["invalid"].(map[string]any)
	if invalid["action"] != actionAccept || invalid["source"] != sourcePeer {
		return fmt.Errorf("peer 127.0.0.1 invalid action/source wrong: %v", invalid)
	}
	notFound, _ := entry["not-found"].(map[string]any)
	if notFound["source"] != "global" {
		return fmt.Errorf("peer 127.0.0.1 not-found should inherit global: %v", notFound)
	}
	fmt.Fprintln(os.Stderr, "OK: Invalid route accepted via per-peer override; status reports peer-actions")
	return nil
}

func plugin15RPKILateSync(ctx context.Context, p *sdk.Plugin) error {
	if !Poll(ctx, 24, 500*time.Millisecond, func() bool { return plugin15UpdatesReceived(ctx, p) >= 1 }) {
		return fmt.Errorf("ze never received the UPDATE")
	}
	if err := plugin14AssertRouteValidation(ctx, p, "10.0.1.0/24", 2, false, "before cache synchronization"); err != nil {
		return err
	}
	count := plugin15RPKICount(ctx, p)
	if count < 0 {
		return fmt.Errorf("the pre-sync RPKI status did not report its VRP count")
	}
	if count != 0 {
		return fmt.Errorf("the cache synced before the UPDATE arrived, so this run does not exercise re-validation")
	}
	before, err := plugin15Map(plugin15Dispatch(ctx, p, "show bgp adj-rib-in"))
	if err != nil {
		return err
	}
	original := plugin14FindRoute(before, "10.0.1.0/24")
	if !plugin14RouteValidation(before, "10.0.1.0/24", 2, false) {
		return fmt.Errorf("the pre-sync route changed before releasing the cache: %v", before)
	}
	if err := os.WriteFile("rpki-sync.release", nil, 0o600); err != nil {
		return fmt.Errorf("release the initial RTR synchronization: %w", err)
	}
	if !Poll(ctx, 24, 500*time.Millisecond, func() bool { return plugin15RPKICount(ctx, p) >= 1 }) {
		return fmt.Errorf("the RTR cache never synced a VRP")
	}
	if err := plugin14AssertRouteValidation(ctx, p, "10.0.1.0/24", 3, true, "after cache synchronization"); err != nil {
		return err
	}
	after, err := plugin15Map(plugin15Dispatch(ctx, p, "show bgp adj-rib-in"))
	if err != nil {
		return err
	}
	retained := plugin14FindRoute(after, "10.0.1.0/24")
	if !plugin14RouteValidation(after, "10.0.1.0/24", 3, true) {
		return fmt.Errorf("the post-sync route lost its validation result: %v", after)
	}
	for _, key := range []string{"attr-hex", "nhop-hex", "nlri-hex"} {
		if retained[key] != original[key] {
			return fmt.Errorf("RPKI revalidation changed received %s: before=%v after=%v", key, original[key], retained[key])
		}
	}
	fmt.Fprintln(os.Stderr, "OK re-validation retained 10.0.1.0/24 as ineligible once the cache synced")
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
	if err := plugin14AssertRouteValidation(ctx, p, prefix, state, false, "origin validation"); err != nil {
		return err
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
	for _, route := range []struct {
		prefix     string
		state      int
		ineligible bool
	}{
		{"10.0.1.0/24", 1, false},
		{"10.0.2.0/24", 1, false},
		{"10.0.3.0/24", 3, true},
		{"192.168.1.0/24", 2, false},
	} {
		if err := plugin14AssertRouteValidation(ctx, p, route.prefix, route.state, route.ineligible, "batch validation"); err != nil {
			return err
		}
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
	if err := plugin14AssertRouteValidation(ctx, p, "10.0.1.0/24", 3, true, "origin rejection"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: route 10.0.1.0/24 correctly rejected (Invalid)")
	return nil
}
