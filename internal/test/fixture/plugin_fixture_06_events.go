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

func fixture06Errors(ctx context.Context, p *sdk.Plugin, attempts int, match func(map[string]any) bool) ([]any, error) {
	data, err := fixture06PollObject(ctx, p, "show errors", attempts, func(data map[string]any) bool {
		errorsList, ok := data["errors"].([]any)
		if !ok {
			return false
		}
		for _, raw := range errorsList {
			entry, _ := raw.(map[string]any)
			if match(entry) {
				return true
			}
		}
		return false
	})
	if err != nil {
		return nil, err
	}
	entries, _ := data["errors"].([]any)
	return entries, nil
}

func fixture06MatchingErrors(entries []any, match func(map[string]any) bool) []map[string]any {
	var matches []map[string]any
	for _, raw := range entries {
		entry, _ := raw.(map[string]any)
		if match(entry) {
			matches = append(matches, entry)
		}
	}
	return matches
}

func fixture06ErrorsConfigAbort(ctx context.Context, p *sdk.Plugin) error {
	p.OnConfigVerify(func([]sdk.ConfigSection) error { return errors.New("test plugin rejects reload") })
	match := func(entry map[string]any) bool {
		return entry["source"] == sourceConfig && entry["code"] == "commit-aborted"
	}
	entries, err := fixture06Errors(ctx, p, 60, match)
	if err != nil {
		return errors.New("timed out waiting for config/commit-aborted entry")
	}
	matches := fixture06MatchingErrors(entries, match)
	if len(matches) == 0 {
		return fmt.Errorf("no commit-aborted entry in %v", entries)
	}
	entry := matches[0]
	detail, _ := entry["detail"].(map[string]any)
	subject, _ := entry["subject"].(string)
	reason, _ := detail["reason"].(string)
	if entry["severity"] != severityError || subject == "" || detail["phase"] != "verify" || reason == "" {
		return fmt.Errorf("invalid commit-aborted entry: %v", entry)
	}
	fmt.Fprintf(os.Stderr, "OK: show errors returned %d entries; commit-aborted present\n", len(entries))
	return nil
}

func fixture06ErrorsReceived(ctx context.Context, p *sdk.Plugin) error {
	match := func(entry map[string]any) bool {
		return entry["source"] == namespaceBGP && entry["code"] == "notification-received" && entry["subject"] == addrLoopback
	}
	entries, err := fixture06Errors(ctx, p, 20, match)
	if err != nil {
		return errors.New("timed out waiting for bgp/notification-received entry")
	}
	matches := fixture06MatchingErrors(entries, match)
	if len(matches) != 1 {
		return fmt.Errorf("expected exactly 1 notification-received entry, got %d: %v", len(matches), matches)
	}
	entry := matches[0]
	detail, _ := entry["detail"].(map[string]any)
	code, codeOK := fixture06Number(detail["code"])
	subcode, subcodeOK := fixture06Number(detail["subcode"])
	if entry["severity"] != severityError || detail["direction"] != directionReceived || !codeOK || code != 6 || !subcodeOK || subcode != 2 {
		return fmt.Errorf("invalid notification-received entry: %v", entry)
	}
	fmt.Fprintf(os.Stderr, "OK: show errors returned %d entry(s); 1 matching notification-received\n", len(entries))
	return nil
}

