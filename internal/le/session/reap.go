// Design: docs/architecture/core-design.md -- conservative cleanup of ended sessions
package session

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/lepath"
)

const sessionRootRel = "tmp/session"

const processQueryTimeout = 5 * time.Second

// procRoot is the Linux process filesystem this reaper reads.
const procRoot = "/proc"

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

type processFact struct {
	PID       int
	Start     string
	StartedAt time.Time
	Argv      []string
	CLI       bool
}

type reapOps struct {
	processes func() ([]processFact, error)
	removeDir func(string) error
	remove    func(string) error
}

// Reap removes only dated session directories whose identity has no live
// process, live pid pin, current-session ownership, or recent transcript.
func Reap(root, configDir string, dry bool) (ReapReport, error) {
	sessionRoot := filepath.Join(root, filepath.FromSlash(sessionRootRel))
	info, err := os.Lstat(sessionRoot)
	if errors.Is(err, os.ErrNotExist) {
		return ReapReport{Dry: dry, MissingRoot: true}, nil
	}
	if err != nil {
		return ReapReport{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return ReapReport{}, fmt.Errorf("session-reap: refusing unsafe session root %s", sessionRoot)
	}
	paths, err := lepath.ResolveSession(root, false)
	if err != nil {
		return ReapReport{}, err
	}
	if configDir == "" {
		configDir = os.Getenv("CLAUDE_CONFIG_DIR")
	}
	if configDir == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return ReapReport{}, homeErr
		}
		configDir = filepath.Join(home, ".claude")
	}
	return reap(root, configDir, paths.ID, dry, reapOps{
		processes: scanProcesses,
		removeDir: os.RemoveAll,
		remove:    os.Remove,
	})
}

func reap(root, configDir, ownID string, dry bool, ops reapOps) (ReapReport, error) {
	report := ReapReport{Dry: dry}
	sessionRoot := filepath.Join(root, filepath.FromSlash(sessionRootRel))
	rootInfo, err := os.Lstat(sessionRoot)
	if errors.Is(err, os.ErrNotExist) {
		report.MissingRoot = true
		return report, nil
	}
	if err != nil {
		return report, err
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return report, fmt.Errorf("session-reap: refusing unsafe session root %s", sessionRoot)
	}
	entries, err := os.ReadDir(sessionRoot)
	if err != nil {
		return report, err
	}
	candidates := make(map[string]string)
	for _, entry := range entries {
		sid, ok := candidateSID(entry.Name())
		if !ok || !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		candidates[sid] = filepath.Join(sessionRoot, entry.Name())
	}
	processes, err := ops.processes()
	if err != nil {
		return report, err
	}
	live := map[string]bool{}
	if ownID != "" {
		live[ownID] = true
	}
	pins, stalePins := pinnedSessions(sessionRoot, processes)
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
			report.Notice = fmt.Sprintf("session-reap: a Claude CLI is running but %s does not exist, so an idle session cannot be told from a dead one. Removed nothing.", projects)
			return report, nil
		}
		transcripts, globErr := filepath.Glob(filepath.Join(projects, "*", "*.jsonl"))
		if globErr != nil {
			return report, globErr
		}
		for _, transcript := range transcripts {
			info, statErr := os.Stat(transcript)
			if statErr == nil && !info.ModTime().Before(oldest) {
				live[strings.TrimSuffix(filepath.Base(transcript), ".jsonl")] = true
			}
		}
	}
	dead := make(map[string]string)
	for sid, path := range candidates {
		if !live[sid] {
			dead[sid] = path
		}
	}
	markers := append(flatMarkers(sessionRoot, dead), stalePins...)
	paths := make([]string, 0, len(dead)+len(markers))
	for _, path := range dead {
		paths = append(paths, path)
	}
	paths = append(paths, markers...)
	sort.Strings(paths)
	report.Paths = paths
	report.RemovedDirs = len(dead)
	report.Kept = len(candidates) - report.RemovedDirs
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

func pinnedSessions(root string, processes []processFact) (map[string]bool, []string) {
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
	sort.Strings(stale)
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
			if strings.HasSuffix(entry.Name(), "-"+sid) {
				markers = append(markers, filepath.Join(root, entry.Name()))
				break
			}
		}
	}
	sort.Strings(markers)
	return markers
}

func scanProcesses() ([]processFact, error) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return scanProcessesWithPS()
	}
	processes := make([]processFact, 0, len(entries))
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		fact, ok := procFact(pid)
		if ok {
			processes = append(processes, fact)
		}
	}
	return processes, nil
}

