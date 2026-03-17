// Design: docs/architecture/config/syntax.md — config file loading and reactor creation
// Detail: loader_routes.go — BGP route type conversion
// Detail: loader_prefix.go — prefix expansion for route splitting

package bgpconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/chaos"
	"codeberg.org/thomas-mangin/ze/internal/component/authz"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/reactor"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	zessh "codeberg.org/thomas-mangin/ze/internal/component/ssh"
	"codeberg.org/thomas-mangin/ze/internal/core/clock"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/network"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// configLogger is the config subsystem logger (lazy initialization).
// Controlled by ze.log.config environment variable.
// Uses LazyLogger to pick up config file settings applied after init().
var configLogger = slogutil.LazyLogger("config")

// Origin attribute values.
const (
	originIGP = "igp"
	originEGP = "egp"
)

// normalizeListenAddr ensures listen address has ip:port format.
// Accepts "ip:port", "[ipv6]:port", or bare "ip"/"[ipv6]" (port from environment/default).
func normalizeListenAddr(addr string, defaultPort int) string {
	if _, err := netip.ParseAddrPort(addr); err == nil {
		return addr
	}
	ip, err := netip.ParseAddr(strings.Trim(addr, "[]"))
	if err != nil {
		return addr
	}
	return netip.AddrPortFrom(ip, uint16(defaultPort)).String()
}

