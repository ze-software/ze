// Design: docs/contributing/running-commands.md -- "When the disk is full", the store trim
// Overview: scratch.go -- filesystem policy and implementation
// Related: cacheclean.go -- the store list the trim shares with cache-clean
//
// This file bounds the build stores le and its sessions fill. Every le
// invocation asks StartStoreTrimWhenDue whether a trim is due, which costs one
// read of a ten-byte stamp file; at most once an hour one invocation wins a
// lock, writes the stamp and starts `le scratch store-trim background` as a
// detached child, so no caller waits for a walk of a cache that can hold tens
// of gigabytes. The child is a process rather than a goroutine because the
// invoking le is usually a hook call that exits in milliseconds and would take
// a goroutine down mid-walk (ai/rules/goroutine-lifecycle.md).
package scratch

import (
	"context"
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

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/session"
)

// StoreTrimKey switches the automatic trim: `on` (the default) or `off`. A
// trim asked for by name still runs when it is off.
const StoreTrimKey = "ze.le.store.trim"

// envTypeString is the env registry type of every store-trim setting.
const envTypeString = "string"

var storeTrimEntry = env.MustRegister(env.EnvEntry{
	Key:         StoreTrimKey,
	Type:        envTypeString,
	Default:     storeTrimOn,
	Description: "whether le trims its build stores automatically, at most once an hour: on or off",
	// Private keeps the key out of `ze env list`. It is a build-host knob and
	// an operator has nothing to do with it.
	Private: true,
})

const (
	storeTrimOn  = "on"
	storeTrimOff = "off"
)

// The two store budgets. Each is a whole count of mebibytes or gibibytes with
// its unit, M or G, from 1M to 1048576G (parseBudget).
const (
	// goCacheBudgetKey is ONE total across the checkout, shared and bootstrap
	// Go caches, never a budget for each (owner decision D-1, 2026-10-05).
	goCacheBudgetKey = "ze.le.store.go-cache-budget"
	// lintCacheBudgetKey is the golangci-lint cache's own budget (D-2).
	lintCacheBudgetKey = "ze.le.store.lint-cache-budget"
)

var goCacheBudgetEntry = env.MustRegister(env.EnvEntry{
	Key:         goCacheBudgetKey,
	Type:        envTypeString,
	Default:     "40G",
	Description: "the one total the checkout, shared and bootstrap Go build caches are trimmed to, oldest entry first: a whole number with M or G",
	// Private for the reason storeTrimEntry gives.
	Private: true,
})

var lintCacheBudgetEntry = env.MustRegister(env.EnvEntry{
	Key:         lintCacheBudgetKey,
	Type:        envTypeString,
	Default:     "10G",
	Description: "the size the golangci-lint cache is trimmed to, oldest entry first, apart from the Go total: a whole number with M or G",
	// Private for the reason storeTrimEntry gives.
	Private: true,
})

// budgetEntry answers the env entry that holds a group's budget, and false
// for a group the trim does not bound.
func budgetEntry(group budgetGroup) (env.EnvEntry, bool) {
	switch group {
	case goCachesBudget:
		return goCacheBudgetEntry, true
	case lintCacheBudget:
		return lintCacheBudgetEntry, true
	case notTrimmed, unspecifiedBudget:
	}
	return env.EnvEntry{}, false
}

// readBudget reads a group's budget from the environment. env.Get answers ""
// for an unset key, and only then does the registered default apply; any
// other value is parsed strictly, so a bad one refuses the group by name.
func readBudget(group budgetGroup) (int64, error) {
	entry, ok := budgetEntry(group)
	if !ok {
		return 0, errors.New("no budget bounds this store")
	}
	value := env.Get(entry.Key)
	if value == "" {
		value = entry.Default
	}
	return parseBudget(entry.Key, value)
}

// The units a budget accepts, and the largest count each takes so the byte
// count stays inside the 1048576G ceiling and far from overflowing int64.
const (
	mebibyte       = int64(1) << 20
	gibibyte       = int64(1) << 30
	budgetBytesMax = int64(1) << 50 // 1048576G
	budgetUnitMebi = 'M'
	budgetUnitGibi = 'G'
)

