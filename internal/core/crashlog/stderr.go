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

// relayQueueLimit is how many bytes of stderr the relay holds while its
// downstream (the real stderr, syslog) is slower than the writers. Past it the
// relay drops bytes, counts them, and writes the count into the stream where
// they went missing. It never makes a writer wait on the downstream.
const relayQueueLimit = 4 * 1024 * 1024

// relayReadSize is how much the pump takes out of the pipe in one read.
const relayReadSize = 64 * 1024

// redirectStderr points os.Stderr at a pipe and starts the relay that copies it
// to the original stderr and to syslog.
//
// Descriptor 2 is NOT redirected. The Go runtime prints a fatal error, an
// unrecovered panic and a SIGQUIT dump only after it has frozen every goroutine,
// with a raw write to descriptor 2. A pipe on descriptor 2 whose reader is a
// goroutine is a pipe nobody reads at that moment: it filled at 64 KiB and the
// dump blocked for ever, so `kill -QUIT` hung the daemon, and a panic trace
// never reached anyone. The runtime therefore keeps the real descriptor 2, and
// armCrashOutput gives it a crash file of its own.
func redirectStderr(syslogAddress, crashDirPath string) error {
	pr, pw, err := os.Pipe()
	if err != nil {
		return err
	}

	var syslogW *syslog.Writer
	if syslogAddress != "" {
		network, raddr := parseSyslogAddr(syslogAddress)
		w, dialErr := syslog.Dial(network, raddr, syslog.LOG_WARNING|syslog.LOG_DAEMON, "ze")
		if dialErr == nil {
			syslogW = w
		}
	}

	os.Stderr = pw
	pipeW = pw
	readerDone = startRelay(pr, origStderr, syslogW, crashDirPath)
	return nil
}

// startRelay starts the two goroutines that live as long as the process: the
// pump, which moves the pipe into a stderrQueue as fast as the writers fill it,
// and the relay, which copies the queue to out and to syslog at whatever pace
// they take. The returned channel closes when the relay has seen the end of r.
func startRelay(r io.Reader, out io.Writer, syslogW *syslog.Writer, crashDirPath string) chan struct{} {
	queue := newStderrQueue(relayQueueLimit)
	done := make(chan struct{})
	go pumpStderr(r, queue)
	go stderrReader(queue, out, syslogW, crashDirPath, done)
	return done
}

// pumpStderr reads r until it ends and hands every byte to the queue, which
// never blocks it. The reader is therefore always draining the pipe, whatever
// the downstream does.
func pumpStderr(r io.Reader, queue *stderrQueue) {
	buf := make([]byte, relayReadSize)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			queue.Write(buf[:n]) //nolint:errcheck // stderrQueue.Write cannot fail
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = nil
			}
			queue.close(err)
			return
		}
	}
}

// Flush drains the stderr relay so queued output reaches the terminal.
// Safe to call multiple times; only the first call acts.
func Flush() {
	flushOnce.Do(func() {
		if pipeW == nil {
			return
		}
		os.Stderr = origStderr
		pipeW.Close() //nolint:errcheck // triggers EOF on the pump
		select {
		case <-readerDone:
		case <-time.After(500 * time.Millisecond):
		}
	})
}

func stderrReader(r io.Reader, out io.Writer, syslogW *syslog.Writer, crashDirPath string, done chan struct{}) {
	defer close(done)

	panicBuf, inPanic, err := relayStderr(r, out, syslogW)
	if err != nil && out != nil {
		var tb textbuf.Buffer
		writeMsg(out, tb.Str("crashlog: stderr relay stopped: ").Err(err).Byte('\n').String())
	}
	if inPanic && crashDirPath != "" {
		writeCrashFile(crashDirPath, crashKeep, string(panicBuf))
	}
}

// relayStderrBuffer is how much of one line the relay holds at a time. A longer
// line is relayed in fragments of this size.
const relayStderrBuffer = 256 * 1024

// relayStderr copies every line of r to out and to syslog, and
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
func relayStderr(r io.Reader, out io.Writer, syslogW *syslog.Writer) ([]byte, bool, error) {
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

		if out != nil {
			if more {
				writeMsg(out, text)
			} else {
				writeMsg(out, text+"\n")
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
