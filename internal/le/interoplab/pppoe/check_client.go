// Design: docs/architecture/testing/interop.md -- typed Ze client checks against an independent accel-ppp concentrator.
// Related: scenarios.go -- accel-ppp starts before the Ze client.
package pppoe

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	accelGateway = "10.11.0.1"
	zeClientAddr = "10.11.0.2"
	// accelTraceLines bounds the accel-ppp log read. One session's verbose
	// trace is a few dozen lines, so the bound only stops a runaway log.
	accelTraceLines = 2000
)

type accelSessions struct {
	count int
}

// checkZeClient MUST measure CHAP-MD5 authentication in addition to the
// shared session proof, which deliberately does not choose an auth method.
func checkZeClient(ctx context.Context, check *interoplab.CheckContext) error {
	return checkZeClientCHAP(ctx, check, "nameserver 192.0.2.53\nnameserver 192.0.2.54\n")
}

// checkZeClientDNSRejected proves a session against an access concentrator
// that Rejects both DNS options: the client drops them (RFC 1661 Section
// 5.4), IPCP opens, and no learned server reaches resolv.conf.
func checkZeClientDNSRejected(ctx context.Context, check *interoplab.CheckContext) error {
	return checkZeClientCHAP(ctx, check, "")
}

// checkZeClientCHAP proves the CHAP session and that Ze's resolv.conf lists
// exactly resolvers, the servers IPCP learned. The resolvers are read before
// the session check stops the client, which withdraws its lease.
func checkZeClientCHAP(ctx context.Context, check *interoplab.CheckContext, resolvers string) error {
	if err := checkZeClientResolvers(ctx, check, resolvers); err != nil {
		return err
	}
	if err := checkZeClientSession(ctx, check); err != nil {
		return err
	}
	logs, err := check.Lab.Logs(ctx, accelImageName, accelTraceLines)
	if err != nil {
		return fmt.Errorf("read accel-ppp trace: %w", err)
	}
	if !logs.Available {
		return errors.New("accel-ppp trace unavailable: CHAP-MD5 authentication was not measured")
	}
	if err := accelCHAPEvidence(logs.Text); err != nil {
		return appendDiagnostics(ctx, check.Lab, err, accelImageName)
	}
	return nil
}

// checkZeClientSession proves the shared session, dataplane and teardown
// contract. Each caller MUST additionally check its authentication evidence.
func checkZeClientSession(ctx context.Context, check *interoplab.CheckContext) (err error) {
	if check == nil || check.Lab == nil {
		return errors.New("PPPoE Ze-client checker has no lab")
	}
	defer func() {
		err = appendDiagnostics(ctx, check.Lab, err, zeImageName, accelImageName)
	}()

	iface, err := waitPPPLink(
		ctx,
		check.Lab,
		zeImageName,
		75*time.Second,
		2*time.Second,
	)
	if err != nil {
		return fmt.Errorf("ze PPP interface did not come up: %w", err)
	}
	links, err := pppLinks(ctx, check.Lab, zeImageName)
	if err != nil {
		return fmt.Errorf("read Ze PPP links: %w", err)
	}
	var tb textbuf.Buffer
	linkError := tb.Str("expected exactly 1 PPP link in Ze, got ").Int(int64(len(links))).
		Str(": [").Join(links, " ").Byte(']').String()
	if err := require(len(links) == 1, linkError); err != nil {
		return err
	}

	address, err := waitAddress(
		ctx,
		check.Lab,
		zeImageName,
		iface,
		zeClientAddr,
		accelGateway,
		20*time.Second,
		time.Second,
	)
	if err != nil {
		return fmt.Errorf(
			"%s address mismatch: expected local %s peer %s, got %q: %w",
			iface,
			zeClientAddr,
			accelGateway,
			strings.TrimSpace(address),
			err,
		)
	}

	route, err := waitRoute(
		ctx,
		check.Lab,
		zeImageName,
		iface,
		accelGateway,
		20*time.Second,
	)
	if err != nil {
		return fmt.Errorf(
			"ze did not install the PPP route to %s on %s, got %q: %w",
			accelGateway,
			iface,
			strings.TrimSpace(route),
			err,
		)
	}

	active, err := waitAccelSessions(ctx, check.Lab, true, 30*time.Second)
	if err != nil {
		return fmt.Errorf("accel-ppp never reported a subscriber session: %w", err)
	}
	if active.count < 1 {
		return errors.New("accel-ppp participation was not measured")
	}

	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     30 * time.Second,
		Interval:    time.Second,
		Description: "dataplane ping through the PPPoE session",
	}, func(probeCtx context.Context) (interoplab.CommandResult, error) {
		return check.Lab.Exec(
			probeCtx,
			zeImageName,
			[]string{pingCommand, "-c", "3", "-W", "3", accelGateway},
			nil,
		)
	}, func(result interoplab.CommandResult) bool {
		return result.ExitCode == 0
	})
	if err != nil {
		return fmt.Errorf("ze cannot ping AC gateway %s through the PPPoE session: %w", accelGateway, err)
	}

	if err := check.Lab.Stop(ctx, zeImageName, 5); err != nil {
		return fmt.Errorf("stop Ze client: %w", err)
	}
	gone, err := waitAccelSessions(ctx, check.Lab, false, 30*time.Second)
	if err != nil {
		return fmt.Errorf("accel-ppp did not drop the session after client stop: %w", err)
	}
	if gone.count != 0 {
		return fmt.Errorf("accel-ppp still reports %d sessions after client stop", gone.count)
	}
	return nil
}

