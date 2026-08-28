package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func plugin01AnswerManyRecords(ctx context.Context, plugin *sdk.Plugin) error {
	dispatchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	answer, err := plugin.DispatchCommandAnswer(dispatchCtx, "system command list")
	if err != nil {
		return err
	}
	if answer.Type != rpc.AnswerTypeMap || answer.Key != "commands" || len(answer.Fields) != 0 {
		return fmt.Errorf("unexpected answer head: type=%q key=%q fields=%v", answer.Type, answer.Key, answer.Fields)
	}
	count := 0
	for record := range answer.Records {
		if len(record.Fault) != 0 {
			return fmt.Errorf("record %d was rejected: %s", count, record.Fault)
		}
		var row map[string]any
		if err := json.Unmarshal(record.Item, &row); err != nil {
			return fmt.Errorf("record %d: %w", count, err)
		}
		if _, ok := row["value"]; !ok {
			return fmt.Errorf("record %d carries no command name: %.120s", count, record.Item)
		}
		if _, ok := row["commands"]; ok {
			return fmt.Errorf("record %d carries the envelope", count)
		}
		count++
	}
	if err := answer.Err(); err != nil {
		return err
	}
	if answer.Verdict() != rpc.VerdictDone {
		return fmt.Errorf("answer verdict %s: %s", answer.Verdict(), answer.Message())
	}
	if count <= rpc.AnswerBufferThreshold {
		return fmt.Errorf("fixture problem: %d commands is inside the %d-record threshold", count, rpc.AnswerBufferThreshold)
	}
	fmt.Fprintf(os.Stderr, "OK: %d records streamed, one line each\n", count)
	return nil
}

func plugin01ReadOneDocument(ctx context.Context, plugin *sdk.Plugin, command string) ([]byte, error) {
	answer, err := plugin.DispatchCommandAnswer(ctx, command)
	if err != nil {
		return nil, err
	}
	if answer.Type != rpc.AnswerTypeDocument || answer.Key != "" || len(answer.Fields) != 0 {
		return nil, fmt.Errorf("%s: unexpected answer head: type=%q key=%q fields=%v", command, answer.Type, answer.Key, answer.Fields)
	}
	var item []byte
	count := 0
	for record := range answer.Records {
		if len(record.Fault) != 0 {
			return nil, fmt.Errorf("%s rejected a record: %s", command, record.Fault)
		}
		item = append(item[:0], record.Item...)
		count++
	}
	if err := answer.Err(); err != nil {
		return nil, err
	}
	if count != 1 || answer.Verdict() != rpc.VerdictDone || answer.Message() != "" {
		return nil, fmt.Errorf("%s: count=%d verdict=%s message=%q", command, count, answer.Verdict(), answer.Message())
	}
	return item, nil
}

func plugin01AnswerUnconditionalFirst(ctx context.Context, _ []string) error {
	plugin, err := newObserver("answer-silent")
	if err != nil {
		return err
	}
	defer plugin.Close() //nolint:errcheck
	result := make(chan error, 1)
	plugin.OnAllPluginsReady(func() error {
		version, scenarioErr := plugin01ReadOneDocument(ctx, plugin, "system version api")
		if scenarioErr == nil {
			var peers []byte
			peers, scenarioErr = plugin01ReadOneDocument(ctx, plugin, "show bgp peer list")
			if scenarioErr == nil && !bytes.Contains(peers, []byte(`"peers"`)) {
				scenarioErr = fmt.Errorf("show bgp peer list lost its envelope: %.120s", peers)
			}
		}
		if scenarioErr == nil {
			fmt.Fprintln(os.Stderr, "OK: the first peer received the record shape")
			scenarioErr = plugin01AtomicWrite("silent-answer.txt", version)
		}
		result <- scenarioErr
		return scenarioErr
	})
	runErr := plugin.Run(ctx, sdk.Registration{})
	select {
	case scenarioErr := <-result:
		return errors.Join(scenarioErr, runErr)
	default:
		return runErr
	}
}