// parseBudget reads a store budget in bytes. It accepts only a positive whole
// number followed by M or G, from 1M to 1048576G. Every other spelling, a bare
// number included, is refused with an error naming the key and the value: a
// budget the trim cannot read refuses that store, and the trim MUST NOT fall
// back to the default and delete under a limit nobody set.
func parseBudget(key, value string) (int64, error) {
	refuse := func(why string) (int64, error) {
		return 0, fmt.Errorf("%s=%q %s; write a whole number with M or G, from 1M to 1048576G", key, value, why)
	}
	if len(value) < 2 {
		return refuse("is not a size")
	}
	digits, unit := value[:len(value)-1], value[len(value)-1]
	var unitBytes int64
	switch unit {
	case budgetUnitMebi:
		unitBytes = mebibyte
	case budgetUnitGibi:
		unitBytes = gibibyte
	default: // any byte may arrive here: the set of spellings is open
		return refuse("has no M or G unit")
	}
	// ParseInt alone accepts a sign and leading zeros, so the first byte is
	// checked here: a sign, a zero, a space and a dot all sort below '1'. A
	// byte above '9' is not a digit, which ParseInt refuses below.
	if digits[0] < '1' {
		return refuse("does not start with a digit from 1 to 9")
	}
	count, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return refuse("is not a whole number")
	}
	if count > budgetBytesMax/unitBytes {
		return refuse("is above the largest budget")
	}
	return count * unitBytes, nil
}

// The files the trigger and the child share, under <root>/tmp/store-trim.
const (
	storeTrimDir  = "tmp/store-trim"
	stampName     = "stamp"
	stampLockName = "stamp.lock"
	runLockName   = "run.lock"
	trimLogName   = "last.log"
)

// trimInterval is how long a written stamp holds the next trim off.
const trimInterval = time.Hour

// Trigger is what one due check did. Only tests branch on it: the caller in
// le's root handler MUST NOT print it or let it change an exit code, and a
// failure is written to tmp/store-trim/last.log instead.
type Trigger uint8

const (
	// TriggerUnspecified is never answered.
	TriggerUnspecified Trigger = iota
	// TriggerFresh means the stamp is under an hour old, so nothing was done.
	TriggerFresh
	// TriggerOff means ze.le.store.trim is off.
	TriggerOff
	// TriggerRefused means ze.le.store.trim holds neither on nor off. No trim
	// starts under a switch nobody can read.
	TriggerRefused
	// TriggerBusy means another invocation holds the stamp lock and is
	// deciding for everyone.
	TriggerBusy
	// TriggerSpawned means this invocation wrote the stamp and started the
	// child.
	TriggerSpawned
	// TriggerFailed means the lock, the stamp or the spawn failed; last.log
	// holds why.
	TriggerFailed
)

// String names the outcome for a test failure message.
func (t Trigger) String() string {
	switch t {
	case TriggerUnspecified:
		return "unspecified"
	case TriggerFresh:
		return "fresh"
	case TriggerOff:
		return "off"
	case TriggerRefused:
		return "refused"
	case TriggerBusy:
		return "busy"
	case TriggerSpawned:
		return "spawned"
	case TriggerFailed:
		return "failed"
	}
	return "Trigger(" + strconv.Itoa(int(t)) + ")"
}

// TrimSpawn starts the background trim for the checkout at root. It MUST
// return without waiting for the trim to finish.
type TrimSpawn func(root string) error

// StartStoreTrimWhenDue starts the background trim when the hourly stamp is
// due, and answers what it did. It never prints and never waits for the trim:
// le's root handler calls it before every command, hooks included.
//
// A checkout that cannot be resolved answers TriggerFailed with nowhere to
// write why; the command le was asked for then reports the same failure itself.
func StartStoreTrimWhenDue(spawn TrimSpawn) Trigger {
	root, err := checkoutRoot()
	if err != nil {
		return TriggerFailed
	}
	return startTrimWhenDue(root, time.Now(), env.Get(storeTrimEntry.Key), spawn)
}

