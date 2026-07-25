// Design: plan/learned/728-diag-netlink-monitor.md -- non-Linux stub for netlink monitor
// Related: monitor_netlink_linux.go -- full implementation
//
//go:build !linux

package cmd

import (
	"context"
	"errors"
	"io"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func streamNetlinkMonitor(_ context.Context, _ *pluginserver.Server, _ io.Writer, _ string, args []string) error {
	if _, err := netlinkGroupFromArgs(args); err != nil {
		return err
	}
	return errors.New("not available on this platform")
}
