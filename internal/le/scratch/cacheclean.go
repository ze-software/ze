// Design: plan/spec-le-is-a-ze-binary.md -- native scratch-link gates
// Overview: scratch.go -- filesystem policy and implementation
//
// This file empties the two Go build caches a Ze checkout fills, and reports
// the disk space each one returned.
//
// Two caches exist, on two filesystems, and a session that empties one keeps
// filling the other. Every le action writes the checkout cache, because
// gotoolchain.Overrides points GOCACHE at cache/go-cache. A bare `go` command
// typed outside le writes the ambient cache, which is the machine default.
//
// The cost of not having this action is recorded: a full cache disk was read
// as a code defect four times (plan/journal/full-disk-false-red.md). The
// checkout path hides it, because cache/ is a symlink onto another filesystem,
// so `df` on the checkout answers about the wrong device. The free space this
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

// The two caches, named as a person reads them in the report.
const (
	checkoutCache = "checkout"
	ambientCache  = "ambient"
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

// CleanCaches empties both Go build caches and reports what each one returned.
//
// The ambient cache is resolved by asking `go env GOCACHE` with the inherited
// GOCACHE removed, so the answer is the machine default rather than whatever
// le set for the calling process. A checkout whose GOCACHE already IS the
// machine default is emptied once and the second row says so.
func (m *Manager) CleanCaches() (CleanReport, int) {
	ctx, cancel := context.WithTimeout(context.Background(), cleanTimeout)
	defer cancel()

	checkout := gotoolchain.GoCache(m.Root)
	report := CleanReport{Caches: []CacheClean{cleanCache(ctx, checkoutCache, checkout)}}

	ambient, err := ambientGoCache(ctx)
	switch {
	case err != nil:
		report.Caches = append(report.Caches, CacheClean{Name: ambientCache, Error: err.Error()})
	case ambient == checkout:
		report.Caches = append(report.Caches, CacheClean{
			Name: ambientCache, Path: ambient,
			Skipped: "the machine default is the checkout cache, which this run already emptied",
		})
	default:
		report.Caches = append(report.Caches, cleanCache(ctx, ambientCache, ambient))
	}
	return report, report.verdict()
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

// cleanCache empties one cache and measures the device it sits on before and
// after. The two readings are taken on the cache path itself, never on the
// checkout, because the checkout and the cache are on different filesystems.
func cleanCache(ctx context.Context, name, path string) CacheClean {
	cache := CacheClean{Name: name, Path: path}
	before, err := diskspace.Free(path)
	if err != nil {
		cache.Error = err.Error()
		return cache
	}
	cache.FreeBefore = before

	if err := goCleanCache(ctx, path); err != nil {
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
