// Design: docs/contributing/running-commands.md -- "Scratch files", the liveness stores of the store trim
// Related: storetrim.go -- trimStores, which runs this pass after the caches
// Related: cachetrim.go -- treeBytes, the allocated-size walk both passes share

package scratch

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/session"
)

// The ages a liveness store must reach before the trim may remove it (owner
// decision D-3, fixed constants, not settings). Each is a floor on top of the
// liveness rule, never instead of it.
const (
	// launcherIdleAge: the `le` script rewrites bin/le-<name>/le on every call
	// under that name, so a day without a rewrite is a day nobody used it, and
	// a reclaimed name costs one warm rebuild.
	launcherIdleAge = 24 * time.Hour
	// sessionIdleAge: on top of Reap's rules, which judge a session dead from
	// the process table and the transcripts alone.
	sessionIdleAge = 24 * time.Hour
	// testbinIdleAge: a functional run builds its testbin and removes it when
	// it ends; one older than this outlived its run.
	testbinIdleAge = 6 * time.Hour
	// startSlack absorbs the second of rounding in a ps-measured start time. A
	// process that started after the testbin was made does not own it (pid
	// reuse), and the slack keeps a jittered owner from looking like reuse.
	startSlack = time.Minute
)

// The liveness store rows, and the one reason all three skip together.
const (
	launcherStore    = "launchers"
	sessionStore     = "sessions"
	testbinStore     = "testbins"
	noSessionVisible = "no session process visible"
	noSessionRoot    = "no tmp/session directory"
	testbinPIDPrefix = "testbin-pid-"
	scratchSubdir    = "scratch"
)

// livenessPass is what one liveness trim reads besides the tree: the clock,
// the process table scanned once, and the operations a test replaces. judge is
// session.Judge bound to the checkout; startedAt is session.ProcessStartTime;
// remove is os.RemoveAll.
type livenessPass struct {
	now       time.Time
	processes []session.Process
	judge     func([]session.Process) (session.Judgement, error)
	startedAt func(pid int) time.Time
	remove    func(path string) error
}

// trimLiveness trims the three stores a live session may be using: named
// launchers, session directories and orphaned testbins. It answers one row
// for each, in that order.
//
// With no Claude CLI in the process table the trim cannot tell a live session
// from a dead one, so it removes nothing from any of the three (AC-11): a
// QEMU guest or a container sees the checkout and none of its sessions.
func trimLiveness(ctx context.Context, root string, pass *livenessPass) []StoreTrim {
	rows := livenessRows(root)
	if !sessionProcessVisible(pass.processes) {
		for index := range rows {
			rows[index].Skipped = noSessionVisible
		}
		return rows
	}
	trimLaunchers(ctx, &rows[0], pass)
	kept, unwalked := trimSessions(ctx, &rows[1], pass)
	if unwalked != "" {
		rows[2].Skipped = unwalked
		return rows
	}
	trimTestbins(ctx, &rows[2], kept, pass)
	return rows
}

// livenessRows answers the three liveness rows, untouched, in report order:
// launchers, sessions, testbins.
func livenessRows(root string) []StoreTrim {
	return []StoreTrim{
		{Name: launcherStore, Path: filepath.Join(root, "bin")},
		{Name: sessionStore, Path: session.RootDir(root)},
		{Name: testbinStore, Path: session.RootDir(root)},
	}
}

// sessionProcessVisible answers whether any Claude CLI runs, the fact every
// liveness judgement here needs before it can call anything dead.
func sessionProcessVisible(processes []session.Process) bool {
	for _, process := range processes {
		if process.CLI {
			return true
		}
	}
	return false
}

