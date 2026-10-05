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
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

func init() {
	Register("ui/le-consistency-answers", uiDriver(runLEConsistencyAnswers))
}

const leConsistencyTimeout = 300 * time.Second

var ansiColor = regexp.MustCompile("\x1b\\[[0-9;]*m")

type processResult struct {
	stdout string
	stderr string
	code   int
}

func runLEConsistencyAnswers(parent context.Context) error {
	ctx, cancel := context.WithTimeout(parent, leConsistencyTimeout)
	defer cancel()

	work, err := os.MkdirTemp("", "le-consistency-answers-")
	if err != nil {
		return fmt.Errorf("FAIL: create fixture directory: %w", err)
	}
	defer os.RemoveAll(work) //nolint:errcheck // fixture cleanup

	binary, err := nativeLEBinary()
	if err != nil {
		return fmt.Errorf("FAIL: %w", err)
	}
	root := filepath.Join(work, "tree")
	if err := os.MkdirAll(filepath.Join(root, "internal", "sample"), 0o750); err != nil {
		return fmt.Errorf("FAIL: create owned consistency tree: %w", err)
	}
	// One source exercises both an error and a warning. The finding identities,
	// not the size of today's repository, make the comparison non-vacuous.
	name := filepath.Join(root, "internal", "sample", "sample.go")
	body := "package sample\n\ntype Sample struct { Value string `json:\"bad_name\"` }\n"
	if err := os.WriteFile(name, []byte(body), 0o600); err != nil {
		return fmt.Errorf("FAIL: write owned consistency input: %w", err)
	}
	run := func(dir string, args ...string) (processResult, error) {
		result, err := uiLeDocvalidAnswersRunCommand(ctx, dir, map[string]string{envRepoRoot: root}, binary, args...)
		return processResult{stdout: string(result.stdout), stderr: string(result.stderr), code: result.code}, err
	}

	// Invoke the command from outside the checkout, as a developer may. The
	// command takes no path and must discover the selected checkout independently
	// of its current working directory.
	bare, err := run(work, "doc", "consistency")
	if err != nil {
		return fmt.Errorf("FAIL: execute `le doc consistency`: %w", err)
	}
	if bare.stderr != "" {
		return fmt.Errorf("FAIL: `le doc consistency` wrote to stderr: %s", bare.stderr)
	}

	// Invoke the same compiled product from the checkout root. Compare reports
	// as multisets because report order is not part of the consistency contract.
	fromRoot, err := run(root, "doc", "consistency")
	if err != nil {
		return fmt.Errorf("FAIL: execute `le doc consistency` from the checkout: %w", err)
	}
	if fromRoot.stderr != "" {
		return fmt.Errorf("FAIL: `le doc consistency` from the checkout wrote to stderr: %s", fromRoot.stderr)
	}
	if bare.code != fromRoot.code {
		return fmt.Errorf("FAIL: `le doc consistency` exited %d outside the checkout and %d at its root", bare.code, fromRoot.code)
	}

	outsideLines := lineBag(bare.stdout)
	rootLines := lineBag(fromRoot.stdout)
	onlyOutside := bagDifference(outsideLines, rootLines)
	onlyRoot := bagDifference(rootLines, outsideLines)
	if len(onlyOutside) != 0 || len(onlyRoot) != 0 {
		return fmt.Errorf(
			"FAIL: the consistency report depends on the working directory:\n%s\n%s",
			formatDifference("only outside the checkout", onlyOutside),
			formatDifference("only at the checkout root", onlyRoot),
		)
	}

	// Exercise the same answer through its data renderer. The payload must be a
	// report, and its finding totals and process status must agree with the bare
	// command.
	answer, err := run(work, "doc", "consistency", "|", "json")
	if err != nil {
		return fmt.Errorf("FAIL: execute `le doc consistency | json`: %w", err)
	}
	if answer.stderr != "" {
		return fmt.Errorf("FAIL: `le doc consistency | json` wrote to stderr: %s", answer.stderr)
	}

	var report map[string]json.RawMessage
	if err := json.Unmarshal([]byte(answer.stdout), &report); err != nil {
		return fmt.Errorf("FAIL: `le doc consistency | json` did not answer JSON: %w\n%s", err, uiLeConsistencyAnswersPrefix(answer.stdout, 400))
	}
	for _, key := range []string{"findings", fieldErrors, "warnings"} {
		if _, ok := report[key]; !ok {
			return fmt.Errorf("FAIL: the report answered no %q key: %v", key, uiLeConsistencyAnswersSortedKeys(report))
		}
	}

	var findings []map[string]json.RawMessage
	if err := json.Unmarshal(report["findings"], &findings); err != nil {
		return fmt.Errorf("FAIL: the report's findings are not an array: %w", err)
	}
	errorsCount, err := uiLeConsistencyAnswersJsonInteger(report["errors"])
	if err != nil {
		return fmt.Errorf("FAIL: the report's errors count is not an integer: %w", err)
	}
	warningsCount, err := uiLeConsistencyAnswersJsonInteger(report["warnings"])
	if err != nil {
		return fmt.Errorf("FAIL: the report's warnings count is not an integer: %w", err)
	}
	if len(findings) != errorsCount+warningsCount {
		return fmt.Errorf(
			"FAIL: %d findings against %d errors and %d warnings",
			len(findings), errorsCount, warningsCount,
		)
	}
	if errorsCount != 1 || warningsCount != 1 || bare.code != 1 {
		return fmt.Errorf("FAIL: authored consistency findings: got %d errors, %d warnings, exit %d; want one each and exit 1", errorsCount, warningsCount, bare.code)
	}
	if answer.code != bare.code {
		return fmt.Errorf("FAIL: `| json` exited %d and the bare command exited %d", answer.code, bare.code)
	}
	if len(findings) == 0 {
		return fmt.Errorf("FAIL: the report answered no findings")
	}

	first := findings[0]
	for _, key := range []string{"severity", actionCheck, fieldFile, fieldMessage} {
		if _, ok := first[key]; !ok {
			return fmt.Errorf("FAIL: a finding carries no %q: %s", key, compactJSON(first))
		}
	}
	var firstFile string
	if err := json.Unmarshal(first["file"], &firstFile); err != nil {
		return fmt.Errorf("FAIL: a finding's file is not a string: %s", compactJSON(first))
	}
	if filepath.IsAbs(firstFile) {
		return fmt.Errorf("FAIL: a finding names an absolute path: %s", firstFile)
	}

	// Row operators act on findings rather than on the report envelope.
	counted, err := run(work, "doc", "consistency", "|", "count")
	if err != nil {
		return fmt.Errorf("FAIL: execute `le doc consistency | count`: %w", err)
	}
	wantCount := strconv.Itoa(len(findings))
	if counted.code != bare.code || counted.stderr != "" || strings.TrimSpace(counted.stdout) != wantCount {
		return fmt.Errorf("FAIL: `le doc consistency | count` answered %q, want %s", counted.stdout, wantCount)
	}
	wantFindings := map[string]string{"json-kebab-case": "ERROR", "design-refs": "WARN"}
	for _, finding := range findings {
		var file, check, severity string
		if err := json.Unmarshal(finding[fieldFile], &file); err != nil {
			return fmt.Errorf("FAIL: invalid finding file: %w", err)
		}
		if err := json.Unmarshal(finding[actionCheck], &check); err != nil {
			return fmt.Errorf("FAIL: invalid finding check: %w", err)
		}
		if err := json.Unmarshal(finding["severity"], &severity); err != nil {
			return fmt.Errorf("FAIL: invalid finding severity: %w", err)
		}
		if file != "internal/sample/sample.go" || wantFindings[check] != severity {
			return fmt.Errorf("FAIL: unexpected authored finding: %s", compactJSON(finding))
		}
		if !strings.Contains(ansiColor.ReplaceAllString(bare.stdout, ""), file) ||
			!strings.Contains(bare.stdout, check) {
			return fmt.Errorf("FAIL: human report omits authored finding %s: %s", check, bare.stdout)
		}
		delete(wantFindings, check)
	}
	if len(wantFindings) != 0 {
		return fmt.Errorf("FAIL: report omitted authored findings: %v", wantFindings)
	}

	fmt.Println("OK")
	return nil
}

