// Design: docs/architecture/api/json-format.md -- the connected and fsm events
// Related: plugin_fixture_12.go -- registers plugin/fsm-connected-events

package fixture

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// p12FSMConnectedEvents is the scenario of test/plugin/fsm-connected-events.ci.
// The plugin subscribes to `connected` and `fsm`, and the daemon must deliver,
// in this order, the `connected` event and the FSM changes into OPENSENT,
// OPENCONFIRM and ESTABLISHED. Each event is recorded by one word as it
// arrives (`connected`, or the FSM state it moved to), and the order is judged
// as the words come in.
func p12FSMConnectedEvents(plugin *sdk.Plugin) p12Scenario {
	seen := make(chan string, 64)
	plugin.OnEvent(func(event string) error {
		var decoded map[string]any
		if json.Unmarshal([]byte(event), &decoded) != nil {
			return nil //nolint:nilerr // a malformed event is skipped, and failing the handler would end the session
		}
		bgp, _ := decoded["bgp"].(map[string]any)
		message, _ := bgp["message"].(map[string]any)
		kind, _ := message["type"].(string)
		word := kind
		if kind == "fsm" {
			transition, _ := bgp["fsm"].(map[string]any)
			word, _ = transition["to"].(string)
		}
		select {
		case seen <- word:
		default:
		}
		return nil
	})

	return func(ctx context.Context, _ *sdk.Plugin) error {
		timer := time.NewTimer(10 * time.Second)
		defer timer.Stop()
		want := []string{"connected", "OPENSENT", "OPENCONFIRM", "ESTABLISHED"}
		var got []string
		for len(want) > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return fmt.Errorf("still waiting for %v after %v", want, got)
			case word := <-seen:
				got = append(got, word)
				if word == want[0] {
					want = want[1:]
				}
			}
		}
		fmt.Fprintln(os.Stderr, "OK: connected and fsm events delivered in order:", got)
		return nil
	}
}
