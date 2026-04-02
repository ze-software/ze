// Design: docs/architecture/config/syntax.md — config file loading and reactor creation
// Detail: loader_routes.go — BGP route type conversion
// Detail: loader_prefix.go — prefix expansion for route splitting
// Detail: loader_create.go — reactor creation from config tree
// Detail: plugins.go — plugin extraction from config tree

package bgpconfig

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/authz"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/reactor"
	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	zessh "codeberg.org/thomas-mangin/ze/internal/component/ssh"
	"codeberg.org/thomas-mangin/ze/internal/core/paths"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

// configLogger is the config subsystem logger (lazy initialization).
// Controlled by ze.log.bgp.config environment variable.
// Uses LazyLogger to pick up config file settings applied after init().
var configLogger = slogutil.LazyLogger("bgp.config")

const loopbackIP = "127.0.0.1"

// Origin attribute values.
const (
	originIGP = "igp"
	originEGP = "egp"
)

// parseTreeWithYANG parses config with optional plugin YANG schemas.
// Returns the parsed tree for further processing by callers.
func parseTreeWithYANG(input string, pluginYANG map[string]string) (*config.Tree, error) {
	// Parse input using YANG-derived schema with plugin augmentations
	var schema *config.Schema
	var schemaErr error
	if len(pluginYANG) > 0 {
		schema, schemaErr = config.YANGSchemaWithPlugins(pluginYANG)
	} else {
		schema, schemaErr = config.YANGSchema()
	}
	if schemaErr != nil {
		return nil, fmt.Errorf("YANG schema: %w", schemaErr)
	}
	p := config.NewParser(schema)
	tree, err := p.Parse(input)
	if err != nil {
		// Check if this looks like old syntax and provide migration hint
		if hint := detectLegacySyntaxHint(input, err); hint != "" {
			return nil, fmt.Errorf("parse config: %w\n\n%s", err, hint)
		}
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Log parse warnings
	if warnings := p.Warnings(); len(warnings) > 0 {
		configLogger().Debug("config parsed", "warnings", warnings)
	}

	// Extract environment block and apply log config early.
	// Lazy loggers (LazyLogger) will pick up these settings on first use.
	envValues := config.ExtractEnvironment(tree)
	slogutil.ApplyLogConfig(envValues)

	return tree, nil
}

// LoadReactor parses config and creates a configured Reactor.
func LoadReactor(input string) (*reactor.Reactor, error) {
	tree, err := parseTreeWithYANG(input, nil)
	if err != nil {
		return nil, err
	}
	plugins, err := ExtractPluginsFromTree(tree)
	if err != nil {
		return nil, err
	}
	plugins, err = expandDependencies(plugins)
	if err != nil {
		return nil, err
	}
	return CreateReactorFromTree(tree, "", "", plugins, nil)
}

// LoadConfigResult holds the output of LoadConfig: a parsed config tree,
// resolved plugin list, and derived config directory.
type LoadConfigResult struct {
	Tree      *config.Tree
	Plugins   []reactor.PluginConfig
	ConfigDir string
}

// LoadConfig parses config with CLI plugin YANG schemas, extracts and resolves
// the plugin list, and returns the parsed tree + plugins without creating a reactor.
// This is the first half of the decomposed LoadReactorWithPlugins.
func LoadConfig(input, configPath string, cliPlugins []string) (*LoadConfigResult, error) {
	// Internal plugin schemas loaded via init()-based registration (LoadRegistered).
	// Only CLI-specified external plugins need explicit loading.
	pluginYANG := plugin.CollectPluginYANG(cliPlugins)

	tree, err := parseTreeWithYANG(input, pluginYANG)
	if err != nil {
		return nil, err
	}

	plugins, err := ExtractPluginsFromTree(tree)
	if err != nil {
		return nil, err
	}

	// Merge CLI plugins with config plugins
	plugins, err = mergeCliPlugins(plugins, cliPlugins)
	if err != nil {
		return nil, fmt.Errorf("resolve plugins: %w", err)
	}

	// Expand dependencies before creating reactor
	plugins, err = expandDependencies(plugins)
	if err != nil {
		return nil, err
	}

	// Set config directory for process execution
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

// CreateReactor creates a Reactor from a LoadConfigResult.
// This is the second half of the decomposed LoadReactorWithPlugins.
func CreateReactor(cfg *LoadConfigResult, configPath string, store storage.Storage) (*reactor.Reactor, error) {
	r, err := CreateReactorFromTree(cfg.Tree, cfg.ConfigDir, configPath, cfg.Plugins, store)
	if err != nil {
		return nil, err
	}

	// Set config path for SIGHUP reload support
	if configPath != "" && configPath != "-" {
		r.SetConfigPath(configPath)
		r.SetReloadFunc(createReloadFunc(store))
	}

	return r, nil
}

// LoadReactorWithPlugins parses config with CLI plugins and creates Reactor.
// configPath is the original file path (used for SIGHUP reload). May be empty or "-".
// store is used by the reload function to re-read config on SIGHUP; may be nil when
// configPath is "" or "-" (reload not supported).
// This is a convenience wrapper around LoadConfig + CreateReactor.
func LoadReactorWithPlugins(store storage.Storage, input, configPath string, cliPlugins []string) (*reactor.Reactor, error) {
	cfg, err := LoadConfig(input, configPath, cliPlugins)
	if err != nil {
		return nil, err
	}
	return CreateReactor(cfg, configPath, store)
}

// LoadReactorFile loads config from file and creates Reactor.
func LoadReactorFile(store storage.Storage, path string) (*reactor.Reactor, error) {
	return LoadReactorFileWithPlugins(store, path, nil)
}

// LoadReactorFileWithPlugins loads config from file and creates Reactor,
// merging CLI-specified plugins with config-declared plugins.
//
// CLI plugins are resolved using plugin.ResolvePlugin():
//   - "ze.X" -> internal plugin (run "ze plugin X")
//   - "./path" -> fork local binary
//   - "/path" -> fork absolute path binary
//   - "cmd args..." -> fork command with args
//   - "auto" -> auto-discover all plugins (not implemented yet)
//
// Plugin YANG schemas are loaded before config parsing to allow plugins
// to augment the config schema (e.g., hostname plugin adds host-name/domain-name).
func LoadReactorFileWithPlugins(store storage.Storage, path string, cliPlugins []string) (*reactor.Reactor, error) {
	var data []byte
	var err error

	// Support stdin when path is "-"
	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = store.ReadFile(path)
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	// Internal plugin schemas loaded via init()-based registration (LoadRegistered).
	// Only CLI-specified external plugins need explicit loading.
	pluginYANG := plugin.CollectPluginYANG(cliPlugins)

	// Parse config into tree
	tree, err := parseTreeWithYANG(string(data), pluginYANG)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Determine config directory
	var configDir string
	var absPath string
	if path == "-" {
		absPath, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
		configDir = absPath
	} else {
		absPath, err = filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("resolve config path: %w", err)
		}
		configDir = filepath.Dir(absPath)
	}

	// Extract plugins from tree
	plugins, err := ExtractPluginsFromTree(tree)
	if err != nil {
		return nil, fmt.Errorf("extract plugins: %w", err)
	}

	// Merge CLI plugins with config plugins
	plugins, err = mergeCliPlugins(plugins, cliPlugins)
	if err != nil {
		return nil, fmt.Errorf("resolve plugins: %w", err)
	}

	// Expand dependencies before creating reactor
	plugins, err = expandDependencies(plugins)
	if err != nil {
		return nil, fmt.Errorf("expand plugin dependencies: %w", err)
	}

	// Wire YANG validator for runtime attribute validation (origin enum, med/local-pref ranges)
	if v, vErr := config.YANGValidatorWithPlugins(pluginYANG); vErr == nil && v != nil {
		plugin.SetYANGValidator(v)
	}

	// Create reactor from tree
	r, err := CreateReactorFromTree(tree, configDir, absPath, plugins, store)
	if err != nil {
		return nil, err
	}

	// Set config path for SIGHUP reload support
	if path != "-" {
		r.SetConfigPath(absPath)
		r.SetReloadFunc(createReloadFunc(store))
	}

	return r, nil
}

// mergeCliPlugins resolves CLI plugin strings and merges them with extracted plugins.
// CLI plugins are added first (higher priority), then config plugins.
// Duplicate plugins (same name) are deduplicated.
func mergeCliPlugins(plugins []reactor.PluginConfig, cliPlugins []string) ([]reactor.PluginConfig, error) {
	if len(cliPlugins) == 0 {
		return plugins, nil
	}

	// Build set of existing plugin names for deduplication
	existing := make(map[string]bool)
	for _, p := range plugins {
		existing[p.Name] = true
	}

	// Resolve and prepend CLI plugins
	var newPlugins []reactor.PluginConfig
	for _, ps := range cliPlugins {
		resolved, err := plugin.ResolvePlugin(ps)
		if err != nil {
			return nil, fmt.Errorf("plugin %q: %w", ps, err)
		}

		// Skip auto for now (would need discovery)
		if resolved.Type == plugin.PluginTypeAuto {
			return nil, fmt.Errorf("plugin 'auto' not yet implemented")
		}

		// Skip if already in config
		if existing[resolved.Name] {
			continue
		}
		existing[resolved.Name] = true

		// Build plugin config based on type
		pc := reactor.PluginConfig{
			Name:    resolved.Name,
			Encoder: "json", // Default encoder
		}

		if resolved.Type == plugin.PluginTypeInternal {
			// Internal plugins run in-process via goroutine
			pc.Internal = true
			// Run is empty - process.go will use internal registry
		} else {
			// External plugins fork via exec
			pc.Run = strings.Join(resolved.Command, " ")
		}

		newPlugins = append(newPlugins, pc)
	}

	// Prepend CLI plugins to config plugins (CLI takes priority)
	return append(newPlugins, plugins...), nil
}

// expandDependencies resolves plugin dependencies from the registry and adds
// missing dependency plugins to the list. Auto-added plugins are Internal=true
// with Encoder="json" since they are Go plugins registered via init().
func expandDependencies(plugins []reactor.PluginConfig) ([]reactor.PluginConfig, error) {
	// Collect current plugin names.
	names := make([]string, 0, len(plugins))
	existing := make(map[string]bool, len(plugins))
	for _, p := range plugins {
		names = append(names, p.Name)
		existing[p.Name] = true
	}

	// Resolve transitive dependencies via registry.
	resolved, err := registry.ResolveDependencies(names)
	if err != nil {
		return nil, fmt.Errorf("expand dependencies: %w", err)
	}

	// Add any new names not already in the plugin list.
	for _, name := range resolved {
		if existing[name] {
			continue
		}
		configLogger().Info("auto-adding dependency plugin", "name", name)
		plugins = append(plugins, reactor.PluginConfig{
			Name:     name,
			Internal: true,
			Encoder:  "json",
		})
		existing[name] = true
	}

	return plugins, nil
}

// detectLegacySyntaxHint checks if a parse error is likely due to old syntax
// and returns a helpful hint for migration.
func detectLegacySyntaxHint(input string, parseErr error) string {
	errMsg := parseErr.Error()

	// Check for common old syntax patterns
	hasNeighborKeyword := strings.Contains(errMsg, "unknown top-level keyword: neighbor")
	hasTemplateNeighbor := strings.Contains(errMsg, "unknown field in template: neighbor")
	hasPeerGlobError := strings.Contains(errMsg, "invalid key for peer") && strings.Contains(errMsg, "invalid IP")

	// Also check input for old syntax patterns
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

// ValidateAuthzConfig validates authorization config in the parsed tree.
// Checks: profile entry regex syntax (hard error), user→profile references (AC-8).
// Exported so ze config validate can also call it.
func ValidateAuthzConfig(tree *config.Tree) error {
	sys := tree.GetContainer("system")
	if sys == nil {
		return nil
	}

	authzContainer := sys.GetContainer("authorization")
	if authzContainer == nil {
		return nil
	}

	profiles := authzContainer.GetList("profile")

	// Validate each profile's entries (regex syntax, empty match).
	for name, profileTree := range profiles {
		p := authz.Profile{Name: name}
		if runContainer := profileTree.GetContainer("run"); runContainer != nil {
			p.Run = extractAuthzSection(runContainer)
		}
		if editContainer := profileTree.GetContainer("edit"); editContainer != nil {
			p.Edit = extractAuthzSection(editContainer)
		}
		if err := p.Validate(); err != nil {
			return fmt.Errorf("authorization profile: %w", err)
		}
	}

	// Check user→profile references (AC-8).
	auth := sys.GetContainer("authentication")
	if auth == nil {
		return nil
	}

	for username, userTree := range auth.GetList("user") {
		for _, pn := range userTree.GetSlice("profile") {
			if _, ok := profiles[pn]; !ok {
				return fmt.Errorf("user %q references undefined profile %q", username, pn)
			}
		}
	}

	return nil
}

// extractAuthzConfig extracts authorization profiles from the parsed config tree.
// Returns a populated Store if system.authorization is present with profiles, nil otherwise.
// User-to-profile assignments come from system.authentication.user[*].profile (leaf-list).
func extractAuthzConfig(tree *config.Tree) *authz.Store {
	sys := tree.GetContainer("system")
	if sys == nil {
		return nil
	}

	authzContainer := sys.GetContainer("authorization")
	if authzContainer == nil {
		return nil
	}

	profiles := authzContainer.GetList("profile")
	if len(profiles) == 0 {
		return nil
	}

	store := authz.NewStore()

	for name, profileTree := range profiles {
		p := authz.Profile{Name: name}

		if runContainer := profileTree.GetContainer("run"); runContainer != nil {
			p.Run = extractAuthzSection(runContainer)
		}

		if editContainer := profileTree.GetContainer("edit"); editContainer != nil {
			p.Edit = extractAuthzSection(editContainer)
		}

		// ValidateAuthzConfig already rejected invalid profiles (regex, empty match).
		store.AddProfile(p)
	}

	// Extract user → profile assignments from authentication block
	if auth := sys.GetContainer("authentication"); auth != nil {
		for username, userTree := range auth.GetList("user") {
			profileNames := userTree.GetSlice("profile")
			if len(profileNames) > 0 {
				store.AssignProfiles(username, profileNames)
			}
		}
	}

	// Warn about match entries that don't match any known builtin command (AC-9).
	// Warning only — plugins may register commands dynamically at runtime.
	validateMatchEntries(store)

	if !store.HasProfiles() {
		return nil
	}

	return store
}

// validateMatchEntries warns about profile match entries that don't match
// any known builtin command prefix. This is a best-effort check because
// plugins register commands dynamically at runtime.
func validateMatchEntries(store *authz.Store) {
	loader, _ := yang.DefaultLoader()
	wireToPath := yang.WireMethodToPath(loader)

	cmds := make([]string, 0, len(wireToPath))
	for _, path := range wireToPath {
		cmds = append(cmds, strings.ToLower(path))
	}

	store.WalkEntries(func(profileName, section string, e authz.Entry) {
		if e.Regex {
			return // regex entries can't be prefix-checked
		}
		match := strings.ToLower(e.Match)
		for _, cmd := range cmds {
			if strings.HasPrefix(cmd, match) || strings.HasPrefix(match, cmd) {
				return // match is a prefix of (or matches) a known command
			}
		}
		configLogger().Warn("authz match entry does not match any known command",
			"profile", profileName, "section", section, "match", e.Match)
	})
}

// extractAuthzSection extracts a run or edit authorization section from the config tree.
func extractAuthzSection(container *config.Tree) authz.Section {
	var s authz.Section

	if v, ok := container.Get("default-action"); ok {
		if v == "allow" {
			s.Default = authz.Allow
		}
	}

	for numStr, entryTree := range container.GetList("entry") {
		num, err := strconv.ParseUint(numStr, 10, 32)
		if err != nil {
			continue
		}

		e := authz.Entry{Number: uint32(num)}

		if v, ok := entryTree.Get("action"); ok {
			if v == "allow" {
				e.Action = authz.Allow
			}
		}

		if v, ok := entryTree.Get("match"); ok {
			e.Match = v
		}

		if v, ok := entryTree.Get("regex"); ok {
			e.Regex = v == "true"
		}

		s.Entries = append(s.Entries, e)
	}

	// Sort entries by number (ascending) for deterministic evaluation order
	sort.Slice(s.Entries, func(i, j int) bool {
		return s.Entries[i].Number < s.Entries[j].Number
	})

	return s
}

// extractSSHConfig extracts SSH server configuration from the parsed config tree.
// Returns the SSH config and true if a system.ssh block is present.
func extractSSHConfig(tree *config.Tree) (zessh.Config, bool) {
	// SSH server settings live under environment.ssh.
	env := tree.GetContainer("environment")
	if env == nil {
		return zessh.Config{}, false
	}

	sshContainer := env.GetContainer("ssh")
	if sshContainer == nil {
		return zessh.Config{}, false
	}

	// ConfigDir intentionally left empty -- host key resolves from binary
	// location via paths.DefaultConfigDir() (e.g., ./bin/ze -> etc/ze/).
	var cfg zessh.Config

	// Read listen addresses from server list entries (YANG: list server { ip; port; }).
	if servers := sshContainer.GetListOrdered("server"); len(servers) > 0 {
		for _, s := range servers {
			ip := "0.0.0.0"
			port := "2222"
			if v, ok := s.Value.Get("ip"); ok {
				ip = v
			}
			if v, ok := s.Value.Get("port"); ok {
				port = v
			}
			cfg.ListenAddrs = append(cfg.ListenAddrs, ip+":"+port)
		}
		cfg.Listen = cfg.ListenAddrs[0]
	} else if addrs := sshContainer.GetSlice("listen"); len(addrs) > 0 {
		// Fallback: compound listen format from env var override.
		cfg.Listen = addrs[0]
		cfg.ListenAddrs = addrs
	}
	if v, ok := sshContainer.Get("host-key"); ok {
		cfg.HostKeyPath = v
	}
	if v, ok := sshContainer.Get("idle-timeout"); ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			cfg.IdleTimeout = uint32(n)
		}
	}
	if v, ok := sshContainer.Get("max-sessions"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxSessions = n
		}
	}

	// Users stay under system.authentication.
	if sys := tree.GetContainer("system"); sys != nil {
		if auth := sys.GetContainer("authentication"); auth != nil {
			for name, entry := range auth.GetList("user") {
				var uc zessh.UserConfig
				uc.Name = name
				if pw, ok := entry.Get("password"); ok {
					uc.Hash = pw
				}
				cfg.Users = append(cfg.Users, uc)
			}
		}
	}

	return cfg, true
}

