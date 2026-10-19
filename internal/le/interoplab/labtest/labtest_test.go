package labtest

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// fakeTester records what start asked of the test. Skipf and Fatalf end the
// calling goroutine as the testing package does, so start never runs past them.
type fakeTester struct {
	ctx      context.Context
	cleanups []func()
	failed   bool
	skipped  string
	fatal    string
	logs     []string
	errors   []string
}

func (f *fakeTester) Helper()                  {}
func (f *fakeTester) Context() context.Context { return f.ctx }
func (f *fakeTester) Cleanup(fn func())        { f.cleanups = append(f.cleanups, fn) }
func (f *fakeTester) Failed() bool             { return f.failed }
func (f *fakeTester) Logf(format string, args ...any) {
	f.logs = append(f.logs, fmt.Sprintf(format, args...))
}

func (f *fakeTester) Errorf(format string, args ...any) {
	f.failed = true
	f.errors = append(f.errors, fmt.Sprintf(format, args...))
}

func (f *fakeTester) Skipf(format string, args ...any) {
	f.skipped = fmt.Sprintf(format, args...)
	runtime.Goexit()
}

func (f *fakeTester) Fatalf(format string, args ...any) {
	f.failed = true
	f.fatal = fmt.Sprintf(format, args...)
	runtime.Goexit()
}

// runStart drives start on its own goroutine, so a Skipf or Fatalf ends it
// there, then runs the registered cleanups last-in first-out, as testing does.
func runStart(t *testing.T, tester *fakeTester, h host, spec Spec) netip.AddrPort {
	t.Helper()
	var address netip.AddrPort
	done := make(chan struct{})
	go func() {
		defer close(done)
		address = start(tester, h, spec).Address
	}()
	<-done
	for _, cleanup := range slices.Backward(tester.cleanups) {
		cleanup()
	}
	return address
}

// scriptedRunner answers each docker verb from a table and records every argv.
type scriptedRunner struct {
	commands [][]string
	fail     map[string]bool
	// exits answers a verb with its exit code, before fail is read.
	exits map[string]int
	// absent answers a verb the way the daemon answers an inspect of a
	// reference it does not hold, before exits is read.
	absent map[string]bool
}

func (r *scriptedRunner) Run(_ context.Context, command interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
	r.commands = append(r.commands, command.Arguments)
	verb := command.Arguments[1]
	if r.absent[verb] {
		reference := command.Arguments[len(command.Arguments)-1]
		return interoplab.ProcessResult{Stderr: "Error response from daemon: No such image: " + reference, ExitCode: 1}, nil
	}
	if code, ok := r.exits[verb]; ok {
		return interoplab.ProcessResult{Stderr: verb + " exited by the fixture", ExitCode: code}, nil
	}
	if r.fail[verb] {
		return interoplab.ProcessResult{Stderr: verb + " refused by the fixture", ExitCode: 1}, nil
	}
	switch verb {
	case "port":
		return interoplab.ProcessResult{Stdout: "127.0.0.1:49153\n[::1]:49153\n"}, nil
	case "build":
		return interoplab.ProcessResult{Stdout: "sha256:fixture\n"}, nil
	case "logs":
		return interoplab.ProcessResult{Stdout: "stayrtr: fetched cache\n"}, nil
	}
	return interoplab.ProcessResult{}, nil
}

func (r *scriptedRunner) verbs() []string {
	verbs := make([]string, 0, len(r.commands))
	for _, argv := range r.commands {
		verbs = append(verbs, argv[1])
	}
	return verbs
}

func fixtureHost(runner *scriptedRunner, docker bool, environment map[string]string) host {
	return host{
		docker: interoplab.NewFixtureDocker(runner),
		lookPath: func(string) (string, error) {
			if docker {
				return "/usr/bin/docker", nil
			}
			return "", errors.New("executable file not found in $PATH")
		},
		lookupEnv: func(name string) (string, bool) {
			value, ok := environment[name]
			return value, ok
		},
		moduleRoot: func() (string, error) { return "/module", nil },
		suffix:     func() string { return "fixture" },
	}
}

