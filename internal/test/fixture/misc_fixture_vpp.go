package fixture

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

func init() {
	Register("vpp/vpp-boot", vppBootDriver)
	Register("vpp/vpp-fib-route", vppRouteDriver)
	Register("vpp/vpp-fib-route-lookup", vppLookupDriver)
	Register("vpp/vpp-iface-create", vppIfaceDriver)
	Register("vpp/vpp-mpls-push", vppMPLSDriver)
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *lockedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(data)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

// fixtureProcess is one child process a fixture started, and the channel its
// exit status arrives on.
//
// done is buffered with one slot and every reader PUTS THE STATUS BACK, so the
// exit can be read any number of times. Without that, a driver that waits for
// the process and then stops it blocks forever on the second read: the wait
// drained the only value, and the stop is left receiving from a channel nothing
// will ever send to again.
type fixtureProcess struct {
	command *exec.Cmd
	done    chan error
	output  *lockedBuffer
}

// exit answers the status the process ended with and leaves it readable for the
// next caller. It MUST only be called once a receive on done has succeeded.
func (p *fixtureProcess) exit(status error) error {
	p.done <- status
	return status
}

func startFixtureProcess(ctx context.Context, env []string, stdin, name string, args ...string) (*fixtureProcess, error) {
	command := exec.CommandContext(ctx, name, args...) //nolint:gosec // the fixture chooses the program and its arguments
	command.Env = env
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if stdin != "" {
		command.Stdin = strings.NewReader(stdin)
	}
	output := new(lockedBuffer)
	command.Stdout = io.MultiWriter(os.Stderr, output)
	command.Stderr = io.MultiWriter(os.Stderr, output)
	if err := command.Start(); err != nil {
		return nil, err
	}
	process := &fixtureProcess{command: command, done: make(chan error, 1), output: output}
	go func() { process.done <- command.Wait() }()
	return process, nil
}

func stopFixtureProcess(process *fixtureProcess, grace time.Duration) {
	if process == nil || process.command.Process == nil {
		return
	}
	_ = syscall.Kill(-process.command.Process.Pid, syscall.SIGTERM)
	select {
	case status := <-process.done:
		_ = process.exit(status)
	case <-time.After(grace):
		_ = syscall.Kill(-process.command.Process.Pid, syscall.SIGKILL)
		_ = process.exit(<-process.done)
	}
}

func waitFixtureProcess(ctx context.Context, process *fixtureProcess, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case status := <-process.done:
		return process.exit(status)
	case <-timer.C:
		return errors.New("process timeout")
	case <-ctx.Done():
		return ctx.Err()
	}
}

func vppWorkPaths(prefix string) (string, string, string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", "", err
	}
	dir, err := os.MkdirTemp(cwd, prefix)
	if err != nil {
		return "", "", "", err
	}
	socket := filepath.Join(dir, "api.sock")
	if len(socket) >= 108 {
		_ = os.RemoveAll(dir)
		return "", "", "", fmt.Errorf("socket path too long: %s", socket)
	}
	return dir, socket, filepath.Join(dir, "vpp-requests.jsonl"), nil
}

func startVPPStub(ctx context.Context, socket, log string, deadline int) (*fixtureProcess, error) {
	return startFixtureProcess(ctx, os.Environ(), "", "ze-test", "vpp-stub", "--socket", socket, "--log", log, "--deadline", strconv.Itoa(deadline), "-v")
}

func startVPPPeer(ctx context.Context, port int, script string) (*fixtureProcess, error) {
	process, err := startFixtureProcess(ctx, os.Environ(), "", "ze-test", "peer", "--port", strconv.Itoa(port), script)
	if err != nil {
		return nil, err
	}
	if !Poll(ctx, 100, 50*time.Millisecond, func() bool { return strings.Contains(process.output.String(), "listening on") }) {
		stopFixtureProcess(process, 2*time.Second)
		return nil, fmt.Errorf("ze-peer did not report listening: %s", process.output.String())
	}
	return process, nil
}

func startVPPZe(ctx context.Context, config string, values map[string]string) (*fixtureProcess, error) {
	return startFixtureProcess(ctx, miscEnvironment(values), config, "ze", "-")
}

