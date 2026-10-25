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

	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/procscan"
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
	// goBuildIdleAge and testTempIdleAge: a test binary removes its
	// t.TempDir() parents and the go command its go-build* work directory when
	// they end, so what stays under the test temp base is what a killed run
	// left. A work directory is 0.5-0.7G and lands on the home device, so its
	// floor is short: a live one's mtime moves each time a bNNN child comes or
	// goes. That holds only while packages build: once a test binary runs, its
	// $WORK gets no new child, and the binary still executes from there (a fuzz
	// coordinator re-execs os.Args[0], and tests re-exec os.Executable()), so a
	// single test binary running past 2h can lose it. A t.TempDir() parent's
	// mtime moves only when the test makes another temp dir, so its floor is
	// longer than le's default test-binary timeout
	// (gotoolchain.DefaultTestTimeout) by far. A run given a -timeout or
	// -fuzztime longer than either floor, or -timeout 0, can outlive it and
	// lose its work directory or its parent.
	goBuildIdleAge  = 2 * time.Hour
	testTempIdleAge = 6 * time.Hour
	// buildStagingIdleAge: a builder writes into its .staging-<pid> directory
	// while it compiles and renames it away within minutes, so one whose
	// newest file is an hour old belongs to a builder that died.
	buildStagingIdleAge = time.Hour
	// agentFileIdleAge: an agent's own binary or scratch tree directly in bin/
	// (a mutation tester, a probe) is rebuilt on each use, so a day with no
	// write anywhere in it is a day nobody used it.
	agentFileIdleAge = 24 * time.Hour
	// buildsKept is how many builds of each platform stay in bin/builds: the
	// newest, which the launcher runs, and the one before it, which
	// `./le rollback` falls back to.
	buildsKept = 2
	// goBuildPrefix names the go command's work directories (cmd/go, $WORK).
	goBuildPrefix = "go-build"
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
	testTempStore    = "test-temp"
	buildsStore      = "builds"
	agentFilesStore  = "agent-files"
	noSessionVisible = "no session process visible"
	noSessionRoot    = "no tmp/session directory"
	testbinPIDPrefix = "testbin-pid-"
	scratchSubdir    = "scratch"
)

// livenessPass is what one liveness trim reads besides the tree: the clock,
// the process table scanned once, and the operations a test replaces. judge is
// session.Judge bound to the checkout; startedAt is procscan.StartTime;
// remove is os.RemoveAll.
type livenessPass struct {
	now       time.Time
	processes []procscan.Process
	judge     func([]procscan.Process) (session.Judgement, error)
	startedAt func(pid int) time.Time
	remove    func(path string) error
}

// trimLiveness trims the stores a live session may be using: named launchers,
// session directories, orphaned testbins, the launcher's builds and agents'
// own entries in bin/. It answers one row for each, in that order.
//
// With no Claude CLI in the process table the trim cannot tell a live session
// from a dead one, so it removes nothing from any of them (AC-11): a QEMU
// guest or a container sees the checkout and none of its sessions.
func trimLiveness(ctx context.Context, root string, pass *livenessPass) []StoreTrim {
	rows := livenessRows(root)
	if !sessionProcessVisible(pass.processes) {
		for index := range rows {
			rows[index].Skipped = noSessionVisible
		}
		return rows
	}
	trimLaunchers(ctx, &rows[0], pass)
	trimBuilds(ctx, &rows[3], pass)
	trimAgentFiles(ctx, &rows[4], pass)
	kept, unwalked := trimSessions(ctx, &rows[1], pass)
	if unwalked != "" {
		rows[2].Skipped = unwalked
		return rows
	}
	trimTestbins(ctx, &rows[2], kept, pass)
	return rows
}

