// Design: docs/architecture/api/commands.md -- config archive trigger handler

package archive

import (
	"os"
	"strings"

	iconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/archive"
	"codeberg.org/thomas-mangin/ze/internal/component/config/system"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
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
			Data:   "usage: config archive <name>",
		}, nil
	}

	archiveName := args[0]

	if ctx.Server == nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "server not available",
		}, nil
	}

	configPath := ctx.Server.ConfigPath()
	if configPath == "" {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "no config file path available",
		}, nil
	}

	data, err := os.ReadFile(configPath) //nolint:gosec // Config path from daemon startup
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "read config: " + err.Error(),
		}, err
	}

	schema, schErr := iconfig.YANGSchema()
	if schErr != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "load schema: " + schErr.Error(),
		}, schErr
	}

	parser := iconfig.NewParser(schema)
	tree, parseErr := parser.Parse(string(data))
	if parseErr != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "parse config: " + parseErr.Error(),
		}, parseErr
	}

	sys := system.ExtractSystemConfig(tree)
	configs := archive.ExtractConfigs(tree)
	if len(configs) == 0 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "no archive blocks configured in system { archive { } }",
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
			Data:   "archive block " + archiveName + " not found (available: " + strings.Join(names, ", ") + ")",
		}, nil
	}

	if err := archive.ValidateLocation(ac.Location); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "invalid location for " + archiveName + ": " + err.Error(),
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
			Data:   "archive " + archiveName + " failed: " + errs[0].Error(),
		}, nil
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"message": "archived " + archiveName + " to " + ac.Location,
		},
	}, nil
}
