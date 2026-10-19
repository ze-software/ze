package interoplab

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"strings"
	"testing"
	"time"
)

// VALIDATES: AC-11, AC-12: a detached container renders every typed need in
// a fixed order after the owner labels, so the reaper can find it.
// PREVENTS: a deployment or perf container started without the labels that let
// a later run remove it after a SIGKILL.
func TestRunDetachedRendersALabeledContainer(t *testing.T) {
	runner := &recordingRunner{}
	docker := newDocker(runner)
	err := docker.RunDetached(t.Context(), DetachedContainer{
		Name: "ze-vpp-1", Image: "vpp:1", Platform: "linux/amd64",
		Network: "le-perf-1", Address: netip.MustParseAddr("172.31.0.10"),
		Capabilities: []string{"NET_ADMIN"}, Devices: []string{"/dev/net/tun"},
		Mounts:           []Mount{{Source: "/tree", Target: "/src", ReadOnly: true}},
		Environment:      []EnvironmentVariable{{Name: "A", Value: "b"}},
		WorkingDirectory: "/src", Entrypoint: "sleep", Command: []string{"infinity"},
	})
	if err != nil {
		t.Fatalf("RunDetached: %v", err)
	}
	if len(runner.commands) != 1 {
		t.Fatalf("recorded %d commands, want one run", len(runner.commands))
	}
	argv := runner.commands[0].Arguments
	labels := docker.owner.labelArguments()
	want := []string{DockerExecutable, "run", "-d", "--name", "ze-vpp-1"}
	want = append(want, labels...)
	want = append(want, "--network", "le-perf-1", "--ip", "172.31.0.10", "--platform", "linux/amd64",
		"--cap-add", "NET_ADMIN", "--device", "/dev/net/tun", "-v", "/tree:/src:ro", "-e", "A=b",
		"-w", "/src", "--entrypoint", "sleep", "vpp:1", "infinity")
	if !slices.Equal(argv, want) {
		t.Errorf("argv =\n%v\nwant\n%v", argv, want)
	}
	if !slices.Contains(argv, labelRun) && !strings.Contains(strings.Join(argv, " "), labelRun) {
		t.Errorf("argv %v carries no %s label", argv, labelRun)
	}
}

// VALIDATES: a detached spec that cannot be rendered is refused before docker runs.
// PREVENTS: an --ip with no --network reaching the daemon as an opaque failure.
func TestRunDetachedRefusesAnIncompleteSpec(t *testing.T) {
	cases := map[string]DetachedContainer{
		"no name":            {Image: "alpine"},
		"no image":           {Name: "x"},
		"address no network": {Name: "x", Image: "alpine", Address: netip.MustParseAddr("10.0.0.1")},
		"empty mount":        {Name: "x", Image: "alpine", Mounts: []Mount{{Target: "/src"}}},
	}
	for name, spec := range cases {
		t.Run(name, func(t *testing.T) {
			runner := &recordingRunner{}
			if err := newDocker(runner).RunDetached(t.Context(), spec); err == nil {
				t.Fatal("RunDetached accepted the spec")
			}
			if len(runner.commands) != 0 {
				t.Errorf("docker ran %v for a refused spec", runner.commands)
			}
		})
	}
}

// VALIDATES: an attached exec carries its bound, its streams and --interactive
// only when asked, and a non-zero exit or a canceled client is an error.
// PREVENTS: a daemon exec reading a closed stdin, or a failed install read as done.
func TestExecAttachedCarriesStreamsAndBound(t *testing.T) {
	runner := &recordingRunner{}
	docker := newDocker(runner)
	stdin := strings.NewReader("config")
	var out strings.Builder
	_, err := docker.ExecAttached(t.Context(), ContainerExec{
		Container: "c", Command: []string{"ze", "-"}, Interactive: true, Stdin: stdin,
		Environment: []EnvironmentVariable{{Name: "ZE_LOG", Value: "debug"}},
		Stdout:      &out, Stderr: &out, Timeout: time.Minute,
	})
	if err != nil {
		t.Fatalf("ExecAttached: %v", err)
	}
	command := runner.commands[0]
	want := []string{DockerExecutable, "exec", "--interactive", "-e", "ZE_LOG=debug", "c", "ze", "-"}
	if !slices.Equal(command.Arguments, want) {
		t.Errorf("argv = %v, want %v", command.Arguments, want)
	}
	if command.Timeout != time.Minute || command.Stdin != stdin || command.Stdout != &out {
		t.Errorf("exec lost its bound or streams: %+v", command)
	}

	runner.run = func(ProcessCommand) (ProcessResult, error) { return ProcessResult{ExitCode: 3}, nil }
	if _, err := docker.ExecAttached(t.Context(), ContainerExec{Container: "c", Command: []string{"false"}, Timeout: time.Second}); err == nil {
		t.Error("a non-zero exit returned no error")
	}
	if got := runner.commands[1].Arguments; slices.Contains(got, "--interactive") {
		t.Errorf("a non-interactive exec carried --interactive: %v", got)
	}
	runner.run = func(ProcessCommand) (ProcessResult, error) { return ProcessResult{}, context.Canceled }
	if _, err := docker.ExecAttached(t.Context(), ContainerExec{Container: "c", Command: []string{"sleep"}, Timeout: time.Second}); !errors.Is(err, context.Canceled) {
		t.Errorf("a canceled exec answered %v, want context.Canceled", err)
	}
	if _, err := docker.ExecAttached(t.Context(), ContainerExec{Container: "c", Command: []string{"x"}}); err == nil {
		t.Error("an unbounded exec was accepted")
	}
}

