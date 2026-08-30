package fixture

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	leDocvalidFixture = "ui/le-docvalid-answers"
	generatedTable    = "docs/features/pipe-operators.generated.md"
)

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func init() {
	Register(leDocvalidFixture, uiDriver(leDocvalidAnswers))
}

type uiLeDocvalidAnswersCommandResult struct {
	stdout []byte
	stderr []byte
	code   int
}

func leDocvalidAnswers(ctx context.Context) error {
	root := os.Getenv("ZE_REPO_ROOT")
	if root == "" {
		return fmt.Errorf("ZE_REPO_ROOT is not set")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve ZE_REPO_ROOT: %w", err)
	}

	work, err := os.MkdirTemp("", "ze-le-docvalid-answers-")
	if err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	defer os.RemoveAll(work) //nolint:errcheck // fixture cleanup

	tags, err := uiLeDocvalidAnswersFeatureTags(filepath.Join(root, "feature-gates.txt"))
	if err != nil {
		return err
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("find go: %w", err)
	}
	le := filepath.Join(work, "le")
	build := exec.CommandContext(ctx, goTool, "build", "-tags", tags, "-o", le, "./cmd/ze") //nolint:gosec // the fixture chooses the program and its arguments
	build.Dir = root
	build.Env = uiLeDocvalidAnswersEnvironment(map[string]string{envCGOEnabled: "0"})
	var buildOutput bytes.Buffer
	build.Stdout = &buildOutput
	build.Stderr = &buildOutput
	if err := build.Run(); err != nil {
		return fmt.Errorf("build full le personality: %w\n%s", err, buildOutput.String())
	}
	// The real checkout must pass the drift gate independently of optional
	// sibling publication checkouts. Its human rendering must be a stable,
	// nonempty clean report, while its data rendering must carry an empty
	// issues row set.
	drift, err := uiLeDocvalidAnswersRunCommand(ctx, work, nil, le, "docvalid", "doc-drift")
	if err != nil {
		return err
	}
	if drift.code != 0 {
		return fmt.Errorf("doc-drift exited %d\n%s", drift.code, joined(drift))
	}
	driftReport := strings.TrimSpace(joined(drift))
	if driftReport == "" || strings.Contains(driftReport, "\n") {
		return fmt.Errorf("doc-drift did not emit one clean report line: %q", joined(drift))
	}
	driftAgain, err := uiLeDocvalidAnswersRunCommand(ctx, work, nil, le, "docvalid", "doc-drift")
	if err != nil {
		return err
	}
	if driftAgain.code != drift.code || joined(driftAgain) != joined(drift) {
		return fmt.Errorf("doc-drift changed over an unchanged checkout\nfirst: %q\nsecond: %q", joined(drift), joined(driftAgain))
	}

	driftData, err := uiLeDocvalidAnswersRunCommand(ctx, work, nil, le, "docvalid", "doc-drift", "|", "json")
	if err != nil {
		return err
	}
	var driftIssues []map[string]any
	if err := json.Unmarshal(driftData.stdout, &driftIssues); err != nil {
		return fmt.Errorf("doc-drift data is not an issue array: %w\n%s", err, uiLeDocvalidAnswersFirstBytes(driftData.stdout, 400))
	}
	if driftData.code != drift.code {
		return fmt.Errorf("doc-drift data exited %d; human rendering exited %d", driftData.code, drift.code)
	}
	for i, finding := range driftIssues {
		for _, key := range []string{fieldFile, fieldMessage} {
			if _, ok := finding[key]; !ok {
				return fmt.Errorf("finding %d has no %q field: %#v", i, key, finding)
			}
		}
	}
	if len(driftIssues) != 0 {
		return fmt.Errorf("the real checkout has %d documentation-drift findings: %#v", len(driftIssues), driftIssues)
	}

	// The contract is intentionally large. Check both the stable human table
	// and every field of the document answer rather than sampling a few rows.
	contract, err := uiLeDocvalidAnswersRunCommand(ctx, work, nil, le, "docvalid", "command-contract")
	if err != nil {
		return err
	}
	if contract.code != 0 {
		return fmt.Errorf("command-contract exited %d\nstdout:\n%s\nstderr:\n%s", contract.code, contract.stdout, contract.stderr)
	}
	contractLines := strings.Split(string(contract.stdout), "\n")
	if len(contractLines) <= 100 {
		return fmt.Errorf("command-contract rendered %d lines, too few to cover the product", len(contractLines))
	}
	contractAgain, err := uiLeDocvalidAnswersRunCommand(ctx, work, nil, le, "docvalid", "command-contract")
	if err != nil {
		return err
	}
	if contractAgain.code != contract.code || !bytes.Equal(contractAgain.stdout, contract.stdout) || !bytes.Equal(contractAgain.stderr, contract.stderr) {
		return fmt.Errorf("command-contract produced different ordered output over one unchanged tree")
	}

	contractData, err := uiLeDocvalidAnswersRunCommand(ctx, work, nil, le, "docvalid", "command-contract", "|", "json")
	if err != nil {
		return err
	}
	var document map[string]any
	if err := json.Unmarshal(contractData.stdout, &document); err != nil {
		return fmt.Errorf("command-contract data is not a document: %w\n%s", err, uiLeDocvalidAnswersFirstBytes(contractData.stdout, 400))
	}
	if contractData.code != contract.code {
		return fmt.Errorf("command-contract data exited %d; human rendering exited %d", contractData.code, contract.code)
	}

	listNames := []string{
		"yang-commands",
		"handlers",
		"local-handlers",
		"orphan-yang",
		"orphan-handlers",
		"orphan-local-handlers",
		"skipped-handlers",
	}
	lists := make(map[string][]any, len(listNames))
	for _, key := range listNames {
		value, ok := document[key]
		if !ok {
			return fmt.Errorf("command contract has no %q field; fields are %v", key, uiLeDocvalidAnswersSortedKeys(document))
		}
		if value == nil {
			lists[key] = nil
			continue
		}
		list, ok := value.([]any)
		if !ok {
			return fmt.Errorf("command contract field %q has type %T, want an array", key, value)
		}
		lists[key] = list
	}

	for total, list := range map[string]string{
		"total-yang":           "yang-commands",
		"total-handlers":       "handlers",
		"total-local-handlers": "local-handlers",
	} {
		got, err := uiLeDocvalidAnswersJsonInteger(document, total)
		if err != nil {
			return err
		}
		if got != len(lists[list]) {
			return fmt.Errorf("command contract says %s=%d but %s has %d rows", total, got, list, len(lists[list]))
		}
	}
	if len(lists["yang-commands"]) <= 100 {
		return fmt.Errorf("command contract contains %d YANG commands, too few to be the product", len(lists["yang-commands"]))
	}
	for _, key := range []string{"orphan-yang", "orphan-handlers"} {
		if len(lists[key]) != 0 {
			return fmt.Errorf("command contract reports %d entries in %s: %#v", len(lists[key]), key, lists[key])
		}
	}
	valid, ok := document["valid"].(bool)
	if !ok {
		return fmt.Errorf("command contract field %q has type %T, want a boolean", "valid", document["valid"])
	}
	if !valid {
		return fmt.Errorf("the command contract for the real checkout is invalid")
	}

	// count is rejected by action name before any checkout walk. A deliberately
	// absent root ensures that a different validation order cannot satisfy this.
	missingRoot := filepath.Join(work, "does-not-exist")
	counted, err := uiLeDocvalidAnswersRunCommand(ctx, work, map[string]string{envRepoRoot: missingRoot}, le, "docvalid", "command-contract", "|", "count")
	if err != nil {
		return err
	}
	if counted.code == 0 {
		return fmt.Errorf("count was accepted over a document answer: %q", counted.stdout)
	}
	if !strings.Contains(string(counted.stderr), "count acts on rows") {
		return fmt.Errorf("count was refused for another reason: %q", counted.stderr)
	}

	listing, err := uiLeDocvalidAnswersRunCommand(ctx, work, nil, le, "docvalid")
	if err != nil {
		return err
	}
	if listing.code != 0 {
		return fmt.Errorf("docvalid listing exited %d: %s", listing.code, listing.stderr)
	}
	for _, wanted := range []string{"command-contract", "doc-drift", "pipe-operators-update", wordWrites, fieldChecks} {
		if !bytes.Contains(listing.stdout, []byte(wanted)) {
			return fmt.Errorf("docvalid listing does not contain %q:\n%s", wanted, listing.stdout)
		}
	}
	foundWriter := false
	for line := range strings.SplitSeq(string(listing.stdout), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "pipe-operators-update ") {
			foundWriter = true
			if !strings.Contains(line, "writes") {
				return fmt.Errorf("generator is not marked as writing: %q", line)
			}
		}
	}
	if !foundWriter {
		return fmt.Errorf("docvalid listing has no action row for pipe-operators-update")
	}

	// Drive the writer over two isolated roots. Both begin stale, both must be
	// overwritten, and every resulting byte and report must be deterministic.
	publishedPath := filepath.Join(root, filepath.FromSlash(generatedTable))
	publishedBefore, err := os.ReadFile(publishedPath) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return fmt.Errorf("read published generated table: %w", err)
	}

	trees := []string{filepath.Join(work, "first-tree"), filepath.Join(work, "second-tree")}
	for _, tree := range trees {
		path := filepath.Join(tree, filepath.FromSlash(generatedTable))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return fmt.Errorf("create isolated tree: %w", err)
		}
		if err := os.WriteFile(path, []byte("# a stale table\n"), 0o600); err != nil {
			return fmt.Errorf("seed stale generated table: %w", err)
		}
	}

	writes := make([]uiLeDocvalidAnswersCommandResult, 0, len(trees))
	for _, tree := range trees {
		written, err := uiLeDocvalidAnswersRunCommand(ctx, work, map[string]string{envRepoRoot: tree}, le, "docvalid", "pipe-operators-update")
		if err != nil {
			return err
		}
		if written.code != 0 {
			return fmt.Errorf("writer for %s exited %d\nstdout:\n%s\nstderr:\n%s", tree, written.code, written.stdout, written.stderr)
		}
		if !strings.Contains(joined(written), generatedTable) {
			return fmt.Errorf("writer report does not name %s: %q", generatedTable, joined(written))
		}
		writes = append(writes, written)
	}
	if joined(writes[0]) != joined(writes[1]) {
		return fmt.Errorf("identical writes emitted different reports\nfirst: %q\nsecond: %q", joined(writes[0]), joined(writes[1]))
	}

	firstFiles, err := digestTree(trees[0])
	if err != nil {
		return err
	}
	secondFiles, err := digestTree(trees[1])
	if err != nil {
		return err
	}
	if differing := differingFiles(firstFiles, secondFiles); len(differing) != 0 {
		return fmt.Errorf("identical writes left different trees behind: %v", firstN(differing, 10))
	}
	generated, ok := firstFiles[generatedTable]
	if !ok {
		return fmt.Errorf("writer did not create %s", generatedTable)
	}
	if bytes.Contains(generated, []byte("stale")) {
		return fmt.Errorf("writer did not overwrite the stale table")
	}
	if !bytes.Equal(generated, publishedBefore) {
		return fmt.Errorf("writer output differs from the table published by the checkout")
	}
	publishedAfter, err := os.ReadFile(publishedPath) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return fmt.Errorf("re-read published generated table: %w", err)
	}
	if !bytes.Equal(publishedBefore, publishedAfter) {
		return fmt.Errorf("the writing action modified the real checkout")
	}

	fmt.Println("OK")
	return nil
}

