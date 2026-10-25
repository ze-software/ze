// Design: docs/architecture/core-design.md -- native scratch-link gates
// Overview: scratch.go -- filesystem policy and implementation
// Related: cacheuse.go -- who may be using the caches, which a full empty refuses on
// Related: cachetrim.go -- the entry walk, the age floor and the removal both share
//
// This file cleans the five build caches a Ze checkout fills, and the stale
// copies the link repair left, and reports the disk space each one returned. By default it removes only the entries older
// than cacheEntryAgeFloor (3h), the floor the store trim keeps: other sessions
// build against these caches all the time, and an entry is safe for 2h after
// Go last looked it up, because Go refreshes an entry's mtime when it uses one
// but at most once an hour (cache.(*DiskCache).used), so the 3h floor leaves an
// hour of margin. An entry a build last looked up more than 2h ago can still be
// removed, and that build recompiles it on its next miss rather than failing.
// Emptying them outright under a running build failed it with
// "cache entry not found" or "no such file", a red that names no cause (the
// staticcheck matrix, 2026-10-21, plan/journal/gate-verdict-depends-on-the-machine.md).
// The full empty is the `all` keyword, and it refuses while any build may be
// using the caches (cacheuse.go). The check is a snapshot taken before the
// empty starts: a build that starts while the empty runs is not seen, and can
// still fail with "cache entry not found". The same list is the store list the store trim
// reads (storetrim.go): each target carries its kind and its budget, so the
// caches are declared once for both.
//
// The checkout's cache/ is a link to the per-user target, cacheTarget(), which
// every le run makes or repairs before it builds (EnsureCache), and which every
// verify worktree's cache/ links to as well. So there is one Go build cache,
// cache/go-cache, one golangci-lint cache, cache/golangci-lint-cache, and one
// Staticcheck matrix cache, gostaticcheck.CacheDir, each a row of its own, and
// no row names the per-user path a second time: before every run made the
// link, a checkout that never ran `le scratch links-ensure` filled a private
// cache/ beside the per-user one, and the per-user one was missed until
// 2026-10-05, at 13G.
//
// A bare `go` command typed outside le writes the ambient cache, `go env
// GOCACHE`, which is the machine default unless `go env -w` named the shared
// cache; its row is a skip when it resolves to the checkout's. The bootstrap
// cache, gotoolchain.BootstrapCache, is the `le` launcher's fallback when cache/
// cannot be created (a QEMU guest that sees the link dangle). The launcher can
// still write it, so it keeps a row: a cache nobody declares is one this action
// walks past, which is how 1.3G of it was found by hand on 2026-09-20.
//
// golangci-lint's cache is not a Go cache, so no `go clean` reaches it; it was
// measured at 9.5G on 2026-09-13. The Staticcheck matrix trims its cache only
// once a day and keeps five days of entries (honnef.co/go/tools/lintcmd/cache,
// DiskCache.Trim), so on 2026-10-23 it held 9.3G with no cleaner reaching it.
// Both layouts are Go's, written by copies of Go's cache code: an entry at
// "%02x"/<hex id>-a or -d, the only names their own trims remove.
//
// The last rows are the stale copies, one for each real cache/ directory the
// link repair renamed aside to cache.stale-<stamp>-<pid> rather than delete
// what a build might be reading. The store trim and cache-clean remove a copy
// whole once the stamp in its name is older than the age floor.
//
// The cost of not having this action is recorded in
// plan/journal/full-disk-false-red.md, one row for each time a full cache disk
// was read as a code defect. That file is the count, because a copy of it here
// goes stale on the next row. The checkout path can hide the disk, because a
// linked cache/ is a symlink onto another filesystem, so `df` on the checkout
// answers about the wrong device. The free space this action prints is read
// with statfs on the cache path itself.
//
// The patched GOROOT trees under cache/stdpatch (internal/le/go/stdpatch/gen)
// are not a cache this action empties: a build in flight compiles inside one.
// No target reaches them. The report names each key with its size, and drops
// only what no build can be using: a tree whose toolchain links dangle on this
// machine, and a staging directory older than stagingIdleAge.
package scratch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/appliance"
	"github.com/ze-software/ze/internal/core/diskspace"
	"github.com/ze-software/ze/internal/core/textbuf"
	gostaticcheck "github.com/ze-software/ze/internal/le/go/staticcheck"
	"github.com/ze-software/ze/internal/le/go/stdpatch/gen"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/job"
	"github.com/ze-software/ze/internal/le/procscan"
)

