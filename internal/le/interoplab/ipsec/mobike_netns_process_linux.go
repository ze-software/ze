//go:build linux

// Design: docs/architecture/testing/qemu-integration.md -- owned native MOBIKE peers.
// Related: mobike_netns_linux.go -- namespace, configuration, and scenario lifecycle.
package ipsec

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	mobikeNativeOutputMax   = 1024 * 1024
	mobikeNativeKillWait    = 2 * time.Second
	mobikeNativeDetachedMax = 4
)

// mobikeNativeOutput retains a bounded log tail. Safe for concurrent use by
// os/exec's stdout/stderr copies and the checker reading live daemon logs.
type mobikeNativeOutput struct {
	mu        sync.Mutex
	data      []byte
	truncated bool
}

func (o *mobikeNativeOutput) Write(data []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	count := len(data)
	if count >= mobikeNativeOutputMax {
		o.data = append(o.data[:0], data[count-mobikeNativeOutputMax:]...)
		o.truncated = true
		return count, nil
	}
	if excess := len(o.data) + count - mobikeNativeOutputMax; excess > 0 {
		copy(o.data, o.data[excess:])
		o.data = o.data[:len(o.data)-excess]
		o.truncated = true
	}
	o.data = append(o.data, data...)
	return count, nil
}

func (o *mobikeNativeOutput) text(lines int) string {
	o.mu.Lock()
	defer o.mu.Unlock()
	answer := string(o.data)
	if lines <= 0 {
		return answer
	}
	end := len(answer)
	if strings.HasSuffix(answer, "\n") {
		end--
	}
	for index := end - 1; index >= 0; index-- {
		if answer[index] != '\n' {
			continue
		}
		lines--
		if lines == 0 {
			return answer[index+1:]
		}
	}
	return answer
}

// mobikeNativeProcess owns one process group and its Wait goroutine. The caller
// MUST call stop before releasing its namespace, including after cancellation.
type mobikeNativeProcess struct {
	command *exec.Cmd
	output  mobikeNativeOutput
	done    chan struct{}
	waitErr error
}

func mobikeNativeExec(ctx context.Context, argv, environ []string, directory string) *exec.Cmd {
	// #nosec G204 -- argv comes from the native lab's fixed process and checker commands.
	command := exec.CommandContext(ctx, argv[0], argv[1:]...)
	command.Dir = directory
	command.Env = environ
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.WaitDelay = mobikeNativeKillWait
	command.Cancel = func() error {
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	return command
}

func mobikeNativeCommand(ctx context.Context, argv, environ []string, directory string) (interoplab.CommandResult, error) {
	ctx, cancel := context.WithTimeout(ctx, mobikeNativeCommandTimeout)
	defer cancel()
	command := mobikeNativeExec(ctx, argv, environ, directory)
	var stdout, stderr mobikeNativeOutput
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	result := interoplab.CommandResult{Stdout: stdout.text(0), Stderr: stderr.text(0)}
	if err == nil {
		if stdout.truncated || stderr.truncated {
			return result, fmt.Errorf("native MOBIKE command %q exceeded the %d-byte output bound", argv, mobikeNativeOutputMax)
		}
		return result, nil
	}
	result.ExitCode = 1
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		result.ExitCode = exit.ExitCode()
	}
	if ctx.Err() != nil {
		result.ExitCode = 124
		err = ctx.Err()
	}
	return result, fmt.Errorf("native MOBIKE command %q: %w\nstdout: %s\nstderr: %s", argv, err, result.Stdout, result.Stderr)
}

// startMOBIKENativeProcess starts a foreground daemon in its own process group.
// The caller MUST call stop on every successful start; the context additionally
// kills the whole process group if the scenario is canceled or exceeds its bound.
func startMOBIKENativeProcess(ctx context.Context, argv, environ []string, directory string) (*mobikeNativeProcess, error) {
	process := &mobikeNativeProcess{
		command: mobikeNativeExec(ctx, argv, environ, directory),
		done:    make(chan struct{}),
	}
	process.command.Stdout = &process.output
	process.command.Stderr = &process.output
	if err := process.command.Start(); err != nil {
		return nil, fmt.Errorf("start native MOBIKE process %q: %w", argv, err)
	}
	go func() {
		process.waitErr = process.command.Wait()
		close(process.done)
	}()
	return process, nil
}

