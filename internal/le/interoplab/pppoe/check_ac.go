// Design: typed Ze access-concentrator checks against an independent pppd client.
// Related: scenarios.go -- Ze starts before the idle pppd client container.
package pppoe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	zeGateway       = "10.20.0.1"
	firstPoolAddr   = "10.20.0.2"
	pppdUsername    = "alice"
	pppdPassword    = "s3cr3t"
	pppdBadPassword = "wrong-secret"
	pppoeService    = "internet"
	pppdLogPath     = "/var/log/ppp/dial.log"
	zeRESTPort      = 9099
	zeRESTToken     = "ze-pppoe-interop" // #nosec G101 -- fixed fixture token authenticates only the isolated interop Docker network.
)

type zeSession struct {
	SID         int    `json:"sid"`
	ServiceName string `json:"service-name"`
	Interface   string `json:"interface"`
}

type restResponse struct {
	Status string          `json:"status"`
	Data   json.RawMessage `json:"data"`
	Error  string          `json:"error"`
}

type clientObservation struct {
	sessions []zeSession
	links    []string
	address  string
	stopped  bool
	stage    string
}

func checkZeAccessConcentrator(
	ctx context.Context,
	check *interoplab.CheckContext,
) (err error) {
	if check == nil || check.Lab == nil {
		return errors.New("PPPoE access-concentrator checker has no lab")
	}
	defer func() {
		err = appendPPPDLog(ctx, check.Lab, err)
		err = appendDiagnostics(ctx, check.Lab, err, zeImageName, clientImageName)
	}()

	if err := waitLogsContain(
		ctx,
		check.Lab,
		zeImageName,
		"PPPoE interface configured",
		60*time.Second,
	); err != nil {
		return fmt.Errorf("ze PPPoE AC did not bind its access interface: %w", err)
	}
	if err := waitZeRESTReady(ctx, check.Lab, 60*time.Second); err != nil {
		return err
	}

	if err := pppdDial(ctx, check.Lab, pppdUsername, pppdPassword, pppoeService); err != nil {
		return err
	}
	sessions, err := waitZeSession(ctx, check.Lab, 45*time.Second)
	if err != nil {
		return err
	}
	if err := checkDiscoverySession(sessions); err != nil {
		return err
	}

	iface, err := checkLCPAuthIPCP(ctx, check.Lab)
	if err != nil {
		return err
	}
	ping, err := exec(
		ctx,
		check.Lab,
		clientImageName,
		[]string{"ping", "-c", "3", "-W", "3", "-I", iface, zeGateway},
	)
	if err != nil {
		return fmt.Errorf("data: ping Ze gateway %s: %w", zeGateway, err)
	}
	if ping.ExitCode != 0 {
		return fmt.Errorf(
			"data: ICMP did not cross the PPPoE session to %s: %s",
			zeGateway,
			strings.TrimSpace(ping.Stderr),
		)
	}

	if err := checkTeardown(ctx, check.Lab); err != nil {
		return err
	}
	return checkRejectedCredential(ctx, check.Lab)
}

func checkDiscoverySession(sessions []zeSession) error {
	if len(sessions) != 1 {
		return fmt.Errorf("ze allocated %d PPPoE sessions, expected exactly one", len(sessions))
	}
	session := sessions[0]
	if session.SID <= 0 {
		return fmt.Errorf("ze session carries a non-positive session id: %d", session.SID)
	}
	if session.ServiceName != pppoeService {
		return fmt.Errorf("ze recorded Service-Name %q, expected %q", session.ServiceName, pppoeService)
	}
	if session.Interface != "eth0" {
		return fmt.Errorf("ze bound the session to %q, expected eth0", session.Interface)
	}
	return nil
}