// VALIDATES: ImagePresent tells present, absent and "could not ask" apart.
// PREVENTS: a daemon that does not answer read as a missing image.
func TestImagePresentTellsAbsentFromUnknown(t *testing.T) {
	runner := &recordingRunner{}
	docker := newDocker(runner)
	if present, err := docker.ImagePresent(t.Context(), "alpine:3"); err != nil || !present {
		t.Errorf("present image answered %v, %v", present, err)
	}
	runner.run = func(ProcessCommand) (ProcessResult, error) {
		return ProcessResult{ExitCode: 1, Stderr: "Error response from daemon: No such image: alpine:3"}, nil
	}
	if present, err := docker.ImagePresent(t.Context(), "alpine:3"); err != nil || present {
		t.Errorf("absent image answered %v, %v", present, err)
	}
	runner.run = func(ProcessCommand) (ProcessResult, error) {
		return ProcessResult{ExitCode: 1, Stderr: "Cannot connect to the Docker daemon"}, nil
	}
	if _, err := docker.ImagePresent(t.Context(), "alpine:3"); err == nil {
		t.Error("an unreachable daemon answered absent with no error")
	}
}

// VALIDATES: ContainerRunning reads the inspect state and tells a stopped
// container from one the daemon could not answer for.
// PREVENTS: a failed inspect read as "not running yet", which a wait loop
// would poll until its deadline and then blame the container.
func TestContainerRunningTellsStoppedFromUnknown(t *testing.T) {
	runner := &recordingRunner{}
	docker := newDocker(runner)
	runner.run = func(ProcessCommand) (ProcessResult, error) { return ProcessResult{Stdout: "true\n"}, nil }
	if running, err := docker.ContainerRunning(t.Context(), "dut"); err != nil || !running {
		t.Errorf("running container answered %v, %v", running, err)
	}
	want := []string{DockerExecutable, "container", "inspect", "--format", "{{.State.Running}}", "dut"}
	if got := runner.commands[0].Arguments; !slices.Equal(got, want) {
		t.Errorf("argv = %v, want %v", got, want)
	}
	runner.run = func(ProcessCommand) (ProcessResult, error) { return ProcessResult{Stdout: "false\n"}, nil }
	if running, err := docker.ContainerRunning(t.Context(), "dut"); err != nil || running {
		t.Errorf("exited container answered %v, %v", running, err)
	}
	runner.run = func(ProcessCommand) (ProcessResult, error) {
		return ProcessResult{ExitCode: 1, Stderr: "Error: No such object: dut"}, nil
	}
	if _, err := docker.ContainerRunning(t.Context(), "dut"); err == nil {
		t.Error("a failed inspect answered not running with no error")
	}
	runner.run = func(ProcessCommand) (ProcessResult, error) { return ProcessResult{Stdout: "\n"}, nil }
	if _, err := docker.ContainerRunning(t.Context(), "dut"); err == nil {
		t.Error("an empty inspect answer was read as a state")
	}
}

// VALIDATES: a named build context renders as one --build-context before the
// main context, and an incomplete one is refused before docker runs.
// PREVENTS: a raw argv escape for the freertr build, or a context path lost.
func TestBuildRendersNamedContexts(t *testing.T) {
	runner := &recordingRunner{}
	runner.run = func(ProcessCommand) (ProcessResult, error) { return ProcessResult{Stdout: "sha256:abc\n"}, nil }
	docker := newDocker(runner)
	build := ImageBuild{Name: "freertr", Tag: "freertr-interop", Dockerfile: "Dockerfile.freertr", Context: "/repo",
		NamedContexts: []NamedBuildContext{{Name: "freertr", Path: "/src/rtr"}}}
	if _, err := docker.Build(t.Context(), build); err != nil {
		t.Fatalf("Build: %v", err)
	}
	argv := runner.commands[0].Arguments
	at := slices.Index(argv, "--build-context")
	if at < 0 || argv[at+1] != "freertr=/src/rtr" || at > slices.Index(argv, "/repo") {
		t.Errorf("argv %v lacks --build-context freertr=/src/rtr before the main context", argv)
	}
	for _, named := range []NamedBuildContext{{Path: "/src"}, {Name: "x"}} {
		build.NamedContexts = []NamedBuildContext{named}
		if _, err := docker.Build(t.Context(), build); err == nil {
			t.Errorf("Build accepted the incomplete context %+v", named)
		}
	}
	if len(runner.commands) != 1 {
		t.Errorf("an incomplete context still ran docker: %d commands", len(runner.commands))
	}
}

