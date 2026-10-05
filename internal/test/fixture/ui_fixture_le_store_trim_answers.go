// Design: docs/contributing/running-commands.md -- when the disk is full: the store trim
// Related: register_le_store_trim_answers.go -- the registration
// Related: ui_fixture_le_binary_dispatches.go -- the built-le fixture this one follows

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
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The budget the fixture sets for the Go caches, and the shape of the seeded
// cache that overruns it. Every entry is far older than the trim's 3h age
// floor, so the floor never stops the trim, and the entries alternate between
// the checkout and the shared cache so only one global oldest-first order over
// the union leaves the right ones.
const (
	leStoreTrimBudget        = "1M"
	leStoreTrimBudgetBytes   = 1 << 20
	leStoreTrimEntryBytes    = 256 << 10
	leStoreTrimEntries       = 12
	leStoreTrimOldestHours   = 30
	leStoreTrimGoBudgetKey   = "ze.le.store.go-cache-budget"
	leStoreTrimLintBudgetKey = "ze.le.store.lint-cache-budget"
	leStoreTrimSwitchKey     = "ze.le.store.trim"
	leStoreTrimCompletePolls = 300
	leStoreTrimBlockBytes    = 512
)

// leStoreTrimEntry is one seeded Go cache entry: where it lives and how old it
// was made.
type leStoreTrimEntry struct {
	path string
	age  time.Duration
}

type leStoreTrimResult struct {
	stdout   string
	stderr   string
	exitCode int
}

