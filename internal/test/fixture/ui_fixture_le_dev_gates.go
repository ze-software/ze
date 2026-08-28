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
	"strconv"
	"strings"
	"unicode/utf8"
)

func init() {
	Register("ui/le-dev-gates-answers", uiDriver(runLEDevAnswers))
}

type uiLeDevGatesAnswersCommandAnswer struct {
	code   int
	stdout []byte
	stderr []byte
}

func runLEDevAnswers(ctx context.Context) error {
	root, ok := os.LookupEnv("ZE_REPO_ROOT")
	if !ok || root == "" {
		return uiLeDevGatesAnswersFailf("ZE_REPO_ROOT is not set")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return uiLeDevGatesAnswersFailf("resolving ZE_REPO_ROOT: %v", err)
	}
	here, _, err := temporaryLEFixtureWorkspace("le-dev-answers-")
	if err != nil {
		return uiLeDevGatesAnswersFailf("creating fixture directory: %v", err)
	}
	defer os.RemoveAll(here)

	binary, err := uiLEBinary(root)
	if err != nil {
		return uiLeDevGatesAnswersFailf("%v", err)
	}

	le := func(args ...string) (uiLeDevGatesAnswersCommandAnswer, error) {
		return uiLeDevGatesAnswersExecute(ctx, here, binary, args...)
	}

	// These are the direct page contracts formerly established indirectly by
	// comparing two implementations. Each gate must pass, emit one UTF-8 page
	// ending in a newline, and keep diagnostics empty.
	pages := []struct {
		name string
		args []string
	}{
		{name: "gokrazy-gosum", args: []string{"gokrazy-gosum"}},
		{name: "arch-map check", args: []string{"arch-map", "check"}},
		{name: "protocol-skeleton report", args: []string{"protocol-skeleton", "report"}},
		{name: "protocol-skeleton selftest", args: []string{"protocol-skeleton", "selftest"}},
	}
	for _, page := range pages {
		answer, err := le(page.args...)
		if err != nil {
			return err
		}
		if answer.code != 0 {
			return uiLeDevGatesAnswersFailf("%s: this checkout does not pass the gate: %s%s", page.name, answer.stdout, answer.stderr)
		}
		if len(answer.stdout) == 0 || answer.stdout[len(answer.stdout)-1] != '\n' || !utf8.Valid(answer.stdout) {
			return uiLeDevGatesAnswersFailf("%s: the command did not emit its complete UTF-8 page: %q", page.name, answer.stdout)
		}
		if len(answer.stderr) != 0 {
			return uiLeDevGatesAnswersFailf("%s: the command wrote to stderr: %s", page.name, answer.stderr)
		}
		if page.name == "protocol-skeleton report" {
			if !bytes.Contains(answer.stdout, []byte("| json for detail")) {
				return uiLeDevGatesAnswersFailf("%s: the page does not advertise its data rendering:\n%s", page.name, answer.stdout)
			}
			if bytes.Contains(answer.stdout, []byte("--verbose for detail")) {
				return uiLeDevGatesAnswersFailf("%s: the page advertises an obsolete detail form:\n%s", page.name, answer.stdout)
			}
		}
	}

	// The answer is intentionally checked by shape: concurrent users may alter
	// the shared tree between observations, but they cannot change this prefix
	// contract.
	working, err := le("working-tree")
	if err != nil {
		return err
	}
	if working.code != 0 {
		return uiLeDevGatesAnswersFailf("working-tree: the command exited %d: %s%s", working.code, working.stdout, working.stderr)
	}
	if !utf8.Valid(working.stdout) || !bytes.HasPrefix(working.stdout, []byte("working tree: ")) {
		return uiLeDevGatesAnswersFailf("working-tree printed something else: %s", working.stdout)
	}

	// One payload, every rendering.
	report, err := le("gokrazy-gosum", "|", "json")
	if err != nil {
		return err
	}
	var gosum map[string]any
	if err := json.Unmarshal(report.stdout, &gosum); err != nil {
		return uiLeDevGatesAnswersFailf("`le gokrazy-gosum | json` did not answer JSON: %v\n%s", err, uiLeDevGatesAnswersPrefix(report.stdout, 400))
	}
	for _, key := range []string{"files", "shared", "conflicts"} {
		if _, found := gosum[key]; !found {
			return uiLeDevGatesAnswersFailf("the gate answered no %q key: %v", key, uiLeDevGatesAnswersSortedKeys(gosum))
		}
	}
	files, ok := gosum["files"].(float64)
	if !ok || files < 1 {
		return uiLeDevGatesAnswersFailf("the gate read %v builddir go.sum files", gosum["files"])
	}
	if !jsonEmpty(gosum["conflicts"]) {
		return uiLeDevGatesAnswersFailf("the gate reported conflicts: %v", gosum["conflicts"])
	}
	for _, rendering := range []string{"yaml", "table"} {
		answer, err := le("gokrazy-gosum", "|", rendering)
		if err != nil {
			return err
		}
		if answer.code != 0 {
			return uiLeDevGatesAnswersFailf("`le gokrazy-gosum | %s` was refused", rendering)
		}
	}

	// Generated blocks carry their own counts and check mode must not write.
	answer, err := le("arch-map", "check", "|", "json")
	if err != nil {
		return err
	}
	var blocks struct {
		Blocks []struct {
			Directories int `json:"directories"`
		} `json:"blocks"`
		Stale   bool `json:"stale"`
		Written bool `json:"written"`
	}
	if err := json.Unmarshal(answer.stdout, &blocks); err != nil {
		return uiLeDevGatesAnswersFailf("`le arch-map check | json` did not answer JSON: %v\n%s", err, uiLeDevGatesAnswersPrefix(answer.stdout, 400))
	}
	if len(blocks.Blocks) != 3 {
		return uiLeDevGatesAnswersFailf("arch-map answered %d blocks, want three", len(blocks.Blocks))
	}
	for _, block := range blocks.Blocks {
		if block.Directories <= 0 {
			return uiLeDevGatesAnswersFailf("a generated block counted nothing: %+v", blocks.Blocks)
		}
	}
	if blocks.Stale {
		return uiLeDevGatesAnswersFailf("the architecture lists are stale in this checkout")
	}
	if blocks.Written {
		return uiLeDevGatesAnswersFailf("check mode reported a write")
	}

	// The selftest exposes a row per case, and count acts on those rows.
	answer, err = le("protocol-skeleton", "selftest", "|", "json")
	if err != nil {
		return err
	}
	var cases []struct {
		Passed bool `json:"passed"`
	}
	if err := json.Unmarshal(answer.stdout, &cases); err != nil {
		return uiLeDevGatesAnswersFailf("the selftest did not answer a row list: %v\n%s", err, uiLeDevGatesAnswersPrefix(answer.stdout, 400))
	}
	if len(cases) < 10 {
		return uiLeDevGatesAnswersFailf("the selftest answered %v, want a row per case", cases)
	}
	for _, row := range cases {
		if !row.Passed {
			return uiLeDevGatesAnswersFailf("a selftest case failed: %v", cases)
		}
	}
	counted, err := le("protocol-skeleton", "selftest", "|", "count")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(counted.stdout)) != strconv.Itoa(len(cases)) {
		return uiLeDevGatesAnswersFailf("`| count` answered %q for %d cases", counted.stdout, len(cases))
	}

	// Areas list their actions and whether those actions write.
	listing, err := le("arch-map")
	if err != nil {
		return err
	}
	if listing.code != 0 {
		return uiLeDevGatesAnswersFailf("`le arch-map` exited %d", listing.code)
	}
	for _, word := range []string{"check", "update", "writes", "checks"} {
		if !bytes.Contains(listing.stdout, []byte(word)) {
			return uiLeDevGatesAnswersFailf("the listing does not carry %q:\n%s", word, listing.stdout)
		}
	}

	unknown, err := le("arch-map", "nonesuch")
	if err != nil {
		return err
	}
	if unknown.code != 2 {
		return uiLeDevGatesAnswersFailf("an unknown action answered %d, want 2", unknown.code)
	}

	refused, err := le("gokrazy-gosum", "gokrazy/")
	if err != nil {
		return err
	}
	if refused.code != 1 {
		return uiLeDevGatesAnswersFailf("a path argument answered %d", refused.code)
	}
	if !bytes.Contains(refused.stderr, []byte("takes no arguments")) {
		return uiLeDevGatesAnswersFailf("the refusal is silent: %q", refused.stderr)
	}
	bare, err := le("working-tree", "3")
	if err != nil {
		return err
	}
	if bare.code != 1 {
		return uiLeDevGatesAnswersFailf("a bare number was accepted as a ceiling")
	}
	nonnumeric, err := le("working-tree", "max-areas", "many")
	if err != nil {
		return err
	}
	if nonnumeric.code != 1 {
		return uiLeDevGatesAnswersFailf("a ceiling that is not a number was accepted")
	}
	high, err := le("working-tree", "max-areas", "99")
	if err != nil {
		return err
	}
	if high.code != 0 {
		return uiLeDevGatesAnswersFailf("a ceiling no tree can exceed still failed")
	}

	help, err := le()
	if err != nil {
		return err
	}
	helpText := append(append([]byte(nil), help.stdout...), help.stderr...)
	for _, command := range []string{"gokrazy-gosum", "working-tree", "arch-map", "protocol-skeleton"} {
		if !bytes.Contains(helpText, []byte(command)) {
			return uiLeDevGatesAnswersFailf("`le` does not list %q in its help", command)
		}
	}

	fmt.Println("OK")
	return nil
}

func uiLeDevGatesAnswersExecute(ctx context.Context, dir, name string, args ...string) (uiLeDevGatesAnswersCommandAnswer, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	answer := uiLeDevGatesAnswersCommandAnswer{stdout: stdout.Bytes(), stderr: stderr.Bytes()}
	if err == nil {
		return answer, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return uiLeDevGatesAnswersCommandAnswer{}, ctxErr
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		answer.code = exitErr.ExitCode()
		return answer, nil
	}
	return uiLeDevGatesAnswersCommandAnswer{}, uiLeDevGatesAnswersFailf("running %s: %v", name, err)
}

func uiLeDevGatesAnswersPrefix(value []byte, limit int) []byte {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

func jsonEmpty(value any) bool {
	switch value := value.(type) {
	case nil:
		return true
	case bool:
		return !value
	case string:
		return value == ""
	case []any:
		return len(value) == 0
	case map[string]any:
		return len(value) == 0
	default:
		return false
	}
}

func uiLeDevGatesAnswersSortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

func uiLeDevGatesAnswersFailf(format string, args ...any) error {
	return fmt.Errorf("FAIL: "+format, args...)
}