func uiLeDocvalidAnswersFeatureTags(path string) (string, error) {
	file, err := os.Open(path) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return "", fmt.Errorf("open feature gates: %w", err)
	}
	defer file.Close() //nolint:errcheck // fixture teardown

	found := make(map[string]struct{})
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 0 && strings.HasPrefix(fields[0], "ze_") {
			found[fields[0]] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read feature gates: %w", err)
	}

	declared := make([]string, 0, len(found))
	for tag := range found {
		declared = append(declared, tag)
	}
	sort.Strings(declared)
	return strings.Join(append([]string{buildTagLE, "ze_docvalid_fixture"}, declared...), ","), nil
}

func uiLeDocvalidAnswersRunCommand(ctx context.Context, dir string, overrides map[string]string, name string, args ...string) (uiLeDocvalidAnswersCommandResult, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Dir = dir
	cmd.Env = uiLeDocvalidAnswersEnvironment(overrides)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := uiLeDocvalidAnswersCommandResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), code: -1}
	if cmd.ProcessState != nil {
		result.code = cmd.ProcessState.ExitCode()
	}
	if err == nil {
		return result, nil
	}
	if _, ok := errors.AsType[*exec.ExitError](err); ok {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return result, ctxErr
		}
		return result, nil
	}
	return result, err
}

