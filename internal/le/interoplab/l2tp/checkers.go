// Design: plan/spec-le-is-a-ze-binary.md -- typed L2TP scenario assertions.
// Related: l2tp.go -- peer lifecycle and exact container configuration.
package l2tp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	localPPPAddress = "10.100.0.1"
	peerPPPAddress  = "10.100.0.2"
	peerPrefix      = "10.100.0.2/32"
	zeAddress       = "172.29.0.2"
	pppLinkType     = "ppp"
)

type labOperations interface {
	Exec(context.Context, string, []string, []interoplab.EnvironmentVariable) (interoplab.CommandResult, error)
	Logs(context.Context, string, int) (interoplab.LogResult, error)
	Signal(context.Context, string, string) error
}

type scenarioCheck func(context.Context, labOperations, time.Duration) error

func checker(check scenarioCheck, timeout time.Duration, diagnosticPeers ...string) interoplab.Checker {
	return func(ctx context.Context, state *interoplab.CheckContext) error {
		if state == nil || state.Lab == nil {
			return errors.New("L2TP checker has no lab")
		}
		err := check(ctx, state.Lab, timeout)
		if err == nil {
			return nil
		}
		return diagnosticError(ctx, state.Lab, err, diagnosticPeers)
	}
}

func checkPPPIPv4(ctx context.Context, lab labOperations, _ time.Duration) error {
	if err := waitPeerLog(ctx, lab, peerZe, "L2TP listener bound", 10*time.Second); err != nil {
		return err
	}
	for _, state := range []string{"session established", "PPP session up", "session IP assigned"} {
		if err := waitPeerLog(ctx, lab, peerZe, state, 60*time.Second); err != nil {
			return err
		}
	}
	iface, err := waitPPPInterface(ctx, lab, peerZe, 60*time.Second)
	if err != nil {
		return err
	}
	address, err := execOutput(ctx, lab, peerZe, []string{"ip", "-o", "addr", commandShow, "dev", iface})
	if err != nil {
		return err
	}
	if !strings.Contains(address, localPPPAddress) || !strings.Contains(address, peerPPPAddress) {
		return fmt.Errorf("%s address mismatch: expected %s peer %s, got %q", iface, localPPPAddress, peerPPPAddress, strings.TrimSpace(address))
	}
	links, err := pppLinks(ctx, lab, peerZe)
	if err != nil {
		return err
	}
	if len(links) != 1 {
		return fmt.Errorf("expected exactly 1 PPP link in Ze, got %d: %v", len(links), links)
	}
	if err := sleepContext(ctx, 2*time.Second); err != nil {
		return err
	}
	if _, err := lab.Exec(ctx, peerLAC, []string{"ping", "-c", "3", "-W", "3", localPPPAddress}, nil); err != nil {
		return fmt.Errorf("LAC cannot ping Ze PPP address %s: %w", localPPPAddress, err)
	}
	if err := requirePeerLog(ctx, lab, peerZe, "subscriber route inject"); err != nil {
		return err
	}
	if err := lab.Signal(ctx, peerLAC, "TERM"); err != nil {
		return fmt.Errorf("stop LAC for teardown: %w", err)
	}
	if err := waitPeerLog(ctx, lab, peerZe, "subscriber routes withdrawn", 30*time.Second); err != nil {
		return err
	}
	return waitL2TPClean(ctx, lab, 30*time.Second)
}

func checkBGPRedistribute(ctx context.Context, lab labOperations, timeout time.Duration) error {
	if err := waitFRRSession(ctx, lab, zeAddress, timeout); err != nil {
		return err
	}
	for _, state := range []struct {
		needle  string
		timeout time.Duration
	}{
		{"L2TP listener bound", 10 * time.Second},
		{"session established", 60 * time.Second},
		{"PPP session up", 60 * time.Second},
		{"session IP assigned", 60 * time.Second},
	} {
		if err := waitPeerLog(ctx, lab, peerZe, state.needle, state.timeout); err != nil {
			return err
		}
	}
	if err := waitPeerLog(ctx, lab, peerLAC, "Connection established", 60*time.Second); err != nil {
		return err
	}
	if _, err := waitPPPInterface(ctx, lab, peerZe, 60*time.Second); err != nil {
		return err
	}
	if err := waitFRRRoute(ctx, lab, peerPrefix, true, 30*time.Second); err != nil {
		return err
	}
	hasRoute, err := frrHasRoute(ctx, lab, peerPrefix)
	if err != nil {
		return err
	}
	if !hasRoute {
		return fmt.Errorf("FRR missing route %s after route readiness", peerPrefix)
	}
	if err := lab.Signal(ctx, peerLAC, "TERM"); err != nil {
		return fmt.Errorf("stop LAC for teardown: %w", err)
	}
	if err := waitPeerLog(ctx, lab, peerZe, "subscriber routes withdrawn", 30*time.Second); err != nil {
		return err
	}
	if err := waitFRRRoute(ctx, lab, peerPrefix, false, 30*time.Second); err != nil {
		return err
	}
	return requireFRRSession(ctx, lab, zeAddress)
}

