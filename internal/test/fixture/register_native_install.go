// Design: docs/guide/operations.md -- the installed daemon's native lifecycle.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/test/runner"
)

func init() {
	Register("ui/native-install-lifecycle", nativeInstallLifecycle)
}

// nativeInstallLifecycle crosses the actual command registries, installed
// executable, bootstrap store and authenticated SSH listener. Unit seams cannot
// prove that these independently registered owners work in a shipped binary.
func nativeInstallLifecycle(ctx context.Context, args []string) (lifecycleErr error) {
	started := time.Now()
	fmt.Fprintln(os.Stderr, "native lifecycle +0s begin")
	defer func() {
		code := 0
		if lifecycleErr != nil {
			code = 1
		}
		fmt.Fprintf(os.Stderr, "native lifecycle +%s end exit=%d\n", time.Since(started).Round(time.Millisecond), code)
	}()
	ctx, cancel := context.WithTimeout(ctx, deploymentBudget(90*time.Second))
	defer cancel()
	if len(args) != 1 {
		return errors.New("native install fixture requires the runner's SSH port")
	}
	port, err := strconv.ParseUint(args[0], 10, 16)
	if err != nil {
		return err
	}
	if port == 0 {
		return errors.New("native install fixture requires a nonzero SSH port")
	}
	dir, err := os.MkdirTemp(".", "native-install-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir) //nolint:errcheck // Only this fixture's private installation is removed.
	prefix, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	configDir := filepath.Join(prefix, "etc", "ze")
	binary := filepath.Join(prefix, "bin", "ze")
	runtimeDir := filepath.Join(prefix, "run")
	if err := os.Mkdir(runtimeDir, 0o700); err != nil {
		return err
	}
	input := "native-admin\nnative-test-secret\n127.0.0.1\n" + args[0] + "\nnative-edge\n"
	installed := nativeInstallCommand(ctx, started, nil, input, "ze", "install", "--prefix", prefix, "--no-service")
	if installed.code != 0 {
		return fmt.Errorf("native install failed: %s%s", installed.stdout, installed.stderr)
	}
	configuration := nativeInstallCommand(ctx, started, map[string]string{envConfigDir: configDir}, "", binary, "config", "cat", "native-edge.conf")
	if configuration.code != 0 {
		return fmt.Errorf("installed config unavailable: %s%s", configuration.stdout, configuration.stderr)
	}
	if strings.Contains(configuration.stdout, "interface {") {
		return fmt.Errorf("native install claimed host interfaces: %s", configuration.stdout)
	}
	remote := map[string]string{envConfigDir: configDir, envSSHHost: addrLoopback, envSSHPort: args[0], envSSHPassword: "native-test-secret", "XDG_RUNTIME_DIR": runtimeDir}
	if err := nativeInstalledLogin(ctx, started, binary, remote); err != nil {
		return err
	}
	reinstalled := nativeInstallCommand(ctx, started, nil, "", "ze", "install", "--prefix", prefix, "--no-service")
	if reinstalled.code != 0 {
		return fmt.Errorf("reinstall without bootstrap input failed: %s%s", reinstalled.stdout, reinstalled.stderr)
	}
	after := nativeInstallCommand(ctx, started, map[string]string{envConfigDir: configDir}, "", binary, "config", "cat", "native-edge.conf")
	if after.code != 0 {
		return fmt.Errorf("reinstalled config unavailable: %s%s", after.stdout, after.stderr)
	}
	if after.stdout != configuration.stdout {
		return errors.New("reinstall changed the initial configuration")
	}
	if err := nativeInstalledLogin(ctx, started, binary, remote); err != nil {
		return err
	}
	removed := nativeInstallCommand(ctx, started, nil, "", "ze", "uninstall", "--prefix", prefix, "--no-service", "--yes")
	if removed.code != 0 {
		return fmt.Errorf("native uninstall failed: %s%s", removed.stdout, removed.stderr)
	}
	if _, err := os.Stat(binary); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("uninstall did not remove installed binary: %v", err)
	}
	if _, err := os.Stat(filepath.Join(configDir, "database")); err != nil {
		return fmt.Errorf("ordinary uninstall removed state: %w", err)
	}
	purged := nativeInstallCommand(ctx, started, nil, "", "ze", "uninstall", "--prefix", prefix, "--no-service", "--yes", "--purge")
	if purged.code != 0 {
		return fmt.Errorf("explicit purge failed: %s%s", purged.stdout, purged.stderr)
	}
	if _, err := os.Stat(configDir); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("explicit purge retained config directory: %v", err)
	}
	if _, err := os.Stat(runtimeDir); err != nil {
		return fmt.Errorf("purge removed an unrelated installation directory: %w", err)
	}
	fmt.Fprintln(os.Stderr, "OK: native install boots authenticated SSH, reinstall retains login/config, and uninstall retains state")
	return nil
}

func nativeInstallCommand(ctx context.Context, started time.Time, env map[string]string, stdin string, argv ...string) fixture10ProcessResult {
	// Concurrent real-store initialization exceeded the generic command's
	// 15-second budget under stress. All state assertions stay within this
	// fixture's authored 90-second lifecycle budget and runner contention factor.
	// Only argv is logged: bootstrap input and environment contain credentials.
	commandStarted := time.Now()
	fmt.Fprintf(os.Stderr, "native lifecycle +%s command begin argv=%q\n", time.Since(started).Round(time.Millisecond), argv)
	result := fixture10RunWithin(ctx, deploymentBudget(45*time.Second), env, stdin, argv...)
	fmt.Fprintf(os.Stderr, "native lifecycle +%s command end exit=%d duration=%s argv=%q\n", time.Since(started).Round(time.Millisecond), result.code, time.Since(commandStarted).Round(time.Millisecond), argv)
	return result
}

