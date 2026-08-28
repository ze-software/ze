package fixture

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func p12PrefixChainOrder(ctx context.Context, plugin *sdk.Plugin) error {
	if !Poll(ctx, 100, 200*time.Millisecond, func() bool {
		return p12NumberAtLeast(p12PeerField(ctx, plugin, "peer1", "127.0.0.1", "updates-received"), 1)
	}) {
		return fmt.Errorf("ze never received the UPDATE")
	}
	return p12Quiesce(ctx, plugin)
}

func p12PrefixEntryOrder(ctx context.Context, plugin *sdk.Plugin) error {
	if !Poll(ctx, 100, 200*time.Millisecond, func() bool {
		return p12NumberAtLeast(p12PeerField(ctx, plugin, "peer1", "127.0.0.1", "updates-received"), 1)
	}) {
		return fmt.Errorf("ze never received the UPDATE")
	}
	if err := p12Quiesce(ctx, plugin); err != nil {
		return err
	}
	total, ok := p12RouteTotal(ctx, plugin)
	if !ok {
		return fmt.Errorf("show bgp adj-rib-in status did not return total-routes")
	}
	if total != 0 {
		return fmt.Errorf("expected 0 routes (first entry rejects), got %d; catch-all accept entry ran first", total)
	}
	fmt.Fprintln(os.Stderr, "OK: the first entry the operator wrote is the entry that matched")
	return nil
}

func p12PrefixReject(ctx context.Context, plugin *sdk.Plugin) error {
	if !Poll(ctx, 100, 200*time.Millisecond, func() bool {
		return p12NumberAtLeast(p12PeerField(ctx, plugin, "peer1", "127.0.0.1", "updates-received"), 1)
	}) {
		return fmt.Errorf("ze never received the UPDATE")
	}
	if err := p12Quiesce(ctx, plugin); err != nil {
		return err
	}
	total, ok := p12RouteTotal(ctx, plugin)
	if !ok {
		return fmt.Errorf("show bgp adj-rib-in status did not return total-routes")
	}
	if total != 0 {
		return fmt.Errorf("expected 0 routes (implicit deny), got %d", total)
	}
	fmt.Fprintln(os.Stderr, "OK: route correctly rejected by prefix-list (implicit deny)")
	return nil
}
func p12WaitRoutes(minimum int) p12Scenario {
	return func(ctx context.Context, plugin *sdk.Plugin) error {
		if !Poll(ctx, 20, 250*time.Millisecond, func() bool {
			total, ok := p12RouteTotal(ctx, plugin)
			return ok && total >= minimum
		}) {
			return fmt.Errorf("adj-rib-in never reached %d routes", minimum)
		}
		return nil
	}
}

func p12RIBCount(ctx context.Context, plugin *sdk.Plugin) (int, bool) {
	status, data, err := p12DispatchObject(ctx, plugin, "show bgp rib received count")
	if err != nil || status != "done" {
		return 0, false
	}
	value, exists := data["count"]
	if !exists {
		return 0, false
	}
	return p12Number(value)
}

func p12WarnOnlyDropsNLRI(ctx context.Context, plugin *sdk.Plugin) error {
	var count int
	var readable bool
	Poll(ctx, 60, 250*time.Millisecond, func() bool {
		count, readable = p12RIBCount(ctx, plugin)
		return readable && count >= 1
	})
	if !readable {
		return fmt.Errorf("bgp-rib did not answer show bgp rib received count")
	}
	if count < 1 {
		return fmt.Errorf("the control route never reached the RIB: count=%d, wanted at least 1", count)
	}
	fmt.Fprintf(os.Stderr, "OK control: an in-limit route is installed, show bgp rib received count=%d\n", count)

	Poll(ctx, 12, 250*time.Millisecond, func() bool {
		count, readable = p12RIBCount(ctx, plugin)
		return readable && count > 2
	})
	if !readable {
		return fmt.Errorf("show bgp rib received count stopped answering")
	}
	if count > 2 {
		return fmt.Errorf("AC-2: warn-only must not install routes past the maximum: count=%d, maximum=2, announced=4, after-control=3", count)
	}
	fmt.Fprintf(os.Stderr, "OK AC-2: %d of 4 announced routes are installed, never past the maximum of 2, so the over-limit UPDATE was dropped\n", count)
	return nil
}

