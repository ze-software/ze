// Design: plan/learned/726-diag-crash-capture.md -- crash capture for panic diagnostics
//
// Package crashlog captures stderr output (including Go panic traces) and
// forwards it to syslog and a crash file on disk. Init() must be the first
// call in main(), before any other initialization.

package crashlog

import (
	"io"
	"log/syslog"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/version"
)

var (
	_ = env.MustRegister(env.EnvEntry{Key: "ze.crash.dir", Type: "string", Description: "Override crash report directory (autodetected if not set)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.crash.keep", Type: "int", Default: "5", Description: "Number of crash files to retain (1-100)"})
)

var (
	startTime  time.Time
	crashDir   string
	crashKeep  int
	syslogAddr string
	origStderr *os.File
	initOnce   sync.Once
)

func Init() {
	initOnce.Do(func() {
		startTime = time.Now()
		origStderr = os.Stderr
		syslogAddr = env.Get("ze.log.destination")
		crashDir = resolveCrashDir()
		crashKeep = parseCrashKeep()

		if err := redirectStderr(syslogAddr, crashDir); err != nil {
			writeMsg(origStderr, "warning: crash capture: "+err.Error()+"\n")
		}

		// redirectStderr replaced os.Stderr with the pipe. A caller that writes
		// a fatal diagnostic and exits gets no reader, so point env at the
		// descriptor saved above, which is still the real stderr.
		env.SetFatalOutput(origStderr)
	})
}

// writeMsg writes a message to w, ignoring errors (crash-path output).
//
//nolint:errcheck // crash-path and pre-logger startup output
func writeMsg(w io.Writer, msg string) {
	io.WriteString(w, msg)
}

// HandlePanic captures a panic value, writes a crash report to disk and
// syslog, and prints it to the original stderr. The caller (main.go) is
// responsible for calling os.Exit(2) after this returns.
func HandlePanic(r any) {
	stack := debug.Stack()
	report := buildCrashReport(r, stack)

	if crashDir != "" {
		writeCrashFile(crashDir, crashKeep, report)
	}

	if syslogAddr != "" {
		var msg []byte
		msg = append(msg, "PANIC: "...)
		msg = appendValue(msg, r)
		writeSyslogCrash(syslogAddr, string(msg))
	}

	writeMsg(origStderr, report)
}

// HandleCaughtPanic writes a crash report when a panic was caught by a
// framework (e.g. bubbletea) before Ze's own recover could fire. When
// stderr is redirected to the pipe reader, the reader produces a more
// complete crash file (with the stack trace), so this function only
// writes to syslog. When stderr is not redirected, this function
// writes the crash file directly.
func HandleCaughtPanic(err error) {
	if crashDir != "" && pipeW == nil {
		writeCrashFile(crashDir, crashKeep, buildCrashReport(err, nil))
	}

	if syslogAddr != "" {
		errStr := err.Error()
		msg := make([]byte, 0, len("PANIC (caught): ")+len(errStr))
		msg = append(msg, "PANIC (caught): "...)
		msg = append(msg, errStr...)
		writeSyslogCrash(syslogAddr, string(msg))
	}
}

func appendValue(b []byte, v any) []byte {
	switch val := v.(type) {
	case string:
		return append(b, val...)
	case error:
		return append(b, val.Error()...)
	default:
		return append(b, "<unknown panic value>"...)
	}
}

func appendCrashMetadata(b []byte) []byte {
	b = append(b, "=== Ze Crash Report ===\n"...)
	b = append(b, "Time: "...)
	b = append(b, time.Now().UTC().Format(time.RFC3339)...)
	b = append(b, "\nVersion: "...)
	b = append(b, version.Release()...)
	b = append(b, "\nBuild: "...)
	b = append(b, version.BuildDate()...)

	if bi, ok := debug.ReadBuildInfo(); ok {
		var commit string
		var modified bool
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				commit = s.Value
				if len(commit) > 12 {
					commit = commit[:12]
				}
			case "vcs.modified":
				modified = s.Value == "true"
			}
		}
		if commit != "" {
			b = append(b, "\nCommit: "...)
			b = append(b, commit...)
			if modified {
				b = append(b, " (modified)"...)
			}
		}
	}

	b = append(b, "\nGo: "...)
	b = append(b, runtime.Version()...)
	b = append(b, "\nOS/Arch: "...)
	b = append(b, runtime.GOOS...)
	b = append(b, '/')
	b = append(b, runtime.GOARCH...)
	b = append(b, "\nPID: "...)
	b = strconv.AppendInt(b, int64(os.Getpid()), 10)
	b = append(b, "\nGoroutines: "...)
	b = strconv.AppendInt(b, int64(runtime.NumGoroutine()), 10)
	b = append(b, "\nUptime: "...)
	b = append(b, time.Since(startTime).Truncate(time.Second).String()...)

	if len(os.Args) > 0 {
		b = append(b, "\nCommand: "...)
		for i, arg := range os.Args {
			if i > 0 {
				b = append(b, ' ')
			}
			if needsQuoting(arg) {
				b = append(b, '"')
				b = append(b, arg...)
				b = append(b, '"')
			} else {
				b = append(b, arg...)
			}
		}
	}

	b = append(b, '\n')
	return b
}

func needsQuoting(s string) bool {
	for i := range len(s) {
		if s[i] == ' ' || s[i] == '\t' {
			return true
		}
	}
	return false
}

func buildCrashReport(panicValue any, stack []byte) string {
	ring := slogutil.GlobalLogRing()
	entries := ring.Snapshot(64, "", "")

	var b []byte
	b = appendCrashMetadata(b)

	b = append(b, "\n=== Panic ===\n"...)
	b = appendValue(b, panicValue)
	b = append(b, '\n')

	if len(stack) > 0 {
		b = append(b, "\n=== Stack Trace ===\n"...)
		b = append(b, stack...)
	}

	if len(entries) > 0 {
		b = append(b, "\n=== Recent Log (last "...)
		b = strconv.AppendInt(b, int64(len(entries)), 10)
		b = append(b, " entries) ===\n"...)
		for i := range entries {
			b = append(b, entries[i].Timestamp.UTC().Format(time.RFC3339)...)
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
	}

	return string(b)
}

func writeSyslogCrash(addr, msg string) {
	network, raddr := parseSyslogAddr(addr)
	w, dialErr := syslog.Dial(network, raddr, syslog.LOG_CRIT|syslog.LOG_DAEMON, "ze")
	if dialErr != nil {
		return
	}
	if err := w.Crit(msg); err != nil {
		writeMsg(origStderr, "crash syslog write failed: "+err.Error()+"\n")
	}
	if err := w.Close(); err != nil {
		writeMsg(origStderr, "crash syslog close failed: "+err.Error()+"\n")
	}
}

func parseSyslogAddr(addr string) (string, string) {
	if addr == "" {
		return "", ""
	}
	if addr[0] == '/' {
		return "unix", addr
	}
	return "udp", addr
}
