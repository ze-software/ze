package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const pluginShellExtra2PollDelay = 100 * time.Millisecond

var pluginShellExtra2SSHAddress = regexp.MustCompile(`127\.0\.0\.1:(\d+)`)

func init() {
	Register("plugin/authz-recovery-admin", pluginShellExtra2AuthzRecoveryAdmin)
	Register("plugin/bgp-capture-replay-wait-update", pluginShellExtra2WaitCapture(`"type":"message"`, "capture-has-messages", "no message event reached the capture file"))
	Register("plugin/bgp-capture-replay-wait-closed", pluginShellExtra2WaitCapture("capture-stop", "capture-closed", "capture was not closed cleanly on shutdown"))
	Register("plugin/bgp-capture-replay-stdin", pluginShellExtra2ReplayStdin)
	Register("plugin/cli-credential-resolution", pluginShellExtra2CredentialResolution)
	Register("plugin/config-edit-no-daemon", pluginShellExtra2ConfigEditNoDaemon)
	Register("plugin/config-plaintext-password-at-boot", pluginShellExtra2PlaintextPasswordAtBoot)
	Register("plugin/config-validate-agrees-with-boot", pluginShellExtra2ValidateBootAgreement)
	Register("plugin/debug-toggle", pluginShellExtra2DebugToggle)
	Register("plugin/dynamic-group-static-peer-wins-trigger", pluginShellExtra2StopTrigger(3*time.Second))
	Register("plugin/dynamic-peer-applies-group-filters-trigger", pluginShellExtra2StopTrigger(5*time.Second))
	Register("plugin/dynamic-peer-gets-group-role-capability-trigger", pluginShellExtra2StopTrigger(5*time.Second))
}

type pluginShellExtra2Result struct {
	stdout string
	stderr string
	err    error
}

func pluginShellExtra2Env(overrides map[string]string) []string {
	base := os.Environ()
	if len(overrides) == 0 {
		return base
	}
	result := make([]string, 0, len(base)+len(overrides))
	for _, entry := range base {
		key, _, _ := strings.Cut(entry, "=")
		if _, replaced := overrides[key]; !replaced {
			result = append(result, entry)
		}
	}
	for key, value := range overrides {
		result = append(result, key+"="+value)
	}
	return result
}

func pluginShellExtra2Run(ctx context.Context, env map[string]string, stdin string, args ...string) pluginShellExtra2Result {
	command := exec.CommandContext(ctx, "ze", args...) //nolint:gosec // the fixture chooses the arguments
	command.Env = pluginShellExtra2Env(env)
	if stdin != "" {
		command.Stdin = strings.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	return pluginShellExtra2Result{stdout: stdout.String(), stderr: stderr.String(), err: err}
}

func pluginShellExtra2RunCombined(ctx context.Context, env map[string]string, args ...string) pluginShellExtra2Result {
	command := exec.CommandContext(ctx, "ze", args...) //nolint:gosec // the fixture chooses the arguments
	command.Env = pluginShellExtra2Env(env)
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	err := command.Run()
	return pluginShellExtra2Result{stdout: output.String(), err: err}
}

func pluginShellExtra2RequireNoArgs(args []string, name string) error {
	if len(args) != 0 {
		return fmt.Errorf("%s takes no arguments", name)
	}
	return nil
}

func pluginShellExtra2WriteReady() error {
	return os.WriteFile("daemon.ready", nil, 0o600)
}

type pluginShellExtra2Daemon struct {
	command *exec.Cmd
	done    chan error
	log     *os.File
}

func pluginShellExtra2StartDaemon(ctx context.Context, config, logPath string, env map[string]string) (*pluginShellExtra2Daemon, error) {
	logFile, err := os.Create(logPath) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, "ze", "start", config) //nolint:gosec // the fixture chooses the program and its arguments
	command.Env = pluginShellExtra2Env(env)
	command.Stdout = os.Stdout
	command.Stderr = logFile
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		_ = logFile.Close()
		return nil, err
	}
	daemon := &pluginShellExtra2Daemon{command: command, done: make(chan error, 1), log: logFile}
	go func() { daemon.done <- command.Wait() }()
	return daemon, nil
}

