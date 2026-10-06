// Design: docs/architecture/testing/runner-architecture.md -- owned UI answer fixtures.

package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func init() {
	Register("ui/le-spec-status-answers", uiDriver(leSpecStatusAnswers))
}

const (
	leSpecBucketAfter     = "after"
	leSpecBucketImmediate = "immediate"
	leSpecCategoryBacklog = "backlog"
	leSpecCategoryIdea    = "idea"
	leSpecCategoryOther   = "other" // The spec category, not an appliance name.
	leSpecFieldBucket     = "bucket"
)

type uiLeSpecStatusAnswersCommandAnswer struct {
	stdout []byte
	stderr []byte
	code   int
}

func leSpecStatusAnswers(ctx context.Context) error {
	here, _, err := temporaryLEFixtureWorkspace("le-spec-status-answers-")
	if err != nil {
		return uiLeSpecStatusAnswersFailf("create fixture working directory: %v", err)
	}
	defer os.RemoveAll(here) //nolint:errcheck // fixture cleanup

	binary, err := nativeLEBinary()
	if err != nil {
		return uiLeSpecStatusAnswersFailf("%v", err)
	}

	// Own both the specs and their Git history. Repeatedly scanning the shared
	// checkout multiplied one git-log process per spec by every rendering and
	// settling attempt, and concurrent edits could still invalidate the result.
	tree := filepath.Join(here, "tree")
	expected, childEnv, err := leSpecStatusTree(ctx, tree)
	if err != nil {
		return err
	}
	page1, json1, json2, err := specStatusOverAStillTree(ctx, tree, here, binary, childEnv)
	if err != nil {
		return err
	}

	if page1.code != 0 {
		return uiLeSpecStatusAnswersFailf("le spec status exited %d; stderr: %q", page1.code, page1.stderr)
	}
	if len(page1.stderr) != 0 {
		return uiLeSpecStatusAnswersFailf("le spec status wrote warnings: %q", page1.stderr)
	}
	trimmedPage := bytes.TrimSpace(page1.stdout)
	if len(trimmedPage) == 0 {
		return uiLeSpecStatusAnswersFailf("le spec status returned an empty page")
	}
	if trimmedPage[0] == '[' || trimmedPage[0] == '{' {
		return uiLeSpecStatusAnswersFailf("le spec status returned structured records instead of the default page")
	}

	if json1.code != 0 {
		return uiLeSpecStatusAnswersFailf("le spec status | json exited %d; stderr: %q", json1.code, json1.stderr)
	}
	if len(json1.stderr) != 0 {
		return uiLeSpecStatusAnswersFailf("le spec status | json wrote warnings: %q", json1.stderr)
	}
	if json2.code != json1.code || !bytes.Equal(json2.stderr, json1.stderr) || !bytes.Equal(json2.stdout, json1.stdout) {
		return uiLeSpecStatusAnswersFailf("two structured inventory answers disagree")
	}

	records1, err := decodeRecords(json1.stdout)
	if err != nil {
		return uiLeSpecStatusAnswersFailf("the structured answer did not decode: %v", err)
	}
	records2, err := decodeRecords(json2.stdout)
	if err != nil {
		return uiLeSpecStatusAnswersFailf("the second structured answer did not decode: %v", err)
	}
	if !reflect.DeepEqual(records1, records2) {
		return uiLeSpecStatusAnswersFailf("two decoded inventory answers disagree")
	}
	if len(records1) != len(expected) {
		return uiLeSpecStatusAnswersFailf("inventory answered %d records for %d authored specs", len(records1), len(expected))
	}
	if err := checkRecordContract(records1, page1.stdout); err != nil {
		return err
	}
	if err := leSpecStatusExpectedRecords(records1, expected); err != nil {
		return err
	}

	counted, err := uiLeSpecStatusAnswersRunCommand(ctx, here, childEnv, binary, "spec", "status", "|", "count")
	if err != nil {
		return err
	}
	if counted.code != 0 {
		return uiLeSpecStatusAnswersFailf("le spec status | count exited %d; stderr: %q", counted.code, counted.stderr)
	}
	if len(counted.stderr) != 0 {
		return uiLeSpecStatusAnswersFailf("le spec status | count wrote warnings: %q", counted.stderr)
	}
	wantCount := fmt.Sprintf("%d", len(records1))
	if strings.TrimSpace(string(counted.stdout)) != wantCount {
		return uiLeSpecStatusAnswersFailf("le spec status | count answered %q, want %s", counted.stdout, wantCount)
	}

	refused, err := uiLeSpecStatusAnswersRunCommand(ctx, here, childEnv, binary, "spec", "status", "--json")
	if err != nil {
		return err
	}
	if refused.code != 2 {
		return uiLeSpecStatusAnswersFailf("le spec status --json exited %d, want 2", refused.code)
	}
	if len(refused.stdout) != 0 {
		return uiLeSpecStatusAnswersFailf("le spec status --json wrote unexpected stdout: %q", refused.stdout)
	}
	if !bytes.Contains(refused.stderr, []byte("usage: le spec status")) ||
		!bytes.Contains(refused.stderr, []byte(`got "--json"`)) {
		return uiLeSpecStatusAnswersFailf("the refusal does not identify the invalid argument: %q", refused.stderr)
	}

	fmt.Println("OK")
	return nil
}

