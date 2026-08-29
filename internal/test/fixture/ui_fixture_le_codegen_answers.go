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
	"strconv"
	"strings"
)

func init() {
	Register("ui/le-codegen-answers", uiDriver(runLECodegenAnswers))
}

type uiLeCodegenAnswersCommandResult struct {
	stdout string
	stderr string
	err    error
}

func (r uiLeCodegenAnswersCommandResult) exitCode() int {
	if r.err == nil {
		return 0
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](r.err); ok {
		return exitErr.ExitCode()
	}
	return -1
}

func (r uiLeCodegenAnswersCommandResult) output() string {
	return r.stdout + r.stderr
}

func uiLeCodegenAnswersRunCommand(ctx context.Context, dir string, env []string, name string, args ...string) uiLeCodegenAnswersCommandResult {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Dir = dir
	cmd.Env = env

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	return uiLeCodegenAnswersCommandResult{
		stdout: stdout.String(),
		stderr: stderr.String(),
		err:    err,
	}
}

func uiLeCodegenAnswersSetEnv(base []string, key, value string) []string {
	prefix := key + "="
	result := make([]string, 0, len(base)+1)
	for _, entry := range base {
		if !strings.HasPrefix(entry, prefix) {
			result = append(result, entry)
		}
	}
	return append(result, prefix+value)
}

func uiLeCodegenAnswersFailf(format string, args ...any) error {
	return fmt.Errorf("FAIL: "+format, args...)
}

func objectKeys(object map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func reportInt(report map[string]json.RawMessage, key string) (int, error) {
	raw, ok := report[key]
	if !ok {
		return 0, uiLeCodegenAnswersFailf("the report answered no %q key: %v", key, objectKeys(report))
	}
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, uiLeCodegenAnswersFailf("the report's %q value is not an integer: %v", key, err)
	}
	return value, nil
}