// livenessRows answers the liveness rows, untouched, in report order:
// launchers, sessions, testbins, builds, agent files. trimLiveness indexes
// them in this order.
func livenessRows(root string) []StoreTrim {
	return []StoreTrim{
		{Name: launcherStore, Path: filepath.Join(root, "bin")},
		{Name: sessionStore, Path: session.RootDir(root)},
		{Name: testbinStore, Path: session.RootDir(root)},
		{Name: buildsStore, Path: gotoolchain.BuildsDir(root)},
		{Name: agentFilesStore, Path: filepath.Join(root, "bin")},
	}
}

// sessionProcessVisible answers whether any Claude CLI runs, the fact every
// liveness judgement here needs before it can call anything dead.
func sessionProcessVisible(processes []procscan.Process) bool {
	for _, process := range processes {
		if process.CLI {
			return true
		}
	}
	return false
}

// trimLaunchers removes each bin/le-<name>/ that nobody has rebuilt or run
// for a day. bin/le and bin/ze* never reach the remove call: NamedLauncherName
// answers only a bin/le-<name> directory. A leftover bin/le-<uname> from the
// launcher before bin/builds is one of those, and goes once idle.
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
		size := measureEntry(row, dir)
		if !entry.IsDir() {
			// The script makes a directory; a file or a symbolic link under
			// the name is not its, and the trim removes only what it makes.
			row.Kept++
			continue
		}
		idle, idleErr := treeIdle(dir, pass.now, launcherIdleAge)
		if idleErr != nil {
			row.Error = idleErr.Error()
			row.Kept++
			continue
		}
		if !idle {
			row.Kept++
			continue
		}
		if pathNamedInArgv(pass.processes, strings.TrimPrefix(gotoolchain.NamedLauncherDir("", name), string(filepath.Separator))) {
			row.Kept++
			continue
		}
		removeLivenessEntry(ctx, row, pass, dir, size)
	}
}

// trimBuilds removes what the launcher leaves in bin/builds once nothing needs
// it (AC-12): builds older than the newest buildsKept of their platform, a
// rolled-back or failed build once a newer one landed, a staging directory
// whose builder died, and the legacy binary once a build of this checkout's
// platform exists. Each also needs no running process to name its path, so a
// command still running from a superseded build keeps its file. The lock and
// any entry the launcher does not write are never removed.
func trimBuilds(ctx context.Context, row *StoreTrim, pass *livenessPass) {
	entries, err := os.ReadDir(row.Path)
	if errors.Is(err, fs.ErrNotExist) {
		row.Skipped = "no bin/builds directory"
		return
	}
	if err != nil {
		row.Error = err.Error()
		return
	}
	published, err := publishedBuilds(row.Path, entries)
	if err != nil {
		row.Error = err.Error()
		return
	}
	for _, entry := range entries {
		path := filepath.Join(row.Path, entry.Name())
		size := measureEntry(row, path)
		removable, judgeErr := buildEntryRemovable(path, entry, published, pass.now)
		if judgeErr != nil {
			row.Error = judgeErr.Error()
			row.Kept++
			continue
		}
		if !removable {
			row.Kept++
			continue
		}
		if pathNamedInArgv(pass.processes, filepath.Join(gotoolchain.BuildsDir(""), entry.Name())) {
			row.Kept++
			continue
		}
		removeLivenessEntry(ctx, row, pass, path, size)
	}
}

// publishedBuild is one build in bin/builds that the launcher could run: its
// directory name, its commit's short hash, and when it was last written.
type publishedBuild struct {
	name     string
	sha      string
	modified time.Time
}

