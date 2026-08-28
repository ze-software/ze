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
	"sort"
	"strings"
)

func init() {
	Register("ui/le-go-extract-answers", uiDriver(leGoExtractAnswers))
}

const goExtractSource = `package sample

import "strings"

// Alpha says alpha.
func Alpha() string {
	return strings.ToUpper("alpha")
}

// Beta says beta.
func Beta() string {
	return "beta"
}
`

const goExtractSourceAfterBeta = `package sample

import "strings"

// Alpha says alpha.
func Alpha() string {
	return strings.ToUpper("alpha")
}
`

const goExtractBetaFile = `package sample

// Beta says beta.
func Beta() string {
	return "beta"
}
`

const goExtractSummary = "extracted 1 symbols (4 lines) from sample.go → beta.go\n"

type goExtractResult struct {
	stdout string
	stderr string
	code   int
}

func leGoExtractAnswers(ctx context.Context) error {
	checkout := os.Getenv("ZE_REPO_ROOT")
	if checkout == "" {
		return fmt.Errorf("FAIL: ZE_REPO_ROOT is not set")
	}

	here, _, err := temporaryLEFixtureWorkspace("le-go-extract-answers-")
	if err != nil {
		return fmt.Errorf("FAIL: create fixture directory: %w", err)
	}
	defer os.RemoveAll(here)

	le, err := uiLEBinary(checkout)
	if err != nil {
		return fmt.Errorf("FAIL: %w", err)
	}

	if _, err := exec.LookPath("goimports"); err != nil {
		return fmt.Errorf("FAIL: goimports is not installed; go-extract requires it")
	}

	commandDir, err := makeGoExtractTree(here, "by-command")
	if err != nil {
		return err
	}
	fromCommand, err := runGoExtractCommand(ctx, commandDir, le,
		"go-extract", "source", "sample.go", "dest", "beta.go", "symbol", "Beta")
	if err != nil {
		return fmt.Errorf("FAIL: run le go-extract: %w", err)
	}
	if fromCommand.code != 0 {
		return fmt.Errorf("FAIL: le go-extract exited %d: %s", fromCommand.code, fromCommand.stderr)
	}
	if fromCommand.stdout != goExtractSummary {
		return fmt.Errorf("FAIL: le go-extract answered %q, want %q", fromCommand.stdout, goExtractSummary)
	}

	byCommand, err := readGoExtractFiles(commandDir)
	if err != nil {
		return err
	}
	wantCommand := map[string]string{
		"beta.go":   goExtractBetaFile,
		"sample.go": goExtractSourceAfterBeta,
	}
	if !reflect.DeepEqual(byCommand, wantCommand) {
		return fmt.Errorf("FAIL: le go-extract wrote different files:\n got: %#v\nwant: %#v", byCommand, wantCommand)
	}
	if !strings.Contains(byCommand["beta.go"], "func Beta") {
		return fmt.Errorf("FAIL: the declaration did not move: %#v", byCommand)
	}
	if strings.Contains(byCommand["sample.go"], "func Beta") {
		return fmt.Errorf("FAIL: the source still holds the declaration: %#v", byCommand)
	}

	jsonDir, err := makeGoExtractTree(here, "by-json")
	if err != nil {
		return err
	}
	answer, err := runGoExtractCommand(ctx, jsonDir, le,
		"go-extract", "source", "sample.go", "dest", "beta.go", "symbol", "Beta", "|", "json")
	if err != nil {
		return fmt.Errorf("FAIL: run `le go-extract | json`: %w", err)
	}
	if answer.code != 0 {
		return fmt.Errorf("FAIL: `le go-extract | json` exited %d: %s", answer.code, answer.stderr)
	}

	var report map[string]any
	if err := json.Unmarshal([]byte(answer.stdout), &report); err != nil {
		preview := answer.stdout
		if len(preview) > 400 {
			preview = preview[:400]
		}
		return fmt.Errorf("FAIL: `le go-extract | json` did not answer JSON: %v\n%s", err, preview)
	}
	if report["source"] != "sample.go" {
		return fmt.Errorf("FAIL: source = %#v", report["source"])
	}
	if report["dest"] != "beta.go" {
		return fmt.Errorf("FAIL: dest = %#v", report["dest"])
	}
	symbols, ok := report["symbols"].([]any)
	if !ok {
		return fmt.Errorf("FAIL: symbols is not an array: %#v", report["symbols"])
	}
	if len(symbols) != 1 {
		return fmt.Errorf("FAIL: the answer names %d symbol(s), want 1", len(symbols))
	}
	row, ok := symbols[0].(map[string]any)
	if !ok {
		return fmt.Errorf("FAIL: the symbol row is not an object: %#v", symbols[0])
	}
	for _, key := range []string{"symbol", "first-line", "last-line"} {
		if _, ok := row[key]; !ok {
			return fmt.Errorf("FAIL: a symbol row carries no %q: %v", key, sortedGoExtractKeys(row))
		}
	}
	if row["symbol"] != "Beta" {
		return fmt.Errorf("FAIL: the row names %#v", row["symbol"])
	}
	first, firstOK := row["first-line"].(float64)
	last, lastOK := row["last-line"].(float64)
	if !firstOK || !lastOK {
		return fmt.Errorf("FAIL: the row has nonnumeric line bounds %#v..%#v", row["first-line"], row["last-line"])
	}
	if first >= last {
		return fmt.Errorf("FAIL: the row spans %v..%v", first, last)
	}

	countDir, err := makeGoExtractTree(here, "by-count")
	if err != nil {
		return err
	}
	counted, err := runGoExtractCommand(ctx, countDir, le,
		"go-extract", "source", "sample.go", "dest", "both.go",
		"symbol", "Alpha", "symbol", "Beta", "|", "count")
	if err != nil {
		return fmt.Errorf("FAIL: run `le go-extract | count`: %w", err)
	}
	if counted.code != 0 {
		return fmt.Errorf("FAIL: `le go-extract | count` exited %d: %s", counted.code, counted.stderr)
	}
	if !strings.Contains(counted.stdout, "2") {
		return fmt.Errorf("FAIL: `le go-extract | count` answered %q, want 2", counted.stdout)
	}

	yamlDir, err := makeGoExtractTree(here, "by-yaml")
	if err != nil {
		return err
	}
	asYAML, err := runGoExtractCommand(ctx, yamlDir, le,
		"go-extract", "source", "sample.go", "dest", "beta.go", "symbol", "Beta", "|", "yaml")
	if err != nil {
		return fmt.Errorf("FAIL: run `le go-extract | yaml`: %w", err)
	}
	if asYAML.code != 0 {
		return fmt.Errorf("FAIL: `le go-extract | yaml` exited %d", asYAML.code)
	}
	if !strings.Contains(asYAML.stdout, "Beta") {
		return fmt.Errorf("FAIL: `le go-extract | yaml` answered nothing usable:\n%s", asYAML.stdout)
	}

	bareDir, err := makeGoExtractTree(here, "by-bare")
	if err != nil {
		return err
	}
	refused, err := runGoExtractCommand(ctx, bareDir, le,
		"go-extract", "sample.go", "beta.go", "Beta")
	if err != nil {
		return fmt.Errorf("FAIL: run bare go-extract form: %w", err)
	}
	if refused.code != 1 {
		return fmt.Errorf("FAIL: a bare positional exited %d, want 1", refused.code)
	}
	if !strings.Contains(refused.stderr, "unknown keyword") {
		return fmt.Errorf("FAIL: the refusal does not name the problem:\n%s", refused.stderr)
	}
	bareFiles, err := readGoExtractFiles(bareDir)
	if err != nil {
		return err
	}
	if bareFiles["sample.go"] != goExtractSource {
		return fmt.Errorf("FAIL: a refused command edited the fixture")
	}

	typoDir, err := makeGoExtractTree(here, "by-typo")
	if err != nil {
		return err
	}
	typo, err := runGoExtractCommand(ctx, typoDir, le,
		"go-extract", "source", "sample.go", "dest", "beta.go",
		"symbol", "Beta", "symbol", "Zeta")
	if err != nil {
		return fmt.Errorf("FAIL: run go-extract with an absent symbol: %w", err)
	}
	if typo.code != 1 {
		return fmt.Errorf("FAIL: a symbol that is not there exited %d, want 1", typo.code)
	}
	if !strings.Contains(typo.stderr, "Zeta") {
		return fmt.Errorf("FAIL: the refusal does not name the symbol:\n%s", typo.stderr)
	}
	typoFiles, err := readGoExtractFiles(typoDir)
	if err != nil {
		return err
	}
	wantTypo := map[string]string{"sample.go": goExtractSource}
	if !reflect.DeepEqual(typoFiles, wantTypo) {
		return fmt.Errorf("FAIL: a refused move wrote something: %v", sortedGoExtractFileNames(typoFiles))
	}

	fmt.Println("OK")
	return nil
}

