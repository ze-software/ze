package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func init() {
	Register("ui/le-qemu-install-answers", uiDriver(leQEMUInstallAnswers))
}

type uiLeQemuInstallAnswersCommandResult struct {
	stdout   string
	stderr   string
	exitCode int
	startErr error
}

type qemuInstallAction struct {
	name   string
	prefix string
}

func leQEMUInstallAnswers(ctx context.Context) error {
	root := os.Getenv("ZE_REPO_ROOT")
	if root == "" {
		return fmt.Errorf("FAIL: ZE_REPO_ROOT is not set")
	}

	work, _, err := temporaryLEFixtureWorkspace("le-qemu-install-answers-")
	if err != nil {
		return fmt.Errorf("FAIL: create fixture working directory: %w", err)
	}
	defer os.RemoveAll(work)

	binary, err := uiLEBinary(root)
	if err != nil {
		return fmt.Errorf("FAIL: %w", err)
	}

	// Deliberately provide an empty executable search path. Each action must
	// report its missing-emulator skip before doing any installer work.
	emptyPath := filepath.Join(work, "empty-path")
	if err := os.MkdirAll(emptyPath, 0o777); err != nil {
		return fmt.Errorf("FAIL: creating empty executable path: %w", err)
	}

	kernel := filepath.Join(work, "installer-kernel")
	if err := os.WriteFile(kernel, []byte("kernel"), 0o666); err != nil {
		return fmt.Errorf("FAIL: writing installer kernel: %w", err)
	}

	runEnv := os.Environ()
	runEnv = envWith(runEnv, "PATH", emptyPath)
	runEnv = envWith(runEnv, "ZE_REPO_ROOT", root)
	runEnv = envWith(runEnv, "ZE_INSTALL_ARCH", "amd64")
	runEnv = envWith(runEnv, "ZE_INSTALL_KERNEL", kernel)

	actions := []qemuInstallAction{
		{name: "install-test", prefix: "INSTALL-QEMU"},
		{name: "install-iso-test", prefix: "INSTALL-ISO-QEMU"},
		{name: "install-scenarios-test", prefix: "INSTALL-SCENARIOS-QEMU"},
		{name: "install-ventoy-test", prefix: "INSTALL-VENTOY-QEMU"},
	}

	invokeLE := func(action string, pipes ...string) uiLeQemuInstallAnswersCommandResult {
		args := []string{"qemu", action}
		args = append(args, pipes...)
		return runQEMUInstallCommand(ctx, work, runEnv, binary, args...)
	}

	listing := runQEMUInstallCommand(ctx, work, runEnv, binary, "qemu")
	if listing.startErr != nil {
		return fmt.Errorf("FAIL: qemu listing failed: %v", listing.startErr)
	}
	if listing.exitCode != 0 {
		return fmt.Errorf("FAIL: qemu listing failed: %s", listing.stderr)
	}
	for _, action := range actions {
		if !strings.Contains(listing.stdout, action.name) {
			return fmt.Errorf("FAIL: qemu listing omitted %s: %s", action.name, listing.stdout)
		}
	}

	for _, action := range actions {
		bare := invokeLE(action.name)
		if bare.startErr != nil {
			return fmt.Errorf("FAIL: %s skip failed to start: %v", action.name, bare.startErr)
		}
		if bare.exitCode != 0 {
			return fmt.Errorf("FAIL: %s skip exited %d: %s", action.name, bare.exitCode, bare.stderr)
		}

		exact := fmt.Sprintf("%s: SKIP qemu-system-x86_64 not found\n", action.prefix)
		if bare.stdout != exact {
			return fmt.Errorf("FAIL: %s skip changed: %q want %q", action.name, bare.stdout, exact)
		}

		answer := invokeLE(action.name, "|", "json")
		if answer.startErr != nil {
			return fmt.Errorf("FAIL: %s json failed to start: %v", action.name, answer.startErr)
		}
		if answer.exitCode != 0 {
			return fmt.Errorf("FAIL: %s json exited %d: %s", action.name, answer.exitCode, answer.stderr)
		}

		var report map[string]any
		decoder := json.NewDecoder(strings.NewReader(answer.stdout))
		if err := decoder.Decode(&report); err != nil {
			return fmt.Errorf("FAIL: %s returned invalid json: %w", action.name, err)
		}

		if report["action"] != action.name {
			return fmt.Errorf("FAIL: %s report action changed: %v", action.name, report)
		}
		if report["verdict"] != "skip" {
			return fmt.Errorf("FAIL: %s report verdict changed: %v", action.name, report)
		}
		if report["arch"] != "amd64" || !qemuInstallJSONTruthy(report["accelerator"]) {
			return fmt.Errorf("FAIL: %s report lost plan data: %v", action.name, report)
		}
		reason, ok := report["reason"].(string)
		if !ok || !strings.Contains(reason, "qemu-system-x86_64 not found") {
			return fmt.Errorf("FAIL: %s report lost skip reason: %v", action.name, report)
		}
	}

	for _, operator := range []string{"json", "ndjson", "table", "text", "yaml", "raw", "no-more"} {
		rendered := invokeLE("install-test", "|", operator)
		if rendered.startErr != nil {
			return fmt.Errorf("FAIL: global pipe %s failed to start: %v", operator, rendered.startErr)
		}
		if rendered.exitCode != 0 {
			return fmt.Errorf("FAIL: global pipe %s exited %d: %s", operator, rendered.exitCode, rendered.stderr)
		}
		if strings.TrimSpace(rendered.stdout) == "" {
			return fmt.Errorf("FAIL: global pipe %s returned no answer", operator)
		}
	}

	logged := invokeLE("install-test", "|", "log", "|", "json")
	if logged.startErr != nil {
		return fmt.Errorf("FAIL: one-shot log refusal failed to start: %v", logged.startErr)
	}
	if logged.exitCode != 1 {
		return fmt.Errorf("FAIL: one-shot log refusal exited %d", logged.exitCode)
	}
	if logged.stdout != "" {
		return fmt.Errorf("FAIL: one-shot log refusal wrote stdout: %q", logged.stdout)
	}
	const logError = "error: log requires a streaming command\n"
	if logged.stderr != logError {
		return fmt.Errorf("FAIL: one-shot log refusal changed: %q", logged.stderr)
	}

	savedPath := filepath.Join(work, "install-answer.json")
	saved := invokeLE("install-test", "|", "save", savedPath)
	if saved.startErr != nil {
		return fmt.Errorf("FAIL: save pipe failed to start: %v", saved.startErr)
	}
	if saved.exitCode != 0 {
		return fmt.Errorf("FAIL: save pipe exited %d: %s", saved.exitCode, saved.stderr)
	}
	info, err := os.Stat(savedPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 {
		return fmt.Errorf("FAIL: save pipe wrote no install report")
	}

	fmt.Println("LE_QEMU_INSTALL_ANSWERS_OK")
	return nil
}

func qemuInstallFeatureTags(root string) (string, error) {
	contents, err := os.ReadFile(filepath.Join(root, "feature-gates.txt"))
	if err != nil {
		return "", err
	}

	found := make(map[string]struct{})
	for _, line := range strings.Split(string(contents), "\n") {
		words := strings.Fields(line)
		if len(words) != 0 && strings.HasPrefix(words[0], "ze_") {
			found[words[0]] = struct{}{}
		}
	}

	declared := make([]string, 0, len(found))
	for tag := range found {
		declared = append(declared, tag)
	}
	sort.Strings(declared)

	tags := append([]string{"ze_le"}, declared...)
	return strings.Join(tags, ","), nil
}

func runQEMUInstallCommand(
	ctx context.Context,
	dir string,
	env []string,
	program string,
	args ...string,
) uiLeQemuInstallAnswersCommandResult {
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Dir = dir
	cmd.Env = env

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := uiLeQemuInstallAnswersCommandResult{
		stdout:   stdout.String(),
		stderr:   stderr.String(),
		exitCode: 0,
	}
	if err == nil {
		return result
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.exitCode = exitErr.ExitCode()
		return result
	}

	result.exitCode = -1
	result.startErr = err
	return result
}

func envWith(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if !strings.HasPrefix(entry, prefix) {
			out = append(out, entry)
		}
	}
	return append(out, prefix+value)
}

func qemuInstallJSONTruthy(value any) bool {
	switch value := value.(type) {
	case nil:
		return false
	case bool:
		return value
	case string:
		return value != ""
	case float64:
		return value != 0
	case []any:
		return len(value) != 0
	case map[string]any:
		return len(value) != 0
	default:
		return true
	}
}
