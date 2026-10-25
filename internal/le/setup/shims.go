// Design: docs/guide/developer-setup.md -- tool shims in a PATH directory setup chooses, the go env GOCACHE setting
// Related: setup.go -- Run visits these rows after the tool rows
// Related: gobin.go -- onPath, shellProfile and shellSingleQuote, which these rows reuse
// Related: ../exec/exec.go -- le exec, which every shim starts, and ShimMarker
//
// gopls, golangci-lint and staticcheck each keep a cache, and each picks its
// default folder (~/Library/Caches on a Mac) unless its own variable says
// otherwise. An editor started from the Dock reads no shell file, so a shell
// variable misses it. A shim that runs `le exec` on the real tool, in a
// directory PATH reaches before the real tool, reaches every caller that finds
// the tool by PATH, and `le exec` sets the variables in the one place they are
// derived.
//
// The shim directory is chosen, never assumed: the earliest PATH entry that is
// an existing directory under HOME this user can write, outside any ze
// checkout, and ahead of every directory holding a real shimmed tool. The Go
// bin directory never qualifies, because it holds the real tool, and that is
// right: `go install` would overwrite a shim there.
//
// Every PATH this file judges is the caller's (callerPath), never le's own: le
// puts the patched GOROOT's bin first on its PATH before it dispatches, and
// that tree sits under the per-user cache, so under HOME and writable. Judged
// against le's PATH it qualified, and the shims went where no shell of the
// developer's looks.
//
// le never writes a shell file: when no PATH entry qualifies, the shims go in
// ~/bin and the shim-dir row prints the line for zsh and for NuShell that puts
// ~/bin first, and the developer adds it.
//
// Plain `go` outside le reads its GOCACHE from `go env`, which setup owns, so
// the GOCACHE row repairs it to the per-user target's go-cache.

package setup

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/textbuf"
	leexec "github.com/ze-software/ze/internal/le/exec"
	"github.com/ze-software/ze/internal/le/go/stdpatch/gen"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/scratch"
)

// The rows these visits report, and the names they read.
const (
	// shimRowSuffix follows the tool name in a shim row: gopls-shim.
	shimRowSuffix = "-shim"
	// shimDirRow is the row for the directory the shims go in.
	shimDirRow = "shim-dir"
	// goCacheRow is the row for the `go env GOCACHE` setting.
	goCacheRow = "go-env-gocache"
	// homeBinName is the directory under HOME the shims go in when no PATH
	// entry qualifies.
	homeBinName = "bin"
	// goCacheKey is the go environment variable the GOCACHE row reads and writes.
	goCacheKey = "GOCACHE"
	// goCacheDir is the directory under the per-user target that holds the
	// Go build cache, the one the checkout's cache/ link reaches.
	goCacheDir = "go-cache"
	// nuBin is NuShell, whose env file gets its own PATH line.
	nuBin = "nu"
	// setupInstall is the command that writes the shims and repairs GOCACHE.
	setupInstall = "./le setup install"
)

// shimMode is what a shim is written with: the developer's shells and editors
// run it, and only the owner writes it.
const shimMode fs.FileMode = 0o755

// nuEnvPathTimeout bounds `nu -c '$nu.env-path'`, which prints a path and exits.
const nuEnvPathTimeout = 30 * time.Second

// shimTools answers the tool rows that get a shim, in table order. The table
// row carries the choice (Tool.Shim), so a new shimmed tool is one field.
func shimTools() []Tool {
	var tools []Tool
	for _, tool := range requiredTools() {
		if tool.Shim {
			tools = append(tools, tool)
		}
	}
	return tools
}

// homeBin answers ~/bin, and whether HOME could be found.
func (s *Setup) homeBin() (string, bool) {
	home := s.home()
	if home == "" {
		return "", false
	}
	return filepath.Join(home, homeBinName), true
}

// shimPlace is the directory one run writes the shims into. Only
// chooseShimDir builds one, so a shim row never writes into a directory nobody
// chose.
type shimPlace struct {
	// dir is the absolute, cleaned directory the shims go in.
	dir string
	// problem says why no PATH entry qualified, and is empty when dir is one.
	// Non-empty means dir is the ~/bin fallback and PATH must change.
	problem string
	// callerPath is the caller's PATH dir was chosen against (callerPath), so
	// every row of one run judges the real tools and the old shims against
	// the same PATH.
	callerPath string
}