// publishedBuilds answers the runnable builds in bin/builds by platform ("" is
// the checkout's own), each list in name order and so oldest first. It reads
// a build the way the launcher's resolver does (ParseBuildName, RunnableBuild),
// so the newest it holds is the one the launcher runs.
func publishedBuilds(dir string, entries []fs.DirEntry) (map[string][]publishedBuild, error) {
	published := map[string][]publishedBuild{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name, ok := gotoolchain.ParseBuildName(entry.Name())
		if !ok {
			continue
		}
		runnable, err := gotoolchain.RunnableBuild(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		if !runnable {
			continue
		}
		info, err := entry.Info()
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		published[name.Platform()] = append(published[name.Platform()], publishedBuild{
			name: entry.Name(), sha: name.Sha(), modified: info.ModTime(),
		})
	}
	return published, nil
}

// buildEntryRemovable answers whether one bin/builds entry is past what the
// launcher still needs, before the argv check. The lock is the live builder's
// (the launcher itself reclaims one whose owner died), and an entry of no
// shape the launcher writes is somebody else's, so neither is ever removable.
func buildEntryRemovable(path string, entry fs.DirEntry, published map[string][]publishedBuild, now time.Time) (bool, error) {
	name := entry.Name()
	if build, ok := gotoolchain.ParseBuildName(name); ok {
		return buildSuperseded(published[build.Platform()], name), nil
	}
	if name == gotoolchain.BuildLockEntry {
		return false, nil
	}
	if name == gotoolchain.BuildLegacyEntry {
		// The launcher runs the legacy binary only while no build of the
		// checkout's own platform exists.
		return len(published[""]) > 0, nil
	}
	if strings.HasPrefix(name, gotoolchain.BuildStagingPrefix) {
		return treeIdle(path, now, buildStagingIdleAge)
	}
	if rest, found := strings.CutPrefix(name, gotoolchain.BuildFailedPrefix); found {
		return failedSuperseded(rest, entry, published)
	}
	return false, nil
}

// buildSuperseded answers whether a build is older than the newest buildsKept
// of its platform. A build-named directory the launcher could not run is not
// in the list, and is kept: it is no build this rule ranks.
func buildSuperseded(builds []publishedBuild, name string) bool {
	for index, build := range builds {
		if build.name == name {
			return index < len(builds)-buildsKept
		}
	}
	return false
}

// failedSuperseded answers whether a .failed- entry (rest is its name after
// the prefix) records something a later build replaced. A rolled-back build,
// .failed-<build name>[.<pid>], is superseded once a newer build of its
// platform landed. A HEAD that did not build, .failed-<commit>.log or
// .reported, is superseded once a build of another commit landed after it:
// HEAD has moved on, so the launcher no longer asks whether that commit was
// rejected. Both records keep the launcher from rebuilding a rejected HEAD on
// every call until then.
func failedSuperseded(rest string, entry fs.DirEntry, published map[string][]publishedBuild) (bool, error) {
	base, suffix, _ := strings.Cut(rest, ".")
	if build, ok := gotoolchain.ParseBuildName(base); ok {
		if !entry.IsDir() {
			return false, nil
		}
		builds := published[build.Platform()]
		if len(builds) == 0 {
			return false, nil
		}
		return builds[len(builds)-1].name > base, nil
	}
	if !failedRecordSuffix(suffix) {
		return false, nil
	}
	if !entry.Type().IsRegular() {
		return false, nil
	}
	info, err := entry.Info()
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, builds := range published {
		for _, build := range builds {
			if strings.HasPrefix(base, build.sha) {
				continue
			}
			if build.modified.After(info.ModTime()) {
				return true, nil
			}
		}
	}
	return false, nil
}

// failedRecordSuffix answers whether a .failed-<commit> file is one of the two
// records the launcher writes for a HEAD that did not build.
func failedRecordSuffix(suffix string) bool {
	return suffix == "log" || suffix == "reported"
}

// trimAgentFiles removes what agents made directly in bin/ (O-8): every entry
// that is not le's own (leOwnedBinEntry) and that nothing wrote to for a day
// and no running process names.
func trimAgentFiles(ctx context.Context, row *StoreTrim, pass *livenessPass) {
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
		if leOwnedBinEntry(entry.Name()) {
			continue
		}
		path := filepath.Join(row.Path, entry.Name())
		size := measureEntry(row, path)
		idle, idleErr := treeIdle(path, pass.now, agentFileIdleAge)
		if idleErr != nil {
			row.Error = idleErr.Error()
			row.Kept++
			continue
		}
		if !idle {
			row.Kept++
			continue
		}
		if pathNamedInArgv(pass.processes, filepath.Join("bin", entry.Name())) {
			row.Kept++
			continue
		}
		removeLivenessEntry(ctx, row, pass, path, size)
	}
}