// WebConfig holds parsed environment.web settings.
type WebConfig struct {
	Host     string // Listen host (e.g. 0.0.0.0)
	Port     string // Listen port (e.g. 3443)
	Insecure bool   // Disable authentication
}

// Listen returns host:port.
func (c WebConfig) Listen() string { return c.Host + ":" + c.Port }

// ExtractWebConfig returns the environment.web config if enabled.
// With the named list pattern, reads the first server entry or uses defaults.
func ExtractWebConfig(tree *config.Tree) (WebConfig, bool) {
	envBlock := tree.GetContainer("environment")
	if envBlock == nil {
		return WebConfig{}, false
	}
	web := envBlock.GetContainer("web")
	if web == nil {
		return WebConfig{}, false
	}

	// Service must be explicitly enabled (default false).
	enabled, _ := web.Get("enabled")
	if enabled != configTrue {
		return WebConfig{}, false
	}

	cfg := WebConfig{Host: "0.0.0.0", Port: "3443"}

	// Read first server list entry if present; otherwise use YANG defaults.
	if servers := web.GetListOrdered("server"); len(servers) > 0 {
		entry := servers[0].Value
		if v, ok := entry.Get("ip"); ok {
			cfg.Host = v
		}
		if v, ok := entry.Get("port"); ok {
			cfg.Port = v
		}
	}

	if v, ok := web.Get("insecure"); ok && v == configTrue {
		cfg.Insecure = true
	}

	// Validate: insecure requires 127.0.0.1 binding.
	if cfg.Insecure && cfg.Host != loopbackIP {
		configLogger().Error("environment.web: insecure forces host to 127.0.0.1", "host", cfg.Host)
		cfg.Host = loopbackIP
	}

	return cfg, true
}