func checkLCPAuthIPCP(
	ctx context.Context,
	lab interoplab.CheckerLab,
) (string, error) {
	iface, err := waitClientPPPLink(ctx, lab, 75*time.Second)
	if err != nil {
		return "", err
	}
	address, err := waitClientAddress(ctx, lab, iface, 75*time.Second)
	if err != nil {
		return "", err
	}
	route, err := waitRoute(
		ctx,
		lab,
		clientImageName,
		iface,
		zeGateway,
		20*time.Second,
	)
	if err != nil {
		return "", fmt.Errorf(
			"ipcp: client route to %s on %s missing, got %q: %w",
			zeGateway,
			iface,
			strings.TrimSpace(route),
			err,
		)
	}
	log, err := pppdLog(ctx, lab)
	if err != nil {
		return "", err
	}

	checks := []struct {
		condition bool
		message   string
	}{
		{
			strings.Contains(log, "sent [LCP ConfReq") && strings.Contains(log, "rcvd [LCP ConfAck"),
			"LCP: Ze did not ack the client's Configure-Request",
		},
		{
			strings.Contains(log, "rcvd [LCP ConfReq") && strings.Contains(log, "sent [LCP ConfAck"),
			"LCP: the client did not ack Ze's Configure-Request",
		},
		{
			logLineWith(log, "rcvd [LCP ConfReq", "<mru 1492>") &&
				logLineWith(log, "sent [LCP ConfReq", "<mru 1492>"),
			"LCP: MRU 1492 was not requested in both directions",
		},
		{
			logLineWith(log, "rcvd [LCP ConfReq", "<magic 0x") &&
				logLineWith(log, "sent [LCP ConfReq", "<magic 0x"),
			"LCP: both ends did not offer a magic number",
		},
		{
			logLineWith(log, "rcvd [LCP ConfReq", "<auth chap MD5>"),
			"LCP: Ze's Configure-Request did not demand CHAP-MD5",
		},
		{strings.Contains(log, "rcvd [CHAP Challenge"), "auth: Ze sent no CHAP-MD5 Challenge"},
		{
			logLineWith(log, "sent [CHAP Response", `name = "`+pppdUsername+`"`),
			"auth: the client sent no named CHAP Response",
		},
		{strings.Contains(log, "rcvd [CHAP Success"), "auth: Ze did not accept the CHAP Response"},
		{
			strings.Contains(address, firstPoolAddr) && strings.Contains(address, zeGateway),
			"ipcp: the client did not install Ze's assigned point-to-point address",
		},
	}
	for _, check := range checks {
		if !check.condition {
			return "", errors.New(check.message)
		}
	}
	return iface, nil
}

func checkTeardown(ctx context.Context, lab interoplab.CheckerLab) error {
	if _, err := exec(
		ctx,
		lab,
		clientImageName,
		[]string{"pkill", "-TERM", "-x", pppdExecutable},
	); err != nil {
		return fmt.Errorf("teardown: signal pppd: %w", err)
	}
	if err := waitPPPDExit(ctx, lab, 30*time.Second); err != nil {
		return fmt.Errorf("teardown: pppd did not exit on SIGTERM: %w", err)
	}
	log, err := pppdLog(ctx, lab)
	if err != nil {
		return err
	}
	if !strings.Contains(log, "Sent PADT") {
		return errors.New("teardown: the client did not send PADT")
	}
	if _, err := waitZeSessionsGone(ctx, lab, 30*time.Second); err != nil {
		return fmt.Errorf("teardown: Ze's session table did not become empty: %w", err)
	}
	links, err := pppLinks(ctx, lab, clientImageName)
	if err != nil {
		return err
	}
	if len(links) != 0 {
		return fmt.Errorf("teardown: client PPP interfaces remain: %v", links)
	}
	return nil
}

