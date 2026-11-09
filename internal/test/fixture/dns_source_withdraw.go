// Design: docs/features/dns-resolver.md -- show system dns drops a consumer the engine removed
// Overview: register_dns_source_withdraw.go -- the observer and trigger registered from these functions
// Related: dns_source_operator.go -- showSystemDNSAnswer, showDNSSelection, judgeSelection

package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/internal/core/dnssource"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	// The consumer name the dns-resolver publishes under.
	withdrawResolverConsumer = "dns-resolver"
	// The marker the trigger waits for, and the one it writes once the
	// SIGHUP is sent (reloadTrigger13).
	withdrawInitialMarker = "observer.initial-ok"
	withdrawReloadMarker  = "reload.done"
	withdrawMarkerPolls   = 400
	withdrawMarkerDelay   = 100 * time.Millisecond
)

// showSystemDNSWithdraw proves, through the daemon dispatcher, that a
// dns-resolver the reload removes leaves show system dns: its OnBye handler
// sends the withdrawal while the engine removes it. Before the reload the
// answer lists the resolver's configured list; after it the answer still
// carries the router's selection, so it came from a live registry, and
// carries no dns-resolver consumer.
func showSystemDNSWithdraw(ctx context.Context, p *sdk.Plugin) error {
	if err := pollSystemDNS(ctx, p, "before the reload", judgeResolverListed); err != nil {
		return err
	}
	// Stderr: a plugin's stdout carries the plugin protocol.
	if _, err := fmt.Fprintln(os.Stderr, "OK: show system dns lists the dns-resolver's configured list before the reload"); err != nil {
		return err
	}
	if err := os.WriteFile(withdrawInitialMarker, []byte("ok"), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", withdrawInitialMarker, err)
	}
	if !Poll(ctx, withdrawMarkerPolls, withdrawMarkerDelay, func() bool {
		_, err := os.Stat(withdrawReloadMarker)
		return err == nil
	}) {
		return errors.New("the trigger wrote no " + withdrawReloadMarker + ": no reload was sent")
	}
	if err := pollSystemDNS(ctx, p, "after the reload", judgeResolverWithdrawn); err != nil {
		return err
	}
	_, err := fmt.Fprintln(os.Stderr, "OK: show system dns drops the removed dns-resolver and keeps the router selection")
	return err
}

// pollSystemDNS asks show system dns until judge accepts the answer, and
// names the last verdict and answer when it never does.
func pollSystemDNS(ctx context.Context, p *sdk.Plugin, phase string, judge func(*showSystemDNSAnswer) error) error {
	var answer showSystemDNSAnswer
	verdict := errors.New("show system dns was never answered")
	Poll(ctx, operatorPollAttempts, operatorPollDelay, func() bool {
		answer = showSystemDNSAnswer{}
		if _, err := Dispatch(ctx, p, "show system dns", &answer); err != nil {
			verdict = err
			return false
		}
		verdict = judge(&answer)
		return verdict == nil
	})
	if verdict == nil {
		return nil
	}
	encoded, err := json.Marshal(answer)
	if err != nil {
		return fmt.Errorf("show system dns %s: %w", phase, verdict)
	}
	return fmt.Errorf("show system dns %s: %w; last answer %s", phase, verdict, encoded)
}

// judgeResolverListed names the first expectation the pre-reload answer
// misses: the router's selection, and the resolver's configured list.
func judgeResolverListed(answer *showSystemDNSAnswer) error {
	if showDNSSelection(answer, "system", "") == nil {
		return errors.New("no router selection (consumer system)")
	}
	forward := showDNSSelection(answer, withdrawResolverConsumer, showDNSForwardPath)
	if forward == nil {
		return fmt.Errorf("no %s selection at %q", withdrawResolverConsumer, showDNSForwardPath)
	}
	return judgeSelection(forward, dnssource.ClassConfigured, showDNSForwardServer)
}

// judgeResolverWithdrawn names the first expectation the post-reload answer
// misses: the router's selection is still there, and no dns-resolver
// consumer is, not even one with no selection left.
func judgeResolverWithdrawn(answer *showSystemDNSAnswer) error {
	if showDNSSelection(answer, "system", "") == nil {
		return errors.New("no router selection (consumer system)")
	}
	for i := range answer.Consumers {
		if answer.Consumers[i].Consumer == withdrawResolverConsumer {
			return fmt.Errorf("the removed %s is still listed with %d selection(s)",
				withdrawResolverConsumer, len(answer.Consumers[i].Selections))
		}
	}
	return nil
}
