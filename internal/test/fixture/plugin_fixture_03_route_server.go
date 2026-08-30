package fixture

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func routeServerObserver03(expectedPeers int, waitForward bool) ObserverScenario {
	return func(ctx context.Context, p *sdk.Plugin) error {
		if !waitPeersEOR03(ctx, p, expectedPeers) {
			return fmt.Errorf("route-server replay did not send End-of-RIB to %d peers", expectedPeers)
		}
		if waitForward {
			if !Poll(ctx, 40, 250*time.Millisecond, func() bool {
				rows, err := peerRows03(ctx, p, "")
				if err != nil {
					return false
				}
				for peer := range rows {
					if count, err := realUpdates03(ctx, p, peer); err == nil && count > 0 {
						return true
					}
				}
				return false
			}) {
				return fmt.Errorf("route-server replay did not forward the route")
			}
		}
		return nil
	}
}

func bgpRSControlWithdraw03(ctx context.Context, p *sdk.Plugin) error {
	if err := quiesce03(ctx, p); err != nil {
		return err
	}
	if !Poll(ctx, 40, 250*time.Millisecond, func() bool {
		count, err := realUpdates03(ctx, p, "127.0.0.3")
		return err == nil && count >= 3
	}) {
		return fmt.Errorf("included client was not sent both halves and the fence")
	}
	if !Poll(ctx, 40, 250*time.Millisecond, func() bool {
		count, err := realUpdates03(ctx, p, "127.0.0.2")
		return err == nil && count >= 2
	}) {
		return fmt.Errorf("excluded client was not sent the withdrawal and the fence")
	}
	count, err := realUpdates03(ctx, p, "127.0.0.2")
	if err != nil {
		return err
	}
	if count != 2 {
		return fmt.Errorf("excluded client was sent %d route UPDATEs, want 2", count)
	}
	return nil
}

func bgpRSFastpath03(ctx context.Context, p *sdk.Plugin) error {
	value, err := done03(ctx, p, "request bgp rib fastpath enable")
	if err != nil {
		return err
	}
	data, ok := value.(map[string]any)
	if !ok || data["enabled"] != true {
		return fmt.Errorf("fast path not enabled: %v", value)
	}
	if !Poll(ctx, 60, 200*time.Millisecond, func() bool {
		value, err := done03(ctx, p, "request bgp rib fastpath status")
		if err != nil {
			return false
		}
		status, ok := value.(map[string]any)
		return ok && number03(status["forwarded"]) > 0
	}) {
		return fmt.Errorf("fast path read no forward-handle bytes")
	}
	return nil
}

func bgpRSPprof03(ctx context.Context, p *sdk.Plugin, port string) error {
	if !waitPeersEOR03(ctx, p, 1) {
		return fmt.Errorf("initial-sync EOR never reached the wire")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+port+"/debug/pprof/", http.NoBody)
	if err != nil {
		return err
	}
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		return fmt.Errorf("pprof endpoint not reachable: %w", err)
	}
	defer response.Body.Close() //nolint:errcheck // the body is read
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read pprof index: %w", err)
	}
	body := string(raw)
	for _, profile := range []string{"profile", "heap", "allocs"} {
		if !strings.Contains(body, profile) {
			return fmt.Errorf("pprof index missing profile %s", profile)
		}
	}
	return nil
}

func pollHTTP03(ctx context.Context, url string, attempts int, ready func(string) bool) (string, error) {
	client := &http.Client{Timeout: time.Second}
	var body string
	var lastErr error
	ok := Poll(ctx, attempts, 250*time.Millisecond, func() bool {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
		if err != nil {
			lastErr = err
			return false
		}
		resp, err := client.Do(request)
		if err != nil {
			lastErr = err
			return false
		}
		defer resp.Body.Close() //nolint:errcheck // the body is read
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = err
			return false
		}
		body = string(raw)
		return ready(body)
	})
	if !ok {
		if lastErr != nil {
			return body, lastErr
		}
		return body, fmt.Errorf("condition not met at %s", url)
	}
	return body, nil
}
