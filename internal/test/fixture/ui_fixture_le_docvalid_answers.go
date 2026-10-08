package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
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

	le, err := nativeLEBinary()
	if err != nil {
		return err
	}
	tree := filepath.Join(work, "tree")
	if err := leDocvalidTree(root, tree); err != nil {
		return err
	}
	owned := map[string]string{envRepoRoot: tree}
	// Native action/renderer assertions use stable owned inputs. The full
	// publication and documentation corpus is judged by its independent gates.
	drift, err := uiLeDocvalidAnswersRunCommand(ctx, work, owned, le, "doc", "yang-contract", "doc-drift")
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
	driftAgain, err := uiLeDocvalidAnswersRunCommand(ctx, work, owned, le, "doc", "yang-contract", "doc-drift")
	if err != nil {
		return err
	}
	if driftAgain.code != drift.code || joined(driftAgain) != joined(drift) {
		return fmt.Errorf("doc-drift changed over an unchanged checkout\nfirst: %q\nsecond: %q", joined(drift), joined(driftAgain))
	}

	driftData, err := uiLeDocvalidAnswersRunCommand(ctx, work, owned, le, "doc", "yang-contract", "doc-drift", "|", "json")
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
		return fmt.Errorf("the owned tree has %d documentation-drift findings: %#v", len(driftIssues), driftIssues)
	}
	// Prove the empty report came from evaluating the owned claims, not from
	// an empty input or a fixture-specific action implementation.
	claimPath := filepath.Join(tree, "docs", "architecture", "api", "text-parser.md")
	if err := os.WriteFile(claimPath, []byte("The parser uses strings.Fields.\n"), 0o600); err != nil {
		return err
	}
	broken, err := uiLeDocvalidAnswersRunCommand(ctx, work, owned, le, "doc", "yang-contract", "doc-drift", "|", "json")
	if err != nil {
		return err
	}
	var brokenIssues []map[string]any
	if err := json.Unmarshal(broken.stdout, &brokenIssues); err != nil {
		return fmt.Errorf("drifted claim did not produce JSON: %w\n%s", err, broken.stdout)
	}
	if broken.code != 1 || len(broken.stderr) != 0 || len(brokenIssues) != 1 ||
		brokenIssues[0][fieldFile] != "docs/architecture/api/text-parser.md" ||
		brokenIssues[0][fieldMessage] != "stale text parser claim references strings.Fields" {
		return fmt.Errorf("drifted claim answered exit %d, stderr %q, issues %#v", broken.code, broken.stderr, brokenIssues)
	}

	// Check the complete embedded product contract, not a minimum row count
	// that happens to match the current checkout's size.
	contract, err := uiLeDocvalidAnswersRunCommand(ctx, work, owned, le, "doc", "yang-contract", "command-contract")
	if err != nil {
		return err
	}
	if contract.code != 0 {
		return fmt.Errorf("command-contract exited %d\nstdout:\n%s\nstderr:\n%s", contract.code, contract.stdout, contract.stderr)
	}
	contractAgain, err := uiLeDocvalidAnswersRunCommand(ctx, work, owned, le, "doc", "yang-contract", "command-contract")
	if err != nil {
		return err
	}
	if contractAgain.code != contract.code || !bytes.Equal(contractAgain.stdout, contract.stdout) || !bytes.Equal(contractAgain.stderr, contract.stderr) {
		return fmt.Errorf("command-contract produced different ordered output over one unchanged tree")
	}

	contractData, err := uiLeDocvalidAnswersRunCommand(ctx, work, owned, le, "doc", "yang-contract", "command-contract", "|", "json")
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
		"orphan-rpcs",
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
	if len(lists["yang-commands"]) == 0 || len(lists["handlers"]) == 0 {
		return fmt.Errorf("command contract has no product commands or handlers")
	}
	for _, value := range lists["yang-commands"] {
		row, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("command contract YANG row has type %T, want a record", value)
		}
		for _, key := range []string{"wire-method", "yang-path", "module"} {
			text, ok := row[key].(string)
			if !ok || text == "" || !bytes.Contains(contract.stdout, []byte(text)) {
				return fmt.Errorf("human command table omits YANG row field %s: %#v", key, row)
			}
		}
	}
	for _, key := range []string{"orphan-yang", "orphan-handlers", "orphan-local-handlers", "orphan-rpcs"} {
		if len(lists[key]) != 0 {
			return fmt.Errorf("command contract reports %d entries in %s: %#v", len(lists[key]), key, lists[key])
		}
	}
	valid, ok := document["valid"].(bool)
	if !ok {
		return fmt.Errorf("command contract field %q has type %T, want a boolean", "valid", document["valid"])
	}
	if !valid {
		return fmt.Errorf("the command contract for the owned registration snapshot is invalid")
	}

	// count is rejected by action name before any checkout walk. A deliberately
	// absent root ensures that a different validation order cannot satisfy this.
	missingRoot := filepath.Join(work, "does-not-exist")
	counted, err := uiLeDocvalidAnswersRunCommand(ctx, work, map[string]string{envRepoRoot: missingRoot}, le, "doc", "yang-contract", "command-contract", "|", "count")
	if err != nil {
		return err
	}
	if counted.code == 0 {
		return fmt.Errorf("count was accepted over a document answer: %q", counted.stdout)
	}
	if !strings.Contains(string(counted.stderr), "count acts on rows") {
		return fmt.Errorf("count was refused for another reason: %q", counted.stderr)
	}

	listing, err := uiLeDocvalidAnswersRunCommand(ctx, work, owned, le, "doc", "yang-contract")
	if err != nil {
		return err
	}
	if listing.code != 0 {
		return fmt.Errorf("doc yang-contract listing exited %d: %s", listing.code, listing.stderr)
	}
	for _, wanted := range []string{"command-contract", "doc-drift", "pipe-operators-update", wordWrites, fieldChecks} {
		if !bytes.Contains(listing.stdout, []byte(wanted)) {
			return fmt.Errorf("doc yang-contract listing does not contain %q:\n%s", wanted, listing.stdout)
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
		return fmt.Errorf("doc yang-contract listing has no action row for pipe-operators-update")
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
		written, err := uiLeDocvalidAnswersRunCommand(ctx, work, map[string]string{envRepoRoot: tree}, le, "doc", "yang-contract", "pipe-operators-update")
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

// leDocvalidTree freezes the local registration sources once. The command
// contract still checks every embedded product YANG command and RPC; only its
// filesystem inputs are isolated from subsequent edits in the shared checkout.
// This is a document/source fixture, not a module or a publication checkout.
func leDocvalidTree(source, tree string) error {
	for _, directory := range []string{"cmd/ze", "internal"} {
		err := filepath.WalkDir(filepath.Join(source, directory), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || (entry.Name() != "register.go" && path != filepath.Join(source, "cmd", "ze", "main.go")) {
				return nil
			}
			relative, err := filepath.Rel(source, path)
			if err != nil {
				return err
			}
			body, err := os.ReadFile(path) //nolint:gosec // fixture snapshots repository source
			if err != nil {
				return err
			}
			destination := filepath.Join(tree, relative)
			if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
				return err
			}
			return os.WriteFile(destination, body, 0o600)
		})
		if err != nil {
			return fmt.Errorf("snapshot local command registrations: %w", err)
		}
	}
	reference, err := os.ReadFile(filepath.Join(source, filepath.FromSlash(generatedTable))) //nolint:gosec // repository reference
	if err != nil {
		return fmt.Errorf("read operator reference: %w", err)
	}
	for name, body := range map[string][]byte{
		generatedTable:                         reference,
		"docs/architecture/api/text-parser.md": []byte("The parser uses textparse.NewScanner.\n"),
	} {
		path := filepath.Join(tree, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return err
		}
		if err := os.WriteFile(path, body, 0o600); err != nil {
			return err
		}
	}
	return nil
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
	return childEnvironment(os.Environ(), overrides)
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
	slices.Sort(keys)
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
	slices.Sort(differing)
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
