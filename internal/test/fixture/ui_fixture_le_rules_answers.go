package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func init() {
	Register("ui/le-rules-answers", uiDriver(leRulesAnswers))
}

type uiLeRulesAnswersCommandResult struct {
	code   int
	stdout string
	stderr string
}

func leRulesAnswers(ctx context.Context) error {
	root := os.Getenv("ZE_REPO_ROOT")
	if root == "" {
		return errors.New("FAIL: ZE_REPO_ROOT is not set")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("FAIL: resolve ZE_REPO_ROOT: %w", err)
	}

	work, err := os.MkdirTemp("", "le-rules-answers-")
	if err != nil {
		return fmt.Errorf("FAIL: create fixture directory: %w", err)
	}
	defer os.RemoveAll(work)

	binary, err := uiLEBinary(root)
	if err != nil {
		return fmt.Errorf("FAIL: %w", err)
	}

	sourceRoot := root
	root = filepath.Join(work, "checkout")
	if err := exportHEAD(ctx, sourceRoot, root); err != nil {
		return fmt.Errorf("FAIL: export HEAD for the rules corpus: %w", err)
	}

	le := func(tree string, args ...string) (uiLeRulesAnswersCommandResult, error) {
		env := uiLeRulesAnswersSetEnv(os.Environ(), "ZE_REPO_ROOT", tree)
		return uiLeRulesAnswersExecute(ctx, work, env, binary, args...)
	}

	// The four read-only corpus gates run against the real checkout. In addition
	// to their verdicts, retain the page contracts proving that each gate read a
	// non-empty population. Product diagnostics belong on stdout for these
	// successful answers, and checkout paths are relative.
	readOnly := []struct {
		name    string
		args    []string
		markers []string
	}{
		{"rules lint", []string{"rules", "lint"}, []string{"rule point(s) state an RFC 2119 level"}},
		{"rules render-check", []string{"rules", "render-check"}, []string{"rules are fresh"}},
		{"rules points-roundtrip-check", []string{"rules", "points-roundtrip-check"}, []string{"round-trip byte-identical"}},
		{"rules gate-map-report", []string{"rules", "gate-map-report"}, []string{"gate map: ", "PUBLISHED: "}},
	}
	for _, gate := range readOnly {
		answer, err := le(root, gate.args...)
		if err != nil {
			return err
		}
		if answer.code != 0 {
			return uiLeRulesAnswersFailf("%s: this checkout does not pass the gate (exit %d): %s%s", gate.name, answer.code, answer.stdout, answer.stderr)
		}
		if answer.stderr != "" {
			return uiLeRulesAnswersFailf("%s: the command wrote to stderr: %s", gate.name, answer.stderr)
		}
		if strings.Contains(answer.stdout, root+string(os.PathSeparator)) {
			return uiLeRulesAnswersFailf("%s: the page exposed an absolute checkout path: %s", gate.name, answer.stdout)
		}
		for _, marker := range gate.markers {
			if !strings.Contains(answer.stdout, marker) {
				return uiLeRulesAnswersFailf("%s: the page does not carry %q: %s", gate.name, marker, answer.stdout)
			}
		}
	}

	// The writing renderer runs over a complete small checkout. Its resulting
	// rule is asserted byte-for-byte, not merely by its status page.
	const oldRule = "# A\n\n**When:** when it happens\n**Severity:** blocking\n\n## S\n\n- **MUST do the OLD thing.**\n"
	const newRule = "# A\n\n**When:** when it happens\n**Severity:** blocking\n\n## S\n\n- **MUST do the NEW thing.**\n"
	fixtureFiles := map[string]string{
		"go.mod":                          "module example.com/fixture\n",
		"feature-gates.txt":               "ze_core\n",
		"ai/rules/aaa.md":                 oldRule,
		"ai/rules/points/aaa/manifest.md": "---\ntitle: A\nwhen: when it happens\nseverity: blocking\n---\ns ## S\n  p\n",
		"ai/rules/points/aaa/s/p.md":      "---\nkind: directive\nlevel: MUST\n---\n- **MUST do the NEW thing.**\n",
	}

	updatedTree := filepath.Join(work, "render-update")
	if err := writeFixture(updatedTree, fixtureFiles); err != nil {
		return err
	}
	updated, err := le(updatedTree, "rules", "render-update")
	if err != nil {
		return err
	}
	if updated.code != 0 {
		return uiLeRulesAnswersFailf("render-update exited %d: %s%s", updated.code, updated.stdout, updated.stderr)
	}
	if updated.stderr != "" {
		return uiLeRulesAnswersFailf("render-update wrote to stderr: %s", updated.stderr)
	}
	if updated.stdout == "" {
		return uiLeRulesAnswersFailf("render-update produced no status page")
	}
	if strings.Contains(updated.stdout, updatedTree+string(os.PathSeparator)) {
		return uiLeRulesAnswersFailf("render-update exposed an absolute fixture path: %s", updated.stdout)
	}
	if err := requireFile(filepath.Join(updatedTree, "ai", "rules", "aaa.md"), newRule, "render-update wrote the wrong rule"); err != nil {
		return err
	}

	// Check mode reports a unified diff on stdout, answers one, and leaves the
	// stale rule byte-identical.
	staleTree := filepath.Join(work, "stale")
	if err := writeFixture(staleTree, fixtureFiles); err != nil {
		return err
	}
	stale, err := le(staleTree, "rules", "render-check")
	if err != nil {
		return err
	}
	if stale.code != 1 {
		return uiLeRulesAnswersFailf("render-check: a stale rule answered %d: %s%s", stale.code, stale.stdout, stale.stderr)
	}
	if stale.stderr != "" {
		return uiLeRulesAnswersFailf("render-check wrote its drift page to stderr: %s", stale.stderr)
	}
	if strings.Contains(stale.stdout, staleTree+string(os.PathSeparator)) {
		return uiLeRulesAnswersFailf("render-check exposed an absolute fixture path: %s", stale.stdout)
	}
	for _, fragment := range []string{"@@ -", "MUST do the OLD thing", "MUST do the NEW thing"} {
		if !strings.Contains(stale.stdout, fragment) {
			return uiLeRulesAnswersFailf("render-check drift page does not carry %q: %s", fragment, stale.stdout)
		}
	}
	if strings.Index(stale.stdout, "MUST do the OLD thing") > strings.Index(stale.stdout, "MUST do the NEW thing") {
		return uiLeRulesAnswersFailf("render-check reversed the old and new sides of its diff: %s", stale.stdout)
	}
	if err := requireFile(filepath.Join(staleTree, "ai", "rules", "aaa.md"), oldRule, "render-check rewrote the rule it was asked to compare"); err != nil {
		return err
	}

	lint, err := jsonAnswer(le, root, "lint")
	if err != nil {
		return err
	}
	for _, key := range []string{"rules", "rule-violations", "points", "point-violations", "empty"} {
		if _, ok := lint[key]; !ok {
			return uiLeRulesAnswersFailf("the lint gate answered no %q key: %v", key, uiLeRulesAnswersSortedKeys(lint))
		}
	}
	rules, err := uiLeRulesAnswersInteger(lint["rules"])
	if err != nil || rules < 20 {
		return uiLeRulesAnswersFailf("the lint read %v rule files", lint["rules"])
	}
	points, err := uiLeRulesAnswersInteger(lint["points"])
	if err != nil || points < 1000 {
		return uiLeRulesAnswersFailf("the lint read %v points", lint["points"])
	}
	empty, ok := lint["empty"].(bool)
	if !ok || empty {
		return uiLeRulesAnswersFailf("the lint read an empty or invalid population: %v", lint["empty"])
	}
	if err := requireRenderings(le, root, "lint"); err != nil {
		return err
	}

	gateMap, err := jsonAnswer(le, root, "gate-map-report")
	if err != nil {
		return err
	}
	for _, key := range []string{"gated", "dangling", "ungated", "missing-rationale", "missing-exception", "corpus-baseline", "declared-none", "retired-ledger", "published"} {
		if _, ok := gateMap[key]; !ok {
			return uiLeRulesAnswersFailf("the gate map answered no %q key: %v", key, uiLeRulesAnswersSortedKeys(gateMap))
		}
	}
	if n, ok := listLength(gateMap["dangling"]); !ok || n != 0 {
		return uiLeRulesAnswersFailf("a binding names no point: %v", gateMap["dangling"])
	}
	baseline, baselineOK := gateMap["baseline"].(bool)
	corpusBaseline, corpusBaselineOK := gateMap["corpus-baseline"].(bool)
	if !baselineOK || !corpusBaselineOK || !baseline || !corpusBaseline {
		return uiLeRulesAnswersFailf("a ratchet did not run over this checkout, so its zero says nothing")
	}
	if n, ok := listLength(gateMap["gated"]); !ok || n < 10 {
		return uiLeRulesAnswersFailf("only %d points are gated", n)
	}
	if err := requireRenderings(le, root, "gate-map-report"); err != nil {
		return err
	}

	counted, err := le(root, "rules", "gate-map-report", "|", "count")
	if err != nil {
		return err
	}
	if counted.code != 1 {
		return uiLeRulesAnswersFailf("`| count` over several lists answered %d", counted.code)
	}
	if !strings.Contains(counted.stderr, "count needs rows") || !strings.Contains(counted.stderr, "gated") {
		return uiLeRulesAnswersFailf("the count refusal does not name the lists: %q", counted.stderr)
	}

	listing, err := le(root, "rules")
	if err != nil {
		return err
	}
	if listing.code != 0 {
		return uiLeRulesAnswersFailf("`le rules` exited %d: %s%s", listing.code, listing.stdout, listing.stderr)
	}
	for _, word := range []string{"lint", "render-check", "render-update", "points-roundtrip-check", "gate-map-report", "writes", "checks"} {
		if !strings.Contains(listing.stdout, word) {
			return uiLeRulesAnswersFailf("the rules listing does not carry %q: %s", word, listing.stdout)
		}
	}

	unknown, err := le(root, "rules", "nonesuch")
	if err != nil {
		return err
	}
	if unknown.code != 2 {
		return uiLeRulesAnswersFailf("an unknown action answered %d rather than 2", unknown.code)
	}

	refused, err := le(root, "rules", "lint", "ai/rules")
	if err != nil {
		return err
	}
	if refused.code != 2 || !strings.Contains(refused.stderr, "takes no arguments") {
		return uiLeRulesAnswersFailf("a path argument answered %d without the required refusal: %q", refused.code, refused.stderr)
	}

	help, err := le(root)
	if err != nil {
		return err
	}
	if !strings.Contains(help.stdout+help.stderr, "rules") {
		return uiLeRulesAnswersFailf("`le` does not list `rules` in its help")
	}

	// The four digest readers judge the real corpus and prove they reached all
	// of their non-empty artifacts and task descriptions.
	digestGates := []struct {
		name    string
		verb    string
		markers []string
		absent  []string
	}{
		{"rules index-check", "index-check", []string{"rules, ai/rules/INDEX.md up to date"}, nil},
		{"rules condensed-check", "condensed-check", []string{"ai/rules/TRIGGERS.md up to date", "ai/rules/CORE.md up to date"}, nil},
		{"rules payload-report", "payload-report", nil, []string{"ai/rules/CORE.md: 0 chars"}},
		{"rules router-report", "router-report", nil, []string{"corpus: 0 past task descriptions"}},
	}
	for _, gate := range digestGates {
		answer, err := le(root, "rules", gate.verb)
		if err != nil {
			return err
		}
		if answer.code != 0 {
			return uiLeRulesAnswersFailf("%s: this checkout does not pass the gate (exit %d): %s%s", gate.name, answer.code, answer.stdout, answer.stderr)
		}
		if answer.stderr != "" {
			return uiLeRulesAnswersFailf("%s wrote to stderr: %s", gate.name, answer.stderr)
		}
		for _, marker := range gate.markers {
			if !strings.Contains(answer.stdout, marker) {
				return uiLeRulesAnswersFailf("%s does not carry %q: %s", gate.name, marker, answer.stdout)
			}
		}
		for _, forbidden := range gate.absent {
			if strings.Contains(answer.stdout, forbidden) {
				return uiLeRulesAnswersFailf("%s reported an empty input: %s", gate.name, answer.stdout)
			}
		}
	}

	// Checked-in generated artifacts are the byte-for-byte contract for both
	// writers. Corrupt each destination, run only the compiled product command,
	// and require exact restoration from one immutable corpus snapshot.
	corpusTree := filepath.Join(work, "digest-corpus")
	if err := os.MkdirAll(corpusTree, 0o755); err != nil {
		return fmt.Errorf("FAIL: create digest corpus: %w", err)
	}
	for _, sub := range []string{"ai", "plan"} {
		if err := copyTree(filepath.Join(root, sub), filepath.Join(corpusTree, sub)); err != nil {
			return fmt.Errorf("FAIL: copy %s corpus: %w", sub, err)
		}
	}
	golden := make(map[string][]byte)
	for _, name := range []string{"INDEX.md", "TRIGGERS.md", "CORE.md"} {
		body, err := os.ReadFile(filepath.Join(corpusTree, "ai", "rules", name))
		if err != nil {
			return fmt.Errorf("FAIL: read checked-in %s: %w", name, err)
		}
		golden[name] = body
	}

	if err := os.WriteFile(filepath.Join(corpusTree, "ai", "rules", "INDEX.md"), []byte("stale index\n"), 0o644); err != nil {
		return fmt.Errorf("FAIL: make INDEX.md stale: %w", err)
	}
	indexUpdate, err := le(corpusTree, "rules", "index-update")
	if err != nil {
		return err
	}
	if indexUpdate.code != 0 || indexUpdate.stderr != "" {
		return uiLeRulesAnswersFailf("index-update exited %d: %s%s", indexUpdate.code, indexUpdate.stdout, indexUpdate.stderr)
	}
	if indexUpdate.stdout == "" || !strings.Contains(indexUpdate.stdout, "INDEX.md") {
		return uiLeRulesAnswersFailf("index-update did not report its artifact: %s", indexUpdate.stdout)
	}
	if strings.Contains(indexUpdate.stdout, corpusTree+string(os.PathSeparator)) {
		return uiLeRulesAnswersFailf("index-update exposed an absolute fixture path: %s", indexUpdate.stdout)
	}
	if err := requireBytes(filepath.Join(corpusTree, "ai", "rules", "INDEX.md"), golden["INDEX.md"], "index-update wrote different INDEX.md bytes"); err != nil {
		return err
	}

	for _, name := range []string{"TRIGGERS.md", "CORE.md"} {
		if err := os.WriteFile(filepath.Join(corpusTree, "ai", "rules", name), []byte("stale digest\n"), 0o644); err != nil {
			return fmt.Errorf("FAIL: make %s stale: %w", name, err)
		}
	}
	condensedUpdate, err := le(corpusTree, "rules", "condensed-update")
	if err != nil {
		return err
	}
	if condensedUpdate.code != 0 || condensedUpdate.stderr != "" {
		return uiLeRulesAnswersFailf("condensed-update exited %d: %s%s", condensedUpdate.code, condensedUpdate.stdout, condensedUpdate.stderr)
	}
	if condensedUpdate.stdout == "" || !strings.Contains(condensedUpdate.stdout, "TRIGGERS.md") || !strings.Contains(condensedUpdate.stdout, "CORE.md") {
		return uiLeRulesAnswersFailf("condensed-update did not report both artifacts: %s", condensedUpdate.stdout)
	}
	if strings.Contains(condensedUpdate.stdout, corpusTree+string(os.PathSeparator)) {
		return uiLeRulesAnswersFailf("condensed-update exposed an absolute fixture path: %s", condensedUpdate.stdout)
	}
	for _, name := range []string{"TRIGGERS.md", "CORE.md"} {
		if err := requireBytes(filepath.Join(corpusTree, "ai", "rules", name), golden[name], "condensed-update wrote different "+name+" bytes"); err != nil {
			return err
		}
	}
	if bytes.Count(golden["INDEX.md"], []byte("\n| ")) <= 20 {
		return uiLeRulesAnswersFailf("the index was written with no rows in it")
	}
	if !bytes.Contains(golden["CORE.md"], []byte("<!-- always-on: precedence rung 1/2 -->")) {
		return uiLeRulesAnswersFailf("the core names no ladder member, so the derivation read nothing")
	}

	payloadContracts := []struct {
		verb string
		keys []string
	}{
		{"index-check", []string{"file", "rules", "written", "stale", "missing", "rows"}},
		{"condensed-check", []string{"written", "empty-corpus", "artifacts"}},
		{"payload-report", []string{"parts", "chars", "tokens", "budget", "met", "headroom-percent", "missing"}},
		{"router-report", []string{"tasks", "corpus-size", "rules-total", "core", "routed", "blocking-routed", "surfaced-any", "missed-blocking", "unroutable-terms"}},
	}
	var router map[string]any
	for _, contract := range payloadContracts {
		answer, err := jsonAnswer(le, root, contract.verb)
		if err != nil {
			return err
		}
		for _, key := range contract.keys {
			if _, ok := answer[key]; !ok {
				return uiLeRulesAnswersFailf("%s answered no %q key: %v", contract.verb, key, uiLeRulesAnswersSortedKeys(answer))
			}
		}
		if err := requireRenderings(le, root, contract.verb); err != nil {
			return err
		}
		if contract.verb == "router-report" {
			router = answer
		}
	}

	tasks, ok := router["tasks"].([]any)
	if !ok {
		return uiLeRulesAnswersFailf("the router tasks payload is not a row set: %T", router["tasks"])
	}
	corpusSize, err := uiLeRulesAnswersInteger(router["corpus-size"])
	if err != nil || corpusSize != len(tasks) {
		return uiLeRulesAnswersFailf("the router names %d tasks for a corpus of %v", len(tasks), router["corpus-size"])
	}
	if len(tasks) == 0 {
		return uiLeRulesAnswersFailf("the router returned no per-task rows")
	}
	first, ok := tasks[0].(map[string]any)
	if !ok {
		return uiLeRulesAnswersFailf("the first router task is not an object: %T", tasks[0])
	}
	if _, ok := first["surfaced"]; !ok {
		return uiLeRulesAnswersFailf("the first per-task row carries no surfaced set: %v", first)
	}

	listing, err = le(root, "rules")
	if err != nil {
		return err
	}
	for _, word := range []string{"index-check", "index-update", "condensed-check", "condensed-update", "payload-report", "router-report"} {
		if !strings.Contains(listing.stdout, word) {
			return uiLeRulesAnswersFailf("the rules listing does not carry %q: %s", word, listing.stdout)
		}
	}

	refused, err = le(root, "rules", "router-report", "plan")
	if err != nil {
		return err
	}
	if refused.code != 2 || !strings.Contains(refused.stderr, "takes no arguments") {
		return uiLeRulesAnswersFailf("a router path argument answered %d without the required refusal: %q", refused.code, refused.stderr)
	}

	_, err = fmt.Fprintln(os.Stdout, "OK")
	return err
}

