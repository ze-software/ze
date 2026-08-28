package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func observerBudgets02() (time.Duration, time.Duration) {
	for _, key := range []string{"ze.test.budget", "ze_test_budget", "ZE_TEST_BUDGET"} {
		if raw := os.Getenv(key); raw != "" {
			if budget, err := time.ParseDuration(raw); err == nil && budget > 0 {
				return time.Duration(float64(budget) * 0.60), time.Duration(float64(budget) * 0.25)
			}
		}
	}
	return 30 * time.Second, 15 * time.Second
}

func routeServerObserver02(name string, expectedPeers int, forwardPrefix string) Driver {
	return eventObserver02(name, []string{"update"}, func(ctx context.Context, plugin *sdk.Plugin, events <-chan string, shutdown <-chan struct{}) error {
		eorTimeout, shutdownTimeout := observerBudgets02()
		eorPeers := make(map[string]bool)
		forwardSeen := forwardPrefix == ""
		idleDeadline := time.Now().Add(eorTimeout)
		hardDeadline := time.Now().Add(eorTimeout * 8)
		for (len(eorPeers) < expectedPeers || !forwardSeen) && time.Now().Before(idleDeadline) && time.Now().Before(hardDeadline) {
			event, ok := nextEvent02(ctx, events, shutdown, 500*time.Millisecond)
			if !ok {
				continue
			}
			body := eventBody02(event)
			message, _ := body["message"].(map[string]any)
			if message["direction"] != "sent" {
				continue
			}
			update, _ := body["update"].(map[string]any)
			nlri, present := update["nlri"]
			emptyNLRI := !present || nlri == nil
			switch value := nlri.(type) {
			case []any:
				emptyNLRI = len(value) == 0
			case string:
				emptyNLRI = value == ""
			case map[string]any:
				emptyNLRI = len(value) == 0
			}
			if emptyNLRI {
				peer, _ := body["peer"].(map[string]any)
				remote, _ := peer["remote"].(map[string]any)
				address, _ := remote["address"].(string)
				if address != "" && !eorPeers[address] {
					eorPeers[address] = true
					idleDeadline = time.Now().Add(eorTimeout)
				}
			} else if !forwardSeen {
				encoded, _ := json.Marshal(update)
				if strings.Contains(string(encoded), forwardPrefix) {
					forwardSeen = true
					idleDeadline = time.Now().Add(eorTimeout)
				}
			}
		}
		if len(eorPeers) < expectedPeers || !forwardSeen {
			return fmt.Errorf("route server did not replay (expected_peers=%d, forward_prefix=%s)", expectedPeers, forwardPrefix)
		}
		requestShutdownAsync02(ctx, plugin)
		return waitForShutdown02(ctx, shutdown, shutdownTimeout)
	})
}

type auditEntry02 struct {
	Action  string `json:"action"`
	Surface string `json:"surface"`
	Outcome string `json:"outcome"`
	Detail  string `json:"detail"`
}

type auditResult02 struct {
	Entries []auditEntry02 `json:"entries"`
	Count   int            `json:"count"`
}

