// Design: docs/architecture/api/commands.md -- config archive trigger handler

package cmd

import (
	"os"

	iconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/archive"
	"github.com/ze-software/ze/internal/component/config/system"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-config-archive:trigger",
			Handler:    handleArchiveTrigger,
		},
	)
}

func handleArchiveTrigger(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) == 0 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "usage: config archive <name>",
		}, nil
	}

	archiveName := args[0]

	if ctx.Server == nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "server not available",
		}, nil
	}

	configPath := ctx.Server.ConfigPath()
	if configPath == "" {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "no config file path available",
		}, nil
	}

	var tb textbuf.Buffer
	data, err := os.ReadFile(configPath) //nolint:gosec // Config path from daemon startup
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb.Str("read config: ").Err(err).String(),
		}, err
	}

	schema, schErr := iconfig.YANGSchema()
	if schErr != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb.Reset().Str("load schema: ").Err(schErr).String(),
		}, schErr
	}

	parser := iconfig.NewParser(schema)
	tree, parseErr := parser.Parse(string(data))
	if parseErr != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb.Reset().Str("parse config: ").Err(parseErr).String(),
		}, parseErr
	}

	sys := system.ExtractSystemConfig(tree)
	configs := archive.ExtractConfigs(tree)
	if len(configs) == 0 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "no archive blocks configured in system { archive { } }",
		}, nil
	}

	var ac *archive.ArchiveConfig
	for i := range configs {
		if configs[i].Name == archiveName {
			ac = &configs[i]
			break
		}
	}

	if ac == nil {
		names := make([]string, 0, len(configs))
		for _, c := range configs {
			names = append(names, c.Name)
		}
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb.Reset().Str("archive block ").Str(archiveName).Str(" not found (available: ").Join(names, ", ").Byte(')').String(),
		}, nil
	}

	if err := archive.ValidateLocation(ac.Location); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb.Reset().Str("invalid location for ").Str(archiveName).Str(": ").Err(err).String(),
		}, err
	}

	var eventFn archive.EventEmitter
	if ctx.Server != nil {
		eventFn = func(_, _ string, content []byte) {
			ctx.Server.EmitEngineEvent("config", "archive", content) //nolint:errcheck // best-effort event
		}
	}

	notifier := archive.NewNotifier(configPath, []archive.ArchiveConfig{*ac}, &sys, eventFn)
	errs := notifier(data)

	if len(errs) > 0 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb.Reset().Str("archive ").Str(archiveName).Str(" failed: ").Err(errs[0]).String(),
		}, nil
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"message": tb.Reset().Str("archived ").Str(archiveName).Str(" to ").Str(archive.RedactURL(ac.Location)).String(),
		},
	}, nil
}
