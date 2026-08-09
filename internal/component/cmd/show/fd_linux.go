// Design: docs/architecture/diagnostics/procfs-diagnostics.md -- FD inspection from /proc/self/fd (lsof replacement)
// Related: sockets_linux.go -- existing /proc reading pattern
//
//go:build linux

package show

import (
	"os"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/procfs"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:system-file-descriptors", Handler: handleShowSystemFD},
	)
}

func handleShowSystemFD(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	const detailMode = "detail"
	mode := "summary"
	for _, a := range args {
		if a == detailMode {
			mode = detailMode
		}
	}

	fds, err := readProcSelfFD()
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
	}

	counts := map[string]int{}
	for _, fd := range fds {
		cat := categorizeFDTarget(fd.target)
		counts[cat]++
	}

	softLimit, hardLimit := readFDLimits()

	result := map[string]any{
		"total":      len(fds),
		"by-type":    counts,
		"soft-limit": softLimit,
		"hard-limit": hardLimit,
	}

	if mode == detailMode {
		details := make([]map[string]any, 0, len(fds))
		for _, fd := range fds {
			details = append(details, map[string]any{
				"fd":     fd.num,
				"target": fd.target,
				"type":   categorizeFDTarget(fd.target),
			})
		}
		result["fds"] = details
	}

	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(result)}, nil
}

type fdEntry struct {
	num    int
	target string
}

func readProcSelfFD() ([]fdEntry, error) {
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return nil, err
	}

	var fds []fdEntry
	for _, e := range entries {
		num, parseErr := strconv.Atoi(e.Name())
		if parseErr != nil {
			continue
		}
		target, readErr := os.Readlink("/proc/self/fd/" + e.Name())
		if readErr != nil {
			target = "(unknown)"
		}
		fds = append(fds, fdEntry{num: num, target: target})
	}
	return fds, nil
}

func categorizeFDTarget(target string) string {
	switch {
	case strings.HasPrefix(target, "socket:"):
		return "socket"
	case strings.HasPrefix(target, "pipe:"):
		return "pipe"
	case strings.HasPrefix(target, "anon_inode:"):
		return "anon_inode"
	case strings.HasPrefix(target, "/dev/"):
		return "device"
	case target == "(unknown)":
		return "unknown"
	default:
		return "file"
	}
}

func readFDLimits() (soft, hard int) {
	lines, err := procfs.ReadFileLines("/proc/self/limits")
	if err != nil {
		return 0, 0
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, "Max open files") {
			continue
		}
		fields := strings.Fields(line)
		// "Max open files  1024  1048576  files"
		if len(fields) >= 6 {
			soft, _ = strconv.Atoi(fields[3])
			hard, _ = strconv.Atoi(fields[4])
		}
		break
	}
	return soft, hard
}
