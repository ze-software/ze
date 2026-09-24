// Design: docs/architecture/diagnostics/crash-capture.md -- stderr redirect and syslog forwarding

package crashlog

import (
	"bufio"
	"errors"
	"io"
	"log/syslog"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var panicPattern = regexp.MustCompile(`^goroutine \d+ \[running\]:`)

var (
	pipeW      *os.File
	readerDone chan struct{}
	flushOnce  sync.Once
)

func redirectStderr(syslogAddress, crashDirPath string) error {
	pr, pw, err := os.Pipe()
	if err != nil {
		return err
	}

	// Save fd 2 to a new fd before dup2 overwrites it with the pipe.
	// Without this, origStderr wraps fd 2 which becomes the pipe,
	// and the reader goroutine would write back into its own pipe.
	if saved := saveStderr(); saved != nil {
		origStderr = saved
	}

	if err := dupStderr(int(pw.Fd())); err != nil {
		pr.Close() //nolint:errcheck // cleanup on dup2 failure
		pw.Close() //nolint:errcheck // cleanup on dup2 failure
		return err
	}

	os.Stderr = pw
	pipeW = pw
	readerDone = make(chan struct{})

	var syslogW *syslog.Writer
	if syslogAddress != "" {
		network, raddr := parseSyslogAddr(syslogAddress)
		w, dialErr := syslog.Dial(network, raddr, syslog.LOG_WARNING|syslog.LOG_DAEMON, "ze")
		if dialErr == nil {
			syslogW = w
		}
	}

	go stderrReader(pr, syslogW, crashDirPath)

	return nil
}

// Flush drains the stderr pipe so buffered output reaches the terminal.
// Safe to call multiple times; only the first call acts.
func Flush() {
	flushOnce.Do(func() {
		if pipeW == nil {
			return
		}
		// Restore fd 2 to the original stderr. This closes the dup2'd
		// reference to the pipe write end on fd 2, so that closing pipeW
		// below fully closes the pipe and the reader goroutine sees EOF.
		_ = dupStderr(int(origStderr.Fd()))
		pipeW.Close() //nolint:errcheck // triggers EOF on the reader goroutine
		select {
		case <-readerDone:
		case <-time.After(500 * time.Millisecond):
		}
		os.Stderr = origStderr
	})
}

func stderrReader(pr *os.File, syslogW *syslog.Writer, crashDirPath string) {
	defer close(readerDone)

	panicBuf, inPanic, err := relayStderr(pr, syslogW)
	if err != nil && origStderr != nil {
		var tb textbuf.Buffer
		writeMsg(origStderr, tb.Str("crashlog: stderr relay stopped: ").Err(err).Byte('\n').String())
	}
	if inPanic && crashDirPath != "" {
		writeCrashFile(crashDirPath, crashKeep, string(panicBuf))
	}
}

// relayStderrBuffer is how much of one line the relay holds at a time. A longer
// line is relayed in fragments of this size.
const relayStderrBuffer = 256 * 1024

// relayStderr copies every line of r to the original stderr and to syslog, and
// collects a panic trace once it sees the start of one. It returns the trace,
// whether a panic was seen, and the read error, which is nil at EOF.
//
// A line longer than the buffer is relayed in fragments, and the relay goes
// on. It stopped at such a line once, as bufio.Scanner does: every later line
// was lost, and nothing read the pipe again, so the process blocked at its next
// write of stderr once the pipe filled. Only the start of a line can open a
// panic trace, so a fragment is never matched against the pattern.
//
// A crash file that ends because the read failed holds a TRUNCATED trace, and a
// reader takes the last frame in it for the last frame there was, so the
// truncation is written into the trace itself.
func relayStderr(r io.Reader, syslogW *syslog.Writer) ([]byte, bool, error) {
	reader := bufio.NewReaderSize(r, relayStderrBuffer)
	var panicBuf []byte
	inPanic := false
	lineStart := true

	for {
		fragment, more, readErr := reader.ReadLine()
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return panicBuf, inPanic, nil
			}
			if inPanic {
				// The trace stops here because the read stopped, not because the
				// panic finished printing. Say so inside the trace: the crash file
				// is the only thing its reader will have.
				var tb textbuf.Buffer
				panicBuf = append(panicBuf, tb.Str("\n=== TRUNCATED: stderr relay stopped: ").Err(readErr).Str(" ===\n").String()...)
			}
			return panicBuf, inPanic, readErr
		}
		text := string(fragment)

		if origStderr != nil {
			if more {
				writeMsg(origStderr, text)
			} else {
				writeMsg(origStderr, text+"\n")
			}
		}

		if syslogW != nil {
			if err := syslogW.Warning(text); err != nil {
				syslogW = nil
			}
		}

		if lineStart && !inPanic && panicPattern.MatchString(text) {
			inPanic = true
			ring := slogutil.GlobalLogRing()
			entries := ring.Recent(64)
			panicBuf = appendCrashMetadata(panicBuf)
			panicBuf = appendRingHeader(panicBuf, entries)
			panicBuf = append(panicBuf, "\n=== Panic ===\n"...)
		}

		if inPanic {
			panicBuf = append(panicBuf, fragment...)
			if !more {
				panicBuf = append(panicBuf, '\n')
			}
		}
		lineStart = !more
	}
}

func appendRingHeader(b []byte, entries []slogutil.LogEntry) []byte {
	if len(entries) == 0 {
		return b
	}
	b = append(b, "=== Recent Log (pre-crash) ===\n"...)
	for i := range entries {
		b = append(b, entries[i].Timestamp.UTC().Format("2006-01-02T15:04:05Z")...)
		b = append(b, " ["...)
		b = append(b, entries[i].Level...)
		b = append(b, "] "...)
		if entries[i].Component != "" {
			b = append(b, entries[i].Component...)
			b = append(b, ": "...)
		}
		b = append(b, entries[i].Message...)
		b = append(b, '\n')
	}
	b = append(b, '\n')
	return b
}