// callerPath answers the PATH of the program that started le, the one every
// shim decision and the gopls-answers row are judged against. le's own PATH is
// not it: gen.Activate puts the patched GOROOT's bin first before dispatch, and
// a le started by another le inherits its parent's tree bin as well.
// gen.CallerEnvironment restores the caller's PATH and drops every patched
// tree's bin, so this is the PATH a developer's shell and editor search.
//
// The error says the checkout's patch set could not be read, so le cannot tell
// a patched tree's bin from a directory of the caller's.
func (s *Setup) callerPath() (string, error) {
	environ, err := gen.CallerEnvironment(s.Root, []string{"PATH=" + s.env("PATH")})
	if err != nil {
		return "", fmt.Errorf("cannot tell le's own PATH entries from the caller's, so no shim directory can be judged "+
			"(the checkout must hold %s; `git status %s` shows what is missing): %w", gen.PatchDir, gen.PatchDir, err)
	}
	for _, entry := range environ {
		if path, ok := strings.CutPrefix(entry, "PATH="); ok {
			return path, nil
		}
	}
	// CallerEnvironment removes PATH when the caller had none set, so an empty
	// PATH is the caller's own answer: nothing on it qualifies.
	return "", nil
}

// chooseShimDir answers the shim directory and whether HOME could be found.
// The first entry of callerPath that qualifies wins (a duplicate entry counts at its
// first position, the one a lookup reaches); with none, ~/bin is the fallback
// and the place carries the reason, naming each entry under HOME it passed
// over and why, so a developer whose own directory was refused sees the cause.
func (s *Setup) chooseShimDir(callerPath string) (shimPlace, bool) {
	bin, ok := s.homeBin()
	if !ok {
		return shimPlace{}, false
	}
	entries := filepath.SplitList(callerPath)
	limit, holder, holderTool := s.firstRealToolEntry(callerPath, entries)
	home := filepath.Clean(s.home())
	var skipped textbuf.Buffer
	for _, entry := range entries[:limit] {
		if !filepath.IsAbs(entry) {
			continue
		}
		dir := filepath.Clean(entry)
		if !insideDir(dir, home) {
			continue
		}
		refusal := shimDirRefusal(dir)
		if refusal == "" {
			return shimPlace{dir: dir, callerPath: callerPath}, true
		}
		skipped.Str("; skipped ").Str(dir).Str(": ").Str(refusal)
	}
	var problem textbuf.Buffer
	problem.Str("no PATH entry is an existing directory under ").Str(home).
		Str(" that this user can write, outside a ze checkout")
	if holder != "" {
		problem.Str(", ahead of ").Str(holder).Str(", which holds the real ").Str(holderTool)
	}
	problem.Str(skipped.String())
	return shimPlace{dir: bin, problem: problem.String(), callerPath: callerPath}, true
}

// firstRealToolEntry answers the position in entries, callerPath split, of the
// first directory holding
// a real shimmed tool, that directory and the tool, or len(entries) and "" when
// no real shimmed tool is on PATH. A shim directory must come before it, or a
// lookup finds the real tool first and the shim never runs.
func (s *Setup) firstRealToolEntry(callerPath string, entries []string) (int, string, string) {
	limit, holder, holderTool := len(entries), "", ""
	for _, tool := range shimTools() {
		program, found := realTool(callerPath, tool.Probe[0], "")
		if !found {
			continue
		}
		dir := filepath.Dir(program)
		position := pathIndex(entries, dir)
		if position >= 0 && position < limit {
			limit, holder, holderTool = position, dir, tool.Probe[0]
		}
	}
	return limit, holder, holderTool
}

// shimDirRefusal answers why dir, an absolute PATH entry under HOME, cannot
// hold the shims, or "" when it can: it must be an existing directory this
// user can write, outside a ze checkout, where a checkout's own bin would make
// the shims come and go with the tree.
func shimDirRefusal(dir string) string {
	info, err := os.Stat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return "does not exist"
	}
	if err != nil {
		var tb textbuf.Buffer
		return tb.Str("cannot be read: ").Err(err).String()
	}
	if !info.IsDir() {
		return "is not a directory"
	}
	if err := unix.Access(dir, unix.W_OK); err != nil {
		var tb textbuf.Buffer
		return tb.Str("is not writable: ").Err(err).String()
	}
	if checkout, inside := lepath.CheckoutContaining(dir); inside {
		var tb textbuf.Buffer
		return tb.Str("is inside the ze checkout ").Str(checkout).String()
	}
	return ""
}