func procFact(pid int) (processFact, bool) {
	base := filepath.Join(procRoot, strconv.Itoa(pid))
	stat, err := os.ReadFile(filepath.Join(base, "stat")) //nolint:gosec // a numeric pid directory under /proc
	if err != nil {
		return processFact{}, false
	}
	closing := bytes.LastIndexByte(stat, ')')
	if closing < 0 {
		return processFact{}, false
	}
	fields := bytes.Fields(stat[closing+1:])
	if len(fields) <= 19 {
		return processFact{}, false
	}
	start := string(fields[19])
	cmdline, _ := os.ReadFile(filepath.Join(base, "cmdline")) //nolint:gosec // a numeric pid directory under /proc
	parts := bytes.Split(cmdline, []byte{0})
	argv := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) != 0 {
			argv = append(argv, string(part))
		}
	}
	comm, _ := os.ReadFile(filepath.Join(base, "comm")) //nolint:gosec // a numeric pid directory under /proc
	name := strings.TrimSpace(string(comm))
	cli := filepath.Base(name) == claudeProcess
	if !cli {
		for _, argument := range argv {
			for component := range strings.SplitSeq(argument, "/") {
				if component == claudeProcess {
					cli = true
				}
			}
		}
	}
	startedAt := time.Time{}
	if cli {
		startedAt = processStartTime(pid)
	}
	return processFact{PID: pid, Start: start, StartedAt: startedAt, Argv: argv, CLI: cli}, true
}

func processStartTime(pid int) time.Time {
	ctx, cancel := context.WithTimeout(context.Background(), processQueryTimeout)
	defer cancel()
	output, err := exec.CommandContext(ctx, "ps", "-o", "etime=", "-p", strconv.Itoa(pid)).Output() //nolint:gosec // fixed process query
	if err != nil {
		return time.Time{}
	}
	seconds, ok := elapsedSeconds(string(output))
	if !ok {
		return time.Time{}
	}
	return time.Now().Add(-time.Duration(seconds) * time.Second)
}

// elapsedSeconds reads the portable ps elapsed-time field, [[dd-]hh:]mm:ss.
//
// The field is `etime`, not `etimes`. Only Linux ps offers `etimes`, the whole
// age in seconds; BSD and macOS refuse the keyword outright and exit non-zero,
// which took the entire process scan with it. So the reaper saw no processes,
// judged no session either dead or running, and reported nothing to remove over
// eleven live session directories.
func elapsedSeconds(value string) (int64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}

	var days int64
	if before, after, found := strings.Cut(value, "-"); found {
		parsed, err := strconv.ParseInt(before, 10, 64)
		if err != nil {
			return 0, false
		}
		days, value = parsed, after
	}

	parts := strings.Split(value, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	total := days * 24 * 60 * 60

	// Right to left, so the same loop reads mm:ss and hh:mm:ss.
	multiplier := int64(1)
	for _, part := range slices.Backward(parts) {
		unit, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil || unit < 0 {
			return 0, false
		}
		total += unit * multiplier
		multiplier *= 60
	}
	return total, true
}

func scanProcessesWithPS() ([]processFact, error) {
	ctx, cancel := context.WithTimeout(context.Background(), processQueryTimeout)
	defer cancel()
	output, err := exec.CommandContext(ctx, "ps", "-eo", "pid=,lstart=,etime=,comm=,args=").Output()
	if err != nil {
		return nil, err
	}
	var processes []processFact
	for line := range strings.SplitSeq(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}
		pid, pidErr := strconv.Atoi(fields[0])
		age, ageOK := elapsedSeconds(fields[6])
		if pidErr != nil || !ageOK {
			continue
		}
		start := processPathToken(strings.Join(fields[1:6], " "))
		argv := fields[8:]
		cli := filepath.Base(fields[7]) == claudeProcess
		if !cli {
			for _, argument := range argv {
				for component := range strings.SplitSeq(argument, "/") {
					if component == claudeProcess {
						cli = true
					}
				}
			}
		}
		processes = append(processes, processFact{
			PID: pid, Start: start, StartedAt: time.Now().Add(-time.Duration(age) * time.Second),
			Argv: argv, CLI: cli,
		})
	}
	return processes, nil
}

func processPathToken(value string) string {
	var token textbuf.Buffer
	token.Reset()
	separator := false
	for _, character := range strings.TrimSpace(value) {
		safe := character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '.' || character == '_' || character == '-'
		if safe {
			if separator {
				token.Byte('_')
			}
			token.WriteRune(character)
			separator = false
			continue
		}
		if token.Len() != 0 {
			separator = true
		}
	}
	return strings.Trim(token.String(), "_")
}

// The process name this reaper looks for, and the summary block kind it keeps.
const (
	claudeProcess = "claude"
	snapshotKind  = "snapshot"
)
