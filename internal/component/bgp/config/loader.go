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

	"codeberg.org/thomas-mangin/ze/internal/chaos"
	"codeberg.org/thomas-mangin/ze/internal/component/authz"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/grmarker"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/reactor"
	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	_ "codeberg.org/thomas-mangin/ze/internal/component/config/migration" // init() registers migration function
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	zessh "codeberg.org/thomas-mangin/ze/internal/component/ssh"
	"codeberg.org/thomas-mangin/ze/internal/core/clock"
	"codeberg.org/thomas-mangin/ze/internal/core/network"
	"codeberg.org/thomas-mangin/ze/internal/core/paths"
	"codeberg.org/thomas-mangin/ze/internal/core/report"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

// configLogger is the config subsystem logger (lazy initialization).
// Controlled by ze.log.bgp.config environment variable.
// Uses LazyLogger to pick up config file settings applied after init().
var configLogger = slogutil.LazyLogger("bgp.config")

// Origin attribute values.
const (
	originIGP = "igp"
	originEGP = "egp"
)

// LoadReactor parses config and creates a configured Reactor.
func LoadReactor(input string) (*reactor.Reactor, error) {
	result, err := config.LoadConfig(input, "", nil)
	if err != nil {
		return nil, err
	}
	return CreateReactorFromTree(result.Tree, "", "", result.Plugins, nil)
}

// loadContext stores the config.LoadConfigResult and Storage for in-process BGP plugin use.
// Set by the hub after LoadConfig. Retrieved by BGP's RunEngine to create the reactor
// without re-parsing the config.
var loadContext struct {
	result     *config.LoadConfigResult
	configPath string
	store      any // storage.Storage (any to avoid import cycle)
}

// StoreLoadContext saves the LoadConfigResult and storage for retrieval by
// the BGP plugin's RunEngine. Must be called after LoadConfig.
func StoreLoadContext(result *config.LoadConfigResult, configPath string, store any) {
	loadContext.result = result
	loadContext.configPath = configPath
	loadContext.store = store
}

// GetLoadContext returns the stored LoadConfigResult, config path, and storage.
// Returns nil result if StoreLoadContext was not called.
func GetLoadContext() (*config.LoadConfigResult, string, any) {
	return loadContext.result, loadContext.configPath, loadContext.store
}

// chaosConfig stores chaos testing parameters for BGP plugin injection.
var chaosConfig struct {
	seed int64
	rate float64
}

// StoreLoadChaos saves chaos config for retrieval by the BGP plugin's RunEngine.
func StoreLoadChaos(seed int64, rate float64) {
	chaosConfig.seed = seed
	chaosConfig.rate = rate
}

// GetLoadChaos returns the stored chaos config (seed, rate). Zero seed means disabled.
func GetLoadChaos() (int64, float64) {
	return chaosConfig.seed, chaosConfig.rate
}

// CreateReactor creates a Reactor from a config.LoadConfigResult.
func CreateReactor(cfg *config.LoadConfigResult, configPath string, store storage.Storage) (*reactor.Reactor, error) {
	r, err := CreateReactorFromTree(cfg.Tree, cfg.ConfigDir, configPath, cfg.Plugins, store)
	if err != nil {
		return nil, err
	}

	if configPath != "" && configPath != "-" {
		r.SetConfigPath(configPath)
		r.SetReloadFunc(createReloadFunc(store))
	}

	return r, nil
}

// LoadReactorWithPlugins parses config with CLI plugins and creates Reactor.
func LoadReactorWithPlugins(store storage.Storage, input, configPath string, cliPlugins []string) (*reactor.Reactor, error) {
	cfg, err := config.LoadConfig(input, configPath, cliPlugins)
	if err != nil {
		return nil, err
	}
	return CreateReactor(cfg, configPath, store)
}

// LoadReactorFile loads config from file and creates Reactor.
func LoadReactorFile(store storage.Storage, path string) (*reactor.Reactor, error) {
	return LoadReactorFileWithPlugins(store, path, nil)
}

// LoadReactorFileWithPlugins loads config from file and creates Reactor.
func LoadReactorFileWithPlugins(store storage.Storage, path string, cliPlugins []string) (*reactor.Reactor, error) {
	var data []byte
	var err error

	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = store.ReadFile(path)
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	pluginYANG := plugin.CollectPluginYANG(cliPlugins)

	if v, vErr := config.YANGValidatorWithPlugins(pluginYANG); vErr == nil && v != nil {
		plugin.SetYANGValidator(v)
	}

	cfg, err := config.LoadConfig(string(data), path, cliPlugins)
	if err != nil {
		return nil, err
	}

	return CreateReactor(cfg, path, store)
}

// injectChaos wraps the reactor's clock, dialer, and listener with chaos fault injection
// when the coordinator has non-zero chaos seed stored by the hub.
func injectChaos(r *reactor.Reactor, coord registry.CoordinatorAccessor) {
	seed, _ := coord.GetExtra("bgp.chaosSeed").(int64)
	if seed == 0 {
		return
	}
	rate, _ := coord.GetExtra("bgp.chaosRate").(float64)
	if rate < 0 {
		rate = 0.1
	}
	resolvedSeed := chaos.ResolveSeed(seed)
	chaosLogger := slogutil.Logger("chaos")
	cfg := chaos.ChaosConfig{Seed: resolvedSeed, Rate: rate, Logger: chaosLogger}
	c, d, l := chaos.NewChaosWrappers(clock.RealClock{}, &network.RealDialer{}, network.RealListenerFactory{}, cfg)
	r.SetClock(c)
	r.SetDialer(d)
	r.SetListenerFactory(l)
	chaosLogger.Info("chaos self-test mode enabled", "seed", resolvedSeed, "rate", rate)
}

