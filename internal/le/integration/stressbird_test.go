// Related: stressbird.go -- the native BIRD stress scenario these fixtures drive.
//
// VALIDATES: the BIRD baseline performs every namespace, process, peer, query, and cleanup step.
// PREVENTS: a wrapper that reports success without running the complete four-round scenario.
package integration

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

type stressBirdRecorder struct {
	events        []string
	commands      []stressBirdCommand
	euid          int
	files         map[string]bool
	missing       map[string]bool
	runCodes      map[string]int
	routeCounts   []int
	routeIndex    int
	peerCodes     []int
	peerIndex     int
	peerWaitError error
	now           time.Time
}

type stressBirdRecordedProcess struct {
	name     string
	code     int
	waitErr  error
	exited   bool
	recorder *stressBirdRecorder
}

func newStressBirdRecorder() *stressBirdRecorder {
	return &stressBirdRecorder{
		files: map[string]bool{
			"/repo/bin/ze-test": true,
			"/repo/test/stress/scenarios/04-bulk-ipv4-bird/bird.conf": true,
		},
		missing:  make(map[string]bool),
		runCodes: make(map[string]int),
		now:      time.Unix(1_700_000_000, 0),
	}
}

func (r *stressBirdRecorder) effectiveUID() int { return r.euid }

func (r *stressBirdRecorder) LookPath(name string) (string, error) {
	r.events = append(r.events, "look "+name)
	if r.missing[name] {
		return "", os.ErrNotExist
	}
	return "/fixture/bin/" + name, nil
}

func (r *stressBirdRecorder) FileExists(path string) bool {
	r.events = append(r.events, "file "+path)
	return r.files[path]
}

func (r *stressBirdRecorder) PID() int { return 4815 }

func (r *stressBirdRecorder) Environ() []string {
	return []string{"PATH=/fixture/bin", "ZE_STRESS_SUFFIX=fixture"}
}

func (r *stressBirdRecorder) Getenv(key string) string {
	if key == "ZE_STRESS_SUFFIX" {
		return "fixture"
	}
	return ""
}

func (r *stressBirdRecorder) Run(_ context.Context, command stressBirdCommand) (stressBirdCommandResult, error) {
	r.commands = append(r.commands, stressBirdCommand{
		argv: slices.Clone(command.argv), dir: command.dir, environ: slices.Clone(command.environ),
		outputPath: command.outputPath, timeout: command.timeout,
	})
	line := "run " + strings.Join(command.argv, " ")
	r.events = append(r.events, line)
	code := r.runCodes[line]
	if code == 0 && strings.Contains(line, "apt-get install") {
		if strings.Contains(line, "iproute2") {
			delete(r.missing, "ip")
			delete(r.missing, "ethtool")
		}
		if strings.Contains(line, "bird2") {
			delete(r.missing, "bird")
			delete(r.missing, "birdc")
		}
	}
	if strings.Contains(line, " birdc ") {
		count := 0
		if len(r.routeCounts) > 0 {
			at := min(r.routeIndex, len(r.routeCounts)-1)
			count = r.routeCounts[at]
			r.routeIndex++
		}
		return stressBirdCommandResult{
			stdout: fmt.Sprintf("%d of %d routes (1 network)\n", count, count),
			code:   code,
		}, nil
	}
	if strings.Contains(line, " ss ") {
		return stressBirdCommandResult{stdout: "LISTEN 0 4096 0.0.0.0:179\n", code: code}, nil
	}
	return stressBirdCommandResult{code: code}, nil
}

func (r *stressBirdRecorder) Start(_ context.Context, command stressBirdCommand) (stressBirdProcess, error) {
	r.commands = append(r.commands, stressBirdCommand{
		argv: slices.Clone(command.argv), dir: command.dir, environ: slices.Clone(command.environ),
		outputPath: command.outputPath, timeout: command.timeout,
	})
	line := "start " + strings.Join(command.argv, " ") + " stdout=" + command.outputPath
	r.events = append(r.events, line)
	if slices.Contains(command.argv, "bird") {
		return &stressBirdRecordedProcess{name: "bird", recorder: r}, nil
	}
	code := 0
	if r.peerIndex < len(r.peerCodes) {
		code = r.peerCodes[r.peerIndex]
	}
	r.peerIndex++
	return &stressBirdRecordedProcess{
		name: "peer", code: code, waitErr: r.peerWaitError, exited: true, recorder: r,
	}, nil
}