// leOwnedBinEntry answers whether a bin/ entry is le's or ze's, which the
// agent-files row never judges: the dispatcher bin/le and its le.new.<pid>
// while it is replaced, the named launchers bin/le-<name>/ (the launchers
// row's), the builds directory (the builds row's) and every ze binary.
func leOwnedBinEntry(name string) bool {
	if name == "le" {
		return true
	}
	if strings.HasPrefix(name, "le-") {
		return true
	}
	if strings.HasPrefix(name, "le.") {
		return true
	}
	if strings.HasPrefix(name, "ze") {
		return true
	}
	return name == filepath.Base(gotoolchain.BuildsDir(""))
}

// treeIdle answers whether nothing in a bin/ entry was written for age: a
// regular file by its own mtime, a directory by the newest mtime anywhere in
// its tree. The walk stops at the first young entry. A named launcher's build
// writes its .new.<pid> file inside, and an agent's tool rewrites what it
// keeps, so either use refreshes the tree. A symbolic link, or anything else
// that is neither a file nor a directory, is never idle: the trim removes only
// what a build or an agent made. An entry a peer already removed is not idle,
// and not an error.
func treeIdle(path string, now time.Time, age time.Duration) (bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.Mode().IsRegular() {
		return olderThan(info.ModTime(), now, age), nil
	}
	if !info.IsDir() {
		return false, nil
	}
	idle := true
	walkErr := filepath.WalkDir(path, func(_ string, entry fs.DirEntry, walkErr error) error {
		if errors.Is(walkErr, fs.ErrNotExist) {
			return nil
		}
		if walkErr != nil {
			return walkErr
		}
		entryInfo, infoErr := entry.Info()
		if errors.Is(infoErr, fs.ErrNotExist) {
			return nil
		}
		if infoErr != nil {
			return infoErr
		}
		if olderThan(entryInfo.ModTime(), now, age) {
			return nil
		}
		idle = false
		return fs.SkipAll
	})
	if walkErr != nil {
		return false, walkErr
	}
	return idle, nil
}

// pathNamedInArgv answers whether a running process names a path under the
// checkout in its argv. The `le` script execs a build by its absolute path,
// and a tool is run as bin/<name> or by its absolute path, so a running one
// carries its checkout-relative path, which is what path is (A-4). The path
// must stand as whole names: bin/le-a does not match bin/le-ab, nor bin/gomu
// bin/gomu-rfc.
func pathNamedInArgv(processes []procscan.Process, path string) bool {
	for _, process := range processes {
		for _, argument := range process.Argv {
			if argumentNamesPath(argument, path) {
				return true
			}
		}
	}
	return false
}

// argumentNamesPath answers whether path occurs in one argument with no file
// name character on either side, so `-o=bin/x`, `/abs/bin/x/le` and `bin/x`
// name bin/x and `bin/x-y` or `cabin/x` do not. The scan moves one byte past
// each occurrence, so it ends within len(argument) steps.
func argumentNamesPath(argument, path string) bool {
	offset := 0
	for {
		index := strings.Index(argument[offset:], path)
		if index < 0 {
			return false
		}
		start := offset + index
		end := start + len(path)
		if nameBoundary(argument, start-1) {
			if nameBoundary(argument, end) {
				return true
			}
		}
		offset = start + 1
	}
}

// nameBoundary answers whether the byte at index ends a file name: the edge of
// the argument, or a byte no file name here is spelled with (a separator, `=`,
// a colon).
func nameBoundary(argument string, index int) bool {
	if index < 0 {
		return true
	}
	if index >= len(argument) {
		return true
	}
	return !fileNameCharacter(argument[index])
}

