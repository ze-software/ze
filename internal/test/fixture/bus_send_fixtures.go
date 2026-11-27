// Design: docs/architecture/api/architecture.md -- Phase 6 bus send authorization and refusal functional test fixtures.
// Overview: register_bus_send_fixtures.go -- driver registrations for Phase 6 bus send tests.
// Related: fixture.go -- Poll, ReportFailure, newObserver.
// Related: bus_send_undeclared_probe.go -- S20 undeclared send probe.
// Related: bus_send_op_cap_probe.go -- S37 engine-operation capability probe.

package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	busEndpointPathPrefix = "/bus/endpoint/"
	busPollAttempts       = 150
	busPollDelay          = 100 * time.Millisecond

	busMsgShowBGPPeer = "cli:show bgp peer"
	busMsgClearBGPRib = "cli:clear bgp rib"
	busMsgBGPUpdate   = "bgp:update"
	busMsgBGPState    = "bgp:state"
	busMsgBGPRaw      = "bgp:raw"
	busMsgSysRIBInst  = "system-rib:route-install"
	busMsgAcmeAlert   = "acme:alert"

	busCmdShowBGPPeerList = "show bgp peer list"
	busCmdClearBGPRibIn   = "clear bgp rib in *"
	busCmdShowInterface   = "show interface"

	busUndeclaredMarker = "registration does not declare"
	busAllowListMarker  = "send allow-list"
	busInspectFixMarker = "show config running | implied"
	busMetricPrefix     = "ze_bus_send_refused_total{"
)

type busRefusedMetricKey struct {
	participant string
	message     string
}

// busEndpointPath returns the canonical bus path for endpointName.
func busEndpointPath(endpointName string) string {
	var tb textbuf.Buffer
	return tb.Str(busEndpointPathPrefix).Str(endpointName).String()
}

// busCurrentEndpointName returns the authenticated endpoint name assigned by
// the daemon, falling back to fallback when unset.
func busCurrentEndpointName(fallback string) string {
	if name := env.Get("ze.plugin.name"); name != "" {
		return name
	}
	return fallback
}

// busScenarioWorker runs scenario for a connected bus fixture plugin, reports
// any failure to stderr before transport teardown, and posts the outcome on
// result. Owned by busRunPlugin; terminates when scenario returns or ctx ends.
func busScenarioWorker(ctx context.Context, p *sdk.Plugin, scenario ObserverScenario, result chan<- error) {
	scenarioErr := invokeScenario(ctx, p, scenario)
	if scenarioErr != nil {
		ReportFailure(scenarioErr)
		result <- scenarioErr
		_ = p.Close() //nolint:errcheck // closing transport terminates Run after reporting failure
		return
	}
	result <- nil
}

// busRunPlugin connects as an external plugin with reg (answering Stage 1
// declaration query mode when requested), runs scenario after all daemon
// plugins are ready, and stays connected until the daemon stops without
// dispatching request quiesce or request shutdown.
func busRunPlugin(
	ctx context.Context,
	fallbackName string,
	reg *sdk.Registration,
	setup func(*sdk.Plugin) error,
	scenario ObserverScenario,
) error {
	if sdk.QueryModeRequested() {
		code := sdk.RunOrDeclare(reg, func() int { return 0 })
		if code != 0 {
			return fmt.Errorf("declare query for %s exited %d", fallbackName, code)
		}
		return nil
	}

	p, err := newObserver(fallbackName)
	if err != nil {
		return fmt.Errorf("connect plugin %s: %w", fallbackName, err)
	}
	defer p.Close() //nolint:errcheck // transport close error is secondary to runErr

	if setup != nil {
		if err := setup(p); err != nil {
			return fmt.Errorf("configure plugin %s: %w", fallbackName, err)
		}
	}

	result := make(chan error, 1)
	if scenario != nil {
		p.OnAllPluginsReady(func() error {
			go busScenarioWorker(ctx, p, scenario, result)
			return nil
		})
	}

	runErr := p.Run(ctx, reg)
	select {
	case scenarioErr := <-result:
		if scenarioErr != nil {
			return errors.Join(scenarioErr, runErr)
		}
		return nil
	default:
		if ctx.Err() != nil {
			return nil
		}
		return runErr
	}
}