func checkInitiator(ctx context.Context, lab labOperations, _ time.Duration) error {
	if err := waitPeerLog(ctx, lab, peerLAC, "Listening on IP address", 15*time.Second); err != nil {
		return err
	}
	arguments := []string{
		"wget", "-qO-",
		"--header=Authorization: Bearer secret",
		"--header=Content-Type: application/json",
		"--post-data={\"command\":\"request l2tp outgoing-call remote xl2tpd called 12345\"}",
		"http://127.0.0.1:17012/api/v1/execute",
	}
	result, err := lab.Exec(ctx, peerZe, arguments, nil)
	if err != nil && strings.TrimSpace(result.Stdout+result.Stderr) == "" {
		return fmt.Errorf("trigger outgoing L2TP call: %w", err)
	}
	if err := waitPeerLog(ctx, lab, peerZe, "tunnel now established (initiator)", 15*time.Second); err != nil {
		return err
	}
	if err := waitPeerLog(ctx, lab, peerLAC, "Connection established", 15*time.Second); err != nil {
		return err
	}
	// xl2tpd may log Outgoing-Call-Request before rejecting message type 7. The
	// positive tunnel state on both peers above is the RFC 2661 control proof.
	return nil
}

func checkRadiusAttributes(ctx context.Context, lab labOperations, timeout time.Duration) error {
	if err := waitPeerLog(ctx, lab, peerZe, "session established", timeout); err != nil {
		return err
	}
	if err := waitPeerLog(ctx, lab, peerZe, "session IP assigned", timeout); err != nil {
		return err
	}
	if err := waitPeerLog(ctx, lab, peerLAC, "Connection established", timeout); err != nil {
		return err
	}
	if _, err := waitPPPInterface(ctx, lab, peerZe, timeout); err != nil {
		return err
	}
	access, err := waitRadiusLine(ctx, lab, "Access-Request", timeout)
	if err != nil {
		return err
	}
	authPortID, err := radiusField(access, "NAS-Port-Id")
	if err != nil {
		return err
	}
	matched, err := regexp.MatchString(`^lns1:[0-9]+\.[0-9]+$`, authPortID)
	if err != nil {
		return fmt.Errorf("compile NAS-Port-Id assertion: %w", err)
	}
	if !matched {
		return fmt.Errorf("Access-Request NAS-Port-Id %q does not match lns1:{tunnel-id}.{session-id}", authPortID)
	}
	accounting, err := waitRadiusLine(ctx, lab, "Accounting-Request", timeout)
	if err != nil {
		return err
	}
	if !strings.Contains(accounting, "Acct-Status-Type=Start") {
		return fmt.Errorf("first Accounting-Request is not a Start: %s", accounting)
	}
	var tb textbuf.Buffer
	framedAddress := tb.Str("Framed-IP-Address=").Str(peerPPPAddress).String()
	if !strings.Contains(accounting, framedAddress) {
		return fmt.Errorf("Accounting-Start does not report Framed-IP-Address=%s: %s", peerPPPAddress, accounting)
	}
	accountingPortID, err := radiusField(accounting, "NAS-Port-Id")
	if err != nil {
		return err
	}
	if accountingPortID != authPortID {
		return fmt.Errorf("NAS-Port-Id differs across auth and accounting: %s != %s", authPortID, accountingPortID)
	}
	return nil
}

