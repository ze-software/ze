// Design: docs/architecture/config/yang-config-design.md -- static plugin config verification
// Related: ../plugin/server/startup.go -- runtime ConfigSection extraction

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// ParseTreeForValidation parses config text without applying environment/log
// side effects. It is for static validation paths that must inspect the same
// tree shape as LoadConfig without mutating process state.
func ParseTreeForValidation(input string) (*Tree, error) {
	return parseTreeWithYANG(input, nil)
}

// VerifyPluginConfigContent parses content and runs registered in-process,
// side-effect-free plugin config verifiers. It does not call live external
// plugin OnConfigVerify callbacks or plugin OnConfigure callbacks.
func VerifyPluginConfigContent(input string) error {
	tree, err := ParseTreeForValidation(input)
	if err != nil {
		return err
	}
	return JoinPluginConfigVerifyErrors(VerifyPluginConfig(tree))
}

// VerifyPluginConfigContentTransition verifies a candidate config against the
// previous config, including roots deleted by the candidate. Deleted roots are
// delivered as "{}", matching runtime reload's OnConfigVerify payload.
func VerifyPluginConfigContentTransition(previous, candidate string) error {
	newTree, err := ParseTreeForValidation(candidate)
	if err != nil {
		return err
	}
	oldTree, _ := ParseTreeForValidation(previous)
	return JoinPluginConfigVerifyErrors(VerifyPluginConfigTransition(oldTree, newTree))
}

// VerifyPluginConfig runs side-effect-free in-process verifiers for plugins
// whose config roots are present in the candidate tree. Live external plugin
// OnConfigVerify callbacks participate only in daemon reload/commit.
func VerifyPluginConfig(tree *Tree) []error {
	if tree == nil {
		return nil
	}
	return VerifyPluginConfigMap(tree.ToMap())
}

// VerifyPluginConfigTransition is VerifyPluginConfig with deletion awareness.
// If a registered root existed in previous but is absent from candidate, its
// verifier receives an empty JSON object for that root, matching reload.go.
func VerifyPluginConfigTransition(previous, candidate *Tree) []error {
	var oldMap map[string]any
	if previous != nil {
		oldMap = previous.ToMap()
	}
	var newMap map[string]any
	if candidate != nil {
		newMap = candidate.ToMap()
	}
	return VerifyPluginConfigMapTransition(oldMap, newMap)
}

// VerifyPluginConfigMap is the map-form equivalent of VerifyPluginConfig.
func VerifyPluginConfigMap(configTree map[string]any) []error {
	return VerifyPluginConfigMapTransition(nil, configTree)
}

// VerifyPluginConfigMapTransition is the map-form equivalent of
// VerifyPluginConfigTransition.
func VerifyPluginConfigMapTransition(previousTree, configTree map[string]any) []error {
	if configTree == nil {
		return nil
	}

	var errs []error
	for _, reg := range registry.All() {
		if reg.InProcessConfigVerifier == nil || len(reg.ConfigRoots) == 0 {
			continue
		}
		sections, err := buildPluginConfigSectionsTransition(previousTree, configTree, reg.ConfigRoots)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", reg.Name, err))
			continue
		}
		if len(sections) == 0 {
			continue
		}
		if err := reg.InProcessConfigVerifier(sections); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", reg.Name, err))
		}
	}
	return errs
}

func buildPluginConfigSectionsTransition(previousTree, configTree map[string]any, roots []string) ([]rpc.ConfigSection, error) {
	sections := make([]rpc.ConfigSection, 0, len(roots))
	for _, root := range roots {
		subtree := ExtractConfigSubtree(configTree, root)
		if subtree == nil {
			if previousTree == nil || ExtractConfigSubtree(previousTree, root) == nil {
				continue
			}
			sections = append(sections, rpc.ConfigSection{Root: root, Data: "{}"})
			continue
		}
		data, err := json.Marshal(subtree)
		if err != nil {
			return nil, fmt.Errorf("marshal %s config: %w", root, err)
		}
		sections = append(sections, rpc.ConfigSection{Root: root, Data: string(data)})
	}
	return sections, nil
}

// JoinPluginConfigVerifyErrors converts verifier errors into a single message
// suitable for API commit hooks.
func JoinPluginConfigVerifyErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}
	var b strings.Builder
	b.WriteString("plugin config verify failed:")
	for _, err := range errs {
		fmt.Fprintf(&b, "\n  %v", err)
	}
	return errors.New(b.String())
}

// BuildPluginConfigSections builds the ConfigSection payload expected by
// OnConfigVerify from a full candidate config tree and declared config roots.
func BuildPluginConfigSections(configTree map[string]any, roots []string) ([]rpc.ConfigSection, error) {
	sections := make([]rpc.ConfigSection, 0, len(roots))
	for _, root := range roots {
		subtree := ExtractConfigSubtree(configTree, root)
		if subtree == nil {
			continue
		}
		data, err := json.Marshal(subtree)
		if err != nil {
			return nil, fmt.Errorf("marshal %s config: %w", root, err)
		}
		sections = append(sections, rpc.ConfigSection{Root: root, Data: string(data)})
	}
	return sections, nil
}

// ExtractConfigSubtree extracts a subtree from configTree and wraps it in its
// full path, matching the runtime plugin server's ConfigSection shape.
func ExtractConfigSubtree(configTree map[string]any, path string) any {
	if path == "*" {
		return configTree
	}

	parts := strings.FieldsFunc(path, func(r rune) bool { return r == '/' })
	if len(parts) == 0 {
		return configTree
	}

	var current any = configTree
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = m[part]
		if current == nil {
			return nil
		}
	}

	result := current
	for i := len(parts) - 1; i >= 0; i-- {
		result = map[string]any{parts[i]: result}
	}
	return result
}
