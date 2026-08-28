package fixture

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const recordDriverConfig = `plugin {
	external record-plugin {
		run "ze-test record-plugin"
		encoder json
	}
}

bgp {
	peer peer1 {
		connection {
			remote {
				ip 127.0.0.2
			}
			local {
				ip 127.0.0.1
				accept false
			}
		}
		session {
			asn {
				local 65533
				remote 65533
			}
		}
	}
}

system {
	authentication {
		user admin {
			password "$2a$04$UlwuiuH82Unfsq.XEMPGJeDkXwbm3KW.nvVaVXOd/JeFK8VjMjrQO"
			profile [ admin ]
		}
	}
	authorization {
		profile admin {
			run {
				default-action allow
			}
			edit {
				default-action allow
			}
		}
	}
}

environment {
	ssh {
		enabled true
		server main {
			ip 127.0.0.1;
			port 0;
		}
	}
}
`

type recordDriver struct {
	ctx       context.Context
	directory string
	configDir string
	logPath   string
	command   *exec.Cmd
	done      chan struct{}
	cliEnv    []string
}

func commandPartialFaultDriver(ctx context.Context, _ []string) error {
	driver, err := startRecordDriver(ctx)
	if err != nil {
		return err
	}
	defer driver.close()
	driver.waitForCommand("show test records fault")

	code, answer, stderr, err := runCaptured(ctx, driver.cliEnv, "", "ze", "cli", "-c", "show test records fault | raw")
	if err != nil || code != 0 {
		return fmt.Errorf("the walk with a refused row lost the whole answer: %v %s%s\n%s", err, answer, stderr, driver.log())
	}
	path := filepath.Join(driver.directory, "fault.json")
	if err := os.WriteFile(path, []byte(answer), 0o600); err != nil {
		return err
	}
	return verifyPartialFault(ctx, []string{path})
}

func ownedCommandStreamsDriver(ctx context.Context, _ []string) error {
	driver, err := startRecordDriver(ctx)
	if err != nil {
		return err
	}
	defer driver.close()
	if !driver.waitForCommand("show test records walk") {
		return fmt.Errorf("the plugin never registered its command\n%s", driver.log())
	}

	path := filepath.Join(driver.directory, "walk.ndjson")
	stderr, err := driver.runCLIToFile("show test records walk | ndjson", path)
	if err != nil {
		return fmt.Errorf("the plugin command did not answer: %v %s", err, stderr)
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	byteCount := info.Size()
	lineCount, err := countFileLines11(path)
	if err != nil {
		return err
	}
	if byteCount <= 16777216 {
		return fmt.Errorf("fixture problem -- the walk is %d bytes, inside the 16777216-byte wire message", byteCount)
	}
	if lineCount <= 256 {
		return fmt.Errorf("fixture problem -- the walk is %d rows, inside the 256-record threshold", lineCount)
	}
	if err := verifyCommandStream(ctx, []string{path}); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "OK: the plugin walk streamed %d rows and %d bytes\n", lineCount, byteCount)
	return nil
}

func readsEngineAnswerDriver(ctx context.Context, _ []string) error {
	driver, err := startRecordDriver(ctx)
	if err != nil {
		return err
	}
	defer driver.close()
	driver.waitForCommand("show test engine answer")

	code, commands, stderr, err := runCaptured(ctx, driver.cliEnv, "", "ze", "cli", "-c", "system command list | ndjson")
	if err != nil || code != 0 {
		return fmt.Errorf("system command list did not answer: %v %s%s", err, commands, stderr)
	}
	commandCount := strings.Count(commands, "\n")
	if commandCount <= 256 {
		return fmt.Errorf("fixture problem -- the daemon registers %d commands, inside the 256-record threshold", commandCount)
	}
	code, reading, stderr, err := runCaptured(ctx, driver.cliEnv, "", "ze", "cli", "-c", "show test engine answer | raw")
	if err != nil || code != 0 {
		return fmt.Errorf("the plugin could not report what it read: %v %s%s\n%s", err, reading, stderr, driver.log())
	}
	path := filepath.Join(driver.directory, "reading.json")
	if err := os.WriteFile(path, []byte(reading), 0o600); err != nil {
		return err
	}
	return verifyEngineAnswer(ctx, []string{path, strconv.Itoa(commandCount)})
}

