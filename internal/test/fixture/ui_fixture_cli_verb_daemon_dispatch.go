package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

func init() {
	Register("ui/cli-verb-daemon-dispatch", uiDriver(runCLIVerbDaemonDispatch))
}

type uiCliVerbDaemonDispatchCommandResult struct {
	code int
	out  string
}

type uiCliVerbDaemonDispatchDaemonProcess struct {
	cmd     *exec.Cmd
	stdout  bytes.Buffer
	stderr  bytes.Buffer
	done    chan error
	waited  bool
	waitErr error
}

func runCLIVerbDaemonDispatch(ctx context.Context) (retErr error) {
	workDir, err := os.MkdirTemp("", "ze-cli-verb-daemon-dispatch-")
	if err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	defer os.RemoveAll(workDir) //nolint:errcheck // fixture cleanup

	passwordHash, err := uiCliVerbDaemonDispatchMakePasswordHash(ctx, workDir)
	if err != nil {
		return err
	}

	config := `system {
    authentication {
        user ci {
            password "` + passwordHash + `"
            profile [ admin ]
        }
    }
}
`
	if err := os.WriteFile(filepath.Join(workDir, "verb.conf"), []byte(config), 0o600); err != nil {
		return fmt.Errorf("write verb.conf: %w", err)
	}

	sshAddrPath, err := filepath.Abs(filepath.Join(workDir, "ssh.addr"))
	if err != nil {
		return fmt.Errorf("resolve ssh.addr: %w", err)
	}
	readyPath, err := filepath.Abs(filepath.Join(workDir, "ready"))
	if err != nil {
		return fmt.Errorf("resolve ready file: %w", err)
	}
	configDir, err := filepath.Abs(workDir)
	if err != nil {
		return fmt.Errorf("resolve config directory: %w", err)
	}

	daemonEnv := uiCliVerbDaemonDispatchEnvironment(map[string]string{
		envSSHEphemeral: sshAddrPath,
		envReadyFile:    readyPath,
		envConfigDir:    configDir,
	})
	daemon, err := uiCliVerbDaemonDispatchStartDaemon(ctx, workDir, daemonEnv)
	if err != nil {
		return err
	}
	defer func() {
		if daemon == nil {
			return
		}
		if stopErr := daemon.stop(); retErr == nil && stopErr != nil {
			retErr = stopErr
		}
	}()

	// Both files are startup barriers. The daemon must also remain alive during
	// all 200 checks. The 100 ms interval keeps the original 20 second bound.
	if err := pollDaemonReady(ctx, daemon, sshAddrPath, readyPath); err != nil {
		return err
	}

	addrBytes, err := os.ReadFile(sshAddrPath) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return fmt.Errorf("read ssh.addr: %w", err)
	}
	addr := strings.TrimSpace(string(addrBytes))
	colon := strings.LastIndex(addr, ":")
	if colon < 0 {
		return fmt.Errorf("invalid ssh.addr %q", addr)
	}
	host, port := addr[:colon], addr[colon+1:]
	cliEnv := uiCliVerbDaemonDispatchEnvironment(map[string]string{
		envSSHHost:     host,
		envSSHPort:     port,
		envSSHUsername: "ci",
		envSSHPassword: valueSecret,
		envConfigDir:   configDir,
	})

	run := func(args ...string) (uiCliVerbDaemonDispatchCommandResult, error) {
		return uiCliVerbDaemonDispatchRunZE(ctx, workDir, cliEnv, args...)
	}

	// 1. A daemon-dispatched child beneath a grouping container must resolve
	// through the verb-relative tree and dispatch on its absolute daemon path.
	r, err := run("show", "log", "levels")
	if err != nil {
		return err
	}
	if r.code != 0 {
		return fmt.Errorf("ze show log levels exit=%d: %s", r.code, r.out)
	}
	if !strings.Contains(r.out, "levels") {
		return errors.New(r.out)
	}

	// 2. A complete path rooted beneath another verb must not acquire an extra
	// show prefix.
	r, err = run("show", "system", "subsystem", "list")
	if err != nil {
		return err
	}
	if r.code != 0 {
		return fmt.Errorf("ze show system subsystem list exit=%d: %s", r.code, r.out)
	}
	if !strings.Contains(r.out, "subsystems") {
		return errors.New(r.out)
	}

	// 3. The raw CLI resolver must reach the same daemon command.
	r, err = run("cli", "-c", "show log levels")
	if err != nil {
		return err
	}
	if r.code != 0 {
		return fmt.Errorf("ze cli -c \"show log levels\" exit=%d: %s", r.code, r.out)
	}

	// 4. An undeclared grouping container lists its children and fails.
	r, err = run("show", "bgp", "peer")
	if err != nil {
		return err
	}
	if r.code != 1 {
		return fmt.Errorf("ze show bgp peer exit=%d, want 1: %s", r.code, r.out)
	}
	if !strings.Contains(r.out, "subcommands") {
		return errors.New(r.out)
	}
	if !strings.Contains(r.out, "list") {
		return errors.New(r.out)
	}

	// 5. The bare host alias must return inventory while the daemon is up.
	r, err = run("show", "host")
	if err != nil {
		return err
	}
	if r.code != 0 {
		return fmt.Errorf("ze show host (daemon up) exit=%d, want 0: %s", r.code, r.out)
	}
	if strings.Contains(r.out, "unknown command") {
		return fmt.Errorf("ze show host (daemon up) answered unknown command: %s", r.out)
	}
	if strings.Contains(r.out, "subcommands") {
		return fmt.Errorf("ze show host (daemon up) printed a subcommand list: %s", r.out)
	}
	// This field is present on every supported Unix. Unlike the platform section
	// key, it remains visible when rendering unwraps a single surviving section.
	if !strings.Contains(r.out, "fd-limit-hard-max") {
		return fmt.Errorf("ze show host (daemon up) returned no inventory: %s", r.out)
	}

	// 6. Declaring the shorter alias must not remove the longer argv path.
	r, err = run("show", "host", "all")
	if err != nil {
		return err
	}
	if r.code != 0 {
		return fmt.Errorf("ze show host all (daemon up) exit=%d, want 0: %s", r.code, r.out)
	}
	if !strings.Contains(r.out, "fd-limit-hard-max") {
		return fmt.Errorf("ze show host all (daemon up) returned no inventory: %s", r.out)
	}

	// 7. A positional value tail must not be mistaken for path words.
	r, err = run("show", "log", "recent", "count", "5")
	if err != nil {
		return err
	}
	if r.code != 0 {
		return fmt.Errorf("ze show log recent count 5 exit=%d: %s", r.code, r.out)
	}
	if strings.Contains(r.out, "unknown command") {
		return fmt.Errorf("ze show log recent count 5 answered unknown command: %s", r.out)
	}

	// 8. The named value must reach count's uint32 validator.
	r, err = run("show", "log", "recent", "count", "not-a-number")
	if err != nil {
		return err
	}
	if r.code != 1 {
		return fmt.Errorf("ze show log recent count not-a-number exit=%d, want 1: %s", r.code, r.out)
	}
	if !strings.Contains(r.out, "expected unsigned integer") {
		return fmt.Errorf("the value never reached the count argument: %s", r.out)
	}
	if !strings.Contains(r.out, "not-a-number") {
		return fmt.Errorf("the daemon did not receive the typed value: %s", r.out)
	}

	// 10. A declared command's trailing value must reach the handler that reads
	// it. `show route lookup` states one `ip` leaf whose pattern admits this
	// token, so the daemon binds it and netip.ParseAddr is what refuses it. The
	// address is malformed on purpose, so nothing reaches the FIB.
	//
	// The token used to be `not-an-ip`, from before the leaf existed. The leaf
	// declares a pattern that token fails, so the daemon now refuses it ahead of
	// the handler and check 10b covers that half.
	verb, err := run("show", "route", "lookup", "1.2.3.4.5")
	if err != nil {
		return err
	}
	if verb.code != 1 {
		return fmt.Errorf("ze show route lookup 1.2.3.4.5 exit=%d, want 1: %s", verb.code, verb.out)
	}
	if strings.Contains(verb.out, "unknown command") {
		return fmt.Errorf("a declared value command answered unknown command: %s", verb.out)
	}
	if strings.Contains(verb.out, "usage:") {
		return fmt.Errorf("the value never reached the handler: %s", verb.out)
	}
	if !strings.Contains(verb.out, "invalid destination") {
		return fmt.Errorf("the value did not reach netip.ParseAddr: %s", verb.out)
	}
	if !strings.Contains(verb.out, "1.2.3.4.5") {
		return fmt.Errorf("the daemon did not receive the typed value: %s", verb.out)
	}

	// 10b. A value the declared type refuses is refused by the daemon, and the
	// message says why. The published form is `show route lookup <ip>`, a bare
	// value with no keyword, so a message offering `ip` as a valid keyword names
	// a spelling the grammar never asks for and drops the only fact the operator
	// needs (plan/journal/guard-addition-drops-what-it-refuses.md).
	typed, err := run("show", "route", "lookup", "not-an-ip")
	if err != nil {
		return err
	}
	if typed.code != 1 {
		return fmt.Errorf("ze show route lookup not-an-ip exit=%d, want 1: %s", typed.code, typed.out)
	}
	if strings.Contains(typed.out, "unknown command") {
		return fmt.Errorf("a value the type refuses answered unknown command: %s", typed.out)
	}
	if !strings.Contains(typed.out, "not-an-ip") {
		return fmt.Errorf("the refusal does not name the value: %s", typed.out)
	}
	if !strings.Contains(typed.out, "does not match expected pattern") {
		return fmt.Errorf("the refusal does not say why the value was refused: %s", typed.out)
	}
	if strings.Contains(typed.out, "valid keywords") {
		return fmt.Errorf("a bare value slot was offered as a keyword: %s", typed.out)
	}

	// 11. The verb resolver and the daemon's raw-line resolver must agree. Reuse
	// the verb run from check 10 so this remains one verb launch, as before.
	dashC, err := run("cli", "-c", "show route lookup 1.2.3.4.5")
	if err != nil {
		return err
	}
	if verb.code != dashC.code {
		return fmt.Errorf("exit codes differ: verb=%d -c=%d", verb.code, dashC.code)
	}
	if verb.out != dashC.out {
		return fmt.Errorf("the verb form and `ze cli -c` disagree:\nverb:\n%s\n-c:\n%s", verb.out, dashC.out)
	}

	// 12. Widening value tails must retain unknown-command diagnosis.
	r, err = run("show", "nosuchthing")
	if err != nil {
		return err
	}
	if r.code != 1 {
		return fmt.Errorf("ze show nosuchthing exit=%d, want 1: %s", r.code, r.out)
	}
	if !strings.Contains(r.out, "unknown command") {
		return fmt.Errorf("an unknown command was accepted: %s", r.out)
	}

	// 13. A longer local registration beneath a declared daemon node must retain
	// its own longest-prefix value handling.
	r, err = run("show", "debug", "profile", "name", "default")
	if err != nil {
		return err
	}
	if r.code != 0 {
		return fmt.Errorf("ze show debug profile name default exit=%d, want 0: %s", r.code, r.out)
	}
	if !strings.Contains(r.out, "MODULE") {
		return fmt.Errorf("the local debug-profile handler was not reached: %s", r.out)
	}
	if strings.Contains(r.out, "no credentials") {
		return fmt.Errorf("the local command was sent to the daemon: %s", r.out)
	}

	// 14. A short local registration must not swallow commands declared below
	// it. Expectations come from raw daemon dispatch, not rendered live values.
	interfaceCases := [][]string{
		{argShow, argInterface, "brief"},
		{argShow, argInterface, "scan"},
		{argShow, argInterface, fieldErrors},
		{argShow, argInterface, argType, "no-such-type"},
		{argShow, argInterface, "rate", nameNoSuchInterface},
		{argShow, argInterface, argName, nameNoSuchInterface, "detail"},
		{argShow, argInterface, argName, nameNoSuchInterface, "counters"},
	}
	for _, argv := range interfaceCases {
		line := strings.Join(argv, " ")
		verbResult, runErr := run(argv...)
		if runErr != nil {
			return runErr
		}
		cliResult, runErr := run("cli", "-c", line)
		if runErr != nil {
			return runErr
		}
		if verbResult.code != cliResult.code {
			return fmt.Errorf("ze %s and `ze cli -c \"%s\"` disagree on exit code: verb=%d -c=%d\nverb:\n%s\n-c:\n%s", line, line, verbResult.code, cliResult.code, verbResult.out, cliResult.out)
		}
		if verbResult.out != cliResult.out {
			return fmt.Errorf("ze %s and `ze cli -c \"%s\"` disagree:\nverb:\n%s\n-c:\n%s", line, line, verbResult.out, cliResult.out)
		}
		if strings.Contains(verbResult.out, "ze interface show") {
			return fmt.Errorf("ze %s reached cmdShow and printed its usage: %s", line, verbResult.out)
		}
	}

	if err := daemon.stop(); err != nil {
		return err
	}
	daemon = nil

	// 9. The same bare host argv must use its in-process fallback after the
	// daemon has stopped, rather than being treated as a grouping container.
	offlineEnv := uiCliVerbDaemonDispatchEnvironment(map[string]string{
		envSSHHost:     host,
		envSSHPort:     port,
		envSSHUsername: "ci",
		envSSHPassword: valueSecret,
		envConfigDir:   configDir,
	})
	r, err = uiCliVerbDaemonDispatchRunZE(ctx, workDir, offlineEnv, "show", "host")
	if err != nil {
		return err
	}
	if r.code != 0 {
		return fmt.Errorf("ze show host (daemon down) exit=%d, want 0: %s", r.code, r.out)
	}
	if strings.Contains(r.out, "subcommands") {
		return fmt.Errorf("ze show host printed a subcommand list instead of the inventory: %s", r.out)
	}
	if !strings.Contains(r.out, "fd-limit-hard-max") {
		return fmt.Errorf("ze show host (daemon down) returned no inventory: %s", r.out)
	}

	fmt.Println("OK")
	return nil
}