func (r *stressBirdRecorder) Remove(path string) error {
	r.events = append(r.events, "remove "+path)
	return nil
}

func (r *stressBirdRecorder) Sleep(_ context.Context, duration time.Duration) error {
	r.events = append(r.events, "sleep "+duration.String())
	r.now = r.now.Add(duration)
	return nil
}

func (r *stressBirdRecorder) Now() time.Time { return r.now }

func (p *stressBirdRecordedProcess) Exited() (bool, int, error) {
	p.recorder.events = append(p.recorder.events, "poll "+p.name)
	return p.exited, p.code, p.waitErr
}

func (p *stressBirdRecordedProcess) Wait(timeout time.Duration) (int, error) {
	p.recorder.events = append(p.recorder.events, "wait "+p.name+" "+timeout.String())
	return p.code, p.waitErr
}

func (p *stressBirdRecordedProcess) Terminate() error {
	p.recorder.events = append(p.recorder.events, "terminate "+p.name)
	p.exited = true
	return nil
}

func (p *stressBirdRecordedProcess) Kill() error {
	p.recorder.events = append(p.recorder.events, "kill "+p.name)
	p.exited = true
	p.waitErr = nil
	return nil
}

func runRecordedStressBird(t *testing.T, recorder *stressBirdRecorder) (StressBirdReport, int) {
	t.Helper()
	runner := stressBirdRunner{
		root: "/repo", system: recorder,
		environ: []string{"PATH=/fixture/bin", "ZE_STRESS_SUFFIX=fixture"},
	}
	return runner.run(context.Background())
}

func TestStressBirdRunsEveryRoundAndCleansEveryResource(t *testing.T) {
	recorder := newStressBirdRecorder()
	recorder.routeCounts = []int{100_000, 250_000, 500_000, 1_000_000}

	report, code := runRecordedStressBird(t, recorder)
	if code != 0 {
		t.Fatalf("run code = %d, failure = %#v", code, report.Failure)
	}
	if !report.Passed {
		t.Fatal("successful scenario did not report passed")
	}
	if got := report.Text(); got != "PASS  1 scenario(s): 04-bulk-ipv4-bird" {
		t.Fatalf("success text = %q", got)
	}
	gotCounts := make([]int, 0, len(report.Rounds))
	for _, round := range report.Rounds {
		gotCounts = append(gotCounts, round.ObservedRoutes)
	}
	wantCounts := []int{100_000, 250_000, 500_000, 1_000_000}
	if !slices.Equal(gotCounts, wantCounts) {
		t.Fatalf("observed route counts = %v, want %v", gotCounts, wantCounts)
	}
	wantEnvironment := []string{"PATH=/fixture/bin", "ZE_STRESS_SUFFIX=fixture"}
	for index, command := range recorder.commands {
		if !slices.Equal(command.environ, wantEnvironment) {
			t.Fatalf("command %d environment = %q, want %q", index, command.environ, wantEnvironment)
		}
	}
	assertStressBirdCallOrder(t, recorder.events)
}

func TestStressBirdTimesOutTheFirstRedRoundAndStillCleansUp(t *testing.T) {
	recorder := newStressBirdRecorder()
	recorder.routeCounts = []int{99_999}

	report, code := runRecordedStressBird(t, recorder)
	if code != stressBirdTimeoutCode {
		t.Fatalf("timeout code = %d, want %d", code, stressBirdTimeoutCode)
	}
	if report.Failure == nil || report.Failure.Phase != "bird-routes" || report.Failure.Round != 100_000 {
		t.Fatalf("failure = %#v, want first round bird-routes failure", report.Failure)
	}
	if len(report.Rounds) != 1 || report.Rounds[0].RouteQueries != 61 {
		t.Fatalf("rounds = %#v, want one round and 61 bounded route queries", report.Rounds)
	}
	if anyStressBirdEvent(recorder.events, "start ip netns exec ze-stress-bb-fixture /repo/bin/ze-test peer --mode inject --dial 172.31.0.2:179 --inject-prefix 10.64.0.0/24") {
		t.Fatal("second round started after the first round timed out")
	}
	assertStressBirdCleanupTail(t, recorder.events, 2)
}

