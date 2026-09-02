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

	// The schema is kept, not rebuilt: ApplyPasswordHashing below is
	// schema-driven (it hashes the plaintext- sibling of every ze:bcrypt leaf),
	// and YANGSchemaWithPlugins caches nothing -- it reloads and resolves every
	// embedded YANG module on each call. Parsing once and carrying the schema
	// costs one pointer; asking for it again would cost a second full build on
	// every daemon start and every SIGHUP.
	tree, schema, err := parseTreeWithYANG(input, pluginYANG)
	if err != nil {
		return nil, err
	}
	applyParsedEnvironment(tree)

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

	// Refuse a ze:bcrypt leaf holding the display placeholder, then hash the
	// plaintext- sibling of every such leaf. The two calls are ONE pair, and the
	// commit path makes both in this order (internal/component/cli/editor_commit.go,
	// internal/component/cli/editor_commands.go). Taking only the second half
	// leaves the placeholder in the tree, where CheckPassword
	// (internal/component/authz/auth.go) accepts the literal placeholder string as
	// the credential on a trusted-local transport.
	//
	// This is the same defect class as the validation call above, one leaf further
	// on: ApplyPasswordHashing had callers only on the commit path, so a config
	// FILE carrying `plaintext-password` loaded with an EMPTY canonical leaf,
	// CheckPassword refused every login for that user, and `ze config validate`
	// called the file valid (plan/journal/silent-fall-through.md, 2026-08-14).
	//
	// Both run AFTER the validators, so they judge the tree the operator wrote,
	// and BEFORE plugin extraction and everything downstream that reads a
	// credential.
	if err := RejectMaskedSecretLeaves(tree, schema); err != nil {
		return nil, err
	}

	hashed, err := ApplyPasswordHashing(tree, schema)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	warnPlaintextOnDisk(configPath, hashed)

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
	tree, _, err := parseTreeWithYANG(input, pluginYANG)
	if err != nil {
		return nil, err
	}

	applyParsedEnvironment(tree)

	return tree, nil
}

// applyParsedEnvironment pushes the tree's environment block into the log and
// env layers. Both callers of parseTreeWithYANG do it, and they must do the
// same thing: LoadConfig cannot go through ParseTreeWithYANG because it needs
// the schema that parse built.
func applyParsedEnvironment(tree *Tree) {
	envValues := ExtractEnvironment(tree)
	slogutil.ApplyLogConfig(envValues)
	ApplyEnvConfig(envValues)
}

// warnPlaintextOnDisk warns once that the config source still holds the
// plaintext the load just hashed. LoadConfig itself writes no file, so this
// warning is the only thing that tells the operator the secret stays readable
// where they wrote it.
//
// Two boot paths DO rewrite that file afterwards, and both replace the plaintext
// with the hash: applyEvolutions (cmd/ze/hub/main_evolve.go) re-serializes the
// loaded tree when a schema evolution applies, and RecoverConfig
// (internal/component/config/stamp.go) does the same on the rollback path.
// applyEvolutions also calls store.WriteVersion FIRST, so the archived version
// keeps the ORIGINAL plaintext.
//
// The source name is in the MESSAGE rather than an attribute because the log
// ring keeps only the message: LogEntry (internal/core/slogutil/ring.go) has
// fields for time, level, component and message and none for attributes, so a
// path carried as an attribute is invisible to `show log recent`. The leaf list
// stays an attribute deliberately, and is therefore visible in a structured sink
// and not in `show log recent`: the operator needs the FILE to act, and the leaf
// names only say which users to re-enter.
//
// A caller that names no source gets no invented name. Naming stdin for a
// caller that simply passed no path told the operator to look at the wrong
// place, and four production call sites pass none.
//
// The leaf paths are NOT redacted, and they need no redaction: hashed carries
// schema dot-paths, never values, because hashPlaintextSibling deletes the
// plaintext leaf before this runs. redact.Command would also do nothing here,
// because it blanks the token AFTER a credential key and a whole dot-path is
// one token.
func warnPlaintextOnDisk(configPath string, hashed []string) {
	if len(hashed) == 0 {
		return
	}
	var tb textbuf.Buffer
	var msg string
	switch configPath {
	case "", "-":
		msg = tb.Str("plaintext password in the loaded config").
			Str(": ze hashed it at load, and the source still holds the secret").String()
	default:
		msg = tb.Str("plaintext password in ").Str(configPath).
			Str(": ze hashed it at load, and the file still holds the secret").String()
	}
	loaderLogger().Warn(msg, "leaves", textbuf.Join(hashed, " "))
}

// parseTreeWithYANG parses config text and returns the tree together with the
// schema it was parsed against. The schema is returned because a caller that
// transforms the tree needs the same one (LoadConfig -> ApplyPasswordHashing),
// and rebuilding it means reloading and resolving every YANG module again.
func parseTreeWithYANG(input string, pluginYANG map[string]string) (*Tree, *Schema, error) {
	var schema *Schema
	var schemaErr error
	if len(pluginYANG) > 0 {
		schema, schemaErr = YANGSchemaWithPlugins(pluginYANG)
	} else {
		schema, schemaErr = YANGSchema()
	}
	if schemaErr != nil {
		return nil, nil, fmt.Errorf("YANG schema: %w", schemaErr)
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
				return nil, nil, fmt.Errorf("parse config: %w\n\n%s", err, hint)
			}
		}
	}
	if err != nil {
		return nil, nil, fmt.Errorf("parse config: %w", err)
	}

	// Prune inactive containers and list entries before extracting environment
	// values: `inactive:` is the operator's way to comment out a subtree, and
	// extracting it here would set env vars the operator explicitly disabled.
	// Env-level plumbing that depends on the full tree happens downstream in
	// CreateReactorFromTree (and runs after this prune).
	PruneInactive(tree, schema)

	return tree, schema, nil
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

// ExpandDependencies resolves plugin dependencies from the registry and adds
// missing dependency plugins to the list.
func ExpandDependencies(plugins []plugin.PluginConfig) ([]plugin.PluginConfig, error) {
	names := make([]string, 0, len(plugins))
	existing := make(map[string]bool, len(plugins))
	for _, p := range plugins {
		// Keyed on the registered name, not the operator's label, and
		// plugin.RegistryName is the one rule that answers which registry row a
		// process config names. ResolveDependencies treats a name it cannot find
		// in the registry as an external plugin and skips expanding it, so keying
		// on the label silently dropped both Dependencies and OptionalDependencies:
		// a route server configured as `internal rs { use bgp-rs }` never pulled in
		// bgp-adj-rib-in, and lost its peer-up replay with no error and no warning.
		//
		// Only the resolved name is marked as existing: marking the label too could
		// suppress a genuine dependency that happens to share the operator's chosen
		// label.
		name := plugin.RegistryName(p)
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
//
// It asks ResolvePlugin for the TRANSPORT, which is a different question from
// the one plugin.RegistryName answers: this decides whether the engine runs the
// plugin in a goroutine or forks it, where RegistryName decides which registry
// row the config names. `run ze plugin bgp-rib` names bgp-rib and still forks.
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