// busCheckRefusalBase rejects errors produced by wiring, registration, or
// parser defects so a probe never passes falsely for a non-bus error.
func busCheckRefusalBase(operation, refusal string) error {
	for _, bad := range []string{
		"unknown command",
		"unknown method",
		"unknown protocol",
		"not registered",
		"engine has no Loc-RIB",
		"bad prefix",
		"bad next-hop",
	} {
		if strings.Contains(refusal, bad) {
			return fmt.Errorf("%s failed with %q rather than a bus send refusal: %s", operation, bad, refusal)
		}
	}
	return nil
}

// busAssertUndeclaredRefusal verifies that err is a Reason 1 bus send refusal
// naming participant, msg, the undeclared-registration reason, and the
// inspection command.
func busAssertUndeclaredRefusal(operation string, err error, participant, msg string) error {
	if err == nil {
		return fmt.Errorf("%s succeeded, want undeclared bus send refusal for %s (%s)", operation, participant, msg)
	}
	refusal := err.Error()
	if baseErr := busCheckRefusalBase(operation, refusal); baseErr != nil {
		return baseErr
	}
	if !strings.Contains(refusal, participant) {
		return fmt.Errorf("%s refusal omitted participant %q: %s", operation, participant, refusal)
	}
	if !strings.Contains(refusal, msg) {
		return fmt.Errorf("%s refusal omitted message %q: %s", operation, msg, refusal)
	}
	if !strings.Contains(refusal, busUndeclaredMarker) {
		return fmt.Errorf("%s refusal omitted %q: %s", operation, busUndeclaredMarker, refusal)
	}
	if !strings.Contains(refusal, busInspectFixMarker) {
		return fmt.Errorf("%s refusal omitted %q: %s", operation, busInspectFixMarker, refusal)
	}
	return nil
}

// busAssertAllowListRefusal verifies that err is a Reason 2 bus send allow-list
// refusal naming participant, endpoint, msg, and the exact fix line.
func busAssertAllowListRefusal(operation string, err error, participant, endpoint, msg string) error {
	if err == nil {
		return fmt.Errorf("%s succeeded, want send allow-list refusal for %s (%s)", operation, participant, msg)
	}
	refusal := err.Error()
	if baseErr := busCheckRefusalBase(operation, refusal); baseErr != nil {
		return baseErr
	}
	if !strings.Contains(refusal, participant) {
		return fmt.Errorf("%s refusal omitted participant %q: %s", operation, participant, refusal)
	}
	if !strings.Contains(refusal, msg) {
		return fmt.Errorf("%s refusal omitted message %q: %s", operation, msg, refusal)
	}
	if !strings.Contains(refusal, busAllowListMarker) {
		return fmt.Errorf("%s refusal omitted %q: %s", operation, busAllowListMarker, refusal)
	}
	var fix textbuf.Buffer
	fix.Str("set bus endpoint ").Str(endpoint).Str(" send allow ")
	if !strings.Contains(refusal, fix.String()) {
		return fmt.Errorf("%s refusal omitted fix prefix for endpoint %q: %s", operation, endpoint, refusal)
	}
	return nil
}

// busWriteAtomicFile writes content to path via a temp file rename so polling
// readers never observe a partial write.
func busWriteAtomicFile(path, content string) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}

// busPollFile waits until path exists on disk within the bounded poll budget.
func busPollFile(ctx context.Context, path string) error {
	var lastErr error
	found := Poll(ctx, busPollAttempts, busPollDelay, func() bool {
		_, err := os.Stat(path)
		if err != nil {
			lastErr = err
			return false
		}
		lastErr = nil
		return true
	})
	if !found {
		return fmt.Errorf("timed out waiting for marker file %q: %w", path, lastErr)
	}
	return nil
}

// busWaitFileDriver implements `le test fixture plugin/bus-wait-file <path>`.
func busWaitFileDriver(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: plugin/bus-wait-file <path>")
	}
	if err := busPollFile(ctx, args[0]); err != nil {
		return err
	}
	var tb textbuf.Buffer
	return tb.Str("OK: marker file ").Str(args[0]).Str(" appeared\n").StdOut()
}

// busTouchFileDriver implements `le test fixture plugin/bus-touch-file <path>`.
func busTouchFileDriver(_ context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: plugin/bus-touch-file <path>")
	}
	if err := busWriteAtomicFile(args[0], "go\n"); err != nil {
		return err
	}
	var tb textbuf.Buffer
	return tb.Str("OK: wrote marker file ").Str(args[0]).Byte('\n').StdOut()
}