func vppLogMatches(path string, predicate func(map[string]any) bool) bool {
	file, err := os.Open(path) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return false
	}
	defer file.Close() //nolint:errcheck // read-only fixture evidence
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry map[string]any
		if json.Unmarshal(scanner.Bytes(), &entry) == nil && predicate(entry) {
			return true
		}
	}
	return false
}

func vppBootDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("vpp boot fixture takes no arguments")
	}
	dir, socket, log, err := vppWorkPaths("vpp-boot-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir) //nolint:errcheck // fixture cleanup
	stub, err := startVPPStub(ctx, socket, log, 20)
	if err != nil {
		return err
	}
	defer stopFixtureProcess(stub, 2*time.Second)
	if !waitForFile(ctx, socket, 100, 50*time.Millisecond) {
		return errors.New("stub socket did not appear")
	}
	config := fmt.Sprintf("environment {\n}\n\nbgp {\n router-id 10.0.0.1;\n session { asn { local 65533; } }\n}\n\nvpp {\n enabled true;\n external true;\n api-socket %s;\n}\n", socket)
	ze, err := startVPPZe(ctx, config, map[string]string{envConfigDir: dir, envLogVPP: logLevelInfo, envLogBGP: logLevelWarn})
	if err != nil {
		return err
	}
	seen := Poll(ctx, 50, 100*time.Millisecond, func() bool {
		return vppLogMatches(log, func(entry map[string]any) bool { return entry["msg"] == "sockclnt_create" })
	})
	connected := Poll(ctx, 50, 100*time.Millisecond, func() bool { return strings.Contains(ze.output.String(), "GoVPP connected") })
	stopFixtureProcess(ze, 3*time.Second)
	if !seen {
		return errors.New("stub did not log sockclnt_create")
	}
	if !connected {
		return errors.New("ze did not report GoVPP connected")
	}
	fmt.Fprintln(os.Stderr, "OK: handshake observed, ze connected via external stub")
	return nil
}

func vppRouteConfig(socket string, mpls, observer bool) string {
	families := "ipv4/unicast { prefix { maximum 10000; } }"
	if mpls {
		families += " ipv4/mpls-label { prefix { maximum 10000; } }"
	}
	plugin := ""
	process := ""
	if observer {
		plugin = "plugin { external lookup-test { run \"ze-test fixture vpp/vpp-fib-route-lookup-observer\"; encoder json; } }\n"
		process = "process lookup-test { }"
	}
	fib := "\nfib { vpp { enabled true; } }\n"
	if observer {
		fib = ""
	}
	return fmt.Sprintf("environment {\n}\n%sbgp { peer peer1 { connection { remote { ip 127.0.0.1; } local { ip 127.0.0.1; accept false; } } session { asn { local 1; remote 1; } router-id 1.2.3.4; family { %s } capability { graceful-restart disable; } } behavior { group-updates disable; } %s } }\nvpp { enabled true; external true; api-socket %s; }%s", plugin, families, process, socket, fib)
}

func vppRouteDriver(ctx context.Context, args []string) error {
	return runVPPRouteProgramming(ctx, args, false)
}

func vppMPLSDriver(ctx context.Context, args []string) error {
	return runVPPRouteProgramming(ctx, args, true)
}

