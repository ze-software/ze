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
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"
)

var bgpSummaryPeerOrder = []string{
	"address", "name", "description", "remote-as", "peer-type",
	"state", "uptime", "state-changed", "last-error",
	"routes-received", "routes-accepted", "routes-sent",
	"updates-received", "updates-sent",
	"keepalives-received", "keepalives-sent",
	"eor-received", "eor-sent",
	"connections-dropped",
}

var bgpSummaryRecordOrder = []string{
	"router-id", "local-as", "uptime",
	"peers-configured", "peers-established",
	"family", "peers-in-family", "peers",
}

func init() {
	Register("ui/show-bgp-summary-column-order", uiDriver(showBGPSummaryColumnOrder))
}

func showBGPSummaryColumnOrder(ctx context.Context) (retErr error) {
	root := os.Getenv("ZE_REPO_ROOT")
	if root == "" {
		return errors.New("ZE_REPO_ROOT is not set")
	}
	ze, err := uiZEBinary(root)
	if err != nil {
		return err
	}
	passwordCode, passwordOut, passwordErr, err := runBGPColumnOrderCommand(ctx, nil, "secret\n", ze, "passwd")
	if err != nil {
		return fmt.Errorf("run ze passwd: %w", err)
	}
	if passwordCode != 0 {
		return fmt.Errorf("ze passwd exit=%d: %s%s", passwordCode, passwordOut, passwordErr)
	}
	passwordHash := strings.TrimSpace(passwordOut)

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
	cwd, err := os.MkdirTemp("", "ze-ui-show-bgp-summary-columns-")
	if err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	defer os.RemoveAll(cwd)
	configPath := filepath.Join(cwd, "summary.conf")
	if err := writeBGPColumnOrderFile(configPath, []byte(config)); err != nil {
		return fmt.Errorf("write summary.conf: %w", err)
	}
	sshAddressFile, err := fullBGPColumnOrderPath(cwd, "ssh.addr")
	if err != nil {
		return err
	}
	readyFile, err := fullBGPColumnOrderPath(cwd, "ready")
	if err != nil {
		return err
	}

	// Leave port 179 alone: the suite runs unprivileged, and a bind failure
	// there takes the daemon down before it writes the ready file.
	bgpPort, err := uiFreeTCPPort()
	if err != nil {
		return err
	}
	daemonEnv := replaceBGPColumnOrderEnv(os.Environ(),
		"ZE_SSH_EPHEMERAL", sshAddressFile,
		"ZE_READY_FILE", readyFile,
		"ZE_CONFIG_DIR", cwd,
		"ze_test_bgp_port", strconv.Itoa(bgpPort),
	)

	var daemonStdout bytes.Buffer
	var daemonStderr bytes.Buffer
	daemon := exec.CommandContext(ctx, ze, "-f", configPath)
	daemon.Dir = cwd
	daemon.Env = daemonEnv
	daemon.Stdout = &daemonStdout
	daemon.Stderr = &daemonStderr
	if err := daemon.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}

	daemonDone := make(chan error, 1)
	go func() {
		daemonDone <- daemon.Wait()
	}()
	defer func() {
		if err := stopBGPColumnOrderDaemon(daemon, daemonDone); retErr == nil && err != nil {
			retErr = err
		}
	}()

	ready, err := pollBGPColumnOrder(ctx, 200, 100*time.Millisecond, func() (bool, error) {
		select {
		case waitErr := <-daemonDone:
			return false, fmt.Errorf(
				"daemon exited early: %v\nstdout:\n%s\nstderr:\n%s",
				waitErr, daemonStdout.String(), daemonStderr.String(),
			)
		default:
		}
		return bgpColumnOrderPathExists(sshAddressFile) && bgpColumnOrderPathExists(readyFile), nil
	})
	if err != nil {
		return err
	}
	if !ready {
		return errors.New("daemon did not become ready")
	}

	addressBytes, err := os.ReadFile(sshAddressFile)
	if err != nil {
		return fmt.Errorf("read ssh.addr: %w", err)
	}
	address := strings.TrimSpace(string(addressBytes))
	colon := strings.LastIndexByte(address, ':')
	if colon < 0 {
		return fmt.Errorf("invalid SSH listener address %q", address)
	}
	host, port := address[:colon], address[colon+1:]

	cliEnv := replaceBGPColumnOrderEnv(os.Environ(),
		"ZE_SSH_HOST", host,
		"ZE_SSH_PORT", port,
		"ZE_SSH_USERNAME", "ci",
		"ZE_SSH_PASSWORD", "secret",
		"ZE_CONFIG_DIR", cwd,
	)

	code, out, stderr, err := runBGPColumnOrderCommand(ctx, cliEnv, "", ze, "cli", "-c", "show bgp | text")
	if err != nil {
		return fmt.Errorf("run show bgp | text: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("show bgp | text exit=%d: %s%s", code, out, stderr)
	}

	// The peer table is the value of the outer record's peers key, so its
	// header shares a line with that key. connections-dropped identifies the
	// header unambiguously because no value in the table carries that text.
	var header []string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "connections-dropped") {
			header = strings.Fields(line)
			break
		}
	}
	if header == nil {
		return fmt.Errorf("no peer table in the answer: %q", out)
	}
	if len(header) != 0 && header[0] == "peers" {
		header = header[1:]
	}

	expectedHeader := presentBGPColumnOrderKeys(bgpSummaryPeerOrder, header)
	if !equalBGPColumnOrderStrings(header, expectedHeader) {
		return fmt.Errorf("peer columns %q, want %q", header, expectedHeader)
	}
	if len(header) == 0 || header[0] != "address" {
		return fmt.Errorf("the peer row must lead with the peer it is about: %q", header)
	}
	if len(header) <= 5 || header[5] != "state" {
		return fmt.Errorf("state must be the sixth column, not buried: %q", header)
	}
	if header[len(header)-1] != "connections-dropped" {
		return fmt.Errorf("the fault counter must come last: %q", header)
	}
	if sortedBGPColumnOrderStrings(header) {
		return fmt.Errorf("the peer columns are still alphabetical: %q", header)
	}

	// The outer record carries its own order because it and the peer rows both
	// hold an uptime key in a different place.
	var record []string
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		first, _ := utf8.DecodeRuneInString(line)
		if unicode.IsSpace(first) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 0 {
			record = append(record, fields[0])
		}
	}
	expectedRecord := presentBGPColumnOrderKeys(bgpSummaryRecordOrder, record)
	if !equalBGPColumnOrderStrings(record, expectedRecord) {
		return fmt.Errorf("summary record keys %q, want %q", record, expectedRecord)
	}
	if sortedBGPColumnOrderStrings(record) {
		return fmt.Errorf("the summary record is still alphabetical: %q", record)
	}

	// A program reads JSON, so the column order must not have reached it. JSON
	// object keys have no order to a parser; this verifies that rendering stayed
	// JSON rather than becoming a table.
	code, out, stderr, err = runBGPColumnOrderCommand(ctx, cliEnv, "", ze, "cli", "-c", "show bgp | json")
	if err != nil {
		return fmt.Errorf("run show bgp | json: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("show bgp | json exit=%d: %s%s", code, out, stderr)
	}
	if strings.Contains(out, "┌") || strings.Contains(out, "│") {
		return fmt.Errorf("| json answered a table: %q", out)
	}
	if !strings.HasPrefix(strings.TrimLeftFunc(out, unicode.IsSpace), "{") {
		return fmt.Errorf("| json did not answer JSON: %q", out)
	}

	fmt.Println("OK")
	return nil
}