func checkRejectedCredential(ctx context.Context, lab interoplab.CheckerLab) error {
	if err := pppdDial(
		ctx,
		lab,
		pppdUsername,
		pppdBadPassword,
		pppoeService,
	); err != nil {
		return err
	}
	if err := waitPPPDExit(ctx, lab, 60*time.Second); err != nil {
		return fmt.Errorf("auth-reject: pppd stayed up: %w", err)
	}
	log, err := pppdLog(ctx, lab)
	if err != nil {
		return err
	}
	if !strings.Contains(log, "rcvd [CHAP Challenge") {
		return errors.New("auth-reject: Ze did not challenge the refused dial")
	}
	if !strings.Contains(log, "rcvd [CHAP Failure") {
		return errors.New("auth-reject: Ze did not refuse the wrong CHAP secret")
	}
	if strings.Contains(log, "rcvd [IPCP ConfAck") {
		return errors.New("auth-reject: the refused session reached IPCP")
	}
	links, err := pppLinks(ctx, lab, clientImageName)
	if err != nil {
		return err
	}
	if len(links) != 0 {
		return fmt.Errorf("auth-reject: refused dial left PPP interfaces: %v", links)
	}
	if _, err := waitZeSessionsGone(ctx, lab, 30*time.Second); err != nil {
		return fmt.Errorf("auth-reject: Ze retained the refused session: %w", err)
	}
	return nil
}

func zeSessions(
	ctx context.Context,
	lab interoplab.CheckerLab,
) ([]zeSession, error) {
	command := "show pppoe sessions"
	var tb textbuf.Buffer
	endpoint := tb.Str("http://127.0.0.1:").Int(int64(zeRESTPort)).
		Str("/api/v1/execute").String()
	authorization := tb.Reset().Str("Authorization: Bearer ").Str(zeRESTToken).String()
	request := tb.Reset().Str(`{"command": `).Quoted(command).Byte('}').String()
	body, err := query(ctx, lab, zeImageName, []string{
		"curl",
		"-sS",
		"--fail-with-body",
		"-X",
		"POST",
		endpoint,
		"-H",
		authorization,
		"-H",
		"Content-Type: application/json",
		"-d",
		request,
	})
	if err != nil {
		return nil, fmt.Errorf("REST %s unreachable: %w", command, err)
	}
	var response restResponse
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		return nil, fmt.Errorf("REST %s returned non-JSON: %.200s", command, body)
	}
	if response.Status == "error" {
		return nil, fmt.Errorf("REST %s: %s", command, response.Error)
	}
	if len(response.Data) == 0 || string(response.Data) == "null" {
		return []zeSession{}, nil
	}
	var sessions []zeSession
	if err := json.Unmarshal(response.Data, &sessions); err != nil {
		return nil, fmt.Errorf("show pppoe sessions returned a non-list: %s", response.Data)
	}
	return sessions, nil
}

func waitZeRESTReady(
	ctx context.Context,
	lab interoplab.CheckerLab,
	timeout time.Duration,
) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     timeout,
		Interval:    time.Second,
		Description: "Ze REST API",
	}, func(probeCtx context.Context) ([]zeSession, error) {
		return zeSessions(probeCtx, lab)
	}, func([]zeSession) bool { return true })
	if err != nil {
		return fmt.Errorf("ze REST API never answered: %w", err)
	}
	return nil
}

func waitZeSession(
	ctx context.Context,
	lab interoplab.CheckerLab,
	timeout time.Duration,
) ([]zeSession, error) {
	observation, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     timeout,
		Interval:    time.Second,
		Description: "Ze PPPoE session allocation",
	}, func(probeCtx context.Context) (clientObservation, error) {
		sessions, sessionErr := zeSessions(probeCtx, lab)
		if sessionErr != nil {
			return clientObservation{}, sessionErr
		}
		if len(sessions) != 0 {
			return clientObservation{sessions: sessions}, nil
		}
		running, runningErr := pppdRunning(probeCtx, lab)
		if runningErr != nil {
			return clientObservation{}, runningErr
		}
		if running {
			return clientObservation{}, nil
		}
		log, logErr := pppdLogMeasured(probeCtx, lab)
		if logErr != nil {
			return clientObservation{}, logErr
		}
		return clientObservation{stopped: true, stage: pppdFailureStage(log)}, nil
	}, func(value clientObservation) bool {
		return len(value.sessions) != 0 || value.stopped
	})
	if err != nil {
		return nil, err
	}
	if observation.stopped {
		return nil, fmt.Errorf("pppd exited before Ze allocated a session: %s", observation.stage)
	}
	return observation.sessions, nil
}

