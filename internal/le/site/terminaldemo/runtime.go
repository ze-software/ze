package siteterminaldemo

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/textbuf"
)

//go:embed cards.json
var cardData []byte

var cards map[string]string

func init() {
	if err := json.Unmarshal(cardData, &cards); err != nil {
		panic("BUG: invalid embedded demo cards: " + err.Error())
	}
}

// RuntimeMain runs the compiled command used by demo fixtures and the renderer
// container. It deliberately accepts positional arguments because it is a
// standalone fixture binary, not an le command surface.
func RuntimeMain(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "usage: ze-demo card DEMO PHASE | run DEMO ACTION [ARG...] | validate DEMO | entrypoint COMMAND [ARG...] | shell")
		return 2
	}
	var err error
	switch args[0] {
	case "card":
		if len(args) != 3 {
			err = errors.New("usage: ze-demo card DEMO PHASE")
			break
		}
		err = printCard(stdout, args[1], args[2])
	case "run":
		if len(args) < 3 {
			err = errors.New("usage: ze-demo run DEMO ACTION [ARG...]")
			break
		}
		err = runScenario(args[1], args[2], args[3:], stdout, stderr)
	case commandValidate:
		if len(args) != 2 {
			err = errors.New("usage: ze-demo validate DEMO")
			break
		}
		err = validateDemoRuntime(args[1], stdout, stderr)
	case "entrypoint":
		if len(args) < 2 {
			err = errors.New("usage: ze-demo entrypoint COMMAND [ARG...]")
			break
		}
		err = runContainerEntrypoint(args[1:], stdout, stderr)
	case "shell":
		if len(args) != 1 {
			err = errors.New("usage: ze-demo shell")
			break
		}
		err = execDemoShell()
	default:
		err = fmt.Errorf("unknown ze-demo action %q", args[0])
	}
	if err == nil {
		return 0
	}
	_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
	return 1
}

func printCard(output io.Writer, demo, phase string) error {
	text, ok := cards[demo+":"+phase]
	if !ok {
		return fmt.Errorf("no card for %s %s", demo, phase)
	}
	_, err := fmt.Fprint(output, "\x1b[H\x1b[2J", text)
	return err
}

func demoRoot() string {
	if value := os.Getenv("ZE_DEMO_ROOT"); value != "" {
		return value
	}
	return "/src"
}

func demoStateRoot() string {
	if value := os.Getenv("ZE_DEMO_STATE_ROOT"); value != "" {
		return value
	}
	return filepath.Join(demoRoot(), "tmp", "terminal-demos", "state")
}

// demoTree is the directory every demo lives under, and the base a tape's own
// relative paths resolve against: the shared common.tape and the mounted
// artifacts directory both sit here.
func demoTree() string           { return filepath.Join(demoRoot(), "demos", "terminal") }
func demoDir(id string) string   { return filepath.Join(demoTree(), id) }
func demoState(id string) string { return filepath.Join(demoStateRoot(), id) }
func demoBinary(name string) string {
	return filepath.Join(demoRoot(), "tmp", "terminal-demos", "bin", name)
}

func demoEnvironment() []string {
	home := filepath.Join(demoRoot(), "tmp", "terminal-demos", "home")
	values := map[string]string{
		"HOME":            home,
		"XDG_CONFIG_HOME": filepath.Join(home, ".config"),
		"XDG_DATA_HOME":   filepath.Join(home, ".local", "share"),
		"XDG_RUNTIME_DIR": filepath.Join(demoRoot(), "tmp", "terminal-demos", "runtime"),
		"LANG":            "C.UTF-8", "LC_ALL": "C.UTF-8", "TZ": "UTC", "TERM": "xterm-256color", "PS1": "$ ",
	}
	values["PATH"] = filepath.Dir(demoBinary("ze")) + ":" + os.Getenv("PATH")
	environ := os.Environ()
	for key, value := range values {
		environ = setEnv(environ, key, value)
	}
	return environ
}

func setEnv(environ []string, key, value string) []string {
	prefix := key + "="
	for index := range environ {
		if strings.HasPrefix(environ[index], prefix) {
			environ[index] = prefix + value
			return environ
		}
	}
	return append(environ, prefix+value)
}