// cleanTimeout bounds one `go clean -cache` run. The measurement on
// 2026-08-20 emptied 256G in under a minute, so an hour is a stop for a run
// that has hung rather than a budget for a large cache.
const cleanTimeout = time.Hour

// goCacheKey is the variable that names the cache a `go` command uses.
const goCacheKey = "GOCACHE"

// The caches, named as a person reads them in the report.
const (
	checkoutCache    = "checkout"
	ambientCache     = "ambient"
	lintCache        = "lint"
	bootstrapCache   = "bootstrap"
	staticcheckCache = "staticcheck"
	// staleCacheCopy prefixes the name of each cache.stale-* copy the link
	// repair left; staleCopyName numbers one copy.
	staleCacheCopy = "stale"
	// stdpatchTrees names the patched GOROOT rows, which are kept, not emptied.
	stdpatchTrees = "stdpatch"
)

// cacheNameWidth is the report's name column: the longest name and one space,
// so no name runs into its path.
const cacheNameWidth = len(staticcheckCache) + 1

// bytesPerGiB converts a byte count to the unit the report prints.
const bytesPerGiB = 1 << 30

// bytesPerMiB converts a kept tree's byte count to the unit its row prints.
const bytesPerMiB = 1 << 20

// cleanMode is how much of each cache a run removes.
type cleanMode uint8

const (
	// unspecifiedClean is the zero value and never a valid mode.
	unspecifiedClean cleanMode = iota
	// cleanOldEntries removes the entries older than cacheEntryAgeFloor, which
	// no running build can be about to read. It is the default.
	cleanOldEntries
	// cleanEverything empties every cache, and runs only when no process is
	// seen that may be using one (cacheUsers). It is the `all` keyword.
	cleanEverything
)

// The names the report gives the two modes.
const (
	oldEntriesMode = "old-entries"
	everythingMode = "all"
)

// String names the mode as the report and the JSON answer print it.
func (mode cleanMode) String() string {
	switch mode {
	case cleanOldEntries:
		return oldEntriesMode
	case cleanEverything:
		return everythingMode
	case unspecifiedClean:
		return "unspecified"
	default:
		panic("BUG: cache-clean has an unknown mode")
	}
}

// CacheClean is what one run did to one cache. Freed is what the device
// returned, measured with statfs before and after. A run in the default mode
// also says what it removed and what the age floor kept; a full empty keeps
// nothing, so those stay zero there.
//
// Removed is the bytes of the entries past the floor that are gone after the
// run: the ones this run deleted, and any a concurrent trim (another session's
// store trim, or Go's own) deleted first, because both returned that space.
// EntriesRemoved counts only the entries this run deleted.
type CacheClean struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	FreeBefore     uint64 `json:"free-before"`
	FreeAfter      uint64 `json:"free-after"`
	Freed          int64  `json:"freed"`
	Removed        int64  `json:"removed,omitempty"`
	EntriesRemoved int    `json:"entries-removed,omitempty"`
	KeptYoung      int64  `json:"kept-young,omitempty"`
	Skipped        string `json:"skipped,omitempty"`
	Error          string `json:"error,omitempty"`

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

