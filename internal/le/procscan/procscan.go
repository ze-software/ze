// Design: docs/architecture/core-design.md -- process liveness the session reaper and the lab cleanup read
// Related: ../session/reap.go -- the session-liveness rules over this scan.
// Related: ../interoplab/orphans.go -- the lab cleanup's owner verdict over this scan.

// Package procscan answers the running processes of this host, each with a
// start token that changes when a pid is reused. It is a leaf: it registers no
// action, so the session reaper and the interop lab cleanup both import it.
package procscan

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const processQueryTimeout = 5 * time.Second

// procRoot is the Linux process filesystem this scanner reads.
const procRoot = "/proc"

// claudeProcess is the process name the CLI flag recognizes.
const claudeProcess = "claude"

// Process is one running process as this scanner sees it. It is the
// one process fact every session-liveness judgement reads: the session reaper's rules,
// the store trim's launcher and testbin checks in internal/le/scratch, and the interop lab
// cleanup's owner verdict.
//
// Start is an opaque token that changes when a pid is reused, and is what a
// pid pin records. StartedAt is the wall-clock start: the ps scan fills it for
// every process, the /proc scan only for a Claude CLI, because it costs a ps
// call per process there. A zero StartedAt means "not measured", never "the
// epoch"; StartTime measures one pid on demand.
type Process struct {
	PID       int
	Start     string
	StartedAt time.Time
	Argv      []string
	CLI       bool
}

// Scan answers every running process: /proc where it exists, else
// one `ps` call. An exited process whose pid is not yet reaped (a zombie) is
// not running and is left out. It is the only process scanner the session-liveness rules use.
func Scan() ([]Process, error) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return scanProcessesWithPS()
	}
	processes := make([]Process, 0, len(entries))
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		fact, state := procFact(pid)
		if state == procRunning {
			processes = append(processes, fact)
		}
	}
	return processes, nil
}

// procState is what one /proc/<pid>/stat read answers.
type procState uint8

const (
	// procUnreadable: the stat file did not read or did not parse. It is the
	// zero value, so a forgotten assignment reads as "unknown", never as an answer.
	procUnreadable procState = iota
	// procRunning: the pid holds a process that has not exited.
	procRunning
	// procExited: the process has exited and only its unreaped pid entry is
	// left, a zombie (Z) or dead (X). It owns nothing any more, so every
	// liveness judgement reads it as absent: an orphan sweep that kept a
	// killed `go test` child's lab resources until its parent reaped it would
	// leave them for the next sweep.
	procExited
)

func procFact(pid int) (Process, procState) {
	base := filepath.Join(procRoot, strconv.Itoa(pid))
	stat, err := os.ReadFile(filepath.Join(base, "stat")) //nolint:gosec // a numeric pid directory under /proc
	if err != nil {
		return Process{}, procUnreadable
	}
	start, exited, ok := parseProcStat(stat)
	if !ok {
		return Process{}, procUnreadable
	}
	if exited {
		return Process{}, procExited
	}
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
		startedAt = StartTime(pid)
	}
	return Process{PID: pid, Start: start, StartedAt: startedAt, Argv: argv, CLI: cli}, procRunning
}

// parseProcStat reads the state and the start token out of one
// /proc/<pid>/stat line. The comm field is parenthesized and may itself hold
// spaces and parentheses, so the fields are counted from the last ')'. exited
// is true for a zombie (Z) or a dead (X, or x before Linux 3.13) process.
func parseProcStat(stat []byte) (start string, exited, ok bool) {
	_, remainder, found := bytes.CutLast(stat, []byte{')'})
	if !found {
		return "", false, false
	}
	fields := bytes.Fields(remainder)
	if len(fields) <= 19 {
		return "", false, false
	}
	return string(fields[19]), exitedState(string(fields[0])), true
}

// exitedState answers whether a process state, as /proc or `ps -o state=`
// prints it, is a process that has exited but whose pid is not yet reaped.
// ps may append flags (`Z+`), so only the first letter decides.
func exitedState(state string) bool {
	if strings.HasPrefix(state, "Z") {
		return true
	}
	return strings.HasPrefix(state, "X") || strings.HasPrefix(state, "x")
}

