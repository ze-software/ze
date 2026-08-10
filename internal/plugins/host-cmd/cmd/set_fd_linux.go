// Design: docs/architecture/host/inventory.md -- runtime FD limit adjustment

//go:build linux

package cmd

import (
	"strconv"
	"syscall"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func registerSetFD() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-set:system-file-descriptors",
			Handler:    handleSetSystemFD,
		},
	)
}

func handleSetSystemFD(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) == 0 {
		return &plugin.Response{Status: plugin.StatusError, Error: "usage: set system file-descriptors <limit|max>"}, nil
	}

	var current syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &current); err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "getrlimit: " + err.Error()}, nil //nolint:nilerr // operational error in Response
	}

	var requested uint64
	if args[0] == "max" {
		requested = current.Max
	} else {
		var err error
		requested, err = strconv.ParseUint(args[0], 10, 64)
		if err != nil || requested == 0 {
			return &plugin.Response{Status: plugin.StatusError, Error: "invalid limit: " + args[0]}, nil //nolint:nilerr // invalid input surfaced via Response.Error, not a Go error (matches file:32)
		}
		if requested > current.Max {
			msg := "requested " + args[0] + " exceeds hard limit " + textbuf.StringUint(current.Max)
			return &plugin.Response{Status: plugin.StatusError, Error: msg}, nil
		}
	}

	prev := current.Cur
	current.Cur = requested
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &current); err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "setrlimit: " + err.Error()}, nil //nolint:nilerr // operational error in Response
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"previous":   prev,
			"current":    requested,
			"hard-limit": current.Max,
		},
	}, nil
}