func fixtureSpec() Spec {
	return Spec{
		Name: "stayrtr", Image: "rpki/stayrtr:latest", Command: []string{"-bind", ":3323"},
		Port: interoplab.TCPPort(3323), Ready: func(context.Context, netip.AddrPort) error { return nil },
		ReadyTimeout: time.Second, LogLines: 50,
	}
}

// VALIDATES: AC-19, an absent docker binary: a skip naming the install route
// on an ad hoc run, and a failure naming the same fix under ZE_REQUIRE_DOCKER=1,
// with no docker command run in either arm.
// PREVENTS: a gating run passing because Docker was absent, or an ad hoc run
// on a machine without Docker failing for a reason it cannot fix in the test.
func TestLabtestSkipsOrFailsOnAbsentDocker(t *testing.T) {
	adhoc := &fakeTester{ctx: t.Context()}
	runner := &scriptedRunner{}
	runStart(t, adhoc, fixtureHost(runner, false, nil), fixtureSpec())
	if adhoc.skipped == "" || adhoc.failed {
		t.Fatalf("ad hoc run: skipped %q, failed %v; want a skip", adhoc.skipped, adhoc.failed)
	}
	if !strings.Contains(adhoc.skipped, "install Docker") {
		t.Errorf("skip %q names no install route", adhoc.skipped)
	}

	gating := &fakeTester{ctx: t.Context()}
	runStart(t, gating, fixtureHost(runner, false, map[string]string{requireDockerVariable: "1"}), fixtureSpec())
	if gating.fatal == "" || gating.skipped != "" {
		t.Fatalf("gating run: fatal %q, skipped %q; want a failure", gating.fatal, gating.skipped)
	}
	for _, want := range []string{"install Docker", requireDockerVariable} {
		if !strings.Contains(gating.fatal, want) {
			t.Errorf("failure %q does not name %q", gating.fatal, want)
		}
	}
	if len(runner.commands) != 0 {
		t.Errorf("docker ran %v with no docker binary", runner.commands)
	}
}

// VALIDATES: AC-19, a docker binary whose daemon does not answer fails under
// both switches, naming the probe error and the fix.
// PREVENTS: an unreachable daemon read as "no Docker here" and skipped.
func TestLabtestFailsWhenProbeFails(t *testing.T) {
	for _, environment := range []map[string]string{nil, {requireDockerVariable: "1"}} {
		tester := &fakeTester{ctx: t.Context()}
		runner := &scriptedRunner{fail: map[string]bool{"info": true}}
		runStart(t, tester, fixtureHost(runner, true, environment), fixtureSpec())
		if tester.fatal == "" {
			t.Fatalf("environment %v: a failing probe did not fail the test (skipped %q)", environment, tester.skipped)
		}
		for _, want := range []string{"info refused by the fixture", "docker info"} {
			if !strings.Contains(tester.fatal, want) {
				t.Errorf("failure %q does not name %q", tester.fatal, want)
			}
		}
	}
}

// VALIDATES: AC-19 and Q-6, an image that cannot be pulled: a failure under
// the docker tag; under live a skip ad hoc and a failure with
// ZE_REQUIRE_DOCKER=1. No container starts in any arm.
// PREVENTS: a gating run passing on a registry outage, or a live test failing
// ad hoc on a machine with no internet.
func TestLabtestPullFailurePolicy(t *testing.T) {
	cases := []struct {
		name      string
		liveSkips bool
		require   bool
		wantSkip  bool
	}{
		{name: "docker tag", liveSkips: false, require: false, wantSkip: false},
		{name: "docker tag required", liveSkips: false, require: true, wantSkip: false},
		{name: "live ad hoc", liveSkips: true, require: false, wantSkip: true},
		{name: "live required", liveSkips: true, require: true, wantSkip: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			environment := map[string]string{}
			if tc.require {
				environment[requireDockerVariable] = "1"
			}
			runner := &scriptedRunner{fail: map[string]bool{"pull": true}}
			h := fixtureHost(runner, true, environment)
			h.pullFailureSkips = tc.liveSkips
			tester := &fakeTester{ctx: t.Context()}
			runStart(t, tester, h, fixtureSpec())
			skipped := tester.skipped != ""
			if skipped != tc.wantSkip {
				t.Fatalf("skipped %v (%q), fatal %q; want skip %v", skipped, tester.skipped, tester.fatal, tc.wantSkip)
			}
			if !skipped && !strings.Contains(tester.fatal, "pull refused by the fixture") {
				t.Errorf("failure %q does not name the pull error", tester.fatal)
			}
			if slices.Contains(runner.verbs(), "run") {
				t.Errorf("a container started after a failed pull: %v", runner.verbs())
			}
		})
	}
}

