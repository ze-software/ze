// Design: docs/architecture/testing/interop.md -- the FreeRADIUS admin-login suite, the unsigned EAP answer.
// Overview: radius.go -- the suite, its peers, and the scenario registry these checkers serve.
// Related: checkers.go -- the login, the server record and the ze log readers this file reuses.
// Related: ../../../../test/interop-radius/unsigned-relay.pl -- the relay that removes the server's Message-Authenticator.
// RFC: rfc/short/rfc3579.md
//
// RFC 3579 Section 3.1: "Access-Challenge, Access-Accept, or Access-Reject
// packets including EAP-Message attribute(s) without a Message-Authenticator
// attribute SHOULD be silently discarded by the NAS."
//
// FreeRADIUS signs every such answer and no configuration stops it: rad_encode
// in release_3_2_10 writes the attribute for every Access-Accept,
// Access-Reject and Access-Challenge it sends over UDP. The scenario therefore
// puts the lab relay between ze and the server, and runs the same EAP-MSCHAPv2
// login twice. With the relay passing the answers through, the login completes,
// which proves the relay and its recomputed Response Authenticator. With the
// relay removing only the Message-Authenticator, ze must refuse the answer.
package radius

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	// relayScript is where serverPeerConfig mounts test/interop-radius/unsigned-relay.pl.
	relayScript = "/usr/local/share/ze/unsigned-relay.pl"
	// relayPort is the port ze's scenario config names as its server's port.
	relayPort = "1912"
	// relayUpstream is FreeRADIUS's own listener, which the relay reaches on
	// the lab address because clients.conf authorizes the lab subnet only.
	relayUpstream = "172.27.0.3:1812"
	// relaySecret is the shared secret clients.conf and ze.conf both hold. The
	// relay needs it to recompute the Response Authenticator, RFC 2865
	// Section 3.
	relaySecret   = "ze-interop-fixture-secret" //nolint:gosec // G101: a published lab fixture value that protects nothing.
	relayModePath = "/var/log/freeradius/ze-relay.mode"
	relayLogPath  = "/var/log/freeradius/ze-relay.log"

	relayModePass  = "pass"
	relayModeStrip = "strip"

	// relayListening is the line the relay writes once its socket is bound.
	relayListening = "listening port=" + relayPort

	// unsignedDiscardSeries is the counter ze steps for each discarded answer,
	// internal/component/radius/metrics.go bindRadiusMetrics.
	unsignedDiscardSeries = "ze_radius_unsigned_eap_discards_total"
	metricsURL            = "http://127.0.0.1:9273/metrics"

	// discardMessage and unsignedEAPReason are the log line and the reason
	// internal/component/radius/client.go dispatchResponse writes for this
	// discard. The reason is the String of responseIntegrityUnsignedEAP.
	discardMessage    = "radius: discarding the answer"
	unsignedEAPReason = "EAP-Message without Message-Authenticator"
)

// relayAccessChallenge is RFC 2865 Section 3's code for Access-Challenge, the
// first answer of an EAP conversation and so the first one ze must discard.
const relayAccessChallenge = 11

// relayAccessAccept is RFC 2865 Section 3's code for Access-Accept.
const relayAccessAccept = 2