func runLECodegenAnswers(ctx context.Context) error {
	root := os.Getenv("ZE_REPO_ROOT")
	if root == "" {
		return uiLeCodegenAnswersFailf("ZE_REPO_ROOT is not set")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return uiLeCodegenAnswersFailf("resolve ZE_REPO_ROOT: %v", err)
	}

	work, err := os.MkdirTemp("", "le-codegen-answers-")
	if err != nil {
		return uiLeCodegenAnswersFailf("create temporary directory: %v", err)
	}
	defer os.RemoveAll(work) //nolint:errcheck // fixture cleanup

	binary, err := uiLEBinary(root)
	if err != nil {
		return uiLeCodegenAnswersFailf("%v", err)
	}

	productEnv := uiLeCodegenAnswersSetEnv(os.Environ(), "ZE_REPO_ROOT", root)
	runLE := func(args ...string) uiLeCodegenAnswersCommandResult {
		return uiLeCodegenAnswersRunCommand(ctx, work, productEnv, binary, args...)
	}

	generatedPaths := []string{
		"internal/component/web/page_assets.go",
		"internal/component/lg/page_assets.go",
		"internal/chaos/web/page_assets.go",
	}
	generatedBefore := make(map[string][]byte, len(generatedPaths))
	for _, name := range generatedPaths {
		data, err := os.ReadFile(filepath.Join(root, name)) //nolint:gosec // the path is the fixture's own scratch file
		if err != nil {
			return uiLeCodegenAnswersFailf("snapshot %s before read-only actions: %v", name, err)
		}
		generatedBefore[name] = data
	}

	// A successful check means the in-memory regeneration is byte-for-byte
	// equal to the committed output. Consequently, each corresponding write
	// action would emit exactly the bytes already in the checkout.
	generators := []string{
		"yang glue",
		"plugin imports",
		"feature-tags",
		"web-assets",
	}
	for _, name := range generators {
		checked := runLE(name, "check")
		if checked.exitCode() != 0 {
			return uiLeCodegenAnswersFailf("%s check is RED on this checkout, so the write proof says nothing:\n%s", name, checked.output())
		}
	}

	// One payload, rendered as JSON and as a count.
	answer := runLE("plugin imports", "check", "|", "json")
	var report map[string]json.RawMessage
	if err := json.Unmarshal([]byte(answer.stdout), &report); err != nil {
		preview := answer.stdout
		if len(preview) > 400 {
			preview = preview[:400]
		}
		return uiLeCodegenAnswersFailf("`le plugin-imports check | json` did not answer JSON: %v\n%s", err, preview)
	}

	for _, key := range []string{sectionPlugins, "schemas", "rpcs", "namespaces", "gated-groups", fieldFiles} {
		if _, ok := report[key]; !ok {
			return uiLeCodegenAnswersFailf("the report answered no %q key: %v", key, objectKeys(report))
		}
	}

	plugins, err := reportInt(report, "plugins")
	if err != nil {
		return err
	}
	schemas, err := reportInt(report, "schemas")
	if err != nil {
		return err
	}
	gatedGroups, err := reportInt(report, "gated-groups")
	if err != nil {
		return err
	}
	// Decode the remaining numeric fields too: their presence alone must not
	// permit a value of the wrong JSON type.
	if _, err := reportInt(report, "rpcs"); err != nil {
		return err
	}
	if _, err := reportInt(report, "namespaces"); err != nil {
		return err
	}

	var files []json.RawMessage
	if err := json.Unmarshal(report["files"], &files); err != nil {
		return uiLeCodegenAnswersFailf("the report's %q value is not a list: %v", "files", err)
	}
	if plugins <= 0 || schemas <= 0 {
		return uiLeCodegenAnswersFailf("the report read %d plugins and %d schemas, so it proved nothing", plugins, schemas)
	}
	// There is one composition root and at least one file for every gated
	// group. A tag can produce more than one file when its packages have
	// different build constraints, so this is deliberately a floor.
	if len(files) < 1+gatedGroups {
		return uiLeCodegenAnswersFailf("the report compared %d files against %d gated groups", len(files), gatedGroups)
	}

	counted := runLE("plugin imports", "check", "|", "count")
	if counted.exitCode() != 0 {
		return uiLeCodegenAnswersFailf("`le plugin-imports check | count` exited %d: %s", counted.exitCode(), counted.output())
	}
	if !strings.Contains(counted.stdout, strconv.Itoa(len(files))) {
		return uiLeCodegenAnswersFailf("`le plugin-imports check | count` answered %q, want %d", counted.stdout, len(files))
	}

	// web-assets pages is a document-shaped answer. JSON is supported, while
	// its values retain the distinction between pages with and without assets.
	pages := runLE("web-assets", "pages", "|", "json")
	var sets map[string][]json.RawMessage
	if err := json.Unmarshal([]byte(pages.stdout), &sets); err != nil {
		preview := pages.stdout
		if len(preview) > 400 {
			preview = preview[:400]
		}
		return uiLeCodegenAnswersFailf("`le web-assets pages | json` did not answer JSON: %v\n%s", err, preview)
	}
	if len(sets) < 8 {
		return uiLeCodegenAnswersFailf("the derived sets name %d pages, want at least eight", len(sets))
	}
	var hasLoadedPage bool
	var hasEmptyPage bool
	for _, assets := range sets {
		if len(assets) == 0 {
			hasEmptyPage = true
		} else {
			hasLoadedPage = true
		}
	}
	if !hasLoadedPage {
		return uiLeCodegenAnswersFailf("no page loads an asset, so the walk read nothing")
	}
	if !hasEmptyPage {
		return uiLeCodegenAnswersFailf("every page loads an asset, so the per-page derivation is doing nothing")
	}

	// pages is read-only. Compare bytes before and after so unrelated checkout
	// changes that predate this fixture do not masquerade as writes by pages.
	for _, name := range generatedPaths {
		after, err := os.ReadFile(filepath.Join(root, name)) //nolint:gosec // the path is the fixture's own scratch file
		if err != nil {
			return uiLeCodegenAnswersFailf("read %s after read-only actions: %v", name, err)
		}
		if !bytes.Equal(generatedBefore[name], after) {
			return uiLeCodegenAnswersFailf("a read-only action changed %s", name)
		}
	}

	// The action listing is the developer-facing boundary between checking and
	// writing. It must name both actions and mark each with the right effect.
	for _, name := range generators {
		listing := runLE(name)
		if listing.exitCode() != 0 {
			return uiLeCodegenAnswersFailf("`le %s` exited %d: %s", name, listing.exitCode(), listing.stderr)
		}
		for _, wanted := range []string{actionCheck, "write", wordWrites, fieldChecks} {
			if !strings.Contains(listing.stdout, wanted) {
				return uiLeCodegenAnswersFailf("%s: the listing does not name %q:\n%s", name, wanted, listing.stdout)
			}
		}
		for line := range strings.SplitSeq(listing.stdout, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "write ") && !strings.Contains(line, "writes") {
				return uiLeCodegenAnswersFailf("%s: the write action is not marked as writing: %q", name, line)
			}
			if strings.HasPrefix(trimmed, "check ") && !strings.Contains(line, "checks") {
				return uiLeCodegenAnswersFailf("%s: the check action is marked as writing: %q", name, line)
			}
		}
	}

	// A usage error is distinct from a gate that ran and found stale output.
	typo := runLE("yang glue", "chekc")
	if typo.exitCode() != 2 {
		return uiLeCodegenAnswersFailf("a mistyped action exited %d, want 2 so a caller can tell it from a failed gate", typo.exitCode())
	}

	fmt.Println("OK")
	return nil
}