// VALIDATES: AC-18 and AC-26 over the fake runner: a passing start runs
// exactly info, pull, a labeled loopback-published run and port, answers the
// published address, and its cleanup removes the container; no Docker host
// kernel check runs, because Ze runs in the test process, not in a container.
// PREVENTS: a labtest run paying for or failing on the Ze-kernel check, or a
// container left behind by a test that passed.
func TestLabtestRunsNoKernelCheck(t *testing.T) {
	runner := &scriptedRunner{}
	tester := &fakeTester{ctx: t.Context()}
	address := runStart(t, tester, fixtureHost(runner, true, nil), fixtureSpec())
	if tester.failed || tester.skipped != "" {
		t.Fatalf("start failed %q / skipped %q / errors %v", tester.fatal, tester.skipped, tester.errors)
	}
	if want := netip.MustParseAddrPort("127.0.0.1:49153"); address != want {
		t.Errorf("address = %v, want %v", address, want)
	}
	if want := []string{"info", "pull", "run", "port", "rm"}; !slices.Equal(runner.verbs(), want) {
		t.Fatalf("docker verbs = %v, want exactly %v", runner.verbs(), want)
	}
	run := strings.Join(runner.commands[2], " ")
	for _, want := range []string{"--name ze-labtest-stayrtr-fixture", "-p 127.0.0.1::3323/tcp", "ze.lab"} {
		if !strings.Contains(run, want) {
			t.Errorf("run argv %q lacks %q", run, want)
		}
	}
	if remove := runner.commands[4]; remove[len(remove)-1] != "ze-labtest-stayrtr-fixture" {
		t.Errorf("cleanup removed %v, not the container it started", remove)
	}
}

// VALIDATES: AC-18, a host-network server: no port is published or asked
// for, and the address is the loopback at the server's own port.
// PREVENTS: a host-network container read through `docker port`, which has
// no binding to report.
func TestLabtestHostNetworkAnswersTheLoopback(t *testing.T) {
	runner := &scriptedRunner{}
	tester := &fakeTester{ctx: t.Context()}
	spec := fixtureSpec()
	spec.HostNetwork = true
	address := runStart(t, tester, fixtureHost(runner, true, nil), spec)
	if want := netip.MustParseAddrPort("127.0.0.1:3323"); address != want {
		t.Errorf("address = %v, want %v", address, want)
	}
	if want := []string{"info", "pull", "run", "rm"}; !slices.Equal(runner.verbs(), want) {
		t.Fatalf("docker verbs = %v, want exactly %v", runner.verbs(), want)
	}
	if run := strings.Join(runner.commands[2], " "); !strings.Contains(run, "--network host") {
		t.Errorf("run argv %q is not on the host network", run)
	}
}

// VALIDATES: AC-18, a test that failed gets the last LogLines lines of the
// container log before the container is removed; one that passed gets none.
// PREVENTS: a red with no trace of what the server said, or a log read after
// the container is already gone.
func TestLabtestDumpsLogsOnFailure(t *testing.T) {
	runner := &scriptedRunner{}
	tester := &fakeTester{ctx: t.Context()}
	spec := fixtureSpec()
	spec.Ready = func(context.Context, netip.AddrPort) error { return errors.New("connection refused") }
	spec.ReadyTimeout = 50 * time.Millisecond
	runStart(t, tester, fixtureHost(runner, true, nil), spec)
	if !strings.Contains(tester.fatal, "connection refused") {
		t.Fatalf("failure %q does not name the ready probe's error", tester.fatal)
	}
	if want := []string{"info", "pull", "run", "port", "logs", "rm"}; !slices.Equal(runner.verbs(), want) {
		t.Fatalf("docker verbs = %v, want exactly %v", runner.verbs(), want)
	}
	if logs := runner.commands[4]; !slices.Contains(logs, "50") {
		t.Errorf("logs argv %v does not ask for the spec's 50 lines", logs)
	}
	if !strings.Contains(strings.Join(tester.logs, "\n"), "stayrtr: fetched cache") {
		t.Errorf("the container log never reached t.Log: %v", tester.logs)
	}
}