// busFetchMetrics retrieves the Prometheus text scrape from 127.0.0.1:port.
func busFetchMetrics(ctx context.Context, port string) (string, error) {
	var urlBuf textbuf.Buffer
	url := urlBuf.Str("http://127.0.0.1:").Str(port).Str("/metrics").String()
	client := http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck // response body read completes below
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s returned status %d", url, resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// busParseRefusedMetrics parses ze_bus_send_refused_total{...} series lines
// from Prometheus exposition text.
func busParseRefusedMetrics(body string) (map[busRefusedMetricKey]int, []string) {
	counts := make(map[busRefusedMetricKey]int)
	var lines []string
	for line := range strings.SplitSeq(body, "\n") {
		if !strings.HasPrefix(line, busMetricPrefix) {
			continue
		}
		closeBrace := strings.IndexByte(line, '}')
		if closeBrace < 0 {
			continue
		}
		labelText := line[len(busMetricPrefix):closeBrace]
		valText := strings.TrimSpace(line[closeBrace+1:])
		valFloat, err := strconv.ParseFloat(valText, 64)
		if err != nil {
			continue
		}
		labels := busParsePromLabels(labelText)
		key := busRefusedMetricKey{
			participant: labels["participant"],
			message:     labels["message"],
		}
		counts[key] += int(valFloat)
		lines = append(lines, line)
	}
	return counts, lines
}

// busParsePromLabels parses comma-separated key="value" Prometheus labels,
// supporting escaped quotes and backslashes inside quoted values.
func busParsePromLabels(raw string) map[string]string {
	out := make(map[string]string)
	i := 0
	for i < len(raw) {
		for i < len(raw) && (raw[i] == ',' || raw[i] == ' ') {
			i++
		}
		eq := strings.IndexByte(raw[i:], '=')
		if eq < 0 {
			break
		}
		key := strings.TrimSpace(raw[i : i+eq])
		i += eq + 1
		if i >= len(raw) {
			break
		}
		if raw[i] != '"' {
			break
		}
		i++
		var val textbuf.Buffer
		for i < len(raw) {
			ch := raw[i]
			i++
			if ch == '\\' && i < len(raw) {
				val.Byte(raw[i])
				i++
				continue
			}
			if ch == '"' {
				break
			}
			val.Byte(ch)
		}
		out[key] = val.String()
	}
	return out
}

// busAssertRefusedCountDriver verifies the total ze_bus_send_refused_total
// count across all messages for one participant on the Prometheus endpoint.
// Usage: plugin/bus-assert-refused-count <metrics-port> <participant> <want>.
func busAssertRefusedCountDriver(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return errors.New("usage: plugin/bus-assert-refused-count <metrics-port> <participant> <want>")
	}
	port, participant := args[0], args[1]
	want, err := strconv.Atoi(args[2])
	if err != nil {
		return fmt.Errorf("parse want count %q: %w", args[2], err)
	}

	var got int
	var lastLines []string
	var lastErr error
	matched := Poll(ctx, busPollAttempts, busPollDelay, func() bool {
		body, fetchErr := busFetchMetrics(ctx, port)
		if fetchErr != nil {
			lastErr = fetchErr
			return false
		}
		lastErr = nil
		counts, lines := busParseRefusedMetrics(body)
		lastLines = lines
		sum := 0
		for k, v := range counts {
			if k.participant == participant {
				sum += v
			}
		}
		got = sum
		return sum == want
	})
	if !matched {
		if lastErr != nil {
			return fmt.Errorf("scrape metrics on port %s: %w", port, lastErr)
		}
		return fmt.Errorf("ze_bus_send_refused_total for %s = %d, want %d (series: %v)", participant, got, want, lastLines)
	}
	var tb textbuf.Buffer
	return tb.Str("OK: ze_bus_send_refused_total for ").Str(participant).Str(" = ").Int(int64(got)).Byte('\n').StdOut()
}

