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
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

func init() {
	Register("ui/le-inventory-answers", uiDriver(runLEInventoryAnswers))
}

type uiLeInventoryAnswersCommandResult struct {
	stdout string
	stderr string
	code   int
}

var generatedLine = regexp.MustCompile(`(?m)^Generated: .*$`)

func runLEInventoryAnswers(ctx context.Context) error {
	here, _, err := temporaryLEFixtureWorkspace("le-inventory-answers-")
	if err != nil {
		return fmt.Errorf("FAIL: create fixture directory: %w", err)
	}
	defer os.RemoveAll(here) //nolint:errcheck // fixture cleanup

	binary, err := nativeLEBinary()
	if err != nil {
		return fmt.Errorf("FAIL: %w", err)
	}

	// Own the counted inputs; the plugin and command registries still come
	// from the actual full-feature native binary.
	root := filepath.Join(here, "tree")
	if err := leInventoryTree(root); err != nil {
		return err
	}
	childEnv := childEnvironment(os.Environ(), map[string]string{envRepoRoot: root})
	rootPage, fixturePage, err := inventoryOverAStillTree(ctx, root, here, binary, childEnv)
	if err != nil {
		return err
	}
	if rootPage != fixturePage {
		return fmt.Errorf("FAIL: inventory pages differ, so the answer depends on the working directory: %s",
			firstDifference(rootPage, fixturePage))
	}
	if lines := len(strings.Split(rootPage, "\n")); lines <= 100 {
		return fmt.Errorf("FAIL: the inventory check ran over %d lines, which is too few to mean anything", lines)
	}

	// The command registry has the same checkout-wide, working-directory
	// independent contract, including row ordering.
	commandsAtRoot, err := executeClean(ctx, root, childEnv, binary, "cli list")
	if err != nil {
		return err
	}
	commandsAtFixture, err := executeClean(ctx, here, childEnv, binary, "cli list")
	if err != nil {
		return err
	}
	if commandsAtRoot.stdout != commandsAtFixture.stdout {
		return errors.New("FAIL: command-list pages differ between the checkout and fixture directory")
	}
	if commandsAtRoot.code != commandsAtFixture.code {
		return fmt.Errorf("FAIL: command-list exited %d in the checkout and %d in the fixture directory", commandsAtRoot.code, commandsAtFixture.code)
	}
	if commandsAtRoot.code != 0 {
		return fmt.Errorf("FAIL: command-list exited %d", commandsAtRoot.code)
	}

	// One inventory payload must expose every documented top-level data set.
	answer, err := executeClean(ctx, here, childEnv, binary, "repo", "inventory", "|", "json")
	if err != nil {
		return err
	}
	if answer.code != 0 {
		return fmt.Errorf("FAIL: `le repo inventory | json` exited %d", answer.code)
	}
	var inventory map[string]any
	if err := json.Unmarshal([]byte(answer.stdout), &inventory); err != nil {
		return fmt.Errorf("FAIL: `le repo inventory | json` did not answer JSON: %w\n%s", err, uiLeInventoryAnswersPrefix(answer.stdout, 400))
	}
	for _, key := range []string{
		sectionPlugins,
		"families",
		"yang-modules",
		"rpc-list",
		"total-rpcs",
		"test-counts",
		"package-stats",
		"generated",
	} {
		if _, ok := inventory[key]; !ok {
			return fmt.Errorf("FAIL: inventory answered no %q key: %v", key, uiLeInventoryAnswersSortedKeys(inventory))
		}
	}
	// These values come from the authored tree, not from another rendering or
	// a snapshot of today's checkout. They prove the requested root was read.
	var owned map[string]any
	if err := json.Unmarshal([]byte(`{
		"total-rpcs": 2,
		"rpc-list": [
			{"name":"clear-widget","module":"fixture.yang","covered":false},
			{"name":"show-widget","module":"fixture.yang","covered":true}
		],
		"test-counts":{"widget":1},
		"package-stats":[
			{"area":"internal/","packages":1,"files":1,"lines":3},
			{"area":"pkg/","packages":0,"files":0,"lines":0},
			{"area":"cmd/","packages":1,"files":1,"lines":3}
		]
	}`), &owned); err != nil {
		return fmt.Errorf("FAIL: decode authored inventory expectations: %w", err)
	}
	for key, want := range owned {
		if !reflect.DeepEqual(inventory[key], want) {
			return fmt.Errorf("FAIL: owned inventory %s = %#v, want %#v", key, inventory[key], want)
		}
	}
	for _, row := range []string{"| RPCs | 2 |", "| RPCs with .ci coverage | 1/2 |", "| .ci test files | 1 |", "| Go packages | 2 |", "| Go files | 2 |", "| Go lines | 6 |"} {
		if !strings.Contains(rootPage, row) {
			return fmt.Errorf("FAIL: inventory page omitted authored count %q", row)
		}
	}
	plugins, ok := inventory["plugins"].([]any)
	if !ok {
		return fmt.Errorf("FAIL: inventory plugins have type %T, want an array", inventory["plugins"])
	}
	if len(plugins) <= 10 {
		return fmt.Errorf("FAIL: inventory answered %d plugins, which is too few to be the product", len(plugins))
	}

	listing, err := executeClean(ctx, here, childEnv, binary, "cli list", "|", "json")
	if err != nil {
		return err
	}
	if listing.code != 0 {
		return fmt.Errorf("FAIL: `le cli list | json` exited %d", listing.code)
	}
	var commands []map[string]any
	if err := json.Unmarshal([]byte(listing.stdout), &commands); err != nil {
		return fmt.Errorf("FAIL: `le cli list | json` did not answer a JSON array: %w\n%s", err, uiLeInventoryAnswersPrefix(listing.stdout, 400))
	}
	if len(commands) == 0 {
		return errors.New("FAIL: the command list answered an empty array")
	}
	for _, key := range []string{"verb", fieldPath, "source"} {
		if _, ok := commands[0][key]; !ok {
			return fmt.Errorf("FAIL: a command carries no %q: %v", key, commands[0])
		}
	}

	// A row operator acts on command rows and answers a number rather than the
	// rendered page.
	counted, err := executeClean(ctx, here, childEnv, binary, "cli list", "|", "count")
	if err != nil {
		return err
	}
	if counted.code != 0 {
		return fmt.Errorf("FAIL: `le cli list | count` exited %d", counted.code)
	}
	wantCount := strconv.Itoa(len(commands))
	if strings.TrimSpace(counted.stdout) != wantCount {
		return fmt.Errorf("FAIL: `le cli list | count` answered %q, want %d", counted.stdout, len(commands))
	}

	// Inventory is one document containing several row sets. There is no
	// unambiguous row set for count to consume, so the chain must be refused.
	refused, err := uiLeInventoryAnswersExecute(ctx, here, childEnv, binary, "repo", "inventory", "|", "count")
	if err != nil {
		return fmt.Errorf("FAIL: start refused inventory chain: %w", err)
	}
	if refused.code == 0 {
		return errors.New("FAIL: `le repo inventory | count` was accepted")
	}
	if refused.stdout != "" {
		return fmt.Errorf("FAIL: a refused chain wrote to stdout: %q", refused.stdout)
	}

	fmt.Println("OK")
	return nil
}

