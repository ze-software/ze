// Design: docs/architecture/config/syntax.md -- config file loading and plugin extraction
// Related: loader_extract.go -- environment service config extraction (web, mcp, lg, hub)

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var errPluginAutoNotYetImplemented = errors.New("plugin 'auto' not yet implemented")

// LoadConfigResult holds the output of LoadConfig: a parsed config tree,
// resolved plugin list, startup schema selectors, and derived config directory.
type LoadConfigResult struct {
	Tree       *Tree
	Plugins    []plugin.PluginConfig
	ConfigDir  string
	CLIPlugins []string
}

// LoadConfig parses config with CLI plugin YANG schemas, extracts and resolves
// the plugin list, and returns the parsed tree + plugins without creating a reactor.
//
// It is the loader of the config this process RUNS: once the tree passes the
// validators it publishes the file's `environment { }` block into this
// process's log and env layers. A caller that only reads a config (a show
// command, a doctor or support report, a candidate's validation) calls
// ReadConfig instead, or a read changes the running process's settings.
func LoadConfig(input, configPath string, cliPlugins []string) (*LoadConfigResult, error) {
	return loadConfig(input, configPath, cliPlugins, applyParsedEnvironment)
}

// ReadConfig answers what LoadConfig answers for the same input, through the
// same validators, hashing and plugin resolution, and applies nothing to the
// running process: the file's `environment { }` block stays in the returned
// tree and never reaches this process's log or env layers.
func ReadConfig(input, configPath string, cliPlugins []string) (*LoadConfigResult, error) {
	return loadConfig(input, configPath, cliPlugins, nil)
}

// LoadRunningConfig reconstructs an already validated canonical running map and
// resolves its plugins through the ordinary loader finalizer. It MUST NOT parse
// file bytes, publish environment settings or hash canonical credentials again.
func LoadRunningConfig(running map[string]any, configPath string, cliPlugins []string) (*LoadConfigResult, error) {
	if running == nil {
		return nil, errors.New("running configuration authority is missing")
	}
	tree, err := TreeFromPluginMap(running)
	if err != nil {
		return nil, fmt.Errorf("reconstruct running configuration: %w", err)
	}
	return finalizeConfigLoad(tree, configPath, cliPlugins)
}

// loadConfig is the one body of LoadConfig and ReadConfig. publish is called
// with the validated tree at the point where refusal by the validators is no
// longer possible; nil publishes nothing, which is ReadConfig's whole
// difference.
func loadConfig(input, configPath string, cliPlugins []string, publish func(*Tree)) (*LoadConfigResult, error) {
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

	// Apply the registered ze:validate custom validators. Until this call
	// existed, ValidateTreeAllModules had exactly one non-test caller
	// (`ze config validate`), so every custom validator was bypassed at daemon
	// start and at SIGHUP reload and a hand-edited value reached the wire
	// unvalidated (spec-fixit-config-validators-bypassed-at-startup).
	//
	// Refusing here is also what makes the reload refuse, with no machinery of
	// its own: runReload (cmd/ze/hub/main_reload.go) turns this error into
	// "reload: ", clears the staged candidate and returns before
	// ReloadConfig, the provider refresh and engine.Reload run, so the daemon
	// keeps serving the config it already has (Thomas, 2026-08-11).
	if err := refuseInvalidCustomSections(tree); err != nil {
		return nil, err
	}
	// A config that needs a plugin this build lacks is refused here, before any
	// plugin starts, by the same check the validate and commit doors run
	// (VerifyPluginConfigMapTransition). The tree is lowered only when some
	// registration declares a config need.
	if err := refuseAbsentOptionalProviders(tree.ToPluginMap); err != nil {
		return nil, err
	}
	// A redistribute rule that cannot act is refused here, before any plugin
	// starts, by the same check the validate and commit doors run.
	if err := RefuseRedistributeRules(tree); err != nil {
		return nil, err
	}
	// Validators use supplied values and registries, not candidate environment.
	// Publish only after refusal is impossible at this validation boundary.
	// refuse-by-default: guards ReadConfig -- a nil publish is the read that applies nothing
	if publish != nil {
		publish(tree)
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
	warnWeakPassword(hashed)

	return finalizeConfigLoad(tree, configPath, cliPlugins)
}

func finalizeConfigLoad(tree *Tree, configPath string, cliPlugins []string) (*LoadConfigResult, error) {
	plugins, err := ExtractPluginsFromTree(tree)
	if err != nil {
		return nil, err
	}

	plugins, err = MergeCliPlugins(plugins, cliPlugins)
	if err != nil {
		return nil, fmt.Errorf("resolve plugins: %w", err)
	}

	// The tree decides which optional providers are pulled: a provider that a
	// ConfigNeed names loads only when this config holds that need.
	plugins, err = ExpandDependencies(plugins, ConfigNeedHolds(tree.ToMap()))
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
		Tree:       tree,
		Plugins:    plugins,
		ConfigDir:  configDir,
		CLIPlugins: slices.Clone(cliPlugins),
	}, nil
}

