package testdeployment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/paths"
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

// TestL2TPScaleRunsItsOwnExecutable proves the simulator is the running le.
// Method: a recorder that answers its own executable, and the argv of the first
// simulator command; then a recorder that cannot answer it.
//
// VALIDATES: the simulator runs as `<own executable> test scale l2tp ...`
// (AC-41), and a runner that cannot name its own file fails before it runs
// anything.
func TestL2TPScaleRunsItsOwnExecutable(t *testing.T) {
	system := newL2TPScaleRecorder()
	report, _ := runL2TPScaleAt(context.Background(), "/repo", l2tpScaleRunOptions{Scenario: l2tpScaleScenarios()[0]}, system)
	if len(system.commands) == 0 {
		t.Fatalf("no simulator command ran: %s", report.Failure)
	}
	if argv := system.commands[0].argv; len(argv) < 4 || argv[0] != "/fixture/le" || argv[1] != "test" || argv[2] != "scale" || argv[3] != "l2tp" {
		t.Fatalf("the simulator argv is %q, want /fixture/le test scale l2tp ...", argv)
	}

	broken := newL2TPScaleRecorder()
	broken.executableErr = errors.New("no /proc")
	report, code := runL2TPScaleAt(context.Background(), "/repo", l2tpScaleRunOptions{Scenario: l2tpScaleScenarios()[0]}, broken)
	if code == 0 || !strings.Contains(report.Failure, "does not know its own executable") || len(broken.commands) != 0 {
		t.Fatalf("code %d, failure %q, %d commands: want a refusal before any command", code, report.Failure, len(broken.commands))
	}
}

// l2tpScaleResolvedStore answers the directory a ze started with environ and
// argv opens its store in: the ze.config.dir it reads (any spelling, dotted
// winning, as env.Get reads it), else the directory beside its binary, which
// is what paths.DefaultConfigDir falls back to.
func l2tpScaleResolvedStore(start l2tpScaleStart) string {
	if dir := env.LookupIn(start.environ, l2tpScaleConfigDirKey); dir != "" {
		return dir
	}
	return paths.ConfigDirFromBinary(start.argv[0])
}

// TestL2TPScaleZeNeverResolvesTheRepositoryStore proves each scenario's ze
// opens a store the run owns, never the checkout's etc/ze.
// Method: run the registry twice through the recorder, once with an inherited
// environment that names the operator's etc/ze under both spellings and once
// with none; resolve each ze's store the way ze does; then read the order in
// which the stores were created, ze stopped and the stores removed.
//
// VALIDATES: no ze start resolves /repo/etc/ze, each resolves the fresh store
// under /repo/tmp/evidence made for its scenario, and that store is removed
// after its ze stops. PREVENTS: the harness ze opening the operator's store
// (`blob artifact .../etc/ze/database.zefs is not a live store`).
func TestL2TPScaleZeNeverResolvesTheRepositoryStore(t *testing.T) {
	operator := "/repo/etc/ze"
	inherited := map[string][]string{
		"operator store inherited": {"PATH=/fixture/bin", "ZE_CONFIG_DIR=" + operator, "ze.config.dir=" + operator},
		"no store inherited":       {"PATH=/fixture/bin"},
	}
	for name, environ := range inherited {
		t.Run(name, func(t *testing.T) {
			system := newL2TPScaleRecorder()
			system.environ = environ
			report, code := runL2TPScaleAt(context.Background(), "/repo", l2tpScaleRunOptions{}, system)
			if code != 0 || report.Failed != 0 {
				t.Fatalf("run = code %d report %#v", code, report)
			}
			if len(system.starts) != len(l2tpScaleScenarioRegistry) || len(system.stores) != len(system.starts) {
				t.Fatalf("%d ze starts and %d stores, want one store for each of %d scenarios",
					len(system.starts), len(system.stores), len(l2tpScaleScenarioRegistry))
			}
			if operator != paths.ConfigDirFromBinary(system.starts[0].argv[0]) {
				t.Fatalf("fixture: bin/ze resolves %q beside it, want %q", paths.ConfigDirFromBinary(system.starts[0].argv[0]), operator)
			}
			var want []string
			for index, start := range system.starts {
				resolved := l2tpScaleResolvedStore(start)
				if resolved == operator {
					t.Fatalf("scenario %d: ze resolves the repository store %s; environ %q", index, resolved, start.environ)
				}
				if resolved != system.stores[index] {
					t.Fatalf("scenario %d: ze resolves %q, want its own store %q", index, resolved, system.stores[index])
				}
				if !strings.HasPrefix(resolved, "/repo/tmp/evidence/") {
					t.Fatalf("scenario %d: store %q is not under the tree's scratch", index, resolved)
				}
				want = append(want, "store "+resolved, "stop", "remove "+resolved)
			}
			if !slices.Equal(system.events, want) {
				t.Fatalf("store lifecycle = %q, want %q", system.events, want)
			}
		})
	}
}

// TestL2TPScaleStoreFailuresNameTheFixAndClean proves a store the run cannot
// make stops the scenario before ze starts, and a ze that cannot start still
// has its store removed.
// Method: a recorder whose store creation fails, then one whose start fails.
//
// VALIDATES: no ze start without its own store; the failure names the
// directory and the command to rerun; a failed start removes the store.
func TestL2TPScaleStoreFailuresNameTheFixAndClean(t *testing.T) {
	system := newL2TPScaleRecorder()
	system.storeErr = errors.New("no space left on device")
	report, code := runL2TPScaleAt(context.Background(), "/repo", l2tpScaleRunOptions{Scenario: "slow-radius"}, system)
	if code == 0 || len(system.starts) != 0 {
		t.Fatalf("code %d with %d ze starts, want a refusal before ze starts", code, len(system.starts))
	}
	failure := report.Scenarios[0].Failure
	for _, part := range []string{"/repo/tmp/evidence", "no space left on device", "./le test deployment l2tp-scale"} {
		if !strings.Contains(failure, part) {
			t.Fatalf("failure %q does not name %q", failure, part)
		}
	}

	system = newL2TPScaleRecorder()
	system.startErr = errors.New("exec format error")
	report, _ = runL2TPScaleAt(context.Background(), "/repo", l2tpScaleRunOptions{Scenario: "slow-radius"}, system)
	if report.Scenarios[0].Passed || !slices.Equal(system.removed, system.stores) || len(system.stores) != 1 {
		t.Fatalf("failed start: report %#v, stores %q, removed %q", report.Scenarios[0], system.stores, system.removed)
	}
}

