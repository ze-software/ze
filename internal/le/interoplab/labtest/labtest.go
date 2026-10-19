// Design: docs/architecture/testing/interop.md -- a package test's own server container (labtest)
// Related: ../container.go -- RunDetached and PublishedPort, the wrapper calls this drives.
// Related: pull_docker.go -- what an unpullable image does under the docker tag.
// Related: pull_live.go -- what an unpullable image does under the live tag.

// Package labtest starts one real server container for a package test through
// the interoplab wrapper: labeled, so a sweep removes it when the test binary
// is killed; waited on until its ready probe answers; its log tail written to
// the test log when the test fails; removed by t.Cleanup. Run is the client
// half: one labeled client container run to its exit against a server the
// test runs in-process, its output answered to the test.
//
// It runs no Docker host kernel check (interoplab.DockerKernel): Ze runs in
// the test process on the host, and that check guards a containerised Ze. A
// peer that needs a Ze-kernel feature belongs in an interoplab.Suite lab.
//
// Only _test.go files import this package: `./le arch tier check` refuses a
// shipped file that does.
package labtest

import (
	"context"
	"crypto/rand"
	"errors"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	// requireDockerVariable turns an absent docker binary, and under the live
	// tag an unpullable image, from a skip into a failure. `./le test
	// integration docker-peers` and `live-rpki` set it, so a gating run
	// cannot pass because Docker or the registry was absent.
	requireDockerVariable = "ZE_REQUIRE_DOCKER"
	// containerPrefix starts every labtest container's name.
	containerPrefix = "ze-labtest-"
	// hostNetwork is Docker's name for the host's network namespace.
	hostNetwork = "host"
	// logLinesMax bounds the log tail a failed test receives.
	logLinesMax = 1000
	// readyInterval is the pause between two ready probes.
	readyInterval = 500 * time.Millisecond
	// releaseTimeout bounds the log read and the removal in t.Cleanup, which
	// run after the test's own context is canceled.
	releaseTimeout = 60 * time.Second
	// tcpDialTimeout bounds one TCPReady attempt; Start polls, so this bounds
	// an attempt and not the wait.
	tcpDialTimeout = 2 * time.Second
	// suffixLength is how many random characters keep two runs' names apart.
	suffixLength = 10
)

// Spec declares one server container. Name, Image, Port, ReadyTimeout,
// LogLines and exactly one of Ready and ReadyInContainer are required;
// validate says which is missing.
type Spec struct {
	// Name identifies the server in its container name and in messages:
	// lowercase letters, digits and hyphens. A random suffix keeps two runs
	// of the same test apart.
	Name  string
	Image string
	// Dockerfile, when set, is a Dockerfile path from the module root, such
	// as test/interop-flowexport/Dockerfile.collectors. Start then builds
	// Image from it through Docker.Build, with the Dockerfile's directory as
	// the context, instead of pulling Image, and releases the tag the run
	// owns after the container is removed.
	Dockerfile string
	// Entrypoint replaces the image's entry point; empty keeps it. A server
	// whose image entry point writes its own configuration and ignores the
	// command needs it to serve the configuration the test declares.
	Entrypoint string
	Command    []string
	// Port is the container port the server listens on, with its transport:
	// interoplab.TCPPort or interoplab.UDPPort.
	Port interoplab.ContainerPort
	// HostNetwork runs the container in the host's network namespace, where
	// the server listens on 127.0.0.1:Port itself and reaches a listener the
	// test opened on the host. Without it, Port is published on the host
	// loopback at a port the daemon picks.
	HostNetwork bool
	// Ready answers nil once the server at address serves. Start asks it
	// every half second until ReadyTimeout passes. TCPReady is the probe for
	// a server whose readiness is accepting a connection.
	Ready func(ctx context.Context, address netip.AddrPort) error
	// ReadyInContainer replaces Ready for a server no probe from the host can
	// observe, such as a UDP listener behind a published port, which accepts
	// datagrams before the server binds: Start runs it in the container, and
	// its exit 0 is ready.
	ReadyInContainer []string
	ReadyTimeout     time.Duration
	// LogLines is the container log tail a failed test receives, 1..1000.
	LogLines int
}

