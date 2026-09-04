package interoplab

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type recordingRunner struct {
	commands []processCommand
	run      func(processCommand) (processResult, error)
}

func (r *recordingRunner) Run(_ context.Context, command processCommand) (processResult, error) {
	r.commands = append(r.commands, command)
	if r.run == nil {
		return processResult{}, nil
	}
	return r.run(command)
}

// VALIDATES: scenario selection is an exact name match and every selected directory has a Go checker.
// PREVENTS: a substring selector or an untranslated scenario silently reducing the executed population.
func TestDiscoverSelectsOneExactScenario(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"01-bgp", "02-ipsec"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	checkers := map[string]Checker{
		"01-bgp":   func(context.Context, *CheckContext) error { return nil },
		"02-ipsec": func(context.Context, *CheckContext) error { return nil },
	}

	scenarios, err := Discover(root, "02-ipsec", checkers)
	if err != nil {
		t.Fatalf("Discover returned an error: %v", err)
	}
	if len(scenarios) != 1 {
		t.Fatalf("Discover returned %d scenarios, want 1", len(scenarios))
	}
	if scenarios[0].Name != "02-ipsec" {
		t.Errorf("Discover selected %q, want 02-ipsec", scenarios[0].Name)
	}

	if _, err := Discover(root, "ipsec", checkers); err == nil {
		t.Fatal("Discover accepted a substring selector")
	}
	delete(checkers, "01-bgp")
	if _, err := Discover(root, "", checkers); err == nil {
		t.Fatal("Discover silently skipped a scenario without a Go checker")
	}
}

// VALIDATES: the core reads the same environment spellings and value rules as the four Python lab runners.
// PREVENTS: NO_BUILD, VERBOSE, the selector, or the session budget changing meaning during the port.
func TestReadEnvironmentPreservesLabKnobs(t *testing.T) {
	values := map[string]string{
		"NO_BUILD":          "1",
		"VERBOSE":           "1",
		"SESSION_TIMEOUT":   "47",
		"INTEROP_SCENARIO":  " 02-ipsec ",
		"ZE_INTEROP_SUFFIX": "job-7",
		"FRR_IMAGE":         "frr:test",
	}
	environment := ReadEnvironment(EnvironmentOptions{
		SelectorVariable: "INTEROP_SCENARIO",
		SuffixVariable:   "ZE_INTEROP_SUFFIX",
		DefaultImage:     "frr:default",
		DefaultSuffix:    "pid-1",
		Lookup: func(name string) (string, bool) {
			value, ok := values[name]
			return value, ok
		},
	})

	if !environment.NoBuild {
		t.Error("NO_BUILD=1 did not enable no-build mode")
	}
	if !environment.Verbose {
		t.Error("VERBOSE=1 did not enable verbose mode")
	}
	if environment.SessionTimeout != 47*time.Second {
		t.Errorf("SESSION_TIMEOUT became %s, want 47s", environment.SessionTimeout)
	}
	if environment.Selector != "02-ipsec" {
		t.Errorf("selector = %q, want trimmed exact name", environment.Selector)
	}
	if environment.Suffix != "job-7" {
		t.Errorf("suffix = %q, want job-7", environment.Suffix)
	}
	if environment.Image != "frr:test" {
		t.Errorf("image = %q, want frr:test", environment.Image)
	}

	values["SESSION_TIMEOUT"] = "not-a-number"
	environment = ReadEnvironment(EnvironmentOptions{Lookup: func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}})
	if environment.SessionTimeout != 90*time.Second {
		t.Errorf("invalid SESSION_TIMEOUT became %s, want the producer default 90s", environment.SessionTimeout)
	}
}

