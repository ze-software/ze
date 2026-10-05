// Design: docs/contributing/running-commands.md -- "When the disk is full", the store trim
// Overview: storetrim.go -- the trigger, the report and the store-trim action
// Related: cacheclean.go -- the store list the trim shares with cache-clean
//
// This file trims the Go-format caches: the three Go build caches as ONE union
// against one total (owner decision D-1), and the golangci-lint cache against
// its own budget. Both formats keep their entries as files, or for an
// executable directories, named <hash>-a or <hash>-d inside 256 two-hex-digit
// subdirectories, so one walk serves both.
//
// The trim removes individual entries, oldest mtime first, and never runs
// `go clean`: `go clean -cache` empties everything and fails when a peer
// writes while it walks (plan/journal/full-disk-false-red.md, 2026-09-04). Go
// refreshes an entry's mtime on use at most once an hour, so oldest mtime
// first is least recently used to within an hour, and an entry younger than
// the age floor is never removed, because a build that used it within the
// last two hours refreshed it past the floor (spec R-1).
package scratch

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// cacheEntryAgeFloor is the age below which no cache entry is removed,
// whatever the budget (owner decision D-3).
const cacheEntryAgeFloor = 3 * time.Hour

// statBlockBytes is the unit of st_blocks, which POSIX fixes at 512 bytes.
// The budget counts allocated blocks rather than the byte length, because the
// failure it prevents is a full device and blocks are what df counts (A-6).
const statBlockBytes = 512

// hexDigits spells the two-letter names of a cache's 256 subdirectories.
const hexDigits = "0123456789abcdef"

// The suffixes of the names Go's cache trim treats as entries; README,
// trim.txt and any other name are not entries and are never removed.
const (
	actionEntrySuffix = "-a"
	outputEntrySuffix = "-d"
)

// Why a store holds no trim row of its own.
const (
	machineCacheSkip = "the machine-wide Go cache, outside le, is left to Go's own five-day trim"
	absentStoreSkip  = "does not exist, so it holds nothing to trim"
)

// cachePass is the clock, the age floor and the remover one trim runs with.
// trimStores passes the real ones; a test passes a zero floor or a remover
// that answers a writer race (AC-17, AC-18).
type cachePass struct {
	now    time.Time
	floor  time.Duration
	remove func(path string, directory bool) error
}

// cacheEntry is one -a or -d entry found in a store.
type cacheEntry struct {
	path      string
	modified  time.Time
	bytes     int64
	directory bool
	store     int // the index of the store's target and row
}

// trimCaches trims every store in targets that a budget bounds, one pass per
// budget group, and answers one line per budget and one row per target, in
// target order. refused names stores whose path could not be resolved; each
// is refused by that reason and never walked. budgetOf answers a group's
// budget in bytes, or the refusal that keeps every store in the group
// untouched while the other groups proceed (AC-13).
//
// It is not safe to run twice at once over one store: the background child
// holds run.lock for that reason, and a foreground run beside it only costs
// removals that answer ENOENT, which count as gone.
func trimCaches(ctx context.Context, targets []cleanTarget, refused map[string]string,
	budgetOf func(budgetGroup) (int64, error), pass cachePass,
) ([]BudgetTrim, []StoreTrim) {
	rows := make([]StoreTrim, len(targets))
	for index, target := range targets {
		rows[index] = StoreTrim{Name: target.name, Path: target.path}
	}
	budgets := []BudgetTrim{}
	for index, target := range targets {
		if target.budget == notTrimmed {
			rows[index].Skipped = machineCacheSkip
			continue
		}
		entry, bounded := budgetEntry(target.budget)
		if !bounded {
			rows[index].Error = "declares no budget group, so the trim cannot bound it"
			continue
		}
		if slices.ContainsFunc(budgets, func(budget BudgetTrim) bool { return budget.Key == entry.Key }) {
			continue
		}
		line := BudgetTrim{Key: entry.Key}
		budget, err := budgetOf(target.budget)
		if err != nil {
			line.Refused = err.Error()
			refuseGroup(targets, target.budget, line.Refused, rows)
			budgets = append(budgets, line)
			continue
		}
		line.Budget = budget
		entries := collectGroup(ctx, targets, target.budget, refused, rows)
		removeOldest(ctx, entries, pass, &line, rows)
		budgets = append(budgets, line)
	}
	return budgets, rows
}

