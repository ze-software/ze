// Design: docs/architecture/config/syntax.md — config file loading and reactor creation
// Detail: loader_routes.go — BGP route type conversion
// Detail: loader_prefix.go — prefix expansion for route splitting

package config

import (
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // pprof server only starts when configured
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
	"codeberg.org/thomas-mangin/ze/internal/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/reactor"
	"codeberg.org/thomas-mangin/ze/internal/sim"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
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
func parseTreeWithYANG(input string, pluginYANG map[string]string) (*Tree, error) {
	// Parse input using YANG-derived schema with plugin augmentations
	var schema *Schema
	if len(pluginYANG) > 0 {
		schema = YANGSchemaWithPlugins(pluginYANG)
	} else {
		schema = YANGSchema()
	}
	if schema == nil {
		return nil, fmt.Errorf("failed to load YANG schema")
	}
	p := NewParser(schema)
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
	envValues := ExtractEnvironment(tree)
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
	return CreateReactorFromTree(tree, "", plugins)
}

// LoadReactorWithPlugins parses config with CLI plugins and creates Reactor.
// configPath is the original file path (used for SIGHUP reload). May be empty or "-".
// This is used when config data is already read (e.g., from stdin) and plugins
// need to be merged in.
func LoadReactorWithPlugins(input, configPath string, cliPlugins []string) (*reactor.Reactor, error) {
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

	r, err := CreateReactorFromTree(tree, configDir, plugins)
	if err != nil {
		return nil, err
	}

	// Set config path for SIGHUP reload support
	if configPath != "" && configPath != "-" {
		r.SetConfigPath(configPath)
		r.SetReloadFunc(createReloadFunc())
	}

	return r, nil
}

// LoadReactorFile loads config from file and creates Reactor.
func LoadReactorFile(path string) (*reactor.Reactor, error) {
	return LoadReactorFileWithPlugins(path, nil)
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
func LoadReactorFileWithPlugins(path string, cliPlugins []string) (*reactor.Reactor, error) {
	var data []byte
	var err error

	// Support stdin when path is "-"
	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path) //nolint:gosec // Config file path from user
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
	if v := YANGValidatorWithPlugins(pluginYANG); v != nil {
		plugin.SetYANGValidator(v)
	}

	// Create reactor from tree
	r, err := CreateReactorFromTree(tree, configDir, plugins)
	if err != nil {
		return nil, err
	}

	// Set config path for SIGHUP reload support
	if path != "-" {
		r.SetConfigPath(absPath)
		r.SetReloadFunc(createReloadFunc())
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
func CreateReactorFromTree(tree *Tree, configDir string, plugins []reactor.PluginConfig) (*reactor.Reactor, error) {
	// Load environment with config block values (if any)
	envValues := ExtractEnvironment(tree)
	env, err := LoadEnvironmentWithConfig(envValues)
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

	// Always set API socket path so CLI can connect to the daemon
	reactorCfg.APISocketPath = env.SocketPath()

	r := reactor.New(reactorCfg)

	// Start pprof HTTP server from config environment block.
	// CLI --pprof flag takes precedence (started earlier in main.go).
	if env.Debug.Pprof != "" {
		pprofAddr := env.Debug.Pprof
		configLogger().Info("pprof server starting (config)", "addr", pprofAddr)
		go func() {
			if err := http.ListenAndServe(pprofAddr, nil); err != nil { //nolint:gosec // pprof is intentionally bound to configured address
				configLogger().Error("pprof server failed", "error", err)
			}
		}()
	}

	// Inject chaos wrappers from config environment block.
	// CLI flags (--chaos-seed) override this via SetClock/SetDialer/SetListenerFactory after load.
	if env.Chaos.Seed != 0 {
		resolved := sim.ResolveSeed(env.Chaos.Seed)
		chaosLogger := slogutil.Logger("chaos")
		chaosCfg := sim.ChaosConfig{Seed: resolved, Rate: env.Chaos.Rate, Logger: chaosLogger}
		clock, dialer, lf := sim.NewChaosWrappers(sim.RealClock{}, &sim.RealDialer{}, sim.RealListenerFactory{}, chaosCfg)
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
func createReloadFunc() reactor.ReloadFunc {
	return func(configPath string) ([]*reactor.PeerSettings, error) {
		data, err := os.ReadFile(configPath) //nolint:gosec // User-provided config path
		if err != nil {
			return nil, err
		}

		// Parse the config using YANG-derived schema.
		schema := YANGSchema()
		if schema == nil {
			return nil, fmt.Errorf("failed to load YANG schema")
		}
		p := NewParser(schema)
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