// busAssertRefusedSeriesDriver verifies one exact (participant, message) series
// of ze_bus_send_refused_total on the Prometheus endpoint.
// Usage: plugin/bus-assert-refused-series <metrics-port> <participant> <message> <want>.
func busAssertRefusedSeriesDriver(ctx context.Context, args []string) error {
	if len(args) != 4 {
		return errors.New("usage: plugin/bus-assert-refused-series <metrics-port> <participant> <message> <want>")
	}
	port, participant, message := args[0], args[1], args[2]
	want, err := strconv.Atoi(args[3])
	if err != nil {
		return fmt.Errorf("parse want count %q: %w", args[3], err)
	}

	var got int
	var lastLines []string
	var lastErr error
	matched := Poll(ctx, busPollAttempts, busPollDelay, func() bool {
		body, fetchErr := busFetchMetrics(ctx, port)
		if fetchErr != nil {
			lastErr = fetchErr
			return false
		}
		lastErr = nil
		counts, lines := busParseRefusedMetrics(body)
		lastLines = lines
		got = counts[busRefusedMetricKey{participant: participant, message: message}]
		return got == want
	})
	if !matched {
		if lastErr != nil {
			return fmt.Errorf("scrape metrics on port %s: %w", port, lastErr)
		}
		return fmt.Errorf("ze_bus_send_refused_total{participant=%q,message=%q} = %d, want %d (series: %v)",
			participant, message, got, want, lastLines)
	}
	var tb textbuf.Buffer
	return tb.Str("OK: ze_bus_send_refused_total{participant=").Quoted(participant).
		Str(",message=").Quoted(message).Str("} = ").Int(int64(got)).Byte('\n').StdOut()
}

// busSIGHUPReloadDriver copies srcConf onto dstConf, sends SIGHUP to daemon.pid,
// polls `ze cli -c "show system reload status | json"` until generation advances with
// last-outcome "applied", and writes doneMarker.
// Usage: plugin/bus-sighup-reload <src-conf> <dst-conf> <done-marker>.
func busSIGHUPReloadDriver(ctx context.Context, args []string) error {
	return busSIGHUPReloadExpect(ctx, args, "applied", "plugin/bus-sighup-reload")
}

// busSIGHUPReloadRefusedDriver copies srcConf onto dstConf, sends SIGHUP to
// daemon.pid, polls `ze cli -c "show system reload status | json"` until generation
// advances with last-outcome "failed", and writes doneMarker.
// Usage: plugin/bus-sighup-reload-refused <src-conf> <dst-conf> <done-marker>.
func busSIGHUPReloadRefusedDriver(ctx context.Context, args []string) error {
	return busSIGHUPReloadExpect(ctx, args, "failed", "plugin/bus-sighup-reload-refused")
}

