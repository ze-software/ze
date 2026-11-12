// Design: docs/architecture/testing/runner-architecture.md -- owned two-daemon tutorial proof.
// Related: docs/guide/quickstart.md -- the supplied inputs and received-prefix checks.
package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/test/runner"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// quickstartPorts holds three distinct unprivileged ports already leased by the
// runner. Its zero value is invalid; only readQuickstartPorts admits inputs.
type quickstartPorts [3]uint16

// quickstartInput describes one source file and its private port substitutions.
// Resolver output also stays under that daemon's private directory.
// It carries raw filesystem inputs; prepareQuickstart validates the resulting file.
type quickstartInput struct {
	name    string
	peer    string
	sshPort uint16
	bgpPort uint16
}

const quickstartReadyName = "daemon.ready"

// deploymentQuickstart proves the documented chain with the runner's own Ze:
// supplied config, actual TCP/SSH sessions, received UPDATEs and clean shutdown.
// Every child is joined before the case directory or its port lease is released.
func deploymentQuickstart(ctx context.Context, args []string) (retErr error) {
	ports, err := readQuickstartPorts(args)
	if err != nil {
		return err
	}
	repository, err := lepath.Root()
	if err != nil {
		return err
	}
	binary, err := uiZEBinary()
	if err != nil {
		return err
	}
	parent, err := os.Getwd()
	if err != nil {
		return err
	}
	work, err := os.MkdirTemp(parent, "deployment-quickstart-")
	if err != nil {
		return err
	}
	var firstOutput, secondOutput lockedBuffer
	defer func() {
		if retErr != nil {
			fmt.Fprintln(os.Stderr, "quickstart r1:", firstOutput.String())
			fmt.Fprintln(os.Stderr, "quickstart r2:", secondOutput.String())
		}
		retErr = errors.Join(retErr, os.RemoveAll(work))
	}()
	firstConfig, err := prepareQuickstart(ctx, binary, repository, work,
		quickstartInput{name: "r1", peer: "r2", sshPort: ports[0], bgpPort: ports[2]})
	if err != nil {
		return err
	}
	secondConfig, err := prepareQuickstart(ctx, binary, repository, work,
		quickstartInput{name: "r2", peer: "r1", sshPort: ports[1], bgpPort: ports[2]})
	if err != nil {
		return err
	}
	// The hub publishes readiness after the BGP plugin opens its listeners.
	// Start the passive peer first so the active peer needs no startup retry.
	second, err := startQuickstart(ctx, binary, secondConfig, &secondOutput)
	if err != nil {
		return err
	}
	defer func() { retErr = errors.Join(retErr, second.Close()) }()
	if err := awaitQuickstartReady(ctx, second, filepath.Join(filepath.Dir(secondConfig), quickstartReadyName)); err != nil {
		return fmt.Errorf("passive r2 readiness: %w", err)
	}
	first, err := startQuickstart(ctx, binary, firstConfig, &firstOutput)
	if err != nil {
		return err
	}
	defer func() { retErr = errors.Join(retErr, first.Close()) }()

	// This directory deliberately has no credential database. Explicit remote
	// flags and a password must suffice, as they do for a newly installed client.
	clientDir := filepath.Join(work, "client")
	if err := os.Mkdir(clientDir, 0o700); err != nil {
		return err
	}
	clientEnv := childEnvironment(os.Environ(), map[string]string{
		envConfigDirDotted: clientDir,
		envSSHPassword:     "lab",
	})
	readyCtx, cancel := context.WithTimeout(ctx, deploymentBudget(40*time.Second))
	defer cancel()
	last := errors.New("the two peers have not answered yet")
	ready := Poll(readyCtx, 200*runner.ChildParallelFactor(), 100*time.Millisecond, func() bool {
		select {
		case <-first.Done():
			last = fmt.Errorf("r1 exited before route verification: %w", exitCause(first.Wait()))
			return true
		case <-second.Done():
			last = fmt.Errorf("r2 exited before route verification: %w", exitCause(second.Wait()))
			return true
		default:
		}
		last = checkQuickstart(readyCtx, binary, clientEnv, ports[0], netip.MustParsePrefix("198.51.100.0/24"))
		if last != nil {
			return false
		}
		last = checkQuickstart(readyCtx, binary, clientEnv, ports[1], netip.MustParsePrefix("192.0.2.0/24"))
		return last == nil
	})
	if !ready {
		return fmt.Errorf("two-peer route verification did not complete: %w", errors.Join(last, readyCtx.Err()))
	}
	if last != nil {
		return last
	}
	if err := rejectQuickstartPassword(ctx, binary, clientEnv, ports[0]); err != nil {
		return err
	}
	if err := checkQuickstart(ctx, binary, clientEnv, ports[0], netip.MustParsePrefix("198.51.100.0/24")); err != nil {
		return fmt.Errorf("valid login after rejected password: %w", err)
	}
	if err := stopQuickstart(first, &firstOutput); err != nil {
		return fmt.Errorf("stop r1: %w", err)
	}
	if err := stopQuickstart(second, &secondOutput); err != nil {
		return fmt.Errorf("stop r2: %w", err)
	}
	_, err = fmt.Fprintln(os.Stdout, "DEPLOYMENT-QUICKSTART: PASS both sessions established, opposite prefixes received, bad login denied, both daemons stopped cleanly")
	return err
}