func uiCliVerbDaemonDispatchMakePasswordHash(ctx context.Context, workDir string) (string, error) {
	cmd := exec.CommandContext(ctx, "ze", "passwd")
	cmd.Dir = workDir
	cmd.Env = os.Environ()
	cmd.Stdin = strings.NewReader("secret\n")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ze passwd: %w: %s", err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

func uiCliVerbDaemonDispatchStartDaemon(ctx context.Context, workDir string, env []string) (*uiCliVerbDaemonDispatchDaemonProcess, error) {
	d := &uiCliVerbDaemonDispatchDaemonProcess{done: make(chan error, 1)}
	d.cmd = exec.CommandContext(ctx, "ze", "-f", "verb.conf")
	d.cmd.Dir = workDir
	d.cmd.Env = env
	d.cmd.Stdout = &d.stdout
	d.cmd.Stderr = &d.stderr
	if err := d.cmd.Start(); err != nil {
		return nil, fmt.Errorf("start daemon: %w", err)
	}
	go func() {
		d.done <- d.cmd.Wait()
	}()
	return d, nil
}

func pollDaemonReady(ctx context.Context, daemon *uiCliVerbDaemonDispatchDaemonProcess, sshAddrPath, readyPath string) error {
	for attempt := range 200 {
		if exited, _ := daemon.pollExit(); exited {
			return fmt.Errorf("daemon exited early\nstdout:\n%s\nstderr:\n%s", daemon.stdout.String(), daemon.stderr.String())
		}
		if uiCliVerbDaemonDispatchPathExists(sshAddrPath) && uiCliVerbDaemonDispatchPathExists(readyPath) {
			return nil
		}
		if attempt == 199 {
			break
		}

		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
	return errors.New("daemon did not become ready")
}

func uiCliVerbDaemonDispatchPathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (d *uiCliVerbDaemonDispatchDaemonProcess) pollExit() (bool, error) {
	if d.waited {
		return true, d.waitErr
	}
	select {
	case err := <-d.done:
		d.waited = true
		d.waitErr = err
		return true, err
	default:
		return false, nil
	}
}

func (d *uiCliVerbDaemonDispatchDaemonProcess) stop() error {
	if d.waited {
		return nil
	}
	if exited, _ := d.pollExit(); exited {
		return nil
	}

	signalErr := d.cmd.Process.Signal(syscall.SIGTERM)
	if d.wait(5 * time.Second) {
		return nil
	}
	if signalErr != nil {
		return fmt.Errorf("terminate daemon: %w", signalErr)
	}
	if err := d.cmd.Process.Kill(); err != nil {
		if exited, _ := d.pollExit(); !exited {
			return fmt.Errorf("kill daemon: %w", err)
		}
		return nil
	}
	if !d.wait(5 * time.Second) {
		return errors.New("daemon did not exit after kill")
	}
	return nil
}

func (d *uiCliVerbDaemonDispatchDaemonProcess) wait(timeout time.Duration) bool {
	if d.waited {
		return true
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-d.done:
		d.waited = true
		d.waitErr = err
		return true
	case <-timer.C:
		return false
	}
}

func uiCliVerbDaemonDispatchRunZE(ctx context.Context, workDir string, env []string, args ...string) (uiCliVerbDaemonDispatchCommandResult, error) {
	cmd := exec.CommandContext(ctx, "ze", args...) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Dir = workDir
	cmd.Env = env
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	out := stdout.String() + stderr.String()
	if err == nil {
		return uiCliVerbDaemonDispatchCommandResult{code: 0, out: out}, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return uiCliVerbDaemonDispatchCommandResult{code: exitErr.ExitCode(), out: out}, nil
	}
	return uiCliVerbDaemonDispatchCommandResult{}, fmt.Errorf("run ze %s: %w", strings.Join(args, " "), err)
}

func uiCliVerbDaemonDispatchEnvironment(updates map[string]string) []string {
	values := make(map[string]string, len(os.Environ())+len(updates))
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = value
		}
	}
	maps.Copy(values, updates)

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, key+"="+values[key])
	}
	return env
}
