// Design: docs/architecture/config/syntax.md — config file loading and reactor creation
// Detail: loader_routes.go — BGP route type conversion
// Detail: loader_prefix.go — prefix expansion for route splitting
// Detail: loader_create.go — reactor creation from config tree
// Detail: plugins.go — plugin extraction from config tree

package bgpconfig

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/chaos"
	"codeberg.org/thomas-mangin/ze/internal/component/aaa"
	"codeberg.org/thomas-mangin/ze/internal/component/authz"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/grmarker"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/reactor"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	_ "codeberg.org/thomas-mangin/ze/internal/component/config/migration" // init() registers migration function
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/cliio"
	"codeberg.org/thomas-mangin/ze/internal/core/clock"
	"codeberg.org/thomas-mangin/ze/internal/core/network"
	"codeberg.org/thomas-mangin/ze/internal/core/paths"
	"codeberg.org/thomas-mangin/ze/internal/core/report"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

// configLogger is the config subsystem logger (lazy initialization).
// Controlled by ze.log.bgp.config environment variable.
// Uses LazyLogger to pick up config file settings applied after init().
var configLogger = slogutil.LazyLogger("bgp.config")

// LoadReactor parses config and creates a configured Reactor.
func LoadReactor(input string) (*reactor.Reactor, error) {
	result, err := config.LoadConfig(input, "", nil)
	if err != nil {
		return nil, err
	}
	return CreateReactorFromTree(result.Tree, "", "", result.Plugins, nil, false)
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

// CreateReactor creates a Reactor from a config.LoadConfigResult. standalone
// selects self-hosting mode (see reactor.Config.Standalone); production callers
// pass false so the reactor borrows the hub-owned plugin server.
func CreateReactor(cfg *config.LoadConfigResult, configPath string, store storage.Storage, standalone bool) (*reactor.Reactor, error) {
	r, err := CreateReactorFromTree(cfg.Tree, cfg.ConfigDir, configPath, cfg.Plugins, store, standalone)
	if err != nil {
		return nil, err
	}

	if configPath != "" && configPath != "-" {
		r.SetConfigPath(configPath)
		r.SetReloadFunc(createReloadFunc(store, r))
	}

	return r, nil
}

// LoadReactorWithPlugins parses config with CLI plugins and creates a borrow-mode
// (production) Reactor that expects a hub-injected plugin server before start.
func LoadReactorWithPlugins(store storage.Storage, input, configPath string, cliPlugins []string) (*reactor.Reactor, error) {
	cfg, err := config.LoadConfig(input, configPath, cliPlugins)
	if err != nil {
		return nil, err
	}
	return CreateReactor(cfg, configPath, store, false)
}

// LoadReactorWithPluginsStandalone is LoadReactorWithPlugins for callers that own
// the reactor lifecycle and self-host the plugin server (the ze-chaos in-process
// simulation). The reactor creates its own server, signal handler, and starts
// peers inline instead of borrowing a hub-owned server.
func LoadReactorWithPluginsStandalone(store storage.Storage, input, configPath string, cliPlugins []string) (*reactor.Reactor, error) {
	cfg, err := config.LoadConfig(input, configPath, cliPlugins)
	if err != nil {
		return nil, err
	}
	return CreateReactor(cfg, configPath, store, true)
}

// LoadReactorFile loads config from file and creates Reactor.
func LoadReactorFile(store storage.Storage, path string) (*reactor.Reactor, error) {
	return LoadReactorFileWithPlugins(store, path, nil)
}

// LoadReactorFileWithPlugins loads config from file and creates a borrow-mode
// (production) Reactor.
func LoadReactorFileWithPlugins(store storage.Storage, path string, cliPlugins []string) (*reactor.Reactor, error) {
	return loadReactorFile(store, path, cliPlugins, false)
}

// LoadReactorFileStandalone loads config from file and creates a self-hosting
// (standalone) reactor. Used by `ze bgp --child`, which owns the reactor lifecycle
// itself rather than borrowing a hub-owned plugin server.
func LoadReactorFileStandalone(store storage.Storage, path string) (*reactor.Reactor, error) {
	return loadReactorFile(store, path, nil, true)
}

func loadReactorFile(store storage.Storage, path string, cliPlugins []string, standalone bool) (*reactor.Reactor, error) {
	// "-" reads stdin (claiming it once); a real path goes through the storage
	// abstraction, which may be a blob store where path is a key, not a file.
	var data []byte
	var err error

	if cliio.IsStdin(path) {
		data, err = cliio.ReadFile(path)
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

	return CreateReactor(cfg, path, store, standalone)
}

// injectChaos wraps the reactor's clock, dialer, and listener with chaos fault injection
// when the coordinator has non-zero chaos seed stored by the hub.
func injectChaos(r *reactor.Reactor, coord registry.CoordinatorAccessor) {
	bs := coord.Bootstrap()
	seed := bs.ChaosSeed
	if seed == 0 {
		return
	}
	rate := bs.ChaosRate
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
		// Fail closed: reserved names live outside the config namespace (the
		// break-glass recovery profile and the trusted internal identity). They are
		// un-typeable by construction, so this only fires on a hand-crafted tree,
		// but rejecting it here keeps an operator from ever defining a profile that
		// collides with a reserved allow-all name (spec R-8).
		if aaa.IsReservedName(name) {
			return fmt.Errorf("authorization profile %q uses a reserved name", name)
		}
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
		if aaa.IsReservedName(username) {
			return fmt.Errorf("user %q uses a reserved name", username)
		}
		for _, pn := range userTree.GetSlice("profile") {
			if aaa.IsReservedName(pn) {
				return fmt.Errorf("user %q references reserved profile %q", username, pn)
			}
			if _, ok := profiles[pn]; !ok {
				return fmt.Errorf("user %q references undefined profile %q", username, pn)
			}
		}
	}

	// Check tacacs-profile priv-lvl -> profile references, on the same footing as
	// the user references above. These names decide what a TACACS+-authenticated
	// session may run (the authenticator resolves them at login and authorization
	// applies them), so an undefined one is the same operator error as an
	// undefined user profile -- it just arrives through a different door.
	//
	// Catching it here matters because the runtime cannot report it: authorization
	// receives profile names, not the mapping, so it can only ignore a name it
	// cannot resolve. A typo would otherwise load quietly and surface as a session
	// whose profile silently does not apply.
	for level, entry := range auth.GetList("tacacs-profile") {
		for _, pn := range entry.GetSlice("profile") {
			if aaa.IsReservedName(pn) {
				return fmt.Errorf("tacacs-profile %q references reserved profile %q", level, pn)
			}
			if _, ok := profiles[pn]; !ok {
				return fmt.Errorf("tacacs-profile %q references undefined profile %q", level, pn)
			}
		}
	}

	return nil
}

// ExtractAuthzStore extracts authorization profiles and user assignments from a
// parsed config tree. Returns nil when no system.authorization profiles exist.
func ExtractAuthzStore(tree *config.Tree) *authz.Store {
	return extractAuthzConfig(tree)
}

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
	wireToPaths := yang.WireMethodToPaths(loader)

	var cmds []string
	for _, paths := range wireToPaths {
		for _, path := range paths {
			cmds = append(cmds, strings.ToLower(path))
		}
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
// Returns plain data (no ssh package types). The caller converts to ssh.Config.
// ExtractSSHConfig extracts SSH server configuration from the parsed config tree.
// Returns plain data (no ssh package types). The caller converts to ssh.Config.
func ExtractSSHConfig(tree *config.Tree) SSHExtractedConfig {
	env := tree.GetContainer("environment")
	if env == nil {
		return SSHExtractedConfig{}
	}

	sshContainer := env.GetContainer("ssh")
	if sshContainer == nil {
		return SSHExtractedConfig{}
	}

	var cfg SSHExtractedConfig
	cfg.HasConfig = true

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
		cfg.Listen = addrs[0]
		cfg.ListenAddrs = addrs
	}
	if v, ok := sshContainer.Get("host-key"); ok {
		cfg.HostKeyPath = v
	}
	if v, ok := sshContainer.Get("host-certificate"); ok {
		cfg.HostCertPath = v
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

	if sys := tree.GetContainer("system"); sys != nil {
		if auth := sys.GetContainer("authentication"); auth != nil {
			for name, entry := range auth.GetList("user") {
				var uc authz.UserConfig
				uc.Name = name
				if pw, ok := entry.Get("password"); ok {
					uc.Hash = pw
				}
				uc.Profiles = entry.GetSlice("profile")
				for keyName, keyEntry := range entry.GetList("public-keys") {
					pk := authz.SSHPublicKey{Name: keyName}
					if t, ok := keyEntry.Get("type"); ok {
						pk.Type = t
					}
					if k, ok := keyEntry.Get("key"); ok {
						pk.Key = k
					}
					uc.PublicKeys = append(uc.PublicKeys, pk)
				}
				cfg.Users = append(cfg.Users, uc)
			}
		}
	}

	return cfg
}

// ResolveSSHStorage returns blob storage for SSH host key persistence.
// When the main storage is already blob-backed, it is used directly.
// Otherwise, opens the zefs database independently so SSH host keys
// always go into the blob store rather than the filesystem.
// Tries configDir first, then DefaultConfigDir (binary-relative), because
// configDir may not contain database.zefs (e.g., stdin mode, temp dirs).
// Falls back to the passed store if zefs is not available anywhere.
func ResolveSSHStorage(mainStore storage.Storage, configDir string) storage.Storage {
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
func collectPrefixWarnings(rl plugin.ReactorIntrospector) []LoginWarning {
	peerNames := buildPeerNameLookup(rl)
	issues := report.Warnings()

	var warnings []LoginWarning
	for i := range issues {
		issue := &issues[i]
		if issue.Source != "bgp" {
			continue
		}
		switch issue.Code {
		case "prefix-stale":
			label := peerLabelFromSubject(issue.Subject, peerNames)
			var tb textbuf.Buffer
			warnings = append(warnings, LoginWarning{
				Message: tb.Str(label).Str(" has stale prefix data (updated ").Str(detailString(issue.Detail, "updated")).Byte(')').String(),
				Command: tb.Reset().Str("set bgp peer ").Str(issue.Subject).Str(" prefix").String(),
			})
		case "prefix-threshold":
			peerAddr, fam, ok := splitThresholdSubject(issue.Subject)
			if !ok {
				configLogger().Debug("skipping malformed prefix-threshold subject in banner",
					"subject", issue.Subject)
				continue
			}
			label := peerLabelFromSubject(peerAddr, peerNames)
			var tb textbuf.Buffer
			warnings = append(warnings, LoginWarning{
				Message: tb.Str(label).Byte(' ').Str(fam).Str(" prefix count exceeds warning threshold").String(),
			})
		}
	}

	if len(warnings) == 0 {
		return nil
	}
	if len(warnings) == 1 {
		return warnings
	}
	return []LoginWarning{{
		Message: textbuf.IntStr(int64(len(warnings)), " warnings"),
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
	var tb textbuf.Buffer
	return tb.Str("peer ").Str(addr).String()
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
	var b textbuf.Buffer
	if p.Name != "" {
		return b.Reset().Str("peer ").Str(p.Name).Str(" (AS").Int(int64(p.PeerAS)).Byte(')').String()
	}
	return b.Str("peer ").Addr(p.Address).Str(" (AS").Int(int64(p.PeerAS)).Byte(')').String()
}
