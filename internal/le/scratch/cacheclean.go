// Design: docs/architecture/core-design.md -- native scratch-link gates
// Overview: scratch.go -- filesystem policy and implementation
//
// This file empties the three build caches a Ze checkout fills, and reports
// the disk space each one returned.
//
// Two of them are Go caches, on two filesystems, and a session that empties one
// keeps filling the other. Every le action writes the checkout cache, because
// gotoolchain.Overrides points GOCACHE at cache/go-cache. A bare `go` command
// typed outside le writes the ambient cache, which is the machine default.
//
// The third is golangci-lint's, which no `go clean` reaches and which the
// scratch relocation leaves on the checkout's own device. Emptying only the Go
// pair left it growing unbounded: it was measured at 9.5G on 2026-09-13, larger
// than both Go caches together, on a volume that had 1G left.
//
// The cost of not having this action is recorded in
// plan/journal/full-disk-false-red.md, one row for each time a full cache disk
// was read as a code defect. That file is the count, because a copy of it here
// goes stale on the next row. The checkout path hides the disk, because cache/
// is a symlink onto another filesystem, so `df` on the checkout answers about
// the wrong device. The free space this
// action prints is read with statfs on the cache path itself.
package scratch

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/diskspace"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gotoolchain"
)

// cleanTimeout bounds one `go clean -cache` run. The measurement on
// 2026-08-20 emptied 256G in under a minute, so an hour is a stop for a run
// that has hung rather than a budget for a large cache.
const cleanTimeout = time.Hour

// goCacheKey is the variable that names the cache a `go` command uses.
const goCacheKey = "GOCACHE"

// The three caches, named as a person reads them in the report.
const (
	checkoutCache = "checkout"
	ambientCache  = "ambient"
	lintCache     = "lint"
)

// bytesPerGiB converts a byte count to the unit the report prints.
const bytesPerGiB = 1 << 30

// CacheClean is what one `go clean -cache` run did to one cache.
type CacheClean struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	FreeBefore uint64 `json:"free_before"`
	FreeAfter  uint64 `json:"free_after"`
	Freed      int64  `json:"freed"`
	Skipped    string `json:"skipped,omitempty"`
	Error      string `json:"error,omitempty"`
}

// CleanReport is the answer of one cache-clean run, one row for each cache.
type CleanReport struct {
	Caches []CacheClean `json:"caches"`
}

// Text renders one line for each cache. Freed and free are gibibytes, because
// the number a person acts on is whether the disk has room for a build.
func (r CleanReport) Text() string {
	var text textbuf.Buffer
	text.Reset()
	for _, cache := range r.Caches {
		var line textbuf.Buffer
		line.PadRight(cache.Name, 9)
		switch {
		case cache.Error != "":
			line.Str("REFUSE   ").Str(cache.Path).Str(": ").Str(cache.Error)
		case cache.Skipped != "":
			line.Str("SKIP     ").Str(cache.Path).Str(": ").Str(cache.Skipped)
		default:
			line.PadRight(cache.Path, 48).Str(" freed ").Str(gibibytes(cache.Freed)).
				Str(", free ").Str(gibibytes(int64(cache.FreeAfter))) //nolint:gosec // a free-space count never exceeds the signed range on any device this runs on
		}
		text.Str(line.String()).Byte('\n')
	}
	return text.String()
}

// CleanCaches empties every build cache this checkout fills and reports what
// each one returned.
//
// The ambient cache is resolved by asking `go env GOCACHE` with the inherited
// GOCACHE removed, so the answer is the machine default rather than whatever
// le set for the calling process. A checkout whose GOCACHE already IS the
// machine default is emptied once and the second row says so.
func (m *Manager) CleanCaches() (CleanReport, int) {
	ctx, cancel := context.WithTimeout(context.Background(), cleanTimeout)
	defer cancel()

	// The ambient row is the only one whose path has to be resolved, so it is
	// the only one that can fail before anything is emptied.
	ambient, err := ambientGoCache(ctx)

	var report CleanReport
	for _, target := range cleanTargets(ctx, m.Root, ambient) {
		switch {
		case target.name == ambientCache && err != nil:
			report.Caches = append(report.Caches, CacheClean{Name: target.name, Error: err.Error()})
		case target.skipped != "":
			report.Caches = append(report.Caches, CacheClean{
				Name: target.name, Path: target.path, Skipped: target.skipped,
			})
		default:
			report.Caches = append(report.Caches, measureClean(target.name, target.path, target.empty))
		}
	}
	return report, report.verdict()
}

