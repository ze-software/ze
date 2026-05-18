// Design: plan/spec-diag-core.md -- process memory map from /proc/self/status (pmap replacement)
// Related: system.go -- existing Go runtime memory stats
//
//go:build linux

package show

import (
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/procfs"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:system-memory-map", Handler: handleShowSystemMemoryMap},
	)
}

func handleShowSystemMemoryMap(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	status, err := parseProcSelfStatus()
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Data: err.Error()}, nil //nolint:nilerr // operational error in Response
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: status}, nil
}

func parseProcSelfStatus() (map[string]any, error) {
	lines, err := procfs.ReadFileLines("/proc/self/status")
	if err != nil {
		return nil, err
	}

	result := map[string]any{}
	for _, line := range lines {
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		value = strings.TrimSpace(value)
		switch key {
		case "VmRSS":
			result["vm-rss-kb"] = parseKBValue(value)
		case "VmSize":
			result["vm-size-kb"] = parseKBValue(value)
		case "VmSwap":
			result["vm-swap-kb"] = parseKBValue(value)
		case "VmPeak":
			result["vm-peak-kb"] = parseKBValue(value)
		case "VmData":
			result["vm-data-kb"] = parseKBValue(value)
		case "VmStk":
			result["vm-stack-kb"] = parseKBValue(value)
		case "Threads":
			n, _ := strconv.Atoi(value)
			result["threads"] = n
		}
	}
	return result, nil
}

func parseKBValue(s string) int {
	s = strings.TrimSuffix(s, " kB")
	s = strings.TrimSpace(s)
	n, _ := strconv.Atoi(s)
	return n
}