func (p *mobikeNativeProcess) running() bool {
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}

func (p *mobikeNativeProcess) signal(signal syscall.Signal) error {
	if !p.running() {
		return fmt.Errorf("native MOBIKE process exited: %v", p.waitErr) //nolint:errorlint // A clean exit leaves the wait error nil; %w would print %!w(<nil>), and it is diagnostic context, not a cause a caller unwraps.
	}
	if err := syscall.Kill(-p.command.Process.Pid, signal); err != nil {
		return fmt.Errorf("signal native MOBIKE process: %w", err)
	}
	return nil
}

// stop MUST be called for every successful start before namespace deletion.
// SIGCONT lets a paused peer handle SIGTERM; SIGKILL bounds an unresponsive peer.
func (p *mobikeNativeProcess) stop(grace time.Duration) error {
	if !p.running() {
		return nil
	}
	var signalErr error
	for _, signal := range []syscall.Signal{syscall.SIGTERM, syscall.SIGCONT} {
		if err := syscall.Kill(-p.command.Process.Pid, signal); err != nil {
			if !errors.Is(err, syscall.ESRCH) {
				signalErr = errors.Join(signalErr, err)
			}
		}
	}
	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case <-p.done:
		return signalErr
	case <-timer.C:
	}
	if err := syscall.Kill(-p.command.Process.Pid, syscall.SIGKILL); err != nil {
		if !errors.Is(err, syscall.ESRCH) {
			signalErr = errors.Join(signalErr, err)
		}
	}
	timer.Reset(mobikeNativeKillWait)
	select {
	case <-p.done:
		return signalErr
	case <-timer.C:
		return errors.Join(signalErr, errors.New("native MOBIKE process did not exit after SIGKILL"))
	}
}

func (l *mobikeNativeLab) peer(name string) (*mobikeNativePeer, error) {
	peer := l.peers[name]
	if peer == nil {
		return nil, fmt.Errorf("native MOBIKE has no peer %q", name)
	}
	return peer, nil
}

func (l *mobikeNativeLab) peerCommand(name string, argv []string, environ []interoplab.EnvironmentVariable) ([]string, []string, error) {
	peer, err := l.peer(name)
	if err != nil {
		return nil, nil, err
	}
	if len(argv) == 0 {
		return nil, nil, errors.New("native MOBIKE peer command is empty")
	}
	command := append([]string{l.binaries["ip"], "netns", "exec", peer.namespace}, argv...)
	if path, ok := l.binaries[argv[0]]; ok {
		command[4] = path
	}
	if argv[0] == "swanctl" {
		command = append(command, "--uri", l.viciURI())
	}
	// The shared checker uses its Docker-local store path in a shell command.
	// Remap only that path, leaving the CLI command and assertions unchanged.
	for index := 4; index < len(command); index++ {
		command[index] = strings.ReplaceAll(command[index], zeCLIStore, filepath.Join(l.directory, "cli"))
	}
	extra := make([]string, 0, len(environ))
	for _, variable := range environ {
		extra = append(extra, variable.Name+"="+variable.Value)
	}
	return command, l.environment(extra), nil
}

func (l *mobikeNativeLab) Exec(ctx context.Context, name string, argv []string, environ []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	command, environment, err := l.peerCommand(name, argv, environ)
	if err != nil {
		return interoplab.CommandResult{}, err
	}
	return mobikeNativeCommand(ctx, command, environment, l.directory)
}

func (l *mobikeNativeLab) Query(ctx context.Context, name string, argv []string, environ []interoplab.EnvironmentVariable) (string, error) {
	result, err := l.Exec(ctx, name, argv, environ)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(result.Stdout) == "" {
		return "", fmt.Errorf("native MOBIKE peer %s query returned no output", name)
	}
	return result.Stdout, nil
}

