// VALIDATES: the automatic store trim is due at most once an hour, is spawned by
// exactly one of many concurrent le invocations, and obeys ze.le.store.trim.
// PREVENTS: every hook call walking the caches, or twenty sessions each
// starting a trim of the same stores.
package scratch

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
)

// contenderRootKey names the variable that turns this test binary into one of
// the contending child processes TestStoreTrimDueOncePerHour starts.
const contenderRootKey = "ZE_TEST_STORE_TRIM_CONTENDER_ROOT"

// contenders is how many le invocations AC-3 starts together.
const contenders = 20

// recordSpawn answers a spawn seam that appends one line to <root>/spawns for
// each call, with O_APPEND so lines from many processes never interleave.
func recordSpawn(root string) TrimSpawn {
	return func(string) error {
		file, err := os.OpenFile(filepath.Join(root, "spawns"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
		if err != nil {
			return err
		}
		defer file.Close() //nolint:errcheck // the write below owns the verdict
		_, err = file.WriteString("spawn " + strconv.Itoa(os.Getpid()) + "\n")
		return err
	}
}

func spawnCount(t *testing.T, root string) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "spawns"))
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatalf("read spawns: %v", err)
	}
	return strings.Count(string(data), "\n")
}

func writeStampAt(t *testing.T, root string, stamped time.Time) {
	t.Helper()
	path := filepath.Join(root, storeTrimDir, stampName)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strconv.FormatInt(stamped.Unix(), 10)), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readStamp(t *testing.T, root string) int64 {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, storeTrimDir, stampName))
	if err != nil {
		t.Fatalf("read stamp: %v", err)
	}
	seconds, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		t.Fatalf("stamp %q is not unix seconds: %v", data, err)
	}
	return seconds
}

