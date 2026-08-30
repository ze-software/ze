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
	"unicode/utf8"
)

func init() {
	Register("ui/le-weekly-answers", uiDriver(leWeeklyAnswers))
}

type weeklyCommandResult struct {
	stdout   string
	stderr   string
	exitCode int
}

func leWeeklyAnswers(ctx context.Context) error {
	checkout := os.Getenv("ZE_REPO_ROOT")
	if checkout == "" {
		return fmt.Errorf("FAIL: ZE_REPO_ROOT is not set")
	}

	here, err := os.MkdirTemp("", "le-weekly-answers-")
	if err != nil {
		return fmt.Errorf("FAIL: creating fixture directory: %w", err)
	}
	defer os.RemoveAll(here) //nolint:errcheck // fixture cleanup

	binary, err := uiLEBinary(checkout)
	if err != nil {
		return fmt.Errorf("FAIL: %w", err)
	}

	fixture := filepath.Join(here, "fixture")
	posts := filepath.Join(fixture, "website", "changes", "posts")
	archive := filepath.Join(fixture, "scripts", "zeledon", "weekly")
	if err := os.MkdirAll(posts, 0o750); err != nil {
		return fmt.Errorf("FAIL: creating posts fixture: %w", err)
	}
	if err := os.MkdirAll(archive, 0o750); err != nil {
		return fmt.Errorf("FAIL: creating archive fixture: %w", err)
	}

	weeks := [][2]string{
		{"1999-01-04", "1999-01-10"},
		{"1999-01-11", "1999-01-17"},
	}
	for _, week := range weeks {
		body := fmt.Sprintf("---\ncovers: %s .. %s\n---\n\n**📅 Ze Weekly Update**\n\nWeek of %s.\n", week[0], week[1], week[0])
		name := filepath.Join(posts, week[0]+".md")
		if err := os.WriteFile(name, []byte(body), 0o600); err != nil {
			return fmt.Errorf("FAIL: writing %s: %w", name, err)
		}
	}

	env := replaceWeeklyEnv(os.Environ(), "ZE_REPO_ROOT", fixture)
	run := func(args ...string) weeklyCommandResult {
		productArgs := append([]string{"weekly"}, args...)
		return runWeeklyCommand(ctx, here, env, binary, productArgs...)
	}

	plan := run()
	if plan.exitCode != 0 {
		return fmt.Errorf("FAIL: `le weekly` exited %d: %s", plan.exitCode, plan.stderr)
	}
	for _, week := range weeks {
		if !strings.Contains(plan.stdout, "Week of "+week[0]) {
			return fmt.Errorf("FAIL: the plan does not show the message for %s:\n%s", week[0], plan.stdout)
		}
	}
	if !strings.Contains(plan.stdout, "nothing sent") {
		return fmt.Errorf("FAIL: the plan does not say that it published nothing:\n%s", plan.stdout)
	}

	answer := run("|", "json")
	var report map[string]any
	if err := json.Unmarshal([]byte(answer.stdout), &report); err != nil {
		preview := answer.stdout
		if len(preview) > 400 {
			preview = preview[:400]
		}
		return fmt.Errorf("FAIL: `le weekly | json` did not answer JSON: %w\n%s", err, preview)
	}

	action, ok := report["action"].(string)
	if !ok || action != "planned" {
		return fmt.Errorf("FAIL: action = %q, want 'planned'", action)
	}
	channel, ok := report["channel"].(string)
	if !ok || channel != "ze-news" {
		return fmt.Errorf("FAIL: channel = %q", channel)
	}
	reportPosts, ok := report["posts"].([]any)
	if !ok {
		return fmt.Errorf("FAIL: posts = %T, want an array", report["posts"])
	}
	if len(reportPosts) != len(weeks) {
		return fmt.Errorf("FAIL: the answer names %d post(s), want %d", len(reportPosts), len(weeks))
	}

	first, ok := reportPosts[0].(map[string]any)
	if !ok {
		return fmt.Errorf("FAIL: first post row = %T, want an object", reportPosts[0])
	}
	for _, key := range []string{"source", "date", "covers", fieldStatus, "date-stamped", "messages"} {
		if _, found := first[key]; !found {
			keys := make([]string, 0, len(first))
			for present := range first {
				keys = append(keys, present)
			}
			sort.Strings(keys)
			return fmt.Errorf("FAIL: a post row carries no %q: %v", key, keys)
		}
	}
	date, ok := first["date"].(string)
	if !ok || date != weeks[0][0] {
		return fmt.Errorf("FAIL: the sweep ran out of order: %v", first["date"])
	}
	status, ok := first["status"].(string)
	if !ok || status != "planned" {
		return fmt.Errorf("FAIL: status = %q", status)
	}
	if _, found := first["archive"]; found {
		return fmt.Errorf("FAIL: a post nothing published carries an archive key")
	}
	messages, ok := first["messages"].([]any)
	if !ok || len(messages) == 0 {
		return fmt.Errorf("FAIL: messages = %v, want at least one message", first["messages"])
	}
	message, ok := messages[0].(map[string]any)
	if !ok {
		return fmt.Errorf("FAIL: first message = %T, want an object", messages[0])
	}
	text, textOK := message["text"].(string)
	chars, charsOK := message["chars"].(float64)
	if !textOK || !charsOK || chars != float64(utf8.RuneCountInString(text)) {
		return fmt.Errorf("FAIL: chars disagrees with the message it counts")
	}

	counted := run("|", "count")
	if !strings.Contains(counted.stdout, strconv.Itoa(len(reportPosts))) {
		return fmt.Errorf("FAIL: `le weekly | count` answered %q, want %d", counted.stdout, len(reportPosts))
	}

	matched := run("|", "match", weeks[1][0])
	if !strings.Contains(matched.stdout, weeks[1][0]) {
		return fmt.Errorf("FAIL: `le weekly | match %s` dropped the row it selected:\n%s", weeks[1][0], matched.stdout)
	}

	asYAML := run("|", "yaml")
	if asYAML.exitCode != 0 {
		return fmt.Errorf("FAIL: `le weekly | yaml` exited %d", asYAML.exitCode)
	}
	if !strings.Contains(asYAML.stdout, "planned") {
		return fmt.Errorf("FAIL: `le weekly | yaml` answered nothing usable:\n%s", asYAML.stdout)
	}

	refused := run(filepath.Join(posts, "1999-01-04.md"))
	if refused.exitCode != 1 {
		return fmt.Errorf("FAIL: a bare path exited %d, want 1", refused.exitCode)
	}
	if !strings.Contains(refused.stderr, "unknown keyword") {
		return fmt.Errorf("FAIL: the refusal does not name the problem:\n%s", refused.stderr)
	}

	swept := run("confirm", "force")
	if swept.exitCode != 1 {
		return fmt.Errorf("FAIL: force on a sweep exited %d, want 1", swept.exitCode)
	}
	if !strings.Contains(swept.stderr, "source") {
		return fmt.Errorf("FAIL: the refusal does not say what to add:\n%s", swept.stderr)
	}

	fmt.Println("OK")
	return nil
}

func runWeeklyCommand(ctx context.Context, dir string, env []string, name string, args ...string) weeklyCommandResult {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		exitCode = -1
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			exitCode = exitErr.ExitCode()
		} else if stderr.Len() == 0 {
			stderr.WriteString(err.Error())
		}
	}
	return weeklyCommandResult{
		stdout:   stdout.String(),
		stderr:   stderr.String(),
		exitCode: exitCode,
	}
}

func replaceWeeklyEnv(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	for _, item := range env {
		if !strings.HasPrefix(item, prefix) {
			out = append(out, item)
		}
	}
	return append(out, prefix+value)
}