func startRecordDriver(ctx context.Context) (*recordDriver, error) {
	if err := os.WriteFile("daemon.ready", nil, 0o600); err != nil {
		return nil, err
	}
	directory, err := os.MkdirTemp("", "ze-record-driver-")
	if err != nil {
		return nil, err
	}
	driver := &recordDriver{
		ctx:       ctx,
		directory: directory,
		configDir: filepath.Join(directory, "admin"),
		logPath:   filepath.Join(directory, "daemon.log"),
		done:      make(chan struct{}),
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = driver.close()
		}
	}()
	if err := os.Mkdir(driver.configDir, 0o700); err != nil {
		return nil, err
	}
	configPath := filepath.Join(directory, "record-driver.conf")
	if err := os.WriteFile(configPath, []byte(recordDriverConfig), 0o600); err != nil {
		return nil, err
	}
	logFile, err := os.Create(driver.logPath)
	if err != nil {
		return nil, err
	}
	driver.command = exec.CommandContext(ctx, "ze", "start", configPath)
	driver.command.Env = append(os.Environ(), "ze_test_bgp_port="+strconv.Itoa(10000+os.Getpid()%50000))
	driver.command.Stdout = os.Stdout
	driver.command.Stderr = logFile
	if err := driver.command.Start(); err != nil {
		_ = logFile.Close()
		return nil, err
	}
	go func() {
		_ = driver.command.Wait()
		_ = logFile.Close()
		close(driver.done)
	}()

	port, err := driver.waitForSSHPort()
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(os.Stderr, "SSH port: %s\n", port)
	// Preserve the original authorizer wiring boundary after the listener is up.
	select {
	case <-time.After(500 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	initEnv := append(os.Environ(), "ZE_CONFIG_DIR="+driver.configDir)
	initInput := fmt.Sprintf("admin\ntestpass\n127.0.0.1\n%s\n", port)
	code, _, initErr, runErr := runCaptured(ctx, initEnv, initInput, "ze", "init")
	if runErr != nil || code != 0 {
		return nil, fmt.Errorf("ze init exit=%d: %v %s", code, runErr, initErr)
	}
	driver.cliEnv = append(os.Environ(), "ZE_CONFIG_DIR="+driver.configDir, "ZE_SSH_PASSWORD=testpass")
	cleanup = false
	return driver, nil
}

var sshAddress11 = regexp.MustCompile(`127\.0\.0\.1:([0-9]+)`)

func (driver *recordDriver) waitForSSHPort() (string, error) {
	for range 50 {
		log := driver.log()
		match := sshAddress11.FindStringSubmatch(log)
		if len(match) == 2 {
			return match[1], nil
		}
		select {
		case <-driver.done:
			return "", fmt.Errorf("SSH server did not start (daemon exited)\n%s", driver.log())
		case <-driver.ctx.Done():
			return "", driver.ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return "", fmt.Errorf("SSH server did not start (no address in daemon log)\n%s", driver.log())
}

func (driver *recordDriver) waitForCommand(name string) bool {
	for range 50 {
		code, stdout, _, err := runCaptured(driver.ctx, driver.cliEnv, "", "ze", "cli", "-c", "system command list | raw")
		if err == nil && code == 0 && strings.Contains(stdout, name) {
			return true
		}
		select {
		case <-driver.done:
			return false
		case <-driver.ctx.Done():
			return false
		case <-time.After(200 * time.Millisecond):
		}
	}
	return false
}

func (driver *recordDriver) runCLIToFile(command, path string) (string, error) {
	file, err := os.Create(path)
	if err != nil {
		return "", err
	}
	process := exec.CommandContext(driver.ctx, "ze", "cli", "-c", command)
	process.Env = driver.cliEnv
	process.Stdout = file
	var stderr strings.Builder
	process.Stderr = &stderr
	runErr := process.Run()
	closeErr := file.Close()
	return stderr.String(), errors.Join(runErr, closeErr)
}

func countFileLines11(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close() //nolint:errcheck
	buffer := make([]byte, 64*1024)
	lines := 0
	for {
		n, readErr := file.Read(buffer)
		for _, value := range buffer[:n] {
			if value == '\n' {
				lines++
			}
		}
		if errors.Is(readErr, io.EOF) {
			return lines, nil
		}
		if readErr != nil {
			return 0, readErr
		}
	}
}

func (driver *recordDriver) log() string {
	raw, _ := os.ReadFile(driver.logPath)
	return string(raw)
}

func (driver *recordDriver) close() error {
	if driver == nil {
		return nil
	}
	defer os.RemoveAll(driver.directory) //nolint:errcheck
	if driver.command == nil || driver.command.Process == nil {
		return nil
	}
	select {
	case <-driver.done:
		return nil
	default:
	}
	_ = driver.command.Process.Signal(syscall.SIGTERM)
	select {
	case <-driver.done:
		return nil
	case <-time.After(5 * time.Second):
		_ = driver.command.Process.Kill()
		<-driver.done
		return nil
	}
}