func (l *mobikeNativeLab) ExecDetached(ctx context.Context, name string, argv []string, environ []interoplab.EnvironmentVariable) error {
	if len(l.detached) >= mobikeNativeDetachedMax {
		return errors.New("native MOBIKE detached process bound reached")
	}
	command, environment, err := l.peerCommand(name, argv, environ)
	if err != nil {
		return err
	}
	process, err := startMOBIKENativeProcess(ctx, command, environment, l.directory)
	if err != nil {
		return err
	}
	l.detached = append(l.detached, process)
	return nil
}

func (l *mobikeNativeLab) Logs(ctx context.Context, name string, lines int) (interoplab.LogResult, error) {
	if err := ctx.Err(); err != nil {
		return interoplab.LogResult{}, err
	}
	peer, err := l.peer(name)
	if err != nil {
		return interoplab.LogResult{}, err
	}
	if peer.process == nil {
		return interoplab.LogResult{}, fmt.Errorf("native MOBIKE peer %s has not started", name)
	}
	if lines <= 0 || lines > logLinesMax {
		return interoplab.LogResult{}, fmt.Errorf("native MOBIKE log line count outside 1..%d", logLinesMax)
	}
	return interoplab.LogResult{Text: peer.process.output.text(lines), Available: true}, nil
}

func (l *mobikeNativeLab) PeerPID(ctx context.Context, name string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	peer, err := l.peer(name)
	if err != nil {
		return 0, err
	}
	if peer.process == nil {
		return 0, fmt.Errorf("native MOBIKE peer %s has not started", name)
	}
	if !peer.process.running() {
		return 0, fmt.Errorf("native MOBIKE peer %s exited: %v", name, peer.process.waitErr) //nolint:errorlint // A clean exit leaves the wait error nil; %w would print %!w(<nil>), and it is diagnostic context, not a cause a caller unwraps.
	}
	return peer.process.command.Process.Pid, nil
}

func (l *mobikeNativeLab) Signal(ctx context.Context, name, signal string) error {
	if _, err := l.PeerPID(ctx, name); err != nil {
		return err
	}
	var value syscall.Signal
	switch signal {
	case "HUP", "SIGHUP":
		value = syscall.SIGHUP
	case "INT", "SIGINT":
		value = syscall.SIGINT
	case "TERM", "SIGTERM":
		value = syscall.SIGTERM
	case "KILL", "SIGKILL":
		value = syscall.SIGKILL
	case "STOP", "SIGSTOP":
		value = syscall.SIGSTOP
	case "CONT", "SIGCONT":
		value = syscall.SIGCONT
	default:
		return fmt.Errorf("native MOBIKE does not support signal %q", signal)
	}
	return l.peers[name].process.signal(value)
}

func (l *mobikeNativeLab) Pause(ctx context.Context, name string) error {
	return l.Signal(ctx, name, "STOP")
}

func (l *mobikeNativeLab) Unpause(ctx context.Context, name string) error {
	return l.Signal(ctx, name, "CONT")
}

func (l *mobikeNativeLab) Start(ctx context.Context, name string) error {
	peer, err := l.peer(name)
	if err != nil {
		return err
	}
	if peer.process != nil {
		if peer.process.running() {
			return fmt.Errorf("native MOBIKE peer %s is already running", name)
		}
	}
	if len(peer.argv) == 0 {
		return fmt.Errorf("native MOBIKE peer %s has no configured daemon", name)
	}
	command := append([]string{l.binaries["ip"], "netns", "exec", peer.namespace}, peer.argv...)
	peer.process, err = startMOBIKENativeProcess(ctx, command, l.environment(peer.environ), l.directory)
	return err
}

func (l *mobikeNativeLab) Stop(ctx context.Context, name string, timeoutSeconds int) error {
	peer, err := l.peer(name)
	if err != nil {
		return err
	}
	if timeoutSeconds < 0 || timeoutSeconds > 30 {
		return errors.New("native MOBIKE stop timeout must be between zero and 30 seconds")
	}
	if peer.process == nil {
		return fmt.Errorf("native MOBIKE peer %s has not started", name)
	}
	grace := time.Duration(timeoutSeconds) * time.Second
	if ctx.Err() != nil {
		grace = 0
	}
	return peer.process.stop(grace)
}