func p12RouteServerReplay(plugin *sdk.Plugin) p12Scenario {
	var mu sync.Mutex
	eorPeers := make(map[string]struct{})
	forwardSeen := false
	progress := make(chan struct{}, 4)
	plugin.OnEvent(func(event string) error {
		var decoded map[string]any
		if json.Unmarshal([]byte(event), &decoded) != nil {
			return nil
		}
		bgp, _ := decoded["bgp"].(map[string]any)
		message, _ := bgp["message"].(map[string]any)
		if message["direction"] != "sent" {
			return nil
		}
		update, _ := bgp["update"].(map[string]any)
		nlri, hasNLRI := update["nlri"]
		progressed := false
		mu.Lock()
		if !hasNLRI || p12EmptyCollection(nlri) {
			peer, _ := bgp["peer"].(map[string]any)
			remote, _ := peer["remote"].(map[string]any)
			if address, _ := remote["address"].(string); address != "" {
				if _, exists := eorPeers[address]; !exists {
					eorPeers[address] = struct{}{}
					progressed = true
				}
			}
		}
		if !forwardSeen && strings.Contains(string(mustJSONP12(update)), "10.0.0.0/24") {
			forwardSeen = true
			progressed = true
		}
		mu.Unlock()
		if progressed {
			progress <- struct{}{}
		}
		return nil
	})

	return func(ctx context.Context, _ *sdk.Plugin) error {
		idleWindow := 30 * time.Second
		if budget, err := time.ParseDuration(os.Getenv("ze_test_budget")); err == nil && budget > 0 {
			idleWindow = time.Duration(float64(budget) * 0.60)
		}
		idle := time.NewTimer(idleWindow)
		defer idle.Stop()
		hard := time.NewTimer(idleWindow * 8)
		defer hard.Stop()

		for {
			mu.Lock()
			complete := len(eorPeers) >= 2 && forwardSeen
			mu.Unlock()
			if complete {
				return nil
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-hard.C:
				mu.Lock()
				defer mu.Unlock()
				return fmt.Errorf("route server replay exceeded hard deadline (eor-peers=%d forward-seen=%t)", len(eorPeers), forwardSeen)
			case <-idle.C:
				mu.Lock()
				defer mu.Unlock()
				return fmt.Errorf("route server replay made no progress for %s (eor-peers=%d forward-seen=%t)", idleWindow, len(eorPeers), forwardSeen)
			case <-progress:
				if !idle.Stop() {
					select {
					case <-idle.C:
					default:
					}
				}
				idle.Reset(idleWindow)
			}
		}
	}
}

func p12EmptyCollection(value any) bool {
	switch collection := value.(type) {
	case nil:
		return true
	case []any:
		return len(collection) == 0
	case map[string]any:
		return len(collection) == 0
	default:
		return false
	}
}

func mustJSONP12(value any) []byte {
	raw, _ := json.Marshal(value)
	return raw
}

func p12QuiesceBarrier(ctx context.Context, plugin *sdk.Plugin) error {
	for _, command := range []string{
		"update text nhop 101.1.101.1 nlri ipv4/unicast add 1.1.0.0/24",
		"update text nhop 101.1.101.1 nlri ipv4/unicast add 1.2.0.0/25",
	} {
		if _, _, err := plugin.UpdateRoute(ctx, "*", command); err != nil {
			return fmt.Errorf("send %q: %w", command, err)
		}
	}
	return p12Quiesce(ctx, plugin)
}