func pluginShellExtra2StopDaemon(daemon *pluginShellExtra2Daemon) {
	if daemon == nil {
		return
	}
	select {
	case <-daemon.done:
		_ = daemon.log.Close()
		return
	default:
	}
	_ = syscall.Kill(-daemon.command.Process.Pid, syscall.SIGTERM)
	select {
	case <-daemon.done:
	case <-time.After(3 * time.Second):
		_ = syscall.Kill(-daemon.command.Process.Pid, syscall.SIGKILL)
		<-daemon.done
	}
	_ = daemon.log.Close()
}

func pluginShellExtra2Log(path string) string {
	contents, _ := os.ReadFile(path) //nolint:gosec // the path is the fixture's own scratch file
	return string(contents)
}

func pluginShellExtra2WaitSSHPort(ctx context.Context, logPath string) (string, error) {
	var port string
	if !Poll(ctx, 50, 200*time.Millisecond, func() bool {
		match := pluginShellExtra2SSHAddress.FindStringSubmatch(pluginShellExtra2Log(logPath))
		if len(match) != 2 {
			return false
		}
		port = match[1]
		return true
	}) {
		return "", errors.New("SSH server did not start (no address in daemon.log)")
	}
	return port, nil
}

func pluginShellExtra2CommandFailure(prefix string, result pluginShellExtra2Result) error {
	return fmt.Errorf("%s: %w\nOUTPUT: %s%s", prefix, result.err, result.stdout, result.stderr)
}

