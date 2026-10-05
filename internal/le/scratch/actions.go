// Design: docs/architecture/core-design.md -- le action packages
// Overview: scratch.go -- filesystem policy and implementation
package scratch

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

var actions = leaction.New(area,
	leaction.Action{Verb: "links-ensure", Why: "point the tmp/ and cache/ symlinks at their out-of-tree targets before any" +
		" target writes scratch. This replaces the old tmp/go.mod nested-module" +
		" sentinel: `go list ./...` skips a directory SYMLINK named tmp/ (verified)," +
		" so no marker file is needed (plan/spec-relocate-scratch-and-cache.md)",
		Writes:     true,
		Parameters: []leaction.Parameter{{Keyword: "quiet"}},
		AnswerArgs: runEnsure},
	leaction.Action{Verb: "cache-clean", Why: "empty EVERY build cache this checkout fills and report the disk space" +
		" each one returned: the checkout Go cache at cache/go-cache that every le" +
		" action writes, the shared per-user Go cache at ~/.cache/ze/go-cache that" +
		" every verify worktree writes (a SKIP row when cache/ links to it), the" +
		" ambient Go cache a bare `go` command writes, the bootstrap Go cache at" +
		" tmp/go-cache, and the golangci-lint cache at tmp/golangci-lint-cache that" +
		" no `go clean` reaches. Run it when unrelated packages fail to build, when a linker" +
		" says `no space left on device`, or when a whole suite goes red at once." +
		" Read free space with `df -h` on the cache path, never with `stat -f`," +
		" which is a format flag on macOS and prints the path back" +
		" (plan/journal/full-disk-false-red.md)",
		Writes: true,
		Answer: runCacheClean},
	leaction.Action{Verb: storeTrimVerb, Why: "bound the build stores this checkout fills, now, without waiting for" +
		" the hourly automatic trim every le invocation starts as a detached child" +
		" (`" + backgroundKeyword + "` is that child's keyword: it writes to" +
		" tmp/store-trim/last.log and exits when a previous trim still runs)." +
		" The automatic trim follows " + StoreTrimKey + ", on or off; a trim asked" +
		" for by name runs either way and says which",
		Writes:     true,
		Parameters: []leaction.Parameter{{Keyword: backgroundKeyword}},
		AnswerArgs: runStoreTrim},
	leaction.Action{Verb: "migrate", Why: "the same cutover for a checkout whose tmp/ or cache/ is still a REAL" +
		" directory: move its entries to the out-of-tree target and leave a symlink" +
		" behind, refusing rather than clobbering a name the target already holds" +
		" (internal/le/scratch/move.go, migrate). A path that is already a symlink" +
		" needs no migration and takes the ensure route instead",
		Writes: true,
		Answer: runMigrate},
)

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the le scratch command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

func runEnsure(arguments leaction.Arguments) (any, int) {
	manager, err := managerHere()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return answerEnsure(manager, arguments, os.Stderr)
}

func answerEnsure(manager *Manager, arguments leaction.Arguments, stderr io.Writer) (Report, int) {
	report, code := manager.Ensure(false)
	report.Quiet = arguments.Has("quiet")
	writeErrors(stderr, report)
	return report, code
}

func runCacheClean() (any, int) {
	manager, err := managerHere()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return manager.CleanCaches()
}

func runMigrate() (any, int) {
	manager, err := managerHere()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	report, code := manager.Migrate(false)
	writeErrors(os.Stderr, report)
	return report, code
}

func managerHere() (*Manager, error) {
	root, err := checkoutRoot()
	if err != nil {
		return nil, err
	}
	return New(root, os.Environ()), nil
}

// checkoutRoot answers the checkout le works in, absolute and with its
// symlinks resolved, so two spellings of one checkout name one set of stores.
func checkoutRoot() (string, error) {
	root, err := lepath.Root()
	if err != nil {
		return "", err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve checkout root: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve checkout symlinks: %w", err)
	}
	return root, nil
}

func writeErrors(stderr io.Writer, report Report) {
	for _, result := range report.Results {
		if result.Stderr {
			fmt.Fprintln(stderr, result.Line) //nolint:errcheck // CLI diagnostic output
		}
	}
}