// insideDir reports whether path lies strictly under dir. Both are cleaned and
// compared by spelling, so a PATH entry naming HOME through a link does not
// count: the developer's PATH and HOME spell the same tree the same way.
func insideDir(path, dir string) bool {
	if dir == string(filepath.Separator) {
		return path != dir
	}
	return strings.HasPrefix(path, dir+string(filepath.Separator))
}

// realTool answers the first executable named name on path, a PATH value, that is
// neither in skip (the shim directory, or "" to skip none) nor a shim, and
// whether there is one. It is the program a shim starts, so a relative PATH
// entry is passed over: a shim names an absolute path.
func realTool(path, name, skip string) (string, bool) {
	for _, entry := range filepath.SplitList(path) {
		if !filepath.IsAbs(entry) {
			continue
		}
		if skip != "" && filepath.Clean(entry) == skip {
			continue
		}
		candidate := filepath.Join(entry, name)
		info, err := os.Stat(candidate)
		if err != nil {
			continue
		}
		if !isExecutable(info) {
			continue
		}
		shim, err := leexec.IsShim(candidate)
		if err != nil {
			continue
		}
		if shim {
			continue
		}
		return candidate, true
	}
	return "", false
}

// firstExecutable answers the first executable named name in an absolute
// entry of callerPath, shim or not: the program a caller's shell runs for name.
func firstExecutable(callerPath, name string) (string, bool) {
	for _, entry := range filepath.SplitList(callerPath) {
		if !filepath.IsAbs(entry) {
			continue
		}
		candidate := filepath.Join(entry, name)
		info, err := os.Stat(candidate)
		if err != nil {
			// Nothing a lookup can start is there, the way exec.LookPath
			// passes over the entry.
			continue
		}
		if isExecutable(info) {
			return candidate, true
		}
	}
	return "", false
}

// isExecutable reports whether info, a file found through any link, is a
// regular file somebody may execute.
func isExecutable(info fs.FileInfo) bool {
	if !info.Mode().IsRegular() {
		return false
	}
	return info.Mode().Perm()&0o111 != 0
}

// presentPastShim reports whether name is on PATH as a usable program. A shim
// found first counts only when a real copy stands behind it, because a shim
// with nothing to start would otherwise keep the tool row green and the
// install route closed.
//
// This one probe judges le's own PATH, the one Shell.Which searched, rather
// than the caller's: it asks whether this run can start the tool. The two
// answer alike for a real tool, because le's PATH is the caller's with patched
// tree bins added, and a tree's bin holds no shimmed tool that is not a shim.
func (s *Setup) presentPastShim(name string) bool {
	path, ok := s.Shell.Which(name)
	if !ok {
		return false
	}
	shim, err := leexec.IsShim(path)
	if err == nil && shim {
		// realTool passes over every shim, so no directory needs skipping.
		_, found := realTool(s.env("PATH"), name, "")
		return found
	}
	return true
}

// shimScript answers the shim for name: a POSIX sh script that carries
// ShimMarker on its second line and replaces itself with `<le> exec <real>`
// and every argument. A shim whose le has gone says so and exits 1 rather than
// failing with the shell's bare "not found" (R-2).
func shimScript(lePath, name, program string) string {
	var why textbuf.Buffer
	why.Str(name).Str(": cannot start: ").Str(lePath).Str(" is missing or not executable")
	var cause textbuf.Buffer
	cause.Str("  the checkout this shim was written for has moved or gone, so le exec cannot start ").Str(program)
	var fix textbuf.Buffer
	fix.Str("  fix: run ").Str(setupInstall).Str(" from the checkout you use now; it rewrites the shims")

	var script textbuf.Buffer
	script.Str("#!/bin/sh\n").Str(leexec.ShimMarker).Str("\n")
	script.Str("# Starts the real ").Str(name).Str(" through le exec, so its cache lands under the per-user target.\n")
	script.Str("if [ ! -x ").Str(shellSingleQuote(lePath)).Str(" ]; then\n")
	script.Str("\tprintf '%s\\n' ").Str(shellSingleQuote(why.String())).Str(" ").
		Str(shellSingleQuote(cause.String())).Str(" ").Str(shellSingleQuote(fix.String())).Str(" >&2\n")
	script.Str("\texit 1\nfi\n")
	script.Str("exec ").Str(shellSingleQuote(lePath)).Str(" exec ").Str(shellSingleQuote(program)).Str(" \"$@\"\n")
	return script.String()
}