func checkRecordContract(records []map[string]json.RawMessage, page []byte) error {
	required := []string{fieldName, fieldStatus, leSpecFieldBucket, "category", fieldUpdated, "git-modified", statusStale}
	lines := strings.Split(string(page), "\n")
	lineAt := 0
	section := ""

	for i, record := range records {
		for _, key := range required {
			if _, ok := record[key]; !ok {
				return uiLeSpecStatusAnswersFailf("record %d carries no %q field", i, key)
			}
		}

		name, err := stringField(record, "name")
		if err != nil || name == "" {
			return uiLeSpecStatusAnswersFailf("record %d has an invalid name: %v", i, err)
		}

		status, err := stringField(record, "status")
		if err != nil {
			return uiLeSpecStatusAnswersFailf("record %q has an invalid status: %v", name, err)
		}
		bucket, err := stringField(record, leSpecFieldBucket)
		if err != nil {
			return uiLeSpecStatusAnswersFailf("record %q has an invalid bucket: %v", name, err)
		}
		category, err := stringField(record, "category")
		if err != nil {
			return uiLeSpecStatusAnswersFailf("record %q has an invalid category: %v", name, err)
		}
		updated, err := stringField(record, "updated")
		if err != nil {
			return uiLeSpecStatusAnswersFailf("record %q has an invalid updated value: %v", name, err)
		}
		modified, err := stringField(record, "git-modified")
		if err != nil {
			return uiLeSpecStatusAnswersFailf("record %q has an invalid git-modified value: %v", name, err)
		}
		if modified != "unknown" {
			if _, err := time.Parse("2006-01-02", modified); err != nil {
				return uiLeSpecStatusAnswersFailf("record %q has git-modified %q, want an ISO git date or unknown", name, modified)
			}
		}
		var stale bool
		if err := json.Unmarshal(record["stale"], &stale); err != nil {
			return uiLeSpecStatusAnswersFailf("record %q has a non-boolean stale value: %v", name, err)
		}

		row := -1
		for j := lineAt; j < len(lines); j++ {
			if strings.HasPrefix(lines[j], "── ") {
				section = lines[j]
			}
			if strings.Contains(lines[j], name) {
				row = j
				break
			}
		}
		if row < 0 {
			return uiLeSpecStatusAnswersFailf("the page has no row for record %q in record order", name)
		}
		lineAt = row + 1
		pageStale := strings.HasPrefix(strings.TrimSpace(lines[row]), "STALE ")
		if pageStale != stale {
			return uiLeSpecStatusAnswersFailf("the page row for %q has stale=%v, record has %v: %q", name, pageStale, stale, lines[row])
		}
		// The page's SECTIONS are the status-derived CATEGORY, never the release
		// BUCKET. The two were one field until 6fb9cd8814 (2026-09-05) split
		// them: `bucket` is now the directory the spec sits in (after,
		// immediate, pre-release, internal/le/spec/path) and `category` is
		// the backlog / idea / other split the sections print
		// (specstatus.Category). Reading the section off `bucket` asks the
		// wrong record for the answer.
		wantSection := map[string]string{
			leSpecCategoryBacklog: "Committed backlog",
			leSpecCategoryIdea:    "Idea capture",
			leSpecCategoryOther:   "Other",
		}[category]
		if wantSection == "" || !strings.Contains(section, wantSection) {
			return uiLeSpecStatusAnswersFailf("the page files %q under %q, want category %q", name, section, category)
		}
		if !slices.Contains([]string{leSpecBucketAfter, leSpecBucketImmediate, "pre-release"}, bucket) {
			return uiLeSpecStatusAnswersFailf("record %q carries bucket %q, which names no release bucket", name, bucket)
		}
		for key, value := range map[string]string{
			fieldName:         name,
			fieldStatus:       status,
			leSpecFieldBucket: bucket,
			fieldUpdated:      updated,
		} {
			if value != "" && !strings.Contains(lines[row], value) {
				return uiLeSpecStatusAnswersFailf("the page row for %q does not render %s %q: %q", name, key, value, lines[row])
			}
		}
		for _, key := range []string{"phase", "depends"} {
			raw, ok := record[key]
			if !ok {
				continue
			}
			values, err := textualValues(raw)
			if err != nil {
				return uiLeSpecStatusAnswersFailf("record %q has an invalid %s value: %v", name, key, err)
			}
			for _, value := range values {
				if value != "" && !strings.Contains(lines[row], value) {
					return uiLeSpecStatusAnswersFailf("the page row for %q does not render %s value %q: %q", name, key, value, lines[row])
				}
			}
		}
	}
	return nil
}

