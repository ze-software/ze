// Design: plan/spec-diag-crash-capture.md -- crash capture for panic diagnostics
//
// Package crashlog captures stderr output (including Go panic traces) and
// forwards it to syslog and a crash file on disk. Init() must be the first
// call in main(), before any other initialization.

package crashlog

import (
	"io"
	"log/syslog"
	"os"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/internal/core/version"
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

		if crashDir != "" {
			writeMsg(origStderr, "crash dir: "+crashDir+"\n")
		}

		if err := redirectStderr(syslogAddr, crashDir); err != nil {
			writeMsg(origStderr, "warning: crash capture: "+err.Error()+"\n")
		}
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

func buildCrashReport(panicValue any, stack []byte) string {
	ring := slogutil.GlobalLogRing()
	entries := ring.Snapshot(64, "", "")

	uptime := time.Since(startTime).Truncate(time.Second)

	var b []byte
	b = append(b, "=== Ze Crash Report ===\n"...)
	b = append(b, "Time: "...)
	b = append(b, time.Now().UTC().Format(time.RFC3339)...)
	b = append(b, "\nVersion: "...)
	b = append(b, version.Release()...)
	b = append(b, "\nBuild: "...)
	b = append(b, version.BuildDate()...)
	b = append(b, "\nUptime: "...)
	b = append(b, uptime.String()...)
	b = append(b, "\n\n=== Panic ===\n"...)
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
