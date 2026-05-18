// Design: plan/spec-diag-netlink-monitor.md -- non-Linux stub for netlink monitor
// Related: sockets_other.go -- existing non-Linux stub pattern
//
//go:build !linux

package show

import (
	"context"
	"errors"
	"io"
	"strings"

	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

var errUnknownNetlinkGroup = errors.New("unknown netlink group (valid: route, link, address, all)")

func streamNetlinkMonitor(_ context.Context, _ *pluginserver.Server, _ io.Writer, _ string, args []string) error {
	group := netlinkGroupAll
	if len(args) > 0 {
		group = strings.ToLower(args[0])
	}
	switch group {
	case netlinkGroupRoute, netlinkGroupLink, netlinkGroupAddress, netlinkGroupAll:
	default:
		return errUnknownNetlinkGroup
	}
	return errors.New("not available on this platform")
}