// trimLaunchers removes each bin/le-<name>/ that nobody has rebuilt or run
// for a day. bin/le, bin/ze* and the shared platform launcher never reach the
// remove call: NamedLauncherName answers only a named build's directory.
func trimLaunchers(ctx context.Context, row *StoreTrim, pass *livenessPass) {
	entries, err := os.ReadDir(row.Path)
	if errors.Is(err, fs.ErrNotExist) {
		row.Skipped = "no bin/ directory"
		return
	}
	if err != nil {
		row.Error = err.Error()
		return
	}
	for _, entry := range entries {
		name, ok := gotoolchain.NamedLauncherName(entry.Name())
		if !ok {
			continue
		}
		dir := gotoolchain.NamedLauncherDir(filepath.Dir(row.Path), name)
		idle, idleErr := launcherIdle(dir, pass.now)
		if idleErr != nil {
			row.Error = idleErr.Error()
			continue
		}
		size := measureEntry(row, dir)
		if !idle {
			row.Kept++
			continue
		}
		if launcherNamedInArgv(pass.processes, name) {
			row.Kept++
			continue
		}
		removeLivenessEntry(ctx, row, pass, dir, size)
	}
}

// launcherIdle answers whether a named launcher's directory and its le binary
// are both older than launcherIdleAge. A directory that is a symbolic link, or
// not a directory, is never idle: the trim removes only what the script makes.
// A directory a peer already removed is not idle, and not an error. A missing
// le (a build that failed) leaves the directory's own age to judge,
// and a build in progress writes its .new.<pid> file there, which refreshes it.
func launcherIdle(dir string, now time.Time) (bool, error) {
	info, err := os.Lstat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, nil
	}
	if !olderThan(info.ModTime(), now, launcherIdleAge) {
		return false, nil
	}
	binary, err := os.Lstat(filepath.Join(dir, "le"))
	if errors.Is(err, fs.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return olderThan(binary.ModTime(), now, launcherIdleAge), nil
}

// launcherNamedInArgv answers whether a running process names the launcher's
// directory. The `le` script execs the binary by its absolute path, so a
// running named le carries bin/le-<name>/ in its argv (A-4). The trailing
// separator keeps le-a from matching le-ab.
func launcherNamedInArgv(processes []session.Process, name string) bool {
	var needle textbuf.Buffer
	needle.Str(strings.TrimPrefix(gotoolchain.NamedLauncherDir("", name), string(filepath.Separator))).
		Byte(filepath.Separator)
	path := needle.String()
	for _, process := range processes {
		for _, argument := range process.Argv {
			if strings.Contains(argument, path) {
				return true
			}
		}
	}
	return false
}

// trimSessions removes each session directory Reap's rules would remove and
// that is also older than sessionIdleAge, and answers the directories left
// standing, which the testbin pass walks. When no directory could be listed
// it answers why instead, so the testbin row says so rather than reporting an
// empty store it never looked at.
func trimSessions(ctx context.Context, row *StoreTrim, pass *livenessPass) ([]string, string) {
	judgement, err := pass.judge(pass.processes)
	if err != nil {
		row.Error = err.Error()
		return nil, "the session directories could not be judged, so none was walked: " + row.Error
	}
	if judgement.MissingRoot {
		row.Skipped = noSessionRoot
		return nil, noSessionRoot
	}
	if judgement.Notice != "" {
		row.Skipped = judgement.Notice
		return judgement.Dirs, ""
	}
	dead := make(map[string]bool, len(judgement.Dead))
	for _, dir := range judgement.Dead {
		dead[dir] = true
	}
	kept := make([]string, 0, len(judgement.Dirs))
	for _, dir := range judgement.Dirs {
		size := measureEntry(row, dir)
		if !dead[dir] {
			row.Kept++
			kept = append(kept, dir)
			continue
		}
		info, statErr := os.Lstat(dir)
		if errors.Is(statErr, fs.ErrNotExist) {
			continue
		}
		if statErr != nil {
			row.Error = statErr.Error()
			kept = append(kept, dir)
			continue
		}
		if !olderThan(info.ModTime(), pass.now, sessionIdleAge) {
			row.Kept++
			kept = append(kept, dir)
			continue
		}
		if !removeLivenessEntry(ctx, row, pass, dir, size) {
			kept = append(kept, dir)
		}
	}
	return kept, ""
}