func waitZeSessionsGone(
	ctx context.Context,
	lab interoplab.CheckerLab,
	timeout time.Duration,
) ([]zeSession, error) {
	returnValue, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     timeout,
		Interval:    time.Second,
		Description: "empty Ze PPPoE session table",
	}, func(probeCtx context.Context) ([]zeSession, error) {
		return zeSessions(probeCtx, lab)
	}, func(sessions []zeSession) bool {
		return len(sessions) == 0
	})
	return returnValue, err
}

func pppdDial(
	ctx context.Context,
	lab interoplab.CheckerLab,
	username string,
	password string,
	service string,
) error {
	var tb textbuf.Buffer
	clearLog := tb.Str("rm -f ").Str(pppdLogPath).String()
	if _, err := exec(
		ctx,
		lab,
		clientImageName,
		[]string{"sh", "-c", clearLog},
	); err != nil {
		return fmt.Errorf("clear pppd log: %w", err)
	}
	arguments := []string{
		pppdExecutable,
		"plugin",
		"pppoe.so",
		"nic-eth0",
		"user",
		username,
		"password",
		password,
		"noauth",
		"refuse-pap",
		"refuse-eap",
		"refuse-mschap",
		"refuse-mschap-v2",
		"noipdefault",
		"nodefaultroute",
		"noaccomp",
		"nopcomp",
		"mtu",
		"1492",
		"mru",
		"1492",
		"lcp-echo-interval",
		"10",
		"lcp-echo-failure",
		"5",
		"maxfail",
		"1",
		"nodetach",
		"debug",
	}
	if service != "" {
		arguments = append(arguments, "rp_pppoe_service", service)
	}
	shell := tb.Reset().Str("exec ").Join(arguments, " ").Str(" >").
		Str(pppdLogPath).Str(" 2>&1").String()
	if err := lab.ExecDetached(
		ctx,
		clientImageName,
		[]string{"sh", "-c", shell},
		nil,
	); err != nil {
		return fmt.Errorf("start pppd: %w", err)
	}
	return nil
}

func pppdLog(ctx context.Context, lab interoplab.CheckerLab) (string, error) {
	var tb textbuf.Buffer
	command := tb.Str("cat ").Str(pppdLogPath).Str(" 2>/dev/null").String()
	return query(
		ctx,
		lab,
		clientImageName,
		[]string{"sh", "-c", command},
	)
}

func pppdLogMeasured(ctx context.Context, lab interoplab.CheckerLab) (string, error) {
	var tb textbuf.Buffer
	command := tb.Str("cat ").Str(pppdLogPath).Str(" 2>/dev/null").String()
	result, err := exec(
		ctx,
		lab,
		clientImageName,
		[]string{"sh", "-c", command},
	)
	return result.Stdout, err
}

func appendPPPDLog(
	ctx context.Context,
	lab interoplab.CheckerLab,
	problem error,
) error {
	if problem == nil {
		return nil
	}
	log, err := pppdLogMeasured(context.WithoutCancel(ctx), lab)
	if err != nil {
		return fmt.Errorf("%w\n--- pppd log unavailable: %w", problem, err)
	}
	return fmt.Errorf("%w\n--- pppd log ---\n%s", problem, log)
}