func TestStressBirdPreservesTheFirstFailingProcessCode(t *testing.T) {
	recorder := newStressBirdRecorder()
	recorder.peerCodes = []int{23}

	report, code := runRecordedStressBird(t, recorder)
	if code != 23 {
		t.Fatalf("peer failure code = %d, want 23", code)
	}
	if report.Failure == nil || report.Failure.Phase != "peer" || report.Failure.ExitCode != 23 {
		t.Fatalf("failure = %#v, want peer exit 23", report.Failure)
	}
	if got := report.Text(); got != "FAIL  04-bulk-ipv4-bird: peer inject exited with code 23" {
		t.Fatalf("failure text = %q", got)
	}
	if anyStressBirdEvent(recorder.events, " birdc ") {
		t.Fatal("BIRD was queried after the peer failed")
	}
	assertStressBirdCleanupTail(t, recorder.events, 2)
}

func TestStressBirdFailsClosedOnABIRDQueryError(t *testing.T) {
	recorder := newStressBirdRecorder()
	query := "run ip netns exec ze-stress-ze-fixture birdc -s /tmp/ze-stress-bird-fixture.ctl show route count"
	recorder.runCodes[query] = 29

	report, code := runRecordedStressBird(t, recorder)
	if code != 29 {
		t.Fatalf("birdc failure code = %d, want 29", code)
	}
	if report.Failure == nil || report.Failure.Phase != "bird-routes" || report.Failure.ExitCode != 29 {
		t.Fatalf("failure = %#v, want bird-routes exit 29", report.Failure)
	}
	if len(report.Rounds) != 1 || report.Rounds[0].RouteQueries != 1 {
		t.Fatalf("rounds = %#v, want one failed query", report.Rounds)
	}
	assertStressBirdCleanupTail(t, recorder.events, 2)
}

func TestStressBirdFailsClosedOnANamespaceCommandError(t *testing.T) {
	recorder := newStressBirdRecorder()
	failed := "run ip netns add ze-stress-bb-fixture"
	recorder.runCodes[failed] = 17

	report, code := runRecordedStressBird(t, recorder)
	if code != 17 {
		t.Fatalf("namespace failure code = %d, want 17", code)
	}
	if report.Failure == nil || report.Failure.Phase != "namespace" {
		t.Fatalf("failure = %#v, want namespace failure", report.Failure)
	}
	if anyStressBirdEvent(recorder.events, "start ") {
		t.Fatal("a process started after namespace setup failed")
	}
	assertStressBirdCleanupTail(t, recorder.events, 0)
}

func TestStressBirdRefusesANonRootRunBeforeSetup(t *testing.T) {
	recorder := newStressBirdRecorder()
	recorder.euid = 1000

	report, code := runRecordedStressBird(t, recorder)
	if code != 1 {
		t.Fatalf("non-root code = %d, want 1", code)
	}
	if report.Failure == nil || report.Failure.Phase != "preflight" {
		t.Fatalf("failure = %#v, want preflight refusal", report.Failure)
	}
	if anyStressBirdEvent(recorder.events, "start ") || anyStressBirdEvent(recorder.events, "netns add") {
		t.Fatalf("non-root run reached setup: %v", recorder.events)
	}
	assertStressBirdCleanupTail(t, recorder.events, 0)
}