// HasWebConfig returns true if the parsed config tree has an enabled environment.web block.
func HasWebConfig(tree *config.Tree) bool {
	_, ok := ExtractWebConfig(tree)
	return ok
}

// MCPConfig holds parsed environment.mcp settings.
type MCPConfig struct {
	Host string // Listen host (127.0.0.1 enforced)
	Port string // Listen port
}

// Listen returns host:port.
func (c MCPConfig) Listen() string { return c.Host + ":" + c.Port }

// ExtractMCPConfig returns the environment.mcp config if enabled.
func ExtractMCPConfig(tree *config.Tree) (MCPConfig, bool) {
	envBlock := tree.GetContainer("environment")
	if envBlock == nil {
		return MCPConfig{}, false
	}
	mcp := envBlock.GetContainer("mcp")
	if mcp == nil {
		return MCPConfig{}, false
	}

	// Service must be explicitly enabled (default false).
	enabled, _ := mcp.Get("enabled")
	if enabled != configTrue {
		return MCPConfig{}, false
	}

	cfg := MCPConfig{Host: loopbackIP}

	if servers := mcp.GetListOrdered("server"); len(servers) > 0 {
		entry := servers[0].Value
		if v, ok := entry.Get("ip"); ok {
			cfg.Host = v
		}
		if v, ok := entry.Get("port"); ok {
			cfg.Port = v
		}
	}

	// Enforce loopbackIP binding.
	if cfg.Host != loopbackIP {
		configLogger().Error("environment.mcp: host must be 127.0.0.1", "host", cfg.Host)
		cfg.Host = loopbackIP
	}

	if cfg.Port == "" {
		return MCPConfig{}, false
	}

	return cfg, true
}

