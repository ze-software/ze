//go:build ze_ssh && ze_web

package hub

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/command"
	yangloader "github.com/ze-software/ze/internal/component/config/yang"
	_ "github.com/ze-software/ze/internal/component/plugin/all"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// monitorVerb is the verb root every monitor command lives under.
const monitorVerb = "monitor"

// TestEveryMonitorCommandStreams proves AC-19: every monitor command streams
// live on the four terminal surfaces, `ze monitor ...`, `ze cli -c`, the SSH
// interactive session (and the `ze cli` TUI, the same model) and SSH exec.
//
// Method: the monitor commands are read from the registries (streaming
// prefixes, CLI live views, and the YANG command paths under the monitor verb
// whose RPC is registered), never from a list. Each is judged by
// monitorStreamGaps against the predicate each surface's real route branches
// on. A gap names the command and the surface that answers it once instead of
// streaming. TestMonitorStreamGapsDiscriminate proves the judgment goes red
// when a streaming registration is missing.
func TestEveryMonitorCommandStreams(t *testing.T) {
	commands := monitorCommands(t)
	if len(commands) == 0 {
		t.Fatal("no monitor command was enumerated: the registries are empty")
	}
	for _, gap := range monitorStreamGaps(commands) {
		t.Error(gap)
	}
}

// TestMonitorStreamGapsDiscriminate proves the AC-19 judgment can fail: a
// monitor command no registry streams (a path no handler, view or local
// handler holds) must be named on all four surfaces. The judgment over the
// real registries was also run with one registration removed (the
// RegisterStreamingHandler call of monitor traceroute deleted) and went red
// on SSH exec and `ze cli -c` for that command.
func TestMonitorStreamGapsDiscriminate(t *testing.T) {
	const unstreamed = monitorVerb + " no-such-stream"
	if pluginserver.IsStreamingCommand(unstreamed) {
		t.Fatalf("%q is a streaming command: pick a path nothing registers", unstreamed)
	}
	gaps := monitorStreamGaps([]string{unstreamed})
	for _, surface := range []string{"SSH exec", "ze cli -c", "SSH interactive", "ze monitor"} {
		if !slices.ContainsFunc(gaps, func(g string) bool {
			return strings.HasPrefix(g, `"`+unstreamed+`" on `+surface+":")
		}) {
			t.Errorf("an unstreamed monitor command passes %s: %v", surface, gaps)
		}
	}
}

// monitorStreamGaps answers, for each monitor command, every surface whose
// real route would not stream it. Each predicate is the one the route itself
// branches on:
//
//   - SSH exec: internal/component/ssh exec dispatch sends a command to the
//     streaming executor when pluginserver.IsStreamingCommand holds, and the
//     executor (service_ssh.go) runs the handler
//     pluginserver.GetStreamingHandlerForCommand answers for it.
//   - `ze cli -c`: internal/component/cli/client main streams when
//     pluginserver.IsStreamingCommand holds (isMonitorCommand).
//   - SSH interactive and the `ze cli` TUI: the model (model_keys.go) opens
//     the registered live view first, then a streaming session when
//     pluginserver.IsStreamingCommand holds.
//   - `ze monitor ...`: cmdutil runs a registered local handler in-process (a
//     live loop), else hands the command to `ze cli -c`, the route above.
func monitorStreamGaps(commands []string) []string {
	views := cli.RegisteredViews()
	var gaps []string
	for _, cmd := range commands {
		streaming := pluginserver.IsStreamingCommand(cmd)
		if !streaming {
			gaps = append(gaps,
				`"`+cmd+`" on SSH exec: no streaming handler, the exec channel answers one RPC`,
				`"`+cmd+`" on ze cli -c: no streaming handler, the client answers one RPC`)
		}
		if streaming {
			handler, _, err := pluginserver.GetStreamingHandlerForCommand(cmd)
			if handler == nil && err == nil {
				gaps = append(gaps, `"`+cmd+`" on SSH exec: the streaming executor finds no handler`)
			}
		}
		if !streaming && !hasViewFor(views, cmd) {
			gaps = append(gaps, `"`+cmd+`" on SSH interactive: neither a live view nor a streaming handler`)
		}
		if !streaming && !command.HasLocal(cmd) {
			gaps = append(gaps, `"`+cmd+`" on ze monitor: neither a local live loop nor a streaming handler`)
		}
	}
	return gaps
}

// monitorCommands enumerates every monitor command from the registries.
func monitorCommands(t *testing.T) []string {
	t.Helper()
	schema, err := yangloader.DefaultLoader()
	if err != nil {
		t.Fatalf("load YANG: %v", err)
	}
	seen := make(map[string]bool)
	var commands []string
	add := func(path string) {
		key := strings.ToLower(strings.TrimSpace(path))
		if !isMonitorPath(key) || seen[key] {
			return
		}
		seen[key] = true
		commands = append(commands, key)
	}
	for _, prefix := range pluginserver.StreamingPrefixes() {
		add(prefix)
	}
	for _, v := range cli.RegisteredViews() {
		add(v.Prefix)
	}
	wireToPaths := yangloader.WireMethodToPaths(schema)
	for _, rpc := range pluginserver.AllBuiltinRPCs() {
		if rpc.Handler == nil {
			continue
		}
		for _, p := range wireToPaths[rpc.WireMethod] {
			add(p)
		}
	}
	slices.Sort(commands)
	return commands
}

func isMonitorPath(path string) bool {
	return strings.HasPrefix(path, monitorVerb+" ")
}

// hasViewFor reports whether a registered live view's prefix covers cmd on a
// word boundary, the match the model's view registry applies.
func hasViewFor(views []cli.ViewInfo, cmd string) bool {
	for _, v := range views {
		if cmd == v.Prefix || strings.HasPrefix(cmd, v.Prefix+" ") {
			return true
		}
	}
	return false
}