func runProcess(ctx context.Context, dir, name string, args ...string) (processResult, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Dir = dir
	cmd.Env = os.Environ()
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := processResult{stdout: stdout.String(), stderr: stderr.String()}
	if err == nil {
		return result, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return processResult{}, ctxErr
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		result.code = exitErr.ExitCode()
		return result, nil
	}
	return processResult{}, err
}

func lineBag(text string) map[string]int {
	bag := make(map[string]int)
	for line := range strings.SplitSeq(ansiColor.ReplaceAllString(text, ""), "\n") {
		bag[line]++
	}
	return bag
}

func bagDifference(left, right map[string]int) []string {
	var difference []string
	for line, leftCount := range left {
		for n := leftCount - right[line]; n > 0; n-- {
			difference = append(difference, line)
		}
	}
	slices.Sort(difference)
	return difference
}

func formatDifference(label string, lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	if len(lines) > 10 {
		lines = lines[:10]
	}
	prefixed := make([]string, len(lines))
	for i, line := range lines {
		prefixed[i] = "  " + label + ": " + line
	}
	return strings.Join(prefixed, "\n")
}

func uiLeConsistencyAnswersJsonInteger(raw json.RawMessage) (int, error) {
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, err
	}
	return value, nil
}

func uiLeConsistencyAnswersSortedKeys(values map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func compactJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(encoded)
}

func uiLeConsistencyAnswersPrefix(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