func waitPeerLog(ctx context.Context, lab labOperations, peer, needle string, timeout time.Duration) error {
	var tb textbuf.Buffer
	description := tb.Str(peer).Str(" log containing ").Str(needle).String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: timeout, Interval: time.Second, Description: description,
	}, func(probeCtx context.Context) (interoplab.LogResult, error) {
		result, err := lab.Logs(probeCtx, peer, 1000)
		if err != nil {
			return interoplab.LogResult{}, err
		}
		if !result.Available {
			return interoplab.LogResult{}, fmt.Errorf("%s logs unavailable", peer)
		}
		return result, nil
	}, func(result interoplab.LogResult) bool { return strings.Contains(result.Text, needle) })
	if err != nil {
		return fmt.Errorf("%s log missing %q: %w", peer, needle, err)
	}
	return nil
}

func requirePeerLog(ctx context.Context, lab labOperations, peer, needle string) error {
	result, err := lab.Logs(ctx, peer, 1000)
	if err != nil {
		return err
	}
	if !result.Available {
		return fmt.Errorf("%s logs unavailable", peer)
	}
	if !strings.Contains(result.Text, needle) {
		return fmt.Errorf("%s log missing %q", peer, needle)
	}
	return nil
}

func waitPPPInterface(ctx context.Context, lab labOperations, peer string, timeout time.Duration) (string, error) {
	var tb textbuf.Buffer
	description := tb.Str(peer).Str(" PPP interface").String()
	links, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: timeout, Interval: 2 * time.Second, Description: description,
	}, func(probeCtx context.Context) ([]string, error) {
		return pppLinks(probeCtx, lab, peer)
	}, func(value []string) bool { return len(value) > 0 })
	if err != nil {
		return "", err
	}
	sort.Strings(links)
	return links[0], nil
}

func pppLinks(ctx context.Context, lab labOperations, peer string) ([]string, error) {
	result, err := lab.Exec(ctx, peer, []string{"ip", "-o", commandLink, commandShow, "type", pppLinkType}, nil)
	if err != nil {
		return nil, err
	}
	links := make([]string, 0, 1)
	for line := range strings.SplitSeq(result.Stdout, "\n") {
		_, body, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		fields := strings.Fields(body)
		if len(fields) == 0 {
			continue
		}
		name := strings.TrimSuffix(strings.SplitN(fields[0], "@", 2)[0], ":")
		if name != "" {
			links = append(links, name)
		}
	}
	return links, nil
}

func execOutput(ctx context.Context, lab labOperations, peer string, arguments []string) (string, error) {
	result, err := lab.Exec(ctx, peer, arguments, nil)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(result.Stdout) == "" {
		return "", fmt.Errorf("peer %s query returned no output", peer)
	}
	return result.Stdout, nil
}

func waitL2TPClean(ctx context.Context, lab labOperations, timeout time.Duration) error {
	type state struct {
		zeLinks, zeTunnels, lacLinks, lacTunnels string
	}
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: timeout, Interval: time.Second, Description: "L2TP and PPP cleanup in both peers",
	}, func(probeCtx context.Context) (state, error) {
		commands := []struct {
			peer string
			args []string
			out  *string
		}{
			{peerZe, []string{"ip", "-o", commandLink, commandShow, "type", "ppp"}, nil},
			{peerZe, []string{"ip", "l2tp", commandShow, "tunnel"}, nil},
			{peerLAC, []string{"ip", "-o", commandLink, commandShow, "type", "ppp"}, nil},
			{peerLAC, []string{"ip", "l2tp", commandShow, "tunnel"}, nil},
		}
		value := state{}
		commands[0].out = &value.zeLinks
		commands[1].out = &value.zeTunnels
		commands[2].out = &value.lacLinks
		commands[3].out = &value.lacTunnels
		for _, command := range commands {
			result, commandErr := lab.Exec(probeCtx, command.peer, command.args, nil)
			if commandErr != nil {
				return state{}, commandErr
			}
			*command.out = result.Stdout
		}
		return value, nil
	}, func(value state) bool {
		return strings.TrimSpace(value.zeLinks+value.zeTunnels+value.lacLinks+value.lacTunnels) == ""
	})
	return err
}