func accelSessionCount(
	ctx context.Context,
	lab interoplab.CheckerLab,
) (accelSessions, error) {
	output, err := query(
		ctx,
		lab,
		accelImageName,
		[]string{"accel-cmd", commandShow, "sessions"},
	)
	if err != nil {
		return accelSessions{}, err
	}
	count := 0
	for line := range strings.SplitSeq(output, "\n") {
		if accelSessionPattern.MatchString(line) {
			count++
		}
	}
	return accelSessions{count: count}, nil
}

func waitAccelSessions(
	ctx context.Context,
	lab interoplab.CheckerLab,
	wantActive bool,
	timeout time.Duration,
) (accelSessions, error) {
	returnValue, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     timeout,
		Interval:    2 * time.Second,
		Description: "accel-ppp session table",
	}, func(probeCtx context.Context) (accelSessions, error) {
		return accelSessionCount(probeCtx, lab)
	}, func(sessions accelSessions) bool {
		if wantActive {
			return sessions.count >= 1
		}
		return sessions.count == 0
	})
	return returnValue, err
}

// checkZeClientPAP runs the shared session proof against a PAP-only accel-ppp.
// zeResolvConf is the resolv.conf Ze writes by default (system.go,
// ExtractSystemConfig), the one file the learned servers reach.
const zeResolvConf = "/tmp/resolv.conf"

// zeResolvConfProbe reads zeResolvConf ($1) and exits 0 whether or not the
// file exists: it prints zeResolvConfAbsent when Ze wrote none, and
// zeResolvConfPresent then the file's bytes when it did. A plain cat of an
// absent file exits 1, which Lab.Exec answers as a failed probe that Wait
// never shows its predicate, so an absent file could never be read as the
// empty answer a session that learned no server owes.
const zeResolvConfProbe = `if [ -e "$1" ]; then printf 'present\n'; cat "$1"; else printf 'absent\n'; fi`

// The two first lines zeResolvConfProbe prints.
const (
	zeResolvConfPresent = "present\n"
	zeResolvConfAbsent  = "absent\n"
)

// zeResolvConfArgv runs zeResolvConfProbe on zeResolvConf in the Ze container.
var zeResolvConfArgv = []string{"sh", "-c", zeResolvConfProbe, "sh", zeResolvConf}

// checkZeClientResolvers waits until the Ze container's resolv.conf lists
// exactly want after its generated header: the servers the PPPoE client
// learned from IPCP (RFC 1877), primary first. An empty want proves the
// client learned none.
func checkZeClientResolvers(ctx context.Context, check *interoplab.CheckContext, want string) error {
	if check == nil || check.Lab == nil {
		return errors.New("PPPoE Ze-client checker has no lab")
	}
	iface, err := waitPPPLink(ctx, check.Lab, zeImageName, 75*time.Second, 2*time.Second)
	if err != nil {
		return appendDiagnostics(ctx, check.Lab, fmt.Errorf("ze PPP interface did not come up: %w", err), zeImageName, accelImageName)
	}
	// The address is applied once IPCP opened, just before runSession
	// leases the learned servers, so the read below follows a finished IPCP.
	if _, err := waitAddress(ctx, check.Lab, zeImageName, iface, zeClientAddr, accelGateway, 20*time.Second, time.Second); err != nil {
		return appendDiagnostics(ctx, check.Lab, fmt.Errorf("%s never carried the IPCP address: %w", iface, err), zeImageName, accelImageName)
	}
	// With no learned server and no static one, Resolvers never writes the
	// file (Resolvers.flush), so an absent file is the empty answer too.
	expected := "# Generated by ze\n" + want
	var last string
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     20 * time.Second,
		Interval:    time.Second,
		Description: "the IPCP-learned servers in Ze's resolv.conf",
	}, func(probeCtx context.Context) (interoplab.CommandResult, error) {
		return check.Lab.Exec(probeCtx, zeImageName, zeResolvConfArgv, nil)
	}, func(result interoplab.CommandResult) bool {
		if result.Stdout == zeResolvConfAbsent {
			last = "(absent)"
			return want == ""
		}
		body, present := strings.CutPrefix(result.Stdout, zeResolvConfPresent)
		if !present {
			last = "(unrecognized probe output) " + result.Stdout
			return false
		}
		last = body
		return body == expected
	})
	if err != nil {
		return appendDiagnostics(ctx, check.Lab,
			fmt.Errorf("ze resolv.conf %s = %q, want %q: %w", zeResolvConf, last, expected, err), zeImageName, accelImageName)
	}
	return nil
}

