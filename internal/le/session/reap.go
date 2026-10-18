// Design: docs/architecture/core-design.md -- conservative cleanup of ended sessions
package session

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/procscan"
)

const sessionRootRel = "tmp/session"

// RootDir answers tmp/session under root, the directory Reap and Judge walk.
func RootDir(root string) string {
	return filepath.Join(root, filepath.FromSlash(sessionRootRel))
}

// ReapReport is one conservative session cleanup decision.
type ReapReport struct {
	Dry            bool     `json:"dry"`
	RemovedDirs    int      `json:"removed-directories"`
	RemovedMarkers int      `json:"removed-markers"`
	Kept           int      `json:"kept-running"`
	Paths          []string `json:"paths,omitempty"`
	Notice         string   `json:"notice,omitempty"`
	MissingRoot    bool     `json:"missing-root"`
}

// Text renders the established dry-run paths and summary line.
func (r ReapReport) Text() string {
	if r.Notice != "" {
		return r.Notice + "\n"
	}
	if r.MissingRoot {
		return "session-reap: no tmp/session, nothing to do\n"
	}
	var text textbuf.Buffer
	text.Reset()
	if r.Dry {
		for _, path := range r.Paths {
			text.Str(path).Byte('\n')
		}
	}
	verb := "Removed"
	if r.Dry {
		verb = "Would remove"
	}
	directory := "directories"
	if r.RemovedDirs == 1 {
		directory = "directory"
	}
	marker := "markers"
	if r.RemovedMarkers == 1 {
		marker = "marker"
	}
	text.Str("session-reap: ").Str(verb).Byte(' ').Int(int64(r.RemovedDirs)).
		Str(" dead session ").Str(directory).Str(" and ").Int(int64(r.RemovedMarkers)).
		Byte(' ').Str(marker).Str("; kept ").Int(int64(r.Kept)).Str(" running.\n")
	return text.String()
}

type reapOps struct {
	processes func() ([]procscan.Process, error)
	removeDir func(string) error
	remove    func(string) error
}

// Reap removes only dated session directories whose identity has no live
// process, live pid pin, current-session ownership, or recent transcript.
func Reap(root, configDir string, dry bool) (ReapReport, error) {
	scope, err := newReapScope(root, configDir)
	if err != nil {
		return ReapReport{}, err
	}
	if scope.missingRoot {
		return ReapReport{Dry: dry, MissingRoot: true}, nil
	}
	return reap(root, scope.configDir, scope.ownID, dry, reapOps{
		processes: procscan.Scan,
		removeDir: os.RemoveAll,
		remove:    os.Remove,
	})
}

// Judgement is Reap's decision over tmp/session without its removals: every
// dated session directory, and the ones Reap would remove. Notice is set, and
// Dead is empty, when Reap would remove nothing because it cannot tell an idle
// session from a dead one.
type Judgement struct {
	Dirs        []string
	Dead        []string
	Notice      string
	MissingRoot bool
}

// Judge answers the session directories Reap's rules would remove, and
// removes nothing. It takes the process table from the caller, so a caller
// that also judges other stores by liveness scans the processes once
// (internal/le/scratch, the store trim). The rules are Reap's own: judge is
// the one copy both entries call.
func Judge(root, configDir string, processes []procscan.Process) (Judgement, error) {
	scope, err := newReapScope(root, configDir)
	if err != nil {
		return Judgement{}, err
	}
	if scope.missingRoot {
		return Judgement{MissingRoot: true}, nil
	}
	decision, err := judge(root, scope.configDir, scope.ownID, processes)
	if err != nil {
		return Judgement{}, err
	}
	judgement := Judgement{Notice: decision.notice, MissingRoot: decision.missingRoot}
	for _, path := range decision.candidates {
		judgement.Dirs = append(judgement.Dirs, path)
	}
	for _, path := range decision.dead {
		judgement.Dead = append(judgement.Dead, path)
	}
	slices.Sort(judgement.Dirs)
	slices.Sort(judgement.Dead)
	return judgement, nil
}