// validate refuses a spec that cannot work before any docker command runs.
func (s Spec) validate() error {
	if !containerNameSafe(s.Name) {
		return errors.New("spec name " + quote(s.Name) + " is not lowercase letters, digits and hyphens")
	}
	if s.Image == "" {
		return errors.New("spec " + s.Name + " names no image")
	}
	if s.Port.Number == 0 {
		return errors.New("spec " + s.Name + " names container port 0")
	}
	if s.Port.Transport == interoplab.TransportUnspecified {
		return errors.New("spec " + s.Name + " names no transport for its port: use interoplab.TCPPort or" +
			" interoplab.UDPPort")
	}
	if s.Ready == nil && len(s.ReadyInContainer) == 0 {
		return errors.New("spec " + s.Name + " has no ready probe: set Ready or ReadyInContainer")
	}
	if s.Ready != nil && len(s.ReadyInContainer) > 0 {
		return errors.New("spec " + s.Name + " sets both Ready and ReadyInContainer: keep one")
	}
	if s.ReadyTimeout <= 0 {
		return errors.New("spec " + s.Name + " has no positive ready timeout")
	}
	if s.LogLines < 1 {
		return errors.New("spec " + s.Name + " asks for fewer than 1 log line")
	}
	if s.LogLines > logLinesMax {
		return errors.New("spec " + s.Name + " asks for more than 1000 log lines")
	}
	return nil
}

// containerNameSafe reports whether name is non-empty lowercase letters,
// digits and hyphens.
func containerNameSafe(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if r == '-' {
			continue
		}
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
}

func quote(value string) string { return "\"" + value + "\"" }

// Start starts the server spec declares and answers the host address it
// serves at, once Ready accepts it. It skips or fails the test, never
// returns an address that does not serve:
//
//   - no docker binary: a skip naming the install route, a failure under
//     ZE_REQUIRE_DOCKER=1;
//   - `docker info` failing: a failure naming the probe error;
//   - an image that cannot be pulled: a failure, except under the live tag
//     run ad hoc, where it is a skip (owner decision Q-6);
//   - a Spec.Dockerfile image that does not build: a failure;
//   - a container that does not start or never becomes ready: a failure.
//
// t.Cleanup writes the last LogLines log lines to the test log when the test
// failed, then removes the container. MUST be called from the test goroutine.
func Start(t testing.TB, spec Spec) netip.AddrPort {
	t.Helper()
	return StartContainer(t, spec).Address
}

// StartContainer is Start for a test that also runs commands in the server's
// container, such as reading what a collector wrote. It answers the
// Container once Ready, or ReadyInContainer, accepts it. MUST be called from
// the test goroutine.
func StartContainer(t testing.TB, spec Spec) Container {
	t.Helper()
	return start(t, host{
		docker:           interoplab.NewDocker(),
		lookPath:         exec.LookPath,
		lookupEnv:        os.LookupEnv,
		moduleRoot:       moduleRoot,
		pullFailureSkips: pullFailureSkips,
		suffix:           randomSuffix,
	}, spec)
}