// TestStoreTrimDueOncePerHour proves the hourly throttle (AC-2) and that many
// concurrent le invocations spawn exactly one trim (AC-3).
//
// Method: in-process cases pin the stamp-age boundaries (59m59s fresh, 60m
// due, a stamp more than an hour in the future due, R-11) and the lock held
// by a peer. The contention case re-executes this test binary twenty times;
// every child blocks on its stdin until the parent closes all twenty pipes, so
// they reach the due check together, and each runs the real check against one
// shared root with a spawn seam that appends a line. Exactly one line, and a
// stamp that parses, is the pass.
func TestStoreTrimDueOncePerHour(t *testing.T) {
	if root := os.Getenv(contenderRootKey); root != "" {
		if _, err := io.ReadAll(os.Stdin); err != nil {
			t.Fatalf("contender barrier: %v", err)
		}
		startTrimWhenDue(root, time.Now(), "", recordSpawn(root))
		return
	}

	now := time.Unix(1_800_000_000, 0)

	t.Run("absent stamp is due and written", func(t *testing.T) {
		root := t.TempDir()
		if got := startTrimWhenDue(root, now, "", recordSpawn(root)); got != TriggerSpawned {
			t.Fatalf("trigger = %v, want TriggerSpawned", got)
		}
		if got := spawnCount(t, root); got != 1 {
			t.Fatalf("spawns = %d, want 1", got)
		}
		if got := readStamp(t, root); got != now.Unix() {
			t.Fatalf("stamp = %d, want %d", got, now.Unix())
		}
	})

	ages := []struct {
		name string
		age  time.Duration
		want Trigger
	}{
		{"59m59s old is fresh", 59*time.Minute + 59*time.Second, TriggerFresh},
		{"60m old is due", time.Hour, TriggerSpawned},
		{"just written is fresh", 0, TriggerFresh},
		{"one hour ahead is fresh", -time.Hour, TriggerFresh},
		{"more than one hour ahead is due", -time.Hour - time.Second, TriggerSpawned},
	}
	for _, tc := range ages {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeStampAt(t, root, now.Add(-tc.age))
			if got := startTrimWhenDue(root, now, "", recordSpawn(root)); got != tc.want {
				t.Fatalf("trigger = %v, want %v", got, tc.want)
			}
			wantSpawns := 0
			if tc.want == TriggerSpawned {
				wantSpawns = 1
			}
			if got := spawnCount(t, root); got != wantSpawns {
				t.Fatalf("spawns = %d, want %d", got, wantSpawns)
			}
		})
	}

	t.Run("a peer holding the lock means no spawn", func(t *testing.T) {
		root := t.TempDir()
		lockPath := filepath.Join(root, storeTrimDir, stampLockName)
		if err := os.MkdirAll(filepath.Dir(lockPath), 0o750); err != nil {
			t.Fatal(err)
		}
		held, err := os.OpenFile(lockPath, os.O_RDONLY|os.O_CREATE, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		defer held.Close() //nolint:errcheck // closing releases the lock
		if err := syscall.Flock(int(held.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			t.Fatal(err)
		}
		if got := startTrimWhenDue(root, now, "", recordSpawn(root)); got != TriggerBusy {
			t.Fatalf("trigger = %v, want TriggerBusy", got)
		}
		if got := spawnCount(t, root); got != 0 {
			t.Fatalf("spawns = %d, want 0", got)
		}
	})

	t.Run("twenty contenders spawn once", func(t *testing.T) {
		root := t.TempDir()
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		commands := make([]*exec.Cmd, 0, contenders)
		barriers := make([]io.WriteCloser, 0, contenders)
		outputs := make([]*bytes.Buffer, 0, contenders)
		for range contenders {
			command := exec.CommandContext(t.Context(), executable, "-test.run=^TestStoreTrimDueOncePerHour$", "-test.count=1")
			command.Env = append(os.Environ(), contenderRootKey+"="+root)
			output := &bytes.Buffer{}
			command.Stdout = output
			command.Stderr = output
			barrier, pipeErr := command.StdinPipe()
			if pipeErr != nil {
				t.Fatal(pipeErr)
			}
			if startErr := command.Start(); startErr != nil {
				t.Fatal(startErr)
			}
			commands = append(commands, command)
			barriers = append(barriers, barrier)
			outputs = append(outputs, output)
		}
		for _, barrier := range barriers {
			if closeErr := barrier.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
		}
		for index, command := range commands {
			if waitErr := command.Wait(); waitErr != nil {
				t.Fatalf("contender %d: %v\n%s", index, waitErr, outputs[index])
			}
		}
		if got := spawnCount(t, root); got != 1 {
			t.Fatalf("twenty contenders spawned %d trims, want exactly 1", got)
		}
		readStamp(t, root)
	})
}

// TestStoreTrimSwitch proves ze.le.store.trim=off stops the automatic trim
// (AC-15) and that a value other than on or off is refused rather than read
// as either one.
func TestStoreTrimSwitch(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	cases := []struct {
		value string
		want  Trigger
	}{
		{"", TriggerSpawned},
		{"on", TriggerSpawned},
		{"off", TriggerOff},
		{"maybe", TriggerRefused},
	}
	for _, tc := range cases {
		t.Run("value "+tc.value, func(t *testing.T) {
			root := t.TempDir()
			if got := startTrimWhenDue(root, now, tc.value, recordSpawn(root)); got != tc.want {
				t.Fatalf("trigger = %v, want %v", got, tc.want)
			}
			wantSpawns := 0
			if tc.want == TriggerSpawned {
				wantSpawns = 1
			}
			if got := spawnCount(t, root); got != wantSpawns {
				t.Fatalf("spawns = %d, want %d", got, wantSpawns)
			}
		})
	}

	t.Run("a refused switch is logged once an hour, not once a call", func(t *testing.T) {
		root := t.TempDir()
		for range 3 {
			startTrimWhenDue(root, now, "maybe", recordSpawn(root))
		}
		data, err := os.ReadFile(filepath.Join(root, storeTrimDir, trimLogName))
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Count(string(data), "\n"); got != 1 {
			t.Fatalf("last.log holds %d lines, want 1:\n%s", got, data)
		}
		if !strings.Contains(string(data), StoreTrimKey+`="maybe"`) {
			t.Fatalf("last.log does not name the key and value:\n%s", data)
		}
	})
}

// VALIDATES: AC-13 and AC-14 with the Boundary Tests rows. A budget is a whole
// number with an M or G suffix, from 1M to 1048576G, and every other spelling
// is refused with an error naming the key and the value.
// PREVENTS: a trim deleting under a guessed limit because a mistyped budget
// fell back to the default, or a zero budget emptying a cache.
func TestBudgetParse(t *testing.T) {
	accepted := []struct {
		value string
		bytes int64
	}{
		{"1M", 1 << 20},
		{"500M", 500 << 20},
		{"2G", 2 << 30},
		{"40G", 40 << 30},
		{"10G", 10 << 30},
		{"1048576G", 1 << 50},
		{"1073741824M", 1 << 50},
	}
	for _, tc := range accepted {
		got, err := parseBudget(goCacheBudgetKey, tc.value)
		if err != nil {
			t.Errorf("%s=%q refused: %v", goCacheBudgetKey, tc.value, err)
			continue
		}
		if got != tc.bytes {
			t.Errorf("%s=%q = %d bytes, want %d", goCacheBudgetKey, tc.value, got, tc.bytes)
		}
	}

	refused := []string{
		"", "abc", "5", "5X", "5g", "5m", "0M", "0G", "-5G", "+5G", "1.5G", " 5G", "5G ",
		"5GG", "G", "1048577G", "1073741825M", "99999999999999999999G",
	}
	for _, value := range refused {
		got, err := parseBudget(lintCacheBudgetKey, value)
		if err == nil {
			t.Errorf("%s=%q accepted as %d bytes, want a refusal", lintCacheBudgetKey, value, got)
			continue
		}
		if !strings.Contains(err.Error(), lintCacheBudgetKey) || !strings.Contains(err.Error(), strconv.Quote(value)) {
			t.Errorf("%s=%q refusal does not name the key and the value: %v", lintCacheBudgetKey, value, err)
		}
	}

	for _, entry := range []struct{ key, value string }{
		{goCacheBudgetEntry.Key, goCacheBudgetEntry.Default},
		{lintCacheBudgetEntry.Key, lintCacheBudgetEntry.Default},
	} {
		if _, err := parseBudget(entry.key, entry.value); err != nil {
			t.Errorf("the registered default %s=%q does not parse: %v", entry.key, entry.value, err)
		}
	}
	if got, want := goCacheBudgetEntry.Default, "40G"; got != want {
		t.Errorf("%s default = %q, want %q (owner decision D-1)", goCacheBudgetKey, got, want)
	}
	if got, want := lintCacheBudgetEntry.Default, "10G"; got != want {
		t.Errorf("%s default = %q, want %q (owner decision D-2)", lintCacheBudgetKey, got, want)
	}
	if !goCacheBudgetEntry.Private || !lintCacheBudgetEntry.Private {
		t.Error("a store budget is a build-host knob and must stay out of `ze env list`")
	}
}

// entryBytes is the size every seeded cache entry writes: a whole number of
// 4096-byte blocks, so its allocated size is the same on APFS and ext4.
const entryBytes = 64 << 10

// trimNow is the clock every cache trim test runs at.
var trimNow = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

// seedEntry writes one cache entry, cache/<first two letters of name>/name,
// with an mtime age before trimNow, and answers its path.
func seedEntry(t *testing.T, cache, name string, age time.Duration) string {
	t.Helper()
	path := filepath.Join(cache, name[:2], name)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte{'x'}, entryBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	stamp := trimNow.Add(-age)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	return path
}

// seededUnit answers the allocated size of one seeded entry, as the trim
// measures it. TestTrimGoCacheOldestFirst holds that measure against du.
func seededUnit(t *testing.T) int64 {
	t.Helper()
	path := seedEntry(t, t.TempDir(), "00unit-a", time.Hour)
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	size, err := allocatedBytes(info)
	if err != nil {
		t.Fatal(err)
	}
	return size
}

func exists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Lstat(path)
	if err == nil {
		return true
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	t.Fatalf("lstat %s: %v", path, err)
	return false
}

// budgetsOf answers a budget seam holding fixed byte counts, zero included,
// which no env value can spell (AC-18).
func budgetsOf(goBytes, lintBytes int64) func(budgetGroup) (int64, error) {
	return func(group budgetGroup) (int64, error) {
		switch group {
		case goCachesBudget:
			return goBytes, nil
		case lintCacheBudget:
			return lintBytes, nil
		case notTrimmed, unspecifiedBudget:
			return 0, errors.New("no budget for this group")
		default:
			panic("BUG: cache trim requested an unknown budget group")
		}
	}
}

// testPass is the clock, floor and remover every cache trim test uses unless
// it says otherwise.
func testPass(floor time.Duration) cachePass {
	return cachePass{now: trimNow, floor: floor, remove: removeCacheEntry}
}

// checkoutOnly answers the store list of a checkout whose shared and
// bootstrap caches do not exist, so the checkout cache alone is the Go union.
func checkoutOnly(t *testing.T) (root string, targets []cleanTarget) {
	t.Helper()
	root = t.TempDir()
	return root, cleanTargets(root, "", t.TempDir())
}

func rowNamed(t *testing.T, rows []StoreTrim, name string) StoreTrim {
	t.Helper()
	for _, row := range rows {
		if row.Name == name {
			return row
		}
	}
	t.Fatalf("no %s row in %+v", name, rows)
	return StoreTrim{}
}

func budgetKeyed(t *testing.T, budgets []BudgetTrim, key string) BudgetTrim {
	t.Helper()
	for _, budget := range budgets {
		if budget.Key == key {
			return budget
		}
	}
	t.Fatalf("no %s budget in %+v", key, budgets)
	return BudgetTrim{}
}

// TestTrimGoCacheOldestFirst proves a Go cache over its total loses its
// oldest entries first until it is at or under the total, that only -a and -d
// entries in the two-hex-digit subdirectories are touched (AC-1, AC-5), and
// the two boundary rows: exactly at the total removes nothing, one byte over
// removes the single oldest entry. It also holds the allocated-size measure
// against du (A-6).
//
// Method: five entries aged 10h down to 6h, all older than the 3h floor, plus
// README, trim.txt, a foreign file at the top, a suffixless file in a hex
// subdirectory and an -a file in a non-hex subdirectory. Budgets of five,
// five-minus-one-byte and three entries are run on fresh copies.
func TestTrimGoCacheOldestFirst(t *testing.T) {
	unit := seededUnit(t)

	seed := func(t *testing.T) ([]cleanTarget, []string, []string) {
		_, targets := checkoutOnly(t)
		cache := targets[0].path
		var entries []string
		for index, name := range []string{"a1e1-a", "b2e2-d", "c3e3-a", "d4e4-d", "e5e5-a"} {
			entries = append(entries, seedEntry(t, cache, name, time.Duration(10-index)*time.Hour))
		}
		foreign := []string{
			filepath.Join(cache, "README"), filepath.Join(cache, "trim.txt"), filepath.Join(cache, "notes-a"),
			filepath.Join(cache, "a1", "orphan"), filepath.Join(cache, "zz", "zzzz-a"),
		}
		for _, path := range foreign {
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
				t.Fatal(err)
			}
			old := trimNow.Add(-100 * time.Hour)
			if err := os.Chtimes(path, old, old); err != nil {
				t.Fatal(err)
			}
		}
		return targets, entries, foreign
	}

	cases := []struct {
		name    string
		budget  int64
		removed int
	}{
		{"exactly at the total", 5 * unit, 0},
		{"one byte over", 5*unit - 1, 1},
		{"two entries over", 3 * unit, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			targets, entries, foreign := seed(t)
			budgets, rows := trimCaches(t.Context(), targets, nil, budgetsOf(tc.budget, 1<<40), testPass(cacheEntryAgeFloor))
			for index, path := range entries {
				if got, want := exists(t, path), index >= tc.removed; got != want {
					t.Errorf("entry %d (%s) exists=%v, want %v: the oldest %d go first", index, filepath.Base(path), got, want, tc.removed)
				}
			}
			for _, path := range foreign {
				if !exists(t, path) {
					t.Errorf("%s was removed; only -a/-d entries in hex subdirectories are cache entries", path)
				}
			}
			row := rowNamed(t, rows, checkoutCache)
			if row.EntriesRemoved != tc.removed || row.SizeBefore != 5*unit || row.SizeAfter != int64(5-tc.removed)*unit {
				t.Errorf("checkout row = %+v, want %d removed, %d -> %d bytes", row, tc.removed, 5*unit, int64(5-tc.removed)*unit)
			}
			budget := budgetKeyed(t, budgets, goCacheBudgetKey)
			if budget.SizeAfter > tc.budget || budget.EntriesRemoved != tc.removed || budget.Unmet != "" {
				t.Errorf("go budget line = %+v, want at or under %d with %d removed", budget, tc.budget, tc.removed)
			}
		})
	}

	t.Run("allocated size matches du", func(t *testing.T) {
		path := seedEntry(t, t.TempDir(), "f0du-a", time.Hour)
		output, err := exec.CommandContext(t.Context(), "du", "-k", path).Output()
		if err != nil {
			t.Fatalf("du: %v", err)
		}
		kibibytes, err := strconv.ParseInt(strings.Fields(string(output))[0], 10, 64)
		if err != nil {
			t.Fatalf("du output %q: %v", output, err)
		}
		if got := kibibytes * 1024; got != unit {
			t.Errorf("du says %d bytes allocated, the trim measures %d", got, unit)
		}
	})
}

