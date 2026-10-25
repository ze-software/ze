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
		Parameters: []leaction.Parameter{{Keyword: "quiet", Kind: leaction.KindSwitch, Help: "print nothing when the links are already right"}},
		AnswerArgs: runEnsure},
	leaction.Action{Verb: "cache-clean", Why: "clean EVERY build cache this checkout fills and report, for each, what it" +
		" removed, what it kept and the disk space left: the one Go cache at cache/go-cache, which every le" +
		" action and every verify worktree writes through the cache/ link to ~/.cache/ze, the" +
		" ambient Go cache a bare `go` command writes (a SKIP row when `go env GOCACHE` is that same" +
		" directory), the launcher's fallback Go cache at tmp/go-cache, written only when cache/ cannot" +
		" be created, the golangci-lint cache at cache/golangci-lint-cache, the Staticcheck matrix" +
		" cache at cache/staticcheck-matrix, and one row for each cache.stale-* copy the link repair" +
		" renamed aside, removed whole once it is older than the floor." +
		" By default it removes only the entries older than the store trim's 3h floor, so it is safe" +
		" while other sessions build: an entry is safe for 2h after Go last looked it up (Go refreshes" +
		" an entry's mtime at most hourly; the floor is 3h), and a build that later misses an older" +
		" entry recompiles it rather than failing." +
		" `" + everythingMode + "` empties every cache outright, and refuses, naming each process, while any" +
		" go, compile, link, vet, staticcheck, golangci-lint or test binary runs, or a le job or verify run" +
		" holds a slot; a build that starts while the empty runs is not seen. Both modes drop a patched GOROOT under cache/stdpatch whose toolchain links dangle," +
		" and a staging directory a killed tree build left. Run it when unrelated packages fail to build," +
		" when a linker says `no space left on device`, or when a whole suite goes red at once, after" +
		" `./le scratch store-trim`. Read free space with `df -h` on the cache path, never with `stat -f`," +
		" which is a format flag on macOS and prints the path back" +
		" (plan/journal/full-disk-false-red.md)",
		Writes:     true,
		Parameters: []leaction.Parameter{{Keyword: everythingMode, Kind: leaction.KindSwitch, Help: "empty every cache, not only the entries older than three hours"}},
		AnswerArgs: runCacheClean},
	leaction.Action{Verb: storeTrimVerb, Why: "bound the build stores this checkout fills, now, without waiting for" +
		" the hourly automatic trim every le invocation starts as a detached child" +
		" (`" + backgroundKeyword + "` is that child's keyword: it writes to" +
		" tmp/store-trim/last.log and exits when a previous trim still runs)." +
		" The automatic trim follows " + StoreTrimKey + ", on or off; a trim asked" +
		" for by name runs either way and says which",
		Writes:     true,
		Parameters: []leaction.Parameter{{Keyword: backgroundKeyword, Kind: leaction.KindSwitch, Help: "run the trim in the background"}},
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

func runCacheClean(arguments leaction.Arguments) (any, int) {
	manager, err := managerHere()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return manager.CleanCaches(cleanModeOf(arguments))
}

// cleanModeOf answers the mode the cache-clean keywords select: the safe
// default with no keyword, and the full empty only for `all`.
func cleanModeOf(arguments leaction.Arguments) cleanMode {
	if arguments.Has(everythingMode) {
		return cleanEverything
	}
	return cleanOldEntries
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
