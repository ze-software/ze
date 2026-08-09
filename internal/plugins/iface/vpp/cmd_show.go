// Design: docs/architecture/firewall/backend-command-dispatch.md -- VPP dataplane trace handlers

package ifacevpp

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	vppcomp "github.com/ze-software/ze/internal/component/vpp"
)

const maxTraceCount = 10000

var validNodeName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:vpp-trace-start", Handler: handleVPPTraceStart},
		pluginserver.RPCRegistration{WireMethod: "ze-show:vpp-trace-show", Handler: handleVPPTraceShow},
		pluginserver.RPCRegistration{WireMethod: "ze-show:vpp-trace-clear", Handler: handleVPPTraceClear},
		pluginserver.RPCRegistration{WireMethod: "ze-show:vpp-runtime", Handler: handleVPPRuntime},
	)
}

func handleVPPTraceStart(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	inputNode := "dpdk-input"
	count := 100
	for i, a := range args {
		switch a {
		case "node":
			if i+1 < len(args) {
				inputNode = args[i+1]
			}
		case "count":
			if i+1 < len(args) {
				if n, err := strconv.Atoi(args[i+1]); err == nil && n > 0 {
					count = min(n, maxTraceCount)
				}
			}
		}
	}
	if !validNodeName.MatchString(inputNode) {
		return &plugin.Response{Status: plugin.StatusError, Error: "invalid node name: must match [a-zA-Z0-9_-]+"}, nil
	}
	output, err := vppcomp.TraceStart(inputNode, count)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"command":    "trace add " + inputNode,
			"count":      count,
			"output":     strings.TrimSpace(output),
			"input-node": inputNode,
		},
	}, nil
}

func handleVPPTraceShow(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	output, err := vppcomp.TraceShow()
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error
	}
	lines := strings.Count(output, "\n")
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"command": "show trace",
			"output":  strings.TrimSpace(output),
			"lines":   lines,
		},
	}, nil
}

func handleVPPTraceClear(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	output, err := vppcomp.TraceClear()
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"command": "clear trace",
			"output":  strings.TrimSpace(output),
		},
	}, nil
}

func handleVPPRuntime(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	output, err := vppcomp.ShowRuntime()
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error
	}
	lines := strings.Count(output, "\n")
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"command": "show runtime",
			"output":  strings.TrimSpace(output),
			"lines":   lines,
		},
	}, nil
}
