// Design: docs/architecture/testing/interop.md -- ze against a real peer
// Related: l2tp.go -- the first proof built on what this file holds
//
// Package deployment proves ze against software somebody else wrote. Each
// action here starts a real peer daemon in a container, points ze at it, and
// answers whether the protocol did what the RFC says it does. A stub proves
// that ze agrees with ze; only another implementation proves interoperability
// (ai/rules/interop-and-goal-validation.md).
//
// This file holds what every such proof needs and none of what any one of them
// is about: the daemon build, the container, the collector that watches a
// process's output for the lines that decide the verdict, and the stop path
// that leaves no container behind.
//
// Every external command is reached through PATH rather than through an
// injected seam. The proof IS the argv that reaches docker, so a test that
// replaced docker with a function would be testing a different program; the
// tests put a recording docker on PATH instead, and the Python original is
// compared through the same recording (scripts/evidence/l2tp_parity_test.go).
package deployment

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// LogTailLines is how many of a daemon's last output lines a failed run
// reports.
//
// The tail is bounded because the daemon runs with debug logging on: an
// unbounded log would put a whole session's output into a JSON document, and
// the lines that explain a failure are the last ones. 80 is what the Python
// original printed.
const LogTailLines = 80

// pollInterval is how often a wait re-reads what the collector has seen. The
// events being waited for are a listener binding and a tunnel forming, both of
// which take hundreds of milliseconds at least, so a shorter interval would
// only spin.
const pollInterval = 200 * time.Millisecond

// stopGrace is how long a process gets to exit after it is asked to. Each of
// these processes is a docker client with nothing to flush, so the grace is
// about letting it close its connection rather than about saving work.
const stopGrace = 3 * time.Second

// The two bounds on a docker query. An inspect or a remove talks to a local
// daemon and answers in milliseconds, so a minute means the daemon is not
// answering; a pull reaches a registry, so it gets the longer one.
const (
	inspectTimeout = time.Minute
	pullTimeout    = 15 * time.Minute
)

// killGrace is how long a killed process gets to be reaped before the run stops
// waiting for it. A process that survives SIGKILL is stuck in the kernel, and
// nothing this tool does will change that.
const killGrace = 2 * time.Second

// collector watches one process's output for the lines that decide a verdict,
// and keeps a bounded tail of everything it saw.
//
// The needles are declared up front rather than searched for later, which is
// what lets the tail be bounded: a line that decides the verdict is marked as
// it arrives, so it does not have to survive in a buffer until somebody asks.
//
// Safe for concurrent use.
type collector struct {
	mu    sync.Mutex
	tail  []string
	marks map[string]bool
	kept  int
	wg    sync.WaitGroup
}

// newCollector answers a collector watching for each needle.
func newCollector(needles ...string) *collector {
	marks := make(map[string]bool, len(needles))
	for _, needle := range needles {
		marks[needle] = false
	}
	return &collector{marks: marks, tail: make([]string, LogTailLines)}
}

// add records one line: it marks every needle the line carries, and it joins
// the bounded tail.
func (c *collector) add(line string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for needle := range c.marks {
		if !c.marks[needle] && strings.Contains(line, needle) {
			c.marks[needle] = true
		}
	}

	c.tail[c.kept%LogTailLines] = line
	c.kept++
}

// saw reports whether any line carried needle. A needle the collector was not
// built to watch is never seen, because nothing marked it.
func (c *collector) saw(needle string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.marks[needle]
}

// tail answers the last lines, oldest first, at most LogTailLines of them.
func (c *collector) tailLines() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := min(c.kept, LogTailLines)
	lines := make([]string, 0, count)
	for i := c.kept - count; i < c.kept; i++ {
		lines = append(lines, c.tail[i%LogTailLines])
	}
	return lines
}

// lineMax bounds one line. A longer one is delivered in pieces rather than
// buffered, so no daemon can make this tool hold an arbitrary amount of memory.
const lineMax = 64 * 1024

// stream starts the goroutine that reads r into the collector, echoing each
// line to progress with prefix in front of it.
//
// The goroutine ends when r reaches its end, which happens when the process
// holding the other end exits. The caller MUST call wait after the process has
// been stopped.
func (c *collector) stream(prefix string, r io.Reader, progress io.Writer) {
	c.wg.Go(func() {
		// ReadSlice rather than a Scanner: a line longer than the buffer
		// arrives in pieces with ErrBufferFull, where a Scanner would end the
		// scan and lose the rest of the stream. Losing a daemon's output
		// because one of its lines was long is the wrong answer for a tool
		// whose job is to report what the daemon said.
		reader := bufio.NewReaderSize(r, lineMax)
		for {
			chunk, err := reader.ReadSlice('\n')
			if line := strings.TrimRight(string(chunk), "\n"); line != "" {
				c.add(line)
				if progress != nil {
					var tb textbuf.Buffer
					io.WriteString(progress, tb.Str(prefix).Str(line).Byte('\n').String()) //nolint:errcheck // progress output
				}
			}
			if err != nil && !errors.Is(err, bufio.ErrBufferFull) {
				return
			}
		}
	})
}

// wait blocks until every stream this collector started has ended. It MUST be
// called after the processes writing to it have been stopped, or it does not
// return.
func (c *collector) wait() { c.wg.Wait() }

