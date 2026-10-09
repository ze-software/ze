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

func checkZeClient(ctx context.Context, check *interoplab.CheckContext) (err error) {
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

// checkZeClientPAP runs checkZeClient against an accel-ppp that loads only
// auth_pap, then reads accel-ppp's own trace for the PAP exchange: the session
// coming up proves Ze authenticated, and the trace proves the method was PAP.
func checkZeClientPAP(ctx context.Context, check *interoplab.CheckContext) error {
	if err := checkZeClient(ctx, check); err != nil {
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

// accelPAPEvidence proves from accel-ppp's verbose PPP trace that Ze sent a
// PAP Authenticate-Request and accel-ppp acknowledged it, with no CHAP
// exchange at all.
func accelPAPEvidence(trace string) error {
	if !strings.Contains(trace, "recv [PAP AuthReq") {
		return errors.New("auth: accel-ppp received no PAP Authenticate-Request from Ze")
	}
	if !strings.Contains(trace, "send [PAP AuthAck") {
		return errors.New("auth: accel-ppp did not acknowledge Ze's PAP Authenticate-Request")
	}
	if strings.Contains(trace, "CHAP") {
		return errors.New("auth: CHAP appeared in a session accel-ppp restricted to PAP")
	}
	return nil
}