func pluginShellExtra2AuthzRecoveryAdmin(ctx context.Context, args []string) error {
	if err := pluginShellExtra2RequireNoArgs(args, "authz recovery fixture"); err != nil {
		return err
	}
	if err := pluginShellExtra2WriteReady(); err != nil {
		return err
	}
	shared, err := os.MkdirTemp("", "authz-recovery-admin-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(shared) //nolint:errcheck // fixture cleanup

	initResult := pluginShellExtra2Run(ctx, map[string]string{envConfigDir: shared}, "admin\nrecoverypass\n127.0.0.1\n2222\n\n", "init")
	if initResult.err != nil {
		return pluginShellExtra2CommandFailure("ze init failed", initResult)
	}
	_, _ = os.Stderr.WriteString(initResult.stderr)
	daemon, err := pluginShellExtra2StartDaemon(ctx, "authz-recovery-admin.conf", "daemon.log", map[string]string{
		envConfigDir:   shared,
		"ze_log_authz": logLevelInfo,
	})
	if err != nil {
		return err
	}
	defer pluginShellExtra2StopDaemon(daemon)

	port, err := pluginShellExtra2WaitSSHPort(ctx, "daemon.log")
	if err != nil {
		return fmt.Errorf("%w\n%s", err, pluginShellExtra2Log("daemon.log"))
	}
	fmt.Fprintf(os.Stderr, "ssh on :%s\n", port)
	cliResult := pluginShellExtra2RunCombined(ctx, map[string]string{
		envConfigDir:   shared,
		envSSHHost:     addrLoopback,
		envSSHPort:     port,
		envSSHPassword: "recoverypass",
	}, "cli", "--user", "admin", "-c", "show version")
	if cliResult.err != nil {
		return fmt.Errorf("bootstrap admin must reach the box via the recovery profile: %w\nOUTPUT: %s%s\n%s", cliResult.err, cliResult.stdout, cliResult.stderr, pluginShellExtra2Log("daemon.log"))
	}
	fmt.Fprintf(os.Stderr, "OK: recovery admin allowed 'show version': %s\n", strings.TrimRight(cliResult.stdout, "\n"))
	if !strings.Contains(pluginShellExtra2Log("daemon.log"), "break-glass recovery admin") {
		return fmt.Errorf("daemon did not log the break-glass recovery decision\n%s", pluginShellExtra2Log("daemon.log"))
	}
	fmt.Fprintln(os.Stderr, "OK: daemon logged break-glass recovery admin decision")
	return nil
}

func pluginShellExtra2WaitCapture(token, success, failure string) Driver {
	return func(ctx context.Context, args []string) error {
		if err := pluginShellExtra2RequireNoArgs(args, "capture barrier"); err != nil {
			return err
		}
		path := filepath.Join("capture", "bgp-127.0.0.1.jsonl")
		if !Poll(ctx, 151, pluginShellExtra2PollDelay, func() bool {
			contents, err := os.ReadFile(path) //nolint:gosec // the path is the fixture's own scratch file
			return err == nil && bytes.Contains(contents, []byte(token))
		}) {
			entries, _ := os.ReadDir("capture")
			names := make([]string, 0, len(entries))
			for _, entry := range entries {
				names = append(names, entry.Name())
			}
			return fmt.Errorf("%s; capture entries: %s", failure, strings.Join(names, ", "))
		}
		fmt.Fprintln(os.Stdout, success) //nolint:errcheck // progress output
		return nil
	}
}

func pluginShellExtra2ReplayStdin(ctx context.Context, args []string) error {
	if err := pluginShellExtra2RequireNoArgs(args, "capture stdin replay fixture"); err != nil {
		return err
	}
	capture, err := os.ReadFile(filepath.Join("capture", "bgp-127.0.0.1.jsonl"))
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, executable, "replay", "-") //nolint:gosec // the fixture chooses the program and its arguments
	command.Stdin = bytes.NewReader(capture)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err = command.Run()
	if writeErr := os.WriteFile("stdin-replay.out", stdout.Bytes(), 0o600); writeErr != nil {
		return writeErr
	}
	if writeErr := os.WriteFile("stdin-replay.err", stderr.Bytes(), 0o600); writeErr != nil {
		return writeErr
	}
	if err != nil {
		return fmt.Errorf("replay from stdin failed: %w\n%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "announce=[10.0.0.0/24]") {
		return fmt.Errorf("replay from stdin lost the announced prefix\n%s", stdout.String())
	}
	if !regexp.MustCompile(`(?m)^capture {3}-$`).MatchString(stdout.String()) {
		return fmt.Errorf("replay report does not name stdin as its capture\n%s", stdout.String())
	}
	fmt.Fprintln(os.Stdout, "stdin-replay-ok") //nolint:errcheck // progress output
	return nil
}

func pluginShellExtra2CredentialResolution(ctx context.Context, args []string) error {
	if err := pluginShellExtra2RequireNoArgs(args, "CLI credential resolution fixture"); err != nil {
		return err
	}
	if err := pluginShellExtra2WriteReady(); err != nil {
		return err
	}
	daemon, err := pluginShellExtra2StartDaemon(ctx, "cli-credential-resolution.conf", "daemon.log", map[string]string{
		envTestBGPPort: strconv.Itoa(10000 + os.Getpid()%50000),
	})
	if err != nil {
		return err
	}
	defer pluginShellExtra2StopDaemon(daemon)
	port, err := pluginShellExtra2WaitSSHPort(ctx, "daemon.log")
	if err != nil {
		return fmt.Errorf("%w\n%s", err, pluginShellExtra2Log("daemon.log"))
	}
	fmt.Fprintf(os.Stderr, "SSH port: %s\n", port)
	if err := pluginShellExtra2Sleep(ctx, 500*time.Millisecond); err != nil {
		return err
	}
	emptyDir, err := os.MkdirTemp("", "cli-credential-resolution-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(emptyDir) //nolint:errcheck // fixture cleanup
	if _, err := os.Stat(filepath.Join(emptyDir, "database.zefs")); !errors.Is(err, os.ErrNotExist) {
		return errors.New("test setup wrong: the store must not exist")
	}
	before, _ := os.Stat("daemon.log")
	common := map[string]string{
		envConfigDir: emptyDir,
		envSSHHost:   addrLoopback,
		envSSHPort:   port,
	}
	loginEnv := map[string]string{
		envConfigDir:   common["ZE_CONFIG_DIR"],
		envSSHHost:     common["ZE_SSH_HOST"],
		envSSHPort:     common["ZE_SSH_PORT"],
		envSSHPassword: valueTestPassword,
	}
	login := pluginShellExtra2RunCombined(ctx, loginEnv, "cli", "--user", "alice", "-c", "show bgp peer list")
	if login.err != nil {
		return fmt.Errorf("ze cli --user alice must log in with no credential store: %w\nOUTPUT: %s%s\n%s", login.err, login.stdout, login.stderr, pluginShellExtra2Log("daemon.log"))
	}
	log := pluginShellExtra2Log("daemon.log")
	start := int64(0)
	if before != nil {
		start = before.Size()
	}
	if start > int64(len(log)) {
		start = int64(len(log))
	}
	newLog := log[start:]
	if !regexp.MustCompile(`SSH auth success.*username=alice`).MatchString(newLog) {
		return fmt.Errorf("daemon did not log auth success for username=alice\nNEW LOG: %s", newLog)
	}
	fmt.Fprintln(os.Stderr, "OK: AC-6 -- alice logged in with no credential store")

	missing := pluginShellExtra2RunCombined(ctx, common, "cli", "-c", "show bgp peer list")
	if missing.err == nil {
		return fmt.Errorf("ze cli with no store and no --user must not succeed\nOUTPUT: %s%s", missing.stdout, missing.stderr)
	}
	missingOutput := missing.stdout + missing.stderr
	if !strings.Contains(missingOutput, "--user") {
		return fmt.Errorf("error must name --user as the way forward\nOUTPUT: %s", missingOutput)
	}
	if !strings.Contains(missingOutput, "ze.ssh.password") {
		return fmt.Errorf("error must name ze.ssh.password as the way forward\nOUTPUT: %s", missingOutput)
	}
	fmt.Fprintln(os.Stderr, "OK: AC-7 -- no credentials gives an actionable error")
	fmt.Fprintln(os.Stderr, "OK: all credential-resolution tests passed")
	return nil
}

func pluginShellExtra2ConfigEditNoDaemon(ctx context.Context, args []string) error {
	if err := pluginShellExtra2RequireNoArgs(args, "config edit bootstrap fixture"); err != nil {
		return err
	}
	if err := pluginShellExtra2WriteReady(); err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "config-edit-bootstrap-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir) //nolint:errcheck // fixture cleanup
	result := pluginShellExtra2Run(ctx, map[string]string{envConfigDir: dir}, "admin\nsecret123\n127.0.0.1\n2222\n", "init")
	_, _ = os.Stdout.WriteString(result.stdout)
	_, _ = os.Stderr.WriteString(result.stderr)
	if result.err != nil {
		return fmt.Errorf("ze init: %w", result.err)
	}
	if _, err := os.Stat(filepath.Join(dir, "database.zefs")); err != nil {
		return errors.New("database.zefs not created")
	}
	fmt.Fprintln(os.Stderr, "OK: ze init created database for editor bootstrap")
	return nil
}

func pluginShellExtra2PlaintextPasswordAtBoot(ctx context.Context, args []string) error {
	if err := pluginShellExtra2RequireNoArgs(args, "plaintext password boot fixture"); err != nil {
		return err
	}
	daemon, err := pluginShellExtra2StartDaemon(ctx, "plaintext-at-boot.conf", "daemon.log", map[string]string{
		envTestBGPPort: strconv.Itoa(10000 + os.Getpid()%50000),
	})
	if err != nil {
		return err
	}
	defer pluginShellExtra2StopDaemon(daemon)
	port, err := pluginShellExtra2WaitSSHPort(ctx, "daemon.log")
	if err != nil {
		return fmt.Errorf("%w\n%s", err, pluginShellExtra2Log("daemon.log"))
	}
	if err := pluginShellExtra2Sleep(ctx, 500*time.Millisecond); err != nil {
		return err
	}
	emptyDir, err := os.MkdirTemp("", "plaintext-password-cli-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(emptyDir) //nolint:errcheck // fixture cleanup
	login := pluginShellExtra2RunCombined(ctx, map[string]string{
		envConfigDir:   emptyDir,
		envSSHHost:     addrLoopback,
		envSSHPort:     port,
		envSSHPassword: "labsecret",
	}, "cli", "--user", "lab", "-c", "show bgp peer list")
	if login.err != nil {
		return fmt.Errorf("plaintext-password in a config file must authenticate: %w\nOUTPUT: %s%s\n%s", login.err, login.stdout, login.stderr, pluginShellExtra2Log("daemon.log"))
	}
	log := pluginShellExtra2Log("daemon.log")
	if !regexp.MustCompile(`SSH auth success.*username=lab`).MatchString(log) {
		return fmt.Errorf("daemon did not log auth success for username=lab\n%s", log)
	}
	fmt.Fprintln(os.Stderr, "OK: AC-1 -- the config file's plaintext-password logged lab in")
	if !strings.Contains(log, "plaintext-at-boot.conf") {
		return fmt.Errorf("the daemon must warn, naming the config file that still holds the plaintext\n%s", log)
	}
	if strings.Contains(log, "labsecret") {
		return errors.New("the secret reached the log")
	}
	fmt.Fprintln(os.Stderr, "OK: AC-3 -- one warning names the file and carries no secret")
	config, err := os.ReadFile("plaintext-at-boot.conf")
	if err != nil {
		return err
	}
	if !bytes.Contains(config, []byte(`plaintext-password "labsecret"`)) {
		return errors.New("the load path rewrote the operator's config file")
	}
	fmt.Fprintln(os.Stderr, "OK: the operator's file is untouched")
	return nil
}

func pluginShellExtra2ValidateBootAgreement(ctx context.Context, args []string) error {
	if err := pluginShellExtra2RequireNoArgs(args, "validate and boot agreement fixture"); err != nil {
		return err
	}
	if err := os.MkdirAll("shapes", 0o750); err != nil {
		return err
	}
	tests := []struct {
		name string
		want string
	}{
		{name: "plugin_internal", want: verdictValid},
		{name: "plugin_two", want: verdictValid},
		{name: "legacy_env", want: verdictInvalid},
		{name: "unknown_internal", want: verdictInvalid},
	}
	failed := false
	for _, test := range tests {
		config := filepath.Join("shapes", test.name+".conf")
		validateLog := filepath.Join("shapes", test.name+".validate.log")
		bootLog := filepath.Join("shapes", test.name+".boot.log")
		validation := pluginShellExtra2RunCombined(ctx, nil, "config", "validate", config)
		if err := os.WriteFile(validateLog, []byte(validation.stdout+validation.stderr), 0o600); err != nil {
			return err
		}
		valid := validation.err == nil
		if test.want == verdictValid && !valid {
			fmt.Fprintf(os.Stderr, "FAIL[%s]: expected the validator to accept this shape\n%s%s", test.name, validation.stdout, validation.stderr)
			failed = true
			continue
		}
		if test.want == verdictInvalid && valid {
			fmt.Fprintf(os.Stderr, "FAIL[%s]: expected the validator to reject this shape\n", test.name)
			failed = true
			continue
		}
		booted, err := pluginShellExtra2BootConfig(ctx, config, bootLog)
		if err != nil {
			return err
		}
		if test.want == verdictValid && !booted {
			fmt.Fprintf(os.Stderr, "FAIL[%s]: validate accepted it, boot refused it\n--- boot log ---\n%s", test.name, pluginShellExtra2Log(bootLog))
			failed = true
			continue
		}
		if test.want == verdictInvalid && booted {
			fmt.Fprintf(os.Stderr, "FAIL[%s]: validate rejected it, boot accepted it\n", test.name)
			failed = true
			continue
		}
		bootVerdict := "no"
		if booted {
			bootVerdict = valueYes
		}
		fmt.Fprintf(os.Stderr, "OK[%s]: validate=%s boot=%s\n", test.name, test.want, bootVerdict)
	}
	if strings.Contains(pluginShellExtra2Log(filepath.Join("shapes", "plugin_internal.boot.log")), "expected 'external'") {
		fmt.Fprintln(os.Stderr, "FAIL: the deleted hub parser is back on the boot path")
		failed = true
	}
	if failed {
		return errors.New("validate and boot verdicts disagreed")
	}
	return nil
}

func pluginShellExtra2BootConfig(ctx context.Context, configPath, logPath string) (bool, error) {
	config, err := os.Open(configPath) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return false, err
	}
	defer config.Close()               //nolint:errcheck // read-only fixture input
	logFile, err := os.Create(logPath) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return false, err
	}
	command := exec.CommandContext(ctx, "ze", "-")
	command.Stdin = config
	command.Stdout = logFile
	command.Stderr = logFile
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		_ = logFile.Close()
		return false, err
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	finished := false
	booted := Poll(ctx, 100, pluginShellExtra2PollDelay, func() bool {
		log := pluginShellExtra2Log(logPath)
		if strings.Contains(log, "Ze running") || strings.Contains(log, "hub: started") {
			return true
		}
		select {
		case <-done:
			finished = true
			return true
		default:
			return false
		}
	})
	if finished {
		booted = false
	} else {
		log := pluginShellExtra2Log(logPath)
		booted = booted && (strings.Contains(log, "Ze running") || strings.Contains(log, "hub: started"))
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
			<-done
		}
	}
	if err := logFile.Close(); err != nil {
		return false, err
	}
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	return booted, nil
}

