// Design: fail-closed peer queries and bounded PPPoE observations.
// Related: check_client.go -- accel-ppp checker.
// Related: check_ac.go -- pppd checker.
package pppoe

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

var pppLinkPattern = regexp.MustCompile(`^\d+:\s+([^:@]+)`)
var accelSessionPattern = regexp.MustCompile(`\bppp\d+\b`)

func query(
	ctx context.Context,
	lab interoplab.CheckerLab,
	peer string,
	argv []string,
) (string, error) {
	output, err := lab.Query(ctx, peer, argv, nil)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(output) == "" {
		return "", fmt.Errorf("peer %s query returned empty output", peer)
	}
	return output, nil
}

func exec(
	ctx context.Context,
	lab interoplab.CheckerLab,
	peer string,
	argv []string,
) (interoplab.CommandResult, error) {
	result, err := lab.Exec(ctx, peer, argv, nil)
	if err != nil {
		return result, err
	}
	return result, nil
}

func pppLinks(
	ctx context.Context,
	lab interoplab.CheckerLab,
	peer string,
) ([]string, error) {
	result, err := exec(ctx, lab, peer, []string{"ip", "-o", "link", commandShow, "type", "ppp"})
	if err != nil {
		return nil, err
	}
	unique := make(map[string]struct{})
	for line := range strings.SplitSeq(result.Stdout, "\n") {
		match := pppLinkPattern.FindStringSubmatch(line)
		if len(match) == 2 {
			unique[strings.TrimSpace(match[1])] = struct{}{}
		}
	}
	links := make([]string, 0, len(unique))
	for link := range unique {
		links = append(links, link)
	}
	sort.Strings(links)
	return links, nil
}

func pppAddress(
	ctx context.Context,
	lab interoplab.CheckerLab,
	peer string,
	iface string,
) (string, error) {
	result, err := exec(
		ctx,
		lab,
		peer,
		[]string{"ip", "-o", "addr", commandShow, "dev", iface},
	)
	return result.Stdout, err
}

func waitPPPLink(
	ctx context.Context,
	lab interoplab.CheckerLab,
	peer string,
	timeout time.Duration,
	interval time.Duration,
) (string, error) {
	var tb textbuf.Buffer
	description := tb.Str("PPP interface in ").Str(peer).String()
	links, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     timeout,
		Interval:    interval,
		Description: description,
	}, func(probeCtx context.Context) ([]string, error) {
		return pppLinks(probeCtx, lab, peer)
	}, func(found []string) bool {
		return len(found) != 0
	})
	if err != nil {
		return "", err
	}
	return links[0], nil
}

func waitAddress(
	ctx context.Context,
	lab interoplab.CheckerLab,
	peer string,
	iface string,
	local string,
	remote string,
	timeout time.Duration,
	interval time.Duration,
) (string, error) {
	var tb textbuf.Buffer
	description := tb.Str("PPP address on ").Str(iface).String()
	address, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     timeout,
		Interval:    interval,
		Description: description,
	}, func(probeCtx context.Context) (string, error) {
		return pppAddress(probeCtx, lab, peer, iface)
	}, func(output string) bool {
		return strings.Contains(output, local) && strings.Contains(output, remote)
	})
	return address, err
}

func waitRoute(
	ctx context.Context,
	lab interoplab.CheckerLab,
	peer string,
	iface string,
	target string,
	timeout time.Duration,
) (string, error) {
	var tb textbuf.Buffer
	description := tb.Str("PPP route to ").Str(target).String()
	device := tb.Reset().Str("dev ").Str(iface).String()
	route, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     timeout,
		Interval:    time.Second,
		Description: description,
	}, func(probeCtx context.Context) (string, error) {
		result, execErr := exec(
			probeCtx,
			lab,
			peer,
			[]string{"ip", "-o", "route", commandShow, target, "dev", iface},
		)
		return result.Stdout, execErr
	}, func(output string) bool {
		return strings.Contains(output, target) && strings.Contains(output, device)
	})
	return route, err
}

func waitLogsContain(
	ctx context.Context,
	lab interoplab.CheckerLab,
	peer string,
	needle string,
	timeout time.Duration,
) error {
	var tb textbuf.Buffer
	description := tb.Str(needle).Str(" in ").Str(peer).Str(" logs").String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     timeout,
		Interval:    time.Second,
		Description: description,
	}, func(probeCtx context.Context) (interoplab.LogResult, error) {
		return lab.Logs(probeCtx, peer, 1000)
	}, func(logs interoplab.LogResult) bool {
		return logs.Available && strings.Contains(logs.Text, needle)
	})
	return err
}

func pppdRunning(
	ctx context.Context,
	lab interoplab.CheckerLab,
) (bool, error) {
	result, err := lab.Exec(ctx, clientImageName, []string{"pgrep", "-x", pppdExecutable}, nil)
	if err == nil {
		return true, nil
	}
	if result.ExitCode == 1 {
		return false, nil
	}
	return false, err
}

func require(condition bool, message string) error {
	if condition {
		return nil
	}
	return errors.New(message)
}

func appendDiagnostics(
	ctx context.Context,
	lab interoplab.CheckerLab,
	problem error,
	peers ...string,
) error {
	if problem == nil {
		return nil
	}
	var detail textbuf.Buffer
	for _, peer := range peers {
		logs, err := lab.Logs(context.WithoutCancel(ctx), peer, 80)
		if err != nil {
			detail.Str("\n--- ").Str(peer).Str(" logs unavailable: ").Err(err)
			continue
		}
		if !logs.Available {
			detail.Str("\n--- ").Str(peer).Str(" logs unavailable")
			continue
		}
		detail.Str("\n--- ").Str(peer).Str(" logs ---\n").Str(logs.Text)
	}
	return fmt.Errorf("%w%s", problem, detail.String())
}