// It MUST additionally measure the PAP exchange, not the CHAP-MD5 exchange
// required by checkZeClient.
func checkZeClientPAP(ctx context.Context, check *interoplab.CheckContext) error {
	if err := checkZeClientSession(ctx, check); err != nil {
		return err
	}
	logs, err := check.Lab.Logs(ctx, accelImageName, accelTraceLines)
	if err != nil {
		return fmt.Errorf("read accel-ppp trace: %w", err)
	}
	if !logs.Available {
		return errors.New("accel-ppp trace unavailable: PAP authentication was not measured")
	}
	if err := accelPAPEvidence(logs.Text); err != nil {
		return appendDiagnostics(ctx, check.Lab, err, accelImageName)
	}
	return nil
}

// accelCHAPEvidence requires the ordered negotiation and successful exchange
// from the independent peer, never just the configured authentication module.
// accel-ppp 1.14.0 producers: ppp/ppp_lcp.c (send_conf_req, lcp_recv_conf_ack),
// ppp/ppp_auth.c (auth_print), and auth/auth_chap_md5.c (chap_send_challenge,
// chap_recv_response, chap_send_success).
// https://github.com/accel-ppp/accel-ppp/tree/1.14.0/accel-pppd
func accelCHAPEvidence(trace string) error {
	if strings.Contains(trace, "PAP") {
		return interoplab.WithSearchedOutput(errors.New("auth: PAP appeared in the CHAP-MD5 session"), trace)
	}
	steps := [...]struct {
		packet  string
		detail  string
		missing string
	}{
		{"send [LCP ConfReq ", "<auth CHAP-md5>", "LCP: accel-ppp sent no CHAP-MD5 Configure-Request"},
		{"recv [LCP ConfAck ", "<auth CHAP-md5>", "LCP: Ze sent no CHAP-MD5 Configure-Ack"},
		{"send [CHAP Challenge ", "", "auth: accel-ppp sent no CHAP Challenge"},
		{"recv [CHAP Response ", `name="` + pppdUsername + `"`, "auth: accel-ppp received no named CHAP Response from Ze"},
		{"send [CHAP Success ", `"Authentication succeeded"`, "auth: accel-ppp sent no CHAP Success"},
	}
	next := 0
	for line := range strings.SplitSeq(trace, "\n") {
		if !strings.Contains(line, steps[next].packet) {
			continue
		}
		if !strings.Contains(line, steps[next].detail) {
			continue
		}
		next++
		if next == len(steps) {
			return nil
		}
	}
	return interoplab.WithSearchedOutput(errors.New(steps[next].missing), trace)
}

// accelPAPEvidence proves from accel-ppp's verbose PPP trace that Ze sent a
// PAP Authenticate-Request and accel-ppp acknowledged it, with no CHAP
// exchange at all.
func accelPAPEvidence(trace string) error {
	if !strings.Contains(trace, "recv [PAP AuthReq") {
		return interoplab.WithSearchedOutput(errors.New("auth: accel-ppp received no PAP Authenticate-Request from Ze"), trace)
	}
	if !strings.Contains(trace, "send [PAP AuthAck") {
		return interoplab.WithSearchedOutput(errors.New("auth: accel-ppp did not acknowledge Ze's PAP Authenticate-Request"), trace)
	}
	if strings.Contains(trace, "CHAP") {
		return interoplab.WithSearchedOutput(errors.New("auth: CHAP appeared in a session accel-ppp restricted to PAP"), trace)
	}
	return nil
}
