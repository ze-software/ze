package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
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
	root := os.Getenv("ZE_REPO_ROOT")
	if root == "" {
		return errors.New("FAIL: ZE_REPO_ROOT is not set")
	}

	here, _, err := temporaryLEFixtureWorkspace("le-inventory-answers-")
	if err != nil {
		return fmt.Errorf("FAIL: create fixture directory: %w", err)
	}
	defer os.RemoveAll(here) //nolint:errcheck // fixture cleanup

	binary, err := uiLEBinary(root)
	if err != nil {
		return fmt.Errorf("FAIL: %w", err)
	}

	// The inventory is rooted in the checkout and must not depend on the
	// caller's working directory. Its timestamp is the sole variable field.
	inventoryAtRoot, err := executeClean(ctx, root, binary, "inventory")
	if err != nil {
		return err
	}
	inventoryAtFixture, err := executeClean(ctx, here, binary, "inventory")
	if err != nil {
		return err
	}
	if inventoryAtRoot.code != inventoryAtFixture.code {
		return fmt.Errorf("FAIL: inventory exited %d in the checkout and %d in the fixture directory", inventoryAtRoot.code, inventoryAtFixture.code)
	}
	if inventoryAtRoot.code != 0 {
		return fmt.Errorf("FAIL: inventory exited %d", inventoryAtRoot.code)
	}

	rootPage := generatedLine.ReplaceAllString(inventoryAtRoot.stdout, "Generated: <when>")
	fixturePage := generatedLine.ReplaceAllString(inventoryAtFixture.stdout, "Generated: <when>")
	if rootPage != fixturePage {
		left := strings.Split(rootPage, "\n")
		right := strings.Split(fixturePage, "\n")
		for i := range maxInt(len(left), len(right)) {
			a := "<end>"
			b := "<end>"
			if i < len(left) {
				a = left[i]
			}
			if i < len(right) {
				b = right[i]
			}
			if a != b {
				return fmt.Errorf("FAIL: inventory pages differ at line %d:\n  checkout: %s\n  fixture:  %s", i+1, a, b)
			}
		}
		return errors.New("FAIL: inventory pages differ")
	}
	if lines := len(strings.Split(rootPage, "\n")); lines <= 100 {
		return fmt.Errorf("FAIL: the inventory check ran over %d lines, which is too few to mean anything", lines)
	}

	// The command registry has the same checkout-wide, working-directory
	// independent contract, including row ordering.
	commandsAtRoot, err := executeClean(ctx, root, binary, "command list")
	if err != nil {
		return err
	}
	commandsAtFixture, err := executeClean(ctx, here, binary, "command list")
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
	answer, err := executeClean(ctx, here, binary, "inventory", "|", "json")
	if err != nil {
		return err
	}
	if answer.code != 0 {
		return fmt.Errorf("FAIL: `le inventory | json` exited %d", answer.code)
	}
	var inventory map[string]any
	if err := json.Unmarshal([]byte(answer.stdout), &inventory); err != nil {
		return fmt.Errorf("FAIL: `le inventory | json` did not answer JSON: %w\n%s", err, uiLeInventoryAnswersPrefix(answer.stdout, 400))
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
	plugins, ok := inventory["plugins"].([]any)
	if !ok {
		return fmt.Errorf("FAIL: inventory plugins have type %T, want an array", inventory["plugins"])
	}
	if len(plugins) <= 10 {
		return fmt.Errorf("FAIL: inventory answered %d plugins, which is too few to be the product", len(plugins))
	}

	listing, err := executeClean(ctx, here, binary, "command list", "|", "json")
	if err != nil {
		return err
	}
	if listing.code != 0 {
		return fmt.Errorf("FAIL: `le command list | json` exited %d", listing.code)
	}
	var commands []map[string]any
	if err := json.Unmarshal([]byte(listing.stdout), &commands); err != nil {
		return fmt.Errorf("FAIL: `le command list | json` did not answer a JSON array: %w\n%s", err, uiLeInventoryAnswersPrefix(listing.stdout, 400))
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
	counted, err := executeClean(ctx, here, binary, "command list", "|", "count")
	if err != nil {
		return err
	}
	if counted.code != 0 {
		return fmt.Errorf("FAIL: `le command list | count` exited %d", counted.code)
	}
	wantCount := strconv.Itoa(len(commands))
	if !strings.Contains(counted.stdout, wantCount) {
		return fmt.Errorf("FAIL: `le command list | count` answered %q, want %d", counted.stdout, len(commands))
	}

	// Inventory is one document containing several row sets. There is no
	// unambiguous row set for count to consume, so the chain must be refused.
	refused, err := uiLeInventoryAnswersExecute(ctx, here, binary, "inventory", "|", "count")
	if err != nil {
		return fmt.Errorf("FAIL: start refused inventory chain: %w", err)
	}
	if refused.code == 0 {
		return errors.New("FAIL: `le inventory | count` was accepted")
	}
	if refused.stdout != "" {
		return fmt.Errorf("FAIL: a refused chain wrote to stdout: %q", refused.stdout)
	}

	fmt.Println("OK")
	return nil
}

func executeClean(ctx context.Context, dir, name string, args ...string) (uiLeInventoryAnswersCommandResult, error) {
	result, err := uiLeInventoryAnswersExecute(ctx, dir, name, args...)
	if err != nil {
		return uiLeInventoryAnswersCommandResult{}, fmt.Errorf("FAIL: start %q: %w", append([]string{name}, args...), err)
	}
	if result.stderr != "" {
		return uiLeInventoryAnswersCommandResult{}, fmt.Errorf("FAIL: %q wrote to stderr: %s", append([]string{name}, args...), result.stderr)
	}
	return result, nil
}

func uiLeInventoryAnswersExecute(ctx context.Context, dir, name string, args ...string) (uiLeInventoryAnswersCommandResult, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Dir = dir
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
	sort.Strings(keys)
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
