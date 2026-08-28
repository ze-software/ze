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
	Register("ui/le-binary-dispatches", uiDriver(leBinaryDispatches))
}

type uiLeBinaryDispatchesCommandResult struct {
	stdout   string
	stderr   string
	exitCode int
}

func leBinaryDispatches(ctx context.Context) error {
	root := os.Getenv("ZE_REPO_ROOT")
	if root == "" {
		return uiLeBinaryDispatchesFailf("ZE_REPO_ROOT is not set")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return uiLeBinaryDispatchesFailf("resolve ZE_REPO_ROOT: %v", err)
	}

	work, err := os.MkdirTemp("", "le-binary-dispatches-")
	if err != nil {
		return uiLeBinaryDispatchesFailf("create fixture directory: %v", err)
	}
	defer os.RemoveAll(work)

	tags, err := uiLEFeatureTags(root)
	if err != nil {
		return uiLeBinaryDispatchesFailf("%v", err)
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		return uiLeBinaryDispatchesFailf("find go: %v", err)
	}
	lePath := filepath.Join(work, "le")
	build := exec.CommandContext(ctx, goTool, "build", "-tags", strings.Join(tags, ","), "-o", lePath, "./cmd/ze")
	build.Dir = root
	build.Env = os.Environ()
	var buildOutput bytes.Buffer
	build.Stdout = &buildOutput
	build.Stderr = &buildOutput
	if err := build.Run(); err != nil {
		return uiLeBinaryDispatchesFailf("building the full le personality: %v\n%s", err, buildOutput.String())
	}

	le := func(args ...string) (uiLeBinaryDispatchesCommandResult, error) {
		cmd := exec.CommandContext(ctx, lePath, args...)
		cmd.Dir = work
		cmd.Env = os.Environ()
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		result := uiLeBinaryDispatchesCommandResult{
			stdout:   stdout.String(),
			stderr:   stderr.String(),
			exitCode: 0,
		}
		if err == nil {
			return result, nil
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.exitCode = exitErr.ExitCode()
			return result, nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return result, uiLeBinaryDispatchesFailf("le %s: %v", strings.Join(args, " "), ctxErr)
		}
		return result, uiLeBinaryDispatchesFailf("execute le %s: %v", strings.Join(args, " "), err)
	}

	// The usage page reads back the composition root: a tool that failed to
	// register is absent from this list.
	usage, err := le("--help")
	if err != nil {
		return err
	}
	if usage.exitCode != 0 {
		return uiLeBinaryDispatchesFailf("le --help exited %d", usage.exitCode)
	}
	usageText := usage.stderr + usage.stdout
	for _, command := range []string{"perf-bench", "docs-to-code", "token-economy"} {
		if !strings.Contains(usageText, command) {
			return uiLeBinaryDispatchesFailf("le --help does not list the %s command", command)
		}
	}

	// The performance nudge is advisory and must never block the build.
	listing, err := le("perf-bench", "|", "json")
	if err != nil {
		return err
	}
	if listing.exitCode != 0 {
		return uiLeBinaryDispatchesFailf("`le perf-bench | json` exited %d", listing.exitCode)
	}
	listingPayload, err := decodeObject(listing.stdout)
	if err != nil {
		return uiLeBinaryDispatchesFailf("decode `le perf-bench | json`: %v", err)
	}
	verbs, err := actionVerbs(listingPayload)
	if err != nil {
		return err
	}
	if _, ok := verbs["suggestion-report"]; !ok {
		return uiLeBinaryDispatchesFailf("the perf-bench area lost suggestion-report: %v", sortedSet(verbs))
	}
	if _, ok := verbs["record"]; !ok {
		return uiLeBinaryDispatchesFailf("the perf-bench area lists no record verb: %v", sortedSet(verbs))
	}

	nudge, err := le("perf-bench", "suggestion-report", "|", "json")
	if err != nil {
		return err
	}
	if nudge.exitCode != 0 {
		return uiLeBinaryDispatchesFailf("the perf nudge exited %d; it is advisory and never blocks", nudge.exitCode)
	}
	report, err := decodeObject(nudge.stdout)
	if err != nil {
		return uiLeBinaryDispatchesFailf("decode the perf nudge: %v", err)
	}
	if _, ok := report["origin"]; !ok {
		return uiLeBinaryDispatchesFailf("the nudge answered no origin: %v", uiLeBinaryDispatchesSortedKeys(report))
	}

	// Check the source-anchor reverse index and preserve the whole gate names.
	docs, err := le("docs-to-code", "|", "json")
	if err != nil {
		return err
	}
	if docs.exitCode != 0 {
		return uiLeBinaryDispatchesFailf("`le docs-to-code | json` exited %d", docs.exitCode)
	}
	docsPayload, err := decodeObject(docs.stdout)
	if err != nil {
		return uiLeBinaryDispatchesFailf("decode `le docs-to-code | json`: %v", err)
	}
	docVerbs, err := actionVerbs(docsPayload)
	if err != nil {
		return err
	}
	for _, verb := range []string{"check", "update", "index-check", "index-update"} {
		if _, ok := docVerbs[verb]; !ok {
			return uiLeBinaryDispatchesFailf("the docs-to-code area lost %q: %v", verb, sortedSet(docVerbs))
		}
	}

	anchors, err := le("docs-to-code", "index-check", "|", "json")
	if err != nil {
		return err
	}
	if anchors.exitCode != 0 && anchors.exitCode != 1 {
		return uiLeBinaryDispatchesFailf("the anchor check exited %d, want a verdict", anchors.exitCode)
	}
	checked, err := decodeObject(anchors.stdout)
	if err != nil {
		return uiLeBinaryDispatchesFailf("decode the anchor check: %v", err)
	}
	paths, err := uiLeBinaryDispatchesInteger(checked, "paths")
	if err != nil {
		return err
	}
	if paths <= 100 {
		return uiLeBinaryDispatchesFailf("the anchor check read %d code paths, so it proved nothing", paths)
	}
	checkedFile, err := stringValue(checked, "file")
	if err != nil {
		return err
	}
	if checkedFile != "ai/CODE-TO-DOCS.md" {
		return uiLeBinaryDispatchesFailf("the check names %q rather than the index it is about", checkedFile)
	}

	// Build an isolated transcript store so runtime and results do not depend on
	// a developer's machine-local corpus.
	store := filepath.Join(work, "store", "-fixture")
	if err := os.MkdirAll(store, 0o755); err != nil {
		return uiLeBinaryDispatchesFailf("create transcript store: %v", err)
	}
	call := map[string]any{
		"type":       "assistant",
		"session_id": "fix00001",
		"sessionId":  "fix00001",
		"message": map[string]any{
			"id": "msg_1",
			"usage": map[string]any{
				"input_tokens":                10,
				"cache_creation_input_tokens": 20,
				"cache_read_input_tokens":     120000,
				"output_tokens":               30,
			},
			"content": []any{map[string]any{
				"type": "tool_use",
				"id":   "toolu_1",
				"name": "Read",
				"input": map[string]any{
					"file_path": "internal/a.go",
				},
			}},
		},
	}
	result := map[string]any{
		"type": "user",
		"message": map[string]any{
			"content": []any{map[string]any{
				"type":        "tool_result",
				"tool_use_id": "toolu_1",
				"content":     strings.Repeat("x", 3600),
			}},
		},
	}
	transcriptPath := filepath.Join(store, "fix00001.jsonl")
	transcript, err := os.OpenFile(transcriptPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return uiLeBinaryDispatchesFailf("create transcript: %v", err)
	}
	encoder := json.NewEncoder(transcript)
	for _, record := range []any{call, call, result} {
		if err := encoder.Encode(record); err != nil {
			transcript.Close()
			return uiLeBinaryDispatchesFailf("write transcript: %v", err)
		}
	}
	if err := transcript.Close(); err != nil {
		return uiLeBinaryDispatchesFailf("close transcript: %v", err)
	}

	fixtureArgs := []string{"token-economy", "root", filepath.Dir(store), "project", "-fixture"}
	economy, err := le(fixtureArgs...)
	if err != nil {
		return err
	}
	if economy.exitCode != 0 {
		return uiLeBinaryDispatchesFailf("`le token-economy` exited %d; it is a report and never blocks", economy.exitCode)
	}
	if !strings.Contains(economy.stdout, "API calls: 1") {
		return uiLeBinaryDispatchesFailf("two records of one call were not deduped:\n%s", economy.stdout)
	}
	if !strings.Contains(economy.stdout, "Store: "+store) {
		return uiLeBinaryDispatchesFailf("the report does not name the corpus it read:\n%s", economy.stdout)
	}

	economyJSONArgs := append(append([]string{}, fixtureArgs...), "|", "json")
	economyJSON, err := le(economyJSONArgs...)
	if err != nil {
		return err
	}
	if economyJSON.exitCode != 0 {
		return uiLeBinaryDispatchesFailf("`le token-economy | json` exited %d", economyJSON.exitCode)
	}
	payload, err := decodeObject(economyJSON.stdout)
	if err != nil {
		return uiLeBinaryDispatchesFailf("decode `le token-economy | json`: %v", err)
	}
	for _, key := range []string{"state", "store", "project", "totals", "histogram", "capped", "cap", "top"} {
		if _, ok := payload[key]; !ok {
			return uiLeBinaryDispatchesFailf("the report answered no %q key: %v", key, uiLeBinaryDispatchesSortedKeys(payload))
		}
	}
	totals, err := object(payload, "totals")
	if err != nil {
		return err
	}
	calls, err := uiLeBinaryDispatchesInteger(totals, "calls")
	if err != nil {
		return err
	}
	if calls != 1 {
		return uiLeBinaryDispatchesFailf("the payload counted %d calls, want 1", calls)
	}
	state, err := stringValue(payload, "state")
	if err != nil {
		return err
	}
	if state != "ok" {
		return uiLeBinaryDispatchesFailf("the payload state is %q, want 'ok'", state)
	}

	// An absent store is stated explicitly, never rendered as a zero-valued
	// report that could be mistaken for free work.
	absent, err := le("token-economy", "root", work, "project", "-nothing-here")
	if err != nil {
		return err
	}
	if absent.exitCode != 0 {
		return uiLeBinaryDispatchesFailf("an absent store exited %d, want 0", absent.exitCode)
	}
	if !strings.Contains(absent.stdout, "Found no transcript store there") {
		return uiLeBinaryDispatchesFailf("an absent store did not say so:\n%s", absent.stdout)
	}

	refused, err := le("token-economy", "cap", "0")
	if err != nil {
		return err
	}
	if refused.exitCode == 0 {
		return uiLeBinaryDispatchesFailf("a cap below the bound was accepted")
	}

	unknown, err := le("no-such-tool")
	if err != nil {
		return err
	}
	if unknown.exitCode == 0 {
		return uiLeBinaryDispatchesFailf("le accepted a command nobody registered")
	}

	fmt.Println("OK")
	return nil
}

