// Design: docs/architecture/iface/netlink-monitor.md -- non-Linux stub for netlink monitor
// Related: monitor_netlink_linux.go -- full implementation
//
//go:build !linux

package cmd

import (
	"context"
	"errors"
	"io"

	"github.com/ze-software/ze/internal/component/command"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func streamNetlinkMonitor(_ context.Context, _ *pluginserver.Server, _ io.Writer, _ string, args command.ValidatedArgs) error {
	if _, err := netlinkGroupFromArgs(args.Tokens()); err != nil {
		return err
	}
	return errors.New("not available on this platform")
}
