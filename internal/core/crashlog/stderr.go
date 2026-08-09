// Design: docs/architecture/diagnostics/crash-capture.md -- stderr redirect and syslog forwarding

package crashlog

import (
	"bufio"
	"log/syslog"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
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

	scanner := bufio.NewScanner(pr)
	scanner.Buffer(make([]byte, 0, 256*1024), 256*1024)
	var panicBuf []byte
	inPanic := false

	for scanner.Scan() {
		line := scanner.Text()

		if origStderr != nil {
			writeMsg(origStderr, line+"\n")
		}

		if syslogW != nil {
			if err := syslogW.Warning(line); err != nil {
				syslogW = nil
			}
		}

		if !inPanic && panicPattern.MatchString(line) {
			inPanic = true
			ring := slogutil.GlobalLogRing()
			entries := ring.Snapshot(64, "", "")
			panicBuf = appendCrashMetadata(panicBuf)
			panicBuf = appendRingHeader(panicBuf, entries)
			panicBuf = append(panicBuf, "\n=== Panic ===\n"...)
		}

		if inPanic {
			panicBuf = append(panicBuf, line...)
			panicBuf = append(panicBuf, '\n')
		}
	}

	if inPanic && crashDirPath != "" {
		writeCrashFile(crashDirPath, crashKeep, string(panicBuf))
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