// trimTestbins removes each testbin-pid-<pid>-<label>/ in a kept session
// directory, or in its scratch/ directory where the functional runner builds
// it, whose run is over: the pid is not running, or the process running under
// it started after the testbin was made. A testbin-<suffix>/ (an explicit
// ze.suffix) never matches the prefix, and leaves only with its session.
func trimTestbins(ctx context.Context, row *StoreTrim, sessionDirs []string, pass *livenessPass) {
	for _, sessionDir := range sessionDirs {
		for _, parent := range []string{sessionDir, filepath.Join(sessionDir, scratchSubdir)} {
			entries, err := os.ReadDir(parent)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				row.Error = err.Error()
				continue
			}
			for _, entry := range entries {
				pid, ok := testbinPID(entry.Name())
				if !ok {
					continue
				}
				trimTestbin(ctx, row, pass, filepath.Join(parent, entry.Name()), pid)
			}
		}
	}
}

// trimTestbin judges one testbin directory and removes it when its run is
// over and it is older than testbinIdleAge.
func trimTestbin(ctx context.Context, row *StoreTrim, pass *livenessPass, dir string, pid int) {
	info, err := os.Lstat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err != nil {
		row.Error = err.Error()
		return
	}
	if !info.IsDir() {
		return
	}
	size := measureEntry(row, dir)
	if !olderThan(info.ModTime(), pass.now, testbinIdleAge) {
		row.Kept++
		return
	}
	if testbinOwned(pass, pid, info.ModTime()) {
		row.Kept++
		return
	}
	removeLivenessEntry(ctx, row, pass, dir, size)
}

// testbinPID answers the pid a testbin-pid-<pid>-<label> name carries.
func testbinPID(name string) (int, bool) {
	rest, found := strings.CutPrefix(name, testbinPIDPrefix)
	if !found {
		return 0, false
	}
	digits, label, found := strings.Cut(rest, "-")
	if !found {
		return 0, false
	}
	if label == "" {
		return 0, false
	}
	pid, err := strconv.Atoi(digits)
	if err != nil {
		return 0, false
	}
	if pid <= 0 {
		return 0, false
	}
	return pid, true
}

// testbinOwned answers whether the process running under pid can be the run
// that made a testbin at made. A start time nobody can measure is ownership:
// the testbin is kept rather than removed under a run.
func testbinOwned(pass *livenessPass, pid int, made time.Time) bool {
	for _, process := range pass.processes {
		if process.PID != pid {
			continue
		}
		started := process.StartedAt
		if started.IsZero() {
			started = pass.startedAt(pid)
		}
		if started.IsZero() {
			return true
		}
		return !started.After(made.Add(startSlack))
	}
	return false
}

// olderThan answers whether a modification time is at least age before now.
// A time in the future is never old.
func olderThan(modified, now time.Time, age time.Duration) bool {
	return now.Sub(modified) >= age
}

// measureEntry adds one entry's allocated size to the row's size before and
// after, and answers it so a removal can take it off the size after. A size
// that cannot be measured is the row's error, and the entry is still judged:
// the size is the report's, never the rule's.
func measureEntry(row *StoreTrim, path string) int64 {
	size, err := treeBytes(path)
	if err != nil {
		row.Error = err.Error()
	}
	row.SizeBefore += size
	row.SizeAfter += size
	return size
}

// removeLivenessEntry removes one judged directory and answers whether it is
// gone. A peer that already removed it is gone but not counted as removed by
// this run, the rule removeOldest follows for a cache entry; a writer adding files
// while it goes is contention, kept for the next trim; the trim's deadline
// stops every removal.
func removeLivenessEntry(ctx context.Context, row *StoreTrim, pass *livenessPass, path string, size int64) bool {
	if err := ctx.Err(); err != nil {
		row.Error = err.Error()
		row.Kept++
		return false
	}
	err := pass.remove(path)
	switch {
	case err == nil:
		row.EntriesRemoved++
		row.SizeAfter -= size
		return true
	case errors.Is(err, fs.ErrNotExist):
		row.SizeAfter -= size
		return true
	case isWriterRace(err):
		row.Contended = path
		row.Kept++
		return false
	default:
		row.Error = err.Error()
		row.Kept++
		return false
	}
}
