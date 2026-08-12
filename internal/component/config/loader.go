// Design: docs/architecture/config/syntax.md -- config file loading and plugin extraction
// Related: loader_extract.go -- environment service config extraction (web, mcp, lg, hub)

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var errPluginAutoNotYetImplemented = errors.New("plugin 'auto' not yet implemented")

// LoadConfigResult holds the output of LoadConfig: a parsed config tree,
// resolved plugin list, and derived config directory.
type LoadConfigResult struct {
	Tree      *Tree
	Plugins   []plugin.PluginConfig
	ConfigDir string
}

// LoadConfig parses config with CLI plugin YANG schemas, extracts and resolves
// the plugin list, and returns the parsed tree + plugins without creating a reactor.
func LoadConfig(input, configPath string, cliPlugins []string) (*LoadConfigResult, error) {
	pluginYANG := plugin.CollectPluginYANG(cliPlugins)

	tree, err := ParseTreeWithYANG(input, pluginYANG)
	if err != nil {
		return nil, err
	}

	// Apply the registered ze:validate custom validators. Until this call
	// existed, ValidateTreeAllModules had exactly one non-test caller
	// (`ze config validate`), so every custom validator was bypassed at daemon
	// start and at SIGHUP reload and a hand-edited value reached the wire
	// unvalidated (spec-fixit-config-validators-bypassed-at-startup).
	//
	// Refusing here is also what makes the reload refuse, with no machinery of
	// its own: runReload (cmd/ze/hub/main_reload.go) turns this error into
	// "reload: parse config", clears the staged candidate and returns before
	// ReloadConfig, the provider refresh and engine.Reload run, so the daemon
	// keeps serving the config it already has (Thomas, 2026-08-11).
	if err := refuseInvalidCustomSections(tree); err != nil {
		return nil, err
	}

	plugins, err := ExtractPluginsFromTree(tree)
	if err != nil {
		return nil, err
	}

	plugins, err = MergeCliPlugins(plugins, cliPlugins)
	if err != nil {
		return nil, fmt.Errorf("resolve plugins: %w", err)
	}

	plugins, err = ExpandDependencies(plugins)
	if err != nil {
		return nil, err
	}

	var configDir string
	if configPath != "" && configPath != "-" {
		configDir = filepath.Dir(configPath)
	} else {
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			return nil, fmt.Errorf("get working directory: %w", cwdErr)
		}
		configDir = cwd
	}

	return &LoadConfigResult{
		Tree:      tree,
		Plugins:   plugins,
		ConfigDir: configDir,
	}, nil
}

// ParseTreeWithYANG parses config with optional plugin YANG schemas.
// Returns the parsed tree for further processing by callers.
func ParseTreeWithYANG(input string, pluginYANG map[string]string) (*Tree, error) {
	tree, err := parseTreeWithYANG(input, pluginYANG)
	if err != nil {
		return nil, err
	}

	envValues := ExtractEnvironment(tree)
	slogutil.ApplyLogConfig(envValues)
	ApplyEnvConfig(envValues)

	return tree, nil
}