// parseTreeWithYANG parses config with optional plugin YANG schemas.
// Returns the parsed tree for further processing by callers.
func parseTreeWithYANG(input string, pluginYANG map[string]string) (*config.Tree, error) {
	// Parse input using YANG-derived schema with plugin augmentations
	var schema *config.Schema
	if len(pluginYANG) > 0 {
		schema = config.YANGSchemaWithPlugins(pluginYANG)
	} else {
		schema = config.YANGSchema()
	}
	if schema == nil {
		return nil, fmt.Errorf("failed to load YANG schema")
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

// LoadReactorWithPlugins parses config with CLI plugins and creates Reactor.
// configPath is the original file path (used for SIGHUP reload). May be empty or "-".
// store is used by the reload function to re-read config on SIGHUP; may be nil when
// configPath is "" or "-" (reload not supported).
// This is used when config data is already read (e.g., from stdin) and plugins
// need to be merged in.
func LoadReactorWithPlugins(store storage.Storage, input, configPath string, cliPlugins []string) (*reactor.Reactor, error) {
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

	r, err := CreateReactorFromTree(tree, configDir, configPath, plugins, store)
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
		return nil, err
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

	// Wire YANG validator for runtime attribute validation (origin enum, med/local-pref ranges)
	if v := config.YANGValidatorWithPlugins(pluginYANG); v != nil {
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

// CreateReactorFromTree creates a Reactor directly from a parsed config tree.
func CreateReactorFromTree(tree *config.Tree, configDir, configPath string, plugins []reactor.PluginConfig, store storage.Storage) (*reactor.Reactor, error) {
	// Load environment with config block values (if any)
	envValues := config.ExtractEnvironment(tree)
	env, err := config.LoadEnvironmentWithConfig(envValues)
	if err != nil {
		return nil, fmt.Errorf("load environment: %w", err)
	}

	// Extract global BGP settings directly from tree
	var routerID uint32
	var localAS uint32
	var listen string
	if bgpContainer := tree.GetContainer("bgp"); bgpContainer != nil {
		if v, ok := bgpContainer.Get("router-id"); ok {
			if ip, parseErr := netip.ParseAddr(v); parseErr == nil {
				routerID = ipToUint32(ip)
			}
		}
		if v, ok := bgpContainer.Get("local-as"); ok {
			if n, parseErr := strconv.ParseUint(v, 10, 32); parseErr == nil {
				localAS = uint32(n)
			}
		}
		if v, ok := bgpContainer.Get("listen"); ok {
			listen = normalizeListenAddr(v, env.TCP.Port)
		}
	}

	// Build peers from tree (resolves templates, extracts routes)
	peers, err := PeersFromConfigTree(tree)
	if err != nil {
		return nil, err
	}

	// Validate plugin references
	if err := ValidatePluginReferences(tree, plugins); err != nil {
		return nil, err
	}

	// Derive ConfiguredFamilies from peer capabilities.
	// Multiprotocol capabilities declare which families each peer supports.
	var configuredFamilies []string
	familySeen := make(map[string]bool)
	for _, ps := range peers {
		for _, cap := range ps.Capabilities {
			if mp, ok := cap.(*capability.Multiprotocol); ok {
				family := nlri.Family{AFI: mp.AFI, SAFI: mp.SAFI}
				fs := family.String()
				if !familySeen[fs] {
					familySeen[fs] = true
					configuredFamilies = append(configuredFamilies, fs)
				}
			}
		}
	}

	// Build reactor config
	reactorCfg := &reactor.Config{
		ListenAddr:         listen,
		RouterID:           routerID,
		LocalAS:            localAS,
		ConfigDir:          configDir,
		ConfigTree:         tree.ToMap(),
		MaxSessions:        env.TCP.Attempts, // tcp.attempts: exit after N sessions (0=unlimited)
		ConfiguredFamilies: configuredFamilies,
		Plugins:            plugins,
		RecentUpdateMax:    env.Reactor.CacheMax,
	}

	r := reactor.New(reactorCfg)

	// Start pprof HTTP server from config environment block.
	// CLI --pprof flag takes precedence (started earlier in main.go).
	if env.Debug.Pprof != "" {
		startPprofServer(env.Debug.Pprof)
	}

	// Start Prometheus metrics HTTP server from telemetry config block.
	// Creates a shared registry that the reactor (and future components) register metrics into.
	if addr, port, path, ok := metrics.ExtractTelemetryConfig(tree.ToMap()); ok {
		reg := metrics.NewPrometheusRegistry()
		var srv metrics.Server
		if err := srv.Start(reg, addr, port, path); err != nil {
			configLogger().Warn("metrics server failed to start", "error", err)
		} else {
			configLogger().Info("prometheus metrics enabled",
				"address", addr, "port", port, "path", path)
			r.SetMetricsRegistry(reg)
			registry.SetMetricsRegistry(reg)
		}
	}

	// Validate authorization config (AC-8: reject undefined profile references).
	if err := ValidateAuthzConfig(tree); err != nil {
		return nil, fmt.Errorf("authorization config: %w", err)
	}

	// Extract authz profiles from config (independent of SSH).
	authzStore := extractAuthzConfig(tree)

	// Start SSH server from system config block.
	// SSH binds the port immediately, but the command executor is wired later
	// via SetExecutorFactory after the reactor's API server starts (post-start hook).
	var sshSrv *zessh.Server
	if sshCfg, ok := extractSSHConfig(tree); ok {
		sshCfg.Storage = store
		sshCfg.ConfigPath = configPath
		srv, sshErr := zessh.NewServer(sshCfg)
		if sshErr != nil {
			configLogger().Warn("SSH server config error", "error", sshErr)
		} else if startErr := srv.Start(context.Background(), nil, nil); startErr != nil {
			configLogger().Warn("SSH server failed to start", "error", startErr)
		} else {
			configLogger().Info("SSH server listening", "address", srv.Address())
			sshSrv = srv
		}
	}

	// Deferred wiring: after reactor starts and Dispatcher is available,
	// connect authorization store and SSH executor (if configured).
	if authzStore != nil || sshSrv != nil {
		r.SetPostStartFunc(func() {
			d := r.Dispatcher()
			if d == nil {
				return
			}

			// Wire authorization (applies to API socket + SSH)
			if authzStore != nil {
				d.SetAuthorizer(authzStore)
				configLogger().Info("authorization profiles loaded")
			}

			// Wire SSH command executor with per-session username
			if sshSrv != nil {
				apiServer := r.APIServer()
				sshSrv.SetExecutorFactory(func(username string) zessh.CommandExecutor {
					return func(input string) (string, error) {
						ctx := &pluginserver.CommandContext{
							Server:   apiServer,
							Username: username,
						}
						resp, err := d.Dispatch(ctx, input)
						if err != nil {
							return "", err
						}
						if resp == nil {
							return "", nil
						}
						return formatResponseData(resp.Data), nil
					}
				})
				sshSrv.SetShutdownFunc(func() { r.Stop() })
				configLogger().Info("SSH command executor wired")
			}
		})
	}

	// Inject chaos wrappers from config environment block.
	// CLI flags (--chaos-seed) override this via SetClock/SetDialer/SetListenerFactory after load.
	if env.Chaos.Seed != 0 {
		resolved := chaos.ResolveSeed(env.Chaos.Seed)
		chaosLogger := slogutil.Logger("chaos")
		chaosCfg := chaos.ChaosConfig{Seed: resolved, Rate: env.Chaos.Rate, Logger: chaosLogger}
		clock, dialer, lf := chaos.NewChaosWrappers(clock.RealClock{}, &network.RealDialer{}, network.RealListenerFactory{}, chaosCfg)
		r.SetClock(clock)
		r.SetDialer(dialer)
		r.SetListenerFactory(lf)
		chaosLogger.Info("chaos self-test mode enabled (config)", "seed", resolved, "rate", env.Chaos.Rate)
	}

	// Add peers
	for _, ps := range peers {
		if err := r.AddPeer(ps); err != nil {
			return nil, fmt.Errorf("add peer %s: %w", ps.Address, err)
		}
	}

	return r, nil
}

// createReloadFunc creates a ReloadFunc that parses config files.
// It returns full PeerSettings to ensure reloaded peers are identical to initial load.
// Uses PeersFromConfigTree which resolves templates and extracts routes directly.
func createReloadFunc(store storage.Storage) reactor.ReloadFunc {
	return func(configPath string) ([]*reactor.PeerSettings, error) {
		data, err := store.ReadFile(configPath)
		if err != nil {
			return nil, err
		}

		// Parse the config using YANG-derived schema.
		schema := config.YANGSchema()
		if schema == nil {
			return nil, fmt.Errorf("failed to load YANG schema")
		}
		p := config.NewParser(schema)
		tree, err := p.Parse(string(data))
		if err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}

		return PeersFromConfigTree(tree)
	}
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
			"Run 'ze bgp config check <file>' to verify, then\n" +
			"Run 'ze bgp config migrate <file>' to upgrade."
	}

	return ""
}

// ValidateAuthzConfig validates authorization config in the parsed tree.
// Checks: profile entry regex syntax (hard error), user→profile references (AC-8).
// Exported so ze validate can also call it.
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
	sys := tree.GetContainer("system")
	if sys == nil {
		return zessh.Config{}, false
	}

	sshContainer := sys.GetContainer("ssh")
	if sshContainer == nil {
		return zessh.Config{}, false
	}

	// ConfigDir intentionally left empty — host key resolves from binary
	// location via paths.DefaultConfigDir() (e.g., ./bin/ze → etc/ze/).
	// The configDir parameter is the config file directory (or cwd for stdin),
	// which is wrong for host key placement.
	var cfg zessh.Config

	if addrs := sshContainer.GetSlice("listen"); len(addrs) > 0 {
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

	// Extract users from system.authentication.user list.
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

	return cfg, true
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