// startTrimWhenDue is the due check over an explicit root, clock and switch
// value. Off answers before any file is read. The stamp is read once without
// the lock, because that answers almost every call; only a due stamp takes the
// lock, and under it the stamp is read again, because a peer may have written
// it between the two reads.
func startTrimWhenDue(root string, now time.Time, automatic string, spawn TrimSpawn) Trigger {
	enabled, switchErr := parseStoreTrim(automatic)
	if switchErr == nil && !enabled {
		return TriggerOff
	}
	stampPath := filepath.Join(root, storeTrimDir, stampName)
	if stampFresh(stampPath, now) {
		return TriggerFresh
	}

	if err := os.MkdirAll(filepath.Join(root, storeTrimDir), 0o750); err != nil {
		return TriggerFailed
	}
	lock, err := os.OpenFile(filepath.Join(root, storeTrimDir, stampLockName), os.O_RDONLY|os.O_CREATE, 0o600) //nolint:gosec // a fixed name under the resolved checkout's tmp/store-trim
	if err != nil {
		appendTrimLog(root, now, "open the stamp lock: "+err.Error())
		return TriggerFailed
	}
	defer lock.Close() //nolint:errcheck // closing releases the lock; the stamp write owns the verdict
	held, err := tryLock(lock)
	if err != nil {
		appendTrimLog(root, now, "lock the stamp: "+err.Error())
		return TriggerFailed
	}
	if !held {
		return TriggerBusy
	}

	if stampFresh(stampPath, now) {
		return TriggerFresh
	}
	// The stamp is written before the spawn, so a spawn that fails, or a
	// switch nobody can read, is retried and logged once an hour rather than
	// by every le call until it is fixed.
	if err := writeStamp(stampPath, now); err != nil {
		appendTrimLog(root, now, "write the stamp: "+err.Error())
		return TriggerFailed
	}
	if switchErr != nil {
		appendTrimLog(root, now, switchErr.Error())
		return TriggerRefused
	}
	if err := spawn(root); err != nil {
		appendTrimLog(root, now, "start the background trim: "+err.Error())
		return TriggerFailed
	}
	return TriggerSpawned
}

// parseStoreTrim reads ze.le.store.trim. Unset is on, the registered default.
func parseStoreTrim(value string) (bool, error) {
	if value == "" {
		return true, nil
	}
	if value == storeTrimOn {
		return true, nil
	}
	if value == storeTrimOff {
		return false, nil
	}
	return false, fmt.Errorf("%s=%q is neither %s nor %s, so the automatic store trim does not run", StoreTrimKey, value, storeTrimOn, storeTrimOff)
}

// stampFresh reports whether the stamp holds off the next trim: written under
// an hour ago, or at most an hour ahead of the clock. A stamp further ahead
// means the clock moved back, and is due, as Go's own trim.txt check treats it
// (R-11). A missing or unreadable stamp is due; the lock then serializes the
// invocations that see it.
func stampFresh(path string, now time.Time) bool {
	data, err := os.ReadFile(path) //nolint:gosec // a fixed name under the resolved checkout's tmp/store-trim
	if err != nil {
		return false
	}
	seconds, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return false
	}
	age := now.Sub(time.Unix(seconds, 0))
	if age >= trimInterval {
		return false
	}
	return age >= -trimInterval
}

// writeStamp replaces the stamp through a rename, so an unlocked reader sees
// the old value or the new one and never a partial write.
func writeStamp(path string, now time.Time) error {
	temporary := path + ".new." + strconv.Itoa(os.Getpid())
	if err := os.WriteFile(temporary, strconv.AppendInt(nil, now.Unix(), 10), 0o600); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		os.Remove(temporary) //nolint:errcheck // the rename error is the one reported
		return err
	}
	return nil
}

// tryLock takes an exclusive advisory lock without waiting. It answers false
// when another open file holds it.
func tryLock(file *os.File) (bool, error) {
	err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return false, nil
	}
	if errors.Is(err, syscall.EAGAIN) {
		return false, nil
	}
	return false, err
}

