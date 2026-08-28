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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/lepath"
)

const sessionRootRel = "tmp/session"

const processQueryTimeout = 5 * time.Second

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
	var text strings.Builder
	if r.Dry {
		for _, path := range r.Paths {
			text.WriteString(path)
			text.WriteByte('\n')
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
	fmt.Fprintf(&text, "session-reap: %s %d dead session %s and %d %s; kept %d running.\n",
		verb, r.RemovedDirs, directory, r.RemovedMarkers, marker, r.Kept)
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
	var argv strings.Builder
	var oldest time.Time
	cliRunning := false
	for _, process := range processes {
		for _, argument := range process.Argv {
			argv.WriteString(argument)
			argv.WriteByte(0)
		}
		argv.WriteByte('\n')
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
		if statErr != nil || !info.IsDir() {
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
		content, readErr := os.ReadFile(path)
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
	entries, err := os.ReadDir("/proc")
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
	base := filepath.Join("/proc", strconv.Itoa(pid))
	stat, err := os.ReadFile(filepath.Join(base, "stat"))
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
	cmdline, _ := os.ReadFile(filepath.Join(base, "cmdline"))
	parts := bytes.Split(cmdline, []byte{0})
	argv := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) != 0 {
			argv = append(argv, string(part))
		}
	}
	comm, _ := os.ReadFile(filepath.Join(base, "comm"))
	name := strings.TrimSpace(string(comm))
	cli := filepath.Base(name) == "claude"
	if !cli {
		for _, argument := range argv {
			for _, component := range strings.Split(argument, "/") {
				if component == "claude" {
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
	output, err := exec.CommandContext(ctx, "ps", "-o", "etimes=", "-p", strconv.Itoa(pid)).Output() //nolint:gosec // fixed process query
	if err != nil {
		return time.Time{}
	}
	seconds, err := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Now().Add(-time.Duration(seconds) * time.Second)
}

func scanProcessesWithPS() ([]processFact, error) {
	ctx, cancel := context.WithTimeout(context.Background(), processQueryTimeout)
	defer cancel()
	output, err := exec.CommandContext(ctx, "ps", "-eo", "pid=,lstart=,etimes=,comm=,args=").Output()
	if err != nil {
		return nil, err
	}
	var processes []processFact
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}
		pid, pidErr := strconv.Atoi(fields[0])
		age, ageErr := strconv.ParseInt(fields[6], 10, 64)
		if pidErr != nil || ageErr != nil {
			continue
		}
		start := processPathToken(strings.Join(fields[1:6], " "))
		argv := fields[8:]
		cli := filepath.Base(fields[7]) == "claude"
		if !cli {
			for _, argument := range argv {
				for _, component := range strings.Split(argument, "/") {
					if component == "claude" {
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
	var token strings.Builder
	separator := false
	for _, character := range strings.TrimSpace(value) {
		safe := character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '.' || character == '_' || character == '-'
		if safe {
			if separator {
				token.WriteByte('_')
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
