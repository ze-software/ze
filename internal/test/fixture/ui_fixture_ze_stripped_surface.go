package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

func init() {
	Register("ui/ze-stripped-surface", uiDriver(runZEStrippedSurface))
}

type uiZeStrippedSurfaceCommandResult struct {
	stdout string
	stderr string
	code   int
}

type productHarness struct {
	dir string
}

type observedProcess struct {
	cmd    *exec.Cmd
	stdout bytes.Buffer
	stderr bytes.Buffer
	done   chan struct{}

	mu      sync.Mutex
	waitErr error
}

// Dispatch executes the compiled product binary directly and captures its output.
func (h *productHarness) Dispatch(
	ctx context.Context,
	env []string,
	stdin string,
	inheritStderr bool,
	args ...string,
) (uiZeStrippedSurfaceCommandResult, error) {
	cmd := exec.CommandContext(ctx, "ze-stripped", args...) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Dir = h.dir
	if env != nil {
		cmd.Env = env
	}
	if stdin == "" {
		cmd.Stdin = os.Stdin
	} else {
		cmd.Stdin = strings.NewReader(stdin)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	if inheritStderr {
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stderr = &stderr
	}

	err := cmd.Run()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return uiZeStrippedSurfaceCommandResult{}, ctxErr
	}

	result := uiZeStrippedSurfaceCommandResult{
		stdout: stdout.String(),
		stderr: stderr.String(),
		code:   0,
	}
	if err == nil {
		return result, nil
	}

	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		result.code = exitErr.ExitCode()
		return result, nil
	}
	return uiZeStrippedSurfaceCommandResult{}, fmt.Errorf("start ze-stripped %q: %w", args, err)
}

// Observe starts and continuously observes the compiled daemon process.
func (h *productHarness) Observe(ctx context.Context, env []string, args ...string) (*observedProcess, error) {
	p := &observedProcess{done: make(chan struct{})}
	p.cmd = exec.CommandContext(ctx, "ze-stripped", args...) //nolint:gosec // the fixture chooses the program and its arguments
	p.cmd.Dir = h.dir
	p.cmd.Env = env
	p.cmd.Stdin = os.Stdin
	p.cmd.Stdout = &p.stdout
	p.cmd.Stderr = &p.stderr

	if err := p.cmd.Start(); err != nil {
		return nil, fmt.Errorf("start ze-stripped daemon: %w", err)
	}
	go func() {
		err := p.cmd.Wait()
		p.mu.Lock()
		p.waitErr = err
		p.mu.Unlock()
		close(p.done)
	}()
	return p, nil
}

// Poll waits for both readiness files, checking daemon liveness before every
// readiness check and using a bounded polling interval rather than a blind sleep.
func (h *productHarness) Poll(
	ctx context.Context,
	p *observedProcess,
	timeout time.Duration,
	interval time.Duration,
	paths ...string,
) error {
	deadline := time.Now().Add(timeout)
	for {
		select {
		case <-p.done:
			return fmt.Errorf(
				"daemon exited early\nstdout:\n%s\nstderr:\n%s",
				p.stdout.String(),
				p.stderr.String(),
			)
		default:
		}

		ready := true
		for _, path := range paths {
			if _, err := os.Stat(path); err != nil {
				ready = false
				break
			}
		}
		if ready {
			return nil
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return errors.New("daemon did not become ready")
		}
		wait := interval
		wait = min(wait, remaining)
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-p.done:
			if !timer.Stop() {
				<-timer.C
			}
			return fmt.Errorf(
				"daemon exited early\nstdout:\n%s\nstderr:\n%s",
				p.stdout.String(),
				p.stderr.String(),
			)
		case <-timer.C:
		}
	}
}

func (p *observedProcess) Stop() error {
	select {
	case <-p.done:
		return nil
	default:
	}

	if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("terminate daemon: %w", err)
	}
	if waitForProcess(p.done, 5*time.Second) {
		return nil
	}

	if err := p.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("kill daemon: %w", err)
	}
	if !waitForProcess(p.done, 5*time.Second) {
		return errors.New("daemon did not exit within five seconds after kill")
	}
	return nil
}

func waitForProcess(done <-chan struct{}, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}