func busSIGHUPReloadExpect(ctx context.Context, args []string, wantOutcome, usageName string) error {
	if len(args) != 3 {
		return fmt.Errorf("usage: %s <src-conf> <dst-conf> <done-marker>", usageName)
	}
	srcConf, dstConf, doneMarker := args[0], args[1], args[2]

	baseline, _, err := busReadReloadStatusCLI(ctx)
	if err != nil {
		return fmt.Errorf("read baseline reload-status: %w", err)
	}

	rawConf, err := os.ReadFile(srcConf) //nolint:gosec // test scratch file in runner workdir
	if err != nil {
		return fmt.Errorf("read %s: %w", srcConf, err)
	}
	if err := os.WriteFile(dstConf, rawConf, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", dstConf, err)
	}

	pidBytes, err := os.ReadFile(fileDaemonPID)
	if err != nil {
		return fmt.Errorf("read %s: %w", fileDaemonPID, err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		return fmt.Errorf("parse %s: %w", fileDaemonPID, err)
	}
	if err := syscall.Kill(pid, syscall.SIGHUP); err != nil {
		return fmt.Errorf("send SIGHUP to daemon %d: %w", pid, err)
	}

	var gen int64
	var outcome string
	var pollErr error
	advanced := Poll(ctx, busPollAttempts, busPollDelay, func() bool {
		gen, outcome, pollErr = busReadReloadStatusCLI(ctx)
		if pollErr != nil {
			return false
		}
		return gen > baseline
	})
	if !advanced {
		return fmt.Errorf("SIGHUP reload of %s never advanced generation past %d (last gen=%d outcome=%q err=%v)",
			srcConf, baseline, gen, outcome, pollErr)
	}
	if outcome != wantOutcome {
		return fmt.Errorf("SIGHUP reload of %s ended with last-outcome=%q (generation=%d), want %s",
			srcConf, outcome, gen, wantOutcome)
	}
	var genBuf textbuf.Buffer
	if err := busWriteAtomicFile(doneMarker, genBuf.Int(gen).Byte('\n').String()); err != nil {
		return err
	}
	var tb textbuf.Buffer
	return tb.Str("OK: SIGHUP reload ").Str(srcConf).Byte(' ').Str(wantOutcome).Str(" at generation ").Int(gen).Byte('\n').StdOut()
}

// busReadReloadStatusCLI runs `ze cli -c "show system reload status | json"` and
// extracts its generation and last-outcome fields.
func busReadReloadStatusCLI(ctx context.Context) (int64, string, error) {
	code, stdout, stderr, err := runCaptured(ctx, os.Environ(), "", "ze", "cli", "-c", "show system reload status | json")
	if err != nil {
		return -1, "", fmt.Errorf("run ze cli: %w", err)
	}
	if code != 0 {
		return -1, "", fmt.Errorf("ze cli exited %d: %s", code, strings.TrimSpace(stderr))
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		return -1, "", fmt.Errorf("decode reload-status JSON: %w (stdout=%q)", err, stdout)
	}
	genFloat, ok := payload["generation"].(float64)
	if !ok {
		return -1, "", fmt.Errorf("reload-status missing numeric generation: %v", payload)
	}
	outcome, _ := payload["last-outcome"].(string)
	return int64(genFloat), outcome, nil
}

// busAssertSamePIDDriver verifies that all named marker files carry the exact
// same non-empty PID, proving an endpoint kept running across reloads.
// Usage: plugin/bus-assert-same-pid <file1> <file2> [<file3>...].
func busAssertSamePIDDriver(_ context.Context, args []string) error {
	if len(args) < 2 {
		return errors.New("usage: plugin/bus-assert-same-pid <file1> <file2> [...]")
	}
	var expected string
	for _, path := range args {
		data, err := os.ReadFile(path) //nolint:gosec // test scratch file in runner workdir
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		pid := strings.TrimSpace(string(data))
		if pid == "" {
			return fmt.Errorf("marker %s has empty PID", path)
		}
		if expected == "" {
			expected = pid
			continue
		}
		if pid != expected {
			return fmt.Errorf("endpoint restarted across reloads: %s had PID %s, %s had PID %s",
				args[0], expected, path, pid)
		}
	}
	var tb textbuf.Buffer
	return tb.Str("OK: endpoint kept same PID ").Str(expected).Str(" across ").Int(int64(len(args))).Str(" phases\n").StdOut()
}

// busEventCollectorDriver registers reception of one message and appends each
// delivered JSON event line to outFile. When doneNeedle and doneFile are
// supplied, it writes doneFile once a delivered event contains doneNeedle.
// Usage: plugin/bus-event-collector <message> <out-file> [<done-needle> <done-file>].
func busEventCollectorDriver(ctx context.Context, args []string) error {
	if len(args) != 2 && len(args) != 4 {
		return errors.New("usage: plugin/bus-event-collector <message> <out-file> [<done-needle> <done-file>]")
	}
	message, outFile := args[0], args[1]
	doneNeedle, doneFile := "", ""
	if len(args) == 4 {
		doneNeedle, doneFile = args[2], args[3]
	}

	reg := &sdk.Registration{
		Receives: []sdk.MessageReceiveDecl{{Message: message}},
	}
	var mu sync.Mutex
	var recorded []string
	return busRunPlugin(ctx, "collector", reg, func(p *sdk.Plugin) error {
		p.OnEvent(func(event string) error {
			mu.Lock()
			recorded = append(recorded, strings.TrimSpace(event))
			var body textbuf.Buffer
			for _, line := range recorded {
				body.Str(line).Byte('\n')
			}
			snapshot := body.String()
			mu.Unlock()

			if err := busWriteAtomicFile(outFile, snapshot); err != nil {
				return err
			}
			if doneFile != "" && strings.Contains(event, doneNeedle) {
				if err := busWriteAtomicFile(doneFile, "done\n"); err != nil {
					return err
				}
			}
			return nil
		})
		return nil
	}, nil)
}

// busAssertCollectorDriver parses JSON lines in logFile and verifies the
// collector recorded exactly one event with expectedSender and expectedDetail,
// and none of the forbidden substrings.
// Usage: plugin/bus-assert-collector <log-file> <expected-sender> <expected-detail> [<forbidden>...].
func busAssertCollectorDriver(_ context.Context, args []string) error {
	if len(args) < 3 {
		return errors.New("usage: plugin/bus-assert-collector <log-file> <expected-sender> <expected-detail> [<forbidden>...]")
	}
	logFile, wantSender, wantDetail := args[0], args[1], args[2]
	forbidden := args[3:]

	raw, err := os.ReadFile(logFile) //nolint:gosec // test scratch file in runner workdir
	if err != nil {
		return fmt.Errorf("read collector log %s: %w", logFile, err)
	}
	content := string(raw)
	for _, bad := range forbidden {
		if strings.Contains(content, bad) {
			return fmt.Errorf("collector log %s contains forbidden payload %q:\n%s", logFile, bad, content)
		}
	}
	var lines []string
	for line := range strings.SplitSeq(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	if len(lines) != 1 {
		return fmt.Errorf("collector log %s recorded %d events, want 1:\n%s", logFile, len(lines), content)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &decoded); err != nil {
		return fmt.Errorf("decode collector event JSON: %w (line=%q)", err, lines[0])
	}
	gotSender, _ := decoded["sender"].(string)
	if gotSender != wantSender {
		return fmt.Errorf("collector event sender = %q, want %q (line=%s)", gotSender, wantSender, lines[0])
	}
	gotDetail, _ := decoded["detail"].(string)
	if gotDetail != wantDetail {
		return fmt.Errorf("collector event detail = %q, want %q (line=%s)", gotDetail, wantDetail, lines[0])
	}
	var tb textbuf.Buffer
	return tb.Str("OK: collector recorded sender=").Str(gotSender).Str(" detail=").Str(gotDetail).Str(" (1 event)\n").StdOut()
}

// busSenderStampDriver implements S25 (bus-sender-stamp.ci). Role "b" emits
// two acme:alert events spoofing sender "/bus/endpoint/a" and "a", asserting
// zero deliveries before writing bDoneFile. Role "a" waits for bDoneFile, emits
// an authentic acme:alert, and asserts delivery to collector before writing
// aDoneFile.
// Usage: plugin/bus-sender-stamp <a|b> <b-done-file> <a-done-file>.
func busSenderStampDriver(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return errors.New("usage: plugin/bus-sender-stamp <a|b> <b-done-file> <a-done-file>")
	}
	role, bDoneFile, aDoneFile := args[0], args[1], args[2]
	if role != "a" && role != "b" {
		return fmt.Errorf("bus-sender-stamp role %q must be a or b", role)
	}
	reg := &sdk.Registration{
		Sends: []sdk.MessageSendDecl{{Message: busMsgAcmeAlert}},
	}
	return busRunPlugin(ctx, role, reg, nil, func(ctx context.Context, p *sdk.Plugin) error {
		if role == "b" {
			spoofPayloads := []string{
				`{"sender":"/bus/endpoint/a","detail":"spoof-full-from-b"}`,
				`{"sender":"a","detail":"spoof-short-from-b"}`,
			}
			for _, payload := range spoofPayloads {
				delivered, err := p.EmitEvent(ctx, "acme", "alert", "", "", payload)
				if err != nil {
					return fmt.Errorf("role b emit %s: %w", payload, err)
				}
				if delivered != 0 {
					return fmt.Errorf("role b spoofed emit %s was delivered to %d receiver(s), want 0 because bus must stamp /bus/endpoint/b",
						payload, delivered)
				}
			}
			if err := busWriteAtomicFile(bDoneFile, "b-done\n"); err != nil {
				return err
			}
			var tb textbuf.Buffer
			return tb.Str("OK: role b spoofed emits were blocked by bus sender stamping (delivered=0)\n").StdErr()
		}

		if err := busPollFile(ctx, bDoneFile); err != nil {
			return fmt.Errorf("role a waiting for role b: %w", err)
		}
		delivered, err := p.EmitEvent(ctx, "acme", "alert", "", "", `{"detail":"authentic-from-a"}`)
		if err != nil {
			return fmt.Errorf("role a authentic emit: %w", err)
		}
		if delivered != 1 {
			return fmt.Errorf("role a authentic emit delivered to %d receiver(s), want 1", delivered)
		}
		if err := busWriteAtomicFile(aDoneFile, "a-done\n"); err != nil {
			return err
		}
		var tb textbuf.Buffer
		return tb.Str("OK: role a authentic emit delivered (delivered=1)\n").StdErr()
	})
}

// busSendAllowListReloadDriver implements S27 (bus-send-allow-list-reload.ci).
// Endpoint "ops" declares cli:show bgp peer and cli:clear bgp rib and runs
// three command batches across two live SIGHUP reloads without restarting.
func busSendAllowListReloadDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("plugin/bus-send-allow-list-reload takes no arguments")
	}
	reg := &sdk.Registration{
		Sends: []sdk.MessageSendDecl{
			{Message: busMsgShowBGPPeer},
			{Message: busMsgClearBGPRib},
		},
	}
	return busRunPlugin(ctx, "ops", reg, nil, func(ctx context.Context, p *sdk.Plugin) error {
		epName := busCurrentEndpointName("ops")
		epPath := busEndpointPath(epName)
		var pidBuf textbuf.Buffer
		pidLine := pidBuf.Int(int64(os.Getpid())).Byte('\n').String()

		// Phase 0: no send allow-list written.
		if err := busExpectCLISucceeds(ctx, p, busCmdShowBGPPeerList); err != nil {
			return fmt.Errorf("phase 0: %w", err)
		}
		if err := busExpectCLISucceeds(ctx, p, busCmdClearBGPRibIn); err != nil {
			return fmt.Errorf("phase 0: %w", err)
		}
		if err := busExpectCLIUndeclared(ctx, p, busCmdShowInterface, epPath); err != nil {
			return fmt.Errorf("phase 0: %w", err)
		}
		if err := busWriteAtomicFile("phase0.done", pidLine); err != nil {
			return err
		}

		// Phase 1: send { allow [ "cli:show bgp peer" ]; }.
		if err := busPollFile(ctx, "reload1.applied"); err != nil {
			return fmt.Errorf("phase 1: %w", err)
		}
		if err := busExpectCLISucceeds(ctx, p, busCmdShowBGPPeerList); err != nil {
			return fmt.Errorf("phase 1: %w", err)
		}
		if err := busExpectCLIAllowListRefused(ctx, p, busCmdClearBGPRibIn, epPath, epName); err != nil {
			return fmt.Errorf("phase 1: %w", err)
		}
		if err := busExpectCLIUndeclared(ctx, p, busCmdShowInterface, epPath); err != nil {
			return fmt.Errorf("phase 1: %w", err)
		}
		if err := busWriteAtomicFile("phase1.done", pidLine); err != nil {
			return err
		}

		// Phase 2: send { allow none; }.
		if err := busPollFile(ctx, "reload2.applied"); err != nil {
			return fmt.Errorf("phase 2: %w", err)
		}
		if err := busExpectCLIAllowListRefused(ctx, p, busCmdShowBGPPeerList, epPath, epName); err != nil {
			return fmt.Errorf("phase 2: %w", err)
		}
		if err := busExpectCLIAllowListRefused(ctx, p, busCmdClearBGPRibIn, epPath, epName); err != nil {
			return fmt.Errorf("phase 2: %w", err)
		}
		if err := busExpectCLIUndeclared(ctx, p, busCmdShowInterface, epPath); err != nil {
			return fmt.Errorf("phase 2: %w", err)
		}
		if err := busWriteAtomicFile("phase2.done", pidLine); err != nil {
			return err
		}
		var tb textbuf.Buffer
		return tb.Str("OK: ops allow-list reload phases 0, 1, 2 verified with same PID\n").StdErr()
	})
}