// LGConfig holds parsed environment.looking-glass settings.
type LGConfig struct {
	Host string // Listen host (e.g., 0.0.0.0).
	Port string // Listen port (e.g., 8444).
	TLS  bool   // Enable TLS.
}

// Listen returns host:port.
func (c LGConfig) Listen() string { return c.Host + ":" + c.Port }

// ExtractLGConfig returns the environment.looking-glass config if enabled.
func ExtractLGConfig(tree *config.Tree) (LGConfig, bool) {
	if tree == nil {
		return LGConfig{}, false
	}
	envBlock := tree.GetContainer("environment")
	if envBlock == nil {
		return LGConfig{}, false
	}
	lg := envBlock.GetContainer("looking-glass")
	if lg == nil {
		return LGConfig{}, false
	}

	// Service must be explicitly enabled (default false).
	enabled, _ := lg.Get("enabled")
	if enabled != configTrue {
		return LGConfig{}, false
	}

	cfg := LGConfig{Host: "0.0.0.0", Port: "8443"}

	if servers := lg.GetListOrdered("server"); len(servers) > 0 {
		entry := servers[0].Value
		if v, ok := entry.Get("ip"); ok {
			cfg.Host = v
		}
		if v, ok := entry.Get("port"); ok {
			cfg.Port = v
		}
	}

	if v, ok := lg.Get("tls"); ok && v == configTrue {
		cfg.TLS = true
	}

	return cfg, true
}