// cleanTarget is one cache this action empties: how a person reads it in the
// report, where it lives, and what emptying it takes.
type cleanTarget struct {
	name    string
	path    string
	empty   func() error
	skipped string
}

// cleanTargets answers every cache a checkout at root fills, in report order.
//
// It takes the ambient path rather than resolving it, so the whole plan is
// readable without running a command, and so a caller that failed to resolve it
// still gets the other two rows.
func cleanTargets(ctx context.Context, root, ambient string) []cleanTarget {
	checkout := gotoolchain.GoCache(root)
	lint := gotoolchain.LintCache(root)

	targets := []cleanTarget{{
		name: checkoutCache, path: checkout,
		empty: func() error { return goCleanCache(ctx, checkout) },
	}, {
		name: ambientCache, path: ambient,
		empty: func() error { return goCleanCache(ctx, ambient) },
	}, {
		name: lintCache, path: lint,
		empty: func() error { return removeCache(lint) },
	}}
	if ambient == checkout {
		targets[1].skipped = "the machine default is the checkout cache, which this run already emptied"
	}
	return targets
}

// verdict answers 1 when any cache refused, so a caller sees the failure.
func (r CleanReport) verdict() int {
	for _, cache := range r.Caches {
		if cache.Error != "" {
			return 1
		}
	}
	return 0
}

// measureClean empties one cache with the emptier that cache needs, and
// measures the device it sits on before and after. The two readings are taken
// on the cache path itself, never on the checkout, because a cache can sit on
// a filesystem the checkout does not.
func measureClean(name, path string, empty func() error) CacheClean {
	cache := CacheClean{Name: name, Path: path}
	before, err := diskspace.Free(path)
	if err != nil {
		cache.Error = err.Error()
		return cache
	}
	cache.FreeBefore = before

	if err := empty(); err != nil {
		cache.Error = err.Error()
		return cache
	}

	after, err := diskspace.Free(path)
	if err != nil {
		cache.Error = err.Error()
		return cache
	}
	cache.FreeAfter = after
	cache.Freed = int64(after) - int64(before) //nolint:gosec // a free-space count never exceeds the signed range on any device this runs on
	return cache
}

// goCleanCache runs `go clean -cache` against one explicit cache directory.
func goCleanCache(ctx context.Context, cache string) error {
	var text textbuf.Buffer
	command := exec.CommandContext(ctx, "go", "clean", "-cache")
	command.Env = append(os.Environ(), text.Str(goCacheKey).Byte('=').Str(cache).String())
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go clean -cache under %s=%s: %w: %s",
			goCacheKey, cache, err, strings.TrimSpace(string(output)))
	}
	return nil
}

// removeCache empties a cache the Go toolchain does not own by deleting it.
//
// `golangci-lint cache clean` exists and is not used: it empties whatever
// GOLANGCI_LINT_CACHE names for the process that runs it, so it answers about
// the caller's environment rather than about this checkout, and it refuses
// altogether on a machine where the linter is not installed. The path is what
// this action is emptying, and deleting it costs one slow lint run because the
// next one recreates it.
func removeCache(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

// ambientGoCache answers the cache a `go` command uses with no override in the
// environment.
func ambientGoCache(ctx context.Context) (string, error) {
	command := exec.CommandContext(ctx, "go", "env", goCacheKey)
	command.Env = withoutGoCache(os.Environ())
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("go env %s: %w", goCacheKey, err)
	}
	path := strings.TrimSpace(string(output))
	if path == "" {
		return "", fmt.Errorf("go env %s answered nothing", goCacheKey)
	}
	return path, nil
}

// withoutGoCache copies an environment with every GOCACHE entry dropped.
func withoutGoCache(environ []string) []string {
	var text textbuf.Buffer
	prefix := text.Str(goCacheKey).Byte('=').String()
	kept := make([]string, 0, len(environ))
	for _, entry := range environ {
		if strings.HasPrefix(entry, prefix) {
			continue
		}
		kept = append(kept, entry)
	}
	return kept
}

// gibibytes renders a byte count in the unit a disk is discussed in.
func gibibytes(bytes int64) string {
	var text textbuf.Buffer
	return text.Float(float64(bytes)/bytesPerGiB, 1).Byte('G').String()
}
