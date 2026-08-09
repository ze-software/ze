// Design: docs/architecture/testing/ci-format.md -- test plugin runner

package cli

import (
	"log/slog"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// cmdPluginExternal runs a REGISTERED engine plugin's own RunEngine function
// (the same function internal invocation mode passes a net.Pipe() end to)
// via a genuine external TLS connect-back instead. Test-only tool: proves a
// plugin's runtime IsInternal()-guarded refuse/warn behavior actually fires
// when launched as a real subprocess, not just against a synthetic
// net.Pipe() in a Go unit test.
//
// Mirrors the configure sequence internal/component/plugin's
// GetInternalPluginRunner applies for internal plugins (ConfigureEngineLogger
// with the canonical subsystem name) so the plugin's own logger actually
// writes somewhere observable instead of the discard logger most plugins
// default to at init() -- without this, a refuse/warn log line would be
// silently dropped before ever reaching this process's stderr, which the
// engine relays back to its own log output.
//
// Not a general-purpose external plugin launcher: production external
// plugins should be built as standalone binaries using pkg/plugin (see
// examples/plugin/go/main.go), which speak the same protocol without
// depending on the engine's own plugin registry being linked in.
func cmdPluginExternal(args []string) int {
	if len(args) != 1 {
		slog.Error("plugin-external: usage: ze-test plugin-external <registered-plugin-name>")
		return 1
	}
	name := args[0]

	reg := registry.Lookup(name)
	if reg == nil {
		slog.Error("plugin-external: unknown registered plugin", "name", name)
		return 1
	}
	if reg.RunEngine == nil {
		slog.Error("plugin-external: plugin has no RunEngine", "name", name)
		return 1
	}
	if reg.ConfigureEngineLogger != nil {
		reg.ConfigureEngineLogger(plugin.CanonicalSubsystemName(name))
	}

	conn, err := sdk.DialTLSEnvRaw(name)
	if err != nil {
		slog.Error("plugin-external: TLS connect failed", "name", name, "error", err)
		return 1
	}

	return reg.RunEngine(conn)
}
