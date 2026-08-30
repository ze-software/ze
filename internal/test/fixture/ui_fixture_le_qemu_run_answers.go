package fixture

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
)

func init() {
	Register("ui/le-qemu-run-answers", uiDriver(leQEMURunAnswers))
}

type uiLeQemuRunAnswersCommandResult struct {
	stdout []byte
	stderr []byte
	code   int
	err    error
}

type recordedCall struct {
	Program string   `json:"program"`
	Args    []string `json:"args"`
	Dotted  []string `json:"dotted"`
}

func leQEMURunAnswers(ctx context.Context) error {
	root := os.Getenv("ZE_REPO_ROOT")
	if root == "" {
		return fmt.Errorf("FAIL: ZE_REPO_ROOT is not set")
	}
	work, _, err := temporaryLEFixtureWorkspace("le-qemu-run-answers-")
	if err != nil {
		return fmt.Errorf("FAIL: create fixture working directory: %w", err)
	}
	defer os.RemoveAll(work) //nolint:errcheck // fixture cleanup
	binary, err := uiLEBinary(root)
	if err != nil {
		return fmt.Errorf("FAIL: %w", err)
	}

	fixtureRoot := filepath.Join(work, "fixture")
	if err := os.MkdirAll(filepath.Join(fixtureRoot, "tmp"), 0o750); err != nil {
		return fmt.Errorf("FAIL: create fixture root: %w", err)
	}
	if err := os.WriteFile(filepath.Join(fixtureRoot, "go.mod"), []byte("module fixture\n"), 0o600); err != nil {
		return fmt.Errorf("FAIL: write fixture go.mod: %w", err)
	}
	if err := os.WriteFile(filepath.Join(fixtureRoot, "feature-gates.txt"), []byte("ze_core internal/core\n"), 0o600); err != nil {
		return fmt.Errorf("FAIL: write fixture feature gates: %w", err)
	}

	arm := runtime.GOARCH == archARM64
	arch := "x86_64"
	if arm {
		arch = "aarch64"
	}
	qemuName := "qemu-system-" + arch

	cache := filepath.Join(work, "cache")
	isoDir := filepath.Join(cache, "ze", "alpine-iso")
	if err := os.MkdirAll(isoDir, 0o750); err != nil {
		return fmt.Errorf("FAIL: create ISO cache: %w", err)
	}
	isoName := "alpine-virt-3.21.3-" + arch + ".iso"
	iso := []byte("ui fixture iso")
	if err := os.WriteFile(filepath.Join(isoDir, isoName), iso, 0o600); err != nil {
		return fmt.Errorf("FAIL: write fixture ISO: %w", err)
	}
	sum := sha256.Sum256(iso)
	checksum := hex.EncodeToString(sum[:]) + "  " + isoName + "\n"
	if err := os.WriteFile(filepath.Join(isoDir, isoName+".sha256"), []byte(checksum), 0o600); err != nil {
		return fmt.Errorf("FAIL: write fixture ISO checksum: %w", err)
	}

	brew := filepath.Join(work, "brew")
	if arm {
		firmware := filepath.Join(brew, "share", "qemu", "edk2-aarch64-code.fd")
		if err := os.MkdirAll(filepath.Dir(firmware), 0o750); err != nil {
			return fmt.Errorf("FAIL: create firmware directory: %w", err)
		}
		if err := os.WriteFile(firmware, []byte("firmware"), 0o600); err != nil {
			return fmt.Errorf("FAIL: write firmware: %w", err)
		}
	}

	stubs := filepath.Join(work, "stubs")
	if err := os.MkdirAll(stubs, 0o750); err != nil {
		return fmt.Errorf("FAIL: create stand-in directory: %w", err)
	}
	buildEnv := uiLeQemuRunAnswersEnvironment(os.Environ(), map[string]string{envCGOEnabled: "0"})
	standIn, err := buildQEMUStandIn(ctx, work, buildEnv)
	if err != nil {
		return err
	}
	for _, name := range []string{qemuName, programSSH} {
		if err := copyExecutable(standIn, filepath.Join(stubs, name)); err != nil {
			return fmt.Errorf("FAIL: install %s stand-in: %w", name, err)
		}
	}

	baseEnv := uiLeQemuRunAnswersEnvironment(os.Environ(), map[string]string{
		envPath:                  stubs + string(os.PathListSeparator) + os.Getenv("PATH"),
		envRepoRoot:              fixtureRoot,
		"XDG_CACHE_HOME":         cache,
		"HOMEBREW_PREFIX":        brew,
		"ZE_QEMU_SSH_PORT":       "2222",
		"ZE_QEMU_BOOT_TIMEOUT":   "30s",
		"ze.l2tp.ncp.ip-timeout": "9",
	})
	runLE := func(guestCode, record string, args ...string) uiLeQemuRunAnswersCommandResult {
		recordPath := filepath.Join(work, record)
		_ = os.Remove(recordPath)
		env := uiLeQemuRunAnswersEnvironment(baseEnv, map[string]string{
			"QEMU_GUEST_CODE": guestCode,
			"QEMU_RECORD":     recordPath,
		})
		return uiLeQemuRunAnswersRunCommand(ctx, work, env, binary, args...)
	}

	listing := runLE("0", "listing.ndjson", "qemu")
	if listing.code != 0 || !bytes.Contains(listing.stdout, []byte("run")) {
		return fmt.Errorf("FAIL: qemu listing did not publish run: %q %q", listing.stdout, listing.stderr)
	}

	base := []string{areaQEMU, actionRun, argCommand, "printf ui-proof", fieldPackages, "git bash", "timeout", "30s"}
	answerArgs := append(append([]string{}, base...), "|", "json")
	answer := runLE("0", "calls.ndjson", answerArgs...)
	if answer.code != 0 {
		return fmt.Errorf("FAIL: qemu run exited %d: %s", answer.code, uiLeQemuRunAnswersTail(answer.stderr))
	}
	var report map[string]any
	if err := json.Unmarshal(answer.stdout, &report); err != nil {
		return fmt.Errorf("FAIL: qemu run returned invalid JSON %q: %w", answer.stdout, err)
	}
	if report["verdict"] != verdictPass {
		return fmt.Errorf("FAIL: QEMU verdict is %#v", report)
	}
	plan, ok := report["plan"].(map[string]any)
	if !ok {
		return fmt.Errorf("FAIL: QEMU report has no object plan: %#v", report)
	}
	for _, key := range []string{"qemu-argv", "bootstrap-command", "setup-command", "ssh-port", "command-timeout-seconds", "alpine-version", "go-version"} {
		if _, exists := plan[key]; !exists {
			return fmt.Errorf("FAIL: run plan has no %q: %v", key, uiLeQemuRunAnswersSortedKeys(plan))
		}
	}
	if plan["command"] != "printf ui-proof" {
		return fmt.Errorf("FAIL: command changed: %#v", plan["command"])
	}
	packages, ok := plan["packages"].([]any)
	if !ok || len(packages) != 2 || packages[0] != "git" || packages[1] != "bash" {
		return fmt.Errorf("FAIL: packages changed: %#v", plan["packages"])
	}

	calls, err := readCalls(filepath.Join(work, "calls.ndjson"))
	if err != nil {
		return fmt.Errorf("FAIL: read QEMU calls: %w", err)
	}
	if len(calls) != 3 {
		return fmt.Errorf("FAIL: qemu run made %d commands, want 3: %#v", len(calls), calls)
	}
	wantPrograms := []string{areaQEMU, programSSH, programSSH}
	for i, call := range calls {
		if call.Program != wantPrograms[i] {
			return fmt.Errorf("FAIL: command order changed: %#v", calls)
		}
		if !contains(call.Dotted, "ze.l2tp.ncp.ip-timeout") {
			return fmt.Errorf("FAIL: dotted environment key was lost: %#v", call)
		}
	}

	failed := runLE("3", "failed.ndjson", answerArgs...)
	if failed.code != 3 {
		return fmt.Errorf("FAIL: guest exit 3 became %d: %s", failed.code, uiLeQemuRunAnswersTail(failed.stderr))
	}
	var failedReport map[string]any
	if err := json.Unmarshal(failed.stdout, &failedReport); err != nil {
		return fmt.Errorf("FAIL: guest failure report is invalid JSON %q: %w", failed.stdout, err)
	}
	if failedReport["verdict"] != "fail" {
		return fmt.Errorf("FAIL: guest failure report changed: %q", failed.stdout)
	}

	for _, operator := range []string{renderJSON, "ndjson", renderTable, "text", renderYAML, renderRaw, "no-more"} {
		args := append(append([]string{}, base...), "|", operator)
		rendered := runLE("0", operator+".ndjson", args...)
		if rendered.code != 0 {
			return fmt.Errorf("FAIL: global pipe %s exited %d: %s", operator, rendered.code, uiLeQemuRunAnswersTail(rendered.stderr))
		}
		if len(bytes.TrimSpace(rendered.stdout)) == 0 {
			return fmt.Errorf("FAIL: global pipe %s returned no structured answer", operator)
		}
	}

	logRecord := filepath.Join(work, "log.ndjson")
	logArgs := append(append([]string{}, base...), "|", "log")
	logged := runLE("0", "log.ndjson", logArgs...)
	if logged.code != 1 {
		return fmt.Errorf("FAIL: one-shot pipe log exited %d, want 1", logged.code)
	}
	if len(logged.stdout) != 0 {
		return fmt.Errorf("FAIL: one-shot pipe log returned data: %q", logged.stdout)
	}
	if string(logged.stderr) != "error: log requires a streaming command\n" {
		return fmt.Errorf("FAIL: one-shot pipe log refusal changed: %q", logged.stderr)
	}
	if _, err := os.Stat(logRecord); err == nil {
		return fmt.Errorf("FAIL: one-shot pipe log started the QEMU harness")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("FAIL: inspect one-shot pipe log record: %w", err)
	}

	savedPath := filepath.Join(work, "saved-answer.json")
	_ = os.Remove(savedPath)
	saveArgs := append(append([]string{}, base...), "|", "save", savedPath)
	saved := runLE("0", "save.ndjson", saveArgs...)
	if saved.code != 0 {
		return fmt.Errorf("FAIL: global pipe save exited %d: %s", saved.code, uiLeQemuRunAnswersTail(saved.stderr))
	}
	info, err := os.Stat(savedPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("FAIL: global pipe save wrote no answer")
	}

	fmt.Println("LE_QEMU_RUN_ANSWERS_OK")
	return nil
}