// VALIDATES: the spec boundaries: LogLines 1..1000, a positive ready timeout,
// a ready probe, an image, a non-zero port and a name docker accepts.
// PREVENTS: a spec that cannot work reaching the daemon as an opaque failure.
func TestLabtestRefusesAnUnusableSpec(t *testing.T) {
	valid := fixtureSpec()
	valid.LogLines = 1000
	if err := valid.validate(); err != nil {
		t.Fatalf("LogLines 1000 refused: %v", err)
	}
	cases := map[string]func(*Spec){
		"log lines 0":    func(s *Spec) { s.LogLines = 0 },
		"log lines 1001": func(s *Spec) { s.LogLines = 1001 },
		"no image":       func(s *Spec) { s.Image = "" },
		"port 0":         func(s *Spec) { s.Port.Number = 0 },
		"no transport":   func(s *Spec) { s.Port.Transport = interoplab.TransportUnspecified },
		"no ready":       func(s *Spec) { s.Ready = nil },
		"two ready":      func(s *Spec) { s.ReadyInContainer = []string{"true"} },
		"ready timeout":  func(s *Spec) { s.ReadyTimeout = 0 },
		"no name":        func(s *Spec) { s.Name = "" },
		"name uppercase": func(s *Spec) { s.Name = "StayRTR" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			spec := fixtureSpec()
			mutate(&spec)
			if err := spec.validate(); err == nil {
				t.Fatal("validate accepted the spec")
			}
		})
	}
}

// runClient drives run on its own goroutine, as runStart drives start.
func runClient(t *testing.T, tester *fakeTester, h host, client Client) interoplab.CommandResult {
	t.Helper()
	var result interoplab.CommandResult
	done := make(chan struct{})
	go func() {
		defer close(done)
		result = run(tester, h, client)
	}()
	<-done
	return result
}

func fixtureClient() Client {
	return Client{
		Name: "gnmic", Image: "ghcr.io/openconfig/gnmic:fixture", Command: []string{"capabilities"},
		HostNetwork: true, Mounts: []interoplab.Mount{{Source: "/host/certs", Target: "/certs", ReadOnly: true}},
		Timeout: time.Second,
	}
}

// VALIDATES: a client's own non-zero exit is answered to the test, not
// failed, with the host network and the mount on the one run it made.
// PREVENTS: a test asserting a refused request failing before it can read the
// refusal, or a client that cannot reach the in-process server.
func TestLabtestClientAnswersItsOwnExit(t *testing.T) {
	runner := &scriptedRunner{exits: map[string]int{"run": 1}}
	tester := &fakeTester{ctx: t.Context()}
	result := runClient(t, tester, fixtureHost(runner, true, nil), fixtureClient())
	if tester.failed {
		t.Fatalf("a client exit 1 failed the test: %q", tester.fatal)
	}
	if result.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want the client's 1", result.ExitCode)
	}
	if !slices.Equal(runner.verbs(), []string{"info", "pull", "run"}) {
		t.Fatalf("docker verbs = %v, want info, pull, run", runner.verbs())
	}
	argv := strings.Join(runner.commands[2], " ")
	if !strings.Contains(argv, "--network host") {
		t.Errorf("run argv %q carries no --network host", argv)
	}
	if !strings.Contains(argv, "-v /host/certs:/certs:ro") {
		t.Errorf("run argv %q carries no read-only mount", argv)
	}
}

// VALIDATES: an exit code docker keeps for itself fails the test, and so does
// a client that names no timeout, before any docker command runs.
// PREVENTS: a refused mount or a missing image read as the client refusing.
func TestLabtestClientFailsWhenDockerFails(t *testing.T) {
	runner := &scriptedRunner{exits: map[string]int{"run": 125}}
	tester := &fakeTester{ctx: t.Context()}
	runClient(t, tester, fixtureHost(runner, true, nil), fixtureClient())
	if !strings.Contains(tester.fatal, "docker run exited 125") {
		t.Errorf("fatal = %q, want it to name docker's exit 125", tester.fatal)
	}

	unbounded := fixtureClient()
	unbounded.Timeout = 0
	refused := &scriptedRunner{}
	tester = &fakeTester{ctx: t.Context()}
	runClient(t, tester, fixtureHost(refused, true, nil), unbounded)
	if !strings.Contains(tester.fatal, "no positive timeout") {
		t.Errorf("fatal = %q, want it to name the missing timeout", tester.fatal)
	}
	if len(refused.commands) != 0 {
		t.Errorf("a refused client ran docker verbs %v", refused.verbs())
	}
}

