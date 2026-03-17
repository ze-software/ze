// Design: docs/architecture/api/commands.md — BGP plugin process configuration handlers
// Overview: doc.go — bgp-cmd-meta plugin overview

package meta

import (
	"fmt"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:plugin-encoding", Handler: handleBgpPluginEncoding, Help: "Set event encoding (json|text)"},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:plugin-format", Handler: handleBgpPluginFormat, Help: "Set wire format (hex|base64|parsed|full)"},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:plugin-ack", Handler: handleBgpPluginAck, Help: "Set ACK timing (sync|async)"},
	)
}

// handleBgpPluginEncoding sets event encoding for this process.
// Syntax: bgp plugin encoding <json|text>.
func handleBgpPluginEncoding(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("missing encoding: bgp plugin encoding <json|text>")
	}

	enc := strings.ToLower(args[0])
	switch enc {
	case plugin.EncodingJSON, plugin.EncodingText:
		if ctx.Process != nil {
			ctx.Process.SetEncoding(enc)
		}
	default: // invalid encoding -> return error
		return nil, fmt.Errorf("invalid encoding: %s (valid: json, text)", args[0])
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"encoding": enc,
		},
	}, nil
}

// handleBgpPluginFormat sets wire format for this process.
// Syntax: bgp plugin format <hex|base64|parsed|full>.
func handleBgpPluginFormat(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("missing format: bgp plugin format <hex|base64|parsed|full>")
	}

	format := strings.ToLower(args[0])
	switch format {
	case plugin.FormatHex, plugin.FormatBase64, plugin.FormatParsed, plugin.FormatFull:
		if ctx.Process != nil {
			ctx.Process.SetFormat(format)
		}
	default: // invalid format -> return error
		return nil, fmt.Errorf("invalid format: %s (valid: hex, base64, parsed, full)", args[0])
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"format": format,
		},
	}, nil
}

// handleBgpPluginAck sets ACK timing for this process.
// Syntax: bgp plugin ack <sync|async>.
func handleBgpPluginAck(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("missing mode: bgp plugin ack <sync|async>")
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
	default: // invalid mode -> return error
		return nil, fmt.Errorf("invalid mode: %s (valid: sync, async)", args[0])
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"ack": mode,
		},
	}, nil
}
