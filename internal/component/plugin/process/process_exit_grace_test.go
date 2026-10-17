// Design: docs/architecture/api/process-protocol.md -- how an external plugin is started and stopped
// Overview: process.go -- Stop, superviseExit, monitorCmd, the stop these tests exercise
//
// Goal: prove the daemon's stop lets a plugin finish its own exit before any
// signal reaches it, still stops a plugin that never exits, and still reaches
// what a plugin that exited on its own left behind. Method: the test binary
// re-runs itself as a real external plugin that connects back over the hub's
// TLS, writes its stderr through a goroutine relay the way crashlog.Init does
// in every ze process, and is stopped by (*Process).Stop the moment its start
// returns.
//
// Linux only: elsewhere the stop kills the group at once, by design
// (exitwait_other.go, stopGivesExitGrace).

//go:build linux

package process

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
)

const (
	// exitChildModeEnv selects the fake plugin's behavior when the test binary
	// runs as the child. Unset, TestMain runs the tests.
	exitChildModeEnv = "ZE_TEST_EXIT_GRACE_CHILD"
	// exitChildPidEnv names the file the fake plugin writes its pid to.
	exitChildPidEnv = "ZE_TEST_EXIT_GRACE_PID"
	// exitChildDescendantPidEnv names the file the fake plugin writes the pid
	// of the descendant it starts to, in exitChildLeavesDescendant mode.
	exitChildDescendantPidEnv = "ZE_TEST_EXIT_GRACE_DESCENDANT_PID"

	exitChildFlushes          = "flushes-on-eof"
	exitChildIgnores          = "ignores-eof"
	exitChildLeavesDescendant = "leaves-a-descendant"
	// exitChildDescendant is the mode of the process the fake plugin starts in
	// exitChildLeavesDescendant mode: it sleeps until something kills it.
	exitChildDescendant = "descendant"
	exitChildLastLine   = "plugin line written just before the stop"

	// exitChildRelayLag is how long the child's stderr relay holds a line
	// before it writes it on. A loaded machine produces the same lag for free;
	// the test produces it on purpose, so a stop that does not wait for the
	// plugin's own exit loses the line on every run rather than on some.
	exitChildRelayLag = 300 * time.Millisecond
)

// TestStopKeepsTheLineAPluginWroteJustBeforeIt holds what the stop owes a
// plugin that exits when its connection ends: the time to write out what it
// already wrote.
//
// Every ze process points os.Stderr at a pipe of its own that a goroutine
// copies to the real stderr (crashlog.Init), so a line the plugin wrote is in
// that relay's queue, not in the daemon's pipe, until the relay catches up.
// Only the plugin's own orderly exit drains the queue. The fake plugin here has
// the same shape, with a relay that lags by exitChildRelayLag.
//
// VALIDATES: the daemon's plugin log carries a line the plugin wrote
// immediately before (*Process).Stop.
// PREVENTS: the stop that canceled the process context and so SIGKILLed the
// plugin's group at once, which lost the plugin's last stderr line under load
// and failed attach-process-unattached-is-silent, attach-process-runtime-
// subscribe and drain-restart-kind on CI.
//
// DISCRIMINATES: make Stop kill the group at once (call group.kill() at the top
// of superviseExit's stop branch) and the line never reaches the relay; leave the
// connection open until the grace runs out and the plugin is killed, not exited.
func TestStopKeepsTheLineAPluginWroteJustBeforeIt(t *testing.T) {
	var relayed strings.Builder
	captureRelay(t, &relayed)

	proc, pid := startExitGraceChild(t, exitChildFlushes)
	proc.Stop()
	waitStopped(t, proc, pluginExitGrace+processStopBudget)

	if !strings.Contains(relayed.String(), exitChildLastLine) {
		t.Fatalf("the plugin's last stderr line never reached the daemon log; relayed:\n%s", relayed.String())
	}
	// The relay lag is shorter than the grace, so the line alone would also
	// survive a stop that killed the plugin when the grace ran out. The exit
	// status says which happened: this plugin exits 0 on its closed connection.
	if state := proc.Cmd().ProcessState; state == nil || !state.Exited() || state.ExitCode() != 0 {
		t.Fatalf("the plugin did not exit on its own when its connection closed: %v", state)
	}
	if processAlive(pid, processStopBudget) {
		t.Fatalf("the plugin at pid %d is still running after the stop", pid)
	}
}