// resolveSSHStorage returns blob storage for SSH host key persistence.
// When the main storage is already blob-backed, it is used directly.
// Otherwise, opens the zefs database independently so SSH host keys
// always go into the blob store rather than the filesystem.
// Tries configDir first, then DefaultConfigDir (binary-relative), because
// configDir may not contain database.zefs (e.g., stdin mode, temp dirs).
// Falls back to the passed store if zefs is not available anywhere.
func resolveSSHStorage(mainStore storage.Storage, configDir string) storage.Storage {
	if storage.IsBlobStorage(mainStore) {
		return mainStore
	}
	// Try configDir first, then binary-relative default.
	// configDir is almost never empty (LoadConfig sets it to cwd for stdin),
	// but may not contain database.zefs when the config file is elsewhere.
	candidates := [2]string{configDir, paths.DefaultConfigDir()}
	for _, dir := range candidates {
		if dir == "" {
			continue
		}
		dbPath := filepath.Join(dir, "database.zefs")
		blobStore, err := storage.NewBlob(dbPath, dir)
		if err == nil {
			return blobStore
		}
	}
	return mainStore
}

// loadZefsUsers reads SSH credentials from the zefs database (written by ze init).
// Opens database.zefs directly rather than using the storage abstraction,
// because storage may be filesystem-based (stdin mode) which can't read zefs keys.
// The zefs stores a bcrypt hash (written by ze init). This function uses the
// hash directly as UserConfig.Hash -- no re-hashing needed.
// Returns nil if keys are missing.
func loadZefsUsers() ([]zessh.UserConfig, error) {
	dir := paths.DefaultConfigDir()
	if dir == "" {
		return nil, fmt.Errorf("cannot resolve config dir")
	}
	dbPath := filepath.Join(dir, "database.zefs")
	db, err := zefs.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open credential store %s: %w", dbPath, err)
	}
	defer db.Close() //nolint:errcheck // read-only access

	username, err := db.ReadFile(zefs.KeySSHUsername.Pattern)
	if err != nil {
		return nil, fmt.Errorf("read ssh username: %w", err)
	}
	hash, err := db.ReadFile(zefs.KeySSHPassword.Pattern)
	if err != nil {
		return nil, fmt.Errorf("read ssh password hash: %w", err)
	}
	name := string(username)
	if name == "" {
		return nil, fmt.Errorf("empty username in zefs")
	}
	return []zessh.UserConfig{{Name: name, Hash: string(hash)}}, nil
}

