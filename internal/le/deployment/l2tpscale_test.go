package deployment

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestL2TPScaleScenarioRegistryIsExact(t *testing.T) {
	want := []string{"2k-sessions", "clean-teardown", "pool-exhaustion", "slow-radius"}
	if got := l2tpScaleScenarios(); !slices.Equal(got, want) {
		t.Fatalf("L2TP scale scenarios = %q, want %q", got, want)
	}
	if len(l2tpScaleScenarioRegistry) != 4 {
		t.Fatalf("L2TP scale registry has %d scenarios, want 4", len(l2tpScaleScenarioRegistry))
	}
	wantShape := []struct {
		tunnels int
		session int
		delay   time.Duration
		dwell   time.Duration
	}{
		{tunnels: 10, session: 200, dwell: time.Second},
		{tunnels: 2, session: 10, dwell: time.Second},
		{tunnels: 1, session: 300, dwell: time.Second},
		{tunnels: 2, session: 5, delay: 500 * time.Millisecond, dwell: time.Second},
	}
	for index, scenario := range l2tpScaleScenarioRegistry {
		want := wantShape[index]
		if scenario.tunnels != want.tunnels || scenario.sessions != want.session ||
			scenario.radiusDelay != want.delay || scenario.dwell != want.dwell || scenario.check == nil {
			t.Fatalf("scenario %q shape drifted: %#v", scenario.name, scenario)
		}
	}
}

func TestL2TPScaleRegistryRunsEveryScenarioNonVacuously(t *testing.T) {
	system := newL2TPScaleRecorder()
	report, code := runL2TPScaleAt(context.Background(), "/repo", l2tpScaleRunOptions{}, system)
	if code != 0 || report.Passed != 4 || report.Failed != 0 {
		t.Fatalf("native scale run = code %d report %#v", code, report)
	}
	if len(system.starts) != 4 || len(system.commands) != 4 || system.stops != 4 {
		t.Fatalf("effects = %d ze starts, %d simulators, %d stops; want 4 each",
			len(system.starts), len(system.commands), system.stops)
	}
	for index, scenario := range report.Scenarios {
		if scenario.Name != l2tpScaleScenarios()[index] || scenario.Result == nil || len(scenario.ResultBytes) == 0 {
			t.Fatalf("scenario %d is vacuous: %#v", index, scenario)
		}
		var exact L2TPScaleResult
		if err := json.Unmarshal(scenario.ResultBytes, &exact); err != nil {
			t.Fatalf("scenario %q result bytes are not JSON: %v", scenario.Name, err)
		}
		if exact.SessionsUp != scenario.Result.SessionsUp {
			t.Fatalf("scenario %q decoded result drifted from bytes", scenario.Name)
		}
	}
	if !strings.Contains(string(system.starts[2].stdin), "start 10.99.0.2") ||
		!strings.Contains(string(system.starts[2].stdin), "end 10.99.0.255") {
		t.Fatalf("pool exhaustion did not use the 254-address pool:\n%s", system.starts[2].stdin)
	}
	if !strings.Contains(strings.Join(system.commands[3].argv, " "), "--radius-delay 500ms") {
		t.Fatalf("slow RADIUS command = %q", system.commands[3].argv)
	}
}

func TestL2TPScaleCheckersPreserveVerdictBoundaries(t *testing.T) {
	if warnings, err := checkL2TPScale2K(&L2TPScaleResult{
		TunnelsUp: 10, SessionsUp: 2_000, RADIUSAcctStart: 2_000, SessionsPerSec: 99,
	}); err != nil || len(warnings) != 1 {
		t.Fatalf("sub-target rate = warnings %v error %v", warnings, err)
	}
	if _, err := checkL2TPScalePoolExhaustion(&L2TPScaleResult{SessionsUp: 255}); err == nil {
		t.Fatal("pool checker accepted more sessions than addresses")
	}
	if _, err := checkL2TPScalePoolExhaustion(&L2TPScaleResult{SessionsUp: 254}); err != nil {
		t.Fatalf("pool checker rejected the exact pool size: %v", err)
	}
	if _, err := checkL2TPScaleSlowRADIUS(&L2TPScaleResult{
		SessionsUp: 10, Errors: []string{"diagnostic retained by former checker"},
	}); err != nil {
		t.Fatalf("slow RADIUS checker changed its session-count verdict: %v", err)
	}
	if _, err := checkL2TPScaleCleanTeardown(&L2TPScaleResult{
		SessionsUp: 20, Errors: []string{"leak"},
	}); err == nil {
		t.Fatal("clean teardown accepted a simulator error")
	}
}

