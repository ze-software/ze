// Design: docs/architecture/diagnostics/procfs-diagnostics.md -- non-destructive host-side plugin process inspection
// Related: process.go -- Process lifecycle, PID tracking, and stderr relay
// Related: ../../../core/procfs/process.go -- /proc/<pid> inspection types and reader

package process

import (
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/procfs"
)

const (
	stderrTailMaxLines = 32
	stderrTailMaxBytes = 256
)

// stderrTailRing is a bounded per-process circular buffer of recent stderr
// lines (at most 32 lines x 256 bytes = 8 KiB per Process).
// Protected by its own leaf mutex; never acquires Process.mu.
type stderrTailRing struct {
	mu    sync.Mutex
	lines [stderrTailMaxLines]string
	next  int
	count int
}

// Add appends line to the ring, truncating cleanly to at most 256 UTF-8 bytes.
func (r *stderrTailRing) Add(line string) {
	clipped := truncateStderrLine(line)
	r.mu.Lock()
	r.lines[r.next] = clipped
	r.next = (r.next + 1) % stderrTailMaxLines
	if r.count < stderrTailMaxLines {
		r.count++
	}
	r.mu.Unlock()
}

// Snapshot returns the buffered lines in chronological order (oldest to newest).
// Returns a non-nil empty slice when no lines have been captured.
func (r *stderrTailRing) Snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.count == 0 {
		return []string{}
	}
	out := make([]string, r.count)
	start := (r.next - r.count + stderrTailMaxLines) % stderrTailMaxLines
	for i := range r.count {
		out[i] = r.lines[(start+i)%stderrTailMaxLines]
	}
	return out
}

func truncateStderrLine(line string) string {
	if len(line) <= stderrTailMaxBytes {
		return line
	}
	cut := stderrTailMaxBytes
	for cut > 0 && !utf8.RuneStart(line[cut]) {
		cut--
	}
	return strings.Clone(line[:cut])
}

// HostProcessSnapshot is a strictly read-only, non-destructive host-side view
// of a plugin process. Collecting it never sends any OS signal to the child.
type HostProcessSnapshot struct {
	PID        int                    `json:"pid"`
	Running    bool                   `json:"running"`
	Stage      string                 `json:"stage"`
	External   bool                   `json:"-"`
	Proc       *procfs.ProcessInspect `json:"host,omitempty"`
	ProcError  string                 `json:"procfs-error,omitempty"`
	StderrTail []string               `json:"stderr-tail"`
}

// PID returns the live OS PID of a forked plugin process, or 0 for in-daemon
// plugins and forked processes that have not started or have already exited.
func (p *Process) PID() int {
	return int(p.pid.Load())
}

// IsExternal reports whether this process runs as a forked OS child process
// (either a foreign external command or a separate-process compiled plugin).
func (p *Process) IsExternal() bool {
	return p.isExternal.Load()
}

// HostInspect collects a non-destructive host-side diagnostic snapshot of p
// without sending any signal to the child process.
func (p *Process) HostInspect() HostProcessSnapshot {
	pid := p.PID()
	external := p.IsExternal()
	snap := HostProcessSnapshot{
		PID:        pid,
		Running:    p.Running(),
		Stage:      p.Stage().String(),
		External:   external,
		StderrTail: p.stderrTail.Snapshot(),
	}
	if pid > 0 {
		info, err := procfs.ReadProcessInspect(pid)
		if err != nil {
			snap.ProcError = err.Error()
		} else {
			snap.Proc = &info
		}
	} else if external {
		snap.ProcError = "process has no live OS pid"
	}
	return snap
}