func TestStressBirdPortsTheRuntimeDependencySetupWithoutPython(t *testing.T) {
	recorder := newStressBirdRecorder()
	recorder.missing["ip"] = true
	recorder.missing["ethtool"] = true
	recorder.routeCounts = []int{100_000, 250_000, 500_000, 1_000_000}

	report, code := runRecordedStressBird(t, recorder)
	if code != 0 {
		t.Fatalf("runtime setup code = %d, failure = %#v", code, report.Failure)
	}
	for _, command := range []string{
		"run apt-get update -qq",
		"run apt-get install -y --no-install-recommends iproute2 ethtool tcpdump jq",
	} {
		if !anyStressBirdEvent(recorder.events, command) {
			t.Fatalf("runtime preflight omitted %q; events = %v", command, recorder.events)
		}
	}
}

func TestStressBirdPortsTheBIRDPreflightInstallWithoutPythonOrSudo(t *testing.T) {
	recorder := newStressBirdRecorder()
	recorder.missing["bird"] = true
	recorder.missing["birdc"] = true
	recorder.routeCounts = []int{100_000, 250_000, 500_000, 1_000_000}

	report, code := runRecordedStressBird(t, recorder)
	if code != 0 {
		t.Fatalf("preflight install code = %d, failure = %#v", code, report.Failure)
	}
	install := "run apt-get install -y --no-install-recommends bird2"
	if !anyStressBirdEvent(recorder.events, install) {
		t.Fatalf("BIRD preflight omitted %q; events = %v", install, recorder.events)
	}
	for _, forbidden := range []string{"python", "sudo", "go run", "make"} {
		if anyStressBirdEvent(recorder.events, forbidden) {
			t.Fatalf("native preflight invoked forbidden wrapper %q; events = %v", forbidden, recorder.events)
		}
	}
}

func TestStressBirdKillsAPeerWhoseBoundedWaitExpires(t *testing.T) {
	recorder := newStressBirdRecorder()
	recorder.peerWaitError = errStressBirdWaitTimeout

	report, code := runRecordedStressBird(t, recorder)
	if code != stressBirdTimeoutCode {
		t.Fatalf("peer timeout code = %d, want %d", code, stressBirdTimeoutCode)
	}
	if report.Failure == nil || report.Failure.Phase != "peer" {
		t.Fatalf("failure = %#v, want peer timeout", report.Failure)
	}
	if !anyStressBirdEvent(recorder.events, "kill peer") {
		t.Fatal("timed-out peer was not killed")
	}
	assertStressBirdCleanupTail(t, recorder.events, 2)
}

