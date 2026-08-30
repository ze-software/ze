package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"
)

func init() {
	Register("ui/show-column-order-absent-unchanged", uiDriver(showColumnOrderAbsentUnchanged))
}

func showColumnOrderAbsentUnchanged(ctx context.Context) error {
	if err := runShowColumnOrderAbsentUnchanged(ctx); err != nil {
		return err
	}
	_, err := fmt.Fprintln(os.Stdout, "OK")
	return err
}

func runShowColumnOrderAbsentUnchanged(ctx context.Context) (retErr error) {
	code, passwordHash, passwordErr, err := uiShowColumnOrderAbsentUnchangedRunCaptured(ctx, nil, "secret\n", "ze", "passwd")
	if err != nil {
		return fmt.Errorf("run ze passwd: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("ze passwd exit=%d: %s%s", code, passwordHash, passwordErr)
	}
	passwordHash = strings.TrimSpace(passwordHash)

	cwd, err := os.MkdirTemp("", "ze-ui-show-column-order-absent-")
	if err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	defer os.RemoveAll(cwd) //nolint:errcheck // fixture cleanup

	config := `bgp {
    router-id 192.0.2.254
    session {
        asn {
            local 65000
        }
    }
    group transit {
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
}

system {
    authentication {
        user ci {
            password "` + passwordHash + `"
            profile [ admin ]
        }
    }
}
`
	configPath := filepath.Join(cwd, "absent.conf")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		return fmt.Errorf("write absent.conf: %w", err)
	}

	sshAddressFile := filepath.Join(cwd, "ssh.addr")
	readyFile := filepath.Join(cwd, "ready")

	bgpPort, err := uiFreeTCPPort()
	if err != nil {
		return err
	}
	daemonEnv := uiShowColumnOrderAbsentUnchangedReplaceEnv(os.Environ(), map[string]string{
		envSSHEphemeral: sshAddressFile,
		envReadyFile:    readyFile,
		envConfigDir:    cwd,
		envTestBGPPort:  fmt.Sprintf("%d", bgpPort),
	})

	daemon, err := startManagedCommand(ctx, daemonEnv, "ze", "-f", configPath)
	if err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	defer func() {
		if err := daemon.stop(); retErr == nil && err != nil {
			retErr = err
		}
	}()

	ready, err := uiShowColumnOrderAbsentUnchangedPoll(ctx, 200, 100*time.Millisecond, func() (bool, error) {
		select {
		case <-daemon.done:
			return false, fmt.Errorf(
				"daemon exited early\nstdout:\n%s\nstderr:\n%s",
				daemon.stdout.String(), daemon.stderr.String(),
			)
		default:
		}

		_, sshErr := os.Stat(sshAddressFile)
		_, readyErr := os.Stat(readyFile)
		return sshErr == nil && readyErr == nil, nil
	})
	if err != nil {
		return err
	}
	if !ready {
		return errors.New("daemon did not become ready")
	}

	addressBytes, err := os.ReadFile(sshAddressFile) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return fmt.Errorf("read ssh.addr: %w", err)
	}
	address := strings.TrimSpace(string(addressBytes))
	colon := strings.LastIndexByte(address, ':')
	if colon < 0 {
		return fmt.Errorf("invalid SSH address %q", address)
	}
	host, port := address[:colon], address[colon+1:]

	cliEnv := uiShowColumnOrderAbsentUnchangedReplaceEnv(os.Environ(), map[string]string{
		envSSHHost:     host,
		envSSHPort:     port,
		envSSHUsername: "ci",
		envSSHPassword: valueSecret,
		envConfigDir:   cwd,
	})

	code, out, stderr, err := uiShowColumnOrderAbsentUnchangedRunCaptured(
		ctx,
		cliEnv,
		"",
		"ze", "cli", "-c", "show bgp peer 192.0.2.1 detail | text",
	)
	if err != nil {
		return fmt.Errorf("run show bgp peer detail: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("show bgp peer detail exit=%d: %s%s", code, out, stderr)
	}
	keys := recordKeys(out)
	if len(keys) <= 5 {
		return fmt.Errorf("the detail record is too small to say anything: %q", out)
	}
	if !alphabetical(keys) {
		return fmt.Errorf("a command that declares no order rendered out of alphabetical order: %v", keys)
	}

	code, out, stderr, err = uiShowColumnOrderAbsentUnchangedRunCaptured(
		ctx,
		cliEnv,
		"",
		"ze", "cli", "-c", "show bgp | text",
	)
	if err != nil {
		return fmt.Errorf("run show bgp: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("show bgp exit=%d: %s%s", code, out, stderr)
	}
	keys = recordKeys(out)
	if len(keys) <= 3 {
		return fmt.Errorf("the summary record is too small to say anything: %q", out)
	}
	if alphabetical(keys) {
		return fmt.Errorf("the declaring command rendered alphabetically, so this file proves nothing: %v", keys)
	}

	return nil
}

func recordKeys(text string) []string {
	var keys []string
	for line := range strings.SplitSeq(text, "\n") {
		if line == "" {
			continue
		}
		r, _ := utf8.DecodeRuneInString(line)
		if unicode.IsSpace(r) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 0 {
			keys = append(keys, fields[0])
		}
	}
	return keys
}

func alphabetical(values []string) bool {
	ordered := append([]string(nil), values...)
	sort.Strings(ordered)
	if len(values) != len(ordered) {
		return false
	}
	for i := range values {
		if values[i] != ordered[i] {
			return false
		}
	}
	return true
}

func uiShowColumnOrderAbsentUnchangedPoll(
	ctx context.Context,
	attempts int,
	delay time.Duration,
	check func() (bool, error),
) (bool, error) {
	for attempt := range attempts {
		ready, err := check()
		if err != nil || ready {
			return ready, err
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
			return false, ctx.Err()
		case <-timer.C:
		}
	}
	return false, nil
}

func uiShowColumnOrderAbsentUnchangedRunCaptured(
	ctx context.Context,
	env []string,
	stdin, name string,
	args ...string,
) (code int, stdout, stderr string, startErr error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // the fixture chooses the program and its arguments
	if env != nil {
		cmd.Env = env
	}
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	err := cmd.Run()
	stdout, stderr = out.String(), errOut.String()
	if err == nil {
		return 0, stdout, stderr, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return exitErr.ExitCode(), stdout, stderr, nil
	}
	return -1, stdout, stderr, err
}

func uiShowColumnOrderAbsentUnchangedReplaceEnv(base []string, replacements map[string]string) []string {
	result := make([]string, 0, len(base)+len(replacements))
	seen := make(map[string]bool, len(replacements))
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			key = entry
		}
		if value, replace := replacements[key]; replace {
			if !seen[key] {
				result = append(result, key+"="+value)
				seen[key] = true
			}
			continue
		}
		result = append(result, entry)
	}
	for key, value := range replacements {
		if !seen[key] {
			result = append(result, key+"="+value)
		}
	}
	return result
}

type managedCommand struct {
	cmd     *exec.Cmd
	stdout  bytes.Buffer
	stderr  bytes.Buffer
	done    chan struct{}
	waitErr error
}

func startManagedCommand(ctx context.Context, env []string, name string, args ...string) (*managedCommand, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Env = env

	managed := &managedCommand{
		cmd:  cmd,
		done: make(chan struct{}),
	}
	cmd.Stdout = &managed.stdout
	cmd.Stderr = &managed.stderr

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go func() {
		managed.waitErr = cmd.Wait()
		close(managed.done)
	}()
	return managed, nil
}

func (cmd *managedCommand) stop() error {
	select {
	case <-cmd.done:
		return nil
	default:
	}

	if err := cmd.cmd.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("terminate daemon: %w", err)
	}
	if uiShowColumnOrderAbsentUnchangedWaitForDone(cmd.done, 5*time.Second) {
		return nil
	}

	if err := cmd.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("kill daemon: %w", err)
	}
	if !uiShowColumnOrderAbsentUnchangedWaitForDone(cmd.done, 5*time.Second) {
		return errors.New("daemon did not exit after kill")
	}
	return nil
}

func uiShowColumnOrderAbsentUnchangedWaitForDone(done <-chan struct{}, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}