func auditRequest02(ctx context.Context, client *http.Client, method, path string, payload any) (map[string]any, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://127.0.0.1:18087/api/v1"+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s %s: HTTP %s: %s", method, path, resp.Status, raw)
	}
	out := make(map[string]any)
	if len(raw) != 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func auditRESTReady02(ctx context.Context, client *http.Client) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:18087/api/v1/commands", http.NoBody)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close() //nolint:errcheck
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func auditConfigCommit02(ctx context.Context, plugin *sdk.Plugin) error {
	if err := apiRIBReady02(ctx, plugin); err != nil {
		return err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	if !Poll(ctx, 20, 500*time.Millisecond, func() bool {
		return auditRESTReady02(ctx, client)
	}) {
		return fmt.Errorf("REST API did not respond")
	}
	created, err := auditRequest02(ctx, client, http.MethodPost, "/config/sessions", nil)
	if err != nil {
		return err
	}
	sessionID, _ := created["session-id"].(string)
	if sessionID == "" {
		return fmt.Errorf("create session response=%v", created)
	}
	if _, err := auditRequest02(ctx, client, http.MethodPut, "/config/sessions/"+sessionID, map[string]any{"path": "bgp.router-id", "value": "10.0.0.2"}); err != nil {
		return err
	}
	committed, err := auditRequest02(ctx, client, http.MethodPost, "/config/sessions/"+sessionID+"/commit", nil)
	if err != nil {
		return err
	}
	if committed["status"] != "committed" {
		return fmt.Errorf("commit response=%v", committed)
	}
	fmt.Fprintln(os.Stderr, "OK: REST config commit succeeded")

	var audit auditResult02
	if !Poll(ctx, 20, 250*time.Millisecond, func() bool {
		raw, err := requireDone02(ctx, plugin, "show audit action config-commit")
		if err != nil || decode02(raw, &audit) != nil {
			return false
		}
		return len(audit.Entries) > 0
	}) {
		return fmt.Errorf("show audit returned no config-commit entries")
	}
	entry := audit.Entries[len(audit.Entries)-1]
	if entry.Action != "config-commit" || entry.Surface != "rest" || entry.Outcome != "success" {
		return fmt.Errorf("unexpected audit entry=%+v", entry)
	}
	if !strings.Contains(entry.Detail, "router-id") ||
		!strings.Contains(entry.Detail, "1.2.3.4") ||
		!strings.Contains(entry.Detail, "10.0.0.2") {
		return fmt.Errorf("audit detail missing router-id change: %+v", entry)
	}
	fmt.Fprintln(os.Stderr, "OK: show audit includes REST config commit")

	raw, err := requireDone02(ctx, plugin, "show audit action config-commit since 1970-01-01T00:00:00Z until 2999-01-01T00:00:00Z")
	if err != nil {
		return err
	}
	var ranged auditResult02
	if err := decode02(raw, &ranged); err != nil {
		return err
	}
	if ranged.Count < 1 {
		return fmt.Errorf("ranged show audit returned no entries: %+v", ranged)
	}
	fmt.Fprintln(os.Stderr, "OK: show audit time range filter includes config commit")

	created, err = auditRequest02(ctx, client, http.MethodPost, "/config/sessions", nil)
	if err != nil {
		return err
	}
	discardID, _ := created["session-id"].(string)
	if discardID == "" {
		return fmt.Errorf("create discard session response=%v", created)
	}
	if _, err := auditRequest02(ctx, client, http.MethodPut, "/config/sessions/"+discardID, map[string]any{"path": "bgp.router-id", "value": "10.0.0.3"}); err != nil {
		return err
	}
	discarded, err := auditRequest02(ctx, client, http.MethodDelete, "/config/sessions/"+discardID, nil)
	if err != nil {
		return err
	}
	if discarded["status"] != "discarded" {
		return fmt.Errorf("discard response=%v", discarded)
	}
	raw, err = requireDone02(ctx, plugin, "show audit action config-discard")
	if err != nil {
		return err
	}
	audit = auditResult02{}
	if err := decode02(raw, &audit); err != nil {
		return err
	}
	if len(audit.Entries) == 0 {
		return fmt.Errorf("show audit returned no config-discard entries")
	}
	entry = audit.Entries[len(audit.Entries)-1]
	if entry.Action != "config-discard" || entry.Surface != "rest" || entry.Outcome != "success" {
		return fmt.Errorf("unexpected discard audit entry=%+v", entry)
	}
	if !strings.Contains(entry.Detail, "10.0.0.3") {
		return fmt.Errorf("discard audit detail missing discarded value: %+v", entry)
	}
	fmt.Fprintln(os.Stderr, "OK: show audit includes REST config discard")
	return nil
}
