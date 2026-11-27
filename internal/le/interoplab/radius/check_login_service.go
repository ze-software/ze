// Design: docs/architecture/testing/interop.md -- FreeRADIUS admin-login login-service gate scenario (OD-1 option d).
// Overview: radius.go -- the suite, its peers, and the scenario registry these checkers serve.
// Related: checkers.go -- shared SSH login, FreeRADIUS record, and ze log assertions.
// Related: check_unsigned_accept.go -- Prometheus metrics scraping and labeled counter helpers.
//
// OD-1 option (d): system/authentication/radius carries an optional login-service
// leafref. FreeRADIUS still answers Access-Accept on every door carrying the
// unchanged NAS-Port-Id ("ssh" on SSH, "web" on Web), and Ze enforces the
// configured login-service set locally after Access-Accept arrives.
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
	webProbeURL             = "https://127.0.0.1:8443/show/environment/daemon/"
	loginServiceGateMessage = "login refused by login-service gate"
	loginServiceMetricName  = "ze_aaa_login_service_refused_total"
	nasPortIDSSH            = `NAS-Port-Id = "ssh"`
	nasPortIDWeb            = `NAS-Port-Id = "web"`
	webStatusMarker         = "ZE-WEB-HTTP="
)

// checkLoginServiceRefused proves OD-1 option (d) against a real FreeRADIUS
// server: radiusop is admitted on SSH (with Filter-Id profile enforcement) and
// terminally refused on Web (HTTP 401, refusal logged and counted), while
// NAS-Port-Id remains "ssh" and "web" unchanged on the wire.
func checkLoginServiceRefused(ctx context.Context, lab *scenarioLab) error {
	if err := lab.assertLocalControl(ctx, papShape); err != nil {
		return err
	}

	// Assertion 2: positive control on Web using unrestricted localop proves
	// the web HTTPS listener is ready and accepts credentials.
	if err := lab.waitWebStatus(ctx, localUser, localPassword, 200); err != nil {
		return fmt.Errorf("assertion 2: unrestricted localop failed on Web: %w", err)
	}

	// Assertion 3: radiusop is admitted on its included door (SSH) through
	// FreeRADIUS with Filter-Id profile enforcement.
	sshResult, err := lab.login(ctx, radiusUser, papPassword, allowedCommand)
	if err != nil {
		return fmt.Errorf("assertion 3: %w", err)
	}
	if sshResult.ExitCode != 0 {
		return fmt.Errorf("assertion 3: %s could not run %q on included SSH door (exit %d): %s",
			radiusUser, allowedCommand, sshResult.ExitCode, sshResult.Output)
	}
	if err := lab.assertAuthSource(ctx, radiusUser, sourceRADIUS); err != nil {
		return fmt.Errorf("assertion 3: %w", err)
	}
	if err := lab.assertServerRecord(ctx, wantRecord(verdictAccept, radiusUser, papShape)); err != nil {
		return fmt.Errorf("assertion 3: %w", err)
	}
	if err := lab.assertFilterIDProfile(ctx, papPassword); err != nil {
		return fmt.Errorf("assertion 3: %w", err)
	}
	if err := lab.assertServerLogContains(ctx, nasPortIDSSH); err != nil {
		return fmt.Errorf("assertion 3: FreeRADIUS did not observe unchanged %s: %w", nasPortIDSSH, err)
	}

	// Assertion 4: radiusop presents the SAME valid FreeRADIUS password on the
	// excluded Web door and is refused HTTP 401 terminally.
	webStatus, err := lab.webBasicStatus(ctx, radiusUser, papPassword)
	if err != nil {
		return fmt.Errorf("assertion 4: Web Basic Auth probe for %s failed: %w", radiusUser, err)
	}
	if webStatus != 401 {
		return fmt.Errorf("assertion 4: %s on excluded Web door returned HTTP %d, want 401", radiusUser, webStatus)
	}

	// Assertion 5: FreeRADIUS received the Web Access-Request with unchanged
	// NAS-Port-Id = "web" (proving the request reached FreeRADIUS and was
	// refused locally by Ze after Access-Accept, not by an unreachable server).
	if err := lab.assertServerLogContains(ctx, nasPortIDWeb); err != nil {
		return fmt.Errorf("assertion 5: FreeRADIUS did not observe unchanged %s: %w", nasPortIDWeb, err)
	}

	// Assertion 6: Ze logged the login-service gate refusal naming radiusop and web.
	if err := lab.assertLoginServiceRefusedLogged(ctx, radiusUser, "web"); err != nil {
		return fmt.Errorf("assertion 6: %w", err)
	}

	// Assertion 7: Prometheus counter ze_aaa_login_service_refused_total{service="web"} >= 1.
	if err := lab.assertLoginServiceRefusedCounted(ctx, "web"); err != nil {
		return fmt.Errorf("assertion 7: %w", err)
	}

	// Assertion 8: wrong password (local password) still stops chain on SSH.
	return lab.assertRejectStopsChain(ctx, localPassword, papShape, 8)
}

