// Design: docs/architecture/core-design.md -- native scratch-link gates
// Overview: scratch.go -- filesystem policy and implementation
//
// This file empties the four build caches a Ze checkout fills, and reports the
// disk space each one returned.
//
// Three of them are Go caches, on as many filesystems, and a session that
// empties one keeps filling the others. Every le action writes the checkout
// cache, because gotoolchain.Overrides points GOCACHE at cache/go-cache. A bare
// `go` command typed outside le writes the ambient cache, which is the machine
// default. The bootstrap cache is what runs with no inherited GOCACHE at all:
// the `le` script building bin/ze-le, the deployment daemon and VPP evidence
// builds, and the QEMU guest. It was missed until 2026-09-20, because each of
// those four writers spelled the path itself and this action could not know
// about a path nobody declared; it held 1.3G and an operator found it by hand.
// gotoolchain.BootstrapCache is the declaration all of them now read.
//
// The fourth is golangci-lint's, which no `go clean` reaches and which the
// scratch relocation leaves on the checkout's own device. Emptying only the Go
// caches left it growing unbounded: it was measured at 9.5G on 2026-09-13,
// larger than both Go caches together, on a volume that had 1G left.
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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/diskspace"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/go/toolchain"
)

// cleanTimeout bounds one `go clean -cache` run. The measurement on
// 2026-08-20 emptied 256G in under a minute, so an hour is a stop for a run
// that has hung rather than a budget for a large cache.
const cleanTimeout = time.Hour

// goCacheKey is the variable that names the cache a `go` command uses.
const goCacheKey = "GOCACHE"

// The three caches, named as a person reads them in the report.
const (
	checkoutCache  = "checkout"
	ambientCache   = "ambient"
	lintCache      = "lint"
	bootstrapCache = "bootstrap"
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

	// Contended carries the emptier's complaint when the sweep FREED space and
	// then met a writer, rather than failing to clean. `go clean -cache` walks
	// the cache and removes each subdirectory, so a concurrent `go` command
	// writing into one it has already walked leaves it non-empty and the run
	// ends "unlinkat .../18: directory not empty". Measured 2026-09-19: the
	// cache had gone 41G to 114M and the command still exited 1
	// (plan/journal/full-disk-false-red.md).
	//
	// It is a separate field from Error because the two ask the operator for
	// different things. An Error means the disk is still full and the next
	// build will fail the same way. This means the space came back and another
	// session is using the cache, which is the normal state of a shared
	// checkout and needs no action at all.
	Contended string `json:"contended,omitempty"`
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
		case cache.Contended != "":
			line.PadRight(cache.Path, 48).Str(" freed ").Str(gibibytes(cache.Freed)).
				Str(", free ").Str(gibibytes(int64(cache.FreeAfter))). //nolint:gosec // a free-space count never exceeds the signed range on any device this runs on
				Str(" (a concurrent build kept writing: ").Str(cache.Contended).Byte(')')
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
	bootstrap := gotoolchain.BootstrapCache(root)

	targets := []cleanTarget{{
		name: checkoutCache, path: checkout,
		empty: func() error { return goCleanCache(ctx, checkout) },
	}, {
		name: ambientCache, path: ambient,
		empty: func() error { return goCleanCache(ctx, ambient) },
	}, {
		name: bootstrapCache, path: bootstrap,
		empty: func() error { return goCleanCache(ctx, bootstrap) },
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
//
// A CONTENDED cache is not a refusal and does not reach this. It freed its
// space and then lost a race with a live writer, which is the ordinary state of
// a checkout several sessions share. Exiting 1 there told an operator the clean
// had failed on a run that had just returned 41G, and the next thing they
// reached for was `rm -rf` on a cache directory, which is the action this
// command exists to make unnecessary (plan/journal/full-disk-false-red.md).
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

	// The emptier's error is held, not returned, until the disk has been read
	// again. What it MEANS depends on whether the space came back, and only the
	// second reading answers that.
	emptyErr := empty()

	after, err := diskspace.Free(path)
	if err != nil {
		cache.Error = err.Error()
		return cache
	}
	cache.FreeAfter = after
	if emptyErr != nil {
		if !isWriterRace(emptyErr) {
			cache.Error = emptyErr.Error()
			return cache
		}
		cache.Contended = emptyErr.Error()
	}
	cache.Freed = int64(after) - int64(before) //nolint:gosec // a free-space count never exceeds the signed range on any device this runs on
	return cache
}

// isWriterRace answers whether an emptier's failure is another session writing
// rather than a cache that could not be emptied.
//
// It reads the error and NOT the free-space delta. The delta looks like the
// obvious test and is not one: this device is shared, so a second session
// allocating while this one frees can leave the reading flat or negative after
// a clean that worked perfectly, and a small cache frees less than the
// granularity the reading has. That is the machine-dependent verdict this
// repository already collects rows about
// (plan/journal/gate-verdict-depends-on-the-machine.md).
//
// ENOTEMPTY is a race BY CONSTRUCTION, which is what makes it a sound signal.
// `go clean -cache` only ever removes, so a directory it walked cannot be
// non-empty unless something put entries back behind it. Nothing else this
// command does produces that error.
//
// The match is on text because the error arrives as another tool's combined
// output rather than as a wrapped syscall error, and `go clean` prints the
// operating system's wording. Both spellings are listed: Go's os package
// renders ENOTEMPTY as "directory not empty", and a localized or BSD libc path
// can produce the bare errno name.
func isWriterRace(err error) bool {
	if errors.Is(err, syscall.ENOTEMPTY) {
		return true
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "directory not empty") || strings.Contains(text, "enotempty")
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
