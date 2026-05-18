// Design: plan/spec-diag-crash-capture.md -- stderr redirect and syslog forwarding

package crashlog

import (
	"bufio"
	"log/syslog"
	"os"
	"regexp"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

var panicPattern = regexp.MustCompile(`^goroutine \d+ \[running\]:`)

func redirectStderr(syslogAddress, crashDirPath string) error {
	pr, pw, err := os.Pipe()
	if err != nil {
		return err
	}

	if err := dupStderr(int(pw.Fd())); err != nil {
		pr.Close() //nolint:errcheck // cleanup on dup2 failure
		pw.Close() //nolint:errcheck // cleanup on dup2 failure
		return err
	}

	os.Stderr = pw

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

func stderrReader(pr *os.File, syslogW *syslog.Writer, crashDirPath string) {
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
			panicBuf = appendRingHeader(panicBuf, entries)
			panicBuf = append(panicBuf, "=== Panic ===\n"...)
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
