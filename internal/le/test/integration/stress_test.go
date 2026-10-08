package testintegration

import (
	"context"
	"os"
	"slices"
	"strconv"
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
	if got := StressScenarios(); !slices.Equal(got, want) {
		t.Fatalf("stress scenarios = %q, want %q", got, want)
	}
	if len(stressScenarioRegistry) != 5 {
		t.Fatalf("stress registry has %d scenarios, want 5", len(stressScenarioRegistry))
	}

	bulk := mustStressScenario(t, "01-bulk-ipv4")
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

	mixed := mustStressScenario(t, "02-multi-peer")
	if len(mixed.rounds) != 2 || mixed.rounds[0].prefixes != 500_000 ||
		mixed.rounds[1].prefixes != 250_000 || mixed.rounds[1].nexthop != "2001:db8::3" {
		t.Fatalf("mixed-family scenario drifted: %#v", mixed.rounds)
	}
	flap := mustStressScenario(t, "03-session-flap")
	if len(flap.rounds) != 11 || flap.rounds[9].pause != 2*time.Second ||
		flap.rounds[10].pause != 0 || flap.rounds[10].dwell != "5s" {
		t.Fatalf("flap scenario drifted: %#v", flap.rounds)
	}
	profile := mustStressScenario(t, "05-profile-1m")
	if len(profile.rounds) != 1 || profile.rounds[0].prefixes != 1_000_000 ||
		profile.rounds[0].dwell != "60s" || profile.rounds[0].timeout != 600*time.Second {
		t.Fatalf("profile scenario drifted: %#v", profile.rounds)
	}
}