func busExpectCLISucceeds(ctx context.Context, p *sdk.Plugin, command string) error {
	status, raw, err := p.DispatchCommand(ctx, command)
	if err != nil {
		return fmt.Errorf("dispatch %q: %w", command, err)
	}
	if status != statusDone {
		return fmt.Errorf("dispatch %q returned status %q (data=%s), want %s", command, status, string(raw), statusDone)
	}
	return nil
}

func busExpectCLIUndeclared(ctx context.Context, p *sdk.Plugin, command, participant string) error {
	status, raw, err := p.DispatchCommand(ctx, command)
	if err == nil {
		if status == statusDone {
			return fmt.Errorf("undeclared command %q succeeded (data=%s)", command, string(raw))
		}
		err = errors.New(string(raw))
	}
	var msgBuf textbuf.Buffer
	msg := msgBuf.Str("cli:").Str(command).String()
	return busAssertUndeclaredRefusal("dispatch "+command, err, participant, msg)
}

func busExpectCLIAllowListRefused(ctx context.Context, p *sdk.Plugin, command, participant, endpoint string) error {
	status, raw, err := p.DispatchCommand(ctx, command)
	if err == nil {
		if status == statusDone {
			return fmt.Errorf("allow-list excluded command %q succeeded (data=%s)", command, string(raw))
		}
		err = errors.New(string(raw))
	}
	var msgBuf textbuf.Buffer
	msg := msgBuf.Str("cli:").Str(command).String()
	return busAssertAllowListRefusal("dispatch "+command, err, participant, endpoint, msg)
}

