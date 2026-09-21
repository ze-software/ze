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

func communityAttributes04(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("unexpected arguments: %v", args)
	}
	p, err := newObserver("fixture-community-attributes-04")
	if err != nil {
		return err
	}
	defer p.Close() //nolint:errcheck // fixture teardown
	events := make(chan map[string]any, 1)
	p.OnEvent(func(event string) error {
		var decoded map[string]any
		if json.Unmarshal([]byte(event), &decoded) != nil {
			return nil //nolint:nilerr // a malformed event is skipped, and failing the handler would end the session
		}
		bgp, _ := decoded["bgp"].(map[string]any)
		update, _ := bgp["update"].(map[string]any)
		attr, _ := update["attr"].(map[string]any)
		if len(attr) != 0 {
			select {
			case events <- attr:
			default:
			}
		}
		return nil
	})
	p.SetStartupSubscriptions([]string{eventUpdate}, nil, "parsed")
	result := make(chan error, 1)
	p.OnAllPluginsReady(func() error {
		go func() {
			var scenarioErr error
			select {
			case attr := <-events:
				scenarioErr = checkCommunityAttributes04(attr)
			case <-time.After(15 * time.Second):
				scenarioErr = errors.New("no UPDATE event carrying attributes arrived")
			}
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _, _ = p.DispatchCommand(shutdownCtx, "request shutdown")
			result <- scenarioErr
		}()
		return nil
	})
	runErr := p.Run(ctx, sdk.Registration{})
	select {
	case scenarioErr := <-result:
		return errors.Join(scenarioErr, runErr)
	default:
		return runErr
	}
}

func checkCommunityAttributes04(attr map[string]any) error {
	want := map[string][]string{
		"communities":          {"65001:100"},
		"extended-communities": {"target:65000:1", "mark:46", "target:65536:100", "traffic-action:sample-terminal", "traffic-action:sample-terminal"},
		// RFC 5701 Section 3 assigns sub-type 0x0002 to the IPv6 address
		// specific Route Target, so this reads the way its 8-octet sibling
		// does. The address is bracketed because it contains colons of its
		// own, which is the one place the two shapes differ.
		//
		// It held bare hex until 2026-09-21 and then the undecoded
		// `0x0002:...` for an hour. `ze bgp decode` answered the first and the
		// plugin feed the second, over the same octets; both now answer this.
		"ipv6-extended-communities": {"target:[2001:db8::]:1"},
		"large-communities":         {"65001:1:2"},
	}
	for key, expected := range want {
		got := stringSlice04(attr[key])
		if strings.Join(got, "\x00") != strings.Join(expected, "\x00") {
			return fmt.Errorf("%s: expected %v, got %v in %v", key, expected, got, attr)
		}
	}
	for _, key := range []string{"attr-8", "attr-16", "attr-25", "attr-32"} {
		if _, ok := attr[key]; ok {
			return fmt.Errorf("%s fell back to raw hex: %v", key, attr)
		}
	}
	fmt.Fprintln(os.Stderr, "OK: all four community attributes decoded")
	return nil
}
