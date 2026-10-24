// Design: docs/architecture/api/commands.md -- show config running, the configuration the daemon applied
// Related: register_running_rpc.go -- registers the handler.
// Related: cmd_dump.go -- maskMapValues, the same masking by leaf name over a file's tree

package cli

import (
	"errors"
	"slices"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// errNoRunningConfig is the refusal of a daemon that holds no running tree.
// Answering an empty map instead would read as "the daemon runs an empty
// config", which no caller could tell from the failure.
var errNoRunningConfig = errors.New("show config running: this daemon holds no running configuration yet.\n" +
	"The configuration is installed when the daemon finishes loading it. Wait for start-up to complete, then run:\n" +
	"  show config running")

// handleShowConfigRunning answers `show config running`: the configuration the
// daemon applied, read from the running tree the reload commits
// (plugin.Coordinator.GetConfigTree, replaced by SetConfigTree only after a
// reload's transaction committed), never from the config file. A reload the
// daemon refused leaves that tree untouched, so this answers the config still
// in force while the file already holds the refused text.
//
// Every secret leaf (ze:sensitive or ze:bcrypt, config.SecretKeys) is replaced
// with config.SecretDataPlaceholder. The answer leaves the daemon over SSH,
// REST and gRPC, so it never carries the $9$ form `show config dump` writes:
// that form is reversible.
func handleShowConfigRunning(ctx *pluginserver.CommandContext, _ command.ValidatedArgs) (*zePlugin.Response, error) {
	if ctx == nil {
		return runningRefusal(errNoRunningConfig), nil
	}
	lifecycle := ctx.Reactor()
	if lifecycle == nil {
		return runningRefusal(errNoRunningConfig), nil
	}
	tree := lifecycle.GetConfigTree()
	if tree == nil {
		return runningRefusal(errNoRunningConfig), nil
	}
	schema, err := config.YANGSchema()
	if err != nil {
		return runningRefusal(errors.Join(errors.New("show config running: the YANG schema did not load, so secret leaves cannot be masked"), err)), nil
	}
	return &zePlugin.Response{
		Status: zePlugin.StatusDone,
		Data:   zePlugin.Map(runningConfigView(tree, config.SecretKeys(schema))),
	}, nil
}

// runningConfigView answers a deep copy of the running tree with every string
// under a secret leaf name replaced by the placeholder. It copies rather than
// masks in place because the tree is the daemon's live running configuration:
// the next reload diffs against it.
//
// The recursion is bounded by the YANG schema's depth: the tree is the daemon's
// own parsed config, never input a peer controls.
func runningConfigView(tree map[string]any, secretKeys map[string]bool) map[string]any {
	view := make(map[string]any, len(tree))
	for key, value := range tree {
		view[key] = runningConfigValue(key, value, secretKeys)
	}
	return view
}

// runningConfigValue copies one value of the running tree under its leaf name.
func runningConfigValue(key string, value any, secretKeys map[string]bool) any {
	switch typed := value.(type) {
	case map[string]any:
		return runningConfigView(typed, secretKeys)
	case []any:
		list := make([]any, len(typed))
		for i, item := range typed {
			list[i] = runningConfigValue(key, item, secretKeys)
		}
		return list
	case []string:
		return slices.Clone(typed)
	case string:
		if secretKeys[key] && typed != "" {
			return config.SecretDataPlaceholder
		}
		return typed
	}
	return value
}

// runningRefusal is the error answer of `show config running`.
func runningRefusal(err error) *zePlugin.Response {
	return &zePlugin.Response{Status: zePlugin.StatusError, Error: err.Error()}
}
