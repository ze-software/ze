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
	"slices"
	"strings"
	"syscall"
	"time"
)

// vppMsgCreateLoopback is the VPP binary API message the stub logs for a loopback create.
const vppMsgCreateLoopback = "create_loopback"

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

// plugin16VPPSecondLoopback closes lo0 and opens lo1, so a reload adds a
// second loopback beside the first. The base config closes lo1.
const plugin16VPPSecondLoopback = "\t}\n\tdummy lo1 {\n"

const plugin16VPPAddressUnit = "\t\tunit 0 {\n\t\t\tipv4 {\n\t\t\t\taddress [ 10.42.0.1/32 ];\n\t\t\t}\n\t\t}\n"

func init() {
	Register("plugin/vpp-loopback-reapply", plugin16VPPReapply)
	Register("plugin/vpp-loopback-reload-create", plugin16VPPReloadCreate)
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

// plugin16ReloadOutcome waits until ze's stderr records how the SIGHUP reload
// ended, and returns the line that says so, or "" when the wait ran out. The
// reload is a transaction: stopping ze while it runs cancels it and rolls it
// back, so the fixture must not stop ze before one of these lines appears.
// sighupReload (cmd/ze/hub/main_reload.go) prints the first once the whole
// reload has returned, and the second for a reload that failed.
func plugin16ReloadOutcome(ctx context.Context, zeLog string, attempts int) string {
	outcome := ""
	Poll(ctx, attempts, 50*time.Millisecond, func() bool {
		content, err := os.ReadFile(zeLog) //nolint:gosec // the path is the fixture's own scratch file
		if err != nil {
			return false
		}
		for _, line := range []string{plugin16ReloadCompleted, plugin16ReloadFailed} {
			if strings.Contains(string(content), line) {
				outcome = line
				return true
			}
		}
		return false
	})
	return outcome
}

const (
	plugin16ReloadCompleted = "sighup reload complete"
	plugin16ReloadFailed    = "reload error: "
)

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

// plugin16VPPRun is one ze daemon driving the VPP stub, started with the base
// config and past its first apply.
type plugin16VPPRun struct {
	ze         *exec.Cmd
	socketPath string
	requestLog string
	configPath string
	zeLog      string
}

// plugin16StartVPP starts the VPP stub and a ze daemon on the base config, and
// returns once the first apply has created lo0. The returned stop function
// stops ze, then the stub, then removes the scratch directory; it is non-nil
// whenever err is nil.
func plugin16StartVPP(ctx context.Context) (run *plugin16VPPRun, stop func(), err error) {
	var cleanups []func()
	stopAll := func() {
		for _, cleanup := range slices.Backward(cleanups) {
			cleanup()
		}
	}
	defer func() {
		if err != nil {
			stopAll()
		}
	}()
	tmp, err := os.MkdirTemp("", "ze-vpp-reapply-")
	if err != nil {
		return nil, nil, err
	}
	cleanups = append(cleanups, func() { os.RemoveAll(tmp) }) //nolint:errcheck // fixture cleanup
	socketPath := filepath.Join(tmp, "api.sock")
	if len(socketPath) >= 108 {
		return nil, nil, fmt.Errorf("driver: socket path too long: %s", socketPath)
	}
	requestLog := filepath.Join(tmp, "vpp-requests.jsonl")
	configPath := filepath.Join(tmp, "ze.conf")
	zeLog := filepath.Join(tmp, "ze.log")
	if err := os.WriteFile(configPath, []byte(fmt.Sprintf(plugin16VPPBaseConfig, socketPath, "")), 0o600); err != nil {
		return nil, nil, err
	}

	executable, err := os.Executable()
	if err != nil {
		return nil, nil, err
	}
	stub := exec.CommandContext(ctx, executable, "test", "vpp", "stub", "--socket", socketPath, "--log", requestLog, "--deadline", "120") //nolint:gosec // the fixture chooses the program and its arguments
	stub.Stdout = io.Discard
	stub.Stderr = io.Discard
	stubDone, err := plugin16StartProcess(stub)
	if err != nil {
		return nil, nil, fmt.Errorf("start vpp stub: %w", err)
	}
	cleanups = append(cleanups, func() { plugin16StopProcess(stub, stubDone) })
	if !Poll(ctx, WaitAttempts(10, 50*time.Millisecond, 200), 50*time.Millisecond, func() bool {
		_, err := os.Stat(socketPath)
		return err == nil
	}) {
		return nil, nil, plugin16VPPFailure("the stub socket never appeared", zeLog, nil)
	}

	configDir := filepath.Join(tmp, "etc")
	if err := os.Mkdir(configDir, 0o700); err != nil {
		return nil, nil, err
	}
	logFile, err := os.OpenFile(zeLog, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return nil, nil, err
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
		return nil, nil, fmt.Errorf("start ze: %w", err)
	}
	cleanups = append(cleanups, func() {
		plugin16StopProcess(ze, zeDone)
		logFile.Close() //nolint:errcheck // fixture teardown
	})

	// The first apply waits for the whole daemon start, which creates and
	// fsyncs the store before any plugin runs. On a saturated disk that was
	// measured past 40s, so both waits derive from the test budget: 45% for the
	// start, 30% for the reload's address and 15% for the reload's outcome
	// leave room for the stop below.
	entries, err := plugin16WaitVPPMessage(ctx, requestLog, vppMsgCreateLoopback, WaitAttempts(45, 50*time.Millisecond, 800))
	if err != nil {
		return nil, nil, err
	}
	foundCreate := false
	for _, entry := range entries {
		if entry.Message == vppMsgCreateLoopback {
			foundCreate = true
			break
		}
	}
	if !foundCreate {
		return nil, nil, plugin16VPPFailure("the first apply never created the loopback", zeLog, entries)
	}
	return &plugin16VPPRun{ze: ze, socketPath: socketPath, requestLog: requestLog, configPath: configPath, zeLog: zeLog}, stopAll, nil
}

func plugin16VPPReapply(ctx context.Context, _ []string) error {
	run, stop, err := plugin16StartVPP(ctx)
	if err != nil {
		return err
	}
	defer stop()
	ze, socketPath, requestLog, configPath, zeLog := run.ze, run.socketPath, run.requestLog, run.configPath, run.zeLog
	if err := os.WriteFile(configPath, []byte(fmt.Sprintf(plugin16VPPBaseConfig, socketPath, plugin16VPPAddressUnit)), 0o600); err != nil {
		return err
	}
	if err := ze.Process.Signal(syscall.SIGHUP); err != nil {
		return fmt.Errorf("reload ze: %w", err)
	}
	entries, err := plugin16WaitVPPMessage(ctx, requestLog, "sw_interface_add_del_address", WaitAttempts(30, 50*time.Millisecond, 800))
	if err != nil {
		return err
	}

	creates := make([]plugin16VPPEntry, 0, 2)
	adds := make([]plugin16VPPEntry, 0, 2)
	for _, entry := range entries {
		switch entry.Message {
		case vppMsgCreateLoopback:
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
	// The address is programmed during the reload's apply phase, before the
	// transaction commits. Stopping ze now would cancel the reload and roll it
	// back, so wait for the reload to report its outcome first.
	switch plugin16ReloadOutcome(ctx, zeLog, WaitAttempts(15, 50*time.Millisecond, 400)) {
	case plugin16ReloadCompleted:
		// Committed: ze can stop, and the requests below are the whole reload.
	case plugin16ReloadFailed:
		return plugin16VPPFailure("the reload that programmed the address failed", zeLog, entries)
	default:
		return plugin16VPPFailure("the reload never reported its outcome", zeLog, entries)
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

// plugin16VPPReloadCreate proves a reload that creates a VPP interface commits.
//
// The reload adds `dummy lo1`, which the iface decomposer plans as an
// add-interface operation, and the transaction waits for (interface, created)
// on lo1 before it commits (settlement rule iface-add-interface-settles-created).
// VPP sends no event for a create, so the backend announces it itself
// (emitCreated, internal/plugins/iface/vpp/monitor.go). Without that the
// reload times out after 5s and rolls back.
func plugin16VPPReloadCreate(ctx context.Context, _ []string) error {
	run, stop, err := plugin16StartVPP(ctx)
	if err != nil {
		return err
	}
	defer stop()
	if err := os.WriteFile(run.configPath, []byte(fmt.Sprintf(plugin16VPPBaseConfig, run.socketPath, plugin16VPPSecondLoopback)), 0o600); err != nil {
		return err
	}
	if err := run.ze.Process.Signal(syscall.SIGHUP); err != nil {
		return fmt.Errorf("reload ze: %w", err)
	}
	outcome := plugin16ReloadOutcome(ctx, run.zeLog, WaitAttempts(45, 50*time.Millisecond, 800))
	entries, err := plugin16ReadVPPLog(run.requestLog)
	if err != nil {
		return err
	}
	switch outcome {
	case plugin16ReloadCompleted:
	case plugin16ReloadFailed:
		return plugin16VPPFailure("the reload that created lo1 failed", run.zeLog, entries)
	default:
		return plugin16VPPFailure("the reload never reported its outcome", run.zeLog, entries)
	}
	creates := 0
	for _, entry := range entries {
		if entry.Message == vppMsgCreateLoopback {
			creates++
		}
	}
	if creates != 2 {
		return plugin16VPPFailure(fmt.Sprintf("create_loopback count is %d, want 2 (lo0 at start, lo1 on reload)", creates), run.zeLog, entries)
	}
	fmt.Fprintln(os.Stderr, "OK: the reload created lo1 and committed")
	return nil
}
