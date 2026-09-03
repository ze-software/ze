// Design: docs/architecture/core-design.md -- le action packages
// Overview: scratch.go -- filesystem policy and implementation
package scratch

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

var actions = leaction.New(area,
	leaction.Action{Verb: "links-ensure", Why: "point the tmp/ and cache/ symlinks at their out-of-tree targets before any" +
		" target writes scratch. This replaces the old tmp/go.mod nested-module" +
		" sentinel: `go list ./...` skips a directory SYMLINK named tmp/ (verified)," +
		" so no marker file is needed (plan/spec-relocate-scratch-and-cache.md)",
		Writes:     true,
		Parameters: []leaction.Parameter{{Keyword: "quiet"}},
		AnswerArgs: runEnsure},
	leaction.Action{Verb: "cache-clean", Why: "empty BOTH Go build caches and report the disk space each one" +
		" returned: the checkout cache at cache/go-cache that every le action" +
		" writes, and the ambient cache a bare `go` command writes. Run it when" +
		" unrelated packages fail to build, when a linker says `no space left on" +
		" device`, or when a whole suite goes red at once. cache/ is a symlink" +
		" onto another filesystem, so `df` on the checkout answers about the" +
		" wrong device and a full cache disk reads as a code defect" +
		" (plan/journal/full-disk-false-red.md)",
		Writes: true,
		Answer: runCacheClean},
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
	root, err := lepath.Root()
	if err != nil {
		return nil, err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve checkout root: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolve checkout symlinks: %w", err)
	}
	return New(root, os.Environ()), nil
}

func writeErrors(stderr io.Writer, report Report) {
	for _, result := range report.Results {
		if result.Stderr {
			fmt.Fprintln(stderr, result.Line) //nolint:errcheck // CLI diagnostic output
		}
	}
}