func nativeInstalledLogin(ctx context.Context, started time.Time, binary string, remote map[string]string) (loginErr error) {
	fmt.Fprintf(os.Stderr, "native lifecycle +%s daemon spawn begin argv=%q\n", time.Since(started).Round(time.Millisecond), []string{binary, "start"})
	command := exec.CommandContext(ctx, binary, "start") //nolint:gosec // The installed executable belongs to this fixture.
	command.Env = fixture10Environment(map[string]string{envConfigDir: remote[envConfigDir], "XDG_RUNTIME_DIR": remote["XDG_RUNTIME_DIR"]})
	var output lockedBuffer
	command.Stdout = io.MultiWriter(os.Stderr, &output)
	command.Stderr = io.MultiWriter(os.Stderr, &output)
	process, err := runner.StartServerProcess(command)
	if err != nil {
		fmt.Fprintf(os.Stderr, "native lifecycle +%s daemon spawn end exit=1 error=%v\n", time.Since(started).Round(time.Millisecond), err)
		return err
	}
	// Stop MUST join this process owner before the installation directory or
	// runner port lease is released. Readiness observers never reap the child.
	defer func() {
		fmt.Fprintf(os.Stderr, "native lifecycle +%s daemon stop begin pid=%d\n", time.Since(started).Round(time.Millisecond), command.Process.Pid)
		if err := process.Stop(syscall.SIGTERM, deploymentBudget(fixture10ShutdownTimeout), 0); err != nil {
			loginErr = errors.Join(loginErr, fmt.Errorf("stop installed daemon: %w\ndaemon stdout/stderr:\n%s", err, output.String()))
		}
		fmt.Fprintf(os.Stderr, "native lifecycle +%s daemon stop end pid=%d\n", time.Since(started).Round(time.Millisecond), command.Process.Pid)
	}()
	fmt.Fprintf(os.Stderr, "native lifecycle +%s daemon spawn end exit=0 pid=%d\n", time.Since(started).Round(time.Millisecond), command.Process.Pid)
	last, err := nativeWaitForLogin(ctx, process.Done(), process.Wait, func(probeCtx context.Context) fixture10ProcessResult {
		return nativeInstallCommand(probeCtx, started, remote, "", binary, "cli", "--user", "native-admin", "-c", "show version")
	}, 100*time.Millisecond)
	if err != nil {
		return nativeLoginFailure(&output, last, err)
	}
	prefix := filepath.Dir(filepath.Dir(binary))
	busy := nativeInstallCommand(ctx, started, nil, "", "ze", "uninstall", "--prefix", prefix, "--no-service", "--yes")
	if busy.code == 0 {
		return errors.New("uninstall removed a live unmanaged daemon")
	}
	if _, err := os.Stat(binary); err != nil {
		return fmt.Errorf("busy refusal removed installed binary: %w", err)
	}
	return nil
}

// nativeWaitForLogin waits for actual authenticated state within the caller's
// lifecycle context. It MUST cancel and join its one exit watcher before return.
// done and wait MUST observe the same process owner; its caller MUST defer Stop.
func nativeWaitForLogin(ctx context.Context, done <-chan struct{}, wait func() error, probe func(context.Context) fixture10ProcessResult, delay time.Duration) (fixture10ProcessResult, error) {
	last := fixture10ProcessResult{code: -1}
	select {
	case <-done:
		return last, nativeDaemonExit(wait)
	default:
	}
	waitCtx, cancel := context.WithCancelCause(ctx)
	joined := make(chan struct{})
	go nativeWatchDaemonExit(waitCtx, done, wait, cancel, joined)
	defer func() {
		cancel(nil)
		<-joined
	}()
	for {
		if cause := context.Cause(waitCtx); cause != nil {
			return last, cause
		}
		last = probe(waitCtx)
		if cause := context.Cause(waitCtx); cause != nil {
			return last, cause
		}
		if last.code == 0 {
			return last, nil
		}
		timer := time.NewTimer(delay)
		select {
		case <-waitCtx.Done():
			timer.Stop()
			return last, context.Cause(waitCtx)
		case <-timer.C:
		}
	}
}

// nativeWatchDaemonExit MUST retain the Wait owner's result and close joined.
// Canceling the probe context also interrupts an in-flight CLI child.
func nativeWatchDaemonExit(ctx context.Context, done <-chan struct{}, wait func() error, cancel context.CancelCauseFunc, joined chan<- struct{}) {
	defer close(joined)
	select {
	case <-done:
		cancel(nativeDaemonExit(wait))
	case <-ctx.Done():
	}
}

func nativeDaemonExit(wait func() error) error {
	if err := wait(); err != nil {
		return fmt.Errorf("installed daemon exited before authenticated readiness: %w", err)
	}
	return errors.New("installed daemon exited with status 0 before authenticated readiness")
}

func nativeLoginFailure(output *lockedBuffer, last fixture10ProcessResult, cause error) error {
	return fmt.Errorf("installed daemon did not accept bootstrap login: %w\nlast client exit=%d\nclient stdout:\n%s\nclient stderr:\n%s\ndaemon stdout/stderr:\n%s", cause, last.code, last.stdout, last.stderr, output.String())
}
