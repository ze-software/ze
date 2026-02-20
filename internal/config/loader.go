// Design: docs/architecture/config/syntax.md — config file loading and BGP route encoding

package config

import (
	"encoding/hex"
	"fmt"
	"io"
	"maps"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // pprof server only starts when configured
	"net/netip"
	"os"
	"path/filepath"
	"slices"
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

// FlowSpec action names.
const flowSpecRedirectNextHop = "redirect-to-nexthop"

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
	return CreateReactorFromTree(tree, "", plugins)
}

// LoadReactorWithPlugins parses config with CLI plugins and creates Reactor.
// configPath is the original file path (used for SIGHUP reload). May be empty or "-".
// This is used when config data is already read (e.g., from stdin) and plugins
// need to be merged in.
func LoadReactorWithPlugins(input, configPath string, cliPlugins []string) (*reactor.Reactor, error) {
	// Start with all internal plugin YANG (GR, hostname, etc.)
	pluginYANG := plugin.GetAllInternalPluginYANG()
	// Add CLI-specified plugins (may override internal)
	maps.Copy(pluginYANG, plugin.CollectPluginYANG(cliPlugins))

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

	// Collect plugin YANG before parsing (plugins may augment schema)
	// Start with all internal plugin YANG (GR, hostname, etc.)
	pluginYANG := plugin.GetAllInternalPluginYANG()
	// Add CLI-specified plugins (may override internal)
	maps.Copy(pluginYANG, plugin.CollectPluginYANG(cliPlugins))

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
			listen = v
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

// convertMVPNRoute converts config MVPN route to reactor MVPN route.
func convertMVPNRoute(mr MVPNRouteConfig) (reactor.MVPNRoute, error) {
	route := reactor.MVPNRoute{
		IsIPv6:          mr.IsIPv6,
		SourceAS:        mr.SourceAS,
		LocalPreference: mr.LocalPreference,
		MED:             mr.MED,
	}

	// Route type
	switch mr.RouteType {
	case "source-ad":
		route.RouteType = 5
	case "shared-join":
		route.RouteType = 6
	case "source-join":
		route.RouteType = 7
	default:
		return route, fmt.Errorf("unknown MVPN route type: %s", mr.RouteType)
	}

	// Origin
	route.Origin = parseOrigin(mr.Origin)

	// Parse RD
	if mr.RD != "" {
		rd, err := ParseRouteDistinguisher(mr.RD)
		if err != nil {
			return route, fmt.Errorf("parse RD: %w", err)
		}
		route.RD = rd.Bytes
	}

	// Parse Source/RP IP
	if mr.Source != "" {
		ip, err := netip.ParseAddr(mr.Source)
		if err != nil {
			return route, fmt.Errorf("parse source: %w", err)
		}
		route.Source = ip
	}

	// Parse Group IP
	if mr.Group != "" {
		ip, err := netip.ParseAddr(mr.Group)
		if err != nil {
			return route, fmt.Errorf("parse group: %w", err)
		}
		route.Group = ip
	}

	// Parse NextHop
	if mr.NextHop != "" {
		ip, err := netip.ParseAddr(mr.NextHop)
		if err != nil {
			return route, fmt.Errorf("parse next-hop: %w", err)
		}
		route.NextHop = ip
	}

	// Parse extended communities
	if mr.ExtendedCommunity != "" {
		ec, err := ParseExtendedCommunity(mr.ExtendedCommunity)
		if err != nil {
			return route, fmt.Errorf("parse extended-community: %w", err)
		}
		route.ExtCommunityBytes = ec.Bytes
	}

	// Parse originator-id (RFC 4456)
	if mr.OriginatorID != "" {
		ip, err := netip.ParseAddr(mr.OriginatorID)
		if err != nil {
			return route, fmt.Errorf("parse originator-id: %w", err)
		}
		if ip.Is4() {
			b := ip.As4()
			route.OriginatorID = uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
		}
	}

	// Parse cluster-list (RFC 4456, space-separated IPs)
	if mr.ClusterList != "" {
		parts := strings.FieldsSeq(mr.ClusterList)
		for p := range parts {
			p = strings.Trim(p, "[]")
			if p == "" {
				continue
			}
			ip, err := netip.ParseAddr(p)
			if err != nil {
				return route, fmt.Errorf("parse cluster-list: %w", err)
			}
			if ip.Is4() {
				b := ip.As4()
				route.ClusterList = append(route.ClusterList, uint32(b[0])<<24|uint32(b[1])<<16|uint32(b[2])<<8|uint32(b[3]))
			}
		}
	}

	return route, nil
}

// convertVPLSRoute converts config VPLS route to reactor VPLS route.
func convertVPLSRoute(vr VPLSRouteConfig) (reactor.VPLSRoute, error) {
	route := reactor.VPLSRoute{
		Name:            vr.Name,
		Endpoint:        vr.Endpoint,
		Base:            vr.Base,
		Offset:          vr.Offset,
		Size:            vr.Size,
		LocalPreference: vr.LocalPreference,
		MED:             vr.MED,
	}

	// Origin
	route.Origin = parseOrigin(vr.Origin)

	// Parse RD
	if vr.RD != "" {
		rd, err := ParseRouteDistinguisher(vr.RD)
		if err != nil {
			return route, fmt.Errorf("parse RD: %w", err)
		}
		route.RD = rd.Bytes
	}

	// Parse NextHop
	if vr.NextHop != "" {
		ip, err := netip.ParseAddr(vr.NextHop)
		if err != nil {
			return route, fmt.Errorf("parse next-hop: %w", err)
		}
		route.NextHop = ip
	}

	// Parse AS Path
	if vr.ASPath != "" {
		asPath, err := parseASPathSimple(vr.ASPath)
		if err != nil {
			return route, fmt.Errorf("parse as-path: %w", err)
		}
		route.ASPath = asPath
	}

	// Parse communities
	if vr.Community != "" {
		comm, err := ParseCommunity(vr.Community)
		if err != nil {
			return route, fmt.Errorf("parse community: %w", err)
		}
		route.Communities = comm.Values
	}

	// Parse extended communities
	if vr.ExtendedCommunity != "" {
		ec, err := ParseExtendedCommunity(vr.ExtendedCommunity)
		if err != nil {
			return route, fmt.Errorf("parse extended-community: %w", err)
		}
		route.ExtCommunityBytes = sortExtCommunities(ec.Bytes)
	}

	// Parse originator-id
	if vr.OriginatorID != "" {
		ip, err := netip.ParseAddr(vr.OriginatorID)
		if err != nil {
			return route, fmt.Errorf("parse originator-id: %w", err)
		}
		if ip.Is4() {
			b := ip.As4()
			route.OriginatorID = uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
		}
	}

	// Parse cluster-list (space-separated IPs)
	if vr.ClusterList != "" {
		parts := strings.FieldsSeq(vr.ClusterList)
		for p := range parts {
			// Remove brackets
			p = strings.Trim(p, "[]")
			if p == "" {
				continue
			}
			ip, err := netip.ParseAddr(p)
			if err != nil {
				return route, fmt.Errorf("parse cluster-list: %w", err)
			}
			if ip.Is4() {
				b := ip.As4()
				route.ClusterList = append(route.ClusterList, uint32(b[0])<<24|uint32(b[1])<<16|uint32(b[2])<<8|uint32(b[3]))
			}
		}
	}

	return route, nil
}

// convertFlowSpecRoute converts config FlowSpec route to reactor FlowSpec route.
// RFC 8955 Section 4 defines the FlowSpec NLRI format.
// RFC 8955 Section 7 defines the Traffic Filtering Actions (extended communities).
// RFC 8955 Section 8 defines the FlowSpec VPN variant (SAFI 134) with Route Distinguisher.
func convertFlowSpecRoute(fr FlowSpecRouteConfig) (reactor.FlowSpecRoute, error) {
	route := reactor.FlowSpecRoute{
		Name:   fr.Name,
		IsIPv6: fr.IsIPv6,
	}

	// Parse RD for flow-vpn
	if fr.RD != "" {
		rd, err := ParseRouteDistinguisher(fr.RD)
		if err != nil {
			return route, fmt.Errorf("parse RD: %w", err)
		}
		route.RD = rd.Bytes
	}

	// Parse NextHop
	if fr.NextHop != "" {
		ip, err := netip.ParseAddr(fr.NextHop)
		if err != nil {
			return route, fmt.Errorf("parse next-hop: %w", err)
		}
		route.NextHop = ip
	}

	// Build FlowSpec NLRI from match criteria (RFC 8955 Section 4)
	// For VPN routes, use component bytes (no length prefix - VPN adds its own)
	isVPN := fr.RD != ""
	flowFamily := "ipv4/flow"
	if fr.IsIPv6 {
		flowFamily = "ipv6/flow"
	}
	if builder := registry.ConfigNLRIBuilder(flowFamily); builder != nil {
		route.NLRI = builder(fr.NLRI, fr.IsIPv6, isVPN)
	}

	// Build communities (RFC 1997)
	if c := fr.Community; c != "" {
		comm, err := ParseCommunity(c)
		if err != nil {
			return route, fmt.Errorf("parse community: %w", err)
		}
		// Convert []uint32 to wire bytes (4 bytes each, big-endian)
		for _, v := range comm.Values {
			route.CommunityBytes = append(route.CommunityBytes,
				byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
		}
	}

	// Build extended communities (RFC 8955 Section 7)
	// Actions like discard, rate-limit, redirect are encoded as extended communities
	if ec := fr.ExtendedCommunity; ec != "" {
		extComm, err := ParseExtendedCommunity(ec)
		if err != nil {
			return route, fmt.Errorf("parse extended-community: %w", err)
		}
		route.ExtCommunityBytes = append(route.ExtCommunityBytes, extComm.Bytes...)

		// Build IPv6 Extended Communities (attribute 25) for redirect-to-nexthop with IPv6
		route.IPv6ExtCommunityBytes = buildIPv6ExtCommunityFromString(ec)
	}

	// Sort extended communities by type for RFC 4360 compliance
	route.ExtCommunityBytes = sortExtCommunities(route.ExtCommunityBytes)

	// Handle raw attributes (e.g., attribute 25 for IPv6 Extended Communities)
	if fr.Attribute != "" {
		rawAttr, err := ParseRawAttribute(fr.Attribute)
		if err != nil {
			return route, fmt.Errorf("parse raw attribute: %w", err)
		}
		// Attribute 25 = IPv6 Extended Communities (RFC 5701)
		if rawAttr.Code == 25 {
			route.IPv6ExtCommunityBytes = append(route.IPv6ExtCommunityBytes, rawAttr.Value...)
		}
	}

	return route, nil
}

// sortExtCommunities sorts extended communities by type for RFC 4360 compliance.
// Each extended community is 8 bytes. Sorting by the 64-bit value puts lower
// type codes first (e.g., origin 0x0003 before redirect 0x8008).
// Trailing bytes that don't form a complete community are discarded.
func sortExtCommunities(data []byte) []byte {
	if len(data) < 16 { // Need at least 2 communities to sort
		return data
	}

	// Validate and truncate to complete communities only
	count := len(data) / 8
	if count*8 != len(data) {
		// Discard trailing bytes that don't form a complete community
		data = data[:count*8]
	}
	communities := make([]uint64, count)
	for i := range count {
		offset := i * 8
		communities[i] = uint64(data[offset])<<56 |
			uint64(data[offset+1])<<48 |
			uint64(data[offset+2])<<40 |
			uint64(data[offset+3])<<32 |
			uint64(data[offset+4])<<24 |
			uint64(data[offset+5])<<16 |
			uint64(data[offset+6])<<8 |
			uint64(data[offset+7])
	}

	// Sort by value (lower type codes first)
	slices.Sort(communities)

	// Rebuild byte slice
	result := make([]byte, len(data))
	for i, c := range communities {
		offset := i * 8
		result[offset] = byte(c >> 56)
		result[offset+1] = byte(c >> 48)
		result[offset+2] = byte(c >> 40)
		result[offset+3] = byte(c >> 32)
		result[offset+4] = byte(c >> 24)
		result[offset+5] = byte(c >> 16)
		result[offset+6] = byte(c >> 8)
		result[offset+7] = byte(c)
	}
	return result
}

// buildIPv6ExtCommunityFromString builds IPv6 Extended Communities (attribute 25, RFC 5701)
// from an extended community string. Only extracts redirect-to-nexthop with IPv6 addresses.
// RFC 7674 Section 3.2 defines the Redirect to IPv6 action (subtype 0x000c).
func buildIPv6ExtCommunityFromString(ec string) []byte {
	var result []byte
	parts := strings.Fields(ec)

	for i := 0; i < len(parts); i++ {
		if parts[i] == flowSpecRedirectNextHop && i+1 < len(parts) {
			// Check if next part is an IPv6 address
			if ip, err := netip.ParseAddr(parts[i+1]); err == nil && ip.Is6() {
				// RFC 5701: IPv6 Extended Community = subtype(2) + IPv6(16) + copy_flag(2) = 20 bytes
				ipBytes := ip.As16()
				result = append(result, 0x00, 0x0c) // Subtype 0x000c = redirect to IP
				result = append(result, ipBytes[:]...)
				result = append(result, 0x00, 0x00) // Copy flag = 0
			}
			i++ // Skip the IP address part
		}
	}

	return result
}

// convertMUPRoute converts config MUP route to reactor MUP route.
func convertMUPRoute(mr MUPRouteConfig) (reactor.MUPRoute, error) {
	route := reactor.MUPRoute{
		IsIPv6: mr.IsIPv6,
	}

	// Route type
	switch mr.RouteType {
	case "mup-isd":
		route.RouteType = 1
	case "mup-dsd":
		route.RouteType = 2
	case "mup-t1st":
		route.RouteType = 3
	case "mup-t2st":
		route.RouteType = 4
	default:
		return route, fmt.Errorf("unknown MUP route type: %s", mr.RouteType)
	}

	// Parse NextHop
	if mr.NextHop != "" {
		ip, err := netip.ParseAddr(mr.NextHop)
		if err != nil {
			return route, fmt.Errorf("parse next-hop: %w", err)
		}
		route.NextHop = ip
	}

	// Parse extended communities
	if mr.ExtendedCommunity != "" {
		ec, err := ParseExtendedCommunity(mr.ExtendedCommunity)
		if err != nil {
			return route, fmt.Errorf("parse extended-community: %w", err)
		}
		route.ExtCommunityBytes = ec.Bytes
	}

	// Build MUP NLRI via registry (avoids direct plugin import)
	mupFamily := "ipv4/mup"
	if mr.IsIPv6 {
		mupFamily = "ipv6/mup"
	}
	mupArgs := mupRouteConfigToArgs(mr)
	nlriHex, err := registry.EncodeNLRIByFamily(mupFamily, mupArgs)
	if err != nil {
		return route, fmt.Errorf("build MUP NLRI: %w", err)
	}
	nlriBytes, err := hex.DecodeString(nlriHex)
	if err != nil {
		return route, fmt.Errorf("decode MUP NLRI hex: %w", err)
	}
	route.NLRI = nlriBytes

	// Parse SRv6 Prefix-SID if present
	if mr.PrefixSID != "" {
		sid, err := ParsePrefixSIDSRv6(mr.PrefixSID)
		if err != nil {
			return route, fmt.Errorf("parse prefix-sid-srv6: %w", err)
		}
		route.PrefixSID = sid.Bytes
	}

	return route, nil
}

// mupRouteConfigToArgs converts MUPRouteConfig to CLI-style args for registry.EncodeNLRIByFamily.
func mupRouteConfigToArgs(mr MUPRouteConfig) []string {
	var args []string
	if mr.RouteType != "" {
		args = append(args, "route-type", mr.RouteType)
	}
	if mr.RD != "" {
		args = append(args, "rd", mr.RD)
	}
	if mr.Prefix != "" {
		args = append(args, "prefix", mr.Prefix)
	}
	if mr.Address != "" {
		args = append(args, "address", mr.Address)
	}
	if mr.TEID != "" {
		args = append(args, "teid", mr.TEID)
	}
	if mr.QFI != 0 {
		args = append(args, "qfi", strconv.FormatUint(uint64(mr.QFI), 10))
	}
	if mr.Endpoint != "" {
		args = append(args, "endpoint", mr.Endpoint)
	}
	if mr.Source != "" {
		args = append(args, "source", mr.Source)
	}
	return args
}

// parseOrigin converts origin string to code.
// Empty or unset defaults to IGP (0).
func parseOrigin(s string) uint8 {
	switch strings.ToLower(s) {
	case "", originIGP:
		return 0 // IGP is default
	case originEGP:
		return 1
	default:
		return 2 // incomplete
	}
}

// parseASPathSimple parses an AS path string like "[ 30740 30740 ]" to []uint32.
func parseASPathSimple(s string) ([]uint32, error) {
	s = strings.Trim(s, "[]")
	parts := strings.Fields(s)
	result := make([]uint32, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		n, err := strconv.ParseUint(p, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid ASN: %s", p)
		}
		result = append(result, uint32(n))
	}
	return result, nil
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

// parseSplitLen parses a split specification like "/25" and returns the prefix length.
// Returns 0 if no split or invalid format.
func parseSplitLen(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	s = strings.TrimPrefix(s, "/")
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 128 {
		return 0
	}
	return n
}

// expandPrefix expands a prefix into more-specific prefixes with the given length.
// For example, 10.0.0.0/24 expanded to /25 produces two /25 prefixes.
// Returns the original prefix unchanged if targetLen is invalid.
// Note: route.splitPrefix has the same logic but returns an error for invalid input.
func expandPrefix(prefix netip.Prefix, targetLen int) []netip.Prefix {
	sourceBits := prefix.Bits()

	// Validate target length
	maxBits := 32
	if prefix.Addr().Is6() {
		maxBits = 128
	}

	if targetLen <= sourceBits || targetLen > maxBits {
		return []netip.Prefix{prefix}
	}

	// Calculate number of resulting prefixes: 2^(targetLen - sourceBits)
	numPrefixes := 1 << (targetLen - sourceBits)
	result := make([]netip.Prefix, 0, numPrefixes)

	baseAddr := prefix.Addr()
	for i := range numPrefixes {
		newAddr := addToAddr(baseAddr, i, targetLen)
		result = append(result, netip.PrefixFrom(newAddr, targetLen))
	}

	return result
}

// addToAddr adds an offset to an address at the given prefix boundary.
// Identical to route.addToAddr — not consolidated because config must not import plugin packages.
func addToAddr(addr netip.Addr, offset int, prefixLen int) netip.Addr {
	if offset == 0 {
		return addr
	}

	maxBits := 32
	if addr.Is6() {
		maxBits = 128
	}
	shift := maxBits - prefixLen

	if addr.Is4() {
		v4 := addr.As4()
		val := uint32(v4[0])<<24 | uint32(v4[1])<<16 | uint32(v4[2])<<8 | uint32(v4[3])
		val += uint32(offset) << shift //nolint:gosec // offset is bounded
		return netip.AddrFrom4([4]byte{byte(val >> 24), byte(val >> 16), byte(val >> 8), byte(val)})
	}

	// IPv6
	v6 := addr.As16()
	hi := uint64(v6[0])<<56 | uint64(v6[1])<<48 | uint64(v6[2])<<40 | uint64(v6[3])<<32 |
		uint64(v6[4])<<24 | uint64(v6[5])<<16 | uint64(v6[6])<<8 | uint64(v6[7])
	lo := uint64(v6[8])<<56 | uint64(v6[9])<<48 | uint64(v6[10])<<40 | uint64(v6[11])<<32 |
		uint64(v6[12])<<24 | uint64(v6[13])<<16 | uint64(v6[14])<<8 | uint64(v6[15])

	if shift >= 64 {
		hi += uint64(offset) << (shift - 64) //nolint:gosec // offset is bounded
	} else {
		addLo := uint64(offset) << shift //nolint:gosec // offset is bounded
		newLo := lo + addLo
		if newLo < lo {
			hi++
		}
		lo = newLo
	}

	return netip.AddrFrom16([16]byte{
		byte(hi >> 56), byte(hi >> 48), byte(hi >> 40), byte(hi >> 32),
		byte(hi >> 24), byte(hi >> 16), byte(hi >> 8), byte(hi),
		byte(lo >> 56), byte(lo >> 48), byte(lo >> 40), byte(lo >> 32),
		byte(lo >> 24), byte(lo >> 16), byte(lo >> 8), byte(lo),
	})
}
