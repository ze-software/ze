package fixture

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	sshclient "github.com/ze-software/ze/internal/core/ssh/client"
)

const (
	extra1AdminHash    = `$2a$04$UlwuiuH82Unfsq.XEMPGJeDkXwbm3KW.nvVaVXOd/JeFK8VjMjrQO`
	extra1OperatorHash = `$2a$04$MdupNBXL8iUHgRUDfsFl0ue/.FEH3db8U.LPd/XtMhiBmd4MoRM0m`
)

type extra1Daemon struct {
	command *exec.Cmd
	log     *os.File
	logPath string
	cached  string
	cleanup func()
}

func init() {
	Register("plugin/aaa-radius-admin", extra1RadiusAdmin)
	Register("plugin/aaa-radius-fallback", extra1RadiusFallback)
	Register("plugin/answer-unknown-command", extra1AnswerUnknownCommand)
	Register("plugin/as112-probe-anycast-not-loopback", extra1AS112ProbeAnycast)
	Register("plugin/as112-probe-anycast-not-loopback-probe", extra1AS112UnreachableProbe)
	Register("plugin/audit-auth-fail", extra1AuditAuthFail)
	Register("plugin/audit-persistence", extra1AuditPersistence)
	Register("plugin/authz-allow", extra1AuthzAllow)
	Register("plugin/authz-default", extra1AuthzDefault)
	Register("plugin/authz-deny", extra1AuthzDeny)
	Register("plugin/authz-no-applicable-profile", extra1AuthzNoApplicableProfile)
}

func extra1Write(path, body string) error {
	return os.WriteFile(path, []byte(body), 0o644)
}

func extra1Wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func extra1Environment(overrides map[string]string) []string {
	environment := append([]string(nil), os.Environ()...)
	for key, value := range overrides {
		prefix := key + "="
		for index := len(environment) - 1; index >= 0; index-- {
			if strings.HasPrefix(environment[index], prefix) {
				environment = append(environment[:index], environment[index+1:]...)
			}
		}
		environment = append(environment, prefix+value)
	}
	return environment
}

func extra1StartDaemon(ctx context.Context, configPath, logPath string, environment map[string]string) (*extra1Daemon, error) {
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, "ze", "start", configPath)
	command.Env = extra1Environment(environment)
	command.Stdout = io.Discard
	command.Stderr = logFile
	command.Dir = filepath.Dir(configPath)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		_ = logFile.Close()
		return nil, err
	}
	return &extra1Daemon{command: command, log: logFile, logPath: logPath}, nil
}

func (daemon *extra1Daemon) stop() {
	if daemon == nil {
		return
	}
	if daemon.command != nil && daemon.command.Process != nil {
		_ = syscall.Kill(-daemon.command.Process.Pid, syscall.SIGTERM)
		done := make(chan struct{})
		go func() {
			_ = daemon.command.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = syscall.Kill(-daemon.command.Process.Pid, syscall.SIGKILL)
			<-done
		}
		_ = daemon.log.Sync()
		contents, _ := os.ReadFile(daemon.logPath)
		daemon.cached = string(contents)
		_ = daemon.log.Close()
		daemon.command = nil
	}
	if daemon.cleanup != nil {
		daemon.cleanup()
		daemon.cleanup = nil
	}
}

func (daemon *extra1Daemon) contents() string {
	if daemon == nil {
		return ""
	}
	if daemon.command == nil {
		return daemon.cached
	}
	_ = daemon.log.Sync()
	contents, _ := os.ReadFile(daemon.logPath)
	return string(contents)
}

var extra1SSHAddress = regexp.MustCompile(`127\.0\.0\.1:([0-9]+)`)

func (daemon *extra1Daemon) waitSSH(ctx context.Context) (string, error) {
	var port string
	if Poll(ctx, 50, 200*time.Millisecond, func() bool {
		for _, line := range strings.Split(daemon.contents(), "\n") {
			if !strings.Contains(line, "SSH server listening") {
				continue
			}
			match := extra1SSHAddress.FindStringSubmatch(line)
			if len(match) == 2 {
				port = match[1]
				return true
			}
		}
		return false
	}) {
		return port, nil
	}
	return "", fmt.Errorf("SSH server did not start: %s", daemon.contents())
}

func extra1Credentials(port, username, password string) sshclient.Credentials {
	return sshclient.Credentials{Host: "127.0.0.1", Port: port, Username: username, Auth: password}
}

func extra1Command(port, username, password, command string) (string, error) {
	// le-ci-dispatch: dynamic -- each caller supplies and asserts its own literal command
	return sshclient.ExecCommand(extra1Credentials(port, username, password), command)
}

