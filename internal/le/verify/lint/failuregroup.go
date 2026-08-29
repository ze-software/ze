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

	"github.com/ze-software/ze/internal/le/verify/failuregroup"
)

// pathCollector tees a child's output, keeping the distinct Go files its
// findings name. It writes nothing of its own: the output still streams to the
// operator exactly as before, and this only watches it go past.
type pathCollector struct {
	seen    []string
	partial []byte
}

func newPathCollector() *pathCollector { return &pathCollector{} }

// Write scans one chunk of child output. A finding split across two writes is
// handled by carrying the unterminated tail into the next call, so a path is
// never missed because of where the pipe happened to break.
func (c *pathCollector) Write(p []byte) (int, error) {
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

// paths answers the collected files.
func (c *pathCollector) paths() []string { return c.seen }

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