func plugin01AnswerUnconditionalSecond(ctx context.Context, plugin *sdk.Plugin) error {
	var first []byte
	if !Poll(ctx, 40, plugin01PollDelay, func() bool {
		var err error
		first, err = os.ReadFile("silent-answer.txt")
		return err == nil
	}) {
		return errors.New("the first peer wrote no answer within 10s")
	}
	second, err := plugin01ReadOneDocument(ctx, plugin, "system version api")
	if err != nil {
		return err
	}
	if !bytes.Equal(first, second) {
		return fmt.Errorf("second peer received %.120q, first peer got %.120q", second, first)
	}
	fmt.Fprintln(os.Stderr, "OK: the second peer received the same bytes")
	return nil
}

const plugin01SSHBaseConfig = `bgp {
	peer peer1 {
		connection {
			remote {
				ip 127.0.0.2
			}
			local {
				ip 127.0.0.1
				accept false
			}
		}
		session {
			asn {
				local 65533
				remote 65533
			}
		}
	}
}

system {
	authentication {
		user admin {
			password "$2a$04$UlwuiuH82Unfsq.XEMPGJeDkXwbm3KW.nvVaVXOd/JeFK8VjMjrQO"
			profile [ admin ]
		}
	}
	authorization {
		profile admin {
			run {
				default-action allow
			}
			edit {
				default-action allow
			}
		}
	}
}

environment {
	ssh {
		enabled true
		server main {
			ip 127.0.0.1;
			port 0;
		}
	}
}
`

const plugin01RecordPluginConfig = `plugin {
	external record-plugin {
		run "ze-test record-plugin"
		encoder json
	}
}

`

type plugin01SSHRuntime struct {
	adminDir string
	workDir  string
	port     string
	cancel   context.CancelFunc
	command  *exec.Cmd
	log      *os.File
}

func plugin01Environment(values map[string]string) []string {
	environment := make(map[string]string, len(os.Environ())+len(values))
	for _, entry := range os.Environ() {
		key, value, found := strings.Cut(entry, "=")
		if found {
			environment[key] = value
		}
	}
	for key, value := range values {
		environment[key] = value
	}
	result := make([]string, 0, len(environment))
	for key, value := range environment {
		result = append(result, key+"="+value)
	}
	return result
}

func plugin01Wait(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func plugin01SSHPort(log []byte) string {
	const host = "127.0.0.1:"
	start := bytes.Index(log, []byte(host))
	if start < 0 {
		return ""
	}
	start += len(host)
	end := start
	for end < len(log) && log[end] >= '0' && log[end] <= '9' {
		end++
	}
	if end == start {
		return ""
	}
	return string(log[start:end])
}

func plugin01StartSSHRuntime(ctx context.Context, configPath, config string) (*plugin01SSHRuntime, error) {
	if err := os.WriteFile("daemon.ready", nil, 0o600); err != nil {
		return nil, err
	}
	workDir, err := os.MkdirTemp("", "ze-fixture-ssh-")
	if err != nil {
		return nil, err
	}
	configPath = filepath.Join(workDir, configPath)
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		_ = os.RemoveAll(workDir)
		return nil, err
	}
	logPath := filepath.Join(workDir, "daemon.log")
	log, err := os.Create(logPath)
	if err != nil {
		_ = os.RemoveAll(workDir)
		return nil, err
	}
	daemonCtx, cancel := context.WithCancel(ctx)
	command := exec.CommandContext(daemonCtx, "ze", "start", configPath)
	command.Dir = workDir
	command.Cancel = func() error {
		if command.Process == nil {
			return nil
		}
		return command.Process.Signal(syscall.SIGTERM)
	}
	command.WaitDelay = 2 * time.Second
	command.Env = plugin01Environment(map[string]string{
		"ze.config.dir":    workDir,
		"ze_test_bgp_port": strconv.Itoa(10000 + os.Getpid()%50000),
	})
	command.Stderr = log
	if err := command.Start(); err != nil {
		cancel()
		_ = log.Close()
		_ = os.RemoveAll(workDir)
		return nil, err
	}
	runtime := &plugin01SSHRuntime{workDir: workDir, cancel: cancel, command: command, log: log}
	var logBody []byte
	if !Poll(ctx, 50, 200*time.Millisecond, func() bool {
		_ = log.Sync()
		logBody, _ = os.ReadFile(logPath)
		runtime.port = plugin01SSHPort(logBody)
		return runtime.port != ""
	}) {
		runtime.Close()
		return nil, fmt.Errorf("SSH server did not start (no address in daemon.log): %s", logBody)
	}
	fmt.Fprintf(os.Stderr, "SSH port: %s\n", runtime.port)
	if err := plugin01Wait(ctx, 500*time.Millisecond); err != nil {
		runtime.Close()
		return nil, err
	}
	runtime.adminDir, err = os.MkdirTemp("", "ze-fixture-admin-")
	if err != nil {
		runtime.Close()
		return nil, err
	}
	initCommand := exec.CommandContext(ctx, "ze", "init")
	initCommand.Env = plugin01Environment(map[string]string{"ZE_CONFIG_DIR": runtime.adminDir})
	initCommand.Stdin = strings.NewReader("admin\ntestpass\n127.0.0.1\n" + runtime.port + "\n")
	initCommand.Stdout = io.Discard
	initCommand.Stderr = os.Stderr
	if err := initCommand.Run(); err != nil {
		runtime.Close()
		return nil, fmt.Errorf("ze init: %w", err)
	}
	return runtime, nil
}

