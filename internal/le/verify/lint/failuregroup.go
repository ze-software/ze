// Related: verifylint.go -- streamCommand, the child whose output is scanned
// Related: ../failuregroup/failuregroup.go -- Paths and Declare, shared with the
// tracked-build stage
//
// A lint red must say WHICH files its findings were about, or the commit gate
// charges it to every commit in the checkout (../../commit/verification.go,
// structuralGateReds). golangci-lint names them; this keeps them.

package verifylint

import (
	"bytes"
	"io"
	"sync"

	"github.com/ze-software/ze/internal/le/verify/failuregroup"
)

// pathCollector tees a child's output, keeping the distinct Go files its
// findings name. It writes nothing of its own: the output still streams to the
// operator exactly as before, and this only watches it go past.
type pathCollector struct {
	// mu guards seen and partial. One collector is the watch half of BOTH
	// cmd.Stdout and cmd.Stderr (verifylint.go), and os/exec copies each pipe
	// on its own goroutine, so two writers reach this one buffer at once.
	// Unguarded, the append and the compaction below interleave and the
	// re-slice reads a length that belonged to the other goroutine's buffer:
	// `slice bounds out of range [32893:70]`, which killed the whole verify
	// run rather than the stage it was watching.
	mu      sync.Mutex
	seen    []string
	partial []byte
}

func newPathCollector() *pathCollector { return &pathCollector{} }

// Write scans one chunk of child output. A finding split across two writes is
// handled by carrying the unterminated tail into the next call, so a path is
// never missed because of where the pipe happened to break.
func (c *pathCollector) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.seen) >= failuregroup.MaxPaths {
		return len(p), nil
	}
	c.partial = append(c.partial, p...)
	cut := bytes.LastIndexByte(c.partial, '\n')
	if cut < 0 {
		return len(p), nil
	}
	c.seen = failuregroup.Merge(c.seen, failuregroup.Paths(string(c.partial[:cut+1])))
	c.partial = c.partial[:copy(c.partial, c.partial[cut+1:])]

	return len(p), nil
}

// paths answers the collected files. It takes the same lock as Write, because
// the caller reads it after the child exits but the copying goroutines are not
// guaranteed to have finished by then.
func (c *pathCollector) paths() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.seen
}

// discardNil answers a writer that is safe to tee into, for a caller that wants
// no attribution.
func discardNil(w io.Writer) io.Writer {
	if w == nil {
		return io.Discard
	}

	return w
}

// declareLintFailureGroup prints the group the verify engine reads back from
// this stage's detail log.
func declareLintFailureGroup(w io.Writer, paths []string) error {
	return failuregroup.Declare(w, "lint:verify lint/run", "lint",
		"golangci-lint reported findings in the files named here",
		"./le verify lint run", paths)
}