// busSendAllowListNamespacesDriver implements S36 (bus-send-allow-list-namespaces.ci).
// Endpoint "ops2" declares bgp:update, cli:show bgp peer, and cli:clear bgp rib.
// Usage: plugin/bus-send-allow-list-namespaces <restricted|control> <established-file> <done-file>.
func busSendAllowListNamespacesDriver(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return errors.New("usage: plugin/bus-send-allow-list-namespaces <restricted|control> <established-file> <done-file>")
	}
	mode, estFile, doneFile := args[0], args[1], args[2]
	if mode != "restricted" && mode != "control" {
		return fmt.Errorf("bus-send-allow-list-namespaces mode %q must be restricted or control", mode)
	}
	reg := &sdk.Registration{
		Sends: []sdk.MessageSendDecl{
			{Message: busMsgBGPUpdate},
			{Message: busMsgShowBGPPeer},
			{Message: busMsgClearBGPRib},
		},
	}
	return busRunPlugin(ctx, "ops2", reg, nil, func(ctx context.Context, p *sdk.Plugin) error {
		epName := busCurrentEndpointName("ops2")
		epPath := busEndpointPath(epName)

		if err := busPollFile(ctx, estFile); err != nil {
			return fmt.Errorf("%s: wait for edge1 established: %w", mode, err)
		}
		announced, _, err := p.UpdateRoute(ctx, "127.0.0.1", "update text origin igp nhop 192.0.2.1 nlri add 203.0.113.0/24")
		if err != nil {
			return fmt.Errorf("%s: UpdateRoute to edge1 failed: %w", mode, err)
		}
		if announced < 1 {
			return fmt.Errorf("%s: UpdateRoute announced=%d, want >= 1", mode, announced)
		}

		if mode == "restricted" {
			if err := busExpectCLIAllowListRefused(ctx, p, busCmdShowBGPPeerList, epPath, epName); err != nil {
				return fmt.Errorf("restricted: %w", err)
			}
			if err := busExpectCLIAllowListRefused(ctx, p, busCmdClearBGPRibIn, epPath, epName); err != nil {
				return fmt.Errorf("restricted: %w", err)
			}
		} else {
			if err := busExpectCLISucceeds(ctx, p, busCmdShowBGPPeerList); err != nil {
				return fmt.Errorf("control: %w", err)
			}
			if err := busExpectCLISucceeds(ctx, p, busCmdClearBGPRibIn); err != nil {
				return fmt.Errorf("control: %w", err)
			}
		}
		if err := busWriteAtomicFile(doneFile, mode+"\n"); err != nil {
			return err
		}
		var tb textbuf.Buffer
		return tb.Str("OK: bus-send-allow-list-namespaces ").Str(mode).Str(" verified\n").StdErr()
	})
}