// TestStopKillsAPluginThatIgnoresTheClosedConnection holds the bound on the
// stop: a plugin that goes on running once its connection is closed is killed
// when the exit grace runs out, so a hung plugin cannot hold a shutdown.
//
// VALIDATES: a plugin that never exits is gone, and the process goroutines
// are finished, within pluginExitGrace plus the kernel's reaping.
// PREVENTS: a stop that waits for the plugin's own exit with no bound, which
// would leave a hung plugin running past the daemon that started it.
//
// DISCRIMINATES: drop the group.kill() after the grace timer in superviseExit and the
// plugin sleeps on, so the Wait below times out.
func TestStopKillsAPluginThatIgnoresTheClosedConnection(t *testing.T) {
	proc, pid := startExitGraceChild(t, exitChildIgnores)
	proc.Stop()
	waitStopped(t, proc, pluginExitGrace+processStopBudget)

	if processAlive(pid, processStopBudget) {
		t.Fatalf("the plugin at pid %d ignored its closed connection and is still running after the exit grace", pid)
	}
	if state := proc.Cmd().ProcessState; state == nil || state.Exited() {
		t.Fatalf("the plugin was not killed: %v", state)
	}
}

// TestStopReachesWhatAPluginThatExitedOnItsOwnLeftBehind holds what the stop
// owes the descendants of a plugin that exited on its own once its connection
// closed: the same group signal a plugin that had to be killed takes them
// with. A plugin's descendants share its process group, and nothing else
// stops them once the plugin is gone.
//
// VALIDATES: a process the plugin started, and left running in its group, is
// gone after the stop of a plugin that exited 0 on its closed connection.
// PREVENTS: a stop that reaps the exited plugin without signaling its group,
// which leaves that descendant running after the daemon stopped the plugin.
//
// DISCRIMINATES: in monitorCmd, record the exit as group.exit(false) and the
// descendant sleeps on.
func TestStopReachesWhatAPluginThatExitedOnItsOwnLeftBehind(t *testing.T) {
	descendantPath := filepath.Join(t.TempDir(), "descendant.pid")
	t.Setenv(exitChildDescendantPidEnv, descendantPath)

	proc, _ := startExitGraceChild(t, exitChildLeavesDescendant)
	descendant := waitForPluginPid(t, descendantPath)
	proc.Stop()
	waitStopped(t, proc, pluginExitGrace+processStopBudget)

	// The branch under test runs only for a plugin that exited on its own; a
	// plugin killed by the grace takes its group with it on another path.
	if state := proc.Cmd().ProcessState; state == nil || !state.Exited() || state.ExitCode() != 0 {
		t.Fatalf("the plugin did not exit on its own when its connection closed: %v", state)
	}
	if processRunning(descendant, processStopBudget) {
		t.Fatalf("the process the plugin started, pid %d, is still running after the stop", descendant)
	}
}