func runBGPColumnOrderCommand(ctx context.Context, env []string, stdin, name string, args ...string) (int, string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	if env != nil {
		cmd.Env = env
	}
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return 0, stdout.String(), stderr.String(), nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), stdout.String(), stderr.String(), nil
	}
	return -1, stdout.String(), stderr.String(), err
}

func pollBGPColumnOrder(ctx context.Context, attempts int, delay time.Duration, check func() (bool, error)) (bool, error) {
	for attempt := 0; attempt < attempts; attempt++ {
		ready, err := check()
		if err != nil || ready {
			return ready, err
		}
		if attempt == attempts-1 {
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

func stopBGPColumnOrderDaemon(cmd *exec.Cmd, done <-chan error) error {
	if cmd.ProcessState != nil {
		return nil
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("terminate daemon: %w", err)
	}

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-done:
		return nil
	case <-timer.C:
	}

	if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("kill daemon: %w", err)
	}
	timer.Reset(5 * time.Second)
	select {
	case <-done:
		return nil
	case <-timer.C:
		return errors.New("daemon did not exit after being killed")
	}
}

func writeBGPColumnOrderFile(name string, data []byte) error {
	file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func fullBGPColumnOrderPath(dir, name string) (string, error) {
	path := dir + string(os.PathSeparator) + name
	absolute, err := os.Stat(dir)
	if err != nil {
		return "", fmt.Errorf("stat working directory: %w", err)
	}
	if !absolute.IsDir() {
		return "", fmt.Errorf("working directory %q is not a directory", dir)
	}
	return path, nil
}

func bgpColumnOrderPathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func replaceBGPColumnOrderEnv(base []string, pairs ...string) []string {
	replacements := make(map[string]string, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		replacements[pairs[i]] = pairs[i+1]
	}

	env := make([]string, 0, len(base)+len(replacements))
	for _, entry := range base {
		key := entry
		if equals := strings.IndexByte(entry, '='); equals >= 0 {
			key = entry[:equals]
		}
		if _, replaced := replacements[key]; !replaced {
			env = append(env, entry)
		}
	}
	for i := 0; i < len(pairs); i += 2 {
		env = append(env, pairs[i]+"="+pairs[i+1])
	}
	return env
}

func presentBGPColumnOrderKeys(order, actual []string) []string {
	present := make(map[string]struct{}, len(actual))
	for _, key := range actual {
		present[key] = struct{}{}
	}
	result := make([]string, 0, len(order))
	for _, key := range order {
		if _, ok := present[key]; ok {
			result = append(result, key)
		}
	}
	return result
}

func equalBGPColumnOrderStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func sortedBGPColumnOrderStrings(values []string) bool {
	ordered := append([]string(nil), values...)
	sort.Strings(ordered)
	return equalBGPColumnOrderStrings(values, ordered)
}