// running is one started process, with the goroutine that reaps it and the
// goroutine that reads its output.
//
// stop MUST be called for every running a caller starts, and wait on the
// collector MUST come after it.
type running struct {
	cmd  *exec.Cmd
	done chan struct{}
	read *os.File
}

// startWatched starts cmd with BOTH its output streams going into seen, and
// answers the handle the caller stops.
//
// The pipe is made here rather than by StderrPipe, because an *os.File is
// handed to the child directly: Wait then answers as soon as the child exits,
// rather than waiting for a copy this package would have to drain first.
//
// Standard output joins the same pipe rather than being left unread. A pipe
// nobody reads fills at 64 KB and the child then blocks in write for ever,
// which is a hang the run reports as a timeout on a daemon that was working.
// The Python original left ze's stdout in exactly that state.
func startWatched(cmd *exec.Cmd, prefix string, seen *collector, progress io.Writer) (*running, error) {
	read, write, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = write
	if cmd.Stdout == nil {
		cmd.Stdout = write
	}

	if err := cmd.Start(); err != nil {
		read.Close()  //nolint:errcheck // the start already failed
		write.Close() //nolint:errcheck // the start already failed
		return nil, err
	}
	// The child holds its own descriptor now. Closing the parent's copy is
	// what lets the reader see the end of the stream when the child exits.
	write.Close() //nolint:errcheck // the child holds the other end

	seen.stream(prefix, read, progress)

	proc := &running{cmd: cmd, done: make(chan struct{}), read: read}
	go func() {
		defer close(proc.done)
		cmd.Wait() //nolint:errcheck // the exit status is read through exited, not here
	}()

	return proc, nil
}

// exited reports whether the process has ended.
func (r *running) exited() bool {
	select {
	case <-r.done:
		return true
	default:
		return false
	}
}

// stop ends the process and releases the pipe. It MUST be called for every
// process startWatched started, and it is safe to call for one that has already
// exited.
func (r *running) stop() {
	defer r.read.Close() //nolint:errcheck // the stream is finished either way

	if r.exited() {
		return
	}
	r.cmd.Process.Signal(syscall.SIGTERM) //nolint:errcheck // a process that refuses SIGTERM is killed below

	select {
	case <-r.done:
		return
	case <-time.After(stopGrace):
	}

	r.cmd.Process.Kill() //nolint:errcheck // nothing further can be done about a process that survives this

	select {
	case <-r.done:
	case <-time.After(killGrace):
	}
}

// await waits for seen to report needle, and answers whether it arrived.
//
// It gives up early when the process being watched has exited, because nothing
// more will be written after that, and a run that waited out the whole deadline
// on a dead daemon reports the timeout rather than the crash.
func await(seen *collector, needle string, proc *running, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if seen.saw(needle) {
			return true
		}
		if proc.exited() {
			// One last read: the process can exit between the two checks
			// above, with the line already delivered.
			return seen.saw(needle)
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(pollInterval)
	}
}

// look reports every named command that is not on PATH, as one error naming the
// first of them. Checking all before using any means an operator missing two
// learns about the first in one run rather than after a build.
func look(names ...string) error {
	for _, name := range names {
		if _, err := exec.LookPath(name); err != nil {
			var tb textbuf.Buffer
			return errors.New(tb.Str("missing required command: ").Str(name).String())
		}
	}
	return nil
}

// ensureImage pulls image when the local daemon does not already hold it.
//
// The inspect comes first so that a run on a machine with no network still
// works once the image is cached, which is what makes these proofs runnable on
// a laptop.
func ensureImage(image string, progress io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), inspectTimeout)
	defer cancel()

	inspect := exec.CommandContext(ctx, "docker", "image", "inspect", image) //nolint:gosec // the image name is le's own setting
	if err := inspect.Run(); err == nil {
		return nil
	}

	if progress != nil {
		var tb textbuf.Buffer
		io.WriteString(progress, tb.Str("pulling ").Str(image).Str("...\n").String()) //nolint:errcheck // progress output
	}
	pullCtx, pullCancel := context.WithTimeout(context.Background(), pullTimeout)
	defer pullCancel()

	pull := exec.CommandContext(pullCtx, "docker", "pull", image) //nolint:gosec // the image name is le's own setting
	pull.Stdout = progress
	pull.Stderr = progress
	if err := pull.Run(); err != nil {
		var tb textbuf.Buffer
		return errors.New(tb.Str("docker pull ").Str(image).Str(" failed").String())
	}
	return nil
}

// removeContainer takes the container down. Its failure is not reported: the
// run is already over by the time this is called, and the reason the run ended
// is what the caller must be told about.
func removeContainer(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), inspectTimeout)
	defer cancel()

	exec.CommandContext(ctx, "docker", "rm", "-f", name).Run() //nolint:errcheck,gosec // cleanup; the run's own verdict is what the caller reads
}

// scratchDir answers a fresh directory under the tree for one run's files, and
// the tree-relative parent it was made in.
//
// It lives under the checkout rather than in the system temporary directory
// because it is bind-mounted into a container, and a Docker daemon in a VM --
// which is every macOS install -- shares the checkout and nothing else.
func scratchDir(tree, prefix string) (string, error) {
	parent := filepath.Join(tree, "tmp", "evidence")
	if err := os.MkdirAll(parent, 0o750); err != nil {
		return "", err
	}
	return os.MkdirTemp(parent, prefix)
}
