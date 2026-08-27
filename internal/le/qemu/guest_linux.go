//go:build linux

// Design: plan/spec-le-is-a-ze-binary.md -- step 10 guest-side evidence ports
package qemu

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/featuretags"
)

const (
	guestPollInterval = 100 * time.Millisecond
	processGrace      = 3 * time.Second
	processKillWait   = 2 * time.Second
	collectorLineMax  = 4096
)

const (
	guestStorageBlobKey = "ZE_STORAGE_BLOB"
	guestConfigDirKey   = "ZE_CONFIG_DIR"
	guestEvidenceZeKey  = "ZE_EVIDENCE_ZE_BINARY"
	guestTestBinKey     = "ZE_TEST_BIN"
)

var execLookPath = exec.LookPath

type guestProcess struct {
	command *exec.Cmd
	done    chan struct{}
}

// startGuestProcess starts one owned process group. Its caller MUST call stop
// before releasing the lab; context cancellation is the second bounded owner.
func startGuestProcess(ctx context.Context, namespace string, argv, environ []string, prefix string) (*guestProcess, *lineCollector, error) {
	if len(argv) == 0 {
		return nil, nil, errors.New("start guest process: empty command")
	}
	name, args := argv[0], argv[1:]
	if namespace != "" {
		name = "ip"
		args = append([]string{"netns", "exec", namespace}, argv...)
	}
	// #nosec G204 -- guest evidence actions provide closed command shapes whose variable paths are fixture-created binaries and configuration.
	command := exec.CommandContext(ctx, name, args...)
	// The cancellation owner below terminates the whole process group with the
	// established TERM/KILL bounds. CommandContext's default cancels only the
	// group leader, so leave its hook inert.
	command.Cancel = func() error { return nil }
	command.Env = environ
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("open %s stdout: %w", argv[0], err)
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("open %s stderr: %w", argv[0], err)
	}
	if err := command.Start(); err != nil {
		return nil, nil, fmt.Errorf("start %s: %w", argv[0], err)
	}
	collector := newLineCollector(prefix)
	collector.read(stdout)
	collector.read(stderr)
	process := &guestProcess{command: command, done: make(chan struct{})}
	go func() {
		_ = command.Wait()
		close(process.done)
	}()
	go func() {
		<-ctx.Done()
		if process.exited() {
			return
		}
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGTERM)
		timer := time.NewTimer(processGrace)
		defer timer.Stop()
		<-timer.C
		if !process.exited() {
			_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		}
	}()
	return process, collector, nil
}

func (p *guestProcess) exited() bool {
	if p == nil {
		return true
	}
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

// stop reaps the process group startGuestProcess returned. Every successful
// start MUST reach this method, including failure and cancellation paths.
func (p *guestProcess) stop() {
	if p == nil || p.exited() {
		return
	}
	_ = syscall.Kill(-p.command.Process.Pid, syscall.SIGTERM)
	select {
	case <-p.done:
		return
	case <-time.After(processGrace):
	}
	_ = syscall.Kill(-p.command.Process.Pid, syscall.SIGKILL)
	select {
	case <-p.done:
	case <-time.After(processKillWait):
	}
}

func (p *guestProcess) kill() error {
	if p == nil || p.exited() {
		return nil
	}
	if err := syscall.Kill(-p.command.Process.Pid, syscall.SIGKILL); err != nil {
		return err
	}
	select {
	case <-p.done:
		return nil
	case <-time.After(5 * time.Second):
		return errors.New("process did not exit after SIGKILL")
	}
}

func (p *guestProcess) signal(sig syscall.Signal) error {
	if p == nil || p.exited() {
		return errors.New("process is not running")
	}
	return syscall.Kill(-p.command.Process.Pid, sig)
}

type lineCollector struct {
	prefix string
	mu     sync.RWMutex
	lines  []string
	wake   chan struct{}
}

func newLineCollector(prefix string) *lineCollector {
	return &lineCollector{prefix: prefix, lines: make([]string, 0, 128), wake: make(chan struct{}, 1)}
}

func (c *lineCollector) read(reader io.Reader) {
	go func() {
		scanner := bufio.NewScanner(reader)
		buffer := make([]byte, 64*1024)
		scanner.Buffer(buffer, 1024*1024)
		var tb textbuf.Buffer
		for scanner.Scan() {
			line := tb.Str(scanner.Text()).Byte('\n').String()
			tb.Reset()
			c.mu.Lock()
			if len(c.lines) == collectorLineMax {
				copy(c.lines, c.lines[1:])
				c.lines[len(c.lines)-1] = line
			} else {
				c.lines = append(c.lines, line)
			}
			c.mu.Unlock()
			fmt.Fprint(os.Stderr, c.prefix, line) //nolint:errcheck // live evidence progress
			select {
			case c.wake <- struct{}{}:
			default:
			}
		}
	}()
}

func (c *lineCollector) snapshot() []string {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]string(nil), c.lines...)
}