// CleanReport is the answer of one cache-clean run: its mode, one row for each
// cache, and one row for each patched GOROOT entry. A full empty that found a
// process that may be using the caches carries that process list and the
// refusal instead, and touched nothing.
type CleanReport struct {
	Mode    string        `json:"mode"`
	Refusal string        `json:"refusal,omitempty"`
	Users   []CacheUser   `json:"users,omitempty"`
	Caches  []CacheClean  `json:"caches"`
	Trees   []PatchedTree `json:"trees"`
}

// PatchedTree is one entry of a patched GOROOT directory (cache/stdpatch, or
// tmp/stdpatch where cache/ is not usable): a key, or a build's staging
// directory. A live key stays: another session's build may be compiling with
// GOROOT inside it, and a GOROOT removed under a running compile fails that
// build with "no such file" errors that never name the cause. A key is about
// 30 MiB and a stale key is never selected again, so its size is printed.
//
// Dropped says why the run removed it. Only two kinds go, because no build can
// be using either: a tree whose toolchain links name nothing on this machine
// (gen.DeadTree: the package manager replaced the toolchain in place), and a
// staging directory older than stagingIdleAge, which a killed build left
// (docs/contributing/patching-the-standard-library.md).
type PatchedTree struct {
	Path    string `json:"path"`
	Bytes   int64  `json:"bytes"`
	Dropped string `json:"dropped,omitempty"`
	Error   string `json:"error,omitempty"`
}

// stagingIdleAge is the age past which a staging directory belongs to no build.
// A build writes its staging directory, applies the patch and renames it into
// place in well under a minute (the `go list` of sameStdlib is its longest
// step), and the directory's mtime is set when the build starts, so three hours
// is a build that was killed, never one still running.
const stagingIdleAge = 3 * time.Hour

// The reasons a patched GOROOT entry is dropped.
const (
	deadTreeDrop = "its toolchain links name nothing on this machine, so no build can use it"
	stagingDrop  = "a staging directory older than the staging age, which a killed build left"
)

// Text renders one line for each cache. Freed and free are gibibytes, because
// the number a person acts on is whether the disk has room for a build.
func (r CleanReport) Text() string {
	var text textbuf.Buffer
	text.Reset()
	if r.Refusal != "" {
		text.Str("REFUSE   ").Str(r.Refusal).Byte('\n')
		for _, user := range r.Users {
			text.Str("  ").Str(user.line()).Byte('\n')
		}
		return text.String()
	}
	for _, cache := range r.Caches {
		var line textbuf.Buffer
		line.PadRight(cache.Name, cacheNameWidth)
		switch {
		case cache.Error != "":
			line.Str("REFUSE   ").Str(cache.Path).Str(": ").Str(cache.Error)
		case cache.Skipped != "":
			line.Str("SKIP     ").Str(cache.Path).Str(": ").Str(cache.Skipped)
		case cache.Contended != "":
			cache.measured(&line)
			line.Str(" (a concurrent build kept writing: ").Str(cache.Contended).Byte(')')
		default:
			cache.measured(&line)
		}
		text.Str(line.String()).Byte('\n')
	}
	for _, tree := range r.Trees {
		var line textbuf.Buffer
		line.PadRight(stdpatchTrees, cacheNameWidth)
		switch {
		case tree.Error != "" && tree.Dropped != "":
			line.Str("REFUSE   ").Str(tree.Path).Str(": ").Str(tree.Dropped).Str(", and removing it failed: ").Str(tree.Error).
				Str(". No build can use it, so fix the permission the error names, or delete the directory by hand,").
				Str(" then run `./le scratch cache-clean` again")
		case tree.Error != "":
			line.Str("KEEP     ").Str(tree.Path).Str(": its size is unknown: ").Str(tree.Error)
		case tree.Dropped != "":
			line.Str("DROP     ").PadRight(tree.Path, 48).Str(" ").Str(mebibytes(tree.Bytes)).Str(", ").Str(tree.Dropped)
		default:
			line.Str("KEEP     ").PadRight(tree.Path, 48).Str(" ").Str(mebibytes(tree.Bytes)).
				Str(", a patched GOROOT a running build may use")
		}
		text.Str(line.String()).Byte('\n')
	}
	return text.String()
}