func TestL2TPScaleSelectionAndActionAreReachable(t *testing.T) {
	report, code := runL2TPScaleAt(
		context.Background(), "/repo", l2tpScaleRunOptions{Scenario: "missing"}, newL2TPScaleRecorder(),
	)
	if code != 1 || report.Failure == "" || len(report.Scenarios) != 0 {
		t.Fatalf("unknown selection = code %d report %#v", code, report)
	}
	found := false
	for _, action := range Actions().Actions {
		if action.Verb == L2TPScaleAction {
			found = true
		}
	}
	if !found {
		t.Fatalf("native action %q is not reachable", L2TPScaleAction)
	}
}

func TestL2TPScaleSessionTimeoutDefaultsAndOverrides(t *testing.T) {
	if got := l2tpScaleSessionTimeout(""); got != 120*time.Second {
		t.Fatalf("empty timeout = %s", got)
	}
	if got := l2tpScaleSessionTimeout("not-a-number"); got != 120*time.Second {
		t.Fatalf("invalid timeout = %s", got)
	}
	if got := l2tpScaleSessionTimeout("0"); got != 0 {
		t.Fatalf("zero timeout = %s", got)
	}
	if got := l2tpScaleSessionTimeout("37"); got != 37*time.Second {
		t.Fatalf("configured timeout = %s", got)
	}
}

type l2tpScaleRecorder struct {
	files    map[string]bool
	starts   []l2tpScaleStart
	commands []l2tpScaleCommand
	stops    int
	port     int
}

func newL2TPScaleRecorder() *l2tpScaleRecorder {
	return &l2tpScaleRecorder{
		files: map[string]bool{"/repo/bin/ze": true, "/repo/bin/ze-test": true},
		port:  20_000,
	}
}

func (r *l2tpScaleRecorder) FileExists(path string) bool { return r.files[path] }

func (r *l2tpScaleRecorder) Getenv(string) string { return "" }

func (r *l2tpScaleRecorder) Environ() []string { return []string{"PATH=/fixture/bin"} }

func (r *l2tpScaleRecorder) freeUDPPort() (int, error) {
	r.port++
	return r.port, nil
}

func (r *l2tpScaleRecorder) Start(
	_ context.Context,
	start l2tpScaleStart,
) (l2tpScaleProcess, error) {
	r.starts = append(r.starts, l2tpScaleStart{
		argv: slices.Clone(start.argv), environ: slices.Clone(start.environ), stdin: slices.Clone(start.stdin),
	})
	return &l2tpScaleRecordedProcess{recorder: r}, nil
}

func (r *l2tpScaleRecorder) Run(
	_ context.Context,
	command l2tpScaleCommand,
) (l2tpScaleCommandResult, error) {
	r.commands = append(r.commands, l2tpScaleCommand{
		argv: slices.Clone(command.argv), environ: slices.Clone(command.environ), timeout: command.timeout,
	})
	tunnels := l2tpScaleFlagInt(command.argv, "--tunnels")
	perTunnel := l2tpScaleFlagInt(command.argv, "--sessions")
	sessions := tunnels * perTunnel
	if sessions == 300 {
		sessions = 254
	}
	result := L2TPScaleResult{
		TunnelsRequested:  tunnels,
		TunnelsUp:         tunnels,
		SessionsRequested: tunnels * perTunnel,
		SessionsUp:        sessions,
		SetupTime:         2 * time.Second,
		TeardownTime:      time.Second,
		SessionsPerSec:    500,
		RADIUSAuth:        int64(sessions),
		RADIUSAcctStart:   int64(sessions),
		RADIUSAcctStop:    int64(sessions),
	}
	content, err := json.Marshal(result)
	if err != nil {
		return l2tpScaleCommandResult{}, err
	}
	return l2tpScaleCommandResult{stdout: append(content, '\n')}, nil
}

func (r *l2tpScaleRecorder) Sleep(context.Context, time.Duration) error { return nil }

type l2tpScaleRecordedProcess struct {
	recorder *l2tpScaleRecorder
}

func (*l2tpScaleRecordedProcess) Exited() (bool, int, []byte, error) {
	return false, 0, nil, nil
}

func (p *l2tpScaleRecordedProcess) Stop(time.Duration, time.Duration) error {
	p.recorder.stops++
	return nil
}

func l2tpScaleFlagInt(argv []string, name string) int {
	for index, value := range argv {
		if value == name && index+1 < len(argv) {
			parsed, err := strconv.Atoi(argv[index+1])
			if err != nil {
				panic(fmt.Sprintf("fixture flag %s: %v", name, err))
			}
			return parsed
		}
	}
	panic("fixture missing flag " + name)
}

var _ l2tpScaleSystem = (*l2tpScaleRecorder)(nil)
var _ l2tpScaleProcess = (*l2tpScaleRecordedProcess)(nil)