func assertStressBirdCallOrder(t *testing.T, events []string) {
	t.Helper()
	ordered := []string{
		"look ip",
		"look ethtool",
		"look ip",
		"look ethtool",
		"look bird",
		"look birdc",
		"look bird",
		"look birdc",
		"file /repo/bin/ze-test",
		"file /repo/test/stress/scenarios/04-bulk-ipv4-bird/bird.conf",
		"run ip netns del ze-stress-ze-fixture",
		"run ip netns del ze-stress-bb-fixture",
		"remove /tmp/ze-stress-ze-fixture.log",
		"remove /tmp/ze-stress-peer-fixture.log",
		"remove /tmp/ze-stress-pcap-fixture.txt",
		"remove /tmp/ze-stress-bird-fixture.log",
		"remove /tmp/ze-stress-bird-fixture.pid",
		"remove /tmp/ze-stress-bird-fixture.ctl",
		"run ip netns add ze-stress-ze-fixture",
		"run ip netns add ze-stress-bb-fixture",
		"run ip link add ze-v-fixtur type veth peer name bb-v-fixtur",
		"run ip link set ze-v-fixtur netns ze-stress-ze-fixture",
		"run ip link set bb-v-fixtur netns ze-stress-bb-fixture",
		"run ip netns exec ze-stress-ze-fixture ip addr add 172.31.0.2/24 dev ze-v-fixtur",
		"run ip netns exec ze-stress-ze-fixture ip link set ze-v-fixtur up",
		"run ip netns exec ze-stress-ze-fixture ip link set lo up",
		"run ip netns exec ze-stress-bb-fixture ip addr add 172.31.0.3/24 dev bb-v-fixtur",
		"run ip netns exec ze-stress-bb-fixture ip link set bb-v-fixtur up",
		"run ip netns exec ze-stress-bb-fixture ip link set lo up",
		"run ip netns exec ze-stress-ze-fixture ethtool -K ze-v-fixtur tx off rx off",
		"run ip netns exec ze-stress-bb-fixture ethtool -K bb-v-fixtur tx off rx off",
		"start ip netns exec ze-stress-ze-fixture bird -f -c /repo/test/stress/scenarios/04-bulk-ipv4-bird/bird.conf -P /tmp/ze-stress-bird-fixture.pid -s /tmp/ze-stress-bird-fixture.ctl stdout=/tmp/ze-stress-bird-fixture.log",
		"sleep 2s",
		"poll bird",
		"run ip netns exec ze-stress-ze-fixture ss -tln sport = 179",
	}
	prefixes := []struct {
		base    string
		count   int
		timeout time.Duration
	}{
		{base: "10.0.0.0/24", count: 100_000, timeout: 120 * time.Second},
		{base: "10.64.0.0/24", count: 250_000, timeout: 180 * time.Second},
		{base: "10.128.0.0/24", count: 500_000, timeout: 300 * time.Second},
		{base: "11.0.0.0/24", count: 1_000_000, timeout: 600 * time.Second},
	}
	for _, round := range prefixes {
		ordered = append(ordered,
			fmt.Sprintf("start ip netns exec ze-stress-bb-fixture /repo/bin/ze-test peer --mode inject --dial 172.31.0.2:179 --inject-prefix %s --inject-count %d --inject-nexthop 172.31.0.3 --inject-asn 65100 --inject-dwell 30s stdout=/tmp/ze-stress-peer-fixture.log", round.base, round.count),
			"wait peer "+round.timeout.String(),
			"run ip netns exec ze-stress-ze-fixture birdc -s /tmp/ze-stress-bird-fixture.ctl show route count",
		)
	}
	ordered = append(ordered, "terminate bird", "wait bird 5s")
	for range prefixes {
		ordered = append(ordered, "terminate peer", "wait peer 5s")
	}
	ordered = append(ordered,
		"run ip netns del ze-stress-ze-fixture",
		"run ip netns del ze-stress-bb-fixture",
		"remove /tmp/ze-stress-ze-fixture.log",
		"remove /tmp/ze-stress-peer-fixture.log",
		"remove /tmp/ze-stress-pcap-fixture.txt",
		"remove /tmp/ze-stress-bird-fixture.log",
		"remove /tmp/ze-stress-bird-fixture.pid",
		"remove /tmp/ze-stress-bird-fixture.ctl",
	)
	if !slices.Equal(events, ordered) {
		t.Fatalf("external call order =\n%q\nwant\n%q", events, ordered)
	}
}

func assertStressBirdCleanupTail(t *testing.T, events []string, processes int) {
	t.Helper()
	wantTail := []string{
		"run ip netns del ze-stress-ze-fixture",
		"run ip netns del ze-stress-bb-fixture",
		"remove /tmp/ze-stress-ze-fixture.log",
		"remove /tmp/ze-stress-peer-fixture.log",
		"remove /tmp/ze-stress-pcap-fixture.txt",
		"remove /tmp/ze-stress-bird-fixture.log",
		"remove /tmp/ze-stress-bird-fixture.pid",
		"remove /tmp/ze-stress-bird-fixture.ctl",
	}
	if len(events) < len(wantTail) {
		t.Fatalf("events too short for cleanup: %v", events)
	}
	if got := events[len(events)-len(wantTail):]; !slices.Equal(got, wantTail) {
		t.Fatalf("cleanup tail =\n%q\nwant\n%q", got, wantTail)
	}
	terminateCount := 0
	for _, event := range events {
		if strings.HasPrefix(event, "terminate ") {
			terminateCount++
		}
	}
	if terminateCount != processes {
		t.Fatalf("terminated processes = %d, want %d; events = %v", terminateCount, processes, events)
	}
}

func anyStressBirdEvent(events []string, needle string) bool {
	for _, event := range events {
		if strings.Contains(event, needle) {
			return true
		}
	}
	return false
}

var _ stressBirdSystem = (*stressBirdRecorder)(nil)
var _ stressBirdProcess = (*stressBirdRecordedProcess)(nil)