func (l *scenarioLab) webBasicStatus(ctx context.Context, user, password string) (int, error) {
	var tb textbuf.Buffer
	cred := tb.Str(user).Byte(':').Str(password).String()
	script := tb.Reset().
		Str("code=$(curl -sk --connect-timeout 3 --max-time 15 -o /dev/null -w '%{http_code}' -u ").
		Str(shellQuote(cred)).Byte(' ').Str(shellQuote(webProbeURL)).
		Str(" 2>/dev/null || echo 000); echo ").Str(webStatusMarker).Str("\"$code\"").String()
	result, err := l.lab.Exec(ctx, zePeer, []string{"sh", "-c", script}, nil)
	if err != nil {
		return 0, fmt.Errorf("exec web Basic probe: %w", err)
	}
	return parseWebStatusOutput(result.Stdout)
}

func (l *scenarioLab) waitWebStatus(ctx context.Context, user, password string, want int) error {
	var tb textbuf.Buffer
	desc := tb.Str("Web Basic Auth HTTP ").Int(int64(want)).Str(" for ").Str(user).String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     l.timeout,
		Interval:    time.Second,
		Description: desc,
	}, func(probe context.Context) (int, error) {
		return l.webBasicStatus(probe, user, password)
	}, func(code int) bool {
		return code == want
	})
	return err
}

func parseWebStatusOutput(output string) (int, error) {
	for line := range strings.SplitSeq(output, "\n") {
		raw, found := strings.CutPrefix(strings.TrimSpace(line), webStatusMarker)
		if !found {
			continue
		}
		code, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			return 0, fmt.Errorf("web status marker %q is not a number", line)
		}
		return code, nil
	}
	return 0, fmt.Errorf("web probe produced no %s marker: %s", webStatusMarker, output)
}

func (l *scenarioLab) readServerLog(ctx context.Context) (string, error) {
	result, err := l.lab.Logs(ctx, serverPeer, zeLogLines)
	if err != nil {
		return "", err
	}
	if !result.Available {
		return "", errors.New("freeradius container log could not be read")
	}
	return result.Text, nil
}

func (l *scenarioLab) assertServerLogContains(ctx context.Context, needle string) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     l.timeout,
		Interval:    time.Second,
		Description: "FreeRADIUS log carrying " + needle,
	}, l.readServerLog, func(text string) bool {
		return strings.Contains(text, needle)
	})
	return err
}

func (l *scenarioLab) assertLoginServiceRefusedLogged(ctx context.Context, account, service string) error {
	var tb textbuf.Buffer
	wantAccount := tb.Str("account=").Str(account).String()
	wantService := tb.Reset().Str("service=").Str(service).String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     l.timeout,
		Interval:    time.Second,
		Description: "ze log '" + loginServiceGateMessage + "' for " + account + " on " + service,
	}, l.readZeLog, func(text string) bool {
		for line := range strings.SplitSeq(text, "\n") {
			if !strings.Contains(line, loginServiceGateMessage) {
				continue
			}
			if !strings.Contains(line, wantAccount) {
				continue
			}
			if !strings.Contains(line, wantService) {
				continue
			}
			return true
		}
		return false
	})
	return err
}

func (l *scenarioLab) assertLoginServiceRefusedCounted(ctx context.Context, service string) error {
	var tb textbuf.Buffer
	seriesPrefix := tb.Str(loginServiceMetricName).Str(`{service="`).Str(service).Str(`"} `).String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     l.timeout,
		Interval:    time.Second,
		Description: seriesPrefix + "above zero",
	}, l.scrapeMetrics, func(text string) bool {
		val, ok, err := loginServiceMetricValue(text, seriesPrefix)
		if err != nil {
			return false
		}
		if !ok {
			return false
		}
		return val > 0
	})
	return err
}

func loginServiceMetricValue(text, prefix string) (float64, bool, error) {
	for line := range strings.SplitSeq(text, "\n") {
		rest, ok := strings.CutPrefix(line, prefix)
		if !ok {
			continue
		}
		val, err := strconv.ParseFloat(strings.TrimSpace(rest), 64)
		if err != nil {
			return 0, false, fmt.Errorf("parse metric %q: %w", line, err)
		}
		return val, true, nil
	}
	return 0, false, nil
}