func execDemoShell() error {
	environ := demoEnvironment()
	for _, directory := range []string{envValue(environ, "HOME"), envValue(environ, "XDG_CONFIG_HOME"), envValue(environ, "XDG_DATA_HOME"), envValue(environ, "XDG_RUNTIME_DIR")} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return err
		}
	}
	if err := os.Chdir(demoRoot()); err != nil {
		return err
	}
	return crashlog.Exec("/bin/bash", []string{"bash", "--noprofile", "--norc", "-i"}, environ)
}

func envValue(environ []string, key string) string {
	prefix := key + "="
	for _, entry := range environ {
		if value, ok := strings.CutPrefix(entry, prefix); ok {
			return value
		}
	}
	return ""
}

type commandOptions struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	env    []string
	dir    string
	// deadline bounds a runCommand call. Zero selects commandDeadline; a
	// caller whose command legitimately runs longer names its own bound.
	deadline time.Duration
}

// commandDeadline bounds one runCommand call whose caller names no deadline.
// Every lab command a validator runs to completion answers in seconds, so two
// minutes only ever ends a command that hangs.
const commandDeadline = 2 * time.Minute

// errCommandDeadline marks a command that runCommand stopped at its deadline.
// A poll that retries a failing command reads it to tell a hang, which the
// next attempt repeats, from a lab that is not ready yet.
var errCommandDeadline = errors.New("command stopped at its deadline")

// newProcess builds a process with no deadline: a daemon the lab starts, or a
// foreground run whose caller owns its lifetime.
func newProcess(name string, args []string, options commandOptions) *exec.Cmd {
	return newProcessContext(context.Background(), name, args, options)
}

func newProcessContext(ctx context.Context, name string, args []string, options commandOptions) *exec.Cmd {
	process := exec.CommandContext(ctx, name, args...) //nolint:gosec // names and arguments come from the closed demo action table
	process.Stdin = options.stdin
	process.Stdout = options.stdout
	process.Stderr = options.stderr
	process.Env = options.env
	process.Dir = options.dir
	return process
}

// runCommand runs one command to completion and answers its output. It stops
// a command that passes its deadline, and names that command in the error, so
// a hung `ze cli` inside a poll cannot stall the lab until the container's own
// one-hour limit removes it with no message.
func runCommand(name string, args []string, options commandOptions) ([]byte, error) {
	running, err := beginCommand(name, args, options)
	if err != nil {
		return nil, err
	}
	return running.wait()
}

// runningCommand is a command beginCommand started and its deadline. The
// caller MUST call wait exactly once: wait reaps the process and releases the
// deadline. It is not safe for concurrent use.
type runningCommand struct {
	process  *exec.Cmd
	output   *bytes.Buffer
	expired  func() bool
	release  context.CancelFunc
	command  string
	deadline time.Duration
}

// beginCommand starts one command under its deadline and answers before the
// command ends, so a caller can observe the lab while the command runs. The
// caller MUST call wait on the answer.
func beginCommand(name string, args []string, options commandOptions) (*runningCommand, error) {
	var output bytes.Buffer
	if options.stdout == nil {
		options.stdout = &output
	}
	if options.stderr == nil {
		options.stderr = &output
	}
	deadline := options.deadline
	if deadline == 0 {
		deadline = commandDeadline
	}
	command := strings.Join(append([]string{name}, args...), " ")

	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	process := newProcessContext(ctx, name, args, options)
	// A child that outlives the stopped command keeps the output pipe open,
	// so wait would block on it without this bound.
	process.WaitDelay = time.Second
	if err := process.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("%s: %w", command, err)
	}
	expired := func() bool { return errors.Is(ctx.Err(), context.DeadlineExceeded) }
	return &runningCommand{process: process, output: &output, expired: expired, release: cancel, command: command, deadline: deadline}, nil
}

// wait MUST be called once after beginCommand. It answers the output the
// command wrote where the caller named no writer of its own.
func (running *runningCommand) wait() ([]byte, error) {
	defer running.release()
	err := running.process.Wait()
	if err == nil {
		return running.output.Bytes(), nil
	}
	if running.expired() {
		return running.output.Bytes(), fmt.Errorf("%w: %q did not finish within %s and was stopped, so the demo check that ran it has no answer.\nOutput so far:\n%s\nRead the lab logs under %s to see what it waited for, then run the check again:\n./le site terminal-demo check validation",
			errCommandDeadline, running.command, running.deadline, running.output.String(), demoStateRoot())
	}
	return running.output.Bytes(), fmt.Errorf("%s: %w\n%s", running.command, err, running.output.String())
}

