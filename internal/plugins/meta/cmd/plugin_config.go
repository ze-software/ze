// Design: docs/architecture/api/commands.md — plugin process configuration handlers

package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

var (
	errMissingEncodingBgpPluginEncodingJsontext        = errors.New("missing encoding: bgp plugin encoding <json|text>")
	errMissingFormatBgpPluginFormatHexbase64parsedfull = errors.New("missing format: bgp plugin format <hex|base64|parsed|full>")
	errMissingModeBgpPluginAckSyncasync                = errors.New("missing mode: bgp plugin ack <sync|async>")
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:plugin-encoding", Handler: handleBgpPluginEncoding},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:plugin-format", Handler: handleBgpPluginFormat},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:plugin-ack", Handler: handleBgpPluginAck},
	)
}

func handleBgpPluginEncoding(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) == 0 {
		return nil, errMissingEncodingBgpPluginEncodingJsontext
	}

	enc := strings.ToLower(args[0])
	switch enc {
	case plugin.EncodingJSON, plugin.EncodingText:
		if ctx.Process != nil {
			ctx.Process.SetEncoding(enc)
		}
	default:
		return nil, fmt.Errorf("invalid encoding: %s (valid: json, text)", args[0])
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"encoding": enc,
		},
	}, nil
}

func handleBgpPluginFormat(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) == 0 {
		return nil, errMissingFormatBgpPluginFormatHexbase64parsedfull
	}

	format := strings.ToLower(args[0])
	switch format {
	case plugin.FormatHex, plugin.FormatBase64, plugin.FormatParsed, plugin.FormatFull:
		if ctx.Process != nil {
			ctx.Process.SetFormat(format)
		}
	default:
		return nil, fmt.Errorf("invalid format: %s (valid: hex, base64, parsed, full)", args[0])
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"format": format,
		},
	}, nil
}

func handleBgpPluginAck(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) == 0 {
		return nil, errMissingModeBgpPluginAckSyncasync
	}

	mode := strings.ToLower(args[0])
	switch mode {
	case "sync":
		if ctx.Process != nil {
			ctx.Process.SetSync(true)
		}
	case "async":
		if ctx.Process != nil {
			ctx.Process.SetSync(false)
		}
	default:
		return nil, fmt.Errorf("invalid mode: %s (valid: sync, async)", args[0])
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"ack": mode,
		},
	}, nil
}
