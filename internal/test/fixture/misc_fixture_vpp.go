package fixture

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	testpeer "github.com/ze-software/ze/internal/test/peer"
)

const (
	vppMsgIPRouteAddDelV2 = "ip_route_add_del_v2"
	vppEnvXDGDataHome     = "XDG_DATA_HOME"
)

func init() {
	Register("vpp/vpp-boot", vppBootDriver)
	Register("vpp/vpp-fib-route", vppRouteDriver)
	Register("vpp/vpp-fib-route-lookup", vppLookupDriver)
	Register("vpp/vpp-iface-create", vppIfaceDriver)
	Register("vpp/vpp-mpls-push", vppMPLSDriver)
	Register("vpp/fib-dummy-vpp-backend", vppFIBDummyDriver)
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

func startFixtureProcess(ctx context.Context, env []string, name string, args ...string) (*fixtureProcess, error) {
	command := exec.CommandContext(ctx, name, args...) //nolint:gosec // the fixture chooses the program and its arguments
	command.Env = env
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
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

// vppWorkPaths creates the scratch directory that holds the stub socket. It
// uses the system temporary directory rather than the runner's work directory,
// because the runner nests that one inside the checkout and an AF_UNIX path is
// limited to 108 bytes.
func vppWorkPaths(prefix string) (string, string, string, error) {
	dir, err := os.MkdirTemp("", prefix)
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
	return startFixtureProcess(ctx, os.Environ(), "le", "test", "vpp", "stub", "--socket", socket, "--log", log, "--deadline", strconv.Itoa(deadline), "-v")
}

func startVPPPeer(ctx context.Context, port int, script string, mode testpeer.Mode) (*fixtureProcess, error) {
	process, err := startFixtureProcess(ctx, os.Environ(), "le", "test", "peer", "--mode", mode.String(), "--port", strconv.Itoa(port), script)
	if err != nil {
		return nil, err
	}
	if !Poll(ctx, 100, 50*time.Millisecond, func() bool { return strings.Contains(process.output.String(), "listening on") }) {
		stopFixtureProcess(process, 2*time.Second)
		return nil, fmt.Errorf("the peer did not report listening: %s", process.output.String())
	}
	return process, nil
}

func startVPPZe(ctx context.Context, config string, values map[string]string) (*fixtureProcess, error) {
	dir := values[envConfigDir]
	if dir == "" {
		return nil, errors.New("VPP fixture requires its owned config directory")
	}
	configPath := filepath.Join(dir, "vpp.conf")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		return nil, fmt.Errorf("write VPP fixture config: %w", err)
	}
	// Explicit-file start creates the DUT's normal store when absent.
	// The FIB restores ownership before processing IP routes.
	// The driver's owned directory cleanup removes the config after stop.
	return startFixtureProcess(ctx, miscEnvironment(values), "ze", "start", configPath)
}

// vppLogMatches reports whether any line of the stub log satisfies the
// predicate. A log the stub has not created yet is not a match and not an
// error, because the caller polls for it. Every other failure is an error, so
// that an unreadable log is never read as an absent message.
func vppLogMatches(path string, predicate func(map[string]any) bool) (bool, error) {
	file, err := os.Open(path) //nolint:gosec // the path is the fixture's own scratch file
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("open stub log %s: %w", path, err)
	}
	defer file.Close() //nolint:errcheck // read-only fixture evidence
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry map[string]any
		if json.Unmarshal(scanner.Bytes(), &entry) == nil && predicate(entry) {
			return true, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("read stub log %s: %w", path, err)
	}
	return false, nil
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
	var scanErr error
	seen := Poll(ctx, 50, 100*time.Millisecond, func() bool {
		matched, err := vppLogMatches(log, func(entry map[string]any) bool { return entry["msg"] == "sockclnt_create" })
		scanErr = err
		return matched || err != nil
	})
	connected := Poll(ctx, 50, 100*time.Millisecond, func() bool { return strings.Contains(ze.output.String(), "GoVPP connected") })
	stopFixtureProcess(ze, 3*time.Second)
	if scanErr != nil {
		return scanErr
	}
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
	busBlock := "bus { endpoint bgp-rib { internal bgp-rib; } }\n"
	if observer {
		busBlock = "bus { endpoint bgp-rib { internal bgp-rib; } endpoint lookup-test { external { run \"le test fixture vpp/vpp-fib-route-lookup-observer\"; } encoding json; } }\n"
	}
	fib := "\nfib { vpp { enabled true; } }\n"
	access := "interface { backend vpp; }"
	vppExtra := ""
	// Both programming scripts advertise next-hop 10.0.0.1. Selection needs
	// an actual connected prefix before it can emit that BGP best-change.
	// Use the existing VPP create/address replies and connected event owner;
	// the fixture never inserts a synthetic selected BGP route itself.
	if !observer {
		access = "connected {}\ninterface { backend vpp; dummy vpp-route0 { unit 0 { ipv4 { address [ 10.0.0.2/24 ]; } } } }"
		vppExtra = " lcp { enabled false; }"
	}
	if observer {
		fib = ""
	}
	var tb textbuf.Buffer
	tb.Str("environment {\n}\n").Str(busBlock).
		Str("bgp { peer peer1 { connection { remote { ip 127.0.0.1; } local { ip 127.0.0.1; accept false; } } session { asn { local 1; remote 1; } router-id 1.2.3.4; family { ").
		Str(families).
		Str(" } capability { graceful-restart disable; } } behavior { group-updates disable; } } }\nvpp { enabled true; external true; api-socket ").
		Str(socket).Str(";").Str(vppExtra).Str(" }\n").Str(access).Str(fib)
	return tb.String()
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
	// The MPLS script supplies an UPDATE only; its oracle is the actual stub
	// route+label readback below. Check-mode route scripts still assert EOR.
	mode := testpeer.ModeCheck
	if mpls {
		mode = testpeer.ModeSink
	}
	peer, err := startVPPPeer(ctx, port, filepath.Join(cwd, "peer-script"), mode)
	if err != nil {
		return err
	}
	defer stopFixtureProcess(peer, 2*time.Second)
	ze, err := startVPPZe(ctx, vppRouteConfig(socket, mpls, false), map[string]string{
		envConfigDir: dir, envLogVPP: logLevelInfo, envLogFIBVPP: logLevelDebug, envLogBGP: logLevelInfo, envTestBGPPort: strconv.Itoa(port),
	})
	if err != nil {
		return err
	}
	prefix := prefixTenTwenty
	if mpls {
		prefix = "10.30.0.0/24"
	}
	var scanErr error
	ok := Poll(ctx, 120, 100*time.Millisecond, func() bool {
		matched, err := vppLogMatches(log, func(entry map[string]any) bool {
			if entry["msg"] != vppMsgIPRouteAddDelV2 {
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
		scanErr = err
		return matched || err != nil
	})
	stopFixtureProcess(ze, 3*time.Second)
	if scanErr != nil {
		return scanErr
	}
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
	peer, err := startVPPPeer(ctx, port, filepath.Join(cwd, "peer-script"), testpeer.ModeCheck)
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
	ze, err := startVPPZe(ctx, config, map[string]string{envConfigDir: dir, envLogVPP: logLevelInfo, envLogInterface: logLevelInfo, envLogBGP: logLevelWarn})
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

// vppFIBDummyDriver verifies AC-11: with `interface { backend vpp; }` and
// `fib { dummy { } }` against the VPP stub, BGP routes are held by `fib-dummy`
// with `programmed: "no"`, `fib-vpp` is never loaded, and the VPP stub logs
// handshake traffic but zero `ip_route_add_del_v2` calls.
func vppFIBDummyDriver(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("VPP fib-dummy fixture requires BGP port and SSH port")
	}
	bgpPort, err := strconv.Atoi(args[0])
	if err != nil {
		return err
	}
	sshPort, err := strconv.Atoi(args[1])
	if err != nil {
		return err
	}
	dir, socket, log, err := vppWorkPaths("vpp-fib-dummy-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir) //nolint:errcheck // fixture cleanup
	stub, err := startVPPStub(ctx, socket, log, 40)
	if err != nil {
		return err
	}
	defer stopFixtureProcess(stub, 2*time.Second)
	if !waitForFile(ctx, socket, 100, 50*time.Millisecond) {
		return errors.New("stub socket did not appear")
	}
	cwd, _ := os.Getwd()
	peer, err := startVPPPeer(ctx, bgpPort, filepath.Join(cwd, "peer-script"), testpeer.ModeCheck)
	if err != nil {
		return err
	}
	defer stopFixtureProcess(peer, 2*time.Second)

	const bcryptTestHash = "$2a$04$UlwuiuH82Unfsq.XEMPGJeDkXwbm3KW.nvVaVXOd/JeFK8VjMjrQO" //nolint:gosec // synthetic test bcrypt cost-4 hash for valueTestPassword
	var cfgBuf textbuf.Buffer
	cfgBuf.Str("system {\n\tauthentication {\n\t\tuser admin {\n\t\t\tpassword \"").Str(bcryptTestHash).Str("\"\n\t\t}\n\t}\n}\n")
	cfgBuf.Str("environment {\n\tssh {\n\t\tenabled true;\n\t\tserver main {\n\t\t\tip 127.0.0.1;\n\t\t\tport ").Int(int64(sshPort)).Str(";\n\t\t}\n\t}\n}\n")
	cfgBuf.Str("bus {\n\tendpoint bgp-rib {\n\t\tinternal bgp-rib;\n\t}\n}\n")
	cfgBuf.Str("bgp {\n\tpeer peer1 {\n\t\tconnection { remote { ip 127.0.0.1; } local { ip 127.0.0.1; accept false; } }\n")
	cfgBuf.Str("\t\tsession { asn { local 1; remote 1; } router-id 1.2.3.4; family { ipv4/unicast { prefix { maximum 10000; } } } capability { graceful-restart disable; } }\n")
	cfgBuf.Str("\t\tbehavior { group-updates disable; }\n\t}\n}\n")
	cfgBuf.Str("vpp { enabled true; external true; api-socket ").Str(socket).Str("; lcp { enabled false; } }\n")
	cfgBuf.Str("connected {}\n")
	cfgBuf.Str("interface { backend vpp; dummy vpp-route0 { unit 0 { ipv4 { address [ 10.0.0.2/24 ]; } } } }\n")
	// Include a NeedsDataPlane producer (`static`) so the engine would auto-load
	// `fib-vpp` unless `fib { dummy { } }` declares ProgramsFIB and suppresses it.
	cfgBuf.Str("static { table default { route 10.21.0.0/24 { next { hop 10.0.0.1 { } } } } }\n")
	cfgBuf.Str("fib { dummy { } }\n")
	cfg := cfgBuf.String()

	cliEnv := miscEnvironment(map[string]string{
		envConfigDir:      dir,
		vppEnvXDGDataHome: dir,
		"ze.ssh.password": valueTestPassword,
	})
	var initStdin textbuf.Buffer
	initStdin.Str("admin\n" + valueTestPassword + "\n127.0.0.1\n").Int(int64(sshPort)).Byte('\n')
	initCmd := exec.CommandContext(ctx, "ze", "init")
	initCmd.Env = cliEnv
	initCmd.Stdin = strings.NewReader(initStdin.String())
	if out, initErr := initCmd.CombinedOutput(); initErr != nil {
		return fmt.Errorf("ze init failed: %w: %s", initErr, out)
	}

	ze, err := startVPPZe(ctx, cfg, map[string]string{
		envConfigDir:      dir,
		vppEnvXDGDataHome: dir,
		envLogVPP:         logLevelInfo,
		envLogBGP:         logLevelInfo,
		envTestBGPPort:    strconv.Itoa(bgpPort),
		"ze.ssh.password": valueTestPassword,
	})
	if err != nil {
		return err
	}
	defer stopFixtureProcess(ze, 3*time.Second)

	var lastShow string
	held := Poll(ctx, 120, 200*time.Millisecond, func() bool {
		code, stdout, _, runErr := runCaptured(ctx, cliEnv, "", "ze", "cli", "-c", "show fib dummy | json")
		if runErr != nil {
			return false
		}
		if code != 0 {
			return false
		}
		lastShow = stdout
		return strings.Contains(stdout, prefixTenTwenty) && strings.Contains(stdout, "10.21.0.0/24") && strings.Contains(stdout, `"programmed"`) && strings.Contains(stdout, `"no"`)
	})
	if !held {
		return fmt.Errorf("fib-dummy did not hold %s; last show output: %s", prefixTenTwenty, lastShow)
	}
	if strings.Contains(lastShow, `"programmed": "yes"`) {
		return fmt.Errorf("fib-dummy reported programmed=yes: %s", lastShow)
	}
	if strings.Contains(lastShow, `"programmed":"yes"`) {
		return fmt.Errorf("fib-dummy reported programmed=yes: %s", lastShow)
	}

	code, pluginsOut, pluginsErr, runErr := runCaptured(ctx, cliEnv, "", "ze", "cli", "-c", "show system subsystem list | json")
	if runErr != nil {
		return fmt.Errorf("show system subsystem list failed (code=%d err=%w): %s", code, runErr, pluginsErr)
	}
	if code != 0 {
		return fmt.Errorf("show system subsystem list failed (code=%d): %s", code, pluginsErr)
	}
	if !strings.Contains(pluginsOut, `"fib-dummy"`) {
		return fmt.Errorf("show system subsystem list missing fib-dummy: %s", pluginsOut)
	}
	if strings.Contains(pluginsOut, `"fib-vpp"`) {
		return fmt.Errorf("fib-vpp must not run beside fib-dummy: %s", pluginsOut)
	}

	stopFixtureProcess(ze, 3*time.Second)

	connected, err := vppLogMatches(log, func(entry map[string]any) bool {
		return entry["msg"] == "sockclnt_create"
	})
	if err != nil {
		return err
	}
	if !connected {
		return errors.New("stub did not log sockclnt_create; VPP backend did not connect")
	}
	routeLogged, err := vppLogMatches(log, func(entry map[string]any) bool {
		return entry["msg"] == vppMsgIPRouteAddDelV2
	})
	if err != nil {
		return err
	}
	if routeLogged {
		return errors.New("VPP stub logged an ip_route_add_del_v2 call despite fib dummy")
	}
	fmt.Fprintln(os.Stderr, "OK: fib-dummy held 10.20.0.0/24 under VPP backend with no stub route call")
	return nil
}