func (c *lineCollector) wait(ctx context.Context, timeout time.Duration, process *guestProcess, predicate func([]string) bool, fatal func([]string) error) error {
	deadline, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		lines := c.snapshot()
		if fatal != nil {
			if err := fatal(lines); err != nil {
				return err
			}
		}
		if predicate(lines) {
			return nil
		}
		if process != nil && process.exited() {
			return errors.New("process exited before the expected output")
		}
		select {
		case <-deadline.Done():
			if errors.Is(deadline.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("expected output did not arrive within %s", timeout)
			}
			return deadline.Err()
		case <-c.wake:
		}
	}
}

func waitGuest(ctx context.Context, timeout, interval time.Duration, predicate func() (bool, error)) error {
	deadline, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		ready, err := predicate()
		if err != nil {
			return err
		}
		if ready {
			return nil
		}
		select {
		case <-deadline.Done():
			if errors.Is(deadline.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("condition was not met within %s", timeout)
			}
			return deadline.Err()
		case <-ticker.C:
		}
	}
}

func guestRun(ctx context.Context, namespace string, argv, environ []string) (commandResult, error) {
	if len(argv) == 0 {
		return commandResult{}, errors.New("run guest command: empty command")
	}
	name, args := argv[0], argv[1:]
	if namespace != "" {
		name = "ip"
		args = append([]string{"netns", "exec", namespace}, argv...)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	// #nosec G204 -- guest evidence actions provide closed command shapes whose variable paths are fixture-created binaries and configuration.
	command := exec.CommandContext(ctx, name, args...)
	command.Env = environ
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	result := commandResult{Stdout: stdout.String(), Stderr: stderr.String()}
	if err == nil {
		return result, nil
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		result.Code = exit.ExitCode()
		return result, nil
	}
	return result, fmt.Errorf("start %s: %w", argv[0], err)
}

func guestRequired(ctx context.Context, namespace string, argv []string, operation string) error {
	result, err := guestRun(ctx, namespace, argv, nil)
	if err != nil {
		return err
	}
	if result.Code != 0 {
		fmt.Fprint(os.Stderr, result.Stdout, result.Stderr) //nolint:errcheck // peer diagnostics
		return fmt.Errorf("%s failed with exit %d", operation, result.Code)
	}
	return nil
}

func requireGuestCommands(names ...string) error {
	for _, name := range names {
		if _, err := exec.LookPath(name); err != nil {
			return fmt.Errorf("missing required command: %s", name)
		}
	}
	return nil
}

func hasGuestNetAdmin() bool {
	if os.Geteuid() == 0 {
		return true
	}
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return false
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if !strings.HasPrefix(line, "CapEff:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return false
		}
		value, err := strconv.ParseUint(fields[1], 16, 64)
		return err == nil && value&(1<<12) != 0
	}
	return false
}