func (runtime *plugin01SSHRuntime) Close() {
	if runtime == nil {
		return
	}
	runtime.cancel()
	if runtime.command != nil {
		_ = runtime.command.Wait()
	}
	if runtime.log != nil {
		_ = runtime.log.Close()
	}
	if runtime.adminDir != "" {
		_ = os.RemoveAll(runtime.adminDir)
	}
	if runtime.workDir != "" {
		_ = os.RemoveAll(runtime.workDir)
	}
}
func (runtime *plugin01SSHRuntime) path(name string) string {
	return filepath.Join(runtime.workDir, name)
}

func (runtime *plugin01SSHRuntime) CLI(ctx context.Context, command string, extraEnv map[string]string) ([]byte, []byte, error) {
	values := map[string]string{
		"ZE_CONFIG_DIR":   runtime.adminDir,
		"ZE_SSH_PASSWORD": "testpass",
	}
	for key, value := range extraEnv {
		values[key] = value
	}
	executable := exec.CommandContext(ctx, "ze", "cli", "-c", command)
	executable.Env = plugin01Environment(values)
	var stdout, stderr bytes.Buffer
	executable.Stdout = &stdout
	executable.Stderr = &stderr
	err := executable.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func plugin01CommandRows(raw []byte) ([]json.RawMessage, error) {
	var document struct {
		Commands []json.RawMessage `json:"commands"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, err
	}
	return document.Commands, nil
}

func plugin01LinesStartingObject(raw []byte) int {
	count := 0
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		if len(line) > 0 && line[0] == '{' {
			count++
		}
	}
	return count
}

func plugin01OutputLines(raw []byte) int {
	trimmed := bytes.TrimRight(raw, "\r\n")
	if len(trimmed) == 0 {
		return 0
	}
	return len(bytes.Split(trimmed, []byte{'\n'}))
}

func plugin01AnswerFirstBoundsLongWalk(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("answer-first-bounds-long-walk takes no arguments")
	}
	runtime, err := plugin01StartSSHRuntime(ctx, "answer-first-bounds.conf", plugin01SSHBaseConfig)
	if err != nil {
		return err
	}
	defer runtime.Close()

	full, stderr, err := runtime.CLI(ctx, "system command list | raw", nil)
	if err != nil {
		return fmt.Errorf("system command list did not answer: %w: %s", err, stderr)
	}
	fullRows, err := plugin01CommandRows(full)
	if err != nil {
		return err
	}
	if len(fullRows) <= rpc.AnswerBufferThreshold {
		return fmt.Errorf("fixture problem -- the daemon registers %d commands, inside the %d-record threshold", len(fullRows), rpc.AnswerBufferThreshold)
	}
	fmt.Fprintf(os.Stderr, "OK: the unbounded walk answers %d records\n", len(fullRows))

	const want = 10
	bounded, stderr, err := runtime.CLI(ctx, "system command list | first 10 | raw", nil)
	if err != nil {
		return fmt.Errorf("the bounded chain did not answer: %w: %s", err, stderr)
	}
	boundedRows, err := plugin01CommandRows(bounded)
	if err != nil {
		return err
	}
	if len(boundedRows) != want {
		return fmt.Errorf("| first %d carries %d records, want %d", want, len(boundedRows), want)
	}
	streamed, stderr, err := runtime.CLI(ctx, "system command list | ndjson", nil)
	if err != nil {
		return fmt.Errorf("the unbounded ndjson walk did not answer: %w: %s", err, stderr)
	}
	if lines := plugin01LinesStartingObject(streamed); lines != len(fullRows) {
		return fmt.Errorf("the unbounded walk rendered %d ndjson lines, want %d", lines, len(fullRows))
	}
	boundedNDJSON, stderr, err := runtime.CLI(ctx, "system command list | first 10 | ndjson", nil)
	if err != nil {
		return fmt.Errorf("the bounded ndjson chain did not answer: %w: %s", err, stderr)
	}
	if lines := plugin01LinesStartingObject(boundedNDJSON); lines != 1 {
		return fmt.Errorf("| first %d | ndjson rendered %d lines, want one document", want, lines)
	}
	counted, stderr, err := runtime.CLI(ctx, "system command list | first 10 | count", nil)
	if err != nil {
		return fmt.Errorf("the counted chain did not answer: %w: %s", err, stderr)
	}
	if !bytes.Contains(counted, []byte(strconv.Itoa(want))) {
		return fmt.Errorf("| first %d | count reported %s, want %d", want, counted, want)
	}
	fmt.Fprintf(os.Stderr, "OK: | first %d bounds a walk of %d records to %d\n", want, len(fullRows), want)
	fullPath := runtime.path("full.json")
	boundedPath := runtime.path("bounded.json")
	if err := os.WriteFile(fullPath, full, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(boundedPath, bounded, 0o600); err != nil {
		return err
	}
	return plugin01CompareBounded(ctx, []string{fullPath, boundedPath, strconv.Itoa(want)})
}

func plugin01AnswerSingleRecord(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("answer-single-record takes no arguments")
	}
	runtime, err := plugin01StartSSHRuntime(ctx, "answer-single-record.conf", plugin01SSHBaseConfig)
	if err != nil {
		return err
	}
	defer runtime.Close()

	full, stderr, err := runtime.CLI(ctx, "system command list | raw", nil)
	if err != nil {
		return fmt.Errorf("system command list did not answer: %w: %s", err, stderr)
	}
	fullRows, err := plugin01CommandRows(full)
	if err != nil {
		return err
	}
	if len(fullRows) <= rpc.AnswerBufferThreshold {
		return fmt.Errorf("fixture problem -- the daemon registers %d commands, inside the %d-record threshold", len(fullRows), rpc.AnswerBufferThreshold)
	}
	fmt.Fprintf(os.Stderr, "OK: the command list answers %d records\n", len(fullRows))
	for _, want := range []int{1, 2, len(fullRows)} {
		command := "system command list | raw"
		if want != len(fullRows) {
			command = fmt.Sprintf("system command list | first %d | raw", want)
		}
		output, commandStderr, commandErr := runtime.CLI(ctx, command, nil)
		if commandErr != nil {
			return fmt.Errorf("%s did not answer: %w: %s", command, commandErr, commandStderr)
		}
		if lines := plugin01OutputLines(output); lines != 1 {
			return fmt.Errorf("%s rendered %d lines, want one document", command, lines)
		}
		rows, err := plugin01CommandRows(output)
		if err != nil {
			return err
		}
		if len(rows) != want {
			return fmt.Errorf("%s carries %d records, want %d", command, len(rows), want)
		}
		if !bytes.HasPrefix(output, []byte(`{"commands":[`)) {
			return fmt.Errorf("%s lost its envelope: %.200s", command, output)
		}
	}
	fmt.Fprintln(os.Stderr, "OK: one record and many records read through one path")
	ndjson, stderr, err := runtime.CLI(ctx, "system command list | ndjson", nil)
	if err != nil {
		return fmt.Errorf("system command list | ndjson did not answer: %w: %s", err, stderr)
	}
	if records := plugin01LinesStartingObject(ndjson); records != len(fullRows) {
		return fmt.Errorf("ndjson rendered %d record lines, want %d", records, len(fullRows))
	}
	for _, line := range bytes.Split(ndjson, []byte{'\n'}) {
		if bytes.HasPrefix(line, []byte(`{"commands"`)) {
			return errors.New("a record line carries the commands envelope")
		}
	}
	fmt.Fprintln(os.Stderr, "OK: the streamed answer reached the operator one record per line")
	return nil
}

func plugin01AnswerPayloadUnchanged(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("answer-payload-unchanged takes no arguments")
	}
	runtime, err := plugin01StartSSHRuntime(ctx, "answer-payload-unchanged.conf", plugin01RecordPluginConfig+plugin01SSHBaseConfig)
	if err != nil {
		return err
	}
	defer runtime.Close()

	if !Poll(ctx, 50, 200*time.Millisecond, func() bool {
		output, _, commandErr := runtime.CLI(ctx, "system command list | raw", nil)
		return commandErr == nil && bytes.Contains(output, []byte("show test records walk"))
	}) {
		return errors.New("record plugin command did not enter the registry")
	}
	collapsed, stderr, err := runtime.CLI(ctx, "show test records fault | raw", nil)
	if err != nil {
		return fmt.Errorf("the collapsed answer did not reach the operator: %w: %s", err, stderr)
	}
	streamed, stderr, err := runtime.CLI(ctx, "show test records walk | raw", nil)
	if err != nil {
		return fmt.Errorf("the streamed answer did not reach the operator: %w: %s", err, stderr)
	}
	collapsedPath := runtime.path("collapsed.json")
	streamedPath := runtime.path("streamed.json")
	if err := os.WriteFile(collapsedPath, collapsed, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(streamedPath, streamed, 0o600); err != nil {
		return err
	}
	return plugin01ComparePayloads(ctx, []string{collapsedPath, streamedPath})
}

func plugin01ReadRelayCount(path string) (connections int, forwarded int64, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	fields := strings.Fields(string(raw))
	if len(fields) != 2 {
		return 0, 0, fmt.Errorf("%s carries %q, want connections and bytes", path, raw)
	}
	connections, err = strconv.Atoi(fields[0])
	if err != nil {
		return 0, 0, err
	}
	forwarded, err = strconv.ParseInt(fields[1], 10, 64)
	return connections, forwarded, err
}

func plugin01WaitForFile(ctx context.Context, path string) bool {
	return Poll(ctx, 100, 100*time.Millisecond, func() bool {
		info, err := os.Stat(path)
		return err == nil && info.Size() > 0
	})
}

func plugin01AnswerTruncationDetected(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("answer-truncation-detected takes no arguments")
	}
	runtime, err := plugin01StartSSHRuntime(ctx, "answer-truncation.conf", plugin01SSHBaseConfig)
	if err != nil {
		return err
	}
	defer runtime.Close()

	wholeCtx, cancelWhole := context.WithCancel(ctx)
	defer cancelWhole()
	wholePort := runtime.path("whole.port")
	wholeCount := runtime.path("whole.count")
	wholeDone := make(chan error, 1)
	go func() {
		wholeDone <- plugin01CutRelay(wholeCtx, []string{wholePort, runtime.port, "0", wholeCount})
	}()
	if !plugin01WaitForFile(ctx, wholePort) {
		return errors.New("whole.port never appeared")
	}
	wholePortRaw, err := os.ReadFile(wholePort)
	if err != nil {
		return err
	}
	relayEnvironment := map[string]string{
		"ZE_SSH_USERNAME": "admin",
		"ZE_SSH_PASSWORD": "testpass",
		"ZE_SSH_PORT":     strings.TrimSpace(string(wholePortRaw)),
	}
	full, stderr, err := runtime.CLI(ctx, "system command list | ndjson", relayEnvironment)
	if err != nil {
		return fmt.Errorf("the relay could not carry a complete answer: %w: %s", err, stderr)
	}
	fullLines := plugin01LinesStartingObject(full)
	if fullLines < rpc.AnswerBufferThreshold+1 {
		return fmt.Errorf("fixture problem -- the complete answer is only %d records", fullLines)
	}
	var connections int
	var forwarded int64
	if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
		var readErr error
		connections, forwarded, readErr = plugin01ReadRelayCount(wholeCount)
		return readErr == nil && connections >= 2
	}) {
		return errors.New("the relay never reported a complete run")
	}
	cut := forwarded / 2
	fmt.Fprintf(os.Stderr, "OK: a complete answer is %d records and %d bytes on the wire\n", fullLines, forwarded)

	cutCtx, cancelCut := context.WithCancel(ctx)
	defer cancelCut()
	cutPort := runtime.path("cut.port")
	cutCount := runtime.path("cut.count")
	cutDone := make(chan error, 1)
	go func() {
		cutDone <- plugin01CutRelay(cutCtx, []string{cutPort, runtime.port, strconv.FormatInt(cut, 10), cutCount})
	}()
	if !plugin01WaitForFile(ctx, cutPort) {
		return errors.New("cut.port never appeared")
	}
	cutPortRaw, err := os.ReadFile(cutPort)
	if err != nil {
		return err
	}
	relayEnvironment["ZE_SSH_PORT"] = strings.TrimSpace(string(cutPortRaw))
	partial, cutStderr, cutErr := runtime.CLI(ctx, "system command list | ndjson", relayEnvironment)
	if cutErr == nil {
		return fmt.Errorf("a cut answer exited 0, so the client took it for a complete one: %s", cutStderr)
	}
	if !bytes.Contains(cutStderr, []byte("answer ended before its terminator")) {
		return fmt.Errorf("the client did not report truncation: %s", cutStderr)
	}
	partialLines := plugin01LinesStartingObject(partial)
	if partialLines >= fullLines {
		return fmt.Errorf("the cut answer is %d records of %d, so the relay cut nothing", partialLines, fullLines)
	}
	fmt.Fprintf(os.Stderr, "OK: a cut answer of %d records was reported as truncated\n", partialLines)

	cancelCut()
	cancelWhole()
	select {
	case relayErr := <-cutDone:
		if relayErr != nil {
			return relayErr
		}
	case <-time.After(time.Second):
	}
	select {
	case relayErr := <-wholeDone:
		if relayErr != nil {
			return relayErr
		}
	case <-time.After(time.Second):
	}
	return nil
}

func plugin01CanonicalJSON(raw json.RawMessage) (string, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", err
	}
	canonical, err := json.Marshal(value)
	return string(canonical), err
}

func plugin01CompareBounded(_ context.Context, args []string) error {
	if len(args) != 3 {
		return errors.New("compare requires FULL BOUNDED WANT")
	}
	want, err := strconv.Atoi(args[2])
	if err != nil {
		return err
	}
	read := func(path string) ([]json.RawMessage, error) {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var document struct {
			Commands []json.RawMessage `json:"commands"`
		}
		err = json.Unmarshal(raw, &document)
		return document.Commands, err
	}
	full, err := read(args[0])
	if err != nil {
		return err
	}
	bounded, err := read(args[1])
	if err != nil {
		return err
	}
	if len(bounded) != want {
		return fmt.Errorf("the bounded answer carries %d records, want %d", len(bounded), want)
	}
	known := make(map[string]struct{}, len(full))
	for _, row := range full {
		canonical, err := plugin01CanonicalJSON(row)
		if err != nil {
			return err
		}
		known[canonical] = struct{}{}
	}
	for _, row := range bounded {
		canonical, err := plugin01CanonicalJSON(row)
		if err != nil {
			return err
		}
		if _, ok := known[canonical]; !ok {
			return fmt.Errorf("a bounded record is not a record of the walk: %s", row)
		}
	}
	fmt.Fprintf(os.Stderr, "OK: the %d bounded records are records of the walk, byte for byte\n", want)
	return nil
}

func plugin01ProducedRow(index, fill int) []byte {
	return []byte(fmt.Sprintf(`{"index":%d,"fill":"%s"}`, index, strings.Repeat("x", fill)))
}

func plugin01CompactJSON(raw []byte) ([]byte, error) {
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return nil, err
	}
	return compact.Bytes(), nil
}

func plugin01ComparePayloads(_ context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("compare requires COLLAPSED STREAMED")
	}
	collapsedRaw, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	var collapsed map[string]json.RawMessage
	if err := json.Unmarshal(collapsedRaw, &collapsed); err != nil {
		return err
	}
	keys := make([]string, 0, len(collapsed))
	for key := range collapsed {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	if !slices.Equal(keys, []string{"errors", "rows"}) {
		return fmt.Errorf("collapsed answer carries %v, want errors and rows", keys)
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(collapsed["rows"], &rows); err != nil {
		return err
	}
	wantIndexes := []int{0, 1, 2, 3, 4, 5, 7, 8, 9, 10, 11}
	if len(rows) != len(wantIndexes) {
		return fmt.Errorf("collapsed answer carries %d rows, want %d", len(rows), len(wantIndexes))
	}
	for index, row := range rows {
		compact, err := plugin01CompactJSON(row)
		if err != nil {
			return err
		}
		if !bytes.Equal(compact, plugin01ProducedRow(wantIndexes[index], 64)) {
			return fmt.Errorf("collapsed row %d changed: %.80s", wantIndexes[index], compact)
		}
	}
	var faults []map[string]any
	if err := json.Unmarshal(collapsed["errors"], &faults); err != nil {
		return err
	}
	if len(faults) != 1 {
		return fmt.Errorf("collapsed answer rejected %v, want one rejection", faults)
	}
	faultKeys := make([]string, 0, len(faults[0]))
	for key := range faults[0] {
		faultKeys = append(faultKeys, key)
	}
	slices.Sort(faultKeys)
	if !slices.Equal(faultKeys, []string{"encoded-bytes", "limit-bytes", "message", "record"}) {
		return fmt.Errorf("rejection fields are %v", faultKeys)
	}
	fmt.Fprintf(os.Stderr, "OK: %d collapsed rows are the bytes the producer wrote\n", len(rows))

	streamedRaw, err := os.ReadFile(args[1])
	if err != nil {
		return err
	}
	var streamedDocument map[string]json.RawMessage
	if err := json.Unmarshal(streamedRaw, &streamedDocument); err != nil {
		return err
	}
	if len(streamedDocument) != 1 || streamedDocument["rows"] == nil {
		return fmt.Errorf("streamed answer carries unexpected keys")
	}
	var streamed []json.RawMessage
	if err := json.Unmarshal(streamedDocument["rows"], &streamed); err != nil {
		return err
	}
	if len(streamed) <= rpc.AnswerBufferThreshold || len(streamed) != 300 {
		return fmt.Errorf("streamed answer carries %d rows, want 300 beyond threshold", len(streamed))
	}
	for index, row := range streamed {
		compact, err := plugin01CompactJSON(row)
		if err != nil {
			return err
		}
		if !bytes.Equal(compact, plugin01ProducedRow(index, 60000)) {
			return fmt.Errorf("streamed row %d changed: %.80s", index, compact)
		}
	}
	fmt.Fprintf(os.Stderr, "OK: %d streamed rows are the bytes the producer wrote\n", len(streamed))
	return nil
}

func plugin01AtomicWrite(path string, data []byte) error {
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func plugin01CutRelay(ctx context.Context, args []string) error {
	if len(args) != 4 {
		return errors.New("relay requires PORT_FILE DAEMON_PORT LIMIT COUNT_FILE")
	}
	daemonPort, err := strconv.Atoi(args[1])
	if err != nil {
		return err
	}
	limit, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer listener.Close() //nolint:errcheck
	go func() {
		<-ctx.Done()
		listener.Close() //nolint:errcheck
	}()
	port := listener.Addr().(*net.TCPAddr).Port
	if err := plugin01AtomicWrite(args[0], []byte(strconv.Itoa(port))); err != nil {
		return err
	}
	var forwarded int64
	connections := 0
	for limit == 0 || forwarded < limit {
		client, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		connections++
		daemon, err := net.DialTimeout("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(daemonPort)), 5*time.Second)
		if err != nil {
			client.Close() //nolint:errcheck
			return err
		}
		clientTCP := client.(*net.TCPConn)
		daemonTCP := daemon.(*net.TCPConn)
		upstreamDone := make(chan struct{})
		go func() {
			_, _ = io.Copy(daemonTCP, clientTCP)
			_ = daemonTCP.CloseWrite()
			close(upstreamDone)
		}()
		buffer := make([]byte, 65536)
		for limit == 0 || forwarded < limit {
			n, readErr := daemonTCP.Read(buffer)
			if n > 0 {
				if _, err := clientTCP.Write(buffer[:n]); err != nil {
					break
				}
				forwarded += int64(n)
			}
			if readErr != nil {
				break
			}
		}
		_ = clientTCP.CloseRead()
		_ = clientTCP.CloseWrite()
		_ = daemonTCP.CloseRead()
		_ = daemonTCP.CloseWrite()
		_ = clientTCP.Close()
		_ = daemonTCP.Close()
		select {
		case <-upstreamDone:
		case <-time.After(time.Second):
		}
		count := []byte(fmt.Sprintf("%d %d", connections, forwarded))
		if err := plugin01AtomicWrite(filepath.Clean(args[3]), count); err != nil {
			return err
		}
	}
	return nil
}
