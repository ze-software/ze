// Design: ai/rules/cli.md -- "-" means stdin/stdout across every command
package cliio

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sync/atomic"
)

// StdinToken is the conventional CLI token meaning "stdin when reading, stdout
// when writing". It is the ONLY special token this package interprets: no
// /dev/* handling, no shell expansion.
const StdinToken = "-"

// MaxStdinBytes caps a whole-of-stdin read (ReadFile / WriteFile can stream, but
// ReadFile buffers). It mirrors the offline pipe carrier's cap
// (cmd/ze/ze_core_pipe.go maxStdin) so an unbounded pipe cannot exhaust memory.
// Streaming consumers (OpenReader) are deliberately uncapped: ze analyze replay
// streams multi-GB MRT and must not be forced through a bounded buffer.
const MaxStdinBytes = 256 << 20 // 256 MB

// ErrStdinClaimed is returned when "-" is read a second time in one process.
// stdin is consumable exactly once; a second read would silently return empty,
// which downstream cannot tell from a genuinely empty file
// (ai/rules/evidence.md). Fail closed instead.
var ErrStdinClaimed = errors.New("cliio: stdin (\"-\") already consumed; it can be read at most once per command")

// stdin and stdout are indirections over the process's standard streams so
// tests can substitute buffers. Production code never rebinds them.
var (
	stdin  io.Reader = os.Stdin
	stdout io.Writer = os.Stdout
)

// stdinClaimed records whether "-" has been read in this process. It is set by
// the first ReadFile("-")/OpenReader("-") and blocks any later claim.
var stdinClaimed atomic.Bool

// IsStdin reports whether path is the stdin/stdout token "-". A file literally
// named "-" is addressed with the conventional escape "./-".
func IsStdin(path string) bool { return path == StdinToken }

// SwapStreams replaces the stdin/stdout streams and resets the stdin-once guard,
// returning a function that restores the previous streams (and resets the guard
// again). It exists for tests and for in-process harnesses that drive CLI
// commands with in-memory streams; a one-shot `ze` invocation uses the real
// os.Stdin/os.Stdout and never calls this.
func SwapStreams(in io.Reader, out io.Writer) (restore func()) {
	prevIn, prevOut := stdin, stdout
	stdin, stdout = in, out
	stdinClaimed.Store(false)
	return func() {
		stdin, stdout = prevIn, prevOut
		stdinClaimed.Store(false)
	}
}

// claimStdin marks stdin consumed, failing closed if it was already claimed.
func claimStdin() error {
	if !stdinClaimed.CompareAndSwap(false, true) {
		return ErrStdinClaimed
	}
	return nil
}

// ReadFile returns the bytes at path, reading (capped) stdin when path is "-".
// The stdin read claims the process's single stdin (ErrStdinClaimed on a second
// claim). A real path is read whole with os.ReadFile.
func ReadFile(path string) ([]byte, error) {
	if IsStdin(path) {
		if err := claimStdin(); err != nil {
			return nil, err
		}
		data, err := io.ReadAll(io.LimitReader(stdin, MaxStdinBytes+1))
		if err != nil {
			return nil, fmt.Errorf("cliio: read stdin: %w", err)
		}
		if len(data) > MaxStdinBytes {
			return nil, fmt.Errorf("cliio: stdin exceeds %d bytes", MaxStdinBytes)
		}
		return data, nil
	}
	return os.ReadFile(path) //nolint:gosec // path is a CLI arg resolved by cliio; "-" already handled
}

// OpenReader returns a reader over path, or stdin when path is "-". The caller
// must Close the result; closing a stdin reader is a no-op (os.Stdin stays
// open). The reader streams -- it is never fully buffered -- so multi-GB inputs
// (MRT) are safe. The stdin form claims the process's single stdin.
func OpenReader(path string) (io.ReadCloser, error) {
	if IsStdin(path) {
		if err := claimStdin(); err != nil {
			return nil, err
		}
		return io.NopCloser(stdin), nil
	}
	f, err := os.Open(path) //nolint:gosec // path is a CLI arg resolved by cliio; "-" already handled
	if err != nil {
		return nil, err
	}
	return f, nil
}

// nopWriteCloser adapts an io.Writer to io.WriteCloser with a no-op Close, so a
// "-" writer can be Closed like a file without closing os.Stdout.
type nopWriteCloser struct{ w io.Writer }

func (n nopWriteCloser) Write(p []byte) (int, error) { return n.w.Write(p) }
func (nopWriteCloser) Close() error                  { return nil }

// Create returns a writer over path, or stdout when path is "-". The caller must
// Close the result; closing a stdout writer is a no-op (os.Stdout stays open).
func Create(path string) (io.WriteCloser, error) {
	if IsStdin(path) {
		return nopWriteCloser{w: stdout}, nil
	}
	f, err := os.Create(path) //nolint:gosec // path is a CLI arg resolved by cliio; "-" already handled
	if err != nil {
		return nil, err
	}
	return f, nil
}

// WriteFile writes data to path, or to stdout when path is "-". perm is ignored
// for stdout (it is an already-open fd).
func WriteFile(path string, data []byte, perm fs.FileMode) error {
	if IsStdin(path) {
		if _, err := stdout.Write(data); err != nil {
			return fmt.Errorf("cliio: write stdout: %w", err)
		}
		return nil
	}
	return os.WriteFile(path, data, perm) //nolint:gosec // path is a CLI arg resolved by cliio; "-" already handled
}