// VALIDATES: a Spec.Dockerfile image is built from the module root, with the
// Dockerfile's directory as its context, never pulled, and the tag the run
// owns is released after the container is removed.
// PREVENTS: a test of a locally built peer failing on `docker pull` of a tag
// no registry holds, or a run-owned image tag outliving the test.
func TestLabtestBuildsADockerfileImage(t *testing.T) {
	runner := &scriptedRunner{}
	tester := &fakeTester{ctx: t.Context()}
	spec := fixtureSpec()
	spec.Image = "ze-collectors:test"
	spec.Dockerfile = "test/interop-collectors/Dockerfile"
	runStart(t, tester, fixtureHost(runner, true, nil), spec)
	if tester.fatal != "" {
		t.Fatalf("start failed: %s", tester.fatal)
	}
	if want := []string{"info", "build", "run", "port", "rm", "image"}; !slices.Equal(runner.verbs(), want) {
		t.Fatalf("docker verbs = %v, want exactly %v", runner.verbs(), want)
	}
	build := strings.Join(runner.commands[1], " ")
	if !strings.Contains(build, "-f /module/test/interop-collectors/Dockerfile /module/test/interop-collectors") {
		t.Errorf("build argv %q does not build the Dockerfile from the module root in its own directory", build)
	}
	if !strings.Contains(build, "-t ze-collectors:test") {
		t.Errorf("build argv %q does not tag the spec's image", build)
	}
}

// VALIDATES: a failed build fails the test, naming the image and the
// Dockerfile, and starts no container.
// PREVENTS: a run that reports a missing container when the build was what
// failed.
func TestLabtestFailsWhenTheBuildFails(t *testing.T) {
	runner := &scriptedRunner{fail: map[string]bool{"build": true}}
	tester := &fakeTester{ctx: t.Context()}
	spec := fixtureSpec()
	spec.Image = "ze-collectors:test"
	spec.Dockerfile = "test/interop-collectors/Dockerfile"
	runStart(t, tester, fixtureHost(runner, true, nil), spec)
	if !strings.Contains(tester.fatal, "cannot build ze-collectors:test from test/interop-collectors/Dockerfile") {
		t.Fatalf("failure %q does not name the image and its Dockerfile", tester.fatal)
	}
	if want := []string{"info", "build"}; !slices.Equal(runner.verbs(), want) {
		t.Fatalf("docker verbs = %v, want exactly %v", runner.verbs(), want)
	}
}

// VALIDATES: ReadyInContainer runs its command in the started container,
// ready on exit 0 and never ready on a non-zero exit; the answered Container
// runs commands in the same container.
// PREVENTS: a UDP server judged ready by a host probe the published port
// answers before the server binds, or an Exec that reaches another container.
func TestLabtestReadyInContainer(t *testing.T) {
	spec := fixtureSpec()
	spec.Ready = nil
	spec.ReadyInContainer = []string{"sh", "-c", "grep -q bound /proc/net/udp"}

	runner := &scriptedRunner{}
	tester := &fakeTester{ctx: t.Context()}
	var container Container
	done := make(chan struct{})
	go func() {
		defer close(done)
		container = start(tester, fixtureHost(runner, true, nil), spec)
		if _, err := container.Exec(t.Context(), "cat", "/srv/collector.out"); err != nil {
			tester.Errorf("exec: %v", err)
		}
	}()
	<-done
	if tester.fatal != "" {
		t.Fatalf("start failed: %s", tester.fatal)
	}
	if want := []string{"info", "pull", "run", "port", "exec", "exec"}; !slices.Equal(runner.verbs(), want) {
		t.Fatalf("docker verbs = %v, want exactly %v", runner.verbs(), want)
	}
	for _, index := range []int{4, 5} {
		if exec := strings.Join(runner.commands[index], " "); !strings.Contains(exec, "ze-labtest-stayrtr-fixture") {
			t.Errorf("exec argv %q does not run in the started container", exec)
		}
	}
	if probe := strings.Join(runner.commands[4], " "); !strings.HasSuffix(probe, "sh -c grep -q bound /proc/net/udp") {
		t.Errorf("ready argv %q is not the spec's command", probe)
	}

	failing := &scriptedRunner{fail: map[string]bool{"exec": true}}
	refused := &fakeTester{ctx: t.Context()}
	spec.ReadyTimeout = 50 * time.Millisecond
	runStart(t, refused, fixtureHost(failing, true, nil), spec)
	if !strings.Contains(refused.fatal, "exec refused by the fixture") {
		t.Fatalf("failure %q does not name the in-container probe's error", refused.fatal)
	}
}

