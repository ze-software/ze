// Design: docs/architecture/testing/interop.md -- the FreeRADIUS admin-login suite, the unsigned PAP answer.
// Overview: radius.go -- the suite, its peers, and the scenario registry these checkers serve.
// Related: check_unsigned_eap.go -- the relay helpers and the metrics scrape this file reuses.
// Related: ../../../../test/interop-radius/unsigned-relay.pl -- the relay that removes the server's Message-Authenticator.
//
// draft-ietf-radext-deprecating-radius-10 Section 4.6: "Response packets
// which do not contain Message-Authenticator MUST be silently discarded."
//
// This is the BlastRADIUS (CVE-2024-3596) defense on the PAP path, where no
// EAP-Message is present and RFC 3579 asks nothing. FreeRADIUS signs every
// Access-Accept it sends, so the relay supplies the unsigned answer an
// attacker would forge: it removes the Message-Authenticator and recomputes
// the Response Authenticator, which leaves an Access-Accept that RFC 2865
// alone would accept. With require-message-authenticator at its default, ze
// must refuse it; on a server row an operator set false, the same answer
// completes the login.
package radius

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	// unsignedResponseSeries is the per-server counter ze steps for each
	// unsigned answer it discards, internal/component/radius/metrics.go
	// bindRadiusMetrics.
	unsignedResponseSeries = "ze_radius_unsigned_response_discards_total"

	// blastRADIUSDiscard is the warning internal/component/radius/client.go
	// discardUnsignedResponse writes for that discard.
	blastRADIUSDiscard = "radius: discarding an answer without Message-Authenticator"
)

// checkPAPUnsigned proves the default server row refuses an Access-Accept
// whose only defect is a missing Message-Authenticator. The pass leg first
// completes the same login through the same relay, so the refusal that
// follows is explained by the attribute alone.
func checkPAPUnsigned(ctx context.Context, lab *scenarioLab) error {
	if err := lab.startRelay(ctx); err != nil {
		return fmt.Errorf("assertion 1: %w", err)
	}
	if err := lab.assertLocalControl(ctx, papShape); err != nil {
		return err
	}

	result, err := lab.login(ctx, radiusUser, papPassword, allowedCommand)
	if err != nil {
		return fmt.Errorf("assertion 2: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("assertion 2: %s could not log in through the relay passing FreeRADIUS's signed Access-Accept (exit %d), so the relay path is broken and a refusal later proves nothing: %s",
			radiusUser, result.ExitCode, result.Output)
	}
	if err := lab.assertAuthSource(ctx, radiusUser, sourceRADIUS); err != nil {
		return fmt.Errorf("assertion 2: %w", err)
	}
	if err := lab.assertServerRecord(ctx, wantRecord(verdictAccept, radiusUser, papShape)); err != nil {
		return fmt.Errorf("assertion 3: %w", err)
	}
	if err := lab.assertRelayed(ctx, relayModePass, relayAccessAccept, relayAbsent, relayPresent); err != nil {
		return fmt.Errorf("assertion 4: %w", err)
	}

	text, err := lab.readZeLog(ctx)
	if err != nil {
		return fmt.Errorf("assertion 5: %w", err)
	}
	if strings.Contains(text, blastRADIUSDiscard) {
		return fmt.Errorf("assertion 5: ze logged %q while every answer was signed", blastRADIUSDiscard)
	}
	radiusLogins := radiusSuccesses(text)

	// The forgery: the relay removes the Message-Authenticator and nothing
	// else, so the only difference from the login above is the attribute.
	if err := lab.setRelayMode(ctx, relayModeStrip); err != nil {
		return fmt.Errorf("assertion 6: %w", err)
	}
	result, err = lab.login(ctx, radiusUser, papPassword, allowedCommand)
	if err != nil {
		return fmt.Errorf("assertion 6: %w", err)
	}
	if result.ExitCode == 0 {
		return fmt.Errorf("assertion 6: ze completed a PAP login for %s on an Access-Accept whose Message-Authenticator the relay removed; draft-ietf-radext-deprecating-radius-10 Section 4.6 discards it: %s",
			radiusUser, result.Output)
	}
	if err := lab.assertRelayed(ctx, relayModeStrip, relayAccessAccept, relayAbsent, relayStripped); err != nil {
		return fmt.Errorf("assertion 7: %w", err)
	}
	if err := lab.assertZeLogged(ctx, blastRADIUSDiscard); err != nil {
		return fmt.Errorf("assertion 8: %w", err)
	}
	if err := lab.assertUnsignedResponsesCounted(ctx); err != nil {
		return fmt.Errorf("assertion 9: %w", err)
	}
	text, err = lab.readZeLog(ctx)
	if err != nil {
		return fmt.Errorf("assertion 10: %w", err)
	}
	if after := radiusSuccesses(text); after != radiusLogins {
		return fmt.Errorf("assertion 10: ze logged %d RADIUS logins for %s after the unsigned Access-Accept, want %d", after, radiusUser, radiusLogins)
	}
	return nil
}