// TestL2TPScaleRealStoreIsLiveAndRemoved proves the real system's store is a
// live ze store under the tree's scratch, and that removal leaves nothing.
// Method: zeStore in a temporary tree, storage.Open on the answer, removeStore.
func TestL2TPScaleRealStoreIsLiveAndRemoved(t *testing.T) {
	root := t.TempDir()
	system := realL2TPScaleSystem{}
	dir, err := system.zeStore(root, "2k-sessions")
	if err != nil {
		t.Fatalf("zeStore: %v", err)
	}
	if filepath.Dir(dir) != filepath.Join(root, "tmp", "evidence") {
		t.Fatalf("store %q is not directly under %s/tmp/evidence", dir, root)
	}
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatalf("ze could not open the store the run made: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := system.removeStore(dir); err != nil {
		t.Fatalf("removeStore: %v", err)
	}
	if _, err := os.Lstat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("store %s still exists after removeStore: %v", dir, err)
	}
}

// TestL2TPScaleTextSaysWhyEachScenarioFailed proves the text form carries
// each failed scenario's failure, the same text the JSON form carries.
// Method: render a report with one passed and two failed scenarios, one of
// whose failures spans two lines, and compare the exact text; then check each
// JSON failure field's lines appear in the text.
func TestL2TPScaleTextSaysWhyEachScenarioFailed(t *testing.T) {
	storeless := "ze exited early: error: config storage unavailable: blob artifact /repo/etc/ze/database.zefs is not a live store; run ze init --from /repo/etc/ze/database.zefs\n"
	report := l2tpScaleReport{
		Action: L2TPScaleAction, Passed: 1, Failed: 2, Code: 1,
		Scenarios: []L2TPScaleScenarioReport{
			{Name: "2k-sessions", Failure: storeless},
			{Name: "clean-teardown", Passed: true},
			{Name: "pool-exhaustion", Failure: "sessions: 300/254\nsecond line"},
		},
	}
	want := "FAIL  1 passed, 2 failed: 2k-sessions pool-exhaustion\n" +
		"  2k-sessions: " + strings.TrimRight(storeless, "\n") + "\n" +
		"  pool-exhaustion: sessions: 300/254\n" +
		"    second line"
	text := report.Text()
	if text != want {
		t.Fatalf("text =\n%s\nwant\n%s", text, want)
	}
	content, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var decoded l2tpScaleReport
	if err := json.Unmarshal(content, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range decoded.Scenarios {
		if scenario.Passed {
			if strings.Contains(text, scenario.Name+":") {
				t.Fatalf("passed scenario %q printed a failure line", scenario.Name)
			}
			continue
		}
		for line := range strings.SplitSeq(strings.TrimRight(scenario.Failure, "\n"), "\n") {
			if !strings.Contains(text, line) {
				t.Fatalf("text form lacks %q from the JSON failure of %q", line, scenario.Name)
			}
		}
	}
}

type l2tpScaleRecorder struct {
	executableErr error
	storeErr      error
	startErr      error
	files         map[string]bool
	env           map[string]string
	environ       []string
	starts        []l2tpScaleStart
	commands      []l2tpScaleCommand
	stops         int
	port          int
	stores        []string
	removed       []string
	// events is the order of store creation, ze stop and store removal.
	events []string
}

func newL2TPScaleRecorder() *l2tpScaleRecorder {
	return &l2tpScaleRecorder{
		files: map[string]bool{"/repo/bin/ze": true},
		port:  20_000,
	}
}

func (r *l2tpScaleRecorder) Executable() (string, error) {
	if r.executableErr != nil {
		return "", r.executableErr
	}
	return "/fixture/le", nil
}

func (r *l2tpScaleRecorder) FileExists(path string) bool { return r.files[path] }

func (r *l2tpScaleRecorder) Getenv(key string) string { return r.env[key] }

func (r *l2tpScaleRecorder) Environ() []string {
	if r.environ != nil {
		return slices.Clone(r.environ)
	}
	return []string{"PATH=/fixture/bin"}
}

// zeStore answers a fresh directory under root's scratch, as the real system
// does, without touching the disk.
func (r *l2tpScaleRecorder) zeStore(root, scenario string) (string, error) {
	if r.storeErr != nil {
		return "", r.storeErr
	}
	dir := filepath.Join(root, "tmp", "evidence", "l2tp-scale-"+scenario+"-"+strconv.Itoa(len(r.stores)))
	r.stores = append(r.stores, dir)
	r.events = append(r.events, "store "+dir)
	return dir, nil
}

func (r *l2tpScaleRecorder) removeStore(dir string) error {
	r.removed = append(r.removed, dir)
	r.events = append(r.events, "remove "+dir)
	return nil
}

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
	if r.startErr != nil {
		return nil, r.startErr
	}
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

func (p *l2tpScaleRecordedProcess) Stop() error {
	p.recorder.stops++
	p.recorder.events = append(p.recorder.events, "stop")
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
