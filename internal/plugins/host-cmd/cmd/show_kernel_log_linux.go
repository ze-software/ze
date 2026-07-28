// Design: plan/learned/727-diag-core.md — kernel log reader from /dev/kmsg (dmesg replacement)

//go:build linux

package cmd

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

const (
	defaultKernelLogCount = 50
	maxKernelLogCount     = 10000
	// kmsgRecordMax bounds one /dev/kmsg record. Each read returns exactly one
	// record, and a buffer shorter than the record makes the kernel return
	// EINVAL, so this must stay at or above the kernel's own record ceiling
	// (CONSOLE_EXT_LOG_MAX, 8192).
	kmsgRecordMax = 8192
)

var kmsgLevelNames = [8]string{
	"emerg", "alert", "crit", "err", "warning", "notice", "info", "debug",
}

func RegisterShowKernelLog() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:system-kernel-log", Handler: handleShowSystemKernelLog},
	)
}

func handleShowSystemKernelLog(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	count, maxLevel := parseKernelLogArgs(args)

	entries, err := readKmsg(count, maxLevel)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"entries": entries,
			"count":   len(entries),
		},
	}, nil
}

// parseKernelLogArgs reads the `count N` / `level L` pair out of the dispatched
// argument list, falling back to the defaults for anything absent or outside
// range. Split out of the handler so the range checks are reachable from a test
// on every host: the handler itself cannot run without /dev/kmsg.
func parseKernelLogArgs(args []string) (count, maxLevel int) {
	count = defaultKernelLogCount
	maxLevel = len(kmsgLevelNames) - 1

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "count":
			if i+1 < len(args) {
				i++
				n, err := strconv.Atoi(args[i])
				if err == nil && n >= 1 && n <= maxKernelLogCount {
					count = n
				}
			}
		case "level":
			if i+1 < len(args) {
				i++
				maxLevel = parseLevelArg(args[i])
			}
		}
	}
	return count, maxLevel
}

func parseLevelArg(s string) int {
	for i, name := range kmsgLevelNames {
		if s == name {
			return i
		}
	}
	n, err := strconv.Atoi(s)
	if err == nil && n >= 0 && n <= 7 {
		return n
	}
	return 7
}

// readKmsg drains the kernel ring buffer and returns the newest count entries
// at or below maxLevel.
//
// It reads the descriptor with RAW syscalls, deliberately not os.OpenFile plus
// (*os.File).Read. On Linux os.OpenFile registers every descriptor it hands back
// with the runtime netpoller (os/file_unix.go newFile: `pollable := kind ==
// kindOpenFile || ...`; the "not pollable" carve-out below that line covers only
// the BSDs), and for a pollable descriptor EAGAIN is not returned to the caller
// -- the runtime parks the goroutine until the fd is readable again. /dev/kmsg
// becomes readable again only when the kernel logs a NEW message, so the EAGAIN
// exit below was unreachable: once the ring buffer was drained this function
// blocked, the ze-show:system-kernel-log RPC never returned, and `show system
// kernel-log` hung the daemon until its caller timed out.
//
// That went unseen because the open itself fails EPERM on any host with
// kernel.dmesg_restrict=1 and no CAP_SYSLOG, which is every unprivileged host:
// the handler returned a clean StatusError long before it could hang, so the
// hang only appears once the process HAS the capability. Reading the descriptor
// directly keeps EAGAIN observable, which is the entire premise of opening with
// O_NONBLOCK.
func readKmsg(count, maxLevel int) ([]map[string]any, error) {
	fd, err := syscall.Open("/dev/kmsg", syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: "/dev/kmsg", Err: err}
	}
	defer syscall.Close(fd) //nolint:errcheck // diagnostic read-only fd, close error not actionable

	return drainKmsg(fd, count, maxLevel), nil
}

// drainKmsg reads every currently queued record from fd and returns the newest
// count of them at or below maxLevel. Separate from readKmsg so a test can drive
// both loop exits (drained-then-EAGAIN, and end-of-file) over a pipe on a host
// where /dev/kmsg cannot be opened.
func drainKmsg(fd, count, maxLevel int) []map[string]any {
	var entries []map[string]any
	buf := make([]byte, kmsgRecordMax)
	for {
		n, readErr := syscall.Read(fd, buf)
		if readErr != nil {
			// EINTR is not a failure: a raw syscall sees the runtime's own
			// preemption signals, which (*os.File).Read used to absorb.
			if errors.Is(readErr, syscall.EINTR) {
				continue
			}
			// EAGAIN is the documented end of the ring buffer for a
			// non-blocking reader. Any other error ends the scan with what was
			// already collected rather than failing a diagnostic command.
			break
		}
		if n == 0 {
			// A raw read reports end-of-file as (0, nil), where
			// (*os.File).Read reported io.EOF. Without this the loop spins.
			break
		}
		entry := parseKmsgLine(string(buf[:n]))
		if entry != nil {
			level, _ := entry["level-num"].(int)
			if level <= maxLevel {
				entries = append(entries, entry)
			}
		}
	}

	if len(entries) > count {
		entries = entries[len(entries)-count:]
	}
	return entries
}

func isEAGAIN(err error) bool {
	return errors.Is(err, syscall.EAGAIN)
}

func parseKmsgLine(line string) map[string]any {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	before, after, ok := strings.Cut(line, ";")
	if !ok {
		return nil
	}
	prefix := before
	message := after

	parts := strings.SplitN(prefix, ",", 4)
	if len(parts) < 3 {
		return nil
	}

	level, _ := strconv.Atoi(parts[0])
	seq, _ := strconv.ParseUint(parts[1], 10, 64)
	timestampUS, _ := strconv.ParseUint(parts[2], 10, 64)

	levelName := "unknown"
	if level >= 0 && level < len(kmsgLevelNames) {
		levelName = kmsgLevelNames[level]
	}

	return map[string]any{
		"level":        levelName,
		"level-num":    level,
		"sequence":     seq,
		"timestamp-us": timestampUS,
		"message":      strings.TrimSpace(message),
	}
}