func uiLeDocvalidAnswersEnvironment(overrides map[string]string) []string {
	values := make(map[string]string)
	for _, entry := range os.Environ() {
		if key, value, ok := strings.Cut(entry, "="); ok {
			values[key] = value
		}
	}
	maps.Copy(values, overrides)
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	answer := make([]string, 0, len(keys))
	for _, key := range keys {
		answer = append(answer, key+"="+values[key])
	}
	return answer
}

func joined(result uiLeDocvalidAnswersCommandResult) string {
	return ansiEscape.ReplaceAllString(string(result.stdout)+string(result.stderr), "")
}

func uiLeDocvalidAnswersJsonInteger(document map[string]any, key string) (int, error) {
	value, ok := document[key]
	if !ok {
		return 0, fmt.Errorf("command contract has no %q field; fields are %v", key, uiLeDocvalidAnswersSortedKeys(document))
	}
	number, ok := value.(float64)
	if !ok || number < 0 || number != float64(int(number)) {
		return 0, fmt.Errorf("command contract field %q is not a nonnegative integer: %#v", key, value)
	}
	return int(number), nil
}

func uiLeDocvalidAnswersSortedKeys(document map[string]any) []string {
	keys := make([]string, 0, len(document))
	for key := range document {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func digestTree(root string) (map[string][]byte, error) {
	answer := make(map[string][]byte)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		contents, err := os.ReadFile(path) //nolint:gosec // the path is the fixture's own scratch file
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		answer[filepath.ToSlash(relative)] = contents
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("digest %s: %w", root, err)
	}
	return answer, nil
}

func differingFiles(left, right map[string][]byte) []string {
	all := make(map[string]struct{}, len(left)+len(right))
	for path := range left {
		all[path] = struct{}{}
	}
	for path := range right {
		all[path] = struct{}{}
	}
	var differing []string
	for path := range all {
		leftBytes, leftOK := left[path]
		rightBytes, rightOK := right[path]
		if !leftOK || !rightOK || !bytes.Equal(leftBytes, rightBytes) {
			differing = append(differing, path)
		}
	}
	sort.Strings(differing)
	return differing
}

func uiLeDocvalidAnswersFirstBytes(value []byte, count int) []byte {
	if len(value) <= count {
		return value
	}
	return value[:count]
}

func firstN(values []string, count int) []string {
	if len(values) <= count {
		return values
	}
	return values[:count]
}