// appendTrimLog records a trigger failure in last.log, the one place the
// trigger may write, because it MUST NOT print into the command it rides on.
func appendTrimLog(root string, now time.Time, message string) {
	if err := os.MkdirAll(filepath.Join(root, storeTrimDir), 0o750); err != nil {
		return
	}
	file, err := os.OpenFile(filepath.Join(root, storeTrimDir, trimLogName), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600) //nolint:gosec // a fixed name under the resolved checkout's tmp/store-trim
	if err != nil {
		return
	}
	defer file.Close() //nolint:errcheck // a log line nobody can be told about failing
	var line textbuf.Buffer
	line.Str(now.UTC().Format(time.RFC3339)).Str(" trigger: ").Str(message).Byte('\n')
	file.WriteString(line.String()) //nolint:errcheck // the trigger has no channel to report a failed log write on
}

// DetachedTrim answers the spawn le's root handler uses: it starts this same
// executable as `<leading...> scratch store-trim background` in its own
// session, with stdin from the null device and stdout and stderr into
// tmp/store-trim/last.log, and does not wait. leading is empty for the le
// binary and `le` for a ze binary carrying le, whose argv names the root
// handler first.
//
// last.log is truncated at each spawn, so it holds the latest run. A child
// from an earlier spawn still running appends to the same file and the new
// child refuses on run.lock, so the file stays bounded by one run's output.
// The started process is released rather than waited for; a long-lived
// parent therefore holds a zombie entry until it exits, which costs a pid and
// nothing else.
func DetachedTrim(leading []string) TrimSpawn {
	return func(root string) error {
		executable, err := os.Executable()
		if err != nil {
			return fmt.Errorf("find this executable: %w", err)
		}
		logFile, err := os.OpenFile(filepath.Join(root, storeTrimDir, trimLogName), os.O_WRONLY|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0o600) //nolint:gosec // a fixed name under the resolved checkout's tmp/store-trim
		if err != nil {
			return err
		}
		defer logFile.Close() //nolint:errcheck // the child holds its own descriptor
		null, err := os.Open(os.DevNull)
		if err != nil {
			return err
		}
		defer null.Close() //nolint:errcheck // the child holds its own descriptor

		args := append(slices.Clone(leading), area, storeTrimVerb, backgroundKeyword)
		child := exec.CommandContext(context.Background(), executable, args...) //nolint:gosec // this same executable, with a fixed argv
		child.Dir = root
		child.Env = append(env.Without(os.Environ(), lepath.RootKey), "ZE_REPO_ROOT="+root)
		child.Stdin = null
		child.Stdout = logFile
		child.Stderr = logFile
		child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := child.Start(); err != nil {
			return err
		}
		return child.Process.Release()
	}
}

// The words of the action, shared by its registration and the spawn.
const (
	storeTrimVerb     = "store-trim"
	backgroundKeyword = "background"
)

// trimReport is the answer of one store trim: whether the automatic trim is
// on, one line for each budget, and one row for each store.
type trimReport struct {
	Automatic string       `json:"automatic"`
	Refusal   string       `json:"refusal,omitempty"`
	Budgets   []BudgetTrim `json:"budgets"`
	Stores    []StoreTrim  `json:"stores"`
}

// BudgetTrim is what one pass did against one budget. The three Go caches
// share one line, because their budget is one total over their union (owner
// decision D-1); the lint cache has its own.
type BudgetTrim struct {
	Key            string `json:"key"`
	Budget         int64  `json:"budget"`
	SizeBefore     int64  `json:"size-before"`
	SizeAfter      int64  `json:"size-after"`
	EntriesRemoved int    `json:"entries-removed"`
	Unmet          string `json:"unmet,omitempty"`
	Refused        string `json:"refused,omitempty"`
	Error          string `json:"error,omitempty"`
}