// refuseGroup refuses every store of a group whose budget did not parse.
func refuseGroup(targets []cleanTarget, group budgetGroup, reason string, rows []StoreTrim) {
	for index, target := range targets {
		if target.budget == group {
			rows[index].Refused = reason
		}
	}
}

// collectGroup walks every store of one group and answers their entries as
// one list, filling each row's size. A store reached twice, through a linked
// cache/ or any other alias, is compared by real path and walked once, so the
// union counts it once (AC-7).
//
// A store root is walked only when it is an absolute path to a real
// directory. A relative path, the shape the shared path takes when the
// per-user target cannot be resolved, is refused: walked, it would trim
// whatever directory le happened to run in. A root that is itself a symbolic
// link is refused too, so no link can lead the trim outside the store; a link
// in a PARENT, such as a linked cache/, is how the stores are laid out and is
// resolved rather than refused.
func collectGroup(ctx context.Context, targets []cleanTarget, group budgetGroup,
	refused map[string]string, rows []StoreTrim,
) []cacheEntry {
	seen := map[string]string{}
	var entries []cacheEntry
	for index, target := range targets {
		if target.budget != group {
			continue
		}
		if reason, ok := refused[target.name]; ok {
			rows[index].Refused = reason
			continue
		}
		if !filepath.IsAbs(target.path) {
			rows[index].Refused = "is not an absolute path, so the trim will not walk it"
			continue
		}
		info, err := os.Lstat(target.path)
		if errors.Is(err, fs.ErrNotExist) {
			rows[index].Skipped = absentStoreSkip
			continue
		}
		if err != nil {
			rows[index].Error = err.Error()
			continue
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			rows[index].Refused = "is a symbolic link; the trim walks only a real directory, so no link can lead it outside the store"
			continue
		}
		if !info.IsDir() {
			rows[index].Refused = "is not a directory"
			continue
		}
		real, err := filepath.EvalSymlinks(target.path)
		if err != nil {
			rows[index].Error = err.Error()
			continue
		}
		if first, ok := seen[real]; ok {
			rows[index].Skipped = "the same directory as the " + first + " cache, counted once"
			continue
		}
		seen[real] = target.name

		found, err := walkGoFormatCache(ctx, real, index)
		if err != nil {
			rows[index].Error = err.Error()
			continue
		}
		for _, entry := range found {
			rows[index].SizeBefore += entry.bytes
		}
		rows[index].SizeAfter = rows[index].SizeBefore
		entries = append(entries, found...)
	}
	return entries
}

// removeOldest removes entries oldest mtime first, across every store of the
// group, until their combined size is at or under the budget. A union exactly
// at the budget loses nothing. It stops at the first entry younger than the
// floor, because every entry after it in mtime order is younger still, and
// the budget line then says how many bytes the floor kept (AC-4).
//
// An entry that is already gone (ENOENT: a peer trim or Go's own trim got
// there first) counts as freed, not as removed by this run. A directory entry
// that gains a file while it is removed (ENOTEMPTY) is a live writer, which
// is contention and not failure, and the next trim retries it (AC-17).
func removeOldest(ctx context.Context, entries []cacheEntry, pass cachePass, line *BudgetTrim, rows []StoreTrim) {
	var total int64
	for _, entry := range entries {
		total += entry.bytes
	}
	line.SizeBefore = total
	line.SizeAfter = total
	if total <= line.Budget {
		return
	}

	slices.SortFunc(entries, compareEntryAge)
	contended := make([]int, len(rows))
	for position, entry := range entries {
		if total <= line.Budget {
			break
		}
		if pass.now.Sub(entry.modified) < pass.floor {
			line.Unmet = youngerThanFloor(entries[position:], pass.floor)
			break
		}
		if ctx.Err() != nil {
			line.Error = "the trim reached its " + storeTrimTimeout.String() + " limit and stopped"
			break
		}
		err := pass.remove(entry.path, entry.directory)
		switch {
		case err == nil:
			total -= entry.bytes
			rows[entry.store].SizeAfter -= entry.bytes
			rows[entry.store].EntriesRemoved++
			line.EntriesRemoved++
		case errors.Is(err, fs.ErrNotExist):
			total -= entry.bytes
			rows[entry.store].SizeAfter -= entry.bytes
		case isWriterRace(err):
			contended[entry.store]++
		default:
			if rows[entry.store].Error == "" {
				rows[entry.store].Error = err.Error()
			}
		}
	}
	line.SizeAfter = total
	for store, count := range contended {
		if count == 0 {
			continue
		}
		var text textbuf.Buffer
		rows[store].Contended = text.Int(int64(count)).Str(" directory entries gained files while being removed; the next trim retries them").String()
	}
}