// measured renders a cleaned cache's numbers: what the run removed and what
// the age floor kept when it ran in the default mode, then what the device
// returned and holds free.
func (cache CacheClean) measured(line *textbuf.Buffer) {
	line.PadRight(cache.Path, 48)
	if cache.Removed != 0 || cache.KeptYoung != 0 || cache.EntriesRemoved != 0 {
		line.Str(" removed ").Str(gibibytes(cache.Removed)).Str(" (").Int(int64(cache.EntriesRemoved)).
			Str(" entries), kept ").Str(gibibytes(cache.KeptYoung)).Str(" younger than ").Str(cacheEntryAgeFloor.String()).Byte(',')
	}
	line.Str(" freed ").Str(gibibytes(cache.Freed)).
		Str(", free ").Str(gibibytes(int64(cache.FreeAfter))) //nolint:gosec // a free-space count never exceeds the signed range on any device this runs on
}

// patchedTrees answers one row for each entry of every patched GOROOT
// directory under root, measured with the store trim's treeBytes (allocated
// size, symlinks not followed, so a link into the real GOROOT costs only
// itself). It removes a dead tree and an old staging directory, and nothing
// else. A tree directory that does not exist holds no key.
//
// Every key is judged, where a build judges only its own (gen.dropDeadTree).
// The build runs in a QEMU guest as well, where the host's links name nothing;
// this action is an operator's command on the machine whose cache it is, and
// a guest's tree directory is a tmpfs of its own (guestPatchedTree,
// internal/le/test/qemu/run.go), so "dead here" is the answer that matters.
func patchedTrees(root string, now time.Time) []PatchedTree {
	var trees []PatchedTree
	for _, dir := range gen.CacheDirs(root) {
		entries, err := os.ReadDir(dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			trees = append(trees, PatchedTree{Path: dir, Error: err.Error()})
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			trees = append(trees, judgePatchedTree(filepath.Join(dir, entry.Name()), entry, now))
		}
	}
	return trees
}

// judgePatchedTree measures one entry and removes it when no build can be
// using it.
func judgePatchedTree(path string, entry os.DirEntry, now time.Time) PatchedTree {
	tree := PatchedTree{Path: path}
	bytes, err := treeBytes(path)
	if err != nil {
		tree.Error = err.Error()
		return tree
	}
	tree.Bytes = bytes
	if strings.HasPrefix(entry.Name(), gen.StagingPrefix) {
		info, err := entry.Info()
		if err != nil {
			tree.Error = err.Error()
			return tree
		}
		if now.Sub(info.ModTime()) < stagingIdleAge {
			return tree
		}
		tree.Dropped = stagingDrop
	} else {
		if !gen.DeadTree(filepath.Join(path, "goroot")) {
			return tree
		}
		tree.Dropped = deadTreeDrop
	}
	if err := os.RemoveAll(path); err != nil {
		tree.Error = err.Error()
	}
	return tree
}

// CleanCaches cleans every build cache this checkout fills and reports what
// each one returned. The default mode removes the entries older than the age
// floor; cleanEverything empties every cache, and refuses before touching any
// when a process that may be using one is running (cacheUsers).
//
// The ambient cache is resolved by asking `go env GOCACHE` with the inherited
// GOCACHE removed, so the answer is the machine default rather than whatever
// le set for the calling process. A checkout whose GOCACHE already IS the
// machine default is cleaned once and the second row says so. The shared
// cache is resolved from this manager's environment (cacheTarget), the same
// answer a verify worktree's cache/ link reaches.
func (m *Manager) CleanCaches(mode cleanMode) (CleanReport, int) {
	return m.cleanCaches(mode, cleanPass{
		scan:    procscan.Scan,
		holders: job.Holders,
		self:    []int{os.Getpid(), os.Getppid()},
		entries: cachePass{now: time.Now(), floor: cacheEntryAgeFloor, remove: removeCacheEntry, run: cacheCleanRun},
	})
}

