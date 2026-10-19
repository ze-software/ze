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
		address = start(tester, h, spec)
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
}

func (r *scriptedRunner) Run(_ context.Context, command interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
	r.commands = append(r.commands, command.Arguments)
	verb := command.Arguments[1]
	if r.fail[verb] {
		return interoplab.ProcessResult{Stderr: verb + " refused by the fixture", ExitCode: 1}, nil
	}
	switch verb {
	case "port":
		return interoplab.ProcessResult{Stdout: "127.0.0.1:49153\n[::1]:49153\n"}, nil
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
		suffix: func() string { return "fixture" },
	}
}

func fixtureSpec() Spec {
	return Spec{
		Name: "stayrtr", Image: "rpki/stayrtr:latest", Command: []string{"-bind", ":3323"},
		Port: 3323, Ready: func(context.Context, netip.AddrPort) error { return nil },
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
	for _, want := range []string{"--name ze-labtest-stayrtr-fixture", "-p 127.0.0.1::3323", "ze.lab"} {
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
		"port 0":         func(s *Spec) { s.Port = 0 },
		"no ready":       func(s *Spec) { s.Ready = nil },
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
