package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

func init() {
	Register("ui/cli-format-default", uiDriver(cliFormatDefault))
}

type cliFormatDefaultProcess struct {
	cmd    *exec.Cmd
	stdout bytes.Buffer
	stderr bytes.Buffer
	done   chan struct{}
	mu     sync.Mutex
	err    error
}

func cliFormatDefault(ctx context.Context) error {
	passwordCode, passwordHash, passwordErr := cliFormatDefaultRun(ctx, os.Environ(), "secret\n", "passwd")
	if passwordCode != 0 {
		return fmt.Errorf("ze passwd exit=%d: %s%s", passwordCode, passwordHash, passwordErr)
	}
	passwordHash = strings.TrimSpace(passwordHash)

	config := fmt.Sprintf(`environment {
    cli {
        format {
            default table;
        }
    }
}

bgp {
    router-id 192.0.2.254
    session {
        asn {
            local 65000
        }
    }
    peer peer1 {
        connection {
            remote {
                ip 192.0.2.1
            }
            local {
                ip 127.0.0.1
            }
        }
        session {
            asn {
                remote 65001
            }
        }
    }
}

system {
    authentication {
        user ci {
            password "%s"
            profile [ admin ]
        }
    }
}
`, passwordHash)
	work, err := os.MkdirTemp("", "ze-ui-cli-format-default-")
	if err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	defer os.RemoveAll(work) //nolint:errcheck // fixture cleanup
	configPath := filepath.Join(work, "format.conf")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		return fmt.Errorf("write format.conf: %w", err)
	}

	sshAddressFile := filepath.Join(work, "ssh.addr")
	readyFile := filepath.Join(work, "ready")
	bgpPort, err := uiFreeTCPPort()
	if err != nil {
		return err
	}
	daemonEnv := cliFormatDefaultEnvironment(os.Environ(), []string{
		"ZE_SSH_EPHEMERAL=" + sshAddressFile,
		"ZE_READY_FILE=" + readyFile,
		"ZE_CONFIG_DIR=" + work,
		"ze_test_bgp_port=" + strconv.Itoa(bgpPort),
	}, "ZE_CLI_FORMAT")

	daemon, err := cliFormatDefaultStart(ctx, work, daemonEnv, "-f", configPath)
	if err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	defer daemon.stop()

	// Both files are startup barriers. Poll exactly 200 times, with 100 ms
	// between unsuccessful attempts, rather than relying on a guessed sleep.
	err = cliFormatDefaultPoll(ctx, 200, 100*time.Millisecond, func() (bool, error) {
		if exited, waitErr := daemon.exited(); exited {
			return false, fmt.Errorf("daemon exited early: %w\nstdout:\n%s\nstderr:\n%s", waitErr, daemon.stdout.String(), daemon.stderr.String())
		}
		if _, err := os.Stat(sshAddressFile); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return false, nil
			}
			return false, fmt.Errorf("stat ssh.addr: %w", err)
		}
		if _, err := os.Stat(readyFile); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return false, nil
			}
			return false, fmt.Errorf("stat ready: %w", err)
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("daemon did not become ready: %w", err)
	}

	addressBytes, err := os.ReadFile(sshAddressFile) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return fmt.Errorf("read ssh.addr: %w", err)
	}
	host, port, err := net.SplitHostPort(strings.TrimSpace(string(addressBytes)))
	if err != nil {
		return fmt.Errorf("parse ssh.addr: %w", err)
	}
	cliEnv := cliFormatDefaultEnvironment(os.Environ(), []string{
		"ZE_SSH_HOST=" + host,
		"ZE_SSH_PORT=" + port,
		"ZE_SSH_USERNAME=ci",
		"ZE_SSH_PASSWORD=secret",
		"ZE_CONFIG_DIR=" + work,
	}, "ZE_CLI_FORMAT")

	// 1. AC-4: an unformatted command uses the daemon's committed table default.
	code, out, stderr := cliFormatDefaultRun(ctx, cliEnv, "", "cli", "-c", "show version")
	if code != 0 {
		return fmt.Errorf("ze cli -c \"show version\" exit=%d: %s%s", code, out, stderr)
	}
	if !cliFormatDefaultIsTable(out) {
		return fmt.Errorf("the committed `cli format default table` did not reach `ze cli -c`: %q", out)
	}
	if !strings.Contains(out, "version") {
		return fmt.Errorf("the answer lost its field name: %q", out)
	}

	// 2. AC-5: an explicit machine format overrides the configured default.
	code, out, stderr = cliFormatDefaultRun(ctx, cliEnv, "", "cli", "-c", "show version", "--format", "json")
	if code != 0 {
		return fmt.Errorf("--format json exit=%d: %s%s", code, out, stderr)
	}
	if cliFormatDefaultIsTable(out) {
		return fmt.Errorf("--format json was overridden by the configured default: %q", out)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &object); err != nil {
		return fmt.Errorf("--format json did not answer JSON (%w): %q", err, out)
	}
	if _, ok := object["version"]; !ok {
		return fmt.Errorf("--format json answered JSON without the field: %q", out)
	}

	// 3. AC-6: the daemon honors an explicit YAML pipe.
	code, out, stderr = cliFormatDefaultRun(ctx, cliEnv, "", "cli", "-c", "show version | yaml")
	if code != 0 {
		return fmt.Errorf("| yaml exit=%d: %s%s", code, out, stderr)
	}
	if cliFormatDefaultIsTable(out) {
		return fmt.Errorf("an explicit | yaml lost to the configured default: %q", out)
	}
	if !strings.HasPrefix(out, "version:") {
		return fmt.Errorf("| yaml did not answer YAML: %q", out)
	}

	// 4. A format in the command outranks the flag.
	code, out, stderr = cliFormatDefaultRun(ctx, cliEnv, "", "cli", "-c", "show version | yaml", "--format", "json")
	if code != 0 {
		return fmt.Errorf("| yaml with --format json exit=%d: %s%s", code, out, stderr)
	}
	if !strings.HasPrefix(out, "version:") {
		return fmt.Errorf("--format overrode an explicit | yaml: %q", out)
	}

	// 5. AC-7: an emptied row answer remains visibly nothing-to-report.
	code, out, stderr = cliFormatDefaultRun(ctx, cliEnv, "", "cli", "-c", "show bgp peer list | match zzz-no-such-token")
	if code != 0 {
		return fmt.Errorf("an emptied answer exit=%d: %s%s", code, out, stderr)
	}
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return errors.New("an emptied answer printed nothing, so a human cannot tell it ran")
	}
	if trimmed != "OK" && trimmed != "(empty)" {
		return fmt.Errorf("an emptied answer must say nothing-to-report, got %q", out)
	}

	// 6. The machine form contains no human OK marker and carries no rows.
	code, out, stderr = cliFormatDefaultRun(ctx, cliEnv, "", "cli", "-c", "show bgp peer list | match zzz-no-such-token | json")
	if code != 0 {
		return fmt.Errorf("an emptied answer in json exit=%d: %s%s", code, out, stderr)
	}
	if strings.Contains(out, "OK") {
		return fmt.Errorf("a machine format must not receive OK: %q", out)
	}
	if strings.TrimSpace(out) != "" {
		var parsed any
		if err := json.Unmarshal([]byte(out), &parsed); err != nil {
			return fmt.Errorf("the emptied answer was not JSON (%w): %q", err, out)
		}
		peers := parsed
		if parsedObject, ok := parsed.(map[string]any); ok {
			if value, found := parsedObject["peers"]; found {
				peers = value
			} else {
				peers = map[string]any{}
			}
		}
		count, err := cliFormatDefaultLength(peers)
		if err != nil {
			return fmt.Errorf("the emptied answer has no countable rows: %q: %w", out, err)
		}
		if count != 0 {
			return fmt.Errorf("the emptied answer still carries rows: %q", out)
		}
	}

	// 7. An unknown format is refused and named in stderr.
	code, out, stderr = cliFormatDefaultRun(ctx, cliEnv, "", "cli", "-c", "show version", "--format", "bogus")
	if code != 1 {
		return fmt.Errorf("--format bogus exit=%d, want 1: %s%s", code, out, stderr)
	}
	if !strings.Contains(stderr, "bogus") {
		return fmt.Errorf("the refusal did not name the rejected format: %q", stderr)
	}

	// 8. AC-9: raw returns the dispatcher's JSON despite the table default.
	code, out, stderr = cliFormatDefaultRun(ctx, cliEnv, "", "cli", "-c", "show version | raw")
	if code != 0 {
		return fmt.Errorf("| raw exit=%d: %s%s", code, out, stderr)
	}
	if cliFormatDefaultIsTable(out) {
		return fmt.Errorf("| raw lost to the configured default: %q", out)
	}
	object = nil
	if err := json.Unmarshal([]byte(out), &object); err != nil {
		return fmt.Errorf("| raw did not answer the dispatcher JSON (%w): %q", err, out)
	}
	if _, ok := object["version"]; !ok {
		return fmt.Errorf("| raw answered JSON without the field: %q", out)
	}

	// 9. AC-9 and AC-10: completion parses the same channel and keeps every selector.
	code, out, stderr = cliFormatDefaultRun(ctx, cliEnv, "", "completion", "peers")
	if code != 0 {
		return fmt.Errorf("ze completion peers exit=%d: %s%s", code, out, stderr)
	}
	for _, selector := range []string{addrTestNet1First, peerNameOne, "as65001"} {
		if !strings.Contains(out, selector) {
			return fmt.Errorf("completion lost the %s selector to the configured format: %q", selector, out)
		}
	}

	fmt.Println("OK")
	return nil
}