func extra1InitCLI(ctx context.Context, port string) (string, error) {
	configDir, err := os.MkdirTemp("", "plugin-shell-extra-1-cli-")
	if err != nil {
		return "", err
	}
	command := exec.CommandContext(ctx, "ze", "init")
	command.Env = extra1Environment(map[string]string{"ZE_CONFIG_DIR": configDir})
	command.Stdin = strings.NewReader(fmt.Sprintf("admin\ntestpass\n127.0.0.1\n%s\n", port))
	output, err := command.CombinedOutput()
	if err != nil {
		_ = os.RemoveAll(configDir)
		return "", fmt.Errorf("ze init: %w\n%s", err, output)
	}
	return configDir, nil
}

func extra1CLI(ctx context.Context, configDir, port, username, password, commandText string) (string, error) {
	arguments := []string{"cli", "--remote", net.JoinHostPort("127.0.0.1", port)}
	if username != "" {
		arguments = append(arguments, "--user", username)
	}
	arguments = append(arguments, "-c", commandText)
	command := exec.CommandContext(ctx, "ze", arguments...)
	command.Env = extra1Environment(map[string]string{
		"ZE_CONFIG_DIR":   configDir,
		"ZE_SSH_PASSWORD": password,
	})
	output, err := command.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func extra1TouchReady() error {
	return os.WriteFile("daemon.ready", nil, 0o644)
}

func extra1BGPPort() string {
	return strconv.Itoa(10000 + os.Getpid()%50000)
}

func extra1RunDaemon(ctx context.Context, configPath, logPath, config string, environment map[string]string) (*extra1Daemon, string, error) {
	workDir, err := os.MkdirTemp("", "plugin-shell-extra-1-daemon-")
	if err != nil {
		return nil, "", err
	}
	daemon, port, err := extra1RunDaemonIn(ctx, workDir, configPath, logPath, config, environment)
	if err != nil {
		_ = os.RemoveAll(workDir)
		return nil, "", err
	}
	daemon.cleanup = func() { _ = os.RemoveAll(workDir) }
	return daemon, port, nil
}

func extra1RunDaemonIn(ctx context.Context, workDir, configName, logName, config string, environment map[string]string) (*extra1Daemon, string, error) {
	configPath := filepath.Join(workDir, filepath.Base(configName))
	logPath := filepath.Join(workDir, filepath.Base(logName))
	if err := extra1Write(configPath, config); err != nil {
		return nil, "", err
	}
	if environment == nil {
		environment = make(map[string]string)
	}
	environment["ze_test_bgp_port"] = extra1BGPPort()
	daemon, err := extra1StartDaemon(ctx, configPath, logPath, environment)
	if err != nil {
		return nil, "", err
	}
	port, err := daemon.waitSSH(ctx)
	if err != nil {
		daemon.stop()
		return nil, "", err
	}
	return daemon, port, nil
}

func extra1RequireCommand(port, username, password, command string) (string, error) {
	output, err := extra1Command(port, username, password, command)
	if err != nil {
		return output, fmt.Errorf("%s: %w", command, err)
	}
	return output, nil
}

func extra1ContainsBoth(output, first, second string) bool {
	return strings.Contains(output, first) && strings.Contains(output, second)
}

type extra1RadiusMock struct {
	command *exec.Cmd
	cancel  context.CancelFunc
	work    string
}

func extra1StartRadiusMock(ctx context.Context) (*extra1RadiusMock, string, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, "", err
	}
	work, err := os.MkdirTemp("", "ze-radius-admin-")
	if err != nil {
		return nil, "", err
	}
	logPath := filepath.Join(work, "mock.log")
	addrPath := filepath.Join(work, "mock.addr")
	mockLog, err := os.Create(logPath)
	if err != nil {
		_ = os.RemoveAll(work)
		return nil, "", err
	}
	mockCtx, cancel := context.WithCancel(ctx)
	command := exec.CommandContext(mockCtx, executable, "radius-mock", "--port", "0", "--key", "ze-mock-key", "--user", "admin:testpass:admin", "--addr-file", addrPath)
	command.Stdout = io.Discard
	command.Stderr = mockLog
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		cancel()
		_ = mockLog.Close()
		_ = os.RemoveAll(work)
		return nil, "", err
	}
	_ = mockLog.Close()
	mock := &extra1RadiusMock{command: command, cancel: cancel, work: work}
	var address string
	if !Poll(ctx, 30, 100*time.Millisecond, func() bool {
		contents, err := os.ReadFile(addrPath)
		if err != nil {
			return false
		}
		address = strings.TrimSpace(string(contents))
		return address != ""
	}) {
		logContents, _ := os.ReadFile(logPath)
		extra1StopRadiusMock(mock)
		return nil, "", fmt.Errorf("RADIUS mock did not report address: %s", logContents)
	}
	return mock, address, nil
}

func extra1StopRadiusMock(mock *extra1RadiusMock) {
	if mock == nil {
		return
	}
	mock.cancel()
	if mock.command != nil {
		_ = mock.command.Wait()
	}
	_ = os.RemoveAll(mock.work)
}