// compareEntryAge orders entries oldest mtime first, and by path between
// equal mtimes so a run is reproducible.
func compareEntryAge(first, second cacheEntry) int {
	if order := first.modified.Compare(second.modified); order != 0 {
		return order
	}
	return strings.Compare(first.path, second.path)
}

// youngerThanFloor says how many bytes the floor kept, over entries that are
// all younger than it.
func youngerThanFloor(young []cacheEntry, floor time.Duration) string {
	var bytes int64
	for _, entry := range young {
		bytes += entry.bytes
	}
	var text textbuf.Buffer
	return text.Int(bytes).Str(" bytes (").Str(gibibytes(bytes)).Str(") are younger than the ").
		Str(floor.String()).Str(" floor, which no trim removes").String()
}

// walkGoFormatCache answers every -a and -d entry in the 256 two-hex-digit
// subdirectories of the cache at root, and nothing else. Nothing is followed
// through a symbolic link: a subdirectory that is a link is not walked, and an
// entry that is a link is measured and removed as the link itself.
//
// The answer holds one element per entry, which the filesystem bounds; at
// about a hundred bytes each, a cache of a million entries costs a hundred
// megabytes in the child for the length of its run.
func walkGoFormatCache(ctx context.Context, root string, store int) ([]cacheEntry, error) {
	var entries []cacheEntry
	for high := range len(hexDigits) {
		for low := range len(hexDigits) {
			if ctx.Err() != nil {
				return nil, errors.New("the trim reached its " + storeTrimTimeout.String() + " limit while walking " + root)
			}
			subdirectory := filepath.Join(root, string([]byte{hexDigits[high], hexDigits[low]}))
			found, err := walkCacheSubdirectory(subdirectory, store)
			if err != nil {
				return nil, err
			}
			entries = append(entries, found...)
		}
	}
	return entries, nil
}

// walkCacheSubdirectory answers the entries of one subdirectory. An absent
// one holds none, and a name that is not a real directory is not a cache
// subdirectory at all.
func walkCacheSubdirectory(subdirectory string, store int) ([]cacheEntry, error) {
	info, err := os.Lstat(subdirectory)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}
	listing, err := os.ReadDir(subdirectory)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var entries []cacheEntry
	for _, item := range listing {
		name := item.Name()
		if !isCacheEntryName(name) {
			continue
		}
		entryInfo, err := item.Info()
		if errors.Is(err, fs.ErrNotExist) {
			continue // a peer removed it between the listing and the stat
		}
		if err != nil {
			return nil, err
		}
		path := filepath.Join(subdirectory, name)
		entry := cacheEntry{path: path, modified: entryInfo.ModTime(), directory: entryInfo.IsDir(), store: store}
		if entry.directory {
			entry.bytes, err = treeBytes(path)
		} else {
			entry.bytes, err = allocatedBytes(entryInfo)
		}
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// isCacheEntryName answers whether a name is one Go's own trim removes.
func isCacheEntryName(name string) bool {
	if strings.HasSuffix(name, actionEntrySuffix) {
		return true
	}
	return strings.HasSuffix(name, outputEntrySuffix)
}

// treeBytes answers the allocated size of a directory entry and everything
// under it, without following a symbolic link. A file that vanishes during
// the walk counts as nothing.
func treeBytes(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, item fs.DirEntry, walkErr error) error {
		if errors.Is(walkErr, fs.ErrNotExist) {
			return nil
		}
		if walkErr != nil {
			return walkErr
		}
		info, err := item.Info()
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		size, err := allocatedBytes(info)
		if err != nil {
			return err
		}
		total += size
		return nil
	})
	return total, err
}

// allocatedBytes answers the space a file takes on its device. A filesystem
// that reports no block count is an error, never a size of zero, because a
// zero would let the store pass its budget unmeasured.
func allocatedBytes(info fs.FileInfo) (int64, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("%s: the filesystem reports no allocated size", info.Name())
	}
	return stat.Blocks * statBlockBytes, nil
}

// removeCacheEntry removes one entry: a file with Remove, an executable's
// directory with RemoveAll, which answers ENOTEMPTY when a writer adds a file
// while it runs.
func removeCacheEntry(path string, directory bool) error {
	if directory {
		return os.RemoveAll(path)
	}
	return os.Remove(path)
}