func runZEStrippedSurface(ctx context.Context) (retErr error) {
	wd, err := os.MkdirTemp("", "ze-ui-stripped-surface-")
	if err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	defer os.RemoveAll(wd) //nolint:errcheck // fixture cleanup
	h := &productHarness{dir: wd}

	help, err := h.Dispatch(ctx, nil, "", true, "help", "command", "--json")
	if err != nil {
		return err
	}
	if help.code != 0 {
		return fmt.Errorf("ze-stripped help command failed with exit code %d", help.code)
	}
	var commandEntries []struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(help.stdout), &commandEntries); err != nil {
		return fmt.Errorf("decode command discovery output: %w", err)
	}
	commands := make(map[string]struct{}, len(commandEntries))
	for _, entry := range commandEntries {
		commands[entry.Path] = struct{}{}
	}
	for _, forbidden := range []string{
		"install",
		"service",
		"uninstall",
	} {
		if _, present := commands[forbidden]; present {
			return fmt.Errorf("%s unexpectedly present", forbidden)
		}
	}
	for _, required := range []string{
		"update system firmware check",
		"update system firmware download",
		"update system firmware apply",
		"update system firmware restart",
		"update system firmware rollback",
	} {
		if _, present := commands[required]; !present {
			return fmt.Errorf("%s unexpectedly missing", required)
		}
	}

	install, err := h.Dispatch(ctx, nil, "", false, "install")
	if err != nil {
		return err
	}
	if install.code == 0 {
		return errors.New("install unexpectedly succeeded")
	}
	if !strings.Contains(install.stderr, "unknown command: install") {
		return errors.New(install.stderr)
	}

	passwd, err := h.Dispatch(ctx, nil, "secret\n", true, "passwd")
	if err != nil {
		return err
	}
	if passwd.code != 0 {
		return fmt.Errorf("ze-stripped passwd failed with exit code %d", passwd.code)
	}
	passwordHash := strings.TrimSpace(passwd.stdout)

	config := `system {
    authentication {
        user ci {
            password "` + passwordHash + `"
            profile [ admin ]
        }
    }
}
`
	if err := os.WriteFile(filepath.Join(wd, "stripped.conf"), []byte(config), 0o600); err != nil {
		return fmt.Errorf("write stripped.conf: %w", err)
	}

	dbPath := filepath.Join(wd, "database.zefs")
	usernamePath := filepath.Join(wd, "username.txt")
	passwordPath := filepath.Join(wd, "password.txt")
	if err := os.WriteFile(usernamePath, []byte("ci"), 0o600); err != nil {
		return fmt.Errorf("write username.txt: %w", err)
	}
	if err := os.WriteFile(passwordPath, []byte(passwordHash), 0o600); err != nil {
		return fmt.Errorf("write password.txt: %w", err)
	}
	for _, write := range []struct {
		key string
		src string
	}{
		{key: "meta/auth/local/username", src: "username.txt"},
		{key: "meta/auth/local/password", src: "password.txt"},
	} {
		result, err := h.Dispatch(ctx, nil, "", false, "data", "--path", dbPath, "write", write.key, write.src)
		if err != nil {
			return err
		}
		if result.code != 0 {
			return commandFailure("data write", result)
		}
	}

	sshAddressPath := filepath.Join(wd, "ssh.addr")
	readyPath := filepath.Join(wd, "ready")
	daemonEnv := updateEnvironment(
		os.Environ(),
		"ZE_SSH_EPHEMERAL="+sshAddressPath,
		"ZE_READY_FILE="+readyPath,
		"ZE_CONFIG_DIR="+wd,
	)
	daemon, err := h.Observe(ctx, daemonEnv, "-f", "stripped.conf")
	if err != nil {
		return err
	}
	defer func() {
		if stopErr := daemon.Stop(); stopErr != nil {
			retErr = stopErr
		}
	}()

	if err := h.Poll(ctx, daemon, 15*time.Second, 100*time.Millisecond, sshAddressPath, readyPath); err != nil {
		return err
	}

	addressBytes, err := os.ReadFile(sshAddressPath) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return fmt.Errorf("read ssh.addr: %w", err)
	}
	address := strings.TrimSpace(string(addressBytes))
	colon := strings.LastIndex(address, ":")
	if colon < 0 {
		return fmt.Errorf("invalid SSH address %q", address)
	}
	host, port := address[:colon], address[colon+1:]
	cliEnv := updateEnvironment(
		os.Environ(),
		"ZE_SSH_HOST="+host,
		"ZE_SSH_PORT="+port,
		"ZE_SSH_USERNAME=ci",
		"ZE_SSH_PASSWORD=secret",
		"ZE_CONFIG_DIR="+wd,
	)

	metadataWrite, err := h.Dispatch(
		ctx,
		cliEnv,
		"",
		false,
		"data", "--path", dbPath, "write", "meta/ssh/"+host+"/"+port+"/username", "username.txt",
	)
	if err != nil {
		return err
	}
	if metadataWrite.code != 0 {
		return commandFailure("SSH username data write", metadataWrite)
	}

	firmwareCheck, err := h.Dispatch(
		ctx,
		cliEnv,
		"",
		false,
		"cli", "-c", "update system firmware check",
	)
	if err != nil {
		return err
	}
	combined := firmwareCheck.stdout + firmwareCheck.stderr
	if firmwareCheck.code == 0 {
		return errors.New("firmware check unexpectedly succeeded: " + combined)
	}
	if !strings.Contains(combined, "self-update unavailable in minimal build") {
		return errors.New(combined)
	}
	if strings.Contains(combined, "update checker not configured") {
		return errors.New(combined)
	}

	fmt.Println("OK")
	return nil
}

func commandFailure(operation string, result uiZeStrippedSurfaceCommandResult) error {
	return fmt.Errorf(
		"%s failed with exit code %d\nstdout:\n%s\nstderr:\n%s",
		operation,
		result.code,
		result.stdout,
		result.stderr,
	)
}

func updateEnvironment(base []string, updates ...string) []string {
	values := make(map[string]string, len(updates))
	order := make([]string, 0, len(updates))
	for _, update := range updates {
		key, _, _ := strings.Cut(update, "=")
		if _, exists := values[key]; !exists {
			order = append(order, key)
		}
		values[key] = update
	}

	result := make([]string, 0, len(base)+len(updates))
	seen := make(map[string]bool, len(updates))
	for _, entry := range base {
		key, _, _ := strings.Cut(entry, "=")
		if replacement, replace := values[key]; replace {
			if !seen[key] {
				result = append(result, replacement)
				seen[key] = true
			}
			continue
		}
		result = append(result, entry)
	}
	for _, key := range order {
		if !seen[key] {
			result = append(result, values[key])
		}
	}
	return result
}

var _ io.Writer = (*bytes.Buffer)(nil)