// processRunning answers whether pid is still running, waiting up to budget for
// it to stop. A zombie counts as stopped: a killed descendant is reparented to
// whichever process reaps orphans here, and processAlive would see it until
// that process gets to it.
func processRunning(pid int, budget time.Duration) bool {
	deadline := time.Now().Add(budget)
	for {
		stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
		if err != nil {
			return false
		}
		// The state is the first field after the parenthesized command name.
		if end := strings.LastIndexByte(string(stat), ')'); end > 0 && strings.HasPrefix(string(stat[end:]), ") Z") {
			return false
		}
		if time.Now().After(deadline) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestMain turns the test binary into the fake plugin the three tests above
// start, when the parent test set exitChildModeEnv: it then runs no test and
// exits the process the way a plugin's main does. The dispatch lives here, not
// in a Test function, because a Test that only re-enters the binary asserts
// nothing and the parent's assertions are the check.
func TestMain(m *testing.M) {
	if mode := os.Getenv(exitChildModeEnv); mode != "" {
		os.Exit(runExitGraceChild(mode))
	}
	os.Exit(m.Run())
}

// runExitGraceChild is the fake plugin's main. It answers the exit code.
func runExitGraceChild(mode string) int {
	if mode == exitChildDescendant {
		time.Sleep(time.Minute) // sleep(load-model): a descendant that runs until its group is killed
		return 3
	}
	if err := os.WriteFile(os.Getenv(exitChildPidEnv), []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		return 2
	}

	// The relay crashlog.Init installs in every ze process: os.Stderr becomes a
	// pipe, and a goroutine copies it to the real stderr. Here it also lags.
	realStderr := os.Stderr
	read, write, err := os.Pipe()
	if err != nil {
		return 2
	}
	os.Stderr = write
	relayDone := make(chan struct{})
	go func() {
		defer close(relayDone)
		time.Sleep(exitChildRelayLag)    // sleep(load-model): stands in for a relay a loaded machine has not scheduled yet
		_, _ = io.Copy(realStderr, read) //nolint:errcheck // the parent's assertion is the check
	}()

	conn, err := dialExitGraceHub()
	if err != nil {
		return 2
	}
	if mode == exitChildLeavesDescendant {
		if startErr := startExitGraceDescendant(); startErr != nil {
			return 2
		}
	}
	_, _ = os.Stderr.WriteString(exitChildLastLine + "\n") //nolint:errcheck // the parent's assertion is the check

	if mode == exitChildIgnores {
		time.Sleep(time.Minute) // sleep(load-model): a plugin that never notices its connection closed
		return 3
	}

	// The SDK's Run returns when the engine closes the connection; this is that read.
	_, _ = io.Copy(io.Discard, conn) //nolint:errcheck // EOF or a reset both mean the engine closed it
	// The flush a plugin's main runs on its way out (flushCrashlog).
	_ = write.Close() //nolint:errcheck // the relay sees EOF either way
	<-relayDone
	return 0
}

// startExitGraceDescendant starts the test binary again as a process the fake
// plugin leaves behind in its own process group, and writes its pid where the
// parent test reads it. Its output goes to /dev/null, so it holds no pipe of
// the plugin's open.
func startExitGraceDescendant() error {
	// #nosec G204 - the test binary re-running itself
	descendant := exec.Command(os.Args[0], "-test.run=^$") //nolint:noctx // it must outlive the plugin, which is the point
	descendant.Env = append(os.Environ(), exitChildModeEnv+"="+exitChildDescendant)
	if err := descendant.Start(); err != nil {
		return err
	}
	return os.WriteFile(os.Getenv(exitChildDescendantPidEnv), []byte(strconv.Itoa(descendant.Process.Pid)), 0o600)
}

// dialExitGraceHub is the plugin half of the hub connect-back, from the env the
// daemon's start gives every external plugin: a TLS dial checked against the
// daemon's root, then the auth line. The SDK's own dial is not reachable from
// this package's tests (it imports this package).
func dialExitGraceHub() (net.Conn, error) {
	tlsConf, err := ipc.TLSConfigWithRoot([]byte(os.Getenv("ZE_PLUGIN_CA_PEM")))
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), pluginStartBudget)
	defer cancel()
	addr := net.JoinHostPort(os.Getenv("ZE_PLUGIN_HUB_HOST"), os.Getenv("ZE_PLUGIN_HUB_PORT"))
	conn, err := (&tls.Dialer{Config: tlsConf}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	if authErr := ipc.SendAuth(ctx, conn, os.Getenv("ZE_PLUGIN_HUB_TOKEN"), os.Getenv("ZE_PLUGIN_NAME")); authErr != nil {
		conn.Close() //nolint:errcheck,gosec // the child exits on this path
		return nil, authErr
	}
	return conn, nil
}

// startExitGraceChild starts the test binary as an external plugin in mode,
// through the daemon's own start and a TLS hub acceptor, and answers the
// process and the plugin's pid once the plugin has connected back.
func startExitGraceChild(t *testing.T, mode string) (*Process, int) {
	t.Helper()

	acceptor := startTestHubAcceptor(t)
	pidPath := filepath.Join(t.TempDir(), "plugin.pid")
	t.Setenv(exitChildModeEnv, mode)
	t.Setenv(exitChildPidEnv, pidPath)
	// A race-enabled binary sleeps atexit_sleep_ms (1s by default) after main
	// returns, which is as long as the exit grace, so a -race run of this test
	// would see the fake plugin killed after a clean exit it had already made.
	// The variable is read at a binary's start, so it reaches the child alone.
	t.Setenv("GORACE", "atexit_sleep_ms=0")

	run := "exec '" + os.Args[0] + "' -test.run='^$'"
	proc := NewProcess(plugin.PluginConfig{Name: "exit-grace-plugin", Run: run, Encoder: "json"})
	proc.SetAcceptor(acceptor)

	if err := proc.StartWithContext(t.Context()); err != nil {
		t.Fatalf("start the fake plugin: %v", err)
	}
	pid := waitForPluginPid(t, pidPath)
	t.Cleanup(func() {
		// The test stops what the product failed to stop. The signal fails
		// when the product did stop it, which is the passing case.
		_ = plugin.KillProcessGroup(pid) //nolint:errcheck // the group is already gone when the product stopped it
	})
	return proc, pid
}

// waitStopped joins the process goroutines after Stop and fails the test when
// they do not finish within budget.
func waitStopped(t *testing.T, proc *Process, budget time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	if err := proc.Wait(ctx); err != nil {
		t.Fatalf("the process goroutines did not finish within %s of the stop: %v", budget, err)
	}
}

// startTestHubAcceptor starts a hub acceptor serving a self-signed leaf for
// 127.0.0.1, and hands the same certificate to the plugin as the root it checks
// the hub against, which is all a real plugin's TLS connect-back needs. The
// pki package builds the daemon's own authority, but it is not reachable from
// this package's tests (it imports this package).
func startTestHubAcceptor(t *testing.T) *ipc.PluginAcceptor {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate the hub key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1)},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("self-sign the hub certificate: %v", err)
	}
	leaf := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	rootPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	listeners, err := ipc.StartListeners([]string{"127.0.0.1:0"}, func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		return &leaf, nil
	})
	if err != nil {
		t.Fatalf("start the hub listener: %v", err)
	}
	acceptor := ipc.NewPluginAcceptor(listeners[0], "a-plugin-secret-of-at-least-32-chars", rootPEM)
	acceptor.Start()
	t.Cleanup(acceptor.Stop)
	return acceptor
}