func stringField(record map[string]json.RawMessage, key string) (string, error) {
	var value string
	if err := json.Unmarshal(record[key], &value); err != nil {
		return "", err
	}
	return value, nil
}

func textualValues(raw json.RawMessage) ([]string, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	var out []string
	var visit func(any)
	visit = func(v any) {
		switch v := v.(type) {
		case string:
			out = append(out, v)
		case []any:
			for _, item := range v {
				visit(item)
			}
		}
	}
	visit(value)
	return out, nil
}

func decodeRecords(answer []byte) ([]map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(answer))
	var records []map[string]json.RawMessage
	if err := decoder.Decode(&records); err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("extra value after the record array")
		}
		return nil, err
	}
	return records, nil
}

func uiLeSpecStatusAnswersRunCommand(ctx context.Context, dir string, env []string, name string, args ...string) (uiLeSpecStatusAnswersCommandAnswer, error) {
	fmt.Printf("spec-status fixture: %s %s\n", filepath.Base(name), strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Dir = dir
	cmd.Env = env
	plugin.KillGroupOnCancel(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	answer := uiLeSpecStatusAnswersCommandAnswer{stdout: stdout.Bytes(), stderr: stderr.Bytes(), code: 0}
	if ctx.Err() != nil {
		return answer, uiLeSpecStatusAnswersFailf("execute %s %v: %v\nstdout:\n%s\nstderr:\n%s",
			name, args, ctx.Err(), answer.stdout, answer.stderr)
	}
	if err == nil {
		return answer, nil
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		answer.code = exit.ExitCode()
		return answer, nil
	}
	return uiLeSpecStatusAnswersCommandAnswer{}, uiLeSpecStatusAnswersFailf("execute %s: %v", name, err)
}

func uiLeSpecStatusAnswersFailf(format string, args ...any) error {
	return fmt.Errorf("FAIL: "+format, args...)
}

// specStatusOverAStillTree checks deterministic bytes over a fixture-owned tree.
// Running from both directories proves root selection does not depend on cwd.
func specStatusOverAStillTree(ctx context.Context, root, here, binary string, childEnv []string) (
	page, structured, structuredAgain uiLeSpecStatusAnswersCommandAnswer, err error,
) {
	page, err = uiLeSpecStatusAnswersRunCommand(ctx, here, childEnv, binary, "spec", "status")
	if err != nil {
		return page, structured, structuredAgain, err
	}
	structured, err = uiLeSpecStatusAnswersRunCommand(ctx, root, childEnv, binary, "spec", "status", "|", "json")
	if err != nil {
		return page, structured, structuredAgain, err
	}
	pageAgain, runErr := uiLeSpecStatusAnswersRunCommand(ctx, here, childEnv, binary, "spec", "status")
	if runErr != nil {
		return page, structured, structuredAgain, runErr
	}
	if !reflect.DeepEqual(page, pageAgain) {
		err = uiLeSpecStatusAnswersFailf("two page answers over the owned tree disagree:\nfirst: %#v\nsecond: %#v", page, pageAgain)
		return page, structured, structuredAgain, err
	}
	structuredAgain, err = uiLeSpecStatusAnswersRunCommand(ctx, root, childEnv, binary, "spec", "status", "|", "json")
	return page, structured, structuredAgain, err
}

type leSpecStatusCase struct {
	name, dir, bucket, status, category, updated, modified string
	stale                                                  bool
}

// leSpecStatusTree uses real Git, but never inherits the caller's repository,
// history, hooks, signing setup, identity, or dates.
func leSpecStatusTree(ctx context.Context, root string) ([]leSpecStatusCase, []string, error) {
	const committed = "2000-01-02"
	expected := []leSpecStatusCase{
		{"fixture-ready", "plan/immediate", leSpecBucketImmediate, "ready", leSpecCategoryBacklog, committed, committed, false},
		{"fixture-untracked", "plan/immediate", leSpecBucketImmediate, "design", leSpecCategoryBacklog, "2001-02-03", "unknown", false},
		{"fixture-fresh", dirPlan, leSpecBucketAfter, "skeleton", leSpecCategoryIdea, time.Now().UTC().Format("2006-01-02"), committed, false},
		{"fixture-stale", dirPlan, leSpecBucketAfter, "skeleton", leSpecCategoryIdea, "2000-01-01", committed, true},
		{"fixture-blocked", "plan/pre-release", "pre-release", "blocked", leSpecCategoryOther, "2000-01-01", committed, false},
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, nil, err
	}
	baseEnv := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") {
			baseEnv = append(baseEnv, entry)
		}
	}
	childEnv := childEnvironment(baseEnv, map[string]string{
		envRepoRoot:          root,
		envGitConfigGlobal:   os.DevNull,
		envGitConfigSystem:   "1",
		envGitAuthorName:     gitFixtureName,
		envGitAuthorEmail:    gitFixtureEmail,
		envGitCommitName:     gitFixtureName,
		envGitCommitEmail:    gitFixtureEmail,
		"GIT_AUTHOR_DATE":    committed + "T12:00:00Z",
		"GIT_COMMITTER_DATE": committed + "T12:00:00Z",
	})
	for _, spec := range expected {
		dir := filepath.Join(root, spec.dir)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, nil, err
		}
		var body textbuf.Buffer
		body.Str("# Spec: ").Str(spec.name).Str("\n\n| Field | Value |\n|---|---|\n| Status | ").
			Str(spec.status).Str(" |\n| Phase | fixture-phase |\n| Depends | fixture-dependency |\n")
		// The ready spec proves Updated falls back to the actual Git date.
		if spec.name != "fixture-ready" {
			body.Str("| Updated | ").Str(spec.updated).Str(" |\n")
		}
		if err := os.WriteFile(filepath.Join(dir, "spec-"+spec.name+".md"), []byte(body.String()), 0o600); err != nil {
			return nil, nil, err
		}
	}
	commands := [][]string{
		{argInit, argQuiet, "--template=", "--initial-branch=fixture"},
		{argAdd, "--", dirPlan},
		{"rm", "--cached", "--", "plan/immediate/spec-fixture-untracked.md"},
		{"-c", "core.hooksPath=" + os.DevNull, "-c", gitCommitNoSign, argCommit, argQuiet, "-m", "Fixture specs"},
	}
	for _, args := range commands {
		answer, err := uiLeSpecStatusAnswersRunCommand(ctx, root, childEnv, "git", args...)
		if err != nil {
			return nil, nil, err
		}
		if answer.code != 0 {
			return nil, nil, uiLeSpecStatusAnswersFailf("git %v exited %d\nstdout:\n%s\nstderr:\n%s",
				args, answer.code, answer.stdout, answer.stderr)
		}
	}
	return expected, childEnv, nil
}

