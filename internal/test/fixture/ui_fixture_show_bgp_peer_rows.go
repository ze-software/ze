package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func init() {
	Register("ui/show-bgp-peer-rows", uiDriver(showBGPPeerRows))
}

type peerRowsFixture struct{}

type dispatchResult struct {
	code   int
	stdout string
	stderr string
	err    error
}

// Dispatch runs a compiled product command and captures both output streams.
func (peerRowsFixture) Dispatch(ctx context.Context, argv, env []string, stdin *string) dispatchResult {
	if len(argv) == 0 {
		return dispatchResult{code: -1, err: errors.New("empty command")}
	}

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // the fixture chooses the program and its arguments
	if env != nil {
		cmd.Env = env
	}
	if stdin == nil {
		cmd.Stdin = os.Stdin
	} else {
		cmd.Stdin = strings.NewReader(*stdin)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	code := -1
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	}

	return dispatchResult{
		code:   code,
		stdout: stdout.String(),
		stderr: stderr.String(),
		err:    err,
	}
}

// Observe turns a failed fixture observation into an error.
func (peerRowsFixture) Observe(condition bool, format string, args ...any) error {
	if condition {
		return nil
	}
	return fmt.Errorf(format, args...)
}

// Poll makes exactly attempts observations, waiting delay only between them.
func (peerRowsFixture) Poll(
	ctx context.Context,
	attempts int,
	delay time.Duration,
	observe func() (bool, error),
) (bool, error) {
	for attempt := range attempts {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
		}

		ready, err := observe()
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

type uiShowBgpPeerRowsDaemonProcess struct {
	cmd     *exec.Cmd
	done    <-chan error
	stdout  *bytes.Buffer
	stderr  *bytes.Buffer
	exited  bool
	waitErr error
}

func uiShowBgpPeerRowsStartDaemon(ctx context.Context, dir string, env []string) (*uiShowBgpPeerRowsDaemonProcess, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.CommandContext(ctx, "ze", "-f", "rows.conf")
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start daemon: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	return &uiShowBgpPeerRowsDaemonProcess{
		cmd:    cmd,
		done:   done,
		stdout: &stdout,
		stderr: &stderr,
	}, nil
}

func (d *uiShowBgpPeerRowsDaemonProcess) checkExited() bool {
	if d.exited {
		return true
	}
	select {
	case d.waitErr = <-d.done:
		d.exited = true
		return true
	default:
		return false
	}
}

func (d *uiShowBgpPeerRowsDaemonProcess) wait(timeout time.Duration) bool {
	if d.checkExited() {
		return true
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case d.waitErr = <-d.done:
		d.exited = true
		return true
	case <-timer.C:
		return d.checkExited()
	}
}

func (d *uiShowBgpPeerRowsDaemonProcess) Stop() error {
	if d.checkExited() {
		return nil
	}

	signalErr := d.cmd.Process.Signal(syscall.SIGTERM)
	if errors.Is(signalErr, os.ErrProcessDone) {
		signalErr = nil
	}
	if d.wait(5 * time.Second) {
		return signalErr
	}

	killErr := d.cmd.Process.Kill()
	if errors.Is(killErr, os.ErrProcessDone) {
		killErr = nil
	}
	if !d.wait(5 * time.Second) {
		return errors.New("daemon did not exit within 5s after kill")
	}
	if signalErr != nil {
		return signalErr
	}
	return killErr
}

func showBGPPeerRows(ctx context.Context) (retErr error) {
	fixture := peerRowsFixture{}

	secretInput := "secret\n"
	password := fixture.Dispatch(ctx, []string{"ze", "passwd"}, nil, &secretInput)
	if err := fixture.Observe(
		password.code == 0,
		"ze passwd exit=%d: %s%s: %v",
		password.code,
		password.stdout,
		password.stderr,
		password.err,
	); err != nil {
		return err
	}
	passwordHash := strings.TrimSpace(password.stdout)

	config := `bgp {
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
    peer peer2 {
        connection {
            remote {
                ip 192.0.2.2
            }
            local {
                ip 127.0.0.1
            }
        }
        session {
            asn {
                remote 65002
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
	workingDirectory, err := os.MkdirTemp("", "ze-ui-show-bgp-peer-rows-")
	if err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	defer os.RemoveAll(workingDirectory) //nolint:errcheck // fixture cleanup
	configPath := filepath.Join(workingDirectory, "rows.conf")
	sshAddressFile := filepath.Join(workingDirectory, "ssh.addr")
	readyFile := filepath.Join(workingDirectory, "ready")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		return fmt.Errorf("write rows.conf: %w", err)
	}

	bgpPort, err := uiFreeTCPPort()
	if err != nil {
		return err
	}
	daemonEnv := uiShowBgpPeerRowsEnvironment(map[string]string{
		envSSHEphemeral: sshAddressFile,
		envReadyFile:    readyFile,
		envConfigDir:    workingDirectory,
		envTestBGPPort:  strconv.Itoa(bgpPort),
	})
	daemon, err := uiShowBgpPeerRowsStartDaemon(ctx, workingDirectory, daemonEnv)
	if err != nil {
		return err
	}
	cleaned := false
	defer func() {
		if cleaned {
			return
		}
		if err := daemon.Stop(); err != nil && retErr == nil {
			retErr = err
		}
	}()

	ready, err := fixture.Poll(ctx, 200, 100*time.Millisecond, func() (bool, error) {
		if daemon.checkExited() {
			return false, fmt.Errorf(
				"daemon exited early\nstdout:\n%s\nstderr:\n%s",
				daemon.stdout.String(),
				daemon.stderr.String(),
			)
		}
		return uiShowBgpPeerRowsPathExists(sshAddressFile) && uiShowBgpPeerRowsPathExists(readyFile), nil
	})
	if err != nil {
		return err
	}
	if err := fixture.Observe(ready, "daemon did not become ready"); err != nil {
		return err
	}

	addressBytes, err := os.ReadFile(sshAddressFile) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return fmt.Errorf("read ssh.addr: %w", err)
	}
	address := strings.TrimSpace(string(addressBytes))
	colon := strings.LastIndexByte(address, ':')
	if err := fixture.Observe(
		colon >= 0 && colon+1 < len(address),
		"invalid SSH listener address %q",
		address,
	); err != nil {
		return err
	}
	host, port := address[:colon], address[colon+1:]

	cliEnv := uiShowBgpPeerRowsEnvironment(map[string]string{
		envSSHHost:     host,
		envSSHPort:     port,
		envSSHUsername: "ci",
		envSSHPassword: valueSecret,
		envConfigDir:   workingDirectory,
	})

	tests := []struct {
		commandTemplate string
		addressKey      string
	}{
		{commandTemplate: "show bgp peer %s statistics", addressKey: fieldAddress},
		{commandTemplate: "show bgp peer %s capabilities", addressKey: fieldPeer},
	}

	for _, test := range tests {
		severalCommand := fmt.Sprintf(test.commandTemplate, "*")
		oneCommand := fmt.Sprintf(test.commandTemplate, "192.0.2.1")

		several, err := rowsOf(ctx, fixture, cliEnv, severalCommand)
		if err != nil {
			return err
		}
		one, err := rowsOf(ctx, fixture, cliEnv, oneCommand)
		if err != nil {
			return err
		}

		if err := fixture.Observe(
			len(several) == 2,
			"%s matched %d peers, want 2",
			severalCommand,
			len(several),
		); err != nil {
			return err
		}

		severalAddresses, err := rowStrings(several, test.addressKey)
		if err != nil {
			return fmt.Errorf("%s lost a peer row: %v: %w", severalCommand, several, err)
		}
		sort.Strings(severalAddresses)
		if err := fixture.Observe(
			uiShowBgpPeerRowsEqualStrings(severalAddresses, []string{addrTestNet1First, addrTestNet1Second}),
			"%s lost a peer row: %v",
			severalCommand,
			several,
		); err != nil {
			return err
		}

		if err := fixture.Observe(
			len(one) == 1,
			"%s answered %d rows, want 1",
			oneCommand,
			len(one),
		); err != nil {
			return err
		}
		oneAddress, ok := one[0][test.addressKey].(string)
		if err := fixture.Observe(
			ok && oneAddress == addrTestNet1First,
			"%s named the wrong peer: %v",
			test.commandTemplate,
			one,
		); err != nil {
			return err
		}

		oneKeys := uiShowBgpPeerRowsSortedKeys(one[0])
		severalKeys := uiShowBgpPeerRowsSortedKeys(several[0])
		if err := fixture.Observe(
			uiShowBgpPeerRowsEqualStrings(oneKeys, severalKeys),
			"one matched peer answers different fields: %v vs %v",
			oneKeys,
			severalKeys,
		); err != nil {
			return err
		}

		severalCount, err := countOf(ctx, fixture, cliEnv, severalCommand)
		if err != nil {
			return err
		}
		if err := fixture.Observe(
			severalCount == 2,
			"`| count` over two peers: %q",
			severalCommand,
		); err != nil {
			return err
		}

		oneCount, err := countOf(ctx, fixture, cliEnv, oneCommand)
		if err != nil {
			return err
		}
		if err := fixture.Observe(
			oneCount == 1,
			"`| count` over one peer: %q",
			oneCommand,
		); err != nil {
			return err
		}

		firstText, err := cli(ctx, fixture, cliEnv, severalCommand+" | first 1 | json compact")
		if err != nil {
			return err
		}
		var answered any
		if err := json.Unmarshal([]byte(firstText), &answered); err != nil {
			return fmt.Errorf("decode first-row answer %q: %w", firstText, err)
		}
		firstValue := answered
		if envelope, ok := answered.(map[string]any); ok {
			firstValue = envelope["peers"]
		}
		first, err := rowsFromValue(firstValue)
		if err != nil {
			return fmt.Errorf("`| first 1` did not answer rows: %v: %w", answered, err)
		}
		if err := fixture.Observe(
			len(first) == 1,
			"`| first 1` did not answer one row: %v",
			answered,
		); err != nil {
			return err
		}
		firstAddress, ok := first[0][test.addressKey].(string)
		if err := fixture.Observe(
			ok && (firstAddress == addrTestNet1First || firstAddress == addrTestNet1Second),
			"`| first 1` answered a peer that is not configured: %v",
			answered,
		); err != nil {
			return err
		}
	}

	if err := daemon.Stop(); err != nil {
		cleaned = true
		return err
	}
	cleaned = true

	_, err = fmt.Fprintln(os.Stdout, "OK")
	return err
}

func cli(ctx context.Context, fixture peerRowsFixture, env []string, command string) (string, error) {
	result := fixture.Dispatch(ctx, []string{"ze", areaCLI, "-c", command}, env, nil)
	if err := fixture.Observe(
		result.code == 0,
		"%s exit=%d: %s%s",
		command,
		result.code,
		result.stdout,
		result.stderr,
	); err != nil {
		return "", err
	}
	return result.stdout, nil
}

func rowsOf(ctx context.Context, fixture peerRowsFixture, env []string, command string) ([]map[string]any, error) {
	text, err := cli(ctx, fixture, env, command+" | json compact")
	if err != nil {
		return nil, err
	}

	var parsed any
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		return nil, fmt.Errorf("decode %s answer %q: %w", command, text, err)
	}
	rows, err := rowsFromValue(parsed)
	if err != nil {
		return nil, fmt.Errorf("%s did not answer rows: %q: %w", command, text, err)
	}
	return rows, nil
}

func countOf(ctx context.Context, fixture peerRowsFixture, env []string, command string) (int, error) {
	text, err := cli(ctx, fixture, env, command+" | count")
	if err != nil {
		return 0, err
	}

	var parsed any
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		return 0, fmt.Errorf("decode count answer %q: %w", text, err)
	}
	if envelope, ok := parsed.(map[string]any); ok {
		parsed = envelope["count"]
	}
	return integerValue(parsed)
}

func rowsFromValue(value any) ([]map[string]any, error) {
	values, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("got %T, want array", value)
	}
	rows := make([]map[string]any, len(values))
	for index, value := range values {
		row, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("row %d has type %T, want object", index, value)
		}
		rows[index] = row
	}
	return rows, nil
}

func integerValue(value any) (int, error) {
	switch value := value.(type) {
	case float64:
		return int(value), nil
	case string:
		result, err := strconv.Atoi(value)
		if err != nil {
			return 0, fmt.Errorf("invalid integer %q: %w", value, err)
		}
		return result, nil
	default:
		return 0, fmt.Errorf("count has type %T", value)
	}
}

func rowStrings(rows []map[string]any, key string) ([]string, error) {
	values := make([]string, len(rows))
	for index, row := range rows {
		value, ok := row[key].(string)
		if !ok {
			return nil, fmt.Errorf("row %d field %q has type %T", index, key, row[key])
		}
		values[index] = value
	}
	return values, nil
}

func uiShowBgpPeerRowsSortedKeys(row map[string]any) []string {
	keys := make([]string, 0, len(row))
	for key := range row {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func uiShowBgpPeerRowsEqualStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func uiShowBgpPeerRowsPathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func uiShowBgpPeerRowsEnvironment(overrides map[string]string) []string {
	values := make(map[string]string)
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = value
		}
	}
	maps.Copy(values, overrides)

	env := make([]string, 0, len(values))
	for key, value := range values {
		env = append(env, key+"="+value)
	}
	return env
}
