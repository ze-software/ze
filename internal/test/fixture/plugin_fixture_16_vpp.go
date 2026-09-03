package fixture

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type plugin16VPPEntry struct {
	Message string         `json:"msg"`
	Fields  map[string]any `json:"fields"`
}

const plugin16VPPBaseConfig = `environment {
}

bgp {
	router-id 10.0.0.1;
	session { asn { local 65533; } }
}

vpp {
	enabled true;
	external true;
	api-socket %s;
}

interface {
	backend vpp;
	dummy lo0 {
%s	}
}
`

const plugin16VPPAddressUnit = "\t\tunit 0 {\n\t\t\tipv4 {\n\t\t\t\taddress [ 10.42.0.1/32 ];\n\t\t\t}\n\t\t}\n"

func init() {
	Register("plugin/vpp-loopback-reapply", plugin16VPPReapply)
}

// plugin16ReadVPPLog reads the entries the stub has written so far. A log the
// stub has not created yet is empty and not an error, because the caller polls
// for it. Every other failure is an error, so that an unreadable log is never
// read as a message the plugin failed to send.
func plugin16ReadVPPLog(path string) ([]plugin16VPPEntry, error) {
	file, err := os.Open(path) //nolint:gosec // the path is the fixture's own scratch file
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open stub log %s: %w", path, err)
	}
	defer file.Close() //nolint:errcheck // fixture teardown
	entries := make([]plugin16VPPEntry, 0, 16)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry plugin16VPPEntry
		if json.Unmarshal(scanner.Bytes(), &entry) != nil {
			// The stub appends while this runs, so the last line can be half
			// written. Stop here and let the next poll read it whole.
			break
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read stub log %s: %w", path, err)
	}
	return entries, nil
}

func plugin16WaitVPPMessage(ctx context.Context, path, name string, attempts int) ([]plugin16VPPEntry, error) {
	var entries []plugin16VPPEntry
	var readErr error
	Poll(ctx, attempts, 50*time.Millisecond, func() bool {
		entries, readErr = plugin16ReadVPPLog(path)
		if readErr != nil {
			return true
		}
		for _, entry := range entries {
			if entry.Message == name {
				return true
			}
		}
		return false
	})
	return entries, readErr
}

func plugin16StartProcess(cmd *exec.Cmd) (<-chan error, error) {
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	return done, nil
}

func plugin16StopProcess(cmd *exec.Cmd, done <-chan error) {
	if cmd == nil || cmd.Process == nil || done == nil {
		return
	}
	select {
	case <-done:
		return
	default:
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	select {
	case <-done:
		return
	case <-time.After(15 * time.Second):
	}
	_ = cmd.Process.Kill()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		fmt.Fprintf(os.Stderr, "driver: pid %d outlived SIGKILL\n", cmd.Process.Pid)
	}
}

func plugin16VPPFailure(reason, zeLog string, entries []plugin16VPPEntry) error {
	fmt.Fprintf(os.Stderr, "FAIL: %s\n", reason)
	for _, entry := range entries {
		fmt.Fprintf(os.Stderr, "stub: %s %v\n", entry.Message, entry.Fields)
	}
	if content, err := os.ReadFile(zeLog); err == nil { //nolint:gosec // the path is the fixture's own scratch file
		_, _ = os.Stderr.Write(content)
	}
	return fmt.Errorf("%s", reason)
}