// shimFile is what the shim directory holds at a shim's path before this run touches it.
type shimFile int

const (
	shimFileUnspecified shimFile = iota
	// shimFileAbsent means nothing is there.
	shimFileAbsent
	// shimFileCurrent means a shim with exactly the wanted content.
	shimFileCurrent
	// shimFileStale means a shim that names another le or another tool path.
	shimFileStale
	// shimFileForeign means a file setup did not write, which it never touches.
	shimFileForeign
)

// inspectShim answers what path holds, compared with the wanted content.
func inspectShim(path, want string) (shimFile, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return shimFileAbsent, nil
	}
	if err != nil {
		return shimFileUnspecified, err
	}
	if !info.Mode().IsRegular() {
		return shimFileForeign, nil
	}
	shim, err := leexec.IsShim(path)
	if err != nil {
		return shimFileUnspecified, err
	}
	if !shim {
		return shimFileForeign, nil
	}
	data, err := os.ReadFile(path) //nolint:gosec // G304: a shim in the developer's own shim directory
	if err != nil {
		return shimFileUnspecified, err
	}
	if string(data) == want {
		return shimFileCurrent, nil
	}
	return shimFileStale, nil
}

// writeShim puts content at path through a temporary file and a rename, so an
// editor starting the tool mid-write runs the old shim or the new one, never
// half of one.
func writeShim(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".shim-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	_, writeErr := temp.WriteString(content)
	closeErr := temp.Close()
	err = errors.Join(writeErr, closeErr)
	if err == nil {
		err = os.Chmod(tempPath, shimMode)
	}
	if err == nil {
		err = os.Rename(tempPath, path)
	}
	if err != nil {
		return errors.Join(err, os.Remove(tempPath))
	}
	return nil
}

// visitShims reports the shim row of every shimmed tool and the shim-dir row,
// all against the one directory chooseShimDir picks for this run.
func (s *Setup) visitShims(report *Report) {
	callerPath, err := s.callerPath()
	if err != nil {
		for _, tool := range shimTools() {
			report.Add(Outcome{Name: tool.Name + shimRowSuffix, State: StateMissing, Detail: err.Error()})
		}
		report.Add(Outcome{Name: shimDirRow, State: StateMissing, Detail: err.Error()})
		return
	}
	place, ok := s.chooseShimDir(callerPath)
	if !ok {
		const noHome = "no home directory to put the shims under; set HOME"
		for _, tool := range shimTools() {
			report.Add(Outcome{Name: tool.Name + shimRowSuffix, State: StateMissing, Detail: noHome})
		}
		report.Add(Outcome{Name: shimDirRow, State: StateMissing, Detail: noHome})
		return
	}
	for _, tool := range shimTools() {
		report.Add(s.visitShim(report, tool, place))
	}
	report.Add(s.visitShimDir(report, place))
}