func cliFormatDefaultRun(ctx context.Context, env []string, stdin string, args ...string) (int, string, string) {
	cmd := exec.CommandContext(ctx, "ze", args...) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Env = env
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return 0, stdout.String(), stderr.String()
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return exitErr.ExitCode(), stdout.String(), stderr.String()
	}
	if stderr.Len() != 0 && !strings.HasSuffix(stderr.String(), "\n") {
		stderr.WriteByte('\n')
	}
	stderr.WriteString(err.Error())
	return -1, stdout.String(), stderr.String()
}

func cliFormatDefaultStart(ctx context.Context, dir string, env []string, args ...string) (*cliFormatDefaultProcess, error) {
	process := &cliFormatDefaultProcess{done: make(chan struct{})}
	process.cmd = exec.CommandContext(ctx, "ze", args...) //nolint:gosec // the fixture chooses the program and its arguments
	process.cmd.Dir = dir
	process.cmd.Env = env
	process.cmd.Stdout = &process.stdout
	process.cmd.Stderr = &process.stderr
	if err := process.cmd.Start(); err != nil {
		return nil, err
	}
	go func() {
		err := process.cmd.Wait()
		process.mu.Lock()
		process.err = err
		process.mu.Unlock()
		close(process.done)
	}()
	return process, nil
}