func plugin16VPPReapply(ctx context.Context, _ []string) error {
	tmp, err := os.MkdirTemp("", "ze-vpp-reapply-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp) //nolint:errcheck // fixture cleanup
	socketPath := filepath.Join(tmp, "api.sock")
	if len(socketPath) >= 108 {
		return fmt.Errorf("driver: socket path too long: %s", socketPath)
	}
	requestLog := filepath.Join(tmp, "vpp-requests.jsonl")
	configPath := filepath.Join(tmp, "ze.conf")
	zeLog := filepath.Join(tmp, "ze.log")
	if err := os.WriteFile(configPath, []byte(fmt.Sprintf(plugin16VPPBaseConfig, socketPath, "")), 0o600); err != nil {
		return err
	}

	executable, err := os.Executable()
	if err != nil {
		return err
	}
	stub := exec.CommandContext(ctx, executable, "vpp-stub", "--socket", socketPath, "--log", requestLog, "--deadline", "120") //nolint:gosec // the fixture chooses the program and its arguments
	stub.Stdout = io.Discard
	stub.Stderr = io.Discard
	stubDone, err := plugin16StartProcess(stub)
	if err != nil {
		return fmt.Errorf("start vpp stub: %w", err)
	}
	defer plugin16StopProcess(stub, stubDone)
	if !Poll(ctx, 200, 50*time.Millisecond, func() bool {
		_, err := os.Stat(socketPath)
		return err == nil
	}) {
		return plugin16VPPFailure("the stub socket never appeared", zeLog, nil)
	}

	configDir := filepath.Join(tmp, "etc")
	if err := os.Mkdir(configDir, 0o700); err != nil {
		return err
	}
	logFile, err := os.OpenFile(zeLog, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return err
	}
	ze := exec.CommandContext(ctx, "ze", "start", configPath) //nolint:gosec // the fixture chooses the program and its arguments
	ze.Stdout = logFile
	ze.Stderr = logFile
	ze.Env = plugin15Environment(map[string]string{
		"ze.config.dir":    configDir,
		envLogVPP:          logLevelInfo,
		"ze.log.interface": logLevelInfo,
		envLogBGP:          logLevelWarn,
	})
	zeDone, err := plugin16StartProcess(ze)
	if err != nil {
		logFile.Close() //nolint:errcheck // fixture teardown
		return fmt.Errorf("start ze: %w", err)
	}
	defer func() {
		plugin16StopProcess(ze, zeDone)
		logFile.Close() //nolint:errcheck // fixture teardown
	}()

	entries, err := plugin16WaitVPPMessage(ctx, requestLog, "create_loopback", 800)
	if err != nil {
		return err
	}
	foundCreate := false
	for _, entry := range entries {
		if entry.Message == "create_loopback" {
			foundCreate = true
			break
		}
	}
	if !foundCreate {
		return plugin16VPPFailure("the first apply never created the loopback", zeLog, entries)
	}
	if err := os.WriteFile(configPath, []byte(fmt.Sprintf(plugin16VPPBaseConfig, socketPath, plugin16VPPAddressUnit)), 0o600); err != nil {
		return err
	}
	if err := ze.Process.Signal(syscall.SIGHUP); err != nil {
		return fmt.Errorf("reload ze: %w", err)
	}
	entries, err = plugin16WaitVPPMessage(ctx, requestLog, "sw_interface_add_del_address", 800)
	if err != nil {
		return err
	}

	creates := make([]plugin16VPPEntry, 0, 2)
	adds := make([]plugin16VPPEntry, 0, 2)
	for _, entry := range entries {
		switch entry.Message {
		case "create_loopback":
			creates = append(creates, entry)
		case "sw_interface_add_del_address":
			if isAdd, _ := entry.Fields["is_add"].(bool); isAdd {
				adds = append(adds, entry)
			}
		}
	}
	if len(adds) == 0 {
		return plugin16VPPFailure("the second apply never programmed the address", zeLog, entries)
	}
	problems := make([]string, 0, 2)
	indices := make([]any, 0, len(creates))
	for _, entry := range creates {
		indices = append(indices, entry.Fields["sw_if_index"])
	}
	if len(creates) != 1 {
		problems = append(problems, fmt.Sprintf("AC-1: create_loopback count is %d (sw_if_index %v), want 1: the reload leaked a loopback", len(creates), indices))
	}
	if len(creates) != 0 {
		live := creates[0].Fields["sw_if_index"]
		strays := make([]any, 0, len(adds))
		for _, entry := range adds {
			if entry.Fields["sw_if_index"] != live {
				strays = append(strays, entry.Fields["sw_if_index"])
			}
		}
		if len(strays) != 0 {
			problems = append(problems, fmt.Sprintf("AC-2: address programmed on sw_if_index %v, want %v: the name resolves to an interface the first apply did not make", strays, live))
		}
	}
	if len(problems) != 0 {
		return plugin16VPPFailure(strings.Join(problems, "; "), zeLog, entries)
	}
	for _, entry := range entries {
		fmt.Fprintf(os.Stderr, "stub: %s %v\n", entry.Message, entry.Fields)
	}
	fmt.Fprintln(os.Stderr, "OK: one create_loopback across two applies")
	return nil
}