// VALIDATES: Docker receives the producer's option order, immutable image result, and overlap retry.
// PREVENTS: a concurrent image tag race or a network overlap stopping the full suite.
func TestDockerBuildNetworkAndContainerArguments(t *testing.T) {
	runner := &recordingRunner{}
	runner.run = func(command processCommand) (processResult, error) {
		joined := strings.Join(command.Arguments, " ")
		switch {
		case strings.HasPrefix(joined, "docker build "):
			return processResult{Stdout: "sha256:built\n"}, nil
		case strings.Contains(joined, "--subnet=172.30.0.0/24"):
			return processResult{ExitCode: 1, Stderr: "Pool overlaps with other one"}, nil
		default:
			return processResult{}, nil
		}
	}
	docker := newDocker(runner)

	image, err := docker.Build(t.Context(), ImageBuild{
		Name:       "ze",
		Tag:        "ze-interop",
		Dockerfile: "Dockerfile.ze",
		Context:    "/repo",
		BuildArgs:  []string{"ZE_FEATURES=ze_bgp ze_ssh"},
	})
	if err != nil {
		t.Fatalf("Build returned an error: %v", err)
	}
	if image.Reference != "sha256:built" {
		t.Errorf("image reference = %q, want immutable image id", image.Reference)
	}

	network, err := docker.createNetwork(t.Context(), NetworkSpec{
		Name: "lab-net",
		Candidates: []Subnet{
			{IPv4: netip.MustParsePrefix("172.30.0.0/24")},
			{IPv4: netip.MustParsePrefix("172.31.0.0/24"), IPv6: netip.MustParsePrefix("fd00:1::/64")},
		},
	})
	if err != nil {
		t.Fatalf("createNetwork returned an error: %v", err)
	}
	if network.IPv4 != netip.MustParsePrefix("172.31.0.0/24") {
		t.Errorf("selected subnet = %s, want second non-overlapping candidate", network.IPv4)
	}

	err = docker.runContainer(t.Context(), network, PeerConfig{
		Name:         "peer",
		Container:    "lab-peer",
		Image:        "sha256:peer",
		Host:         3,
		Capabilities: []string{"NET_ADMIN", "SYS_ADMIN"},
		Mounts:       []Mount{{Source: "/host/peer.conf", Target: "/etc/peer.conf", ReadOnly: true}},
		Environment:  []EnvironmentVariable{{Name: "SESSION_TIMEOUT", Value: "47"}},
		Arguments:    []string{"--privileged"},
		Command:      []string{"start", "/etc/peer.conf"},
	})
	if err != nil {
		t.Fatalf("runContainer returned an error: %v", err)
	}

	wantBuild := []string{"docker", "build", "-t", "ze-interop", "--build-arg", "ZE_FEATURES=ze_bgp ze_ssh", "-f", "Dockerfile.ze", "/repo", "-q"}
	if got := runner.commands[0].Arguments; !reflect.DeepEqual(got, wantBuild) {
		t.Errorf("build argv = %#v, want %#v", got, wantBuild)
	}
	if runner.commands[0].Timeout != dockerBuildTimeoutDefault {
		t.Errorf("build timeout = %s, want %s", runner.commands[0].Timeout, dockerBuildTimeoutDefault)
	}
	wantRun := []string{
		"docker", "run", "-d", "--name", "lab-peer", "--network", "lab-net", "--ip", "172.31.0.3",
		"--ip6", "fd00:1::3", "--cap-add", "NET_ADMIN", "--cap-add", "SYS_ADMIN",
		"-v", "/host/peer.conf:/etc/peer.conf:ro", "-e", "SESSION_TIMEOUT=47", "--privileged",
		"sha256:peer", "start", "/etc/peer.conf",
	}
	if got := runner.commands[len(runner.commands)-1].Arguments; !reflect.DeepEqual(got, wantRun) {
		t.Errorf("run argv = %#v, want %#v", got, wantRun)
	}
}

// VALIDATES: docker build -q must print the immutable image ID that this run consumes.
// PREVENTS: an empty successful build silently falling back to a shared mutable tag.
func TestDockerBuildRejectsEmptyImageID(t *testing.T) {
	docker := newDocker(&recordingRunner{})
	_, err := docker.Build(t.Context(), ImageBuild{
		Name:       "ze",
		Tag:        "ze-interop",
		Dockerfile: "Dockerfile.ze",
		Context:    "/repo",
	})
	if err == nil {
		t.Fatal("Build accepted empty stdout as an image reference")
	}
}

