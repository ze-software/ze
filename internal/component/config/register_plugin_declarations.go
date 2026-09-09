// Design: docs/architecture/api/commands.md -- where a command is served
// Related: loader.go -- LoadConfig, the one parse this reader calls
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

// readConfiguredPlugins answers the plugins a config file declares.
//
// It goes through LoadConfig, the one parse the daemon itself starts from, so
// the command reports the plugin set the daemon would start rather than a
// second reading of the same file. A config the daemon would refuse is refused
// here too, with the loader's own message.
func readConfiguredPlugins(path string) ([]plugin.PluginConfig, error) {
	input, err := cliio.ReadFile(path)
	if err != nil {
		return nil, err
	}
	loaded, err := LoadConfig(string(input), path, nil)
	if err != nil {
		return nil, err
	}
	return loaded.Plugins, nil
}