// VALIDATES: AC-18: a published port is bound on the host loopback at a port
// the daemon picks, after the network flags, with its transport named, and
// port 0 or a port with no transport is refused.
// PREVENTS: a test server published on every host interface, a UDP server
// published as TCP, or `-p` with no container port reaching the daemon as an
// opaque failure.
func TestRunDetachedPublishesOnLoopback(t *testing.T) {
	runner := &recordingRunner{}
	docker := newDocker(runner)
	err := docker.RunDetached(t.Context(), DetachedContainer{
		Name: "ze-labtest-1", Image: "stayrtr:1", PublishedPorts: []ContainerPort{TCPPort(3323), UDPPort(123)},
		Command: []string{"-bind", ":3323"},
	})
	if err != nil {
		t.Fatalf("RunDetached: %v", err)
	}
	want := []string{DockerExecutable, "run", "-d", "--name", "ze-labtest-1"}
	want = append(want, docker.owner.labelArguments()...)
	want = append(want, "-p", "127.0.0.1::3323/tcp", "-p", "127.0.0.1::123/udp", "stayrtr:1", "-bind", ":3323")
	if argv := runner.commands[0].Arguments; !slices.Equal(argv, want) {
		t.Errorf("argv =\n%v\nwant\n%v", argv, want)
	}

	for _, port := range []ContainerPort{TCPPort(0), {Number: 8080}} {
		refused := &recordingRunner{}
		if err := newDocker(refused).RunDetached(t.Context(), DetachedContainer{
			Name: "x", Image: "alpine", PublishedPorts: []ContainerPort{port},
		}); err == nil {
			t.Fatalf("RunDetached accepted container port %+v", port)
		}
		if len(refused.commands) != 0 {
			t.Errorf("docker ran %v for a refused spec", refused.commands)
		}
	}
}

// VALIDATES: AC-18: PublishedPort asks `docker port` for the port under its
// own transport and answers the first bound address; an answer that is no address is an error
// naming what the daemon printed.
// PREVENTS: a dual-stack answer read as one garbled address, or a parse
// failure turned into port 0.
func TestPublishedPortReadsTheFirstBinding(t *testing.T) {
	runner := &recordingRunner{run: func(ProcessCommand) (ProcessResult, error) {
		return ProcessResult{Stdout: "127.0.0.1:49153\n[::1]:49153\n"}, nil
	}}
	address, err := newDocker(runner).PublishedPort(t.Context(), "ze-labtest-1", TCPPort(3323))
	if err != nil {
		t.Fatalf("PublishedPort: %v", err)
	}
	if want := netip.MustParseAddrPort("127.0.0.1:49153"); address != want {
		t.Errorf("address = %v, want %v", address, want)
	}
	want := []string{DockerExecutable, "port", "ze-labtest-1", "3323/tcp"}
	if argv := runner.commands[0].Arguments; !slices.Equal(argv, want) {
		t.Errorf("argv = %v, want %v", argv, want)
	}

	garbled := &recordingRunner{run: func(ProcessCommand) (ProcessResult, error) {
		return ProcessResult{Stdout: "no binding\n"}, nil
	}}
	_, err = newDocker(garbled).PublishedPort(t.Context(), "ze-labtest-1", TCPPort(3323))
	if err == nil {
		t.Fatal("PublishedPort accepted an answer that is no address")
	}
	if !strings.Contains(err.Error(), "no binding") {
		t.Errorf("error %q does not name what docker printed", err)
	}

	udp := &recordingRunner{run: func(ProcessCommand) (ProcessResult, error) {
		return ProcessResult{Stdout: "127.0.0.1:49154\n"}, nil
	}}
	if _, err := newDocker(udp).PublishedPort(t.Context(), "ze-labtest-1", UDPPort(123)); err != nil {
		t.Fatalf("PublishedPort UDP: %v", err)
	}
	want = []string{DockerExecutable, "port", "ze-labtest-1", "123/udp"}
	if argv := udp.commands[0].Arguments; !slices.Equal(argv, want) {
		t.Errorf("argv = %v, want %v", argv, want)
	}

	unchosen := &recordingRunner{}
	if _, err := newDocker(unchosen).PublishedPort(t.Context(), "ze-labtest-1", ContainerPort{Number: 123}); err == nil {
		t.Fatal("PublishedPort accepted a port with no transport")
	}
	if len(unchosen.commands) != 0 {
		t.Errorf("docker ran %v for a port with no transport", unchosen.commands)
	}
}
