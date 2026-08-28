package integration

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestStressScenarioRegistryIsExact(t *testing.T) {
	want := []string{
		"01-bulk-ipv4",
		"02-multi-peer",
		"03-session-flap",
		"04-bulk-ipv4-bird",
		"05-profile-1m",
	}
	if got := stressScenarios(); !slices.Equal(got, want) {
		t.Fatalf("stress scenarios = %q, want %q", got, want)
	}
	if len(stressScenarioRegistry) != 5 {
		t.Fatalf("stress registry has %d scenarios, want 5", len(stressScenarioRegistry))
	}

	bulk, _ := stressScenarioNamed("01-bulk-ipv4")
	gotBulk := make([]string, 0, len(bulk.rounds))
	for _, round := range bulk.rounds {
		gotBulk = append(gotBulk, stressRoundIdentity(round)+"/"+round.timeout.String())
	}
	wantBulk := []string{
		"10.0.0.0/24/100000/15s/2m0s",
		"10.64.0.0/24/250000/15s/3m0s",
		"10.128.0.0/24/500000/15s/5m0s",
		"11.0.0.0/24/1000000/15s/10m0s",
	}
	if !slices.Equal(gotBulk, wantBulk) {
		t.Fatalf("bulk rounds = %q, want %q", gotBulk, wantBulk)
	}

	mixed, _ := stressScenarioNamed("02-multi-peer")
	if len(mixed.rounds) != 2 || mixed.rounds[0].prefixes != 500_000 ||
		mixed.rounds[1].prefixes != 250_000 || mixed.rounds[1].nexthop != "2001:db8::3" {
		t.Fatalf("mixed-family scenario drifted: %#v", mixed.rounds)
	}
	flap, _ := stressScenarioNamed("03-session-flap")
	if len(flap.rounds) != 11 || flap.rounds[9].pause != 2*time.Second ||
		flap.rounds[10].pause != 0 || flap.rounds[10].dwell != "5s" {
		t.Fatalf("flap scenario drifted: %#v", flap.rounds)
	}
	profile, _ := stressScenarioNamed("05-profile-1m")
	if len(profile.rounds) != 1 || profile.rounds[0].prefixes != 1_000_000 ||
		profile.rounds[0].dwell != "60s" || profile.rounds[0].timeout != 600*time.Second {
		t.Fatalf("profile scenario drifted: %#v", profile.rounds)
	}
}

func TestStressRegistryRunsEveryScenarioNonVacuously(t *testing.T) {
	recorder := newStressRecorder()
	report, code := runStressAt(context.Background(), "/repo", stressOptions{}, recorder)
	if code != 0 || report.Failed != 0 || report.Passed != 5 {
		t.Fatalf("native stress run = code %d report %#v", code, report)
	}
	if len(report.Scenarios) != 5 {
		t.Fatalf("runner produced %d scenario verdicts, want 5", len(report.Scenarios))
	}

	peerStarts := 0
	zeStarts := 0
	birdStarts := 0
	for _, event := range recorder.events {
		switch {
		case strings.Contains(event, "/bin/ze-test peer --mode inject"):
			peerStarts++
		case strings.Contains(event, "/bin/ze start "):
			zeStarts++
		case strings.Contains(event, " bird -f "):
			birdStarts++
		}
	}
	if peerStarts != 22 || zeStarts != 4 || birdStarts != 1 {
		t.Fatalf("external starts = peers %d, ze %d, bird %d; want 22, 4, 1\nevents: %v",
			peerStarts, zeStarts, birdStarts, recorder.events)
	}
	for _, scenario := range report.Scenarios {
		if scenario.Name == stressBirdScenario {
			if scenario.Bird == nil || len(scenario.Bird.Rounds) != 4 {
				t.Fatalf("BIRD scenario did no route rounds: %#v", scenario)
			}
			continue
		}
		if len(scenario.Rounds) == 0 {
			t.Fatalf("scenario %q passed without an injector round", scenario.Name)
		}
		for _, round := range scenario.Rounds {
			if round.Metrics.Bytes != 8_388_608 || round.Metrics.Messages != 4096 {
				t.Fatalf("scenario %q lost injector metrics: %#v", scenario.Name, round.Metrics)
			}
		}
	}
}

func TestStressHarnessBuildsZeFromCheckoutWhenMissing(t *testing.T) {
	recorder := newStressRecorder()
	delete(recorder.files, "/repo/bin/ze")
	report, code := runStressAt(
		context.Background(), "/repo", stressOptions{Scenario: "01-bulk-ipv4"}, recorder,
	)
	if code != 0 || report.Passed != 1 {
		t.Fatalf("run after native build = code %d report %#v", code, report)
	}
	for _, command := range recorder.commands {
		if len(command.argv) == 0 || command.argv[0] != "go" {
			continue
		}
		if command.dir != "/repo" || !slices.Contains(command.environ, "CGO_ENABLED=0") {
			t.Fatalf("build command = dir %q env %q argv %q", command.dir, command.environ, command.argv)
		}
		return
	}
	t.Fatal("missing bin/ze did not invoke the native Go build")
}