func makeGoExtractTree(parent, name string) (string, error) {
	path := filepath.Join(parent, name)
	if err := os.RemoveAll(path); err != nil {
		return "", fmt.Errorf("FAIL: remove %s: %w", path, err)
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", fmt.Errorf("FAIL: create %s: %w", path, err)
	}
	if err := os.WriteFile(filepath.Join(path, "sample.go"), []byte(goExtractSource), 0o644); err != nil {
		return "", fmt.Errorf("FAIL: write fixture in %s: %w", path, err)
	}
	return path, nil
}

func readGoExtractFiles(path string) (map[string]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("FAIL: read %s: %w", path, err)
	}
	out := make(map[string]string, len(entries))
	for _, entry := range entries {
		contents, err := os.ReadFile(filepath.Join(path, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("FAIL: read %s: %w", filepath.Join(path, entry.Name()), err)
		}
		out[entry.Name()] = string(contents)
	}
	return out, nil
}

func runGoExtractCommand(ctx context.Context, dir, program string, args ...string) (goExtractResult, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Dir = dir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := goExtractResult{stdout: stdout.String(), stderr: stderr.String()}
	if err == nil {
		return result, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.code = exitErr.ExitCode()
		return result, nil
	}
	return result, err
}

func sortedGoExtractKeys(row map[string]any) []string {
	keys := make([]string, 0, len(row))
	for key := range row {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedGoExtractFileNames(files map[string]string) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