// cleanPass is what one cache-clean run reads besides the tree: the process
// scanner and the job registry a full empty is refused on, the pids that are
// this run itself, and the clock, floor and remover the default mode uses. A
// test replaces each one.
type cleanPass struct {
	scan    func() ([]procscan.Process, error)
	holders func(root string) ([]job.Holder, error)
	self    []int
	entries cachePass
}

// cleanCaches is CleanCaches with the pass a test replaces.
func (m *Manager) cleanCaches(mode cleanMode, pass cleanPass) (CleanReport, int) {
	report := CleanReport{Mode: mode.String()}
	switch mode {
	case cleanOldEntries:
	case cleanEverything:
		// The process table is read before this run starts any `go` of its
		// own, so the only processes of this run it can see are the pids in
		// pass.self.
		if refusal, users := m.refuseFullEmpty(pass); refusal != "" {
			report.Refusal = refusal
			report.Users = users
			return report, 1
		}
	case unspecifiedClean:
		panic("BUG: cache-clean was asked to run with no mode")
	default:
		panic("BUG: cache-clean has an unknown mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), cleanTimeout)
	defer cancel()

	// The ambient row is the only one whose path has to be resolved, so it is
	// the only one that can fail before anything is emptied. Its failure
	// refuses its own row and leaves the others.
	ambient, ambientErr := ambientGoCache(ctx)

	for _, target := range cleanTargets(m.Root, ambient) {
		switch {
		case target.name == ambientCache && ambientErr != nil:
			report.Caches = append(report.Caches, CacheClean{Name: target.name, Error: ambientErr.Error()})
		case target.failed != "":
			report.Caches = append(report.Caches, CacheClean{Name: target.name, Path: target.path, Error: target.failed})
		case target.skipped != "":
			report.Caches = append(report.Caches, CacheClean{
				Name: target.name, Path: target.path, Skipped: target.skipped,
			})
		case mode == cleanEverything:
			report.Caches = append(report.Caches, measureClean(target.name, target.path, target.emptier(ctx)))
		case target.kind == staleCopy:
			report.Caches = append(report.Caches, cleanStaleCopy(target, pass.entries))
		default:
			report.Caches = append(report.Caches, cleanOldCacheEntries(ctx, target, pass.entries))
		}
	}
	report.Trees = patchedTrees(m.Root, pass.entries.now)
	return report, report.verdict()
}

// cacheKind is the layout of a cache, which decides how cache-clean empties
// it. Both kinds hold Go's entry layout, 256 two-hex-digit subdirectories of
// `-a` and `-d` entries, which the store trim walks the same way for either.
type cacheKind uint8

const (
	// unspecifiedCache is the zero value and never a valid kind.
	unspecifiedCache cacheKind = iota
	// goBuildCache is a Go toolchain build cache, emptied with `go clean -cache`.
	goBuildCache
	// copiedFormatCache is another tool's copy of that layout, golangci-lint's
	// or Staticcheck's, which no `go clean` owns, so it is emptied by deleting it.
	copiedFormatCache
	// staleCopy is a whole cache/ directory the link repair renamed aside. It
	// holds caches of every kind, and is removed whole, never walked.
	staleCopy
)

// budgetGroup names the budget the store trim holds a cache to.
type budgetGroup uint8

const (
	// unspecifiedBudget is the zero value and never a valid group.
	unspecifiedBudget budgetGroup = iota
	// goCachesBudget is the ONE total across the checkout and bootstrap Go
	// caches, ze.le.store.go-cache-budget (owner decision D-1, 2026-10-05). A
	// cache in this group is never held to a budget of its own.
	goCachesBudget
	// lintCacheBudget is the lint cache's own budget,
	// ze.le.store.lint-cache-budget, never mixed with the Go total (D-2).
	lintCacheBudget
	// staticcheckCacheBudget is the Staticcheck matrix cache's budget,
	// ze.le.store.staticcheck-cache-budget, never mixed with the Go or lint
	// budget.
	staticcheckCacheBudget
	// pastFloor is held to no byte budget: the trim removes the copy whole once
	// the repair stamp in its name is older than the age floor, because nothing
	// builds through a renamed cache/ and only a build already running when
	// the repair moved it can still read it.
	pastFloor
	// notTrimmed is a cache the trim leaves alone. The ambient cache is the
	// machine's rather than le's, and Go's own five-day trim covers it (D-6).
	notTrimmed
)

// cleanTarget is one cache: how a person reads it in the report, where it
// lives, its kind, the budget the store trim holds it to, why this run skips
// it when it does, why it could not be judged at all when it could not, and
// whether it lies within or holds a kernel cache, which neither cache-clean nor
// the store trim may touch.
type cleanTarget struct {
	name        string
	path        string
	kind        cacheKind
	budget      budgetGroup
	skipped     string
	failed      string
	holdsKernel bool
}

// emptier answers what emptying this target takes, derived from its kind.
func (target cleanTarget) emptier(ctx context.Context) func() error {
	switch target.kind {
	case goBuildCache:
		return func() error { return goCleanCache(ctx, target.path) }
	case copiedFormatCache:
		return func() error { return removeCache(target.path) }
	case staleCopy:
		return func() error { return removeCache(target.path) }
	case unspecifiedCache:
		return func() error { return fmt.Errorf("BUG: cache %s was declared with no kind", target.name) }
	default:
		panic("BUG: cache target has an unknown kind")
	}
}

// heldKernelCache answers the first kernel cache namespace a row holds, judged
// as spelled (path) and where the cache/ link lands (landed), and whether it
// holds one.
func heldKernelCache(path, landed string) (string, bool) {
	for _, namespace := range appliance.KernelCacheNamespaces() {
		if holdsKernelCache(path, namespace) {
			return namespace, true
		}
		if holdsKernelCache(landed, namespace) {
			return namespace, true
		}
	}
	return "", false
}

// cleanTargets answers every cache a checkout at root fills, in report order.
// It is the one store list: cache-clean empties every row, and the store trim
// bounds the rows whose budget is not notTrimmed.
//
// It takes the ambient path rather than resolving it, so a caller that failed
// to resolve it still gets the other rows. The per-user target is no argument:
// the checkout's cache/ links there, so every row under cache/ already names it.
func cleanTargets(root, ambient string) []cleanTarget {
	checkout := gotoolchain.GoCache(root)
	ambientTarget := cleanTarget{name: ambientCache, path: ambient, kind: goBuildCache, budget: notTrimmed}
	// `go env -w GOCACHE` names the shared cache on a set-up host, and the
	// checkout's cache/go-cache reaches it through the link, so the two are
	// compared where they land on disk, never as spelled.
	same, err := SameDirectory(ambient, checkout)
	if err != nil {
		// Unjudged, the ambient row could be the checkout cache cleaned twice or
		// a cache nobody cleans, so it is a failed row and nothing touches it.
		ambientTarget.failed = "cannot tell whether the machine default is the checkout cache: " + err.Error()
	}
	if same {
		ambientTarget.skipped = "the machine default is the checkout cache, which this run already cleaned"
	}

	fixed := []cleanTarget{
		{name: checkoutCache, path: checkout, kind: goBuildCache, budget: goCachesBudget},
		ambientTarget,
		{name: bootstrapCache, path: gotoolchain.BootstrapCache(root), kind: goBuildCache, budget: goCachesBudget},
		{name: lintCache, path: gotoolchain.LintCache(root), kind: copiedFormatCache, budget: lintCacheBudget},
		{name: staticcheckCache, path: gostaticcheck.CacheDir(root), kind: copiedFormatCache, budget: staticcheckCacheBudget},
	}
	copies, err := staleCacheCopies(root)
	targets := make([]cleanTarget, 0, len(fixed)+len(copies)+1)
	targets = append(targets, fixed...)
	// A checkout whose listing failed holds an unknown number of copies, so it
	// answers one failed row rather than none: no row would read as "no stale
	// copy", a zero standing in for an answer.
	if err != nil {
		targets = append(targets, cleanTarget{
			name: staleCacheCopy, path: root, kind: staleCopy, budget: pastFloor, failed: err.Error(),
		})
	}
	for index, copied := range copies {
		targets = append(targets, cleanTarget{name: staleCopyName(index), path: copied, kind: staleCopy, budget: pastFloor})
	}
	// A built kernel takes about thirty minutes to rebuild, on a host of its
	// own arch, so no clean or trim reaches into a kernel cache, or empties a
	// directory that holds one, whatever path a target resolved to. Each row is
	// judged as spelled and where the cache/ link lands (gen.Landing), because
	// every row under cache/ reaches the per-user target, beside the kernels.
	for index := range targets {
		// A failed row names the checkout it could not list, which is no cache,
		// and nothing touches it, so the guard has nothing to judge there.
		if targets[index].failed != "" {
			continue
		}
		namespace, held := heldKernelCache(targets[index].path, gen.Landing(root, targets[index].path))
		if held {
			targets[index].holdsKernel = true
			targets[index].skipped = "it holds the kernel cache " + namespace +
				", and a built kernel takes about thirty minutes to rebuild on a host of its own arch"
		}
	}
	return targets
}

// staleCacheCopies answers each cache.stale-* directory at root, in name order,
// which is repair order. It lists root and compares names rather than globbing,
// because root is the operator's path: a checkout under a directory named with
// `[`, `*` or `?` would make a glob pattern malformed or match other names. A
// root it cannot list is an error, never an empty answer.
func staleCacheCopies(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("cannot list %s for %s%s* copies: %w", root, cacheName, staleCacheInfix, err)
	}
	var copies []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if !strings.HasPrefix(entry.Name(), cacheName+staleCacheInfix) {
			continue
		}
		copies = append(copies, filepath.Join(root, entry.Name()))
	}
	return copies, nil
}