// TestTrimGoCacheRespectsAgeFloor proves no entry younger than the floor is
// removed, whatever the budget, and that the budget line then says the budget
// is unmet and how many bytes are younger than the floor (AC-4).
//
// Method: one entry older than the floor and two younger, against a budget of
// one entry. The old one goes; the two young ones stay and are reported.
func TestTrimGoCacheRespectsAgeFloor(t *testing.T) {
	unit := seededUnit(t)
	_, targets := checkoutOnly(t)
	cache := targets[0].path
	old := seedEntry(t, cache, "a0old-a", 5*time.Hour)
	young := []string{
		seedEntry(t, cache, "b0young-a", 2*time.Hour),
		seedEntry(t, cache, "c0young-d", 2*time.Hour+59*time.Minute),
	}

	budgets, rows := trimCaches(t.Context(), targets, nil, budgetsOf(unit, 1<<40), testPass(cacheEntryAgeFloor))
	if exists(t, old) {
		t.Error("the entry older than the floor was kept although the cache is over budget")
	}
	for _, path := range young {
		if !exists(t, path) {
			t.Errorf("%s is younger than the %v floor and was removed", filepath.Base(path), cacheEntryAgeFloor)
		}
	}
	budget := budgetKeyed(t, budgets, goCacheBudgetKey)
	want := strconv.FormatInt(2*unit, 10) + " bytes"
	if !strings.Contains(budget.Unmet, want) || !strings.Contains(budget.Unmet, "younger") {
		t.Errorf("budget unmet = %q, want it to name %s younger than the floor", budget.Unmet, want)
	}
	if row := rowNamed(t, rows, checkoutCache); row.EntriesRemoved != 1 || row.Refused != "" || row.Error != "" {
		t.Errorf("checkout row = %+v, want one removal and no refusal", row)
	}
	if code := (trimReport{Budgets: budgets, Stores: rows}).verdict(); code != 0 {
		t.Errorf("an unmet budget exited %d; it is reported, not a failure", code)
	}
}

