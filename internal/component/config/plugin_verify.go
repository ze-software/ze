// Design: docs/architecture/config/yang-config-design.md -- static plugin config verification
// Related: ../plugin/server/startup.go -- runtime ConfigSection extraction

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/configorder"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// ParseTreeForValidation parses config text without applying environment/log
// side effects. It is for static validation paths that must inspect the same
// tree shape as LoadConfig without mutating process state.
func ParseTreeForValidation(input string) (*Tree, error) {
	tree, _, err := parseTreeWithYANG(input, nil)
	return tree, err
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
//
// It lowers with ToPluginMap, not ToMap, so a verifier reads the payload the
// configure callback will read. A verifier handed a list with no order would
// refuse a config the daemon then loads, or accept one the daemon then refuses.
func VerifyPluginConfig(tree *Tree) []error {
	if tree == nil {
		return nil
	}
	return verifyTree(nil, tree)
}

// verifyTree runs the dependency refusal over candidate, then the map door.
// The refusal reads the tree rather than a lowered map because the producers
// boot uses read the tree: ExtractPluginsFromTree for the `plugin { }` block and
// CollectContainerPaths for the roots that auto-load a plugin.
//
// An extraction failure is reported here, once, as a *PluginExtractionError,
// and the dependency refusal is then not asked: the set of plugins boot would
// start is unknown, so it cannot answer, and the extraction failure already
// refuses the config, as LoadConfig refuses it. A failure ExtractPluginsFromTree
// produced in the `plugin { }` block carries no Section. Doors whose own check
// of a section reports that section's failure skip the relayed copy by its
// Section; every other door reports it from here.
func verifyTree(previous map[string]any, candidate *Tree) []error {
	errs := VerifyPluginConfigMapTransition(previous, candidate.ToPluginMap())
	declared, err := ExtractPluginsFromTree(candidate)
	if err != nil {
		var extractErr *PluginExtractionError
		if !errors.As(err, &extractErr) {
			extractErr = &PluginExtractionError{Err: err}
		}
		return append([]error{extractErr}, errs...)
	}
	if err := refuseUnresolvableDependencies(candidate, declared); err != nil {
		errs = append([]error{err}, errs...)
	}
	return errs
}

// refuseUnresolvableDependencies refuses a config whose plugins cannot all
// start in this build, with the error boot refuses it with.
//
// It asks registry.ResolveDependencies for the set boot starts: the plugins
// declared, which verifyTree extracted with ExtractPluginsFromTree from the
// `plugin { }` block and the registered extractors (dependencyRequest, as
// ExpandDependencies asks at LoadConfig) and the plugins the config's roots select
// (registry.PluginsForConfigPaths, as boot auto-load asks in plugin/server
// getConfigPathPlugins), less the declared ones, which boot also skips. A hard
// dependency this build lacks answers *registry.MissingDependencyError, naming
// the plugin, the dependency and the build tag, so `ze config validate`, the
// editor and API commit, and doctor refuse what boot refuses, in the same
// words. Without it they called a config valid that the daemon then refused to
// start.
//
// Not covered: the FIB plugin boot adds for the configured data plane
// (appendDataPlaneWriter), because the data plane is read by the iface
// component, which config does not import.
func refuseUnresolvableDependencies(tree *Tree, declared []plugin.PluginConfig) error {
	names, external := dependencyRequest(declared)
	configured := make(map[string]bool, len(declared))
	for _, pc := range declared {
		for _, name := range plugin.RegistryNames(pc) {
			configured[name] = true
		}
	}
	for _, name := range registry.PluginsForConfigPaths(CollectContainerPaths(tree)) {
		if configured[name] {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil
	}

	_, err := registry.ResolveDependencies(names, external, ConfigNeedHolds(tree.ToMap()))
	return err
}

// VerifyPluginConfigTransition is VerifyPluginConfig with deletion awareness.
// If a registered root existed in previous but is absent from candidate, its
// verifier receives an empty JSON object for that root, matching reload.go.
func VerifyPluginConfigTransition(previous, candidate *Tree) []error {
	var oldMap map[string]any
	if previous != nil {
		oldMap = previous.ToPluginMap()
	}
	if candidate == nil {
		return nil
	}
	return verifyTree(oldMap, candidate)
}

// refuseRedistributeRulesMap runs RefuseRedistributeRules over a lowered
// config. A config with no `redistribute` root costs no rebuild.
func refuseRedistributeRulesMap(configTree map[string]any) error {
	// refuse-by-default: guards RefuseRedistributeRules -- a config with no redistribute root holds no rule to refuse
	if _, ok := configTree["redistribute"]; !ok {
		return nil
	}
	tree, err := TreeFromPluginMap(configTree)
	if err != nil {
		return fmt.Errorf("redistribute: rebuild the config to check its rules: %w", err)
	}
	return RefuseRedistributeRules(tree)
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
	// The same check LoadConfig runs at boot and reload, so validate, commit
	// and load agree on a config that needs a plugin this build lacks.
	if err := refuseAbsentOptionalProviders(func() map[string]any { return configTree }); err != nil {
		errs = append(errs, err)
	}
	// The same redistribute check LoadConfig runs, so validate and commit
	// refuse a rule the daemon would refuse at boot.
	if err := refuseRedistributeRulesMap(configTree); err != nil {
		errs = append(errs, err)
	}
	for _, reg := range registry.All() {
		if reg.InProcessConfigVerifier == nil || len(reg.ConfigRoots) == 0 {
			continue
		}
		sections, err := buildPluginConfigSectionsTransition(previousTree, configTree, reg.ConfigSectionRoots())
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
	var b textbuf.Buffer
	b.Str("plugin config verify failed:")
	for _, err := range errs {
		b.Str("\n  ").Err(err)
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
//
// When the extracted node is itself a list whose order was lowered
// (Tree.ToPluginMap), the order travels with it. The order of a list lives
// beside the list rather than inside it, so extracting the list alone would
// leave the order behind in the parent and hand the plugin a multi-entry list
// it has to refuse. Every list NESTED in the extracted subtree already carries
// its own order, because the subtree carries the container that holds both.
func ExtractConfigSubtree(configTree map[string]any, path string) any {
	if path == "*" {
		return configTree
	}

	parts := strings.FieldsFunc(path, func(r rune) bool { return r == '/' })
	if len(parts) == 0 {
		return configTree
	}

	var parent map[string]any
	var current any = configTree
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		parent = m
		current = m[part]
		if current == nil {
			return nil
		}
	}

	leaf := map[string]any{parts[len(parts)-1]: current}
	orderKey := configorder.OrderKey(parts[len(parts)-1])
	if order, ok := parent[orderKey]; ok {
		leaf[orderKey] = order
	}

	var result any = leaf
	for i := len(parts) - 2; i >= 0; i-- {
		result = map[string]any{parts[i]: result}
	}
	return result
}