// leStoreTrimAnswers proves the hourly store trim through the built le binary.
// It builds le from this checkout, seeds a throwaway checkout and a throwaway
// per-user cache with Go cache entries over a small budget, then runs a cheap
// le command twice. The first run spawns the detached trim, which leaves the
// union under budget, oldest entries removed first, and writes the stamp. The
// second run, inside the hour, spawns nothing: the stamp is not rewritten and
// the trim log is untouched. Last, `le scratch store-trim` answers one row per
// store in text and JSON.
//
// The real checkout and the real per-user cache are never named to the le it
// runs: ZE_REPO_ROOT, XDG_CACHE_HOME and CLAUDE_CONFIG_DIR all point into the
// fixture's own directory, so the small budget can only trim what it seeded.
func leStoreTrimAnswers(ctx context.Context) error {
	source := os.Getenv("ZE_REPO_ROOT")
	if source == "" {
		return leStoreTrimFailf("ZE_REPO_ROOT is not set")
	}
	source, err := filepath.Abs(source)
	if err != nil {
		return leStoreTrimFailf("resolve ZE_REPO_ROOT: %v", err)
	}

	work, err := os.MkdirTemp("", "le-store-trim-")
	if err != nil {
		return leStoreTrimFailf("create fixture directory: %v", err)
	}
	defer os.RemoveAll(work) //nolint:errcheck // fixture cleanup

	// le resolves its checkout through its symlinks, so the fixture compares
	// against the same spelling.
	work, err = filepath.EvalSymlinks(work)
	if err != nil {
		return leStoreTrimFailf("resolve fixture directory: %v", err)
	}
	lePath, err := leStoreTrimBuild(ctx, source, work)
	if err != nil {
		return err
	}

	checkout := filepath.Join(work, "checkout")
	perUser := filepath.Join(work, "xdg")
	checkoutCache := filepath.Join(checkout, "cache", "go-cache")
	sharedCache := filepath.Join(perUser, "ze", "go-cache")
	entries, err := leStoreTrimSeed(checkoutCache, sharedCache)
	if err != nil {
		return err
	}
	environment := childEnvironment(envRootedAt(checkout), map[string]string{
		leStoreTrimGoBudgetKey:   leStoreTrimBudget,
		leStoreTrimLintBudgetKey: leStoreTrimBudget,
		leStoreTrimSwitchKey:     "on",
		"XDG_CACHE_HOME":         perUser,
		"CLAUDE_CONFIG_DIR":      filepath.Join(work, "claude"),
	})
	le := func(args ...string) (leStoreTrimResult, error) {
		return leStoreTrimRun(ctx, lePath, checkout, environment, args...)
	}

	stampPath := filepath.Join(checkout, "tmp", "store-trim", "stamp")
	logPath := filepath.Join(checkout, "tmp", "store-trim", "last.log")
	if _, err := os.Lstat(stampPath); !errors.Is(err, os.ErrNotExist) {
		return leStoreTrimFailf("the seeded checkout already holds a stamp: %v", err)
	}

	first, err := le("scratch")
	if err != nil {
		return err
	}
	if first.exitCode != 0 {
		return leStoreTrimFailf("the first `le scratch` exited %d: %s", first.exitCode, first.stderr)
	}
	if err := leStoreTrimAwaitChild(ctx, logPath); err != nil {
		return err
	}
	stampBefore, err := leStoreTrimStamp(stampPath)
	if err != nil {
		return err
	}
	if err := leStoreTrimUnderBudget(entries); err != nil {
		return err
	}
	logBefore, err := os.ReadFile(logPath) //nolint:gosec // the fixture's own throwaway checkout
	if err != nil {
		return leStoreTrimFailf("read the trim log: %v", err)
	}
	if runs := strings.Count(string(logBefore), ": run started\n"); runs != 1 {
		return leStoreTrimFailf("the trim log names %d runs, want exactly 1:\n%s", runs, logBefore)
	}

	second, err := le("scratch")
	if err != nil {
		return err
	}
	if second.exitCode != first.exitCode {
		return leStoreTrimFailf("the second `le scratch` exited %d, the first %d", second.exitCode, first.exitCode)
	}
	if second.stdout != first.stdout {
		return leStoreTrimFailf("the trigger changed the command's output:\nfirst:\n%s\nsecond:\n%s", first.stdout, second.stdout)
	}
	stampAfter, err := os.Lstat(stampPath)
	if err != nil {
		return leStoreTrimFailf("the stamp vanished after the second run: %v", err)
	}
	// The stamp is written through a temporary file and a rename, so a second
	// spawn would leave a different file here even within the same second.
	if !os.SameFile(stampBefore, stampAfter) {
		return leStoreTrimFailf("the second run inside the hour rewrote the stamp, so it started a trim")
	}
	logAfter, err := os.ReadFile(logPath) //nolint:gosec // the fixture's own throwaway checkout
	if err != nil {
		return leStoreTrimFailf("read the trim log after the second run: %v", err)
	}
	if !bytes.Equal(logBefore, logAfter) {
		return leStoreTrimFailf("the second run inside the hour touched the trim log:\n%s", logAfter)
	}

	if err := leStoreTrimForeground(le, checkoutCache); err != nil {
		return err
	}
	var verdict textbuf.Buffer
	return verdict.Str("OK\n").StdOut()
}

// leStoreTrimBuild compiles the le personality from the checkout at source into
// work/le. The name matters: le derives the trim child's argv from it.
func leStoreTrimBuild(ctx context.Context, source, work string) (string, error) {
	tags, err := uiLEFeatureTags(source)
	if err != nil {
		return "", leStoreTrimFailf("%v", err)
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		return "", leStoreTrimFailf("find go: %v", err)
	}
	lePath := filepath.Join(work, "le")
	build := exec.CommandContext(ctx, goTool, "build", "-tags", strings.Join(tags, ","), "-o", lePath, "./cmd/ze") //nolint:gosec // the fixture chooses the program and its arguments
	build.Dir = source
	build.Env = os.Environ()
	var output bytes.Buffer
	build.Stdout = &output
	build.Stderr = &output
	if err := build.Run(); err != nil {
		return "", leStoreTrimFailf("building the le personality: %v\n%s", err, output.String())
	}
	return lePath, nil
}