// exitCause answers why a daemon exited, in a form %w can carry: a clean exit
// has no error, and %w renders nil as a formatting fault.
func exitCause(exit error) error {
	if exit == nil {
		return errors.New("exit status 0")
	}
	return exit
}

func readQuickstartPorts(args []string) (quickstartPorts, error) {
	if len(args) != 3 {
		return quickstartPorts{}, errors.New("deployment-quickstart needs the runner's three leased ports")
	}
	var ports quickstartPorts
	for index, arg := range args {
		port, err := strconv.ParseUint(arg, 10, 16)
		if err != nil {
			return quickstartPorts{}, fmt.Errorf("leased port %q: %w", arg, err)
		}
		if port < 1024 {
			return quickstartPorts{}, fmt.Errorf("leased port %d must be unprivileged", port)
		}
		for previous := range index {
			if uint64(ports[previous]) == port {
				return quickstartPorts{}, fmt.Errorf("leased port %d is repeated", port)
			}
		}
		ports[index] = uint16(port)
	}
	return ports, nil
}

func prepareQuickstart(ctx context.Context, binary, repository, work string, input quickstartInput) (string, error) {
	body, err := os.ReadFile(filepath.Join(repository, "examples", "quickstart", input.name+".conf")) //nolint:gosec // the path is a quickstart example the fixture names under the repository
	if err != nil {
		return "", err
	}
	dir := filepath.Join(work, input.name)
	if err := os.Mkdir(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "ze.conf")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return "", err
	}
	environment := daemonEnvironment(os.Environ(), dir, daemonStoreDisk)
	updates := [][]string{
		{"environment", "ssh", "server", "main", leafPort, strconv.Itoa(int(input.sshPort))},
		{"bgp", "peer", input.peer, "connection", "local", leafPort, strconv.Itoa(int(input.bgpPort))},
		{"bgp", "peer", input.peer, "connection", "remote", leafPort, strconv.Itoa(int(input.bgpPort))},
		{"system", "dns", "resolv-conf-path", filepath.Join(dir, "resolv.conf")},
	}
	for _, update := range updates {
		argv := append([]string{binary, "config", "set", path}, update...)
		code, stdout, stderr, err := runCaptured(ctx, environment, "", argv...)
		if err != nil {
			return "", err
		}
		if code != 0 {
			return "", fmt.Errorf("adapt %s private config via config set: exit %d: %s%s", input.name, code, stdout, stderr)
		}
	}
	code, stdout, stderr, err := runCaptured(ctx, environment, "", binary, "config", "validate", path)
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("validate private %s config: exit %d: %s%s", input.name, code, stdout, stderr)
	}
	return path, nil
}

func startQuickstart(ctx context.Context, binary, path string, output *lockedBuffer) (*runner.ServerProcess, error) {
	command := exec.CommandContext(ctx, binary, "start", path) //nolint:gosec // binary and configuration belong to this functional run
	command.Env = childEnvironment(daemonEnvironment(os.Environ(), filepath.Dir(path), daemonStoreDisk), map[string]string{
		envReadyFile: filepath.Join(filepath.Dir(path), quickstartReadyName),
	})
	command.Stdout, command.Stderr = output, output
	return runner.StartServerProcess(command)
}