func executeClean(ctx context.Context, dir string, childEnv []string, name string, args ...string) (uiLeInventoryAnswersCommandResult, error) {
	result, err := uiLeInventoryAnswersExecute(ctx, dir, childEnv, name, args...)
	if err != nil {
		return uiLeInventoryAnswersCommandResult{}, fmt.Errorf("FAIL: start %q: %w", append([]string{name}, args...), err)
	}
	if result.stderr != "" {
		return uiLeInventoryAnswersCommandResult{}, fmt.Errorf("FAIL: %q wrote to stderr: %s", append([]string{name}, args...), result.stderr)
	}
	return result, nil
}

func uiLeInventoryAnswersExecute(ctx context.Context, dir string, childEnv []string, name string, args ...string) (uiLeInventoryAnswersCommandResult, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Dir = dir
	cmd.Env = childEnv
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := uiLeInventoryAnswersCommandResult{stdout: stdout.String(), stderr: stderr.String()}
	if err == nil {
		return result, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		result.code = exitErr.ExitCode()
		return result, nil
	}
	return uiLeInventoryAnswersCommandResult{}, err
}

func uiLeInventoryAnswersSortedKeys(value map[string]any) []string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func uiLeInventoryAnswersPrefix(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// inventoryOverAStillTree compares exactly two readings of immutable owned
// inputs. No settling loop can conceal a nondeterministic answer.
func inventoryOverAStillTree(ctx context.Context, root, here, binary string, childEnv []string) (string, string, error) {
	atRoot, err := executeClean(ctx, root, childEnv, binary, "repo", "inventory")
	if err != nil {
		return "", "", err
	}
	outside, err := executeClean(ctx, here, childEnv, binary, "repo", "inventory")
	if err != nil {
		return "", "", err
	}
	if atRoot.code != 0 || outside.code != 0 {
		return "", "", fmt.Errorf("FAIL: inventory exited %d at its root and %d outside it", atRoot.code, outside.code)
	}
	return generatedLine.ReplaceAllString(atRoot.stdout, "Generated: <when>"),
		generatedLine.ReplaceAllString(outside.stdout, "Generated: <when>"), nil
}

func leInventoryTree(root string) error {
	files := map[string]string{
		"go.mod":                            "module fixture.invalid/inventory\n\ngo 1.26\n",
		"internal/widget/widget.go":         "package widget\n\nconst Name = \"widget\"\n",
		"internal/widget/yang/fixture.yang": "module fixture {\n namespace \"urn:fixture\";\n prefix f;\n rpc show-widget {\n }\n rpc clear-widget {\n }\n}\n",
		"cmd/widget/main.go":                "package main\n\nfunc main() {}\n",
		"test/widget/show.ci":               "cmd=foreground:exec=ze show widget\nexpect=exit:code=0\n",
	}
	for relative, body := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return fmt.Errorf("FAIL: create inventory input directory: %w", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			return fmt.Errorf("FAIL: write inventory input %s: %w", relative, err)
		}
	}
	return nil
}

// pastTheEnd stands for a line one of two readings does not have, so a page that
// merely grew reads as a difference at the first line the shorter one lacks.
const pastTheEnd = "<end>"

// firstDifference names the first line two pages disagree on, so the reader sees
// WHAT differs rather than that something does. Both callers want that: one
// compares two readings of one tree, the other two working directories.
func firstDifference(left, right string) string {
	a := strings.Split(left, "\n")
	b := strings.Split(right, "\n")
	for i := range maxInt(len(a), len(b)) {
		first, second := pastTheEnd, pastTheEnd
		if i < len(a) {
			first = a[i]
		}
		if i < len(b) {
			second = b[i]
		}
		if first != second {
			return fmt.Sprintf("line %d reads %q and %q", i+1, first, second)
		}
	}

	return "no line differs, so the two are equal"
}