// StartTime measures one process's wall-clock start from its `ps`
// elapsed time, to the second. It answers the zero time when the process is
// gone or ps cannot say, and a caller MUST read that zero as "unknown".
func StartTime(pid int) time.Time {
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
	if len(parts) < 2 {
		return 0, false
	}
	if len(parts) > 3 {
		return 0, false
	}
	total := days * 24 * 60 * 60

	// Right to left, so the same loop reads mm:ss and hh:mm:ss.
	multiplier := int64(1)
	for _, part := range slices.Backward(parts) {
		unit, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil {
			return 0, false
		}
		if unit < 0 {
			return 0, false
		}
		total += unit * multiplier
		multiplier *= 60
	}
	return total, true
}

func scanProcessesWithPS() ([]Process, error) {
	ctx, cancel := context.WithTimeout(context.Background(), processQueryTimeout)
	defer cancel()
	output, err := exec.CommandContext(ctx, "ps", "-eo", "pid=,state=,lstart=,etime=,comm=,args=").Output()
	if err != nil {
		return nil, err
	}
	return parsePS(string(output)), nil
}

// parsePS reads the output of `ps -eo pid=,state=,lstart=,etime=,comm=,args=`.
// lstart prints five words, so etime is the eighth field and comm the ninth.
// A zombie or dead line is left out, as procExited is on the /proc path.
func parsePS(output string) []Process {
	var processes []Process
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		pid, pidErr := strconv.Atoi(fields[0])
		age, ageOK := elapsedSeconds(fields[7])
		if pidErr != nil {
			continue
		}
		if !ageOK {
			continue
		}
		if exitedState(fields[1]) {
			continue
		}
		start := processPathToken(strings.Join(fields[2:7], " "))
		argv := fields[9:]
		cli := filepath.Base(fields[8]) == claudeProcess
		if !cli {
			for _, argument := range argv {
				for component := range strings.SplitSeq(argument, "/") {
					if component == claudeProcess {
						cli = true
					}
				}
			}
		}
		processes = append(processes, Process{
			PID: pid, Start: start, StartedAt: time.Now().Add(-time.Duration(age) * time.Second),
			Argv: argv, CLI: cli,
		})
	}
	return processes
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

// Lookup answers one pid. found is false only when the host answered and no
// running process holds that pid: a pid held by an exited, unreaped process
// (a zombie) is absent too, because that process owns nothing any more. An error means the host could not be asked, and a
// caller MUST read it as "unknown", never as "absent": the lab cleanup removes
// a resource whose owner is absent, so a failed question read as absence would
// remove a live run's containers.
func Lookup(pid int) (Process, bool, error) {
	if _, err := os.Stat(procRoot); err != nil {
		return lookupWithPS(pid)
	}
	base := filepath.Join(procRoot, strconv.Itoa(pid))
	if _, err := os.Stat(base); err != nil {
		if os.IsNotExist(err) {
			return Process{}, false, nil
		}
		return Process{}, false, err
	}
	fact, state := procFact(pid)
	switch state {
	case procRunning:
		return fact, true, nil
	case procExited:
		return Process{}, false, nil
	case procUnreadable:
		// The directory exists but its stat did not read: the process may
		// have exited between the two calls, or /proc refuses this reader.
		// Only a second look that finds the directory gone is an absence.
		if _, err := os.Stat(base); os.IsNotExist(err) {
			return Process{}, false, nil
		}
		var problem textbuf.Buffer
		return Process{}, false, errors.New(problem.Str("the start token of pid ").Int(int64(pid)).
			Str(" is unreadable under ").Str(procRoot).String())
	}
	panic("BUG: procscan: a procState with no case in Lookup")
}

func lookupWithPS(pid int) (Process, bool, error) {
	processes, err := scanProcessesWithPS()
	if err != nil {
		return Process{}, false, err
	}
	for _, process := range processes {
		if process.PID == pid {
			return process, true, nil
		}
	}
	return Process{}, false, nil
}