func buildQEMUStandIn(ctx context.Context, work string, env []string) (string, error) {
	sourceDir := filepath.Join(work, "qemu-stand-in-src")
	if err := os.MkdirAll(sourceDir, 0o750); err != nil {
		return "", fmt.Errorf("FAIL: create stand-in source directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "go.mod"), []byte("module qemu-stand-in\n\ngo 1.22\n"), 0o600); err != nil {
		return "", fmt.Errorf("FAIL: write stand-in module: %w", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "main.go"), []byte(qemuStandInSource), 0o600); err != nil {
		return "", fmt.Errorf("FAIL: write stand-in source: %w", err)
	}
	binary := filepath.Join(work, "qemu-stand-in")
	result := uiLeQemuRunAnswersRunCommand(ctx, sourceDir, env, "go", "build", "-o", binary, ".")
	if result.code != 0 {
		return "", fmt.Errorf("FAIL: compile QEMU stand-ins: %s", uiLeQemuRunAnswersTail(result.stderr))
	}
	return binary, nil
}

func copyExecutable(source, target string) error {
	in, err := os.Open(source) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return err
	}
	defer in.Close()                                                           //nolint:errcheck // fixture teardown
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755) //nolint:gosec // the fixture writes an executable stand-in, so it needs the execute bit
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Chmod(target, 0o755) //nolint:gosec // the fixture writes an executable stand-in, so it needs the execute bit
}