// busSendRefusalDoctorDriver implements Story 10 (bus-send-refusal-doctor.ci).
// Endpoint "monitor" declares cli:show bgp peer and cli:clear bgp rib; under
// send { allow [ "cli:show bgp peer" ]; } show bgp peer list succeeds and
// clear bgp rib in * triggers a bus send refusal reported by show doctor and
// counted by ze_bus_send_refused_total.
// Usage: plugin/bus-send-refusal-doctor <done-file>.
func busSendRefusalDoctorDriver(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: plugin/bus-send-refusal-doctor <done-file>")
	}
	doneFile := args[0]
	reg := &sdk.Registration{
		Sends: []sdk.MessageSendDecl{
			{Message: busMsgShowBGPPeer},
			{Message: busMsgClearBGPRib},
		},
	}
	return busRunPlugin(ctx, "monitor", reg, nil, func(ctx context.Context, p *sdk.Plugin) error {
		epName := busCurrentEndpointName("monitor")
		epPath := busEndpointPath(epName)

		if err := busExpectCLISucceeds(ctx, p, busCmdShowBGPPeerList); err != nil {
			return fmt.Errorf("allowed command failed: %w", err)
		}
		if err := busExpectCLIAllowListRefused(ctx, p, busCmdClearBGPRibIn, epPath, epName); err != nil {
			return err
		}
		if err := busWriteAtomicFile(doneFile, "refused\n"); err != nil {
			return err
		}
		var tb textbuf.Buffer
		return tb.Str("OK: bus send refusal triggered for ").Str(epPath).Byte('\n').StdErr()
	})
}