func startCommand(name string, args, environ []string, logPath string) (int, error) {
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec // the path comes from the closed demo scenario table
	if err != nil {
		return 0, err
	}
	process := newProcess(name, args, commandOptions{stdout: log, stderr: log, env: environ})
	if err := process.Start(); err != nil {
		_ = log.Close()
		return 0, err
	}
	if err := log.Close(); err != nil {
		return 0, err
	}
	return process.Process.Pid, nil
}

func writePIDs(path string, pids []int) error {
	var text textbuf.Buffer
	for _, pid := range pids {
		fmt.Fprintf(&text, "%d\n", pid)
	}
	return os.WriteFile(path, []byte(text.String()), 0o600)
}

func stopPIDs(path string) {
	data, err := os.ReadFile(path) //nolint:gosec // the path comes from the closed demo scenario table
	if err == nil {
		for field := range strings.FieldsSeq(string(data)) {
			pid, parseErr := strconv.Atoi(field)
			if parseErr == nil {
				_ = syscall.Kill(pid, syscall.SIGTERM)
			}
		}
	}
	_ = os.Remove(path)
}

func waitForFileText(path, expected string, attempts int) error {
	var data []byte
	for range attempts {
		data, _ = os.ReadFile(path) //nolint:gosec // the path comes from the closed demo scenario table
		if bytes.Contains(data, []byte(expected)) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %q in %s\n%s", expected, path, data)
}

// waitForCommandText runs fn until it succeeds with an answer that contains
// expected, compared byte for byte. The comparison is case-sensitive on
// purpose: folded, the link flag `UP` was also found in `group default`, which
// every link prints whatever its state. A command stopped at its deadline ends
// the wait at once, because the next attempt would hang the same way.
func waitForCommandText(attempts int, expected string, fn func() (string, error)) (string, error) {
	var output string
	var err error
	for range attempts {
		output, err = fn()
		if err == nil {
			if strings.Contains(output, expected) {
				return output, nil
			}
		}
		if errors.Is(err, errCommandDeadline) {
			return output, err
		}
		// sleep(poll-interval): the next attempt reads the command's answer again.
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		return output, fmt.Errorf("timeout waiting for %q: %w\n%s", expected, err, output)
	}
	if strings.TrimSpace(output) == "" {
		return output, fmt.Errorf("timeout waiting for %q: the command ran and printed nothing on its last attempt", expected)
	}
	return output, fmt.Errorf("timeout waiting for %q: the command ran, and its last answer does not hold it:\n%s", expected, output)
}

// awaitCheck runs check until it answers nil, at most attempts times, 100
// milliseconds apart, and answers the last error otherwise. A command stopped
// at its deadline ends the wait at once, because the next attempt would hang
// the same way: that error comes back as it is, for the caller to pass on
// unwrapped. attempts MUST be at least one.
func awaitCheck(attempts int, check func() error) error {
	var last error
	for range attempts {
		last = check()
		if last == nil {
			return nil
		}
		if errors.Is(last, errCommandDeadline) {
			return last
		}
		// sleep(poll-interval): the next attempt reads the live state again.
		time.Sleep(100 * time.Millisecond)
	}
	return last
}

// hungCommand answers the first of errs that marks a command stopped at its
// deadline, and nil when none does. A poll that otherwise ignores a failed
// read passes its errors here, so a hung command ends the poll at once.
func hungCommand(errs ...error) error {
	for _, err := range errs {
		if errors.Is(err, errCommandDeadline) {
			return err
		}
	}
	return nil
}

func waitPort(address, name string, attempts int) error {
	for range attempts {
		dialer := net.Dialer{Timeout: 100 * time.Millisecond}
		connection, err := dialer.DialContext(context.Background(), "tcp", address)
		if err == nil {
			if err := connection.Close(); err != nil {
				return err
			}
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s to listen on %s", name, address)
}

func runZe(args, environ []string, input io.Reader) (string, error) {
	output, err := runCommand("ze", args, commandOptions{stdin: input, env: environ})
	return string(output), err
}

func cli(environ []string, text string) (string, error) {
	return runZe([]string{commandCLI, "-c", text}, environ, nil)
}