// formatResponseData converts a command response Data value to a human-readable string.
// Strings pass through directly. Maps and other complex types are JSON-encoded with indentation.
func formatResponseData(data any) string {
	if data == nil {
		return ""
	}
	if s, ok := data.(string); ok {
		return s
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", data)
	}
	return string(b)
}

// collectPrefixWarnings gathers prefix warnings for the login banner.
// Two kinds: stale prefix data (config-level) and active threshold exceeded (runtime).
// If exactly one warning exists, the specific detail is shown.
// If more than one, a count is shown with the command to investigate.
func collectPrefixWarnings(rl plugin.ReactorIntrospector) []cli.LoginWarning {
	peers := rl.Peers()
	now := time.Now()

	var warnings []cli.LoginWarning
	for i := range peers {
		p := &peers[i]
		label := peerLabel(p)

		if reactor.IsPrefixDataStale(p.PrefixUpdated, now) {
			warnings = append(warnings, cli.LoginWarning{
				Message: fmt.Sprintf("%s has stale prefix data (updated %s)", label, p.PrefixUpdated),
				Command: "update bgp peer " + p.Address.String() + " prefix",
			})
		}
		for _, family := range p.PrefixWarnings {
			warnings = append(warnings, cli.LoginWarning{
				Message: fmt.Sprintf("%s %s prefix count exceeds warning threshold", label, family),
			})
		}
	}

	if len(warnings) == 0 {
		return nil
	}
	if len(warnings) == 1 {
		return warnings
	}
	return []cli.LoginWarning{{
		Message: fmt.Sprintf("%d warnings", len(warnings)),
		Command: "show bgp warnings",
	}}
}

// peerLabel returns a human-readable label for a peer (name or IP + AS).
func peerLabel(p *plugin.PeerInfo) string {
	if p.Name != "" {
		return fmt.Sprintf("peer %s (AS%d)", p.Name, p.PeerAS)
	}
	return fmt.Sprintf("peer %s (AS%d)", p.Address, p.PeerAS)
}
