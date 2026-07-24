// Design: plan/learned/673-diag-0-umbrella.md -- control-plane capture display

package cmd

import (
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func HandleShowCapture(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	protocol := ""
	tunnelIDFilter := uint16(0)
	peerFilter := ""
	limit := 0
	for i, a := range args {
		switch a {
		case capL2TP, capBGP:
			protocol = a
		case "tunnel-id":
			if i+1 < len(args) {
				if n, err := strconv.ParseUint(args[i+1], 10, 16); err == nil {
					tunnelIDFilter = uint16(n)
				}
			}
		case "peer":
			if i+1 < len(args) {
				peerFilter = args[i+1]
			}
		case argCount:
			if i+1 < len(args) {
				if n, err := strconv.Atoi(args[i+1]); err == nil && n > 0 {
					limit = n
				}
			}
		}
	}

	result := map[string]any{}

	// The l2tp branch lives in capture_l2tp.go (//go:build ze_l2tp) with a
	// not-in-this-build stub counterpart, so this always-on dispatcher never
	// imports the gated l2tp package.
	captureL2TPInto(result, protocol, limit, tunnelIDFilter, peerFilter)

	if protocol == "" || protocol == capBGP {
		if ctx != nil && ctx.Reactor() != nil {
			if cp, ok := ctx.Reactor().(plugin.BGPCaptureProvider); ok {
				entries := cp.BGPCaptureSnapshot(limit, peerFilter)
				if entries != nil {
					result["bgp"] = entries
					result["bgp-count"] = len(entries)
				} else {
					result["bgp"] = "capture not enabled"
				}
			}
		} else if protocol == capBGP {
			result["bgp"] = "reactor not available"
		}
	}

	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(result)}, nil
}
