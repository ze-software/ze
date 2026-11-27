// Design: docs/architecture/diagnostics/procfs-diagnostics.md -- show system memory (OS view) from /proc/self/status
// Related: system.go -- existing Go runtime memory stats
//
//go:build linux

package show

import (
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/procfs"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-cmd:show-system-memory-map", Handler: handleShowSystemMemoryMap},
	)
}

func handleShowSystemMemoryMap(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	pluginName, hasPlugin, refusal := extractPluginPipeTarget(validated, "ze show system memory | plugin <name>")
	if refusal != nil {
		return refusal, nil
	}
	if hasPlugin {
		return inspectPluginSystemMemoryMap(ctx, pluginName)
	}

	status, err := parseProcSelfStatus()
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(status)}, nil
}

func inspectPluginSystemMemoryMap(ctx *pluginserver.CommandContext, pluginName string) (*plugin.Response, error) {
	srv, refusal := pluginInspectServer(ctx)
	if refusal != nil {
		return refusal, nil
	}
	res, opErr := inspectPluginHostFunc(srv, pluginName)
	if opErr != nil {
		return plugin.OperatorErrorResponse(opErr), nil
	}

	if res.ProcessMode == "internal" {
		pid := os.Getpid()
		info, err := procfs.ReadProcessInspect(pid)
		if err != nil {
			return plugin.OperatorErrorResponse(opererr.Wrap(
				err,
				"cannot read /proc status for daemon process",
				"reading /proc/self/status failed",
				"ze show system memory",
			)), nil //nolint:nilerr // operator error in Response
		}
		data := processInspectMemoryMap(info)
		data[keyPlugin] = res.Plugin
		data[keyProcessMode] = res.ProcessMode
		data[keyPID] = pid
		data[keySharedDaemonProcess] = true
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(data)}, nil
	}

	if res.Host.PID <= 0 {
		return plugin.OperatorErrorResponse(opererr.New(
			"plugin "+strconv.Quote(pluginName)+" has no running OS process",
			"the external plugin process is not currently running",
			"ze show system subsystem list",
		)), nil
	}
	if res.Host.Proc == nil {
		if res.Host.ProcError != "" {
			return plugin.OperatorErrorResponse(opererr.Wrap(
				errors.New(res.Host.ProcError),
				"cannot read /proc status for plugin "+strconv.Quote(pluginName),
				"reading /proc status for the external plugin process failed",
				"ze show system subsystem list",
			)), nil
		}
		return plugin.OperatorErrorResponse(opererr.New(
			"cannot read /proc status for plugin "+strconv.Quote(pluginName),
			"could not read /proc status for the external plugin process",
			"ze show system subsystem list",
		)), nil
	}

	data := processInspectMemoryMap(*res.Host.Proc)
	data[keyPlugin] = res.Plugin
	data[keyProcessMode] = res.ProcessMode
	data[keyPID] = res.Host.PID
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(data)}, nil
}

func processInspectMemoryMap(info procfs.ProcessInspect) map[string]any {
	data := map[string]any{
		"vm-rss-kb":   info.VmRSSKB,
		"vm-size-kb":  info.VmSizeKB,
		"vm-swap-kb":  info.VmSwapKB,
		"vm-peak-kb":  info.VmPeakKB,
		"vm-data-kb":  info.VmDataKB,
		"vm-stack-kb": info.VmStackKB,
		"threads":     info.Threads,
		keyState:      info.State,
		"wchan":       info.Wchan,
	}
	if info.FDCount != nil {
		data["fd-count"] = *info.FDCount
	}
	return data
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
