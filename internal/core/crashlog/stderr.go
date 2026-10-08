// Design: docs/architecture/diagnostics/crash-capture.md -- stderr redirect and syslog forwarding

package crashlog

import (
	"bytes"
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

// relayStderrBuffer is how much of one line the relay holds for syslog and for
// panic detection. A longer line is handed on in fragments of this size.
const relayStderrBuffer = 256 * 1024

// relayStderr copies r to out as each read returns, collects a panic trace once
// it sees the start of one, and sends each line to syslog. It returns the
// trace, whether a panic was seen, and the read error, which is nil at EOF.
//
// out gets the bytes as they arrive, a partial line included. The relay once
// forwarded whole lines only, so a prompt written with no line end ("username: "
// from `ze init`) stayed in the relay while the process waited for the answer
// to it, and the operator saw nothing.
//
// Syslog and panic detection keep the line framing: one syslog message per
// line, and a line is complete at its newline, at the end of input, or when it
// fills relayStderrBuffer, after which the rest of it is relayed as fragments.
// A syslog message is a record, and a prompt split from its answer would be two.
// Only the start of a line can open a panic trace, so a fragment is never
// matched against the pattern. The relay once stopped at an overlong line, as
// bufio.Scanner does: every later line was lost, and nothing read the pipe
// again, so the process blocked at its next write of stderr once the pipe
// filled.
//
// A crash file that ends because the read failed holds a TRUNCATED trace, and a
// reader takes the last frame in it for the last frame there was, so the
// truncation is written into the trace itself.
func relayStderr(r io.Reader, out io.Writer, syslogW *syslog.Writer) ([]byte, bool, error) {
	lines := lineRelay{syslogW: syslogW, lineStart: true}
	chunk := make([]byte, relayReadSize)
	line := make([]byte, 0, relayStderrBuffer)

	for {
		n, readErr := r.Read(chunk)
		data := chunk[:n]
		if out != nil && n > 0 {
			out.Write(data) //nolint:errcheck // the real stderr has nobody to report to
		}
		for len(data) > 0 {
			room := relayStderrBuffer - len(line)
			end := bytes.IndexByte(data, '\n')
			if end >= 0 && end <= room {
				line = append(line, data[:end]...)
				lines.take(line, true)
				line = line[:0]
				data = data[end+1:]
				continue
			}
			keep := min(len(data), room)
			line = append(line, data[:keep]...)
			data = data[keep:]
			if len(line) == relayStderrBuffer {
				lines.take(line, false)
				line = line[:0]
			}
		}
		if readErr == nil {
			continue
		}

		// The input ended, so a partial line is as complete as it will get.
		if len(line) > 0 {
			lines.take(line, true)
		}
		if errors.Is(readErr, io.EOF) {
			return lines.panicBuf, lines.inPanic, nil
		}
		if lines.inPanic {
			// The trace stops here because the read stopped, not because the
			// panic finished printing. Say so inside the trace: the crash file
			// is the only thing its reader will have.
			var tb textbuf.Buffer
			lines.panicBuf = append(lines.panicBuf, tb.Str("\n=== TRUNCATED: stderr relay stopped: ").Err(readErr).Str(" ===\n").String()...)
		}
		return lines.panicBuf, lines.inPanic, readErr
	}
}

// lineRelay is the line-framed half of the relay: syslog and panic collection.
// Not safe for concurrent use; relayStderr owns it.
type lineRelay struct {
	syslogW   *syslog.Writer
	panicBuf  []byte
	inPanic   bool
	lineStart bool // the next fragment starts a line
}

// take hands on one line, or one fragment of an overlong line when complete is
// false. A "\r" before the newline is dropped, as bufio.Reader.ReadLine did.
func (l *lineRelay) take(fragment []byte, complete bool) {
	if complete {
		fragment = bytes.TrimSuffix(fragment, []byte{'\r'})
	}

	if l.syslogW != nil {
		if err := l.syslogW.Warning(string(fragment)); err != nil {
			l.syslogW = nil
		}
	}

	if l.lineStart && !l.inPanic && panicPattern.Match(fragment) {
		l.inPanic = true
		ring := slogutil.GlobalLogRing()
		entries := ring.Recent(64)
		l.panicBuf = appendCrashMetadata(l.panicBuf)
		l.panicBuf = appendRingHeader(l.panicBuf, entries)
		l.panicBuf = append(l.panicBuf, "\n=== Panic ===\n"...)
	}

	if l.inPanic {
		l.panicBuf = append(l.panicBuf, fragment...)
		if complete {
			l.panicBuf = append(l.panicBuf, '\n')
		}
	}
	l.lineStart = complete
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