// visitShim reports the shim for one tool row, and writes or rewrites it on an
// install run. It rewrites a stale shim every run, so a tool reinstalled to
// another path or a checkout that moved is picked up by rerunning (R-3). Once
// the shim in place.dir is current, a le shim for the same tool anywhere else
// is removed, so a shim directory that changed leaves no copy behind.
func (s *Setup) visitShim(report *Report, tool Tool, place shimPlace) Outcome {
	name := tool.Probe[0]
	row := tool.Name + shimRowSuffix

	root, err := filepath.Abs(s.Root)
	if err != nil {
		var tb textbuf.Buffer
		return Outcome{Name: row, State: StateMissing, Detail: tb.Str("cannot name the checkout: ").Err(err).String()}
	}
	program, found := realTool(place.callerPath, name, place.dir)
	if !found {
		var tb textbuf.Buffer
		tb.Str("no real ").Str(name).Str(" on PATH outside ").Str(place.dir).Str("; the ").Str(tool.Name).
			Str(" row installs it, then re-run ").Str(setupInstall)
		return Outcome{Name: row, State: StateMissing, Detail: tb.String()}
	}

	path := filepath.Join(place.dir, name)
	want := shimScript(filepath.Join(root, "le"), name, program)
	held, err := inspectShim(path, want)
	if err != nil {
		var tb textbuf.Buffer
		return Outcome{Name: row, State: StateMissing, Detail: tb.Str("cannot read ").Str(path).Str(": ").Err(err).String()}
	}
	strays, err := s.strayShims(name, place)
	if err != nil {
		var tb textbuf.Buffer
		return Outcome{Name: row, State: StateMissing, Detail: tb.Str("cannot look for old shims: ").Err(err).String()}
	}

	var runs textbuf.Buffer
	runs.Str(path).Str(" runs ").Str(program)
	switch held {
	case shimFileCurrent:
		if len(strays) == 0 {
			return Outcome{Name: row, State: StatePresent, Detail: runs.String()}
		}
	case shimFileForeign:
		return s.shimConflict(report, row, path)
	case shimFileAbsent, shimFileStale:
		// Written below, or reported in a check run.
	case shimFileUnspecified:
		panic("BUG: inspectShim answered no state without an error")
	default:
		panic("BUG: unknown shim file state")
	}

	if s.Check {
		var tb textbuf.Buffer
		if held != shimFileCurrent {
			tb.Str(path).Str(" is not written or is out of date; ")
		}
		if len(strays) > 0 {
			tb.Str("old shims ").Str(strings.Join(strays, ", ")).Str(" are still in place; ")
		}
		tb.Str(setupInstall).Str(" fixes it")
		return Outcome{Name: row, State: StateMissing, Detail: tb.String()}
	}
	var detail textbuf.Buffer
	if held == shimFileStale {
		detail.Str("rewritten: ")
	}
	detail.Str(runs.String())
	if held != shimFileCurrent {
		if err := writeShim(path, want); err != nil {
			var tb textbuf.Buffer
			return Outcome{Name: row, State: StateMissing, Detail: tb.Str("cannot write ").Str(path).Str(": ").Err(err).String()}
		}
	}
	// The new shim is in place before any old one goes, so a tool started
	// meanwhile always finds one of them.
	for _, stray := range strays {
		if err := os.Remove(stray); err != nil {
			var tb textbuf.Buffer
			return Outcome{Name: row, State: StateMissing, Detail: tb.Str("cannot remove the old shim ").Str(stray).Str(": ").Err(err).String()}
		}
	}
	if len(strays) > 0 {
		detail.Str("; removed old shims ").Str(strings.Join(strays, ", "))
	}
	return Outcome{Name: row, State: StateInstalled, Detail: detail.String()}
}