// leSpecStatusExpectedRecords is independent of production inventory types and
// rejects renderings that agree with each other but misclassify the same spec.
func leSpecStatusExpectedRecords(records []map[string]json.RawMessage, expected []leSpecStatusCase) error {
	for i, spec := range expected {
		for key, want := range map[string]string{
			"name": spec.name, "title": spec.name, "path": spec.dir + "/spec-" + spec.name + ".md",
			"status": spec.status, leSpecFieldBucket: spec.bucket, "category": spec.category,
			"updated": spec.updated, "git-modified": spec.modified,
			"phase": "fixture-phase", "depends": "fixture-dependency",
		} {
			got, err := stringField(records[i], key)
			if err != nil {
				return uiLeSpecStatusAnswersFailf("record %d field %q: %v", i, key, err)
			}
			if got != want {
				return uiLeSpecStatusAnswersFailf("record %d field %q = %q, want %q", i, key, got, want)
			}
		}
		var stale bool
		if err := json.Unmarshal(records[i]["stale"], &stale); err != nil {
			return uiLeSpecStatusAnswersFailf("record %d stale: %v", i, err)
		}
		if stale != spec.stale {
			return uiLeSpecStatusAnswersFailf("record %q stale = %v, want %v", spec.name, stale, spec.stale)
		}
	}
	return nil
}