func checkEAPUnsigned(ctx context.Context, lab *scenarioLab) error {
	if err := lab.startRelay(ctx); err != nil {
		return fmt.Errorf("assertion 1: %w", err)
	}
	if err := lab.assertLocalControl(ctx, eapShape); err != nil {
		return err
	}

	// The positive control: the same server, the same relay, the same login,
	// with the server's signature left in place.
	result, err := lab.login(ctx, radiusUser, eapPassword, allowedCommand)
	if err != nil {
		return fmt.Errorf("assertion 2: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("assertion 2: %s could not log in through the relay passing FreeRADIUS's signed answers (exit %d), so the relay path is broken and a refusal later proves nothing: %s",
			radiusUser, result.ExitCode, result.Output)
	}
	if err := lab.assertAuthSource(ctx, radiusUser, sourceRADIUS); err != nil {
		return fmt.Errorf("assertion 2: %w", err)
	}
	if err := lab.assertServerRecord(ctx, wantRecord(verdictStateEcho, radiusUser, eapShape)); err != nil {
		return fmt.Errorf("assertion 3: %w", err)
	}
	if err := lab.assertServerRecord(ctx, wantRecord(verdictAccept, radiusUser, eapShape)); err != nil {
		return fmt.Errorf("assertion 3: %w", err)
	}
	if err := lab.assertRelayed(ctx, relayModePass, relayAccessAccept, "present"); err != nil {
		return fmt.Errorf("assertion 4: %w", err)
	}
	discards, err := lab.readUnsignedDiscards(ctx)
	if err != nil {
		return fmt.Errorf("assertion 5: %w", err)
	}
	if discards != 0 {
		return fmt.Errorf("assertion 5: %s is %d after a conversation whose every answer was signed", unsignedDiscardSeries, discards)
	}

	text, err := lab.readZeLog(ctx)
	if err != nil {
		return fmt.Errorf("assertion 6: %w", err)
	}
	radiusLogins := radiusSuccesses(text, radiusUser)

	// The refusal: the relay now removes the Message-Authenticator and nothing
	// else, so the only difference from the login above is the attribute.
	if err := lab.setRelayMode(ctx, relayModeStrip); err != nil {
		return fmt.Errorf("assertion 6: %w", err)
	}
	result, err = lab.login(ctx, radiusUser, eapPassword, allowedCommand)
	if err != nil {
		return fmt.Errorf("assertion 6: %w", err)
	}
	if result.ExitCode == 0 {
		return fmt.Errorf("assertion 6: ze completed an EAP login for %s through answers whose Message-Authenticator the relay removed; RFC 3579 Section 3.1 discards them: %s",
			radiusUser, result.Output)
	}
	if err := lab.assertRelayed(ctx, relayModeStrip, relayAccessChallenge, "stripped"); err != nil {
		return fmt.Errorf("assertion 7: %w", err)
	}
	if err := lab.assertUnsignedDiscardLogged(ctx); err != nil {
		return fmt.Errorf("assertion 8: %w", err)
	}
	if err := lab.assertUnsignedDiscardsCounted(ctx); err != nil {
		return fmt.Errorf("assertion 9: %w", err)
	}
	text, err = lab.readZeLog(ctx)
	if err != nil {
		return fmt.Errorf("assertion 10: %w", err)
	}
	if after := radiusSuccesses(text, radiusUser); after != radiusLogins {
		return fmt.Errorf("assertion 10: ze logged %d RADIUS logins for %s after the unsigned answers, want %d", after, radiusUser, radiusLogins)
	}
	return nil
}

// startRelay writes the pass mode, starts the relay in the server's container
// and waits for its bound socket. The mode is written first, so the first
// request the relay meets is never refused for a missing mode.
func (l *scenarioLab) startRelay(ctx context.Context) error {
	if err := l.setRelayMode(ctx, relayModePass); err != nil {
		return err
	}
	argv := []string{"perl", relayScript, relayPort, relayUpstream, relaySecret, relayModePath, relayLogPath}
	if err := l.lab.ExecDetached(ctx, serverPeer, argv, nil); err != nil {
		return fmt.Errorf("start the relay: %w", err)
	}
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     l.timeout,
		Interval:    time.Second,
		Description: "relay log '" + relayListening + "'",
	}, l.readRelayLog, func(text string) bool {
		return strings.Contains(text, relayListening)
	})
	return err
}

