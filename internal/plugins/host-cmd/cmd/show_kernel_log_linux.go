// Design: plan/learned/727-diag-core.md — kernel log reader from /dev/kmsg (dmesg replacement)

//go:build linux

package cmd

import (
	"errors"
	"io"
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
	count := defaultKernelLogCount
	maxLevel := 7

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

func readKmsg(count, maxLevel int) ([]map[string]any, error) {
	f, err := os.OpenFile("/dev/kmsg", os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // diagnostic read-only fd, close error not actionable

	var entries []map[string]any
	buf := make([]byte, 8192)
	for {
		n, readErr := f.Read(buf)
		if n > 0 {
			entry := parseKmsgLine(string(buf[:n]))
			if entry != nil {
				level, _ := entry["level-num"].(int)
				if level <= maxLevel {
					entries = append(entries, entry)
				}
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) || errors.Is(readErr, os.ErrClosed) {
				break
			}
			if isEAGAIN(readErr) {
				break
			}
			break
		}
	}

	if len(entries) > count {
		entries = entries[len(entries)-count:]
	}
	return entries, nil
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