func pluginShellExtra2DebugToggle(ctx context.Context, args []string) error {
	if err := pluginShellExtra2RequireNoArgs(args, "debug toggle fixture"); err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "debug-toggle-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir) //nolint:errcheck // fixture cleanup
	env := map[string]string{"ze_debug_store": filepath.Join(dir, "debug.zefs")}
	require := func(want string, argv ...string) error {
		result := pluginShellExtra2Run(ctx, env, "", argv...)
		output := result.stdout + result.stderr
		if result.err != nil || !strings.Contains(output, want) {
			return fmt.Errorf("%s: %w: %s", strings.Join(argv, " "), result.err, output)
		}
		return nil
	}
	if err := require("enabled", "set", "debug", "module", "bgp.reactor"); err != nil {
		return err
	}
	if err := require("bgp.reactor", "show", "debug", "profile", "name", "default"); err != nil {
		return err
	}
	if err := require("disabled", "delete", "debug", "module", "bgp.reactor"); err != nil {
		return err
	}
	if err := require("enabled", "set", "debug", "module", "bgp.reactor"); err != nil {
		return err
	}
	if err := require("saved", "set", "debug", "profile", "name", "myprofile"); err != nil {
		return err
	}
	if err := require("myprofile", "show", "debug", "profile"); err != nil {
		return err
	}
	if err := require("cleared", "clear", "debug"); err != nil {
		return err
	}
	cleared := pluginShellExtra2Run(ctx, env, "", "show", "debug", "profile", "name", "default")
	if cleared.err != nil {
		return pluginShellExtra2CommandFailure("show after clear", cleared)
	}
	if strings.Contains(cleared.stdout+cleared.stderr, "bgp.reactor") {
		return errors.New("show after clear still has module")
	}
	if err := require("timeout", "set", "debug", "timeout", "30m"); err != nil {
		return err
	}
	if result := pluginShellExtra2Run(ctx, env, "", "set", "debug", "timeout", "30"); result.err == nil {
		return errors.New("bare number accepted")
	}
	if result := pluginShellExtra2Run(ctx, env, "", "set", "debug", "module", "has/slash"); result.err == nil {
		return errors.New("slash accepted")
	}
	fmt.Fprintln(os.Stderr, "OK: all verb-first debug tests passed")
	return nil
}