func TestStressProfileScenarioCapturesAllProfiles(t *testing.T) {
	recorder := newStressRecorder()
	recorder.pprof = true
	report, code := runStressAt(
		context.Background(), "/repo", stressOptions{Scenario: "05-profile-1m"}, recorder,
	)
	if code != 0 || len(report.Scenarios) != 1 {
		t.Fatalf("profile run = code %d report %#v", code, report)
	}
	scenario := report.Scenarios[0]
	if len(scenario.Profiles) != 3 {
		t.Fatalf("profile results = %#v, want heap, goroutine, and CPU", scenario.Profiles)
	}
	want := map[string]bool{"heap": true, "goroutine": true, "cpu": true}
	for _, profile := range scenario.Profiles {
		if !want[profile.Name] || profile.Bytes != 1024 {
			t.Fatalf("profile result = %#v", profile)
		}
		delete(want, profile.Name)
	}
	if len(want) != 0 {
		t.Fatalf("missing profile results: %v", want)
	}
	startsCPU := false
	for _, event := range recorder.events {
		if strings.Contains(event, "/debug/pprof/profile?seconds=90") {
			startsCPU = true
		}
	}
	if !startsCPU {
		t.Fatalf("profile scenario never started the 90-second CPU capture: %v", recorder.events)
	}
}

func TestStressSelectionRejectsUnknownScenario(t *testing.T) {
	report, code := runStressAt(
		context.Background(), "/repo", stressOptions{Scenario: "missing"}, newStressRecorder(),
	)
	if code != 1 || report.Failure == "" || len(report.Scenarios) != 0 {
		t.Fatalf("unknown scenario = code %d report %#v", code, report)
	}
}

func TestParseStressPeerMetricsPreservesResultBytes(t *testing.T) {
	metrics := parseStressPeerMetrics([]byte(
		"inject built: 4096 messages, 8388608 bytes in 1.25s\n" +
			"inject sent: 8388608 bytes in 250ms (32.0 MB/s)\n",
	))
	if metrics.Messages != 4096 || metrics.Bytes != 8_388_608 || metrics.BuildTime != "1.25s" ||
		metrics.SendTime != "250ms" || metrics.MBps != 32 {
		t.Fatalf("parsed metrics = %#v", metrics)
	}
}

type stressRecorder struct {
	*stressBirdRecorder
	pprof bool
}

func newStressRecorder() *stressRecorder {
	base := newStressBirdRecorder()
	base.routeCounts = []int{100_000, 250_000, 500_000, 1_000_000}
	base.files["/repo/bin/ze"] = true
	for _, scenario := range stressScenarioRegistry {
		base.files["/repo/test/stress/scenarios/"+scenario.name+"/"+scenario.config] = true
	}
	return &stressRecorder{stressBirdRecorder: base}
}

func (r *stressRecorder) Getenv(key string) string {
	if key == "ZE_PPROF" && r.pprof {
		return "1"
	}
	return r.stressBirdRecorder.Getenv(key)
}

func (r *stressRecorder) Start(_ context.Context, command stressBirdCommand) (stressBirdProcess, error) {
	r.commands = append(r.commands, stressBirdCommand{
		argv: slices.Clone(command.argv), dir: command.dir, environ: slices.Clone(command.environ),
		outputPath: command.outputPath, timeout: command.timeout,
	})
	line := "start " + strings.Join(command.argv, " ") + " stdout=" + command.outputPath
	r.events = append(r.events, line)
	name := "service"
	exited := false
	if slices.Contains(command.argv, "peer") {
		name = "peer"
		exited = true
	}
	if slices.Contains(command.argv, "bird") {
		name = "bird"
	}
	return &stressBirdRecordedProcess{name: name, exited: exited, recorder: r.stressBirdRecorder}, nil
}

func (r *stressRecorder) ReadFile(string) ([]byte, error) {
	return []byte(
		"inject built: 4096 messages, 8388608 bytes in 1.25s\n" +
			"inject sent: 8388608 bytes in 250ms (32.0 MB/s)\n",
	), nil
}

func (r *stressRecorder) fileSize(string) (int64, error) { return 1024, nil }

func (r *stressRecorder) MkdirAll(string, os.FileMode) error { return nil }

var _ stressSystem = (*stressRecorder)(nil)