func uiLeBinaryDispatchesFailf(format string, args ...any) error {
	return fmt.Errorf("FAIL: "+format, args...)
}

func decodeObject(text string) (map[string]any, error) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func uiLeBinaryDispatchesInteger(values map[string]any, key string) (int64, error) {
	value, ok := values[key]
	if !ok {
		return 0, uiLeBinaryDispatchesFailf("payload answered no %q key", key)
	}
	number, ok := value.(json.Number)
	if !ok {
		return 0, uiLeBinaryDispatchesFailf("payload key %q is not an integer: %T", key, value)
	}
	result, err := number.Int64()
	if err != nil {
		return 0, uiLeBinaryDispatchesFailf("payload key %q is not an integer: %v", key, err)
	}
	return result, nil
}

func stringValue(values map[string]any, key string) (string, error) {
	value, ok := values[key]
	if !ok {
		return "", uiLeBinaryDispatchesFailf("payload answered no %q key", key)
	}
	result, ok := value.(string)
	if !ok {
		return "", uiLeBinaryDispatchesFailf("payload key %q is not a string: %T", key, value)
	}
	return result, nil
}

func object(values map[string]any, key string) (map[string]any, error) {
	value, ok := values[key]
	if !ok {
		return nil, uiLeBinaryDispatchesFailf("payload answered no %q key", key)
	}
	result, ok := value.(map[string]any)
	if !ok {
		return nil, uiLeBinaryDispatchesFailf("payload key %q is not an object: %T", key, value)
	}
	return result, nil
}

func array(values map[string]any, key string) ([]any, error) {
	value, ok := values[key]
	if !ok {
		return nil, uiLeBinaryDispatchesFailf("payload answered no %q key", key)
	}
	result, ok := value.([]any)
	if !ok {
		return nil, uiLeBinaryDispatchesFailf("payload key %q is not an array: %T", key, value)
	}
	return result, nil
}

func actionVerbs(payload map[string]any) (map[string]struct{}, error) {
	actions, err := array(payload, "actions")
	if err != nil {
		return nil, err
	}
	verbs := make(map[string]struct{}, len(actions))
	for index, raw := range actions {
		action, ok := raw.(map[string]any)
		if !ok {
			return nil, uiLeBinaryDispatchesFailf("actions item %d is not an object: %T", index, raw)
		}
		verb, err := stringValue(action, "verb")
		if err != nil {
			return nil, err
		}
		verbs[verb] = struct{}{}
	}
	return verbs, nil
}

func uiLeBinaryDispatchesSortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedSet(values map[string]struct{}) []string {
	items := make([]string, 0, len(values))
	for value := range values {
		items = append(items, value)
	}
	sort.Strings(items)
	return items
}

func uiLeBinaryDispatchesFirstBytes(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