// mustStressScenario answers the registered scenario. A name the registry does
// not carry fails the test, so a renamed scenario cannot pass as a zero value.
func mustStressScenario(t *testing.T, name string) stressScenario {
	t.Helper()
	scenario, found := stressScenarioNamed(name)
	if !found {
		t.Fatalf("stress registry has no scenario %q", name)
	}
	return scenario
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
		case strings.Contains(event, "/bin/le test peer --mode inject"):
			peerStarts++
		case strings.Contains(event, "/"+stressZeBinaryRel+" start "):
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

// TestStressHarnessBuildsZeFromCheckoutEveryRun pins that a run measures the
// tree it was started from. Method: leave a bin/ze in the fixture, run one
// scenario, and require a build with the manifest's gate tags into the
// runner's own path, and that the DUT started is that build.
func TestStressHarnessBuildsZeFromCheckoutEveryRun(t *testing.T) {
	recorder := newStressRecorder()
	report, code := runStressAt(
		context.Background(), "/repo", stressOptions{Scenario: "01-bulk-ipv4"}, recorder,
	)
	if code != 0 || report.Passed != 1 {
		t.Fatalf("run = code %d report %#v", code, report)
	}
	built := "/repo/" + stressZeBinaryRel
	if got := report.Scenarios[0].Binary; got != built {
		t.Fatalf("report binary = %q, want %q", got, built)
	}
	var build *stressBirdCommand
	for i := range recorder.commands {
		if len(recorder.commands[i].argv) > 0 && recorder.commands[i].argv[0] == "go" {
			build = &recorder.commands[i]
			break
		}
	}
	if build == nil {
		t.Fatal("a present bin/ze was reused: the run never built the DUT from the checkout")
	}
	want := []string{"go", "build", "-tags", stressRecorderTags, "-o", built, "./cmd/ze"}
	if !slices.Equal(build.argv, want) {
		t.Fatalf("build argv = %q, want %q", build.argv, want)
	}
	if build.dir != "/repo" || !slices.Contains(build.environ, "CGO_ENABLED=0") {
		t.Fatalf("build command = dir %q env %q", build.dir, build.environ)
	}
	for _, event := range recorder.events {
		if strings.Contains(event, "/repo/bin/ze") && strings.Contains(event, " start ") {
			t.Fatalf("the DUT started from bin/ze: %s", event)
		}
	}
}

// TestStressProfileScenarioReachesRoundThreePaths pins the three things the
// profile scenario adds so its profile carries the paths perf round 3 changed:
// an eBGP receiver the DUT forwards to, and a best-table query made while the
// injector still holds its routes. Method: record the run and read the order of
// its commands.
func TestStressProfileScenarioReachesRoundThreePaths(t *testing.T) {
	recorder := newStressRecorder()
	report, code := runStressAt(
		context.Background(), "/repo", stressOptions{Scenario: scenarioProfile1M}, recorder,
	)
	if code != 0 || len(report.Scenarios) != 1 {
		t.Fatalf("profile run = code %d report %#v", code, report)
	}
	reach := stressProfileReach
	address, sink, inject, query, zeStart := -1, -1, -1, -1, -1
	for i, event := range recorder.events {
		switch {
		case strings.Contains(event, "ip addr add "+reach.receiverCIDR+" dev "):
			address = i
		case strings.Contains(event, "--mode sink --bind "+reach.receiverIP+" --port 179 --asn 65200"):
			sink = i
		case strings.Contains(event, "--mode inject"):
			inject = i
		case strings.Contains(event, "curl") && strings.Contains(event, reach.queryURL):
			query = i
		case strings.Contains(event, " start "+stressConfigDir("fixture")+"/"):
			zeStart = i
		}
	}
	if address < 0 || sink < 0 || query < 0 || inject < 0 || zeStart < 0 {
		t.Fatalf("missing step: address %d sink %d inject %d query %d ze %d\n%s",
			address, sink, inject, query, zeStart, strings.Join(recorder.events, "\n"))
	}
	if address > sink || sink > zeStart || zeStart > inject || inject > query {
		t.Fatalf("steps out of order: address %d sink %d ze %d inject %d query %d",
			address, sink, zeStart, inject, query)
	}
	queries := report.Scenarios[0].Queries
	if len(queries) != 1 || queries[0].Bytes != stressRecorderQueryBytes || queries[0].Path != reach.queryURL {
		t.Fatalf("queries = %#v", queries)
	}
}

// TestStressDUTRunsOnARunPrivateConfigCopy pins that the DUT never opens its
// config store in the checkout. Ze keeps the store beside its config and
// refuses one another user owns, and a VM guest sees the checkout over 9p with
// the host user's uid while the DUT runs as root. Method: record a run and read
// where the config was written, with what mode, what Ze was started on, and
// that the copy is removed before and after the run.
func TestStressDUTRunsOnARunPrivateConfigCopy(t *testing.T) {
	recorder := newStressRecorder()
	source := "/repo/test/stress/scenarios/" + scenarioProfile1M + "/" + zeConfigFile
	recorder.sources[source] = []byte("bgp { }\n")
	report, code := runStressAt(
		context.Background(), "/repo", stressOptions{Scenario: scenarioProfile1M}, recorder,
	)
	if code != 0 || len(report.Scenarios) != 1 {
		t.Fatalf("profile run = code %d report %#v", code, report)
	}
	dir := stressConfigDir("fixture")
	if strings.HasPrefix(dir, "/repo") {
		t.Fatalf("config copy %q sits in the checkout", dir)
	}
	copied := dir + "/" + zeConfigFile
	write, ok := recorder.written[copied]
	if !ok {
		t.Fatalf("no config copy written at %q\n%s", copied, strings.Join(recorder.events, "\n"))
	}
	if string(write.content) != "bgp { }\n" || write.mode != 0o600 {
		t.Fatalf("copy = %q mode %v, want the scenario config at 0600", write.content, write.mode)
	}
	clear, mkdir, wrote, start, removed := -1, -1, -1, -1, -1
	for i, event := range recorder.events {
		switch {
		case event == "remove-all "+dir && clear < 0:
			clear = i
		case event == "remove-all "+dir:
			removed = i
		case event == "mkdir "+dir+" "+os.FileMode(0o700).String():
			mkdir = i
		case event == "write "+copied:
			wrote = i
		case strings.Contains(event, " start "+copied):
			start = i
		case strings.Contains(event, " start "+source):
			t.Fatalf("the DUT started on the checkout's config: %s", event)
		}
	}
	if clear < 0 || mkdir < 0 || wrote < 0 || start < 0 || removed < 0 {
		t.Fatalf("missing step: clear %d mkdir %d write %d start %d remove %d\n%s",
			clear, mkdir, wrote, start, removed, strings.Join(recorder.events, "\n"))
	}
	if clear > mkdir || mkdir > wrote || wrote > start || start > removed {
		t.Fatalf("steps out of order: clear %d mkdir %d write %d start %d remove %d",
			clear, mkdir, wrote, start, removed)
	}
}

// TestStressProfileQueryRefusesAnEmptyTable pins that a best table the looking
// glass answers with no bytes fails the run rather than reading as reach.
func TestStressProfileQueryRefusesAnEmptyTable(t *testing.T) {
	recorder := newStressRecorder()
	recorder.queryBytes = "0"
	report, code := runStressAt(
		context.Background(), "/repo", stressOptions{Scenario: scenarioProfile1M}, recorder,
	)
	if code == 0 || report.Scenarios[0].Passed || !strings.Contains(report.Scenarios[0].Failure, "empty best table") {
		t.Fatalf("empty query = code %d report %#v", code, report)
	}
}

// TestStressProfileQueryWaitsForAPopulatedTable pins that the best-table query
// waits until the RIB holds a route. The injector's last byte reaches the DUT's
// socket before the RIB has stored anything, and a query made then measured an
// empty table: a 2026-10-08 guest smoke answered 122 bytes for 20000 prefixes.
// Method: answer two empty probes, then a populated one, and read the order.
func TestStressProfileQueryWaitsForAPopulatedTable(t *testing.T) {
	recorder := newStressRecorder()
	recorder.probeTotals = []int{0, 0, 5}
	report, code := runStressAt(
		context.Background(), "/repo", stressOptions{Scenario: scenarioProfile1M}, recorder,
	)
	if code != 0 || len(report.Scenarios) != 1 {
		t.Fatalf("profile run = code %d report %#v", code, report)
	}
	if recorder.probes != 3 {
		t.Fatalf("probes = %d, want 3 (two empty, one populated)", recorder.probes)
	}
	lastProbe, query := -1, -1
	for i, event := range recorder.events {
		if strings.Contains(event, stressProfileReach.probeURL()) {
			lastProbe = i
		}
		if strings.Contains(event, "%{size_download}") {
			query = i
		}
	}
	if query < lastProbe {
		t.Fatalf("full query %d ran before the populated probe %d", query, lastProbe)
	}
	queries := report.Scenarios[0].Queries
	if len(queries) != 1 || queries[0].Routes != 5 {
		t.Fatalf("queries = %#v, want 5 routes recorded", queries)
	}
}

// TestStressProfileQueryRefusesATableThatStaysEmpty pins that a best table with
// no route for the whole round fails the run, rather than reading as reach.
func TestStressProfileQueryRefusesATableThatStaysEmpty(t *testing.T) {
	recorder := newStressRecorder()
	recorder.probeTotals = []int{0}
	report, code := runStressAt(
		context.Background(), "/repo", stressOptions{Scenario: scenarioProfile1M}, recorder,
	)
	scenario := report.Scenarios[0]
	if code == 0 || scenario.Passed || scenario.ExitCode != stressBirdTimeoutCode ||
		!strings.Contains(scenario.Failure, "best table stayed empty") {
		t.Fatalf("empty table = code %d report %#v", code, report)
	}
}

// TestStressProfileConfigMatchesTheHarness pins the profile scenario's ze.conf
// to the addresses, AS and listener the harness drives, and to the policies
// that make the filter delta run on both sides. Method: read the real file.
func TestStressProfileConfigMatchesTheHarness(t *testing.T) {
	content, err := os.ReadFile("../../../../test/stress/scenarios/" + scenarioProfile1M + "/" + zeConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	config := string(content)
	for _, want := range []string{
		"ip " + stressProfileReach.receiverIP + ";",
		"remote 65200;",
		"import [ modify:STRESS-IMPORT ];",
		"export [ modify:STRESS-EXPORT ];",
		"community-add [",
		"use bgp-rs;",
		"use bgp-rib;",
		"use bgp-filter-modify;",
		"tls false;",
		"ip 127.0.0.1;",
		"port 8443;",
	} {
		if !strings.Contains(config, want) {
			t.Fatalf("%s/%s lacks %q", scenarioProfile1M, zeConfigFile, want)
		}
	}
	if !strings.HasPrefix(stressProfileReach.queryURL, "http://127.0.0.1:8443/") {
		t.Fatalf("query URL %q does not name the configured looking glass", stressProfileReach.queryURL)
	}
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

// TestStressPrefixesShortensEveryRound pins the smoke knob: a positive count
// replaces each round's count for the run, and the registry keeps its own.
func TestStressPrefixesShortensEveryRound(t *testing.T) {
	recorder := newStressRecorder()
	report, code := runStressAt(
		context.Background(), "/repo", stressOptions{Scenario: "01-bulk-ipv4", Prefixes: 1}, recorder,
	)
	if code != 0 || len(report.Scenarios) != 1 || len(report.Scenarios[0].Rounds) != 4 {
		t.Fatalf("smoke run = code %d report %#v", code, report)
	}
	for _, round := range report.Scenarios[0].Rounds {
		if round.Prefixes != 1 {
			t.Fatalf("round prefixes = %d, want 1", round.Prefixes)
		}
	}
	if mustStressScenario(t, "01-bulk-ipv4").rounds[0].prefixes != 100_000 {
		t.Fatal("the smoke count leaked into the registry")
	}
}

// TestStressPrefixesRefusesANegativeCount pins the boundary below the knob's
// valid range: -1 is refused before any scenario runs, 0 keeps the registry.
func TestStressPrefixesRefusesANegativeCount(t *testing.T) {
	report, code := runStressAt(
		context.Background(), "/repo", stressOptions{Scenario: "01-bulk-ipv4", Prefixes: -1}, newStressRecorder(),
	)
	if code != 1 || len(report.Scenarios) != 0 || !strings.Contains(report.Failure, "negative") {
		t.Fatalf("negative prefixes = code %d report %#v", code, report)
	}
}

// TestStressBuildFailureCarriesTheBuildOutput pins that a DUT build which
// fails says why. A smoke run once reported `build Ze: ` and nothing else,
// because the compiler exited non-zero with an empty stderr and the message
// was stderr alone. Method: fail the recorded build four ways and read the
// scenario's failure.
func TestStressBuildFailureCarriesTheBuildOutput(t *testing.T) {
	var long strings.Builder
	for line := 1; line <= 30; line++ {
		long.WriteString("build line ")
		if line < 10 {
			long.WriteString("0")
		}
		long.WriteString(strconv.Itoa(line))
		long.WriteString("\n")
	}
	cases := []struct {
		name    string
		result  stressBirdCommandResult
		want    []string
		wantNot []string
	}{
		{
			name:   "stderr",
			result: stressBirdCommandResult{stderr: "cmd/ze/main.go:3:2: undefined: zeMain\n", code: 1},
			want:   []string{"exited 1", "undefined: zeMain"},
		},
		{
			name:   "stdout only",
			result: stressBirdCommandResult{stdout: "go: downloading go1.26.0\n", code: 1},
			want:   []string{"exited 1", "go: downloading go1.26.0"},
		},
		{
			name:   "silent",
			result: stressBirdCommandResult{code: 2},
			want:   []string{"exited 2", "printed nothing"},
		},
		{
			name:    "long stderr keeps its tail",
			result:  stressBirdCommandResult{stderr: long.String(), code: 1},
			want:    []string{"build line 30", "build line 11"},
			wantNot: []string{"build line 10"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := newStressRecorder()
			recorder.build = &tc.result
			report, code := runStressAt(
				context.Background(), "/repo", stressOptions{Scenario: "01-bulk-ipv4"}, recorder,
			)
			if code == 0 || len(report.Scenarios) != 1 {
				t.Fatalf("failed build = code %d report %#v", code, report)
			}
			failure := report.Scenarios[0].Failure
			for _, want := range tc.want {
				if !strings.Contains(failure, want) {
					t.Fatalf("failure %q does not carry %q", failure, want)
				}
			}
			for _, unwanted := range tc.wantNot {
				if strings.Contains(failure, unwanted) {
					t.Fatalf("failure %q carries %q, which is past the tail", failure, unwanted)
				}
			}
		})
	}
}

// TestStressCleanupReportsOnlyANamespaceLeftBehind pins what cleanup calls a
// problem. `ip netns del` exits 1 for a namespace that does not exist, and a
// run that fails before it creates its namespaces still cleans them up, so the
// smoke run reported two deletions that had nothing to delete. Method: fail the
// build so no namespace is created, fail both deletions, and read the report
// with and without a namespace file still present.
func TestStressCleanupReportsOnlyANamespaceLeftBehind(t *testing.T) {
	for _, leftBehind := range []bool{false, true} {
		recorder := newStressRecorder()
		recorder.build = &stressBirdCommandResult{stderr: "boom\n", code: 1}
		recorder.runCodes["run ip netns del ze-stress-ze-fixture"] = 1
		recorder.runCodes["run ip netns del ze-stress-bb-fixture"] = 1
		if leftBehind {
			recorder.files[stressNamespacePath("ze-stress-bb-fixture")] = true
		}
		report, _ := runStressAt(
			context.Background(), "/repo", stressOptions{Scenario: "01-bulk-ipv4"}, recorder,
		)
		problems := report.Scenarios[0].CleanupErrors
		if !leftBehind {
			if len(problems) != 0 {
				t.Fatalf("absent namespaces reported as cleanup errors: %q", problems)
			}
			continue
		}
		if len(problems) != 1 || !strings.Contains(problems[0], "namespace ze-stress-bb-fixture left behind") {
			t.Fatalf("left-behind namespace = %q, want one problem naming ze-stress-bb-fixture", problems)
		}
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
	pprof      bool
	queryBytes string
	// build, when set, answers the DUT's compile in place of a success.
	build *stressBirdCommandResult
	// sources answers ReadFile for the paths it holds; written records each
	// WriteFile by path, with its content and mode.
	sources map[string][]byte
	written map[string]stressRecordedWrite
	// probeTotals answers each successive best-table probe with this route
	// total; the last entry repeats. probes counts the probes made.
	probeTotals []int
	probes      int
}

type stressRecordedWrite struct {
	content []byte
	mode    os.FileMode
}

// stressRecorderTags stands in for the manifest's gate tags, and
// stressRecorderQueryBytes for the size of the looking glass's best table.
const (
	stressRecorderTags       = "ze_core ze_distro ze_bgp ze_lg"
	stressRecorderQueryBytes = 4096
)

func (r *stressRecorder) daemonBuildTags(string) (string, error) { return stressRecorderTags, nil }

func (r *stressRecorder) Run(ctx context.Context, command stressBirdCommand) (stressBirdCommandResult, error) {
	result, err := r.stressBirdRecorder.Run(ctx, command)
	if r.build != nil && command.argv[0] == "go" {
		return *r.build, nil
	}
	if slices.Contains(command.argv, "curl") && slices.Contains(command.argv, "%{size_download}") {
		result.stdout = r.queryBytes
	}
	if slices.Contains(command.argv, "curl") && slices.Contains(command.argv, stressProfileReach.probeURL()) {
		total := r.probeTotals[min(r.probes, len(r.probeTotals)-1)]
		r.probes++
		result.stdout = `{"routes":[],"pagination":{"total_results":` + strconv.Itoa(total) + `}}`
	}
	return result, err
}

func newStressRecorder() *stressRecorder {
	base := newStressBirdRecorder()
	base.routeCounts = []int{100_000, 250_000, 500_000, 1_000_000}
	base.files["/repo/bin/ze"] = true
	for _, scenario := range stressScenarioRegistry {
		base.files["/repo/test/stress/scenarios/"+scenario.name+"/"+scenario.config] = true
	}
	return &stressRecorder{
		stressBirdRecorder: base, queryBytes: "4096",
		sources: map[string][]byte{}, written: map[string]stressRecordedWrite{},
		probeTotals: []int{5},
	}
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

func (r *stressRecorder) ReadFile(path string) ([]byte, error) {
	if content, ok := r.sources[path]; ok {
		return content, nil
	}
	return []byte(
		"inject built: 4096 messages, 8388608 bytes in 1.25s\n" +
			"inject sent: 8388608 bytes in 250ms (32.0 MB/s)\n",
	), nil
}

func (r *stressRecorder) fileSize(string) (int64, error) { return 1024, nil }

func (r *stressRecorder) MkdirAll(path string, mode os.FileMode) error {
	r.events = append(r.events, "mkdir "+path+" "+mode.String())
	return nil
}

func (r *stressRecorder) WriteFile(path string, content []byte, mode os.FileMode) error {
	r.events = append(r.events, "write "+path)
	r.written[path] = stressRecordedWrite{content: slices.Clone(content), mode: mode}
	return nil
}

func (r *stressRecorder) RemoveAll(path string) error {
	r.events = append(r.events, "remove-all "+path)
	return nil
}

var _ stressSystem = (*stressRecorder)(nil)
