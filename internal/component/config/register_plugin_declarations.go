// Design: docs/architecture/api/commands.md -- where a command is served
// Related: loader.go -- ReadConfig, the one parse this reader calls
// Related: declared_schemas.go -- PublishDeclaredSchemas, which says whether this process has a daemon in it
// Related: ../plugin/declarations.go -- `show plugin declarations`, the reader's one caller
//
// register_plugin_declarations.go closes the seam that lets `show plugin
// declarations` answer for the plugins a config file names.
//
// The dependency runs one way: this package parses the config and imports
// `internal/component/plugin` for PluginConfig, so the plugin package cannot
// import it back. The plugin package therefore declares the reader type and
// this package registers a reader for it, which is the same registration
// pattern every other cross-component seam uses.

package config

import (
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/cliio"
)

func init() {
	plugin.SetConfiguredPluginReader(readConfiguredPlugins)
}

// readConfiguredPlugins answers the plugins a config file declares, and
// whether the process that read it is a command with no daemon or a running
// daemon.
//
// It goes through ReadConfig, the body of LoadConfig the daemon itself starts
// from, so the command reports the plugin set the daemon would start rather
// than a second reading of the same file. A config the daemon would refuse is
// refused here too, with the loader's own message. ReadConfig and not
// LoadConfig, because this command is daemon-served: LoadConfig would push the
// file's environment block into the answering daemon's log and env settings.
//
// The file is parsed whole, after the step that reads which external plugins
// it declares (PublishDeclaredSchemas). Served by a daemon, a section of a
// plugin the daemon started with is read; with no daemon it is refused, with
// the command that reaches a running ze. The first
// pass alone would not answer this list: it leaves out the compiled-in
// plugins, the plugins a section declares inline and their dependencies.
//
// The process is the answer of that same step (DeclaredSchemas.Offline), so
// the command that starts no external plugin with no daemon and the parse that
// refuses an external plugin's section with no daemon decide it in one place.
func readConfiguredPlugins(path string) (plugin.ConfiguredPlugins, error) {
	input, err := cliio.ReadFile(path)
	if err != nil {
		return plugin.ConfiguredPlugins{}, err
	}
	declared, err := PublishDeclaredSchemas(ConfigSource{Text: string(input), Path: path})
	if err != nil {
		return plugin.ConfiguredPlugins{}, err
	}
	loaded, err := ReadConfig(string(input), path, nil)
	if err != nil {
		return plugin.ConfiguredPlugins{}, declared.Explain(err)
	}
	process := plugin.ConfigProcessDaemon
	if declared.Offline() {
		process = plugin.ConfigProcessOffline
	}
	return plugin.ConfiguredPlugins{Plugins: loaded.Plugins, Process: process}, nil
}