// VALIDATES: an image with a slower external build can set its own bounded timeout.
// PREVENTS: a declared per-image budget collapsing to the machine build budget.
func TestDockerBuildUsesDeclaredTimeout(t *testing.T) {
	runner := &recordingRunner{run: func(processCommand) (processResult, error) {
		return processResult{Stdout: "sha256:slow\n"}, nil
	}}
	docker := newDocker(runner)
	_, err := docker.Build(t.Context(), ImageBuild{
		Name:       "peer",
		Tag:        "peer-image",
		Dockerfile: "Dockerfile.peer",
		Context:    "/peer",
		Timeout:    15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Build returned an error: %v", err)
	}
	if runner.commands[0].Timeout != 15*time.Minute {
		t.Errorf("build timeout = %s, want 15m", runner.commands[0].Timeout)
	}
	if _, err := docker.Build(t.Context(), ImageBuild{Tag: "bad", Timeout: -time.Second}); err == nil {
		t.Fatal("Build accepted a negative timeout")
	}
}

// VALIDATES: an image that declares no timeout takes the machine budget, and
// BUILD_TIMEOUT sets that budget in whole seconds.
// PREVENTS: one wall-clock constant killing a build that the machine completes,
// which is what a 2-CPU host did to the 40-minute ze interop image.
func TestDockerBuildTakesTheMachineBudget(t *testing.T) {
	absent := func(string) (string, bool) { return "", false }
	if got := machineBuildTimeout(absent); got != dockerBuildTimeoutDefault {
		t.Errorf("budget with no variable = %s, want %s", got, dockerBuildTimeoutDefault)
	}
	answers := map[string]time.Duration{
		"1800": 30 * time.Minute,
		"7200": 2 * time.Hour,
		"":     dockerBuildTimeoutDefault,
		"soon": dockerBuildTimeoutDefault,
		"0":    dockerBuildTimeoutDefault,
		"-60":  dockerBuildTimeoutDefault,
	}
	for value, want := range answers {
		lookup := func(name string) (string, bool) {
			if name != "BUILD_TIMEOUT" {
				t.Errorf("read variable %q, want BUILD_TIMEOUT", name)
			}
			return value, true
		}
		if got := machineBuildTimeout(lookup); got != want {
			t.Errorf("budget for BUILD_TIMEOUT=%q = %s, want %s", value, got, want)
		}
	}

	runner := &recordingRunner{run: func(processCommand) (processResult, error) {
		return processResult{Stdout: "sha256:machine\n"}, nil
	}}
	docker := newDocker(runner)
	docker.buildTimeout = 45 * time.Minute
	if _, err := docker.Build(t.Context(), ImageBuild{Name: "ze", Tag: "ze-interop", Dockerfile: "Dockerfile.ze", Context: "/src"}); err != nil {
		t.Fatalf("Build returned an error: %v", err)
	}
	if runner.commands[0].Timeout != 45*time.Minute {
		t.Errorf("undeclared build timeout = %s, want 45m", runner.commands[0].Timeout)
	}
}

// VALIDATES: the constructor every suite calls reads BUILD_TIMEOUT itself.
// PREVENTS: the knob answering in a unit test and reaching no interop run,
// because NewDocker is the only Docker a suite builds an image with.
func TestNewDockerReadsTheMachineBudget(t *testing.T) {
	t.Setenv("BUILD_TIMEOUT", "1200")
	if got := NewDocker().buildTimeout; got != 20*time.Minute {
		t.Errorf("NewDocker build budget = %s, want 20m", got)
	}
}

// VALIDATES: first-run pre-clean accepts only Docker's exact missing-container answer.
// PREVENTS: a clean host failing before network creation or a different removal error passing.
func TestRemoveContainerAcceptsOnlyExactAbsence(t *testing.T) {
	answer := "Error response from daemon: No such container: lab-peer"
	runner := &recordingRunner{run: func(processCommand) (processResult, error) {
		return processResult{ExitCode: 1, Stderr: answer}, nil
	}}
	docker := newDocker(runner)
	if err := docker.RemoveContainer(t.Context(), "lab-peer"); err != nil {
		t.Fatalf("exact absent-container answer returned an error: %v", err)
	}

	answer = "Error response from daemon: No such container: lab-peer-other"
	if err := docker.RemoveContainer(t.Context(), "lab-peer"); err == nil {
		t.Fatal("absence for a different container was accepted")
	}
	answer = "removal already in progress"
	if err := docker.RemoveContainer(t.Context(), "lab-peer"); err == nil {
		t.Fatal("a real container removal failure was accepted")
	}
	want := []string{"docker", "rm", "-f", "lab-peer"}
	for _, command := range runner.commands {
		if !reflect.DeepEqual(command.Arguments, want) {
			t.Errorf("remove argv = %#v, want %#v", command.Arguments, want)
		}
	}
}

// VALIDATES: container PID lookup returns only a measured positive integer.
// PREVENTS: raw-frame injection targeting host PID zero after an empty or malformed inspect answer.
func TestContainerPIDRejectsInvalidOutput(t *testing.T) {
	tests := []struct {
		name   string
		stdout string
	}{
		{name: "empty"},
		{name: "zero", stdout: "0\n"},
		{name: "negative", stdout: "-7\n"},
		{name: "not integer", stdout: "unknown\n"},
	}
	for _, one := range tests {
		t.Run(one.name, func(t *testing.T) {
			docker := newDocker(&recordingRunner{run: func(processCommand) (processResult, error) {
				return processResult{Stdout: one.stdout}, nil
			}})
			if _, err := docker.containerPID(t.Context(), "lab-peer"); err == nil {
				t.Fatalf("containerPID accepted %q", one.stdout)
			}
		})
	}
}

// VALIDATES: a successful but empty query and a failed command are different from an empty protocol result.
// PREVENTS: Docker or peer failures silently becoming valid-looking empty output.
func TestLabQueryNeverAnswersSilentlyEmpty(t *testing.T) {
	runner := &recordingRunner{}
	runner.run = func(command processCommand) (processResult, error) {
		if command.Arguments[len(command.Arguments)-1] == "failed" {
			return processResult{ExitCode: 9, Stderr: "peer refused query"}, nil
		}
		return processResult{}, nil
	}
	lab := newLab(newDocker(runner), []PeerConfig{{Name: "peer", Container: "lab-peer"}})

	if _, err := lab.Query(t.Context(), "peer", []string{"show", "empty"}, nil); err == nil {
		t.Fatal("Query returned an empty string without an error")
	}
	if _, err := lab.Exec(t.Context(), "peer", []string{"show", "failed"}, nil); err == nil {
		t.Fatal("Exec hid a non-zero peer command")
	}
}

// VALIDATES: peer exec, logs, signal, and restart preserve argv, output streams, and exit failures.
// PREVENTS: environment loss, merged output, or an unreadable empty log appearing available.
func TestPeerCommandAndLogAPIsPreserveTheirContracts(t *testing.T) {
	runner := &recordingRunner{}
	logCalls := 0
	runner.run = func(command processCommand) (processResult, error) {
		joined := strings.Join(command.Arguments, " ")
		if strings.HasPrefix(joined, "docker run --rm ") {
			return processResult{Stdout: "kernel-ready\n"}, nil
		}
		if strings.HasPrefix(joined, "docker exec ") {
			return processResult{Stdout: "answer\n", Stderr: "notice\n"}, nil
		}
		if strings.HasPrefix(joined, "docker inspect ") {
			return processResult{Stdout: "4321\n"}, nil
		}
		if strings.HasPrefix(joined, "docker logs ") {
			logCalls++
			if logCalls == 2 {
				return processResult{ExitCode: 2, Stderr: "log unavailable"}, nil
			}
		}
		return processResult{}, nil
	}
	docker := newDocker(runner)
	lab := newLab(docker, []PeerConfig{{Name: "peer", Container: "lab-peer"}})

	preflight, err := docker.RunOneShot(t.Context(), OneShotContainer{
		Image:     "alpine:3",
		Arguments: []string{"--privileged"},
		Command:   []string{"modprobe", "pppoe"},
		Timeout:   30 * time.Second,
	})
	if err != nil {
		t.Fatalf("RunOneShot returned an error: %v", err)
	}
	if preflight.Stdout != "kernel-ready\n" {
		t.Errorf("RunOneShot stdout = %q, want kernel-ready", preflight.Stdout)
	}
	result, err := lab.Exec(t.Context(), "peer", []string{"show", "state"}, []EnvironmentVariable{{Name: "TOKEN", Value: "secret"}})
	if err != nil {
		t.Fatalf("Exec returned an error: %v", err)
	}
	if result.Stdout != "answer\n" {
		t.Errorf("Exec stdout = %q, want answer", result.Stdout)
	}
	if result.Stderr != "notice\n" {
		t.Errorf("Exec stderr = %q, want notice", result.Stderr)
	}
	detachedCommand := []string{"sh", "-c", "exec pppd call peer >/var/log/ppp/dial.log 2>&1"}
	if err := lab.ExecDetached(t.Context(), "peer", detachedCommand, nil); err != nil {
		t.Fatalf("ExecDetached returned an error: %v", err)
	}
	logs, err := lab.Logs(t.Context(), "peer", 20)
	if err != nil {
		t.Fatalf("Logs returned an error for a readable empty log: %v", err)
	}
	if !logs.Available {
		t.Error("readable empty log was marked unavailable")
	}
	if logs.Text != "" {
		t.Errorf("empty log text = %q", logs.Text)
	}
	if _, err := lab.Logs(t.Context(), "peer", 21); err == nil {
		t.Fatal("failed Docker logs command returned an available empty log")
	}
	pid, err := lab.PeerPID(t.Context(), "peer")
	if err != nil {
		t.Fatalf("PeerPID returned an error: %v", err)
	}
	if pid != 4321 {
		t.Errorf("PeerPID = %d, want 4321", pid)
	}
	if err := lab.Signal(t.Context(), "peer", ""); err != nil {
		t.Fatalf("Signal returned an error: %v", err)
	}
	if err := lab.Stop(t.Context(), "peer", 5); err != nil {
		t.Fatalf("Stop returned an error: %v", err)
	}
	if err := lab.Pause(t.Context(), "peer"); err != nil {
		t.Fatalf("Pause returned an error: %v", err)
	}
	if err := lab.Unpause(t.Context(), "peer"); err != nil {
		t.Fatalf("Unpause returned an error: %v", err)
	}
	if err := lab.Start(t.Context(), "peer"); err != nil {
		t.Fatalf("Start returned an error: %v", err)
	}

	wantDetached := []string{
		"docker", "exec", "-d", "lab-peer",
		"sh", "-c", "exec pppd call peer >/var/log/ppp/dial.log 2>&1",
	}
	if got := runner.commands[2].Arguments; !reflect.DeepEqual(got, wantDetached) {
		t.Errorf("detached exec argv = %#v, want %#v", got, wantDetached)
	}
	if runner.commands[7].Timeout != 35*time.Second {
		t.Errorf("docker stop command timeout = %s, want 35s", runner.commands[7].Timeout)
	}
	joined := recordedCommands(runner.commands)
	assertOrder(t, joined,
		"docker run --rm --privileged alpine:3 modprobe pppoe",
		"docker exec -e TOKEN=secret lab-peer show state",
		"docker exec -d lab-peer sh -c exec pppd call peer >/var/log/ppp/dial.log 2>&1",
		"docker logs lab-peer --tail 20",
		"docker logs lab-peer --tail 21",
		"docker inspect --format {{.State.Pid}} lab-peer",
		"docker kill --signal TERM lab-peer",
		"docker stop -t 5 lab-peer",
		"docker pause lab-peer",
		"docker unpause lab-peer",
		"docker start lab-peer",
	)
}

// VALIDATES: a leaf can render protocol configuration after the core selects a subnet.
// PREVENTS: protocol names or fixed topology branches entering the shared engine.
func TestScenarioPreparerReceivesSelectedNetwork(t *testing.T) {
	runner := &recordingRunner{}
	runner.run = func(command processCommand) (processResult, error) {
		joined := strings.Join(command.Arguments, " ")
		if strings.Contains(joined, "docker network rm dynamic-net") {
			if len(runner.commands) <= 4 {
				return processResult{ExitCode: 1, Stderr: "network dynamic-net not found"}, nil
			}
		}
		return processResult{}, nil
	}
	preparedNetwork := netip.Prefix{}
	hostCleanups := 0
	checkerCalls := 0
	preflightCalls := 0
	source := ScenarioSource{
		Name: "dynamic",
		Checker: func(_ context.Context, check *CheckContext) error {
			checkerCalls++
			if check.Network.IPv4 != netip.MustParsePrefix("172.31.8.0/24") {
				return errors.New("checker received the wrong selected network")
			}
			if check.Source.Name != "dynamic" {
				return errors.New("checker received the wrong scenario source")
			}
			return nil
		},
	}
	suite := Suite{
		Docker: newDocker(runner),
		Preflight: func(ctx context.Context, docker *Docker) error {
			preflightCalls++
			_, err := docker.RunOneShot(ctx, OneShotContainer{
				Image:   "alpine:3",
				Command: []string{"host-probe"},
				Timeout: 30 * time.Second,
			})
			return err
		},
		NoBuild: true,
		Scenarios: []ScenarioPlan{{
			Source:     source,
			Network:    NetworkSpec{Name: "dynamic-net", Candidates: []Subnet{{IPv4: netip.MustParsePrefix("172.31.8.0/24")}}},
			Containers: []string{"dynamic-peer"},
			Prepare: func(_ context.Context, prepare PrepareContext) (PreparedScenario, error) {
				preparedNetwork = prepare.Network.IPv4
				return PreparedScenario{
					Peers: []PeerConfig{{Name: "peer", Container: "dynamic-peer", Image: "peer-image", Host: 3}},
					Cleanup: func() error {
						hostCleanups++
						return nil
					},
				}, nil
			},
		}},
	}

	report := suite.Run(t.Context())
	if report.Code != 0 {
		t.Fatalf("Suite code = %d, want 0: %#v", report.Code, report)
	}
	if preflightCalls != 1 {
		t.Errorf("preflight ran %d times, want 1", preflightCalls)
	}
	if preparedNetwork != netip.MustParsePrefix("172.31.8.0/24") {
		t.Errorf("preparer network = %s, want selected subnet", preparedNetwork)
	}
	if checkerCalls != 1 {
		t.Errorf("checker ran %d times, want 1", checkerCalls)
	}
	if hostCleanups != 1 {
		t.Errorf("prepared host cleanup ran %d times, want 1", hostCleanups)
	}
}

// VALIDATES: setup runs in dependency order, the typed checker runs once, and cleanup always follows it.
// PREVENTS: a checker running before peers are ready or a successful scenario leaking Docker resources.
func TestSuiteRunsLifecycleAndCheckerInOrder(t *testing.T) {
	runner := &recordingRunner{}
	checkerCalls := 0
	runner.run = func(command processCommand) (processResult, error) {
		joined := strings.Join(command.Arguments, " ")
		if strings.Contains(joined, "docker network rm lab-net") {
			if len(runner.commands) <= 4 {
				return processResult{ExitCode: 1, Stderr: "network lab-net not found"}, nil
			}
		}
		if strings.Contains(joined, "docker exec lab-peer show state") {
			return processResult{Stdout: "ready\n"}, nil
		}
		return processResult{}, nil
	}
	checker := func(ctx context.Context, check *CheckContext) error {
		checkerCalls++
		output, err := check.Lab.Query(ctx, "peer", []string{"show", "state"}, nil)
		if err != nil {
			return err
		}
		if output != "ready\n" {
			return errors.New("peer returned the wrong state")
		}
		return nil
	}
	source := ScenarioSource{Name: "one", Directory: "/scenarios/one", Checker: checker}
	suite := Suite{
		Docker:  newDocker(runner),
		NoBuild: true,
		Scenarios: []ScenarioPlan{{
			Source:  source,
			Network: NetworkSpec{Name: "lab-net", Candidates: []Subnet{{IPv4: netip.MustParsePrefix("172.29.0.0/24")}}},
			Peers: []PeerConfig{
				{Name: "ze", Container: "lab-ze", Image: "ze-image", Host: 2},
				{
					Name:      "peer",
					Container: "lab-peer",
					Image:     "peer-image",
					Host:      3,
					Ready: &ReadyProbe{
						Command:  []string{"peer-ready"},
						Timeout:  50 * time.Millisecond,
						Interval: time.Millisecond,
					},
				},
			},
		}},
	}

	report := suite.Run(t.Context())
	if report.Code != 0 {
		t.Fatalf("Suite code = %d, want 0: %#v", report.Code, report)
	}
	if checkerCalls != 1 {
		t.Errorf("checker ran %d times, want 1", checkerCalls)
	}
	joined := recordedCommands(runner.commands)
	assertOrder(t, joined,
		"docker info",
		"docker rm -f lab-ze",
		"docker rm -f lab-peer",
		"docker network rm lab-net",
		"docker network create --subnet=172.29.0.0/24 lab-net",
		"docker run -d --name lab-ze",
		"docker run -d --name lab-peer",
		"docker exec lab-peer peer-ready",
		"docker exec lab-peer show state",
		"docker rm -f lab-ze",
		"docker rm -f lab-peer",
		"docker network rm lab-net",
	)
}

// VALIDATES: a container-start failure marks the scenario failed and still removes every declared resource.
// PREVENTS: setup failures bypassing the finally-style cleanup contract.
func TestSuiteCleansUpAfterSetupFailure(t *testing.T) {
	runner := &recordingRunner{}
	runner.run = func(command processCommand) (processResult, error) {
		joined := strings.Join(command.Arguments, " ")
		if strings.Contains(joined, "docker network rm fail-net") {
			if len(runner.commands) <= 3 {
				return processResult{ExitCode: 1, Stderr: "network fail-net not found"}, nil
			}
		}
		if strings.Contains(joined, "docker rm -f fail-peer") {
			if len(runner.commands) > 5 {
				return processResult{ExitCode: 23, Stderr: "cleanup busy"}, nil
			}
		}
		if strings.Contains(joined, "docker run -d --name fail-peer") {
			return processResult{ExitCode: 17, Stderr: "cannot start peer"}, nil
		}
		return processResult{}, nil
	}
	suite := Suite{
		Docker:  newDocker(runner),
		NoBuild: true,
		Scenarios: []ScenarioPlan{{
			Source:  ScenarioSource{Name: "failure", Checker: func(context.Context, *CheckContext) error { return nil }},
			Network: NetworkSpec{Name: "fail-net", Candidates: []Subnet{{IPv4: netip.MustParsePrefix("172.28.0.0/24")}}},
			Peers:   []PeerConfig{{Name: "peer", Container: "fail-peer", Image: "peer", Host: 3}},
		}},
	}

	report := suite.Run(t.Context())
	if report.Code != 1 {
		t.Fatalf("Suite code = %d, want 1", report.Code)
	}
	if len(report.Scenarios) != 1 {
		t.Fatalf("Suite returned %d scenario results, want 1", len(report.Scenarios))
	}
	if report.Scenarios[0].Passed {
		t.Fatal("failed container start produced a passing scenario")
	}
	if len(report.Scenarios[0].CleanupErrors) != 1 {
		t.Fatalf("cleanup errors = %#v, want one recorded removal failure", report.Scenarios[0].CleanupErrors)
	}
	joined := recordedCommands(runner.commands)
	lastRemove := strings.LastIndex(joined, "docker rm -f fail-peer")
	failedRun := strings.Index(joined, "docker run -d --name fail-peer")
	if lastRemove < failedRun {
		t.Fatalf("cleanup did not follow the failed run:\n%s", joined)
	}
	if !strings.Contains(joined[failedRun:], "docker network rm fail-net") {
		t.Fatalf("network cleanup did not follow the failed run:\n%s", joined)
	}
}

// VALIDATES: waits stop at their bound and distinguish no measurement from not-ready state.
// PREVENTS: an external peer failure becoming an unbounded loop or a valid zero value.
func TestWaitIsBoundedAndFailsClosed(t *testing.T) {
	attempts := 0
	_, report, err := Wait(t.Context(), WaitOptions{
		Timeout:     5 * time.Millisecond,
		Interval:    time.Millisecond,
		Description: "peer state",
	}, func(context.Context) (string, error) {
		attempts++
		return "", errors.New("peer command failed")
	}, func(value string) bool { return value == "ready" })
	if err == nil {
		t.Fatal("Wait returned success without one measured value")
	}
	if report.Attempts != attempts {
		t.Errorf("report attempts = %d, probe calls = %d", report.Attempts, attempts)
	}
	if attempts < 1 {
		t.Fatal("Wait did not call the probe")
	}
	if report.TransientFailures != attempts {
		t.Errorf("transient failures = %d, want %d", report.TransientFailures, attempts)
	}

	value, report, err := Wait(t.Context(), WaitOptions{
		Timeout:     50 * time.Millisecond,
		Interval:    time.Millisecond,
		Description: "peer ready",
	}, func(context.Context) (string, error) {
		return "ready", nil
	}, func(value string) bool { return value == "ready" })
	if err != nil {
		t.Fatalf("ready Wait returned an error: %v", err)
	}
	if value != "ready" {
		t.Errorf("ready Wait returned %q", value)
	}
	if report.Attempts != 1 {
		t.Errorf("ready Wait took %d attempts, want 1", report.Attempts)
	}
}

func recordedCommands(commands []processCommand) string {
	lines := make([]string, 0, len(commands))
	for _, command := range commands {
		lines = append(lines, strings.Join(command.Arguments, " "))
	}
	return strings.Join(lines, "\n")
}

func assertOrder(t *testing.T, text string, needles ...string) {
	t.Helper()
	offset := 0
	for _, needle := range needles {
		index := strings.Index(text[offset:], needle)
		if index < 0 {
			t.Fatalf("%q did not occur after offset %d:\n%s", needle, offset, text)
		}
		offset += index + len(needle)
	}
}
