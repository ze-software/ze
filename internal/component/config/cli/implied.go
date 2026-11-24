// Design: docs/architecture/api/architecture.md -- shared `| implied` pipe helpers and `{config, implied}` text renderer for config view commands.
// Related: register_running_rpc.go -- registers `| implied` on `show config running`.
// Related: running_rpc.go -- daemon `show config running | implied` handler.
// Related: register.go -- registers `| implied` on `show config source` and `show config dump`, plus the envelope text renderer.
// Related: config_supplied_tools.go -- offline `show config source | implied` handler.
// Related: config_data.go -- offline `show config dump | implied` handler.
// Related: cmd_show.go -- `ze config show` stderr plain-display notice.

package cli

import (
	"encoding/json"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
)

const (
	pipeOpImplied         = "implied"
	impliedFilterDesc     = "Include registered defaults and implied configuration beside written config"
	impliedKeyConfig      = "config"
	impliedKeyImplied     = "implied"
	maxImpliedConfigDepth = 64
)

// hasPipeOperation reports whether ops contains a declared operation with name
// (matched case-insensitively).
func hasPipeOperation(ops []command.PipeOperation, name string) bool {
	for _, op := range ops {
		if strings.EqualFold(op.Name, name) {
			return true
		}
	}
	return false
}

// impliedEnvelopeWire decodes the JSON `{config, implied}` envelope produced
// by `show config running | implied`, `show config source | implied`, and
// `show config dump | implied`.
type impliedEnvelopeWire struct {
	Config  map[string]any       `json:"config"`
	Implied config.ImpliedReport `json:"implied"`
}

// renderImpliedEnvelopeText is the registered `| text` renderer for the
// `{config, implied}` envelope. It renders the masked written configuration in
// hierarchical Ze syntax and combines it with report via
// config.RenderImpliedText so `# implied by:` blocks and inline `# implied:`
// annotations appear beside the written configuration while `| json` and
// `| yaml` keep the structured envelope unchanged.
func renderImpliedEnvelopeText(answer json.RawMessage) (string, error) {
	var env impliedEnvelopeWire
	if err := json.Unmarshal(answer, &env); err != nil {
		return "", err
	}
	schema, err := config.YANGSchema()
	if err != nil {
		return "", err
	}
	writtenText := ""
	if len(env.Config) > 0 {
		cfgTree, err := treeFromImpliedConfigMap(env.Config)
		if err != nil {
			return "", err
		}
		writtenText = config.Serialize(cfgTree, schema)
	}
	return config.RenderImpliedText(writtenText, env.Implied), nil
}

// treeFromRunningMap reconstructs a *config.Tree from the daemon's active
// running configuration map. It uses config.TreeFromPluginMap directly on the
// common path and falls back to cleaning non-plugin scalar types (such as Go
// ints in synthetic test maps) only when direct reconstruction fails.
func treeFromRunningMap(m map[string]any) (*config.Tree, error) {
	return treeFromImpliedConfigMap(m)
}

// treeFromImpliedConfigMap reconstructs a *config.Tree from a masked or
// resolved configuration map (such as `show config dump`'s dumpMap) for text
// serialization and offline implied evaluation.
func treeFromImpliedConfigMap(m map[string]any) (*config.Tree, error) {
	if len(m) == 0 {
		return config.NewTree(), nil
	}
	tree, err := config.TreeFromPluginMap(m)
	if err == nil {
		return tree, nil
	}
	cleaned := cleanImpliedConfigMap(m, 0)
	return config.TreeFromPluginMap(cleaned)
}

// cleanImpliedConfigMap produces a TreeFromPluginMap-compatible copy of m,
// converting Go integer scalars to float64 and dropping nil values or
// resolved non-string object slices that have no hierarchical config syntax.
// Recursion is bounded by maxImpliedConfigDepth.
func cleanImpliedConfigMap(m map[string]any, depth int) map[string]any {
	if depth >= maxImpliedConfigDepth {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		if v == nil {
			continue
		}
		switch typed := v.(type) {
		case string, bool, float64, []string:
			out[k] = typed
		case int:
			out[k] = float64(typed)
		case int64:
			out[k] = float64(typed)
		case uint64:
			out[k] = float64(typed)
		case map[string]any:
			sub := cleanImpliedConfigMap(typed, depth+1)
			if sub != nil {
				out[k] = sub
			}
		case []any:
			if stringsOnly, ok := stringSliceFromAny(typed); ok {
				out[k] = stringsOnly
			}
		}
	}
	return out
}

func stringSliceFromAny(items []any) ([]string, bool) {
	out := make([]string, 0, len(items))
	for _, item := range items {
		s, ok := item.(string)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}