func pppdFailureStage(log string) string {
	switch {
	case !strings.Contains(log, "PADO"):
		return "discovery: the AC sent no PADO"
	case !strings.Contains(log, "PADS"):
		return "discovery: the AC sent no PADS"
	case strings.Contains(log, "rcvd [CHAP Failure") || strings.Contains(log, "authentication failed"):
		return "auth: the AC refused the credential"
	case !strings.Contains(log, "rcvd [LCP ConfReq"):
		return "lcp: the AC never sent its own Configure-Request"
	case !strings.Contains(log, "sent [LCP ConfAck") || !strings.Contains(log, "rcvd [LCP ConfAck"):
		return "lcp: a Configure-Ack is missing in one direction"
	case !strings.Contains(log, "rcvd [CHAP Challenge") && !strings.Contains(log, "sent [PAP AuthReq"):
		return "auth: the AC asked for a method and never started it"
	case !strings.Contains(log, "rcvd [IPCP ConfAck"):
		return "ipcp: no address was agreed"
	default:
		return "after IPCP"
	}
}

func waitClientPPPLink(
	ctx context.Context,
	lab interoplab.CheckerLab,
	timeout time.Duration,
) (string, error) {
	observation, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     timeout,
		Interval:    time.Second,
		Description: "PPP interface in the pppd client",
	}, func(probeCtx context.Context) (clientObservation, error) {
		links, linkErr := pppLinks(probeCtx, lab, clientImageName)
		if linkErr != nil {
			return clientObservation{}, linkErr
		}
		if len(links) != 0 {
			return clientObservation{links: links}, nil
		}
		running, runningErr := pppdRunning(probeCtx, lab)
		if runningErr != nil {
			return clientObservation{}, runningErr
		}
		if running {
			return clientObservation{}, nil
		}
		log, logErr := pppdLogMeasured(probeCtx, lab)
		if logErr != nil {
			return clientObservation{}, logErr
		}
		return clientObservation{stopped: true, stage: pppdFailureStage(log)}, nil
	}, func(value clientObservation) bool {
		return len(value.links) != 0 || value.stopped
	})
	if err != nil {
		return "", err
	}
	if observation.stopped {
		return "", fmt.Errorf("pppd exited before a PPP interface appeared: %s", observation.stage)
	}
	return observation.links[0], nil
}

func waitClientAddress(
	ctx context.Context,
	lab interoplab.CheckerLab,
	iface string,
	timeout time.Duration,
) (string, error) {
	observation, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     timeout,
		Interval:    time.Second,
		Description: "address assigned by Ze over IPCP",
	}, func(probeCtx context.Context) (clientObservation, error) {
		address, addressErr := pppAddress(probeCtx, lab, clientImageName, iface)
		if addressErr != nil {
			return clientObservation{}, addressErr
		}
		if strings.Contains(address, firstPoolAddr) && strings.Contains(address, zeGateway) {
			return clientObservation{address: address}, nil
		}
		running, runningErr := pppdRunning(probeCtx, lab)
		if runningErr != nil {
			return clientObservation{}, runningErr
		}
		if running {
			return clientObservation{address: address}, nil
		}
		log, logErr := pppdLogMeasured(probeCtx, lab)
		if logErr != nil {
			return clientObservation{}, logErr
		}
		return clientObservation{
			address: address,
			stopped: true,
			stage:   pppdFailureStage(log),
		}, nil
	}, func(value clientObservation) bool {
		assigned := strings.Contains(value.address, firstPoolAddr) &&
			strings.Contains(value.address, zeGateway)
		return assigned || value.stopped
	})
	if err != nil {
		return observation.address, err
	}
	if observation.stopped {
		return observation.address, fmt.Errorf(
			"pppd exited before it installed an address: %s",
			observation.stage,
		)
	}
	return observation.address, nil
}

func waitPPPDExit(
	ctx context.Context,
	lab interoplab.CheckerLab,
	timeout time.Duration,
) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     timeout,
		Interval:    time.Second,
		Description: "pppd exit",
	}, func(probeCtx context.Context) (bool, error) {
		return pppdRunning(probeCtx, lab)
	}, func(running bool) bool { return !running })
	return err
}

func logLineWith(log, prefix, needle string) bool {
	for line := range strings.SplitSeq(log, "\n") {
		if strings.HasPrefix(line, prefix) && strings.Contains(line, needle) {
			return true
		}
	}
	return false
}