// ParseTreeWithYANG parses config with optional plugin YANG schemas.
// Returns the parsed tree for further processing by callers. It publishes the
// file's `environment { }` block into this process's log and env layers, as
// LoadConfig does, so a caller that only reads a config calls
// ParseTreeForValidation (or ReadConfig) instead.
func ParseTreeWithYANG(input string, pluginYANG map[string]string) (*Tree, error) {
	tree, _, err := parseTreeWithYANG(input, pluginYANG)
	if err != nil {
		return nil, err
	}

	applyParsedEnvironment(tree)

	return tree, nil
}

// applyParsedEnvironment pushes the tree's environment block into the log and
// env layers. LoadConfig and ParseTreeWithYANG do it, and they must do the
// same thing: LoadConfig cannot go through ParseTreeWithYANG because it needs
// the schema that parse built. ReadConfig is the parse that skips it.
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
// One boot path DOES rewrite that file afterwards and replaces the plaintext
// with the hash: RecoverConfig (internal/component/config/stamp.go)
// re-serializes the loaded tree on the rollback path.
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
func warnPlaintextOnDisk(configPath string, hashed []HashedPassword) {
	if len(hashed) == 0 {
		return
	}
	leaves := make([]string, 0, len(hashed))
	for _, h := range hashed {
		leaves = append(leaves, h.Path)
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
	loaderLogger().Warn(msg, "leaves", textbuf.Join(leaves, " "))
}

// warnWeakPassword warns once for each weak password the load just hashed. The
// load path sets the password whatever the policy says, so this warning is the
// only trace a weak credential leaves: a config file that ze accepted keeps
// working, and the operator learns which account to re-enter.
//
// The reason names the rule and never the password (password_strength.go), so
// the line is safe in a log ring an operator can read.
func warnWeakPassword(hashed []HashedPassword) {
	for _, warning := range PasswordWeaknessWarnings(hashed) {
		loaderLogger().Warn(warning)
	}
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
		tree, err = parseSetConfig(schema, input, true)
	case FormatSet:
		tree, err = parseSetConfig(schema, input, false)
	case FormatHierarchical:
		p := NewParser(schema)
		tree, err = p.Parse(input)
		if err != nil {
			if hint := detectLegacySyntaxHint(input, err); hint != "" {
				return nil, nil, &parseConfigError{cause: err, hint: hint}
			}
		}
	default:
		panic("BUG: invalid detected config format")
	}
	if err != nil {
		return nil, nil, &parseConfigError{cause: err}
	}

	// Prune inactive containers and list entries before extracting environment
	// values: `inactive:` is the operator's way to comment out a subtree, and
	// extracting it here would set env vars the operator explicitly disabled.
	// Env-level plumbing that depends on the full tree happens downstream in
	// CreateReactorFromTree (and runs after this prune).
	PruneInactive(tree, schema)

	return tree, schema, nil
}

// parseSetConfig parses set-format config and fails closed on every field the
// schema does not know.
//
// The strict parse answers first. When it refuses, a second, lenient parse
// collects every unknown field rather than stopping at the first, so the
// refusal names them all and says why a field the operator wrote can be
// unknown. A build with a feature compiled out (feature-gates.txt) would
// otherwise boot a committed set-meta config minus its gated blocks:
// tacacs/radius authentication silently degrading to local auth was the
// concrete fail-open this closes (feature-gate-12 review). The lenient tree is
// never returned: the strict parse already refused the config.
func parseSetConfig(schema *Schema, input string, hasMeta bool) (*Tree, error) {
	sp := NewSetParser(schema)
	var strictErr error
	if hasMeta {
		tree, _, err := sp.ParseWithMeta(input)
		if err == nil {
			return tree, nil
		}
		strictErr = err
	} else {
		tree, err := sp.Parse(input)
		if err == nil {
			return tree, nil
		}
		strictErr = err
	}

	lenient := NewSetParser(schema)
	lenient.SetPreMigration(true)
	var err error
	if hasMeta {
		_, _, err = lenient.ParseWithMeta(input)
	} else {
		_, err = lenient.Parse(input)
	}
	if err != nil {
		return nil, err
	}

	if fields := lenient.UnknownFields(); len(fields) > 0 {
		// The strict refusal names the first field with its hint, and the
		// other fields follow, each line named once (unknownFieldsError).
		return nil, opererr.Wrap(newUnknownFieldsError(strictErr, fields),
			"config contains fields unknown to this build",
			"a feature compiled out of this binary owns them, or this schema no longer defines them, and loading would silently drop them",
			"remove or correct each field named above, or rebuild ze with the build tag a field names")
	}

	return nil, strictErr
}