func waitFRRSession(ctx context.Context, lab labOperations, neighbor string, timeout time.Duration) error {
	var tb textbuf.Buffer
	description := tb.Str("FRR BGP session with ").Str(neighbor).String()
	command := tb.Reset().Str(commandShow).Str(" bgp neighbor ").Str(neighbor).String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: timeout, Interval: 2 * time.Second, Description: description,
	}, func(probeCtx context.Context) (string, error) {
		return execOutput(probeCtx, lab, peerFRR, []string{commandVTYSH, "-c", command})
	}, func(output string) bool { return strings.Contains(output, "BGP state = Established") })
	return err
}

func requireFRRSession(ctx context.Context, lab labOperations, neighbor string) error {
	var tb textbuf.Buffer
	command := tb.Str(commandShow).Str(" bgp neighbor ").Str(neighbor).String()
	output, err := execOutput(ctx, lab, peerFRR, []string{commandVTYSH, "-c", command})
	if err != nil {
		return err
	}
	if !strings.Contains(output, "BGP state = Established") {
		return errors.New("BGP session dropped after withdrawal")
	}
	return nil
}

func waitFRRRoute(ctx context.Context, lab labOperations, prefix string, present bool, timeout time.Duration) error {
	var tb textbuf.Buffer
	description := tb.Str("FRR route ").Str(prefix).String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: timeout, Interval: 2 * time.Second, Description: description,
	}, func(probeCtx context.Context) (bool, error) {
		return frrHasRoute(probeCtx, lab, prefix)
	}, func(value bool) bool { return value == present })
	return err
}

func frrHasRoute(ctx context.Context, lab labOperations, prefix string) (bool, error) {
	var tb textbuf.Buffer
	command := tb.Str(commandShow).Str(" bgp ipv4 unicast ").Str(prefix).Str(" json").String()
	output, err := execOutput(ctx, lab, peerFRR, []string{commandVTYSH, "-c", command})
	if err != nil {
		return false, err
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		return false, fmt.Errorf("decode FRR route JSON: %w", err)
	}
	if routeObject(data) {
		return true, nil
	}
	for _, value := range data {
		nested, ok := value.(map[string]any)
		if ok && routeObject(nested) {
			return true, nil
		}
	}
	return false, nil
}

func routeObject(data map[string]any) bool {
	_, hasPaths := data["paths"]
	_, hasPrefix := data["prefix"]
	return hasPaths || hasPrefix
}

func waitRadiusLine(ctx context.Context, lab labOperations, kind string, timeout time.Duration) (string, error) {
	var tb textbuf.Buffer
	prefix := tb.Str("RADIUS-RX ").Str(kind).Byte(' ').String()
	description := tb.Reset().Str(kind).Str(" at RADIUS peer").String()
	line, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: timeout, Interval: time.Second, Description: description,
	}, func(probeCtx context.Context) (string, error) {
		result, err := lab.Logs(probeCtx, peerRadius, 1000)
		if err != nil {
			return "", err
		}
		if !result.Available {
			return "", errors.New("RADIUS logs unavailable")
		}
		for candidate := range strings.SplitSeq(result.Text, "\n") {
			if strings.HasPrefix(candidate, prefix) {
				return candidate, nil
			}
		}
		return "", nil
	}, func(value string) bool { return value != "" })
	if err != nil {
		return "", err
	}
	return line, nil
}

func radiusField(line, name string) (string, error) {
	var tb textbuf.Buffer
	prefix := tb.Str(name).Byte('=').String()
	for field := range strings.FieldsSeq(line) {
		if value, found := strings.CutPrefix(field, prefix); found {
			return value, nil
		}
	}
	return "", fmt.Errorf("%s missing from %s", name, line)
}

func diagnosticError(ctx context.Context, lab labOperations, cause error, peers []string) error {
	var diagnostics textbuf.Buffer
	for _, peer := range peers {
		result, err := lab.Logs(ctx, peer, 80)
		if err != nil {
			diagnostics.Str("\n--- ").Str(peer).Str(" logs unavailable: ").Err(err).Str(" ---")
			continue
		}
		if !result.Available {
			diagnostics.Str("\n--- ").Str(peer).Str(" logs unavailable ---")
			continue
		}
		diagnostics.Str("\n--- ").Str(peer).Str(" logs ---\n").Str(result.Text)
	}
	return fmt.Errorf("%w%s", cause, diagnostics.String())
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