func uiLeQemuRunAnswersRunCommand(ctx context.Context, dir string, env []string, program string, args ...string) uiLeQemuRunAnswersCommandResult {
	cmd := exec.CommandContext(ctx, program, args...) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Dir = dir
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		code = -1
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			code = exitErr.ExitCode()
		}
	}
	return uiLeQemuRunAnswersCommandResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), code: code, err: err}
}

func uiLeQemuRunAnswersEnvironment(base []string, replacements map[string]string) []string {
	values := make(map[string]string, len(base)+len(replacements))
	for _, entry := range base {
		if key, value, found := strings.Cut(entry, "="); found {
			values[key] = value
		}
	}
	maps.Copy(values, replacements)
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+values[key])
	}
	return result
}

func readCalls(path string) ([]recordedCall, error) {
	file, err := os.Open(path) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint:errcheck // fixture teardown
	var calls []recordedCall
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if len(bytes.TrimSpace(scanner.Bytes())) == 0 {
			continue
		}
		var call recordedCall
		if err := json.Unmarshal(scanner.Bytes(), &call); err != nil {
			return nil, err
		}
		calls = append(calls, call)
	}
	return calls, scanner.Err()
}

func uiLeQemuRunAnswersSortedKeys(value map[string]any) []string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func contains(values []string, wanted string) bool {
	return slices.Contains(values, wanted)
}

// qemuRunTailBytes is how much of a failed run's stderr the report keeps.
const qemuRunTailBytes = 800

func uiLeQemuRunAnswersTail(value []byte) string {
	if len(value) > qemuRunTailBytes {
		value = value[len(value)-qemuRunTailBytes:]
	}
	return string(value)
}

const qemuStandInSource = `package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type call struct {
	Program string   ` + "`json:\"program\"`" + `
	Args    []string ` + "`json:\"args\"`" + `
	Dotted  []string ` + "`json:\"dotted\"`" + `
}

func main() {
	name := filepath.Base(os.Args[0])
	program := "ssh"
	if strings.HasPrefix(name, "qemu-system-") {
		program = "qemu"
	}
	dotted := make([]string, 0)
	for _, entry := range os.Environ() {
		key := entry
		if before, _, found := strings.Cut(entry, "="); found {
			key = before
		}
		if strings.ContainsRune(key, '.') {
			dotted = append(dotted, key)
		}
	}
	sort.Strings(dotted)
	record, err := os.OpenFile(os.Getenv("QEMU_RECORD"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(125)
	}
	if err := json.NewEncoder(record).Encode(call{Program: program, Args: os.Args[1:], Dotted: dotted}); err != nil {
		record.Close()
		fmt.Fprintln(os.Stderr, err)
		os.Exit(125)
	}
	if err := record.Close(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(125)
	}

	if program == "qemu" {
		fmt.Println("login:")
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), "SSHD_READY") {
				fmt.Println("SSHD_READY")
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(125)
		}
		return
	}

	if len(os.Args) != 0 && os.Args[len(os.Args)-1] == "true" {
		return
	}
	code := 0
	if text := os.Getenv("QEMU_GUEST_CODE"); text != "" {
		parsed, err := strconv.Atoi(text)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(125)
		}
		code = parsed
	}
	os.Exit(code)
}
`
