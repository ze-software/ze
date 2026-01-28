package plugin

import (
	"io"

	"codeberg.org/thomas-mangin/ze/internal/plugin/gr"
	"codeberg.org/thomas-mangin/ze/internal/plugin/rib"
	"codeberg.org/thomas-mangin/ze/internal/plugin/rr"
)

// InternalPluginRunner is a function that runs a plugin in-process.
// It reads from in, writes to out, and returns an exit code.
type InternalPluginRunner func(in io.Reader, out io.Writer) int

// internalPluginRunners maps plugin names to their runner functions.
// This is the single source of truth for available internal plugins.
// Used by:
//   - IsInternalPlugin() in resolve.go
//   - AvailableInternalPlugins() in resolve.go
//   - startInternal() in process.go
var internalPluginRunners = map[string]InternalPluginRunner{
	"rib": func(in io.Reader, out io.Writer) int {
		return rib.NewRIBManager(in, out).Run()
	},
	"gr": func(in io.Reader, out io.Writer) int {
		return gr.NewGRPlugin(in, out).Run()
	},
	"rr": func(in io.Reader, out io.Writer) int {
		return rr.NewRouteServer(in, out).Run()
	},
}

// GetInternalPluginRunner returns the runner function for an internal plugin.
// Returns nil if the plugin is not found.
func GetInternalPluginRunner(name string) InternalPluginRunner {
	return internalPluginRunners[name]
}