// TestTrimLintCache proves the lint cache is trimmed against its own budget by
// the same rules, apart from the Go total in both directions (AC-6).
//
// Method: two entries in the checkout Go cache, three in the lint cache, one
// of them younger than the floor. With the Go budget exactly at the Go size
// and the lint budget at two entries, only the oldest lint entry goes. With the
// Go budget at one entry and the lint budget at its size, only the oldest Go
// entry goes.
func TestTrimLintCache(t *testing.T) {
	unit := seededUnit(t)
	cases := []struct {
		name                   string
		goBudget, lintBudget   int64
		goRemoved, lintRemoved bool
	}{
		{"lint over, go under", 2 * unit, 2 * unit, false, true},
		{"go over, lint under", unit, 3 * unit, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, targets := checkoutOnly(t)
			goOld := seedEntry(t, gotoolchain.GoCache(root), "a0go-a", 8*time.Hour)
			goNew := seedEntry(t, gotoolchain.GoCache(root), "a1go-a", 7*time.Hour)
			lintOld := seedEntry(t, gotoolchain.LintCache(root), "b0lint-a", 9*time.Hour)
			lintNew := seedEntry(t, gotoolchain.LintCache(root), "b1lint-d", 6*time.Hour)
			lintYoung := seedEntry(t, gotoolchain.LintCache(root), "b2lint-a", time.Hour)

			budgets, _ := trimCaches(t.Context(), targets, nil, budgetsOf(tc.goBudget, tc.lintBudget), testPass(cacheEntryAgeFloor))
			if got := !exists(t, goOld); got != tc.goRemoved {
				t.Errorf("oldest go entry removed=%v, want %v", got, tc.goRemoved)
			}
			if got := !exists(t, lintOld); got != tc.lintRemoved {
				t.Errorf("oldest lint entry removed=%v, want %v", got, tc.lintRemoved)
			}
			if !exists(t, goNew) || !exists(t, lintNew) || !exists(t, lintYoung) {
				t.Error("a second entry went: each pass stops at its own budget")
			}
			if got := budgetKeyed(t, budgets, lintCacheBudgetKey).SizeBefore; got != 3*unit {
				t.Errorf("lint budget line counts %d bytes, want %d: Go entries never count toward it", got, 3*unit)
			}
			if got := budgetKeyed(t, budgets, goCacheBudgetKey).SizeBefore; got != 2*unit {
				t.Errorf("go budget line counts %d bytes, want %d: lint entries never count toward it", got, 2*unit)
			}
		})
	}
}

