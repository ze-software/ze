// Design: docs/architecture/resolve.md -- Clearing a resolver cache
// Related: register_clear_caches.go -- the scenario registrations
// Related: plugin_fixture_04.go -- command04, requireDone04 and the polling helpers

package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The clear answers name what they did under one key, with one of two values.
const (
	clearKeyAction04   = "action"
	clearActionAll04   = "clear-all"
	clearActionEntry04 = "delete-entry"
	clearKeyFound04    = "found"
)

// requireRefused04 dispatches command and requires the dispatcher to refuse
// it, with a message that holds every word in want.
func requireRefused04(ctx context.Context, p *sdk.Plugin, command string, want ...string) error {
	status, value, err := command04(ctx, p, command)
	if err != nil {
		return err
	}
	if status != statusError {
		return fmt.Errorf("%s: status=%s value=%v, want a refusal", command, status, value)
	}
	text := fmt.Sprint(value)
	for _, word := range want {
		if !strings.Contains(text, word) {
			return fmt.Errorf("%s: refusal %q does not name %q", command, text, word)
		}
	}
	fmt.Fprintf(os.Stderr, "OK: %s refused: %s\n", command, text)
	return nil
}

// requireAnswer04 dispatches command and requires each key of want in the answer.
func requireAnswer04(ctx context.Context, p *sdk.Plugin, command string, want map[string]any) (map[string]any, error) {
	status, value, err := command04(ctx, p, command)
	if err != nil {
		return nil, err
	}
	data, _ := value.(map[string]any)
	if status != statusDone || data == nil {
		return nil, fmt.Errorf("%s: status=%s answer=%v", command, status, value)
	}
	for key, value := range want {
		if fmt.Sprint(data[key]) != fmt.Sprint(value) {
			return nil, fmt.Errorf("%s: %s=%v, want %v (answer %v)", command, key, data[key], value, data)
		}
	}
	fmt.Fprintf(os.Stderr, "OK: %s -> %v\n", command, data)
	return data, nil
}

// peeringDBServed04 looks up the AS-SET of 65001 and requires the answer the
// mock gives on its served-th request, so a cached answer is told from a query.
func peeringDBServed04(ctx context.Context, p *sdk.Plugin, served int) error {
	command := "resolve peeringdb as-set 65001"
	data, err := requireDone04(ctx, p, command)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}
	want := fmt.Sprint("AS-SERVED-", served)
	if !strings.Contains(string(encoded), `"`+want+`"`) {
		return fmt.Errorf("%s: answer %s, want %s", command, encoded, want)
	}
	fmt.Fprintf(os.Stderr, "OK: %s -> %s\n", command, want)
	return nil
}

// clearPeeringDBCache04 proves each form of clear peeringdb cache through the
// dispatcher: the bare form clears nothing, asn clears one ASN and all clears
// everything, each shown by the next lookup reaching the mock again.
func clearPeeringDBCache04(ctx context.Context, p *sdk.Plugin) error {
	if err := waitPeerEOR04(ctx, p); err != nil {
		return err
	}
	steps := []func() error{
		func() error { return peeringDBServed04(ctx, p, 1) },
		func() error { return peeringDBServed04(ctx, p, 1) },
		func() error { return requireRefused04(ctx, p, "clear peeringdb cache", "all", "asn") },
		func() error { return peeringDBServed04(ctx, p, 1) },
		func() error {
			_, err := requireAnswer04(ctx, p, "clear peeringdb cache asn 65001", map[string]any{clearKeyAction04: clearActionEntry04, "as-number": 65001, clearKeyFound04: true})
			return err
		},
		func() error { return peeringDBServed04(ctx, p, 2) },
		func() error {
			_, err := requireAnswer04(ctx, p, "clear peeringdb cache asn 0.64999", map[string]any{"as-number": 64999, clearKeyFound04: false})
			return err
		},
		func() error {
			data, err := requireAnswer04(ctx, p, "clear peeringdb cache all", map[string]any{clearKeyAction04: clearActionAll04})
			if err == nil && number04(data["removed"]) < 1 {
				err = fmt.Errorf("clear peeringdb cache all: removed=%v, want at least the AS-SET entry", data["removed"])
			}
			return err
		},
		func() error { return peeringDBServed04(ctx, p, 3) },
		func() error { return requireRefused04(ctx, p, "clear peeringdb cache asn AS-FOO", "AS-FOO") },
	}
	return runSteps04(steps)
}

// clearIRRCache04 proves clear irr cache reaches the IRR client the
// bgp-filter-irr plugin built, through the plugin's resolver-cache-clear
// callback: the hub's own resolver never looks AS-TEST up, so the entries can
// only be the plugin's, and the answer names the plugin as the owner that
// removed them. The same steps run with the plugin internal and external.
func clearIRRCache04(ctx context.Context, p *sdk.Plugin) error {
	command := "clear irr cache as-set AS-TEST"
	if !Poll(ctx, 100, 200*time.Millisecond, func() bool {
		status, value, err := command04(ctx, p, command)
		data, _ := value.(map[string]any)
		return err == nil && status == statusDone && data["found"] == true && ownerRemoved04(data, "bgp-filter-irr") > 0
	}) {
		return errors.New(command + ": never found the AS-SET the IRR filter refreshed in the bgp-filter-irr owner")
	}
	fmt.Fprintf(os.Stderr, "OK: %s -> found true, removed by owner bgp-filter-irr\n", command)
	steps := []func() error{
		func() error {
			_, err := requireAnswer04(ctx, p, command, map[string]any{clearKeyAction04: clearActionEntry04, "name": "AS-TEST", clearKeyFound04: false})
			return err
		},
		func() error { return requireRefused04(ctx, p, "clear irr cache", "all", "as-set") },
		func() error {
			_, err := requireAnswer04(ctx, p, "clear irr cache all", map[string]any{clearKeyAction04: clearActionAll04})
			return err
		},
	}
	return runSteps04(steps)
}

// ownerRemoved04 returns what owner removed in a clear answer's owners list,
// or -1 when the answer does not list owner.
func ownerRemoved04(data map[string]any, owner string) int {
	rows, _ := data["owners"].([]any)
	for _, row := range rows {
		fields, _ := row.(map[string]any)
		if fields["owner"] == owner {
			return int(number04(fields["removed"]))
		}
	}
	return -1
}

// clearCymruCache04 proves each form of clear cymru cache through the
// dispatcher. The hub's Cymru lookups go through its DNS resolver, which asks
// port 53 only, so no mock answers here and the re-query proof is the unit
// test's (internal/component/resolve/cymru/clear_test.go).
func clearCymruCache04(ctx context.Context, p *sdk.Plugin) error {
	if err := waitPeerEOR04(ctx, p); err != nil {
		return err
	}
	steps := []func() error{
		func() error { return requireRefused04(ctx, p, "clear cymru cache", "all", "address") },
		func() error {
			_, err := requireAnswer04(ctx, p, "clear cymru cache all", map[string]any{clearKeyAction04: clearActionAll04})
			return err
		},
		func() error {
			_, err := requireAnswer04(ctx, p, "clear cymru cache address 2001:DB8:0::1", map[string]any{clearKeyAction04: clearActionEntry04, "ip-address": "2001:db8::1", clearKeyFound04: false})
			return err
		},
		func() error { return requireRefused04(ctx, p, "clear cymru cache address router1", "router1") },
	}
	return runSteps04(steps)
}

// runSteps04 runs each step in order and stops at the first failure.
func runSteps04(steps []func() error) error {
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}