// fileNameCharacter answers whether a byte can be part of a bin/ entry's name:
// a letter, a digit, a hyphen, an underscore or a dot.
func fileNameCharacter(character byte) bool {
	switch {
	case character >= 'a' && character <= 'z':
		return true
	case character >= 'A' && character <= 'Z':
		return true
	case character >= '0' && character <= '9':
		return true
	}
	return character == '-' || character == '_' || character == '.'
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

// testTempRow answers the test temp row, untouched: the base every le-run Go
// test keeps its temporary directories under (gotoolchain.TestTempDir), or a
// skip that says why this home has none. It creates nothing.
func testTempRow() StoreTrim {
	row := StoreTrim{Name: testTempStore}
	home, err := os.UserHomeDir()
	if err != nil {
		row.Skipped = err.Error()
		return row
	}
	base, err := gotoolchain.TestTempBase(home)
	if err != nil {
		row.Skipped = err.Error()
		return row
	}
	row.Path = base
	return row
}

// trimTestTemp removes each entry under the test temp base older than its
// floor: goBuildIdleAge for a go-build* work directory, testTempIdleAge for
// anything else, a t.TempDir() parent above all. Age alone judges it, with no
// session liveness: the base is per user and shared by every checkout and
// session, so no one session owns an entry, and a run still writing one is
// younger than the floor. Only pass.now and pass.remove are read.
//
// A killed permission test leaves a directory it chmodded to 0o000 or 0o500,
// which no walk can measure and RemoveAll cannot empty. An old entry is first
// opened back to its owner (openTestTempEntry); a young one that cannot be
// measured is contention, a run still holding it, never the row's error.
func trimTestTemp(ctx context.Context, row *StoreTrim, pass *livenessPass) {
	if row.Skipped != "" {
		return
	}
	entries, err := os.ReadDir(row.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err != nil {
		row.Error = err.Error()
		return
	}
	for _, entry := range entries {
		path := filepath.Join(row.Path, entry.Name())
		info, err := os.Lstat(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			row.Error = err.Error()
			continue
		}
		floor := testTempIdleAge
		if strings.HasPrefix(entry.Name(), goBuildPrefix) {
			floor = goBuildIdleAge
		}
		old := olderThan(info.ModTime(), pass.now, floor)
		if old {
			if err := openTestTempEntry(path); err != nil {
				row.Error = testTempOpenFailure(err, row.Path)
				row.Kept++
				continue
			}
		}
		size, err := treeBytes(path)
		switch {
		case err == nil:
		case !old && errors.Is(err, fs.ErrPermission):
			row.Contended = path
		default:
			row.Error = err.Error()
		}
		row.SizeBefore += size
		row.SizeAfter += size
		if !old {
			row.Kept++
			continue
		}
		removeLivenessEntry(ctx, row, pass, path, size)
	}
}

// testTempOpenFailure answers the row error for an old entry that cannot be
// opened. A permission refusal there is an entry this user does not own, which
// a `sudo -E` test run leaves as root's, so the error names the chown that
// gives the base back; any other failure is reported as it is.
func testTempOpenFailure(err error, base string) string {
	if errors.Is(err, fs.ErrPermission) {
		return err.Error() + "; sudo chown -R $USER " + base
	}
	return err.Error()
}

// openTestTempEntry gives every directory under path, path included, back
// to its owner (0700), so a walk can measure it and RemoveAll can empty it.
// WalkDir visits a directory before it reads it, so a 0o000 directory is
// opened before the walk needs to enter it. It follows no symbolic link.
func openTestTempEntry(path string) error {
	return filepath.WalkDir(path, func(walked string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		return os.Chmod(walked, 0o700) //nolint:gosec // a directory needs its search bit to be walked and emptied; owner only
	})
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