// StoreTrim is what one trim did to one store.
type StoreTrim struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	SizeBefore     int64  `json:"size-before"`
	SizeAfter      int64  `json:"size-after"`
	EntriesRemoved int    `json:"entries-removed"`
	Kept           int    `json:"kept,omitempty"`
	Skipped        string `json:"skipped,omitempty"`
	Refused        string `json:"refused,omitempty"`
	Contended      string `json:"contended,omitempty"`
	Error          string `json:"error,omitempty"`
}

// verdict answers 1 when the switch, a budget or a store was refused, or a
// store failed, so a caller sees it. An unmet budget and a contended store
// are reported and exit 0: neither is something the operator got wrong, and
// the next trim retries both.
func (r trimReport) verdict() int {
	if r.Refusal != "" {
		return 1
	}
	for _, budget := range r.Budgets {
		if budget.Refused != "" {
			return 1
		}
		if budget.Error != "" {
			return 1
		}
	}
	for _, store := range r.Stores {
		if store.Refused != "" {
			return 1
		}
		if store.Error != "" {
			return 1
		}
	}
	return 0
}

// Text renders the switch state, then one line for each budget, then one
// line for each store.
func (r trimReport) Text() string {
	var text textbuf.Buffer
	text.Reset()
	switch {
	case r.Refusal != "":
		text.Str("automatic trim: REFUSE ").Str(r.Refusal).Str("; this trim ran because it was asked for\n")
	case r.Automatic == storeTrimOff:
		text.Str("automatic trim: off (").Str(StoreTrimKey).Str("=off); this trim ran because it was asked for\n")
	default:
		text.Str("automatic trim: on, at most once an hour\n")
	}
	for _, budget := range r.Budgets {
		var line textbuf.Buffer
		line.Str("budget ").Str(budget.Key)
		switch {
		case budget.Refused != "":
			line.Str(" REFUSE ").Str(budget.Refused)
		default:
			line.Byte('=').Str(gibibytes(budget.Budget)).Str(": ").Str(gibibytes(budget.SizeBefore)).Str(" -> ").
				Str(gibibytes(budget.SizeAfter)).Str(", ").Int(int64(budget.EntriesRemoved)).Str(" removed")
		}
		if budget.Unmet != "" {
			line.Str("; budget unmet: ").Str(budget.Unmet)
		}
		if budget.Error != "" {
			line.Str("; ERROR ").Str(budget.Error)
		}
		text.Str(line.String()).Byte('\n')
	}
	for _, store := range r.Stores {
		var line textbuf.Buffer
		line.PadRight(store.Name, cacheNameWidth)
		switch {
		case store.Refused != "":
			line.Str("REFUSE   ").Str(store.Path).Str(": ").Str(store.Refused)
		case store.Skipped != "":
			line.Str("SKIP     ").Str(store.Path).Str(": ").Str(store.Skipped)
		default:
			line.PadRight(store.Path, 48).Str(" ").Str(gibibytes(store.SizeBefore)).Str(" -> ").
				Str(gibibytes(store.SizeAfter)).Str(", ").Int(int64(store.EntriesRemoved)).Str(" removed")
			if store.Kept != 0 {
				line.Str(", ").Int(int64(store.Kept)).Str(" kept")
			}
			if store.Contended != "" {
				line.Str(" (a concurrent build kept writing: ").Str(store.Contended).Byte(')')
			}
			if store.Error != "" {
				line.Str("; ERROR ").Str(store.Error)
			}
		}
		text.Str(line.String()).Byte('\n')
	}
	return text.String()
}