// staleCopyName names the stale copy at position index of staleCacheCopies,
// counted from 1: stale-1 is the oldest repair. Every row of a report names one
// store, so two copies need two names, and a number keeps the name inside the
// report's name column, where the stamp would not fit; the row's path carries
// the stamp.
func staleCopyName(index int) string {
	return staleCacheCopy + "-" + strconv.Itoa(index+1)
}

// staleCopyAge answers how long ago the link repair renamed the copy at path
// aside, read from the stamp in its name, which a rename leaves as it was. The
// directory's own times describe the last build that wrote it, not the repair.
func staleCopyAge(path string, now time.Time) (time.Duration, error) {
	stamp, found := strings.CutPrefix(filepath.Base(path), cacheName+staleCacheInfix)
	if !found {
		return 0, fmt.Errorf("%s is not named %s%s<stamp>, so its repair time is unknown", path, cacheName, staleCacheInfix)
	}
	if len(stamp) < len(staleCacheStamp) {
		return 0, fmt.Errorf("%s carries no %s repair stamp, so its age is unknown; remove it by hand once no build reads it", path, staleCacheStamp)
	}
	repaired, err := time.Parse(staleCacheStamp, stamp[:len(staleCacheStamp)])
	if err != nil {
		return 0, fmt.Errorf("%s carries no %s repair stamp (%w), so its age is unknown; remove it by hand once no build reads it", path, staleCacheStamp, err)
	}
	return now.Sub(repaired), nil
}