// TCPReady answers nil once a TCP connection to address succeeds. A
// published port on Docker's userland proxy accepts before the server
// listens, so a server with a protocol-level readiness names that instead.
func TCPReady(ctx context.Context, address netip.AddrPort) error {
	dialer := net.Dialer{Timeout: tcpDialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", address.String())
	if err != nil {
		return err
	}
	return conn.Close()
}

// Client declares one client container that runs a command to its exit: a
// real client driving a server the test runs in-process. Every field but
// Command, HostNetwork and Mounts is required; validate says which is missing.
type Client struct {
	// Name identifies the client in messages: lowercase letters, digits and
	// hyphens.
	Name  string
	Image string
	// Command is the argument list after the image, the image's entry point
	// kept.
	Command []string
	// HostNetwork runs the client in the host's network namespace, where it
	// reaches a listener the test opened on the host loopback.
	HostNetwork bool
	// Mounts are bind mounts: a file the test wrote, such as a CA
	// certificate, read inside the client.
	Mounts []interoplab.Mount
	// Timeout bounds the whole run, the docker client included.
	Timeout time.Duration
}

// validate refuses a client that cannot work before any docker command runs.
func (c Client) validate() error {
	if !containerNameSafe(c.Name) {
		return errors.New("client name " + quote(c.Name) + " is not lowercase letters, digits and hyphens")
	}
	if c.Image == "" {
		return errors.New("client " + c.Name + " names no image")
	}
	if c.Timeout <= 0 {
		return errors.New("client " + c.Name + " has no positive timeout")
	}
	return nil
}

// dockerRunExitMin is the lowest exit code `docker run` keeps for itself:
// 125 when the daemon refuses the run, 126 when the command cannot be
// invoked, 127 when it is not found. A client's own answer is below it.
const dockerRunExitMin = 125

// Run runs the client to its exit and answers its result, whatever exit code
// the client chose: a test asserting a refused request reads a non-zero exit
// and the client's stderr. It skips or fails the test as Start does when
// Docker or the image is absent, and fails it when the container did not run
// to an exit of its own: refused arguments, a docker client stopped by
// Timeout or the test's context, or an exit code docker keeps. The wrapper
// labels and removes the container. MUST be called from the test goroutine.
func Run(t testing.TB, client Client) interoplab.CommandResult {
	t.Helper()
	return run(t, host{
		docker:           interoplab.NewDocker(),
		lookPath:         exec.LookPath,
		lookupEnv:        os.LookupEnv,
		pullFailureSkips: pullFailureSkips,
		suffix:           randomSuffix,
	}, client)
}

func run(t tester, h host, client Client) interoplab.CommandResult {
	t.Helper()
	if err := client.validate(); err != nil {
		t.Fatalf("labtest: %v. Fix: correct the labtest.Client in this test", err)
		return interoplab.CommandResult{}
	}
	if !h.prepare(t, client.Name, client.Image, "") {
		return interoplab.CommandResult{}
	}

	container := interoplab.OneShotContainer{Image: client.Image, Mounts: client.Mounts,
		Command: client.Command, Timeout: client.Timeout}
	if client.HostNetwork {
		container.Network = hostNetwork
	}
	result, err := h.docker.RunOneShot(t.Context(), container)
	if interoplab.OneShotDidNotExit(err) {
		t.Fatalf("labtest %s: the client did not run to an exit: %v", client.Name, err)
		return interoplab.CommandResult{}
	}
	if result.ExitCode >= dockerRunExitMin {
		t.Fatalf("labtest %s: docker run exited %d, a code docker keeps for its own failure: %v",
			client.Name, result.ExitCode, err)
		return interoplab.CommandResult{}
	}
	return result
}

// tester is the part of testing.TB that start uses, so a test can drive both
// skip and failure arms.
type tester interface {
	Helper()
	Context() context.Context
	Cleanup(func())
	Failed() bool
	Logf(format string, args ...any)
	Errorf(format string, args ...any)
	Skipf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// host is what start reads from the machine.
type host struct {
	docker    *interoplab.Docker
	lookPath  func(string) (string, error)
	lookupEnv func(string) (string, bool)
	// moduleRoot answers the directory a Spec.Dockerfile path starts at.
	moduleRoot func() (string, error)
	// pullFailureSkips is true under the live tag: an unpullable image skips
	// an ad hoc run there.
	pullFailureSkips bool
	suffix           func() string
}

func start(t tester, h host, spec Spec) Container {
	t.Helper()
	if err := spec.validate(); err != nil {
		t.Fatalf("labtest: %v. Fix: correct the labtest.Spec in this test", err)
		return Container{}
	}
	if !h.prepare(t, spec.Name, spec.Image, spec.Dockerfile) {
		return Container{}
	}

	ctx := t.Context()
	name := containerPrefix + spec.Name + "-" + h.suffix()
	t.Cleanup(func() { h.release(t, name, spec.LogLines) })
	if err := h.docker.RunDetached(ctx, detached(name, spec)); err != nil {
		t.Fatalf("labtest %s: the container did not start: %v", spec.Name, err)
		return Container{}
	}
	address, err := h.address(ctx, name, spec)
	if err != nil {
		t.Fatalf("labtest %s: %v", spec.Name, err)
		return Container{}
	}
	ready := spec.Ready
	if ready == nil {
		ready = h.readyInContainer(name, spec.ReadyInContainer)
	}
	options := interoplab.WaitOptions{Timeout: spec.ReadyTimeout, Interval: readyInterval,
		Description: "labtest " + spec.Name + " ready at " + address.String()}
	probe := func(ctx context.Context) (struct{}, error) { return struct{}{}, ready(ctx, address) }
	if _, _, err := interoplab.Wait(ctx, options, probe, func(struct{}) bool { return true }); err != nil {
		t.Fatalf("labtest %s: %v. The container log follows this test's failure", spec.Name, err)
		return Container{}
	}
	return Container{Address: address, name: name, docker: h.docker}
}

// prepare answers true once Docker answers and image is local, pulled or,
// when dockerfile is set, built from it, and otherwise skips or fails the
// test as Start documents and answers false.
func (h host) prepare(t tester, name, image, dockerfile string) bool {
	t.Helper()
	required := dockerRequired(h.lookupEnv)
	if _, err := h.lookPath(interoplab.DockerExecutable); err != nil {
		h.absent(t, required, err)
		return false
	}
	ctx := t.Context()
	if err := h.docker.Probe(ctx); err != nil {
		t.Fatalf("labtest %s: the Docker daemon does not answer: %v. Fix: start Docker and check that"+
			" `docker info` succeeds for this user", name, err)
		return false
	}
	if dockerfile != "" {
		return h.build(t, name, image, dockerfile)
	}
	if err := h.docker.Pull(ctx, image, 0); err != nil {
		h.unpullable(t, required, image, err)
		return false
	}
	return true
}

// dockerRequired reports whether ZE_REQUIRE_DOCKER=1 makes a missing Docker
// a failure.
func dockerRequired(lookupEnv func(string) (string, bool)) bool {
	value, ok := lookupEnv(requireDockerVariable)
	if !ok {
		return false
	}
	return value == "1"
}

// absent skips or fails a test on a machine with no docker binary.
func (h host) absent(t tester, required bool, cause error) {
	t.Helper()
	if required {
		t.Fatalf("labtest: no docker binary (%v), and %s=1 says this run must start containers."+
			" Fix: install Docker (https://docs.docker.com/engine/install/) and put `docker` on PATH",
			cause, requireDockerVariable)
		return
	}
	t.Skipf("labtest: no docker binary (%v). Fix: install Docker"+
		" (https://docs.docker.com/engine/install/) to run this test", cause)
}

// unpullable skips or fails a test whose image cannot be pulled (Q-6).
func (h host) unpullable(t tester, required bool, image string, cause error) {
	t.Helper()
	if h.pullFailureSkips && !required {
		t.Skipf("labtest: cannot pull %s (%v); a live test skips on an ad hoc run."+
			" Fix: check the internet connection and `docker pull %s`", image, cause, image)
		return
	}
	t.Fatalf("labtest: cannot pull %s: %v. Fix: check the registry is reachable and `docker pull %s`"+
		" succeeds", image, cause, image)
}

// detached renders spec as the wrapper's container.
func detached(name string, spec Spec) interoplab.DetachedContainer {
	container := interoplab.DetachedContainer{Name: name, Image: spec.Image, Entrypoint: spec.Entrypoint,
		Command: spec.Command}
	if spec.HostNetwork {
		container.Network = hostNetwork
		return container
	}
	container.PublishedPorts = []interoplab.ContainerPort{spec.Port}
	return container
}

// address answers where the started server listens on the host.
func (h host) address(ctx context.Context, name string, spec Spec) (netip.AddrPort, error) {
	if spec.HostNetwork {
		return netip.AddrPortFrom(netip.AddrFrom4([4]byte{127, 0, 0, 1}), spec.Port.Number), nil
	}
	return h.docker.PublishedPort(ctx, name, spec.Port)
}

// release writes the log tail of a failed test, then removes the container.
// It runs from t.Cleanup, after the test's context is canceled, so it runs
// under its own bound.
func (h host) release(t tester, name string, lines int) {
	ctx, cancel := context.WithTimeout(context.Background(), releaseTimeout)
	defer cancel()

	if t.Failed() {
		logs, err := h.docker.Logs(ctx, name, lines)
		if err != nil {
			t.Logf("labtest: cannot read the log of %s: %v", name, err)
		}
		if err == nil {
			t.Logf("labtest: last %d log lines of %s:\n%s", lines, name, logs.Text)
		}
	}
	if err := h.docker.RemoveContainer(ctx, name); err != nil {
		t.Errorf("labtest: cannot remove container %s: %v. Fix: `./le test integration cleanup` removes"+
			" it once this process has exited", name, err)
	}
}

// randomSuffix answers characters no other run's container name holds.
func randomSuffix() string {
	return strings.ToLower(rand.Text())[:suffixLength]
}