// strayShims answers every le shim for name outside place.dir: in ~/bin, where
// setup put the shims before it chose a PATH entry, in every absolute entry of
// the caller's PATH, and in every absolute entry of le's own PATH, which holds
// the patched GOROOT's bin where a run that judged le's PATH wrote them. Only
// a regular file carrying ShimMarker counts, so a file setup did
// not write is never on the list. Each directory is looked at once, and
// directories are told apart by identity rather than spelling: on a
// case-insensitive disk ~/Bin is ~/bin, and a second spelling of dir must not
// list the shim this run keeps.
func (s *Setup) strayShims(name string, place shimPlace) ([]string, error) {
	candidates := []string{place.dir}
	candidates = append(candidates, filepath.SplitList(place.callerPath)...)
	candidates = append(candidates, filepath.SplitList(s.env("PATH"))...)
	if bin, ok := s.homeBin(); ok {
		candidates = append(candidates, bin)
	}
	var seen []fs.FileInfo
	var strays []string
	for index, entry := range candidates {
		if !filepath.IsAbs(entry) {
			continue
		}
		clean := filepath.Clean(entry)
		dirInfo, err := os.Stat(clean)
		if unreachableEntry(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if slices.ContainsFunc(seen, func(known fs.FileInfo) bool { return os.SameFile(known, dirInfo) }) {
			continue
		}
		seen = append(seen, dirInfo)
		if index == 0 {
			// place.dir itself, listed first so every other spelling of it is seen.
			continue
		}
		path := filepath.Join(clean, name)
		info, err := os.Lstat(path)
		if unreachableEntry(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		shim, err := leexec.IsShim(path)
		if err != nil {
			return nil, err
		}
		if shim {
			strays = append(strays, path)
		}
	}
	return strays, nil
}

// unreachableEntry reports whether a lookup error means the PATH entry holds
// nothing under that name this user can reach: nothing there, an entry that
// is a file rather than a directory, or a directory this user cannot search. A
// command lookup passes over the same entries, so no shim there can run.
func unreachableEntry(err error) bool {
	if errors.Is(err, fs.ErrNotExist) {
		return true
	}
	if errors.Is(err, unix.ENOTDIR) {
		return true
	}
	return errors.Is(err, fs.ErrPermission)
}

// shimConflict answers the outcome for a file in the shim directory that setup
// did not write. Setup leaves it alone and names the command that moves it
// aside.
func (s *Setup) shimConflict(report *Report, row, path string) Outcome {
	var run textbuf.Buffer
	report.Note(run.Str("  Run: mv ").Str(shellSingleQuote(path)).Str(" ").
		Str(shellSingleQuote(path + ".not-a-shim")).Str(" && ").Str(setupInstall).String())
	var why textbuf.Buffer
	report.Note(why.Str("    ").Str(path).
		Str(" is not a shim ./le setup wrote, so setup will not overwrite it, and the shim cannot go there until it moves").String())
	var detail textbuf.Buffer
	detail.Str(path).Str(" is not a le shim; move it aside")
	return Outcome{Name: row, State: StatePending, Detail: detail.String()}
}

// pathIndex answers the position of dir among the PATH entries, or -1.
func pathIndex(entries []string, dir string) int {
	want := filepath.Clean(dir)
	for index, entry := range entries {
		if entry == "" {
			continue
		}
		if filepath.Clean(entry) == want {
			return index
		}
	}
	return -1
}

// visitShimDir reports the directory the shims went in. A PATH entry that
// qualified is present. The ~/bin fallback is PENDING in both modes: only the
// developer can change a shell file, so the row prints the lines that put
// ~/bin first and writes none.
func (s *Setup) visitShimDir(report *Report, place shimPlace) Outcome {
	if place.problem == "" {
		var tb textbuf.Buffer
		tb.Str(place.dir).Str(" is the first entry of the caller's PATH that can hold the shims, ahead of every real tool")
		return Outcome{Name: shimDirRow, State: StatePresent, Detail: tb.String()}
	}
	s.notePathLines(report, place.dir)
	var tb textbuf.Buffer
	tb.Str(place.problem).Str("; the shims are in ").Str(place.dir).Str(", which PATH must reach first")
	return Outcome{Name: shimDirRow, State: StatePending, Detail: tb.String()}
}

// notePathLines prints the line that puts bin first on PATH for a POSIX shell
// profile, and for NuShell when nu is on PATH.
func (s *Setup) notePathLines(report *Report, bin string) {
	var run textbuf.Buffer
	report.Note(run.Str("  Run: export PATH=").Str(shellSingleQuote(bin)).Str(":\"$PATH\"").String())
	var why textbuf.Buffer
	report.Note(why.Str("    the shims in ").Str(bin).Str(" must come first on PATH; add that line to ").
		Str(s.shellProfile()).Str(" too, so every new shell finds them first").String())

	if !s.Shell.Present(nuBin) {
		return
	}
	var line textbuf.Buffer
	line.Str("$env.PATH = ($env.PATH | split row (char esep) | prepend r#'").Str(bin).Str("'# | uniq)")
	result := s.Shell.Run(Cmd{Argv: []string{nuBin, "-c", "$nu.env-path"}, Timeout: nuEnvPathTimeout})
	envFile := strings.TrimSpace(result.Out)
	var nu textbuf.Buffer
	if result.OK() && envFile != "" {
		report.Note(nu.Str("  NuShell: add to ").Str(envFile).Str(": ").Str(line.String()).String())
		return
	}
	report.Note(nu.Str("  NuShell: `nu -c '$nu.env-path'` did not name the env file (").Str(result.complaint()).
		Str("); add to your env.nu: ").Str(line.String()).String())
}

// cacheEnviron answers the two variables the per-user cache target is derived
// from, as this run sees them.
func (s *Setup) cacheEnviron() []string {
	return []string{"XDG_CACHE_HOME=" + s.env("XDG_CACHE_HOME"), "HOME=" + s.home()}
}

// goEnvironWithoutCache answers this process's environment with GOCACHE
// removed, so `go env GOCACHE` reports the setting rather than the variable le
// exports to its own children.
func goEnvironWithoutCache() []string {
	prefix := goCacheKey + "="
	environ := os.Environ()
	kept := make([]string, 0, len(environ))
	for _, entry := range environ {
		if strings.HasPrefix(entry, prefix) {
			continue
		}
		kept = append(kept, entry)
	}
	return kept
}

// visitGoCacheSetting reports the `go env GOCACHE` setting and, on an install
// run, rewrites it to the per-user target's go-cache with `go env -w`. A
// failed rewrite names the exact command, and the other rows still run (R-5).
func (s *Setup) visitGoCacheSetting(report *Report) Outcome {
	if !s.Shell.Present(toolGo) {
		return Outcome{Name: goCacheRow, State: StateSkipped, Detail: "go is not installed; the go row installs it"}
	}
	target, err := scratch.CacheTarget(s.cacheEnviron())
	if err != nil {
		var tb textbuf.Buffer
		return Outcome{Name: goCacheRow, State: StateMissing, Detail: tb.Str("cannot name the per-user cache: ").Err(err).String()}
	}
	want := filepath.Join(target, goCacheDir)

	environ := goEnvironWithoutCache()
	result := s.Shell.Run(Cmd{
		Argv:    []string{toolGo, goEnvSubcommand, goCacheKey},
		Dir:     s.Root,
		Env:     environ,
		Timeout: goEnvTimeout,
	})
	if !result.OK() {
		var tb textbuf.Buffer
		return Outcome{Name: goCacheRow, State: StateMissing, Detail: tb.Str("go env GOCACHE failed: ").Str(result.complaint()).String()}
	}
	current := strings.TrimSpace(result.Out)
	if current == "" {
		return Outcome{Name: goCacheRow, State: StateMissing, Detail: "go env GOCACHE answered nothing"}
	}
	same, err := scratch.SameDirectory(current, want)
	if err != nil {
		var tb textbuf.Buffer
		return Outcome{Name: goCacheRow, State: StateMissing, Detail: tb.Str("cannot tell whether go env GOCACHE ").Str(current).
			Str(" is ").Str(want).Str(": ").Err(err).String()}
	}
	if same {
		return Outcome{Name: goCacheRow, State: StatePresent, Detail: want}
	}

	if s.Check {
		var tb textbuf.Buffer
		tb.Str("go env GOCACHE is ").Str(current).Str(", not ").Str(want).Str("; ").Str(setupInstall).Str(" sets it")
		return Outcome{Name: goCacheRow, State: StateMissing, Detail: tb.String()}
	}
	var assignment textbuf.Buffer
	assignment.Str(goCacheKey).Byte('=').Str(want)
	written := s.Shell.Run(Cmd{
		Argv:    []string{toolGo, goEnvSubcommand, "-w", assignment.String()},
		Dir:     s.Root,
		Env:     environ,
		Timeout: goEnvTimeout,
	})
	if !written.OK() {
		var run textbuf.Buffer
		report.Note(run.Str("  Run: go env -w GOCACHE=").Str(shellSingleQuote(want)).String())
		var why textbuf.Buffer
		report.Note(why.Str("    `go env -w` failed, so plain go outside le keeps writing ").Str(current).String())
		var tb textbuf.Buffer
		return Outcome{Name: goCacheRow, State: StateMissing, Detail: tb.Str("go env -w GOCACHE failed: ").Str(written.complaint()).String()}
	}
	var tb textbuf.Buffer
	return Outcome{Name: goCacheRow, State: StateInstalled, Detail: tb.Str("was ").Str(current).Str(", now ").Str(want).String()}
}