// detectLegacySyntaxHint checks if a parse error is likely due to ExaBGP syntax
// and returns a hint naming the ExaBGP migration, which converts it.
func detectLegacySyntaxHint(input string, parseErr error) string {
	errMsg := parseErr.Error()

	hasNeighborKeyword := strings.Contains(errMsg, "unknown top-level keyword: neighbor")
	hasTemplateNeighbor := strings.Contains(errMsg, "unknown field in template: neighbor")

	lines := strings.SplitSeq(input, "\n")
	for line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "neighbor ") {
			hasNeighborKeyword = true
			break
		}
	}

	if hasNeighborKeyword || hasTemplateNeighbor {
		return "Hint: This config appears to use ExaBGP syntax.\n" +
			"Run 'ze exabgp migrate <file>' to convert it to Ze's syntax."
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
			if v, ok := proc.Get("respawn"); ok {
				// A value the daemon cannot read is refused rather than taken as
				// false: the operator wrote the leaf to say something, and
				// starting as though they had not is the silent wrong answer.
				//
				// An absent leaf is left as RespawnUnstated, which is not the
				// same as false: silence leaves the decision to the plugin's own
				// declaration, and false takes a restart away from a plugin
				// willing to have one (plugin.RespawnRequest).
				respawn, err := ParseBoolStrict(v)
				if err != nil {
					return nil, fmt.Errorf("plugin %q: invalid respawn %q: %w", name, v, err)
				}
				pc.Respawn = plugin.RespawnDeclined
				if respawn {
					pc.Respawn = plugin.RespawnAsked
				}
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

	for _, extractor := range extractors {
		inline, err := extractor.extract(tree)
		if err != nil {
			return nil, &PluginExtractionError{Section: extractor.section, Err: err}
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

// PluginExtractionError is the refusal ExtractPluginsFromTree answers when a
// registered extractor fails. Section names the config section that extractor
// reads, so a door whose own check of that section already reports the
// failure (the editor's bgp pass, `ze config validate`'s bgp resolve) can tell
// the relayed copy from a failure nobody else reports.
type PluginExtractionError struct {
	Section string
	Err     error
}

func (e *PluginExtractionError) Error() string { return e.Err.Error() }

func (e *PluginExtractionError) Unwrap() error { return e.Err }

// sectionExtractor is one registered extractor and the section it reads.
type sectionExtractor struct {
	section string
	extract PluginExtractorFunc
}

var (
	pluginExtractorMu sync.RWMutex
	pluginExtractors  []sectionExtractor
)

// RegisterPluginExtractor registers a function that extracts inline plugin configs
// from the named section of a parsed config tree. Called from init() in
// component packages. The section is carried on the extractor's failure
// (PluginExtractionError) so the section's own check can own the report.
func RegisterPluginExtractor(section string, fn PluginExtractorFunc) {
	pluginExtractorMu.Lock()
	defer pluginExtractorMu.Unlock()
	pluginExtractors = append(pluginExtractors, sectionExtractor{section: section, extract: fn})
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
// missing dependency plugins to the list. needHolds answers the config needs
// for the config the plugins come from (ConfigNeedHolds over its tree); it
// MUST NOT be nil, and registry.ResolveDependencies refuses a nil one.
func ExpandDependencies(plugins []plugin.PluginConfig, needHolds registry.NeedHolds) ([]plugin.PluginConfig, error) {
	names, external := dependencyRequest(plugins)
	existing := make(map[string]bool, len(names))
	for _, name := range names {
		existing[name] = true
	}

	resolved, err := registry.ResolveDependencies(names, external, needHolds)
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

// dependencyRequest answers what registry.ResolveDependencies is asked for the
// plugins a config declares in its `plugin { }` block: each one's registered
// name, and the names an `external` block runs as another program. Boot
// (ExpandDependencies) and the validate and commit doors
// (refuseUnresolvableDependencies) both ask through it, so they resolve the
// same set.
func dependencyRequest(plugins []plugin.PluginConfig) (names []string, external map[string]bool) {
	names = make([]string, 0, len(plugins))
	external = make(map[string]bool, len(plugins))
	for _, p := range plugins {
		// Keyed on the registered name, not the operator's label, and
		// plugin.RegistryName is the one rule that answers which registry row a
		// process config names. ResolveDependencies treats a name it cannot find
		// in the registry as an external plugin and skips expanding it, so keying
		// on the label silently dropped both Dependencies and OptionalDependencies:
		// a route server configured as `internal rs { use bgp-rs }` never pulled in
		// bgp-adj-rib-in, and lost its peer-up replay with no error and no warning.
		//
		// Only the resolved name is requested: requesting the label too could
		// suppress a genuine dependency that happens to share the operator's chosen
		// label.
		name := plugin.RegistryName(p)
		names = append(names, name)
		// An `external` block names a PROGRAM, and that program declares its own
		// dependencies over the protocol. Where this binary also carries a plugin
		// of that name, the registration is NOT this block's: expanding it starts
		// plugins the program never asked for, and the operator sees a plugin they
		// did not configure fail on a platform they never named.
		if p.RunsExternalProgram() {
			external[name] = true
		}
	}

	return names, external
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