func buildGuestZe(ctx context.Context, root, output string, overrideKeys ...string) (string, error) {
	for _, key := range overrideKeys {
		if named := os.Getenv(key); named != "" {
			info, err := os.Stat(named)
			if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
				return "", fmt.Errorf("ze binary override is not an executable file: %s", named)
			}
			return named, nil
		}
	}
	if err := requireGuestCommands("go"); err != nil {
		return "", err
	}
	tags, err := featuretags.DaemonBuildTags(root, featuretags.DaemonBase)
	if err != nil {
		return "", err
	}
	bindir := filepath.Join(root, "tmp", "evidence", "bin")
	if err := os.MkdirAll(bindir, 0o750); err != nil {
		return "", fmt.Errorf("create evidence binary directory: %w", err)
	}
	binary := filepath.Join(bindir, output)
	environ := withGuestEnv(os.Environ(), map[string]string{
		"CGO_ENABLED": "0",
		"GOCACHE":     settingFromEnv("GOCACHE", filepath.Join(root, "tmp", "go-cache")),
	})
	result, err := guestRun(ctx, "", []string{"go", "build", "-tags", tags, "-o", binary, "./cmd/ze"}, environ)
	if err != nil {
		return "", err
	}
	if result.Code != 0 {
		fmt.Fprint(os.Stderr, result.Stdout, result.Stderr) //nolint:errcheck // build diagnostics
		return "", fmt.Errorf("go build ./cmd/ze failed with exit %d", result.Code)
	}
	return binary, nil
}

func settingFromEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func withGuestEnv(base []string, values map[string]string) []string {
	out := make([]string, 0, len(base)+len(values))
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if ok {
			if _, replaced := values[key]; replaced {
				continue
			}
		}
		out = append(out, entry)
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		var line textbuf.Buffer
		out = append(out, line.Str(key).Byte('=').Str(values[key]).String())
	}
	return out
}

func withoutGuestEnv(base []string, removed ...string) []string {
	names := make(map[string]bool, len(removed))
	for _, name := range removed {
		names[name] = true
	}
	out := make([]string, 0, len(base))
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if ok && names[key] {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func pidSuffix(prefix string) string {
	value := strconv.Itoa(os.Getpid())
	if len(value) > 6 {
		value = value[len(value)-6:]
	}
	return prefix + value
}

func cleanupNamespaces(ctx context.Context, namespaces, rootLinks []string, order *[]string) {
	var tb textbuf.Buffer
	for _, namespace := range namespaces {
		if order != nil {
			*order = append(*order, tb.Str("term:").Str(namespace).String())
			tb.Reset()
		}
		pids, _ := guestRun(ctx, "", []string{"ip", "netns", "pids", namespace}, nil)
		for raw := range strings.FieldsSeq(pids.Stdout) {
			pid, err := strconv.Atoi(raw)
			if err == nil {
				_ = syscall.Kill(pid, syscall.SIGTERM)
			}
		}
	}
	select {
	case <-ctx.Done():
	case <-time.After(200 * time.Millisecond):
	}
	for _, namespace := range namespaces {
		if order != nil {
			*order = append(*order, tb.Str("kill:").Str(namespace).String())
			tb.Reset()
		}
		pids, _ := guestRun(context.Background(), "", []string{"ip", "netns", "pids", namespace}, nil)
		for raw := range strings.FieldsSeq(pids.Stdout) {
			pid, err := strconv.Atoi(raw)
			if err == nil {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	}
	for _, link := range rootLinks {
		if order != nil {
			*order = append(*order, tb.Str("link:").Str(link).String())
			tb.Reset()
		}
		if _, err := guestRun(context.Background(), "", []string{"ip", "link", "delete", link}, nil); err != nil {
			fmt.Fprintf(os.Stderr, "cleanup root link %s: %v\n", link, err) //nolint:errcheck // cleanup diagnostics
		}
	}
	for _, namespace := range namespaces {
		if order != nil {
			*order = append(*order, tb.Str("netns:").Str(namespace).String())
			tb.Reset()
		}
		if _, err := guestRun(context.Background(), "", []string{"ip", "netns", "delete", namespace}, nil); err != nil {
			fmt.Fprintf(os.Stderr, "cleanup network namespace %s: %v\n", namespace, err) //nolint:errcheck // cleanup diagnostics
		}
	}
}