func (l *scenarioLab) setRelayMode(ctx context.Context, mode string) error {
	var tb textbuf.Buffer
	script := tb.Str("printf '%s\\n' ").Str(mode).Str(" > ").Str(relayModePath).
		Str(" && cat ").Str(relayModePath).String()
	output, err := l.lab.Query(ctx, serverPeer, []string{"sh", "-c", script}, nil)
	if err != nil {
		return fmt.Errorf("set the relay mode to %s: %w", mode, err)
	}
	if strings.TrimSpace(output) != mode {
		return fmt.Errorf("the relay mode file reads %q after writing %q", strings.TrimSpace(output), mode)
	}
	return nil
}

func (l *scenarioLab) readRelayLog(ctx context.Context) (string, error) {
	return l.lab.Query(ctx, serverPeer, []string{"cat", relayLogPath}, nil)
}

// assertRelayed waits for the relay's record of an answer it carried in mode,
// with the server's code, an EAP-Message, and the Message-Authenticator in the
// named state. It is the server-side evidence that FreeRADIUS answered and
// what reached ze.
func (l *scenarioLab) assertRelayed(ctx context.Context, mode string, code int, signature string) error {
	var tb textbuf.Buffer
	want := tb.Str("relayed mode=").Str(mode).Str(" code=").Int(int64(code)).String()
	tail := tb.Reset().Str("eap-message=present message-authenticator=").Str(signature).String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     l.timeout,
		Interval:    time.Second,
		Description: "relay record '" + want + " ... " + tail + "'",
	}, l.readRelayLog, func(text string) bool {
		return relayRecorded(text, want, tail)
	})
	return err
}

func relayRecorded(text, head, tail string) bool {
	for line := range strings.SplitSeq(text, "\n") {
		if !strings.HasPrefix(line, head+" ") {
			continue
		}
		if strings.HasSuffix(line, tail) {
			return true
		}
	}
	return false
}

func (l *scenarioLab) assertUnsignedDiscardLogged(ctx context.Context) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     l.timeout,
		Interval:    time.Second,
		Description: "ze log '" + discardMessage + "' naming '" + unsignedEAPReason + "'",
	}, l.readZeLog, func(text string) bool {
		for line := range strings.SplitSeq(text, "\n") {
			if !strings.Contains(line, discardMessage) {
				continue
			}
			if strings.Contains(line, unsignedEAPReason) {
				return true
			}
		}
		return false
	})
	return err
}

func (l *scenarioLab) assertUnsignedDiscardsCounted(ctx context.Context) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     l.timeout,
		Interval:    time.Second,
		Description: unsignedDiscardSeries + " above zero",
	}, l.scrapeMetrics, func(text string) bool {
		value, err := unsignedDiscardCount(text)
		if err != nil {
			return false
		}
		return value > 0
	})
	return err
}

func (l *scenarioLab) readUnsignedDiscards(ctx context.Context) (int64, error) {
	text, err := l.scrapeMetrics(ctx)
	if err != nil {
		return 0, err
	}
	return unsignedDiscardCount(text)
}

func (l *scenarioLab) scrapeMetrics(ctx context.Context) (string, error) {
	return l.lab.Query(ctx, zePeer, []string{"wget", "-qO-", metricsURL}, nil)
}

// unsignedDiscardCount reads the unlabeled unsignedDiscardSeries counter out of
// a Prometheus text exposition. A series the exposition does not carry is an
// error, never a zero: a counter ze never registered and one that never
// stepped are different answers.
func unsignedDiscardCount(text string) (int64, error) {
	series := unsignedDiscardSeries
	for line := range strings.SplitSeq(text, "\n") {
		value, found := strings.CutPrefix(line, series+" ")
		if !found {
			continue
		}
		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return 0, fmt.Errorf("%s carries %q, not a number", series, value)
		}
		return int64(parsed), nil
	}
	return 0, errors.New("ze's metrics carry no " + series + " series")
}

// radiusSuccesses counts ze's successful logins for username that the RADIUS
// backend satisfied.
func radiusSuccesses(text, username string) int {
	count := 0
	for _, source := range authSuccessSources(text, username) {
		if source == sourceRADIUS {
			count++
		}
	}
	return count
}