// checkPAPUnsignedLegacy proves the per-server escape hatch: on a row whose
// require-message-authenticator is false, the same stripped Access-Accept
// completes the login, and ze logs no BlastRADIUS discard.
func checkPAPUnsignedLegacy(ctx context.Context, lab *scenarioLab) error {
	if err := lab.startRelay(ctx); err != nil {
		return fmt.Errorf("assertion 1: %w", err)
	}
	if err := lab.setRelayMode(ctx, relayModeStrip); err != nil {
		return fmt.Errorf("assertion 1: %w", err)
	}
	if err := lab.assertLocalControl(ctx, papShape); err != nil {
		return err
	}

	result, err := lab.login(ctx, radiusUser, papPassword, allowedCommand)
	if err != nil {
		return fmt.Errorf("assertion 2: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("assertion 2: %s could not log in on an unsigned Access-Accept from a server row with require-message-authenticator false (exit %d): %s",
			radiusUser, result.ExitCode, result.Output)
	}
	if err := lab.assertAuthSource(ctx, radiusUser, sourceRADIUS); err != nil {
		return fmt.Errorf("assertion 2: %w", err)
	}
	if err := lab.assertServerRecord(ctx, wantRecord(verdictAccept, radiusUser, papShape)); err != nil {
		return fmt.Errorf("assertion 3: %w", err)
	}
	// The login above rode an answer the relay really stripped: without this
	// record a relay left in pass mode would make the leg vacuous.
	if err := lab.assertRelayed(ctx, relayModeStrip, relayAccessAccept, relayAbsent, relayStripped); err != nil {
		return fmt.Errorf("assertion 4: %w", err)
	}
	text, err := lab.readZeLog(ctx)
	if err != nil {
		return fmt.Errorf("assertion 5: %w", err)
	}
	if strings.Contains(text, blastRADIUSDiscard) {
		return fmt.Errorf("assertion 5: ze logged %q for a server row with require-message-authenticator false", blastRADIUSDiscard)
	}
	return nil
}

// assertZeLogged waits for a ze log line carrying message.
func (l *scenarioLab) assertZeLogged(ctx context.Context, message string) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     l.timeout,
		Interval:    time.Second,
		Description: "ze log '" + message + "'",
	}, l.readZeLog, func(text string) bool {
		return strings.Contains(text, message)
	})
	return err
}

func (l *scenarioLab) assertUnsignedResponsesCounted(ctx context.Context) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     l.timeout,
		Interval:    time.Second,
		Description: unsignedResponseSeries + " above zero",
	}, l.scrapeMetrics, func(text string) bool {
		value, err := labeledCounterSum(text, unsignedResponseSeries)
		if err != nil {
			return false
		}
		return value > 0
	})
	return err
}

// labeledCounterSum adds every labeled sample of series in a Prometheus text
// exposition. A series the exposition does not carry is an error, never a
// zero: a counter ze never stepped and one it never registered are different
// answers, and the checker only asks after a discard should have stepped it.
func labeledCounterSum(text, series string) (float64, error) {
	var sum float64
	found := false
	for line := range strings.SplitSeq(text, "\n") {
		rest, ok := strings.CutPrefix(line, series+"{")
		if !ok {
			continue
		}
		_, valueText, cut := strings.CutLast(rest, "}")
		if !cut {
			return 0, fmt.Errorf("%s carries an unterminated label set: %q", series, line)
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(valueText), 64)
		if err != nil {
			return 0, fmt.Errorf("%s carries %q, not a number", series, valueText)
		}
		sum += value
		found = true
	}
	if !found {
		return 0, errors.New("ze's metrics carry no " + series + " sample")
	}
	return sum, nil
}