// leStoreTrimSeed writes leStoreTrimEntries Go cache entries of real data,
// alternating between the two caches, the oldest first, each an hour younger
// than the one before it.
func leStoreTrimSeed(checkoutCache, sharedCache string) ([]leStoreTrimEntry, error) {
	data := bytes.Repeat([]byte{'z'}, leStoreTrimEntryBytes)
	now := time.Now()
	entries := make([]leStoreTrimEntry, 0, leStoreTrimEntries)
	for index := range leStoreTrimEntries {
		cache := checkoutCache
		if index%2 == 1 {
			cache = sharedCache
		}
		subdirectory := filepath.Join(cache, "0"+strconv.Itoa(index%10))
		if err := os.MkdirAll(subdirectory, 0o750); err != nil {
			return nil, leStoreTrimFailf("create cache subdirectory: %v", err)
		}
		path := filepath.Join(subdirectory, "seeded"+strconv.Itoa(index)+"-a")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return nil, leStoreTrimFailf("seed cache entry: %v", err)
		}
		age := time.Duration(leStoreTrimOldestHours-index) * time.Hour
		stamp := now.Add(-age)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			return nil, leStoreTrimFailf("age cache entry: %v", err)
		}
		entries = append(entries, leStoreTrimEntry{path: path, age: age})
	}
	return entries, nil
}

func leStoreTrimRun(ctx context.Context, lePath, dir string, environment []string, args ...string) (leStoreTrimResult, error) {
	command := exec.CommandContext(ctx, lePath, args...) //nolint:gosec // the fixture chooses the program and its arguments
	command.Dir = dir
	command.Env = environment
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	result := leStoreTrimResult{stdout: stdout.String(), stderr: stderr.String(), exitCode: 0}
	if err == nil {
		return result, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		result.exitCode = exitErr.ExitCode()
		return result, nil
	}
	return result, leStoreTrimFailf("execute le %s: %v", strings.Join(args, " "), err)
}

// leStoreTrimAwaitChild waits for the detached trim to finish. The child
// writes its report to last.log as its last act, and the Go cache budget line
// is in that report, so its presence means the trim is over. The wait is
// bounded at 300 polls 100 milliseconds apart, and absence is a failure: a
// trigger that never spawned leaves no log at all.
func leStoreTrimAwaitChild(ctx context.Context, logPath string) error {
	finished := Poll(ctx, leStoreTrimCompletePolls, 100*time.Millisecond, func() bool {
		content, err := os.ReadFile(logPath) //nolint:gosec // the fixture's own throwaway checkout
		if err != nil {
			return false
		}
		return strings.Contains(string(content), "budget "+leStoreTrimGoBudgetKey)
	})
	if finished {
		return nil
	}
	content, err := os.ReadFile(logPath) //nolint:gosec // the fixture's own throwaway checkout
	if err != nil {
		return leStoreTrimFailf("the first le run started no background trim: %v", err)
	}
	return leStoreTrimFailf("the background trim did not finish in 30s; last.log:\n%s", content)
}

// leStoreTrimStamp answers the stamp's file and checks it holds a unix time
// no later than now.
func leStoreTrimStamp(stampPath string) (os.FileInfo, error) {
	info, err := os.Lstat(stampPath)
	if err != nil {
		return nil, leStoreTrimFailf("the trim wrote no stamp: %v", err)
	}
	content, err := os.ReadFile(stampPath) //nolint:gosec // the fixture's own throwaway checkout
	if err != nil {
		return nil, leStoreTrimFailf("read the stamp: %v", err)
	}
	seconds, err := strconv.ParseInt(strings.TrimSpace(string(content)), 10, 64)
	if err != nil {
		return nil, leStoreTrimFailf("the stamp %q is not unix seconds: %v", content, err)
	}
	if seconds > time.Now().Unix() {
		return nil, leStoreTrimFailf("the stamp %d is in the future", seconds)
	}
	return info, nil
}