func pluginShellExtra2StopTrigger(delay time.Duration) Driver {
	return func(ctx context.Context, args []string) error {
		if err := pluginShellExtra2RequireNoArgs(args, "daemon stop trigger"); err != nil {
			return err
		}
		for _, path := range []string{fileDaemonPID, fileDaemonReady} {
			if !Poll(ctx, 600, pluginShellExtra2PollDelay, func() bool {
				_, err := os.Stat(path)
				return err == nil
			}) {
				if ctx.Err() != nil {
					return nil //nolint:nilerr // the context is canceled, so the caller is already stopping
				}
				return fmt.Errorf("timed out waiting for %s", path)
			}
		}
		if err := pluginShellExtra2Sleep(ctx, delay); err != nil {
			if ctx.Err() != nil {
				return nil //nolint:nilerr // the context is canceled, so the caller is already stopping
			}
			return err
		}
		pid, err := pluginShellExtra2ReadPID("daemon.pid")
		if err != nil {
			return err
		}
		return syscall.Kill(pid, syscall.SIGTERM)
	}
}

func pluginShellExtra2ReadPID(path string) (int, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid < 1 {
		return 0, fmt.Errorf("invalid pid %q", strings.TrimSpace(string(data)))
	}
	return pid, nil
}

func pluginShellExtra2Sleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
