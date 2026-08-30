// Design: docs/architecture/testing/interop.md -- fail-closed strongSwan and Ze queries with bounded observations.
// Related: ipsec.go -- topology and rendered configuration.
// Related: checkers.go -- protocol assertions built from these typed operations.
package ipsec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	zePeer   = "ze"
	swanPeer = "strongswan"
	frrPeer  = "frr"

	zeIP   = "172.28.0.2"
	swanIP = "172.28.0.3"
	frrIP  = "172.28.0.4"

	logLinesMax = 10000

	childWaitTimeout    = 30 * time.Second
	selectorWaitTimeout = 90 * time.Second
	xfrmWaitTimeout     = 30 * time.Second

	swanConnection = "ze"
)

var (
	pkiPlaceholder = regexp.MustCompile(`%%PKI_B64:([A-Za-z0-9._-]+)%%`)
	pemHeader      = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]*:`)
	spiPattern     = regexp.MustCompile(`proto \w+ spi (0x[0-9a-fA-F]+)`)
	espSPIPattern  = regexp.MustCompile(`proto esp spi (0x[0-9a-fA-F]+)`)
	xfrmBytes      = regexp.MustCompile(`(\d+)\(bytes\)`)
	srcDstPattern  = regexp.MustCompile(`^src (\S+) dst (\S+)`)
)

type scenarioLab struct {
	check   *interoplab.CheckContext
	timeout time.Duration
	state   *scenarioState
}

func newScenarioLab(check *interoplab.CheckContext, timeout time.Duration, state *scenarioState) *scenarioLab {
	return &scenarioLab{check: check, timeout: timeout, state: state}
}

func (l *scenarioLab) exec(ctx context.Context, peer string, command ...string) (string, error) {
	result, err := l.check.Lab.Exec(ctx, peer, command, nil)
	if err != nil {
		return result.Stdout, err
	}
	return result.Stdout, nil
}

func (l *scenarioLab) execQuiet(ctx context.Context, peer string, command ...string) string {
	result, _ := l.check.Lab.Exec(ctx, peer, command, nil)
	return result.Stdout
}

func (l *scenarioLab) query(ctx context.Context, peer string, command ...string) (string, error) {
	return l.check.Lab.Query(ctx, peer, command, nil)
}

func (l *scenarioLab) logs(ctx context.Context, peer string) (string, error) {
	result, err := l.check.Lab.Logs(ctx, peer, logLinesMax)
	if err != nil {
		return "", err
	}
	if !result.Available {
		return "", fmt.Errorf("%s logs were not read", peer)
	}
	if strings.TrimSpace(result.Text) == "" {
		return "", fmt.Errorf("%s logs are empty; checker read no peer output", peer)
	}
	return result.Text, nil
}

func (l *scenarioLab) waitLog(ctx context.Context, peer, needle string, timeout time.Duration) error {
	var tb textbuf.Buffer
	description := tb.Str(peer).Str(" log ").Quoted(needle).String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: timeout, Interval: 2 * time.Second, Description: description,
	}, func(probe context.Context) (string, error) {
		return l.logs(probe, peer)
	}, func(logs string) bool {
		return strings.Contains(logs, needle)
	})
	return err
}

func (l *scenarioLab) listSAs(ctx context.Context) (string, error) {
	return l.exec(ctx, swanPeer, "swanctl", "--list-sas")
}

func (l *scenarioLab) waitSA(ctx context.Context, timeout time.Duration) error {
	var tb textbuf.Buffer
	description := tb.Str("strongSwan SA ").Str(swanConnection).String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: timeout, Interval: 2 * time.Second, Description: description,
	}, func(probe context.Context) (string, error) {
		return l.listSAs(probe)
	}, func(output string) bool {
		if !strings.Contains(output, "ESTABLISHED") {
			return false
		}
		return strings.Contains(output, swanConnection)
	})
	return err
}

func (l *scenarioLab) waitChild(ctx context.Context, child string) error {
	var tb textbuf.Buffer
	description := tb.Str("strongSwan Child SA ").Str(child).String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: childWaitTimeout, Interval: 2 * time.Second, Description: description,
	}, func(probe context.Context) (string, error) {
		return l.listSAs(probe)
	}, func(output string) bool {
		if child == "" {
			return strings.Contains(output, "INSTALLED")
		}
		if !strings.Contains(output, child) {
			return false
		}
		return strings.Contains(output, "INSTALLED")
	})
	return err
}

func (l *scenarioLab) waitChildSelectors(ctx context.Context, local, remote string) error {
	var tb textbuf.Buffer
	localSelector := tb.Str("local  ").Str(local).String()
	remoteSelector := tb.Reset().Str("remote ").Str(remote).String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: selectorWaitTimeout, Interval: 2 * time.Second, Description: "strongSwan Child SA selectors",
	}, func(probe context.Context) (string, error) {
		return l.listSAs(probe)
	}, func(output string) bool {
		if !strings.Contains(output, localSelector) {
			return false
		}
		return strings.Contains(output, remoteSelector)
	})
	return err
}

func (l *scenarioLab) waitXFRM(ctx context.Context, peer string) (string, error) {
	return l.waitOutput(ctx, peer, []string{"ip", "xfrm", "state"}, xfrmWaitTimeout, "XFRM ESP state", func(output string) bool {
		return strings.Contains(output, "proto esp")
	})
}

func (l *scenarioLab) waitOutput(ctx context.Context, peer string, command []string, timeout time.Duration, description string, ready func(string) bool) (string, error) {
	output, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: timeout, Interval: 2 * time.Second, Description: description,
	}, func(probe context.Context) (string, error) {
		return l.exec(probe, peer, command...)
	}, ready)
	return output, err
}

func (l *scenarioLab) xfrmState(ctx context.Context, peer string) (string, error) {
	return l.exec(ctx, peer, "ip", "xfrm", "state")
}

func (l *scenarioLab) xfrmPolicy(ctx context.Context, peer string) (string, error) {
	output, err := l.exec(ctx, peer, "ip", "xfrm", "policy")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(output) == "" {
		return "", fmt.Errorf("ip xfrm policy printed nothing in %s; checker read no policy state", peer)
	}
	return output, nil
}

func (l *scenarioLab) xfrmCounters(ctx context.Context, peer string) (map[string]uint64, error) {
	output, err := l.exec(ctx, peer, "ip", "-s", "xfrm", "state")
	if err != nil {
		return nil, err
	}
	return parseXFRMCounters(output), nil
}

func parseXFRMCounters(output string) map[string]uint64 {
	counters := make(map[string]uint64)
	spi := ""
	current := false
	for line := range strings.SplitSeq(output, "\n") {
		if line != "" && line[0] != ' ' && line[0] != '\t' {
			spi = ""
			current = false
		}
		if match := spiPattern.FindStringSubmatch(line); match != nil {
			spi = match[1]
			current = false
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "lifetime current") {
			current = true
			continue
		}
		if strings.HasPrefix(trimmed, "lifetime config") || strings.HasPrefix(trimmed, "stats") {
			current = false
			continue
		}
		if !current || spi == "" {
			continue
		}
		for _, match := range xfrmBytes.FindAllStringSubmatch(line, -1) {
			value, err := strconv.ParseUint(match[1], 10, 64)
			if err == nil {
				counters[spi] += value
			}
		}
	}
	return counters
}

func assertESPAdvanced(before, after map[string]uint64, who string) error {
	common := make([]string, 0)
	for spi, earlier := range before {
		later, ok := after[spi]
		if !ok {
			continue
		}
		common = append(common, spi)
		if later > earlier {
			return nil
		}
	}
	sort.Strings(common)
	return fmt.Errorf("%s (before=%v after=%v, common SPIs %v)", who, before, after, common)
}

func (l *scenarioLab) checkXFRMCount(ctx context.Context, peer string, expected int) error {
	output, err := l.xfrmState(ctx, peer)
	if err != nil {
		return err
	}
	count := strings.Count(output, "proto esp")
	if count != expected {
		return fmt.Errorf("%s XFRM SA count %d != %d", peer, count, expected)
	}
	return nil
}

func (l *scenarioLab) ping(ctx context.Context, peer, target string, count int) string {
	return l.execQuiet(ctx, peer, "ping", "-c", strconv.Itoa(count), "-W", "2", target)
}

func (l *scenarioLab) verifyTunnelTraffic(ctx context.Context, message string) error {
	zeBefore, err := l.xfrmCounters(ctx, zePeer)
	if err != nil {
		return err
	}
	swanBefore, err := l.xfrmCounters(ctx, swanPeer)
	if err != nil {
		return err
	}
	l.ping(ctx, zePeer, swanIP, 4)
	zeAfter, err := l.xfrmCounters(ctx, zePeer)
	if err != nil {
		return err
	}
	if err := assertESPAdvanced(zeBefore, zeAfter, message); err != nil {
		return err
	}
	swanAfter, err := l.xfrmCounters(ctx, swanPeer)
	if err != nil {
		return err
	}
	return assertESPAdvanced(swanBefore, swanAfter, "strongSwan accepted no ESP from Ze")
}

func (l *scenarioLab) espSPIs(ctx context.Context, peer string) (map[string]struct{}, error) {
	output, err := l.xfrmState(ctx, peer)
	if err != nil {
		return nil, err
	}
	matches := espSPIPattern.FindAllStringSubmatch(output, -1)
	spis := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		spis[match[1]] = struct{}{}
	}
	return spis, nil
}

func sameStrings(first, second map[string]struct{}) bool {
	if len(first) != len(second) {
		return false
	}
	for value := range first {
		if _, ok := second[value]; !ok {
			return false
		}
	}
	return true
}

func newStrings(before, after map[string]struct{}) bool {
	for value := range after {
		if _, ok := before[value]; !ok {
			return true
		}
	}
	return false
}

func (l *scenarioLab) espPolicyPairs(ctx context.Context, peer string) (map[string]struct{}, error) {
	output, err := l.xfrmPolicy(ctx, peer)
	if err != nil {
		return nil, err
	}
	pairs := make(map[string]struct{})
	source := ""
	destination := ""
	hasESP := false
	var tb textbuf.Buffer
	keep := func() {
		if source == "" || !hasESP {
			return
		}
		pair := []string{source, destination}
		sort.Strings(pair)
		key := tb.Reset().Str(pair[0]).Byte('|').Str(pair[1]).String()
		pairs[key] = struct{}{}
	}
	for line := range strings.SplitSeq(output, "\n") {
		if match := srcDstPattern.FindStringSubmatch(line); match != nil {
			keep()
			source = match[1]
			destination = match[2]
			hasESP = false
			continue
		}
		if source != "" && strings.Contains(line, "proto esp") {
			hasESP = true
		}
	}
	keep()
	return pairs, nil
}

func policyPair(first, second string) string {
	values := []string{first, second}
	sort.Strings(values)
	var tb textbuf.Buffer
	return tb.Str(values[0]).Byte('|').Str(values[1]).String()
}

func (l *scenarioLab) waitPolicyPair(ctx context.Context, peer, first, second string, timeout time.Duration) error {
	expected := policyPair(first, second)
	var tb textbuf.Buffer
	description := tb.Str(peer).Str(" ESP policy ").Str(expected).String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: timeout, Interval: 2 * time.Second, Description: description,
	}, func(probe context.Context) (map[string]struct{}, error) {
		return l.espPolicyPairs(probe, peer)
	}, func(pairs map[string]struct{}) bool {
		_, ok := pairs[expected]
		return ok && len(pairs) == 1
	})
	return err
}

func (l *scenarioLab) zeCLI(ctx context.Context, command string) (string, error) {
	var tb textbuf.Buffer
	seed := tb.Str("printf '%s\\n%s\\n127.0.0.1\\n%s\\n' ").Str(zeCLIUser).Byte(' ').
		Str(zeCLIPassword).Byte(' ').Str(zeCLIPort).Str(" | ZE_CONFIG_DIR=").
		Str(zeCLIStore).Str(" ze init").String()
	run := tb.Reset().Str("ZE_CONFIG_DIR=").Str(zeCLIStore).Str(" ZE_SSH_PASSWORD=").
		Str(zeCLIPassword).Str(" ze cli -c ").Str(shellQuote(command)).String()
	shell := tb.Reset().Str("[ -d ").Str(zeCLIStore).Str(" ] || ").Str(seed).
		Str(" >/dev/null; ").Str(run).String()
	return l.query(ctx, zePeer, "sh", "-c", shell)
}

func shellQuote(value string) string {
	var tb textbuf.Buffer
	tb.Byte('\'')
	for index := range len(value) {
		if value[index] == '\'' {
			tb.Str("'\\''")
			continue
		}
		tb.Byte(value[index])
	}
	return tb.Byte('\'').String()
}

func (l *scenarioLab) assertZeSelectors(ctx context.Context, local, remote string) error {
	output, err := l.zeCLI(ctx, "show vpn ipsec sa")
	if err != nil {
		return err
	}
	for field, value := range map[string]string{"ts-local": local, "ts-remote": remote} {
		pattern := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(field) + `\s+` + regexp.QuoteMeta(value) + `\s*$`)
		if !pattern.MatchString(output) {
			return fmt.Errorf("show vpn ipsec sa does not report %s %s; output: %s", field, value, output)
		}
	}
	return nil
}

func (l *scenarioLab) reloadZe(ctx context.Context, source string) error {
	if l.state.renderedConfig == "" {
		return errors.New("scenario rendered config path is empty")
	}
	pkiDir := findPKIDir(l.state.root, l.check.Source.Directory)
	if err := renderZeConfig(source, pkiDir, l.state.renderedConfig); err != nil {
		return err
	}
	return l.check.Lab.Signal(ctx, zePeer, "HUP")
}

func (l *scenarioLab) breakLink(ctx context.Context) error {
	_, err := l.exec(ctx, swanPeer, "iptables", "-I", "OUTPUT", "1", "-d", zeIP, "-j", "DROP")
	return err
}

func (l *scenarioLab) restoreLink(ctx context.Context) {
	l.execQuiet(ctx, swanPeer, "iptables", "-D", "OUTPUT", "-d", zeIP, "-j", "DROP")
}

func (l *scenarioLab) frrOutput(ctx context.Context, command string) (string, error) {
	return l.exec(ctx, frrPeer, "vtysh", "-c", command)
}

func (l *scenarioLab) waitFRRSession(ctx context.Context, neighbor string) error {
	var tb textbuf.Buffer
	description := tb.Str("FRR session ").Str(neighbor).String()
	command := tb.Reset().Str("show bgp neighbor ").Str(neighbor).String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: l.timeout, Interval: 2 * time.Second, Description: description,
	}, func(probe context.Context) (string, error) {
		return l.frrOutput(probe, command)
	}, func(output string) bool {
		return strings.Contains(output, "BGP state = Established")
	})
	return err
}

func frrHasRoute(output string) bool {
	if strings.TrimSpace(output) == "" {
		return false
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		return false
	}
	if _, ok := payload["paths"]; ok {
		return true
	}
	if _, ok := payload["prefix"]; ok {
		return true
	}
	for _, value := range payload {
		entry, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := entry["paths"]; ok {
			return true
		}
		if _, ok := entry["prefix"]; ok {
			return true
		}
	}
	return false
}

func (l *scenarioLab) waitFRRRoute(ctx context.Context, prefix string, present bool) error {
	var tb textbuf.Buffer
	description := tb.Str("FRR route ").Str(prefix).String()
	command := tb.Reset().Str("show bgp ipv4 unicast ").Str(prefix).Str(" json").String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 30 * time.Second, Interval: 2 * time.Second, Description: description,
	}, func(probe context.Context) (string, error) {
		result, execErr := l.check.Lab.Exec(probe, frrPeer, []string{"vtysh", "-c", command}, nil)
		if execErr != nil {
			return "", execErr
		}
		return result.Stdout, nil
	}, func(output string) bool {
		return frrHasRoute(output) == present
	})
	return err
}