func fixture06ErrorsSent(ctx context.Context, p *sdk.Plugin) error {
	if err := fixture06WaitEOR(ctx, p, 1); err != nil {
		return err
	}
	if err := fixture06DispatchDone(ctx, p, "request peer 127.0.0.1 teardown 4"); err != nil {
		return err
	}
	match := func(entry map[string]any) bool {
		return entry["source"] == namespaceBGP && entry["code"] == "notification-sent" && entry["subject"] == addrLoopback
	}
	entries, err := fixture06Errors(ctx, p, 20, match)
	if err != nil {
		return errors.New("timed out waiting for bgp/notification-sent entry")
	}
	matches := fixture06MatchingErrors(entries, match)
	if len(matches) != 1 {
		return fmt.Errorf("expected exactly 1 notification-sent entry, got %d: %v", len(matches), matches)
	}
	entry := matches[0]
	detail, _ := entry["detail"].(map[string]any)
	code, codeOK := fixture06Number(detail["code"])
	subcode, subcodeOK := fixture06Number(detail["subcode"])
	if entry["severity"] != severityError || detail["direction"] != directionSent || !codeOK || code != 6 || !subcodeOK || subcode != 4 {
		return fmt.Errorf("invalid notification-sent entry: %v", entry)
	}
	fmt.Fprintf(os.Stderr, "OK: show errors returned %d entry(s); 1 matching notification-sent\n", len(entries))
	return nil
}

func fixture06EventMonitorBasic(ctx context.Context, p *sdk.Plugin) error {
	if err := fixture06WaitEOR(ctx, p, 1); err != nil {
		return err
	}
	return streamingOnlyRefused(ctx, p, "monitor event")
}

func fixture06EventMonitorExclude(ctx context.Context, p *sdk.Plugin) error {
	if err := fixture06WaitEOR(ctx, p, 1); err != nil {
		return err
	}
	return streamingOnlyRefused(ctx, p, "monitor event exclude keepalive")
}

func fixture06EventMonitorInclude(ctx context.Context, p *sdk.Plugin) error {
	if err := fixture06WaitEOR(ctx, p, 1); err != nil {
		return err
	}
	return streamingOnlyRefused(ctx, p, "monitor event include update,state")
}

func fixture06EventMonitorPeer(ctx context.Context, p *sdk.Plugin) error {
	if err := fixture06WaitEOR(ctx, p, 1); err != nil {
		return err
	}
	return streamingOnlyRefused(ctx, p, "monitor event peer 10.0.0.1")
}

// streamingOnlyRefused dispatches command, a streaming-only command, from a
// plugin, which is a route that answers once, and answers nil when the daemon
// refused it naming the terminal command that streams it. Before the refusal
// the daemon answered {"status":"monitor-configured"}: a reply shaped like
// success that carried no events.
func streamingOnlyRefused(ctx context.Context, p *sdk.Plugin, command string) error {
	var raw json.RawMessage
	status, err := Dispatch(ctx, p, command, &raw)
	if err == nil {
		return fmt.Errorf("%s: answered status=%q data=%s, want the streaming-only refusal", command, status, raw)
	}
	if !strings.Contains(err.Error(), "'ze "+command+"'") {
		return fmt.Errorf("%s: refusal %q does not name 'ze %s'", command, err.Error(), command)
	}
	fmt.Fprintf(os.Stderr, "OK: %s refused on a one-shot route: %v\n", command, err)
	return nil
}

func fixture06EventPredicateWait(ctx context.Context, p *sdk.Plugin) error {
	const prefix = "203.0.113.0/24"
	events := make(chan string, 64)
	p.OnEvent(func(event string) error {
		select {
		case events <- event:
		default:
		}
		return nil
	})
	if err := fixture06DispatchDone(ctx, p, "request subscribe bgp event update"); err != nil {
		return err
	}
	announced, _, err := p.UpdateRoute(ctx, "*", "update text origin igp nhop 101.1.101.1 nlri add "+prefix)
	if err != nil {
		return err
	}
	if announced != 1 {
		return fmt.Errorf("announced %d NLRIs, want 1", announced)
	}
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return fmt.Errorf("event predicate never matched an update for %s within 15s", prefix)
		case event := <-events:
			if strings.Contains(event, prefix) {
				if len(event) > 160 {
					event = event[:160]
				}
				fmt.Fprintf(os.Stderr, "OK: event predicate matched the echoed update: %s\n", event)
				return nil
			}
		}
	}
}