func (process *cliFormatDefaultProcess) exited() (bool, error) {
	select {
	case <-process.done:
		process.mu.Lock()
		defer process.mu.Unlock()
		return true, process.err
	default:
		return false, nil
	}
}

func (process *cliFormatDefaultProcess) stop() {
	if exited, _ := process.exited(); exited {
		return
	}
	_ = process.cmd.Process.Signal(syscall.SIGTERM)
	if process.wait(5 * time.Second) {
		return
	}
	_ = process.cmd.Process.Kill()
	_ = process.wait(5 * time.Second)
}

func (process *cliFormatDefaultProcess) wait(timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-process.done:
		return true
	case <-timer.C:
		return false
	}
}

func cliFormatDefaultPoll(ctx context.Context, attempts int, delay time.Duration, condition func() (bool, error)) error {
	for attempt := range attempts {
		ready, err := condition()
		if err != nil {
			return err
		}
		if ready {
			return nil
		}
		if attempt+1 == attempts {
			break
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
	return fmt.Errorf("condition was false after %d attempts", attempts)
}

func cliFormatDefaultEnvironment(base, additions []string, removals ...string) []string {
	remove := make(map[string]struct{}, len(removals)+len(additions))
	for _, key := range removals {
		remove[key] = struct{}{}
	}
	for _, entry := range additions {
		key, _, _ := strings.Cut(entry, "=")
		remove[key] = struct{}{}
	}
	env := make([]string, 0, len(base)+len(additions))
	for _, entry := range base {
		key, _, _ := strings.Cut(entry, "=")
		if _, excluded := remove[key]; !excluded {
			env = append(env, entry)
		}
	}
	return append(env, additions...)
}

func cliFormatDefaultIsTable(text string) bool {
	return strings.ContainsRune(text, '┌') || strings.ContainsRune(text, '│')
}

func cliFormatDefaultLength(value any) (int, error) {
	if value == nil {
		return 0, nil
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice, reflect.String:
		return reflected.Len(), nil
	default:
		return 0, fmt.Errorf("JSON value of type %T has no length", value)
	}
}
