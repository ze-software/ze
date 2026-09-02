package fixture

import (
	"context"
	"fmt"
	"strings"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func plugin15SubscriberEnricher(ctx context.Context, p *sdk.Plugin) error {
	subscriber := plugin15DispatchUntilDone(ctx, p, "show subscriber")
	if !plugin15Done(subscriber) {
		return fmt.Errorf("show subscriber: status=%s data=%.200s", subscriber.status, subscriber.text())
	}
	summary, err := plugin15Map(subscriber)
	if err != nil {
		return fmt.Errorf("show subscriber json: %w", err)
	}
	if plugin15Number(summary["total"]) != 0 {
		return fmt.Errorf("show subscriber total=%v want 0", summary["total"])
	}
	cos := plugin15DispatchUntilDone(ctx, p, "show class-of-service")
	if !plugin15Done(cos) {
		return fmt.Errorf("show class-of-service: status=%s", cos.status)
	}
	profiles, err := plugin15Slice(cos)
	if err != nil || len(profiles) == 0 {
		return fmt.Errorf("show class-of-service: expected profiles, got %s", cos.text())
	}
	for _, value := range profiles {
		profile, _ := value.(map[string]any)
		if profile["name"] == profileResidential {
			return plugin15WaitEOR(ctx, p, "peer1")
		}
	}
	return fmt.Errorf("show class-of-service: expected residential in %v", profiles)
}

// plugin15SubscriberSummary reads the subscriber surfaces, which say nothing
// about BGP. It still waits for the End-of-RIB first, because
// `subscriber-summary-show.ci` pins that message on its peer and Observe stops
// the daemon the moment this returns: without the wait the peer can be left
// with a Cease notification where the test declared an UPDATE.
func plugin15SubscriberSummary(ctx context.Context, p *sdk.Plugin) error {
	if err := plugin15WaitEOR(ctx, p, "*"); err != nil {
		return err
	}
	r := plugin15DispatchUntilDone(ctx, p, "show subscriber")
	if !plugin15Done(r) {
		return fmt.Errorf("show subscriber: status=%s data=%.200s", r.status, r.text())
	}
	summary, err := plugin15Map(r)
	if err != nil {
		return fmt.Errorf("show subscriber json: %w", err)
	}
	for _, key := range []string{"total", "pppoe", "l2tp"} {
		if plugin15Number(summary[key]) != 0 {
			return fmt.Errorf("show subscriber %s=%v want 0", key, summary[key])
		}
	}
	sessions, _ := summary["sessions"].([]any)
	if len(sessions) != 0 {
		return fmt.Errorf("show subscriber sessions=%d want 0", len(sessions))
	}
	missing := plugin15Dispatch(ctx, p, "show subscriber id detail")
	if missing.status != statusError {
		return fmt.Errorf("show subscriber id detail (no id): expected rejection")
	}
	unknown := plugin15Dispatch(ctx, p, "show subscriber id nonexistent-id detail")
	if unknown.status != statusError {
		return fmt.Errorf("show subscriber detail nonexistent-id: expected status=error got %q", unknown.status)
	}
	if !strings.Contains(unknown.text(), "not found") {
		return fmt.Errorf("show subscriber detail nonexistent-id: error must say not found; got %q", unknown.text())
	}
	return nil
}