// leStoreTrimUnderBudget checks the union of both caches is at or under the
// budget, counting allocated size as le does, and that every removed entry is
// older than every kept one, whichever cache each lived in.
func leStoreTrimUnderBudget(entries []leStoreTrimEntry) error {
	var keptBytes int64
	keptAgeMax := time.Duration(-1)
	removedAgeMin := time.Duration(-1)
	removed := 0
	for _, entry := range entries {
		info, err := os.Lstat(entry.path)
		if errors.Is(err, os.ErrNotExist) {
			removed++
			if removedAgeMin < 0 || entry.age < removedAgeMin {
				removedAgeMin = entry.age
			}
			continue
		}
		if err != nil {
			return leStoreTrimFailf("stat %s: %v", entry.path, err)
		}
		size, err := leStoreTrimAllocated(info)
		if err != nil {
			return err
		}
		keptBytes += size
		keptAgeMax = max(keptAgeMax, entry.age)
	}
	if removed == 0 {
		return leStoreTrimFailf("the trim removed nothing from a cache seeded over its budget")
	}
	if keptBytes > leStoreTrimBudgetBytes {
		return leStoreTrimFailf("the caches hold %d bytes after the trim, over the %d-byte budget", keptBytes, leStoreTrimBudgetBytes)
	}
	if keptAgeMax >= removedAgeMin {
		return leStoreTrimFailf("an entry %v old was kept while one %v old was removed", keptAgeMax, removedAgeMin)
	}
	return nil
}

// leStoreTrimAllocated answers the bytes a file occupies on disk, the measure
// le's trim counts against a budget. A platform that reports no block count is
// an error, never a zero.
func leStoreTrimAllocated(info os.FileInfo) (int64, error) {
	status, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, leStoreTrimFailf("%s: no block count on this platform", info.Name())
	}
	return status.Blocks * leStoreTrimBlockBytes, nil
}

// leStoreTrimForeground runs `le scratch store-trim` in text and JSON and
// checks it answers one row per store: every store the JSON names appears
// exactly once in the text, no name repeats, and the checkout cache the
// fixture seeded is a measured row under the budget.
func leStoreTrimForeground(le func(args ...string) (leStoreTrimResult, error), checkoutCache string) error {
	text, err := le("scratch", "store-trim")
	if err != nil {
		return err
	}
	if text.exitCode != 0 {
		return leStoreTrimFailf("`le scratch store-trim` exited %d:\n%s%s", text.exitCode, text.stdout, text.stderr)
	}
	answer, err := le("scratch", "store-trim", "|", "json")
	if err != nil {
		return err
	}
	if answer.exitCode != 0 {
		return leStoreTrimFailf("`le scratch store-trim | json` exited %d:\n%s%s", answer.exitCode, answer.stdout, answer.stderr)
	}
	var report struct {
		Stores []struct {
			Name      string `json:"name"`
			Path      string `json:"path"`
			SizeAfter int64  `json:"size-after"`
			Skipped   string `json:"skipped"`
			Refused   string `json:"refused"`
		} `json:"stores"`
	}
	if err := json.Unmarshal([]byte(answer.stdout), &report); err != nil {
		return leStoreTrimFailf("decode `le scratch store-trim | json`: %v\n%s", err, answer.stdout)
	}
	if len(report.Stores) == 0 {
		return leStoreTrimFailf("`le scratch store-trim | json` answered no store")
	}
	lines := strings.Split(text.stdout, "\n")
	names := make([]string, 0, len(report.Stores))
	seededMeasured := false
	for _, store := range report.Stores {
		if slices.Contains(names, store.Name) {
			return leStoreTrimFailf("the store %q has two rows", store.Name)
		}
		names = append(names, store.Name)
		rows := 0
		for _, line := range lines {
			if strings.HasPrefix(line, store.Name+" ") {
				rows++
			}
		}
		if rows != 1 {
			return leStoreTrimFailf("the text names the store %q on %d rows, want 1:\n%s", store.Name, rows, text.stdout)
		}
		if store.Path != checkoutCache {
			continue
		}
		if store.Skipped != "" {
			return leStoreTrimFailf("the seeded checkout cache was skipped: %s", store.Skipped)
		}
		if store.Refused != "" {
			return leStoreTrimFailf("the seeded checkout cache was refused: %s", store.Refused)
		}
		if store.SizeAfter > leStoreTrimBudgetBytes {
			return leStoreTrimFailf("the checkout cache answers %d bytes, over the budget", store.SizeAfter)
		}
		seededMeasured = true
	}
	if !seededMeasured {
		return leStoreTrimFailf("no store row names the seeded checkout cache %s:\n%s", checkoutCache, text.stdout)
	}
	return nil
}

func leStoreTrimFailf(format string, args ...any) error {
	return fmt.Errorf("FAIL: "+format, args...)
}