// reapScope is what both Reap and Judge resolve before judging: this
// process's own session id, which is always live, and the Claude config
// directory whose transcripts keep an idle session alive.
type reapScope struct {
	ownID       string
	configDir   string
	missingRoot bool
}

func newReapScope(root, configDir string) (reapScope, error) {
	sessionRoot := filepath.Join(root, filepath.FromSlash(sessionRootRel))
	info, err := os.Lstat(sessionRoot)
	if errors.Is(err, os.ErrNotExist) {
		return reapScope{missingRoot: true}, nil
	}
	if err != nil {
		return reapScope{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return reapScope{}, fmt.Errorf("session-reap: refusing unsafe session root %s", sessionRoot)
	}
	paths, err := lepath.ResolveSession(root, false)
	if err != nil {
		return reapScope{}, err
	}
	if configDir == "" {
		configDir = os.Getenv("CLAUDE_CONFIG_DIR")
	}
	if configDir == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return reapScope{}, homeErr
		}
		configDir = filepath.Join(home, ".claude")
	}
	return reapScope{ownID: paths.ID, configDir: configDir}, nil
}

// reapDecision is judge's answer: every candidate directory and the dead ones,
// keyed by session id, the pid pins whose process is gone, and the notice that
// stops every removal.
type reapDecision struct {
	candidates  map[string]string
	dead        map[string]string
	stalePins   []string
	notice      string
	missingRoot bool
}

func reap(root, configDir, ownID string, dry bool, ops reapOps) (ReapReport, error) {
	report := ReapReport{Dry: dry}
	processes, err := ops.processes()
	if err != nil {
		return report, err
	}
	decision, err := judge(root, configDir, ownID, processes)
	if err != nil {
		return report, err
	}
	if decision.missingRoot {
		report.MissingRoot = true
		return report, nil
	}
	if decision.notice != "" {
		report.Notice = decision.notice
		return report, nil
	}
	sessionRoot := filepath.Join(root, filepath.FromSlash(sessionRootRel))
	dead := decision.dead
	markers := append(flatMarkers(sessionRoot, dead), decision.stalePins...)
	paths := make([]string, 0, len(dead)+len(markers))
	for _, path := range dead {
		paths = append(paths, path)
	}
	paths = append(paths, markers...)
	slices.Sort(paths)
	report.Paths = paths
	report.RemovedDirs = len(dead)
	report.Kept = len(decision.candidates) - report.RemovedDirs
	if !dry {
		for _, path := range dead {
			_ = ops.removeDir(path)
		}
	}
	for _, marker := range markers {
		if dry {
			report.RemovedMarkers++
			continue
		}
		if err := ops.remove(marker); err == nil {
			report.RemovedMarkers++
		}
	}
	return report, nil
}