// pinnedImage is a digest-pinned reference: the local copy, when present, is
// exactly the image the test names.
const pinnedImage = "rpki/stayrtr:0.6@sha256:f584829f268b26b26b351000b60aa7c3507933873b6a6d77ffe9dcabcfdc7025"

// VALIDATES: a digest-pinned image already on the host starts with no pull, so
// a host that holds it runs offline; an absent pinned image is pulled; an
// absent pinned image that cannot be pulled keeps the Q-6 failure; a tag
// reference, which can move, is still pulled without an inspect.
// METHOD: the fake runner answers `docker image inspect` present or absent and
// the test reads the docker verbs the start ran.
// PREVENTS: a docker-tagged test failing offline on a `docker pull` of an image
// the host already holds byte for byte.
func TestLabtestPullsAPinnedImageOnlyWhenAbsent(t *testing.T) {
	cases := []struct {
		name      string
		image     string
		absent    bool
		pullFails bool
		want      []string
		wantFatal string
	}{
		{name: "pinned present", image: pinnedImage, want: []string{"info", "image", "run", "port", "rm"}},
		{name: "pinned absent", image: pinnedImage, absent: true,
			want: []string{"info", "image", "pull", "run", "port", "rm"}},
		{name: "pinned absent unpullable", image: pinnedImage, absent: true, pullFails: true,
			want: []string{"info", "image", "pull"}, wantFatal: "pull refused by the fixture"},
		{name: "tag", image: "rpki/stayrtr:latest", want: []string{"info", "pull", "run", "port", "rm"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := &scriptedRunner{absent: map[string]bool{"image": tc.absent}, fail: map[string]bool{"pull": tc.pullFails}}
			spec := fixtureSpec()
			spec.Image = tc.image
			tester := &fakeTester{ctx: t.Context()}
			runStart(t, tester, fixtureHost(runner, true, map[string]string{}), spec)
			if !slices.Equal(runner.verbs(), tc.want) {
				t.Errorf("docker verbs = %v, want %v", runner.verbs(), tc.want)
			}
			if tester.skipped != "" {
				t.Errorf("skipped %q, want no skip under the docker tag", tester.skipped)
			}
			if tc.wantFatal == "" && tester.fatal != "" {
				t.Errorf("fatal %q, want a start", tester.fatal)
			}
			if !strings.Contains(tester.fatal, tc.wantFatal) {
				t.Errorf("fatal %q, want it to name %q", tester.fatal, tc.wantFatal)
			}
		})
	}
}

// VALIDATES: an inspect that fails for a reason other than absence fails the
// test naming that reason, and never pulls or starts a container.
// PREVENTS: a daemon that does not answer being read as an absent image.
func TestLabtestFailsWhenThePinnedInspectFails(t *testing.T) {
	runner := &scriptedRunner{exits: map[string]int{"image": 1}}
	spec := fixtureSpec()
	spec.Image = pinnedImage
	tester := &fakeTester{ctx: t.Context()}
	runStart(t, tester, fixtureHost(runner, true, map[string]string{}), spec)
	if !strings.Contains(tester.fatal, "image exited by the fixture") {
		t.Errorf("fatal %q, want it to name the inspect failure", tester.fatal)
	}
	if want := []string{"info", "image"}; !slices.Equal(runner.verbs(), want) {
		t.Errorf("docker verbs = %v, want %v", runner.verbs(), want)
	}
}