func uiLeRulesAnswersExecute(ctx context.Context, dir string, env []string, program string, args ...string) (uiLeRulesAnswersCommandResult, error) {
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := uiLeRulesAnswersCommandResult{code: 0, stdout: stdout.String(), stderr: stderr.String()}
	if err == nil {
		return result, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.code = exitErr.ExitCode()
		return result, nil
	}
	return uiLeRulesAnswersCommandResult{}, fmt.Errorf("FAIL: execute %s: %w", program, err)
}

func jsonAnswer(le func(string, ...string) (uiLeRulesAnswersCommandResult, error), tree, verb string) (map[string]any, error) {
	answer, err := le(tree, "rules", verb, "|", "json")
	if err != nil {
		return nil, err
	}
	if answer.code != 0 {
		return nil, uiLeRulesAnswersFailf("`le rules %s | json` exited %d: %s%s", verb, answer.code, answer.stdout, answer.stderr)
	}
	if answer.stderr != "" {
		return nil, uiLeRulesAnswersFailf("`le rules %s | json` wrote to stderr: %s", verb, answer.stderr)
	}
	decoder := json.NewDecoder(strings.NewReader(answer.stdout))
	decoder.UseNumber()
	var decoded map[string]any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, uiLeRulesAnswersFailf("`le rules %s | json` did not answer JSON: %v; %.400s", verb, err, answer.stdout)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, uiLeRulesAnswersFailf("`le rules %s | json` answered trailing data: %.400s", verb, answer.stdout)
	}
	return decoded, nil
}