// judge applies Reap's liveness rules to tmp/session under root and removes
// nothing.
func judge(root, configDir, ownID string, processes []procscan.Process) (reapDecision, error) {
	var decision reapDecision
	sessionRoot := filepath.Join(root, filepath.FromSlash(sessionRootRel))
	rootInfo, err := os.Lstat(sessionRoot)
	if errors.Is(err, os.ErrNotExist) {
		decision.missingRoot = true
		return decision, nil
	}
	if err != nil {
		return decision, err
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return decision, fmt.Errorf("session-reap: refusing unsafe session root %s", sessionRoot)
	}
	entries, err := os.ReadDir(sessionRoot)
	if err != nil {
		return decision, err
	}
	candidates := make(map[string]string)
	for _, entry := range entries {
		sid, ok := candidateSID(entry.Name())
		if !ok || !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		candidates[sid] = filepath.Join(sessionRoot, entry.Name())
	}
	decision.candidates = candidates
	live := map[string]bool{}
	if ownID != "" {
		live[ownID] = true
	}
	pins, stalePins := pinnedSessions(sessionRoot, processes)
	decision.stalePins = stalePins
	for sid := range pins {
		live[sid] = true
	}
	var argv textbuf.Buffer
	argv.Reset()
	var oldest time.Time
	cliRunning := false
	for _, process := range processes {
		for _, argument := range process.Argv {
			argv.Str(argument).Byte(0)
		}
		argv.Byte('\n')
		if process.CLI {
			cliRunning = true
			if oldest.IsZero() || process.StartedAt.Before(oldest) {
				oldest = process.StartedAt
			}
		}
	}
	argvText := argv.String()
	for sid := range candidates {
		if strings.Contains(argvText, sid) {
			live[sid] = true
		}
	}
	if cliRunning {
		projects := filepath.Join(configDir, "projects")
		info, statErr := os.Stat(projects)
		projectsUsable := statErr == nil && info.IsDir()
		if !projectsUsable {
			decision.notice = fmt.Sprintf("session-reap: a Claude CLI is running but %s does not exist, so an idle session cannot be told from a dead one. Removed nothing.", projects)
			return decision, nil
		}
		transcripts, globErr := filepath.Glob(filepath.Join(projects, "*", "*.jsonl"))
		if globErr != nil {
			return decision, globErr
		}
		for _, transcript := range transcripts {
			info, statErr := os.Stat(transcript)
			if statErr == nil && !info.ModTime().Before(oldest) {
				live[strings.TrimSuffix(filepath.Base(transcript), ".jsonl")] = true
			}
		}
	}
	decision.dead = make(map[string]string)
	for sid, path := range candidates {
		if !live[sid] {
			decision.dead[sid] = path
		}
	}
	return decision, nil
}

func candidateSID(name string) (string, bool) {
	if len(name) < len("2000-01-01-")+1 || name[4] != '-' || name[7] != '-' || name[10] != '-' {
		return "", false
	}
	for _, index := range []int{0, 1, 2, 3, 5, 6, 8, 9} {
		if name[index] < '0' || name[index] > '9' {
			return "", false
		}
	}
	sid := name[11:]
	if !safeReapSID(sid) {
		return "", false
	}
	return sid, true
}

func safeReapSID(sid string) bool {
	if sid == "" || sid == "." || sid == ".." {
		return false
	}
	for _, character := range sid {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '.' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func pinnedSessions(root string, processes []procscan.Process) (map[string]bool, []string) {
	starts := make(map[int]string, len(processes))
	for _, process := range processes {
		starts[process.PID] = process.Start
	}
	entries, _ := os.ReadDir(root)
	live := map[string]bool{}
	var stale []string
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, ".sid-by-pid-") || strings.HasSuffix(name, ".tmp") || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		remainder := strings.TrimPrefix(name, ".sid-by-pid-")
		pidText, start, found := strings.Cut(remainder, "-")
		pid, parseErr := strconv.Atoi(pidText)
		path := filepath.Join(root, name)
		if !found || parseErr != nil || start == "" || starts[pid] != start {
			stale = append(stale, path)
			continue
		}
		content, readErr := os.ReadFile(path) //nolint:gosec // a session state file this walk found under the checkout
		if readErr != nil {
			continue
		}
		sid := strings.TrimSpace(strings.SplitN(string(content), "\n", 2)[0])
		if safeReapSID(sid) {
			live[sid] = true
		}
	}
	slices.Sort(stale)
	return live, stale
}

func flatMarkers(root string, dead map[string]string) []string {
	entries, _ := os.ReadDir(root)
	var markers []string
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || strings.HasPrefix(entry.Name(), ".sid-by-pid-") {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		for sid := range dead {
			if lepath.MarkerOwnedBy(entry.Name(), sid) {
				markers = append(markers, filepath.Join(root, entry.Name()))
				break
			}
		}
	}
	slices.Sort(markers)
	return markers
}

// snapshotKind is the summary block kind the reaper keeps.
const snapshotKind = "snapshot"