func parseTreeWithYANG(input string, pluginYANG map[string]string) (*Tree, error) {
	var schema *Schema
	var schemaErr error
	if len(pluginYANG) > 0 {
		schema, schemaErr = YANGSchemaWithPlugins(pluginYANG)
	} else {
		schema, schemaErr = YANGSchema()
	}
	if schemaErr != nil {
		return nil, fmt.Errorf("YANG schema: %w", schemaErr)
	}

	format := DetectFormat(input)

	var tree *Tree
	var err error
	switch format {
	case FormatSetMeta:
		tree, err = parseSetWithMigration(schema, input, true)
	case FormatSet:
		tree, err = parseSetWithMigration(schema, input, false)
	case FormatHierarchical:
		p := NewParser(schema)
		tree, err = p.Parse(input)
		if err != nil {
			if hint := detectLegacySyntaxHint(input, err); hint != "" {
				return nil, fmt.Errorf("parse config: %w\n\n%s", err, hint)
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Prune inactive containers and list entries before extracting environment
	// values: `inactive:` is the operator's way to comment out a subtree, and
	// extracting it here would set env vars the operator explicitly disabled.
	// Env-level plumbing that depends on the full tree happens downstream in
	// CreateReactorFromTree (and runs after this prune).
	PruneInactive(tree, schema)

	return tree, nil
}

// parseSetWithMigration parses set-format config, applying migrations for stale fields.
func parseSetWithMigration(schema *Schema, input string, hasMeta bool) (*Tree, error) {
	sp := NewSetParser(schema)
	if hasMeta {
		tree, _, err := sp.ParseWithMeta(input)
		if err == nil {
			return tree, nil
		}
	} else {
		tree, err := sp.Parse(input)
		if err == nil {
			return tree, nil
		}
	}

	sp2 := NewSetParser(schema)
	sp2.SetPreMigration(true)
	var tree *Tree
	var err error
	if hasMeta {
		tree, _, err = sp2.ParseWithMeta(input)
	} else {
		tree, err = sp2.Parse(input)
	}
	if err != nil {
		return nil, err
	}

	if fn := getMigrateFunc(); fn != nil {
		if applied, migrateErr := fn(tree); migrateErr == nil && len(applied) > 0 {
			loaderLogger().Info("applied config migrations", "count", len(applied), "migrations", applied)
		}
	}

	// Fail closed on any field the lenient pass dropped. The pre-migration
	// parse records each unknown field as a warning and PRUNES it from the tree
	// (setparser.go walkAndSet/walkAndDelete/walkAndMarkInactive and the meta
	// walk), so a tree-level migration -- which only rewrites data present in
	// the tree -- can never heal the pruned field: a surviving warning is
	// silent config loss, not a heal-able rename. (A field the schema knows
	// makes the strict parse above succeed and return early, so reaching this
	// point means the field is genuinely unknown to this build.) Without this
	// check, a build with a feature compiled out (feature-gates.txt) would boot
	// a committed set-meta config minus its gated blocks: tacacs/radius
	// authentication silently degrading to local auth was the concrete
	// fail-open this closes (feature-gate-12 review).
	if ws := sp2.Warnings(); len(ws) > 0 {
		return nil, fmt.Errorf("config contains fields unknown to this build (a feature compiled out of this binary, or a legacy field this schema no longer defines); refusing to load a config that would silently drop them: %s", textbuf.Join(ws, "; "))
	}

	return tree, nil
}

// detectLegacySyntaxHint checks if a parse error is likely due to old ExaBGP syntax
// and returns a helpful hint for migration.
func detectLegacySyntaxHint(input string, parseErr error) string {
	errMsg := parseErr.Error()

	hasNeighborKeyword := strings.Contains(errMsg, "unknown top-level keyword: neighbor")
	hasTemplateNeighbor := strings.Contains(errMsg, "unknown field in template: neighbor")
	hasPeerGlobError := strings.Contains(errMsg, "invalid key for peer") && strings.Contains(errMsg, "invalid IP")

	lines := strings.SplitSeq(input, "\n")
	for line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "neighbor ") {
			hasNeighborKeyword = true
			break
		}
	}

	if hasNeighborKeyword || hasTemplateNeighbor || hasPeerGlobError {
		return "Hint: This config appears to use deprecated ExaBGP syntax.\n" +
			"Run 'ze config validate <file>' to verify, then\n" +
			"Run 'ze config migrate <file>' to upgrade."
	}

	return ""
}

// ExtractPluginsFromTree extracts plugin configurations from a parsed config tree.
// Extracts explicit plugins from plugin { internal <name> { ... } } and
// plugin { external <name> { ... } }, plus inline plugins from registered
// plugin extractors (e.g., BGP peer process bindings).
func ExtractPluginsFromTree(tree *Tree) ([]plugin.PluginConfig, error) {
	var plugins []plugin.PluginConfig
	seen := make(map[string]bool)

	if pluginContainer := tree.GetContainer("plugin"); pluginContainer != nil {
		for name, proc := range pluginContainer.GetList("internal") {
			if strings.HasPrefix(name, "_") {
				return nil, fmt.Errorf("plugin name %q: names starting with underscore are reserved", name)
			}
			if seen[name] {
				return nil, fmt.Errorf("plugin %q: duplicate plugin name", name)
			}
			seen[name] = true
			useVal, _ := proc.Get("use")
			if useVal == "" {
				return nil, fmt.Errorf("plugin %q: internal plugin requires use", name)
			}
			if runVal, _ := proc.Get("run"); runVal != "" {
				return nil, fmt.Errorf("plugin %q: internal plugins do not support run", name)
			}
			plugins = append(plugins, plugin.PluginConfig{
				Name:     name,
				Internal: true,
				Run:      useVal,
			})
		}

		for name, proc := range pluginContainer.GetList("external") {
			if strings.HasPrefix(name, "_") {
				return nil, fmt.Errorf("plugin name %q: names starting with underscore are reserved", name)
			}
			if seen[name] {
				return nil, fmt.Errorf("plugin %q: duplicate plugin name", name)
			}
			seen[name] = true
			pc := plugin.PluginConfig{Name: name}
			runVal, _ := proc.Get("run")
			useVal, _ := proc.Get("use")
			if runVal != "" && useVal != "" {
				return nil, fmt.Errorf("plugin %q: run and use are mutually exclusive", name)
			}
			if runVal != "" {
				pc.Run = runVal
			}
			if useVal != "" {
				pc.Internal = true
				pc.Run = useVal
			}
			if v, ok := proc.Get("encoder"); ok {
				pc.Encoder = v
			}
			if v, ok := proc.Get("timeout"); ok {
				d, err := time.ParseDuration(v)
				if err != nil {
					return nil, fmt.Errorf("plugin %q: invalid timeout %q: %w", name, v, err)
				}
				if d < 0 {
					return nil, fmt.Errorf("plugin %q: timeout must be positive, got %q", name, v)
				}
				pc.StageTimeout = d
			}
			if pc.Encoder == EncoderText {
				pc.ReceiveUpdate = true
			}
			if !pc.Internal {
				MarkInternalPlugin(&pc)
			}
			plugins = append(plugins, pc)
		}
	}

	// Inline plugins from registered extractors (e.g., BGP peer process bindings).
	pluginExtractorMu.RLock()
	extractors := pluginExtractors
	pluginExtractorMu.RUnlock()

	explicit := make(map[string]bool, len(plugins))
	for _, p := range plugins {
		explicit[p.Name] = true
	}

	for _, extract := range extractors {
		inline, err := extract(tree)
		if err != nil {
			return nil, err
		}
		for _, ip := range inline {
			if !explicit[ip.Name] {
				plugins = append(plugins, ip)
				explicit[ip.Name] = true
			}
		}
	}

	return plugins, nil
}

// PluginExtractorFunc extracts additional plugin configs from a parsed tree.
type PluginExtractorFunc func(tree *Tree) ([]plugin.PluginConfig, error)

var (
	pluginExtractorMu sync.RWMutex
	pluginExtractors  []PluginExtractorFunc
)

// RegisterPluginExtractor registers a function that extracts inline plugin configs
// from a parsed config tree. Called from init() in component packages.
func RegisterPluginExtractor(fn PluginExtractorFunc) {
	pluginExtractorMu.Lock()
	defer pluginExtractorMu.Unlock()
	pluginExtractors = append(pluginExtractors, fn)
}

// MergeCliPlugins resolves CLI plugin strings and merges them with extracted plugins.
func MergeCliPlugins(plugins []plugin.PluginConfig, cliPlugins []string) ([]plugin.PluginConfig, error) {
	if len(cliPlugins) == 0 {
		return plugins, nil
	}

	existing := make(map[string]bool)
	for _, p := range plugins {
		existing[p.Name] = true
	}

	var newPlugins []plugin.PluginConfig
	for _, ps := range cliPlugins {
		resolved, err := plugin.ResolvePlugin(ps)
		if err != nil {
			return nil, fmt.Errorf("plugin %q: %w", ps, err)
		}
		if resolved.Type == plugin.PluginTypeAuto {
			return nil, errPluginAutoNotYetImplemented
		}
		if existing[resolved.Name] {
			continue
		}
		existing[resolved.Name] = true

		pc := plugin.PluginConfig{
			Name:    resolved.Name,
			Encoder: "json",
		}
		if resolved.Type == plugin.PluginTypeInternal {
			pc.Internal = true
		} else {
			pc.Run = textbuf.Join(resolved.Command, " ")
		}
		newPlugins = append(newPlugins, pc)
	}

	return append(newPlugins, plugins...), nil
}

// registryName returns the name a plugin config is known by in the plugin
// registry, which is NOT always the operator's config label.
//
// `plugin { internal <label> { use <plugin> } }` names the instance <label>
// while running the registered plugin <plugin>. Dependency resolution must key
// on the latter: ResolveDependencies treats a name it cannot find in the
// registry as an external plugin and skips expanding it, so keying on the label
// silently dropped both Dependencies and OptionalDependencies. A route server
// configured as `internal rs { use bgp-rs }` therefore never pulled in
// bgp-adj-rib-in, and lost its peer-up replay with no error and no warning.
//
// Falls back to the label when Run names no registered plugin, which is the
// external case (Run is a command line there, not a registry name).
func registryName(p plugin.PluginConfig) string {
	if p.Run == "" {
		return p.Name
	}
	if candidate := strings.TrimPrefix(p.Run, "ze."); registry.Has(candidate) {
		return candidate
	}
	return p.Name
}

// ExpandDependencies resolves plugin dependencies from the registry and adds
// missing dependency plugins to the list.
func ExpandDependencies(plugins []plugin.PluginConfig) ([]plugin.PluginConfig, error) {
	names := make([]string, 0, len(plugins))
	existing := make(map[string]bool, len(plugins))
	for _, p := range plugins {
		// Keyed on the registered name, not the label. Only the resolved name is
		// marked as existing: marking the label too could suppress a genuine
		// dependency that happens to share the operator's chosen label.
		name := registryName(p)
		names = append(names, name)
		existing[name] = true
	}

	resolved, err := registry.ResolveDependencies(names)
	if err != nil {
		return nil, fmt.Errorf("expand dependencies: %w", err)
	}

	for _, name := range resolved {
		if existing[name] {
			continue
		}
		loaderLogger().Info("auto-adding dependency plugin", "name", name)
		plugins = append(plugins, plugin.PluginConfig{
			Name:     name,
			Internal: true,
			Encoder:  "json",
		})
		existing[name] = true
	}

	return plugins, nil
}

// MarkInternalPlugin sets Internal=true if Run resolves to an internal plugin.
func MarkInternalPlugin(pc *plugin.PluginConfig) {
	if pc.Run == "" {
		return
	}
	resolved, err := plugin.ResolvePlugin(pc.Run)
	if err != nil {
		return
	}
	if resolved.Type == plugin.PluginTypeInternal {
		pc.Internal = true
	}
}

// MigrateFunc applies config migrations to a parsed tree.
// Returns the list of applied migration names and any error.
type MigrateFunc func(tree *Tree) (applied []string, err error)

var (
	migrateMu   sync.RWMutex
	migrateFunc MigrateFunc
)

// RegisterMigrateFunc sets the config migration function. Called from
// config/migration's init() to break the import cycle between config and
// config/migration (migration imports config for Tree manipulation).
func RegisterMigrateFunc(fn MigrateFunc) {
	migrateMu.Lock()
	defer migrateMu.Unlock()
	migrateFunc = fn
}

func getMigrateFunc() MigrateFunc {
	migrateMu.RLock()
	defer migrateMu.RUnlock()
	return migrateFunc
}