func awaitQuickstartReady(ctx context.Context, process *runner.ServerProcess, path string) error {
	readyCtx, cancel := context.WithTimeout(ctx, deploymentBudget(30*time.Second))
	defer cancel()
	last := errors.New("the passive peer has not published readiness")
	ready := Poll(readyCtx, 300*runner.ChildParallelFactor(), 100*time.Millisecond, func() bool {
		select {
		case <-process.Done():
			last = fmt.Errorf("daemon exited before readiness: %w", exitCause(process.Wait()))
			return true
		default:
		}
		_, last = os.Stat(path)
		if last == nil {
			return true
		}
		return !errors.Is(last, os.ErrNotExist)
	})
	if !ready {
		return fmt.Errorf("wait for %s: %w", path, errors.Join(last, readyCtx.Err()))
	}
	return last
}

func queryQuickstart(ctx context.Context, binary string, environment []string, port uint16, query string) (string, error) {
	address := net.JoinHostPort(addrLoopback, strconv.Itoa(int(port)))
	code, stdout, stderr, err := runCaptured(ctx, environment, "", binary, "cli", "--remote", address, "--user", "lab", "-c", query)
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("remote command %q on %s: exit %d: %s%s", query, address, code, stdout, stderr)
	}
	return stdout, nil
}

func checkQuickstart(ctx context.Context, binary string, environment []string, port uint16, expected netip.Prefix) error {
	answer, err := queryQuickstart(ctx, binary, environment, port, "show bgp | json compact")
	if err != nil {
		return err
	}
	var summary struct {
		Established int `json:"peers-established"`
	}
	if err := json.Unmarshal([]byte(answer), &summary); err != nil {
		return fmt.Errorf("decode BGP summary: %w", err)
	}
	if summary.Established != 1 {
		return fmt.Errorf("SSH port %d reports %d established peers, want 1", port, summary.Established)
	}
	answer, err = queryQuickstart(ctx, binary, environment, port, "show bgp rib | received | json compact")
	if err != nil {
		return err
	}
	var rib struct {
		Routes []struct {
			Prefix    netip.Prefix         `json:"prefix"`
			Direction rpc.MessageDirection `json:"direction"`
		} `json:"routes"`
	}
	if err := json.Unmarshal([]byte(answer), &rib); err != nil {
		return fmt.Errorf("decode received routes: %w", err)
	}
	if len(rib.Routes) != 1 {
		return fmt.Errorf("SSH port %d reports %d received routes, want %s alone", port, len(rib.Routes), expected)
	}
	if rib.Routes[0].Prefix != expected {
		return fmt.Errorf("SSH port %d received %s, want %s", port, rib.Routes[0].Prefix, expected)
	}
	if rib.Routes[0].Direction != rpc.DirectionReceived {
		return fmt.Errorf("SSH port %d answered an outbound route for a received-route query", port)
	}
	return nil
}

func rejectQuickstartPassword(ctx context.Context, binary string, environment []string, port uint16) error {
	bad := childEnvironment(environment, map[string]string{envSSHPassword: "deliberately-wrong"})
	address := net.JoinHostPort(addrLoopback, strconv.Itoa(int(port)))
	code, stdout, stderr, err := runCaptured(ctx, bad, "", binary, "cli", "--remote", address, "--user", "lab", "-c", "show bgp peer list | json compact")
	if err != nil {
		return err
	}
	if code == 0 {
		return errors.New("quickstart accepted a deliberately incorrect password")
	}
	// This pinned SSH producer reports an authentication failure by this phrase.
	// A missing database, failed TCP connect or command refusal cannot satisfy it.
	if !strings.Contains(stderr, "unable to authenticate") {
		return fmt.Errorf("bad login failed for another reason: exit %d: %s%s", code, stdout, stderr)
	}
	return nil
}

func stopQuickstart(process *runner.ServerProcess, output *lockedBuffer) error {
	if err := process.Stop(syscall.SIGTERM, deploymentBudget(90*time.Second), 0); err != nil {
		return err
	}
	// Stop can normalize an intentional signal death. A clean Ze shutdown must
	// instead exit zero through its handler and publish its completion message.
	if err := process.Wait(); err != nil {
		return err
	}
	if !strings.Contains(output.String(), "Ze stopped.") {
		return errors.New("daemon exited without its normal shutdown completion")
	}
	return nil
}