// readGRMarker reads and removes the Graceful Restart marker from storage.
// RFC 4724 Section 4.1: reactor uses the expiry to set R bit in OPEN capabilities.
func readGRMarker(r *reactor.Reactor, store storage.Storage) {
	if store == nil {
		return
	}
	if expiry, ok := grmarker.Read(store); ok {
		r.SetRestartUntil(expiry)
		slogutil.Logger("bgp.gr").Info("GR restart marker found", "expires", expiry)
	}
	if err := grmarker.Remove(store); err != nil {
		slogutil.Logger("bgp.gr").Warn("failed to remove GR marker", "error", err)
	}
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

// collectPrefixWarnings gathers BGP-sourced prefix warnings for the login banner.
// Reads from the report bus instead of per-peer state. Two kinds: stale prefix
// data (raised at peer add via report.RaiseWarning) and active threshold
// exceeded (raised by Session.applyPrefixCheck on the upward edge).
//
// If exactly one warning exists, the specific detail is shown.
// If more than one, a count is shown with the command to investigate.
//
// Malformed prefix-threshold subjects (missing the composite "<addr>/<family>"
// form) are skipped with a debug log rather than producing visually broken
// banner lines. Producers in this codebase always use the composite form;
// the skip handles future producers and protects the operator UI.
func collectPrefixWarnings(rl plugin.ReactorIntrospector) []cli.LoginWarning {
	peerNames := buildPeerNameLookup(rl)
	issues := report.Warnings()

	var warnings []cli.LoginWarning
	for i := range issues {
		issue := &issues[i]
		if issue.Source != "bgp" {
			continue
		}
		switch issue.Code {
		case "prefix-stale":
			label := peerLabelFromSubject(issue.Subject, peerNames)
			warnings = append(warnings, cli.LoginWarning{
				Message: fmt.Sprintf("%s has stale prefix data (updated %s)", label, detailString(issue.Detail, "updated")),
				Command: "update bgp peer " + issue.Subject + " prefix",
			})
		case "prefix-threshold":
			peerAddr, fam, ok := splitThresholdSubject(issue.Subject)
			if !ok {
				configLogger().Debug("skipping malformed prefix-threshold subject in banner",
					"subject", issue.Subject)
				continue
			}
			label := peerLabelFromSubject(peerAddr, peerNames)
			warnings = append(warnings, cli.LoginWarning{
				Message: fmt.Sprintf("%s %s prefix count exceeds warning threshold", label, fam),
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
		Command: "show warnings",
	}}
}

// buildPeerNameLookup walks the reactor peers once to build a peer-address-to-
// peer map, used to enrich bus warnings (which only carry the peer address)
// with the human-readable peer name from config. Stores PeerInfo by value
// (not pointer) so the map's lifetime does not depend on the lifetime of the
// slice returned by rl.Peers().
func buildPeerNameLookup(rl plugin.ReactorIntrospector) map[string]plugin.PeerInfo {
	if rl == nil {
		return nil
	}
	peers := rl.Peers()
	out := make(map[string]plugin.PeerInfo, len(peers))
	for i := range peers {
		out[peers[i].Address.String()] = peers[i]
	}
	return out
}

// peerLabelFromSubject returns a human-readable peer label given the peer
// address from a bus subject and the name lookup map. Falls back to the
// raw address when the peer is not found in the lookup (e.g., already removed).
func peerLabelFromSubject(addr string, lookup map[string]plugin.PeerInfo) string {
	if p, ok := lookup[addr]; ok {
		return peerLabel(&p)
	}
	return fmt.Sprintf("peer %s", addr)
}

// splitThresholdSubject parses the composite subject "<addr>/<afi>/<safi>"
// into peer address and family string. Returns ok=false when the format
// does not match (no "/", or "/" at start or end). Callers must check ok
// before using the returned values; malformed subjects should be skipped
// rather than producing broken UI text.
func splitThresholdSubject(subject string) (peerAddr, family string, ok bool) {
	idx := strings.Index(subject, "/")
	if idx <= 0 || idx == len(subject)-1 {
		return subject, "", false
	}
	return subject[:idx], subject[idx+1:], true
}

// detailString returns the string value of detail[key], or "" if missing or
// not a string. Used for safe extraction of bus-detail fields in user-facing text.
func detailString(detail map[string]any, key string) string {
	if detail == nil {
		return ""
	}
	v, ok := detail[key]
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// peerLabel returns a human-readable label for a peer (name or IP + AS).
func peerLabel(p *plugin.PeerInfo) string {
	if p.Name != "" {
		return fmt.Sprintf("peer %s (AS%d)", p.Name, p.PeerAS)
	}
	return fmt.Sprintf("peer %s (AS%d)", p.Address, p.PeerAS)
}