func requireRenderings(le func(string, ...string) (uiLeRulesAnswersCommandResult, error), tree, verb string) error {
	for _, operator := range []string{"yaml", "table"} {
		answer, err := le(tree, "rules", verb, "|", operator)
		if err != nil {
			return err
		}
		if answer.code != 0 {
			return uiLeRulesAnswersFailf("`le rules %s | %s` was refused with exit %d: %s%s", verb, operator, answer.code, answer.stdout, answer.stderr)
		}
	}
	return nil
}

func uiLeRulesAnswersInteger(value any) (int, error) {
	switch v := value.(type) {
	case json.Number:
		n, err := strconv.ParseInt(v.String(), 10, 64)
		return int(n), err
	case float64:
		return int(v), nil
	case int:
		return v, nil
	default:
		return 0, fmt.Errorf("not an integer: %T", value)
	}
}

func listLength(value any) (int, bool) {
	rows, ok := value.([]any)
	return len(rows), ok
}

func uiLeRulesAnswersSortedKeys(value map[string]any) []string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

func uiLeRulesAnswersSetEnv(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if !strings.HasPrefix(entry, prefix) {
			out = append(out, entry)
		}
	}
	return append(out, prefix+value)
}

func writeFixture(root string, files map[string]string) error {
	for relative, body := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("FAIL: create %s: %w", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return fmt.Errorf("FAIL: write %s: %w", path, err)
		}
	}
	return nil
}

func requireFile(path, expected, message string) error {
	return requireBytes(path, []byte(expected), message)
}

func requireBytes(path string, expected []byte, message string) error {
	actual, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("FAIL: %s: %w", message, err)
	}
	if !bytes.Equal(actual, expected) {
		return uiLeRulesAnswersFailf("%s\nexpected:\n%s\nactual:\n%s", message, expected, actual)
	}
	return nil
}

func copyTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if entry.Type()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("unsupported corpus entry %s (%s)", path, entry.Type())
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		closeOutErr := output.Close()
		closeInErr := input.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeOutErr != nil {
			return closeOutErr
		}
		return closeInErr
	})
}

func uiLeRulesAnswersFailf(format string, args ...any) error {
	return fmt.Errorf("FAIL: "+format, args...)
}
