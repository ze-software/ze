package terminaldemo

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
)

//go:embed cards.json
var cardData []byte

var cards map[string]string

func init() {
	if err := json.Unmarshal(cardData, &cards); err != nil {
		panic(fmt.Sprintf("invalid embedded demo cards: %v", err))
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
	return syscall.Exec("/bin/bash", []string{"bash", "--noprofile", "--norc", "-i"}, environ) //nolint:gosec // a fixed interactive shell inside the demo container
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
}

func newProcess(name string, args []string, options commandOptions) *exec.Cmd {
	process := exec.CommandContext(context.Background(), name, args...) //nolint:gosec // names and arguments come from the closed demo action table
	process.Stdin = options.stdin
	process.Stdout = options.stdout
	process.Stderr = options.stderr
	process.Env = options.env
	process.Dir = options.dir
	return process
}

func runCommand(name string, args []string, options commandOptions) ([]byte, error) {
	var output bytes.Buffer
	if options.stdout == nil {
		options.stdout = &output
	}
	if options.stderr == nil {
		options.stderr = &output
	}
	process := newProcess(name, args, options)
	if err := process.Run(); err != nil {
		return output.Bytes(), fmt.Errorf("%s: %w\n%s", strings.Join(append([]string{name}, args...), " "), err, output.String())
	}
	return output.Bytes(), nil
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
	var text strings.Builder
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

func waitForCommandText(attempts int, expected string, fn func() (string, error)) (string, error) {
	var output string
	var err error
	for range attempts {
		output, err = fn()
		if err == nil && strings.Contains(strings.ToLower(output), strings.ToLower(expected)) {
			return output, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return output, fmt.Errorf("timeout waiting for %q: %w\n%s", expected, err, output)
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