// runStoreTrim is `le scratch store-trim`: the trim in the foreground, which
// ignores the hourly stamp. With `background` it is the detached child the
// trigger starts, whose stdout is last.log: it refuses to run beside a
// previous child still holding run.lock, and opens its output with a header.
func runStoreTrim(arguments leaction.Arguments) (any, int) {
	root, err := checkoutRoot()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	if !arguments.Has(backgroundKeyword) {
		return trimStores(root, env.Get(storeTrimEntry.Key))
	}

	if err := os.MkdirAll(filepath.Join(root, storeTrimDir), 0o750); err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	lock, err := os.OpenFile(filepath.Join(root, storeTrimDir, runLockName), os.O_RDONLY|os.O_CREATE, 0o600) //nolint:gosec // a fixed name under the resolved checkout's tmp/store-trim
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	defer lock.Close() //nolint:errcheck // closing releases the run lock
	held, err := tryLock(lock)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	var header textbuf.Buffer
	header.Str(time.Now().UTC().Format(time.RFC3339)).Str(" store-trim pid ").Int(int64(os.Getpid()))
	if !held {
		header.Str(": a previous trim still holds run.lock, so this one exits\n").StdOut() //nolint:errcheck // stdout is last.log
		return nil, 0
	}
	header.Str(": run started\n").StdOut() //nolint:errcheck // stdout is last.log
	return trimStores(root, env.Get(storeTrimEntry.Key))
}

// storeTrimTimeout bounds one whole trim, the background child's included,
// so a walk that hangs on a stalled filesystem cannot leave a child holding
// run.lock for ever (R-6). It follows cleanTimeout: a stop for a run that has
// hung, not a budget for a large store.
const storeTrimTimeout = time.Hour

// trimStores trims every store and reports it. ze.le.store.trim governs only
// whether le starts a trim by itself: a trim that runs was asked for, by name
// or by the trigger, so an unreadable switch is refused by name in the report
// and exits 1 while every store is still trimmed, the rule a bad budget follows
// (AC-13).
func trimStores(root, automatic string) (trimReport, int) {
	return trimStoresScanning(root, automatic, session.ScanProcesses)
}

// trimStoresScanning is trimStores with the process scanner a test replaces.
func trimStoresScanning(root, automatic string, scan func() ([]session.Process, error)) (trimReport, int) {
	report := trimReport{Automatic: storeTrimOn}
	enabled, err := parseStoreTrim(automatic)
	switch {
	case err != nil:
		report.Automatic = automatic
		report.Refusal = err.Error()
	case !enabled:
		report.Automatic = storeTrimOff
	}

	ctx, cancel := context.WithTimeout(context.Background(), storeTrimTimeout)
	defer cancel()

	// The per-user target is the one trimmed store path that can fail to
	// resolve, and its failure refuses the shared row alone. The ambient cache
	// is not le's to trim (owner decision D-6), so its row is a skip either
	// way; its path is still resolved, with the one `go env` run cache-clean
	// makes, so the row names the cache it leaves alone, or says why it cannot.
	refused := map[string]string{}
	perUser, perUserErr := New(root, os.Environ()).cacheTarget()
	if perUserErr != nil {
		refused[sharedCache] = perUserErr.Error()
	}
	ambient, ambientErr := ambientGoCache(ctx)
	pass := cachePass{now: time.Now(), floor: cacheEntryAgeFloor, remove: removeCacheEntry}
	report.Budgets, report.Stores = trimCaches(ctx, cleanTargets(root, ambient, perUser), refused, readBudget, pass)
	if ambientErr != nil {
		for index := range report.Stores {
			if report.Stores[index].Name == ambientCache {
				report.Stores[index].Skipped += "; its path did not resolve: " + ambientErr.Error()
			}
		}
	}
	report.Stores = append(report.Stores, livenessStores(ctx, root, scan)...)
	return report, report.verdict()
}

// livenessStores scans the process table once and trims the stores a live
// session may be using against it. A scan that fails is every row's error:
// with no process table nothing can be judged dead.
func livenessStores(ctx context.Context, root string, scan func() ([]session.Process, error)) []StoreTrim {
	processes, err := scan()
	if err != nil {
		rows := livenessRows(root)
		for index := range rows {
			rows[index].Error = "process scan: " + err.Error()
		}
		return rows
	}
	return trimLiveness(ctx, root, &livenessPass{
		now:       time.Now(),
		processes: processes,
		judge: func(processes []session.Process) (session.Judgement, error) {
			return session.Judge(root, "", processes)
		},
		startedAt: session.ProcessStartTime,
		remove:    os.RemoveAll,
	})
}
