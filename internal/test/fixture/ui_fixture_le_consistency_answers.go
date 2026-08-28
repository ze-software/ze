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
	"sort"
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

	root := os.Getenv("ZE_REPO_ROOT")
	if root == "" {
		return fmt.Errorf("FAIL: ZE_REPO_ROOT is not set")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("FAIL: resolve ZE_REPO_ROOT: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("FAIL: stat ZE_REPO_ROOT: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("FAIL: ZE_REPO_ROOT is not a directory: %s", root)
	}

	work, err := os.MkdirTemp("", "le-consistency-answers-")
	if err != nil {
		return fmt.Errorf("FAIL: create fixture directory: %w", err)
	}
	defer os.RemoveAll(work)

	binary, err := uiLEBinary(root)
	if err != nil {
		return fmt.Errorf("FAIL: %w", err)
	}

	// Invoke the command from outside the checkout, as a developer may. The
	// command takes no path and must discover the real checkout independently
	// of its current working directory.
	bare, err := runProcess(ctx, work, binary, "consistency")
	if err != nil {
		return fmt.Errorf("FAIL: execute `le consistency`: %w", err)
	}
	if bare.stderr != "" {
		return fmt.Errorf("FAIL: `le consistency` wrote to stderr: %s", bare.stderr)
	}

	// Invoke the same compiled product from the checkout root. Compare reports
	// as multisets because report order is not part of the consistency contract.
	fromRoot, err := runProcess(ctx, root, binary, "consistency")
	if err != nil {
		return fmt.Errorf("FAIL: execute `le consistency` from the checkout: %w", err)
	}
	if fromRoot.stderr != "" {
		return fmt.Errorf("FAIL: `le consistency` from the checkout wrote to stderr: %s", fromRoot.stderr)
	}
	if bare.code != fromRoot.code {
		return fmt.Errorf("FAIL: `le consistency` exited %d outside the checkout and %d at its root", bare.code, fromRoot.code)
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
	if n := bagSize(outsideLines); n <= 100 {
		return fmt.Errorf("FAIL: the comparison ran over %d lines, which is too few to mean anything", n)
	}

	// Exercise the same answer through its data renderer. The payload must be a
	// report, and its finding totals and process status must agree with the bare
	// command.
	answer, err := runProcess(ctx, work, binary, "consistency", "|", "json")
	if err != nil {
		return fmt.Errorf("FAIL: execute `le consistency | json`: %w", err)
	}
	if answer.stderr != "" {
		return fmt.Errorf("FAIL: `le consistency | json` wrote to stderr: %s", answer.stderr)
	}

	var report map[string]json.RawMessage
	if err := json.Unmarshal([]byte(answer.stdout), &report); err != nil {
		return fmt.Errorf("FAIL: `le consistency | json` did not answer JSON: %v\n%s", err, uiLeConsistencyAnswersPrefix(answer.stdout, 400))
	}
	for _, key := range []string{"findings", "errors", "warnings"} {
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
	if answer.code != bare.code {
		return fmt.Errorf("FAIL: `| json` exited %d and the bare command exited %d", answer.code, bare.code)
	}
	if len(findings) == 0 {
		return fmt.Errorf("FAIL: the report answered no findings")
	}

	first := findings[0]
	for _, key := range []string{"severity", "check", "file", "message"} {
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
	counted, err := runProcess(ctx, work, binary, "consistency", "|", "count")
	if err != nil {
		return fmt.Errorf("FAIL: execute `le consistency | count`: %w", err)
	}
	wantCount := strconv.Itoa(len(findings))
	if !strings.Contains(counted.stdout, wantCount) {
		return fmt.Errorf("FAIL: `le consistency | count` answered %q, want %s", counted.stdout, wantCount)
	}

	fmt.Println("OK")
	return nil
}

func runProcess(ctx context.Context, dir, name string, args ...string) (processResult, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
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
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.code = exitErr.ExitCode()
		return result, nil
	}
	return processResult{}, err
}

func lineBag(text string) map[string]int {
	bag := make(map[string]int)
	for _, line := range strings.Split(ansiColor.ReplaceAllString(text, ""), "\n") {
		bag[line]++
	}
	return bag
}

func bagSize(bag map[string]int) int {
	total := 0
	for _, count := range bag {
		total += count
	}
	return total
}

func bagDifference(left, right map[string]int) []string {
	var difference []string
	for line, leftCount := range left {
		for n := leftCount - right[line]; n > 0; n-- {
			difference = append(difference, line)
		}
	}
	sort.Strings(difference)
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
	sort.Strings(keys)
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
