// Design: docs/contributing/terminal-demos.md -- deadline-bound owned journey startup.
package siteterminaldemo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/test/runner"
)

// webConfigWorkspace owns a private scenario store, never a previous run's PID
// file. The constructor's caller MUST defer Close before starting any child;
// every started child MUST be joined before Close removes its files.
// It is single-owner; its zero value is invalid.
type webConfigWorkspace struct {
	directory string
	environ   []string
}

func newWebConfigWorkspace(ctx context.Context) (*webConfigWorkspace, error) {
	if err := context.Cause(ctx); err != nil {
		return nil, err
	}
	stateRoot, err := filepath.Abs(demoStateRoot())
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(stateRoot, 0o750); err != nil {
		return nil, err
	}
	directory, err := os.MkdirTemp(stateRoot, "web-config-")
	if err != nil {
		return nil, err
	}
	configDir := filepath.Join(directory, "config")
	if err := os.Mkdir(configDir, 0o750); err != nil {
		return nil, errors.Join(err, os.RemoveAll(directory))
	}
	environ := env.Without(scenarioEnv(filepath.Base(directory), demoPassword), "ze.config.dir")
	environ = append(environ, "ZE_CONFIG_DIR="+configDir)
	return &webConfigWorkspace{directory: directory, environ: environ}, nil
}

// Close MUST run only after every child using this workspace has been joined.
func (w *webConfigWorkspace) Close() error {
	return os.RemoveAll(w.directory)
}

// start MUST return a retained daemon owner only after readiness succeeds. The
// caller MUST defer Stop immediately; startup failures stop and join it here.
func (w *webConfigWorkspace) start(ctx context.Context) (_ *runner.ServerProcess, err error) {
	if _, err := w.run(ctx, []string{commandInit}, strings.NewReader(initText(demoPassword)), "init.log"); err != nil {
		return nil, err
	}
	active, err := w.run(ctx, []string{commandConfig, commandCat, zeConfigFile}, nil, "active.log")
	if err != nil {
		return nil, err
	}
	source, err := os.ReadFile(filepath.Join(demoDir("web-config"), "ze.conf"))
	if err != nil {
		return nil, err
	}
	activePath := filepath.Join(w.directory, "active.conf")
	if err := os.WriteFile(activePath, append(active, source...), 0o600); err != nil {
		return nil, err
	}
	if _, err := w.run(ctx, importScenarioConfigArgs(activePath), nil, "import.log"); err != nil {
		return nil, err
	}
	logPath := filepath.Join(w.directory, "daemon.log")
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, err
	}
	server, startErr := runner.StartServerProcess(w.command(ctx, daemonArgs(), nil, log))
	if startErr != nil {
		return nil, errors.Join(startErr, log.Close())
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, server.Stop(syscall.SIGTERM, runner.TeardownGraceTimeout, 0))
			output, readErr := os.ReadFile(logPath)
			err = fmt.Errorf("%w\n%s", errors.Join(err, readErr), output)
		}
	}()
	if err := log.Close(); err != nil {
		return nil, err
	}
	if err := w.waitReady(ctx, server, logPath); err != nil {
		return nil, err
	}
	return server, nil
}

func (w *webConfigWorkspace) command(ctx context.Context, args []string, input io.Reader, log *os.File) *exec.Cmd {
	process := exec.CommandContext(ctx, "ze", args...) //nolint:gosec // The executable and arguments are the closed web-config setup actions.
	process.Env = w.environ
	process.Dir = w.directory
	process.Stdin = input
	process.Stdout = log
	process.Stderr = log
	return process
}

func (w *webConfigWorkspace) run(ctx context.Context, args []string, input io.Reader, logName string) (_ []byte, err error) {
	if err := context.Cause(ctx); err != nil {
		return nil, err
	}
	logPath := filepath.Join(w.directory, logName)
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, log.Close()) }()
	process, err := runner.StartServerProcess(w.command(ctx, args, input, log))
	if err != nil {
		return nil, errors.Join(err, context.Cause(ctx))
	}
	defer func() { err = errors.Join(err, process.Close()) }()
	waitErr := process.Wait()
	output, readErr := os.ReadFile(logPath)
	if err := errors.Join(waitErr, readErr, context.Cause(ctx)); err != nil {
		return nil, fmt.Errorf("web-config startup ze %s: %w\n%s", strings.Join(args, " "), err, output)
	}
	return output, nil
}

// waitReady consumes the original authored deadline, not a fresh polling budget.
func (w *webConfigWorkspace) waitReady(ctx context.Context, server *runner.ServerProcess, path string) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := context.Cause(ctx); err != nil {
			return err
		}
		select {
		case <-server.Done():
			return errors.Join(errors.New("web-config daemon exited before readiness"), server.Wait())
		default:
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(content, []byte("web server listening")) {
			return nil
		}
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		case <-server.Done():
			return errors.Join(errors.New("web-config daemon exited before readiness"), server.Wait())
		case <-ticker.C:
		}
	}
}