// TestTrimGoCachesAsOneUnion proves the checkout, shared and bootstrap caches
// are trimmed as ONE union against one total, in one global mtime order
// (AC-1, AC-7, AC-21), that a cache reached twice through a link is counted
// once, and that a store root the trim must not walk is refused by name.
//
// Method: AC-21 scaled down, 20G/15G/10G against 40G becoming 4, 3 and 2
// entries against 8: each cache alone is under the total, together they are
// one over, and the single oldest entry, in the bootstrap cache, goes. Then a
// budget of 6 with the mtimes interleaved (oldest in bootstrap, next in shared,
// next in checkout) removes exactly one entry from each. Then cache/ is linked
// to the per-user directory and the shared row says it is the checkout cache.
// Last, a relative shared path and a symlinked bootstrap root are refused.
func TestTrimGoCachesAsOneUnion(t *testing.T) {
	unit := seededUnit(t)

	seed := func(t *testing.T) ([]cleanTarget, map[string]string) {
		root, perUser := t.TempDir(), t.TempDir()
		shared := filepath.Join(perUser, "go-cache")
		oldest := map[string]string{
			bootstrapCache: seedEntry(t, gotoolchain.BootstrapCache(root), "b0boot-a", 30*time.Hour),
			sharedCache:    seedEntry(t, shared, "a0shared-a", 29*time.Hour),
			checkoutCache:  seedEntry(t, gotoolchain.GoCache(root), "c0check-a", 28*time.Hour),
		}
		for index := range 3 {
			seedEntry(t, gotoolchain.GoCache(root), "c"+strconv.Itoa(index+1)+"check-a", time.Duration(10+index)*time.Hour)
		}
		for index := range 2 {
			seedEntry(t, shared, "a"+strconv.Itoa(index+1)+"shared-d", time.Duration(10+index)*time.Hour)
		}
		seedEntry(t, gotoolchain.BootstrapCache(root), "b1boot-a", 10*time.Hour)
		return cleanTargets(root, "", perUser), oldest
	}

	t.Run("each under, union over", func(t *testing.T) {
		targets, oldest := seed(t)
		budgets, rows := trimCaches(t.Context(), targets, nil, budgetsOf(8*unit, 1<<40), testPass(cacheEntryAgeFloor))
		if exists(t, oldest[bootstrapCache]) {
			t.Error("the globally oldest entry, in the bootstrap cache, was kept: a per-cache reading removes nothing here")
		}
		if !exists(t, oldest[sharedCache]) || !exists(t, oldest[checkoutCache]) {
			t.Error("more than the one entry the union was over went")
		}
		budget := budgetKeyed(t, budgets, goCacheBudgetKey)
		if budget.SizeBefore != 9*unit || budget.SizeAfter != 8*unit {
			t.Errorf("go budget line %d -> %d, want %d -> %d", budget.SizeBefore, budget.SizeAfter, 9*unit, 8*unit)
		}
		if got := rowNamed(t, rows, bootstrapCache).EntriesRemoved; got != 1 {
			t.Errorf("bootstrap row removed %d, want 1 (R-12: each cache reports what it lost)", got)
		}
	})

	t.Run("one global mtime order", func(t *testing.T) {
		targets, oldest := seed(t)
		_, rows := trimCaches(t.Context(), targets, nil, budgetsOf(6*unit, 1<<40), testPass(cacheEntryAgeFloor))
		for name, path := range oldest {
			if exists(t, path) {
				t.Errorf("the %s cache's oldest entry was kept: removals follow one order across the three", name)
			}
			if got := rowNamed(t, rows, name).EntriesRemoved; got != 1 {
				t.Errorf("%s row removed %d, want 1", name, got)
			}
		}
	})

	t.Run("a linked checkout counts the shared cache once", func(t *testing.T) {
		root, perUser := t.TempDir(), t.TempDir()
		if err := os.Symlink(perUser, filepath.Join(root, "cache")); err != nil {
			t.Fatal(err)
		}
		seedEntry(t, gotoolchain.GoCache(root), "c0once-a", 10*time.Hour)
		seedEntry(t, gotoolchain.GoCache(root), "c1once-a", 9*time.Hour)
		budgets, rows := trimCaches(t.Context(), cleanTargets(root, "", perUser), nil, budgetsOf(1<<40, 1<<40), testPass(cacheEntryAgeFloor))
		if got := budgetKeyed(t, budgets, goCacheBudgetKey).SizeBefore; got != 2*unit {
			t.Errorf("the union counts %d bytes, want %d: one cache reached twice is counted once", got, 2*unit)
		}
		shared := rowNamed(t, rows, sharedCache)
		if !strings.Contains(shared.Skipped, checkoutCache) || shared.SizeBefore != 0 {
			t.Errorf("shared row = %+v, want it skipped as the same cache as the checkout's", shared)
		}
	})

	t.Run("a store root the trim must not walk", func(t *testing.T) {
		root, elsewhere := t.TempDir(), t.TempDir()
		victim := seedEntry(t, elsewhere, "d0victim-a", 50*time.Hour)
		if err := os.MkdirAll(filepath.Dir(gotoolchain.BootstrapCache(root)), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(elsewhere, gotoolchain.BootstrapCache(root)); err != nil {
			t.Fatal(err)
		}
		// perUser "" is what trimStores holds when cacheTarget fails, which
		// makes the shared path the relative "go-cache".
		_, rows := trimCaches(t.Context(), cleanTargets(root, "", ""), nil, budgetsOf(0, 0), testPass(0))
		if !exists(t, victim) {
			t.Error("an entry behind a symlinked store root was removed")
		}
		if got := rowNamed(t, rows, bootstrapCache).Refused; !strings.Contains(got, "symbolic link") {
			t.Errorf("bootstrap row refusal = %q, want it to name the symbolic link", got)
		}
		if got := rowNamed(t, rows, sharedCache).Refused; !strings.Contains(got, "absolute") {
			t.Errorf("shared row refusal = %q, want a relative path refused", got)
		}
	})
}

// TestTrimWriterRace proves an -d directory entry that gains a file while it is
// removed is contention rather than failure, that an entry a peer already
// removed is not a failure, and that the run continues past both (AC-17).
//
// Method: a remover seam answers ENOTEMPTY for one directory entry and ENOENT
// for another, the two errors a concurrent writer and a concurrent trim
// produce; every other entry, a real -d directory included, is removed by the
// production remover.
func TestTrimWriterRace(t *testing.T) {
	_, targets := checkoutOnly(t)
	cache := targets[0].path
	raced := filepath.Join(cache, "a0", "a0race-d")
	if err := os.MkdirAll(raced, 0o750); err != nil {
		t.Fatal(err)
	}
	vanished := seedEntry(t, cache, "b0gone-a", 9*time.Hour)
	executable := filepath.Join(cache, "c0", "c0exe-d")
	if err := os.MkdirAll(executable, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(executable, "a.out"), bytes.Repeat([]byte{'x'}, entryBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{raced, executable} {
		old := trimNow.Add(-10 * time.Hour)
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}
	kept := seedEntry(t, cache, "d0kept-a", 5*time.Hour)
	// ENOTEMPTY leaves the directory charged. Budget its actual allocation
	// together with the newest file, including filesystems with directory blocks.
	var retainedBytes int64
	for _, path := range []string{raced, kept} {
		size, err := treeBytes(path)
		if err != nil {
			t.Fatal(err)
		}
		retainedBytes += size
	}

	pass := testPass(cacheEntryAgeFloor)
	// The trim walks the store's real path, which on macOS puts /private in
	// front of a temporary directory, so the seam matches on the entry name.
	pass.remove = func(path string, directory bool) error {
		switch filepath.Join(cache, filepath.Base(filepath.Dir(path)), filepath.Base(path)) {
		case raced:
			return &os.PathError{Op: "unlinkat", Path: path, Err: syscall.ENOTEMPTY}
		case vanished:
			return &os.PathError{Op: "remove", Path: path, Err: syscall.ENOENT}
		}
		return removeCacheEntry(path, directory)
	}
	budgets, rows := trimCaches(t.Context(), targets, nil, budgetsOf(retainedBytes, 1<<40), pass)
	row := rowNamed(t, rows, checkoutCache)
	if row.Contended == "" || row.Error != "" || row.Refused != "" {
		t.Errorf("checkout row = %+v, want contention recorded and no error", row)
	}
	if exists(t, executable) {
		t.Error("the -d directory entry after the raced one was not removed: the run must continue")
	}
	if !exists(t, kept) {
		t.Error("the newest entry went although the budget was met without it")
	}
	if !exists(t, raced) {
		t.Error("the contended directory was removed")
	}
	if row.SizeAfter != retainedBytes || row.EntriesRemoved != 1 {
		t.Errorf("checkout accounting = %+v, want %d retained bytes and one removed entry; ENOENT is freed, not removed",
			row, retainedBytes)
	}
	if code := (trimReport{Budgets: budgets, Stores: rows}).verdict(); code != 0 {
		t.Errorf("contention exited %d, want 0", code)
	}
}

// TestTrimmedEntryIsAMiss proves a trimmed entry is a cache miss for the go
// command's build and vet (AC-18, A-3), and that a cache hit refreshes an
// entry's mtime once it is an hour old, which makes oldest-first
// least-recently-used (A-1).
//
// Method: a package with no imports is compiled into a temporary GOCACHE, every
// entry is aged two hours, and the compile is repeated: a hit refreshes at
// least one entry to now. The cache is then trimmed with a zero budget and a
// zero floor through the test seam, and both verbs over the same package must
// exit 0.
func TestTrimmedEntryIsAMiss(t *testing.T) {
	module := t.TempDir()
	if err := os.WriteFile(filepath.Join(module, "go.mod"), []byte("module example.com/trimmed\n\ngo 1.21\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(module, "p.go"), []byte("package p\n\n// F answers one.\nfunc F() int { return 1 }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(t.TempDir(), "go-cache")
	run := func(verb string) {
		t.Helper()
		command := exec.CommandContext(t.Context(), "go", verb, "./...")
		command.Dir = module
		command.Env = append(os.Environ(), "GOCACHE="+cache, "GOFLAGS=", "GOWORK=off", "GOTOOLCHAIN=local")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("go %s against the trimmed cache: %v\n%s", verb, err, output)
		}
	}
	entries := func() []string {
		t.Helper()
		found, err := filepath.Glob(filepath.Join(cache, "??", "*-[ad]"))
		if err != nil {
			t.Fatal(err)
		}
		return found
	}
	compile := "bu" + "ild"

	run(compile)
	aged := time.Now().Add(-2 * time.Hour)
	for _, path := range entries() {
		if err := os.Chtimes(path, aged, aged); err != nil {
			t.Fatal(err)
		}
	}
	run(compile)
	refreshed := 0
	for _, path := range entries() {
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.ModTime().After(aged.Add(time.Hour)) {
			refreshed++
		}
	}
	if refreshed == 0 {
		t.Error("a cache hit refreshed no entry aged two hours: oldest mtime first would not be least recently used")
	}

	targets := []cleanTarget{{name: checkoutCache, path: cache, kind: goBuildCache, budget: goCachesBudget}}
	pass := cachePass{now: time.Now(), floor: 0, remove: removeCacheEntry}
	_, rows := trimCaches(t.Context(), targets, nil, budgetsOf(0, 0), pass)
	// A zero budget is met once the entries left allocate nothing: Go writes
	// empty outputs, which take no block, and the budget counts blocks.
	for _, path := range entries() {
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		if size, err := allocatedBytes(info); err != nil || size != 0 {
			t.Errorf("a zero budget with a zero floor kept %s, %d bytes (%v)", path, size, err)
		}
	}
	if rows[0].EntriesRemoved == 0 || rows[0].SizeAfter != 0 {
		t.Fatalf("a zero budget with a zero floor removed nothing (row %+v)", rows[0])
	}
	run(compile)
	run("vet")
}

// TestTrimReportJSON proves the report's shape and verdict (AC-16, AC-13): one
// row per store with kebab-case keys, a bad budget refusing exactly its own
// stores by key and value while the others proceed, and exit 1 only for a
// refusal, an unreadable switch included.
//
// Method: a budget seam refuses the Go group and keeps a lint budget, then the
// report is rendered as JSON and as text. trimStores itself runs with an
// unreadable ze.le.store.trim against an empty checkout whose per-user target
// is a temporary directory.
func TestTrimReportJSON(t *testing.T) {
	root, targets := checkoutOnly(t)
	goEntry := seedEntry(t, gotoolchain.GoCache(root), "a0json-a", 10*time.Hour)
	lintEntry := seedEntry(t, gotoolchain.LintCache(root), "b0json-a", 10*time.Hour)
	refusal := errors.New(goCacheBudgetKey + `="abc" is not a size`)
	budgetOf := func(group budgetGroup) (int64, error) {
		if group == goCachesBudget {
			return 0, refusal
		}
		return 0, nil
	}
	budgets, rows := trimCaches(t.Context(), targets, nil, budgetOf, testPass(cacheEntryAgeFloor))
	report := trimReport{Automatic: storeTrimOn, Budgets: budgets, Stores: rows}

	if !exists(t, goEntry) {
		t.Error("an entry was removed from a store whose budget was refused")
	}
	if exists(t, lintEntry) {
		t.Error("the lint store did not proceed beside a refused Go budget")
	}
	if len(rows) != len(targets) {
		t.Errorf("%d rows for %d stores, want one each", len(rows), len(targets))
	}
	for _, name := range []string{checkoutCache, sharedCache, bootstrapCache} {
		if got := rowNamed(t, rows, name).Refused; !strings.Contains(got, goCacheBudgetKey) || !strings.Contains(got, `"abc"`) {
			t.Errorf("%s row refusal = %q, want the key and the value", name, got)
		}
	}
	if code := report.verdict(); code != 1 {
		t.Errorf("a refused budget exited %d, want 1", code)
	}

	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"size-before"`, `"size-after"`, `"entries-removed"`, `"refused"`, `"budgets"`, `"stores"`, `"key"`} {
		if !bytes.Contains(encoded, []byte(key)) {
			t.Errorf("JSON lacks %s: %s", key, encoded)
		}
	}
	for _, field := range []string{"SizeBefore", "EntriesRemoved", "sizeBefore"} {
		if bytes.Contains(encoded, []byte(field)) {
			t.Errorf("JSON carries the non-kebab key %s: %s", field, encoded)
		}
	}
	text := report.Text()
	for _, name := range []string{checkoutCache, sharedCache, ambientCache, bootstrapCache, lintCache, goCacheBudgetKey} {
		if !strings.Contains(text, name) {
			t.Errorf("text report does not name %s:\n%s", name, text)
		}
	}

	t.Run("an unreadable switch is refused and the stores still trim", func(t *testing.T) {
		t.Setenv("XDG_CACHE_HOME", t.TempDir())
		root := t.TempDir()
		report, code := trimStores(root, "maybe")
		if code != 1 || !strings.Contains(report.Refusal, StoreTrimKey) {
			t.Errorf("trimStores(maybe) = %d, refusal %q; want 1 naming the key", code, report.Refusal)
		}
		if len(report.Stores) != len(cleanTargets(root, "", t.TempDir()))+len(livenessRows(root)) {
			t.Errorf("trimStores(maybe) reported %d stores; the switch governs only the automatic trim", len(report.Stores))
		}
		// The ambient row is a skip, and still names the cache it leaves
		// alone: an empty path would read as an answer.
		if ambient := rowNamed(t, report.Stores, ambientCache); ambient.Path == "" || ambient.Skipped == "" {
			t.Errorf("ambient row = %+v, want its resolved path and its skip reason", ambient)
		}
	})
}