func runVPPRouteProgramming(ctx context.Context, args []string, mpls bool) error {
	if len(args) != 1 {
		return errors.New("VPP route fixture requires the BGP port")
	}
	port, err := strconv.Atoi(args[0])
	if err != nil {
		return err
	}
	dir, socket, log, err := vppWorkPaths("vpp-route-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir) //nolint:errcheck // fixture cleanup
	stub, err := startVPPStub(ctx, socket, log, 30)
	if err != nil {
		return err
	}
	defer stopFixtureProcess(stub, 2*time.Second)
	if !waitForFile(ctx, socket, 100, 50*time.Millisecond) {
		return errors.New("stub socket did not appear")
	}
	cwd, _ := os.Getwd()
	peer, err := startVPPPeer(ctx, port, filepath.Join(cwd, "peer-script"))
	if err != nil {
		return err
	}
	defer stopFixtureProcess(peer, 2*time.Second)
	ze, err := startVPPZe(ctx, vppRouteConfig(socket, mpls, false), map[string]string{
		envConfigDir: dir, envLogVPP: logLevelInfo, "ze.log.fib.vpp": logLevelDebug, envLogBGP: logLevelInfo, envTestBGPPort: strconv.Itoa(port),
	})
	if err != nil {
		return err
	}
	prefix := prefixTenTwenty
	if mpls {
		prefix = "10.30.0.0/24"
	}
	ok := Poll(ctx, 120, 100*time.Millisecond, func() bool {
		return vppLogMatches(log, func(entry map[string]any) bool {
			if entry["msg"] != "ip_route_add_del" {
				return false
			}
			fields, _ := entry["fields"].(map[string]any)
			if fields["is_add"] != true || fields["prefix"] != prefix {
				return false
			}
			if !mpls {
				return true
			}
			labels, _ := fields["labels"].([]any)
			for _, label := range labels {
				if label == float64(100) {
					return true
				}
			}
			return false
		})
	})
	stopFixtureProcess(ze, 3*time.Second)
	if !ok {
		if data, readErr := os.ReadFile(log); readErr == nil { //nolint:gosec // the path is the fixture's own scratch file
			fmt.Fprintf(os.Stderr, "--- stub log ---\n%s--- end ---\n", data)
		}
		return fmt.Errorf("stub did not observe programmed route %s", prefix)
	}
	if mpls {
		fmt.Fprintln(os.Stderr, "OK: fib-vpp programmed 10.30.0.0/24 with label 100 into stub")
	} else {
		fmt.Fprintln(os.Stderr, "OK: fib-vpp programmed 10.20.0.0/24 into stub")
	}
	return nil
}

func vppLookupDriver(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("VPP lookup fixture requires the BGP port")
	}
	port, err := strconv.Atoi(args[0])
	if err != nil {
		return err
	}
	dir, socket, log, err := vppWorkPaths("vpp-lookup-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir) //nolint:errcheck // fixture cleanup
	stub, err := startVPPStub(ctx, socket, log, 30)
	if err != nil {
		return err
	}
	defer stopFixtureProcess(stub, 2*time.Second)
	if !waitForFile(ctx, socket, 100, 50*time.Millisecond) {
		return errors.New("stub socket did not appear")
	}
	cwd, _ := os.Getwd()
	peer, err := startVPPPeer(ctx, port, filepath.Join(cwd, "peer-script"))
	if err != nil {
		return err
	}
	defer stopFixtureProcess(peer, 2*time.Second)
	ze, err := startVPPZe(ctx, vppRouteConfig(socket, false, true), map[string]string{
		envConfigDir: dir, envLogVPP: logLevelInfo, "ze.log.iface": logLevelDebug, envTestBGPPort: strconv.Itoa(port),
	})
	if err != nil {
		return err
	}
	if err := waitFixtureProcess(ctx, ze, 25*time.Second); err != nil {
		stopFixtureProcess(ze, 3*time.Second)
		return fmt.Errorf("ze did not exit within 25s: %w", err)
	}
	if !strings.Contains(ze.output.String(), "OK: route lookup returned prefix=10.20.0.0/24") {
		return errors.New("route lookup did not succeed")
	}
	return nil
}

func vppIfaceDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("VPP interface fixture takes no arguments")
	}
	dir, socket, log, err := vppWorkPaths("vpp-iface-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir) //nolint:errcheck // fixture cleanup
	stub, err := startVPPStub(ctx, socket, log, 20)
	if err != nil {
		return err
	}
	defer stopFixtureProcess(stub, 2*time.Second)
	if !waitForFile(ctx, socket, 100, 50*time.Millisecond) {
		return errors.New("stub socket did not appear")
	}
	config := fmt.Sprintf("environment {\n}\nbgp { router-id 10.0.0.1; session { asn { local 65533; } } }\nvpp { enabled true; external true; api-socket %s; }\ninterface { backend vpp; }\n", socket)
	ze, err := startVPPZe(ctx, config, map[string]string{envConfigDir: dir, envLogVPP: logLevelInfo, "ze.log.interface": logLevelInfo, envLogBGP: logLevelWarn})
	if err != nil {
		return err
	}
	loaded := Poll(ctx, 100, 100*time.Millisecond, func() bool {
		output := ze.output.String()
		return strings.Contains(output, "interface backend loaded") && strings.Contains(output, "backend=vpp")
	})
	stopFixtureProcess(ze, 3*time.Second)
	if !loaded {
		return errors.New("ze did not log interface backend loaded backend=vpp")
	}
	fmt.Fprintln(os.Stderr, "OK: ifacevpp loaded and name map populated")
	return nil
}