// holdsKernelCache answers whether path lies within the kernel cache namespace
// or holds it.
func holdsKernelCache(path, namespace string) bool {
	if pathWithin(path, namespace) {
		return true
	}
	return pathWithin(namespace, path)
}

// pathWithin reports whether path is dir or below it.
func pathWithin(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
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
	if r.Refusal != "" {
		return 1
	}
	for _, cache := range r.Caches {
		if cache.Error != "" {
			return 1
		}
	}
	for _, tree := range r.Trees {
		if tree.Dropped == "" {
			continue
		}
		if tree.Error != "" {
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

// cleanOldCacheEntries removes every entry of one cache older than the age
// floor, oldest first, with the store trim's walk and removal, and measures
// the device before and after. It is the store trim with a budget of zero: the
// floor alone decides, so an entry a running build may be about to read stays
// (cacheEntryAgeFloor). A cache that does not exist holds nothing to clean.
func cleanOldCacheEntries(ctx context.Context, target cleanTarget, pass cachePass) CacheClean {
	cache := CacheClean{Name: target.name, Path: target.path}
	if _, err := os.Lstat(target.path); errors.Is(err, os.ErrNotExist) {
		cache.Skipped = absentStoreSkip
		return cache
	}
	before, err := diskspace.Free(target.path)
	if err != nil {
		cache.Error = err.Error()
		return cache
	}
	cache.FreeBefore = before
	entries, err := walkGoFormatCache(ctx, target.path, 0, pass.run)
	if err != nil {
		cache.Error = err.Error()
		return cache
	}
	for _, entry := range entries {
		if pass.now.Sub(entry.modified) < pass.floor {
			cache.KeptYoung += entry.bytes
		}
	}

	line := BudgetTrim{}
	rows := []StoreTrim{{Name: target.name, Path: target.path}}
	removeOldest(ctx, entries, pass, &line, rows)
	cache.Removed = line.SizeBefore - line.SizeAfter
	cache.EntriesRemoved = line.EntriesRemoved
	cache.Contended = rows[0].Contended
	after, err := diskspace.Free(target.path)
	if err != nil {
		cache.Error = err.Error()
		return cache
	}
	cache.FreeAfter = after
	cache.Freed = int64(after) - int64(before) //nolint:gosec // a free-space count never exceeds the signed range on any device this runs on
	if line.Error != "" {
		cache.Error = line.Error
		return cache
	}
	cache.Error = rows[0].Error
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

// mebibytes renders a kept tree's size, one decimal, in MiB.
func mebibytes(bytes int64) string {
	var text textbuf.Buffer
	return text.Float(float64(bytes)/bytesPerMiB, 1).Str("M").String()
}

// cleanStaleCopy is cache-clean's default mode for a stale cache copy: the
// store trim's rule, so a copy past the age floor is removed whole and a
// younger one is kept and reported as kept young. The free space is read on
// the directory holding the copy, which outlives its removal.
func cleanStaleCopy(target cleanTarget, pass cachePass) CacheClean {
	cache := CacheClean{Name: target.name, Path: target.path}
	holder := filepath.Dir(target.path)
	before, err := diskspace.Free(holder)
	if err != nil {
		cache.Error = err.Error()
		return cache
	}
	cache.FreeBefore = before
	row := StoreTrim{Name: target.name, Path: target.path}
	trimStaleCopy(target, pass, &row)
	if row.Error != "" {
		cache.Error = row.Error
		return cache
	}
	cache.Removed = row.SizeBefore - row.SizeAfter
	cache.EntriesRemoved = row.EntriesRemoved
	if row.Kept != 0 {
		cache.KeptYoung = row.SizeBefore
	}
	after, err := diskspace.Free(holder)
	if err != nil {
		cache.Error = err.Error()
		return cache
	}
	cache.FreeAfter = after
	cache.Freed = int64(after) - int64(before) //nolint:gosec // a free-space count never exceeds the signed range on any device this runs on
	return cache
}
